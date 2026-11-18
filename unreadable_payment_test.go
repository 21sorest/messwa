package main

import (
	"context"
	"path/filepath"
	"testing"
	"testing/synctest"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"

	"messwa/internal/store"
)

// blankPayment is a WhatsApp Pay message as a linked device sometimes gets
// it: no note, no transaction data, so no amount or currency.
func blankPayment(buyer types.JID, id string) *events.Message {
	return &events.Message{
		Info:    types.MessageInfo{MessageSource: types.MessageSource{Chat: buyer}, ID: id, Timestamp: time.Now()},
		Message: &waE2E.Message{SendPaymentMessage: &waE2E.SendPaymentMessage{}},
	}
}

// TestUnreadablePaymentSetting is the 17 Nov case: the buyer pays with
// WhatsApp Pay and the linked device gets a payment with no amount. Off,
// the QR stays put after recovery; on, the QR goes out at the agreed price
// once recovery gives up.
func TestUnreadablePaymentSetting(t *testing.T) {
	for _, on := range []bool{false, true} {
		path := filepath.Join(t.TempDir(), "data.db")
		synctest.Test(t, func(t *testing.T) {
			f := newSellFixture(t, path)
			f.cfg.Sell.AcceptUnreadablePayments = on
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			m := f.manager(ctx)
			id, err := m.Start(ctx, 0, f.date, "lunch", 20, 20, "")
			if err != nil {
				t.Fatal(err)
			}
			synctest.Wait()
			f.conn.emit(f.reply(buyerA, "interested"))
			synctest.Wait()
			time.Sleep(time.Second)
			f.conn.emit(blankPayment(buyerA, "pay-a"))
			synctest.Wait()
			if got := texts(sentTo(f.conn.messages(), buyerA)); len(got) != 3 {
				t.Fatalf("on=%v: A got %q before recovery ended", on, got)
			}
			time.Sleep(45 * time.Second)
			synctest.Wait()
			got := texts(sentTo(f.conn.messages(), buyerA))
			task := f.task(id)
			if on {
				if len(got) != 4 || got[3] != "[QR]" || task.State != store.TaskSold {
					t.Fatalf("on: A got %q, task %s", got, task.State)
				}
				ds, err := f.st.Deliveries(ctx)
				if err != nil || len(ds) != 1 || ds[0].State != "sent" || ds[0].Amount != "20.00" {
					t.Fatalf("ledger = %+v, %v", ds, err)
				}
			} else if len(got) != 3 || task.State != store.TaskActive {
				t.Fatalf("off: A got %q, task %s", got, task.State)
			}
			cancel()
			synctest.Wait()
		})
	}
}
