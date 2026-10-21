---
key: HOLODEX-332
status: in-progress
profile: full
release_note: "Person images on the People grid, cast lists and avatars now load at a size that fits the card instead of the full-size original, so image-heavy pages load faster."
---

# HOLODEX-332 · Person image width variants (child of HOLODEX-13)

Done when every small person-image surface fetches a lazily generated, disk-cached width tier
(160 / 320 / 640) via `srcset`, and the banner, viewer and crop editor still get the original.

**Design package:** [images.md, "Person image width variants"](../architecture/images.md) ·
[people-images.md, "Image size per surface"](../specs/people-images.md)

## Gates — definition of done

- [x] spec `write-spec` → `docs/specs/**` — `people-images.md` "Image size per surface"
- [x] architecture `architecture` → `docs/architecture/**` — `images.md` "Person image width variants" (lazy cached tiers; eager write-time tiers rejected)
- [~] design `design-handoff` → `docs/design/**` — no visual change: markup only (`srcset` / `sizes`), every surface looks the same
- [x] backend → `{cmd,internal,providers}/**` — `personimage.Variant` / `SnapWidth` (bounded, shared decodes), `entityimage.StoreVariant` and variant-aware `Remove`, `?w=` in `servePersonImageFile`
- [x] frontend → `web/src/**` — `personImageSrcset.ts`, `sizes` in `PersonImageFrame` and every small-surface caller; banner, viewer and crop editor stay on the original
- [x] testing `testing-strategy` — testing-strategy §4 row "Person image width variants"; `variant_test.go`, `person_image_variants_test.go`, `entityimage_test.go`, `personImageSrcset.test.ts`, `api.test.ts`
- [x] security `security-review` — no request value reaches a path (`SnapWidth` → fixed tiers, ids server-assigned); variants serve only after the existing ownership checks; hard-link publish never replaces a file

## Up next — ordered (position = priority)

1. [ ] [—] Pin images.md's "Decided in" to the squash sha after #543 merges — `docs/architecture/images.md`

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-20 · build lanes merged
- skills: code-review, security-review, handoff
- handoff: Backend and frontend lanes merged and QA'd together on the stress fixture: tiers serve correctly, poster cards pick `w=160`/`320`/`640` by density, the banner and viewer keep the original. Code review bounded the variant decodes; security review clean. PR #543 marked ready. Next: pin images.md's "Decided in" after merge. Studio and film variants spun out to HOLODEX-590.

### 2026-10-20 · contract locked
- skills: architecture, testing-strategy, implement
- handoff: Crossed into build. The contract is locked: lazy cached tiers 160/320/640 via `?w=`, with snap-up and no upscale, in images.md, the spec and the testing strategy. No sign-off was owed (design `[~]`), main is merged in, and the Draft PR is open. Start at the backend and frontend lanes.

## Dropped — newest first (the reason is the point)

- [~] [architecture] Eager write-time tiers (the video poster model) — dropped 2026-10-20: every ingest path plus a boot backfill, for what lazy generation gets from one code path
