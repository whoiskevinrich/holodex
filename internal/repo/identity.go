package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"holodex/internal/model"
)

// Shared name-identity spine (F43, ADR-061). One normalized nameKey per entity type,
// unique across canonical names AND aliases, consumed identically by person, studio,
// and tag. resolveOrCreateByName is the single resolve-or-create merge point; the
// per-entity wrappers (resolveOrCreatePerson/resolveOrCreateStudio and the tag call
// site) route through it. All normalization is computed in SQL — never in Go — so the
// alias_key generated column, the canonical unique index, and every resolve/collision
// predicate use one byte-identical key (SQLite lower()/trim() is ASCII-only; matching
// it in Go would drift on non-ASCII names).

// canonicalTable maps a name-identity entity type to its canonical table. entityType
// is a trusted internal literal (never user input), so composing it into SQL is safe.
func canonicalTable(entityType string) string {
	switch entityType {
	case model.EnrichEntityPerson:
		return "people"
	case model.EnrichEntityStudio:
		return "studios"
	case model.EntityTag:
		return "tags"
	default:
		return ""
	}
}

// nameKeyExpr returns the SQLite expression that normalizes `col` to the entity's
// identity key (ADR-061 D2 / RD2): person & studio fold case + edge whitespace; tag
// also folds internal whitespace (`"sci fi"` → `"scifi"`). Diacritics are deliberately
// not folded here (that is a search concern, not identity). `col` is a trusted literal
// — a column name or "?". It is the single source of truth for the identity key, used
// to build the resolve queries (below) and the person alias predicate (aliases.go).
func nameKeyExpr(entityType, col string) string {
	if entityType == model.EntityTag {
		return fmt.Sprintf("replace(lower(trim(%s)), ' ', '')", col)
	}
	return fmt.Sprintf("lower(trim(%s))", col)
}

// identityQueries holds the per-entity resolve SQL. Built once at init from
// canonicalTable + nameKeyExpr, so the scan hot path (resolveOrCreateByName, called
// per person/tag/studio per video) does zero per-call string formatting.
type identityQueries struct{ canonicalSelect, aliasSelect, insert string }

var identityQueryByType = func() map[string]identityQueries {
	m := make(map[string]identityQueries, 3)
	for _, et := range []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EntityTag} {
		table := canonicalTable(et)
		m[et] = identityQueries{
			canonicalSelect: fmt.Sprintf(`SELECT id FROM %s WHERE %s = %s`, table, nameKeyExpr(et, "name"), nameKeyExpr(et, "?")),
			aliasSelect:     fmt.Sprintf(`SELECT entity_id FROM entity_aliases WHERE entity_type = ? AND alias_key = %s LIMIT 1`, nameKeyExpr(et, "?")),
			insert:          fmt.Sprintf(`INSERT INTO %s (name) VALUES (?)`, table),
		}
	}
	return m
}()

// resolveOrCreateByName resolves a name to an entity id for person / studio / tag,
// creating the entity if absent (F43, ADR-061). Resolution order (RD3): external-id
// (studios only today) → canonical nameKey → alias key → create. Case/whitespace
// variants converge on the one canonical entity (the "fox"/"Fox" fix); a merged-away
// name routes through the alias table so a merge survives a re-scan / link
// re-derivation. Runs inside the caller's transaction; writeMu serialization + the
// canonical nameKey unique index make the select-then-insert race-free. externalID is
// honored for studios only (ADR-054/055); empty for name-only entities.
func resolveOrCreateByName(ctx context.Context, tx *sql.Tx, entityType, name, externalID string) (int64, error) {
	q, ok := identityQueryByType[entityType]
	if !ok {
		return 0, fmt.Errorf("resolve: unknown entity type %q", entityType)
	}
	name = strings.TrimSpace(name)
	externalID = strings.TrimSpace(externalID)

	// 0. Deny-list (tags only, ADR-075 D2): checked before the resolve order
	// below, so a denied term is refused even if a tags row for it already
	// exists from before it was denied -- denial blocks future association,
	// not just row creation.
	if entityType == model.EntityTag {
		if denied, err := isTagDenied(ctx, tx, name); err != nil {
			return 0, err
		} else if denied {
			return 0, ErrTagDenied
		}
	}

	// 1. External-id first (studios, ADR-054/055): a company id owns exactly one entity.
	if externalID != "" && entityType == model.EnrichEntityStudio {
		var id int64
		switch err := tx.QueryRowContext(ctx,
			`SELECT studio_id FROM studio_external_ids WHERE external_id = ?`, externalID).Scan(&id); {
		case err == nil:
			return id, nil
		case !errors.Is(err, sql.ErrNoRows):
			return 0, fmt.Errorf("resolve %s external id: %w", entityType, err)
		}
	}

	// 2-3. Canonical nameKey, then alias key → canonical entity (survives merges).
	if id, ok, err := lookupByNameKey(ctx, tx, q, entityType, name); err != nil {
		return 0, err
	} else if ok {
		return id, attachExternalID(ctx, tx, entityType, id, externalID)
	}

	// 4. Create, then flag any loose-key near-miss for the review queue (F43 S5,
	//    scan-time flagging — never merges) and attach the id (studios).
	res, err := tx.ExecContext(ctx, q.insert, name)
	if err != nil {
		return 0, fmt.Errorf("insert %s: %w", entityType, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	if err := FlagNearMiss(ctx, tx, entityType, id); err != nil {
		return 0, err
	}
	return id, attachExternalID(ctx, tx, entityType, id, externalID)
}

// queryRower is the read slice both *sql.Tx (inside resolveOrCreateByName's
// transaction) and *sql.DB (ExactEntityMatch's standalone read) satisfy.
type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// lookupByNameKey resolves name to an existing entity via canonical nameKey
// then alias key (ADR-061) — the two-step lookup shared by
// resolveOrCreateByName (which falls through to create-on-miss) and
// ExactEntityMatch (read-only, F48.3c: "same nameKey function, imported not
// reimplemented"). ok is false when neither matches.
func lookupByNameKey(ctx context.Context, qr queryRower, q identityQueries, entityType, name string) (int64, bool, error) {
	var id int64
	switch err := qr.QueryRowContext(ctx, q.canonicalSelect, name).Scan(&id); {
	case err == nil:
		return id, true, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, false, fmt.Errorf("lookup %s name: %w", entityType, err)
	}

	switch err := qr.QueryRowContext(ctx, q.aliasSelect, entityType, name).Scan(&id); {
	case err == nil:
		return id, true, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, false, fmt.Errorf("lookup %s alias: %w", entityType, err)
	}
	return 0, false, nil
}

// attachExternalID records external_id → entity idempotently; a no-op unless the
// entity is a studio carrying an id (ADR-054). Keeps the external-id perimeter in
// studios.go while resolveOrCreateByName stays entity-generic.
func attachExternalID(ctx context.Context, tx *sql.Tx, entityType string, id int64, externalID string) error {
	if externalID == "" || entityType != model.EnrichEntityStudio {
		return nil
	}
	return attachStudioExternalID(ctx, tx, id, externalID)
}

// ExactEntityMatch reports whether name resolves to an existing Person/Studio
// via F43's canonical nameKey or alias-key match (ADR-061) — the same
// normalization resolveOrCreateByName uses (F48.3c: "same nameKey function,
// imported not reimplemented"), reused read-only with no create-on-miss
// fallback. ok is false when no entity exists yet for name.
func (r *Repo) ExactEntityMatch(ctx context.Context, entityType, name string) (id int64, ok bool, err error) {
	q, known := identityQueryByType[entityType]
	if !known {
		return 0, false, fmt.Errorf("exact entity match: unknown entity type %q", entityType)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, nil
	}
	return lookupByNameKey(ctx, r.db, q, entityType, name)
}

// EntityNames returns every Person/Studio id -> name pair for entityType — the
// candidate pool F48.3d's Jaro-Winkler ranking searches when no exact match
// exists. Built on enrichQueueEntities (enrich_queue.go), the same
// "list every name of this entity type" read F47's queue already uses.
func (r *Repo) EntityNames(ctx context.Context, entityType string) (map[int64]string, error) {
	refs, err := r.enrichQueueEntities(ctx, entityType)
	if err != nil {
		return nil, fmt.Errorf("entity names: %w", err)
	}
	out := make(map[int64]string, len(refs))
	for _, ref := range refs {
		out[ref.ID] = ref.Name
	}
	return out, nil
}
