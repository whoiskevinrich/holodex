# ADR-097: Alpha-preserving image normalization — PNG for non-opaque images, extension derived from the bytes

**Status**: Proposed
**Date**: 2026-09-16
**Deciders**: Project owner
**Relates to**: revisits [ADR-038](ADR-038-person-images.md) §2 step 3 (the single JPEG output format) and the `{id}.jpg` layout it fixed for [ADR-057](ADR-057-self-hosted-studio-logo.md)/[ADR-079](ADR-079-studio-image-roles.md) (studio), [ADR-086](ADR-086-film-provider-enrichment.md) (film), [ADR-059](ADR-059-provider-brand-icon.md) (provider icon) and HOLODEX-286's shared `internal/entityimage`; leaves [ADR-009](ADR-009-thumbnail-strategy.md) (video thumbnails/posters) untouched. Bug HOLODEX-396.

---

## Context

Every self-hosted entity image — person, studio, film, provider brand icon, from an owner upload or an enrichment download — passes through one ingest gate, `personimage.Normalize` (ADR-038 §2): sniff-decode with the stdlib, bound the dimensions before the full decode, then **re-encode to JPEG**, which strips EXIF/XMP/ICC and any polyglot payload as a side effect. The disk layout that grew around it hardcodes the result: `{dir}/{entityID}/{imageID}.jpg`, served with an explicit `Content-Type: image/jpeg` and `X-Content-Type-Options: nosniff`.

JPEG has no alpha channel. A studio logo or provider mark with a transparent background — the normal shape of a brand asset, and exactly what an SVG export produces — comes back with an opaque fill: Go's encoder composites transparent pixels to black; whatever exported the file may already have matted it white. On a skinned page (ADR-021) with three different surface colours, neither is the page background, so the mark sits in a visible box. That is the bug reported as "studio SVG images get a white background". (Raw `.svg` is refused at the gate with 400 — the stdlib has no SVG decoder, by design per ADR-038's threat table — so the file that reached Holodex was a raster export; this ADR does **not** add SVG ingest.)

ADR-038 already anticipated the split — its wording is "JPEG *for opaque*" — but the implementation never grew the other branch, and the `.jpg` layout was then copied verbatim into four more packages.

Forces:

- **The hardening must not weaken.** The decode → re-encode round trip is the metadata strip and the polyglot defence; whichever encoder writes the output must carry nothing from the source. Go's `image/png` encoder writes only the pixel chunks (no `tEXt`/`iTXt`/`iCCP`/`eXIf`), so a PNG re-encode strips exactly as a JPEG one does.
- **No migration for a bug fix.** Every existing image is a `.jpg`; a bug fix should not rewrite files or add a column to five tables to say which format each row is in.
- **Photos should stay JPEG.** Portraits, posters and banners are opaque photographs; PNG would be several times the bytes for no gain. Only an image that actually carries transparency should change format.
- **Video posters are not entity images.** `POST /media/{id}/poster` runs the same `Normalize` but writes into the ADR-009 thumbnail pipeline's `{id}.jpg` / `{id}-poster.jpg` slots, which the thumbnail manager regenerates as JPEG. That path must keep producing JPEG bytes.

## Decision

### D1 — Normalize keeps alpha: PNG when the decoded image is not opaque, JPEG otherwise

`personimage.Normalize` keeps every step of ADR-038 §2 and changes only the encoder choice at step 3: after decode and downscale, if the image reports `Opaque() == false` it is re-encoded with `image/png`, else with `image/jpeg` at the existing quality. Every stdlib and `x/image` decoder returns a type with an `Opaque` method (a full-pixel scan for the alpha formats, constant `true` for YCbCr/Gray); an unknown type is treated as opaque so the pre-existing JPEG path stays the fallback. The dimensions and the content hash (ADR-050) are computed over the normalized output exactly as before.

`personimage.NormalizeJPEG` is the same pipeline with the PNG branch disabled, for the one consumer whose storage slot is JPEG by contract — the video poster upload. Nothing else calls it.

### D2 — The on-disk extension is derived from the bytes, never stored

`entityimage.Ext(data)` returns `.png` for an 8-byte PNG signature and `.jpg` otherwise. It only ever sees `Normalize`'s own output, so this is an exact check of a trusted byte stream, not a sniff of untrusted input. `Store` writes to `{id}{Ext(data)}`; `Find(dir, entityID, imageID)` stats `.jpg` then `.png` and returns the one that exists (an image id is server-assigned and never reused, so at most one exists); `Remove` clears both. `providericon` mirrors the same three operations over its flat `{id}{ext}` layout. No table changes; every existing `.jpg` keeps serving through `Find` unchanged.

The per-entity wrappers (`personimage`/`studioimage`/`filmimage`) expose `Find` in place of the former `ImagePath` — a path is now a lookup, not a formula — and remain the only call surface, per HOLODEX-286.

### D3 — Serve with the Content-Type of the file that was found

`serveEntityImageFile` and the provider-icon route set `Content-Type` from `entityimage.ContentType(path)` — `image/png` or `image/jpeg` by extension — and keep `X-Content-Type-Options: nosniff` and the immutable cache. The mapping is explicit rather than `mime.TypeByExtension`, which consults the OS registry on Windows and could disagree with the header we promise not to let the browser second-guess.

### D4 — Rejected alternatives

- **Always PNG.** Simplest, but turns every photograph into a 3–5× larger file on hardware ADR-003 sized for a NAS; the alpha branch is the exception, and the extension check keeps it cheap.
- **A `format` column.** Truthful but five migrations (person/studio/film/provider-icon rows plus the sink's insert shape) to record something the first eight bytes of the file already say.
- **Keep `.jpg` as the filename and sniff on serve.** Zero code in the storage layer, but a `.jpg` file holding PNG bytes misleads anyone inspecting the data directory and makes the `nosniff` header a lie about the file's own name.
- **Rasterize SVG server-side.** Would address the report's literal wording, but adds an SVG parser to the untrusted-bytes perimeter (external references, scripts, foreign objects) for a format ADR-038 deliberately excludes. If wanted later it is its own ADR with its own security review; a transparent PNG export already reaches parity for a logo.

## Consequences

- A transparent logo, icon or provider mark uploads and serves with its alpha intact on every skin. Opaque uploads are byte-for-byte unchanged, so existing content hashes and dedup (ADR-050) are unaffected.
- The ingest threat model of ADR-038 is preserved: same sniff-decode, same bomb guard, same decode → re-encode strip; the only new code on the perimeter is an 8-byte prefix comparison of bytes Holodex itself produced.
- `personimage.Find` and friends return an error where `ImagePath` returned a string; the three serve routes and the promote/backfill readers now 404/skip on lookup failure exactly as they did on a failed `os.Open`.
- Video posters still flatten transparency (`NormalizeJPEG`) — a transparent poster is unusual and the thumbnail pipeline's JPEG contract is not worth reopening for it.
- The hash of a transparent image changes (PNG bytes instead of JPEG bytes). Only re-ingest of a previously-flattened image is affected: it is no longer a duplicate of its old JPEG self, which is the desired outcome.
