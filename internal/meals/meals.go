// Package meals maps timestamps to meal slots.
package meals

import (
	"maps"
	"sort"
	"sync/atomic"
	"time"

	"messwa/internal/config"
)

type Schedule struct {
	loc   *time.Location
	meals []meal // from config.toml, sorted by end time
}

// portal holds the meal windows the mess portal last gave, which win over
// config.toml's. They are the same for every account, so one copy serves
// every Schedule.
var portal atomic.Pointer[map[string]config.MealTime]

// SetPortalTimes makes times, by meal name, the windows every Schedule
// uses from now on. Meals it leaves out keep config.toml's window; nil
// goes back to config.toml for all of them.
func SetPortalTimes(times map[string]config.MealTime) {
	if times == nil {
		portal.Store(nil)
		return
	}
	m := maps.Clone(times)
	portal.Store(&m)
}

// PortalTimes returns the windows set by SetPortalTimes, or nil.
func PortalTimes() map[string]config.MealTime {
	if p := portal.Load(); p != nil {
		return maps.Clone(*p)
	}
	return nil
}

// Times returns each of cfg's meals with the window in use: the portal's
// when it has one, else config.toml's.
func Times(cfg *config.Config) []config.Meal {
	p := portal.Load()
	out := make([]config.Meal, 0, len(cfg.Meals))
	for _, m := range cfg.Meals {
		if p != nil {
			if t, ok := (*p)[m.Name]; ok {
				m.Starts, m.Ends = t.Starts, t.Ends
			}
		}
		out = append(out, m)
	}
	return out
}

type meal struct {
	name       string
	start, end int // minutes after midnight
}

// Slot is one serving of one meal on one date.
type Slot struct {
	Meal  string    `json:"meal"`
	Date  string    `json:"date"` // YYYY-MM-DD in the schedule's timezone
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

func New(cfg *config.Config) *Schedule {
	s := &Schedule{loc: cfg.Location()}
	for _, m := range cfg.Meals {
		st, _ := config.ParseClock(m.Starts)
		en, _ := config.ParseClock(m.Ends)
		s.meals = append(s.meals, meal{m.Name, st, en})
	}
	sortMeals(s.meals)
	return s
}

func sortMeals(ms []meal) {
	sort.Slice(ms, func(i, j int) bool { return ms[i].end < ms[j].end })
}

// current is s's meals with the portal's windows over config.toml's.
func (s *Schedule) current() []meal {
	p := portal.Load()
	if p == nil {
		return s.meals
	}
	out := make([]meal, len(s.meals))
	for i, m := range s.meals {
		if t, ok := (*p)[m.name]; ok {
			st, err1 := config.ParseClock(t.Starts)
			en, err2 := config.ParseClock(t.Ends)
			if err1 == nil && err2 == nil && en > st {
				m.start, m.end = st, en
			}
		}
		out[i] = m
	}
	sortMeals(out)
	return out
}

func (s *Schedule) Location() *time.Location { return s.loc }

// Next returns the first slot that hasn't ended at t. If name is non-empty,
// only that meal counts. A post at 10:00 with no meal named is for today's
// lunch; a post saying "lunch" at 15:00 is for tomorrow's.
func (s *Schedule) Next(t time.Time, name string) (Slot, bool) {
	local := t.In(s.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	for d := 0; d < 2; d++ {
		date := day.AddDate(0, 0, d)
		for _, m := range s.current() {
			if name != "" && m.name != name {
				continue
			}
			end := date.Add(time.Duration(m.end) * time.Minute)
			if end.After(local) {
				return Slot{
					Meal:  m.name,
					Date:  date.Format("2006-01-02"),
					Start: date.Add(time.Duration(m.start) * time.Minute),
					End:   end,
				}, true
			}
		}
	}
	return Slot{}, false
}

// Current returns the slot being served at t, if any.
func (s *Schedule) Current(t time.Time) (Slot, bool) {
	slot, ok := s.Next(t, "")
	if ok && !t.Before(slot.Start) {
		return slot, true
	}
	return Slot{}, false
}

// Windows lists every meal slot for the date containing t.
func (s *Schedule) Windows(t time.Time) []Slot {
	local := t.In(s.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	var out []Slot
	for _, m := range s.current() {
		out = append(out, Slot{
			Meal:  m.name,
			Date:  day.Format("2006-01-02"),
			Start: day.Add(time.Duration(m.start) * time.Minute),
			End:   day.Add(time.Duration(m.end) * time.Minute),
		})
	}
	return out
}

// Without returns the schedule minus the meals hide reports true for. It
// returns s itself when nothing is hidden.
func (s *Schedule) Without(hide func(string) bool) *Schedule {
	out := &Schedule{loc: s.loc}
	for _, m := range s.meals {
		if !hide(m.name) {
			out.meals = append(out.meals, m)
		}
	}
	if len(out.meals) == len(s.meals) {
		return s
	}
	return out
}

// Prev is the meal served before slot: the last one on its date to end by
// its start, else the day before's last. An offer for slot may go up only
// once Prev has ended; posted while another meal is still being served, it
// gets read as one for that meal.
func (s *Schedule) Prev(slot Slot) (Slot, bool) {
	day, err := time.ParseInLocation("2006-01-02", slot.Date, s.loc)
	if err != nil {
		return Slot{}, false
	}
	var prev Slot
	for _, w := range s.Windows(day) {
		if w.Meal != slot.Meal && !w.End.After(slot.Start) && w.End.After(prev.End) {
			prev = w
		}
	}
	if prev.End.IsZero() {
		ws := s.Windows(day.AddDate(0, 0, -1))
		if len(ws) == 0 {
			return Slot{}, false
		}
		prev = ws[len(ws)-1]
	}
	return prev, true
}
