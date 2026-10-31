package thumbnail

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"testing"

	"holodex/internal/fieldsource"
	"holodex/internal/model"
)

func writeSlot(t *testing.T, m *Manager, id int64, slot, data string) {
	t.Helper()
	if err := os.WriteFile(m.slotPath(id, slot), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestPresentFollowsDecision is G5: undecided serves file, else auto; a local
// decision serves its slot; a file or provider decision serves file; a missing
// chosen slot falls through; with no slot the pair is removed.
func TestPresentFollowsDecision(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRepo()
	m := testManager(t, fr)
	const id = 4
	present := func() string {
		t.Helper()
		if err := m.Present(ctx, id); err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(m.posterPath(id))
		if os.IsNotExist(err) {
			return "<none>"
		}
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	if got := present(); got != "<none>" {
		t.Fatalf("no slots: %q", got)
	}
	writeSlot(t, m, id, SlotAuto, "AUTO")
	if got := present(); got != "AUTO" {
		t.Fatalf("auto only: %q", got)
	}
	writeSlot(t, m, id, SlotFile, "FILE")
	if got := present(); got != "FILE" {
		t.Fatalf("undecided with art: %q", got)
	}
	fr.decide(id, fieldsource.ForLocal(fieldsource.LocalAutomatic))
	if got := present(); got != "AUTO" {
		t.Fatalf("local:automatic: %q", got)
	}
	fr.decide(id, fieldsource.ForLocal(fieldsource.LocalFrame))
	if got := present(); got != "FILE" {
		t.Fatalf("local:frame with no frame slot should fall through to file: %q", got)
	}
	writeSlot(t, m, id, SlotFrame, "FRAME")
	if got := present(); got != "FRAME" {
		t.Fatalf("local:frame: %q", got)
	}
	for _, src := range []string{fieldsource.File, fieldsource.ForProvider("provider-1")} {
		fr.decide(id, src)
		if got := present(); got != "FILE" {
			t.Fatalf("%s decision: %q", src, got)
		}
	}
	if got := readString(t, m.thumbPath(id)); got != "FILE" {
		t.Fatalf("thumb tier = %q, want the file slot", got)
	}
}

// TestPresentLeavesUnchangedPair is G5's last case: presenting the same winning
// bytes again must not rewrite the pair, so its ?v= token stays put.
func TestPresentLeavesUnchangedPair(t *testing.T) {
	ctx := context.Background()
	m := testManager(t, newFakeRepo())
	writeSlot(t, m, 1, SlotFile, "FILE")
	if err := m.Present(ctx, 1); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(m.posterPath(1))
	if err := m.Present(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.Stat(m.posterPath(1)); !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("unchanged poster was rewritten")
	}
}

// TestOwnerImagesKeepNewestOfEach is G6 (P0-8): a new frame replaces the frame
// slot and leaves the upload alone, and the reverse; each row carries the newest
// hash, and the frame its time.
func TestOwnerImagesKeepNewestOfEach(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRepo()
	m := testManager(t, fr)
	at1, at2 := 12.5, 760.0
	steps := []struct {
		kind, data string
		at         *float64
	}{
		{fieldsource.LocalUpload, "UP1", nil},
		{fieldsource.LocalFrame, "FR1", &at1},
		{fieldsource.LocalFrame, "FR2", &at2},
	}
	for _, s := range steps {
		if err := m.SetOwnerImage(ctx, 2, s.kind, []byte(s.data), s.at); err != nil {
			t.Fatal(err)
		}
	}
	if got := readString(t, m.slotPath(2, SlotUpload)); got != "UP1" {
		t.Fatalf("upload slot = %q", got)
	}
	if got := readString(t, m.slotPath(2, SlotFrame)); got != "FR2" {
		t.Fatalf("frame slot = %q", got)
	}
	if ps, _ := fr.source(2, fieldsource.LocalFrame); ps.Hash != ContentHash([]byte("FR2")) || ps.FrameAt == nil || *ps.FrameAt != at2 {
		t.Fatalf("frame row = %+v", ps)
	}
	if got := readString(t, m.posterPath(2)); got != "FR2" {
		t.Fatalf("served = %q, want the newest pick", got)
	}
	if d, _ := fr.PosterDecision(ctx, 2); d != "local:frame" {
		t.Fatalf("decision = %q", d)
	}
	if err := m.SetOwnerImage(ctx, 2, fieldsource.LocalUpload, []byte("UP2"), &at1); err != nil {
		t.Fatal(err)
	}
	if got := readString(t, m.slotPath(2, SlotFrame)); got != "FR2" {
		t.Fatalf("an upload touched the frame slot: %q", got)
	}
	if ps, _ := fr.source(2, fieldsource.LocalUpload); ps.FrameAt != nil {
		t.Fatal("an upload recorded a frame time")
	}
}

// TestRemoveOwnerImage is the thumbnail half of G9 (P0-9).
func TestRemoveOwnerImage(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRepo()
	m := testManager(t, fr)
	writeSlot(t, m, 3, SlotFile, "FILE")
	if err := m.SetOwnerImage(ctx, 3, fieldsource.LocalUpload, []byte("UP"), nil); err != nil {
		t.Fatal(err)
	}
	cleared, err := m.RemoveOwnerImage(ctx, 3, fieldsource.LocalUpload)
	if err != nil || !cleared {
		t.Fatalf("remove chosen upload = (%v, %v)", cleared, err)
	}
	if d, _ := fr.PosterDecision(ctx, 3); d != "" {
		t.Fatalf("decision left behind: %q", d)
	}
	if got := readString(t, m.posterPath(3)); got != "FILE" {
		t.Fatalf("served = %q, want the fallback", got)
	}
	// An upload that isn't the choice just goes.
	if err := m.SetOwnerImage(ctx, 3, fieldsource.LocalUpload, []byte("UP"), nil); err != nil {
		t.Fatal(err)
	}
	fr.decide(3, fieldsource.File)
	if cleared, err = m.RemoveOwnerImage(ctx, 3, fieldsource.LocalUpload); err != nil || cleared {
		t.Fatalf("remove undecided upload = (%v, %v)", cleared, err)
	}
	if d, _ := fr.PosterDecision(ctx, 3); d != fieldsource.File {
		t.Fatalf("decision changed: %q", d)
	}
	if _, err := m.RemoveOwnerImage(ctx, 3, fieldsource.LocalAutomatic); err == nil {
		t.Fatal("removing the automatic slot is not an owner action")
	}
}

// TestBackfillSlots is G12: each served pair seeds its slot by state, an upload
// becomes the decision with its state reset, and a second run changes nothing.
func TestBackfillSlots(t *testing.T) {
	ctx := context.Background()
	fr := newFakeRepo()
	m := testManager(t, fr)
	for id, st := range map[int64]string{1: model.ThumbnailEmbedded, 2: model.ThumbnailGenerated, 3: model.ThumbnailUploaded} {
		for _, p := range []string{m.thumbPath(id), m.posterPath(id)} {
			if err := os.WriteFile(p, []byte(st), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		_ = fr.SetThumbnailState(ctx, id, st)
	}
	n, err := m.BackfillSlots(ctx)
	if err != nil || n != 3 {
		t.Fatalf("backfill = (%d, %v)", n, err)
	}
	if got := readString(t, m.slotPath(1, SlotFile)); got != model.ThumbnailEmbedded {
		t.Fatalf("file slot = %q", got)
	}
	if _, ok := fr.source(2, fieldsource.LocalAutomatic); !ok {
		t.Fatal("generated row has no automatic source")
	}
	if got := readString(t, m.slotPath(3, SlotUpload)); got != model.ThumbnailUploaded {
		t.Fatalf("upload slot = %q", got)
	}
	if d, _ := fr.PosterDecision(ctx, 3); d != "local:upload" {
		t.Fatalf("upload decision = %q", d)
	}
	if fr.state(3) != "" {
		t.Fatalf("uploaded state not reset: %q", fr.state(3))
	}
	if n, err := m.BackfillSlots(ctx); err != nil || n != 0 {
		t.Fatalf("second backfill = (%d, %v)", n, err)
	}
}

func TestThumbTierScalesWideSlot(t *testing.T) {
	m := testManager(t, newFakeRepo())
	img := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	img.Set(0, 0, color.White)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	out, err := m.thumbTier(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(out))
	if err != nil || cfg.Width != 400 || cfg.Height != 200 {
		t.Fatalf("thumb tier = %dx%d, %v", cfg.Width, cfg.Height, err)
	}
}
