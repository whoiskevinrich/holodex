package mcp

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

func newTestServer(t *testing.T) (*Server, *repo.Repo) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "mcp.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	r := repo.New(database)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return New(r, log), r
}

func seed(t *testing.T, r *repo.Repo, path, title string, dur, w int, people, tags []string) {
	t.Helper()
	rec := time.Date(2023, 1, 15, 12, 0, 0, 0, time.UTC)
	v := &model.Video{
		FilePath: path, Title: title, Duration: dur, Width: w, Height: w * 9 / 16,
		VideoCodec: "h264", AudioCodec: "aac", BitrateKbps: 5000, Container: "MP4",
		FileMtime: time.Now().UTC().Truncate(time.Second), RecordedAt: &rec,
	}
	for _, p := range people {
		v.People = append(v.People, model.Person{Name: p})
	}
	for _, tg := range tags {
		v.Tags = append(v.Tags, model.Tag{Name: tg})
	}
	if _, err := r.UpsertVideo(context.Background(), v, nil); err != nil {
		t.Fatalf("seed %s: %v", path, err)
	}
}

func call(t *testing.T, h func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) *mcp.CallToolResult {
	t.Helper()
	var req mcp.CallToolRequest
	req.Params.Arguments = args
	res, err := h(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return res
}

func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range r.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			return tc.Text
		}
	}
	t.Fatalf("no text content in result: %+v", r)
	return ""
}

func TestSearchVideos(t *testing.T) {
	s, r := newTestServer(t)
	seed(t, r, "/m/a.mkv", "Sunrise", 120, 3840, []string{"Alice"}, []string{"nature"})
	seed(t, r, "/m/b.mkv", "Sunset", 60, 1280, []string{"Bob"}, []string{"city"})

	// Plain query.
	var resp searchResponse
	if err := json.Unmarshal([]byte(resultText(t, call(t, s.searchVideos, map[string]any{"query": "sun"}))), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 2 || resp.Page != 1 || resp.PageSize != 20 {
		t.Errorf("query: total=%d page=%d size=%d", resp.Total, resp.Page, resp.PageSize)
	}

	// Filter by a person name → one result, resolution classified from width.
	resp = searchResponse{}
	json.Unmarshal([]byte(resultText(t, call(t, s.searchVideos, map[string]any{"people": []any{"Alice"}}))), &resp)
	if resp.Total != 1 || len(resp.Results) != 1 || resp.Results[0].Title != "Sunrise" {
		t.Fatalf("person filter: %+v", resp)
	}
	if resp.Results[0].Resolution != "4K" {
		t.Errorf("resolution = %q, want 4K", resp.Results[0].Resolution)
	}
	if resp.Results[0].People[0] != "Alice" {
		t.Errorf("people = %v", resp.Results[0].People)
	}

	// Unknown person name → empty (AND semantics can't match).
	resp = searchResponse{}
	json.Unmarshal([]byte(resultText(t, call(t, s.searchVideos, map[string]any{"people": []any{"Nobody"}}))), &resp)
	if resp.Total != 0 || len(resp.Results) != 0 {
		t.Errorf("unknown person should be empty: %+v", resp)
	}

	// resolution + date filters.
	resp = searchResponse{}
	json.Unmarshal([]byte(resultText(t, call(t, s.searchVideos, map[string]any{"resolution": "4K", "date_from": "2023-01-01", "date_to": "2023-12-31"}))), &resp)
	if resp.Total != 1 || resp.Results[0].Title != "Sunrise" {
		t.Errorf("resolution+date filter: %+v", resp)
	}
}

func TestGetVideo(t *testing.T) {
	s, r := newTestServer(t)
	seed(t, r, "/m/a.mp4", "Clip", 90, 1920, []string{"Alice"}, []string{"demo"})
	id := "1"

	var d videoDetail
	if err := json.Unmarshal([]byte(resultText(t, call(t, s.getVideo, map[string]any{"id": id}))), &d); err != nil {
		t.Fatal(err)
	}
	if d.Title != "Clip" || d.VideoCodec != "h264" || d.AudioCodec != "aac" || d.Container != "MP4" || d.BitrateKbps != 5000 {
		t.Errorf("detail = %+v", d)
	}
	if d.Resolution != "FHD" {
		t.Errorf("resolution = %q, want FHD", d.Resolution)
	}
	if len(d.People) != 1 || d.People[0].Name != "Alice" {
		t.Errorf("people = %+v", d.People)
	}

	// Unknown id → tool error result (not a transport error).
	res := call(t, s.getVideo, map[string]any{"id": "99999"})
	if !res.IsError {
		t.Errorf("missing id should be an error result")
	}
}

func TestListPeopleAndTags(t *testing.T) {
	s, r := newTestServer(t)
	seed(t, r, "/m/a.mkv", "A", 60, 1920, []string{"Alice", "Bob"}, []string{"nature"})
	seed(t, r, "/m/b.mkv", "B", 60, 1920, []string{"Alice"}, []string{"nature", "city"})

	var people []namedCount
	json.Unmarshal([]byte(resultText(t, call(t, s.listPeople, map[string]any{"sort": "count"}))), &people)
	if len(people) != 2 || people[0].Name != "Alice" || people[0].VideoCount != 2 {
		t.Errorf("people by count = %+v", people)
	}

	// Substring filter.
	var filtered []namedCount
	json.Unmarshal([]byte(resultText(t, call(t, s.listPeople, map[string]any{"query": "bo"}))), &filtered)
	if len(filtered) != 1 || filtered[0].Name != "Bob" {
		t.Errorf("people query=bo: %+v", filtered)
	}

	var tags []namedCount
	json.Unmarshal([]byte(resultText(t, call(t, s.listTags, map[string]any{"sort": "count"}))), &tags)
	if len(tags) != 2 || tags[0].Name != "nature" || tags[0].VideoCount != 2 {
		t.Errorf("tags by count = %+v", tags)
	}
}
