// Command messwa merges WhatsApp mess buy/sell groups into one deduplicated
// stream and serves it, plus limited DM metadata, over MCP.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mau.fi/whatsmeow/types/events"

	"messwa/internal/config"
	"messwa/internal/dashboard"
	"messwa/internal/ingest"
	"messwa/internal/mcpserver"
	"messwa/internal/store"
	"messwa/internal/wa"
)

const usage = `usage: messwa [-config config.toml] <command>

commands:
  login [-phone 91XXXXXXXXXX]   link as a WhatsApp companion device (QR, or pairing code with -phone)
  groups                        list your groups and their JIDs, to fill in config.toml
  import-history [-keep]        load the group history saved by login (configured groups only)
  run                           ingest messages and serve MCP over HTTP
  inspect-payment -peer JID     inspect live incoming payment fields from one DM (stop run first)
  registrations [-from DATE] [-to DATE]
                                read your meal registrations from the mess portal
  fetch-qr [-out IMAGE]         fetch the current portal QR into a private PNG file
  deliver-qr -date DATE -meal MEAL -peer JID -amount INR [-qr IMAGE | -portal-qr]
                                arm one meal's QR delivery after a matching payment message

inspection (read the database only, safe while run is up):
  chat [-hours 48]              the merged group stream, each post annotated with its listing
  listings [-hours 48]          deduplicated listings with tags and status
  rebuild                       re-derive all listings from the stored group log
  dashboard [-listen addr] [-at time]
                                serve the dashboard alone (run serves it too)
`

func main() {
	cfgPath := flag.String("config", "config.toml", "config file")
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()
	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(2)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fatal(err)
	}
	if err := config.EnsureDir(cfg.StateDir); err != nil {
		fatal(err)
	}
	args := flag.Args()[1:]
	switch flag.Arg(0) {
	case "login":
		err = login(ctx, cfg, log, args)
	case "groups":
		err = groups(ctx, cfg, log)
	case "run":
		err = run(ctx, cfg, log)
	case "inspect-payment":
		err = inspectPayment(ctx, cfg, log, args)
	case "registrations":
		err = registrations(ctx, cfg, args)
	case "fetch-qr":
		err = fetchPortalQR(ctx, cfg, args)
	case "deliver-qr":
		err = deliverQR(ctx, cfg, log, args)
	case "import-history":
		err = importHistory(ctx, cfg, args)
	case "chat":
		err = chat(ctx, cfg, args)
	case "listings":
		err = listings(ctx, cfg, args)
	case "rebuild":
		err = rebuild(ctx, cfg)
	case "dashboard":
		err = dashboardCmd(ctx, cfg, log, args)
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "messwa:", err)
	os.Exit(1)
}

func login(ctx context.Context, cfg *config.Config, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("login", flag.ExitOnError)
	phone := fs.String("phone", "", "link with a pairing code for this number instead of a QR code")
	fs.Parse(args)
	c, err := wa.Open(ctx, cfg, log)
	if err != nil {
		return err
	}
	if c.LoggedIn() {
		return fmt.Errorf("already linked as %s; to relink, remove the device in WhatsApp and delete %s", c.WA.Store.ID, cfg.SessionDB())
	}
	if *phone == "" && fs.NFlag() == 0 {
		*phone = askPhone()
	}

	// The history sync after pairing is the only backlog WhatsApp sends.
	// Save group conversations (never DMs) so import-history can load the
	// market groups once they're configured.
	f, err := os.OpenFile(historyFile(cfg), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	var mu sync.Mutex
	var last time.Time
	saved := 0
	enc := json.NewEncoder(f)
	c.WA.AddEventHandler(func(evt any) {
		hs, ok := evt.(*events.HistorySync)
		if !ok {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		for _, conv := range hs.Data.GetConversations() {
			for _, e := range c.HistoryGroupEntries(ctx, conv) {
				if err := enc.Encode(e); err == nil {
					saved++
				}
			}
		}
		last = time.Now()
	})

	if err := c.Login(ctx, *phone); err != nil {
		return err
	}
	defer c.WA.Disconnect()
	fmt.Println("Linked. Waiting for WhatsApp's history sync (keep the phone online)...")
	start := time.Now()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
		mu.Lock()
		quiet, n := time.Since(last), saved
		mu.Unlock()
		elapsed := time.Since(start)
		fmt.Printf("\r  %d group messages saved, %s elapsed   ", n, elapsed.Round(time.Second))
		if (n > 0 && quiet > 90*time.Second) || elapsed > 4*time.Minute && quiet > 90*time.Second || elapsed > 15*time.Minute {
			break
		}
	}
	fmt.Printf("\nSaved %d group messages to %s. Next: messwa groups\n", saved, historyFile(cfg))
	return nil
}

// askPhone asks how to link. A number means pairing code, blank means QR.
func askPhone() string {
	fmt.Println("Link messwa to WhatsApp.")
	fmt.Println("  - To link with a code, enter your number with country code, digits only (e.g. 91XXXXXXXXXX).")
	fmt.Println("  - To scan a QR code instead, just press Enter.")
	fmt.Print("Number: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	var digits strings.Builder
	for _, r := range line {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	return digits.String()
}

func historyFile(cfg *config.Config) string { return cfg.StateDir + "/history-groups.jsonl" }

func importHistory(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("import-history", flag.ExitOnError)
	keep := fs.Bool("keep", false, "keep the history file afterwards (it holds every group's messages)")
	fs.Parse(args)
	if len(cfg.Groups) == 0 {
		return errors.New("no groups in config yet")
	}
	data, err := os.ReadFile(historyFile(cfg))
	if err != nil {
		return err
	}
	var entries []store.LogEntry
	for _, line := range bytes.Split(data, []byte("\n")) {
		var e store.LogEntry
		if len(line) > 0 && json.Unmarshal(line, &e) == nil {
			entries = append(entries, e)
		}
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].TS.Before(entries[j].TS) })
	st, err := store.Open(cfg.DataDB())
	if err != nil {
		return err
	}
	defer st.Close()
	in := ingest.New(cfg, st)
	per := map[string]int{}
	for _, e := range entries {
		if !in.IsMarketGroup(e.Chat) {
			continue
		}
		if err := in.Handle(ctx, e); err != nil {
			return err
		}
		per[e.Chat]++
	}
	for _, g := range cfg.Groups {
		fmt.Printf("%-40s %5d events\n", g, per[g])
	}
	if !*keep {
		return os.Remove(historyFile(cfg))
	}
	return nil
}

func groups(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	c, err := wa.Open(ctx, cfg, log)
	if err != nil {
		return err
	}
	if !c.LoggedIn() {
		return errors.New("not linked; run messwa login first")
	}
	if err := c.WA.Connect(); err != nil {
		return err
	}
	defer c.WA.Disconnect()
	gs, err := c.Groups(ctx)
	if err != nil {
		return err
	}
	for _, g := range gs {
		fmt.Printf("%-40s %4d members  %s\n", g.JID, g.Participants, g.Name)
	}
	fmt.Println("\nCopy the mess groups' JIDs into `groups = [...]` in config.toml.")
	return nil
}

func run(ctx context.Context, cfg *config.Config, log *slog.Logger) error {
	if len(cfg.Groups) == 0 {
		return errors.New("no groups in config; run messwa groups and fill in `groups`")
	}
	token := ""
	if cfg.MCP.TokenEnv != "" {
		token = os.Getenv(cfg.MCP.TokenEnv)
	}
	if token == "" && !loopback(cfg.MCP.Listen) {
		return fmt.Errorf("mcp.listen %q is not loopback; set %s to a bearer token", cfg.MCP.Listen, cfg.MCP.TokenEnv)
	}

	// Bind first so a port clash fails before touching WhatsApp.
	ln, err := net.Listen("tcp", cfg.MCP.Listen)
	if err != nil {
		return err
	}
	var dashLn net.Listener
	if cfg.Dash.Listen != "" {
		if dashLn, err = net.Listen("tcp", cfg.Dash.Listen); err != nil {
			return fmt.Errorf("dashboard: %w", err)
		}
	}
	st, err := store.Open(cfg.DataDB())
	if err != nil {
		return err
	}
	defer st.Close()
	c, err := wa.Open(ctx, cfg, log)
	if err != nil {
		return err
	}
	if !c.LoggedIn() {
		return errors.New("not linked; run messwa login first")
	}
	hub := dashboard.NewHub()
	in := ingest.New(cfg, st)
	in.OnChange(hub.Notify)
	c.Ingest(in, st)
	if err := c.WA.Connect(); err != nil {
		return err
	}
	defer c.WA.Disconnect()
	log.Info("connected to WhatsApp", "groups", len(cfg.Groups))

	go func() {
		for t := time.NewTicker(time.Hour); ; {
			now := time.Now()
			// Save asking prices before pruning, while every listing of the
			// last 12 days is still there. Older servings are already saved.
			if err := st.SnapshotPrices(ctx, now.In(cfg.Location()).AddDate(0, 0, -12).Format("2006-01-02")); err != nil {
				log.Error("snapshot prices", "err", err)
			}
			if err := st.Prune(ctx, now.Add(-cfg.DMs.Retention.Duration), now.Add(-14*24*time.Hour)); err != nil {
				log.Error("prune", "err", err)
			}
			select {
			case <-ctx.Done():
				t.Stop()
				return
			case <-t.C:
			}
		}
	}()

	ms := mcpserver.New(cfg, st, c)
	if dashLn != nil {
		go serve(ctx, log, dashLn, dashboard.Handler(cfg, ms, hub))
		log.Info("serving dashboard", "url", "http://"+cfg.Dash.Listen+"/")
	}
	srv := ms.MCP()
	var h http.Handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	if token != "" {
		h = bearer(token, h)
	}
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		if !c.WA.IsConnected() {
			http.Error(w, "whatsapp disconnected", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	hs := &http.Server{Addr: cfg.MCP.Listen, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	log.Info("serving MCP", "url", "http://"+cfg.MCP.Listen+"/mcp", "auth", token != "")
	if err := hs.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// serve runs an HTTP server on ln until ctx ends.
func serve(ctx context.Context, log *slog.Logger, ln net.Listener, h http.Handler) error {
	// Requests inherit ctx so open event streams end when we shut down.
	hs := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	if err := hs.Serve(ln); !errors.Is(err, http.ErrServerClosed) {
		log.Error("http", "addr", ln.Addr().String(), "err", err)
		return err
	}
	return nil
}

// dashboardCmd serves the dashboard from the database alone, without
// connecting to WhatsApp. Names come from posts, not your contacts.
func dashboardCmd(ctx context.Context, cfg *config.Config, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("dashboard", flag.ExitOnError)
	listen := fs.String("listen", "127.0.0.1:8768", "address to serve on")
	at := fs.String("at", "", `show the market as it was at this time, "2006-01-02 15:04" local`)
	fs.Parse(args)
	st, err := store.Open(cfg.DataDB())
	if err != nil {
		return err
	}
	defer st.Close()
	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	ms := mcpserver.New(cfg, st, contactsFromDB(cfg))
	if *at != "" {
		t, err := time.ParseInLocation("2006-01-02 15:04", *at, cfg.Location())
		if err != nil {
			return fmt.Errorf("-at: %w", err)
		}
		offset := time.Until(t)
		ms.SetClock(func() time.Time { return time.Now().Add(offset) })
	}
	fmt.Printf("dashboard on http://%s/\n", *listen)
	return serve(ctx, log, ln, dashboard.Handler(cfg, ms, dashboard.NewHub()))
}

func bearer(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
