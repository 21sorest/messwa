package dashboard

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"messwa/internal/accounts"
	"messwa/internal/config"
	"messwa/internal/ingest"
	"messwa/internal/mcpserver"
	"messwa/internal/store"
)

func TestInsights(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Groups = []string{"g1@g.us"}
	st, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	in := ingest.New(cfg, st)
	posted := time.Date(2026, 11, 3, 11, 0, 0, 0, cfg.Location()) // a Tuesday
	for _, p := range []ingest.GroupPost{
		{Chat: "g1@g.us", ID: "a", Sender: "911@s.whatsapp.net", PushName: "Asha", TS: posted, Text: "Sell bakul lunch @25"},
		{Chat: "g1@g.us", ID: "b", Sender: "922@s.whatsapp.net", TS: posted, Text: "Selling kadamba dinner"},
	} {
		if err := in.Post(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if err := in.Reaction(ctx, "g1@g.us", "a", "911@s.whatsapp.net", "", "✅", posted.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	now := posted.Add(4 * time.Hour) // lunch is over, dinner isn't
	rule := store.SoldRule{Emojis: cfg.Sold.Emojis, ClosedEmojis: cfg.Sold.ClosedEmojis}
	if err := st.SnapshotFacts(ctx, rule, now); err != nil {
		t.Fatal(err)
	}
	ms := mcpserver.New(cfg, st, nil)
	ms.SetClock(func() time.Time { return now })
	d := startDashboard(t, cfg, st, ms, NewHub())

	// Analytics are for admins: no session gets 401, a member 403.
	if res, err := http.Get(d.URL + "/api/insights"); err != nil || res.StatusCode != 401 {
		t.Fatalf("signed out: %v %v", res, err)
	}
	if res, err := http.Get(d.URL + "/insights.html"); err != nil || res.Request.URL.Path != "/login.html" {
		t.Fatalf("signed out page should go to sign-in: %v %v", res, err)
	}
	m := d.as(t, accounts.Member)
	for _, p := range []string{"/api/insights", "/insights.html"} {
		if res := m.get(t, p, nil); res.StatusCode != 403 {
			t.Fatalf("member %s: %d", p, res.StatusCode)
		}
	}

	c := d.as(t, accounts.Admin)
	var s insightsState
	if res := c.get(t, "/api/insights?weeks=4", &s); res.StatusCode != 200 {
		t.Fatalf("admin: %d", res.StatusCode)
	}
	if s.First != "2026-11-03" || s.From != "2026-11-03" || s.Today != "2026-11-03" {
		t.Fatalf("dates first %q from %q today %q", s.First, s.From, s.Today)
	}
	if len(s.Rows) != 1 || len(s.Sellers) != 1 {
		t.Fatalf("rows %v sellers %v: want only the ended bakul lunch", s.Rows, s.Sellers)
	}
	row := map[string]any{}
	for i, c := range s.Cols {
		row[c] = s.Rows[0][i]
	}
	if row["mess"] != "bakul" || row["meal"] != "lunch" || row["price"] != 25.0 || row["sold"] != float64(posted.Add(5*time.Minute).Unix()) {
		t.Fatalf("row %v", row)
	}
	if s.Sellers[0].Name != "Asha" || s.Sellers[0].Phone != "+911" {
		t.Fatalf("seller %+v", s.Sellers[0])
	}
	if res := c.get(t, "/insights.html", nil); res.StatusCode != 200 {
		t.Fatalf("admin page: %d", res.StatusCode)
	}
}
