<script module lang="ts">
	// Static per-state tone, shared across every chip instance (allocated once, not
	// once per mount). 'unreviewed' + reviewed=true overrides to text-ink below.
	const TONE: Record<'unreviewed' | 'auto_applied' | 'not_matched', string> = {
		unreviewed: 'text-muted',
		auto_applied: 'text-accent',
		not_matched: 'text-muted'
	};
</script>

<script lang="ts">
	// Read-only queue-row sibling of EnrichProviderChips (F47 S2, ADR-065). Same chip
	// shell (icon + name), but the *row's* action drives resolution, not the chip
	// itself — so this carries no button/menu, just a state label. Tokens only.
	import ProviderIcon from './ProviderIcon.svelte';
	import { providers as providerDir } from '$lib/providers.svelte';
	import type { EnrichQueueProviderState } from '$lib/types';

	let {
		provider,
		state,
		reviewed = false
	}: {
		provider: string;
		state: EnrichQueueProviderState['state'];
		/** Owner has opened this provider's picker this session — ephemeral UI state
		 *  (EnrichQueueRow's local set), not a synced domain state (design handoff
		 *  table's pre- vs post-first-click 'not yet reviewed' → 'Needs review'). */
		reviewed?: boolean;
	} = $props();

	// "needs review"/"not matched" are neutral backlog states, never text-warn — only
	// an actual request failure earns that (the F43 regression this handoff calls out).
	const label = $derived(
		state === 'unreviewed'
			? reviewed
				? 'Needs review'
				: 'not yet reviewed'
			: state === 'auto_applied'
				? '✓ Auto-applied'
				: 'Not matched'
	);
	const tone = $derived(state === 'unreviewed' && reviewed ? 'text-ink' : TONE[state]);

	$effect(() => {
		void providerDir.load();
	});
</script>

<span
	class="inline-flex items-center gap-1.5 rounded-theme border border-rule bg-surface px-2 py-1 text-xs"
>
	<ProviderIcon name={provider} iconUrl={providerDir.iconUrl(provider)} size={16} />
	<span class="font-medium text-ink">{provider}</span>
	<span class={tone}>{label}</span>
</span>
