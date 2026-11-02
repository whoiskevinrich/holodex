package query

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"
)

// sexp renders a tree as an S-expression, positions left out, so tests compare shape.
func sexp(n Node) string {
	switch t := n.(type) {
	case *Or:
		return "(or " + sexps(t.Items) + ")"
	case *And:
		return "(and " + sexps(t.Items) + ")"
	case *Not:
		return "(not " + sexp(t.X) + ")"
	case *Word:
		return fmt.Sprintf("%q", t.Text)
	case *Cond:
		if t.ID > 0 {
			return fmt.Sprintf("%s#%d", t.Field, t.ID)
		}
		return t.Field + ":" + condValue(t)
	}
	return "?"
}

func sexps(ns []Node) string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = sexp(n)
	}
	return strings.Join(parts, " ")
}

func mustParse(t *testing.T, s string) *Query {
	t.Helper()
	q, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return q
}

func asErr(t *testing.T, err error) *Error {
	t.Helper()
	var e *Error
	if !errors.As(err, &e) {
		t.Fatalf("error %v is not *Error", err)
	}
	return e
}

// Q1: precedence — NOT, then AND (adjacency), then OR; and the two readings of
// motivating query 1.
func TestParsePrecedence(t *testing.T) {
	cases := map[string]string{
		"a b OR c":          `(or (and "a" "b") "c")`,
		"-a b":              `(and (not "a") "b")`,
		"-(a OR b)":         `(not (or "a" "b"))`,
		"NOT a b":           `(and (not "a") "b")`,
		"a AND b OR c":      `(or (and "a" "b") "c")`,
		"a OR b OR c":       `(or "a" "b" "c")`,
		"(a b) c":           `(and "a" "b" "c")`,
		"tag:a tag:b":       `(and tag:a tag:b)`,
		`"science fiction"`: `"science fiction"`,
		"or not":            `(and "or" "not")`,
		"((tag:science-fiction tag:fantasy) OR tag:action) -tag:comedy person:\"Harrison Ford\"": `(and (or (and tag:science-fiction tag:fantasy) tag:action) (not tag:comedy) person:"Harrison Ford")`,
		"(tag:science-fiction tag:fantasy) OR tag:action -tag:comedy person:\"Harrison Ford\"":   `(or (and tag:science-fiction tag:fantasy) (and tag:action (not tag:comedy) person:"Harrison Ford"))`,
		"studio:Paramount -matched:any": `(and studio:Paramount (not matched:any))`,
		"tag:comedy duration:<2m":       `(and tag:comedy duration:<120s)`,
		"duration:20m..1h year:>=1990":  `(and duration:1200s..3600s year:>=1990)`,
		"resolution:fhd":                `resolution:FHD`,
		"tag:12":                        `tag#12`,
		`tag:"12"`:                      `tag:"12"`,
	}
	for in, want := range cases {
		if got := sexp(mustParse(t, in).Root); got != want {
			t.Errorf("Parse(%q) = %s, want %s", in, got, want)
		}
	}
}

var corpus = []string{
	"((tag:science-fiction tag:fantasy) OR tag:action) -tag:comedy person:\"Harrison Ford\"",
	"studio:Paramount -matched:any",
	"tag:comedy duration:<2m",
	"a b OR c", "-a b", "-(a OR b)", "-(a b)", "--a", `"or" "AND" "-x" "a:b"`,
	"tag:12 person:3 studio:4 category:5 resolution:4K year:2001 year:1990..1999 duration:>=30s",
	"matched:provider-1 OR -matched:any", `tag:"2001"`, "(a OR (b c)) d", "é ü 中文",
}

// Q2: parse → print → parse gives an equal tree, and printing is idempotent.
func TestRoundTrip(t *testing.T) {
	for _, in := range corpus {
		q := mustParse(t, in)
		once := q.String()
		q2 := mustParse(t, once)
		if sexp(q.Root) != sexp(q2.Root) {
			t.Errorf("%q: reparse of %q gave %s, want %s", in, once, sexp(q2.Root), sexp(q.Root))
		}
		if twice := q2.String(); twice != once {
			t.Errorf("%q: printed %q then %q", in, once, twice)
		}
	}
}

// Q3: never panics; anything that parses round-trips.
func FuzzParse(f *testing.F) {
	for _, s := range corpus {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		q, err := Parse(in)
		if err != nil {
			return
		}
		once := q.String()
		q2, err := Parse(once)
		if err != nil {
			t.Fatalf("%q printed as %q, which doesn't parse: %v", in, once, err)
		}
		if sexp(q.Root) != sexp(q2.Root) || q2.String() != once {
			t.Fatalf("%q: round trip %q → %s, want %s", in, once, sexp(q2.Root), sexp(q.Root))
		}
	})
}

// panicResolver fails the test if a refused input ever reaches a name lookup (Q4).
type panicResolver struct{ t *testing.T }

func (p panicResolver) Lookup(context.Context, string, string) ([]Candidate, error) {
	p.t.Fatal("resolver called on a refused input")
	return nil, nil
}
func (p panicResolver) NameOf(context.Context, string, int64) (string, bool, error) {
	p.t.Fatal("resolver called on a refused input")
	return "", false, nil
}
func (p panicResolver) Suggest(context.Context, string, string, int) ([]string, error) {
	p.t.Fatal("resolver called on a refused input")
	return nil, nil
}
func (p panicResolver) Providers(context.Context) ([]string, error) {
	p.t.Fatal("resolver called on a refused input")
	return nil, nil
}

func nested(depth int) string {
	return strings.Repeat("(", depth) + "a" + strings.Repeat(")", depth)
}

func words(n int) string {
	return strings.TrimSpace(strings.Repeat("a ", n))
}

// Q4: each cap refuses one past its value, naming it, before any lookup; the cap parses.
func TestCaps(t *testing.T) {
	refused := map[string]string{
		strings.Repeat("a", MaxBytes+1):       CodeTooLong,
		nested(MaxDepth + 1):                  CodeTooDeep,
		strings.Repeat("-", MaxDepth+1) + "a": CodeTooDeep,
		words(MaxConditions + 1):              CodeTooMany,
	}
	for in, code := range refused {
		q, err := Parse(in)
		if err == nil {
			_ = q.Canonicalize(context.Background(), panicResolver{t})
			t.Errorf("%d-byte input parsed; want %s", len(in), code)
			continue
		}
		if e := asErr(t, err); e.Code != code {
			t.Errorf("%d-byte input: code %s, want %s", len(in), e.Code, code)
		}
	}
	for _, in := range []string{strings.Repeat("a", MaxBytes), nested(MaxDepth), words(MaxConditions)} {
		if _, err := Parse(in); err != nil {
			t.Errorf("%d-byte input at the cap refused: %v", len(in), err)
		}
	}
}

// Q5: errors carry the bad token's offsets, in UTF-16 code units.
func TestErrorOffsets(t *testing.T) {
	cases := []struct {
		in, code, bad string
	}{
		{"tag:a colour:red", CodeUnknownField, "colour"},
		{"(tag:a OR tag:b", CodeSyntax, "("},
		{"tag:a )", CodeSyntax, ")"},
		{"duration:5m..", CodeBadRange, "5m.."},
		{"é \"Ab\" year:abc", CodeBadValue, "abc"},
		{"中文 tag:", CodeBadValue, ""},
	}
	for _, c := range cases {
		_, err := Parse(c.in)
		if err == nil {
			t.Errorf("Parse(%q) succeeded", c.in)
			continue
		}
		e := asErr(t, err)
		if e.Code != c.code {
			t.Errorf("Parse(%q) code %s, want %s", c.in, e.Code, c.code)
		}
		u := utf16.Encode([]rune(c.in))
		if got := string(utf16.Decode(u[e.Start:e.End])); got != c.bad {
			t.Errorf("Parse(%q) marks %q, want %q", c.in, got, c.bad)
		}
	}
}

type stubResolver struct {
	names     map[string]map[string][]Candidate // kind → name key → matches
	byID      map[string]map[int64]string
	providers []string
}

func key(s string) string { return strings.Join(strings.Fields(strings.ToLower(s)), " ") }

func (s stubResolver) Lookup(_ context.Context, kind, name string) ([]Candidate, error) {
	return s.names[kind][key(name)], nil
}
func (s stubResolver) NameOf(_ context.Context, kind string, id int64) (string, bool, error) {
	n, ok := s.byID[kind][id]
	return n, ok, nil
}
func (s stubResolver) Suggest(_ context.Context, kind, name string, _ int) ([]string, error) {
	if kind == FieldTag && strings.HasPrefix(key(name), "science") {
		return []string{"science-fiction"}, nil
	}
	return nil, nil
}
func (s stubResolver) Providers(context.Context) ([]string, error) { return s.providers, nil }

func newStub() stubResolver {
	sf := Candidate{ID: 12, Name: "science-fiction"}
	return stubResolver{
		names: map[string]map[string][]Candidate{
			FieldTag: {"science-fiction": {sf}, "sci-fi": {sf}, "comedy": {{ID: 3, Name: "comedy"}}, "2001": {{ID: 40, Name: "2001"}}},
			FieldPerson: {
				"harrison ford": {{ID: 7, Name: "Harrison Ford"}},
				"chris evans":   {{ID: 8, Name: "Chris Evans"}, {ID: 9, Name: "Chris Evans (II)"}},
			},
		},
		byID: map[string]map[int64]string{
			FieldTag:    {12: "science-fiction", 3: "comedy", 40: "2001"},
			FieldPerson: {7: "Harrison Ford"},
		},
		providers: []string{"provider-1"},
	}
}

// Q6: canonicalisation binds names, aliases and ids; reports unknown and ambiguous names.
func TestCanonicalize(t *testing.T) {
	ctx := context.Background()
	r := newStub()
	ok := map[string]string{
		"tag:sci-fi":               "tag:12",
		"tag:SCIENCE-FICTION":      "tag:12",
		`person:"harrison   ford"`: "person:7",
		"tag:12":                   "tag:12",
		`tag:"2001"`:               "tag:40",
		"tag:2001":                 "tag:40", // no tag with id 2001: the digits are a name
		"duration:<2m":             "duration:<120s",
		"comedy":                   "comedy",
		"MATCHED:Provider-1":       "matched:provider-1",
	}
	for in, want := range ok {
		q := mustParse(t, in)
		if err := q.Canonicalize(ctx, r); err != nil {
			t.Errorf("Canonicalize(%q): %v", in, err)
			continue
		}
		if got := q.String(); got != want {
			t.Errorf("Canonicalize(%q) = %q, want %q", in, got, want)
		}
	}

	q := mustParse(t, "tag:sciencefiction OR tag:action")
	e := asErr(t, q.Canonicalize(ctx, r))
	if e.Code != CodeUnknownName || e.Field != FieldTag || len(e.Suggestions) == 0 || e.Suggestions[0] != "science-fiction" {
		t.Errorf("unknown tag error = %+v", e)
	}
	if e.Start != 4 || e.End != 18 {
		t.Errorf("unknown tag marks [%d,%d), want [4,18)", e.Start, e.End)
	}

	q = mustParse(t, `person:"Chris Evans"`)
	e = asErr(t, q.Canonicalize(ctx, r))
	if e.Code != CodeAmbiguous || len(e.Candidates) != 2 {
		t.Errorf("ambiguous person error = %+v", e)
	}

	q = mustParse(t, "matched:provider-9")
	if e = asErr(t, q.Canonicalize(ctx, r)); e.Code != CodeUnknownName {
		t.Errorf("unknown provider error = %+v", e)
	}
}

// Q7: the owner-only check finds `matched:` anywhere and passes everything else.
func TestCheckOwner(t *testing.T) {
	for _, in := range []string{"matched:any", "matched:provider-1", "-matched:any", "tag:a OR (b -matched:any)"} {
		q := mustParse(t, in)
		if err := q.CheckOwner(false); err == nil || asErr(t, err).Code != CodeOwnerOnly {
			t.Errorf("CheckOwner(false) on %q = %v, want owner_only", in, err)
		}
		if err := q.CheckOwner(true); err != nil {
			t.Errorf("CheckOwner(true) on %q = %v", in, err)
		}
	}
	q := mustParse(t, "tag:a OR -b duration:<2m")
	if q.CheckOwner(false) != nil || q.CheckOwner(true) != nil {
		t.Error("a query without matched: was refused")
	}
}

func TestDisplayUsesCurrentNames(t *testing.T) {
	ctx := context.Background()
	r := newStub()
	q := mustParse(t, "(tag:sci-fi tag:comedy) OR -person:7")
	if err := q.Canonicalize(ctx, r); err != nil {
		t.Fatal(err)
	}
	r.byID[FieldTag][12] = "sf" // renamed
	d, err := q.Display(ctx, r)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kind != "or" || d.Expr != "tag:12 tag:3 OR -person:7" {
		t.Fatalf("display root = %+v", d)
	}
	sf := d.Items[0].Items[0]
	if sf.Label != "sf" || sf.ID != 12 || sf.Expr != "tag:12" {
		t.Errorf("renamed tag displays as %+v", sf)
	}
	if not := d.Items[1]; not.Kind != "not" || not.Items[0].Label != "Harrison Ford" {
		t.Errorf("not node = %+v", not)
	}
}

func TestRewriteAndEntityIDs(t *testing.T) {
	q := mustParse(t, "person:5 OR (tag:2 -person:5)")
	if !RewriteID(q.Root, FieldPerson, 5, 9) {
		t.Fatal("RewriteID changed nothing")
	}
	if got := q.String(); got != "person:9 OR tag:2 -person:9" {
		t.Errorf("rewritten = %q", got)
	}
	ids := EntityIDs(q.Root)
	if len(ids[FieldPerson]) != 1 || ids[FieldPerson][0] != 9 || ids[FieldTag][0] != 2 {
		t.Errorf("EntityIDs = %v", ids)
	}
}
