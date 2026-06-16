package personimage

import (
	"context"
	"fmt"

	"holodex/internal/model"
)

// imageRepo is the repo subset the asset sink needs (satisfied by *repo.Repo). Kept
// an interface so personimage stays free of an import on repo.
type imageRepo interface {
	InsertPersonImage(ctx context.Context, personID int64, role, source, provider, externalID string, w, h, byteSize int) (int64, error)
	DeletePersonImage(ctx context.Context, personID, imageID int64) error
}

// Sink stores enrichment-downloaded image bytes as person images (F24, ADR-037),
// satisfying enrich.ImageSink. It is the one place the enrichment asset path meets
// the same normalize-then-atomic-write storage the upload handler uses, so a
// provider photo gets the identical metadata strip and bomb guard as an upload.
type Sink struct {
	repo   imageRepo
	dir    string
	maxDim int
}

// NewSink builds the asset sink over the image repo, the on-disk root, and the
// downscale bound.
func NewSink(r imageRepo, dir string, maxDim int) *Sink {
	return &Sink{repo: r, dir: dir, maxDim: maxDim}
}

// StoreAsset normalizes raw provider bytes and stores them as the given core role
// for a person, recording enrichment provenance (provider + externalID). A decode
// failure means nothing is written (the guard the spec requires). On a disk-write
// failure the just-inserted row is rolled back so the index never points at a
// missing file.
func (s *Sink) StoreAsset(ctx context.Context, personID int64, role, provider, externalID string, raw []byte) error {
	norm, w, h, err := Normalize(raw, s.maxDim)
	if err != nil {
		return fmt.Errorf("normalize asset: %w", err)
	}
	id, err := s.repo.InsertPersonImage(ctx, personID, role, model.PersonImageSourceEnrichment, provider, externalID, w, h, len(norm))
	if err != nil {
		return fmt.Errorf("insert asset row: %w", err)
	}
	if err := Store(s.dir, personID, id, norm); err != nil {
		_ = s.repo.DeletePersonImage(ctx, personID, id)
		return fmt.Errorf("store asset: %w", err)
	}
	return nil
}
