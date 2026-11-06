package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"holodex/internal/model"
	"holodex/internal/query"
)

// queryKinds maps an F87 entity field to its table and entity type. Categories have no
// aliases (entity_type "").
var queryKinds = map[string]struct{ table, entityType string }{
	query.FieldTag:      {"tags", model.EntityTag},
	query.FieldPerson:   {"people", model.EnrichEntityPerson},
	query.FieldStudio:   {"studios", model.EnrichEntityStudio},
	query.FieldCategory: {"categories", ""},
}

func queryKind(kind string) (table, entityType, keyType string, err error) {
	k, ok := queryKinds[kind]
	if !ok {
		return "", "", "", fmt.Errorf("query: unknown entity kind %q", kind)
	}
	keyType = k.entityType
	if keyType == "" { // categories key like tags (ux_categories_namekey)
		keyType = model.EntityTag
	}
	return k.table, k.entityType, keyType, nil
}

// QueryLookup returns every entity a name means for an F87 condition: the one whose
// canonical name key matches and every one carrying it as an alias, by the identity
// lookup's own name key (entity-identity.md). More than one is ambiguous (P0-7).
func (r *Repo) QueryLookup(ctx context.Context, kind, name string) ([]query.Candidate, error) {
	table, entityType, keyType, err := queryKind(kind)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if keyType != model.EntityTag {
		// The tag key already drops spaces; a person or studio key only trims, so a
		// typed run of spaces would miss the one-space name (P0-1).
		name = strings.Join(strings.Fields(name), " ")
	}
	if name == "" {
		return nil, nil
	}
	q := fmt.Sprintf(`SELECT id, name FROM %s WHERE %s = %s`, table, nameKeyExpr(keyType, "name"), nameKeyExpr(keyType, "?"))
	args := []any{name}
	if entityType != "" {
		q += fmt.Sprintf(` UNION SELECT e.id, e.name FROM entity_aliases a JOIN %s e ON e.id = a.entity_id
			WHERE a.entity_type = ? AND a.alias_key = %s`, table, nameKeyExpr(keyType, "?"))
		args = append(args, entityType, name)
	}
	rows, err := r.db.QueryContext(ctx, q+` ORDER BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("query lookup %s: %w", kind, err)
	}
	defer rows.Close()
	var out []query.Candidate
	for rows.Next() {
		var c query.Candidate
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// QueryNameOf returns an entity's current name; ok is false when it no longer exists.
func (r *Repo) QueryNameOf(ctx context.Context, kind string, id int64) (string, bool, error) {
	table, _, _, err := queryKind(kind)
	if err != nil {
		return "", false, err
	}
	var name string
	err = r.db.QueryRowContext(ctx, `SELECT name FROM `+table+` WHERE id = ?`, id).Scan(&name)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("query name of %s %d: %w", kind, id, err)
	}
	return name, true, nil
}

// QueryNames returns every name and alias of kind, deduplicated: the pool an unknown
// name's suggestions are ranked from.
func (r *Repo) QueryNames(ctx context.Context, kind string) ([]string, error) {
	table, entityType, _, err := queryKind(kind)
	if err != nil {
		return nil, err
	}
	q := `SELECT name FROM ` + table
	var args []any
	if entityType != "" {
		q += ` UNION SELECT alias FROM entity_aliases WHERE entity_type = ?`
		args = append(args, entityType)
	}
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query names %s: %w", kind, err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// QueryProviders returns the providers any video is matched to: the names a
// `matched:` condition may use.
func (r *Repo) QueryProviders(ctx context.Context) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT DISTINCT provider FROM entity_enrichment WHERE entity_type = ? ORDER BY provider`, model.EnrichEntityVideo)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
