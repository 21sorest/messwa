package main

import (
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestConversationUnreadablePayment(t *testing.T) {
	for _, payload := range []string{"", "unreadable payload"} {
		text := conversationPaymentText(&waE2E.Message{SendPaymentMessage: &waE2E.SendPaymentMessage{TransactionData: proto.String(payload)}})
		if !strings.HasPrefix(text, "Amount unavailable.") || strings.Contains(text, "Reported amount:") {
			t.Fatalf("unreadable payment shows an amount: %s", text)
		}
	}
}
