<script lang="ts">
	// The owner's hover-montage status line under the player (F83, handoff "Media page:
	// the owner's status line"). Owner-only: the caller mounts it only for the owner, and
	// it hides itself when generation is off. In manual mode this is how the owner tests
	// montages in production, one video at a time.
	import { api } from '#lib/api.ts';
	import type { MontageStatus } from '#lib/types.ts';
	import { MONTAGE_POLL_MS, montageActionLabel, montageStatusView, shouldPollMontage, type MontageAction } from './montageStatus';

	let { videoId }: { videoId: number } = $props();

	let status = $state<MontageStatus | null>(null);
	let busy = $state(false);
	let failedToLoad = $state(false);
	let previewing = $state(false);
	let previewVersion = $state(0);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let seq = 0; // only the newest load may schedule the next poll

	const reducedMotion =
		typeof window !== 'undefined' && !!window.matchMedia?.('(prefers-reduced-motion: reduce)').matches;
	const view = $derived(status ? montageStatusView(status) : null);

	async function load(id: number) {
		clearTimeout(timer);
		const mine = ++seq;
		try {
			const next = await api.montageStatus(id);
			if (mine !== seq) return; // superseded by a newer load, or unmounted
			if (status?.state !== 'ready' && next.state === 'ready') previewVersion++;
			status = next;
			failedToLoad = false;
			if (shouldPollMontage(next)) timer = setTimeout(() => load(id), MONTAGE_POLL_MS);
		} catch {
			if (mine === seq) failedToLoad = true;
		}
	}

	$effect(() => {
		const id = videoId;
		status = null;
		previewing = false;
		load(id);
		return () => {
			seq++; // an in-flight load must not schedule a poll after teardown
			clearTimeout(timer);
		};
	});

	async function act(a: MontageAction) {
		if (a === 'preview') {
			previewing = !previewing;
			return;
		}
		busy = true;
		try {
			await api.requestMontage(videoId);
			previewing = false;
			await load(videoId);
		} catch {
			failedToLoad = true;
		} finally {
			busy = false;
		}
	}
</script>

{#if status && view && status.mode !== 'off'}
	<div class="space-y-2">
		<div class="flex flex-wrap items-center gap-2 text-xs">
			<span class="text-muted">Montage</span>
			<span class="min-w-0 flex-1 text-ink" aria-live="polite">
				{view.value}{#if view.detail}<span class="text-muted" title={view.detail}
						>{' · '}<span class="inline-block max-w-[32ch] truncate align-bottom">{view.detail}</span></span
					>{/if}
			</span>
			{#if view.actions.length}
				<span class="flex items-center gap-2">
					{#each view.actions as a (a)}
						<button
							type="button"
							class="btn-quiet rounded-theme px-2 py-0.5 text-xs"
							disabled={busy}
							aria-pressed={a === 'preview' ? previewing : undefined}
							onclick={() => act(a)}
						>
							{montageActionLabel[a]}
						</button>
					{/each}
				</span>
			{/if}
		</div>
		{#if previewing && status.state === 'ready'}
			<!-- Card-size preview: the same silent attributes as the grid card's layer. Under
			     reduced motion it waits paused on its first frame with controls. -->
			<!-- svelte-ignore a11y_media_has_caption -->
			<video
				src={api.montageURL(videoId, previewVersion)}
				muted
				playsinline
				loop
				autoplay={!reducedMotion}
				controls={reducedMotion}
				disablepictureinpicture
				class="w-60 rounded-theme border border-rule bg-black"
			></video>
		{/if}
	</div>
{:else if failedToLoad}
	<p class="text-xs text-warn">Couldn't load the montage status.</p>
{/if}
