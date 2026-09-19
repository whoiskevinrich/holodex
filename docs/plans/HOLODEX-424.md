---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-424
status: in-progress
release_note: The media detail page now shows a provider link pill for the provider you matched the video to — a TMDB-matched video links to its TMDB page just as a matched person does — instead of only when the file carried an external-id tag.
---

# HOLODEX-424 · Media detail provider pill derives from the provider match too

Raised 2026-09-19 as "the media details page needs an enrichment provider link pill like the
Person page has". The source said otherwise: the pill **shipped** in F63 (HOLODEX-394, PR #344,
merged 2026-09-17) after the year on the header meta row — but `externalLinksForVideo` reads only
the resolver's `external_provider_id`, while person/studio/film read the accepted match from
`entity_external_ids`. A TMDB-matched video with no file tag therefore renders nothing, and even
with one it shows IMDb (the sidecar emits `external_provider_id = imdb:<id>`), never TMDB.
Derivation gap, not missing UI.

**Decision (Option A, from a three-skin before/after mockup):** fold the match id already stamped
on the video's enrichment rows (`EnrichmentRow.ExternalID`, `tmdb:812`) into the same list —
resolved value first, then rows, dedup by lowercase namespace, `ProviderLink` per pill. TMDB's
existing `video` template links it with no sidecar change. **Option B** (do HOLODEX-382 first and
reuse `externalLinksForEntity`) deferred: a data-model move gating a ~20-line read; RD4 still holds.

## Gates — definition of done

- [x] design `design-handoff` — `provider-link-badge-handoff.md` §6 / DD6 +
  `provider-link-badge-media-match-mockup.svg` (three skins × today / after / match+tag /
  hover / degraded, plus the derivation strip)
- [x] spec `write-spec` — `provider-link-badge-coverage.md` P0-7b (five ACs), RD12, non-goal
  wording
- [~] architecture — n/a: ADR-098 D4 ("video has no identity rows") and RD4 still hold; this reads
  a second input from rows the handler already fetches. Flag if a superseding note is wanted.
- [ ] backend — `externalLinksForVideo`: merge resolved value + `enrichRows[].ExternalID`, dedup
  by namespace, same `ProviderLink` precedence; `TestExternalLinks_Video` gains the five P0-7b
  rows (match only · match + foreign-ns tag · same-ns dedup · degraded match · extraction-only →
  `null`)
- [~] frontend — none: page already `{#each}`es `external_links`; `enrichment/CLAUDE.md`
  `ProviderLinkBadge` row refreshed (it still said "video later")
- [ ] testing `testing-strategy` — §4 projection row extended to the match input
- [~] security — n/a: read-only projection; hrefs still come from validated templates /
  ingest-validated `_source_url`; `isHttpUrl` gate unchanged
- [ ] three-skin QA — Aladdin (backend-films testbed) renders `1992 · IMDb TMDB` on one 24 px
  line in all three skins; 375 px wraps per segment; a match-only fixture renders `· TMDB`

## Up next

1. Implement the backend change + tests; `go test ./internal/api/...`
2. Testing-strategy row; three-skin QA on the testbed
3. Mark PR ready → CI fires In Review for 424
4. Hand-sweep leftovers from F63: **HOLODEX-391, 393, 394 still `In Review`** (PR #344 merged;
   390 + 392 moved to Done this session; the other three were denied by the auto-mode classifier)

## Session log

- **2026-09-19** — `/design-handoff`. Traced the gap to `externalLinksForVideo`; rendered the
  three-skin mockup; Kevin picked Option A. Filed HOLODEX-424, renamed the branch, In Progress
  fired. Handoff §6, SVG, spec P0-7b/RD12, this worklog. Draft PR opened. **Handoff:** the design
  and spec gates are green; the next session implements the ~20-line backend change in
  `externalLinksForVideo` and its five test rows, then QAs on the testbed.
