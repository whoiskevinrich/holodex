# Duplicates components

The Duplicates review queue (F43 S5, ADR-061): possible-duplicate entity pairs the owner merges
or dismisses.

| File | Purpose |
|---|---|
| `DuplicateComparePanel.svelte` | The F70 compare panel a **person** pair expands into: the match-kind label, then two columns of evidence side by side, then the row's own verdict snippet repeated as a footer. **Each column is `PersonHoverCard`'s anatomy laid flat** — same fields, same order, same absent-is-absent rule — with a strip of five 44 px `PersonImageFrame`s (headshot + up to 4 non-headshot gallery images, no `+N`) where the card's single 48 px headshot was. Composes `GET /people/{id}/card` (through F68's `loadPersonCard` cache, so a person already hovered costs nothing) + `GET /people/{id}/images`; per-side failure is inline with its own Retry and never disables a verdict. |
| `DuplicatePairRow.svelte` | One pair row: both entities (name · count), the variation kind, and the Keep-separate / Merge verdicts. A person pair also carries the disclosure that opens `DuplicateComparePanel`; the page above owns which panel is open. |
| `DuplicatesBanner.svelte` | Owner-only "N possible duplicates" notice above an entity list, deep-linking the Owner hub's Duplicates tab. Self-gating and self-fetching. |
