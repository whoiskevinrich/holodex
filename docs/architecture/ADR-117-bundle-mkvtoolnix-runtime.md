# ADR-117: Bundle MKVToolNix in the runtime image; MKV writeback edits tags in place

**Status:** Proposed
**Date:** 2026-09-28
**Deciders:** Project owner

**Amends:** [ADR-007](ADR-007-docker-structure.md) §Consequences, which calls `mkvtoolnix`
"test-only — the runtime image does not need it … Phase 3 writeback tooling is a separate future
ADR." This is that ADR.
**Extends:** [ADR-041](ADR-041-metadata-writeback.md) §file-safety (copy → write → rename), which is
unchanged. Only the tool that performs the *write* step for Matroska changes.
**Spec:** [phase-3-enrichment](../specs/phase-3-enrichment.md) F17.2 already names mkvpropedit as the
MKV writer. This brings the shipped image in line with it.
**Relates to:** HOLODEX-484/485 (the cover-replace fixes on both MKV paths),
[ADR-116](ADR-116-remux-fragmented-mp4-on-write.md) (the other writeback remux),
[ADR-073](ADR-073-post-write-baseline-resync.md) (the post-write re-probe is unchanged).

---

## Context

`writeMKVBatch` has two Matroska backends:

- **mkvpropedit** (MKVToolNix): edits the temp copy's Segment Info title, global `TAGS` element and
  attachments in place. It is used when `mkvpropedit`, `mkvextract` and `mkvmerge` are all on `PATH`.
- **ffmpeg** (`-map 0 -c copy -map_metadata 0` into a new file): the fallback.

The runtime image installs only `ffmpeg` and `exiftool`, so **every production MKV write takes the
ffmpeg fallback**. The mkvpropedit backend is tested, but it has never run in production.

That fallback rebuilds the container, which has caused real failures:

- **HOLODEX-413**: ffmpeg exposes an image attachment as an attached-pic video stream and refuses to
  copy one it can't decode ("dimensions not set"). One bad cover blocked every later writeback on
  that file.
- **HOLODEX-484**: the new cover's `-metadata:s:t:N` labels are addressed by output attachment
  index, which the copied attachments shift. A `cover.webp` made the remux fail outright.
- **Fidelity**: a remux rewrites every cluster, cue and header through ffmpeg's model of Matroska.
  Anything ffmpeg doesn't model can be lost or normalized: editions and ordered chapters, segment
  linking UIDs, tags at track/chapter target levels, per-track flags. ADR-041's contract is that
  "every other tag, attachment, and stream on the file is preserved". A remux keeps that only as far
  as ffmpeg understands the file.

The owner wants **no host dependencies**. The image is the server, so a tool the app needs belongs
in the image.

Measured on 2026-09-28, on `debian:bookworm-slim` with the image's current packages already
installed: `apt-get install --no-install-recommends mkvtoolnix` adds **10 packages / 37.0 MB**
(mkvtoolnix v74, libmatroska7, libebml5, and `libqt5core5a`, which the CLI tools link against; no
GUI).

## Decision

### D1 — The runtime image installs `mkvtoolnix`

The runtime stage adds `mkvtoolnix` to its existing `apt-get install --no-install-recommends` line
beside `ffmpeg` and `libimage-exiftool-perl`, from the same Debian bookworm archive, so it gets
security updates the same way (rebuild the image).

### D2 — No switch, flag or config: tool detection stays the selector

`writeMKVBatch` already prefers mkvpropedit when all three tools are present. Nothing in the code
changes to select it. The image contents are the selector, which keeps two properties:

- **Rollback is removing the package.** The ffmpeg path is unchanged and becomes the active path
  again with no code or data change.
- **Dev hosts and custom images without MKVToolNix keep working.** They take the ffmpeg path, which
  stays fully maintained and tested (HOLODEX-484's tests remain).

A config toggle was rejected: it would be a second way of choosing the backend, which can disagree
with what's installed, and there's no case where the owner wants the remux while the in-place tool
is present.

### D3 — What changes and what doesn't, stated honestly

**Changes:** MKV writes now edit the temp copy in place instead of remuxing it.

- The container structure, streams, chapters, editions, track-level tags and non-cover attachments
  are no longer rebuilt, so they survive **byte-for-byte**.
- The attached-pic and attachment-index failure classes (HOLODEX-413/484) no longer apply to the
  production path.
- The global `TAGS` element is merged via `mkvextract` + `mergeTagsXML`, not `-map_metadata 0`.
  Existing Simple tags survive verbatim, and a re-written field replaces its old Simple
  case-insensitively, so a legacy `Year` written by ffmpeg becomes `YEAR`.

**Does not change:**

- ADR-041's **copy → write → rename**. The job still copies the whole file to `.holodex-tmp` first,
  so a write still needs **free space equal to the file's size** and still does a full-file read
  and write. Per write, the I/O is about the same as the `-c copy` remux. The gain is fidelity and
  fewer failure modes, not speed or disk space. (Editing the original in place would save the copy
  but breaks the atomicity guarantee, and is not proposed.)
- The post-write re-probe (ADR-073) and the readers (`exiftool`/`ffprobe`). WebM goes through the
  same backend.

### D4 — Verify against the built image, because CI doesn't run integration tests

`ci.yml` has its `-tags integration` job commented out. So the verification that counts runs the
writeback integration suite **inside the built runtime image**:

- `make test-image` builds the image and compiles the integration tests into a static test binary
  (`CGO_ENABLED=0 go test -c -tags integration`).
- It runs that binary in the image with the source mounted. Every tool the tests shell out to is
  then the image's own: ffmpeg 5.1, exiftool, MKVToolNix.

With MKVToolNix present, the existing genre/edition/part round-trip tests exercise the mkvpropedit
backend for MKV, and HOLODEX-484/485's cover tests exercise both backends explicitly. Run it before
any release that changes the image or `internal/writeback`. Wiring it into CI is a follow-up
(HOLODEX-487).

### D5 — Close the one latent crash the switch makes reachable

A Title field carrying ADR-110 `Delete` would index an empty `Values` slice on the mkvpropedit path.
No producer emits one today, since ADR-110 deletes are genres/tag-key only. But that path becomes
the production path, so it deletes the Segment Info title (`--edit info --delete title`) instead of
panicking.

## Alternatives considered

- **Keep the ffmpeg path only; drop the mkvpropedit backend.** This removes a dependency, but keeps
  the fidelity loss and the attachment failure classes on every MKV write. Rejected: the owner's
  library is MKV-heavy (subtitled AMVs with font attachments), which is exactly what a remux handles
  worst.
- **Install MKVToolNix from the upstream `mkvtoolnix.download` apt repo** (a newer version).
  Rejected: it adds a third-party signing key and repo to the image for no feature we need. Debian's
  v74 supports everything used here (`-J` identification, `=UID` selectors, `--tags global:`).
- **Static upstream AppImage/binaries.** Rejected: no Debian security updates, and an extra download
  step in the build.

## Consequences

- The image grows by about 37 MB, on top of ADR-007's ~180 MB estimate.
- The MKV write path in production changes for the first time since writeback shipped. The
  `test-image` run in D4 is the gate for that. Rollback is D2's package removal.
- MKVToolNix joins ffmpeg and exiftool as a parser of untrusted media inside the container. It runs
  in the same process context, arguments are passed as argv with no shell, and paths are absolute
  temp paths under the media root.
- The ffmpeg MKV path becomes the **fallback** in production. It stays under test (HOLODEX-484) so a
  rollback lands on known-good code.
