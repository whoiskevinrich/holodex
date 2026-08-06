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

// ErrTagNameTooLong is returned by resolveOrCreateByName when a tag name (entityType
// == model.EntityTag) exceeds maxNameLen runes.
var ErrTagNameTooLong = errors.New("tag: name is too long")

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

// externalIDCols names the join table + FK column an entity type's provider ids live
// in (mirrors idMove's table+fk shape, identity_ops.go). Declared literally per case —
// like canonicalTable, rather than derived from entityType by string concatenation —
// so the naming is one source of truth instead of an assumed convention re-derived at
// each of externalSelect's and attachInsert's build sites below. ok is false for an
// entity type with no external identity (tags).
type externalIDCols struct{ table, idColumn string }

func externalIDTable(entityType string) (externalIDCols, bool) {
	switch entityType {
	case model.EnrichEntityStudio:
		return externalIDCols{table: "studio_external_ids", idColumn: "studio_id"}, true // ADR-054
	case model.EnrichEntityPerson:
		return externalIDCols{table: "person_external_ids", idColumn: "person_id"}, true // ADR-055, F32
	default:
		return externalIDCols{}, false
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
// per person/tag/studio per video) does zero per-call string formatting — externalSelect
// AND attachInsert both get this treatment (not just the read side), since a non-empty
// externalID exercises both on every call. Both are empty for an entity type with no
// external-id table (tags).
type identityQueries struct{ canonicalSelect, aliasSelect, insert, externalSelect, attachInsert string }

var identityQueryByType = func() map[string]identityQueries {
	m := make(map[string]identityQueries, 3)
	for _, et := range []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EntityTag} {
		table := canonicalTable(et)
		q := identityQueries{
			canonicalSelect: fmt.Sprintf(`SELECT id FROM %s WHERE %s = %s`, table, nameKeyExpr(et, "name"), nameKeyExpr(et, "?")),
			aliasSelect:     fmt.Sprintf(`SELECT entity_id FROM entity_aliases WHERE entity_type = ? AND alias_key = %s LIMIT 1`, nameKeyExpr(et, "?")),
			insert:          fmt.Sprintf(`INSERT INTO %s (name) VALUES (?)`, table),
		}
		if cols, ok := externalIDTable(et); ok {
			q.externalSelect = fmt.Sprintf(`SELECT %s FROM %s WHERE external_id = ?`, cols.idColumn, cols.table)
			q.attachInsert = fmt.Sprintf(`INSERT OR IGNORE INTO %s (%s, external_id) VALUES (?, ?)`, cols.table, cols.idColumn)
		}
		m[et] = q
	}
	return m
}()

// resolveOrCreateByName resolves a name to an entity id for person / studio / tag,
// creating the entity if absent (F43, ADR-061). Resolution order (RD3): external-id
// (studio ADR-054, person ADR-055/F32) → canonical nameKey → alias key → create.
// Case/whitespace variants converge on the one canonical entity (the "fox"/"Fox" fix);
// a merged-away name routes through the alias table so a merge survives a re-scan /
// link re-derivation. Runs inside the caller's transaction; writeMu serialization +
// the canonical nameKey unique index make the select-then-insert race-free. externalID
// is honored only for entity types with an external-id table (externalIDTable); empty
// for name-only entities (tags) or when no id is known.
func resolveOrCreateByName(ctx context.Context, tx *sql.Tx, entityType, name, externalID string) (int64, error) {
	q, ok := identityQueryByType[entityType]
	if !ok {
		return 0, fmt.Errorf("resolve: unknown entity type %q", entityType)
	}
	name = strings.TrimSpace(name)
	externalID = strings.TrimSpace(externalID)
	// Tag entities are lower-cased at the one choke point every tag-creation path
	// shares (scanner, manual attach, materialization) -- keeps UX, storage, and
	// writeback in sync without a case-fold at each call site. Person/studio names
	// keep their natural casing; only tags fold. curationNorm is the same trim+lower
	// rule metadata_curation and resolver.NormKey already use (curation.go).
	if entityType == model.EntityTag {
		name = curationNorm(name)
	}

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

	// 1. External-id first (studio ADR-054, person ADR-055/F32): a provider id owns
	// exactly one entity.
	if externalID != "" && q.externalSelect != "" {
		var id int64
		switch err := tx.QueryRowContext(ctx, q.externalSelect, externalID).Scan(&id); {
		case err == nil:
			return id, nil
		case !errors.Is(err, sql.ErrNoRows):
			return 0, fmt.Errorf("resolve %s external id: %w", entityType, err)
		}
	}

	// 2-3. Canonical nameKey, then alias key → canonical entity (survives merges).
	// Runs before the length cap below so a tags row that predates the cap (the
	// scanner had none before ADR-075 item 11) still resolves instead of becoming
	// permanently unreachable.
	if id, ok, err := lookupByNameKey(ctx, tx, q, entityType, name); err != nil {
		return 0, err
	} else if ok {
		return id, attachExternalID(ctx, tx, entityType, q.attachInsert, id, externalID)
	}

	// 3b. Length cap (tags only, ADR-075 item 11): the rename/alias HTTP handlers
	// already cap at model.MaxNameLen runes, but manual attach and materialization
	// call straight in here, bypassing them -- moved into the one choke point every
	// tag-creation path (scanner included) shares, per this ADR's own
	// single-choke-point reasoning for the deny-list above. Only gates *creating* a
	// new row (it runs after the lookup above), not resolving an existing one.
	if entityType == model.EntityTag && len([]rune(name)) > model.MaxNameLen {
		return 0, ErrTagNameTooLong
	}

	// 3c. Cross-table collision with categories (tags only, ADR-078 D3): the
	// symmetric pre-flight check to CreateCategory/RenameCategory's tag-side
	// check, at the one choke point every tag-creation path shares -- the DB
	// triggers from migration 0035 are the correctness backstop either way.
	if entityType == model.EntityTag {
		if collides, err := nameCollidesInTable(ctx, tx, "categories", name, 0); err != nil {
			return 0, err
		} else if collides {
			return 0, ErrTagNameCollidesWithCategory
		}
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
	return id, attachExternalID(ctx, tx, entityType, q.attachInsert, id, externalID)
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

// attachExternalID records external_id → entity idempotently via the precomputed
// attachInsert (identityQueries.attachInsert, "" for an entity type with no
// external-id table — tags); a no-op when externalID is empty. INSERT OR IGNORE: the
// external_id PK means an id already owned by another entity is left where it is —
// the id-first lookup in resolveOrCreateByName would already have returned that
// owner, so this only ever records a genuinely new (id, entity) pair.
func attachExternalID(ctx context.Context, tx *sql.Tx, entityType, attachInsert string, id int64, externalID string) error {
	if externalID == "" || attachInsert == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, attachInsert, id, externalID); err != nil {
		return fmt.Errorf("attach %s external id: %w", entityType, err)
	}
	return nil
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
