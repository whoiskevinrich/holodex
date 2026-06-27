package personimage

import (
	"context"
	"errors"
	"fmt"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// imageRepo is the repo subset the asset sink needs (satisfied by *repo.Repo). Kept
// an interface so personimage stays free of an import on repo.
type imageRepo interface {
	InsertPersonImage(ctx context.Context, in repo.PersonImageInsert) (int64, error)
	DeletePersonImage(ctx context.Context, personID, imageID int64) error
	SuppressedPersonImageURLs(ctx context.Context, personID int64) (map[string]struct{}, error)
	// CorePersonImage returns the filled core-role image, or repo.ErrNotFound when the
	// slot is empty — used by StoreAssetIfAbsent to avoid clobbering.
	CorePersonImage(ctx context.Context, personID int64, role string) (model.PersonImage, error)
}

// Sink stores enrichment-downloaded image bytes as person images (F25, ADR-038),
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

// StoreAsset normalizes raw provider bytes and stores them as the given role for a
// person, recording enrichment provenance (provider + externalID) and the upstream
// asset URL (so a later delete can suppress re-adding it, F25/ADR-043). A decode
// failure means nothing is written (the guard the spec requires). On a disk-write
// failure the just-inserted row is rolled back so the index never points at a
// missing file.
func (s *Sink) StoreAsset(ctx context.Context, personID int64, role, provider, externalID, url string, raw []byte) error {
	norm, w, h, err := Normalize(raw, s.maxDim)
	if err != nil {
		return fmt.Errorf("normalize asset: %w", err)
	}
	id, err := s.repo.InsertPersonImage(ctx, repo.PersonImageInsert{
		PersonID:   personID,
		Role:       role,
		Source:     model.PersonImageSourceEnrichment,
		Provider:   provider,
		ExternalID: externalID,
		SourceURL:  url,
		Width:      w,
		Height:     h,
		ByteSize:       len(norm),
	})
	if err != nil {
		return fmt.Errorf("insert asset row: %w", err)
	}
	if err := Store(s.dir, personID, id, norm); err != nil {
		_ = s.repo.DeletePersonImage(ctx, personID, id)
		return fmt.Errorf("store asset: %w", err)
	}
	return nil
}

// StoreAssetIfAbsent stores the asset under a core role only when that slot is empty,
// so enrichment can seed a poster from the headshot portrait (F25.29) without
// clobbering an existing owner- or provider-set image. A filled slot is left untouched.
func (s *Sink) StoreAssetIfAbsent(ctx context.Context, personID int64, role, provider, externalID, url string, raw []byte) error {
	switch _, err := s.repo.CorePersonImage(ctx, personID, role); {
	case err == nil:
		return nil // slot already filled — leave it
	case errors.Is(err, repo.ErrNotFound):
		return s.StoreAsset(ctx, personID, role, provider, externalID, url, raw)
	default:
		return fmt.Errorf("check core slot: %w", err)
	}
}

// SuppressedAssetURLs returns the asset URLs the owner has deleted for this person,
// so enrichment skips re-adding them (F25, ADR-043).
func (s *Sink) SuppressedAssetURLs(ctx context.Context, personID int64) (map[string]struct{}, error) {
	return s.repo.SuppressedPersonImageURLs(ctx, personID)
}
