// Command aliasseed stages the alias states that are tedious to reach by hand, so the
// F58/ADR-088 surfaces (provider badge, collision review line, suppression durability)
// can be looked at in a real browser without re-deriving the setup each time.
//
//	go run ./testdata/aliasseed                 # seed into ./data/holodex.db
//	go run ./testdata/aliasseed -video 91       # ...and link the people to a video
//	go run ./testdata/aliasseed -clean          # remove everything it created
//
// Stop the backend first: the server holds its own connection, and two writers on one
// SQLite file will contend past the busy timeout.
//
// Every alias mutation goes through the real repo write path (ApplyProviderAliases,
// AddEntityAlias, DeleteEntityAlias) rather than hand-written SQL. That is the whole
// point: a seed built from INSERT statements encodes what someone *believed* those states
// look like, and drifts silently the moment the guards change. Going through the writer
// means the fixture is, by construction, exactly what production produces — including the
// RD6 drop and the collision skip, which are decisions the writer makes, not rows.
//
// Entity creation itself is direct SQL: which rows exist is uninteresting scaffolding,
// unlike the alias state layered on top. Seeded people carry aliases, which counts as
// "authored identity" (ADR-072 §4), so the orphan sweep skips them even with no video
// link.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"holodex/internal/db"
	"holodex/internal/model"
	"holodex/internal/repo"
)

// The provider namespace seeded rows are attributed to. Matches the usual local sidecar
// so the chip badge reads the same as it would after a genuine enrich.
const seedProvider = "tmdb"

// Seeded entity names. Deliberately plausible-but-fictional: obviously-fake strings like
// "Owner Typed Name" get mistaken for a rendering bug, while a name indistinguishable
// from real data gets mistaken for real data. -clean matches on exactly these.
const (
	personHolder   = "Ishiro Honda"    // holds the name person 2's provider will want
	personSubject  = "Inoshiro Honda"  // the entity under test: gets every alias state
	studioSubject  = "Toho Fixture Co" // proves the panel/payload are entity-generic (RD8)
	ownerAlias     = "Honda-san"       // typed by the "owner" — renders with no badge
	suppressedName = "Honda Inoshiro"  // added by the provider, then deleted → suppressed
)

func main() {
	dbPath := flag.String("db", "./data/holodex.db", "path to the Holodex SQLite database")
	videoID := flag.Int64("video", 0, "optional video id to link the seeded people to")
	clean := flag.Bool("clean", false, "remove everything this tool creates, then exit")
	flag.Parse()

	if err := run(*dbPath, *videoID, *clean); err != nil {
		log.Fatalf("aliasseed: %v", err)
	}
}

func run(dbPath string, videoID int64, clean bool) error {
	if _, err := os.Stat(dbPath); err != nil {
		return fmt.Errorf("database %q: %w (run the server once to create it)", dbPath, err)
	}
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer database.Close()
	r := repo.New(database)
	ctx := context.Background()

	if clean {
		return cleanup(ctx, database)
	}
	return seed(ctx, r, database, videoID)
}

func seed(ctx context.Context, r *repo.Repo, database *sql.DB, videoID int64) error {
	holder, err := ensurePerson(ctx, database, personHolder)
	if err != nil {
		return err
	}
	subject, err := ensurePerson(ctx, database, personSubject)
	if err != nil {
		return err
	}
	studio, err := ensureStudio(ctx, database, studioSubject)
	if err != nil {
		return err
	}

	// State 1 — an owner-typed alias. Renders with no badge, and is the name the
	// provider will later collide with.
	if _, err := r.AddEntityAlias(ctx, model.EnrichEntityPerson, holder, ownerAlias); err != nil {
		return fmt.Errorf("owner alias: %w", err)
	}

	// States 2-4 in one call, because the writer decides all three:
	//   本多猪四郎        → kept, badged (provider-sourced)
	//   Inoshiro-Honda   → dropped, RD6 near-duplicate of the canonical name
	//   Honda-san        → skipped + queued, the name `holder` already owns
	//   Honda Inoshiro   → kept for now; deleted below to leave a suppression
	skipped, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, subject, seedProvider,
		[]string{"本多猪四郎", "Inoshiro-Honda", ownerAlias, suppressedName})
	if err != nil {
		return fmt.Errorf("provider aliases: %w", err)
	}

	// State 5 — a durable suppression. Deleting a provider-sourced alias records one, so
	// the re-apply below must NOT bring the name back. Re-applying here rather than
	// trusting the write means the seeded state is verified, not asserted.
	if err := deleteAliasByName(ctx, r, database, model.EnrichEntityPerson, subject, suppressedName); err != nil {
		return err
	}
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityPerson, subject, seedProvider,
		[]string{suppressedName}); err != nil {
		return fmt.Errorf("re-apply: %w", err)
	}

	// Entity-generic (RD8): the same panel and payload on studio.
	if _, err := r.ApplyProviderAliases(ctx, model.EnrichEntityStudio, studio, seedProvider,
		[]string{"TOHO Company Limited"}); err != nil {
		return fmt.Errorf("studio aliases: %w", err)
	}

	if videoID != 0 {
		if err := linkToVideo(ctx, database, videoID, holder, subject); err != nil {
			return err
		}
	}

	return report(ctx, r, holder, subject, studio, skipped, videoID)
}

// report prints the seeded state read back through the same reads the API uses, so a
// silent no-op is visible rather than assumed.
func report(ctx context.Context, r *repo.Repo, holder, subject, studio int64, skipped []repo.SkippedAlias, videoID int64) error {
	for _, e := range []struct {
		label      string
		entityType string
		id         int64
	}{
		{personHolder, model.EnrichEntityPerson, holder},
		{personSubject, model.EnrichEntityPerson, subject},
		{studioSubject, model.EnrichEntityStudio, studio},
	} {
		aliases, err := r.AliasesForEntity(ctx, e.entityType, e.id)
		if err != nil {
			return err
		}
		fmt.Printf("%s (%s %d)\n", e.label, e.entityType, e.id)
		for _, a := range aliases {
			src := "owner-typed"
			if a.Source != "" {
				src = "from " + a.Source
			}
			fmt.Printf("    %-24s %s\n", a.Alias, src)
		}
		sk, err := r.SkippedAliasesForEntity(ctx, e.entityType, e.id)
		if err != nil {
			return err
		}
		for _, s := range sk {
			fmt.Printf("    %-24s skipped — held by entity %d\n", s.Alias, s.ConflictID)
		}
	}
	fmt.Printf("\n%d name(s) skipped on this run; %q suppressed and confirmed not resurrected.\n",
		len(skipped), suppressedName)
	if videoID == 0 {
		fmt.Println("No video link (-video N to add one). The people carry aliases, so the")
		fmt.Println("orphan sweep treats them as authored and will not prune them.")
	} else {
		fmt.Printf("Linked both people to video %d as 'actor'.\n", videoID)
	}
	fmt.Println("Re-run with -clean to remove everything above.")
	return nil
}

func ensurePerson(ctx context.Context, database *sql.DB, name string) (int64, error) {
	return ensureRow(ctx, database, "people", name)
}

func ensureStudio(ctx context.Context, database *sql.DB, name string) (int64, error) {
	return ensureRow(ctx, database, "studios", name)
}

// ensureRow inserts a named entity if absent and returns its id, so the tool is
// idempotent. table is a literal from the two callers above, never user input.
func ensureRow(ctx context.Context, database *sql.DB, table, name string) (int64, error) {
	if _, err := database.ExecContext(ctx,
		`INSERT OR IGNORE INTO `+table+` (name) VALUES (?)`, name); err != nil {
		return 0, fmt.Errorf("create %s %q: %w", table, name, err)
	}
	var id int64
	if err := database.QueryRowContext(ctx,
		`SELECT id FROM `+table+` WHERE name = ?`, name).Scan(&id); err != nil {
		return 0, fmt.Errorf("resolve %s %q: %w", table, name, err)
	}
	return id, nil
}

// deleteAliasByName routes through DeleteEntityAlias so the suppression side effect fires
// — deleting the row directly would leave the fixture missing the state it exists to show.
func deleteAliasByName(ctx context.Context, r *repo.Repo, database *sql.DB, entityType string, id int64, alias string) error {
	var aliasID int64
	err := database.QueryRowContext(ctx,
		`SELECT id FROM entity_aliases WHERE entity_type = ? AND entity_id = ? AND alias = ?`,
		entityType, id, alias).Scan(&aliasID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil // already deleted on a previous run; the suppression is what matters
	}
	if err != nil {
		return fmt.Errorf("find alias %q: %w", alias, err)
	}
	if err := r.DeleteEntityAlias(ctx, entityType, id, aliasID); err != nil {
		return fmt.Errorf("delete alias %q: %w", alias, err)
	}
	return nil
}

func linkToVideo(ctx context.Context, database *sql.DB, videoID int64, people ...int64) error {
	var exists int
	if err := database.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM videos WHERE id = ?`, videoID).Scan(&exists); err != nil {
		return fmt.Errorf("check video %d: %w", videoID, err)
	}
	if exists == 0 {
		return fmt.Errorf("video %d not found", videoID)
	}
	for _, p := range people {
		if _, err := database.ExecContext(ctx,
			`INSERT OR IGNORE INTO video_people (video_id, person_id, role) VALUES (?, ?, 'actor')`,
			videoID, p); err != nil {
			return fmt.Errorf("link person %d to video %d: %w", p, videoID, err)
		}
	}
	return nil
}

// cleanup removes the seeded entities. Deleting the entity row is enough for aliases and
// suppressions — migration 0022's and 0044's AFTER DELETE triggers take those, and the
// video_people rows go by ON DELETE CASCADE. identity_review_queue has no such trigger
// (its rows self-heal only in the read, which INNER JOINs), so those are removed by hand.
func cleanup(ctx context.Context, database *sql.DB) error {
	var removed int64
	for _, e := range []struct{ table, name string }{
		{"people", personHolder},
		{"people", personSubject},
		{"studios", studioSubject},
	} {
		var id int64
		err := database.QueryRowContext(ctx,
			`SELECT id FROM `+e.table+` WHERE name = ?`, e.name).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("find %s %q: %w", e.table, e.name, err)
		}
		entityType := model.EnrichEntityPerson
		if e.table == "studios" {
			entityType = model.EnrichEntityStudio
		}
		if _, err := database.ExecContext(ctx,
			`DELETE FROM identity_review_queue WHERE entity_type = ? AND (id_lo = ? OR id_hi = ?)`,
			entityType, id, id); err != nil {
			return fmt.Errorf("clear review queue for %s %d: %w", entityType, id, err)
		}
		res, err := database.ExecContext(ctx, `DELETE FROM `+e.table+` WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("delete %s %q: %w", e.table, e.name, err)
		}
		n, _ := res.RowsAffected()
		removed += n
		fmt.Printf("removed %s %q (id %d)\n", e.table, e.name, id)
	}
	if removed == 0 {
		fmt.Println("nothing to clean up.")
	}
	return nil
}
