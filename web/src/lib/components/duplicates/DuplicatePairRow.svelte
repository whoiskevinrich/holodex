<script lang="ts">
	// One dense pair row in the Duplicates review queue (F43 S5, ADR-061 — Option A,
	// ratified). Shows both entities (name · count), the variation kind, and the verdicts.
	// Two verdict grammars, split on `showsComparePanel`:
	//
	// **Tag and film pairs (RD7):** Keep separate carries the accent, Merge is the bordered
	// neutral (F70 P0-4): the probe found keep-separate is the dominant verdict 185:31, so
	// accenting Merge optimized the page backwards. Merge is still two-step and still
	// bordered — it is the least reversible action here, which is why it is NOT `.btn-quiet`.
	//
	// **Person and studio pairs (598, the Videos lane's grammar):** the row offers only Keep
	// separate, as the ghost; the survivor is chosen in the compare panel, with "Merge into
	// this one" (the accent pill) at the foot of each column, behind a `ConfirmDialog`. The
	// whole row toggles the panel (RD16) and the names open their page in a new tab (RD17).
	// On these pairs the match-kind label moves INTO the panel, which keeps this row to one
	// 40 px line.
	//
	// **The row owns every verdict, `busy` and the confirm** (the duplicates folder rule):
	// the panel renders the `verdicts` and `mergeInto` snippets it is handed.
	// Tokens only; QA Cinémathèque.
	import { videoCount, toMessage, refLabel } from '#lib/format.ts';
	import type { DuplicatePair, EntityRef } from '#lib/types.ts';
	import ConfirmDialog from '#lib/components/shared/ConfirmDialog.svelte';
	import { invalidatePersonCard, loadPersonCard } from '#lib/components/person/personCard.svelte.ts';
	import DuplicateComparePanel from './DuplicateComparePanel.svelte';
	import DuplicateStudioPanel, { invalidateStudioEvidence } from './DuplicateStudioPanel.svelte';
	import {
		disclosureId,
		labelPlacement,
		matchKindLabel,
		panelId,
		profileHref,
		sharedIdChip,
		showsComparePanel
	} from './queue';

	type ShowAs = 'survivor' | 'merged';

	let {
		pair,
		merge,
		dismiss,
		onresolved,
		expanded = false,
		onexpand
	}: {
		pair: DuplicatePair;
		/** Fold `fromId` into `survivorId` (the per-entity merge). `showAs` is studio-only. */
		merge: (survivorId: number, fromId: number, showAs?: ShowAs) => Promise<unknown>;
		/** Record the pair keep-separate. */
		dismiss: () => Promise<unknown>;
		onresolved: () => void;
		/** Whether this row's compare panel is open. The page owns it so only one is (RD12). */
		expanded?: boolean;
		/** Ask the page to open/close this row's panel. */
		onexpand?: (open: boolean) => void;
	} = $props();

	let choosing = $state(false); // tag/film: showing the "keep which name?" survivor buttons
	let busy = $state(false);
	let error = $state('');
	let disclosure = $state<HTMLButtonElement | null>(null);
	// Person/studio: the open merge confirm. `otherFilms` drops the person body's "films"
	// clause when the merged-away side has none; it comes from the card the panel already
	// cached, so it costs no request.
	let confirm = $state<{ keep: EntityRef; other: EntityRef; otherFilms: number } | null>(null);
	let showAs = $state<ShowAs>('survivor');
	let confirmError = $state('');

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

	function openConfirm(keep: EntityRef, other: EntityRef) {
		confirmError = '';
		showAs = 'survivor';
		confirm = { keep, other, otherFilms: 0 };
		if (pair.entity_type !== 'person') return;
		const lease = loadPersonCard(other.id);
		lease.promise
			.then((card) => {
				if (confirm?.other.id === other.id) confirm.otherFilms = card.film_count;
			})
			.catch(() => {})
			.finally(() => lease.release());
	}

	async function confirmMerge() {
		if (!confirm || busy) return;
		busy = true;
		confirmError = '';
		try {
			await merge(confirm.keep.id, confirm.other.id, isStudio ? showAs : undefined);
			// The survivor may sit in another pair; its next panel must not show it pre-merge.
			const invalidate = isStudio ? invalidateStudioEvidence : invalidatePersonCard;
			invalidate(confirm.keep.id);
			invalidate(confirm.other.id);
			confirm = null;
			onresolved();
		} catch (e) {
			confirmError = toMessage(e);
			busy = false;
		}
	}

	// `.btn-row`/`.btn-pill` (app.css) carry the shape and size shared with the
	// other owner queue rows (ExtractionQueueRow, EnrichQueueRow).
	const PILL_ACTION = 'btn-row btn-pill btn-accent';
	const GHOST = 'btn-row btn-ghost px-2';
	const TOGGLE = 'btn-row btn-quiet';

	// Why the pair was flagged, and where that explanation renders — both in `queue.ts`,
	// where they are unit-tested (`queue.test.ts`).
	const match = $derived(matchKindLabel(pair.match_kind));
	const labelInRow = $derived(labelPlacement(pair) === 'row');
	const chip = $derived(sharedIdChip(pair));
	const comparable = $derived(showsComparePanel(pair.entity_type));
	const isStudio = $derived(pair.entity_type === 'studio');
	const open = $derived(comparable && expanded);

	// Escape collapses the open panel and returns focus to its disclosure. Bound at the
	// window rather than on a container because at most one panel is open app-wide, so
	// there is nothing to disambiguate — and `use:dismissable` is explicitly wrong here:
	// it also closes on outside click, which would collapse the evidence the moment you
	// reached for another row's verdict. An open confirm owns Escape first.
	function onWindowKeydown(e: KeyboardEvent) {
		if (e.key !== 'Escape' || !open || confirm) return;
		onexpand?.(false);
		disclosure?.focus();
	}

	// RD16: the whole row is a pointer convenience on top of the disclosure. A click on any
	// link or button (the disclosure included — it toggles itself) is that control's own.
	function onRowClick(e: MouseEvent) {
		if (!comparable || (e.target as Element).closest('a, button')) return;
		onexpand?.(!open);
	}
</script>

{#snippet verdicts()}
	{#if comparable}
		<button onclick={doDismiss} disabled={busy} class={GHOST}> Keep separate </button>
	{:else if choosing}
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

<!-- The panel's per-column verdict (C5). Named, because "this one" alone is ambiguous to a
     screen reader. -->
{#snippet mergeInto(keep: EntityRef, other: EntityRef)}
	<button
		onclick={() => openConfirm(keep, other)}
		disabled={busy}
		aria-label={`Merge ${refLabel(other)} into ${refLabel(keep)}`}
		class={PILL_ACTION}
	>
		Merge into this one
	</button>
{/snippet}

{#snippet name(ref: EntityRef)}
	{#if comparable}
		<a
			href={profileHref(pair.entity_type, ref.id)}
			target="_blank"
			rel="noopener"
			aria-label={`Open ${refLabel(ref)} in a new tab`}
			class="truncate text-ink hover:text-accent">{refLabel(ref)}</a
		>
		<span aria-hidden="true" class="-ml-0.5 shrink-0 text-xs text-muted">↗</span>
	{:else}
		<span class="truncate text-ink">{refLabel(ref)}</span>
	{/if}
{/snippet}

<svelte:window onkeydown={onWindowKeydown} />

<!-- The click is pointer-only and duplicates the disclosure button, so no keyboard path
     depends on it; the root takes no role or tabindex of its own (RD16, P0-1). -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<div
	class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm {comparable
		? 'cursor-pointer hover:bg-surface-2'
		: ''}"
	role="group"
	aria-label={`Possible duplicate: ${refLabel(pair.a)} and ${refLabel(pair.b)}`}
	onclick={onRowClick}
>
	{#if comparable}
		<button
			type="button"
			bind:this={disclosure}
			id={disclosureId(pair)}
			onclick={() => onexpand?.(!open)}
			aria-expanded={open}
			aria-controls={panelId(pair)}
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
		{@render name(pair.a)}
		<span class="shrink-0 text-xs text-muted">{videoCount(pair.a.video_count ?? 0)}</span>
		<span aria-hidden="true" class="shrink-0 text-muted">↔</span>
		{@render name(pair.b)}
		<span class="shrink-0 text-xs text-muted">{videoCount(pair.b.video_count ?? 0)}</span>
		<!-- The strongest signal the queue carries names its asserter instead of printing the
		     variation slug (F71 P0-6). Accent, not `text-warn`: on this surface warn already
		     means *weak*, which is the opposite of what a provider asserting one record means.
		     It REPLACES the slug rather than joining it, so the row keeps its one 40px line at
		     every width — which is why this stays in the row for a person too, unlike the
		     match-kind label. Outlined accent, not solid `bg-accent`: that is reserved for a
		     page's one primary action (app.css), and this is the same treatment
		     ExtractionQueueRow's staged chips use. -->
		{#if chip}
			<span
				class="shrink-0 rounded-full border border-accent bg-accent/10 px-2 py-0.5 text-xs text-accent"
			>
				{chip.text}
			</span>
		{:else}
			<span class="shrink-0 text-xs text-muted">· {pair.variation}</span>
		{/if}
		{#if labelInRow}
			<span class="shrink-0 text-xs" class:text-warn={match.weak} class:text-muted={!match.weak}>
				· {match.text}
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
	{#if isStudio}
		<DuplicateStudioPanel
			{pair}
			panelId={panelId(pair)}
			matchLabel={match.text}
			matchWeak={match.weak}
			{verdicts}
			{mergeInto}
		/>
	{:else}
		<DuplicateComparePanel
			{pair}
			panelId={panelId(pair)}
			matchLabel={match.text}
			matchWeak={match.weak}
			{verdicts}
			{mergeInto}
		/>
	{/if}
{/if}

{#if confirm}
	{@const keep = refLabel(confirm.keep)}
	{@const other = refLabel(confirm.other)}
	<ConfirmDialog
		title={`Merge into ${keep}?`}
		confirmLabel={isStudio && showAs === 'merged' ? 'Merge and rename' : 'Merge'}
		{busy}
		error={confirmError}
		onconfirm={confirmMerge}
		oncancel={() => (confirm = null)}
	>
		{#snippet body()}
			{#if isStudio}
				<p>
					<span class="font-semibold">{other}</span>’s {videoCount(confirm!.other.video_count ?? 0)} and links
					move to {keep}.
				</p>
				<fieldset class="space-y-1">
					<legend class="text-muted">Show the studio as</legend>
					<label class="flex items-center gap-2">
						<input type="radio" name="dup-show-as" value="survivor" bind:group={showAs} class="accent-accent" />
						<span>{keep}</span>
						<span class="text-xs text-muted">its name now</span>
					</label>
					<label class="flex items-center gap-2">
						<input type="radio" name="dup-show-as" value="merged" bind:group={showAs} class="accent-accent" />
						<span>{other}</span>
					</label>
				</fieldset>
				<!-- P0-11: this sentence names the spelling writeback will write, so it changes with
				     the pick rather than staying generic. -->
				{#if showAs === 'survivor'}
					<p>
						The other spelling stays as an alias. Writeback will offer <span class="font-semibold">{keep}</span>
						for these files.
					</p>
				{:else}
					<p>
						This renames the studio, as renaming it on its own page would. “{keep}” stays as an alias.
						Writeback will offer <span class="font-semibold">{other}</span> for these files.
					</p>
				{/if}
			{:else}
				<p>
					<span class="font-semibold">{other}</span>’s {videoCount(confirm!.other.video_count ?? 0)}{confirm!
						.otherFilms > 0
						? ', films'
						: ''} and links move to {keep}. “{other}” stays as an alias, so it still finds this person.
				</p>
			{/if}
			<p class="text-muted">This can’t be undone.</p>
		{/snippet}
	</ConfirmDialog>
{/if}
