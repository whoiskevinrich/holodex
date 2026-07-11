package resolver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fixedNow is the deterministic clock the derive tests inject so Age never depends on
// the wall clock (AC-8: purity — a fixed now in yields a deterministic value out).
var fixedNow = time.Date(2026, time.July, 8, 12, 0, 0, 0, time.UTC)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestDeriveAge(t *testing.T) {
	tests := []struct {
		name      string
		birthdate string
		deathdate string
		want      string
		ok        bool
	}{
		{"before birthday this year", "1990-12-25", "", "35", true},
		{"after birthday this year", "1990-03-14", "", "36", true},
		{"on birthday", "1990-07-08", "", "36", true},
		{"absent birthdate", "", "", "", false},
		{"unparseable birthdate", "unknown", "", "", false},
		{"future birthdate", "2030-01-01", "", "", false},
		{"deceased → no running age", "1950-01-01", "1999-01-01", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := deriveAge(map[string]string{"birthdate": tc.birthdate, "deathdate": tc.deathdate}, fixedNow)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("deriveAge = (%q,%v), want (%q,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestDeriveAgeAtDeath(t *testing.T) {
	tests := []struct {
		name      string
		birthdate string
		deathdate string
		want      string
		ok        bool
	}{
		{"whole years floor before deathday", "1950-06-01", "1999-01-15", "48", true},
		{"whole years floor after deathday", "1950-01-15", "1999-06-01", "49", true},
		{"missing deathdate", "1950-01-01", "", "", false},
		{"missing birthdate", "", "1999-01-01", "", false},
		{"unparseable deathdate", "1950-01-01", "unknown", "", false},
		{"death before birth", "1999-01-01", "1950-01-01", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := deriveAgeAtDeath(map[string]string{"birthdate": tc.birthdate, "deathdate": tc.deathdate}, fixedNow)
			if ok != tc.ok || got != tc.want {
				t.Fatalf("deriveAgeAtDeath = (%q,%v), want (%q,%v)", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// TestDeriveLeapDay pins the ADR-063 §D4 convention: a Feb-29 birthdate crosses its
// birthday exactly once, between Feb-28 and Mar-01.
func TestDeriveLeapDay(t *testing.T) {
	in := map[string]string{"birthdate": "2000-02-29"}
	feb28, _ := deriveAge(in, date("2026-02-28"))
	mar01, _ := deriveAge(in, date("2026-03-01"))
	if feb28 != "25" {
		t.Fatalf("Feb-28: want age 25, got %q", feb28)
	}
	if mar01 != "26" {
		t.Fatalf("Mar-01: want age 26, got %q", mar01)
	}
}

// TestDerive_AppendsAgeUnderBirthdate covers the pass end-to-end: a living person's
// Age row is appended directly after birthdate, stamped Computed with a computed:
// winning source, nil Decision/Candidates/InSync (non-adoptable), and DerivedFrom
// carrying the birthdate registry label.
func TestDerive_AppendsAgeUnderBirthdate(t *testing.T) {
	resolved := []ResolvedField{
		{Canonical: "name", Values: []string{"Maya"}},
		{Canonical: "birthdate", Values: []string{"1990-03-14"}},
		{Canonical: "deathdate", Values: nil},
		{Canonical: "nationality", Values: []string{"American"}},
	}
	out := Derive(resolved, fixedNow)

	age, ok := fieldByKey(out, "age")
	if !ok {
		t.Fatal("want an age row")
	}
	if age.Values[0] != "36" {
		t.Fatalf("age value = %q, want 36", age.Values[0])
	}
	if !age.Computed {
		t.Fatal("age must be marked Computed")
	}
	if age.WinningSource != "computed:age" {
		t.Fatalf("winning source = %q, want computed:age", age.WinningSource)
	}
	if age.Decision != nil || age.Candidates != nil || age.InSync != nil {
		t.Fatal("a computed row must carry no decision/candidate/in-sync state")
	}
	if age.Label != "Age" {
		t.Fatalf("label = %q, want Age", age.Label)
	}
	if len(age.DerivedFrom) != 1 || age.DerivedFrom[0] != "Born" {
		t.Fatalf("derived_from = %v, want [Born]", age.DerivedFrom)
	}
	// Positioned immediately after birthdate.
	var bdIdx, ageIdx = -1, -1
	for i, f := range out {
		switch f.Canonical {
		case "birthdate":
			bdIdx = i
		case "age":
			ageIdx = i
		}
	}
	if ageIdx != bdIdx+1 {
		t.Fatalf("age at index %d, birthdate at %d — want age directly after birthdate", ageIdx, bdIdx)
	}
	// A living person shows no age_at_death.
	if _, ok := fieldByKey(out, "age_at_death"); ok {
		t.Fatal("a living person must not have an age_at_death row")
	}
}

// TestDerive_DeceasedShowsAgeAtDeathOnly asserts the mutual exclusion: a person with a
// deathdate shows age_at_death and no running age.
func TestDerive_DeceasedShowsAgeAtDeathOnly(t *testing.T) {
	resolved := []ResolvedField{
		{Canonical: "birthdate", Values: []string{"1950-01-01"}},
		{Canonical: "deathdate", Values: []string{"1999-06-15"}},
	}
	out := Derive(resolved, fixedNow)

	if _, ok := fieldByKey(out, "age"); ok {
		t.Fatal("a deceased person must not have a running age row")
	}
	aad, ok := fieldByKey(out, "age_at_death")
	if !ok {
		t.Fatal("want an age_at_death row")
	}
	if aad.Values[0] != "49" {
		t.Fatalf("age_at_death = %q, want 49", aad.Values[0])
	}
	if len(aad.DerivedFrom) != 2 || aad.DerivedFrom[0] != "Born" || aad.DerivedFrom[1] != "Died" {
		t.Fatalf("derived_from = %v, want [Born Died]", aad.DerivedFrom)
	}
}

// TestDerive_MissingBirthdateNoRow covers AC-4/AC-5: absent or unparseable birthdate
// yields neither row and never a placeholder.
func TestDerive_MissingBirthdateNoRow(t *testing.T) {
	for _, bd := range []string{"", "unknown"} {
		out := Derive([]ResolvedField{{Canonical: "birthdate", Values: valsOrNil(bd)}}, fixedNow)
		if _, ok := fieldByKey(out, "age"); ok {
			t.Fatalf("birthdate=%q: want no age row", bd)
		}
		if _, ok := fieldByKey(out, "age_at_death"); ok {
			t.Fatalf("birthdate=%q: want no age_at_death row", bd)
		}
	}
}

func valsOrNil(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

// TestDerive_PureAndDeterministic asserts the pass mutates only by appending and is a
// pure function of (resolved, now): same inputs → identical output, no I/O.
func TestDerive_PureAndDeterministic(t *testing.T) {
	in := []ResolvedField{{Canonical: "birthdate", Values: []string{"1990-03-14"}}}
	a := Derive(in, fixedNow)
	b := Derive(in, fixedNow)
	if len(a) != len(b) {
		t.Fatalf("nondeterministic length: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i].Canonical != b[i].Canonical || (len(a[i].Values) > 0 && a[i].Values[0] != b[i].Values[0]) {
			t.Fatalf("nondeterministic row %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// TestResolverPackageIsClockFree enforces ADR-051/ADR-063 AC-8: nothing in the
// resolver package reads the wall clock — the derive pass takes now as a parameter.
func TestResolverPackageIsClockFree(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(filepath.Clean(name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(src), "time.Now(") {
			t.Fatalf("%s reads the wall clock (time.Now) — the resolver must stay clock-free (ADR-051); inject now instead", name)
		}
	}
}
