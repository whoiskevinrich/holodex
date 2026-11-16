<script lang="ts">
	// The Filters panel's Fields control (F90 handoff, option A): one row per chosen field with
	// a Has / Missing toggle and ×, then "+ Add a field…", a combobox over the list's fields.
	// The list only suggests, so focus stays in the input and ↑/↓ move aria-activedescendant
	// (this folder's approved exception, as the media tag input). The page owns the conditions.
	import type { FieldCond, FilterField } from '#lib/fieldConditions.ts';
	import SegmentedRadio from './SegmentedRadio.svelte';

	let {
		fields,
		conds,
		onchange
	}: {
		/** The list's fields in mapping order; null until they load. */
		fields: readonly FilterField[] | null;
		conds: readonly FieldCond[];
		onchange: (conds: FieldCond[]) => void;
	} = $props();

	const uid = $props.id();
	const listId = `${uid}-fields`;
	const HAS = [
		{ value: 'has', label: 'Has' },
		{ value: 'missing', label: 'Missing' }
	] as const;

	// The rows keep the order they were added in, though the URL may hold has before missing.
	let order = $state<string[]>([]);
	const labelOf = (key: string) => fields?.find((f) => f.key === key)?.label;
	// A field gone from the mapping draws no row; its chip shows the problem instead.
	const rows = $derived(
		conds
			.filter((c) => labelOf(c.key))
			.toSorted((a, b) => rank(a.key) - rank(b.key))
	);
	function rank(key: string) {
		const i = order.indexOf(key);
		return i < 0 ? order.length : i;
	}

	let text = $state('');
	let open = $state(false);
	let highlight = $state(0);
	const left = $derived((fields ?? []).filter((f) => !conds.some((c) => c.key === f.key)));
	const matches = $derived(left.filter((f) => f.label.toLowerCase().includes(text.trim().toLowerCase())));

	function set(key: string, has: boolean | null) {
		const next = conds.filter((c) => c.key !== key);
		if (has !== null) {
			const i = conds.findIndex((c) => c.key === key);
			next.splice(i < 0 ? next.length : i, 0, { key, has });
		}
		onchange(next);
	}
	function add(f: FilterField) {
		order = [...order.filter((k) => k !== f.key), f.key];
		set(f.key, true);
		text = '';
		highlight = 0;
	}
	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			if (open) e.stopPropagation();
			open = false;
			return;
		}
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			e.preventDefault();
			open = true;
			const n = matches.length;
			if (n) highlight = (highlight + (e.key === 'ArrowDown' ? 1 : n - 1)) % n;
		} else if (e.key === 'Enter' && open && matches[highlight]) {
			e.preventDefault();
			add(matches[highlight]);
		}
	}
	$effect(() => {
		if (highlight >= matches.length) highlight = 0;
	});
	// Seed the order from the conditions the panel opened with.
	$effect.pre(() => {
		const missing = conds.map((c) => c.key).filter((k) => !order.includes(k));
		if (missing.length) order = [...order, ...missing];
	});
</script>

<div>
	<span class="mb-1 block text-xs text-muted">Fields</span>
	{#each rows as c (c.key)}
		{@const label = labelOf(c.key) ?? c.key}
		<div class="flex items-center gap-2 py-0.5">
			<span class="min-w-0 flex-1 truncate text-sm text-ink" title={label}>{label}</span>
			<SegmentedRadio
				small
				options={HAS}
				value={c.has ? 'has' : 'missing'}
				label={`${label}: has or missing`}
				onchange={(v) => set(c.key, v === 'has')}
			/>
			<button type="button" class="btn-quiet px-1.5 text-sm" aria-label={`Remove ${label}`} onclick={() => set(c.key, null)}>
				×
			</button>
		</div>
	{/each}
	<div class="relative mt-1">
		<input
			type="text"
			role="combobox"
			aria-expanded={open}
			aria-controls={listId}
			aria-autocomplete="list"
			aria-activedescendant={open && matches[highlight] ? `${listId}-${highlight}` : undefined}
			aria-label="Add a field"
			placeholder={fields && !left.length ? 'All fields added' : '+ Add a field…'}
			disabled={!fields || !left.length}
			bind:value={text}
			onfocus={() => (open = true)}
			onblur={() => (open = false)}
			oninput={() => {
				open = true;
				highlight = 0;
			}}
			{onkeydown}
			class="w-full rounded-theme border border-rule bg-surface-2 px-2 py-1.5 text-sm text-ink placeholder:text-muted focus:border-accent focus:outline-none"
		/>
		{#if open && left.length}
			<div class="absolute left-0 right-0 top-full z-50 mt-1 rounded-theme border border-rule bg-surface shadow-lg">
				<p class="px-3 pt-2 pb-1 text-xs font-medium uppercase tracking-wide text-muted">Fields</p>
				{#if matches.length}
					<ul id={listId} role="listbox" aria-label="Fields" class="max-h-64 overflow-y-auto pb-1">
						{#each matches as f, i (f.key)}
							<!-- svelte-ignore a11y_click_events_have_key_events -->
							<li
								id={`${listId}-${i}`}
								role="option"
								aria-selected={i === highlight}
								onmousedown={(e) => e.preventDefault()}
								onclick={() => add(f)}
								onmouseenter={() => (highlight = i)}
								class="cursor-pointer truncate px-3 py-1.5 text-sm text-ink {i === highlight ? 'bg-surface-2' : 'hover:bg-surface-2'}"
								title={f.label}
							>
								{f.label}
							</li>
						{/each}
					</ul>
				{:else}
					<p id={listId} class="px-3 pb-2 text-sm text-muted">No field matches</p>
				{/if}
			</div>
		{/if}
	</div>
</div>
