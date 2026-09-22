// CI-only journey over an isolated synthetic vault. This drives the compiled
// reader, native source links and browser history; it never calls a write route.
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from 'playwright-core';

const binary = resolve(process.argv[2]);
const evidence = resolve(process.argv[3]);
assert.equal(process.env.GITHUB_ACTIONS, 'true', 'this journey may run only in GitHub Actions');
const work = await mkdtemp(join(tmpdir(), 'yomihon-source-locations-'));
const vault = join(work, 'vault');
const base = 'http://127.0.0.1:19869';
const longLabel = 'The method for the frozen event handbook, including the exact scope and the observation that does not establish an interval for changing team documents';
const source = '# Source\n\n' + 'Introduction context.\n\n'.repeat(65) +
  '## Methods\n\nOnly a frozen event handbook was tested.\n\n' + 'Intermediate context.\n\n'.repeat(60) +
  '## Limitations\n\nChanging team documents were not tested.\n\n' +
  'A single observed boundary. ^Quote-1\n\n' + 'Final context.\n\n'.repeat(30);
const declarations = [
  'Source#Limitations|Study limitations', 'Other#observation',
  `Source#Methods|${longLabel}`, 'Source#methods|Discarded duplicate',
  'Source#^Quote-1', 'Source', 'Source#Missing|Missing evidence',
  'Source#^absent', 'unwritten', 'twin',
];
const claim = '# Claim\n\nDoes the frozen-handbook study justify the same interval for changing team documents?\n\n' +
  'A location names evidence to inspect; it does not establish that the claim is true.\n\n'.repeat(30);
const note = (refs) => `---\ntitle: Claim\nbased_on:\n${refs.map((ref) => `  - "[[${ref}]]"`).join('\n')}\n---\n\n${claim}`;
await mkdir(evidence, { recursive: true });
await mkdir(join(vault, 'A'), { recursive: true });
await mkdir(join(vault, 'B'), { recursive: true });
for (const [name, body] of Object.entries({
  'Source.md': source,
  'Other.md': '## Observation\n\nA second source observation.\n',
  'Claim.md': note(declarations),
  'File-only.md': note(['Source', 'Other', 'unwritten', 'twin']),
  'Single.md': '---\nbased_on: "[[Source#methods]]"\n---\nOne location.\n',
  'A/twin.md': 'One ambiguous note.\n',
  'B/twin.md': 'Another ambiguous note.\n',
})) await writeFile(join(vault, name), body);

const child = spawn(binary, ['serve', vault], {
  env: { ...process.env, HOME: work, XDG_CONFIG_HOME: join(work, '.config'), YOMIHON_PORT: '19869' },
  stdio: ['ignore', 'pipe', 'pipe'],
});
let serverLog = '';
child.stdout.on('data', (data) => { serverLog += data; });
child.stderr.on('data', (data) => { serverLog += data; });
const report = { journeys: [], costs: null };
let browser;

async function activate(page, target, input) {
  if (input === 'keyboard') {
    // Discover the element through the browser's native sequential focus order.
    // Do not programmatically focus a link that a keyboard reader cannot reach.
    for (let step = 0; step < 200; step++) {
      if (await target.evaluate((el) => el === document.activeElement)) {
        await page.keyboard.press('Enter');
        return;
      }
      await page.keyboard.press('Tab');
    }
    assert.fail('the intended control was not reachable through Tab');
  } else if (input === 'touch') {
    await target.tap();
  } else {
    await target.click();
  }
}

async function poll(page, predicate, argument) {
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline) {
    if (await page.evaluate(predicate, argument)) return;
    await delay(50);
  }
  assert.fail('the destination did not settle within 10 seconds');
}

async function sources(page, input) {
  const disclosure = page.locator('details').filter({ has: page.locator('.y-basedon') });
  if (await disclosure.isVisible() && await disclosure.getAttribute('open') === null) {
    await activate(page, disclosure.locator('summary'), input);
  }
  const nav = page.locator('.y-basedon:visible');
  assert.equal(await nav.count(), 1, 'exactly one responsive source block is visible');
  return nav;
}

async function follow(page, label, id, input, journeyName) {
  const nav = await sources(page, input);
  const link = nav.getByRole('link', { name: label, exact: true });
  await link.scrollIntoViewIfNeeded();
  const beforeInputTraversal = await link.evaluate((el) => ({ scroll: scrollY, linkTop: el.getBoundingClientRect().top }));
  await activate(page, link, input);
  await page.waitForURL((url) => url.pathname === '/notes/Source.md' && decodeURIComponent(url.hash.slice(1)) === id);
  await poll(page, (anchor) => {
    const el = document.getElementById(anchor);
    if (!el) return false;
    const top = el.getBoundingClientRect().top;
    return top >= 56 && top < 200;
  }, id);
  const destination = await page.evaluate((anchor) => ({
    scroll: scrollY,
    top: document.getElementById(anchor).getBoundingClientRect().top,
  }), id);
  assert.ok(destination.scroll > 1000, 'the link reaches a passage beyond the opening viewport');
  const passage = id === 'methods' ? 'Only a frozen event handbook was tested.' :
    id === 'limitations' ? 'Changing team documents were not tested.' : 'A single observed boundary.';
  assert.ok(await page.locator('.y-prose').getByText(passage, { exact: id !== '^quote-1' }).isVisible(), 'the intended evidence is present');
  await page.screenshot({ path: join(evidence, `destination-${journeyName}-${id.replace('^', 'block-')}.png`) });
  await page.goBack();
  await page.waitForURL('**/notes/Claim.md');
  assert.ok(await page.locator('.y-prose').getByText('Does the frozen-handbook study justify the same interval for changing team documents?', { exact: true }).isVisible(), 'Back returns to the question being checked');
  const nativeBack = await page.evaluate(() => {
    const details = [...document.querySelectorAll('details')].find((el) => el.querySelector('.y-basedon'));
    return { scroll: scrollY, detailsOpen: details?.open ?? null };
  });
  const disclosure = page.locator('details').filter({ has: page.locator('.y-basedon') });
  const reopened = await disclosure.isVisible() && nativeBack.detailsOpen === false;
  const returned = await sources(page, input);
  assert.ok(await returned.getByRole('link', { name: label, exact: true }).isVisible(), 'the same source remains available after Back');
  destination.returnContext = {
    beforeInputTraversal, nativeBack,
    afterReopening: await returned.getByRole('link', { name: label, exact: true }).evaluate((el) => ({ scroll: scrollY, linkTop: el.getBoundingClientRect().top })),
    reopened,
  };
  return destination;
}

async function journey(width, javaScriptEnabled, input) {
  const name = `${width}-${javaScriptEnabled ? 'js' : 'no-js'}-${input}`;
  const context = await browser.newContext({
    viewport: { width, height: 900 }, reducedMotion: 'reduce', javaScriptEnabled,
    hasTouch: input === 'touch', isMobile: input === 'touch',
  });
  const page = await context.newPage();
  try {
    await page.goto(`${base}/notes/Claim.md`);
    let nav = await sources(page, input);
    assert.equal(await nav.locator('.ui-navitem__count').innerText(), '4', 'badge counts distinct groups including unresolved declarations');
    const links = await nav.locator('a').evaluateAll((anchors) => anchors.map((a) => ({ text: a.textContent.trim(), href: a.getAttribute('href') })));
    assert.deepEqual(links.map((link) => link.href), [
      '/notes/Source.md', '/notes/Source.md#limitations', '/notes/Source.md#methods',
      '/notes/Source.md#%5Equote-1', '/notes/Source.md', '/notes/Source.md', '/notes/Other.md#observation',
    ], 'file groups and locations retain authored order with duplicates collapsed');
    assert.equal(links[2].text, longLabel, 'first authored label wins');
    assert.ok((await nav.innerText()).includes('[[unwritten]]') && (await nav.innerText()).includes('[[twin]]'), 'unresolved and ambiguous declarations remain visible');
    const overflow = await nav.evaluate((el) => el.scrollWidth > el.clientWidth);
    assert.equal(overflow, false, 'long distinguishing labels wrap inside the source block');
    await nav.screenshot({ path: join(evidence, `sources-${width}-${javaScriptEnabled ? 'js' : 'no-js'}-${input}.png`) });
    const methods = await follow(page, longLabel, 'methods', input, name);
    const limitations = await follow(page, 'Study limitations', 'limitations', input, name);
    const block = await follow(page, '^Quote-1', '^quote-1', input, name);
    assert.ok(methods.scroll < limitations.scroll && limitations.scroll < block.scroll, 'three authored places reach distinct destinations');
    for (const label of ['Missing evidence', '^absent', 'Source']) {
      nav = await sources(page, input);
      const link = label === 'Source' ? nav.getByRole('link', { name: 'Source', exact: true }) : nav.locator('a.wikilink-degraded').filter({ hasText: label });
      assert.equal(await link.getAttribute('href'), '/notes/Source.md', 'bare and missing-place links open the file');
      if (label !== 'Source') assert.ok(await link.locator('.y-basedon__reason').isVisible(), 'fallback explanation is visible without hover');
      await activate(page, link, input);
      await page.waitForURL(`${base}/notes/Source.md`);
      assert.equal(await page.evaluate(() => scrollY), 0, 'file fallback lands at top');
      await page.goBack();
      await page.waitForURL('**/notes/Claim.md');
    }
    await page.goto(`${base}/notes/Single.md`);
    nav = await sources(page, input);
    assert.equal(await nav.locator('a').count(), 1, 'single location has no redundant parent');
    assert.equal(await nav.locator('a').innerText(), 'Source › Methods', 'heading supplies its own words');
    await page.screenshot({ path: join(evidence, `${name}.png`), fullPage: true });
    report.journeys.push({ name, methods, limitations, block });
    console.log(`PASS source-location journey ${name}: ${JSON.stringify({ methods, limitations, block })}`);
  } catch (error) {
    report.journeys.push({ name, failure: String(error) });
    await page.screenshot({ path: join(evidence, `failure-${name}.png`), fullPage: true }).catch(() => {});
    throw error;
  } finally {
    await context.close();
  }
}

try {
  let ready = false;
  for (let attempt = 0; attempt < 60; attempt++) {
    assert.equal(child.exitCode, null, `owned server exited: ${serverLog}`);
    if (serverLog.includes('msg="yomihon serving" addr=127.0.0.1:19869')) {
      const response = await fetch(base, { signal: AbortSignal.timeout(2000) });
      if (response.status === 200) { ready = true; break; }
    }
    await delay(250);
  }
  assert.ok(ready, `owned server was not ready: ${serverLog}`);
  browser = await chromium.launch({ channel: 'chrome', headless: true });
  report.browser = browser.version();
  for (const width of [1600, 800, 390]) await journey(width, true, 'pointer');
  await journey(390, true, 'touch');
  await journey(390, false, 'touch');
  await journey(800, true, 'keyboard');

  // Interleaved, warmed HTTP measurements over otherwise identical claim bodies.
  // This records complete-request cost, not a synthetic budget or a benchmark
  // proving performance on the owner's vault. CI hardware and load remain noise.
  const timings = { 'File-only': [], Claim: [] };
  for (let sample = 0; sample < 25; sample++) {
    for (const name of sample % 2 ? ['Claim', 'File-only'] : ['File-only', 'Claim']) {
      const start = performance.now();
      const response = await fetch(`${base}/notes/${name}.md`, { signal: AbortSignal.timeout(10000) });
      assert.equal(response.status, 200);
      await response.text();
      if (sample >= 5) timings[name].push(performance.now() - start);
    }
  }
  report.costs = Object.fromEntries(Object.entries(timings).map(([name, values]) => {
    values.sort((a, b) => a - b);
    return [name, { samples: values.length, medianMs: (values[9] + values[10]) / 2, p95Ms: values[18] }];
  }));
  report.costFixture = { sourceBytes: Buffer.byteLength(source), distinctRenderedSources: 2, locations: 6 };
  console.log(`MEASURE source-location request costs: ${JSON.stringify(report.costs)}`);
} finally {
  if (browser) await browser.close();
  if (child.exitCode === null && child.signalCode === null) {
    const stopped = once(child, 'exit');
    child.kill('SIGTERM');
    await stopped;
  }
  await writeFile(join(evidence, 'server.log'), serverLog);
  await writeFile(join(evidence, 'report.json'), JSON.stringify(report, null, 2));
  await rm(work, { recursive: true, force: true });
}
