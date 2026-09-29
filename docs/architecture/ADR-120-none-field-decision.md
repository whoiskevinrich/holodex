# ADR-120: A `none` decision resolves a replace field to no value, and writeback removes the tag

**Status:** Proposed
**Date:** 2026-09-29
**Deciders:** Project owner

**Extends:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (the decision grammar gains a
fourth source) and [ADR-110](ADR-110-tag-writeback-file-contract.md) (the tag `Delete` write,
until now reserved for genres/tag keys, gets a second producer). **Relates to:**
[ADR-053](ADR-053-studio-entity-and-resolved-link-derivation.md) (`video_studios` derived from the resolved studio,
prune-on-empty), [ADR-093](ADR-093-writeback-readback-and-tristate-in-sync.md) (`in_sync`),
[ADR-113](ADR-113-owner-offered-empty-fields.md) (the resolver's empty-field drop and its
exceptions), [ADR-117](ADR-117-bundle-mkvtoolnix-runtime.md) (MKV in-place writes).
HOLODEX-493 (first adopter: `studio`) ·
[studio-detach-handoff.md](../design/studio-detach-handoff.md).

---

## Context

The owner wants to **detach a video's studio**: say "this video has no studio" when its file carries
a studio tag. The approved UI is an attached chip in `StudioPicker` whose `×` detaches
(HOLODEX-493 handoff). Nothing below the UI can express that yet:

- **The grammar has no word for it.** `fieldsource.Valid` accepts `file`, `manual` and
  `provider:<name>` (`internal/fieldsource/fieldsource.go:26`). A standing decision always names a
  layer to read.
- **The API refuses the nearest thing.** `PUT /media/{id}/fields/{canonical}/decision` with
  `source: manual` and an empty `manual_value` is a 400 (`internal/api/decisions.go:43-58`).
- **`DELETE …/decision` is the opposite.** It removes the decision, so the field falls back to the
  file value, which is the studio the owner wanted gone.
- **Writeback cannot remove a replace field's tag.** `writeback.FieldWrite.Delete` exists
  (ADR-110), but its only producer is `writequeue.buildBatch` for `genres` and tag-key fields. Any
  other field whose values sanitize to empty is silently skipped (`writequeue.go:490-496`), and the
  HTTP handler 400s a whole batch on an entry with no values (`internal/api/writeback.go:271`).
  `Revert` says so outright: "the write path has no 'clear this tag' primitive today".

Most of the machinery for an empty *outcome* already exists, because a pin to a layer that happens
to be empty has been legal since F37:

- `ResolveFields` keeps a replace field with a **standing** decision even when its value is empty
  (`resolver.go:349-368`, "blank-pin … F37 RD3"). The decision marker is still reported, so the
  owner keeps a control.
- `in_sync` compares the decided value against the file's (`resolver.go:814-823`). An empty
  decision against a file value is already `false`, which is exactly what the writeback cockpit
  needs.
- `ReconcileVideoStudios` with no names removes the video's links (`internal/repo/studios.go:33-43`),
  and the studio decision path already calls it through `decideStudioForVideo` → `relinkStudiosWithContext`.
- The HOLODEX-270 composite-key collision gate keys an empty studio as `""`, so two videos that both
  resolve to no studio still collide and the gate runs for a detach (`internal/repo/video_collision.go:315`).

What's missing is a *name* for "decided: nothing" that isn't confused with "pinned to a source
that is currently empty", and a write that removes the tag.

## Decision

### D1. A fourth decision source: `none`

`fieldsource.None = "none"`. `Valid` accepts it, and `manual_value` must be empty with it (anything
else is a 400). It is stored in `field_source_decisions.source` like any other source. That column
has no CHECK constraint (`0016_field_source_decisions.up.sql`), so **no migration**.

`resolveDecided` gets an explicit `case fieldsource.None: return nil, ""`. It must **not** fall into
the existing `default:` branch, which treats any unrecognised source as manual (`resolver.go:579`).
The marker reports `source: "none", standing: true` with empty values. `ForNamespace` never returns
`none`: an undecided field can't implicitly resolve to it.

Why a new token rather than an empty manual value: see Options. In short, `manual` with `""` is what
an accidental empty Custom submit looks like, and the API's refusal of it is a guard worth keeping.

### D2. `none` is allowed only on fields that opt in; `studio` is the first

The API accepts `none` only for canonicals in an allowlist in `internal/api/decisions.go`, initially
`{studio}`. Anything else gets a 400 naming the field. A field joins the list only when all of these
hold:

1. It is a replace field (merge/multi fields are already refused before this check).
2. It is **not an identity key**. `title` (composite key plus filename fallback) and
   `external_provider_id` (ADR-055) never join.
3. It is **not an image**. `WriteBatch` rejects image deletes (`writeback.go:66-71`), so an image
   `none` could never reach the file.
4. Its tag can be deleted on every container it maps to, proven by a writeback test per container
   (see D4).

`studio` qualifies. Although it is in the collision key, D5 keeps the gate. `release_date` also sits
in the key and feeds the film year sync; it stays out until someone asks for it and the year sync
says what "no date" means.

Only the media endpoint gets `none`. Person/studio/film entity endpoints (`record|manual|provider`)
don't: an entity's own record has no file tag to suppress, so a blank manual value there is a
rename question, not this one.

### D3. `video_studios` follows the decision immediately, not after writeback

No new sync code. A `none` studio decision goes down the existing path: `setFieldDecision` →
`decideStudioForVideo` → collision gate → `SetDecisionChecked` → relink → `ReconcileVideoStudios`
with no names. The video's links are removed at decision time, and the media detail's `studios`
array is `[]` on the next read. That is what the `+ Add studio` state in the handoff depends on.

**Consequence carried over from ADR-053, not introduced here:** prune-on-empty hard-deletes a studio
left with zero links. Detaching the only video of a curated studio (uploaded logo, halo, enrichment)
deletes that studio, just as reassigning that video to another studio does today. Detach makes the
case more reachable. It is a pre-existing behaviour with its own follow-up (Action item 7), not
something this ADR changes.

`DELETE …/decision` is unchanged: it reverts to the file value and relinks. It is the one-call undo
for a `none`, alongside picking the `·file` chip.

### D4. Writeback removes the tag, through an explicit `clear` flag

An empty value list keeps meaning "nothing to write" (the safety behaviour that stops a
sanitised-away value from wiping a tag). Removal is asked for explicitly:

- **Queue:** `writequeue.JobField` gains `Clear bool`. In `buildBatch`, a replace field with
  `Clear: true` and no values maps through `writeback.TagForField` to a `Mapped{Delete: true}`, the
  shape ADR-110 already writes for genres. `Clear` with values present is a programming error and is
  rejected.
- **HTTP:** a writeback entry may be `{ "field": "studio", "clear": true }` with no `values`. It is
  accepted only when the video's standing decision for that field is `none`, so the client cannot
  delete an arbitrary tag by sending `clear`. Every other empty entry still gets the existing 400.
- **Cockpit:** a row whose applied value comes from a `none` decision sends `clear: true` instead of
  `values: []` (`WritebackFormDialog.svelte:306` today sends `[]` and gets the 400). The row shows
  applied `—` against the file value, like any other lagging decision (applied vs. on file).
- **Containers:** the delete args are the ones ADR-110 uses (exiftool `-TAG=`, ffmpeg `key=`, the
  mkvpropedit path from ADR-117). `studio` gets one writeback test per mapped container, run
  under `make test-image`, before it goes on the D2 list.
- **Read-back:** after the write, ADR-073/093 read-back refreshes the file layer, the file has no
  studio, and `in_sync` becomes `true` (`""` = `""`).

**Revert:** `Revert` rewrites the snapshot's prior value, which for a detach is the old studio, so a
cleared tag reverts fully with no change. The long-standing gap that a revert of an *added* tag is
skipped could now use `Clear`. That's out of scope here, noted as Action item 6.

### D5. The collision gate runs for `none`

A `none` studio changes the composite key exactly as a new studio does, so it goes through the same
HOLODEX-270 check and returns the same `{conflict}` shape. `StudioPicker` already renders the
verdict for any `decide` result, and the handoff reuses it for detach. There is no bypass for "only
removing".

### D6. Wire and SPA

`DecisionSource` in `web/src/lib/types.ts` gains `'none'`. `sourceChips` must not render a chip for
it: `none` is a decision, not a candidate, which is the handoff's "none is never a source" rule. The
picker detects `decision.source === 'none'` only to know no studio is linked, which `studios.length`
already tells it. Visitors see nothing (the studio block's `isOwner || values.length` gate already
hides an empty field).

## Options considered

### A. New source token `none` (chosen)

| Dimension | Assessment |
|---|---|
| Complexity | Low: one constant, one resolver case, an allowlist check, one queue flag |
| Migration | None (no CHECK on `source`) |
| Legibility | High: the stored row says what the owner meant |
| Risk | Every `switch` on source must handle it. The resolver's `default: // manual` is the trap, and it's named in D1 |

**Pros:** Unambiguous in the DB, the API and the marker. The API keeps refusing an empty manual.
The UI can tell "cleared" from "pinned to an empty provider".
**Cons:** It widens a wire contract that the SPA, MCP payloads and any external client read.

### B. Allow `manual` with an empty value

| Dimension | Assessment |
|---|---|
| Complexity | Lowest: drop one validation line |
| Legibility | Poor: `manual ""` is indistinguishable from a sanitiser that ate the input |
| Risk | An empty Custom textarea submit silently clears the field. The current 400 exists to stop that |

Rejected. It removes a guard in order to add a feature.

### C. Edit `video_studios` directly (a "link-only" detach)

Rejected. ADR-053 makes `ReconcileVideoStudios` the table's **sole writer**, derived from the
resolved value. The next relink (scan, enrich, any decision) would re-add the link from the file.

### D. Pin to a source that's empty (the F37 blank pin)

Not applicable. It only yields "no studio" when the chosen layer is empty. The case here is a file
that *has* a studio.

### E. Implicit delete whenever a replace field's values sanitize to empty

Rejected. It turns today's safe skip into a destructive write. Every value a sanitiser rejects
would become a tag deletion. D4's explicit `clear`, gated on a standing `none`, keeps deletion
something the owner decided.

## Consequences

- **Easier:** the owner can remove a studio, and the same token is ready for other replace fields
  once they pass D2's bar. The writeback cockpit gains a real "remove this tag" row.
- **Harder:** anything that enumerates decision sources (resolver, `winnerToDecisionSource`, SPA
  `sourceChips`, MCP payload docs) must handle a fourth value.
- **Watch:** the prune-on-empty consequence in D3; a `none` on a field whose provider later offers a
  value (the decision stands, so the provider value doesn't win, the same as any pin).

## Action items

1. [ ] `fieldsource.None` + `Valid`; resolver `case None`; unit tests for the marker (`standing`,
   empty values, `in_sync` false against a file value, true against none).
2. [ ] API: accept `none` (empty `manual_value` only) for the D2 allowlist `{studio}`; 400 otherwise.
   Collision gate and relink covered by an integration test (links removed, `studios: []`).
3. [ ] Writequeue `JobField.Clear` → `Mapped{Delete}`; HTTP `clear: true` accepted only against a
   standing `none`; per-container studio delete tests under `make test-image`.
4. [ ] SPA: `DecisionSource` + `'none'`; `sourceChips` skips it; cockpit sends `clear`;
   `StudioPicker` attached-chip detach per the handoff.
5. [ ] Spec + testing-strategy for HOLODEX-493; `/security-review` (new owner mutation that can
   delete file tags).
6. [ ] Follow-up HOLODEX-495 (not this epic): let `Revert` use `Clear` for a tag the original write added.
7. [ ] Follow-up HOLODEX-494 (not this epic): prune-on-empty deletes a curated studio (logo, halo, enrichment)
   when its last video is detached or reassigned. Decide whether curation should keep it alive.
