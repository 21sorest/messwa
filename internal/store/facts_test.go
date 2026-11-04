package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestFactsOutlivePruneAndRebuild(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	rule := SoldRule{Emojis: []string{"✅"}, ClosedEmojis: []string{"❌"}}
	posted := time.Date(2026, 11, 3, 11, 0, 0, 0, time.UTC)
	end := posted.Add(3 * time.Hour)

	add := func(seller, chat, id, mess string, price int, end time.Time) *Listing {
		l := &Listing{Seller: seller, SellerName: seller + " name", Kind: "sell", Meal: "lunch", MealDate: "2026-11-03", MealEnd: end,
			Mess: mess, Diet: "veg", Price: price, Qty: 1, Text: "x", Norm: "x", FirstSeen: posted, LastSeen: posted}
		if err := s.CreateListing(ctx, l); err != nil {
			t.Fatal(err)
		}
		if err := s.AddPost(ctx, Post{Chat: chat, ID: id, ListingID: l.ID, Sender: seller, TS: posted}); err != nil {
			t.Fatal(err)
		}
		return l
	}
	sold := add("a@s.whatsapp.net", "g1", "m1", "bakul", 20, end)
	// A repost in another group an hour later is the same listing.
	if err := s.AddPost(ctx, Post{Chat: "g2", ID: "m2", ListingID: sold.ID, Sender: sold.Seller, TS: posted.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	s.SetReaction(ctx, "g2", "m2", "a@s.whatsapp.net", "", "✅", posted.Add(70*time.Minute))
	s.SetReaction(ctx, "g1", "m1", "b@s.whatsapp.net", "", "✅", posted.Add(5*time.Minute))
	add("c@s.whatsapp.net", "g1", "m3", "kadamba", -1, end)
	s.SetReaction(ctx, "g1", "m3", "c@s.whatsapp.net", "", "❌", posted.Add(time.Minute))
	add("d@s.whatsapp.net", "g1", "m4", "palash", 25, end.Add(48*time.Hour)) // meal not over yet

	if err := s.SnapshotFacts(ctx, rule, end.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	fs, err := s.Facts(ctx, "2026-11-01", "2026-11-30")
	if err != nil || len(fs) != 2 {
		t.Fatalf("facts %+v %v: want the two ended listings", fs, err)
	}
	var f Fact
	for _, x := range fs {
		if x.Mess == "bakul" {
			f = x
		}
	}
	if f.SoldAt.Unix() != posted.Add(70*time.Minute).Unix() || f.Posts != 2 || f.Groups != 2 || f.OtherTicks != 1 || f.Price != 20 || f.Closed {
		t.Fatalf("sold fact %+v", f)
	}
	for _, x := range fs {
		if x.Mess == "kadamba" && (!x.Closed || !x.SoldAt.IsZero()) {
			t.Fatalf("closed fact %+v", x)
		}
	}

	// An un-tick within the freeze window un-sells it.
	s.SetReaction(ctx, "g2", "m2", "a@s.whatsapp.net", "", "", posted.Add(2*time.Hour))
	s.SnapshotFacts(ctx, rule, end.Add(time.Hour))
	if fs, _ = s.Facts(ctx, "2026-11-03", "2026-11-03"); !soldIn(fs, "bakul").IsZero() {
		t.Fatal("un-tick inside the freeze window should un-sell")
	}
	s.SetReaction(ctx, "g2", "m2", "a@s.whatsapp.net", "", "✅", posted.Add(2*time.Hour))
	s.SnapshotFacts(ctx, rule, end.Add(time.Hour))

	// Rebuild drops every listing and re-creates them under new IDs. Pruning
	// then deletes the reactions. Neither changes a frozen fact or adds a row.
	if err := s.ResetDerived(ctx); err != nil {
		t.Fatal(err)
	}
	add("a@s.whatsapp.net", "g1", "m1", "bakul", 20, end)
	later := end.Add(FactsFreeze + time.Hour)
	if err := s.SnapshotFacts(ctx, rule, later); err != nil {
		t.Fatal(err)
	}
	fs, _ = s.Facts(ctx, "2026-11-01", "2026-11-30")
	if len(fs) != 2 || soldIn(fs, "bakul").IsZero() {
		t.Fatalf("after rebuild %+v: want the same two facts, still sold", fs)
	}
	if err := s.Prune(ctx, later, later); err != nil {
		t.Fatal(err)
	}
	if fs, _ = s.Facts(ctx, "2026-11-01", "2026-11-30"); len(fs) != 2 {
		t.Fatalf("after prune %d facts", len(fs))
	}
	if d, _ := s.FirstFactDate(ctx); d != "2026-11-03" {
		t.Fatalf("first fact date %q", d)
	}
}

func soldIn(fs []Fact, mess string) time.Time {
	for _, f := range fs {
		if f.Mess == mess {
			return f.SoldAt
		}
	}
	return time.Time{}
}
