package portal

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
)

// Bill is one month's mess bill, in paise, as the portal's
// /registrations/bill endpoint reports it. Availed is for meals already
// served; Projected is for registered meals still to come.
type Bill struct {
	Availed   int64 `json:"non_projected"`
	Projected int64 `json:"projected"`
}

// Total is the whole month's bill in paise.
func (b Bill) Total() int64 { return b.Availed + b.Projected }

// Bill reads the bill for month (1-12) of year.
func (c *Client) Bill(ctx context.Context, year, month int) (Bill, error) {
	if month < 1 || month > 12 || year < 2000 || year > 9999 {
		return Bill{}, errors.New("bill month must be 1-12 of a four-digit year")
	}
	data, err := c.getData(ctx, "/registrations/bill", url.Values{"month": {strconv.Itoa(month)}, "year": {strconv.Itoa(year)}})
	if err != nil {
		return Bill{}, err
	}
	var raw struct {
		Availed   *int64 `json:"non_projected"`
		Projected *int64 `json:"projected"`
	}
	if err := json.Unmarshal(data, &raw); err != nil || raw.Availed == nil || raw.Projected == nil || *raw.Availed < 0 || *raw.Projected < 0 {
		return Bill{}, errors.New("invalid bill response: expected non_projected and projected amounts")
	}
	return Bill{Availed: *raw.Availed, Projected: *raw.Projected}, nil
}
