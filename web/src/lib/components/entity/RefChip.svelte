<script lang="ts">
	// Copyable `kind:id` reference handle (F60 RD1 / HOLODEX-374, handoff §1): the last item
	// of an entity page's meta line, visible to visitors — a ref is not a mutation, and a
	// visitor pasting `film:42` into a bug report is the point. The string comes from the
	// API payload's `ref`; the chip never parses or assembles it. Click / Enter / Space
	// copies and shows "Copied" for 1.5 s. If the clipboard API rejects (insecure context,
	// permissions), the text is selected in place so the user can copy it themselves — the
	// chip must never silently do nothing.
	let { ref }: { ref: string } = $props();

	let copied = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let label: HTMLElement | undefined;

	async function copy() {
		try {
			await navigator.clipboard.writeText(ref);
			copied = true;
			clearTimeout(timer);
			timer = setTimeout(() => (copied = false), 1500);
		} catch {
			if (!label) return;
			const range = document.createRange();
			range.selectNodeContents(label);
			const selection = window.getSelection();
			selection?.removeAllRanges();
			selection?.addRange(range);
		}
	}
</script>

<button
	type="button"
	class="btn-row btn-pill gap-1 border border-rule font-ui text-ink transition-colors duration-150 hover:ring-1 hover:ring-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
	aria-label="Copy reference {ref}"
	onclick={copy}
>
	<span bind:this={label} class="select-text font-mono">{copied ? 'Copied' : ref}</span>
	{#if copied}
		<svg class="h-3 w-3 text-muted" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
			<path d="M3 8.5l3 3 7-7" stroke-linecap="round" stroke-linejoin="round" />
		</svg>
	{:else}
		<svg class="h-3 w-3 text-muted" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" aria-hidden="true">
			<rect x="5.5" y="5.5" width="8" height="8" rx="1.5" />
			<path d="M10.5 5.5v-2a1 1 0 0 0-1-1h-6a1 1 0 0 0-1 1v6a1 1 0 0 0 1 1h2" />
		</svg>
	{/if}
</button>
<span class="sr-only" aria-live="polite">{copied ? `Copied ${ref}` : ''}</span>
