<script lang="ts">
	// Per-field source-of-truth control (F36, ADR-051; HOLODEX-112 refinement). Owner-only,
	// replace-field only. One row of source-tagged, single-select value chips (a radiogroup):
	//   • the file baseline first, anchored, always tagged ·file (the file-first mental model is
	//     the whole point of F36 — the baseline never becomes just another value in the row);
	//   • one chip per DISTINCT candidate value, tagged with every source that supplies it — a
	//     provider that agrees with the file folds into the file chip (·file + tmdb), so an agreed
	//     value is never shown twice (the old wart);
	//   • a trailing Custom chip: the frozen manual literal when decided, else the inline-input opener;
	//   • a trailing "file out of sync" warn pill when the decided value differs from the file's tag
	//     (RD2 — the single text-warn signal per field).
	// The SELECTED chip *is* the resolved value, so divergence is self-evident (two value chips): the
	// old separate resolved chip, segmented control, candidates line, and "providers differ" hint are
	// all gone. Replace chips lead with a ● radio dot (pick one); merge fields keep the ✕-per-chip
	// CurationChip (drop any) — one shared shell, a different glyph per selection model.
	// Changing a source is a DB-only decision (RD5): it calls `decide`, never a file write. The file
	// is touched only by the header "Write decisions to file" action. Tokens only; QA 3 skins.
	// Entity-agnostic: it knows only `field` + `decide` (+ the entity's `baselineKey` — 'file' for
	// videos, 'record' for persons, F37 RD4), so People/Studio fast-follows reuse it as-is.
	// The optional `onadopt` interceptor serves identity fields (F37 RD1 — the person name):
	// selecting a non-baseline chip then calls `onadopt` INSTEAD of `decide` (no decision is ever
	// written), letting the page open a confirm flow (rename). Selection stays at rest on the
	// committed chip — nothing changes until the page's flow lands and the detail refetches.
	import { tick } from 'svelte';
	import type { DecisionSource, ResolvedField } from '$lib/types';
	import { chipToResolvedValue, outOfSync, resolveSelection, sourceChips, type SourceChip } from '$lib/f36';
	import { toMessage } from '$lib/format';
	import CurationChip from './CurationChip.svelte';

	let {
		field,
		decide,
		baselineKey = 'file',
		groupLabel,
		onadopt
	}: {
		field: ResolvedField;
		// Persist a decision and refetch the detail (baseline ⇒ page maps it to clear-or-pin;
		// manual ⇒ literal). Owned by the page so this control stays free of api/transport
		// concerns (cf. EnrichPicker). Never called while `onadopt` is set.
		decide: (source: DecisionSource, manualValue?: string) => Promise<void>;
		// The entity's baseline source key: 'file' (default, videos) or 'record' (persons).
		baselineKey?: string;
		// Radiogroup aria-label override (the name row announces its rename consequence).
		groupLabel?: string;
		// Intercept mode (F37 RD1): when set, activating a non-baseline chip (or committing the
		// Custom input) invokes this with the candidate source + value instead of deciding.
		onadopt?: (source: DecisionSource, value: string) => void;
	} = $props();

	const chips = $derived(sourceChips(field, baselineKey));
	// selection resolves the committed key + whether it's an RD6 implicit winner in one walk
	// (HOLODEX-245) — committedKey is the server's stored decision as a chip key (a manual
	// decision maps to the Custom chip). pendingKey is an optimistic override while a selection
	// settles, so the dot + aria-checked track the arrow/click immediately (QA 3.14) before the
	// debounced commit + refetch land. The effect below clears it once the server has caught up.
	const selection = $derived(resolveSelection(field, chips, baselineKey));
	const committedKey = $derived(selection.key);
	let pendingKey = $state<string | null>(null);
	const selectedKey = $derived(pendingKey ?? committedKey);
	// isPending: the selected chip is an RD6 implicit winner (empty baseline, no standing
	// decision) rather than a real decision — the chip renders distinctly for this. Only
	// meaningful while no optimistic override is in flight; a mid-selection pendingKey is never
	// itself the RD6 case (arrowing/clicking always targets a real chip to decide).
	const isPending = $derived(pendingKey === null && selection.pending);

	let busy = $state(false);
	let error = $state('');
	let editing = $state(false); // Custom inline input open
	let draft = $state('');
	let groupEl = $state<HTMLElement | null>(null);
	let commitTimer: ReturnType<typeof setTimeout> | undefined;

	// Drop the optimistic override once the server's committed decision matches it (after the
	// commit + refetch). Idempotent, so a fresh selection mid-flight is never clobbered.
	$effect(() => {
		if (pendingKey !== null && pendingKey === committedKey) pendingKey = null;
	});

	function commitDecision(source: DecisionSource, manualValue?: string) {
		clearTimeout(commitTimer);
		busy = true;
		error = '';
		decide(source, manualValue)
			.catch((e) => {
				error = toMessage(e);
				pendingKey = null; // revert the optimistic selection on failure
			})
			.finally(() => {
				busy = false;
			});
	}

	function focusChip(key: string) {
		groupEl?.querySelector<HTMLElement>(`[data-seg="${key}"]`)?.focus();
	}

	// Activate a chip now (click / Space / Enter): the Custom chip opens the inline input (its commit
	// decides); a baseline/provider chip commits immediately. Re-selecting the current source is a
	// no-op. Focus already sits on the activated chip, so this never moves focus.
	// Intercept mode (RD1): a non-baseline activation hands off to `onadopt` — no decision, no
	// optimistic selection (the confirm flow owns what happens next).
	function activate(chip: SourceChip) {
		clearTimeout(commitTimer);
		if (chip.key === 'custom') {
			if (!onadopt) pendingKey = 'custom';
			startCustom();
			return;
		}
		if (chip.key === committedKey) {
			pendingKey = null;
			return;
		}
		if (onadopt) {
			onadopt(chip.decisionSource, chip.value);
			return;
		}
		pendingKey = chip.key;
		commitDecision(chip.decisionSource);
	}

	// Roving radiogroup arrow keys move focus + selection optimistically (native radio semantics,
	// QA 3.14), but the network commit is debounced — arrowing across chips issues ONE decision for
	// the settled chip, not one per keypress (cf. EnrichPicker's debounce). Space/Enter commit the
	// focused chip immediately. One handler on the group reads the focused chip via data-seg, so it
	// covers both the CurationChip radios and the bespoke Custom chip without per-chip wiring.
	function onGroupKey(e: KeyboardEvent) {
		const focused = (e.target as HTMLElement | null)?.closest?.('[data-seg]') as HTMLElement | null;
		const i = chips.findIndex((c) => c.key === focused?.dataset.seg);
		if (i < 0) return; // key came from the inline input (no data-seg) — leave it alone
		const n = chips.length;
		const delta =
			e.key === 'ArrowRight' || e.key === 'ArrowDown'
				? 1
				: e.key === 'ArrowLeft' || e.key === 'ArrowUp'
					? -1
					: 0;
		if (delta) {
			e.preventDefault();
			const target = chips[(i + delta + n) % n];
			if (onadopt) {
				// Intercept mode (RD1): arrows rove focus only — selection never follows focus,
				// because nothing is decided until the page's confirm flow lands. Space/Enter
				// (or click) activates the focused chip and opens that flow.
				focusChip(target.key);
				return;
			}
			pendingKey = target.key; // selection follows focus immediately
			focusChip(target.key);
			clearTimeout(commitTimer);
			if (target.key === 'custom') {
				startCustom();
			} else {
				commitTimer = setTimeout(() => {
					if (target.key !== committedKey) commitDecision(target.decisionSource);
					else pendingKey = null;
				}, 250);
			}
		} else if (e.key === ' ' || e.key === 'Enter') {
			e.preventDefault();
			activate(chips[i]);
		}
	}

	function startCustom() {
		draft = field.decision?.manual_value ?? field.values[0] ?? '';
		editing = true;
	}
	function commitCustom() {
		const v = draft.trim();
		editing = false;
		if (!v) {
			cancelCustom();
			return;
		}
		if (onadopt) {
			// Intercept mode (RD1): a committed custom value routes into the confirm flow
			// (e.g. the rename dialog with the typed name) — never a manual decision.
			onadopt('manual', v);
			return;
		}
		commitDecision('manual', v);
	}
	async function cancelCustom() {
		editing = false;
		pendingKey = null; // drop the optimistic 'custom' selection — no decision was made
		await tick(); // let the chip button re-render before we move focus back onto it (a11y 3.15)
		focusChip('custom'); // Escape/empty returns focus to the Custom chip (handoff a11y)
	}
	function onCustomKey(e: KeyboardEvent) {
		if (e.key === 'Enter') {
			e.preventDefault();
			commitCustom();
		} else if (e.key === 'Escape') {
			e.preventDefault();
			cancelCustom();
		}
	}

	// The bespoke Custom-opener chip shares the CurationChip radio shell so the row reads as one
	// system; it carries a + glyph instead of a value until a literal is committed. It renders only
	// while there is no manual decision, so it never shows the selected state — hence a static idle
	// class (the frozen literal, once set, is a CurationChip radio that owns the selected styling).
	const openerCls =
		'curation-chip inline-flex items-center gap-1.5 rounded-full border border-rule bg-surface-2 px-2 py-0.5 text-xs text-muted hover:text-ink focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-accent';
</script>

<div class="mt-1 flex flex-wrap items-center gap-2" class:opacity-60={busy} aria-busy={busy}>
	<!-- Roving-tabindex radiogroup: the group itself is not a tab stop (tabindex -1); the checked
	     chip is (QA 3.14). One keydown handler serves every chip via data-seg. -->
	<div
		bind:this={groupEl}
		role="radiogroup"
		aria-label={groupLabel ?? `Source of truth for ${field.label}`}
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
						onblur={commitCustom}
						autofocus
						aria-label={`Custom value for ${field.label}`}
						placeholder="Custom value…"
						class="w-32 rounded-full border border-accent bg-bg px-2 py-0.5 text-xs text-ink placeholder-muted focus:outline-none focus:ring-1 focus:ring-accent"
					/>
				{:else if chip.value}
					<!-- Decided manual literal: a value chip (·manual) that re-opens the editor on select. -->
					<CurationChip
						item={chipToResolvedValue(chip)}
						isOwner={false}
						radio={{
							key: 'custom',
							checked: selectedKey === 'custom',
							tabindex: selectedKey === 'custom' ? 0 : -1,
							onselect: () => activate(chip)
						}}
					/>
				{:else}
					<!-- Opener: choosing it opens the inline input; on commit it becomes the chip above. -->
					<button
						type="button"
						role="radio"
						data-seg="custom"
						aria-checked={selectedKey === 'custom'}
						tabindex={selectedKey === 'custom' ? 0 : -1}
						aria-label={`Set a custom value for ${field.label}`}
						onclick={() => activate(chip)}
						class={openerCls}
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
					item={chipToResolvedValue(chip)}
					isOwner={false}
					radio={{
						key: chip.key,
						checked: selectedKey === chip.key,
						tabindex: selectedKey === chip.key ? 0 : -1,
						onselect: () => activate(chip),
						pending: isPending
					}}
				/>
			{/if}
		{/each}
	</div>

	{#if outOfSync(field)}
		<span
			class="inline-block rounded-full border border-warn px-2 py-0.5 text-[0.65rem] text-warn"
			aria-label={`${field.label} is out of sync with the file`}
		>
			file out of sync
		</span>
	{/if}

	{#if error}
		<p class="w-full text-xs text-warn" aria-live="polite">{error}</p>
	{/if}
</div>
