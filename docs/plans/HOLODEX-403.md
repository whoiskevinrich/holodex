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
- [ ] backend — `resolver.Options.LastWritten` + `replaceMarkers` image branch; `repo.LastWrittenValues`;
  `resolveOptions` loads it for the video detail; tests
- [ ] frontend — `curation/SourceImageTiles.svelte`; `isCockpitRow` admits `image_url`;
  `needsDecision` stops excluding it; dialog wires tiles + upload placeholder/note; `curation/CLAUDE.md`
- [ ] testing `testing-strategy` — row; `writebackCockpit.test.ts` image cases; Go tests above; live pass
- [~] security `security-review` — n/a expected: no new endpoint; the image write path (SSRF allowlist,
  ADR-039) is untouched; the ledger read is owner-gated with the detail it rides on — re-check at PR

## Up next — ordered (position = priority)

1. [ ] [backend] ADR-101 D1/D2 — `LastWrittenValues` (newest per `field_key`), `Options.LastWritten`,
   `replaceMarkers` image branch, resolver tests
2. [ ] [frontend] `SourceImageTiles.svelte` (chip-row keyboard handler verbatim), dialog wiring,
   `isCockpitRow`/`needsDecision` admit `image_url`, upload placeholder + note
3. [ ] [testing] tests + live pass on `backend-films` (Dune): pick tmdb → PUT + writeback; re-open → `=`
4. [ ] [—] `/code-review high --fix`; three-skin QA
5. [ ] [—] Fold `SourceBadge` onto `SourceChipRow` (carried over from HOLODEX-400's list)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-19 · gates
- skills: design-handoff (house format), architecture (ADR-101, number via `adr-claims.mjs --reserve`)
- handoff: worktree `HOLODEX-403-poster-chooser` stacked on the cockpit branch; Jira In Progress;
  owner chose ledger-witnessed sync (B) and the upload placeholder; spec/ADR/design landed. #359
  merged mid-session → merged `main`, Draft PR opened against `main`. Next: backend (Up next 1).
