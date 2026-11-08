// Browser containment lock for authored Markdown HTML and the first-party
// reading shell. It starts a controlled second origin rather than inferring
// privacy from DOM shape: a green run means the hostile fixtures caused zero
// requests there while the application module still ran.
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints the watched-red modes.
import { createServer } from "node:http";

import { chromium } from "playwright-core";

const BASE = process.env.YOMIHON_BASE || "http://127.0.0.1:9610";
const READING_PATH = "/notes/Notes/browser-boundary.md";
const MUTATE = process.env.MUTATE || "";
const ATTACKER_TOKEN = "BROWSER_BOUNDARY_ATTACKER";

const SITES = [
	"authored-markup-inert",
	"authored-remote-resource-explicit",
	"response-nonce-bound",
	"application-runtime-survives",
	"reading-csp-defense-in-depth",
];

const MUTATIONS = {
	"activate-attributed-kbd": {
		target: "authored-markup-inert",
		phase: "reading",
	},
	"escape-bare-kbd": { target: "authored-markup-inert", phase: "reading" },
	"activate-authored-script": {
		target: "authored-markup-inert",
		phase: "reading",
	},
	"restore-authored-remote-image": {
		target: "authored-remote-resource-explicit",
		phase: "reading",
	},
	"mismatch-response-nonce": {
		target: "response-nonce-bound",
		phase: "reading",
	},
	"break-application-entry": {
		target: "application-runtime-survives",
		phase: "reading",
	},
	"weaken-reading-csp": {
		target: "reading-csp-defense-in-depth",
		phase: "bypass",
	},
};

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
			`BROKEN browser-boundary: unknown assertion site ${site}`,
		);
	throw new LockFired(site, `FAIL browser-boundary: ${message}`);
};
const broken = (message) => {
	throw new ProbeBroken(`BROKEN browser-boundary: ${message}`);
};
const notApplied = (message) => {
	throw new NotApplied(`NOT-APPLIED browser-boundary: ${message}`);
};
const occurrences = (body, needle) => body.split(needle).length - 1;

for (const [name, mutation] of Object.entries(MUTATIONS)) {
	if (!SITES.includes(mutation.target)) {
		console.error(
			`browser-boundary: mutation ${name} aims at unknown site ${mutation.target}`,
		);
		process.exit(2);
	}
}
for (const site of SITES) {
	if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
		console.error(`browser-boundary: assertion site ${site} has no mutation`);
		process.exit(2);
	}
}

if (MUTATE === "list") {
	for (const name of Object.keys(MUTATIONS)) console.log(name);
	process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
	console.error(`browser-boundary: unknown MUTATE mode ${MUTATE}`);
	process.exit(2);
}

const mutationFor = (phase) => MUTATE && MUTATIONS[MUTATE].phase === phase;

const startAttacker = async () => {
	const requests = [];
	const server = createServer((request, response) => {
		requests.push(request.url || "/");
		response.setHeader("Access-Control-Allow-Origin", "*");
		if (request.url === "/remote-script") {
			response.setHeader("Content-Type", "application/javascript");
			response.end("globalThis.injectedRemoteScriptRan=true;");
			return;
		}
		if (request.url?.includes("image")) {
			response.setHeader("Content-Type", "image/gif");
			response.end(
				Buffer.from("R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=", "base64"),
			);
			return;
		}
		response.statusCode = 204;
		response.end();
	});
	const close = async () => {
		if (!server.listening) return;
		await new Promise((resolve, reject) =>
			server.close((error) => (error ? reject(error) : resolve())),
		);
	};
	try {
		await new Promise((resolve, reject) => {
			server.once("error", reject);
			server.listen(0, "127.0.0.1", resolve);
		});
	} catch (error) {
		await close();
		throw error;
	}
	const address = server.address();
	if (!address || typeof address === "string") {
		await close();
		broken("controlled second origin has no TCP address");
	}
	return {
		authority: `127.0.0.1:${address.port}`,
		requests,
		reset() {
			requests.length = 0;
		},
		close,
	};
};

const weakenedReadingPolicy = () =>
	`default-src * data: blob: 'unsafe-inline'; base-uri *; connect-src *; font-src * data:; ` +
	`form-action *; frame-ancestors 'none'; frame-src *; img-src * data: blob:; media-src *; ` +
	`object-src *; script-src * 'unsafe-inline'; style-src * 'unsafe-inline'; worker-src *`;

const installReadingResponse = async (page, attacker, phase) => {
	const proof = { requests: 0, attackerTokens: 0, mutationMatches: 0, csp: "" };
	await page.route(BASE + READING_PATH, async (route) => {
		proof.requests += 1;
		const response = await route.fetch();
		const original = await response.text();
		proof.csp = response.headers()["content-security-policy"] || "";
		proof.attackerTokens = occurrences(original, ATTACKER_TOKEN);
		let body = original.replaceAll(ATTACKER_TOKEN, attacker.authority);
		let headers;

		if (phase === "reading" && mutationFor(phase)) {
			switch (MUTATE) {
				case "activate-attributed-kbd": {
					const needle = "&lt;kbd onclick=globalThis.noteKbdEventRan=true&gt;";
					proof.mutationMatches = occurrences(body, needle);
					body = body.replace(
						needle,
						"<kbd onclick=globalThis.noteKbdEventRan=true>",
					);
					break;
				}
				case "escape-bare-kbd": {
					const needle = "<kbd>Ctrl</kbd>";
					proof.mutationMatches = occurrences(body, needle);
					body = body.replace(needle, "&lt;kbd&gt;Ctrl&lt;/kbd&gt;");
					break;
				}
				case "activate-authored-script": {
					const needle =
						"&lt;script&gt;globalThis.noteScriptRan=true&lt;/script&gt;";
					proof.mutationMatches = occurrences(body, needle);
					body = body.replace(
						needle,
						"<script>globalThis.noteScriptRan=true</script>",
					);
					break;
				}
				case "restore-authored-remote-image": {
					const needle = `<a href="http://${attacker.authority}/markdown-image" rel="external noreferrer" referrerpolicy="no-referrer">remote pixel</a>`;
					proof.mutationMatches = occurrences(body, needle);
					body = body.replace(
						needle,
						`<img src="http://${attacker.authority}/markdown-image" alt="remote pixel">`,
					);
					break;
				}
				case "mismatch-response-nonce": {
					const needle = /<script nonce="[^"]+" type="module"/g;
					proof.mutationMatches = [...body.matchAll(needle)].length;
					body = body.replace(
						needle,
						'<script nonce="wrong-response-nonce" type="module"',
					);
					break;
				}
				case "break-application-entry": {
					const needle = /src="\/static\/yomihon\.js(?:\?v=[a-f0-9]{12})?"/g;
					proof.mutationMatches = [...body.matchAll(needle)].length;
					body = body.replace(
						needle,
						'src="/static/missing-browser-boundary.js"',
					);
					break;
				}
				default:
					break;
			}
		}

		if (phase === "bypass") {
			const needle = "<p>BROWSER-BOUNDARY-END</p>";
			const injected = [
				"<script>globalThis.injectedInlineRan=true</script>",
				'<button onclick="globalThis.injectedEventRan=true">injected event</button>',
				`<script src="http://${attacker.authority}/remote-script"></script>`,
				'<script src="/static/yomihon.js?hostile-boundary"></script>',
				`<img src="http://${attacker.authority}/image">`,
				`<link rel="prefetch" href="http://${attacker.authority}/prefetch">`,
				`<iframe src="http://${attacker.authority}/frame"></iframe>`,
				`<video poster="http://${attacker.authority}/poster"><source src="http://${attacker.authority}/media"></video>`,
				`<style>@import url("http://${attacker.authority}/import");.injected{background:url("http://${attacker.authority}/style")}</style>`,
			].join("");
			const matches = occurrences(body, needle);
			if (matches !== 1)
				broken(`bypass injection marker matched ${matches}, want exactly 1`);
			body = body.replace(needle, needle + injected);
			if (mutationFor(phase)) {
				proof.mutationMatches = proof.csp ? 1 : 0;
				headers = response.headers();
				headers["content-security-policy"] = weakenedReadingPolicy();
				delete headers["content-length"];
				delete headers["content-encoding"];
			}
		}

		if (headers) await route.fulfill({ response, body, headers });
		else await route.fulfill({ response, body });
	});
	return proof;
};

const proveResponseRoute = (proof, phase) => {
	if (proof.requests !== 1)
		broken(
			`${phase} document was requested ${proof.requests} times, want exactly 1`,
		);
	if (proof.attackerTokens === 0)
		broken(`${phase} fixture contains no ${ATTACKER_TOKEN} tokens`);
	if (mutationFor(phase) && proof.mutationMatches !== 1) {
		notApplied(
			`${MUTATE} matched ${proof.mutationMatches} production sites, want exactly 1`,
		);
	}
};

let browser;
let attacker;
try {
	browser = await chromium.launch({ channel: "chrome", headless: true });
	attacker = await startAttacker();
	// Case 1: the renderer grants only the ruled inert subset, binds every app
	// script to the response nonce, and leaves ordinary app interactions alive.
	{
		attacker.reset();
		const context = await browser.newContext({
			viewport: { width: 1280, height: 800 },
		});
		const page = await context.newPage();
		const proof = await installReadingResponse(page, attacker, "reading");
		await page.goto(BASE + READING_PATH, { waitUntil: "domcontentloaded" });
		proveResponseRoute(proof, "reading");

		const prose = page.locator(".y-prose");
		if ((await prose.count()) !== 1)
			broken(
				`reading page has ${await prose.count()} .y-prose regions, want 1`,
			);
		if (
			(await prose.locator('ruby[lang="ja"] > rt[lang="ja"]').count()) !== 1 ||
			(await prose.locator("br").count()) !== 1
		) {
			fail(
				"authored-markup-inert",
				"the ruled ruby/rt/br subset did not survive rendering",
			);
		}
		const attributedFormatting = await prose
			.locator("kbd,sub,sup,mark,u")
			.evaluateAll(
				(elements) =>
					elements.filter((element) => element.attributes.length !== 0).length,
			);
		if (attributedFormatting !== 0) {
			fail(
				"authored-markup-inert",
				`attributed formatting element count=${attributedFormatting}, want 0`,
			);
		}
		for (const [tag, words] of [
			["kbd", "Ctrl"],
			["sub", "2"],
			["sup", "2"],
			["mark", "marked"],
			["u", "under"],
		]) {
			const elements = prose.locator(tag);
			if (
				(await elements.count()) !== 1 ||
				(await elements.textContent()) !== words ||
				(await elements.evaluate((element) => element.attributes.length)) !== 0
			) {
				fail(
					"authored-markup-inert",
					`the ruled bare ${tag} text/element did not survive rendering`,
				);
			}
		}
		const activeAuthored = await prose
			.locator(
				"script,meta,button[onclick],kbd[onclick],sub[onclick],sup[onclick],mark[onclick],u[onclick],link,iframe,form,video,source,style",
			)
			.count();
		const proseText = await prose.textContent();
		if (
			activeAuthored !== 0 ||
			!proseText.includes("globalThis.noteScriptRan=true")
		) {
			fail(
				"authored-markup-inert",
				`authored active-element count=${activeAuthored}, script source visible=${proseText.includes("globalThis.noteScriptRan=true")}`,
			);
		}

		const remoteLink = prose.locator(
			`a[href="http://${attacker.authority}/markdown-image"]`,
		);
		if (
			(await remoteLink.count()) !== 1 ||
			(await remoteLink.textContent()) !== "remote pixel"
		) {
			fail(
				"authored-remote-resource-explicit",
				"the remote Markdown image is not one explicit external link",
			);
		}
		await page.waitForTimeout(250);
		if (attacker.requests.length !== 0) {
			fail(
				"authored-remote-resource-explicit",
				`authored content made automatic requests: ${attacker.requests.join(", ")}`,
			);
		}

		const nonceMatch = proof.csp.match(
			/script-src 'nonce-([^']+)' 'strict-dynamic'/,
		);
		const scripts = await page
			.locator("script")
			.evaluateAll((elements) => elements.map((element) => element.nonce));
		if (
			!nonceMatch ||
			scripts.length < 2 ||
			scripts.some((nonce) => nonce !== nonceMatch[1]) ||
			!proof.csp.includes("script-src-attr 'none'") ||
			proof.csp.includes("script-src 'self'")
		) {
			fail(
				"response-nonce-bound",
				`CSP/scripts disagree: csp=${JSON.stringify(proof.csp)}, script nonces=${JSON.stringify(scripts)}`,
			);
		}

		await page
			.waitForFunction(
				() => document.documentElement.dataset.js === "on",
				null,
				{ timeout: 2000 },
			)
			.catch(() => {});
		const filterHidden = await page
			.locator("[data-nav-filter]")
			.getAttribute("hidden");
		if (
			(await page.evaluate(() => document.documentElement.dataset.js)) !==
				"on" ||
			filterHidden !== null
		) {
			fail(
				"application-runtime-survives",
				`module boot/filter reveal failed: data-js=${await page.evaluate(() => document.documentElement.dataset.js)}, hidden=${filterHidden}`,
			);
		}
		// The chord, not the header control. Pressing that control opens the
		// palette because the markup says so, which a browser does whether or
		// not a single line of ours ran — so it would answer here for the wrong
		// reason. A chord is not a press on anything the markup can name, so it
		// reaches the palette only through the module this case is about.
		await page.keyboard.press("ControlOrMeta+k");
		if (
			!(await page.locator("[data-search]").evaluate((dialog) => dialog.open))
		) {
			fail(
				"application-runtime-survives",
				"the nonce-authorized module did not answer the palette chord",
			);
		}
		await context.close();
	}

	// Case 2: inject active markup after the renderer. This does not excuse a
	// renderer regression; it independently proves CSP stops execution and all
	// automatic fetches if markup reaches the response through a future bug.
	{
		attacker.reset();
		const context = await browser.newContext({
			viewport: { width: 1280, height: 800 },
		});
		const page = await context.newPage();
		const sameOriginHostileResponses = [];
		page.on("response", (response) => {
			if (response.url().includes("hostile-boundary"))
				sameOriginHostileResponses.push(response.url());
		});
		const proof = await installReadingResponse(page, attacker, "bypass");
		await page.goto(BASE + READING_PATH, { waitUntil: "domcontentloaded" });
		proveResponseRoute(proof, "bypass");
		await page.waitForTimeout(300);
		const state = await page.evaluate(() => ({
			inline: globalThis.injectedInlineRan === true,
			event: globalThis.injectedEventRan === true,
			remote: globalThis.injectedRemoteScriptRan === true,
			app: document.documentElement.dataset.js,
		}));
		if (
			state.inline ||
			state.event ||
			state.remote ||
			state.app !== "on" ||
			attacker.requests.length !== 0 ||
			sameOriginHostileResponses.length !== 0
		) {
			fail(
				"reading-csp-defense-in-depth",
				`injected state=${JSON.stringify(state)}, attacker=${JSON.stringify(attacker.requests)}, unnonced-self-responses=${JSON.stringify(sameOriginHostileResponses)}`,
			);
		}
		await context.close();
	}

	console.log(
		"PASS browser-boundary: authored markup is inert, app nonces work, and the reading surface made zero second-origin requests",
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
			if (error.site === target) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
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
	const closers = [];
	if (browser) closers.push(browser.close());
	if (attacker) closers.push(attacker.close());
	const results = await Promise.allSettled(closers);
	for (const result of results) {
		if (result.status === "rejected") {
			console.error("browser-boundary cleanup failed", result.reason);
			process.exitCode = 1;
		}
	}
}
