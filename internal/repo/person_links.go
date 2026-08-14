package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Person link derivation (F40, ADR-072). video_people is a derived index over the
// video's resolved person-typed fields — the person analogue of video_studios
// (studios.go), generalized as RelinkVideoEntity at the API layer. Unlike studio,
// a person carries authored identity (aliases, curated images, manual decisions),
// so a person left with zero links is orphan-stamped, never deleted immediately
// (ADR-072 §4) — the one policy difference from ReconcileVideoStudios.

// PersonRoleName is one desired (name, role) pairing for a video, produced by
// resolving the video's marked person-typed fields (registry.PersonTypedFields).
// role is the empty sentinel '' for a person-typed field with no declared role —
// never treat it as "absent"; it is a legitimate, stable value.
type PersonRoleName struct {
	Name string
	Role string
}

// personLinkKey identifies one (person, role) row in video_people.
type personLinkKey struct {
	personID int64
	role     string
}

// ReconcileVideoPeople makes video_people for one video hold exactly the given
// (name, role) pairs (ADR-072 RD2/RD3): resolves-or-creates each name (alias
// routing, homonym-safe — the same choke point studio and tag use), replaces the
// video's rows, and orphan-stamps (never deletes) any person left with zero links
// anywhere. One write transaction under writeMu; idempotent. Passing nil/empty
// links removes all of the video's people links (and orphan-stamps as needed) —
// the soft-delete path. extIDByName maps a resolved name -> its provider external id
// (F32, ADR-055), mirroring ReconcileVideoStudios' extIDByName so resolve-or-create
// can id-dedup a video's cast/crew credits in the SAME transaction as the link
// reconcile; a name absent from the map (or a nil map) resolves by name only. Pass
// nil when no ids are known.
func (r *Repo) ReconcileVideoPeople(ctx context.Context, videoID int64, links []PersonRoleName, extIDByName map[string]string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	return r.ReconcileVideoPeopleLocked(ctx, videoID, links, extIDByName)
}

// ReconcileVideoPeopleLocked is ReconcileVideoPeople's implementation for a caller
// that already holds writeMu — obtainable only from inside a SetCurationChecked
// check/commit callback (ADR-084), which is what lets the People curation fast path
// commit its relink write in the same locked critical section as the curation write
// itself instead of a separate, unlocked step after it (HOLODEX-277). Do not call
// this without holding writeMu: it performs the same full-replace write
// ReconcileVideoPeople does, with no locking of its own — same
// xLocked-plus-doc-comment contract as setCurationLocked/setDecisionLocked
// (curation.go/decisions.go).
func (r *Repo) ReconcileVideoPeopleLocked(ctx context.Context, videoID int64, links []PersonRoleName, extIDByName map[string]string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	foldedExtIDByName := foldedExtIDIndex(extIDByName)
	desired := make(map[personLinkKey]struct{}, len(links))
	for _, l := range links {
		name := strings.TrimSpace(l.Name)
		if name == "" {
			continue
		}
		pid, err := resolveOrCreatePerson(ctx, tx, name, extIDFor(extIDByName, foldedExtIDByName, name))
		if err != nil {
			return err
		}
		desired[personLinkKey{pid, l.Role}] = struct{}{}
	}

	current := map[personLinkKey]struct{}{}
	rows, err := tx.QueryContext(ctx, `SELECT person_id, role FROM video_people WHERE video_id = ?`, videoID)
	if err != nil {
		return fmt.Errorf("current people links: %w", err)
	}
	for rows.Next() {
		var k personLinkKey
		if err := rows.Scan(&k.personID, &k.role); err != nil {
			rows.Close()
			return err
		}
		current[k] = struct{}{}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	touched := map[int64]struct{}{}
	for k := range desired {
		touched[k.personID] = struct{}{}
		if _, have := current[k]; have {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO video_people (video_id, person_id, role) VALUES (?, ?, ?)`,
			videoID, k.personID, k.role); err != nil {
			return fmt.Errorf("link person: %w", err)
		}
	}
	for k := range current {
		touched[k.personID] = struct{}{}
		if _, keep := desired[k]; keep {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM video_people WHERE video_id = ? AND person_id = ? AND role = ?`,
			videoID, k.personID, k.role); err != nil {
			return fmt.Errorf("unlink person: %w", err)
		}
	}

	// Orphan-stamp/clear (ADR-072 §4): only the people touched by this reconcile
	// need re-checking — every other person's orphan status is unaffected.
	now := time.Now().UTC().Format(timeLayout)
	for pid := range touched {
		var linkCount int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM video_people WHERE person_id = ?`, pid).Scan(&linkCount); err != nil {
			return fmt.Errorf("count person links: %w", err)
		}
		if linkCount == 0 {
			if _, err := tx.ExecContext(ctx,
				`UPDATE people SET orphaned_at = ? WHERE id = ? AND orphaned_at IS NULL`, now, pid); err != nil {
				return fmt.Errorf("stamp orphan: %w", err)
			}
		} else if _, err := tx.ExecContext(ctx,
			`UPDATE people SET orphaned_at = NULL WHERE id = ? AND orphaned_at IS NOT NULL`, pid); err != nil {
			return fmt.Errorf("clear orphan: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit people links: %w", err)
	}
	return nil
}

// PersonLinkCount returns the total number of video_people rows — the fast-path
// gate for the one-time startup backfill (mirrors StudioLinkCount, ADR-072 P0-4).
func (r *Repo) PersonLinkCount(ctx context.Context) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM video_people`).Scan(&n)
	return n, err
}

// SweepOrphanedPeople deletes people orphaned more than graceDays ago that carry
// no authored identity (ADR-072 §4/P0-9): an alias (which also covers merge
// history — a merge always registers the loser's name as an alias of the
// survivor), a curated image, or any manual field decision/curation. Authored
// orphans are skipped and counted, never deleted. Idempotent; safe to run
// repeatedly. Returns (deleted, skipped) for the caller's System Activity record.
func (r *Repo) SweepOrphanedPeople(ctx context.Context, graceDays int) (deleted, skipped int, err error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	cutoff := time.Now().UTC().AddDate(0, 0, -graceDays).Format(timeLayout)
	rows, err := tx.QueryContext(ctx, `SELECT id FROM people WHERE orphaned_at IS NOT NULL AND orphaned_at < ?`, cutoff)
	if err != nil {
		return 0, 0, fmt.Errorf("find orphaned people: %w", err)
	}
	var candidates []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, 0, err
		}
		candidates = append(candidates, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	for _, id := range candidates {
		authored, err := personHasAuthoredIdentity(ctx, tx, id)
		if err != nil {
			return 0, 0, err
		}
		if authored {
			skipped++
			continue
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM people WHERE id = ?`, id); err != nil {
			return 0, 0, fmt.Errorf("delete orphaned person: %w", err)
		}
		deleted++
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit orphan sweep: %w", err)
	}
	return deleted, skipped, nil
}

// personHasAuthoredIdentity reports whether a person carries any owner-authored
// work that the orphan sweep must never destroy (ADR-072 §4/Q7): an alias
// (including one registered by a merge), a curated image, or any manual field
// decision or curation.
func personHasAuthoredIdentity(ctx context.Context, tx *sql.Tx, personID int64) (bool, error) {
	var exists int
	err := tx.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM entity_aliases WHERE entity_type = 'person' AND entity_id = ?)
		    OR EXISTS(SELECT 1 FROM person_images WHERE person_id = ?)
		    OR EXISTS(SELECT 1 FROM field_source_decisions WHERE entity_type = 'person' AND entity_id = ?)
		    OR EXISTS(SELECT 1 FROM metadata_curation WHERE entity_type = 'person' AND entity_id = ?)
	`, personID, personID, personID, personID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("authored identity check: %w", err)
	}
	return exists != 0, nil
}
