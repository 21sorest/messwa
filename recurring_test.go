package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"messwa/internal/portal"
	"messwa/internal/store"
)

type recurringPortal struct {
	fakeMealPortal
	rows []portal.Registration
	err  error
}

func (p *recurringPortal) Registrations(context.Context, string, string) ([]portal.Registration, error) {
	return p.rows, p.err
}

func recurringFixture(t *testing.T, path string) (*sellFixture, *sellManager, *recurringPortal, context.CancelFunc) {
	f := newSellFixture(t, path)
	f.cfg.MCP.SellTasks = true
	ctx, cancel := context.WithCancel(t.Context())
	m := f.manager(ctx)
	p := &recurringPortal{fakeMealPortal: fakeMealPortal{mess: "palash"}}
	for i := 1; i <= 5; i++ {
		date := time.Now().In(f.cfg.Location()).AddDate(0, 0, i).Format("2006-01-02")
		p.rows = append(p.rows, portal.Registration{Date: date, Meal: "lunch", Mess: "palash", Category: "registered"})
	}
	m.portalFor = func(context.Context, int64) (mealPortal, error) { return p, nil }
	return f, m, p, cancel
}

func testRule(p *recurringPortal) store.RecurringRule {
	return store.RecurringRule{Name: "Weekday lunches", From: p.rows[0].Date, Through: p.rows[len(p.rows)-1].Date, Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, Meals: []string{"lunch"}, Min: 30, Max: 60, Before: 15, Enabled: true}
}

func waitingRuleSchedules(t *testing.T, st *store.Store, id int64) []store.SellSchedule {
	t.Helper()
	all, err := st.SellSchedules(t.Context(), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	out := []store.SellSchedule{}
	for _, sc := range all {
		if sc.RuleID == id && sc.State == store.ScheduleWaiting {
			out = append(out, sc)
		}
	}
	return out
}

func TestRecurringGenerationKeepsExceptionsAndPreventsOverlaps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f, m, p, cancel := recurringFixture(t, path)
		defer cancel()
		ctx := t.Context()
		r := testRule(p)
		preview, err := m.PreviewRecurringRule(ctx, 7, r, false)
		if err != nil || preview.New != 5 {
			t.Fatalf("preview %+v %v", preview, err)
		}
		r, err = m.SaveRecurringRule(ctx, 7, r, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := waitingRuleSchedules(t, f.st, r.ID); len(got) != 5 {
			t.Fatalf("generated %d", len(got))
		}
		m.refreshRules(ctx)
		if got := waitingRuleSchedules(t, f.st, r.ID); len(got) != 5 {
			t.Fatal("refresh duplicated schedules")
		}
		second := testRule(p)
		second.Name = "Overlapping rule"
		second, err = m.SaveRecurringRule(ctx, 7, second, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := waitingRuleSchedules(t, f.st, second.ID); len(got) != 0 {
			t.Fatal("overlapping rule duplicated meals")
		}
		kept := waitingRuleSchedules(t, f.st, r.ID)[0]
		if err := m.CancelPlannedSchedule(ctx, kept.ID, kept.Revision); err != nil {
			t.Fatal(err)
		}
		m.refreshRules(ctx)
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 4 || len(waitingRuleSchedules(t, f.st, second.ID)) != 0 {
			t.Fatal("kept meal was regenerated")
		}
		exceptions, err := f.st.KeptMeals(ctx, 7)
		if err != nil || !exceptions[kept.Date+"|"+kept.Meal] {
			t.Fatal("exception not durable")
		}
		if err := m.AllowRecurringMeal(ctx, 7, kept.Date, kept.Meal); err != nil {
			t.Fatal(err)
		}
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 5 {
			t.Fatal("restored meal not generated")
		}
		cancel()
		synctest.Wait()
	})
}

func TestRecurringRuleEditsPreserveOverridesAndActiveSales(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f, m, p, cancel := recurringFixture(t, path)
		defer cancel()
		ctx := t.Context()
		r, err := m.SaveRecurringRule(ctx, 7, testRule(p), false)
		if err != nil {
			t.Fatal(err)
		}
		sc := waitingRuleSchedules(t, f.st, r.ID)[0]
		if _, _, err := m.PlanSchedule(ctx, 7, sc.ID, sc.Revision, sc.Date, sc.Meal, 35, 75, sc.StartsAt, true); err != nil {
			t.Fatal(err)
		}
		r.Min, r.Max = 40, 80
		r, err = m.SaveRecurringRule(ctx, 7, r, true)
		if err != nil {
			t.Fatal(err)
		}
		for _, got := range waitingRuleSchedules(t, f.st, r.ID) {
			if got.ID == sc.ID {
				if got.MinPaise != 3500 || !got.Overridden {
					t.Fatal("individual override overwritten")
				}
			} else if got.MinPaise != 4000 {
				t.Fatal("waiting defaults not applied")
			}
		}
		active := waitingRuleSchedules(t, f.st, r.ID)[1]
		task, err := m.StartNow(ctx, 7, active.Date, active.Meal, 40, 80, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := m.DeleteRecurringRule(ctx, 7, r.ID, r.Revision, true); err != nil {
			t.Fatal(err)
		}
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 0 {
			t.Fatal("rule's waiting schedules not removed")
		}
		replacement := testRule(p)
		replacement.Name = "Replacement"
		replacement, err = m.SaveRecurringRule(ctx, 7, replacement, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(waitingRuleSchedules(t, f.st, replacement.ID)) != 0 {
			t.Fatal("another rule recreated explicitly removed meals")
		}
		got, err := f.st.GetSellTask(ctx, task)
		if err != nil || got.State != store.TaskActive {
			t.Fatal("rule removal stopped an active sale")
		}
		cancel()
		synctest.Wait()
	})
}

func TestRecurringPicksUpNewRegistrationsWithoutRuleEdits(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f, m, p, cancel := recurringFixture(t, path)
		defer cancel()
		ctx := t.Context()
		r := testRule(p)
		late := p.rows[4]
		p.rows = p.rows[:4]
		r, err := m.SaveRecurringRule(ctx, 7, r, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 4 {
			t.Fatal("scheduled unconfirmed registration")
		}
		p.rows = append(p.rows, late)
		m.refreshRules(ctx)
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 5 {
			t.Fatal("did not pick up new registration")
		}
		failed := waitingRuleSchedules(t, f.st, r.ID)[0]
		if ok, err := f.st.EndSellSchedule(ctx, failed.ID, store.ScheduleFailed, 0, "Invalid registration", time.Now()); err != nil || !ok {
			t.Fatal(err)
		}
		second := testRule(p)
		second.Name = "Overlap after failure"
		second, err = m.SaveRecurringRule(ctx, 7, second, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(waitingRuleSchedules(t, f.st, second.ID)) != 0 {
			t.Fatal("overlapping rule retried a terminally failed meal")
		}
		cancel()
		synctest.Wait()
	})
}

func TestRecurringRestartRespectsExceptionsAndLaterRegistrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f, m, p, stop := recurringFixture(t, path)
		ctx := t.Context()
		r, err := m.SaveRecurringRule(ctx, 7, testRule(p), false)
		if err != nil {
			t.Fatal(err)
		}
		kept := waitingRuleSchedules(t, f.st, r.ID)[0]
		if err := m.CancelPlannedSchedule(ctx, kept.ID, kept.Revision); err != nil {
			t.Fatal(err)
		}
		stop()
		synctest.Wait()
		runCtx, cancel := context.WithCancel(t.Context())
		defer cancel()
		next := f.manager(runCtx)
		next.portalFor = m.portalFor
		if err := next.Resume(runCtx, true); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 4 {
			t.Fatal("restart duplicated or recreated kept meals")
		}
		p.rows = append(p.rows, portal.Registration{Date: p.rows[0].Date, Meal: "dinner", Mess: "palash", Category: "registered"})
		r.Meals = append(r.Meals, "dinner")
		r, err = next.SaveRecurringRule(ctx, 7, r, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 5 {
			t.Fatal("new confirmed registration not picked up")
		}
		p.err = errors.New("portal temporarily unavailable")
		next.refreshRules(ctx)
		rules, _ := f.st.RecurringRules(ctx)
		if rules[0].LastError == "" {
			t.Fatal("generation failure not retained for alerts")
		}
		p.err = nil
		next.refreshRules(ctx)
		rules, _ = f.st.RecurringRules(ctx)
		if rules[0].LastError != "" {
			t.Fatal("recovered rule error not cleared")
		}
		cancel()
		synctest.Wait()
	})
}

func TestRecurringRejectsInvalidAndUsedRegistrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f, m, p, cancel := recurringFixture(t, path)
		defer cancel()
		ctx := t.Context()
		value := "used"
		originalRule := testRule(p)
		p.rows[0].AvailedAt = &value
		p.rows[1].CancelledAt = &value
		p.rows[2].Category = "extra"
		p.rows = append(p.rows, p.rows[3])
		r, err := m.SaveRecurringRule(ctx, 7, originalRule, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(waitingRuleSchedules(t, f.st, r.ID)) != 1 {
			t.Fatal("used, cancelled, extra or ambiguous registration scheduled")
		}
		if _, _, err := m.PlanSchedule(ctx, 8, waitingRuleSchedules(t, f.st, r.ID)[0].ID, 1, p.rows[4].Date, "lunch", 30, 60, waitingRuleSchedules(t, f.st, r.ID)[0].StartsAt, false); err == nil {
			t.Fatal("cross-account edit allowed")
		}
		stale := r
		r.Min = 40
		r, err = m.SaveRecurringRule(ctx, 7, r, false)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := m.SaveRecurringRule(ctx, 7, stale, true); err == nil {
			t.Fatal("stale rule edit accepted")
		}
		if err := m.DeleteRecurringRule(ctx, 8, r.ID, r.Revision, true); err == nil {
			t.Fatal("cross-account rule deletion accepted")
		}
		cancel()
		synctest.Wait()
	})
}

func TestRecurringMedianPricingFollowsTheMarket(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f, m, p, cancel := recurringFixture(t, path)
		defer cancel()
		ctx := t.Context()
		ask := []int{40, 40, 40}
		m.prices = func(context.Context, string, string) ([]store.ServingPrices, error) {
			var out []store.ServingPrices
			for i := 0; i < 7; i++ {
				date := time.Now().In(f.cfg.Location()).AddDate(0, 0, -i).Format("2006-01-02")
				out = append(out, store.ServingPrices{Date: date, Meal: "lunch", Mess: "palash", Prices: ask})
			}
			return out, nil
		}
		r := testRule(p)
		r.Pricing = "median"
		r.Relative = map[string]store.RelativePrice{"lunch": {Low: -25, High: 50}}

		preview, err := m.PreviewRecurringRule(ctx, 7, r, false)
		if err != nil || preview.New != 5 || preview.Lowest != 30 || preview.Highest != 60 || preview.Fallback != 0 {
			t.Fatalf("preview %+v %v", preview, err)
		}
		r, err = m.SaveRecurringRule(ctx, 7, r, false)
		if err != nil {
			t.Fatal(err)
		}
		got := waitingRuleSchedules(t, f.st, r.ID)
		if len(got) != 5 || got[0].MinPaise != 3000 || got[0].MaxPaise != 6000 {
			t.Fatalf("median prices not applied: %+v", got)
		}

		edited := got[0]
		edited.MinPaise, edited.MaxPaise = 5500, 7000
		if ok, err := f.st.UpdateSellSchedule(ctx, edited); err != nil || !ok {
			t.Fatalf("override %v %v", ok, err)
		}

		ask = []int{60, 60, 60}
		m.refreshRules(ctx)
		for _, sc := range waitingRuleSchedules(t, f.st, r.ID) {
			want := [2]int64{4500, 9000}
			if sc.ID == edited.ID {
				want = [2]int64{5500, 7000}
			}
			if sc.MinPaise != want[0] || sc.MaxPaise != want[1] {
				t.Fatalf("schedule %d priced %d-%d, want %v", sc.ID, sc.MinPaise, sc.MaxPaise, want)
			}
		}

		cancel()
		synctest.Wait()
	})
}

func TestRecurringMedianFallsBackToFixedRange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f, m, p, cancel := recurringFixture(t, path)
		defer cancel()
		ctx := t.Context()
		m.prices = func(context.Context, string, string) ([]store.ServingPrices, error) { return nil, nil }
		r := testRule(p)
		r.Pricing = "median"
		r.Relative = map[string]store.RelativePrice{"lunch": {Low: -10, High: 20}}
		preview, err := m.PreviewRecurringRule(ctx, 7, r, false)
		if err != nil || preview.Fallback != 5 || preview.Lowest != 30 || preview.Highest != 60 {
			t.Fatalf("preview %+v %v", preview, err)
		}
		r, err = m.SaveRecurringRule(ctx, 7, r, false)
		if err != nil {
			t.Fatal(err)
		}
		if got := waitingRuleSchedules(t, f.st, r.ID); len(got) != 5 || got[0].MinPaise != 3000 || got[0].MaxPaise != 6000 {
			t.Fatalf("fallback prices %+v", got)
		}
		bad := testRule(p)
		bad.Pricing = "median"
		if _, err := m.PreviewRecurringRule(ctx, 7, bad, false); err == nil {
			t.Fatal("accepted median pricing without a meal's percentages")
		}
		bad.Relative = map[string]store.RelativePrice{"lunch": {Low: 20, High: 10}}
		if _, err := m.PreviewRecurringRule(ctx, 7, bad, false); err == nil {
			t.Fatal("accepted opening below lowest")
		}
		cancel()
		synctest.Wait()
	})
}

func TestRelativeRangeRoundsToFiveRupees(t *testing.T) {
	for _, c := range []struct {
		median    float64
		rel       store.RelativePrice
		low, high int
	}{
		{40, store.RelativePrice{Low: -15, High: 25}, 35, 50},
		{47, store.RelativePrice{Low: 0, High: 0}, 45, 45},
		{2, store.RelativePrice{Low: -90, High: 0}, 1, 1},
	} {
		if lo, hi := relativeRange(c.median, c.rel); lo != c.low || hi != c.high {
			t.Errorf("relativeRange(%v, %+v) = %d-%d, want %d-%d", c.median, c.rel, lo, hi, c.low, c.high)
		}
	}
}
