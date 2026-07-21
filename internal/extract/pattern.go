// Package extract implements on-demand filename metadata extraction (F48,
// ADR-067): parsing a filename against an owner-configured token grammar and
// feeding the result into the entity_enrichment shadow store as a new "filename"
// namespace, alongside "file"/"tmdb" (ADR-033). Pattern compilation and matching
// are pure — no I/O, no network — mirroring the F27 resolver's own posture.
package extract

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// DefaultDelimiter separates values within a multi-value token (e.g. {people})
// when no delimiter is configured (F48.1d).
const DefaultDelimiter = ", "

// tokenFields maps a recognized pattern token name to the canonical field key it
// produces (Concepts & Model). A token not present here is still matched
// (consumed, so it participates in matching literal boundaries) but produces no
// output value — e.g. {resolution} (F48.1c).
var tokenFields = map[string]string{
	"studio": "studio",
	"title":  "title",
	"people": "people",
	"year":   "release_date", // year granularity
}

var tokenRe = regexp.MustCompile(`\{(\w+)\}`)

// bareYearRe matches a value that is nothing but a 4-digit year. Such a value
// in the {people} position is a misparse, not a name — e.g. the leading
// parenthetical of "[Studio] Title (2011) (1080p)" matches {people} under the
// "({people}) ({resolution})" pattern. A real person is never named "2011", so
// these are dropped from the people split (a year belongs to {year}).
var bareYearRe = regexp.MustCompile(`^\d{4}$`)

// Pattern is a compiled filename token pattern (F48.1), ready to match against a
// filename stem (the base name with its extension removed).
type Pattern struct {
	raw string
	re  *regexp.Regexp
}

// String returns the original pattern text, for logging/display.
func (p *Pattern) String() string { return p.raw }

// Compile parses a token-grammar pattern string, e.g.
// "[{studio}] {title} ({people}, {year}) {resolution}", into a Pattern. Literal
// text (brackets, parens, spaces, punctuation) is matched verbatim; each
// {token} becomes a capture group. Compilation is pure and validates the
// grammar (F48.1a's "rejects unparseable token grammar" hook) — it performs no
// I/O and touches no filesystem or network.
func Compile(pattern string) (*Pattern, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, fmt.Errorf("extract: empty pattern")
	}

	locs := tokenRe.FindAllStringSubmatchIndex(pattern, -1)
	if len(locs) == 0 {
		return nil, fmt.Errorf("extract: pattern %q has no {token} placeholders", pattern)
	}

	var sb strings.Builder
	sb.WriteString("^")
	seen := make(map[string]bool, len(locs))
	last := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		name := pattern[loc[2]:loc[3]]
		if seen[name] {
			return nil, fmt.Errorf("extract: pattern %q repeats token {%s}", pattern, name)
		}
		seen[name] = true

		sb.WriteString(regexp.QuoteMeta(pattern[last:start]))
		if name == "year" {
			// Year is matched strictly as 4 digits so a following literal
			// delimiter (e.g. the ", " before it in "{people}, {year}") has an
			// unambiguous boundary to anchor on.
			fmt.Fprintf(&sb, "(?P<%s>\\d{4})", name)
		} else {
			fmt.Fprintf(&sb, "(?P<%s>.+?)", name)
		}
		last = end
	}
	sb.WriteString(regexp.QuoteMeta(pattern[last:]))
	sb.WriteString("$")

	re, err := regexp.Compile(sb.String())
	if err != nil {
		return nil, fmt.Errorf("extract: compile pattern %q: %w", pattern, err)
	}
	return &Pattern{raw: pattern, re: re}, nil
}

// CompileAll compiles an ordered list of pattern strings, stopping at the first
// invalid one (F48.1a's save-time validation).
func CompileAll(patterns []string) ([]*Pattern, error) {
	out := make([]*Pattern, 0, len(patterns))
	for _, raw := range patterns {
		p, err := Compile(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// Match tries filenameStem (no extension) against the pattern, returning the
// mapped canonical-field values on a full match. delimiter splits multi-value
// tokens like {people}; an empty delimiter falls back to DefaultDelimiter.
// ok is false when the pattern doesn't fully match the stem (F48.1b).
func (p *Pattern) Match(filenameStem, delimiter string) (fields map[string][]string, ok bool) {
	m := p.re.FindStringSubmatch(filenameStem)
	if m == nil {
		return nil, false
	}
	if delimiter == "" {
		delimiter = DefaultDelimiter
	}

	out := make(map[string][]string)
	for i, name := range p.re.SubexpNames() {
		if i == 0 || name == "" {
			continue
		}
		val := strings.TrimSpace(m[i])
		if val == "" {
			continue
		}
		field, mapped := tokenFields[name]
		if !mapped {
			continue // consumed but ignored (F48.1c), e.g. {resolution}
		}
		if multiValueFields[name] {
			// A multi-value token (F48.1d — {people}): split its captured text
			// into several values on the configured delimiter, dropping any
			// bare 4-digit year misparsed into the people position.
			for _, p := range splitValues(val, delimiter) {
				if !bareYearRe.MatchString(p) {
					out[field] = append(out[field], p)
				}
			}
		} else {
			out[field] = append(out[field], val)
		}
	}
	return out, true
}

// MatchFirst tries filename's stem against each pattern in order and returns
// the first full match (F48.1b) — "one convention among many" is safe because a
// file that matches none of them yields ok=false and falls through to tag-only
// resolution unchanged.
func MatchFirst(patterns []*Pattern, filename, delimiter string) (fields map[string][]string, ok bool) {
	stem := stemOf(filename)
	for _, p := range patterns {
		if fields, ok := p.Match(stem, delimiter); ok {
			return fields, true
		}
	}
	return nil, false
}

// stemOf returns the filename's base name with its extension removed, matching
// the scanner's own filename-stem convention (internal/scanner/scanner.go).
func stemOf(filename string) string {
	base := filepath.Base(filename)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// splitValues splits raw on delimiter, trimming and dropping empty parts.
func splitValues(raw, delimiter string) []string {
	parts := strings.Split(raw, delimiter)
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
