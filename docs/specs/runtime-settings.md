# Spec: Runtime owner-editable settings (F40)

**Status**: Draft
**Phase**: Phase 3 follow-up (operator/owner surface)
**Owner**: Project owner
**Date**: 2026-07-05
**Feature block**: **F40** — let the owner change selected operator settings **from the UI, at
runtime**, persisted and effective without a restart. Debuts with the **person gallery cap**
(`person_gallery_max`); ships the reusable mechanism, wires exactly one value.

**Issue**: [HOLODEX-140](https://whoiskevinrich.atlassian.net/browse/HOLODEX-140)
**ADR**: [ADR-059](../architecture/ADR-059-runtime-owner-settings.md) (the mechanism — generic
`settings` KV, typed registry, precedence layer above config, hot-reload, security posture)

**Depends on** (all shipped):
- config load + `PERSON_GALLERY_MAX` → `repo.SetGalleryCap` → `Repo.GalleryCapValue()` ([ADR-043](../architecture/ADR-043-gallery-cap-and-enrichment-suppression.md), [ADR-014](../architecture/ADR-014-configuration-and-data-layout.md))
- the owner gate (`requireOwner`, [ADR-030](../architecture/ADR-030-access-control-gating-seam.md)) and the existing `/admin/*` endpoints
- the `/owner` hub (`web/src/routes/owner/+layout.svelte`, tabs Status/Metadata keys/Trash) and `/capabilities` (already advertises `person_gallery_max`)
- golang-migrate ([ADR-016](../architecture/ADR-016-database-migrations.md))

**Touches the owner gate + a new persisted, owner-writable surface → a `/security-review` sign-off is
required before merge** (label `needs-security-review`).

---

## Problem Statement

Every operator setting in Holodex is **frozen at startup** — `holodex.yaml`/env is read once and
cannot change without editing a file and restarting the process. The owner wants to adjust the
per-person **gallery cap** (`person_gallery_max`, today 20) to hold more images, but the only path is
a config edit + restart, and there is **no settings surface in the app at all**: the `/owner` hub has
Status, Metadata keys, and Trash — none of which edit settings. The cost of not solving it: routine
tuning requires shell access to the deployment and a bounce, which is disproportionate for a
single-owner media server the owner otherwise runs entirely from the browser.

## Goals

1. **Owner changes the gallery cap from the UI and it takes effect immediately** — no restart, and
   the value persists across restarts.
2. **A reusable settings seam** — the next operator preference (card layout, delete-grace, thumbnail
   dims) is added as a registry entry + a refresh hook, not a new subsystem.
3. **Startup config still works and still provisions** — an instance with only `holodex.yaml`/env and
   no owner overrides behaves exactly as today; the config value is the default the owner sees and
   the "reset" baseline.
4. **No hot-path regression** — reading the effective cap stays an in-memory field load (no per-read
   DB hit on gallery insert or `/capabilities`).
5. **Contained blast radius** — only a typed allowlist of keys is writable, values are validated
   server-side, and the whole surface is owner-gated.

## Non-Goals

- **A general config editor.** F40 exposes only keys on the in-code registry (debut: one). Arbitrary
  `holodex.yaml`/env keys are **not** editable. *(Why: most config is deploy-time provisioning, not
  runtime preference; a typed allowlist is the security boundary.)*
- **Multi-user / roles.** Single-owner model (ADR-030) stands; "settings" means *the owner*. No admin
  tier, no per-user prefs.
- **Rewriting `holodex.yaml` from the app.** The override lives in the DB, separate from the operator's
  deploy config. *(Why: avoids racing env precedence and mutating read-only mounts — ADR-059 Option C.)*
- **Env as a runtime lock.** An explicitly-set env var seeds and resets but does **not** beat a written
  override. *(Why: single owner sets both; the UI is the most recent specific intent — ADR-059.)*
- **Raising per-enrich image yield.** F40 lets the *stored accumulation* cap grow; a single provider
  enrich is still bounded by the sidecar's own cap (TMDB `maxPersonPhotos = 20`) because `/enrich`
  carries no count. That is a **separate protocol change**, filed as a follow-up (see Open Questions).

---

## Users & Value

- **Owner**: raises (or lowers) the gallery cap from `/owner/settings` and sees it apply live — no SSH,
  no restart. Gains a home for future tunables.
- **Operator** provisioning a fresh instance: unchanged — `PERSON_GALLERY_MAX` still sets the starting
  value; the owner can later override it in-app.
- **Visitor**: unaffected; the gallery renders against whatever the effective cap is (already surfaced
  via `/capabilities`).

---

## Functional Requirements

### Must-Have (P0)

#### FR1 — Generic settings store (migration `0021`)

A `settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL DEFAULT (datetime('now')))`
table (ADR-059 §1). Additive; `.down.sql` drops it. **No seed rows** — an absent row means "no
override." Generic string KV; typing lives in the registry (FR3), not the schema.

- **Given** a fresh DB, **when** migrations run, **then** `settings` exists and is empty, and every
  effective setting equals its config seed.

#### FR2 — Override precedence + hot-reload

Effective value = `DB override ?? config seed (yaml/env) ?? built-in default` (ADR-059 §2). The
in-memory field (`Repo.galleryCap`) stays the read path; the write path updates the DB row **and**
refreshes the field atomically under `writeMu`; on boot, after the existing config seed, a new
`repo.LoadSettingOverrides()` re-applies persisted overrides.

- **Given** an owner sets the cap to 50, **when** the next gallery insert or `/capabilities` runs,
  **then** the effective cap is 50 with no restart and no added DB read on that path.
- **Given** the cap was set to 50, **when** the process restarts, **then** the effective cap is still
  50 (loaded from `settings`).
- **Given** no override row exists, **when** the value is read, **then** it equals the config seed
  (or the built-in default if the seed is unset) — identical to today.

#### FR3 — Typed registry (allowlist)

An in-code `settings.Registry` declares each editable key with `type`, config-backed `default`,
validation (`min`/`max`/enum), and UI `label`/`help` (ADR-059 §4). **Debut registry has exactly one
entry**: `person_gallery_max` (int, default = config seed, `min 1`, `max 500`, label "Gallery images
per person"). Only registered keys are writable.

- **Given** a write to a key **not** in the registry, **then** it is rejected (400) and nothing is
  persisted.
- **Given** a write with a value outside the key's bounds or of the wrong type, **then** it is
  rejected (400/422) and the stored value is unchanged.

#### FR4 — Owner-gated settings API

- `GET /admin/settings` → the registry joined with current effective values (`key, value, default,
  type, min, max, label, help`), owner-gated.
- `PUT /admin/settings/{key}` → body `{ "value": <typed> }`; validate → upsert → refresh field →
  return the new effective value.
- `DELETE /admin/settings/{key}` → remove the override ("reset to default"); effective falls back to
  the config seed.
- **Given** an unauthenticated caller, **when** any of these are hit, **then** `401` (matches the
  existing `/admin/*` gate) and no state changes.

#### FR5 — `/owner/settings` tab

A fourth `/owner` tab, **Settings**, reads `GET /admin/settings` and renders one control per registry
entry from its schema (today: a number input for the gallery cap, showing default + bounds, with a
"Reset to default" affordance calling `DELETE`). Token-themed only (no hardcoded palette/radii);
loading / error / saved states themed; **QA all three skins** (Cinémathèque, Broadcast, Brutalist).

- **Given** the owner edits the cap and saves, **then** the new value is shown as effective and a
  subsequent person page reflects it (the `/capabilities` value updates).
- **Given** a save fails validation, **then** a themed inline error using `text-warn`/`border-warn`
  is shown and the prior value stands.

### Nice-to-Have (P1)

- **P1-a** — a second registry entry (e.g. `card_layout`) wired to prove the seam with a non-int
  (enum) type. *Not required for F40 to ship; it is the proof the mechanism generalized.*

### Future Considerations (P2)

- **P2-a** — per-key `locked` flag in the registry so a hosted/multi-tenant build could let env pin a
  value against UI edits (ADR-059 sub-decision escape hatch). Design the registry struct so this is
  additive.
- **P2-b** — change-audit (who/when) beyond `updated_at`. Single-owner makes "who" trivial today.

---

## Acceptance Criteria

1. Owner sets `person_gallery_max` to 50 in `/owner/settings` → the gallery accepts up to 50 `extra`
   images for a person and `/capabilities.person_gallery_max` returns 50, **without a restart**.
2. That value survives a process restart (persisted in `settings`).
3. "Reset to default" removes the override → the effective value returns to the `PERSON_GALLERY_MAX`
   config seed (or 20 if unset).
4. A write to an unknown key, an out-of-bounds value, or a wrong-typed value is rejected server-side;
   nothing persists.
5. All four `/admin/settings` operations are owner-gated (`401` unauthenticated).
6. Reading the effective cap on the gallery-insert path and `/capabilities` performs **no** new
   per-call DB read (in-memory field remains the read path).
7. An instance with only config (no override row) is byte-for-byte behaviorally identical to pre-F40.
8. The Settings tab renders with tokens only across all three skins in loading/error/saved states.

---

## Test Notes (for `/testing-strategy`)

- **Migration** — `0021` up creates the empty table; down drops it; effective values equal config seed
  on a fresh DB.
- **Registry validation** — reject unknown key; reject out-of-bounds / wrong type; accept in-bounds.
- **Precedence + round-trip** — `DB ?? config ?? default` for each registered key; write → simulate
  reload (`LoadSettingOverrides`) → effective reflects the override; delete → falls back to seed.
- **Hot-path** — assert the gallery insert and `/capabilities` read the in-memory field (no DB read
  added); `GalleryCapValue()` behavior unchanged when no override.
- **API** — owner-gating (`401` unauth) on GET/PUT/DELETE; `PUT` returns the new effective value;
  `DELETE` resets; `/capabilities` reflects the change.
- **SPA** — Settings tab reads the registry schema and renders the control; save/reset/validation-error
  paths; three-skin QA.
- **Backward compat** — golden: config-only instance produces identical behavior to pre-F40.

---

## Open Questions

- **[engineering] Sidecar per-enrich yield** — F40 caps *accumulation*, not per-enrich yield. Should a
  future change pass a desired image count on `/enrich` and raise/param the sidecar cap so one provider
  pull can fill a higher gallery cap? **Filed as the deferred follow-up** ([HOLODEX-142](https://whoiskevinrich.atlassian.net/browse/HOLODEX-142));
  non-blocking for F40. *(Resolved as out-of-scope here; tracked separately.)*
- **[engineering, non-blocking] Max ceiling** — is `max 500` the right hard bound for the gallery cap,
  given per-image normalization + storage cost? Tunable in the registry without a migration.

---

## Timeline Considerations

Single feature block: one migration, one `internal/settings` package, a repo method pair + boot hook,
three endpoints, one SPA tab. No flag needed — absence of overrides is a no-op by construction. Ship
behind the existing owner gate after `/security-review` on the implementation diff.
