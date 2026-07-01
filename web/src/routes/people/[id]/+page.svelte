<script lang="ts">
	import { tick } from 'svelte';
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import type {
		DecisionSource,
		EnrichSource,
		Person,
		PersonAlias,
		PersonDetailResponse,
		PersonImageRole,
		PersonImageSet,
		PersonRenameConflict,
		ResolvedField,
		Video
	} from '$lib/types';
	import { providerOf } from '$lib/f36';
	import AsyncState from '$lib/components/AsyncState.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import CurationFieldRow from '$lib/components/CurationFieldRow.svelte';
	import EntityVideos from '$lib/components/EntityVideos.svelte';
	import ProvenanceBadge from '$lib/components/ProvenanceBadge.svelte';
	import EnrichPicker from '$lib/components/EnrichPicker.svelte';
	import PersonPicker from '$lib/components/PersonPicker.svelte';
	import PersonBanner from '$lib/components/PersonBanner.svelte';
	import PersonImageFrame from '$lib/components/PersonImageFrame.svelte';
	import PersonGallery from '$lib/components/PersonGallery.svelte';
	import SourceSelect from '$lib/components/SourceSelect.svelte';
	import UrlValueList from '$lib/components/UrlValueList.svelte';
	import { videoCount } from '$lib/format';

	let person = $state<Person | null>(null);
	let videos = $state<Video[]>([]);
	// F37: the unified resolved view (baseline `record`, no in_sync) — same shape the media
	// page consumes; the raw enriched[] block is retired.
	let resolved = $state<ResolvedField[]>([]);
	let images = $state<PersonImageSet>({ roles: {}, gallery: [] });
	let loading = $state(true);
	let error = $state('');

	// Owner core-slot upload (F25): one hidden file input, retargeted per role.
	let coreInput = $state<HTMLInputElement | null>(null);
	let uploadRole = $state<PersonImageRole>('headshot');
	let uploadBusy = $state('');
	let imageError = $state('');

	// Owner-curated routing aliases (F23, ADR-036) — search & scan routing, deliberately a
	// separate system from the display-only "Also known as" merge row in Details (F37 RD2).
	// Read from the person payload; add/delete are owner-gated. Errors render inline.
	let aliases = $state<PersonAlias[]>([]);
	let newAlias = $state('');
	let aliasBusy = $state(false);
	let aliasError = $state('');
	let aliasInput = $state<HTMLInputElement | null>(null);
	// Merge (F23): the picker for "merge another person in", and the collision prompt
	// shown when an added alias — or a rename (F37 RD1) — already names a different,
	// existing person.
	let mergeOpen = $state(false);
	let conflict = $state<Person | null>(null);

	// Enrichment controls (owner-only, F22). sources is loaded once when the client
	// is confirmed owner; the picker drives a provider resolve→apply.
	let sources = $state<EnrichSource[]>([]);
	let pickerOpen = $state(false);
	let busy = $state('');
	// Action errors render inline in the panel — never via the page-level `error`,
	// which AsyncState uses to replace the whole page.
	let actionError = $state('');

	// Rename flow (F37 RD1 — name materializes, never pins). Selecting a non-record name
	// chip (or committing a custom name) opens the confirm dialog; on confirm the person is
	// renamed and the old name kept as an F23 alias. A 409 swaps the dialog to the merge
	// offer, routing into the existing F23 conflict confirm — never an auto-merge.
	let renameTo = $state(''); // '' = dialog closed
	let renameBusy = $state(false);
	let renameError = $state('');
	let renameConflict = $state<PersonRenameConflict | null>(null);
	let nameRowEl = $state<HTMLElement | null>(null);

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.effectiveOwner); // owner AND Admin mode on (F29)
	// Banner renders only when a real one is set — for everyone, including the owner
	// (F25.30). Not every person has a banner-sized image; an empty 5:2 placeholder band
	// would dominate the page with generic art. Mirrors the poster rule (F25.27).
	const hasBanner = $derived(images.roles.banner?.present ?? false);
	// v1 enriches People from the first person-capable provider.
	const provider = $derived(sources.find((s) => s.entity_types.includes('person'))?.name ?? '');
	// The provider is "linked" (Clear offered) when any resolved field carries one of its
	// candidates or values — the F37 replacement for scanning the retired enriched[].
	const enrichedByProvider = $derived(
		!!provider &&
			resolved.some(
				(f) =>
					(f.candidates ?? []).some((c) => providerOf(c.source) === provider) ||
					(f.items ?? []).some((it) => it.sources.includes(provider))
			)
	);

	// Field partitions (F37 handoff): Name first, then the replace fields, then the
	// merge fields ("Also known as"). A field with no value and no candidates doesn't
	// render; visitors additionally see only fields that resolved to a value.
	const nameField = $derived(resolved.find((f) => f.canonical === 'name'));
	const replaceFields = $derived(
		resolved.filter(
			(f) =>
				f.canonical !== 'name' &&
				!f.multi &&
				(isOwner
					? f.values.length > 0 || (f.candidates ?? []).length > 0
					: f.values.some((v) => v.trim() !== ''))
		)
	);
	// Split the compact single-line vitals from the long-text prose (bio): the vitals
	// tile the two-column grid up top, and the full-width prose reads last, so a long
	// bio no longer buries the scannable facts (design-critique 2026-07-01).
	const compactFields = $derived(replaceFields.filter((f) => f.display !== 'long_text'));
	const longFields = $derived(replaceFields.filter((f) => f.display === 'long_text'));
	const mergeFields = $derived(
		resolved.filter((f) => !!f.multi && (isOwner || f.values.length > 0))
	);

	// The provider name behind a visitor row's ProvenanceBadge — the winning namespace
	// unless it is the record baseline or a manual literal (mirrors the media page's
	// `file:` check with the person's `record:` prefix, RD4).
	function winnerProvider(f: ResolvedField): string {
		const ns = (f.winning_source ?? '').split(':')[0];
		return ns === 'record' || ns === 'file' || ns === 'manual' ? '' : ns;
	}

	function applyPersonDetail(res: PersonDetailResponse) {
		person = res.person;
		videos = res.items ?? [];
		resolved = res.resolved ?? [];
		images = res.images ?? { roles: {}, gallery: [] };
		aliases = res.person.aliases ?? [];
	}

	function load(current: number) {
		loading = true;
		error = '';
		api
			.getPerson(current)
			.then((res) => {
				applyPersonDetail(res);
				aliasError = '';
			})
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	}

	$effect(() => load(id));

	// Load providers once the client is confirmed owner (the layout polls caps).
	$effect(() => {
		if (isOwner && sources.length === 0) {
			api
				.enrichSources()
				.then((res) => (sources = res.sources ?? []))
				.catch(() => {});
		}
	});

	// Refetch-after-mutate (cf. the media page's applyMediaDetail idiom): re-read the
	// person so resolved[] reflects a new decision/curation/enrichment without flashing
	// the whole page through the AsyncState loading view. Non-fatal on error.
	async function reloadDetail() {
		try {
			applyPersonDetail(await api.getPerson(id));
		} catch {
			// Non-fatal — the mutation already succeeded; a full reload reconciles.
		}
	}

	async function clearProvider() {
		if (!provider) return;
		busy = 'clear';
		actionError = '';
		try {
			await api.enrichClear(id, provider);
			await reloadDetail();
		} catch (e) {
			actionError = toMessage(e);
		} finally {
			busy = '';
		}
	}

	// F37: persist a per-field source decision then refetch so resolved[] reflects it.
	// DB-only — a person has no file, so there is no writeback and no sync state. Unlike
	// the media page, selecting the record chip is a STANDING blank-pin decision (RD3:
	// `{source:"record"}` suppresses a provider value until re-decided), so every chip
	// maps to PUT. `name` never reaches here (the SourceSelect intercept owns it, RD1).
	async function decideField(canonical: string, source: DecisionSource, manualValue?: string) {
		await api.setPersonFieldDecision(id, canonical, {
			source,
			...(source === 'manual' ? { manual_value: manualValue ?? '' } : {})
		});
		await reloadDetail();
	}

	// Rename flow (RD1). openRename is the SourceSelect `onadopt` interceptor for the
	// name field: no decision call ever fires; the confirm dialog owns what happens next.
	function openRename(_source: DecisionSource, value: string) {
		const next = value.trim();
		if (!next || next === person?.name) return;
		renameError = '';
		renameConflict = null;
		renameTo = next;
	}

	function closeRename() {
		// ConfirmDialog returns focus to the chip that opened it on unmount (a11y).
		renameTo = '';
		renameConflict = null;
		renameError = '';
	}

	async function confirmRename() {
		if (renameBusy || !renameTo) return;
		renameBusy = true;
		renameError = '';
		try {
			const res = await api.renamePerson(id, renameTo);
			if (res.conflict) {
				// Name taken — swap to the merge offer (never auto-merge, F23 invariant).
				renameConflict = res.conflict;
				return;
			}
			renameTo = '';
			await reloadDetail();
			// Focus lands back on the name row: the record chip now carries the new name.
			await tick();
			nameRowEl?.querySelector<HTMLElement>('[data-seg="record"]')?.focus();
		} catch (e) {
			renameError = toMessage(e);
		} finally {
			renameBusy = false;
		}
	}

	// The owner chose "Merge into …" from the rename collision: route into the existing
	// F23 merge confirmation (the conflict panel in the Aliases card) — same informed
	// confirm as an alias collision, with both video counts. Nothing merges until the
	// owner confirms there.
	function mergeFromRename() {
		if (!renameConflict) return;
		conflict = {
			id: renameConflict.id,
			name: renameConflict.name,
			video_count: renameConflict.video_count
		};
		closeRename();
	}

	async function addAlias(e: SubmitEvent) {
		e.preventDefault();
		const value = newAlias.trim();
		if (!value || aliasBusy) return;
		aliasBusy = true;
		aliasError = '';
		try {
			const res = await api.addAlias(id, value);
			if (res.conflict) {
				// The name already belongs to a real, separate person — never merge
				// silently (homonyms exist). Surface it; the owner decides.
				conflict = res.conflict;
				return;
			}
			aliases = res.aliases ?? aliases;
			newAlias = '';
			aliasInput?.focus(); // keep focus for quick multi-add
		} catch (err) {
			aliasError = toMessage(err);
		} finally {
			aliasBusy = false;
		}
	}

	// The owner confirmed the colliding person is the same human → merge them in.
	async function mergeConflict() {
		if (!conflict) return;
		aliasBusy = true;
		aliasError = '';
		try {
			await api.mergePersons(id, conflict.id);
			conflict = null;
			newAlias = '';
			onMerged();
		} catch (err) {
			aliasError = toMessage(err);
		} finally {
			aliasBusy = false;
		}
	}

	// After any merge, reload so the (now larger) video list + new alias show.
	function onMerged() {
		load(id);
	}

	async function removeAlias(a: PersonAlias) {
		if (aliasBusy) return;
		aliasError = '';
		const prev = aliases;
		aliases = aliases.filter((x) => x.id !== a.id); // optimistic
		try {
			await api.deleteAlias(id, a.id);
		} catch (err) {
			aliases = prev; // restore on failure
			aliasError = toMessage(err);
		}
	}

	// Version stamp for a core role's serving URL, or undefined (empty slot → the
	// backend serves the themed placeholder; no ?v= needed).
	function roleVersion(role: PersonImageRole): number | undefined {
		return images.roles[role]?.version || undefined;
	}

	// Owner: open the file picker targeting a specific core role (F25.7).
	function pickCore(role: PersonImageRole) {
		uploadRole = role;
		coreInput?.click();
	}

	async function onCorePick(e: Event) {
		const input = e.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		input.value = '';
		if (!file || uploadBusy) return;
		uploadBusy = uploadRole;
		imageError = '';
		try {
			await api.uploadPersonImage(id, file, uploadRole);
			// Refresh just the image set — the new ?v= stamp busts the cache for the
			// uploaded role — without flashing the whole page through the AsyncState
			// loading view (a full load() would). Awaited so the busy label holds until
			// the new image is in hand, giving a clean swap (e.g. add-banner → banner).
			await reloadImages();
		} catch (err) {
			imageError = toMessage(err);
		} finally {
			uploadBusy = '';
		}
	}

	// Re-read the image set after a gallery change without refetching 500 videos.
	async function reloadImages() {
		try {
			images = await api.getPersonImages(id);
		} catch (err) {
			// Non-fatal (the mutation already succeeded; a full reload reconciles), but log
			// it so a persistently failing refresh is debuggable rather than silent.
			console.error('reloadImages: image set refresh failed', err);
		}
	}
</script>

<!-- Uniform owner "Edit" overlay for a core image slot (banner/headshot/poster) — one
     shape, one label, one scrim across all three, positioned per call site. -->
{#snippet editBtn(role: PersonImageRole, position: string)}
	{#if isOwner}
		<button
			onclick={() => pickCore(role)}
			disabled={uploadBusy === role}
			aria-label={`Replace ${role}`}
			title={`Replace ${role}`}
			class="absolute z-10 {position} rounded-theme bg-bg/80 px-2.5 py-1.5 text-xs font-semibold text-ink shadow-sm backdrop-blur-sm hover:text-accent disabled:opacity-60"
		>
			{uploadBusy === role ? '…' : 'Edit'}
		</button>
	{/if}
{/snippet}

<AsyncState {loading} error={error || (!person ? 'Not found.' : '')}>
	<EntityVideos
		backHref="/people"
		backLabel="All people"
		name={person?.name ?? ''}
		{videos}
		empty="No videos for this person."
	>
		{#snippet hero()}
			<!-- F25 hero: a 5:2 parallax banner with the 1:1 headshot overlapping its lower-left,
			     beside the optional 2:3 poster and the name — so the name reads as one unit with
			     the face (not stranded above the banner). Headshot + poster share a height (sized
			     by height; width follows the aspect). The poster shows only when a real one exists:
			     an empty poster slot would just duplicate the headshot's placeholder. Owners set a
			     missing poster from the gallery below (promote-with-crop). -->
			<div class="relative">
				{#if hasBanner}
					<PersonBanner personId={id} name={person?.name ?? ''} version={roleVersion('banner')} eager />
					{@render editBtn('banner', 'right-2 top-2')}
				{:else if isOwner}
					<!-- No banner set: the owner gets an explicit add path here, since the band was the
					     only entry point for setting one. Visitors see nothing — the row below sits flush
					     at the top with no overhang. Reuses the core-slot upload (F25.30). -->
					<button
						onclick={() => pickCore('banner')}
						disabled={uploadBusy === 'banner'}
						title="Add banner"
						class="flex w-full items-center justify-center gap-2 rounded-theme border border-dashed border-rule bg-surface px-3 py-4 text-sm font-semibold text-muted hover:border-accent hover:text-accent disabled:opacity-60"
					>
						{uploadBusy === 'banner' ? 'Adding…' : '+ Add banner'}
					</button>
				{/if}
				<!-- The headshot+name row overhangs the banner only when there is one; with no band it
				     sits flush (a small gap below the owner's add-banner control). (F25.30) -->
				<div class="flex items-end gap-3 pl-3 {hasBanner ? '-mt-10 sm:-mt-12' : isOwner ? 'mt-3' : ''}">
					<div class="relative shrink-0">
						<PersonImageFrame
							personId={id}
							role="headshot"
							name={person?.name ?? ''}
							version={roleVersion('headshot')}
							frameClass="portrait-frame--1x1 h-28 w-auto sm:h-36"
							eager
						/>
						{@render editBtn('headshot', 'bottom-1 right-1')}
					</div>
					{#if images.roles.poster?.present}
						<div class="relative shrink-0">
							<PersonImageFrame
								personId={id}
								role="poster"
								name={person?.name ?? ''}
								alt=""
								version={roleVersion('poster')}
								frameClass="portrait-frame--2x3 h-28 w-auto sm:h-36"
								eager
							/>
							{@render editBtn('poster', 'bottom-1 right-1')}
						</div>
					{/if}
					<div class="min-w-0 flex-1 pb-1">
						<h1 class="skin-title truncate text-2xl font-semibold text-ink">{person?.name ?? ''}</h1>
						<p class="text-sm text-muted">{videoCount(videos.length)}</p>
					</div>
				</div>
				{#if isOwner}
					<input
						bind:this={coreInput}
						onchange={onCorePick}
						type="file"
						accept="image/*"
						class="hidden"
						aria-hidden="true"
						tabindex="-1"
					/>
				{/if}
				{#if imageError}
					<p class="mt-2 text-sm text-warn">{imageError}</p>
				{/if}
			</div>
		{/snippet}

		{#snippet detail()}
			<!-- Details (F37): every replace field on the F36 source-chip radiogroup with the
			     `record` baseline; `aliases` as the display-only "Also known as" merge row.
			     Deliberate absences vs. the media page: no Write button, no out-of-sync pill
			     (a person has no file), and the Name row RENAMES (RD1) instead of pinning. -->
			{#if resolved.length || (isOwner && provider)}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted">Details</h2>
						{#if isOwner && provider}
							<div class="flex flex-wrap items-center gap-2">
								<button
									onclick={() => (pickerOpen = true)}
									class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink"
								>
									Enrich from {provider}
								</button>
								{#if enrichedByProvider}
									<button
										onclick={clearProvider}
										disabled={busy === 'clear'}
										title={`Remove the enrichment data ${provider} added to this person`}
										class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface-2 disabled:opacity-60"
									>
										Clear {provider} data
									</button>
								{/if}
							</div>
						{/if}
					</div>

					{#if resolved.length}
						<dl class="grid grid-cols-1 gap-3 text-sm sm:grid-cols-2">
							{#if nameField && isOwner}
								<!-- The Name row (RD1): the same chip radiogroup, but selecting a
								     non-record chip opens the rename confirm — never a decision. -->
								<div class="sm:col-span-2" bind:this={nameRowEl}>
									<dt class="mb-1 text-muted">{nameField.label}:</dt>
									<dd>
										<SourceSelect
											field={nameField}
											baselineKey="record"
											groupLabel="Name — selecting a source renames this person"
											onadopt={openRename}
											decide={async () => {
												/* unreachable: the intercept owns every non-record selection (RD1) */
											}}
										/>
									</dd>
								</div>
							{/if}

							{#each compactFields as f (f.canonical)}
								{#if isOwner}
									<!-- Replace field, owner: the selected chip IS the value (media idiom). -->
									<div>
										<dt class="mb-1 text-muted">{f.label}:</dt>
										<dd>
											<SourceSelect
												field={f}
												baselineKey="record"
												decide={(s, mv) => decideField(f.canonical, s, mv)}
											/>
										</dd>
									</div>
								{:else}
									<!-- Visitor: read-only resolved value, exactly like the old rows. -->
									<div>
										<dt class="inline text-muted">{f.label}:</dt>
										{#if f.display === 'url'}
											<dd class="inline"><UrlValueList values={f.values} /></dd>
										{:else}
											<dd class="inline text-ink">{f.values.join(', ')}</dd>
										{/if}
										{#if winnerProvider(f)}
											<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
										{/if}
									</div>
								{/if}
							{/each}

							{#each mergeFields as f (f.canonical)}
								<!-- "Also known as" (RD2): provider aliases as an F30 merge row —
								     display-only curation (✕ suppress / + Add); kept chips never route
								     scans or search (that is the separate Aliases card below). No
								     nowrite toggle: persons have no writeback. -->
								<div class="sm:col-span-2">
									<dt class="mb-1 text-muted">
										{f.canonical === 'aliases' ? 'Also known as' : f.label}:
									</dt>
									<dd>
										<CurationFieldRow
											field={f}
											{isOwner}
											showWriteToggle={false}
											curate={(req) => api.curatePerson(id, req)}
											clearCuration={(req) => api.clearPersonCuration(id, req)}
											onchanged={reloadDetail}
										/>
									</dd>
								</div>
							{/each}

							{#each longFields as f (f.canonical)}
								<!-- Long-text (bio) reads last as a full-width prose block, so it
								     doesn't bury the compact vitals above (design-critique 2026-07-01).
								     Long-text fit (P1-1): the resolved value is the reading surface;
								     the chip row beneath is the source selector (chips stay clamped). -->
								<div class="sm:col-span-2">
									<dt class="inline text-muted">{f.label}:</dt>
									{#if f.values[0]?.trim()}
										<dd class="mt-1 block leading-relaxed text-ink">{f.values[0]}</dd>
									{:else if isOwner}
										<dd class="mt-1 block text-muted">—</dd>
									{/if}
									{#if isOwner}
										<dd class="block">
											<SourceSelect
												field={f}
												baselineKey="record"
												decide={(s, mv) => decideField(f.canonical, s, mv)}
											/>
										</dd>
									{:else if winnerProvider(f)}
										<ProvenanceBadge provider={winnerProvider(f)} label={winnerProvider(f)} />
									{/if}
								</div>
							{/each}
						</dl>
					{:else}
						<p class="text-sm text-muted">No details yet.</p>
					{/if}

					{#if actionError}
						<p class="text-sm text-warn">{actionError}</p>
					{/if}
				</section>
			{/if}

			<!-- The F23 routing-alias card — deliberately its own system below Details (RD2):
			     these names drive search and scan routing, unlike the display-only
			     "Also known as" chips above. -->
			{#if aliases.length || isOwner}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted">Aliases</h2>
						{#if isOwner}
							<button
								onclick={() => (mergeOpen = true)}
								class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
							>
								Merge a person in…
							</button>
						{/if}
					</div>
					<p class="text-sm text-muted">
						Searching either name finds this person, and future scans match it too.
					</p>

					<div class="flex flex-wrap gap-2" aria-live="polite">
						{#each aliases as a (a.id)}
							<span
								class="inline-flex items-center gap-1 rounded-full bg-surface-2 px-2.5 py-0.5 text-sm text-ink"
							>
								{a.alias}
								{#if isOwner}
									<button
										onclick={() => removeAlias(a)}
										disabled={aliasBusy}
										aria-label={`Remove alias ${a.alias}`}
										class="leading-none text-muted hover:text-accent focus:text-accent disabled:opacity-60"
									>
										×
									</button>
								{/if}
							</span>
						{/each}
						{#if !aliases.length && isOwner}
							<p class="text-sm text-muted">No aliases yet.</p>
						{/if}
					</div>

					{#if isOwner}
						<form onsubmit={addAlias} class="flex flex-wrap items-center gap-2">
							<input
								bind:this={aliasInput}
								bind:value={newAlias}
								type="text"
								placeholder="Add an alias"
								aria-label="Add an alias"
								aria-describedby={aliasError ? 'alias-error' : undefined}
								class="min-w-0 flex-1 rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
							/>
							<button
								type="submit"
								disabled={aliasBusy}
								class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
							>
								Add
							</button>
						</form>
					{/if}

					{#if conflict}
						<div class="space-y-2 rounded-theme border border-rule bg-surface-2 p-3" aria-live="polite">
							<p class="text-sm text-ink">
								<span class="font-semibold">{conflict.name}</span> ({videoCount(conflict.video_count ?? 0)})
								is already a separate person. Are they the same as {person?.name}?
							</p>
							<div class="flex flex-wrap items-center gap-2">
								<button
									onclick={mergeConflict}
									disabled={aliasBusy}
									class="rounded-theme bg-accent px-3 py-1.5 text-sm font-semibold text-accent-ink disabled:opacity-60"
								>
									Yes, merge them in
								</button>
								<button
									onclick={() => { conflict = null; }}
									disabled={aliasBusy}
									class="rounded-theme border border-rule px-3 py-1.5 text-sm text-ink hover:bg-surface disabled:opacity-60"
								>
									No, keep separate
								</button>
							</div>
						</div>
					{/if}

					{#if aliasError}
						<p id="alias-error" class="text-sm text-warn">{aliasError}</p>
					{/if}
				</section>
			{/if}

			{#if images.gallery.length || isOwner}
				<section class="rounded-theme border border-rule bg-surface p-4">
					<PersonGallery
						personId={id}
						name={person?.name ?? ''}
						items={images.gallery}
						owner={isOwner}
						onchanged={reloadImages}
					/>
				</section>
			{/if}
		{/snippet}
	</EntityVideos>
</AsyncState>

{#if pickerOpen && provider}
	<EnrichPicker
		entityName={person?.name ?? ''}
		{provider}
		resolve={(prov, q) => api.enrichResolve(id, prov, q)}
		apply={(prov, extId) => api.enrichApply(id, prov, extId)}
		onclose={() => (pickerOpen = false)}
		onapplied={reloadDetail}
	/>
{/if}

{#if mergeOpen && person}
	<PersonPicker
		canonicalId={id}
		canonicalName={person.name}
		onclose={() => (mergeOpen = false)}
		onmerged={onMerged}
	/>
{/if}

<!-- Rename confirm (RD1) — the F23 merge-confirm modal idiom (role=dialog, aria-modal,
     focus trapped, Escape cancels + focus returns to the opening chip via ConfirmDialog's
     trigger restore). Informational, never warn-toned (accent variant). -->
{#if renameTo && !renameConflict}
	<ConfirmDialog
		title={`Rename to “${renameTo}”?`}
		confirmLabel="Rename"
		variant="accent"
		busy={renameBusy}
		error={renameError}
		onconfirm={confirmRename}
		oncancel={closeRename}
	>
		{#snippet body()}
			<p>
				“{person?.name}” is kept as an alias — search and future scans still match it.
			</p>
		{/snippet}
	</ConfirmDialog>
{:else if renameConflict}
	<!-- Name-collision (409): swap to the merge offer. Confirming routes into the existing
	     F23 merge confirmation (the conflict panel above) — never an auto-merge. -->
	<ConfirmDialog
		title={`“${renameConflict.name}” already exists (${videoCount(renameConflict.video_count)})`}
		confirmLabel={`Merge with “${renameConflict.name}”…`}
		cancelLabel="Keep separate"
		variant="accent"
		busy={false}
		onconfirm={mergeFromRename}
		oncancel={closeRename}
	>
		{#snippet body()}
			<p>
				Renaming would collide with that person. You can merge this person ({videoCount(
					videos.length
				)}) with them instead — videos combine and both names stay searchable.
			</p>
		{/snippet}
	</ConfirmDialog>
{/if}
