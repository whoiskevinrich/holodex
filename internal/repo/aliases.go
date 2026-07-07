package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"holodex/internal/model"
)

// Person aliases (F23, ADR-036): owner-curated alternate names on the shared
// entity_aliases spine (F43, ADR-061, entity_type='person'), mirrored into
// entity_aliases_fts by triggers so global search matches any alias. These wrappers
// delegate to the entity-generic identity ops (identity_ops.go) so person, studio, and
// tag share one implementation; the person-typed signatures keep the F23 surface stable.

// AliasesForPerson returns a person's aliases ordered case-insensitively by name.
func (r *Repo) AliasesForPerson(ctx context.Context, personID int64) ([]model.PersonAlias, error) {
	return r.AliasesForEntity(ctx, model.EnrichEntityPerson, personID)
}

// AddPersonAlias adds an alias to a person, idempotent by normalized key (ADR-036).
func (r *Repo) AddPersonAlias(ctx context.Context, personID int64, alias string) (model.PersonAlias, error) {
	return r.AddEntityAlias(ctx, model.EnrichEntityPerson, personID, alias)
}

// DeletePersonAlias removes one alias, scoped to its person; ErrNotFound if absent.
func (r *Repo) DeletePersonAlias(ctx context.Context, personID, aliasID int64) error {
	return r.DeleteEntityAlias(ctx, model.EnrichEntityPerson, personID, aliasID)
}

// resolveOrCreatePerson resolves an extracted person name to a person id via the shared
// name-identity spine (F43, ADR-061): case/whitespace variants converge, and a merged-
// away name routes through the alias table so a merge survives a re-scan. Thin wrapper
// over resolveOrCreateByName; runs inside the scan transaction.
func resolveOrCreatePerson(ctx context.Context, tx *sql.Tx, name string) (int64, error) {
	return resolveOrCreateByName(ctx, tx, model.EnrichEntityPerson, name, "")
}

// PersonConflict returns the existing OTHER person that a candidate alias already
// resolves to — by an exact person name or by being another person's alias — or
// (nil, nil) if the name is free. Used to refuse a silent merge of possibly-distinct
// same-named people (F23, ADR-036): the caller surfaces this for the owner to confirm.
// selfID is the person the alias is being added to (excluded).
func (r *Repo) PersonConflict(ctx context.Context, selfID int64, alias string) (*model.Person, error) {
	id, found, err := r.EntityConflict(ctx, model.EnrichEntityPerson, selfID, alias)
	if err != nil || !found {
		return nil, err
	}
	return r.GetPerson(ctx, id)
}

// MergePersons folds the `merged` person into `canonical` (F23, ADR-036): moves its
// videos, re-points its aliases, registers its name as an alias of canonical, drops its
// shadow enrichment/decisions/curation, and deletes it. See MergeEntities. ErrNotFound
// if either person is missing; an error if the two ids are equal.
func (r *Repo) MergePersons(ctx context.Context, canonicalID, mergedID int64) error {
	return r.MergeEntities(ctx, model.EnrichEntityPerson, canonicalID, mergedID)
}

// ErrNameTaken is returned by RenameEntity (and RenamePerson) when the new name already
// belongs to another entity of the same type; the caller surfaces that entity for the
// explicit merge flow (a rename never silently collides or auto-merges).
var ErrNameTaken = errors.New("name taken")

// RenamePerson sets a person's name and keeps the previous name as an alias so search
// and scan routing still match the old spelling (F37 P0-5). See RenameEntity: it
// returns the conflicting person id with ErrNameTaken when newName's nameKey already
// belongs to another person, and mutates nothing.
func (r *Repo) RenamePerson(ctx context.Context, id int64, newName string) (conflictID int64, err error) {
	return r.RenameEntity(ctx, model.EnrichEntityPerson, id, newName)
}

// searchPeopleByAlias returns person ids/names whose any alias matches the FTS query,
// deduped by person (a person with several matching aliases appears once). The
// entity_type filter in the JOIN scopes the shared entity_aliases_fts to people (F43,
// ADR-061); studio/tag alias search rides the same mirror (S3).
func (r *Repo) searchPeopleByAlias(ctx context.Context, match string, limit int) ([]model.Person, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT DISTINCT p.id, p.name
		FROM entity_aliases_fts f
		JOIN entity_aliases a ON a.id = f.rowid AND a.entity_type = 'person'
		JOIN people p         ON p.id = a.entity_id
		WHERE entity_aliases_fts MATCH ? LIMIT ?`, match, limit)
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
