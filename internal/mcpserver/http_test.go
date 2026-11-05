package mcpserver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"messwa/internal/accounts"
	"messwa/internal/config"
	"messwa/internal/portal"
	"messwa/internal/store"
)

// fakePortal records whose key a portal tool read with.
type fakePortal struct {
	user  int64
	calls *[]int64
}

func (f fakePortal) Registrations(context.Context, string, string) ([]portal.Registration, error) {
	*f.calls = append(*f.calls, f.user)
	return []portal.Registration{}, nil
}

func (f fakePortal) QR(context.Context) (portal.CurrentQR, error) {
	return portal.CurrentQR{}, errors.New("not in tests")
}

type headerRT struct{ auth string }

func (h headerRT) RoundTrip(r *http.Request) (*http.Response, error) {
	if h.auth != "" {
		r = r.Clone(r.Context())
		r.Header.Set("Authorization", h.auth)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func TestHTTPAuth(t *testing.T) {
	ctx := context.Background()
	t.Setenv(accounts.LegacyKeyEnv, "")
	cfg := config.Default()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	accounts.CheapHashing()
	acc, _ := accounts.New(st, "")
	admin, _ := acc.Add(ctx, "boss", "long enough pw", accounts.Admin)
	member, _ := acc.Add(ctx, "asha", "long enough pw", accounts.Member)
	memberTok, _, _ := acc.CreateToken(ctx, member.ID, "")
	adminTok, _, _ := acc.CreateToken(ctx, admin.ID, "")
	revokedTok, rt, _ := acc.CreateToken(ctx, member.ID, "")
	acc.RevokeToken(ctx, 0, rt.ID)

	s := New(cfg, st, nil)
	s.SetAccounts(acc)
	var calls []int64
	s.portalFor = func(_ context.Context, id int64) (portalReader, error) { return fakePortal{id, &calls}, nil }

	serve := func(legacy string, anonymous bool) *httptest.Server {
		h, err := s.HTTPHandler(legacy, anonymous)
		if err != nil {
			t.Fatal(err)
		}
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		return srv
	}
	connect := func(srv *httptest.Server, auth string) (*mcp.ClientSession, error) {
		tr := &mcp.StreamableClientTransport{Endpoint: srv.URL, HTTPClient: &http.Client{Transport: headerRT{auth}}, MaxRetries: -1, DisableStandaloneSSE: true}
		return mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, tr, nil)
	}
	call := func(sess *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	regs := map[string]any{"from": "2026-11-03", "to": "2026-11-04"}
	text := func(r *mcp.CallToolResult) string {
		if len(r.Content) == 0 {
			return ""
		}
		tc, _ := r.Content[0].(*mcp.TextContent)
		if tc == nil {
			return ""
		}
		return tc.Text
	}

	open := serve("shared-secret", true)
	for _, tc := range []struct {
		name, auth string
		user       int64
		dms        bool
	}{
		{"member token", "Bearer " + memberTok, member.ID, false},
		{"admin token", "Bearer " + adminTok, admin.ID, true},
		{"legacy shared token", "Bearer shared-secret", admin.ID, true},
		{"tokenless loopback", "", admin.ID, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sess, err := connect(open, tc.auth)
			if err != nil {
				t.Fatal(err)
			}
			defer sess.Close()
			calls = nil
			if r := call(sess, "my_registrations", regs); r.IsError || len(calls) != 1 || calls[0] != tc.user {
				t.Errorf("my_registrations read with %v (error %v %q), want user %d", calls, r.IsError, text(r), tc.user)
			}
			r := call(sess, "list_recent_chats", nil)
			if r.IsError == tc.dms {
				t.Errorf("list_recent_chats error=%v %q, want allowed=%v", r.IsError, text(r), tc.dms)
			}
			if !tc.dms && !strings.Contains(text(r), "admins") {
				t.Errorf("refusal: %q", text(r))
			}
		})
	}

	for name, auth := range map[string]string{"revoked": "Bearer " + revokedTok, "made up": "Bearer mwa_nope", "wrong shared": "Bearer shared-secrex"} {
		if _, err := connect(open, auth); err == nil {
			t.Errorf("%s token connected", name)
		}
	}
	closed := serve("", false)
	if _, err := connect(closed, ""); err == nil {
		t.Error("tokenless request connected with anonymous off")
	}
	if sess, err := connect(closed, "Bearer "+memberTok); err != nil {
		t.Errorf("member token without anonymous: %v", err)
	} else {
		sess.Close()
	}

	// A token stops working as soon as it's revoked, mid-session.
	sess, err := connect(closed, "Bearer "+adminTok)
	if err != nil {
		t.Fatal(err)
	}
	ts, _ := acc.Tokens(ctx, admin.ID)
	acc.RevokeToken(ctx, admin.ID, ts[0].ID)
	if _, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "meal_schedule"}); err == nil {
		t.Error("revoked token kept working")
	}
}

func TestOwnerBeforeAdmin(t *testing.T) {
	// Before anyone runs messwa user add -admin, tokenless loopback keeps
	// reading the portal with MESS_API_KEY, as it did before accounts.
	ctx := context.Background()
	t.Setenv(accounts.LegacyKeyEnv, "env-key")
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	acc, _ := accounts.New(st, "")
	s := New(config.Default(), st, nil)
	s.SetAccounts(acc)
	var calls []int64
	s.portalFor = func(_ context.Context, id int64) (portalReader, error) { return fakePortal{id, &calls}, nil }
	h, _ := s.HTTPHandler("", true)
	srv := httptest.NewServer(h)
	defer srv.Close()
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "test"}, nil).Connect(ctx, &mcp.StreamableClientTransport{Endpoint: srv.URL, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "my_registrations", Arguments: map[string]any{"from": "2026-11-03", "to": "2026-11-03"}})
	if err != nil || res.IsError || len(calls) != 1 || calls[0] != 0 {
		t.Errorf("owner stand-in: %v %+v %v", err, res, calls)
	}
}
