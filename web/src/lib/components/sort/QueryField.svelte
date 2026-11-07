<script lang="ts">
	// A page-owned F87 query field (F89 handoff "Query field"): the nav box's look, the same
	// suggestion panel, error echo and Did you mean rows, over its own QueryAssist, so typing
	// here never touches the nav box. Enter applies the text; the parent gets the canonical
	// query, and keeps its last valid one while the text is refused.
	import { activity } from '#lib/activity.svelte.ts';
	import QuerySuggestPanel from '#lib/components/entity/QuerySuggestPanel.svelte';
	import { QueryAssist } from '#lib/queryAssist.svelte.ts';

	let {
		text = $bindable(''),
		invalid = $bindable(false),
		onapply
	}: {
		text: string;
		/** The server refused the text as it stands. */
		invalid?: boolean;
		/** The canonical query the text parsed to, '' for none. */
		onapply: (canonical: string) => void;
	} = $props();

	const assist = new QueryAssist();
	const LIST_ID = 'definition-suggestions';
	const ERROR_ID = 'definition-query-error';
	let input = $state<HTMLInputElement>();
	let open = $state(false);
	const error = $derived(assist.errorFor(text));
	const showPanel = $derived(open && (error !== null || assist.suggestions.length > 0));

	$effect(() => {
		invalid = error !== null;
	});
	$effect(() => {
		input?.focus();
	});

	function update() {
		if (input) assist.update(text, input.selectionStart ?? text.length, activity.effectiveOwner);
	}

	function pick(i: number) {
		const next = assist.accept(text, i);
		if (!next) return false;
		text = next.text;
		requestAnimationFrame(() => {
			input?.setSelectionRange(next.caret, next.caret);
			update();
		});
		return true;
	}

	/** Applies the text now; false when the server refuses it. Save calls this first. */
	export async function apply(): Promise<boolean> {
		if (text.trim() === '') {
			assist.reset();
			onapply('');
			return true;
		}
		const r = await assist.parseNow(text);
		if (r.ok) {
			open = false;
			onapply(r.canonical);
		} else {
			open = true;
		}
		return r.ok;
	}

	function onkeydown(e: KeyboardEvent) {
		if (showPanel && assist.suggestions.length) {
			if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
				e.preventDefault();
				assist.move(e.key === 'ArrowDown' ? 1 : -1);
				return;
			}
			if (e.key === 'Tab' && !e.shiftKey && pick(assist.highlight)) {
				e.preventDefault();
				return;
			}
		}
		if (e.key === 'Escape' && open) {
			e.stopPropagation();
			open = false;
		} else if (e.key === 'Enter') {
			e.preventDefault();
			void apply();
		}
	}
</script>

<div
	class="relative"
	onfocusout={(e) => {
		if (!e.currentTarget.contains(e.relatedTarget as Node | null)) open = false;
	}}
>
	<div class="relative">
		<span
			aria-hidden="true"
			class="pointer-events-none absolute left-2 top-1/2 -translate-y-1/2 rounded-theme border border-accent px-1 text-xs leading-4 text-accent"
			>query</span
		>
		<input
			bind:this={input}
			bind:value={text}
			oninput={() => {
				open = true;
				update();
			}}
			onfocus={() => (open = true)}
			onkeyup={(e) => (e.key === 'ArrowLeft' || e.key === 'ArrowRight' || e.key === 'Home' || e.key === 'End') && update()}
			{onkeydown}
			aria-label="Definition query"
			placeholder="tag:noir -studio:…"
			role="combobox"
			autocomplete="off"
			spellcheck="false"
			aria-autocomplete="list"
			aria-expanded={showPanel}
			aria-controls={LIST_ID}
			aria-activedescendant={assist.highlight >= 0 ? `${LIST_ID}-${assist.highlight}` : undefined}
			aria-describedby={error ? ERROR_ID : undefined}
			class="w-full rounded-theme border bg-bg py-1.5 pl-14 pr-7 font-mono text-[13px] text-ink outline-none placeholder:text-muted {error
				? 'border-warn'
				: 'border-rule focus:border-accent'}"
		/>
		{#if text}
			<button
				type="button"
				aria-label="Clear query"
				onclick={() => {
					text = '';
					void apply();
					input?.focus();
				}}
				class="absolute right-1.5 top-1/2 -translate-y-1/2 rounded-theme px-1 text-muted hover:text-ink"
			>
				×
			</button>
		{/if}
	</div>
	{#if showPanel}
		<QuerySuggestPanel
			{text}
			{error}
			suggestions={assist.suggestions}
			highlight={assist.highlight}
			heading={assist.heading}
			listId={LIST_ID}
			errorId={ERROR_ID}
			isOwner={activity.effectiveOwner}
			enterHint="Enter applies the query"
			onpick={pick}
			onhover={(i) => (assist.highlight = i)}
		/>
	{/if}
</div>
