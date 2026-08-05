# Spec: Configurable per-provider search query patterns (F54)

**Status**: Draft
**Phase**: Phase 3 follow-up (enrichment quality)
**Owner**: Project owner
**Date**: 2026-08-05
**Feature block**: **F54** — let a provider advertise, and an operator override, a **search query
pattern** (`{studio?} {title?} {performers?} {year?}`) rendered from a video's already-resolved
fields instead of its raw title; additionally, sanitize the raw-title fallback itself (strip
bracket/comma punctuation and resolution tokens) so an un-enriched file's messy filename doesn't
sink its own first search.

**Issue**: [HOLODEX-254](https://whoiskevinrich.atlassian.net/browse/HOLODEX-254)
**ADR**: [ADR-080](../architecture/ADR-080-configurable-provider-search-patterns.md) (the
mechanism — three-tier precedence, token grammar, sanitizer, wire-contract-unchanged posture)
**Design handoff**: [configurable-provider-search-patterns-handoff.md](../design/configurable-provider-search-patterns-handoff.md)
(confirms zero `EnrichPicker.svelte` diff; pins the exact seeded-string content per scenario, incl.
the empty-sanitization fallback; specs the optional P1 transparency caption)

**Depends on** (all shipped):
- the provider sidecar contract, `GET /describe` / `POST /resolve` ([ADR-033](../architecture/ADR-033-metadata-source-plugins.md))
- `resolver.ResolveFields` resolving `studio`/`title`/`actors`/`director`/`release_date` before
  `getMedia` returns ([ADR-052](../architecture/ADR-052-baseline-source-contract.md))
- the operator provider registry, `metadata-sources.yaml` + hot-reload via `POST /admin/reload-config`
  (`internal/enrich`, [ADR-033](../architecture/ADR-033-metadata-source-plugins.md))
- `EnrichPicker.svelte`'s generic `entityName` seed prop (F22.5b) — this spec changes what value
  callers pass in, not the component itself

**Touches untrusted provider-advertised content (`/describe.preferred_search_pattern`) that flows
into an outbound HTTP query → a `/security-review` sign-off is required before merge** (label
`needs-security-review`, already applied to the issue).

---

## Problem Statement

Every enrichment search Holodex sends today is one free-text string seeded from the entity's raw
`title` — the resolved file title, or for a freshly scanned file with no clean tags yet, literally
the filename (`[MyStudio] My Title (Some Actor, Other Actor) 720p`). A provider's search index
frequently does better with a shaped, cleaner query, and Holodex already resolves the structured
fields (studio, title, cast, year) that could build one — it just never uses them for search. The
cost of not solving it: weak or zero matches on exactly the files that most need enrichment (newly
added, nothing curated yet), forcing the owner to hand-edit the search box every time.

## Goals

1. **A provider gets a materially better default query** when the operator or the provider itself
   configures a shaped pattern, with zero change to how the owner interacts with the Enrich picker.
2. **Every un-enriched file gets a cleaner default query, unconditionally** — bracket punctuation and
   resolution tokens are stripped from the title-based fallback with no configuration required.
3. **No wire-protocol change.** `POST /resolve`'s `hint.query` stays exactly the single string it is
   today; `providers/tmdb/` needs zero code changes to keep working.
4. **No regression, no migration** for any provider/entity with no pattern configured — behavior is
   additive on top of today's (except the unconditional sanitizer in Goal 2, which is deliberately a
   pure quality fix, not a gated feature).
5. **Fail-soft on bad config.** A malformed or unknown-token pattern (operator or provider-supplied)
   never disables the provider or 500s a resolve — it's skipped, logged, and falls through to the
   next tier.

## Non-Goals

- **Person/Studio pattern support.** `model.Person`/`model.Studio` carry only `Name`/`Aliases` today
  — no studio/year/performer fields exist to fill a template with. Those entities stay name-seeded.
  *(Why: no data to build a pattern from; revisit if those models gain more fields.)*
- **Literal decoration in patterns** (e.g. `"{title} ({year})"`, custom separators). v1 patterns are
  space-joined tokens only. *(Why: matches the concrete ask; real demand for punctuation can be a
  small, later grammar extension.)*
- **A settings UI for this config.** Lives in `metadata-sources.yaml` only, like every other provider
  knob (`base_url`, `asset_hosts`, `enabled`). *(Why: consistency with existing provider config; no UI
  exists for any of those today either.)*
- **Structured `hint.fields` over the wire.** Core renders one opaque string; providers don't receive
  field-level query control. *(Why: one provider exists today; a protocol change isn't justified until
  a second provider actually wants field-level control — see ADR-080 D1.)*
- **Full scene-release-tag parsing in the sanitizer** (codecs, sources, encoder groups like `x264`,
  `WEB-DL`). Only punctuation and resolution/quality tokens are stripped. *(Why: matches the concrete
  ask; F48/ADR-067's filename **extraction** already owns real structured parsing — once a file goes
  through that path, clean resolved fields take over and the sanitizer stops being the deciding
  factor.)*

---

## Users & Value

- **Owner**: opens the Enrich picker on a fresh, un-enriched file and sees a cleaner default query
  (Goal 2) with zero setup, and — if they've configured a `search_pattern` for a provider, or the
  provider advertises one — a shaped query that's more likely to land a strong match on open. No new
  UI to learn; the search box behaves exactly as today, just seeded better.
- **Operator** adding/tuning a provider: gains one optional YAML key (`search_pattern`, or
  `default_search_pattern` for a fleet default) to shape that provider's queries, discoverable next to
  `base_url`/`asset_hosts` in the same file.
- **Provider author** (e.g. a future non-TMDB sidecar): can advertise a sensible query shape via
  `/describe` with zero operator configuration required — the "works out of the box" property.

---

## Functional Requirements

### Must-Have (P0)

#### FR1 — Operator pattern config (`metadata-sources.yaml`)

`enrich.Source` gains `search_pattern` (per-provider override, highest precedence). `fileConfig`
gains `default_search_pattern` (applies to any enabled provider that specifies neither its own
override nor advertises a preference). Both optional; absent = no change to today's behavior at that
tier.

- **Given** a provider has `search_pattern: "{studio?} {title?} {performers?} {year?}"` set, **when**
  the owner opens Enrich for a video of that provider, **then** the search box is seeded with the
  rendered pattern, not the raw title.
- **Given** no `search_pattern`/`default_search_pattern` is configured anywhere, **when** any resolve
  happens, **then** behavior is unchanged from today (modulo FR4's unconditional sanitizer).

#### FR2 — Provider-advertised preference (`/describe.preferred_search_pattern`)

`enrich.Manifest` gains one optional key, `preferred_search_pattern` (string). Additive: an old
provider that omits it, or an old Holodex that doesn't parse it, both work unchanged — no protocol
version bump. Consulted only when the operator has not set an explicit `search_pattern` for that
provider (FR1 tier 1 wins).

- **Given** a provider's `/describe` response includes `preferred_search_pattern` and the operator has
  set no `search_pattern` override, **when** a resolve happens for that provider, **then** the
  provider's preferred pattern is used.
- **Given** both an operator `search_pattern` and a provider `preferred_search_pattern` exist, **when**
  a resolve happens, **then** the operator's value wins.

#### FR3 — Token grammar, rendering, and precedence fallthrough

New `internal/enrich/query.go`. Grammar: `{name}` (required) or `{name?}` (optional), `name` ∈
`studio | title | performers | year`, space-joined, no other punctuation. Values: `studio`/`title`
from their top-precedence resolved value; `performers` = top 3 of `actors`+`director` combined,
space-joined; `year` = the 4-digit year parsed from `release_date`. Rendering an optional token with
no value drops it; rendering a required token with no value fails that whole tier, which falls
through to the next tier in FR1/FR2's precedence order, ultimately to FR4's sanitized-title floor. An
unknown token name in a configured pattern is a config error: log a warning, drop just that pattern
(the provider stays enabled), matching `enrich.go`'s existing skip-malformed-entries posture.

- **Given** a pattern where every token resolves (or is optional and empty), **then** the rendered
  string joins the non-empty values with single spaces, in token order.
- **Given** a pattern with a *required* token (no `?`) that has no resolved value, **then** that whole
  tier is skipped and the next-lower tier is evaluated.
- **Given** a pattern referencing an unknown token name, **then** it is rejected at config-load time
  (logged, dropped) — the provider is unaffected otherwise.

#### FR4 — Unconditional title sanitizer

A `sanitizeTitle(s string) string` applied to (a) the `{title}`/`{title?}` token's value wherever
substituted, and (b) the raw-title floor tier (used when no pattern renders at any tier, including
"no pattern configured at all"). Behavior: delete `[`, `]`, `(`, `)`, `{`, `}`, `,` characters
(keeping their contents); strip resolution/quality tokens matching `\b\d{3,4}p\b` and `\b[48]k\b`
(case-insensitive, word-bounded); collapse repeated whitespace to one space and trim. This tier
applies **regardless of configuration** — there is no opt-out, since it has no plausible worse case
than sending the literal cluttered string. If stripping leaves nothing (a degenerate title that is
*only* bracket/resolution noise, e.g. `[720p]`), the sanitizer returns the raw, unsanitized input
instead of an empty string — the search box must never be seeded blank.

- **Given** a video whose resolved title is `[MyStudio] My Title (Some Actor, Other Actor) 720p`,
  **when** no pattern is configured for the provider (raw-title floor applies), **then** the rendered
  query is the bracket/comma/resolution-stripped, whitespace-collapsed version — not the literal
  string.
- **Given** a title containing a number that is not a resolution token (e.g. `Agent 007`), **when**
  sanitized, **then** it is left unchanged (the resolution regex is word-bounded to `\d{3,4}p`/`[48]k`
  only).
- **Given** a configured pattern using `{title?}`, **when** rendered, **then** the substituted value is
  the sanitized title, not the raw resolved value.

#### FR5 — Wiring: choke point, response payload, zero picker changes

`videoHint()` (`internal/api/enrich.go`) calls the new `Source.BuildQuery(resolvedFields)` before
constructing `enrich.Hint`; a `false`/empty result falls back to FR4's sanitized-title floor
(never the fully-raw title again, per FR4). The video-detail response (`getMedia`) adds one small
computed field, `enrich_queries: {provider_name: string}`, using the same rendering — so the SPA has
the value in the same request that renders the page. The video-detail page passes
`enrich_queries[provider.name]` (falling back to the sanitized title if absent) into
`EnrichPicker.svelte`'s existing `entityName` prop. **No changes to `EnrichPicker.svelte` itself** —
the owner can still freely retype the box; this changes the seeded default only.

- **Given** the video-detail page loads, **then** its response includes a rendered query per
  enabled+applicable provider, computed server-side from the same resolved fields already in that
  response.
- **Given** the owner opens Enrich for a provider with a rendered query available, **then** the search
  box is seeded with it and auto-searches on open exactly as today's title-seeded flow does (F22.5b's
  existing auto-search-on-open behavior is unaffected).

### Nice-to-Have (P1)

- **P1-a** — a small caption under the search box when the seeded value differs from the raw title
  (e.g. "Built from search pattern"), giving the owner visibility into why the box isn't just the
  title. *Not required — the existing "type to search" affordance already lets the owner see and edit
  the seeded value; this is pure transparency polish.*

### Future Considerations (P2)

- **P2-a** — structured `hint.fields` over the wire (ADR-080 D1 Option B), if a second provider wants
  field-level query control instead of a pre-formatted string.
- **P2-b** — literal decoration in patterns (parens, brackets, custom separators) if real operator
  demand shows up.
- **P2-c** — Person/Studio pattern support, contingent on those models gaining studio/year/performer-
  equivalent fields.
- **P2-d** — an owner-facing settings UI for `search_pattern`/`default_search_pattern` (ADR-060's
  pattern), if YAML-edit-and-reload proves too heavy in practice.
- **P2-e** — broader sanitizer coverage (scene-release tags: codecs, sources, encoder groups), if F4's
  narrow punctuation+resolution scope proves insufficient in practice.

---

## Acceptance Criteria

1. A provider with an operator-configured `search_pattern` produces a rendered query using that
   pattern for every video where at least the pattern's required tokens resolve; the raw title is
   never used when a configured pattern fully renders.
2. A provider with no operator override but a `/describe.preferred_search_pattern` uses that pattern
   under the same rendering rules.
3. A provider with neither uses `default_search_pattern` if the operator set one, else falls to the
   sanitized-title floor (FR4) — never the fully-raw title.
4. An optional token (`{x?}`) with no resolved value is omitted from the output with no artifact
   (no `"undefined"`, no doubled delimiters visible to the owner).
5. A required token (`{x}`) with no resolved value drops that entire tier to the next one — verified
   for all three precedence tiers plus the floor.
6. An unknown token name in either an operator or provider pattern is rejected at load/ingest time
   (logged, dropped) without disabling the provider or breaking resolve for it.
7. `[MyStudio] My Title (Some Actor, Other Actor) 720p` sanitizes to `MyStudio My Title Some Actor
   Other Actor` (brackets/parens/comma removed, `720p` stripped, whitespace collapsed) — verified as
   both the floor-tier output and the `{title?}` token's substituted value.
8. A title containing digits that aren't a resolution token (`Agent 007`, `Suite 1080`) is left
   unchanged by the sanitizer.
8a. A title that sanitizes to an empty string (e.g. `[720p]` alone) falls back to the raw,
    unsanitized title — the search box is never seeded blank.
9. `EnrichPicker.svelte`'s diff for this feature is zero — the component's own file is unchanged;
   only what its `entityName` prop receives changes.
10. `POST /resolve`'s request body shape is unchanged (`{entity_type, hint: {query, external_ids}}`) —
    verified against `providers/tmdb/` with no sidecar-side changes required.
11. An instance with `metadata-sources.yaml` unchanged from before this feature (no new keys) behaves
    identically to today for pattern tiers 1–3, and gains only FR4's sanitized-floor improvement.

---

## Test Notes (for `/testing-strategy`)

- **Precedence resolution** — all four tiers (operator override, provider preference, global default,
  sanitized floor) individually, plus fallthrough when a higher tier's required token is missing.
- **Token rendering** — optional-token omission; required-token tier failure; `performers` cap at 3;
  `year` parsed correctly from `release_date` including edge cases (missing/partial dates).
- **Config validation** — unknown token name in `search_pattern`/`default_search_pattern`/
  `preferred_search_pattern` is rejected at parse/ingest, logged, and doesn't take down the provider;
  hot-reload (`POST /admin/reload-config`) picks up a pattern change.
- **Sanitizer** — bracket/paren/brace/comma stripping; resolution-token regex including word-boundary
  false-positive guards (`Agent 007`, `Suite 1080` unaffected); whitespace collapse; applied both to
  the floor tier and to `{title}` token substitution.
- **Wire contract** — `POST /resolve` body shape unchanged; a golden test against `providers/tmdb/`
  confirming zero sidecar-side changes are needed.
- **API/SPA** — `getMedia` response carries `enrich_queries` correctly per enabled+applicable provider;
  `EnrichPicker.svelte` is unchanged (a file-diff assertion, not just behavioral); F22.5b's
  auto-search-on-open still fires against the new seeded value.
- **Backward-compat golden** — a provider/config with no pattern keys set produces identical `hint.query`
  values to pre-F54, except where FR4's sanitizer changes the floor-tier output (a second golden
  confirming the sanitized floor is a pure improvement, never worse than the literal title).
- **Security** (feeds `/security-review`) — an adversarial `preferred_search_pattern` from a provider
  (oversized string, control characters, an absurd number of tokens) is sanitized/bounded on ingest and
  cannot cause a resource or injection issue when rendered into the outbound `hint.query`.

---

## Open Questions

- **[engineering, non-blocking] `performers` cap of 3** — is 3 the right cap, or should it be
  configurable per pattern (e.g. `{performers:2?}`)? Start fixed at 3; revisit if real match-quality
  data suggests otherwise.
- **[engineering, non-blocking] `director` in `performers`** — should the `performers` token include
  `director` alongside `actors`, or actors only? Directors are less likely to be what a provider's
  cast-search indexes well. Default to actors+director as scoped in ADR-080; flip to actors-only if
  match-quality data says otherwise post-launch.
- **[engineering, non-blocking] Sanitizer resolution vocabulary** — `\d{3,4}p` and `[48]k` cover the
  common cases; is there a real-world filename pattern in the owner's own library that this misses
  (e.g. `UHD`, `HD`, `SD` literal tokens)? Non-blocking; extend the regex list if real files surface a
  gap during QA.

---

## Timeline Considerations

Single feature block: two new YAML/wire fields (`search_pattern`/`default_search_pattern`,
`preferred_search_pattern`), one new small package file (`internal/enrich/query.go`), one call-site
change (`videoHint()`), one response-field addition (`getMedia`), one caller-side prop-value change
(video-detail page → `EnrichPicker`). No migration, no flag — absence of any pattern config is a
no-op beyond FR4's unconditional sanitizer. Ship after `/security-review` on the implementation diff
(untrusted provider-advertised string flowing into an outbound query).
