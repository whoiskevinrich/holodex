// Shared button/wrapper classes for the segmented sort toggles (SortToggle,
// CompletenessSortToggle) — same active/inactive treatment and pill shape,
// factored out so the two components don't carry duplicate copies.
export function segmentedToggleClass(active: boolean): string {
	return active ? 'bg-accent px-3 py-1 text-accent-ink' : 'px-3 py-1 text-muted hover:text-ink';
}

export const segmentedToggleWrapperClass = 'flex overflow-hidden rounded-theme border border-rule text-sm';
