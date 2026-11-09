<script lang="ts">
	// The trim picker (F88, docs/design/video-trimming-handoff.md): two handles on one bar set the
	// first and last frames kept on a muted, never-playing copy of the video, then a review step in
	// the same dialog takes the save choice. Each end's still is captured when its handle comes to
	// rest, and those captures go with the job for the screen check. Tokens only; QA Cinémathèque.
	import { onMount, tick } from 'svelte';
	import Modal from '#lib/components/shared/Modal.svelte';
	import { formatBytes, formatDuration, toMessage } from '#lib/format.ts';
	import { TRIM, nameRefusalCopy, refusalCopy, shortDate, type MaintenancePreflight } from '#lib/maintenance.ts';
	import { FALLBACK_FRAME, FrameReader, WheelStepper, captureFrame, settled, stepTarget, timecode } from './framePicker.ts';
	import {
		cutsNothing,
		editionFileName,
		handleBounds,
		reviewSummary,
		trimSummary,
		type TrimChoice,
		type TrimEnd,
		type TrimMode as Mode
	} from './trimPicker.ts';

	let {
		src,
		filename,
		preflight,
		onconfirm,
		onclose
	}: {
		src: string;
		filename: string;
		preflight: (t: { inTime: number; outTime: number; mode: Mode; edition: string }) => Promise<MaintenancePreflight>;
		/** Queues the trim. Resolves to the refusal if the re-run check refused; throws on failure. */
		onconfirm: (t: TrimChoice) => Promise<MaintenancePreflight | void>;
		onclose: () => void;
	} = $props();

	let videoEl = $state<HTMLVideoElement | null>(null);
	let scrubArea = $state<HTMLElement | null>(null);
	let bar = $state<HTMLElement | null>(null);
	let panel = $state<HTMLElement | null>(null);
	let reviewHeading = $state<HTMLElement | null>(null);

	let duration = $state(0);
	let interval = $state(FALLBACK_FRAME);
	let firstFrame = $state(0);
	let lastFrame = $state(0);
	let inTime = $state(0);
	let outTime = $state(0);
	let active = $state<TrimEnd>('in');
	let ready = $state(false);
	let decodeFailed = $state(false);
	let captureError = $state('');
	let resting = $state(true); // a handle moved and its frame hasn't been captured yet
	let dragging = false;
	let moves = 0; // counts handle moves, so a capture that a newer move overtook is not trusted
	let wantEnd: TrimEnd | null = null; // an end chosen while the other was still coming to rest

	let stills = $state<Record<TrimEnd, { file: File; url: string } | null>>({ in: null, out: null });

	const reader = new FrameReader(() => scheduleRest());
	const wheel = new WheelStepper();
	let alive = true;
	let restTimer: ReturnType<typeof setTimeout> | undefined;

	// Announced when a handle comes to rest, never on every step of a drag.
	let announced = $state('');

	// Step 2.
	let step = $state<1 | 2>(1);
	let mode = $state<Mode>('replace');
	let edition = $state('Trimmed');
	let pf = $state<MaintenancePreflight | null>(null);
	let checking = $state(false);
	let checkError = $state('');
	let queuing = $state(false);
	let queueError = $state('');
	let checkToken = 0;
	let checkTimer: ReturnType<typeof setTimeout> | undefined;
	let nameInput = $state<HTMLInputElement | null>(null);

	const range = $derived({ inTime, outTime, interval, duration });
	const time = (end: TrimEnd) => (end === 'in' ? inTime : outTime);
	const setTime = (end: TrimEnd, t: number) => {
		if (end === 'in') inTime = t;
		else outTime = t;
	};

	// The presented frame's start: from the browser's report, else estimated from one frame.
	const frameStart = (t: number) => Math.floor(t / interval + 1e-6) * interval;

	async function frameAt(v: HTMLVideoElement, t: number): Promise<number> {
		if (FrameReader.supported) {
			const read = await reader.read(v, t);
			if (read !== null) return read;
		} else {
			v.currentTime = t;
			await settled(v);
		}
		return frameStart(t);
	}

	async function captureEnd(end: TrimEnd) {
		if (!videoEl) return;
		const file = await captureFrame(videoEl);
		const old = stills[end];
		if (old) URL.revokeObjectURL(old.url);
		if (!alive) return;
		stills[end] = { file, url: URL.createObjectURL(file) };
	}

	// Find the file's two ends, capture them, and settle on the first frame.
	async function onLoadedMetadata() {
		const v = videoEl;
		if (!v) return;
		// Audio that decodes with video that doesn't raises no error, only a picture of no size.
		if (!v.videoWidth) {
			decodeFailed = true;
			return;
		}
		duration = v.duration || 0;
		try {
			if (FrameReader.supported) {
				reader.watch(v);
				const m = await reader.measure(v, 0);
				if (m) {
					interval = m.interval;
					firstFrame = m.first;
				}
			}
			if (!alive) return;
			lastFrame = await frameAt(v, Math.max(0, duration - interval / 2));
			outTime = lastFrame;
			await captureEnd('out');
			if (!alive) return;
			inTime = firstFrame = await frameAt(v, firstFrame + interval / 2);
			await captureEnd('in');
			ready = true;
			resting = false;
		} catch (e) {
			captureError = toMessage(e);
		}
	}

	// A handle has stopped: read which frame is really on screen, and capture it.
	function scheduleRest() {
		if (!ready) return;
		clearTimeout(restTimer);
		restTimer = setTimeout(rest, 150);
	}

	async function rest() {
		const v = videoEl;
		if (!v || !alive || !ready || dragging || v.seeking) return;
		const end = active;
		const t = reader.count > 0 ? reader.last : frameStart(v.currentTime);
		const { min, max } = handleBounds(end, range, firstFrame, lastFrame);
		// The frame on screen can sit just past the bound the seek aimed inside: go back in.
		if (t < min - interval / 2 || t > max + interval / 2) {
			seekHandle(t < min ? min : max);
			return;
		}
		setTime(end, t);
		const move = moves;
		try {
			await captureEnd(end);
		} catch (e) {
			captureError = toMessage(e);
			return;
		}
		captureError = '';
		if (move !== moves || v.seeking) return; // a newer move's own rest takes it from here
		resting = false;
		announced = trimSummary({ inTime, outTime, interval, duration }, firstFrame, lastFrame);
		if (wantEnd) {
			const next = wantEnd;
			wantEnd = null;
			activate(next);
		}
	}

	function seekHandle(t: number) {
		if (!videoEl || !duration) return;
		const { min, max } = handleBounds(active, range, firstFrame, lastFrame);
		const clamped = Math.min(Math.max(t, min), max);
		setTime(active, clamped);
		moves++;
		resting = true;
		// Mid-frame, so float error can't land on a neighbour. The last frame has no room after it.
		videoEl.currentTime = Math.min(clamped + interval / 2, duration);
		scheduleRest();
	}

	function stepActive(dir: 1 | -1) {
		if (!videoEl || !ready || step !== 1) return;
		seekHandle(stepTarget(time(active), interval, dir, duration) - interval / 2);
	}

	// The moved end must finish first: its frame and capture are only true once it has rested. An
	// end chosen before then (Tab, a click on its still) takes over as soon as it has.
	function activate(end: TrimEnd) {
		wantEnd = null;
		if (active === end) return;
		if (resting) {
			wantEnd = end;
			return;
		}
		active = end;
		if (videoEl && ready) videoEl.currentTime = Math.min(time(end) + interval / 2, duration);
	}

	function pointerTime(e: PointerEvent): number {
		if (!bar) return 0;
		const r = bar.getBoundingClientRect();
		return ((e.clientX - r.left) / r.width) * duration;
	}

	function onHandleDown(end: TrimEnd, e: PointerEvent) {
		e.stopPropagation();
		if (!ready || (resting && end !== active)) return;
		activate(end);
		dragging = true;
		bar?.setPointerCapture(e.pointerId);
	}

	// Pressing the track moves the nearer handle there.
	function onBarDown(e: PointerEvent) {
		if (!ready) return;
		const t = pointerTime(e);
		const nearer = Math.abs(t - inTime) <= Math.abs(t - outTime) ? 'in' : 'out';
		if (resting && nearer !== active) return;
		activate(nearer);
		dragging = true;
		bar?.setPointerCapture(e.pointerId);
		seekHandle(t);
	}

	function onBarMove(e: PointerEvent) {
		if (bar?.hasPointerCapture(e.pointerId)) seekHandle(pointerTime(e));
	}

	function onBarUp() {
		dragging = false;
		scheduleRest();
	}

	// ← → step the active handle anywhere in the dialog except on a button (handoff, "Scrub bar").
	function onKeydown(e: KeyboardEvent) {
		if (!ready || step !== 1 || e.target instanceof HTMLButtonElement) return;
		if (e.key === 'ArrowLeft' || e.key === 'ArrowRight') {
			e.preventDefault();
			stepActive(e.key === 'ArrowRight' ? 1 : -1);
		}
	}

	function onHandleKeydown(e: KeyboardEvent) {
		if (!ready) return;
		const { min, max } = handleBounds(active, range, firstFrame, lastFrame);
		if (e.key === 'Home') seekHandle(min);
		else if (e.key === 'End') seekHandle(max);
		else return;
		e.preventDefault();
	}

	onMount(() => {
		const v = videoEl;
		const area = scrubArea;
		const p = panel;
		if (v) v.volume = 0;
		// Non-passive so the wheel steps frames instead of scrolling the page.
		const onWheel = (e: WheelEvent) => {
			e.preventDefault();
			if (!ready) return;
			const n = wheel.push(e);
			if (n) stepActive(n > 0 ? 1 : -1);
		};
		area?.addEventListener('wheel', onWheel, { passive: false });
		p?.addEventListener('keydown', onKeydown);
		return () => {
			alive = false;
			reader.stop();
			clearTimeout(restTimer);
			clearTimeout(checkTimer);
			area?.removeEventListener('wheel', onWheel);
			p?.removeEventListener('keydown', onKeydown);
			for (const s of Object.values(stills)) if (s) URL.revokeObjectURL(s.url);
			// Stop the detached element buffering the stream.
			v?.removeAttribute('src');
			v?.load();
		};
	});

	// Run the check for the current choice. It is never shown and then pulled away: the primary is
	// withdrawn from the moment a check starts.
	function check(delay = 0) {
		const token = ++checkToken;
		checking = true;
		checkError = '';
		clearTimeout(checkTimer);
		checkTimer = setTimeout(async () => {
			try {
				const res = await preflight({ inTime, outTime, mode, edition });
				if (token === checkToken) pf = res;
			} catch (e) {
				if (token === checkToken) checkError = toMessage(e);
			} finally {
				if (token === checkToken) checking = false;
			}
		}, delay);
	}

	async function toReview() {
		if (!canReview) return;
		step = 2;
		queueError = '';
		check();
		await tick();
		reviewHeading?.focus();
	}

	// The Back button leaves with this step, so focus goes to the handle in hand: the arrow keys work
	// from anywhere in the dialog, but only from inside it.
	async function backToPicker() {
		step = 1;
		await tick();
		panel?.querySelector<HTMLElement>(`[aria-label="${active === 'in' ? 'First' : 'Last'} frame kept"]`)?.focus();
	}

	async function chooseMode(next: Mode) {
		mode = next;
		check();
		if (next === 'edition') {
			await tick();
			nameInput?.select();
		}
	}

	async function confirm() {
		const a = stills.in;
		const b = stills.out;
		if (!canConfirm || !a || !b) return;
		queuing = true;
		queueError = '';
		try {
			const refused = await onconfirm({ inTime, outTime, mode, edition, captureIn: a.file, captureOut: b.file });
			if (refused) pf = refused;
		} catch (e) {
			queueError = toMessage(e);
		} finally {
			queuing = false;
		}
	}

	const pct = (t: number) => (duration ? (t / duration) * 100 : 0);
	const position = $derived(timecode(time(active), interval));
	const nothingToCut = $derived(cutsNothing(range, firstFrame, lastFrame));
	const canReview = $derived(ready && !decodeFailed && !resting && !nothingToCut && !!stills.in && !!stills.out);
	const summary = $derived(ready ? trimSummary(range, firstFrame, lastFrame) : '');
	const previewName = $derived(editionFileName(filename, edition));
	// The server's answer wins once it is about this name; until then, an empty name is plain.
	const nameRefusal = $derived(
		mode !== 'edition' ? '' : !previewName ? 'Enter a name for this edition.' : !checking && pf ? nameRefusalCopy(pf) : ''
	);
	const refused = $derived(!checking && pf && !pf.ok && !nameRefusalCopy(pf) ? refusalCopy(pf, TRIM) : '');
	const canConfirm = $derived(!checking && !queuing && !!pf?.ok && !nameRefusal && !checkError);
	const primaryLabel = $derived(queuing ? 'Queuing…' : mode === 'edition' ? 'Save edition' : 'Replace original');
	const ends: { end: TrimEnd; label: string; name: string }[] = [
		{ end: 'in', label: 'First frame kept', name: 'first' },
		{ end: 'out', label: 'Last frame kept', name: 'last' }
	];
</script>

<Modal titleId="trim-picker-title" {onclose} size="media" align="center" busy={queuing} bind:panel>
	<!-- Step 1 stays mounted, hidden, through the review, so ‹ Back returns to the same decoded video. -->
	<div class="min-h-0 flex-col gap-4 overflow-x-hidden overflow-y-auto p-5 {step === 1 ? 'flex' : 'hidden'}">
		<h2 id="trim-picker-title" class="text-lg text-ink">Trim video</h2>

		<div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_13.5rem]">
			<div class="min-w-0" bind:this={scrubArea}>
				<p class="mb-1.5 text-xs uppercase tracking-wide text-muted">
					{active === 'in' ? 'First frame kept' : 'Last frame kept'}
				</p>
				<div class="relative aspect-video border border-rule bg-black">
					<!-- Silent by construction: muted, volume 0, no controls, never played. -->
					<!-- svelte-ignore a11y_media_has_caption -->
					<video
						bind:this={videoEl}
						{src}
						muted
						preload="auto"
						playsinline
						class="h-full w-full object-contain"
						onloadedmetadata={onLoadedMetadata}
						onseeked={scheduleRest}
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

				<!-- svelte-ignore a11y_no_static_element_interactions -->
				<div
					bind:this={bar}
					data-testid="trim-bar"
					class="relative mt-5 h-1.5 cursor-pointer touch-none rounded-full bg-rule"
					onpointerdown={onBarDown}
					onpointermove={onBarMove}
					onpointerup={onBarUp}
					onpointercancel={onBarUp}
				>
					<div
						class="absolute inset-y-0 bg-accent"
						style:left="{pct(inTime)}%"
						style:width="{Math.max(0, pct(outTime) - pct(inTime))}%"
					></div>
					{#each ends as { end, label } (end)}
						{@const isActive = active === end}
						{@const limits = handleBounds(end, range, firstFrame, lastFrame)}
						<!-- A 44px touch target around the 10×20 handle. The active one draws on top. -->
						<div
							role="slider"
							tabindex="0"
							aria-label={label}
							aria-valuemin={Math.round(limits.min * 1000) / 1000}
							aria-valuemax={Math.round(limits.max * 1000) / 1000}
							aria-valuenow={Math.round(time(end) * 1000) / 1000}
							aria-valuetext={timecode(time(end), interval)}
							aria-disabled={!ready}
							class="absolute top-1/2 flex h-11 w-11 -translate-x-1/2 -translate-y-1/2 items-center justify-center focus:outline-none [&:focus-visible>span]:ring-2 [&:focus-visible>span]:ring-accent {isActive
								? 'z-10'
								: ''}"
							style:left="{pct(time(end))}%"
							onpointerdown={(e) => onHandleDown(end, e)}
							onfocus={() => activate(end)}
							onkeydown={onHandleKeydown}
						>
							<span class="block h-5 w-2.5 rounded-theme bg-accent {isActive ? 'ring-2 ring-ink' : ''}"></span>
						</div>
					{/each}
				</div>

				<div class="mt-3 flex justify-between text-xs tabular-nums text-muted">
					<span>{position}</span>
					<span>{formatDuration(duration)}</span>
				</div>
				<p class="mt-1 min-h-5 text-sm text-ink">{summary}</p>
				<p class="sr-only" aria-live="polite">{announced}</p>
				{#if captureError}
					<p class="mt-1 text-sm text-warn" role="alert">{captureError}</p>
				{/if}
				<p class="mt-1 text-xs text-muted">Drag a handle · ← → or the mouse wheel step the active handle one frame</p>
			</div>

			<div class="grid grid-cols-2 content-start gap-3 lg:grid-cols-1">
				{#each ends as { end, label, name } (end)}
					<div>
						<p class="mb-1.5 text-xs uppercase tracking-wide text-muted">{label}</p>
						<button
							type="button"
							aria-pressed={active === end}
							aria-label="Work on the {name} frame, {formatDuration(time(end))}"
							disabled={!ready}
							class="relative block aspect-video w-full bg-black focus:outline-none focus-visible:ring-2 focus-visible:ring-accent {active === end
								? 'border-2 border-accent'
								: 'border border-rule'}"
							onclick={() => activate(end)}
						>
							{#if stills[end]}
								<img src={stills[end]?.url} alt="" class="h-full w-full object-contain" />
							{/if}
						</button>
						<p class="mt-1 text-xs tabular-nums text-muted">{timecode(time(end), interval)}</p>
					</div>
				{/each}
				<p class="col-span-2 text-xs text-muted lg:col-span-1">Click a frame to work on that end</p>
			</div>
		</div>

		<div class="flex justify-end gap-2">
			<button type="button" class="btn-ghost rounded-theme px-3 py-1.5 text-sm" onclick={onclose}>Cancel</button>
			<!-- Withdrawn (ghost look), never dimmed, until there's a range to review. -->
			<button
				type="button"
				class="rounded-theme px-3 py-1.5 text-sm {canReview ? 'bg-accent font-medium text-accent-ink' : 'btn-ghost cursor-not-allowed'}"
				disabled={!canReview}
				onclick={toReview}
			>
				Review trim…
			</button>
		</div>
	</div>

	{#if step === 2}
		<div class="flex min-h-0 flex-col gap-4 overflow-y-auto p-5">
			<h2 bind:this={reviewHeading} tabindex="-1" class="text-lg text-ink focus:outline-none">Review trim</h2>

			<div class="grid gap-3 sm:grid-cols-2">
				{#each ends as { end, label } (end)}
					<div class="max-w-[16rem]">
						<p class="mb-1.5 text-xs uppercase tracking-wide text-muted">{label}</p>
						<div class="aspect-video border border-rule bg-black">
							<img src={stills[end]?.url} alt={label} class="h-full w-full object-contain" />
						</div>
						<p class="mt-1 text-xs tabular-nums text-muted">{timecode(time(end), interval)}</p>
					</div>
				{/each}
			</div>

			<p class="text-sm text-ink">{reviewSummary(range)}</p>

			{#if refused}
				<p class="text-sm text-ink">{refused}</p>
			{:else}
				<fieldset class="space-y-2" disabled={queuing}>
					<legend class="sr-only">How to save the trim</legend>
					<label class="flex cursor-pointer items-start gap-2 rounded-theme border px-3 py-2 {mode === 'replace' ? 'border-accent' : 'border-rule'}">
						<input type="radio" name="trim-mode" class="mt-1" checked={mode === 'replace'} onchange={() => chooseMode('replace')} />
						<span>
							<span class="block text-sm text-ink">Replace original</span>
							<span class="block text-xs text-muted">
								Same library item.{pf?.expires_at ? ` Original kept until ${shortDate(pf.expires_at)}, then deleted.` : ''}
							</span>
						</span>
					</label>
					<div class="rounded-theme border px-3 py-2 {mode === 'edition' ? 'border-accent' : 'border-rule'}">
						<label class="flex cursor-pointer items-start gap-2">
							<input type="radio" name="trim-mode" class="mt-1" checked={mode === 'edition'} onchange={() => chooseMode('edition')} />
							<span>
								<span class="block text-sm text-ink">Save as edition</span>
								<span class="block text-xs text-muted">A new library item with the same people, Tags and edits. The original isn't touched.</span>
							</span>
						</label>
						{#if mode === 'edition'}
							<div class="mt-2 pl-6">
								<label for="trim-edition-name" class="mb-1 block text-xs text-ink">Edition name</label>
								<input
									id="trim-edition-name"
									bind:this={nameInput}
									bind:value={edition}
									oninput={() => check(250)}
									aria-describedby={nameRefusal ? 'trim-name-refusal' : undefined}
									class="w-full max-w-xs rounded-theme border border-rule bg-bg px-2.5 py-1.5 text-sm text-ink"
								/>
								<p class="mt-1 break-all font-mono text-xs text-muted">{previewName}</p>
								{#if nameRefusal}
									<p id="trim-name-refusal" class="mt-1 text-xs text-warn" role="alert">{nameRefusal}</p>
								{/if}
							</div>
						{/if}
					</div>
				</fieldset>

				{#if checking}
					<p class="text-sm text-muted">Checking…</p>
				{:else if checkError}
					<p class="text-sm text-warn" role="alert">{checkError}</p>
				{:else if pf}
					<p class="text-xs text-muted">Needs {formatBytes(pf.need_bytes)} free · {formatBytes(pf.free_bytes)} available</p>
				{/if}
			{/if}

			{#if queueError}
				<p class="text-sm text-warn" role="alert">{queueError}</p>
			{/if}

			<div class="flex flex-wrap items-center justify-between gap-2">
				<button type="button" class="btn-quiet px-3 py-1.5 text-sm" disabled={queuing} onclick={backToPicker}>‹ Back</button>
				<div class="flex gap-2">
					<button type="button" class="btn-ghost rounded-theme px-3 py-1.5 text-sm" disabled={queuing} onclick={onclose}>Cancel</button>
					{#if !refused}
						<button
							type="button"
							class="rounded-theme px-3 py-1.5 text-sm {canConfirm ? 'bg-accent font-medium text-accent-ink' : 'btn-ghost cursor-not-allowed'}"
							disabled={!canConfirm}
							onclick={confirm}
						>
							{primaryLabel}
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}
</Modal>
