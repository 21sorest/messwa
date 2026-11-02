// Package dashboard serves a read-only web view of the merged listings,
// laid out like an auction house: messes down the left, listings in a grid,
// sort and filter buttons along the bottom.
//
// It shows sellers' names and phone numbers from the groups and nothing
// from DMs. It has no write endpoints.
package dashboard

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"messwa/internal/config"
	"messwa/internal/mcpserver"
	"messwa/internal/meals"
)

//go:embed static
var static embed.FS

type mess struct {
	Name    string `json:"name"`
	VegOnly bool   `json:"veg_only"`
}

type slot struct {
	Meal  string `json:"meal"`
	Date  string `json:"date"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
}

type state struct {
	Now      int64                `json:"now"`
	TZ       string               `json:"tz"`
	Messes   []mess               `json:"messes"`
	Today    []slot               `json:"today"`
	Current  *slot                `json:"current,omitempty"`
	Next     *slot                `json:"next,omitempty"`
	Listings []mcpserver.FeedItem `json:"listings"`
}

// Handler serves the page at / and its data at /api/state and /api/listing.
func Handler(cfg *config.Config, srv *mcpserver.Server) http.Handler {
	mux := http.NewServeMux()
	files, _ := fs.Sub(static, "static")
	mux.Handle("GET /", http.FileServerFS(files))

	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		now := srv.Now()
		items, err := srv.Feed(r.Context(), now.Add(-30*time.Hour))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		st := state{Now: now.Unix(), TZ: cfg.Timezone, Listings: items}
		for _, m := range cfg.Messes {
			st.Messes = append(st.Messes, mess{m.Name, m.VegOnly})
		}
		sc := srv.Schedule()
		for _, s := range sc.Today {
			st.Today = append(st.Today, toSlot(&s))
		}
		if sc.Current != nil {
			c := toSlot(sc.Current)
			st.Current = &c
		}
		if sc.Next != nil {
			n := toSlot(sc.Next)
			st.Next = &n
		}
		writeJSON(w, st)
	})

	mux.HandleFunc("GET /api/listing", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		out, err := srv.Get(r.Context(), id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, out)
	})

	return headers(mux)
}

func toSlot(s *meals.Slot) slot {
	return slot{Meal: s.Meal, Date: s.Date, Start: s.Start.Unix(), End: s.End.Unix()}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

// headers locks the page down: no framing, no third-party scripts, and
// nothing the posts contain can load or run code.
func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
