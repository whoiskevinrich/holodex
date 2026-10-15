<script lang="ts">
	// Test fixture for Modal.svelte.test.ts: an opener, a modal A that can open a stacked
	// modal B, and a window-level Escape listener standing in for a page's own handler.
	import Modal from '../Modal.svelte';

	let { busy = false, onpageescape = () => {} }: { busy?: boolean; onpageescape?: () => void } = $props();

	let a = $state(false);
	let b = $state(false);
</script>

<svelte:window onkeydown={(e) => e.key === 'Escape' && onpageescape()} />

<button onclick={() => (a = true)}>Open A</button>

{#if a}
	<Modal titleId="a-title" onclose={() => (a = false)} {busy}>
		<div class="p-4">
			<h2 id="a-title">Modal A</h2>
			<input aria-label="A field" />
			<input aria-label="Claiming field" tabindex="-1" onkeydown={(e) => e.key === 'Escape' && e.preventDefault()} />
			<button tabindex="-1">Roving row</button>
			<button disabled>Disabled</button>
			<button onclick={() => (b = true)}>Open B</button>
		</div>
	</Modal>
{/if}

{#if b}
	<Modal titleId="b-title" onclose={() => (b = false)} size="media" align="center">
		<h2 id="b-title">Modal B</h2>
		<button onclick={() => (b = false)}>Close B</button>
	</Modal>
{/if}
