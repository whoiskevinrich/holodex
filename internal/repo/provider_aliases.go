package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"holodex/internal/model"
)

// SkippedAlias is a provider-supplied name that was not added because another entity of
// the same type already holds it (ADR-088 D5). The pair is queued for the owner in
// identity_review_queue; this value is what the detail read surfaces so the Aliases panel
// can say which name was dropped and why.
type SkippedAlias struct {
	Alias      string `json:"alias"`
	ConflictID int64  `json:"conflict_id"`
}

// aliasFold implements spec F58 RD6's near-duplicate test: lowercase and drop every
// non-alphanumeric rune, so a candidate differing from the canonical name only by
// punctuation or spacing folds onto it.
//
// This is an import-time filter and nothing else. It is deliberately NOT
// entity_aliases.alias_key's fold (a stored generated column, lower+trim, plus
// space-stripping for tags) and must never be used where alias_key is — conflating the
// two would change what collides, reopening an F43/ADR-061 decision this feature does
// not touch. Per-entity scoping is the reason: "Mary Jane" and "MaryJane" are
// deliberately distinct people, and only this filter's narrow "is it the same name we
// already have" question may treat them alike.
//
// Narrow by design. Against a canonical "Hayao Miyazaki" it drops "Hayao-Miyazaki" but
// keeps "H. Miyazaki", "Miyazaki, Hayao", and "宮崎駿" — an over-eager filter silently
// costs the entity reach, which is worse than importing one redundant name.
func aliasFold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// ApplyProviderAliases writes a provider's alternate names into the identity spine
// (HOLODEX-306, spec F58 P0-2/P0-5, ADR-088 D3/D5). Rows carry `source` so the UI can
// badge them; nothing else reads it, and in particular the scan resolve predicate stays
// source-blind — a provider alias routes files exactly as an owner-typed one does, which
// is the whole point of the collapse.
//
// A candidate is skipped when it is empty or over the name cap, when it folds onto the
// entity's own canonical name (RD6), when the owner has suppressed it for this entity
// (D4 — this is what makes a delete durable across re-enrich), or when another entity of
// the same type already holds the name. Only that last case is reported back: the pair
// goes into identity_review_queue as 'provider-alias' for the owner to resolve, honoring
// entity_keep_separate so a pair already dismissed is never re-proposed (F43 RD5).
//
// Purely additive (RD5): a name the provider has since dropped is left alone. Only the
// owner and a merge ever remove an alias.
//
// Returning an error here fails the caller's enrichment, so the enrich path treats this
// as best-effort — a name conflict must never cost an entity its bio, birthdate, and
// photo. One transaction under the write lock.
func (r *Repo) ApplyProviderAliases(ctx context.Context, entityType string, entityID int64, source string, names []string) ([]SkippedAlias, error) {
	table := canonicalTable(entityType)
	if table == "" {
		return nil, fmt.Errorf("apply provider aliases: unknown entity type %q", entityType)
	}
	if source == "" {
		return nil, errors.New("apply provider aliases: empty source")
	}
	if len(names) == 0 {
		return nil, nil
	}

	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("apply provider aliases: %w", err)
	}
	defer tx.Rollback()

	var canonical string
	switch err := tx.QueryRowContext(ctx,
		`SELECT name FROM `+table+` WHERE id = ?`, entityID).Scan(&canonical); {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrNotFound
	case err != nil:
		return nil, fmt.Errorf("apply provider aliases (%s): %w", entityType, err)
	}
	canonicalFold := aliasFold(canonical)

	var skipped []SkippedAlias
	// A provider can list the same name twice in different spellings; fold-dedupe so the
	// batch does one insert attempt per distinct name and cannot queue the same review
	// pair from two candidates.
	seen := make(map[string]bool, len(names))

	for _, raw := range names {
		alias := strings.TrimSpace(raw)
		if alias == "" || len([]rune(alias)) > model.MaxNameLen {
			continue
		}
		fold := aliasFold(alias)
		// An empty fold means punctuation-only. Equal to the canonical fold means RD6's
		// near-duplicate, which subsumes the "provider echoed our own name back" case
		// since aliasFold is strictly coarser than nameKey.
		if fold == "" || fold == canonicalFold || seen[fold] {
			continue
		}
		seen[fold] = true

		suppressed, err := aliasSuppressed(ctx, tx, entityType, entityID, alias)
		if err != nil {
			return nil, err
		}
		if suppressed {
			continue
		}

		otherID, conflict, err := entityConflict(ctx, tx, entityType, entityID, alias)
		if err != nil {
			return nil, err
		}
		if conflict {
			if err := queueProviderAliasPair(ctx, tx, entityType, entityID, otherID); err != nil {
				return nil, err
			}
			skipped = append(skipped, SkippedAlias{Alias: alias, ConflictID: otherID})
			continue
		}

		// OR IGNORE covers the ordinary re-enrich case: this entity already has the name,
		// so the write is a no-op and the row keeps its original id.
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias, source) VALUES (?, ?, ?, ?)`,
			entityType, entityID, alias, source); err != nil {
			return nil, fmt.Errorf("insert provider alias (%s): %w", entityType, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("apply provider aliases: %w", err)
	}
	return skipped, nil
}

// aliasSuppressed reports whether the owner has deleted this name from this entity
// before (ADR-088 D4). Keyed per-entity, so suppressing a name on one person leaves
// every other entity free to receive or add it.
func aliasSuppressed(ctx context.Context, tx *sql.Tx, entityType string, entityID int64, alias string) (bool, error) {
	var n int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entity_alias_suppressions
		  WHERE entity_type = ? AND entity_id = ? AND alias_key = `+nameKeyExpr(entityType, "?"),
		entityType, entityID, alias).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("alias suppression lookup (%s): %w", entityType, err)
	}
	return n > 0, nil
}

// queueProviderAliasPair records a skipped-collision pair for the owner. Ordered id_lo /
// id_hi so the same pair reached from either direction is one row, and gated on
// entity_keep_separate so a pair the owner has already dismissed is never re-proposed
// (F43 RD5 — a kept-separate pair never nags, and a re-enrich would otherwise nag on
// every run).
func queueProviderAliasPair(ctx context.Context, tx *sql.Tx, entityType string, a, b int64) error {
	lo, hi := orderPair(a, b)
	_, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO identity_review_queue (entity_type, id_lo, id_hi, variation)
		SELECT ?, ?, ?, 'provider-alias'
		WHERE NOT EXISTS (
			SELECT 1 FROM entity_keep_separate ks
			 WHERE ks.entity_type = ? AND ks.id_lo = ? AND ks.id_hi = ?)`,
		entityType, lo, hi, entityType, lo, hi)
	if err != nil {
		return fmt.Errorf("queue provider-alias pair (%s): %w", entityType, err)
	}
	return nil
}
