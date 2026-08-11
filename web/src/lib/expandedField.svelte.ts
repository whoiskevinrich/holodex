// F56.9 — at most one SourceBadge expanded across a detail page at a time. A module-level
// singleton (mirrors adminMode.svelte.ts/theme.svelte.ts) rather than a page-owned
// PopoverMenu instance, because SourceBadge's prop contract (field/decide/baselineKey only,
// per the two-tier field editing design handoff) doesn't thread an expand/collapse callback
// through three separate detail pages (Video/Person/Studio) — every SourceBadge instance
// coordinates through this instead. Keyed by canonical field name; expanding one field
// collapses whichever other field (if any) was open, discarding its staged selection.
class ExpandedFieldState {
	key = $state<string | null>(null);

	isOpen(canonical: string): boolean {
		return this.key === canonical;
	}

	expand(canonical: string) {
		this.key = canonical;
	}

	close() {
		this.key = null;
	}

	// Callers reset this on their own id-driven reload/navigation trigger (media/person/
	// studio detail pages) — the singleton has no entity scope of its own, so a field left
	// expanded on one entity would otherwise render pre-expanded on the next same-type
	// entity navigated to.
	reset() {
		this.key = null;
	}
}

export const expandedField = new ExpandedFieldState();
