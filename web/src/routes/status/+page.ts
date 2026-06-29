// Permanent redirect: /status moved under the Owner hub (F35). Keeps old bookmarks
// and the ActivityIndicator/F29 references working.
import { redirect } from '@sveltejs/kit';

export function load() {
	redirect(308, '/owner/status');
}
