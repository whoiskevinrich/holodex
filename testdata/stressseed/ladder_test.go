package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/repo"
)

// As with claim_test.go, Go ignores directories named testdata when matching
// ./..., so none of this runs under `make test`. Run it explicitly:
//
//	go test ./testdata/stressseed
//
// These cover the properties the fixture's *addresses* rest on. An assertion
// written against "media 103" is only durable if 103 keeps meaning what it meant
// when the assertion was written, so the invariants below — blocks, OFAT,
// reproducibility — are the ones whose silent breakage would be most expensive.

func TestLadderIsValid(t *testing.T) {
	if err := validateLadder(ladder); err != nil {
		t.Fatalf("the shipped ladder violates its own invariants: %v", err)
	}
}

func TestValidateLadder_RejectsSilentAddressCollisions(t *testing.T) {
	people := func(s *spec, n int) { s.people = n }
	cases := map[string][]dimension{
		"two dimensions sharing a block": {
			{key: "a", entity: kindVideo, block: 100, rungs: counts(people, 1)},
			{key: "b", entity: kindVideo, block: 100, rungs: counts(people, 1)},
		},
		"more rungs than the block holds": {
			{key: "a", entity: kindVideo, block: 100, rungs: make([]rung, blockSize+1)},
		},
		"a block overlapping the supporting-entity pool": {
			{key: "a", entity: kindVideo, block: poolBase, rungs: counts(people, 1)},
		},
		"a repeated variant label": {
			{key: "a", entity: kindVideo, block: 100, rungs: counts(people, 5, 5)},
		},
		"an entity kind with no table or route": {
			{key: "a", entity: entityKind("sprocket"), block: 100, rungs: counts(people, 1)},
		},
		// Dimensions of one entity kind share a single ID sequence, so a block
		// declared out of order gets steered backwards over existing rows.
		"a descending block within an entity kind": {
			{key: "a", entity: kindVideo, block: 200, rungs: counts(people, 1)},
			{key: "b", entity: kindVideo, block: 100, rungs: counts(people, 1)},
		},
		"a dimension with no rungs": {
			{key: "a", entity: kindVideo, block: 100},
		},
	}
	for name, dims := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validateLadder(dims); err == nil {
				t.Fatal("expected validation to refuse this ladder")
			}
		})
	}
}

// D2 — every dimension carries an empty rung. This is the invariant most likely
// to be dropped when someone adds a dimension in a hurry, and it is half the
// layout bug class: HOLODEX-328 existed because empty sections were never seen.
func TestEveryDimensionHasAnEmptyRung(t *testing.T) {
	for _, dim := range ladder {
		empty := false
		for _, rg := range dim.rungs {
			switch v := rg.value.(type) {
			case int:
				empty = empty || v == 0
			case string:
				empty = empty || v == ""
			}
		}
		if !empty {
			t.Errorf("dimension %q has no empty rung — a fixture that never renders the "+
				"zero case ships the empty-state bugs (spec D2)", dim.key)
		}
	}
}

// D3 — a rung varies its own dimension and nothing else. A rung that quietly
// moved a second axis would make every failure in that block unattributable,
// which is the exact failure mode OFAT was chosen to avoid.
func TestRungsVaryExactlyOneAxis(t *testing.T) {
	for _, dim := range ladder {
		for _, rg := range dim.rungs {
			got := baseline()
			rg.apply(&got)

			base := baseline()
			differing := []string{}
			if got.people != base.people {
				differing = append(differing, "people")
			}
			if got.tags != base.tags {
				differing = append(differing, "tags")
			}
			if got.studios != base.studios {
				differing = append(differing, "studios")
			}
			if got.text != base.text {
				differing = append(differing, "text")
			}
			// A rung may equal the baseline on its own axis (people=02 would),
			// so "more than one" is the violation, not "not exactly one".
			if len(differing) > 1 {
				t.Errorf("%s=%s moved %v — a rung must vary one axis only (spec D3)",
					dim.key, rg.variant, differing)
			}
		}
	}
}

// D4 — the addresses are the deliverable. Entities land inside their declared
// block, and supporting entities stay out of the addressable range entirely.
func TestGenerate_EntitiesLandInTheirReservedBlock(t *testing.T) {
	entries, database := seed(t)

	byDim := map[string]dimension{}
	for _, dim := range ladder {
		byDim[dim.key] = dim
	}
	for _, e := range entries {
		dim := byDim[e.Dimension]
		if e.ID < dim.block || e.ID >= dim.block+blockSize {
			t.Errorf("%s=%s landed at %d, outside block [%d,%d)",
				e.Dimension, e.Variant, e.ID, dim.block, dim.block+blockSize)
		}
	}

	for _, table := range []string{"people", "studios", "tags"} {
		var lowest sql.NullInt64
		if err := database.QueryRow(`SELECT MIN(id) FROM ` + table).Scan(&lowest); err != nil {
			t.Fatalf("min id in %s: %v", table, err)
		}
		if lowest.Valid && lowest.Int64 < poolBase {
			t.Errorf("a supporting %s landed at id %d, below poolBase %d — supporting "+
				"entities must never occupy an addressable ID", table, lowest.Int64, poolBase)
		}
	}
}

// AC1 — running twice produces the same fixture. This is what lets an assertion
// outlive a regeneration; without it every `go run` would invalidate every
// address previously written down.
//
// Timestamps are excluded deliberately: UpsertVideo stamps indexed_at with
// time.Now() itself, so byte-identity was never on offer. What has to be stable
// is everything an assertion can address — ids, names, URLs and link counts.
func TestGenerate_IsReproducible(t *testing.T) {
	dir := t.TempDir()
	first := addressable(t, dir)
	second := addressable(t, dir)

	if !reflect.DeepEqual(first, second) {
		t.Errorf("a second run moved the fixture.\nfirst:  %v\nsecond: %v", first, second)
	}
}

// The reset that makes reproducibility work must also clear rows a rung no
// longer produces — otherwise a deleted rung leaves an orphan at an address the
// manifest no longer mentions, which is worse than never having generated it.
func TestGenerate_ClearsRowsFromAPreviousRun(t *testing.T) {
	dir := t.TempDir()
	_, database := seedInto(t, dir)

	if _, err := database.Exec(
		`INSERT INTO videos (file_path, indexed_at, file_mtime) VALUES ('/stress/stale.mp4', '', '')`); err != nil {
		t.Fatalf("insert stale row: %v", err)
	}
	database.Close()

	_, database = seedInto(t, dir)
	var stale int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM videos WHERE file_path = '/stress/stale.mp4'`).Scan(&stale); err != nil {
		t.Fatalf("count stale: %v", err)
	}
	if stale != 0 {
		t.Error("a row from a previous run survived regeneration")
	}
}

// The relationship counts a rung declares must be the counts actually written.
// The reconcile APIs are full-replace, so a loop where a single call belongs
// would leave exactly one link and pass every test that only checked "non-empty".
func TestGenerate_LinkCountsMatchTheSpec(t *testing.T) {
	entries, database := seed(t)

	for _, e := range entries {
		for _, link := range []struct {
			table string
			want  int
		}{
			{"video_people", e.Axes.People},
			{"video_tags", e.Axes.Tags},
			{"video_studios", e.Axes.Studios},
		} {
			var got int
			if err := database.QueryRow(
				`SELECT COUNT(*) FROM `+link.table+` WHERE video_id = ?`, e.ID).Scan(&got); err != nil {
				t.Fatalf("count %s for %d: %v", link.table, e.ID, err)
			}
			if got != link.want {
				t.Errorf("media %d (%s=%s) declares %d rows in %s but has %d",
					e.ID, e.Dimension, e.Variant, link.want, link.table, got)
			}
		}
	}
}

// AC5 — the manifest resolves an ID and enumerates a dimension. Those are the
// two questions it exists to answer; a manifest that can do neither is a log.
func TestManifest_ResolvesAndEnumerates(t *testing.T) {
	dir := t.TempDir()
	entries, database := seedInto(t, dir)
	database.Close()

	path, err := writeManifest(dir, buildManifest(entries, 1, 100))
	if err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var m manifest
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("the manifest is not valid JSON: %v", err)
	}

	if len(m.Entities) != len(entries) {
		t.Fatalf("manifest describes %d entities, %d were generated", len(m.Entities), len(entries))
	}
	for _, e := range entries {
		got, ok := m.Entities[strconv.FormatInt(e.ID, 10)]
		if !ok {
			t.Fatalf("manifest cannot resolve id %d", e.ID)
		}
		if got.Dimension != e.Dimension || got.Variant != e.Variant || got.URL != e.URL {
			t.Errorf("manifest entry for %d disagrees with what was generated: %+v vs %+v", e.ID, got, e)
		}
		if !strings.HasSuffix(got.URL, strconv.FormatInt(e.ID, 10)) {
			t.Errorf("manifest URL %q does not address id %d", got.URL, e.ID)
		}
	}

	for _, dim := range ladder {
		ids := m.ByDimension[dim.key]
		if len(ids) != len(dim.rungs) {
			t.Errorf("dimension %q enumerates %d ids for %d rungs — enumeration is what "+
				"lets a fix be checked on every page sharing a dimension, not just the one "+
				"the owner noticed", dim.key, len(ids), len(dim.rungs))
		}
	}
}

// addressable reduces a seeded fixture to the part an assertion can be written
// against, which is what "reproducible" has to mean here.
func addressable(t *testing.T, dir string) []string {
	t.Helper()
	entries, database := seedInto(t, dir)
	defer database.Close()

	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, strings.Join([]string{
			strconv.FormatInt(e.ID, 10), e.Dimension, e.Variant, e.Name, e.URL,
		}, "|"))
	}
	return out
}

func seed(t *testing.T) ([]entry, *sql.DB) {
	t.Helper()
	return seedInto(t, t.TempDir())
}

// seedInto runs a generation against dir and hands back both halves of the
// result. The caller owns the returned handle; on Windows a lingering one stops
// t.TempDir cleanup, so tests that re-open close it first.
func seedInto(t *testing.T, dir string) ([]entry, *sql.DB) {
	t.Helper()
	database, err := db.Open(filepath.Join(dir, "holodex.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	entries, err := generate(context.Background(), database, repo.New(database), testPersonField(t))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return entries, database
}

// testMappingsYAML is the minimal mapping the seeder needs to build its cast
// through the file layer. Written here rather than read from the developer's own
// metadata-mappings.yaml, so these tests assert on the fixture and not on the
// machine running them.
const testMappingsYAML = `fields:
  - canonical: actors
    label: Actors
    multi: true
    sources:
      - Cast
`

func testMappingsPath(t *testing.T) string {
	t.Helper()
	return writeMappings(t, "valid", testMappingsYAML)
}

func testPersonField(t *testing.T) personField {
	t.Helper()
	pf, err := loadPersonField(testMappingsPath(t))
	if err != nil {
		t.Fatalf("loadPersonField: %v", err)
	}
	return pf
}

func writeMappings(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write mappings: %v", err)
	}
	return path
}

// The cast has to reach the file layer, because that is the layer the server
// re-derives video_people from on startup. A fixture whose people exist only in
// video_people empties itself the first time it is served (see filelayer.go), and
// nothing else in this file would notice — every link assertion would still pass
// against the database the seeder just wrote.
func TestGenerate_CastIsWrittenToTheFileLayer(t *testing.T) {
	entries, database := seed(t)
	pf := testPersonField(t)

	for _, e := range entries {
		var got int
		if err := database.QueryRow(
			`SELECT COUNT(*) FROM video_metadata WHERE video_id = ? AND source_key = ?`,
			e.ID, pf.fileKey).Scan(&got); err != nil {
			t.Fatalf("count file tags for %d: %v", e.ID, err)
		}
		if got != e.Axes.People {
			t.Errorf("media %d (%s=%s) has %d people but %d %q file tags — the startup "+
				"relink resolves from these tags, so a mismatch is a fixture that erases "+
				"itself when served", e.ID, e.Dimension, e.Variant, e.Axes.People, got, pf.fileKey)
		}
	}
}

// A name carrying a resolver separator would be split by the server into more
// people than the manifest claims, so the seeder refuses it rather than shipping
// a page that disagrees with its own address book.
func TestCastTags_RefusesNamesTheResolverWouldSplit(t *testing.T) {
	pf := testPersonField(t)

	for _, name := range []string{"stress person 1/2", "Downey Jr., Robert", "a;b", "two\nlines"} {
		if _, err := pf.castTags([]string{name}); err == nil {
			t.Errorf("expected %q to be refused — the resolver splits on %q",
				name, multiValueSeparators)
		}
	}
	if _, err := pf.castTags([]string{"stress person 001"}); err != nil {
		t.Errorf("a separator-free name must be accepted, got %v", err)
	}
}

// The guard above is only correct while its separator set matches the resolver's.
func TestMultiValueSeparators_MatchTheResolver(t *testing.T) {
	for _, sep := range multiValueSeparators {
		if got := mapping.SplitMulti("a" + string(sep) + "b"); len(got) != 2 {
			t.Errorf("mapping.SplitMulti no longer splits on %q (got %v) — "+
				"multiValueSeparators has drifted from the resolver", sep, got)
		}
	}
}

func TestLoadPersonField_RefusesAMappingThatCannotCarryACast(t *testing.T) {
	cases := map[string]string{
		"no person-typed field at all": `fields:
  - canonical: title
    sources:
      - Title
`,
		// A provider-only cast cannot work: the file layer is what the fixture
		// seeds, and tmdb:/filename: sources resolve from data it does not write.
		"actors mapped to providers only": `fields:
  - canonical: actors
    multi: true
    sources:
      - tmdb:actors
      - filename:people
`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := loadPersonField(writeMappings(t, "invalid", body)); err == nil {
				t.Fatal("expected a refusal: seeding a cast this mapping cannot resolve " +
					"would produce links the next boot deletes")
			}
		})
	}
}
