package mcpserver

import (
	"context"
	"errors"
	"messwa/internal/store"
)

// ConversationPeer accepts only a buyer key from this account's sale view.
func (s *Server) ConversationPeer(ctx context.Context, id int64, key string) (string, error) {
	t, err := s.st.GetSellTask(ctx, id)
	if err != nil {
		return "", err
	}
	if t == nil {
		return "", errors.New("sale not found")
	}
	if s.boundUser != 0 && t.UserID != s.boundUser {
		return "", errors.New("sale not found")
	}
	buyers, err := s.st.SellTaskBuyers(ctx, id)
	if err != nil {
		return "", err
	}
	for _, b := range buyers {
		p := s.person(ctx, b.Peer, b.Alt, "")
		if firstOf(p.Phone, b.Peer) == key {
			return b.Peer, nil
		}
	}
	return "", errors.New("buyer not found")
}
func (s *Server) SaleConversation(ctx context.Context, id int64, key string) ([]store.SaleMessage, error) {
	peer, err := s.ConversationPeer(ctx, id, key)
	if err != nil {
		return nil, err
	}
	return s.st.SaleMessages(ctx, id, peer)
}
func (s *Server) ConversationMedia(ctx context.Context, id int64, key, msg string) ([]byte, string, error) {
	peer, err := s.ConversationPeer(ctx, id, key)
	if err != nil {
		return nil, "", err
	}
	return s.st.SaleMessageMedia(ctx, id, peer, msg)
}
