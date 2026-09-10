---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-362
status: in-review
release_note: Permanent delete now sits behind a menu on the media page, so it can no longer be hit by overshooting Move to Trash.
---

# HOLODEX-362 · Split-button the Manage block so permanent delete is not adjacent to Move to Trash

The media detail page's owner-only Manage block rendered both delete paths as the same control:
identical `border-warn` / `text-warn` outline, identical padding and radius, `gap-2` apart. One is
reversible (soft-delete, restorable from `/trash`, purged on a grace timer); the other destroys the
file immediately. Nothing in the rendering said which was which, and overshooting the first by one
button-width landed on the second. Found by a `design-critique` pass over the
[HOLODEX-342](HOLODEX-342.md) stress fixture. Done means only the reversible action is on screen at
rest, and the irreversible one — once revealed — is visibly *heavier* than the action it drops from.

**The hierarchy is carried by fill vs. outline, not by leaving the warn family.** The tempting
cheap fix was to demote `Move to Trash` to a neutral `border-rule` so warn would mean "no undo"
exclusively. Rejected on the owner's call: moving a file to Trash is still destructive from the
owner's point of view — it leaves the library and purges on a timer — so a neutral button
undersells it. The default segment stays warn-outlined; the menu item is solid `bg-warn` /
`text-warn-ink`. That pairing is not new: all three skins already ship a `--warn-ink` documented for
a solid destructive fill, because [HOLODEX-324](HOLODEX-324.md) did that contrast work.

**This is a reuse job, not a new primitive** — a claim that was wrong when this session first made
it, and worth recording because it changed the shape of the work. `EnrichProviderChips.svelte`
already implements exactly this control: `relative inline-flex items-stretch rounded-theme border`
wrapping a primary segment and a `border-l` trigger, with an absolutely-positioned `role="menu"`
beneath. `use:dismissable` already owns Escape / click-outside with the
focus-return-on-keyboard-only split. Two deliberate divergences: a chevron instead of the overflow
glyph (the menu holds a *variant of the action to its left*, not more actions of the same kind), and
a warn-bordered wrapper because the whole control is destructive. `PopoverMenu` was **not** reused —
it is keyed by numeric row id and carries an inline value/busy/error form slot, so a single instance
with no form would have to invent a fake id to use it.

## Gates — definition of done

- [~] spec `write-spec` — n/a: no new capability or changed requirement. Both delete paths already
  existed with the same semantics; this changes how one of them is reached
- [~] architecture `architecture` — n/a: no seam, stack or data-model decision. ADR-037's
  soft-delete/purge model is untouched
- [x] design `design-handoff` — [manage-split-button-handoff.md](../design/manage-split-button-handoff.md)
  + committed [SVG mockup](../design/manage-split-button-mockup.svg) (before/after, all three skins,
  behaviour panel). Revises [delete-media-handoff.md](../design/delete-media-handoff.md) §1
- [~] backend — n/a: frontend-only. Both endpoints and both confirm dialogs are unchanged
- [x] frontend — split button in `web/src/routes/media/[id]/+page.svelte` §Manage
- [x] testing `testing-strategy` — the Delete/Trash (F24) row in `docs/testing-strategy.md` now
  carries the split-button behaviours and the measured token values. **No geometry assertion**, and
  that is the documented call rather than a gap: §12.2 excludes colour, focus and contrast from that
  harness, so §5's computed-token method is the right instrument here and is what ran
- [~] security `security-review` — n/a: no auth, access or infrastructure change. The `isOwner`
  gate on the Manage block is byte-identical

## Up next — ordered (position = priority)

1. [ ] [M] The visitor right rail is ~65% empty — the other half of the same `design-critique`.
   `+page.svelte`'s Metadata section is gated on a bare `{#if isOwner}` (the comment there records it
   as deliberate: "visitors previously saw a filtered subset"), and `File` likewise, so with Tags and
   People alone the rail runs ~250px against an ~800px left column. Two ways out: restore a
   read-only field list for visitors, or collapse to one column when the rail is thin. **Not
   ticketed yet** — it reverses a decision already taken in code, so it wants the owner's call first.
2. [ ] [S] `FilmAttachDialog.svelte` trips the theming rule's own grep
   (`text-muted[^"]*disabled:opacity`). It is a `placeholder:text-muted` on a `text-ink` input rather
   than a dimmed label, so it is probably a false positive on the pattern — but the check is
   documented as "should be empty" and currently is not. Either fix the input or tighten the grep.
   Pre-existing, untouched here.
3. [ ] [—] `.claude/launch.json` in this worktree inlines a live provider API token. Gitignored and
   untracked (verified), so nothing leaked — but ADR-094 moved local dev credentials to env, and this
   file did not follow.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-10 · split button, built out of the chip pattern that already existed
- skills: design-critique, design-handoff, code-review (high --fix), handoff
- verified, live, against `backend-amv` media 209 as owner: at rest the block renders one control
  and `Delete permanently` is **absent from the DOM** — the assertion that actually encodes the fix.
  Opening moves focus to the first `role=menuitem`; Escape closes **and** returns focus to the
  chevron; an outside click closes and **leaves focus alone**; choosing the item closes the menu and
  opens the unchanged purge `ConfirmDialog`, the default segment the soft one. Both dialogs were
  opened and cancelled — nothing was deleted.
- three-skin QA by computed token, not by eye: outline resolves to `--warn` and the item to
  `bg-warn`/`text-warn-ink` in every skin (cinémathèque `#e2603f`/`#170e08`, broadcast
  `#ff6f61`/`#0a0610`, brutalist `#ff5e3a`/`#0a0a0a`), `--radius` following the skin (2px / 0 / 0).
  Contrast computed from the token blocks: 5.42–7.36:1, all AA. Green: `npm run check` 0 errors
  (15 warnings, all pre-existing, none in this file) · vitest 272/272.
- checked rather than assumed: the menu is `absolute left-0`, so it grows rightward out of a
  right-rail control — the [HOLODEX-356](HOLODEX-356.md) horizontal-overflow failure class. Measured
  it: the menu is only **25px** wider than the wrapper and the rail always has more slack than that,
  and at 768 the layout is single-column anyway (`scrollWidth === clientWidth`, no sideways scroll).
  `left-0` stands; `right-0` would have been cargo-culted from `EnrichProviderChips`, where it exists
  because chips sit in a flex-wrap row that can reach the viewport edge.
- correction worth carrying: this session first told the owner a split button would be "a new shape"
  for the repo and priced the consistency cost accordingly. It is not — `EnrichProviderChips` had
  shipped it. Grep for the *shape* (`inline-flex items-stretch`, `aria-haspopup`) before pricing a
  new control, not just for the component name.
- handoff: complete and pushed ready for review; nothing left to build. The one open question is
  queue item 1 — the visitor right rail is the other half of the critique that produced this ticket,
  and it deliberately stops short of a ticket because fixing it means reversing a gating decision
  the code records as intentional. That is the owner's call, not a cleanup.
