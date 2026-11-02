package extract

import (
	"regexp"
	"strings"
)

// identityIDPattern bounds an identity marker's id to a provider's id characters (F86 P0-11,
// security-perimeter.md "Rename names"). A name read back may come from any file in the library, so
// nothing outside this set, and nothing longer, ever reaches a provider as a hint.
const identityIDPattern = `[A-Za-z0-9_-]{1,64}`

// LiftMarkers is liftMarkers for the rename read-back (F86), which parses a stem it built itself.
func LiftMarkers(stem string) (rest string, lifted map[string]string) { return liftMarkers(stem) }

// IdentityLifter lifts `{<provider>-<id>}` markers for a fixed set of providers. It runs only in
// the rename read-back: an owner's filename patterns parse exactly as before (F86 P0-11).
type IdentityLifter struct {
	providers []string
	res       []*regexp.Regexp
}

// NewIdentityLifter compiles one marker pattern per provider, once.
func NewIdentityLifter(providers []string) *IdentityLifter {
	l := &IdentityLifter{providers: providers}
	for _, p := range providers {
		l.res = append(l.res, regexp.MustCompile(`\s*\{`+regexp.QuoteMeta(p)+`-(`+identityIDPattern+`)\}\s*`))
	}
	return l
}

// Lift removes the first marker for each provider from stem, the way liftMarkers removes edition
// and part markers, and returns the id per provider.
func (l *IdentityLifter) Lift(stem string) (rest string, ids map[string]string) {
	rest = stem
	for i, re := range l.res {
		loc := re.FindStringSubmatchIndex(rest)
		if loc == nil {
			continue
		}
		if ids == nil {
			ids = make(map[string]string, len(l.res))
		}
		ids[l.providers[i]] = rest[loc[2]:loc[3]]
		rest = strings.TrimSpace(rest[:loc[0]] + " " + rest[loc[1]:])
	}
	return rest, ids
}
