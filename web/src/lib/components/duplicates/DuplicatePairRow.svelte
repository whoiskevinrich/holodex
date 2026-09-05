<script lang="ts">
	// One dense pair row in the Duplicates review queue (F43 S5, ADR-061 — Option A,
	// ratified). Shows both entities (name · count), the variation kind, a match-kind
	// label when the match isn't a direct canonical-name collision (the two names shown
	// can otherwise look unrelated with no explanation — it came through an alias), and
	// the two verdicts: Merge (pick which name survives, fold the other in) and Keep
	// separate (records keep-separate — the pair never re-surfaces; the row fades out).
	// Tokens only; QA 3 skins.
	import { videoCount, toMessage } from '$lib/format';
	import type { DuplicatePair } from '$lib/types';

	let {
		pair,
		merge,
		dismiss,
		onresolved
	}: {
		pair: DuplicatePair;
		/** Fold `fromId` into `survivorId` (the per-entity merge). */
		merge: (survivorId: number, fromId: number) => Promise<unknown>;
		/** Record the pair keep-separate. */
		dismiss: () => Promise<unknown>;
		onresolved: () => void;
	} = $props();

	let choosing = $state(false); // showing the "keep which name?" survivor buttons
	let busy = $state(false);
	let error = $state('');

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
</script>

<div
	class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
	role="group"
	aria-label={`Possible duplicate: ${pair.a.name} and ${pair.b.name}`}
>
	<div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1">
		<span class="truncate text-ink">{pair.a.name}</span>
		<span class="shrink-0 text-xs text-muted">{videoCount(pair.a.video_count ?? 0)}</span>
		<span aria-hidden="true" class="shrink-0 text-muted">↔</span>
		<span class="truncate text-ink">{pair.b.name}</span>
		<span class="shrink-0 text-xs text-muted">{videoCount(pair.b.video_count ?? 0)}</span>
		<span class="shrink-0 text-xs text-muted">· {pair.variation}</span>
		{#if matchKindLabel[pair.match_kind]}
			<span
				class="shrink-0 text-xs"
				class:text-warn={pair.match_kind === 'alias'}
				class:text-muted={pair.match_kind !== 'alias'}
			>
				· {matchKindLabel[pair.match_kind]}
			</span>
		{/if}
	</div>

	{#if error}
		<span class="text-warn" role="alert">{error}</span>
	{/if}

	<div class="flex shrink-0 flex-wrap items-center gap-2">
		{#if choosing}
			<span class="text-xs text-muted">Keep:</span>
			<button onclick={() => doMerge(pair.a.id, pair.b.id)} disabled={busy} class={PILL_ACTION}>
				{pair.a.name}
			</button>
			<button onclick={() => doMerge(pair.b.id, pair.a.id)} disabled={busy} class={PILL_ACTION}>
				{pair.b.name}
			</button>
			<button onclick={() => (choosing = false)} disabled={busy} class={TOGGLE}> Cancel </button>
		{:else}
			<button onclick={() => (choosing = true)} disabled={busy} class={PILL_ACTION}> Merge </button>
			<button onclick={doDismiss} disabled={busy} class={GHOST}> Keep separate </button>
		{/if}
	</div>
</div>
