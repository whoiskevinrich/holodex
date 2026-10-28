<script lang="ts">
	// The F70 compare panel (HOLODEX-451, docs/design/duplicates-pair-evidence-handoff.md):
	// a flagged PERSON pair expanded in place into two columns of evidence, so the owner
	// can tell one person from two without leaving the queue.
	//
	// **Each column IS the F68 hover card laid flat** — same fields, same order, same
	// absent-is-absent rule (no "—", no "unknown"), under a 240 px lead image (598, RD18)
	// with a strip of five 44 px frames that swap what the lead shows. The hover card was
	// never the wrong content, only the wrong container (spec RD2).
	//
	// The panel NEVER adjudicates (RD3/RD4): it reports who said what and stops. No
	// conflict chip, no summary line, no co-appearance marker — those were all tried and
	// are exactly the cross-side comparison the epic forbids. Every verdict comes from the
	// row's own snippets — Keep separate in the footer, Merge into this one at the foot of
	// each column — so the row and panel can never drift apart or disagree about `busy`.
	import type { DuplicatePair, EntityRef, PersonCard, PersonImage } from '#lib/types.ts';
	import { onDestroy, onMount, type Snippet } from 'svelte';
	import { videoCount, refLabel, sortExternalLinks } from '#lib/format.ts';
	import { loadPersonCard, type CardLease } from '#lib/components/person/personCard.svelte.ts';
	import {
		loadPersonImages,
		stripGallery,
		STRIP_GALLERY_SLOTS
	} from '#lib/components/person/personImages.ts';
	import PersonImageFrame from '#lib/components/person/PersonImageFrame.svelte';
	import NationalityFlags from '#lib/components/person/NationalityFlags.svelte';
	import ProviderLinkBadge from '#lib/components/enrichment/ProviderLinkBadge.svelte';
	import { profileHref } from './queue';

	let {
		pair,
		panelId,
		matchLabel,
		matchWeak,
		verdicts,
		mergeInto
	}: {
		pair: DuplicatePair;
		panelId: string;
		/** The row's own match-kind string, passed rather than forked (handoff: reuse, don't copy). */
		matchLabel: string;
		/** `alias` is the weak signal — `text-warn`; `mixed` is merely explanatory — `text-muted`. */
		matchWeak: boolean;
		/** The row's verdict block, repeated in the footer: same handlers, same `busy`. */
		verdicts: Snippet;
		/** The row's Merge into this one, rendered at the foot of each column. */
		mergeInto: Snippet<[EntityRef, EntityRef]>;
	} = $props();

	// What each side's lead shows: the headshot, or a strip image by id. Per side and
	// panel-local, so choosing a frame on A never touches B, and a reopened panel (a fresh
	// mount) starts on the headshots again.
	type LeadKey = 'headshot' | number;
	let lead = $state<Record<'a' | 'b', LeadKey>>({ a: 'headshot', b: 'headshot' });

	// The strip is a SAMPLE, not an index (OQ2): five frames, no `+N`. Slot 1 is always
	// the headshot role — the backend serves a themed placeholder when there is none, so
	// a person with no images gets exactly one frame, never five empty wells. The slice
	// rule itself is `stripGallery` (pinned in `personImages.test.ts`), and the skeleton
	// below is derived from the SAME cap rather than repeating the 5 — the loading state
	// must reserve exactly what the loaded state will fill, or the column jumps.
	const LOADING_SLOTS = Array.from({ length: STRIP_GALLERY_SLOTS + 1 }, (_, i) => i);

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
			side.gallery = stripGallery(images);
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

{#snippet newTab(href: string, label: string)}
	<a {href} target="_blank" rel="noopener" class="text-accent hover:text-ink"
		>{label}<span aria-hidden="true" class="ml-0.5 text-muted">↗</span></a
	>
{/snippet}

{#snippet column(key: 'a' | 'b', ref: EntityRef, other: EntityRef)}
	{@const side = sides[key]}
	{@const card = side.card}
	{@const name = shownName(key, ref)}
	{@const segments = meta(card)}
	{@const aliases = card?.aliases?.slice(0, 3) ?? []}
	{@const leadImage = side.gallery.find((img) => img.id === lead[key])}
	<section
		aria-label={name}
		class="min-w-0 rounded-theme border border-rule bg-surface p-3"
	>
		{#if card}
			<!-- Keyed, so a swap is a fresh frame: an error on one image never hides the next. -->
			{#key lead[key]}
				<PersonImageFrame
					personId={ref.id}
					role={leadImage?.role ?? 'headshot'}
					imageId={leadImage?.id}
					{name}
					version={leadImage ? leadImage.version : card.headshot_version}
					frameClass="portrait-frame--1x1 w-60"
					sizes="240px"
					eager
				/>
			{/key}
			<!-- No strip for a person with one image: the lead already shows it (P0-3).
			     5 × 44 + 4 × 4 = 236 px, so the strip spans the 240 px lead. -->
			{#if side.gallery.length}
				<div class="mt-1 flex gap-1">
					{@render frame(key, ref, name, 'headshot', 0)}
					{#each side.gallery as img, i (img.id)}
						{@render frame(key, ref, name, img.id, i + 1, img)}
					{/each}
				</div>
			{/if}
		{:else}
			<!-- Loading holds the lead and all five strip slots, so nothing shifts when the
			     images land (asking for the headshot before its version is known would fetch
			     it twice). A FAILED side holds the lead only: nothing is ever going to land
			     there, and five empty wells claim five images this person may not have —
			     the same overstatement P0-3 rules out for a person with no images. -->
			<span class="portrait-frame portrait-frame--1x1 block w-60" aria-hidden="true"></span>
			{#if !side.failed}
				<div class="mt-1 flex gap-1">
					{#each LOADING_SLOTS as slot (slot)}
						<span class="portrait-frame portrait-frame--1x1 w-11 shrink-0" aria-hidden="true"></span>
					{/each}
				</div>
			{/if}
		{/if}

		<span class="mt-2 flex items-center gap-1.5">
			<a
				href={profileHref('person', ref.id)}
				target="_blank"
				rel="noopener"
				aria-label={`Open ${name} in a new tab`}
				class="skin-title truncate font-display text-sm font-semibold text-ink hover:text-accent">{name}</a
			>
			<span aria-hidden="true" class="shrink-0 text-xs text-muted">↗</span>
			{#if card?.nationality?.length}
				<NationalityFlags values={card.nationality} />
			{/if}
		</span>
		{#if segments.length}
			<span class="block text-xs text-muted">{segments.join(' · ')}</span>
		{/if}
		{#if aliases.length}
			<span class="block truncate text-xs italic text-muted">also credited as {aliases.join(', ')}</span>
		{:else if card}
			<!-- Says so in words (P0-10's rule, applied to both panels), so the columns line up. -->
			<span class="block text-xs text-muted">No other names</span>
		{/if}

		{#if side.failed}
			<p class="mt-2 text-xs text-warn" role="alert">Couldn't load this side.</p>
			<button type="button" onclick={() => load(key, ref)} class="btn-row btn-quiet mt-1">
				Retry
			</button>
		{:else if card}
			<!-- Every link here opens a new tab (P0-9), so the queue page never moves. -->
			<div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 border-t border-rule pt-2 text-xs">
				{@render newTab(`/people/${ref.id}#videos`, 'Videos')}
				{#if card.film_count > 0}
					{@render newTab(`/people/${ref.id}#films`, 'Films')}
				{/if}
				{#each sortExternalLinks(card.external_links ?? []) as link (link.provider)}
					<span class="inline-flex items-center gap-0.5">
						<ProviderLinkBadge {link} entityName={name} />
						{#if link.url}<span aria-hidden="true" class="text-muted">↗</span>{/if}
					</span>
				{/each}
			</div>
		{/if}
		<div class="mt-2 border-t border-rule pt-2">
			{@render mergeInto(ref, other)}
		</div>
	</section>
{/snippet}

<!-- One strip frame: a button that puts its image in the lead. The ring is an overlay,
     because an inset ring on the button itself would sit under the image. -->
{#snippet frame(key: 'a' | 'b', ref: EntityRef, name: string, id: LeadKey, n: number, img?: PersonImage)}
	{@const chosen = lead[key] === id}
	<button
		type="button"
		onclick={() => (lead[key] = id)}
		aria-pressed={chosen}
		aria-label={`Show image ${n + 1} of ${name}`}
		class="group relative w-11 shrink-0 rounded-theme"
	>
		<!-- Same person, five times: the button carries the label, so the image is decorative. -->
		<PersonImageFrame
			personId={ref.id}
			role={img?.role ?? 'headshot'}
			imageId={img?.id}
			{name}
			alt=""
			version={img ? img.version : sides[key].card?.headshot_version}
			frameClass="portrait-frame--1x1 w-11"
			sizes="44px"
		/>
		<span
			aria-hidden="true"
			class="pointer-events-none absolute inset-0 rounded-theme {chosen
				? 'ring-2 ring-inset ring-accent'
				: 'group-hover:ring-1 group-hover:ring-inset group-hover:ring-rule group-focus-visible:ring-1 group-focus-visible:ring-inset group-focus-visible:ring-rule'}"
		></span>
	</button>
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
		{@render column('a', pair.a, pair.b)}
		{@render column('b', pair.b, pair.a)}
	</div>
	<div class="mt-3 flex flex-wrap items-center justify-end gap-2 border-t border-rule pt-3">
		{@render verdicts()}
	</div>
</div>
