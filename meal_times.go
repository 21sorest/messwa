package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"time"

	"messwa/internal/config"
	"messwa/internal/meals"
	"messwa/internal/portal"
)

// Meal times come from the mess portal, which serves breakfast 06:30 to
// 10:30 where config.toml says 07:30 to 09:30. A post at 09:32 that names
// no meal is for breakfast; with config.toml's window the bot filed it as
// lunch and offered the poster a lunch they never wanted.
const mealTimesEvery = 3 * time.Hour

func mealTimesFile(cfg *config.Config) string { return filepath.Join(cfg.StateDir, "meal-times.json") }

// syncMealTimes puts the portal's meal windows in use: the last ones saved
// in the state directory at once, so a start without network still has
// them, then fresh ones now and every mealTimesEvery until ctx ends. If the
// portal never answers, config.toml's windows stay.
func syncMealTimes(ctx context.Context, cfg *config.Config, log *slog.Logger) {
	path := mealTimesFile(cfg)
	if b, err := os.ReadFile(path); err == nil {
		var saved map[string]config.MealTime
		if json.Unmarshal(b, &saved) == nil && validMealTimes(saved) {
			meals.SetPortalTimes(saved)
		}
	}
	c, err := portal.Public()
	if err != nil {
		log.Warn("meal times stay as saved: no mess portal address", "err", err)
		return
	}
	for {
		refreshMealTimes(ctx, c, cfg, path, log)
		select {
		case <-ctx.Done():
			return
		case <-time.After(mealTimesEvery):
		}
	}
}

func refreshMealTimes(ctx context.Context, c *portal.Client, cfg *config.Config, path string, log *slog.Logger) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	got, err := c.MealTimings(ctx, time.Now().In(cfg.Location()).Format("2006-01-02"))
	if err != nil {
		log.Warn("could not read meal times from the mess portal", "err", err)
		return
	}
	times := map[string]config.MealTime{}
	for meal, w := range got {
		times[meal] = config.MealTime{Starts: w.Starts, Ends: w.Ends}
	}
	if !validMealTimes(times) {
		log.Warn("mess portal meal times overlap; keeping the ones in use", "times", times)
		return
	}
	if maps.Equal(times, meals.PortalTimes()) {
		return
	}
	meals.SetPortalTimes(times)
	log.Info("meal times from the mess portal", "times", times)
	b, _ := json.Marshal(times)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		log.Warn("could not save meal times", "err", err)
	}
}

// validMealTimes reports whether times parse and no two meals overlap, so
// a slot can only ever be one meal.
func validMealTimes(times map[string]config.MealTime) bool {
	if len(times) == 0 {
		return false
	}
	type span struct{ st, en int }
	var spans []span
	for _, t := range times {
		st, err1 := config.ParseClock(t.Starts)
		en, err2 := config.ParseClock(t.Ends)
		if err1 != nil || err2 != nil || en <= st {
			return false
		}
		for _, o := range spans {
			if st < o.en && o.st < en {
				return false
			}
		}
		spans = append(spans, span{st, en})
	}
	return true
}
