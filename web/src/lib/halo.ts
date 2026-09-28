// Studio image halo (HOLODEX-463, ADR-109): the owner turns the `.logo-halo` glow on per
// image role, saved per palette mode. Cinémathèque is the only look and it is dark
// (ADR-115), so the mode in force is always PALETTE_MODE; a saved "light" choice is kept
// but never glows. The rendering side is pure CSS (app.css glows `.halo-dark`), so a
// component only has to emit the classes.
import type { HaloMode } from '$lib/types';

// PALETTE_MODE is the mode the halo toggle reads and writes: Cinémathèque is dark.
export const PALETTE_MODE: HaloMode = 'dark';

// haloClass maps a role's saved modes to the CSS hooks; none (the default) is no halo.
export function haloClass(modes: readonly HaloMode[] | undefined): string {
	return (modes ?? []).map((m) => `halo-${m}`).join(' ');
}
