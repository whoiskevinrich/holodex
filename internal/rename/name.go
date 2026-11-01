// Package rename turns a video's resolved metadata into a file name and renames the file in its
// own folder (F86, docs/specs/file-rename.md). It is the one place a metadata value becomes a path
// component, so every name passes through this file's builder
// (docs/architecture/security-perimeter.md, "Rename names").
package rename

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// MaxNameBytes is the longest whole file name, extension and disambiguator included (P0-4).
const MaxNameBytes = 255

// Bad-name reasons. Each is a plan chip (handoff "Bad name") and never carries the name itself.
var (
	ErrInvalidText = errors.New("not valid text")
	ErrEmptyName   = errors.New("empty name")
	ErrReserved    = errors.New("a name Windows reserves")
	ErrTrailing    = errors.New("ends in a dot or a space")
	ErrTooLong     = errors.New("longer than 255 bytes")
	ErrBadChar     = errors.New("contains a character no file name may hold")
	ErrNotLocal    = errors.New("does not stay in its folder")
)

// forbidden are the characters P0-4 removes from every value. Braces are removed from values but
// allowed in a whole name, where only the builder's own markers put them.
const forbidden = `<>:"/\|?*`

// removedRune reports a rune P0-4 removes from a value: the forbidden set, braces, control
// characters (C0, DEL, C1) and invisible formatting characters (zero-width, text direction).
func removedRune(r rune) bool {
	return strings.ContainsRune(forbidden, r) || r == '{' || r == '}' ||
		unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}

// SanitizeValue applies P0-4 to one token value: removed characters go, each whitespace run left
// behind becomes one space, the ends are trimmed, and the result is NFC. A value that isn't valid
// UTF-8 is refused. An empty result means the token has no value.
func SanitizeValue(s string) (string, error) {
	if !utf8.ValidString(s) {
		return "", ErrInvalidText
	}
	var b strings.Builder
	space := false
	for _, r := range s {
		// ASCII tabs and newlines separate words; the C1 control NEL is removed like any other.
		if unicode.IsSpace(r) && (r < 0x80 || !unicode.IsControl(r)) {
			space = true
			continue
		}
		if removedRune(r) {
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
	}
	return norm.NFC.String(b.String()), nil
}

// reservedStems are the names Windows reserves, upper-cased. COM and LPT also take a superscript
// one, two or three.
var reservedStems = func() map[string]bool {
	m := map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true, "CONIN$": true, "CONOUT$": true}
	for _, p := range []string{"COM", "LPT"} {
		for _, d := range []string{"1", "2", "3", "4", "5", "6", "7", "8", "9", "¹", "²", "³"} {
			m[p+d] = true
		}
	}
	return m
}()

// IsReserved reports a name Windows reserves. The part before the first dot decides, in any case
// and ignoring trailing spaces, so `con.mkv`, `NUL.tar.mkv` and `CON .mkv` are all reserved.
func IsReserved(name string) bool {
	stem, _, _ := strings.Cut(name, ".")
	return reservedStems[strings.ToUpper(strings.TrimRight(stem, " "))]
}

// CheckName runs every P0-4 and P0-5 rule on a whole built file name, template text and markers
// included. It never repairs a name: a name that breaks a rule is refused.
func CheckName(name string) error {
	if !utf8.ValidString(name) || !norm.NFC.IsNormalString(name) {
		return ErrInvalidText
	}
	if name == "" || name == "." || name == ".." {
		return ErrEmptyName
	}
	for _, r := range name {
		if strings.ContainsRune(forbidden, r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return ErrBadChar
		}
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return ErrTrailing
	}
	if IsReserved(name) {
		return ErrReserved
	}
	if len(name) > MaxNameBytes {
		return ErrTooLong
	}
	if !filepath.IsLocal(name) || filepath.Base(name) != name {
		return ErrNotLocal
	}
	return nil
}

// FoldKey is the form every collision check compares: NFC, then case-folded.
func FoldKey(name string) string {
	return norm.NFC.String(cases.Fold().String(norm.NFC.String(name)))
}
