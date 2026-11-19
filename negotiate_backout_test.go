package main

import (
	"context"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"messwa/internal/store"
)

// A buyer asked to pay who then offers less than they agreed didn't take
// the deal. They lose the turn and wait for you, with no word from the
// bot, and the next buyer is asked to pay. On autopilot they're skipped
// and hear not to pay.
func TestHolderWhoOffersLessLosesTurn(t *testing.T) {
	for _, tc := range []struct {
		name      string
		autopilot bool
		state     string
		note      string
		told      []string
	}{
		{"manual", false, store.BuyerParked, "agreed ₹60, then offered ₹50; check the chat", nil},
		{"autopilot", true, store.BuyerSkipped, "agreed ₹60, then offered ₹50; check the chat", []string{"wait dont pay", "cant do this one sorry"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "data.db")
			synctest.Test(t, func(t *testing.T) {
				f := newSellFixture(t, path)
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				m := f.negotiatingManager(ctx, 60)
				id, _, err := m.StartNow(ctx, 0, f.date, "lunch", 30, 60, tc.autopilot)
				if err != nil {
					t.Fatal(err)
				}
				synctest.Wait()

				// B agrees the opening 60 and holds the turn.
				f.conn.emit(f.reply(buyerB, "price?"))
				settle()
				f.conn.emit(dm(buyerB, "ok"))
				settle()
				// C haggles down and agrees less; they wait behind B.
				f.conn.emit(f.reply(buyerC, "price?"))
				settle()
				f.conn.emit(dm(buyerC, "40?"))
				settle()
				f.conn.emit(dm(buyerC, "ok"))
				settle()
				if b := buyerState(t, f, id, buyerB); b.State != store.BuyerOffered {
					t.Fatalf("B = %+v, want asked to pay", b)
				}
				before := len(sentTo(f.conn.messages(), buyerB))
				beforeC := len(sentTo(f.conn.messages(), buyerC))

				f.conn.emit(dm(buyerB, "Naa I said for 50 bro"))
				settle()
				time.Sleep(time.Minute)
				synctest.Wait()

				b := buyerState(t, f, id, buyerB)
				if b.State != tc.state || b.Note != tc.note {
					t.Fatalf("B = %+v, want %s with note %q", b, tc.state, tc.note)
				}
				got := texts(sentTo(f.conn.messages(), buyerB))[before:]
				if len(got) != len(tc.told) {
					t.Fatalf("B was told %q, want %q", got, tc.told)
				}
				for i := range got {
					if got[i] != tc.told[i] {
						t.Fatalf("B was told %q, want %q", got, tc.told)
					}
				}
				if c := buyerState(t, f, id, buyerC); c.State != store.BuyerOffered {
					t.Fatalf("C = %+v, want asked to pay after B backed out", c)
				}
				if len(sentTo(f.conn.messages(), buyerC)) <= beforeC {
					t.Fatal("C was never asked to pay")
				}
			})
		})
	}
}

// Naming the agreed price, or more, after the pay prompt keeps the turn.
func TestHolderWhoRepeatsPriceKeepsTurn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	synctest.Test(t, func(t *testing.T) {
		f := newSellFixture(t, path)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		m := f.negotiatingManager(ctx, 60)
		id, _, err := m.StartNow(ctx, 0, f.date, "lunch", 30, 60, false)
		if err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		f.conn.emit(f.reply(buyerB, "price?"))
		settle()
		f.conn.emit(dm(buyerB, "ok"))
		settle()
		f.conn.emit(dm(buyerB, "60 right?"))
		settle()
		if b := buyerState(t, f, id, buyerB); b.State != store.BuyerOffered {
			t.Fatalf("B = %+v, want still asked to pay", b)
		}
	})
}
