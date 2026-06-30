# Manual QA Checklist: Per-field source-of-truth decisions (F36)

**Spec**: [Per-field source-of-truth (F36)](../specs/field-source-of-truth.md) · **ADR**: [ADR-051](../architecture/ADR-051-per-field-source-of-truth-decisions.md) · **Design**: [handoff](field-source-of-truth-handoff.md)
**Gate**: [ADR-030](../architecture/ADR-030-access-control-gating-seam.md) · **Write safety**: [ADR-041](../architecture/ADR-041-metadata-writeback.md)/[ADR-048](../architecture/ADR-048-metadata-curation-and-write-queue.md)
**Theming contract**: [ADR-021](../architecture/ADR-021-frontend-theming-and-skins.md) + [theming.md](theming.md) — **tokens only, QA all three skins.**

> Run this **before merge**. Items are grouped **by verifier** — each actor runs only their section:
> - **§2 Smoke** — automated test / build gate (`svelte-check`, token-guard `rg`, unit/integration). Green = pass; pre-check `[x]` with the test named.
> - **§3 Agent** — an AI agent drives the running app (DOM/ARIA/Network/computed-style). Deterministic, no human judgment.
> - **§4 Human** — needs a human's eye (per-skin "look", legibility, the at-a-glance read).
>
> §1 is one-time **setup** §3/§4 depend on. Every item is numbered `section.item` — cite the number when filing a miss.
>
> **Core invariants for every item:**
> 1. **Decisions are DB-only (RD5).** Changing a source must issue a decision API call and **never** a file write — no `WriteBatch`, no `.holodex-tmp`/`.holodex-new`, no file-write spinner. The file changes **only** on "Write decisions to file".
> 2. **One batched atomic write per file (RD5/P0-4).** A write of N decided fields = **one** queued job = **one** `WriteBatch` invocation. Never one write per field/toggle.
> 3. **File-first default (RD4).** An undecided replace field shows the **file** value; a provider is a candidate, not the winner.
> 4. **The server gate is the only authority.** Hiding the owner-only control is presentation; `requireOwner` on the decision + writeback endpoints is the boundary.

---

## 1. Setup / preconditions

> **Who does this:** whoever sets up the session (developer or agent) — *not* the §4 human. **Quick "is it ready?" check:** open a media item as owner; under **Metadata**, a **replace** field (e.g. Title) shows a small **Keep file · Adopt {provider} · Custom** selector under its value, and the header button reads **Write decisions to file**.

- [ ] 1.1 App running with a library item that has, on at least one **replace** field, **both** a file value and a **provider** value (so a real choice exists). A second matched provider with a *different* value for some replace field enables the multi-provider checks (§3.9, §4.4). Dev: `--media-path E:/AMVTestCopy --host 127.0.0.1`.
- [ ] 1.2 At least one **merge** field present (genres/actors) to confirm it is **unchanged**.
- [ ] 1.3 Owner setup: `ADMIN_TOKEN` unset (open/owner) or set then unlocked; know how to simulate a **non-owner** (token set, none entered) for §3.12.
- [ ] 1.4 Devtools open — Console (errors), Network (watch the decision `PUT`/`DELETE` and the `writeback` POST), and a file-system watch on the media file's directory to confirm **no** `.holodex-tmp`/`.holodex-new` appears on a decision toggle (invariant 1).
- [ ] 1.5 Know how to externally edit a tag (e.g. `exiftool -P` / `mkvpropedit`) + run **Refresh** (F31), to drive the out-of-sync and source-pin checks.

---

## 2. Smoke — automated (green in CI)

- [ ] 2.1 **`svelte-check`** passes with the new `SourceSelect` component, the `+page.svelte` replace-field changes, and the renamed write button.
- [ ] 2.2 **Token-discipline guard** empty on the changed markup: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` — `SourceSelect` uses `rounded-theme`/`border-rule`/`bg-surface-2`/`bg-accent`/`text-accent`/`text-warn` only; any SVG uses `currentColor`.
- [ ] 2.3 **Resolver — decision short-circuit (replace).** Unit: a decision pins display to the decided source over mapping order; absence of a decision resolves **file-first** under `default_source: file`; `default_source: mapping` restores first-non-empty. *(pure resolver test, no I/O.)*
- [ ] 2.4 **Resolver — merge untouched.** Unit: merge-field resolution (union + per-value curation) is **byte-identical** with and without F36 (RD1 regression guard).
- [ ] 2.5 **Source-pin follows live layer (P0-2).** Unit/integration: a `file` decision reflects a re-extracted changed file value; an `adopt provider` decision reflects a re-enriched value; a `manual` decision is frozen.
- [ ] 2.6 **Writeback uses the decided value + ONE batch (P0-4/RD5).** Integration: with N decided replace fields, a write produces **exactly one** `WriteBatch` invocation for the file, carrying the decided values (assert call count = 1 — the per-field-regression guard). Merge fields contribute their curated set to the **same** batch.
- [ ] 2.7 **Decision is DB-only (RD5).** Integration: `PUT/DELETE …/decision` performs **no** file write (no `WriteBatch` call, no temp file).
- [ ] 2.8 **API auth/status.** `PUT/DELETE /media/{id}/fields/{canonical}/decision` → 401/403 without owner; 400 bad source/canonical; 404 unknown id/field; 409 soft-deleted; 200/204 happy path. Untrusted `manual_value` is sanitized (same path as F30 add).
- [ ] 2.9 **Sync recompute.** Unit: a field is "out of sync" iff decided value ≠ value embedded in the file; after a successful write it reads in-sync **without** mutating the decision.

> `/security-review` sign-off required before merge (owner gate + untrusted `manual_value` that feeds a file write).

---

## 3. Agent — drive the running app

**Selector presence & default (RD1/RD4)**

- [ ] 3.1 On a **replace** field, a `role="radiogroup"` `SourceSelect` renders for the owner with segments `Keep file`, one `Adopt {provider}` per matched provider, and `Custom`.
- [ ] 3.2 **Undecided** replace field: the resolved chip shows the **file** value with `·file` provenance; `Keep file` is `aria-checked="true"`; the provider appears only as a muted **candidate** (row 3), not as the chip value.
- [ ] 3.3 A **merge** field shows the unchanged `CurationFieldRow` chips (no `SourceSelect`) — RD1.
- [ ] 3.4 The candidates line (row 3) renders **only** when ≥1 provider candidate exists; a file-only replace field shows no candidates line and no provider segment.

**Decisions are DB-only (invariant 1 / RD5)**

- [ ] 3.5 Selecting `Adopt {provider}` issues a single `PUT …/decision` (Network); **no** writeback POST, **no** `.holodex-tmp`/`.holodex-new` on disk, **no** file-write spinner. After refetch the chip shows the provider value `·{provider}` (accent).
- [ ] 3.6 Selecting `Custom` opens the inline input; committing issues `PUT …/decision {source:"manual"}`; chip shows the literal `·manual` (muted). Escape/empty cancels with no call.
- [ ] 3.7 Selecting `Keep file` from a decided state issues `DELETE …/decision` (or a `file` PUT) and returns the field to the file value/default.
- [ ] 3.8 Source-pin: externally edit the file tag, run **Refresh**; a field decided `Keep file` updates to the new file value with no re-decision (P0-2).

**Multi-provider (P1-1/P1-2)**

- [ ] 3.9 With two matched providers supplying **different** values for a replace field, both `Adopt` segments render and a **muted** "providers differ" hint shows on the candidates line — **not** a `text-warn` pill (Open-Q3).
- [ ] 3.10 A provider with **no** value for the field has **no** `Adopt` segment (edge case).

**Sync indicators (RD2)**

- [ ] 3.11 When a decided value differs from the file's embedded tag, a `text-warn` **out-of-sync** pill shows on row 1, and the header shows **Write decisions to file · {n} out of sync** with the matching count. After a successful write, both clear and the decision is unchanged.

**Writeback batching (invariant 2 / P0-4)**

- [ ] 3.12 Decide several fields, then **Write decisions to file**: Network shows **one** writeback request; server logs/activity show **one** `kind=writeback` job → **one** `WriteBatch` for the file (not one per field). The write spinner appears **only** here.

**Owner gating (invariant 4)**

- [ ] 3.13 As a **non-owner**, `SourceSelect`, the candidates line, the out-of-sync pill, and the write button are absent; the field renders the read-only resolved value exactly as today. Direct `PUT …/decision` still returns 401/403.

**Keyboard / a11y (P0-8)**

- [ ] 3.14 `SourceSelect` is **one** Tab stop (lands on the checked segment); **Left/Right/Up/Down** move and change selection (native radio semantics); `aria-checked` tracks selection; each segment's `aria-label` names its value.
- [ ] 3.15 Focus-visible ring on segments and the Custom input; Escape in the Custom input returns focus to the `Custom` segment. No keyboard trap.

---

## 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist)

> **Nav:** open a media item with a provider-matched replace field, in **owner** mode. Switch skins via the header picker. For each skin below, eyeball the Metadata section.

- [ ] 4.1 **The selector reads as a control, in every skin.** The `SourceSelect` looks like a segmented toggle (not stray text); the **selected** segment is unmistakably distinct from idle ones, and the distinction survives Brutalist (where a filled `bg-accent` can read heavy — confirm the chosen treatment still reads "selected" without shouting). No clipped corners, no collision with the chip.
- [ ] 4.2 **Provenance + candidates are legible, not noisy.** The `·file`/`·{provider}`/`·manual` suffix and the muted candidates line are readable but clearly secondary to the value; the field doesn't feel cluttered at two-column width.
- [ ] 4.3 **The two signals don't read as one alarm.** When a field is both out-of-sync (warn pill, row 1) and has "providers differ" (muted, row 3), they're visually separate and only the out-of-sync one carries warn weight. Confirm in all three skins (warn-on-surface contrast differs per skin).
- [ ] 4.4 **Multi-provider choice is obvious.** With IMDB + TMDB both matched, the two `Adopt` options are distinguishable and the disagreement is noticeable at a glance without being alarming.
- [ ] 4.5 **Write button.** "Write decisions to file · {n} out of sync" is legible; the count reads as attention (warn) but the button doesn't look broken when the count is hidden (n=0).
- [ ] 4.6 **The fix, felt.** Edit a file tag externally → Refresh → the field shows **your file value** by default (no provider masking). Then Adopt the provider, confirm the value switches; Keep file, confirm it switches back. The mental model ("file is the baseline, I choose") is clear without instruction.
- [ ] 4.7 **Empty/edge states themed.** A file-only field (no provider) shows just `Keep file | Custom`; a long title truncates gracefully; nothing overflows the card in any skin.
