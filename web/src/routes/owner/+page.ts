// /owner lands on the Status tab (F35). The hub itself has no bare-index content;
// Status is the default view.
import { redirect } from '@sveltejs/kit';

export function load() {
	redirect(308, '/owner/status');
}
