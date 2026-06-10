<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { Person, Video } from '$lib/types';
	import AsyncState from '$lib/components/AsyncState.svelte';
	import EntityVideos from '$lib/components/EntityVideos.svelte';

	let person = $state<Person | null>(null);
	let videos = $state<Video[]>([]);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(Number($page.params.id));

	$effect(() => {
		const current = id;
		loading = true;
		error = '';
		api
			.getPerson(current)
			.then((res) => {
				person = res.person;
				videos = res.items ?? [];
			})
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	});
</script>

<AsyncState {loading} error={error || (!person ? 'Not found.' : '')}>
	<EntityVideos
		backHref="/people"
		backLabel="All people"
		name={person?.name ?? ''}
		{videos}
		empty="No videos for this person."
	/>
</AsyncState>
