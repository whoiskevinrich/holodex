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

> **Who does this:** §1 gets the environment ready and is normally done by **whoever sets up the session** (a developer or the agent) — *not* the human running §4. If you're doing the human pass, you don't need to touch `go test`, `curl`, or YAML; just make sure the app is open and someone has started the fake provider for you. **Quick "is it ready?" check:** open the app, go to **People → Hayao Miyazaki**, and confirm you see an **"Enrich from fake"** button and fields tagged **"from fake"**. If you do, skip to §4.

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
- [x] 3.12 Enrich run appears in **`/status` → Recent jobs as `kind=enrich`** with a "provider → entity (N fields)" detail (F22.6b). *(Implemented + smoke-tested `TestServiceEnrichRecordsJobRun`. To see it live: restart the backend so migration 0006 applies, then enrich a person.)*
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

> **First, get to the enrichment screen** (someone sets up §1 for you):
> 1. Open **http://localhost:5173** in a browser.
> 2. If a bar near the top asks for an **admin token**: click **Status** in the top navigation → type the token (ask whoever set this up — in this session it's `secret`) → **Unlock**. If you're never asked, you're already good.
> 3. Click **People** (top nav) → click **Hayao Miyazaki** → you're on the person page. The **"Enrich from fake"** button (top-right of the **Enrichment** box) opens the search popup ("the picker").
> 4. Change the look with the **three coloured dots** at the top-right — **Cinémathèque** (gold), **Broadcast** (cyan), **Brutalist** (lime). Several items below say "repeat in all 3 skins" — click each dot and look again.
>
> Filing a miss: note the **item number** (e.g. "4.2") and **which skin**. "Pass" = it looks right; you don't need to understand the code.

**Appearance — repeat in all 3 skins (the coloured dots)**

- [ ] 4.1 **The "from fake" tags look like info, not an error.** On the person page each field (Bio, Born, Nationality, …) has a small pill after it reading **"from fake"**. It should be a quiet **outlined** pill in the theme's main colour (gold / cyan / lime), clearly **not** red/orange (that's reserved for errors) and **not** a solid filled button. *(For reference: it uses the `--accent` colour, which must look different from the `--warn` red.)*
- [ ] 4.2 **"Strong match" is easy to read.** Click **Enrich from fake** to open the picker; the match row shows **"Strong match"** in the theme colour on the right. Confirm it's comfortably readable against the popup background.
- [ ] 4.3 **You can always see what's selected.** Click into the picker's search box, and Tab between buttons — whatever is focused gets a visible **coloured outline**. Nothing should be focused "invisibly".
- [ ] 4.4 **Nothing looks unstyled.** Glance at each situation and confirm none show plain white boxes or off-theme colours: **searching** (type in the picker), **empty** (clear the box → grey help text), **results** (a match row), **filled** (the field list on the person page).
- [ ] 4.5 **Tags don't overlap or overflow.** At normal width, then with the window narrowed to phone size (~375px wide), the "from fake" pills shouldn't bump into other badges or spill off the edge.

**Text & feel**

- [ ] 4.6 **Japanese characters show properly.** The person's **Aliases** field includes **宮崎駿**. Confirm those are real characters, **not** empty boxes/▯ ("tofu"). Re-check especially in **Broadcast** and **Brutalist** (their blocky fonts are where tofu shows up first).
- [ ] 4.7 **A provider problem shows a tidy message, not a crash.** With the picker open, if the fake provider is stopped (ask the dev, or it's already down), typing a name shows a short error line and **the rest of the page keeps working** — no blank screen, no raw error dump. *(Known nitpick: today's wording is technical — already logged. You're only checking the page survives.)*
- [ ] 4.8 **Opening the picker feels smooth.** It should pop in with a quick, subtle fade — no jarring jump or flicker. If your OS **"reduce motion"** setting is on, it should just appear instantly (no animation) — that's correct, not a bug.

> **Skipped in v1:** person **photos** aren't built yet, so there's nothing to check there.
