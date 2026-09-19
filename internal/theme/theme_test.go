package theme

import (
	"strings"
	"testing"
)

// The five primaries of the shipped Cinémathèque block (web/src/app.css) and its
// hand-tuned derived tokens. Both copies are load-bearing: change app.css, change
// these, and the gate below says whether the derivation still holds.
var cinematheque = Input{Name: "Cinémathèque", Base: "cinematheque",
	BG: "#0c0a09", Ink: "#f3ece1", Accent: "#e8a33d", Muted: "#9b9082", Warn: "#e2603f"}

var cinemathequeDerived = map[string]string{
	"surface":        "#15110e",
	"surface-2":      "#181310",
	"rule":           "#2a2622",
	"accent-ink":     "#1a1206",
	"warn-ink":       "#170e08",
	"logo-plate":     "#e9e0d0",
	"logo-plate-ink": "#2a2018",
}

// Spec R11 / ADR-102 D4: Cinémathèque re-expressed as primaries + derivation must
// match its hand-tuned block within ΔE*ab <= 2 for every derived token. Failing
// means the rule (and app.css with it) gets tuned — never this tolerance.
func TestDeriveMatchesCinematheque(t *testing.T) {
	c, err := Parse(cinematheque)
	if err != nil {
		t.Fatal(err)
	}
	d := Derive(c)
	got := map[string]RGB{
		"surface": d.Surface, "surface-2": d.Surface2, "rule": d.Rule,
		"accent-ink": d.AccentInk, "warn-ink": d.WarnInk,
		"logo-plate": d.LogoPlate, "logo-plate-ink": d.LogoPlateInk,
	}
	for k, want := range cinemathequeDerived {
		w, _ := parseHex(want)
		painted, _ := parseHex(got[k].Hex()) // compare what the browser paints: 8-bit sRGB
		if de := DeltaE(painted, w); de > 2 {
			t.Errorf("%s: derived %s vs hand-tuned %s, ΔE*ab = %.2f > 2", k, got[k].Hex(), want, de)
		}
	}
}

func TestContrastCinemathequePasses(t *testing.T) {
	c, _ := Parse(cinematheque)
	for _, p := range Contrast(c) {
		if !p.Pass {
			t.Errorf("%s = %.2f:1, want >= %.1f", p.Name, p.Ratio, AAText)
		}
	}
}

func TestContrastRatioKnownValues(t *testing.T) {
	w, _ := parseHex("#ffffff")
	b, _ := parseHex("#000000")
	if r := ContrastRatio(w, b); r < 20.99 || r > 21.01 {
		t.Fatalf("white/black = %.3f, want 21", r)
	}
	if r := ContrastRatio(b, w); r < 20.99 || r > 21.01 {
		t.Fatalf("order must not matter: %.3f", r)
	}
	// HOLODEX-324's documented pair: Cinémathèque warn-ink on warn is 5.42:1.
	wi, _ := parseHex("#170e08")
	wn, _ := parseHex("#e2603f")
	if r := ContrastRatio(wi, wn); r < 5.3 || r > 5.5 {
		t.Fatalf("warn-ink on warn = %.2f, want ~5.42", r)
	}
}

func TestContrastFlagsALowPair(t *testing.T) {
	in := cinematheque
	in.Muted = "#3a3530" // barely lighter than bg
	c, _ := Parse(in)
	var flagged bool
	for _, p := range Contrast(c) {
		if p.Name == "muted on bg" && !p.Pass {
			flagged = true
		}
	}
	if !flagged {
		t.Fatal("expected muted on bg to fail AA")
	}
}

func TestParseValidation(t *testing.T) {
	ok, err := Parse(Input{Name: "Rich", BG: "#0b0a0c", Ink: "#EFE9E0", Accent: "#c04", Muted: "#9a9188", Warn: "#e2603f"})
	if err != nil {
		t.Fatal(err)
	}
	if ok.Base != "cinematheque" || ok.Tokens["ink"] != "#efe9e0" || ok.Tokens["accent"] != "#cc0044" {
		t.Fatalf("normalisation: %+v", ok)
	}
	full := Input{Name: "x", BG: "#000", Ink: "#fff", Accent: "#f00", Muted: "#888", Warn: "#f80"}
	bad := []struct {
		name string
		mut  func(*Input)
		want string
	}{
		{"no name", func(i *Input) { i.Name = "" }, "name"},
		{"long name", func(i *Input) { i.Name = strings.Repeat("x", 41) }, "name"},
		{"bad base", func(i *Input) { i.Base = "broadcast" }, "base"},
		{"missing colour", func(i *Input) { i.Warn = "" }, "warn"},
		{"css text", func(i *Input) { i.BG = "red; background:url(x)" }, "bg"},
		{"rgb()", func(i *Input) { i.BG = "rgb(0,0,0)" }, "bg"},
	}
	for _, tc := range bad {
		in := full
		tc.mut(&in)
		if _, err := Parse(in); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want mention of %q", tc.name, err, tc.want)
		}
	}
}
