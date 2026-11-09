# rename/

File names (F86, HOLODEX-602): renaming media files in place from an ordered template list. The
surfaces are the owner's File names tab (`routes/owner/file-names`) and the File name row in the
video page's Maintenance section. Every name is shown whole, never truncated. The shared wording
(reason chips, lock lines, read-back explanations) lives in `#lib/rename.ts`, so a reason reads
the same on every surface. Design: `docs/design/file-rename-handoff.md`.

| File | Purpose |
|---|---|
| `RenamingCard.svelte` | The tab's top card: the mode radiogroup (saves at once), the settle window (Job only), the template editor and the character table sharing one unsaved draft (each kept in localStorage until saved or discarded), Preview, Save (labelled Save templates, Save characters or Save changes, bound out as `saveLabel`) and the one-line fold. Its `save()` is also the plan bar's Save. |
| `CharacterTable.svelte` | The card's Characters section (spec P0-4, HOLODEX-633): one row per character with its stand-in input (spaces shown as ␣ while unfocused), a live example from `characterExample`, and its own error. The nine a file name can't hold come first with no ×; + Add character appends a draft row with a one-character input; Reset to defaults puts the default table into the card's draft. |
| `TemplateList.svelte` | The ordered template rows inside the card: ↑ ↓ × (withdrawn at the ends and on the last row), Alt+↑/↓ from an input, + Add template, per-row counts and errors, and token chips that insert at the last-focused cursor. |
| `RenamePlan.svelte` | The plan: the reason filter row (`?reason=` in the URL, Out of date by default while it isn't empty), select-all over the server's totals, `LoadMoreButton` paging, the sticky bar with its inline confirm, and the run's per-row job states polled after Yes. |
| `RenamePlanRow.svelte` | One plan row, the same shape for every reason: the 16 px checkbox slot (empty when it can't rename), the struck old name → new name, the explanation line and the lock button. Its title links to the video page. |
| `FileNameRow.svelte` | The video page's File name row: the current name and plan state, the single-video Rename (no confirm), Lock name / Unlock, the "Renamed from" box with Undo rename, undo refused, failed with Retry and Dismiss, and History. Hidden while renaming is Off. |
| `NameLockButton.svelte` | The padlock toggle. Labelled (video page, locked plan rows): the glyph shows what the click does. Icon-only (plan rows): the glyph shows the state, with `aria-pressed`. |
| `Padlock.svelte` | The 12 px stroke padlock from `person/CoreImageEditButton`, open or closed. |
