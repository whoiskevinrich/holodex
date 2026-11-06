<script lang="ts">
	// The 12 px stroke padlock, CoreImageEditButton's glyph; `open` swings the shackle free.
	let { open = false, class: klass = 'h-3 w-3' }: { open?: boolean; class?: string } = $props();
</script>

<svg class={klass} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true" data-lock={open ? 'open' : 'closed'}>
	{#if open}
		<path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zM8 11V7a4 4 0 017.75-1.4" />
	{:else}
		<path stroke-linecap="round" stroke-linejoin="round" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
	{/if}
</svg>
