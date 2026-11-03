package main

import (
	"strings"
	"testing"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"google.golang.org/protobuf/proto"
)

func TestPaymentFields(t *testing.T) {
	m := &waE2E.Message{Conversation: proto.String("private text"), SendPaymentMessage: &waE2E.SendPaymentMessage{TransactionData: proto.String("test transaction")}}
	names, fields, err := paymentFields(m)
	if err != nil || len(names) != 2 || len(fields) != 1 || !strings.Contains(string(fields["sendPaymentMessage"]), "test transaction") {
		t.Fatalf("unexpected diagnostic: %v %v %v", names, fields, err)
	}
	for _, data := range fields {
		if strings.Contains(string(data), "private text") {
			t.Fatal("ordinary message text leaked")
		}
	}
	_, fields, err = paymentFields(&waE2E.Message{Conversation: proto.String("Unknown message type, please view it on the WhatsApp app")})
	if err != nil || len(fields) != 0 {
		t.Fatal("plain text was classified as payment")
	}
	_, fields, err = paymentFields(nil)
	if err != nil || len(fields) != 0 {
		t.Fatal("nil message")
	}
}
