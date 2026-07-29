<script lang="ts">
	// F49 (ADR-074): the inline editor behind an auto-registered row's "Attach to…" pill.
	// It attaches the row's provider key to a canonical field, so the key becomes a
	// candidate source of that field instead of rendering as its own display-only row —
	// the GH #178 duplicate-paragraph fix, done in-app rather than in YAML.
	//
	// Same shell as PromoteFieldEditor (accent-bordered inline expander, DD1 — not a
	// dialog), and it reuses that editor's inputClass verbatim. Three things it does that
	// the promote editor does not, each load-bearing:
	//
	//   DD2 — the target list is the entity type's EFFECTIVE field set from the server, not
	//         what this page rendered. Undecided empty fields never render, and they are
	//         exactly the targets an owner needs (a person's empty `bio` is missing from
	//         the page precisely when a provider's biography key is the only one on it).
	//   DD3 — a row can carry several providers but a claim is per (provider, key), so two
	//         or more providers get a checklist. Attaching both is right for a duplicated
	//         paragraph and wrong when `provA:rating` is an age certificate and
	//         `provB:rating` is a score; unchecking one leaves it auto-registering alone.
	//   DD4 — claims append at LOWEST precedence, so on a replace field the attached value
	//         usually disappears from view. The outcome sentence says which will happen
	//         before the owner commits.
	//
	// Tokens only; QA 3 skins.
	import { untrack } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { FieldTarget, PromotionEntityType } from '$lib/types';

	let {
		entityType,
		fieldKey,
		label,
		providers,
		providerValues,
		entityNoun,
		onclaimed,
		onchanged,
		onclose
	}: {
		entityType: PromotionEntityType;
		// The provider key being attached (an auto-registered row's canonical IS the key).
		fieldKey: string;
		label: string;
		// Every provider supplying this row, in row order — one claim each (DD3).
		providers: string[];
		// provider → that provider's value on this row, for the checklist captions.
		providerValues: Record<string, string>;
		// Pluralized entity noun for the global-scope caption (people / studios / videos).
		entityNoun: string;
		onclaimed: (info: { targetLabel: string; providers: string[] }) => void;
		onchanged: () => Promise<void> | void;
		onclose: () => void;
	} = $props();

	let targets = $state<FieldTarget[]>([]);
	let target = $state('');
	let picked = $state<string[]>(untrack(() => [...providers])); // all checked by default (DD3)
	let promoted = $state(false);
	let busy = $state(false);
	let error = $state('');

	let formEl = $state<HTMLElement | null>(null);
	let focused = false;
	$effect(() => {
		// Opening moves focus to the first control — the checklist if present, else the
		// target select (the design's focus contract). Queried rather than bound so the
		// two branches don't fight over one ref.
		//
		// targets.length is read deliberately, before the guard, so this re-runs when the
		// options land: on the first pass the select is still `disabled` (no options yet)
		// and a disabled control cannot take focus, so a single-provider row — the common
		// one, with no checklist to focus instead — would drop focus to <body>, taking the
		// Escape handler on this element with it. Latched so the retry can't steal focus
		// back from an owner who has already tabbed on.
		void targets.length;
		if (focused) return;
		const el = formEl?.querySelector<HTMLElement>('input[type="checkbox"], select');
		el?.focus();
		focused = !!el && document.activeElement === el;
	});

	// The picker's options, and — DD6 — whether this key currently holds an F44 promotion
	// that attaching would destroy (RD3, ADR-074 §D5; the server does it in the same
	// transaction). The promotion case is rare from here, since a promoted key normally
	// renders as its own first-class field rather than an auto-registered row, but silently
	// destroying it would be the worst way to find out. A failed targets load is the only
	// one worth reporting: without options there is nothing to attach to.
	$effect(() => {
		let live = true;
		Promise.all([api.listFieldTargets(entityType), api.listFieldPromotions(entityType).catch(() => [])])
			.then(([rows, promotions]) => {
				if (!live) return;
				// Sorted by label: the select is the whole field set (17 in the shipped video
				// example), and mapping order is not something the owner can scan by.
				targets = [...rows].sort((a, b) => a.label.localeCompare(b.label));
				target = targets[0]?.canonical ?? '';
				promoted = promotions.some((p) => p.field_key === fieldKey);
			})
			.catch((e) => {
				if (live) error = toMessage(e);
			});
		return () => {
			live = false;
		};
	});

	const chosen = $derived(targets.find((t) => t.canonical === target));

	function toggle(provider: string) {
		picked = picked.includes(provider)
			? picked.filter((p) => p !== provider)
			: [...picked, provider];
	}

	async function attach() {
		if (busy || !target || picked.length === 0) return;
		busy = true;
		error = '';
		// One request per provider. On any failure, refetch and report rather than
		// presenting a partial write as a success (§8).
		const ordered = providers.filter((p) => picked.includes(p));
		try {
			for (const provider of ordered) {
				await api.claimField(entityType, provider, fieldKey, target);
			}
		} catch (e) {
			error = toMessage(e);
			await onchanged();
			busy = false;
			return;
		}
		onclaimed({ targetLabel: chosen?.label ?? target, providers: ordered });
		await onchanged();
		onclose();
	}

	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onclose();
		}
	}

	const inputClass =
		'rounded-theme border border-rule bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent';
</script>

<!-- min-w-0 is load-bearing: a grid item defaults to min-width:auto, so without it the
     checklist's nowrap value caption widens the whole Details column and scrolls the page. -->
<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="mt-2 min-w-0 rounded-theme border border-accent bg-surface-2 p-3 sm:col-span-2"
	class:opacity-60={busy}
	aria-busy={busy}
	onkeydown={onKey}
	role="form"
	bind:this={formEl}
>
	<p class="mb-2 text-xs text-muted">
		Attach “{fieldKey}” to a field — shared across all {entityNoun}
	</p>

	{#if providers.length > 1}
		<!-- min-w-0 defeats the UA's `min-inline-size: min-content` on <fieldset>, which would
		     otherwise refuse to shrink and let a long value caption widen the whole column. -->
		<fieldset class="mb-2 min-w-0">
			<legend class="text-xs text-muted">Providers to attach</legend>
			<p class="text-xs text-warn">
				These providers share a key name, which is not proof they mean the same thing. Uncheck any
				that don't belong — it keeps its own row.
			</p>
			<div class="mt-1 space-y-0.5">
				{#each providers as p (p)}
					<label class="flex items-center gap-2 text-xs text-muted">
						<input
							type="checkbox"
							checked={picked.includes(p)}
							onchange={() => toggle(p)}
							class="accent-accent"
						/>
						<span class="text-ink">{p}</span>
						{#if providerValues[p]}
							<span class="min-w-0 flex-1 truncate">{providerValues[p]}</span>
						{/if}
					</label>
				{/each}
			</div>
		</fieldset>
	{/if}

	<label class="flex flex-col gap-0.5 text-xs text-muted">
		Attach to
		<select
			bind:value={target}
			class={inputClass}
			disabled={targets.length === 0}
		>
			{#each targets as t (t.canonical)}
				<option value={t.canonical}>{t.label}</option>
			{/each}
		</select>
	</label>

	{#if chosen}
		<p class="mt-2 text-xs text-muted">
			{#if chosen.merge}
				<span class="text-ink">{chosen.label}</span> merges its sources, so these values join the
				list right away.
			{:else}
				<span class="text-ink">{chosen.label}</span> shows one source at a time. This value joins
				as a candidate at the lowest precedence — if another source is already winning, pick this
				one from the field's source chip.
			{/if}
		</p>
	{/if}

	{#if promoted}
		<p class="mt-2 text-xs text-warn">
			“{fieldKey}” is promoted as <span class="font-medium">{label}</span>. Attaching removes that
			promotion.
		</p>
	{/if}

	<div class="mt-2 flex items-center justify-end gap-2">
		{#if error}
			<span class="text-xs text-warn" aria-live="polite">{error}</span>
		{/if}
		<button type="button" onclick={onclose} disabled={busy} class="text-xs text-muted hover:text-ink">
			Cancel
		</button>
		<button
			type="button"
			onclick={attach}
			disabled={busy || !target || picked.length === 0}
			class="rounded-theme bg-accent px-3 py-1 text-xs text-accent-ink"
		>
			Attach
		</button>
	</div>
</div>
