package dashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"messwa/internal/accounts"
	"messwa/internal/config"
	"messwa/internal/mcpserver"
	"messwa/internal/store"
)

const testSecret = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

// testDash is a dashboard on a test server with its account service.
type testDash struct {
	*httptest.Server
	acc *accounts.Accounts
}

// startDashboard serves Handler for tests. Sign in with as.
func startDashboard(t *testing.T, cfg *config.Config, st *store.Store, ms *mcpserver.Server, hub *Hub) *testDash {
	t.Helper()
	accounts.CheapHashing()
	acc, err := accounts.New(st, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(Handler(cfg, ms, hub, acc))
	t.Cleanup(srv.Close)
	return &testDash{srv, acc}
}

// testClient is a browser signed in to a testDash.
type testClient struct {
	*http.Client
	d    *testDash
	user *accounts.User
	csrf string
}

// as creates a user with role and returns a client signed in as them.
func (d *testDash) as(t *testing.T, role accounts.Role) *testClient {
	t.Helper()
	name := string(role) + strings.ReplaceAll(t.Name(), "/", "_")
	name = name[:min(len(name), 32)]
	u, err := d.acc.ByName(context.Background(), name)
	if err != nil {
		if u, err = d.acc.Add(context.Background(), name, "test password", role); err != nil {
			t.Fatal(err)
		}
	}
	jar, _ := cookiejar.New(nil)
	c := &testClient{Client: &http.Client{Jar: jar}, d: d, user: u}
	res := c.send(t, "POST", "/api/login", map[string]string{"name": name, "password": "test password"})
	var me struct{ CSRF string }
	json.NewDecoder(res.Body).Decode(&me)
	res.Body.Close()
	if res.StatusCode != 200 || me.CSRF == "" {
		t.Fatalf("sign in as %s: %d", name, res.StatusCode)
	}
	c.csrf = me.CSRF
	return c
}

// get fetches path and decodes JSON into out when it's not nil.
func (c *testClient) get(t *testing.T, path string, out any) *http.Response {
	t.Helper()
	res, err := c.Get(c.d.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if out != nil && res.StatusCode == 200 {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			t.Fatal(err)
		}
	}
	return res
}

// send makes a same-origin request with the CSRF token, like the pages do.
func (c *testClient) send(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		r = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, c.d.URL+path, r)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", c.d.URL)
	if c.csrf != "" {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	res, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestSignIn(t *testing.T) {
	cfg := config.Default()
	st := newTestStore(t)
	d := startDashboard(t, cfg, st, mcpserver.New(cfg, st, nil), NewHub())
	anon := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

	// Before any admin exists the sign-in page says how to make one.
	var setup struct {
		NeedsAdmin bool `json:"needs_admin"`
	}
	res, _ := anon.Get(d.URL + "/api/setup")
	json.NewDecoder(res.Body).Decode(&setup)
	res.Body.Close()
	if !setup.NeedsAdmin {
		t.Error("setup should ask for an admin")
	}

	// Without a session, pages go to sign-in and the API refuses.
	for path, want := range map[string]string{"/": "/login.html", "/sales.html": "/login.html?next=sales.html", "/account.html": "/login.html?next=account.html"} {
		res, err := anon.Get(d.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusSeeOther || res.Header.Get("Location") != want {
			t.Errorf("%s: %d to %q, want %s", path, res.StatusCode, res.Header.Get("Location"), want)
		}
	}
	for _, path := range []string{"/api/state", "/api/sales", "/api/listing?id=1", "/api/events", "/api/me", "/api/mcp-tokens"} {
		res, _ := anon.Get(d.URL + path)
		res.Body.Close()
		if res.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s without a session: %d", path, res.StatusCode)
		}
	}
	// The sign-in page and its assets are public, under the same CSP.
	for _, path := range []string{"/login.html", "/login.js", "/common.js", "/style.css", "/account.css"} {
		res, _ := anon.Get(d.URL + path)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
		if !strings.Contains(res.Header.Get("Content-Security-Policy"), "script-src 'self'") {
			t.Errorf("%s has no CSP", path)
		}
	}

	admin := d.as(t, accounts.Admin)
	res = admin.get(t, "/", nil)
	if res.StatusCode != 200 || res.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("signed-in board: %d %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}

	// The cookie can't be read by scripts or sent cross-site.
	res = (&testClient{Client: &http.Client{}, d: d}).send(t, "POST", "/api/login", map[string]string{"name": admin.user.Name, "password": "test password"})
	res.Body.Close()
	sc := res.Header.Get("Set-Cookie")
	if !strings.Contains(sc, "HttpOnly") || !strings.Contains(sc, "SameSite=Strict") || !strings.Contains(sc, cookieName+"=") {
		t.Errorf("cookie: %s", sc)
	}

	// Wrong password.
	res = (&testClient{Client: &http.Client{}, d: d}).send(t, "POST", "/api/login", map[string]string{"name": admin.user.Name, "password": "nope"})
	res.Body.Close()
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong password: %d", res.StatusCode)
	}

	// Signing out ends the session.
	res = admin.send(t, "POST", "/api/logout", nil)
	res.Body.Close()
	if res := admin.get(t, "/api/me", nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("after sign-out: %d", res.StatusCode)
	}
}

func TestCSRF(t *testing.T) {
	cfg := config.Default()
	st := newTestStore(t)
	d := startDashboard(t, cfg, st, mcpserver.New(cfg, st, nil), NewHub())
	c := d.as(t, accounts.Member)

	csrf := c.csrf
	c.csrf = ""
	res := c.send(t, "POST", "/api/mcp-tokens", map[string]string{"label": "x"})
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("no CSRF token: %d", res.StatusCode)
	}
	c.csrf = csrf

	req, _ := http.NewRequest("POST", d.URL+"/api/mcp-tokens", strings.NewReader(`{"label":"x"}`))
	req.Header.Set("Origin", "http://evil.example")
	req.Header.Set("X-CSRF-Token", csrf)
	res, _ = c.Do(req)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("cross-site origin: %d", res.StatusCode)
	}

	// Sign-in itself needs a same-site Origin too.
	req, _ = http.NewRequest("POST", d.URL+"/api/login", strings.NewReader(`{"name":"a","password":"b"}`))
	res, _ = http.DefaultClient.Do(req)
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Errorf("login without Origin: %d", res.StatusCode)
	}

	res = c.send(t, "POST", "/api/mcp-tokens", map[string]string{"label": "x"})
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Errorf("with CSRF token: %d", res.StatusCode)
	}
}

func TestRequireRole(t *testing.T) {
	cfg := config.Default()
	st := newTestStore(t)
	d := startDashboard(t, cfg, st, mcpserver.New(cfg, st, nil), NewHub())
	// The same wrapping PR #8 uses for /api/insights.
	h := withSession(d.acc, requireRole(accounts.Admin, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { io.WriteString(w, "secret") })))
	srv := httptest.NewServer(h)
	defer srv.Close()
	for role, want := range map[accounts.Role]int{accounts.Admin: 200, accounts.Member: 403} {
		c := d.as(t, role)
		req, _ := http.NewRequest("GET", srv.URL, nil)
		for _, ck := range c.Jar.Cookies(mustURL(d.URL)) {
			req.AddCookie(ck)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != want {
			t.Errorf("%s: %d, want %d", role, res.StatusCode, want)
		}
	}
	res, _ := http.Get(srv.URL)
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Errorf("anonymous: %d", res.StatusCode)
	}
}

func TestAccountAPI(t *testing.T) {
	cfg := config.Default()
	st := newTestStore(t)
	d := startDashboard(t, cfg, st, mcpserver.New(cfg, st, nil), NewHub())
	c := d.as(t, accounts.Member)

	res := c.send(t, "PUT", "/api/mess-key", map[string]string{"key": "very-secret-mess-key"})
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("save key: %d", res.StatusCode)
	}
	res, _ = c.Get(d.URL + "/api/me")
	body, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if bytes.Contains(body, []byte("very-secret")) || !bytes.Contains(body, []byte(`"mess_key_set":true`)) {
		t.Errorf("/api/me: %s", body)
	}

	// A token is shown once, listed by prefix only, and revocable.
	res = c.send(t, "POST", "/api/mcp-tokens", map[string]string{"label": "laptop"})
	var created struct {
		ID    int64  `json:"id"`
		Token string `json:"token"`
	}
	json.NewDecoder(res.Body).Decode(&created)
	res.Body.Close()
	if !strings.HasPrefix(created.Token, "mwa_") {
		t.Fatalf("token: %+v", created)
	}
	res, _ = c.Get(d.URL + "/api/mcp-tokens")
	body, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if bytes.Contains(body, []byte(created.Token)) || !bytes.Contains(body, []byte("laptop")) {
		t.Errorf("token list: %s", body)
	}
	other := d.as(t, accounts.Admin)
	res = other.send(t, "DELETE", "/api/mcp-tokens/"+itoa(created.ID), nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Errorf("revoked someone else's token: %d", res.StatusCode)
	}
	res = c.send(t, "DELETE", "/api/mcp-tokens/"+itoa(created.ID), nil)
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("revoke: %d", res.StatusCode)
	}

	// Changing the password keeps this session and checks the old one.
	res = c.send(t, "POST", "/api/password", map[string]string{"current": "wrong", "new": "a new long password"})
	res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("wrong current password: %d", res.StatusCode)
	}
	res = c.send(t, "POST", "/api/password", map[string]string{"current": "test password", "new": "a new long password"})
	res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Errorf("change password: %d", res.StatusCode)
	}
	if res := c.get(t, "/api/me", nil); res.StatusCode != 200 {
		t.Errorf("session after password change: %d", res.StatusCode)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

func mustURL(s string) *url.URL {
	u, err := url.Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}
