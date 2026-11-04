package dashboard

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// beeperIndex finds group posts in Beeper's local index.db, so the dashboard
// can link each copy of a post to the same message in Beeper Desktop.
//
// Beeper's room IDs and sortKeys are the same on every device signed in to
// the account, so a link built from Beeper Server's database on this machine
// opens the right message in Beeper Desktop on a laptop.
type beeperIndex struct {
	path string

	once sync.Once
	db   *sql.DB
	err  error
}

func newBeeperIndex(path string) *beeperIndex {
	if path == "" {
		return nil
	}
	return &beeperIndex{path: path}
}

func (b *beeperIndex) open() (*sql.DB, error) {
	b.once.Do(func() {
		u := url.URL{Scheme: "file", Path: b.path, RawQuery: "mode=ro&_pragma=query_only(1)&_pragma=busy_timeout(2000)"}
		b.db, b.err = sql.Open("sqlite", u.String())
		if b.err == nil {
			b.db.SetMaxOpenConns(2)
		}
	})
	return b.db, b.err
}

type beeperPost struct {
	Chat string // group JID, xxxx@g.us
	TS   time.Time
}

// Links returns a beeper:// link for each post it finds, keyed by index into
// posts. senders are the poster's JIDs, phone and LID forms; they pick the
// right message when several land in the same second. Posts Beeper doesn't
// have, such as ones its 24-hour disappearing timer already removed, get no
// link.
func (b *beeperIndex) Links(ctx context.Context, posts []beeperPost, senders []string) (map[int]string, error) {
	db, err := b.open()
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, s := range senders {
		if c := beeperContact(s); c != "" {
			ids[c] = true
		}
	}
	type room struct{ id, account string }
	rooms := map[string]*room{}
	out := map[int]string{}
	for i, p := range posts {
		r, ok := rooms[p.Chat]
		if !ok {
			var rm room
			err := db.QueryRowContext(ctx, `SELECT threadID, coalesce(accountID, '') FROM threads
				WHERE json_extract(thread, '$.extra.bridge.channel.id') = ? LIMIT 1`, p.Chat).Scan(&rm.id, &rm.account)
			switch {
			case err == sql.ErrNoRows:
				r = nil
			case err != nil:
				return nil, fmt.Errorf("beeper room for %s: %w", p.Chat, err)
			default:
				r = &rm
			}
			rooms[p.Chat] = r
		}
		if r == nil {
			continue
		}
		ms := p.TS.Unix() * 1000
		rows, err := db.QueryContext(ctx, `SELECT hsOrder, senderContactID FROM mx_room_messages
			WHERE roomID = ? AND timestamp BETWEEN ? AND ? AND isDeleted = 0 AND type NOT IN ('HIDDEN', 'REACTION')`, r.id, ms, ms+999)
		if err != nil {
			return nil, fmt.Errorf("beeper messages: %w", err)
		}
		var found, any int64
		n := 0
		for rows.Next() {
			var order int64
			var sender string
			if err := rows.Scan(&order, &sender); err != nil {
				rows.Close()
				return nil, err
			}
			n++
			any = order
			if ids[sender] {
				found = order
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		// One message in that second is the post even if the sender ID
		// didn't match, which happens when Beeper only knows one of the
		// person's two IDs.
		if found == 0 && n == 1 {
			found = any
		}
		if found == 0 {
			continue
		}
		link := fmt.Sprintf("beeper://select-thread/whatsapp/%s/%d", r.id, found)
		if r.account != "" {
			link += "?accountID=" + url.QueryEscape(r.account)
		}
		out[i] = link
	}
	return out, nil
}

// beeperContact turns a WhatsApp JID into the user ID Beeper's WhatsApp
// bridge gives that person: 123@lid becomes @whatsapp_lid-123:beeper.local
// and 9198...@s.whatsapp.net becomes @whatsapp_9198...:beeper.local.
func beeperContact(jid string) string {
	user, server, ok := strings.Cut(jid, "@")
	if !ok || user == "" {
		return ""
	}
	user, _, _ = strings.Cut(user, ":") // device suffix
	switch server {
	case "lid":
		return "@whatsapp_lid-" + user + ":beeper.local"
	case "s.whatsapp.net":
		return "@whatsapp_" + user + ":beeper.local"
	}
	return ""
}
