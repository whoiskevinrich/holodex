<script lang="ts">
	// F44 (ADR-062): the shared inline editor that drives promote / edit / de-promote of a
	// non-canonical field (DD2 — one component for both). It renders in-flow as an
	// accent-bordered sub-form (DD1 — an inline expander, no popover): label / render mode /
	// group / order. Promote opens it empty (inherited label/render shown as placeholders);
	// Edit opens it pre-filled and adds a Remove-promotion action. Both commit via PUT
	// (api.promoteField); Remove issues the DELETE (api.unpromoteField). Tokens only; 3 skins.
	import { untrack } from 'svelte';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { PromotionEntityType, PromotionRender } from '$lib/types';

	let {
		entityType,
		fieldKey,
		mode,
		label = '',
		render = '',
		group = 'extended',
		order = 0,
		inheritedLabel = '',
		entityNoun,
		onchanged,
		onclose
	}: {
		entityType: PromotionEntityType;
		fieldKey: string;
		mode: 'promote' | 'edit';
		label?: string;
		render?: PromotionRender;
		group?: 'primary' | 'attributes' | 'extended';
		order?: number;
		// The tier-3/4 label shown as the Label placeholder in promote mode (empty ⇒ inherit).
		inheritedLabel?: string;
		// Pluralized entity noun for the global-scope caption (people / studios / videos).
		entityNoun: string;
		onchanged: () => Promise<void> | void;
		onclose: () => void;
	} = $props();

	// '' render means inline text; the <select> represents it as 'text' so the option is
	// visible, and we translate back to '' on save (the server coerces 'text' → '' anyway).
	// Seed the editable drafts from the props once (untrack signals the intentional
	// snapshot — the editor is remounted per open, so the props never change mid-life).
	let labelDraft = $state(untrack(() => label));
	let renderDraft = $state<'text' | 'long_text' | 'chips' | 'url' | 'image_url'>(
		untrack(() => (render === '' ? 'text' : render))
	);
	let groupDraft = $state<'primary' | 'attributes' | 'extended'>(untrack(() => group));
	let orderDraft = $state(untrack(() => order));
	let busy = $state(false);
	let error = $state('');

	let labelEl = $state<HTMLInputElement | null>(null);
	$effect(() => {
		// Opening the editor moves focus to the first field (a11y — the design's focus contract).
		labelEl?.focus();
	});

	// In edit mode, load the stored promotion so group/order (not carried on the resolved
	// field) pre-fill — otherwise a Save would reset them. Best-effort; keep the passed
	// label/render as the instant values on failure.
	$effect(() => {
		if (mode !== 'edit') return;
		let live = true;
		api
			.listFieldPromotions(entityType)
			.then((rows) => {
				if (!live) return;
				const row = rows.find((p) => p.field_key === fieldKey);
				if (!row) return;
				labelDraft = row.label ?? '';
				renderDraft = !row.render ? 'text' : row.render;
				groupDraft = row.group ?? 'extended';
				orderDraft = row.order ?? 0;
			})
			.catch(() => {});
		return () => {
			live = false;
		};
	});

	async function run(fn: () => Promise<unknown>) {
		busy = true;
		error = '';
		try {
			await fn();
			await onchanged();
			onclose();
		} catch (e) {
			error = toMessage(e);
			busy = false; // stay open on error so the owner can retry; nothing moved
		}
	}

	function save() {
		if (busy) return;
		run(() =>
			api.promoteField(entityType, fieldKey, {
				label: labelDraft.trim(),
				render: (renderDraft === 'text' ? '' : renderDraft) as PromotionRender,
				group: groupDraft,
				order: Number.isFinite(orderDraft) ? orderDraft : 0
			})
		);
	}
	function remove() {
		if (busy) return;
		run(() => api.unpromoteField(entityType, fieldKey));
	}
	function onKey(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			e.preventDefault();
			onclose();
		}
	}

	const scopeVerb = $derived(mode === 'promote' ? 'Promote' : 'Editing');
	const inputClass =
		'rounded-theme border border-rule bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent';
</script>

<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
<div
	class="mt-2 rounded-theme border border-accent bg-surface-2 p-3 sm:col-span-2"
	class:opacity-60={busy}
	aria-busy={busy}
	onkeydown={onKey}
	role="form"
>
	<p class="mb-2 text-xs text-muted">
		{scopeVerb} “{fieldKey}” — shared across all {entityNoun}
	</p>

	<div class="grid grid-cols-1 gap-2 sm:grid-cols-2">
		<label class="flex flex-col gap-0.5 text-xs text-muted sm:col-span-2">
			Label
			<input
				bind:this={labelEl}
				bind:value={labelDraft}
				maxlength="64"
				placeholder={inheritedLabel || fieldKey}
				class={inputClass}
			/>
		</label>

		<label class="flex flex-col gap-0.5 text-xs text-muted">
			Render mode
			<select bind:value={renderDraft} class={inputClass}>
				<option value="text">Text</option>
				<option value="long_text">Long text</option>
				<option value="chips">Chips (multi-value)</option>
				<option value="url">Link</option>
				<option value="image_url">Image</option>
			</select>
		</label>

		<label class="flex flex-col gap-0.5 text-xs text-muted">
			Group
			<select bind:value={groupDraft} class={inputClass}>
				<option value="primary">Primary</option>
				<option value="attributes">Attributes</option>
				<option value="extended">Extended</option>
			</select>
		</label>

		<label class="flex flex-col gap-0.5 text-xs text-muted">
			Order
			<input type="number" bind:value={orderDraft} class={`w-16 ${inputClass}`} />
		</label>
	</div>

	<div class="mt-2 flex items-center gap-2">
		{#if mode === 'edit'}
			<button
				type="button"
				onclick={remove}
				disabled={busy}
				class="text-xs text-warn hover:border-warn focus-visible:text-warn"
			>
				Remove promotion
			</button>
		{/if}
		<div class="ml-auto flex items-center gap-2">
			{#if error}
				<span class="text-xs text-warn" aria-live="polite">{error}</span>
			{/if}
			<button type="button" onclick={onclose} disabled={busy} class="text-xs text-muted hover:text-ink">
				Cancel
			</button>
			<button
				type="button"
				onclick={save}
				disabled={busy}
				class="rounded-theme bg-accent px-3 py-1 text-xs text-accent-ink"
			>
				{mode === 'promote' ? 'Promote' : 'Save'}
			</button>
		</div>
	</div>
</div>
