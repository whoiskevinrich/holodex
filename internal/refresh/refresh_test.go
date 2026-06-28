package refresh_test

import (
	"context"
	"errors"
	"testing"

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
	upserts   int
	gotVideo  *model.Video
}

func (f *fakeStore) RefreshTarget(_ context.Context, _ int64) (string, error) {
	if f.targetErr != nil {
		return "", f.targetErr
	}
	return f.path, nil
}

func (f *fakeStore) UpsertVideo(_ context.Context, v *model.Video, _ []model.ExtraMetadata) (int64, error) {
	f.upserts++
	f.gotVideo = v
	return 7, nil
}

// Refresh resolves the target, re-extracts, and persists unconditionally — there
// is no (size, mtime) change-detection on this path (the forced re-extract that
// lets a refresh catch an mtime-preserving edit).
func TestRefreshForcesReExtractAndPersists(t *testing.T) {
	want := &model.Video{FilePath: "/m/clip.mp4", Title: "New Title"}
	ext := &fakeExt{v: want}
	store := &fakeStore{path: "/m/clip.mp4"}

	rep, err := refresh.NewService(ext, store).Refresh(context.Background(), 42)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if !rep.FileOK || rep.VideoID != 42 {
		t.Fatalf("report = %+v", rep)
	}
	if ext.calls != 1 || ext.gotPath != "/m/clip.mp4" {
		t.Fatalf("extractor not called with the resolved path: calls=%d path=%q", ext.calls, ext.gotPath)
	}
	if store.upserts != 1 || store.gotVideo != want {
		t.Fatalf("did not persist the extracted video unconditionally: upserts=%d", store.upserts)
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
			_, err := refresh.NewService(ext, store).Refresh(context.Background(), 1)
			if !errors.Is(err, tc.err) {
				t.Fatalf("want %v, got %v", tc.err, err)
			}
			if ext.calls != 0 || store.upserts != 0 {
				t.Fatalf("resolve error must short-circuit: extract=%d upsert=%d", ext.calls, store.upserts)
			}
		})
	}
}

// A file that can't be read fails the refresh before persistence — the row is
// never mutated on a file error (an unmounted volume must not corrupt the index).
func TestRefreshFileErrorDoesNotPersist(t *testing.T) {
	ext := &fakeExt{err: errors.New("stat: no such file")}
	store := &fakeStore{path: "/m/gone.mp4"}
	if _, err := refresh.NewService(ext, store).Refresh(context.Background(), 5); err == nil {
		t.Fatal("want an error when the file can't be read")
	}
	if store.upserts != 0 {
		t.Fatalf("must not persist when extract fails: upserts=%d", store.upserts)
	}
}
