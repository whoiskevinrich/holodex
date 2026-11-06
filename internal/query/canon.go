package query

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Resolver binds names to identity for Canonicalize and names to ids for Display. kind
// is an entity field name (tag, person, studio, category). The API implements it over
// the repository, through the alias-aware name lookup (entity-identity.md).
type Resolver interface {
	// Lookup returns every entity whose name or alias is name. More than one is
	// ambiguous; none is unknown.
	Lookup(ctx context.Context, kind, name string) ([]Candidate, error)
	// NameOf returns an entity's current name; ok is false when it no longer exists.
	NameOf(ctx context.Context, kind string, id int64) (name string, ok bool, err error)
	// Suggest returns up to n names close to an unknown name.
	Suggest(ctx context.Context, kind, name string, n int) ([]string, error)
	// Providers returns the provider names a `matched:` condition may name.
	Providers(ctx context.Context) ([]string, error)
}

const maxSuggestions = 5

var kindLabel = map[string]string{
	FieldTag: "Tag", FieldPerson: "person", FieldStudio: "studio", FieldCategory: "category",
}

// Canonicalize binds every entity condition to an id (an unquoted number names an id
// that must exist, falling back to a name of those digits), and checks `matched:`
// providers. It stops at the first name that is unknown or ambiguous, returning an
// *Error at its offset with suggestions or the candidates. Bare words stay text (RD1).
func (q *Query) Canonicalize(ctx context.Context, r Resolver) error {
	var providers []string
	for _, c := range Conds(q.Root) {
		switch {
		case c.Field == FieldMatched && c.Value != MatchedAny:
			if providers == nil {
				var err error
				if providers, err = r.Providers(ctx); err != nil {
					return err
				}
			}
			i := slices.IndexFunc(providers, func(p string) bool { return strings.EqualFold(p, c.Value) })
			if i < 0 {
				e := q.errAt(CodeUnknownName, fmt.Sprintf("No provider called %q.", c.Value), c.Pos+len(c.Field)+1, c.End)
				e.Field, e.Suggestions = c.Field, providers
				return e
			}
			c.Value = providers[i]
		case entityFields[c.Field]:
			if err := q.bind(ctx, r, c); err != nil {
				return err
			}
		}
	}
	return nil
}

func (q *Query) bind(ctx context.Context, r Resolver, c *Cond) error {
	if c.ID > 0 && !c.Quoted {
		_, ok, err := r.NameOf(ctx, c.Field, c.ID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		c.ID = 0 // no such id: try the digits as a name
	}
	found, err := r.Lookup(ctx, c.Field, c.Value)
	if err != nil {
		return err
	}
	valPos := c.Pos + len(c.Field) + 1
	switch len(found) {
	case 1:
		c.ID = found[0].ID
		return nil
	case 0:
		e := q.errAt(CodeUnknownName, fmt.Sprintf("No %s called %q.", kindLabel[c.Field], c.Value), valPos, c.End)
		e.Field = c.Field
		if e.Suggestions, err = r.Suggest(ctx, c.Field, c.Value, maxSuggestions); err != nil {
			return err
		}
		return e
	}
	e := q.errAt(CodeAmbiguous, fmt.Sprintf("More than one %s is called %q. Pick one.", kindLabel[c.Field], c.Value), valPos, c.End)
	e.Field, e.Candidates = c.Field, found
	return e
}

// EntityIDs returns the ids each entity field names, for the stale-ref check and the
// merge rewrite. Only meaningful on a canonical query.
func EntityIDs(n Node) map[string][]int64 {
	out := map[string][]int64{}
	for _, c := range Conds(n) {
		if entityFields[c.Field] && c.ID > 0 && !slices.Contains(out[c.Field], c.ID) {
			out[c.Field] = append(out[c.Field], c.ID)
		}
	}
	return out
}

// RewriteID points every condition on field naming from at to instead (a merge, ADR-121
// D5), reporting whether anything changed.
func RewriteID(n Node, field string, from, to int64) bool {
	changed := false
	for _, c := range Conds(n) {
		if c.Field == field && c.ID == from {
			c.ID, changed = to, true
		}
	}
	return changed
}

// DisplayNode is the parse endpoint's output tree: the shape chips and the Advanced
// query row render from, with current names. It is output only, never accepted as input.
// Expr is the node's own canonical text, so a client removing a chip rebuilds the query
// by joining its siblings' Expr without parsing.
type DisplayNode struct {
	Kind  string        `json:"kind"` // or | and | not | cond | word
	Expr  string        `json:"expr"`
	Items []DisplayNode `json:"items,omitempty"`
	Field string        `json:"field,omitempty"`
	Op    string        `json:"op,omitempty"`
	Value string        `json:"value,omitempty"`
	ID    int64         `json:"id,omitempty"`
	Lo    int64         `json:"lo,omitempty"`
	Hi    int64         `json:"hi,omitempty"`
	Label string        `json:"label,omitempty"` // the entity's current name, the provider, or the word
	// Missing: the entity no longer exists. Only a stored definition reaches this, since
	// canonicalising refuses an unknown id.
	Missing bool `json:"missing,omitempty"`
}

// Display builds the display tree of a canonical query, naming each entity by its
// current name, so a renamed Tag shows its new name while the canonical id is unchanged.
func (q *Query) Display(ctx context.Context, r Resolver) (DisplayNode, error) {
	return display(ctx, r, q.Root)
}

func display(ctx context.Context, r Resolver, n Node) (DisplayNode, error) {
	d := DisplayNode{Expr: Print(n)}
	var kids []Node
	switch t := n.(type) {
	case *Or:
		d.Kind, kids = "or", t.Items
	case *And:
		d.Kind, kids = "and", t.Items
	case *Not:
		d.Kind, kids = "not", []Node{t.X}
	case *Word:
		d.Kind, d.Label = "word", t.Text
	case *Cond:
		d.Kind, d.Field, d.Op, d.Value, d.ID, d.Lo, d.Hi = "cond", t.Field, t.Op, condValue(t), t.ID, t.Lo, t.Hi
		d.Label = t.Value
		if t.Field == FieldAdded {
			d.Label = DateLabel(t)
		}
		if entityFields[t.Field] && t.ID > 0 {
			name, ok, err := r.NameOf(ctx, t.Field, t.ID)
			if err != nil {
				return d, err
			}
			if ok {
				d.Label = name
			} else {
				d.Label, d.Missing = "#"+strconv.FormatInt(t.ID, 10), true
			}
		}
	}
	for _, k := range kids {
		kd, err := display(ctx, r, k)
		if err != nil {
			return d, err
		}
		d.Items = append(d.Items, kd)
	}
	return d, nil
}
