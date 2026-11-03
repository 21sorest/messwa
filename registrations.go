package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"time"

	"messwa/internal/config"
	"messwa/internal/portal"
)

func registrations(ctx context.Context, cfg *config.Config, args []string) error {
	fs := flag.NewFlagSet("registrations", flag.ContinueOnError)
	from := fs.String("from", time.Now().In(cfg.Location()).Format("2006-01-02"), "first date, YYYY-MM-DD; defaults to today in the configured timezone")
	to := fs.String("to", "", "last date, YYYY-MM-DD; defaults to from")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *to == "" {
		*to = *from
	}
	if err := portal.ValidateRange(*from, *to); err != nil {
		return err
	}
	c, err := portal.FromEnv()
	if err != nil {
		return err
	}
	rows, err := c.Registrations(ctx, *from, *to)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(rows)
}
