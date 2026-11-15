package portal

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPortalOutagesRemainRetryableWithoutExposingTransportDetails(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Setenv("MESS_API_URL", srv.URL+"/api")
	c, err := New("private-test-key")
	if err != nil {
		t.Fatal(err)
	}
	c.http.Transport = srv.Client().Transport
	srv.Close()
	_, err = c.Registrations(t.Context(), "2026-11-16", "2026-11-16")
	var temporary interface{ Temporary() bool }
	if !errors.As(err, &temporary) || !temporary.Temporary() {
		t.Fatalf("transport error not retryable: %v", err)
	}
	if strings.Contains(err.Error(), "private-test-key") || strings.Contains(err.Error(), srv.URL) {
		t.Fatalf("transport details exposed: %v", err)
	}
}

func TestPortalHTTPRetryClassification(t *testing.T) {
	for _, tc := range []struct {
		status int
		retry  bool
	}{{429, true}, {503, true}, {401, false}, {403, false}, {404, false}} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) }))
			defer srv.Close()
			t.Setenv("MESS_API_URL", srv.URL+"/api")
			c, err := New("test-key")
			if err != nil {
				t.Fatal(err)
			}
			c.http.Transport = srv.Client().Transport
			_, err = c.Registrations(t.Context(), "2026-11-16", "2026-11-16")
			var temporary interface{ Temporary() bool }
			retry := errors.As(err, &temporary) && temporary.Temporary()
			if err == nil || retry != tc.retry {
				t.Fatalf("status %d: error %v retry=%v", tc.status, err, retry)
			}
		})
	}
}
