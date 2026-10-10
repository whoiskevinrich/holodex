# Handoff: Session-expired banner — hold the reload while editing

**Spec**: [owner-session-persistence.md](../specs/owner-session-persistence.md) (OS8) ·
**Jira**: [HOLODEX-502](https://whoiskevinrich.atlassian.net/browse/HOLODEX-502) ·
**Date**: 2026-10-09 · **Skin**: Cinémathèque (the only look)

## Decision

When the auth proxy in front of Holodex ends the owner's sign-in, an idle page still recovers by
reloading itself, as it has since HOLODEX-127. A page with something to lose doesn't reload. It
shows a warning strip under the header instead, and the owner signs in again **in a new tab**, so
what they typed stays where it is. The owner chose this on 2026-10-09 ("hold the reload while
editing", option B), over keeping a draft across the reload (A) and never reloading by itself (C).

![Three states of a warning strip under the Holodex header, in Cinémathèque. 1, edit: "Your sign-in expired. Your edits are still here. Sign in again before you save." with a bordered "Sign in again ↗" button. 2, write: "Your sign-in expired, so your last change wasn't saved. Sign in again, then save it again." sitting above an open Edit Overview dialog whose text is still in place, with the dialog's own inline line "Your sign-in expired, so this change wasn't saved." 3, resubmit: "You're signed in again. Your last change still isn't saved. Save it again." with a Dismiss button.](session-expired-banner-mockup.svg)

## States

| State | When | Lead (warn, semibold) · body (ink) | Action | Role |
|---|---|---|---|---|
| **edit** | The sign-in lapsed while the page has unsaved input. Nothing is lost yet. | "Your sign-in expired." · "Your edits are still here. Sign in again before you save." | `Sign in again ↗` | `status` |
| **write** | A save was redirected by the proxy and never reached Holodex. | "Your sign-in expired, so your last change wasn't saved." · "Sign in again, then save it again." | `Sign in again ↗` | `alert` |
| **resubmit** | The sign-in is back after a **write**. | "You're signed in again." · "Your last change still isn't saved. Save it again." | `Dismiss` | `status` |

- **edit** clears by itself once the sign-in is back. Nothing was lost, so there is nothing to say.
- **resubmit** clears on the next successful save, or on Dismiss.
- **write** is never downgraded to **edit** by a later failed background read.
- No banner when nothing would be lost: no unsaved input and no failed save means the silent reload.

## Interaction

- **Sign in again ↗** opens the current page in a new tab (`noopener`). Loading a page there signs
  in through the proxy, and this page keeps its input. The tooltip reads "Opens in a new tab".
- **Unsaved input** is any typing or change in a field since the last successful save or the last
  move to another page, **or any open dialog** (pickers, the crop editor and radio-card choices change
  state by click and fire no input event). The header search box doesn't count, and neither does a
  change to the query string or hash on the same page.
- **An owed change stays owed.** After a **write** or during **resubmit**, a later lapse never falls
  back to the silent reload, even for an action that involved no typing.
- **The failing control keeps its own error.** A redirected save also fails the control that sent
  it, and its inline error now reads "Your sign-in expired, so this change wasn't saved." The banner
  doesn't replace that line. The line says *which* change failed, and the banner says what to do.

## Layout and tokens

- A full-width strip directly under the header: `sticky top-0 z-[70] border-b border-warn bg-surface
  px-6 py-2 text-sm text-ink`, wrapping (`flex-wrap`) at narrow widths.
- **Sticky, above every overlay.** Dialogs are `fixed inset-0 z-50` and the image viewer is `z-[60]`.
  The edit worth protecting is usually inside an open dialog, and its backdrop would otherwise cover
  the only way out, because closing the dialog to reach the banner would throw the text away. The
  owner may also be scrolled far down the page when they save.
- **Sign in again** is `btn-ghost px-3 py-1.5 text-sm`, an immediate resolve. **Dismiss** is
  `btn-quiet px-3 py-1.5 text-sm`, a UI-only toggle. The page's solid accent stays with the page's
  own primary action, which during a held save is the dialog's Save.
- The lead uses `text-warn`, not `--accent`. Attention states use `--warn` (owner-session spec OS4).

## Component

`web/src/lib/components/activity/SessionExpiredBanner.svelte`, mounted once in the root layout
under `</header>`. It reads the held state from `$lib/reauth.svelte`, and nothing else renders it.
