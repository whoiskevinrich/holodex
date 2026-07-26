<script lang="ts">
	// Typeahead multi-select for a facet (people or tags) — F4.2/F4.3. Filters the
	// pre-fetched option list client-side (fine at personal-library scale) and
	// binds the chosen ids back to the caller, which feeds them into the query.
	type Option = { id: number; name: string; video_count?: number };

	let {
		label,
		items,
		selected = $bindable()
	}: { label: string; items: Option[]; selected: number[] } = $props();

	let query = $state('');
	let open = $state(false);

	const selectedItems = $derived(items.filter((i) => selected.includes(i.id)));
	const matches = $derived.by(() => {
		const needle = query.trim().toLowerCase();
		if (!needle) return [];
		return items
			.filter((i) => !selected.includes(i.id) && i.name.toLowerCase().includes(needle))
			.slice(0, 8);
	});

	function add(id: number) {
		if (!selected.includes(id)) selected = [...selected, id];
		query = '';
		open = false;
	}
	function remove(id: number) {
		selected = selected.filter((s) => s !== id);
	}
	function onKey(e: KeyboardEvent) {
		if (e.key === 'Enter' && matches.length) {
			e.preventDefault();
			add(matches[0].id);
		} else if (e.key === 'Escape') {
			open = false;
		}
	}
</script>

<div class="relative min-w-[12rem]">
	<span class="mb-1 block text-xs text-muted">{label}</span>
	<div class="flex flex-wrap items-center gap-1 rounded-theme border border-rule bg-surface px-2 py-1.5">
		{#each selectedItems as it (it.id)}
			<span class="flex items-center gap-1 rounded-theme bg-accent px-1.5 py-0.5 text-xs text-accent-ink">
				{it.name}
				<button onclick={() => remove(it.id)} aria-label={`Remove ${it.name}`} class="leading-none">×</button>
			</span>
		{/each}
		<input
			bind:value={query}
			onfocus={() => (open = true)}
			oninput={() => (open = true)}
			onblur={() => setTimeout(() => (open = false), 120)}
			onkeydown={onKey}
			placeholder={selectedItems.length ? '' : `Any ${label.toLowerCase()}`}
			class="min-w-[6rem] flex-1 bg-transparent text-sm text-ink outline-none placeholder:text-muted"
		/>
	</div>

	{#if open && matches.length}
		<ul class="absolute z-10 mt-1 max-h-56 w-full overflow-auto rounded-theme border border-rule bg-surface py-1 text-sm shadow-lg">
			{#each matches as m (m.id)}
				<li>
					<button
						onclick={() => add(m.id)}
						class="flex w-full items-center justify-between px-3 py-1.5 text-left text-ink hover:bg-surface-2"
					>
						<span>{m.name}</span>
						{#if m.video_count}<span class="text-xs text-muted">{m.video_count}</span>{/if}
					</button>
				</li>
			{/each}
		</ul>
	{/if}
</div>
