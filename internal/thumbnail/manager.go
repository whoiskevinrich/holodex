// Package thumbnail produces and manages per-video cover images (ADR-009).
//
// Images come from two sources with very different cost, handled in tiers:
//
//   - Tier 1 (embedded art): the scanner calls ExtractEmbedded at index time for
//     files whose container carries cover art — a near-free `exiftool -b` byte
//     copy. The bytes are written straight to disk.
//   - Tier 2 (generated frames): files without embedded art are enqueued for
//     background ffmpeg frame extraction, drained by a bounded, low-priority
//     worker pool so generation never overloads a modest host.
//   - Tier 3 (priority bump): items the user is currently viewing are pushed to
//     the front of the queue (EnqueueHigh) so visible cards fill in first.
//
// Generated images are written once to DATA_PATH/thumbnails/{id}.jpg and served
// as static files — the disk file is the cache (ADR-022 deferred the in-process
// cache). The per-video lifecycle is tracked in videos.thumbnail_state; the queue
// here is just the in-flight work buffer, re-seeded from the DB on restart.
package thumbnail

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// Repository is the subset of the data layer the pipeline needs. It is expressed
// in repo types (matching the scanner's convention) so production wiring needs no
// adapter; tests provide an in-memory implementation.
type Repository interface {
	SetThumbnailState(ctx context.Context, id int64, state string) error
	ResetThumbnailState(ctx context.Context, id int64) error
	ThumbnailBackfillCandidates(ctx context.Context, limit int) ([]repo.ThumbnailCandidate, error)
	ThumbnailCandidateByID(ctx context.Context, id int64) (repo.ThumbnailCandidate, bool, error)
}

// Config tunes the pipeline (ADR-009 env vars). Dir is DATA_PATH/thumbnails.
type Config struct {
	Enabled      bool
	Backfill     string // "eager" | "lazy"
	Workers      int
	Nice         bool
	SeekPercent  int
	Width        int
	Dir          string
	FfmpegPath   string
	ExiftoolPath string
}

// Manager owns the queue and worker pool. A single instance is shared by the
// scanner (Tier 1 + new-file enqueue) and the API handlers (Tier 3 + serving).
type Manager struct {
	cfg   Config
	log   *slog.Logger
	repo  Repository
	queue *queue

	// Binary-invoking seams, overridable in tests so the pipeline can be
	// exercised without real ffmpeg/exiftool. Defaulted in New.
	gen func(ctx context.Context, c repo.ThumbnailCandidate, outPath string) error
	art func(ctx context.Context, path, dst string) (bool, error)
}

// New constructs a Manager. It is always safe to construct even when disabled;
// the methods become no-ops so callers need no nil checks.
func New(cfg Config, log *slog.Logger, r Repository) *Manager {
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.Width <= 0 {
		cfg.Width = 400
	}
	if cfg.FfmpegPath == "" {
		cfg.FfmpegPath = "ffmpeg"
	}
	if cfg.ExiftoolPath == "" {
		cfg.ExiftoolPath = "exiftool"
	}
	m := &Manager{cfg: cfg, log: log, repo: r, queue: newQueue()}
	m.gen = m.generateFrame
	m.art = func(ctx context.Context, path, dst string) (bool, error) {
		return extractCoverArt(ctx, cfg.ExiftoolPath, path, dst)
	}
	// Create the thumbnail dir once, up front (New runs before the scanner and
	// worker goroutines start), so neither ExtractEmbedded nor a worker has to.
	if cfg.Enabled && cfg.Dir != "" {
		if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
			log.Warn("thumbnail dir create failed", "dir", cfg.Dir, "err", err)
		}
	}
	return m
}

// Enabled reports whether generation (Tier 2/3) is active.
func (m *Manager) Enabled() bool { return m.cfg.Enabled }

// Available reports whether the ffmpeg binary resolves on PATH, so the caller can
// warn at startup. Embedded-art extraction additionally needs exiftool.
func (m *Manager) Available() error {
	if !m.cfg.Enabled {
		return nil
	}
	_, err := exec.LookPath(m.cfg.FfmpegPath)
	return err
}

// Run starts the worker pool and (in eager mode) the one-shot backfill sweep,
// blocking until ctx is cancelled. Intended to be launched in its own goroutine.
func (m *Manager) Run(ctx context.Context) {
	if !m.cfg.Enabled {
		return
	}
	if err := os.MkdirAll(m.cfg.Dir, 0o755); err != nil {
		m.log.Error("thumbnail dir create failed; generation disabled", "dir", m.cfg.Dir, "err", err)
		return
	}

	var wg sync.WaitGroup
	for i := 0; i < m.cfg.Workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.worker(ctx)
		}()
	}
	if m.cfg.Backfill != "lazy" {
		go m.backfill(ctx)
	}
	wg.Wait()
}

// Enqueue adds one video at normal (backfill) priority — used by the scanner for
// a newly-indexed file without embedded art (Tier 2).
func (m *Manager) Enqueue(id int64) {
	if !m.cfg.Enabled {
		return
	}
	m.queue.push(id, false)
}

// EnqueueHigh pushes ids to the front of the queue — used for currently-visible
// items (Tier 3) and explicit regeneration.
func (m *Manager) EnqueueHigh(ids []int64) {
	if !m.cfg.Enabled {
		return
	}
	for _, id := range ids {
		m.queue.push(id, true)
	}
}

// QueueDepth is the count of pending jobs, surfaced as thumbnail_queue_depth
// (F11.8; full Prometheus deferred).
func (m *Manager) QueueDepth() int { return m.queue.depth() }

// ExtractEmbedded performs Tier 1: writes the container's embedded cover art for
// an indexed file to disk and marks the video "embedded". Returns ok=false (no
// error) when no art could be extracted, signalling the caller to fall back to
// background generation.
func (m *Manager) ExtractEmbedded(ctx context.Context, id int64, path string) (bool, error) {
	if !m.cfg.Enabled {
		return false, nil
	}
	ok, err := m.art(ctx, path, m.thumbPath(id))
	if err != nil || !ok {
		return false, err
	}
	if err := m.repo.SetThumbnailState(ctx, id, model.ThumbnailEmbedded); err != nil {
		return false, err
	}
	return true, nil
}

// ThumbPath is the on-disk location for a video's thumbnail (ADR-009/ADR-014).
// Exported so the serving handler and the pipeline share one filename scheme.
func ThumbPath(dir string, id int64) string {
	return filepath.Join(dir, strconv.FormatInt(id, 10)+".jpg")
}

func (m *Manager) thumbPath(id int64) string { return ThumbPath(m.cfg.Dir, id) }
