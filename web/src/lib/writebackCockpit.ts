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

// isCockpitRow is true for the rows that get the chooser: replace fields with a text value.
// image_url rows keep HOLODEX-245's read-only comparison (the poster chooser is HOLODEX-403);
// merge rows never carry a decision (RD1) and stay on their seeded text (HOLODEX-401).
export function isCockpitRow(field: ResolvedField): boolean {
	return isReplaceField(field) && field.display !== 'image_url';
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
	return 'write';
}

// isUnverifiable: the field is writable but its mapping declares no file read-back source
// (ADR-093 — `in_sync` absent), so the dialog can never later report `=` for it. The row still
// gets a checkbox and a chooser; it just carries the "can't verify" note.
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
	if (opts.staged.key === 'custom' && opts.staged.custom.trim() === '') return false;
	return !!field.decision?.standing || opts.touched;
}
