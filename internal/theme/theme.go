// Package theme is the server side of the custom palette (F66, ADR-102 D4/D5): it
// parses `theme.custom` from holodex.yaml, derives the pair-partner tokens by the
// same rule app.css uses (so the boot-time contrast check judges what the browser
// will actually paint), and computes WCAG contrast for the four load-bearing pairs.
//
// The derivation is deliberately tiny — every derived token is an oklab
// color-mix() of the five primaries — and it is gated by TestDeriveMatchesCinematheque:
// Cinémathèque re-expressed as its primaries must land within ΔE*ab <= 2 of the
// hand-tuned block for every derived token (spec R11). A failing gate means the rule
// gets tuned, never the tolerance. The [data-palette='custom'] block in app.css and
// the constants in Derive must be edited together.
package theme

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
)

// allowedBases are the skins a custom palette may ride. v1 is Cinémathèque only
// (spec RD9): the other two carry hand-tuned tokens and literal-colour flourishes
// that have not passed the derivation gate.
var allowedBases = map[string]bool{"cinematheque": true}

// Primaries are the five owner-set colours, in the order app.css and the SPA use.
var Primaries = []string{"bg", "ink", "accent", "muted", "warn"}

// Input is the raw config block (mirrors config.ThemeCustom; kept here so the
// package has no config dependency).
type Input struct {
	Name, Base, BG, Ink, Accent, Muted, Warn string
}

// Custom is the parsed, validated palette. Tokens are normalised to "#rrggbb".
type Custom struct {
	Name   string
	Base   string
	Tokens map[string]string
}

var hexRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// Parse validates the block (spec R9). Every field is required except base, which
// defaults to cinematheque. Colours must be #rgb or #rrggbb and nothing else, so
// no owner-supplied string can ever reach CSS text.
func Parse(in Input) (*Custom, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len([]rune(name)) > 40 {
		return nil, errors.New("theme.custom.name is required (1–40 characters)")
	}
	base := strings.ToLower(strings.TrimSpace(in.Base))
	if base == "" {
		base = "cinematheque"
	}
	if !allowedBases[base] {
		return nil, fmt.Errorf("theme.custom.base %q is not supported (v1: cinematheque)", in.Base)
	}
	raw := map[string]string{"bg": in.BG, "ink": in.Ink, "accent": in.Accent, "muted": in.Muted, "warn": in.Warn}
	tokens := make(map[string]string, len(raw))
	for _, k := range Primaries {
		v := strings.TrimSpace(raw[k])
		if !hexRe.MatchString(v) {
			return nil, fmt.Errorf("theme.custom.%s %q is not a #rgb/#rrggbb colour", k, raw[k])
		}
		c, _ := parseHex(v)
		tokens[k] = c.Hex()
	}
	return &Custom{Name: name, Base: base, Tokens: tokens}, nil
}

// Derived are the pair-partner tokens app.css computes from the primaries in its
// [data-palette='custom'] block, in the same order.
type Derived struct {
	Surface, Surface2, Rule, AccentInk, WarnInk, LogoPlate, LogoPlateInk RGB
}

// Derive replicates app.css's [data-palette='custom'] block exactly. A percentage
// is the second colour's share, as in color-mix(in oklab, A, B p%).
func Derive(c *Custom) Derived {
	bg, _ := parseHex(c.Tokens["bg"])
	ink, _ := parseHex(c.Tokens["ink"])
	accent, _ := parseHex(c.Tokens["accent"])
	warn, _ := parseHex(c.Tokens["warn"])
	black := RGB{}
	surface2 := Mix(bg, ink, 6)
	return Derived{
		Surface:  Mix(bg, ink, 4.5),
		Surface2: surface2,
		Rule:     Mix(bg, ink, 15.5),
		// Ink on a fill: the fill tinted toward the background, then deepened toward
		// black — a near-black that keeps the fill's hue (HOLODEX-324's lesson, derived).
		AccentInk:    Mix(Mix(accent, bg, 84), black, 23),
		WarnInk:      Mix(Mix(warn, bg, 84), black, 23),
		LogoPlate:    Mix(Mix(ink, bg, 3), accent, 7),
		LogoPlateInk: Mix(surface2, accent, 11),
	}
}

// Pair is one contrast check: Fore on Back against the WCAG AA text floor.
type Pair struct {
	Name  string  `json:"pair"`
	Ratio float64 `json:"ratio"`
	Pass  bool    `json:"pass"`
}

// AAText is the WCAG 2 contrast floor for normal text.
const AAText = 4.5

// Contrast evaluates the four load-bearing pairs (spec R12). The ink partners are
// the derived ones, so the check judges what the browser paints.
func Contrast(c *Custom) []Pair {
	bg, _ := parseHex(c.Tokens["bg"])
	ink, _ := parseHex(c.Tokens["ink"])
	accent, _ := parseHex(c.Tokens["accent"])
	muted, _ := parseHex(c.Tokens["muted"])
	warn, _ := parseHex(c.Tokens["warn"])
	d := Derive(c)
	mk := func(name string, fore, back RGB) Pair {
		r := ContrastRatio(fore, back)
		return Pair{Name: name, Ratio: math.Round(r*100) / 100, Pass: r >= AAText}
	}
	return []Pair{
		mk("ink on bg", ink, bg),
		mk("muted on bg", muted, bg),
		mk("accent-ink on accent", d.AccentInk, accent),
		mk("warn-ink on warn", d.WarnInk, warn),
	}
}
