# Spec: Provider render hints + non-canonical field auto-registration (F39)

**Status**: Draft
**Phase**: Phase 3 enrichment follow-up
**Owner**: Project owner
**Date**: 2026-07-04
**Feature block**: **F39** — make a provider's **non-canonical** advertised fields first-class in the UI
with **zero per-operator mapping config**, by (a) letting a provider carry per-field render hints in
`GET /describe`, and (b) presence-driven **auto-registration** of any stored non-canonical shadow field into
the resolved output as a display-only row.

**Issue**: [HOLODEX-128](https://whoiskevinrich.atlassian.net/browse/HOLODEX-128)
**ADR**: [ADR-056](../architecture/ADR-056-provider-field-render-hints.md) (the decision + the four-tier
ladder + the persisted hint store + security posture)
**Design**: [provider-render-hints-handoff.md](../design/provider-render-hints-handoff.md)

**Depends on** (all shipped):
- the provider contract + shadow store ([F22](metadata-plugins.md) / [ADR-033](../architecture/ADR-033-metadata-source-plugins.md), `entity_enrichment`, `enrich.Manifest`, `GET /describe`)
- the canonical field registry ([F27](../reference/canonical-fields.md), `internal/registry`, `FieldDef{Canonical,Label,Display,Description}`)
- the entity-agnostic resolver ([ADR-052](../architecture/ADR-052-baseline-source-contract.md), `ResolveFields`, video/person/studio baselines)
- the asset-host allowlist ([ADR-039](../architecture/ADR-039-provider-asset-urls.md), `asset_hosts`) — reused to gate provider-selected `image_url`
- the read-only field render on each entity page (`web/src/routes/{media,people,studios}/[id]/+page.svelte`)

**Touches** the untrusted-provider perimeter and renders provider-influenced URLs → a **`/security-review`**
sign-off is required before merge (labelled `needs-security-review`).

---

## Problem Statement

The metadata-provider contract already permits a provider to advertise field keys in `GET /describe` that are
not part of the canonical registry. Some upstreams expose a materially richer attribute set than the canonical
vocabulary covers, so a meaningful share of a provider's advertised fields can be **non-canonical**.

Today a non-canonical advertised field:
- renders only via a **title-cased fallback label** derived from the key (`registry.Lookup`),
- has **no default ordering** relative to canonical fields,
- has **no render-mode hint** (plain text vs. paragraph vs. chips vs. link),

…and — worse than the ticket states — this only happens at all on **video**, where an **operator** first
hand-authors a `metadata-mappings.yaml` entry per field, per source. On **person** and **studio**, whose field
sets are synthesized in code from a fixed canonical list, a non-canonical field is **stored in the shadow store
but never rendered**, with no operator remap path. There is no mechanism for a **provider** to communicate
presentation intent for the fields it already advertises, so a rich provider degrades to a pile of unlabeled,
unordered, invisible-on-entities extra rows.

## Goals

1. **First-class out of the box.** A provider that advertises non-canonical fields (with hints) has them
   rendered with correct labels and reasonable ordering **with zero per-operator mapping config**, on all
   three entity types (video, person, studio).
2. **Provider expresses intent.** `GET /describe` optionally carries a per-field **label**, **render mode**,
   and **ordering group** for its advertised keys.
3. **Operator still wins.** An operator `metadata-mappings.yaml` entry overrides any provider-supplied hint;
   the code registry still owns every canonical key.
4. **Zero-impact when absent.** A provider that advertises no hints, and an entity with no non-canonical
   values, are **unaffected** — identical rendering to today. No protocol bump.
5. **Bounded blast radius.** Auto-registered fields are **display-only** (no source decisions, no curation, no
   writeback); the untrusted-hint surface is sanitized, validated, and — for `image_url` — allowlist-gated.

## Non-Goals

- **Editing non-canonical fields.** Auto-registered fields are read-only. An owner who wants source
  decisions / curation on a non-canonical field **promotes** it with a `metadata-mappings.yaml` entry, which
  makes it a first-class mapped field (the existing F36/F30 controls apply). *(Why: keeps auto-registration out
  of `field_source_decisions`/`metadata_curation`/writeback — a render change, not an editing model.)*
- **A promotion UI.** Surfacing "promote this field to a mapping" as an in-app affordance is a follow-up;
  promotion is a YAML edit today.
- **Auto-surfacing unmapped *canonical* provider fields.** If an operator declined to map `overview` on video,
  it stays hidden. Auto-registration is strictly for keys the code registry does **not** know. *(Why: respects
  operator intent; keeps the predicate precisely "non-canonical".)*
- **New render modes beyond the vocabulary below.** `text` / `long_text` / `chips` / `url` / `image_url` only.
- **Provider-supplied precedence over canonical fields.** A hint never reorders canonical fields; auto-
  registered fields always sort **after** them.

---

## Users & Value

- **Operator** running a rich provider: extra attributes show up labeled and ordered without editing YAML.
- **Provider author**: one optional `field_hints` block makes their extra fields present well everywhere,
  instead of asking every operator to configure each key.
- **Visitor / owner**: person and studio pages gain the provider's richer attributes (gender, trivia, credited-
  as, home page, …) as clean read-only rows with a "from `<provider>`" badge.

---

## Functional Requirements

### FR1 — `/describe` carries optional `field_hints`

The manifest gains an optional `field_hints` object keyed by field key (contract §4.7). `fields[]` is
unchanged (`string[]`). Each hint object has optional `label`, `render`, `group`, `order`; unknown keys are
ignored. See [ADR-056 §D1](../architecture/ADR-056-provider-field-render-hints.md) for the exact table and the
`acme` example.

- Holodex decodes `enrich.Manifest.FieldHints map[string]FieldHint`.
- **Sanitize/validate on ingest** (untrusted): `label` → strip control chars, trim, cap 64 chars; `render` →
  coerce to the vocabulary, unknown → `text`; `group` → coerce to `primary|attributes|extended`, unknown →
  `extended`; `order` → integer, default 0.
- A hint whose key is `_`-prefixed, or is a canonical registry key, is **ignored** (tiers 4-reserved / 2).

### FR2 — Persist provider hints (`provider_field_hints`)

Migration `00NN_provider_field_hints` (append-only, with `.down.sql`):

```sql
CREATE TABLE provider_field_hints (
  provider   TEXT    NOT NULL,
  field_key  TEXT    NOT NULL,
  label      TEXT    NOT NULL DEFAULT '',
  render     TEXT    NOT NULL DEFAULT '',
  hint_group TEXT    NOT NULL DEFAULT '',
  ord        INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT    NOT NULL,
  PRIMARY KEY (provider, field_key)
);
```

- Whenever Holodex reads `GET /describe` during an owner action (enrich / match / `POST /admin/reload-config`),
  it **replaces** that provider's rows in one write txn (delete-where-provider + insert current) under the
  single-writer `writeMu`.
- The **read path** (`GET /people/{id}`, `/studios/{id}`, `/media/{id}`) reads only this table + the shadow
  store — no inline provider call.

### FR3 — Four-tier label/render/order resolution

For any field key, resolve `(label, render/display, group, order)` top-down, first tier wins
([ADR-056 §D2](../architecture/ADR-056-provider-field-render-hints.md)):

1. Operator `metadata-mappings.yaml` (incl. the new `display:` — FR5).
2. Code registry (`registry.Lookup`) — for canonical keys; owns their label/render.
3. **Provider hint** (persisted `provider_field_hints`) — **non-canonical keys only**.
4. Title-case fallback — humanized label, `render=text`, `group=extended`, `order=0`.

A helper resolves tiers 3→4 for a non-canonical key given `(key, provider)`.

### FR4 — Presence-driven auto-registration (all three entities)

After the canonical/mapped fields are resolved for an entity, enumerate its shadow-store keys and append a
**display-only** `ResolvedField` for each key that satisfies **all**:

- has ≥1 stored value for this entity (presence gate), **and**
- is **not** `_`-prefixed (reserved sidecar, never displayed), **and**
- is **not** in the code registry (genuinely non-canonical), **and**
- is **not** already produced by a mapping/synthesized field (no double render).

Each synthesized field carries the tier-3/4 `label` + `display`, its value(s) from the shadow store (dedup +
multi-split as for a merge field when `render=chips`), a `WinningSource`/provenance = the supplying provider,
and **no** `Decision`/`Candidates`/`Items`-curation. Sorting: canonical/mapped fields keep their order;
auto-registered fields follow, ordered by (group rank `primary<attributes<extended`, then `order`, then key).

Entity seams:
- **Person** — extend `personFields`/`personResolved` (`internal/api/person_fields.go`) to append auto-
  registered fields from the person's enrichment rows.
- **Studio** — the studio equivalent (`internal/api/studios.go`).
- **Video** — the media-detail resolve path appends auto-registered fields from the video's enrichment rows,
  after the mapped fields.

### FR5 — Render mode on the mapping + resolver propagation

- `mapping.Field` gains `Display string yaml:"display"` (operator may set it; a synthesized auto-registered
  field sets it from the hint).
- `resolver.ResolveFields` sets `ResolvedField.Display = f.Display` when non-empty, else
  `registry.Lookup(f.Canonical).Display`. Single change; backward-compatible (empty `Display` = today).

### FR6 — SPA renders the hinted mode; auto-registered rows are read-only

Each entity page's **read-only** field branch switches on `f.display`:

| `display` | Render |
|---|---|
| `text` / *(empty)* | inline `Label: v1, v2` |
| `long_text` | block paragraph |
| `chips` | static pill list (border-rule pills) — **new** |
| `url` | `UrlValueList` (http/https link, `rel=noopener noreferrer`, new tab) |
| `image_url` | `<img>` — **only if the value clears the asset-host allowlist** (FR7); else fall back to `text` |

Auto-registered fields render in this read-only branch for **owner and visitor alike** (no `SourceSelect`, no
curation), each with a `ProvenanceBadge` for the supplying provider, in an "Additional details" grouping after
the curatable fields. See the [design handoff](../design/provider-render-hints-handoff.md) for placement,
tokens, and the three-skin QA.

### FR7 — Security (untrusted hints)

- `image_url` value must be on the asset-host allowlist (provider `base_url` host or operator `asset_hosts`,
  [ADR-039](../architecture/ADR-039-provider-asset-urls.md)); otherwise the image does **not** render (text
  fallback). The allowlist check happens server-side (the resolved field marks whether the value is
  allowlisted) or is enforced by the same URL policy the SPA already applies for `poster_url`/`logo`.
- `url` — `http`/`https` only; non-http → text.
- `label`/`render`/`group` sanitized/validated on ingest (FR1). Labels render as escaped text.
- `_`-prefixed keys never auto-register.

---

## Acceptance Criteria

(Mirrors and extends the HOLODEX-128 AC.)

1. A provider that advertises non-canonical fields **with hints** and returns values for them → those fields
   render with the hinted **label**, **render mode**, and **ordering** on video, person, and studio, with
   **zero** `metadata-mappings.yaml` config.
2. A non-canonical field **without** a hint but with a stored value → renders with the title-cased label +
   `text`, ordered after canonical fields (today's floor, now reached automatically on all entities).
3. An operator `metadata-mappings.yaml` entry for the same key → **overrides** the provider hint (label /
   render / order / sources), and the field becomes a first-class curatable mapped field.
4. A provider hint on a **canonical** key (e.g. `bio`) → **ignored**; the registry label/render stands.
5. A provider that advertises **no** `field_hints` → **no behavioral or visual change**; protocol stays v1.
6. A `_`-prefixed field (e.g. `_studio_external_ids`) → never auto-registers, never renders.
7. An `image_url` hint whose value host is **not** allowlisted → renders as text, **not** an `<img>`; an
   allowlisted host renders the image.
8. Auto-registered fields expose **no** owner editing controls (no source chips, no curation, no writeback);
   promoting via a mapping restores full controls.
9. Presence gate: an advertised non-canonical key with **no** stored value for the entity → **no** row.
10. All three skins (Cinémathèque, Broadcast, Brutalist) render every mode (text/long_text/chips/url/image_url)
    with tokens only, in loading/empty/populated states.

---

## Test Notes (for `/testing-strategy`)

- **Contract decode** — `field_hints` present/absent/partial/garbage; unknown render/group coerced; canonical
  and `_`-keys ignored; old manifest (no `field_hints`) decodes unchanged.
- **Ladder** — unit-test tier precedence: mapping > registry > hint > title-case; hint never shadows canonical.
- **Persistence** — describe refresh replaces a provider's rows; read path reads without a provider call.
- **Auto-registration** — presence gate; predicate exclusions (`_`-key, canonical, already-mapped); ordering by
  (group, order, key); rides `ResolveFields` for all three entities (extend the ADR-052 non-video baseline
  test).
- **Security** — `image_url` allowlist gate (allowlisted → img, other → text); label sanitize/cap; render/group
  coercion; `_`-key invisibility.
- **SPA** — the `chips` read-only renderer; auto-registered rows show a provenance badge and no controls;
  three-skin QA.
- **Backward compat** — the golden case: a provider with no hints, an entity with no non-canonical values →
  byte-identical resolved output to pre-F39.

---

## Rollout

Single feature block (all three entities), one migration, additive contract change (no protocol bump). Ship
behind the normal owner gate; no flag needed — absence of hints/values is a no-op by construction. Update the
reference stub (`testdata/enrich-stub/`) and the TMDB provider spec with a `field_hints` example so the
worked example exercises the new channel.
