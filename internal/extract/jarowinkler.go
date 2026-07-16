package extract

import (
	"strings"
	"unicode/utf8"
)

// FuzzyMatchThreshold is the Jaro-Winkler similarity cutoff above which a
// candidate counts as EntityMatch = MatchFuzzy rather than MatchNone
// (F48.3d), and the cutoff BestFuzzyMatch uses to decide whether it has a
// suggestion at all. Not specified numerically by the spec; chosen to match
// ADR-066's own StrongMatchThreshold (0.85) for consistency with the
// codebase's one other confidence cutoff. A v1 assumption, explicitly
// flagged as subject to empirical tuning (ADR-067 Action Item 4) — it never
// affects auto-apply directly (Route's exact-match gate does), only which
// candidates the review queue pre-fills as a suggestion.
const FuzzyMatchThreshold = 0.85

// FuzzyAgreementThreshold is the same cutoff applied to filename-vs-tag value
// agreement (Concepts & Model "Source agreement" fuzzy tier) — one constant
// so the two fuzzy notions can't silently drift apart.
const FuzzyAgreementThreshold = FuzzyMatchThreshold

// JaroWinkler returns the Jaro-Winkler similarity of a and b in [0, 1].
// Case-insensitive (folds like F43's own identity keys, though this is a
// fuzzy ranking signal, not an identity comparison — ADR-061's exact nameKey
// match is untouched by this function). Pure, no I/O.
func JaroWinkler(a, b string) float64 {
	a, b = strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	j := jaro(a, b)
	if j == 0 {
		return 0
	}
	prefix := commonPrefixLen(a, b, 4)
	return j + float64(prefix)*0.1*(1-j)
}

func jaro(a, b string) float64 {
	if a == b {
		return 1
	}
	r1, r2 := []rune(a), []rune(b)
	len1, len2 := len(r1), len(r2)
	if len1 == 0 || len2 == 0 {
		return 0
	}

	matchDistance := max(len1, len2)/2 - 1
	if matchDistance < 0 {
		matchDistance = 0
	}

	s1Matches := make([]bool, len1)
	s2Matches := make([]bool, len2)
	matches := 0
	for i := range r1 {
		start := max(0, i-matchDistance)
		end := min(i+matchDistance+1, len2)
		for j := start; j < end; j++ {
			if s2Matches[j] || r1[i] != r2[j] {
				continue
			}
			s1Matches[i] = true
			s2Matches[j] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}

	transpositions := 0
	k := 0
	for i := range r1 {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if r1[i] != r2[k] {
			transpositions++
		}
		k++
	}
	transpositions /= 2

	m := float64(matches)
	return (m/float64(len1) + m/float64(len2) + (m-float64(transpositions))/m) / 3
}

// commonPrefixLen returns the length of the common prefix of a and b, capped
// at max (Winkler's standard cap of 4 runes).
func commonPrefixLen(a, b string, max int) int {
	r1, r2 := []rune(a), []rune(b)
	n := 0
	for n < len(r1) && n < len(r2) && n < max && r1[n] == r2[n] {
		n++
	}
	return n
}

// classifyAgreement buckets how a filename-derived value compares to the
// existing tag value for the same field (Concepts & Model, "Source
// agreement"). tagJoined == "" means the tag has no data — the field is
// filename-only (AgreementSingleSource). Callers only invoke this once a
// filename value is known to exist (Process never scores a field with
// nothing new to contribute).
func classifyAgreement(filenameJoined, tagJoined string) Agreement {
	if strings.TrimSpace(tagJoined) == "" {
		return AgreementSingleSource
	}
	if strings.EqualFold(strings.TrimSpace(filenameJoined), strings.TrimSpace(tagJoined)) {
		return AgreementExact
	}
	if JaroWinkler(filenameJoined, tagJoined) >= FuzzyAgreementThreshold {
		return AgreementFuzzy
	}
	return AgreementConflict
}

// classifySpecificity buckets a single extracted value's structuredness
// (Concepts & Model, "Value specificity"). entity fields use word count
// (multi-word vs. single-word); non-entity fields use a coarse length
// heuristic — a v1 approximation, since the spec doesn't define "structured/
// complete" numerically (subject to empirical tuning, ADR-067 Action Item 4).
func classifySpecificity(value string, entity bool) Specificity {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return SpecificityGarbled
	}
	if entity {
		if len(strings.Fields(trimmed)) >= 2 {
			return SpecificityFull
		}
		return SpecificityPartial
	}
	if utf8.RuneCountInString(trimmed) < 3 {
		return SpecificityPartial
	}
	return SpecificityFull
}

// BestFuzzyMatch ranks name against every candidate (id -> name) by
// Jaro-Winkler similarity and returns the closest match at or above
// FuzzyMatchThreshold (F48.3d's advisory suggestion for the review queue —
// never itself a resolution). ok is false when no candidate clears the
// threshold, or candidates is empty.
func BestFuzzyMatch(name string, candidates map[int64]string) (id int64, score float64, ok bool) {
	for cid, cname := range candidates {
		if s := JaroWinkler(name, cname); s > score {
			score, id = s, cid
		}
	}
	if score < FuzzyMatchThreshold {
		return 0, score, false
	}
	return id, score, true
}
