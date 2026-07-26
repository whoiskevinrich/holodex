# QA Checklist: Writeback dialog selection + undecided grouping (HOLODEX-213)

Work through this against a running app. Enrichment testbed: start `provider-tmdb` (:9100), then
`backend-films` (:7800), then the `web` dev server (:5173) — open **http://localhost:5173/**. Needs:
one enriched video whose provider values win by mapping precedence and which has **no** standing
field decisions (e.g. Dune 1984, `/media/6` on the films testbed), signed in as owner.

Spec [`field-source-of-truth.md`](../specs/field-source-of-truth.md) §Writeback · design handoff
[`writeback-selection-handoff.md`](writeback-selection-handoff.md).

Legend: **[smoke]** = quick programmatic check · **[agent]** = verified this session
(`javascript_tool` / unit tests) · **[human]** = needs a human look.

---

## 1. Setup / smoke

1.1 **[smoke]** `npm --prefix web run test` passes, including the `needsWriteback` cases in
`f36.test.ts`: an out-of-sync decided field; a provider winning by mapping precedence; `poster_url`
with no file candidate; a merge field marked out of sync; and the field-for-field agreement
assertion against `outOfSyncCount`.
1.2 **[smoke]** `npm --prefix web run check` passes with 0 errors (4 pre-existing a11y warnings on
the two dialog backdrops).

## 2. Agent-verified (this session)

2.1 **[agent]** On a video with no decisions, the header button reads `Write decisions to file` with
no count, and the dialog opens with **0** checkboxes checked and a disabled `Write 0 fields to file`.
2.2 **[agent]** `poster_url` is not checked on open — confirms no image download or cover-art embed
is armed by default.
2.3 **[agent]** With `title` and `tagline` pinned to `provider:tmdb`, the header reads `· 2 out of
sync`, exactly `title` and `tagline` are checked and visible on open, and the footer reads `Write 2
fields to file`.
2.4 **[agent]** The disclosure reads `12 provider values you haven't decided on` in that state, and
`14` when nothing is decided — decided + undecided always equals the full row count.
2.5 **[agent]** Clicking the disclosure flips `aria-expanded` to `true` and reveals every undecided
row; clicking again re-collapses it.
2.6 **[agent]** `Select all` expands the group, checks every undecided row, and the footer follows
(`Write 14 fields to file`). Collapsing the group afterwards **keeps** the selection.
2.7 **[agent]** On open, focus lands inside the dialog in both states: the first decided row's
checkbox when there are decisions, the dialog element itself when there are none (the collapsed
group's hidden inputs must not swallow it).
2.8 **[agent]** No element inside the dialog renders below `opacity: 1`, and the row label,
disclosure label, and `Select all` all clear 4.5:1 against the dialog surface in all three skins
(measured 4.67–16.76).
2.9 **[agent]** No console errors on open, expand, select-all, or close.

## 3. Human look

3.1 **[human]** Open a video page as the owner (any item under **Browse**), find the **Write
decisions to file** button in the Metadata section header, and click it. If you have made no source
choices on this video, the dialog should say so in a plain sentence rather than showing an empty
list — and the line underneath should offer you the provider values it *could* write, with a count.
Nothing should look broken or blank.
3.2 **[human]** Click that count line. The provider values should unfold below it, all unchecked and
all perfectly readable — no washed-out or greyed-looking rows. Click it again to fold them away.
3.3 **[human]** Click **Select all**. Every row should tick, and the button at the bottom right
should update to match the number of ticked rows. Untick one row and confirm the bottom-right number
drops by one.
3.4 **[human]** Now make a real choice on the page behind the dialog: pick a provider value for one
field using its value chips, then reopen the dialog. That field should be at the top, already
ticked, and the count line below should have gone down by one.
3.5 **[human]** Repeat 3.1–3.2 in each of the three skins (header picker: Cinémathèque, Broadcast,
Brutalist). In every skin the fold/unfold line, the **Select all** button, and the field labels
should be comfortably legible against the dialog background, and the **Select all** button should
read as the accent colour of that skin.
3.6 **[human]** With the dialog open, press `Tab` repeatedly. Focus should cycle within the dialog
and never escape to the page behind it — including when the provider values are folded away. Press
`Escape` to close; focus should return to the button that opened it.
