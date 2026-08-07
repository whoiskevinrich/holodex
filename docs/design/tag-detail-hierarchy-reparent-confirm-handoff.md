# Design Handoff: Reparent-confirm flow for the Children control (HOLODEX-259)

**Spec**: [tag-detail-hierarchy-and-categories.md](../specs/tag-detail-hierarchy-and-categories.md)
**ADR**: none — spec confirms no new ADR (extends ADR-075/ADR-078).
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) +
[theming.md](theming.md) — **tokens only, QA all three skins.**
**Prior art**: `ConfirmDialog.svelte` (`web/src/lib/components/shared/`) — the existing
warn-styled modal idiom (focus trap, Esc/backdrop cancel, focus returned to trigger), already
used for category delete on `/tags`. The `nearMissCard` snippet on `tags/+page.svelte` — the
lighter-weight inline idiom this flow deliberately does *not* use (see rationale below).
**Depends on**: `SetTagParent` (`internal/repo/tag_hierarchy.go`, `ErrTagCycle` guard), the
tag-detail read (`GetTag`) gaining `Ancestors` (already shipped) and `Children` (this spec's own
P0 backend work) — this flow reuses both, no new endpoint.
**Surfaces**: `tags/[id]/+page.svelte` only (the new Children control). Scope of this handoff is
narrowly the confirm interaction — the rest of the Children/Parent/Categories controls follow
existing patterns already covered by the spec's own component references.

---

## Overview

Of the three new controls the spec adds to `tags/{id}` (parent, children, categories), the
Children control's "+ Add child" input has one branch with no existing component to port: when
the resolved name matches an existing tag that already has its own parent or its own children,
attaching it here would silently relocate an established branch, not just graft a fresh leaf.
This handoff covers that one branch — what interrupts the owner, what it says, and why.

### Design-system-fit audit

**One new interaction, zero new components. It's an existing modal idiom (`ConfirmDialog`)
with copy specific to this flow.**

- **The surface** — `ConfirmDialog.svelte`, `variant="destructive"` (warn-colored confirm
  button, `bg-warn`/`text-warn-ink`). Already the app's one "stop and make me confirm" idiom;
  already used for a structural, non-data-loss-but-consequential action (category delete
  unassigns tags, doesn't delete them — the same risk profile as this reparent).
- **Not the near-miss card** — `nearMissCard` is deliberately non-blocking: it appears *after*
  a save already committed, dismissible by simply not clicking either button, and reversible in
  one more click either way. A reparent that moves an existing branch is different in kind: it
  needs to interrupt *before* committing, because "just merge it back" isn't a clean undo once a
  subtree has moved. Matches the flow's own risk callout — this warrants the stronger interrupt
  the near-miss pattern intentionally avoids.
- **Data needed to write the copy** — already available with no new endpoint: `GetTag`'s
  existing `Ancestors` field (last entry = the candidate's current parent name) and the same
  spec's new `Children` field (count = subtree size), fetched via `api.getTag(candidateId)`
  immediately after resolve-or-create. A brand-new tag or a childless root naturally has neither,
  so the same fetch-and-inspect drives all three outcomes (create, attach, confirm) uniformly —
  no separate "was this newly created" flag needed from the resolve call.

---

## 1. Decision logic (when the dialog appears)

```
owner submits "+ Add child" with name
  → api.resolveOrCreateTag(name)  [existing call, unchanged]
  → api.getTag(candidateId)        [existing call, reused — not a new endpoint]
      candidate.ancestors: string[] | undefined
      candidate.children:  {id, name}[] | undefined   (new field, this spec's own P0 work)

  if candidate.ancestors is empty/absent AND candidate.children is empty/absent:
      → attach immediately: SetTagParent(candidate.id, currentTag.id), no dialog
  else:
      → open ConfirmDialog (see §2)
```

- **Brand-new tag** (just created by `resolveOrCreateTag`) always takes the immediate-attach
  path — it cannot have ancestors or children by construction. No special-case check needed.
- **Existing root tag with no children** also takes the immediate-attach path — same low blast
  radius as a new leaf.
- **Cycle case** (the typed name resolves to an ancestor of the *current* tag): not handled by
  this dialog at all. `SetTagParent`'s existing `ErrTagCycle` guard (the same one the Parent
  control already surfaces) fires on the actual mutation call, whichever path reaches it —
  immediate-attach or post-confirm. Surface the error in whichever surface was showing at that
  moment: the add-child form's own error slot on the immediate path, or `ConfirmDialog`'s
  `error` prop if the dialog was open. Reuse the existing message shape, direction-flipped:
  `Can't move "{name}" here — {currentTag.name} is already under it.`

## 2. The confirm dialog

**Component**: `ConfirmDialog` (existing, unmodified), `variant="destructive"`.

**Title**: `Move "{candidate.name}" here?`

**Body** — composed from which of `ancestors`/`children` is non-empty, so the copy always names
what's actually about to move rather than a generic "has a subtree" line:

| Case | Body copy |
|---|---|
| Has a parent, no children | `"{name}" is currently under "{currentParentName}". Moving it here removes it from that parent — {currentParentName}'s other children are unaffected.` |
| Has children, no parent (currently a root) | `"{name}" has {n} child tag{s} of its own. They'll move here along with it — nothing is deleted, but its whole branch relocates.` |
| Has both | Both sentences, in that order, as two paragraphs. |

- `{currentParentName}` = `candidate.ancestors[candidate.ancestors.length - 1]` (the immediate
  parent, not the full chain — the chain is more than the owner needs to make this call).
- `{n}` = `candidate.children.length`. Use `tagCount(n)` (`$lib/format`, already used for
  category tag counts elsewhere) for singular/plural, not a hand-rolled `s`.
- Never say "subtree" or "branch" without also naming the count or the parent — those words
  alone don't tell the owner what's actually at stake, which is the whole point of not reusing
  the placeholder copy verbatim.

**Buttons**: `confirmLabel="Move it here"`, default `cancelLabel="Cancel"`.

**On confirm**: `SetTagParent(candidate.id, currentTag.id)`. On success, close the dialog,
clear the add-child input, refetch the current tag's `Children` (mirrors how the Parent
control's `applyParent` reloads after a successful set). On a cycle error, keep the dialog open
and surface it via `ConfirmDialog`'s `error` prop (busy resets to false so Cancel/retry both
work).

**On cancel**: close the dialog only. **Leave the add-child input's typed text in place** and
return focus to it — don't reset to empty. The owner may have meant a different tag entirely (a
typo that happened to match something with a subtree) and editing beats retyping from scratch;
this mirrors how the app never clears a form on a recoverable stop (the Parent control's own
cancel doesn't wipe the typeahead value either).

## 3. States and interactions

| Element | State | Behavior |
|---|---|---|
| Confirm dialog | Opening | Focus lands on Cancel (ConfirmDialog default) — an accidental Enter never moves a branch. |
| Confirm dialog | Confirming | `busy=true`: both buttons disabled, confirm label reads "Working…" (ConfirmDialog default — no custom busy copy needed). |
| Confirm dialog | Confirm fails (cycle) | `error` renders in the dialog's warn-colored error slot; dialog stays open, both buttons re-enable. |
| Confirm dialog | Confirm fails (other) | Same slot, generic `toMessage(err)` — mirrors every other mutation on this page. |
| Confirm dialog | Cancel / Esc / backdrop click | Dialog closes, focus returns to the add-child input (not the "+ Add child" trigger — the input, since the trigger is likely already the add-child form's own text field per `ConfirmDialog`'s `trigger?.focus?.()` on unmount, provided the input is still the last-focused element before the dialog opened). |
| Add-child input | After a cycle error surfaces inline (non-dialog path) | Input keeps focus and its text, same as above — consistent behavior whether the error came from the dialog or the immediate-attach path. |

## 4. Edge cases

- **Candidate has a parent AND children, and the parent-name lookup would be redundant with an
  already-visible ancestor breadcrumb** — not applicable here; the candidate is a *different*
  tag than the one whose page the owner is on, so its ancestors aren't otherwise visible. The
  dialog is the only place this information appears.
- **Very long tag names** — no truncation added; existing tag-name inputs elsewhere on this page
  don't truncate either (e.g. the near-miss card's `{target.name}`), so this stays consistent
  rather than introducing a new wrapping rule for one dialog.
- **Rapid double-submit of "+ Add child"** — guarded the same way every other action on this
  page already is (`tagMenu.busy`/equivalent local busy flag disabling the form during the
  resolve+getTag round trip before the dialog can even open).

## 5. Accessibility

Entirely inherited from `ConfirmDialog` — no new work required: `role="dialog"`,
`aria-modal="true"`, `aria-labelledby`/`aria-describedby` wired to the title/body, a Tab focus
trap, Esc-to-cancel, and focus restored to the triggering element on close. The only
implementation note is which element counts as "the triggering element" — it must be the
add-child text input (see §3), not the surrounding "+ Add child" toggle, so focus lands back
where the owner can immediately retype.

## 6. Visual reference

Rendered against the real Cinémathèque skin tokens (`--surface:#15110e`, `--rule:#2a2622`,
`--ink:#f3ece1`, `--muted:#9b9082`, `--warn:#e2603f`, `--warn-ink:#fdf1ee`,
`font-display: Fraunces Variable`): the two confirm-copy cases side by side, in context above the
Children control's chip row. See the mockup rendered earlier in this session
(`reparent_confirm_dialog_mockup`). QA all three skins before merging — Brutalist and Broadcast
each remap `--warn`/`--warn-ink` differently (see `app.css`), and this is the one control in the
new card that actually exercises those tokens.
