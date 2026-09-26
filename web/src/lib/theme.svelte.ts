// Instance skin (F67, ADR-102). The skin is instance identity: the owner's choice
// arrives in /capabilities.theme and is applied to <html> for every viewer. There is
// no viewer preference any more (ADR-021 §5 superseded) — the only localStorage use
// is a paint cache of the last server-applied value, read once before first paint so
// a non-default instance doesn't flash Cinémathèque on a cold load. The server value
// always overwrites it (spec R7/R8).
import { api } from '$lib/api';
import { paletteMode } from '$lib/halo';
import type { HaloMode, ShippedTheme, ThemeCapability, ThemeCustom, ThemeId } from '$lib/types';

export const THEMES: readonly ShippedTheme[] = ['cinematheque', 'broadcast', 'brutalist'] as const;

export const THEME_LABELS: Record<ShippedTheme, string> = {
	cinematheque: 'Cinémathèque',
	broadcast: 'Broadcast',
	brutalist: 'Brutalist'
};

// The five primaries an owner may set (spec R9); everything else derives in app.css.
export const CUSTOM_TOKENS = ['bg', 'ink', 'accent', 'muted', 'warn'] as const;

const DEFAULT: ShippedTheme = 'cinematheque';
// Deliberately not the old `holodex-theme` preference key, so nothing reads a stale
// preference as if it were the instance skin.
const CACHE_KEY = 'holodex-theme-cache';

// The server only ever emits #rrggbb (spec R9); the cache is held to the same shape so a
// tampered entry cannot put anything but a colour into a custom property.
const HEX = /^#[0-9a-f]{6}$/i;

function isShipped(v: unknown): v is ShippedTheme {
	return typeof v === 'string' && (THEMES as readonly string[]).includes(v);
}

// isCustom guards a cached palette before it reaches <html>: a missing token would
// otherwise land as the literal string "undefined" and break every color until
// /capabilities arrives.
function isCustom(v: unknown): v is ThemeCustom {
	if (typeof v !== 'object' || v === null) return false;
	const c = v as Partial<ThemeCustom>;
	return (
		isShipped(c.base) &&
		typeof c.tokens === 'object' &&
		c.tokens !== null &&
		CUSTOM_TOKENS.every((k) => HEX.test(c.tokens?.[k] ?? ''))
	);
}

// baseOf resolves the data-theme to put on <html>: the shipped skin itself, or the
// custom palette's base (the flourishes and fonts it inherits, ADR-102 D4).
function baseOf(t: ThemeCapability): ShippedTheme {
	if (t.active === 'custom' && t.custom && isShipped(t.custom.base)) return t.custom.base;
	return isShipped(t.active) ? t.active : DEFAULT;
}

function readCache(): ThemeCapability | null {
	try {
		const raw = localStorage.getItem(CACHE_KEY);
		if (!raw) return null;
		const parsed = JSON.parse(raw) as ThemeCapability;
		if (typeof parsed !== 'object' || parsed === null || typeof parsed.active !== 'string') return null;
		if (parsed.custom !== null && !isCustom(parsed.custom)) return null;
		return parsed;
	} catch {
		return null; // blocked storage or a corrupt entry: render the default, fix on arrival
	}
}

function writeCache(t: ThemeCapability) {
	try {
		const next = JSON.stringify(t);
		// capabilities polls for the life of the tab; only touch storage on a real change.
		if (localStorage.getItem(CACHE_KEY) !== next) localStorage.setItem(CACHE_KEY, next);
	} catch {
		// private window / blocked storage — the next load just flashes once
	}
}

export class ThemeState {
	// current is the shipped skin on <html> (data-theme) — for a custom palette, its
	// base. Consumers that need a skin id for assets (e.g. ?skin= on placeholder
	// SVGs) read this and always get a shipped id.
	current = $state<ShippedTheme>(DEFAULT);
	// active is the instance's chosen id, including 'custom'.
	active = $state<ThemeId>(DEFAULT);
	// custom is the configured palette, or null when none is (spec R3).
	custom = $state<ThemeCustom | null>(null);
	// mode is the active palette's brightness (HOLODEX-463): every shipped skin is dark;
	// a custom palette is light when its --bg is. Mirrored to <html data-mode>, which is
	// what app.css keys the per-mode studio image halo on.
	mode = $state<HaloMode>('dark');

	// init applies the paint cache, if any, before /capabilities arrives.
	init() {
		const cached = readCache();
		if (cached) this.apply(cached);
	}

	// applyServer is the authoritative path: called whenever /capabilities loads.
	applyServer(t: ThemeCapability) {
		this.apply(t);
		writeCache(t);
	}

	// select is the owner's action (S2's Appearance tab): optimistic apply, persist,
	// revert on failure. Resolves to true when the server accepted it.
	async select(id: ThemeId): Promise<boolean> {
		// The server would 400 anyway (spec R2); refusing here avoids an optimistic
		// flash to the default skin before the revert.
		if (id === 'custom' && !this.custom) return false;
		const previous: ThemeCapability = { active: this.active, custom: this.custom };
		this.apply({ active: id, custom: this.custom });
		try {
			this.applyServer(await api.setTheme(id));
			return true;
		} catch {
			this.apply(previous);
			return false;
		}
	}

	private apply(t: ThemeCapability) {
		const base = baseOf(t);
		const custom = t.active === 'custom' && t.custom ? t.custom : null;
		this.current = base;
		this.active = custom ? 'custom' : base;
		this.custom = t.custom ?? null;
		this.mode = custom ? paletteMode(custom.tokens.bg) : 'dark';
		if (typeof document === 'undefined') return;
		const root = document.documentElement;
		root.dataset.theme = base;
		root.dataset.mode = this.mode;
		// data-palette switches on app.css's derivation block (ADR-102 D4): every
		// pair-partner token is computed from the five primaries below.
		if (custom) root.dataset.palette = 'custom';
		else delete root.dataset.palette;
		// Inline primaries outrank every [data-theme] block (ADR-102 D4); clearing them
		// returns the base to its stock values with no residue (spec R4).
		for (const k of CUSTOM_TOKENS) {
			if (custom) root.style.setProperty(`--${k}`, custom.tokens[k]);
			else root.style.removeProperty(`--${k}`);
		}
	}
}

export const theme = new ThemeState();
