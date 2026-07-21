// Browser lock for the one-source brand system: one canonical SVG favicon and
// one decorative current-color header mark beside the visible lowercase name.
// Env: YOMIHON_BASE, PAGE_PATH, and MUTATE.
import { chromium } from "playwright-core";

const BASE = process.env.YOMIHON_BASE || "http://127.0.0.1:9610";
const PAGE = process.env.PAGE_PATH || "/";
const MARK_PATH = "/static/yomihon-mark.svg";
const MUTATE = process.env.MUTATE || "";
const SITES = [
	"favicon",
	"accessible-name",
	"decorative-mark",
	"mask-source",
	"current-color",
	"rendered-silhouette",
	"forced-colors",
	"forced-colors-silhouette",
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
	if (!SITES.includes(site)) {
		throw new ProbeBroken(
			`BROKEN brand-contract: unknown assertion site ${site}`,
		);
	}
	throw new LockFired(site, `FAIL brand-contract: ${message}`);
};
const broken = (message) => {
	throw new ProbeBroken(`BROKEN brand-contract: ${message}`);
};
const notApplied = (message) => {
	throw new NotApplied(`NOT-APPLIED brand-contract: ${message}`);
};

const rewriteDocument = (needle, replacement, label) => async (page) => {
	let requests = 0;
	let matches = 0;
	await page.route(BASE + PAGE, async (route) => {
		requests += 1;
		const response = await route.fetch();
		const original = await response.text();
		matches += original.split(needle).length - 1;
		await route.fulfill({
			response,
			body: original.replace(needle, replacement),
		});
	});
	return () => {
		if (requests !== 1)
			return `${label} document was requested ${requests} times, want exactly 1`;
		if (matches !== 1)
			return `${label} needle matched ${matches} times, want exactly 1`;
		return "";
	};
};

const rewriteStylesheet = (check, rule, label) => async (page) => {
	let requests = 0;
	let matches = 0;
	await page.route("**/static/app.css", async (route) => {
		requests += 1;
		const response = await route.fetch();
		const original = await response.text();
		matches += check(original);
		await route.fulfill({ response, body: `${original}\n${rule}\n` });
	});
	return () => {
		if (requests !== 1)
			return `${label} stylesheet was requested ${requests} times, want exactly 1`;
		if (matches !== 1)
			return `${label} contract matched ${matches} times, want exactly 1`;
		return "";
	};
};

const collapseMarkPath = async (page) => {
	let requests = 0;
	let matches = 0;
	let changes = 0;
	await page.route(`**${MARK_PATH}`, async (route) => {
		requests += 1;
		const response = await route.fetch();
		const original = await response.text();
		const paths = original.match(/<path d="[^"]+"\/>/g) || [];
		matches += paths.length;
		let body = original;
		if (paths.length === 1 && paths[0] !== '<path d="M0 0"/>') {
			body = original.replace(paths[0], '<path d="M0 0"/>');
			changes += 1;
		}
		await route.fulfill({ response, body });
	});
	return () => {
		if (requests < 1) return "brand mark was never requested";
		if (matches !== requests)
			return `brand path matched ${matches} times across ${requests} responses, want exactly once per response`;
		if (changes !== requests)
			return `brand path changed ${changes} times across ${requests} responses, want every response changed`;
		return "";
	};
};

const exactCount = (source, needle) => source.split(needle).length - 1;

const MUTATIONS = {
	"rewrite-favicon-path": {
		target: "favicon",
		apply: rewriteDocument(
			MARK_PATH,
			"/static/missing-yomihon-mark.svg",
			"favicon path",
		),
	},
	"rename-wordmark": {
		target: "accessible-name",
		apply: rewriteDocument(
			"<span>yomihon</span>",
			"<span>Yomihon</span>",
			"visible wordmark",
		),
	},
	"expose-decorative-mark": {
		target: "decorative-mark",
		apply: rewriteDocument(
			'class="y-brand__mark" aria-hidden="true"',
			'class="y-brand__mark" aria-hidden="false"',
			"decorative mark",
		),
	},
	"drop-mask-source": {
		target: "mask-source",
		apply: rewriteStylesheet(
			(source) => {
				const standard = exactCount(source, `mask-image:url(${MARK_PATH})`);
				const prefixed = exactCount(
					source,
					`-webkit-mask-image:url(${MARK_PATH})`,
				);
				return standard === 2 && prefixed === 1 ? 1 : 0;
			},
			".y-brand__mark{-webkit-mask-image:none!important;mask-image:none!important}",
			"brand mask",
		),
	},
	"detach-current-color": {
		target: "current-color",
		apply: rewriteStylesheet(
			(source) =>
				exactCount(source, "background-color:currentColor") === 1 ? 1 : 0,
			".y-brand__mark{background-color:transparent!important}",
			"current-color paint",
		),
	},
	"collapse-svg-path": {
		target: "rendered-silhouette",
		apply: collapseMarkPath,
	},
	"hide-in-forced-colors": {
		target: "forced-colors",
		apply: rewriteStylesheet(
			(source) =>
				exactCount(source, "@media (forced-colors:active)") === 1 &&
				exactCount(source, "background-color:canvastext") === 1 &&
				exactCount(source, "forced-color-adjust:none") === 1
					? 1
					: 0,
			"@media (forced-colors:active){.y-brand__mark{background-color:transparent!important}}",
			"forced-colors paint",
		),
	},
	"drop-forced-colors-mask": {
		target: "forced-colors-silhouette",
		apply: rewriteStylesheet(
			(source) =>
				exactCount(source, "@media (forced-colors:active)") === 1 &&
				exactCount(source, `mask-image:url(${MARK_PATH})`) === 2
					? 1
					: 0,
			"@media (forced-colors:active){.y-brand__mark{-webkit-mask-image:none!important;mask-image:none!important}}",
			"forced-colors mask",
		),
	},
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
	if (!SITES.includes(mutation.target)) {
		console.error(
			`brand-contract: mutation ${name} aims at unknown site ${mutation.target}`,
		);
		process.exit(2);
	}
}
for (const site of SITES) {
	if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
		console.error(`brand-contract: assertion site ${site} has no mutation`);
		process.exit(2);
	}
}

if (MUTATE === "list") {
	for (const name of Object.keys(MUTATIONS)) console.log(name);
	process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
	console.error(`brand-contract: unknown MUTATE mode ${MUTATE}`);
	process.exit(2);
}

const markPresentation = (mark) =>
	mark.evaluate((element) => {
		const style = getComputedStyle(element);
		const rect = element.getBoundingClientRect();
		const canvas = document.createElement("canvas");
		canvas.width = 1;
		canvas.height = 1;
		const context = canvas.getContext("2d", { willReadFrequently: true });
		if (!context) return { issue: "a 2D canvas context is unavailable" };
		const channels = (color) => {
			context.clearRect(0, 0, 1, 1);
			context.fillStyle = color;
			context.fillRect(0, 0, 1, 1);
			return Array.from(context.getImageData(0, 0, 1, 1).data);
		};
		const backgroundRGBA = channels(style.backgroundColor);
		const colorRGBA = channels(style.color);
		return {
			backgroundColor: style.backgroundColor,
			color: style.color,
			backgroundRGBA,
			colorRGBA,
			maskImage: style.maskImage,
			webkitMaskImage: style.webkitMaskImage,
			width: rect.width,
			height: rect.height,
		};
	});

const renderedSilhouette = async (page, mark) => {
	const screenshot = await mark.screenshot({ animations: "disabled" });
	return page.evaluate(
		async (source) => {
			const image = new Image();
			image.src = source;
			await image.decode();
			const canvas = document.createElement("canvas");
			canvas.width = image.naturalWidth;
			canvas.height = image.naturalHeight;
			const context = canvas.getContext("2d", { willReadFrequently: true });
			if (!context) return { issue: "a 2D screenshot canvas is unavailable" };
			context.drawImage(image, 0, 0);
			const data = context.getImageData(0, 0, canvas.width, canvas.height).data;
			const pixel = (xRatio, yRatio) => {
				const x = Math.floor((canvas.width - 1) * xRatio);
				const y = Math.floor((canvas.height - 1) * yRatio);
				return Array.from(
					data.slice(
						(y * canvas.width + x) * 4,
						(y * canvas.width + x) * 4 + 4,
					),
				);
			};
			return {
				width: canvas.width,
				height: canvas.height,
				samples: {
					upperLeft: pixel(0.22, 0.2),
					upperOpen: pixel(0.5, 0.2),
					upperRight: pixel(0.78, 0.2),
					junction: pixel(0.5, 0.58),
					descender: pixel(0.5, 0.82),
					lowerLeft: pixel(0.22, 0.82),
					lowerRight: pixel(0.78, 0.82),
				},
			};
		},
		`data:image/png;base64,${screenshot.toString("base64")}`,
	);
};

const maxChannelDelta = (left, right) =>
	Math.max(
		...left
			.slice(0, 3)
			.map((channel, index) => Math.abs(channel - right[index])),
	);

const assertRenderedSilhouette = (site, label, silhouette) => {
	if (silhouette.issue) broken(silhouette.issue);
	if (silhouette.width < 32 || silhouette.height < 32) {
		broken(
			`${label} screenshot is ${silhouette.width}x${silhouette.height}, want at least 32x32 device pixels`,
		);
	}
	const {
		upperLeft,
		upperOpen,
		upperRight,
		junction,
		descender,
		lowerLeft,
		lowerRight,
	} = silhouette.samples;
	const foreground = [upperLeft, upperRight, junction, descender];
	const background = [upperOpen, lowerLeft, lowerRight];
	const foregroundDrift = Math.max(
		...foreground.slice(1).map((sample) => maxChannelDelta(upperLeft, sample)),
	);
	const backgroundDrift = Math.max(
		...background.slice(1).map((sample) => maxChannelDelta(upperOpen, sample)),
	);
	const contrast = maxChannelDelta(upperLeft, upperOpen);
	if (foregroundDrift > 24 || backgroundDrift > 24 || contrast < 48) {
		fail(
			site,
			`${label} rendered pixels are ${JSON.stringify(silhouette)}, want two branches and a descender around an open upper field and clear lower corners`,
		);
	}
};

const browser = await chromium.launch({ channel: "chrome", headless: true });
let proof = null;
try {
	const page = await browser.newPage({
		viewport: { width: 1280, height: 800 },
		deviceScaleFactor: 2,
	});
	proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
	const response = await page.goto(BASE + PAGE, { waitUntil: "networkidle" });
	if (!response || response.status() !== 200) {
		broken(
			`navigation returned ${response?.status() ?? "no response"}, want 200`,
		);
	}
	if (proof) {
		const issue = proof();
		if (issue) notApplied(`${MUTATE}: ${issue}`);
	}

	const favicons = page.locator('head link[rel~="icon"]');
	if ((await favicons.count()) !== 1) {
		fail(
			"favicon",
			`the document has ${await favicons.count()} icon links, want exactly 1`,
		);
	}
	const favicon = await favicons.first().evaluate((element) => ({
		href: element.getAttribute("href"),
		type: element.getAttribute("type"),
	}));
	if (favicon.href !== MARK_PATH || favicon.type !== "image/svg+xml") {
		fail(
			"favicon",
			`favicon contract is ${JSON.stringify(favicon)}, want ${MARK_PATH} as image/svg+xml`,
		);
	}

	const brand = page.locator("a.y-brand__name");
	if ((await brand.count()) !== 1)
		broken(`the header has ${await brand.count()} brand links, want 1`);
	const namedBrand = page.getByRole("link", { name: "yomihon", exact: true });
	if ((await namedBrand.count()) !== 1 || !(await namedBrand.isVisible())) {
		fail(
			"accessible-name",
			"the visible Home link does not have the exact accessible name yomihon",
		);
	}

	const mark = brand.locator(".y-brand__mark");
	if (
		(await mark.count()) !== 1 ||
		(await mark.getAttribute("aria-hidden")) !== "true"
	) {
		fail(
			"decorative-mark",
			"the header mark is not one aria-hidden decorative child of the brand link",
		);
	}

	for (const theme of ["light", "dark"]) {
		await page.evaluate((value) => {
			document.documentElement.dataset.theme = value;
		}, theme);
		const presentation = await markPresentation(mark);
		if (presentation.issue) broken(presentation.issue);
		const mask =
			presentation.maskImage !== "none"
				? presentation.maskImage
				: presentation.webkitMaskImage;
		if (
			!mask.includes(MARK_PATH) ||
			presentation.width < 16 ||
			presentation.height < 16
		) {
			fail(
				"mask-source",
				`${theme} mark presentation is ${JSON.stringify(presentation)}, want the canonical mask at 16px or larger`,
			);
		}
		if (
			presentation.backgroundRGBA.join(",") !==
				presentation.colorRGBA.join(",") ||
			presentation.backgroundRGBA[3] !== 255
		) {
			fail(
				"current-color",
				`${theme} mark paint is ${JSON.stringify(presentation)}, want opaque currentColor`,
			);
		}
		assertRenderedSilhouette(
			"rendered-silhouette",
			theme,
			await renderedSilhouette(page, mark),
		);
	}

	await page.emulateMedia({ forcedColors: "active" });
	const forced = await markPresentation(mark);
	if (forced.issue) broken(forced.issue);
	if (forced.backgroundRGBA[3] !== 255 || !(await namedBrand.isVisible())) {
		fail(
			"forced-colors",
			`forced-colors mark/name presentation is ${JSON.stringify(forced)}, want an opaque mark and visible name`,
		);
	}
	const forcedMask =
		forced.maskImage !== "none" ? forced.maskImage : forced.webkitMaskImage;
	if (!forcedMask.includes(MARK_PATH)) {
		fail(
			"forced-colors-silhouette",
			`forced-colors mask is ${JSON.stringify(forced)}, want the canonical mask to remain active`,
		);
	}
	assertRenderedSilhouette(
		"forced-colors-silhouette",
		"forced colors",
		await renderedSilhouette(page, mark),
	);

	console.log(
		"PASS brand-contract: one local favicon and one decorative current-color mark preserve the exact yomihon name in light, dark, and forced colors",
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
	await browser.close();
}
