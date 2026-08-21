# Spec: Cast Billing Order + Role-Scoped Joined Queries (F46 Phase 3)

**Status**: Draft — implementation handoff (not yet built)
**Feature block**: F46 Phase 3
**Epic**: [HOLODEX-178](https://whoiskevinrich.atlassian.net/browse/HOLODEX-178) (F46: Queryable person/video attributes)
**Jira**: [HOLODEX-180](https://whoiskevinrich.atlassian.net/browse/HOLODEX-180)
**Depends on (hard prerequisite)**: [HOLODEX-114](https://whoiskevinrich.atlassian.net/browse/HOLODEX-114) / [ADR-059](../architecture/ADR-059-person-link-resolved-derivation.md) (F40) — must ship first. This spec assumes `video_people.role` and the `(video_id, person_id, role)` PK already exist, and that `RelinkVideoPeople` is the sole writer of that table.
**Relates to**:
- [HOLODEX-176](https://whoiskevinrich.atlassian.net/browse/HOLODEX-176) — closed **Won't Do** 2026-07-12: the generic F46 typed-field/query substrate was rejected as speculative infrastructure with zero shipped consumers. This spec follows that precedent — purpose-built, no registry.
- [docs/specs/age-in-media.md](age-in-media.md) (HOLODEX-173) — the precedent this spec follows: a narrow, bespoke join, not a generic substrate.
- [HOLODEX-102](https://whoiskevinrich.atlassian.net/browse/HOLODEX-102) / [HOLODEX-22](https://whoiskevinrich.atlassian.net/browse/HOLODEX-22) (F32, [docs/specs/video-credits-people.md](video-credits-people.md)) — headshots + `external_id` de-dup. F32's provider contract carries TMDB's `order` too, but only to cap the cast list at ingest; it never persists order as queryable data. No dependency either direction with this spec.

**Decision (resolved with owner, this session)**: purpose-built, no generic F46 registry; billing order stored raw with no hardcoded "lead" threshold; actors only (no crew billing-order/role joins this phase).

---

## Problem Statement

Holodex's TMDB provider already fetches each cast member's `order` (billing order) on every film enrich (`providers/tmdb/tmdb.go:207-215`, `movieCastEntry.Order`), but discards it after using it only to truncate the actor list to the top 10 (`tmdb.go:538-560`). Once flattened into a plain `[]string` under `fields.actors`, there is no way to tell a film's lead from a background extra. This blocks any query or UI surface that cares about billing prominence — e.g. "find every film where person X had a leading role" — and was the concrete example (`video_people` today is a bare `(video_id, person_id)` pair with no attributes) that motivated the original F46 "lead actor" query ask. The data literally doesn't exist in the schema; this is independent of the query-engine/registry question HOLODEX-176 already settled.

## Goals

- Persist TMDB's cast billing order onto the video↔person link so it survives past ingest, refreshed on every re-enrich.
- Enable role-scoped, joined video queries: "videos where person X holds role=actor within billing order ≤ N" — extending the existing person-filter EXISTS pattern, not inventing a new query DSL.
- Land as a thin layer on top of HOLODEX-114/ADR-059's `video_people.role`/PK work — add nothing that duplicates or re-decides that ADR.
- Keep "lead" semantics out of the schema/ingest layer: store the raw order, let callers decide thresholds.

## Non-Goals

- **A generic F46 typed-field/query substrate.** Rejected at [HOLODEX-176](https://whoiskevinrich.atlassian.net/browse/HOLODEX-176) (Won't Do, 2026-07-12) as speculative infrastructure with no shipped consumer. Age-in-Media (HOLODEX-173) already set the "narrow, purpose-built" precedent; this spec follows it.
- **Crew billing order / role-scoped joins for director, producer, etc.** TMDB crew credits carry a `job` string, not a billing `order` — there is no analogous "billing prominence" signal for crew. Actors only, this phase.
- **A hardcoded "lead" threshold.** No `is_lead` boolean, no `order == 0` special-casing in schema or ingest. Order is stored raw; any "lead" semantics are a caller-side query threshold, not baked in here.
- **Filtering videos by arbitrary person attributes (eye color, nationality, computed age) joined through role.** No person-attribute value index exists today — building one is exactly what HOLODEX-176 declined to do. This spec only extends the *existing* person-ID EXISTS-join pattern with role + order scoping. Resolving "which people match attribute X" into a person-ID set to feed into that filter is a separate, already-possible (or future) concern.
- **Implementing HOLODEX-114/ADR-059 itself** (role column, PK migration, `RelinkVideoPeople`, orphan sweep, owner-view link picker, studio parity). Hard prerequisite, tracked and shipped separately.
- **F32's headshot/external-id work** (HOLODEX-102/HOLODEX-22). Unrelated to billing order; no dependency either direction.

## User Stories

- As the owner browsing a film's video-detail page, I want the cast list to reflect real billing order (not scan/dedup insertion order), so the displayed order matches the film's actual credits.
- As the owner querying the library, I want to filter to "videos where person X appears as an actor, billed in the top N," so I can find every film where an actor had a leading role, distinct from a cameo.
- As a future API consumer (e.g. a person-page "starring roles" shelf), I want to query "videos where this person's billed order ≤ N," so that view can be built without re-deriving order client-side.
- As the owner re-enriching a film, I want billing order refreshed on each enrich (not stuck from an earlier version), so the data reflects TMDB's current credits.

## Requirements

### Must-Have (P0)

1. **Schema**: add a nullable `billed_order INTEGER` column to `video_people`, built on HOLODEX-114's `(video_id, person_id, role)` PK. Next migration after HOLODEX-114's, with a matching down migration.
   - AC: migration adds the column without altering the PK HOLODEX-114 established.
   - AC: down migration drops the column cleanly.
   - AC: rows with no billing-order signal (director role, owner-curated actor links with no provider data) have `billed_order = NULL`, not `0` — `0` is a real top-billing value and must not collide with "unknown."

2. **Provider**: the TMDB sidecar stops discarding `movieCastEntry.Order`; it threads order through the enrich response so `RelinkVideoPeople` (ADR-059) can set `billed_order` when deriving the `actor` role for each resolved person.
   - AC: re-enriching a film whose upstream cast order changed updates `billed_order` on the next `RelinkVideoPeople` pass — order is not sticky from a stale enrich.
   - AC: the existing top-10 actor-name truncation either widens to cover what's needed or is decoupled from order capture — truncating the flattened name list must not also silently truncate which entries carry a billing order.

3. **Query layer**: extend `internal/repo.VideoFilter` with a role + billed-order-scoped person join, following the existing `PersonIDs`/`PersonIDsAny` EXISTS-clause shape (`internal/repo/repo.go:391-403`) — e.g. `PersonIDsAny` gains an optional role filter and a billed-order-max bound, compiling to `EXISTS (SELECT 1 FROM video_people vp WHERE vp.video_id = v.id AND vp.person_id IN (...) AND vp.role = ? AND vp.billed_order <= ?)`.
   - AC: a query for "person X, role=actor, billed_order ≤ 0" returns only videos where X is literally top-billed.
   - AC: omitting the role/order bound preserves today's existing person-filter behavior unchanged (backward compatible).
   - AC: filtering by role alone (no order bound) works correctly for rows with `billed_order IS NULL`.

### Nice-to-Have (P1)

4. Surface billed order in the video-detail API response per cast member (e.g. `resolved.cast[].billed_order`) so the frontend can order/badge the cast list by real billing rather than insertion order. No UI badge design implied — just making the data available.
5. Expose the new filter through the existing video-list/search API as query parameters (e.g. `person_role=actor&person_billed_order_max=N`), rather than only on the internal `VideoFilter` struct.

### Future Considerations (P2)

6. A "starring roles" shelf on the person page, built on requirement 3/5.
7. Extending billing-order-style joined predicates to other relationship types, if a concrete second use case emerges. Explicitly not designed for speculatively here (see Non-Goals).

## Success Metrics

This is small, internal, single-owner infrastructure with no user-facing metric to track — success is binary: does a role+order-scoped query return correct results against the films testbed library, and does re-enriching update billing order without manual intervention. Verified via the testing strategy (below), not an ongoing metric.

## Open Questions

None outstanding. All three from the original ticket were resolved with the owner before drafting:

- **Generic substrate vs. purpose-built** → purpose-built, hard-dependent on HOLODEX-114 rather than re-implementing its decisions.
- **"Lead" definition** → deferred to caller/query-time threshold; raw order persisted, no hardcoded semantics in schema or ingest.
- **Crew scope** → actors only.

## Timeline Considerations

- **Hard dependency**: [HOLODEX-114](https://whoiskevinrich.atlassian.net/browse/HOLODEX-114) (F40/ADR-059) must merge first. This spec's schema step assumes `video_people.role` and its PK already exist, and hooks `billed_order` population into `RelinkVideoPeople` rather than writing `video_people` a second way. Implementation cannot start before HOLODEX-114 lands.
- No external deadline. Sequenced as F46 Phase 3, behind HOLODEX-114, in the Jira backlog.

## Gate status

- [x] Spec (`/write-spec`) — this document
- [ ] ADR (`/architecture`) — schema change (`billed_order` column) + provider contract change (TMDB order-threading) + query-engine join extension
- [ ] Testing strategy (`/testing-strategy`)
- [ ] Security review (`/security-review`) — provider-sourced order data now feeds query/filter results; confirm no injection surface in the new WHERE-clause parameterization

---

_Captured from a brainstorm on the original HOLODEX-176 ask — see epic HOLODEX-178 for full context. Scope and open questions resolved with the owner on 2026-07-13._
