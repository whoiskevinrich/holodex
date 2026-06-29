// Admin Mode (F29). A per-device view preference that hides ALL owner-only
// controls and data for a faithful "visitor view", letting the owner QA the
// public surface (across all three skins) and declutter their own browsing.
//
// Presentation only: toggling never touches the admin token, capabilities, or
// any server authorization — the owner gate (ADR-030) stays the sole authority.
// The effective owner gate across the app is `activity.isOwner && adminMode.enabled`.
//
// Mirrors theme.svelte.ts: persisted to localStorage, default ON (so gaining
// admin visibly does something and controls don't stay hidden after unlock).
const KEY = 'holodex-admin-mode';

export class AdminModeState {
	enabled = $state(true);

	// Transient screen-reader announcement for state changes that do NOT originate
	// from the toggle itself (auto-reveal on an owner-only route, P0-6) — the
	// switch's own aria-checked already announces manual flips. Rendered by an
	// aria-live region in the layout.
	announcement = $state('');

	// init reads the saved preference; call once on mount (beside theme.init()).
	// Only an explicit stored "false" disables it; an absent/garbage value keeps
	// the default ON (never throws if localStorage is unavailable).
	init() {
		if (typeof localStorage !== 'undefined' && localStorage.getItem(KEY) === 'false') {
			this.enabled = false;
		}
	}

	set(v: boolean) {
		this.enabled = v;
		if (typeof localStorage !== 'undefined') localStorage.setItem(KEY, String(v));
	}

	toggle() {
		this.set(!this.enabled);
	}

	// reveal forces Admin mode ON and announces it — for auto-reveal when the owner
	// lands on an owner-only route while in visitor view (P0-6). No-op if already on.
	reveal() {
		if (this.enabled) return;
		this.set(true);
		this.announcement = 'Owner view on.';
	}
}

export const adminMode = new AdminModeState();
