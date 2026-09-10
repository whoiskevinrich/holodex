// Command stressseed builds the dev-time stress fixture (HOLODEX-342): a
// deliberately adversarial library whose entities are worse than anything real
// data produces, so layout and UX bugs surface locally instead of reaching the
// owner.
//
//	go run ./testdata/stressseed -mappings <the profile's mappings file>
//	go run ./testdata/stressseed -big            # shorthand for -count 2000
//	go run ./testdata/stressseed -data ./data/x  # somewhere else
//	rm -rf ./data/stress                         # teardown — the whole fixture
//
// -mappings must name the same file the server will read; see filelayer.go for
// why the two have to agree. It defaults to the fixture's own committed mapping,
// which the `backend-stress` profile also points the server at.
//
// What varies is declared as a table in ladder.go, not written as loops: each
// dimension reserves an ID block, and each of its rungs mutates a neutral
// baseline in exactly one axis (HOLODEX-344). Every run clears the fixture's own
// rows first and steers each ID sequence to the block that owns it, so an
// entity's address is a function of the table alone and stays put across runs.
// manifest.json records what landed where.
//
// The fixture never shares a DATA_PATH with a real library (spec D5). Two things
// enforce that. First, paths are derived from config.Defaults() plus the -data
// flag alone — never config.Load — so no DATA_PATH env var, .env, or holodex.yaml
// can redirect the seeder onto real data. Second, the seeder refuses to run
// against a database holding rows it did not create; see claim.go.
//
// Run the fixture with the `backend-stress` profile in .claude/launch.json, which
// points the server at the same directory with FILMS_ENABLED=true. Stop that
// server first: it holds its own connection, and two writers on one SQLite file
// contend past the busy timeout.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"holodex/internal/config"
	"holodex/internal/db"
	"holodex/internal/repo"
)

const (
	defaultDataPath = "./data/stress"
	defaultCount    = 100
	bigCount        = 2000
)

func main() {
	dataPath := flag.String("data", defaultDataPath, "isolated data directory for the fixture")
	count := flag.Int("count", defaultCount, "how many of every kind the breadth pool carries")
	big := flag.Bool("big", false, fmt.Sprintf("shorthand for -count %d (pagination, scroll perf)", bigCount))
	seed := flag.Uint64("seed", 1, "RNG seed — the same seed reproduces the same fixture")
	mappings := flag.String("mappings", defaultMappingsPath(),
		"metadata-mappings.yaml the fixture builds its file layer against")
	flag.Parse()

	// An explicit -count wins over -big, so the two can be combined without the
	// shorthand silently overwriting the specific number.
	if *big && !flagWasSet("count") {
		*count = bigCount
	}

	if err := run(*dataPath, *count, *seed, *mappings, personasPath); err != nil {
		log.Fatalf("stressseed: %v", err)
	}
}

func flagWasSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

// stressMappingsPath is the mapping the fixture owns, relative to the repo root
// the seeder is run from. See mappings.yaml for why the fixture does not borrow
// the operator's: the mapping decides which file tag carries each derived link,
// and `studio` has to be `multi: true` for the studios ladder to be expressible
// at all.
const stressMappingsPath = "testdata/stressseed/mappings.yaml"

// stressSourcesPath is the fixture's provider registry — the same argument as the
// mapping, one layer out (HOLODEX-348). The seeder never reads it; the SERVER
// does, via METADATA_SOURCES_PATH in the `backend-stress` profile. It is named
// here so the drift guard between it and the persona table has one place to point
// at, and so the two halves of the fixture's committed configuration sit together.
const stressSourcesPath = "testdata/stressseed/sources.yaml"

// defaultMappingsPath deliberately ignores METADATA_MAPPINGS_PATH, unlike the
// server. Honouring it would mean a shell that had exported it for the `backend`
// profile silently redirected the fixture onto the operator's own mapping, whose
// replace-mode `studio` collapses the studios ladder — the exact failure the
// committed file exists to prevent. `backend-stress` points the server at the
// same path, and -mappings overrides both halves together when it has to.
func defaultMappingsPath() string {
	return stressMappingsPath
}

func run(dataPath string, count int, seed uint64, mappingsPath, personasFile string) error {
	if count < 0 {
		return fmt.Errorf("-count %d: must not be negative", count)
	}
	// Before anything is opened or cleared. seedBreadthVideos refuses an oversized
	// -count too, but by then reset() has emptied the previous fixture and the
	// addressed video dimensions have been rebuilt — so a typo would cost the
	// operator the fixture it was about to refuse to replace.
	if count > breadthCeiling() {
		return fmt.Errorf("-count %d exceeds %d, the most the pool range [%d,%d) can hold",
			count, breadthCeiling(), poolBase, derivedBase)
	}

	cfg := config.Defaults()
	cfg.ApplyOverrides(config.Overrides{DataPath: dataPath})

	// Before touching anything: a mapping that cannot carry a cast means the
	// people ladder would be erased on the next boot, so fail now rather than
	// after writing a fixture that destroys itself.
	ff, err := loadFields(mappingsPath, personasFile, demands(ladder))
	if err != nil {
		return err
	}

	c, err := inspect(cfg.DatabasePath)
	if err != nil {
		return err
	}
	if len(c.foreign) > 0 {
		return fmt.Errorf("%s holds rows this tool did not create (%s).\n"+
			"Refusing to write into it. Point -data at a directory of the fixture's own,\n"+
			"or remove that one first: rm -rf %s",
			cfg.DatabasePath, strings.Join(c.foreign, ", "), dataPath)
	}
	// Re-seeding half a fixture is worse than not seeding it: the rows from the
	// old seed stay, and nothing about the result is reproducible any more.
	if c.claimed && c.seed != seed {
		return fmt.Errorf("%s was seeded with -seed %d, not %d.\n"+
			"Regenerate from scratch instead: rm -rf %s", cfg.DatabasePath, c.seed, seed, dataPath)
	}

	database, err := db.Open(cfg.DatabasePath) // creates the directory, applies migrations
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	if err := markOwned(ctx, database, seed, count); err != nil {
		return err
	}

	// The manifest is the fixture's completion marker, so it has to go before the
	// rebuild starts rather than merely being overwritten after it finishes. The claim
	// marker is committed above and reset() commits immediately below, so a run that
	// dies in between — a full disk, or the `backend-stress` server still holding the
	// database past the busy timeout — leaves a half-rebuilt fixture. Addresses are
	// steered, so the surviving entities have the ids and names the *old* manifest
	// describes: the geometry harness's preflight compares one of them and passes, then
	// measures rungs that were never rebuilt and reports layout verdicts for a seeding
	// failure. Removing it first makes the failure say what it is — `npm run geometry`
	// then prints manifest.mjs's "Seed one first" instead of a plausible green run.
	if err := os.Remove(manifestPathIn(cfg.DataPath)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("clear previous manifest: %w", err)
	}

	// The `backend-stress` profile points MEDIA_PATH at this directory, and it is
	// deliberately empty: the scanner walks it, sees zero files, and then skips
	// its end-of-scan deactivation sweep ("scan saw zero media files"), so it
	// cannot deactivate seeded rows — which have no files behind them by design
	// (spec D1). Pointing the profile at a real library instead would let the
	// scanner mix real media into the fixture.
	mediaPath := filepath.Join(dataPath, "media")
	if err := os.MkdirAll(mediaPath, 0o755); err != nil {
		return fmt.Errorf("create empty media dir: %w", err)
	}

	// The maxima are the server's own configured downscale limits, not numbers
	// chosen here: Normalize applies them on every real ingest, so passing anything
	// else would store an image at a size the running app never would — and the
	// wrong-ratio rung is precisely a claim about stored size (images.go).
	targets := imageTargets{
		thumbnailDir: cfg.ThumbnailPath,
		personDir:    cfg.PersonImagePath,
		studioDir:    cfg.StudioImagePath,
		filmDir:      cfg.FilmImagePath,
		personMaxDim: cfg.PersonImageMaxDimension,
		studioMaxDim: cfg.StudioImageMaxDimension,
		filmMaxDim:   cfg.FilmImageMaxDimension,
	}

	entries, pool, err := generate(ctx, database, repo.New(database), ff, targets, count)
	if err != nil {
		return err
	}
	manifestPath, err := writeManifest(cfg.DataPath, buildManifest(entries, seed, count, pool))
	if err != nil {
		return err
	}

	report(cfg, mediaPath, manifestPath, ff, entries, pool, seed)
	return nil
}

// report prints where the fixture landed and what it addressed, so an operator
// can see every path the tool considers its own before trusting the isolation
// claim — and can find a dimension's block without opening the manifest.
func report(cfg config.Config, mediaPath, manifestPath string, ff fixtureFields, entries []entry, pool *breadthPool, seed uint64) {
	fmt.Printf("fixture claimed at %s (seed %d, count %d)\n\n", cfg.DataPath, seed, pool.Count)
	for _, p := range []struct{ label, path string }{
		{"database", cfg.DatabasePath},
		{"manifest", manifestPath},
		{"thumbnails", cfg.ThumbnailPath},
		{"person images", cfg.PersonImagePath},
		{"studio images", cfg.StudioImagePath},
		{"film images", cfg.FilmImagePath},
		{"provider icons", cfg.ProviderIconPath},
		{"media (empty)", mediaPath},
	} {
		fmt.Printf("  %-16s %s\n", p.label, p.path)
	}

	// Naming the tags is worth the lines: they are the one input that has to match
	// the server's, and a mismatch shows up as silently empty links rather than an
	// error (filelayer.go).
	fmt.Printf("\nwritten through the file layer, so the server resolves them from the same\n" +
		"place it would resolve real media — and the startup relink re-derives the\n" +
		"links instead of wiping them:\n")
	for _, f := range []fileField{ff.person, ff.studio, ff.overview} {
		fmt.Printf("  %-8s file tag %q\n", f.canonical, f.fileKey)
	}

	// Said out loud because an empty Activity page looks like a bug: reset() clears
	// job_runs so the startup relink re-runs on every boot (see seededTables), and that
	// takes every other subsystem's history with it.
	fmt.Printf("\nSystem Activity starts empty — job history is cleared so the startup\n" +
		"relink runs again on the next boot rather than being skipped as already-done.\n")

	fmt.Printf("\n%d entities across %d dimensions:\n", len(entries), len(ladder))
	width := 0
	for _, dim := range ladder {
		width = max(width, len(dim.key))
	}
	for _, dim := range ladder {
		variants := make([]string, 0, len(dim.rungs))
		for _, rg := range dim.rungs {
			variants = append(variants, rg.variant)
		}
		fmt.Printf("  %-*s %d-%d  %s\n", width, dim.key, dim.block,
			dim.block+int64(len(dim.rungs))-1, strings.Join(variants, " "))
	}

	printBreadth(pool)

	// A dimension that is quietly short of a rung looks identical to one that never
	// had it. The derived kinds cannot carry the whole palette — an empty name is
	// skipped by the reconcile, an over-long one is rejected outright — so the
	// omissions are printed with their cause, or the next person to look would file
	// the gap as a bug in the fixture.
	printPaletteExclusions()
	fmt.Printf("\nServe it with the `backend-stress` launch profile; tear it down with rm -rf %s\n", cfg.DataPath)
}

// printBreadth reports the population half of the fixture: how many of each kind
// the breadth pool added, and the id range they occupy.
//
// The range is printed rather than only the count because it is the claim worth
// checking by eye — every bulk row sits above poolBase and below derivedBase, so
// a number outside that gap means an entity landed on an address the ladder had
// reserved, which is the one failure the block scheme exists to prevent.
func printBreadth(pool *breadthPool) {
	if pool.Count == 0 {
		fmt.Printf("\nno breadth pool (-count 0): the ladder alone, for a fast reseed\n")
		return
	}
	fmt.Printf("\n%d of every kind in the breadth pool, ids in [%d,%d) and addressed by\n"+
		"nobody — the population axis, for pagination and scroll perf:\n", pool.Count, poolBase, derivedBase)
	for _, table := range []string{"videos", "people", "studios", "tags", "films", categoriesTable} {
		r, ok := pool.Tables[table]
		if !ok {
			continue
		}
		fmt.Printf("  %-12s %5d  %d-%d\n", table, r.N, r.Lo, r.Hi)
	}
}

// printPaletteExclusions reports every text rung a derived kind cannot be named
// with, and why. Reasons are grouped so the common ones are stated once.
func printPaletteExclusions() {
	type exclusion struct {
		kind entityKind
		rung string
	}
	byReason := map[string][]exclusion{}
	var order []string
	for _, dim := range ladder {
		// Only the dimensions built from namePalette — the ones whose rungs *are*
		// names. A derived kind now also has an image dimension (HOLODEX-345), which
		// draws no rung from the text palette at all; listing its exclusions would
		// claim it lost rungs it never asked for, and would print each kind's
		// exclusions twice besides.
		if !dim.entity.derived() || !dim.ownsTitle {
			continue
		}
		for _, v := range textPalette {
			reason := nameRejects(dim.entity, v)
			if reason == "" {
				continue
			}
			if _, seen := byReason[reason]; !seen {
				order = append(order, reason)
			}
			byReason[reason] = append(byReason[reason], exclusion{dim.entity, v.key})
		}
	}
	if len(order) == 0 {
		return
	}

	fmt.Printf("\ntext rungs a derived entity's name cannot carry:\n")
	for _, reason := range order {
		names := make([]string, 0, len(byReason[reason]))
		for _, e := range byReason[reason] {
			names = append(names, fmt.Sprintf("%s/%s", e.kind, e.rung))
		}
		fmt.Printf("  %s\n    %s\n", strings.Join(names, " "), reason)
	}
}
