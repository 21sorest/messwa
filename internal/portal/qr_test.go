package portal

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestQRRefreshAndMealChecks(t *testing.T) {
	var token, account, status atomic.Value
	token.Store("synthetic-token-one")
	account.Store("synthetic-account")
	status.Store("registered")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.Header.Get("Authorization") != "test-key" {
			t.Error("unexpected portal request")
		}
		switch r.URL.Path {
		case "/api/auth/me":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]string{"id": account.Load().(string), "token": token.Load().(string), "email": "not-returned@example.com"}})
		case "/api/registrations":
			row := map[string]any{"meal_date": "2026-11-04", "meal_type": "lunch", "meal_mess": "palash", "category": "registered", "cancelled_at": nil, "availed_at": nil}
			rows := []any{row}
			switch status.Load().(string) {
			case "cancelled":
				row["cancelled_at"] = "2026-11-04T10:00:00Z"
			case "availed":
				row["availed_at"] = "2026-11-04T13:00:00Z"
			case "guest":
				row["category"] = "guest"
			case "missing":
				rows = []any{}
			case "duplicate":
				rows = append(rows, row)
			}
			json.NewEncoder(w).Encode(map[string]any{"data": rows})
		default:
			t.Error("unapproved endpoint", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	t.Setenv("MESS_API_URL", srv.URL+"/api")
	c, err := New("test-key")
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = srv.Client().Transport
	ctx := context.Background()
	first, err := c.QRForMeal(ctx, "2026-11-04", "lunch", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(first.PNG)); err != nil {
		t.Fatal("invalid QR PNG", err)
	}
	if strings.Contains(first.AccountHash, "synthetic-account") {
		t.Fatal("raw account ID exposed")
	}
	token.Store("synthetic-token-two")
	second, err := c.QRForMeal(ctx, "2026-11-04", "lunch", first.AccountHash)
	if err != nil || bytes.Equal(first.PNG, second.PNG) || first.AccountHash != second.AccountHash {
		t.Fatal("token rotation did not refresh QR while retaining account binding", err)
	}
	account.Store("another-account")
	if _, err := c.QRForMeal(ctx, "2026-11-04", "lunch", first.AccountHash); err == nil {
		t.Fatal("changed account accepted")
	}
	account.Store("synthetic-account")
	for _, state := range []string{"cancelled", "availed", "guest", "missing", "duplicate"} {
		status.Store(state)
		if _, err := c.QRForMeal(ctx, "2026-11-04", "lunch", first.AccountHash); err == nil {
			t.Fatal("invalid meal accepted", state)
		}
	}
}

func TestQRBadProfiles(t *testing.T) {
	for _, body := range []string{`{"data":null}`, `{"data":{}}`, `{"data":{"id":"account"}}`, `{"data":{"id":"account","token":""}}`, `{"data":{"id":"account","token":5}}`, `{"data":{"id":"account","token":"` + strings.Repeat("x", 2049) + `"}}`} {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		t.Setenv("MESS_API_URL", srv.URL)
		c, err := New("test-key")
		if err != nil {
			t.Fatal(err)
		}
		c.http.Transport = srv.Client().Transport
		if _, err := c.QR(context.Background()); err == nil {
			t.Error("invalid profile accepted")
		}
		srv.Close()
	}
}
