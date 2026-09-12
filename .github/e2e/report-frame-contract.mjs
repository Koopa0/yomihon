// Layout lock for the report shell: the sandboxed iframe must fill the reading
// column, and a visible link beside the title must open the frame's own address.
//
// Go tests cannot see the CSS half. This probe measures the painted iframe on
// the e2e vault's browser-boundary briefing.
//
// Env: YOMIHON_BASE, PAGE_PATH (defaults to /reports/browser-boundary.html),
// and MUTATE. MUTATE=list prints every watched regression.
import { chromium } from "playwright-core";

const BASE = process.env.YOMIHON_BASE || "http://127.0.0.1:9610";
const PAGE =
	process.env.PAGE_PATH || "/reports/browser-boundary.html";
const MUTATE = process.env.MUTATE || "";
const SITES = ["report-frame-fills-column", "report-raw-link-matches-frame"];

const MIN_FRAME_HEIGHT = 400;

class LockFired extends Error {
	constructor(site, message) {
		super(message);
		this.site = site;
	}
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
	if (!SITES.includes(site))
		throw new ProbeBroken(
			`BROKEN report-frame-contract: unknown assertion site ${site}`,
		);
	throw new LockFired(site, `FAIL report-frame-contract: ${message}`);
};
const broken = (message) => {
	throw new ProbeBroken(`BROKEN report-frame-contract: ${message}`);
};
const notApplied = (message) => {
	throw new NotApplied(`NOT-APPLIED report-frame-contract: ${message}`);
};

const stripReportFrameHeight = async (page) => {
	let seen = 0;
	await page.route("**/static/app.css", async (route) => {
		const response = await route.fetch();
		const original = await response.text();
		seen += 1;
		await route.fulfill({
			response,
			body: `${original}\n.y-reportframe{height:auto!important;flex:1 1 auto!important;min-height:0!important}\n`,
		});
	});
	return () =>
		seen > 0
			? ""
			: "the stylesheet was never requested, so the collapsed frame rule reached no page";
};

const hideReportRawLink = async (page) => {
	let seen = 0;
	await page.route("**/static/app.css", async (route) => {
		const response = await route.fetch();
		const original = await response.text();
		seen += 1;
		await route.fulfill({
			response,
			body: `${original}\n.y-reportraw{display:none!important}\n`,
		});
	});
	return () =>
		seen > 0
			? ""
			: "the stylesheet was never requested, so the hidden-link rule reached no page";
};

const breakReportRawLinkHref = async (page) => {
	let requests = 0;
	let matches = 0;
	await page.route(BASE + PAGE, async (route) => {
		requests += 1;
		const response = await route.fetch();
		const original = await response.text();
		const rewritten = original.replace(
			/(<a class="y-reportraw" href=")([^"]+)(")/,
			(_match, prefix, href, suffix) => {
				matches += 1;
				return `${prefix}${href}-drift${suffix}`;
			},
		);
		await route.fulfill({ response, body: rewritten });
	});
	return () => {
		if (requests !== 1)
			return `report shell was requested ${requests} times, want exactly 1`;
		if (matches !== 1)
			return `report raw link href matched ${matches} times, want exactly 1`;
		return "";
	};
};

const MUTATIONS = {
	"strip-report-frame-height": {
		target: "report-frame-fills-column",
		apply: stripReportFrameHeight,
	},
	"hide-report-raw-link": {
		target: "report-raw-link-matches-frame",
		apply: hideReportRawLink,
	},
	"break-report-raw-link-href": {
		target: "report-raw-link-matches-frame",
		apply: breakReportRawLinkHref,
	},
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
	if (!SITES.includes(mutation.target)) {
		console.error(
			`report-frame-contract: mutation ${name} aims at unknown site ${mutation.target}`,
		);
		process.exit(2);
	}
}
for (const site of SITES) {
	if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
		console.error(
			`report-frame-contract: assertion site ${site} has no mutation`,
		);
		process.exit(2);
	}
}

if (MUTATE === "list") {
	for (const name of Object.keys(MUTATIONS)) console.log(name);
	process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
	console.error(`report-frame-contract: unknown MUTATE mode ${MUTATE}`);
	process.exit(2);
}

const measureReportShell = (page) =>
	page.evaluate(() => {
		const frame = document.querySelector(".y-reportframe");
		const link = document.querySelector(".y-reportraw");
		return {
			frameHeight: frame?.getBoundingClientRect().height ?? 0,
			frameSrc: frame?.getAttribute("src") ?? "",
			linkHref: link?.getAttribute("href") ?? "",
			linkVisible: link ? link.getClientRects().length > 0 : false,
		};
	});

const browser = await chromium.launch({ channel: "chrome", headless: true });
let proof = null;
try {
	const context = await browser.newContext({
		viewport: { width: 1440, height: 900 },
	});
	const page = await context.newPage();
	proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
	await page.goto(BASE + PAGE, { waitUntil: "domcontentloaded" });
	if (proof) {
		const issue = proof();
		if (issue) notApplied(`${MUTATE}: ${issue}`);
	}

	const box = await measureReportShell(page);
	if (box.frameSrc === "") {
		broken("the report shell paints no .y-reportframe to measure");
	}
	if (box.frameHeight < MIN_FRAME_HEIGHT) {
		fail(
			"report-frame-fills-column",
			`iframe height=${box.frameHeight}px, want at least ${MIN_FRAME_HEIGHT}px`,
		);
	}
	if (box.linkHref === "") {
		fail(
			"report-raw-link-matches-frame",
			"the report shell paints no .y-reportraw link beside the title",
		);
	}
	if (!box.linkVisible) {
		fail(
			"report-raw-link-matches-frame",
			"the report raw link is not visible on the painted page",
		);
	}
	if (box.linkHref !== box.frameSrc) {
		fail(
			"report-raw-link-matches-frame",
			`raw link href=${JSON.stringify(box.linkHref)} does not match iframe src=${JSON.stringify(box.frameSrc)}`,
		);
	}

	await context.close();
	console.log(
		`PASS report-frame-contract: iframe height=${box.frameHeight}px and raw link points at ${box.linkHref}`,
	);
} catch (error) {
	if (error instanceof NotApplied) {
		console.error(error.message);
		console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
		process.exitCode = 2;
	} else if (error instanceof LockFired) {
		console.error(error.message);
		if (MUTATE) {
			const { target } = MUTATIONS[MUTATE];
			if (error.site === target)
				console.log(`MUTATE-RESULT: caught ${MUTATE}`);
			else
				console.error(
					`no catch: ${MUTATE} targets ${target}, but ${error.site} fired first`,
				);
		}
		process.exitCode = 1;
	} else if (error instanceof ProbeBroken) {
		console.error(error.message);
		process.exitCode = 1;
	} else {
		console.error(error);
		process.exitCode = 1;
	}
} finally {
	await browser.close();
}
