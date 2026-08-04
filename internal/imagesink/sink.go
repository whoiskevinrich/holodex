// Package imagesink is the entity-generic enrich.ImageSink adapter (F51, ADR-079):
// it dispatches by entity type to the right storage engine — person_images (with its
// gallery/suppression/content-hash-dedup machinery) or studio_images (three core
// roles, no gallery) — behind one interface, so enrich.Service.downloadAssets stays
// entity-agnostic. This is the second real use of the person image-asset pipeline
// (Person was the first), the bar this codebase uses elsewhere to generalize a
// subsystem rather than duplicate it (BaselineSource/ADR-052, resolveOrCreateByName/
// ADR-061).
package imagesink

import (
	"context"
	"errors"
	"fmt"

	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/studioimage"
)

// personRepo is the repo subset the person side of the sink needs (satisfied by
// *repo.Repo). Kept an interface so this package stays free of a direct repo import
// requirement beyond the insert/delete payload types.
type personRepo interface {
	InsertPersonImage(ctx context.Context, in repo.PersonImageInsert) (int64, error)
	DeletePersonImage(ctx context.Context, personID, imageID int64) error
	SuppressedPersonImageURLs(ctx context.Context, personID int64) (map[string]struct{}, error)
	CorePersonImage(ctx context.Context, personID int64, role string) (model.PersonImage, error)
	LockedCoreRoles(ctx context.Context, personID int64) (map[string]struct{}, error)
	ExistingPersonImageURLs(ctx context.Context, personID int64) (map[string]struct{}, error)
}

// studioRepo is the repo subset the studio side of the sink needs.
type studioRepo interface {
	ReplaceStudioImage(ctx context.Context, in repo.StudioImageInsert) (int64, error)
	GetStudioImage(ctx context.Context, studioID int64, role string) (repo.StudioImage, error)
	DeleteStudioImage(ctx context.Context, studioID int64, role string) error
	LockedStudioImageRoles(ctx context.Context, studioID int64) (map[string]struct{}, error)
}

// Sink implements enrich.ImageSink over both entity kinds. Constructed once in main
// and wired via enrich.Service.SetImageSink.
type Sink struct {
	personRepo   personRepo
	personDir    string
	personMaxDim int

	studioRepo   studioRepo
	studioDir    string
	studioMaxDim int
}

// New builds the combined sink over both storage engines' on-disk roots and
// downscale bounds.
func New(pr personRepo, personDir string, personMaxDim int, sr studioRepo, studioDir string, studioMaxDim int) *Sink {
	return &Sink{
		personRepo: pr, personDir: personDir, personMaxDim: personMaxDim,
		studioRepo: sr, studioDir: studioDir, studioMaxDim: studioMaxDim,
	}
}

// StoreAsset normalizes raw provider bytes and stores them under the given role for
// an entity, dispatching by entityType. See enrich.ImageSink for the contract.
func (s *Sink) StoreAsset(ctx context.Context, entityType string, entityID int64, role, provider, externalID, url string, raw []byte, overCap bool) error {
	switch entityType {
	case model.EnrichEntityPerson:
		return s.storePersonAsset(ctx, entityID, role, provider, externalID, url, raw, overCap)
	case model.EnrichEntityStudio:
		return s.storeStudioAsset(ctx, entityID, role, provider, externalID, raw)
	default:
		return fmt.Errorf("imagesink: unsupported entity type %q", entityType)
	}
}

// StoreAssetIfAbsent stores under a core role only when that slot is currently empty.
func (s *Sink) StoreAssetIfAbsent(ctx context.Context, entityType string, entityID int64, role, provider, externalID, url string, raw []byte) error {
	switch entityType {
	case model.EnrichEntityPerson:
		switch _, err := s.personRepo.CorePersonImage(ctx, entityID, role); {
		case err == nil:
			return nil // slot already filled — leave it
		case errors.Is(err, repo.ErrNotFound):
			return s.storePersonAsset(ctx, entityID, role, provider, externalID, url, raw, false)
		default:
			return fmt.Errorf("check core slot: %w", err)
		}
	case model.EnrichEntityStudio:
		switch _, err := s.studioRepo.GetStudioImage(ctx, entityID, role); {
		case err == nil:
			return nil
		case errors.Is(err, repo.ErrNotFound):
			return s.storeStudioAsset(ctx, entityID, role, provider, externalID, raw)
		default:
			return fmt.Errorf("check studio image slot: %w", err)
		}
	default:
		return fmt.Errorf("imagesink: unsupported entity type %q", entityType)
	}
}

// SuppressedAssetURLs returns asset URLs the owner deleted, so enrichment skips
// re-adding them. A studio has no suppression store — deleting a core slot simply
// empties it — so it always returns an empty set.
func (s *Sink) SuppressedAssetURLs(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error) {
	if entityType == model.EnrichEntityPerson {
		return s.personRepo.SuppressedPersonImageURLs(ctx, entityID)
	}
	return map[string]struct{}{}, nil
}

// LockedCoreRoles returns the roles the owner set by hand, which enrichment must
// never overwrite (ADR-049, generalized to studio by F51/ADR-079).
func (s *Sink) LockedCoreRoles(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error) {
	switch entityType {
	case model.EnrichEntityPerson:
		return s.personRepo.LockedCoreRoles(ctx, entityID)
	case model.EnrichEntityStudio:
		return s.studioRepo.LockedStudioImageRoles(ctx, entityID)
	default:
		return map[string]struct{}{}, nil
	}
}

// ExistingAssetURLs returns asset URLs already stored, for the URL dedup fast-path.
// Always empty for a studio (no gallery to dedup against).
func (s *Sink) ExistingAssetURLs(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error) {
	if entityType == model.EnrichEntityPerson {
		return s.personRepo.ExistingPersonImageURLs(ctx, entityID)
	}
	return map[string]struct{}{}, nil
}

// storePersonAsset is the pre-F51 person store path, unchanged.
func (s *Sink) storePersonAsset(ctx context.Context, personID int64, role, provider, externalID, url string, raw []byte, overCap bool) error {
	norm, w, h, err := personimage.Normalize(raw, s.personMaxDim)
	if err != nil {
		return fmt.Errorf("normalize asset: %w", err)
	}
	id, err := s.personRepo.InsertPersonImage(ctx, repo.PersonImageInsert{
		PersonID:    personID,
		Role:        role,
		Source:      model.PersonImageSourceEnrichment,
		Provider:    provider,
		ExternalID:  externalID,
		SourceURL:   url,
		ContentHash: personimage.Hash(norm),
		Width:       w,
		Height:      h,
		ByteSize:    len(norm),
		OverCap:     overCap,
	})
	// A gallery extra duplicating an image the person already has is a silent skip
	// (F34/ADR-050) — nothing written, like a full gallery. The bytes never reach disk.
	if errors.Is(err, repo.ErrDuplicateImage) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("insert asset row: %w", err)
	}
	if err := personimage.Store(s.personDir, personID, id, norm); err != nil {
		_ = s.personRepo.DeletePersonImage(ctx, personID, id)
		return fmt.Errorf("store asset: %w", err)
	}
	return nil
}

// storeStudioAsset normalizes and replaces the studio's image for one role (every
// studio role is core/single-slot — always a replace, no cap/dedup to consider).
func (s *Sink) storeStudioAsset(ctx context.Context, studioID int64, role, provider, externalID string, raw []byte) error {
	norm, w, h, err := personimage.Normalize(raw, s.studioMaxDim)
	if err != nil {
		return fmt.Errorf("normalize asset: %w", err)
	}
	existing, existErr := s.studioRepo.GetStudioImage(ctx, studioID, role)
	id, err := s.studioRepo.ReplaceStudioImage(ctx, repo.StudioImageInsert{
		StudioID: studioID, Role: role, Source: model.StudioImageSourceEnrichment,
		Provider: provider, ExternalID: externalID, Width: w, Height: h, ByteSize: len(norm),
	})
	if err != nil {
		return fmt.Errorf("insert studio asset row: %w", err)
	}
	if err := studioimage.Store(s.studioDir, studioID, id, norm); err != nil {
		_ = s.studioRepo.DeleteStudioImage(ctx, studioID, role)
		return fmt.Errorf("store studio asset: %w", err)
	}
	if existErr == nil && existing.ID != 0 {
		_ = studioimage.Remove(s.studioDir, studioID, existing.ID) // best-effort; a left-behind file is harmless
	}
	return nil
}
