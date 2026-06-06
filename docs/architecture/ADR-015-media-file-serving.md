# ADR-015: Media File Serving — HTTP Range Requests

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

The video detail page renders an inline `<video>` player (F7.2). Browser playback — and any seeking/scrubbing within a video — requires the server to honor HTTP `Range` requests and respond with `206 Partial Content`. A naive "read whole file and write it" handler would break seeking, waste memory on large files, and stall playback start. This is a hard requirement on the file-serving endpoint and must be designed in from the start.

## Decision

Serve media files **by video ID** through a streaming endpoint that fully supports HTTP Range requests.

```
GET /api/v1/media/:id/stream
```

- Look up the video's canonical path by ID (never accept a client-supplied path — ADR-011 security model).
- Use Go's `http.ServeContent`, which natively handles `Range`, `If-Range`, `206 Partial Content`, `Content-Range`, `Accept-Ranges: bytes`, and conditional requests (`ETag`/`Last-Modified`).
- Set `Content-Type` from the container (e.g. `video/mp4`, `video/x-matroska`).

## Rationale

- **Seeking requires Range.** Without `206` support, the `<video>` scrub bar cannot jump; the browser must download linearly.
- **`http.ServeContent` is the idiomatic, correct primitive** — it handles range parsing, multi-range, and validators so we don't hand-roll error-prone byte math.
- **Memory-safe.** Streaming via the `io.ReadSeeker` interface means large 4K files are never buffered whole in memory.
- **Consistent security.** Serving strictly by ID with a path looked up from the index preserves the no-path-traversal guarantee (ADR-011).

## Consequences

- The stream endpoint opens the file as an `*os.File` (an `io.ReadSeeker`) and passes it to `http.ServeContent` with the file's mod time as the validator.
- MKV (`video/x-matroska`) and some codecs may not play in all browsers; the UI falls back to a download/"open file" link when the browser cannot decode (F7.2). This ADR governs *delivery*, not codec compatibility; transcoding remains out of scope (a documented non-goal).
- Thumbnails and images are served by separate static handlers with long-lived `Cache-Control` headers (ADR-009).
- A `Range`-aware endpoint also enables future partial-content use cases (preview scrubbing, Phase 3 trailers).
