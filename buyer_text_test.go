package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"messwa/internal/config"
)

func TestMain(m *testing.M) {
	// Tests pin the first variant of each reply and send its parts at once.
	pick = func(int) int { return 0 }
	partGap = 0
	os.Exit(m.Run())
}

func TestDayLabel(t *testing.T) {
	loc := time.FixedZone("IST", 5*3600+1800)
	now := time.Date(2026, 11, 12, 23, 30, 0, 0, loc)
	for date, want := range map[string]string{
		"2026-11-12": "",
		"2026-11-13": "tmrw",
		"2026-11-15": "15 nov",
		"2026-12-01": "1 dec",
	} {
		if got := dayLabel(now, date); got != want {
			t.Errorf("dayLabel(%s) = %q, want %q", date, got, want)
		}
	}
}

// Every variant must stay in the groups' style: lowercase, no ₹, nothing
// a real seller wouldn't type.
func TestBuyerTextStyle(t *testing.T) {
	defer func(p func(int) int) { pick = p }(pick)
	for v := 0; v < 4; v++ {
		pick = func(n int) int { return min(v, n-1) }
		l, tmrw := testLines("nonveg", ""), testLines("nonveg", "tmrw")
		for _, r := range []reply{
			l.opening(60), tmrw.opening(60), l.counter(50, 60, 1), l.counter(45, 50, 2), l.lowest(40, 45),
			l.which(60), l.otherMeal(60, false), l.otherMeal(60, true), l.restate(60), l.queued(50), l.queuedHandoff(50), l.outbid(false), l.outbid(true),
			l.dropped(), l.nudge(60), l.quietDrop(50, 60), l.have(60), l.payOther(), l.noScreenshots(),
			l.sold(), l.payPrompt("30.00", false), l.payPrompt("1.00", true),
		} {
			if len(r) == 0 || len(r) > 3 {
				t.Errorf("%q: want one to three messages", r)
			}
			for _, part := range r {
				for _, bad := range []string{"₹", "negotiable", "Please", "WhatsApp", "QR", ":"} {
					if strings.Contains(part, bad) {
						t.Errorf("%q contains %q", part, bad)
					}
				}
			}
		}
	}
	if got := (lines{}).payPrompt("30.00", false); got[0] != "ok 30" {
		t.Errorf("prompt starts %q", got[0])
	}
	if got := testLines("veg", "tmrw").opening(30); len(got) != 1 || !strings.HasSuffix(got[0], ", its for tmrw") {
		t.Errorf("opening for tomorrow = %q", got)
	}
	if got := testLines("veg", "").queued(30); got[1] != "someone else is paying rn, if they dont in 2 min its yours" {
		t.Errorf("queued = %q", got)
	}
}

// testLines are the built-in lines for a kadamba lunch on day.
func testLines(diet, day string) lines {
	l := saleLines(nil, "kadamba", diet, "lunch", "", time.Time{}, nil)
	l.vals["day"] = day
	return l
}

// An account's own wording replaces the built-in lines, fills in the
// buyer and drops optional bits with nothing in them.
func TestOwnWording(t *testing.T) {
	cfg := config.Default()
	cfg.IsolateSettings()
	if _, err := cfg.Update(config.Settings{"sell.messages": json.RawMessage(`{
		"counter": [["ok [{buyer} ]{price} for you", "was {was} before"]],
		"opening": [["{price}[ for {day}], {mealtime} at {mess}"]],
		"sold": [["gone[ {buyer}], sorry"]]
	}`)}); err != nil {
		t.Fatal(err)
	}
	l := saleLines(cfg, "kadamba", "veg", "lunch", time.Now().In(cfg.Location()).Format("2006-01-02"), time.Time{}, nil)
	if got := l.forBuyer("Asha").counter(40, 45, 1); strings.Join(got, "|") != "ok Asha 40 for you|was 45 before" {
		t.Errorf("counter = %q", got)
	}
	if got := l.counter(40, 45, 1); got[0] != "ok 40 for you" {
		t.Errorf("counter with no name = %q", got)
	}
	if got := l.opening(40); got[0] != "40, lunch at kadamba" {
		t.Errorf("opening today = %q", got)
	}
	if got := l.counter(40, 45, 2); got[0] != "40?" {
		t.Errorf("a slot left out should keep the built-in line, got %q", got)
	}
	if !isSold(cfg, "gone Asha, sorry") || !isSold(cfg, "gone, sorry") || !isSold(cfg, "sold sorry") || isSold(cfg, "pay me 40") {
		t.Error("isSold should take the account's sold lines and the built-in ones, nothing else")
	}

	if _, err := cfg.Update(config.Settings{"sell.messages": json.RawMessage(`{"opening": [["{price} bro"]]}`), "sell.own_wording": json.RawMessage(`false`)}); err != nil {
		t.Fatal(err)
	}
	if got := l.opening(40); got[0] != "40" {
		t.Errorf("own_wording off should send the built-in line, got %q", got)
	}
}
