<script lang="ts">
	// The owner's add-tag input with existing-tag suggestions (HOLODEX-519,
	// docs/design/media-tag-input-handoff.md). Enter takes the typed text (or the highlighted
	// row); Tab takes a suggestion (the highlighted one, else the first). A text-input combobox,
	// so it uses aria-activedescendant — focus must stay in the input while the owner types —
	// not the roving tabindex the picker dialogs use (D3). It owns the input and its list only;
	// the caller owns the add itself, its errors, the near-miss nudge and the Add/Done buttons.
	import { onMount } from 'svelte';
	import { api } from '$lib/api';
	import { suggestTags } from '$lib/tagInput';
	import { videoCount } from '$lib/format';
	import type { Tag } from '$lib/types';

	let {
		value = $bindable(''),
		exclude,
		onadd,
		onclose
	}: {
		value?: string;
		// Ids of the tags already on the video — never suggested.
		exclude: ReadonlySet<number>;
		// Adds the name; resolves to the attached tag, or undefined when the add failed.
		onadd: (name: string) => Promise<Tag | undefined>;
		// Esc with no list open.
		onclose: () => void;
	} = $props();

	const uid = $props.id();
	const listId = `tag-add-list-${uid}`;
	const hintId = `tag-add-hint-${uid}`;

	let input = $state<HTMLInputElement | null>(null);
	let all = $state<Tag[]>([]);
	let focused = $state(false);
	let dismissed = $state(false);
	let active = $state(-1);
	let announcement = $state('');

	// An aid, not the control: a failed fetch just means no suggestions.
	onMount(() => {
		api
			.listTags()
			.then((r) => (all = r.items))
			.catch(() => {});
	});

	export function focus() {
		input?.focus();
	}

	// The caller's Add button: same as Enter with nothing highlighted.
	export function addTyped() {
		if (typed) void add(typed);
	}

	const matches = $derived(suggestTags(all, value, exclude));
	const typed = $derived(value.trim());
	const exact = $derived(all.some((t) => t.name.toLowerCase() === typed.toLowerCase()));
	// Rows: the matches, then "Add “x” as a new tag" unless the text already names a tag.
	const rowCount = $derived(matches.length + (typed && !exact ? 1 : 0));
	const open = $derived(focused && !dismissed && typed !== '' && all.length > 0 && rowCount > 0);

	async function add(name: string) {
		active = -1;
		const tag = await onadd(name);
		if (!tag) return;
		if (!all.some((t) => t.id === tag.id)) all = [...all, tag];
		announcement = `Added ${tag.name}`;
	}

	// The row's name: a match's tag, or the typed text for the "add as new" row.
	function rowName(i: number) {
		return i < matches.length ? matches[i].name : typed;
	}

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
			// An arrow reopens a list Esc dismissed.
			dismissed = false;
			if (!open) return;
			e.preventDefault();
			const step = e.key === 'ArrowDown' ? 1 : -1;
			active = active === -1 ? (step === 1 ? 0 : rowCount - 1) : (active + step + rowCount) % rowCount;
		} else if (e.key === 'Enter') {
			e.preventDefault();
			if (open && active >= 0) void add(rowName(active));
			else if (typed) void add(typed);
		} else if (e.key === 'Tab' && !e.shiftKey && open && matches.length > 0) {
			e.preventDefault();
			void add(rowName(active >= 0 ? active : 0));
		} else if (e.key === 'Escape') {
			e.preventDefault();
			if (open) dismissed = true;
			else onclose();
		}
	}

	function oninput() {
		active = -1;
		dismissed = false;
	}
</script>

<span class="relative inline-flex">
	<input
		bind:this={input}
		bind:value
		{onkeydown}
		{oninput}
		onfocus={() => (focused = true)}
		onblur={() => (focused = false)}
		type="text"
		placeholder="Add a tag"
		aria-label="Add a tag"
		role="combobox"
		aria-autocomplete="list"
		aria-expanded={open}
		aria-controls={listId}
		aria-activedescendant={open && active >= 0 ? `${listId}-${active}` : undefined}
		aria-describedby={hintId}
		autocomplete="off"
		class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
	/>
	<span id={hintId} class="sr-only">Enter adds what you typed. Tab adds the suggestion.</span>
	<span class="sr-only" aria-live="polite">{announcement}</span>
	{#if open}
		<ul
			id={listId}
			role="listbox"
			aria-label="Existing tags"
			class="absolute left-0 top-full z-20 mt-1 w-full min-w-[16rem] max-w-[calc(100vw-2rem)] rounded-theme border border-rule bg-surface py-1 text-sm"
		>
			{#each matches as t, i (t.id)}
				{@const at = t.name.toLowerCase().indexOf(typed.toLowerCase())}
				<!-- svelte-ignore a11y_click_events_have_key_events (the keyboard lives on the combobox input, D3) -->
				<li
					id="{listId}-{i}"
					role="option"
					aria-selected={i === active}
					aria-label="{t.name}, {videoCount(t.video_count ?? 0)}"
					onmousedown={(e) => e.preventDefault()}
					onmouseenter={() => (active = i)}
					onclick={() => add(t.name)}
					class="flex cursor-pointer items-center gap-3 border-l-2 px-3 py-1.5 {i === active
						? 'border-accent bg-surface-2'
						: 'border-transparent'}"
				>
					<span class="min-w-0 flex-1 truncate text-ink"
						>{t.name.slice(0, at)}<span class="font-semibold {i === active ? 'text-accent' : ''}"
							>{t.name.slice(at, at + typed.length)}</span
						>{t.name.slice(at + typed.length)}</span
					>
					<span class="text-xs tabular-nums text-muted">{t.video_count ?? 0}</span>
				</li>
			{/each}
			{#if rowCount > matches.length}
				{@const i = matches.length}
				<!-- svelte-ignore a11y_click_events_have_key_events (the keyboard lives on the combobox input, D3) -->
				<li
					id="{listId}-{i}"
					role="option"
					aria-selected={i === active}
					onmousedown={(e) => e.preventDefault()}
					onmouseenter={() => (active = i)}
					onclick={() => add(typed)}
					class="cursor-pointer truncate border-l-2 px-3 py-1.5 text-muted {matches.length
						? 'border-t border-t-rule'
						: ''} {i === active ? 'border-l-accent bg-surface-2' : 'border-l-transparent'}"
				>
					Add “{typed}” as a new tag
				</li>
			{/if}
		</ul>
	{/if}
</span>
