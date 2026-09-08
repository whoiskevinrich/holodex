# ADR-093: A field's sync state is unknown, not false, when nothing reads the written tag back

**Status:** Proposed
**Date:** 2026-09-07
**Deciders:** Project owner

**Extends:** [ADR-051](ADR-051-per-field-source-of-truth-decisions.md) (the `in_sync` marker this
makes tri-state) · [ADR-073](ADR-073-post-write-baseline-resync.md) (which fixed the *stale*
baseline; this fixes the *absent* one) · [ADR-041](ADR-041-metadata-writeback.md) (the write-target
table) · [ADR-052](ADR-052-baseline-source-contract.md) (the baseline contract being read through).
**Issue:** [HOLODEX-335](https://whoiskevinrich.atlassian.net/browse/HOLODEX-335).

---

## Context

After a successful writeback, `title` and `release_date` reported `in_sync: false` forever. The
tags were genuinely written — `ffprobe` read both back off the file — and every other field written
in the same batch (`studio`, `overview`) cleared. Sampled across seven entities and both container
formats (six Matroska, one MP4), every one behaved identically, so this was neither a stale cache
nor a failed write. Re-running the writeback rewrote the same correct tags and changed nothing.

ADR-073 fixed the adjacent failure — the stored baseline was never refreshed after a write — and it
holds: the post-write re-extract runs, and `videos`/`extra_metadata` do carry the written values.
What remained is one step earlier.

**Two tables decide this, and nothing connects them.** Writeback picks its destination tag from
`internal/writeback`'s `formatMap` (`release_date` → `Year`, `title` → `Title`, per container). The
resolver computes `in_sync` in `replaceMarkers` as `decided == fileVal`, where `fileVal` comes from
`baselineValue` — a walk of the field's **`sources:` list in `metadata-mappings.yaml`**, looking for
one in the `file:` namespace. A canonical field can therefore be writable and unreadable at the same
time, and the shipped example mapping made exactly that mistake:

```yaml
- canonical: release_date
  sources:
    - filename:release_date   # not a baseline namespace
    - tmdb:release_date       # not a baseline namespace
```

No `file:` source, so `baselineValue` returns `("", _, false)`, `fileVal` is `""`, and every
standing decision compares unequal to it. `studio` cleared only because it happens to list a bare
`Publisher` (= `file:Publisher`), which is the same tag writeback writes.

**The comparison collapsed two different situations into `false`.** `fileVal == ""` means either
"the file tag is declared and genuinely empty" (a real mismatch worth reporting) or "no source
declares where to read this from" (unknowable). `replaceMarkers` discarded `baselineValue`'s `ok`
and treated both as the first.

The consequence is worse than a cosmetic pill. `outOfSyncCount` drives the header's "*N* out of
sync", and `needsWriteback` pre-checks exactly those rows in the batch dialog — so the false
positives were indistinguishable from real drift, and invited repeated no-op remuxes of large files.

Empirical check of the round-trip (exiftool 13, both containers), since the fix depends on which
written tags are actually readable:

| Canonical | Write target | Reads back as | Reachable by a `file:` source? |
|---|---|---|---|
| `title` | `Title` / `QuickTime:Title` | `Title` | yes — consumed into `videos.title`, addressed as `file:title` |
| `release_date` | `Year` / `QuickTime:Year` | `Year` | yes — lands in `extra_metadata` |
| `overview` | `Comment` / `QuickTime:Comment` | `Comment` | yes |
| `studio` | `Publisher` / `QuickTime:Publisher` | `Publisher` | yes (already mapped) |
| `tagline` | `Subtitle` / `QuickTime:Keywords` | `Subtitle` / **`Keywords`** | Matroska only — `Keywords` is in the extractor's `tagKeys` and is consumed into `Extracted.Tags` |
| `original_title` | `OriginalMediaType` (Matroska only) | `Originalmediatype` | Matroska only — MP4 has no write target |
| `original_language` | `Language` / `QuickTime:MediaLanguage` | `Language` / **nothing** | Matroska only — `QuickTime:MediaLanguage` is not a defined exiftool tag; the write is dropped with a warning |

## Decision

**D1. `in_sync` is tri-state: `true`, `false`, or absent.** `replaceMarkers` returns `nil` when the
field declares no baseline source at all, so the API omits `in_sync` entirely. A new
`hasBaselineSource` predicate answers "does this field declare a baseline source", deliberately
separate from `baselineValue`'s `ok` ("did a baseline source carry a value") — collapsing those two
questions is the bug. A field that *does* declare a `file:` source whose tag is empty still reports
`false`, because that is a real mismatch.

The frontend needs no change: `outOfSync` is already `in_sync === false`, so absent reads as "not
out of sync" — the same contract person and studio fields have used since F37, where there is no
file to compare against. The cost is that such a field is no longer pre-checked in the batch dialog;
that is correct, because we cannot tell whether it needs writing.

**D2. The shipped example mapping declares the file tag for every replace field it writes.**
`release_date` gains `Year`, `overview` gains `Comment` (promoted from a commented aside to a real
source), and the commented `title` block says explicitly that `file:title` is what makes a written
title reportable. Listed first, matching `studio`'s existing shape and the file-first default.

**D3. The three fields whose write target cannot be read back stay provider-only, on purpose.**
`tagline`, `original_title`, and `original_language` (see the table above) get no `file:` source.
Adding one would make their sync state *look* knowable and then report every MP4 write as out of
sync — strictly worse than D1's honest "unknown".

**D4. A test locks the two tables together — on the shipped example only.**
`TestExampleMappingCoversWriteTargets` fails when a replace field in the example mapping is
writeback-capable but declares no `file:` source that *matches the tag writeback writes*, unless it
is listed in `unreadableWriteTargets` with a written reason. It compares keys rather than checking
mere presence, because `overview: [Description, tmdb:overview]` would satisfy a presence check while
reading a different tag than the one written — reproducing this bug exactly. Scoped to replace
fields: merge fields carry no `in_sync` by ADR-051 RD1, which also sidesteps `genres`/`actors`,
whose write targets (`Genre`/`Artist`) are consumed into `Extracted.Tags`/`People` rather than
`extra_metadata`.

Two limits, stated so this is not mistaken for closing the gap. It guards the *example*, which is
not the file any deployment runs; and a canonical the example ships commented out — today `title`,
one of the two fields this ADR is named for — is absent from `Fields()` and therefore uncovered.

## Consequences

- **Existing deployments need a one-line config edit.** `metadata-mappings.yaml` is gitignored, so
  D2 reaches new installs only. Until an operator adds the `file:` source, D1 changes their symptom
  from a permanently-lit pill to no sync signal at all for that field — honest, but not yet useful.
  The migration note lives in `docs/reference/canonical-fields.md`.
- **The obvious next step is to run D4's check against the live mapping, not the example.** The same
  comparison, applied at config load / `reload-config` and logged as a warning per
  writable-but-unreadable replace field, would have surfaced this on the operator's own install
  instead of via a bug report, and would make D2's migration self-announcing rather than
  doc-dependent. `mapping.Load` has no validation surface today, and the check needs the writeback
  tag table, so it wants a call site above both packages rather than a new import in the loader.
  Deliberately not done here: it is a new startup surface, not a narrowing of this defect.
- **Adding a file source changes precedence for that field.** Under the file-first default the file
  layer becomes the undecided winner, so a video whose `YEAR` tag holds a bare `2022` will show
  `2022` where it previously showed the provider's `2022-10-19`. That is the ADR-033/051 model
  working as designed (and what `studio` has always done), but it is a visible change for anyone
  adopting D2 into an existing config.
- **`in_sync: nil` is now a third state consumers must handle.** Anything that treats the marker as
  a boolean with a default will read "unknown" as "in sync". That is the safe direction — it
  suppresses a claim rather than inventing one — and matches the existing person/studio contract,
  but a future consumer wanting "needs attention" semantics must check for absence explicitly.
- **D3 documents two genuine writeback defects rather than fixing them.** `original_language` on MP4
  writes to a tag exiftool does not define, so the value is silently dropped; `tagline` on MP4 lands
  in a tag the extractor folds into content tags. Both are filed separately (HOLODEX-336) — they are
  write-path bugs, not sync-reporting bugs, and fixing them here would have widened a one-field
  regression fix into a container-mapping revision.
- The deeper fix — computing `in_sync` against the *write target* for the video's container rather
  than the field's declared baseline source — was considered and deferred. `markWriteTargets`
  already resolves that tag per container, so the data is present; it would fix every deployment
  with no config edit and remove the two-table hazard entirely. It needs a tag → read-location map
  (`Title` is consumed into `videos.title`, not `extra_metadata`) and would split "the file
  candidate chip" from "the sync comparison" into two different notions of the file value. Worth
  revisiting if D2's config migration proves to be a recurring support burden.
