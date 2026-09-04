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
import type { EnrichEntityKind } from './types';

export async function runEnrichRefresh(
	kind: EnrichEntityKind,
	id: number,
	provider: string,
	setBusy: (v: string) => void,
	setError: (v: string) => void,
	reloadDetail: () => Promise<void>
): Promise<void> {
	setBusy(provider);
	setError('');
	try {
		await api.enrichRefresh(kind, id, provider);
		await reloadDetail();
	} catch (e) {
		setError(toMessage(e));
	} finally {
		setBusy('');
	}
}

// A provider that resolved ambiguously must not be silently dropped — openPicker is
// called with the first `needs_review` result so the owner sees it immediately.
export async function runEnrichRefreshAll(
	kind: EnrichEntityKind,
	id: number,
	setRefreshingAll: (v: boolean) => void,
	setError: (v: string) => void,
	reloadDetail: () => Promise<void>,
	openPicker: (provider: string) => void
): Promise<void> {
	setRefreshingAll(true);
	setError('');
	try {
		const { results } = await api.enrichRefreshAll(kind, id);
		await reloadDetail();
		const needsReview = results.find((r) => r.status === 'needs_review');
		if (needsReview) openPicker(needsReview.provider);
	} catch (e) {
		setError(toMessage(e));
	} finally {
		setRefreshingAll(false);
	}
}
