package wording

import (
	"strings"
	"testing"
)

// The built-in lines must pass the checks an account's own lines do.
func TestDefaultsCheck(t *testing.T) {
	b := Book{}
	ids := map[string]bool{}
	for _, s := range Slots {
		if ids[s.ID] {
			t.Errorf("slot %s listed twice", s.ID)
		}
		ids[s.ID] = true
		b[s.ID] = s.Default
		for _, v := range s.Allowed() {
			if !known(v) {
				t.Errorf("%s uses {%s}, which isn't in Vars", s.ID, v)
			}
		}
	}
	if err := b.Check(); err != nil {
		t.Fatal(err)
	}
	if got := b.Clean(); len(got) != 0 {
		t.Errorf("Clean should drop slots set to their defaults, kept %v", got)
	}
}

func known(name string) bool {
	for _, v := range Vars {
		if v.Name == name {
			return true
		}
	}
	return false
}

func TestFill(t *testing.T) {
	vals := Values{"price": "35", "day": "tmrw", "buyer": ""}
	for tmpl, want := range map[string]string{
		"its {price}[, for {day}]":   "its 35, for tmrw",
		"ok [{buyer} ]{price}":       "ok 35",
		"[{buyer}, ]{price}[ {was}]": "35",
		"{price}  bro ":              "35 bro",
		"have it [for real]":         "have it for real",
		"{nope} {price}":             "35",
	} {
		if got := Fill(tmpl, vals); got != want {
			t.Errorf("Fill(%q) = %q, want %q", tmpl, got, want)
		}
	}
}

func TestMatches(t *testing.T) {
	for _, c := range []struct {
		tmpl, text string
		want       bool
	}{
		{"sold sorry", "sold sorry", true},
		{"sold sorry", "sold sorry bro", false},
		{"gone[ {buyer}], sorry", "gone, sorry", true},
		{"gone[ {buyer}], sorry", "gone Asha K, sorry", true},
		{"gone [{buyer}] sorry", "gone sorry", true},
		{"its sold (really)", "its sold (really)", true},
		{"its sold (really)", "its sold really", false},
	} {
		if got := Matches(c.tmpl, c.text); got != c.want {
			t.Errorf("Matches(%q, %q) = %v", c.tmpl, c.text, got)
		}
	}
}

func TestCheck(t *testing.T) {
	for _, c := range []struct {
		book Book
		err  string
	}{
		{Book{"counter": {{"take it for cheap"}}}, "Counter offer, variant 1: needs {price}"},
		{Book{"counter": {{"{price}"}, {"{price} {wait}"}}}, "variant 2: {wait} isn't available here"},
		{Book{"sold": {{"sold", "sorry"}}}, "must be a single message"},
		{Book{"nudge": {}}, "needs at least one variant"},
		{Book{"nudge": {{"u there?", " "}}}, "has an empty message"},
		{Book{"nudge": {{"[{buyer}]"}}}, "could come out empty"},
		{Book{"nudge": {{"u there [[{buyer}]]"}}}, "[ ] inside [ ]"},
		{Book{"nudge": {{"u there [{buyer}"}}}, "without its ]"},
		{Book{"nudge": {{"u there {buyer"}}}, "{ without its }"},
		{Book{"sold": {{"{buyer}"}}}, "words of its own"},
		{Book{"sold": {{"[{buyer} ]gone"}}}, ""},
		{Book{"thanks": {{"ty"}}}, `no message called "thanks"`},
		{Book{"nudge": {{strings.Repeat("a", MaxLen+1)}}}, "over 300 characters"},
		{Book{"nudge": {{"[{buyer} ]u there?"}, {"{buyer} hello"}}}, ""},
	} {
		err := c.book.Check()
		switch {
		case c.err == "" && err != nil:
			t.Errorf("%v: %v", c.book, err)
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%v: got %v, want %q", c.book, err, c.err)
		}
	}
}

func TestVariants(t *testing.T) {
	b := Book{"nudge": {{"u there?"}}}
	if got := b.Variants("nudge"); len(got) != 1 || got[0][0] != "u there?" {
		t.Errorf("own nudge = %v", got)
	}
	if got := b.Variants("sold"); len(got) != 4 {
		t.Errorf("a slot left out should keep its defaults, got %v", got)
	}
	if got := Book(nil).Variants("opening"); len(got) != 3 {
		t.Errorf("a nil book should give the defaults, got %v", got)
	}
}
