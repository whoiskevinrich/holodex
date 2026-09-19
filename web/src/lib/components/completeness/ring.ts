// The completeness ring's one reading (F65.4, design handoff § Ring component):
// what the accent arc shows, whether an ink "second lap" is drawn over it, and
// the aria-label — pure, so the geometry is unit-tested off the component.

import type { CompletenessSummary } from '$lib/types';

/** Circumference used for stroke-dasharray: 2π·7 ≈ 43.98, and 44 closes a full
 * ring with no visible gap at 14 px (handoff § Ring component). */
export const RING_CIRCUMFERENCE = 44;

export interface RingReading {
	/** The accent arc, 0–100: `required`, or `extras` when the type has no required band. */
	ring: number;
	/** The ink arc drawn over a full ring, 0–100 — only once `required` is 100 (RD6, O2). */
	overfill: number;
	/** `role="img"` label; the only text the ring ever has. */
	label: string;
	/** Both bands null — an entity type the score doesn't cover; mount nothing. */
	empty: boolean;
}

export function ringReading({ required, extras }: CompletenessSummary): RingReading {
	const clauses: string[] = [];
	if (required !== null) clauses.push(`required ${required}%`);
	if (extras !== null) clauses.push(`extras ${extras}%`);
	return {
		ring: required ?? extras ?? 0,
		overfill: required === 100 ? (extras ?? 0) : 0,
		label: `Completeness: ${clauses.join(', ')}`,
		empty: required === null && extras === null
	};
}

/** stroke-dasharray arc length for a 0–100 value. 1–4 % stays a visible tick —
 * never clamped to zero (handoff § Edge cases). */
export function arc(pct: number): number {
	return (pct / 100) * RING_CIRCUMFERENCE;
}
