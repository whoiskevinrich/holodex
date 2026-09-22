---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-25
status: in-progress
release_note: Every index page now remembers the filters you last used — Media's filter set, People's and Studios' completeness sort and Missing facets, and Tags' type filter — the same way it already remembered your sort.
---

# HOLODEX-25 · Persist selected filters (sticky per page, localStorage)

Sort has had a memory since SP1 (`sortPreference.svelte.ts`); the filters beside it reset on
every mount — reload, or ← Back from a detail page — so an owner mid-curation re-picked
"Completeness ↓" + "Missing: birthdate" on every return. The ticket originally scoped this to
session-only + Media; the owner widened it (2026-09-21) to **localStorage** and **every index
page** (Media, People, Studios, Tags; Films has no filter controls). Spec'd as **SP5** in
`docs/specs/sort-persistence.md`, built as `filterPreference.ts`, a sibling of SP1's module.

## Gates — definition of done

- [x] spec `write-spec` — SP5 added to `docs/specs/sort-persistence.md` (storage, per-page shape, URL-wins precedence on Media, owner-only posture, acceptance criteria)
- [~] architecture `architecture` — n/a: client-only localStorage, same posture as SP1 (no ADR then either)
- [~] design `design-handoff` — n/a: no new surface; existing controls just remember their state
- [x] frontend — `web/src/lib/filterPreference.ts` + wiring in `routes/+page.svelte`, `people`, `studios`, `tags`
- [x] testing `testing-strategy` — `filterPreference.test.ts` (round-trip, per-page keys, malformed/rejected/unavailable storage, both validators); pages verified live in the dev server
- [~] security `security-review` — n/a: no auth/access change; owner-only filters are still stripped from the request for a non-owner
- [x] `code-review high --fix` — 2 findings, both fixed: (1) `replaceState` on a hard load of `/` ran before the router was initialized (deferred a macrotask); (2) a restored `completenessDir` leaked into the People/Studios shuffle/jump-nav conditions with Admin mode off (owner-gated `completeness` derived)

## Up next — ordered (position = priority)

1. [ ] [—] Owner-mode live check: sign in as owner, set Completeness ↓ + a Missing facet on /people, open a person, ← Back → both restored and scroll position kept
2. [ ] [—] Merge; HOLODEX-25 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-21 · scoped, built, reviewed, verified non-owner paths live
- skills: code-review high --fix
- Inventory: only People/Studios `completenessDir` + `missingFacetIDs`, Tags `typeFilter`, and
  Media's whole filter set were unpersisted; sort/view/density already were. Media stores its
  shareable query string (minus `q` and `sort`) so restore reuses `paramsToFilters`; URL
  wins whenever there is one, a bare `/` restores and then syncs the URL.
- Live in the dev server (non-owner): Tags type filter survives leave/return; Media
  resolution filter survives nav-away/back, a `?resolution=4K` deep link overrides the saved
  set, "Clear filters" persists empty, garbage in every key falls back cleanly, hard load of
  `/` with a saved set now loads and syncs the URL, ← Back from a video still hits the cache.
- handoff: code + spec + tests done; owner-mode live check outstanding (couldn't sign in from
  the agent session); PR to open.
