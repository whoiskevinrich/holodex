package rename

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// N8: every removed character, one subtest each.
func TestSanitizeValueRemoves(t *testing.T) {
	for _, r := range []rune{'<', '>', '?', '!', '{', '}',
		0x00, 0x1F, 0x7F, 0x85, 0x9F, 0x200B, 0x200E, 0x202E, 0x2066, 0xFEFF} {
		t.Run(string(r), func(t *testing.T) {
			got, err := SanitizeValue("Ada"+string(r)+"Love", nil)
			if err != nil || got != "AdaLove" {
				t.Fatalf("SanitizeValue(%U) = %q, %v; want %q", r, got, err, "AdaLove")
			}
		})
	}
}

// HOLODEX-632: every substituted character, one subtest each.
func TestSanitizeValueSubstitutes(t *testing.T) {
	for r, want := range map[rune]string{
		':': "Ada-Love", '|': "Ada-Love", '*': "Ada-Love",
		'"': "Ada'Love", '/': "Ada+Love", '\\': "Ada+Love", ';': "Ada,Love",
	} {
		t.Run(string(r), func(t *testing.T) {
			got, err := SanitizeValue("Ada"+string(r)+"Love", nil)
			if err != nil || got != want {
				t.Fatalf("SanitizeValue(%q) = %q, %v; want %q", r, got, err, want)
			}
		})
	}
}

// HOLODEX-633: an owner's table replaces the default. A stand-in's spaces collapse with the value's,
// substitution is one pass, and a forbidden character outside the table is still removed.
func TestSanitizeValueOwnerTable(t *testing.T) {
	chars := Characters{':': " - ", '&': " and ", '!': ";", ';': "!", '-': "-"}
	for in, want := range map[string]string{
		"Fast & Furious": "Fast and Furious",
		"Fast&Furious":   "Fast and Furious",
		"a: b":           "a - b",
		"Wow!":           "Wow;", // one pass: the ; stand-in isn't itself substituted
		"a;b":            "a!b",
		"a?b":            "ab", // not in this table, but no file name may hold it
		"a - b":          "a- b",
	} {
		if got, err := SanitizeValue(in, chars); err != nil || got != want {
			t.Errorf("SanitizeValue(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
}

func TestSanitizeValueWhitespaceAndNFC(t *testing.T) {
	cases := map[string]string{
		"a\t\n b": "a b",
		"  Ada  ": "Ada",
		"a{b}c":   "abc",
		"Café":   "Café", // NFD é composes
		"../etc":  "..+etc",
		// A substituted dash joins the word before it, so it never forms " - ".
		"Star Wars: Episode IV": "Star Wars- Episode IV",
		"a : b":                 "a- b",
		"Fire | Ice":            "Fire- Ice",
		"a - b":                 "a - b", // a dash the value already had is kept as is
		"M*A*S*H":               "M-A-S-H",
		`The "Thing"`:           "The 'Thing'",
		"AC/DC":                 "AC+DC",
		"Love; Death":           "Love, Death",
		"Airplane!":             "Airplane",
		"Who Goes There?":       "Who Goes There",
		"a:b*c?":                "a-b-c",
		" : a":                  "a",
		"*batteries":            "batteries", // no word before the dash, so a name never starts with "-"
		"{edition-x}":           "edition-x",
	}
	for in, want := range cases {
		got, err := SanitizeValue(in, nil)
		if err != nil || got != want {
			t.Errorf("SanitizeValue(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := SanitizeValue("a\xffb", nil); !errors.Is(err, ErrInvalidText) {
		t.Errorf("invalid UTF-8: err = %v, want ErrInvalidText", err)
	}
}

// N9: a whole name ending in a dot or a space is refused, never trimmed. The builder reads P0-4 as
// the whole name, so a stem ending in a dot before the extension (`Ada..mkv`) is valid.
func TestCheckNameTrailing(t *testing.T) {
	for _, n := range []string{"Ada.", "Ada ", "Ada.mkv.", "Ada.mkv "} {
		if err := CheckName(n); !errors.Is(err, ErrTrailing) {
			t.Errorf("CheckName(%q) = %v, want ErrTrailing", n, err)
		}
	}
	if err := CheckName("Ada..mkv"); err != nil {
		t.Errorf("CheckName(Ada..mkv) = %v, want nil", err)
	}
}

// N10: reserved names, with and without an extension and in any case; the part before the first
// dot decides.
func TestCheckNameReserved(t *testing.T) {
	reserved := []string{"CON", "con", "Con.mkv", "NUL", "NUL.tar.mkv", "CON .mkv", "CONIN$", "conout$.mkv",
		"AUX.mkv", "PRN", "COM¹", "LPT³.mkv"}
	for i := 1; i <= 9; i++ {
		reserved = append(reserved, "COM"+string(rune('0'+i)), "lpt"+string(rune('0'+i))+".mkv")
	}
	for _, n := range reserved {
		if err := CheckName(n); !errors.Is(err, ErrReserved) {
			t.Errorf("CheckName(%q) = %v, want ErrReserved", n, err)
		}
	}
	for _, n := range []string{"CONSOLE.mkv", "COM10.mkv", "COM0.mkv", "Ada CON.mkv"} {
		if err := CheckName(n); err != nil {
			t.Errorf("CheckName(%q) = %v, want nil", n, err)
		}
	}
}

// N11 (whole-name half): bytes, not characters.
func TestCheckNameBytes(t *testing.T) {
	ok := strings.Repeat("a", 251) + ".mkv"
	if err := CheckName(ok); err != nil {
		t.Errorf("255 bytes: %v", err)
	}
	if err := CheckName("a" + ok); !errors.Is(err, ErrTooLong) {
		t.Errorf("256 bytes: %v, want ErrTooLong", err)
	}
	multi := strings.Repeat("é", 126) + ".mkv" // 126 characters, 256 bytes
	if err := CheckName(multi); !errors.Is(err, ErrTooLong) {
		t.Errorf("multibyte 256 bytes: %v, want ErrTooLong", err)
	}
}

// N2 (whole-name half) and N12: the builder's whole-name check refuses what a template that bypassed
// saving could still produce, and every accepted name stays in its folder.
func TestCheckNameWhole(t *testing.T) {
	bad := map[string]error{
		"":           ErrEmptyName,
		".":          ErrEmptyName,
		"..":         ErrEmptyName,
		"a/b.mkv":    ErrBadChar,
		`a\b.mkv`:    ErrBadChar,
		"a:b.mkv":    ErrBadChar,
		"a\x01b.mkv": ErrBadChar,
		"a​b":        ErrBadChar,
		"Café.mkv":  ErrInvalidText,
		"a\xff.mkv":  ErrInvalidText,
	}
	for n, want := range bad {
		if err := CheckName(n); !errors.Is(err, want) {
			t.Errorf("CheckName(%q) = %v, want %v", n, err, want)
		}
	}
	for _, n := range []string{"[Acme] Ada.mkv", "Ada {provider-1-42}.mkv", "Ada {hx-7}.mkv", "..etc.mkv"} {
		if err := CheckName(n); err != nil {
			t.Errorf("CheckName(%q) = %v, want nil", n, err)
		}
		dir := filepath.Join("lib", "films")
		if filepath.Dir(filepath.Join(dir, n)) != dir {
			t.Errorf("%q leaves its folder", n)
		}
	}
}

// N13: collisions compare NFC and case-folded.
func TestFoldKey(t *testing.T) {
	if FoldKey("ada lovelace.mkv") != FoldKey("Ada Lovelace.MKV") {
		t.Error("case must fold")
	}
	if FoldKey("Café.mkv") != FoldKey("Café.mkv") {
		t.Error("NFD must equal NFC")
	}
}
