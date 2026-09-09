package main

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"holodex/internal/model"
)

// The ladder is a table, not a program (HOLODEX-344). Adding a dimension is
// adding a row to `ladder`; adding a rung is adding an entry to that row. Block
// allocation, name encoding, manifest emission and the overflow check all read
// the table rather than knowing anything about a particular dimension, so none
// of them has to change when the fixture grows.
//
// Two rules the table encodes structurally rather than by convention:
//
// D3, one factor at a time: a rung produces a spec by mutating the neutral
// baseline in exactly one axis. There is no way to express a cross-product here,
// which is the point — 900 entities that cannot attribute a failure are worse
// than 30 that name the broken knob.
//
// D4, reserved ID blocks: each dimension declares the block its entities are
// addressed in, so inserting a rung renumbers only within one block and never
// invalidates an assertion written against another.

const (
	// blockSize is how many IDs a dimension owns. Overflowing it would push a
	// dimension's entities into the next dimension's addresses, so generation
	// refuses rather than silently colliding.
	blockSize = 100

	// poolBase is where supporting entities start — the people, tags and studios
	// that exist only to be counted by a cardinality rung. They are deliberately
	// above every dimension block so a block never has to skip over them, and so
	// an ID below poolBase is always an addressed video or film.
	poolBase = 9000

	// derivedBase is where the *derived* kinds' blocks start, above the pool rather
	// than below it (HOLODEX-346).
	//
	// A person, studio or tag cannot be created on its own: the only path into
	// those tables is a reconcile driven by a video's file layer, so an addressed
	// one needs a carrier video, and a carrier video can only be made once the
	// videos sequence has been steered past every addressed video block. By then
	// the people, studio and tag sequences have long since passed poolBase — the
	// cardinality rungs created their supporting entities on the way — and SQLite's
	// AUTOINCREMENT counter cannot be rewound. So the addressed people sit above
	// their own supporting cast, which is the opposite order to videos and films.
	//
	// The alternative was to renumber every existing block downward to make room at
	// the bottom, which is the one thing D4 promises never to do.
	derivedBase = 20000

	// derivedCeiling bounds the derived address space so a typo in a block cannot
	// push entities somewhere nothing checks. The gap below it is headroom for the
	// supporting entities, which grow with the ladder and with `-count`
	// (HOLODEX-350); if they ever reach derivedBase, generation fails on the
	// per-rung block check rather than quietly colliding.
	derivedCeiling = 30000
)

// entityKind is the entity a dimension's rungs address. It decides which ID
// sequence gets steered and which URL the manifest records.
type entityKind string

const (
	kindVideo entityKind = "video"
	kindFilm  entityKind = "film"

	// The derived kinds. Each has its own detail page — /people/{id},
	// /studios/{id}, /tags/{id} — which is why their names are worth addressing
	// rather than leaving to fall out of the video ladder: the name is an h1 there,
	// with a docked rename pencil beside it, not a label inside a tile.
	kindPerson entityKind = "person"
	kindStudio entityKind = "studio"
	kindTag    entityKind = "tag"
)

// urlFor renders the page an addressed entity lives at, which is the whole
// reason the manifest is worth emitting: it turns "media 123" into a link.
//
// Note the route is /media, not /video — the URL an entity is addressed by is
// not derivable from its name, which is why both this and table() are explicit
// mappings rather than string arithmetic.
func (k entityKind) urlFor(id int64) string {
	switch k {
	case kindVideo:
		return fmt.Sprintf("/media/%d", id)
	case kindFilm:
		return fmt.Sprintf("/films/%d", id)
	case kindPerson:
		return fmt.Sprintf("/people/%d", id)
	case kindStudio:
		return fmt.Sprintf("/studios/%d", id)
	case kindTag:
		return fmt.Sprintf("/tags/%d", id)
	default:
		return ""
	}
}

// table is the SQL table whose ID sequence gets steered to place this kind's
// entities in their reserved block.
func (k entityKind) table() string {
	switch k {
	case kindVideo:
		return "videos"
	case kindFilm:
		return "films"
	case kindPerson:
		return "people"
	case kindStudio:
		return "studios"
	case kindTag:
		return "tags"
	default:
		return ""
	}
}

// derived reports whether this kind's rows can only come into existence as a
// byproduct of a video — a reconcile over the video's resolved file layer for
// people and studios, an attach for tags. Nothing in the repo creates one from a
// name alone, and a studio that loses its last link is deleted outright.
//
// Two consequences, both structural rather than stylistic. A rung of a derived
// kind has to seed a carrier video to hang its entity off, and its reserved block
// lives above poolBase rather than below it — see derivedBase.
func (k entityKind) derived() bool {
	switch k {
	case kindPerson, kindStudio, kindTag:
		return true
	default:
		return false
	}
}

// addressSpace is the ID range this kind's reserved blocks have to fall inside.
// There are two ranges rather than one because the two halves are numbered in
// opposite orders around the supporting-entity pool (see derivedBase).
func (k entityKind) addressSpace() (lo, hi int64) {
	if k.derived() {
		return derivedBase, derivedCeiling
	}
	return 1, poolBase
}

// spec is one fixture entity as the ladder describes it: the neutral baseline
// with exactly one axis modified. Every field is an axis the encoded name
// reports, so a name is a complete coordinate rather than a label — which is
// what lets the owner read "this is the 25-people one" off the page itself.
type spec struct {
	// Video axes.
	people  int
	tags    int
	studios int
	text    textVariant

	// image is the one axis every entity kind that renders a picture shares, so
	// unlike the two halves below it is not keyed to a kind: a video, a person, a
	// studio and a film each have image slots, and the same rung means the same
	// thing on all four. That is deliberate — it is what lets a crop fix found on
	// the media page be checked on the person page without translating the address.
	image imageVariant

	// Film axes. A film is a separate entity kind with a separate ID sequence and
	// separate rungs, so only one half of this struct describes any given entity —
	// which is why encodeName and axesOf are both keyed by entity kind rather than
	// rendering the whole thing. A film entry carrying the video baseline's
	// people=2 in its manifest would be a plain falsehood about that film.
	cast   int
	scenes int
}

// textVariant pairs the string under test with the short label that names it.
// The label is what appears in an encoded name and a manifest entry; the value
// is what actually goes into the field being tortured.
type textVariant struct {
	key   string
	value string
}

// baseline is the boring entity every rung starts from (D3). The counts are
// small and non-zero on purpose: a baseline of zero everywhere would mean the
// people dimension was also silently testing the no-tags empty state, and a
// failure could not be attributed.
func baseline() spec {
	return spec{
		people:  2,
		tags:    3,
		studios: 1,
		text:    textVariant{key: "plain", value: "Stress baseline"},
		cast:    2,
		scenes:  3,
		// The baseline carries no image, which is the one place the neutral value is
		// also the zero rung rather than a small non-zero one. Two reasons. An
		// un-enriched library really does look like this, so it is the honest
		// default; and giving every entity a picture would mean seeding one for each
		// of the ~50 supporting people a cardinality rung creates, which is minutes
		// of JPEG encoding on every run for entities nothing is addressed at.
		image: imageNone,
	}
}

// rung is one step of one dimension: the label it is addressed by, and the
// single-axis mutation that produces its entity.
type rung struct {
	// variant labels the rung in names, the manifest and bug reports. It is the
	// stable half of an address — the ID says where, the variant says what.
	variant string

	// value is the rung's position on its axis, recorded in the manifest so an
	// assertion can be written against a threshold ("every page where people >=
	// 10") rather than against an enumerated list of IDs.
	value any

	// apply mutates the baseline in this dimension's axis and no other.
	apply func(*spec)
}

// dimension is one row of the ladder.
type dimension struct {
	key    string     // addressed as, in names and the manifest
	entity entityKind // what the rungs build, and therefore which sequence is steered
	block  int64      // first ID of this dimension's reserved block (D4)
	finds  string     // the bug class this dimension exists to surface
	rungs  []rung

	// noEmptyRung, when non-empty, is why this dimension cannot carry the zero
	// case — and is the only thing that excuses it from D2.
	//
	// It is a stated reason rather than a bool, and rather than a skip list in the
	// test, because "this dimension has no empty rung" is nearly always a bug: D2
	// exists because empty states are half the layout bug class, and HOLODEX-328 is
	// what it cost to learn that. The only legitimate excuse is that the *app*
	// cannot reach the empty state either, which is a claim about the code that
	// belongs next to the dimension making it.
	noEmptyRung string

	// ownsTitle says this dimension's subject *is* the entity's name — a video's
	// title, or a person's, studio's or tag's own name — so that name must be the
	// raw variant rather than the encoded coordinate.
	//
	// Entities normally carry their coordinate as their name, so the owner can
	// tell what they are looking at without opening the manifest. A dimension
	// that tortures the title cannot afford that prefix: the empty rung has to
	// be genuinely empty to test the empty-title layout at all (D2), and a
	// prefixed 60-character unbroken token is no longer unbroken. Those entities
	// stay addressable through their reserved block and the manifest — which is
	// exactly why D4 has three layers and not one.
	ownsTitle bool
}

// counts renders a cardinality ladder over one axis. The axis is a parameter so
// the same helper builds the people, tags, studios and cast dimensions — that is
// what keeps "adding a dimension is a row" true rather than aspirational.
//
// Variants are zero-padded so names sort and align in a list.
func counts(axis func(*spec, int), values ...int) []rung {
	out := make([]rung, 0, len(values))
	for _, n := range values {
		out = append(out, rung{
			variant: fmt.Sprintf("%02d", n),
			value:   n,
			apply:   func(s *spec) { axis(s, n) },
		})
	}
	return out
}

// texts renders the text rungs, carrying each variant's label through to the
// spec so the encoded name and the manifest agree on what to call it.
func texts(variants ...textVariant) []rung {
	out := make([]rung, 0, len(variants))
	for _, v := range variants {
		out = append(out, rung{
			variant: v.key,
			value:   v.value,
			apply:   func(s *spec) { s.text = v },
		})
	}
	return out
}

// images renders the image rungs. Exactly texts() one type over: the variant's
// key addresses the rung and its value describes it, so the zero rung's empty
// value is what hasEmptyRung reads.
func images(variants ...imageVariant) []rung {
	out := make([]rung, 0, len(variants))
	for _, v := range variants {
		out = append(out, rung{
			variant: v.key,
			value:   v.value,
			apply:   func(s *spec) { s.image = v },
		})
	}
	return out
}

// ladder is the table. Rows must be ordered by ascending block within an entity
// kind — not merely for readability: dimensions sharing a kind share one ID
// sequence, so a block declared out of order would be steered backwards over
// rows that already exist. validateLadder enforces it.
var ladder = []dimension{
	{
		key:    "people",
		entity: kindVideo,
		block:  100,
		finds:  "headshot shrink, credit row wrap, overflow past the container",
		// Zero is a rung, not an oversight (D2): the empty-cast state is half
		// this dimension's bug class, and HOLODEX-328 exists because it was
		// missed. The top rung is past anything real data produces on purpose.
		rungs: counts(func(s *spec, n int) { s.people = n }, 0, 1, 5, 10, 25, 50),
	},
	{
		key:       "text",
		entity:    kindVideo,
		block:     200,
		finds:     "wrapping, truncation, container overflow, bidi bleed",
		ownsTitle: true,
		// D7: lorem is the *friendliest* long text there is — short Latin words
		// with spaces everywhere wrap beautifully, so it only stresses vertical
		// space. It is kept as the weakest rung; the ones that actually break a
		// flex container are the unbroken token, CJK, RTL and the diacritics.
		rungs: texts(textPalette...),
	},
	{
		key:    "tags",
		entity: kindVideo,
		block:  300,
		finds:  "chip wrapping, row height blowout, filter-bar overflow",
		// Tags are the one relationship here that is authored rather than derived
		// (ADR-075 D3: only a file-sourced rescan clears them, and the fixture has
		// no files), so this dimension needs nothing from the mapping. The top rung
		// is well past what a real library carries because the failure is a wrap,
		// and a wrap needs enough chips to reach the second and third row.
		rungs: counts(func(s *spec, n int) { s.tags = n }, 0, 1, 5, 30),
	},
	{
		key:    "studios",
		entity: kindVideo,
		block:  400,
		finds:  "empty section, single-item layout, multi-studio row wrap",
		// Capped lower than people on purpose: co-productions in the low single
		// digits are the realistic maximum, and the rung that actually finds bugs
		// is 0 (the empty section) rather than the top. Reaching 5 at all requires
		// `studio` to be `multi: true` in the mapping — loadFields refuses
		// otherwise rather than letting the rungs collapse silently.
		rungs: counts(func(s *spec, n int) { s.studios = n }, 0, 1, 5),
	},
	{
		key:    "videoimage",
		entity: kindVideo,
		block:  700,
		finds:  "text-over-thumbnail contrast, grid crop, the broken-image retry loop",
		// Block 700 rather than 500 because 500 and 600 belong to the film
		// dimensions below. Blocks are unique across the whole table, and D4's
		// promise is that a block never moves once assertions can be written against
		// it — so the video half skips over the film half rather than renumbering it.
		// The same inversion HOLODEX-346 accepted for the derived kinds.
		rungs: images(imagePalette...),
	},
	{
		key:    "scenes",
		entity: kindFilm,
		block:  500,
		finds:  "scene badge, ordering, empty film, scene-list overflow",
		// "Scene" is not an entity — film_videos.scene_number is a role a video
		// plays inside a film (ADR-085) — so these rungs vary how many videos a
		// film has attached, drawn from the scene pool rather than from the
		// addressed video rungs. Attaching an addressed rung would have put a film
		// badge on it and broken OFAT: a failure on the people=25 page could then
		// be the cast or the film attachment. The pool carries the text palette
		// instead, so the scene list still inherits the text torture.
		rungs: counts(func(s *spec, n int) { s.scenes = n }, 0, 1, 6, 12),
	},
	{
		key:    "filmcast",
		entity: kindFilm,
		block:  600,
		finds:  "shared cast-tile sizing with the media page, billing order, empty cast",
		// film_people_roles is authored, not derived (ADR-085 §2) — it is a
		// property of the film rather than a union over its scenes, which is the
		// derived FilmCast. Same rungs as the video people ladder because the tile
		// size is shared between the two pages: a fix on one has to be checked on
		// the other, and equal rungs make that a like-for-like comparison.
		rungs: counts(func(s *spec, n int) { s.cast = n }, 0, 1, 5, 10, 25, 50),
	},
	{
		key:    "filmimage",
		entity: kindFilm,
		block:  800,
		finds:  "poster crop, the 8/3 banner hero, the header fade over a bright image",
		// The film banner is the largest image the app renders and the only one with
		// text laid over it by default (.portrait-frame--banner::before is a fade,
		// not a scrim), so `bright` is the rung to look at first here.
		rungs: images(imagePalette...),
	},

	// The derived half (HOLODEX-346). These three exist because a person, studio
	// and tag each has a detail page of its own where the name is the h1 — the same
	// palette that tortures a video title has to reach those headings, and the
	// cast tile, studio chip and tag chip it renders in besides.
	//
	// They come last in the table, and must: each rung seeds a carrier video to
	// hang its entity off, and a carrier can only be made once the videos sequence
	// has been steered out of the addressed range. validateLadder enforces it.
	{
		key:       "persontext",
		ownsTitle: true,
		entity:    kindPerson,
		block:     derivedBase,
		finds:     "hero heading wrap, the docked rename pencil, cast-tile label truncation",
		noEmptyRung: "ReconcileVideoPeople skips an empty name, so an unnamed person cannot " +
			"exist — the empty *cast* is covered by the people=00 rung instead",
		rungs: texts(namePalette(kindPerson)...),
	},
	{
		key:       "studiotext",
		ownsTitle: true,
		entity:    kindStudio,
		block:     derivedBase + blockSize,
		finds:     "studio heading wrap, chip width on the media page, studio-list column width",
		noEmptyRung: "ReconcileVideoStudios skips an empty name, and prunes a studio that " +
			"loses its last link — the empty studio *section* is the studios=00 rung",
		rungs: texts(namePalette(kindStudio)...),
	},
	{
		key:       "tagtext",
		ownsTitle: true,
		entity:    kindTag,
		block:     derivedBase + 2*blockSize,
		finds:     "chip wrap and height, filter-bar overflow, tag-page heading",
		noEmptyRung: "the repo would create an empty tag but the HTTP layer refuses one, so " +
			"seeding it would show a state the app cannot reach; the empty tag *row* " +
			"is the tags=00 rung",
		rungs: texts(namePalette(kindTag)...),
	},

	// The derived kinds that render pictures. A tag has no image dimension because
	// it has no image: it is a chip and a heading, and slotsFor says so.
	//
	// These come after the text dimensions and must, for the same reason those come
	// after the video ones: each rung seeds a carrier video, and blocks ascend
	// within an entity kind because they share one ID sequence.
	{
		key:    "personimage",
		entity: kindPerson,
		block:  derivedBase + 3*blockSize,
		finds:  "avatar crop at every size, the 8/3 hero banner, gallery tile shape",
		// The most image-dense entity in the app: four slots, three of them core, and
		// the headshot is rendered at w-12, w-20 and w-32 on different pages from the
		// same stored file. `ratio` is the rung to look at first — a 21/9 headshot is
		// in the ticket by name, and object-position: center 28% means a cover crop
		// here is not even centred.
		rungs: images(imagePalette...),
	},
	{
		key:    "studioimage",
		entity: kindStudio,
		block:  derivedBase + 4*blockSize,
		finds:  "logo letterboxing, the transparent-logo black plate, list-well icon width",
		// The one kind whose slots are object-contain, which makes it the only place
		// the `alpha` rung shows something `black` does not: a flattened transparent
		// logo is a plate that does not fill its well rather than an image that does.
		// Against these three dark skins it reads as a logo that half-disappeared, not
		// as an obvious black box — which is the harder failure to notice.
		rungs: images(imagePalette...),
	},
}

// textPalette is the shared set of adversarial strings. It is a package-level
// var rather than an argument list inline in the text dimension because the
// scene pool cycles the same palette through its titles (generate.go): a film
// whose scene list is all plain names would not inherit the text torture the
// spec promises it, and two divergent copies of the palette would be the
// obvious way for that to rot.
var textPalette = []textVariant{
	{"empty", ""},
	{"single", "x"},
	{"unbroken", "Supercalifragilisticexpialidociousandthensomemoreforgoodmeasure"},
	{"cjk", "日本語のタイトルは折り返しの規則が違うので幅の計算が狂いやすい"},
	{"rtl", "عنوان طويل بالعربية لاختبار اتجاه النص والتفاف الأسطر في الواجهة"},
	// Mixed bidi is a separate rung from `rtl`, not a longer version of it: the two
	// fail differently. A pure-RTL string finds direction and alignment bugs; a
	// string that changes direction mid-line finds bidi bleed — neutral characters
	// (the digits, the parentheses, the em dash) taking their direction from the
	// run beside them, so punctuation lands at the wrong end of the line. Only the
	// second one can produce that, and the dimension's `finds` already promised it.
	{"bidi", "Episode 12 — مقدمة الفيلم الوثائقي (Director's Cut) — 1080p"},
	// Not just a run of emoji: the AC's failure mode is the *multi-codepoint*
	// sequence. A ZWJ family, a flag built from two regional indicators, a skin-tone
	// modifier and an emoji carrying a variation selector are each one grapheme
	// cluster made of several code points, so anything counting runes, slicing
	// bytes, or sizing a line box per code point breaks here and not on 🎬. The
	// joiners and selectors are invisible in this source line, which is exactly why
	// TestEmojiRungKeepsItsMultiCodepointSequences exists.
	{"emoji", "🎬🎥📽️🍿🎞️ 👨‍👩‍👧‍👦 🧑‍💻 🏳️‍🌈 ❤️‍🔥 👍🏽 🇯🇵 🎬🎥📽️🍿🎞️ 👨‍👩‍👧‍👦 🧑‍💻 🏳️‍🌈 ❤️‍🔥 👍🏽 🇯🇵"},
	{"diacritics", "Z̸̢̛͇͓a̷̡̮͐l̶̪̀g̵̛̭o̴̠͐ ̷̣̈t̶̰́e̶̪͐x̷̱̌t̸̗̽ ̴̙̇w̷̢̌i̶̻͐t̵̰̏h̶̬̀ ̸̜̐s̶̙̈t̷̗̏a̷̪̐c̸̣̈k̶̝̇e̷̙̊d̸̯̄ ̶̬̇m̷̜̊a̸̡̽r̶̢̈k̷̙̇s̸̪̈"},
	{"lorem", lorem},
}

// hasEmptyRung reports whether a dimension renders the zero case: a count of 0,
// or a text variant of "". Both spellings matter — the ladder addresses "no
// people" and "no title" as the same idea in two types.
func hasEmptyRung(dim dimension) bool {
	for _, rg := range dim.rungs {
		switch v := rg.value.(type) {
		case int:
			if v == 0 {
				return true
			}
		case string:
			if v == "" {
				return true
			}
		}
	}
	return false
}

// namePalette is the subset of textPalette a derived entity's *name* can carry.
//
// The exclusions are computed from the platform's own limits rather than written
// down as a list, so a rung cannot be silently lost: change the palette and the
// filter re-derives; change the limit and the filter follows. nameRejects returns
// the reason, which the seeder prints and the tests assert, because an omission
// with no stated cause is indistinguishable from a bug.
//
// Tag values are lowercased here because resolveOrCreateByName lowercases a tag
// on the way in (curationNorm, the "fox"/"Fox" fix). Seeding the mixed-case form
// would store something other than what the manifest claims, and the manifest
// being true is the entire point of addressing these at all.
func namePalette(kind entityKind) []textVariant {
	out := make([]textVariant, 0, len(textPalette))
	for _, v := range textPalette {
		if nameRejects(kind, v) != "" {
			continue
		}
		if kind == kindTag {
			v.value = strings.ToLower(v.value)
		}
		out = append(out, v)
	}
	return out
}

// nameRejects explains why this kind cannot be named with this variant, or ""
// when it can. Each reason is a real limit in the code, not a preference.
func nameRejects(kind entityKind, v textVariant) string {
	if !kind.derived() {
		return ""
	}
	if v.value == "" {
		if kind == kindTag {
			// Unlike people and studios, an empty tag row really is creatable — the
			// repo path has no guard and would insert one. The HTTP layer refuses it
			// (400 "name is required"), so seeding one would put a state on the page
			// that the running app cannot produce, and any layout bug found there
			// would be unreportable.
			return "only the repo API can create an empty tag; the HTTP layer refuses one, " +
				"so the fixture would be showing a state the app cannot reach"
		}
		return "the reconcile that maintains this table skips an empty name, so the entity " +
			"would never be created at all"
	}
	switch kind {
	case kindPerson, kindStudio:
		// People and studios are derived from a *multi* file field, and the resolver
		// splits a multi value on these before it ever reaches the repo. A name
		// carrying one would arrive as several entities — the same refusal linkTags
		// already makes at seed time, hoisted to the table so the rung is never built.
		if strings.ContainsAny(v.value, multiValueSeparators) {
			return fmt.Sprintf("the resolver splits a multi field on %q, so this name would "+
				"fracture into several entities", multiValueSeparators)
		}
	case kindTag:
		// resolveOrCreateByName rejects an over-long tag with ErrTagNameTooLong, and
		// AttachMaterializedTags skips it silently — so without this the rung would
		// vanish from the fixture with nothing said.
		if n := utf8.RuneCountInString(v.value); n > model.MaxNameLen {
			return fmt.Sprintf("%d characters is over model.MaxNameLen (%d), which the repo "+
				"rejects with ErrTagNameTooLong", n, model.MaxNameLen)
		}
	}
	return ""
}

// lorem is the vertical-overflow rung: at least loremMin characters. Weakest of
// the text set by design (D7), kept because vertical overflow is still a real
// failure mode.
//
// The length is asserted rather than trusted. This string spent HOLODEX-344 and
// -347 at 1393 characters under a comment claiming 1500 — harmless in itself, but
// the failure it hides is not: an overview clamped to N lines stops overflowing
// once the text is short enough, and the rung would then pass by having quietly
// become a different test.
const lorem = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod " +
	"tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis " +
	"nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat. Duis " +
	"aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat " +
	"nulla pariatur. Excepteur sint occaecat cupidatat non proident, sunt in culpa qui " +
	"officia deserunt mollit anim id est laborum. Sed ut perspiciatis unde omnis iste " +
	"natus error sit voluptatem accusantium doloremque laudantium, totam rem aperiam, " +
	"eaque ipsa quae ab illo inventore veritatis et quasi architecto beatae vitae dicta " +
	"sunt explicabo. Nemo enim ipsam voluptatem quia voluptas sit aspernatur aut odit aut " +
	"fugit, sed quia consequuntur magni dolores eos qui ratione voluptatem sequi nesciunt. " +
	"Neque porro quisquam est, qui dolorem ipsum quia dolor sit amet, consectetur, adipisci " +
	"velit, sed quia non numquam eius modi tempora incidunt ut labore et dolore magnam " +
	"aliquam quaerat voluptatem. Ut enim ad minima veniam, quis nostrum exercitationem " +
	"ullam corporis suscipit laboriosam, nisi ut aliquid ex ea commodi consequatur. Quis " +
	"autem vel eum iure reprehenderit qui in ea voluptate velit esse quam nihil molestiae " +
	"consequatur, vel illum qui dolorem eum fugiat quo voluptas nulla pariatur. At vero eos " +
	"et accusamus et iusto odio dignissimos ducimus qui blanditiis praesentium voluptatum " +
	"deleniti atque corrupti quos dolores et quas molestias excepturi sint occaecati " +
	"cupiditate non provident, similique sunt in culpa qui officia deserunt mollitia animi."

// loremMin is the length the lorem rung has to reach to still be doing its job.
// 1500 is the number the ticket asked for; what matters is that it is far past
// any line clamp on the page, so shortening the string fails the test instead of
// silently weakening the rung.
const loremMin = 1500

// encodeName renders a spec as the coordinate the owner reads off the page and
// searches for (D4, layer 2): `STRESS people=05 tags=03 studios=01 text=cjk`.
//
// Every axis is reported, not just the varied one. Under OFAT the others are at
// baseline, so the odd value out *is* the dimension under test — which makes a
// name readable without consulting the manifest, and makes a screenshot pasted
// into a bug report self-describing.
func encodeName(kind entityKind, s spec) string {
	if kind.derived() {
		// A derived entity's coordinate is not always its name: when the dimension
		// owns the name (persontext and friends) the page shows the raw variant and
		// this is only what the manifest and the carrier video carry. When it does
		// not — personimage, studioimage — this *is* the entity's name, and every
		// rung needs a different one or resolveOrCreateByName would fold the whole
		// dimension into a single entity. Which is why the image axis has to be here.
		return fmt.Sprintf("STRESS %s text=%s image=%s",
			strings.ToUpper(string(kind)), s.text.key, s.image.key)
	}
	if kind == kindFilm {
		// A film's name is also its identity: CreateFilm resolves-or-creates by
		// (name, year), so two rungs sharing a name would silently become one
		// film. Every film rung varies one of these three axes, so the triple is
		// unique across the whole film half of the ladder.
		return fmt.Sprintf("STRESS FILM cast=%02d scenes=%02d image=%s", s.cast, s.scenes, s.image.key)
	}
	return fmt.Sprintf("STRESS people=%02d tags=%02d studios=%02d text=%s image=%s",
		s.people, s.tags, s.studios, s.text.key, s.image.key)
}

// ladderDemands is the highest rung the table reaches on each axis that has to be
// expressible through the file layer. It is derived from the table rather than
// written down, so raising a rung cannot leave the mapping check behind.
type ladderDemands struct {
	people  int
	studios int
	cast    int
	scenes  int
}

func demands(dims []dimension) ladderDemands {
	var d ladderDemands
	for _, dim := range dims {
		for _, rg := range dim.rungs {
			s := baseline()
			rg.apply(&s)
			// Only the axes belonging to the rung's own entity kind count: the
			// baseline's film axes ride along on every video spec, and taking the
			// max over those would demand a scene pool from a ladder with no film
			// dimensions at all.
			switch dim.entity {
			case kindVideo:
				d.people = max(d.people, s.people)
				d.studios = max(d.studios, s.studios)
			case kindFilm:
				d.cast = max(d.cast, s.cast)
				d.scenes = max(d.scenes, s.scenes)
			}
		}
	}
	return d
}
