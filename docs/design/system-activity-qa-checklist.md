# QA Checklist: System Activity (F21)

Work through this against a running app. Dev: backend on `:7800`
(`go run ./cmd/holodex` with a media path), frontend on `:5173` (`npm --prefix web run dev`),
then open **http://localhost:5173/status**.

Legend: **[auto]** = already verified programmatically this session · **[eye]** = needs a human look.

## Reachability & shell
- [x] **[auto]** `/status` loads (HTTP 200, no console errors after a clean dev-server start).
- [x] **[auto]** "Status" appears in the header nav.
- [x] **[eye]** Page header: "System Activity" title + subtitle render.

## Status cards (live data)
- [x] **[auto]** Scan card shows Idle/Running, last-run time + duration, next-scheduled, and added/updated/removed chips.
- [x] **[auto]** Thumbnails card shows depth / in-flight / high / normal / workers.
- [x] **[auto]** Library card shows active videos (+ inactive), people, tags (showed 202 / 17 / 14).
- [x] **[auto]** System card shows Ready, uptime, version, media-path set/missing.
- [x] **[eye]** Numbers update on their own within a few seconds (3s poll) — e.g. uptime ticks.
- [x] **[eye]** Drop a file into the media dir → within a scan cycle, Library count rises and a new history row appears.

## Header activity indicator
- [x] **[eye]** While a scan is running **or** the thumbnail queue is non-empty, the pill appears in the header with a pulsing dot ("Indexing…" / "N thumbnails").
- [x] **[eye]** It disappears when work finishes; clicking it lands on `/status`.
- [x] **[eye]** Indicator shows on other pages too (Media/People/…), not just `/status`.
  > Note: with a small/clean library, scans finish in ~25ms, so the running state can be too brief to catch — to force it, point the backend at a large/changed library or temporarily slow the scan.

## Controls (owner)
- [x] **[auto]** "Rescan library" → inline confirm ("Rescan the whole library?") → "Yes, rescan" → toast "Scan started."
- [x] **[eye]** "Cancel" dismisses the confirm without triggering a scan.
- [x] **[eye]** "Reload config" → toast "Config reloaded — N fields."
- [x] **[eye]** Triggering rescan while one is running shows "A scan is already running." (informational, not an error).

## Job history
- [x] **[auto]** Table lists recent runs newest-first with When/Trigger/Duration/Added/Updated/Removed/Errors/Status.
- [x] **[eye]** A run with errors shows an accent-ringed "error" status + the error message sub-row (hard to stage; verify if you have a failing file).
- [x] **[eye]** Empty state ("No scans recorded yet.") on a brand-new DB.

## Gating (F21.7) — needs `ADMIN_TOKEN` to exercise
- [x] **[auto]** Open mode (no token): controls visible; `controls_unauthenticated` banner shows on a non-loopback bind.
- [x] **[eye]** Start the backend with `ADMIN_TOKEN=secret` **on loopback** (`--host 127.0.0.1`): `/status` cards still load (reads gated → 401 until unlocked), controls hidden, a token input ("This server requires an admin token") appears.
- [x] **[eye]** Enter the wrong token → still locked; enter the right token → controls appear, cards/history load.
- [x] **[eye]** With a token set + loopback bind, the `controls_unauthenticated` banner is **absent**.
- [x] **[eye]** With `ADMIN_TOKEN` set, the backend startup log does **not** warn; with it unset on a non-loopback bind, it **does** warn.

## Theming — all three skins (CLAUDE.md)
- [x] **[auto]** Renders in **Cinémathèque** (gold, serif, soft radius), **Broadcast** (cyan, square), **Brutalist** (lime, mono, square) — screenshots captured.
- [x] **[eye]** In each skin: the Rescan button (bg-accent) and the accent-ringed error/banner read legibly against the surface; nothing collides.
- [x] **[eye]** The pulsing dot is visible against each skin's header/surface.
- [x] **[eye]** Token-only check stays clean: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` is empty.

## States & responsive
- [x] **[eye]** Loading state ("Loading activity…") on first paint before data arrives.
- [x] **[eye]** Error state (themed box) when the backend is down — stop the backend and reload.
- [x] **[eye]** Reduced motion: with OS "reduce motion" on, the dot doesn't pulse (static accent).
- [x] **[eye]** Mobile width (~375px): cards stack to 1 column; the history table scrolls horizontally; the indicator label hides (dot only).

## Accessibility
- [ ] **[eye]** Keyboard: Tab to the controls and the token input; Enter activates; the rescan confirm is reachable.
- [x] **[auto]** No a11y warnings from `svelte-check` (the `<a role=status>` warning was fixed).
