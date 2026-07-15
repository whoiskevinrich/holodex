package extract

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"holodex/internal/mapping"
	"holodex/internal/model"
)

// fakeVideoLookup is an in-memory VideoLookup keyed by video id.
type fakeVideoLookup struct {
	videos map[int64]*model.Video
	extras map[int64][]model.ExtraMetadata
}

func (f *fakeVideoLookup) GetVideo(_ context.Context, id int64) (*model.Video, []model.ExtraMetadata, error) {
	v, ok := f.videos[id]
	if !ok {
		return nil, nil, errNotFound
	}
	return v, f.extras[id], nil
}

var errNotFound = errors.New("video not found")

// fakeEnrichmentWriter captures UpsertEnrichment calls (Store's write path,
// F48.2a) without a real repo.
type fakeEnrichmentWriter struct {
	calls []fakeEnrichmentCall
}

type fakeEnrichmentCall struct {
	EntityType string
	EntityID   int64
	Provider   string
	Fields     map[string][]string
}

func (f *fakeEnrichmentWriter) UpsertEnrichment(_ context.Context, entityType string, entityID int64, provider, _ string, fields map[string][]string) error {
	f.calls = append(f.calls, fakeEnrichmentCall{entityType, entityID, provider, fields})
	return nil
}

// testMappings builds a *mapping.Store with title/people/studio/release_date
// fields sourced from both "file:" tags and "filename:" (F48.2b), so
// fileTagValues has something real to read.
func testMappings(t *testing.T) *mapping.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metadata-mappings.yaml")
	yaml := `
fields:
  - canonical: title
    sources: ["file:title", "filename:title"]
  - canonical: people
    sources: ["file:Artist", "filename:people"]
    multi: true
  - canonical: studio
    sources: ["file:Publisher", "filename:studio"]
  - canonical: release_date
    sources: ["file:Date", "filename:release_date"]
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write mappings: %v", err)
	}
	store, err := mapping.NewStore(path)
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}
	return store
}

// testPatterns wraps CompileAll's output directly into a PatternStore,
// bypassing YAML — this package can reach PatternStore's unexported fields.
func testPatterns(t *testing.T, patterns ...string) *PatternStore {
	t.Helper()
	compiled, err := CompileAll(patterns)
	if err != nil {
		t.Fatalf("CompileAll: %v", err)
	}
	s := &PatternStore{}
	s.cur.Store(&compiledPatterns{patterns: compiled, delimiter: ", "})
	return s
}

// TestOrchestrator_ExtractVideo_NoMatch proves F48.1b: a filename matching no
// configured pattern falls through untouched — no shadow-store write, no
// field processing.
func TestOrchestrator_ExtractVideo_NoMatch(t *testing.T) {
	videos := &fakeVideoLookup{videos: map[int64]*model.Video{
		1: {FilePath: "/media/unrelated_filename.mkv"},
	}}
	writer := &fakeEnrichmentWriter{}
	o := &Orchestrator{
		Videos:   videos,
		Mappings: testMappings(t),
		Patterns: testPatterns(t, "[{studio}] {title} ({people}, {year})"),
		Store:    writer,
	}

	res, err := o.ExtractVideo(context.Background(), 1)
	if err != nil {
		t.Fatalf("ExtractVideo: %v", err)
	}
	if res.Matched {
		t.Fatalf("Matched = true, want false")
	}
	if len(res.Fields) != 0 {
		t.Fatalf("Fields = %v, want empty", res.Fields)
	}
	if len(writer.calls) != 0 {
		t.Fatalf("Store called %d times, want 0", len(writer.calls))
	}
}

// TestOrchestrator_ExtractVideo_MatchStoresAndRoutes is the F48.5 end-to-end
// shape: a matching filename stores its parsed values into the filename
// shadow provider (F48.2a) and routes every produced field (F48.3/F48.4).
// This is also the F48.5d regression guard in miniature — on-demand, batch,
// and import-time all call this exact method with no per-trigger branching,
// so proving its behavior here proves it for all three triggers at once.
func TestOrchestrator_ExtractVideo_MatchStoresAndRoutes(t *testing.T) {
	videos := &fakeVideoLookup{
		videos: map[int64]*model.Video{
			1: {FilePath: "/media/[Acme Studios] Big Movie (2020).mkv"},
		},
	}
	writer := &fakeEnrichmentWriter{}
	enq := &fakeEnqueuer{}
	o := &Orchestrator{
		Videos:   videos,
		Mappings: testMappings(t),
		Patterns: testPatterns(t, "[{studio}] {title} ({year})"),
		Store:    writer,
		Deps: Deps{
			Resolver:         &fakeResolver{studios: map[int64]string{9: "Acme Studios"}},
			ManualSource:     &fakeManualSource{},
			Reviews:          &fakeReviewStore{},
			Queue:            enq,
			AutoApplyEnabled: true,
		},
	}

	res, err := o.ExtractVideo(context.Background(), 1)
	if err != nil {
		t.Fatalf("ExtractVideo: %v", err)
	}
	if !res.Matched {
		t.Fatalf("Matched = false, want true")
	}

	// Store wrote the raw parsed values into the filename shadow provider
	// (F48.2a), independent of how each field later routed.
	if len(writer.calls) != 1 {
		t.Fatalf("Store called %d times, want 1", len(writer.calls))
	}
	wantStored := map[string][]string{"studio": {"Acme Studios"}, "title": {"Big Movie"}, "release_date": {"2020"}}
	if !reflect.DeepEqual(writer.calls[0].Fields, wantStored) {
		t.Fatalf("stored fields = %#v, want %#v", writer.calls[0].Fields, wantStored)
	}

	outcomes := make(map[string]Outcome, len(res.Fields))
	for _, f := range res.Fields {
		outcomes[f.Field] = f.Outcome
	}
	// studio: exact entity match + high specificity + single-source agreement
	// clears the TierHigh gate (0.80) -> auto-applies.
	if outcomes["studio"] != OutcomeAutoApplied {
		t.Errorf("studio outcome = %v, want %v", outcomes["studio"], OutcomeAutoApplied)
	}
	var sawStudioEnqueue bool
	for _, call := range enq.calls {
		if call.VideoID != 1 {
			t.Errorf("enqueue for video %d, want 1", call.VideoID)
		}
		for _, fld := range call.Fields {
			if fld.Field == "studio" {
				sawStudioEnqueue = true
			}
		}
	}
	if !sawStudioEnqueue {
		t.Fatalf("enqueue calls = %#v, want a studio field enqueued for video 1", enq.calls)
	}
}

// TestOrchestrator_ExtractVideo_UsesFileBaseline proves fileTagValues feeds
// Process the video's existing file-tag value for source-agreement
// classification, not just the freshly extracted filename value. The title
// deliberately conflicts with the filename ("Something Else" vs. "Big
// Movie") so the candidate scores below TierMedium's auto-apply threshold and
// routes to review — the only path that surfaces the tag value Process saw.
func TestOrchestrator_ExtractVideo_UsesFileBaseline(t *testing.T) {
	videos := &fakeVideoLookup{
		videos: map[int64]*model.Video{
			1: {FilePath: "/media/Big Movie (2020).mkv", Title: "Something Else"},
		},
	}
	gotTagValues := map[string]string{}
	o := &Orchestrator{
		Videos:   videos,
		Mappings: testMappings(t),
		Patterns: testPatterns(t, "{title} ({year})"),
		Store:    &fakeEnrichmentWriter{},
		Deps: Deps{
			ManualSource: &fakeManualSource{},
			Reviews:      &recordingReviewStore{onUpsert: func(field, tagValue string) { gotTagValues[field] = tagValue }},
		},
	}
	res, err := o.ExtractVideo(context.Background(), 1)
	if err != nil {
		t.Fatalf("ExtractVideo: %v", err)
	}
	for _, f := range res.Fields {
		if f.Field == "title" && f.Outcome != OutcomeQueued {
			t.Fatalf("title outcome = %v, want %v (test assumes review-routing to observe the tag value)", f.Outcome, OutcomeQueued)
		}
	}
	if gotTagValues["title"] != "Something Else" {
		t.Fatalf("tag value seen by routing for title = %q, want the existing file:title baseline %q", gotTagValues["title"], "Something Else")
	}
}

// TestOrchestrator_ExtractVideo_VideoNotFound propagates the lookup error
// rather than panicking on a nil video.
func TestOrchestrator_ExtractVideo_VideoNotFound(t *testing.T) {
	o := &Orchestrator{
		Videos:   &fakeVideoLookup{videos: map[int64]*model.Video{}},
		Mappings: testMappings(t),
		Patterns: testPatterns(t, "{title} ({year})"),
	}
	if _, err := o.ExtractVideo(context.Background(), 404); !errors.Is(err, errNotFound) {
		t.Fatalf("ExtractVideo err = %v, want to wrap %v", err, errNotFound)
	}
}

// recordingReviewStore captures the field/tag value pair Process routed to
// review, so a test can assert what the orchestrator fed it without needing
// a full Route/Score assertion.
type recordingReviewStore struct {
	onUpsert func(field, tagValue string)
}

func (r *recordingReviewStore) UpsertExtractionReview(_ context.Context, _ int64, fieldKey, _, tagValue string, _ float64, _ int64) error {
	if r.onUpsert != nil {
		r.onUpsert(fieldKey, tagValue)
	}
	return nil
}
