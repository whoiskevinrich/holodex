// Package scanner walks MEDIA_PATH, extracts metadata, and reconciles the index.
//
// Design (ADR-011, ADR-018):
//   - Walk recursively; if FollowSymlinks, resolve each entry to its canonical
//     path (filepath.EvalSymlinks) and dedup via a visited-set — this also makes
//     symlink-loop detection automatic.
//   - For each file compare (size, mtime) against the stored row:
//     no row            -> extract + insert
//     unchanged         -> skip (no extraction subprocess)
//     size/mtime differ -> re-extract + update
//   - Files modified within MinAge are skipped this cycle (mid-copy guard).
//   - Active rows not seen during the walk are marked active=false.
package scanner

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"holodex/internal/metadata"
	"holodex/internal/model"
	"holodex/internal/repo"
)

type Config struct {
	MediaPath      string
	FollowSymlinks bool
	MaxDepth       int
	MinAge         time.Duration
	Workers        int
}

// Repository is the subset of the data layer the scanner needs (ADR-018).
type Repository interface {
	StatByPath(ctx context.Context, path string) (stat repo.VideoStat, ok bool, err error)
	UpsertVideo(ctx context.Context, v *model.Video, extra []model.ExtraMetadata) (int64, error)
	DeactivateExcept(ctx context.Context, seenIDs []int64) (int64, error)
}

// Extractor reads embedded metadata for one file.
type Extractor interface {
	Extract(ctx context.Context, path string) (metadata.Extracted, error)
}

// Thumbnailer is the thumbnail pipeline seam (ADR-009). After indexing a file the
// scanner extracts embedded cover art (Tier 1) or, failing that, enqueues the
// file for background frame generation (Tier 2). Optional — nil disables both.
type Thumbnailer interface {
	ExtractEmbedded(ctx context.Context, id int64, path string) (bool, error)
	Enqueue(id int64)
}

// Metrics is the observability seam (F13.2): each completed pass reports its
// duration and how many files it (re)indexed. Optional — nil disables it.
type Metrics interface {
	ObserveScan(d time.Duration, indexed int)
}

type Scanner struct {
	cfg     Config
	log     *slog.Logger
	repo    Repository
	ext     Extractor
	thumbs  Thumbnailer
	metrics Metrics
	scanMu  sync.Mutex      // ensures only one reconciliation pass runs at a time
	baseCtx context.Context // server-lifetime ctx for manual rescans (F13.3)
}

func New(cfg Config, log *slog.Logger, repo Repository, ext Extractor) *Scanner {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	return &Scanner{cfg: cfg, log: log, repo: repo, ext: ext}
}

// SetThumbnailer wires the thumbnail pipeline. Called once at startup before Run.
func (s *Scanner) SetThumbnailer(t Thumbnailer) { s.thumbs = t }

// SetBaseContext supplies the server-lifetime context used by manually-triggered
// rescans (TriggerRescan), so an admin rescan is cancelled on shutdown rather than
// when the HTTP request returns. Call once at startup before Run; without it
// manual rescans fall back to context.Background().
func (s *Scanner) SetBaseContext(ctx context.Context) { s.baseCtx = ctx }

// SetMetrics wires scan-duration / indexed-files instrumentation (F13.2). Called
// once at startup before Run; nil leaves the scanner uninstrumented.
func (s *Scanner) SetMetrics(m Metrics) { s.metrics = m }

var mediaExts = map[string]struct{}{".mp4": {}, ".mkv": {}}

func isMedia(name string) bool {
	_, ok := mediaExts[strings.ToLower(filepath.Ext(name))]
	return ok
}

// stats accumulates a scan-cycle summary (ADR-019).
type stats struct {
	mu                                    sync.Mutex
	seen, added, updated, skipped, errors int
	seenIDs                               []int64
}

func (s *stats) recordSeen(id int64) {
	s.mu.Lock()
	s.seenIDs = append(s.seenIDs, id)
	s.mu.Unlock()
}

func (s *stats) incSeen()    { s.mu.Lock(); s.seen++; s.mu.Unlock() }
func (s *stats) incAdded()   { s.mu.Lock(); s.added++; s.mu.Unlock() }
func (s *stats) incUpdated() { s.mu.Lock(); s.updated++; s.mu.Unlock() }
func (s *stats) incSkipped() { s.mu.Lock(); s.skipped++; s.mu.Unlock() }
func (s *stats) incErrors()  { s.mu.Lock(); s.errors++; s.mu.Unlock() }

// ScanOnce performs a single reconciliation pass over MediaPath. Concurrent
// callers (the periodic ticker and the fs-watcher) are serialized so two passes
// never race on the seen-set / removal reconciliation.
func (s *Scanner) ScanOnce(ctx context.Context) error {
	if s.cfg.MediaPath == "" {
		s.log.Warn("MEDIA_PATH not set; skipping scan")
		return nil
	}
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	return s.scanLocked(ctx)
}

// TriggerRescan starts a full reconciliation pass in the background (F13.3),
// returning true if it started one and false if a scan was already running — in
// which case the in-flight pass already satisfies the request. It is the seam
// behind POST /api/v1/admin/rescan: the handler returns 202 immediately while the
// scan proceeds. Concurrent triggers are deduplicated by the same scanMu that
// serializes the periodic and watch-driven passes, so spamming the endpoint can
// never stack up passes.
func (s *Scanner) TriggerRescan() bool {
	if s.cfg.MediaPath == "" {
		return false
	}
	if !s.scanMu.TryLock() {
		return false
	}
	ctx := s.baseCtx
	if ctx == nil {
		ctx = context.Background()
	}
	go func() {
		defer s.scanMu.Unlock()
		if err := s.scanLocked(ctx); err != nil {
			s.log.Error("admin rescan failed", "err", err)
		}
	}()
	return true
}

// scanLocked runs one reconciliation pass. The caller must hold scanMu.
func (s *Scanner) scanLocked(ctx context.Context) error {
	start := time.Now()
	st := &stats{}

	jobs := make(chan string, s.cfg.Workers*2)
	var wg sync.WaitGroup
	for i := 0; i < s.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				s.index(ctx, path, st)
			}
		}()
	}

	walkErr := s.walk(ctx, jobs, st)
	close(jobs)
	wg.Wait()

	removed, err := s.repo.DeactivateExcept(ctx, st.seenIDs)
	if err != nil {
		s.log.Error("reconcile removed files failed", "err", err)
	}

	dur := time.Since(start)
	if s.metrics != nil {
		s.metrics.ObserveScan(dur, st.added+st.updated)
	}
	s.log.Info("scan complete",
		"media_path", s.cfg.MediaPath,
		"seen", st.seen, "added", st.added, "updated", st.updated,
		"removed", removed, "skipped", st.skipped, "errors", st.errors,
		"duration_ms", dur.Milliseconds())
	return walkErr
}

// index handles one media file: change-detect, extract, upsert.
func (s *Scanner) index(ctx context.Context, path string, st *stats) {
	info, err := os.Stat(path)
	if err != nil {
		st.incErrors()
		s.log.Warn("stat failed", "path", path, "err", err)
		return
	}
	mtime := info.ModTime().UTC().Truncate(time.Second)

	// Mid-copy guard: skip files that changed within MinAge (F1.9).
	if s.cfg.MinAge > 0 && time.Since(info.ModTime()) < s.cfg.MinAge {
		st.incSkipped()
		return
	}

	prev, ok, err := s.repo.StatByPath(ctx, path)
	if err != nil {
		st.incErrors()
		s.log.Warn("stat lookup failed", "path", path, "err", err)
		return
	}
	if ok && prev.Size == info.Size() && prev.Mtime.Equal(mtime) {
		st.incSkipped()
		st.recordSeen(prev.ID) // unchanged but still present
		return
	}

	ex, err := s.ext.Extract(ctx, path)
	if err != nil {
		st.incErrors()
		s.log.Warn("extract failed; skipping", "path", path, "err", err)
		return
	}

	v := buildVideo(path, info, mtime, ex)
	id, err := s.repo.UpsertVideo(ctx, v, ex.Extra)
	if err != nil {
		st.incErrors()
		s.log.Warn("upsert failed", "path", path, "err", err)
		return
	}
	st.recordSeen(id)
	if ok {
		st.incUpdated()
	} else {
		st.incAdded()
	}
	s.handleThumbnail(ctx, id, path, ex.HasCoverArt)
}

// handleThumbnail runs Tier 1 (embedded cover art) for a freshly indexed file,
// falling back to enqueuing it for Tier 2 background generation. Best-effort: a
// failure here never affects indexing.
func (s *Scanner) handleThumbnail(ctx context.Context, id int64, path string, hasCoverArt bool) {
	if s.thumbs == nil {
		return
	}
	if hasCoverArt {
		if ok, err := s.thumbs.ExtractEmbedded(ctx, id, path); err != nil {
			s.log.Warn("cover art extract failed; will generate", "path", path, "err", err)
		} else if ok {
			return
		}
	}
	s.thumbs.Enqueue(id)
}

func buildVideo(path string, info os.FileInfo, mtime time.Time, ex metadata.Extracted) *model.Video {
	v := &model.Video{
		FilePath:    path,
		FileSize:    info.Size(),
		Title:       ex.Title,
		Duration:    ex.DurationSec,
		Width:       ex.Width,
		Height:      ex.Height,
		VideoCodec:  ex.VideoCodec,
		AudioCodec:  ex.AudioCodec,
		BitrateKbps: ex.BitrateKbps,
		Container:   ex.Container,
		RecordedAt:  ex.RecordedAt,
		FileMtime:   mtime,
	}
	// Fall back to file mtime when no embedded date (F2.7).
	if v.RecordedAt == nil {
		v.RecordedAt = &mtime
	}
	// Title falls back to the filename stem so untagged files remain findable.
	if strings.TrimSpace(v.Title) == "" {
		v.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	for _, name := range ex.People {
		v.People = append(v.People, model.Person{Name: name})
	}
	for _, name := range ex.Tags {
		v.Tags = append(v.Tags, model.Tag{Name: name})
	}
	return v
}

// walk traverses MediaPath, sending canonical media-file paths to jobs. It
// follows symlinks (when configured) with loop detection via a visited-set and
// honors MaxDepth (ADR-011).
func (s *Scanner) walk(ctx context.Context, jobs chan<- string, st *stats) error {
	visitedDirs := make(map[string]struct{})
	visitedFiles := make(map[string]struct{})
	return s.walkDir(ctx, s.cfg.MediaPath, 0, visitedDirs, visitedFiles, jobs, st)
}

func (s *Scanner) walkDir(ctx context.Context, dir string, depth int, visitedDirs, visitedFiles map[string]struct{}, jobs chan<- string, st *stats) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if s.cfg.MaxDepth > 0 && depth > s.cfg.MaxDepth {
		return nil
	}
	canonDir, err := s.canonical(dir)
	if err != nil {
		s.log.Warn("resolve dir failed", "path", dir, "err", err)
		return nil
	}
	if _, seen := visitedDirs[canonDir]; seen {
		return nil // symlink loop or already-visited real dir
	}
	visitedDirs[canonDir] = struct{}{}

	entries, err := os.ReadDir(dir)
	if err != nil {
		s.log.Warn("read dir failed", "path", dir, "err", err)
		return nil
	}
	for _, e := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		full := filepath.Join(dir, e.Name())
		info, err := s.entryInfo(full, e)
		if err != nil {
			continue
		}
		switch {
		case info.IsDir():
			if err := s.walkDir(ctx, full, depth+1, visitedDirs, visitedFiles, jobs, st); err != nil {
				return err
			}
		case info.Mode().IsRegular() && isMedia(e.Name()):
			canon, err := s.canonical(full)
			if err != nil {
				continue
			}
			if _, dup := visitedFiles[canon]; dup {
				continue // same real file reached via two paths (ADR-011 dedup)
			}
			visitedFiles[canon] = struct{}{}
			st.incSeen()
			select {
			case jobs <- canon:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	return nil
}

// entryInfo resolves an entry's info, following symlinks only when configured.
func (s *Scanner) entryInfo(full string, e os.DirEntry) (os.FileInfo, error) {
	if e.Type()&os.ModeSymlink == 0 {
		return e.Info()
	}
	if !s.cfg.FollowSymlinks {
		return nil, os.ErrInvalid // skip symlinks when disabled (F1.7)
	}
	return os.Stat(full) // follow to target
}

// canonical resolves symlinks to a real path when following is enabled; with
// following disabled it returns a cleaned absolute path.
func (s *Scanner) canonical(path string) (string, error) {
	if !s.cfg.FollowSymlinks {
		return filepath.Clean(path), nil
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// Run does the initial scan, starts the OS-level fs-watcher (F1.5, primary), and
// runs the periodic scan loop (the reliable fallback if events are missed).
func (s *Scanner) Run(ctx context.Context, interval time.Duration) {
	if err := s.ScanOnce(ctx); err != nil {
		s.log.Error("initial scan failed", "err", err)
	}
	go s.runWatcher(ctx)

	if interval <= 0 {
		interval = 300 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.ScanOnce(ctx); err != nil {
				s.log.Error("scan failed", "err", err)
			}
		}
	}
}

// runWatcher watches MediaPath (and subdirs) for changes and, on relevant events,
// nudges a debounced full ScanOnce (F1.5) — the watcher carries no event detail
// into the scan; it just makes the next reconciliation happen sooner than the
// periodic tick. fsnotify is per-directory and non-recursive, so we watch every
// directory and add new ones as they appear. Best-effort: any failure falls back
// to the periodic scan.
func (s *Scanner) runWatcher(ctx context.Context) {
	if s.cfg.MediaPath == "" {
		return
	}
	w, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Warn("fs-watch unavailable; periodic scan only", "err", err)
		return
	}
	defer w.Close()
	s.addWatchesRecursive(w, s.cfg.MediaPath)
	s.log.Info("fs-watch active", "media_path", s.cfg.MediaPath)

	// Debounce bursts of events (e.g. a large copy) into a single rescan.
	var debounce *time.Timer
	trigger := func() {
		if debounce != nil {
			debounce.Stop()
		}
		debounce = time.AfterFunc(2*time.Second, func() {
			if err := s.ScanOnce(ctx); err != nil {
				s.log.Error("watch-triggered scan failed", "err", err)
			}
		})
	}

	for {
		select {
		case <-ctx.Done():
			if debounce != nil {
				debounce.Stop()
			}
			return
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			// Watch a newly-created subdirectory (just this one — its own children
			// fire their own Create events once it's watched, so no re-walk needed).
			if event.Op&fsnotify.Create != 0 {
				if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
					if err := w.Add(event.Name); err != nil {
						s.log.Debug("watch add failed", "path", event.Name, "err", err)
					}
				}
			}
			// Re-index on any media-file change, or any create (covers new dirs/files).
			if isMedia(event.Name) || event.Op&fsnotify.Create != 0 {
				trigger()
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			s.log.Warn("fs-watch error", "err", err)
		}
	}
}

func (s *Scanner) addWatchesRecursive(w *fsnotify.Watcher, root string) {
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // skip unreadable entries, keep walking
		}
		if d.IsDir() {
			if err := w.Add(path); err != nil {
				s.log.Debug("watch add failed", "path", path, "err", err)
			}
		}
		return nil
	})
}
