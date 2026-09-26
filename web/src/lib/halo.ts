// Studio image halo (HOLODEX-463, ADR-109): the owner turns the `.logo-halo` glow on per
// image role, saved separately for a dark and a light palette. Holodex has no light
// mode; "light" is a custom palette whose --bg is bright, so the mode is derived from
// that one colour. The rendering side is pure CSS (app.css keys `.halo-dark` /
// `.halo-light` on <html data-mode>), so a component only has to emit the classes.
import type { HaloMode } from '$lib/types';

// WCAG relative luminance of a #rrggbb colour.
function luminance(hex: string): number {
	const [r, g, b] = [1, 3, 5].map((i) => {
		const c = parseInt(hex.slice(i, i + 2), 16) / 255;
		return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
	});
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

// paletteMode classifies a background as light or dark. 0.179 is the luminance at which
// black and white text reach equal contrast on it — above it, dark ink reads better, so
// the page is a light one. A malformed value falls back to dark (every shipped skin).
export function paletteMode(bgHex: string | undefined): HaloMode {
	if (!bgHex || !/^#[0-9a-f]{6}$/i.test(bgHex)) return 'dark';
	return luminance(bgHex) > 0.179 ? 'light' : 'dark';
}

// haloClass maps a role's saved modes to the CSS hooks; none (the default) is no halo.
export function haloClass(modes: readonly HaloMode[] | undefined): string {
	return (modes ?? []).map((m) => `halo-${m}`).join(' ');
}
