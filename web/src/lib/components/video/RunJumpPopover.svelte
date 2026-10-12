<script lang="ts">
	import { onMount, tick, untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import type { MediaBrief } from '#lib/types.ts';
	import { api } from '#lib/api.ts';
	import { formatDuration } from '#lib/format.ts';
	import { setPlayIntent } from '#lib/playlistContext.ts';
	import { runHref, type Run } from '#lib/run.ts';
	import {
		ROW_PX,
		centredScrollTop,
		matchIndices,
		parseJumpQuery,
		splitMatch,
		viewportPx,
		windowRange
	} from '#lib/runJump.ts';

	// The run strip's jump list (spec RD14 / P0-13; run-position-jump-handoff.md). Every item
	// in the run, in play order, in a virtualised list that scrolls with the wheel. The query
	// box takes a position (scrolls to it, no filter) or title words (filters, keeping run
	// positions). Rows are links, so a middle-click opens a tab with no autoplay; a press sets
	// the play intent, as the strip's Next does. Dismissal is the parent's `use:dismissable`.
	let { run, current, onclose }: { run: Run; current: number; onclose: () => void } = $props();

	const order = $derived(run.order);
	let details = $state<Map<number, MediaBrief> | null>(null);
	let loadFailed = $state(false);
	let query = $state('');
	let scrollTop = $state(0);
	/** Index into `rows` of the highlighted row: the roving tabindex lands here. */
	let active = $state(0);

	let input = $state<HTMLInputElement>();
	let list = $state<HTMLDivElement>();
	let panel = $state<HTMLDivElement>();
	/** Leftward nudge so a trigger far right doesn't push the panel off-screen (16px gutter). */
	let nudge = $state(0);

	const parsed = $derived(parseJumpQuery(query, order.length));
	const searching = $derived(parsed.kind === 'words' && details == null && !loadFailed);
	/** The run indices shown: the whole run, or a title search's matches. */
	const rows = $derived.by(() => {
		if (parsed.kind !== 'words') return order.map((_, i) => i);
		if (!details) return [];
		const d = details;
		return matchIndices(order, (id) => d.get(id)?.title, parsed.text);
	});
	const vp = $derived(viewportPx(rows.length));
	const [start, end] = $derived(windowRange(scrollTop, vp, rows.length));
	/** The rows to draw: the window, plus the roving-tabindex row wherever it is, so keyboard
	 *  focus survives being scrolled away from and Tab always has a row to land on. */
	const drawn = $derived.by(() => {
		const out: number[] = [];
		for (let r = start; r < end; r++) out.push(r);
		if (active < rows.length && (active < start || active >= end)) out.push(active);
		return out;
	});

	// A list re-mounted after the no-match or searching state starts at DOM scrollTop 0; put it
	// where the window thinks it is, or the drawn rows sit outside the viewport.
	$effect(() => {
		if (list) list.scrollTop = untrack(() => scrollTop);
	});

	/** The failure shows whenever the box isn't a position, which still works without titles. */
	const titlesFailed = $derived(loadFailed && parsed.kind !== 'number');
	const caption = $derived.by(() => {
		if (parsed.kind === 'number') return `Enter plays #${parsed.index + 1}`;
		if (parsed.kind !== 'words') return '';
		if (searching) return 'Searching…';
		return rows.length === 1 ? '1 match' : `${rows.length} matches`;
	});

	function scrollToRow(row: number) {
		scrollTop = centredScrollTop(row, vp, rows.length);
		if (list) list.scrollTop = scrollTop;
	}

	onMount(() => {
		active = current;
		input?.focus();
		// clientWidth, not innerWidth: the latter grows with the very overflow being corrected.
		const room = document.documentElement.clientWidth - 16;
		if (panel) nudge = Math.max(0, panel.getBoundingClientRect().right - room);
		// After the first render sizes the list, so the centring has a height to work with.
		tick().then(() => scrollToRow(current));
		let cancelled = false;
		api.mediaBrief(order).then(
			(items) => {
				if (!cancelled) details = new Map(items.map((b) => [b.id, b]));
			},
			() => {
				if (!cancelled) loadFailed = true;
			}
		);
		return () => (cancelled = true);
	});

	// A new query re-aims the list: a position scrolls to that row, words go to the top.
	function onQuery() {
		if (parsed.kind === 'number') {
			active = parsed.index;
			scrollToRow(active);
		} else if (parsed.kind === 'words') {
			active = 0;
			scrollToRow(0);
		} else if (parsed.kind === 'empty') {
			active = current;
			scrollToRow(current);
		}
	}

	function play(index: number) {
		const id = order[index];
		onclose();
		if (index === current) return;
		setPlayIntent(id);
		goto(runHref(id, run));
	}

	async function focusRow(row: number) {
		if (!rows.length) return;
		active = Math.max(0, Math.min(rows.length - 1, row));
		const top = active * ROW_PX;
		if (list && (top < list.scrollTop || top + ROW_PX > list.scrollTop + vp)) scrollToRow(active);
		await tick();
		list?.querySelector<HTMLElement>(`[data-row="${active}"]`)?.focus();
	}

	function onInputKey(e: KeyboardEvent) {
		if (e.key === 'ArrowDown') {
			e.preventDefault();
			focusRow(active);
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (parsed.kind === 'number') play(parsed.index);
			else if (parsed.kind === 'words' && rows.length) play(rows[0]);
		}
	}

	function onRowKey(e: KeyboardEvent, row: number) {
		const step: Record<string, number> = { ArrowDown: 1, ArrowUp: -1, PageDown: 8, PageUp: -8 };
		if (e.key in step) {
			e.preventDefault();
			if (e.key === 'ArrowUp' && row === 0) input?.focus();
			else focusRow(row + step[e.key]);
		} else if (e.key === 'Home' || e.key === 'End') {
			e.preventDefault();
			focusRow(e.key === 'Home' ? 0 : rows.length - 1);
		} else if (e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) {
			// Typing in the list goes back to the box, carrying the character.
			e.preventDefault();
			query += e.key;
			input?.focus();
			onQuery();
		}
	}

	function onRowClick(e: MouseEvent, index: number) {
		// A modified click is the browser's (new tab / window): no intent, nothing closes.
		if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
		e.preventDefault();
		play(index);
	}
</script>

<div
	bind:this={panel}
	style:translate={nudge ? `-${nudge}px 0` : undefined}
	role="dialog"
	aria-label="Go to a position in this run"
	class="absolute left-0 right-0 top-full z-20 mt-1 rounded-theme border border-rule bg-surface p-2.5 text-sm sm:right-auto sm:w-[26rem]"
>
	<div class="relative">
		<input
			bind:this={input}
			bind:value={query}
			oninput={onQuery}
			onkeydown={onInputKey}
			placeholder="Go to # or search titles"
			aria-label="Go to a position or search titles"
			aria-describedby="run-jump-caption"
			class="w-full rounded-theme border bg-bg px-2.5 py-1.5 pr-32 text-ink placeholder:text-muted {query
				? 'border-accent'
				: 'border-rule'} focus:outline-none"
		/>
		<span
			id="run-jump-caption"
			aria-live="polite"
			class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-xs {titlesFailed
				? 'text-warn'
				: 'text-muted'}"
		>
			{titlesFailed ? "Couldn't load titles" : caption}
		</span>
	</div>
	{#if parsed.kind === 'out-of-range'}
		<p class="mt-1.5 text-xs text-warn" aria-live="polite">Pick a number from 1 to {order.length}</p>
	{/if}

	{#if searching}
		<p class="py-6 text-center text-muted">Searching…</p>
	{:else if parsed.kind === 'words' && !rows.length && !loadFailed}
		<p class="py-6 text-center text-muted">No titles match “{query.trim()}”</p>
	{:else if rows.length}
		<div
			bind:this={list}
			onscroll={() => (scrollTop = list?.scrollTop ?? 0)}
			class="relative mt-1.5 overflow-y-auto overscroll-contain"
			style:height="{vp}px"
		>
			<ul aria-label="Items in this run" class="relative" style:height="{rows.length * ROW_PX}px">
				{#each drawn as row (rows[row])}
					{@const index = rows[row]}
					{@const id = order[index]}
					{@const b = details?.get(id)}
					{@const now = index === current}
					{@const title = b?.title ?? (details ? 'Unavailable' : '')}
					<li class="absolute inset-x-0 h-11" style:top="{row * ROW_PX}px">
						<a
							href={runHref(id, run)}
							data-row={row}
							tabindex={row === active ? 0 : -1}
							onclick={(e) => onRowClick(e, index)}
							onkeydown={(e) => onRowKey(e, row)}
							aria-label="{index + 1}, {title || 'loading'}{b ? `, ${formatDuration(b.duration_sec)}` : ''}{now
								? ', now playing'
								: ''}"
							class="flex h-full items-center gap-2.5 rounded-theme px-1.5 hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline focus-visible:outline-1 focus-visible:outline-accent {row ===
								active && parsed.kind === 'number'
								? 'bg-surface-2'
								: ''} {now ? 'shadow-[inset_2px_0_0_var(--accent)]' : ''}"
						>
							<span class="w-8 shrink-0 text-right text-xs tabular-nums {now ? 'text-accent' : 'text-muted'}"
								>{index + 1}</span
							>
							{#if b?.thumbnail_url}
								<img src={b.thumbnail_url} alt="" loading="lazy" class="h-8 w-14 shrink-0 rounded-theme bg-black object-cover" />
							{:else}
								<span class="h-8 w-14 shrink-0 rounded-theme {details ? 'bg-black' : 'bg-surface-2'}"></span>
							{/if}
							{#if title}
								<span class="min-w-0 flex-1 truncate {b ? 'text-ink' : 'text-muted'} {now ? 'font-medium' : ''}">
									{#if parsed.kind === 'words' && b}
										{@const [pre, hit, post] = splitMatch(b.title, parsed.text)}
										{pre}<span class="underline decoration-accent underline-offset-2">{hit}</span>{post}
									{:else}
										{title}
									{/if}
								</span>
							{:else}
								<span class="h-2.5 w-40 max-w-full flex-1 rounded-theme bg-surface-2"></span>
							{/if}
							{#if now}<span class="shrink-0 text-xs text-accent">▶ now</span>{/if}
							{#if b}
								<span class="shrink-0 text-xs text-muted max-[400px]:hidden">{formatDuration(b.duration_sec)}</span>
							{/if}
						</a>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
	<p class="mt-1.5 text-xs text-muted max-sm:hidden">↑↓ move · Enter plays · Esc closes</p>
</div>
