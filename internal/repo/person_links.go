package repo

import (
	"context"
	"fmt"
	"strings"
)

// Person link derivation (F40, ADR-072). video_people is a derived index over the
// video's resolved person-typed fields — the person analogue of video_studios
// (studios.go), generalized as RelinkVideoEntity at the API layer. A person left
// with zero links is orphan-stamped, never deleted immediately — the same rule as
// studios and tags (orphans.go, HOLODEX-535).

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
// routing, homonym-safe — the same choke point studio and tag use) and replaces the
// video's rows; a person left with zero links anywhere is orphan-stamped by trigger,
// never deleted. One write transaction under writeMu; idempotent. Passing nil/empty
// links removes all of the video's people links — the soft-delete path. extIDByName maps a resolved name -> its provider external id
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

	for k := range desired {
		if _, have := current[k]; have {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO video_people (video_id, person_id, role) VALUES (?, ?, ?)`,
			videoID, k.personID, k.role); err != nil {
			return fmt.Errorf("link person: %w", err)
		}
	}
	// A person left with no links is orphan-stamped (and cleared when one
	// returns) by the video_people_*_orphan triggers (migration 0059).
	for k := range current {
		if _, keep := desired[k]; keep {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM video_people WHERE video_id = ? AND person_id = ? AND role = ?`,
			videoID, k.personID, k.role); err != nil {
			return fmt.Errorf("unlink person: %w", err)
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
