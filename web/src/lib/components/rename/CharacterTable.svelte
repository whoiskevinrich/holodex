<script lang="ts">
	// The Renaming card's Characters section (spec P0-4, HOLODEX-633): one row per character a file
	// name value may not keep, with its stand-in, a live example and its own error. The nine a file
	// name can't hold have no ×. + Add character appends a draft row; Reset to defaults puts the
	// default table into the draft.
	import { tick } from 'svelte';
	import { DEFAULT_CHARACTERS, FORBIDDEN_CHARACTERS, characterExample, characterOrder, draftTable, sameTable } from '#lib/rename.ts';

	export interface CharacterRow {
		id: number;
		char: string;
		standIn: string;
		// Added in this draft: its character is an input, so a refused one can be fixed.
		added?: boolean;
	}

	// A row's error, and which of its inputs it belongs to.
	export interface CharacterError {
		message: string;
		on: 'character' | 'stand-in';
	}

	let {
		rows = $bindable(),
		errors = {},
		onedit
	}: {
		rows: CharacterRow[];
		// Keyed by row id, from the last Preview or Save.
		errors?: Record<number, CharacterError>;
		// A row's id when one row changed, null when the whole table did.
		onedit?: (id: number | null) => void;
	} = $props();

	let root: HTMLElement;
	let nextId = 0;
	let focused = $state<number | null>(null);

	const isDefault = $derived.by(() => {
		const table = draftTable(rows);
		return !!table && sameTable(table, DEFAULT_CHARACTERS);
	});

	function edit(i: number, patch: Partial<CharacterRow>) {
		rows = rows.map((r, k) => (k === i ? { ...r, ...patch } : r));
		onedit?.(rows[i].id);
	}

	function remove(i: number) {
		rows = rows.filter((_, k) => k !== i);
		onedit?.(null);
	}

	async function add() {
		const id = Date.now() + nextId++;
		rows = [...rows, { id, char: '', standIn: '', added: true }];
		onedit?.(null);
		await tick();
		root?.querySelector<HTMLInputElement>(`[data-row="${id}"] [data-act="char"]`)?.focus();
	}

	function reset() {
		if (isDefault) return;
		const fresh = Date.now() + nextId;
		nextId += Object.keys(DEFAULT_CHARACTERS).length;
		rows = characterOrder(DEFAULT_CHARACTERS).map((char, k) => ({ id: fresh + k, char, standIn: DEFAULT_CHARACTERS[char] }));
		onedit?.(null);
	}

	// While not focused, a stand-in's spaces show as ␣ so ` and ` is visible.
	const shown = (r: CharacterRow) => (focused === r.id ? r.standIn : r.standIn.replaceAll(' ', '␣'));
	const label = (r: CharacterRow) => r.char || 'new character';
	const withdrawn = 'text-rule hover:no-underline cursor-default';
</script>

<div bind:this={root} class="space-y-2">
	<div class="grid grid-cols-[3rem_6rem_minmax(0,1fr)_auto] items-center gap-x-3 gap-y-1.5 md:grid-cols-[4rem_6rem_minmax(0,1fr)_auto]">
		<span class="text-xs text-muted">Character</span>
		<span class="text-xs text-muted">Stand-in</span>
		<span class="text-xs text-muted">Example</span>
		<span></span>
		{#each rows as r, i (r.id)}
			{@const err = errors[r.id]}
			{@const fixed = FORBIDDEN_CHARACTERS.includes(r.char) && !r.added}
			{@const ex = r.char ? characterExample(r.char, r.standIn) : null}
			<div data-row={r.id} class="contents">
				{#if r.added}
					<input
						data-act="char"
						type="text"
						maxlength="2"
						spellcheck="false"
						autocomplete="off"
						aria-label="Character"
						aria-invalid={err?.on === 'character' ? 'true' : undefined}
						aria-describedby={err ? `character-error-${r.id}` : undefined}
						value={r.char}
						oninput={(e) => edit(i, { char: e.currentTarget.value })}
						class="w-10 rounded-theme border bg-bg px-2 py-1 text-center font-mono text-sm text-ink {err?.on === 'character'
							? 'border-warn'
							: 'border-rule'}"
					/>
				{:else}
					<span class="pl-1.5 font-mono text-sm text-ink">{r.char}</span>
				{/if}
				<input
					data-act="stand-in"
					type="text"
					maxlength="8"
					spellcheck="false"
					autocomplete="off"
					placeholder="removed"
					aria-label="Stand-in for {label(r)}"
					aria-invalid={err?.on === 'stand-in' ? 'true' : undefined}
					aria-describedby={err ? `character-error-${r.id}` : undefined}
					value={shown(r)}
					onfocus={() => (focused = r.id)}
					onblur={() => (focused = null)}
					oninput={(e) => edit(i, { standIn: e.currentTarget.value })}
					class="w-24 rounded-theme border bg-bg px-2 py-1 font-mono text-sm text-ink placeholder:font-ui placeholder:text-muted {err?.on ===
					'stand-in'
						? 'border-warn'
						: 'border-rule'}"
				/>
				{#if err}
					<p id="character-error-{r.id}" class="min-w-0 text-xs text-warn">{err.message}</p>
				{:else if ex}
					<p class="min-w-0 text-xs text-muted wrap-anywhere">{ex.sample} → {ex.result}</p>
				{:else}
					<span></span>
				{/if}
				{#if fixed}
					<span></span>
				{:else}
					<button type="button" class="btn-row btn-quiet rounded-theme px-1.5" aria-label="Remove {label(r)}" onclick={() => remove(i)}
						>×</button
					>
				{/if}
			</div>
		{/each}
	</div>
	<p class="text-xs text-muted">
		The nine characters a file name can't hold always have a row and no ×. Their stand-in can change, but the character is never
		kept.
	</p>
	<div class="flex flex-wrap items-center justify-between gap-2">
		<button type="button" class="btn-quiet rounded-theme px-3 py-1.5 text-sm" onclick={add}>+ Add character</button>
		<button
			type="button"
			class="btn-quiet rounded-theme px-2 py-1.5 text-sm {isDefault ? withdrawn : ''}"
			aria-disabled={isDefault}
			onclick={reset}>Reset to defaults</button
		>
	</div>
</div>
