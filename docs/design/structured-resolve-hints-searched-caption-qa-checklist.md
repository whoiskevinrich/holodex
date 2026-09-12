# QA Checklist: Enrich picker "Searched" caption (HOLODEX-369)

**Spec**: [configurable-provider-search-patterns.md](../specs/configurable-provider-search-patterns.md) FR9 ·
**Handoff**: [structured-resolve-hints-searched-caption-handoff.md](structured-resolve-hints-searched-caption-handoff.md) ·
**ADR**: [ADR-095](../architecture/ADR-095-structured-resolve-hints.md) D6 ·
**Jira**: HOLODEX-369 (epic HOLODEX-367)

Conventions: every item is numbered `section.item` and tagged by verifier —
`[smoke]` automated tests, `[agent]` agent-driven live QA, `[human]` needs human eyes.

---

## §1 Setup

- **1.1** `[agent]` Start `backend-films` and `web` (see [[reference-holodex-preview-testbeds]]) with
  the in-process `enrich.Fake` provider configured for `video`. Owner view on — the picker is owner-only.
- **1.2** `[agent]` Give the fake three canned `/resolve` responses, selectable by query text:
  (a) 2 candidates + `searched: ["<basename>", "Acme Pictures Ada Lovelace"]`;
  (b) 0 candidates + the same 2-entry `searched`;
  (c) 0 candidates + a 10-entry `searched` whose second entry is ~600 chars.
  Plus (d) a response with **no** `searched` key at all.
- **1.3** `[agent]` Pick one video whose basename has brackets and parens so the first entry
  exercises truncation at the dialog's `max-w-lg`.

## §2 Smoke — `[smoke]` (existing `EnrichPicker` test file; no new harness)

- **2.1** Response (d): no caption element in the DOM — not an empty `<p>`.
- **2.2** Response with `searched: ["one"]`: label + the query, **no** toggle button.
- **2.3** Response (a): label + first entry + a button reading `+1 more`, `aria-expanded="false"`,
  `aria-controls` pointing at an `<ol>` that is not rendered/hidden.
- **2.4** Activate the toggle: `aria-expanded="true"`, button reads `show less`, `<ol>` has 2 `<li>`
  in issue order, the first `<li>` equals `searched[0]`.
- **2.5** Type one character into the box: caption gone immediately (before the debounce fires);
  next response repopulates it collapsed (`showAll` reset).
- **2.6** Response (b): caption present on the no-results state, below the
  `No matches for "…"` status line.
- **2.7** `loading` true: caption absent. `error` set: caption absent.
- **2.8** Stale-response guard: an older resolve resolving after a newer one must not overwrite
  `searched` (same `searchId` check that protects `candidates`).
- **2.9** The status `<p>` (`aria-live="polite"`) does **not** contain the caption text.
- **2.10** `enrichVideoResolve`'s typed return includes `searched?: string[]`; person/studio/film
  resolve callers compile unchanged.

## §3 Agent — live `[agent]`

- **3.1** `[smoke]` *(was `[agent]`; automated by HOLODEX-372 — `cd web && npm run geometry --only searched-list-scrolls-inside-its-cap …`, five assertions under `enrich-picker-open:flood`)* Response (c) expanded: `<ol>` scrolls (scrollHeight > clientHeight), the dialog's
  total height does not exceed 80 vh, the candidates `<ul>` is still laid out (not squeezed to 0).
- **3.2** `[agent]` The 600-char entry: inline line and its `<li>` both have `title` equal to the
  full string and render on one line (`getBoundingClientRect().height` ≈ one text line).
- **3.3** `[agent]` Tab order with response (a): search box → `+1 more` → active result row → None of
  these match → ✕; Shift+Tab reverses; Tab from ✕ wraps to the search box (trap intact).
- **3.4** `[agent]` Tab order with `searched.length === 1`: the toggle is absent from the order.
- **3.5** `[agent]` Three skins, via `javascript_tool` computed styles (screenshots time out here —
  see [[reference-holodex-skin-qa-without-screenshots]]): label/toggle color equals `--muted`, query
  color equals `--ink`, on Cinémathèque / Broadcast / Brutalist; contrast of `--muted` on
  `--surface` ≥ 4.5:1 in each.
- **3.6** `[agent]` No horizontal overflow: `document.documentElement.scrollWidth ===
  clientWidth` with response (c) expanded, in all three skins.
- **3.7** `[agent]` Batch path: run refresh-all on the same video with response (c); the Activity
  page's detail line for that run contains `searched:` followed by the entries joined with ` · `,
  and contains no `/` or `\` path separator.
- **3.8** `[agent]` Reduced motion: toggling with `prefers-reduced-motion: reduce` emulated behaves
  identically (there is no transition to remove — confirm none was added).

## §4 Human — `[human]`

Open a video's page as the owner, click **Enrich** for the fake provider, and search for something
that returns nothing.

- **4.1** `[human]` Under "No matches for …" there is a second, quieter line starting **Searched**
  followed by the file's own name in brighter text and a small **+1 more**. It should read as an
  explanation of the line above it, not as a new control panel — same size as the status line, no
  icon, no box.
- **4.2** `[human]` Click **+1 more**: a short numbered list drops down; the first item is the same
  filename you already saw. Click **show less**: it folds back. Nothing else on the dialog moved.
- **4.3** `[human]` Now search for something that *does* match. The **Searched** line is in the same
  place, just above the results — it did not jump to the bottom.
- **4.4** `[human]` Type one letter in the box. The **Searched** line disappears at once and comes
  back (for the new search) about a third of a second later.
- **4.5** `[human]` Switch skins with the header picker (Cinémathèque → Broadcast → Brutalist). The
  line stays legible in each; in the two monospace skins the filename lines up cleanly. Nothing looks
  like a hardcoded color that ignored the skin.
- **4.6** `[human]` With the ten-query fixture, expand the list: it shows about four lines and
  scrolls inside itself; the dialog does not grow off the bottom of the window.
