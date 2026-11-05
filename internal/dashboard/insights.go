package dashboard

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"messwa/internal/config"
	"messwa/internal/mcpserver"
)

// insightsCols names the fields of each row in /api/insights, in order.
// Rows are arrays because a month of the market is ~25,000 of them.
var insightsCols = []string{
	"date", "meal", "mess", "diet", "kind", "price", "qty",
	"first", "last", "end", "sold", "posts", "groups", "seller", "closed", "others", "deleted",
}

type insightsState struct {
	Now   int64  `json:"now"`
	TZ    string `json:"tz"`
	Today string `json:"today"`
	// From is the first meal date in Rows, First the earliest one stored.
	From    string   `json:"from"`
	First   string   `json:"first"`
	Messes  []mess   `json:"messes"`
	Windows []window `json:"windows"`
	// Sellers are indexed by each row's seller field. Names are untrusted
	// push names.
	Sellers []seller `json:"sellers"`
	Cols    []string `json:"cols"`
	Rows    [][]any  `json:"rows"`
}

type window struct {
	Meal   string `json:"meal"`
	Starts string `json:"starts"`
	Ends   string `json:"ends"`
}

type seller struct {
	Name  string `json:"name,omitempty"`
	Phone string `json:"phone,omitempty"`
}

// insightsHandler serves the listing facts for the last ?weeks= weeks of
// meals, or every stored one when weeks is 0 or missing.
func insightsHandler(cfg *config.Config, srv *mcpserver.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		now := srv.Now()
		today := now.In(cfg.Location()).Format("2006-01-02")
		first, err := srv.FirstFactDate(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		from := first
		if weeks, _ := strconv.Atoi(r.URL.Query().Get("weeks")); weeks > 0 {
			d := now.In(cfg.Location()).AddDate(0, 0, 1-7*weeks).Format("2006-01-02")
			if d > from {
				from = d
			}
		}
		st := insightsState{Now: now.Unix(), TZ: cfg.Timezone, Today: today, From: from, First: first,
			Messes: []mess{}, Windows: []window{}, Sellers: []seller{}, Cols: insightsCols, Rows: [][]any{}}
		for _, m := range cfg.Messes {
			st.Messes = append(st.Messes, mess{m.Name, m.VegOnly})
		}
		for _, m := range cfg.Meals {
			st.Windows = append(st.Windows, window{m.Name, m.Starts, m.Ends})
		}
		if first == "" {
			writeJSON(w, r, st)
			return
		}
		facts, err := srv.Facts(r.Context(), from, today)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		idx := map[string]int{}
		for _, f := range facts {
			i, ok := idx[f.Seller]
			if !ok {
				i = len(st.Sellers)
				idx[f.Seller] = i
				st.Sellers = append(st.Sellers, seller{Phone: phoneOf(f.Seller)})
			}
			// Facts come oldest first, so the last name wins.
			if f.SellerName != "" {
				st.Sellers[i].Name = f.SellerName
			}
			st.Rows = append(st.Rows, []any{
				f.Date, f.Meal, f.Mess, f.Diet, f.Kind, f.Price, f.Qty,
				f.FirstSeen.Unix(), f.LastSeen.Unix(), f.End.Unix(), unixOrZero(f.SoldAt), f.Posts, f.Groups,
				i, b2i(f.Closed), f.OtherTicks, b2i(f.Deleted),
			})
		}
		writeJSON(w, r, st)
	}
}

func phoneOf(jid string) string {
	if u, ok := strings.CutSuffix(jid, "@s.whatsapp.net"); ok {
		return "+" + u
	}
	return ""
}

func unixOrZero(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
