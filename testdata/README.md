# Test Fixtures

Deterministic media corpus for Holodex's extraction/classification tests
(see [`docs/testing-strategy.md`](../docs/testing-strategy.md) §3).

Two generators live here. `gen.sh` (below) builds the **media** corpus; `aliasseed/`
builds **database state** — the F58/ADR-088 alias states (provider badge, collision
review line, suppression) that are tedious to stage by hand. See
[`aliasseed/README.md`](aliasseed/README.md).

## Regenerate

```bash
./testdata/gen.sh
```

Requires `ffmpeg`, `exiftool`, and `mkvtoolnix` (`mkvpropedit`). On the CI/test
image these are installed on top of the runtime image (ADR-007).

## What's in the corpus

| File | Exercises |
|------|-----------|
| `mp4/fhd_full.mp4` | All six core fields (F2.1–F2.6) |
| `mp4/publisher.mp4` | `Publisher` extended tag → mapping source key (ADR-013) |
| `mp4/nometa.mp4` | Filename/mtime fallback (F2.7) |
| `mp4/scope4k.mp4` | 3840×1606 → width-based 4K+ (ADR-012) |
| `mp4/nearmiss_fhd.mp4` | 1888 wide → FHD via 10% tolerance (ADR-012) |
| `mp4/unicode.mp4` | "Amélie" → FTS diacritic folding (ADR-017) |
| `mp4/withart.mp4` | Embedded cover art → Tier-1 thumbnail (ADR-009) |
| `mkv/multilevel.mkv` | TITLE at level 50 + 70, ACTOR at 50, **track title to ignore** (ADR-010) |
| `mp4/corrupt.mp4` | Unreadable file → skipped + logged, scan continues (NFR) |
| `mp4/empty.mp4` | Zero-byte → graceful skip |

## Golden files

Generated media is git-ignored (`.gitignore`); the **generator and the
`*.golden.json` expectations are committed**. Extraction tests compare output to
the goldens; regenerate intentional changes with:

```bash
go test ./internal/metadata -run Extract -update
```

> The `mkv/multilevel.mkv` case is the key adversarial fixture: `Episode 1`
> (level 50) must win the title; `The Collection` (level 70) and the
> `Director Commentary` track name must **not** become the title, and the track
> name must not leak into people/tags.
