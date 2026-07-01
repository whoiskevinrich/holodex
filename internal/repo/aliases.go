package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"holodex/internal/model"
)

// Person aliases (F23, ADR-036): owner-curated alternate names, each mirrored into
// person_aliases_fts by triggers so global search matches any alias. Reads are
// unlocked (WAL); writes take writeMu like the rest of the write path.

// AliasesForPerson returns a person's aliases ordered case-insensitively by name.
// Always returns a non-nil slice on success so the JSON serializes as [] not null.
func (r *Repo) AliasesForPerson(ctx context.Context, personID int64) ([]model.PersonAlias, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, alias FROM person_aliases WHERE person_id = ? ORDER BY alias COLLATE NOCASE`, personID)
	if err != nil {
		return nil, fmt.Errorf("aliases for person: %w", err)
	}
	defer rows.Close()
	out := []model.PersonAlias{}
	for rows.Next() {
		var a model.PersonAlias
		if err := rows.Scan(&a.ID, &a.Alias); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// AddPersonAlias adds an alias to a person and returns the stored row. Adding an
// alias the person already has (case-insensitively) is idempotent: the existing
// row is returned, no duplicate, no error (ADR-036). The caller validates/trims
// the alias and confirms the person exists; an empty alias is rejected here as a
// guard. The unique (person_id, alias COLLATE NOCASE) constraint dedupes.
func (r *Repo) AddPersonAlias(ctx context.Context, personID int64, alias string) (model.PersonAlias, error) {
	if alias == "" {
		return model.PersonAlias{}, errors.New("empty alias")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO person_aliases (person_id, alias) VALUES (?, ?)
		 ON CONFLICT(person_id, alias) DO NOTHING`, personID, alias); err != nil {
		return model.PersonAlias{}, fmt.Errorf("add person alias: %w", err)
	}
	// Resolve the row (inserted or pre-existing) by the unique key — the NOCASE
	// collation means "rob" returns an existing "Rob".
	var a model.PersonAlias
	if err := r.db.QueryRowContext(ctx,
		`SELECT id, alias FROM person_aliases WHERE person_id = ? AND alias = ?`,
		personID, alias).Scan(&a.ID, &a.Alias); err != nil {
		return model.PersonAlias{}, fmt.Errorf("resolve person alias: %w", err)
	}
	return a, nil
}

// DeletePersonAlias removes one alias, scoped to its person so a mismatched
// (alias, person) pair can't delete another person's alias. ErrNotFound when no
// such alias belongs to the person.
func (r *Repo) DeletePersonAlias(ctx context.Context, personID, aliasID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM person_aliases WHERE id = ? AND person_id = ?`, aliasID, personID)
	if err != nil {
		return fmt.Errorf("delete person alias: %w", err)
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

// resolveOrCreatePerson resolves an extracted person name to a person id, routing
// through the alias table so a merged person's name lands on the canonical person
// and the merge survives a re-scan (F23, ADR-036). Resolution order: exact person
// name (NOCASE) → alias (NOCASE) → create. Runs inside the scan transaction; the
// alias lookup is backed by idx_person_aliases_alias.
func resolveOrCreatePerson(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	name = strings.TrimSpace(name)
	var id int64
	switch err := tx.QueryRowContext(ctx, `SELECT id FROM people WHERE name = ?`, name).Scan(&id); {
	case err == nil:
		return id, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, fmt.Errorf("resolve person name: %w", err)
	}
	switch err := tx.QueryRowContext(ctx,
		`SELECT person_id FROM person_aliases WHERE alias = ? LIMIT 1`, name).Scan(&id); {
	case err == nil:
		return id, nil
	case !errors.Is(err, sql.ErrNoRows):
		return 0, fmt.Errorf("resolve person alias: %w", err)
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO people (name) VALUES (?)`, name)
	if err != nil {
		return 0, fmt.Errorf("insert person: %w", err)
	}
	return res.LastInsertId()
}

// PersonConflict returns the existing OTHER person that a candidate alias already
// resolves to — by an exact person name (NOCASE) or by being another person's
// alias — or (nil, nil) if the name is free. Used to refuse a silent merge of
// possibly-distinct same-named people (F23, ADR-036): the caller surfaces this for
// the owner to confirm. selfID is the person the alias is being added to (excluded).
func (r *Repo) PersonConflict(ctx context.Context, selfID int64, alias string) (*model.Person, error) {
	alias = strings.TrimSpace(alias)
	var id int64
	err := r.db.QueryRowContext(ctx, `
		SELECT id FROM people WHERE name = ? AND id <> ?
		UNION
		SELECT person_id FROM person_aliases WHERE alias = ? AND person_id <> ?
		LIMIT 1`, alias, selfID, alias, selfID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("person conflict: %w", err)
	}
	return r.GetPerson(ctx, id)
}

// MergePersons folds the `merged` person into `canonical` (F23, ADR-036): moves the
// merged person's videos (de-duped union), re-points its aliases, registers its name
// as an alias of canonical, drops its shadow enrichment, and deletes it. One
// transaction under the write lock. ErrNotFound if either person is missing; an
// error if the two ids are equal.
func (r *Repo) MergePersons(ctx context.Context, canonicalID, mergedID int64) error {
	if canonicalID == mergedID {
		return errors.New("cannot merge a person into itself")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	// Both must exist; capture the merged name (becomes the new alias).
	var canonicalName, mergedName string
	if err := tx.QueryRowContext(ctx, `SELECT name FROM people WHERE id = ?`, canonicalID).Scan(&canonicalName); err != nil {
		return mergeLookupErr(err)
	}
	if err := tx.QueryRowContext(ctx, `SELECT name FROM people WHERE id = ?`, mergedID).Scan(&mergedName); err != nil {
		return mergeLookupErr(err)
	}

	for _, step := range []struct {
		desc string
		sql  string
		args []any
	}{
		// 1. Move associations as a de-duped union (composite PK + OR IGNORE).
		{"move videos", `INSERT OR IGNORE INTO video_people (video_id, person_id)
			SELECT video_id, ? FROM video_people WHERE person_id = ?`, []any{canonicalID, mergedID}},
		{"clear merged videos", `DELETE FROM video_people WHERE person_id = ?`, []any{mergedID}},
		// 2. Preserve a prior merge chain: re-point merged's aliases, drop collisions.
		{"repoint aliases", `UPDATE OR IGNORE person_aliases SET person_id = ? WHERE person_id = ?`, []any{canonicalID, mergedID}},
		{"drop collided aliases", `DELETE FROM person_aliases WHERE person_id = ?`, []any{mergedID}},
		// 3. The merged person's name becomes an alias of canonical.
		{"name → alias", `INSERT OR IGNORE INTO person_aliases (person_id, alias) VALUES (?, ?)`, []any{canonicalID, mergedName}},
		// 4. Drop the merged person's shadow enrichment (canonical keeps its own).
		{"drop enrichment", `DELETE FROM entity_enrichment WHERE entity_type = ? AND entity_id = ?`, []any{model.EnrichEntityPerson, mergedID}},
		// 4b. Drop its field-source decisions and value curation the same way
		// (F37 RD5): the canonical person's own rows win, nothing is migrated —
		// otherwise the rows would orphan against the deleted id.
		{"drop decisions", `DELETE FROM field_source_decisions WHERE entity_type = ? AND entity_id = ?`, []any{model.EnrichEntityPerson, mergedID}},
		{"drop curation", `DELETE FROM metadata_curation WHERE entity_type = ? AND entity_id = ?`, []any{model.EnrichEntityPerson, mergedID}},
		// 5. Remove the now-empty duplicate (cascade + FTS trigger clean up the rest).
		{"delete person", `DELETE FROM people WHERE id = ?`, []any{mergedID}},
		// 6. Tidy a degenerate self-alias (canonical's own name routed in via step 2).
		{"drop self-alias", `DELETE FROM person_aliases WHERE person_id = ? AND alias = ?`, []any{canonicalID, canonicalName}},
	} {
		if _, err := tx.ExecContext(ctx, step.sql, step.args...); err != nil {
			return fmt.Errorf("merge (%s): %w", step.desc, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit merge: %w", err)
	}
	return nil
}

// ErrNameTaken is returned by RenamePerson when the new name already belongs
// to another person; the caller surfaces that person for the explicit merge
// flow (F37 RD1 — a rename never silently collides or auto-merges).
var ErrNameTaken = errors.New("name taken")

// RenamePerson sets a person's name and keeps the previous name as an F23
// alias in one transaction (F37 P0-5), so search and scan routing still match
// the old spelling. The people_au / person_aliases_ai triggers keep both FTS
// mirrors correct. When newName already names another person (exact match,
// mirroring the people.name UNIQUE constraint's binary collation) it returns
// that person's id with ErrNameTaken and mutates nothing. Renaming to the
// current name is a no-op success. The caller validates/trims newName.
func (r *Repo) RenamePerson(ctx context.Context, id int64, newName string) (conflictID int64, err error) {
	if newName == "" {
		return 0, errors.New("empty name")
	}
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op after Commit

	var oldName string
	switch err := tx.QueryRowContext(ctx, `SELECT name FROM people WHERE id = ?`, id).Scan(&oldName); {
	case errors.Is(err, sql.ErrNoRows):
		return 0, ErrNotFound
	case err != nil:
		return 0, fmt.Errorf("rename: load person: %w", err)
	}
	if oldName == newName {
		return 0, nil // no-op: nothing to rename, nothing to alias
	}
	var cid int64
	switch err := tx.QueryRowContext(ctx,
		`SELECT id FROM people WHERE name = ? AND id <> ?`, newName, id).Scan(&cid); {
	case err == nil:
		return cid, ErrNameTaken
	case !errors.Is(err, sql.ErrNoRows):
		return 0, fmt.Errorf("rename: conflict check: %w", err)
	}

	for _, step := range []struct {
		desc string
		sql  string
		args []any
	}{
		// 1. The rename itself; the people_au trigger refreshes people_fts.
		{"set name", `UPDATE people SET name = ? WHERE id = ?`, []any{newName, id}},
		// 2. Keep the old name reachable for search + scan routing (idempotent
		//    per the (person_id, alias) NOCASE key, matching AddPersonAlias).
		{"old name → alias", `INSERT OR IGNORE INTO person_aliases (person_id, alias) VALUES (?, ?)`, []any{id, oldName}},
		// 3. Tidy a degenerate self-alias — renaming to one of the person's own
		//    aliases leaves that alias equal to the canonical name (mirrors the
		//    merge transaction's self-alias cleanup; NOCASE column comparison).
		{"drop self-alias", `DELETE FROM person_aliases WHERE person_id = ? AND alias = ?`, []any{id, newName}},
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

func mergeLookupErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return fmt.Errorf("merge: load person: %w", err)
}

// searchPeopleByAlias returns person ids/names whose any alias matches the FTS
// query, deduped by person (a person with several matching aliases appears once).
func (r *Repo) searchPeopleByAlias(ctx context.Context, match string, limit int) ([]model.Person, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT p.id, p.name
		FROM person_aliases_fts f
		JOIN person_aliases a ON a.id = f.rowid
		JOIN people p         ON p.id = a.person_id
		WHERE person_aliases_fts MATCH ? LIMIT ?`, match, limit)
	if err != nil {
		return nil, fmt.Errorf("search aliases: %w", err)
	}
	defer rows.Close()
	var out []model.Person
	for rows.Next() {
		var p model.Person
		if err := rows.Scan(&p.ID, &p.Name); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
