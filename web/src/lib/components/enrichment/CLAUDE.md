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
| `ProviderLinkBadge.svelte` | One outbound provider-link pill (HOLODEX-266, ADR-083 D2/D3) for a person/studio/video's stored external id — a clickable `<a>` when the provider declared a link template, else a non-interactive "known to" `<span>`. Shared by `EntityVideos` (studio) and the person page's own header. |
| `ProviderStatusChip.svelte` | Read-only sibling of `EnrichProviderChips` for queue rows — same chip shell, no button/menu, just a state label. |
