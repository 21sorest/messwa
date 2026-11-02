// Command messwa merges WhatsApp mess buy/sell groups into one deduplicated
// stream and serves it, plus limited DM metadata, over MCP.
package main

import (
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
	"sync"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.mau.fi/whatsmeow/types/events"

	"messwa/internal/config"
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

inspection (read the database only, safe while run is up):
  chat [-hours 48]              the merged group stream, each post annotated with its listing
  listings [-hours 48]          deduplicated listings with tags and status
  rebuild                       re-derive all listings from the stored group log
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
	case "import-history":
		err = importHistory(ctx, cfg, args)
	case "chat":
		err = chat(ctx, cfg, args)
	case "listings":
		err = listings(ctx, cfg, args)
	case "rebuild":
		err = rebuild(ctx, cfg)
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
	c.Ingest(ingest.New(cfg, st), st)
	if err := c.WA.Connect(); err != nil {
		return err
	}
	defer c.WA.Disconnect()
	log.Info("connected to WhatsApp", "groups", len(cfg.Groups))

	go func() {
		for t := time.NewTicker(time.Hour); ; {
			now := time.Now()
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

	srv := mcpserver.New(cfg, st, c).MCP()
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
	if err := hs.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
