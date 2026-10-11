package repo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"holodex/internal/model"
)

// dashedTagAlias is a multi-word tag name spelled with each run of whitespace as one
// dash (F43 RD13: "science fiction" → "science-fiction"), or "" for a single-word
// name, which has no dashed spelling. A tag's nameKey folds whitespace but not
// punctuation, so the dashed spelling is a different name that would otherwise create
// a second tag on scan.
func dashedTagAlias(name string) string {
	words := strings.Fields(name)
	if len(words) < 2 {
		return ""
	}
	return strings.Join(words, "-")
}

// addDashedTagAliasTx gives tag id its dashed alias inside the caller's transaction
// (F43 P0-12). It never merges: a dashed spelling that already resolves to another tag
// is skipped, leaving that pair to the near-miss queue (RD13). Owner-authored source
// (empty), so the alias reads as an ordinary alias in the panel. Reports whether a row
// was written.
func addDashedTagAliasTx(ctx context.Context, tx *sql.Tx, id int64, name string) (bool, error) {
	alias := dashedTagAlias(name)
	if alias == "" {
		return false, nil
	}
	if _, conflict, err := entityConflict(ctx, tx, model.EntityTag, id, alias); err != nil {
		return false, err
	} else if conflict {
		return false, nil
	}
	res, err := tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias) VALUES (?, ?, ?)`,
		model.EntityTag, id, alias)
	if err != nil {
		return false, fmt.Errorf("insert dashed tag alias: %w", err)
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// BackfillDashedTagAliases gives every existing multi-word tag its dashed alias
// (F43 P0-12), skipping any the owner has removed (a suppression, recorded by
// DeleteEntityAlias) and any that resolve to another tag. Idempotent — a re-run finds
// every alias already present — because job_runs retention can prune the boot gate.
// Returns the number of aliases added.
func (r *Repo) BackfillDashedTagAliases(ctx context.Context) (int64, error) {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("dashed tag alias backfill: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	type tagRow struct {
		id          int64
		name, alias string
	}
	rows, err := tx.QueryContext(ctx, `SELECT id, name FROM tags`)
	if err != nil {
		return 0, fmt.Errorf("dashed tag alias backfill: list tags: %w", err)
	}
	var tags []tagRow
	for rows.Next() {
		var t tagRow
		if err := rows.Scan(&t.id, &t.name); err != nil {
			rows.Close()
			return 0, fmt.Errorf("dashed tag alias backfill: scan tag: %w", err)
		}
		if t.alias = dashedTagAlias(t.name); t.alias != "" {
			tags = append(tags, t)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("dashed tag alias backfill: list tags: %w", err)
	}

	var added int64
	for _, t := range tags {
		suppressed, err := aliasSuppressed(ctx, tx, model.EntityTag, t.id, t.alias)
		if err != nil {
			return 0, err
		}
		if suppressed {
			continue
		}
		ok, err := addDashedTagAliasTx(ctx, tx, t.id, t.name)
		if err != nil {
			return 0, err
		}
		if ok {
			added++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("dashed tag alias backfill: %w", err)
	}
	return added, nil
}
