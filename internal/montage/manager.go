// Package montage makes hover montages (F83, HOLODEX-578): a short silent MP4 of
// clips sampled across a video, played by a grid card on hover or focus.
//
// It is a tier of its own beside the still thumbnail pipeline (media-ingest.md,
// "Hover montages"): its own state table, queue, workers, storage and routes. The
// dependency runs one way only. This package may read the still queue's depth so
// montages wait behind stills; internal/thumbnail never imports this package.
package montage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"holodex/internal/repo"
)

// Mode is the owner's generation mode (spec F83 RD6).
type Mode string

const (
	ModeOff       Mode = "off"       // nothing is made, not even on request
	ModeManual    Mode = "manual"    // only owner requests (the default)
	ModeAutomatic Mode = "automatic" // startup backfill plus browse bumps
)

// ParseMode reads the configured mode. Empty is manual; anything unknown is an
// error, so a typo can never silently turn generation on.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case "", ModeManual:
		return ModeManual, nil
	case ModeOff, ModeAutomatic:
		return Mode(s), nil
	}
	return "", fmt.Errorf("montage_mode %q: want off, manual or automatic", s)
}

// Row states in video_montages.
const (
	StateGenerated = "generated"
	StateFailed    = "failed"
)

// ErrDisabled is returned by Request in off mode.
var ErrDisabled = errors.New("montage generation is off")

// ErrUnavailable is returned by Request when the workers couldn't start (the
// montage directory couldn't be created), so a request never waits forever.
var ErrUnavailable = errors.New("montage generation unavailable")

// ErrTooShort is recorded as the failure reason for videos under MinDurationSec.
var ErrTooShort = errors.New("too short")

// Repository is the state the manager reads and writes. *repo.Repo satisfies it.
type Repository interface {
	SetMontageState(ctx context.Context, id int64, state, errText string) error
	MontageState(ctx context.Context, id int64) (repo.MontageRow, bool, error)
	MontageAttempted(ctx context.Context, ids []int64) (map[int64]bool, error)
	MontageBackfillCandidates(ctx context.Context, minDurationSec, limit int) ([]repo.MontageCandidate, error)
	MontageCandidateByID(ctx context.Context, id int64) (repo.MontageCandidate, bool, error)
}

// StillQueue is the one read across the boundary: how many stills are waiting.
type StillQueue interface {
	QueueDepth() int
}

// Config configures the manager.
type Config struct {
	Mode       Mode
	Workers    int    // default 1
	Width      int    // no wider than the still thumbnail; default 400
	Nice       bool   // nice/ionice ffmpeg like stills
	Dir        string // DATA_PATH/montages
	FfmpegPath string // default "ffmpeg"
}

// Manager owns the montage queue and its workers.
type Manager struct {
	cfg    Config
	log    *slog.Logger
	repo   Repository
	stills StillQueue
	queue  *queue

	inFlight    atomic.Int64
	unavailable atomic.Bool // Run couldn't create Dir; Request refuses
	mu          sync.Mutex
	making      map[int64]struct{}

	// gen is the generation seam; tests replace it to avoid ffmpeg.
	gen func(ctx context.Context, src string, starts []float64, dst string) error
	// stillPoll is how often a worker re-checks the still queue while it waits.
	stillPoll time.Duration
}

// New builds a manager. stills may be nil (no stills pipeline: never wait).
func New(cfg Config, log *slog.Logger, r Repository, stills StillQueue) *Manager {
	if cfg.Mode == "" {
		cfg.Mode = ModeManual
	}
	if cfg.Workers <= 0 {
		cfg.Workers = 1
	}
	if cfg.Width <= 0 {
		cfg.Width = 400
	}
	if cfg.FfmpegPath == "" {
		cfg.FfmpegPath = "ffmpeg"
	}
	m := &Manager{
		cfg:       cfg,
		log:       log,
		repo:      r,
		stills:    stills,
		queue:     newQueue(),
		making:    make(map[int64]struct{}),
		stillPoll: 2 * time.Second,
	}
	m.gen = m.generate
	return m
}

// Mode reports the configured mode.
func (m *Manager) Mode() Mode { return m.cfg.Mode }

// Available reports whether ffmpeg can be found.
func (m *Manager) Available() error {
	_, err := exec.LookPath(m.cfg.FfmpegPath)
	return err
}

// Path is where a video's montage lives.
func Path(dir string, id int64) string {
	return filepath.Join(dir, strconv.FormatInt(id, 10)+".mp4")
}

// Run starts the workers (and, in automatic mode, the backfill) and blocks until
// ctx is done. In off mode it returns at once.
func (m *Manager) Run(ctx context.Context) {
	if m.cfg.Mode == ModeOff {
		return
	}
	if err := os.MkdirAll(m.cfg.Dir, 0o755); err != nil {
		m.log.Error("montage dir; montages unavailable", "dir", m.cfg.Dir, "err", err)
		m.unavailable.Store(true)
		return
	}
	var wg sync.WaitGroup
	for range m.cfg.Workers {
		wg.Add(1)
		go func() { defer wg.Done(); m.worker(ctx) }()
	}
	if m.cfg.Mode == ModeAutomatic {
		go m.backfill(ctx)
	}
	wg.Wait()
}

// Request queues one video's montage at the front of the montage queue (spec
// P0-15). It is how manual mode makes anything at all.
func (m *Manager) Request(id int64) error {
	if m.cfg.Mode == ModeOff {
		return ErrDisabled
	}
	if m.unavailable.Load() {
		return ErrUnavailable
	}
	m.queue.push(id, true)
	return nil
}

// EnqueueVisible is the browse bump: in automatic mode it queues the given
// videos that have never had a montage attempted. In manual and off mode it does
// nothing, so browsing never starts work the owner didn't ask for (RD6).
func (m *Manager) EnqueueVisible(ctx context.Context, ids []int64) {
	if m.cfg.Mode != ModeAutomatic || len(ids) == 0 {
		return
	}
	attempted, err := m.repo.MontageAttempted(ctx, ids)
	if err != nil {
		m.log.Warn("montage enqueue visible", "err", err)
		return
	}
	for _, id := range ids {
		if !attempted[id] {
			m.queue.push(id, false)
		}
	}
}

// Invalidate re-queues a video whose file changed (spec P0-12), but only when a
// montage was attempted before: a file change never starts a first montage.
func (m *Manager) Invalidate(ctx context.Context, id int64) {
	if m.cfg.Mode == ModeOff {
		return
	}
	if _, ok, err := m.repo.MontageState(ctx, id); err != nil || !ok {
		return
	}
	m.queue.push(id, false)
}

// Status is one video's montage status as the owner sees it.
type Status struct {
	State     string `json:"state"` // none | waiting | making | ready | failed
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Status reports where a video's montage stands. A queued or running job wins
// over the stored row, so a Remake reads as waiting, then making.
func (m *Manager) Status(ctx context.Context, id int64) (Status, error) {
	m.mu.Lock()
	_, making := m.making[id]
	m.mu.Unlock()
	switch {
	case making:
		return Status{State: "making"}, nil
	case m.queue.isPending(id):
		return Status{State: "waiting"}, nil
	}
	row, ok, err := m.repo.MontageState(ctx, id)
	if err != nil {
		return Status{}, err
	}
	// A failed row wins over an older file, so a failed Remake shows its reason
	// (the card keeps playing the old montage meanwhile).
	if ok && row.State == StateFailed {
		return Status{State: "failed", Error: row.Error}, nil
	}
	if fi, statErr := os.Stat(Path(m.cfg.Dir, id)); statErr == nil && !fi.IsDir() {
		return Status{State: "ready", SizeBytes: fi.Size()}, nil
	}
	return Status{State: "none"}, nil
}

// Stats is the montage queue's activity read-model, never merged into the still
// thumbnail's (spec P0-14).
type Stats struct {
	Mode     Mode `json:"mode"`
	Depth    int  `json:"depth"`
	High     int  `json:"high"`
	Normal   int  `json:"normal"`
	InFlight int  `json:"in_flight"`
	Workers  int  `json:"workers"`
}

// QueueStats snapshots the queue.
func (m *Manager) QueueStats() Stats {
	h, n := m.queue.counts()
	workers := m.cfg.Workers
	if m.cfg.Mode == ModeOff {
		workers = 0
	}
	return Stats{Mode: m.cfg.Mode, Depth: h + n, High: h, Normal: n,
		InFlight: int(m.inFlight.Load()), Workers: workers}
}

// QueueDepth is the montage queue's waiting count, for /metrics.
func (m *Manager) QueueDepth() int {
	h, n := m.queue.counts()
	return h + n
}
