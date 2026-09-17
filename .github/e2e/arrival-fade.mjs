// Behavior lock for the arrival fade: an opening page rises the last part of
// the way to full strength instead of cutting in, on every page surface, and
// reduced motion removes it.
//
// Four things have to hold together, and no one of them alone is the
// experience. The fade has to be declared on every page the reader can land
// on, which is why the surfaces below are enumerated by hand rather than
// sampled — the hook is the id every main landmark carries, and the class
// beside it is not the same on all of them. Its declared start has to be a
// legible page, asserted at a controlled animation time and read again from
// the keyframes themselves, because an opacity sampled whenever the probe
// happened to look cannot tell a fade that began at .7 from one that began at
// zero and has since climbed past it. The backwards fill has to be there, or
// the first painted frame is the page at full strength and the animation dims
// it from there — the same abrupt change with its direction reversed. And a
// reader who asks for less motion has to get a page that simply appears.
//
// The reduced-motion half is the house rule rather than a declaration of its
// own: every decorative duration inside the reading chrome collapses under
// that preference, so what this asserts is the collapsed duration on the main
// landmark, not the absence of a name.
//
// Env: YOMIHON_BASE, PAGE_PATH (a note with a heading anchor in its prose),
// and MUTATE.
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const PAGE = process.env.PAGE_PATH || '/notes/Notes/reading-fidelity.md';
const MUTATE = process.env.MUTATE || '';

const FADE = 'y-page-in';
const HIGHLIGHT = 'y-arrive';
const STATUS = '/status';
const L01 = '/notes/Writing/lessons/japanese/L01.md';

const SITES = [
  'every-surface-declares',
  'declared-start',
  'fill-and-tempo',
  'target-highlight-intact',
  'arrival-is-legible',
  'reduce-collapses-motion',
];

// The two browsing contexts differ only in the motion preference they report.
// A mutation names the one it edits, so a regression injected for the reduced
// reader cannot fire at a site that speaks for the other one.
const CONTEXTS = ['motion', 'reduce'];

// Every page template the reader can land on, with the shape its main landmark
// takes and something only that page renders. Reading the resolved animation
// on all of them is the point: a rule written against the reading column's
// class would leave settings untouched, and settings is one of the places this
// was noticed. The status and marker are here so a run where every route
// answered with the same page — the not-found one, say — cannot report twelve
// passes over one document.
const SURFACES = [
  { page: 'home', path: '/', status: 200, mainClass: 'y-main y-deskmain', marker: '#main-content [data-home-block="search"]' },
  { page: 'mode index', path: '/folders', status: 200, mainClass: 'y-main y-deskmain', marker: '#main-content[data-index="folders"]' },
  { page: 'folder', path: '/folders/Notes', status: 200, mainClass: 'y-main y-deskmain', marker: '#main-content .y-foldercount' },
  { page: 'note', path: '/notes/Notes/alpha.md', status: 200, mainClass: 'y-main', marker: '#main-content[data-preview-endpoint]' },
  { page: 'file', path: '/notes/Notes/plain.txt', status: 200, mainClass: 'y-main', marker: '#main-content .y-prose.y-source' },
  { page: 'search', path: '/search', status: 200, mainClass: 'y-main', marker: '#main-content .y-searchpage' },
  { page: 'health', path: '/health', status: 200, mainClass: 'y-main', marker: '#main-content .y-healthlede' },
  { page: 'report', path: '/reports/browser-boundary.html', status: 200, mainClass: 'y-main y-reportmain', marker: '#main-content .y-reporthead' },
  { page: 'syllabus', path: '/syllabus/Maps/study.md', status: 200, mainClass: 'y-main', marker: '#main-content .y-syl' },
  { page: 'preferences', path: '/preferences', status: 200, mainClass: 'y-prefs', marker: '#main-content .y-prefs__lede' },
  { page: 'not found', path: '/no-such-page', status: 404, mainClass: 'y-main', marker: '#main-content #notfound-title' },
  // The recovery page answers a refused status submission and nothing else, so
  // it is reached by submitting the note's own form with a field taken out.
  // The server refuses it before it reaches a file, which is why this probe
  // still belongs among the ones that only read.
  { page: 'status recovery', path: STATUS, status: 422, mainClass: 'y-main y-recoverymain', marker: '#main-content #recovery-title', viaRefusedPost: true },
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
  if (!SITES.includes(site)) throw new ProbeBroken(`BROKEN arrival-fade: unknown assertion site ${site}`);
  throw new LockFired(site, `FAIL arrival-fade: ${message}`);
};
const broken = (message) => { throw new ProbeBroken(`BROKEN arrival-fade: ${message}`); };
const notApplied = (message) => { throw new NotApplied(`NOT-APPLIED arrival-fade: ${message}`); };

// Times are compared as numbers. Chrome serialises a computed duration in
// seconds whatever unit it was written in, and the minified token keeps its
// own spelling, so two strings that mean the same span do not match.
const seconds = (text) => {
  const value = String(text).trim();
  if (value.endsWith('ms')) return Number.parseFloat(value) / 1000;
  if (value.endsWith('s')) return Number.parseFloat(value);
  return Number.NaN;
};
// Same reason for the easing curve: the token survives minification as
// cubic-bezier(.2,0,0,1) and the computed value is spelled out with zeroes and
// spaces. The four control numbers are what has to agree.
const curveNumbers = (text) => (String(text).match(/-?\d*\.?\d+(?:e-?\d+)?/g) || []).map(Number);
const sameCurve = (left, right) => {
  const a = curveNumbers(left);
  const b = curveNumbers(right);
  return a.length === 4 && b.length === 4 && a.every((value, index) => Math.abs(value - b[index]) < 1e-6);
};

// A mutation edits what the browser receives, never what the probe believes.
// Appending to the stylesheet lands the rule after everything the product's
// own sheet declares, so it stands in for a later rule that undid the one
// being locked.
const appendStylesheet = (rule) => async (context) => {
  let requests = 0;
  await context.route('**/static/app.css', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    await route.fulfill({ response, body: `${original}\n${rule}\n` });
  });
  return () => (requests >= 1 ? '' : 'the stylesheet was never requested, so the rule reached no page');
};

// A rewrite has to prove it landed. The stylesheet is fetched once per served
// copy and the needle occurs once in each, so the count that means "applied"
// is one per request — not one overall, which a second navigation would break,
// and not "at least one", which a needle the minifier reshaped would pass with
// zero.
const rewriteStylesheet = (needle, replacement, label) => async (context) => {
  let requests = 0;
  let matches = 0;
  await context.route('**/static/app.css', async (route) => {
    requests += 1;
    const response = await route.fetch();
    const original = await response.text();
    const found = original.split(needle).length - 1;
    matches += found;
    await route.fulfill({ response, body: original.replace(needle, replacement) });
  });
  return () => {
    if (requests < 1) return `${label} stylesheet was never requested, so the rewrite reached no page`;
    if (matches !== requests) return `${label} needle matched ${matches} times over ${requests} served copies, want one each`;
    return '';
  };
};

const MUTATIONS = {
  // The rule written against the reading column's class instead of the shared
  // id — the shape that silently leaves settings out.
  'narrow-the-hook-to-y-main': {
    target: 'every-surface-declares',
    context: 'motion',
    apply: rewriteStylesheet(`main#main-content{animation:${FADE}`, `.y-main{animation:${FADE}`, 'arrival-fade hook'),
  },
  // The fade starting from an invisible page, which is a brief blank document
  // rather than a softened arrival.
  'start-the-arrival-blank': {
    target: 'declared-start',
    context: 'motion',
    apply: rewriteStylesheet(`@keyframes ${FADE}{0%{opacity:.7}`, `@keyframes ${FADE}{0%{opacity:0}`, 'arrival-fade start'),
  },
  // Without the backwards fill the first painted frame is the page at full
  // strength and the animation dips it from there.
  'drop-the-backwards-fill': {
    target: 'fill-and-tempo',
    context: 'motion',
    apply: appendStylesheet('main#main-content{animation-fill-mode:none!important}'),
  },
  // The page fade taking over the name the fragment highlight already owns,
  // which leaves a reader who followed a link to a section with no signal
  // saying where they landed.
  'rename-the-target-highlight': {
    target: 'target-highlight-intact',
    context: 'motion',
    apply: rewriteStylesheet(`.y-prose :target{animation:${HIGHLIGHT}`, `.y-prose :target{animation:${FADE}`, 'fragment highlight'),
  },
  // An arriving page that is not there to be read or pressed.
  'hide-the-arrival': {
    target: 'arrival-is-legible',
    context: 'motion',
    apply: appendStylesheet('main#main-content{visibility:hidden!important}'),
  },
  // The duration handed back to the reader who asked for less motion.
  'restore-motion-under-reduce': {
    target: 'reduce-collapses-motion',
    context: 'reduce',
    apply: appendStylesheet('@media (prefers-reduced-motion:reduce){main#main-content{animation-duration:var(--dur-base)!important}}'),
  },
};

// The table has to name each page once. A row duplicated by a copy-paste would
// otherwise let the walk report its full count over a page it read twice, and
// a row lost to a bad edit would quietly shrink what "every surface" covers.
if (SURFACES.length !== 12) {
  console.error(`arrival-fade: the surface table names ${SURFACES.length} page templates, want the 12 that render a main landmark`);
  process.exit(2);
}
for (const field of ['path', 'marker', 'page']) {
  const distinct = new Set(SURFACES.map((surface) => surface[field]));
  if (distinct.size !== SURFACES.length) {
    console.error(`arrival-fade: the surface table holds ${distinct.size} distinct ${field} values over ${SURFACES.length} rows`);
    process.exit(2);
  }
}

for (const [name, mutation] of Object.entries(MUTATIONS)) {
  if (!SITES.includes(mutation.target)) {
    console.error(`arrival-fade: mutation ${name} aims at unknown site ${mutation.target}`);
    process.exit(2);
  }
  if (!CONTEXTS.includes(mutation.context)) {
    console.error(`arrival-fade: mutation ${name} names unknown browsing context ${mutation.context}`);
    process.exit(2);
  }
}
for (const site of SITES) {
  if (!Object.values(MUTATIONS).some((mutation) => mutation.target === site)) {
    console.error(`arrival-fade: assertion site ${site} has no mutation`);
    process.exit(2);
  }
}

if (MUTATE === 'list') {
  for (const name of Object.keys(MUTATIONS)) console.log(name);
  process.exit(0);
}
if (MUTATE && !Object.hasOwn(MUTATIONS, MUTATE)) {
  console.error(`arrival-fade: unknown MUTATE mode ${MUTATE}`);
  process.exit(2);
}

// What the landmark resolves to and how it is standing right now. Every value
// is taken at the moment of the read: getComputedStyle hands back a live
// declaration, so a property fetched later would describe the page as it had
// become, not as it was asked about.
const readLandmark = (page) => page.evaluate(() => {
  const element = document.getElementById('main-content');
  if (!element) return { present: false };
  const style = getComputedStyle(element);
  const root = getComputedStyle(document.documentElement);
  const rect = element.getBoundingClientRect();
  const probe = document.elementFromPoint(
    Math.min(Math.max(rect.left + rect.width / 2, 1), innerWidth - 1),
    Math.min(Math.max(rect.top + rect.height / 2, 1), innerHeight - 1),
  );
  return {
    present: true,
    mainClass: element.getAttribute('class'),
    name: style.animationName,
    duration: style.animationDuration,
    fill: style.animationFillMode,
    timing: style.animationTimingFunction,
    visibility: style.visibility,
    opacity: style.opacity,
    width: rect.width,
    height: rect.height,
    hit: probe === element || element.contains(probe),
    durBase: root.getPropertyValue('--dur-base'),
    easeStandard: root.getPropertyValue('--ease-standard'),
    path: location.pathname,
  };
});

// What the first painted frame carries, read without waiting for one. The
// keyframes come from the stylesheet the browser parsed, and the composed
// value comes from the animation restarted and held still at its own time
// zero — neither is a sample of whatever frame happened to be on screen, which
// is the reading that cannot tell a fade beginning at .7 from one beginning at
// zero and already past it. Restarting leaves the landmark mid-fade, so this
// runs after anything that asks how the page is standing.
const readDeclaredStart = (page) => page.evaluate(() => {
  const element = document.getElementById('main-content');
  if (!element) return { present: false };
  const name = getComputedStyle(element).animationName;

  let keyframes = null;
  for (const sheet of document.styleSheets) {
    let rules;
    try { rules = [...sheet.cssRules]; } catch { continue; }
    for (const rule of rules) {
      if (rule.type === CSSRule.KEYFRAMES_RULE && rule.name === name) {
        keyframes = [...rule.cssRules].map((frame) => ({ at: frame.keyText, opacity: frame.style.opacity }));
      }
    }
  }

  // Cancelling the name and restoring it starts the animation over; paused
  // first, so the restarted one stands still where it begins.
  element.style.animationPlayState = 'paused';
  element.style.animationName = 'none';
  void element.offsetWidth;
  element.style.animationName = '';
  void element.offsetWidth;
  const startOpacity = getComputedStyle(element).opacity;
  const restarted = element.getAnimations().map((animation) => ({ name: animation.animationName, time: animation.currentTime }));
  element.style.animationPlayState = '';
  element.style.animationName = '';

  return { present: true, name, keyframes, startOpacity, restarted };
});

// Waits out whatever the landmark is playing. The two frames come first
// because an entry animation asked for before the document has rendered once
// has not been created yet, and the wait would then be over before it began. A
// page restored from the back cache plays nothing and settles at once, which
// is what going back should feel like.
const settle = (page) => page.evaluate(async () => {
  const element = document.getElementById('main-content');
  if (!element) return;
  await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  await Promise.all(element.getAnimations().map((animation) => animation.finished.catch(() => {})));
  await new Promise((resolve) => requestAnimationFrame(resolve));
});

const browser = await chromium.launch({ channel: 'chrome', headless: true });
let proof = null;
try {
  const motion = await browser.newContext({ viewport: { width: 1280, height: 800 }, reducedMotion: 'no-preference' });
  const reduce = await browser.newContext({ viewport: { width: 1280, height: 800 }, reducedMotion: 'reduce' });
  const contexts = { motion, reduce };
  if (MUTATE) proof = await MUTATIONS[MUTATE].apply(contexts[MUTATIONS[MUTATE].context]);

  // A mutation is confirmed as soon as its own context has been served a page,
  // and before any assertion in that context can fire. Confirming at the end
  // instead would let an unrelated failure be reported as a catch.
  let confirmed = false;
  const confirm = (which) => {
    if (!proof || confirmed || MUTATIONS[MUTATE].context !== which) return;
    const issue = proof();
    if (issue) notApplied(`${MUTATE}: ${issue}`);
    confirmed = true;
  };

  const page = await motion.newPage();

  // Every surface the reader can land on declares the fade on its main
  // landmark.
  for (const surface of SURFACES) {
    let response;
    if (surface.viaRefusedPost) {
      const source = await page.goto(BASE + L01, { waitUntil: 'domcontentloaded' });
      if (!source || source.status() !== 200) broken(`${L01} answered ${source?.status() ?? 'nothing'}, want 200`);
      const form = page.locator('form[action="/status"]').first();
      if (await form.count() !== 1) broken('the lesson fixture exposes no native status form to refuse');
      await form.locator('input[name="to"]').evaluate((input) => input.remove());
      [response] = await Promise.all([
        page.waitForNavigation({ waitUntil: 'domcontentloaded' }),
        form.evaluate((element) => element.requestSubmit()),
      ]);
    } else {
      response = await page.goto(BASE + surface.path, { waitUntil: 'domcontentloaded' });
    }
    if (!response) broken(`${surface.page} at ${surface.path} produced no response`);
    if (response.status() !== surface.status) broken(`${surface.page} at ${surface.path} answered ${response.status()}, want ${surface.status}`);
    confirm('motion');

    const landmarks = await page.locator('main#main-content').count();
    if (landmarks !== 1) broken(`${surface.page} renders ${landmarks} main landmarks, want 1`);
    const markers = await page.locator(surface.marker).count();
    if (markers !== 1) broken(`${surface.page} matched ${markers} of its own marker ${surface.marker}, want 1 — the route did not answer with the page it names`);

    const landmark = await readLandmark(page);
    if (landmark.mainClass !== surface.mainClass) broken(`${surface.page} landmark carries class ${JSON.stringify(landmark.mainClass)}, want ${JSON.stringify(surface.mainClass)}`);
    if (landmark.name !== FADE) {
      fail('every-surface-declares', `${surface.page} at ${surface.path} resolves animation-name ${JSON.stringify(landmark.name)} on its main landmark, want ${FADE}`);
    }
  }

  // The narrow layout is the same declaration; a rule that only reached the
  // wide reading column would be a second way to miss a surface.
  await page.setViewportSize({ width: 390, height: 780 });
  const narrow = await page.goto(BASE + '/preferences', { waitUntil: 'domcontentloaded' });
  if (!narrow || narrow.status() !== 200) broken(`/preferences answered ${narrow?.status() ?? 'nothing'} at narrow width, want 200`);
  const narrowLandmark = await readLandmark(page);
  if (narrowLandmark.name !== FADE) {
    fail('every-surface-declares', `settings at 390px resolves animation-name ${JSON.stringify(narrowLandmark.name)}, want ${FADE}`);
  }
  await page.setViewportSize({ width: 1280, height: 800 });

  // What the first painted frame carries, asserted twice over and neither time
  // from a sampled frame.
  const note = await page.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!note || note.status() !== 200) broken(`${PAGE} answered ${note?.status() ?? 'nothing'}, want 200`);
  const reading = await readLandmark(page);
  if (!reading.present) broken(`${PAGE} renders no main landmark`);
  if (reading.name !== FADE) fail('every-surface-declares', `${PAGE} resolves animation-name ${JSON.stringify(reading.name)}, want ${FADE}`);

  if (reading.fill !== 'backwards') {
    fail('fill-and-tempo', `the landmark resolves animation-fill-mode ${JSON.stringify(reading.fill)}, want backwards so the first painted frame is the animation's own start`);
  }
  const wanted = seconds(reading.durBase);
  if (!Number.isFinite(wanted) || wanted <= 0) broken(`--dur-base reads ${JSON.stringify(reading.durBase)}, which is not a duration`);
  if (Math.abs(seconds(reading.duration) - wanted) > 1e-6) {
    fail('fill-and-tempo', `the landmark runs for ${JSON.stringify(reading.duration)}, want the ${JSON.stringify(reading.durBase)} of --dur-base`);
  }
  if (!sameCurve(reading.timing, reading.easeStandard)) {
    fail('fill-and-tempo', `the landmark eases on ${JSON.stringify(reading.timing)}, want the ${JSON.stringify(reading.easeStandard)} of --ease-standard`);
  }

  const declared = await readDeclaredStart(page);
  if (!declared.present) broken(`${PAGE} renders no main landmark to restart`);
  if (!Array.isArray(declared.keyframes) || declared.keyframes.length !== 2) {
    fail('declared-start', `the stylesheet the browser parsed holds ${JSON.stringify(declared.keyframes)} for ${FADE}, want a start and an end`);
  }
  const start = declared.keyframes.find((frame) => frame.at === '0%');
  const end = declared.keyframes.find((frame) => frame.at === '100%');
  if (!start || !end) fail('declared-start', `${FADE} is written at ${JSON.stringify(declared.keyframes.map((frame) => frame.at))}, want 0% and 100%`);
  if (Number.parseFloat(start.opacity) !== 0.7 || Number.parseFloat(end.opacity) !== 1) {
    fail('declared-start', `${FADE} is declared from opacity ${JSON.stringify(start.opacity)} to ${JSON.stringify(end.opacity)}, want a legible .7 rising to 1`);
  }
  if (declared.restarted.length !== 1 || declared.restarted[0].name !== FADE || declared.restarted[0].time !== 0) {
    broken(`the landmark held ${JSON.stringify(declared.restarted)} after being restarted paused, want one ${FADE} standing at 0`);
  }
  if (Number.parseFloat(declared.startOpacity) !== 0.7) {
    fail('declared-start', `at its own time zero the landmark composes to opacity ${JSON.stringify(declared.startOpacity)}, want .7`);
  }

  // A reader who followed a link to a section gets both: the page rising, and
  // the section's own echo saying where they landed.
  const anchor = page.locator('#main-content .y-prose :is(h1,h2,h3,h4,h5,h6)[id]').first();
  if (await anchor.count() !== 1) broken(`${PAGE} has no heading anchor in its prose to land on`);
  const anchorID = await anchor.getAttribute('id');
  await page.evaluate((id) => { location.hash = `#${id}`; }, anchorID);
  const landed = await page.evaluate((id) => {
    const element = document.getElementById(id);
    return {
      target: element.matches(':target'),
      name: getComputedStyle(element).animationName,
      landmark: getComputedStyle(document.getElementById('main-content')).animationName,
    };
  }, anchorID);
  if (!landed.target) broken(`the heading ${anchorID} did not become the document's target`);
  if (landed.name !== HIGHLIGHT || landed.landmark !== FADE) {
    fail('target-highlight-intact', `the landed heading plays ${JSON.stringify(landed.name)} under a landmark playing ${JSON.stringify(landed.landmark)}, want ${HIGHLIGHT} under ${FADE}`);
  }

  // Three real arrivals: a link out of the reading page to settings, the back
  // that returns from it, and a link inside the arriving page's own content.
  // Each one has to end at a page that is there to be read.
  const legible = async (where) => {
    await settle(page);
    const landmark = await readLandmark(page);
    if (!landmark.present) broken(`${where} renders no main landmark`);
    if (landmark.visibility !== 'visible' || !landmark.hit || landmark.width <= 0 || landmark.height <= 0 || Number.parseFloat(landmark.opacity) !== 1) {
      fail('arrival-is-legible', `${where} settled at ${JSON.stringify({ visibility: landmark.visibility, opacity: landmark.opacity, hit: landmark.hit, width: landmark.width, height: landmark.height })}, want a painted landmark at full strength`);
    }
  };

  await Promise.all([page.waitForNavigation({ waitUntil: 'domcontentloaded' }), page.locator('a.y-prefslink').first().click()]);
  if (new URL(page.url()).pathname !== '/preferences') broken(`the settings link led to ${page.url()}, want /preferences`);
  await legible('settings, arrived from the reading page');

  await page.goBack({ waitUntil: 'domcontentloaded' });
  if (new URL(page.url()).pathname !== new URL(BASE + PAGE).pathname) broken(`going back led to ${page.url()}, want ${PAGE}`);
  await legible('the reading page, arrived by going back');

  // The click that produces this navigation is itself the proof that the page
  // which just arrived accepts a press.
  await Promise.all([page.waitForNavigation({ waitUntil: 'domcontentloaded' }), page.locator('#main-content a.y-crumbs__link').first().click()]);
  if (new URL(page.url()).pathname !== '/folders/Notes') broken(`the breadcrumb led to ${page.url()}, want /folders/Notes`);
  await legible('the folder, arrived from a link inside the page that had just arrived');

  // The reader who asks for less motion gets a page that simply appears. The
  // duration is collapsed by the house rule, so what is asserted is how far
  // below the token it has fallen, not a serialisation of the collapsed value.
  const quiet = await reduce.newPage();
  const quietResponse = await quiet.goto(BASE + PAGE, { waitUntil: 'domcontentloaded' });
  if (!quietResponse || quietResponse.status() !== 200) broken(`${PAGE} answered ${quietResponse?.status() ?? 'nothing'} for the reduced-motion reader, want 200`);
  confirm('reduce');
  const quietLandmark = await readLandmark(quiet);
  if (!quietLandmark.present) broken(`${PAGE} renders no main landmark for the reduced-motion reader`);
  const quietSeconds = seconds(quietLandmark.duration);
  const quietWanted = seconds(quietLandmark.durBase);
  if (!Number.isFinite(quietSeconds) || !Number.isFinite(quietWanted) || quietWanted <= 0) {
    broken(`the reduced-motion landmark reads duration ${JSON.stringify(quietLandmark.duration)} against --dur-base ${JSON.stringify(quietLandmark.durBase)}`);
  }
  if (quietSeconds > quietWanted / 1000) {
    fail('reduce-collapses-motion', `the reduced-motion landmark still runs for ${JSON.stringify(quietLandmark.duration)}, want a duration collapsed far below the ${JSON.stringify(quietLandmark.durBase)} of --dur-base`);
  }

  if (proof && !confirmed) broken(`${MUTATE} was never confirmed, so this run proves nothing about it`);

  console.log(`PASS arrival-fade: ${SURFACES.length} page surfaces declare ${FADE} from a legible .7, it is gated for the reduced-motion reader, and three real arrivals land readable`);
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
