// Shared "Refresh" (RD7/P0-5) and "Refresh all" (RD8/P1-2) actions for the video/
// person/studio/film detail pages (F47). Each page keeps its own busy/error/picker state
// and reload routine — these just run the fetch→reload→error sequence identically for
// all four, taking that state as setter callbacks instead of owning it.
//
// Films (F59/ADR-089 D5) needed no change here: these are generic over EnrichEntityKind
// and the path comes from ENRICH_ENTITY_BASE, so widening the union was enough. A
// film-specific branch in this file would be a regression, not a fix — api.test.ts pins
// that films drive the generic path.
import { api } from './api';
import { toMessage } from './format';
import type { EnrichEntityKind, RefreshAllResult } from './types';

// enrichedLine is the inline success line after an enrich lands (HOLODEX-86): which
// provider, and how many fields it returned. An empty response says so rather than
// claiming a success the field list won't show.
export function enrichedLine(provider: string, fields: number): string {
	if (fields === 0) return `No values came back from ${provider}`;
	return `Enriched from ${provider} · ${fields} ${fields === 1 ? 'field' : 'fields'}`;
}

// refreshAllLine summarizes a Refresh-all fan-out as one clause, not one per provider —
// a ten-provider entity would otherwise repeat "Enriched from" ten times. It covers only
// the providers that ran; the other statuses report elsewhere: rate_limited on the error
// slot, needs_review by opening the picker, no_candidates nowhere (nothing happened).
export function refreshAllLine(results: RefreshAllResult[]): string {
	const ran = results.filter((r) => r.status === 'refreshed' || r.status === 'auto_applied');
	const landed = ran.filter((r) => r.enriched?.length);
	const empty = ran.filter((r) => !r.enriched?.length);
	const total = landed.reduce((n, r) => n + (r.enriched?.length ?? 0), 0);
	const names = (rs: RefreshAllResult[]) => rs.map((r) => r.provider).join(', ');
	return [
		landed.length ? enrichedLine(names(landed), total) : '',
		empty.length ? enrichedLine(names(empty), 0) : ''
	]
		.filter(Boolean)
		.join('; ');
}

export async function runEnrichRefresh(
	kind: EnrichEntityKind,
	id: number,
	provider: string,
	setBusy: (v: string) => void,
	setError: (v: string) => void,
	reloadDetail: () => Promise<void>,
	setStatus: (v: string) => void
): Promise<void> {
	setBusy(provider);
	setError('');
	setStatus('');
	try {
		const { enriched } = await api.enrichRefresh(kind, id, provider);
		await reloadDetail();
		setStatus(enrichedLine(provider, enriched?.length ?? 0));
	} catch (e) {
		setError(toMessage(e));
	} finally {
		setBusy('');
	}
}

// rateLimitedLine is the inline status text for a paused provider (F66 RD8) — the
// same sentence the single-call 503 body carries.
export function rateLimitedLine(provider: string, secs: number): string {
	return `${provider} is rate-limiting — try again in ${secs} s`;
}

// A provider that resolved ambiguously must not be silently dropped — openPicker is
// called with the first `needs_review` result so the owner sees it immediately. A
// rate-limited provider (ADR-103 D4) is reported per row, so the other providers'
// results still land; its line goes on the same inline error slot a failed call uses.
export async function runEnrichRefreshAll(
	kind: EnrichEntityKind,
	id: number,
	setRefreshingAll: (v: boolean) => void,
	setError: (v: string) => void,
	reloadDetail: () => Promise<void>,
	openPicker: (provider: string) => void,
	setStatus: (v: string) => void
): Promise<void> {
	setRefreshingAll(true);
	setError('');
	setStatus('');
	try {
		const { results } = await api.enrichRefreshAll(kind, id);
		await reloadDetail();
		setStatus(refreshAllLine(results));
		const limited = results.filter((r) => r.status === 'rate_limited');
		if (limited.length) {
			setError(limited.map((r) => rateLimitedLine(r.provider, r.retry_after ?? 0)).join(' · '));
		}
		const needsReview = results.find((r) => r.status === 'needs_review');
		if (needsReview) openPicker(needsReview.provider);
	} catch (e) {
		setError(toMessage(e));
	} finally {
		setRefreshingAll(false);
	}
}
