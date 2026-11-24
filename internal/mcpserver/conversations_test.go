package mcpserver

import (
	"messwa/internal/config"
	"messwa/internal/store"
	"path/filepath"
	"testing"
	"time"
)

func TestConversationRejectsOtherAccount(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "sale.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	now := time.Now()
	id, err := st.CreateSellTask(t.Context(), store.SellTask{UserID: 2, Date: "2026-11-24", Meal: "lunch", MinPaise: 4000, MaxPaise: 4000, CreatedAt: now, EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	srv := New(config.Default(), st, nil)
	srv.BindAccount(1, nil)
	if _, err = srv.SaleConversation(t.Context(), id, "buyer"); err == nil {
		t.Fatal("other account can read conversation")
	}
	if _, _, err = srv.ConversationMedia(t.Context(), id, "buyer", "image"); err == nil {
		t.Fatal("other account can read media")
	}
}
