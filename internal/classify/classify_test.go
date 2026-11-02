package classify

import "testing"

func TestKind(t *testing.T) {
	cases := map[string]string{
		"Selling kadamb lunch 🍛":          "sell",
		"anyone selling yuktahar dinner?": "buy",
		"Need palash bf":                  "buy",
		"kadamb NV dinner available":      "sell",
		"anyone want lunch? north":        "sell",
		"WTB lunch":                       "buy",
		"good morning":                    "",
	}
	for in, want := range cases {
		if got := Kind(Prepare(in)); got != want {
			t.Errorf("Kind(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPickAndDiet(t *testing.T) {
	meals := []Named{{"breakfast", []string{"bf", "b/f"}}, {"lunch", nil}, {"dinner", nil}}
	if got := Pick(Prepare("selling B/F and lunch"), meals); len(got) != 2 || got[0] != "breakfast" {
		t.Errorf("Pick = %v", got)
	}
	if got := Pick(Prepare("sell dinner, lunch"), meals); got[0] != "dinner" {
		t.Errorf("Pick order = %v", got)
	}
	if Diet(Prepare("Kadamba Non-Veg dinner")) != "nonveg" || Diet(Prepare("kadamba veg")) != "veg" {
		t.Error("diet")
	}
	if Prepare("bfast").Has("bf") {
		t.Error("bf must match whole words only")
	}
}
