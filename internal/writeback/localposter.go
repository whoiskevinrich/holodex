package writeback

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"holodex/internal/fieldsource"
	"holodex/internal/thumbnail"
)

// A local poster source is embedded from its slot, never downloaded (F85,
// writeback.md). An image FieldWrite whose value is a local:<kind>#<sha256> token
// is staged by StageLocalPosters before WriteBatch: the job's own slot file is
// copied to a temp file, and the write takes the same cover path a downloaded
// image takes. The slot path is built from config (SetLocalPosterDir), the job's
// video id and the closed kind set — never from the payload, which only names a
// kind — and the guarded image fetcher is never called (security-perimeter.md).

// ErrLocalPosterNotHeld is a local poster write whose slot is empty: the job fails
// and no other image is substituted.
var ErrLocalPosterNotHeld = errors.New("local poster not held")

// localPosterDir is DATA_PATH/thumbnails, wired once at startup. Left "", every
// local poster write is refused.
var localPosterDir string

// SetLocalPosterDir wires the poster slot directory (thumbnail.Config.Dir).
func SetLocalPosterDir(dir string) { localPosterDir = dir }

// StageLocalPosters stages every local-token image write in batch for videoID:
// each such entry's value becomes a temp copy of the slot, and written maps its
// index to the token of the bytes actually copied — what the write ledger must
// record, so a slot replaced between enqueue and write reads out of sync rather
// than falsely in sync. An unknown token or an empty slot fails the whole batch;
// no other image is substituted. cleanup removes the temp copies and is always
// safe to call.
func StageLocalPosters(videoID int64, batch []FieldWrite) (written map[int]string, cleanup func(), err error) {
	var temps []string
	cleanup = func() {
		for _, p := range temps {
			_ = os.Remove(p)
		}
	}
	for i := range batch {
		f := &batch[i]
		if !f.IsImage || len(f.Values) == 0 || !strings.HasPrefix(f.Values[0], "local:") {
			continue
		}
		kind, _, ok := fieldsource.ParseLocalToken(f.Values[0])
		slot, known := thumbnail.SlotForKind(kind)
		if !ok || !known {
			cleanup()
			return nil, func() {}, fmt.Errorf("cover image: unknown local poster %q", f.Values[0])
		}
		if localPosterDir == "" {
			cleanup()
			return nil, func() {}, errors.New("cover image: no local poster directory configured")
		}
		data, err := os.ReadFile(thumbnail.SlotPath(localPosterDir, videoID, slot))
		if err == nil && len(data) == 0 {
			err = os.ErrNotExist
		}
		if err != nil {
			cleanup()
			if errors.Is(err, os.ErrNotExist) {
				return nil, func() {}, fmt.Errorf("cover image: no %s poster: %w", kind, ErrLocalPosterNotHeld)
			}
			return nil, func() {}, fmt.Errorf("cover image: read %s poster: %w", kind, err)
		}
		tmp, err := writeTempImage(data)
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		temps = append(temps, tmp)
		if written == nil {
			written = map[int]string{}
		}
		written[i] = fieldsource.LocalToken(kind, thumbnail.ContentHash(data))
		f.Values = []string{tmp}
		f.staged = true
	}
	return written, cleanup, nil
}

// imageFile returns the local file an image FieldWrite embeds: its staged slot
// copy, or the provider URL downloaded through the guarded fetcher. The caller
// must call cleanup.
func imageFile(ctx context.Context, f FieldWrite) (path string, cleanup func(), err error) {
	if f.staged {
		return f.Values[0], func() {}, nil // StageLocalPosters' cleanup owns it
	}
	return downloadImageToTemp(ctx, f.Values[0])
}
