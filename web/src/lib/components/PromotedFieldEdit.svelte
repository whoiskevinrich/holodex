<script lang="ts">
	// F44 (ADR-062): the owner-only Edit / Remove-promotion affordance on a promoted
	// (already first-class) field row, shared by all three detail pages. Mirrors the
	// Promote pill that AutoFieldRows owns for un-promoted rows — so both sides of the
	// promotion line use one component each, not per-page copies. Renders a small Edit
	// pill that opens the shared PromoteFieldEditor (edit mode) in-flow beneath the row.
	// The caller gates on `isOwner && field.promoted`; a visitor never sees this.
	import type { PromotionEntityType, ResolvedField } from '$lib/types';
	import PromoteFieldEditor from './PromoteFieldEditor.svelte';

	let {
		field,
		entityType,
		entityNoun,
		onchanged
	}: {
		field: ResolvedField;
		entityType: PromotionEntityType;
		entityNoun: string;
		onchanged: () => Promise<void> | void;
	} = $props();

	let open = $state(false);
</script>

<div class="-mt-1 sm:col-span-2">
	{#if open}
		<PromoteFieldEditor
			{entityType}
			fieldKey={field.canonical}
			mode="edit"
			label={field.label}
			render={field.display ?? ''}
			{entityNoun}
			{onchanged}
			onclose={() => (open = false)}
		/>
	{:else}
		<button
			type="button"
			onclick={() => (open = true)}
			aria-label={`Edit ${field.label} promotion`}
			class="inline-flex items-center gap-1 rounded-full border border-rule px-2 py-0.5 text-xs text-muted hover:border-accent hover:text-accent focus-visible:text-accent"
		>
			<svg class="h-3 w-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path stroke-linecap="round" stroke-linejoin="round" d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7"/><path stroke-linecap="round" stroke-linejoin="round" d="M18.5 2.5a2.12 2.12 0 013 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
			Edit
		</button>
	{/if}
</div>
