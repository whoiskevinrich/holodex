---
# Flightplan worklog — one epic, one worklog, one definition of done.
# Schema: ../README.md · design: ../../docs/architecture/ADR-064-flightplan-plugin.md
key: HOLODEX-388
status: in-progress
release_note: Removing a film's banner or poster now works when the image came from a metadata provider — the Remove control clears the whole slot instead of silently doing nothing.
---

# HOLODEX-388 · Film image Remove is a silent no-op for a provider-sourced banner/poster

The Film detail page's × ("Remove banner") confirmed, returned 204, reloaded — and the banner
stayed. `deleteFilmImage` deleted only the `source = 'upload'` row; a banner adopted from TMDB
enrichment lives under `source = 'provider:tmdb'` (ADR-086 §2's coexisting rows), so the DELETE
matched nothing and the idempotent 204 masked it. **Remove now clears the whole role slot** —
every source's row plus its file — matching Studio's "delete unlocks the slot for the next
enrich" rule. Not the alternative (remove only the displayed row, upload peels back to reveal the
provider image): that makes the × need two clicks to empty a role, the same confusion in a
different costume.

## Gates — definition of done

- [~] spec `write-spec` — n/a: bug fix; the documented intent (Remove empties the slot) is unchanged
- [~] architecture `architecture` — n/a: ADR-086 §2 unchanged; delete semantics were an implementation gap
- [~] design `design-handoff` — n/a: no UI change; `EntityImageSlot` already wires × → DELETE → reload
- [x] backend — `repo.DeleteFilmImageRole` (role-wide, returns removed ids) + `deleteFilmImage`
  removes every file; `DeleteFilmImage`'s per-source form stays for the enrichment sink
- [~] frontend — n/a
- [x] testing `testing-strategy` — two scenarios added; `TestFilmImage_DeleteClearsProviderRow`
  (provider-only slot + upload/provider pair) — confirmed red against the old handler
  (`200, want 404 — provider row survived`), green with the fix
- [~] security `security-review` — n/a: no auth/access/infra change; same owner-gated route

## Up next — ordered (position = priority)

1. [ ] [—] Poster role has the same latent bug and the same fix — nothing further to do, but QA it
   once on a real provider-enriched film

## Session log — append-only (cap: last 8 sessions; older → archive/)

### 2026-09-16 · diagnosed, fixed, tested
- skills: debug, code-review high --fix
- handoff: PR open on `HOLODEX-388-film-image-remove-provider-row`; all gates green, marked ready
  for review; nothing open on the branch.
