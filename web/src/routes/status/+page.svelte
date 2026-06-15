<script lang="ts">
	// System Activity page (F21.4): live status cards, owner controls, and the
	// 30-day job history. Reads the shared activity store (also driving the header
	// indicator) and owns history fetching.
	import { onMount, onDestroy } from 'svelte';
	import { activity } from '$lib/activity.svelte';
	import { api, setAdminToken } from '$lib/api';
	import type { JobRun } from '$lib/types';
	import { toMessage, formatAgo, formatUntil, formatDurMs, formatUptime } from '$lib/format';
	import StatusCard from '$lib/components/StatusCard.svelte';
	import JobHistory from '$lib/components/JobHistory.svelte';

	let runs = $state<JobRun[]>([]);
	let tokenInput = $state('');
	let tokenError = $state('');
	let toast = $state('');
	let confirmingRescan = $state(false);
	let busy = $state(false);

	// Toasts auto-clear so a stale "Scan started." doesn't linger.
	let toastTimer: ReturnType<typeof setTimeout> | undefined;
	function showToast(msg: string) {
		toast = msg;
		clearTimeout(toastTimer);
		toastTimer = setTimeout(() => (toast = ''), 4000);
	}

	const a = $derived(activity.data);
	const isOwner = $derived(activity.isOwner);
	const needToken = $derived(activity.needToken);

	async function loadHistory() {
		try {
			runs = (await api.activityHistory()).runs ?? [];
		} catch {
			// History is best-effort on this page; the cards already show liveness.
		}
	}

	onMount(() => {
		activity.start();
		loadHistory();
	});
	onDestroy(() => {
		activity.stop();
		clearTimeout(toastTimer);
	});

	// Refresh history whenever a scan finishes (running -> idle).
	let prevState: string | undefined;
	$effect(() => {
		const s = a?.scan.state;
		if (prevState === 'running' && s === 'idle') loadHistory();
		prevState = s;
	});

	async function submitToken(e: Event) {
		e.preventDefault();
		if (!tokenInput.trim()) return;
		setAdminToken(tokenInput);
		tokenInput = '';
		// Independent calls — run them together; both must finish before we judge owner.
		await Promise.all([activity.refreshCaps(), activity.refresh()]);
		if (activity.caps && !activity.caps.owner) {
			tokenError = 'Incorrect token — try again.';
			return;
		}
		tokenError = '';
		await loadHistory();
	}

	async function doRescan() {
		confirmingRescan = false;
		busy = true;
		try {
			const r = await api.rescan();
			showToast(r.started ? 'Scan started.' : 'A scan is already running.');
		} catch (e) {
			showToast(toMessage(e));
		} finally {
			busy = false;
			activity.refresh();
		}
	}

	async function doReload() {
		busy = true;
		try {
			const r = await api.reloadConfig();
			showToast(`Config reloaded — ${r.fields} fields.`);
		} catch (e) {
			showToast(toMessage(e));
		} finally {
			busy = false;
		}
	}
</script>

<section class="mx-auto max-w-5xl space-y-6">
	<header class="space-y-1">
		<h1 class="skin-title text-2xl font-semibold text-ink">System Activity</h1>
		<p class="text-sm text-muted">What Holodex is doing under the hood — scans, the thumbnail queue, and the last 30 days of jobs.</p>
	</header>

	{#if activity.loading && !a}
		<p class="py-16 text-center text-sm text-muted">Loading activity…</p>
	{:else if !a && needToken}
		<form onsubmit={submitToken} class="space-y-2 rounded-theme border border-rule bg-surface px-3 py-3">
			<p class="text-sm text-ink">This server requires an admin token to view activity.</p>
			<div class="flex flex-wrap items-center gap-2">
				<input
					bind:value={tokenInput}
					type="password"
					placeholder="Admin token"
					class="rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink outline-none placeholder:text-muted focus:border-accent"
				/>
				<button type="submit" class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink">Unlock</button>
				{#if tokenError}<span class="text-sm text-warn">{tokenError}</span>{/if}
			</div>
		</form>
	{:else if !a}
		<p class="rounded-theme border border-warn bg-surface px-3 py-2 text-sm text-ink">
			{activity.error || 'Activity is unavailable.'}
		</p>
	{:else}
		{#if a.system.controls_unauthenticated}
			<p class="rounded-theme border border-warn bg-surface px-3 py-2 text-sm text-ink">
				<span class="font-semibold text-warn">⚠ Admin controls are reachable without a token</span>
				on a non-loopback bind. Set <code class="font-mono text-ink">ADMIN_TOKEN</code> to require authentication.
			</p>
		{/if}

		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
			<StatusCard title="Scan">
				{#if a.scan.state === 'running'}
					<p class="flex items-center gap-2 text-2xl font-semibold text-accent">
						<span class="activity-dot h-2.5 w-2.5 rounded-full bg-accent"></span>Indexing…
					</p>
					<p class="text-sm text-muted">{a.scan.trigger} · started {formatAgo(a.scan.started_at)}</p>
				{:else if a.scan.last_run}
					<p class="text-2xl font-semibold text-ink">{formatAgo(a.scan.last_run.finished_at)}</p>
					<p class="text-sm text-muted">
						last scan · {formatDurMs(a.scan.last_run.duration_ms)}{#if a.scan.next_scheduled_at} · next {formatUntil(a.scan.next_scheduled_at)}{/if}
					</p>
				{:else}
					<p class="text-2xl font-semibold text-ink">Never run</p>
					{#if a.scan.next_scheduled_at}
						<p class="text-sm text-muted">next {formatUntil(a.scan.next_scheduled_at)}</p>
					{/if}
				{/if}
				{#if a.scan.last_run}
					{@const lr = a.scan.last_run}
					{#if lr.added || lr.updated || lr.removed || lr.errors}
						<div class="flex flex-wrap gap-1.5 text-xs">
							{#if lr.added}<span class="rounded-theme bg-surface-2 px-1.5 py-0.5 text-muted">+{lr.added} added</span>{/if}
							{#if lr.updated}<span class="rounded-theme bg-surface-2 px-1.5 py-0.5 text-muted">{lr.updated} updated</span>{/if}
							{#if lr.removed}<span class="rounded-theme bg-surface-2 px-1.5 py-0.5 text-muted">{lr.removed} removed</span>{/if}
							{#if lr.errors > 0}<span class="rounded-theme border border-warn px-1.5 py-0.5 text-warn">{lr.errors} errors</span>{/if}
						</div>
					{:else}
						<p class="text-xs text-muted">no changes</p>
					{/if}
				{/if}
			</StatusCard>

			<StatusCard title="Thumbnails">
				{#if a.thumbnails.depth > 0}
					<p class="text-2xl font-semibold tabular-nums text-ink">{a.thumbnails.depth}</p>
					<p class="text-sm text-muted">pending · {a.thumbnails.in_flight} in flight</p>
					<p class="text-sm text-muted">{a.thumbnails.high} high / {a.thumbnails.normal} normal · {a.thumbnails.workers} workers</p>
				{:else}
					<p class="text-2xl font-semibold text-ink">Queue empty</p>
					<p class="text-sm text-muted">{a.thumbnails.workers} workers idle</p>
				{/if}
			</StatusCard>

			<StatusCard title="Library">
				<p class="text-2xl font-semibold tabular-nums text-ink">{a.library.videos_active}</p>
				<p class="text-sm text-muted">videos{#if a.library.videos_inactive > 0} · {a.library.videos_inactive} inactive{/if}</p>
				<p class="text-sm text-muted">{a.library.people} people · {a.library.tags} tags</p>
			</StatusCard>

			<StatusCard title="System">
				<p class="text-2xl font-semibold text-ink">{a.system.ready ? 'Ready' : 'Not ready'}</p>
				<p class="text-sm text-muted">up {formatUptime(a.system.uptime_seconds)} · v{a.system.version}</p>
				<p class="text-sm text-muted">media path {a.system.media_path_present ? 'set' : 'missing'}</p>
			</StatusCard>
		</div>

		{#if isOwner}
			<div class="space-y-2">
				<h2 class="text-xs uppercase tracking-wide text-muted">Actions</h2>
				<div class="flex flex-wrap items-center gap-2">
					{#if confirmingRescan}
						<span class="text-sm text-ink">Rescan the whole library?</span>
						<button onclick={doRescan} disabled={busy} class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60">Yes, rescan</button>
						<button onclick={() => (confirmingRescan = false)} class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2">Cancel</button>
					{:else}
						<button onclick={() => (confirmingRescan = true)} disabled={busy} class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60">Rescan library</button>
						<button onclick={doReload} disabled={busy} class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60">Reload config</button>
					{/if}
					{#if toast}<span class="text-sm text-muted">{toast}</span>{/if}
				</div>
			</div>
		{/if}

		<section class="space-y-3">
			<h2 class="skin-title text-lg font-semibold text-ink">Recent jobs</h2>
			<JobHistory {runs} />
		</section>
	{/if}
</section>
