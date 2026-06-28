// Package refresh implements per-item metadata refresh (F31, ADR-047): an
// owner-gated action that re-reads one media file's embedded metadata — forced,
// bypassing the scanner's (size, mtime) change-detection so it catches edits
// another system made even when the file's mtime is unchanged — and persists the
// file layer. A later slice adds re-enrich of the item's linked providers.
//
// The service is split into a side-effect-free plan phase (read the file, build
// the candidate row) and a committing apply phase (persist). F31 runs them
// back-to-back; keeping them separable is what lets a future batch op interpose
// conflict resolution between them (ADR-047). This slice covers the file layer
// only — the typed per-source RefreshReport with change/disagreement detail
// arrives with the re-enrich slice.
package refresh

import (
	"context"

	"holodex/internal/model"
)

// FileExtractor builds a candidate Video from a file without persisting it — the
// read half of a refresh, with no change-detection. Implemented by
// *scanner.Scanner.BuildVideoFromFile.
type FileExtractor interface {
	BuildVideoFromFile(ctx context.Context, path string) (*model.Video, []model.ExtraMetadata, error)
}

// Store resolves a refresh target and persists the file layer. Implemented by
// *repo.Repo. RefreshTarget must distinguish a missing row from a soft-deleted
// one (repo.ErrNotFound vs repo.ErrDeleted) so the handler can answer 404 vs 409;
// those sentinels propagate unwrapped through Refresh.
type Store interface {
	RefreshTarget(ctx context.Context, id int64) (path string, err error)
	UpsertVideo(ctx context.Context, v *model.Video, extra []model.ExtraMetadata) (int64, error)
}

// Service orchestrates a per-item refresh.
type Service struct {
	ext   FileExtractor
	store Store
}

// NewService wires the refresh service (F31). ext re-reads files (the scanner);
// store resolves targets and persists (the repo).
func NewService(ext FileExtractor, store Store) *Service {
	return &Service{ext: ext, store: store}
}

// Report is the outcome of a refresh. This slice reports only the file source;
// per-source results, the changed-field counts, and the sources_disagree flag
// are added with the re-enrich slice (F31.14).
type Report struct {
	VideoID int64 `json:"video_id"`
	FileOK  bool  `json:"file_ok"`
}

// plan is the read phase: the candidate file layer for one item. No writes.
// (Re-enrich adds provider fetch + diff here.)
type plan struct {
	video *model.Video
	extra []model.ExtraMetadata
}

// doPlan resolves the target and re-extracts the file — no persistence. It
// returns repo.ErrNotFound / repo.ErrDeleted unwrapped for the handler to map,
// and never touches a soft-deleted row (RefreshTarget refuses it).
func (s *Service) doPlan(ctx context.Context, id int64) (*plan, error) {
	path, err := s.store.RefreshTarget(ctx, id)
	if err != nil {
		return nil, err
	}
	v, extra, err := s.ext.BuildVideoFromFile(ctx, path)
	if err != nil {
		return nil, err
	}
	return &plan{video: v, extra: extra}, nil
}

// Refresh re-extracts one media file (forced) and persists the file layer,
// returning a report. A missing/soft-deleted id, or a file that can't be read,
// returns an error before any write — the row is never mutated on failure.
func (s *Service) Refresh(ctx context.Context, id int64) (Report, error) {
	p, err := s.doPlan(ctx, id)
	if err != nil {
		return Report{}, err
	}
	if _, err := s.store.UpsertVideo(ctx, p.video, p.extra); err != nil {
		return Report{}, err
	}
	return Report{VideoID: id, FileOK: true}, nil
}
