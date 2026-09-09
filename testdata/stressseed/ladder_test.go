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
	"unicode"
	"unicode/utf8"

	"holodex/internal/db"
	"holodex/internal/mapping"
	"holodex/internal/model"
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
	// Most fixtures below carry a zero rung they do not otherwise need, because
	// validateLadder also enforces D2: without it a case would be refused for
	// having no empty rung rather than for the collision it is meant to provoke,
	// and the whole table would go green while testing one rule seven times. The
	// `blames` substring is the second half of that guard.
	one := func() []rung { return counts(people, 0) }
	named := func() []rung { return texts(textVariant{"single", "x"}) }

	cases := map[string]struct {
		dims   []dimension
		blames string
	}{
		"two dimensions sharing a block": {
			dims: []dimension{
				{key: "a", entity: kindVideo, block: 100, rungs: one()},
				{key: "b", entity: kindVideo, block: 100, rungs: one()},
			},
			blames: "both claim block",
		},
		"more rungs than the block holds": {
			dims:   []dimension{{key: "a", entity: kindVideo, block: 100, rungs: make([]rung, blockSize+1)}},
			blames: "a block holds",
		},
		"a block overlapping the supporting-entity pool": {
			dims:   []dimension{{key: "a", entity: kindVideo, block: poolBase, rungs: one()}},
			blames: "addressable range",
		},
		"a repeated variant label": {
			dims:   []dimension{{key: "a", entity: kindVideo, block: 100, rungs: counts(people, 0, 0)}},
			blames: "repeats the variant",
		},
		"an entity kind with no table or route": {
			dims:   []dimension{{key: "a", entity: entityKind("sprocket"), block: 100, rungs: one()}},
			blames: "no table or route",
		},
		// Dimensions of one entity kind share a single ID sequence, so a block
		// declared out of order gets steered backwards over existing rows.
		"a descending block within an entity kind": {
			dims: []dimension{
				{key: "a", entity: kindVideo, block: 200, rungs: one()},
				{key: "b", entity: kindVideo, block: 100, rungs: one()},
			},
			blames: "blocks must ascend",
		},
		"a dimension with no rungs": {
			dims:   []dimension{{key: "a", entity: kindVideo, block: 100}},
			blames: "declares no rungs",
		},

		// The derived half (HOLODEX-346).
		"a dimension with no empty rung and no stated reason": {
			dims:   []dimension{{key: "a", entity: kindPerson, block: derivedBase, rungs: named()}},
			blames: "noEmptyRung",
		},
		"a dimension claiming it cannot have an empty rung while it has one": {
			dims: []dimension{
				{key: "a", entity: kindVideo, block: 100, noEmptyRung: "invented", rungs: one()},
			},
			blames: "also claims it cannot",
		},
		"a derived block inside the video address space": {
			dims: []dimension{
				{key: "a", entity: kindPerson, block: 100, noEmptyRung: "n/a", rungs: named()},
			},
			blames: "addressable range",
		},
		// A carrier video can only be made once the videos sequence is out of the
		// addressed range, which the first non-video dimension does — so a video
		// dimension after one would steer backwards and collide.
		"a video dimension after a derived one": {
			dims: []dimension{
				{key: "a", entity: kindPerson, block: derivedBase, noEmptyRung: "n/a", rungs: named()},
				{key: "b", entity: kindVideo, block: 100, rungs: one()},
			},
			blames: "must come first",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := validateLadder(tc.dims)
			if err == nil {
				t.Fatal("expected validation to refuse this ladder")
			}
			if !strings.Contains(err.Error(), tc.blames) {
				t.Errorf("refused for the wrong reason — want a message mentioning %q, got:\n%v",
					tc.blames, err)
			}
		})
	}
}

// D2 — every dimension carries an empty rung. This is the invariant most likely
// to be dropped when someone adds a dimension in a hurry, and it is half the
// layout bug class: HOLODEX-328 existed because empty sections were never seen.
func TestEveryDimensionHasAnEmptyRung(t *testing.T) {
	for _, dim := range ladder {
		if hasEmptyRung(dim) {
			continue
		}
		// The only excuse is that the app cannot reach the empty state either, and
		// the dimension has to say so. validateLadder enforces the same rule; this
		// asserts the excuses actually in the shipped table are the ones we meant.
		if dim.noEmptyRung == "" {
			t.Errorf("dimension %q has no empty rung — a fixture that never renders the "+
				"zero case ships the empty-state bugs (spec D2)", dim.key)
			continue
		}
		if !dim.entity.derived() {
			t.Errorf("dimension %q excuses itself from the empty rung, but only the derived "+
				"kinds have a structural reason to: %q", dim.key, dim.noEmptyRung)
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
			if got.cast != base.cast {
				differing = append(differing, "cast")
			}
			if got.scenes != base.scenes {
				differing = append(differing, "scenes")
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

// seededTables names films and videos but neither film_videos nor
// film_people_roles: those are cleared only by ON DELETE CASCADE, which in SQLite
// is silently inert unless `PRAGMA foreign_keys` is on. If it ever were not, a
// re-seed would double every film's scenes and credits while every other
// assertion here still passed — the fixture would be reproducible in name only.
func TestGenerate_ReSeedingDoesNotAccumulateFilmLinks(t *testing.T) {
	dir := t.TempDir()
	_, database := seedInto(t, dir)

	before := map[string]int{}
	for _, table := range []string{"film_videos", "film_people_roles"} {
		before[table] = countRows(t, database, table)
		if before[table] == 0 {
			t.Fatalf("%s is empty after a seed, so this test proves nothing", table)
		}
	}
	database.Close()

	_, database = seedInto(t, dir)
	for table, want := range before {
		if got := countRows(t, database, table); got != want {
			t.Errorf("%s holds %d rows after re-seeding but %d after the first seed — "+
				"rows from the previous run survived", table, got, want)
		}
	}
}

func countRows(t *testing.T, database *sql.DB, table string) int {
	t.Helper()
	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

// The relationship counts a rung declares must be the counts actually written.
// The reconcile APIs are full-replace, so a loop where a single call belongs
// would leave exactly one link and pass every test that only checked "non-empty".
func TestGenerate_LinkCountsMatchTheSpec(t *testing.T) {
	entries, database := seed(t)

	for _, e := range entries {
		var rows []struct {
			table string
			col   string
			want  int
		}
		switch e.Entity {
		case kindVideo:
			rows = append(rows,
				struct {
					table string
					col   string
					want  int
				}{"video_people", "video_id", e.Axes.Video.People},
				struct {
					table string
					col   string
					want  int
				}{"video_tags", "video_id", e.Axes.Video.Tags},
				struct {
					table string
					col   string
					want  int
				}{"video_studios", "video_id", e.Axes.Video.Studios})
		case kindFilm:
			rows = append(rows,
				struct {
					table string
					col   string
					want  int
				}{"film_videos", "film_id", e.Axes.Film.Scenes},
				struct {
					table string
					col   string
					want  int
				}{"film_people_roles", "film_id", e.Axes.Film.Cast})
		}
		for _, link := range rows {
			var got int
			if err := database.QueryRow(
				`SELECT COUNT(*) FROM `+link.table+` WHERE `+link.col+` = ?`, e.ID).Scan(&got); err != nil {
				t.Fatalf("count %s for %d: %v", link.table, e.ID, err)
			}
			if got != link.want {
				t.Errorf("%s %d (%s=%s) declares %d rows in %s but has %d",
					e.Entity, e.ID, e.Dimension, e.Variant, link.want, link.table, got)
			}
		}
	}
}

// Exactly one half of the coordinate describes any entity. A film reporting the
// video baseline's people=2 would be a falsehood the manifest states as fact, and
// an assertion written against it would be measuring nothing.
func TestManifestAxesAreScopedToTheEntityKind(t *testing.T) {
	entries, _ := seed(t)

	for _, e := range entries {
		switch e.Entity {
		case kindVideo:
			if e.Axes.Video == nil || e.Axes.Film != nil {
				t.Errorf("media %d carries axes %+v; a video has no film coordinate", e.ID, e.Axes)
			}
		case kindFilm:
			if e.Axes.Film == nil || e.Axes.Video != nil {
				t.Errorf("film %d carries axes %+v; a film has no video coordinate", e.ID, e.Axes)
			}
		}
	}
}

// Scenes must not be drawn from the addressed video rungs. Attaching one would
// put a film section on a page whose dimension is people or text, so a layout
// failure there could be either cause — the attribution loss OFAT exists to
// prevent (spec D3).
func TestScenesAreDrawnFromThePoolNotFromAddressedRungs(t *testing.T) {
	entries, database := seed(t)

	addressed := map[int64]entry{}
	for _, e := range entries {
		if e.Entity == kindVideo {
			addressed[e.ID] = e
		}
	}

	rows, err := database.Query(`SELECT DISTINCT video_id FROM film_videos`)
	if err != nil {
		t.Fatalf("list scene videos: %v", err)
	}
	defer rows.Close()

	scenes := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		scenes++
		if e, clash := addressed[id]; clash {
			t.Errorf("film scene uses media %d, which is the addressed %s=%s rung — "+
				"that page now has a second varied axis", id, e.Dimension, e.Variant)
		}
		if id < poolBase {
			t.Errorf("scene video %d is inside the addressable range below %d", id, poolBase)
		}
	}
	if scenes == 0 {
		t.Fatal("no film scenes were attached, so this test proved nothing")
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

	entries, err := generate(context.Background(), database, repo.New(database), testFields(t), testTargets(dir))
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	return entries, database
}

// testTargets puts the asset roots under the same directory as the database, the
// way config.derive() does, so a seeded run's images land inside the test's own
// temp dir and the file half of a rung can be inspected.
//
// The maxima are the production defaults from config.Defaults(). They are copied
// rather than imported so a test that starts failing because the app lowered a
// downscale limit fails *here*, naming the fixture, instead of silently storing
// something smaller than the manifest describes.
func testTargets(dir string) imageTargets {
	return imageTargets{
		thumbnailDir: filepath.Join(dir, "thumbnails"),
		personDir:    filepath.Join(dir, "person-images"),
		studioDir:    filepath.Join(dir, "studio-images"),
		filmDir:      filepath.Join(dir, "film-images"),
		personMaxDim: 2000,
		studioMaxDim: 1000,
		filmMaxDim:   1500,
	}
}

// testMappingsYAML is the minimal mapping the seeder needs to build its derived
// links through the file layer. Written here rather than read from the shipped
// testdata/stressseed/mappings.yaml, so a test failure names the seeder rather
// than the mapping, and the mapping's own obligations are asserted separately by
// TestShippedMappingSatisfiesTheLadder.
const testMappingsYAML = `fields:
  - canonical: actors
    label: Actors
    multi: true
    sources:
      - Cast
  - canonical: studio
    label: Studio
    multi: true
    sources:
      - Publisher
  - canonical: overview
    label: Overview
    sources:
      - Comment
`

func testMappingsPath(t *testing.T) string {
	t.Helper()
	return writeMappings(t, "valid", testMappingsYAML)
}

func testFields(t *testing.T) fixtureFields {
	t.Helper()
	ff, err := loadFields(testMappingsPath(t), demands(ladder))
	if err != nil {
		t.Fatalf("loadFields: %v", err)
	}
	return ff
}

func writeMappings(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write mappings: %v", err)
	}
	return path
}

// The derived links have to reach the file layer, because that is the layer the
// server re-derives video_people and video_studios from on startup. A fixture
// whose links exist only in those tables empties itself the first time it is
// served (see filelayer.go), and nothing else in this file would notice — every
// link assertion would still pass against the database the seeder just wrote.
func TestGenerate_DerivedLinksAreWrittenToTheFileLayer(t *testing.T) {
	entries, database := seed(t)
	ff := testFields(t)

	for _, e := range entries {
		if e.Entity != kindVideo {
			continue
		}
		for _, want := range []struct {
			field fileField
			count int
		}{{ff.person, e.Axes.Video.People}, {ff.studio, e.Axes.Video.Studios}} {
			var got int
			if err := database.QueryRow(
				`SELECT COUNT(*) FROM video_metadata WHERE video_id = ? AND source_key = ?`,
				e.ID, want.field.fileKey).Scan(&got); err != nil {
				t.Fatalf("count file tags for %d: %v", e.ID, err)
			}
			if got != want.count {
				t.Errorf("media %d (%s=%s) declares %d %s but has %d %q file tags — the "+
					"startup relink resolves from these tags, so a mismatch is a fixture "+
					"that erases itself when served",
					e.ID, e.Dimension, e.Variant, want.count, want.field.canonical, got, want.field.fileKey)
			}
		}
	}
}

// A name carrying a resolver separator would be split by the server into more
// people than the manifest claims, so the seeder refuses it rather than shipping
// a page that disagrees with its own address book.
func TestLinkTags_RefusesNamesTheResolverWouldSplit(t *testing.T) {
	pf := testFields(t).person

	for _, name := range []string{"stress person 1/2", "Downey Jr., Robert", "a;b", "two\nlines"} {
		if _, err := pf.linkTags([]string{name}); err == nil {
			t.Errorf("expected %q to be refused — the resolver splits on %q",
				name, multiValueSeparators)
		}
	}
	if _, err := pf.linkTags([]string{"stress person 001"}); err != nil {
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

func TestLoadFields_RefusesAMappingThatCannotCarryTheLadder(t *testing.T) {
	// Every case names the field it is breaking, and the assertion checks the
	// refusal actually mentions it. Without that, a case that broke `studio` would
	// still "pass" once loadFields grew a check for some unrelated field it also
	// omits — the whole table would go green while testing one thing repeatedly.
	// That is not hypothetical: adding the `overview` requirement to loadFields is
	// exactly what would have done it.
	const (
		actors   = "  - canonical: actors\n    multi: true\n    sources:\n      - Cast\n"
		studio   = "  - canonical: studio\n    multi: true\n    sources:\n      - Publisher\n"
		overview = "  - canonical: overview\n    sources:\n      - Comment\n"
	)
	cases := map[string]struct {
		body   string
		blames string
	}{
		"no person-typed field at all": {
			body: "fields:\n" + studio + overview, blames: "person-typed",
		},
		// A provider-only source cannot work: the file layer is what the fixture
		// seeds, and tmdb:/filename: sources resolve from data it does not write.
		"actors mapped to providers only": {
			body: "fields:\n" +
				"  - canonical: actors\n    multi: true\n    sources:\n      - tmdb:actors\n      - filename:people\n" +
				studio + overview,
			blames: "person-typed",
		},
		"studio not mapped at all": {
			body: "fields:\n" + actors + overview, blames: "studio",
		},
		// The one HOLODEX-347 exists for: a REPLACE studio resolves through
		// firstNonEmpty, so every rung above 1 would collapse to 1 and the manifest
		// would claim a cardinality the page never renders.
		"studio is a replace field": {
			body:   "fields:\n" + actors + "  - canonical: studio\n    sources:\n      - Publisher\n" + overview,
			blames: "studio",
		},
		"actors is a replace field": {
			body:   "fields:\n" + "  - canonical: actors\n    sources:\n      - Cast\n" + studio + overview,
			blames: "actors",
		},
		// The text half (HOLODEX-346). There is no videos.overview column, so an
		// unmapped overview means the rung writes a row nothing resolves.
		"overview not mapped at all": {
			body: "fields:\n" + actors + studio, blames: "overview",
		},
		// The mirror of the studio case: a MULTI overview is run through SplitMulti,
		// so the lorem rung arrives as two dozen comma-separated fragments instead of
		// the paragraph the fixture claims.
		"overview is a multi field": {
			body: "fields:\n" + actors + studio +
				"  - canonical: overview\n    multi: true\n    sources:\n      - Comment\n",
			blames: "overview",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := loadFields(writeMappings(t, "invalid", tc.body), demands(ladder))
			if err == nil {
				t.Fatal("expected a refusal: seeding a field this mapping cannot resolve " +
					"would produce a fixture that disagrees with the page serving it")
			}
			if !strings.Contains(err.Error(), tc.blames) {
				t.Errorf("refusal does not name the broken field %q, so this case could be "+
					"passing for an unrelated reason:\n%v", tc.blames, err)
			}
		})
	}
}

// The shipped mapping is part of the fixture's contract, not a sample: the
// `backend-stress` profile hands the server this exact file, so a ladder the
// mapping cannot express is a fixture that lies on every page.
func TestShippedMappingSatisfiesTheLadder(t *testing.T) {
	// stressMappingsPath is relative to the repository root, which is where the
	// seeder and the `backend-stress` server both run from; `go test` runs from the
	// package directory. Walking back up rather than hardcoding "mappings.yaml"
	// keeps the constant itself under test — a typo in it would fail here instead
	// of at the next seed.
	path := filepath.Join("..", "..", stressMappingsPath)
	if _, err := loadFields(path, demands(ladder)); err != nil {
		t.Fatalf("%s cannot express the ladder: %v", stressMappingsPath, err)
	}
}

// The scene pool consumes the videos sequence once the film half starts, and
// SQLite's AUTOINCREMENT counter cannot be rewound below rows that already
// exist — so a video dimension after a film one would collide rather than land
// in its block. Silent corruption, hence a table-level refusal.
func TestValidateLadder_RejectsAVideoDimensionAfterAFilmOne(t *testing.T) {
	err := validateLadder([]dimension{
		{key: "f", entity: kindFilm, block: 100, rungs: counts(func(s *spec, n int) { s.scenes = n }, 0, 1)},
		{key: "v", entity: kindVideo, block: 200, rungs: counts(func(s *spec, n int) { s.people = n }, 0, 1)},
	})
	if err == nil {
		t.Fatal("expected a refusal: the scene pool has already taken the videos sequence")
	}
	if !strings.Contains(err.Error(), "must come first") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
}

// Film cast is drawn from the people the video ladder creates, so a cast rung
// above the top people rung would fail per-entity, mid-seed, with a "person not
// found" that names neither cause.
func TestValidateLadder_RejectsACastLadderTallerThanThePeopleLadder(t *testing.T) {
	err := validateLadder([]dimension{
		{key: "people", entity: kindVideo, block: 100, rungs: counts(func(s *spec, n int) { s.people = n }, 0, 5)},
		{key: "filmcast", entity: kindFilm, block: 200, rungs: counts(func(s *spec, n int) { s.cast = n }, 0, 50)},
	})
	if err == nil {
		t.Fatal("expected a refusal: the person pool never reaches 50")
	}
	if !strings.Contains(err.Error(), "film cast ladder reaches") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
}

// --- The text palette (HOLODEX-346) ---------------------------------------
//
// The palette is this ticket's whole deliverable, and every one of its rungs can
// decay into a weaker test without anything failing: shorten the lorem and it
// stops overflowing, drop the joiners out of the emoji rung and it stops being
// multi-codepoint, put a space in the unbroken token and it stops breaking flex
// containers. None of that is visible in review — the joiners and combining
// marks are literally invisible in the source line — so each rung's defining
// property is asserted here rather than trusted to the string.

// paletteValue looks a rung up by key so a test names the rung it is about.
func paletteValue(t *testing.T, key string) string {
	t.Helper()
	for _, v := range textPalette {
		if v.key == key {
			return v.value
		}
	}
	t.Fatalf("the palette has no %q rung", key)
	return ""
}

// The AC's checklist, as a checklist. Its job is to fail when a rung is deleted —
// the properties each one has to have are asserted separately below.
func TestTextPaletteCoversEveryCategoryTheTicketNamed(t *testing.T) {
	for _, key := range []string{
		"empty", "single", "lorem", "unbroken", "cjk", "rtl", "bidi", "emoji", "diacritics",
	} {
		paletteValue(t, key)
	}
}

func TestLoremRungIsLongEnoughToStillOverflow(t *testing.T) {
	if n := utf8.RuneCountInString(paletteValue(t, "lorem")); n < loremMin {
		t.Errorf("the lorem rung is %d characters, below loremMin=%d — past a line clamp "+
			"it stops overflowing at all, and the rung would pass by having become a "+
			"different test", n, loremMin)
	}
}

// 60+ characters is only half of it: what breaks a flex container is the absence
// of anywhere to break. A space or a hyphen gives the browser a wrap opportunity
// and the rung quietly becomes an ordinary long title.
func TestUnbrokenRungHasNowhereToBreak(t *testing.T) {
	v := paletteValue(t, "unbroken")
	if n := utf8.RuneCountInString(v); n < 60 {
		t.Errorf("the unbroken rung is %d characters; the ticket asks for 60+", n)
	}
	for _, r := range v {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			t.Errorf("the unbroken rung contains %q, which a browser can break at — the "+
				"rung has to have no wrap opportunity at all", r)
		}
	}
}

// The two direction rungs are a pair and only work as one: `rtl` finds alignment
// and direction bugs, `bidi` finds neutrals (digits, brackets, the em dash)
// taking their direction from whichever run they sit beside. If `rtl` picked up
// Latin text the pair would collapse into one test run twice.
func TestDirectionRungsAreAPair(t *testing.T) {
	hasRTL := func(s string) bool {
		for _, r := range s {
			if unicode.Is(unicode.Arabic, r) || unicode.Is(unicode.Hebrew, r) {
				return true
			}
		}
		return false
	}
	hasLatin := func(s string) bool {
		for _, r := range s {
			if r < utf8.RuneSelf && unicode.IsLetter(r) {
				return true
			}
		}
		return false
	}

	if rtl := paletteValue(t, "rtl"); !hasRTL(rtl) || hasLatin(rtl) {
		t.Errorf("the rtl rung must be RTL with no Latin run, got %q", rtl)
	}
	if bidi := paletteValue(t, "bidi"); !hasRTL(bidi) || !hasLatin(bidi) {
		t.Errorf("the bidi rung must change direction mid-string, got %q", bidi)
	}
}

// A run of 🎬 is not what the ticket asked for. The failure mode is the
// multi-codepoint grapheme cluster: anything counting runes, slicing bytes or
// sizing a line box per code point breaks on a ZWJ family and not on a single
// emoji. The joiners and selectors are invisible in ladder.go, so an editor
// normalising the file could strip them with nothing to show for it.
func TestEmojiRungKeepsItsMultiCodepointSequences(t *testing.T) {
	v := paletteValue(t, "emoji")
	runes := []rune(v)

	regionalPair := false
	for i := 1; i < len(runes); i++ {
		isRI := func(r rune) bool { return r >= 0x1F1E6 && r <= 0x1F1FF }
		if isRI(runes[i-1]) && isRI(runes[i]) {
			regionalPair = true
			break
		}
	}
	skinTone := false
	for _, r := range runes {
		if r >= 0x1F3FB && r <= 0x1F3FF {
			skinTone = true
			break
		}
	}

	for _, c := range []struct {
		what string
		ok   bool
	}{
		{"a zero-width joiner (U+200D), e.g. the 👨‍👩‍👧‍👦 family", strings.ContainsRune(v, '\u200D')},
		{"a variation selector (U+FE0F)", strings.ContainsRune(v, '\uFE0F')},
		{"a skin-tone modifier (U+1F3FB–U+1F3FF)", skinTone},
		{"a flag built from two regional indicators", regionalPair},
	} {
		if !c.ok {
			t.Errorf("the emoji rung no longer contains %s, so it is testing single "+
				"code points rather than multi-codepoint sequences", c.what)
		}
	}
}

// "Zalgo-lite" means the marks outnumber the letters they sit on — that is what
// grows the line box. A string with one accent per letter renders at normal
// height and finds nothing.
func TestDiacriticsRungStacksMarksAboveTheBaseCharacters(t *testing.T) {
	marks, base := 0, 0
	for _, r := range paletteValue(t, "diacritics") {
		switch {
		case unicode.Is(unicode.Mn, r):
			marks++
		case !unicode.IsSpace(r):
			base++
		}
	}
	if marks <= base {
		t.Errorf("the diacritics rung has %d combining marks over %d base characters — "+
			"stacked marks are what blow the line box out, and one mark per letter does "+
			"not", marks, base)
	}
}

// The overview is the second free-text field on a video and the only one with no
// column behind it: it resolves from the file layer or not at all. A rung that
// wrote the title and forgot the overview would look complete on the browse card
// and leave the detail page's other text container untested.
func TestOverviewCarriesTheTextPalette(t *testing.T) {
	entries, database := seed(t)
	ff := testFields(t)

	byVariant := map[string]entry{}
	for _, e := range entries {
		if e.Dimension == "text" {
			byVariant[e.Variant] = e
		}
	}
	for _, v := range textPalette {
		e, ok := byVariant[v.key]
		if !ok {
			t.Fatalf("the text dimension produced no entity for the %q rung", v.key)
		}

		var title string
		if err := database.QueryRow(`SELECT title FROM videos WHERE id = ?`, e.ID).Scan(&title); err != nil {
			t.Fatalf("read title of %d: %v", e.ID, err)
		}
		if title != v.value {
			t.Errorf("media %d (text=%s) has title %q, want the raw variant %q — the text "+
				"dimension owns the title, so a coordinate prefix here would un-break the "+
				"unbroken rung and un-empty the empty one", e.ID, v.key, title, v.value)
		}

		got := overviewRows(t, database, e.ID, ff.overview.fileKey)
		if v.value == "" {
			if len(got) != 0 {
				t.Errorf("media %d (text=empty) wrote overview rows %q — the empty rung "+
					"has to be absent rather than blank, which is what the page renders "+
					"differently", e.ID, got)
			}
			continue
		}
		if len(got) != 1 || got[0] != v.value {
			t.Errorf("media %d (text=%s) has overview rows %q, want exactly one carrying "+
				"the variant", e.ID, v.key, got)
		}
	}
}

func overviewRows(t *testing.T, database *sql.DB, videoID int64, sourceKey string) []string {
	t.Helper()
	rows, err := database.Query(
		`SELECT value FROM video_metadata WHERE video_id = ? AND source_key = ?`, videoID, sourceKey)
	if err != nil {
		t.Fatalf("read overview of %d: %v", videoID, err)
	}
	defer func() { _ = rows.Close() }()

	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan overview of %d: %v", videoID, err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read overview of %d: %v", videoID, err)
	}
	return out
}

// --- The derived kinds (HOLODEX-346) --------------------------------------
//
// A person, studio and tag each has a detail page where its name is the h1, so
// the palette has to reach those names too. They cannot be created from a name
// alone, which is what makes this half structurally different from the video
// one: every rung seeds a carrier video, and the entity is produced by the
// server's own derivation rather than written directly.

// The exclusions are the interesting part of namePalette, so they are asserted
// against the limits they claim rather than against a hardcoded list. A rung
// dropped for no reason, or kept despite a reason, both fail here.
func TestNamePaletteExcludesExactlyWhatThePlatformRejects(t *testing.T) {
	for _, kind := range []entityKind{kindPerson, kindStudio, kindTag} {
		t.Run(string(kind), func(t *testing.T) {
			kept := map[string]bool{}
			for _, v := range namePalette(kind) {
				kept[v.key] = true
			}
			for _, v := range textPalette {
				reason := nameRejects(kind, v)
				if kept[v.key] == (reason != "") {
					t.Errorf("rung %q: kept=%v but reason=%q — a rung is dropped if and only "+
						"if the platform rejects it", v.key, kept[v.key], reason)
				}
			}

			// And the reasons have to be true, not merely stated.
			for _, v := range textPalette {
				switch reason := nameRejects(kind, v); {
				case reason == "":
					if v.value == "" {
						t.Errorf("rung %q kept for %s, but every derived kind refuses an "+
							"empty name", v.key, kind)
					}
					if kind == kindTag && utf8.RuneCountInString(v.value) > model.MaxNameLen {
						t.Errorf("rung %q kept for tag at %d runes, over MaxNameLen %d",
							v.key, utf8.RuneCountInString(v.value), model.MaxNameLen)
					}
					if kind != kindTag && strings.ContainsAny(v.value, multiValueSeparators) {
						t.Errorf("rung %q kept for %s despite carrying a resolver separator",
							v.key, kind)
					}
				case v.value != "" && kind != kindTag:
					if !strings.ContainsAny(v.value, multiValueSeparators) {
						t.Errorf("rung %q dropped for %s with no separator in it: %s",
							v.key, kind, reason)
					}
				}
			}
		})
	}
}

// Tags are lowercased on the way in (curationNorm — the "fox"/"Fox" fix), so a
// mixed-case palette string would be stored as something other than what the
// manifest records. namePalette lowercases first; this is the assertion that the
// two normalisations still agree.
func TestTagRungsAreSeededInTheFormTheRepoStores(t *testing.T) {
	for _, v := range namePalette(kindTag) {
		if got := strings.ToLower(v.value); got != v.value {
			t.Errorf("tag rung %q is seeded as %q but the repo stores %q — the manifest "+
				"would name a tag that does not exist", v.key, v.value, got)
		}
	}
}

// The HOLODEX-344 lesson applied to the derived half: a person or studio that
// exists only because the seeder inserted it is erased by the first relink. Each
// one has to be reproducible from its carrier's file layer, which is exactly what
// the startup backfill resolves from.
func TestDerivedEntitiesAreReproducibleFromTheFileLayer(t *testing.T) {
	entries, database := seed(t)
	ff := testFields(t)

	for _, e := range entries {
		if !e.Entity.derived() || e.Entity == kindTag {
			continue // tags are authored, not derived (ADR-075 D3) — no file tag to check
		}
		field := ff.person
		if e.Entity == kindStudio {
			field = ff.studio
		}

		var carrier int64
		var value string
		if err := database.QueryRow(`
			SELECT m.video_id, m.value FROM video_metadata m
			WHERE m.source_key = ? AND m.value = ?`,
			field.fileKey, e.Axes.Name.Value).Scan(&carrier, &value); err != nil {
			t.Errorf("%s=%s (id %d) has no %q file tag carrying its name, so the startup "+
				"relink would resolve it to nothing and orphan it: %v",
				e.Dimension, e.Variant, e.ID, field.fileKey, err)
			continue
		}
		// The carrier exists to make the entity; it must not be addressable itself,
		// or a layout failure on it would look like a failure of a rung.
		if carrier < poolBase {
			t.Errorf("%s=%s is carried by video %d, inside the addressed range below %d",
				e.Dimension, e.Variant, carrier, poolBase)
		}
	}
}

// The derived blocks live above the pool rather than below it, which inverts the
// usual rule — so the thing worth asserting is that the two ranges stay disjoint:
// every addressed derived entity inside its block, every supporting one outside
// every block.
func TestDerivedBlocksDoNotOverlapTheSupportingPool(t *testing.T) {
	entries, database := seed(t)

	addressed := map[entityKind]map[int64]bool{}
	for _, e := range entries {
		if !e.Entity.derived() {
			continue
		}
		if addressed[e.Entity] == nil {
			addressed[e.Entity] = map[int64]bool{}
		}
		addressed[e.Entity][e.ID] = true
	}

	for _, dim := range ladder {
		if !dim.entity.derived() {
			continue
		}
		rows, err := database.Query(
			`SELECT id FROM `+dim.entity.table()+` WHERE id >= ? AND id < ?`,
			dim.block, dim.block+blockSize)
		if err != nil {
			t.Fatalf("read %s block: %v", dim.key, err)
		}
		found := 0
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				t.Fatalf("scan %s block: %v", dim.key, err)
			}
			found++
			if !addressed[dim.entity][id] {
				t.Errorf("a supporting %s landed at id %d, inside %q's reserved block",
					dim.entity, id, dim.key)
			}
		}
		_ = rows.Close()
		if found != len(dim.rungs) {
			t.Errorf("block for %q holds %d rows, want one per rung (%d)",
				dim.key, found, len(dim.rungs))
		}
	}
}

// Every block in the fixture rests on the assumption that steering only ever
// moves a counter forward. validateLadder's ordering rules are what keep that
// true, so this is the assertion at the point of use: if they were ever wrong,
// the seeder stops rather than placing an entity inside a block that already
// belongs to something else — an id that no later check can tell apart from a
// correct one, because it is still inside the block it was meant to be in.
func TestSteer_RefusesToRewindOverExistingRows(t *testing.T) {
	_, database := seed(t)

	for _, table := range []string{"videos", "people", "studios", "tags"} {
		if err := steer(t.Context(), database, table, 1); err == nil {
			t.Errorf("steering %s back to 1 was accepted, but the table is full of rows "+
				"above it", table)
		}
	}
	// Forward is still allowed, or the fixture could not be built at all.
	if err := steer(t.Context(), database, "films", derivedCeiling); err != nil {
		t.Errorf("steering forward must still work: %v", err)
	}
}
