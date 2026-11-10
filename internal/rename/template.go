package rename

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"holodex/internal/extract"
)

// Template tokens (P0-1). An identity token is `{<provider>-id}`, one per configured provider.
const (
	TokTitle   = "title"
	TokStudio  = "studio"
	TokPeople  = "people"
	TokYear    = "year"
	TokEdition = "edition"
	TokPart    = "part"
	TokFilm    = "film"
)

// filmMarkerRe matches the `{film-<name>}` marker {film} renders. A value never holds a brace
// (P0-4), so the marker can't be forged from one.
var filmMarkerRe = regexp.MustCompile(`\s*\{film-([^{}]+)\}\s*`)

// liftFilm removes the first film marker from stem, the way extract.LiftMarkers removes edition
// and part markers, and returns the film's name.
func liftFilm(stem string) (rest, film string) {
	loc := filmMarkerRe.FindStringSubmatchIndex(stem)
	if loc == nil {
		return stem, ""
	}
	return strings.TrimSpace(stem[:loc[0]] + " " + stem[loc[1]:]), stem[loc[2]:loc[3]]
}

// plainTokens are the tokens the F48 compiler captures itself, by the field it reads back as.
var plainTokens = map[string]string{TokTitle: "title", TokStudio: "studio", TokPeople: "people", TokYear: "release_date"}

// IdentityToken is the template token for a provider's confirmed id.
func IdentityToken(provider string) string { return provider + "-id" }

var templateTokenRe = regexp.MustCompile(`\{([^{}]*)\}`)

// maxOptional bounds a template's optional tokens (P0-1): each one doubles its variants.
const maxOptional = 6

// Template is one compiled rename template: the text the owner wrote, and the plain templates it
// stands for once its optional tokens are kept or dropped (P0-1), most tokens kept first.
type Template struct {
	Text     string
	ids      *extract.IdentityLifter
	variants []*variant
}

// variant is one plain template: its text, and the F48 pattern that reads its names back
// (architecture "One grammar both ways").
type variant struct {
	text string
	// pattern is nil when nothing but markers is left once they are lifted; rest is then the
	// literal stem every name from this variant reads back to.
	pattern *extract.Pattern
	rest    string
}

// ParseTemplate validates one template against P0-1 and compiles it. providers are the configured
// provider names; an identity token for any other provider is unknown.
func ParseTemplate(text string, providers []string) (*Template, error) {
	return parseTemplate(text, providers, extract.NewIdentityLifter(providers))
}

func parseTemplate(text string, providers []string, ids *extract.IdentityLifter) (*Template, error) {
	text = norm.NFC.String(strings.TrimSpace(text))
	if text == "" {
		return nil, errors.New("a template can't be empty")
	}
	known := map[string]string{} // token → its marker form for the F48 pattern
	for t := range plainTokens {
		known[t] = "{" + t + "}"
	}
	known[TokEdition] = "{edition-e}"
	known[TokPart] = "{part-1}"
	known[TokFilm] = "{film-x}"
	for _, p := range providers {
		known[IdentityToken(p)] = "{" + p + "-x}"
	}

	seen := map[string]bool{}
	required := map[string]bool{}
	var optional []string // in written order
	last := 0
	for _, loc := range templateTokenRe.FindAllStringSubmatchIndex(text, -1) {
		if err := checkLiteral(text[last:loc[0]]); err != nil {
			return nil, err
		}
		name, opt := strings.CutSuffix(text[loc[2]:loc[3]], "?")
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("{%s} isn't a token", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("{%s} appears twice", name)
		}
		seen[name] = true
		if opt {
			optional = append(optional, name)
		} else {
			required[name] = true
		}
		last = loc[1]
	}
	if err := checkLiteral(text[last:]); err != nil {
		return nil, err
	}

	hasIdentity, anyIdentity := false, false
	for _, p := range providers {
		hasIdentity = hasIdentity || required[IdentityToken(p)]
		anyIdentity = anyIdentity || seen[IdentityToken(p)]
	}
	if !required[TokTitle] && !hasIdentity {
		if seen[TokTitle] || anyIdentity {
			return nil, errors.New("keep {title} or an identity token without ?: without one, unrelated videos could get the same name")
		}
		return nil, errors.New("add {title} or an identity token: without one, unrelated videos could get the same name")
	}
	if len(optional) > maxOptional {
		return nil, fmt.Errorf("at most %d tokens can be optional", maxOptional)
	}

	t := &Template{Text: text, ids: ids}
	compiled := map[string]*extract.Pattern{} // variants that differ only in markers share one
	for _, drop := range dropSets(len(optional)) {
		dropped := map[string]bool{}
		for _, i := range drop {
			dropped[optional[i]] = true
		}
		v, err := compileVariant(dropOptional(text, dropped), known, ids, compiled)
		if err != nil {
			return nil, err
		}
		t.variants = append(t.variants, v)
	}
	return t, nil
}

// compileVariant compiles one plain template, already checked, into the pattern that reads its
// names back.
func compileVariant(text string, known map[string]string, ids *extract.IdentityLifter, compiled map[string]*extract.Pattern) (*variant, error) {
	v := &variant{text: text}
	var pat strings.Builder
	last := 0
	for _, loc := range templateTokenRe.FindAllStringSubmatchIndex(text, -1) {
		pat.WriteString(text[last:loc[0]])
		pat.WriteString(known[text[loc[2]:loc[3]]])
		last = loc[1]
	}
	pat.WriteString(text[last:])
	rest, _ := ids.Lift(pat.String())
	rest, _ = liftFilm(rest)
	rest, _ = extract.LiftMarkers(rest)
	if !templateTokenRe.MatchString(rest) {
		v.rest = rest
		return v, nil
	}
	if p, ok := compiled[rest]; ok {
		v.pattern = p
		return v, nil
	}
	p, err := extract.Compile(rest)
	if err != nil {
		return nil, err
	}
	compiled[rest] = p
	v.pattern = p
	return v, nil
}

// dropSets lists every subset of n optional tokens, by index: the empty set first, then each size
// in turn, each in written order.
func dropSets(n int) [][]int {
	sets := [][]int{nil}
	for size := 1; size <= n; size++ {
		var walk func(from int, cur []int)
		walk = func(from int, cur []int) {
			if len(cur) == size {
				sets = append(sets, append([]int(nil), cur...))
				return
			}
			for i := from; i < n; i++ {
				walk(i+1, append(cur, i))
			}
		}
		walk(0, nil)
	}
	return sets
}

// dropOptional gives the plain template a template stands for when its dropped optional tokens
// have no value (P0-1). Each one goes with the literal text touching it, up to a space or a
// neighbouring token, and the space it leaves is closed up. Every kept token loses its ?.
func dropOptional(text string, dropped map[string]bool) string {
	locs := templateTokenRe.FindAllStringSubmatchIndex(text, -1)
	var b strings.Builder
	last := 0 // text[:last] is written or dropped
	for i, loc := range locs {
		name, _ := strings.CutSuffix(text[loc[2]:loc[3]], "?")
		if !dropped[name] {
			b.WriteString(text[last:loc[0]] + "{" + name + "}")
			last = loc[1]
			continue
		}
		start := last
		if j := strings.LastIndexFunc(text[last:loc[0]], unicode.IsSpace); j >= 0 {
			_, size := utf8.DecodeRuneInString(text[last+j:])
			start = last + j + size
		}
		b.WriteString(text[last:start])
		next := len(text)
		if i+1 < len(locs) {
			next = locs[i+1][0]
		}
		end := next
		if j := strings.IndexFunc(text[loc[1]:next], unicode.IsSpace); j >= 0 {
			end = loc[1] + j
		}
		if out := b.String(); out == "" || strings.TrimRightFunc(out, unicode.IsSpace) != out {
			end += len(text[end:next]) - len(strings.TrimLeftFunc(text[end:next], unicode.IsSpace))
		}
		last = end
	}
	b.WriteString(text[last:])
	return strings.TrimSpace(b.String())
}

// checkLiteral applies P0-4's character rules to a template's own text: a character P0-4 removes
// from values is refused, never cleaned silently.
func checkLiteral(s string) error {
	for _, r := range s {
		if r == '{' || r == '}' {
			return errors.New("a brace outside a token")
		}
		if removedRune(r) {
			if unicode.IsPrint(r) {
				return fmt.Errorf("%q can't be in a file name", r)
			}
			return errors.New("an invisible or control character can't be in a file name")
		}
	}
	return nil
}

// ParseList validates an ordered template list. errs has one entry per row (nil when the row is
// valid) and list is nil when any row is invalid.
func ParseList(texts []string, providers []string) (list []*Template, errs []error) {
	errs = make([]error, len(texts))
	bad := len(texts) == 0
	ids := extract.NewIdentityLifter(providers)
	for i, s := range texts {
		t, err := parseTemplate(s, providers, ids)
		if err != nil {
			errs[i], bad = err, true
			continue
		}
		list = append(list, t)
	}
	if bad {
		return nil, errs
	}
	return list, errs
}

// ListError turns ParseList's per-row errors into the refusal a save or a preview returns.
func ListError(errs []error) *TemplateListError {
	rows := make([]string, len(errs))
	for i, e := range errs {
		if e != nil {
			rows[i] = e.Error()
		}
	}
	return &TemplateListError{Rows: rows}
}

// Values are a video's raw resolved values by token. A token with no value is absent or empty.
type Values map[string][]string

// render fills the template from sanitised values with its first variant that fits: the one that
// keeps every optional token with a value. ok is false when a required token has no value, so the
// template doesn't fit (P0-2). written records the value written per token (P0-10).
func (t *Template) render(v Values, delimiter string, chars Characters) (stem string, written map[string][]string, ok bool, err error) {
	for _, vr := range t.variants {
		if stem, written, ok, err = vr.render(v, delimiter, chars); ok || err != nil {
			return stem, written, ok, err
		}
	}
	return "", nil, false, nil
}

func (vr *variant) render(v Values, delimiter string, chars Characters) (stem string, written map[string][]string, ok bool, err error) {
	written = map[string][]string{}
	var b strings.Builder
	last := 0
	for _, loc := range templateTokenRe.FindAllStringSubmatchIndex(vr.text, -1) {
		b.WriteString(vr.text[last:loc[0]])
		last = loc[1]
		tok := vr.text[loc[2]:loc[3]]
		vals, err := sanitizeAll(tokenValues(tok, v[tok]), chars)
		if err != nil {
			return "", nil, false, err
		}
		if len(vals) == 0 {
			return "", nil, false, nil
		}
		written[tok] = vals
		switch {
		case tok == TokPeople:
			b.WriteString(strings.Join(vals, delimiterOr(delimiter)))
		case tok == TokEdition:
			b.WriteString("{edition-" + vals[0] + "}")
		case tok == TokPart:
			b.WriteString("{part-" + vals[0] + "}")
		case tok == TokFilm:
			b.WriteString("{film-" + vals[0] + "}")
		case plainTokens[tok] != "":
			b.WriteString(vals[0])
		default: // identity
			b.WriteString("{" + strings.TrimSuffix(tok, "-id") + "-" + vals[0] + "}")
		}
	}
	b.WriteString(vr.text[last:])
	return b.String(), written, true, nil
}

// tokenValues picks the values a token renders: every cast name for {people}, the four-digit year
// of a release date for {year}, else the first value.
func tokenValues(tok string, raw []string) []string {
	switch {
	case len(raw) == 0:
		return nil
	case tok == TokPeople:
		return raw
	case tok == TokYear:
		if len(raw[0]) >= 4 && isDigits(raw[0][:4]) {
			return []string{raw[0][:4]}
		}
		return nil
	}
	return raw[:1]
}

func sanitizeAll(raw []string, chars Characters) ([]string, error) {
	var out []string
	for _, s := range raw {
		v, err := SanitizeValue(s, chars)
		if err != nil {
			return nil, err
		}
		if v != "" {
			out = append(out, v)
		}
	}
	return out, nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

func delimiterOr(d string) string {
	if d == "" {
		return extract.DefaultDelimiter
	}
	return d
}

// read parses a stem with this template alone, by its first variant that fits, returning the value
// per token.
func (t *Template) read(stem, delimiter string) (map[string][]string, bool) {
	rest, ids := t.ids.Lift(stem)
	rest, film := liftFilm(rest)
	rest, markers := extract.LiftMarkers(rest)
	for _, vr := range t.variants {
		if out, ok := vr.read(rest, delimiter); ok {
			for f, v := range markers {
				out[f] = []string{v}
			}
			if film != "" {
				out[TokFilm] = []string{film}
			}
			for p, id := range ids {
				out[IdentityToken(p)] = []string{id}
			}
			return out, true
		}
	}
	return nil, false
}

// read matches what is left of a stem once its markers are lifted.
func (vr *variant) read(rest, delimiter string) (map[string][]string, bool) {
	out := map[string][]string{}
	if vr.pattern == nil {
		if rest != vr.rest {
			return nil, false
		}
	} else {
		fields, ok := vr.pattern.Match(rest, delimiter)
		if !ok {
			return nil, false
		}
		for tok, field := range plainTokens {
			if vals := fields[field]; len(vals) > 0 {
				out[tok] = vals
			}
		}
	}
	return out, true
}

// ReadBack parses a stem with the first template in list that fits it (P0-11), with the
// disambiguator removed first.
func ReadBack(list []*Template, stem, delimiter string) (values map[string][]string, idx int, ok bool) {
	stem = StripDisambiguator(stem)
	for i, t := range list {
		if v, ok := t.read(stem, delimiter); ok {
			return v, i, true
		}
	}
	return nil, -1, false
}

var disambiguatorRe = regexp.MustCompile(`\s*\{hx-[0-9]+\}$`)

// StripDisambiguator removes a trailing ` {hx-<id>}` from a stem; it never becomes a value (P0-11).
func StripDisambiguator(stem string) string { return disambiguatorRe.ReplaceAllString(stem, "") }

// ReadBackError refuses a name whose read-back differs from the values written (P0-3).
type ReadBackError struct {
	Parsed map[string][]string
}

func (e *ReadBackError) Error() string { return "the name would not read back as the values written" }

// ErrNoTemplate means no template has a value for every token (P0-2).
var ErrNoTemplate = errors.New("no template fits")

// Name is a built name: the file name, the template that gave it and the value per token.
type Name struct {
	Name     string
	Template string
	Index    int
	Written  map[string][]string
}

// Build gives a video's desired name (P0-2): the first template whose every token has a value,
// read back with the whole list (P0-3) and checked whole (P0-4, P0-5). ext is the current file's
// extension, which never changes. A refusal never falls through to a later template.
func Build(list []*Template, v Values, delimiter string, chars Characters, ext string) (Name, error) {
	for i, t := range list {
		stem, written, ok, err := t.render(v, delimiter, chars)
		if err != nil {
			return Name{}, err
		}
		if !ok {
			continue
		}
		n := Name{Name: stem + ext, Template: t.Text, Index: i, Written: written}
		if err := CheckName(n.Name); err != nil {
			return n, err
		}
		parsed, _, ok := ReadBack(list, stem, delimiter)
		if !ok || !reflect.DeepEqual(parsed, written) {
			return n, &ReadBackError{Parsed: parsed}
		}
		return n, nil
	}
	return Name{}, ErrNoTemplate
}
