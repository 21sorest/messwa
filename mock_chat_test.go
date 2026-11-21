package main

import (
	"encoding/base64"
	"path/filepath"
	"testing"
	"time"

	"messwa/internal/config"
	"messwa/internal/store"
)

// TestMockPaymentPayload checks the mock's WhatsApp Pay reads back as the
// amount it was made for.
func TestMockPaymentPayload(t *testing.T) {
	at := time.Unix(1793700000, 0)
	p, err := decodePaymentPayload(base64.StdEncoding.EncodeToString(mockPaymentPayload("45.00", at)))
	if err != nil || p.Amount != "45.00" || p.Currency != "INR" || !p.Timestamp.Equal(at) {
		t.Fatalf("payload = %+v, %v", p, err)
	}
}

// TestMockClock pretends it's 19:00, when dinner can go up, and back.
func TestMockClock(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	cfg := config.Default()
	cfg.ApplySaved(nil)
	m := &mockChat{cfg: cfg, st: st}
	if err := m.setClock(t.Context(), "19:00"); err != nil {
		t.Fatal(err)
	}
	for _, c := range []*config.Config{cfg, cfg.Live()} {
		if got := time.Now().In(c.Location()).Format("15:04"); got != "19:00" && got != "19:01" {
			t.Fatalf("pretend clock reads %s", got)
		}
	}
	date := time.Now().In(cfg.Location()).Format("2006-01-02")
	slot, err := findSlot(cfg, date, "dinner")
	if err != nil || tooEarly(cfg, slot, time.Now()) != nil || !time.Now().Before(slot.End) {
		t.Fatalf("dinner at a pretend 19:00: %+v, %v, %v", slot, err, tooEarly(cfg, slot, time.Now()))
	}
	if err := m.setClock(t.Context(), ""); err != nil || cfg.Location().String() != cfg.Timezone {
		t.Fatalf("real time: %v, %s", err, cfg.Location())
	}
	if err := m.setClock(t.Context(), "7pm"); err == nil {
		t.Error("7pm accepted")
	}
}
