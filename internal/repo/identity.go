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
	case model.EnrichEntityFilm:
		return "films"
	default:
		return ""
	}
}

// Provider identity lives in the one polymorphic entity_external_ids table (migration
// 0046, ADR-096 D2) for every kind — person, studio, tag, film — keyed by entity_type,
// so the resolve and attach statements below are kind-agnostic constants rather than
// per-kind table lookups. external_id is the namespace-qualified "<provider>:<id>"
// string (ADR-082's value shape) and is unique PER KIND (the PK), which is the de-dup
// guarantee: an id owns exactly one entity of its kind.
const (
	externalIDSelect = `SELECT entity_id FROM entity_external_ids WHERE entity_type = ? AND external_id = ?`
	externalIDAttach = `INSERT OR IGNORE INTO entity_external_ids (entity_type, entity_id, external_id) VALUES (?, ?, ?)`
)

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
// per person/tag/studio per video) does zero per-call string formatting. The
// external-id statements need no per-kind build (externalIDSelect/externalIDAttach).
type identityQueries struct{ canonicalSelect, aliasSelect, insert string }

var identityQueryByType = func() map[string]identityQueries {
	m := make(map[string]identityQueries, 3)
	for _, et := range []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EntityTag} {
		table := canonicalTable(et)
		q := identityQueries{
			canonicalSelect: fmt.Sprintf(`SELECT id FROM %s WHERE %s = %s`, table, nameKeyExpr(et, "name"), nameKeyExpr(et, "?")),
			aliasSelect:     fmt.Sprintf(`SELECT entity_id FROM entity_aliases WHERE entity_type = ? AND alias_key = %s LIMIT 1`, nameKeyExpr(et, "?")),
			insert:          fmt.Sprintf(`INSERT INTO %s (name) VALUES (?)`, table),
		}
		m[et] = q
	}
	return m
}()

// LookupEntityIDByName resolves a name to an EXISTING entity id, creating nothing.
// It is the read-only prefix of resolveOrCreateByName's order: canonical nameKey, then
// the alias key. The external-id step is skipped (it needs an id the caller does not
// have) and the create step is deliberately absent.
//
// This exists because "does this provider-supplied name already name someone in my
// library?" is a genuine read-only question (F59/ADR-089 D2: the film cast difference
// must be computed by resolved identity, not by display string — otherwise an alias or
// a case variant reads as a missing person). PersonIDByName is not enough: it is a bare
// `name = ? COLLATE NOCASE` match that never consults entity_aliases, so a merged-away
// or aliased name would look absent.
//
// Runs outside a transaction — callers are read paths. Keep the resolution order in
// step with resolveOrCreateByName below.
func (r *Repo) LookupEntityIDByName(ctx context.Context, entityType, name string) (int64, bool, error) {
	q, ok := identityQueryByType[entityType]
	if !ok {
		return 0, false, fmt.Errorf("lookup entity id: unsupported entity type %q", entityType)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return 0, false, nil
	}
	var id int64
	switch err := r.db.QueryRowContext(ctx, q.canonicalSelect, name).Scan(&id); {
	case err == nil:
		return id, true, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, false, fmt.Errorf("lookup %s canonical name: %w", entityType, err)
	}
	switch err := r.db.QueryRowContext(ctx, q.aliasSelect, entityType, name).Scan(&id); {
	case err == nil:
		return id, true, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, false, fmt.Errorf("lookup %s alias: %w", entityType, err)
	}
	return 0, false, nil
}

// resolveOrCreateByName resolves a name to an entity id for person / studio / tag,
// creating the entity if absent (F43, ADR-061). Resolution order (RD3): external-id
// (studio ADR-054, person ADR-055/F32) → canonical nameKey → alias key → create.
// Case/whitespace variants converge on the one canonical entity (the "fox"/"Fox" fix);
// a merged-away name routes through the alias table so a merge survives a re-scan /
// link re-derivation. Runs inside the caller's transaction; writeMu serialization +
// the canonical nameKey unique index make the select-then-insert race-free. externalID
// is empty when no id is known (name-only scan credits).
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

	// 1. External-id first (studio ADR-054, person ADR-055/F32, all kinds ADR-096 D2):
	// a provider id owns exactly one entity of its kind. This step precedes the
	// nameKey/alias steps so a merge or provider link survives a rescan whose
	// spelling exactly names a different entity (TestResolvePrecedence_*).
	if externalID != "" {
		var id int64
		switch err := tx.QueryRowContext(ctx, externalIDSelect, entityType, externalID).Scan(&id); {
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
		return id, attachExternalID(ctx, tx, entityType, id, externalID)
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

// attachExternalID records external_id → entity idempotently; a no-op when
// externalID is empty. INSERT OR IGNORE: the per-kind PK means an id already owned by
// another entity of that kind is left where it is, silently.
//
// That silence is correct HERE and only here (F71 P0-3, ADR-107 D4). Both callers are
// resolveOrCreateByName, and by the time either reaches this writer its step 1 ("external-id
// first") has already looked the id up and returned early if anyone owned it — so a contested id
// cannot arrive here at all, and a guard in this function would be dead code on the scan
// path while still writing review rows inside the caller's scan transaction, which may
// roll back. (ADR-107 D4 gives the reason as "would fire on every relink"; the
// transactional half is the one that actually holds — step 1's early return means the
// flood never materializes.) The public Repo.AttachExternalID is the enrich path, where the
// entity came from an owner's pick and there is no id lookup upstream; it wraps this writer
// with the contested-id guard instead of inheriting the silence. Do not move it in here.
func attachExternalID(ctx context.Context, tx execer, entityType string, id int64, externalID string) error {
	if externalID == "" {
		return nil
	}
	if _, err := tx.ExecContext(ctx, externalIDAttach, entityType, id, externalID); err != nil {
		return fmt.Errorf("attach %s external id: %w", entityType, err)
	}
	return nil
}

// AttachExternalID records a provider id for an entity of any kind outside the scan
// path — the enrich service calls it once a provider record has been adopted for a
// person/studio/tag/film (ADR-096 D2: the identity row and the "which id did we enrich
// against" memo are the same fact). Not for videos, which have no row in
// entity_external_ids.
//
// Idempotent, and NOT silent about a conflict (F71 P0-3, ADR-107 D4). If the id already
// belongs to a different entity of the same kind, the pair goes to
// identity_review_queue as 'shared-external-id' and this returns nil: the enrich must
// still succeed, because the provider's field values are already stored and only the
// identity claim is contested. The entity that holds the spine row keeps it — the owner's
// merge decides, never this write (ADR-107 D3). This is where the collisions the data
// carries came from: the entity is one the OWNER picked in the enrich UI, so unlike the
// scan path there is no id-first resolve upstream to guarantee the id is free.
//
// Why the owner is re-read rather than keyed on RowsAffected: INSERT OR IGNORE collapses
// three outcomes into one nil — inserted, already owned by THIS entity, and owned by
// another. The middle case is the common one on every re-enrich and refresh sweep, so a
// guard that queued whenever nothing was inserted would flood the queue. Only the
// follow-up lookup separates benign idempotence from a real conflict.
func (r *Repo) AttachExternalID(ctx context.Context, entityType string, entityID int64, externalID string) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if err := attachExternalID(ctx, r.db, entityType, entityID, externalID); err != nil {
		return err
	}
	// Tag is excluded with video: it is not enrichable, so a contested tag id is not the
	// owner-mis-pick this feature reports (ADR-107 D5, named test). Video never reaches
	// here at all — enrich.identityEntityType stops it a layer up.
	if externalID == "" || !sharedExternalIDKind(entityType) {
		return nil
	}
	// Same critical section as the attach above: r.writeMu is held for the whole method
	// and both statements run on r.db, so nothing can repoint the id between the write
	// and this read. There is no AttachExternalIDLocked to hand a caller's tx, so leaving
	// and re-entering the lock is the one thing that would make this racy.
	var owner int64
	switch err := r.db.QueryRowContext(ctx, externalIDSelect, entityType, externalID).Scan(&owner); {
	case errors.Is(err, sql.ErrNoRows):
		// Unreachable: the attach either inserted the row or found one already there.
		return nil
	case err != nil:
		return fmt.Errorf("lookup %s external id owner: %w", entityType, err)
	case owner == entityID:
		return nil // inserted just now, or this entity already held it — the common case
	}
	// The queue write, the kind predicate and the boot sweep that shares them live in
	// shared_external_id.go.
	_, err := queueSharedExternalIDPair(ctx, r.db, entityType, owner, entityID)
	return err
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

// ExternalIDsForEntity returns every namespace-qualified external id stored for an
// entity of any kind (entity_external_ids, ADR-096 D2) — the read source for the
// HOLODEX-266/ADR-083 provider-link badge projection. Each value is already
// "<namespace>:<id>" (ADR-082's value shape, shared with the video
// _person_external_ids/_studio_external_ids enrichment sidecars). Empty, not an
// error, for an entity with no ids.
func (r *Repo) ExternalIDsForEntity(ctx context.Context, entityType string, entityID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT external_id FROM entity_external_ids WHERE entity_type = ? AND entity_id = ? ORDER BY external_id`,
		entityType, entityID)
	if err != nil {
		return nil, fmt.Errorf("external ids for %s: %w", entityType, err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
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
