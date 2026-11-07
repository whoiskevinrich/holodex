<script lang="ts">
	// Edit definition (F89 P0-4, docs/design/smart-playlist-rules-handoff.md): a smart
	// playlist's query, sort and cap, edited on its own page over a live preview. Nothing is
	// stored until Save definition, through the same owner PATCH as every other save; a draft
	// is checked with that PATCH's dry run, so the rules live in one place (SEC-11).
	import { untrack } from 'svelte';
	import { beforeNavigate, goto } from '$app/navigation';
	import { api } from '#lib/api.ts';
	import { MEDIA_SORTS } from '#lib/filters.ts';
	import { filterLabels, loadScopeNames } from '#lib/filterLabels.ts';
	import { mediaSchema, type MediaQuery } from '#lib/listState.ts';
	import { parsedExpr } from '#lib/parsedExpr.svelte.ts';
	import {
		capReadout,
		chipAtFault,
		draftDirty,
		draftQueryString,
		missingProblem,
		toProblems,
		type DefinitionProblem
	} from '#lib/playlistDefinition.ts';
	import { printNames } from '#lib/query.ts';
	import type { Facet, Playlist, SortOrder } from '#lib/types.ts';
	import ConfirmDialog from '#lib/components/shared/ConfirmDialog.svelte';
	import CapSelect from '#lib/components/sort/CapSelect.svelte';
	import FilterChip from '#lib/components/sort/FilterChip.svelte';
	import FilterPanel from '#lib/components/sort/FilterPanel.svelte';
	import MediaFilterFields from '#lib/components/sort/MediaFilterFields.svelte';
	import QueryChips from '#lib/components/sort/QueryChips.svelte';
	import QueryField from '#lib/components/sort/QueryField.svelte';
	import SortDropdown from '#lib/components/sort/SortDropdown.svelte';
	import DefinitionPreview from './DefinitionPreview.svelte';

	let {
		playlist,
		savedCount,
		onclose,
		onsaved
	}: {
		playlist: Playlist;
		/** What the playlist holds now, for the readout's "(was n)". */
		savedCount: number;
		onclose: () => void;
		onsaved: (playlist: Playlist) => void;
	} = $props();

	// The draft, seeded once from what is stored.
	const storedQs = untrack(() => playlist.query ?? '');
	const storedParams = new URLSearchParams(storedQs);
	const seed = Math.floor(Math.random() * 2 ** 31);
	let query = $state<MediaQuery>(mediaSchema.parseQuery(storedParams));
	// missing_facet isn't list state any more (listState.ts), but a stored query can hold it.
	let missingFacets = $state(storedParams.getAll('missing_facet'));
	let sort = $state(untrack(() => playlist.sort) as SortOrder);
	let cap = $state(untrack(() => playlist.cap));
	const set = (patch: Partial<MediaQuery>) => (query = { ...query, ...patch });

	const draftQs = $derived(draftQueryString(query, missingFacets));
	const storedDraftQs = draftQueryString(mediaSchema.parseQuery(storedParams), storedParams.getAll('missing_facet'));
	let text = $state('');
	let appliedText = $state('');
	const dirty = $derived(
		draftDirty({ qs: draftQs, sort, cap }, { qs: storedDraftQs, sort: playlist.sort, cap: playlist.cap }, text, appliedText)
	);
	const signature = $derived(`${draftQs}|${sort}|${cap}`);

	// The query field and chips. The field shows names; the draft holds the canonical query.
	const expr = $derived(query.expr);
	const parsed = parsedExpr(() => expr, true);
	let textInvalid = $state(false);
	let field = $state<QueryField>();
	$effect(() => {
		const p = parsed.parsed;
		if (!expr) text = appliedText = '';
		else if (p?.ok) text = appliedText = printNames(p.tree);
	});

	// The other filters' chips, as on browse.
	let facets = $state<Facet[]>([]);
	let scopeNames = $state<Record<string, string>>({});
	$effect(() => loadScopeNames(query, scopeNames));
	const filterChips = $derived(filterLabels(query, scopeNames, (c) => facets.find((f) => f.canonical === c)?.label));
	const filterCount = $derived(filterChips.filter((c) => c.kind === 'filter').length);
	const chipCount = $derived(parsed.count + filterChips.length + missingFacets.length + (query.q ? 1 : 0));
	function clearAll() {
		query = mediaSchema.parseQuery(new URLSearchParams());
		missingFacets = [];
	}

	// Would this draft save? Checked as it changes, a beat after the last change.
	let refusals = $state<DefinitionProblem[]>([]);
	let checkSeq = 0;
	$effect(() => {
		const body = { query: draftQs, sort: sort as string, cap };
		void playlist.visibility; // switching to Public re-judges the draft at once
		const mine = ++checkSeq;
		const t = setTimeout(() => {
			api
				.checkPlaylist(playlist.id, body)
				.then(() => {
					if (mine === checkSeq) refusals = [];
				})
				.catch((e) => {
					if (mine === checkSeq) refusals = toProblems(e, parsed.chips);
				});
		}, 200);
		return () => clearTimeout(t);
	});
	const gone = $derived(missingProblem(parsed.chips));
	const problems = $derived(gone ? [gone] : refusals);

	let matches = $state<number | null>(null);
	const readout = $derived(
		matches == null ? '' : capReadout(matches, cap, sort === 'random', dirty ? savedCount : undefined)
	);

	let busy = $state(false);
	// Save: the field's text first, so what's typed is what's saved.
	async function save() {
		if (busy) return;
		busy = true;
		try {
			if (field && !(await field.apply())) return;
			const res = await api.updatePlaylist(playlist.id, {
				query: draftQueryString(query, missingFacets),
				sort,
				cap
			});
			leaving = true;
			onsaved(res.playlist);
		} catch (e) {
			refusals = toProblems(e, parsed.chips);
		} finally {
			busy = false;
		}
	}

	// Leaving with a changed draft asks first (P0-4): Cancel, a link, Back.
	let leaving = false;
	let confirmDiscard = $state(false);
	let pendingUrl: URL | null = null;
	beforeNavigate((nav) => {
		if (leaving || !dirty) return;
		nav.cancel(); // a page unload gets the browser's own prompt from this
		if (nav.willUnload) return;
		pendingUrl = nav.to?.url ?? null;
		confirmDiscard = true;
	});
	function cancel() {
		if (!dirty) return onclose();
		pendingUrl = null;
		confirmDiscard = true;
	}
	function discard() {
		leaving = true;
		confirmDiscard = false;
		if (pendingUrl) void goto(pendingUrl);
		else onclose();
	}
</script>

<div class="space-y-3">
	<div class="flex flex-wrap items-center justify-between gap-3 rounded-theme border border-rule bg-surface px-3 py-2">
		<p class="flex min-w-0 flex-wrap items-baseline gap-x-2">
			<span class="text-xs uppercase tracking-wide text-muted">Editing definition of</span>
			<span class="max-w-[24ch] truncate text-sm text-accent" title={playlist.name}>{playlist.name}</span>
		</p>
		<div class="ml-auto flex items-center gap-2">
			<button type="button" onclick={cancel} class="btn-quiet px-3 py-1.5 text-sm">Cancel</button>
			<button type="button" onclick={save} disabled={busy || problems.length > 0 || textInvalid} class="btn-accent px-3 py-1.5 text-sm">
				Save definition
			</button>
		</div>
	</div>

	<QueryField bind:this={field} bind:text bind:invalid={textInvalid} onapply={(c) => set({ expr: c || undefined })} />

	{#if chipCount > 0}
		<div class="flex items-center gap-1.5 overflow-x-auto sm:flex-wrap" data-list-chips>
			{#if query.q}
				<FilterChip label={`“${query.q}”`} onremove={() => set({ q: undefined })} />
			{/if}
			{#each filterChips.filter((c) => c.kind === 'scope') as c (c.id)}
				<FilterChip label={c.label} kind={c.kind} onremove={() => set(c.clear)} />
			{/each}
			<QueryChips
				model={parsed.chips}
				onremove={(path) => set({ expr: parsed.without(path) })}
				problem={(c) => chipAtFault(c, null) || problems.some((p) => chipAtFault(c, p))}
			/>
			{#each filterChips.filter((c) => c.kind !== 'scope') as c (c.id)}
				<FilterChip label={c.label} kind={c.kind} onremove={() => set(c.clear)} />
			{/each}
			{#each missingFacets as m (m)}
				<FilterChip label={`Missing: ${m}`} ownerOnly onremove={() => (missingFacets = missingFacets.filter((x) => x !== m))} />
			{/each}
			{#if chipCount >= 2}
				<button type="button" class="btn-quiet shrink-0 text-xs" onclick={clearAll}>Clear</button>
			{/if}
		</div>
	{/if}

	<div class="flex flex-wrap items-center gap-2">
		<SortDropdown compact owner bind:sort options={MEDIA_SORTS} />
		<CapSelect {cap} onchange={(c) => (cap = c)} />
		<FilterPanel count={filterCount} resultLabel={readout} onclear={clearAll}>
			<MediaFilterFields {query} {set} onfacets={(f) => (facets = f)} />
		</FilterPanel>
		<p class="text-xs text-muted" aria-live="polite">{busy ? 'Saving…' : readout}</p>
	</div>

	{#if problems.length > 0}
		<div aria-live="polite" class="space-y-2">
			{#each problems as problem (problem.sentence)}
				<div>
					<p class="text-sm text-warn">{problem.sentence}</p>
					{#if problem.fix}<p class="text-xs text-muted">{problem.fix}</p>{/if}
				</div>
			{/each}
		</div>
	{/if}

	{#if !parsed.refused && !gone}
		<p class="text-xs text-muted">{cap != null ? `Preview — the first ${cap.toLocaleString()}, in this sort` : 'Preview'}</p>
		<DefinitionPreview
			filters={{ ...query, missing_facet: missingFacets, sort, seed: sort === 'random' ? seed : undefined }}
			{cap}
			{signature}
			onmatch={(m) => (matches = m)}
		/>
	{/if}
</div>

{#if confirmDiscard}
	<ConfirmDialog
		title={`Discard changes to ${playlist.name}?`}
		confirmLabel="Discard"
		cancelLabel="Keep editing"
		onconfirm={discard}
		oncancel={() => (confirmDiscard = false)}
	>
		{#snippet body()}
			<p>Your edits to this definition haven’t been saved.</p>
		{/snippet}
	</ConfirmDialog>
{/if}
