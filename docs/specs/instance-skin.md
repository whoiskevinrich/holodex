# Spec: Instance skin — owner-set skin on an Appearance tab + custom palette from `holodex.yaml` (F67)

**Status**: Draft
**Phase**: Phase 3 (presentation / operability) — rides ADR-021's token substrate; adds one
operator setting and one config block, no new subsystem
**Owner**: Project owner
**Date**: 2026-09-19
**Feature block**: **F67** — the skin stops being a *per-browser viewer preference* and becomes
**instance identity** the owner sets. The header skin picker is removed; an **Appearance** tab on
`/owner` selects one of the three shipped skins or an owner-defined **custom palette** declared in
`holodex.yaml`. The selection persists **server-side** and ships to every viewer.

**Epic**: [HOLODEX-425](https://whoiskevinrich.atlassian.net/browse/HOLODEX-425) ·
stories [426](https://whoiskevinrich.atlassian.net/browse/HOLODEX-426) settings store + `/capabilities` ·
[427](https://whoiskevinrich.atlassian.net/browse/HOLODEX-427) Appearance tab + picker removal ·
[428](https://whoiskevinrich.atlassian.net/browse/HOLODEX-428) custom palette

**Depends on** (all shipped):
- Frontend theming and skins ([ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md),
  [`docs/design/theming.md`](../design/theming.md)) — the semantic token set
  (`bg surface surface-2 ink muted rule accent accent-ink warn warn-ink logo-plate logo-plate-ink
  font-display font-ui radius`), the three `[data-theme]` blocks in `web/src/app.css`, and the
  `[data-theme]`-gated flourishes (`.app-atmosphere`, `.video-frame`, `.skin-title`). This spec
  **supersedes the viewer-preference half** of ADR-021 (F8.2's `localStorage` persistence).
- Operator config → SPA bootstrap — `holodex.yaml` keys reach the SPA read-only through the ungated
  `/capabilities` payload (`card_layout`, `films_enabled`, …). F67 adds `theme` to it.
- Owner mode (F29, `requireOwner`) — every mutation in this spec is owner-gated.
- The `/owner` tab shell (`web/src/routes/owner/+layout.svelte`) — Appearance is its tenth tab.

**Related, not depended on**: F51 installable PWA ([pwa-support.md](pwa-support.md),
HOLODEX-234) — branding there is icon/favicon only; wiring the manifest's `theme_color` to the
active skin is a P2 below, not part of either epic.

**ADR**: [ADR-102](../architecture/ADR-102-instance-skin-and-settings-store.md) — D1 instance identity ·
D2 `settings` store + the library/deployment ownership boundary · D3 read/write channels · D4 derived palette applied
inline · D5 validation posture · D6 restart-to-apply (supersedes ADR-021 §5 only).
**Design**: [instance-skin-handoff.md](../design/instance-skin-handoff.md) +
[mockup](../design/instance-skin-mockup.svg) — option B (miniature browse preview cards, real
`.video-frame` tiles under `data-theme`) approved 2026-09-19; OQ1 resolved: nothing takes the header slot.

---

## Problem Statement

Holodex ships three skins but the choice is a per-browser `localStorage` preference with a
hardcoded Cinémathèque default: an operator cannot make their instance *look like theirs* — every
visitor and every fresh browser lands on the stock default, and the only way to change the palette
is to edit `app.css` and rebuild the image. For a self-hosted archive the skin is the instance's
identity, not a viewer's whim; the current model puts the control in the wrong hands and offers no
customization at all. Not solving it leaves every public-release instance visually identical and
keeps "Cinémathèque, but with my accent" a fork-the-CSS job.

## Goals

1. **The owner sets the skin for the instance, from the UI, once.** Every viewer (owner and
   visitor, every browser) sees the same skin; a fresh browser needs no interaction to land on it.
2. **An owner can declare a custom palette without touching frontend code or rebuilding.** Five
   colors in `holodex.yaml` on top of a shipped base skin produce a complete, contrast-safe skin.
3. **Contrast pairs stay structurally safe.** Every token that is a *pair partner* (`accent-ink`,
   `warn-ink`, surfaces, rules) is derived, never typed, so an owner cannot recreate the
   HOLODEX-324 class of bug by hand.
4. **Zero regression for the three shipped skins.** Cinémathèque stays the default; the
   three-skin QA rule and every `[data-theme]` flourish keep working unchanged.

## Non-Goals

- **An in-app palette editor** (color pickers, live preview, server-side save of the palette) —
  the brainstorm's L2 rung. Deferred until an operator other than the project owner asks; it is
  where persistence, undo and a design surface make the size explode for no evidenced demand.
- **Multiple named custom skins / import / export** (L3) — never, absent demand.
- **Per-visitor skin choice** — *removed*, not deferred. The skin is instance identity; a viewer
  control contradicts that model.
- **Fonts and radius as palette inputs** — the five bundled fonts are the offline guarantee; a
  font-family knob either breaks offline or needs an upload pipeline. Fonts/radius/flourishes
  come from `base`.
- **A light skin / `color-scheme: light`** — a different, larger project; all three skins and their
  flourishes assume dark. Custom palettes are dark-only in the sense that derivation assumes a
  dark `bg` and a light `ink`; the contrast WARN catches the inversion, it does not support it.
- **Raw CSS injection** (`theme.custom.css: …`) — turns a color feature into a security surface.
  Inputs are hex colors only.
- **A base other than Cinémathèque in v1** — Broadcast and Brutalist carry hand-tuned tokens and
  literal-color flourishes that may not survive derivation; widen only after the derivation gate
  (R11) passes for them.

## Resolved Decisions

Decisions taken in the 2026-09-16 brainstorm and its follow-up, recorded so implementation does
not re-litigate them.

| # | Decision | Alternative rejected | Why |
|---|---|---|---|
| RD1 | **Skin is instance identity, set by the owner, persisted server-side.** | Keep the per-browser preference and add an owner default (`default_theme` in YAML). | The owner choosing "what visitors see" is the whole point on a self-hosted archive; a viewer override contradicts it and the stale-`localStorage` trap (a viewer who ever touched the picker never sees the new default) disappears with it. |
| RD2 | **The header skin picker is removed. The control is an Appearance tab on `/owner`.** | Keep the header picker as a fourth "custom" chip. | One control, owner-only, where the other operator settings live. Visitors get no control (RD1). |
| RD3 | **Persistence is a `settings` key/value table — the first UI-set operator setting in Holodex.** | Write the choice back into `holodex.yaml`. | The app writing its own config file is a new and fragile pattern; a KV row under `writeMu` is the existing repo idiom. ADR-102 D2 draws the boundary by *ownership*: library-owned values (survive a `/data` restore) → `settings`; deployment-owned → YAML/env. The v1 palette stays YAML only because it is *authored* and the editor is deferred. |
| RD4 | **Custom palette = `base` skin + five primaries (`bg ink accent muted warn`); every other token is derived in CSS.** | Expose all ~15 tokens. | Pair partners typed by hand are exactly how HOLODEX-324 happened. Five inputs are enough for "Cinémathèque, but mine". |
| RD5 | **Custom palette is declared in `holodex.yaml` (`theme.custom`), applied at boot, restart to apply.** | A hot-reloadable file or a DB-stored palette. | Same posture as `card_layout`; `/admin/reload-config` covers only `metadata-mappings.yaml` today and widening it is not this feature. |
| RD6 | **Custom tokens are applied as inline `style` on `<html>` over the base's `data-theme`.** | A generated `[data-theme='custom']` stylesheet served at runtime. | Inline custom properties beat every selector, so the base's 15 `[data-theme]` flourishes and fonts stay for free; no runtime CSS generation, no bundle rebuild. |
| RD7 | **A contrast failure is a `WARN` at boot naming the pair, never a refusal.** | Refuse to load a failing palette. | Owner's server, owner's eyes; matches the `in_sync` mapping-WARN posture (ADR-093). |
| RD8 | **A `localStorage` *paint cache* is allowed; it is never authoritative.** | No `localStorage` at all. | Avoids a Cinémathèque flash on cold load of a non-default instance; the server value always overwrites it, so the preference model stays gone. |
| RD9 | **`base` is `cinematheque` only in v1.** | Any of the three. | See Non-Goals; widen after R11 passes for another base. |
| RD10 | **When the row is absent the instance skin is `cinematheque`.** | — | Unchanged default; zero-config instances look exactly as today. |

## User Stories

**Owner / operator**
1. As the owner, I want to pick the instance's skin on an owner page and have every viewer see it,
   so the archive looks the way I decided without each browser needing to be told.
2. As an operator publishing an instance, I want to declare my own accent and background in config
   so the instance reads as *mine* without forking the stylesheet.
3. As the owner, I want a wrong palette to still load and tell me which pair fails contrast, so a
   typo never takes the site down and I know what to fix.
4. As the owner, I want the custom palette to keep Cinémathèque's fonts, radius and film-frame
   flourishes, so "custom" means my colors, not a stripped-down skin.

**Visitor**
5. As a visitor, I want the site to render in the instance's skin on first paint of a cold load,
   so I never see a default skin flash before the owner's choice.
6. As a visitor, I want no skin control in the header, so the interface is the archive, not a
   settings panel.

**Edge / error**
7. As the owner, if `theme.custom` is malformed I want the block treated as absent (with a log
   line), so the Appearance tab simply shows three cards and the instance falls back to the stored
   shipped skin.
8. As the owner, if the stored skin is `custom` and I later remove `theme.custom` from config, I
   want the instance to fall back to the base skin (Cinémathèque) and the tab to show the stored
   value as unavailable, so a config edit never leaves the site unstyled.

## Requirements

### Must-Have (P0)

**R1 — `settings` store.** Migration adds `settings(key TEXT PRIMARY KEY, value TEXT NOT NULL,
updated_at TEXT NOT NULL)`; repo exposes `GetSetting(key) (string, bool, error)` and
`PutSetting(key, value)` serialized under `writeMu`. Manual down drops the table.
- [ ] Round-trips a value; a missing key returns `ok=false`, not an error.
- [ ] Concurrent `PutSetting` calls serialize (no `SQLITE_BUSY` under the existing writer model).

**R2 — Owner-gated skin setting.** `PUT /admin/theme` with body
`{"theme": "cinematheque"|"broadcast"|"brutalist"|"custom"}` stores `theme.active`.
- [ ] Visitor → the `requireOwner` failure (401/403 as today); no write.
- [ ] Unknown value → 400; `"custom"` when `theme.custom` is not configured → 400 with a message
      naming the missing config; no write in either case.
- [ ] Success → 200 with the new `theme` object (same shape as `/capabilities.theme`).

**R3 — `/capabilities.theme`.** The ungated bootstrap payload gains
`theme: { active: string, custom: { name, base, tokens } | null }` where `tokens` is the map of
the five primaries as written in config (hex, normalized to `#rrggbb`).
- [ ] `active` is the stored value, or `cinematheque` when no row exists (RD10).
- [ ] `active` is `custom` only if `custom` is non-null; a stored `custom` with no config reports
      `active: "cinematheque"` and `custom: null` (story 8).
- [ ] Visitors and owner receive identical `theme`.

**R4 — SPA applies the server skin; the viewer preference is gone.** `theme.svelte.ts` no longer
reads or writes a preference. On `/capabilities` it sets `data-theme` on `<html>` to `active`
(or to `custom.base` when `active === "custom"`) and, for `custom`, sets the five primaries as
inline custom properties on `<html>` (`--bg`, `--ink`, `--accent`, `--muted`, `--warn`); for any
other value it clears those inline properties.
- [ ] Switching from `custom` to a shipped skin leaves no inline `--*` residue on `<html>`.
- [ ] No code path writes a *preference* to `localStorage`; the paint cache (R8) is the only key.

**R5 — Appearance tab.** New route `/owner/appearance`, appended to the `/owner` tab list as
**Appearance**. It renders one selectable card per shipped skin and — when
`/capabilities.theme.custom` is non-null — a fourth card for the custom palette, labelled with
its `name` and "based on Cinémathèque". Each card is drawn in *its own* tokens (a swatch strip of
`bg / surface / ink / accent / warn` plus a display-font sample), so the choice is legible without
applying it. The active card is marked (`aria-pressed`/radio semantics per the design handoff).
- [ ] Selecting a card applies instantly (optimistic `data-theme` + inline-token flip) and
      persists via R2; on failure the previous skin is restored and the existing error idiom shown.
- [ ] Owner-only route (visitor is redirected/denied like the other `/owner` tabs).
- [ ] Tokens only; the cards' per-skin rendering uses scoped `data-theme` on the card, never
      hardcoded values.

**R6 — Header picker removed.** The skin picker in `web/src/routes/+layout.svelte` is deleted; no
viewer-facing skin control remains anywhere.
- [ ] `rg 'THEME_LABELS' web/src` finds only the Appearance tab.

**R7 — First paint.** A cold load renders in the instance's skin before `/capabilities` returns
whenever the paint cache (R8) holds a value; otherwise it renders Cinémathèque and switches on
arrival. Visible flash is bounded to the first-ever load of a browser.

**R8 — Paint cache.** After applying the server value the SPA writes it to `localStorage`
(`holodex-theme-cache`: `{active, base, tokens}`); on boot, before the first paint, it applies
that cache if present. The server value always overwrites it on arrival.
- [ ] A cache holding a stale custom palette is replaced, not merged, by the server value.
- [ ] Every read/write is wrapped so a blocked `localStorage` never throws (private windows).

**R9 — `theme.custom` in `holodex.yaml`.**
```yaml
theme:
  custom:
    name: "Rich Archive"     # label on the Appearance card; 1–40 chars
    base: cinematheque       # v1: cinematheque only (RD9)
    bg:     "#0b0a0c"        # page background
    ink:    "#efe9e0"        # primary text
    accent: "#c0483f"        # primary/active color
    muted:  "#9a9188"        # secondary text
    warn:   "#e2603f"        # error/attention — distinct from accent
```
- [ ] All five colors and `name` are required; `base` defaults to `cinematheque`.
- [ ] Colors accept `#rgb` / `#rrggbb` only (case-insensitive); anything else is a validation
      failure. Nothing here is ever emitted as raw CSS text — values are parsed to RGB and
      re-serialized.
- [ ] A missing block ⇒ no custom skin (three cards). A malformed block ⇒ one log line naming the
      key and the block is treated as absent — **never a boot failure** (story 7).
- [ ] Env override is not offered for the block (multi-value); documented as YAML-only.

**R10 — Derived tokens.** `app.css` gains a derivation layer so that, when the five primaries are
present on `<html>`, the remaining color tokens resolve from them: `surface`, `surface-2`, `rule`
from `bg` mixed toward `ink`; `accent-ink` and `warn-ink` chosen by the luminance of `accent` /
`warn`; `logo-plate` / `logo-plate-ink` from `ink` / `bg`. Fonts, radius and flourishes are the
base's.
- [ ] With no inline primaries the derivation layer is inert and the three shipped blocks render
      byte-for-byte as today (R12).
- [ ] The derivation uses `color-mix()` / relative color syntax already supported by the browsers
      the three skins target; no JS color math in components.

**R11 — Derivation acceptance gate.** Before S3 is marked done, Cinémathèque re-expressed as its
five primaries + the derivation layer must match the hand-tuned block for every derived token
within ΔE\* ≤ 2 (or an equivalent stated tolerance), measured from computed styles. Passing is
the evidence the derivation model is right; failing means the derivation is tuned until it passes
— not that the tolerance moves.

**R12 — Contrast check at boot.** For a configured palette the server computes WCAG contrast for
`ink/bg`, `muted/bg`, `accent-ink/accent`, `warn-ink/warn` (the ink partners derived by the same
rule as R10) and logs one `WARN` per pair below 4.5:1, naming the pair and the ratio. The palette
is still applied (RD7).

**R13 — Three-skin QA rule widened.** `.claude/rules/frontend-theming.md` and
`docs/design/theming.md` add "and the custom override, when one is configured" to the QA rule and
document RD6 (inline primaries over the base) as the mechanism.

**R14 — Configuration docs.** `docs/reference/configuration.md` gains an `## Appearance` section
(the `theme.custom` block, restart-to-apply, the WARN behaviour, the Appearance tab as the
selector) and `holodex.yaml.example` gains the commented block next to `card_layout`.

### Nice-to-Have (P1)

**R15 — Contrast hint on the custom card.** The Appearance card for a custom palette shows the
boot-time contrast result ("all pairs pass" / "muted on bg 3.9:1") so the owner sees the WARN
without reading logs. Requires `/capabilities.theme.custom.contrast` — small, but it is the only
piece of this spec that is UI-for-the-palette, so it can trail.

**R16 — Custom card reads "unavailable" when the stored skin is `custom` but config lost the
block** (story 8), instead of silently showing Cinémathèque as active.

### Future Considerations (P2)

- **In-app palette editor** (L2) — the settings store (R1) and `/capabilities.theme.custom` shape
  are designed so an editor would *write* the same five primaries to a `theme.custom` setting row
  that overrides YAML; nothing in v1 should assume the palette is YAML-only.
- **More bases** — R10/R11 are written per base; Broadcast/Brutalist become eligible when they
  pass R11.
- **PWA manifest `theme_color` / `background_color` from the active skin** (F51 cross-over).
- **Other operator settings migrating to the `settings` table** (e.g. `card_layout`) — out of
  scope; the table is generic on purpose but v1 stores exactly one key.

## Behavior detail

**Resolution order on the server.** `active = settings["theme.active"] ?? "cinematheque"`. If
`active == "custom"` and `theme.custom` is not configured (absent or malformed), the effective
`active` reported is `"cinematheque"` and `custom` is `null`; the stored row is left untouched so
restoring the config restores the choice.

**Apply order in the SPA.** Boot → apply paint cache (if any) → fetch `/capabilities` → apply
server `theme` → write paint cache. Applying means: set `data-theme` to the effective base; set
or clear the five inline primaries.

**Appearance tab optimistic flow.** Click → apply locally → `PUT /admin/theme` → on 2xx write the
paint cache; on error re-apply the previous `theme` object and show the error idiom. The tab
never shows a "save" button — selection *is* the save.

**Restart-to-apply for the palette.** Editing `theme.custom` requires a server restart; the
Appearance tab and `/capabilities` reflect the new block after it. `/admin/reload-config` does
not reload it (RD5) — the docs say so.

## Data model

```sql
-- 0050_settings.up.sql
CREATE TABLE settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
-- 0050_settings.down.sql
DROP TABLE settings;
```

One key in v1: `theme.active`. Values are the four skin ids. No FK, no enum constraint in SQL —
validation is at the handler (R2) so the set can grow without a migration.

## API

| Method | Path | Gate | Body / response |
|---|---|---|---|
| `GET` | `/capabilities` | none | `…, "theme": { "active": "custom", "custom": { "name": "Rich Archive", "base": "cinematheque", "tokens": { "bg": "#0b0a0c", "ink": "#efe9e0", "accent": "#c0483f", "muted": "#9a9188", "warn": "#e2603f" } } }` — `custom` is `null` when not configured |
| `PUT` | `/admin/theme` | owner | `{ "theme": "broadcast" }` → `200 { "theme": {…} }`; `400` unknown value / `custom` without config |

No `GET /admin/theme` — `/capabilities` already carries the value for everyone.

## Config

See R9. Restart to apply. No env override for the block. Section in
`docs/reference/configuration.md` under **Appearance**, cross-referenced from **Presentation**
(`card_layout`) since both are look-and-feel keys.

## UI

Design handoff (pending) covers: the Appearance tab's card grid (three or four cards; each drawn
in its own tokens; swatch strip + display-font sample; active state), the empty/error states,
the removal of the header picker and what — if anything — takes its place in the header at narrow
widths, and the three-skin + custom QA matrix. The mockup from the brainstorm (YAML block ↔
fourth chip ↔ tinted browse grid) is the starting point but its header-chip picker is superseded
by RD2.

## Success Metrics

This is an owner-facing operability feature on a single-owner product; the metrics are
verification outcomes, not funnels.

**Leading (at merge)**
- Zero visual regression: the three-skin QA matrix plus the custom override passes; R11's
  derivation gate passes for Cinémathèque with the stated tolerance.
- A cold load of a custom-skinned instance shows no default-skin flash on the second and later
  loads of a browser (R7/R8), verified by a computed-style probe before `/capabilities` resolves.
- `rg 'localStorage' web/src/lib/theme.svelte.ts` shows only the paint cache.

**Lagging (post-release)**
- The project owner's own instance runs a custom palette from `theme.custom` without editing
  `app.css` — the feature's one known user validates the five-primary model.
- Any operator request for an in-app editor (L2) is the trigger to revisit the P2 list; none
  within a release cycle means the deferral was right.

## Open Questions

1. ~~**[design]** What occupies the header slot the picker leaves at narrow widths?~~ **Resolved
   2026-09-19 (design handoff):** nothing — the right group is *Owner* + *Owner view* for owners,
   empty for visitors; the removed control was the widest item so every breakpoint loosens.
2. **[engineering, non-blocking]** Exact derivation ratios (`color-mix` percentages) that satisfy
   R11 for Cinémathèque — tuned during S3, not decided here.
3. **[engineering, non-blocking]** Whether `accent-ink` / `warn-ink` luminance switching is
   expressible purely in CSS (`oklch(from …)` with a step function) or needs the server to emit
   the two ink tokens alongside the five primaries. If the latter, `tokens` in `/capabilities`
   grows by two derived keys; the config surface does not change.

## Timeline / routing

Three stories, one Draft PR on `HOLODEX-425-instance-skin` (ADR-069):

1. **S1 (HOLODEX-426)** — R1–R4, R7–R8. Backend + SPA plumbing; landable on its own (the three
   shipped skins, owner-set, no UI yet beyond the API).
2. **S2 (HOLODEX-427)** — R5–R6. Appearance tab, header picker removed. Needs the design handoff.
3. **S3 (HOLODEX-428)** — R9–R14. Custom palette, derivation, contrast WARN, docs.

Gates per the change-routing table: **spec** (this document) · **ADR** (settings store +
skin-as-identity, supersedes ADR-021 in part) · **design handoff** (S2) · **testing strategy**
(R1–R3 handler/repo tests, R11 computed-style gate, R12 contrast unit test, the QA matrix) ·
**security review** (new owner-gated write; YAML → CSS custom-property surface is hex-only by
construction — R9).
