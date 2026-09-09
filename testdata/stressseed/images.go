package main

// The adversarial image set (HOLODEX-345, spec D8).
//
// Every entity kind that renders an image gets a dimension whose rungs vary one
// axis: what the image *is*. Brightness is one rung of seven, not the whole set —
// `app.css` hardcodes 1/1 headshot, 2/3 poster and 8/3 banner frames and
// `web/src/lib/cropGeometry.ts` is keyed to those same numbers, so an image whose
// aspect disagrees with its frame is as much a bug source as one the overlaid
// text cannot be read against.
//
// Two things about this file are load-bearing and non-obvious.
//
// **The bytes go through the app's own normalizer.** personimage.Normalize is
// what every production ingest path runs — upload, enrichment, promote — and it
// decodes, bomb-guards, downscales to the kind's configured maximum and re-encodes
// to JPEG at quality 85. Writing hand-rolled bytes straight to disk would let the
// fixture display images the running app cannot produce, which is the same lie
// HOLODEX-346 refused for the empty tag name. It also means the stored width and
// height are whatever Normalize decided, not whatever this file asked for, so the
// row and the file cannot drift.
//
// **Transparency does not survive, and that is the finding rather than a
// limitation.** JPEG has no alpha channel and Go's encoder reads a pixel through
// color.Color.RGBA(), which is alpha-premultiplied — so a fully transparent pixel
// arrives as 0,0,0 whatever colour sits under it. A transparent-background logo is
// therefore stored as an opaque BLACK plate with the mark still on it. See the
// `alpha` rung for where that is visible and where it collapses into `black`.

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"holodex/internal/imagesink"
	"holodex/internal/model"
	"holodex/internal/personimage"
	"holodex/internal/repo"
	"holodex/internal/thumbnail"
)

const (
	// imageShortEdge sizes a well-formed fixture image. It is CROP_SHORT_EDGE from
	// web/src/lib/cropGeometry.ts, which is what the in-app crop editor produces —
	// so a fixture image is the size a real one would be, and the degenerate rung
	// below is degenerate relative to the real thing rather than to a number picked
	// here.
	imageShortEdge = 600

	// imageTinyEdge is the degenerate rung: far too few pixels for any frame, so
	// the browser upscales it and the result is visibly mushy. 32 is the ticket's
	// number and is well under every frame's short edge.
	imageTinyEdge = 32

	// galleryTiles is how many 'extra' images a person's gallery rung seeds. More
	// than one so the gallery renders as a grid rather than as a single tile — the
	// tiles all carry the same variant, so this is the text dimension's argument
	// rather than a second axis: one knob reaching a second container.
	galleryTiles = 3
)

// aspect is a frame's shape, written the way app.css writes it so the two can be
// compared by eye.
type aspect struct{ w, h int }

func (a aspect) String() string { return fmt.Sprintf("%d/%d", a.w, a.h) }

// size renders the aspect at a given short edge, which is how a real image for
// this frame would be produced.
func (a aspect) size(shortEdge int) (int, int) {
	if a.w >= a.h {
		return shortEdge * a.w / a.h, shortEdge
	}
	return shortEdge, shortEdge * a.h / a.w
}

// The frames the app actually renders. Kept as named constants rather than
// literals at each use so a change in app.css has one place to land here.
var (
	square    = aspect{1, 1}  // .portrait-frame--1x1 — headshot, studio icon and logo
	portrait  = aspect{2, 3}  // .portrait-frame--2x3 — poster, everywhere
	banner    = aspect{8, 3}  // .portrait-frame--banner — person and film banners
	landscape = aspect{16, 9} // .video-frame — the video grid's default layout

	// ultrawide is not a frame the app has. It is the wrong-ratio rung's answer for
	// anything squarish or taller than it is wide: extreme enough that a cover crop
	// throws most of the image away and a contain fit letterboxes it to a sliver.
	ultrawide = aspect{21, 9}
)

// imageSlot is one image an entity kind carries: where it is stored, the frame the
// UI renders it in, and therefore what "wrong ratio" means for it.
//
// wrong is declared per slot rather than computed, because "unlike this frame" has
// no general definition — the answer for a 1/1 headshot is a 21/9 letterbox, and
// the answer for an 8/3 banner is a 2/3 portrait. Both are in the ticket by name.
type imageSlot struct {
	// role is the DB role the row carries. Empty for a video, whose two images are
	// addressed by path off the video id and recorded only as a thumbnail_state.
	role string

	// label names the slot in the monogram, so a screenshot says which slot it is.
	label string

	frame aspect
	wrong aspect

	// contain is true where the UI letterboxes the image inside its box instead of
	// cropping to fill it. It changes which rungs are visible: a black plate under
	// object-cover is indistinguishable from any other black plate, while under
	// object-contain it is a black rectangle floating in a themed well — which is
	// exactly how a transparent logo fails. Recorded so the manifest can say so.
	contain bool
}

// slotsFor is the image surface of one entity kind. A tag has none: it renders as
// a chip and a heading, never as a picture.
func slotsFor(kind entityKind) []imageSlot {
	switch kind {
	case kindVideo:
		// Two files, one state field. The grid frames a video at 16/9 by default and
		// at 2/3 under card_layout=poster, so neither is "the" frame — the wrong-ratio
		// answer is a square, which is wrong under both.
		return []imageSlot{
			{label: "thumb", frame: landscape, wrong: square},
			{label: "poster", frame: portrait, wrong: square},
		}
	case kindPerson:
		return []imageSlot{
			{role: model.PersonImageHeadshot, label: "headshot", frame: square, wrong: ultrawide},
			{role: model.PersonImageBanner, label: "banner", frame: banner, wrong: portrait},
			{role: model.PersonImagePoster, label: "poster", frame: portrait, wrong: square},
			{role: model.PersonImageExtra, label: "gallery", frame: portrait, wrong: ultrawide},
		}
	case kindStudio:
		// EntityImageSlot defaults to fit='contain' and the studios list uses
		// object-contain too, because a logo cropped to fill its box is a logo with
		// its edges cut off. That makes the studio the one kind where the alpha rung
		// shows something the black rung does not.
		return []imageSlot{
			{role: model.StudioImageLogo, label: "logo", frame: square, wrong: ultrawide, contain: true},
			{role: model.StudioImageIcon, label: "icon", frame: square, wrong: ultrawide, contain: true},
			{role: model.StudioImagePoster, label: "poster", frame: portrait, wrong: square, contain: true},
		}
	case kindFilm:
		return []imageSlot{
			{role: model.FilmImagePoster, label: "poster", frame: portrait, wrong: square},
			{role: model.FilmImageBanner, label: "banner", frame: banner, wrong: portrait, contain: true},
		}
	default:
		return nil
	}
}

// imageVariant is one rung of an image dimension: a treatment applied to every
// slot the entity kind has.
//
// Applying it to every slot at once is one axis, not several — the same reasoning
// the text dimension uses for torturing a title and an overview together. A rung
// that made the headshot bright and the banner black would be a cross-product, and
// a failure on that page could not be attributed to either.
type imageVariant struct {
	// key addresses the rung, in names, the monogram and the manifest.
	key string

	// value describes the bytes for a manifest reader. It is empty for the zero
	// rung and only for the zero rung: hasEmptyRung reads it, because "no image"
	// and "no title" are the same idea in two types (D2).
	value string

	plate color.NRGBA // the ground the monogram sits on
	ink   color.NRGBA // the monogram itself

	// useWrong swaps each slot's frame for its wrong aspect.
	useWrong bool

	// shortEdge overrides imageShortEdge. Zero means the well-formed size.
	shortEdge int

	// rowOnly writes the database row and no file: the referenced-but-missing
	// asset. It is a different state from absent, and the two fail differently —
	// see the rung's comment in the palette.
	rowOnly bool

	// absent writes neither a row nor a file. The zero rung.
	absent bool
}

// imageNone is D2's zero rung, and also the baseline every other dimension's
// entities carry (see baseline()). It is declared apart from the palette because
// two places name it and neither should do so by position.
//
// It is not the same state as `missing`: here nothing claims an image exists, so
// the UI draws its own empty state — a monogram plate for a studio or film, a
// themed placeholder SVG for a person, a play glyph for a video — and no request
// is made at all.
var imageNone = imageVariant{key: "none", value: "", absent: true}

// imagePalette is the shared set of adversarial images, in the order they are
// addressed. Every image dimension carries all of it, so a rung means the same
// thing on a person as on a film and a fix can be checked across kinds.
var imagePalette = []imageVariant{
	imageNone,

	// The ticket's original instinct, kept: near-white rather than pure white, so
	// it is a plausible photograph rather than obviously synthetic. Any white text
	// the page lays over an image is unreadable here.
	{key: "bright", value: "near-white plate",
		plate: color.NRGBA{R: 250, G: 249, B: 244, A: 255},
		ink:   color.NRGBA{R: 60, G: 58, B: 52, A: 255}},

	// The opposite failure, and on this app the more dangerous one. All three skins
	// are dark grounds (verified: every one computes a near-black body background),
	// so a black plate does not read as broken — it reads as absent. An image that
	// disappears into the page is harder to notice than one that shouts, which is
	// why this rung is not merely `bright` inverted.
	{key: "black", value: "pure black plate",
		plate: color.NRGBA{A: 255},
		ink:   color.NRGBA{R: 150, G: 150, B: 150, A: 255}},

	// The rung the ticket added on top of brightness. A 1/1 poster and a 21/9
	// headshot are both in the AC by name; slotsFor decides which is which, since
	// "wrong" is only meaningful against a particular frame.
	{key: "ratio", value: "aspect deliberately unlike the frame",
		plate:    color.NRGBA{R: 74, G: 96, B: 128, A: 255},
		ink:      color.NRGBA{R: 240, G: 240, B: 245, A: 255},
		useWrong: true},

	// Degenerate resolution. The frame is right, there are simply almost no pixels
	// in it, so the browser upscales and the result is mush.
	{key: "tiny", value: fmt.Sprintf("%dpx upscaled into the frame", imageTinyEdge),
		plate:     color.NRGBA{R: 120, G: 92, B: 40, A: 255},
		ink:       color.NRGBA{R: 250, G: 244, B: 230, A: 255},
		shortEdge: imageTinyEdge},

	// A transparent-background PNG — a logo, the way a logo is actually shipped —
	// put through the app's own normalizer rather than written to disk raw.
	//
	// It does not stay transparent, and that is the point. JPEG has no alpha and Go
	// premultiplies on the way in, so every transparent pixel is stored as pure
	// black no matter what colour sits under it (proven by TestAlphaFlattensToBlack).
	// So this rung renders as a coloured mark on a black plate.
	//
	// Where that is visible: any object-contain slot — the studio logo and icon, the
	// film banner — where the plate does not fill its box and reads as a black
	// rectangle floating in a themed well. Under object-cover it fills the frame and
	// is honestly hard to tell from `black`; it is kept there anyway so one rung key
	// means one thing on every kind, which is what lets a fix be checked across them.
	//
	// Note the failure is quieter than it sounds. Every skin is a dark ground, so the
	// plate does not glare — a transparent logo mostly *vanishes*, leaving its mark
	// floating. Look for the missing plate, not for a black box.
	{key: "alpha", value: "transparent PNG, flattened to black by the JPEG re-encode",
		plate: color.NRGBA{}, // fully transparent
		ink:   color.NRGBA{R: 235, G: 72, B: 72, A: 255}},

	// Referenced but not there: the row says an image exists, the file does not.
	// Reachable in the real thing by deleting a file out from under the database, or
	// by a half-failed ingest.
	//
	// The three kinds fail differently, which is why this is worth a rung rather
	// than an assumption. EntityImageSlot (studio, film) has no error handler at
	// all, so the browser draws its broken-image glyph. PersonImageFrame hides the
	// img on error, leaving an empty well. VideoCard retries five times with backoff
	// before falling back to the play glyph, so this is also the only rung that
	// makes the page issue failing requests in a loop.
	{key: "missing", value: "row present, file absent",
		plate:   color.NRGBA{R: 128, G: 128, B: 128, A: 255},
		ink:     color.NRGBA{A: 255},
		rowOnly: true},
}

// imageTargets is where each kind's images are written and how far the app would
// downscale them. The maxima are the server's own configured values rather than a
// number chosen here: passing a different one would store a different size than
// the running app stores, and the wrong-ratio rung is a claim about stored size.
type imageTargets struct {
	thumbnailDir string
	personDir    string
	studioDir    string
	filmDir      string

	personMaxDim int
	studioMaxDim int
	filmMaxDim   int
}

// dirFor is the asset root for a kind, and the maximum dimension its ingest path
// would downscale to. A video poster goes through the person maximum in the real
// app too (internal/api/video_poster.go), which is why it is not a fourth number.
func (t imageTargets) dirFor(kind entityKind) (string, int) {
	switch kind {
	case kindVideo:
		return t.thumbnailDir, t.personMaxDim
	case kindPerson:
		return t.personDir, t.personMaxDim
	case kindStudio:
		return t.studioDir, t.studioMaxDim
	case kindFilm:
		return t.filmDir, t.filmMaxDim
	default:
		return "", 0
	}
}

// clearImages removes every asset root the fixture writes into, so a run starts
// from no files at all.
//
// This is not tidiness. The `missing` rung is *defined* by a file's absence, so a
// file left behind by an earlier run — one whose rung has since changed variant, or
// been deleted — would make that rung silently pass while showing a working image.
// The database half is already handled by reset(): every image table cascades from
// the entity tables it clears.
func clearImages(t imageTargets) error {
	for _, dir := range []string{t.thumbnailDir, t.personDir, t.studioDir, t.filmDir} {
		if dir == "" {
			continue
		}
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("clear image dir %s: %w", dir, err)
		}
	}
	return nil
}

// imageWriter is the repo and the asset roots the image rungs write through,
// bundled so the per-slot helpers do not each take five arguments. The context
// stays a parameter rather than a field, as it should.
type imageWriter struct {
	repo    *repo.Repo
	targets imageTargets
}

// seed gives one addressed entity every image its kind renders, all carrying the
// same variant. It is a no-op for the zero rung and for kinds with no images,
// which is what lets generate() call it unconditionally after every rung.
func (iw imageWriter) seed(ctx context.Context, kind entityKind, id int64, v imageVariant) error {
	if v.absent {
		return nil
	}
	slots := slotsFor(kind)
	if len(slots) == 0 {
		return nil
	}
	dir, maxDim := iw.targets.dirFor(kind)
	if dir == "" {
		return fmt.Errorf("no asset directory for entity kind %q", kind)
	}

	for _, slot := range slots {
		tiles := 1
		if kind == kindPerson && slot.role == model.PersonImageExtra {
			tiles = galleryTiles
		}
		for tile := range tiles {
			label := monogram(kind, id, slot, v, maxDim, tile, tiles)
			norm, w, h, err := renderNormalized(slot, v, label, maxDim)
			if err != nil {
				return fmt.Errorf("%s %d %s: %w", kind, id, slot.label, err)
			}
			if err := iw.store(ctx, kind, id, dir, slot, v, norm, w, h); err != nil {
				return fmt.Errorf("%s %d %s: %w", kind, id, slot.label, err)
			}
		}
	}
	return nil
}

// geometry is the single answer to "what shape and how many pixels is this rung's
// image for this slot". The renderer and the monogram burned into it both read it,
// and they must: a size stated in one place and drawn in another is a label that
// eventually lies, and this label's whole job is to be believed.
//
// It applies the kind's downscale cap here rather than leaving it to Normalize.
// The stored file is identical either way — Normalize leaves an image already
// inside the cap alone — but only this way does the number in the monogram match
// the number on disk. A studio's 1000px maximum turns the ratio rung's 1400x600
// into 1000x428; a label computed before that would be burned into the image
// claiming 1400x600 forever, which is exactly the fact it exists to report.
func (v imageVariant) geometry(slot imageSlot, maxDim int) (aspect, int, int) {
	frame := slot.frame
	if v.useWrong {
		frame = slot.wrong
	}
	short := v.shortEdge
	if short == 0 {
		short = imageShortEdge
	}
	w, h := frame.size(short)
	if maxDim > 0 && (w > maxDim || h > maxDim) {
		if w >= h {
			w, h = maxDim, h*maxDim/w
		} else {
			w, h = w*maxDim/h, maxDim
		}
	}
	return frame, max(w, 1), max(h, 1)
}

// renderNormalized draws the variant's source image and puts it through the same
// normalizer every production ingest uses, returning the bytes and the dimensions
// as *stored* — which is what the database row has to record.
func renderNormalized(slot imageSlot, v imageVariant, label []string, maxDim int) ([]byte, int, int, error) {
	_, w, h := v.geometry(slot, maxDim)

	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(src, src.Bounds(), image.NewUniform(v.plate), image.Point{}, draw.Src)
	drawLabel(src, label, v.ink)

	// PNG rather than JPEG as the source container, because the alpha rung needs a
	// format that can carry alpha as far as the normalizer — which is exactly where
	// the fixture wants to observe it being thrown away.
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		return nil, 0, 0, fmt.Errorf("encode source png: %w", err)
	}
	norm, gotW, gotH, err := personimage.Normalize(buf.Bytes(), maxDim)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("normalize: %w", err)
	}
	return norm, gotW, gotH, nil
}

// store writes one slot's row and, unless the rung is the missing one, its file.
// Each kind goes in through the same call the app uses, so the fixture exercises
// the real write path rather than a parallel one.
func (iw imageWriter) store(ctx context.Context, kind entityKind, id int64, dir string, slot imageSlot, v imageVariant, norm []byte, w, h int) error {
	switch kind {
	case kindVideo:
		return iw.storeVideo(ctx, id, dir, slot, v, norm)
	case kindPerson:
		return iw.storePerson(ctx, id, dir, slot, v, norm, w, h)
	case kindStudio:
		in := repo.StudioImageInsert{StudioID: id, Role: slot.role, Source: model.StudioImageSourceUpload}
		if v.rowOnly {
			in.Width, in.Height, in.ByteSize = w, h, len(norm)
			_, err := iw.repo.ReplaceStudioImage(ctx, in)
			return err
		}
		_, err := imagesink.ReplaceStudioImageFile(ctx, iw.repo, dir, in, norm, w, h)
		return err
	case kindFilm:
		in := repo.FilmImageInsert{FilmID: id, Role: slot.role, Source: model.FilmImageSourceUpload}
		if v.rowOnly {
			in.Width, in.Height, in.ByteSize = w, h, len(norm)
			_, err := iw.repo.ReplaceFilmImage(ctx, in)
			return err
		}
		_, err := imagesink.ReplaceFilmImageFile(ctx, iw.repo, dir, in, norm, w, h)
		return err
	default:
		return fmt.Errorf("no image store for entity kind %q", kind)
	}
}

// storeVideoImage writes a video's thumbnail or poster.
//
// A video is the one kind whose images are not rows: the path is derived from the
// id and the database records only thumbnail_state, which is what decides whether
// the API emits a thumbnail_url at all. So the missing rung here sets the state and
// writes nothing — the page then asks for an image the server cannot find, which is
// precisely the state being reproduced.
func (iw imageWriter) storeVideo(ctx context.Context, id int64, dir string, slot imageSlot, v imageVariant, norm []byte) error {
	// 'generated' rather than 'uploaded': it is what the thumbnail pipeline leaves
	// behind for a file it extracted a frame from, so it is the state the vast
	// majority of a real library is in.
	if err := iw.repo.SetThumbnailState(ctx, id, model.ThumbnailGenerated); err != nil {
		return fmt.Errorf("set thumbnail state: %w", err)
	}
	if v.rowOnly {
		return nil
	}
	path := thumbnail.ThumbPath(dir, id)
	if slot.label == "poster" {
		path = thumbnail.PosterPath(dir, id)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create thumbnail dir: %w", err)
	}
	if err := os.WriteFile(path, norm, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// storePersonImage writes one person image row and its file.
//
// Person is the one kind with no ReplaceXImageFile helper in the sink, so it
// follows the sink's own person path by hand: insert the row, then write the bytes
// under the returned id, because the id *is* the filename.
func (iw imageWriter) storePerson(ctx context.Context, id int64, dir string, slot imageSlot, v imageVariant, norm []byte, w, h int) error {
	imageID, err := iw.repo.InsertPersonImage(ctx, repo.PersonImageInsert{
		PersonID:    id,
		Role:        slot.role,
		Source:      model.PersonImageSourceUpload,
		ContentHash: personimage.Hash(norm),
		Width:       w,
		Height:      h,
		ByteSize:    len(norm),
		// The gallery cap is a product rule about what an owner may upload, not a
		// claim about what the table can hold. The fixture seeds a fixed few tiles
		// and stays far under it, but says so rather than relying on the number.
		OverCap: false,
	})
	if err != nil {
		return fmt.Errorf("insert person image row: %w", err)
	}
	if v.rowOnly {
		return nil
	}
	if err := personimage.Store(dir, id, imageID, norm); err != nil {
		return fmt.Errorf("store person image: %w", err)
	}
	return nil
}

// monogram is the legible label burned into every fixture image (the ticket's
// requirement, and the image half of D4's "a name is a coordinate").
//
// It names the entity, the rung and the slot, and then states the pixel size —
// which is the wrong-ratio rung's whole subject, and the one fact a screenshot
// otherwise cannot carry. A cropped image showing "600x600" inside a 2/3 frame is
// a self-describing bug report.
func monogram(kind entityKind, id int64, slot imageSlot, v imageVariant, maxDim, tile, tiles int) []string {
	frame, w, h := v.geometry(slot, maxDim)

	label := slot.label
	if tiles > 1 {
		label = fmt.Sprintf("%s %d/%d", label, tile+1, tiles)
	}
	return []string{
		strings.ToUpper(string(kind)) + " " + fmt.Sprint(id),
		strings.ToUpper(v.key),
		label,
		fmt.Sprintf("%dx%d %s", w, h, frame),
	}
}

// drawLabel renders the monogram centred in dst, scaled up by the largest whole
// factor that still fits.
//
// The scale is an integer and the blit is nearest-neighbour because the point is
// legibility at a glance, not typography: basicfont is a 7x13 bitmap face, and any
// non-integer scale of a bitmap face is worse than a blocky one. Lines that do not
// fit are dropped from the bottom rather than overflowing, which is how the 32px
// degenerate rung stays a picture of something instead of a picture of noise.
func drawLabel(dst *image.NRGBA, lines []string, ink color.NRGBA) {
	face := basicfont.Face7x13
	width := 0
	for _, line := range lines {
		width = max(width, font.MeasureString(face, line).Ceil())
	}
	if width == 0 || len(lines) == 0 {
		return
	}

	bounds := dst.Bounds()
	// Nine tenths, so the label has a margin and a cover-cropped frame still shows
	// most of it.
	scale := min(bounds.Dx()*9/10/width, bounds.Dy()*9/10/(len(lines)*face.Height))
	if scale < 1 {
		scale = 1
	}
	// Drop lines from the bottom until the block fits at scale 1.
	fit := lines
	for len(fit) > 1 && len(fit)*face.Height*scale > bounds.Dy() {
		fit = fit[:len(fit)-1]
	}
	height := len(fit) * face.Height

	small := image.NewNRGBA(image.Rect(0, 0, width, height))
	drawer := &font.Drawer{Dst: small, Src: image.NewUniform(ink), Face: face}
	for i, line := range fit {
		drawer.Dot = fixed.P(0, i*face.Height+face.Ascent)
		drawer.DrawString(line)
	}

	offX := bounds.Min.X + (bounds.Dx()-width*scale)/2
	offY := bounds.Min.Y + (bounds.Dy()-height*scale)/2
	for y := range height {
		for x := range width {
			c := small.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			for dy := range scale {
				for dx := range scale {
					px, py := offX+x*scale+dx, offY+y*scale+dy
					if image.Pt(px, py).In(bounds) {
						dst.SetNRGBA(px, py, c)
					}
				}
			}
		}
	}
}
