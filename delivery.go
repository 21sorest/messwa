package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"

	"messwa/internal/config"
	"messwa/internal/meals"
	"messwa/internal/portal"
	"messwa/internal/store"
	"messwa/internal/wa"
)

//go:embed assets/qr-placeholder.png
var placeholderQR []byte

func deliverQR(ctx context.Context, cfg *config.Config, log *slog.Logger, args []string) error {
	fs := flag.NewFlagSet("deliver-qr", flag.ContinueOnError)
	date := fs.String("date", "", "meal date, YYYY-MM-DD in the configured timezone")
	meal := fs.String("meal", "", "configured meal name")
	peer := fs.String("peer", "", "reserved buyer's full WhatsApp JID")
	amount := fs.String("amount", "", "agreed INR amount, e.g. 60.00")
	qr := fs.String("qr", "", "PNG or JPEG image to send (defaults to an embedded, labelled test image)")
	portalQR := fs.Bool("portal-qr", false, "fetch the current QR from the portal immediately before delivery; requires MESS_API_KEY")
	offer := fs.String("offer-text", "", "optional offer to send to the selected buyer once connected and armed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *portalQR && *qr != "" {
		return errors.New("choose either -portal-qr or -qr")
	}
	day, err := time.ParseInLocation("2006-01-02", *date, cfg.Location())
	if err != nil {
		return errors.New("-date must be YYYY-MM-DD")
	}
	var slot meals.Slot
	for _, w := range meals.New(cfg).Windows(day) {
		if w.Meal == *meal {
			slot = w
		}
	}
	if slot.Meal == "" {
		return errors.New("-meal must name a configured meal")
	}
	if !time.Now().Before(slot.End) {
		return errors.New("meal has already ended")
	}
	jid, err := types.ParseJID(*peer)
	if err != nil || jid.User == "" || (jid.Server != types.DefaultUserServer && jid.Server != types.HiddenUserServer) {
		return errors.New("-peer must be a personal WhatsApp JID")
	}
	jid = jid.ToNonAD()
	if !decimalAmount.MatchString(*amount) || strings.Trim(*amount, "0.") == "" {
		return errors.New("-amount must be a positive INR value with two decimal places")
	}
	// Normalize without floats so e.g. 060.00 matches the received 60.00.
	parts := strings.Split(*amount, ".")
	parts[0] = strings.TrimLeft(parts[0], "0")
	if parts[0] == "" {
		parts[0] = "0"
	}
	*amount = parts[0] + "." + parts[1]
	data, mime, width, height, err := readDeliveryImage(*qr)
	if err != nil {
		return err
	}
	imageHash := fmt.Sprintf("%x", sha256.Sum256(data))
	var portalClient *portal.Client
	var accountHash string
	if *portalQR {
		portalClient, err = portal.FromEnv()
		if err != nil {
			return err
		}
		current, err := portalClient.QRForMeal(ctx, *date, *meal, "")
		if err != nil {
			return err
		}
		accountHash = current.AccountHash
		// Bind restarts to the account, not the token that rotates daily.
		imageHash = "portal:" + accountHash
	}
	c, err := wa.Open(ctx, cfg, log)
	if err != nil {
		return err
	}
	if !c.LoggedIn() {
		return errors.New("not linked; run messwa login first")
	}
	if jid == c.WA.Store.GetJID().ToNonAD() || jid == c.WA.Store.GetLID().ToNonAD() {
		return errors.New("buyer cannot be your own account")
	}
	st, err := store.Open(cfg.DataDB())
	if err != nil {
		return err
	}
	defer st.Close()
	d := store.Delivery{Date: *date, Meal: *meal, Peer: jid.String(), Amount: *amount, ImageHash: imageHash, ArmedAt: time.Now()}
	armedAt, err := st.ArmDelivery(ctx, d)
	if err != nil {
		return err
	}
	// WhatsApp event timestamps have second precision. Start at the next second
	// so messages from just before the first arming cannot qualify.
	armedAt = armedAt.Add(time.Second)
	ctx, cancel := context.WithDeadline(ctx, slot.End)
	defer cancel()
	done := make(chan error, 1)
	var mu sync.Mutex
	attempted := false
	c.WA.AddEventHandler(func(evt any) {
		e, ok := evt.(*events.Message)
		if !ok {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if attempted || ctx.Err() != nil || e.Info.IsFromMe || e.Info.IsGroup {
			return
		}
		matches := e.Info.Chat.ToNonAD() == jid || e.Info.SenderAlt.ToNonAD() == jid
		if !matches && e.Info.Chat.Server == types.HiddenUserServer && jid.Server == types.DefaultUserServer {
			pn, lookupErr := c.WA.Store.LIDs.GetPNForLID(ctx, e.Info.Chat.ToNonAD())
			matches = lookupErr == nil && pn.ToNonAD() == jid
		}
		if !matches || !qualifiesForDelivery(e, d.Amount, armedAt, slot.End, time.Now()) {
			return
		}
		attempted = true
		id := c.WA.GenerateMessageID()
		err := deliverOnce(ctx, st, d, e.Info.ID, id, func() error {
			if portalClient != nil {
				current, err := portalClient.QRForMeal(ctx, d.Date, d.Meal, accountHash)
				if err != nil {
					return err
				}
				data, mime, width, height, err = deliveryImageBytes(current.PNG)
				if err != nil {
					return err
				}
			}
			log.Info("payment assumed; sending meal image", "date", d.Date, "meal", d.Meal, "amount", d.Amount)
			up, err := c.WA.Upload(ctx, data, whatsmeow.MediaImage)
			if err != nil {
				return err
			}
			_, err = c.WA.SendMessage(ctx, e.Info.Chat, &waE2E.Message{ImageMessage: &waE2E.ImageMessage{
				Caption: proto.String(fmt.Sprintf("%s %s", d.Date, d.Meal)), Mimetype: proto.String(mime), Width: proto.Uint32(uint32(width)), Height: proto.Uint32(uint32(height)),
				URL: &up.URL, DirectPath: &up.DirectPath, MediaKey: up.MediaKey, FileEncSHA256: up.FileEncSHA256, FileSHA256: up.FileSHA256, FileLength: &up.FileLength,
			}}, whatsmeow.SendRequestExtra{ID: id})
			return err
		})
		if err != nil {
			done <- err
			return
		}
		log.Info("meal image sent", "date", d.Date, "meal", d.Meal, "message_id", id)
		done <- nil
	})
	if err := c.WA.Connect(); err != nil {
		return err
	}
	defer c.WA.Disconnect()
	if *offer != "" {
		if !c.WA.WaitForConnection(20 * time.Second) {
			return errors.New("WhatsApp connection not ready; offer was not sent")
		}
		if _, err := c.WA.SendMessage(ctx, jid, &waE2E.Message{Conversation: proto.String(*offer)}); err != nil {
			return fmt.Errorf("offer send failed or is uncertain; inspect chat before retrying: %w", err)
		}
		log.Info("offer sent to selected buyer")
	}
	fmt.Fprintf(os.Stderr, "Armed %s %s for INR %s, ending %s. Only a new payment message from the selected buyer can trigger delivery. Payment is assumed, not bank-verified.\n", d.Date, d.Meal, d.Amount, slot.End.Format(time.RFC3339))
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func deliverOnce(ctx context.Context, st *store.Store, d store.Delivery, paymentID, messageID string, send func() error) error {
	claimed, err := st.ClaimDelivery(ctx, d, paymentID, messageID)
	if err != nil {
		return err
	}
	if !claimed {
		return errors.New("meal already claimed; no duplicate delivery attempted")
	}
	if err := send(); err != nil {
		return fmt.Errorf("delivery attempt failed or is uncertain; no automatic retry: %w", err)
	}
	if err := st.FinishDelivery(ctx, d, messageID); err != nil {
		return fmt.Errorf("image sent but completion could not be recorded: %w", err)
	}
	return nil
}

func qualifiesForDelivery(e *events.Message, amount string, armedAt, endsAt, now time.Time) bool {
	if e == nil || e.Info.IsFromMe || e.Info.IsGroup || e.Info.ID == "" || e.Info.Timestamp.Before(armedAt) || !e.Info.Timestamp.Before(endsAt) || !now.Before(endsAt) || e.Info.Timestamp.After(now.Add(5*time.Second)) {
		return false
	}
	if e.Info.Chat.Server != types.DefaultUserServer && e.Info.Chat.Server != types.HiddenUserServer {
		return false
	}
	p, err := decodePaymentPayload(e.Message.GetSendPaymentMessage().GetTransactionData())
	return err == nil && p.Currency == "INR" && p.Amount == amount && !p.Timestamp.Before(armedAt) && p.Timestamp.Before(endsAt) && !p.Timestamp.After(now.Add(5*time.Second))
}

func readDeliveryImage(path string) ([]byte, string, int, int, error) {
	data := placeholderQR
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return nil, "", 0, 0, err
		}
		defer f.Close()
		data, err = io.ReadAll(io.LimitReader(f, 5*1024*1024+1))
		if err != nil {
			return nil, "", 0, 0, err
		}
	}
	return deliveryImageBytes(data)
}

func deliveryImageBytes(data []byte) ([]byte, string, int, int, error) {
	if len(data) > 5*1024*1024 {
		return nil, "", 0, 0, errors.New("image exceeds 5 MiB")
	}
	info, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (format != "png" && format != "jpeg") || info.Width <= 0 || info.Height <= 0 || info.Width > 8192 || info.Height > 8192 {
		return nil, "", 0, 0, errors.New("image must be a valid PNG or JPEG, up to 8192 pixels per side")
	}
	return data, "image/" + format, info.Width, info.Height, nil
}
