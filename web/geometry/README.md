# Geometry assertion harness (HOLODEX-349)

Layout invariants, measured in a real browser against the [stress
fixture](../../testdata/stressseed/README.md), across four viewport widths.

```bash
# 0. once per machine — `npm ci` installs no browser binaries (playwright ships no
#    install script), and the harness only discovers that after preflight has passed.
npm --prefix web exec -- playwright install chromium

# 1. seed the fixture
go run ./testdata/stressseed

# 2. start the `backend-stress`, `enrich-stub` and `web` launch profiles

# 3. measure — every line runs from the repository root and leaves your shell there.
#    (`cd web && npm run geometry` works too, but a failing preflight then strands you in
#    web/, and PowerShell has no `( … )` subshell to undo that.)
npm --prefix web run geometry
npm --prefix web run geometry -- --list             # what would be measured, and where
npm --prefix web run geometry -- --only person-tiles-stay-legible
npm --prefix web run geometry -- --width phone --headed
```

Exit code 0 means every invariant holds. Anything else names what broke and where.

**On Windows, run the `web` dev server on Node 24.16.0 or later.** Node 24.0–24.15 bundle
a libuv whose TCP-connect path overruns a stack buffer at random
([libuv#5106](https://github.com/libuv/libuv/issues/5106), fixed in
[Node 24.16.0](https://github.com/nodejs/node/pull/62561)); the Vite dev proxy opens one
outbound connection per `/api` request, so the full matrix's ~640 page loads reliably kill
the server partway through — it exits `0xC0000409` with no output — while a `--only` run
makes too few connections to hit it (HOLODEX-381). The harness stops at the first refused
connection and exits 2 with this diagnosis rather than scoring the rest of the matrix as
errors.

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
10/25/50, at every width — six pages × four cells — and it picks up an 80-person rung
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
| `measure` | `width` · `height` · `overflowX` · `overflowY` · `fontSize` · `gutterRight` (viewport right edge − element right edge) · `mainTop` (element top − `<main>` top, so the site header's wrap never moves it) |
| `widths` | Width keys to run at (`wide`, `lg`, `narrow`, `phone`). Default: all. For an invariant that only exists at one width — the phone-only list layout — so it is never measured, and never vacuously passed, where that layout doesn't apply. |
| `expect` | `{ min }`, `{ max }`, or both. Inclusive. |
| `applies` | `each` (default) bounds every match; `count` bounds how many matched. |
| `atLeast` | Matches required before the assertion means anything. Default 1. |
| `prepare` | `metadata-fold`, `source-badge:<field>`, `visitor-view`, `enrich-picker-open:<provider>`, `person-card-open` — subtrees that exist only after an interaction (two folds, the role switch, the Enrich picker opened on the stub's ten-entry `searched[]` cascade, and the person hover card on the right-most People tile). |
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

Four widths (1440, 1024, 768 and a 375px `phone`, added for F73's list toolbar,
HOLODEX-472) = four cells, keyed by width alone.

There is no skin axis: Cinémathèque is the only look (ADR-115, HOLODEX-476), so nothing
selects or awaits one. Widths matter because column counts are width-derived (`density.svelte.ts`) and `.stage-grid`
collapses to one column below `lg`. Below `sm` (the `phone` cell) the list toolbar changes
shape: icon-only controls, the Filters sheet, the right-edge A–Z rail.

## What holds the page still

All in [`browser.mjs`](browser.mjs), and every one of them was a real hazard:

- **Reduced motion is emulated.** The staggered `reel-rise` mount animation translates
  every grid child 8px for up to a quarter second; the animation is gated on
  `prefers-reduced-motion: no-preference`, so emulating the preference removes the race
  rather than sleeping through it.
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
