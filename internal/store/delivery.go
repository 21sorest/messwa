package store

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const deliverySchema = `
CREATE TABLE IF NOT EXISTS qr_deliveries (
 date TEXT NOT NULL,
 meal TEXT NOT NULL,
 peer TEXT NOT NULL,
 amount TEXT NOT NULL,
 image_hash TEXT NOT NULL,
 armed_at INTEGER NOT NULL,
 state TEXT NOT NULL DEFAULT 'armed',
 payment_id TEXT UNIQUE,
 message_id TEXT NOT NULL DEFAULT '',
 PRIMARY KEY(date, meal)
);`

type Delivery struct {
	Date, Meal, Peer, Amount, ImageHash string
	ArmedAt                             time.Time
}

// An existing slot can only resume with the exact same buyer, price and image.
func (s *Store) ArmDelivery(ctx context.Context, d Delivery) (time.Time, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO qr_deliveries(date,meal,peer,amount,image_hash,armed_at)
 VALUES(?,?,?,?,?,?) ON CONFLICT(date,meal) DO NOTHING`, d.Date, d.Meal, d.Peer, d.Amount, d.ImageHash, d.ArmedAt.Unix())
	if err != nil {
		return time.Time{}, err
	}
	var peer, amount, hash, state string
	var armedAt int64
	err = s.db.QueryRowContext(ctx, `SELECT peer,amount,image_hash,state,armed_at FROM qr_deliveries WHERE date=? AND meal=?`, d.Date, d.Meal).Scan(&peer, &amount, &hash, &state, &armedAt)
	if err != nil {
		return time.Time{}, err
	}
	if peer != d.Peer || amount != d.Amount || hash != d.ImageHash {
		return time.Time{}, errors.New("meal already armed with different buyer, amount or image")
	}
	if state != "armed" {
		return time.Time{}, fmt.Errorf("meal delivery is %s; inspect the buyer's chat before any manual follow-up", state)
	}
	return time.Unix(armedAt, 0), nil
}

// Claim before network I/O. Uncertain sends deliberately remain 'sending' and
// are never retried automatically, even after a process restart.
func (s *Store) ClaimDelivery(ctx context.Context, d Delivery, paymentID, messageID string) (bool, error) {
	if paymentID == "" || messageID == "" {
		return false, errors.New("missing payment or outgoing message ID")
	}
	r, err := s.db.ExecContext(ctx, `UPDATE qr_deliveries SET state='sending',payment_id=?,message_id=?
 WHERE date=? AND meal=? AND peer=? AND amount=? AND image_hash=? AND state='armed'`, paymentID, messageID, d.Date, d.Meal, d.Peer, d.Amount, d.ImageHash)
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	return n == 1, err
}

func (s *Store) FinishDelivery(ctx context.Context, d Delivery, messageID string) error {
	r, err := s.db.ExecContext(ctx, `UPDATE qr_deliveries SET state='sent' WHERE date=? AND meal=? AND state='sending' AND message_id=?`, d.Date, d.Meal, messageID)
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("delivery state changed before completion was recorded")
	}
	return nil
}
