// A held application stylesheet distinguishes polling from an async false
// result. Both cases use the real Home arrival and the same served duration.
import { readFile } from 'node:fs/promises';
import { chromium } from 'playwright-core';

const BASE = process.env.YOMIHON_BASE || 'http://127.0.0.1:9610';
const MUTATE = process.env.MUTATE || '';
const MODE = 'restore-async-predicate';
if (MUTATE === 'list') {
  console.log(MODE);
  process.exit(0);
}

class SetupFailure extends Error {}
const requireSetup = (condition, message) => {
  if (!condition) throw new SetupFailure(message);
};

// Keep the returned state separate from the subsequent lifecycle drain: the
// stylesheet becoming available later cannot repair an early helper return.
const snapshot = () => {
  const appCSS = [...document.styleSheets].some((sheet) => {
    if (!sheet.href) return false;
    const url = new URL(sheet.href);
    return url.origin === location.origin && url.pathname === '/static/app.css';
  });
  return {
    appCSS,
    animations: document.getAnimations()
      .filter((animation) => animation.animationName === 'y-come-forward')
      .map((animation) => ({ state: animation.playState, pending: animation.pending })),
  };
};

let browser;
let applied = false;
try {
  requireSetup(!MUTATE || MUTATE === MODE, `unknown mutation ${MUTATE}`);
  let source = await readFile(new URL('./support/arrival.mjs', import.meta.url), 'utf8');
  if (MUTATE) {
    const needle = 'function arrivalReady(';
    requireSetup(source.split(needle).length - 1 === 1 && !source.includes(`async ${needle}`), 'mutation declaration is missing or ambiguous');
    source = source.replace(needle, `async ${needle}`);
    applied = true;
  }
  const { arrived } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  requireSetup(typeof arrived === 'function', 'shared arrival export is missing');
  browser = await chromium.launch({ channel: 'chrome', headless: true });

  const runCase = async (delayed) => {
    const context = await browser.newContext({ reducedMotion: 'no-preference' });
    const page = await context.newPage();
    let release;
    const held = new Promise((resolve) => { release = resolve; });
    let delivered;
    const delivery = new Promise((resolve) => { delivered = resolve; });
    let prepared;
    const preparation = new Promise((resolve) => { prepared = resolve; });
    let routeURL;
    let routeError;
    let timer;
    let setupTimer;
    try {
      await context.route((url) => url.origin === new URL(BASE).origin && url.pathname === '/static/app.css', async (route) => {
        try {
          requireSetup(!routeURL, 'application stylesheet requested more than once');
          routeURL = route.request().url();
          const response = await route.fetch();
          requireSetup(response.status() === 200, `stylesheet HTTP ${response.status()}`);
          const original = await response.text();
          const needle = '--dur-arrival: 320ms;';
          requireSetup(original.split(needle).length - 1 === 1, 'arrival duration declaration is missing or ambiguous');
          const body = original.replace(needle, '--dur-arrival: 2s;');
          prepared();
          await held;
          await route.fulfill({ response, body });
          console.log(`receipt: arrival-readiness CSS delivered ${routeURL}, duration edits=1, HTTP=200`);
        } catch (error) {
          routeError = error;
          prepared();
          await route.abort().catch(() => {});
        } finally {
          delivered();
        }
      });
      const response = await page.goto(new URL('/', BASE).href, { waitUntil: 'commit' });
      requireSetup(response?.status() === 200, 'Home document did not commit HTTP 200');
      await page.locator('main#_y-main .y-home').waitFor({ state: 'attached', timeout: 3000 });
      await page.waitForFunction(() => [...document.querySelectorAll('link[rel="stylesheet"]')].some((link) => new URL(link.href).pathname === '/static/app.css'), null, { timeout: 3000 });
      const linkedURL = await page.evaluate(() => [...document.querySelectorAll('link[rel="stylesheet"]')].find((link) => new URL(link.href).pathname === '/static/app.css').href);
      await Promise.race([
        preparation,
        new Promise((_, reject) => {
          setupTimer = setTimeout(() => reject(new SetupFailure('actual application stylesheet route was not prepared')), 3000);
        }),
      ]);
      clearTimeout(setupTimer);
      if (routeError) throw routeError;
      requireSetup(routeURL === linkedURL && new URL(page.url()).pathname === '/', 'held route does not match the actual Home stylesheet link');
      requireSetup(!(await page.evaluate(snapshot)).appCSS, 'stylesheet was already loaded before the held stimulus');

      // Release is scheduled independently of helper settlement in every run.
      timer = setTimeout(release, 250);
      const lifecycle = (async () => {
        await delivery;
        if (routeError) throw routeError;
        await page.waitForFunction(() => document.getAnimations().some((animation) => animation.animationName === 'y-come-forward'), null, { timeout: 3000 });
        return page.evaluate(async () => {
          const animations = document.getAnimations().filter((animation) => animation.animationName === 'y-come-forward');
          const durations = animations.map((animation) => animation.effect.getTiming().duration);
          const outcomes = await Promise.all(animations.map((animation) => animation.finished.then(() => 'finished', () => 'canceled')));
          return { count: animations.length, durations, outcomes };
        });
      })();
      // Observe rejection immediately while the helper is still pending.
      const observedLifecycle = lifecycle.then((value) => ({ value }), (error) => ({ error }));
      if (!delayed) {
        await delivery;
        if (routeError) throw routeError;
        await page.waitForFunction(() => document.getAnimations().some((animation) => animation.animationName === 'y-come-forward' && animation.playState === 'running'), null, { timeout: 3000 });
      }
      const invocation = await page.evaluate(snapshot);
      requireSetup(delayed ? !invocation.appCSS : invocation.appCSS && invocation.animations.some((animation) => animation.state === 'running'), 'required stylesheet/arrival stimulus was lost before invocation');
      console.log(`invocation-hit: arrival-readiness ${delayed ? 'delayed-css' : 'CSS-ready-running'} ${JSON.stringify(invocation)}`);
      const handle = await arrived(page);
      const returned = await page.evaluate(snapshot);
      const value = await handle.jsonValue();
      await handle.dispose();
      const receipt = await observedLifecycle;
      if (receipt.error) throw receipt.error;
      requireSetup(receipt.value.count > 0 && receipt.value.durations.every((duration) => duration === 2000) && receipt.value.outcomes.every((outcome) => outcome === 'finished'), 'actual two-second Home arrival did not finish');
      console.log(`receipt: arrival-readiness ${delayed ? 'delayed-css' : 'CSS-ready-running'} return=${JSON.stringify({ ...returned, value })} lifecycle=${JSON.stringify(receipt.value)}`);
      return returned;
    } finally {
      clearTimeout(timer);
      clearTimeout(setupTimer);
      release();
      await context.close();
    }
  };

  const delayed = await runCase(true);
  const control = await runCase(false);
  requireSetup(control.appCSS && control.animations.every((animation) => !animation.pending && !['running', 'paused'].includes(animation.state)), 'CSS-ready running positive control did not wait for the real arrival');
  console.log('PASS arrival-readiness CSS-ready-running positive control');
  if (!delayed.appCSS) {
    console.error('caught: arrival-readiness returned before app stylesheet loaded');
    if (MUTATE && applied) console.log(`MUTATE-RESULT: caught ${MODE}`);
    process.exitCode = 1;
  } else if (delayed.animations.some((animation) => animation.pending || ['running', 'paused'].includes(animation.state))) {
    console.error('FAIL arrival-readiness returned before arrival finished');
    process.exitCode = MUTATE ? 2 : 1;
  } else {
    console.log('PASS arrival-readiness delayed-css and actual arrival finished');
  }
} catch (error) {
  console.error(`BROKEN arrival-readiness: ${error.stack || error}`);
  if (MUTATE) console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
  process.exitCode = 2;
} finally {
  await browser?.close();
}
