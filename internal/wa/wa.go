// Package wa owns the WhatsApp connection. It converts whatsmeow events into
// ingest calls and drops everything outside the allowed chats on arrival.
//
// This client never sends read receipts or presence, so your phone keeps
// showing messages as unread and notifications behave as before.
package wa

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/mdp/qrterminal/v3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"messwa/internal/config"
	"messwa/internal/ingest"
	mstore "messwa/internal/store"
)

type Client struct {
	WA  *whatsmeow.Client
	cfg *config.Config
	log *slog.Logger
}

func Open(ctx context.Context, cfg *config.Config, log *slog.Logger) (*Client, error) {
	store.SetOSInfo("messwa", [3]uint32{0, 1, 0})
	container, err := sqlstore.New(ctx, "sqlite", "file:"+cfg.SessionDB()+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", waLog.Noop)
	if err != nil {
		return nil, fmt.Errorf("session store: %w", err)
	}
	dev, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{WA: whatsmeow.NewClient(dev, waLog.Noop), cfg: cfg, log: log}, nil
}

func (c *Client) LoggedIn() bool { return c.WA.Store.ID != nil }

// Login links this program as a companion device, by QR code or, when
// phone is given (country code + number, digits only), by pairing code.
func (c *Client) Login(ctx context.Context, phone string) error {
	if c.LoggedIn() {
		return fmt.Errorf("already linked as %s; delete %s to relink", c.WA.Store.ID, c.cfg.SessionDB())
	}
	done := make(chan struct{})
	c.WA.AddEventHandler(func(evt any) {
		if _, ok := evt.(*events.PairSuccess); ok {
			close(done)
		}
	})
	if phone == "" {
		qr, err := c.WA.GetQRChannel(ctx)
		if err != nil {
			return err
		}
		if err := c.WA.Connect(); err != nil {
			return err
		}
		for item := range qr {
			switch item.Event {
			case "code":
				fmt.Println("\nWhatsApp > Settings > Linked devices > Link a device, then scan:")
				qrterminal.GenerateHalfBlock(item.Code, qrterminal.L, os.Stdout)
			case "success":
				fmt.Println("Linked.")
			default:
				if item.Error != nil {
					return item.Error
				}
				if item.Event != "success" {
					return fmt.Errorf("pairing: %s", item.Event)
				}
			}
		}
	} else {
		if err := c.WA.Connect(); err != nil {
			return err
		}
		code, err := c.WA.PairPhone(ctx, phone, true, whatsmeow.PairClientChrome, "Chrome (Linux)")
		if err != nil {
			return err
		}
		fmt.Printf("On your phone: Linked devices > Link a device > Link with phone number instead, then enter %s\n", code)
	}
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	// Give the initial sync a moment so contacts and LID mappings land.
	time.Sleep(10 * time.Second)
	c.WA.Disconnect()
	return nil
}

type GroupSummary struct {
	JID, Name    string
	Participants int
}

func (c *Client) Groups(ctx context.Context) ([]GroupSummary, error) {
	gs, err := c.WA.GetJoinedGroups(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]GroupSummary, 0, len(gs))
	for _, g := range gs {
		out = append(out, GroupSummary{g.JID.String(), g.Name, len(g.Participants)})
	}
	return out, nil
}

// Saved reports whether jid (or its alternate) is in your address book, and
// the best display name for it.
func (c *Client) Saved(ctx context.Context, jids ...string) (bool, string) {
	name := ""
	for _, s := range jids {
		if s == "" {
			continue
		}
		j, err := types.ParseJID(s)
		if err != nil {
			continue
		}
		ci, err := c.WA.Store.Contacts.GetContact(ctx, j)
		if err != nil || !ci.Found {
			continue
		}
		if ci.FullName != "" || ci.FirstName != "" {
			return true, firstNonEmpty(ci.FullName, ci.FirstName)
		}
		name = firstNonEmpty(name, ci.PushName, ci.BusinessName)
	}
	return false, name
}

// Ingest wires events into the ingester. Call before Connect.
func (c *Client) Ingest(in *ingest.Ingester, st *mstore.Store) {
	c.WA.AddEventHandler(func(evt any) {
		ctx := context.Background()
		var err error
		switch e := evt.(type) {
		case *events.Message:
			err = c.message(ctx, in, e)
		case *events.HistorySync:
			c.history(ctx, in, e)
		case *events.Connected:
			go c.refreshGroups(ctx, st)
		case *events.GroupInfo:
			if in.IsMarketGroup(e.JID.String()) {
				go c.refreshGroups(ctx, st)
			}
		case *events.LoggedOut:
			c.log.Error("WhatsApp logged this device out; run `messwa login` again")
			os.Exit(1)
		}
		if err != nil {
			c.log.Error("ingest", "err", err)
		}
	})
}

func (c *Client) refreshGroups(ctx context.Context, st *mstore.Store) {
	for _, g := range c.cfg.Groups {
		jid, err := types.ParseJID(g)
		if err != nil {
			c.log.Error("bad group JID in config", "jid", g)
			continue
		}
		info, err := c.WA.GetGroupInfo(ctx, jid)
		if err != nil {
			c.log.Warn("group info", "jid", g, "err", err)
			continue
		}
		var admins []string
		for _, p := range info.Participants {
			if p.IsAdmin || p.IsSuperAdmin {
				for _, j := range []types.JID{p.JID, p.PhoneNumber, p.LID} {
					if !j.IsEmpty() {
						admins = append(admins, j.ToNonAD().String())
					}
				}
			}
		}
		if err := st.SetGroup(ctx, g, info.Name, admins); err != nil {
			c.log.Error("save group", "err", err)
		}
	}
}

// history replays the initial history sync for allowed chats, skipping
// anything older than two days since expired listings are useless.
func (c *Client) history(ctx context.Context, in *ingest.Ingester, e *events.HistorySync) {
	cutoff := uint64(time.Now().Add(-48 * time.Hour).Unix())
	for _, conv := range e.Data.GetConversations() {
		chat, err := types.ParseJID(conv.GetID())
		if err != nil || !c.allowedChat(in, chat) {
			continue
		}
		for _, hm := range conv.GetMessages() {
			wm := hm.GetMessage()
			if wm.GetMessageTimestamp() < cutoff {
				continue
			}
			evt, err := c.WA.ParseWebMessage(chat, wm)
			if err != nil {
				continue
			}
			if err := c.message(ctx, in, evt); err != nil {
				c.log.Error("history", "err", err)
			}
		}
	}
}

func (c *Client) allowedChat(in *ingest.Ingester, chat types.JID) bool {
	switch chat.Server {
	case types.GroupServer:
		return in.IsMarketGroup(chat.String())
	case types.DefaultUserServer, types.HiddenUserServer:
		own := c.WA.Store.GetJID().ToNonAD()
		lid := c.WA.Store.GetLID().ToNonAD()
		return chat.ToNonAD() != own && chat.ToNonAD() != lid // skip notes-to-self
	}
	return false // status, newsletters, broadcast lists
}

func (c *Client) message(ctx context.Context, in *ingest.Ingester, e *events.Message) error {
	info := e.Info
	if !c.allowedChat(in, info.Chat) {
		return nil
	}
	m := e.Message
	if info.IsGroup {
		sender, alt := c.identity(ctx, info.Sender, info.SenderAlt)
		chat := info.Chat.String()
		if r := m.GetReactionMessage(); r != nil {
			return in.Reaction(ctx, chat, r.GetKey().GetID(), sender, alt, r.GetText(), info.Timestamp)
		}
		if pm := m.GetProtocolMessage(); pm != nil {
			switch pm.GetType() {
			case waE2E.ProtocolMessage_REVOKE:
				return in.Delete(ctx, chat, pm.GetKey().GetID())
			case waE2E.ProtocolMessage_MESSAGE_EDIT:
				return in.Edit(ctx, chat, pm.GetKey().GetID(), sender, alt, textOf(pm.GetEditedMessage()), info.Timestamp)
			}
			return nil
		}
		return in.Post(ctx, ingest.GroupPost{
			Chat: chat, ID: info.ID, Sender: sender, SenderAlt: alt, PushName: info.PushName,
			TS: info.Timestamp, Text: textOf(m), ReplyTo: replyTo(m),
		})
	}

	if m.GetReactionMessage() != nil || m.GetProtocolMessage() != nil {
		return nil
	}
	peerAlt := info.SenderAlt
	if info.IsFromMe {
		peerAlt = info.RecipientAlt
	}
	peer, alt := c.identity(ctx, info.Chat, peerAlt)
	saved, _ := c.Saved(ctx, peer, alt)
	push := ""
	if !info.IsFromMe {
		push = info.PushName
	}
	return in.DM(ctx, ingest.DirectMessage{
		Peer: peer, PeerAlt: alt, PushName: push, FromMe: info.IsFromMe,
		ID: info.ID, TS: info.Timestamp, Text: textOf(m), Saved: saved,
	})
}

// identity returns (phone-number JID if known, LID) or the reverse when
// only the LID is known. Device suffixes are dropped.
func (c *Client) identity(ctx context.Context, jid, alt types.JID) (string, string) {
	jid, alt = jid.ToNonAD(), alt.ToNonAD()
	if jid.Server == types.HiddenUserServer {
		if alt.IsEmpty() {
			if pn, err := c.WA.Store.LIDs.GetPNForLID(ctx, jid); err == nil && !pn.IsEmpty() {
				alt = pn.ToNonAD()
			}
		}
		if !alt.IsEmpty() {
			jid, alt = alt, jid
		}
	} else if alt.IsEmpty() && jid.Server == types.DefaultUserServer {
		if lid, err := c.WA.Store.LIDs.GetLIDForPN(ctx, jid); err == nil && !lid.IsEmpty() {
			alt = lid.ToNonAD()
		}
	}
	a := ""
	if !alt.IsEmpty() {
		a = alt.String()
	}
	return jid.String(), a
}

func textOf(m *waE2E.Message) string {
	switch {
	case m == nil:
		return ""
	case m.GetConversation() != "":
		return m.GetConversation()
	case m.GetExtendedTextMessage() != nil:
		return m.GetExtendedTextMessage().GetText()
	case m.GetImageMessage() != nil:
		return strings.TrimSpace("[image] " + m.GetImageMessage().GetCaption())
	case m.GetVideoMessage() != nil:
		return strings.TrimSpace("[video] " + m.GetVideoMessage().GetCaption())
	case m.GetDocumentMessage() != nil:
		return strings.TrimSpace("[document] " + m.GetDocumentMessage().GetCaption())
	case m.GetAudioMessage() != nil:
		return "[voice note]"
	case m.GetStickerMessage() != nil:
		return "[sticker]"
	}
	return ""
}

func replyTo(m *waE2E.Message) string {
	if ci := m.GetExtendedTextMessage().GetContextInfo(); ci != nil {
		return ci.GetStanzaID()
	}
	return m.GetImageMessage().GetContextInfo().GetStanzaID()
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s != "" {
			return s
		}
	}
	return ""
}
