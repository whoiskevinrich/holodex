# ADR-004: Metadata Extraction — ffprobe + exiftool + ffmpeg

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

Holodex must extract title, people/cast, tags/genres, duration, resolution, date, and technical codec detail from MP4 and MKV files. These formats have divergent tag schemas, and different encoders (Handbrake, MakeMKV, mkvmerge, various hardware encoders) write metadata to different tag locations within the same container format. No single tool reads all of them reliably.

Three tools were identified as complementary rather than competing:

| Tool | Strength |
|------|----------|
| **ffprobe** | Technical stream metadata: codec, bitrate, duration, resolution, container format. Reads `format_tags` for common atoms. |
| **exiftool** | Broadest tag coverage across MP4 iTunes atoms, XMP, ID3-in-MP4, and MKV SimpleTag schemas. Most consistent across encoder variations. Handles edge cases ffprobe misses. |
| **ffmpeg** | Required for thumbnail extraction (Phase 2) and metadata writeback (Phase 3). Not used for reading in Phase 1. |

## Decision

Use **all three tools** in a layered extraction pipeline per file:

1. **exiftool** (primary tag source): Title, people/cast, tags/genres, recording date. JSON output (`-json -G1`). Provides the richest and most encoder-agnostic tag coverage for both MP4 and MKV.
2. **ffprobe** (technical metadata + fallback tags): Duration, width, height, video codec, audio codec, bitrate, container. Also used as a fallback for any tag field exiftool does not populate.
3. **ffmpeg** (Phase 2+): Thumbnail frame extraction and metadata writeback. Not invoked during Phase 1 indexing.

## Rationale

- **Tag schema fragmentation**: MP4 files may carry iTunes atoms (`©nam`, `©ART`), XMP sidecars, or encoder-specific atoms depending on origin. MKV files use Matroska SimpleTag blocks with varying `TAGNAME` conventions (`TITLE`, `ARTIST`, `ACTOR`, `GENRE`, etc.). exiftool normalizes these into a consistent JSON output with group-prefixed keys.
- **Encoder variance**: Hardware encoders (e.g. Sony cameras, capture cards) and software encoders (Handbrake, mkvmerge) write tags to different locations within the same format. exiftool's tag database covers thousands of these variants; ffprobe's `format_tags` coverage is shallower.
- **ffprobe for streams**: exiftool does not reliably expose per-stream codec details (video codec, audio codec, bitrate). ffprobe's `-show_streams` output is the authoritative source for technical metadata.
- **ffmpeg already required**: ffmpeg (which bundles ffprobe) is in the Docker image for Phase 2 thumbnail generation. Adding exiftool is the only incremental cost.
- **Writeback consideration (Phase 3)**: exiftool supports writing tags back to MP4 files. For MKV writeback, `mkvpropedit` (part of mkvtoolnix) is more reliable. This ADR scopes Phase 1–2; Phase 3 writeback tooling will be addressed in ADR-041.

## Extraction Pipeline (per file)

```
File
 ├─ exiftool -json -G1 -struct <file>
 │   └─ yields: title, people, tags, genres, date, cover art presence
 │
 ├─ ffprobe -v quiet -print_format json -show_format -show_streams <file>
 │   └─ yields: duration, width, height, video_codec, audio_codec, bitrate,
 │              container, format_tags (fallback for any field exiftool missed)
 │
 └─ merge layer: exiftool values take precedence; ffprobe fills gaps
```

## Field Mapping Reference

| Field | Primary source | Fallback |
|-------|---------------|---------|
| title | exiftool `Title`, `XMP:Title`, `QuickTime:Title`, `Matroska:TITLE` | ffprobe `format_tags.title` |
| people | exiftool `Cast`, `Artist`, `QuickTime:Artist`, `Matroska:ACTOR`, `Matroska:ARTIST` | ffprobe `format_tags.artist` |
| tags/genres | exiftool `Genre`, `Keywords`, `Subject`, `Matroska:GENRE`, `Matroska:KEYWORDS` | ffprobe `format_tags.genre` |
| date | exiftool `DateTimeOriginal`, `CreateDate`, `Matroska:DATE_RECORDED` | ffprobe `format_tags.date`; file mtime as final fallback |
| duration | ffprobe `format.duration` | exiftool `Duration` |
| width / height | ffprobe video stream `width`, `height` | — |
| video codec | ffprobe video stream `codec_name` | — |
| audio codec | ffprobe audio stream `codec_name` | — |
| bitrate | ffprobe `format.bit_rate` | — |
| container | ffprobe `format.format_name` | — |

## Consequences

- Docker image must include both `ffmpeg` (provides `ffprobe`) and `exiftool` (Perl-based; ~30 MB installed).
- Each file indexed requires two subprocess calls. To keep initial scan performance acceptable, the scanner runs configurable parallel workers (`SCAN_WORKERS`, default: 4). At 4 workers, a 10k-file library with ~50ms per file takes ~125 seconds — acceptable for a background job.
- The extraction layer is encapsulated in a `metadata.Extractor` interface in Go, allowing individual tool implementations to be swapped or extended without touching scanner logic.
- exiftool supports batch mode (`-stay_open True`) which keeps a persistent process alive and pipes filenames to it, eliminating per-file process spawn overhead. This should be used for large scans.
