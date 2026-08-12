# Design Handoff: Video composite-key collision verdict (HOLODEX-270)

Spec: [`docs/specs/video-composite-key-collision.md`](../specs/video-composite-key-collision.md) · Epic: HOLODEX-267

## Overview

One new component, `CollisionOfferCard`, plugs into `NameEditControl`'s existing `verdict`
snippet slot on the Video Title mount — the same slot `MergeOfferCard` already fills for
Person/Studio/Tag renames. It fires when a Title edit's proposed composite key {title, people,
date, studio} exactly matches another active video. Unlike `MergeOfferCard`'s merge-in/keep-
separate choice, there is no merge verb here (two video files can't be folded together): the
choice is **"View existing video"** vs. **"Save anyway, keep both"** — two equally-weighted,
non-destructive actions.

## A required precondition: generalize `NameEditControl`'s conflict type

`NameEditControl.svelte`'s `verdict` prop is currently typed
`Snippet<[EntityRef, () => void]>` and `onCommit` returns `Promise<{ok:true} | {conflict:
EntityRef}>` (`web/src/lib/components/entity/NameEditControl.svelte:25,34`). `EntityRef` is `{id,
name, video_count?}` (`web/src/lib/types.ts:18-22`) — it has no room for a colliding video's
people/date/studio. Rather than shoehorning video fields into `EntityRef` (which would pollute a
type shared by the Person/Studio/Tag merge-picker and duplicate-review surfaces), make
`NameEditControl` generic over the conflict payload:

```ts
<script generics="TConflict = EntityRef" lang="ts">
...
onCommit: (value: string) => Promise<{ ok: true } | { conflict: TConflict }>;
verdict?: Snippet<[TConflict, () => void]>;
```

Person/Studio/Tag call sites need **zero changes** — TS infers `TConflict = EntityRef` from their
existing `onCommit` implementations (all three already resolve `{conflict: EntityRef}`), and the
default type parameter covers it if ever spelled out explicitly. Only the Video Title mount
supplies an explicit `NameEditControl<VideoCollisionRef>` (or lets inference do it from its own
`onCommit`'s return type — same effect, no explicit generic needed at the call site either, since
Svelte infers `generics` from prop values same as a normal generic function call).

## New type: `VideoCollisionRef`

Add to `web/src/lib/types.ts`, near `EntityRef`:

```ts
// VideoCollisionRef is the minimal shape the composite-key collision 409 body returns for the
// OTHER (colliding) video — enough for CollisionOfferCard to render without a follow-up fetch
// (HOLODEX-270). Deliberately separate from EntityRef: a video isn't on the identity spine and
// carries different identifying fields (people/date/studio, not a single name).
export interface VideoCollisionRef {
	id: number;
	title: string;
	people: string[]; // display names, already resolved server-side — no extra lookup needed
	recorded_at: string | null; // ISO date, same format as Video.recorded_at
	studio: string | null; // display name; null when the video has no studio linked
}
```

This is the exact shape the backend's 409 response body should carry (see the companion backend
task) — resolved display strings, not raw ids, so the card never needs a second round trip.

## `CollisionOfferCard.svelte`

New file: `web/src/lib/components/entity/CollisionOfferCard.svelte` — co-located with
`MergeOfferCard`/`NameEditControl` since it's the same verdict-slot mechanism, just for Video
instead of Person/Studio/Tag (per the component-folder rule: group by product mechanism, not
strictly by "which of the three entity types" when the mechanism itself is shared infrastructure).

### Props

```ts
let {
	video, // VideoCollisionRef — the colliding video
	busy = false,
	error = '',
	onviewexisting,
	onsaveanyway
}: {
	video: VideoCollisionRef;
	busy?: boolean;
	error?: string;
	onviewexisting: () => void; // navigates to /media/{video.id}; caller owns the goto()
	onsaveanyway: () => void; // caller re-submits the pending edit with an override flag
} = $props();
```

Presentational only, exactly like `MergeOfferCard` — the caller (Video Title's mount code) owns
`busy`/`error` state and the actual navigation/re-submit calls.

### Layout

Same card shell as `MergeOfferCard` (`space-y-2 rounded-theme border border-rule bg-surface-2
p-3`, `aria-live="polite"`). Body differs:

```
┌─────────────────────────────────────────────────────────────┐
│ "{title}" already matches another video:                     │
│   {video.title}                                              │
│   {people.join(', ') || '—'} · {formatYear(recorded_at)} ·   │
│   {studio || '—'}                                             │
│                                                                │
│ [View existing video]  [Save anyway, keep both]               │
│ {error text, if any}                                          │
└─────────────────────────────────────────────────────────────┘
```

- Line 1 (`text-sm text-ink`): `"{name}" already matches another video:` — mirrors
  `MergeOfferCard`'s opening sentence structure but names what collided (the whole composite key,
  not just the name) rather than asking a question, since there's no merge choice to frame as a
  question.
- Line 2 (`text-sm font-semibold text-ink`): the colliding video's title, as a plain text label —
  **not a link**; navigation happens only via the explicit "View existing video" button, so a
  stray click on the title text doesn't accidentally leave the page mid-edit (consistent with
  `MergeOfferCard`'s conflict name also being plain text, not a link).
  - Reuse `formatYear` from `$lib/format.ts` for the date (already used elsewhere for Video's own
    recorded-year display) rather than raw ISO text.
  - Join people with `', '`; render an em dash `—` when the array is empty or studio is null —
    matches the existing "Missing" -> em-dash convention used elsewhere in this codebase for
    absent optional fields (see `EntityVideoMeta.svelte`'s own separator/absence handling for
    precedent, though it doesn't need copying verbatim).

### States and Interactions

| Element | State | Behavior |
|---|---|---|
| Card | shown | `aria-live="polite"` announces it to screen readers the instant it mounts (same as `MergeOfferCard`) — the owner just submitted a rename and didn't navigate, so the verdict appearing needs to be announced, not just visually present. |
| "View existing video" button | default | `.btn-accent px-3 py-1.5 text-sm` — same visual weight as `MergeOfferCard`'s primary action, but this is a **navigation**, not a destructive/affirmative commit; still gets the accent treatment because it's the more common/expected choice (owner usually wants to check the existing video first) — see Accessibility note on button labeling below for why this doesn't read as "confirm merge." |
| "View existing video" button | busy | disabled, same `disabled={busy}` pattern as `MergeOfferCard`; no label swap needed since navigation is instant once clicked (no network round trip on this action itself — it's a client-side `goto()`). |
| "Save anyway, keep both" button | default | `.btn-ghost px-3 py-1.5 text-sm` |
| "Save anyway, keep both" button | busy | disabled; caller may swap label to "Saving…" the same way `NameEditControl`'s own Save button does (`{busy ? 'Saving…' : 'Save'}`) — optional, not required, since the card doesn't own the busy state itself. |
| Both buttons | error | Both re-enabled (busy resets to false on failure, same as `NameEditControl.commit`'s `finally` block); error text renders below in `text-sm text-warn`, identical placement/token to `MergeOfferCard`. |

No hover/loading spinner beyond the existing disabled-button pattern — this codebase doesn't use
spinners on these small inline actions elsewhere (`MergeOfferCard`, `NameEditControl` both just
disable + relabel).

### Responsive

No breakpoint-specific behavior — `flex flex-wrap items-center gap-2` on the button row (same as
`MergeOfferCard`) lets buttons wrap under narrow viewports without a separate mobile layout.

### Edge Cases

- **No people, no studio**: renders `— · {year} · —` — still useful for disambiguation via title
  + date alone; never hide the row for missing optional fields (consistent with the rest of the
  app's "Missing" convention rather than collapsing whitespace).
- **Very long title**: no truncation added — the card already wraps naturally inside its
  `rounded-theme border` container at the page's existing content width; `MergeOfferCard` doesn't
  truncate either, so this doesn't diverge from precedent.
- **`onviewexisting` navigation and unsaved edit-form state**: by the time this card renders,
  `NameEditControl` has already called `closeEdit()` (see its `commit()` — on `'conflict' in res`
  it does `conflict = res.conflict; closeEdit(); return;`), so there's no separate "discard your
  edit?" prompt needed — the inline form is already gone, only the verdict card remains.

### Accessibility

- `aria-live="polite"` on the card root (copied from `MergeOfferCard`).
- Both buttons are real `<button>` elements with visible text labels — no icon-only affordances,
  so no extra `aria-label` needed beyond what the button text already provides.
- Focus: after the card mounts, focus is **not** auto-moved into it (matches `MergeOfferCard`,
  which also doesn't steal focus) — the `aria-live` announcement is sufficient, and auto-focus
  would yank a screen-reader/keyboard user away from the flow unexpectedly right after a form
  submit.
- On resolving via "Save anyway, keep both" that succeeds, the caller should call
  `NameEditControl`'s exposed conflict-resolution callback (the `() => void` second snippet
  argument, same as `MergeOfferCard`'s `onkeepseparate`/`onmerge` pattern calling back into
  `resolveConflict`) so the card unmounts and focus returns to the pencil, exactly like the
  existing Person/Studio/Tag flow already does.
