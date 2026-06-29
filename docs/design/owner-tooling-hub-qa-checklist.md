# Manual QA Checklist: Owner tooling hub + nav split (F35)

**Spec**: [Owner tooling hub (F35)](../specs/owner-tooling-hub.md) · **Gate**: [ADR-030](../architecture/ADR-030-access-control-gating-seam.md) · **Design**: [handoff](owner-tooling-hub-handoff.md)
**Builds on**: [Admin Mode (F29)](../specs/admin-mode.md) — same toggle, relabeled.
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped into sections **by verifier**, so each actor runs only their own:
> - **§2 Smoke** — automated test or build gate (`svelte-check`, token-guard `rg`, redirect/route test). Green build = pass; pre-checked `[x]` with the test named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/routes/`localStorage`/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (per-skin "look", legibility, the at-a-glance read).
>
> §1 is one-time **setup** that §3 and §4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.
>
> **Core invariant for every item:** this is an information-architecture + visibility change. The server
> `requireOwner` gate is unchanged and remains the **only** authority — hiding the gear or a route is
> presentation/navigation, never the security boundary (spec P0-4). Hidden ≠ unauthorized.

---

## 1. Setup / preconditions

> **Who does this:** whoever sets up the session (a developer or the agent) — *not* the §4 human. **Quick "is it ready?" check:** open the app as owner; in the header, the content nav should read just **Media · People · Tags**, and to the right (near the skin picker) there's a **gear "Owner"**. Click it → you land on an **Owner** page with tabs **Status · Metadata keys · Trash**. That's the feature.

- [ ] 1.1 App running with a library that has captured **extended metadata** (so Metadata keys has rows), at least one **soft-deleted item** (so Trash is non-empty), and live **system activity** (so Status has cards/jobs). Dev: `--media-path E:/AMVTestCopy --host 127.0.0.1`.
- [ ] 1.2 Be set up as **owner**: either `ADMIN_TOKEN` unset (open/owner), or set then unlocked via `/owner/status`. Note the `/owner`, `/owner/status`, `/owner/keys`, `/owner/trash`, and **old** `/status` `/keys` `/trash` URLs.
- [ ] 1.3 Devtools open (Console for errors; Application → Local Storage to watch `holodex-admin-mode`; Network to confirm no owner data on visitor loads). Skin picker reachable in the header.
- [ ] 1.4 Know how to toggle **Owner view / Preview** (the relabeled F29 switch) and how to simulate a **non-owner** (token set, none entered) for the gating checks.

---

## 2. Smoke — automated (green in CI)

- [ ] 2.1 **Routes** — `/owner`, `/owner/status`, `/owner/keys`, `/owner/trash` resolve; the three pages render under the `owner/` group via the shared `+layout`. *(route/`svelte-check` build.)*
- [ ] 2.2 **Redirects** — `/status` → `/owner/status`, `/keys` → `/owner/keys`, `/trash` → `/owner/trash` (permanent redirect). *(unit/integration test on the redirect loaders.)*
- [ ] 2.3 **`svelte-check`** passes with the moved pages, the new `owner/+layout`, the header edits, and the relabeled toggle strings.
- [ ] 2.4 **Token-discipline guard** empty against the changed markup: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` — gear/tabs use `rounded-theme`/`border-rule`/`bg-surface-2`/`text-accent` only; gear SVG uses `currentColor`.
- [ ] 2.5 **No "Admin" left in the header UI** — `rg -i '"Admin"|>Admin<|aria-label="Admin' web/src/routes/+layout.svelte` returns nothing user-facing (the relabel is complete). *(The internal `adminMode` store/key are intentionally unchanged — out of scope.)*
- [ ] 2.6 **Owner gate on the group** — the `owner/+layout` enforces `effectiveOwner` (redirect/auto-reveal); a non-owner request does not render owner data. *(integration test if available.)*

> `/security-review` sign-off required before merge in addition to these (touches owner-gating — ADR-030). Focus: the relocation removes only public **link visibility** and adds **route gating**; every owner-only **API** still enforces `requireOwner` regardless of nav.

---

## 3. Agent — drive the running app

**Header — content nav & gear**

- [ ] 3.1 **Owner, Owner-view ON**: the content nav is exactly **Media · People · Tags** — no Keys/Status/Trash links in that row.
- [ ] 3.2 The **Owner gear** is present in the chrome cluster, after the content/chrome `border-l` separator, in order `ActivityIndicator → Owner-view toggle → Owner gear → skin picker`.
- [ ] 3.3 The gear is an `<a href="/owner">` with `aria-label="Owner tools"`; keyboard-reachable; focus-visible ring present.
- [ ] 3.4 On an `/owner` route the gear shows **active** state: `text-accent` and `aria-current="page"` (no `bg-accent` fill on the gear).

**Toggle relabel (spec P0-7)**

- [ ] 3.5 The view toggle's visible label (≥`sm`) is **"Owner view"**, `aria-label="Owner view"`; it is still `role="switch"` with `aria-checked` reflecting state. The string **"Admin"** appears nowhere in the header DOM.
- [ ] 3.6 Toggling still flips `adminMode.enabled` and persists to `localStorage['holodex-admin-mode']` (key unchanged) with no reload.

**Hub & tabs (spec P0-1)**

- [ ] 3.7 `/owner` renders the shell: a `skin-title` **"Owner"** `<h1>`, a subtitle, and a tab row **Status · Metadata keys · Trash**, with a default tab selected (Status).
- [ ] 3.8 Each tab is an `<a>` to `/owner/{status|keys|trash}`; clicking moves `aria-current="page"` and renders that page **without a full-app reload**; the URL updates.
- [ ] 3.9 The **active** tab is `bg-surface-2 text-ink` (NOT `bg-accent`); inactive tabs are `text-muted` with `hover:text-ink`.
- [ ] 3.10 Each tab's content matches the old page (Metadata-keys table, Status cards/jobs, Trash list) — relocated, not restyled.

**Redirects & gating (spec P0-4, P0-5)**

- [ ] 3.11 Visiting old `/status`, `/keys`, `/trash` lands on the `/owner/*` equivalent (permanent redirect).
- [ ] 3.12 **Non-owner** (token set, none entered): the **gear is absent** from the DOM; visiting `/owner` or any `/owner/*` URL **redirects home** and renders **no** owner data (e.g. no metadata-keys rows, no trash items) — verify the network response carries no owner payload.
- [ ] 3.13 **Visitor leak closed**: as a non-owner, the header shows only Media/People/Tags + search + activity + skins — confirm no `/keys` or `/status` link is present (the pre-F35 leak).

**Auto-reveal at the group gate (spec P0-6)**

- [ ] 3.14 With **Preview ON (visitor view)** as owner, navigate directly to **`/owner/trash`** (URL bar) → Owner view **flips ON** (toggle `aria-checked="true"`, stored `"true"`), the hub + page render **fully**, and the `aria-live="polite"` region announces **"Owner view on."**
- [ ] 3.15 Auto-reveal fires **once at the `/owner` layout**: switching between tabs afterward does not re-announce or re-flip; navigating to a **public** route (`/`, `/media/[id]`) while in Preview does **not** flip the toggle.

**A11y & responsive**

- [ ] 3.16 Nav focus order: `ActivityIndicator → Owner-view toggle → Owner gear → skin-picker buttons`.
- [ ] 3.17 Active gear/tab convey state by more than color (`aria-current` present alongside `text-accent` / `bg-surface-2`).
- [ ] 3.18 Below `sm`: the gear **drops its "Owner" label, keeps the icon**, retains `aria-label="Owner tools"`; the header doesn't wrap awkwardly; the tab row wraps without horizontal scroll.

---

## 4. Human — needs your eyes (all three skins)

> **How to run this:** open the app in a browser as the owner. In the header there's a **skin picker** — run every item below **three times**, once in each skin: **Cinémathèque**, **Broadcast**, **Brutalist**. You're checking it *looks right* and reads clearly — not that links work (the agent checked that). The new bits: the top nav is now just **Media · People · Tags**, and the owner tools moved behind a **gear "Owner"** near the skin picker that opens an **Owner** page with tabs.

- [ ] 4.1 **The bar looks calmer and reads in tiers.** The three content links (Media/People/Tags) sit together; the cluster on the right (activity dot, the **Owner view** eye switch, the **Owner** gear, the skin swatches) reads as a separate "my controls" group, divided by a faint vertical line. It shouldn't feel like one long undifferentiated list of links anymore.
- [ ] 4.2 **The gear reads as "owner tools."** It's a cog with an "Owner" label (on a wide window), quiet grey when you're elsewhere, and picks up the skin's **highlight color** (gold / cyan / lime) when you're on an Owner page — *as colored text, not a filled button.* Only the **Owner view** eye switch is allowed to be a solid filled pill.
- [ ] 4.3 **No more "Admin" anywhere.** The old "Admin" toggle now says **"Owner view"**; nothing in the header says "Admin". The eye switch still clearly shows on (filled) vs off (outline).
- [ ] 4.4 **The Owner page looks native.** The heading "Owner" uses the skin's display font (serif on Cinémathèque, the pixel/mono faces on Broadcast/Brutalist). The tabs **Status · Metadata keys · Trash** sit in a neat row above a thin dividing line.
- [ ] 4.5 **The selected tab is obvious but quiet.** The active tab has a subtle filled-panel background (not the bright highlight color); the others are muted text that brighten on hover. You can tell which tab you're on at a glance. Pay extra attention on **Broadcast**, where the panel shade is close to the page — confirm the active tab still clearly looks selected.
- [ ] 4.6 **Corner shape matches the skin** — softly rounded on Cinémathèque, **squarer** on Broadcast and Brutalist, for both the gear and the tabs, like the controls around them.
- [ ] 4.7 **Switching tabs feels in-place.** Clicking Status → Metadata keys → Trash swaps the content below the tabs without the whole app flashing or jumping to the top.
- [ ] 4.8 **A visitor sees a clean public bar.** Turn **Owner view off** (the eye switch): the gear disappears and the bar is just Media/People/Tags + search + the skin picker — no owner tools peeking through. Do this in all three skins.
- [ ] 4.9 **Direct link to an Owner page while in visitor view just works.** With Owner view off, paste `/owner/trash` and go: it should quietly switch you back to owner view and show the page — never an empty or "forbidden" screen.
- [ ] 4.10 **Old bookmarks still land.** Paste the old `/status` (and `/keys`, `/trash`) URLs: each should take you to the matching Owner tab, not a 404.
- [ ] 4.11 **Narrow window** — drag the browser narrow: the gear keeps just its icon (label tucks away) and the three little icons (activity, eye, gear) stay distinguishable; the tab row wraps tidily instead of scrolling sideways.
- [ ] 4.12 **Overall it looks intentional** — the gear reads as a sibling of the skin picker and Owner-view switch (a deliberate "my tools" control), and the Owner page feels like a real section of the app, not three old pages bolted together.
