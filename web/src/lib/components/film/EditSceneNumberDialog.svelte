<script lang="ts">
	// Edit scene number (HOLODEX-326): corrects an already-attached video's scene
	// number in place, without a detach+reattach round trip. Reuses ConfirmDialog's
	// chrome (focus trap, Esc/backdrop cancel) with a single number input as the body --
	// a collision surfaces as a plain error string, the same convention
	// FilmAttachDialog's scene-number step already uses, not an interactive verdict
	// card (unlike NameEditControl's name collisions, which offer a merge). Shared by
	// the film detail page's scenes grid and the media detail page's Films chip row --
	// the only difference between the two call sites is which name `contextLabel` shows.
	import ConfirmDialog from '$lib/components/shared/ConfirmDialog.svelte';
	import type { FilmSceneCollision } from '$lib/types';
	import { parseSceneNumberInput } from './sceneNumber';

	let {
		contextLabel,
		currentScene,
		onsave,
		onclose
	}: {
		// What's being renumbered, shown under the input for context: the video's title on
		// the film page, or the film's name on the media page -- whichever name the caller
		// isn't already showing elsewhere in the same UI.
		contextLabel: string;
		currentScene: number | null;
		onsave: (value: number | null) => Promise<{ ok: true } | { conflict: FilmSceneCollision }>;
		onclose: () => void;
	} = $props();

	// bind:value on type="number" coerces to a Number (or '' when cleared) --
	// parseSceneNumberInput knows not to call string methods on this value.
	let value = $state(currentScene ?? '');
	let busy = $state(false);
	let error = $state('');

	async function save() {
		const parsed = parseSceneNumberInput(value);
		if ('error' in parsed) {
			error = parsed.error;
			return;
		}
		const next = parsed.value;
		busy = true;
		error = '';
		try {
			const res = await onsave(next);
			if ('conflict' in res) {
				error = `Scene ${next} is already "${res.conflict.video_title}".`;
				return;
			}
			onclose();
		} finally {
			busy = false;
		}
	}
</script>

<ConfirmDialog
	title="Edit scene number"
	confirmLabel="Save"
	variant="accent"
	{busy}
	{error}
	onconfirm={save}
	oncancel={onclose}
>
	{#snippet body()}
		<label class="block space-y-1.5">
			<span class="text-xs uppercase tracking-wide text-muted">{contextLabel}</span>
			<input
				bind:value
				type="number"
				min="1"
				step="1"
				placeholder="Unnumbered"
				class="w-full rounded-theme border border-rule bg-surface px-3 py-1.5 text-ink focus:border-accent focus:outline-none"
			/>
		</label>
		<p class="text-xs text-muted">Leave blank for unnumbered.</p>
	{/snippet}
</ConfirmDialog>
