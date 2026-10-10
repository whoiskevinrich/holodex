// Held ForwardAuth re-auth (HOLODEX-502). When the upstream auth proxy session
// lapses, api.ts recovers with a top-level reload (HOLODEX-127) — but a reload
// would wipe unsaved input, and a write the proxy redirected never reached Holodex.
// So the reload is *held* when there is something to lose, and the layout shows a
// banner instead. This module is the state both sides read; the decision lives in
// api.ts (triggerReauth / checkRedirect).

// Why the reload is held:
//   'edit'     — the session lapsed while the page has unsaved input; nothing lost yet.
//   'write'    — a write was redirected by the proxy, so that change was not saved.
//   'resubmit' — the session is back after a 'write'; the change still needs saving.
export type ReauthHold = 'edit' | 'write' | 'resubmit';

class ReauthState {
	hold = $state<ReauthHold | null>(null);
	// Input since the last successful write or page change. Plain (not $state): only
	// api.ts reads it, at the moment it decides whether to reload.
	dirty = false;
}

export const reauth = new ReauthState();

// editing: is there input a reload would lose? Typed input (dirty), or an open
// dialog — click-driven edits (pickers, the crop editor, radio cards) fire no
// input event, and an open dialog is where an edit in progress lives.
export function editing(): boolean {
	return reauth.dirty || (typeof document !== 'undefined' && !!document.querySelector('[role="dialog"]'));
}

// trackEdits marks the page dirty on any user input outside the global search box
// (typing a search is not an edit worth holding a reload for). Document-level, so
// no edit control has to register itself. Returns the cleanup for an $effect.
export function trackEdits(): () => void {
	function onInput(e: Event) {
		const el = e.target as Element | null;
		if (el?.closest?.('[data-search-box]')) return;
		reauth.dirty = true;
	}
	document.addEventListener('input', onInput, true);
	document.addEventListener('change', onInput, true);
	return () => {
		document.removeEventListener('input', onInput, true);
		document.removeEventListener('change', onInput, true);
	};
}
