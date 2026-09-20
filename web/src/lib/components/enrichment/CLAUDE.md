# Enrichment components

Provider metadata enrichment: the disambiguation picker, per-provider chip controls, the
provenance/brand-icon system, and the enrichment review queue row.

| File | Purpose |
|---|---|
| `EnrichPicker.svelte` | Modal listbox of provider candidates the owner searches and confirms (roving-tabindex combobox/listbox). |
| `EnrichProviderChips.svelte` | Owner enrich controls as one compact chip per provider (icon + name + primary action + overflow menu). Shared by person/media/studio detail pages. |
| `EnrichQueueRow.svelte` | One row in the Enrichment review queue — a status chip per outstanding provider plus one derived row action ("Review" / "Try again"). |
| `ProvenanceBadge.svelte` | Labels where a resolved field value came from — a provider's brand icon, or a muted "from file" pill. |
| `ProviderIcon.svelte` | Provider brand glyph: the self-hosted icon when cached, else a themed monogram fallback. |
| `ProviderLinkBadge.svelte` | One outbound provider-link pill (HOLODEX-266, ADR-083 D2/D3) per external id — a clickable `<a>` when the provider declared a link template (or stored a `_source_url`, ADR-098), else a non-interactive "known to" `<span>`. Person/studio mount it through `EntityVideoMeta`; film and media mount it directly on their year / meta line (F63 DD4/DD5). A video's list is the resolved `external_provider_id` **plus** the provider match on its enrichment rows (HOLODEX-424, DD6) — never an identity row. |
| `ProviderStatusChip.svelte` | Read-only sibling of `EnrichProviderChips` for queue rows — same chip shell, no button/menu, just a state label. |

## Rules

### Ungated-aspect image slots use the plate with `object-contain`

A thumbnail whose source aspect Holodex does **not** enforce at ingest — a `/resolve`
`candidates[].image_url` (F64), any provider-hot-linked render — sits in the `FilmsRow` tile
idiom (`rounded-theme bg-logo-plate overflow-hidden`, monogram `font-display font-semibold
text-logo-plate-ink`) with **`object-contain`**, so a wide logo letterboxes and a portrait
fills. `object-cover` is reserved for roles whose aspect ingest guards (film banner/poster,
HOLODEX-386). Same rule as `entity/CLAUDE.md` "Frame follows source aspect"; recorded here
because the picker is where it was decided the second time. Handoff:
`docs/design/candidates-image-handoff.md`.

### The candidate slot's box is kind-shaped, always 60 px tall, never `aspect-*`

`EnrichPicker` takes a required `entityType` and draws the slot from
`SLOT_CLASS[slotShape(entityType)]` (`$lib/candidateImage`): person/film `w-10 h-15`, video
`w-27 h-15` (the provider sends a backdrop), studio `w-30 h-15` (HOLODEX-414). Add a kind by
adding a `SlotShape`, never an `{#if}` in the template. Both axes are explicit on purpose — an
`aspect-*` box with a `w-full` `<img>` can borrow the image's natural width (the F64
fixture-size trap), and the geometry harness asserts the slot's px exactly. The height is the
row-height floor (76 px collapsed) in every picker from `sm` up; only the width changes per
kind. Below `sm` the two wide shapes drop to 80 wide at the same aspect and the row stacks the
match strength under the name — a 315 px dialog left a studio name ~30 px otherwise (QA §4.4).
Handoff: `docs/design/candidates-image-per-kind-handoff.md`.
