---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Copy to <worklog.dir>/<KEY>.md (SessionStart scaffolds this automatically if missing).
# Schema + design: see the Flightplan plugin's own README and ADR-001 (in the plugin repo).
key: HOLODEX-602                 # the tracker key; must match the branch key regex
status: in-progress                 # DERIVED from the Gates below — nothing settled is todo, some movement
                             # is in-progress, all settled with a release_note is in-review. Only
                             # `done` and `released` are read from here (a merge and a release are
                             # facts the checklist can't see). Any other value is ignored, so this
                             # field cannot drift. If the status looks wrong, a gate is wrong.
profile: full                # the gate posture (see flightplan.yaml `postures:`). Which gates this
                             # epic HAS — a judgment, so no hook sets it. SessionStart prompts every
                             # session until it does, and the Gates rows below are matched to it.
depends-on: []               # [KEY-…] cross-epic deps that must land first
release_note: The owner can have Holodex rename media files from an ordered list of templates, with a preview first, names valid on Windows and Linux, stale-name review, and undo.  # the ONE user-facing sentence; authored once by /handoff, flows to the
                             # Release-Note: git trailer → release notes. An epic can't close with all
                             # gates [x] but this empty.
# approved:                  # the owner's sign-offs on `approve: true` gates (ADR-007). [x] means the
#   design:                  # artifact is committed; THIS means the owner looked and said yes. Written
#     on: 2026-09-20         # only by /implement (which asks) or /handoff (for a yes given this session);
#     at: 3f2a9c1            # `at` is the commit the yes was given against — change the artifact after it
                             # and the sign-off is stale: re-confirmed with the diff, never revoked.
approved:
  design:
    on: 2026-11-01
    at: 86dfb519
---

# HOLODEX-602 · Rename media files from a template (F86)

Done means the owner can rename library files in place from an ordered template list, through the
progressive plan → manual → writeback → job modes, without a video ever losing its identity or a
file ever being overwritten.

**Design package:** [spec F86](../specs/file-rename.md) · [architecture](../architecture/file-rename.md) · [handoff](../design/file-rename-handoff.md) · [testing-strategy §39](../testing-strategy.md#39-rename-media-files-from-a-template-f86-holodex-602)   <!-- links; source of truth for *what*; this file is source of truth for *where it stands* -->

## Gates — definition of done

<!-- These rows are GENERATED at scaffold time from flightplan.yaml `gates:`, filtered to the
     `postures:` default; an empty roster leaves no rows at all, and status then comes from the
     release note alone. The rows below are only the unscaffolded template's. Once `profile:` is set,
     match them to that posture, adding rows as well as removing them. States: [ ] not started · [/] in progress · [~] deliberately
     skipped · [x] done. [~] and [x] both count as SETTLED — an epic can reach a full count with a
     documented skip (ADR-004). PostToolUse(Skill) flips a gate to [/] when its skill runs; ONLY
     /handoff writes [~] or [x]. -->

- [x] spec `write-spec` → `docs/specs/**`
- [x] architecture `architecture` → `docs/architecture/**`
- [x] design `design-handoff` → `docs/design/**`
- [x] backend → `{cmd,internal,providers}/**`. Plan (`acdf164d`) and Manual (`da951199`, `991628a4`, `98a99eae`) are built and tested against §39. On-writeback and Job are a later slice, blocked on the owner's watch-state test (Up next 5–6).
- [x] frontend → `web/src/**`. Plan and Manual are built in `9b3b546d`: the File names tab (Renaming card and plan), the video page's File name row, and the `rename/` components. Three plan fields were added for it in `c9112f31`: lock source and date, providers, and read-back values. §39.6 W1–W7 pass, and §39.7's `[agent]` items 1, 2, 5, 6, 7, 11–13 and 18 pass in the browser. The owner compared the build with the three approved mockups side by side and said it matches (2026-11-01).
- [x] testing `testing-strategy`
- [x] security `security-review`. The design review (2026-10-31, `c25e21e4`) found no hole in the designed perimeter's intent, but nine gaps in how it was written. Each is closed in the doc that owns it: the template text and the whole name now face the character checks; the Windows reserved set is complete and judged before the first dot; invisible and invalid characters are removed or refused; the Linux root check is bound to a folder descriptor; the fallback `Lstat`s the target; the via name is built from the id; identity read-back is bounded; name state and job errors stay owner-only; and the apply body is capped. The tests are F7, F8, A8, A9 and §39.11. The implementation review (2026-11-01, `origin/main...66cbe9e2`, Plan and Manual) found one gap. A provider resolve carried every provider's identity id read from the file name, which told provider A which other providers the owner uses; that is fixed in `e419ff5a` with a test. Everything else holds. One builder runs `CheckName` on the whole name, and the worker re-plans and refuses a changed name. On Linux every rename goes through an opened folder descriptor whose `/proc` path is checked against the library root. Symlinks are refused on both sides. `RENAME_NOREPLACE` falls back to the folded listing, then `Lstat` plus `SameFile`. The via name is `hx-<id>`. Every rename and settings route is in the `requireOwner` group, and A1 walks them. The apply body takes ids only, capped at 1 MB. The settings validator gates every write. No rename field reaches `/media` or MCP. Activity rows carry counts, and full errors go only to owner reads and the operator log. Below the bar: a provider that opts into filename hints still sees the whole file name, other providers' markers included, which the operator's `send_filename` deny governs. Settled for Plan and Manual. `until:` On-writeback or Job lands, which adds a rename path (Up next 6).

<!-- Deliberate-skip example — always say why; `until:` records what would reopen the concern later
     (as a fresh up-next item or its own issue — the gate itself stays settled):
- [~] security `security-review` — until: a mutation endpoint exists (read-only slice so far) -->

## Up next — ordered (position = priority)

<!-- Numbered queue. Position is the priority — no P1/P2 tags. Each item: [gate] one-liner — file path.
     ⛔ marks blocked (say on what). → KEY promotes a separable item to its own issue.
     The top item is surfaced in the SessionStart banner, clipped if long (the trailing path
     survives; the middle is dropped) — so keep it to one line.
     The CHECKBOX settles an item and nothing else does: [x] done, [~] dropped, same states as a gate
     row. A strikethrough is emphasis, not a state — `~~half~~ now do the rest` is a live item
     (ADR-008). This queue holds LIVE work only: delete a done item, move a dropped one to
     ## Dropped at the end of this file (ADR-010). The banner counts any settled item left here. -->

1. [ ] [testing] Confirm F1/F8 (Linux-only primitive tests, F8 is the folder-swap security test) pass in CI; the Windows dev box can't run them — `internal/rename/fs_linux_test.go`
2. [ ] [—] Owner test: rename one file by hand, check the media server's watch state; gates On-writeback/Job modes only — spec Open Questions
3. [ ] [backend] On-writeback mode: rename step inside the write job, after WriteBatch, before PostWriteFunc (R23) ⛔ blocked on item 2 — `internal/writequeue/`
4. [ ] [backend] Job mode: settle-window sweep over `video_file_names.desired_*` (R14, sweep half of R24) ⛔ blocked on item 2 — `internal/rename/`
5. [ ] [security] `/security-review` of the On-writeback and Job diffs when items 3–4 land — `docs/architecture/security-perimeter.md`
6. [ ] [—] Decide whether HOLODEX-192 (old rename-schema placeholder, now Relates-linked) closes as a duplicate of this epic
7. [ ] [—] `TestSyncTagWritebackBulk_DedupsSharedVideo` fails on this Windows box at the pre-feature commit too (3 s drain deadline); not this epic's — `internal/api/tag_writeback_sync_test.go`
8. [ ] [backend] After Undo then Unlock, the plan calls the video out of date with every field "changed" (spec question) → HOLODEX-618
9. [ ] [backend] Stale-plan failure as a reason code on `/rename/status`, not error-text matching → HOLODEX-616
10. [ ] [backend] Ids-only plan for Select all; a `video_id` plan that resolves only that folder → HOLODEX-617

## Session log — append-only (cap: last 8 sessions; older → archive/)

<!-- One entry per session, newest at the top. PostToolUse(Skill) creates the entry + appends the
     `- skills:` line mechanically; /handoff writes the `- handoff:` sentence the next SessionStart
     banner echoes. Shape:
### 2026-07-10 · what happened this session
- skills: write-spec, architecture
- handoff: the sentence the next session should wake up to
-->

### 2026-11-10 · {film} token
- skills: code-review
- handoff: Templates take a `{film}` token, which renders the first linked film (by name, in its display spelling) as a `{film-<name>}` marker that reads back but never makes a film link (spec P0-1, testing N22), with a `{film}` chip in the editor; the design handoff's chip list doesn't name it yet, left alone so the design sign-off stays current.

### 2026-11-01 · frontend gate (Plan and Manual)
- skills: code-review, handoff
- handoff: Plan and Manual are complete: the File names tab and the video page's File name row are built (`9b3b546d`, plan fields in `c9112f31`), the owner said the build matches the mockups and re-pinned the design at `86dfb519`, so every gate is settled and PR #575 is marked ready; next is Up next 1 (F1/F8 in CI), with On-writeback and Job still waiting on the owner's watch-state test and three backend follow-ups filed (HOLODEX-616/617/618).

### 2026-11-01 · security review (implementation)
- skills: code-review, handoff
- handoff: The security gate is settled for Plan and Manual. The backend review found one leak: a provider lookup carried other providers' ids read from the file name. It is fixed in `e419ff5a`, and every other perimeter promise holds in code. `needs-security-review` is cleared. Next is the frontend (item 1). On-writeback and Job get their own security pass when they land (item 6).

### 2026-11-01 · jargon doc
- skills: code-review, handoff
- handoff: Started `docs/reference/jargon.md` (first entry: the fold) and a CLAUDE.md rule to explain jargon on first use; the fold-vs-owner-lock call (Up next 7) is still waiting on the owner's answer. Next is the backend security review.

### 2026-11-01 · fold keeps the name lock
- skills: code-review
- handoff: With the owner's yes, a fold now merges the name lock (locked if either side was; owner/undo outrank auto; a tie keeps newer's) while newer's settle state still wins, in file-rename.md and `FoldMovedVideo`. Next is the backend security review (Up next 1), then the frontend.

### 2026-11-01 · Manual gaps closed
- skills: code-review, handoff
- handoff: `backend` is `[x]` for Plan and Manual: apply now writes one batch summary that the last job completes with renamed/failed counts, a fold-healed hand edit locks (`991628a4`), and R12 identity plus undo-of-undo are pinned (`98a99eae`). On-writeback and Job wait on the owner's test (Up next 4). Next is the backend security review, then the frontend.

### 2026-11-01 · spec wording fixes
- skills: code-review
- handoff: With the owner's yes, the spec's P0-3 `]` example now says only `] ` refuses, and P0-1 says rename refuses unknown tokens though F48 accepts them; next is Up next 1, the Manual test gaps.

### 2026-11-01 · backend Plan + Manual
- skills: code-review, handoff
- handoff: The backend for Plan (`acdf164d`) and Manual (`da951199`) is in: migration 0068, the settings allowlist with the rename keys, builder and read-back, collision plan, plan/preview/apply/undo/lock routes, the no-overwrite primitive, the journalled worker with boot recovery, scanner and echo integration. `backend` stays `[/]` on the R12/R24 gaps and the blocked On-writeback/Job modes. Next is Up next 1, then the backend security review.

### 2026-11-01 · session
- skills: code-review, handoff
- handoff: The collision disambiguator is now ` {hx-<id>}` (braces, matching the marker grammar) across spec, arch, design and testing (`84362775`), and the whole-name check says braces are the builder's own; the owner re-confirmed the design at 84362775. Next is the backend: Plan, then Manual.

## Dropped — newest first (the reason is the point)

<!-- Up-next items decided AGAINST, moved here by /handoff when dropped (ADR-010). Done items are
     deleted instead — git and the session log already record them; a dropped item has no other
     record, so its reason lives here. Shape:
- [~] [testing] <thing you decided not to do> — dropped 2026-07-29, <why>
-->
