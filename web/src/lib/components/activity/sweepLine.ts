// Pure pieces of SweepStatusLine (F66 RD10) — kept out of the component so the
// running→idle edge and the done-line derivation are unit-testable without a DOM.
import type { SweepKind, SweepSkippedProvider, SweepStatus, SweepSummary } from '$lib/types';

// The done line is last_run for this kind, unless the owner dismissed that batch.
export function doneLineFor(
	sweep: SweepStatus | undefined,
	kind: SweepKind,
	dismissedBatch: string
): SweepSummary | null {
	if (!sweep || sweep.state !== 'idle' || !sweep.last_run) return null;
	if (sweep.last_run.kind !== kind || sweep.last_run.batch_id === dismissedBatch) return null;
	return sweep.last_run;
}

// Edge detector: returns true exactly once when a sweep of this kind the page saw
// running goes idle. A page that mounts after the sweep finished never fires — its
// initial load was already fresh.
export interface SweepEdge {
	sawRunning: boolean;
}

export function sweepFinished(
	edge: SweepEdge,
	sweep: SweepStatus | undefined,
	kind: SweepKind
): boolean {
	if (!sweep) return false;
	if (sweep.state === 'running' && sweep.kind === kind) {
		edge.sawRunning = true;
		return false;
	}
	if (sweep.state === 'idle' && edge.sawRunning) {
		edge.sawRunning = false;
		return true;
	}
	return false;
}

// "tmdb stopped responding · a, b rate-limited", at most three names per reason
// then "and N more" (handoff: very long done line).
export function skippedText(list: SweepSkippedProvider[]): string {
	const byReason = new Map<string, string[]>();
	for (const s of list) byReason.set(s.reason, [...(byReason.get(s.reason) ?? []), s.provider]);
	return [...byReason]
		.map(([reason, names]) => {
			const shown = names.slice(0, 3).join(', ');
			const more = names.length > 3 ? ` and ${names.length - 3} more` : '';
			return `${shown}${more} ${reason}`;
		})
		.join(' · ');
}
