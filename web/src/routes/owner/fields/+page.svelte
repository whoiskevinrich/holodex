<script lang="ts">
	// Attached keys (F49 FR8, ADR-074 · handoff §3). Lists the provider keys the owner has
	// attached to a Holodex field, grouped by entity type, each removable in one click.
	//
	// The page exists because a claim is invisible by construction: it succeeds by deleting
	// the row that was its own evidence. Once the confirmation strip on the entity page is
	// gone, an attachment made last month has no surface at all — not on the page (the row
	// is suppressed), not in the promotions list (different table), and not in YAML for
	// person or studio (no such file). This is that surface.
	//
	// It also renders DD9's Inactive marker: ADR-074 §D4 keeps a claim whose target field
	// no longer exists forever and inert, and this is the only place it can be seen. The
	// check is client-side against the targets response the picker already uses — no
	// backend work, no extra request. Tokens only; QA 3 skins.
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { FieldClaim, FieldTarget, PromotionEntityType } from '$lib/types';

	type Group = { type: PromotionEntityType; label: string; claims: FieldClaim[] };

	const order: Array<{ type: PromotionEntityType; label: string }> = [
		{ type: 'video', label: 'Video' },
		{ type: 'person', label: 'People' },
		{ type: 'studio', label: 'Studios' }
	];

	let groups = $state<Group[]>([]);
	let targets = $state<Record<string, FieldTarget[]>>({});
	let loading = $state(true);
	let error = $state('');
	let removeError = $state('');
	let removing = $state<string[]>([]);

	const rowKey = (t: PromotionEntityType, c: FieldClaim) => `${t}/${c.provider}/${c.field_key}`;

	// Six cheap keyed lookups, issued together. Partial failure fails the page: a list that
	// silently omits an entity type lies about what is attached, which is worse than no list.
	async function load() {
		loading = true;
		error = '';
		try {
			const [claims, found] = await Promise.all([
				Promise.all(order.map((o) => api.listFieldClaims(o.type))),
				Promise.all(order.map((o) => api.listFieldTargets(o.type)))
			]);
			groups = order
				.map((o, i) => ({ ...o, claims: claims[i] ?? [] }))
				.filter((g) => g.claims.length > 0);
			targets = Object.fromEntries(order.map((o, i) => [o.type, found[i] ?? []]));
		} catch (e) {
			error = toMessage(e);
		} finally {
			loading = false;
		}
	}
	$effect(() => {
		load();
	});

	// The target's label, or null when the target no longer exists — DD9's inert claim.
	// Matched case-insensitively, the way the server matches it: a claim's canonical is
	// lower-cased on write, while a target's comes verbatim from the mapping, which only
	// trims (mapping.go) and is compared with EqualFold everywhere it is used. A YAML
	// field declared `canonical: Overview` would otherwise mark a live attachment Inactive.
	function targetOf(type: PromotionEntityType, canonical: string): FieldTarget | null {
		const want = canonical.toLowerCase();
		return (targets[type] ?? []).find((t) => t.canonical.toLowerCase() === want) ?? null;
	}

	async function remove(group: Group, claim: FieldClaim) {
		const key = rowKey(group.type, claim);
		if (removing.includes(key)) return;
		removing = [...removing, key];
		removeError = '';
		try {
			await api.unclaimField(group.type, claim.provider, claim.field_key);
			// Matched by key, never by reference: a removal that resolved while this one was
			// in flight has already replaced every group object, so the ones this call closed
			// over are no longer the ones in the array.
			groups = groups
				.map((g) =>
					g.type === group.type
						? { ...g, claims: g.claims.filter((c) => rowKey(g.type, c) !== key) }
						: g
				)
				.filter((g) => g.claims.length > 0);
		} catch (e) {
			removeError = toMessage(e);
		} finally {
			removing = removing.filter((k) => k !== key);
		}
	}
</script>

<div class="space-y-5">
	<p class="text-sm text-muted">
		Provider keys you've attached to a Holodex field. An attached key stops showing as its own
		row — its value becomes a candidate of the field instead. Removing an attachment brings the
		row back.
	</p>

	{#if loading}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">{error}</p>
	{:else if groups.length === 0}
		<p class="py-16 text-center text-sm text-muted">
			No attached keys yet. Attach one from a provider row on any video, person or studio page.
		</p>
	{:else}
		{#if removeError}
			<p class="text-sm text-warn" aria-live="polite">{removeError}</p>
		{/if}
		{#each groups as g (g.type)}
			<section class="space-y-0 rounded-theme border border-rule bg-surface">
				<h2 class="px-3 pb-2 pt-3 text-xs uppercase tracking-wide text-muted">
					{g.label} · {g.claims.length}
				</h2>
				{#each g.claims as c (c.provider + '/' + c.field_key)}
					{@const target = targetOf(g.type, c.canonical)}
					{@const busy = removing.includes(rowKey(g.type, c))}
					<div
						class="flex flex-wrap items-center gap-x-3 gap-y-2 border-t border-rule px-3 py-2.5 text-sm"
						class:opacity-60={busy}
					>
						<span class="font-mono text-ink">{c.provider}:{c.field_key}</span>
						<span class="text-muted" aria-hidden="true">→</span>
						{#if target}
							<span class="text-ink">{target.label}</span>
							<span class="font-mono text-xs text-muted">{c.canonical}</span>
						{:else}
							<span class="font-mono text-xs text-muted">{c.canonical}</span>
							<span class="text-warn">
								Inactive — target field no longer exists, this attachment does nothing.
							</span>
						{/if}
						<button
							type="button"
							onclick={() => remove(g, c)}
							disabled={busy}
							aria-label={`Remove attachment of ${c.provider}:${c.field_key} from ${target?.label ?? c.canonical}`}
							class="btn-ghost ml-auto px-2 py-1 text-xs"
						>
							Remove
						</button>
					</div>
				{/each}
			</section>
		{/each}
	{/if}
</div>
