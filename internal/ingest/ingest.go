// Package ingest turns chat events into listings and DM records. It decides
// what gets stored; anything it drops never reaches the MCP server.
package ingest

import (
	"context"
	"strings"
	"time"

	"messwa/internal/classify"
	"messwa/internal/config"
	"messwa/internal/meals"
	"messwa/internal/store"
)

type Ingester struct {
	cfg    *config.Config
	st     *store.Store
	sched  *meals.Schedule
	meals  []classify.Named
	messes []classify.Named
	groups map[string]bool
}

func New(cfg *config.Config, st *store.Store) *Ingester {
	in := &Ingester{cfg: cfg, st: st, sched: meals.New(cfg), groups: map[string]bool{}}
	for _, m := range cfg.Meals {
		in.meals = append(in.meals, classify.Named{Name: m.Name, Phrases: m.Keywords})
	}
	for _, m := range cfg.Messes {
		in.messes = append(in.messes, classify.Named{Name: m.Name, Phrases: m.Aliases})
	}
	for _, g := range cfg.Groups {
		in.groups[g] = true
	}
	return in
}

func (in *Ingester) IsMarketGroup(jid string) bool { return in.groups[jid] }

// GroupPost is a new message in one of the market groups.
type GroupPost struct {
	Chat      string
	ID        string
	Sender    string // primary JID (phone number when known)
	SenderAlt string // the other identity (LID or phone number), if known
	PushName  string
	TS        time.Time
	Text      string
	ReplyTo   string // ID of the quoted message, if any
}

func (in *Ingester) Post(ctx context.Context, p GroupPost) error {
	if !in.groups[p.Chat] || strings.TrimSpace(p.Text) == "" {
		return nil
	}
	if seen, err := in.st.HasPost(ctx, p.Chat, p.ID); err != nil || seen {
		return err
	}
	if p.ReplyTo != "" {
		handled, err := in.soldReply(ctx, p)
		if err != nil || handled {
			return err
		}
	}

	l := in.classify(p.Text, p.TS)
	if l.Kind == "" && l.MealSource != "keyword" {
		return nil // chatter, not a listing
	}
	l.Seller, l.SellerAlt, l.SellerName = p.Sender, p.SenderAlt, p.PushName
	l.FirstSeen, l.LastSeen = p.TS, p.TS

	match, err := in.findDuplicate(ctx, &l, p)
	if err != nil {
		return err
	}
	if match == nil {
		if err := in.st.CreateListing(ctx, &l); err != nil {
			return err
		}
		match = &l
	}
	return in.st.AddPost(ctx, store.Post{Chat: p.Chat, ID: p.ID, ListingID: match.ID, Sender: p.Sender, TS: p.TS, Text: p.Text})
}

// soldReply handles the seller replying "sold" to their own post.
func (in *Ingester) soldReply(ctx context.Context, p GroupPost) (bool, error) {
	l, err := in.st.PostListing(ctx, p.Chat, p.ReplyTo)
	if err != nil || l == nil || !sameUser(l, p.Sender, p.SenderAlt) {
		return false, err
	}
	t := classify.Prepare(p.Text)
	if len(strings.Fields(classify.Normalize(p.Text))) > 6 || !t.Has(in.cfg.Sold.Words...) {
		return false, nil
	}
	return true, in.st.MarkSoldByText(ctx, l.ID, p.TS)
}

// findDuplicate treats posts as one listing when the same person offers the
// same kind of deal for the same meal slot within the dedupe window. People
// hold one registration per meal, so wording differences don't matter. A
// listing already marked sold never absorbs a new post: a repost after a
// buyer backs out should show as available.
func (in *Ingester) findDuplicate(ctx context.Context, l *store.Listing, p GroupPost) (*store.Listing, error) {
	recent, err := in.st.RecentBySeller(ctx, []string{p.Sender, p.SenderAlt}, p.TS.Add(-in.cfg.Dedupe.Window.Duration))
	if err != nil {
		return nil, err
	}
	rule := store.SoldRule{Emojis: in.cfg.Sold.Emojis, AdminsCount: in.cfg.Sold.AdminsCount}
	for i := range recent {
		r := &recent[i]
		if r.Meal != l.Meal || r.MealDate != l.MealDate || r.Kind != l.Kind {
			continue
		}
		if r.Mess != "" && l.Mess != "" && r.Mess != l.Mess {
			continue
		}
		if sold, err := in.st.IsSold(ctx, rule, r); err != nil || sold {
			if err != nil {
				return nil, err
			}
			continue
		}
		return r, nil
	}
	return nil, nil
}

// classify fills in everything derivable from the text and post time.
func (in *Ingester) classify(text string, ts time.Time) store.Listing {
	t := classify.Prepare(text)
	l := store.Listing{
		Text: text,
		Norm: classify.Normalize(text),
		Kind: classify.Kind(t),
		Diet: classify.Diet(t),
	}
	if m := classify.Pick(t, in.messes); len(m) > 0 {
		l.Mess = m[0]
	}
	var slot meals.Slot
	var ok bool
	for _, name := range classify.Pick(t, in.meals) {
		// Several meals named: the listing is for whichever comes first.
		if s, found := in.sched.Next(ts, name); found && (!ok || s.End.Before(slot.End)) {
			slot, ok = s, true
		}
	}
	l.MealSource = "keyword"
	if !ok {
		slot, ok = in.sched.Next(ts, "")
		l.MealSource = "time"
	}
	if ok {
		l.Meal, l.MealDate, l.MealEnd = slot.Meal, slot.Date, slot.End
	}
	return l
}

// Edit handles a sender editing one of their posts.
func (in *Ingester) Edit(ctx context.Context, chat, id, editor, editorAlt, text string, at time.Time) error {
	if !in.groups[chat] {
		return nil
	}
	l, err := in.st.PostListing(ctx, chat, id)
	if err != nil || l == nil || !sameUser(l, editor, editorAlt) {
		return err
	}
	if err := in.st.EditPost(ctx, chat, id, text, at); err != nil {
		return err
	}
	if classify.Prepare(text).Has(in.cfg.Sold.Words...) {
		if err := in.st.MarkSoldByText(ctx, l.ID, at); err != nil {
			return err
		}
	}
	// Re-derive tags from the edited text but keep old ones the edit dropped,
	// so "SOLD" replacing the whole post doesn't erase what was sold.
	n := in.classify(text, l.FirstSeen)
	if n.Kind == "" {
		n.Kind = l.Kind
	}
	if n.MealSource != "keyword" {
		n.Meal, n.MealSource, n.MealDate, n.MealEnd = l.Meal, l.MealSource, l.MealDate, l.MealEnd
	}
	if n.Mess == "" {
		n.Mess = l.Mess
	}
	if n.Diet == "" {
		n.Diet = l.Diet
	}
	n.ID = l.ID
	return in.st.UpdateListing(ctx, &n)
}

func (in *Ingester) Delete(ctx context.Context, chat, id string) error {
	if !in.groups[chat] {
		return nil
	}
	return in.st.DeletePost(ctx, chat, id)
}

func (in *Ingester) Reaction(ctx context.Context, chat, target, reactor, reactorAlt, emoji string, at time.Time) error {
	if !in.groups[chat] {
		return nil
	}
	return in.st.SetReaction(ctx, chat, target, reactor, reactorAlt, emoji, at)
}

// DirectMessage is one message in a one-to-one chat.
type DirectMessage struct {
	Peer     string // primary JID of the other person
	PeerAlt  string
	PushName string // only set for incoming messages
	FromMe   bool
	ID       string
	TS       time.Time
	Text     string
	Saved    bool // the peer is in your address book
}

// DM records activity for every DM, but content only for unsaved numbers.
func (in *Ingester) DM(ctx context.Context, m DirectMessage) error {
	if err := in.st.TouchPeer(ctx, m.Peer, m.PeerAlt, m.PushName, m.FromMe, m.TS); err != nil {
		return err
	}
	if m.Saved {
		return in.st.DropDMContent(ctx, m.Peer)
	}
	if strings.TrimSpace(m.Text) == "" {
		return nil
	}
	return in.st.AddDM(ctx, m.Peer, m.ID, m.FromMe, m.TS, m.Text)
}

func sameUser(l *store.Listing, jid, alt string) bool {
	for _, a := range []string{jid, alt} {
		if a != "" && (a == l.Seller || a == l.SellerAlt) {
			return true
		}
	}
	return false
}
