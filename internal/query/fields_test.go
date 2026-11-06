package query

import (
	"errors"
	"testing"
)

var testFields = Fields{
	"director":     {Kind: KindText, Label: "Director"},
	"actors":       {Kind: KindList, Label: "Actors"},
	"release_date": {Kind: KindDate, Label: "Release date"},
	"runtime":      {Kind: KindNumber, Label: "Runtime"},
	"container":    {Kind: KindText, Label: "Container", OwnerOnly: true},
}

// Q4: a mapped name parses by its kind and reprints to the same bytes; unmapped, it's an
// unknown field; a built-in keeps its meaning.
func TestMappedFieldsParse(t *testing.T) {
	for _, src := range []string{
		`director:"Ridley Scott"`,
		`actors:"Sigourney Weaver" -director:alien`,
		`release_date:<30d`,
		`release_date:2026-01..2026-06`,
		`runtime:>90`,
		`runtime:60..120`,
		`released:2026`,
		`updated:<30d`,
	} {
		q, err := ParseWith(src, testFields)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := q.String(); got != src {
			t.Errorf("print %s = %s", src, got)
		}
	}

	if _, err := Parse(`director:x`); !isCode(err, CodeUnknownField) {
		t.Errorf("no catalogue: %v, want unknown field", err)
	}
	if _, err := ParseWith(`nope:x`, testFields); !isCode(err, CodeUnknownField) {
		t.Errorf("unmapped: %v, want unknown field", err)
	}
	q, err := ParseWith(`studio:acme`, Fields{"studio": {Kind: KindText}})
	if err != nil {
		t.Fatal(err)
	}
	if c := Conds(q.Root)[0]; c.Kind != "" || !IsEntityField(c.Field) {
		t.Errorf("built-in studio parsed as %+v", c)
	}
	for _, name := range []string{"tag", "added", "released", "watched", "plays", "played"} {
		if !IsBuiltin(name) {
			t.Errorf("%s isn't reserved", name)
		}
	}

	for src, code := range map[string]string{
		`runtime:long`:           CodeBadValue,
		`runtime:"90"`:           CodeBadValue,
		`release_date:someday`:   CodeBadValue,
		`updated:<30m`:           CodeBadValue,
		`released:2026..2025`:    CodeBadRange,
		`release_date:"2026-01"`: CodeBadValue,
	} {
		if _, err := ParseWith(src, testFields); !isCode(err, code) {
			t.Errorf("%s: %v, want %s", src, err, code)
		}
	}
}

// Q5: a hidden field is owner-only alone, under NOT and inside OR; `released` follows the
// release-date field's visibility.
func TestHiddenFieldIsOwnerOnly(t *testing.T) {
	for _, src := range []string{`container:mkv`, `-container:mkv`, `(tag:1 OR container:mkv)`} {
		q, err := ParseWith(src, testFields)
		if err != nil {
			t.Fatal(err)
		}
		if !OwnerOnly(q.Root) {
			t.Errorf("%s isn't owner-only", src)
		}
		if err := q.CheckOwner(false); !isCode(err, CodeOwnerOnly) {
			t.Errorf("%s as a visitor: %v", src, err)
		}
		if err := q.CheckOwner(true); err != nil {
			t.Errorf("%s as the owner: %v", src, err)
		}
	}
	for _, tc := range []struct {
		hidden bool
	}{{false}, {true}} {
		q, err := ParseWith(`released:2026`, Fields{FieldReleased: {Kind: KindDate, OwnerOnly: tc.hidden}})
		if err != nil {
			t.Fatal(err)
		}
		if OwnerOnly(q.Root) != tc.hidden {
			t.Errorf("released with hidden=%v: OwnerOnly = %v", tc.hidden, OwnerOnly(q.Root))
		}
	}
}

// P0-6: a stored condition on a field gone from the mapping is named, not refused, and
// reprints as written, so a merge can rewrite the definition around it.
func TestStoredGoneField(t *testing.T) {
	const src = `director:"Ridley Scott" tag:12 runtime:>90 release_date:<30d`
	q, err := ParseStored(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := GoneFields(q.Root); len(got) != 3 || got[0] != "director" {
		t.Errorf("gone = %v", got)
	}
	if got := q.String(); got != src {
		t.Errorf("reprint = %s", got)
	}
	if !RewriteID(q.Root, FieldTag, 12, 13) {
		t.Fatal("no rewrite")
	}
	if got, want := q.String(), `director:"Ridley Scott" tag:13 runtime:>90 release_date:<30d`; got != want {
		t.Errorf("rewritten = %s", got)
	}
	q, err = ParseStored(`director:"Ridley Scott"`, testFields)
	if err != nil {
		t.Fatal(err)
	}
	if got := GoneFields(q.Root); len(got) != 0 {
		t.Errorf("still mapped, gone = %v", got)
	}
}

func TestNeedsIndex(t *testing.T) {
	for src, want := range map[string]bool{
		`tag:1`: false, `added:<30d`: false, `alien`: false,
		`director:x`: true, `-director:x`: true, `updated:<1y`: true, `released:2026`: true,
		`(tag:1 OR runtime:>9)`: true,
	} {
		q, err := ParseWith(src, testFields)
		if err != nil {
			t.Fatal(err)
		}
		if NeedsIndex(q.Root) != want {
			t.Errorf("%s: NeedsIndex = %v", src, !want)
		}
	}
}

func TestNormValue(t *testing.T) {
	for _, tc := range []struct {
		kind, raw, want string
		ok              bool
	}{
		{KindText, "  Ridley   SCOTT ", "ridley scott", true},
		{KindList, "", "", false},
		{KindDate, "2026", "2026-01-01", true},
		{KindDate, "2026-03", "2026-03-01", true},
		{KindDate, "2026-03-09", "2026-03-09", true},
		{KindDate, "2026-03-09T10:00:00Z", "2026-03-09", true},
		{KindDate, "2026-13", "", false},
		{KindDate, "soon", "", false},
		{KindDate, "", "", false},
	} {
		got, ok := NormValue(tc.kind, tc.raw)
		if got != tc.want || ok != tc.ok {
			t.Errorf("NormValue(%s, %q) = %q, %v", tc.kind, tc.raw, got, ok)
		}
	}
}

func isCode(err error, code string) bool {
	var e *Error
	return errors.As(err, &e) && e.Code == code
}
