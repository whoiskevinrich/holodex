// Shared System Activity store (F21). Polls the read-model so the header
// indicator and the /status page agree on one source of truth; the polling is
// transport-agnostic so SSE (F21.8) can later replace refresh() without touching
// consumers. capabilities is fetched once (ungated, rarely changes); activity is
// polled while the tab is visible.
import { api, ApiError, endSession } from './api';
import { adminMode } from './adminMode.svelte';
import type { Activity, Capabilities } from './types';
import { toMessage } from './format';

// Poll cadence for the activity read-model (ms). A property of the shared store.
const POLL_MS = 3000;

class ActivityState {
	data = $state<Activity | null>(null);
	caps = $state<Capabilities | null>(null);
	error = $state('');
	loading = $state(true);

	private timer: ReturnType<typeof setInterval> | null = null;
	private subscribers = 0;

	// active drives the header indicator: any background work in progress.
	get active(): boolean {
		const d = this.data;
		return !!d && (d.scan.state === 'running' || d.thumbnails.depth > 0);
	}

	// Capability predicates live here (next to caps) so every surface interprets
	// owner/locked state the same way (F21.7; future Pro-mode reuse).
	get isOwner(): boolean {
		return !!this.caps && this.caps.owner;
	}
	// Effective owner gate (F29): owner AND Admin mode on — the single source of truth
	// for showing owner-only UI. Every surface reads this (not bare `isOwner`) so a new
	// page can't silently forget the admin-mode half. Presentation only; the server
	// gate (ADR-030) stays the sole authority — toggling Admin mode changes no token.
	get effectiveOwner(): boolean {
		return this.isOwner && adminMode.enabled;
	}
	get needToken(): boolean {
		return !!this.caps && this.caps.auth_required && !this.caps.owner;
	}
	get cardLayout(): 'wide' | 'poster' {
		return this.caps?.card_layout ?? 'wide';
	}
	// Per-person gallery cap (F25). Falls back to 20 (the server default) until caps load.
	get galleryMax(): number {
		return this.caps?.person_gallery_max ?? 20;
	}

	async refresh() {
		try {
			this.data = await api.activity();
			this.error = '';
		} catch (e) {
			this.error = toMessage(e);
			// A 401 mid-session means the owner cookie expired or was revoked
			// (ADR-046) — fall back cleanly to the token prompt / read-only view.
			if (e instanceof ApiError && e.status === 401) await this.dropIfNotOwner();
		} finally {
			this.loading = false;
		}
	}

	// signOut ends the owner session (clears the cookie) and collapses the UI back
	// to the read-only / token-prompt state. Lives here so every caller (status
	// page, a future header control) logs out consistently (ADR-046).
	async signOut() {
		await endSession();
		await this.dropIfNotOwner();
	}

	// dropIfNotOwner re-reads capabilities and discards the now-unauthorized
	// activity data when the session is no longer the owner. Shared by the 401
	// fallback and explicit sign-out.
	private async dropIfNotOwner() {
		await this.refreshCaps();
		if (!this.isOwner) this.data = null;
	}

	async refreshCaps() {
		try {
			this.caps = await api.capabilities();
		} catch {
			// caps is ungated; a failure here just leaves controls hidden.
		}
	}

	// start begins polling; ref-counted so the indicator (in the layout) and the
	// /status page share one interval and the last leaver stops it. The cadence is
	// a store-level constant — a property of the shared resource, not the caller.
	start() {
		this.subscribers++;
		if (this.timer) return;
		this.refreshCaps();
		this.refresh();
		this.timer = setInterval(() => {
			if (typeof document === 'undefined' || document.visibilityState === 'visible') {
				this.refresh();
			}
		}, POLL_MS);
	}

	stop() {
		this.subscribers = Math.max(0, this.subscribers - 1);
		if (this.subscribers === 0 && this.timer) {
			clearInterval(this.timer);
			this.timer = null;
		}
	}
}

export const activity = new ActivityState();
