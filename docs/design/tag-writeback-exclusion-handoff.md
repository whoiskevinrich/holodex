# Design Handoff: Tag writeback exclusion — Details card + bulk actions (HOLODEX-239)

**Spec**: [tag-writeback-exclusion.md](../specs/tag-writeback-exclusion.md) ·
**ADR**: [ADR-077](../architecture/ADR-077-tag-writeback-exclusion.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Prior art (same dialog, different feature)**:
[writeback-selection-handoff.md](writeback-selection-handoff.md) — the design-system-fit audit
format and "no dimming" rule this handoff follows.
**Surfaces**: `tags/[id]/+page.svelte` (new `detail` snippet), `tags/+page.svelte` (Manage-mode
bar), new `WritebackBatchDialog.svelte`.

---

## Overview

Two additions, both owner-gated (`activity.effectiveOwner`):

1. **`tags/{id}` Details card** — the tag detail page has never had a `detail` snippet (only
   `people/[id]` uses `EntityVideos`'s extension point today). This is the first one. It hosts a
   writeback-inclusion toggle and a manual sync trigger.
2. **`/tags` Manage-mode bar** — extends the existing merge-only selection bar with three actions
   that apply across every selected tag once 2+ are selected.

Both actions that enqueue a file write (single-tag sync, bulk sync) open the same new
**`WritebackBatchDialog`** — an N-video progress/completion shell, sibling to
`WritebackFormDialog` rather than a prop-mode of it (per spec: "extends the same visual/
interaction shell... rather than reusing it unmodified" — there is no per-video field list here,
only an aggregate `pending/running/done/failed` count, a structurally different body).

### Design-system fit (the `/design-system` check)

**No new tokens. One new component, composed entirely from existing idioms:**

- **Card shell** — `rounded-theme border border-rule bg-surface p-4` + `text-xs uppercase
  tracking-wide text-muted` heading, verbatim from `people/[id]`'s `detail` snippet — the only
  other consumer of this extension point.
- **Inclusion toggle** — `CurationChip`'s "don't write" document glyph (`aria-pressed` icon
  button), lifted from value-scope to tag-entity-scope. Same SVG path, same interaction idiom
  (pressed = excluded), new call site.
- **Sync trigger / bulk buttons** — `.btn-accent` (the affirmative action) and `.btn-ghost`
  (a resolve), exactly as `writeback-selection-handoff.md` already established for this dialog
  family. Disabled state is what `.btn-accent`/`.btn-ghost` already do (drop border, demote to
  `text-muted`) — never `disabled:opacity-*` on a `text-muted` label.
- **Batch dialog chrome** — backdrop, `role="dialog"`, focus trap, `Escape`-to-close: copied from
  `WritebackFormDialog`'s existing pattern (the same `offsetParent`-filtered focus logic, since a
  collapsed/hidden state can again be present).
- **Progress bar** — a plain `<div>` with `bg-accent` fill on a `bg-surface-2` track, width driven
  by `(done + failed) / enqueued`. No new primitive.

Audit output: **one new component (a dialog shell + a progress bar), zero new tokens, zero new
CSS.**

---

## The polling extension (one new function, same shape as the existing one)

`web/src/lib/writebackJob.ts` already exports `waitForWritebackJob` (single job, throws on
`'failed'`). This adds `waitForWritebackBatch`, sharing the same backoff constants
(`POLL_START_MS`/`POLL_MAX_MS`/`POLL_GROWTH`/`JOB_POLL_TIMEOUT_MS`) and the same
`ApiError`-duck-typed immediate-rethrow-on-session-expiry behavior, but differing where the spec
requires it to differ:

- **Resolves with the final `{pending, running, done, failed}` counts instead of `void`** — the
  caller needs the summary to render, not just a completion signal.
- **Never throws on `failed > 0`.** Spec P0: "Enqueue failures are logged, non-fatal — visible in
  the dialog's result, but don't leave the flag or other jobs in an inconsistent state." A batch
  is done once `pending + running === 0`, regardless of how many of those landed in `failed` — the
  dialog reads the returned counts and shows a warning line, not an error state, when `failed >
  0`.
- **On timeout, resolves with the last-known counts instead of hanging forever** — a 50+-video
  batch (the spec's own success-metric floor) may still be running past `JOB_POLL_TIMEOUT_MS`; the
  dialog's completion copy for that case says it's fine to close and check back later, which is as
  much of the P1 "still going after I left the page" need as this change covers (see Non-goals).

---

## Component: `WritebackBatchDialog.svelte`

New file, `web/src/lib/components/writeback/`.

### Props

```ts
{
  scopeLabel: string;               // "Dog" (single tag) or "Comedy, Drama, Noir" (bulk)
  videoCountHint: number | null;    // exact tag.video_count for single-tag; null for bulk (see below)
  trigger: () => Promise<{ batch_id: string; enqueued: number }>;
  batchStatus: (batchId: string) => Promise<BatchStatus>;
  onclose: () => void;
  onapplied: () => void;            // fired once the batch reaches pending+running===0 (or enqueued===0)
}
```

**Why `videoCountHint` is `null` for bulk, not a summed estimate.** A video attached to two
selected tags is enqueued once (`VideoIDsForTags`'s dedup, ADR-077 D2) — summing each tag's
`video_count` would overcount and the dialog would state a number the batch then contradicts.
Bulk's confirm step instead lists the tag names it's about to sync; the exact figure appears
after `trigger()` resolves (`enqueued`), which is always correct because it's the server's own
count. Single-tag can state the number up front because `tag.video_count` **is** the exact set
`VideoIDsForTag` will enqueue against.

### States

| Phase | Trigger | UI |
|---|---|---|
| `confirm` | dialog opens | Scope line (`scopeLabel` + count-if-known), Cancel + "Start sync" |
| `starting` | "Start sync" clicked | Buttons disabled, no spinner needed (this call is fast — it's the enqueue, not the writes) |
| `progress` | `trigger()` resolved with `enqueued > 0` | Progress bar + "`{done+failed}` of `{enqueued}`", polling `batchStatus` |
| `done` | `pending+running` reaches 0, `failed === 0` | "`{done}` file{s} updated.", Close |
| `partial` | same, `failed > 0` | "`{done}` updated, `{failed}` failed — check Activity for details.", Close (not an error state — spec P0: non-fatal) |
| `zero` | `trigger()` resolved with `enqueued === 0` | "Nothing to sync — no videos currently carry a genre value from this tag." Close |
| `error` | `trigger()` itself rejects (network/5xx) | The thrown message, Retry + Cancel |

No `busy`-spinner row-by-row like `WritebackFormDialog` — the batch endpoint has no per-video
identity to spin against, only aggregate counts, so the affordance is a single progress bar, not N
row icons.

### Layout

```
┌ Sync writeback ───────────────────────────────────────┐
│ Dog                                                    │
│ This will write to 214 files.                          │
│                                          Cancel  [Start sync] │
└─────────────────────────────────────────────────────────┘
                    ↓ (Start sync)
┌ Sync writeback ───────────────────────────────────────┐
│ Dog                                                    │
│ ▓▓▓▓▓▓▓▓▓▓▓▓░░░░░░░░░░░░░░░░  87 of 214                │
└─────────────────────────────────────────────────────────┘
                    ↓ (pending+running → 0)
┌ Sync writeback ───────────────────────────────────────┐
│ Dog                                                    │
│ 214 files updated.                                      │
│                                                  [Close] │
└─────────────────────────────────────────────────────────┘
```

Bulk `confirm` phase (no exact count available yet):

```
┌ Sync writeback ───────────────────────────────────────┐
│ Comedy, Drama, Noir                                     │
│ This will sync every video currently carrying any of     │
│ these tags.                                              │
│                                          Cancel  [Start sync] │
└─────────────────────────────────────────────────────────┘
```

### Behaviour notes

- **No dimming.** Disabled buttons use `.btn-ghost`/`.btn-accent`'s own disabled treatment
  (border drop, `text-muted` demote) — never `disabled:opacity-*` layered on a `text-muted` label,
  per the standing theming rule and `writeback-selection-handoff.md`'s precedent.
- **Focus trap / `Escape`** — identical mechanism to `WritebackFormDialog`
  (`offsetParent`-filtered focusable-element query, focus returned to the trigger on close).
  `Escape` is disabled only during `starting` (an in-flight enqueue call, mirroring `busy` there);
  it's allowed during `progress` — closing the dialog does not cancel the batch, it just stops
  polling (`cancelled: () => unmounted`), exactly like the single-job dialog already does.
- **`aria-live="polite"`** on the progress/completion line, same as the single-job dialog's status
  line, so a screen reader announces state changes without the caller needing to refocus.
- **The `zero` phase is not an error.** Spec P0: "A tag attached to zero videos: the trigger is
  disabled/no-op." The Details-card trigger button is already `disabled` in that case (see below),
  so `zero` is reachable in practice only via the bulk path (a selection whose union happens to be
  empty) — handled the same way rather than as a special case.

---

## Details card (`tags/[id]/+page.svelte`)

New `{#snippet detail()}` passed to `<EntityVideos>`, rendered only for owners
(`activity.effectiveOwner`) — non-owners see nothing new, matching the existing
`people/[id]` gating this card's shell is copied from.

```svelte
<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
  <h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
  <dl class="space-y-3 text-sm">
    <div class="flex items-center justify-between gap-3">
      <div>
        <dt class="text-ink">File writeback</dt>
        <dd class="text-xs text-muted">
          {included: "Included in this tag's videos' Genre field on write."
           : "Excluded from Genre writeback — stays searchable in Holodex."}
        </dd>
      </div>
      <!-- CurationChip's "don't write" glyph, tag-entity-scoped -->
      <button aria-pressed={!writeback_enabled} title="…" class="rounded-theme border p-1.5 …">
        <svg …the same document-glyph path…/>
      </button>
    </div>
    <div class="flex items-center justify-between gap-3 border-t border-rule pt-3">
      <div>
        <dt class="text-ink">Sync to files</dt>
        <dd class="text-xs text-muted">Push this tag's current decision out to already-written files.</dd>
      </div>
      <button class="btn-accent px-3 py-1.5 text-sm" disabled={video_count === 0}>Sync writeback now</button>
    </div>
  </dl>
</section>
```

- **Two `dt`/`dd` rows in one `<dl>`, not a single hard-coded control** — per spec P0 ("Built
  generically enough to hold a second row later... not hard-coded to only ever contain this
  toggle") — a future tag-categories row slots in as a third `div` without restructuring.
- **Toggle button posts immediately** (`PATCH /tags/{id}/writeback`) — no confirm step, matching
  spec P0 ("changing the flag alone never enqueues a write" — it's the lowest-stakes control on
  the card, reversible with one more click). The card's local `tag` state updates from the PATCH
  response (`{tag}`) so the label/glyph flips without a full page reload.
- **Sync button is `disabled` (not hint-driven) when `video_count === 0`** — spec P0 says the
  trigger itself is "disabled/no-op," unlike the `/tags` Merge button's hint-on-click pattern
  (which the spec doesn't ask this control to follow).
- Clicking Sync opens `WritebackBatchDialog` with `scopeLabel={tag.name}`,
  `videoCountHint={tag.video_count}`, `trigger={() => api.syncTagWriteback(tag.id)}`.

---

## Bulk bar (`tags/+page.svelte`)

Three new buttons appear **inside the existing Manage-mode bar, only once `selectedIds.length >=
2`** (the spec's own threshold — distinct from the Merge button beside them, which stays visible
below 2 and answers with a hint instead, an existing pattern this change doesn't touch):

```svelte
{#if selectedIds.length >= 2}
  <button class="btn-ghost px-3 py-1 text-sm" onclick={() => bulkSetWriteback(false)}>
    Turn off writeback
  </button>
  <button class="btn-ghost px-3 py-1 text-sm" onclick={() => bulkSetWriteback(true)}>
    Turn on writeback
  </button>
  <button class="btn-accent px-3 py-1 text-sm" onclick={() => (bulkSyncOpen = true)}>
    Sync writeback now
  </button>
{/if}
```

- **On/off stay two separate, always-both-visible buttons** — spec P0: "a selection can span tags
  already in different states, so on/off stay explicit and separate." Neither is a single toggle.
- **Bulk toggle applies regardless of each tag's individual prior state** (`PATCH /tags/writeback
  {tag_ids, enabled}`, 204) and never enqueues — same one-click-no-confirm posture as the
  single-tag toggle, for the same reason.
- **Bulk sync** opens `WritebackBatchDialog` with `scopeLabel` = the selected tags' names joined
  (truncated to "`{n} tags`" past a handful, to keep the confirm line readable), `videoCountHint =
  null`, `trigger={() => api.syncTagsWriteback(selectedIds)}`.
- On any bulk action's success, `reload()` — the existing post-mutation pattern every other
  Manage-mode action in this file already follows.

---

## Non-goals (explicitly out of this change, matching the spec's own P1/P2 split)

- **`.activity-dot` ambient "still syncing" indicator (spec P1).** `waitForWritebackBatch`'s
  timeout path already tells the owner it's safe to close and check back later — building the
  cross-page ambient dot is a separate, larger change (new global polling state) the spec itself
  marks non-blocking. Flagged, not built.
- **Exact pre-trigger video count for bulk sync.** Would need a new "preview union" endpoint;
  the spec's P1 acceptance criterion ("states the number of videos") is satisfied for the
  single-tag path (`tag.video_count` is already exact) — bulk gets the honest tag-name list
  instead of a knowingly-wrong number.

---

## Measured contrast (all three skins, dialog + card surfaces)

Reuses `WritebackFormDialog`'s already-measured surface/border tokens (`bg-surface`, `border-rule`,
`text-muted`, `.btn-accent`) verbatim — see `writeback-selection-handoff.md`'s table
(4.67–16.76:1 across all three skins for these same tokens). No new color combination is
introduced by this change; the QA checklist re-verifies rather than re-measures.
