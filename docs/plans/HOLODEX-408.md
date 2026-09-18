---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-408
status: in-progress
release_note: People tagged in a file's Artist/Cast/Actor/Performer/Director tags now link on first import, and a comma-separated list always becomes separate people even when the mapping omits `multi: true`.
---

# HOLODEX-408 · Embedded person tags link on first import (+ HOLODEX-409)

Two bugs found from one report ("a file with `person one, person two` imports as one person"),
both on the ADR-072 derivation path and fixed together on this branch:

- **HOLODEX-408** — `mapExiftool` routed `peopleKeys` tags into `ex.People` and never into
  `Extra`, so `file:Artist` had nothing to read in `video_metadata`; since ADR-072 made the
  resolved field the sole `video_people` writer, embedded-tag people linked **nothing** on a fresh
  import. Fix: person tags also land in `Extra` under their canonical key.
- **HOLODEX-409** — a person-typed field without `multi: true` took the resolver's replace branch
  and derived `"A, B"` as one person (the reported symptom, via the filename `{people}` path).
  Fix: the mapping loader forces `multi: true` on `entity: person` fields.

Spec: [person-media-linking.md](../specs/person-media-linking.md) RD9 addendum (d)/(e).

## Gates — definition of done

- [x] spec `write-spec` — RD9 addendum (d)/(e) + corrected writeback round-trip read half
- [~] architecture `architecture` — n/a: restores ADR-072's stated lossless-cutover constraint; no new decision
- [~] design `design-handoff` — n/a: no UI change (the Actors row now simply resolves from the file)
- [x] backend — `internal/metadata/extractor.go` (Extra append) · `internal/mapping/mapping.go` (forced multi)
- [~] frontend — n/a
- [x] testing `testing-strategy` — new cardinal row under F40; `TestFirstImportLinksPeopleFromEmbeddedTags` (real scanner → repo → `RelinkVideoEntity` over `mapExiftool` output), mutation-tested both ways; extractor unit tests updated
- [~] security `security-review` — n/a: no auth/access/infra change
- [x] code-review `code-review high --fix` — 2 findings: reach of the fix on already-indexed files → spun off as HOLODEX-410; worklog wording fixed

## Up next — ordered (position = priority)

1. [x] [—] `/code-review high --fix`, commit, push, open PR (ready — all gates green)
2. [ ] [—] On merge: sweep **HOLODEX-409** to Done by hand (CI transitions only the branch key, 408)
3. [ ] [—] **HOLODEX-410** — files first indexed between 2026-08-05 (PR #212) and this deploy still have no embedded-tag people: a rescan skips unchanged files, so only a per-item owner **Refresh** re-extracts them today. Decide a one-time re-extract backfill vs. bulk Refresh (options in the ticket). Nothing before the cutover is affected.

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-17 · debug → repro → fix
- skills: debug, code-review
- handoff: reproduced on a scratch fixture (tag with/without space, filename with/without space)
  under `backend-repro` (local launch.json, scratchpad media/config); all four variants link
  correctly post-fix with `multi: true` absent. 408 + 409 filed and In Progress, branch
  `HOLODEX-408-file-tag-people-link`; 410 (backfill for already-indexed files) filed from the
  code review. Full `go test ./...` green. Next: PR.
