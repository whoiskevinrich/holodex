# ADR-080: Configurable per-provider metadata search query patterns

**Status:** Proposed
**Date:** 2026-08-05
**Deciders:** Project owner

**Extends:** [ADR-033](ADR-033-metadata-source-plugins.md) (provider sidecar contract — `/describe`/`/resolve`, core owns the registry) ·
[ADR-013](ADR-013-metadata-field-mapping.md) (the operator-YAML config-file idiom this reuses) ·
[ADR-056](ADR-056-provider-field-render-hints.md) (precedent for an additive, no-protocol-bump `/describe` extension and an explicit operator-over-provider precedence ladder).
**Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (explicit standing override outranks an automatic default — the same posture this ADR's precedence chain follows) ·
[ADR-067](ADR-067-filename-extraction-confidence-and-rollback.md) (F48's `{token}` filename-**matching** grammar — a deliberate non-reuse; see D3) ·
[ADR-060](ADR-060-runtime-owner-settings.md) (the DB-backed owner-settings pattern considered and deferred; see D2).
**Contract:** [metadata provider contract](../specs/metadata-provider-contract.md) §2.2 (`/describe` manifest — gains one optional key) / §2.3 (`/resolve` hint — unchanged).
**Spec:** [Configurable provider search patterns (F53)](../specs/configurable-provider-search-patterns.md).
**Issue:** [HOLODEX-254](https://whoiskevinrich.atlassian.net/browse/HOLODEX-254).

---

## Context

Every enrichment search Holodex sends today is one free-text string, seeded from the entity's own raw title/name (`internal/api/enrich.go` `videoHint()`/`enrichResolve()`/`enrichStudioResolve()`, wired to `enrich.Hint{Query string}`). For a video, that's the file's resolved `title` field or, on the SPA side, `EnrichPicker.svelte`'s `entityName` prop — literally whatever text sits in the title, filename-derived or not. TMDB's own sidecar already does its own best-effort regex parse of that string into `(title, year)` before querying its search API (`providers/tmdb/tmdb.go` `parseReleaseFilename`) — i.e., the *provider*, not Holodex, is the one guessing structure out of an unstructured string today.

A provider's search index frequently does better with a shaped query — e.g. `{studio} {title} {performers} {year}` instead of a bare title — but Holodex has no way to build one, and no way for a provider to say what shape it wants. The data to build such a query already exists server-side: `resolver.ResolveFields` for a video already resolves `studio`, `title`, `actors`/`director`, and `release_date` before `getMedia` returns (per ADR-052's entity-generic core). Person and Studio are thinner (`model.Person`/`model.Studio` carry only `Name`/`Aliases`) — no studio/year/performer context exists to fill a template with, so this ADR's scope is video-only for now.

The case that benefits most is also the one with the least structured data to work with: a **freshly scanned file with no enrichment and no clean file tags yet** has no resolved `studio`/`performers`/`year` to build a shaped query from at all — the `title` field itself is frequently just the raw filename (`[MyStudio] My Title (Some Actor, Other Actor) 720p`), bracket punctuation, comma-separated cast lists, and resolution markers included verbatim. That clutter actively hurts a provider's search relevance and is exactly what today's literal-title fallback sends as-is.

### Forces

- **Zero wire-protocol break.** `POST /resolve`'s `hint.query` (a single string) must stay exactly that shape — one provider (TMDB) exists today and nothing warrants a protocol version bump for it (contract §2.2's "unknown keys ignored" rule is the only extension budget this ADR should spend).
- **Provider expressiveness without provider trust.** A provider should be able to *suggest* a query shape (mirrors ADR-056's `field_hints` precedent), but an operator's explicit choice must outrank it, and a malformed suggestion must never break enrichment for that provider (fail-soft, matching `enrich.go`'s existing "skip malformed entries" posture at `parse()`).
- **No regression, no migration.** A provider/entity with no *pattern* configured must behave exactly as today (raw title), unaffected by anything in D2/D3/D5. The one deliberate exception is D4: the raw-title floor's *content* changes unconditionally, for everyone, with no config to opt into — because stripping punctuation/resolution tokens has no plausible worse case than sending them literally, this is treated as a pure quality fix rather than a gated feature.
- **Reuse the resolved data that already exists.** The video detail response already carries `resolver.ResolvedField`s (studio/title/actors/director/release_date) by the time a Hint is built — no new resolution plumbing should be needed.
- **Don't force an abstraction across two different template directions.** F48 (ADR-067) already has a `{token}` grammar, but it *matches* a filename into fields (string → fields). This feature needs the inverse (fields → string), with optional-token semantics F48 never needed. They are siblings in *shape*, not one mechanism.

---

## Decision

Core renders a per-provider query **pattern** — an ordered list of `{token}`/`{token?}` placeholders over already-resolved video fields — into the existing single-string `hint.query`, before the unchanged `/resolve` call. Five sub-decisions:

### D1 — Core renders a string; the `/resolve` wire contract is untouched

The pattern is resolved to a plain string entirely inside Holodex, at the same choke point that builds `enrich.Hint` today (`internal/api/enrich.go`). The provider sidecar receives exactly what it receives now: `{"entity_type": ..., "hint": {"query": "...", "external_ids": [...]}}`. No new field on `Hint`, no protocol-version bump, no change required in `providers/tmdb/`.

**Chosen over:** extending `hint` with structured fields (`hint.fields: {studio, title, performers, year}`) and letting each provider decide how to use them — see Options, D1.

### D2 — Config carriage: operator YAML override, provider-advertised preference, one global default

Three tiers, highest-to-lowest authority, mirroring ADR-051's "an explicit standing decision outranks an automatic default":

1. **Operator override** — `enrich.Source` gains `search_pattern` (`internal/enrich/enrich.go`), set per-provider in `metadata-sources.yaml`, parallel to `asset_hosts`/`base_url`. Highest authority: the operator who configured this provider knows their own search engine best.
2. **Provider-advertised preference** — `enrich.Manifest` (the `GET /describe` wire type) gains one optional key, `preferred_search_pattern` (string, additive — absent/unknown-key-safe, no protocol bump, same idiom as ADR-056's `field_hints` and ADR-059's `brand_icon`). Consulted only when the operator hasn't set an explicit override for that provider.
3. **Global default** — `fileConfig` gains `default_search_pattern` (top-level key in `metadata-sources.yaml`), applied to any enabled provider that specifies neither of the above.
4. **Floor (not itself configurable): raw title.** If no tier renders (see D3), or none is configured at all, behavior is exactly today's.

**Chosen over:** a DB-backed, owner-editable runtime setting (ADR-060's pattern). Every other provider knob (`base_url`, `asset_hosts`, `enabled`) already lives in `metadata-sources.yaml` with no UI; adding a UI for only this one field would be an inconsistent, unrequested surface. Revisit via ADR-060's pattern if operators end up iterating on patterns often enough that file-edit-and-reload becomes real friction.

### D3 — Token grammar: space-joined, optional-aware, video-scoped

```
pattern  := token (" " token)*
token    := "{" name ["?"] "}"
name     := "studio" | "title" | "performers" | "year"
```

| token | source | notes |
|---|---|---|
| `studio` | `resolver.ResolvedField["studio"]`, top-precedence value | |
| `title` | `resolver.ResolvedField["title"]` | |
| `performers` | `actors` + `director`, top 3 values, space-joined | capped — avoids runaway-long queries |
| `year` | 4-digit year parsed from `release_date` | |

Rendering: for each token in order, resolve its value. `{x?}` (optional) with no value is dropped from the output; `{x}` (no `?`, required) with no value fails the **whole tier**, which falls through to the next precedence tier in D2. Non-empty values join on a single space. An unknown token name is a config error: `parse()` logs a warning and drops just that `search_pattern`/`preferred_search_pattern` (the provider itself stays enabled, falling through the tiers) — the exact fail-soft posture `enrich.go`'s `parse()` already applies to a missing `name`/`base_url`.

The `title` token's value is passed through the **sanitizer** (D5) before substitution — see below; `studio`/`performers`/`year` are already clean structured values and are used as-is.

**Chosen over:** extending F48's `internal/extract` `PatternStore`/`{token}` grammar to add optional-token semantics. Rejected — that grammar's direction is filename → fields (matching), this one is fields → string (rendering); forcing a shared abstraction over two different directions of intent would cost more than the ~40 lines of overlap it'd save (simplicity-first). A new sibling file, `internal/enrich/query.go`, mirrors the *shape* of `PatternStore` (same atomic-store/hot-reload idiom) without sharing its matching logic.

**Scope:** video only. Person/Studio stay name-seeded — `model.Person`/`model.Studio` have no studio/year/performer fields to fill a template with today. Extending this to those entities is future work contingent on those models gaining more fields, not blocked by anything in this ADR.

### D4 — Sanitize the `title` token and the raw-title floor

The raw-title floor (D2 tier 4) and the `{title}`/`{title?}` token (D3) both draw on `resolver.ResolvedField["title"]`, which for an un-tagged, freshly scanned file is frequently just the filename — bracketed studio tags, comma-joined cast lists, and resolution/quality markers included verbatim (`[MyStudio] My Title (Some Actor, Other Actor) 720p`). Sending that literally is the worst case for exactly the files that most need a search hit: no enrichment yet, no clean file metadata, nothing but a cluttered filename to go on.

A **title sanitizer**, applied to that value wherever it is used (the `{title}` token substitution, and the D2-tier-4 fallback):

1. Delete bracket/paren/brace characters (`[`, `]`, `(`, `)`, `{`, `}`) and commas — keep their contents as plain words, drop the punctuation itself.
2. Strip resolution/quality tokens: `\b\d{3,4}p\b` (`480p`, `720p`, `1080p`, `2160p`) and `\b[48]k\b` (`4k`/`8k`), case-insensitive, word-bounded so it never eats a real digit sequence that happens to end in `p`/`k`.
3. Collapse repeated whitespace left behind by (1)/(2) to a single space and trim the ends.

`[MyStudio] My Title (Some Actor, Other Actor) 720p` → `MyStudio My Title Some Actor Other Actor`. Deliberately narrow scope, matching the ask: punctuation and resolution only — not a full scene-release-tag parser (no attempt to strip codec/source tags like `x264`/`WEB-DL`/encoder group names). A more aggressive cleanup is a natural follow-up once real search-quality data exists to justify it.

**Chosen over:** leaving the floor tier literal and relying solely on operator-configured patterns. Rejected as the primary fix — a fresh, un-enriched file is exactly the case where no pattern *can* apply yet (no resolved studio/performers/year to fill one), so improving the floor tier itself is the only lever that helps that case.

### D5 — Consumption: existing choke points, existing payloads, zero picker changes

`videoHint()` (and, only if a pattern applies, its siblings) calls the new `Source.BuildQuery(resolvedFields)` before constructing `enrich.Hint`; a `false` return (nothing rendered at any tier) falls back to today's raw-title call, unchanged. The video-detail response (`getMedia`, which already has `resolved[]` in scope) adds one small computed field — e.g. `enrich_queries: {provider_name: string}` — so the SPA gets the rendered value in the same request, no extra round trip.

`EnrichPicker.svelte` requires **no code change**: it already accepts a generic `entityName` prop it seeds the search box from (`EnrichPicker.svelte:37`). The video-detail page just starts passing the server-computed value instead of the raw title. The owner can still freely retype the box — this changes the *default*, not the interaction model.

**Chosen over:** a new endpoint the picker calls on open to fetch the rendered query. Rejected — the data needed is already loaded by the same request that renders the page; a second round trip buys nothing and adds a loading-state case the picker doesn't have today.

---

## Options Considered

### D1 — where the query gets built

#### A — core renders a string, `/resolve` contract unchanged (chosen)
**Pros:** zero protocol change; zero change to `providers/tmdb/` or any future provider; matches the literal ask (a pattern *string*). **Cons:** Holodex, not the provider, decides delimiter/ordering/capping — a provider with a fussier search engine can't ask for something the token vocabulary doesn't cover (mitigated: the vocabulary is exactly the fields the ask named, and is extensible later).

#### B — structured `hint.fields` over the wire
**Pros:** more powerful — a provider could weight fields differently (try title+year first, fall back to studio) instead of receiving one flattened string. **Cons:** every provider (not just TMDB) would need field-aware search logic instead of a plain-text query; a protocol change, even an additive one, is real scope for a single-provider system today. Rejected for v1 — revisit if/when a second provider wants query control finer than a pre-built string.

### D2 — precedence tiers

#### A — operator > provider > global default > raw title (chosen)
**Pros:** consistent with every other "operator wins" precedent in this codebase (ADR-051, ADR-056 D2); an operator with no opinion still benefits from a provider's own advertised preference; a global default covers providers that advertise nothing without per-provider busywork. **Cons:** one more optional YAML key to document — negligible.

#### B — provider preference always wins over operator config
**Pros:** none identified beyond deferring to the party who presumably knows their own search engine best. **Cons:** contradicts every existing precedence precedent in this codebase and removes the operator's ability to fix a provider's bad suggestion without disabling the provider entirely. Rejected.

### D4 — sanitizing the fallback/`title` value

#### A — strip bracket punctuation + resolution tokens, collapse whitespace (chosen)
**Pros:** directly targets the worst case (a freshly scanned, un-enriched file whose only "title" is its cluttered filename); no dependency on any pattern being configured; scoped narrowly enough to review and test exhaustively. **Cons:** a hand-rolled regex/character-strip is one more small piece of string-handling to maintain, and a sanitizer this narrow won't catch scene-tag cruft (`x264`, `WEB-DL`, encoder names) — acceptable, since that wasn't the ask and a broader cleanup can follow once real search-quality data justifies it.

#### B — leave the floor tier literal; rely on operator-configured patterns to work around messy titles
**Pros:** no new code. **Cons:** doesn't help the case that needs it most — a fresh, un-enriched file has no resolved studio/performers/year, so no pattern (D2 tiers 1-3) can apply yet; the literal, cluttered title is the only thing sent. Rejected.

### D5 — how the SPA gets the rendered query

#### A — embed in the existing entity payload (chosen)
**Pros:** no new endpoint, no new loading state, no `EnrichPicker` change at all. **Cons:** the video-detail handler grows by one small map — negligible.

#### B — new endpoint, picker fetches on open
**Pros:** keeps the entity payload smaller. **Cons:** a second round trip on every picker open, and a new loading/error state the picker doesn't have today, for data the page already has in hand. Rejected.

---

## Trade-off Analysis

**Provider expressiveness vs. protocol stability.** D1's core-renders-a-string choice trades away a provider's ability to make fine-grained per-field search decisions for zero protocol churn. That's the right trade with one provider in the registry; it stops being free the moment a second provider wants field-level control rather than a pre-formatted string, at which point D1-Option-B (structured `hint.fields`) becomes worth relitigating.

**Config surface consistency vs. discoverability.** D2 keeps the pattern in `metadata-sources.yaml` alongside every other provider knob rather than introducing a settings-UI precedent (ADR-060) for just this one field. The cost is that changing a pattern requires a file edit + `POST /admin/reload-config`, not a form — acceptable for a knob operators set once when adding a provider, not one they'd want to A/B test from a UI.

**A new, distinct grammar vs. reusing F48's.** D3 introduces optional-token (`{x?}`) semantics that don't exist anywhere else in the codebase, in a new file rather than extended onto `internal/extract`. The two grammars solve inverse problems (match vs. render); sharing code between them would be an abstraction serving no second caller, which the codebase's simplicity-first convention argues against. The cost is a second small `{token}`-parsing implementation to maintain, not a shared one.

**Narrow sanitization vs. a full release-tag parser.** D4 deliberately strips only punctuation and resolution tokens, not the full scene-release-tag vocabulary (codecs, sources, encoder groups) a dedicated filename parser would handle. That keeps the change small and testable and matches exactly what was asked; the cost is that a title like `My Title x264-GROUP` still reaches the provider with `x264-GROUP` attached. F48 (ADR-067) already owns real filename *parsing* into structured fields — once a file goes through that path, D2's earlier tiers (studio/title/performers/year, all clean structured values) take over and D4's sanitizer stops being the deciding factor. D4 exists specifically for the gap before that: a file with nothing but a messy title yet.

---

## Consequences

**What becomes easier**
- A configured provider (or an operator who knows their provider's search behavior) gets materially better match rates without any change to how the owner uses the Enrich picker.
- A future provider can ship a sensible default (`preferred_search_pattern`) with zero operator configuration required — the ADR-056-style "works out of the box" property.
- A freshly scanned, un-enriched file — the case with the least structured data and the most to gain — gets a cleaner default search query (D4) even with zero provider configuration at all.

**What becomes harder**
- One more piece of provider config to reason about when debugging a bad or missing enrichment match (was it the pattern, or the provider's own search?) — mitigated by the fail-soft/skip-and-log posture, so a bad pattern degrades to "acts like today," never a hard failure.
- `internal/enrich` gains a second small config-parsing surface (`query.go`) alongside `enrich.go` — kept intentionally thin (one grammar, four token names) to limit that cost.

**What we'll need to revisit**
- **Structured `hint.fields`** (D1-Option-B), if/when a second provider wants query control finer than a single formatted string.
- **Literal decoration in patterns** (e.g. `"{title} ({year})"`) — v1 is space-joined tokens only; real operator demand for punctuation/brackets would need a small grammar extension.
- **Person/Studio pattern support**, contingent on those entities gaining studio/year/performer-equivalent fields.
- **An owner-facing settings UI** for this knob (ADR-060's pattern), if YAML-edit-and-reload proves too heavy in practice.

---

## Action Items

1. [x] ADR-080 recorded; add to `docs/architecture/README.md`.
2. [x] `/write-spec` — [F53 spec](../specs/configurable-provider-search-patterns.md) (functional requirements, acceptance criteria for the picker's new default-query behavior).
3. [x] `/design-handoff` — [confirms zero `EnrichPicker.svelte` diff](../design/configurable-provider-search-patterns-handoff.md) beyond the seeded value (per D5); pins the exact content spec per scenario (incl. an empty-sanitization fallback the original spec pass missed) and specs the optional P1 transparency caption; [QA checklist](../design/configurable-provider-search-patterns-qa-checklist.md).
4. [ ] `/testing-strategy` — precedence-tier resolution (all 4 D2 tiers + fallthrough on a failed required token), optional-token omission, malformed-pattern fail-soft/skip, the D4 sanitizer (bracket/comma stripping, resolution-token regex word-boundary cases — e.g. a title that legitimately ends in a number+letter must not be mistaken for a resolution tag, whitespace collapse). Note D4 is the one **unconditional** behavior change in this ADR (§ below) — its golden cases should confirm sanitization is a pure improvement, never a worse query than the literal title.
5. [ ] **Implementation (HOLODEX-254)** — `enrich.Source.SearchPattern` + `fileConfig.DefaultSearchPattern`; `enrich.Manifest.PreferredSearchPattern`; `internal/enrich/query.go` (grammar + `BuildQuery` + `sanitizeTitle`); wire into `videoHint()`; `getMedia` response gains `enrich_queries`; SPA passes it into `EnrichPicker`'s existing `entityName` prop.
6. [ ] Provider-contract spec (`docs/specs/metadata-provider-contract.md`) §2.2 — document `preferred_search_pattern` (shape, defaults, unknown-key-safe, no protocol bump).
7. [ ] `/security-review` before merge — untrusted provider-advertised string (`preferred_search_pattern`) flows into a rendered query sent back out over HTTP; confirm sanitization posture matches ADR-056's precedent for untrusted `/describe` content.
