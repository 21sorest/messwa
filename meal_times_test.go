package main

import (
	"testing"

	"messwa/internal/config"
)

func TestValidMealTimes(t *testing.T) {
	ok := map[string]config.MealTime{"breakfast": {Starts: "06:30", Ends: "10:30"}, "lunch": {Starts: "11:30", Ends: "15:30"},
		"snacks": {Starts: "16:45", Ends: "18:15"}, "dinner": {Starts: "18:30", Ends: "22:30"}}
	if !validMealTimes(ok) {
		t.Error("portal times of 15 Nov refused")
	}
	for _, bad := range []map[string]config.MealTime{
		nil,
		{"lunch": {Starts: "11:30", Ends: "15:30"}, "snacks": {Starts: "15:00", Ends: "18:15"}},
		{"lunch": {Starts: "15:30", Ends: "11:30"}},
		{"lunch": {Starts: "noon", Ends: "15:30"}},
	} {
		if validMealTimes(bad) {
			t.Errorf("validMealTimes(%v) took it", bad)
		}
	}
}
