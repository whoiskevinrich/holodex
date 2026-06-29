# QA: Granular Metadata Curation & Merge (F30)

**Feature**: cross-source merge + value-level curation (add/edit/suppress/no-write) + durable batch-writeback queue
**Related**: [ADR-046](../architecture/ADR-046-metadata-curation-and-write-queue.md), [spec](metadata-curation.md), [design handoff](../design/metadata-curation-handoff.md)

> Items are numbered `section.item` and tagged by verifier: **[smoke]** (build/unit, no
> running app), **[agent]** (scriptable against a running server), **[human]** (eyeball in the
> browser). Sections are grouped by tag.

---

## 0. Setup

**0.1** [smoke] `go build ./...` — clean build.
**0.2** [smoke] `cd web && npm run check` — 0 errors (pre-existing `WritebackFormDialog` a11y warnings are acceptable).
**0.3** [human] Start Holodex with a media library containing ≥1 MKV or MP4 file and `ADMIN_TOKEN` set; in the SPA enter the owner token so admin controls appear.
**0.4** [human] Configure at least one **merge-mode** field in `metadata-mappings.yaml` — e.g. `genres` with `multi: true` and `sources: [tmdb:genres, file:genres]` — and one with `casing: lower`. Reload config (admin) or restart.
**0.5** [human] Apply enrichment (real or fake provider) to the test video so a provider supplies `genres`, giving values to merge against the file's own.

---

## 1. Resolver — merge / dedup / casing / curation (unit)

**1.1** [smoke] `go test ./internal/resolver/...` — all pass, including:
**1.2** [smoke] `TestResolve_MergeUnionAcrossSources` — file+provider union; a shared value carries 2 sources.
**1.3** [smoke] `TestResolve_MergeDedupCaseInsensitive` — `Science Fiction` + `science fiction` → one value.
**1.4** [smoke] `TestResolve_CasingLowerAndTitle` — `casing: lower`/`title` change the output form only.
**1.5** [smoke] `TestResolve_ManualAddJoinsUnion` — a manual value appears with `manual` provenance.
**1.6** [smoke] `TestResolve_SuppressSurvivesReenrich` — a suppressed value stays gone when a provider re-supplies it.
**1.7** [smoke] `TestResolve_NoWriteFlaggedButShown` — a no-write value is shown with `no_write=true`.
**1.8** [smoke] `TestResolve_ScalarManualOverride` — manual value overrides a scalar field; `winning_source=manual:title`.
**1.9** [smoke] Regression: existing F27 precedence tests still pass (`TestResolve_ProviderWinsOverFile`, browse-title tests).

---

## 2. Curation API — auth & validation

**2.1** [agent] `POST /api/v1/media/{id}/curation` without `X-Admin-Token` → `401`.
**2.2** [agent] `POST …/curation` with valid token, missing `field` or invalid `action` → `400`.
**2.3** [agent] `POST …/curation` with `action:add` and an empty/whitespace `value` → `400`.
**2.4** [agent] `POST …/curation` against a non-existent video id → `404`.
**2.5** [agent] `POST …/curation/clear` for a decision that doesn't exist → `204` (idempotent no-op).
**2.6** [agent] A control-character / over-long `value` is sanitized (stored trimmed/clean) — confirm via the detail response, not a 500.

---

## 3. Curation round-trip (agent, against a running server)

**3.1** [agent] Add a manual genre `Sci-Fi`: `POST …/curation {field:genres,value:"Sci-Fi",action:add}` → `204`; `GET /media/{id}` shows `Sci-Fi` in `resolved[genres].items` with `manual:true`.
**3.2** [agent] Suppress a provider value (e.g. `Drama`): `action:suppress` → `204`; it disappears from `resolved[genres].values`.
**3.3** [agent] **Suppression survives re-enrich**: re-apply enrichment; `Drama` does **not** reappear.
**3.4** [agent] No-write a value: `action:nowrite` → the value stays in `items` with `no_write:true`.
**3.5** [agent] Clear the suppression (`/curation/clear {action:suppress}`) → the value reappears.
**3.6** [agent] Dedup: with file and provider both supplying `Action`, `resolved[genres]` lists it once with two `sources`.
**3.7** [agent] Casing: a field configured `casing:lower` returns lower-cased values in `resolved`.

---

## 4. Write queue (agent / unit)

**4.1** [smoke] `go test ./internal/writequeue/...` — all pass.
**4.2** [smoke] `TestQueue_WritesAndAudits` — one batch job → single write, `job_runs(kind=writeback,success)`, audit rows.
**4.3** [smoke] `TestQueue_FailureMarksFailedAndKeepsRow` — a failing write records a failed `job_run` and the original is untouched.
**4.4** [smoke] `TestQueue_RecoverRunningRequeues` — a row left `running` (crash) is re-run on boot.
**4.5** [agent] `POST /media/{id}/writeback {fields:[…]}` with the queue wired → **202** with `{job_id, queued}` (not 204).
**4.6** [agent] After the worker drains, `GET /admin/activity/history` shows a `kind=writeback` entry; `file_writebacks` has one row per written field.
**4.7** [agent] Set `WRITEBACK_CONCURRENCY=1` (default), enqueue several writes → they run one at a time (serialized); `queued` count reflects depth.
**4.8** [human] Confirm the written tags landed: `exiftool -GENRE <file>` (MP4) or `mkvextract`/`exiftool` for MKV shows the curated set — **suppressed/no-write values absent, each value once**.
**4.9** [human] No `.holodex-tmp` / `.holodex-new` file remains beside the media after a write; a forced kill mid-write leaves the original intact and the job re-runs on restart.

---

## 5. UI — curation chips (human)

> Detail page → **Metadata** section, owner logged in. Each set field's values render as chips.

**5.1** [human] A merge field (genres) shows one chip per value; a value from a provider shows an accented `·tmdb`, a file/manual value a muted `·file` / `·manual`; a shared value shows both (`·tmdb + file`).
**5.2** [human] **Add**: click **+ Add**, type a value, Enter → a new `·manual` chip appears; Esc/blank cancels; a duplicate (case-insensitive) doesn't create a second chip.
**5.3** [human] **Edit**: pencil → inline input; Enter commits (chip updates), Esc cancels.
**5.4** [human] **Remove**: ✕ suppresses the value; the chip disappears and stays gone after reload.
**5.5** [human] **Don't-write**: the write-toggle marks the chip struck-through/dimmed; it stays visible but is excluded from the next write.
**5.6** [human] A scalar field (e.g. title) shows a single chip with **no remove / no Add**; editing it sets an override.
**5.7** [human] **Non-owner** (no token): chips render read-only — no edit/remove/add/toggle controls.
**5.8** [human] `image_url` (poster) and `long_text` (overview) fields are unchanged (no chip controls).

---

## 6. UI — theming (human, all 3 skins)

**6.1** [human] **Cinémathèque**: chips use `bg-surface-2`/`text-ink`, provider provenance reads in `text-accent`, controls go muted→accent on hover; the add-input ring is `accent`.
**6.2** [human] **Broadcast**: repeat 6.1 — no hardcoded colours; chips legible on the surface.
**6.3** [human] **Brutalist**: repeat 6.1 — accent provenance and chip text meet contrast (the usual offender).
**6.4** [human] `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src/lib/components/Curation*.svelte` is empty (tokens only; `rounded-full` pills are intentional).
**6.5** [human] Error states (a failed curation call) render in `text-warn`; remove/edit do **not** use warn.

---

## 7. Accessibility (human)

**7.1** [human] Tab reaches each chip's edit/remove/toggle controls and the **+ Add** affordance.
**7.2** [human] Edit/Add inputs: Enter commits, Esc cancels; focus returns sensibly after commit/cancel.
**7.3** [human] Each chip exposes a meaningful `aria-label` ("{value}, from {sources}[, not written to file]"); the write-toggle is a button with `aria-pressed`.
**7.4** [human] Error/queue status regions announce via `aria-live="polite"` without stealing focus.

---

## 8. Deferred (tracked, not gaps)

- **"Show removed (N)" restore UI** — suppressions are reversible via `/curation/clear` but there is no in-page list of removed values yet (re-add the same value to restore for now).
- **`WritebackFormDialog` "Queued" copy** — the dialog resolves on the 202 but still reads "Written"; queue-status surfacing (position N → writing → written/failed) is a follow-up.
- **Person-entity curation** (F30 fast-follow) — the model is entity-typed; only `entity_type='person'` wiring remains.
