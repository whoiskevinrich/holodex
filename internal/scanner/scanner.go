// Package scanner walks MEDIA_PATH, extracts metadata, and reconciles the index.
//
// Design (ADR-011, ADR-018):
//   - Walk recursively; if FollowSymlinks, resolve each entry to its canonical
//     path (filepath.EvalSymlinks) and dedup via a visited-set — this also makes
//     symlink-loop detection automatic.
//   - For each file compare (size, mtime) against the stored row:
//       no row            -> extract + insert
//       unchanged         -> skip (no extraction subprocess)
//       size/mtime differ -> re-extract + update
//   - Files modified within MinAge are skipped this cycle (mid-copy guard).
//   - Active rows not seen during the walk are marked active=false.
package scanner

import (
	"context"
	"log/slog"
	"time"
)

type Config struct {
	MediaPath      string
	FollowSymlinks bool
	MaxDepth       int
	MinAge         time.Duration
	Workers        int
}

type Scanner struct {
	cfg Config
	log *slog.Logger
	// TODO(phase1): repository + metadata.Extractor dependencies.
}

func New(cfg Config, log *slog.Logger) *Scanner {
	return &Scanner{cfg: cfg, log: log}
}

// ScanOnce performs a single reconciliation pass over MediaPath.
func (s *Scanner) ScanOnce(ctx context.Context) error {
	if s.cfg.MediaPath == "" {
		s.log.Warn("MEDIA_PATH not set; skipping scan")
		return nil
	}
	// TODO(phase1): implement walk + change detection + extraction per ADR-018.
	s.log.Info("scan requested", "media_path", s.cfg.MediaPath)
	return nil
}

// Run starts the periodic scan loop plus (later) the filesystem watcher.
func (s *Scanner) Run(ctx context.Context, interval time.Duration) {
	if err := s.ScanOnce(ctx); err != nil {
		s.log.Error("initial scan failed", "err", err)
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
