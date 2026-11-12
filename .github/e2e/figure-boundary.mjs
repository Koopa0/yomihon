// Browser lock for figures: a vault's interactive HTML, each in a frame of its
// own. It starts controlled outside listeners — one per way out, so a
// connection says which way it came — and logs every HTTP request, every TCP
// connection and every UDP packet that reaches them. Logging only requests
// would read a connection that never sent one as closed.
//
// A green run means: a figure nobody approved caused no traffic of any kind
// and ran no script; a figure that frames another page was refused and caused
// none either; an approved figure ran exactly its own two scripts, was told
// the theme, set its frame's height, and reached the outside only through the
// residuals listed in KNOWN_RESIDUALS. Any other channel turns it red.
//
// Env: YOMIHON_BASE, YOMIHON_FIXTURE_ROOT, YOMIHON_FIXTURE_HOME, PAGE_PATH and
// MUTATE. MUTATE=list prints the modes.
import { createHash } from 'node:crypto';
import { createSocket } from 'node:dgram';
import { mkdir, readFile, realpath, rm, writeFile } from 'node:fs/promises';
import { createServer } from 'node:http';
import { basename, dirname, join } from 'node:path';

import { chromium } from 'playwright-core';

import { arrived } from './support/arrival.mjs';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/figures.md';
const MUTATE = process.env.MUTATE || '';
const FIGURES = 'Notes/figures';
const APPROVED = ['moving-figure.html', 'leaving-figure.html'];

// The ways out an approved figure still has, measured and accepted for
// approved figures only: a WebRTC peer sends UDP to its STUN address, and a
// frame the figure creates or a navigation it starts opens a TCP connection
// before the page's frame-src refuses the request. Each is allowed here as a
// connection or a packet and never as an HTTP request; anything else that
// arrives is a new channel.
const KNOWN_RESIDUALS = {
  webrtc: 'udp',
  frame: 'tcp',
  leave: 'tcp',
};

// Channels the approved moving figure tries, each on its own listener.
const CHANNELS = [
  'fetch', 'xhr', 'beacon', 'image', 'websocket', 'eventsource', 'worker', 'frame', 'preconnect',
  'prefetch', 'script', 'import', 'style', 'open', 'form', 'ping',
];

const SITES = [
  'frame-attributes',
  'scriptless-no-traffic',
  'unapproved-runs-nothing',
  'refused-not-served',
  'approved-runs-its-own-text',
  'only-known-residuals',
  'height-from-the-figure-only',
];

const MUTATIONS = {
  // The approved frame gains same-origin access to the page.
  'frame-same-origin': 'frame-attributes',
  // The scriptless policy admits pictures from anywhere.
  'weaken-scriptless-policy': 'scriptless-no-traffic',
  // A figure nobody approved is served as if approved.
  'unapproved-as-approved': 'unapproved-runs-nothing',
  // The route serves a figure it refuses.
  'serve-refused-figure': 'refused-not-served',
  // The approved figure's policy loses the hash of its second script.
  'drop-second-hash': 'approved-runs-its-own-text',
  // The approved figure's policy admits connections.
  'open-connect-src': 'only-known-residuals',
  // The page's frame-src admits any address.
  'widen-page-frame-src': 'only-known-residuals',
  // The page takes a height from any window.
  'height-from-any-window': 'height-from-the-figure-only',
};

class LockFired extends Error {
  constructor(site, message) {
    super(message);
    this.site = site;
  }
}
class ProbeBroken extends Error {}
class NotApplied extends Error {}

for (const [mode, site] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(site)) {
    console.error(`figure-boundary: mutation ${mode} aims at unknown site ${site}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).includes(site)) {
    console.error(`figure-boundary: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}
if (MUTATE === 'list') {
  for (const mode of Object.keys(MUTATIONS)) console.log(mode);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`figure-boundary: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

const fail = (site, message) => {
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN figure-boundary: unknown site ${site}`);
  throw new LockFired(site, `FAIL figure-boundary: ${message}`);
};
const applied = new Set();
// once rewrites exactly one site; a needle that matches none or several is a
// mutation that did not apply, which is reported rather than read as green.
const once = (body, needle, replacement, mode) => {
  const count = body.split(needle).length - 1;
  if (count !== 1) throw new NotApplied(`NOT-APPLIED figure-boundary: ${mode} matched ${count} sites`);
  applied.add(mode);
  return body.replace(needle, replacement);
};

// ---------- the outside ----------
const events = [];
const closers = [];
const listenTCP = async (channel) => {
  const server = createServer((request, response) => {
    events.push({ channel, kind: 'http', detail: `${request.method} ${request.url}` });
    response.setHeader('Access-Control-Allow-Origin', '*');
    response.end('ok');
  });
  server.on('connection', () => events.push({ channel, kind: 'tcp' }));
  server.on('upgrade', (request, socket) => {
    events.push({ channel, kind: 'http', detail: `UPGRADE ${request.url}` });
    socket.destroy();
  });
  await new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  closers.push(() => new Promise((resolve) => server.close(resolve)));
  return `127.0.0.1:${server.address().port}`;
};
const listenUDP = async (channel) => {
  const socket = createSocket('udp4');
  socket.on('message', () => events.push({ channel, kind: 'udp' }));
  await new Promise((resolve) => socket.bind(0, '127.0.0.1', resolve));
  closers.push(() => new Promise((resolve) => socket.close(resolve)));
  return `127.0.0.1:${socket.address().port}`;
};

// ---------- approvals, written where the server reads them ----------
const fixtureRoot = async () => {
  const supplied = process.env.YOMIHON_FIXTURE_ROOT;
  if (!supplied) throw new ProbeBroken('BROKEN figure-boundary: no disposable fixture root');
  const root = await realpath(supplied);
  if (basename(root) !== 'vault' || !basename(dirname(root)).startsWith('yomihon-serve.')) {
    throw new ProbeBroken('BROKEN figure-boundary: unsafe fixture root');
  }
  return root;
};
const fixtureHome = async (root) => {
  const supplied = process.env.YOMIHON_FIXTURE_HOME;
  if (!supplied) throw new ProbeBroken('BROKEN figure-boundary: no disposable server home');
  const home = await realpath(supplied);
  if (basename(home) !== 'home' || dirname(home) !== dirname(root)) {
    throw new ProbeBroken('BROKEN figure-boundary: the server home is not beside the disposable vault');
  }
  return home;
};
// The approval command needs a person at a terminal, which a probe is not, so
// the record it would write is written here in its documented shape: the
// per-vault directory named by the digest of the resolved root.
const approve = async (root, home) => {
  const config = process.platform === 'darwin' ? join(home, 'Library', 'Application Support') : join(home, '.config');
  const vaultDir = join(config, 'yomihon', createHash('sha256').update(root.normalize('NFC')).digest('hex').slice(0, 32));
  const approvals = [];
  for (const name of APPROVED) {
    const bytes = await readFile(join(root, FIGURES, name));
    approvals.push({ path: `${FIGURES}/${name}`, sha256: createHash('sha256').update(bytes).digest('hex'), time: new Date().toISOString() });
  }
  await mkdir(vaultDir, { recursive: true, mode: 0o700 });
  const file = join(vaultDir, 'figures.json');
  await writeFile(file, `${JSON.stringify({ version: 1, vault: root, approvals }, null, 2)}\n`, { mode: 0o600 });
  return file;
};

// ---------- the run ----------
let browser;
let approvalsFile;
let routeError;
try {
  const root = await fixtureRoot();
  const home = await fixtureHome(root);
  approvalsFile = await approve(root, home);

  const outside = {};
  for (const channel of CHANNELS) outside[channel] = await listenTCP(channel);
  outside.webrtc = await listenUDP('webrtc');
  const still = await listenTCP('still');
  const idle = await listenTCP('idle');
  const refused = await listenTCP('refused');
  const leave = await listenTCP('leave');
  const map = Object.entries(outside).map(([channel, at]) => `${channel}=${at}`).join(';');
  const tokens = [
    ['FIGURE_OUTSIDE_STILL', still],
    ['FIGURE_OUTSIDE_IDLE', idle],
    ['FIGURE_OUTSIDE_REFUSED', refused],
    ['FIGURE_OUTSIDE_LEAVE', leave],
    ['FIGURE_OUTSIDE_MAP', map],
  ];
  const withOutside = (body) => tokens.reduce((text, [token, value]) => text.replaceAll(token, value), body);

  // The outside listeners are on loopback, which Chrome's local-network
  // checks treat as private and a real remote host is not. Left on, they
  // block some requests to the stand-in that would reach a real one, and the
  // probe would report a way out closed that is open on the internet.
  browser = await chromium.launch({ channel: 'chrome', headless: true, args: ['--disable-features=LocalNetworkAccessChecks'] });
  const context = await browser.newContext();
  // The reading page keeps a list of every message it receives, and which
  // figure frame, if any, sent it. Only the top page: a figure gets nothing.
  await context.addInitScript(() => {
    if (window.top !== window) return;
    window.__figureMessages = [];
    window.addEventListener('message', (event) => {
      const frames = [...document.querySelectorAll('iframe.figure-embed__frame')];
      const from = frames.find((frame) => frame.contentWindow === event.source);
      window.__figureMessages.push({ data: event.data, from: from ? from.getAttribute('src') : null });
    });
  });

  // Only yomihon's own responses are rewritten. A request to the outside is
  // left to the browser alone: intercepting it would put the probe between
  // the figure and the listener whose log is the evidence.
  await context.route(`${BASE}/**`, async (route) => {
    try {
      const url = new URL(route.request().url());
      if (url.pathname === PAGE) {
        const response = await route.fetch();
        let body = await response.text();
        const headers = response.headers();
        if (MUTATE === 'frame-same-origin') {
          body = once(body, `src="/embed/${FIGURES}/moving-figure.html" title="moving-figure.html" sandbox="allow-scripts"`,
            `src="/embed/${FIGURES}/moving-figure.html" title="moving-figure.html" sandbox="allow-scripts allow-same-origin"`, MUTATE);
        }
        if (MUTATE === 'unapproved-as-approved') {
          body = once(body, `src="/embed/${FIGURES}/idle-figure.html" title="idle-figure.html" sandbox=""`,
            `src="/embed/${FIGURES}/idle-figure.html" title="idle-figure.html" sandbox="allow-scripts"`, MUTATE);
        }
        if (MUTATE === 'serve-refused-figure') {
          const stand = body.match(/<div class="figure-embed figure-embed--refused">[\s\S]*?<\/div>/g) || [];
          body = once(body, stand[0] ?? '<no refused stand-in>',
            `<iframe class="figure-embed__frame" src="/embed/${FIGURES}/refused-figure.html" title="refused" sandbox="" height="100"></iframe>`, MUTATE);
        }
        if (MUTATE === 'widen-page-frame-src') {
          headers['content-security-policy'] = once(headers['content-security-policy'], "frame-src 'self'", 'frame-src *', MUTATE);
        }
        return route.fulfill({ response, body, headers });
      }
      if (url.pathname === '/static/diagrams.js' && MUTATE === 'height-from-any-window') {
        const response = await route.fetch();
        let body = await response.text();
        body = once(body, "if (event.origin !== 'null') return;", '', MUTATE);
        body = once(body, 'if (frame.contentWindow !== event.source) continue;', '', MUTATE);
        return route.fulfill({ response, body });
      }
      if (url.pathname.startsWith(`/embed/${FIGURES}/`)) {
        const name = url.pathname.slice(`/embed/${FIGURES}/`.length);
        const response = await route.fetch();
        const headers = response.headers();
        let body = withOutside(await response.text());
        let status = response.status();
        const policy = headers['content-security-policy'] || '';
        if (MUTATE === 'weaken-scriptless-policy' && name === 'still-figure.html') {
          headers['content-security-policy'] = once(policy, "img-src 'none'", 'img-src *', MUTATE);
        }
        if (MUTATE === 'unapproved-as-approved' && name === 'idle-figure.html') {
          headers['content-security-policy'] = once(once(policy, 'sandbox;', 'sandbox allow-scripts;', MUTATE),
            "script-src 'none'", "script-src 'unsafe-inline'", MUTATE);
        }
        if (MUTATE === 'serve-refused-figure' && name === 'refused-figure.html') {
          body = withOutside(await readFile(join(root, FIGURES, name), 'utf8'));
          headers['content-type'] = 'text/html; charset=utf-8';
          status = 200;
          applied.add(MUTATE);
        }
        if (MUTATE === 'drop-second-hash' && name === 'moving-figure.html') {
          const hashes = policy.match(/'sha256-[^']+'/g) || [];
          headers['content-security-policy'] = once(policy, ` ${hashes.at(-1)}`, '', MUTATE);
        }
        if (MUTATE === 'open-connect-src' && name === 'moving-figure.html') {
          headers['content-security-policy'] = once(policy, "connect-src 'none'", 'connect-src *', MUTATE);
        }
        return route.fulfill({ status, body, headers });
      }
      return route.continue();
    } catch (error) {
      routeError ??= error;
      return route.abort();
    }
  });

  const page = await context.newPage();
  await page.goto(BASE + PAGE);
  await arrived(page, 10000);
  if (routeError) throw routeError;
  if (!(await page.content()).includes('FIGURES-END')) throw new ProbeBroken('BROKEN figure-boundary: the fixture page did not arrive whole');

  // Wait for the approved figure to have spoken, then for the slow ways out
  // (a WebRTC gather, a delayed navigation) to have had their chance.
  const messages = async () => page.evaluate(() => window.__figureMessages);
  const said = (list, probe, src) => list.some((m) => m.data && m.data.probe === probe && (src === undefined || m.from === src));
  const moving = `/embed/${FIGURES}/moving-figure.html`;
  for (let i = 0; i < 100 && !said(await messages(), 'second-ran', moving) && !said(await messages(), 'first-ran', moving); i++) {
    await page.waitForTimeout(100);
  }
  await page.waitForTimeout(4000);
  if (routeError) throw routeError;
  const heard = await messages();

  // scriptless-no-traffic and unapproved-runs-nothing
  for (const channel of ['still', 'idle']) {
    const seen = events.filter((e) => e.channel === channel);
    if (seen.length) {
      fail(channel === 'still' ? 'scriptless-no-traffic' : 'unapproved-runs-nothing',
        `a figure nobody approved reached the outside: ${JSON.stringify(seen)}`);
    }
  }
  if (said(heard, 'idle-ran')) fail('unapproved-runs-nothing', 'a figure nobody approved ran its script');

  // frame-attributes
  const frames = await page.evaluate(() => [...document.querySelectorAll('iframe.figure-embed__frame')].map((frame) => ({
    src: frame.getAttribute('src'),
    sandbox: frame.getAttribute('sandbox'),
    referrer: frame.getAttribute('referrerpolicy'),
    allow: frame.getAttribute('allow'),
    title: frame.getAttribute('title'),
    height: frame.getAttribute('height'),
  })));
  const want = {
    [`/embed/${FIGURES}/still-figure.html`]: '',
    [moving]: 'allow-scripts',
    [`/embed/${FIGURES}/leaving-figure.html`]: 'allow-scripts',
    [`/embed/${FIGURES}/idle-figure.html`]: '',
  };
  for (const [src, sandbox] of Object.entries(want)) {
    const frame = frames.find((f) => f.src === src);
    if (!frame) fail('frame-attributes', `no frame for ${src}`);
    if (frame.sandbox !== sandbox) fail('frame-attributes', `${src} sandbox=${JSON.stringify(frame.sandbox)}, want ${JSON.stringify(sandbox)}`);
    if (frame.referrer !== 'no-referrer' || frame.allow !== '' || !frame.title) {
      fail('frame-attributes', `${src} has referrerpolicy=${frame.referrer} allow=${JSON.stringify(frame.allow)} title=${JSON.stringify(frame.title)}`);
    }
  }
  if (said(heard, 'parent-readable')) fail('frame-attributes', 'the approved figure read the page around it');

  // refused-not-served
  const refusal = await page.request.get(`${BASE}/embed/${FIGURES}/refused-figure.html`);
  if (refusal.status() !== 422 || (await refusal.text()).includes('FIGURE_OUTSIDE')) {
    fail('refused-not-served', `the route answered the refused figure with ${refusal.status()}`);
  }
  if (frames.some((f) => f.src.endsWith('/refused-figure.html')) || events.some((e) => e.channel === 'refused')) {
    fail('refused-not-served', `the refused figure was framed or reached the outside: ${JSON.stringify(events.filter((e) => e.channel === 'refused'))}`);
  }

  // approved-runs-its-own-text
  for (const probe of ['first-ran', 'second-ran']) {
    if (!said(heard, probe, moving)) fail('approved-runs-its-own-text', `the approved figure's ${probe.replace('-ran', '')} script did not run`);
  }
  if (!said(heard, 'leaving-ran')) fail('approved-runs-its-own-text', 'the approved leaving figure did not run');
  for (const probe of ['eval-ran', 'injected-ran', 'srcdoc-ran']) {
    if (said(heard, probe)) fail('approved-runs-its-own-text', `script outside the approved text ran: ${probe}`);
  }
  const theme = await page.evaluate(() => document.documentElement.dataset.theme
    || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'));
  const told = heard.find((m) => m.data && m.data.probe === 'theme' && m.from === moving);
  if (!told || told.data.value !== theme) fail('approved-runs-its-own-text', `the approved figure was told theme ${JSON.stringify(told?.data.value)}, want ${theme}`);

  // only-known-residuals
  const fresh = events.filter((e) => !['still', 'idle', 'refused'].includes(e.channel));
  const unexpected = fresh.filter((e) => e.kind === 'http' || KNOWN_RESIDUALS[e.channel] !== e.kind);
  if (unexpected.length) fail('only-known-residuals', `an approved figure reached the outside by a new channel: ${JSON.stringify(unexpected)}`);
  const residual = Object.fromEntries(Object.entries(KNOWN_RESIDUALS).map(([channel, kind]) => [
    channel, `${kind} x${fresh.filter((e) => e.channel === channel).length}`,
  ]));
  console.log(`figure-boundary: known residuals of approved figures this run: ${JSON.stringify(residual)}`);

  // height-from-the-figure-only
  const height = frames.find((f) => f.src === moving)?.height;
  if (height !== '333') fail('height-from-the-figure-only', `the approved figure's frame is ${height} high, want the 333 it reported`);
  await page.evaluate(() => window.postMessage({ type: 'yomihon-figure-height', height: 99 }, '*'));
  await page.waitForTimeout(200);
  const after = await page.evaluate((src) => document.querySelector(`iframe[src="${src}"]`).getAttribute('height'), moving);
  if (after !== '333') fail('height-from-the-figure-only', `a message from the page itself set a figure's height to ${after}`);

  if (MUTATE && !applied.has(MUTATE)) throw new NotApplied(`NOT-APPLIED figure-boundary: ${MUTATE} never applied`);
  console.log('figure-boundary: ok');
} catch (error) {
  if (error instanceof NotApplied) {
    console.error(error.message);
    process.exitCode = 2;
  } else if (error instanceof LockFired) {
    console.error(error.message);
    if (MUTATE && MUTATIONS[MUTATE] === error.site && applied.has(MUTATE)) console.log(`MUTATE-RESULT: caught ${MUTATE}`);
    process.exitCode = 1;
  } else {
    console.error(error instanceof ProbeBroken ? error.message : `BROKEN figure-boundary: ${error.stack || error}`);
    process.exitCode = 1;
  }
} finally {
  if (browser) await browser.close();
  for (const close of closers) await close();
  if (approvalsFile) await rm(approvalsFile, { force: true });
}
