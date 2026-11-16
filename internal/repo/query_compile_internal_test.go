package repo

import (
	"strings"
	"testing"
	"time"

	"holodex/internal/query"
)

// Q8: the compiler binds every value, parenthesises every AND / OR / NOT, and compiles a
// bare word to the title MATCH through ftsPrefixQuery OR an exact-Tag subtree.
func TestCompileExpr(t *testing.T) {
	src := `((tag:12 tag:34) OR person:56) -studio:78 "it's here" duration:20m..1h year:<1990 matched:provider-x resolution:HD category:9`
	q, err := query.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	sql, args := compileEnv{now: time.Now(), providers: []string{"provider-x"}}.expr(q.Root)

	for _, input := range []string{"it's", "here", "provider-x", "1990", "1200", "3600"} {
		if strings.Contains(sql, input) {
			t.Errorf("SQL contains input %q:\n%s", input, sql)
		}
	}
	want := map[any]bool{int64(12): false, int64(34): false, int64(56): false, int64(78): false, int64(9): false,
		`"it's"* "here"*`: false, "it's here": false, "provider-x": false, int64(1200): false, int64(3600): false, "1990-01-01T00:00:00Z": false}
	for _, a := range args {
		if _, ok := want[a]; ok {
			want[a] = true
		}
	}
	for v, seen := range want {
		if !seen {
			t.Errorf("arg %v not bound; args %v", v, args)
		}
	}
	if n := strings.Count(sql, "?"); n != len(args) {
		t.Errorf("%d placeholders, %d args", n, len(args))
	}
	if !strings.HasPrefix(sql, "((") || !strings.Contains(sql, ") OR (") || !strings.Contains(sql, "(NOT (") {
		t.Errorf("composites not parenthesised:\n%s", sql)
	}
	if !strings.Contains(sql, "videos_fts MATCH ?) OR EXISTS (SELECT 1 FROM video_tags") {
		t.Errorf("bare word is not title OR exact Tag:\n%s", sql)
	}
}

// The flat keys and the compiler share one fragment per condition (risk 4).
func TestFlatAndExprShareFragments(t *testing.T) {
	q, _ := query.Parse("tag:5")
	exprSQL, _ := compileExpr(q.Root)
	flat, _ := VideoFilter{TagIDs: []int64{5}}.build(nil)
	if !strings.Contains(flat, exprSQL) {
		t.Errorf("tag:5 compiles to\n%s\nwhich isn't in ?tag=5's\n%s", exprSQL, flat)
	}
}

// F90 P0-1: matched: binds the declared names, and counts nothing they don't name.
func TestMatchedClauseDeclaredOnly(t *testing.T) {
	sql, args := matchedClause(query.MatchedAny, []string{"provider-1", "provider-2"})
	if !strings.Contains(sql, "ee.provider IN (?,?)") || len(args) != 2 {
		t.Errorf("matched:any = %s %v", sql, args)
	}
	for _, tc := range []struct {
		provider string
		declared []string
	}{{query.MatchedAny, nil}, {"provider-9", []string{"provider-1"}}, {"filename", []string{"provider-1"}}} {
		if sql, _ := matchedClause(tc.provider, tc.declared); sql != "0" {
			t.Errorf("matched:%s with %v = %s, want 0", tc.provider, tc.declared, sql)
		}
	}
}
