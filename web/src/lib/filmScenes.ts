// A film's scene order (F56): numbered scenes ascending, unnumbered ones after all of
// them in whatever order the API returned (RD5 — no ordering guarantee among unnumbered
// scenes, so no secondary key). One function, so the film page's grid and a Play all run
// over the film (F75) can't disagree about what comes next.
export function sortScenes<T extends { scene_number?: number | null }>(scenes: readonly T[]): T[] {
	return [...scenes].sort((a, b) => {
		if (a.scene_number == null && b.scene_number == null) return 0;
		if (a.scene_number == null) return 1;
		if (b.scene_number == null) return -1;
		return a.scene_number - b.scene_number;
	});
}
