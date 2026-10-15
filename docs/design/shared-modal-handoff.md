# Design Handoff: Shared Modal primitive (HOLODEX-215)

**Spec**: none. No product rule changes; every dialog keeps its content, copy, and actions.
**Theming contract**: [theming.md](theming.md). Tokens only, QA Cinémathèque.
**Surfaces**: a new `shared/Modal.svelte`, and the 11 dialogs that adopt it (listed under
[Call sites](#call-sites)).
**Mockup**: ![Shared Modal: two positions, stacking, and four widths](shared-modal-mockup.svg)
**Jira**: [HOLODEX-215](https://whoiskevinrich.atlassian.net/browse/HOLODEX-215)

---

## Problem

Eleven dialogs each build their own modal shell. The shells drift:

- five vertical paddings (6, 8, 10, 12, and 20vh) and two alignments
- seven widths
- two z-indices
- seven focus-trap selectors
- Escape bound in three different places

Every focus or dismissal bug has to be fixed in each copy separately. Because most dialogs bind
Escape on the window, one Escape closes every open dialog at once. `PersonGallery` works around
this by routing Escape by hand.

## Decisions (approved 2026-10-14)

1. **Two positions.** Placement is chosen by name, never by a `vh` value.
2. **Four widths.** Width is chosen by name, never by a `max-w-*` class.
3. **A stack.** Escape and backdrop clicks close only the topmost modal. A modal opened over
   another one sits one layer higher.
4. **Modal owns the surface and the motion. The caller owns the inside.**

Rejected options:

- **Keep each dialog's exact vh and width.** That would have kept the drift and given every
  call site a knob to keep drifting with.
- **Mirror each dialog's current Escape binding.** That would have kept the double-close.
- **Leave panel classes with callers.** That would have repeated the surface classes eleven
  times.

## Component: `shared/Modal.svelte`

| Prop | Values | Default | Notes |
|---|---|---|---|
| `titleId` | string | required | Wired to `aria-labelledby`. The caller renders the heading with this id. |
| `onclose` | `() => void` | required | Called on Escape and on a backdrop click when the modal is dismissable. |
| `size` | `sm` · `md` · `lg` · `media` | `md` | See [Widths](#widths). |
| `align` | `top` · `center` | `top` | See [Positions](#positions). |
| `busy` | boolean | `false` | When true, Escape and backdrop clicks are ignored. The caller still disables its own buttons. |
| `describedBy` | string | — | Optional `aria-describedby`. ConfirmDialog uses it. |
| `data-*` | any | — | Passed through to the panel. HotkeySheet uses it for `data-hotkey-sheet`. |
| `returnFocus` | boolean | `true` | Set it to false only when the caller moves focus itself on close. HotkeySheet's launcher rows do this. |
| `panel` | bindable `HTMLElement` | — | For callers that query inside the panel. |
| `children` | snippet | required | Everything inside the panel. |

There are no styling props. Padding, inner layout, the title element, and the buttons all belong
to the caller. This follows the `shared/` "no knobs" rule.

### Positions

| `align` | Backdrop | Used by |
|---|---|---|
| `top` | `items-start py-[10vh]` | forms, pickers, confirms: everything except media |
| `center` | `items-center py-[6vh]` | PersonGalleryModal, PersonImageViewer |

The positions that change: Confirm and Hotkeys move up from 12 to 10vh, CropEditor moves down from
8 to 10vh, and WritebackBatch moves up from 20 to 10vh.

### Widths

| `size` | Class | Used by |
|---|---|---|
| `sm` | `max-w-md` (28rem) | ConfirmDialog (and its wrappers), HotkeySheet, CropEditor, WritebackBatchDialog |
| `md` | `max-w-lg` (32rem) | PickerShell family, EnrichPicker, EntityPickerDialog, ExtractionPreviewDialog (was 36rem) |
| `lg` | `max-w-3xl` (48rem) | WritebackFormDialog, FilmBulkAttachDialog |
| `media` | `max-w-5xl` (64rem) | PersonGalleryModal, PersonImageViewer (was 56rem; its image is height-bound, so the change barely shows) |

### Surface

| Part | Classes / tokens |
|---|---|
| Backdrop | `fixed inset-0 flex justify-center px-4`, `role="presentation"`; `bg-bg/70` at the base layer, `bg-bg/85` when stacked |
| Panel | `flex w-full flex-col rounded-theme border border-rule bg-surface shadow-xl`, plus `max-h-[80vh]` (`media`: `max-h-[85vh]`), `tabindex="-1"`, `role="dialog"`, `aria-modal="true"` |
| Overflow | The panel never scrolls. The caller's body region takes `min-h-0 flex-1 overflow-y-auto` (HOLODEX-417). |

The writeback family (WritebackForm, WritebackBatch, and ExtractionPreview) moves from `shadow-lg`
to `shadow-xl` and gains the rise animation. Their header bars stay unchanged because the caller
owns them.

### Layers

The base layer is `z-50`. Each modal opened while another is open goes one step higher: the second
is `z-[60]`. Two stacked modals is the most the app ever opens (Gallery → Viewer). A third would
also take `z-[60]` and still sit on top, because it comes later in the DOM.

**A stacked modal's backdrop is `bg-bg/85`** (approved 2026-10-14). The rule follows the stack,
not the dialog: a second dim layer over an already-dimmed modal needs to read as a separate layer.
Today only the Viewer stacks, so it looks the same as before. Flattening everything to `/70` was
rejected because it let the gallery show through the Viewer's surround.

## States and interactions

| Trigger | Topmost modal, idle | Topmost modal, `busy` | Modal below the top |
|---|---|---|---|
| Escape | `onclose()`; the event stops there | ignored, and it does not fall through to the modal below | not reached |
| Click on the backdrop itself | `onclose()` | ignored | not reachable (covered) |
| Click inside the panel | nothing | nothing | — |
| Tab / Shift+Tab | cycles within the panel | cycles within the panel | — |

- **Open.** On mount, Modal records `document.activeElement` as the opener. It then focuses
  the first focusable element, or the panel if there is none. A caller that wants a different
  target focuses it in its own `onMount`, which runs after Modal's: Cancel, a search input, the
  checked radio, or the first grid item.
- **Close.** On teardown, focus returns to the opener, unless `returnFocus` is false or the opener
  has left the DOM.
- **Focusable set.** One selector everywhere:
  `button, input, select, textarea, a[href], [tabindex]:not([tabindex="-1"])`, excluding disabled
  and hidden elements. If nothing is focusable, Tab is swallowed and focus stays on the panel.
- **Escape inside a field that handles it.** A control that calls `stopPropagation` on its own
  Escape (SourceChipRow's custom input) keeps that Escape. Modal listens on the window in the
  bubble phase, so a stopped event never reaches it.
- **Hotkey suppression.** `routes/+layout.svelte` and `lib/reauth.svelte.ts` detect a dialog by
  looking for `[role="dialog"]`. Those checks keep working unchanged. HotkeySheet keeps its
  `data-hotkey-sheet` exemption by passing the attribute through to the panel.

## Motion

| Element | Trigger | Animation | Duration | Easing |
|---|---|---|---|---|
| Panel | mount | opacity 0 → 1, scale 0.98 → 1 | 150ms | `cubic-bezier(0.2, 0.7, 0.2, 1)` |

Only under `prefers-reduced-motion: no-preference`. Close is instant, as today. This replaces the
three copied keyframes: `confirm-rise`, the HotkeySheet rise, and `merge-rise`.

## Responsive

There are no breakpoints. `w-full` with a `max-w-*` cap and `px-4` on the backdrop gives a 16px
gutter on a phone, and every size collapses to full width minus that gutter.

## Call sites

| Dialog | `size` | `align` | Busy source | Notes |
|---|---|---|---|---|
| `shared/ConfirmDialog` | sm | top | `busy` | `describedBy`; initial focus is the checked radio, otherwise Cancel |
| `shared/HotkeySheet` | sm | top | — | `returnFocus=false` when a row launches; `data-hotkey-sheet` |
| `entity/PickerShell` | md (lg for FilmBulkAttach) | top | — | becomes the picker chrome (✕, header snippet) over Modal; its `widthClass`/`paddingClass` props are removed |
| `entity/EntityPickerDialog` | md | top | — | **gains Escape**, which it lacks today |
| `enrichment/EnrichPicker` | md | top | — | |
| `extraction/ExtractionPreviewDialog` | md | top | `busy` | |
| `writeback/WritebackFormDialog` | lg | top | `busy` | |
| `writeback/WritebackBatchDialog` | sm | top | `phase === 'starting'` | |
| `writeback/CropEditor` | sm | top | `busy` | |
| `person/PersonGalleryModal` | media | center | — | Escape now its own, not routed by PersonGallery |
| `person/PersonImageViewer` | media | center | — | stacked layer; ←/→ stay with PersonGallery |

ConfirmDialog's wrappers (`SourceEditModal`, `MergeCanonicalDialog`, and `EditSceneNumberDialog`)
inherit the change without edits.

Out of scope: `sort/FilterPanel` (a sheet that is modal only when narrow), and `video/PlaylistPicker`
and `video/RunJumpPopover` (non-modal popovers on `use:dismissable`).

## Accessibility

- Every panel has `role="dialog"`, `aria-modal="true"`, and `aria-labelledby={titleId}`.
  ExtractionPreview, WritebackForm, and WritebackBatch currently lack `role="presentation"` on the
  backdrop, and gain it.
- A modal underneath the top one stays in the DOM. `aria-modal` on the topmost one tells assistive
  tech to ignore it.
- `aria-live` status regions inside callers stay unconditionally rendered, as today.

## Edge cases

- **Opener removed while open:** focus return is skipped, and the browser drops focus to `body`.
- **Busy flips during an Escape:** the check runs when the key is pressed.
- **Two modals unmount in the same tick** (closing the gallery while the viewer is open): the stack
  pops both, and focus returns to the gallery's opener.
