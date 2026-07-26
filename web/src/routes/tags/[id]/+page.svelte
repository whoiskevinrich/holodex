<script lang="ts">
	import { page } from '$app/stores';
	import { api } from '$lib/api';
	import { toMessage } from '$lib/format';
	import type { Tag, Video } from '$lib/types';
	import AsyncState from '$lib/components/shared/AsyncState.svelte';
	import EntityVideos from '$lib/components/entity/EntityVideos.svelte';

	let tag = $state<Tag | null>(null);
	let videos = $state<Video[]>([]);
	let loading = $state(true);
	let error = $state('');

	const id = $derived(Number($page.params.id));

	$effect(() => {
		const current = id;
		loading = true;
		error = '';
		api
			.getTag(current)
			.then((res) => {
				tag = res.tag;
				videos = res.items ?? [];
			})
			.catch((e) => (error = toMessage(e)))
			.finally(() => (loading = false));
	});
</script>

<AsyncState {loading} error={error || (!tag ? 'Not found.' : '')}>
	<EntityVideos
		backHref="/tags"
		backLabel="All tags"
		name={tag?.name ?? ''}
		{videos}
		empty="No videos for this tag."
	/>
</AsyncState>
