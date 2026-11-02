<script lang="ts">
	// The video page's File name row in Maintenance (handoff "Video page File name row"): the
	// current name, its plan state, the single-video Rename, the name lock, undo and History.
	// Hidden while renaming is Off. The rename and undo don't confirm: one file, undone here.
	import { onDestroy, onMount } from 'svelte';
	import { api, ApiError } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import { clockTime, shortDate } from '#lib/maintenance.ts';
	import {
		atLeastManual,
		changedSince,
		failureLine,
		lockLine,
		parseMode,
		readBack,
		staleFailure,
		tokenLabel,
		badNameLine,
		type RenameHistoryEntry,
		type RenameMode,
		type RenamePlanRow,
		type RenameStatus
	} from '#lib/rename.ts';
	import NameLockButton from './NameLockButton.svelte';
	import Padlock from './Padlock.svelte';

	let { videoId, heldUntil }: { videoId: number; heldUntil?: string } = $props();

	const POLL_MS = 1500;

	let mode = $state<RenameMode>('off');
	let row = $state<RenamePlanRow | null>(null);
	let templates = $state<string[]>([]);
	let status = $state<RenameStatus>({ queued: false, failed: false });
	let history = $state<RenameHistoryEntry[]>([]);
	let startedAt = $state('');
	let undoTaken = $state('');
	let message = $state('');
	let historyOpen = $state(false);
	let busy = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;
	let destroyed = false;

	async function load() {
		try {
			// The plan resolves the whole library, so Off (the default) skips it.
			mode = parseMode((await api.adminSettings())['rename.mode']);
			if (mode === 'off') return;
			const [plan, st, hist] = await Promise.all([
				api.renamePlan({ videoId, reason: 'all', limit: 1 }),
				api.renameStatus(videoId),
				api.renameHistory(videoId)
			]);
			mode = plan.mode;
			templates = plan.templates;
			row = plan.rows[0] ?? null;
			status = st;
			history = hist;
			if (st.queued) poll();
			else startedAt = '';
		} catch (e) {
			message = toMessage(e);
		}
	}

	function poll() {
		clearTimeout(timer);
		if (destroyed) return;
		timer = setTimeout(async () => {
			try {
				const st = await api.renameStatus(videoId);
				if (st.queued) poll();
				else await load();
			} catch {
				poll();
			}
		}, POLL_MS);
	}

	onMount(load);
	onDestroy(() => {
		destroyed = true;
		clearTimeout(timer);
	});

	async function run(fn: () => Promise<unknown>) {
		if (busy) return;
		busy = true;
		message = '';
		try {
			await fn();
		} catch (e) {
			message = toMessage(e);
		} finally {
			busy = false;
		}
	}

	const rename = () =>
		run(async () => {
			const res = await api.renameApply([videoId]);
			if (res.queued === 0) {
				await load();
				return;
			}
			startedAt = new Date().toISOString();
			status = { queued: true, failed: false };
			poll();
		});

	const undo = () =>
		run(async () => {
			try {
				await api.renameUndo(videoId);
			} catch (e) {
				if (e instanceof ApiError && e.status === 409 && e.body?.reason === 'taken') {
					undoTaken = String(e.body.name ?? '');
					return;
				}
				throw e;
			}
			startedAt = new Date().toISOString();
			status = { queued: true, failed: false };
			poll();
		});

	const retry = () =>
		run(async () => {
			if (staleFailure(status.error)) {
				await api.renameDismiss(videoId);
				await load();
				return;
			}
			await api.renameRetry(videoId);
			startedAt = new Date().toISOString();
			status = { queued: true, failed: false };
			poll();
		});

	const dismiss = () =>
		run(async () => {
			await api.renameDismiss(videoId);
			status = { queued: false, failed: false };
		});

	const title = $derived(row?.title || row?.current || '');
	const locked = $derived(row?.reason === 'locked');
	// The latest rename, while the file still has the name it gave (an undo's own record locks
	// instead, so it shows the lock line rather than another undo).
	const lastRename = $derived.by(() => {
		const last = history.at(-1);
		return last && row && last.new_name === row.current && last.trigger !== 'undo' ? last : null;
	});
	const rb = $derived(row?.reason === 'wont-read-back' ? readBack(row, templates) : null);
	const editField = $derived(rb ? (rb.token === 'studio' ? 'studio name' : tokenLabel(rb.token)) : 'metadata');
</script>

{#if row && mode !== 'off'}
	<div class="flex flex-wrap items-start gap-x-3 gap-y-2">
		<span class="w-28 shrink-0 pt-1.5 text-sm text-muted">File name</span>
		<div class="min-w-0 flex-1 space-y-1 pt-1.5">
			<div class="flex flex-wrap items-start justify-between gap-x-3 gap-y-1">
				<p class="min-w-0 font-mono text-sm text-ink wrap-anywhere">
					{row.current}
					{#if locked}<span class="ml-1 inline-flex align-middle text-ink" role="img" aria-label="Locked"><Padlock /></span>{/if}
				</p>
				{#if !status.queued && row.reason !== 'held'}
					<span class="max-sm:order-last max-sm:w-full">
						<NameLockButton {videoId} {title} {locked} labelled onchange={load} />
					</span>
				{/if}
			</div>

			{#if status.queued}
				<p class="flex items-center gap-2 text-xs text-muted">
					<span class="inline-flex items-center gap-1 rounded-full bg-accent px-2 py-0.5 text-[0.65rem] text-accent-ink">
						<svg class="h-3 w-3 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
							<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
							<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
						</svg>
						renaming
					</span>
					{startedAt ? `Started ${clockTime(startedAt)}. ` : ''}You can leave this page.
				</p>
			{:else if status.failed}
				<p class="text-xs text-warn">
					{staleFailure(status.error) ? failureLine(status.error) : `Rename failed: ${status.error ?? 'unknown error'}.`}
				</p>
				<div class="mt-2 flex gap-3 text-xs">
					<button type="button" class="text-accent hover:underline" onclick={retry}
						>{staleFailure(status.error) ? 'Refresh' : 'Retry'}</button
					>
					<button type="button" class="text-muted hover:text-ink hover:underline" onclick={dismiss}>Dismiss</button>
				</div>
			{:else if undoTaken}
				<p class="text-xs text-warn">Can't undo: <span class="font-mono">{undoTaken}</span> is taken.</p>
				<div class="mt-2 flex gap-2">
					<button type="button" class="btn-quiet rounded-theme px-2 py-1 text-sm" onclick={() => (undoTaken = '')}>Dismiss</button>
				</div>
			{:else}
				{#if lastRename}
					<div class="rounded-theme border border-rule bg-surface px-3 py-2 text-sm text-ink">
						Renamed {shortDate(lastRename.created_at)} from <span class="font-mono wrap-anywhere">{lastRename.old_name}</span>
					</div>
				{/if}
				{#if locked}
					<p class="text-xs text-muted">{lockLine(row.lock_source, row.locked_at)} Holodex leaves its name alone.</p>
				{:else if row.reason === 'held'}
					<p class="text-xs text-muted">
						Can't rename while the original is held{heldUntil ? ` (until ${shortDate(heldUntil)})` : ''}.
					</p>
				{:else if row.reason === 'already-named' && !lastRename}
					<p class="text-xs text-muted">Matches template {(row.template ?? 0) + 1}.</p>
				{:else if row.reason === 'will-rename' || row.reason === 'out-of-date'}
					<p class="text-xs text-muted">Would be <span class="font-mono text-ink wrap-anywhere">{row.name}</span></p>
					{#if row.reason === 'out-of-date'}
						<p class="text-xs text-muted">{changedSince(row.changes)}</p>
					{/if}
				{:else if row.reason === 'wont-read-back'}
					<p class="text-xs text-warn">
						Won't rename:
						{#if rb}
							{tokenLabel(rb.token)} <span class="font-mono">{rb.value}</span>
							{#if rb.breaker}contains <span class="font-mono">{rb.breaker}</span>, which ends {`{${rb.token}}`} in template
								{rb.template}.{:else}would read back as <span class="font-mono">{rb.readsAs || '(nothing)'}</span>.{/if}
						{:else}the name wouldn't read back to the same values.{/if}
					</p>
					<p class="text-xs text-muted">Edit the template or the {editField} in File names.</p>
				{:else if row.reason === 'clash'}
					<p class="text-xs text-warn">
						Won't rename: <span class="font-mono">{row.name}</span> is already in this folder and isn't in your library.
					</p>
				{:else if row.reason === 'bad-name'}
					<p class="text-xs text-warn">Won't rename: {badNameLine(row)}</p>
					<p class="text-xs text-muted">Edit the template or the metadata in File names.</p>
				{:else if row.reason === 'no-template'}
					<p class="text-xs text-muted">No template fits: each one needs a value this video doesn't have.</p>
				{/if}

				{#if (lastRename && !locked) || history.length > 0 || ((row.reason === 'will-rename' || row.reason === 'out-of-date') && atLeastManual(mode))}
					<div class="mt-2 flex flex-wrap gap-2">
						{#if (row.reason === 'will-rename' || row.reason === 'out-of-date') && atLeastManual(mode)}
							<button type="button" class="btn-ghost rounded-theme px-3 py-1.5 text-sm" aria-busy={busy} onclick={rename}>Rename</button>
						{/if}
						{#if lastRename && !locked && atLeastManual(mode)}
							<button type="button" class="btn-quiet rounded-theme px-2 py-1.5 text-sm" aria-busy={busy} onclick={undo}
								>Undo rename</button
							>
						{/if}
						{#if history.length > 0}
							<button
								type="button"
								class="btn-quiet rounded-theme px-2 py-1.5 text-sm"
								aria-expanded={historyOpen}
								onclick={() => (historyOpen = !historyOpen)}>History ({history.length}) {historyOpen ? '▴' : '▾'}</button
							>
						{/if}
					</div>
				{/if}
				{#if historyOpen && history.length > 0}
					<ol class="space-y-1 text-xs">
						<li class="flex flex-wrap gap-x-3 text-muted">
							<span class="w-14 shrink-0">Arrived</span><span class="font-mono wrap-anywhere">{history[0].old_name}</span>
						</li>
						{#each history as h, i (i)}
							<li class="flex flex-wrap gap-x-3 {i === history.length - 1 ? 'text-ink' : 'text-muted'}">
								<span class="w-14 shrink-0 text-muted">{shortDate(h.created_at)}</span>
								<span class="w-16 shrink-0 text-muted">{h.trigger}</span>
								<span class="min-w-0 font-mono wrap-anywhere">{h.new_name}</span>
							</li>
						{/each}
					</ol>
				{/if}
			{/if}
			{#if message}
				<p class="text-xs text-warn" role="alert">{message}</p>
			{/if}
		</div>
	</div>
{/if}
