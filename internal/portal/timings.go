package portal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// MealWindow is when one meal is served, as "HH:MM" in IST.
type MealWindow struct {
	Starts string `json:"starts"`
	Ends   string `json:"ends"`
}

// MealTimings reads when each meal is served on date (YYYY-MM-DD) from the
// portal's /config/meal-timings, which needs no API key (see Public). The portal lists
// each mess and diet on its own; a meal's window here runs from the
// earliest start to the latest end among them.
func (c *Client) MealTimings(ctx context.Context, date string) (map[string]MealWindow, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, errors.New("date must be YYYY-MM-DD")
	}
	data, err := c.getData(ctx, "/config/meal-timings", url.Values{"on": {date}})
	if err != nil {
		return nil, err
	}
	return parseMealTimings(data)
}

func parseMealTimings(data json.RawMessage) (map[string]MealWindow, error) {
	var messes map[string][]struct {
		Meal  string `json:"meal"`
		Start string `json:"start_time"`
		End   string `json:"end_time"`
	}
	if err := json.Unmarshal(data, &messes); err != nil {
		return nil, errors.New("invalid meal timings response: expected timings by mess")
	}
	clock := func(s string) (int, bool) {
		t, err := time.Parse("15:04:05", s)
		if err != nil {
			t, err = time.Parse("15:04", s)
		}
		return t.Hour()*60 + t.Minute(), err == nil
	}
	type span struct{ start, end int }
	spans := map[string]span{}
	for mess, rows := range messes {
		for _, r := range rows {
			switch r.Meal {
			case "breakfast", "lunch", "snacks", "dinner":
			default:
				continue
			}
			st, ok1 := clock(r.Start)
			en, ok2 := clock(r.End)
			if !ok1 || !ok2 || en <= st {
				return nil, fmt.Errorf("invalid meal timings response: %s %s runs %q to %q", mess, r.Meal, r.Start, r.End)
			}
			if sp, ok := spans[r.Meal]; ok {
				st, en = min(st, sp.start), max(en, sp.end)
			}
			spans[r.Meal] = span{st, en}
		}
	}
	if len(spans) == 0 {
		return nil, errors.New("meal timings response lists no meals")
	}
	out := map[string]MealWindow{}
	for meal, sp := range spans {
		out[meal] = MealWindow{fmt.Sprintf("%02d:%02d", sp.start/60, sp.start%60), fmt.Sprintf("%02d:%02d", sp.end/60, sp.end%60)}
	}
	return out, nil
}
