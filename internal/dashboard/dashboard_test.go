package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"messwa/internal/config"
	"messwa/internal/ingest"
	"messwa/internal/mcpserver"
	"messwa/internal/store"
)

func TestHandler(t *testing.T) {
	cfg := config.Default()
	cfg.Groups = []string{"g1@g.us"}
	st, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	in := ingest.New(cfg, st)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, cfg.Location())
	if err := in.Post(context.Background(), ingest.GroupPost{Chat: "g1@g.us", ID: "a", Sender: "911@s.whatsapp.net", TS: now, Text: "Sell palash lunch @25 <script>"}); err != nil {
		t.Fatal(err)
	}
	ms := mcpserver.New(cfg, st, nil)
	ms.SetClock(func() time.Time { return now.Add(time.Minute) })
	srv := httptest.NewServer(Handler(cfg, ms))
	defer srv.Close()

	res, err := http.Get(srv.URL + "/api/state")
	if err != nil {
		t.Fatal(err)
	}
	var s state
	if err := json.NewDecoder(res.Body).Decode(&s); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(s.Listings) != 1 || s.Listings[0].Mess != "palash" || s.Listings[0].Diet != "veg" || *s.Listings[0].Price != 25 {
		t.Fatalf("listings: %+v", s.Listings)
	}
	if s.Current != nil || s.Next == nil || s.Next.Meal != "lunch" {
		t.Errorf("schedule: current %+v next %+v", s.Current, s.Next)
	}
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "script-src 'self'") {
		t.Error("missing CSP")
	}

	res, err = http.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 || !strings.HasPrefix(res.Header.Get("Content-Type"), "text/html") {
		t.Errorf("index: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	res, err = http.Post(srv.URL+"/api/state", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST got %d, want 405", res.StatusCode)
	}
}
