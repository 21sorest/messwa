// Package classify pulls structure out of free-text mess posts.
package classify

import (
	"strings"
	"unicode"
)

// Normalize lowercases, turns everything that isn't a letter or digit into a
// space, and collapses runs of spaces. Emojis and punctuation disappear.
func Normalize(s string) string {
	var b strings.Builder
	space := true
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			space = false
		} else if !space {
			b.WriteByte(' ')
			space = true
		}
	}
	return strings.TrimSpace(b.String())
}

// Text is a post prepared for phrase matching.
type Text struct{ padded string }

func Prepare(s string) Text { return Text{" " + Normalize(s) + " "} }

// Has reports whether any phrase appears as whole words.
func (t Text) Has(phrases ...string) bool {
	return t.First(phrases...) >= 0
}

// First returns the earliest position any phrase appears at, or -1.
func (t Text) First(phrases ...string) int {
	best := -1
	for _, p := range phrases {
		n := Normalize(p)
		if n == "" {
			continue
		}
		if i := strings.Index(t.padded, " "+n+" "); i >= 0 && (best < 0 || i < best) {
			best = i
		}
	}
	return best
}

var (
	strongBuy = []string{"anyone selling", "anybody selling", "any one selling", "someone selling", "want to buy", "wanna buy", "wtb", "looking for", "lf"}
	sell      = []string{"sell", "selling", "sale", "for sale", "available", "avail", "anyone want", "anybody want", "anyone wants", "wts", "giving"}
	weakBuy   = []string{"buy", "buying", "need", "needed", "want", "wanted", "require", "required"}
)

// Kind returns "sell", "buy", or "" when the post says neither.
func Kind(t Text) string {
	switch {
	case t.Has(strongBuy...):
		return "buy"
	case t.Has(sell...):
		return "sell"
	case t.Has(weakBuy...):
		return "buy"
	}
	return ""
}

// Diet returns "nonveg", "veg", or "".
func Diet(t Text) string {
	switch {
	case t.Has("nonveg", "non veg", "nv", "chicken", "egg", "eggs", "mutton", "fish"):
		return "nonveg"
	case t.Has("veg"):
		return "veg"
	}
	return ""
}

// Named is anything that can be spotted in text by a list of phrases.
type Named struct {
	Name    string
	Phrases []string
}

// Pick returns the names that appear in t, earliest mention first.
func Pick(t Text, options []Named) []string {
	type hit struct {
		name string
		pos  int
	}
	var hits []hit
	for _, o := range options {
		if p := t.First(append([]string{o.Name}, o.Phrases...)...); p >= 0 {
			hits = append(hits, hit{o.Name, p})
		}
	}
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && hits[j].pos < hits[j-1].pos; j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	out := make([]string, len(hits))
	for i, h := range hits {
		out[i] = h.name
	}
	return out
}
