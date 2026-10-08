import { performance } from 'node:perf_hooks';
import { errors } from 'playwright-core';

// Host polling also advances when application scripting is disabled. One
// deadline bounds both browser evaluation and retries.
export const arrived = async (page, timeout = 3000) => {
  const deadline = performance.now() + timeout;
  let last = { appCSS: null, animations: null };
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
        const animations = document.getAnimations()
          .filter((animation) => animation.animationName === 'y-come-forward')
          .map((animation) => ({ state: animation.playState, pending: animation.pending }));
        function arrivalReady() {
          return appCSS && !animations.some((animation) =>
            animation.pending || ['running', 'paused'].includes(animation.state));
        }
        // Read the boolean inside the browser before Playwright can await a
        // Promise returned by a faulty async predicate.
        return { ready: Boolean(arrivalReady()), appCSS, animations };
      });
      last = { appCSS: result.appCSS, animations: result.animations };
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
