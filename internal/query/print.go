package query

import (
	"strconv"
	"strings"
)

// String prints the query in canonical form.
func (q *Query) String() string { return Print(q.Root) }

// Print writes n in canonical form: entity ids once canonicalised (`tag:12`), durations
// in seconds, years as numbers, bare words as text, one space between AND items,
// ` OR ` between OR items, and parentheses only where precedence needs them. Printing
// is deterministic, so a canonical query reprints to the same bytes.
func Print(n Node) string {
	var b strings.Builder
	printNode(&b, n, false)
	return b.String()
}

// printNode writes n; group says n sits where an OR, or any composite under a NOT, must
// be parenthesised.
func printNode(b *strings.Builder, n Node, group bool) {
	switch t := n.(type) {
	case *Or:
		if group {
			b.WriteByte('(')
		}
		for i, it := range t.Items {
			if i > 0 {
				b.WriteString(" OR ")
			}
			printNode(b, it, false)
		}
		if group {
			b.WriteByte(')')
		}
	case *And:
		if group {
			b.WriteByte('(')
		}
		for i, it := range t.Items {
			if i > 0 {
				b.WriteByte(' ')
			}
			_, isOr := it.(*Or)
			printNode(b, it, isOr)
		}
		if group {
			b.WriteByte(')')
		}
	case *Not:
		b.WriteByte('-')
		_, isOr := t.X.(*Or)
		_, isAnd := t.X.(*And)
		printNode(b, t.X, isOr || isAnd)
	case *Cond:
		b.WriteString(t.Field)
		b.WriteByte(':')
		b.WriteString(condValue(t))
	case *Word:
		b.WriteString(quoteIfNeeded(t.Text, false))
	}
}

func condValue(c *Cond) string {
	switch c.Field {
	case FieldYear:
		return comparison(c, "")
	case FieldDuration:
		return comparison(c, "s")
	case FieldResolution, FieldAdded:
		return c.Value
	case FieldMatched:
		return quoteIfNeeded(c.Value, false)
	}
	if c.ID > 0 {
		return strconv.FormatInt(c.ID, 10)
	}
	return quoteIfNeeded(c.Value, true)
}

func comparison(c *Cond, unit string) string {
	lo := strconv.FormatInt(c.Lo, 10) + unit
	switch c.Op {
	case OpRange:
		return lo + ".." + strconv.FormatInt(c.Hi, 10) + unit
	case OpEq:
		return lo
	}
	return c.Op + lo
}

// quoteIfNeeded quotes s when it would not reparse as the same single term: spaces,
// parentheses, a colon, a leading `-`, a keyword, or (for an entity name) all digits,
// which would read as an id.
func quoteIfNeeded(s string, entityName bool) string {
	need := s == "" || strings.ContainsAny(s, " \t\r\n():") || strings.HasPrefix(s, "-") ||
		s == "OR" || s == "AND" || s == "NOT"
	if !need && entityName {
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			need = true
		}
	}
	if need {
		return `"` + s + `"`
	}
	return s
}
