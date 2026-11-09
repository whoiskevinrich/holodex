<script lang="ts">
	// Owner → File names (F86, HOLODEX-602): the Renaming card above the rename plan. Off shows the
	// card alone; the plan is computed only from Plan up (docs/design/file-rename-handoff.md).
	import { onMount } from 'svelte';
	import { api } from '#lib/api.ts';
	import { toMessage } from '#lib/format.ts';
	import {
		parseCharacters,
		parseMode,
		type CharacterTable,
		type RenameMode,
		type RenamePlan as Plan,
		type RenamePreview
	} from '#lib/rename.ts';
	import RenamingCard from '#lib/components/rename/RenamingCard.svelte';
	import RenamePlan from '#lib/components/rename/RenamePlan.svelte';

	let loaded = $state(false);
	let error = $state('');
	let mode = $state<RenameMode>('off');
	let settleDays = $state(7);
	let saved = $state<string[]>([]);
	let savedCharacters = $state<CharacterTable>({});
	let saveLabel = $state('Save templates');
	let latest = $state<Plan | null>(null);
	let planView = $state<RenamePlan>();
	let card = $state<RenamingCard>();
	// Whether the plan shows an unsaved draft, so Discard reloads only then.
	let previewing = false;

	async function loadSettings() {
		error = '';
		try {
			const s = await api.adminSettings();
			mode = parseMode(s['rename.mode']);
			settleDays = Number(s['rename.settle_days']) || 7;
			try {
				const t = JSON.parse(s['rename.templates'] ?? '[]');
				saved = Array.isArray(t) ? t : [];
			} catch {
				saved = [];
			}
			savedCharacters = parseCharacters(s['rename.characters']);
			loaded = true;
		} catch (e) {
			error = toMessage(e);
		}
	}

	onMount(loadSettings);

	async function preview(draft: RenamePreview) {
		await planView?.reload(draft);
		previewing = true;
	}

	function showSaved() {
		previewing = false;
		planView?.reload(null);
	}

	function onsaved(wrote: { templates?: string[]; characters?: CharacterTable }) {
		if (wrote.templates) saved = wrote.templates;
		if (wrote.characters) savedCharacters = wrote.characters;
		showSaved();
	}
</script>

<div class="space-y-5">
	<p class="max-w-2xl text-sm text-muted">
		Give the files on disk names built from your metadata. A name stays in its own folder and keeps its extension. Nothing
		on disk changes until you rename.
	</p>

	{#if error}
		<p class="py-16 text-center text-sm text-warn" role="alert">
			Couldn't load the renaming settings. <button type="button" class="text-accent hover:underline" onclick={loadSettings}>Retry</button>
		</p>
	{:else if !loaded}
		<p class="py-16 text-center text-sm text-muted">Loading…</p>
	{:else}
		<RenamingCard
			bind:this={card}
			{mode}
			{settleDays}
			{saved}
			{savedCharacters}
			bind:saveLabel
			providers={latest?.providers ?? []}
			counted={latest ? { templates: latest.templates, counts: latest.template_counts } : null}
			onmode={(m) => (mode = m)}
			onsettle={(n) => (settleDays = n)}
			onpreview={preview}
			ondiscard={() => previewing && showSaved()}
			{onsaved}
		/>
		{#if mode !== 'off'}
			<RenamePlan bind:this={planView} {mode} onloaded={(p) => (latest = p)} onsavetemplates={() => card?.save()} {saveLabel} />
		{/if}
	{/if}
</div>
