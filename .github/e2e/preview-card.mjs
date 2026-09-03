// Behavior lock: the card that shows where a link leads without leaving the
// page. It is checked in a browser because almost nothing about it exists in
// the served HTML: the card's markup is one empty element, and every claim
// worth making — that it waits before opening, that it lands beside its own
// link, that it holds the section the link addressed, that it goes away again,
// that a touch screen never sees it — is a fact about a live pointer over a
// laid-out page.
//
// The regression class is quiet: a broken card is an absent card, which reads
// exactly like a hover the page did not register, and every server-side test
// stays green while it happens.
//
// Env: YOMIHON_BASE (default http://127.0.0.1:9610), PAGE_PATH (the note that
// carries the links). MUTATE names one of the self-test modes below;
// MUTATE=list prints them.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';

// The links under test, by the words on screen. The first is a section link
// written with no display text, so it carries the section name in CJK with
// punctuation in it; the second names a section in the reader's own words. The
// third is written at a section its destination does not answer to, and the
// fourth leaves the vault entirely — neither may ever open a card.
const SECTION_LINK = 'back to the material';
const SECTION_HEADING = 'Sensory material';
const WHOLE_NOTE_LINK = 'Glass Tide#Glass Tide';
// A section named in CJK, with punctuation in it, addressed by a link written
// with no display text. Its fragment passes through three encodings between the
// anchor that stamped it and the cut that answers to it.
const CJK_LINK = 'Glass Tide#第三節：失約的燈';
const CJK_HEADING = '第三節：失約的燈';
const DEGRADED_LINK = 'Glass Tide#A section nobody wrote';
const EXTERNAL_LINK = 'https://example.invalid/lamps';

// The opening words of the destination, which a card cut at a section must not
// carry, and the words of the section itself, which it must.
const NOTE_OPENING = 'The note a cross-note heading link is written at';

const SITES = [
	'card-opens-on-hover',
	'card-waits-out-a-passing-pointer',
	'card-opens-on-focus',
	'card-anchored-to-its-link',
	'card-shows-the-section-the-link-addressed',
	'card-scrolls-inside-itself',
	'a-link-that-cannot-be-previewed-opens-nothing',
	'escape-dismisses-the-card',
	'the-pointer-leaving-dismisses-the-card',
	'scrolling-dismisses-the-card',
	'focus-stays-on-the-link',
	'a-coarse-pointer-opens-no-card',
	'a-missing-note-answers-with-words',
	'a-section-the-note-does-not-have-shows-none-of-it',
	'an-open-card-adds-no-second-place-with-one-name',
	'the-excerpt-comes-from-this-origin',
];

class LockFired extends Error {
	constructor(site, message) {
		super(message);
		this.site = site;
	}
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
	if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN preview-card: unknown assertion site ${site}`);
	throw new LockFired(site, `FAIL preview-card: ${message}`);
};
const broken = (message) => {
	throw new ProbeBroken(`BROKEN preview-card: ${message}`);
};
const notApplied = (message) => {
	throw new NotApplied(`NOT-APPLIED preview-card: ${message}`);
};

// Rewrites the client module the whole feature lives in. The replacement is
// counted, so a needle that no longer matches the source reports itself rather
// than passing as a mutation nobody noticed.
const rewriteModule = (needle, replacement) => async (context) => {
	let matched = -1;
	await context.route('**/static/preview.js', async (route) => {
		const response = await route.fetch();
		const original = await response.text();
		matched = original.split(needle).length - 1;
		await route.fulfill({ response, body: original.split(needle).join(replacement) });
	});
	return () =>
		matched === 1 ? '' : `the module needle ${JSON.stringify(needle)} matched ${matched === -1 ? 'nothing, because the module was never fetched' : `${matched} times, want 1`}`;
};

// Appends to the product's own stylesheet, landing outside the layer it
// declares, so the rule outranks what it stands in for without an importance
// flag.
const weakenStylesheet = (rule) => async (context) => {
	let served = false;
	await context.route('**/static/app.css', async (route) => {
		const response = await route.fetch();
		const original = await response.text();
		served = true;
		await route.fulfill({ response, body: `${original}\n${rule}\n` });
	});
	return () => (served ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

// Rewrites what the preview route answers with, which is the only way to see
// what the card does with an answer it did not expect.
const rewriteFragment = (needle, replacement) => async (context) => {
	// Counted across every fragment the run fetches, not per response: only one
	// of them carries the needle, and a per-response count would be answered by
	// whichever fragment happened to be fetched last.
	let fetched = 0;
	let rewritten = 0;
	await context.route('**/preview/**', async (route) => {
		const response = await route.fetch();
		const original = await response.text();
		fetched += 1;
		rewritten += original.split(needle).length - 1;
		await route.fulfill({ response, body: original.split(needle).join(replacement) });
	});
	return () =>
		rewritten > 0
			? ''
			: `the fragment needle ${JSON.stringify(needle)} was rewritten in none of the ${fetched} fragments this run fetched`;
};

const MUTATIONS = {
	// The defect itself: nothing listens for the pointer arriving.
	'never-listen-for-the-pointer': {
		target: 'card-opens-on-hover',
		apply: rewriteModule("link.addEventListener('pointerenter', () => schedule(link, openDelay));", ''),
	},
	// The card opens the instant a pointer touches a link, so crossing a
	// paragraph of them flashes a card for each.
	'open-with-no-delay': {
		target: 'card-waits-out-a-passing-pointer',
		apply: rewriteModule('const openDelay = 250;', 'const openDelay = 0;'),
	},
	// Reaching a link by keyboard stops asking the question a hover asks.
	'ignore-the-keyboard': {
		target: 'card-opens-on-focus',
		apply: rewriteModule("link.addEventListener('focus', () => schedule(link, 0));", ''),
	},
	// The card keeps its content and loses its place: with no anchor it falls
	// back to the corner of the window, beside nothing.
	'unanchor-the-card': {
		target: 'card-anchored-to-its-link',
		apply: weakenStylesheet('.y-preview{position-anchor:none;position-area:none;inset:8px auto auto 8px}'),
	},
	// The address the card asks for loses its fragment, so every card shows the
	// destination from the top and the promise the link made goes unkept.
	'drop-the-fragment': {
		target: 'card-shows-the-section-the-link-addressed',
		apply: rewriteModule('const fragment = decodeURIComponent(link.hash.slice(1));', "const fragment = '';"),
	},
	// The card grows to whatever it holds, so a long note pushes it off the
	// screen instead of scrolling inside it.
	'let-the-card-grow': {
		target: 'card-scrolls-inside-itself',
		apply: weakenStylesheet('.y-preview{max-block-size:none;overflow:visible}'),
	},
	// The selector widens to every wikilink, so a link the renderer marked as
	// landing somewhere other than it says gets a card promising otherwise.
	'preview-every-wikilink': {
		target: 'a-link-that-cannot-be-previewed-opens-nothing',
		apply: rewriteModule(':not(.wikilink-degraded)', ''),
	},
	// Nothing answers the key a reader presses to get a card out of the way
	// without moving the pointer.
	'deafen-escape': {
		target: 'escape-dismisses-the-card',
		apply: rewriteModule("if (event.key === 'Escape') close();", ''),
	},
	// The pointer leaves and the card stays, so it covers the paragraph the
	// reader went back to.
	'never-notice-the-pointer-leaving': {
		target: 'the-pointer-leaving-dismisses-the-card',
		apply: rewriteModule("link.addEventListener('pointerleave', release);", ''),
	},
	// The page moves under a card pinned to a link that is no longer there.
	'ignore-the-page-moving': {
		target: 'scrolling-dismisses-the-card',
		apply: rewriteModule('if (!card.contains(event.target)) close();', ''),
	},
	// Focus follows the card, so a reader tabbing through the prose is put
	// inside an excerpt and has to find their way back out of it.
	'pull-focus-into-the-card': {
		target: 'focus-stays-on-the-link',
		apply: rewriteModule(
			'if (!card.matches(\':popover-open\')) card.showPopover();',
			"if (!card.matches(':popover-open')) card.showPopover();\n      card.tabIndex = -1;\n      card.focus();",
		),
	},
	// The gate goes and a tap on a touch screen raises a card over the note it
	// was a request to open.
	'preview-on-any-pointer': {
		target: 'a-coarse-pointer-opens-no-card',
		apply: rewriteModule("if (!matchMedia('(pointer: fine)').matches) return;", ''),
	},
	// The refusal loses its words, which is the card that opens empty and
	// reads as a hover the page never registered.
	'answer-a-missing-note-with-nothing': {
		target: 'a-missing-note-answers-with-words',
		apply: rewriteFragment('y-preview__notice', 'y-preview__silence'),
	},
	// The refusal turns back into the whole note: the words on screen are the
	// destination's own, so nothing says they are not the section the link
	// named, and the reader reads the wrong passage believing it is the right
	// one.
	'widen-a-missing-section-to-the-note': {
		target: 'a-section-the-note-does-not-have-shows-none-of-it',
		apply: rewriteFragment('<p class="y-preview__notice"', '<div class="y-prose"><p>the whole note</p></div><p class="y-preview__notice"'),
	},
	// The refusal keeps its words and loses the one control it offers, leaving
	// a reader told what they cannot see and not told where it is.
	'remove-the-way-on': {
		target: 'a-section-the-note-does-not-have-shows-none-of-it',
		apply: rewriteFragment('y-preview__morelink', 'y-preview__nolink'),
	},
	// The excerpt keeps the names the renderer stamped on it, so while the card
	// is open the page holds two elements answering to one name and a fragment
	// naming it reaches whichever came first.
	're-anchor-the-excerpt': {
		target: 'an-open-card-adds-no-second-place-with-one-name',
		apply: rewriteFragment('<h2', '<h2 id="main-content"'),
	},
	// The card fills itself instead of asking the route that holds the
	// excerpts. It looks like a working card and is showing something nothing
	// in the vault said.
	'fill-the-card-without-asking': {
		target: 'the-excerpt-comes-from-this-origin',
		apply: rewriteModule(
			"const response = await fetch(url, { headers: { Accept: 'text/html' }, signal });",
			`const response = new Response('<div data-preview-body><div class="y-prose"><h2>Sensory material</h2></div></div>', { headers: { 'Content-Type': 'text/html' } });`,
		),
	},
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
	if (!SITES.includes(mutation.target)) {
		console.error(`preview-card: mutation ${name} aims at unknown site ${mutation.target}`);
		process.exit(2);
	}
}
for (const site of SITES) {
	if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
		console.error(`preview-card: assertion site ${site} has no mutation`);
		process.exit(2);
	}
}

if (MUTATE === 'list') {
	for (const name of Object.keys(MUTATIONS)) console.log(name);
	process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
	console.error(`preview-card: unknown MUTATE mode ${MUTATE}`);
	process.exit(2);
}

const mutation = MUTATE ? MUTATIONS[MUTATE] : null;

// Only the assertion a mode aims at proves that mode applied. Asking anywhere
// else would report not-applied for a mutation working perfectly where it was
// written to work.
const proveApplied = (site, proof) => {
	if (!mutation || mutation.target !== site) return;
	const issue = proof();
	if (issue) notApplied(`${MUTATE}: ${issue}`);
};

// Everything one look at the card can say, read in one pass so the parts
// cannot describe different moments.
const cardState = (page) =>
	page.evaluate(() => {
		const card = document.querySelector('[data-preview-card]');
		if (!card) return { present: false };
		const box = card.getBoundingClientRect();
		const active = document.activeElement;
		return {
			present: true,
			open: card.matches(':popover-open'),
			text: card.textContent.replace(/\s+/g, ' ').trim(),
			box: { top: box.top, left: box.left, right: box.right, bottom: box.bottom, width: box.width, height: box.height },
			contentHeight: card.scrollHeight,
			activeInCard: Boolean(active && card.contains(active)),
			viewport: { width: window.innerWidth, height: window.innerHeight },
			anchoredCount: document.querySelectorAll('[data-preview-open]').length,
			ids: [...document.querySelectorAll('[id]')].map((el) => el.id),
			activeText: active ? active.textContent.replace(/\s+/g, ' ').trim() : null,
		};
	});

const linkBox = (link) =>
	link.evaluate((el) => {
		const box = el.getBoundingClientRect();
		return { top: box.top, left: box.left, right: box.right, bottom: box.bottom };
	});

// Waits for the card to reach a state, and answers with whether it got there
// rather than throwing, so each caller words its own failure.
const settles = async (page, open, timeout) =>
	page
		.waitForFunction(
			(wanted) => Boolean(document.querySelector('[data-preview-card]')?.matches(':popover-open')) === wanted,
			open,
			{ timeout },
		)
		.then(
			() => true,
			() => false,
		);

const only = async (page, label) => {
	const found = page.locator(`main a:text-is("${label}")`);
	const count = await found.count();
	if (count !== 1) broken(`the source note carries ${count} links labelled ${JSON.stringify(label)}, want exactly 1`);
	return found;
};

const browser = await chromium.launch({ channel: 'chrome', headless: true });
try {
	const context = await browser.newContext({ viewport: { width: 1280, height: 800 } });
	const proof = mutation ? await mutation.apply(context) : null;

	// Every request the whole run makes, so the origin claim is made over what
	// the browser actually asked for rather than over what the source says.
	const offOrigin = [];
	const excerptRequests = [];
	const watch = (target) => {
		for (const event of ['request', 'requestfailed']) {
			target.on(event, (request) => {
				const url = request.url();
				if (url.startsWith(`${BASE}/preview/`)) excerptRequests.push(url);
				else if (!url.startsWith(BASE) && !url.startsWith('data:') && !url.startsWith('blob:')) offOrigin.push(url);
			});
		}
	};
	watch(context);

	const page = await context.newPage();
	const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
	if (!response || response.status() !== 200) broken(`the source note returned ${response?.status() ?? 'no response'}, want 200`);
	{
		const state = await cardState(page);
		if (!state.present) broken('the reading page carries no card element, so nothing below could ever open');
		if (state.open) broken('the card is open before anything was hovered');
	}

	// Hover. The delay is what makes a card an answer to a question rather than
	// a thing that happens while a pointer crosses a paragraph.
	const section = await only(page, SECTION_LINK);
	await section.hover();
	const opened = await settles(page, true, 4000);
	proveApplied('card-opens-on-hover', proof);
	if (!opened) {
		fail('card-opens-on-hover', `resting the pointer on ${JSON.stringify(SECTION_LINK)} opened no card`);
	}
	// Where the words in the card came from, asked of the browser's own record
	// of what it fetched rather than of the card's contents: a card can be full
	// and still be showing something no note said.
	proveApplied('the-excerpt-comes-from-this-origin', proof);
	if (excerptRequests.length === 0) {
		fail('the-excerpt-comes-from-this-origin', 'the card is open and this origin was never asked for an excerpt, so what it holds came from somewhere this probe has not looked');
	}
	if (offOrigin.length > 0) {
		fail('the-excerpt-comes-from-this-origin', `the page asked ${offOrigin.length} things of somewhere else, starting with ${offOrigin[0]}`);
	}

	{
		const state = await cardState(page);
		const box = await linkBox(section);
		proveApplied('card-anchored-to-its-link', proof);
		// Beside its own link, on one side of it or the other. The card is put
		// there by the stylesheet from an anchor the module names, so this is
		// the one assertion that fails when that name stops being written.
		const gap = Math.min(Math.abs(state.box.top - box.bottom), Math.abs(box.top - state.box.bottom));
		if (gap > 48) {
			fail('card-anchored-to-its-link', `the card's nearest edge sits ${Math.round(gap)}px from the link's own line, so it is not beside the link it belongs to`);
		}
		// Which side of the link the card takes is the stylesheet's own answer —
		// a card that would run off the window flips instead — so what is checked
		// is that the two boxes still share a column, not which edge they share.
		if (state.box.left > box.right + 24 || state.box.right < box.left - 24) {
			fail('card-anchored-to-its-link', `the card spans ${Math.round(state.box.left)}-${Math.round(state.box.right)}px and its link spans ${Math.round(box.left)}-${Math.round(box.right)}px, so the two are not over one another at all`);
		}

		proveApplied('card-shows-the-section-the-link-addressed', proof);
		if (!state.text.includes(SECTION_HEADING)) {
			fail('card-shows-the-section-the-link-addressed', `the card does not carry ${JSON.stringify(SECTION_HEADING)}, the section its link addressed; it reads ${JSON.stringify(state.text.slice(0, 160))}`);
		}
		if (state.text.includes(NOTE_OPENING)) {
			fail('card-shows-the-section-the-link-addressed', 'the card carries the destination\'s opening words, so the link\'s own fragment was not asked for');
		}

		// Two elements answering to one name is a defect no server-side test can
		// see: the excerpt is correct on its own and the page is correct on its
		// own, and only the two together are wrong.
		proveApplied('an-open-card-adds-no-second-place-with-one-name', proof);
		if (state.ids.length < 5) {
			broken(`the page names only ${state.ids.length} places with the card open, so a scan of them proves nothing`);
		}
		{
			const count = new Map();
			for (const id of state.ids) count.set(id, (count.get(id) ?? 0) + 1);
			for (const [id, n] of count) {
				if (n > 1) {
					fail('an-open-card-adds-no-second-place-with-one-name', `with the card open the id ${JSON.stringify(id)} is on this page ${n} times, so a fragment naming it reaches whichever came first`);
				}
			}
		}

		proveApplied('focus-stays-on-the-link', proof);
		if (state.activeInCard) {
			fail('focus-stays-on-the-link', `focus moved into the card, onto ${JSON.stringify(state.activeText?.slice(0, 60))}, so a reader tabbing through the prose is put inside an excerpt`);
		}
		if (state.anchoredCount !== 1) {
			broken(`${state.anchoredCount} links claim the card's anchor, want exactly 1`);
		}
	}

	// Escape, without moving the pointer at all.
	await page.keyboard.press('Escape');
	proveApplied('escape-dismisses-the-card', proof);
	if (!(await settles(page, false, 2000))) {
		fail('escape-dismisses-the-card', 'Escape left the card open, so a reader has to move the pointer to get the paragraph under it back');
	}

	// The keyboard. Reaching a link is already deliberate, so it opens at once.
	await section.focus();
	proveApplied('card-opens-on-focus', proof);
	if (!(await settles(page, true, 4000))) {
		fail('card-opens-on-focus', `reaching ${JSON.stringify(SECTION_LINK)} by keyboard opened no card, so the feature is a pointer's alone`);
	}
	await page.keyboard.press('Escape');
	await settles(page, false, 2000);

	// The same question asked of a section whose name is not ASCII.
	{
		const cjk = await only(page, CJK_LINK);
		await cjk.hover();
		if (!(await settles(page, true, 4000))) {
			broken(`resting the pointer on ${JSON.stringify(CJK_LINK)} opened no card, so the section it names cannot be looked for`);
		}
		const state = await cardState(page);
		proveApplied('card-shows-the-section-the-link-addressed', proof);
		if (!state.text.includes(CJK_HEADING)) {
			fail('card-shows-the-section-the-link-addressed', `the card does not carry ${JSON.stringify(CJK_HEADING)}, the section its link addressed; it reads ${JSON.stringify(state.text.slice(0, 160))}`);
		}
		await page.mouse.move(4, 4);
		await settles(page, false, 2000);
	}

	// The whole-note link, whose destination is long enough that the card has
	// to keep it inside itself.
	const whole = await only(page, WHOLE_NOTE_LINK);
	await whole.hover();
	if (!(await settles(page, true, 4000))) {
		broken(`resting the pointer on ${JSON.stringify(WHOLE_NOTE_LINK)} opened no card, so nothing below has a card to measure`);
	}
	{
		const state = await cardState(page);
		proveApplied('card-scrolls-inside-itself', proof);
		// The content, not the card: a card that grew to hold everything has
		// nothing left over to scroll, and asking it whether it overflows would
		// report exactly the defect below as a fixture that got shorter.
		if (state.contentHeight <= state.viewport.height) {
			broken(`the long destination fills ${Math.round(state.contentHeight)}px in an ${state.viewport.height}px window, so the fixture no longer holds enough for a card to have to scroll`);
		}
		if (state.box.height > state.viewport.height) {
			fail('card-scrolls-inside-itself', `the card is ${Math.round(state.box.height)}px tall in a ${state.viewport.height}px window, so the excerpt runs off the screen instead of scrolling inside the card`);
		}
	}

	// The pointer travels away, to a place no link lives.
	await page.mouse.move(4, 4);
	proveApplied('the-pointer-leaving-dismisses-the-card', proof);
	if (!(await settles(page, false, 2000))) {
		fail('the-pointer-leaving-dismisses-the-card', 'the pointer left the link and the card stayed over the words the reader went back to');
	}

	// Opened again, and this time the page moves under it. It is opened from
	// the keyboard with the pointer parked in a corner: scrolling takes the link
	// out from under a pointer resting on it, so a card dismissed by a hover
	// that ended would otherwise be read as a card dismissed by the scroll.
	await page.mouse.move(4, 4);
	if (!(await settles(page, false, 2000))) broken('the card would not close before the scroll check, so that check has no starting state');
	await whole.focus();
	if (!(await settles(page, true, 4000))) broken('the card did not reopen, so scrolling has nothing to dismiss');
	await page.mouse.wheel(0, 400);
	proveApplied('scrolling-dismisses-the-card', proof);
	if (!(await settles(page, false, 2000))) {
		fail('scrolling-dismisses-the-card', 'the page moved and the card stayed, pinned to a link that has gone');
	}
	await page.evaluate(() => window.scrollTo(0, 0));
	await page.evaluate(() => document.activeElement?.blur());
	await settles(page, false, 2000);

	// A passing pointer. The excerpt is already held from the hover above, so
	// what is being measured here is the wait and not a fetch.
	await page.mouse.move(4, 4);
	await settles(page, false, 2000);
	await whole.hover();
	await page.waitForTimeout(120);
	proveApplied('card-waits-out-a-passing-pointer', proof);
	{
		const state = await cardState(page);
		if (state.open) {
			fail('card-waits-out-a-passing-pointer', 'the card opened within 120ms of the pointer arriving, so crossing a paragraph of links flashes one for each');
		}
	}
	await page.mouse.move(4, 4);
	await settles(page, false, 2000);

	// The two links that must open nothing: one the renderer marked as landing
	// somewhere other than it says, and one that leaves this machine.
	for (const label of [DEGRADED_LINK, EXTERNAL_LINK]) {
		const link = await only(page, label);
		await link.hover();
		await page.waitForTimeout(700);
		proveApplied('a-link-that-cannot-be-previewed-opens-nothing', proof);
		const state = await cardState(page);
		if (state.open) {
			fail('a-link-that-cannot-be-previewed-opens-nothing', `${JSON.stringify(label)} opened a card, and it reads ${JSON.stringify(state.text.slice(0, 160))} — a promise this link cannot keep`);
		}
		await page.mouse.move(4, 4);
	}

	// What the route says about an address with no note behind it, asked from
	// the page itself so the answer travels the way the card's own does.
	{
		const answer = await page.evaluate(async () => {
			const response = await fetch('/preview/Notes/nobody-wrote-this.md', { headers: { Accept: 'text/html' } });
			return { status: response.status, body: await response.text() };
		});
		proveApplied('a-missing-note-answers-with-words', proof);
		if (answer.status !== 404) {
			fail('a-missing-note-answers-with-words', `an address with no note behind it answered ${answer.status}, want 404`);
		}
		if (!answer.body.includes('y-preview__notice')) {
			fail('a-missing-note-answers-with-words', `the refusal carries no sentence, so the card would open empty: ${JSON.stringify(answer.body.slice(0, 200))}`);
		}
		if (answer.body.includes('<html') || answer.body.includes('<script')) {
			fail('a-missing-note-answers-with-words', 'the refusal is a whole document rather than the fragment the card inserts');
		}
	}

	// A note that is there, addressed at a place inside it that is not. The note
	// is found, so the answer is not a refusal of the address; what it must not
	// do is hand over a different part of the note without saying so.
	{
		const widened = await page.evaluate(async () => {
			const response = await fetch('/preview/Notes/Glass%20Tide.md?section=nowhere-in-this-note', { headers: { Accept: 'text/html' } });
			return { status: response.status, body: await response.text() };
		});
		proveApplied('a-section-the-note-does-not-have-shows-none-of-it', proof);
		if (widened.status !== 200) {
			fail('a-section-the-note-does-not-have-shows-none-of-it', `a note that is there, addressed at a place it does not have, answered ${widened.status}; the note was found, so the answer is not a refusal of the address`);
		}
		if (widened.body.includes('y-prose')) {
			fail('a-section-the-note-does-not-have-shows-none-of-it', `the answer carries a passage of the note: ${JSON.stringify(widened.body.slice(0, 220))} — words the reader did not ask for, wearing the name of the section they did`);
		}
		if (!widened.body.includes('y-preview__notice')) {
			fail('a-section-the-note-does-not-have-shows-none-of-it', 'the answer shows nothing and says nothing about why');
		}
		if (!widened.body.includes('y-preview__morelink')) {
			fail('a-section-the-note-does-not-have-shows-none-of-it', 'the answer refuses and offers no way on to the note itself');
		}
	}

	if (offOrigin.length > 0) {
		fail('the-excerpt-comes-from-this-origin', `the page asked ${offOrigin.length} things of somewhere else, starting with ${offOrigin[0]}`);
	}
	await page.close();
	await context.close();

	// A touch screen, where a tap on a link is a request to open the note. The
	// emulation is checked before it is relied on: a context that failed to
	// become coarse would report a card that never appeared as a pass.
	const touch = await browser.newContext({ viewport: { width: 1280, height: 800 }, hasTouch: true, isMobile: true });
	if (mutation) await mutation.apply(touch);
	watch(touch);
	const tapping = await touch.newPage();
	const touchResponse = await tapping.goto(BASE + PAGE, { waitUntil: 'networkidle' });
	if (!touchResponse || touchResponse.status() !== 200) broken(`the source note returned ${touchResponse?.status() ?? 'no response'} to the touch context`);
	const coarse = await tapping.evaluate(() => matchMedia('(pointer: coarse)').matches);
	if (!coarse) broken('the touch context still reports a fine pointer, so this check would pass over an emulation that never happened');
	const touchLink = tapping.locator(`main a:text-is("${SECTION_LINK}")`);
	await touchLink.hover();
	await tapping.waitForTimeout(900);
	proveApplied('a-coarse-pointer-opens-no-card', proof);
	{
		const state = await cardState(tapping);
		if (state.present && state.open) {
			fail('a-coarse-pointer-opens-no-card', 'a card opened on a touch screen, over the note the tap was a request to open');
		}
	}
	await touch.close();

	console.log(
		'PASS preview-card: the card waits for a held pointer, opens beside its own link with the section that link addressed, keeps focus where it was, goes on Escape, on the pointer leaving and on the page moving, and never appears under a touch',
	);
} catch (err) {
	if (err instanceof NotApplied) {
		console.error(err.message);
		console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
		process.exitCode = 2;
	} else if (err instanceof LockFired) {
		console.error(err.message);
		if (MUTATE) {
			const { target } = MUTATIONS[MUTATE];
			if (err.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
			else console.error(`no catch: ${MUTATE} targets ${target}, but ${err.site} fired first`);
		}
		process.exitCode = 1;
	} else if (err instanceof ProbeBroken) {
		console.error(err.message);
		process.exitCode = 1;
	} else {
		console.error(err);
		process.exitCode = 1;
	}
} finally {
	await browser.close();
}
