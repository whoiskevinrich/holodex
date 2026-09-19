<script lang="ts">
	// Completeness ring badge (F65.4, HOLODEX-412, design handoff § Ring component):
	// `required` fills the accent ring; once required is 100, `extras` draws as a
	// second lap in ink over it (the "overfill", RD6). Owner-only by payload, not by
	// prop — the caller mounts this iff the item carries `completeness`; the API
	// strips the field for visitors (F65.5). Static arcs, no motion, not interactive.
	import type { CompletenessSummary } from '$lib/types';
	import { arc, ringReading, RING_CIRCUMFERENCE } from './ring';

	let {
		required,
		extras,
		size = 'card'
	}: CompletenessSummary & { size?: 'card' | 'row' } = $props();

	const reading = $derived(ringReading({ required, extras }));
</script>

{#if !reading.empty}
	<span class="completeness-ring inline-flex shrink-0" role="img" aria-label={reading.label}>
		<svg viewBox="0 0 20 20" class={size === 'card' ? 'h-3.5 w-3.5' : 'h-3 w-3'} aria-hidden="true">
			<!-- Track is `muted`, not `rule`: on the card the ring sits on a bg-black/70 chip
			     over a poster, where every skin's rule is too close to the chip to read. -->
			<circle cx="10" cy="10" r="7" class="fill-none stroke-muted" stroke-width="3" />
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
		</svg>
	</span>
{/if}
