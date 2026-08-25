# Film components

The two asymmetric attach pickers for the Films entity (F56, ADR-085, RD9) — video→film and
film→video are deliberately separate components, not one shared modal (see each file's header
comment for why).

| File | Purpose |
|---|---|
| `FilmAttachDialog.svelte` | Video→film attach (design handoff §3b): single-select film search (poster thumb + name/year rows), then a second in-dialog step for scene number / full-film flag. When search finds no match, a "+ Create as a new film" action calls the get-or-create `createFilm` endpoint and advances straight into the attach step — the only way to create the system's first film. Chrome comes from `PickerShell` (shared with `EntityPicker`/`CategoryPicker`); result rows and the two-step confirm are this component's own body. |
| `FilmBulkAttachDialog.svelte` | Film→video bulk attach (design handoff §4): whole-library search, default-unattached scope, studio/cast filter chips, multi-select + roving-tabindex, sequential auto-numbering from a starting scene number, all-or-nothing 409 handling. Chrome comes from `PickerShell` (widened via `widthClass`/`paddingClass`), same as `FilmAttachDialog` — result shape and commit flow still differ enough to keep this its own component. |

The film poster/thumb upload control moved to `entity/EntityImageSlot.svelte` (HOLODEX-286) — it's
now shared with Studio's icon/logo/poster roles rather than filed per-entity.
