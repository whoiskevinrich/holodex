// Search-query pattern rendering (F54, ADR-080): a per-provider search query built
// from a video's already-resolved fields instead of its raw title. This is the
// inverse direction of internal/extract's {token} filename-MATCHING grammar
// (fields → string here, vs. string → fields there) — a deliberate non-reuse (D3);
// the two share only the {name}/{name?} placeholder shape.
package enrich

import (
	"regexp"
	"strings"
)

// queryTokenNames is the fixed vocabulary a search_pattern/preferred_search_pattern/
// default_search_pattern may reference (ADR-080 D3). Unknown names make the whole
// pattern invalid — see ValidatePattern.
var queryTokenNames = map[string]bool{
	"studio":     true,
	"title":      true,
	"performers": true,
	"year":       true,
}

// performersCap bounds the {performers} token to avoid a runaway-long query (ADR-080
// D3) — the top N of the combined actors+director list, in that order.
const performersCap = 3

// queryToken is one parsed {name} or {name?} placeholder.
type queryToken struct {
	name     string
	optional bool
}

// queryTokenRe matches exactly one bare "{name}" or "{name?}" placeholder — the only
// shape a pattern token may take (no literal decoration, ADR-080 Non-Goals).
var queryTokenRe = regexp.MustCompile(`^\{([a-z]+)(\??)\}$`)

// parseQueryPattern parses a raw pattern string into an ordered token list. ok is
// false when the pattern is empty, contains anything other than whitespace-separated
// bare {name}/{name?} tokens, or references a name outside queryTokenNames — the
// single validity gate both config-load validation (validatedPattern) and render-time
// rendering (renderPattern) share.
func parseQueryPattern(pattern string) (tokens []queryToken, ok bool) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil, false
	}
	parts := strings.Fields(pattern)
	tokens = make([]queryToken, 0, len(parts))
	for _, part := range parts {
		m := queryTokenRe.FindStringSubmatch(part)
		if m == nil || !queryTokenNames[m[1]] {
			return nil, false
		}
		tokens = append(tokens, queryToken{name: m[1], optional: m[2] == "?"})
	}
	return tokens, true
}

// ValidatePattern reports whether pattern is a well-formed search-query pattern
// (ADR-080 D3) — exported so config ingestion (enrich.go's validatedPattern, and the
// Service's /describe preferred-pattern cache) can reject a malformed pattern once,
// at load/fetch time, rather than BuildQuery silently degrading on every render.
func ValidatePattern(pattern string) bool {
	_, ok := parseQueryPattern(pattern)
	return ok
}

// QueryFields holds the already-resolved video fields BuildQuery renders a pattern's
// tokens from (ADR-080 D3). Studio/Title are the field's top-precedence resolved
// value; Performers is the full, uncapped, ordered actors-then-director list (capping
// to performersCap happens at render time, in one place); ReleaseDate is the raw
// resolved value (e.g. "2023-08-01"), from which the {year} token parses a leading
// 4-digit year. Deliberately plain strings, not resolver.ResolvedField, so this
// package stays free of a dependency on internal/resolver — callers (internal/api)
// do that translation.
type QueryFields struct {
	Studio      string
	Title       string
	Performers  []string
	ReleaseDate string
}

// bracketCommaRe matches the punctuation the title sanitizer deletes outright (ADR-080
// D4): bracket/paren/brace characters and commas. Their contents are kept as plain
// words; only the punctuation itself is removed.
var bracketCommaRe = regexp.MustCompile(`[\[\](){},]`)

// resolutionRe matches a resolution/quality token — 3-4 digits followed by a literal
// "p" (480p/720p/1080p/2160p), or a literal "4k"/"8k" — case-insensitive and
// word-bounded so it never eats a real digit sequence that happens to end in p/k
// (e.g. "Agent 007", "Suite 1080" are both left untouched).
var resolutionRe = regexp.MustCompile(`(?i)\b\d{3,4}p\b|\b[48]k\b`)

// whitespaceRe collapses the runs of whitespace punctuation-stripping leaves behind.
var whitespaceRe = regexp.MustCompile(`\s+`)

// yearRe pulls a leading 4-digit year off a resolved release_date value (e.g.
// "2023-08-01" -> "2023"); a missing or non-ISO value simply produces no match.
var yearRe = regexp.MustCompile(`^\d{4}`)

// sanitizeTitle strips bracket/paren/brace/comma punctuation and resolution/quality
// tokens from a title and collapses whitespace (ADR-080 D4) — applied unconditionally
// to the {title} token and the raw-title floor tier, with no config gate. If
// stripping would leave nothing (a degenerate title that is only bracket/resolution
// noise, e.g. "[720p]"), the original input is returned unchanged instead of an empty
// string — the search box must never be seeded blank (spec AC-8a).
func sanitizeTitle(s string) string {
	stripped := bracketCommaRe.ReplaceAllString(s, "")
	stripped = resolutionRe.ReplaceAllString(stripped, "")
	stripped = strings.TrimSpace(whitespaceRe.ReplaceAllString(stripped, " "))
	if stripped == "" {
		return s
	}
	return stripped
}

// SanitizeTitle exposes sanitizeTitle for the one other raw-title call site outside
// this package: refresh-all's automated hint builder (internal/api's
// enrichQueryHint), which has no per-provider pattern context to render against but
// still owes the same unconditional D4 floor-tier cleanup.
func SanitizeTitle(s string) string { return sanitizeTitle(s) }

// tokenValue resolves one token's substitution value from fields, or "" if the
// underlying data is absent — the caller (renderPattern) decides whether that empty
// value drops the token (optional) or fails the whole tier (required).
func tokenValue(name string, fields QueryFields) string {
	switch name {
	case "studio":
		return strings.TrimSpace(fields.Studio)
	case "title":
		return sanitizeTitle(strings.TrimSpace(fields.Title))
	case "performers":
		top := fields.Performers
		if len(top) > performersCap {
			top = top[:performersCap]
		}
		return strings.Join(top, " ")
	case "year":
		return yearRe.FindString(strings.TrimSpace(fields.ReleaseDate))
	default:
		return "" // unreachable: parseQueryPattern already rejects unknown names
	}
}

// renderPattern renders one precedence tier. ok is false when the pattern itself is
// malformed (parseQueryPattern), or when a required (non-"?") token has no value —
// per ADR-080 D3, that failure drops the WHOLE tier, it never renders with a gap
// where the missing token would have been. An optional token with no value is simply
// omitted. ok is also false for a pattern that renders zero non-empty tokens (e.g.
// every token in it was optional and absent).
func renderPattern(pattern string, fields QueryFields) (string, bool) {
	tokens, ok := parseQueryPattern(pattern)
	if !ok {
		return "", false
	}
	parts := make([]string, 0, len(tokens))
	for _, t := range tokens {
		val := tokenValue(t.name, fields)
		if val == "" {
			if t.optional {
				continue
			}
			return "", false
		}
		parts = append(parts, val)
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, " "), true
}

// BuildQuery renders this provider's search query from fields, walking ADR-080 D2's
// precedence chain highest-to-lowest: this Source's own operator-configured
// SearchPattern, then preferredPattern (the provider's /describe-advertised
// preference, already validated/cached by the caller — Service.PreferredSearchPattern),
// then defaultPattern (the operator's fleet-wide default, Registry.DefaultSearchPattern).
// The first tier that renders (parses, and has at least one non-empty token) wins.
// If none does — including "nothing configured at all" — the result is the D4
// sanitized-title floor, which (per sanitizeTitle's own empty-input guard) is never
// itself empty as long as fields.Title is non-empty. BuildQuery never returns "" for
// a video with any title at all; a caller with a genuinely empty title gets "" back,
// same as today's behavior in that (pathological) case.
func (s Source) BuildQuery(fields QueryFields, preferredPattern, defaultPattern string) string {
	for _, pattern := range []string{s.SearchPattern, preferredPattern, defaultPattern} {
		if q, ok := renderPattern(pattern, fields); ok {
			return q
		}
	}
	return sanitizeTitle(strings.TrimSpace(fields.Title))
}
