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

> **Who does this:** whoever sets up the session (developer or agent) — *not* the §4 human. **Quick "is it ready?" check:** open a media item as owner; under **Metadata**, a **replace** field (e.g. Title) shows a **row of value chips** — the file value first with a leading **● dot** and `·file`, one chip per provider value, and a **＋ Custom** chip — and the header button reads **Write decisions to file**. (HOLODEX-112 refinement: the old `Keep file · Adopt · Custom` segmented control + candidates line are replaced by this chip radiogroup.)

- [ ] 1.1 App running with a library item that has, on at least one **replace** field, **both** a file value and a **provider** value (so a real choice exists). A second matched provider with a *different* value for some replace field enables the multi-provider checks (§3.9, §4.4). Dev: `--media-path E:/AMVTestCopy --host 127.0.0.1`.
- [ ] 1.2 At least one **merge** field present (genres/actors) to confirm it is **unchanged**.
- [ ] 1.3 Owner setup: `ADMIN_TOKEN` unset (open/owner) or set then unlocked; know how to simulate a **non-owner** (token set, none entered) for §3.12.
- [ ] 1.4 Devtools open — Console (errors), Network (watch the decision `PUT`/`DELETE` and the `writeback` POST), and a file-system watch on the media file's directory to confirm **no** `.holodex-tmp`/`.holodex-new` appears on a decision toggle (invariant 1).
- [ ] 1.5 Know how to externally edit a tag (e.g. `exiftool -P` / `mkvpropedit`) + run **Refresh** (F31), to drive the out-of-sync and source-pin checks.

---

## 2. Smoke — automated (green in CI)

- [ ] 2.1 **`svelte-check`** passes with the `SourceSelect` chip radiogroup, the `CurationChip` `radio` mode, the `f36.ts` `sourceChips`/`selectedChipKey` helpers (+ `f36.test.ts`), and the `+page.svelte` replace-field wiring.
- [ ] 2.2 **Token-discipline guard** empty on the changed markup: `rg 'zinc-|sky-|emerald-|amber-|rounded-(lg|md|sm|xl)' web/src --glob '*.svelte'` — the chips use `rounded-full`/`border-rule`/`bg-surface-2`/`border-accent`/`text-accent`/`text-warn` only (selection is `border-accent` + accent dot, **not** a filled `bg-accent`); any SVG uses `currentColor`.
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

**Chip row presence & default (RD1/RD4)**

- [ ] 3.1 On a **replace** field, a `role="radiogroup"` renders for the owner: a `role="radio"` **file** chip first (leading ● dot, `·file`), one chip per **distinct** provider value, and a trailing **Custom** chip.
- [ ] 3.2 **Undecided** replace field: the **file** chip is `aria-checked="true"` and shows the file value `·file`; the provider appears as its **own sibling chip** (`·{provider}`), never as the file chip's value.
- [ ] 3.3 A **merge** field shows the unchanged `CurationFieldRow` chips — same shell, but **✕-per-chip** (no ● dot, no radiogroup) — RD1.
- [ ] 3.4 A file-only replace field (no matched provider value) shows just the **file** chip + **Custom**; a provider value equal to the file value **folds** into the file chip as `·file + {provider}` (the value appears once, not twice).

**Decisions are DB-only (invariant 1 / RD5)**

- [ ] 3.5 Selecting a **provider** chip issues a single `PUT …/decision` (Network); **no** writeback POST, **no** `.holodex-tmp`/`.holodex-new` on disk, **no** file-write spinner. After refetch that chip is `aria-checked` and shows `·{provider}` (accent).
- [ ] 3.6 Selecting **Custom** opens the inline input; committing issues `PUT …/decision {source:"manual"}`; the Custom chip becomes the `·manual` value chip (selected). Escape/empty cancels with no call and **returns focus to the Custom chip**; a changed-then-Escaped value is **not** committed.
- [ ] 3.7 Selecting the **file** chip from a decided state issues `DELETE …/decision` (or a `file` PUT) and returns the field to the file value/default.
- [ ] 3.8 Source-pin: externally edit the file tag, run **Refresh**; a field decided to the **file** chip updates to the new file value with no re-decision (P0-2).

**Multi-provider (P1-1/P1-2)**

- [ ] 3.9 With two matched providers supplying **different** values for a replace field, **both** provider chips render (distinct values, self-evident divergence — no "providers differ" hint, no `text-warn`); two providers with the **same** value fold into **one** chip tagged `·{p1} + {p2}`.
- [ ] 3.10 A provider with **no** value for the field contributes **no** chip (edge case).

**Sync indicators (RD2)**

- [ ] 3.11 When a decided value differs from the file's embedded tag, a `text-warn` **out-of-sync** pill shows on row 1, and the header shows **Write decisions to file · {n} out of sync** with the matching count. After a successful write, both clear and the decision is unchanged.

**Writeback batching (invariant 2 / P0-4)**

- [ ] 3.12 Decide several fields, then **Write decisions to file**: Network shows **one** writeback request; server logs/activity show **one** `kind=writeback` job → **one** `WriteBatch` for the file (not one per field). The write spinner appears **only** here.

**Owner gating (invariant 4)**

- [ ] 3.13 As a **non-owner**, the chip radiogroup, the out-of-sync pill, and the write button are absent; the field renders the read-only resolved value exactly as today. Direct `PUT …/decision` still returns 401/403.

**Keyboard / a11y (P0-8)**

- [ ] 3.14 The chip radiogroup is **one** Tab stop (lands on the checked chip; the group itself is `tabindex="-1"`); **Left/Right/Up/Down** move and change selection (native radio semantics, debounced commit); `aria-checked` tracks selection; each chip's `aria-label` names its value + source.
- [ ] 3.15 Focus-visible ring on chips and the Custom input; Escape in the Custom input returns focus to the **Custom** chip. No keyboard trap.

---

## 4. Human — needs an eye (run in **all three skins**: Cinémathèque · Broadcast · Brutalist)

> **Nav:** open a media item with a provider-matched replace field, in **owner** mode. Switch skins via the header picker. For each skin below, eyeball the Metadata section.

- [ ] 4.1 **Selection reads at a glance, in every skin.** The **selected** chip is unmistakable via its **● filled dot + accent border** (not fill/colour alone), and the distinction survives Brutalist (the chip stays on `bg-surface-2` — confirm it reads "selected" without shouting). No clipped corners, no dot/value collision.
- [ ] 4.2 **The row reads as one vocabulary with the merge chips.** The replace chips (● dot) and the merge chips (✕ on hover) sit in the same Metadata grid and clearly belong to one system; the `·source` suffix is readable but secondary; the field isn't cluttered at two-column width.
- [ ] 4.3 **The dedup fix, felt.** On a field where file and provider **agree** (e.g. Studio), the value shows **once** as a single `·file + {provider}` chip — not twice. Confirm in all three skins.
- [ ] 4.4 **Divergence is obvious without alarm.** Where file and provider (or two providers) **differ**, the two value chips make the disagreement self-evident at a glance; nothing reads as a warning (only a real out-of-sync decision shows the warn pill). Confirm warn-on-surface contrast in all three skins.
- [ ] 4.5 **Write button.** "Write decisions to file · {n} out of sync" is legible; the count reads as attention (warn) but the button doesn't look broken when the count is hidden (n=0).
- [ ] 4.6 **The fix, felt.** Edit a file tag externally → Refresh → the field shows **your file value** by default (no provider masking). Then pick the provider chip, confirm the value switches; pick the file chip, confirm it switches back. The mental model ("file is the baseline, I choose") is clear without instruction.
- [ ] 4.7 **Empty/edge states themed.** A file-only field shows just the **file** chip + **Custom**; an empty file baseline shows `—`; a long title truncates gracefully (full value on hover); nothing overflows the card in any skin.
