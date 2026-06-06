# ADR-012: Resolution Classification — Width-Based Buckets with 10% Tolerance

**Status**: Accepted  
**Date**: 2026-06-05  
**Deciders**: Project owner

---

## Context

The browse filter (requirement F4.5) classifies each video into a resolution bucket: SD / HD / FHD / 4K+. Two design questions:

1. **Bucket by height or width?** Height-based bucketing mislabels cinematic content: a 4K film in 2.39:1 is **3840×1606** — its height (1606) is below 1080, so a height rule would file it as FHD despite being a 4K-class master. Plex and Jellyfin both bucket by width to avoid this.
2. **Hard thresholds or tolerance?** Real-world encodes routinely land a hair under the nominal width — a "4K" master cropped to 3792, a "1080p" file at 1888 — and would drop a tier under strict cutoffs.

## Decision

**Bucket by frame width**, using the nominal width of each tier with a **10% downward tolerance** on the lower bound, so near-miss encodes round up to their intended tier.

### Nominal tier widths
| Tier | Nominal width |
|------|--------------|
| HD (720p) | 1280 |
| FHD (1080p) | 1920 |
| 4K+ (UHD) | 3840 |

### Effective cutoffs (nominal − 10%)
| Bucket | Width range (px) |
|--------|------------------|
| **SD** | `width < 1152` |
| **HD** | `1152 ≤ width < 1728` |
| **FHD** | `1728 ≤ width < 3456` |
| **4K+** | `width ≥ 3456` |

(`1152 = 1280×0.9`, `1728 = 1920×0.9`, `3456 = 3840×0.9`.)

### Worked examples
| File | Width | Bucket |
|------|-------|--------|
| 3840×2160 (UHD) | 3840 | 4K+ |
| 3792×1600 (cropped 4K scope) | 3792 | 4K+ ✓ (would be FHD without tolerance) |
| 1920×1080 (FHD) | 1920 | FHD |
| 1888×1062 (slightly under FHD) | 1888 | FHD ✓ |
| 1280×720 (HD) | 1280 | HD |
| 1152×480 | 1152 | HD ✓ |
| 854×480 (SD) | 854 | SD |

## Rationale

- **Width is aspect-ratio robust.** It correctly tiers letterboxed/cinemascope content that height-based rules demote.
- **10% tolerance matches encoder reality.** Mastering crops, mod-16 alignment, and re-encodes frequently shave a few percent off the nominal width; the tolerance keeps those files in the tier a human would assign.
- **Matches user expectation from mainstream media servers** (Plex/Jellyfin width-based labeling).

## Consequences

- `width` and `height` are stored per video (already in the data model); the bucket is **computed**, not stored — so the thresholds can be tuned later without re-indexing.
- The filter query (`?resolution=4K`) translates to a width range predicate (e.g. `width >= 3456`), which is index-friendly.
- The four-bucket set is fixed for v1; a future QHD/1440p split (`2304 ≤ width < 3456`) can be added without schema change if demand arises.
- These thresholds are constants in one place in the classification function; if exposed as configuration later it is a non-breaking addition.
