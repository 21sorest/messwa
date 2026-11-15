package dashboard

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name, remote string
		xff          []string
		want         string
	}{
		{"direct", "203.0.113.5:4000", nil, "203.0.113.5"},
		{"direct ignores forwarded header", "203.0.113.5:4000", []string{"198.51.100.1"}, "203.0.113.5"},
		{"proxy without header", "127.0.0.1:4000", nil, "127.0.0.1"},
		{"proxy", "127.0.0.1:4000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"proxy over IPv6 loopback", "[::1]:4000", []string{"198.51.100.1"}, "198.51.100.1"},
		{"forged entries before the proxy's", "127.0.0.1:4000", []string{"10.0.0.1, 198.51.100.1"}, "198.51.100.1"},
		{"forged header line before the proxy's", "127.0.0.1:4000", []string{"10.0.0.1", "198.51.100.1"}, "198.51.100.1"},
		{"garbage", "127.0.0.1:4000", []string{"not-an-ip"}, "127.0.0.1"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/api/login", nil)
		r.RemoteAddr = c.remote
		for _, v := range c.xff {
			r.Header.Add("X-Forwarded-For", v)
		}
		if got := clientIP(r); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestViaHTTPS(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "127.0.0.1:4000"
	if viaHTTPS(r) {
		t.Error("plain loopback request counted as HTTPS")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !viaHTTPS(r) {
		t.Error("proxied HTTPS request not counted as HTTPS")
	}
	r.RemoteAddr = "203.0.113.5:4000"
	if viaHTTPS(r) {
		t.Error("believed X-Forwarded-Proto from a non-local client")
	}
	r.Header.Del("X-Forwarded-Proto")
	r.TLS = &tls.ConnectionState{}
	if !viaHTTPS(r) {
		t.Error("direct TLS request not counted as HTTPS")
	}
}
