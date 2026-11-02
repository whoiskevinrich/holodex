<script lang="ts">
	// The File names tab's Renaming card: the mode, the settle window (Job only) and the ordered
	// template editor with its unsaved draft (handoff "Renaming card"). Folds to one line once the
	// templates are saved and valid; open while a draft exists or a template has an error.
	import { untrack } from 'svelte';
	import { api, ApiError } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import { segmentedToggleWrapperClass } from '#lib/components/sort/segmentedToggle.ts';
	import {
		MODES,
		MODE_LINES,
		loadDraft,
		sameList,
		sentence,
		storeDraft,
		type RenameMode
	} from '#lib/rename.ts';
	import TemplateList, { type TemplateItem } from './TemplateList.svelte';

	let {
		mode,
		settleDays,
		saved,
		providers = [],
		counted = null,
		onmode,
		onsettle,
		onpreview,
		ondiscard,
		onsaved
	}: {
		mode: RenameMode;
		settleDays: number;
		saved: string[];
		providers?: string[];
		// The plan's template list and its per-template counts, for the rows whose text matches.
		counted?: { templates: string[]; counts: { names: number; wont_read_back: number }[] } | null;
		onmode: (mode: RenameMode) => void;
		onsettle?: (days: number) => void;
		// Recomputes the plan from the draft; rejects with ApiError 422 (`body.rows`) on a refusal.
		onpreview: (templates: string[]) => Promise<void>;
		ondiscard?: () => void;
		onsaved: (templates: string[]) => void;
	} = $props();

	let ids = 0;
	const toItems = (list: string[]): TemplateItem[] => list.map((text) => ({ id: ++ids, text }));

	// Seeded once from a stored draft, else the saved list; a save or discard resets it explicitly.
	let items = $state<TemplateItem[]>(
		untrack(() => {
			const draft = loadDraft();
			return toItems(draft && !sameList(draft, saved) ? draft : saved);
		})
	);
	let errors = $state<string[]>([]);
	let opened = $state(false);
	let busy = $state(false);
	let message = $state('');
	let settleText = $state(untrack(() => String(settleDays)));
	let settleError = $state('');

	const list = $derived(items.map((t) => t.text));
	const dirty = $derived(!sameList(list, saved));
	const errorRows = $derived(errors.flatMap((e, i) => (e ? [i + 1] : [])));
	const open = $derived(opened || dirty || errorRows.length > 0 || saved.length === 0);
	const counts = $derived(
		list.map((t, i) => (counted && counted.templates[i] === t ? (counted.counts[i] ?? null) : null))
	);

	$effect(() => storeDraft(dirty ? list : null));

	function edited(i: number) {
		message = '';
		if (i < 0) errors = [];
		else if (errors[i]) errors[i] = '';
	}

	function refusal(e: unknown): boolean {
		const rows = e instanceof ApiError && e.status === 422 ? e.body?.rows : undefined;
		if (!Array.isArray(rows)) return false;
		errors = rows.map((r) => (typeof r === 'string' && r ? sentence(r) : ''));
		return true;
	}

	async function preview() {
		busy = true;
		message = '';
		try {
			errors = [];
			await onpreview(list);
		} catch (e) {
			if (!refusal(e)) message = toMessage(e);
		} finally {
			busy = false;
		}
	}

	// save is also the plan bar's Save templates while a preview is showing.
	export async function save() {
		if (errorRows.length > 0 || busy) return;
		busy = true;
		message = '';
		try {
			await api.putSetting('rename.templates', JSON.stringify(list));
			errors = [];
			opened = false;
			storeDraft(null);
			onsaved(list);
		} catch (e) {
			if (!refusal(e)) message = toMessage(e);
		} finally {
			busy = false;
		}
	}

	function discard() {
		items = toItems(saved);
		errors = [];
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

	const modeLabel = $derived(MODES.find((m) => m.value === mode)?.label ?? mode);
	const fixHint = $derived(
		errorRows.length === 0
			? ''
			: errorRows.length === 1
				? `Fix template ${errorRows[0]} to save.`
				: `Fix templates ${errorRows.slice(0, -1).join(', ')} and ${errorRows.at(-1)} to save.`
	);
</script>

<section class="rounded-theme border border-rule bg-surface" aria-label="Renaming">
	{#if mode !== 'off' && !open}
		<div class="flex flex-wrap items-center justify-between gap-2 px-4 py-3">
			<p class="min-w-0 text-sm text-ink">
				{modeLabel}
				<span class="text-muted"> · {saved.length} {saved.length === 1 ? 'template' : 'templates'} · first: </span>
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
				{#if opened && !dirty && errorRows.length === 0 && saved.length > 0}
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
					The first template whose every token has a value names the file. The extension never changes.
				</p>
				<TemplateList bind:items {errors} {counts} {providers} onedit={edited} />
			</div>
			{#if dirty}
				<div class="flex flex-wrap items-center justify-between gap-3 border-t border-rule px-4 py-2.5">
					<p class="text-sm text-muted">Unsaved changes. The plan shows these templates after Preview.</p>
					<div class="flex flex-wrap items-center gap-2">
						{#if fixHint}<span class="text-xs text-warn">{fixHint}</span>{/if}
						<button type="button" class="btn-quiet rounded-theme px-2 py-1.5 text-sm" onclick={discard}>Discard</button>
						<button type="button" class="btn-ghost rounded-theme px-3 py-1.5 text-sm" aria-busy={busy} onclick={preview}
							>Preview</button
						>
						<button
							type="button"
							class="btn-accent rounded-theme px-3 py-1.5 text-sm"
							disabled={errorRows.length > 0}
							aria-busy={busy}
							onclick={save}>Save templates</button
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
