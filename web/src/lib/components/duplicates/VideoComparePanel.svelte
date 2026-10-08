<script lang="ts">
	// The compare panel a video pair expands into (F76, HOLODEX-521 — option B of the
	// signed-off handoff): ONE fact table whose two value columns are the two files, so
	// every difference sits on one row. A row renders when either side has a value; a side
	// without one leaves its cell blank (absent-is-absent per cell, never "—"), so the
	// columns never shift. Facts use the media page File section's labels and formats.
	//
	// The panel owns its fetch and the label editors' UI state only. Every verdict —
	// Keep this one, Keep both, saving labels — is the row's: it renders the snippets it is
	// handed and calls `label`, so a failed fetch here can't disable an action.
	// Tokens only; QA Cinémathèque.
	import { tick, type Snippet } from 'svelte';
	import { api } from '$lib/api';
	import { formatDuration, resolutionBucket, toMessage } from '$lib/format';
	import type { VideoCompare, VideoCompareSide, VideoDuplicatePair } from '$lib/types';
	import { factRows, labelError, type LabelField } from './videoPairs';

	let {
		pair,
		panelId,
		busy,
		verdicts,
		keepThis,
		label
	}: {
		pair: VideoDuplicatePair;
		panelId: string;
		busy: boolean;
		verdicts: Snippet;
		keepThis: Snippet<[VideoCompareSide, VideoCompareSide]>;
		label: (field: LabelField, a: string, b: string) => Promise<void>;
	} = $props();

	let data = $state<VideoCompare | null>(null);
	let brokenPosters = $state(new Set<number>());
	let loadError = $state('');

	async function load() {
		loadError = '';
		try {
			data = await api.videoDuplicateCompare(pair.a.id, pair.b.id);
		} catch (e) {
			loadError = toMessage(e);
		}
	}
	$effect(() => {
		load();
	});

	const rows = $derived(data ? factRows(data.a, data.b) : []);

	// Label editor: which field is being edited and the two working values.
	let editing = $state<LabelField | null>(null);
	let va = $state('');
	let vb = $state('');
	let attempted = $state(false);
	let editorReturn: HTMLElement | null = null;
	let firstInput = $state<HTMLInputElement | null>(null);

	const editError = $derived(editing && attempted ? labelError(editing, va, vb) : '');

	function startEdit(field: LabelField, e: MouseEvent) {
		if (!data) return;
		editorReturn = e.currentTarget as HTMLElement;
		editing = field;
		attempted = false;
		va = (field === 'edition' ? data.a.edition : data.a.part) ?? '';
		vb = (field === 'edition' ? data.b.edition : data.b.part) ?? '';
		tick().then(() => firstInput?.focus());
	}

	function cancelEdit() {
		editing = null;
		editorReturn?.focus();
	}

	async function save() {
		if (!editing) return;
		attempted = true;
		if (labelError(editing, va, vb)) return;
		try {
			await label(editing, va, vb);
		} catch {
			// The row shows the error; the editor stays open with the owner's values.
		}
	}

	function onInputKeydown(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			save();
		} else if (e.key === 'Escape') {
			// Stop here so the row's window handler doesn't also collapse the panel.
			e.stopPropagation();
			cancelEdit();
		}
	}

	const inputClass = (field: LabelField) =>
		`${field === 'edition' ? 'w-40' : 'w-28'} max-w-full rounded-full border border-accent bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent`;

	// While a label editor is open its field's row is the input row, inserted at the row's
	// normal place (before "Your work") when neither file had a value yet.
	const displayRows = $derived.by(() => {
		if (!editing || rows.some((r) => r.key === editing)) return rows;
		const row = { key: editing, label: editing === 'edition' ? 'Edition' : 'Part', a: '', b: '' };
		const at = rows.findIndex((r) => r.key === 'work');
		return at === -1 ? [...rows, row] : [...rows.slice(0, at), row, ...rows.slice(at)];
	});
</script>

{#snippet poster(s: VideoCompareSide)}
	<div class="video-frame">
		<!-- A file with no cover yet keeps the themed empty frame, never a broken-image glyph. -->
		{#if !brokenPosters.has(s.id)}
			<img
				src={api.thumbnailURL(s.id)}
				alt={`${resolutionBucket(s.width)} copy`}
				onerror={() => (brokenPosters = new Set(brokenPosters).add(s.id))}
				class="h-full w-full object-cover"
			/>
		{/if}
		{#if s.duration_sec}
			<span class="absolute bottom-1.5 right-1.5 z-[2] rounded-theme bg-black/70 px-1.5 py-0.5 text-xs tabular-nums text-ink">
				{formatDuration(s.duration_sec)}
			</span>
		{/if}
		{#if s.width > 0}
			<span
				class="absolute left-1.5 top-1.5 z-[2] rounded-theme bg-accent px-1.5 py-0.5 text-[10px] font-semibold text-accent-ink shadow-xs ring-1 ring-black/20"
			>
				{resolutionBucket(s.width)}
			</span>
		{/if}
	</div>
{/snippet}

{#snippet labelInput(field: LabelField, side: 'a' | 'b')}
	{#if side === 'a'}
		<input
			bind:this={firstInput}
			bind:value={va}
			onkeydown={onInputKeydown}
			inputmode={field === 'part' ? 'numeric' : undefined}
			placeholder={field === 'edition' ? 'Edition' : 'Part number'}
			aria-label={`${field === 'edition' ? 'Edition' : 'Part number'} for ${data?.a.file_name}`}
			class={inputClass(field)}
		/>
	{:else}
		<input
			bind:value={vb}
			onkeydown={onInputKeydown}
			inputmode={field === 'part' ? 'numeric' : undefined}
			placeholder={field === 'edition' ? 'Edition' : 'Part number'}
			aria-label={`${field === 'edition' ? 'Edition' : 'Part number'} for ${data?.b.file_name}`}
			class={inputClass(field)}
		/>
	{/if}
{/snippet}

<div id={panelId} class="border-t border-rule bg-surface-2 px-3 py-3" role="group" aria-label={`Compare the two files of ${pair.title}`}>
	{#if loadError}
		<p class="py-4 text-xs text-warn" role="alert">
			Couldn't load these files.
			<button onclick={load} class="btn-row btn-quiet ml-2">Retry</button>
		</p>
	{:else if !data}
		<p class="py-6 text-center text-sm text-muted">Loading…</p>
	{:else}
		{@const d = data}
		<table class="w-full table-fixed border-collapse text-xs">
			<caption class="sr-only">Compare the two files of {pair.title}</caption>
			<colgroup>
				<col class="w-[76px] sm:w-26" />
				<col />
				<col />
			</colgroup>
			<thead>
				<tr>
					<td></td>
					<th scope="col" class="px-1 pb-2 align-top font-normal sm:px-2">{@render poster(d.a)}</th>
					<th scope="col" class="px-1 pb-2 align-top font-normal sm:px-2">{@render poster(d.b)}</th>
				</tr>
			</thead>
			<tbody>
				{#each displayRows as r (r.key)}
					{#if editing && r.key === editing}
						<tr class="border-t border-rule">
							<th scope="row" class="px-1 py-1.5 text-left align-middle font-normal text-muted sm:px-2">{r.label}</th>
							<td class="px-1 py-1.5 sm:px-2">{@render labelInput(editing, 'a')}</td>
							<td class="px-1 py-1.5 sm:px-2">{@render labelInput(editing, 'b')}</td>
						</tr>
					{:else}
						<tr class="border-t border-rule">
							<th scope="row" class="px-1 py-1.5 text-left align-top font-normal text-muted sm:px-2">{r.label}</th>
							{#each [r.a, r.b] as v, i (i)}
								<td
									class="truncate px-1 py-1.5 align-top sm:px-2"
									class:text-sm={r.key === 'file'}
									class:text-ink={r.key !== 'folder'}
									class:text-muted={r.key === 'folder'}
									title={r.key === 'file' ? (i === 0 ? d.a.file_path : d.b.file_path) : v}
								>
									{v}
								</td>
							{/each}
						</tr>
					{/if}
				{/each}
				<tr class="border-t border-rule">
					<td></td>
					{#each [[d.a, d.b], [d.b, d.a]] as [keep, trash] (keep.id)}
						<td class="px-1 py-2 sm:px-2">
							<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
								{@render keepThis(keep, trash)}
								<a
									href={`/media/${keep.id}`}
									target="_blank"
									rel="noopener"
									aria-label={`Open ${keep.file_name} in a new tab`}
									class="text-xs text-accent hover:text-ink"
								>
									Open <span aria-hidden="true">↗</span>
								</a>
							</div>
						</td>
					{/each}
				</tr>
			</tbody>
		</table>
	{/if}

	<div class="mt-3 flex flex-wrap items-center justify-end gap-2 border-t border-rule pt-3">
		{#if editing}
			{#if editError}
				<span class="mr-auto text-xs text-warn" role="alert">{editError}</span>
			{/if}
			<button onclick={cancelEdit} disabled={busy} class="btn-row btn-quiet">Cancel</button>
			<button onclick={save} disabled={busy} class="btn-row btn-pill btn-accent">
				{editing === 'edition' ? 'Save editions' : 'Save parts'}
			</button>
		{:else}
			{#if data?.can_label_editions}
				<button onclick={(e) => startEdit('edition', e)} disabled={busy} class="btn-row btn-quiet">Label as editions…</button>
			{/if}
			{#if data?.can_label_parts}
				<button onclick={(e) => startEdit('part', e)} disabled={busy} class="btn-row btn-quiet">Label as parts…</button>
			{/if}
			{@render verdicts()}
		{/if}
	</div>
</div>
