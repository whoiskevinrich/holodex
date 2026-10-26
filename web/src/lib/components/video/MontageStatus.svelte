<script lang="ts">
	// The owner's "Hover montage" row in the media page's Maintenance section (F83, handoff
	// "Media page: the Maintenance rows"). Owner-only: the caller mounts it inside the
	// owner-only section, and it hides itself when generation is off. In manual mode this is how the owner tests
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
	<div class="flex flex-wrap items-start gap-x-3 gap-y-2">
		<span class="w-28 shrink-0 pt-1.5 text-sm text-muted">Hover montage</span>
		<div class="min-w-0 flex-1 space-y-2">
		<div class="flex flex-wrap items-center gap-2 text-sm">
			<span class="text-ink" aria-live="polite">
				{view.value}{#if view.detail}<span class="text-muted" title={view.detail}
						>{' · '}<span class="inline-block max-w-[32ch] truncate align-bottom">{view.detail}</span></span
					>{/if}
			</span>
			{#each view.actions as a (a)}
				<button
					type="button"
					class="btn-ghost rounded-theme px-3 py-1.5 text-sm"
					disabled={busy}
					aria-pressed={a === 'preview' ? previewing : undefined}
					onclick={() => act(a)}
				>
					{montageActionLabel[a]}
				</button>
			{/each}
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
	</div>
{:else if failedToLoad}
	<p class="text-sm text-warn">Couldn't load the montage status.</p>
{/if}
