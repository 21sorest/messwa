package dashboard

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

func TestBeeperContact(t *testing.T) {
	for in, want := range map[string]string{
		"238942368575599@lid":            "@whatsapp_lid-238942368575599:beeper.local",
		"919392803070@s.whatsapp.net":    "@whatsapp_919392803070:beeper.local",
		"919392803070:12@s.whatsapp.net": "@whatsapp_919392803070:beeper.local",
		"120363416069274282@g.us":        "",
		"":                               "",
	} {
		if got := beeperContact(in); got != want {
			t.Errorf("beeperContact(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBeeperLinks(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE threads (threadID TEXT PRIMARY KEY, accountID TEXT, thread JSON NOT NULL)`,
		`CREATE TABLE mx_room_messages (roomID TEXT, senderContactID TEXT, timestamp INTEGER, isDeleted INTEGER, type TEXT, hsOrder INTEGER)`,
		`INSERT INTO threads VALUES ('!room:beeper.local', 'whatsapp.91999', '{"extra":{"bridge":{"channel":{"id":"111@g.us"}}}}')`,
		`INSERT INTO threads VALUES ('!other:beeper.local', 'whatsapp.91999', '{"extra":{"bridge":{"channel":{"id":"222@g.us"}}}}')`,
		// Two messages in the same second; the seller's is 501.
		`INSERT INTO mx_room_messages VALUES ('!room:beeper.local', '@whatsapp_lid-77:beeper.local', 1793782779000, 0, 'TEXT', 500)`,
		`INSERT INTO mx_room_messages VALUES ('!room:beeper.local', '@whatsapp_lid-55:beeper.local', 1793782779000, 0, 'TEXT', 501)`,
		// Alone in its second, sender unknown to us: still the post.
		`INSERT INTO mx_room_messages VALUES ('!other:beeper.local', '@whatsapp_lid-99:beeper.local', 1793782800000, 0, 'TEXT', 900)`,
		// Deleted, so no link.
		`INSERT INTO mx_room_messages VALUES ('!room:beeper.local', '@whatsapp_lid-55:beeper.local', 1793782900000, 1, 'TEXT', 950)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatal(q, err)
		}
	}
	db.Close()

	b := newBeeperIndex(path)
	posts := []beeperPost{
		{Chat: "111@g.us", TS: time.Unix(1793782779, 0)},
		{Chat: "222@g.us", TS: time.Unix(1793782800, 0)},
		{Chat: "111@g.us", TS: time.Unix(1793782900, 0)},
		{Chat: "333@g.us", TS: time.Unix(1793782779, 0)}, // a group Beeper doesn't have
	}
	links, err := b.Links(context.Background(), posts, []string{"9198@s.whatsapp.net", "55@lid"})
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]string{
		0: "beeper://select-thread/whatsapp/!room:beeper.local/501?accountID=whatsapp.91999",
		1: "beeper://select-thread/whatsapp/!other:beeper.local/900?accountID=whatsapp.91999",
	}
	if len(links) != len(want) {
		t.Fatalf("links = %v, want %v", links, want)
	}
	for i, w := range want {
		if links[i] != w {
			t.Errorf("links[%d] = %q, want %q", i, links[i], w)
		}
	}
}
