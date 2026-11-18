package main

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"

	"messwa/internal/config"
	"messwa/internal/meals"
)

// Times an admin saved before a restart are back in use after it.
func TestWatchMealTimesLoadsSaved(t *testing.T) {
	cfg := config.Default()
	cfg.StateDir = t.TempDir()
	if err := os.WriteFile(mealTimesFile(cfg), []byte(`{"breakfast":{"starts":"06:30","ends":"10:30"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer meals.SetShared(nil)
	watchMealTimes(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if got := meals.Times(cfg); got[0].Starts != "06:30" || got[0].Ends != "10:30" {
		t.Fatalf("breakfast = %+v", got[0])
	}
}
