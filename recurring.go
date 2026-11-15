package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"messwa/internal/config"
	"messwa/internal/market"
	"messwa/internal/meals"
	"messwa/internal/portal"
	"messwa/internal/store"
)

func (p proxyPortal) Registrations(ctx context.Context, from, to string) ([]portal.Registration, error) {
	real, err := p.real(ctx)
	if err != nil {
		return nil, err
	}
	reader, ok := real.(registrationReader)
	if !ok {
		return nil, errors.New("portal cannot list registrations")
	}
	return reader.Registrations(ctx, from, to)
}

func (m *sellManager) validateRule(r store.RecurringRule) error {
	if strings.TrimSpace(r.Name) == "" || len(r.Name) > 80 {
		return errors.New("give the rule a name, up to 80 characters")
	}
	from, e1 := time.ParseInLocation("2006-01-02", r.From, m.cfg.Location())
	through, e2 := time.ParseInLocation("2006-01-02", r.Through, m.cfg.Location())
	if e1 != nil || e2 != nil || through.Before(from) || through.After(from.AddDate(1, 0, 0)) {
		return errors.New("choose start and end dates no more than a year apart")
	}
	if len(r.Weekdays) == 0 || len(r.Weekdays) > 7 || len(r.Meals) == 0 || len(r.Meals) > len(m.cfg.Meals) {
		return errors.New("select weekdays and meals")
	}
	seen := map[int]bool{}
	for _, d := range r.Weekdays {
		if d < 0 || d > 6 || seen[d] {
			return errors.New("choose distinct weekdays")
		}
		seen[d] = true
	}
	names := map[string]bool{}
	for _, meal := range r.Meals {
		slot, err := findSlot(m.cfg, r.From, meal)
		if err != nil || meal != baseMeal(meal) || names[meal] {
			return errors.New("choose distinct configured meals")
		}
		names[meal] = true
		if !slot.Start.Add(-time.Duration(r.Before) * time.Minute).Before(slot.End) {
			return errors.New("posting time must be before each selected meal ends")
		}
	}
	if r.Min < 1 || r.Max < r.Min || r.Max > 10000 {
		return errors.New("use whole rupees from 1 to 10000, with opening at least minimum")
	}
	if r.Before < -90 || r.Before > 30 {
		return errors.New("posting time must be 30 minutes before to 90 minutes after serving starts")
	}
	switch r.Pricing {
	case "":
	case "median":
		for _, meal := range r.Meals {
			p, ok := r.Relative[meal]
			if !ok {
				return fmt.Errorf("set %s's price against its median", meal)
			}
			if p.Low < -90 || p.Low > 200 || p.High < p.Low || p.High > 300 {
				return fmt.Errorf("%s: lowest must be -90%% to +200%% of the median, and opening at least lowest and at most +300%%", meal)
			}
		}
	default:
		return errors.New("price meals at a fixed range or against their median")
	}
	return nil
}

// rulePrice is the range a rule sells one meal at, in paise. median is
// false when the meal used the fixed range: the rule is fixed-price, or
// the meal has no market median yet.
type rulePrice func(date, meal string) (minPaise, maxPaise int64, median bool)

// rulePricer prices r's meals. A median-priced meal uses the median ask
// for its own mess and diet, read as of its latest same-weekday serving
// that has happened, so next Wednesday's lunch goes by last Wednesday's.
func (m *sellManager) rulePricer(ctx context.Context, r store.RecurringRule, rows []portal.Registration) rulePrice {
	fixed := func(string, string) (int64, int64, bool) { return int64(r.Min) * 100, int64(r.Max) * 100, false }
	if r.Pricing != "median" || m.prices == nil {
		return fixed
	}
	loc := m.cfg.Location()
	today := time.Now().In(loc)
	ps, err := m.prices(ctx, today.Add(-market.Window-7*24*time.Hour).Format("2006-01-02"), today.Format("2006-01-02"))
	if err != nil {
		m.log.Warn("could not read market prices; recurring rule uses its fixed range", "rule", r.ID, "err", err)
		return fixed
	}
	regs := map[string]portal.Registration{}
	for _, row := range rows {
		regs[row.Date+"|"+row.Meal] = row
	}
	todayDate := today.Format("2006-01-02")
	return func(date, meal string) (int64, int64, bool) {
		rel, ok := r.Relative[meal]
		row, registered := regs[date+"|"+meal]
		if !ok || !registered {
			return fixed(date, meal)
		}
		mess, diet := row.MessDiet()
		if diet == "" && m.cfg.VegOnly(mess) {
			diet = "veg"
		}
		median, have := market.MedianAsk(ps, pastWeekday(date, todayDate, loc), baseMeal(meal), mess, diet)
		if !have {
			return fixed(date, meal)
		}
		lo, hi := relativeRange(median, rel)
		return int64(lo) * 100, int64(hi) * 100, true
	}
}

// pastWeekday steps date back a week at a time until it is not after today.
func pastWeekday(date, today string, loc *time.Location) string {
	d, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return date
	}
	for d.Format("2006-01-02") > today {
		d = d.AddDate(0, 0, -7)
	}
	return d.Format("2006-01-02")
}

// relativeRange applies rel to median, in whole rupees rounded to ₹5 as
// people post them.
func relativeRange(median float64, rel store.RelativePrice) (int, int) {
	at := func(pct int) int {
		v := int(math.Round(median*float64(100+pct)/100/5)) * 5
		return min(max(v, 1), 10000)
	}
	lo, hi := at(rel.Low), at(rel.High)
	return lo, max(hi, lo)
}

func ruleMatches(r store.RecurringRule, date, meal string, loc *time.Location) bool {
	if date < r.From || date > r.Through || !slices.Contains(r.Meals, meal) {
		return false
	}
	d, err := time.ParseInLocation("2006-01-02", date, loc)
	return err == nil && slices.Contains(r.Weekdays, int(d.Weekday()))
}

func (m *sellManager) ruleRows(ctx context.Context, user int64) ([]portal.Registration, error) {
	p, err := m.portalFor(ctx, user)
	if err != nil {
		return nil, err
	}
	reader, ok := p.(registrationReader)
	if !ok {
		return nil, errors.New("portal cannot list registrations")
	}
	now := time.Now().In(m.cfg.Location())
	return reader.Registrations(ctx, now.Format("2006-01-02"), now.AddDate(0, 0, 27).Format("2006-01-02"))
}

// ruleAt is when rule r posts slot's offer: r.Before minutes before serving,
// but not while the meal before is still served.
func ruleAt(cfg *config.Config, r store.RecurringRule, slot meals.Slot) time.Time {
	at := slot.Start.Add(-time.Duration(r.Before) * time.Minute)
	if opens := listOpens(cfg, slot); at.Before(opens) {
		return opens
	}
	return at
}

// ruleCandidates only returns confirmed, unused, unambiguous registrations.
// Manual schedules, prior tasks, and completed/failed occurrences take priority.
func (m *sellManager) ruleCandidates(ctx context.Context, r store.RecurringRule, rows []portal.Registration, price rulePrice) ([]store.SellSchedule, int, int, error) {
	kept, err := m.st.KeptMeals(ctx, r.UserID)
	if err != nil {
		return nil, 0, 0, err
	}
	schedules, err := m.st.SellSchedules(ctx, time.Time{})
	if err != nil {
		return nil, 0, 0, err
	}
	blocked := map[string]bool{}
	for _, sc := range schedules {
		if sc.State == store.ScheduleWaiting || sc.State == store.ScheduleStarting || (sc.RuleID > 0 && sc.State != store.ScheduleCancelled) {
			blocked[sc.Date+"|"+sc.Meal] = true
		}
	}
	tasks, err := m.st.SellTasks(ctx, time.Time{})
	if err != nil {
		return nil, 0, 0, err
	}
	for _, t := range tasks {
		blocked[t.Date+"|"+t.Meal] = true
	}
	deliveries, err := m.st.Deliveries(ctx)
	if err != nil {
		return nil, 0, 0, err
	}
	for _, d := range deliveries {
		blocked[d.Date+"|"+d.Meal] = true
	}
	registrations := map[string][]portal.Registration{}
	for _, row := range rows {
		registrations[row.Date+"|"+row.Meal] = append(registrations[row.Date+"|"+row.Meal], row)
	}
	out := []store.SellSchedule{}
	exceptions, existing := 0, 0
	now := time.Now()
	today := now.In(m.cfg.Location()).Format("2006-01-02")
	last := now.In(m.cfg.Location()).AddDate(0, 0, 27).Format("2006-01-02")
	for _, regs := range registrations {
		row := regs[0]
		if len(regs) != 1 || row.Category != "registered" || row.Cancelled() || row.Availed() || row.Date < today || row.Date > last || !ruleMatches(r, row.Date, row.Meal, m.cfg.Location()) || m.cfg.Live().HiddenMeal(row.Meal) {
			continue
		}
		slot, err := findSlot(m.cfg, row.Date, row.Meal)
		if err != nil {
			return nil, 0, 0, err
		}
		if !now.Before(slot.End) {
			continue
		}
		key := row.Date + "|" + row.Meal
		if kept[key] {
			exceptions++
			continue
		}
		if blocked[key] {
			existing++
			continue
		}
		lo, hi, _ := price(row.Date, row.Meal)
		out = append(out, store.SellSchedule{RuleID: r.ID, RuleRevision: r.Revision, UserID: r.UserID, Date: row.Date, Meal: row.Meal, MinPaise: lo, MaxPaise: hi, StartsAt: ruleAt(m.cfg, r, slot), EndsAt: slot.End, CreatedAt: now, Autopilot: r.Autopilot})
	}
	slices.SortFunc(out, func(a, b store.SellSchedule) int { return a.StartsAt.Compare(b.StartsAt) })
	return out, exceptions, existing, nil
}

func (m *sellManager) PreviewRecurringRule(ctx context.Context, user int64, r store.RecurringRule, apply bool) (store.RecurringPreview, error) {
	r.UserID = user
	var out store.RecurringPreview
	if err := m.validateRule(r); err != nil {
		return out, err
	}
	if r.ID > 0 {
		if err := m.ownedRule(ctx, user, r.ID, r.Revision); err != nil {
			return out, err
		}
	}
	schedules, err := m.st.SellSchedules(ctx, time.Time{})
	if err != nil {
		return out, err
	}
	for _, sc := range schedules {
		if sc.RuleID != r.ID || r.ID == 0 || sc.State != store.ScheduleWaiting {
			continue
		}
		if sc.Overridden {
			out.PreservedOverrides++
			continue
		}
		out.Waiting++
		if apply && r.Enabled && ruleMatches(r, sc.Date, sc.Meal, m.cfg.Location()) {
			slot, e := findSlot(m.cfg, sc.Date, sc.Meal)
			if e == nil && !ruleAt(m.cfg, r, slot).After(time.Now()) {
				out.StartsNow++
			}
		}
	}
	if !r.Enabled {
		return out, nil
	}
	rows, err := m.ruleRows(ctx, user)
	if err != nil {
		return out, err
	}
	price := m.rulePricer(ctx, r, rows)
	candidates, kept, existing, err := m.ruleCandidates(ctx, r, rows, price)
	if err != nil {
		return out, err
	}
	out.New, out.Kept, out.Existing = len(candidates), kept, existing
	for i, sc := range candidates {
		if !sc.StartsAt.After(time.Now()) {
			out.StartsNow++
		}
		if _, _, median := price(sc.Date, sc.Meal); r.Pricing == "median" && !median {
			out.Fallback++
		}
		lo, hi := int(sc.MinPaise/100), int(sc.MaxPaise/100)
		if i == 0 || lo < out.Lowest {
			out.Lowest = lo
		}
		if hi > out.Highest {
			out.Highest = hi
		}
	}
	return out, nil
}

func (m *sellManager) ownedRule(ctx context.Context, user, id, revision int64) error {
	rules, err := m.st.RecurringRules(ctx)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if r.ID == id && r.UserID == user && r.Revision == revision {
			return nil
		}
	}
	return errors.New("rule changed, was deleted, or belongs to another account; reload")
}

func (m *sellManager) SaveRecurringRule(ctx context.Context, user int64, r store.RecurringRule, apply bool) (store.RecurringRule, error) {
	m.rulesMu.Lock()
	defer m.rulesMu.Unlock()
	r.UserID = user
	r.Name = strings.TrimSpace(r.Name)
	if err := m.validateRule(r); err != nil {
		return r, err
	}
	if r.Enabled && !m.cfg.Live().MCP.SellTasks {
		return r, errors.New("enable selling before enabling a recurring rule")
	}
	var err error
	r, err = m.st.SaveRecurringRule(ctx, r)
	if err != nil {
		return r, err
	}
	var rows []portal.Registration
	var rowsErr error
	if r.Enabled || (apply && r.Pricing == "median") {
		rows, rowsErr = m.ruleRows(ctx, user)
	}
	price := m.rulePricer(ctx, r, rows)
	if apply {
		schedules, err := m.st.SellSchedules(ctx, time.Time{})
		if err != nil {
			return r, err
		}
		for _, sc := range schedules {
			if sc.RuleID != r.ID || sc.State != store.ScheduleWaiting || sc.Overridden {
				continue
			}
			sc.RuleRevision = r.Revision
			cancel := !r.Enabled || !ruleMatches(r, sc.Date, sc.Meal, m.cfg.Location())
			if !cancel {
				slot, e := findSlot(m.cfg, sc.Date, sc.Meal)
				if e != nil {
					return r, e
				}
				sc.MinPaise, sc.MaxPaise, _ = price(sc.Date, sc.Meal)
				sc.Autopilot, sc.StartsAt = r.Autopilot, ruleAt(m.cfg, r, slot)
			}
			ok, e := m.st.ApplyRecurringSchedule(ctx, sc, cancel)
			if e != nil {
				return r, e
			}
			if ok {
				m.disarm(sc.ID)
				if !cancel {
					m.arm(sc)
				}
			}
		}
	}
	if r.Enabled {
		if rowsErr != nil {
			r.LastError = rowsErr.Error()
		} else {
			r.LastError = m.generateRule(ctx, r, rows)
		}
		_ = m.st.RecurringRuleError(ctx, r.ID, r.Revision, r.LastError)
	}
	m.changed()
	return r, nil
}

func (m *sellManager) generateRule(ctx context.Context, r store.RecurringRule, rows []portal.Registration) string {
	price := m.rulePricer(ctx, r, rows)
	if r.Pricing == "median" {
		if err := m.repriceRule(ctx, r, price); err != nil {
			return err.Error()
		}
	}
	candidates, _, _, err := m.ruleCandidates(ctx, r, rows, price)
	if err != nil {
		return err.Error()
	}
	for _, sc := range candidates {
		if !m.cfg.Live().MCP.SellTasks {
			return ""
		}
		if ctx.Err() != nil {
			return "Rule refresh interrupted; will retry"
		}
		id, err := m.st.CreateSellSchedule(ctx, sc)
		if err != nil {
			// A manual save, cancellation or another rule may have won the slot.
			remaining, _, _, checkErr := m.ruleCandidates(ctx, r, rows, price)
			if checkErr == nil && !slices.ContainsFunc(remaining, func(x store.SellSchedule) bool { return x.Date == sc.Date && x.Meal == sc.Meal }) {
				continue
			}
			return fmt.Sprintf("%s %s: %s", sc.Date, sc.Meal, err)
		}
		sc.ID = id
		m.arm(sc)
	}
	return ""
}

// repriceRule moves r's waiting meals to the current median. Meals edited
// by hand, meals waiting on a retry and meals from an older revision of
// the rule keep their prices.
func (m *sellManager) repriceRule(ctx context.Context, r store.RecurringRule, price rulePrice) error {
	schedules, err := m.st.SellSchedules(ctx, time.Time{})
	if err != nil {
		return err
	}
	for _, sc := range schedules {
		if sc.RuleID != r.ID || sc.RuleRevision != r.Revision || sc.State != store.ScheduleWaiting || sc.Overridden || sc.Attempts > 0 || !sc.RetryAt.IsZero() {
			continue
		}
		lo, hi, median := price(sc.Date, sc.Meal)
		if !median || (lo == sc.MinPaise && hi == sc.MaxPaise) {
			continue
		}
		sc.MinPaise, sc.MaxPaise = lo, hi
		ok, err := m.st.ApplyRecurringSchedule(ctx, sc, false)
		if err != nil {
			return err
		}
		if ok {
			m.disarm(sc.ID)
			m.arm(sc)
		}
	}
	return nil
}

func (m *sellManager) DeleteRecurringRule(ctx context.Context, user, id, revision int64, cancel bool) error {
	m.rulesMu.Lock()
	defer m.rulesMu.Unlock()
	ids, err := m.st.DeleteRecurringRule(ctx, user, id, revision, cancel)
	if err != nil {
		return err
	}
	for _, id := range ids {
		m.disarm(id)
	}
	m.changed()
	return nil
}

func (m *sellManager) AllowRecurringMeal(ctx context.Context, user int64, date, meal string) error {
	slot, err := findSlot(m.cfg, date, meal)
	if err != nil {
		return err
	}
	if !time.Now().Before(slot.End) {
		return errors.New("this meal has ended")
	}
	if err := m.st.AllowRecurringMeal(ctx, user, date, meal); err != nil {
		return err
	}
	m.refreshRules(ctx)
	m.changed()
	return nil
}

// Run inside the account runtime. No external scheduler or second WhatsApp
// connection is needed; ordinary saved schedules keep execution/retry ownership.
func (m *sellManager) startRecurring() {
	m.rulesOnce.Do(func() {
		go func() {
			m.refreshRules(m.base)
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-m.base.Done():
					return
				case <-ticker.C:
					m.refreshRules(m.base)
				}
			}
		}()
	})
}

func (m *sellManager) refreshRules(ctx context.Context) {
	m.rulesMu.Lock()
	defer m.rulesMu.Unlock()
	if ctx.Err() != nil || !m.cfg.Live().MCP.SellTasks {
		return
	}
	rules, err := m.st.RecurringRules(ctx)
	if err != nil {
		m.log.Error("read recurring rules", "err", err)
		return
	}
	type input struct {
		rows []portal.Registration
		err  error
	}
	byUser := map[int64]input{}
	today := time.Now().In(m.cfg.Location()).Format("2006-01-02")
	last := time.Now().In(m.cfg.Location()).AddDate(0, 0, 27).Format("2006-01-02")
	for _, r := range rules {
		if !r.Enabled || r.Through < today || r.From > last {
			continue
		}
		if _, ok := byUser[r.UserID]; !ok {
			rows, err := m.ruleRows(ctx, r.UserID)
			byUser[r.UserID] = input{rows, err}
		}
		in := byUser[r.UserID]
		detail := ""
		if in.err != nil {
			detail = in.err.Error()
		} else {
			detail = m.generateRule(ctx, r, in.rows)
		}
		if err := m.st.RecurringRuleError(ctx, r.ID, r.Revision, detail); err != nil {
			m.log.Error("save recurring rule result", "err", err)
		}
	}
	m.changed()
}
