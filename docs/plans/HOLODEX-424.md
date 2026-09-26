---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-424
status: in-review
profile: feature
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
- [x] backend — `externalLinksForVideo`: resolved value + one id per provider from its **newest**
  enrichment row (a re-match upserts without clearing — `/code-review high` caught the stale-id
  case), dedup by namespace through the new shared `linksFromIDs` (also what
  `externalLinksForEntity` now calls); `TestExternalLinks_Video` re-tabled on a `match` column +
  namespace→url `want` map, six new cases incl. the backdated re-match; mutation-checked both
  ways; `go test ./internal/api/` green
- [~] frontend — none: page already `{#each}`es `external_links`; `enrichment/CLAUDE.md`
  `ProviderLinkBadge` row refreshed (it still said "video later")
- [x] testing `testing-strategy` — §4 projection row + §11 P0 coverage map extended to the
  match input and the re-match rule
- [x] three-skin QA — 2026-09-19 on backend-films (fresh scratch DB, this worktree's sidecar on
  :9101 — the shared :9100 sidecar predates 391 and declares no templates, so every pill degraded
  until swapped). Aladdin: **before the match, no pill at all** (the films mapping never fed
  `external_provider_id` from the filename); after `tmdb:812`: `2016 · [IMDb] [TMDB]` on one 24 px
  line ×3 skins (pill 22 px, gap 8, text AA 6.31 / 4.9 / 5.73), hrefs `imdb.com/title/tt0103639/` +
  `themoviedb.org/movie/812`, `rel=noopener noreferrer`, aria "View … on X's site (opens in a new
  tab)"; hover + keyboard `:focus-visible` (Tab from "+ Set part" → IMDb → TMDB) go accent border /
  ink text; 375 px wraps to two lines, `scrollWidth == clientWidth`. **Match-only fixture** (provider
  `external_provider_id` row dropped): `2016 · [TMDB]` ×3 skins — the reported gap. Unmatched
  (300): no pill, no trailing `·`. Screenshots captured in-session (all three skins)

## Up next

1. PR #363 marked ready → CI fires In Review for 424; on merge CI fires Done
4. Hand-sweep leftovers from F63: **HOLODEX-391, 393, 394 still `In Review`** (PR #344 merged;
   390 + 392 moved to Done this session; the other three were denied by the auto-mode classifier)

## Session log

- **2026-09-19** — `/design-handoff`. Traced the gap to `externalLinksForVideo`; rendered the
  three-skin mockup; Kevin picked Option A. Filed HOLODEX-424, renamed the branch, In Progress
  fired. Handoff §6, SVG, spec P0-7b/RD12, this worklog. Draft PR opened. **Handoff:** the design
  and spec gates are green.
- **2026-09-19 (later)** — Backend implemented; `/code-review high --fix` found the re-match
  stale-id case → newest-row-per-provider rule + test; testing-strategy updated. **Handoff:** only
  three-skin QA on the testbed stands between this and `gh pr ready`.
- **2026-09-19 (QA)** — Three-skin QA green (see gate). Merged main, PR marked ready. **Handoff:**
  nothing open; on merge, CI moves 424 to Done. Leftover from F63: 391/393/394 still need the
  by-hand Done sweep.

### 2026-09-19 · session
- skills: code-review
