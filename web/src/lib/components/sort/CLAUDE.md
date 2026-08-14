# Sort components

Sort controls shared across the browse/people/tags index pages.

| File | Purpose |
|---|---|
| `SortDropdown.svelte` | Media sort `<select>`; options/order come from the single source of truth in `lib/filters.ts`. |
| `SortReroll.svelte` | "Shuffle again" button shown beside the sort picker while Random is active. |
| `SortToggle.svelte` | A–Z / Most-videos / Random segmented control for the people & tags indexes. |
| `segmentedToggle.ts` | Shared button/wrapper classes for the segmented-toggle look — used by `SortToggle` here and by `entity/CompletenessSortToggle.svelte`. |
