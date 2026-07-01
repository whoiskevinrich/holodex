<script lang="ts">
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { api } from '$lib/api';
	import { activity } from '$lib/activity.svelte';
	import type { DecisionSource, EnrichedField, EnrichSource, ExtraMetadata, MappedField, MediaDetailResponse, RefreshReport, RelatedResponse, ResolvedField, Video } from '$lib/types';
	import { formatBitrate, formatBytes, formatDuration, formatYear, resolutionBucket, toMessage } from '$lib/format';
	import { isReplaceField, outOfSyncCount } from '$lib/f36';
	import RelatedShelf from '$lib/components/RelatedShelf.svelte';
	import UrlValueList from '$lib/components/UrlValueList.svelte';
	import PersonPoster from '$lib/components/PersonPoster.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import EnrichPicker from '$lib/components/EnrichPicker.svelte';
	import ProvenanceBadge from '$lib/components/ProvenanceBadge.svelte';
	import WritebackFormDialog from '$lib/components/WritebackFormDialog.svelte';
	import CurationFieldRow from '$lib/components/CurationFieldRow.svelte';
	import SourceSelect from '$lib/components/SourceSelect.svelte';

	let video = $state<Video | null>(null);
	let extra = $state<ExtraMetadata[]>([]);
	let fields = $state<MappedField[]>([]);
	let resolved = $state<ResolvedField[]>([]);
	let enriched = $state<EnrichedField[]>([]);
	let related = $state<RelatedResponse | null>(null);
	let loading = $state(true);
	let error = $state('');
	let playFailed = $state(false);
	let showRaw = $state(false);
	let showEnriched = $state(false);
	let regenerating = $state(false);
	let thumbVersion = $state(0); // cache-bust the preview after a regenerate

	// Owner-only delete (F24, ADR-037). confirmMode drives which confirm dialog shows.
	let confirmMode = $state<'soft' | 'purge' | null>(null);
	let deleteBusy = $state(false);
	let deleteError = $state('');

	// Film enrichment (F26). sources loaded once; picker drives resolve→apply.
	let sources = $state<EnrichSource[]>([]);
	let pickerOpen = $state(false);
	let enrichBusy = $state('');
	let enrichError = $state('');

	// Metadata writeback (F28, ADR-041). writebackOpen drives the batch form dialog.
	let writebackOpen = $state(false);

	// Per-item metadata refresh (F31, ADR-047). refreshing drives the spinner/disable;
	// refreshStatus is the inline aria-live outcome line (no toast system).
	let refreshing = $state(false);
	let refreshStatus = $state<{ tone: 'muted' | 'warn'; text: string } | null>(null);

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	// Prefer the resolved title (may come from an enrichment provider) over the
	// filename-derived video.title. Falls back gracefully when no mapping is configured.
	const displayTitle = $derived(
		resolved.find((f) => f.canonical === 'title')?.values[0] ?? video?.title ?? ''
	);
	const provider = $derived(sources.find((s) => s.entity_types.includes('video'))?.name ?? '');
	// F36: "Write decisions to file" is available whenever the owner has resolved fields — a
	// decided file value is just as writable as an adopted provider value (RD5/P0-4). The count
	// of out-of-sync fields rides alongside it (RD2).
	const canWriteback = $derived(isOwner && resolved.length > 0);
	const outOfSyncN = $derived(outOfSyncCount(resolved));

	const graceDays = $derived(
		activity.caps?.delete_grace_period_seconds
			? Math.round(activity.caps.delete_grace_period_seconds / 86400)
			: 0
	);

	function openConfirm(mode: 'soft' | 'purge') {
		deleteError = '';
		confirmMode = mode;
	}

	async function confirmDelete() {
		if (!video || deleteBusy) return;
		deleteBusy = true;
		deleteError = '';
		try {
			await api.deleteMedia(video.id, { purge: confirmMode === 'purge' });
			// The item is gone from the library — returning to the grid (where it no
			// longer appears) is the feedback (no toast system yet, per the handoff).
			goto('/');
		} catch (e) {
			deleteError = toMessage(e);
			deleteBusy = false; // keep the dialog open so the message is visible
		}
	}

	async function regenerateThumbnail() {
		if (!video) return;
		regenerating = true;
		try {
			const status = await api.regenerateThumbnail(video.id);
			if (status === 200) {
				thumbVersion += 1; // embedded art extracted synchronously; bust now
			} else {
				setTimeout(() => (thumbVersion += 1), 4000); // queued; give the worker a moment
			}
		} catch (e) {
			// Generation may be disabled (503) or the request may fail; non-fatal.
			console.warn('thumbnail regenerate failed', e);
		} finally {
			regenerating = false;
		}
	}

	// Apply a freshly-fetched media detail to the page state (shared by the initial
	// load, post-enrich, and post-refresh refetches).
	function applyMediaDetail(res: MediaDetailResponse) {
		video = res.video;
		extra = res.metadata ?? [];
		fields = res.fields ?? [];
		resolved = res.resolved ?? [];
		enriched = res.enriched ?? [];
	}

	// F36: persist a per-field source decision then refetch so resolved[] reflects it. DB-only
	// (RD5) — no file write here; the file changes only via "Write decisions to file". Selecting
	// Keep file clears the decision (reverts to the file default), so it maps to DELETE.
	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		if (source === 'file') {
			await api.clearFieldDecision(id, canonical);
		} else {
			await api.setFieldDecision(id, canonical, {
				source,
				...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
			});
		}
		await reloadDetail();
	}

	// Refresh metadata (F31): force re-extract the file + re-enrich linked providers,
	// then refetch the detail so resolved[] reflects the new data and bust the cover.
	async function refreshMetadata() {
		if (!video || refreshing) return;
		refreshing = true;
		refreshStatus = null;
		try {
			const report = await api.refreshMedia(id);
			applyMediaDetail(await api.getMedia(id));
			thumbVersion += 1; // a re-extract may surface new embedded cover art
			refreshStatus = summarizeRefresh(report);
		} catch (e) {
			refreshStatus = { tone: 'warn', text: toMessage(e) };
		} finally {
			refreshing = false;
		}
	}

	// Build the inline status line from the report. A failed source reads as a warn
	// line (the file still updated); otherwise muted "synced" / "already in sync".
	function summarizeRefresh(r: RefreshReport): { tone: 'muted' | 'warn'; text: string } {
		const failed = r.sources.filter((s) => !s.ok).map((s) => s.source);
		if (failed.length) {
			return { tone: 'warn', text: `${failed.join(', ')} lookup failed — file metadata still updated` };
		}
		if (!r.changed) {
			return { tone: 'muted', text: 'Already in sync — nothing changed' };
		}
		const providers = r.sources.filter((s) => s.source !== 'file' && s.ok).map((s) => s.source);
		return {
			tone: 'muted',
			text: providers.length ? `Synced from file and ${providers.join(', ')}` : 'Synced from file'
		};
	}

	// Hide the full-viewport atmosphere overlay (.app-atmosphere::after, z-40) while a
	// video plays so the scan/vignette flourishes don't sit on top of the picture —
	// worst in Broadcast. Pure-CSS-gated: we only toggle the class; app.css owns the rule.
	function setPlaying(on: boolean) {
		document.body?.classList.toggle('is-playing', on);
	}
	// Restore the atmosphere on teardown (navigate away mid-play). The data-load effect
	// below also resets it per id — this route's component is REUSED across /media/[id]
	// param changes, so a play state from the previous item would otherwise linger.
	$effect(() => () => setPlaying(false));

	$effect(() => {
		const current = id;
		let cancelled = false; // ignore a stale response if id changes before it resolves
		loading = true;
		error = '';
		playFailed = false;
		refreshStatus = null; // a freshly-opened item starts with no refresh outcome
		setPlaying(false); // a freshly-opened item starts with the atmosphere visible
		api
			.getMedia(current)
			.then((res) => {
				if (cancelled) return;
				applyMediaDetail(res);
			})
			.catch((e) => {
				if (!cancelled) error = toMessage(e);
			})
			.finally(() => {
				if (!cancelled) loading = false;
			});
		return () => (cancelled = true);
	});

	// Load providers once the client is confirmed owner.
	$effect(() => {
		if (isOwner && sources.length === 0) {
			api
				.enrichSources()
				.then((res) => (sources = res.sources ?? []))
				.catch(() => {});
		}
	});

	// reloadDetail re-fetches the detail so resolved[] reflects new enrichment or
	// curation (the resolver re-runs server-side on each GET). Non-fatal on error.
	async function reloadDetail() {
		try {
			applyMediaDetail(await api.getMedia(id));
		} catch {
			// Non-fatal — caller's optimistic state stands.
		}
	}

	async function onApplied(f: EnrichedField[]) {
		enriched = f;
		await reloadDetail();
	}

	async function clearProvider() {
		if (!provider) return;
		enrichBusy = 'clear';
		enrichError = '';
		try {
			await api.enrichVideoClear(id, provider);
			enriched = enriched.filter((f) => f.provider !== provider);
		} catch (e) {
			enrichError = toMessage(e);
		} finally {
			enrichBusy = '';
		}
	}

	// Related "More with …" shelves (QW2/QW3). Non-blocking and tracks ONLY `id`, so it
	// fetches once per page view and the shelves don't reshuffle on incidental re-renders
	// (skin switch, thumbnail regenerate) — "stable per page view" (ADR-031). A fresh
	// item id draws anew; an error just omits the shelves. The cancel guard prevents a
	// slow response for a previous item from landing on the current one.
	$effect(() => {
		const current = id;
		let cancelled = false;
		related = null;
		api
			.related(current)
			.then((res) => {
				if (!cancelled) related = res;
			})
			.catch(() => {
				if (!cancelled) related = null;
			});
		return () => (cancelled = true);
	});
</script>

{#if loading}
	<p class="py-16 text-center text-sm text-muted">Loading…</p>
{:else if error || !video}
	<p class="rounded-theme border border-accent bg-surface px-3 py-2 text-sm text-ink">
		{error || 'Not found.'}
	</p>
{:else}
	<article class="mx-auto max-w-4xl space-y-6">
		<a href="/" class="text-sm text-muted hover:text-ink">← Back to library</a>

		<div class="group relative overflow-hidden rounded-theme border border-rule bg-black">
			{#if playFailed}
				<div class="flex aspect-video flex-col items-center justify-center gap-3 bg-surface text-center">
					<p class="text-sm text-muted">This browser can't decode this file's codec.</p>
					<a href={api.streamURL(video.id)} download class="rounded-theme bg-accent px-4 py-2 text-sm font-medium text-accent-ink">
						Download / open file
					</a>
				</div>
			{:else}
				<!-- svelte-ignore a11y_media_has_caption -->
				<!-- The generated cover (ADR-009) is the poster, so the player shows the
				     same frame as the card instead of a black box until play. -->
				<video
					src={api.streamURL(video.id)}
					poster={video.thumbnail_url
						? api.thumbnailReload(video.thumbnail_url, thumbVersion)
						: undefined}
					controls
					preload="metadata"
					class="aspect-video w-full bg-black"
					onplay={() => setPlaying(true)}
					onpause={() => setPlaying(false)}
					onended={() => setPlaying(false)}
					onerror={() => (playFailed = true)}
				></video>
				<button
					onclick={regenerateThumbnail}
					disabled={regenerating}
					title="Regenerate thumbnail"
					aria-label="Regenerate thumbnail"
					class="absolute right-2 top-2 z-10 rounded-theme bg-black/60 p-1.5 text-muted opacity-0 transition hover:text-ink focus-visible:opacity-100 group-hover:opacity-100 disabled:opacity-50"
				>
					<svg
						class="h-4 w-4 {regenerating ? 'animate-spin' : ''}"
						fill="currentColor"
						viewBox="0 0 24 24"
						aria-hidden="true"
					>
						<path
							d="M17.65 6.35A7.96 7.96 0 0 0 12 4a8 8 0 1 0 7.74 10h-2.08A6 6 0 1 1 12 6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"
						/>
					</svg>
				</button>
			{/if}
		</div>

		<header class="space-y-2">
			<h1 class="skin-title text-2xl font-semibold text-ink">{displayTitle}</h1>
			<div class="flex flex-wrap items-center gap-2 text-sm text-muted">
				<span class="rounded-theme bg-accent px-2 py-0.5 text-accent-ink">{resolutionBucket(video.width)}</span>
				<span>{video.width}×{video.height}</span>
				<span>·</span>
				<span>{formatDuration(video.duration_sec)}</span>
				{#if formatYear(video.recorded_at)}
					<span>·</span><span>{formatYear(video.recorded_at)}</span>
				{/if}
			</div>
		</header>

		{#if video.tags?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">Tags</h2>
				<div class="flex flex-wrap gap-2">
					{#each video.tags as t (t.id)}
						<a href={`/tags/${t.id}`} class="rounded-theme bg-surface-2 px-2.5 py-1 text-sm text-ink hover:text-accent">
							{t.name}
						</a>
					{/each}
				</div>
			</section>
		{/if}

		{#if video.people?.length}
			<section class="space-y-1.5">
				<h2 class="text-xs uppercase tracking-wide text-muted">People</h2>
				<!-- F25: 2:3 poster cards (placeholder when a person has no poster). -->
				<ul class="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-6">
					{#each video.people as p (p.id)}
						<li>
							<a
								href={`/people/${p.id}`}
								class="group block space-y-1.5 text-ink"
								title={p.name}
							>
								<div class="rounded-theme transition group-hover:opacity-90">
									<PersonPoster personId={p.id} name={p.name} />
								</div>
								<span class="line-clamp-2 text-xs text-muted group-hover:text-accent">{p.name}</span>
							</a>
						</li>
					{/each}
				</ul>
			</section>
		{/if}

		<!-- Metadata section (F27): resolved fields (merged file + enrichment) with
		     enrichment controls and writeback inline in the header. Falls back to
		     file-only fields when no resolver output is present. -->
		{#if resolved.length || fields.length || isOwner}
			<section class="space-y-1.5">
				<div class="flex flex-wrap items-center justify-between gap-2">
					<h2 class="text-xs uppercase tracking-wide text-muted">Metadata</h2>
					<div class="flex flex-wrap items-center gap-2">
						{#if isOwner}
							<button
								onclick={refreshMetadata}
								disabled={refreshing}
								title="Refresh metadata from the file and providers"
								aria-label="Refresh metadata from the file and providers"
								class="flex items-center gap-1 rounded-theme px-2 py-0.5 text-xs text-muted hover:text-accent focus-visible:text-accent disabled:opacity-60"
							>
								<svg
									class="h-3.5 w-3.5 {refreshing ? 'animate-spin' : ''}"
									fill="currentColor"
									viewBox="0 0 24 24"
									aria-hidden="true"
								>
									<path
										d="M17.65 6.35A7.96 7.96 0 0 0 12 4a8 8 0 1 0 7.74 10h-2.08A6 6 0 1 1 12 6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"
									/>
								</svg>
								{refreshing ? 'Refreshing…' : 'Refresh'}
							</button>
						{/if}
						{#if isOwner && provider}
							<button
								onclick={() => (pickerOpen = true)}
								class="rounded-theme bg-accent px-2.5 py-1 text-xs font-semibold text-accent-ink"
							>
								Enrich from {provider}
							</button>
							{#if enriched.some((f) => f.provider === provider)}
								<button
									onclick={clearProvider}
									disabled={enrichBusy === 'clear'}
									title={`Remove ${provider} enrichment data`}
									class="rounded-theme border border-rule px-2.5 py-1 text-xs text-ink hover:bg-surface-2 disabled:opacity-60"
								>
									Clear {provider}
								</button>
							{/if}
						{/if}
						{#if canWriteback}
							<button
								onclick={() => (writebackOpen = true)}
								class="flex items-center gap-1 rounded-theme px-2 py-0.5 text-xs text-muted hover:text-accent focus-visible:text-accent"
								title="Write decided field values to the file tags"
							>
								<svg class="h-3.5 w-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M12 3v13m0 0l-4-4m4 4l4-4M5 20h14"/></svg>
								Write decisions to file{#if outOfSyncN > 0}<span class="text-warn"> · {outOfSyncN} out of sync</span>{/if}
							</button>
						{/if}
					</div>
				</div>
				{#if enrichError}
					<p class="text-xs text-warn">{enrichError}</p>
				{/if}
				{#if refreshStatus}
					<p
						class="text-xs {refreshStatus.tone === 'warn' ? 'text-warn' : 'text-muted'}"
						aria-live="polite"
					>
						{refreshStatus.text}
					</p>
				{/if}
				{#if resolved.length}
				<dl class="grid grid-cols-1 gap-3 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
					{#each resolved as f (f.canonical)}
						{@const winnerProvider = f.winning_source && !f.winning_source.startsWith('file:') ? f.winning_source.split(':')[0] : ''}
						{#if f.display === 'image_url'}
							<div class="sm:col-span-2">
								<dt class="mb-1 text-muted">{f.label}:</dt>
								<dd>
									<img
										src={f.values[0]}
										alt={f.label}
										class="max-h-64 rounded-theme border border-rule object-contain"
									/>
								</dd>
								{#if winnerProvider}<ProvenanceBadge provider={winnerProvider} label={winnerProvider} />{/if}
							</div>
						{:else if f.display === 'long_text'}
							<div class="sm:col-span-2">
								<dt class="inline text-muted">{f.label}:</dt>
								<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
								{#if winnerProvider}<ProvenanceBadge provider={winnerProvider} label={winnerProvider} />{/if}
							</div>
						{:else if f.display === 'url'}
							<div>
								<dt class="inline text-muted">{f.label}:</dt>
								<dd class="inline"><UrlValueList values={f.values} /></dd>
								{#if winnerProvider}<ProvenanceBadge provider={winnerProvider} label={winnerProvider} />{/if}
							</div>
						{:else}
							<!-- Curatable text/set field (F30): per-value chips with provenance,
							     edit/remove/no-write, and an add affordance for set fields. -->
							<div>
								<dt class="mb-1 text-muted">{f.label}:</dt>
								<dd>
									{#if isReplaceField(f) && isOwner}
										<!-- F36 (ADR-051): replace field gets the owner-only source control
										     (resolved chip + SourceSelect + candidates). Merge fields and the
										     visitor view keep the F30 CurationFieldRow read-only render. -->
										<SourceSelect field={f} decide={(s, mv) => decideField(f.canonical, s, mv)} />
									{:else}
										<CurationFieldRow
											field={f}
											videoId={id}
											{isOwner}
											people={video.people ?? []}
											personStyle={f.canonical === 'actors' || f.canonical === 'director'}
											onchanged={reloadDetail}
										/>
									{/if}
								</dd>
							</div>
						{/if}
					{/each}
				</dl>
				{:else if fields.length}
				<dl class="grid grid-cols-1 gap-2 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
					{#each fields as f (f.canonical)}
						<div>
							<dt class="inline text-muted">{f.label}:</dt>
							<dd class="inline">{f.values.join(', ')}</dd>
						</div>
					{/each}
				</dl>
				{:else}
				<p class="rounded-theme border border-rule bg-surface px-4 py-3 text-sm text-muted">
					No metadata extracted yet.
				</p>
				{/if}
			</section>
		{/if}

		<section class="space-y-1.5">
			<h2 class="text-xs uppercase tracking-wide text-muted">File</h2>
			<div class="grid grid-cols-1 gap-2 rounded-theme border border-rule bg-surface p-4 text-sm sm:grid-cols-2">
			<div><span class="text-muted">File size:</span> {formatBytes(video.file_size)}</div>
			{#if video.container}<div><span class="text-muted">Container:</span> {video.container}</div>{/if}
			{#if video.video_codec}<div><span class="text-muted">Video codec:</span> {video.video_codec}</div>{/if}
			{#if video.audio_codec}<div><span class="text-muted">Audio codec:</span> {video.audio_codec}</div>{/if}
			{#if video.bitrate_kbps}
				<div><span class="text-muted">Bitrate:</span> {formatBitrate(video.bitrate_kbps)}</div>
			{/if}
			<div class="truncate sm:col-span-2" title={video.file_path}>
				<span class="text-muted">Path:</span> {video.file_path}
			</div>
		</div>
		</section>

		<!-- "More with …" shelves (QW3): person first, then tag. Each self-omits when
		     its block is null or empty, so an item with no siblings shows no rail. -->
		{#if related?.person}
			<RelatedShelf
				title={related.person.name}
				href={`/people/${related.person.id}`}
				items={related.person.items}
			/>
		{/if}
		{#if related?.tag}
			<RelatedShelf title={related.tag.name} href={`/tags/${related.tag.id}`} items={related.tag.items} />
		{/if}

		<!-- Owner-only Manage block (F24): destructive actions, kept apart from the
		     content and the Back link so a delete is never adjacent to navigation.
		     Effective gate (F29) so visitor view hides it. -->
		{#if isOwner}
			<section class="space-y-2 border-t border-rule pt-4">
				<h2 class="text-xs uppercase tracking-wide text-muted">Manage</h2>
				<div class="flex flex-wrap gap-2">
					<button
						onclick={() => openConfirm('soft')}
						class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10"
					>
						Move to Trash
					</button>
					<button
						onclick={() => openConfirm('purge')}
						class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10"
					>
						Delete permanently
					</button>
				</div>
			</section>
		{/if}

		<!-- Admin-only metadata sources (F29): the raw file-extracted payload and the raw
		     provider enrichment payload, kept as audit/debug disclosures at the bottom of
		     the page (below Manage). Owner + Admin mode only (effectiveOwner); each
		     self-omits when empty. Headings aligned to "Enrichment data: {source}". -->
		{#if isOwner && extra.length}
			<section>
				<button onclick={() => (showRaw = !showRaw)} class="text-sm text-muted hover:text-ink">
					{showRaw ? '▾' : '▸'} Enrichment data: File Extraction ({extra.length})
				</button>
				{#if showRaw}
					<table class="mt-2 w-full text-left text-xs">
						<tbody>
							{#each extra as m (m.source_key)}
								<tr class="border-b border-rule">
									<td class="py-1 pr-4 font-mono text-muted">{m.source_key}</td>
									<td class="py-1 text-ink">{m.value}</td>
								</tr>
							{/each}
						</tbody>
					</table>
				{/if}
			</section>
		{/if}

		{#if isOwner && enriched.length}
			<section>
				<button onclick={() => (showEnriched = !showEnriched)} class="text-sm text-muted hover:text-ink">
					{showEnriched ? '▾' : '▸'} Enrichment data: {provider} ({enriched.length})
				</button>
				{#if showEnriched}
					<dl class="mt-2 grid grid-cols-1 gap-2 text-xs sm:grid-cols-2">
						{#each enriched as f (f.canonical + f.provider)}
							{#if f.display === 'image_url'}
								<div class="sm:col-span-2">
									<dt class="inline text-muted">{f.label}:</dt>
									<dd class="inline text-ink">{f.values[0]}</dd>
								</div>
							{:else}
								<div>
									<dt class="inline text-muted">{f.label}:</dt>
									<dd class="inline text-ink">{f.display === 'long_text' ? f.values[0].slice(0, 120) + '…' : f.values.join(', ')}</dd>
								</div>
							{/if}
						{/each}
					</dl>
				{/if}
			</section>
		{/if}
	</article>

	{#if writebackOpen && video}
		<WritebackFormDialog
			fields={resolved}
			videoId={id}
			filePath={video.file_path}
			writeback={api.writebackMedia}
			onclose={() => (writebackOpen = false)}
			onapplied={() => (thumbVersion += 1)}
		/>
	{/if}

	{#if pickerOpen && provider && video}
		<EnrichPicker
			entityName={video.title}
			{provider}
			resolve={(prov, q) => api.enrichVideoResolve(id, prov, q)}
			apply={(prov, extId) => api.enrichVideoApply(id, prov, extId)}
			onclose={() => (pickerOpen = false)}
			onapplied={onApplied}
		/>
	{/if}

	{#if confirmMode === 'soft'}
		<ConfirmDialog
			title="Move to Trash?"
			confirmLabel="Move to Trash"
			busy={deleteBusy}
			error={deleteError}
			onconfirm={confirmDelete}
			oncancel={() => (confirmMode = null)}
		>
			{#snippet body()}
				<p>
					<span class="font-semibold">{video?.title}</span> will be hidden from your library{graceDays
						? ` and permanently deleted in ${graceDays} ${graceDays === 1 ? 'day' : 'days'}`
						: ''}. You can restore it from Trash{graceDays ? ' until then' : ''}.
				</p>
			{/snippet}
		</ConfirmDialog>
	{:else if confirmMode === 'purge'}
		<ConfirmDialog
			title="Delete permanently?"
			confirmLabel="Delete permanently"
			busy={deleteBusy}
			error={deleteError}
			onconfirm={confirmDelete}
			oncancel={() => (confirmMode = null)}
		>
			{#snippet body()}
				<p>
					<span class="font-semibold">{video?.title}</span> and its file will be
					<span class="font-semibold">permanently removed</span> from disk. This cannot be undone.
				</p>
				<p class="truncate font-mono text-xs text-muted" title={video?.file_path}>{video?.file_path}</p>
			{/snippet}
		</ConfirmDialog>
	{/if}
{/if}
