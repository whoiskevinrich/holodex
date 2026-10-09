<script lang="ts">
	// One video pair in the Duplicates queue's Videos group (F76, HOLODEX-521). The two
	// files share a provider match, so they share a title: it shows ONCE, followed by a
	// per-side "4K · 34:14" summary. The row's only verdict is Keep both — keeping one copy
	// needs the facts, so "Keep this one" lives in the compare panel, one per column.
	//
	// **The row owns every verdict and `busy`** (the duplicates folder rule): the panel
	// renders the `verdicts` and `keepThis` snippets it is handed and calls `label`, so a
	// failed panel fetch can never disable an action and the row and panel can't disagree
	// about `busy`. The keep-one confirm is the row's too.
	// Tokens only; QA Cinémathèque.
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import { resolutionBucket, toMessage } from '$lib/format';
	import type { VideoCompareSide, VideoDuplicatePair } from '$lib/types';
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import VideoComparePanel from './VideoComparePanel.svelte';
	import { carryLines, sideSummary, videoDisclosureId, videoPanelId, type LabelField } from './videoPairs';

	let {
		pair,
		onresolved,
		expanded = false,
		onexpand
	}: {
		pair: VideoDuplicatePair;
		onresolved: () => void;
		/** Whether this row's compare panel is open. The page owns it so only one is. */
		expanded?: boolean;
		onexpand?: (open: boolean) => void;
	} = $props();

	let busy = $state(false);
	let error = $state('');
	let disclosure = $state<HTMLButtonElement | null>(null);
	let confirm = $state<{ keep: VideoCompareSide; trash: VideoCompareSide } | null>(null);
	let confirmError = $state('');

	const PILL_ACTION = 'btn-row btn-pill btn-accent';
	const GHOST = 'btn-row btn-ghost px-2';

	const graceDays = $derived(
		activity.caps?.delete_grace_period_seconds
			? Math.round(activity.caps.delete_grace_period_seconds / 86400)
			: 0
	);

	async function run(action: () => Promise<unknown>) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await action();
			onresolved();
		} catch (e) {
			error = toMessage(e);
			busy = false;
		}
	}

	const keepBoth = () => run(() => api.keepBothVideos(pair.a.id, pair.b.id));

	async function confirmKeep() {
		if (!confirm || busy) return;
		busy = true;
		confirmError = '';
		try {
			await api.keepVideo(confirm.keep.id, confirm.trash.id);
			confirm = null;
			onresolved();
		} catch (e) {
			confirmError = toMessage(e);
			busy = false;
		}
	}

	/** Saving labels resolves the pair; a failure surfaces in the row and rethrows so the
	 *  panel keeps its editor open. */
	async function label(field: LabelField, a: string, b: string) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await api.labelVideoPair(field, [
				{ id: pair.a.id, value: a },
				{ id: pair.b.id, value: b }
			]);
			onresolved();
		} catch (e) {
			error = toMessage(e);
			busy = false;
			throw e;
		}
	}

	const open = $derived(expanded);

	// Escape collapses the open panel (as the person queue does). A label editor stops
	// its own Escape first, and an open confirm owns Escape while it is up.
	function onWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' || !open || confirm) return;
		onexpand?.(false);
		disclosure?.focus();
	}
</script>

{#snippet verdicts()}
	<button onclick={keepBoth} disabled={busy} class={GHOST}> Keep both </button>
{/snippet}

{#snippet keepThis(keep: VideoCompareSide, trash: VideoCompareSide)}
	<button
		onclick={() => {
			confirmError = '';
			confirm = { keep, trash };
		}}
		disabled={busy}
		aria-label={`Keep the ${resolutionBucket(keep.width)} copy, ${keep.file_name}`}
		class={PILL_ACTION}
	>
		Keep this one
	</button>
{/snippet}

<svelte:window onkeydown={onWindowKeydown} />

<div
	class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
	role="group"
	aria-label={`Possible duplicate files: ${pair.title}`}
>
	<button
		type="button"
		bind:this={disclosure}
		id={videoDisclosureId(pair)}
		onclick={() => onexpand?.(!open)}
		aria-expanded={open}
		aria-controls={videoPanelId(pair)}
		aria-label={open ? `Hide the comparison for ${pair.title}` : `Compare the two files of ${pair.title}`}
		title={open ? 'Hide comparison' : 'Compare'}
		class="btn-quiet flex h-7 w-7 shrink-0 items-center justify-center rounded-theme hover:bg-surface-2"
	>
		<svg
			class="h-4 w-4 transition-transform duration-200 motion-reduce:transition-none"
			class:rotate-180={open}
			viewBox="0 0 24 24"
			fill="none"
			stroke="currentColor"
			stroke-width="2"
			aria-hidden="true"
		>
			<path stroke-linecap="round" stroke-linejoin="round" d="M6 9l6 6 6-6" />
		</svg>
	</button>

	<!-- Same wrap contract as DuplicatePairRow: from `sm` up the title truncates and the
	     row stays one line; below it the facts wrap under the title. -->
	<div class="flex min-w-64 flex-1 flex-wrap items-center gap-x-2 gap-y-1 sm:flex-nowrap">
		<span class="truncate text-ink" title={pair.title}>{pair.title}</span>
		<span class="shrink-0 text-xs text-muted">{sideSummary(pair.a)} ↔ {sideSummary(pair.b)}</span>
		<span class="shrink-0 text-xs text-muted">· same provider match</span>
	</div>

	{#if error}
		<span class="text-warn" role="alert">{error}</span>
	{/if}

	<div class="flex shrink-0 flex-wrap items-center gap-2">
		{@render verdicts()}
	</div>
</div>

{#if open}
	<VideoComparePanel {pair} panelId={videoPanelId(pair)} {busy} {verdicts} {keepThis} {label} />
{/if}

{#if confirm}
	<ConfirmDialog
		title="Move the other copy to Trash?"
		confirmLabel="Move to Trash"
		{busy}
		error={confirmError}
		onconfirm={confirmKeep}
		oncancel={() => (confirm = null)}
	>
		{#snippet body()}
			{@const lines = carryLines(confirm!.keep.if_kept)}
			<p>
				<span class="font-semibold">{confirm!.trash.file_name}</span>
				({resolutionBucket(confirm!.trash.width)}) will be hidden from your library{graceDays
					? ` and permanently deleted in ${graceDays} ${graceDays === 1 ? 'day' : 'days'}`
					: ''}. You can restore it from Trash{graceDays ? ' until then' : ''}.
			</p>
			{#if lines.length}
				<p>Moving to the copy you keep:</p>
				<ul class="list-disc space-y-0.5 pl-5 text-muted">
					{#each lines as line (line)}
						<li>{line}</li>
					{/each}
				</ul>
			{:else}
				<p class="text-muted">It carries no playlists, film link or edits of its own.</p>
			{/if}
		{/snippet}
	</ConfirmDialog>
{/if}
