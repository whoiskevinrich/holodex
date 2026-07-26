# Component folder layout

Components are grouped by **feature/domain**, not by component type. Each subfolder has its own
`CLAUDE.md` listing every file in it with a one-line purpose — read that first when you're
already inside a folder; this file only covers how to decide *which* folder a component belongs
in.

## The eleven folders

`person/`, `enrichment/`, `extraction/`, `curation/`, `duplicates/`, `entity/`, `activity/`,
`video/`, `sort/`, `writeback/`, `shared/`.

## Classification rule

Two axes are both in play, and neither one wins outright — apply them in this order:

1. **Consumer-based, when a component is genuinely used across multiple entity types.**
   `entity/` holds components consumed by person, studio, *and* tag routes (`EntityPicker`,
   `MergeCanonicalDialog`, `EntityVideos`) — they can't belong to any single feature folder
   without misleading the reader about where they're used.
2. **Function-based, otherwise.** Most folders group by what the component *does* in the
   product (enrichment review, curation/source-of-truth, writeback-to-file), independent of how
   many pages happen to import it. `writeback/CropEditor.svelte` is filed here even though today
   it has one caller (`person/PersonGallery.svelte`) — it's grouped by the writeback mechanism
   (WYSIWYG crop → canvas → upload) it implements, not its current call sites, since a second
   caller (e.g. a studio logo crop) would use the exact same mechanism.
3. **`shared/` is the fallback**, reserved for components with zero domain knowledge of their
   own (a loading shell, a generic confirm dialog) — not a catch-all for "used in more than one
   place." A component that knows about entities/fields/providers/jobs belongs in a domain
   folder even if only one page currently renders it.

## Adding a new component

- If it's consumed by 2+ distinct entity types (person/studio/tag) with no shared function-based
  home already covering it → `entity/`.
- Else, name the product mechanism it belongs to and use that folder, even if there's currently
  only one caller.
- Only reach for `shared/` if the component has no domain knowledge to leak — it would make
  sense in a component library with no idea what Holodex is.
- Update that folder's `CLAUDE.md` table with the new file in the same change.
