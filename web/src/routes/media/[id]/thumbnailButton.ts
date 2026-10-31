// The Maintenance → Thumbnail button's label (F85, writeback-poster-chooser-handoff.md §7): it
// opens the frame picker, and reads `Saving frame…` while the picked frame posts. "Reset to
// automatic" in the picker still regenerates, so that keeps its own busy label.
export function thumbnailButtonLabel(state: { savingFrame: boolean; regenerating: boolean }): string {
	if (state.savingFrame) return 'Saving frame…';
	return state.regenerating ? 'Regenerating…' : 'Pick a frame';
}
