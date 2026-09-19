---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-403
status: in-progress
depends-on: [HOLODEX-400]
release_note: The "Write metadata to file" dialog's Poster row is now a chooser — pick the file's cover art or a provider poster, and a poster you've already written reads as in sync instead of being re-embedded on every write.
---

# HOLODEX-403 · Poster row as an image-tile chooser + ledger-witnessed image sync

Pulled forward by the owner (2026-09-19) because the cockpit (HOLODEX-400) removed the last
checkbox, leaving the poster with no writeback path: a field with nothing to decide is read-only,
and the poster had nothing to decide only because it had no chooser. Started stacked on
`HOLODEX-400-writeback-cockpit`; #359 merged the same hour, so the branch merged `main` and the
Draft PR targets `main` directly.

Two halves: the **tile chooser** (frontend, third chooser shape) and the **sync witness**
(ADR-101: for image fields `in_sync` comes from the newest `file_writebacks` row, because nothing
reads cover art back and a decided poster would otherwise re-embed on every Write).

## Gates — definition of done

- [x] spec `write-spec` — two lines in `docs/specs/field-source-of-truth.md` §Sync state (2026-09-19)
- [x] architecture `architecture` — `docs/architecture/ADR-101-ledger-witnessed-image-sync.md` + index row (2026-09-19)
- [x] design `design-handoff` — `docs/design/writeback-poster-chooser-handoff.md` + `writeback-poster-chooser-mockup.svg` (2026-09-19)
- [x] backend — `resolver.Options.LastWritten` + `ledgerWitness` branch in `replaceMarkers`;
  `repo.LastWrittenValues`; `getMedia` loads it; `TestResolveFields_ImageSyncFromLedger`,
  `TestLastWrittenValues` (2026-09-19)
- [x] frontend — `curation/SourceImageTiles.svelte`; `isCockpitRow` admits `image_url` (+`isImageRow`);
  `rowClass` witness clause keyed on `in_sync`; dialog wires tiles, `entityImage`/`entityImageUploaded`
  props, placeholder + ADR-049 note, thumbnail on the `=` line; both `CLAUDE.md` tables (2026-09-19)
- [x] testing `testing-strategy` — row; Vitest 364 / Go green; live pass incl. the 3-minute real write,
  ledger row, `=` on re-open, decision-only, upload placeholder, keyboard, three skins (2026-09-19)
- [~] security `security-review` — n/a: no new endpoint; the image write path (SSRF allowlist, ADR-039)
  and the decision endpoint are untouched; the new ledger read is a SELECT on the video's own rows,
  served inside the existing detail response

## Up next — ordered (position = priority)

1. [ ] [—] Handoff §8.9 `[human]` — Kevin's look at the tiles in all three skins, then `gh pr ready`
2. [ ] [—] Fold `SourceBadge` onto `SourceChipRow` (carried over from HOLODEX-400's list)
3. [ ] [—] Preview-tool trap: `preview_start` is pinned to the session's launch dir, so a second
   worktree's code is NOT what it serves — the backend can run through a junction (`wt403`) with a
   launch entry whose `cwd` is under the session root, but Vite refuses the realpath; run Vite via
   Bash from the worktree instead. Consider a memory/`reference-holodex-preview-testbeds` update

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 (b) · backend + frontend + tests
- skills: code-review high --fix
- handoff: ADR-101 witness implemented end to end and live-verified (real 21 GB write landed, ledger row,
  `=` on re-open); tiles chooser shipped; two review findings fixed (witness keyed on `in_sync` not
  display; standing manual literal keeps its tile). Draft PR #364 against `main`. Remaining: Kevin's
  §8.9 look → `gh pr ready`.

### 2026-09-19 · gates
- skills: design-handoff (house format), architecture (ADR-101, number via `adr-claims.mjs --reserve`)
- handoff: worktree `HOLODEX-403-poster-chooser` stacked on the cockpit branch; Jira In Progress;
  owner chose ledger-witnessed sync (B) and the upload placeholder; spec/ADR/design landed. #359
  merged mid-session → merged `main`, Draft PR opened against `main`. Next: backend (Up next 1).
