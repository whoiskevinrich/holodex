# ADR-120: The owner can clear a replace field — a manual decision with no value, and writeback removes the tag

**Status:** Proposed
**Date:** 2026-09-29
**Deciders:** Project owner

**Extends:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (a manual decision may carry
no value, by explicit request) and [ADR-110](ADR-110-tag-writeback-file-contract.md) (the tag
`Delete` write, until now reserved for genres/tag keys, gets a second producer). **Relates to:**
[ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) (`video_studios` derived from the
resolved studio, prune-on-empty),
[ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md) (`in_sync`),
[ADR-113](ADR-113-owner-offered-empty-fields.md) (the resolver's empty-field drop and its
exceptions), [ADR-117](ADR-117-bundle-mkvtoolnix-runtime.md) (MKV in-place writes).
HOLODEX-493 (first adopter: `studio`) ·
[studio-detach-handoff.md](../design/studio-detach-handoff.md).

---

## Context

**The use case.** File parsing marks a value as the studio that isn't one: a filename token, a
publisher, or a mis-mapped tag. The owner wants to **clear** it. It shouldn't be replaced with
another studio, and nothing should be written *in its place*: the video should simply have no
studio, in Holodex and then in the file. The approved UI is an attached chip in `StudioPicker`
whose `×` detaches (HOLODEX-493 handoff).

Nothing below the UI can express that yet:

- **The API refuses it.** `PUT /media/{id}/fields/{canonical}/decision` with `source: manual` and
  an empty `manual_value` is a 400 (`internal/api/decisions.go:43-58`). The refusal is deliberate:
  an empty Custom submit is far more often a slip than an intent.
- **`DELETE …/decision` is the opposite.** It removes the decision, so the field falls back to the
  file value, which is the mis-parsed studio.
- **Writeback cannot remove a replace field's tag.** `writeback.FieldWrite.Delete` exists
  (ADR-110), but its only producer is `writequeue.buildBatch` for `genres` and tag-key fields. Any
  other field whose values sanitize to empty is silently skipped (`writequeue.go:490-496`). The
  HTTP handler 400s a whole batch on an entry with no values (`internal/api/writeback.go:271`).
  `Revert` says outright that the write path has no "clear this tag" primitive today.

Most of the machinery for an empty *outcome* already exists, because a pin to a layer that happens
to be empty has been legal since F37:

- `resolveDecided`'s manual branch already returns no item for an empty `manual_value`
  (`resolver.go:579-583`). `ResolveFields` keeps a replace field with a **standing** decision even
  when its value is empty (`resolver.go:349-368`, "blank-pin … F37 RD3"), so the decision marker is
  still reported and the owner keeps a control.
- `in_sync` compares the decided value against the file's (`resolver.go:814-823`). An empty decision
  against a file value is already `false`, which is what the writeback cockpit needs.
- `ReconcileVideoStudios` with no names removes the video's links (`internal/repo/studios.go:33-43`),
  and the studio decision path already calls it (`decideStudioForVideo` → `relinkStudiosWithContext`).
- The repo stores `manual_value` verbatim (`internal/repo/decisions.go`), and the column is
  `NOT NULL DEFAULT ''` (migration 0016).
- The HOLODEX-270 composite-key collision gate keys an empty studio as `""`, so the gate runs for a
  clear too (`internal/repo/video_collision.go:315`).

What's missing is a *deliberate* way to record "the owner decided: no value", and a write that
removes the tag.

## Decision

### D1. A cleared field is a manual decision with no value, recorded only by an explicit `clear`

The decision grammar does **not** change. The owner is the source (`manual`) and the value is
empty. "No value" is a value, not a new source.

- **API:** the decision PUT body gains `clear: true`. `{ "source": "manual", "clear": true }` stores
  a standing decision with `source = manual`, `manual_value = ''`. The rules are:
  - `clear` with any other source is a 400.
  - `clear` with a non-empty `manual_value` is a 400.
  - `manual` with an empty value **without** `clear` stays a 400, the existing guard against an
    empty Custom submit.
- **Storage:** unchanged. The existing row shape, no migration. The only path that writes
  `manual` + `''` is `clear`, so such a row always means "owner cleared".
- **Resolver:** unchanged. The manual branch already yields no item for an empty value, and the
  standing decision keeps the field in `resolved[]`.
- **Marker:** `{ source: "manual", standing: true }` with no `manual_value` (the field is
  `omitempty`) and empty `values`. A cleared field is recognised by exactly that: `source ===
  'manual'` and an empty `manual_value`. The SPA reads it through one helper (`isCleared(decision)`),
  never an inline check.

**Nothing is written as a value.** The file never receives a placeholder string. Clearing ends in a
tag *deletion* (D4) or in nothing at all.

### D2. Clearing is allowed only on fields that opt in; `studio` is the first

The API accepts `clear` only for canonicals in an allowlist in `internal/api/decisions.go`,
initially `{studio}`. Anything else gets a 400 naming the field. A field joins the list only when
all of these hold:

1. It is a replace field (merge/multi fields are already refused before this check).
2. It is **not an identity key**. `title` (composite key plus filename fallback) and
   `external_provider_id` (ADR-055) never join.
3. It is **not an image**. `WriteBatch` rejects image deletes (`writeback.go:66-71`), so a cleared
   image could never reach the file.
4. Its tag can be deleted on every container it maps to, proven by a writeback test per container
   (see D4).

`studio` qualifies, and D5 keeps its collision gate even though it's in the collision key.
`release_date` is also in the key and feeds the film year sync, so it stays out until someone asks
for it and the year sync says what "no date" means.

Two endpoints accept `clear`, and they are the two places an owner sets a video's studio:

- the media decision PUT (one video, D1), and
- the **film-studio cascade**, `POST /films/{id}/studio/cascade` (every video attached to the film,
  ADR-087; see D6). It already parses its body with the same `decodeDecisionBody`, so `clear` is
  validated once for both.

The entity *record* endpoints (person/studio/film fields, `record|manual|provider`) don't get it: an
entity's own record has no file tag to suppress, so blanking a value there is a rename question,
not this one. A film's studio is not one of those record fields. It's the union of its videos'
studios (`FilmStudios`), so clearing it means clearing theirs.

### D3. `video_studios` follows the decision immediately, not after writeback

No new sync code. A cleared studio goes down the existing path: `setFieldDecision` →
`decideStudioForVideo` → collision gate → `SetDecisionChecked` → relink → `ReconcileVideoStudios`
with no names. The video's links are removed at decision time, and the media detail's `studios`
array is `[]` on the next read. That's what the handoff's `+ Add studio` state depends on.

A re-scan that parses the same wrong studio back into the file layer changes nothing: the standing
decision still wins, and once D4 has removed the tag the parse has nothing to find.

**Consequence carried over from ADR-053, not introduced here:** prune-on-empty hard-deletes a studio
left with zero links. Clearing the only video of a curated studio (uploaded logo, halo, enrichment)
deletes that studio, just as reassigning that video does today. Clearing makes the case more
reachable. The follow-up is HOLODEX-494.

`DELETE …/decision` is unchanged: it reverts to the file value and relinks. It's the one-call undo
for a clear, alongside picking the `·file` chip.

### D4. Writeback removes the tag, through an explicit `clear` flag

An empty value list keeps meaning "nothing to write", the safety behaviour that stops a
sanitised-away value from wiping a tag. Removal is asked for explicitly, with the same flag name as
D1:

- **Queue:** `writequeue.JobField` gains `Clear bool`. In `buildBatch`, a replace field with
  `Clear: true` and no values maps through `writeback.TagForField` to a `Mapped{Delete: true}`, the
  shape ADR-110 already writes for genres. `Clear` with values present is a programming error and is
  rejected.
- **HTTP:** a writeback entry may be `{ "field": "studio", "clear": true }` with no `values`. It's
  accepted only when the video's standing decision for that field is cleared (D1), so a client can't
  delete an arbitrary tag by sending `clear`. Every other empty entry still gets the existing 400.
- **What a clear deletes: every file tag the field reads from, not just its write target.** A
  field *writes* one tag per container (`studio` → `Publisher`, `internal/writeback/tags.go`) but
  *reads* from an ordered list of file tags in its mapping (`studio`: `Publisher`, `Label`, `Studio`,
  `ProductionCompany`, first one found wins). Deleting only `Publisher` would leave a `Label` the
  parser found, so the file layer would resolve that value again and the field would never read in
  sync. So a `Clear` job expands, at write time, to one `Delete` for the write target plus one for
  each un-namespaced source in the field's live mapping. Each source name is first **qualified the
  way the write target is** (the target's group prefix: `QuickTime:Label` on MP4 because the
  target is `QuickTime:Publisher`, bare `Label` on MKV) and then kept only if
  `writeback.ValidTagKeyName(container, name)` accepts it. Namespaced sources (`filename:`, `tmdb:`) aren't file tags and are skipped. This is
  the same mapping-derived, allowlist-checked tag-name path ADR-110 uses for tag keys, and it never
  trusts a tag name from the request (security C2). A source tag that can't be deleted on some
  container survives: read-back then shows `in_sync: false`, and the cockpit keeps the row open.
  That's honest rather than silent.
- **Write target:** `markWriteTargets` (`internal/api/writeback.go:107`) must stamp a cleared row
  with the tag it would delete. Today a standing decision with no value gets an empty
  `write_target`, which is ADR-113's deliberate "blank pin stays unwritable" rule, so the dialog
  would disable the cleared row and the clear could never be written. The exception is narrow: a
  standing `manual` decision with an empty value, on an allowlisted field, asks the mapper for its
  tag. A blank pin (a `file` or `provider:` pin to a layer that happens to be empty) keeps today's
  unwritable stamp. The two can't be confused, because `manual` + `''` was unreachable before D1.
- **Cockpit:** a cleared row sends `clear: true` instead of `values: []`
  (`WritebackFormDialog.svelte:306` sends `[]` today and gets the 400). The row shows applied `—`
  against the file value, like any other lagging decision (applied vs. on file).
- **Containers:** the delete args are the ones ADR-110 uses (exiftool `-TAG=`, ffmpeg `key=`, the
  mkvpropedit path from ADR-117). `studio` gets one writeback test per mapped container, run under
  `make test-image`, before it goes on the D2 list.
- **Read-back:** after the write, ADR-073/093 read-back refreshes the file layer, the file has no
  studio, and `in_sync` becomes `true` (`""` = `""`).

**Revert:** `Revert` rewrites the snapshot's prior value, which for a clear is the old studio, so it
reverts fully with no change. The separate, long-standing gap is that reverting a write which
*added* a tag is skipped. That could now use `Clear`, and is HOLODEX-495.

### D5. The collision gate runs for a clear

Clearing the studio changes the composite key exactly as a new studio does, so it goes through the
same HOLODEX-270 check and returns the same `{conflict}` shape. `StudioPicker` already renders the
verdict for any `decide` result, and the handoff reuses it for detach. There is no bypass for "only
removing".

### D6. Clearing a film's studio: the same decision per video, but writeback is immediate

Clearing on the film page is D1 applied to each attached video by the existing cascade
(`cascadeFilmStudio`, ADR-087). Everything about the **decision** is identical to the media page:

- each video gets a standing `manual` + `''` decision via `decideStudioForVideo`;
- the collision gate runs per video (D5), and ADR-087's best-effort holds: a collision or error
  excludes only that video;
- each video's links are removed (D3), so the film page's studio union empties as the videos clear.

The **writeback** differs, and only in timing:

| | Media page | Film page |
|---|---|---|
| Decision | one video | every attached video, best-effort |
| Writeback | none at decision time; the owner writes later from the cockpit (ADR-091) | enqueued **in the same request**, one shared `batchID` (ADR-087 decide-then-enqueue), then handed to `WritebackBatchDialog` |
| Job shape | cockpit sends `{field, clear: true}` | cascade builds `JobField{Field: "studio", Clear: true}` |

**The cascade must build a `Clear` job, not `Values: names`.** On a clear, `names` is empty.
`buildBatch` silently skips an empty non-genres field, so the job would write nothing while the
cascade reported the video as `enqueued`. That would be a false success on the one surface that
promises the write. D4's "HTTP `clear` only against a standing cleared decision" holds trivially
here, because the cascade sets that decision for each video before building its job.

**Scope on a mixed-studio film: matching videos only** (owner decision 2026-09-29,
[F74 spec](../specs/studio-clear.md) R5). The film page shows one chip per studio in the union, and a
chip's `×` clears only the videos carrying *that* studio. A cascade `clear` therefore requires a
`studio_id`, and the handler walks `VideoIDsForFilm` ∩ the videos linked to that studio in
`video_studios`. `studio_id` without `clear` is a 400: a *change* still sets every video, as
ADR-087 does. The per-video decision mechanics above are unchanged; only the set of video ids
differs.

### D7. Wire and SPA

- `DecisionSource` in `web/src/lib/types.ts` is **unchanged**.
- `api.setFieldDecision` gains an optional `clear`.
- `isCleared(decision)` sits beside `sourceChips` in `$lib/f36`. `sourceChips` must not emit an
  empty Custom chip for a cleared decision: the cleared state is shown by the UI (the picker's empty
  Linked-now section), never as a candidate. That's the handoff's "no 'No studio' chip" rule.
- Visitors see nothing, because the studio block's `isOwner || values.length` gate already hides an
  empty field.

## Options considered

### A. Manual decision, empty value, explicit `clear` request (chosen)

| Dimension | Assessment |
|---|---|
| Complexity | Lowest of the safe options: no grammar change, no resolver change, no migration; one request flag, one allowlist, one queue flag |
| Legibility | Good: "the owner decided the value is nothing", matching how the owner thinks about it (owner review 2026-09-29) |
| Risk | A cleared row and a manual row share a source, so any code that renders `manual_value` must tolerate `''`. That's why `isCleared` is the single test |

**Pros:** The grammar keeps meaning *where the value comes from*. Existing resolver behaviour
already handles it. The empty-Custom guard survives, because clearing needs its own flag.
**Cons:** "Cleared" is inferred from `manual` + `''` rather than stored as its own word. That's
safe only because `clear` is the sole writer, which D1 makes a rule.

### B. A new decision source, `none`

Rejected on owner review. `none` is a value, not a source: every other source names the layer the
value comes from, and here the owner is still that layer. It would also widen a wire contract that
the SPA, the MCP payloads and every `switch` on source read. The resolver's `default: // manual`
branch would silently mistreat it wherever a case was missed.

### C. Allow `manual` with an empty value, no flag

Rejected. It removes the guard against an empty Custom submit in order to add a feature: a slip
would silently clear the field, and then clear the file's tag on the next writeback.

### D. Edit `video_studios` directly (a link-only detach)

Rejected. ADR-053 makes `ReconcileVideoStudios` the table's **sole writer**, derived from the
resolved value. The next relink (scan, enrich or any decision) would re-add the link from the file.

### E. Implicit tag delete whenever a replace field's values sanitize to empty

Rejected. It turns today's safe skip into a destructive write. D4's explicit `clear`, gated on a
standing cleared decision, keeps deletion something the owner decided.

## Consequences

- **Easier:** the owner can clear a mis-parsed studio, and any replace field that passes D2's bar
  can follow. The writeback cockpit gains a real "remove this tag" row.
- **Harder:** anything that renders a manual decision's value must handle an empty one, via
  `isCleared`.
- **Watch:**
  - The prune-on-empty consequence in D3 (HOLODEX-494).
  - A cleared field whose provider later offers a value: the decision stands, so the provider value
    doesn't win. That's the same as any pin, and it's the point.

## Action items

1. [ ] API: `clear` on the decision PUT (manual only, empty value only, D2 allowlist `{studio}`);
   the existing empty-manual 400 unchanged. Integration test: collision gate runs, links removed,
   `studios: []`, marker `{manual, standing}` with empty values, `in_sync` false against a file value.
2. [ ] Writequeue `JobField.Clear` → `Mapped{Delete}`; HTTP `clear: true` accepted only against a
   standing cleared decision; per-container studio delete tests under `make test-image`.
3. [ ] SPA: `isCleared` in `$lib/f36`; `sourceChips` emits no empty Custom chip; the cockpit sends
   `clear`; `StudioPicker` attached-chip detach per the handoff.
3b. [ ] Film cascade (D6): accept `clear`; build `JobField{Clear: true}` per cleared video. Test:
   a cleared cascade enqueues a tag delete (not an empty-values job) for every non-colliding
   video, and a colliding one is excluded as today. `FilmStudioCascadeDialog` gets the same
   attached-chip detach, handing off to `WritebackBatchDialog` as today.
4. [ ] Spec + testing-strategy for HOLODEX-493; `/security-review` (a new owner mutation that can
   delete file tags).
5. [ ] Follow-up HOLODEX-495 (not this epic): let `Revert` use `Clear` for a tag the original write
   added.
6. [ ] Follow-up HOLODEX-494 (not this epic): prune-on-empty deletes a curated studio (logo, halo,
   enrichment) when its last video is cleared or reassigned. Decide whether curation should keep it
   alive.
