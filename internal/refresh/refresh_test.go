package refresh_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"holodex/internal/enrich"
	"holodex/internal/model"
	"holodex/internal/refresh"
	"holodex/internal/repo"
)

type fakeExt struct {
	v       *model.Video
	extra   []model.ExtraMetadata
	err     error
	calls   int
	gotPath string
}

func (f *fakeExt) BuildVideoFromFile(_ context.Context, path string) (*model.Video, []model.ExtraMetadata, error) {
	f.calls++
	f.gotPath = path
	if f.err != nil {
		return nil, nil, f.err
	}
	return f.v, f.extra, nil
}

type fakeStore struct {
	path      string
	targetErr error
	old       *model.Video
	oldExtra  []model.ExtraMetadata
	upserts   int
	gotVideo  *model.Video
	recorded  []model.JobRun
}

func (f *fakeStore) RefreshTarget(_ context.Context, _ int64) (string, error) {
	if f.targetErr != nil {
		return "", f.targetErr
	}
	return f.path, nil
}

func (f *fakeStore) GetVideo(_ context.Context, _ int64) (*model.Video, []model.ExtraMetadata, error) {
	if f.old == nil {
		return nil, nil, repo.ErrNotFound
	}
	return f.old, f.oldExtra, nil
}

func (f *fakeStore) UpsertVideo(_ context.Context, v *model.Video, _ []model.ExtraMetadata) (int64, error) {
	f.upserts++
	f.gotVideo = v
	return 7, nil
}

func (f *fakeStore) RecordJobRun(_ context.Context, run model.JobRun) error {
	f.recorded = append(f.recorded, run)
	return nil
}

type fakeEnricher struct {
	matches    []enrich.Match
	matchesErr error
	reErr      map[string]error // provider -> error
	reCalls    []string
}

func (f *fakeEnricher) ProviderMatches(_ context.Context, _ string, _ int64) ([]enrich.Match, error) {
	return f.matches, f.matchesErr
}

func (f *fakeEnricher) ReEnrich(_ context.Context, _ string, _ int64, provider, _ string) ([]model.EnrichedField, error) {
	f.reCalls = append(f.reCalls, provider)
	if f.reErr != nil {
		if err := f.reErr[provider]; err != nil {
			return nil, err
		}
	}
	return []model.EnrichedField{{Canonical: "title"}}, nil
}

func sourceByName(rep refresh.Report, name string) (refresh.SourceResult, bool) {
	for _, sr := range rep.Sources {
		if sr.Source == name {
			return sr, true
		}
	}
	return refresh.SourceResult{}, false
}

// Refresh resolves the target, re-extracts, and persists unconditionally — there
// is no (size, mtime) change-detection on this path (the forced re-extract that
// lets a refresh catch an mtime-preserving edit). The file diff drives Changed.
func TestRefreshForcesReExtractAndPersists(t *testing.T) {
	newV := &model.Video{FilePath: "/m/clip.mp4", Title: "New Title"}
	ext := &fakeExt{v: newV}
	store := &fakeStore{path: "/m/clip.mp4", old: &model.Video{FilePath: "/m/clip.mp4", Title: "Old Title"}}

	rep, err := refresh.NewService(ext, store, nil, nil).Refresh(context.Background(), 42)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rep.VideoID != 42 || !rep.Changed {
		t.Fatalf("report = %+v", rep)
	}
	if ext.calls != 1 || ext.gotPath != "/m/clip.mp4" {
		t.Fatalf("extractor not called with the resolved path: calls=%d path=%q", ext.calls, ext.gotPath)
	}
	if store.upserts != 1 || store.gotVideo != newV {
		t.Fatalf("did not persist the extracted video unconditionally: upserts=%d", store.upserts)
	}
	if file, ok := sourceByName(rep, "file"); !ok || !file.OK || !file.Changed {
		t.Fatalf("file source = %+v (ok=%v)", file, ok)
	}
}

// An unchanged file re-extracts and persists, but reports the file source as not
// changed (the "already in sync" signal).
func TestRefreshUnchangedFileReportsNoChange(t *testing.T) {
	same := func() *model.Video { return &model.Video{FilePath: "/m/a.mp4", Title: "T", Width: 1920} }
	ext := &fakeExt{v: same()}
	store := &fakeStore{path: "/m/a.mp4", old: same()}

	rep, err := refresh.NewService(ext, store, nil, nil).Refresh(context.Background(), 1)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if store.upserts != 1 {
		t.Fatalf("forced re-extract still persists: upserts=%d", store.upserts)
	}
	if rep.Changed {
		t.Fatalf("unchanged file should report no change: %+v", rep)
	}
}

// A missing or soft-deleted target short-circuits before any file read or write,
// and the repo sentinel propagates unwrapped so the handler can map 404 vs 409.
func TestRefreshResolveErrorsDoNotExtractOrPersist(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{"not found", repo.ErrNotFound},
		{"soft-deleted", repo.ErrDeleted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ext := &fakeExt{}
			store := &fakeStore{targetErr: tc.err}
			_, err := refresh.NewService(ext, store, nil, nil).Refresh(context.Background(), 1)
			if !errors.Is(err, tc.err) {
				t.Fatalf("want %v, got %v", tc.err, err)
			}
			if ext.calls != 0 || store.upserts != 0 {
				t.Fatalf("resolve error must short-circuit: extract=%d upsert=%d", ext.calls, store.upserts)
			}
			if len(store.recorded) != 0 {
				t.Fatalf("a rejected request (404/409) is not a recorded run: %+v", store.recorded)
			}
		})
	}
}

// Every refresh records exactly one combined job_runs row (kind=refresh), with a
// partial-failure marked as an error and the item referenced as #id (no path).
func TestRefreshRecordsOneCombinedJobRun(t *testing.T) {
	ext := &fakeExt{v: &model.Video{FilePath: "/m/r.mp4", Title: "New"}}
	store := &fakeStore{path: "/m/r.mp4", old: &model.Video{FilePath: "/m/r.mp4", Title: "Old"}}
	enr := &fakeEnricher{
		matches: []enrich.Match{{Provider: "tmdb", ExternalID: "1"}},
		reErr:   map[string]error{"tmdb": errors.New("down")},
	}

	if _, err := refresh.NewService(ext, store, enr, nil).Refresh(context.Background(), 42); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(store.recorded) != 1 {
		t.Fatalf("want exactly one job row, got %d", len(store.recorded))
	}
	run := store.recorded[0]
	if run.Kind != model.JobKindRefresh || run.Trigger != model.TriggerManual {
		t.Fatalf("kind/trigger = %q/%q", run.Kind, run.Trigger)
	}
	if run.Status != model.JobStatusErr || run.Errors != 1 {
		t.Fatalf("a failed provider should mark the row errored: %+v", run)
	}
	if !strings.Contains(run.Detail, "#42") || strings.Contains(run.Detail, "/m/") {
		t.Fatalf("detail must reference #id and carry no path: %q", run.Detail)
	}
	// Attribution (ADR-071) — the same video the detail names, as columns, so a
	// failed refresh is findable by entity and not only by reading the text.
	if run.EntityType != model.EnrichEntityVideo || run.EntityID != 42 {
		t.Errorf("attribution = %q/#%d, want video/#42", run.EntityType, run.EntityID)
	}
}

// A file that can't be read fails the refresh before persistence — the row is
// never mutated on a file error (an unmounted volume must not corrupt the index).
func TestRefreshFileErrorDoesNotPersist(t *testing.T) {
	ext := &fakeExt{err: errors.New("stat: no such file")}
	store := &fakeStore{path: "/m/gone.mp4"}
	if _, err := refresh.NewService(ext, store, nil, nil).Refresh(context.Background(), 5); err == nil {
		t.Fatal("want an error when the file can't be read")
	}
	if store.upserts != 0 {
		t.Fatalf("must not persist when extract fails: upserts=%d", store.upserts)
	}
}

// A successful refresh re-derives the item's studio links via the relink hook
// (F38, ADR-053), and a relink failure is swallowed — the file + enrichment writes
// already committed, so the derived link must never fail the refresh.
func TestRefreshRelinksStudios(t *testing.T) {
	ext := &fakeExt{v: &model.Video{FilePath: "/m/s.mp4", Title: "New"}}
	store := &fakeStore{path: "/m/s.mp4", old: &model.Video{FilePath: "/m/s.mp4", Title: "Old"}}
	svc := refresh.NewService(ext, store, nil, nil)

	var relinked []int64
	svc.SetRelinker(func(_ context.Context, id int64) error {
		relinked = append(relinked, id)
		return errors.New("relink boom") // must not fail the refresh
	})

	if _, err := svc.Refresh(context.Background(), 42); err != nil {
		t.Fatalf("a relink error must not fail the refresh: %v", err)
	}
	if len(relinked) != 1 || relinked[0] != 42 {
		t.Fatalf("relink should fire once with the refreshed id, got %v", relinked)
	}
}

// A rejected refresh (missing/soft-deleted target) short-circuits before the relink
// hook — no derived work for an item that was never touched.
func TestRefreshResolveErrorSkipsRelink(t *testing.T) {
	svc := refresh.NewService(&fakeExt{}, &fakeStore{targetErr: repo.ErrNotFound}, nil, nil)
	called := false
	svc.SetRelinker(func(_ context.Context, _ int64) error { called = true; return nil })
	if _, err := svc.Refresh(context.Background(), 1); !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if called {
		t.Fatal("relink must not fire when the target resolve fails")
	}
}

// Refresh re-enriches every linked provider after the file commit, one source
// result each, reusing the persisted match (no picker).
func TestRefreshReEnrichesLinkedProviders(t *testing.T) {
	ext := &fakeExt{v: &model.Video{FilePath: "/m/x.mp4"}}
	store := &fakeStore{path: "/m/x.mp4"}
	enr := &fakeEnricher{matches: []enrich.Match{{Provider: "tmdb", ExternalID: "tmdb:1"}, {Provider: "imdb", ExternalID: "tt9"}}}

	rep, err := refresh.NewService(ext, store, enr, nil).Refresh(context.Background(), 3)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if store.upserts != 1 {
		t.Fatalf("file must commit before providers: upserts=%d", store.upserts)
	}
	if len(enr.reCalls) != 2 {
		t.Fatalf("expected both providers re-enriched, got %v", enr.reCalls)
	}
	for _, p := range []string{"tmdb", "imdb"} {
		if sr, ok := sourceByName(rep, p); !ok || !sr.OK {
			t.Fatalf("provider %q source missing/not ok: %+v", p, sr)
		}
	}
}

// A provider failure is isolated to its own source result; the file commit and
// the other providers are unaffected and the refresh still succeeds.
func TestRefreshProviderFailureIsolated(t *testing.T) {
	ext := &fakeExt{v: &model.Video{FilePath: "/m/y.mp4"}}
	store := &fakeStore{path: "/m/y.mp4"}
	enr := &fakeEnricher{
		matches: []enrich.Match{{Provider: "tmdb", ExternalID: "1"}, {Provider: "imdb", ExternalID: "2"}},
		reErr:   map[string]error{"tmdb": errors.New("502 bad gateway")},
	}

	rep, err := refresh.NewService(ext, store, enr, nil).Refresh(context.Background(), 9)
	if err != nil {
		t.Fatalf("a provider failure must not fail the refresh: %v", err)
	}
	if store.upserts != 1 {
		t.Fatalf("file must still commit on provider failure: upserts=%d", store.upserts)
	}
	tmdb, _ := sourceByName(rep, "tmdb")
	if tmdb.OK || tmdb.Error == "" {
		t.Fatalf("failed provider should report not-ok with an error: %+v", tmdb)
	}
	if imdb, _ := sourceByName(rep, "imdb"); !imdb.OK {
		t.Fatalf("healthy provider should be unaffected: %+v", imdb)
	}
}

// No persisted match → the provider step is a clean no-op (no ReEnrich calls, no
// error), leaving only the file source.
func TestRefreshNoMatchesIsFileOnly(t *testing.T) {
	enr := &fakeEnricher{} // no matches
	ext := &fakeExt{v: &model.Video{FilePath: "/m/z.mp4"}}
	store := &fakeStore{path: "/m/z.mp4"}

	rep, err := refresh.NewService(ext, store, enr, nil).Refresh(context.Background(), 2)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(enr.reCalls) != 0 {
		t.Fatalf("no match should not call ReEnrich: %v", enr.reCalls)
	}
	if len(rep.Sources) != 1 || rep.Sources[0].Source != "file" {
		t.Fatalf("expected only the file source: %+v", rep.Sources)
	}
}

// ReExtract is the file-only half of a refresh used by the post-writeback hook
// (HOLODEX-196 #4): it re-reads the file into the file layer and relinks
// studios so a just-written People/Studio entity is created, WITHOUT re-pulling
// providers or recording an activity row.
func TestReExtract_FileOnlyAndRelinks(t *testing.T) {
	newV := &model.Video{FilePath: "/m/clip.mp4", Title: "New Title"}
	ext := &fakeExt{v: newV}
	store := &fakeStore{path: "/m/clip.mp4", old: &model.Video{FilePath: "/m/clip.mp4", Title: "Old Title"}}
	enr := &fakeEnricher{matches: []enrich.Match{{Provider: "tmdb"}}}

	svc := refresh.NewService(ext, store, enr, nil)
	var relinked int64
	svc.SetRelinker(func(_ context.Context, id int64) error { relinked = id; return nil })

	if err := svc.ReExtract(context.Background(), 42); err != nil {
		t.Fatalf("ReExtract: %v", err)
	}
	if ext.calls != 1 || store.upserts != 1 || store.gotVideo != newV {
		t.Fatalf("did not re-extract+persist: calls=%d upserts=%d", ext.calls, store.upserts)
	}
	if relinked != 42 {
		t.Fatalf("relinker not called with the id: got %d", relinked)
	}
	if len(enr.reCalls) != 0 {
		t.Fatalf("ReExtract must not re-enrich, got provider calls %v", enr.reCalls)
	}
	if len(store.recorded) != 0 {
		t.Fatalf("ReExtract must not record an activity row, got %+v", store.recorded)
	}
}

// A missing/soft-deleted target propagates unwrapped, before any write.
func TestReExtract_TargetErrorPropagates(t *testing.T) {
	store := &fakeStore{targetErr: repo.ErrNotFound}
	err := refresh.NewService(&fakeExt{}, store, nil, nil).ReExtract(context.Background(), 9)
	if !errors.Is(err, repo.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if store.upserts != 0 {
		t.Fatalf("must not write on a rejected target: upserts=%d", store.upserts)
	}
}
