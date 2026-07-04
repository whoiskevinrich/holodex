# ADR-056: Provider field render hints + presence-driven auto-registration of non-canonical fields

**Status:** Proposed
**Date:** 2026-07-04
**Deciders:** Project owner

**Extends:** [ADR-033](ADR-033-metadata-source-plugins.md) (provider contract + `entity_enrichment` shadow store — adds an optional, additive `/describe.field_hints` channel and a persisted per-provider hint store) · [ADR-013](ADR-013-metadata-field-mapping.md) (canonical field mapping + the `Display` render hint that lived only in the code registry — this ADR lets the mapping and a provider carry it too) · [ADR-052](ADR-052-baseline-source-contract.md) (entity-agnostic `ResolveFields` — auto-registration rides the same core for video/person/studio) · [ADR-039](ADR-039-provider-asset-urls.md) (asset-host allowlist — reused to gate provider-selected `image_url` rendering). **Relates to:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (per-field decisions — auto-registered fields are deliberately **outside** the decision/curation model; they are display-only) · [ADR-055](ADR-055-enrichment-unique-key-invariant.md) (untrusted-provider perimeter — hints are untrusted input, sanitized/validated on the same posture). **Spec:** [Provider render hints (F39)](../specs/provider-render-hints.md). **Contract:** [metadata provider contract](../specs/metadata-provider-contract.md) §4.7 (added here). **Issue:** [HOLODEX-128](https://whoiskevinrich.atlassian.net/browse/HOLODEX-128).

---

## Context

A metadata provider advertises the field keys it can supply in `GET /describe` (`fields[]`) and returns
their values in `/enrich`. The **canonical** vocabulary — the keys with an accurate label, a render mode,
and a place in the page — lives in a **code-level registry** (`internal/registry/registry.go`,
`FieldDef{Canonical, Label, Display, Description}`). A key **outside** that registry is *non-canonical*.

The provider contract already *permits* non-canonical keys and even flags the gap in its own "Open items":

> any key outside the recommended person set needs a Holodex label + precedence entry to render/order well;
> propose the key rather than inventing display-only keys.

So a provider with a materially richer attribute set than the canonical registry covers degrades to a pile
of unlabeled, unordered extra rows — and only if an **operator** hand-authors `metadata-mappings.yaml`
entries per field, per source. There is no way for a **provider** to communicate presentation intent for the
fields it already advertises.

### Current state (survey, 2026-07-04)

| Layer | Today |
|---|---|
| `/describe` manifest (`enrich.Manifest`) | `fields []string` — flat key list, advisory. No per-field label/render/order. |
| Code registry (`registry.FieldDef`) | `Canonical, Label, Display, Description`. **No ordering field at all.** `Lookup(key)` returns a title-cased `Label` + empty `Display` for unknown keys. |
| Render vocabulary (`Display`) | `""` (inline text) · `long_text` · `url` · `image_url`. Fixed per canonical field; **not** overridable per mapping. |
| Mapping (`mapping.Field`) | `Canonical, Label, Sources, Multi, Merge, Casing, Browse, Filterable`. **No `Display`.** Render mode always comes from the registry. |
| Resolver (`ResolveFields`) | Iterates **only the configured/synthesized `[]mapping.Field`**; sets `Display: registry.Lookup(f.Canonical).Display`. A shadow key with no field is never emitted. |
| **Video** fields | Operator-shaped via `metadata-mappings.yaml`. A non-canonical provider key renders **only** if the operator maps `provider:key`. |
| **Person / Studio** fields | Synthesized in code (`personFields`, studio equivalent) from a **fixed canonical list**. A non-canonical provider key is **stored in the shadow store but never rendered**, and there is **no** operator remap path. |
| SPA render | `web/src/routes/{media,people,studios}/[id]/+page.svelte` switch on `f.display`. Media already handles `image_url`/`long_text`/`url`; person handles `url`/`long_text`. |

The person/studio row is the sharp edge: for those entities auto-registration is not a convenience, it is the
**only** path that makes a non-canonical field visible at all.

### Forces

- **Zero per-operator config.** The headline goal (HOLODEX-128 AC): a rich provider's non-canonical fields
  render with correct labels and reasonable ordering **out of the box**.
- **Additive, no protocol bump** (AC). The contract's "unknown keys ignored" rule must carry the new channel:
  an old provider omits it, an old Holodex ignores it, protocol stays v1.
- **Operator override is sacred** (AC). An operator `metadata-mappings.yaml` entry must still win over any
  provider-supplied hint. The **code registry** (the shared schema contract) must also win — a provider must
  not be able to relabel `bio` or force `poster_url` to render as text.
- **Providers are untrusted** ([ADR-033](ADR-033-metadata-source-plugins.md)/[ADR-055](ADR-055-enrichment-unique-key-invariant.md)).
  A hint's label/render/group is external input: sanitize it, validate against an allowlist, and never let it
  widen the fetch/navigation surface without the operator's existing trust decision.
- **The read path does not call the provider.** A person-detail `GET` resolves fields without contacting the
  sidecar, so the hints must be available at render time from local state — not fetched inline.
- **Entity-generic** ([ADR-052](ADR-052-baseline-source-contract.md)). Whatever we build should ride the one
  `ResolveFields` core for video, person, and studio, not fork per entity.
- **Don't over-reach into editing.** Making non-canonical fields *editable* (source decisions, curation) would
  drag in the whole F36/F30 apparatus per key. The goal is to *render them well*, not to curate them.

---

## Decision

Add an **optional, additive `field_hints` map to `/describe`**; **persist** each provider's hints locally;
and **auto-register** — presence-driven — any non-canonical shadow field into the resolved output as a
**display-only** row, with label/render/order resolved through a four-tier ladder that keeps operator and
code-registry authority above the provider. Four sub-decisions (owner, via question cards, 2026-07-04).

### D1 — Hint carriage: an additive `field_hints` map in `/describe`

`/describe` gains one optional key, `field_hints`, an object keyed by field key. `fields[]` is **unchanged**
(still a flat `string[]`):

```json
{
  "provider": "acme",
  "protocol_version": 1,
  "entity_types": ["person"],
  "fields": ["bio", "birthdate", "gender", "trivia", "credited_as", "home_page"],
  "field_hints": {
    "gender":      { "label": "Gender",            "render": "text",      "group": "attributes", "order": 10 },
    "credited_as": { "label": "Also credited as",  "render": "chips",     "group": "attributes", "order": 20 },
    "trivia":      { "label": "Trivia",            "render": "long_text", "group": "extended" },
    "home_page":   { "label": "Home page",         "render": "url",       "group": "extended" }
  }
}
```

Each hint object (all keys optional; **unknown keys ignored**, mirroring the asset-object rule):

| Key | Type | Meaning | Invalid / absent → |
|---|---|---|---|
| `label` | string | Display label | sanitized (control chars stripped, ≤64 chars); empty/absent → title-cased key |
| `render` | string | Render mode: `text` \| `long_text` \| `chips` \| `url` \| `image_url` | unknown/absent → `text` |
| `group` | string | Ordering band: `primary` \| `attributes` \| `extended` | unknown/absent → `extended` (lowest) |
| `order` | integer | Secondary sort within a group | absent → `0`; ties break by advertise order, then key |

**Chosen over** (a) making `fields[]` polymorphic (`string | object`) — rejected: an **old** Holodex
decoding `fields` as `[]string` breaks on an object element, so it is *not* backward-compatible; (b) a parallel
`field_specs[]` array of objects — rejected: duplicates `fields[]` and invites the two lists to drift. A
keyed map is the contract's existing extension idiom (unknown keys ignored) and cannot break an old decoder.

No protocol bump: `field_hints` is additive and forward-compatible under the contract's §2.2 rule.

### D2 — The four-tier resolution ladder (who wins label / render / order)

For a given field key, label/render/order resolve top-down, first tier with an answer wins:

1. **Operator `metadata-mappings.yaml`** — explicit deployer intent. Always wins (AC).
2. **Code registry** (`registry.FieldDef`) — the shared schema contract. A provider hint **cannot** touch a
   canonical key: `bio`, `poster_url`, `logo`, … keep their registry label/render regardless of what a
   provider advertises.
3. **Provider `/describe` hint** — **non-canonical keys only**. This is the new tier.
4. **Title-case fallback** — today's floor (`registry.Lookup`): humanized label, `text` render, no order.

The load-bearing rule that keeps the contract stable: **provider hints are consulted only for keys the code
registry does not know.** A hint on a canonical key is silently ignored (tier 2 shadows it).

### D3 — Persist provider hints (read-path availability)

Holodex persists each provider's advertised hints in a small table, refreshed whenever it fetches `/describe`
during an owner action (enrich / match / `reload-config`). The **read path reads only the persisted table** —
no inline provider call. Chosen over an in-memory cache (which renders the title-case fallback on a cold start
after restart until the cache warms): persistence gives stable rendering across restarts, and the manifest is
tiny. Cost: one migration (`provider_field_hints`).

### D4 — Presence-driven, display-only auto-registration; all three entities

- **Trigger = presence.** A non-canonical field is auto-registered (rendered) **only when the entity actually
  has a stored value** for it in the shadow store. No empty/placeholder rows; a field appears the moment an
  enrich populates it. (Chosen over advertised-driven, which would render empty rows for advertised-but-absent
  keys.)
- **Predicate.** Auto-register a shadow key iff it is: present for the entity **and** not internal
  (`_`-prefixed, §4.2/§4.6 sidecars stay invisible) **and** not in the code registry (i.e. genuinely
  non-canonical) **and** not already produced by a mapping/synthesized field. An unmapped **canonical** key is
  deliberately *not* auto-surfaced — that respects an operator who chose not to map it and keeps the rule
  precisely "the registry doesn't know this key."
- **Display-only.** Auto-registered fields are **read-only rows** for owner and visitor alike — a label, the
  value(s) in the hinted render mode, and a provenance badge. They carry **no** `SourceSelect`, no source
  decision, and no value-level curation. An owner who wants to curate or re-source a non-canonical field
  **promotes** it by adding a `metadata-mappings.yaml` entry (tier 1), which makes it a first-class mapped
  field with the full F36/F30 controls. This keeps auto-registration out of the decision/curation stores
  entirely.
- **Ordering.** Canonical/mapped fields render first in their existing order; auto-registered fields follow,
  sorted by (group rank `primary < attributes < extended`, then `order`, then key). "Reasonable ordering out
  of the box," nudgeable by the provider's `group`/`order`.
- **All three entities at once** — video, person, studio — since person/studio have no render path today and
  the seam is shared.

### Mechanism (the seams)

1. **Contract** — `enrich.Manifest` gains `FieldHints map[string]FieldHint` (`json:"field_hints,omitempty"`);
   `FieldHint{Label, Render, Group string; Order int}`. Decoded, sanitized, and validated on ingest.
2. **Store** — migration `00NN_provider_field_hints`:
   `provider_field_hints(provider TEXT, field_key TEXT, label TEXT, render TEXT, hint_group TEXT, ord INTEGER, updated_at, PRIMARY KEY(provider, field_key))`. Refreshed per provider (delete-then-insert in one write txn under `writeMu`) each time `Describe` is read in an owner action.
3. **Overlay** — a small resolver-adjacent helper `registry`-style lookup that, given `(key, provider)`,
   returns the tier-3/4 `(label, display, group, order)` for a **non-canonical** key from the persisted hints,
   else the title-case floor. Tier 1/2 are unchanged (`mapping.Field` / `registry.Lookup`).
4. **Render hint on the mapping** — `mapping.Field` gains `Display string yaml:"display"`; `ResolveFields`
   sets `Display = f.Display` if non-empty else `registry.Lookup(f.Canonical).Display`. This lets both an
   operator mapping (tier 1) **and** a synthesized auto-registered field (tier 3) carry a render mode, with a
   single resolver change.
5. **Auto-registration step** — per-entity, after the canonical resolve, enumerate present non-canonical
   shadow keys (the predicate above), synthesize a display-only `ResolvedField` per key via the overlay, sort,
   and append. Rides `ResolveFields` output for all three entities (ADR-052).
6. **SPA** — each entity page's read-only branch switches on `f.display` including the new `chips` mode (a
   static pill list); auto-registered rows render there with a `ProvenanceBadge`. See the
   [design handoff](../design/provider-render-hints-handoff.md).

### Security (untrusted hints; the `image_url`/`url` widening)

Because D2 lets a provider hint select **`image_url`/`url`** (owner-approved via question card, 2026-07-04),
the perimeter must hold the same line ADR-039 draws for assets:

- **`image_url`** — the field's value(s) must clear the **asset-host allowlist** (the provider's `base_url`
  host or an operator `asset_hosts` entry, [ADR-039](ADR-039-provider-asset-urls.md)). A value on a
  non-allowlisted host **does not render as an image** (falls back to `text`), so a provider cannot make the
  viewer's browser beacon an arbitrary host. The operator's `asset_hosts` decision is the trust gate — never
  the provider response.
- **`url`** — `http`/`https` only, `rel="noopener noreferrer"`, `target="_blank"` (the existing
  `UrlValueList` behavior); non-http values fall back to text.
- **Label / render / group** are sanitized and allowlist-validated on ingest (control chars stripped, label
  capped, unknown render/group coerced to the safe default). Labels render as **escaped text** (Svelte), never
  HTML.
- **Reserved `_`-keys** never auto-register (they are never displayed, §4.2/§4.6).

Rendering provider-influenced URLs is why this change carries a **`/security-review`** gate before merge.

---

## Options Considered

### D1 — hint carriage shape

#### A — additive `field_hints` map keyed by field key (chosen)
**Pros:** cannot break an old decoder (`fields[]` stays `[]string`; unknown keys ignored); keyed, so no drift
with `fields[]`; matches the contract's asset-object extension idiom. **Cons:** two places name the field key
(`fields[]` and a `field_hints` key) — but the map is optional and a hint for a key not in `fields[]` is
simply inert.

#### B — polymorphic `fields[]` (`string | object`)
**Pros:** one list. **Cons:** an old Holodex decoding `[]string` **fails** on an object element — not
backward-compatible, violating the AC. Rejected.

#### C — parallel `field_specs[]` array of objects
**Pros:** richer than strings. **Cons:** duplicates `fields[]`; the two lists drift; more surface for no gain
over the map. Rejected.

### D2 — provider authority scope

#### A — hints for non-canonical keys only; code registry wins for canonical (chosen)
**Pros:** the shared schema contract stays stable (no provider can relabel `bio`); the change is exactly
scoped to the gap. **Cons:** a provider cannot "fix" a canonical field's label it dislikes — correct: that is
a maintainer decision, not a provider one.

#### B — hints may override canonical fields too
**Pros:** maximal provider expressiveness. **Cons:** two providers could fight over `bio`'s label; the
registry stops being a stable contract. Rejected — contradicts the "operator/registry win" force.

### D3 — hint availability on the read path

#### A — persist a `provider_field_hints` table (chosen)
**Pros:** stable rendering across restarts; read path is a pure local read; auditable. **Cons:** one migration
+ a refresh-on-describe write.

#### B — in-memory manifest cache
**Pros:** no migration. **Cons:** cold start after restart renders the title-case fallback until an owner
action warms the cache — a visible, confusing inconsistency for the same page. Rejected (the AC blesses the
fallback, but a *stable* result is worth one small table).

### D4 — auto-registration trigger + editability

#### A — presence-driven, display-only (chosen)
**Pros:** never shows empty rows; zero entanglement with the decision/curation stores; smallest diff; an
operator promotes to full editing via a mapping. **Cons:** a non-canonical field is not curatable until
promoted — acceptable (promotion is one YAML entry and is the explicit "I care about this field" signal).

#### B — advertised-driven and/or editable
**Pros:** fields appear before any value; full parity with canonical fields. **Cons:** empty placeholder rows;
drags F36 decisions + F30 curation into every non-canonical key (per-key stores, chips, writeback questions) —
a large surface for a "render it well" goal. Rejected.

---

## Trade-off Analysis

**Provider expressiveness vs. a stable schema contract.** The registry is the shared vocabulary the resolver,
the provider protocol, and the SPA all trust. D2 keeps it authoritative for canonical keys and lets a provider
speak **only** about keys the registry has no opinion on — so the contract gains reach without losing
stability. The four-tier ladder makes the precedence explicit and testable rather than emergent.

**Render reach vs. fetch/navigation surface.** Allowing `image_url`/`url` from a provider hint (D2/owner) is
the expressive win; the cost is that a provider could point the browser at a host. Reusing ADR-039's
`asset_hosts` allowlist for the image case means the widening rides an **existing** operator trust decision —
no new trust primitive, and a non-allowlisted image simply degrades to text. `url` inherits the existing
link-hardening. The net new surface is a link the user must click, on an operator-configured sidecar's data.

**Display-only now vs. editable later.** Making auto-registered fields read-only is the decisive
simplification: it keeps them out of `field_source_decisions` and `metadata_curation`, out of writeback, and
off the owner-mutation surface — the diff is a resolver append + a render branch, not a new editing model. The
promotion path (`metadata-mappings.yaml`) means nothing is foreclosed: a field an operator actually wants to
curate becomes fully first-class the moment they map it.

**Persistence vs. ephemerality.** One tiny table buys restart-stable rendering and a pure read path, versus a
cold-start flicker to the fallback. For a single-owner curated library where the same page should look the
same after a restart, the table earns its keep.

---

## Consequences

**What becomes easier**
- A provider with a rich attribute set is first-class with **zero operator config** — the headline goal.
- Person and studio finally render their non-canonical provider fields at all (today: stored-but-invisible).
- The render mode is no longer registry-only: an operator mapping can set `display:` (a latent F27 gap), and a
  provider can suggest one — both through the single `ResolveFields` `Display` change.
- Ordering exists as a concept (group/order), where before there was only declaration order.

**What becomes harder**
- The `/describe` ingest path now sanitizes/validates untrusted presentation strings (label/render/group) and
  gates `image_url` on the asset-host allowlist — a new untrusted-input surface (mitigated, and behind a
  `/security-review`).
- Contributors must respect the ladder: a provider hint must never shadow a canonical field; auto-registered
  fields must stay display-only (no decision/curation coupling).
- One more migration + a refresh-on-describe write in the owner-action path.

**What we'll need to revisit**
- **Promotion UX** — surfacing "promote this field to a mapping" as an owner affordance (today it is a manual
  YAML edit). Deferred; capture as a follow-up issue.
- **`chips` for canonical multi fields** — the new static-chip render mode could later back read-only canonical
  multi rows too; out of scope here.
- **Provider hint conflicts across providers** — when two providers hint the *same* non-canonical key
  differently, the persisted table is per-provider; the auto-registration overlay picks the value-supplying
  provider's hint (and, on ties, provider trust order). Confirm when a second real provider exists.

---

## Action Items

1. [x] ADR-056 recorded; added to `docs/architecture/README.md`.
2. [x] Provider-contract spec §4.7 — `field_hints` documented (shape, defaults, unknown-keys rule, no protocol
   bump, security posture); §2.2 manifest table + Open-items note updated.
3. [x] `docs/reference/canonical-fields.md` — presence-driven auto-registration + the provider-hint tier +
   the `chips` render mode noted; render-modes table updated.
4. [ ] **Implementation (F39, HOLODEX-128)** — `enrich.Manifest.FieldHints`; migration
   `provider_field_hints`; refresh-on-describe; the tier-3/4 overlay; `mapping.Field.Display` +
   `ResolveFields` propagation; the per-entity presence-driven auto-registration append; SPA `chips` render +
   auto-registered read-only rows with provenance. Gates: `/write-spec` (done — F39 spec),
   `/design-handoff` (done — handoff + QA checklist), `/testing-strategy` (done — §9 F39 block),
   `/security-review` (pending, before merge).
5. [x] `docs/testing-strategy.md` — F39 block added (ladder precedence, presence gate, `_`-key invisibility,
   `image_url` allowlist gate, display-only, backward-compat golden).
