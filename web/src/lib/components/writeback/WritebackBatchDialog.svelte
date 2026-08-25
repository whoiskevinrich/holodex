<script lang="ts">
	// N-video writeback progress dialog (HOLODEX-239, ADR-077 D2/D3). Sibling of
	// WritebackFormDialog, not a mode of it: there is no per-video field list here,
	// only an aggregate pending/running/done/failed count from a shared batchID, so
	// the body is a confirm step + a progress bar rather than N editable rows. Chrome
	// (backdrop, focus trap, Escape-to-close) is copied from that dialog's existing
	// pattern. Tokens only; QA 3 skins.
	import { onMount } from 'svelte';
	import { toMessage } from '$lib/format';
	import { waitForWritebackBatch, type BatchStatus } from '$lib/writebackJob';

	let {
		scopeLabel,
		videoCountHint,
		trigger,
		batchStatus,
		onclose,
		onapplied,
		initialBatch
	}: {
		scopeLabel: string;
		// Exact affected-video count when known up front (single tag: tag.video_count
		// is exactly what VideoIDsForTag will enqueue against). null when a bulk
		// selection's deduplicated union can't be known without enqueuing (see
		// tag-writeback-exclusion-handoff.md) — the real figure appears from
		// trigger()'s own `enqueued` once the sync actually starts.
		videoCountHint: number | null;
		// Absent when `initialBatch` is set — that caller's batch is already enqueued,
		// so there's nothing left to trigger.
		trigger?: () => Promise<{ batch_id: string; enqueued: number }>;
		batchStatus: (batchId: string) => Promise<BatchStatus>;
		onclose: () => void;
		// Fired once the batch settles (pending+running reach 0) or turns out to have
		// nothing to enqueue — the caller's cue to refresh anything it shows. Optional:
		// a sync never changes tag.writeback_enabled/video_count, so most callers have
		// nothing to refresh.
		onapplied?: () => void;
		// Seeds the dialog directly into 'progress' for a batch a caller already
		// started server-side by the time this dialog mounts (F57, ADR-086 D2 — the
		// Film-studio cascade) — skipping the confirm/starting interstitial AND
		// trigger() itself, rather than wrapping the already-known {batch_id, enqueued}
		// in a fake trigger() call. Additive: undefined for every existing caller, which
		// keeps today's confirm → starting → progress sequence unchanged.
		initialBatch?: { batch_id: string; enqueued: number };
	} = $props();

	// 'timeout' is its own phase (not a flag layered on 'progress') so every
	// terminal outcome — settled or not — reads as one explicit state instead of
	// forcing the template to re-derive "still running" from a side boolean.
	type Phase = 'confirm' | 'starting' | 'progress' | 'done' | 'partial' | 'zero' | 'timeout' | 'error';
	let phase = $state<Phase>(initialBatch ? 'progress' : 'confirm');
	let enqueued = $state(initialBatch?.enqueued ?? 0);
	let status = $state<BatchStatus>({ pending: 0, running: 0, done: 0, failed: 0 });
	let errorMsg = $state('');

	let dialogEl = $state<HTMLElement | null>(null);
	let dialogTrigger: HTMLElement | null = null;
	let unmounted = false; // stops an in-flight poll if the dialog goes away

	onMount(() => {
		dialogTrigger = document.activeElement as HTMLElement | null;
		if (initialBatch) {
			void watch(initialBatch.batch_id);
		} else {
			const first = [
				...(dialogEl?.querySelectorAll<HTMLElement>('button:not(:disabled)') ?? [])
			].find((el) => el.offsetParent !== null);
			first?.focus();
		}
		return () => {
			unmounted = true;
			dialogTrigger?.focus?.();
		};
	});

	function trapTab(e: KeyboardEvent) {
		if (e.key !== 'Tab' || !dialogEl) return;
		const focusable = [...dialogEl.querySelectorAll<HTMLElement>('button, [tabindex="0"]')].filter(
			(el) => !(el as HTMLButtonElement).disabled && el.offsetParent !== null
		);
		if (focusable.length === 0) return;
		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		if (e.shiftKey && document.activeElement === first) {
			e.preventDefault();
			last.focus();
		} else if (!e.shiftKey && document.activeElement === last) {
			e.preventDefault();
			first.focus();
		}
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && phase !== 'starting') onclose();
		trapTab(e);
	}

	async function start() {
		phase = 'starting';
		errorMsg = '';
		try {
			const res = await trigger?.();
			if (!res) return;
			enqueued = res.enqueued;
			if (enqueued === 0) {
				phase = 'zero';
				onapplied?.();
				return;
			}
			phase = 'progress';
			await watch(res.batch_id);
		} catch (e) {
			errorMsg = toMessage(e);
			phase = 'error';
		}
	}

	// Polls an already-enqueued batch to settlement — shared by start() (a batch this
	// dialog just triggered) and initialBatch's onMount (a batch triggered before this
	// dialog even mounted).
	async function watch(batchId: string) {
		try {
			const final = await waitForWritebackBatch(batchId, batchStatus, {
				cancelled: () => unmounted
			});
			status = final;
			const settled = final.pending + final.running === 0;
			phase = !settled ? 'timeout' : final.failed > 0 ? 'partial' : 'done';
			onapplied?.();
		} catch (e) {
			errorMsg = toMessage(e);
			phase = 'error';
		}
	}

	const progressed = $derived(status.done + status.failed);
	const progressPct = $derived(enqueued > 0 ? Math.round((progressed / enqueued) * 100) : 0);
</script>

<div
	class="fixed inset-0 z-50 flex items-start justify-center bg-bg/70 px-4 py-[20vh]"
	onclick={(e) => {
		if (e.target === e.currentTarget && phase !== 'starting') onclose();
	}}
>
	<div
		bind:this={dialogEl}
		role="dialog"
		aria-modal="true"
		aria-labelledby="writeback-batch-title"
		tabindex="-1"
		onkeydown={onKeydown}
		class="w-full max-w-md overflow-hidden rounded-theme border border-rule bg-surface shadow-lg"
	>
		<div class="flex items-center justify-between border-b border-rule px-4 py-3">
			<h2 id="writeback-batch-title" class="text-sm font-semibold text-ink">Sync writeback</h2>
			<button
				onclick={() => phase !== 'starting' && onclose()}
				disabled={phase === 'starting'}
				aria-label="Close"
				class="rounded-theme p-1 text-muted hover:text-ink"
			>
				<svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true">
					<path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
				</svg>
			</button>
		</div>

		<div class="space-y-3 px-4 py-3">
			<p class="text-sm font-medium text-ink">{scopeLabel}</p>

			{#if phase === 'confirm' || phase === 'starting'}
				<p class="text-xs text-muted">
					{#if videoCountHint !== null}
						This will write to up to {videoCountHint} file{videoCountHint === 1 ? '' : 's'} —
						videos this wouldn't change the written value for are skipped.
					{:else}
						This will sync every video currently carrying any of these tags.
					{/if}
				</p>
			{:else if phase === 'progress' || phase === 'timeout'}
				<div class="space-y-1" aria-live="polite">
					<div class="h-2 w-full overflow-hidden rounded-theme bg-surface-2">
						<div class="h-full bg-accent transition-all" style="width: {progressPct}%"></div>
					</div>
					<p class="text-xs text-muted">
						{#if phase === 'timeout'}
							Still going — {progressed} of {enqueued} settled so far. You can close this and check
							back later.
						{:else}
							{progressed} of {enqueued}
						{/if}
					</p>
				</div>
			{:else if phase === 'done'}
				<p class="text-xs text-muted" aria-live="polite">
					{status.done} file{status.done === 1 ? '' : 's'} updated.
				</p>
			{:else if phase === 'partial'}
				<p class="text-xs text-warn" aria-live="polite">
					{status.done} updated, {status.failed} failed — check Activity for details.
				</p>
			{:else if phase === 'zero'}
				<p class="text-xs text-muted" aria-live="polite">
					Nothing to sync — no videos currently carry a genre value from this tag.
				</p>
			{:else if phase === 'error'}
				<p class="text-xs text-warn" aria-live="polite">{errorMsg}</p>
			{/if}
		</div>

		<div class="flex justify-end gap-2 border-t border-rule px-4 py-3">
			{#if phase === 'confirm'}
				<button onclick={() => onclose()} class="btn-ghost px-3 py-1.5 text-sm">Cancel</button>
				<button onclick={start} class="btn-accent px-3 py-1.5 text-sm">Start sync</button>
			{:else if phase === 'starting'}
				<button disabled class="btn-accent px-3 py-1.5 text-sm">Starting…</button>
			{:else if phase === 'error'}
				<button onclick={() => onclose()} class="btn-ghost px-3 py-1.5 text-sm">Cancel</button>
				<button onclick={start} class="btn-accent px-3 py-1.5 text-sm">Retry</button>
			{:else if phase !== 'progress'}
				<!-- done, partial, zero, timeout — every terminal outcome gets a Close;
				     only actively 'progress'ing shows no footer buttons. -->
				<button onclick={() => onclose()} class="btn-accent px-3 py-1.5 text-sm">Close</button>
			{/if}
		</div>
	</div>
</div>
