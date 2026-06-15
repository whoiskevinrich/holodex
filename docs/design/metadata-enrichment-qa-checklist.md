# Manual QA Checklist: Metadata Enrichment for People (F22)

**Spec**: [Metadata Source Plugins (F22)](../specs/metadata-plugins.md) · **ADR**: [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) · **Design**: [handoff](metadata-enrichment-handoff.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. It is a script, not a reference — work top to bottom, check each box, stop and file a bug on any miss.
> This consolidates and operationalizes the handoff's "Three-skin QA checklist" (§5 here) — run §5 in addition to, not instead of, the functional and a11y sections.
> **Every item is numbered (`section.item`)** — cite the number (e.g. "2.4 failed") when filing a miss so feedback is unambiguous.

**Legend** — each item is tagged by *how it's verified*, so the work can be split:
> - **[smoke]** = covered by an automated test or build gate (`go test`, `svelte-check`, the token-guard `rg`); a green build means it passes. Items already green this session are pre-checked `[x]`.
> - **[agent]** = an AI agent can verify by driving the running app programmatically (DOM / ARIA / network / computed-style inspection, API calls) — deterministic, no human judgment needed.
> - **[human]** = needs a human's eye or judgment (visual legibility, contrast, aesthetics, per-skin "look", "feels right", CJK tofu).

---

## 1. Setup / preconditions

*(Setup actions — perform these first; the tag marks who performs.)*

**Provider config file — `metadata-sources.yaml`**

- [ ] 1.1 **[agent]** **Create it from the committed template**: copy `metadata-sources.yaml.example` → `metadata-sources.yaml` (PowerShell: `Copy-Item metadata-sources.yaml.example metadata-sources.yaml`). The real file is **gitignored**, like `holodex.yaml` and `metadata-mappings.yaml` — never commit it.
- [ ] 1.2 **[agent]** **Place it where the server resolves the path** (config precedence is CLI > env > yaml > default, ADR-014):
  - **Default** — `./metadata-sources.yaml`, i.e. in the **server process's current working directory**: the **repo root** in local dev (`go run ./cmd/holodex`, or the launch.json preview), and the **container WORKDIR** in Docker.
  - **Override the location** — set `METADATA_SOURCES_PATH=/abs/path/to/metadata-sources.yaml` (env, or in a local `.env`), or `metadata_sources_path: "..."` in `holodex.yaml`. In Docker, **mount** the file at whatever path you point this to (same pattern as `metadata-mappings.yaml`).
  - A **missing** file is not an error — it just means "no providers" and the Enrich button won't appear. Confirm the path actually resolved by checking the startup log line `metadata source providers loaded ... enabled=N` (N ≥ 1).
- [ ] 1.3 **[agent]** **Enable one provider entry** with `name:` (e.g. `fake` or `tmdb`), `enabled: true`, `entity_types: [person]`, and `base_url:` pointing at a **running** provider reachable from the server that speaks the contract (`/describe` `/resolve` `/enrich` `/healthz`). Note: the in-process `enrich.Fake` is **test-only** (Go unit tests); for manual QA run a small **stub HTTP server** (or the real sidecar) at that `base_url` — e.g. `base_url: http://127.0.0.1:9100`. No network or real API keys are needed for a stub (F22.10a/b).
- [ ] 1.4 **[agent]** Core boots clean: `/status` lists the provider as **ok** with a version (F22.1a, F22.8a); no provider-protocol error in logs (F22.1e).
- [ ] 1.5 **[human]** Pick **one Person record** to test that the fake returns a confident match for (e.g. a person named to hit the fake's "Hayao Miyazaki" → `tmdb:608` fixture). Note its `/people/[id]` URL.
- [ ] 1.6 **[human]** Pick **one Person** the fake returns **no match** for (ambiguous/empty), for the no-results path (§3).
- [ ] 1.7 **[agent]** Exercise **both token states**, in separate passes:
  - [ ] 1.7.1 **[agent]** `ADMIN_TOKEN` **unset** (open-mode): owner controls available without a token; record this as the baseline-functional pass.
  - [ ] 1.7.2 **[agent]** `ADMIN_TOKEN` **set**: do a pass with **no token entered** (locked) and a pass **after unlocking** with the correct token.
- [ ] 1.8 **[human]** Browser devtools open: Network tab (to watch `/resolve`, `/enrich`, asset fetches) and Console (no errors). Have the skin picker (header) reachable.
- [ ] 1.9 **[human]** Reduced-motion test ready: a second profile/toggle with `prefers-reduced-motion: reduce`.

---

## 2. Functional — owner-gated enrich flow

**Enrich button visibility**

- [ ] 2.1 **[agent]** **Owner** (open-mode, or `ADMIN_TOKEN` set + unlocked): "Enrich from {provider}" button renders, solid accent CTA, top-right of the person panel (F22.5a).
- [ ] 2.2 **[agent]** **Non-owner** (`ADMIN_TOKEN` set, no token): button is **not rendered at all** — no disabled tease (handoff: "not owner → not rendered").
- [ ] 2.3 **[agent]** **Needs-token**: in the locked state the button is replaced by the `/status`-style **unlock form**; after entering the correct token the Enrich button appears (F22.9a).
- [ ] 2.4 **[agent]** **No provider configured** (disable the fake, restart): button absent; muted hint "No metadata source configured" — no error (handoff edge case).

**Picker + search → confirm**

- [ ] 2.5 **[agent]** Clicking Enrich opens the **modal picker**; focus moves to the search input.
- [ ] 2.6 **[agent]** Typing a name (≥2 chars) debounces (~300ms) then fires `POST /resolve` with `query` (Network tab). Under 2 chars shows help text, no call.
- [ ] 2.7 **[agent]** Candidate list renders: label (ink) · disambiguation (muted, truncated) · **confidence chip**.
- [ ] 2.8 **[agent]** **Confidence labels are humane, not raw numbers**: ≥0.85 "Strong match", 0.5–0.85 "Possible match", <0.5 "Weak match" (optional `%` in muted tabular-nums). No bare `0.98`.
- [ ] 2.9 **[agent]** Activate a row (click/Enter) → `aria-selected`, "Confirm" CTA appears (or Enter confirms directly).
- [ ] 2.10 **[agent]** Confirm → picker closes, `POST /enrich` fires, success toast "Enriched from {provider}." (~4s auto-clear).
- [ ] 2.11 **[agent]** **Fields populate**: configured Person fields (`bio`, `birthdate`, `nationality`, `website`, `aliases`, `photo`) render in the `<dl>` (F22.5c).
- [ ] 2.12 **[agent]** **Provenance badges appear**: each resolved field shows where it came from — provider fields "from {provider}", any file-sourced field "from file" (F22.7a).
- [ ] 2.13 **[agent]** Enrich run shows in **`/status` 30-day history** as `kind=enrich` with provider, entity, outcome (F22.6b).

**Re-enrich + clear**

- [ ] 2.14 **[agent]** **Re-enrich** (click Enrich again on the same person): the picker is **skipped** — goes straight to `/enrich` with the stored `external_id`; toast on completion (F22.4b, handoff re-enrich).
- [ ] 2.15 **[agent]** **Clear-provider**: the owner-only "Clear {provider} data" button (muted) prompts confirm-before-act; on confirm, the provider's `entity_enrichment` rows are removed and fields **fall back to the next source** (file or empty) (F22.7b, F22.4c).
- [x] 2.16 **[smoke]** After clear, file-sourced fields are **untouched** (non-destructive shadow layer — F22.4c/F22.3c). *(repo `TestEnrichmentShadowStore`: re-scan + clear leave file rows intact.)*

---

## 3. Matching paths

- [ ] 3.1 **[agent]** **Embedded-ID auto-match** (rare for People — simulate): give the test Person a known external ID (paste/store path, or seed `entity_enrichment.external_id`), then enrich → core resolves **deterministically, no picker** (F22.5b). Document how you simulated it (People rarely carry IDs until Series/Video generalization).
- [ ] 3.2 **[agent]** **Name-search + manual confirm** (the dominant path): no embedded ID → picker shows candidates → owner picks one → confirm. Verified in §2.
- [ ] 3.3 **[agent]** **No results**: search the no-match person → "No matches for '{query}'." (muted), input stays focused to retype (handoff edge case).
- [ ] 3.4 **[agent]** **Ambiguous**: multiple similar candidates returned → all listed with disambiguation lines; owner stays in control, nothing auto-applies (always-confirm in v1, spec OQ#5).

---

## 4. States

- [ ] 4.1 **[agent]** **Loading**: picker search shows `AsyncState` loading "Searching {provider}…" (muted, centered).
- [ ] 4.2 **[agent]** **Empty**: pre-search / cleared input shows help text; no spurious call.
- [ ] 4.3 **[agent]** **Error — provider unreachable**: stop the fake (or point `base_url` at a dead host), search → picker shows an inline **`border-warn`** message "{provider} is unavailable right now." — a single failure, not a page break (F22.2c, F22.9b, handoff edge case). The page and other providers keep working.
- [ ] 4.4 **[human]** **CJK aliases** (e.g. 宮崎駿): provider-supplied CJK aliases render in the `<dl>` and alias chips **without tofu** (boxes); verify in body `font-ui` (handoff edge case — re-check per skin in §5).
- [ ] 4.5 **[human]** **Photo asset — success** *(DEFERRED in v1 — `assets` is parsed but not fetched; skip until the person-photo slice ships)*: `assets.photo` downloads **core-side** (never via a provider redirect) and is stored at **`${DATABASE_PATH}/images/people/<person_id>.{jpg,png}`** (Phase 3 F14.3; `DATABASE_PATH` defaults to `${DATA_PATH}/holodex.db`, so the images dir sits beside the DB under `DATA_PATH`). Shown on the person card/page; `thumb-shimmer` hook shows while downloading (F22.5e).
- [ ] 4.6 **[human]** **Photo asset — fallback** *(DEFERRED in v1)*: with a broken/absent photo URL, the UI falls back to the existing no-photo treatment and **does not block** field display (handoff edge case).
- [ ] 4.7 **[agent]** **Slow connection**: throttle to Slow 3G — all provider calls are explicit and show their loading state; nothing auto-polls (unlike activity) (handoff edge case).

---

## 5. Three-skin visual QA

Render `/people/[id]` and exercise the picker in **each** skin via the header picker. Repeat the sub-checklist three times — regressions routinely appear in only one skin.

Reference tokens (from `app.css`): radius `--radius` = **2px / 0 / 0**; accent = Cinémathèque gold `#e8a33d` / Broadcast cyan `#36e0d0` / Brutalist lime `#d6ff3f`; warn = `#e2603f` / `#ff6f61` / `#ff5e3a`.

### 5a. Cinémathèque (`data-theme='cinematheque'`)

- [ ] 5a.1 **[human]** **Provenance chips**: file = muted pill on `bg-surface-2`; provider = **outlined-accent** pill (`border-accent`/`text-accent`) — gold outline reads on `bg-surface`, **does NOT use `--warn`** (`#e2603f`) and is distinguishable from any solid-accent CTA.
- [ ] 5a.2 **[agent]** **Picker panel + backdrop**: computed `--radius` is **2px**; backdrop dims (`bg-bg/70`); no error-colored borders on normal states.
- [ ] 5a.3 **[human]** **Confidence chip**: "Strong match" accent text legible on gold.
- [ ] 5a.4 **[agent]** **`.skin-title`**: serif display face (Fraunces), normal casing, no caret.
- [ ] 5a.5 **[human]** **Focus ring** (`focus:border-accent`) visible on input + buttons.
- [ ] 5a.6 **[agent]** **Reduced-motion**: picker open is instant (computed `animation: none`) when `prefers-reduced-motion: reduce`.

### 5b. Broadcast (`data-theme='broadcast'`)

- [ ] 5b.1 **[human]** **Provenance chips**: file = muted pill; provider = outlined-cyan pill — legible on `bg-surface`, **NOT** the coral `--warn` (`#ff6f61`); distinct from solid-accent CTA.
- [ ] 5b.2 **[agent]** **Picker panel + backdrop**: computed `--radius` is **0** — **no stray rounded corners** anywhere on the picker.
- [ ] 5b.3 **[human]** **Confidence chip**: "Strong match" cyan text legible (bright accent — eyeball contrast).
- [ ] 5b.4 **[agent]** **`.skin-title`**: VT323 mono display, **UPPERCASE**, **caret `▮`** flourish after the heading (`::after` content).
- [ ] 5b.5 **[human]** **Focus ring** visible on input + buttons.
- [ ] 5b.6 **[human]** **CJK aliases** (宮崎駿) render without tofu in the mono UI face (Share Tech Mono falls back for CJK — confirm).
- [ ] 5b.7 **[agent]** **Reduced-motion**: picker open instant.

### 5c. Brutalist (`data-theme='brutalist'`)

- [ ] 5c.1 **[human]** **Provenance chips**: file = muted pill; provider = outlined-lime pill — legible on `bg-surface`, **NOT** the red-orange `--warn` (`#ff5e3a`); distinct from solid-accent CTA.
- [ ] 5c.2 **[agent]** **Picker panel + backdrop**: computed `--radius` is **0** — no stray rounded corners.
- [ ] 5c.3 **[human]** **Confidence chip**: "Strong match" lime text legible (very bright accent — eyeball contrast).
- [ ] 5c.4 **[agent]** **`.skin-title`**: Spline Sans Mono, **UPPERCASE**, no caret (Broadcast-only).
- [ ] 5c.5 **[human]** **Focus ring** visible on input + buttons.
- [ ] 5c.6 **[human]** **CJK aliases** render without tofu in the mono face.
- [ ] 5c.7 **[agent]** **Reduced-motion**: picker open instant.

### 5d. All-skin sweep (each skin)

- [ ] 5d.1 **[human]** **Loading / empty / error / populated** states all themed — no raw white/black, no hardcoded color leaking through.
- [ ] 5d.2 **[human]** Provenance chip does not **collide** with the resolution/quality badge or the active-accent state at any viewport (desktop / mobile <640).

---

## 6. Accessibility

- [ ] 6.1 **[agent]** **Dialog**: picker has `role="dialog" aria-modal="true"`, labelled by its heading (`aria-labelledby`). Focus is **trapped** inside.
- [ ] 6.2 **[agent]** **Esc** closes the picker; **focus returns** to the Enrich button on close (also on backdrop click and close button).
- [ ] 6.3 **[agent]** **Combobox/listbox**: input has `role="combobox" aria-expanded aria-controls aria-activedescendant`; list `role="listbox"`; rows `role="option" aria-selected`.
- [ ] 6.4 **[agent]** **Keyboard nav**: ↑/↓ move the active option (wrap/clamp matching search-history), **Enter** confirms active, **Esc** closes, **Tab** reaches Confirm/Cancel. Whole flow operable **mouse-free**.
- [ ] 6.5 **[agent]** **`aria-activedescendant`** tracks the active row as you arrow through (inspect in devtools).
- [ ] 6.6 **[agent]** **`aria-live="polite"`** results region announces the count ("3 matches") after a search.
- [ ] 6.7 **[agent]** **Confidence color is not the sole signal**: the word ("Strong match") is present, not color alone (color-blind safe).
- [ ] 6.8 **[agent]** **Provenance badges** carry `aria-label="source: from {label}"` (full phrase, not just the bare provider name).
- [ ] 6.9 **[agent]** **Owner controls when not owner are ABSENT from the DOM** — not visually hidden — so nothing misleading appears in the a11y tree (inspect the accessibility tree with no token).

---

## 7. Security-adjacent manual checks

- [x] 7.1 **[smoke]** With `ADMIN_TOKEN` **set** and **no token**, the enrichment endpoints (`/resolve`, `/enrich`, clear) return **401** (F22.9a). *(api `TestEnrichGated`.)* The SPA also hides the controls — confirm the UI side **[agent]**.
- [ ] 7.2 **[agent]** **No upstream API keys** appear in: the read-model / `/api` responses, the `/status` page, or the logs during an enrich (F22.9d, F22.8a). The fake/provider owns its own key in its container env only.
- [x] 7.3 **[smoke]** **Untrusted-response handling**: overlong / garbage field values are **length-capped and sanitized**, and a malformed response **fails the single fetch, not the server** (F22.9b). *(enrich `TestSanitizeValue` / `TestSanitizeFieldsCaps` + client decode tests.)* Confirm the **UI** shows no layout break / no raw HTML **[agent]**.
- [x] 7.4 **[smoke]** **SSRF posture**: core only ever calls the allowlisted `base_url` and **does not follow a provider redirect to another host** (F22.2b, F22.9c). *(enrich `TestHTTPClientNoCrossHostRedirect` + `TestRegistryLoadAndAllowlist`.)*

> Reminder (CLAUDE.md routing): this feature touches access + outbound network → a **`/security-review` sign-off is required before merge** in addition to these checks (done 2026-06-15, clean — recorded in ADR-033 status).

---

## 8. Token-discipline gate

- [x] 8.1 **[smoke]** The guard from CLAUDE.md returns **empty** against the new components:

```powershell
rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'
```

- [x] 8.2 **[smoke]** No literal palette / hex / named-font / fixed-radius in the new markup (`EnrichPicker.svelte`, `ProvenanceBadge.svelte`, the person `<dl>`) — covered by 8.1. Any skin-specific flourish lives in `app.css` gated by `[data-theme]` on a shared hook class, not per-component markup.
- [x] 8.3 **[smoke]** `rounded-full` on provenance/alias chips is the **only** intentional fixed radius (pill shape — allowed by the guard); everything else uses `rounded-theme`.
