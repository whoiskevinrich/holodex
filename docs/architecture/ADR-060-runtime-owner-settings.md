# ADR-060: Runtime owner-editable settings — DB-backed override over startup config, debuting with the person gallery cap

**Status**: Proposed
**Date**: 2026-07-05
**Deciders**: Project owner
**Relates to**: [ADR-014](ADR-014-configuration-and-data-layout.md) (config strategy & precedence — this adds a layer *above* it), [ADR-043](ADR-043-gallery-cap-and-enrichment-suppression.md) (`PERSON_GALLERY_MAX` + `Repo.GalleryCapValue()` — the first value promoted), [ADR-030](ADR-030-access-control-gating-seam.md) (`requireOwner` gate — the write surface), [ADR-016](ADR-016-database-migrations.md) (migrations). Spec: [Runtime settings (F41)](../specs/runtime-settings.md) ([HOLODEX-140](https://whoiskevinrich.atlassian.net/browse/HOLODEX-140)). Follow-up: [HOLODEX-142](https://whoiskevinrich.atlassian.net/browse/HOLODEX-142) (sidecar per-enrich count, see §Consequences).

---

## Context

Holodex config is **startup-only**. `config.Load()` reads YAML + env once (ADR-014/027); the
values are then frozen in memory. `PERSON_GALLERY_MAX` (ADR-043) is representative: it flows
`config → repo.SetGalleryCap → Repo.galleryCap` (an immutable field), is enforced at
`InsertPersonImage` time, and is advertised read-only to the SPA on `/capabilities`. Changing
it requires editing `holodex.yaml`/env **and restarting the process**.

The owner wants to change the gallery cap **from the owner UI, at runtime**. Today that surface
does not exist: the `/owner` hub has three tabs — Status, Metadata keys, Trash — all of which are
status/discovery/mutation panels, **none of which edit operator settings**. There is no settings
table, no settings API, and no reload path. So "make the gallery count changeable in the owner
page" is not a config tweak — it requires introducing **the first owner-editable runtime setting
mechanism** in the product. This ADR decides that mechanism, and deliberately scopes the *debut
wiring* to the single `person_gallery_max` value so the framework proves out on one field before
others (card layout, delete-grace, thumbnail dims) are promoted onto it.

### Forces

- **The UI must actually take effect.** A slider the owner moves that a restart silently reverts
  is a lie. The written value has to become the *effective* value, hot, without a bounce.
- **Reads are hot-path.** `GalleryCapValue()` is called on every gallery insert and every
  `/capabilities` hit. The mechanism must not add a DB read per call — the in-memory field that
  exists today stays the read path.
- **Don't erode ADR-014.** Deploy-time config (env/yaml) is how an operator provisions an
  instance. A runtime override layer must compose with it predictably, not fight it.
- **Single-owner model (ADR-030).** There is no admin tier; every write is already owner-gated.
  "Editable settings" means *the owner*, through `requireOwner` — no new identity class.
- **Proportionality.** This is one number today. The mechanism should be a thin, reusable seam,
  not a settings subsystem — but also not a bespoke one-off that the second setting has to rebuild.

## Decision

**Add a generic DB-backed key/value settings store that the owner edits through an owner-gated
API and a new `/owner/settings` tab. A written setting overrides the startup-config value as a
new top precedence layer; the in-memory field stays the hot read path and is refreshed on write
and on boot. A small in-code registry declares which keys are settable, their type, bounds, and
config-backed default — the debut registry has exactly one entry: `person_gallery_max`.**

### 1. Storage — one generic KV table (migration `0021`)

```sql
CREATE TABLE settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,          -- stringified; typed via the registry
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Additive; the `down` drops it. No seed rows — **absence of a row means "no override"**, so the
effective value falls through to config (below). A generic string KV keeps the table itself
setting-agnostic; typing and validation live in code, not the schema.

### 2. Precedence — a runtime override layer *above* startup config

Effective value resolves as a three-tier fallback (extends ADR-014's env-over-yaml precedence
with a new top layer):

```
effective = DB override  ??  config seed (yaml/env)  ??  built-in default constant
```

- **Built-in default** — the code constant (`repo.GalleryCap = 20`), unchanged.
- **Config seed** — `PERSON_GALLERY_MAX` / `person_gallery_max`, loaded at startup exactly as
  today. This becomes the **provisioning default and the "reset" baseline**, not the ceiling.
- **DB override** — a `settings` row written by the owner. **When present, it wins.**

Rationale: the owner's live UI action is the most recent, most specific intent; it *must* take
effect or the feature is hollow. Env/yaml keeps its ADR-014 job — bootstrapping a fresh instance
and providing the value the owner sees before they ever touch it. **"Reset to default" deletes the
row**, dropping back to the config seed. (Alternative — env as a hard override that beats the DB —
considered and rejected below.)

### 3. Hot-reload — DB is durable truth, the in-memory field is the hot cache

Reads do **not** touch the DB. The existing `Repo.galleryCap` int stays the read path
(`GalleryCapValue()` unchanged). The settings write path updates **both**, atomically under the
existing single-writer `writeMu`:

1. validate against the registry (bounds/type);
2. upsert the `settings` row;
3. refresh the in-memory field (`repo.SetGalleryCap(n)`).

At boot, after `config.Load()` seeds the fields as today, a new `repo.LoadSettingOverrides()` pass
reads the `settings` table and re-applies any overrides into their in-memory fields. So a restart
preserves the owner's value (it's in the DB), and steady-state reads stay a bare int load — zero
added hot-path cost. `/capabilities` continues to surface `person_gallery_max` from
`GalleryCapValue()` and now reflects the override with no change to the endpoint.

### 4. Registry — a typed allowlist of settable keys (not arbitrary KV)

A small in-code `settings.Registry` declares each editable key: `key`, `type` (int/bool/enum/string),
`default` (sourced from config), validation (`min`/`max`/enum members), and UI `label`/`help`. It
serves three jobs: (a) **security** — only registered keys are writable, so the endpoint can never
be used to set arbitrary rows; (b) **validation** — bounds enforced server-side before the upsert;
(c) **schema for the UI** — the settings page renders controls and the "default" hint from the
registry, not hard-coded markup.

**Debut registry = one entry**, `person_gallery_max` (type int, default = config seed, `min 1`,
`max` a sane ceiling e.g. 500 to bound enrichment/storage blast radius, label "Gallery images per
person"). The framework is general; the wiring is one field. Adding the *next* setting is a
registry entry + its own in-memory refresh hook — not new plumbing.

### 5. API — owner-gated, consistent with the existing `/admin/*` surface

- `GET /admin/settings` — returns the registry joined with current effective values (key, value,
  default, type, bounds, label, help). Owner-gated (`requireOwner`), matching `/admin/status`,
  `/admin/rescan`, `/admin/reload-config`.
- `PUT /admin/settings/{key}` — body `{ "value": <typed> }`; validates against the registry,
  upserts, refreshes the field; returns the new effective value. `DELETE /admin/settings/{key}`
  removes the override ("reset to default").

The SPA adds a fourth `/owner` tab, **Settings**, that reads `GET /admin/settings` and renders one
control per registry entry (today: the gallery-cap number input with its default/bounds). Theming
follows the token discipline (no hardcoded palette); QA across all three skins.

## Options Considered

### Option A — Generic KV table + typed registry (chosen)

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low–Med — one table, one repo method pair, a tiny registry, 2 endpoints, 1 tab |
| Reusability | High — next setting is a registry entry + a refresh hook |
| Hot-path cost | Zero added — in-memory field stays the read path |
| Blast radius | Contained — registry allowlists keys; owner-gated; server-validated |

**Pros:** proportionate now (one field), reusable later (framework); durable across restart;
composes cleanly with ADR-014 as a new top layer; no per-read DB cost. **Cons:** slightly more than
a one-off (a table + registry rather than a single hard-coded endpoint).

### Option B — Bespoke single endpoint for the gallery cap only

| Dimension | Assessment |
|-----------|------------|
| Complexity | Low — one endpoint, persist the int somewhere |
| Reusability | **None** — the second setting rebuilds all of this |
| Hot-path cost | Zero |

**Rejected.** It still needs somewhere durable to persist the int (a table or a one-cell row), so it
is barely less code than the generic store — while guaranteeing that the *next* promoted setting
(card layout is already asked-for) re-litigates persistence, precedence, and UI from scratch. The
generic seam is the same effort with none of the re-work.

### Option C — Make `holodex.yaml` hot-reloadable / write the file back from the app

| Dimension | Assessment |
|-----------|------------|
| Complexity | Med–High — file watcher or app-writes-operator-file |
| Correctness | Poor — racy with env precedence; app mutates operator-owned deploy config |

**Rejected.** Writing back to the operator's `holodex.yaml` blurs deploy-time config and runtime
preference, races with env vars (which win in ADR-014 and can't be rewritten), and in Docker the
file may be a read-only mount. A separate DB override layer keeps operator provisioning and owner
runtime prefs cleanly separated. (`/admin/reload-config` already exists but only re-reads *metadata
mappings*, deliberately not config — this ADR does not change that.)

### Sub-decision — DB override vs. env as the hard override

Considered letting an explicitly-set `PERSON_GALLERY_MAX` **beat** the DB (env as an operator lock).
Rejected for the single-owner model: the same person sets both, and the one they touched *most
recently and most specifically* is the UI. Env-wins would make the slider silently no-op whenever an
env var happened to be set — the exact "lie" force #1 forbids. Env therefore seeds and resets; the DB
override wins when present. (If a genuine operator-lock need ever appears — e.g. a hosted multi-tenant
build — revisit with a per-key `locked` flag in the registry rather than blanket env precedence.)

## Trade-off Analysis

The dominant force is **"the written value must actually take effect, cheaply, and survive a
restart."** That rules out anything that doesn't persist (must survive restart → DB) and anything
that reads the store on the hot path (must stay cheap → keep the in-memory field, refresh on write).
Given a DB store is required anyway, the marginal cost of making it a *generic* KV + registry over a
*bespoke* one-off is a table and a small struct — paid once, amortized across every future promoted
setting, the first of which (card layout) is already requested. The only genuinely contestable axis —
override vs. env precedence — resolves cleanly under the single-owner model toward "most recent
specific intent wins," with a named escape hatch (per-key lock) reserved for a future that may never
arrive.

## Consequences

**Easier**
- The owner tunes the gallery cap from the UI, live, no restart; the value persists.
- Every future operator preference (card layout, delete-grace window, thumbnail max-dim) has a home:
  add a registry entry + a refresh hook. The `/owner/settings` tab grows a control for free.
- `/capabilities` already carries the value, so the propagation-to-SPA path is unchanged.

**Harder / newer**
- A new persisted surface the owner can change → a **spec (F41)** and, because it touches the owner
  gate, a **`/security-review`** on the implementation: registry allowlist enforced (no arbitrary-key
  writes), server-side bounds validation, `requireOwner` on both read and write, and value coercion
  (reject non-int, clamp to `[min,max]`). No new SSRF/asset surface.
- One more boot step (`LoadSettingOverrides`) and the discipline that every registry entry wires its
  own in-memory refresh — a setting that persists but isn't re-applied on boot would be a latent bug;
  a test asserts round-trip (write → restart → effective) per registered key.

**Unchanged / out of scope**
- **The sidecar per-enrich ceiling is not addressed here.** `person_gallery_max` bounds how many
  `extra` images *accumulate* (across enrichments/providers); a single provider enrich still yields
  ≤ the sidecar's own `maxPersonPhotos` (TMDB: 20), because `/enrich` carries no count parameter. So
  setting the cap to 50 permits 50 to accumulate but won't conjure 50 from one TMDB pull. Making
  per-enrich yield honor a requested count is a **separate protocol change** (add a count to the
  `/enrich` request + raise/param the sidecar cap) — filed as a follow-up HOLODEX issue, deliberately
  not blocked on by this ADR.
- ADR-014 precedence *within* config (env-over-yaml) is unchanged; this only adds a layer above it.
- ADR-043's over-cap owner-upload override and enrichment suppression are unaffected — the cap they
  reference is now the DB-overridable effective value, which is the intended behavior.

## Action Items

1. [ ] Migration `0021_settings.{up,down}.sql` — the `settings` KV table.
2. [ ] `internal/settings`: the typed `Registry` (debut entry `person_gallery_max`) + validation.
3. [ ] `repo`: settings upsert/delete/list + `LoadSettingOverrides()`; keep `GalleryCapValue()` as the hot read.
4. [ ] Boot wiring in `cmd/holodex`: after the existing `SetGalleryCap(cfg…)` seed, call `LoadSettingOverrides()`.
5. [ ] API: owner-gated `GET /admin/settings`, `PUT /admin/settings/{key}`, `DELETE /admin/settings/{key}`.
6. [ ] SPA: `/owner/settings` tab (fourth), token-themed, QA all three skins; render controls from the registry.
7. [ ] Tests: registry validation (reject unknown key, out-of-bounds, wrong type); repo round-trip (write → reload → effective); API owner-gating (401 unauth) + `/capabilities` reflects the override.
8. [ ] `/security-review` on the implementation diff (gate, allowlist, validation).
9. [ ] Update the ADR index (README) and the F41 spec; file the deferred sidecar-count follow-up issue.
