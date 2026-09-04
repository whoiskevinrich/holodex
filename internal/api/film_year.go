package api

import (
	"context"
	"strconv"
	"strings"

	"holodex/internal/repo"
)

// Film release-year fill (F59/ADR-089 D3).
//
// `release_date` is an ordinary film scalar that resolves through the standard
// ADR-051 decision grammar. `films.year` is NOT a resolved field -- it is the other
// half of the (name, year) identity key (migration 0043) -- so it is updated as a
// *consequence* of that resolution rather than being resolved itself.
//
// Two deliberate narrowings, both recorded because each looks like a missing feature:
//
//  1. **Fill-only, never overwrite** (enforced in repo.FillFilmYear). A film's year is
//     owner-asserted at creation; rewriting it from a provider would silently change an
//     entity's identity, and could not be undone on clear because no prior value is
//     stored. Filling a blank is additive and reversible-by-construction.
//  2. **The identity write is gated, not the enrich.** ADR-089 D3 originally said a
//     collision rejects the whole apply including the enrichment rows. That over-reached
//     into the shadow store, which ADR-033 makes deliberately additive and ungated -- by
//     the time release_date is readable, those rows exist. So the enrich lands normally
//     and only the identity column is withheld, with the occupant named. The invariant
//     that actually matters is preserved: (name, year) never duplicates, and the year
//     either changes completely or not at all.

// filmReleaseYear extracts a 4-digit year from a resolved release_date value.
// Accepts the contract's preferred "YYYY-MM-DD" and a bare "YYYY"; anything else
// yields 0, which FillFilmYear treats as "nothing to do".
func filmReleaseYear(value string) int {
	value = strings.TrimSpace(value)
	if len(value) < 4 {
		return 0
	}
	year, err := strconv.Atoi(value[:4])
	if err != nil || year <= 0 {
		return 0
	}
	return year
}

// syncFilmYear fills a film's year from its currently-resolved release_date. Called
// after any mutation that can change what release_date resolves to -- enrich apply,
// enrich clear, and the per-field decision set/clear -- so the identity column
// follows the *resolved* value rather than whatever one provider happened to send.
//
// Returns a non-nil collision when the fill was withheld because another film already
// holds (name, year). That is reported to the owner, not treated as a request failure:
// the surrounding mutation already succeeded and is not being rolled back.
func (h *Handlers) syncFilmYear(ctx context.Context, id int64) *repo.FilmYearCollision {
	film, err := h.repo.GetFilm(ctx, id)
	if err != nil {
		h.log.Warn("film year sync: load film", "id", id, "err", err)
		return nil
	}
	if film.Year > 0 {
		return nil // fill-only; skip the resolve entirely.
	}

	var year int
	for _, f := range h.resolveFilm(ctx, id, film) {
		if f.Canonical != "release_date" {
			continue
		}
		if len(f.Values) > 0 {
			year = filmReleaseYear(f.Values[0])
		}
		break
	}
	if year == 0 {
		return nil
	}

	collision, err := h.repo.FillFilmYear(ctx, id, year)
	if err != nil {
		h.log.Warn("film year sync: fill", "id", id, "year", year, "err", err)
		return nil
	}
	if collision != nil {
		h.log.Info("film year fill withheld: (name, year) already taken",
			"film_id", id, "year", year, "occupant_id", collision.FilmID)
	}
	return collision
}
