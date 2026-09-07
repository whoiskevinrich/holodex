# Sort components

Sort and display controls shared across the browse/people/tags index pages.

| File | Purpose |
|---|---|
| `DensitySlider.svelte` | Grid-density control shared by the media list and the People poster index (both drive the site-wide `mediaDensity`). Its range spans `DENSITY_MIN..capForWidth(viewport)`, not `..DENSITY_MAX`, so no stop is inert; hides itself when the viewport allows no choice. Optional `label` renders the caption — and its reserved width — that the media list wants and the People row deliberately omits. |
| `SortDropdown.svelte` | Media sort `<select>`; options/order come from the single source of truth in `lib/filters.ts`. |
| `SortReroll.svelte` | "Shuffle again" button shown beside the sort picker while Random is active. |
| `SortToggle.svelte` | A–Z / Most-videos / Random segmented control for the people & tags indexes. |
| `segmentedToggle.ts` | Shared button/wrapper classes for the segmented-toggle look — used by `SortToggle` here and by `entity/CompletenessSortToggle.svelte`. |
