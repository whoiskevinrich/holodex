# Shared components

Generic, feature-agnostic UI primitives with no domain knowledge of their own — safe to import
from any feature folder.

| File | Purpose |
|---|---|
| `AsyncState.svelte` | Shared loading/error shell so route pages don't each re-implement the "Loading…" + error-box choreography. |
| `ConfirmDialog.svelte` | Destructive confirm dialog (focus trap, Esc/backdrop cancel, focus returned to trigger); the modal idiom other pickers/dialogs follow. |
| `ExpandableText.svelte` | Line-clamped long text with a chevron expand/collapse toggle (the `CompletenessPanel` chevron idiom applied to prose); used for Media Overview, Person bio, and Film description. **Long prose is this component, no exceptions** — bio, overview, description, comments, and whatever comes next all render muted `text-sm leading-relaxed` with the chevron, for owner and visitor alike. It deliberately has no styling props (`tone` was removed in HOLODEX-365 once it turned out only one of three call sites set it): a knob makes "one look for prose" opt-in per page, and a page that needs a different look is a design question, not a prop. Only `lines` (4 / 5) is configurable, because that is a layout fit. |
| `HotkeySheet.svelte` | The `?` keyboard-shortcuts sheet (F62): a derived *This page* group from the hotkey registry (`lib/actions/hotkey.svelte.ts`) plus a static *Navigation* group; ConfirmDialog's surface/trap/motion; Page rows are launchers (close, then `focus()`→`click()` the bound button). Two columns past six rows, never scrolls. |
