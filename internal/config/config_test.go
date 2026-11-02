package config

import "testing"

func TestExampleConfig(t *testing.T) {
	c, err := Load("../../config.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Meals) != 4 || len(c.Messes) != 3 {
		t.Errorf("tables appended to defaults instead of replacing: %d meals, %d messes", len(c.Meals), len(c.Messes))
	}
	if c.Dedupe.Window.Hours() != 3 || c.DMs.Retention.Hours() != 72 {
		t.Errorf("durations: %v %v", c.Dedupe.Window, c.DMs.Retention)
	}
}
