// Browser lock for the text-token floor. The least-prominent text tokens are
// used at 10-13px, including inside elevated and hover surfaces, so they owe
// the normal-text WCAG AA ratio (4.5:1), not the 3:1 large-text threshold.
// Chrome performs the OKLCH conversion; the lock reads the resulting pixels
// from a canvas rather than maintaining a second color-conversion algorithm.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/alpha.md';
const MUTATE = process.env.MUTATE || '';
const SITES = ['light-text-aa', 'dark-text-aa'];

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN contrast-contract: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL contrast-contract: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN contrast-contract: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED contrast-contract: ${message}`); };

const weakenStylesheet = (rule) => async (page) => {
  let requests = 0;
  await page.route('**/static/app.css', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => requests === 1 ? '' : `app.css was requested ${requests} times, want exactly 1`;
};

const MUTATIONS = {
  'weaken-light-faint-text': {
    target: 'light-text-aa',
    apply: weakenStylesheet(':root,[data-theme="light"]{--fg-faint:oklch(0.585 0.018 107)}'),
  },
  'weaken-dark-faint-text': {
    target: 'dark-text-aa',
    apply: weakenStylesheet('[data-theme="dark"]{--fg-faint:oklch(0.540 0.012 72)}'),
  },
};

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`contrast-contract: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`contrast-contract: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`contrast-contract: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const measureTheme = (page, theme) => page.evaluate((selectedTheme) => {
  document.documentElement.dataset.theme = selectedTheme;
  const root = getComputedStyle(document.documentElement);
  const textTokens = ['--fg', '--fg-muted', '--fg-subtle', '--fg-faint'];
  const surfaces = ['--bg', '--panel', '--elevated', '--overlay'];
  const canvas = document.createElement('canvas');
  canvas.width = 1;
  canvas.height = 1;
  const context = canvas.getContext('2d', { willReadFrequently: true });
  if (!context) return { issue: 'a 2D canvas context is unavailable' };

  const rgb = (property) => {
    const value = root.getPropertyValue(property).trim();
    if (!value) return { issue: `${property} is empty` };
    context.clearRect(0, 0, 1, 1);
    context.fillStyle = value;
    context.fillRect(0, 0, 1, 1);
    const pixel = context.getImageData(0, 0, 1, 1).data;
    if (pixel[3] !== 255) return { issue: `${property} rasterized with alpha ${pixel[3]}` };
    return { value, channels: [pixel[0], pixel[1], pixel[2]] };
  };
  const luminance = (channels) => {
    const linear = channels.map((channel) => {
      const value = channel / 255;
      return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
    });
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
  };

  const measurements = [];
  for (const text of textTokens) {
    const foreground = rgb(text);
    if (foreground.issue) return { issue: foreground.issue };
    for (const surface of surfaces) {
      const background = rgb(surface);
      if (background.issue) return { issue: background.issue };
      const a = luminance(foreground.channels);
      const b = luminance(background.channels);
      const ratio = (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
      measurements.push({ text, surface, ratio, foreground: foreground.value, background: background.value });
    }
  }
  return { measurements };
}, theme);

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  proof = MUTATE ? await MUTATIONS[MUTATE].apply(page) : null;
  const response = await page.goto(BASE + PAGE, { waitUntil: 'networkidle' });
  if (!response || response.status() !== 200) broken(`navigation returned ${response?.status() ?? 'no response'}, want 200`);
  if (proof) {
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
  }

  for (const theme of ['light', 'dark']) {
    const result = await measureTheme(page, theme);
    if (result.issue) broken(result.issue);
    const weakest = result.measurements.reduce((left, right) => left.ratio < right.ratio ? left : right);
    if (weakest.ratio < 4.5) {
      fail(`${theme}-text-aa`, `${weakest.text} on ${weakest.surface} is ${weakest.ratio.toFixed(3)}:1 (${weakest.foreground} on ${weakest.background}), want at least 4.5:1`);
    }
  }

  console.log('PASS contrast-contract: every text token clears 4.5:1 on every light and dark product surface');
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
