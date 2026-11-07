package query_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"holodex/internal/query"
)

// F89 testing-strategy §40 Q1: the date grammar parses and prints canonically, and a
// relative value prints as written, never as a resolved date.
func TestDateGrammarRoundTrip(t *testing.T) {
	for in, want := range map[string]string{
		"added:<30d":             "added:<30d",
		"added:<=30d":            "added:<30d",
		"added:<2w":              "added:<2w",
		"added:<6mo":             "added:<6mo",
		"added:<1y":              "added:<1y",
		"added:>30d":             "added:>30d",
		"added:>=30d":            "added:>30d",
		"added:>2026-01-01":      "added:>2026-01-01",
		"added:<=2025-12-31":     "added:<=2025-12-31",
		"added:2026-01..2026-06": "added:2026-01..2026-06",
		"added:2026":             "added:2026",
		"-added:<30d tag:3":      "-added:<30d tag:3",
	} {
		q, err := query.Parse(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := q.String(); got != want {
			t.Errorf("%q prints %q, want %q", in, got, want)
		}
		if again, err := query.Parse(q.String()); err != nil || again.String() != want {
			t.Errorf("%q does not reparse to itself: %v", want, err)
		}
	}
}

// Q2: `added:<30m` is refused at the value with both readings suggested; duration keeps
// m as minutes.
func TestDateMinutesSuggestion(t *testing.T) {
	_, err := query.Parse("added:<30m")
	var qe *query.Error
	if !errors.As(err, &qe) || qe.Code != query.CodeBadValue {
		t.Fatalf("added:<30m: %v, want a bad_value error", err)
	}
	if !strings.Contains(qe.Message, "30mo") || !strings.Contains(qe.Message, "30d") {
		t.Errorf("message %q doesn't suggest 30mo and 30d", qe.Message)
	}
	if qe.Start != len("added:") {
		t.Errorf("error starts at %d, want the value at %d", qe.Start, len("added:"))
	}
	if _, err := query.Parse("duration:<30m"); err != nil {
		t.Errorf("duration:<30m: %v", err)
	}
	for _, bad := range []string{"added:30d", "added:<0d", "added:2026-13", "added:2026-02-30", "added:7d..30d", "added:2026-06..2026-01", `added:"2026"`, "added:soon"} {
		if _, err := query.Parse(bad); err == nil {
			t.Errorf("%q parsed, want an error", bad)
		}
	}
}

func cond(t *testing.T, src string) *query.Cond {
	t.Helper()
	q, err := query.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	return query.Conds(q.Root)[0]
}

// Q3: bounds come from a fake clock in whole days of the server's zone (RD12): the
// cut moves day to day but not within a day; a year-month range covers its months
// inclusively; a month step past a month's end clamps to its last day.
func TestDateBounds(t *testing.T) {
	loc := time.FixedZone("server", -7*3600)
	at := func(s string) time.Time {
		v, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	day := func(s string) time.Time { return at(s + " 00:00") }

	c := cond(t, "added:<30d")
	lo, hi, ok := query.Bounds(c, at("2026-11-02 09:15"), loc)
	if !ok || !lo.Equal(day("2026-10-03")) || !hi.IsZero() {
		t.Errorf("<30d at 2 Nov = [%v, %v) %v, want from 3 Oct midnight", lo, hi, ok)
	}
	if lo2, _, _ := query.Bounds(c, at("2026-11-02 23:59"), loc); !lo2.Equal(lo) {
		t.Errorf("<30d moved within a day: %v vs %v", lo2, lo)
	}
	if later, _, _ := query.Bounds(c, at("2026-12-03 09:15"), loc); !later.Equal(day("2026-11-03")) {
		t.Errorf("<30d 31 days later = %v, want 3 Nov", later)
	}
	if _, hi, _ := query.Bounds(cond(t, "added:>2w"), at("2026-11-02 09:15"), loc); !hi.Equal(day("2026-10-19")) {
		t.Errorf(">2w upper bound = %v, want 19 Oct", hi)
	}
	if lo, _, _ := query.Bounds(cond(t, "added:<1mo"), at("2026-03-31 12:00"), loc); !lo.Equal(day("2026-02-28")) {
		t.Errorf("<1mo on 31 Mar = %v, want 28 Feb", lo)
	}
	if lo, _, _ := query.Bounds(cond(t, "added:<1y"), at("2028-02-29 12:00"), loc); !lo.Equal(day("2027-02-28")) {
		t.Errorf("<1y on 29 Feb = %v, want 28 Feb the year before", lo)
	}

	for src, want := range map[string][2]string{
		"added:2026-01..2026-06": {"2026-01-01", "2026-07-01"},
		"added:2026":             {"2026-01-01", "2027-01-01"},
		"added:2026-02-03":       {"2026-02-03", "2026-02-04"},
		"added:<2026":            {"", "2026-01-01"},
		"added:<=2025-12-31":     {"", "2026-01-01"},
		"added:>2026-01":         {"2026-02-01", ""},
		"added:>=2026-01":        {"2026-01-01", ""},
	} {
		lo, hi, ok := query.Bounds(cond(t, src), at("2026-11-02 09:15"), loc)
		if !ok {
			t.Errorf("%s: no bounds", src)
			continue
		}
		if (want[0] == "") != lo.IsZero() || (want[0] != "" && !lo.Equal(day(want[0]))) ||
			(want[1] == "") != hi.IsZero() || (want[1] != "" && !hi.Equal(day(want[1]))) {
			t.Errorf("%s = [%v, %v), want [%s, %s)", src, lo, hi, want[0], want[1])
		}
	}
}

// The chip reads in words (P0-1).
func TestDateLabel(t *testing.T) {
	for src, want := range map[string]string{
		"added:<30d":             "last 30 days",
		"added:<1w":              "last 1 week",
		"added:>6mo":             "over 6 months ago",
		"added:2026":             "2026",
		"added:2026-01..2026-06": "2026-01 to 2026-06",
		"added:<2026":            "before 2026",
		"added:>=2026-03":        "2026-03 or later",
	} {
		if got := query.DateLabel(cond(t, src)); got != want {
			t.Errorf("%s label %q, want %q", src, got, want)
		}
	}
}
