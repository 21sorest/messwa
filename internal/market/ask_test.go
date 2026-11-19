package market

import (
	"testing"

	"messwa/internal/store"
)

func TestMedianAsk(t *testing.T) {
	// 2026-11-11 is a Wednesday.
	ps := []store.ServingPrices{
		{Date: "2026-11-04", Meal: "lunch", Mess: "kadamba", Diet: "veg", Prices: []int{100, 110}},
		{Date: "2026-10-28", Meal: "lunch", Mess: "kadamba", Diet: "veg", Prices: []int{90}},
		{Date: "2026-11-10", Meal: "lunch", Mess: "kadamba", Diet: "veg", Prices: []int{40, 45, 50}},
		{Date: "2026-11-11", Meal: "lunch", Mess: "bakul", Diet: "veg", Prices: []int{30, 35, 40}},
		{Date: "2026-11-11", Meal: "dinner", Mess: "kadamba", Diet: "veg", Prices: []int{60, 60, 60}},
		{Date: "2026-09-01", Meal: "lunch", Mess: "kadamba", Diet: "veg", Prices: []int{500, 500, 500}},
		{Date: "2026-11-10", Meal: "lunch", Mess: "kadamba", Diet: "nonveg", Prices: []int{110, 120, 130}},
		{Date: "2026-11-11", Meal: "snacks", Mess: "kadamba", Diet: "veg", Prices: []int{15, 20, 25}},
	}
	cases := []struct {
		name, date, meal, mess, diet string
		want                         float64
		ok                           bool
	}{
		{"same weekday at this mess when the serving has too few", "2026-11-11", "lunch", "kadamba", "", 100, true},
		{"serving at this mess", "2026-11-10", "lunch", "kadamba", "veg", 45, true},
		{"every mess for the serving", "2026-11-11", "lunch", "palash", "veg", 35, true},
		{"any ask for the meal", "2026-11-12", "lunch", "palash", "veg", 45, true},
		{"nothing for the meal", "2026-11-11", "breakfast", "kadamba", "veg", 0, false},
		{"non-veg leaves out veg asks", "2026-11-10", "lunch", "kadamba", "nonveg", 120, true},
		{"non-veg for the meal over the window", "2026-11-12", "lunch", "bakul", "nonveg", 120, true},
		{"every diet when the diet has no asks", "2026-11-11", "snacks", "kadamba", "egg", 20, true},
	}
	for _, c := range cases {
		got, ok := MedianAsk(ps, c.date, c.meal, c.mess, c.diet)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: got %v %v, want %v %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestMedianAskGala(t *testing.T) {
	ps := []store.ServingPrices{
		{Date: "2026-11-04", Meal: "dinner", Mess: "kadamba", Diet: "veg", Prices: []int{50, 55, 60}},
		{Date: "2026-11-11", Meal: "dinner", Mess: "kadamba", Diet: "veg", Prices: []int{200, 220, 250}, Gala: true},
	}
	// A week after the gala, the same weekday's dinner goes by the normal one.
	if got, ok := MedianAsk(ps, "2026-11-18", "dinner", "kadamba", "veg"); got != 55 || !ok {
		t.Errorf("the week after: got %v %v, want 55", got, ok)
	}
	// The gala dinner itself goes by its own asks.
	if got, ok := MedianAsk(ps, "2026-11-11", "dinner", "kadamba", "veg"); got != 220 || !ok {
		t.Errorf("the gala: got %v %v, want 220", got, ok)
	}
	if got := Regular(ps); len(got) != 1 || got[0].Gala {
		t.Errorf("Regular kept %+v", got)
	}
}
