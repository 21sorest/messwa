package store

import (
	"context"
	"time"
)

// The group log is every event from the market groups, unfiltered. Listings
// are derived from it, so after changing the tagging rules you can wipe the
// derived tables and replay the log.
const logSchema = `
CREATE TABLE IF NOT EXISTS group_log (
	chat       TEXT NOT NULL,
	id         TEXT NOT NULL,
	type       TEXT NOT NULL,
	sender     TEXT NOT NULL,
	sender_alt TEXT NOT NULL DEFAULT '',
	push_name  TEXT NOT NULL DEFAULT '',
	ts         INTEGER NOT NULL,
	text       TEXT NOT NULL DEFAULT '',
	reply_to   TEXT NOT NULL DEFAULT '',
	target     TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (chat, id, type)
);
CREATE INDEX IF NOT EXISTS group_log_ts ON group_log(ts);
`

// LogEntry is one event in a market group. For reactions Text is the emoji
// (empty when removed) and Target is the reacted-to message. For edits and
// deletes Target is the affected message.
type LogEntry struct {
	Chat      string    `json:"chat"`
	ChatName  string    `json:"chat_name,omitempty"`
	ID        string    `json:"id"`
	Type      string    `json:"type"` // post, reaction, edit, delete
	Sender    string    `json:"sender"`
	SenderAlt string    `json:"sender_alt,omitempty"`
	PushName  string    `json:"push_name,omitempty"`
	TS        time.Time `json:"ts"`
	Text      string    `json:"text,omitempty"`
	ReplyTo   string    `json:"reply_to,omitempty"`
	Target    string    `json:"target,omitempty"`
}

// AppendLog stores an event and reports whether it was new.
func (s *Store) AppendLog(ctx context.Context, e LogEntry) (bool, error) {
	res, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO group_log(chat,id,type,sender,sender_alt,push_name,ts,text,reply_to,target) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		e.Chat, e.ID, e.Type, e.Sender, e.SenderAlt, e.PushName, e.TS.Unix(), e.Text, e.ReplyTo, e.Target)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// LogLine is a log entry plus the listing its post landed in, if any.
type LogLine struct {
	LogEntry
	ListingID int64
}

func (s *Store) Log(ctx context.Context, since time.Time) ([]LogLine, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT g.chat,g.id,g.type,g.sender,g.sender_alt,g.push_name,g.ts,g.text,g.reply_to,g.target,coalesce(p.listing_id,0)
		FROM group_log g LEFT JOIN posts p ON g.type='post' AND p.chat=g.chat AND p.id=g.id
		WHERE g.ts >= ? ORDER BY g.ts, g.rowid`, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LogLine
	for rows.Next() {
		var l LogLine
		var ts int64
		if err := rows.Scan(&l.Chat, &l.ID, &l.Type, &l.Sender, &l.SenderAlt, &l.PushName, &ts, &l.Text, &l.ReplyTo, &l.Target, &l.ListingID); err != nil {
			return nil, err
		}
		l.TS = time.Unix(ts, 0)
		out = append(out, l)
	}
	return out, rows.Err()
}

// ResetDerived deletes listings, posts and reactions so the log can be replayed.
func (s *Store) ResetDerived(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM posts; DELETE FROM reactions; DELETE FROM listings;`)
	return err
}

// SetGroupName records a group's name without touching its admin list.
func (s *Store) SetGroupName(ctx context.Context, jid, name string) error {
	if name == "" {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO groups(jid,name) VALUES(?,?) ON CONFLICT(jid) DO UPDATE SET name=excluded.name`, jid, name)
	return err
}

func (s *Store) pruneLog(ctx context.Context, before time.Time) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM group_log WHERE ts < ?`, before.Unix())
	return err
}
