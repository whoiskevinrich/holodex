// Package mcp exposes the Holodex library over the Model Context Protocol
// (F10, ADR-005): four read-only tools (search_videos, get_video, list_people,
// list_tags) backed by the same repository the REST API uses — no duplication.
// Transports are stdio (local, via `holodex -mcp-transport stdio`) and
// Streamable HTTP at /mcp with a legacy SSE stream at /mcp/sse.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"holodex/internal/mapping"
	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/repo"
)

const serverVersion = "0.2.0"

// Server wraps the MCP server and the repository its tools read from.
type Server struct {
	repo     *repo.Repo
	log      *slog.Logger
	mappings *mapping.Store // configurable mapped fields (F20.6); nil disables them
	mcp      *mcpserver.MCPServer
}

// New builds the MCP server and registers the four Phase 2 tools. mappings may be
// nil (no configurable filterable fields on search_videos).
func New(r *repo.Repo, log *slog.Logger, mappings *mapping.Store) *Server {
	s := &Server{repo: r, log: log, mappings: mappings}
	m := mcpserver.NewMCPServer("holodex", serverVersion)
	s.register(m)
	s.mcp = m
	return s
}

// ServeStdio runs the MCP server over stdin/stdout, blocking until stdin closes.
// This is the `docker exec -i holodex holodex -mcp-transport stdio` entrypoint.
func (s *Server) ServeStdio() error { return mcpserver.ServeStdio(s.mcp) }

// StartHTTP serves Streamable HTTP at /mcp and the legacy SSE transport at
// /mcp/sse (+ /mcp/message) on addr, shutting down when ctx is cancelled.
func (s *Server) StartHTTP(ctx context.Context, addr string) error {
	streamable := mcpserver.NewStreamableHTTPServer(s.mcp, mcpserver.WithEndpointPath("/mcp"))
	sse := mcpserver.NewSSEServer(s.mcp, mcpserver.WithStaticBasePath("/mcp"))

	mux := http.NewServeMux()
	mux.Handle("/mcp", streamable)
	mux.Handle("/mcp/sse", sse.SSEHandler())
	mux.Handle("/mcp/message", sse.MessageHandler())

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		// Short drain: MCP holds no long-running requests (tools return promptly),
		// unlike the web server's 15s graceful window.
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
	}()
	s.log.Info("mcp http listening", "addr", addr, "endpoints", "/mcp, /mcp/sse")
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) register(m *mcpserver.MCPServer) {
	m.AddTool(mcp.NewTool("search_videos",
		mcp.WithDescription("Search the video library by title, people, tags, duration, resolution, and recorded date. Returns a page of matching videos with pagination."),
		mcp.WithString("query", mcp.Description("Case-insensitive title substring to match")),
		mcp.WithArray("people", mcp.Description("Person names; a video must include every one (AND)"), mcp.WithStringItems()),
		mcp.WithArray("tags", mcp.Description("Tag names; a video must include every one (AND)"), mcp.WithStringItems()),
		mcp.WithNumber("duration_min", mcp.Description("Minimum duration in seconds")),
		mcp.WithNumber("duration_max", mcp.Description("Maximum duration in seconds")),
		mcp.WithString("resolution", mcp.Description("Resolution tier"), mcp.Enum("SD", "HD", "FHD", "4K")),
		mcp.WithString("date_from", mcp.Description("Earliest recorded date, ISO YYYY-MM-DD")),
		mcp.WithString("date_to", mcp.Description("Latest recorded date, ISO YYYY-MM-DD")),
		mcp.WithNumber("page", mcp.Description("1-based page number"), mcp.DefaultNumber(1)),
		mcp.WithNumber("page_size", mcp.Description("Results per page, 1-100"), mcp.DefaultNumber(20)),
		mcp.WithObject("fields", mcp.Description(`Filterable configurable metadata fields keyed by canonical name, e.g. {"studio":"Acme"} (F20.6)`)),
	), s.searchVideos)

	m.AddTool(mcp.NewTool("get_video",
		mcp.WithDescription("Fetch full metadata for one video by id, including file path, technical stream details, people, and tags."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Video id")),
	), s.getVideo)

	m.AddTool(mcp.NewTool("list_people",
		mcp.WithDescription("List people with their video counts, optionally filtered by a name substring."),
		mcp.WithString("query", mcp.Description("Case-insensitive name substring filter")),
		mcp.WithString("sort", mcp.Description("Sort order"), mcp.Enum("name", "count"), mcp.DefaultString("name")),
	), s.listPeople)

	m.AddTool(mcp.NewTool("list_tags",
		mcp.WithDescription("List tags with their video counts, optionally filtered by a name substring."),
		mcp.WithString("query", mcp.Description("Case-insensitive name substring filter")),
		mcp.WithString("sort", mcp.Description("Sort order"), mcp.Enum("name", "count"), mcp.DefaultString("name")),
	), s.listTags)
}

// ---- tool handlers ----

func (s *Server) searchVideos(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	page := req.GetInt("page", 1)
	if page < 1 {
		page = 1
	}
	pageSize := req.GetInt("page_size", 20)
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	f := repo.VideoFilter{
		Query:          req.GetString("query", ""),
		DurationMinSec: req.GetInt("duration_min", 0),
		DurationMaxSec: req.GetInt("duration_max", 0),
		DateFrom:       req.GetString("date_from", ""),
		DateTo:         req.GetString("date_to", ""),
		Limit:          pageSize,
		Offset:         (page - 1) * pageSize,
	}
	if b, ok := metadata.ParseResolutionBucket(req.GetString("resolution", "")); ok {
		f.WidthMin, f.WidthMax = metadata.ResolutionWidthRange(b)
	}

	empty := searchResponse{Results: []searchItem{}, Page: page, PageSize: pageSize}
	// Resolve people/tag names to ids (AND semantics). An unknown name can match
	// no video, so allFound=false short-circuits the whole search to empty — the
	// caller asked for someone or something not in the library.
	resolve := func(names []string, lookup func(context.Context, string) (int64, bool, error)) (ids []int64, allFound bool, err error) {
		for _, name := range names {
			id, ok, lerr := lookup(ctx, name)
			if lerr != nil || !ok {
				return nil, ok, lerr
			}
			ids = append(ids, id)
		}
		return ids, true, nil
	}
	pids, ok, err := resolve(req.GetStringSlice("people", nil), s.repo.PersonIDByName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return jsonResult(empty)
	}
	tids, ok, err := resolve(req.GetStringSlice("tags", nil), s.repo.TagIDByName)
	if err != nil {
		return nil, err
	}
	if !ok {
		return jsonResult(empty)
	}
	f.PersonIDs, f.TagIDs = pids, tids

	// Filterable mapped fields (F20.6): {canonical: value}, resolved via the config.
	if s.mappings != nil {
		if raw, ok := req.GetArguments()["fields"].(map[string]any); ok {
			cur := s.mappings.Current()
			for canonical, v := range raw {
				val := strings.TrimSpace(fmt.Sprintf("%v", v))
				if val == "" {
					continue
				}
				if fld, ok := cur.ByCanonical(canonical); ok && fld.Filterable {
					f.MappedFilters = append(f.MappedFilters, repo.MappedFilter{SourceKeys: fld.Sources, Value: val})
				}
			}
		}
	}

	vids, total, err := s.repo.ListVideos(ctx, f)
	if err != nil {
		return nil, err
	}
	resp := searchResponse{Results: make([]searchItem, 0, len(vids)), Total: total, Page: page, PageSize: pageSize}
	for i := range vids {
		resp.Results = append(resp.Results, toSearchItem(&vids[i]))
	}
	return jsonResult(resp)
}

func (s *Server) getVideo(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idStr, err := req.RequireString("id")
	if err != nil {
		return mcp.NewToolResultError("id is required"), nil
	}
	id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
	if err != nil || id <= 0 {
		return mcp.NewToolResultError("invalid id"), nil
	}
	v, extra, err := s.repo.GetVideo(ctx, id)
	if errors.Is(err, repo.ErrNotFound) {
		return mcp.NewToolResultError(fmt.Sprintf("video %d not found", id)), nil
	}
	if err != nil {
		return nil, err
	}
	return jsonResult(toVideoDetail(v, extra))
}

func (s *Server) listPeople(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	people, err := s.repo.ListPeople(ctx, req.GetString("sort", "name") == "count")
	if err != nil {
		return nil, err
	}
	counts := mapSlice(people, func(p model.Person) namedCount {
		return namedCount{ID: p.ID, Name: p.Name, VideoCount: p.VideoCount}
	})
	return jsonResult(filterNamed(counts, req.GetString("query", "")))
}

func (s *Server) listTags(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tags, err := s.repo.ListTags(ctx, req.GetString("sort", "name") == "count")
	if err != nil {
		return nil, err
	}
	counts := mapSlice(tags, func(t model.Tag) namedCount {
		return namedCount{ID: t.ID, Name: t.Name, VideoCount: t.VideoCount}
	})
	return jsonResult(filterNamed(counts, req.GetString("query", "")))
}

// ---- response shapes (MCP Tool Response Schemas, phase-2 spec) ----

type searchItem struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	DurationSec  int      `json:"duration_sec"`
	Resolution   string   `json:"resolution"`
	RecordedAt   string   `json:"recorded_at,omitempty"`
	People       []string `json:"people"`
	Tags         []string `json:"tags"`
	ThumbnailURL *string  `json:"thumbnail_url"`
}

type searchResponse struct {
	Results  []searchItem `json:"results"`
	Total    int          `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
}

type idName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type namedCount struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	VideoCount int    `json:"video_count"`
}

type videoDetail struct {
	ID           string                `json:"id"`
	Title        string                `json:"title"`
	FilePath     string                `json:"file_path"`
	FileSize     int64                 `json:"file_size"`
	DurationSec  int                   `json:"duration_sec"`
	Width        int                   `json:"width"`
	Height       int                   `json:"height"`
	Resolution   string                `json:"resolution"`
	VideoCodec   string                `json:"video_codec,omitempty"`
	AudioCodec   string                `json:"audio_codec,omitempty"`
	BitrateKbps  int                   `json:"bitrate_kbps,omitempty"`
	Container    string                `json:"container,omitempty"`
	RecordedAt   *string               `json:"recorded_at"`
	IndexedAt    string                `json:"indexed_at"`
	People       []idName              `json:"people"`
	Tags         []idName              `json:"tags"`
	Metadata     []model.ExtraMetadata `json:"metadata,omitempty"`
	ThumbnailURL *string               `json:"thumbnail_url"`
}

func toSearchItem(v *model.Video) searchItem {
	item := searchItem{
		ID:           strconv.FormatInt(v.ID, 10),
		Title:        v.Title,
		DurationSec:  v.Duration,
		Resolution:   string(metadata.ClassifyResolution(v.Width)),
		People:       mapSlice(v.People, func(p model.Person) string { return p.Name }),
		Tags:         mapSlice(v.Tags, func(t model.Tag) string { return t.Name }),
		ThumbnailURL: thumbnailURL(v),
	}
	if v.RecordedAt != nil {
		item.RecordedAt = v.RecordedAt.UTC().Format("2006-01-02")
	}
	return item
}

func toVideoDetail(v *model.Video, extra []model.ExtraMetadata) videoDetail {
	d := videoDetail{
		ID:           strconv.FormatInt(v.ID, 10),
		Title:        v.Title,
		FilePath:     v.FilePath,
		FileSize:     v.FileSize,
		DurationSec:  v.Duration,
		Width:        v.Width,
		Height:       v.Height,
		Resolution:   string(metadata.ClassifyResolution(v.Width)),
		VideoCodec:   v.VideoCodec,
		AudioCodec:   v.AudioCodec,
		BitrateKbps:  v.BitrateKbps,
		Container:    v.Container,
		IndexedAt:    v.IndexedAt.UTC().Format(time.RFC3339),
		People:       mapSlice(v.People, func(p model.Person) idName { return idName{ID: strconv.FormatInt(p.ID, 10), Name: p.Name} }),
		Tags:         mapSlice(v.Tags, func(t model.Tag) idName { return idName{ID: strconv.FormatInt(t.ID, 10), Name: t.Name} }),
		Metadata:     extra,
		ThumbnailURL: thumbnailURL(v),
	}
	if v.RecordedAt != nil {
		rec := v.RecordedAt.UTC().Format(time.RFC3339)
		d.RecordedAt = &rec
	}
	return d
}

// thumbnailURL returns the REST serving path when an image exists, else nil. The
// path is relative; an MCP client on the same host resolves it against :7800.
func thumbnailURL(v *model.Video) *string {
	if !model.HasThumbnailImage(v.ThumbnailState) {
		return nil
	}
	u := fmt.Sprintf("/api/v1/media/%d/thumbnail", v.ID)
	return &u
}

// mapSlice projects each element of xs through f — one generic in place of the
// per-type people/tag mappers.
func mapSlice[T, R any](xs []T, f func(T) R) []R {
	out := make([]R, len(xs))
	for i, x := range xs {
		out[i] = f(x)
	}
	return out
}

// filterNamed keeps the rows whose name contains query (case-insensitive); an
// empty query passes everything. Shared by list_people and list_tags.
func filterNamed(items []namedCount, query string) []namedCount {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return items
	}
	out := make([]namedCount, 0, len(items))
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Name), q) {
			out = append(out, it)
		}
	}
	return out
}

func jsonResult(v any) (*mcp.CallToolResult, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal mcp result: %w", err)
	}
	return mcp.NewToolResultText(string(b)), nil
}
