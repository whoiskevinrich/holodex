<script lang="ts">
	// Tag deny-list management (F50 S8, ADR-075 D2, P1-1) — mirrors owner/duplicates'
	// shell exactly (one bordered section, uppercase count heading, list rows below).
	// A denied term blocks a tag from being created, from any origin (scanner, manual
	// attach, materialization) — exact match, case-insensitive; it never retroactively
	// removes an existing tag of the same name.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { DeniedTag } from '$lib/types';

	let terms = $state<DeniedTag[]>([]);
	let loading = $state(true);
	let error = $state('');

	let newTerm = $state('');
	let adding = $state(false);
	let addError = $state('');
	// Set of denied term-keys (lowercased) that also match a live tag name — the
	// spec's forward-only caveat, surfaced under that row once discovered.
	let existingTagCaveats = $state<Set<string>>(new Set());

	let removing = $state<string | null>(null);
	let removeError = $state('');

	function load() {
		loading = true;
		error = '';
		api
			.deniedTags()
			.then((res) => (terms = res.terms ?? []))
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	}
	$effect(load);

	async function addTerm(e: SubmitEvent) {
		e.preventDefault();
		const term = newTerm.trim();
		if (!term || adding) return;
		adding = true;
		addError = '';
		try {
			const res = await api.denyTag(term);
			newTerm = '';
			load();
			// Forward-only caveat: the server reports whether this term already
			// names a live tag, so no follow-up fetch/scan is needed here.
			if (res.existing_tag) {
				existingTagCaveats = new Set(existingTagCaveats).add(term.toLowerCase());
			}
		} catch (err) {
			addError = toMessage(err);
		} finally {
			adding = false;
		}
	}

	async function removeTerm(term: string) {
		removing = term;
		removeError = '';
		try {
			await api.removeDeniedTag(term);
			terms = terms.filter((t) => t.term !== term);
		} catch (err) {
			removeError = toMessage(err);
		} finally {
			removing = null;
		}
	}
</script>

<div class="space-y-4">
	<p class="text-sm text-muted">
		Blocks a term from becoming a tag, from any source — exact match, case-insensitive.
		Denying <span class="font-semibold text-ink">Gnome</span> does not block
		<span class="font-semibold text-ink">Garden Gnome</span>.
	</p>

	<form onsubmit={addTerm} class="flex flex-wrap items-center gap-2">
		<input
			bind:value={newTerm}
			type="text"
			placeholder="Term to deny"
			aria-label="Term to deny"
			class="min-w-0 flex-1 rounded-theme border border-rule bg-surface px-3 py-1.5 text-sm text-ink focus:border-accent focus:outline-none"
		/>
		<button
			type="submit"
			disabled={adding || !newTerm.trim()}
			class="rounded-theme border border-warn px-3 py-1.5 text-sm text-warn hover:bg-warn/10 disabled:opacity-60"
		>
			Deny
		</button>
	</form>
	{#if addError}
		<p class="text-sm text-warn" role="alert">{addError}</p>
	{/if}

	<section class="rounded-theme border border-rule bg-surface">
		<h2 class="px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted">
			Denied terms · {terms.length}
		</h2>
		{#if loading}
			<p class="py-16 text-center text-sm text-muted">Loading…</p>
		{:else if error}
			<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
		{:else if terms.length === 0}
			<p class="py-16 text-center text-sm text-muted">No denied terms yet.</p>
		{:else}
			{#if removeError}
				<p class="px-3 pb-2 text-sm text-warn" role="alert">{removeError}</p>
			{/if}
			<ul>
				{#each terms as t (t.term)}
					<li class="border-t border-rule px-3 py-2 first:border-t-0">
						<div class="flex items-center justify-between gap-2">
							<span class="text-sm text-ink">{t.term}</span>
							<button
								type="button"
								onclick={() => removeTerm(t.term)}
								disabled={removing === t.term}
								class="btn-row btn-ghost px-2"
							>
								Remove
							</button>
						</div>
						{#if existingTagCaveats.has(t.term.toLowerCase())}
							<p class="mt-1 text-xs text-muted">
								Existing tags with this name aren't removed — this only blocks new/re-materialized
								tags.
							</p>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</section>
</div>
