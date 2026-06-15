# Manual QA Checklist: Metadata Enrichment for People (F22)

**Spec**: [Metadata Source Plugins (F22)](../specs/metadata-plugins.md) · **ADR**: [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) · **Design**: [handoff](metadata-enrichment-handoff.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped into three sections **by verifier**, so each actor runs only their own:
> - **§2 Smoke** — covered by an automated test or build gate (`go test`, `svelte-check`, the token-guard `rg`). Green build = pass; pre-checked `[x]` with the test named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/network/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (legibility, contrast, aesthetics, per-skin "look", "feels right", CJK tofu).
>
> §1 is one-time **setup** that both §3 and §4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.
>
> **Run log:** §2 is green in CI. §3 items marked `[x]` were **driven live 2026-06-15 against a local stub provider**; **⚠** = gap found (tracked in the main `TASKS.md`); **~** = not exercised this run. §4 is open for the human pass.

---

## 1. Setup / preconditions

- [ ] 1.1 **Create the provider config**: copy `metadata-sources.yaml.example` → `metadata-sources.yaml` (gitignored). Default location is `./metadata-sources.yaml` in the server's CWD (repo root in dev; container WORKDIR in Docker); override via `METADATA_SOURCES_PATH` / `metadata_sources_path` / Docker mount.
- [ ] 1.2 **Enable one provider** (`name`, `enabled: true`, `entity_types: [person]`, `base_url`) pointing at a **running** provider speaking the contract (`/describe` `/resolve` `/enrich` `/healthz`). For manual QA, start the bundled fake provider with **`preview_start enrich-stub`** (or `node testdata/enrich-stub/stub.js`) on `http://127.0.0.1:9100` and uncomment the `fake` block in `metadata-sources.yaml` — no network/keys needed (see [`testdata/enrich-stub/README.md`](../../testdata/enrich-stub/README.md), F22.10). *(The in-process `enrich.Fake` is `go test`-only — not runnable as a server.)*
- [ ] 1.3 **Core boots clean**: `POST /admin/reload-config` (or restart) loads it; `/status` lists the provider **ok** with a version; no protocol error in logs (F22.1, F22.8a).
- [ ] 1.4 Pick a **Person the provider matches** (note its `/people/[id]` URL) and a **Person it does not** (for the no-results path).
- [ ] 1.5 Exercise **both token states**: `ADMIN_TOKEN` unset (open) vs set (locked, then unlocked via `/status`).
- [ ] 1.6 Devtools open (Network + Console); skin picker reachable; a `prefers-reduced-motion: reduce` profile ready.

---

## 2. Smoke — automated (green in CI)

- [x] 2.1 **Shadow store is non-destructive**: a re-scan + a clear leave file-sourced rows intact (F22.4/F22.3c). *(repo `TestEnrichmentShadowStore`.)*
- [x] 2.2 **Enrich endpoints 401 without token** when `ADMIN_TOKEN` is set (F22.9a). *(api `TestEnrichGated`.)*
- [x] 2.3 **Untrusted-response bounding**: overlong/garbage values length-capped + sanitized; malformed body fails the single fetch, not the server (F22.9b). *(enrich `TestSanitizeValue` / `TestSanitizeFieldsCaps` + client decode tests.)*
- [x] 2.4 **SSRF posture**: only the allowlisted `base_url` is dialed; a cross-host redirect is not followed (F22.2b/F22.9c). *(enrich `TestHTTPClientNoCrossHostRedirect` + `TestRegistryLoadAndAllowlist`.)*
- [x] 2.5 **Token-discipline guard** is empty against the new components: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`.
- [x] 2.6 **No literal palette / hex / named-font / fixed-radius** in `EnrichPicker.svelte` / `ProvenanceBadge.svelte` / the person `<dl>` (covered by 2.5).
- [x] 2.7 **`rounded-full`** on provenance/alias chips is the only intentional fixed radius (pill shape — allowed by the guard); everything else uses `rounded-theme`.

> `/security-review` sign-off (2026-06-15, clean) is recorded in ADR-033 status — required before merge in addition to these.

---

## 3. Agent — drive the running app

**Owner-gating & button visibility**

- [x] 3.1 **Owner** (open, or token + unlocked): "Enrich from {provider}" button renders (solid accent CTA, top-right of the person panel) (F22.5a).
- [x] 3.2 **Non-owner** (token set, none entered): button **absent from the DOM** — not hidden (F22.9a; a11y tree clean).
- [x] 3.3 **Needs-token**: locked state shows the `/status`-style unlock form; after the correct token the button appears.
- [x] 3.4 ⚠ **No provider configured**: button is correctly **absent**, but the handoff's muted "No metadata source configured" hint **isn't rendered**. *(gap — TASKS.)*

**Picker flow**

- [x] 3.5 Click Enrich → **modal opens, focus moves to the search input** (F22.5b).
- [x] 3.6 Typing ≥2 chars **debounces (~300 ms) then fires `POST /resolve`**; under 2 chars shows help text, no call.
- [x] 3.7 Candidate renders: label (ink) · disambiguation (muted, truncated) · confidence chip.
- [x] 3.8 **Confidence label is humane** — "Strong match" (≥0.85) etc., **not the raw `0.9`**.
- [x] 3.9 Activate a row (Enter/click) → `aria-selected`; Enter confirms.
- [x] 3.10 Confirm → `POST /enrich`, **dialog closes, fields populate** in the `<dl>` (F22.5c). ⚠ the handoff's success **toast** isn't shown (fields-populate is the only feedback). *(gap — TASKS.)*
- [x] 3.11 **Provenance badges** "from {provider}" on each resolved field (F22.7).
- [ ] 3.12 ⚠ Enrich run should appear in **`/status` history as `kind=enrich`** (F22.6b) — **not recorded** (`Service.Enrich` never writes a `JobRun`). *(functional gap — TASKS.)*
- [ ] 3.13 ~ **Re-enrich skips the picker** (F22.4b) — the re-enrich-without-picker shortcut UI **isn't implemented** (deferred); Enrich always opens the picker.
- [x] 3.14 **Clear-provider** → the provider's rows are removed and fields fall back to the next source (F22.7b).

**Matching**

- [ ] 3.15 ~ **Embedded-ID auto-match** (F22.5b) — not exercised this run (People rarely carry IDs; needs a seeded `external_id`).
- [x] 3.16 **Name-search + manual confirm** (the dominant path) — verified in the picker flow.
- [x] 3.17 **No results**: a non-matching query → "No matches…" / empty list, input stays focused.

**States**

- [ ] 3.18 ~ **Loading**: "Searching {provider}…" (transient — not captured this run).
- [x] 3.19 **Empty / pre-search**: "Type at least two characters to search." help text; no spurious call.
- [x] 3.20 **Error — provider unreachable**: page survives, picker stays open, error in the **`text-warn`** `aria-live` region. ⚠ message is the raw `"API …/resolve failed: 502"` rather than the friendly "{provider} is unavailable right now." *(gap — TASKS.)*
- [ ] 3.21 ~ **Slow connection** (Slow-3G throttle): calls explicit, loading shown, nothing auto-polls — not run.

**Accessibility**

- [x] 3.22 Dialog: `role="dialog" aria-modal="true"`, `aria-labelledby` → the heading.
- [x] 3.23 **Esc** (and backdrop / close button) closes the picker. ⚠ focus does **not** return to the Enrich button (goes to `<body>`) — the focus-trap follow-up. *(gap — TASKS.)*
- [x] 3.24 Input `role="combobox"` + `aria-controls="enrich-candidates"` + `aria-expanded`; list `role="listbox"`; rows `role="option" aria-selected`.
- [x] 3.25 `aria-activedescendant` tracks the active row.
- [x] 3.26 `aria-live="polite"` region announces the count ("1 match").
- [x] 3.27 Confidence is conveyed by the **word** ("Strong match"), not color alone (color-blind safe).
- [x] 3.28 Provenance badges carry `aria-label="source: from {provider}"` (full phrase).
- [x] 3.29 Owner controls are **absent from the DOM** for a non-owner (verified in 3.2).

**Per-skin computed styles (repeat for Cinémathèque / Broadcast / Brutalist)**

- [x] 3.30 Picker panel computed `border-radius` matches `--radius` (**2px / 0 / 0**).
- [x] 3.31 `.skin-title` font + casing + caret: Fraunces/normal/none · VT323/UPPER/`▮` · Spline-Mono/UPPER/none.
- [x] 3.32 **Provider chip color = `--accent` in every skin** (`#e8a33d` / `#36e0d0` / `#d6ff3f`), **never `--warn`**.
- [ ] 3.33 ~ Reduced-motion: picker open animation is gated in `@media (prefers-reduced-motion: no-preference)` (code-confirmed; live emulation not available via preview).

**Security (agent)**

- [x] 3.34 **No upstream key or `base_url`** appears in `/api` responses or `/status` (F22.9d) — `/enrich/sources` returns only `name` + `entity_types`.

---

## 4. Human — needs a human's eye

**Per-skin legibility (repeat for each skin)**

- [ ] 4.1 **Provenance chips read correctly**: file = muted pill on `bg-surface-2`; provider = outlined-accent pill that's legible on `bg-surface` and clearly **not** an error color (compare against `--warn` `#e2603f` / `#ff6f61` / `#ff5e3a`) and distinct from a solid-accent CTA.
- [ ] 4.2 **Confidence chip** ("Strong match") accent text is legible on each accent (gold / bright cyan / bright lime).
- [ ] 4.3 **Focus ring** (`focus:border-accent`) is visible on the input + buttons.
- [ ] 4.4 **Loading / empty / error / populated** states are all themed — no raw white/black, no hardcoded color leaking through.
- [ ] 4.5 Provenance chip does **not collide** with the resolution/quality badge or the active-accent state at any viewport (desktop / mobile <640).

**Content / rendering**

- [ ] 4.6 **CJK aliases** (e.g. 宮崎駿) render **without tofu** (boxes) in the `<dl>` and alias chips — re-check in each mono-faced skin (Broadcast / Brutalist).
- [ ] 4.7 **Photo asset — success** *(DEFERRED v1 — `assets` parsed but not fetched; skip until the person-photo slice)*: stored at `${DATABASE_PATH}/images/people/<id>.{jpg,png}` (F14.3), `thumb-shimmer` while downloading.
- [ ] 4.8 **Photo asset — fallback** *(DEFERRED v1)*: broken/absent photo → existing no-photo treatment, field display not blocked.

**Feel**

- [ ] 4.9 The picker open **feels instant**; with `prefers-reduced-motion: reduce` there's no fade/scale (eyeball — pairs with the computed check 3.33).
