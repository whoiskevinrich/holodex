<script lang="ts">
	// Completeness ring badge (F65.4, HOLODEX-412, design handoff § Ring component):
	// `required` fills the accent ring; once required is 100, `extras` draws as a
	// second lap in ink over it (the "overfill", RD6). Owner-only by payload, not by
	// prop — the caller mounts this iff the item carries `completeness`; the API
	// strips the field for visitors (F65.5).
	//
	// F65.8 (HOLODEX-435): the ring is a <button>. Pressing it fires the same
	// per-entity step the F66 sweep runs — POST …/enrich/refresh-all, every
	// provider that supports the kind, Force — with sweep semantics: fire and
	// forget, no picker on needs_review, no toast on rate_limited. While the
	// request is in flight the ring spins a quarter arc; when it lands the ring
	// re-reads its own bands (GET …/{id}/completeness, which drains the F65 store)
	// and redraws, so the list need not re-fetch. Because it is a button it must
	// never sit inside a card's or row's <a> — every mount hoists it as a sibling.
	import type { CompletenessSummary, EnrichEntityKind } from '$lib/types';
	import { api } from '$lib/api';
	import { arc, bandsAfterRefresh, ringButtonLabel, ringReading, RING_BUSY_ARC, RING_CIRCUMFERENCE } from './ring';
	import { currentBands, ringKey, ringRefresh } from './ringRefresh.svelte';

	let {
		required,
		extras,
		size = 'card',
		entity,
		onrefreshed
	}: CompletenessSummary & {
		size?: 'card' | 'row';
		/** Which entity a press refreshes — the same kind/id the list item carries. */
		entity: { kind: EnrichEntityKind; id: number };
		/** Fired after the refresh and the re-read settle, with the bands now drawn. */
		onrefreshed?: (bands: CompletenessSummary) => void;
	} = $props();

	// Busy and the post-refresh bands are shared per entity (ringRefresh.svelte.ts):
	// a page can mount the same entity twice, and every ring for it must agree.
	// The list's bands stand until a refresh lands fresher ones; a later list
	// fetch with different bands wins again (currentBands).
	const key = $derived(ringKey(entity));
	const shared = $derived(ringRefresh[key]);
	const busy = $derived(shared?.busy ?? false);
	const bands = $derived(currentBands(shared, { required, extras }));
	const reading = $derived(ringReading(bands));

	async function refresh(e: MouseEvent) {
		// Never the card/row link's click, even if a mount forgets to hoist.
		e.preventDefault();
		e.stopPropagation();
		if (busy) return;
		const over = { required, extras };
		ringRefresh[key] = { busy: true };
		let next: CompletenessSummary | null = null;
		try {
			await api.enrichRefreshAll(entity.kind, entity.id);
		} catch {
			// Sweep semantics: an unattended refresh reports nothing; the next
			// press is the retry, and the entity page has the detail.
		}
		try {
			next = await api.entityCompleteness(entity.kind, entity.id);
		} catch {
			// A failed re-read never blanks the ring (bandsAfterRefresh).
		}
		ringRefresh[key] = { busy: false, over, bands: bandsAfterRefresh(over, next) };
		onrefreshed?.(bands);
	}
</script>

{#if !reading.empty}
	<button
		type="button"
		class="completeness-ring inline-flex shrink-0 cursor-pointer rounded-full focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent disabled:cursor-progress"
		aria-label={ringButtonLabel(reading, busy)}
		title={busy ? 'Refreshing enrichment…' : 'Refresh enrichment'}
		aria-busy={busy}
		disabled={busy}
		onclick={refresh}
	>
		<svg viewBox="0 0 20 20" class={size === 'card' ? 'h-3.5 w-3.5' : 'h-3 w-3'} aria-hidden="true">
			<!-- Track is `muted`, not `rule`: on the card the ring sits on a bg-black/70 chip
			     over a poster, where every skin's rule is too close to the chip to read. -->
			<circle cx="10" cy="10" r="7" class="fill-none stroke-muted" stroke-width="3" />
			{#if busy}
				<!-- A fixed quarter lap, spun by app.css (reduced-motion: held still and dimmed). -->
				<circle
					cx="10"
					cy="10"
					r="7"
					class="ring-busy fill-none stroke-accent"
					stroke-width="3"
					stroke-dasharray="{RING_BUSY_ARC} {RING_CIRCUMFERENCE}"
					transform="rotate(-90 10 10)"
				/>
			{:else}
				<circle
					cx="10"
					cy="10"
					r="7"
					class="ring-required fill-none stroke-accent"
					stroke-width="3"
					stroke-dasharray="{arc(reading.ring)} {RING_CIRCUMFERENCE}"
					transform="rotate(-90 10 10)"
				/>
				{#if reading.overfill > 0}
					<circle
						cx="10"
						cy="10"
						r="7"
						class="ring-overfill fill-none stroke-ink"
						stroke-width="3"
						stroke-dasharray="{arc(reading.overfill)} {RING_CIRCUMFERENCE}"
						transform="rotate(-90 10 10)"
					/>
				{/if}
			{/if}
		</svg>
	</button>
{/if}
