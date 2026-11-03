package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"

	"messwa/internal/config"
	"messwa/internal/wa"
)

// This opt-in diagnostic includes saved contacts only when explicitly selected.
// It never attaches the normal ingester or saves message content to the database.
func inspectPayment(ctx context.Context, cfg *config.Config, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("inspect-payment", flag.ContinueOnError)
	peer := fs.String("peer", "", "test sender's full WhatsApp JID, e.g. 91XXXXXXXXXX@s.whatsapp.net")
	raw := fs.Bool("raw", false, "include raw payment fields in stdout; may expose financial details")
	if err := fs.Parse(args); err != nil {
		return err
	}
	jid, err := types.ParseJID(*peer)
	if err != nil || jid.User == "" || (jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer) {
		return errors.New("-peer must be a personal WhatsApp JID, not a group or broadcast")
	}
	jid = jid.ToNonAD()
	c, err := wa.Open(ctx, cfg, log)
	if err != nil {
		return err
	}
	if !c.LoggedIn() {
		return errors.New("not linked; run messwa login first")
	}
	start := time.Now()
	var mu sync.Mutex
	seen := map[string]bool{}
	enc := json.NewEncoder(os.Stdout)
	c.WA.AddEventHandler(func(evt any) {
		e, ok := evt.(*events.Message)
		if !ok {
			return
		}
		if e.Info.IsFromMe || e.Info.IsGroup || e.Info.Timestamp.Before(start) {
			return
		}
		matches := e.Info.Chat.ToNonAD() == jid || e.Info.SenderAlt.ToNonAD() == jid
		if !matches && e.Info.Chat.Server == types.HiddenUserServer && jid.Server == types.DefaultUserServer {
			pn, lookupErr := c.WA.Store.LIDs.GetPNForLID(ctx, e.Info.Chat.ToNonAD())
			matches = lookupErr == nil && pn.ToNonAD() == jid
		}
		if !matches {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if seen[e.Info.ID] {
			return
		}
		typesPresent, payment, err := paymentFields(e.Message)
		if err != nil {
			log.Error("inspect payment", "err", err)
			return
		}
		var decoded *paymentPayloadSummary
		if payload := e.Message.GetSendPaymentMessage().GetTransactionData(); payload != "" {
			decoded, err = decodePaymentPayload(payload)
			if err != nil {
				log.Warn("payment prefix not recognized", "err", err)
			}
		}
		if !*raw {
			payment = nil
		}
		if err := enc.Encode(struct {
			ID        string                     `json:"message_id"`
			Timestamp time.Time                  `json:"timestamp"`
			Types     []string                   `json:"message_fields"`
			Payment   map[string]json.RawMessage `json:"payment_fields,omitempty"`
			Verified  bool                       `json:"bank_verified"`
			Decoded   *paymentPayloadSummary     `json:"decoded_prefix,omitempty"`
		}{e.Info.ID, e.Info.Timestamp, typesPresent, payment, false, decoded}); err != nil {
			log.Error("write diagnostic", "err", err)
			return
		}
		seen[e.Info.ID] = true
	})
	if err := c.WA.Connect(); err != nil {
		return err
	}
	defer c.WA.Disconnect()
	fmt.Fprintln(os.Stderr, "Listening to new incoming messages from the selected peer. Payment fields may contain financial details; keep output private. Ctrl+C stops. No bank verification is performed.")
	<-ctx.Done()
	return ctx.Err()
}

// Report field names for other message types, but never their text or media.
// shortcut: payment payloads are opaque diagnostics until tested with real events.
func paymentFields(m *waE2E.Message) ([]string, map[string]json.RawMessage, error) {
	names := []string{}
	payments := map[string]json.RawMessage{}
	if m == nil {
		return names, payments, nil
	}
	var resultErr error
	m.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		name := fd.JSONName()
		names = append(names, name)
		if strings.Contains(strings.ToLower(name), "payment") && fd.Kind() == protoreflect.MessageKind && !fd.IsList() && !fd.IsMap() {
			data, err := protojson.Marshal(v.Message().Interface())
			if err != nil {
				resultErr = err
				return false
			}
			payments[name] = data
		}
		return true
	})
	sort.Strings(names)
	return names, payments, resultErr
}
