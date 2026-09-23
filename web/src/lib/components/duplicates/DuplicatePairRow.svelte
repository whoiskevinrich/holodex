<script lang="ts">
	// One dense pair row in the Duplicates review queue (F43 S5, ADR-061 — Option A,
	// ratified). Shows both entities (name · count), the variation kind, and the two
	// verdicts: Keep separate (records keep-separate — the pair never re-surfaces; the
	// row is removed, no fade) and Merge (pick which name survives, fold the other in).
	//
	// **Keep separate carries the accent, Merge is the bordered neutral** (F70 P0-4):
	// the probe found keep-separate is the dominant verdict 185:31, so accenting Merge
	// optimized the page backwards. Merge is still two-step and still bordered — it is
	// the least reversible action here, which is why it is NOT `.btn-quiet`.
	//
	// A PERSON pair also gets a disclosure opening `DuplicateComparePanel` (F70,
	// HOLODEX-451) — and on a person pair the match-kind label moves INTO that panel,
	// which is what keeps this row to one 40 px line. Every other entity kind keeps the
	// label here: it has no panel to move it to, and without it two unalike names are
	// paired with nothing explaining why.
	// Tokens only; QA 3 skins.
	import { videoCount, toMessage, refLabel } from '$lib/format';
	import type { DuplicatePair } from '$lib/types';
	import DuplicateComparePanel from './DuplicateComparePanel.svelte';

	let {
		pair,
		merge,
		dismiss,
		onresolved,
		expanded = false,
		onexpand
	}: {
		pair: DuplicatePair;
		/** Fold `fromId` into `survivorId` (the per-entity merge). */
		merge: (survivorId: number, fromId: number) => Promise<unknown>;
		/** Record the pair keep-separate. */
		dismiss: () => Promise<unknown>;
		onresolved: () => void;
		/** Whether this row's compare panel is open. The page owns it so only one is (RD12). */
		expanded?: boolean;
		/** Ask the page to open/close this row's panel. */
		onexpand?: (open: boolean) => void;
	} = $props();

	let choosing = $state(false); // showing the "keep which name?" survivor buttons
	let busy = $state(false);
	let error = $state('');
	let disclosure = $state<HTMLButtonElement | null>(null);

	async function doMerge(survivorId: number, fromId: number) {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await merge(survivorId, fromId);
			onresolved();
		} catch (e) {
			error = toMessage(e);
			busy = false;
		}
	}

	async function doDismiss() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await dismiss();
			onresolved();
		} catch (e) {
			error = toMessage(e);
			busy = false;
		}
	}

	// `.btn-row`/`.btn-pill` (app.css) carry the shape and size shared with the
	// other owner queue rows (ExtractionQueueRow, EnrichQueueRow).
	const PILL_ACTION = 'btn-row btn-pill btn-accent';
	const GHOST = 'btn-row btn-ghost px-2';
	const TOGGLE = 'btn-row btn-quiet';

	// matchKindLabel explains WHY the pair was flagged — the two canonical names shown
	// above can look nothing alike when the match came through an alias, which read as
	// unexplained/wrong before this label existed. 'canonical' needs no badge (that's
	// the two names visibly matching, self-evident).
	const matchKindLabel: Record<string, string> = {
		mixed: 'via alias',
		alias: 'alias match only — weak signal'
	};
	const label = $derived(matchKindLabel[pair.match_kind] ?? '');
	const weak = $derived(pair.match_kind === 'alias');

	// The panel is person-only (RD10): a studio/tag/film pair has no evidence worth two
	// columns, and a control that cannot change what you see is one the reader learns to
	// distrust (ExpandableText's rule).
	const comparable = $derived(pair.entity_type === 'person');
	const panelId = $derived(`dup-panel-${pair.entity_type}-${pair.a.id}-${pair.b.id}`);
	const open = $derived(comparable && expanded);

	// Escape collapses the open panel and returns focus to its disclosure. Bound at the
	// window rather than on a container because at most one panel is open app-wide, so
	// there is nothing to disambiguate — and `use:dismissable` is explicitly wrong here:
	// it also closes on outside click, which would collapse the evidence the moment you
	// reached for another row's verdict.
	function onWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' || !open) return;
		onexpand?.(false);
		disclosure?.focus();
	}
</script>

{#snippet verdicts()}
	{#if choosing}
		<span class="text-xs text-muted">Keep:</span>
		<button onclick={() => doMerge(pair.a.id, pair.b.id)} disabled={busy} class={PILL_ACTION}>
			{refLabel(pair.a)}
		</button>
		<button onclick={() => doMerge(pair.b.id, pair.a.id)} disabled={busy} class={PILL_ACTION}>
			{refLabel(pair.b)}
		</button>
		<button onclick={() => (choosing = false)} disabled={busy} class={TOGGLE}> Cancel </button>
	{:else}
		<button onclick={doDismiss} disabled={busy} class={PILL_ACTION}> Keep separate </button>
		<button onclick={() => (choosing = true)} disabled={busy} class={GHOST}> Merge </button>
	{/if}
{/snippet}

<svelte:window onkeydown={onWindowKeydown} />

<div
	class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
	role="group"
	aria-label={`Possible duplicate: ${refLabel(pair.a)} and ${refLabel(pair.b)}`}
>
	{#if comparable}
		<button
			type="button"
			bind:this={disclosure}
			id={`dup-disclosure-${pair.entity_type}-${pair.a.id}-${pair.b.id}`}
			onclick={() => onexpand?.(!open)}
			aria-expanded={open}
			aria-controls={panelId}
			aria-label={open
				? `Hide evidence for ${refLabel(pair.a)} and ${refLabel(pair.b)}`
				: `Compare ${refLabel(pair.a)} and ${refLabel(pair.b)}`}
			title={open ? 'Hide evidence' : 'Compare'}
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
	{/if}

	<!-- `sm:flex-nowrap` is the one that matters: from 640px up this block never wraps,
	     so a long pair TRUNCATES and the scan line stays one row tall (P0-1). With
	     `flex-wrap` on at every width the browser wraps before it shrinks, and a long
	     pair would grow the row again. The two names carry `truncate`, whose
	     `overflow:hidden` is what lets them shrink past their content to absorb it.
	     Below 640px it wraps: there the meta spans are all `shrink-0` and together
	     outweigh the row, so nowrap crushed both names to ~22px of ellipsis. A phone
	     gets a taller, readable row instead — the genuinely-narrow escape hatch.
	     `min-w-64`, NOT `min-w-0`: with a zero floor this block shrinks to nothing
	     instead of letting the ROOT wrap, so at in-between widths the `shrink-0` meta
	     overflowed its own box and painted under the verdict pills. The floor is what
	     makes the root's `flex-wrap` fire and drop the verdicts to their own line. -->
	<div class="flex min-w-64 flex-1 flex-wrap items-center gap-x-2 gap-y-1 sm:flex-nowrap">
		<span class="truncate text-ink">{refLabel(pair.a)}</span>
		<span class="shrink-0 text-xs text-muted">{videoCount(pair.a.video_count ?? 0)}</span>
		<span aria-hidden="true" class="shrink-0 text-muted">↔</span>
		<span class="truncate text-ink">{refLabel(pair.b)}</span>
		<span class="shrink-0 text-xs text-muted">{videoCount(pair.b.video_count ?? 0)}</span>
		<span class="shrink-0 text-xs text-muted">· {pair.variation}</span>
		{#if label && !comparable}
			<span class="shrink-0 text-xs" class:text-warn={weak} class:text-muted={!weak}>
				· {label}
			</span>
		{/if}
	</div>

	{#if error}
		<span class="text-warn" role="alert">{error}</span>
	{/if}

	<div class="flex shrink-0 flex-wrap items-center gap-2">
		{@render verdicts()}
	</div>
</div>

{#if open}
	<DuplicateComparePanel {pair} {panelId} matchLabel={label} matchWeak={weak} {verdicts} />
{/if}
