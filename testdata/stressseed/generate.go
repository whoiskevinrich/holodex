package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"holodex/internal/model"
	"holodex/internal/repo"
)

// seededTables are the tables the fixture owns outright. Every run clears them
// before generating, which is what makes the fixture reproducible rather than
// merely idempotent: an upsert-in-place run would leave rows from a rung that
// has since been deleted, and an interrupted run would leave half a fixture
// behind with no way to tell. Clearing is safe here and nowhere else — the
// caller has already proved this database is the fixture's own (see claim.go).
//
// Ordered parents-last: videos first sheds most of the link rows by cascade.
var seededTables = []string{"videos", "films", "people", "studios", "tags"}

// generate builds every dimension in the ladder and returns what it addressed.
//
// The walk is one-factor-at-a-time (D3): each rung starts from the neutral
// baseline, mutates exactly one axis, and becomes exactly one entity. There is
// deliberately no nesting here — a cross-product would be a loop inside this
// loop, and its absence is the design.
func generate(ctx context.Context, database *sql.DB, r *repo.Repo, pf personField) ([]entry, error) {
	if err := validateLadder(ladder); err != nil {
		return nil, err
	}
	if err := reset(ctx, database); err != nil {
		return nil, err
	}

	// Supporting entities are steered once, into the pool range above every
	// dimension block, so a person created to satisfy a cardinality rung can
	// never land on an address an assertion was written against.
	for _, table := range []string{"people", "studios", "tags"} {
		if err := steer(ctx, database, table, poolBase); err != nil {
			return nil, err
		}
	}

	var entries []entry
	for _, dim := range ladder {
		if err := steer(ctx, database, dim.entity.table(), dim.block); err != nil {
			return nil, err
		}
		for _, rg := range dim.rungs {
			s := baseline()
			rg.apply(&s)

			id, err := materialize(ctx, r, pf, dim, rg, s)
			if err != nil {
				return nil, fmt.Errorf("%s=%s: %w", dim.key, rg.variant, err)
			}
			// The block is only a real address if nothing escapes it. Steering
			// puts the first entity in the right place; this catches the case
			// where a dimension has outgrown the block it declared, which would
			// otherwise silently overwrite the next dimension's addresses.
			if id < dim.block || id >= dim.block+blockSize {
				return nil, fmt.Errorf(
					"dimension %q overflowed its reserved block: %s landed at id %d, outside [%d,%d).\n"+
						"Give it a larger block or move the dimensions above it",
					dim.key, rg.variant, id, dim.block, dim.block+blockSize)
			}

			entries = append(entries, entry{
				ID:        id,
				Entity:    dim.entity,
				Dimension: dim.key,
				Variant:   rg.variant,
				Value:     rg.value,
				Name:      encodeName(s),
				URL:       dim.entity.urlFor(id),
				Axes:      axesOf(s),
			})
		}
	}
	return entries, nil
}

// validateLadder enforces the invariants the table's readers assume. They are
// checked rather than trusted because the failure mode is silent: two dimensions
// sharing a block do not error, they overwrite each other's addresses.
func validateLadder(dims []dimension) error {
	blocks := map[int64]string{}
	highest := map[entityKind]int64{}
	for _, dim := range dims {
		if dim.entity.table() == "" || dim.entity.urlFor(1) == "" {
			return fmt.Errorf("dimension %q addresses entity kind %q, which has no table or route mapping",
				dim.key, dim.entity)
		}
		if len(dim.rungs) == 0 {
			return fmt.Errorf("dimension %q declares no rungs, so it would reserve a block "+
				"and contribute nothing to the fixture", dim.key)
		}
		if len(dim.rungs) > blockSize {
			return fmt.Errorf("dimension %q declares %d rungs but a block holds %d",
				dim.key, len(dim.rungs), blockSize)
		}
		if dim.block <= 0 || dim.block+blockSize > poolBase {
			return fmt.Errorf("dimension %q block %d is outside the addressable range [1,%d)",
				dim.key, dim.block, poolBase)
		}
		if other, taken := blocks[dim.block]; taken {
			return fmt.Errorf("dimensions %q and %q both claim block %d", other, dim.key, dim.block)
		}
		blocks[dim.block] = dim.key

		// Table order is load-bearing for dimensions sharing an entity kind:
		// generate() steers one AUTOINCREMENT counter per kind, in table order,
		// and steering it backwards over rows that already exist places the next
		// entity somewhere neither the block nor the overflow check predicts.
		if prev, seen := highest[dim.entity]; seen && dim.block <= prev {
			return fmt.Errorf("dimension %q declares block %d after a %s dimension at %d; "+
				"blocks must ascend within an entity kind, because they share one ID sequence",
				dim.key, dim.block, dim.entity, prev)
		}
		highest[dim.entity] = dim.block

		seen := map[string]bool{}
		for _, rg := range dim.rungs {
			if seen[rg.variant] {
				return fmt.Errorf("dimension %q repeats the variant %q", dim.key, rg.variant)
			}
			seen[rg.variant] = true
		}
	}
	return nil
}

// materialize turns one spec into one video and its links, and returns the id it
// was addressed at.
func materialize(ctx context.Context, r *repo.Repo, pf personField, dim dimension, rg rung, s spec) (int64, error) {
	cast := make([]string, 0, s.people)
	for i := range s.people {
		cast = append(cast, poolName("person", i))
	}
	tags, err := pf.castTags(cast)
	if err != nil {
		return 0, err
	}

	now := time.Now().UTC()
	id, err := r.UpsertVideo(ctx, &model.Video{
		// Stable and unique per rung. It never points at a real file — the
		// seeder bypasses the scanner by design (D1) — but it is what UpsertVideo
		// identifies a row by, so it has to be derived from the address.
		FilePath:  fmt.Sprintf("/stress/%s/%s.mp4", dim.key, rg.variant),
		Title:     title(dim, s),
		FileSize:  1 << 20,
		Duration:  600,
		Width:     1920,
		Height:    1080,
		Container: "mp4",
		IndexedAt: now,
		FileMtime: now,
		// The cast goes in as file-layer tags, which is what makes it survive:
		// the server re-derives video_people from these on startup. See
		// filelayer.go for what happened when it did not.
	}, tags)
	if err != nil {
		return 0, fmt.Errorf("upsert video: %w", err)
	}

	// Reconciling here too, rather than leaving the links for the server's
	// startup backfill, so the fixture is complete the moment seeding finishes
	// instead of only after it has been served once. This is not a second source
	// of truth: it is the same names the derivation will resolve from the tags
	// just written, so the backfill re-derives an identical set and changes
	// nothing. One call, not one per person — the API is a full replace, so a
	// loop would leave only the last link standing.
	people := make([]repo.PersonRoleName, 0, len(cast))
	for _, name := range cast {
		people = append(people, repo.PersonRoleName{Name: name, Role: pf.role})
	}
	if err := r.ReconcileVideoPeople(ctx, id, people, nil); err != nil {
		return 0, fmt.Errorf("link %d people: %w", s.people, err)
	}

	studios := make([]string, 0, s.studios)
	for i := range s.studios {
		studios = append(studios, poolName("studio", i))
	}
	if err := r.ReconcileVideoStudios(ctx, id, studios, nil); err != nil {
		return 0, fmt.Errorf("link %d studios: %w", s.studios, err)
	}

	for i := range s.tags {
		if _, err := r.AttachTagToVideo(ctx, id, poolName("tag", i)); err != nil {
			return 0, fmt.Errorf("attach tag %d: %w", i, err)
		}
	}
	return id, nil
}

// title is what the entity carries in the UI: its encoded coordinate, unless the
// dimension has declared that it owns the title outright. See dimension.ownsTitle
// for why that exception has to exist.
func title(dim dimension, s spec) string {
	if dim.ownsTitle {
		return s.text.value
	}
	return encodeName(s)
}

// poolName names a supporting entity. Zero-padded and lowercase: padding keeps
// them ordered in a list, and lowercase matches what the tag writer stores
// anyway, so a seeded name and a read-back name are the same string.
func poolName(kind string, i int) string {
	return fmt.Sprintf("stress %s %03d", kind, i+1)
}

// reset clears the fixture's own rows and rewinds the ID sequences behind them,
// so a run always builds from empty and always lands on the same addresses.
func reset(ctx context.Context, database *sql.DB) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reset: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op once committed

	for _, table := range seededTables {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table); err != nil {
			return fmt.Errorf("clear %s: %w", table, err)
		}
	}
	// Deliberately no sequence rewind here: every sequence generate() depends on
	// is steered explicitly before it is used, so rewinding would be redundant
	// rather than defensive. Removed after a mutation test showed it made no
	// difference to reproducibility — steering is what holds the addresses still.
	return tx.Commit()
}

// steer points a table's AUTOINCREMENT counter at `next`, so the next row
// inserted through the ordinary repo API lands there.
//
// This is how reserved blocks (D4) are honoured without a caller-chosen-ID
// escape hatch: no repo method accepts one, and adding raw INSERTs to get them
// would mean the fixture stopped exercising the write path the app itself uses —
// tag folding, association rules, FTS triggers and all. Steering the counter
// keeps the real API and still lands the row on a chosen address.
//
// sqlite_sequence has no unique index, so this is delete-then-insert rather than
// an upsert.
func steer(ctx context.Context, database *sql.DB, table string, next int64) error {
	if next < 1 {
		return fmt.Errorf("steer %s: next id %d must be positive", table, next)
	}
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin steer %s: %w", table, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM sqlite_sequence WHERE name = ?`, table); err != nil {
		return fmt.Errorf("clear sequence for %s: %w", table, err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sqlite_sequence (name, seq) VALUES (?, ?)`, table, next-1); err != nil {
		return fmt.Errorf("set sequence for %s: %w", table, err)
	}
	return tx.Commit()
}
