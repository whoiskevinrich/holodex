<script lang="ts">
	// The ordered template editor (handoff calls 10, 11, 12, 15): one row per template with ↑ ↓ ×,
	// + Add template, and token chips that insert at the cursor of the template last edited.
	import { tick } from 'svelte';
	import { nf } from '#lib/rename.ts';

	export interface TemplateItem {
		id: number;
		text: string;
	}

	let {
		items = $bindable(),
		errors = [],
		counts = [],
		providers = [],
		lockLast = true,
		onedit
	}: {
		items: TemplateItem[];
		// One message per row ('' for a valid row), from the last Preview or Save.
		errors?: string[];
		// Per-template counts from the plan, null where the plan didn't use this row's text.
		counts?: ({ names: number; wont_read_back: number } | null)[];
		providers?: string[];
		// × is withdrawn on the last template while the mode isn't Off (handoff call 15).
		lockLast?: boolean;
		onedit?: (index: number) => void;
	} = $props();

	const tokens = $derived([
		'{title}',
		'{studio}',
		'{people}',
		'{year}',
		'{edition}',
		'{part}',
		...providers.map((p) => `{${p}-id}`)
	]);

	let root: HTMLElement;
	let nextId = 0;
	// Where a chip inserts: the last template input that had focus, and its selection then.
	let last: { id: number; start: number; end: number } | null = null;

	function el(id: number, act: string): HTMLElement | null {
		return root?.querySelector(`[data-row="${id}"] [data-act="${act}"]`) ?? null;
	}

	async function move(i: number, delta: number, act: string) {
		const j = i + delta;
		if (j < 0 || j >= items.length) return;
		const next = items.slice();
		[next[i], next[j]] = [next[j], next[i]];
		const id = items[i].id;
		const input = act === 'input' ? (el(id, 'input') as HTMLInputElement | null) : null;
		const caret = input ? [input.selectionStart, input.selectionEnd] : null;
		items = next;
		onedit?.(-1);
		await tick();
		const target = el(id, act);
		target?.focus();
		if (caret && target instanceof HTMLInputElement) target.setSelectionRange(caret[0], caret[1]);
	}

	function remove(i: number) {
		if (lockLast && items.length <= 1) return;
		items = items.filter((_, k) => k !== i);
		onedit?.(-1);
	}

	async function add() {
		const id = Date.now() + nextId++;
		items = [...items, { id, text: '' }];
		onedit?.(-1);
		await tick();
		el(id, 'input')?.focus();
	}

	function edit(i: number, text: string) {
		items = items.map((t, k) => (k === i ? { ...t, text } : t));
		onedit?.(i);
	}

	function keydown(e: KeyboardEvent, i: number) {
		if (!e.altKey || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return;
		e.preventDefault();
		move(i, e.key === 'ArrowUp' ? -1 : 1, 'input');
	}

	function remember(id: number, input: HTMLInputElement) {
		last = { id, start: input.selectionStart ?? input.value.length, end: input.selectionEnd ?? input.value.length };
	}

	async function insert(token: string) {
		if (items.length === 0) return;
		const active = document.activeElement;
		if (active instanceof HTMLInputElement && root.contains(active)) {
			const row = active.closest<HTMLElement>('[data-row]');
			if (row) remember(Number(row.dataset.row), active);
		}
		let i = last ? items.findIndex((t) => t.id === last!.id) : -1;
		let start: number, end: number;
		if (i < 0) {
			i = items.length - 1;
			start = end = items[i].text.length;
		} else {
			({ start, end } = last!);
		}
		const text = items[i].text;
		edit(i, text.slice(0, start) + token + text.slice(end));
		const caret = start + token.length;
		last = { id: items[i].id, start: caret, end: caret };
		await tick();
		const input = el(items[i].id, 'input') as HTMLInputElement | null;
		input?.focus();
		input?.setSelectionRange(caret, caret);
	}

	const withdrawn = 'text-rule hover:no-underline cursor-default';
</script>

<div bind:this={root} class="space-y-2">
	<ol class="space-y-2">
		{#each items as item, i (item.id)}
			{@const err = errors[i]}
			{@const c = counts[i]}
			{@const first = i === 0}
			{@const lastRow = i === items.length - 1}
			{@const noRemove = lockLast && items.length <= 1}
			<li data-row={item.id}>
				<div class="flex flex-wrap items-center gap-x-2 gap-y-1">
					<span class="w-4 shrink-0 text-right font-mono text-xs text-muted">{i + 1}</span>
					<input
						data-act="input"
						type="text"
						spellcheck="false"
						autocomplete="off"
						aria-label="Template {i + 1}"
						aria-invalid={err ? 'true' : undefined}
						aria-describedby={err ? `template-error-${item.id}` : undefined}
						value={item.text}
						oninput={(e) => edit(i, e.currentTarget.value)}
						onkeydown={(e) => keydown(e, i)}
						onblur={(e) => remember(item.id, e.currentTarget)}
						class="w-full min-w-0 rounded-theme border bg-bg px-2.5 py-1.5 font-mono text-sm text-ink md:w-[420px] {err
							? 'border-warn'
							: 'border-rule'}"
					/>
					<span class="flex items-center gap-1 max-md:pl-6">
						{#if c}
							<span class="mr-2 text-xs text-muted">
								names {nf.format(c.names)}{#if c.wont_read_back > 0}<span class="text-warn"> · {nf.format(c.wont_read_back)} won't read back</span>{/if}
							</span>
						{/if}
						<button
							type="button"
							data-act="up"
							class="btn-row btn-quiet rounded-theme px-1.5 {first ? withdrawn : ''}"
							aria-label="Move template {i + 1} up"
							aria-disabled={first}
							onclick={() => move(i, -1, 'up')}>↑</button
						>
						<button
							type="button"
							data-act="down"
							class="btn-row btn-quiet rounded-theme px-1.5 {lastRow ? withdrawn : ''}"
							aria-label="Move template {i + 1} down"
							aria-disabled={lastRow}
							onclick={() => move(i, 1, 'down')}>↓</button
						>
						<button
							type="button"
							data-act="remove"
							class="btn-row btn-quiet rounded-theme px-1.5 {noRemove ? withdrawn : ''}"
							aria-label="Remove template {i + 1}"
							aria-disabled={noRemove}
							onclick={() => remove(i)}>×</button
						>
					</span>
				</div>
				{#if err}
					<p id="template-error-{item.id}" class="mt-1 pl-6 text-xs text-warn">{err}</p>
				{/if}
			</li>
		{/each}
	</ol>
	<button type="button" class="btn-quiet rounded-theme px-3 py-1.5 text-sm" onclick={add}>+ Add template</button>
	<div class="flex flex-wrap items-center gap-1.5">
		<span class="text-xs text-muted">Insert a token:</span>
		{#each tokens as t (t)}
			<button
				type="button"
				class="rounded-full border border-rule bg-bg px-2 py-0.5 font-mono text-xs text-ink hover:bg-surface-2"
				onmousedown={(e) => e.preventDefault()}
				onclick={() => insert(t)}>{t}</button
			>
		{/each}
	</div>
	<p class="text-xs text-muted">Inserts at the cursor in the template you were last editing.</p>
</div>
