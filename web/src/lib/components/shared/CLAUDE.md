# Shared components

Generic, feature-agnostic UI primitives with no domain knowledge of their own — safe to import
from any feature folder.

| File | Purpose |
|---|---|
| `AsyncState.svelte` | Shared loading/error shell so route pages don't each re-implement the "Loading…" + error-box choreography. |
| `ConfirmDialog.svelte` | Destructive confirm dialog (focus trap, Esc/backdrop cancel, focus returned to trigger); the modal idiom other pickers/dialogs follow. |
| `ExpandableText.svelte` | Line-clamped long text with a chevron expand/collapse toggle (the `CompletenessPanel` chevron idiom applied to prose); used for Media Overview, Person bio, and Film description. |
