package store

import (
	"context"
	"fmt"
	"time"
)

// A gala dinner is the special dinner every mess serves once or twice a
// semester, each with its own menu. It sells for far more than a normal
// dinner, so the insights page keeps it out of its usual numbers and the
// price medians leave it out. gala_dinners holds the dates an admin marked;
// it lives with the market tables, so every account sees the same dates.
const galaSchema = `
CREATE TABLE IF NOT EXISTS gala_dinners (
	date      TEXT PRIMARY KEY,
	marked_at INTEGER NOT NULL
);`

// GalaMeal is the meal a gala date applies to.
const GalaMeal = "dinner"

// galaFact is true for a listing_facts row that is a gala dinner.
const galaFact = `(meal = 'dinner' AND meal_date IN (SELECT date FROM gala_dinners))`

// GalaDinners returns every marked gala dinner date, oldest first.
func (s *Store) GalaDinners(ctx context.Context) ([]string, error) {
	rows, err := s.boardDB().QueryContext(ctx, `SELECT date FROM gala_dinners ORDER BY date`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// SetGalaDinner marks date's dinner as a gala dinner, or unmarks it.
func (s *Store) SetGalaDinner(ctx context.Context, date string, gala bool) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return fmt.Errorf("date must be YYYY-MM-DD")
	}
	var err error
	if gala {
		_, err = s.boardDB().ExecContext(ctx, `INSERT OR IGNORE INTO gala_dinners(date, marked_at) VALUES(?, ?)`, date, time.Now().Unix())
	} else {
		_, err = s.boardDB().ExecContext(ctx, `DELETE FROM gala_dinners WHERE date = ?`, date)
	}
	return err
}

// GalaFacts returns the facts of every marked gala dinner, oldest first.
func (s *Store) GalaFacts(ctx context.Context) ([]Fact, error) {
	return s.queryFacts(ctx, `WHERE `+galaFact)
}
