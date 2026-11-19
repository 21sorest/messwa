package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestGalaDinnersLeaveNormalFacts(t *testing.T) {
	ctx := context.Background()
	s, err := Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	end := time.Date(2026, 11, 18, 16, 0, 0, 0, time.UTC).Unix()
	for i, row := range []struct {
		meal  string
		price int
	}{{"lunch", 60}, {"dinner", 200}, {"dinner", 220}} {
		if _, err := s.db.Exec(`INSERT INTO listing_facts(key,seller,kind,meal,meal_date,mess,diet,price,qty,first_seen,last_seen,meal_end,posts,groups)
			VALUES(?,'a','sell',?,'2026-11-18','kadamba','veg',?,1,?,?,?,1,1)`, string(rune('a'+i)), row.meal, row.price, end-60, end-60, end); err != nil {
			t.Fatal(err)
		}
		l := &Listing{Seller: string(rune('a' + i)), Kind: "sell", Meal: row.meal, MealDate: "2026-11-18", Mess: "kadamba", Price: row.price, Qty: 1, Text: "x", Norm: "x", FirstSeen: time.Unix(end-60, 0), LastSeen: time.Unix(end-60, 0)}
		if err := s.CreateListing(ctx, l); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SetGalaDinner(ctx, "18-11-2026", true); err == nil {
		t.Fatal("took a date that isn't YYYY-MM-DD")
	}
	if err := s.SetGalaDinner(ctx, "2026-11-18", true); err != nil {
		t.Fatal(err)
	}
	if d, err := s.GalaDinners(ctx); err != nil || len(d) != 1 || d[0] != "2026-11-18" {
		t.Fatalf("gala dinners %v %v", d, err)
	}
	if fs, err := s.Facts(ctx, "2026-11-18", "2026-11-18"); err != nil || len(fs) != 1 || fs[0].Meal != "lunch" {
		t.Fatalf("facts %+v %v: want only lunch", fs, err)
	}
	if fs, err := s.GalaFacts(ctx); err != nil || len(fs) != 2 || fs[0].Meal != "dinner" {
		t.Fatalf("gala facts %+v %v: want both dinners", fs, err)
	}
	if ts, err := s.FactTotals(ctx, "2026-11-18"); err != nil || len(ts) != 1 || ts[0].Offers != 1 || ts[0].AskSum != 60 {
		t.Fatalf("totals %+v %v: want the lunch alone", ts, err)
	}
	ps, err := s.Prices(ctx, "2026-11-18", "2026-11-18")
	if err != nil || len(ps) != 2 {
		t.Fatalf("prices %+v %v", ps, err)
	}
	for _, p := range ps {
		if p.Gala != (p.Meal == "dinner") {
			t.Errorf("%s gala = %v", p.Meal, p.Gala)
		}
	}

	if err := s.SetGalaDinner(ctx, "2026-11-18", false); err != nil {
		t.Fatal(err)
	}
	if fs, err := s.Facts(ctx, "2026-11-18", "2026-11-18"); err != nil || len(fs) != 3 {
		t.Fatalf("after unmarking, facts %+v %v: want all three", fs, err)
	}
}
