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
// Exactly one half is populated, chosen by the entry's entity kind. spec carries
// both halves so a rung can be written without knowing which it is, but only one
// describes any given entity — a film reporting the video baseline's people=2
// would be a plain falsehood about that film, and an assertion written against it
// would be measuring nothing.
type axes struct {
	Video *videoAxes `json:"video,omitempty"`
	Film  *filmAxes  `json:"film,omitempty"`
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

func axesOf(kind entityKind, s spec) axes {
	if kind == kindFilm {
		return axes{Film: &filmAxes{Cast: s.cast, Scenes: s.scenes}}
	}
	return axes{Video: &videoAxes{People: s.people, Tags: s.tags, Studios: s.studios, Text: s.text.key}}
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

func buildManifest(entries []entry, seed uint64, count int) manifest {
	m := manifest{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Seed:        seed,
		Count:       count,
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
