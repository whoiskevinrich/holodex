<script module lang="ts">
	import { api } from '$lib/api';
	import type { PersonImageSet } from '$lib/types';

	// The strip's half of the session cache. `loadPersonCard` already gives
	// `/people/{id}/card` one request per person per session (F68), but
	// `api.getPersonImages` has no cache of its own — without this, reopening a panel
	// re-fetched the image set every time and P0-5 held for only half the payload.
	// Same contract as the card cache: shared across every panel, and a REJECTED
	// promise is evicted so the next open retries rather than replaying the failure.
	const imageCache = new Map<number, Promise<PersonImageSet>>();

	function loadPersonImages(id: number): Promise<PersonImageSet> {
		let p = imageCache.get(id);
		if (!p) {
			p = api.getPersonImages(id).catch((err) => {
				if (imageCache.get(id) === p) imageCache.delete(id);
				throw err;
			});
			imageCache.set(id, p);
		}
		return p;
	}
</script>

<script lang="ts">
	// The F70 compare panel (HOLODEX-451, docs/design/duplicates-pair-evidence-handoff.md):
	// a flagged PERSON pair expanded in place into two columns of evidence, so the owner
	// can tell one person from two without leaving the queue.
	//
	// **Each column IS the F68 hover card laid flat** — same fields, same order, same
	// absent-is-absent rule (no "—", no "unknown"), with a strip of five 44 px frames
	// where the card's single 48 px headshot was. The hover card was never the wrong
	// content, only the wrong container (spec RD2).
	//
	// The panel NEVER adjudicates (RD3/RD4): it reports who said what and stops. No
	// conflict chip, no summary line, no co-appearance marker — those were all tried and
	// are exactly the cross-side comparison the epic forbids. The footer carries verdicts
	// and nothing else, rendered from the row's own snippet so the two verdict pairs can
	// never drift apart or disagree about `busy`.
	import type { DuplicatePair, EntityRef, PersonCard, PersonImage } from '$lib/types';
	import { onDestroy, onMount, type Snippet } from 'svelte';
	import { videoCount, refLabel, sortExternalLinks } from '$lib/format';
	import { loadPersonCard, type CardLease } from '$lib/components/person/personCard.svelte';
	import PersonImageFrame from '$lib/components/person/PersonImageFrame.svelte';
	import NationalityFlags from '$lib/components/person/NationalityFlags.svelte';
	import ProviderLinkBadge from '$lib/components/enrichment/ProviderLinkBadge.svelte';

	let {
		pair,
		panelId,
		matchLabel,
		matchWeak,
		verdicts
	}: {
		pair: DuplicatePair;
		panelId: string;
		/** The row's own match-kind string, passed rather than forked (handoff: reuse, don't copy). */
		matchLabel: string;
		/** `alias` is the weak signal — `text-warn`; `mixed` is merely explanatory — `text-muted`. */
		matchWeak: boolean;
		/** The row's verdict block, repeated in the footer: same handlers, same `busy`. */
		verdicts: Snippet;
	} = $props();

	// The strip is a SAMPLE, not an index (OQ2): five frames, no `+N`. Slot 1 is always
	// the headshot role — the backend serves a themed placeholder when there is none, so
	// a person with no images gets exactly one frame, never five empty wells.
	const GALLERY_SLOTS = 4;
	const LOADING_SLOTS = [0, 1, 2, 3, 4];

	interface Side {
		card: PersonCard | null;
		gallery: PersonImage[];
		failed: boolean;
	}
	const blank = (): Side => ({ card: null, gallery: [], failed: false });
	let sides = $state<Record<'a' | 'b', Side>>({ a: blank(), b: blank() });

	// `loadPersonCard` shares F68's per-session cache, so a person already hovered
	// anywhere costs nothing here and reopening this panel issues no second request
	// (P0-5). `release()` REFCOUNTS — it only aborts while the request is still in
	// flight and nobody else is waiting; a settled entry stays cached. So holding the
	// lease for the panel's lifetime and releasing on teardown is both correct and
	// enough: collapsing mid-flight cancels, collapsing after it landed keeps the cache.
	//
	// onMount/onDestroy rather than `$effect`: the panel is created fresh by the row's
	// `{#if expanded}` and its `pair` never changes under it, and `load()` writes the
	// same `sides` an effect would have been tracking.
	let leases: CardLease[] = [];
	onMount(() => {
		load('a', pair.a);
		load('b', pair.b);
	});
	onDestroy(() => {
		for (const l of leases) l.release();
		leases = [];
	});

	async function load(key: 'a' | 'b', ref: EntityRef) {
		const side = sides[key];
		side.failed = false;
		const lease = loadPersonCard(ref.id);
		leases.push(lease);
		try {
			// Both reads are needed before the column is worth drawing; the card alone
			// has no strip and the images alone have no name.
			const [card, images] = await Promise.all([lease.promise, loadPersonImages(ref.id)]);
			side.card = card;
			// The headshot is slot 1 already; `role` is the only thing that tells the two
			// apart, since the ids aren't otherwise comparable.
			side.gallery = images.gallery
				.filter((img) => img.role !== 'headshot')
				.slice(0, GALLERY_SLOTS);
		} catch {
			// Not cached (the card cache drops failed flights), so Retry re-requests.
			side.failed = true;
		}
	}

	function shownName(key: 'a' | 'b', ref: EntityRef): string {
		return sides[key].card?.display_name ?? sides[key].card?.name ?? refLabel(ref);
	}

	// PersonHoverCard's meta rule verbatim: age XOR †age_at_death, the video count always,
	// films only when there are any. Absent segments are never pushed, so the ` · ` join
	// can't orphan a separator.
	function meta(card: PersonCard | null): string[] {
		if (!card) return [];
		const parts: string[] = [];
		if (card.age !== undefined) parts.push(String(card.age));
		else if (card.age_at_death !== undefined) parts.push(`†${card.age_at_death}`);
		parts.push(videoCount(card.video_count));
		if (card.film_count > 0) parts.push(`${card.film_count} film${card.film_count === 1 ? '' : 's'}`);
		return parts;
	}
</script>

{#snippet column(key: 'a' | 'b', ref: EntityRef)}
	{@const side = sides[key]}
	{@const card = side.card}
	{@const name = shownName(key, ref)}
	{@const segments = meta(card)}
	{@const aliases = card?.aliases?.slice(0, 3) ?? []}
	<section
		aria-label={name}
		class="min-w-0 rounded-theme border border-rule bg-surface p-3"
	>
		<div class="flex gap-1">
			{#if card}
				<PersonImageFrame
					personId={ref.id}
					role="headshot"
					name={name}
					version={card.headshot_version}
					frameClass="portrait-frame--1x1 w-11 shrink-0"
					eager
				/>
				{#each side.gallery as img (img.id)}
					<!-- Same person, five times: announcing the name five times is noise. -->
					<PersonImageFrame
						personId={ref.id}
						role={img.role}
						imageId={img.id}
						name={name}
						alt=""
						version={img.version}
						frameClass="portrait-frame--1x1 w-11 shrink-0"
					/>
				{/each}
			{:else}
				<!-- Loading holds all five slots so nothing shifts when the images land
				     (asking for the headshot before its version is known would fetch it
				     twice). A FAILED side holds one: nothing is ever going to land there,
				     and five empty wells claim five images this person may not have —
				     the same overstatement P0-3 rules out for a person with no images. -->
				{#each side.failed ? [0] : LOADING_SLOTS as slot (slot)}
					<span class="portrait-frame portrait-frame--1x1 w-11 shrink-0" aria-hidden="true"></span>
				{/each}
			{/if}
		</div>

		<span class="mt-2 flex items-center gap-1.5">
			<span class="skin-title truncate font-display text-sm font-semibold">{name}</span>
			{#if card?.nationality?.length}
				<NationalityFlags values={card.nationality} />
			{/if}
		</span>
		{#if segments.length}
			<span class="block text-xs text-muted">{segments.join(' · ')}</span>
		{/if}
		{#if aliases.length}
			<span class="block truncate text-xs italic text-muted">also credited as {aliases.join(', ')}</span>
		{/if}

		{#if side.failed}
			<p class="mt-2 text-xs text-warn" role="alert">Couldn't load this side.</p>
			<button type="button" onclick={() => load(key, ref)} class="btn-row btn-quiet mt-1">
				Retry
			</button>
		{:else if card}
			<!-- The panel's only in-app path to a profile: the pair-row names are still
			     `<span>`s (P2-1) and there is no `+N`, so without these it's a dead end. -->
			<div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-rule pt-2 text-xs">
				<a href={`/people/${ref.id}#videos`} class="text-accent hover:text-ink">Videos</a>
				{#if card.film_count > 0}
					<a href={`/people/${ref.id}#films`} class="text-accent hover:text-ink">Films</a>
				{/if}
				{#each sortExternalLinks(card.external_links ?? []) as link (link.provider)}
					<ProviderLinkBadge {link} entityName={name} />
				{/each}
			</div>
		{/if}
	</section>
{/snippet}

<div
	id={panelId}
	role="group"
	aria-label={`Compare ${refLabel(pair.a)} and ${refLabel(pair.b)}`}
	class="border-t border-rule bg-surface-2 px-3 py-3"
>
	{#if matchLabel}
		<!-- Why the DETECTOR fired — it describes the pair, so it can't sit in a column,
		     and RD3/RD4 keep it out of the footer. It opens the well instead. -->
		<p class="mb-3 text-xs" class:text-warn={matchWeak} class:text-muted={!matchWeak}>
			{matchLabel}
		</p>
	{/if}
	<div class="field-grid gap-4">
		{@render column('a', pair.a)}
		{@render column('b', pair.b)}
	</div>
	<div class="mt-3 flex flex-wrap items-center justify-end gap-2 border-t border-rule pt-3">
		{@render verdicts()}
	</div>
</div>
