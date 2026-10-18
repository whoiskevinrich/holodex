// Owner view (F29, renamed from "Admin Mode" by F35 P2-2). A per-device view
// preference; turning it off hides ALL owner-only controls and data for a
// faithful visitor preview, letting the owner QA the public surface and declutter
// their own browsing.
//
// Presentation only: toggling never touches the admin token, capabilities, or
// any server authorization — the owner gate (ADR-030) stays the sole authority.
// The effective owner gate across the app is `activity.isOwner && ownerView.enabled`.
//
// Persisted to localStorage, default ON (so gaining
// admin visibly does something and controls don't stay hidden after unlock).
const KEY = 'holodex-owner-view';
// The pre-rename key. init() carries its value over once, then deletes it.
const LEGACY_KEY = 'holodex-admin-mode';

export class OwnerViewState {
	enabled = $state(true);

	// Transient screen-reader announcement for state changes that do NOT originate
	// from the toggle itself (auto-reveal on an owner-only route, P0-6) — the
	// switch's own aria-checked already announces manual flips. Rendered by an
	// aria-live region in the layout.
	announcement = $state('');

	// init reads the saved preference; call once on mount (beside theme.init()).
	// Only an explicit stored "false" disables it; an absent/garbage value keeps
	// the default ON (never throws if localStorage is unavailable). A value saved
	// under the legacy key migrates to the new key when the new one is absent.
	init() {
		if (typeof localStorage === 'undefined') return;
		const legacy = localStorage.getItem(LEGACY_KEY);
		if (legacy !== null) {
			if (localStorage.getItem(KEY) === null) localStorage.setItem(KEY, legacy);
			localStorage.removeItem(LEGACY_KEY);
		}
		if (localStorage.getItem(KEY) === 'false') this.enabled = false;
	}

	set(v: boolean) {
		this.enabled = v;
		if (typeof localStorage !== 'undefined') localStorage.setItem(KEY, String(v));
	}

	toggle() {
		this.set(!this.enabled);
	}

	// reveal forces Owner view ON and announces it — for auto-reveal when the owner
	// lands on an owner-only route while in visitor view (P0-6). No-op if already on.
	reveal() {
		if (this.enabled) return;
		this.set(true);
		this.announcement = 'Owner view on.';
	}
}

export const ownerView = new OwnerViewState();
