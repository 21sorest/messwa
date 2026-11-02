package ingest

import (
	"context"
	"fmt"
	"time"

	"messwa/internal/store"
)

// Handle logs a market-group event and applies it. Events from other groups
// are ignored. Replays of an already-logged event are no-ops.
func (in *Ingester) Handle(ctx context.Context, e store.LogEntry) error {
	if !in.groups[e.Chat] {
		return nil
	}
	if e.ChatName != "" {
		if err := in.st.SetGroupName(ctx, e.Chat, e.ChatName); err != nil {
			return err
		}
	}
	isNew, err := in.st.AppendLog(ctx, e)
	if err != nil || !isNew {
		return err
	}
	if err := in.apply(ctx, e); err != nil {
		return err
	}
	if in.change != nil {
		in.change()
	}
	return nil
}

func (in *Ingester) apply(ctx context.Context, e store.LogEntry) error {
	switch e.Type {
	case "post":
		return in.Post(ctx, GroupPost{Chat: e.Chat, ID: e.ID, Sender: e.Sender, SenderAlt: e.SenderAlt, PushName: e.PushName, TS: e.TS, Text: e.Text, ReplyTo: e.ReplyTo})
	case "reaction":
		return in.Reaction(ctx, e.Chat, e.Target, e.Sender, e.SenderAlt, e.Text, e.TS)
	case "edit":
		return in.Edit(ctx, e.Chat, e.Target, e.Sender, e.SenderAlt, e.Text, e.TS)
	case "delete":
		return in.Delete(ctx, e.Chat, e.Target)
	}
	return fmt.Errorf("unknown event type %q", e.Type)
}

// Rebuild throws away all listings and re-derives them from the group log,
// for use after changing the tagging rules.
func (in *Ingester) Rebuild(ctx context.Context) (int, error) {
	if err := in.st.ResetDerived(ctx); err != nil {
		return 0, err
	}
	log, err := in.st.Log(ctx, time.Unix(0, 0))
	if err != nil {
		return 0, err
	}
	for _, l := range log {
		if !in.groups[l.Chat] {
			continue
		}
		if err := in.apply(ctx, l.LogEntry); err != nil {
			return 0, err
		}
	}
	return len(log), nil
}
