package rename

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"

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
)

// plainTokens are the tokens the F48 compiler captures itself, by the field it reads back as.
var plainTokens = map[string]string{TokTitle: "title", TokStudio: "studio", TokPeople: "people", TokYear: "release_date"}

// IdentityToken is the template token for a provider's confirmed id.
func IdentityToken(provider string) string { return provider + "-id" }

var templateTokenRe = regexp.MustCompile(`\{([^{}]*)\}`)

// Template is one compiled rename template: the text the owner wrote, and the F48 pattern that
// reads its names back (architecture "One grammar both ways").
type Template struct {
	Text   string
	tokens []string
	ids    *extract.IdentityLifter
	// pattern is nil when nothing but markers is left once they are lifted; rest is then the
	// literal stem every name from this template reads back to.
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
	for _, p := range providers {
		known[IdentityToken(p)] = "{" + p + "-x}"
	}

	var tokens []string
	seen := map[string]bool{}
	var pat strings.Builder
	last := 0
	for _, loc := range templateTokenRe.FindAllStringSubmatchIndex(text, -1) {
		if err := checkLiteral(text[last:loc[0]]); err != nil {
			return nil, err
		}
		name := text[loc[2]:loc[3]]
		form, ok := known[name]
		if !ok {
			return nil, fmt.Errorf("{%s} isn't a token", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("{%s} appears twice", name)
		}
		seen[name] = true
		tokens = append(tokens, name)
		pat.WriteString(text[last:loc[0]])
		pat.WriteString(form)
		last = loc[1]
	}
	if err := checkLiteral(text[last:]); err != nil {
		return nil, err
	}
	pat.WriteString(text[last:])

	hasIdentity := false
	for _, p := range providers {
		hasIdentity = hasIdentity || seen[IdentityToken(p)]
	}
	if !seen[TokTitle] && !hasIdentity {
		return nil, errors.New("add {title} or an identity token: without one, unrelated videos could get the same name")
	}

	t := &Template{Text: text, tokens: tokens, ids: ids}
	rest, _ := ids.Lift(pat.String())
	rest, _ = extract.LiftMarkers(rest)
	if templateTokenRe.MatchString(rest) {
		p, err := extract.Compile(rest)
		if err != nil {
			return nil, err
		}
		t.pattern = p
	} else {
		t.rest = rest
	}
	return t, nil
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

// render fills the template from sanitised values. ok is false when a token has no value, so the
// template doesn't fit (P0-2). written records the value written per token (P0-10).
func (t *Template) render(v Values, delimiter string) (stem string, written map[string][]string, ok bool, err error) {
	written = map[string][]string{}
	var b strings.Builder
	last := 0
	for _, loc := range templateTokenRe.FindAllStringSubmatchIndex(t.Text, -1) {
		b.WriteString(t.Text[last:loc[0]])
		last = loc[1]
		tok := t.Text[loc[2]:loc[3]]
		vals, err := sanitizeAll(tokenValues(tok, v[tok]))
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
		case plainTokens[tok] != "":
			b.WriteString(vals[0])
		default: // identity
			b.WriteString("{" + strings.TrimSuffix(tok, "-id") + "-" + vals[0] + "}")
		}
	}
	b.WriteString(t.Text[last:])
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

func sanitizeAll(raw []string) ([]string, error) {
	var out []string
	for _, s := range raw {
		v, err := SanitizeValue(s)
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

// read parses a stem with this template alone, returning the value per token.
func (t *Template) read(stem, delimiter string) (map[string][]string, bool) {
	rest, ids := t.ids.Lift(stem)
	rest, markers := extract.LiftMarkers(rest)
	out := map[string][]string{}
	if t.pattern == nil {
		if rest != t.rest {
			return nil, false
		}
	} else {
		fields, ok := t.pattern.Match(rest, delimiter)
		if !ok {
			return nil, false
		}
		for tok, field := range plainTokens {
			if vals := fields[field]; len(vals) > 0 {
				out[tok] = vals
			}
		}
	}
	for f, v := range markers {
		out[f] = []string{v}
	}
	for p, id := range ids {
		out[IdentityToken(p)] = []string{id}
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
func Build(list []*Template, v Values, delimiter, ext string) (Name, error) {
	for i, t := range list {
		stem, written, ok, err := t.render(v, delimiter)
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
