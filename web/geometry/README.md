# Geometry assertion harness (HOLODEX-349)

Layout invariants, measured in a real browser against the [stress
fixture](../../testdata/stressseed/README.md), across three skins and three viewport widths.

```bash
# 0. once per machine — `npm ci` installs no browser binaries (playwright ships no
#    install script), and the harness only discovers that after preflight has passed.
#    In a subshell so the whole block stays paste-able from the repository root.
(cd web && npx playwright install chromium)

# 1. seed the fixture
go run ./testdata/stressseed

# 2. start the `backend-stress` and `web` launch profiles

# 3. measure
cd web && npm run geometry
npm run geometry -- --list                          # what would be measured, and where
npm run geometry -- --only person-tiles-stay-legible
npm run geometry -- --skin brutalist --width narrow --headed
```

Exit code 0 means every invariant holds. Anything else names what broke and where.

## Why this and not screenshots

Spec [D6](../../docs/specs/stress-fixture.md). The owner looks at the fixture and says
*"media 902's headshots are unusably small"*. What gets written back is not a picture of
media 902 and not an assertion about media 902 — it is **an invariant about any page with
that property**:

```js
when: (e) => (e.axes.video?.people ?? e.axes.film?.cast ?? 0) >= 10,
selector: 'li.curation-chip .portrait-frame--2x3',
measure: 'width',
expect: { min: 40 }
```

That one entry covers the `people` rungs at 10/25/50 **and** the `filmcast` rungs at
10/25/50, in six skin/width combinations — 36 checks — and it picks up an 80-person rung
the day somebody adds one, without being edited. A screenshot diff would have pinned one
page, needed byte-stable images, and gone red on every restyle. It is also the only
technique available here: browser screenshots time out on Holodex, so `getBoundingClientRect`
plus computed style is the established measurement method.

## Writing an assertion

Add an entry to [`assertions.mjs`](assertions.mjs) — that file is the deliverable,
everything else is machinery. `docs/testing-strategy.md` §11 covers *when* one is worth
adding.

| field | |
|---|---|
| `key` | Stable slug; names it in the report. |
| `finds` | What breaking this looks like to a human. Required — a failure has to explain itself. |
| `when(entry)` | Selects pages by their manifest coordinate. **Write the property, not the id.** |
| `urls` | …or literal pages, for surfaces the manifest does not address (the list pages). Exactly one of `when` / `urls`. |
| `selector` | CSS, or `:document` for the page itself. |
| `measure` | `width` · `height` · `overflowX` · `overflowY` · `fontSize` |
| `expect` | `{ min }`, `{ max }`, or both. Inclusive. |
| `applies` | `each` (default) bounds every match; `count` bounds how many matched. |
| `atLeast` | Matches required before the assertion means anything. Default 1. |
| `prepare` | `metadata-fold`, `source-badge:<field>` — subtrees that exist only after an interaction. |
| `requires` | `{why, met(manifest)}` — skip, with the reason printed, when the fixture is the wrong shape. |
| `blockedBy` | A filed, unfixed ticket. Reported but does not fail the run. |

`when` reads the entry's `axes`, which the seeder writes into
`data/stress/manifest.json`. Only one kind-keyed half is populated per entity —
`axes.video`, `axes.film` or `axes.name`, plus `axes.image` alongside — so a predicate
that reaches into the wrong half simply does not select that entity rather than throwing.

## Four things it refuses to do quietly

**Pass on nothing.** "Every matched element is at least 40px" is trivially true of zero
elements, so a renamed class would turn a real assertion green. A selector matching fewer
than `atLeast` elements is reported as `VOID`, not as a pass. This fired on the harness's
own first live run and was a genuine defect in it.

**Select nothing.** An assertion whose `when` matches no page in the fixture is reported
too — it means the coordinate it asks for is gone, and coverage evaporated silently.

**Measure the wrong server.** Preflight checks `/capabilities` for `owner` and
`films_enabled`, then fetches one seeded entity and compares its title to the manifest.
Every backend profile binds `:7800`, and a dev server in a worktree without its own
`.claude/launch.json` will happily serve a different checkout.

**Call a known-open bug a regression.** `blockedBy` keeps the run green while a filed bug
is open — but the moment *every* check under that marker passes, the run goes red with
`NEWS`, because a stale marker is how a fixed bug stops being guarded. That verdict is
reached over the whole assertion, never per page: while a bug is open most pages still
pass, and scoring staleness per page produced 101 false alarms before it was corrected.

## The run matrix

Three skins × two widths = six cells, ~45 seconds for the current table.

Skins are not a re-paint: each changes `--radius` and the display font, so text metrics
and wrapping differ, and Broadcast appends a `▮` glyph to every `.skin-title`. Widths
matter because column counts are width-derived (`density.svelte.ts`) and `.stage-grid`
collapses to one column below `lg`.

## What holds the page still

All in [`browser.mjs`](browser.mjs), and every one of them was a real hazard:

- **Reduced motion is emulated.** The staggered `reel-rise` mount animation translates
  every grid child 8px for up to a quarter second; the animation is gated on
  `prefers-reduced-motion: no-preference`, so emulating the preference removes the race
  rather than sleeping through it.
- **The skin is set before the page loads,** via an init script writing `localStorage`.
  Poking `data-theme` after hydration would leave `PersonImageFrame`'s `?skin=` image URLs
  on the previous skin.
- **`/capabilities` is awaited.** Owner-gated surfaces — the whole metadata list, the chip
  row — mount only after it answers, so measuring before it does reports an empty visitor
  page.
- **`networkidle` is not used.** `VideoCard` retries a missing thumbnail with backoff
  reaching ~30s, so a fresh fixture keeps the network busy long after the layout settles.
- **Density is pinned** so viewport width is the only variable in a column count.

## Files

| | |
|---|---|
| [`assertions.mjs`](assertions.mjs) | The table. This is the file you edit. |
| [`manifest.mjs`](manifest.mjs) | Reads the seed manifest; resolves `when` to pages. |
| [`evaluate.mjs`](evaluate.mjs) | Measurements → verdicts. The vacuity guard and `blockedBy` live here. |
| [`browser.mjs`](browser.mjs) | Playwright: the run matrix, page preparation, the probe. |
| [`report.mjs`](report.mjs) | Failure output and the exit code. |
| [`run.mjs`](run.mjs) | CLI: validate, plan, preflight, measure, report. |

The pure halves are unit-tested under `npm run test` (vitest) — no browser, no server.
