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
// why the two have to agree. It defaults to METADATA_MAPPINGS_PATH.
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
	count := flag.Int("count", defaultCount, "how many media entities the collection carries")
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

	if err := run(*dataPath, *count, *seed, *mappings); err != nil {
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

// defaultMappingsPath honours METADATA_MAPPINGS_PATH so the seeder reads the same
// mapping the `backend-stress` profile hands the server. That matters because the
// mapping decides which file tag carries the cast, and a fixture built against a
// different mapping than the one serving it would have its links re-derived away
// (filelayer.go). Unlike DATA_PATH this is a read-only input and cannot redirect
// a write, so taking it from the environment does not weaken the D5 isolation.
func defaultMappingsPath() string {
	if p := os.Getenv("METADATA_MAPPINGS_PATH"); p != "" {
		return p
	}
	return config.Defaults().MetadataMappingsPath
}

func run(dataPath string, count int, seed uint64, mappingsPath string) error {
	if count < 0 {
		return fmt.Errorf("-count %d: must not be negative", count)
	}

	cfg := config.Defaults()
	cfg.ApplyOverrides(config.Overrides{DataPath: dataPath})

	// Before touching anything: a mapping that cannot carry a cast means the
	// people ladder would be erased on the next boot, so fail now rather than
	// after writing a fixture that destroys itself.
	pf, err := loadPersonField(mappingsPath)
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

	entries, err := generate(ctx, database, repo.New(database), pf)
	if err != nil {
		return err
	}
	manifestPath, err := writeManifest(cfg.DataPath, buildManifest(entries, seed, count))
	if err != nil {
		return err
	}

	report(cfg, mediaPath, manifestPath, pf, entries, count, seed)
	return nil
}

// report prints where the fixture landed and what it addressed, so an operator
// can see every path the tool considers its own before trusting the isolation
// claim — and can find a dimension's block without opening the manifest.
func report(cfg config.Config, mediaPath, manifestPath string, pf personField, entries []entry, count int, seed uint64) {
	fmt.Printf("fixture claimed at %s (seed %d, count %d)\n\n", cfg.DataPath, seed, count)
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

	// Naming the tag is worth a line: it is the one input that has to match the
	// server's, and a mismatch shows up as a silently empty cast rather than an
	// error (filelayer.go).
	fmt.Printf("\ncast written as file tag %q, which this mapping resolves %q from —\n"+
		"so the server's startup relink re-derives these links instead of wiping them\n",
		pf.fileKey, pf.canonical)

	fmt.Printf("\n%d entities across %d dimensions:\n", len(entries), len(ladder))
	for _, dim := range ladder {
		variants := make([]string, 0, len(dim.rungs))
		for _, rg := range dim.rungs {
			variants = append(variants, rg.variant)
		}
		fmt.Printf("  %-8s %d-%d  %s\n", dim.key, dim.block,
			dim.block+int64(len(dim.rungs))-1, strings.Join(variants, " "))
	}
	fmt.Printf("\nServe it with the `backend-stress` launch profile; tear it down with rm -rf %s\n", cfg.DataPath)
}
