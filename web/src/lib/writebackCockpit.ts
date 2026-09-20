// HOLODEX-400 — pure view-model helpers for the writeback dialog's cockpit rows
// (docs/design/writeback-cockpit-handoff.md). The dialog stages a per-row source pick
// (the same SourceChip model SourceBadge/SourceEditModal use) and only commits on Write;
// these helpers decide, from that staged pick alone, what class a row is, what value it
// would write, and whether the write must be preceded by a decision. No I/O, no Svelte —
// unit-tested in writebackCockpit.test.ts.
import {
	fileCandidateValue,
	isReplaceField,
	isWritable,
	resolveSelection,
	type SourceChip
} from './f36';
import type { ResolvedField } from './types';

// StagedPick is a row's local, uncommitted selection: the chip key plus the Custom literal
// (only meaningful when key === 'custom').
export interface StagedPick {
	key: string | null;
	custom: string;
}

// isCockpitRow is true for the rows that get a chooser: every replace field — text (chip
// row / stacked rows) and image_url (image tiles, HOLODEX-403 / ADR-101 D3). Merge rows
// never carry a decision (RD1) and stay read-only (HOLODEX-401).
export function isCockpitRow(field: ResolvedField): boolean {
	return isReplaceField(field);
}

// isImageRow: the cockpit row whose chooser is image tiles.
export function isImageRow(field: ResolvedField): boolean {
	return isCockpitRow(field) && field.display === 'image_url';
}

// CHIP_VALUE_MAX_CHARS: the longest candidate a CurationChip shows whole. The chip's value span
// is `max-w-[14rem] truncate` at text-xs; 32 characters is what fits in the widest skin font
// (Broadcast's mono), so a value past it is guaranteed clipped somewhere.
export const CHIP_VALUE_MAX_CHARS = 32;

// stacksCandidates: the writeback dialog renders this cockpit row as stacked full-width radio
// rows (SourceRadioList) instead of a chip row. Always for `long_text`; otherwise whenever any
// candidate is long enough for a chip to truncate it (HOLODEX-434) — a clipped value cannot be
// compared against its neighbours, and comparing them is the whole point of the row. The Custom
// chip counts too: its value is the STANDING manual literal (sourceChips), which the chip row
// renders as a value chip with the same clip; a literal typed in the dialog lives in the staged
// pick, not in chips, so the layout never flips under the owner mid-edit. Image rows never
// stack (tiles).
export function stacksCandidates(field: ResolvedField, chips: SourceChip[]): boolean {
	if (!isCockpitRow(field) || isImageRow(field)) return false;
	if (field.display === 'long_text') return true;
	return chips.some((c) => c.value.trim().length > CHIP_VALUE_MAX_CHARS);
}

// stagedValue is the value the staged pick would write — the chip's value, or the trimmed
// Custom literal. An unknown/null key writes nothing.
export function stagedValue(chips: SourceChip[], staged: StagedPick): string {
	if (staged.key === 'custom') return staged.custom.trim();
	return (chips.find((c) => c.key === staged.key)?.value ?? '').trim();
}

// RowClass is the handoff's §1 table. `unverifiable` (in_sync unknown) is orthogonal — see
// isUnverifiable — because such a row is still one of these three.
export type RowClass = 'write' | 'matches' | 'unwritable';

// rowClass classifies a row from its LIVE value: unwritable when the container has no tag for
// the field (HOLODEX-216); `matches` when the value equals the file's own tag value (the `=`
// gutter tier); otherwise `write`. A field with no candidates at all (person-style payload)
// can never read as matching — there is no file value to match.
export function rowClass(field: ResolvedField, value: string): RowClass {
	if (!isWritable(field)) return 'unwritable';
	if (field.candidates !== undefined && value.trim() === fileCandidateValue(field).trim()) {
		return 'matches';
	}
	// The backend's own sync verdict stands in for the file value when the staged pick is
	// still the resolved winner: for an image row the file candidate never carries the
	// embedded cover art's URL, so only the write ledger can say the file has it (ADR-101) —
	// and that verdict must hold even when the allowlist degraded the row's display to text,
	// so it is keyed on `in_sync`, not on isImageRow. Only a STANDING decision is witnessed:
	// the resolver reports `in_sync: true` for every undecided field by construction (nothing
	// decided, nothing to lag), which says nothing about the file — an undecided provider
	// poster that reads as "matches" here would save a decision and never write
	// (HOLODEX-433). For a decided text row the clause is redundant (in_sync true already
	// means decided == file candidate). Re-pointing differs again.
	if (
		field.decision?.standing === true &&
		field.in_sync === true &&
		value.trim() === (field.values[0] ?? '').trim()
	) {
		return 'matches';
	}
	return 'write';
}

// isUnverifiable: the field is writable but nothing can witness its sync state — no file
// read-back source for a text field (ADR-093), no ledger row yet for an image field
// (ADR-101) — so `in_sync` is absent. The row still gets its chooser; the baseline chip/tile
// says "not read back" instead of claiming the file is empty.
export function isUnverifiable(field: ResolvedField): boolean {
	return isCockpitRow(field) && isWritable(field) && field.in_sync === undefined;
}

// needsDecision: must submit() call decide() for this row before enqueuing the write? True when
// no standing decision exists (the checkbox is the commit — HOLODEX-219/273), or when the staged
// pick differs from the committed selection (a different chip, or a different Custom literal).
// An untouched, already-decided row makes no call.
export function needsDecision(field: ResolvedField, chips: SourceChip[], staged: StagedPick): boolean {
	if (!isCockpitRow(field) || staged.key === null) return false;
	if (!field.decision?.standing) return true;
	const current = resolveSelection(field, chips).key;
	if (staged.key !== current) return true;
	if (staged.key === 'custom') {
		return staged.custom.trim() !== (field.decision.manual_value ?? '').trim();
	}
	return false;
}

// The golden record (owner's term, 2026-09-19) is Holodex's own source of truth for the
// entity — baseline + enrichment shadow + decisions, resolved. Only the MAPPED subset of it
// reaches the file: a field with a `write_target` for this container. The dialog therefore
// has two destinations per row — the file (willWrite) and the system alone
// (savesDecisionOnly) — and its gutter names which one Write will touch.

// willWrite is the single gate behind the dialog's footer count and its write set. A row is
// written when it is writable, differs from the file, carries a value, AND is decided — either
// standing before the dialog opened, or `touched` (the owner picked a chip in this dialog;
// picking IS the confirm). An undecided row the owner never touched is never written and never
// decided, so opening the dialog and pressing Write can only ever sync decisions the owner
// actually made. A non-cockpit row (image_url, merge) has no decision to make — no chooser yet
// (HOLODEX-403 / HOLODEX-401) and, for merge fields, no decision model at all (RD1) — so it is
// never written from the dialog. There is no checkbox anywhere: deciding is the check action.
export function willWrite(
	field: ResolvedField,
	value: string,
	opts: { staged: StagedPick; touched: boolean }
): boolean {
	if (!isCockpitRow(field)) return false;
	if (rowClass(field, value) !== 'write') return false;
	if (isBlankCustom(opts.staged)) return false;
	return !!field.decision?.standing || opts.touched;
}

// isBlankCustom: a Custom pick with nothing typed — "nothing chosen yet", never a value.
export function isBlankCustom(staged: StagedPick): boolean {
	return staged.key === 'custom' && staged.custom.trim() === '';
}

// savesDecisionOnly: Write will record this row's pick in Holodex but write nothing to the
// file — the row is touched and its pick differs from the standing decision (needsDecision),
// yet it cannot or need not reach the file: no file tag for this container (unmapped), or the
// pick IS the file's own value (a re-point to the baseline). Deciding an unmapped field here is
// the point of the cockpit — the decision is part of the golden record whether or not the file
// can carry it. An untouched row never saves anything.
export function savesDecisionOnly(
	field: ResolvedField,
	chips: SourceChip[],
	value: string,
	opts: { staged: StagedPick; touched: boolean }
): boolean {
	if (!isCockpitRow(field) || !opts.touched || isBlankCustom(opts.staged)) return false;
	if (willWrite(field, value, opts)) return false;
	return needsDecision(field, chips, opts.staged);
}
