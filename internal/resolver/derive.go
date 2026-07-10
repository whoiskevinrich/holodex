package resolver

import (
	"slices"
	"strconv"
	"strings"
	"time"

	"holodex/internal/fieldsource"
	"holodex/internal/registry"
)

// Derive appends one computed ResolvedField per registered Computed canonical whose
// formula is applicable, modelled on AutoRegisterFields (F45, ADR-063 §D2). It is
// PURE: it reads only already-resolved input values by canonical out of `resolved`
// and takes `now` as its sole time source — nothing in this package reads the clock,
// so the resolver stays clock-free (ADR-051). The clock is injected by the caller at
// the API handler boundary.
//
// Each computed field's formula runs against the resolved input values; when it
// returns computable=true the value is inserted as a display-only row placed
// immediately after its primary dependency (so Age reads directly under Birthdate,
// spec D2). A missing/unparseable input yields no row (spec D3 — absent, never a
// placeholder). Emitted rows are stamped Computed=true with a "computed:<canonical>"
// winning source and nil Decision/Candidates/InSync — structurally non-adoptable.
func Derive(resolved []ResolvedField, now time.Time) []ResolvedField {
	values := firstValues(resolved)
	for _, def := range computedFields {
		fn := derivations[def.Canonical]
		if fn == nil {
			continue
		}
		value, ok := fn(values, now)
		if !ok {
			continue
		}
		resolved = insertComputed(resolved, def, value)
	}
	return resolved
}

// computedFields is the KnownFields subset with Computed=true, filtered once at init
// so the per-read Derive pass iterates only the derived entries rather than scanning
// the whole registry on every entity read.
var computedFields = func() []registry.FieldDef {
	var out []registry.FieldDef
	for _, def := range registry.KnownFields {
		if def.Computed {
			out = append(out, def)
		}
	}
	return out
}()

// formula computes a derived value from the resolved input values (keyed by canonical)
// and the injected clock. It returns computable=false when a required input is missing
// or unparseable, or when the field does not apply (e.g. deriveAge yields nothing once
// a deathdate is present). Formulas read the full input map so one can branch on a
// field outside its own DependsOn (deriveAge reads deathdate); DependsOn drives input
// gathering and the provenance labels, not the branch logic.
type formula func(in map[string]string, now time.Time) (value string, computable bool)

// derivations is the closed Go formula registry keyed by canonical (ADR-063 §D4) —
// no DSL, no user-authored formulas. Each key must be a registry.FieldDef with
// Computed=true.
var derivations = map[string]formula{
	"age":          deriveAge,
	"age_at_death": deriveAgeAtDeath,
}

// deriveAge computes floor(now − birthdate) in whole years for a LIVING person. A
// person with a deathdate shows age_at_death instead (D4), so this yields no row when
// a deathdate is present. Returns computable=false for an absent/unparseable birthdate
// or a birthdate in the future.
func deriveAge(in map[string]string, now time.Time) (string, bool) {
	if _, ok := parseDate(in["deathdate"]); ok {
		return "", false // deceased → age_at_death takes over
	}
	bd, ok := parseDate(in["birthdate"])
	if !ok {
		return "", false
	}
	age := wholeYearsBetween(bd, now)
	if age < 0 {
		return "", false // birthdate in the future — nonsensical, omit
	}
	return strconv.Itoa(age), true
}

// deriveAgeAtDeath computes floor(deathdate − birthdate) in whole years; it requires
// both inputs to be present and parseable, and that death is not before birth.
func deriveAgeAtDeath(in map[string]string, _ time.Time) (string, bool) {
	bd, ok := parseDate(in["birthdate"])
	if !ok {
		return "", false
	}
	dd, ok := parseDate(in["deathdate"])
	if !ok {
		return "", false
	}
	age := wholeYearsBetween(bd, dd)
	if age < 0 {
		return "", false // death before birth — nonsensical, omit
	}
	return strconv.Itoa(age), true
}

// parseDate parses a YYYY-MM-DD ISO date; any other value is treated as a missing
// input. Location is UTC — a date has no time-of-day, and the whole-year arithmetic
// compares calendar month/day only, so the zone never affects the result.
func parseDate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// wholeYearsBetween returns floor(end − start) in whole years by the birthday
// convention: the year difference, minus one when end has not yet reached start's
// month/day this year. Leap-day convention (ADR-063 §D4): a Feb-29 birthdate ticks
// over exactly once per year, between Feb-28 and Mar-01.
func wholeYearsBetween(start, end time.Time) int {
	years := end.Year() - start.Year()
	if end.Month() < start.Month() || (end.Month() == start.Month() && end.Day() < start.Day()) {
		years--
	}
	return years
}

// firstValues collects the first (winning) display value per canonical from the
// resolved rows — the input set the formulas read.
func firstValues(resolved []ResolvedField) map[string]string {
	out := make(map[string]string, len(resolved))
	for _, f := range resolved {
		if _, seen := out[f.Canonical]; !seen && len(f.Values) > 0 {
			out[f.Canonical] = f.Values[0]
		}
	}
	return out
}

// insertComputed builds the display-only computed row for def=value and inserts it
// immediately after its primary dependency (DependsOn[0]) so it renders adjacent to
// the field it is derived from (spec D2). When the dependency is not found it appends
// at the end (defensive; a computable field always has its dependency resolved).
func insertComputed(resolved []ResolvedField, def registry.FieldDef, value string) []ResolvedField {
	row := ResolvedField{
		Canonical:     def.Canonical,
		Label:         def.Label,
		Display:       def.Display,
		Values:        []string{value},
		WinningSource: fieldsource.ForComputed(def.Canonical),
		Computed:      true,
		DerivedFrom:   dependencyLabels(def.DependsOn),
	}
	at := len(resolved)
	if len(def.DependsOn) > 0 {
		for i, f := range resolved {
			if f.Canonical == def.DependsOn[0] {
				at = i + 1
				break
			}
		}
	}
	return slices.Insert(resolved, at, row)
}

// dependencyLabels maps each dependency canonical to its registry label (e.g.
// "birthdate" → "Born") for the transitive "calculated from …" provenance copy, so
// the phrase tracks whatever the dependency row is actually labelled.
func dependencyLabels(deps []string) []string {
	out := make([]string, len(deps))
	for i, dep := range deps {
		out[i] = registry.Lookup(dep).Label
	}
	return out
}
