# ADR-101: An image field's sync state is witnessed by the write ledger, not read back from the file

**Status:** Proposed
**Date:** 2026-09-19
**Deciders:** Project owner

**Extends:** [ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md) (tri-state `in_sync`;
this adds a second witness for the one field kind its read-back can never serve) ·
[ADR-041](ADR-041-metadata-writeback.md) (the `file_writebacks` audit row this promotes to a
witness) · [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (image fields carry the same
per-field decision as every replace field — no new seam) · [ADR-049](ADR-049-manual-image-precedence.md)
(an owner upload keeps the page; this ADR is about the file's cover art only).
**Issue:** [HOLODEX-403](https://whoiskevinrich.atlassian.net/browse/HOLODEX-403) (parent
HOLODEX-167; follows the writeback cockpit, HOLODEX-400).

---

## Context

The writeback dialog became the cockpit for the golden record (HOLODEX-400): every replace field
gets its chooser, the gutter names the destination Write will touch, and a standing decision that
lags the file is always written. Image fields (`poster_url`, `Display: image_url`) were left out —
read-only, never written from the dialog — because two things were missing:

1. **A chooser.** The decisions API already accepts `poster_url` (`internal/api/decisions.go`
   refuses only `Multi || Merge`) and the resolver honours a standing decision on it
   (`resolveDecided`), so the *decision* needs no new seam. What was missing was a control: the
   media page keeps `poster_url` in `METADATA_ELSEWHERE` (no `SourceBadge`), and HOLODEX-320
   declined a provider-poster chooser as a new surface. Inside the cockpit it is one row.

2. **A sync witness.** ADR-093 computes `in_sync` as `decided == fileVal`, where `fileVal` is read
   back through the mapping's `file:` source. Nothing reads embedded cover art back:
   `internal/writeback/snapshot.go` skips `IsImage`, `readback.go` scopes gaps to text fields, and
   even a declared `file:Cover` source yields a marker, never the URL that was written. So an
   image field's `in_sync` is `nil` with a decision and can never become `true`. Under the
   cockpit's rule that is not a cosmetic gap: a decided poster would **lead the dialog on every
   open and be downloaded and re-embedded on every Write** — a full remux on MP4.

The ledger already knows the answer. `file_writebacks` (ADR-041) records `(video_id, field_key,
tag_name, value, source, written_at)` for every *successful* write, and for an image field
`value` is the exact `https://` URL that was downloaded and embedded (`internal/api/writeback.go`,
`internal/writequeue/writequeue.go`). Whether the file carries *this* poster is therefore a
question the ledger can answer and the file cannot cheaply.

## Decision

**D1 — For image fields, `in_sync` is witnessed by the last successful write, not by read-back.**
In `replaceMarkers`, when the field's display is `image_url` and a standing decision exists:
`in_sync = true` iff the newest `file_writebacks` row for `(video_id, field_key)` has `value ==
decided`; `false` iff such a row exists with a different value; **`nil` (unknown) when no row
exists** — the ADR-093 tri-state, with the ledger as the witness. Text fields are untouched:
their witness stays the file read-back, because for them the file is cheap to read and the ledger
could be stale after an external edit.

**D2 — The witness rides `resolver.Options`, loaded by the caller.** `Options.LastWritten
map[string]string` (canonical → last successfully written value) is filled by a new
`repo.LastWrittenValues(ctx, videoID)` (`MAX(written_at)` per `field_key`, one query) and passed
by the one API seam that already builds `Options` (`resolveOptions`). The resolver stays pure:
the ledger is an input, exactly as decisions are.

**D3 — Image fields join the cockpit as ordinary rows.** `poster_url` is a cockpit row: the same
staged pick, the same `willWrite` / `savesDecisionOnly` gates, the same gutter. Its chooser is
**image tiles** (a third chooser shape beside the chip row and the stacked rows): one tile per
candidate — the file's cover art (`·file`) and each provider poster — no Custom tile, because a
pasted URL would have to pass the provider asset-host allowlist (ADR-039) and an owner upload
already has its own control. Picking a provider tile is the confirm; Write records the decision
and enqueues the existing download + embed. Picking `·file` is a decision-only save.

**D4 — An owner upload is not a candidate and is not disturbed.** ADR-049 stands: the uploaded
poster keeps the page (`poster_uploaded`), and this row is the cover art *inside the file*. When
an upload is present the file tile is a placeholder (the upload overwrites the extracted tier on
disk, so the file's own cover has no image to show) with a one-line note. Writing the upload
itself into the file (a local file, not an `https://` URL) is out of scope.

## Consequences

- After one Write, a decided poster reads `=` on the next open and stays quiet until the winner
  changes; the "re-embed forever" trap is closed by construction.
- A poster written **before** this ADR has a ledger row already (ADR-041 audit rows predate it),
  so existing libraries get the witness for free; a poster whose write predates ADR-041's audit
  reads unknown, not false.
- The witness is only as good as the file: an external tool replacing the cover art is invisible
  to the ledger. That is the same trade ADR-093 accepted for a *missing* read-back, and it is the
  right trade for the one field kind the file cannot answer cheaply. A future content hash of the
  embedded attachment could supersede D1 without touching D2–D4.
- No migration: the ledger table is unchanged; one new read query.
- Not covered: writing an owner-uploaded poster into the file; frame-grab as a candidate (it is a
  local file, not a URL); a content-hash witness. Each is a separate story.

## Alternatives considered

- **Read the cover art back and compare bytes** — the honest witness, but it means downloading
  the provider image again on every resolve (or caching bytes/hashes per candidate) to compare
  against a remuxed attachment. Deferred; D1 leaves the door open.
- **Track sync on the decision row** (`written_value` on `field_source_decisions`) — duplicates
  what the ledger already records and would need a migration; rejected.
- **Keep images read-only in the dialog** — leaves the poster with no writeback path at all once
  the cockpit removed the opt-in checkbox (HOLODEX-400), which is what forced this story.
