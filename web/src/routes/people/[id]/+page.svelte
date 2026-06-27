<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import { activity } from '$lib/activity.svelte';
	import type {
		EnrichedField,
		EnrichSource,
		Person,
		PersonAlias,
		PersonImageRole,
		PersonImageSet,
		Video
	} from '$lib/types';
	import AsyncState from '$lib/components/AsyncState.svelte';
	import EntityVideos from '$lib/components/EntityVideos.svelte';
	import ProvenanceBadge from '$lib/components/ProvenanceBadge.svelte';
	import EnrichPicker from '$lib/components/EnrichPicker.svelte';
	import PersonPicker from '$lib/components/PersonPicker.svelte';
	import PersonBanner from '$lib/components/PersonBanner.svelte';
	import PersonImageFrame from '$lib/components/PersonImageFrame.svelte';
	import PersonGallery from '$lib/components/PersonGallery.svelte';
	import UrlValueList from '$lib/components/UrlValueList.svelte';
	import { videoCount } from '$lib/format';

	let person = $state<Person | null>(null);
	let videos = $state<Video[]>([]);
	let enriched = $state<EnrichedField[]>([]);
	let images = $state<PersonImageSet>({ roles: {}, gallery: [] });
	let loading = $state(true);
	let error = $state('');

	// Owner core-slot upload (F25): one hidden file input, retargeted per role.
	let coreInput = $state<HTMLInputElement | null>(null);
	let uploadRole = $state<PersonImageRole>('headshot');
	let uploadBusy = $state('');
	let imageError = $state('');

	// Owner-curated aliases (F23, ADR-036). Read from the person payload; add/delete
	// are owner-gated. Errors render inline in the panel, never page-level.
	let aliases = $state<PersonAlias[]>([]);
	let newAlias = $state('');
	let aliasBusy = $state(false);
	let aliasError = $state('');
	let aliasInput = $state<HTMLInputElement | null>(null);
	// Merge (F23): the picker for "merge another person in", and the collision prompt
	// shown when an added alias already names a different, existing person.
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

	const id = $derived(Number($page.params.id));
	const isOwner = $derived(activity.isOwner);
	// v1 enriches People from the first person-capable provider.
	const provider = $derived(sources.find((s) => s.entity_types.includes('person'))?.name ?? '');
	const enrichedByProvider = $derived(provider && enriched.some((f) => f.provider === provider));

	function load(current: number) {
		loading = true;
		error = '';
		api
			.getPerson(current)
			.then((res) => {
				person = res.person;
				videos = res.items ?? [];
				enriched = res.enriched ?? [];
				images = res.images ?? { roles: {}, gallery: [] };
				aliases = res.person.aliases ?? [];
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

	function onApplied(fields: EnrichedField[]) {
		enriched = fields;
		reloadImages(); // provider may have returned a photo — best-effort refresh
	}

	async function clearProvider() {
		if (!provider) return;
		busy = 'clear';
		actionError = '';
		try {
			await api.enrichClear(id, provider);
			// Clear removed exactly this provider's rows; drop them locally rather
			// than refetching the whole person (+ up to 500 videos).
			enriched = enriched.filter((f) => f.provider !== provider);
		} catch (e) {
			actionError = toMessage(e);
		} finally {
			busy = '';
		}
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
			load(id); // refresh so the new ?v= stamp busts the cache everywhere
		} catch (err) {
			imageError = toMessage(err);
		} finally {
			uploadBusy = '';
		}
	}

	// Re-read the image set after a gallery change without refetching 500 videos.
	async function reloadImages() {
		try {
			const res = await api.getPerson(id);
			images = res.images ?? { roles: {}, gallery: [] };
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
				<PersonBanner personId={id} name={person?.name ?? ''} version={roleVersion('banner')} eager />
				{@render editBtn('banner', 'right-2 top-2')}
				<div class="-mt-10 flex items-end gap-3 pl-3 sm:-mt-12">
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

			{#if aliases.length || isOwner}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-center justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted">Also known as</h2>
						{#if isOwner}
							<button
								onclick={() => (mergeOpen = true)}
								class="rounded-theme border border-rule px-3 py-1 text-sm text-ink hover:bg-surface-2"
							>
								Merge a person in…
							</button>
						{/if}
					</div>

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

			{#if enriched.length || (isOwner && provider)}
				<section class="space-y-3 rounded-theme border border-rule bg-surface p-4">
					<div class="flex flex-wrap items-start justify-between gap-2">
						<h2 class="text-xs uppercase tracking-wide text-muted">Enrichment</h2>
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

					{#if enriched.length}
						<dl class="grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
							{#each enriched as f (f.canonical + f.provider)}
								<div>
									<dt class="inline text-muted">{f.label}:</dt>
									{#if f.display === 'url'}
										<dd class="inline"><UrlValueList values={f.values} /></dd>
									{:else}
										<dd class="inline text-ink">{f.values.join(', ')}</dd>
									{/if}
									<ProvenanceBadge provider={f.provider} label={f.provider} />
								</div>
							{/each}
						</dl>
					{:else}
						<p class="text-sm text-muted">No enrichment yet.</p>
					{/if}

					{#if actionError}
						<p class="text-sm text-warn">{actionError}</p>
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
		onapplied={onApplied}
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
