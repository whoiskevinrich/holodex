package query

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Date conditions (F89 P0-1, RD1, RD12). A date value is relative (`<30d`, `>2w`,
// `<6mo`, `<1y`: within the last, or longer ago than) or absolute (`2026`, `2026-01`,
// `2026-01-05`, each standing for its whole period, behind a comparison or in an
// `a..b` range). It is kept as written in Cond.Value, so a saved relative condition
// moves with time; Bounds turns it into an instant range at read time.

// dateUnits are the relative units, longest first so `mo` wins over a bare `m`.
var dateUnits = []string{"mo", "d", "w", "y"}

// maxDateCount bounds a relative count, so the arithmetic in Bounds stays in range.
const maxDateCount = 9999

// parseDateCond fills c from a date value, returning a message on failure.
func parseDateCond(c *Cond, v string) string {
	if lo, hi, ok := strings.Cut(v, ".."); ok {
		if lo == "" || hi == "" {
			return "use a range like 2026-01..2026-06."
		}
		a, okA := parseAbsDate(lo)
		b, okB := parseAbsDate(hi)
		if !okA || !okB {
			return "a range takes two dates, like 2026-01..2026-06."
		}
		if a.start.After(b.start) {
			return "the range's start is after its end."
		}
		c.Op, c.Value = OpRange, a.text+".."+b.text
		return ""
	}
	op := OpEq
	for _, o := range []string{OpLe, OpGe, OpLt, OpGt} {
		if strings.HasPrefix(v, o) {
			op, v = o, v[len(o):]
			break
		}
	}
	if n, unit, ok := parseRelative(v); ok {
		if op == OpEq {
			return fmt.Sprintf("use <%d%s for the last %s, or >%d%s for longer ago.", n, unit, relWords(n, unit), n, unit)
		}
		// `<=` reads the same as `<`, and `>=` as `>`, on a whole-day scale.
		if op == OpLe {
			op = OpLt
		} else if op == OpGe {
			op = OpGt
		}
		c.Op, c.Value = op, op+strconv.Itoa(n)+unit
		return ""
	}
	if strings.HasSuffix(v, "m") {
		if _, err := strconv.Atoi(strings.TrimSuffix(v, "m")); err == nil {
			n := strings.TrimSuffix(v, "m")
			return "m is minutes; use " + n + "mo for months or " + n + "d for days."
		}
	}
	d, ok := parseAbsDate(v)
	if !ok {
		return "can't read " + strconv.Quote(v) + ". Use a date like 2026-01-05, or <30d for the last 30 days."
	}
	c.Op = op
	c.Value = d.text
	if op != OpEq {
		c.Value = op + d.text
	}
	return ""
}

// parseRelative reads `30d`, `2w`, `6mo` or `1y`.
func parseRelative(s string) (int, string, bool) {
	for _, u := range dateUnits {
		num, ok := strings.CutSuffix(s, u)
		if !ok || num == "" {
			continue
		}
		n, err := strconv.Atoi(num)
		if err != nil || n < 1 || n > maxDateCount {
			return 0, "", false
		}
		return n, u, true
	}
	return 0, "", false
}

// absDate is one calendar period: a year, a month or a day.
type absDate struct {
	text   string // canonical spelling: 2026, 2026-01 or 2026-01-05
	start  time.Time
	years  int // the period's length, in exactly one of these
	months int
	days   int
}

// parseAbsDate reads YYYY, YYYY-MM or YYYY-MM-DD as a period in UTC (Bounds moves it
// into the server's zone).
func parseAbsDate(s string) (absDate, bool) {
	parts := strings.Split(s, "-")
	if len(parts) > 3 || len(parts[0]) != 4 {
		return absDate{}, false
	}
	nums := make([]int, len(parts))
	for i, p := range parts {
		if i > 0 && len(p) != 2 {
			return absDate{}, false
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return absDate{}, false
		}
		nums[i] = n
	}
	y, m, d := nums[0], 1, 1
	if y < 1 {
		return absDate{}, false
	}
	out := absDate{years: 1}
	if len(nums) > 1 {
		m, out.years, out.months = nums[1], 0, 1
		if m < 1 || m > 12 {
			return absDate{}, false
		}
	}
	if len(nums) > 2 {
		d, out.months, out.days = nums[2], 0, 1
		if d < 1 || d > daysIn(y, time.Month(m)) {
			return absDate{}, false
		}
	}
	out.start = time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	out.text = s
	return out, true
}

func daysIn(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// in moves an absDate's start into loc, keeping the calendar date.
func (d absDate) in(loc *time.Location) time.Time {
	return time.Date(d.start.Year(), d.start.Month(), d.start.Day(), 0, 0, 0, 0, loc)
}

func (d absDate) end(loc *time.Location) time.Time {
	return addClamped(d.in(loc), d.years, d.months, d.days)
}

// addClamped adds years, months and days to t, clamping a day past the target month's
// end to its last day: 31 March plus -1 month is 28 (or 29) February (RD12). Go's
// AddDate would normalise to 3 March instead.
func addClamped(t time.Time, years, months, days int) time.Time {
	y, m := t.Year()+years, int(t.Month())+months
	for m < 1 {
		m, y = m+12, y-1
	}
	for m > 12 {
		m, y = m-12, y+1
	}
	d := min(t.Day(), daysIn(y, time.Month(m)))
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, t.Location()).AddDate(0, 0, days)
}

// Bounds is a date condition as the half-open instant range [Lo, Hi); a zero bound is
// open. Relative values count whole days from midnight today in loc (RD12), so a
// result doesn't change through the day. ok is false for a condition that isn't a
// parsed date.
func Bounds(c *Cond, now time.Time, loc *time.Location) (lo, hi time.Time, ok bool) {
	v := c.Value
	switch c.Op {
	case OpRange:
		a, b, _ := strings.Cut(v, "..")
		da, okA := parseAbsDate(a)
		db, okB := parseAbsDate(b)
		return da.in(loc), db.end(loc), okA && okB
	case OpEq:
	default:
		v = v[len(c.Op):]
	}
	if n, unit, isRel := parseRelative(v); isRel {
		t := now.In(loc)
		cut := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
		switch unit {
		case "d":
			cut = cut.AddDate(0, 0, -n)
		case "w":
			cut = cut.AddDate(0, 0, -7*n)
		case "mo":
			cut = addClamped(cut, 0, -n, 0)
		case "y":
			cut = addClamped(cut, -n, 0, 0)
		}
		if c.Op == OpLt {
			return cut, time.Time{}, true
		}
		return time.Time{}, cut, true
	}
	d, isAbs := parseAbsDate(v)
	if !isAbs {
		return time.Time{}, time.Time{}, false
	}
	switch c.Op {
	case OpLt:
		return time.Time{}, d.in(loc), true
	case OpLe:
		return time.Time{}, d.end(loc), true
	case OpGt:
		return d.end(loc), time.Time{}, true
	case OpGe:
		return d.in(loc), time.Time{}, true
	}
	return d.in(loc), d.end(loc), true
}

// NormValue is a resolved value as the index stores it (field-resolution.md): a date as
// YYYY-MM-DD, so a range compares lexically, with a year or year-month standing for its
// first day; anything else lower-cased with its whitespace collapsed. ok is false when
// it has nothing to index: blank, or a date that isn't one.
func NormValue(kind, raw string) (string, bool) {
	s := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	if kind != KindDate {
		return s, s != ""
	}
	switch {
	case len(s) > 10:
		s = s[:10]
	case len(s) == 7:
		s += "-01"
	case len(s) == 4:
		s += "-01-01"
	}
	if d, ok := parseAbsDate(s); ok && d.days == 1 {
		return s, true
	}
	return "", false
}

// DateLabel is a date condition in words, for its chip: `last 30 days`, `over 2
// weeks ago`, `2026`, `2026-01 to 2026-06`, `before 2026`.
func DateLabel(c *Cond) string {
	v := c.Value
	if c.Op == OpRange {
		a, b, _ := strings.Cut(v, "..")
		return a + " to " + b
	}
	if c.Op != OpEq {
		v = v[len(c.Op):]
	}
	if n, unit, ok := parseRelative(v); ok {
		if c.Op == OpLt {
			return "last " + relWords(n, unit)
		}
		return "over " + relWords(n, unit) + " ago"
	}
	switch c.Op {
	case OpLt:
		return "before " + v
	case OpLe:
		return v + " or earlier"
	case OpGt:
		return "after " + v
	case OpGe:
		return v + " or later"
	}
	return v
}

func relWords(n int, unit string) string {
	word := map[string]string{"d": "day", "w": "week", "mo": "month", "y": "year"}[unit]
	if n != 1 {
		word += "s"
	}
	return strconv.Itoa(n) + " " + word
}
