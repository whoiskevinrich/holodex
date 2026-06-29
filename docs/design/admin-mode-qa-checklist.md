# Manual QA Checklist: Admin Mode (F29)

**Spec**: [Admin Mode (F29)](../specs/admin-mode.md) · **Gate**: [ADR-030](../architecture/ADR-030-access-control-gating-seam.md) · **Design**: [handoff](admin-mode-handoff.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped into sections **by verifier**, so each actor runs only their own:
> - **§2 Smoke** — covered by an automated test or build gate (`svelte-check`, the token-guard `rg`, store unit test). Green build = pass; pre-checked `[x]` with the test named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/state/`localStorage`/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (legibility, contrast, the per-skin "look", the at-a-glance ON/OFF read).
>
> §1 is one-time **setup** that §3 and §4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.
>
> **Core invariant for every item:** the toggle is **presentation only** — it never changes the admin
> token, capabilities, or any server authorization (spec P0-5). Hidden ≠ unauthorized.

---

## 1. Setup / preconditions

> **Who does this:** whoever sets up the session (a developer or the agent) — *not* the §4 human. **Quick "is it ready?" check:** open the app as owner; in the header, to the **left of the skin picker**, you should see an **"Admin" toggle**. Click it off and the **Trash** nav link disappears; click it on and Trash returns. That's the feature.

- [ ] 1.1 App running with a library that has at least one **person with images/enrichment** and one **media item** that carries owner-only controls (enrich, writeback, delete, regenerate). Dev: `--media-path E:/AMVTestCopy --host 127.0.0.1`.
- [ ] 1.2 Be set up as **owner**: either `ADMIN_TOKEN` unset (open/owner), or set then unlocked via `/status`. Note the `/media/[id]`, `/people/[id]`, `/people`, and `/trash` URLs for the sweep.
- [ ] 1.3 Devtools open (Application → Local Storage to watch `holodex-admin-mode`; Console for errors). Skin picker reachable (header); a `prefers-reduced-motion: reduce` profile ready.
- [ ] 1.4 Have a **second browser/profile with no `holodex-admin-mode` value** ready (to test the default-ON first-run), and know how to clear the key.

---

## 2. Smoke — automated (green in CI)

- [ ] 2.1 **`adminMode` store** — `enabled` defaults **true**; `init()` restores a stored `false`; absent/garbage `localStorage` value falls back to **true** without throwing; `set`/`toggle` persist to key `holodex-admin-mode`. *(web unit test `adminMode.test.ts`, mirroring the `theme` store guards.)*
- [ ] 2.2 **`svelte-check`** passes with the new store + the effective-gate (`isOwner && adminMode.enabled`) changes across the gated pages/components.
- [ ] 2.3 **Token-discipline guard** empty against the changed markup: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` (the control reuses `rounded-theme`/`bg-accent`/`border-rule` only; no literals; an inline icon SVG must use `currentColor`).
- [ ] 2.4 **No new server surface** — confirm the diff adds **no** backend route/handler/migration (frontend-only quick win; the gate stays server-side, unchanged).

> `/security-review` sign-off is required before merge in addition to these (touches the owner-gate surface — ADR-030). Focus: the toggle changes **no** authorization, and every owner-only **API** still enforces `requireOwner` regardless of toggle state.

---

## 3. Agent — drive the running app

**Control visibility & gating**

- [ ] 3.1 **Owner**: the **"Admin" toggle** is present in the header, between `ActivityIndicator` and the skin picker.
- [ ] 3.2 **Non-owner** (token set, none entered): the toggle is **absent from the DOM** (not hidden), and the app behaves exactly as today.
- [ ] 3.3 The control is a real `<button role="switch">` with `aria-checked` reflecting state and an accessible name "Admin mode"; keyboard-activatable (Enter **and** Space).

**Toggle behavior & persistence**

- [ ] 3.4 Toggle **OFF** → `aria-checked="false"`, `localStorage['holodex-admin-mode'] === "false"`, and **all** owner-gated surfaces update **with no page reload**.
- [ ] 3.5 Toggle **ON** → `aria-checked="true"`, stored `"true"`, controls return — still no reload.
- [ ] 3.6 Toggle OFF, **reload** the page → still OFF after mount (persisted, restored in `init()`).
- [ ] 3.7 **Fresh profile, no stored key**, authenticate as owner → Admin mode is **ON** by default (controls visible).
- [ ] 3.8 **Rapid toggling** (flip several times quickly) → no console errors, state ends consistent with the last click, no orphaned DOM.

**Complete hide-set (spec P0-4) — with Admin mode OFF, each owner-only item is ABSENT from the DOM (not `display:none`/`opacity`)**

- [ ] 3.9 **Header**: the `/trash` nav link is gone.
- [ ] 3.10 **Home (`/`)**: the "Recently Added" owner toggle is gone.
- [ ] 3.11 **Media detail (`/media/[id]`)**: Regenerate thumbnail, Enrich, Clear-provider, Writeback, and Delete (soft/purge) controls are gone — **and** owner-only data (provenance/source badges) is gone.
- [ ] 3.12 **Person detail (`/people/[id]`)**: alias add/delete, "Merge a person in…", image upload (headshot/banner/poster), gallery management, enrich, clear-provider are gone — and owner-only badges are gone.
- [ ] 3.13 **People list (`/people`)**: the "Merge people…" control is gone.
- [ ] 3.14 **Status (`/status`)**: Rescan, Reload config, and owner-only diagnostics are gone. *(The admin-token unlock input is the path back in — confirm it follows the spec's reconciliation: reachable when needed, not a dead end.)*
- [ ] 3.15 Toggle back **ON** on each of the above → every control/datum returns; layout matches the pre-toggle state (no leftover gaps, no duplicated nodes).

**Auto-reveal on owner-only routes (spec P0-6)**

- [ ] 3.16 With Admin mode **OFF**, navigate directly to **`/trash`** (URL bar) → Admin mode **flips ON** (toggle shows `aria-checked="true"`, stored `"true"`), the page renders **fully**, and a visually-hidden `aria-live="polite"` region announces **"Admin mode on."**
- [ ] 3.17 Navigating to a **public** route while OFF (e.g. `/`, `/media/[id]`) does **not** flip the toggle — it stays OFF (auto-reveal is owner-only-route-scoped).

**Security boundary (presentation-only)**

- [ ] 3.18 With Admin mode **OFF**, invoke an owner-only API directly (e.g. `POST /media/{id}/writeback` with the still-present admin token) → the server **still authorizes** it (toggle changed nothing server-side). Conversely, as a true non-owner, the same call is still **rejected** — the gate is the server's, not the toggle's.

**A11y**

- [ ] 3.19 Focus order in the nav: …`ActivityIndicator` → **Admin toggle** → skin-picker buttons. Focus-visible ring present on the toggle.
- [ ] 3.20 Self-toggling never moves focus to `<body>` — the control persists, so focus stays on it after surfaces hide/show.
- [ ] 3.21 On/off meaning is conveyed by more than color: `aria-checked` **and** the icon swap (open-eye ↔ eye-slash) both change with state.

**Responsive**

- [ ] 3.22 Narrow the viewport below `sm` → the text label "Admin" hides, the **icon remains**, and `aria-label="Admin mode"` keeps the accessible name; the control stays in its nav slot.

---

## 4. Human — needs your eyes (all three skins)

> **How to run this:** open the app in a browser as the owner. In the header there's a **skin picker** — you'll run every item below **three times**, once in each skin: **Cinémathèque**, **Broadcast**, **Brutalist**. The new control is the **"Admin" toggle** just to the **left of the skin picker**. You're checking it *looks right*, reads clearly as on vs. off, and that "off" genuinely looks like the public site — not that the buttons work (the agent already checked that).

- [ ] 4.1 **On vs. off is obvious at a glance.** With Admin mode **ON**, the toggle is filled with the skin's **highlight color** with legible text on top (lime on Brutalist, cyan on Broadcast, the warm tone on Cinémathèque). With it **OFF**, it's a quiet outline in muted grey. You can tell which state you're in without reading the label. *(token ref: `bg-accent`/`text-accent-ink` ON; `text-muted` + `border-rule` OFF.)*
- [ ] 4.2 **The icon matches the state** — an open eye when on, an eye with a slash when off (or the chosen admin glyph). It changes when you click, so the meaning isn't carried by color alone.
- [ ] 4.3 **It sits comfortably beside the skin picker** — same height and corner feel, even spacing, no collision or crowding with the skin swatches or the activity indicator, in every skin. *(token ref: shared `rounded-theme`/`border-rule` shell.)*
- [ ] 4.4 **Corner shape matches the skin** — softly rounded on Cinémathèque, **squarer** on Broadcast and Brutalist, like the controls around it.
- [ ] 4.5 **Contrast holds when ON** — the label/icon on the filled accent is easy to read in all three, with special attention to **Broadcast** (brightest accent) where dark-on-bright can get tight.
- [ ] 4.6 **"Off" really looks like the public site.** Turn Admin mode off, then visit a **media page** and a **person page**: no admin buttons, no source/provenance badges, no Trash link — and crucially **no empty gaps or holes** where controls used to be. It should read as a clean visitor page, not an admin page with things blanked out. Do this in all three skins.
- [ ] 4.7 **Turning it back on restores everything** — the controls reappear in their normal places, nothing looks shifted or doubled.
- [ ] 4.8 **Direct link to Trash while off** — with Admin mode off, paste the `/trash` URL and go: the page should just **work** (it quietly switches you back to admin view), not show an empty or "forbidden" page.
- [ ] 4.9 **Narrow window** — drag the browser narrow: the toggle keeps just its icon (label tucks away) and still reads clearly as on/off; the header doesn't wrap awkwardly.
- [ ] 4.10 **Reduced motion** — with "reduce motion" on in your OS, flipping the toggle shouldn't cause distracting animation; controls appearing/disappearing should feel calm (instant or a gentle fade), never a jarring jump.
- [ ] 4.11 **Overall it looks intentional** — the toggle reads as a sibling of the skin picker (a deliberate "view" control), not a bolted-on extra; the ON accent doesn't clash with the skin swatches right next to it.
