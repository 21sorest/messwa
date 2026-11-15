package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestRecurringStorageSurvivesReopenAndRejectsStaleGenerator(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := t.Context()
	now := time.Now()
	r, err := s.SaveRecurringRule(ctx, RecurringRule{UserID: 7, Name: "Lunches", From: "2026-11-16", Through: "2026-11-20", Weekdays: []int{1, 2, 3, 4, 5}, Meals: []string{"lunch"}, Min: 30, Max: 60, Before: 15, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	sc := SellSchedule{RuleID: r.ID, RuleRevision: r.Revision, UserID: 7, Date: r.From, Meal: "lunch", MinPaise: 3000, MaxPaise: 6000, StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), CreatedAt: now}
	id, err := s.CreateSellSchedule(ctx, sc)
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := s.CancelPlannedSchedule(ctx, id, 1); err != nil || !ok {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rules, err := s.RecurringRules(ctx)
	if err != nil || len(rules) != 1 || rules[0].Name != "Lunches" || rules[0].Revision != 1 {
		t.Fatalf("rules %+v %v", rules, err)
	}
	kept, err := s.KeptMeals(ctx, 7)
	if err != nil || !kept[r.From+"|lunch"] {
		t.Fatal("lost kept exception after restart")
	}
	if _, err := s.CreateSellSchedule(ctx, sc); err == nil {
		t.Fatal("generator ignored exception")
	}
	if err := s.AllowRecurringMeal(ctx, 7, r.From, "lunch"); err != nil {
		t.Fatal(err)
	}
	r.Min = 40
	r, err = s.SaveRecurringRule(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateSellSchedule(ctx, sc); err == nil {
		t.Fatal("stale generator created schedule")
	}
	sc.RuleRevision = r.Revision
	if _, err := s.CreateSellSchedule(ctx, sc); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DeleteRecurringRule(ctx, 8, r.ID, r.Revision, true); err == nil {
		t.Fatal("another account deleted rule")
	}
	if _, err := s.DeleteRecurringRule(ctx, 7, r.ID, r.Revision, false); err != nil {
		t.Fatal(err)
	}
	sc.Date = "2026-11-17"
	if _, err := s.CreateSellSchedule(ctx, sc); err == nil {
		t.Fatal("deleted rule created schedule")
	}
	other, err := s.KeptMeals(ctx, 8)
	if err != nil || len(other) != 0 {
		t.Fatal("another account sees exceptions")
	}
}
