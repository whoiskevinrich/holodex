// The owner's Hover montage row in the media page's Maintenance section (F83, handoff
// "Media page: the Maintenance rows"): what each status reads as, and which actions it offers. Waiting
// and making offer none rather than a disabled button.
import { formatBytes } from '#lib/format.ts';
import type { MontageStatus } from '#lib/types.ts';

export type MontageAction = 'make' | 'preview' | 'remake' | 'retry';

export const MONTAGE_POLL_MS = 3000;

export function montageStatusView(st: MontageStatus): { value: string; detail?: string; actions: MontageAction[] } {
	switch (st.state) {
		case 'waiting':
			return { value: 'Waiting in queue…', actions: [] };
		case 'making':
			return { value: 'Making…', actions: [] };
		case 'ready':
			return { value: 'Ready', detail: st.size_bytes ? formatBytes(st.size_bytes) : undefined, actions: ['preview', 'remake'] };
		case 'failed':
			return { value: 'Failed', detail: st.error || undefined, actions: ['retry'] };
		default:
			return { value: 'None yet', actions: ['make'] };
	}
}

// The row polls only while there is something to wait for.
export function shouldPollMontage(st: MontageStatus | null): boolean {
	return st?.state === 'waiting' || st?.state === 'making';
}

export const montageActionLabel: Record<MontageAction, string> = {
	make: 'Make montage',
	preview: 'Preview',
	remake: 'Remake',
	retry: 'Retry'
};
