package meals

import (
	"testing"
	"time"

	"messwa/internal/config"
)

func TestNext(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	at := func(hm string) time.Time {
		v, _ := time.ParseInLocation("2006-01-02 15:04", "2026-11-02 "+hm, cfg.Location())
		return v
	}
	cases := []struct {
		at, name, meal, date string
	}{
		{"06:00", "", "breakfast", "2026-11-02"},
		{"08:00", "", "breakfast", "2026-11-02"},
		{"09:31", "", "lunch", "2026-11-02"},
		{"14:00", "", "lunch", "2026-11-02"},
		{"15:00", "", "snacks", "2026-11-02"},
		{"18:30", "", "dinner", "2026-11-02"},
		{"22:00", "", "breakfast", "2026-11-03"},
		{"14:00", "dinner", "dinner", "2026-11-02"},
		{"15:00", "lunch", "lunch", "2026-11-03"},
	}
	for _, c := range cases {
		got, ok := s.Next(at(c.at), c.name)
		if !ok || got.Meal != c.meal || got.Date != c.date {
			t.Errorf("Next(%s, %q) = %+v, want %s %s", c.at, c.name, got, c.meal, c.date)
		}
	}
	if _, ok := s.Current(at("11:00")); ok {
		t.Error("11:00 should be between meals")
	}
	if cur, ok := s.Current(at("13:00")); !ok || cur.Meal != "lunch" {
		t.Errorf("Current(13:00) = %+v", cur)
	}
}
