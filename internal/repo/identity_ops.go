package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"holodex/internal/model"
)

// Entity-generic name-identity mutations (F43 S2, ADR-061): owner-curated alias
// add/delete, merge, rename, and the keep-separate store — one implementation shared
// by person, studio, and tag over the polymorphic entity_aliases spine. The person
// wrappers in aliases.go delegate here so all three entities route through one code
// path (the ADR-052 "zero core divergence" property, held for identity). Every
// normalized key is computed in SQL via nameKeyExpr — never in Go — so the alias_key
// generated column, the canonical unique index, and every predicate agree byte-for-byte.
//
// Reads are lock-free (WAL); writes take writeMu like the rest of the write path.

// entityIdentity describes the per-type tables a merge repoints. The alias /
// keep-separate / rename machinery is identical across types (it operates on
// entity_aliases keyed by entity_type); only the canonical table, its association
// junction, and any extra FK-to-entity tables (studio external ids) vary.
type entityIdentity struct {
	table   string   // canonical table (people | studios | tags)
	assoc   string   // association junction table
	assocFK string   // junction column referencing the entity
	idMoves []idMove // extra tables whose entity FK is repointed on merge
}

// idMove names a table + column whose reference to the merged entity is repointed
// onto the survivor (studio_external_ids, so a merged studio's provider identity
// keeps resolving to the survivor — matching the migration-0022 fold).
type idMove struct{ table, fk string }

// identityStep is one ordered, described SQL statement in a merge/rename transaction
// (the desc names the step in any wrapped error).
type identityStep struct {
	desc string
	sql  string
	args []any
}

var entityIdentityByType = map[string]entityIdentity{
	model.EnrichEntityPerson: {"people", "video_people", "person_id", nil},
	model.EnrichEntityStudio: {"studios", "video_studios", "studio_id", []idMove{{"studio_external_ids", "studio_id"}}},
	model.EntityTag:          {"tags", "video_tags", "tag_id", nil},
}

// entityAliasKeyByType holds, per entity type, the SQL predicate matching an alias by
// its normalized key: `entity_type = ? AND alias_key = <nameKeyExpr>`. `?` binds the
// entity type, then the raw (trimmed) name. Precomputed so nameKeyExpr's per-entity
// rule (tag also folds internal whitespace) has one source of truth.
var entityAliasKeyByType = func() map[string]string {
	m := make(map[string]string, 3)
	for _, et := range []string{model.EnrichEntityPerson, model.EnrichEntityStudio, model.EntityTag} {
		m[et] = `entity_type = ? AND alias_key = ` + nameKeyExpr(et, "?")
	}
	return m
}()

// EntityExists reports that an entity id is present (ErrNotFound otherwise), skipping
// the count/alias fetches the typed getters do — a cheap existence check on the write
// path (e.g. before adding an alias). entityType is a trusted internal literal.
func (r *Repo) EntityExists(ctx context.Context, entityType string, id int64) error {
	table := canonicalTable(entityType)
	if table == "" {
		return fmt.Errorf("entity exists: unknown entity type %q", entityType)
	}
	var x int
	err := r.db.QueryRowContext(ctx, `SELECT 1 FROM `+table+` WHERE id = ?`, id).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// AliasesForEntity returns an entity's aliases ordered case-insensitively by name.
// Always returns a non-nil slice on success so the JSON serializes as [] not null.
func (r *Repo) AliasesForEntity(ctx context.Context, entityType string, id int64) ([]model.EntityAlias, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, alias FROM entity_aliases WHERE entity_type = ? AND entity_id = ?
		 ORDER BY alias COLLATE NOCASE`, entityType, id)
	if err != nil {
		return nil, fmt.Errorf("aliases for %s: %w", entityType, err)
	}
	defer rows.Close()
	out := []model.EntityAlias{}
	for rows.Next() {
		var a model.EntityAlias
		if err := rows.Scan(&a.ID, &a.Alias); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AliasesForEntities returns aliases for many entities of one type in a single query,
// keyed by entity id (for list reads that show aliases inline, e.g. /tags). Entities
// with no alias are simply absent from the map.
func (r *Repo) AliasesForEntities(ctx context.Context, entityType string, ids []int64) (map[int64][]model.EntityAlias, error) {
	out := make(map[int64][]model.EntityAlias, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT entity_id, id, alias FROM entity_aliases
		WHERE entity_type = ? AND entity_id IN (`+placeholders(len(ids))+`)
		ORDER BY alias COLLATE NOCASE`, append([]any{entityType}, toAnySlice(ids)...)...)
	if err != nil {
		return nil, fmt.Errorf("aliases for %ss: %w", entityType, err)
	}
	defer rows.Close()
	for rows.Next() {
		var entityID int64
		var a model.EntityAlias
		if err := rows.Scan(&entityID, &a.ID, &a.Alias); err != nil {
			return nil, err
		}
		out[entityID] = append(out[entityID], a)
	}
	return out, rows.Err()
}

// AddEntityAlias adds an alias to an entity and returns the stored row. Re-adding an
// alias the entity already has (case/whitespace-folded) is idempotent: the existing
// row is returned, no duplicate, no error. The caller validates/trims the alias,
// confirms the entity exists, and runs EntityConflict first so the alias key isn't
// already owned by another entity of this type (P0-5); an empty alias is rejected here
// as a guard. The unique (entity_type, alias_key) constraint dedupes.
func (r *Repo) AddEntityAlias(ctx context.Context, entityType string, id int64, alias string) (model.EntityAlias, error) {
	if alias == "" {
		return model.EntityAlias{}, errors.New("empty alias")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if _, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias) VALUES (?, ?, ?)`,
		entityType, id, alias); err != nil {
		return model.EntityAlias{}, fmt.Errorf("add %s alias: %w", entityType, err)
	}
	// Resolve the row (inserted or pre-existing) by the normalized key — folding
	// means "rob" returns this entity's existing "Rob".
	var a model.EntityAlias
	if err := r.db.QueryRowContext(ctx,
		`SELECT id, alias FROM entity_aliases WHERE entity_id = ? AND `+entityAliasKeyByType[entityType],
		id, entityType, alias).Scan(&a.ID, &a.Alias); err != nil {
		return model.EntityAlias{}, fmt.Errorf("resolve %s alias: %w", entityType, err)
	}
	return a, nil
}

// DeleteEntityAlias removes one alias, scoped to its (entity_type, entity_id) so a
// mismatched triple can't delete another entity's alias. ErrNotFound when no such
// alias belongs to the entity.
func (r *Repo) DeleteEntityAlias(ctx context.Context, entityType string, id, aliasID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM entity_aliases WHERE id = ? AND entity_type = ? AND entity_id = ?`,
		aliasID, entityType, id)
	if err != nil {
		return fmt.Errorf("delete %s alias: %w", entityType, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// EntityConflict returns the id of the OTHER entity of entityType that `name` already
// resolves to — by canonical nameKey or by being another entity's alias — with
// found=false when the name is free. Used to refuse a silent merge of possibly-distinct
// same-named entities (the homonym rule, F23/RD4): the caller surfaces it for the owner
// to confirm. selfID is the entity the name is being attached to (excluded).
func (r *Repo) EntityConflict(ctx context.Context, entityType string, selfID int64, name string) (int64, bool, error) {
	table := canonicalTable(entityType)
	if table == "" {
		return 0, false, fmt.Errorf("entity conflict: unknown entity type %q", entityType)
	}
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM `+table+` WHERE `+nameKeyExpr(entityType, "name")+` = `+nameKeyExpr(entityType, "?")+` AND id <> ?
		UNION
		SELECT entity_id FROM entity_aliases WHERE `+entityAliasKeyByType[entityType]+` AND entity_id <> ?
		LIMIT 1`, name, selfID, entityType, name, selfID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("%s conflict: %w", entityType, err)
	}
	return id, true, nil
}

// MergeEntities folds `mergedID` into `canonicalID` for person / studio / tag (F43
// P0-3/P0-4, ADR-061 D6): it moves the merged entity's associations (de-duped union)
// and any extra id references (studio external ids) onto the survivor, re-points the
// merged entity's aliases, registers the merged entity's name as an alias of the
// survivor, drops the merged entity's shadow enrichment/decisions/curation (the
// survivor keeps its own — matching MergePersons and the migration-0022 fold), and
// deletes it. Registering the loser's name as an alias is load-bearing: for studios it
// makes the merge survive RelinkVideoStudios re-derivation; for people/tags it makes it
// survive a re-scan (both route the old name through the alias table). One transaction
// under the write lock. ErrNotFound if either id is missing; an error on a self-merge.
func (r *Repo) MergeEntities(ctx context.Context, entityType string, canonicalID, mergedID int64) error {
	if canonicalID == mergedID {
		return errors.New("cannot merge an entity into itself")
	}
	cfg, ok := entityIdentityByType[entityType]
	if !ok {
		return fmt.Errorf("merge: unknown entity type %q", entityType)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// Both must exist; capture the merged name (becomes the new alias) and the
	// canonical name (to tidy a degenerate self-alias at the end).
	var canonicalName, mergedName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM `+cfg.table+` WHERE id = ?`, canonicalID).Scan(&canonicalName); err != nil {
		return mergeEntityLookupErr(entityType, err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT name FROM `+cfg.table+` WHERE id = ?`, mergedID).Scan(&mergedName); err != nil {
		return mergeEntityLookupErr(entityType, err)
	}

	steps := []identityStep{
		// 1. Move associations as a de-duped union (composite PK + OR IGNORE).
		{"move associations", `INSERT OR IGNORE INTO ` + cfg.assoc + ` (video_id, ` + cfg.assocFK + `)
			SELECT video_id, ? FROM ` + cfg.assoc + ` WHERE ` + cfg.assocFK + ` = ?`, []any{canonicalID, mergedID}},
		{"clear merged associations", `DELETE FROM ` + cfg.assoc + ` WHERE ` + cfg.assocFK + ` = ?`, []any{mergedID}},
	}
	// 1b. Repoint any extra id references (studio external ids) onto the survivor so
	//     provider identity keeps resolving there; OR IGNORE drops a duplicate the
	//     survivor already owns (the FK-cascade delete below then clears the loser's).
	for _, mv := range cfg.idMoves {
		steps = append(steps, identityStep{"move " + mv.table, `UPDATE OR IGNORE ` + mv.table + ` SET ` + mv.fk + ` = ? WHERE ` + mv.fk + ` = ?`, []any{canonicalID, mergedID}})
	}
	steps = append(steps, []identityStep{
		// 2. Preserve a prior merge chain: re-point merged's aliases, drop collisions.
		{"repoint aliases", `UPDATE OR IGNORE entity_aliases SET entity_id = ? WHERE entity_type = ? AND entity_id = ?`, []any{canonicalID, entityType, mergedID}},
		{"drop collided aliases", `DELETE FROM entity_aliases WHERE entity_type = ? AND entity_id = ?`, []any{entityType, mergedID}},
		// 3. The merged entity's name becomes an alias of the survivor (load-bearing —
		//    routes the old name to the survivor on re-derivation / re-scan).
		{"name → alias", `INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias) VALUES (?, ?, ?)`, []any{entityType, canonicalID, mergedName}},
		// 4. Drop the merged entity's shadow enrichment / decisions / curation (the
		//    survivor keeps its own — F37 RD5; no-op for tags, which have none).
		{"drop enrichment", `DELETE FROM entity_enrichment WHERE entity_type = ? AND entity_id = ?`, []any{entityType, mergedID}},
		{"drop decisions", `DELETE FROM field_source_decisions WHERE entity_type = ? AND entity_id = ?`, []any{entityType, mergedID}},
		{"drop curation", `DELETE FROM metadata_curation WHERE entity_type = ? AND entity_id = ?`, []any{entityType, mergedID}},
		// 5. Remove the now-empty duplicate (FK cascade + the *_ad_aliases / FTS
		//    triggers clean up its junction / image / external-id / alias / FTS rows).
		{"delete entity", `DELETE FROM ` + cfg.table + ` WHERE id = ?`, []any{mergedID}},
		// 6. Tidy a degenerate self-alias (the survivor's own name routed in via step 2
		//    of a prior merge, or a name→alias collision).
		{"drop self-alias", `DELETE FROM entity_aliases WHERE entity_type = ? AND entity_id = ? AND alias = ?`, []any{entityType, canonicalID, canonicalName}},
	}...)

	for _, step := range steps {
		if _, err := tx.ExecContext(ctx, step.sql, step.args...); err != nil {
			return fmt.Errorf("merge (%s): %w", step.desc, err)
		}
	}
	// The merge resolves any review-queue pair touching the loser (F43 S5) — drop it so
	// a merged-away duplicate leaves no stale row (covers person/studio/tag).
	if err := dropReviewPairsFor(ctx, tx, entityType, mergedID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit merge: %w", err)
	}
	return nil
}

// RenameEntity sets an entity's name and keeps the previous name as an alias in one
// transaction (F43, mirrors F37 RenamePerson), so search and scan/derivation routing
// still match the old spelling. The canonical *_fts UPDATE trigger and entity_aliases_ai
// keep both FTS mirrors correct. When newName's nameKey already belongs to another
// entity of this type (case/whitespace-folded, RD2) it returns that entity's id with
// ErrNameTaken and mutates nothing. Renaming to the current exact name is a no-op
// success; a pure-case self-rename ("Fox"→"fox") proceeds and keeps the old spelling as
// an alias. The caller validates/trims newName.
func (r *Repo) RenameEntity(ctx context.Context, entityType string, id int64, newName string) (conflictID int64, err error) {
	if newName == "" {
		return 0, errors.New("empty name")
	}
	table := canonicalTable(entityType)
	if table == "" {
		return 0, fmt.Errorf("rename: unknown entity type %q", entityType)
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var oldName string
	switch err := tx.QueryRowContext(ctx, `SELECT name FROM `+table+` WHERE id = ?`, id).Scan(&oldName); {
	case errors.Is(err, sql.ErrNoRows):
		return 0, ErrNotFound
	case err != nil:
		return 0, fmt.Errorf("rename: load %s: %w", entityType, err)
	}
	if oldName == newName {
		return 0, nil // no-op: nothing to rename, nothing to alias
	}
	var cid int64
	switch err := tx.QueryRowContext(ctx,
		`SELECT id FROM `+table+` WHERE `+nameKeyExpr(entityType, "name")+` = `+nameKeyExpr(entityType, "?")+` AND id <> ?`,
		newName, id).Scan(&cid); {
	case err == nil:
		return cid, ErrNameTaken
	case !errors.Is(err, sql.ErrNoRows):
		return 0, fmt.Errorf("rename: conflict check: %w", err)
	}

	for _, step := range []identityStep{
		// 1. The rename itself; the <table>_au trigger refreshes the canonical FTS.
		{"set name", `UPDATE ` + table + ` SET name = ? WHERE id = ?`, []any{newName, id}},
		// 2. Keep the old name reachable for search + routing (idempotent per the
		//    (entity_type, alias_key) key, matching AddEntityAlias).
		{"old name → alias", `INSERT OR IGNORE INTO entity_aliases (entity_type, entity_id, alias) VALUES (?, ?, ?)`, []any{entityType, id, oldName}},
		// 3. Tidy a degenerate self-alias — renaming to one of the entity's own aliases
		//    leaves that alias equal to the canonical name (exact-name comparison).
		{"drop self-alias", `DELETE FROM entity_aliases WHERE entity_type = ? AND entity_id = ? AND alias = ?`, []any{entityType, id, newName}},
	} {
		if _, err := tx.ExecContext(ctx, step.sql, step.args...); err != nil {
			return 0, fmt.Errorf("rename (%s): %w", step.desc, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit rename: %w", err)
	}
	return 0, nil
}

func mergeEntityLookupErr(entityType string, err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("merge: load %s: %w", entityType, err)
}

// AddKeepSeparate records that two entities of one type are deliberately distinct
// (RD5): the durable negative of an alias, consulted by the near-miss detector and the
// duplicates queue so a kept-separate pair is never re-proposed. Idempotent; ids are
// ordered (id_lo/id_hi) so a pair is stored once regardless of argument order.
func (r *Repo) AddKeepSeparate(ctx context.Context, entityType string, idA, idB int64) error {
	lo, hi := orderPair(idA, idB)
	if lo == hi {
		return errors.New("cannot keep an entity separate from itself")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	if _, err := r.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO entity_keep_separate (entity_type, id_lo, id_hi) VALUES (?, ?, ?)`,
		entityType, lo, hi); err != nil {
		return fmt.Errorf("keep separate: %w", err)
	}
	return nil
}

// IsKeptSeparate reports whether the pair has been marked deliberately distinct (RD5).
func (r *Repo) IsKeptSeparate(ctx context.Context, entityType string, idA, idB int64) (bool, error) {
	lo, hi := orderPair(idA, idB)
	var x int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM entity_keep_separate WHERE entity_type = ? AND id_lo = ? AND id_hi = ?`,
		entityType, lo, hi).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("is kept separate: %w", err)
	}
	return true, nil
}

// orderPair returns the two ids as (min, max) so a keep-separate pair is stored and
// looked up under one canonical ordering.
func orderPair(a, b int64) (int64, int64) {
	if a <= b {
		return a, b
	}
	return b, a
}
