# ADR-011: Symlink Handling & Path Resolution

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

The scanner walks `MEDIA_PATH` recursively. Real-world media libraries — especially on NAS hardware — frequently use symlinks to stitch a single logical library together from multiple physical drives. The scanner must support this without (a) hanging on symlink loops, (b) double-indexing a file reachable by more than one path, or (c) opening a path-traversal hole.

## Decision

**Follow symlinks by default** (configurable via `FOLLOW_SYMLINKS`, default `true`), resolving every entry to its **canonical real path** and indexing each real file exactly once.

### Resolution & dedup
- Each discovered entry is resolved with `filepath.EvalSymlinks` to a canonical absolute path.
- A **visited-set of canonical paths** is maintained for the duration of a scan. An entry whose canonical path is already in the set is skipped.
- The **canonical path is what gets stored** as `Video.file_path`, so playback/serving is stable regardless of which symlink first surfaced the file.

### Loop protection
- The canonical-path visited-set makes loop detection automatic: a cyclic symlink (`A→B→A`) resolves to an already-visited path and is skipped. A recursion-depth ceiling (`SCAN_MAX_DEPTH`, default 64) is a secondary backstop.

### Targets outside MEDIA_PATH
- Symlinks whose target resolves **outside** `MEDIA_PATH` are **followed and indexed**. Consolidating multiple physical drives into one logical library is a primary supported use case.

### Security model
- The path-traversal NFR is satisfied by **serving files only by video ID**: the API looks up the stored canonical path for an ID and serves that file. Clients never supply a filesystem path, so traversal payloads (`../../etc/passwd`) are structurally impossible — this holds even though indexed files may legitimately live outside `MEDIA_PATH`.

## Configuration

| Env var | Default | Description |
|---------|---------|-------------|
| `FOLLOW_SYMLINKS` | `true` | Follow symlinks during scan (incl. targets outside `MEDIA_PATH`) |
| `SCAN_MAX_DEPTH` | `64` | Recursion-depth backstop for pathological trees |

## Hardlinks

Hardlinks are distinct directory entries pointing to the same inode but with **different canonical paths** (`EvalSymlinks` does not collapse them). Therefore two hardlinks to the same file are indexed as **two `Video` records**. Inode-based dedup (`os.SameFile`) is **deferred** as a future option — it is rarely needed and adds per-entry `stat` overhead.

## Consequences

- A file reachable via several symlinks appears **once** in the library (canonical-path dedup), avoiding duplicate cards.
- Stored paths are canonical, which also makes the `active`/removed-file reconciliation in the rescan logic deterministic.
- When `FOLLOW_SYMLINKS=false`, symlinked entries are skipped entirely (regular files only).
- The visited-set is per-scan in-memory state; for very large libraries it holds one string per unique file (negligible vs. the index itself).
