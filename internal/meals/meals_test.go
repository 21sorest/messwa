package meals

import (
	"os"
	"path/filepath"
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

// The 15 Nov post "buy kadamb" at 09:32 was for breakfast: the mess
// serves it until 10:30, though config.toml says 09:30.
func TestSharedTimes(t *testing.T) {
	cfg := config.Default()
	s := New(cfg)
	at, _ := time.ParseInLocation("2006-01-02 15:04", "2026-11-15 09:32", cfg.Location())
	if got, _ := s.Next(at, ""); got.Meal != "lunch" {
		t.Fatalf("with config.toml's times, 09:32 = %s", got.Meal)
	}
	SetShared(map[string]config.MealTime{"breakfast": {Starts: "06:30", Ends: "10:30"}, "lunch": {Starts: "11:30", Ends: "15:30"}})
	defer SetShared(nil)
	got, _ := s.Next(at, "")
	if got.Meal != "breakfast" || got.End.Format("15:04") != "10:30" {
		t.Errorf("with shared times, 09:32 = %+v", got)
	}
	if w := s.Windows(at); w[1].Meal != "lunch" || w[1].Start.Format("15:04") != "11:30" || w[2].Meal != "snacks" || w[2].End.Format("15:04") != "18:15" {
		t.Errorf("Windows = %+v", w)
	}
	if ts := Times(cfg); ts[0].Starts != "06:30" || ts[3].Ends != "21:30" {
		t.Errorf("Times = %+v", ts)
	}
}

func TestCheck(t *testing.T) {
	cfg := config.Default()
	ok := map[string]config.MealTime{"breakfast": {Starts: "06:30", Ends: "10:30"}, "lunch": {Starts: "11:30", Ends: "15:30"},
		"snacks": {Starts: "16:45", Ends: "18:15"}, "dinner": {Starts: "18:30", Ends: "22:30"}}
	if err := Check(cfg, ok); err != nil {
		t.Errorf("the mess's 15 Nov times: %v", err)
	}
	if err := Check(cfg, nil); err != nil {
		t.Errorf("config.toml's own times: %v", err)
	}
	for name, bad := range map[string]map[string]config.MealTime{
		"overlap":       {"lunch": {Starts: "11:30", Ends: "15:30"}, "snacks": {Starts: "15:00", Ends: "18:15"}},
		"overlaps toml": {"breakfast": {Starts: "07:30", Ends: "13:00"}},
		"backwards":     {"lunch": {Starts: "15:30", Ends: "11:30"}},
		"bad clock":     {"lunch": {Starts: "noon", Ends: "15:30"}},
		"unknown meal":  {"brunch": {Starts: "10:00", Ends: "11:00"}},
	} {
		if err := Check(cfg, bad); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// A save reaches every Schedule at once and survives a reload, as when
// run picks up a save made by messwa dashboard.
func TestSaveAndReload(t *testing.T) {
	defer file.Store(nil)
	defer SetShared(nil)
	path := filepath.Join(t.TempDir(), "meal-times.json")
	if err := UseFile(path); err != nil || Shared() != nil {
		t.Fatalf("no file yet: %v %v", Shared(), err)
	}
	times := map[string]config.MealTime{"breakfast": {Starts: "06:30", Ends: "10:30"}}
	if err := Save(times); err != nil {
		t.Fatal(err)
	}
	SetShared(nil)
	if err := Reload(); err != nil || Shared()["breakfast"] != times["breakfast"] {
		t.Fatalf("after reload = %v, %v", Shared(), err)
	}
	if err := Save(nil); err != nil || Shared() != nil {
		t.Fatalf("reset = %v, %v", Shared(), err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("reset left the file: %v", err)
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
