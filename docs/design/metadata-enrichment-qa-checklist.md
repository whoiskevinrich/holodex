# Manual QA Checklist: Metadata Enrichment for People (F22)

**Spec**: [Metadata Source Plugins (F22)](../specs/metadata-plugins.md) · **ADR**: [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) · **Design**: [handoff](metadata-enrichment-handoff.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. It is a script, not a reference — work top to bottom, check each box, stop and file a bug on any miss.
> This consolidates and operationalizes the handoff's "Three-skin QA checklist" (§5 here) — run §5 in addition to, not instead of, the functional and a11y sections.
> **Every item is numbered (`section.item`)** — cite the number (e.g. "2.4 failed") when filing a miss so feedback is unambiguous.

---

## 1. Setup / preconditions

**Provider config file — `metadata-sources.yaml`**

- [ ] 1.1 **Create it from the committed template**: copy `metadata-sources.yaml.example` → `metadata-sources.yaml` (PowerShell: `Copy-Item metadata-sources.yaml.example metadata-sources.yaml`). The real file is **gitignored**, like `holodex.yaml` and `metadata-mappings.yaml` — never commit it.
- [ ] 1.2 **Place it where the server resolves the path** (config precedence is CLI > env > yaml > default, ADR-014):
  - **Default** — `./metadata-sources.yaml`, i.e. in the **server process's current working directory**: the **repo root** in local dev (`go run ./cmd/holodex`, or the launch.json preview), and the **container WORKDIR** in Docker.
  - **Override the location** — set `METADATA_SOURCES_PATH=/abs/path/to/metadata-sources.yaml` (env, or in a local `.env`), or `metadata_sources_path: "..."` in `holodex.yaml`. In Docker, **mount** the file at whatever path you point this to (same pattern as `metadata-mappings.yaml`).
  - A **missing** file is not an error — it just means "no providers" and the Enrich button won't appear. Confirm the path actually resolved by checking the startup log line `metadata source providers loaded ... enabled=N` (N ≥ 1).
- [ ] 1.3 **Enable one provider entry** with `name:` (e.g. `fake` or `tmdb`), `enabled: true`, `entity_types: [person]`, and `base_url:` pointing at a **running** provider reachable from the server that speaks the contract (`/describe` `/resolve` `/enrich` `/healthz`). Note: the in-process `enrich.Fake` is **test-only** (Go unit tests); for manual QA run a small **stub HTTP server** (or the real sidecar) at that `base_url` — e.g. `base_url: http://127.0.0.1:9100`. No network or real API keys are needed for a stub (F22.10a/b).
- [ ] 1.4 Core boots clean: `/status` lists the provider as **ok** with a version (F22.1a, F22.8a); no provider-protocol error in logs (F22.1e).
- [ ] 1.5 Pick **one Person record** to test that the fake returns a confident match for (e.g. a person named to hit the fake's "Hayao Miyazaki" → `tmdb:608` fixture). Note its `/people/[id]` URL.
- [ ] 1.6 Pick **one Person** the fake returns **no match** for (ambiguous/empty), for the no-results path (§3).
- [ ] 1.7 Exercise **both token states**, in separate passes:
  - [ ] 1.7.1 `ADMIN_TOKEN` **unset** (open-mode): owner controls available without a token; record this as the baseline-functional pass.
  - [ ] 1.7.2 `ADMIN_TOKEN` **set**: do a pass with **no token entered** (locked) and a pass **after unlocking** with the correct token.
- [ ] 1.8 Browser devtools open: Network tab (to watch `/resolve`, `/enrich`, asset fetches) and Console (no errors). Have the skin picker (header) reachable.
- [ ] 1.9 Reduced-motion test ready: a second profile/toggle with `prefers-reduced-motion: reduce`.

---

## 2. Functional — owner-gated enrich flow

**Enrich button visibility**

- [ ] 2.1 **Owner** (open-mode, or `ADMIN_TOKEN` set + unlocked): "Enrich from {provider}" button renders, solid accent CTA, top-right of the person panel (F22.5a).
- [ ] 2.2 **Non-owner** (`ADMIN_TOKEN` set, no token): button is **not rendered at all** — no disabled tease (handoff: "not owner → not rendered").
- [ ] 2.3 **Needs-token**: in the locked state the button is replaced by the `/status`-style **unlock form**; after entering the correct token the Enrich button appears (F22.9a).
- [ ] 2.4 **No provider configured** (disable the fake, restart): button absent; muted hint "No metadata source configured" — no error (handoff edge case).

**Picker + search → confirm**

- [ ] 2.5 Clicking Enrich opens the **modal picker**; focus moves to the search input.
- [ ] 2.6 Typing a name (≥2 chars) debounces (~300ms) then fires `POST /resolve` with `query` (Network tab). Under 2 chars shows help text, no call.
- [ ] 2.7 Candidate list renders: label (ink) · disambiguation (muted, truncated) · **confidence chip**.
- [ ] 2.8 **Confidence labels are humane, not raw numbers**: ≥0.85 "Strong match", 0.5–0.85 "Possible match", <0.5 "Weak match" (optional `%` in muted tabular-nums). No bare `0.98`.
- [ ] 2.9 Activate a row (click/Enter) → `aria-selected`, "Confirm" CTA appears (or Enter confirms directly).
- [ ] 2.10 Confirm → picker closes, `POST /enrich` fires, success toast "Enriched from {provider}." (~4s auto-clear).
- [ ] 2.11 **Fields populate**: configured Person fields (`bio`, `birthdate`, `nationality`, `website`, `aliases`, `photo`) render in the `<dl>` (F22.5c).
- [ ] 2.12 **Provenance badges appear**: each resolved field shows where it came from — provider fields "from {provider}", any file-sourced field "from file" (F22.7a).
- [ ] 2.13 Enrich run shows in **`/status` 30-day history** as `kind=enrich` with provider, entity, outcome (F22.6b).

**Re-enrich + clear**

- [ ] 2.14 **Re-enrich** (click Enrich again on the same person): the picker is **skipped** — goes straight to `/enrich` with the stored `external_id`; toast on completion (F22.4b, handoff re-enrich).
- [ ] 2.15 **Clear-provider**: the owner-only "Clear {provider} data" button (muted) prompts confirm-before-act; on confirm, the provider's `entity_enrichment` rows are removed and fields **fall back to the next source** (file or empty) (F22.7b, F22.4c).
- [ ] 2.16 After clear, file-sourced fields are **untouched** (non-destructive shadow layer — F22.4c/F22.3c).

---

## 3. Matching paths

- [ ] 3.1 **Embedded-ID auto-match** (rare for People — simulate): give the test Person a known external ID (paste/store path, or seed `entity_enrichment.external_id`), then enrich → core resolves **deterministically, no picker** (F22.5b). Document how you simulated it (People rarely carry IDs until Series/Video generalization).
- [ ] 3.2 **Name-search + manual confirm** (the dominant path): no embedded ID → picker shows candidates → owner picks one → confirm. Verified in §2.
- [ ] 3.3 **No results**: search the no-match person → "No matches for '{query}'." (muted), input stays focused to retype (handoff edge case).
- [ ] 3.4 **Ambiguous**: multiple similar candidates returned → all listed with disambiguation lines; owner stays in control, nothing auto-applies (always-confirm in v1, spec OQ#5).

---

## 4. States

- [ ] 4.1 **Loading**: picker search shows `AsyncState` loading "Searching {provider}…" (muted, centered).
- [ ] 4.2 **Empty**: pre-search / cleared input shows help text; no spurious call.
- [ ] 4.3 **Error — provider unreachable**: stop the fake (or point `base_url` at a dead host), search → picker shows an inline **`border-warn`** message "{provider} is unavailable right now." — a single failure, not a page break (F22.2c, F22.9b, handoff edge case). The page and other providers keep working.
- [ ] 4.4 **CJK aliases** (e.g. 宮崎駿): provider-supplied CJK aliases render in the `<dl>` and alias chips **without tofu** (boxes); verify in body `font-ui` (handoff edge case — re-check per skin in §5).
- [ ] 4.5 **Photo asset — success** *(DEFERRED in v1 — `assets` is parsed but not fetched; skip until the person-photo slice ships)*: `assets.photo` downloads **core-side** (never via a provider redirect) and is stored at **`${DATABASE_PATH}/images/people/<person_id>.{jpg,png}`** (Phase 3 F14.3; `DATABASE_PATH` defaults to `${DATA_PATH}/holodex.db`, so the images dir sits beside the DB under `DATA_PATH`). Shown on the person card/page; `thumb-shimmer` hook shows while downloading (F22.5e).
- [ ] 4.6 **Photo asset — fallback** *(DEFERRED in v1)*: with a broken/absent photo URL, the UI falls back to the existing no-photo treatment and **does not block** field display (handoff edge case).
- [ ] 4.7 **Slow connection**: throttle to Slow 3G — all provider calls are explicit and show their loading state; nothing auto-polls (unlike activity) (handoff edge case).

---

## 5. Three-skin visual QA

Render `/people/[id]` and exercise the picker in **each** skin via the header picker. Repeat the sub-checklist three times — regressions routinely appear in only one skin.

Reference tokens (from `app.css`): radius `--radius` = **2px / 0 / 0**; accent = Cinémathèque gold `#e8a33d` / Broadcast cyan `#36e0d0` / Brutalist lime `#d6ff3f`; warn = `#e2603f` / `#ff6f61` / `#ff5e3a`.

### 5a. Cinémathèque (`data-theme='cinematheque'`)

- [ ] 5a.1 **Provenance chips**: file = muted pill on `bg-surface-2`; provider = **outlined-accent** pill (`border-accent`/`text-accent`) — gold outline reads on `bg-surface`, **does NOT use `--warn`** (`#e2603f`) and is distinguishable from any solid-accent CTA.
- [ ] 5a.2 **Picker panel + backdrop**: radius is **2px**; backdrop dims (`bg-bg/70`); no error-colored borders on normal states.
- [ ] 5a.3 **Confidence chip**: "Strong match" accent text legible on gold.
- [ ] 5a.4 **`.skin-title`**: serif display face (Fraunces), normal casing, no caret.
- [ ] 5a.5 **Focus ring** (`focus:border-accent`) visible on input + buttons.
- [ ] 5a.6 **Reduced-motion**: picker open is instant (no fade/scale) when `prefers-reduced-motion: reduce`.

### 5b. Broadcast (`data-theme='broadcast'`)

- [ ] 5b.1 **Provenance chips**: file = muted pill; provider = outlined-cyan pill — legible on `bg-surface`, **NOT** the coral `--warn` (`#ff6f61`); distinct from solid-accent CTA.
- [ ] 5b.2 **Picker panel + backdrop**: radius is **0** — **no stray rounded corners** anywhere on the picker.
- [ ] 5b.3 **Confidence chip**: "Strong match" cyan text legible (bright accent — eyeball contrast).
- [ ] 5b.4 **`.skin-title`**: VT323 mono display, **UPPERCASE**, **caret `▮`** flourish after the heading.
- [ ] 5b.5 **Focus ring** visible on input + buttons.
- [ ] 5b.6 **CJK aliases** (宮崎駿) render without tofu in the mono UI face (Share Tech Mono falls back for CJK — confirm).
- [ ] 5b.7 **Reduced-motion**: picker open instant.

### 5c. Brutalist (`data-theme='brutalist'`)

- [ ] 5c.1 **Provenance chips**: file = muted pill; provider = outlined-lime pill — legible on `bg-surface`, **NOT** the red-orange `--warn` (`#ff5e3a`); distinct from solid-accent CTA.
- [ ] 5c.2 **Picker panel + backdrop**: radius is **0** — no stray rounded corners.
- [ ] 5c.3 **Confidence chip**: "Strong match" lime text legible (very bright accent — eyeball contrast).
- [ ] 5c.4 **`.skin-title`**: Spline Sans Mono, **UPPERCASE**, no caret (Broadcast-only).
- [ ] 5c.5 **Focus ring** visible on input + buttons.
- [ ] 5c.6 **CJK aliases** render without tofu in the mono face.
- [ ] 5c.7 **Reduced-motion**: picker open instant.

### 5d. All-skin sweep (each skin)

- [ ] 5d.1 **Loading / empty / error / populated** states all themed — no raw white/black, no hardcoded color leaking through.
- [ ] 5d.2 Provenance chip does not **collide** with the resolution/quality badge or the active-accent state at any viewport (desktop / mobile <640).

---

## 6. Accessibility

- [ ] 6.1 **Dialog**: picker has `role="dialog" aria-modal="true"`, labelled by its heading (`aria-labelledby`). Focus is **trapped** inside.
- [ ] 6.2 **Esc** closes the picker; **focus returns** to the Enrich button on close (also on backdrop click and close button).
- [ ] 6.3 **Combobox/listbox**: input has `role="combobox" aria-expanded aria-controls aria-activedescendant`; list `role="listbox"`; rows `role="option" aria-selected`.
- [ ] 6.4 **Keyboard nav**: ↑/↓ move the active option (wrap/clamp matching search-history), **Enter** confirms active, **Esc** closes, **Tab** reaches Confirm/Cancel. Whole flow operable **mouse-free**.
- [ ] 6.5 **`aria-activedescendant`** tracks the active row as you arrow through (inspect in devtools).
- [ ] 6.6 **`aria-live="polite"`** results region announces the count ("3 matches") after a search.
- [ ] 6.7 **Confidence color is not the sole signal**: the word ("Strong match") is present, not color alone (color-blind safe).
- [ ] 6.8 **Provenance badges** carry `aria-label="source: from {label}"` (full phrase, not just the bare provider name).
- [ ] 6.9 **Owner controls when not owner are ABSENT from the DOM** — not visually hidden — so nothing misleading appears in the a11y tree (inspect the accessibility tree with no token).

---

## 7. Security-adjacent manual checks

- [ ] 7.1 With `ADMIN_TOKEN` **set** and **no token**, hit the enrichment endpoints directly (`/resolve`, `/enrich`, clear) → all return **401** (F22.9a). The SPA also hides the controls.
- [ ] 7.2 **No upstream API keys** appear in: the read-model / `/api` responses, the `/status` page, or the logs during an enrich (F22.9d, F22.8a). The fake/provider owns its own key in its container env only.
- [ ] 7.3 **Untrusted-response handling**: point at a fake variant returning **overlong / garbage** field values and a malformed asset → values are **length-capped and sanitized** in the UI (no layout break, no raw HTML injection), asset download is size- and content-type-limited, and a malformed response **fails the single fetch, not the server** (F22.9b).
- [ ] 7.4 **SSRF posture**: a `/resolve` or `/enrich` response that names an **unconfigured host** (or a redirect to one) does **not** cause core to call it — core only ever calls the allowlisted `base_url` and does not follow provider redirects (F22.2b, F22.9c). Confirm via outbound-call logging.

> Reminder (CLAUDE.md routing): this feature touches access + outbound network → a **`/security-review` sign-off is required before merge** in addition to these manual checks.

---

## 8. Token-discipline gate

- [ ] 8.1 Run the exact guard from CLAUDE.md against the new components — it must return **empty**:

```powershell
rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'
```

- [ ] 8.2 No literal palette / hex / named-font / fixed-radius in the new markup (`EnrichButton`, `EnrichPicker.svelte`, `CandidateRow`, `ProvenanceBadge`, the person `<dl>`). Any skin-specific flourish lives in `app.css` gated by `[data-theme]` on a shared hook class, not per-component markup.
- [ ] 8.3 `rounded-full` on provenance/alias chips is the **only** intentional fixed radius (pill shape — allowed); everything else uses `rounded-theme`.
