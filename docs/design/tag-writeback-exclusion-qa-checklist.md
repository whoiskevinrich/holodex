# QA Checklist: Tag writeback exclusion frontend (HOLODEX-239)

Work through this against a running app. Testbed: `backend-amv` (file-metadata testbed, small
files — writeback actually touches disk here) — start it, then the `web` dev server (:5173),
signed in as owner. Needs at least one tag attached to 2+ videos.

Spec [`tag-writeback-exclusion.md`](../specs/tag-writeback-exclusion.md) · design handoff
[`tag-writeback-exclusion-handoff.md`](tag-writeback-exclusion-handoff.md).

Legend: **[smoke]** = quick programmatic check · **[agent]** = verified this session
(`javascript_tool` / unit tests) · **[human]** = needs a human look.

---

## 1. Setup / smoke

1.1 **[smoke]** `npm --prefix web run check` passes with 0 new errors.
1.2 **[smoke]** `npm --prefix web run test` passes, including new `writebackJob.test.ts` cases for
`waitForWritebackBatch` (resolves on `pending+running===0`, resolves — doesn't throw — with
`failed>0`, resolves with last-known counts on timeout, rethrows a duck-typed `ApiError`
immediately).

## 2. Agent-verified (this session)

2.1 **[agent]** Non-owner (or owner with Admin mode off) sees no Details card on `tags/{id}` and
no new buttons in Manage mode on `/tags`.
2.2 **[agent]** Owner on `tags/{id}` sees the Details card with correct initial toggle state
(`aria-pressed` matches `tag.writeback_enabled`) and correct label text for both states.
2.3 **[agent]** Clicking the toggle flips it immediately (no dialog, no confirm) via `PATCH
/tags/{id}/writeback`, and does **not** enqueue anything (no batch dialog opens, no job in
Activity).
2.4 **[agent]** On a tag with `video_count === 0`, "Sync writeback now" is `disabled` with an
explanatory `title`.
2.5 **[agent]** On a tag with videos, clicking "Sync writeback now" opens `WritebackBatchDialog` in
the `confirm` phase showing the tag name and exact video count; "Start sync" fires `POST
/tags/{id}/writeback/sync`, transitions to `progress`, and the dialog reaches a `done`/`partial`
state once polling settles (verified against a stubbed `batchStatus` returning decreasing
pending/running counts).
2.6 **[agent]** A batch with `failed > 0` in its final status renders the `partial` copy, not an
error state — the dialog does not throw or show a red error banner for non-zero `failed`.
2.7 **[agent]** `/tags` Manage mode: the three new buttons are absent at 0–1 selected, present at
2+; "Turn off"/"Turn on" are both always visible together (never a single combined toggle).
2.8 **[agent]** Bulk "Turn off writeback" on a mixed-state selection (one tag already off, one on)
sets both to off in one call (`PATCH /tags/writeback`, 204) with no per-tag branching.
2.9 **[agent]** Bulk "Sync writeback now" opens the batch dialog with the selected tags' names
listed (not a possibly-wrong summed count), and `trigger` calls `syncTagsWriteback` with the full
selected-id list.
2.10 **[agent]** No element inside the new card or dialog renders with `disabled:opacity-*` on a
`text-muted` label (`rg 'text-muted[^"]*disabled:opacity' web/src --glob '*.svelte'` stays empty
after this change) and no hardcoded palette/radius classes were introduced (`rg
'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` stays empty).
2.11 **[agent]** Escape closes the batch dialog during `confirm`/`progress`/`done` (not during
`starting`); focus returns to the button that opened it; Tab cycles only within the dialog.
2.12 **[agent]** No console errors across toggle, sync-confirm, sync-progress, and bulk actions.

## 3. Human look

3.1 **[human]** Open a tag with several videos (`/tags` → click a tag) as the owner. Under the
video grid you should see a new **Details** card with two rows: a writeback toggle and a "Sync
writeback now" button. Nothing should look like a placeholder or an afterthought — it should read
like it's always been part of the page.
3.2 **[human]** Click the toggle. It should flip instantly — no dialog, no loading spinner beyond
an instant — and the description line underneath should update to say whether this tag now writes
to files or not.
3.3 **[human]** Click "Sync writeback now". A small dialog should appear naming the tag and stating
how many files it's about to touch, with a "Start sync" button. Click it — the dialog should show a
progress bar filling up, then settle on a plain-English completion line ("N files updated."). Click
Close.
3.4 **[human]** Go to `/tags`, click "Manage tags", and select two or more tags with a mix of
current writeback states (use the Details page from 3.1–3.2 to set one off, one on, beforehand).
You should see two clearly separate buttons — one to turn writeback off, one to turn it on — plus a
"Sync writeback now" button, all appearing only once you've selected 2 or more. Click "Turn off
writeback for selected"; nothing should visually break, and if you reopen one of those tags' Detail
pages its toggle should now read as excluded.
3.5 **[human]** With 2+ tags still selected, click "Sync writeback now for selected". The dialog
should describe the sync as covering the tags you picked (by name, not a single number this time)
and behave the same way as 3.3 once started.
3.6 **[human]** Repeat 3.1–3.3 in each of the three skins (header picker: Cinémathèque, Broadcast,
Brutalist). The Details card, the toggle glyph, the Sync button, and the batch dialog's progress
bar should all read clearly against the background in every skin — no washed-out text, no
invisible progress fill.
3.7 **[human]** With the batch dialog open mid-progress, press `Tab` repeatedly — focus should stay
inside the dialog. Press `Escape` — the dialog should close and focus should return to the button
you clicked to open it.
