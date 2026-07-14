// groupByKind buckets items by a per-item kind into the caller's fixed section
// order, dropping empty groups — the shared shape behind the owner Duplicates
// and Enrichment queues (both render `{groupLabel[g.type]} · {g.items.length}`
// sections). `sortItems`, when given, runs per-group (e.g. the Enrichment
// queue's actionable-first ordering) so it stays a caller concern.
export function groupByKind<K extends string, T>(
	items: T[],
	order: readonly K[],
	kindOf: (item: T) => K,
	sortItems?: (items: T[]) => T[]
): { type: K; items: T[] }[] {
	return order
		.map((k) => {
			const group = items.filter((item) => kindOf(item) === k);
			return { type: k, items: sortItems ? sortItems(group) : group };
		})
		.filter((g) => g.items.length > 0);
}
