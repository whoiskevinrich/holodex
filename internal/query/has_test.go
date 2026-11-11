package query

import (
	"errors"
	"slices"
	"testing"
)

// testing-strategy §42.3: has: (F90 P0-2).

var hasFields = Fields{
	FieldReleased: {Kind: KindDate},
	"director":    {Kind: KindList, Label: "Director"},
}

// Q1: has: parses and prints canonically, and anything it can't check is F87's
// unknown-field error at the field's offset.
func TestHasParsesAndPrints(t *testing.T) {
	for src, want := range map[string]string{
		`has:director`:             "has:director",
		`-has:poster`:              "-has:poster",
		`has:released`:             "has:released",
		`HAS:Director`:             "has:director",
		`has:"director"`:           "has:director",
		`has:director -has:poster`: "has:director -has:poster",
	} {
		q, err := ParseWith(src, hasFields)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if got := Print(q.Root); got != want {
			t.Errorf("%s prints %q, want %q", src, got, want)
		}
		again, err := ParseWith(Print(q.Root), hasFields)
		if err != nil || Print(again.Root) != want {
			t.Errorf("%s does not round-trip: %v", src, err)
		}
	}
	for _, src := range []string{`has:_x`, `has:cover`, `has:poster_url`, `has:nonesuch`, `has:tag`} {
		_, err := ParseWith(src, hasFields)
		var e *Error
		if !errors.As(err, &e) || e.Code != CodeUnknownField || e.Start != len("has:") {
			t.Errorf("%s: %v, want an unknown-field error at the value", src, err)
		}
	}
	if q, err := ParseWith(`has:director`, hasFields); err == nil {
		if c := Conds(q.Root)[0]; c.FieldLabel != "Director" || !NeedsIndex(q.Root) {
			t.Errorf("has:director: label %q, needs index %v", c.FieldLabel, NeedsIndex(q.Root))
		}
	}
}

// A stored has: on a field removed from the mapping is gone, naming that field.
func TestHasStoredOnARemovedField(t *testing.T) {
	q, err := ParseStored(`has:runtime`, hasFields)
	if err != nil {
		t.Fatal(err)
	}
	if got := GoneFields(q.Root); !slices.Equal(got, []string{"runtime"}) {
		t.Errorf("gone = %v", got)
	}
	if got := Print(q.Root); got != "has:runtime" {
		t.Errorf("prints %q", got)
	}
}

// Q2: has: is owner-only alone, under NOT and inside an OR group.
func TestHasIsOwnerOnly(t *testing.T) {
	for _, src := range []string{`has:director`, `-has:poster`, `a OR has:released`, `-(a OR has:director)`} {
		q, err := ParseWith(src, hasFields)
		if err != nil {
			t.Fatal(err)
		}
		if !OwnerOnly(q.Root) {
			t.Errorf("%s is not owner-only", src)
		}
		var e *Error
		if err := q.CheckOwner(false); !errors.As(err, &e) || e.Code != CodeOwnerOnly {
			t.Errorf("%s: visitor check = %v", src, err)
		}
	}
}
