---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-415
status: in-progress
profile: feature
release_note: Replacing a person's poster now shows up on the Media and Film detail Cast grids immediately, instead of the old poster staying pinned in the browser cache.
---

# HOLODEX-415 · Replaced person poster stays stale on the Cast grids

The owner reported stale images after replacing Studio / Film / Person / Media posters. Only the
Person case reproduced: `PeopleGrid` mounted `<PersonPoster>` with no `version`, because the
video-detail `people[]` and `FilmCast` payloads never attached `poster_version` (only `ListPeople`
did). The versionless URL is served `immutable, max-age=1y`, so the first poster the browser ever
saw at that URL is the one it keeps.

## Gates — definition of done

- [~] spec `write-spec` — n/a: bug fix, no behavior change beyond the defect
- [~] design `design-handoff` — n/a: no visual change
- [x] backend — `repo.GetVideo` and `repo.FilmCast` call `attachPersonImageVersions` (one batch
  query each, detail reads only — list cards never draw people images); **media hardening**
  (owner's call): `setThumbnailURL` is now a method that stats `{id}.jpg` / `{id}-poster.jpg` and
  uses the file's mtime (ns) as `?v=` — the token was the *video's* mtime, unchanged by upload /
  regenerate / cover-art, so the grid rode `no-cache` revalidation alone, which `ServeContent`
  resolves at 1s (same-second overwrite → 304, old bytes). Falls back to the video mtime when the
  file can't be stat'd; poster token falls back to the thumbnail's as `servePoster` does.
  `no-cache` header kept as the belt to that brace; the completeness queue's video rows now get
  the same versioned `thumbnail_url` (they shipped empty, so the SPA fell back to the bare route)
- [x] frontend — `PeopleGrid.svelte` passes `version={p.poster_version}` to `PersonPoster`
- [x] testing `testing-strategy` — `TestDetailCastPosterVersion` (0 → id → advances on replace,
  mutation-checked: fails without the fix) + `TestMediaImageURLsVersionOffImageFileMtime`
  (same-second overwrite advances the token; mutation-checked) +
  `TestRemediationQueue_VideoRowThumbnailURL` (mutation-checked); rows added to
  `docs/testing-strategy.md`; live repro on `/media/1` before (red pinned at `?skin=…`) and after
  (`&v=2` blue, `&v=3` green); browse-grid `thumbnail_url` token verified to change on poster upload
- [x] `code-review high --fix` — no findings

## Up next — ordered (position = priority)

1. [ ] [—] Owner to re-check Studio / Film poster replacement on the real instance — neither
   reproduced in dev (`?v={rowid}` + immutable). If they recur, capture page + environment (reverse
   proxy? DB restored from backup — ids restart and an old `?v=N` collides, the HOLODEX-411 testbed
   gotcha) and file separately.
2. [ ] [—] On merge, HOLODEX-415 → Done via CI (branch-keyed)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-18 · diagnosed, reproduced, fixed
- skills: code-review high --fix (no findings), code-review
- Explore sweep mapped all four image kinds' serve headers / URL versions / update paths. Studio
  logo upload verified live (`v=1` → `v=2`, pixel flipped); video poster upload verified on the
  browse grid (`no-cache` revalidated). Person poster reproduced stale on the media Cast grid, fixed
  at the two payload sites + the grid prop, pinned by a repo test.
- Owner: "harden the media path too" → image-file-mtime `?v=` token (second push on #353); then
  "fix the completeness queue thumbnail_url too" → `setThumbnailURL` per queue video (third push;
  live: all 138 video rows versioned).
- handoff: person fix + media hardening shipped and live-verified; PR #353 ready for review (Jira
  In Review via CI); Studio / Film await the owner's re-check on the real instance.
