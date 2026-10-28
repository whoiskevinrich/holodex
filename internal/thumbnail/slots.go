package thumbnail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strconv"

	"golang.org/x/image/draw"

	"holodex/internal/fieldsource"
	"holodex/internal/model"
)

// Poster source slots (F85, media-ingest.md "Poster source slots"). Each image a
// poster can come from has its own file beside the served pair, replaced
// atomically by the next image of its kind, so keeping the newest of each is the
// layout. The served pair ({id}.jpg, {id}-poster.jpg) is written only by Present.

// Slot names, the filename component of {id}-<slot>.jpg. SlotFile and SlotAuto are
// the automatic pipeline's; the owner's slots share the local kind's name.
const (
	SlotFile   = "file"
	SlotAuto   = "auto"
	SlotFrame  = fieldsource.LocalFrame
	SlotUpload = fieldsource.LocalUpload
)

// SlotPath is the on-disk location of one source slot. slot must be one of the
// Slot* constants; callers map a request value through SlotForKind first, so no
// request string ever reaches a filename.
func SlotPath(dir string, id int64, slot string) string {
	return filepath.Join(dir, strconv.FormatInt(id, 10)+"-"+slot+".jpg")
}

// SlotForKind maps a poster source kind — "file" or a local kind — to its slot,
// ok=false for anything outside that closed set.
func SlotForKind(kind string) (string, bool) {
	switch kind {
	case SlotFile:
		return SlotFile, true
	case fieldsource.LocalFrame:
		return SlotFrame, true
	case fieldsource.LocalUpload:
		return SlotUpload, true
	case fieldsource.LocalAutomatic:
		return SlotAuto, true
	}
	return "", false
}

// ContentHash is a slot's content identity: the hex sha256 of its bytes, the
// <hash> half of a local:<kind>#<hash> decision token.
func ContentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) slotPath(id int64, slot string) string { return SlotPath(m.cfg.Dir, id, slot) }

// SetOwnerImage stores the owner's newest picked frame or upload (kind is
// fieldsource.LocalFrame or LocalUpload) in its slot, makes it the poster_url
// decision, and presents it (P0-3, P0-4, P0-8). data must already be normalized
// (security-perimeter.md "Normalization at ingest").
func (m *Manager) SetOwnerImage(ctx context.Context, id int64, kind string, data []byte, frameAt *float64) error {
	if kind != fieldsource.LocalFrame && kind != fieldsource.LocalUpload {
		return fmt.Errorf("owner image: unknown kind %q", kind)
	}
	if kind != fieldsource.LocalFrame {
		frameAt = nil
	}
	slot, _ := SlotForKind(kind)
	if err := os.MkdirAll(m.cfg.Dir, 0o755); err != nil {
		return fmt.Errorf("owner image: %w", err)
	}
	if err := writeAtomic(m.slotPath(id, slot), data); err != nil {
		return fmt.Errorf("owner image: %w", err)
	}
	if err := m.repo.SetPosterSource(ctx, id, kind, ContentHash(data), frameAt, true); err != nil {
		return err
	}
	return m.Present(ctx, id)
}

// RemoveOwnerImage deletes an owner slot and a poster_url decision naming it, then
// presents the fallback (P0-9). It never touches the media file. cleared reports
// whether the removed slot was the decided poster.
func (m *Manager) RemoveOwnerImage(ctx context.Context, id int64, kind string) (cleared bool, err error) {
	slot, ok := SlotForKind(kind)
	if !ok || (kind != fieldsource.LocalFrame && kind != fieldsource.LocalUpload) {
		return false, fmt.Errorf("owner image: unknown kind %q", kind)
	}
	if err := os.Remove(m.slotPath(id, slot)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("remove owner image: %w", err)
	}
	if cleared, err = m.repo.DeletePosterSource(ctx, id, kind); err != nil {
		return false, err
	}
	return cleared, m.Present(ctx, id)
}

// Present writes the served pair from the poster_url decision (media-ingest.md
// "The served pair is a projection of the decision"): a local:<kind> decision's
// slot, else the file slot, else the auto slot. A missing chosen slot falls through
// the same order, and with no slot at all the pair is removed. When the winning
// slot's bytes already match the served poster the pair is left alone, so an
// unchanged poster keeps its ?v= token. Every path that changes a slot or the
// poster_url decision ends here, and nothing else writes the pair.
func (m *Manager) Present(ctx context.Context, id int64) error {
	if m.cfg.Dir == "" {
		return nil
	}
	m.presentMu.Lock()
	defer m.presentMu.Unlock()

	dec, err := m.repo.PosterDecision(ctx, id)
	if err != nil {
		return err
	}
	order := []string{SlotFile, SlotAuto}
	if kind := fieldsource.Local(dec); kind != "" {
		slot, _ := SlotForKind(kind)
		order = append([]string{slot}, order...)
	}
	var data []byte
	for _, slot := range order {
		if data, err = os.ReadFile(m.slotPath(id, slot)); err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("present poster: %w", err)
		}
		data = nil
	}
	if data == nil {
		for _, p := range []string{m.thumbPath(id), m.posterPath(id)} {
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("present poster: %w", err)
			}
		}
		return nil
	}
	if cur, err := os.ReadFile(m.posterPath(id)); err == nil && bytes.Equal(cur, data) {
		if _, err := os.Stat(m.thumbPath(id)); err == nil {
			return nil
		}
	}
	thumb, err := m.thumbTier(data)
	if err != nil {
		return fmt.Errorf("present poster: %w", err)
	}
	if err := writeAtomic(m.thumbPath(id), thumb); err != nil {
		return fmt.Errorf("present poster: %w", err)
	}
	if err := writeAtomic(m.posterPath(id), data); err != nil {
		return fmt.Errorf("present poster: %w", err)
	}
	return nil
}

// thumbTier derives the thumbnail tier (thumbnail_width) from a slot's bytes in
// process: the bytes unchanged when already narrow enough or undecodable here,
// else a CatmullRom downscale re-encoded as JPEG.
func (m *Manager) thumbTier(data []byte) ([]byte, error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= m.cfg.Width {
		return data, nil
	}
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, nil
	}
	h := max((cfg.Height*m.cfg.Width+cfg.Width/2)/cfg.Width, 1)
	dst := image.NewRGBA(image.Rect(0, 0, m.cfg.Width, h))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BackfillSlots is the one-time upgrade to source slots (media-ingest.md
// "Upgrade"): each video whose served pair holds an image and whose slot is not yet
// seeded gets one from the pair, by thumbnail_state — embedded → file, generated →
// auto (with its automatic row), and the retired uploaded → the upload slot plus a
// local:upload decision, its state reset so the sweep extracts or generates the
// automatic slots. A seeded slot is skipped, so a second run changes nothing.
func (m *Manager) BackfillSlots(ctx context.Context) (seeded int, err error) {
	if m.cfg.Dir == "" {
		return 0, nil
	}
	rows, err := m.repo.VideosWithThumbnailImage(ctx)
	if err != nil {
		return 0, err
	}
	for _, row := range rows {
		if ctx.Err() != nil {
			return seeded, ctx.Err()
		}
		var slot string
		switch row.State {
		case model.ThumbnailEmbedded:
			slot = SlotFile
		case model.ThumbnailGenerated:
			slot = SlotAuto
		case model.ThumbnailUploaded:
			// Always redone: the state reset below is what makes it one-time, and a
			// crash between the slot write and the reset must not strand the row.
			slot = SlotUpload
		default:
			continue
		}
		if _, err := os.Stat(m.slotPath(row.ID, slot)); err == nil && slot != SlotUpload {
			continue
		}
		data, err := os.ReadFile(m.posterPath(row.ID))
		if errors.Is(err, os.ErrNotExist) {
			data, err = os.ReadFile(m.thumbPath(row.ID))
		}
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				m.log.Warn("poster slot backfill: read served pair", "id", row.ID, "err", err)
			}
			continue
		}
		if err := writeAtomic(m.slotPath(row.ID, slot), data); err != nil {
			return seeded, err
		}
		switch slot {
		case SlotAuto:
			err = m.repo.SetPosterSource(ctx, row.ID, fieldsource.LocalAutomatic, ContentHash(data), nil, false)
		case SlotUpload:
			if err = m.repo.SetPosterSource(ctx, row.ID, fieldsource.LocalUpload, ContentHash(data), nil, true); err == nil {
				err = m.repo.ResetThumbnailState(ctx, row.ID)
			}
		}
		if err != nil {
			return seeded, err
		}
		seeded++
	}
	return seeded, nil
}
