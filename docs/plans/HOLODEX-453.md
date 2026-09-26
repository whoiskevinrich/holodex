---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-453
status: in-progress
approved:
  design:
    on: 2026-09-25
    at: f03774d
release_note: The Duplicates queue no longer fills up with pairs whose only link is a provider-supplied alternate name. The Aliases panel now says which person or studio already holds a skipped name, and links to them.
---

# HOLODEX-453 · Provider-alias pairs leave the Duplicates queue

The person half of the Duplicates queue was entirely `provider-alias` pairs: 31 of 31 open,
against 185 dismissed keep-separate. On his hands-on pass through the F70 compare panel, Kevin found
**none that was actually the same person** (2026-09-25). He chose to stop surfacing them.

## Decisions — do not re-litigate

- **Hide at the read; keep writing the row.** ADR-108 D1/D2. The row is the skip record behind
  `skipped_aliases`, and it is the row ADR-107 upgrades to `shared-external-id` in place. So
  "stop flagging" (the ticket's option 1 as filed) would have deleted the Aliases panel line and
  that upgrade path. Kevin chose the hide over "stop writing + a new skip table" and over "stop
  writing, drop the line" (2026-09-25).
- **Every `provider-alias` row, not an "alias-to-alias" subset.** The ticket's "31 of 31 alias-only"
  is a probe artifact: `scripts/detect_person_duplicate_evidence.sql`'s `match_kind` is an `ELSE
  'alias'` fallthrough, and the refused name is never written to the refused entity, so a
  provider-alias pair could never classify as anything else. Whether the holder held the name
  canonically or as an alias was never measured, and `entityConflict` doesn't record it.
- **The skipped line names and links the holder** (option A of the mockup), replacing the
  `Review` → `/owner/duplicates` link. Kevin chose A 2026-09-25, over dropping the link and over
  removing the line.
- **`flagNearMissForName` is not the seam.** The ticket named it, but it writes only
  whitespace/punctuation rows. The provider-alias producer is `queueProviderAliasPair`
  (`internal/repo/provider_aliases.go`), and it is left unchanged.

## Gates — definition of done

- [x] spec `write-spec` — F58 amended in place: RD4 amendment note, **P0-5a**, API + UI notes in
      `docs/specs/provider-alias-collapse.md`
- [x] architecture `architecture` — [ADR-108](../architecture/ADR-108-provider-alias-collisions-leave-the-duplicates-queue.md);
      ADR-088's status marks D5's enqueue half superseded; index row added
- [x] design `design-handoff` — `docs/design/provider-alias-skipped-line-handoff.md` + committed
      `provider-alias-skipped-line-mockup.svg` (text extents measured in the browser: every line
      fits its 592 px box). **Option A chosen by Kevin 2026-09-25; sign-off on the artifact is
      `/implement`'s**
- [x] backend — `ListReviewPairs` excludes `provider-alias`; `SkippedAlias.ConflictName` joined
      in `SkippedAliasesForEntity`. Tests: `TestReviewQueue_ProviderAliasRowsAreNotListed`,
      `TestProviderAliasRowSurfacesOnceUpgraded`, `conflict_name` on both collision routes, and
      the sort test now pins the -2 slot on film (no other person non-fuzzy variation is
      listed). The three queue tests fail with the filter removed (mutation-checked)
- [ ] frontend — `AliasPanel.svelte` skipped line; `SkippedAlias.conflict_name` in `types.ts`
- [ ] testing `testing-strategy`
- [ ] security `security-review` — read-path filter + one extra owner-gated field; expect a short
      sign-off

## Up next — ordered (position = priority)

1. ~~`/implement HOLODEX-453`~~ **Done 2026-09-25** — design signed off at `f03774d`.
2. ~~Backend.~~ **Done 2026-09-25** — see the backend gate.
3. Frontend: `AliasPanel.svelte` line per the handoff, then QA all three skins at 375 px.
4. Testing + security gates, then `gh pr ready`.
5. Optional: fix the probe's `qmatch` so a provider-alias row reports the holder's side from
   `detail`, instead of falling through to `alias`.

## Session log — newest first (cap: last 8 sessions; older → archive/)

### 2026-09-25 — the question is decided; design phase done

- skills: architecture, write-spec (amendment), design-handoff, implement, code-review

Confirmed the ticket was untouched on `main`: 451 and 452 merged, but nothing changed the
provider-alias producer or the queue read. Kevin leaned to option 1. The trace found that option 1
at the producer would remove the Aliases panel's skipped line, which reads the same rows, and that
the "alias-only" figure was a probe fallthrough. He chose to hide the rows at the read and to link
the holder on the panel.

Crossed into build (design signed off at `f03774d`, Draft PR #386), then landed the backend. The
code review's one real gap was the sort test: with provider-alias hidden, person no longer lists
any other non-fuzzy variation, so the shared-external-id -2 slot is now pinned against film
`same-title` instead.

- handoff: **Backend is in and green; frontend is next** (Up next 3). `AliasPanel.svelte` reads
  `skipped_aliases[].conflict_name` (add it to `SkippedAlias` in `types.ts`) and links the holder
  per the handoff; QA all three skins at 375 px.
