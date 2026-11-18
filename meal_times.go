package main

import (
	"context"
	"log/slog"
	"maps"
	"path/filepath"
	"time"

	"messwa/internal/config"
	"messwa/internal/meals"
)

// Admins set the meal windows for everyone on the settings page. They live
// in the main state directory, not in one account's settings, so a save
// reaches every account at once. The file name is the one the old mess
// portal sync wrote, so the windows it last fetched carry over.
func mealTimesFile(cfg *config.Config) string { return filepath.Join(cfg.StateDir, "meal-times.json") }

// watchMealTimes loads the shared meal windows, then reloads them every
// 30 seconds until ctx ends, to pick up a save made by another process
// such as messwa dashboard while run is up.
func watchMealTimes(ctx context.Context, cfg *config.Config, log *slog.Logger) {
	if err := meals.UseFile(mealTimesFile(cfg)); err != nil {
		log.Warn("could not read the saved meal times; using config.toml's", "err", err)
	}
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			before := meals.Shared()
			if err := meals.Reload(); err != nil {
				log.Warn("could not reload meal times", "err", err)
				continue
			}
			if now := meals.Shared(); !maps.Equal(now, before) {
				log.Info("meal times changed", "times", now)
			}
		}
	}()
}
