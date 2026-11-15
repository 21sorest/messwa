package meals

import (
	"slices"
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

// The 15 Nov post "buy kadamb" at 09:32 was for breakfast: the portal
// serves it until 10:30, though config.toml says 09:30.
func TestPortalTimes(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	at, _ := time.ParseInLocation("2006-01-02 15:04", "2026-11-15 09:32", cfg.Location())
	if got, _ := s.Next(at, ""); got.Meal != "lunch" {
		t.Fatalf("before the portal, 09:32 = %s", got.Meal)
	}
	SetPortalTimes(map[string]config.MealTime{"breakfast": {Starts: "06:30", Ends: "10:30"}, "lunch": {Starts: "11:30", Ends: "15:30"}})
	defer SetPortalTimes(nil)
	got, _ := s.Next(at, "")
	if got.Meal != "breakfast" || got.End.Format("15:04") != "10:30" {
		t.Errorf("with the portal, 09:32 = %+v", got)
	}
	if w := s.Windows(at); w[1].Meal != "lunch" || w[1].Start.Format("15:04") != "11:30" || w[2].Meal != "snacks" || w[2].End.Format("15:04") != "18:15" {
		t.Errorf("Windows = %+v", w)
	}
	if ts := Times(cfg); ts[0].Starts != "06:30" || ts[3].Ends != "21:30" {
		t.Errorf("Times = %+v", ts)
	}
}

func TestWithoutSnacks(t *testing.T) {
	cfg := config.Default()
	cfg.HideSnacks = true
	s := New(cfg).Without(cfg.HiddenMeal)
	at, _ := time.ParseInLocation("2006-01-02 15:04", "2026-11-02 15:00", cfg.Location())
	if got, ok := s.Next(at, ""); !ok || got.Meal != "dinner" {
		t.Errorf("Next(15:00) = %+v, want dinner", got)
	}
	var names []string
	for _, w := range s.Windows(at) {
		names = append(names, w.Meal)
	}
	if want := []string{"breakfast", "lunch", "dinner"}; !slices.Equal(names, want) {
		t.Errorf("Windows = %v, want %v", names, want)
	}
	cfg.HideSnacks = false
	full := New(cfg)
	if full.Without(cfg.HiddenMeal) != full {
		t.Error("Without should return the same schedule when nothing is hidden")
	}
}

func TestPrev(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	day, _ := time.ParseInLocation("2006-01-02", "2026-11-02", cfg.Location())
	byMeal := map[string]Slot{}
	for _, w := range s.Windows(day) {
		byMeal[w.Meal] = w
	}
	for meal, want := range map[string]string{"lunch": "breakfast", "snacks": "lunch", "dinner": "snacks"} {
		prev, ok := s.Prev(byMeal[meal])
		if !ok || prev.Meal != want || !prev.End.Equal(byMeal[want].End) {
			t.Errorf("Prev(%s) = %+v, %v; want today's %s", meal, prev, ok, want)
		}
	}
	// Breakfast follows the day before's dinner.
	prev, ok := s.Prev(byMeal["breakfast"])
	if !ok || prev.Meal != "dinner" || prev.Date != "2026-11-01" {
		t.Errorf("Prev(breakfast) = %+v, %v; want dinner on 2026-11-01", prev, ok)
	}
}
