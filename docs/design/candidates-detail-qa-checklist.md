# QA Checklist: Revealable candidate detail in the Enrich picker (HOLODEX-380)

**Spec**: [candidates-detail.md](../specs/candidates-detail.md) FR1–FR6 ·
**Handoff**: [candidates-detail-handoff.md](candidates-detail-handoff.md) ·
**Contract**: [metadata-provider-contract.md](../specs/metadata-provider-contract.md) §2.3 / §5 ·
**Jira**: HOLODEX-380

Conventions: every item is numbered `section.item` and tagged by verifier —
`[smoke]` automated tests, `[agent]` agent-driven live QA, `[human]` needs human eyes.

---

## §1 Setup

- **1.1** `[agent]` Start `backend-films` and `web` (see the preview-testbeds memory) with the
  in-process `enrich.Fake` provider configured for `video`. Owner view on — the picker is owner-only.
  **Verify the served bundle matches this worktree** (curl the dev server, grep for a string from
  this change) — a worktree with no local `.claude/launch.json` silently previews `main`.
- **1.2** `[agent]` Give the fake canned `/resolve` responses, selectable by query text:
  (a) **collision** — four candidates labelled `Harbor Lights` (one spelled `harbor  lights`) plus
  one `Harbor Lights II`, each with 2–3 `detail` lines and a `profile_url`;
  (b) **distinct** — three distinct labels, each with `detail`;
  (c) **mixed** — two `Harbor Lights`, only one carrying `detail`;
  (d) **no detail** — three candidates with no `detail` key (pre-F61 provider);
  (e) **stress** — 25 candidates all labelled `Harbor Lights`, each with 8 lines, one line 256
  chars, one candidate with `detail` but no `profile_url`;
  (f) **hostile** — 12 entries, a 400-char entry containing `\n` and `\x1b[31m`, and `"detail": []`.
- **1.3** `[agent]` For §5's unattended items, a video linked to nothing, with the fake returning
  (g) one candidate at 0.91 with `detail`; (h) four equal candidates; (i) one candidate at 0.91
  with **no** `detail` and no `searched`.

## §2 Smoke — `[smoke]`

> **Reconciled 2026-09-13 (testing gate).** This repo has no component-test harness, so
> 2.4–2.7 and 2.10 could not be automated as written — they ran as live `[agent]` items
> against the stub (results in `docs/testing-strategy.md` §5) and the rule behind them is
> `candidateDetail.ts`'s unit tests (2.3). 2.9's first half is the §12 geometry assertion
> `collapsed-detail-row-costs-one-line` (mutation-tested); its second half was measured live.
> Tags below are left as the target for a future harness.

- **2.1** `[smoke]` `sanitizeDetail` table test: under-cap passthrough; 12 → 8 entries; 400 → 256
  chars; `\n`, `\r`, `\x1b` stripped; `[]` → nil; nil → nil. Existing candidate-sanitizer test
  still covers `label` / `disambiguation` / `profile_url` unchanged.
- **2.2** `[smoke]` Decoder: a `/resolve` body with `detail` on one candidate and none on another
  yields `Detail` set on exactly that one; the JSON to the client omits the key when nil.
- **2.3** `[smoke]` `collisionOpen()` unit: fixture (a) opens the four `Harbor Lights` and not
  `Harbor Lights II`; `harbor  lights` counts as a collision; fixture (c) opens only the member with
  `detail`; fixture (b) opens nothing.
- **2.4** `[smoke]` Picker (testing-library): no toggle without `detail`; toggle present with
  `aria-expanded="false"` and no `<ul>` in the DOM; click → `aria-expanded="true"`, `<ul>` with one
  `<li>` per line in order, text verbatim; click again → collapsed.
- **2.5** `[smoke]` Picker: clicking the toggle does **not** call `onConfirm`; Enter/Space on the
  toggle toggle and do not confirm; Enter with the row focused still confirms while lines are open.
- **2.6** `[smoke]` Picker: open row 2, press ↓ then ↑ — row 2 is still open. Resolve a new query —
  every row closed unless FR4 opens it.
- **2.7** `[smoke]` Picker: fixture (a) renders with the four collision rows open on first render
  and `Harbor Lights II` closed; `hide details` text on the open ones.
- **2.8** `[smoke]` Refresh-all with (g): activity entry detail contains
  `applied: <label> — <line> · <line>`; with (h): the entry is unchanged from today (no `applied:`);
  with (i): no resolve entry is written. Assert no `/` or `\` path fragment in the rendered detail.
- **2.9** `[smoke]` Geometry: a row with `detail` closed has the same `offsetHeight` as a row
  without; with fixture (e) all rows open, every `<li>`'s rect is inside the `<ul role="listbox">`'s
  scroll box.
- **2.10** `[smoke]` Focus trap: with one open row, Tab from the search box reaches row →
  `view source ↗` → `details` → footer; Shift+Tab from ✕ wraps back to the last toggle.

## §3 Agent — live, all three skins — `[agent]`

For each skin (1 Cinémathèque, 2, 3): open the media page, Enrich → `acme`, run the fixtures.
Screenshots time out on this picker — use `javascript_tool` for computed styles and geometry.

- **3.1** `[agent]` Fixture (b): each row shows `details` after `view source ↗`; computed color of
  the closed toggle equals the row's `text-muted`; `text-decoration-style` is `dotted`.
- **3.2** `[agent]` Click `details` on row 2: computed color of the toggle equals `text-ink`; the
  `<ul>` has `border-left-color` equal to the search field's `border-color` (`border-rule`); line
  color equals `text-muted`; contrast of line text against the **active** row background
  (`bg-surface-2`) ≥ 4.5:1 in every skin.
- **3.3** `[agent]` Fixture (a): four rows open on first render, one closed; the open rows' toggle
  reads `hide details`.
- **3.4** `[agent]` Fixture (e): scroll the listbox to the bottom; the last row's last line has a
  rect inside the listbox rect; the 256-char line is truncated (`scrollWidth > clientWidth`) and its
  `title` is the full text; the candidate with no `profile_url` shows `details` as the only item on
  its actions line.
- **3.5** `[agent]` Fixture (f): the candidate has exactly 8 lines, the long one is 256 chars, no
  line contains `\n`; the `[]` candidate has no toggle.
- **3.6** `[agent]` Fixture (d): DOM of each row is identical to `main`'s render of the same
  fixture (diff `outerHTML` after stripping Svelte hydration attributes).
- **3.7** `[agent]` Refresh-all with (g): System Activity shows the entry with the `applied:`
  segment; it truncates on one line with the full text in `title`.
- **3.8** `[agent]` Skins 2/3: `font-ui` is monospace there — confirm the lines and the toggle
  inherit it (no hardcoded `font-family` on the new markup).

## §4 Human — `[human]`

Open any video's page as the owner, click **Enrich**, pick the `acme` provider, and type
`Harbor Lights 2023`. The fake provider answers with the fixtures above; the agent will have left
the dev server on fixture (a) unless you ask for another.

- **4.1** `[human]` **Rows look the same as before until you ask.** With fixture (b) loaded, each
  row has three lines — name, the grey summary, and the gold `view source ↗` — plus one small grey
  word `details` with a dotted underline on the third line. Nothing else moved, nothing got taller.
- **4.2** `[human]` **Opening feels like the "+N more" on the Searched line.** Click `details` on
  the second row: two or three grey lines appear directly under that row, indented with a thin
  vertical rule, and the word changes to `hide details` in the same colour as the row's name. The
  row did **not** get selected — the dialog is still open, nothing was applied.
- **4.3** `[human]` **Same-name rows open on their own.** Type the query again with fixture (a):
  four `Harbor Lights` rows arrive already open with their lines showing; `Harbor Lights II` at the
  bottom is closed. You can read the `Record:` line on each and see they differ (28 tags · 3
  images vs 16 tags · 1 image). Clicking `hide details` on one closes only that one.
- **4.4** `[human]` **Keyboard-only.** Tab from the search box: focus lands on the highlighted row,
  then `view source ↗`, then `details`. Press Space on `details` — lines open, nothing applied.
  Press ↓ — the highlight moves to the next row and the lines you opened stay open. Tab back to
  the row and press Enter — the candidate applies as it always has.
- **4.5** `[human]` **Nothing floats or gets cut off.** With fixture (e) — 25 rows all open — scroll
  the list to the bottom. Every line is inside the list; no tooltip or popup appears anywhere; the
  very long line ends with `…` and hovering it shows the full text.
- **4.6** `[human]` **Touch.** On a phone-width window (or a touch device), tap `details` — it
  opens without selecting the row. Tapping the row itself still selects it. If you find `details`
  too small to hit reliably, say so — the handoff allows `py-1` on the button to enlarge it.
- **4.7** `[human]` **All three skins.** Switch skin (owner menu) and repeat 4.2 once per skin:
  the lines are readable against the highlighted row's background, the dotted underline is
  visible, and in the two monospace skins the `Key: value` lines line up like a table.
- **4.8** `[human]` **The audit line reads as a sentence.** After a refresh-all on the (g) video,
  open System Activity: the enrich row ends with `… · applied: Harbor Lights — Studio: … · Record:
  …`. It should be obvious *which* record was bound without opening the video.
