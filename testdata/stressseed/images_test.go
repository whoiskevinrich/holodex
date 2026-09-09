package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"holodex/internal/entityimage"
	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/thumbnail"
)

// The `alpha` rung's whole claim is that transparency does not survive the app's
// own ingest — a transparent-background logo is stored as an opaque black plate.
// Everything written about that rung depends on it, and it is exactly the kind of
// fact that decays silently: nothing in the fixture would look different if a
// future encoder started compositing onto white instead, but the rung would have
// quietly become a duplicate of `bright` rather than of `black`.
//
// The white RGB under the transparent pixels is the point of the assertion. If
// alpha were being ignored rather than premultiplied, this would come out white.
func TestAlphaFlattensToBlack(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			// Transparent, but explicitly white underneath.
			src.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 0})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	norm, _, _, err := personimage.Normalize(buf.Bytes(), 0)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	got, _, err := image.Decode(bytes.NewReader(norm))
	if err != nil {
		t.Fatalf("decode normalized: %v", err)
	}
	r, g, b, _ := got.At(16, 16).RGBA()
	if r>>8 > 8 || g>>8 > 8 || b>>8 > 8 {
		t.Errorf("a fully transparent white pixel normalized to rgb(%d,%d,%d), want near-black.\n"+
			"The `alpha` rung is documented as reproducing the black-plate failure a "+
			"transparent logo suffers; if this is no longer what the app does, that rung "+
			"is testing something else and its comment is wrong.", r>>8, g>>8, b>>8)
	}
}

// TestAlphaFlattensToBlack proves what the normalizer does to transparency; this
// proves the rung actually hands it transparency to do it to.
//
// Without this, someone "fixing" the odd-looking zero-alpha plate to an opaque
// black would turn the alpha rung into a byte-for-byte duplicate of `black` — the
// fixture would still seed, still serve, and still be addressed, while quietly
// testing one thing twice and the transparent-logo path not at all. That is the
// exact failure mode this epic keeps finding, and nothing else here catches it.
func TestAlphaRungActuallyStartsTransparent(t *testing.T) {
	var alpha imageVariant
	for _, v := range imagePalette {
		if v.key == "alpha" {
			alpha = v
		}
	}
	if alpha.key == "" {
		t.Fatal("no alpha rung in the palette")
	}
	if alpha.plate.A != 0 {
		t.Errorf("the alpha rung's plate has alpha %d, so nothing about it is transparent — "+
			"it is now a duplicate of the `black` rung and the transparent-logo failure "+
			"is untested", alpha.plate.A)
	}
	if alpha.ink.A == 0 {
		t.Error("the alpha rung's ink is transparent too, so the whole image flattens to a " +
			"flat black plate with no mark — the point is a mark surviving on a plate " +
			"that should not be there")
	}
}

// A `wrong` aspect that equalled its frame would make the ratio rung a no-op —
// it would seed cleanly, appear in the manifest, and assert nothing. Nothing on
// the page would say so, which is why this is checked rather than eyeballed.
func TestEveryImageSlotHasAWrongAspectUnlikeItsFrame(t *testing.T) {
	for _, kind := range []entityKind{kindVideo, kindPerson, kindStudio, kindFilm} {
		for _, slot := range slotsFor(kind) {
			if slot.frame == slot.wrong {
				t.Errorf("%s/%s declares wrong aspect %s, the same as its frame — the "+
					"ratio rung would render a correctly shaped image", kind, slot.label, slot.frame)
			}
			// Same shape by ratio rather than by struct equality: 2/3 and 4/6 are the
			// same frame written two ways, and would be just as inert.
			if slot.frame.w*slot.wrong.h == slot.wrong.w*slot.frame.h {
				t.Errorf("%s/%s wrong aspect %s reduces to its frame %s",
					kind, slot.label, slot.wrong, slot.frame)
			}
		}
	}
}

// A tag has no image slots, and no image dimension. If either half of that
// changed alone the fixture would either seed images for an entity that cannot
// show them, or address a dimension that writes nothing.
func TestOnlyPictureRenderingKindsHaveImageDimensions(t *testing.T) {
	withSlots := map[entityKind]bool{}
	for _, kind := range []entityKind{kindVideo, kindFilm, kindPerson, kindStudio, kindTag} {
		withSlots[kind] = len(slotsFor(kind)) > 0
	}
	if withSlots[kindTag] {
		t.Error("a tag has image slots, but it renders as a chip and a heading — nothing " +
			"displays a picture for one")
	}

	for _, dim := range ladder {
		if !strings.HasSuffix(dim.key, "image") {
			continue
		}
		if !withSlots[dim.entity] {
			t.Errorf("dimension %q addresses a %s, which has no image slots — every rung "+
				"would be a no-op", dim.key, dim.entity)
		}
	}
}

// Every image dimension carries the whole palette, so a rung key means the same
// thing on a person as on a film. That is what lets a crop fix found on one page
// be checked on another without translating the address, and a dimension that
// quietly dropped a rung would break it silently.
func TestImageDimensionsShareOnePalette(t *testing.T) {
	want := make([]string, 0, len(imagePalette))
	for _, v := range imagePalette {
		want = append(want, v.key)
	}

	found := 0
	for _, dim := range ladder {
		if !strings.HasSuffix(dim.key, "image") {
			continue
		}
		found++
		got := make([]string, 0, len(dim.rungs))
		for _, rg := range dim.rungs {
			got = append(got, rg.variant)
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("dimension %q carries rungs [%s], want the whole palette [%s]",
				dim.key, strings.Join(got, " "), strings.Join(want, " "))
		}
	}
	// Four kinds render pictures; a missing dimension is a whole page nothing
	// stresses, and would otherwise show up only as an absence.
	if found != 4 {
		t.Errorf("found %d image dimensions, want one each for video, film, person and studio", found)
	}
}

// imageRungs is the seeded entities of every image dimension, keyed by
// "<dimension>=<variant>", so the disk assertions below can address one rung.
func imageRungs(entries []entry) map[string]entry {
	out := map[string]entry{}
	for _, e := range entries {
		if strings.HasSuffix(e.Dimension, "image") {
			out[e.Dimension+"="+e.Variant] = e
		}
	}
	return out
}

// The three states a slot can be in are `none` (no row, no file), `missing` (row,
// no file) and everything else (row and file). They are three different failures
// on the page — an empty state, a broken-image glyph, and a rendered image — and
// the difference between the first two is invisible in the database alone.
func TestImageRungsWriteTheFilesTheyClaim(t *testing.T) {
	dir := t.TempDir()
	entries, database := seedInto(t, dir)
	targets := testTargets(dir)
	rungs := imageRungs(entries)

	if len(rungs) == 0 {
		t.Fatal("no image rungs were seeded")
	}

	for key, e := range rungs {
		files := storedFiles(t, database, targets, e)
		switch e.Variant {
		case "none":
			if len(files) != 0 {
				t.Errorf("%s: the zero rung stored %d image rows, want none — the page "+
					"should be drawing its own empty state, not requesting an image", key, len(files))
			}
			if e.Axes.Image != nil {
				t.Errorf("%s: the zero rung reported an image axis; a manifest reader would "+
					"treat it as carrying a deliberately awful image", key)
			}
		case "missing":
			if len(files) == 0 {
				t.Errorf("%s: no rows at all, so nothing references a missing file and the "+
					"broken-image path is never exercised", key)
			}
			for _, si := range files {
				if _, err := os.Stat(si.path); err == nil {
					t.Errorf("%s: %s exists, but this rung is defined by the file being "+
						"absent — the rung renders a working image and asserts nothing", key, si.path)
				}
			}
		default:
			if len(files) == 0 {
				t.Errorf("%s: stored no image at all", key)
			}
			for _, si := range files {
				if _, err := os.Stat(si.path); err != nil {
					t.Errorf("%s: %s missing: %v", key, si.path, err)
				}
			}
		}
	}
}

// A file left behind by an earlier run is the one way the `missing` rung can pass
// while showing a working image: the row is re-created, the stale file is still
// there, and the page renders. Nothing cascades to disk, so clearImages is the
// only thing standing between the fixture and that.
//
// It has to be the *video* rung, and finding that out is the reason this test is
// worth its length. A person, studio or film image lives at
// {dir}/{entity}/{imageID}.jpg, and those image ids come from an unsteered
// AUTOINCREMENT — so a re-seed writes new rows at new ids, and last run's file is
// orphaned at an address nothing references. Harmless, if untidy. A video's
// thumbnail lives at {dir}/{videoID}.jpg with no image id in it at all, and video
// ids *are* steered to be stable (D4) — so run two's `missing` rung lands on
// exactly the path run one left a working image at, and would serve it.
//
// The first version of this test used the person rung and passed with clearImages
// deleted, which is how the distinction surfaced.
func TestReseedingClearsImagesLeftByAnEarlierRun(t *testing.T) {
	dir := t.TempDir()
	entries, database := seedInto(t, dir)
	targets := testTargets(dir)

	var missing entry
	for _, e := range entries {
		if e.Dimension == "videoimage" && e.Variant == "missing" {
			missing = e
		}
	}
	if missing.ID == 0 {
		t.Fatal("no videoimage=missing rung to test against")
	}
	// Not storedFiles: that rung deliberately has no files. The path is the one the
	// *bright* rung would have written, computed the way the server computes it.
	stale := thumbnail.ThumbPath(targets.thumbnailDir, missing.ID)
	if err := os.MkdirAll(filepath.Dir(stale), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(stale, []byte("stale bytes from a previous run"), 0o644); err != nil {
		t.Fatalf("plant stale file: %v", err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("stale file did not land: %v", err)
	}
	// Windows will not let t.TempDir clean up under a live handle, and the second
	// seed opens its own.
	_ = database.Close()

	entries2, database2 := seedInto(t, dir)
	_ = database2
	for _, e := range entries2 {
		if e.Dimension == "videoimage" && e.Variant == "missing" {
			if e.ID != missing.ID {
				t.Fatalf("the missing rung moved from video %d to %d between runs, so the "+
					"stale file was planted somewhere the second run never looks — this test "+
					"is no longer testing anything", missing.ID, e.ID)
			}
			if _, err := os.Stat(stale); err == nil {
				t.Errorf("%s survived a re-seed, so the missing rung would render it "+
					"and silently pass", stale)
			}
		}
	}
}

// storedFiles is where an entity's seeded images live on disk, derived from the
// rows the seeder actually wrote rather than from the ladder — so a row written
// under the wrong role, or a file written under an id the row does not carry,
// shows up as a missing file instead of being papered over by recomputing the
// path from the same table the writer used.
//
// A video is the exception with no rows to read: its two paths come off the video
// id, and thumbnail_state is all the database records.
func storedFiles(t *testing.T, database *sql.DB, targets imageTargets, e entry) []storedImage {
	t.Helper()

	if e.Entity == kindVideo {
		// thumbnail_state is NULL until something sets it, so the zero rung reads as
		// NULL rather than as the empty string.
		var state sql.NullString
		if err := database.QueryRow(`SELECT thumbnail_state FROM videos WHERE id = ?`, e.ID).
			Scan(&state); err != nil {
			t.Fatalf("read thumbnail_state of video %d: %v", e.ID, err)
		}
		if !model.HasThumbnailImage(state.String) {
			// The API only emits a thumbnail_url for these states, so any other one
			// means the page requests nothing at all — the zero rung.
			return nil
		}
		// A video's images carry no row of their own, so the slot label is the only
		// thing naming which of the two files this is.
		return []storedImage{
			{slot: "thumb", path: thumbnail.ThumbPath(targets.thumbnailDir, e.ID)},
			{slot: "poster", path: thumbnail.PosterPath(targets.thumbnailDir, e.ID)},
		}
	}

	var table, fk, dir string
	switch e.Entity {
	case kindPerson:
		table, fk, dir = "person_images", "person_id", targets.personDir
	case kindStudio:
		table, fk, dir = "studio_images", "studio_id", targets.studioDir
	case kindFilm:
		table, fk, dir = "film_images", "film_id", targets.filmDir
	default:
		t.Fatalf("no image table for entity kind %q", e.Entity)
	}

	rows, err := database.Query(
		fmt.Sprintf(`SELECT id, role FROM %s WHERE %s = ? ORDER BY id`, table, fk), e.ID)
	if err != nil {
		t.Fatalf("list %s of %d: %v", table, e.ID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []storedImage
	for rows.Next() {
		var imageID int64
		var role string
		if err := rows.Scan(&imageID, &role); err != nil {
			t.Fatalf("scan %s: %v", table, err)
		}
		// entityimage.Path is what all three wrappers delegate to, so this is the
		// same arithmetic the writer used without going through the writer.
		out = append(out, storedImage{role: role, path: entityimage.Path(dir, e.ID, imageID)})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate %s: %v", table, err)
	}
	return out
}

// storedImage is one seeded image on disk, tagged with enough to say which slot
// it belongs to. Person and studio rows carry a role; a video's two files are not
// rows at all, so they are tagged by slot label instead.
type storedImage struct {
	role string
	slot string
	path string
}

// matches reports whether this stored image belongs to the given slot. A person's
// gallery seeds several tiles under one role, so this is one-to-many by design —
// which is exactly the assumption an index-paired version of this got wrong.
func (si storedImage) matches(slot imageSlot) bool {
	if slot.role != "" {
		return si.role == slot.role
	}
	return si.slot == slot.label
}

// The ratio rung is a claim about *stored* pixels, not about intent: the bytes go
// through the same normalizer the app uses, which downscales to each kind's
// configured maximum. A studio's 1000px cap in particular reshapes a 1400x600
// ultrawide, so this asserts the aspect that actually survives — the number a
// geometry assertion will be comparing the rendered box against.
func TestRatioRungStoresAnAspectUnlikeTheFrame(t *testing.T) {
	dir := t.TempDir()
	entries, database := seedInto(t, dir)
	targets := testTargets(dir)

	checked := 0
	for _, e := range imageRungs(entries) {
		if e.Variant != "ratio" || e.Entity == kindVideo {
			continue // a video's files carry no row to name the slot they belong to
		}
		files := storedFiles(t, database, targets, e)
		for _, slot := range slotsFor(e.Entity) {
			seen := 0
			for _, si := range files {
				if !si.matches(slot) {
					continue
				}
				seen++
				cfg, _, err := imageConfig(si.path)
				if err != nil {
					t.Errorf("%s: %v", si.path, err)
					continue
				}
				stored := float64(cfg.Width) / float64(cfg.Height)
				frame := float64(slot.frame.w) / float64(slot.frame.h)
				// A tenth is far tighter than any of the wrong aspects (the closest pair
				// is 8/3 against 2/3) and far looser than the integer rounding the
				// downscale to each kind's configured maximum introduces.
				if math.Abs(stored-frame) < 0.1 {
					t.Errorf("%s/%s ratio rung stored %dx%d (%.2f), indistinguishable from its "+
						"own frame %s (%.2f) — the rung renders correctly and finds nothing",
						e.Dimension, slot.label, cfg.Width, cfg.Height, stored, slot.frame, frame)
				}
				checked++
			}
			if seen == 0 {
				t.Errorf("%s %d stored nothing for its %s slot", e.Entity, e.ID, slot.label)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no ratio-rung images were checked")
	}
}

// The monogram is the ticket's own requirement — every fixture image identifies
// its entity and variant — and it is the half of the fixture that cannot be
// asserted by looking at the page, because it is burned into pixels. What can be
// asserted is that the label being drawn carries both, and the pixel size the
// ratio rung is entirely about.
func TestMonogramNamesItsEntityVariantAndSize(t *testing.T) {
	for _, kind := range []entityKind{kindVideo, kindPerson, kindStudio, kindFilm} {
		for _, slot := range slotsFor(kind) {
			for _, v := range imagePalette {
				if v.absent {
					continue // nothing is drawn
				}
				lines := strings.Join(monogram(kind, 12345, slot, v, 0, 0, 1), " ")
				for _, want := range []string{
					strings.ToUpper(string(kind)), "12345", strings.ToUpper(v.key), slot.label,
				} {
					if !strings.Contains(lines, want) {
						t.Errorf("%s/%s/%s monogram %q omits %q", kind, slot.label, v.key, lines, want)
					}
				}
				// The size line is what makes a screenshot of a cropped image
				// self-describing: "600x600 in a 2/3 frame" is the whole bug report.
				if !strings.Contains(lines, "x") || !strings.Contains(lines, "/") {
					t.Errorf("%s/%s/%s monogram %q states no pixel size and aspect",
						kind, slot.label, v.key, lines)
				}
			}
		}
	}
}

// The monogram states a pixel size, and a stated size that disagrees with the file
// is worse than none — it is a wrong answer burned into an image, in the one place
// a reader cannot check it against anything.
//
// This is a regression test. The first version computed the label before the
// downscale, so a studio's 1000px cap stored the ratio rung at 1000x428 under a
// monogram reading "1400x600" forever.
func TestMonogramStatesTheSizeActuallyStored(t *testing.T) {
	for _, kind := range []entityKind{kindVideo, kindPerson, kindStudio, kindFilm} {
		// The tightest cap in the app, so the downscale definitely bites.
		const maxDim = 1000
		for _, slot := range slotsFor(kind) {
			for _, v := range imagePalette {
				if v.absent {
					continue
				}
				label := monogram(kind, 1, slot, v, maxDim, 0, 1)
				norm, w, h, err := renderNormalized(slot, v, label, maxDim)
				if err != nil {
					t.Fatalf("%s/%s/%s: %v", kind, slot.label, v.key, err)
				}
				want := fmt.Sprintf("%dx%d", w, h)
				if !strings.Contains(strings.Join(label, " "), want) {
					t.Errorf("%s/%s/%s stored %s but its monogram reads %q",
						kind, slot.label, v.key, want, strings.Join(label, " "))
				}
				if len(norm) == 0 {
					t.Errorf("%s/%s/%s normalized to zero bytes", kind, slot.label, v.key)
				}
			}
		}
	}
}

// The degenerate rung has to stay legible enough to identify, or it stops being a
// picture of something small and becomes a picture of nothing. drawLabel drops
// lines that do not fit rather than overflowing, so the assertion is that at least
// one still lands: a 32px image with no ink at all would be indistinguishable from
// a plate of flat colour.
func TestDegenerateRungStillDrawsSomething(t *testing.T) {
	var tiny imageVariant
	for _, v := range imagePalette {
		if v.key == "tiny" {
			tiny = v
		}
	}
	if tiny.key == "" {
		t.Fatal("no tiny rung in the palette")
	}

	slot := slotsFor(kindPerson)[0]
	norm, w, h, err := renderNormalized(slot, tiny, monogram(kindPerson, 1, slot, tiny, 2000, 0, 1), 2000)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if w > imageTinyEdge || h > imageTinyEdge {
		t.Errorf("the degenerate rung rendered %dx%d, which is not degenerate against a "+
			"%dpx frame", w, h, imageShortEdge)
	}
	img, _, err := image.Decode(bytes.NewReader(norm))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	plate := tiny.plate
	inked := 0
	for y := range h {
		for x := range w {
			r, g, b, _ := img.At(x, y).RGBA()
			// JPEG is lossy, so "not the plate colour" needs a margin rather than an
			// equality check.
			if abs(int(r>>8)-int(plate.R))+abs(int(g>>8)-int(plate.G))+abs(int(b>>8)-int(plate.B)) > 60 {
				inked++
			}
		}
	}
	if inked == 0 {
		t.Error("the 32px rung is a flat plate with no monogram at all, so nothing on the " +
			"page says which entity or variant it is")
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func imageConfig(path string) (image.Config, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, "", err
	}
	defer func() { _ = f.Close() }()
	return image.DecodeConfig(f)
}
