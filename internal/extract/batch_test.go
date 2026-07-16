package extract

import (
	"context"
	"sync"
	"testing"
	"time"

	"holodex/internal/model"
)

// countingResolver wraps a Resolver, counting EntityNames calls per entity
// type — the probe for withCachedResolver's caching behavior.
type countingResolver struct {
	inner Resolver
	mu    sync.Mutex
	calls map[string]int
}

func (c *countingResolver) ExactEntityMatch(ctx context.Context, entityType, name string) (int64, bool, error) {
	return c.inner.ExactEntityMatch(ctx, entityType, name)
}

func (c *countingResolver) EntityNames(ctx context.Context, entityType string) (map[int64]string, error) {
	c.mu.Lock()
	c.calls[entityType]++
	c.mu.Unlock()
	return c.inner.EntityNames(ctx, entityType)
}

// TestOrchestrator_WithCachedResolver_CachesEntityNames proves the batch-run
// fix: without the cache, three videos each carrying a "studio" field would
// each trigger their own full EntityNames table read; withCachedResolver
// amortizes it to one read for the copy's lifetime.
func TestOrchestrator_WithCachedResolver_CachesEntityNames(t *testing.T) {
	counting := &countingResolver{
		inner: &fakeResolver{studios: map[int64]string{9: "Acme Studios"}},
		calls: map[string]int{},
	}
	videos := &fakeVideoLookup{videos: map[int64]*model.Video{
		1: {FilePath: "/media/[Acme Studios] One (2020).mkv"},
		2: {FilePath: "/media/[Acme Studios] Two (2021).mkv"},
		3: {FilePath: "/media/[Acme Studios] Three (2022).mkv"},
	}}
	o := &Orchestrator{
		Videos:   videos,
		Mappings: testMappings(t),
		Patterns: testPatterns(t, "[{studio}] {title} ({year})"),
		Store:    &fakeEnrichmentWriter{},
		Deps: Deps{
			Resolver:     counting,
			ManualSource: &fakeManualSource{},
			Reviews:      &fakeReviewStore{},
		},
	}

	cached := o.withCachedResolver()
	for id := range videos.videos {
		if _, err := cached.ExtractVideo(context.Background(), id); err != nil {
			t.Fatalf("ExtractVideo(%d): %v", id, err)
		}
	}

	if counting.calls["studio"] != 1 {
		t.Fatalf("EntityNames(studio) called %d times across 3 videos, want 1 (cached)", counting.calls["studio"])
	}
}

// TestOrchestrator_WithCachedResolver_NotSharedAcrossCopies proves each
// withCachedResolver call gets its own cache, so a stale batch run's cache
// can never leak into a later one (or into a concurrent on-demand call on
// the shared, uncached Orchestrator).
func TestOrchestrator_WithCachedResolver_NotSharedAcrossCopies(t *testing.T) {
	counting := &countingResolver{
		inner: &fakeResolver{studios: map[int64]string{9: "Acme Studios"}},
		calls: map[string]int{},
	}
	videos := &fakeVideoLookup{videos: map[int64]*model.Video{
		1: {FilePath: "/media/[Acme Studios] One (2020).mkv"},
	}}
	o := &Orchestrator{
		Videos:   videos,
		Mappings: testMappings(t),
		Patterns: testPatterns(t, "[{studio}] {title} ({year})"),
		Store:    &fakeEnrichmentWriter{},
		Deps: Deps{
			Resolver:     counting,
			ManualSource: &fakeManualSource{},
			Reviews:      &fakeReviewStore{},
		},
	}

	if _, err := o.withCachedResolver().ExtractVideo(context.Background(), 1); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := o.withCachedResolver().ExtractVideo(context.Background(), 1); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if counting.calls["studio"] != 2 {
		t.Fatalf("EntityNames(studio) called %d times across 2 independent batch runs, want 2 (one per run, not shared)", counting.calls["studio"])
	}
}

// fakeVideoLister enumerates a fixed id set (F48.5b's AllActiveVideoIDs
// slice) without a real repo.
type fakeVideoLister struct{ ids []int64 }

func (f *fakeVideoLister) AllActiveVideoIDs(context.Context) ([]int64, error) { return f.ids, nil }

// recordingJobRecorder captures RecordJobRun calls and signals done, so a
// test can wait for BatchRunner's background goroutine to finish without
// polling.
type recordingJobRecorder struct {
	done chan model.JobRun
}

func newRecordingJobRecorder() *recordingJobRecorder {
	return &recordingJobRecorder{done: make(chan model.JobRun, 1)}
}

func (r *recordingJobRecorder) RecordJobRun(_ context.Context, run model.JobRun) error {
	r.done <- run
	return nil
}

// TestBatchRunner_TriggerAll_ProcessesEveryVideoAndRecords is the F48.5b
// end-to-end shape: a triggered batch pass visits every active video and
// records one System Activity job run (kind=extraction) when done.
func TestBatchRunner_TriggerAll_ProcessesEveryVideoAndRecords(t *testing.T) {
	videos := &fakeVideoLookup{videos: map[int64]*model.Video{
		1: {FilePath: "/media/One (2020).mkv"},
		2: {FilePath: "/media/unrelated_name.mkv"}, // no pattern match
	}}
	o := &Orchestrator{
		Videos:   videos,
		Mappings: testMappings(t),
		Patterns: testPatterns(t, "{title} ({year})"),
		Store:    &fakeEnrichmentWriter{},
		Deps:     Deps{ManualSource: &fakeManualSource{}, Reviews: &fakeReviewStore{}},
	}
	rec := newRecordingJobRecorder()
	b := &BatchRunner{Orchestrator: o, Videos: &fakeVideoLister{ids: []int64{1, 2}}, Recorder: rec}

	if started := b.TriggerAll(); !started {
		t.Fatal("TriggerAll() = false, want true (no pass already running)")
	}

	select {
	case run := <-rec.done:
		if run.Kind != model.JobKindExtraction {
			t.Errorf("Kind = %q, want %q", run.Kind, model.JobKindExtraction)
		}
		if run.Seen != 2 || run.Updated != 1 {
			t.Errorf("Seen=%d Updated=%d, want Seen=2 (both videos) Updated=1 (only the matching one)", run.Seen, run.Updated)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the batch run to record")
	}
}

// TestBatchRunner_TriggerAll_DedupsConcurrentTriggers mirrors
// scanner.TriggerRescan's contract: a second trigger while one is already
// running is satisfied by the in-flight pass, not started again.
func TestBatchRunner_TriggerAll_DedupsConcurrentTriggers(t *testing.T) {
	b := &BatchRunner{}
	b.mu.Lock() // simulate a pass already running
	defer b.mu.Unlock()

	if started := b.TriggerAll(); started {
		t.Fatal("TriggerAll() = true while a pass is already running, want false")
	}
}
