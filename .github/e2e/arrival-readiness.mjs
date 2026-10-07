// A held application stylesheet distinguishes polling from an async false
// result. Both cases use the real Home arrival and the same served duration.
import { readFile } from 'node:fs/promises';
import { performance } from 'node:perf_hooks';
import { chromium, errors } from 'playwright-core';

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
  // Relative package resolution has no base inside a data URL. Rewrite the
  // same unique import in control and mutant before either is loaded.
  const importNeedle = "from 'playwright-core'";
  requireSetup(source.split(importNeedle).length - 1 === 1, 'helper Playwright import is missing or ambiguous');
  source = source.replace(importNeedle, `from ${JSON.stringify(import.meta.resolve('playwright-core'))}`);
  const originalSource = source;
  const { arrived: controlArrived } = await import(`data:text/javascript;base64,${Buffer.from(originalSource).toString('base64')}`);
  if (MUTATE) {
    const needle = 'function arrivalReady(';
    requireSetup(source.split(needle).length - 1 === 1 && !source.includes(`async ${needle}`), 'mutation declaration is missing or ambiguous');
    source = source.replace(needle, `async ${needle}`);
    applied = true;
  }
  const { arrived } = await import(`data:text/javascript;base64,${Buffer.from(source).toString('base64')}`);
  requireSetup(typeof arrived === 'function' && typeof controlArrived === 'function', 'shared arrival export is missing');

  // These adapters exercise the actual original export at protocol boundaries;
  // they do not manufacture Home/CSS/animation behavior for the browser oracle.
  const deferred = () => {
    let resolve;
    const promise = new Promise((done) => { resolve = done; });
    return { promise, resolve };
  };
  const protocolControls = async () => {
    const receipt = async (promise, label) => {
      let timer;
      try {
        await Promise.race([promise, new Promise((_, reject) => {
          timer = setTimeout(() => reject(new SetupFailure(`${label} receipt was not observed`)), 1000);
        })]);
      } finally {
        clearTimeout(timer);
      }
    };
    const expectTimeout = async (page, label) => {
      const started = performance.now();
      let error;
      let timer;
      try {
        await Promise.race([controlArrived(page, 30), new Promise((_, reject) => {
          timer = setTimeout(() => reject(new SetupFailure(`${label} exceeded the control watchdog`)), 1000);
        })]);
      } catch (caught) {
        error = caught;
      } finally {
        clearTimeout(timer);
      }
      requireSetup(error instanceof errors.TimeoutError && error.constructor === errors.TimeoutError,
        `${label} did not preserve Playwright TimeoutError identity`);
      requireSetup(performance.now() - started < 1000, `${label} cleanup extended the bounded timeout`);
      console.log(`receipt: arrival-readiness protocol ${label} TimeoutError`);
    };
    const expectError = async (page, expected, label) => {
      let error;
      let timer;
      try {
        await Promise.race([controlArrived(page), new Promise((_, reject) => {
          timer = setTimeout(() => reject(new SetupFailure(`${label} exceeded the control watchdog`)), 1000);
        })]);
      } catch (caught) {
        error = caught;
      } finally {
        clearTimeout(timer);
      }
      requireSetup(error === expected, `${label} changed ordinary protocol error identity`);
    };

    const creation = deferred();
    const lateDisposed = deferred();
    let lateDisposals = 0;
    await expectTimeout({ evaluateHandle: () => creation.promise }, 'stalled-handle-creation');
    creation.resolve({
      jsonValue: () => { throw new SetupFailure('late handle was read after the deadline'); },
      dispose: async () => { lateDisposals++; lateDisposed.resolve(); },
    });
    await receipt(lateDisposed.promise, 'late-handle-disposal');
    requireSetup(lateDisposals === 1, 'late protocol handle was not disposed exactly once');

    const valueRead = deferred();
    const valueDisposed = deferred();
    let valueDisposals = 0;
    await expectTimeout({ evaluateHandle: async () => ({
      jsonValue: () => valueRead.promise,
      dispose: async () => { valueDisposals++; valueDisposed.resolve(); },
    }) }, 'stalled-json-value');
    await receipt(valueDisposed.promise, 'timed-out-value-disposal');
    valueRead.resolve(true);
    requireSetup(valueDisposals === 1, 'timed-out value handle was not disposed exactly once');

    const disposal = deferred();
    let disposalCalls = 0;
    await expectTimeout({ evaluateHandle: async () => ({
      jsonValue: async () => false,
      dispose: () => { disposalCalls++; return disposal.promise; },
    }) }, 'stalled-false-handle-disposal');
    disposal.resolve();
    requireSetup(disposalCalls === 1, 'false handle disposal was retried or skipped');

    let calls = 0;
    let falseDisposals = 0;
    let transferredDisposals = 0;
    const transferred = {
      jsonValue: async () => true,
      dispose: async () => { transferredDisposals++; },
    };
    const handle = await controlArrived({ evaluateHandle: async () => {
      calls++;
      if (calls === 1) return {
        jsonValue: async () => false,
        dispose: async () => { falseDisposals++; },
      };
      requireSetup(calls === 2 && falseDisposals === 1, 'retry preceded false-handle disposal');
      return transferred;
    } });
    requireSetup(handle === transferred && transferredDisposals === 0, 'successful JSHandle ownership did not transfer to caller');
    await handle.dispose();
    requireSetup(transferredDisposals === 1, 'caller could not dispose transferred JSHandle');
    const override = await controlArrived({ evaluateHandle: async () => transferred }, 5000);
    requireSetup(override === transferred && transferredDisposals === 1, '5000ms override changed successful ownership');
    await override.dispose();

    const protocolError = new Error('controlled ordinary protocol failure');
    await expectError({ evaluateHandle: async () => { throw protocolError; } }, protocolError, 'handle-creation-error');
    let errorDisposals = 0;
    const errorDisposed = deferred();
    await expectError({ evaluateHandle: async () => ({
      jsonValue: async () => { throw protocolError; },
      dispose: async () => { errorDisposals++; errorDisposed.resolve(); },
    }) }, protocolError, 'json-value-error');
    await receipt(errorDisposed.promise, 'error-handle-disposal');
    requireSetup(errorDisposals === 1, 'error handle was not disposed exactly once');
    await expectError({ evaluateHandle: async () => ({
      jsonValue: async () => false,
      dispose: async () => { throw protocolError; },
    }) }, protocolError, 'false-disposal-error');
    console.log('PASS arrival-readiness actual original helper protocol deadline, disposal and error controls');
  };
  await protocolControls();
  browser = await chromium.launch({ channel: 'chrome', headless: true });

  const runCase = async (delayed, helper = arrived) => {
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
      const handle = await helper(page);
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
  // The correct helper owns the independent already-running control, so a
  // selected async fault cannot turn its own regression into broken setup.
  const control = await runCase(false, controlArrived);
  requireSetup(control.appCSS && control.animations.every((animation) => !animation.pending && !['running', 'paused'].includes(animation.state)), 'CSS-ready running positive control did not wait for the real arrival');
  console.log('PASS arrival-readiness CSS-ready-running positive control');

  const runNoJS = async (initiallyReady, helper) => {
    const context = await browser.newContext({ javaScriptEnabled: false, reducedMotion: 'no-preference' });
    let page;
    const receiptKey = '__yArrivalFirstReadReceipt';
    let release;
    const held = new Promise((resolve) => { release = resolve; });
    let prepared;
    const preparation = new Promise((resolve) => { prepared = resolve; });
    let delivered;
    const delivery = new Promise((resolve) => { delivered = resolve; });
    let setupTimer;
    let routeURL;
    let routeError;
    let routeCount = 0;
    const routeTasks = [];
    let observerInstalled = false;
    let observedHelper;
    let observedCapture;
    let observedLifecycle;
    let observedReady;
    const handles = new Set();
    const stateReceipt = async (stage) => {
      try {
        const state = await page.evaluate(() => ({
          url: location.href,
          readyState: document.readyState,
          links: [...document.querySelectorAll('link[rel="stylesheet"]')].map((link) => ({
            url: link.href,
            href: link.getAttribute('href'),
            media: link.media,
            disabled: link.disabled,
            sheet: Boolean(link.sheet),
          })),
          sheets: [...document.styleSheets].map((sheet) => ({ href: sheet.href, disabled: sheet.disabled })),
          animations: document.getAnimations().filter((animation) => animation.animationName === 'y-come-forward').map((animation) => ({
            target: animation.effect?.target && {
              tag: animation.effect.target.tagName,
              id: animation.effect.target.id,
              className: animation.effect.target.className,
            },
            pending: animation.pending,
            playState: animation.playState,
            currentTime: animation.currentTime,
            timing: animation.effect?.getTiming(),
          })),
        }));
        console.log(`receipt: arrival-readiness no-JS ${initiallyReady ? 'initially-ready control' : 'false-to-ready'} ${stage} route=${routeURL} ${JSON.stringify(state)}`);
      } catch (error) {
        console.error(`diagnostic-unavailable: arrival-readiness no-JS ${stage}: ${error.stack || error}`);
      }
    };
    const numericWait = async (predicate, argument = null) => {
      // Independent oracle: direct protocol evaluation driven by the host,
      // never the helper or an in-page RAF/numeric-timer polling loop.
      const deadline = performance.now() + 3000;
      const timeoutError = new errors.TimeoutError('independent no-JS readiness timed out after 3000ms');
      let expired = false;
      let timer;
      const polling = async () => {
        for (;;) {
          if (expired || performance.now() >= deadline) throw timeoutError;
          const value = await page.evaluate(predicate, argument);
          if (expired || performance.now() >= deadline) throw timeoutError;
          if (value) return value;
          await new Promise((resolve) => setTimeout(resolve, Math.min(50, deadline - performance.now())));
        }
      };
      try {
        return await Promise.race([polling(), new Promise((_, reject) => {
          timer = setTimeout(() => reject(timeoutError), Math.max(0, Math.ceil(deadline - performance.now())));
        })]);
      } finally {
        expired = true;
        clearTimeout(timer);
      }
    };
    const restoreObserver = async () => {
      await page.evaluate((key) => {
        const receipt = document[key];
        if (!receipt || Object.getOwnPropertyDescriptor(document, 'styleSheets')?.get !== receipt.observer) {
          throw new Error('first-read observer ownership was lost');
        }
        if (!delete document.styleSheets || !delete document[key]
          || Object.hasOwn(document, 'styleSheets') || Object.hasOwn(document, key)) {
          throw new Error('first-read observer could not be removed');
        }
        let prototype = Object.getPrototypeOf(document);
        while (prototype && !Object.getOwnPropertyDescriptor(prototype, 'styleSheets')) prototype = Object.getPrototypeOf(prototype);
        if (Object.getOwnPropertyDescriptor(prototype, 'styleSheets')?.get !== receipt.nativeGetter) {
          throw new Error('native stylesheet getter was not restored');
        }
      }, receiptKey);
      observerInstalled = false;
    };
    try {
      page = await context.newPage();
      await context.route((url) => url.origin === new URL(BASE).origin && url.pathname === '/static/app.css', (route) => {
        const task = (async () => {
          try {
            routeCount++;
            requireSetup(routeCount === 1, 'no-JS stylesheet requested more than once');
            routeURL = route.request().url();
            const response = await route.fetch();
            requireSetup(response.status() === 200, `no-JS stylesheet HTTP ${response.status()}`);
            const original = await response.text();
            const needle = '--dur-arrival: 320ms;';
            requireSetup(original.split(needle).length - 1 === 1, 'no-JS arrival duration declaration is missing or ambiguous');
            const body = original.replace(needle, '--dur-arrival: 2s;');
            prepared();
            await held;
            await route.fulfill({ response, body });
            console.log(`receipt: arrival-readiness no-JS CSS delivered ${routeURL}, duration edits=1, HTTP=200`);
          } catch (error) {
            routeError = error;
            prepared();
            await route.abort().catch(() => {});
          } finally {
            delivered();
          }
        })();
        routeTasks.push(task);
        return task;
      });
      const response = await page.goto(new URL('/', BASE).href, { waitUntil: 'commit' });
      requireSetup(response?.status() === 200, 'no-JS Home did not commit HTTP 200');
      await numericWait(() => Boolean(document.querySelector('main#_y-main .y-home')));
      await numericWait(() => [...document.querySelectorAll('link[rel="stylesheet"]')].some((link) => new URL(link.href).pathname === '/static/app.css'));
      const linkedURL = await page.evaluate(() => [...document.querySelectorAll('link[rel="stylesheet"]')].find((link) => new URL(link.href).pathname === '/static/app.css').href);
      await Promise.race([preparation, new Promise((_, reject) => {
        setupTimer = setTimeout(() => reject(new SetupFailure('no-JS actual stylesheet route was not prepared')), 3000);
      })]);
      clearTimeout(setupTimer);
      if (routeError) throw routeError;
      requireSetup(routeURL === linkedURL && new URL(page.url()).pathname === '/', 'no-JS route differs from actual Home stylesheet link');
      requireSetup(!(await page.evaluate(snapshot)).appCSS, 'no-JS held stylesheet already loaded');

      const invoke = () => {
        const started = performance.now();
        // Both outcomes are observed now; the return snapshot precedes any
        // later lifecycle drain and cannot be repaired by eventual readiness.
        observedHelper = helper(page).then(async (handle) => {
          handles.add(handle);
          try {
            if (observerInstalled) {
              requireSetup(await page.evaluate((key) => Boolean(document[key]?.first), receiptKey),
                'helper returned without the actual first stylesheet read');
            }
            const returned = await page.evaluate(snapshot);
            const value = await handle.jsonValue();
            return { returned, value };
          } finally {
            await handle.dispose();
            handles.delete(handle);
          }
        }, (error) => ({ error, helperError: true })).catch((error) => ({ error }));
        return started;
      };
      let started;
      if (!initiallyReady) {
        await page.evaluate((key) => {
          if (Object.hasOwn(document, 'styleSheets') || key in document || !Object.isExtensible(document)) {
            throw new Error('native stylesheet observer cannot be installed');
          }
          let prototype = Object.getPrototypeOf(document);
          while (prototype && !Object.getOwnPropertyDescriptor(prototype, 'styleSheets')) prototype = Object.getPrototypeOf(prototype);
          const nativeGetter = Object.getOwnPropertyDescriptor(prototype, 'styleSheets')?.get;
          if (typeof nativeGetter !== 'function' || !Function.prototype.toString.call(nativeGetter).includes('[native code]')) {
            throw new Error('native Document stylesheet getter is unavailable');
          }
          const receipt = { nativeGetter, reads: 0, first: null };
          receipt.observer = function () {
            if (this !== document) throw new Error('unexpected stylesheet observer receiver');
            const sheets = nativeGetter.call(this);
            receipt.reads++;
            if (receipt.reads === 1) {
              receipt.first = { appCSS: [...sheets].some((sheet) => {
                if (!sheet.href) return false;
                const url = new URL(sheet.href);
                return url.origin === location.origin && url.pathname === '/static/app.css';
              }) };
            }
            return sheets;
          };
          Object.defineProperty(document, key, { value: receipt, configurable: true });
          try {
            Object.defineProperty(document, 'styleSheets', { get: receipt.observer, configurable: true });
          } catch (error) {
            delete document[key];
            throw error;
          }
          if (Object.getOwnPropertyDescriptor(document, 'styleSheets')?.get !== receipt.observer) {
            delete document.styleSheets;
            delete document[key];
            throw new Error('stylesheet observer installation failed');
          }
        }, receiptKey);
        observerInstalled = true;
        started = invoke();
        // Until this receipt, this is the only other observer. It never
        // reads styleSheets; disabled application scripting cannot compete.
        const first = await numericWait((key) => {
          const receipt = document[key];
          return receipt?.first && { first: receipt.first, reads: receipt.reads,
            owned: Object.getOwnPropertyDescriptor(document, 'styleSheets')?.get === receipt.observer };
        }, receiptKey);
        requireSetup(first.owned && first.reads >= 1 && first.first.appCSS === false, 'actual no-JS first helper read was not absent appCSS');
        console.log('invocation-hit: arrival-readiness no-JS first-helper-read appCSS=false');
        await restoreObserver();
      }
      // Node release depends on the actual predicate read, never its result.
      release();
      observedCapture = (async () => {
        await delivery;
        if (routeError) throw routeError;
        await stateReceipt('after-delivery');
        await numericWait(() => document.getAnimations().some((animation) => animation.animationName === 'y-come-forward'));
        await stateReceipt('after-capture');
      })().then(() => ({}), (error) => ({ error }));
      observedLifecycle = (async () => {
        const capture = await observedCapture;
        if (capture.error) throw capture.error;
        return page.evaluate(async () => {
          const animations = document.getAnimations().filter((animation) => animation.animationName === 'y-come-forward');
          const durations = animations.map((animation) => animation.effect.getTiming().duration);
          const outcomes = await Promise.all(animations.map((animation) => animation.finished.then(() => 'finished', () => 'canceled')));
          return { count: animations.length, durations, outcomes };
        });
      })().then((value) => {
        console.log(`receipt: arrival-readiness no-JS ${initiallyReady ? 'initially-ready control' : 'false-to-ready'} lifecycle-resolved ${JSON.stringify(value)}`);
        return { value };
      }, (error) => ({ error }));
      observedReady = (async () => {
        const capture = await observedCapture;
        if (capture.error) throw capture.error;
        await stateReceipt('before-readiness');
        try {
          await numericWait(() => {
            const appCSS = [...document.styleSheets].some((sheet) => {
              if (!sheet.href) return false;
              const url = new URL(sheet.href);
              return url.origin === location.origin && url.pathname === '/static/app.css';
            });
            return appCSS && !document.getAnimations().some((animation) => animation.animationName === 'y-come-forward'
              && (animation.pending || ['running', 'paused'].includes(animation.playState)));
          });
        } catch (error) {
          if (error instanceof errors.TimeoutError) await stateReceipt('readiness-timeout');
          throw error;
        }
        return performance.now();
      })().then((value) => ({ value }), (error) => ({ error }));
      const ready = await observedReady;
      if (ready.error) throw ready.error;
      const lifecycle = await observedLifecycle;
      if (lifecycle.error) throw lifecycle.error;
      requireSetup(lifecycle.value.count > 0 && lifecycle.value.durations.every((duration) => duration === 2000)
        && lifecycle.value.outcomes.every((outcome) => outcome === 'finished'), 'no-JS actual two-second arrival did not finish');
      if (initiallyReady) started = invoke();
      else requireSetup(ready.value - started < 3000, 'independent no-JS readiness was later than the unchanged helper deadline');
      const result = await observedHelper;
      if (routeError) throw routeError;
      requireSetup(routeCount === 1, 'no-JS actual stylesheet route was not unique');
      console.log(`receipt: arrival-readiness no-JS ${initiallyReady ? 'initially-ready control' : 'false-to-ready'} ready-ms=${initiallyReady ? 'before-invocation' : ready.value - started} lifecycle=${JSON.stringify(lifecycle.value)} return=${JSON.stringify(result)}`);
      if (result.error) {
        if (!initiallyReady && result.helperError && result.error instanceof errors.TimeoutError) return { timeout: true };
        throw result.error;
      }
      return { valid: result.value === true && result.returned.appCSS
        && result.returned.animations.every((animation) => !animation.pending && !['running', 'paused'].includes(animation.state)) };
    } finally {
      clearTimeout(setupTimer);
      release();
      try {
        if (observerInstalled) await restoreObserver();
      } finally {
        try {
          await context.close();
        } finally {
          await Promise.all([observedHelper, observedCapture, observedLifecycle, observedReady, ...routeTasks]);
          for (const handle of handles) await handle.dispose();
        }
      }
    }
  };

  const noJSControl = await runNoJS(true, controlArrived);
  requireSetup(noJSControl.valid, 'original helper failed initially-ready no-JS positive control');
  console.log('PASS arrival-readiness initially-ready no-JS positive control');
  // The existing async fault keeps its own assertion and original controls.
  // The no-JS host-poll control is independent of that selected async fault.
  if (!MUTATE) {
    const noJS = await runNoJS(false, arrived);
    if (noJS.timeout) {
      console.error('caught: arrival-readiness no-JS helper did not complete after actual readiness');
      process.exitCode = 1;
    } else if (!noJS.valid) {
      console.error('FAIL arrival-readiness no-JS helper returned before actual readiness');
      process.exitCode = 1;
    } else {
      console.log('PASS arrival-readiness no-JS false-to-ready');
    }
  }
  if (!delayed.appCSS) {
    console.error('caught: arrival-readiness returned before app stylesheet loaded');
    if (MUTATE && applied) console.log(`MUTATE-RESULT: caught ${MODE}`);
    process.exitCode = 1;
  } else if (delayed.animations.some((animation) => animation.pending || ['running', 'paused'].includes(animation.state))) {
    console.error('FAIL arrival-readiness returned before arrival finished');
    process.exitCode = MUTATE ? 2 : 1;
  } else if (!process.exitCode) {
    console.log('PASS arrival-readiness delayed-css and actual arrival finished');
  }
} catch (error) {
  console.error(`BROKEN arrival-readiness: ${error.stack || error}`);
  if (MUTATE) console.log(`MUTATE-RESULT: not-applied ${MUTATE}`);
  process.exitCode = 2;
} finally {
  await browser?.close();
}
