package portal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMealTimings(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/config/meal-timings" || r.URL.Query().Get("on") != "2026-11-15" {
			t.Errorf("request %s", r.URL)
		}
		if r.Header.Get("Authorization") != "" {
			t.Error("meal timings sent an API key")
		}
		w.Write([]byte(`{"data":{
			"kadamba-veg":[{"meal":"breakfast","start_time":"06:30:00","end_time":"10:30:00"},{"meal":"lunch","start_time":"11:30:00","end_time":"15:30:00"}],
			"lounge-veg":[{"meal":"lunch","start_time":"12:00:00","end_time":"16:00:00"}],
			"palash":[{"meal":"dinner","start_time":"18:30:00","end_time":"22:30:00"},{"meal":"supper","start_time":"23:00:00","end_time":"23:30:00"}]}}`))
	}))
	defer srv.Close()
	t.Setenv("MESS_API_URL", srv.URL+"/api")
	c, err := Public()
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = srv.Client().Transport
	got, err := c.MealTimings(context.Background(), "2026-11-15")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]MealWindow{"breakfast": {"06:30", "10:30"}, "lunch": {"11:30", "16:00"}, "dinner": {"18:30", "22:30"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %v, want %v", k, got[k], v)
		}
	}
}

func TestParseMealTimingsRejectsBadWindows(t *testing.T) {
	for _, body := range []string{`[]`, `{}`, `{"x":[{"meal":"lunch","start_time":"15:00:00","end_time":"11:00:00"}]}`,
		`{"x":[{"meal":"lunch","start_time":"noon","end_time":"15:00:00"}]}`} {
		if _, err := parseMealTimings([]byte(body)); err == nil {
			t.Errorf("parseMealTimings(%s) took it", body)
		}
	}
}
