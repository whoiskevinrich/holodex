# Manual QA Checklist: Metadata Enrichment for People (F22)

**Spec**: [Metadata Source Plugins (F22)](../specs/metadata-plugins.md) · **ADR**: [ADR-033](../architecture/ADR-033-metadata-source-plugins.md) · **Design**: [handoff](metadata-enrichment-handoff.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. It is a script, not a reference — work top to bottom, check each box, stop and file a bug on any miss.
> This consolidates and operationalizes the handoff's "Three-skin QA checklist" (§5 here) — run §5 in addition to, not instead of, the functional and a11y sections.

---

## 1. Setup / preconditions

- [ ] Provider config: a `metadata-sources.yaml` exists with the **in-process fake provider** enabled (`name: fake` or `name: tmdb`, `enabled: true`, `entity_types: [person]`), pointed at the httptest/stub `base_url` (F22.10a/b). No network or real API keys are required.
- [ ] Core boots clean: `/status` lists the fake provider as **ok** with a version (F22.1a, F22.8a); no provider-protocol error in logs (F22.1e).
- [ ] Pick **one Person record** to test that the fake returns a confident match for (e.g. a person named to hit the fake's "Hayao Miyazaki" → `tmdb:608` fixture). Note its `/people/[id]` URL.
- [ ] Pick **one Person** the fake returns **no match** for (ambiguous/empty), for the no-results path (§3).
- [ ] Exercise **both token states**, in separate passes:
  - [ ] `ADMIN_TOKEN` **unset** (open-mode): owner controls available without a token; record this as the baseline-functional pass.
  - [ ] `ADMIN_TOKEN` **set**: do a pass with **no token entered** (locked) and a pass **after unlocking** with the correct token.
- [ ] Browser devtools open: Network tab (to watch `/resolve`, `/enrich`, asset fetches) and Console (no errors). Have the skin picker (header) reachable.
- [ ] Reduced-motion test ready: a second profile/toggle with `prefers-reduced-motion: reduce`.

---

## 2. Functional — owner-gated enrich flow

**Enrich button visibility**

- [ ] **Owner** (open-mode, or `ADMIN_TOKEN` set + unlocked): "Enrich from {provider}" button renders, solid accent CTA, top-right of the person panel (F22.5a).
- [ ] **Non-owner** (`ADMIN_TOKEN` set, no token): button is **not rendered at all** — no disabled tease (handoff: "not owner → not rendered").
- [ ] **Needs-token**: in the locked state the button is replaced by the `/status`-style **unlock form**; after entering the correct token the Enrich button appears (F22.9a).
- [ ] **No provider configured** (disable the fake, restart): button absent; muted hint "No metadata source configured" — no error (handoff edge case).

**Picker + search → confirm**

- [ ] Clicking Enrich opens the **modal picker**; focus moves to the search input.
- [ ] Typing a name (≥2 chars) debounces (~300ms) then fires `POST /resolve` with `query` (Network tab). Under 2 chars shows help text, no call.
- [ ] Candidate list renders: label (ink) · disambiguation (muted, truncated) · **confidence chip**.
- [ ] **Confidence labels are humane, not raw numbers**: ≥0.85 "Strong match", 0.5–0.85 "Possible match", <0.5 "Weak match" (optional `%` in muted tabular-nums). No bare `0.98`.
- [ ] Activate a row (click/Enter) → `aria-selected`, "Confirm" CTA appears (or Enter confirms directly).
- [ ] Confirm → picker closes, `POST /enrich` fires, success toast "Enriched from {provider}." (~4s auto-clear).
- [ ] **Fields populate**: configured Person fields (`bio`, `birthdate`, `nationality`, `website`, `aliases`, `photo`) render in the `<dl>` (F22.5c).
- [ ] **Provenance badges appear**: each resolved field shows where it came from — provider fields "from {provider}", any file-sourced field "from file" (F22.7a).
- [ ] Enrich run shows in **`/status` 30-day history** as `kind=enrich` with provider, entity, outcome (F22.6b).

**Re-enrich + clear**

- [ ] **Re-enrich** (click Enrich again on the same person): the picker is **skipped** — goes straight to `/enrich` with the stored `external_id`; toast on completion (F22.4b, handoff re-enrich).
- [ ] **Clear-provider**: the owner-only "Clear {provider} data" button (muted) prompts confirm-before-act; on confirm, the provider's `entity_enrichment` rows are removed and fields **fall back to the next source** (file or empty) (F22.7b, F22.4c).
- [ ] After clear, file-sourced fields are **untouched** (non-destructive shadow layer — F22.4c/F22.3c).

---

## 3. Matching paths

- [ ] **Embedded-ID auto-match** (rare for People — simulate): give the test Person a known external ID (paste/store path, or seed `entity_enrichment.external_id`), then enrich → core resolves **deterministically, no picker** (F22.5b). Document how you simulated it (People rarely carry IDs until Series/Video generalization).
- [ ] **Name-search + manual confirm** (the dominant path): no embedded ID → picker shows candidates → owner picks one → confirm. Verified in §2.
- [ ] **No results**: search the no-match person → "No matches for '{query}'." (muted), input stays focused to retype (handoff edge case).
- [ ] **Ambiguous**: multiple similar candidates returned → all listed with disambiguation lines; owner stays in control, nothing auto-applies (always-confirm in v1, spec OQ#5).

---

## 4. States

- [ ] **Loading**: picker search shows `AsyncState` loading "Searching {provider}…" (muted, centered).
- [ ] **Empty**: pre-search / cleared input shows help text; no spurious call.
- [ ] **Error — provider unreachable**: stop the fake (or point `base_url` at a dead host), search → picker shows an inline **`border-warn`** message "{provider} is unavailable right now." — a single failure, not a page break (F22.2c, F22.9b, handoff edge case). The page and other providers keep working.
- [ ] **CJK aliases** (e.g. 宮崎駿): provider-supplied CJK aliases render in the `<dl>` and alias chips **without tofu** (boxes); verify in body `font-ui` (handoff edge case — re-check per skin in §5).
- [ ] **Photo asset — success**: `assets.photo` downloads (core-side, not provider-redirected), stored under the data dir, shown on the person card/page; `thumb-shimmer` hook shows while downloading (F22.5e).
- [ ] **Photo asset — fallback**: with a broken/absent photo URL, the UI falls back to the existing no-photo treatment and **does not block** field display (handoff edge case).
- [ ] **Slow connection**: throttle to Slow 3G — all provider calls are explicit and show their loading state; nothing auto-polls (unlike activity) (handoff edge case).

---

## 5. Three-skin visual QA

Render `/people/[id]` and exercise the picker in **each** skin via the header picker. Repeat the sub-checklist three times — regressions routinely appear in only one skin.

Reference tokens (from `app.css`): radius `--radius` = **2px / 0 / 0**; accent = Cinémathèque gold `#e8a33d` / Broadcast cyan `#36e0d0` / Brutalist lime `#d6ff3f`; warn = `#e2603f` / `#ff6f61` / `#ff5e3a`.

### 5a. Cinémathèque (`data-theme='cinematheque'`)

- [ ] **Provenance chips**: file = muted pill on `bg-surface-2`; provider = **outlined-accent** pill (`border-accent`/`text-accent`) — gold outline reads on `bg-surface`, **does NOT use `--warn`** (`#e2603f`) and is distinguishable from any solid-accent CTA.
- [ ] **Picker panel + backdrop**: radius is **2px**; backdrop dims (`bg-bg/70`); no error-colored borders on normal states.
- [ ] **Confidence chip**: "Strong match" accent text legible on gold.
- [ ] **`.skin-title`**: serif display face (Fraunces), normal casing, no caret.
- [ ] **Focus ring** (`focus:border-accent`) visible on input + buttons.
- [ ] **Reduced-motion**: picker open is instant (no fade/scale) when `prefers-reduced-motion: reduce`.

### 5b. Broadcast (`data-theme='broadcast'`)

- [ ] **Provenance chips**: file = muted pill; provider = outlined-cyan pill — legible on `bg-surface`, **NOT** the coral `--warn` (`#ff6f61`); distinct from solid-accent CTA.
- [ ] **Picker panel + backdrop**: radius is **0** — **no stray rounded corners** anywhere on the picker.
- [ ] **Confidence chip**: "Strong match" cyan text legible (bright accent — eyeball contrast).
- [ ] **`.skin-title`**: VT323 mono display, **UPPERCASE**, **caret `▮`** flourish after the heading.
- [ ] **Focus ring** visible on input + buttons.
- [ ] **CJK aliases** (宮崎駿) render without tofu in the mono UI face (Share Tech Mono falls back for CJK — confirm).
- [ ] **Reduced-motion**: picker open instant.

### 5c. Brutalist (`data-theme='brutalist'`)

- [ ] **Provenance chips**: file = muted pill; provider = outlined-lime pill — legible on `bg-surface`, **NOT** the red-orange `--warn` (`#ff5e3a`); distinct from solid-accent CTA.
- [ ] **Picker panel + backdrop**: radius is **0** — no stray rounded corners.
- [ ] **Confidence chip**: "Strong match" lime text legible (very bright accent — eyeball contrast).
- [ ] **`.skin-title`**: Spline Sans Mono, **UPPERCASE**, no caret (Broadcast-only).
- [ ] **Focus ring** visible on input + buttons.
- [ ] **CJK aliases** render without tofu in the mono face.
- [ ] **Reduced-motion**: picker open instant.

### 5d. All-skin sweep (each skin)

- [ ] **Loading / empty / error / populated** states all themed — no raw white/black, no hardcoded color leaking through.
- [ ] Provenance chip does not **collide** with the resolution/quality badge or the active-accent state at any viewport (desktop / mobile <640).

---

## 6. Accessibility

- [ ] **Dialog**: picker has `role="dialog" aria-modal="true"`, labelled by its heading (`aria-labelledby`). Focus is **trapped** inside.
- [ ] **Esc** closes the picker; **focus returns** to the Enrich button on close (also on backdrop click and close button).
- [ ] **Combobox/listbox**: input has `role="combobox" aria-expanded aria-controls aria-activedescendant`; list `role="listbox"`; rows `role="option" aria-selected`.
- [ ] **Keyboard nav**: ↑/↓ move the active option (wrap/clamp matching search-history), **Enter** confirms active, **Esc** closes, **Tab** reaches Confirm/Cancel. Whole flow operable **mouse-free**.
- [ ] **`aria-activedescendant`** tracks the active row as you arrow through (inspect in devtools).
- [ ] **`aria-live="polite"`** results region announces the count ("3 matches") after a search.
- [ ] **Confidence color is not the sole signal**: the word ("Strong match") is present, not color alone (color-blind safe).
- [ ] **Provenance badges** carry `aria-label="source: from {label}"` (full phrase, not just the bare provider name).
- [ ] **Owner controls when not owner are ABSENT from the DOM** — not visually hidden — so nothing misleading appears in the a11y tree (inspect the accessibility tree with no token).

---

## 7. Security-adjacent manual checks

- [ ] With `ADMIN_TOKEN` **set** and **no token**, hit the enrichment endpoints directly (`/resolve`, `/enrich`, clear) → all return **401** (F22.9a). The SPA also hides the controls.
- [ ] **No upstream API keys** appear in: the read-model / `/api` responses, the `/status` page, or the logs during an enrich (F22.9d, F22.8a). The fake/provider owns its own key in its container env only.
- [ ] **Untrusted-response handling**: point at a fake variant returning **overlong / garbage** field values and a malformed asset → values are **length-capped and sanitized** in the UI (no layout break, no raw HTML injection), asset download is size- and content-type-limited, and a malformed response **fails the single fetch, not the server** (F22.9b).
- [ ] **SSRF posture**: a `/resolve` or `/enrich` response that names an **unconfigured host** (or a redirect to one) does **not** cause core to call it — core only ever calls the allowlisted `base_url` and does not follow provider redirects (F22.2b, F22.9c). Confirm via outbound-call logging.

> Reminder (CLAUDE.md routing): this feature touches access + outbound network → a **`/security-review` sign-off is required before merge** in addition to these manual checks.

---

## 8. Token-discipline gate

- [ ] Run the exact guard from CLAUDE.md against the new components — it must return **empty**:

```powershell
rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'
```

- [ ] No literal palette / hex / named-font / fixed-radius in the new markup (`EnrichButton`, `EnrichPicker.svelte`, `CandidateRow`, `ProvenanceBadge`, the person `<dl>`). Any skin-specific flourish lives in `app.css` gated by `[data-theme]` on a shared hook class, not per-component markup.
- [ ] `rounded-full` on provenance/alias chips is the **only** intentional fixed radius (pill shape — allowed); everything else uses `rounded-theme`.
