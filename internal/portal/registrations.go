// Package portal reads the same API used by IIITH-mess-MCP. It has no write methods.
package portal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type Registration struct {
	Date        string  `json:"meal_date"`
	Meal        string  `json:"meal_type"`
	Mess        string  `json:"meal_mess"`
	Category    string  `json:"category"`
	CancelledAt *string `json:"cancelled_at"`
	AvailedAt   *string `json:"availed_at"`
}

func (r Registration) Cancelled() bool { return r.CancelledAt != nil && *r.CancelledAt != "" }
func (r Registration) Availed() bool   { return r.AvailedAt != nil && *r.AvailedAt != "" }

// MessDiet splits the portal's mess field, such as "kadamba-veg" or
// "bakul-nonveg", into the mess name and the diet. Diet is empty when the
// field doesn't say.
func (r Registration) MessDiet() (mess, diet string) {
	parts := strings.FieldsFunc(strings.ToLower(r.Mess), func(c rune) bool { return c == '-' || c == '_' || c == ' ' })
	if len(parts) == 0 {
		return "", ""
	}
	rest := strings.Join(parts[1:], "")
	switch {
	case strings.Contains(rest, "non"):
		diet = "nonveg"
	case strings.Contains(rest, "egg"):
		diet = "egg"
	case strings.Contains(rest, "veg"):
		diet = "veg"
	}
	return parts[0], diet
}

type Client struct {
	baseURL string
	key     string
	http    *http.Client
}

// New returns a client that reads the portal with one person's mess API
// key. MESS_API_URL overrides the portal address.
func New(key string) (*Client, error) {
	base := os.Getenv("MESS_API_URL")
	if base == "" {
		base = "https://mess.iiit.ac.in/api"
	}
	if strings.TrimSpace(key) == "" {
		return nil, ErrNoKey
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("MESS_API_URL must be an HTTPS base URL without credentials, query or fragment")
	}
	return &Client{baseURL: strings.TrimRight(base, "/"), key: key, http: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}

// ErrNoKey means the account has no mess API key saved.
var ErrNoKey = errors.New("no mess API key saved on this account; add one on the account page")

func ValidateRange(from, to string) error {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return errors.New("from must be YYYY-MM-DD")
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return errors.New("to must be YYYY-MM-DD")
	}
	if end.Before(start) || end.After(start.AddDate(0, 2, 0)) {
		return errors.New("registration range must be ordered and no longer than two months")
	}
	return nil
}

func (c *Client) Registrations(ctx context.Context, from, to string) ([]Registration, error) {
	if err := ValidateRange(from, to); err != nil {
		return nil, err
	}
	data, err := c.getData(ctx, "/registrations", url.Values{"from": {from}, "to": {to}})
	if err != nil {
		return nil, err
	}
	rows := []Registration{}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, errors.New("invalid registration response: expected registration array")
	}
	if rows == nil {
		rows = []Registration{}
	}
	for _, r := range rows {
		_, dateErr := time.Parse("2006-01-02", r.Date)
		validMeal := r.Meal == "breakfast" || r.Meal == "lunch" || r.Meal == "snacks" || r.Meal == "dinner"
		if dateErr != nil || r.Date < from || r.Date > to || !validMeal || r.Mess == "" || r.Category == "" {
			return nil, errors.New("invalid or out-of-range meal registration in portal response")
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		return rows[i].Meal < rows[j].Meal
	})
	return rows, nil
}

func (c *Client) getData(ctx context.Context, endpoint string, params url.Values) (json.RawMessage, error) {
	u := c.baseURL + endpoint
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, errors.New("could not construct portal read request")
	}
	// The existing MCP uses the API key directly, without a Bearer prefix.
	req.Header.Set("Authorization", c.key)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("could not reach mess portal; check network/DNS and MESS_API_URL")
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, errors.New("mess portal rejected the mess API key")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("mess portal returned HTTP %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024+1))
	if err != nil {
		return nil, errors.New("could not read portal response")
	}
	if len(b) > 2*1024*1024 {
		return nil, errors.New("portal response exceeds 2 MiB")
	}
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(b, &envelope); err != nil || len(envelope.Data) == 0 {
		return nil, errors.New("invalid portal response: expected data field")
	}
	return envelope.Data, nil
}
