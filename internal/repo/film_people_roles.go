package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Film people roles (F56, ADR-085, HOLODEX-281): film-level billing/role data the
// owner enters directly on a film -- additive and separate from video_people
// (migration 0037), which stays per-video and is never touched by this file. See
// films.go's FilmCast doc comment for the read-only inherited-cast union this
// complements.

// FilmPersonRole is one credited role row on a film's detail page: a person plus
// their film-level role/billing_order (migration 0043's film_people_roles) --
// distinct from FilmCast's read-only inherited-cast union.
type FilmPersonRole struct {
	PersonID     int64  `json:"person_id"`
	PersonName   string `json:"person_name"`
	Role         string `json:"role"`
	BillingOrder *int64 `json:"billing_order,omitempty"`
}

// ErrFilmPersonAlreadyCredited is returned by AddFilmPersonRole when the person
// already has a credited role on this film. film_people_roles' PRIMARY KEY is
// (film_id, person_id, role), which would technically allow a person to hold
// several distinct roles on one film, but the API enforces one credited row per
// person per film -- matching the ticket's "add/edit/remove a person's film-level
// role" (singular) framing and letting EditFilmPersonRole freely rewrite the role
// text without an addressing scheme built around role strings (including the
// empty-role sentinel) in a URL. Use EditFilmPersonRole to change an existing row.
var ErrFilmPersonAlreadyCredited = errors.New("person already has a credited role on this film")

// FilmPeopleRoles returns a film's credited roles (migration 0043's
// film_people_roles), billing-order-first (NULLs last) then person name -- the
// film-level counterpart to FilmCast's read-only inherited union.
func (r *Repo) FilmPeopleRoles(ctx context.Context, filmID int64) ([]FilmPersonRole, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT fpr.person_id, p.name, fpr.role, fpr.billing_order
		FROM film_people_roles fpr JOIN people p ON p.id = fpr.person_id
		WHERE fpr.film_id = ?
		ORDER BY fpr.billing_order IS NULL, fpr.billing_order, p.name COLLATE NOCASE`, filmID)
	if err != nil {
		return nil, fmt.Errorf("film people roles: %w", err)
	}
	defer rows.Close()
	out := []FilmPersonRole{}
	for rows.Next() {
		var fpr FilmPersonRole
		var billing sql.NullInt64
		if err := rows.Scan(&fpr.PersonID, &fpr.PersonName, &fpr.Role, &billing); err != nil {
			return nil, err
		}
		if billing.Valid {
			fpr.BillingOrder = &billing.Int64
		}
		out = append(out, fpr)
	}
	return out, rows.Err()
}

// AddFilmPersonRole credits a person with a film-level role (owner-entered, F56
// ADR-085). Assumes the caller already validated the film/person exist (mirrors
// AttachFilmVideo's requireLiveFilmAndVideo pre-check at the API layer) --  an
// invalid id here surfaces as a foreign-key error, not ErrNotFound. Returns
// ErrFilmPersonAlreadyCredited if the person already has a row on this film.
func (r *Repo) AddFilmPersonRole(ctx context.Context, filmID, personID int64, role string, billingOrder *int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	var already int
	err := r.db.QueryRowContext(ctx,
		`SELECT 1 FROM film_people_roles WHERE film_id = ? AND person_id = ?`, filmID, personID).Scan(&already)
	switch {
	case err == nil:
		return ErrFilmPersonAlreadyCredited
	case !errors.Is(err, sql.ErrNoRows):
		return fmt.Errorf("check existing film person role: %w", err)
	}

	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO film_people_roles (film_id, person_id, role, billing_order) VALUES (?, ?, ?, ?)`,
		filmID, personID, role, billingOrder); err != nil {
		return fmt.Errorf("add film person role: %w", err)
	}
	return nil
}

// EditFilmPersonRole full-replaces an already-credited person's role text and
// billing_order. ErrNotFound if the person has no credited role on this film (use
// AddFilmPersonRole to create one).
func (r *Repo) EditFilmPersonRole(ctx context.Context, filmID, personID int64, role string, billingOrder *int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`UPDATE film_people_roles SET role = ?, billing_order = ? WHERE film_id = ? AND person_id = ?`,
		role, billingOrder, filmID, personID)
	if err != nil {
		return fmt.Errorf("edit film person role: %w", err)
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

// RemoveFilmPersonRole removes a person's credited role from a film. ErrNotFound if
// they had none (not idempotent, mirroring DetachFilmVideo).
func (r *Repo) RemoveFilmPersonRole(ctx context.Context, filmID, personID int64) error {
	r.writeMu.Lock()
	defer r.writeMu.Unlock()

	res, err := r.db.ExecContext(ctx,
		`DELETE FROM film_people_roles WHERE film_id = ? AND person_id = ?`, filmID, personID)
	if err != nil {
		return fmt.Errorf("remove film person role: %w", err)
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
