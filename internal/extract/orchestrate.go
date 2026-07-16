package extract

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"holodex/internal/mapping"
	"holodex/internal/model"
	"holodex/internal/resolver"
)

// VideoLookup is the narrow repo slice Orchestrator needs to load a video's
// row and raw file tags. *repo.Repo satisfies this directly.
type VideoLookup interface {
	GetVideo(ctx context.Context, id int64) (*model.Video, []model.ExtraMetadata, error)
}

// Orchestrator ties pattern matching (F48.1), the filename shadow-store write
// (F48.2), and scoring+routing (F48.3/F48.4) into the single shared pipeline
// F48.5d requires every trigger — on-demand (F48.5a), batch (F48.5b), and
// import-time (F48.5c) — to call, so the three can never drift.
type Orchestrator struct {
	Videos   VideoLookup
	Mappings *mapping.Store
	Patterns *PatternStore
	Store    EnrichmentWriter
	Deps     Deps
}

// FieldOutcome is one field's routing result from a single video's run.
// Tagged for direct JSON serialization — the on-demand extraction endpoint
// (F48.5a) returns this straight through, matching the rest of this API's
// convention of handing domain values to writeJSON without a DTO layer.
type FieldOutcome struct {
	Field   string  `json:"field"`
	Outcome Outcome `json:"outcome"`
}

// Result is what ExtractVideo did for one video.
type Result struct {
	// Matched reports whether any configured pattern matched the filename
	// (F48.1b) — false means the file fell through to tag-only resolution
	// unchanged and Fields is empty.
	Matched bool           `json:"matched"`
	Fields  []FieldOutcome `json:"fields"`
}

// ExtractVideo runs F48.1-F48.4 for one video: match the filename against
// the configured patterns, store the parsed values into the filename shadow
// provider (F48.2a), then score and route each produced field (F48.3/F48.4).
// This is the one code path every extraction trigger calls (F48.5d).
func (o *Orchestrator) ExtractVideo(ctx context.Context, videoID int64) (Result, error) {
	v, extra, err := o.Videos.GetVideo(ctx, videoID)
	if err != nil {
		return Result{}, fmt.Errorf("extract: load video %d: %w", videoID, err)
	}

	patterns, delimiter := o.Patterns.Current()
	filenameFields, ok := MatchFirst(patterns, v.FilePath, delimiter)
	if !ok {
		return Result{Matched: false}, nil
	}

	if o.Store != nil {
		if err := Store(ctx, o.Store, model.EnrichEntityVideo, videoID, filenameFields); err != nil {
			return Result{}, fmt.Errorf("extract: store filename shadow values: %w", err)
		}
	}

	baseline := resolver.NewVideoBaseline(v, extra)
	mappings := o.Mappings.Current()

	fields := make([]string, 0, len(filenameFields))
	for field := range filenameFields {
		fields = append(fields, field)
	}
	sort.Strings(fields) // deterministic processing order

	out := make([]FieldOutcome, 0, len(fields))
	for _, field := range fields {
		outcome, err := Process(ctx, o.Deps, FieldExtraction{
			VideoID:        videoID,
			Field:          field,
			FilenameValues: filenameFields[field],
			TagValues:      fileTagValues(baseline, mappings, field),
		})
		if err != nil {
			return Result{}, fmt.Errorf("extract: process field %q: %w", field, err)
		}
		out = append(out, FieldOutcome{Field: field, Outcome: outcome})
	}
	return Result{Matched: true, Fields: out}, nil
}

// withCachedResolver returns a shallow copy of the Orchestrator whose
// Deps.Resolver caches EntityNames per entity type for the copy's lifetime
// (see cachingResolver). Used only by BatchRunner: a library-wide pass calls
// ExtractVideo once per video, and without this every people/studio field
// would otherwise re-read the full, unchanging entity table from scratch —
// exactly the cost resolveEntityMatch's doc comment (process.go) flagged and
// deferred until a real batch caller existed. Single-video triggers
// (on-demand, import-time) keep calling the shared Orchestrator directly, so
// they always see the current table — correct, since there's no repeated
// read to amortize for just one video.
func (o *Orchestrator) withCachedResolver() *Orchestrator {
	cp := *o
	if o.Deps.Resolver != nil {
		cp.Deps.Resolver = newCachingResolver(o.Deps.Resolver)
	}
	return &cp
}

// cachingResolver wraps a Resolver, caching each entity type's EntityNames
// result for its own lifetime. ExactEntityMatch is a single indexed lookup,
// not a full-table read, so it passes through uncached.
type cachingResolver struct {
	inner Resolver

	mu    sync.Mutex
	names map[string]map[int64]string
}

func newCachingResolver(inner Resolver) *cachingResolver {
	return &cachingResolver{inner: inner, names: make(map[string]map[int64]string)}
}

func (c *cachingResolver) ExactEntityMatch(ctx context.Context, entityType, name string) (int64, bool, error) {
	return c.inner.ExactEntityMatch(ctx, entityType, name)
}

func (c *cachingResolver) EntityNames(ctx context.Context, entityType string) (map[int64]string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if names, ok := c.names[entityType]; ok {
		return names, nil
	}
	names, err := c.inner.EntityNames(ctx, entityType)
	if err != nil {
		return nil, err
	}
	c.names[entityType] = names
	return names, nil
}

// fileTagValues collects the file-layer (baseline) values for a canonical
// field, unioned across every "file:" source in its mapping — the "what does
// the file currently say" half of F48.3's source-agreement comparison. A
// field with no mapping entry (shouldn't happen for a filename-produced
// field per F48.2b, but defensive) yields no tag values rather than failing
// the run. Empty values are filtered before mapping.Dedupe runs (rather than
// deduping in place per source) because Dedupe reuses its input slice's
// backing array — running it directly against a Baseline() result would
// corrupt the baseline's own stored values.
func fileTagValues(baseline resolver.BaselineSource, mappings *mapping.Mappings, field string) []string {
	f, ok := mappings.ByCanonical(field)
	if !ok {
		return nil
	}
	var all []string
	for _, src := range f.ParsedSources {
		vals, isBaseline := baseline.Baseline(src)
		if !isBaseline {
			continue
		}
		for _, val := range vals {
			if val != "" {
				all = append(all, val)
			}
		}
	}
	return mapping.Dedupe(all)
}
