# Sort components

Sort and display controls shared across the browse/people/tags index pages.

| File | Purpose |
|---|---|
| `ListToolbar.svelte` | The one list toolbar (F73, `docs/design/list-toolbar-handoff.md`): layout only, no state. One row that never wraps — `[sort] [reroll?] [filters?] ··ml-auto·· [view?] [⋯?]` — then the chips row and the `aria-live` count line. Below `sm`, `reroll` moves into ⋯ when the page has one (375 px budget). State lives in `$lib/listState` (ADR-114). |
| `FilterPanel.svelte` | Filters slot: `FiltersButton` + one panel shown two ways — a non-modal popover from `sm` up, a modal bottom sheet (focus trap, Done / Clear all / live count) below. Fields apply live; no Apply button. |
| `FiltersButton.svelte` | "Filters · n" trigger; below `sm` a funnel + count. Accent when any filter is active. |
| `FilterChip.svelte` | One removable active-filter or entity-scope chip (`kind`), label truncated at 12rem. |
| `PageActions.svelte` | The owner ⋯ page-actions menu (Merge people, Manage tags, Save as playlist…); `compactOnly` items show below `sm` only. |
| `DensitySlider.svelte` | Grid-density control shared by the media list and the People poster index (both drive the site-wide `mediaDensity`). Its range spans `DENSITY_MIN..capForWidth(viewport)`, not `..DENSITY_MAX`, so no stop is inert; hides itself when the viewport allows no choice. Optional `label` renders the caption — and its reserved width — that the media list wants and the People row deliberately omits. |
| `SortDropdown.svelte` | The sort `<select>` for every list. `options` defaults to `MEDIA_SORTS` (`lib/filters.ts`); the entity lists live in `lib/listState.ts`. Owner-only entries go in a trailing `Owner` optgroup. `extra` prepends page-specific entries (the playlist page's `manual` — F69). `compact` is the ListToolbar form (sr-only caption, 32 px, can shrink); `onchange` is where the page records the preference — never an effect (ADR-114 D3). Keep labels ≤ 20 characters: a native select is as wide as its widest option. |
| `SortReroll.svelte` | "Shuffle again" button shown beside the sort picker while Random is active. |
| `segmentedToggle.ts` | Shared button/wrapper classes for the segmented-toggle look — used by the Tags create form's Tag/Category switch. (The F73 migration retired `SortToggle` and `CompletenessSortToggle`: sort is one `SortDropdown` value everywhere, HOLODEX-473.) |
