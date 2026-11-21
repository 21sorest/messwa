package dashboard

import (
	"bytes"
	"image/jpeg"
	"image/png"
	"testing"

	"rsc.io/qr"

	"messwa/internal/config"
)

func TestReadUPIQR(t *testing.T) {
	code, err := qr.Encode("upi://pay?pa=Seller@OkAxis&pn=Seller&cu=INR", qr.H)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := readUPIQR(code.PNG()); err != nil || got != "seller@okaxis" {
		t.Fatalf("PNG: got %q, %v", got, err)
	}
	// A JPEG, as a phone's camera or screenshot might give.
	img, err := png.Decode(bytes.NewReader(code.PNG()))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	if got, err := readUPIQR(buf.Bytes()); err != nil || got != "seller@okaxis" {
		t.Fatalf("JPEG: got %q, %v", got, err)
	}
	// The QR the bot sends a buyer reads back too.
	pay, err := qr.Encode(config.UPIPayURI("seller@okaxis", "30.00", "kadamba lunch"), qr.M)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := readUPIQR(pay.PNG()); err != nil || got != "seller@okaxis" {
		t.Fatalf("pay QR: got %q, %v", got, err)
	}

	web, _ := qr.Encode("https://example.com", qr.M)
	if _, err := readUPIQR(web.PNG()); err == nil {
		t.Error("a web link QR read as a UPI ID")
	}
	if _, err := readUPIQR([]byte("not an image")); err == nil {
		t.Error("text read as a QR")
	}
}
