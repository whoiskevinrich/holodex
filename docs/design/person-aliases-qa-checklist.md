# Manual QA Checklist: Person Aliases (F23)

**Spec**: [Person Aliases (F23)](../specs/person-aliases.md) · **ADR**: [ADR-036](../architecture/ADR-036-person-alias-search-indexing.md) · **Design**: [handoff](person-aliases-handoff.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped into sections **by verifier**, so each actor runs only their own:
> - **§2 Smoke** — covered by an automated test or build gate (`go test`, `svelte-check`, the token-guard `rg`). Green build = pass; pre-checked `[x]` with the test named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/network/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (legibility, contrast, aesthetics, per-skin "look", CJK tofu).
>
> §1 is one-time **setup** that §3 and §4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.

---

## 1. Setup / preconditions

> **Who does this:** whoever sets up the session (a developer or the agent) — *not* the §4 human. **Quick "is it ready?" check:** open the app, go to **People → any person**, and confirm you see an **"Also known as"** panel. If `ADMIN_TOKEN` is unset you'll also see an add field; that means you're set up as owner.

- [ ] 1.1 App running with a library that has at least one **person** with videos (note their `/people/[id]` URL). Dev: `--media-path E:/AMVTestCopy --host 127.0.0.1`.
- [ ] 1.2 Exercise **both token states**: `ADMIN_TOKEN` unset (open/owner) vs set (locked → unlock via `/status`).
- [ ] 1.3 Pick a person and have a couple of test aliases ready, including **one with diacritics** ("Beyoncé") and **one CJK** ("宮崎駿") for §4.
- [ ] 1.4 Devtools open (Network + Console); skin picker reachable (header); a `prefers-reduced-motion: reduce` profile ready.

---

## 2. Smoke — automated (green in CI)

- [ ] 2.1 **Alias store CRUD + per-person case-insensitive uniqueness** — add trims + rejects empty/over-200; "Rob"/"rob" dedupe to one; same alias on two people allowed (F23.1). *(repo `TestPersonAliasesCRUD`.)*
- [ ] 2.2 **Delete scoped to person; cascade on person delete** — delete by id only affects that person's alias; unknown/foreign id → not found; deleting the person removes its aliases (F23.3/F23.7). *(repo `TestPersonAliasesDeleteScopeAndCascade`.)*
- [ ] 2.3 **Search matches aliases + dedup + diacritic fold** — alias surfaces the person; name+alias both matching yields one entry; "beyonce" matches "Beyoncé" (F23.5). *(repo `TestSearchMatchesAlias`.)*
- [ ] 2.4 **Aliases survive a re-scan** — a full upsert pass leaves aliases intact (ADR-036 invariant). *(repo `TestAliasesSurviveRescan`.)*
- [ ] 2.5 **Endpoints owner-gated + validation** — `POST`/`DELETE …/aliases` → 401 without token when gated; 400 invalid; 404 unknown person/alias; `GET /people/{id}` carries `aliases` (F23.2–F23.4). *(api `TestAliasEndpointsGatedAndValidated`, `TestGetPersonIncludesAliases`.)*
- [ ] 2.8 **Scan-time resolution + merge survives re-scan** — an alias-tagged file links to the canonical person (no duplicate), and re-scanning keeps it merged (F23.8, cardinal invariant). *(repo `TestScanResolvesAliasToCanonical`, `TestMergePersons`.)*
- [ ] 2.9 **Merge mechanics** — de-duped video union, name→alias, prior aliases re-pointed, duplicate deleted; self-merge & unknown-id error (F23.9). *(repo `TestMergePersons`, `TestMergePersonsValidation`.)*
- [ ] 2.10 **Collision detection, no auto-merge** — a name owned by another person is detected and the add endpoint returns 409 with that person (F23.10). *(repo `TestPersonConflict`, api `TestAddAliasConflict409`.)*
- [ ] 2.11 **Merge endpoint owner-gated** — `POST …/merge` → 401 without token, 400 self-merge, 404 unknown, 200 + alias on success (F23.9/F23.11). *(api `TestMergeEndpoint`.)*
- [ ] 2.6 **`svelte-check`** passes with the new `aliases` types + page changes.
- [ ] 2.7 **Token-discipline guard** empty against the changed markup: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` (the chip's `rounded-full` pill is the only intentional fixed radius).

> `/security-review` sign-off is required before merge in addition to these (touches the owner gate — ADR-030).

---

## 3. Agent — drive the running app

**Owner-gating & control visibility**

- [ ] 3.1 **Owner** (open, or token + unlocked): the "Also known as" panel shows an **add field + "Add" button**, and each existing chip shows a **✕** (F23.6).
- [ ] 3.2 **Non-owner** (token set, none entered): chips render **read-only**; the add field and every ✕ are **absent from the DOM** (not hidden) (F23.6).
- [ ] 3.3 **Empty + owner**: with no aliases, the panel still renders for the owner (heading + add field + "No aliases yet."). **Empty + non-owner**: the panel is **absent entirely**.

**Add / delete flow**

- [ ] 3.4 Type an alias + Enter (and separately + click "Add") → `POST /people/{id}/aliases` fires; on success a **new chip appears**, the **input clears**, and **focus stays in the input** (multi-add) (F23.2).
- [ ] 3.5 Add a **duplicate** (existing alias, any case) → idempotent: **no second chip**, **no error** shown, input clears.
- [ ] 3.6 Add an **empty/whitespace-only** value → no request fired (client guard) OR 400 handled; inline `text-warn` words shown, typed text preserved.
- [ ] 3.7 Add a **>200-char** value → server 400 → inline `text-warn` "Alias is too long." (words, not color-only), typed text preserved.
- [ ] 3.8 Click a chip's ✕ → `DELETE …/aliases/{aliasId}` → chip removed **optimistically**; on a forced failure the chip is **restored** and an inline error shows.
- [ ] 3.9 After a delete, focus lands on the next chip's ✕ or the add input — **never `<body>`** (the F22 focus-return lesson).

**Search integration (ADR-036 end-to-end)**

- [ ] 3.10 Add alias "Ziggy" to a person → type "zig" in the global search box → the **person appears** in results → click → lands on that person's page.
- [ ] 3.11 With a person whose **name and an alias both match** a query, they appear **once** in the people group.
- [ ] 3.12 Delete the alias → the same query **no longer surfaces** them via that alias.

**Merge — person page**

- [ ] 3.16 Owner sees a **"Merge a person in…"** button; non-owner does not (absent from DOM).
- [ ] 3.17 Click it → `PersonPicker` modal opens (focus to input); the list **excludes the current person** and shows each person's video count.
- [ ] 3.18 Pick a person → **informed confirm** step shows "Merge {name} ({n} videos) into {canonical}?" with Back/Merge; **Merge** → modal closes, the page reloads with the **enlarged video list** and the merged name as a **new alias**; the merged person's page returns 404.
- [ ] 3.19 Re-scan (admin rescan) → the merge **holds** (no duplicate person reappears; alias-tagged files still route to canonical).

**Merge — alias collision**

- [ ] 3.20 Typing an alias that names a **different existing person** does **not** add silently — an inline prompt names that person (+ video count) and offers **merge** / **keep separate** (F23.10).
- [ ] 3.21 **Keep separate** dismisses the prompt and adds nothing; **merge them in** performs the merge (videos move, alias created).

**Merge — People list**

- [ ] 3.22 Owner **"Merge people…"** toggles checkbox select mode (non-owner: no toggle); "Merge N selected" is **disabled until ≥2** chosen.
- [ ] 3.23 Choosing canonical in the **"Keep which name?"** dialog and confirming folds the rest in; the list reloads with the duplicates gone.

**A11y**

- [ ] 3.24 Each ✕ is a real `<button>` with `aria-label="Remove alias {alias}"` (not a bare glyph); keyboard-activatable (Enter/Space).
- [ ] 3.25 The add input has an accessible name; Enter submits; the "Add" control is a real button.
- [ ] 3.26 Both merge modals are `role="dialog" aria-modal="true"`, focus-trapped, Esc-closable, focus returns to the trigger; the picker list is a `role="listbox"` with roving tabindex (↑/↓/Enter).
- [ ] 3.27 The chip-list region is `aria-live="polite"` so add/remove is announced; the validation error is associated via `aria-describedby`.

---

## 4. Human — needs your eyes (all three skins)

> **How to run this:** open the app in a browser. In the header there's a **skin picker** — you'll run every item below **three times**, once in each skin: **Cinémathèque**, **Broadcast**, **Brutalist**. Go to **People → (a person you've added a few aliases to)**. The "Also known as" panel is just under the person's name, above the video grid. You're checking it *looks right* and is readable — not that buttons work (the agent already checked that).

- [ ] 4.1 **Chips are readable** in all three skins — the alias text sits clearly on its pill; the pill is a soft, quiet shape (it should *not* shout for attention the way the "Add" button does). *(token ref: `bg-surface-2` pill, `text-ink` text.)*
- [ ] 4.2 **The ✕ remove mark** is visible at rest (a muted grey) and **changes to the skin's highlight color** when you hover or tab to it — lime on Brutalist, cyan on Broadcast, the warm tone on Cinémathèque. It must **not** turn red/orange (that color is reserved for errors). *(token ref: `text-muted` → `text-accent` on hover; never `--warn`.)*
- [ ] 4.3 **The panel card edges** match the skin: softly rounded on Cinémathèque, **square** on Broadcast and Brutalist. Only the little alias pills stay fully rounded everywhere (that's intentional). *(token ref: `rounded-theme` card vs `rounded-full` chip.)*
- [ ] 4.4 **The "Add" button** reads clearly — it's the skin's main highlight color with legible text on top, the same look as other primary buttons in the app.
- [ ] 4.5 **Type in the add field** — the box's outline should pick up the skin's highlight color when focused, and the text you type is easy to read.
- [ ] 4.6 **Error wording** — trigger an error (paste a very long alias and Add). The message must be readable and in the **error color (a red/orange), clearly different from the highlight color** — this separation matters most on Brutalist, where the highlight is bright lime. The message should be *words you can read*, not just a colored box.
- [ ] 4.7 **Accented + non-Latin names render** — add "Beyoncé" and "宮崎駿"; both must show their actual characters, **no boxes/▯/garbled glyphs**, in all three skins (the Broadcast/Brutalist fonts are blocky — make sure the CJK name still appears).
- [ ] 4.8 **Heading** "Also known as" reads correctly (small, muted) in each skin.
- [ ] 4.9 **Overall it looks intentional** — the aliases panel sits comfortably above the Enrichment panel, spacing feels even, nothing overlaps or collides at a narrow window width (drag the window narrow — chips and the add row should wrap gracefully).
- [ ] 4.10 **Reduced motion** — with "reduce motion" on in your OS, adding/removing a chip and opening the merge dialogs shouldn't animate distractingly.
- [ ] 4.11 **Merge picker looks right** — click "Merge a person in…": the pop-up has the skin's card edges (rounded on Cinémathèque, square on the other two), the highlighted row uses the skin's accent on its left edge, and the **Merge** button is the solid accent while **Back** is a plain outline. The confirm sentence with the two video counts is readable.
- [ ] 4.12 **Collision prompt looks right** — when it appears, "Yes, merge them in" (accent) and "No, keep separate" (outline) are clearly distinct, and the explanatory text reads on its panel in every skin.
- [ ] 4.13 **People-list select mode** — the row checkboxes and the "Keep which name?" radio buttons show the **skin's accent color** (lime/cyan/gold), not a default blue; selected rows get the accent edge; the "Merge … selected" button reads clearly.
