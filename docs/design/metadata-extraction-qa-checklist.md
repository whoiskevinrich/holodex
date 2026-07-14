# QA Checklist: Filename metadata extraction — Extraction tab, preview, revert (F48)

Work through this against a running app. Extraction is local file-tag parsing, no provider
network calls — use the **backend-amv** testbed ([[reference-holodex-preview-testbeds]]): file-
metadata-focused, small fixture files, fast rescan. Needs: at least one file whose name matches a
configured `filename_patterns` entry with (a) a People/Studio conflict between filename and tag,
(b) a field present in the filename only (tag empty), (c) a field present in the tag only, and (d)
a Person/Studio name close-but-not-exact to an existing entity (to exercise the Jaro-Winkler
suggestion without an exact match). A completed extraction batch is also needed for the Revert
checks — run "Extract all" once against the fixture set first.

Spec [`metadata-extraction.md`](../specs/metadata-extraction.md) · ADR
[`ADR-067`](../architecture/ADR-067-filename-extraction-confidence-and-rollback.md) · design handoff
[`metadata-extraction-handoff.md`](metadata-extraction-handoff.md).

Legend: **[smoke]** = quick programmatic check · **[agent]** = verified this session
(`preview_eval` / unit tests) · **[human]** = needs a human look.

---

## §1 Setup

1.1 **[agent]** Start `backend-amv` + the `web` dev server; enter the admin token
(`activity.effectiveOwner`); confirm the Extraction tab and its controls are **absent** as a
visitor (no token) — no DOM presence, not just hidden.

## §2 Smoke

2.1 **[smoke]** `go test ./...` passes, including table-driven confidence-scoring cases (exact
+entity-exists, exact+no-entity, fuzzy, garbled, conflict) for both the 3-component entity rubric
and the 2-component non-entity rubric.
2.2 **[smoke]** `GET /owner/extraction-queue` returns grouped-by-video rows with **zero** writes
made on load (no field is written just by opening the tab).
2.3 **[smoke]** Extraction/review/revert endpoints are `requireOwner`-gated (401 unauthenticated).
2.4 **[smoke]** A fuzzy Jaro-Winkler match scoring above a tier's `AutoApplyThreshold` still routes
to the review queue, not auto-apply (unit test asserts routing, not just the score — F48.3d).
2.5 **[smoke]** A field carrying an existing `manual:` source routes to review on re-extraction
regardless of score (F48.3e).
2.6 **[smoke]** `npm --prefix web run check` passes with the new `ExtractionQueueRow` component and
extraction-queue types.

## §3 Agent live QA (all 3 skins)

3.1 **[agent]** **Extraction tab** renders grouped-by-video rows (not by field type), each video
group showing its pending fields in People → Studio → Title → Release Date → other order; tab
entry styled like the existing Status/Duplicates/Enrichment tabs, no visual outlier.
3.2 **[agent]** **Row values**: a field present in the filename only shows `tag: — (empty)` in
`text-muted italic`, not blank space; symmetric for tag-only.
3.3 **[agent]** **Suggested-entity line**: a People/Studio field with a near-but-not-exact existing
entity shows a separate advisory line below the value row reading "suggested match — not applied:
{name}" — never as an inline chip that could read as already-applied (resolved design choice,
Option B in the handoff).
3.4 **[agent]** **No-suggestion case**: a field with no exact and no fuzzy match shows "no match —
will create new {Person/Studio}" instead of a blank suggestion line.
3.5 **[agent]** **Accept filename / Accept tag**: clicking either enqueues the write, the field row
disappears from its video group in place (no refetch), and the group re-sorts/shrinks.
3.6 **[agent]** **Edit… (entity field)**: opens the picker dialog pre-seeded on the suggestion (if
any) as the top candidate — **not** pre-selected/applied; owner must click to confirm.
3.7 **[agent]** **Dismiss**: durable — reopening the tab (or navigating away and back) does not
resurface the dismissed field; only re-running extraction for that video brings it back (F48.4d).
3.8 **[agent]** **Extract all**: kicks off a background batch visible in System Activity
(`kind=extraction`); the Extraction tab stays usable while it runs, queue grows as results land.
3.9 **[agent]** **Preview dialog (manual batch)**: after resolving several rows, "Review N changes"
appears; opening it shows an old→new diff per field (old value struck through, new value in
accent), not an editable input; unchecking a row excludes it from the write, same as
`WritebackFormDialog`'s existing behavior.
3.10 **[agent]** **Preview dialog (auto-apply context)**: a fresh auto-applied batch surfaces the
preview dialog by default; checking "skip preview next time" and confirming causes the **next**
auto-apply batch to commit without showing the dialog.
3.11 **[agent]** **Revert**: on a completed extraction batch's System Activity row, clicking
"Revert" shows a brief busy state, then an inline "Reverted" status — every field in that batch
is restored to its pre-write value (byte-for-byte on the affected tag).
3.12 **[agent]** **Revert-of-a-revert**: the revert's own activity row also carries a Revert
button (no special-cased UI) — clicking it re-applies the original (post-extraction) values.
3.13 **[agent]** **Warn vs neutral separation** (F43/F47 regression risk, re-verified here):
"Conflict"/"no suggestion"/confidence tier labels read `text-muted`/`text-ink`, never `text-warn`;
only an actual resolve/write/revert failure shows `text-warn`. Check this holds on **Brutalist**
(bright lime accent vs. hot red-orange warn), where the two are most likely to visually collide.
3.14 **[agent]** **Merge → writeback propagation** (F48.8): merging two People with N affected
videos produces N writeback jobs in System Activity, each individually revertible; no second
confirm dialog appears beyond the merge's own informed-confirm.

## §4 Human

4.1 **[human]** Open the Extraction tab in each skin. It should feel like Duplicates/Enrichment's
sibling — same density, same "tidy worklist" feeling — not a bespoke new screen.
4.2 **[human]** Work a video group end-to-end: accept a filename value, accept a tag value, edit
one manually, dismiss the last — confirm the group disappears once every field clears, with no
jarring layout jump.
4.3 **[human]** Trigger the preview dialog and read the diff line at a glance — the struck-through
old value and the accent new value should be immediately distinguishable without close reading, in
all three skins (especially Broadcast's cyan-on-navy and Brutalist's lime-on-black).
4.4 **[human]** Click Revert on a real batch, then open the affected file's detail page and confirm
the resolved fields actually show the pre-extraction values again (not just the activity-row
status text).
4.5 **[human]** With a screen reader, tab through the picker dialog opened from "Edit…" — roving
tabindex should behave identically to the existing `EnrichPicker` (only one tab stop in the
candidate list, arrow keys move it).

> Verify with `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'`
> returning empty for the new/changed markup.
