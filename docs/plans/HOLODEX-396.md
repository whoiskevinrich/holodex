---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-396
status: in-progress
profile: full
release_note: A studio logo, icon or provider mark with a transparent background now keeps it — uploads and provider downloads are no longer flattened onto an opaque box.
---

# HOLODEX-396 · Transparent entity images lose their alpha channel on upload

Reported as "studio SVG images get a white background". Traced: raw `.svg` is refused at the
ADR-038 gate (400, no decoder — by design), so the file that landed was a raster export; the
gate then re-encoded **everything** to JPEG, which has no alpha, so any transparent logo came
back on an opaque fill (Go paints black; the export tool had already matted white). Fix at the
gate, not per entity: `Normalize` emits PNG for a non-opaque image, the disk extension follows
the bytes, the serve header follows the file. No migration. SVG ingest deliberately **not** added
(ADR-097 D4).

## Gates — definition of done

- [x] spec `write-spec` — no new spec; `docs/specs/studio-images.md` P0-5/P0-6 and
  `docs/specs/people-images.md` T1 updated to say PNG-when-alpha
- [x] architecture `architecture` — ADR-097 (revisits ADR-038 §2 step 3; ADR-038 status line
  + index row annotated)
- [~] design `design-handoff` — n/a: no UI change; the logo now sits on the skin surface, which
  is the state every `EntityImageSlot` mockup already showed
- [x] backend — `personimage.Normalize` (PNG branch via `Opaque()`), `NormalizeJPEG` for the
  video poster; `entityimage.Ext/Find/ContentType`, `Store`/`Remove` ext-aware; `providericon`
  mirrored; `serveEntityImageFile` + provider-icon route set `Content-Type` from the file;
  person promote + hash backfill read through `Find`
- [~] frontend — n/a
- [x] testing `testing-strategy` — row added; 3 new `personimage` tests, 2 `entityimage`,
  1 `providericon`, 1 end-to-end `TestStudioImage_TransparentLogoKeepsAlpha`; `go test ./...`
  green
- [/] security `security-review` — touches the untrusted-image ingest perimeter; run before
  marking ready

## Up next — ordered (position = priority)

1. [ ] [—] Human QA `[human]`: upload a transparent PNG logo on a studio page in all three skins —
   the mark should sit directly on the page surface with no box; then delete it and confirm the
   slot returns to the monogram
2. [ ] [—] Re-upload any studio logo that was flattened before this fix (the stored `.jpg` is
   not migrated — it stays opaque until replaced)

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · traced, fixed, tested, ADR written
- skills: debug (entry), code-review high --fix, code-review, security-review
- handoff: Draft PR open on `HOLODEX-396-preserve-image-transparency`; everything but the
  security review is green — run it, mark ready, then the human three-skin check.
