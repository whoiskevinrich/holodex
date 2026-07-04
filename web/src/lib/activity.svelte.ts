// Shared System Activity store (F21). Polls the read-model so the header
// indicator and the /status page agree on one source of truth; the polling is
// transport-agnostic so SSE (F21.8) can later replace refresh() without touching
// consumers. capabilities is fetched once (ungated, rarely changes); activity is
// polled while the tab is visible.
import { api, ApiError, ReauthError, endSession } from './api';
import { adminMode } from './adminMode.svelte';
import type { Activity, Capabilities } from './types';
import { toMessage } from './format';

// Poll cadence for the activity read-model (ms). A property of the shared store.
const POLL_MS = 3000;
// Backoff ceiling: on sustained failure the poll interval doubles up to this, so a
// down/unreachable endpoint is retried gently instead of hammered every 3 s
// (HOLODEX-127 observed hundreds of doomed requests during one lapse).
const MAX_POLL_MS = 30000;
// Tolerate this many consecutive failures before surfacing a hard error, so a single
// transient/opaque blip doesn't blank the surface (HOLODEX-127 acceptance criteria).
const FAIL_GRACE = 3;

class ActivityState {
	data = $state<Activity | null>(null);
	caps = $state<Capabilities | null>(null);
	error = $state('');
	loading = $state(true);

	private timer: ReturnType<typeof setInterval> | null = null;
	private subscribers = 0;
	// Consecutive-failure count + current backoff delay drive the poll gate below.
	private failures = 0;
	private delay = POLL_MS;
	// Epoch ms before which the (still-ticking) interval skips its refresh — how the
	// backoff is applied without tearing down the shared, ref-counted interval.
	private nextPollAt = 0;

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
			this.failures = 0;
			this.delay = POLL_MS;
			this.nextPollAt = 0;
		} catch (e) {
			if (e instanceof ReauthError) {
				// The upstream ForwardAuth session lapsed (HOLODEX-127). A top-level
				// re-auth is already underway (api.ts) — keep the last-good surface and
				// don't flash an error before the document reloads.
				return;
			}
			if (e instanceof ApiError && e.status === 401) {
				// Holodex's *own* owner cookie expired or was revoked (ADR-046) — fall
				// back cleanly to the token prompt / read-only view. Distinct from the
				// ForwardAuth (Authentik) expiry handled above.
				this.error = toMessage(e);
				await this.dropIfNotOwner();
				return;
			}
			// Transient/opaque failure (e.g. a network blip). Keep the last-good `data`
			// on screen; only surface a hard error once failures are sustained, and back
			// off the poll so a persistently-down endpoint isn't hammered.
			this.failures++;
			if (this.failures >= FAIL_GRACE) this.error = toMessage(e);
			this.delay = Math.min(this.delay * 2, MAX_POLL_MS);
			this.nextPollAt = Date.now() + this.delay;
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
		// The interval keeps ticking at POLL_MS, but a refresh only fires when the tab
		// is visible AND we're past any backoff window — so a failing endpoint backs
		// off (nextPollAt) without disturbing the shared, ref-counted interval.
		this.timer = setInterval(() => {
			const visible = typeof document === 'undefined' || document.visibilityState === 'visible';
			if (visible && Date.now() >= this.nextPollAt) {
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
