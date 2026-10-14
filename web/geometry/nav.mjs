#!/usr/bin/env node
// The navigation behaviour harness (HOLODEX-475).
//
//   npm --prefix web run nav -- <flags>     (from the repository root; no cd)
//
// The geometry harness measures one page at a time and never moves between pages, so
// the parts of ADR-114 that only exist *across* a navigation were live-QA only
// (testing-strategy §20.3 items 1–5). Each scenario below drives one of them in a real
// browser and asserts on the URL, localStorage and scroll position:
//
//   - a same-route nav click re-derives state from the URL, so filters clear (D2) —
//     SvelteKit reuses the component there, which no unit test can reproduce
//   - arriving by a link writes no storage (D3) — the `$effect` mutant in §20.4 that
//     survives every unit test
//   - Back restores filters, sort and scroll (R7/R8)
//   - a removal exit goes Back when in-app and to the bare list otherwise (D4)
//   - two tabs keep independent views
//
// Prerequisites are the geometry harness's 2–4 (seeded fixture, `backend-stress`,
// `web`); nothing here reads the manifest, so any owner-default-open library with a
// screenful of videos and people works. Deletes are intercepted and answered 204 in
// the browser, so the removal scenarios never touch the fixture.

import { parseArgs } from 'node:util';
import { WIDTHS, open, goto, settle, launch } from './browser.mjs';

let values;
try {
	({ values } = parseArgs({
		options: {
			base: { type: 'string', default: 'http://localhost:5173' },
			only: { type: 'string', multiple: true, default: [] },
			headed: { type: 'boolean', default: false },
			list: { type: 'boolean', default: false },
			help: { type: 'boolean', default: false }
		}
	}));
} catch (err) {
	console.error(`${/** @type {Error} */ (err).message}\nrun with --help for the flags`);
	process.exit(2);
}

if (values.help) {
	console.log(`nav — ADR-114 navigation behaviour against a running app (HOLODEX-475)

  --base <url>   app under test          (default http://localhost:5173)
  --only <key>   run one scenario; repeatable
  --headed       watch it run
  --list         print the scenarios and exit
`);
	process.exit(0);
}

const base = values.base;
const MEDIA_CARD = 'main a[href^="/media/"]';
const PERSON_ROW = 'main a[href^="/people/"]';
const CHIP = '[data-list-chips] button[aria-label^="Remove "]';
const SORT = '[data-list-toolbar] select';

/** A failed expectation; the scenario stops at the first one. */
class Broken extends Error {}

/** @param {unknown} cond @param {string} message */
function expect(cond, message) {
	if (!cond) throw new Broken(message);
}

/** @param {import('playwright').Page} page */
const here = (page) => {
	const u = new URL(page.url());
	return u.pathname + u.search;
};

/** @param {import('playwright').Page} page */
const sortKeys = (page) =>
	page.evaluate(() =>
		Object.fromEntries(Object.entries(localStorage).filter(([k]) => k.startsWith('holodex:sort:')))
	);

/**
 * clickNav follows a header link the way a person does — a client-side navigation, so
 * SvelteKit keeps the page component and only afterNavigate can re-derive its state.
 *
 * @param {import('playwright').Page} page
 * @param {string} href
 * @param {(u: URL) => boolean} arrived
 */
async function clickNav(page, href, arrived) {
	await page.locator(`header nav a[href="${href}"]`).click();
	await page.waitForURL(arrived, { timeout: 5000 });
	await settle(page);
}

/**
 * scrollTo scrolls and waits until the page is actually there: a list still loading
 * rows is too short to reach the target, and the assertion after Back would then
 * compare against a position the page never had.
 *
 * @param {import('playwright').Page} page
 * @param {number} y
 */
async function scrollTo(page, y) {
	await page.evaluate((to) => window.scrollTo(0, to), y);
	await page.waitForFunction((to) => Math.abs(window.scrollY - to) <= 1, y, { timeout: 5000 }).catch(() => {});
	const at = await page.evaluate(() => window.scrollY);
	expect(Math.abs(at - y) <= 1, `could not scroll to ${y} (stuck at ${at}) — the list is too short to test scroll restore`);
}

/**
 * openVisible clicks the first row link fully inside the viewport, so the click never
 * scrolls the page and moves the position Back is supposed to restore.
 *
 * @param {import('playwright').Page} page
 * @param {string} selector
 * @param {RegExp} detail
 */
async function openVisible(page, selector, detail) {
	const i = await page.$$eval(selector, (els) =>
		els.findIndex((el) => {
			const r = el.getBoundingClientRect();
			return r.height > 0 && r.top >= 0 && r.bottom <= window.innerHeight;
		})
	);
	expect(i >= 0, `no ${selector} fully in view to click`);
	await page.locator(selector).nth(i).click();
	await page.waitForURL((u) => detail.test(u.pathname), { timeout: 5000 });
	await settle(page);
}

/**
 * back presses Back and waits for the list to be where it was: URL first, then scroll.
 * The scroll wait is a poll because restoration lands after the rows re-render.
 *
 * @param {import('playwright').Page} page
 * @param {string} url  path + search expected after Back
 * @param {number} y    scroll position expected after Back
 */
async function backTo(page, url, y) {
	await page.goBack({ waitUntil: 'commit' });
	await page.waitForURL((u) => u.pathname + u.search === url, { timeout: 5000 }).catch(() => {});
	expect(here(page) === url, `Back landed on ${here(page)}, expected ${url}`);
	await page.waitForFunction((to) => Math.abs(window.scrollY - to) <= 2, y, { timeout: 5000 }).catch(() => {});
	const at = await page.evaluate(() => window.scrollY);
	expect(Math.abs(at - y) <= 2, `Back restored scrollY ${at}, expected ${y}`);
}

/**
 * trash deletes the open video through the real UI with the DELETE answered in the
 * browser, so the app runs its whole removal exit and the fixture is untouched.
 *
 * @param {import('playwright').Page} page
 */
async function trash(page) {
	await page.route(/\/api\/v1\/media\/\d+(\?.*)?$/, (route) =>
		route.request().method() === 'DELETE' ? route.fulfill({ status: 204 }) : route.continue()
	);
	await page.locator('[data-delete-split] > button').first().click();
	await page.locator('[role="dialog"] button', { hasText: 'Move to Trash' }).click();
}

/**
 * filterValue picks a Media resolution filter that matches something, so the scenarios
 * that open a card have one to open. Resolved once from the API, not hardcoded: the
 * stress fixture is all FHD, a real library is mostly not.
 */
async function filterValue() {
	const counts = await Promise.all(
		['SD', 'HD', 'FHD', '4K'].map(async (value) => {
			const res = await fetch(`${base}/api/v1/media?resolution=${value}&limit=1`);
			const { total = 0 } = res.ok ? await res.json() : {};
			return { value, total };
		})
	);
	const best = counts.reduce((a, b) => (b.total > a.total ? b : a), { value: '', total: 0 });
	if (!best.value) throw new Error('no resolution filter matches any video — is the library empty?');
	return best.value;
}

/**
 * The scenarios, one per §20.3 live-QA item they replace. Each gets a fresh browser
 * context (empty storage) and throws Broken at its first unmet expectation.
 *
 * @type {{key: string, qa: string, run: (ctx: {context: import('playwright').BrowserContext, page: import('playwright').Page, filter: string}) => Promise<void>}[]}
 */
const SCENARIOS = [
	{
		key: 'same-route-nav-clears-filters',
		qa: '§20.3 item 1 · ADR-114 D2',
		run: async ({ page, filter }) => {
			await goto(page, `${base}/?resolution=${filter}`);
			expect((await page.locator(CHIP).count()) === 1, `no filter chip on /?resolution=${filter}`);
			await clickNav(page, '/', (u) => u.pathname === '/' && !u.searchParams.has('resolution'));
			expect(here(page) === '/', `nav click to Media left ${here(page)}, expected a bare /`);
			expect((await page.locator(CHIP).count()) === 0, 'the filter chip survived a same-route nav click');

			// Tags carries its query in the URL too (`type`), and is a second page component.
			await goto(page, `${base}/tags?type=categories`);
			await clickNav(page, '/tags', (u) => u.pathname === '/tags' && !u.searchParams.has('type'));
			expect(here(page) === '/tags', `nav click to Tags left ${here(page)}, expected a bare /tags`);
		}
	},
	{
		key: 'shared-link-writes-nothing',
		qa: '§20.3 item 4 · ADR-114 D3',
		run: async ({ page }) => {
			await goto(page, `${base}/people`);
			await page.locator(SORT).selectOption('name');
			expect((await sortKeys(page))['holodex:sort:people'] === 'name', 'choosing a sort did not save it');
			const before = await sortKeys(page);

			await goto(page, `${base}/people?sort=count`);
			expect((await page.locator(SORT).inputValue()) === 'count', 'a ?sort=count link did not apply its sort');
			const after = await sortKeys(page);
			expect(
				JSON.stringify(after) === JSON.stringify(before),
				`arriving by link wrote storage: ${JSON.stringify(before)} → ${JSON.stringify(after)}`
			);

			// The saved preference, not the link, is what a nav click comes back to.
			await clickNav(page, '/people', (u) => u.pathname === '/people' && !u.searchParams.has('sort'));
			expect((await page.locator(SORT).inputValue()) === 'name', 'a nav click after the link did not restore A–Z');
		}
	},
	{
		key: 'back-restores-filter-sort-scroll',
		qa: '§20.3 item 2 · R7/R8',
		run: async ({ page, filter }) => {
			const list = `/?resolution=${filter}&sort=title_asc`;
			await goto(page, `${base}${list}`);
			await scrollTo(page, 600);
			await openVisible(page, MEDIA_CARD, /^\/media\/\d+$/);
			await backTo(page, list, 600);
			expect((await page.locator(CHIP).count()) === 1, 'Back lost the filter chip');
			expect((await page.locator(SORT).inputValue()) === 'title_asc', 'Back lost the sort');
		}
	},
	{
		key: 'removal-exit-in-app',
		qa: '§20.3 item 3 · ADR-114 D4',
		run: async ({ page, filter }) => {
			const list = `/?resolution=${filter}`;
			await goto(page, `${base}${list}`);
			await openVisible(page, MEDIA_CARD, /^\/media\/\d+$/);
			await trash(page);
			await page.waitForURL((u) => u.pathname === '/', { timeout: 5000 }).catch(() => {});
			expect(here(page) === list, `trashing a video opened from ${list} landed on ${here(page)}`);
		}
	},
	{
		key: 'back-after-write-refreshes-grid',
		qa: 'HOLODEX-190',
		run: async ({ page, filter }) => {
			// The grid snapshot was saved before the trash, so Back must swap in the server's
			// current list, keeping the scroll. The intercepted DELETE never reaches the
			// fixture, so the list answer is the one that leaves the card out.
			const list = `/?resolution=${filter}&sort=title_asc`;
			await goto(page, `${base}${list}`);
			await scrollTo(page, 600);
			await openVisible(page, MEDIA_CARD, /^\/media\/\d+$/);
			const id = Number(new URL(page.url()).pathname.split('/').pop());
			// Answer as the server would after the delete: one more row asked for, the trashed
			// one dropped, trimmed back to the page size.
			await page.route(/\/api\/v1\/media\?/, async (route) => {
				const url = new URL(route.request().url());
				const limit = Number(url.searchParams.get('limit') ?? 50);
				url.searchParams.set('limit', String(limit + 1));
				const res = await route.fetch({ url: url.toString() });
				const body = await res.json();
				const items = body.items.filter((/** @type {{id: number}} */ v) => v.id !== id);
				const total = body.total - (body.items.length - items.length);
				await route.fulfill({ response: res, json: { ...body, items: items.slice(0, limit), total } });
			});
			// The refetch Back must make. Waiting only for the card to be absent passes vacuously
			// in the instant after Back, before the restored grid has rendered (HOLODEX-574).
			const refetched = page
				.waitForResponse((r) => /\/api\/v1\/media\?/.test(r.url()) && r.url().includes(`resolution=${filter}`), {
					timeout: 5000
				})
				.then(() => true, () => false);
			await trash(page);
			await page.waitForURL((u) => u.pathname === '/', { timeout: 5000 }).catch(() => {});
			expect(await refetched, 'Back restored the grid without refetching it after the trash');
			const gone = `main a[href="/media/${id}"]`;
			await page
				.waitForFunction(
					([card, sel]) => document.querySelector(card) && !document.querySelector(sel),
					[MEDIA_CARD, gone],
					{ timeout: 5000 }
				)
				.catch(() => {});
			expect((await page.locator(MEDIA_CARD).count()) > 0, 'the grid rendered no cards after Back');
			expect((await page.locator(gone).count()) === 0, `Back still shows the trashed /media/${id}`);
			expect(here(page) === list, `trashing landed on ${here(page)}, expected ${list}`);
			const at = await page.evaluate(() => window.scrollY);
			expect(Math.abs(at - 600) <= 2, `the refreshed grid moved the scroll to ${at}, expected 600`);
			await page.unrouteAll({ behavior: 'ignoreErrors' });
		}
	},
	{
		key: 'removal-exit-direct',
		qa: '§20.3 item 3 · ADR-114 D4',
		run: async ({ page, filter }) => {
			// Find a card, then arrive at it cold — as a new tab or a pasted link does.
			await goto(page, `${base}/?resolution=${filter}`);
			const href = await page.locator(MEDIA_CARD).first().getAttribute('href');
			await goto(page, `${base}${href}`);
			await trash(page);
			await page.waitForURL((u) => u.pathname === '/', { timeout: 5000 }).catch(() => {});
			expect(here(page) === '/', `trashing a directly-opened video landed on ${here(page)}, expected a bare /`);
		}
	},
	{
		key: 'two-tabs-independent',
		qa: '§20.3 item 5',
		run: async ({ context, page }) => {
			const tabs = [
				{ page, list: '/people?sort=count', y: 400 },
				{ page: await context.newPage(), list: '/people?sort=random', y: 800 }
			];
			for (const t of tabs) {
				await goto(t.page, `${base}${t.list}`);
				// Random carries its seed into the URL on arrival; Back must return to that.
				t.list = here(t.page);
				await scrollTo(t.page, t.y);
			}
			for (const t of tabs) await openVisible(t.page, PERSON_ROW, /^\/people\/\d+$/);
			for (const t of tabs) await backTo(t.page, t.list, t.y);
			// A fresh context saved nothing, and neither tab chose a sort — both arrived by link.
			const saved = await sortKeys(page);
			expect(Object.keys(saved).length === 0, `two linked tabs wrote the saved sort: ${JSON.stringify(saved)}`);
		}
	},
	{
		key: 'back-after-reload-restores-scroll',
		qa: 'HOLODEX-477 · ADR-118',
		run: async ({ page }) => {
			// A full reload on the detail page (a re-auth redirect, a deploy) empties JS memory;
			// the scroll snapshot has to come back from sessionStorage.
			await goto(page, `${base}/people`);
			await scrollTo(page, 600);
			await openVisible(page, PERSON_ROW, /^\/people\/\d+$/);
			// Wait for the reloaded page to hydrate, as a user sees it before pressing Back.
			await page.reload({ waitUntil: 'networkidle' });
			await settle(page);
			await backTo(page, '/people', 600);
		}
	},
	{
		key: 'link-arrival-starts-fresh',
		qa: 'HOLODEX-573',
		run: async ({ page }) => {
			// A header link back to a list visited earlier is a fresh visit, not a return: it must
			// not restore the scroll (or, on Media, the loaded pages) left by the last visit.
			for (const { list, card, detail } of [
				{ list: '/people', card: PERSON_ROW, detail: /^\/people\/\d+$/ },
				{ list: '/', card: MEDIA_CARD, detail: /^\/media\/\d+$/ }
			]) {
				await goto(page, `${base}${list}`);
				await scrollTo(page, 600);
				await openVisible(page, card, detail);
				await clickNav(page, list, (u) => u.pathname === list);
				// A restore lands a tick after the list renders, so wait for the rows, then a frame.
				await page.locator(card).first().waitFor({ timeout: 5000 });
				await settle(page);
				const at = await page.evaluate(() => window.scrollY);
				expect(at === 0, `a header link back to ${list} restored scrollY ${at}, expected 0`);
			}
		}
	},
	{
		key: 'hash-anchor-lands',
		qa: 'HOLODEX-459',
		run: async ({ page }) => {
			// The F68 hover card's #videos / #films links. Neither section is reachable until the
			// grid's first page lands, so the router's own fragment scroll finds nothing.
			const res = await fetch(`${base}/api/v1/people?sort=count&limit=1`);
			const [person] = (await res.json()).items ?? [];
			expect(person, 'no person in the library to open');
			const path = `/people/${person.id}`;
			/** @param {string} id */
			const landed = (id) =>
				page
					.waitForFunction(
						(el) => {
							const top = document.getElementById(el)?.getBoundingClientRect().top;
							const max = document.documentElement.scrollHeight - innerHeight;
							// 64 is scroll-mt-16; a section near the page's end stops at the max scroll.
							// scrollY > 0: a page too short to scroll would pass without any jump.
							const atEnd = scrollY > 0 && Math.abs(scrollY - max) <= 1 && top < 64 + innerHeight;
							return top != null && (Math.abs(top - 64) <= 2 || atEnd);
						},
						id,
						{ timeout: 5000 }
					)
					.then(() => true, () => false);

			await goto(page, `${base}${path}#videos`);
			expect(await landed('videos'), `a cold load of ${path}#videos did not land on #videos`);

			// A link back to an entity visited earlier carries a stale scroll snapshot; the anchor
			// wins over it. Only Back restores the snapshot.
			await goto(page, `${base}${path}`);
			await settle(page);
			await page.evaluate(() => window.scrollTo(0, 300));
			await clickNav(page, '/people', (u) => u.pathname === '/people');
			await page.evaluate((href) => {
				const a = document.createElement('a');
				a.href = href;
				document.querySelector('main')?.append(a);
				a.click();
			}, `${path}#videos`);
			expect(await landed('videos'), `a link to ${path}#videos (visited before) did not land on #videos`);

			// Back to that #videos entry restores where the grid was, not the anchor.
			const y = (await page.evaluate(() => window.scrollY)) + 300;
			await scrollTo(page, y);
			await openVisible(page, MEDIA_CARD, /^\/media\/\d+$/);
			await backTo(page, path, y);

			// The shelf has its own fetch; wait for it rather than skip on a race.
			const hasFilms = await page.waitForSelector('#films', { timeout: 5000 }).then(() => true, () => false);
			if (hasFilms) {
				await goto(page, `${base}${path}#films`);
				expect(await landed('films'), `a cold load of ${path}#films did not land on #films`);
			} else console.log(`      (${path} has no films; #films not checked)`);
		}
	}
];

const unknown = values.only.filter((k) => !SCENARIOS.some((s) => s.key === k));
if (unknown.length > 0) {
	console.error(`unknown --only: ${unknown.join(', ')} — known: ${SCENARIOS.map((s) => s.key).join(', ')}`);
	process.exit(2);
}
const scenarios = values.only.length ? SCENARIOS.filter((s) => values.only.includes(s.key)) : SCENARIOS;

if (values.list) {
	for (const s of scenarios) console.log(`${s.key.padEnd(34)} ${s.qa}`);
	process.exit(0);
}

// Preflight: the geometry harness's reasoning, minus the manifest. Not an owner means
// the Trash control never renders; a dead server means every scenario would "fail".
let filter;
try {
	const caps = await fetch(`${base}/api/v1/capabilities`).then(
		(r) => (r.ok ? r.json() : Promise.reject(new Error(`GET /api/v1/capabilities → ${r.status}; is the backend up?`))),
		(err) => Promise.reject(new Error(`${base} is not answering (${err.message}); is the \`web\` dev server running?`))
	);
	if (!caps.owner) throw new Error(`${base} does not treat this client as an owner; the removal scenarios need the Trash control`);
	filter = await filterValue();
} catch (err) {
	console.error(`preflight failed: ${/** @type {Error} */ (err).message}`);
	process.exit(2);
}

const browser = await launch(values.headed);
let failed = 0;
let errored = 0;
try {
	for (const s of scenarios) {
		const { context, page } = await open(browser, WIDTHS[0]);
		try {
			await s.run({ context, page, filter });
			console.log(`pass  ${s.key}`);
		} catch (err) {
			// Broken is a verdict; anything else (a timeout, a missing element) means the
			// scenario could not run to one, which is reported apart so it never reads as a pass.
			if (err instanceof Broken) failed++;
			else errored++;
			console.log(`${err instanceof Broken ? 'FAIL ' : 'ERROR'} ${s.key}  (${s.qa})\n      ${/** @type {Error} */ (err).message.split('\n')[0]}`);
		} finally {
			await context.close();
		}
	}
} finally {
	await browser.close();
}

console.log(`\n${scenarios.length - failed - errored} passed, ${failed} failed, ${errored} could not run`);
process.exit(errored > 0 ? 2 : failed > 0 ? 1 : 0);
