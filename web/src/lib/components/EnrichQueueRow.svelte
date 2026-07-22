<script lang="ts">
	// One dense entity row in the Enrichment review queue (F47 S2/S3, ADR-066). Mirrors
	// DuplicatePairRow's rhythm: a status chip per outstanding provider, plus one
	// right-aligned row action derived from the chips' states (RD9) — "Review" opens
	// EnrichPicker for the row's next outstanding provider (auto-apply-on-single-
	// strong-match lives inside EnrichPicker itself, so both this row and the
	// detail-page "Enrich" button get it for free); "Try again" clears a durable "not
	// matched" dismissal and reopens the picker for it. Enrichment rows never
	// disappear on resolve — they update chips in place and re-sort (handoff's
	// Animation/Motion table). Tokens only.
	import { toMessage } from '$lib/format';
	import ProviderStatusChip from './ProviderStatusChip.svelte';
	import EnrichPicker from './EnrichPicker.svelte';
	import type { EnrichCandidate, EnrichedField, EnrichQueueProviderState, EnrichQueueRow } from '$lib/types';

	let {
		row,
		href,
		resolve,
		apply,
		dismiss,
		undismiss,
		onchange
	}: {
		row: EnrichQueueRow;
		href: string;
		resolve: (provider: string, query: string) => Promise<{ candidates: EnrichCandidate[] }>;
		apply: (provider: string, externalId: string) => Promise<{ enriched: EnrichedField[] }>;
		dismiss: (provider: string) => Promise<unknown>;
		undismiss: (provider: string) => Promise<unknown>;
		/** Bubbles the row's updated provider states up so the queue can re-sort/re-group. */
		onchange: (providers: EnrichQueueProviderState[]) => void;
	} = $props();

	// The provider whose EnrichPicker is open ('' = closed).
	let pickerProvider = $state('');
	let busy = $state(false);
	let error = $state('');
	// Providers the owner has opened this session — purely presentational (flips a
	// chip's "not yet reviewed" to "Needs review"); never sent to the parent, since
	// it isn't domain state (unlike `not_matched`/`auto_applied`, ADR-066's real
	// server-confirmed outcomes).
	let reviewed = $state(new Set<string>());

	// Providers still worth a "Review" click.
	const outstanding = $derived(row.providers.filter((p) => p.state === 'unreviewed'));
	// Nothing to review, but not done either — every provider was dismissed.
	const allNotMatched = $derived(
		row.providers.length > 0 && row.providers.every((p) => p.state === 'not_matched')
	);

	function setState(provider: string, state: EnrichQueueProviderState['state']) {
		onchange(row.providers.map((p) => (p.provider === provider ? { ...p, state } : p)));
	}

	// "Review" (RD2/RD3): opens EnrichPicker for the row's next outstanding provider —
	// same resolve/apply wiring as the detail page's "Enrich" button.
	function review() {
		const target = outstanding[0];
		if (!target) return;
		reviewed = new Set(reviewed).add(target.provider);
		pickerProvider = target.provider;
	}

	// "Try again" (RD4): only rendered once every provider on the row is
	// `not_matched`, so clearing the dismissal on all of them (in parallel) and
	// reopening the picker for the first is unconditional — no per-provider filter
	// needed.
	async function tryAgain() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			await Promise.all(row.providers.map((p) => undismiss(p.provider)));
			onchange(row.providers.map((p) => ({ ...p, state: 'unreviewed' })));
			const [first] = row.providers;
			if (first) {
				reviewed = new Set(reviewed).add(first.provider);
				pickerProvider = first.provider;
			}
		} catch (e) {
			error = toMessage(e);
		} finally {
			busy = false;
		}
	}

	// `.btn-row`/`.btn-pill` (app.css) carry the shape and size shared with the
	// other owner queue rows (ExtractionQueueRow, DuplicatePairRow).
	const PILL_ACTION = 'btn-row btn-pill btn-accent';
</script>

<div
	class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
	role="group"
	aria-label={`${row.name}: enrichment status`}
>
	<div class="flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1">
		<a {href} class="truncate text-ink hover:underline" title={row.name}>{row.name}</a>
		{#each row.providers as p (p.provider)}
			<ProviderStatusChip provider={p.provider} state={p.state} reviewed={reviewed.has(p.provider)} />
		{/each}
	</div>

	{#if error}
		<span class="text-warn" role="alert">{error}</span>
	{/if}

	<div class="flex shrink-0 items-center gap-2">
		{#if outstanding.length > 0}
			<button onclick={review} disabled={busy} class={PILL_ACTION}> Review </button>
		{:else if allNotMatched}
			<button onclick={tryAgain} disabled={busy} class={PILL_ACTION}>
				{busy ? 'Trying again…' : 'Try again'}
			</button>
		{/if}
	</div>
</div>

{#if pickerProvider}
	<EnrichPicker
		entityName={row.name}
		provider={pickerProvider}
		{resolve}
		{apply}
		{dismiss}
		onclose={() => (pickerProvider = '')}
		onapplied={() => setState(pickerProvider, 'auto_applied')}
		ondismissed={() => setState(pickerProvider, 'not_matched')}
	/>
{/if}
