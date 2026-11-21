package config

import (
	"net/url"
	"testing"
)

func TestParseUPI(t *testing.T) {
	for in, want := range map[string]string{
		"":                      "",
		"  name@okaxis ":        "name@okaxis",
		"Some.One-1@OkHDFCBank": "some.one-1@okhdfcbank",
		"upi://pay?pa=me@ybl&pn=Me%20Too&mc=0000": "me@ybl",
		"UPI://PAY?pa=me@ybl":                     "me@ybl",
	} {
		if got, err := ParseUPI(in); err != nil || got != want {
			t.Errorf("ParseUPI(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"name", "@okaxis", "name@", "name@ok axis", "upi://pay?pn=Me", "upi://mandate?pa=me@ybl", "https://pay?pa=me@ybl"} {
		if got, err := ParseUPI(in); err == nil {
			t.Errorf("ParseUPI(%q) = %q, want an error", in, got)
		}
	}
}

func TestUPIPayURI(t *testing.T) {
	u, err := url.Parse(UPIPayURI("me@ybl", "30.00", "kadamba lunch"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme != "upi" || u.Host != "pay" || q.Get("pa") != "me@ybl" || q.Get("am") != "30.00" || q.Get("cu") != "INR" || q.Get("tn") != "kadamba lunch" {
		t.Errorf("got %s", u)
	}
	if got := UPIPayURI("me@ybl", "", ""); got != "upi://pay?cu=INR&pa=me%40ybl" {
		t.Errorf("no amount: got %s", got)
	}
}
