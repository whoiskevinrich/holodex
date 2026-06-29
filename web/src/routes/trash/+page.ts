// Permanent redirect: /trash moved under the Owner hub (F35).
import { redirect } from '@sveltejs/kit';

export function load() {
	redirect(308, '/owner/trash');
}
