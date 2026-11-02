// Command messwa merges WhatsApp mess buy/sell groups into one deduplicated
// stream and serves it, plus limited DM metadata, over MCP.
package main

import (
	"context"
	"crypto/subtle"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
  run                           ingest messages and serve MCP over HTTP
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
	if err := c.Login(ctx, *phone); err != nil {
		return err
	}
	fmt.Println("Done. Next: messwa groups")
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
			if err := st.Prune(ctx, now.Add(-cfg.DMs.Retention.Duration), now.Add(-7*24*time.Hour)); err != nil {
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
