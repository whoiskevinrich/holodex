---
key: HOLODEX-186
status: in-progress
depends-on: []
release_note: Enrichment now queues every un-enriched person, studio, and media entry in one place, applies obvious matches automatically, remembers a rejected candidate so it never re-prompts, and lets you refresh an already-linked source with one click.
---

# HOLODEX-186 · F47 — Enrichment review workflow (queue, confidence routing, unmatched flag, refresh)

A generalized, entity-agnostic enrichment review workflow across Person/Studio/Media: a review
queue for un-enriched entries, confidence-based auto-apply for unambiguous single-strong-match
candidates, a durable "not matched" verdict, an optional provider view-source link, and a refresh
bypass (single-provider and all-providers). **Done** = P0–P1 slices (S1–S6) shipped: queue live in
the Owner hub, `EnrichPicker`/`EnrichProviderChips` extended, provider contract amended, QA +
security clean.

**Design package:** [spec](../specs/enrichment-review-workflow.md) · [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md) · [handoff](../design/enrichment-review-workflow-handoff.md) · [testing-strategy](../testing-strategy.md)

## Gates — definition of done

- [x] spec `write-spec` → [enrichment-review-workflow.md](../specs/enrichment-review-workflow.md)
- [x] architecture `architecture` → [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md) — auto-apply-with-revert posture change + new `enrichment_dismissals` store
- [x] backend — S1 (`enrichment_dismissals` migration + dismiss/undismiss/refresh/refresh-all endpoints + `/owner/enrich-queue`) landed
- [x] frontend — [handoff](../design/enrichment-review-workflow-handoff.md) landed (Enrichment tab, `EnrichPicker`/`EnrichProviderChips` additions, Q3 resolved); S2 (Enrichment tab) + S3 (`EnrichPicker`) + S4 (`EnrichProviderChips`) all shipped
- [x] testing `testing-strategy` → [docs/testing-strategy.md](../testing-strategy.md) §4/§5/Phase 3 — written ahead of S1–S4, now exercised by S1's test suite
- [ ] security `security-review` — `profile_url` scheme validation is the one new externally-influenced surface (backend not yet emitting the field — see S3 note; nothing to review server-side until it lands)

## Up next — ordered (position = priority)

1. [x] [backend] S1 data model + endpoints — `enrichment_dismissals` migration, `dismiss`/`undismiss`/`refresh`/`refresh-all` routes, `GET /owner/enrich-queue` — `internal/api`, `internal/db/migrations`
2. [x] [frontend] S2 Enrichment tab — `owner/enrichment/+page.svelte`, `EnrichQueueRow.svelte`, `ProviderStatusChip.svelte` — `web/src/routes/owner`
3. [x] [frontend] S3 `EnrichPicker`: auto-apply on single strong match, "None of these match", view-source link — `web/src/lib/components/EnrichPicker.svelte`
4. [x] [frontend] S4 `EnrichProviderChips`: Refresh primary action + Refresh-all — `web/src/lib/components/EnrichProviderChips.svelte`
5. [x] [—] S5 provider contract doc amendment (`profile_url`, auto-apply posture, Q4) — `docs/specs/metadata-provider-contract.md`
6. [ ] [testing] S6 QA (3-skin) + `/security-review` (`profile_url` scheme validation)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-07-13 · S5 — provider contract doc amendment
- docs: [`docs/specs/metadata-provider-contract.md`](../specs/metadata-provider-contract.md) §2.3
  amended per [ADR-066](../architecture/ADR-066-enrichment-auto-apply-and-dismissal.md) D1's
  action item: replaced the stale "Holodex always shows the owner a picker and never
  auto-applies... `confidence` is advisory" posture with a callout documenting the
  threshold-gated auto-apply behavior (a lone `>=0.85` candidate applies with no picker; any
  other outcome still stops at the owner) and stating explicitly that this is
  **documentation-only** — `candidates[].confidence`'s wire shape/range/semantics are unchanged,
  no `protocol_version` bump (closes Q4). Added the `candidates[].profile_url` field (RD6/P1-1)
  to the `/resolve` response table and both worked examples (§2.3 and §8) — optional, `http`/
  `https` only, silently dropped server-side otherwise, matching the backend's existing
  `sanitizeProfileURL` (already shipped in S1/S3, `internal/enrich/service.go`). Resolved the
  "Open items" `confidence` semantics note (it previously said Holodex never thresholds on it —
  now documents that it does, and what a provider whose scores run high near 0.85 should expect).
  Checked off ADR-066's action items 1–7 (all had already landed in S1–S4/design-handoff/testing-
  strategy but the ADR's own checklist was never updated to reflect it) — left item 8
  (`/security-review`) unchecked, that's S6. Also flipped spec Q4 to resolved with a pointer to
  the shipped section.
- verified: doc-only change, no code touched — read-through against the actual shipped
  `internal/enrich.Candidate`/`StrongMatchThreshold`/`sanitizeProfileURL` (`internal/enrich/
  enrich.go`, `service.go`) to confirm every claim in the amendment matches real behavior,
  including the `auto_apply` field server-computed in `sanitizeCandidates` (b05b4e9) — confirmed
  that field is Holodex's own owner-facing API response shape, not part of what a provider's
  `/resolve` endpoint must return, so it correctly stays out of this provider-facing contract.
- next: S6 — 3-skin QA (Cinémathèque/Broadcast/Brutalist) of the full F47 surface (queue tab,
  auto-apply, dismiss/try-again, refresh/refresh-all) + `/security-review` focused on
  `profile_url` scheme validation and the new dismiss/undismiss/refresh/refresh-all mutations'
  `requireOwner` gating — the last gate before HOLODEX-186 is Done.

### 2026-07-13 · PR #130 merge-conflict resolution + S4 frontend — EnrichProviderChips

- **Merge-conflict resolution (PR #130 vs. main):** an automated CI event flagged merge
  conflicts against `main`. Two things collided: (1) `internal/enrich.Candidate` and
  `sanitizeCandidates` — this branch's `AutoApply` (a separately spawned background task,
  `task_6d29b71d`) and main's `ProfileURL` (`task_886c73ca`, merged as PR #140) both landed on
  the same struct/function independently; kept both fields, they're unrelated. (2) an **ADR
  numbering collision** — this branch's "Enrichment auto-apply-with-revert" ADR and an
  already-merged, now-Superseded main ADR ("Typed field registry…") both claimed **ADR-065**.
  Renumbered this branch's to **ADR-066** (main's was established first) across 23 files
  (`git mv` the ADR file, bulk `ADR-065`→`ADR-066` rename in Go/TS comments, the migration
  comment, specs, this plan, PR title/body) — careful to exclude the *other*, unrelated
  ADR-065 (queryable-fields-substrate) and its spec, which stay ADR-065. Verified clean:
  `go build ./...`, full `go test ./...`, `npm run check` (0 errors), `npm run test` (103
  passed), token guard empty. Committed (`bbbd586`), pushed; PR #130's CI (`analyze
  go/js-ts`, `backend`, `frontend`, `theming`, CodeQL) all passed on the re-run.
- **S4 frontend:** `EnrichProviderChips.svelte` — the flipped primary action once linked
  (RD7/P0-5) and Refresh-all (RD8/P1-2) per the [handoff](../design/enrichment-review-workflow-handoff.md)
  §3. Discovered mid-implementation that the design doc's proposed richer `status(p)` prop
  (to carry a stored `external_id` for a direct client-side `apply()`) is unnecessary — S1's
  backend already shipped `POST .../enrich/{provider}/refresh` (`enrichRefresh`,
  `internal/api/enrich_review.go`) which re-derives the external_id **server-side** via
  `enrich.ExistingMatch`, and `POST .../enrich/refresh-all` (`enrichRefreshAll`) which fans
  out entirely server-side (linked → refresh directly, unlinked → resolve-and-route,
  auto-apply a single strong match or `needs_review`). So the chip component stays exactly as
  simple as before: `linked(p)` still gates "Refresh" vs. "Enrich", but the click just calls a
  new `onrefresh`/`onrefreshall` prop with no id plumbing. Kept `onenrich` as the single
  picker-opening callback for both the unlinked primary click *and* the new "Re-match…" ⋯ menu
  item (RD7: re-match is explicitly "today's Enrich behavior, relabeled" — no separate prop
  needed). "Refresh all" renders as a trailing sibling inside the *same* `flex flex-wrap` chip
  row (not a new container), per the handoff's placement note. On a `needs_review` result from
  refresh-all, the caller opens `EnrichPicker` for the first such provider (never silently
  dropped, RD8) — reusing the existing single `pickerProvider` state already on all three
  detail pages. `web/src/lib/api.ts` gained `enrichRefresh`/`enrichRefreshAll`; `types.ts`
  gained `RefreshAllResult`. Wired into all three call sites (`people`/`studios`/`media`
  `[id]/+page.svelte`), each adding a `refreshProvider`/`refreshAll` pair mirroring the
  existing `clearProvider` busy/error convention (media page uses `enrichRefreshingAll` to
  avoid colliding with its unrelated F31 file-refresh `refreshing` state).
- verified: `npm run check` (0 errors), `npm run test` (103 passed), token guard empty. Live
  QA against `provider-tmdb` + `backend-films` (real TMDB API): enriched Dune (2021, id 209)
  with tmdb, confirmed the chip flips to "tmdb / Refresh / ⋯", the ⋯ menu shows "Re-match…" +
  "Clear tmdb data", clicking "Refresh" fires `POST .../enrich/tmdb/refresh` (200, page
  reloads cleanly, no console errors), and "Refresh all" fires `POST
  .../enrich/refresh-all` (200). 3-skin visual QA not completed this session — the preview
  pane's screenshot capture timed out repeatedly (environment issue, not code); markup/wiring
  verified via the accessibility tree and network requests instead. Full 3-skin screenshot QA
  is S6's job per the plan regardless.
- note: Jira `In Progress` still not fired for HOLODEX-186 (no local `JIRA_*` creds this
  session; the Atlassian MCP connector needs authorization — flagged to the user, not
  actionable from here).
- next: S5 provider contract doc amendment (`profile_url`, auto-apply posture, Q4) —
  `docs/specs/metadata-provider-contract.md` — then S6 (3-skin QA + `/security-review`).

### 2026-07-12 · S3 frontend — EnrichPicker additions
- skills: simplify
- frontend: `web/src/lib/components/EnrichPicker.svelte` — three additions per the handoff §2.
  **Auto-apply (RD1):** the picker's pre-existing initial auto-search (seeded with the entity's
  own name) now, on a lone `>=0.85` candidate, calls `confirm()` itself instead of rendering the
  list — a request-generation counter (`searchId`) guards against a slow auto-search's response
  clobbering a newer, user-typed search or auto-applying a stale match; a manually typed search
  never auto-applies. **"None of these match" (RD4):** a new `dismiss`/`ondismissed` prop pair
  (mirrors `apply`/`onapplied`); `noMatch()` calls `POST .../enrich/{provider}/dismiss`, closes,
  and reports up so the caller can flip to `not_matched` without a refetch — visible once
  candidates exist, alongside the "Enriching…" status line. **View-source link (RD6/P1-1):** an
  optional `EnrichCandidate.profile_url` (new in `types.ts`) renders as "view source ↗", gated
  through `isHttpUrl()` (`format.ts`'s standing guard for any provider-supplied `href`, matching
  `UrlValueList`'s convention) even though the field is also scheme-validated server-side —
  belt-and-suspenders, not redundant. `web/src/lib/api.ts` gained `enrichDismiss` (mirrors the
  S2 `enrichUndismiss` shape). Wired `dismiss` through `EnrichQueueRow.svelte` (→ `not_matched`
  on dismiss, alongside S2's existing `undismiss`/"Try again") and all three detail pages
  (`people`/`studios`/`media` `[id]/+page.svelte`) so "None of these match" works everywhere the
  picker opens, not just the queue — `ondismissed` is optional (the detail pages have no
  per-provider dismissal UI of their own, so they omit it; only `EnrichQueueRow` supplies one).
- note: `profile_url` is frontend-only for now — `internal/enrich.Candidate` has no backend field
  yet, so the link never renders against real data until that lands (spawned as a background
  task, `task_886c73ca`, not blocking S3: the type/render code is inert-safe in the meantime,
  matching the handoff's own "absent → nothing rendered" degrade path). Also spawned
  `task_6d29b71d` — the `0.85` strong-match threshold is now declared independently in
  `internal/enrich.StrongMatchThreshold` (Go) and `EnrichPicker`'s `STRONG_MATCH` (TS), each with
  a "must never drift" comment; a server-computed `auto_apply` verdict on `/resolve` would
  collapse this to one source of truth — not fixed here since it's a `/resolve` response-shape
  change spanning 3 backend routes, out of this diff's scope.
- verified: `npm run check` (0 errors), `npm run test` (103 passed), token guard empty. Live QA
  against `backend-amv` + `provider-tmdb` (real TMDB API): confirmed all three behaviors
  end-to-end — "Alyssa Milano" resolved to one strong match and auto-applied with no picker
  shown; "Alan Smithee" (a pseudonym, correctly ambiguous — 10 candidates) opened the picker,
  "None of these match" flipped the chip to "Not matched" (row action → "Try again"), and
  "Try again" re-resolved fresh. 3-skin QA not run this session (S6's job per the plan).
- next: S4 `EnrichProviderChips` — Refresh primary action once linked, Re-match/Clear into the
  ⋯ overflow, Refresh-all.

### 2026-07-12 · S2 frontend — Enrichment tab
- skills: simplify
- frontend: `web/src/routes/owner/enrichment/+page.svelte` (new) — the entity-generic sibling of
  `owner/duplicates/+page.svelte`: `$state` rows loaded once from `GET /owner/enrich-queue`
  (zero-cost, no provider calls per RD2/RD3), grouped People → Studios → Media (spec Q3),
  actionable-first sort within a group (an `unreviewed` provider sorts above an all-`not_matched`
  row, even though the latter still shows "Try again"). Rows update chips in place on resolve
  instead of dropping out (handoff's Animation/Motion table) via an `onchange` callback the row
  bubbles up. `web/src/lib/components/EnrichQueueRow.svelte` (new) — dense row mirroring
  `DuplicatePairRow`'s rhythm; "Review" opens the *existing, unmodified* `EnrichPicker` for the
  row's next outstanding provider (auto-apply-on-single-strong-match is S3's job, inside
  `EnrichPicker` itself, so this row inherits it for free once S3 lands); "Try again" clears a
  durable `not_matched` dismissal (new `enrichUndismiss` API method) and reopens the picker.
  `web/src/lib/components/ProviderStatusChip.svelte` (new) — read-only sibling of
  `EnrichProviderChips`' chip shell, no button/menu since the row action drives resolution.
  `owner/+layout.svelte` — added the tab entry. `types.ts`/`api.ts` — `EnrichEntityKind`
  (person|studio|video, distinct from F43's `EntityKind` which has no video),
  `EnrichQueueRow`/`EnrichQueueProviderState`, `api.enrichQueue()`/`api.enrichUndismiss()`.
- note: kept "has the owner opened this provider's picker" as local-only UI state
  (`EnrichQueueRow`'s `reviewed` set, mirroring `DuplicatePairRow`'s local `choosing`) rather than
  a fourth `EnrichQueueProviderState.state` value — a `/simplify` altitude finding caught an
  earlier draft smuggling ephemeral "opened but not yet resolved" state into the synced domain
  enum; ADR-066's real states stay `unreviewed | not_matched | auto_applied`.
- verified: `npm run check` (0 errors), `npm run test` (103 passed), token guard
  (`rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)'`) empty. Live 3-skin browser QA not run
  this session — the pinned :5173/:7800 dev-server ports were held by another session's worktree;
  full QA is S6's job per the plan anyway.
- deferred: flagged (not fixed, out of this slice's diff scope) — `owner/duplicates/+page.svelte`
  and this page's `groups` derivation duplicate the same group-by-kind shape; a shared helper is
  worth extracting later (spawned as a background task, not blocking S2).
- next: S3 `EnrichPicker` — auto-apply on single strong match, "None of these match" (new
  `ondismissed` event + `POST .../dismiss`), view-source link.

### 2026-07-12 · S1 backend — data model + endpoints
- skills: simplify
- backend: `internal/db/migrations/0024_enrichment_dismissals.{up,down}.sql` — the `enrichment_dismissals`
  table (D2) with real `AFTER DELETE` cascade triggers on `people`/`studios`/`videos` (stronger than
  `entity_enrichment`'s manual per-merge cleanup, since this table is new). `internal/repo/enrichment_dismissals.go`
  (Dismiss/Undismiss/EnrichmentDismissed) and `internal/repo/enrich_queue.go` (`EnrichQueue` — the
  zero-cost, batched, no-N+1 membership query: an entity qualifies only when it has ≥1 outstanding
  provider that is NOT dismissed; People → Studios → Media order per Q3). `internal/api/enrich_review.go`
  — `GET /owner/enrich-queue` and the entity-generic dismiss/undismiss/refresh/refresh-all handler
  factories (mirroring `enrich.go`'s resolve/apply/clear route shape, mounted in `mountEnrich`); wired a
  dismissal check into all three existing `/enrich/resolve` handlers (409 while dismissed, per RD4's
  block-until-"Try again" invariant). `internal/enrich`: `StrongMatchThreshold`/`SingleStrongMatch` (D1's
  reusable threshold primitive, mirroring `EnrichPicker.matchLabel`'s 0.85 cutoff) — used by
  refresh-all's server-side per-provider routing (refresh linked / auto-apply a single strong match /
  surface `needs_review` / skip a dismissed unlinked provider). Full test coverage: repo (membership/
  states/ordering/cascade), enrich (threshold boundary table), api (queue listing zero-provider-calls,
  dismiss blocks resolve + undismiss clears it, refresh 400-when-unlinked + zero-resolve-calls,
  refresh-all auto-apply + dismissed-skip) — all green, no regressions (`go test ./...`).
- note: D1's *client-triggered* auto-apply (opening a queue row auto-applies via the existing
  `/resolve`+`/enrich` calls, no new endpoint) is S3 frontend work, not this slice — only refresh-all
  needed the threshold check server-side (it's one atomic backend call fanning out over providers).
- next: S2 Enrichment tab (frontend) — `/owner/enrich-queue` is live and tested, unblocking S2–S4.

### 2026-07-12 · `/design-handoff` → Enrichment tab, picker + chip additions
- skills: design-handoff
- handoff: Wrote [enrichment-review-workflow-handoff.md](../design/enrichment-review-workflow-handoff.md)
  against the real `EnrichPicker`/`EnrichProviderChips`/`DuplicatePairRow`/`owner/+layout` source and
  the actual token values (no new tokens/fonts/radii introduced). Resolved spec Q3: queue groups by
  entity type in nav order (People → Studios → Media, not Duplicates' frequency-driven tags-first),
  actionable rows sort first within a group. Cross-referenced the handoff into the spec (pending-pointer
  + Timeline checkbox + Q3) and flipped Jira HOLODEX-186's Design handoff gate + cleared `needs-design`.
  Next: S1 backend (data model + endpoints) unblocks S2–S4 frontend build; Person's Refresh/Refresh-all
  scope still depends on HOLODEX-125 (ADR-055 gap) per the handoff's open question 3.
