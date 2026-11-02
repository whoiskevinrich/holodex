package mcp

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"holodex/internal/model"
	"holodex/internal/query"
)

// TestSearchVideosExpr covers testing-strategy §38.2 A11: search_videos' expr runs the
// shared parse, owner check and caps — the tool builds its filter without
// mediaFilterFor — resolves names through aliases, and answers with the canonical form.
func TestSearchVideosExpr(t *testing.T) {
	s, r := newTestServer(t)
	ctx := context.Background()
	seed(t, r, "/m/a.mkv", "Sunrise", 120, 3840, []string{"Alice"}, []string{"nature"})
	seed(t, r, "/m/b.mkv", "Sunset", 60, 1280, []string{"Bob"}, []string{"city"})
	seed(t, r, "/m/c.mkv", "Noon", 30, 1280, []string{"Alice"}, []string{"city"})
	nature, _, _ := r.TagIDByName(ctx, "nature")
	alice, _, _ := r.PersonIDByName(ctx, "Alice")
	if _, err := r.AddEntityAlias(ctx, model.EntityTag, nature, "outdoors"); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertEnrichment(ctx, model.EnrichEntityVideo, 1, "provider-1", "x", map[string][]string{"title": {"Sunrise"}}); err != nil {
		t.Fatal(err)
	}

	search := func(ctx context.Context, expr string) (*mcp.CallToolResult, searchResponse) {
		t.Helper()
		var req mcp.CallToolRequest
		req.Params.Arguments = map[string]any{"expr": expr}
		res, err := s.searchVideos(ctx, req)
		if err != nil {
			t.Fatalf("search %q: %v", expr, err)
		}
		var resp searchResponse
		if !res.IsError {
			if err := json.Unmarshal([]byte(resultText(t, res)), &resp); err != nil {
				t.Fatal(err)
			}
		}
		return res, resp
	}
	owner, visitor := withOwner(ctx, true), withOwner(ctx, false)

	_, resp := search(visitor, `tag:outdoors OR (person:alice -tag:nature)`)
	if resp.Total != 2 || resp.Expr != "tag:"+strconv.FormatInt(nature, 10)+" OR person:"+strconv.FormatInt(alice, 10)+" -tag:"+strconv.FormatInt(nature, 10) {
		t.Errorf("alias query: total %d expr %q", resp.Total, resp.Expr)
	}

	res, _ := search(visitor, "-matched:any")
	if !res.IsError || !strings.Contains(resultText(t, res), query.CodeOwnerOnly) {
		t.Errorf("visitor matched: %+v", res)
	}
	if _, resp = search(owner, "-matched:any"); resp.Total != 2 {
		t.Errorf("owner -matched:any total %d, want 2", resp.Total)
	}

	res, _ = search(visitor, strings.Repeat("(", query.MaxDepth+1)+"a"+strings.Repeat(")", query.MaxDepth+1))
	if !res.IsError || !strings.Contains(resultText(t, res), query.CodeTooDeep) {
		t.Errorf("depth cap: %+v", res)
	}
	res, _ = search(visitor, "tag:natur")
	if text := resultText(t, res); !res.IsError || !strings.Contains(text, "nature") {
		t.Errorf("unknown tag should suggest nature: %q", text)
	}
}
