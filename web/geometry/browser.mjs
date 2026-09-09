// The browser half (HOLODEX-349): drive Chromium, hold a page still, measure it.
//
// Everything that can go wrong with automated measurement on this app is handled in
// one place here, because each of these was a real hazard rather than a precaution:
// the skin only applies after hydration, the grid mount animation offsets every rect
// by 8px for a quarter second, the metadata fold clips its rows to zero height, and
// column counts come from a localStorage density preference and a resize listener.

import { chromium } from 'playwright';

/**
 * The run matrix. Three skins because the skins are not a re-paint — each changes
 * `--radius`, and each changes the display font, so text metrics and therefore
 * wrapping differ (`web/src/app.css`). Broadcast additionally appends a `▮` glyph to
 * every `.skin-title`, which widens headings.
 *
 * Two widths because the app's column counts are width-derived (`density.svelte.ts`)
 * and `.stage-grid` collapses from two columns to one below 1024px, so a single width
 * would leave half the layout unmeasured.
 */
export const SKINS = ['cinematheque', 'broadcast', 'brutalist'];
export const WIDTHS = [
	{ key: 'wide', width: 1440, height: 900 },
	{ key: 'narrow', width: 768, height: 1024 }
];

/** @returns {{key: string, skin: string, width: number, height: number}[]} */
export function matrix(skins = SKINS, widths = WIDTHS) {
	const out = [];
	for (const skin of skins) {
		for (const w of widths) {
			out.push({ key: `${skin}/${w.key}`, skin, width: w.width, height: w.height });
		}
	}
	return out;
}

/**
 * open starts a browser context pinned to one cell of the matrix.
 *
 * `reducedMotion: 'reduce'` is not a nicety. The staggered `reel-rise` mount animation
 * on every grid child is gated on `prefers-reduced-motion: no-preference`, and while it
 * runs the element is translated 8px and fully transparent — so a rect measured during
 * it is simply wrong. Emulating the preference removes the race instead of sleeping
 * through it.
 *
 * The init script runs before the app's own scripts on every navigation, which is the
 * only way to have the persisted preferences already correct on first paint: the skin
 * is read by `theme.init()` during hydration, and poking `data-theme` afterwards would
 * leave `PersonImageFrame`'s `?skin=` image URLs on the previous skin.
 *
 * @param {import('playwright').Browser} browser
 * @param {{skin: string, width: number, height: number}} cell
 */
export async function open(browser, cell) {
	const context = await browser.newContext({
		viewport: { width: cell.width, height: cell.height },
		reducedMotion: 'reduce',
		deviceScaleFactor: 1
	});
	await context.addInitScript(
		({ skin }) => {
			try {
				localStorage.setItem('holodex-theme', skin);
				// Owner view. The stress profile configures no ADMIN_TOKEN, so the server
				// is default-open (ADR-030) and every request is already an owner — but
				// this flag is a separate, persisted *presentation* switch, and half the
				// surfaces worth measuring (the metadata list, the chip row) do not exist
				// with it off. Pinned rather than assumed.
				localStorage.setItem('holodex-admin-mode', 'true');
				// Column counts are min(preference, cap-for-width). Pinning the preference
				// leaves the width as the only variable, which is the one under test.
				localStorage.setItem('holodex:media-density', '4');
			} catch {
				// A context with storage disabled still renders; it just renders the
				// defaults. Better a measured default than a crashed run.
			}
		},
		{ skin: cell.skin }
	);
	const page = await context.newPage();
	// Hover transforms `.person-hero-media` and `.poster-card`, which would move the
	// very boxes being measured. Park the pointer outside anything interactive.
	await page.mouse.move(0, 0);
	return { context, page };
}

/**
 * goto navigates and then waits for the page to actually be still.
 *
 * `networkidle` is deliberately not used: `VideoCard` retries a missing thumbnail up
 * to five times with backoff reaching ~30s, so a freshly seeded fixture can keep the
 * network busy long past the point the layout has settled. The waits below are for
 * the things that actually move.
 *
 * @param {import('playwright').Page} page
 * @param {string} url
 * @param {string} skin
 */
export async function goto(page, url, skin) {
	// Registered before the navigation, or the response can land before the wait
	// starts. Owner-gated surfaces — the whole metadata list, the chip row — mount only
	// once `/capabilities` has answered, so measuring before it does silently reports an
	// empty visitor page. This was not theoretical: it is what the first live run did.
	const capabilities = page
		.waitForResponse((r) => r.url().includes('/api/v1/capabilities'), { timeout: 15000 })
		.catch(() => null);
	await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 20000 });
	await capabilities;
	// Hydration has run and applied the persisted skin. Until this is true the DOM is
	// the static shell from app.html, which always claims `cinematheque`.
	await page.waitForFunction((want) => document.documentElement.dataset.theme === want, skin, {
		timeout: 15000
	});
	// AsyncState swaps the entire subtree for a "Loading…" paragraph, so a selector
	// query before the data lands matches nothing and reads as a stale selector.
	await page
		.waitForFunction(() => !/(^|\s)Loading…(\s|$)/.test(document.body.innerText), null, { timeout: 15000 })
		.catch(() => {
			// A page that legitimately shows nothing else will time out here; let the
			// probe report what it finds rather than failing the whole cell.
		});
	await settle(page);
}

/**
 * settle waits two animation frames. `.stage-aligned` grids compute their track sizes
 * from a `bind:clientWidth`, i.e. a ResizeObserver, so the first frame after render
 * still has the pre-measurement tracks.
 *
 * @param {import('playwright').Page} page
 */
export async function settle(page) {
	await page.evaluate(
		() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve(null))))
	);
}

/**
 * PREPARATIONS are the named steps an assertion may request before measurement.
 *
 * A named registry rather than arbitrary per-assertion code: there are exactly two
 * subtrees on this app that exist only after an interaction, both of them folds, and
 * spelling them out here means an assertion stays a declaration. Each is idempotent —
 * running it on a page that is already open must not close it.
 *
 * @type {Record<string, (page: import('playwright').Page, arg: string) => Promise<void>>}
 */
export const PREPARATIONS = {
	// The metadata field list is collapsed to `max-height: 0` and `inert` by default,
	// so every `#field-*` row inside it measures as zero-height until it is opened.
	//
	// Each step waits for the container first and for the opened state after. The
	// middle click is conditional because an already-open fold relabels its toggle, and
	// a preparation has to be idempotent; the waits either side are what separate
	// "already open" from "never rendered" — without them, a page whose owner surface
	// had not mounted yet looked exactly like a fold that needed no clicking.
	'metadata-fold': async (page) => {
		await page.waitForSelector('#metadata-fields', { state: 'attached', timeout: 10000 });
		const toggle = page.locator('button[aria-label="Show metadata fields"]');
		if ((await toggle.count()) > 0) await toggle.first().click();
		await page.waitForFunction(() => !document.querySelector('#metadata-fields')?.hasAttribute('inert'), null, {
			timeout: 5000
		});
	},
	// A SourceBadge renders its value and a collapsed badge; the per-provider chips
	// exist only while expanded, and only one field may be expanded at a time.
	'source-badge': async (page, field) => {
		const badge = `[data-source-badge="${field}"]`;
		await page.waitForSelector(badge, { state: 'attached', timeout: 10000 });
		const opener = page.locator(`${badge} button[aria-expanded="false"]`);
		if ((await opener.count()) > 0) await opener.first().click();
		await page.waitForSelector(`${badge} [data-seg]`, { state: 'attached', timeout: 5000 });
	}
};

/**
 * prepare runs an assertion's preparations, then re-settles. A step naming no known
 * preparation is an error rather than a no-op: silently skipping it would measure the
 * closed state and report a confident, wrong answer.
 *
 * @param {import('playwright').Page} page
 * @param {string[]} steps
 */
export async function prepare(page, steps = []) {
	for (const step of steps) {
		const [name, arg = ''] = step.split(':');
		const fn = PREPARATIONS[name];
		if (!fn) throw new Error(`unknown preparation "${step}" — known: ${Object.keys(PREPARATIONS).join(', ')}`);
		await fn(page, arg);
	}
	if (steps.length > 0) {
		// The fold transitions max-height over 200ms; `motion-reduce:transition-none` is
		// on it and the context emulates reduced motion, so this is a frame, not a sleep.
		await settle(page);
	}
}

/**
 * probe measures every element matching a selector, in the page.
 *
 * `:document` measures the document element itself, which is how page-level horizontal
 * overflow is expressed — `scrollWidth - clientWidth` on `<html>` is the number that
 * says "this page scrolls sideways".
 *
 * @param {import('playwright').Page} page
 * @param {string} selector
 * @returns {Promise<{matched: number, elements: import('./evaluate.mjs').Measured[], error?: string}>}
 */
export async function probe(page, selector) {
	if (selector !== ':document') {
		// Give a slow render a chance before concluding the selector is stale, but do
		// not let a genuine miss hang the run — an absent match is a real verdict.
		await page.waitForSelector(selector, { timeout: 3000, state: 'attached' }).catch(() => {});
	}
	try {
		return await page.evaluate((sel) => {
			const nodes =
				sel === ':document'
					? [document.documentElement]
					: Array.from(document.querySelectorAll(sel));
			const elements = nodes.map((el, index) => {
				const rect = el.getBoundingClientRect();
				const style = getComputedStyle(el);
				// scrollWidth/clientWidth are both 0 on a non-scrollable box — an inline <a>
				// or <span>, for instance — so their difference would read as a confident
				// "0, no overflow" for exactly the elements most likely to be overflowing
				// their text. Report it as unmeasurable instead and let evaluate() say so.
				const scrollable = el.clientWidth > 0 || el.clientHeight > 0;
				return {
					index,
					tag:
						el.tagName.toLowerCase() +
						(el.id ? `#${el.id}` : '') +
						(typeof el.className === 'string' && el.className
							? `.${el.className.trim().split(/\s+/).slice(0, 3).join('.')}`
							: ''),
					width: Math.round(rect.width * 100) / 100,
					height: Math.round(rect.height * 100) / 100,
					overflowX: scrollable ? el.scrollWidth - el.clientWidth : null,
					overflowY: scrollable ? el.scrollHeight - el.clientHeight : null,
					fontSize: parseFloat(style.fontSize) || 0,
					visible: rect.width > 0 && rect.height > 0 && style.visibility !== 'hidden',
					text: (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 60)
				};
			});
			return { matched: elements.length, elements };
		}, selector);
	} catch (err) {
		return { matched: 0, elements: [], error: `probe failed: ${/** @type {Error} */ (err).message}` };
	}
}

/** @param {boolean} headed */
export function launch(headed = false) {
	return chromium.launch({ headless: !headed });
}
