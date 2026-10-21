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
- [ ] backend → `{cmd,internal,providers}/**` — lane A: `?w=` on both person routes, lazy variant cache, `Remove` clears variants
- [ ] frontend → `web/src/**` — lane B: width option on the URL builders, `srcset` / `sizes` in `PersonImageFrame` and its wrappers
- [x] testing `testing-strategy` — testing-strategy §4 row "Person image width variants"
- [ ] security `security-review` — a public route now writes to disk: confirm the tier bound and the path discipline

## Up next — ordered (position = priority)

1. [ ] [—] Lane A (backend) and lane B (frontend) in parallel sessions, each merging into `HOLODEX-332-person-image-widths`
2. [ ] [—] Integrated QA on the stress fixture: `w=320` per poster card at max density; banner fetches no `w`
3. [ ] [—] `/code-review high --fix`, `/security-review`, then mark the Draft PR ready
4. [ ] [—] Pin images.md's "Decided in" to the squash sha after merge
5. [ ] [—] File a Relates issue: the same width variants for studio and film images

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-10-20 · contract locked
- skills: architecture, testing-strategy
- handoff: The contract is locked (lazy cached tiers 160/320/640 via `?w=`, snap-up, no upscale) in images.md, the spec and testing strategy; next is the backend and frontend lanes in parallel sessions.

## Dropped — newest first (the reason is the point)

- Eager write-time tiers (the video poster model) — every ingest path plus a boot backfill, for what lazy generation gets from one code path.
