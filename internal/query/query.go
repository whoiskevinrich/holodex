// Package query is the F87 query language (spec query-language.md): one text grammar
// for AND / OR / NOT over a video's Tags, people, studios, browse facets and provider
// match. It is the only implementation of that grammar (browse-and-list-state.md,
// "Boolean queries"): a hand-written recursive-descent parser to a small tree, a
// canonical printer, canonicalisation that binds names to ids through a Resolver, and
// the one owner-only check every entry point calls (security-perimeter.md, "Query
// expressions"). It builds no SQL; internal/repo compiles a tree through the same
// clause fragments the flat browse keys use.
package query

// Caps, enforced in Parse before any name lookup or SQL (security-perimeter.md).
const (
	MaxBytes      = 1000
	MaxDepth      = 8
	MaxConditions = 64
)

// Fields a condition may name.
const (
	FieldTag        = "tag"
	FieldPerson     = "person"
	FieldStudio     = "studio"
	FieldCategory   = "category"
	FieldResolution = "resolution"
	FieldYear       = "year"
	FieldDuration   = "duration"
	FieldMatched    = "matched"
	FieldAdded      = "added"    // F89 P0-1: when the video arrived in the library
	FieldPlaylist   = "playlist" // F89 P0-3: in that playlist as it is now, cap included
	FieldUpdated    = "updated"  // F89 RD2: when anything about the video last changed
	FieldReleased   = "released" // F89 P0-1: the resolved release date
	FieldWatched    = "watched"  // F89 P0-8: yes or no, the owner's mark first
	FieldPlays      = "plays"    // F89 P0-8: how many counted plays
	FieldPlayed     = "played"   // F89 P0-8: when the last counted play happened
)

// The two values of `watched:`.
const (
	WatchedYes = "yes"
	WatchedNo  = "no"
)

// ReleaseDateKey is the mapped field `released:` reads.
const ReleaseDateKey = "release_date"

// A mapped field's kind (F89 P0-6), which decides how its value parses and compares.
const (
	KindText   = "text"   // contains the words, ignoring case
	KindList   = "list"   // one item equals the value, ignoring case
	KindDate   = "date"   // the date syntax
	KindNumber = "number" // F87's comparisons
	// KindGone is a stored condition on a field no longer in the mapping. Only ParseStored
	// produces it, and it never compiles: its playlist reads as stale (P0-6).
	KindGone = "gone"
)

// FieldSpec is one queryable mapped field.
type FieldSpec struct {
	Kind  string
	Label string
	// OwnerOnly: hidden from visitors, so an owner-only condition (SEC-8).
	OwnerOnly bool
}

// Fields is the catalogue of queryable mapped fields, from the owner's mapping and the
// registry's allowlist. Only a field named here reaches the compiler (SEC-8). The
// FieldReleased entry carries the release-date field's visibility.
type Fields map[string]FieldSpec

// Comparison operators a condition carries. OpEq is the plain `field:value`.
const (
	OpEq    = "="
	OpLt    = "<"
	OpLe    = "<="
	OpGt    = ">"
	OpGe    = ">="
	OpRange = ".."
)

// MatchedAny is `matched:any`: matched to at least one provider.
const MatchedAny = "any"

// entityFields name an entity by identity; canonicalisation binds them to an id.
var entityFields = map[string]bool{FieldTag: true, FieldPerson: true, FieldStudio: true, FieldCategory: true, FieldPlaylist: true}

// knownFields is every field the grammar accepts (P0-1).
var knownFields = map[string]bool{
	FieldTag: true, FieldPerson: true, FieldStudio: true, FieldCategory: true,
	FieldResolution: true, FieldYear: true, FieldDuration: true, FieldMatched: true,
	FieldAdded: true, FieldPlaylist: true, FieldUpdated: true, FieldReleased: true,
	FieldWatched: true, FieldPlays: true, FieldPlayed: true,
}

// IsBuiltin reports whether a name belongs to the grammar, so a mapped field of that name
// can't be queried by it (RD7).
func IsBuiltin(field string) bool { return knownFields[field] }

// dateFields compare by the date syntax without being mapped-field conditions.
var dateFields = map[string]bool{FieldAdded: true, FieldUpdated: true, FieldReleased: true, FieldPlayed: true}

// IsEntityField reports whether field names an entity bound by id.
func IsEntityField(field string) bool { return entityFields[field] }

// Node is one node of a parsed query: *Or, *And, *Not, *Cond or *Word.
type Node interface{ node() }

// Or matches when any item matches. It always has at least two items.
type Or struct{ Items []Node }

// And matches when every item matches. It always has at least two items.
type And struct{ Items []Node }

// Not matches when X does not.
type Not struct{ X Node }

// Cond is one `field:value` test. Entity fields carry ID once canonicalised (Value
// keeps the typed name until then); year and duration carry Lo (and Hi for a range),
// durations in seconds; a date field carries its value as written in Value (dates.go);
// resolution carries the bucket in Value; matched carries
// `any` or a provider name in Value. A mapped field carries its Kind, and its value by
// kind: text, list or date in Value, a number in Lo (and Hi). Owner marks a condition on a
// field hidden from visitors. Pos and End are byte offsets into the source.
type Cond struct {
	Field      string
	Op         string
	Value      string
	Quoted     bool
	ID         int64
	Lo, Hi     int64
	Pos        int
	End        int
	Kind       string
	FieldLabel string
	Owner      bool
}

// Word is a bare word or quoted phrase: title text, or the Tag of exactly that name or
// alias (RD1). It stays text in the canonical form and resolves at read time.
type Word struct {
	Text     string
	Pos, End int
}

func (*Or) node()   {}
func (*And) node()  {}
func (*Not) node()  {}
func (*Cond) node() {}
func (*Word) node() {}

// Query is a parsed expression with the source it came from, which errors slice by.
type Query struct {
	Root Node
	src  string
}

// Walk calls fn for every node of n, depth first, parents before children.
func Walk(n Node, fn func(Node)) {
	fn(n)
	switch t := n.(type) {
	case *Or:
		for _, it := range t.Items {
			Walk(it, fn)
		}
	case *And:
		for _, it := range t.Items {
			Walk(it, fn)
		}
	case *Not:
		Walk(t.X, fn)
	}
}

// Conds returns every condition in n, in source order.
func Conds(n Node) []*Cond {
	var out []*Cond
	Walk(n, func(x Node) {
		if c, ok := x.(*Cond); ok {
			out = append(out, c)
		}
	})
	return out
}

// ownerOnly reports whether one condition is the owner's alone.
func (c *Cond) ownerOnly() bool {
	switch c.Field {
	case FieldMatched, FieldWatched, FieldPlays, FieldPlayed:
		return true
	}
	return c.Owner
}

// OwnerOnly reports whether n holds a condition only the owner may run (`matched:`,
// P0-3; the watch conditions, P0-8; a field hidden from visitors, SEC-8), anywhere — under a NOT or inside an OR
// group included.
func OwnerOnly(n Node) bool {
	for _, c := range Conds(n) {
		if c.ownerOnly() {
			return true
		}
	}
	return false
}

// NeedsIndex reports whether n reads the resolved-value index or the change stamp, which
// the dirty set's drain keeps current (field-resolution.md, "Drain").
func NeedsIndex(n Node) bool {
	for _, c := range Conds(n) {
		if c.Kind != "" || c.Field == FieldUpdated || c.Field == FieldReleased {
			return true
		}
	}
	return false
}

// GoneFields returns the fields n names that are no longer in the mapping.
func GoneFields(n Node) []string {
	var out []string
	for _, c := range Conds(n) {
		if c.Kind == KindGone {
			out = append(out, c.Field)
		}
	}
	return out
}

// CheckOwner is the one owner-only check (security-perimeter.md): every entry point
// that runs or saves a query calls it with the caller's owner status. It returns an
// *Error with code CodeOwnerOnly for a non-owner query holding an owner-only condition.
func (q *Query) CheckOwner(isOwner bool) error {
	if isOwner {
		return nil
	}
	for _, c := range Conds(q.Root) {
		if c.ownerOnly() {
			return q.errAt(CodeOwnerOnly, "Part of this query is only available to the owner.", c.Pos, c.End)
		}
	}
	return nil
}
