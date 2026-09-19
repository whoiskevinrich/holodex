# ADR-102: Instance skin — the skin as owner-set instance identity, a `settings` store for library-owned settings, and a derived custom palette applied as inline custom properties

**Status:** Proposed
**Date:** 2026-09-19
**Deciders:** Project owner (brainstorm 2026-09-16; persistence model and the two residual questions confirmed 2026-09-19)

**Supersedes:** [ADR-021](ADR-021-frontend-theming-and-skins.md) **§5 only** ("Skin selection
persists; supersedes the dark/light toggle") and its first-paint consequence — the per-browser
`localStorage` preference, the header skin picker, and the hardcoded default. ADR-021 §1–§4 (semantic
tokens as custom properties, Tailwind utilities mapped to tokens, all visual difference in CSS under
`[data-theme]`, fonts bundled offline) **stand unchanged and are the substrate this ADR rides on**.

**Spec:** [F66 instance-skin.md](../specs/instance-skin.md) (RD1–RD10) ·
**Epic:** [HOLODEX-425](https://whoiskevinrich.atlassian.net/browse/HOLODEX-425)

## Context

ADR-021 made the skin a *viewer* preference: `theme.svelte.ts` reads and writes a `holodex-theme`
`localStorage` key, the header carries a three-chip picker, and the default is the string
`'cinematheque'` in the module. That was the right shape for F8.2 (a dark/light toggle generalized to
three skins) but it puts the control in the wrong hands for a self-hosted archive: the owner cannot
decide what visitors see, every fresh browser lands on the stock default, and an owner who has ever
touched the picker has a stale key that would mask any new default forever. The only route to a
custom palette is editing `app.css` and rebuilding the image.

Three facts about the existing system shape the decision:

1. **Every operator setting is YAML and reaches the SPA read-only.** `holodex.yaml` keys
   (`card_layout`, `films_enabled`, the purge window) are wired into the ungated `/capabilities`
   bootstrap payload (`internal/api/auth.go`). There is **no server-side settings store** — nothing
   the UI can write that the server reads back. `/admin/reload-config` re-reads
   `metadata-mappings.yaml` only; `holodex.yaml` is boot-time.
2. **A skin is ~15 custom properties plus 15 `[data-theme]`-gated flourish rules.** Of the color
   tokens, most are *pair partners* whose only job is to contrast with another token —
   `accent-ink`/`accent`, `warn-ink`/`warn`, `surface`/`surface-2`/`rule` against `bg`. Their values
   are hand-tuned and fragile: `--warn-ink` on Cinémathèque took HOLODEX-324 to reach 5.42:1 because
   *no* light ink can pass on that warn. Any surface that lets an owner type those partners
   recreates that bug class by construction.
3. **Inline `style` custom properties on `<html>` outrank every selector.** Tailwind's
   `@theme inline` block maps utilities to `var(--bg)` etc., so a custom property set inline on the
   root re-tints every component with no new stylesheet and no bundle rebuild — and, because the
   base's `data-theme` attribute is still present, all of its flourishes and fonts survive.

The brainstorm sized a ladder — **S** owner-set default · **L1** YAML palette · **L2** in-app
editor · **L3** skin library — and chose **S + L1**, with the owner's follow-up call that the
selector lives on an Owner page and the header picker goes. That call is what turns S from "one YAML
key" into the first UI-set, server-persisted operator setting, which is the cross-cutting part and
the reason this is an ADR.

## Decision

### D1 — The skin is instance identity: one server-held value, applied to every viewer

The active skin is a single value the owner sets and the server holds. `/capabilities` carries it to
every viewer (owner and visitor alike); the SPA applies it as `data-theme` on `<html>` on arrival.
There is **no viewer preference**: `theme.svelte.ts` no longer reads or writes a preference, and the
header picker is removed. A `localStorage` **paint cache** of the last server-applied value is
permitted — written from the server value after it is applied, read once before first paint to
avoid a default-skin flash on non-default instances — and is **never authoritative**: the server
value overwrites it on every load. When no value is stored the instance skin is `cinematheque`, so a
zero-config instance looks exactly as today.

*Why:* on a self-hosted archive "what visitors see" is the owner's decision, and a viewer override
contradicts it. Removing the preference also removes the stale-key trap in one stroke.

### D2 — A generic `settings` key/value table holds library-owned settings; YAML/env holds the deployment

Migration adds `settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)` with
`GetSetting`/`PutSetting` in the repo, writes serialized under the existing `writeMu`. v1 stores
exactly one key, `theme.active`. Validation of a value's domain lives in the handler that writes it,
not in SQL, so the key set grows without a migration.

The boundary this draws, for every future setting, is **ownership**, stated so an operator can answer
it without reading code:

- A value belongs in **`settings`** when it is **owned by the library** — it should survive a restore
  of `/data` on a new host, and it would be *identical* on a staging copy of the same archive. The
  active skin is the first such value; `card_layout`, the delete grace period and the gallery cap are
  the same kind and are candidates to follow it when they gain a UI.
- A value belongs in **YAML/env** when it is **owned by the deployment** — paths, ports, the owner
  token, provider perimeters (`metadata-sources.yaml`), field mappings — it *differs* per environment
  and is reproduced from `compose.yml` + the three YAML files, never from a DB restore.
- **Tie-break for library-owned values that are *authored* rather than chosen** (a structured block
  you write in an editor, not a pick from a closed set): they may stay in YAML until a UI exists to
  author them. `theme.custom` is exactly this case in v1 — library-owned by the test above, YAML
  because the editor (L2) is deferred; when an editor lands the palette moves to `settings`, and this
  ADR says so now rather than pretending the v1 split is principled.
- **Secrets and security perimeters are never `settings`** — nothing UI-writable may hold a token or
  widen an allowlist.
- **A pin is reserved, not built.** A `settings` key MAY later gain a YAML/env twin that, when set,
  wins and renders its control read-only ("set by config") — the Immich `IMMICH_CONFIG_FILE` model —
  so an infrastructure-as-code operator can reproduce an instance without a DB restore. v1 ships no
  pin; the first "the skin reset when I recreated the container" report is answered by adding
  `theme.active` as that twin, not by revisiting this decision.

Existing YAML keys are **not** migrated by this ADR — the table is generic on purpose, not a mandate.
One stated consequence the ownership test makes explicit: **a backup that omits the DB does not
reproduce the instance's appearance.** That was already true of everything in `/data`; it is now a
property the docs state.

*Why not write the choice back into `holodex.yaml`:* the app rewriting its own config file is a new
and fragile pattern (comments, ordering, a bind-mounted read-only file in Docker); a KV row under
the single-writer model is the existing repo idiom.

### D3 — Read through `/capabilities`, write through one owner-gated endpoint

`/capabilities` gains `theme: { active, custom | null }` — ungated, identical for every viewer, the
same channel `card_layout` already uses. The one write is `PUT /admin/theme {"theme": <id>}` behind
`requireOwner`; it validates the id against the shipped set plus `custom`, and refuses `custom`
(400) when no palette is configured. No `GET /admin/theme` — the value is already in the bootstrap
payload. Server-side resolution is `settings["theme.active"] ?? "cinematheque"`, with a stored
`custom` reported as `cinematheque` + `custom: null` when the palette is absent, leaving the row
intact so restoring the config restores the choice.

### D4 — A custom palette is a base skin plus five primaries; every other token is derived in CSS and applied inline

`holodex.yaml` `theme.custom` declares `name`, `base` (v1: `cinematheque` only) and five hex
colors — `bg`, `ink`, `accent`, `muted`, `warn`. Nothing else is configurable: fonts, radius and
flourishes are the base's. `app.css` gains a **derivation layer** that, when the five primaries are
present as custom properties on `<html>`, resolves `surface`, `surface-2`, `rule` (mix of `bg`
toward `ink`), `accent-ink` and `warn-ink` (chosen by the luminance of their partner), and
`logo-plate`/`logo-plate-ink`; with no primaries present the layer is inert and the three shipped
blocks render byte-for-byte as today.

Application is **inline custom properties on `<html>`** over the base's `data-theme` — set by the
SPA from `/capabilities.theme.custom.tokens`, cleared when a shipped skin is active. Inputs are
`#rgb`/`#rrggbb` only; the server parses to RGB and re-serializes, so **no owner-supplied string is
ever emitted as CSS text** — the surface is five colors, not a stylesheet.

**Acceptance gate for the model (spec R11):** Cinémathèque re-expressed as its five primaries plus
the derivation layer must match its hand-tuned block for every derived token within ΔE\* ≤ 2 in
computed styles. Failing tunes the derivation, never the tolerance. Broadcast/Brutalist become
eligible bases when they pass the same gate — they carry hand-tuned `muted`/`accent-ink` values and
literal-color flourishes (scanlines) that may not survive derivation, which is why v1 restricts.

### D5 — Validation posture: a bad palette is treated as absent; a low-contrast one is a WARN, never a refusal

A missing `theme.custom` block means no custom skin (three cards on the Appearance tab). A malformed
block logs one line naming the key and is treated as absent — **never a boot failure**. For a valid
palette the server computes WCAG contrast for `ink/bg`, `muted/bg`, `accent-ink/accent`,
`warn-ink/warn` (the ink partners derived by the same rule the CSS uses) and logs one `WARN` per pair
below 4.5:1, naming the pair and ratio; the palette is still applied. This is
[ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md)'s startup-WARN posture: owner's
server, owner's eyes, and a typo never takes the site down.

### D6 — The palette is restart-to-apply; the selection is instant

Editing `theme.custom` requires a server restart, the same posture as every other `holodex.yaml`
key; `/admin/reload-config` is **not** widened (it is scoped to `metadata-mappings.yaml` and widening
it is a separate decision). Selecting a skin on the Appearance tab is instant — optimistic apply,
then `PUT`, revert on failure — because it is a `settings` write, not config.

## Options Considered

### Persistence of the active skin

| | A — `settings` KV row **(chosen)** | B — write back into `holodex.yaml` | C — keep the viewer preference + `default_theme` in YAML |
|---|---|---|---|
| Who decides what visitors see | Owner | Owner | Each viewer; owner only sets the fresh-browser default |
| New pattern introduced | UI-set operator setting (generic table) | App rewrites its own config file | None |
| Restart to change skin | No | Depends (YAML is boot-time today → yes, or a new reload path) | No |
| Stale-`localStorage` trap | Gone | Gone | **Kept** — any viewer who touched the picker never sees the new default |
| Docker read-only config mount | Unaffected | Breaks the write | Unaffected |
| Size | S–M (migration + 2 repo funcs + 1 handler) | M (YAML round-trip preserving comments) | S |

### Applying a custom palette

| | A — inline custom properties on `<html>` **(chosen)** | B — runtime `/theme.css` generating a `[data-theme='custom']` block | C — build-time (rebuild the image) |
|---|---|---|---|
| Base flourishes/fonts survive | Yes — `data-theme` stays the base | Only if the block re-declares them | Yes |
| Runtime CSS generation | None | Yes — a CSS-text endpoint, ETag/caching, a second request before paint | None |
| Owner string reaches CSS text | Never (parsed to RGB) | Yes, must be escaped | n/a |
| Flash on cold load | Paint cache covers it | Extra request → worse | None |
| Fits "SPA baked into the image" | Yes | Yes | **No** — the whole point is no rebuild |

### Palette input surface

| | A — five primaries, rest derived **(chosen)** | B — the full ~15-token set | C — raw CSS block |
|---|---|---|---|
| Can the owner recreate HOLODEX-324 by hand | No — pair partners are derived | **Yes** | Yes |
| Expressiveness | "Cinémathèque, but mine" | Full | Unlimited |
| Security surface | Five hex values | Fifteen hex values | Arbitrary CSS (`url()`, `@import`) — a real surface even for a trusted owner |
| Needs a validation model | Contrast WARN on 4 pairs | Contrast WARN on every pair + the derivation problem moves to the owner | Unbounded |
| Evidence of demand | The one known user wants an accent | None | None |

### Where the selector lives

Considered and rejected in the brainstorm: a fourth "custom" chip in the header picker (keeps a
viewer control, contradicts D1) and "custom replaces its base's chip" (hides the stock skin). The
owner chose an Appearance tab on `/owner` and no header control.

## Trade-off Analysis

- **Instance identity vs. viewer choice.** The feature deliberately *removes* a capability (per-viewer
  skins). For a single-owner archive that is a simplification, not a loss; if a multi-user
  deployment ever wants per-viewer skins back, D1's paint cache is *not* the seam — a preference
  would be a new, explicitly authoritative key layered *over* the instance value, and D1 would be
  revisited.
- **A generic settings table for one key.** The table is the cross-cutting part and it ships with
  one row. The alternative — a `theme_active` column somewhere, or a one-off file — would need
  redoing the moment a second library-owned setting appears, and D2's ownership test is the useful
  output either way. Four rules were weighed for it: a *mechanism* rule (UI-set + single-valued +
  instant → `settings`) is circular — it makes the split an accident of which features got a UI;
  an *authored-vs-chosen* rule is legible but fuzzy for scalars; a *lock model* (DB + YAML pin, as
  Immich does) satisfies IaC operators but is a pattern commitment no one-row table earns yet. The
  ownership test is the one with an operator-visible consequence (what a restore brings back), so it
  is the test, authored-vs-chosen is the tie-break, and the pin is reserved in writing.
- **Derivation vs. expressiveness.** Five inputs cannot express Broadcast. That is accepted: the
  derived model is what makes a palette contrast-safe by construction, and the R11 gate is the
  honest test of whether "derived" is good enough — for Cinémathèque, which is the only base the
  owner's own use case needs.
- **Restart-to-apply for the palette.** A hot-reload path would be nicer but drags `holodex.yaml`
  into a reload model it has never had; the palette is edited rarely and the *selection* — the
  thing an owner will actually toggle — is instant.
- **Inline `style` on `<html>`.** Unusual but precisely the right specificity tool: it is the one
  place a runtime value can override every `[data-theme]` block without generating CSS. The cost is
  that the five primaries are visible in the DOM — which they are anyway, in `/capabilities`.

## Consequences

- **Easier:** an operator can make their instance look like theirs from five lines of YAML and one
  click; the three-skin QA rule gains one column ("and the custom override, when configured")
  rather than a new matrix; adding a library-owned setting later is one key and one handler, and the
  ownership test says which existing YAML keys are candidates.
- **Harder / changed:** `theme.svelte.ts` loses its preference (ADR-021 §5); any test or doc that
  assumed `localStorage['holodex-theme']` is a preference must change — the key is renamed
  (`holodex-theme-cache`) so nothing reads the old one by accident. `app.css` grows a derivation
  layer that must stay inert without inline primaries (R10) — a regression there shows in every
  shipped skin, so R11's computed-style diff is a permanent test, not a one-off.
- **Security:** one new owner-gated write (`PUT /admin/theme`) with a closed enum; the YAML → CSS
  path is hex-only by construction (D4). The security review should confirm the server never
  reflects an unparsed config string into `/capabilities`.
- **Revisit when:** an operator asks for an in-app editor (L2 — the `settings` table and the
  `custom.tokens` shape are designed so an editor writes the same five primaries to a `theme.custom`
  settings key that overrides YAML); another base passes R11; a multi-user deployment wants
  per-viewer skins (D1).

## Action Items

1. [ ] Migration `NNNN_settings` + `repo.GetSetting/PutSetting` (S1, HOLODEX-426)
2. [ ] `PUT /admin/theme` + `/capabilities.theme`; config parse/validate for `theme.custom` with the
   hex-only normalization (S1/S3)
3. [ ] `theme.svelte.ts`: server-applied skin, inline primaries, paint cache; header picker removed
   (S1/S2, HOLODEX-427)
4. [ ] `app.css` derivation layer + the R11 computed-style gate for Cinémathèque (S3, HOLODEX-428)
5. [ ] Boot-time contrast WARN on the four pairs (S3)
6. [ ] Docs: `configuration.md` **Appearance** section, `holodex.yaml.example`,
   `.claude/rules/frontend-theming.md` + `docs/design/theming.md` (D1 mechanism, widened QA rule)
7. [ ] ADR index: this row; mark ADR-021 "§5 superseded by ADR-102"
8. [ ] Security review of the new write and the config → payload path before the PR leaves Draft
