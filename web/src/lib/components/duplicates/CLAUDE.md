# Duplicates components

The Duplicates review queue (F43 S5, ADR-061): possible-duplicate entity pairs the owner merges
or dismisses.

| File | Purpose |
|---|---|
| `DuplicatePairRow.svelte` | One pair row: both entities (name · count), the variation kind, and the Merge / Keep-separate verdicts. |
| `DuplicatesBanner.svelte` | Owner-only "N possible duplicates" notice above an entity list, deep-linking the Owner hub's Duplicates tab. Self-gating and self-fetching. |
