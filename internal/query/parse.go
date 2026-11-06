package query

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"holodex/internal/metadata"
)

type tokKind int

const (
	tokEOF tokKind = iota
	tokLParen
	tokRParen
	tokMinus // a `-` glued to the term or group after it
	tokOr
	tokAnd
	tokNot
	tokTerm // a word, a quoted phrase, or field:value
)

type token struct {
	kind     tokKind
	text     string // the term's source text
	pos, end int
}

// Parse parses src into a Query. The caps are checked first and during the parse, so
// an input past one is refused before any lookup (security-perimeter.md). Errors are
// *Error with source offsets. Entity names are not resolved here — see Canonicalize.
//
// Parse knows the built-in fields only. ParseWith adds the mapped ones.
//
// Grammar, loosest first: or := and (OR and)* ; and := unary ([AND] unary)* ;
// unary := (- | NOT) unary | primary ; primary := ( or ) | term. Adjacency is AND.
// OR, AND and NOT are keywords only in upper case; a lowercase `or` is a word.
func Parse(src string) (*Query, error) { return parse(src, nil, false) }

// ParseWith is Parse with the owner's queryable mapped fields (F89 P0-6).
func ParseWith(src string, fields Fields) (*Query, error) { return parse(src, fields, false) }

// ParseStored reads a definition already saved. A condition on a field the catalogue
// doesn't have is kept as KindGone rather than refused, so a field later removed from the
// mapping is a stale ref naming it, and a merge can rewrite the definition around it.
func ParseStored(src string, fields Fields) (*Query, error) { return parse(src, fields, true) }

func parse(src string, fields Fields, stored bool) (*Query, error) {
	if len(src) > MaxBytes {
		return nil, newError(src, CodeTooLong, fmt.Sprintf("This query is too long to run (over %d characters). Try splitting it.", MaxBytes), MaxBytes, len(src))
	}
	if strings.TrimSpace(src) == "" {
		return nil, newError(src, CodeEmpty, "The query is empty.", 0, len(src))
	}
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{src: src, toks: toks, fields: fields, stored: stored}
	root, err := p.parseOr(0)
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != tokEOF {
		if t.kind == tokRParen {
			return nil, newError(src, CodeSyntax, "This query has a bracket that was never opened.", t.pos, t.end)
		}
		return nil, newError(src, CodeSyntax, "Unexpected "+t.text+".", t.pos, t.end)
	}
	return &Query{Root: root, src: src}, nil
}

func lex(src string) ([]token, error) {
	var toks []token
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			toks = append(toks, token{kind: tokLParen, text: "(", pos: i, end: i + 1})
			i++
		case c == ')':
			toks = append(toks, token{kind: tokRParen, text: ")", pos: i, end: i + 1})
			i++
		case c == '-' && i+1 < len(src) && !isSpace(src[i+1]) && src[i+1] != ')':
			toks = append(toks, token{kind: tokMinus, text: "-", pos: i, end: i + 1})
			i++
		default:
			start := i
			for i < len(src) && !isSpace(src[i]) && src[i] != '(' && src[i] != ')' {
				if src[i] == '"' {
					close := strings.IndexByte(src[i+1:], '"')
					if close < 0 {
						return nil, newError(src, CodeSyntax, "This quote is never closed.", i, len(src))
					}
					i += close + 2
					continue
				}
				i++
			}
			text := src[start:i]
			kind := tokTerm
			switch text {
			case "OR":
				kind = tokOr
			case "AND":
				kind = tokAnd
			case "NOT":
				kind = tokNot
			}
			toks = append(toks, token{kind: kind, text: text, pos: start, end: i})
		}
	}
	return append(toks, token{kind: tokEOF, pos: len(src), end: len(src)}), nil
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

type parser struct {
	src    string
	toks   []token
	i      int
	conds  int
	fields Fields
	stored bool
}

func (p *parser) peek() token { return p.toks[p.i] }
func (p *parser) next() token { t := p.toks[p.i]; p.i++; return t }

func (p *parser) parseOr(depth int) (Node, error) {
	first, err := p.parseAnd(depth)
	if err != nil {
		return nil, err
	}
	items := []Node{first}
	for p.peek().kind == tokOr {
		p.next()
		n, err := p.parseAnd(depth)
		if err != nil {
			return nil, err
		}
		items = append(items, n)
	}
	return join(items, false), nil
}

func (p *parser) parseAnd(depth int) (Node, error) {
	var items []Node
	for {
		t := p.peek()
		switch t.kind {
		case tokEOF, tokRParen, tokOr:
			if len(items) == 0 {
				return nil, p.missingOperand(t)
			}
			return join(items, true), nil
		case tokAnd:
			if len(items) == 0 {
				return nil, newError(p.src, CodeSyntax, "AND needs something before it.", t.pos, t.end)
			}
			p.next()
			if k := p.peek().kind; k == tokEOF || k == tokRParen || k == tokOr {
				return nil, p.missingOperand(p.peek())
			}
			continue
		}
		n, err := p.parseUnary(depth)
		if err != nil {
			return nil, err
		}
		items = append(items, n)
	}
}

func (p *parser) missingOperand(t token) error {
	switch t.kind {
	case tokOr:
		return newError(p.src, CodeSyntax, "OR needs something on both sides.", t.pos, t.end)
	case tokRParen:
		return newError(p.src, CodeSyntax, "These parentheses are empty.", t.pos, t.end)
	}
	if p.i > 0 {
		prev := p.toks[p.i-1]
		return newError(p.src, CodeSyntax, prev.text+" needs something after it.", prev.pos, prev.end)
	}
	return newError(p.src, CodeEmpty, "The query is empty.", 0, len(p.src))
}

func (p *parser) parseUnary(depth int) (Node, error) {
	t := p.peek()
	switch t.kind {
	case tokMinus, tokNot:
		if depth+1 > MaxDepth {
			return nil, p.tooDeep(t)
		}
		p.next()
		if k := p.peek().kind; k == tokEOF || k == tokRParen || k == tokOr || k == tokAnd {
			return nil, newError(p.src, CodeSyntax, t.text+" needs something after it.", t.pos, t.end)
		}
		x, err := p.parseUnary(depth + 1)
		if err != nil {
			return nil, err
		}
		return &Not{X: x}, nil
	case tokLParen:
		if depth+1 > MaxDepth {
			return nil, p.tooDeep(t)
		}
		p.next()
		n, err := p.parseOr(depth + 1)
		if err != nil {
			return nil, err
		}
		if p.peek().kind != tokRParen {
			return nil, newError(p.src, CodeSyntax, "This query has an unclosed bracket.", t.pos, t.end)
		}
		p.next()
		return n, nil
	case tokTerm:
		p.next()
		p.conds++
		if p.conds > MaxConditions {
			return nil, newError(p.src, CodeTooMany, fmt.Sprintf("This query is too long to run (over %d conditions). Try splitting it.", MaxConditions), t.pos, len(p.src))
		}
		return p.term(t)
	}
	return nil, p.missingOperand(t)
}

func (p *parser) tooDeep(t token) error {
	return newError(p.src, CodeTooDeep, fmt.Sprintf("This query is too long to run (nested over %d levels). Try splitting it.", MaxDepth), t.pos, t.end)
}

// term turns one term token into a *Word or a *Cond.
func (p *parser) term(t token) (Node, error) {
	field, value, isCond := splitField(t.text)
	if !isCond {
		text := t.text
		if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
			text = text[1 : len(text)-1]
		}
		text = strings.TrimSpace(strings.ReplaceAll(text, `"`, ""))
		if text == "" {
			return nil, newError(p.src, CodeSyntax, "This quote is empty.", t.pos, t.end)
		}
		return &Word{Text: text, Pos: t.pos, End: t.end}, nil
	}
	spec, mapped := p.fields[field]
	if !knownFields[field] && !mapped && !p.stored {
		e := newError(p.src, CodeUnknownField, fmt.Sprintf("%q isn't something you can search by.", field), t.pos, t.pos+len(field))
		e.Field = field
		return nil, e
	}
	valPos := t.pos + len(field) + 1
	c := &Cond{Field: field, Op: OpEq, Pos: t.pos, End: t.end}
	switch {
	case field == FieldReleased:
		c.Owner = spec.OwnerOnly
	case knownFields[field]:
	case mapped:
		c.Kind, c.FieldLabel, c.Owner = spec.Kind, spec.Label, spec.OwnerOnly
	default:
		c.Kind = KindGone
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		value, c.Quoted = value[1:len(value)-1], true
	}
	value = strings.TrimSpace(strings.ReplaceAll(value, `"`, ""))
	bad := func(code, msg string) error {
		e := newError(p.src, code, msg, valPos, t.end)
		e.Field = field
		return e
	}
	if value == "" {
		return nil, bad(CodeBadValue, field+": needs a value.")
	}
	switch {
	case c.Kind == KindText, c.Kind == KindList, c.Kind == KindGone:
		c.Value = value
	case field == FieldYear, field == FieldDuration, c.Kind == KindNumber:
		if c.Quoted {
			return nil, bad(CodeBadValue, field+": takes a number, not a quoted name.")
		}
		num := parseYear
		if field == FieldDuration {
			num = parseDuration
		} else if c.Kind == KindNumber {
			num = parseNumber
		}
		if err := parseComparison(c, value, num); err != "" {
			code := CodeBadValue
			if strings.Contains(value, "..") {
				code = CodeBadRange
			}
			return nil, bad(code, field+": "+err)
		}
	case dateFields[field], c.Kind == KindDate:
		if c.Quoted {
			return nil, bad(CodeBadValue, field+": takes a date, not a quoted name.")
		}
		if err := parseDateCond(c, value); err != "" {
			code := CodeBadValue
			if strings.Contains(value, "..") {
				code = CodeBadRange
			}
			return nil, bad(code, field+": "+err)
		}
	case field == FieldResolution:
		b, ok := metadata.ParseResolutionBucket(value)
		if !ok {
			return nil, bad(CodeBadValue, "resolution: is one of SD, HD, FHD or 4K.")
		}
		c.Value = string(b)
	case field == FieldMatched:
		c.Value = value
		if strings.EqualFold(value, MatchedAny) {
			c.Value = MatchedAny
		}
	default: // entity fields: an unquoted number is an id, anything else a name
		c.Value = value
		if !c.Quoted {
			if id, err := strconv.ParseInt(value, 10, 64); err == nil && id > 0 {
				c.ID = id
			}
		}
	}
	return c, nil
}

// splitField splits `field:value` when the text before the first colon is a field-like
// identifier (letters and underscores); anything else is a bare word.
func splitField(s string) (field, value string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i <= 0 {
		return "", "", false
	}
	for _, r := range s[:i] {
		if !(unicode.IsLetter(r) || r == '_') {
			return "", "", false
		}
	}
	return strings.ToLower(s[:i]), s[i+1:], true
}

// parseComparison fills c.Op / c.Lo / c.Hi from `<n`, `<=n`, `>n`, `>=n`, `a..b` or `n`,
// with num parsing one bound. It returns a message on failure.
func parseComparison(c *Cond, v string, num func(string) (int64, bool)) string {
	if lo, hi, ok := strings.Cut(v, ".."); ok {
		a, okA := num(lo)
		b, okB := num(hi)
		switch {
		case lo == "" || hi == "":
			return "use a range like duration:20m..1h."
		case !okA || !okB:
			return "can't read this range."
		case a > b:
			return "the range's start is after its end."
		}
		c.Op, c.Lo, c.Hi = OpRange, a, b
		return ""
	}
	for _, op := range []string{OpLe, OpGe, OpLt, OpGt} {
		if strings.HasPrefix(v, op) {
			c.Op, v = op, v[len(op):]
			break
		}
	}
	n, ok := num(v)
	if !ok {
		return "can't read " + strconv.Quote(v) + "."
	}
	c.Lo = n
	return ""
}

// parseYear reads a whole year.
func parseYear(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n >= 1 && n <= 9999
}

// parseNumber reads a mapped number field's whole number.
func parseNumber(s string) (int64, bool) {
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && n >= 0 && n <= 1_000_000_000_000
}

// parseDuration reads a whole number of seconds (s), minutes (m, or no unit, as the
// browse duration keys count) or hours (h) as seconds.
func parseDuration(s string) (int64, bool) {
	mult := int64(60)
	switch {
	case strings.HasSuffix(s, "s"):
		mult, s = 1, s[:len(s)-1]
	case strings.HasSuffix(s, "m"):
		s = s[:len(s)-1]
	case strings.HasSuffix(s, "h"):
		mult, s = 3600, s[:len(s)-1]
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 || n > 1_000_000 {
		return 0, false
	}
	return n * mult, true
}

// join builds an And (and=true) or Or over items, flattening a nested node of the same
// kind and collapsing a single item to itself, so equal queries build equal trees.
func join(items []Node, and bool) Node {
	var flat []Node
	for _, it := range items {
		switch t := it.(type) {
		case *And:
			if and {
				flat = append(flat, t.Items...)
				continue
			}
		case *Or:
			if !and {
				flat = append(flat, t.Items...)
				continue
			}
		}
		flat = append(flat, it)
	}
	if len(flat) == 1 {
		return flat[0]
	}
	if and {
		return &And{Items: flat}
	}
	return &Or{Items: flat}
}
