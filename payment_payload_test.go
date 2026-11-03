package main

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"unicode/utf16"
)

// Only the non-identifying prefix of the received test payment is retained.
const observedPaymentPrefix = "AQAAAJEBAACQtbKgoQEAAAMAAABJAE4AUgAAAAIAAAAEAAAAMQAuADAAMAAAAAAA"

func TestDecodeObservedPaymentPrefix(t *testing.T) {
	s, err := decodePaymentPayload(observedPaymentPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if s.Currency != "INR" || s.Amount != "1.00" || s.Status != "unknown" || s.BankVerified || s.HeaderCodes != [2]uint32{1, 401} {
		t.Fatalf("unexpected summary: %+v", s)
	}
	for _, input := range []string{"", "!!!", base64.StdEncoding.EncodeToString(make([]byte, 48)), observedPaymentPrefix[:40]} {
		if _, err := decodePaymentPayload(input); err == nil {
			t.Fatal("accepted invalid prefix")
		}
	}
	b, _ := base64.StdEncoding.DecodeString(observedPaymentPrefix)
	b[36] = 'X'
	if _, err := decodePaymentPayload(base64.StdEncoding.EncodeToString(b)); err == nil {
		t.Fatal("accepted nonnumeric amount")
	}
}

func TestPaymentInteropNote(t *testing.T) {
	// Wrapper from the observed layout, with a synthetic note and no account data.
	metadata, err := hex.DecodeString("0b00000069006e007400650072006f0070004e006f007400650000000100000015000000500000000f0000006a006100760061002e006c0061006e0067002e0043006c00610073007300000025000000aced0005767200106a6176612e6c616e672e537472696e67a0f0a4387a3bb342020000787000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	chars := utf16.Encode([]rune("Lunch \U0001f35b"))
	encoded := make([]byte, (4+2*(len(chars)+1)+3)&^3)
	binary.LittleEndian.PutUint32(encoded, uint32(len(chars)))
	for i, ch := range chars {
		binary.LittleEndian.PutUint16(encoded[4+2*i:], ch)
	}
	b := append(metadata, encoded...)
	note := paymentInteropNote(b)
	if note == nil || *note != "Lunch \U0001f35b" {
		t.Fatalf("unexpected note: %v", note)
	}
	for i := range b {
		if got := paymentInteropNote(b[:i]); got != nil {
			t.Fatalf("accepted truncated note at %d", i)
		}
	}
	duplicate := append(append([]byte{}, b...), b...)
	if paymentInteropNote(duplicate) != nil {
		t.Fatal("accepted duplicate note keys")
	}
	b[32] = 99
	if paymentInteropNote(b) != nil {
		t.Fatal("accepted unknown wrapper")
	}
}

func FuzzDecodePaymentPayload(f *testing.F) {
	f.Add(observedPaymentPrefix)
	f.Add("")
	f.Fuzz(func(t *testing.T, s string) { decodePaymentPayload(s) })
}
