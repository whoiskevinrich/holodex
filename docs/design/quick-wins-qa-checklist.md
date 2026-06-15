# QA Checklist: Quick Wins batch (overlay fix · search history · "More with…" · fluid Back)

Work through this against a running app. Dev: backend on `:7800`
(`go run ./cmd/holodex --media-path <dir> --host 127.0.0.1`), frontend on `:5173`
(`npm --prefix web run dev`), then open **http://localhost:5173/**.

Spec [`quick-wins.md`](../specs/quick-wins.md) · design handoff [`quick-wins-handoff.md`](quick-wins-handoff.md) · [ADR-031](../architecture/ADR-031-related-media-endpoint.md) / [ADR-032](../architecture/ADR-032-browse-state-preservation.md).

Legend: **[auto]** = verified programmatically this session (`preview_eval` / screenshots / tests) · **[eye]** = needs a human look.

---

## 1. Overlay on playback (media detail page)
- [x] **[auto]** Playing the media `<video>` adds `body.is-playing` and `.app-atmosphere::after` computes `display:none`; pausing/ending removes the class and restores it (verified in **Broadcast**, the worst case — scanlines + vignette).
- [x] **[eye]** **Broadcast**: during playback the picture is fully clean (no scanlines, no CRT vignette over the frame); both return on pause/end.
- [ ] **[eye]** **Cinémathèque**: grain + vignette gone during playback, restored on pause.
- [ ] **[eye]** **Brutalist**: no atmosphere to begin with — confirm play/pause causes no flicker or layout shift.
- [ ] **[eye]** Start play → hit ← Back **mid-play** → the overlay is restored on the grid (no stuck-hidden atmosphere).
- [ ] **[eye]** The codec-fallback branch (a file the browser can't decode) has no `<video>` and behaves normally (overlay unaffected).

## 2. Search-history dropdown (header)
- [x] **[auto]** Focusing the **empty** box opens the dropdown listing recent queries (most-recent-first); typing a character hides it; clearing the box re-opens it.
- [x] **[auto]** Clicking a row runs the search (URL becomes `/search?q=…`, syntax like `editor:foo` preserved).
- [x] **[auto]** ↓/↑ move a highlight; the input's `aria-activedescendant` tracks the highlighted `<option>` (combobox a11y); Esc closes.
- [x] **[auto]** `×` removes a single entry (order preserved); "Clear history" empties the list and the `localStorage` key.
- [x] **[auto]** **Broadcast/Brutalist**: panel + rows are square (`--radius:0`); **no `▮` caret** on any row (rows aren't `.skin-title`).
- [ ] **[eye]** **Cinémathèque**: rounded panel/rows; focus accent border reads; active row (`bg-surface-2`) is distinguishable from the panel (`bg-surface`).
- [ ] **[eye]** Re-running an existing query moves it to the top (no duplicate, case-insensitive); the list never exceeds 10.
- [ ] **[eye]** Corrupt the `localStorage` value by hand → reload → search still works, history reads empty (no thrown error).
- [ ] **[eye]** Ctrl/Cmd-K focuses the search and opens the dropdown when history is non-empty.

## 3. "More with …" shelves (media detail page)
- [x] **[auto]** On an item with shared people/tags, a "More with `<person>`" shelf (≤5 cards) and a "More with `<tag>`" shelf render below the detail; headings link to `/people/{id}` / `/tags/{id}`; current item is excluded.
- [x] **[auto]** `GET /api/v1/media/{id}/related` returns the expected shape; near-universal tag is demoted in favor of the distinctive one (repo + handler tests green).
- [x] **[auto]** **Brutalist**: the catalog counter **restarts at 01 per shelf** (person shelf 01…, tag shelf 01… — not continuing), because each shelf wraps its cards in `.video-grid`.
- [x] **[auto]** **Cinémathèque**: letterbox bars render on shelf cards (`.video-frame::before` = 9px).
- [ ] **[eye]** **Broadcast**: scanline wash reads over shelf cards; the `▮` caret sits after the shelf heading (it *is* a `.skin-title`) without crowding the link.
- [ ] **[eye]** An item with **no people** shows no person shelf; **no tags** shows no tag shelf; an item whose people/tags have no siblings shows **no empty rail** (shelf omitted, not a blank box).
- [ ] **[eye]** Cards are visually identical to the browse grid; horizontal scroll works; clicking a card opens its detail page.
- [ ] **[eye]** Shelves load **non-blocking** — the primary detail content is fully usable before `/related` resolves; if `/related` fails, the page is unaffected (shelves just absent).
- [ ] **[eye]** Switching skin or regenerating the thumbnail on the page does **not** reshuffle the shelves (stable per page view); opening the item fresh draws a new set.

## 4. Fluid Back (browse grid)
- [x] **[auto]** Scroll the grid, "Load more" ×2 (≈100+ cards), open an item, press **Back** → all loaded cards restored, scroll position restored (±few px), **zero `GET /api/v1/media`** fired, **no `Loading…` flash**.
- [x] **[auto]** Changing a filter (e.g. Resolution = HD) refetches the filtered set, syncs the URL (`/?resolution=HD`), and resets scroll to the top.
- [x] **[auto]** Hard reload of a filtered URL rebuilds from page 0 at the top (no stale restore); filter state restored from the URL.
- [ ] **[eye]** It *feels* instant — no visible jump-to-top-then-settle, no spinner on Back.
- [ ] **[eye]** Deep-link/share still works: paste a `/?…` URL in a fresh tab → the filters reproduce.
- [ ] **[eye]** No SvelteKit router warning in the console (the raw `history.replaceState` → `$app/navigation` `replaceState` fix).

## 5. Cross-cutting
- [ ] **[eye]** Token-only check stays clean: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` is empty.
- [x] **[auto]** No console errors/warnings across the session; `svelte-check` 0 errors; `go test ./...` green.
- [ ] **[eye]** Mobile width (~375px): the search dropdown fits the form width; shelves scroll horizontally; the grid reflows to 1 column.
- [ ] **[eye]** Reduced motion: nothing in these surfaces animates against an OS "reduce motion" preference (shelves/dropdown have no motion of their own).
