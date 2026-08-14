---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-266                 # the tracker key; must match the branch key regex
status: in-progress               # todo | in-progress | in-review | done | released (coarse; mirrors Jira)
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: Person and studio pages now show a clickable provider badge (e.g. "IMDb") linking out to the source, matching the video badge from F55.
---

# HOLODEX-266 · Provider link badge — extend namespace-qualified display to person/studio

Extends the F55/ADR-082 provider-badge display (clickable provider-name badge, inline in the
entity header's metadata row, linking out to the third-party source) from video to person and
studio detail pages — done when a person/studio with a stored external id shows the same badge
video already gets, without touching completeness scoring or the ADR-054/055 identity model.

**Design package:** [ADR-083](../architecture/ADR-083-provider-link-badge-person-studio.md)
(extends [ADR-082](../architecture/ADR-082-external-provider-id-namespace-qualified-value.md))

## Gates — definition of done

<!-- Keyed to flightplan.yaml `gates`. States: [ ] not started · [/] in progress · [~] deferred · [x] done.
     PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY /handoff sets [x]. -->

- [x] architecture `architecture` → `docs/architecture/ADR-083-provider-link-badge-person-studio.md`
- [x] design `design-handoff` → `docs/design/provider-link-badge-handoff.md`
- [x] backend
- [x] frontend
- [x] testing `testing-strategy`
- [x] security `security-review`

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced verbatim in the SessionStart banner. -->

1. [x] [architecture] ADR: read-only projection (D1), provider-declared `link_templates` resolved
   server-side (D2), one badge per stored id (D3) — `docs/architecture/ADR-083-provider-link-badge-person-studio.md`
2. [x] [design] design-handoff note covering the multi-badge and no-link-degradation states —
   `docs/design/provider-link-badge-handoff.md`
3. [x] [backend] `Manifest.LinkTemplates map[string]map[string]string` (`internal/enrich/enrich.go`)
   + sanitize/validate on `/describe` ingest + `BuildProviderLink(namespace, entityKind, id)` helper
4. [x] [backend] person/studio detail handlers project `person_external_ids`/`studio_external_ids`
   into `external_links: [{provider, label, url}]` via the new helper
5. [x] [frontend] extract the video badge into a shared component; person/studio headers render
   zero-to-many badges from `external_links`
6. [x] [testing] template-mismatch degradation + multi-badge cases
7. [x] [security] `LinkTemplates` validation (single `{id}` placeholder, `https://` scheme, no
   injection via a malicious provider) before interpolation into a served URL

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-08-09 · Post-review hardening — high-effort /code-review pass, 6 fixes applied

- skills: code-review, graphify
- handoff: ran `/code-review high` (8 finder angles + verify pass) over the full branch diff
  after all 6 gates closed; PR #226 was already marked ready. 8 findings survived verification.
  Fixed 6 directly in `internal/api/external_links.go` and `internal/enrich/service.go`:
  `externalLinksForEntity` now lowercases the namespace before building each badge and dedupes by
  namespace, closing the crash where a stale namespace-uniqueness gap in `identity_ops.go`'s merge
  or an uncased provider-emitted id produced two badges with the same key and Svelte's keyed
  `{#each}` threw; `namespaceLabel`'s title-case fallback is now rune-safe (`utf8.DecodeRuneInString`
  + `unicode.ToUpper`) instead of byte-slicing, so a non-ASCII-first-character namespace no longer
  mangles into a replacement-character label; `persistLinkTemplates`'s write-then-reload is now
  serialized under a new `linkTemplatesMu` so two providers' concurrent `/describe` calls (the
  "Refresh All" fan-out) can't interleave their reloads and drop one provider's templates from the
  cache; `reloadLinkTemplates` now keeps the last known-good cache on a transient DB-read error
  instead of wiping every provider's badges to the degraded state; renamed `ExternalLink.Provider`
  to `Namespace` (wire field stays `"provider"` — no frontend/API change) since it never held the
  enriching provider. Left 2 findings unfixed as out-of-scope-for-minimal-edit, both already
  self-acknowledged as deferred earlier in this worklog: `namespaceLabels`' hardcoded 2-entry map
  (needs a config mechanism) and `provider_link_templates`' lack of a prune/reconciliation path for
  a removed provider (needs an admin surface or startup job). `go build`/`vet`/tests all green.
  Next: commit and push this fix-up onto the already-ready PR #226; no gate reopens since these are
  post-review hardening on already-closed gates, not new scope.

### 2026-08-09 · Security gate closed — LinkTemplates injection review, no findings
- skills: security-review, graphify
- handoff: closed item #7, the last gate. Reviewed the full malicious-provider data flow —
  `/describe` response → `SanitizeLinkTemplates`/`ValidateLinkTemplate` (`internal/enrich/
  link_templates.go`) → `persistLinkTemplates`/`BuildProviderLink` (`internal/enrich/service.go`)
  → `provider_link_templates` storage (`internal/repo/provider_link_templates.go`, migration 0041)
  → `external_links` JSON projection (`internal/api/external_links.go`) → `<a href>` render
  (`ProviderLinkBadge.svelte`) — with a dedicated subagent plus independent spot-checks of the
  actual file contents (not just the diff). **No findings.** `ValidateLinkTemplate` requires
  exactly one `{id}` placeholder and validates the placeholder-substituted string through the
  same `validHTTPURL` helper `sanitizeProfileURL` already uses elsewhere (http/https scheme +
  non-empty host) — this PR correctly reused the established pattern rather than diverging from
  it. `BuildLink` interpolates the id via `url.PathEscape`, not raw string substitution or a
  template engine, which blocks `/`, `?`, `#` from the id — the characters that would let an
  attacker-controlled id introduce a new path segment, query string, or fragment and redirect off
  the declared host. SQL access in `provider_link_templates.go` is fully parameterized, no
  string-built queries from provider-supplied namespace/entityKind/template values. The frontend's
  `isHttpUrl` (`format.ts`) is a real scheme allowlist gating `href` as defense-in-depth (the
  backend already only emits validated http(s) URLs); no `{@html}`/unsafe sink anywhere in the
  changed `.svelte` files. One documented, intentional design tradeoff (not a vulnerability):
  `provider_link_templates` is keyed by `(namespace, entity_type)` rather than by provider
  (ADR-083 D2's shared-identity-space model, mirroring ADR-055), so any enabled provider's
  `/describe` can overwrite another provider's link template for a namespace it doesn't own,
  last-write-wins — worst case is still just an unexpected outbound link, matching the threat
  model's own stated ceiling. All 7 worklog gates are now closed — this is the last one. Next:
  mark draft PR #226 ready for review (fires the Jira In Review transition).

### 2026-08-09 · Testing gate closed — external_links projection + BuildProviderLink coverage
- skills: testing-strategy, simplify, security-review
- handoff: closed item #6. `externalLinksForEntity` (`internal/api/external_links.go`) and
  `Service.BuildProviderLink`/`verifiedClient`'s link-template persistence had no dedicated tests
  before this session — only their lower-level building blocks (`ExternalIDsForEntity`,
  `ValidateLinkTemplate`, `SanitizeLinkTemplates`) were covered. New
  `internal/api/external_links_test.go` exercises both layers together against a fake HTTP
  provider, since `persistLinkTemplates` only fires as a side effect of a real provider action
  (`Resolve`) — a direct repo write would skip the D2 wiring under test. `TestExternalLinks_
  MultiBadge`/`_Studio` prove ADR-083 D3 (one badge per stored id, 0..N): two distinct
  namespaced ids attached to the same entity via two separate `ReconcileVideoPeople`/
  `ReconcileVideoStudios` calls (additive across namespaces per `attachExternalID`'s `INSERT OR
  IGNORE`) round-trip as two independently namespace-split, labeled badges, and both handler call
  sites (person, studio) share the one projection. `TestExternalLinks_TemplateMismatch`
  table-drives ADR-083 D2's degraded state (no templates declared, wrong namespace, wrong entity
  kind) — all yield a label-only badge (`url` key omitted, never empty-string or an error).
  `TestExternalLinks_EnrichmentDisabled` covers the other D2 path (no enrichment service wired at
  all, a real deployment state) as its own test rather than a table row, since it needs no fake
  provider. `TestExternalLinks_MalformedIDSkipped` proves a stored id without a `namespace:id`
  separator is silently dropped, not surfaced as a partial entry. On the frontend, `sortExternalLinks`
  and `isHttpUrl` (`format.ts`) — the multi-badge ordering and the badge's XSS-safety `href` gate —
  gained unit coverage in `format.test.ts` (5 new tests); the component layer itself
  (`ProviderLinkBadge`, `EntityVideoMeta`) stays manual-QA-only per the standing frontend-
  automation gap. Updated `docs/testing-strategy.md` in 4 places (§4 backend table row, critical
  invariants bullet, §5 frontend table row, §11 known-gaps bullet). Ran `/simplify`: extracted a
  shared `linksByProvider` test helper (deduped the map-building loop across
  `TestExternalLinks_MultiBadge`/`_Studio`) and split `TestExternalLinks_EnrichmentDisabled` out
  of the mismatch table so it no longer bootstraps and discards a full fake-provider environment.
  Skipped (out of this diff's scope): extracting a shared fake-provider-harness helper across
  `external_links_test.go`, `provider_icon_test.go`, and `internal/enrich/enrich_test.go` — a
  pre-existing duplication those files already share, not introduced here. `go test ./internal/...`
  and `npm run test`/`npm run check` both clean. Next: security-review (item #7 — `LinkTemplates`
  validation, no injection via a malicious provider) — the last gate before the draft PR can leave
  draft.

### 2026-08-09 · Frontend gate closed — ProviderLinkBadge.svelte + person/studio wiring
- skills: simplify, testing-strategy
- handoff: built item #5. Research first: the F55 video badge item #5 references doesn't exist
  in code yet (the handoff doc itself flags this as still-open ADR-082 action item 6), so
  `web/src/lib/components/enrichment/ProviderLinkBadge.svelte` is a fresh build against the
  settled design spec, not an extraction — video-page wiring stays out of scope since the
  backend `external_links` projection was only built for person/studio (items #3-4). New
  `ExternalLink` type in `web/src/lib/types.ts` mirrors the Go `api.ExternalLink` struct;
  `sortExternalLinks` (`format.ts`) gives DD3's alphabetical-by-label order. `ProviderIcon.svelte`
  gained an opt-in `decorative` prop (default `false`, non-breaking for its 4 existing callers) so
  the badge's icon doesn't double-announce the name its own visible text already carries.
  `ProviderLinkBadge` renders `<svelte:element this={linked ? 'a' : 'span'}>` per DD2's linked
  vs. degraded states — the design handoff's a11y spec (aria-label text, `target="_blank"
  rel="noopener noreferrer"` for linked, tabindex-less span for degraded) implemented verbatim.
  Wiring surfaced a real duplication `/simplify` caught (3 of 4 review agents independently
  flagged the same thing): the count+badges row was copy-pasted between `EntityVideos.svelte`'s
  default branch and the person page's own `hero` snippet — not the single-line precedent that
  originally justified hero-owns-its-layout. Extracted `web/src/lib/components/entity/
  EntityVideoMeta.svelte` (count + sorted badges) as the one shared row, called from both;
  `ProviderLinkBadge`'s own linked/degraded branches were also deduped into the single
  `svelte:element` above instead of two near-identical `<a>`/`<span>` blocks. Verified end-to-end
  against real local data, not just `npm run check`/`test`: seeded a `provider_link_templates`
  row + restarted `backend-films` to prove the linked (TMDB) state resolves
  (`/api/v1/people/22` → `external_links[0].url`), confirmed the degraded state pre-existed
  for TMDB (no `link_templates` declared by the sidecar yet — expected, not a bug), and
  browser-verified both `/people/22` (badge renders, correct href/aria-label, no console errors
  on a clean tab) and `/studios/1` (no external id → no badge, no regression) — plus contrast
  ≥4.9:1 across all three skins (Cinémathèque 6.3:1, Broadcast 4.9:1, Brutalist 5.7:1) via
  computed-style checks (browser screenshots time out in this environment). `npm run check`: 0
  errors. `npm run test`: 134/134. Next: testing-strategy (item #6 — template-mismatch
  degradation + multi-badge cases) and security-review (item #7 — `LinkTemplates` validation,
  already backend-side; the frontend has no new server-fetch surface to review, just an `isHttpUrl`
  defense-in-depth `href` gate mirroring `EnrichPicker`'s).

### 2026-08-09 · Backend gate closed — LinkTemplates + external_links projection
- skills: simplify
- handoff: built items #3-4. New migration 0041 adds `provider_link_templates`, keyed by
  `(namespace, entity_type)` rather than `(provider, ...)` like the sibling `provider_field_hints`
  table — a namespace is a shared identity space across providers (ADR-055 D2), so on conflict
  whichever provider's `/describe` was read most recently owns the row (proved with a dedicated
  test: `TestProviderLinkTemplates_LastDescribeWinsAcrossProviders`). `Manifest.LinkTemplates`
  (`internal/enrich/enrich.go`) extends the `/describe` wire contract; `internal/enrich/
  link_templates.go` adds `ValidateLinkTemplate`/`SanitizeLinkTemplates`/`BuildLink` (one `{id}`
  placeholder, http(s)-only, path-escaped on render — not an SSRF gate, since a link template is
  never dialed server-side, only rendered as an outbound `<a href>`). `Service.linkTemplates` is a
  new DB-backed atomic-pointer cache in `internal/enrich/service.go`, deliberately following the
  DB-persisted `fieldHints` posture rather than the in-memory-only `preferredPatterns` one, since
  person/studio badges are visitor-visible and must not silently lose their links on every
  restart. `internal/repo/identity.go` gained `ExternalIDsForEntity`, reusing the existing
  `externalIDTable` helper. `internal/api/external_links.go` projects those rows into
  `ExternalLink{provider, label, url}` — the display label comes from a small Holodex-owned
  `namespaceLabel()` map, deliberately kept OUT of the provider-declared manifest, because a
  value's namespace can differ from the provider that emitted it (TMDB emits `imdb:`-namespaced
  ids). Wired into `getPerson`/`getStudio` as `external_links` (best-effort: a lookup failure logs
  and serves the page with no badges). Ran `/simplify`: extracted a shared `validHTTPURL` helper
  (deduped against the pre-existing `sanitizeProfileURL`), removed a redundant string scan in
  `ValidateLinkTemplate`, and added a `Service.LinkTemplates()` accessor mirroring `FieldHints()`
  to simplify `BuildProviderLink`. Flagged but deliberately left for later, since they're either
  out of this diff's scope or not yet load-bearing at current scale: `fieldHints`/
  `preferredPatterns`/`linkTemplates` are now three separately hand-rolled atomic-cache trios in
  `service.go` worth generalizing before a fourth provider-declared field lands; a disabled
  provider's `provider_link_templates` row currently has no prune/ownership-reconciliation story;
  `namespaceLabels`' 2-entry hardcoded map will need to move to config if it grows past a handful
  of well-known namespaces. All builds/vet/tests pass. Next: frontend (item #5 — shared
  `ProviderLinkBadge.svelte`, wired into person/studio/video headers), then testing-strategy and
  security-review (items #6-7; `internal/enrich` is the SSRF perimeter, but LinkTemplates
  introduces no new server-side-fetch surface — worth confirming formally via `/security-review`
  rather than resting on that reasoning alone).

### 2026-08-09 · Design gate closed — multi-badge handoff written
- skills: design-handoff, simplify, graphify
- handoff: wrote `docs/design/provider-link-badge-handoff.md`, closing the design gate. Treated
  the video badge's visual anatomy (pill shape, icon+label, hover/focus) as already settled from
  this session's earlier mockup/critique pass and scoped this doc to what's actually new for
  person/studio: DD1 badges join the existing muted video-count line rather than opening a new
  row (person/studio have no resolution/duration/year row to slot into the way video does); DD2
  wrap (not truncate/overflow) at 3+ badges, no "+N more" control since that's premature per
  ADR-083's own revisit note; DD3 alphabetical-by-label ordering for determinism. Specced the
  three cardinality states (0/1/N) — zero renders nothing, no "not enriched" placeholder, since
  this is a passive metadata line, not the completeness panel. Specced the degraded no-link-
  template state from ADR-083 D2: badge still renders (the identity signal is the point, not the
  click-through) but as a non-interactive `<span>`, no href/hover/focus/tabindex. Reused
  `EnrichPicker.svelte`'s existing `profile_url` link pattern verbatim for the linked badge's
  `target="_blank" rel="noopener noreferrer"` + aria-label convention rather than inventing a new
  one, since it's the same provider-attested-URL shape. Next: backend (`Manifest.LinkTemplates` +
  the person/studio `external_links` projection, items #3-4).

### 2026-08-09 · Architecture gate closed — ADR-083 written
- skills: architecture, design-handoff
- handoff: this session started from an observation that `imdb_id` was mislabeled for a
  provider-agnostic deployment (already fully resolved at the schema/registry/provider level by
  ADR-082 — the only real gap was frontend display), iterated through mockups to a clickable
  provider-name badge placed inline in the header metadata row (not id text, not a separate
  section), then the owner asked to extend that badge to person and studio. Research showed that's
  not a copy-paste: `person_external_ids`/`studio_external_ids` (ADR-054/055) are join-table
  identity keys outside the resolver, not resolved scalars like video's `external_provider_id`.
  Wrote ADR-083: D1 person/studio badge data is a read-only projection of those tables (no
  resolver/F55-scoring change); D2 the outbound link is built server-side from a new
  provider-declared `Manifest.LinkTemplates` (extends the `/describe` contract alongside
  `IDNamespaces`/`BrandIcon`) rather than a frontend-hardcoded map, keeping "providers declared,
  not compiled in" (ADR-033) intact; D3 render one badge per stored external-id row (0..N) rather
  than inventing a "primary" — an intentional asymmetry from video's single badge. Added the
  ADR-083 row to `docs/architecture/README.md`. HOLODEX-260 (the original F55 epic) is fully done
  and merged (PR #222), so this scope extension got its own epic, HOLODEX-266, rather than
  reopening a closed one; renamed the worktree branch and fired the Jira In Progress transition.
  Next: `/design-handoff` for the multi-badge/no-link states, then backend (`LinkTemplates` +
  the person/studio projection).
