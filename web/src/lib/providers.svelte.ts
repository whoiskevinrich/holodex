// Shared provider directory (ADR-059, HOLODEX-134). The public /providers list maps a
// provider name to its self-hosted brand icon URL (and entity types), so provenance
// badges, the enrich controls, and the website label can all render a provider glyph
// with a monogram fallback — without each caller refetching. Fetched once, lazily;
// providers change only on a config reload, so there is no polling (refresh() re-pulls).
import { api } from './api';
import type { EnrichSource } from './types';

class ProvidersStore {
	// Keyed by provider name. $state so a component reading iconUrl(name) in markup
	// re-renders once load() resolves.
	private byName = $state<Record<string, EnrichSource>>({});
	private loaded = false;
	private inflight: Promise<void> | null = null;

	// load fetches the directory once (idempotent); concurrent callers share the one
	// request. Best-effort — a failure leaves the map empty so every caller falls back
	// to a monogram rather than erroring.
	load(): Promise<void> {
		if (this.loaded) return Promise.resolve();
		if (this.inflight) return this.inflight;
		this.inflight = api
			.providers()
			.then((res) => {
				const next: Record<string, EnrichSource> = {};
				for (const p of res.providers ?? []) next[p.name] = p;
				this.byName = next;
				this.loaded = true;
			})
			.catch(() => {
				// Leave the map empty; callers render monograms.
			})
			.finally(() => {
				this.inflight = null;
			});
		return this.inflight;
	}

	// iconUrl returns the served brand-icon URL for a provider, or '' when none is
	// cached (the caller renders a monogram). Reactive once load() resolves.
	iconUrl(name: string): string {
		return this.byName[name]?.icon_url ?? '';
	}

	// refresh forces a re-fetch after the registry may have changed (config reload).
	async refresh(): Promise<void> {
		this.loaded = false;
		this.inflight = null;
		await this.load();
	}
}

export const providers = new ProvidersStore();
