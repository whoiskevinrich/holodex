<script lang="ts">
	// Held re-auth banner (HOLODEX-502): shown under the header when the upstream auth
	// proxy session lapsed and api.ts held the reload because it would lose something
	// (reauth.svelte.ts). Signing in again opens a new tab — a top-level load there
	// re-establishes the proxy session while this page keeps its input; the banner
	// clears itself once the activity poll gets through again. Sticky above every
	// overlay (z-50 dialogs, the z-[60] image viewer): the edit being protected is
	// usually inside an open dialog, and the owner may be scrolled far from the
	// header. Tokens only; QA Cinémathèque.
	import { reauth } from '$lib/reauth.svelte';

	const COPY = {
		edit: ['Your sign-in expired.', 'Your edits are still here. Sign in again before you save.'],
		write: ["Your sign-in expired, so your last change wasn't saved.", 'Sign in again, then save it again.'],
		resubmit: ["You're signed in again.", "Your last change still isn't saved. Save it again."]
	} as const;

	function signIn() {
		window.open(window.location.href, '_blank', 'noopener');
	}
</script>

<!-- Keyed on the state so each change re-inserts the region: an edit → write
     switch is announced as a fresh alert instead of a silent role mutation. -->
{#key reauth.hold}
{#if reauth.hold}
	{@const [lead, body] = COPY[reauth.hold]}
	<div
		role={reauth.hold === 'write' ? 'alert' : 'status'}
		class="sticky top-0 z-[70] flex flex-wrap items-center justify-between gap-2 border-b border-warn bg-surface px-6 py-2 text-sm text-ink"
	>
		<span><span class="font-semibold text-warn">{lead}</span> {body}</span>
		{#if reauth.hold === 'resubmit'}
			<button type="button" class="btn-quiet px-3 py-1.5 text-sm" onclick={() => (reauth.hold = null)}>
				Dismiss
			</button>
		{:else}
			<button type="button" class="btn-ghost px-3 py-1.5 text-sm" title="Opens in a new tab" onclick={signIn}>
				Sign in again ↗
			</button>
		{/if}
	</div>
{/if}
{/key}
