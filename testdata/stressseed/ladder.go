package main

import "fmt"

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
	// an ID below poolBase is always an addressed entity.
	poolBase = 9000
)

// entityKind is the entity a dimension's rungs address. It decides which ID
// sequence gets steered and which URL the manifest records.
type entityKind string

const kindVideo entityKind = "video"

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
	default:
		return ""
	}
}

// spec is one fixture entity as the ladder describes it: the neutral baseline
// with exactly one axis modified. Every field is an axis the encoded name
// reports, so a name is a complete coordinate rather than a label — which is
// what lets the owner read "this is the 25-people one" off the page itself.
type spec struct {
	people  int
	tags    int
	studios int
	text    textVariant
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

	// ownsTitle says this dimension's subject *is* the entity's title, so the
	// title must be the raw variant rather than the encoded coordinate.
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
		rungs: texts(
			textVariant{"empty", ""},
			textVariant{"single", "x"},
			textVariant{"unbroken", "Supercalifragilisticexpialidociousandthensomemoreforgoodmeasure"},
			textVariant{"cjk", "日本語のタイトルは折り返しの規則が違うので幅の計算が狂いやすい"},
			textVariant{"rtl", "عنوان طويل بالعربية لاختبار اتجاه النص والتفاف الأسطر في الواجهة"},
			textVariant{"emoji", "🎬🎥📽️🍿🎞️🎬🎥📽️🍿🎞️🎬🎥📽️🍿🎞️🎬🎥📽️🍿🎞️"},
			textVariant{"diacritics", "Z̸̢̛͇͓a̷̡̮͐l̶̪̀g̵̛̭o̴̠͐ ̷̣̈t̶̰́e̶̪͐x̷̱̌t̸̗̽ ̴̙̇w̷̢̌i̶̻͐t̵̰̏h̶̬̀ ̸̜̐s̶̙̈t̷̗̏a̷̪̐c̸̣̈k̶̝̇e̷̙̊d̸̯̄ ̶̬̇m̷̜̊a̸̡̽r̶̢̈k̷̙̇s̸̪̈"},
			textVariant{"lorem", lorem},
		),
	},
}

// lorem is the 1500-character vertical-overflow rung. Weakest of the text set by
// design (D7), kept because vertical overflow is still a real failure mode.
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
	"consequatur, vel illum qui dolorem eum fugiat quo voluptas nulla pariatur."

// encodeName renders a spec as the coordinate the owner reads off the page and
// searches for (D4, layer 2): `STRESS people=05 tags=03 studios=01 text=cjk`.
//
// Every axis is reported, not just the varied one. Under OFAT the others are at
// baseline, so the odd value out *is* the dimension under test — which makes a
// name readable without consulting the manifest, and makes a screenshot pasted
// into a bug report self-describing.
func encodeName(s spec) string {
	return fmt.Sprintf("STRESS people=%02d tags=%02d studios=%02d text=%s",
		s.people, s.tags, s.studios, s.text.key)
}
