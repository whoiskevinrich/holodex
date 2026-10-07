# ADR-119: A text field writeback cannot read back is witnessed by the write ledger, and the gap is shown to the owner

> **Archived — not current truth.** This numbered ADR was retired by HOLODEX-528. Its architecture now lives in [writeback.md](../writeback.md). Read those; this file is kept only as history.


**Status:** Proposed
**Date:** 2026-09-28
**Deciders:** Project owner

**Extends:** [ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md) (tri-state `in_sync`;
D5's startup WARN) · [ADR-101](ADR-101-ledger-witnessed-image-sync.md) (the write-ledger witness,
until now for image fields only) · [ADR-041](ADR-041-metadata-writeback.md) (`file_writebacks`).
Rejects an implicit read-back source under [ADR-013](ADR-013-metadata-field-mapping.md).
**Issue:** [HOLODEX-489](https://whoiskevinrich.atlassian.net/browse/HOLODEX-489) (relates HOLODEX-488).

---

## Context

ADR-093 made `in_sync` tri-state. A decided field whose mapping declares no `file:` source has
nothing to compare against, so its sync state is **unknown** (`in_sync` omitted). The writeback
dialog (HOLODEX-400) then leads every decided unknown row: "a decided value the file does not carry
is written". That rule is right for a field that has never been written. It is wrong once Holodex has
written it. Every later dialog open writes the same value again, which is a full-file copy and rename
that changes nothing. To the owner the item looks permanently out of sync.

A production report showed this for `title` (→ `Title`) and `release_date` (→ `Year`), mapped only to
provider and filename sources. The only signal was ADR-093 D5's startup WARN
(`writeback.LogReadbackGaps`), which is gone from view by the time anyone looks at the stuck row.

Two facts shape the fix:

1. **The ledger already witnesses what the file cannot answer.** ADR-101 compares an image field's
   decided value against the newest successful `file_writebacks` row (`repo.LastWrittenValues`,
   loaded on the detail read). That row stores the **canonical** value Holodex wrote, for example
   `2024-02-27` for `release_date`, even though the file got `Year=2024`. So it compares
   like-for-like with the decided value for a text field too.
2. **"Has a file source" has two definitions.** `resolver.baselineValue` counts *any* declared
   `file:` source. `writeback.ReadbackGaps` counts only a `file:` source whose key matches the tag
   writeback writes. `overview: [Description, tmdb:overview]` passes the first and fails the second.
   The WARN fires, the resolver compares against `Description`, and `in_sync` reads **false** for ever
   after a successful write of `Comment`. That is the same stuck row, in a worse form.

## Decision

**D1: a read-back-gap text field is witnessed by the write ledger.** For a replace field that
`writeback.ReadbackGaps` reports for the live mapping, with a standing decision, `in_sync` is:

- `true` when the newest successful `file_writebacks` row for the field holds the decided value;
- `false` when it holds a different value (the decision moved after the last write);
- unknown when there is no row (never written by Holodex).

This is ADR-101's tri-state with the same witness. Image fields keep ADR-101 unchanged.

**D2: the gap set is the one definition.** The resolver does not work out "unreadable" for itself. The
API computes `ReadbackGaps(fields)` for the mapping it resolves with and passes the canonical set in
`resolver.Options`, next to `LastWritten`. For a field in that set, the ledger is the witness even if
some other `file:` source is declared. The file candidate chip still shows whatever that source reads,
because it is a real value on the file, but it no longer decides sync. The WARN and the witness can
no longer disagree.

**D3: the trade-off is ADR-101's, accepted.** The ledger knows what Holodex wrote, not what is in the
file now. An edit to the file made outside Holodex, or a file replaced on disk, is not seen until the
mapping gains a read-back source. The owner accepts this. The mapping fix stays the way to get a
genuine file comparison, and D4 points to it.

**D4: the gap is shown where the owner looks.**

- `GET /api/v1/owner/readback-gaps` (owner-gated) returns `[{canonical, write_tag, add_one_of[]}]`
  for the live mapping. `POST /api/v1/admin/reload-config` adds `readback_gaps` (the count) to its response.
  On the owner's media detail read, each resolved field in the gap set carries
  `readback_gap: {write_tag, add_one_of[]}`, so the dialog needs no extra request.
- System Activity shows a **Mapping checks** block beside *Reload config* that lists each gap and
  the key to add. The reload toast reports the count.
- In the writeback dialog, a decided row that is still unknown (a gap with no ledger row) carries one
  `text-muted` hint line under its chooser. It names the key to add and says the row will be tracked
  from Holodex's own writes after this write. **This narrows the HOLODEX-400 rule** (no warning line
  on a not-read-back row) for this case only. The line is setup guidance, not a warning: `text-muted`,
  never `--warn`, and it disappears after the first write.

**D5: no implicit read-back source.** Treating the written tag as an implicit lowest-priority `file:`
source was considered and rejected. ADR-013 keeps the mapping explicit, and adding a file source
changes precedence. Under the file-first default, a bare `YEAR 2024` would outrank the provider's full
date (HOLODEX-335's lesson).

## Consequences

- A title or release date that has been written stops rewriting on every dialog open and reads `=`.
  The frontend needs no change for this: `rowClass` already treats a standing `in_sync: true` as
  `matches` (ADR-101).
- `in_sync: false` becomes possible for a gap field (the decision moved after the last write), and it
  counts in the page's out-of-sync pill. That is correct: the file really does lag.
- The detail handler computes `ReadbackGaps` for each detail read. It is a pure pass over the field
  list and costs nothing next to the resolve; it can be cached with the mapping if that ever matters.
- An owner who adds the read-back source later moves the field from ledger witness to file
  comparison with no migration. D2's set simply stops naming it.

## Rejected alternatives

- **Implicit read-back source.** See D5.
- **Stop leading unknown rows and leave them unknown.** This fixes the rewrite loop, but a written
  field never settles and the owner still has to guess.
- **Fix `baselineValue` to key-match instead of D2.** That puts the write-target table into the pure
  resolver, which today knows nothing about containers. D2 keeps that knowledge in `internal/writeback`,
  where `ReadbackGaps` already lives.
