import { performance } from 'node:perf_hooks';
import { errors } from 'playwright-core';

// A page is ready to be read once its application stylesheet has been applied
// and, for a return to a kept place, the module that scrolls to the place has
// revealed the column. A page change plays no animation, so there is nothing
// else to wait out. Host polling also advances when application scripting is
// disabled. One deadline bounds both browser evaluation and retries.
export const arrived = async (page, timeout = 3000) => {
  const deadline = performance.now() + timeout;
  let last = { appCSS: null, held: null };
  let timer;
  let expired = false;
  const timeoutError = () => new errors.TimeoutError(
    `arrival readiness timed out after ${timeout}ms: ${JSON.stringify(last)}`,
  );
  const checkDeadline = () => {
    if (expired || performance.now() >= deadline) throw timeoutError();
  };
  const polling = async () => {
    for (;;) {
      checkDeadline();
      const result = await page.evaluate(() => {
        const appCSS = [...document.styleSheets].some((sheet) => {
          if (!sheet.href) return false;
          const url = new URL(sheet.href);
          return url.origin === location.origin && url.pathname === '/static/app.css';
        });
        const held = document.documentElement.dataset.arrival !== undefined;
        // Read the boolean inside the browser before Playwright can await a
        // Promise returned by a faulty async predicate.
        return { ready: Boolean(appCSS && !held), appCSS, held };
      });
      last = { appCSS: result.appCSS, held: result.held };
      checkDeadline();
      if (result.ready) return;
      await new Promise((resolve) => setTimeout(resolve, Math.min(50, deadline - performance.now())));
    }
  };
  try {
    await Promise.race([
      polling(),
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(timeoutError()), Math.max(0, Math.ceil(deadline - performance.now())));
      }),
    ]);
  } finally {
    expired = true;
    clearTimeout(timer);
  }
};
