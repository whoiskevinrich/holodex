<script lang="ts">
	// The name lock toggle (handoff call 6). A labelled button's glyph shows what the click does
	// (Lock name: closed, Unlock: open); the icon-only toggle shows the current state, with
	// aria-pressed. Locks and unlocks at once, with no confirm.
	import { api } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import Padlock from './Padlock.svelte';

	let {
		videoId,
		title,
		locked,
		labelled = false,
		onchange
	}: {
		videoId: number;
		title: string;
		locked: boolean;
		labelled?: boolean;
		onchange?: (locked: boolean) => void;
	} = $props();

	let busy = $state(false);
	let error = $state('');

	async function toggle() {
		if (busy) return;
		busy = true;
		error = '';
		try {
			const res = await api.setNameLock(videoId, !locked);
			onchange?.(res.locked);
		} catch (e) {
			error = toMessage(e);
		} finally {
			busy = false;
		}
	}
</script>

{#if labelled}
	<button
		type="button"
		class="btn-quiet inline-flex items-center gap-1.5 rounded-theme px-2 py-1 text-sm"
		aria-label={locked ? `Unlock name for ${title}` : `Lock name for ${title}`}
		aria-busy={busy}
		onclick={toggle}
	>
		<Padlock open={locked} />
		{locked ? 'Unlock' : 'Lock name'}
	</button>
{:else}
	<button
		type="button"
		class="btn-row btn-quiet rounded-theme px-1.5"
		aria-pressed={locked}
		aria-label={locked ? `Unlock name for ${title}` : `Lock name for ${title}`}
		title={locked ? 'Unlock name' : 'Lock name'}
		aria-busy={busy}
		onclick={toggle}
	>
		<Padlock open={!locked} />
	</button>
{/if}
{#if error}
	<span class="text-xs text-warn" role="alert">{error}</span>
{/if}
