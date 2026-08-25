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

	"holodex/internal/filmimage"
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

// StudioRepo is the repo subset the studio side of the sink needs.
type StudioRepo interface {
	ReplaceStudioImage(ctx context.Context, in repo.StudioImageInsert) (int64, error)
	GetStudioImage(ctx context.Context, studioID int64, role string) (repo.StudioImage, error)
	DeleteStudioImage(ctx context.Context, studioID int64, role string) error
	LockedStudioImageRoles(ctx context.Context, studioID int64) (map[string]struct{}, error)
}

// FilmRepo is the repo subset the film owner-upload path needs (F56/HOLODEX-280,
// ADR-086). Unlike StudioRepo, every method is explicitly scoped by source — see
// internal/repo/film_images.go's doc comment — because film_images' UNIQUE is
// (film_id, role, source), not (film_id, role) alone. Not yet wired into Sink's
// entityType dispatch: no enrichment caller writes film images today (that's
// HOLODEX-284's scope), so ReplaceFilmImageFile below is called directly by the
// owner-upload HTTP handler, the same way ReplaceStudioImageFile was before F51's
// Sink dispatch existed.
type FilmRepo interface {
	ReplaceFilmImage(ctx context.Context, in repo.FilmImageInsert) (int64, error)
	GetFilmImage(ctx context.Context, filmID int64, role, source string) (repo.FilmImage, error)
	DeleteFilmImage(ctx context.Context, filmID int64, role, source string) error
}

// Sink implements enrich.ImageSink over both entity kinds. Constructed once in main
// and wired via enrich.Service.SetImageSink.
type Sink struct {
	personRepo   personRepo
	personDir    string
	personMaxDim int

	studioRepo   StudioRepo
	studioDir    string
	studioMaxDim int
}

// New builds the combined sink over both storage engines' on-disk roots and
// downscale bounds.
func New(pr personRepo, personDir string, personMaxDim int, sr StudioRepo, studioDir string, studioMaxDim int) *Sink {
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
// Person-only (F25.29's headshot-to-poster seed is its sole caller, gated by
// entityType == EnrichEntityPerson in enrich.Service) — studio has no caller that
// needs a fill-if-empty write, every studio store already goes through StoreAsset.
func (s *Sink) StoreAssetIfAbsent(ctx context.Context, entityType string, entityID int64, role, provider, externalID, url string, raw []byte) error {
	if entityType != model.EnrichEntityPerson {
		return fmt.Errorf("imagesink: StoreAssetIfAbsent is person-only, got entity type %q", entityType)
	}
	switch _, err := s.personRepo.CorePersonImage(ctx, entityID, role); {
	case err == nil:
		return nil // slot already filled — leave it
	case errors.Is(err, repo.ErrNotFound):
		return s.storePersonAsset(ctx, entityID, role, provider, externalID, url, raw, false)
	default:
		return fmt.Errorf("check core slot: %w", err)
	}
}

// SuppressedAssetURLs returns asset URLs the owner deleted, so enrichment skips
// re-adding them. Person-only store; every other entity type (studio included) has
// no suppression store — deleting a core slot simply empties it — so it always
// returns an empty set.
func (s *Sink) SuppressedAssetURLs(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error) {
	switch entityType {
	case model.EnrichEntityPerson:
		return s.personRepo.SuppressedPersonImageURLs(ctx, entityID)
	default:
		return map[string]struct{}{}, nil
	}
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
// Person-only; every other entity type (studio included) always returns empty — a
// studio has no gallery to dedup against.
func (s *Sink) ExistingAssetURLs(ctx context.Context, entityType string, entityID int64) (map[string]struct{}, error) {
	switch entityType {
	case model.EnrichEntityPerson:
		return s.personRepo.ExistingPersonImageURLs(ctx, entityID)
	default:
		return map[string]struct{}{}, nil
	}
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
	_, err = ReplaceStudioImageFile(ctx, s.studioRepo, s.studioDir, repo.StudioImageInsert{
		StudioID: studioID, Role: role, Source: model.StudioImageSourceEnrichment,
		Provider: provider, ExternalID: externalID,
	}, norm, w, h)
	return err
}

// ReplaceStudioImageFile replaces the studio's row for `in.Role` with already-
// normalized bytes, writes the file, and removes the superseded file on success — or
// rolls back the just-inserted row on a store failure. Exported so the owner-upload
// endpoint (internal/api) shares this exact sequence with the enrichment path above
// rather than reimplementing it; the two differ only in the Source/Provider/
// ExternalID already set on `in`, and in how they want to report a normalize failure
// (400 for an upload vs. a logged enrichment failure) — which is why normalization
// stays the caller's own first step rather than folding into this function.
func ReplaceStudioImageFile(ctx context.Context, sr StudioRepo, dir string, in repo.StudioImageInsert, norm []byte, width, height int) (int64, error) {
	in.Width, in.Height, in.ByteSize = width, height, len(norm)
	existing, existErr := sr.GetStudioImage(ctx, in.StudioID, in.Role)
	id, err := sr.ReplaceStudioImage(ctx, in)
	if err != nil {
		return 0, fmt.Errorf("insert studio image row: %w", err)
	}
	if err := studioimage.Store(dir, in.StudioID, id, norm); err != nil {
		_ = sr.DeleteStudioImage(ctx, in.StudioID, in.Role)
		return 0, fmt.Errorf("store studio image: %w", err)
	}
	if existErr == nil && existing.ID != 0 {
		_ = studioimage.Remove(dir, in.StudioID, existing.ID) // best-effort; a left-behind file is harmless
	}
	return id, nil
}

// ReplaceFilmImageFile replaces the film's row for `in.Role`+`in.Source` with
// already-normalized bytes, writes the file, and removes the superseded file on
// success — or rolls back the just-inserted row on a store failure. Mirrors
// ReplaceStudioImageFile exactly; exported so the owner-upload endpoint
// (internal/api) shares this sequence rather than reimplementing it.
func ReplaceFilmImageFile(ctx context.Context, fr FilmRepo, dir string, in repo.FilmImageInsert, norm []byte, width, height int) (int64, error) {
	in.Width, in.Height, in.ByteSize = width, height, len(norm)
	existing, existErr := fr.GetFilmImage(ctx, in.FilmID, in.Role, in.Source)
	id, err := fr.ReplaceFilmImage(ctx, in)
	if err != nil {
		return 0, fmt.Errorf("insert film image row: %w", err)
	}
	if err := filmimage.Store(dir, in.FilmID, id, norm); err != nil {
		_ = fr.DeleteFilmImage(ctx, in.FilmID, in.Role, in.Source)
		return 0, fmt.Errorf("store film image: %w", err)
	}
	if existErr == nil && existing.ID != 0 {
		_ = filmimage.Remove(dir, in.FilmID, existing.ID) // best-effort; a left-behind file is harmless
	}
	return id, nil
}
