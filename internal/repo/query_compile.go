package repo

import (
	"fmt"
	"strings"
	"time"

	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/query"
)

// Per-condition clause fragments. VideoFilter.build() and the F87 expression compiler
// both call these, so `tag:12` and `?tag=12` are the same SQL and cannot drift
// (browse-and-list-state.md, "Boolean queries"). Each returns one predicate over `v`
// and its bound args; no input is ever interpolated.

func titleClause(text string) (string, []any) {
	return "v.id IN (SELECT rowid FROM videos_fts WHERE videos_fts MATCH ?)", []any{ftsPrefixQuery(text)}
}

func personClause(id int64) (string, []any) {
	return "EXISTS (SELECT 1 FROM video_people vp WHERE vp.video_id = v.id AND vp.person_id = ?)", []any{id}
}

// tagClause is descendant-inclusive (F50, ADR-075 D1/P0-6): a video tagged only with a
// descendant of id still matches.
func tagClause(id int64) (string, []any) {
	return "EXISTS (SELECT 1 FROM video_tags vt WHERE vt.video_id = v.id AND vt.tag_id IN (" + tagSubtreeQuery + "))", []any{id}
}

func studioClause(id int64) (string, []any) {
	return "EXISTS (SELECT 1 FROM video_studios vs WHERE vs.video_id = v.id AND vs.studio_id = ?)", []any{id}
}

// categoryClause expands a category to its member tags (ADR-078 D2).
func categoryClause(id int64) (string, []any) {
	return "EXISTS (SELECT 1 FROM video_tags vt WHERE vt.video_id = v.id AND vt.tag_id IN (SELECT tag_id FROM category_tags WHERE category_id = ?))", []any{id}
}

// durationClause compares v.duration_sec; op is one of the query package's operators.
func durationClause(op string, sec int64) (string, []any) {
	return "v.duration_sec " + sqlOp(op) + " ?", []any{sec}
}

// widthClause is a resolution bucket's width range (ADR-012); a zero bound is open.
func widthClause(min, max int) (string, []any) {
	var parts []string
	var args []any
	if min > 0 {
		parts, args = append(parts, "v.width >= ?"), append(args, min)
	}
	if max > 0 {
		parts, args = append(parts, "v.width < ?"), append(args, max)
	}
	if len(parts) == 0 {
		return "1", nil
	}
	return strings.Join(parts, " AND "), args
}

func yearStart(y int64) string { return fmt.Sprintf("%04d-01-01T00:00:00Z", y) }
func yearEnd(y int64) string   { return fmt.Sprintf("%04d-12-31T23:59:59Z", y) }

// yearMinClause / yearMaxClause are the flat keys' inclusive year bounds over the
// RFC3339 recorded_at.
func yearMinClause(y int64) (string, []any) { return "v.recorded_at >= ?", []any{yearStart(y)} }
func yearMaxClause(y int64) (string, []any) { return "v.recorded_at <= ?", []any{yearEnd(y)} }

// sqlOp maps a comparison operator to SQL from a fixed set, never from input.
func sqlOp(op string) string {
	switch op {
	case query.OpLt:
		return "<"
	case query.OpLe:
		return "<="
	case query.OpGt:
		return ">"
	case query.OpGe:
		return ">="
	}
	return "="
}

// queryWordTagSeed selects the Tag whose name or alias is exactly the bound word (RD1),
// by the same name key the identity lookup uses (entity-identity.md). Two args: the word twice.
var queryWordTagSeed = fmt.Sprintf(
	`SELECT id FROM tags WHERE %s = %s UNION SELECT entity_id FROM entity_aliases WHERE entity_type = '%s' AND alias_key = %s`,
	nameKeyExpr(model.EntityTag, "name"), nameKeyExpr(model.EntityTag, "?"), model.EntityTag, nameKeyExpr(model.EntityTag, "?"))

// wordClause is a bare word (RD1): the title matches it, or the video has the Tag whose
// name or alias is exactly it, descendants included. A Tag created later widens it.
func wordClause(text string) (string, []any) {
	title, args := titleClause(text)
	tags := `EXISTS (SELECT 1 FROM video_tags vt WHERE vt.video_id = v.id AND vt.tag_id IN (
		WITH RECURSIVE tag_subtree(id) AS (
			` + queryWordTagSeed + `
			UNION
			SELECT t.id FROM tags t JOIN tag_subtree s ON t.parent_tag_id = s.id
		)
		SELECT id FROM tag_subtree))`
	return "(" + title + " OR " + tags + ")", append(args, text, text)
}

// matchedClause is `matched:` (P0-3): the video has enrichment rows in the shadow store
// (metadata-providers.md), from any provider or the named one.
func matchedClause(provider string) (string, []any) {
	const base = "EXISTS (SELECT 1 FROM entity_enrichment ee WHERE ee.entity_type = '" + model.EnrichEntityVideo + "' AND ee.entity_id = v.id"
	if provider == query.MatchedAny {
		return base + ")", nil
	}
	return base + " AND ee.provider = ?)", []any{provider}
}

// instantClause is `added:` or `updated:` (F89 P0-1) over a video column of RFC3339 UTC
// text, so the bounds compare as strings. A video with no value (a row written by a path
// other than the scanner) matches no date, and so its negation includes it. col is a
// literal from the compiler, never input.
func instantClause(col string, c *query.Cond, now time.Time) (string, []any) {
	lo, hi, ok := query.Bounds(c, now, time.Local)
	if !ok {
		return "0", nil
	}
	s, args := col+" IS NOT NULL", []any(nil)
	if !lo.IsZero() {
		s, args = s+" AND "+col+" >= ?", append(args, lo.UTC().Format(timeLayout))
	}
	if !hi.IsZero() {
		s, args = s+" AND "+col+" < ?", append(args, hi.UTC().Format(timeLayout))
	}
	return s, args
}

// watchedClause is `watched:` (P0-8). A video with no summary row is unwatched.
func watchedClause(value string) (string, []any) {
	s := "EXISTS (SELECT 1 FROM video_watch w WHERE w.video_id = v.id AND " + watchedExpr + ")"
	if value == query.WatchedNo {
		s = "NOT " + s
	}
	return s, nil
}

// playsClause is `plays:` (P0-8), where a video with no summary row has no plays.
func playsClause(c *query.Cond) (string, []any) {
	const count = "COALESCE((SELECT w.play_count FROM video_watch w WHERE w.video_id = v.id), 0)"
	if c.Op == query.OpRange {
		return count + " >= ? AND " + count + " <= ?", []any{c.Lo, c.Hi}
	}
	return count + " " + sqlOp(c.Op) + " ?", []any{c.Lo}
}

// PlaylistRef is one referenced playlist a `playlist:` condition compiles against (F89
// P0-3): a snapshot's membership rows, or a smart playlist's own filter, cap included.
// The api layer builds these (it owns the loop, depth and visibility checks, SEC-5/6);
// a condition naming a playlist with no ref matches nothing, so a gap never widens.
type PlaylistRef struct {
	ID       int64
	Snapshot bool
	Filter   VideoFilter // a smart playlist's; unused for a snapshot
}

// compileEnv is what an expression compiles against beyond the tree: the clock relative
// dates count from, and the referenced playlists.
type compileEnv struct {
	now  time.Time
	refs map[int64]*PlaylistRef
}

// compileExpr compiles a canonical F87 tree to one predicate: AND / OR / NOT nest as
// parenthesised SQL, and each condition is the flat keys' own fragment. The tree is
// bounded by the parser's caps, so the SQL is too.
func compileExpr(n query.Node) (string, []any) {
	return compileEnv{now: time.Now()}.expr(n)
}

func (e compileEnv) expr(n query.Node) (string, []any) {
	switch t := n.(type) {
	case *query.And:
		return e.list(t.Items, " AND ")
	case *query.Or:
		return e.list(t.Items, " OR ")
	case *query.Not:
		s, args := e.expr(t.X)
		return "NOT (" + s + ")", args
	case *query.Word:
		return wordClause(t.Text)
	case *query.Cond:
		return e.cond(t)
	}
	return "0", nil
}

func (e compileEnv) list(items []query.Node, sep string) (string, []any) {
	parts := make([]string, len(items))
	var args []any
	for i, it := range items {
		s, a := e.expr(it)
		parts[i] = "(" + s + ")"
		args = append(args, a...)
	}
	return strings.Join(parts, sep), args
}

func (e compileEnv) cond(c *query.Cond) (string, []any) {
	switch c.Field {
	case query.FieldAdded:
		return instantClause("v.added_at", c, e.now)
	case query.FieldUpdated:
		return instantClause("v.changed_at", c, e.now)
	case query.FieldReleased:
		return resolvedClause(query.ReleaseDateKey, query.KindDate, c, e.now)
	case query.FieldPlaylist:
		return e.playlistClause(e.refs[c.ID])
	case query.FieldWatched:
		return watchedClause(c.Value)
	case query.FieldPlays:
		return playsClause(c)
	case query.FieldPlayed:
		// A video never played has no row, so matches no date, and its negation includes it.
		s, args := instantClause("w.last_played_at", c, e.now)
		return "EXISTS (SELECT 1 FROM video_watch w WHERE w.video_id = v.id AND " + s + ")", args
	}
	if c.Kind != "" {
		return resolvedClause(c.Field, c.Kind, c, e.now)
	}
	return compileCond(c)
}

// playlistClause is membership in a referenced playlist as it is now. A smart one is
// its own filter as a subquery, in its sort and cut at its cap when it has one, so the
// referencing playlist sees exactly what the referenced one shows. It counts relative
// dates from the same clock as the outer query.
func (e compileEnv) playlistClause(ref *PlaylistRef) (string, []any) {
	if ref == nil {
		return "0", nil
	}
	if ref.Snapshot {
		return "v.id IN (SELECT pv.video_id FROM playlist_videos pv WHERE pv.playlist_id = ?)", []any{ref.ID}
	}
	inner := ref.Filter
	inner.Now = e.now
	where, args := inner.build()
	sub := "SELECT v.id FROM videos v " + where
	if ref.Filter.Cap > 0 {
		order, orderArgs := ref.Filter.orderBy()
		sub += " ORDER BY " + order + " LIMIT ?"
		args = append(append(args, orderArgs...), ref.Filter.Cap)
	}
	return "v.id IN (" + sub + ")", args
}

func compileCond(c *query.Cond) (string, []any) {
	switch c.Field {
	case query.FieldTag:
		return tagClause(c.ID)
	case query.FieldPerson:
		return personClause(c.ID)
	case query.FieldStudio:
		return studioClause(c.ID)
	case query.FieldCategory:
		return categoryClause(c.ID)
	case query.FieldResolution:
		b, _ := metadata.ParseResolutionBucket(c.Value)
		return widthClause(metadata.ResolutionWidthRange(b))
	case query.FieldDuration:
		if c.Op == query.OpRange {
			lo, a := durationClause(query.OpGe, c.Lo)
			hi, b := durationClause(query.OpLe, c.Hi)
			return lo + " AND " + hi, append(a, b...)
		}
		return durationClause(c.Op, c.Lo)
	case query.FieldYear:
		return yearCond(c)
	case query.FieldMatched:
		return matchedClause(c.Value)
	}
	return "0", nil
}

// yearCond compares whole years over recorded_at. A video with no recorded date has no
// year, so it matches no year condition — and so `-year:…` includes it, rather than
// a NULL comparison dropping it from both sides.
func yearCond(c *query.Cond) (string, []any) {
	var s string
	var args []any
	switch c.Op {
	case query.OpLt:
		s, args = "v.recorded_at < ?", []any{yearStart(c.Lo)}
	case query.OpLe:
		s, args = yearMaxClause(c.Lo)
	case query.OpGt:
		s, args = "v.recorded_at > ?", []any{yearEnd(c.Lo)}
	case query.OpGe:
		s, args = yearMinClause(c.Lo)
	default:
		hi := c.Lo
		if c.Op == query.OpRange {
			hi = c.Hi
		}
		lo, a := yearMinClause(c.Lo)
		up, b := yearMaxClause(hi)
		s, args = lo+" AND "+up, append(a, b...)
	}
	return "v.recorded_at IS NOT NULL AND " + s, args
}
