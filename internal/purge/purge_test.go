package purge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

type fakeRepo struct {
	expired       []repo.TrashItem
	paths         map[int64]string
	missing       map[int64]bool // PurgePath returns ErrNotFound
	hardDeleted   []int64
	softDeleted   []int64
	runs          []model.JobRun
	expiredCalled bool
	held          map[int64]*repo.HeldOriginal
	maintBusy     map[int64]bool // a maintenance job is queued or running

	expiredFilms    []repo.TrashItem
	missingFilms    map[int64]bool // SoftDeleteFilm returns ErrNotFound
	softDeletedFilm []int64
	hardDeletedFilm []int64
	filmListErr     error
}

func (f *fakeRepo) GetHeldOriginal(_ context.Context, id int64) (*repo.HeldOriginal, error) {
	return f.held[id], nil
}

func (f *fakeRepo) GetVideoMaintenanceStatus(_ context.Context, id int64) (repo.MaintenanceStatus, error) {
	return repo.MaintenanceStatus{VideoWritebackStatus: repo.VideoWritebackStatus{Pending: f.maintBusy[id]}}, nil
}

func (f *fakeRepo) ExpiredSoftDeletedFilms(_ context.Context, _ time.Time) ([]repo.TrashItem, error) {
	return f.expiredFilms, f.filmListErr
}
func (f *fakeRepo) SoftDeleteFilm(_ context.Context, id int64) error {
	if f.missingFilms[id] {
		return repo.ErrNotFound
	}
	f.softDeletedFilm = append(f.softDeletedFilm, id)
	return nil
}
func (f *fakeRepo) HardDeleteFilm(_ context.Context, id int64) error {
	f.hardDeletedFilm = append(f.hardDeletedFilm, id)
	return nil
}

func (f *fakeRepo) ExpiredSoftDeleted(_ context.Context, _ time.Time) ([]repo.TrashItem, error) {
	f.expiredCalled = true
	return f.expired, nil
}
func (f *fakeRepo) PurgePath(_ context.Context, id int64) (string, error) {
	if f.missing[id] {
		return "", repo.ErrNotFound
	}
	return f.paths[id], nil
}
func (f *fakeRepo) SoftDelete(_ context.Context, id int64) error {
	f.softDeleted = append(f.softDeleted, id)
	return nil
}
func (f *fakeRepo) HardDelete(_ context.Context, id int64) error {
	f.hardDeleted = append(f.hardDeleted, id)
	return nil
}
func (f *fakeRepo) RecordJobRun(_ context.Context, run model.JobRun) error {
	f.runs = append(f.runs, run)
	return nil
}

func tempFile(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSweepPurgesExpiredAndRemovesFile(t *testing.T) {
	path := tempFile(t, "a.mkv")
	fr := &fakeRepo{
		expired: []repo.TrashItem{{ID: 1, FilePath: path}},
		paths:   map[int64]string{1: path},
	}
	p := New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil)
	p.Sweep(context.Background())

	if !slices.Contains(fr.hardDeleted, 1) {
		t.Errorf("row 1 not hard-deleted")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file not removed: %v", err)
	}
	if len(fr.runs) != 1 || fr.runs[0].Kind != model.JobKindPurge ||
		fr.runs[0].Removed != 1 || fr.runs[0].Errors != 0 || fr.runs[0].Status != model.JobStatusOK {
		t.Errorf("job run = %+v", fr.runs)
	}
}

func TestSweepGraceZeroIsNoOp(t *testing.T) {
	fr := &fakeRepo{expired: []repo.TrashItem{{ID: 1}}}
	p := New(fr, Config{Grace: 0, RemoveFiles: true}, nil)
	p.Sweep(context.Background())

	if fr.expiredCalled {
		t.Errorf("grace=0 should disable auto-purge (no expiry query)")
	}
	if len(fr.runs) != 0 || len(fr.hardDeleted) != 0 {
		t.Errorf("grace=0 should purge nothing: runs=%d hard=%d", len(fr.runs), len(fr.hardDeleted))
	}
}

func TestSweepMissingFileCountsAsSuccess(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "never-existed.mkv")
	fr := &fakeRepo{
		expired: []repo.TrashItem{{ID: 1, FilePath: gone}},
		paths:   map[int64]string{1: gone},
	}
	p := New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil)
	p.Sweep(context.Background())

	// A missing file is the desired end state — finish the row delete, count success.
	if !slices.Contains(fr.hardDeleted, 1) {
		t.Errorf("missing-file item should still hard-delete the row")
	}
	if len(fr.runs) != 1 || fr.runs[0].Removed != 1 || fr.runs[0].Errors != 0 {
		t.Errorf("missing file should count as purged success: %+v", fr.runs)
	}
}

func TestSweepRemovalFailureLeavesRow(t *testing.T) {
	// A non-empty directory makes os.Remove fail with a non-ErrNotExist error
	// portably (Windows + POSIX) — standing in for a permission/read-only failure.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "child"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	fr := &fakeRepo{
		expired: []repo.TrashItem{{ID: 1, FilePath: dir}},
		paths:   map[int64]string{1: dir},
	}
	p := New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil)
	p.Sweep(context.Background())

	// The row must NOT be hard-deleted — disk and DB never desync (F24.8).
	if len(fr.hardDeleted) != 0 {
		t.Errorf("row hard-deleted despite a failed unlink: %v", fr.hardDeleted)
	}
	if len(fr.runs) != 1 || fr.runs[0].Errors != 1 || fr.runs[0].Status != model.JobStatusErr {
		t.Errorf("removal failure should record an error run: %+v", fr.runs)
	}
}

func TestSweepRemoveFilesDisabledKeepsFile(t *testing.T) {
	path := tempFile(t, "a.mkv")
	fr := &fakeRepo{
		expired: []repo.TrashItem{{ID: 1, FilePath: path}},
		paths:   map[int64]string{1: path},
	}
	p := New(fr, Config{Grace: time.Hour, RemoveFiles: false}, nil)
	p.Sweep(context.Background())

	if !slices.Contains(fr.hardDeleted, 1) {
		t.Errorf("row should still be purged (DB-only) when RemoveFiles=false")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file should be left on disk when RemoveFiles=false: %v", err)
	}
}

func TestPurgeNow(t *testing.T) {
	path := tempFile(t, "a.mkv")
	fr := &fakeRepo{paths: map[int64]string{1: path}}
	p := New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil)

	if err := p.PurgeNow(context.Background(), 1); err != nil {
		t.Fatalf("purge now: %v", err)
	}
	if !slices.Contains(fr.softDeleted, 1) {
		t.Errorf("purge-now should mark soft-deleted before unlink (consistent Trash state)")
	}
	if !slices.Contains(fr.hardDeleted, 1) {
		t.Errorf("purge-now should hard-delete the row")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("purge-now should remove the file: %v", err)
	}
	if len(fr.runs) != 1 || fr.runs[0].Trigger != model.TriggerManual {
		t.Errorf("purge-now should record a manual run: %+v", fr.runs)
	}
}

// filmImageDir lays out {dir}/{filmID}/{imageID}.jpg the way entityimage.Store does.
func filmImageDir(t *testing.T, filmID string) (dir, filmDir string) {
	t.Helper()
	dir = t.TempDir()
	filmDir = filepath.Join(dir, filmID)
	if err := os.MkdirAll(filmDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filmDir, "1.jpg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir, filmDir
}

// F78: a trashed film past grace goes on the same pass as media — its image
// directory first, then the row — and counts in the same job run.
func TestSweepPurgesExpiredFilmAndItsImages(t *testing.T) {
	dir, filmDir := filmImageDir(t, "7")
	path := tempFile(t, "a.mkv")
	fr := &fakeRepo{
		expired:      []repo.TrashItem{{ID: 1, FilePath: path}},
		paths:        map[int64]string{1: path},
		expiredFilms: []repo.TrashItem{{ID: 7, Kind: repo.TrashKindFilm}},
	}
	// RemoveFiles=false: film images are app data, so the media-mount knob must not
	// keep them.
	p := New(fr, Config{Grace: time.Hour, RemoveFiles: false, FilmImageDir: dir}, nil)
	p.Sweep(context.Background())

	if !slices.Contains(fr.hardDeletedFilm, 7) {
		t.Errorf("film 7 not hard-deleted")
	}
	if _, err := os.Stat(filmDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("film image dir not removed: %v", err)
	}
	if len(fr.runs) != 1 || fr.runs[0].Removed != 2 {
		t.Errorf("one run counting the media item and the film, got %+v", fr.runs)
	}
}

func TestSweepFilmsOnlyStillRuns(t *testing.T) {
	fr := &fakeRepo{expiredFilms: []repo.TrashItem{{ID: 7, Kind: repo.TrashKindFilm}}}
	p := New(fr, Config{Grace: time.Hour}, nil)
	p.Sweep(context.Background())
	if !slices.Contains(fr.hardDeletedFilm, 7) || len(fr.runs) != 1 {
		t.Errorf("a film-only pass should purge and record: hard=%v runs=%d", fr.hardDeletedFilm, len(fr.runs))
	}
}

func TestPurgeFilmNow(t *testing.T) {
	dir, filmDir := filmImageDir(t, "7")
	fr := &fakeRepo{}
	p := New(fr, Config{Grace: time.Hour, FilmImageDir: dir}, nil)

	if err := p.PurgeFilmNow(context.Background(), 7); err != nil {
		t.Fatalf("purge film now: %v", err)
	}
	if !slices.Contains(fr.softDeletedFilm, 7) {
		t.Errorf("purge-now should move the film to Trash before removing images")
	}
	if !slices.Contains(fr.hardDeletedFilm, 7) {
		t.Errorf("purge-now should hard-delete the film")
	}
	if _, err := os.Stat(filmDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("purge-now should remove the film's images: %v", err)
	}
	if len(fr.runs) != 1 || fr.runs[0].Trigger != model.TriggerManual {
		t.Errorf("purge-now should record a manual run: %+v", fr.runs)
	}
}

func TestPurgeFilmNowNotFound(t *testing.T) {
	fr := &fakeRepo{missingFilms: map[int64]bool{7: true}}
	p := New(fr, Config{Grace: time.Hour}, nil)
	if err := p.PurgeFilmNow(context.Background(), 7); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("PurgeFilmNow(missing) = %v, want ErrNotFound", err)
	}
	if len(fr.hardDeletedFilm) != 0 || len(fr.runs) != 0 {
		t.Errorf("a missing film must purge and record nothing: hard=%v runs=%d", fr.hardDeletedFilm, len(fr.runs))
	}
}

// A failing film listing must not hold up the media purge.
func TestSweepFilmListErrorStillPurgesMedia(t *testing.T) {
	fr := &fakeRepo{
		expired:     []repo.TrashItem{{ID: 1}},
		filmListErr: errors.New("db busy"),
	}
	p := New(fr, Config{Grace: time.Hour}, nil)
	p.Sweep(context.Background())
	if !slices.Contains(fr.hardDeleted, 1) {
		t.Errorf("media item not purged when the film listing failed")
	}
}

func TestPurgeNowNotFound(t *testing.T) {
	fr := &fakeRepo{missing: map[int64]bool{1: true}}
	p := New(fr, Config{Grace: time.Hour, RemoveFiles: true}, nil)
	if err := p.PurgeNow(context.Background(), 1); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("PurgeNow(missing) = %v, want ErrNotFound", err)
	}
}
