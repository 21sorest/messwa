package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExampleConfig(t *testing.T) {
	c, err := Load("../../config.example.toml")
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Meals) != 4 || len(c.Messes) != 4 {
		t.Errorf("defaults: %d meals, %d messes", len(c.Meals), len(c.Messes))
	}
	if c.Dedupe.Window.Hours() != 24 || c.DMs.Retention.Hours() != 72 {
		t.Errorf("durations: %v %v", c.Dedupe.Window, c.DMs.Retention)
	}
}

func TestTablesReplaceDefaults(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.toml")
	os.WriteFile(p, []byte("[[messes]]\nname = \"north\"\n"), 0o600)
	c, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Messes) != 1 || len(c.Meals) != 4 {
		t.Errorf("got %d messes, %d meals", len(c.Messes), len(c.Meals))
	}
}
