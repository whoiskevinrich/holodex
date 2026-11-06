package rename

import (
	"errors"
	"math/rand"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/quick"
)

var providers = []string{"provider-1"}

func mustList(t *testing.T, texts ...string) []*Template {
	t.Helper()
	list, errs := ParseList(texts, providers)
	if list == nil {
		t.Fatalf("ParseList(%q): %v", texts, errs)
	}
	return list
}

// The §39.1 fixture list.
func fixtureList(t *testing.T) []*Template {
	return mustList(t, "[{studio}] {title}", "{title} - {studio}", "{title} {provider-1-id}", "{title}")
}

// N1: the first template whose every token has a value names the video.
func TestBuildFirstFittingTemplate(t *testing.T) {
	list := fixtureList(t)
	cases := []struct {
		v    Values
		want string
	}{
		{Values{"title": {"Ada Lovelace"}, "studio": {"Acme"}}, "[Acme] Ada Lovelace.mkv"},
		{Values{"title": {"Ada Lovelace"}}, "Ada Lovelace.mkv"},
		{Values{"title": {"Ada Lovelace"}, "provider-1-id": {"42"}}, "Ada Lovelace {provider-1-42}.mkv"},
	}
	for _, c := range cases {
		n, err := Build(list, c.v, "", ".mkv")
		if err != nil || n.Name != c.want {
			t.Errorf("Build(%v) = %q, %v; want %q", c.v, n.Name, err, c.want)
		}
	}
	if _, err := Build(list, Values{"studio": {"Acme"}}, "", ".mkv"); !errors.Is(err, ErrNoTemplate) {
		t.Errorf("no title, no id: err = %v, want ErrNoTemplate", err)
	}
}

// Spec P0-2's worked example.
func TestBuildSpecExample(t *testing.T) {
	list := mustList(t, "[{studio}] {title} ({year})", "{title} ({year})", "{title}")
	for _, c := range []struct {
		v    Values
		want string
	}{
		{Values{"title": {"Ada Lovelace"}, "studio": {"Acme"}, "year": {"2023-05-01"}}, "[Acme] Ada Lovelace (2023).mkv"},
		{Values{"title": {"Ada Lovelace"}, "year": {"2023"}}, "Ada Lovelace (2023).mkv"},
		{Values{"title": {"Ada Lovelace"}}, "Ada Lovelace.mkv"},
	} {
		if n, err := Build(list, c.v, "", ".mkv"); err != nil || n.Name != c.want {
			t.Errorf("got %q, %v; want %q", n.Name, err, c.want)
		}
	}
}

// N20: optional tokens drop with the literal text touching them, and every name reads back.
func TestBuildOptionalTokens(t *testing.T) {
	list := mustList(t, "[{studio?}] {title} ({people?}) {year?} {part?} {edition?}")
	all := Values{"title": {"My Title"}, "studio": {"My Studio"}, "people": {"Person One", "Person Two"},
		"year": {"2026"}, "part": {"2"}, "edition": {"My Edition"}}
	for _, c := range []struct {
		v    Values
		want string
	}{
		{all, "[My Studio] My Title (Person One, Person Two) 2026 {part-2} {edition-My Edition}.mkv"},
		{Values{"title": {"My Title"}, "year": {"2026"}}, "My Title 2026.mkv"},
		{Values{"title": {"My Title"}}, "My Title.mkv"},
		{Values{"title": {"My Title"}, "people": {"Person One"}, "edition": {"Final"}}, "My Title (Person One) {edition-Final}.mkv"},
	} {
		if n, err := Build(list, c.v, ", ", ".mkv"); err != nil || n.Name != c.want {
			t.Errorf("Build(%v) = %q, %v; want %q", c.v, n.Name, err, c.want)
		}
	}
	if _, err := Build(list, Values{"studio": {"My Studio"}}, "", ".mkv"); !errors.Is(err, ErrNoTemplate) {
		t.Errorf("no title: err = %v, want ErrNoTemplate", err)
	}
}

func TestDropOptional(t *testing.T) {
	for _, c := range []struct{ text, drop, want string }{
		{"[{studio?}] {title}", "studio", "{title}"},
		{"{title} ({people?}) {year}", "people", "{title} {year}"},
		{"{title} {part?}", "part", "{title}"},
		{"{title}-{part?}", "part", "{title}"},
		{"{title}{part?} x", "part", "{title} x"},
		{"[{studio?}] ({people?}) {title}", "studio people", "{title}"},
		{"{title} {part?}", "", "{title} {part}"},
	} {
		dropped := map[string]bool{}
		for _, d := range strings.Fields(c.drop) {
			dropped[d] = true
		}
		if got := dropOptional(c.text, dropped); got != c.want {
			t.Errorf("dropOptional(%q, %q) = %q, want %q", c.text, c.drop, got, c.want)
		}
	}
}

// N21: optional-token refusals.
func TestParseTemplateOptionalRefuses(t *testing.T) {
	for _, text := range []string{
		"{title?} {studio}",
		"{title?} {provider-1-id?}",
		"{part} {part?}",
		"{title} {studio?} {people?} {year?} {part?} {edition?} {provider-1-id?} {provider-2-id?}",
	} {
		if _, err := ParseTemplate(text, []string{"provider-1", "provider-2"}); err == nil {
			t.Errorf("ParseTemplate(%q) accepted", text)
		}
	}
	if _, err := ParseTemplate("{title?} {provider-1-id}", providers); err != nil {
		t.Errorf("required identity: %v", err)
	}
}

// N2: save-time refusals, each with its own message.
func TestParseTemplateRefuses(t *testing.T) {
	for _, text := range []string{
		"{studio} {year}",         // no title, no identity
		"{title} {title}",         // repeated
		"{title} {resolution}",    // unknown
		"{title} {provider-9-id}", // unconfigured provider
		"{title}: {studio}",
		"{title}/{studio}",
		`{title}\{studio}`,
		"{title}\x01",
		"{title} }",
		"",
	} {
		if _, err := ParseTemplate(text, providers); err == nil {
			t.Errorf("ParseTemplate(%q) accepted", text)
		}
	}
	if _, err := ParseTemplate("[{studio}] {title}", providers); err != nil {
		t.Errorf("[{studio}] {title}: %v", err)
	}
	if _, err := ParseTemplate("{provider-1-id}", providers); err != nil {
		t.Errorf("identity alone: %v", err)
	}
	_, errs := ParseList([]string{"{title}", "{title} {title}"}, providers)
	if errs[0] != nil || errs[1] == nil {
		t.Errorf("per-row errors = %v", errs)
	}
}

// N2 (builder half): a template that bypassed saving still can't produce a bad name.
func TestBuildChecksWholeName(t *testing.T) {
	p, _ := ParseTemplate("{title}", nil)
	tmpl := &Template{Text: "{title}: x", ids: p.ids,
		variants: []*variant{{text: "{title}: x", pattern: p.variants[0].pattern}}}
	if _, err := Build([]*Template{tmpl}, Values{"title": {"Ada"}}, "", ".mkv"); !errors.Is(err, ErrBadChar) {
		t.Fatalf("err = %v, want ErrBadChar", err)
	}
}

// N4: read-back refusal does not fall through.
func TestBuildReadBackRefuses(t *testing.T) {
	list := mustList(t, "{title} - {studio}", "{title}")
	_, err := Build(list, Values{"title": {"Ada - Redux"}, "studio": {"Acme"}}, "", ".mkv")
	var rb *ReadBackError
	if !errors.As(err, &rb) {
		t.Fatalf("err = %v, want ReadBackError", err)
	}
	if !reflect.DeepEqual(rb.Parsed, map[string][]string{"title": {"Ada"}, "studio": {"Redux - Acme"}}) {
		t.Errorf("parsed = %v", rb.Parsed)
	}
	// A value is refused when it holds the literal text after its token: here "] ". A bare "]" is
	// not, because the lazy capture still ends at the right "] " (spec P0-3's rule; its `]` example
	// is loosely worded).
	list = mustList(t, "[{studio}] {title}", "{title}")
	if _, err := Build(list, Values{"title": {"Ada"}, "studio": {"Acme] X"}}, "", ".mkv"); !errors.As(err, &rb) {
		t.Errorf("studio `Acme] X`: err = %v, want ReadBackError", err)
	}
	if _, err := Build(list, Values{"title": {"Ada"}, "studio": {"Acme]"}}, "", ".mkv"); err != nil {
		t.Errorf("studio `Acme]` reads back exactly: err = %v", err)
	}
}

// N5: an earlier template that reads a later template's name differently refuses it.
func TestBuildEarlierTemplateRefuses(t *testing.T) {
	list := mustList(t, "{title} - {studio}", "{title}")
	// No studio, so template 2 names it; template 1 would read "Ada - Redux" as title Ada.
	_, err := Build(list, Values{"title": {"Ada - Redux"}}, "", ".mkv")
	var rb *ReadBackError
	if !errors.As(err, &rb) {
		t.Fatalf("err = %v, want ReadBackError", err)
	}
}

// N3 is the caller's: an unconfirmed id never reaches Values. Here, a missing id doesn't fit.
func TestBuildIdentityNeedsValue(t *testing.T) {
	list := mustList(t, "{title} {provider-1-id}")
	if _, err := Build(list, Values{"title": {"Ada"}}, "", ".mkv"); !errors.Is(err, ErrNoTemplate) {
		t.Fatalf("err = %v", err)
	}
}

// N7: markers render and read back; brace text inside a value can't forge one.
func TestBuildMarkers(t *testing.T) {
	list := mustList(t, "{title} {edition} {part} {provider-1-id}", "{title}")
	v := Values{"title": {"Ada {edition-x}"}, "edition": {"Final Cut"}, "part": {"2"}, "provider-1-id": {"tt42"}}
	n, err := Build(list, v, "", ".mkv")
	if err != nil || n.Name != "Ada edition-x {edition-Final Cut} {part-2} {provider-1-tt42}.mkv" {
		t.Fatalf("got %q, %v", n.Name, err)
	}
	want := map[string][]string{"title": {"Ada edition-x"}, "edition": {"Final Cut"}, "part": {"2"}, "provider-1-id": {"tt42"}}
	if !reflect.DeepEqual(n.Written, want) {
		t.Errorf("written = %v", n.Written)
	}
}

// N16: the extension never changes.
func TestBuildKeepsExtension(t *testing.T) {
	n, err := Build(mustList(t, "{title}"), Values{"title": {"Ada"}}, "", ".MKV")
	if err != nil || n.Name != "Ada.MKV" {
		t.Fatalf("got %q, %v", n.Name, err)
	}
}

// N11: bytes, not characters.
func TestBuildTooLong(t *testing.T) {
	_, err := Build(mustList(t, "{title}"), Values{"title": {strings.Repeat("é", 126)}}, "", ".mkv")
	if !errors.Is(err, ErrTooLong) {
		t.Fatalf("err = %v, want ErrTooLong", err)
	}
}

// N10 through the builder: a value that leaves a reserved name is refused.
func TestBuildReserved(t *testing.T) {
	_, err := Build(mustList(t, "{title}"), Values{"title": {"CON"}}, "", ".mkv")
	if !errors.Is(err, ErrReserved) {
		t.Fatalf("err = %v, want ErrReserved", err)
	}
}

// N17 (stripping half): the disambiguator is removed before parsing and never becomes a value.
func TestReadBackStripsDisambiguator(t *testing.T) {
	v, _, ok := ReadBack(mustList(t, "{title}"), "Ada Lovelace {hx-412}", "")
	if !ok || !reflect.DeepEqual(v, map[string][]string{"title": {"Ada Lovelace"}}) {
		t.Fatalf("got %v, %v", v, ok)
	}
}

// N19 (bound half): an identity marker outside the id alphabet or too long is no hint.
func TestReadBackIdentityBound(t *testing.T) {
	list := mustList(t, "{title} {provider-1-id}", "{title}")
	for _, stem := range []string{"Ada {provider-1-../../x}", "Ada {provider-1-" + strings.Repeat("a", 65) + "}"} {
		v, _, _ := ReadBack(list, stem, "")
		if _, ok := v["provider-1-id"]; ok {
			t.Errorf("%q yielded a hint: %v", stem, v)
		}
	}
}

// hostile is the §39.1 fixture set, plus bare characters for the property test.
var hostile = []string{"Ada - Redux", "Acme]", "../etc", "CON", "con.txt", "Ada.", "Ada ", "a{b}c", "a:b*c?",
	"a\t\n b", "Café", strings.Repeat("x", 254)}

const hostileAlphabet = "aZ0 .-_[](),:*?\"<>|/\\{}\t\né́​‮é"

// N6 + N12: for every value that survives, render → read back gives the sanitised values, and
// every produced name stays in its folder.
func TestRoundTripProperty(t *testing.T) {
	list := mustList(t, "[{studio}] {title} ({people})", "{title}")
	check := func(title, studio string) bool {
		v := Values{"title": {title}, "studio": {studio}, "people": {studio, title}}
		n, err := Build(list, v, ", ", ".mkv")
		if err != nil {
			return true // refused, never wrong
		}
		dir := filepath.Join("lib", "x")
		return filepath.IsLocal(n.Name) && filepath.Base(n.Name) == n.Name &&
			filepath.Dir(filepath.Join(dir, n.Name)) == dir && CheckName(n.Name) == nil
	}
	for _, a := range hostile {
		for _, b := range hostile {
			if !check(a, b) {
				t.Errorf("fixture (%q, %q) produced a bad name", a, b)
			}
		}
	}
	gen := func(r *rand.Rand) string {
		rs := []rune(hostileAlphabet)
		n := r.Intn(12)
		var b strings.Builder
		for range n {
			b.WriteRune(rs[r.Intn(len(rs))])
		}
		return b.String()
	}
	cfg := &quick.Config{MaxCount: 2000, Values: func(args []reflect.Value, r *rand.Rand) {
		args[0], args[1] = reflect.ValueOf(gen(r)), reflect.ValueOf(gen(r))
	}}
	if err := quick.Check(check, cfg); err != nil {
		t.Error(err)
	}
	// Exactness is Build's own read-back comparison; prove a plain value round-trips.
	n, err := Build(list, Values{"title": {"a:b*c?"}}, "", ".mkv")
	if err != nil || !reflect.DeepEqual(n.Written["title"], []string{"abc"}) {
		t.Errorf("got %v, %v", n.Written, err)
	}
}
