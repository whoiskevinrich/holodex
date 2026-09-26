---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-386
status: in-progress
profile: feature
release_note: A portrait image offered as a film's banner is now refused instead of being cropped into the header band — the enrich activity log names the dimensions, and an owner upload gets a plain-language error.
---

# HOLODEX-386 · Refuse a portrait image for the film banner role (landscape guard)

The film header renders the `banner` role cover-fit into an 8:3 band. A sidecar that emits its
*poster* URL under the `banner`/`backdrop` kind (HOLODEX-387) put a cropped slice of poster art
there. Sidecars are third-party, so core guards at ingest: **frame follows source aspect, never
config or role name** (`web/src/lib/components/entity/CLAUDE.md`). Guard, don't remove — the
band's fate (destination vs. container page) is deferred at zero cost via the existing
`{#if film.banner_url}` gate (ADR-089 D4 stands).

## Gates — definition of done

- [x] spec `write-spec` — P0-10a added to `docs/specs/film-provider-enrichment-ux.md` (no new spec)
- [x] design `design-handoff` — option C approved 2026-09-16 (PR #337):
  `docs/design/film-banner-landscape-guard-handoff.md` + mockup SVG
- [x] backend — `filmimage.CheckRoleAspect` / `PortraitBannerError` (dimensions already decoded by
  `personimage.Normalize`, so no client-side fallback); `imagesink.storeFilmAsset` refuses;
  `enrich.downloadAssets` returns notes → `recordEnrichJob` appends `· banner skipped: W×H is
  portrait, banner role requires landscape` + `Skipped`; `uploadFilmImage` 400 `banner refused: …`
- [~] frontend — n/a: no UI change; the no-band header is the existing empty state
- [x] testing `testing-strategy` — row added; `TestCheckRoleAspect`, `TestSinkRefusesPortraitFilmBanner`,
  `TestEnrichRecordsRefusedFilmBanner`, `TestFilmImage_BannerRequiresLandscape`; happy-path banner
  upload switched to a landscape fixture (the guard broke it first — proof it is wired)

## Up next — ordered (position = priority)

1. [ ] [—] Handoff QA 1.3 `[human]`: film page in all three skins with a refused banner should be
   pixel-identical to a film that never had one
2. [ ] [—] Stress seeder (HOLODEX-342 family): its film-banner "ratio" rung still stages a 2:3
   banner through `ReplaceFilmImageFile` (below the guard) — decide whether that rung should
   become a landscape-but-not-8:3 aspect now that portrait is unreachable through ingest
3. [ ] [—] Merge #337 first (this branch stacks on it); then this PR retargets to `main`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · implemented, tested
- skills: code-review high --fix, code-review
- handoff: Draft PR open on `HOLODEX-386-film-banner-landscape-guard`, stacked on #337; all four
  packages green; nothing open on the branch except the human skin QA.
