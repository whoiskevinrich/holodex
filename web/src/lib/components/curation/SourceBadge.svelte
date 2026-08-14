<script lang="ts">
	// Tier-2 per-field source-of-truth control (F56, ADR-051 extension). Owner-only,
	// replace-field only. At rest it renders exactly what a visitor sees — the resolved
	// value plus a ProvenanceBadge — with a click affordance layered onto the *same* badge
	// element (hover/focus ring only, no permanent chrome). Clicking expands the field
	// in place into the F36 CurationChip radio row; chip clicks STAGE a selection locally
	// (no `decide()` call) until the owner clicks Confirm.
	//
	// This is the structural fix for the RD6 pending-chip bug (HOLODEX-245): SourceSelect's
	// "select = commit immediately" design meant the already-"selected" implicit-winner chip
	// had no click that could confirm it (a same-key no-op guard). Here every Confirm calls
	// `decide` unconditionally for whatever is staged, including the pending chip — so
	// confirming the RD6 winner actually creates the standing decision. Deliberately NOT an
	// extension of SourceSelect.svelte (see the F56 design handoff's "Design-system fit"):
	// it builds its own local staged-selection state directly against CurationChip's radio
	// mode + f36.ts's sourceChips/resolveSelection. SourceSelect stays alive only for
	// Person's onadopt-intercepted name field (Tier-1, HOLODEX-269).
	//
	// No `onadopt` — Tier-2 never intercepts into a rename/collision flow; every Confirm
	// calls `decide` directly. Entity-agnostic like SourceSelect (`baselineKey`: 'file' for
	// videos, 'record' for persons/studios). Tokens only; QA 3 skins.
	import { tick, untrack } from 'svelte';
	import type { DecisionSource, ResolvedField, ResolvedValue } from '$lib/types';
	import { isProviderSource, outOfSync, providerOf, resolveSelection, sourceChips, type SourceChip } from '$lib/f36';
	import { toMessage } from '$lib/format';
	import { dismissable } from '$lib/actions/dismissable';
	import { expandedField } from '$lib/expandedField.svelte';
	import CurationChip from './CurationChip.svelte';
	import ProvenanceBadge from '../enrichment/ProvenanceBadge.svelte';

	let {
		field,
		decide,
		baselineKey = 'file'
	}: {
		field: ResolvedField;
		decide: (source: DecisionSource, manualValue?: string) => Promise<void>;
		baselineKey?: string;
	} = $props();

	const chips = $derived(sourceChips(field, baselineKey));
	// "2+ sources" means more than one distinct value/source is actually on offer — the
	// trailing Custom opener never counts on its own (nothing chosen yet), but a committed
	// manual literal does (it IS a second source alongside the baseline). Single-source
	// fields get no badge at all — nothing to decide (handoff "Single candidate source" row).
	const hasCustomValue = $derived(chips.find((c) => c.key === 'custom')?.value.trim() !== '');
	// A provider whose value agrees with the baseline folds into the baseline chip
	// (f36.ts sourceChips) rather than becoming its own row — so chip *count* alone
	// undercounts. chips[0] is always the anchored baseline chip; a folded agreement
	// pushes onto its `sources`, so length > 1 there also means "2+ sources on offer".
	const isMultiSource = $derived(
		chips.filter((c) => c.key !== 'custom').length > 1 || hasCustomValue || chips[0].sources.length > 1
	);

	// selection resolves the committed key + whether it's an RD6 implicit winner in one walk
	// (shared with SourceSelect via f36.ts). The badge's provider icon reflects whichever
	// chip is committed — real decision or RD6 winner alike, since the pending-ness is
	// invisible at rest (handoff: "renders exactly as ProvenanceBadge does today").
	const selection = $derived(resolveSelection(field, chips, baselineKey));
	const selectedChip = $derived(chips.find((c) => c.key === selection.key) ?? chips[0]);
	const badgeProvider = $derived(
		isProviderSource(selectedChip.decisionSource) ? providerOf(selectedChip.decisionSource) : ''
	);

	const expanded = $derived(expandedField.isOpen(field.canonical));

	function itemFor(chip: SourceChip): ResolvedValue {
		return { value: chip.value, sources: chip.sources, manual: chip.manual };
	}

	// stagedKey is the local, uncommitted selection while expanded — distinct from
	// SourceSelect's `pendingKey` (an optimistic override for an in-flight commit). Nothing
	// here ever hits the network until Confirm.
	let stagedKey = $state<string | null>(null);
	let stagedCustomValue = $state('');
	let busy = $state(false);
	let error = $state('');
	let editing = $state(false); // Custom inline input open
	let draft = $state('');
	let groupEl = $state<HTMLElement | null>(null);
	let badgeEl = $state<HTMLButtonElement | null>(null);

	// Seed/clear staged state whenever this field's expansion toggles — including when a
	// sibling SourceBadge takes over the single expanded slot (F56.9). An abandoned expand
	// is a no-op by construction: nothing was ever committed, so there's nothing to undo.
	$effect(() => {
		if (expanded) {
			// untrack: seed only from the expand toggle, not from selection/field.decision —
			// otherwise a same-page reloadDetail() elsewhere (replacing this field's prop
			// object) re-fires this effect while still expanded and clobbers a staged pick.
			untrack(() => {
				stagedKey = selection.key;
				stagedCustomValue = field.decision?.source === 'manual' ? (field.decision.manual_value ?? '') : '';
			});
		} else {
			stagedKey = null;
			stagedCustomValue = '';
			editing = false;
			error = '';
			busy = false;
		}
	});

	function open() {
		expandedField.expand(field.canonical);
	}
	function close(returnFocus = true) {
		expandedField.close();
		if (returnFocus) badgeEl?.focus();
	}

	function focusChip(key: string) {
		groupEl?.querySelector<HTMLElement>(`[data-seg="${key}"]`)?.focus();
	}

	// Stage a chip now (click / Space / Enter) — never commits. The Custom chip opens the
	// inline input instead; its own commit (Enter/blur) stages the typed literal.
	function stage(chip: SourceChip) {
		if (chip.key === 'custom') {
			startCustom();
			return;
		}
		stagedKey = chip.key;
	}

	function startCustom() {
		draft = stagedKey === 'custom' ? stagedCustomValue : (field.decision?.manual_value ?? field.values[0] ?? '');
		editing = true;
	}
	function commitCustomDraft() {
		const v = draft.trim();
		editing = false;
		if (!v) return; // nothing typed — leave whatever was staged before untouched
		stagedCustomValue = v;
		stagedKey = 'custom';
	}
	async function cancelCustomEdit() {
		editing = false;
		await tick(); // let the chip button re-render before moving focus back onto it (a11y)
		focusChip('custom');
	}
	function onCustomKey(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			commitCustomDraft();
		} else if (e.key === 'Escape') {
			e.preventDefault();
			cancelCustomEdit();
		}
	}

	// Roving radiogroup arrow keys move focus AND stage (no debounced auto-commit — nothing
	// commits without Confirm, so there's no network round-trip to debounce). Space/Enter
	// stage the focused chip immediately.
	function onGroupKey(e: KeyboardEvent) {
		const focused = (e.target as HTMLElement | null)?.closest?.('[data-seg]') as HTMLElement | null;
		const i = chips.findIndex((c) => c.key === focused?.dataset.seg);
		if (i < 0) return; // key came from the inline input (no data-seg) — leave it alone
		const n = chips.length;
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown' ? 1 : e.key === 'ArrowLeft' || e.key === 'ArrowUp' ? -1 : 0;
		if (delta) {
			e.preventDefault();
			const target = chips[(i + delta + n) % n];
			focusChip(target.key);
			if (target.key === 'custom') {
				stagedKey = 'custom';
				startCustom();
			} else stagedKey = target.key;
		} else if (e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			stage(chips[i]);
		}
	}

	// Confirm: the only place a decision actually commits (F56's whole point). Fires for
	// whatever is staged, including the RD6 pending chip — this is the HOLODEX-245 fix.
	async function confirm() {
		const chip = chips.find((c) => c.key === stagedKey);
		if (!chip || busy) return;
		busy = true;
		error = '';
		try {
			await decide(chip.decisionSource, chip.key === 'custom' ? stagedCustomValue : undefined);
			// A sibling badge may have taken over the expanded slot while this request was
			// in flight (F56.9) — only collapse/refocus if this field is still the one open,
			// so a stale resolve doesn't steal focus from wherever the owner is now.
			if (expandedField.isOpen(field.canonical)) close();
		} catch (e) {
			error = toMessage(e); // stays expanded, staged selection intact for retry
		} finally {
			busy = false;
		}
	}

	function onDismiss(viaEscape: boolean) {
		// dismissable's window-level capture-phase Escape listener runs (and stops
		// propagation) before the Custom input's own onCustomKey ever sees the keydown —
		// so when the inline input is open, Escape here means "cancel just the input",
		// matching onCustomKey/cancelCustomEdit's documented intent, not "collapse the field".
		if (viaEscape && editing) {
			cancelCustomEdit();
			return;
		}
		// Escape returns focus to the badge (keyboard-close); an outside click leaves focus
		// where the pointer went (dismissable's own contract — mirrors SourceSelect/EnrichProviderChips).
		close(viaEscape);
	}
</script>

<div class="inline-flex flex-wrap items-center gap-2" data-source-badge={field.canonical}>
	{#if !isMultiSource}
		<span class={field.values.join(', ') ? 'text-ink' : 'text-muted'}>{field.values.join(', ') || '—'}</span>
	{:else}
		<span class={field.values.join(', ') ? 'text-ink' : 'text-muted'}>{field.values.join(', ') || '—'}</span>
		<button
			type="button"
			bind:this={badgeEl}
			aria-expanded={expanded}
			aria-label={`${field.label} — from ${badgeProvider || (selectedChip.manual ? 'a custom value' : baselineKey)}, click to change source`}
			onclick={() => {
				if (busy) return; // a Confirm is in flight — don't collapse mid-request (F56 Open Questions)
				expanded ? close() : open();
			}}
			class="inline-flex rounded-full align-middle transition-colors duration-150 hover:ring-1 hover:ring-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
		>
			<ProvenanceBadge provider={badgeProvider} label={badgeProvider} manual={selectedChip.manual} />
		</button>
	{/if}

	{#if expanded}
		<div
			class="mt-1 flex w-full flex-wrap items-center gap-2"
			class:opacity-60={busy}
			aria-busy={busy}
			use:dismissable={{
				enabled: expanded && !busy,
				inside: `[data-source-badge="${field.canonical}"]`,
				onclose: onDismiss
			}}
		>
			<!-- Roving-tabindex radiogroup — identical shell to SourceSelect (CurationChip's
			     existing radio mode), but selecting a chip stages, it never commits. -->
			<div
				bind:this={groupEl}
				role="radiogroup"
				aria-label={`Source of truth for ${field.label}`}
				tabindex={-1}
				class="flex flex-wrap items-center gap-1.5"
				onkeydown={onGroupKey}
			>
				{#each chips as chip (chip.key)}
					{#if chip.key === 'custom'}
						{#if editing}
							<!-- svelte-ignore a11y_autofocus -->
							<input
								bind:value={draft}
								onkeydown={onCustomKey}
								onblur={commitCustomDraft}
								autofocus
								aria-label={`Custom value for ${field.label}`}
								placeholder="Custom value…"
								class="w-32 rounded-full border border-accent bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent"
							/>
						{:else if stagedKey === 'custom' || chip.value}
							<!-- Staged/decided manual literal: a value chip (·manual) that re-opens the editor. -->
							<CurationChip
								item={stagedKey === 'custom'
									? { value: stagedCustomValue, sources: ['manual'], manual: true }
									: itemFor(chip)}
								isOwner={false}
								radio={{
									key: 'custom',
									checked: stagedKey === 'custom',
									tabindex: stagedKey === 'custom' ? 0 : -1,
									onselect: () => stage(chip)
								}}
							/>
						{:else}
							<!-- Opener: choosing it opens the inline input; on commit it becomes the chip above. -->
							<button
								type="button"
								role="radio"
								data-seg="custom"
								aria-checked={stagedKey === 'custom'}
								tabindex={stagedKey === 'custom' ? 0 : -1}
								aria-label={`Set a custom value for ${field.label}`}
								onclick={() => stage(chip)}
								class="curation-chip inline-flex items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-muted hover:text-ink focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent"
							>
								<svg
									class="h-3 w-3 shrink-0"
									viewBox="0 0 24 24"
									fill="none"
									stroke="currentColor"
									stroke-width="2"
									aria-hidden="true"
								>
									<path stroke-linecap="round" stroke-linejoin="round" d="M12 5v14M5 12h14" />
								</svg>
								Custom
							</button>
						{/if}
					{:else}
						<CurationChip
							item={itemFor(chip)}
							isOwner={false}
							radio={{
								key: chip.key,
								checked: stagedKey === chip.key,
								tabindex: stagedKey === chip.key ? 0 : -1,
								onselect: () => stage(chip),
								pending: selection.pending && stagedKey === selection.key
							}}
						/>
					{/if}
				{/each}
			</div>

			<button type="button" class="btn-row btn-pill btn-accent" disabled={busy} onclick={confirm}>Confirm</button>
			<button type="button" class="btn-row btn-ghost px-2" disabled={busy} onclick={() => close()}>Cancel</button>

			{#if error}
				<p class="w-full text-xs text-warn" aria-live="polite">{error}</p>
			{/if}
		</div>
	{/if}

	{#if outOfSync(field)}
		<span
			class="inline-block rounded-full border border-warn px-2 py-0.5 text-[0.65rem] text-warn"
			aria-label={`${field.label} is out of sync with the file`}
		>
			file out of sync
		</span>
	{/if}
</div>
