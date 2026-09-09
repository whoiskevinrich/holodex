package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// manifestName is where the manifest lands inside the fixture's data directory,
// so `rm -rf` still takes the whole fixture with it.
const manifestName = "manifest.json"

// entry is one addressed fixture entity.
type entry struct {
	ID        int64      `json:"id"`
	Entity    entityKind `json:"entity"`
	Dimension string     `json:"dimension"`
	Variant   string     `json:"variant"`
	Value     any        `json:"value"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`

	// Axes is the entity's full coordinate, not just the axis under test. It is
	// what lets an assertion be written against a threshold — "on every page
	// where people >= 10, each headshot is at least 40px wide" (D6) — instead of
	// against a list of IDs that goes stale the moment a rung is inserted.
	Axes axes `json:"axes"`
}

// axes is the entity's full coordinate in the wire format. It is a separate type
// from spec on purpose: spec is free to change shape as dimensions are added,
// while this is a contract with whatever reads the manifest back.
//
// Exactly one of the three kind-keyed halves is populated, chosen by the entry's
// entity kind. spec carries them all so a rung can be written without knowing
// which it is, but only one describes any given entity — a film reporting the
// video baseline's people=2 would be a plain falsehood about that film, and an
// assertion written against it would be measuring nothing. Image is the one axis
// that crosses kinds and so sits alongside whichever half applies.
type axes struct {
	Video *videoAxes `json:"video,omitempty"`
	Film  *filmAxes  `json:"film,omitempty"`

	// Name is the coordinate of a derived kind — a person, studio or tag whose
	// own name is the rung. It is one axis rather than a struct because there is
	// only one: an addressed person has no cardinality of its own, it has a name.
	Name *nameAxes `json:"name,omitempty"`

	// Image is the exception to "exactly one half": it is present alongside
	// whichever half describes the kind, because an image is a property all four
	// picture-rendering kinds have (HOLODEX-345). It is omitted for the zero rung,
	// so its presence means "this entity carries a deliberately awful image" and a
	// reader does not have to know which key means none.
	Image *imageAxes `json:"image,omitempty"`
}

type videoAxes struct {
	People  int    `json:"people"`
	Tags    int    `json:"tags"`
	Studios int    `json:"studios"`
	Text    string `json:"text"`
}

type filmAxes struct {
	Cast   int `json:"cast"`
	Scenes int `json:"scenes"`
}

type nameAxes struct {
	Text string `json:"text"`

	// Value is the name as stored, which is not always the palette's own string:
	// a tag is lowercased on the way in. Recording it means a reader never has to
	// reconstruct the normalisation to search for the entity it is looking at.
	Value string `json:"value"`
}

// imageAxes is what a geometry assertion needs to know about an entity's images
// without re-deriving the seeder's tables: which treatment they carry, and which
// slots to look at.
//
// Slots is spelled out even though it is a function of the entity kind, because
// the alternative is a second copy of slotsFor living in whatever reads the
// manifest — and that copy is the one that would go stale. Contain likewise: the
// difference between a black plate under object-cover and one under object-contain
// is the whole reason the alpha rung is worth addressing, and a harness that has to
// guess which slots letterbox will guess wrong.
type imageAxes struct {
	Variant string          `json:"variant"`
	Slots   []imageSlotAxes `json:"slots"`
}

// imageSlotAxes is one image slot as the manifest reports it. Frame is the aspect
// the bytes were actually produced at, which on the `ratio` rung is deliberately
// not the frame the UI renders them in — that gap is the assertion.
type imageSlotAxes struct {
	Role    string `json:"role,omitempty"`
	Label   string `json:"label"`
	Frame   string `json:"frame"`
	Contain bool   `json:"contain"`
}

// axesOf takes the dimension rather than just the kind, because a derived
// entity's stored name is not always its text variant: a dimension that owns the
// name is named from the palette, and one that does not — an image dimension — is
// named from its coordinate, or every rung would resolve to the same entity.
// title() is the single answer to "what is this thing actually called", and
// nameAxes.Value promises exactly that.
func axesOf(dim dimension, s spec) axes {
	var a axes
	switch kind := dim.entity; {
	case kind.derived():
		a.Name = &nameAxes{Text: s.text.key, Value: title(dim, s)}
	case kind == kindFilm:
		a.Film = &filmAxes{Cast: s.cast, Scenes: s.scenes}
	default:
		a.Video = &videoAxes{People: s.people, Tags: s.tags, Studios: s.studios, Text: s.text.key}
	}
	a.Image = imageAxesOf(dim.entity, s.image)
	return a
}

// imageAxesOf is nil for the zero rung and for a tag, which has no images at all.
func imageAxesOf(kind entityKind, v imageVariant) *imageAxes {
	slots := slotsFor(kind)
	if v.absent || len(slots) == 0 {
		return nil
	}
	out := &imageAxes{Variant: v.key}
	for _, slot := range slots {
		frame := slot.frame
		if v.useWrong {
			frame = slot.wrong
		}
		out.Slots = append(out.Slots, imageSlotAxes{
			Role: slot.role, Label: slot.label, Frame: frame.String(), Contain: slot.contain,
		})
	}
	return out
}

// manifest is the machine-readable half of the addressing scheme (D4, layer 3).
//
// It answers the two questions the agent actually asks. `Entities` resolves
// "media 123" to what makes 123 special, without reverse-engineering the seeder.
// `ByDimension` enumerates every page sharing a dimension — which is the more
// important of the two, because it turns "fix the page the owner noticed" into
// "check the fix on every page that could have the same bug".
type manifest struct {
	GeneratedAt string              `json:"generated_at"`
	Seed        uint64              `json:"seed"`
	Count       int                 `json:"count"`
	BlockSize   int64               `json:"block_size"`
	PoolBase    int64               `json:"pool_base"`
	Dimensions  []manifestDimension `json:"dimensions"`
	Entities    map[string]entry    `json:"entities"`
	ByDimension map[string][]int64  `json:"by_dimension"`

	// Breadth is the population half of the fixture (HOLODEX-350): how many of each
	// kind exist and where they landed. It is a range and a count rather than
	// entities, because no individual bulk row is worth addressing — a breadth
	// assertion is about how many there are, not which one.
	Breadth *breadthPool `json:"breadth,omitempty"`
}

// manifestDimension describes a dimension itself, so a reader can see the shape
// of the ladder — which blocks are taken, what each one is for — without being
// handed only the entities it happened to produce.
type manifestDimension struct {
	Key    string     `json:"key"`
	Entity entityKind `json:"entity"`
	Block  int64      `json:"block"`
	Finds  string     `json:"finds"`
	Rungs  []string   `json:"rungs"`
}

func buildManifest(entries []entry, seed uint64, count int, pool *breadthPool) manifest {
	m := manifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Seed:        seed,
		Count:       count,
		Breadth:     pool,
		BlockSize:   blockSize,
		PoolBase:    poolBase,
		Entities:    make(map[string]entry, len(entries)),
		ByDimension: map[string][]int64{},
	}
	for _, dim := range ladder {
		rungs := make([]string, 0, len(dim.rungs))
		for _, rg := range dim.rungs {
			rungs = append(rungs, rg.variant)
		}
		m.Dimensions = append(m.Dimensions, manifestDimension{
			Key: dim.key, Entity: dim.entity, Block: dim.block, Finds: dim.finds, Rungs: rungs,
		})
	}
	for _, e := range entries {
		// JSON object keys are strings whatever the Go map says, so the key is
		// written as one here rather than leaving the encoder to decide.
		m.Entities[strconv.FormatInt(e.ID, 10)] = e
		m.ByDimension[e.Dimension] = append(m.ByDimension[e.Dimension], e.ID)
	}
	return m
}

// writeManifest emits the manifest, indented because a human reads it too — it
// is the first thing to open when a fixture entity looks wrong.
func writeManifest(dataPath string, m manifest) (string, error) {
	path := filepath.Join(dataPath, manifestName)
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode manifest: %w", err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", path, err)
	}
	return path, nil
}
