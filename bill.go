package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"messwa/internal/config"
)

func bill(ctx context.Context, cfg *config.Config, args []string) error {
	now := time.Now().In(cfg.Location())
	fs := flag.NewFlagSet("bill", flag.ContinueOnError)
	month := fs.String("month", now.Format("2006-01"), "month to read, YYYY-MM; defaults to this month in the configured timezone")
	user := fs.String("user", "", "read with this account's mess API key; defaults to the first admin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	m, err := time.Parse("2006-01", *month)
	if err != nil {
		return fmt.Errorf("month must be YYYY-MM")
	}
	c, err := userPortal(ctx, cfg, *user)
	if err != nil {
		return err
	}
	b, err := c.Bill(ctx, m.Year(), int(m.Month()))
	if err != nil {
		return err
	}
	paise := func(p int64) string { return fmt.Sprintf("%d.%02d", p/100, p%100) }
	fmt.Printf("%s mess bill\n  availed    ₹%s\n  projected  ₹%s\n  total      ₹%s\n", m.Format("January 2006"), paise(b.Availed), paise(b.Projected), paise(b.Total()))
	return nil
}
