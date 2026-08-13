---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-275                 # the tracker key; must match the branch key regex
status: in-review                 # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Fix — API list endpoints (facets, media, people, search) now return an empty array instead of null when there's nothing to show, so the browse page and search no longer crash on a sparse or empty library.
---

# HOLODEX-275 · GET /api/v1/facets marshals empty values as null, not []

Bug fix: `Repo.FacetValues`/`ListVideos`/`ListPeople`/`Search` declared bare `var out []T` (or
left a zero-value `SearchResult` field unset) and returned it straight to a JSON handler on a
zero-row result, marshaling as `null` instead of `[]` — the same bug class as the HOLODEX-270
`VideoCollision.People` precedent (commit 90a691f). Found live browser-QA'ing HOLODEX-272:
`GET /api/v1/facets` returning `null` for `genres`/`director`/`collection` crashed
`MappedFacets.svelte`'s unguarded `.length` read and tore down SvelteKit hydration app-wide.

**Design package:** none (bug fix, no spec/ADR/design churn) · `docs/testing-strategy.md` §11
Known Gaps & Open Questions

## Gates — definition of done

- [~] spec `write-spec` — not applicable; bug fix with no requirement/scope change
- [~] architecture `architecture` — not applicable; no data-model/seam change, existing query shapes only
- [~] design `design-handoff` — not applicable; no new markup, no visual change (a frontend guard already shipped separately on HOLODEX-272)
- [x] backend
- [~] frontend — not applicable to *this* fix; the frontend guard (`facet.values?.length`) already shipped as an incidental fix in HOLODEX-272 (commit 406d4e5, PR #235)
- [x] testing `testing-strategy`
- [~] security `security-review` — not applicable; no auth/access/mutation surface touched, read-only query-shape fix only

## Up next — ordered (position = priority)

1. [x] [backend] `Repo.FacetValues`/`ListVideos`/`ListPeople`/`Search` — explicit `[]T{}` init instead of a bare `var` — `internal/repo/repo.go`
2. [x] [testing] `TestNilSliceRegressions` + testing-strategy.md update — `internal/repo/repo_test.go`, `docs/testing-strategy.md`
3. [x] [—] audit the rest of `internal/api`/`internal/repo` for the same pattern (per the ticket's own ask) — **correction below: the audit was incomplete, four more instances found and fixed in a follow-up pass**
4. [x] [—] push, open PR (#236), sync Jira
5. [x] [backend] `/code-review xhigh` follow-up pass — `ListTags`, `ListStudios`, `videoMetadata`/`GetVideo`, and `GET /media/{id}`'s `studios` field fixed; `ListAllVideos` fixed (latent); cache-hit staleness guard added; capacity hints added to `ListVideos`/`Search`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-08-12 · Root-caused and fixed the nil-slice marshaling bug, audited for siblings
- skills: testing-strategy, graphify, simplify
- handoff: fixed at the shared repo layer (`internal/repo/repo.go`) rather than per-handler —
  `FacetValues`, `ListVideos`, `ListPeople`, and all four `SearchResult` fields (including the
  empty-query early return) now always return a non-nil slice, so every caller (`facets`,
  `GET /media`, `GET /people`, `GET /search`, the person/tag detail video lists) is covered by
  one change. New `TestNilSliceRegressions` asserts all four zero-result paths marshal as `[]`.
  Delegated an Explore-agent audit of the rest of `internal/api`/`internal/repo` for the same
  pattern (the ticket's own ask): five real instances found and fixed here (the four above); the
  rest were already guarded (`enrichQueue`/`extractionQueue`/`adminActivityHistory`'s explicit
  `if rows == nil` checks, `RelatedShelf.Items`'s `make([]T, 0, limit)`, `VideoCollision.People`/
  `.Studios`'s HOLODEX-270/271 precedent) or not reachable empty (config-sourced fields,
  request-decode targets, an internal-only helper never serialized directly). `go build ./...`
  and `go test ./...` both clean. Branched off `origin/main` in the HOLODEX-272 worktree (not a
  new worktree — this is a small, unrelated fix, not "worktree" work) since PR #235 was already
  marked ready for review and shouldn't take unrelated commits. Next: push, open the PR, sync
  Jira.

### 2026-08-12 · `/code-review xhigh` follow-up pass, applied
- skills: code-review (xhigh), simplify
- handoff: the prior session's Explore-agent audit missed four more instances of the same
  nil-slice-to-`null` pattern: `Repo.ListTags`, `Repo.ListStudios`, `Repo.videoMetadata`
  (backing `GET /media/{id}`'s `metadata` field), and `GET /media/{id}`'s `studios` field (a
  `StudiosForVideos` map-lookup miss — nil for any video with no studio link, more broadly
  reachable than any case this PR originally fixed). Fixed all four the same way as the
  original four. Also fixed `Repo.ListAllVideos` (same pattern, currently latent — no live
  caller serializes it directly, but fixing for consistency), added a capacity hint
  (`make(..., 0, limit)`) to `ListVideos` and `Search`'s four result fields (~7x fewer
  allocations, benchmarked), and closed a cache-staleness landmine in `facetValues`'s
  cache-hit path where `json.Unmarshal("null", &v)` succeeds but leaves `v` nil, silently
  bypassing the fix for a stale pre-fix cache entry — dormant today (`cache.New()` only
  returns `Noop{}`, ADR-022) but would activate the moment a real cache backend lands.
  Extended `TestNilSliceRegressions` to cover all of the above at the Go-value level, plus one
  `json.Marshal`-based wire-level assertion — the prior test only checked in-memory nilness,
  which can't distinguish a regression that reintroduces literal JSON `null` specifically at
  encoding time. Corrected this worklog's and `docs/testing-strategy.md`'s prior "no further
  instances found" claim, since it wasn't true. `go build ./...` and
  `go test ./internal/repo/... ./internal/api/...` both clean. Did not attempt: a static
  lint/CI guard for the `var out []T` pattern (a real gap, but the right form — a custom
  `go/analysis` check vs. a grep+allowlist — needs a design decision beyond a same-pass fix)
  and amending the prior two commits to satisfy the "worklog in the same commit"/"log
  `/simplify`" conventions (both are already-pushed history; not rewriting it without being
  asked). This pass's own worklog update ships in the same commit as its code changes.
  Ran `/simplify` on the applied diff before committing: found and fixed two real efficiency
  bugs the capacity-hint change itself introduced in `Repo.Search` — a `Videos:
  make([]model.Video, 0, limit)` field that was always immediately discarded (overwritten by
  `res.Videos = titleVids`, itself already non-nil from the fixed `ListVideos`), and the
  `limit`/struct-literal reordering causing the empty-query fast path to heap-allocate four
  `limit`-capacity backing arrays it never used to need. Fixed by dropping the dead `Videos`
  field and moving the empty-query early return before the capacity-hinted allocations.
  Everything else across the 4 review angles was either already-conventional (matches
  existing `[]T{}`/`make(..., 0, n)` and `?? []` idioms elsewhere in the codebase) or an
  out-of-scope design call (a shared `nonNilSlice[T]` helper, a table-driven test refactor)
  noted but not taken. `go build ./...` and `go test ./internal/repo/... ./internal/api/...`
  both clean after the fix.
