<script lang="ts">
	// Provenance chip (F22.7): labels where a field value came from. Tokens only —
	// "from <provider>" uses an outlined accent (noteworthy, not alarming), the
	// file baseline a muted pill. Never --warn (reserved for errors). The long form
	// goes to aria-label so screen readers don't just hear the bare provider name.
	let { provider = '', label = '' }: { provider?: string; label?: string } = $props();

	const text = $derived(provider ? `from ${label || provider}` : 'from file');
	const isProvider = $derived(!!provider);
</script>

<span
	class="ml-2 inline-block rounded-full px-2 py-0.5 align-middle text-xs {isProvider
		? 'border border-accent text-accent'
		: 'bg-surface-2 text-muted'}"
	aria-label={`source: ${text}`}
>
	{text}
</span>
