<script lang="ts">
	// The thumbnail frame picker (F84, docs/design/thumbnail-frame-picker-handoff.md): scrub a
	// muted, never-playing copy of the video, compare with the current thumbnail, and keep the
	// frame on screen. The frame is captured from the decoded video, not re-cut by timestamp
	// (spec "Rationale"), so what is saved is exactly what was shown. Tokens only; QA Cinémathèque.
	import { onMount } from 'svelte';
	import Modal from '#lib/components/shared/Modal.svelte';
	import { formatDuration, toMessage } from '#lib/format.ts';
	import { FALLBACK_FRAME, WheelStepper, frameInterval, stepTarget, timecode } from './framePicker.ts';

	let {
		src,
		currentSrc,
		startAt = 0,
		onuse,
		onreset,
		onclose
	}: {
		src: string;
		/** The current thumbnail; absent when the video has none yet. */
		currentSrc?: string;
		/** Seconds to open at — the page player's position. */
		startAt?: number;
		/** Saves the captured frame. Throws with a message on failure. */
		onuse: (file: File) => Promise<void>;
		onreset: () => void;
		onclose: () => void;
	} = $props();

	let videoEl = $state<HTMLVideoElement | null>(null);
	let scrubArea = $state<HTMLElement | null>(null);
	let bar = $state<HTMLElement | null>(null);
	let panel = $state<HTMLElement | null>(null);

	let duration = $state(0);
	let interval = $state(FALLBACK_FRAME);
	// Start time of the frame aimed at: the presented frame's media time once a seek settles.
	let aim = $state(0);
	let ready = $state(false);
	let decodeFailed = $state(false);
	let saving = $state(false);
	let error = $state('');

	const canRVFC = typeof HTMLVideoElement !== 'undefined' && 'requestVideoFrameCallback' in HTMLVideoElement.prototype;
	let lastPresented = 0;
	let presented = 0; // frames reported by watchFrames; 0 means the browser isn't reporting any
	let onPresent: (() => void) | null = null; // seekAndRead's early finish
	let alive = true;
	const wheel = new WheelStepper();

	function seek(t: number) {
		if (!videoEl || !duration) return;
		aim = Math.min(Math.max(t, 0), duration);
		videoEl.currentTime = aim;
	}

	function step(dir: 1 | -1) {
		if (!videoEl || !duration || saving) return;
		const t = stepTarget(aim, interval, dir, duration);
		videoEl.currentTime = t;
		aim = Math.max(0, t - interval / 2);
	}

	// Track the presented frame. A paused video presents a frame only after a seek, so this loop
	// is idle while the owner isn't scrubbing.
	function watchFrames(v: HTMLVideoElement) {
		const cb: VideoFrameRequestCallback = (_now, meta) => {
			lastPresented = meta.mediaTime;
			presented++;
			if (!v.seeking) aim = meta.mediaTime;
			onPresent?.();
			if (alive) v.requestVideoFrameCallback(cb);
		};
		v.requestVideoFrameCallback(cb);
	}

	// A settled seek snaps `aim` back to a frame start. A seek inside the presented frame brings
	// no new report, and a raw mid-frame `aim` would make the next step skip a frame. Without
	// any reports, the start is estimated from the fallback interval.
	function onSeeked() {
		if (!videoEl || videoEl.seeking) return;
		aim = presented > 0 ? lastPresented : Math.floor(videoEl.currentTime / interval + 1e-6) * interval;
	}

	// Seek and resolve with the presented frame's media time: as soon as a new frame is reported
	// after the seek, else from watchFrames' record shortly after it settles (a seek inside the
	// same frame presents nothing new, so the record is already right). Null when the browser
	// has reported no frame at all (a page that isn't rendering).
	function seekAndRead(v: HTMLVideoElement, t: number): Promise<number | null> {
		return new Promise((resolve) => {
			const before = presented;
			let done = false;
			const finish = () => {
				if (done) return;
				done = true;
				onPresent = null;
				resolve(presented > 0 ? lastPresented : null);
			};
			onPresent = () => {
				if (presented > before && !v.seeking) finish();
			};
			v.addEventListener('seeked', () => setTimeout(finish, 200), { once: true });
			setTimeout(finish, 1500); // a seek that never settles
			v.currentTime = t;
		});
	}

	// Spec P0-6: one frame is the real interval between two presented frames, probed in
	// 1/120 s steps from the opening frame.
	async function measure(v: HTMLVideoElement, from: number) {
		const first = await seekAndRead(v, from);
		if (first === null) {
			onSeeked();
			return;
		}
		for (let k = 1; k <= 8 && alive; k++) {
			const next = await seekAndRead(v, first + k / 120);
			if (next !== null && next > first) {
				interval = frameInterval(first, next);
				break;
			}
		}
		if (!alive) return;
		await seekAndRead(v, first + interval / 2);
		aim = first;
	}

	async function onLoadedMetadata() {
		if (!videoEl) return;
		duration = videoEl.duration || 0;
		const start = Math.min(Math.max(startAt, 0), duration);
		if (canRVFC) {
			watchFrames(videoEl);
			await measure(videoEl, start);
		} else seek(start);
		ready = true;
	}

	function settled(v: HTMLVideoElement): Promise<void> {
		return v.seeking ? new Promise((r) => v.addEventListener('seeked', () => r(), { once: true })) : Promise.resolve();
	}

	async function capture(v: HTMLVideoElement): Promise<File> {
		await settled(v);
		const canvas = document.createElement('canvas');
		canvas.width = v.videoWidth;
		canvas.height = v.videoHeight;
		const ctx = canvas.getContext('2d');
		if (!ctx) throw new Error("This browser can't capture the frame.");
		ctx.drawImage(v, 0, 0, canvas.width, canvas.height);
		const blob = await new Promise<Blob | null>((r) => canvas.toBlob(r, 'image/jpeg', 0.92));
		if (!blob) throw new Error("This browser can't capture the frame.");
		return new File([blob], 'frame.jpg', { type: 'image/jpeg' });
	}

	async function useFrame() {
		if (!videoEl || !ready || saving) return;
		saving = true;
		error = '';
		try {
			await onuse(await capture(videoEl));
		} catch (e) {
			error = toMessage(e);
		} finally {
			saving = false;
		}
	}

	function seekToPointer(e: PointerEvent) {
		if (!bar) return;
		const r = bar.getBoundingClientRect();
		seek(((e.clientX - r.left) / r.width) * duration);
	}

	function onBarDown(e: PointerEvent) {
		if (!ready || saving) return;
		bar?.setPointerCapture(e.pointerId);
		seekToPointer(e);
	}

	function onBarMove(e: PointerEvent) {
		if (bar?.hasPointerCapture(e.pointerId)) seekToPointer(e);
	}

	// ← → step anywhere in the dialog except on a button (handoff, "Scrub bar").
	function onKeydown(e: KeyboardEvent) {
		if (!ready || e.target instanceof HTMLButtonElement) return;
		if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
			e.preventDefault();
			step(e.key === 'ArrowRight' ? 1 : -1);
		}
	}

	function onBarKeydown(e: KeyboardEvent) {
		if (!ready) return;
		if (e.key === 'Home') seek(0);
		else if (e.key === 'End') seek(Math.max(0, duration - interval / 2));
		else return;
		e.preventDefault();
	}

	onMount(() => {
		const v = videoEl;
		const area = scrubArea;
		const p = panel;
		if (v) v.volume = 0;
		// Non-passive so the wheel steps frames instead of scrolling the page (spec P0-5).
		const onWheel = (e: WheelEvent) => {
			e.preventDefault();
			if (!ready) return;
			const n = wheel.push(e);
			if (n) step(n > 0 ? 1 : -1);
		};
		area?.addEventListener('wheel', onWheel, { passive: false });
		// On the panel itself: clicking the frame focuses the panel, and keys must still step.
		p?.addEventListener('keydown', onKeydown);
		return () => {
			alive = false;
			area?.removeEventListener('wheel', onWheel);
			p?.removeEventListener('keydown', onKeydown);
			// Stop the detached element buffering the stream.
			v?.removeAttribute('src');
			v?.load();
		};
	});

	const pct = $derived(duration ? (aim / duration) * 100 : 0);
	const position = $derived(timecode(aim, interval));
	const canUse = $derived(ready && !decodeFailed && !saving);
</script>

<Modal titleId="frame-picker-title" onclose={onclose} size="media" align="center" busy={saving} bind:panel>
	<div class="flex min-h-0 flex-col gap-4 overflow-y-auto p-5">
		<h2 id="frame-picker-title" class="text-lg text-ink">Choose a thumbnail frame</h2>

		<div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_13.5rem]">
			<div class="min-w-0" bind:this={scrubArea}>
				<p class="mb-1.5 text-xs uppercase tracking-wide text-muted">Selected frame</p>
				<div class="relative aspect-video border border-rule bg-black">
					<!-- Silent by construction (spec P0-7): muted, volume 0, no controls, never played. -->
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						bind:this={videoEl}
						{src}
						muted
						preload="auto"
						playsinline
						class="h-full w-full object-contain"
						onloadedmetadata={onLoadedMetadata}
						onseeked={onSeeked}
						onerror={() => (decodeFailed = true)}
					></video>
					{#if decodeFailed}
						<p class="absolute inset-0 flex items-center justify-center bg-surface p-4 text-center text-sm text-muted">
							This browser can't decode this file's codec.
						</p>
					{:else if !ready}
						<p class="absolute inset-0 flex items-center justify-center text-sm text-muted">Loading video…</p>
					{/if}
				</div>

				<div
					bind:this={bar}
					role="slider"
					tabindex="0"
					aria-label="Frame position"
					aria-valuemin={0}
					aria-valuemax={Math.round(duration * 1000) / 1000}
					aria-valuenow={Math.round(aim * 1000) / 1000}
					aria-valuetext={position}
					aria-disabled={!ready || saving}
					class="relative mt-4 h-1.5 cursor-pointer touch-none rounded-full bg-rule focus:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-4 focus-visible:ring-offset-surface"
					onpointerdown={onBarDown}
					onpointermove={onBarMove}
					onkeydown={onBarKeydown}
				>
					<div class="absolute inset-y-0 left-0 rounded-full bg-accent" style:width="{pct}%"></div>
					<div
						class="absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-accent ring-2 ring-ink"
						style:left="{pct}%"
					></div>
				</div>
				<div class="mt-2 flex justify-between text-xs tabular-nums text-muted">
					<span>{position}</span>
					<span>{formatDuration(duration)}</span>
				</div>
				<p class="mt-1 text-xs text-muted">Drag to scrub · ← → or the mouse wheel step one frame</p>
			</div>

			<div>
				<p class="mb-1.5 text-xs uppercase tracking-wide text-muted">Current</p>
				<div class="flex aspect-video items-center justify-center border border-rule bg-black">
					{#if currentSrc}
						<img src={currentSrc} alt="Current thumbnail" class="h-full w-full object-contain" />
					{:else}
						<span class="text-xs text-muted">No thumbnail yet</span>
					{/if}
				</div>
			</div>
		</div>

		{#if error}
			<p class="text-sm text-warn" role="alert">{error}</p>
		{/if}

		<div class="flex flex-wrap items-center justify-between gap-2">
			<button type="button" class="btn-quiet px-3 py-1.5 text-sm" disabled={saving} onclick={onreset}>
				Reset to automatic
			</button>
			<div class="flex gap-2">
				<button type="button" class="btn-ghost rounded-theme px-3 py-1.5 text-sm" disabled={saving} onclick={onclose}>
					Cancel
				</button>
				<!-- Withdrawn (ghost look), never dimmed, while there's no frame to use. -->
				<button
					type="button"
					class="rounded-theme px-3 py-1.5 text-sm {canUse
						? 'bg-accent font-medium text-accent-ink'
						: 'btn-ghost cursor-not-allowed'}"
					disabled={!canUse}
					onclick={useFrame}
				>
					{saving ? 'Saving…' : 'Use this frame'}
				</button>
			</div>
		</div>
	</div>
</Modal>
