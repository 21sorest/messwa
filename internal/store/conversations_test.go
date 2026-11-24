package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestConversationScopeAndMedia(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := t.Context()
	now := time.Now().Truncate(time.Second)
	id, err := s.CreateSellTask(ctx, SellTask{Date: "2026-11-24", Meal: "lunch", MinPaise: 4000, MaxPaise: 4000, CreatedAt: now, EndsAt: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ClaimOfferPost(ctx, id, "group", "offer", 0, now); err != nil {
		t.Fatal(err)
	}
	task, peer, err := s.ConversationTask(ctx, "buyer", "", "offer", now.Unix())
	if err != nil || task != id || peer != "buyer" {
		t.Fatalf("first reply: %d %s %v", task, peer, err)
	}
	task, _, _ = s.ConversationTask(ctx, "stranger", "", "", now.Unix())
	if task != 0 {
		t.Fatal("unrelated DM recorded")
	}
	if _, err = s.AddSellTaskBuyer(ctx, id, "buyer", "alias", now); err != nil {
		t.Fatal(err)
	}
	task, peer, err = s.ConversationTask(ctx, "alias", "", "", now.Unix())
	if err != nil || task != id || peer != "buyer" {
		t.Fatal("alias did not resolve")
	}
	task, _, _ = s.ConversationTask(ctx, "buyer", "", "", now.Add(2*time.Hour).Unix())
	if task != 0 {
		t.Fatal("recorded outside sale window")
	}
	m := SaleMessage{ID: "payment", At: now.Unix(), Kind: "image", Checking: "Unverified"}
	for range 2 {
		if err = s.AddSaleMessage(ctx, id, "buyer", m); err != nil {
			t.Fatal(err)
		}
	}
	if err = s.SetSaleMessageMedia(ctx, id, "buyer", m.ID, "image/png", []byte("test image")); err != nil {
		t.Fatal(err)
	}
	messages, err := s.SaleMessages(ctx, id, "buyer")
	if err != nil || len(messages) != 1 || !messages[0].Media || messages[0].Checking == "" {
		t.Fatalf("messages: %v %v", messages, err)
	}
	other, err := s.SaleMessages(ctx, id, "stranger")
	if err != nil || len(other) != 0 {
		t.Fatal("wrong buyer can read messages")
	}
	if _, _, err = s.SaleMessageMedia(ctx, id, "stranger", m.ID); err == nil {
		t.Fatal("wrong buyer can read image")
	}
}

func TestConversationFollowsMovedBuyer(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := t.Context()
	now := time.Now().Truncate(time.Second)
	task := func(meal string, created time.Time) int64 {
		id, err := s.CreateSellTask(ctx, SellTask{Date: "2026-11-24", Meal: meal, MinPaise: 4000, MaxPaise: 4000, CreatedAt: created, EndsAt: now.Add(time.Hour)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.AddSellTaskBuyer(ctx, id, "buyer", "", now); err != nil {
			t.Fatal(err)
		}
		return id
	}
	// The newer sale passed the buyer to the older one. One sale runs per
	// meal, so they differ in meal; ConversationTask ignores it.
	older, newer := task("lunch", now.Add(-time.Minute)), task("dinner", now)
	if _, err = s.db.ExecContext(ctx, `UPDATE sell_task_buyers SET state=? WHERE task_id=? AND peer='buyer'`, BuyerMoved, newer); err != nil {
		t.Fatal(err)
	}
	got, _, err := s.ConversationTask(ctx, "buyer", "", "", now.Unix())
	if err != nil || got != older {
		t.Fatalf("recorded under %d, want the sale holding the buyer %d (%v)", got, older, err)
	}
}
