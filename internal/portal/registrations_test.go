package portal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegistrationsContract(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/registrations" || r.URL.Query().Get("from") != "2026-11-03" || r.URL.Query().Get("to") != "2026-11-04" || r.Header.Get("Authorization") != "test-key" {
			t.Error("wrong registration request")
		}
		fmt.Fprint(w, `{"data":[{"meal_date":"2026-11-04","meal_type":"lunch","meal_mess":"kadamba-veg","category":"registered","cancelled_at":null,"availed_at":null,"user_id":"private-user","token":"private-token"},{"meal_date":"2026-11-03","meal_type":"dinner","meal_mess":"palash","category":"registered","cancelled_at":"2026-11-03T10:00:00Z","availed_at":null}]}`)
	}))
	defer srv.Close()
	t.Setenv("MESS_API_URL", srv.URL+"/api")
	c, err := New("test-key")
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = srv.Client().Transport
	rows, err := c.Registrations(context.Background(), "2026-11-03", "2026-11-04")
	if err != nil || len(rows) != 2 {
		t.Fatalf("read failed: %v", err)
	}
	if rows[0].Date != "2026-11-03" || rows[0].CancelledAt == nil || rows[1].CancelledAt != nil {
		t.Fatal("lost date order or cancellation state")
	}
	b, _ := json.Marshal(rows)
	if strings.Contains(string(b), "private-") || strings.Contains(string(b), "user_id") {
		t.Fatal("unneeded personal data exposed")
	}
}

func TestBadResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		ok     bool
	}{
		{"empty", 200, `{"data":[]}`, true}, {"null", 200, `{"data":null}`, true},
		{"missing data", 200, `{}`, false}, {"wrong shape", 200, `{"data":{}}`, false},
		{"invalid row", 200, `{"data":[{}]}`, false},
		{"out of range", 200, `{"data":[{"meal_date":"2027-01-01","meal_type":"lunch","meal_mess":"palash","category":"registered"}]}`, false},
		{"invalid date", 200, `{"data":[{"meal_date":"2026-11-99","meal_type":"lunch","meal_mess":"palash","category":"registered"}]}`, false},
		{"bad json", 200, `{`, false}, {"unauthorized", 401, `private error`, false},
		{"forbidden", 403, `private error`, false}, {"server error", 500, `private error`, false},
		{"too large", 200, strings.Repeat(" ", 2*1024*1024+1), false},
		{"redirect", 302, ``, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.Header().Set("Location", "/redirected")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()
			t.Setenv("MESS_API_URL", srv.URL)
			c, err := New("test-key")
			if err != nil {
				t.Fatal(err)
			}
			c.http.Transport = srv.Client().Transport
			_, err = c.Registrations(context.Background(), "2026-11-03", "2026-11-04")
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected error: %v", err)
			}
			if calls != 1 {
				t.Fatal("redirect followed or request repeated")
			}
			if err != nil && strings.Contains(err.Error(), "private error") {
				t.Fatal("remote error body leaked")
			}
		})
	}
}

func TestRegistrationConfigAndDates(t *testing.T) {
	if _, err := New(" "); err == nil {
		t.Fatal("missing key accepted")
	}
	for _, u := range []string{"http://example.com/api", "https://user:password@example.com/api", "https://example.com?key=x"} {
		t.Setenv("MESS_API_URL", u)
		if _, err := New("test-key"); err == nil {
			t.Fatal("unsafe URL accepted")
		}
	}
	for _, r := range [][2]string{{"", "2026-11-04"}, {"2026-11-04", "2026-11-03"}, {"2026-11-03", "2027-02-04"}, {"2026-11-31", "2026-12-01"}} {
		if ValidateRange(r[0], r[1]) == nil {
			t.Fatal("invalid range accepted")
		}
	}
	if err := ValidateRange("2026-11-03", "2027-01-03"); err != nil {
		t.Fatal(err)
	}
}

func TestMessDiet(t *testing.T) {
	for raw, want := range map[string][2]string{
		"kadamba-veg": {"kadamba", "veg"}, "Kadamba-NonVeg": {"kadamba", "nonveg"}, "bakul_non-veg": {"bakul", "nonveg"},
		"bakul-egg": {"bakul", "egg"}, "palash": {"palash", ""}, "": {"", ""},
	} {
		m, d := Registration{Mess: raw}.MessDiet()
		if m != want[0] || d != want[1] {
			t.Errorf("%q: got %s/%s, want %s/%s", raw, m, d, want[0], want[1])
		}
	}
}
