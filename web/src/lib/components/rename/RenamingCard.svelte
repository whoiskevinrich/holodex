<script lang="ts">
	// The File names tab's Renaming card: the mode, the settle window (Job only), and the ordered
	// template editor and character table sharing one unsaved draft (handoff "Renaming card",
	// "Characters"). Folds to one line once both are saved and valid; open while a draft exists or a
	// row has an error.
	import { untrack } from 'svelte';
	import { api, ApiError } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import { segmentedToggleWrapperClass } from '#lib/components/sort/segmentedToggle.ts';
	import {
		MODES,
		MODE_LINES,
		characterOrder,
		draftTable,
		loadCharacterDraft,
		loadDraft,
		sameList,
		sameTable,
		sentence,
		storeCharacterDraft,
		storeDraft,
		type CharacterTable as Table,
		type RenameMode,
		type RenamePreview
	} from '#lib/rename.ts';
	import TemplateList, { type TemplateItem } from './TemplateList.svelte';
	import CharacterTable, { type CharacterError, type CharacterRow } from './CharacterTable.svelte';

	let {
		mode,
		settleDays,
		saved,
		savedCharacters,
		providers = [],
		counted = null,
		saveLabel = $bindable('Save templates'),
		onmode,
		onsettle,
		onpreview,
		ondiscard,
		onsaved
	}: {
		mode: RenameMode;
		settleDays: number;
		saved: string[];
		savedCharacters: Table;
		providers?: string[];
		// The plan's template list and its per-template counts, for the rows whose text matches.
		counted?: { templates: string[]; counts: { names: number; wont_read_back: number }[] } | null;
		// Out: what Save writes (Save templates, Save characters or Save changes), for the plan bar.
		saveLabel?: string;
		onmode: (mode: RenameMode) => void;
		onsettle?: (days: number) => void;
		// Recomputes the plan from the draft (characters only when they changed); rejects with
		// ApiError 422 (`body.rows` or `body.characters`) on a refusal.
		onpreview: (draft: RenamePreview) => Promise<void>;
		ondiscard?: () => void;
		// What a save wrote: the templates, the characters or both.
		onsaved: (wrote: { templates?: string[]; characters?: Table }) => void;
	} = $props();

	let ids = 0;
	const toItems = (list: string[]): TemplateItem[] => list.map((text) => ({ id: ++ids, text }));
	const toRows = (table: Table): CharacterRow[] =>
		characterOrder(table)
			.filter((char) => Object.hasOwn(table, char))
			.map((char) => ({ id: ++ids, char, standIn: table[char] }));

	// Seeded once from a stored draft, else the saved settings; a save or discard resets it explicitly.
	let items = $state<TemplateItem[]>(
		untrack(() => {
			const draft = loadDraft();
			return toItems(draft && !sameList(draft, saved) ? draft : saved);
		})
	);
	let charRows = $state<CharacterRow[]>(
		untrack(() => {
			const draft = loadCharacterDraft();
			const table = draft && draftTable(draft);
			return draft && !(table && sameTable(table, savedCharacters))
				? draft.map((r) => ({ ...r, id: ++ids }))
				: toRows(savedCharacters);
		})
	);
	let errors = $state<string[]>([]);
	let charErrors = $state<Record<number, CharacterError>>({});
	let opened = $state(false);
	let busy = $state(false);
	let message = $state('');
	let settleText = $state(untrack(() => String(settleDays)));
	let settleError = $state('');

	const list = $derived(items.map((t) => t.text));
	const table = $derived(draftTable(charRows));
	const templatesDirty = $derived(!sameList(list, saved));
	const charsDirty = $derived(!table || !sameTable(table, savedCharacters));
	const dirty = $derived(templatesDirty || charsDirty);
	const errorRows = $derived(errors.flatMap((e, i) => (e ? [i + 1] : [])));
	const charErrorRows = $derived(charRows.filter((r) => charErrors[r.id]));
	const blocked = $derived(errorRows.length > 0 || charErrorRows.length > 0);
	const open = $derived(opened || dirty || blocked || saved.length === 0);
	const counts = $derived(
		list.map((t, i) => (counted && counted.templates[i] === t ? (counted.counts[i] ?? null) : null))
	);
	const label = $derived(
		templatesDirty && charsDirty ? 'Save changes' : charsDirty ? 'Save characters' : 'Save templates'
	);
	const noun = $derived(templatesDirty && charsDirty ? 'changes' : charsDirty ? 'characters' : 'templates');

	$effect(() => {
		saveLabel = label;
	});
	$effect(() => storeDraft(templatesDirty ? list : null));
	$effect(() =>
		storeCharacterDraft(charsDirty ? charRows.map(({ char, standIn, added }) => ({ char, standIn, added })) : null)
	);

	function edited(i: number) {
		message = '';
		if (i < 0) errors = [];
		else if (errors[i]) errors[i] = '';
	}

	function charEdited(id: number | null) {
		message = '';
		if (id === null) charErrors = {};
		else if (charErrors[id]) charErrors = Object.fromEntries(Object.entries(charErrors).filter(([k]) => Number(k) !== id));
	}

	// checkCharacters refuses what can't be sent: a draft row with no character (or more than one),
	// and a character already in the table (object keys would collide).
	function checkCharacters(): boolean {
		const found: Record<number, CharacterError> = {};
		const seen = new Set<string>();
		for (const r of charRows) {
			if ([...r.char].length !== 1) found[r.id] = { message: 'Add one character.', on: 'character' };
			else if (seen.has(r.char)) found[r.id] = { message: `${r.char} is already in the table.`, on: 'character' };
			seen.add(r.char);
		}
		charErrors = found;
		return Object.keys(found).length === 0;
	}

	function refusal(e: unknown): boolean {
		if (!(e instanceof ApiError && e.status === 422)) return false;
		const rows = e.body?.rows;
		const chars = e.body?.characters;
		let handled = false;
		if (Array.isArray(rows)) {
			errors = rows.map((r) => (typeof r === 'string' && r ? sentence(r) : ''));
			handled = true;
		}
		if (chars && typeof chars === 'object') {
			const found: Record<number, CharacterError> = {};
			const unplaced: string[] = [];
			for (const [char, msg] of Object.entries(chars as Record<string, unknown>)) {
				if (typeof msg !== 'string') continue;
				const row = charRows.find((r) => r.char === char);
				// A draft row's character can be refused too; everything else is about the stand-in.
				if (row) found[row.id] = { message: msg, on: row.added && !msg.startsWith('A stand-in') ? 'character' : 'stand-in' };
				else unplaced.push(msg);
			}
			charErrors = found;
			if (unplaced.length) message = unplaced.join(' ');
			handled = true;
		}
		return handled;
	}

	async function preview() {
		if (!checkCharacters()) return;
		busy = true;
		message = '';
		try {
			errors = [];
			await onpreview(charsDirty && table ? { templates: list, characters: table } : { templates: list });
		} catch (e) {
			if (!refusal(e)) message = toMessage(e);
		} finally {
			busy = false;
		}
	}

	// save is also the plan bar's Save button while a preview is showing. It writes whichever of the
	// templates and characters changed; a refusal of the second keeps the first saved.
	export async function save() {
		if (blocked || busy || !checkCharacters()) return;
		busy = true;
		message = '';
		const tpl = list;
		const chars = charsDirty ? table : null;
		const wrote: { templates?: string[]; characters?: Table } = {};
		try {
			if (templatesDirty || !chars) {
				await api.putSetting('rename.templates', JSON.stringify(tpl));
				wrote.templates = tpl;
			}
			if (chars) {
				await api.putSetting('rename.characters', JSON.stringify(chars));
				wrote.characters = chars;
				charRows = toRows(chars);
			}
			errors = [];
			charErrors = {};
			opened = false;
			storeDraft(null);
			storeCharacterDraft(null);
		} catch (e) {
			if (!refusal(e)) message = toMessage(e);
		} finally {
			busy = false;
			if (wrote.templates || wrote.characters) onsaved(wrote);
		}
	}

	function discard() {
		items = toItems(saved);
		charRows = toRows(savedCharacters);
		errors = [];
		charErrors = {};
		message = '';
		ondiscard?.();
	}

	async function chooseMode(m: RenameMode) {
		if (m === mode) return;
		message = '';
		try {
			await api.putSetting('rename.mode', m);
			onmode(m);
		} catch (e) {
			message = toMessage(e);
		}
	}

	// The mode radiogroup's roving tabindex: arrows move focus, Space or Enter chooses.
	let focusIndex = $state(-1);
	const tabStop = $derived(focusIndex >= 0 ? focusIndex : MODES.findIndex((m) => m.value === mode));
	let group = $state<HTMLElement>();

	function modeKey(e: KeyboardEvent, i: number) {
		const step = e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
		if (step) {
			e.preventDefault();
			focusIndex = (i + step + MODES.length) % MODES.length;
			group?.querySelectorAll<HTMLElement>('[role="radio"]')[focusIndex]?.focus();
		} else if (e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			chooseMode(MODES[i].value);
		}
	}

	async function commitSettle() {
		const n = Number(settleText.trim());
		if (!Number.isInteger(n) || n < 1 || n > 365) {
			settleError = 'Enter 1 to 365 days.';
			return;
		}
		settleError = '';
		if (n === settleDays) return;
		try {
			await api.putSetting('rename.settle_days', String(n));
			onsettle?.(n);
		} catch (e) {
			settleError = toMessage(e);
		}
	}

	const charCount = $derived(Object.keys(savedCharacters).length);
	const modeLabel = $derived(MODES.find((m) => m.value === mode)?.label ?? mode);
	const andList = (xs: (string | number)[]) => (xs.length === 1 ? `${xs[0]}` : `${xs.slice(0, -1).join(', ')} and ${xs.at(-1)}`);
	const fixHint = $derived(
		[
			errorRows.length === 0
				? ''
				: `Fix ${errorRows.length === 1 ? 'template' : 'templates'} ${andList(errorRows)} to save.`,
			charErrorRows.length === 0
				? ''
				: `Fix the ${andList(charErrorRows.map((r) => r.char || 'new'))} ${charErrorRows.length === 1 ? 'row' : 'rows'} to save.`
		]
			.filter(Boolean)
			.join(' ')
	);
</script>

<section class="rounded-theme border border-rule bg-surface" aria-label="Renaming">
	{#if mode !== 'off' && !open}
		<div class="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
			<p class="min-w-0 text-sm text-ink">
				{modeLabel}
				<span class="text-muted">
					· {saved.length} {saved.length === 1 ? 'template' : 'templates'} · {charCount} {charCount === 1 ? 'character' : 'characters'} · first:
				</span>
				<span class="font-mono wrap-anywhere">{saved[0]}</span>
			</p>
			<button type="button" class="btn-quiet rounded-theme px-2 py-1 text-sm" aria-expanded="false" onclick={() => (opened = true)}
				>Edit templates ▾</button
			>
		</div>
	{:else}
		<div class="space-y-2 px-4 py-3">
			<div class="flex items-center justify-between gap-2">
				<h2 class="text-xs uppercase tracking-wide text-muted">Renaming</h2>
				{#if opened && !dirty && !blocked && saved.length > 0}
					<button type="button" class="btn-quiet rounded-theme px-2 py-1 text-sm" aria-expanded="true" onclick={() => (opened = false)}
						>Edit templates ▴</button
					>
				{/if}
			</div>
			<div class="flex flex-wrap items-center gap-x-5 gap-y-2">
				<div bind:this={group} role="radiogroup" aria-label="Renaming mode" class="{segmentedToggleWrapperClass} divide-x divide-rule">
					{#each MODES as m, i (m.value)}
						{@const active = m.value === mode}
						<button
							type="button"
							role="radio"
							aria-checked={active}
							tabindex={i === tabStop ? 0 : -1}
							class="px-3 py-1 {active ? 'bg-surface-2 text-ink' : 'text-muted hover:text-ink'}"
							onclick={() => chooseMode(m.value)}
							onkeydown={(e) => modeKey(e, i)}
							onblur={() => (focusIndex = -1)}>{m.label}</button
						>
					{/each}
				</div>
				{#if mode === 'job'}
					<label class="flex items-center gap-2 text-sm text-muted">
						Settle window
						<input
							type="text"
							inputmode="numeric"
							maxlength="3"
							bind:value={settleText}
							onblur={commitSettle}
							onkeydown={(e) => e.key === 'Enter' && commitSettle()}
							aria-invalid={settleError ? 'true' : undefined}
							class="w-14 rounded-theme border bg-bg px-2 py-1 text-sm text-ink {settleError ? 'border-warn' : 'border-rule'}"
						/>
						days
					</label>
				{/if}
			</div>
			{#if settleError}
				<p class="text-xs text-warn" role="alert">{settleError}</p>
			{/if}
			<p class="text-xs text-muted">{MODE_LINES[mode]}</p>
		</div>

		{#if mode !== 'off'}
			<div class="space-y-2 border-t border-rule px-4 py-3">
				<h3 class="text-xs uppercase tracking-wide text-muted">Templates</h3>
				<p class="text-xs text-muted">
					The first template whose every required token has a value names the file. The extension never changes.
				</p>
				<p class="text-xs text-muted">
					Add ? to make a token optional: {'[{studio?}]'} drops, brackets and all, when a video has no studio. {'{part}'},
					{'{edition}'} and {'{film}'} write their own braces: {'{part}'} becomes {'{part-2}'}, and {'{film}'} names the
					linked film, as {'{film-Film Name}'}.
				</p>
				<TemplateList bind:items {errors} {counts} {providers} onedit={edited} />
			</div>
			<div class="space-y-2 border-t border-rule px-4 py-3">
				<h3 class="text-xs uppercase tracking-wide text-muted">Characters</h3>
				<p class="text-xs text-muted">
					Each value swaps these characters before it becomes a file name. Leave the stand-in empty to remove the character.
				</p>
				<CharacterTable bind:rows={charRows} errors={charErrors} onedit={charEdited} />
			</div>
			{#if dirty}
				<div class="flex flex-wrap items-center justify-between gap-3 border-t border-rule px-4 py-2.5">
					<p class="text-sm text-muted">Unsaved changes. The plan shows these {noun} after Preview.</p>
					<div class="flex flex-wrap items-center gap-2">
						{#if fixHint}<span class="text-xs text-warn">{fixHint}</span>{/if}
						<button type="button" class="btn-quiet rounded-theme px-2 py-1.5 text-sm" onclick={discard}>Discard</button>
						<button type="button" class="btn-ghost rounded-theme px-3 py-1.5 text-sm" aria-busy={busy} onclick={preview}
							>Preview</button
						>
						<button
							type="button"
							class="btn-accent rounded-theme px-3 py-1.5 text-sm"
							disabled={blocked}
							aria-busy={busy}
							onclick={save}>{label}</button
						>
					</div>
				</div>
			{/if}
		{/if}
		{#if message}
			<p class="border-t border-rule px-4 py-2 text-xs text-warn" role="alert">{message}</p>
		{/if}
	{/if}
</section>
