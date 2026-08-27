---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-281                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Films now support crediting a person's film-level role (e.g. director, billing order), separate from the cast automatically inherited from the film's scenes.
---

# HOLODEX-281 · film_people_roles CRUD (film-level billing/role data)

Owner-gated CRUD API for `film_people_roles` (F56, ADR-085): add/edit/remove a person's film-level
role independently of that person's per-video `video_people` role, and surface it in `getFilm`'s
response distinct from the read-only inherited cast union.

**Design package:** [docs/specs/films-entity.md](../specs/films-entity.md) · [ADR-085](../architecture/ADR-085-films-entity.md)

## Gates — definition of done

- [x] spec `write-spec` → satisfied by pre-existing `docs/specs/films-entity.md` (F56); no update needed, this ticket is API-surface-only over an already-specified table (migration 0043)
- [x] architecture `architecture` → satisfied by pre-existing ADR-085; no new decision, no new migration
- [~] design `design-handoff` — until: a frontend surface for credited roles is built (this ticket is backend-only)
- [x] backend → `internal/api/film_people_roles.go`, `internal/repo/film_people_roles.go`
- [~] frontend — until: a follow-up ticket wires credited_roles into the film detail page UI
- [x] testing `testing-strategy` → `internal/repo/film_people_roles_test.go`, `internal/api/film_people_roles_test.go` (add/edit/remove + inherited-cast-vs-credited-role distinction), plus a dedicated `docs/testing-strategy.md` bullet resolving HOLODEX-279's deferred-item-(4) reference to this ticket
- [x] security `security-review` → clean; owner-gated (`requireOwner` group), parameterized SQL, existence-checked before mutation, no new data exposure

## Up next — ordered (position = priority)

1. [ ] [frontend] Wire `credited_roles` into the film detail page UI (billing-order display, add/edit/remove affordance) → follow-up ticket, out of this ticket's scope

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-27 · Implemented film_people_roles CRUD
- skills: simplify, security-review
- handoff: Built owner-gated CRUD for `film_people_roles` (add/edit/remove a film-level person role, distinct from `video_people`): `internal/repo/film_people_roles.go` (FilmPeopleRoles/AddFilmPersonRole/EditFilmPersonRole/RemoveFilmPersonRole, `ErrFilmPersonAlreadyCredited` sentinel) and `internal/api/film_people_roles.go` (`POST`/`PUT`/`DELETE /films/{filmId}/roles...`, mirroring `film_videos.go`'s attach/detach shape). Deliberately constrained to one credited role per person per film — addressed by `(filmId, personId)`, not by the schema's full `(film_id, person_id, role)` composite key — so the role text stays freely editable via PUT rather than needing a role-string-in-URL addressing scheme (documented in `ErrFilmPersonAlreadyCredited`'s doc comment). Wired `credited_roles` into `getFilm`'s response alongside the existing read-only `cast`. Added `internal/repo/film_people_roles_test.go` and `internal/api/film_people_roles_test.go` covering add/edit/remove, 409-on-redundant-add, 404-on-uncredited-edit/remove, and confirming `cast` stays untouched by crediting. Ran `/simplify`: 4 parallel review agents found one real issue (a hand-rolled `personIDByName` test helper duplicating the existing `r.PersonIDByName`), fixed; skipped a Tx-wrapping suggestion for `AddFilmPersonRole` after confirming `writeMu` already makes the check-then-insert atomic (no real bug, would be pure churn). Ran `/security-review` (new mutation surface, following the epic's precedent of reviewing every new mutation surface): confirmed owner-gated via the existing `requireOwner` group, parameterized SQL throughout, existence-checked before mutation, no new data exposure — clean, no findings. `go build`/`go vet`/`go test ./...` clean across all packages. Committed, pushed, opened [PR #262](https://github.com/whoiskevinrich/holodex/pull/262). Ran `/code-review high --fix`: 8 finder angles → verified 4 candidates. Fixed one real gap — this epic's own precedent (HOLODEX-280/282/285 all updated `docs/testing-strategy.md` when closing their testing gate; this ticket hadn't) — added a dedicated HOLODEX-281 bullet to `docs/testing-strategy.md` resolving the HOLODEX-279 epic bullet's deferred-item-(4) reference. Two correctness candidates verified PLAUSIBLE but left unfixed as documented/intentional or out-of-scope: `editFilmPersonRole`'s full-replace PUT silently nulls `billing_order` when a caller omits it (explicitly documented full-replace semantics, matches the ticket's own addressing-scheme rationale — flagging for whoever builds the frontend follow-up to send both fields together); and the shared `ErrNotFound` 404 body ("film or person not found") is technically misleading for an edit/remove against a real film+person who simply isn't credited yet, but this exactly mirrors `film_videos.go`'s pre-existing `filmVideoMutationError`/`detachFilmVideo` wording, so fixing it here alone would diverge from established convention rather than fix it consistently. A PK/schema-mismatch candidate ((film_id, person_id, role) key vs. app-only-enforced (film_id, person_id) uniqueness) was investigated and refuted: `AddFilmPersonRole` is the sole writer and `writeMu` fully serializes it, so `EditFilmPersonRole`/`RemoveFilmPersonRole`'s role-omitting WHERE clause can provably never match more than one row today. Backend + testing + security gates closed; design/frontend deferred to a follow-up ticket since this was scoped as API-surface-only.
