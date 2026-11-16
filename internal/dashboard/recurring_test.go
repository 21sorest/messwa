package dashboard

import (
	"context"
	"encoding/json"
	"messwa/internal/accounts"
	"messwa/internal/config"
	"messwa/internal/mcpserver"
	"messwa/internal/store"
	"net/http"
	"testing"
)

type recurringOperator struct {
	dashboardOperator
	user     int64
	received store.RecurringRule
	apply    bool
}

func (o *recurringOperator) PreviewRecurringRule(_ context.Context, user int64, r store.RecurringRule, apply bool) (store.RecurringPreview, error) {
	o.user, o.received, o.apply = user, r, apply
	return store.RecurringPreview{New: 5}, nil
}
func (o *recurringOperator) SaveRecurringRule(_ context.Context, user int64, r store.RecurringRule, apply bool) (store.RecurringRule, error) {
	o.user, o.received, o.apply = user, r, apply
	return r, nil
}
func (o *recurringOperator) DeleteRecurringRule(_ context.Context, user, id, revision int64, cancel bool) error {
	o.user = user
	o.received.ID, o.received.Revision = id, revision
	return nil
}
func (o *recurringOperator) AllowRecurringMeal(_ context.Context, user int64, date, meal string) error {
	o.user = user
	return nil
}

func TestRecurringAPIAuthorizationAndAccountScope(t *testing.T) {
	cfg := config.Default()
	cfg.MCP.SellTasks = true
	st := newTestStore(t)
	srv := mcpserver.New(cfg, st, nil)
	op := &recurringOperator{}
	srv.SetSellTasks(op)
	d := startDashboard(t, cfg, st, srv, NewHub())
	admin, member := d.as(t, accounts.Admin), d.as(t, accounts.Member)
	in := map[string]any{"action": "preview", "apply_waiting": true, "rule": map[string]any{"name": "Lunches", "from": "2026-11-16", "through": "2026-11-20", "weekdays": []int{1, 2, 3, 4, 5}, "meals": []string{"lunch"}, "min": 30, "max": 60, "before": 15, "enabled": true}}
	if res := member.send(t, "POST", "/api/recurring-rules", in); res.StatusCode != 403 {
		t.Fatalf("member write %d", res.StatusCode)
	}
	csrf := admin.csrf
	admin.csrf = ""
	if res := admin.send(t, "POST", "/api/recurring-rules", in); res.StatusCode != 403 {
		t.Fatalf("missing CSRF %d", res.StatusCode)
	}
	admin.csrf = csrf
	for _, action := range []string{"preview", "save", "delete", "allow"} {
		in["action"] = action
		res := admin.send(t, "POST", "/api/recurring-rules", in)
		if res.StatusCode != http.StatusOK {
			t.Fatalf("%s status %d", action, res.StatusCode)
		}
		res.Body.Close()
		if op.user != admin.user.ID {
			t.Fatal("rule operation did not use authenticated account")
		}
	}
	if !op.apply || op.received.Min != 30 {
		t.Fatal("lost rule settings")
	}
	if _, err := st.SaveRecurringRule(t.Context(), store.RecurringRule{UserID: member.user.ID, Name: "Private member rule", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	res, err := admin.Get(d.URL + "/api/recurring-rules")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var rules []store.RecurringRule
	if err := json.NewDecoder(res.Body).Decode(&rules); err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Fatal("another user's rules disclosed")
	}
}
