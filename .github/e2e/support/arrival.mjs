import { performance } from 'node:perf_hooks';
import { errors } from 'playwright-core';

// Host polling also advances when application scripting is disabled. One
// deadline owns protocol work, value reads, false-handle disposal and retries.
export const arrived = async (page, timeout = 3000) => {
  const deadline = performance.now() + timeout;
  const timeoutError = new errors.TimeoutError(`arrival readiness timed out after ${timeout}ms`);
  let timer;
  let expired = false;
  let owned;
  const dispose = (handle) => Promise.resolve().then(() => handle.dispose()).catch(() => {});
  const checkDeadline = () => {
    if (expired || performance.now() >= deadline) throw timeoutError;
  };
  const polling = async () => {
    for (;;) {
      checkDeadline();
      const handle = await page.evaluateHandle(() => {
        function arrivalReady() {
          const appCSS = [...document.styleSheets].some((sheet) => {
            if (!sheet.href) return false;
            const url = new URL(sheet.href);
            return url.origin === location.origin && url.pathname === '/static/app.css';
          });
          if (!appCSS) return false;
          return !document.getAnimations().some((animation) =>
            animation.animationName === 'y-come-forward'
            && (animation.pending || ['running', 'paused'].includes(animation.playState)));
        }
        // Convert before Playwright can await a Promise. The async mutation
        // must still expose its erroneous truthy Promise as early readiness.
        return Boolean(arrivalReady());
      }).then((handle) => {
        if (expired || performance.now() >= deadline) {
          void dispose(handle);
          throw timeoutError;
        }
        owned = handle;
        return handle;
      });
      const ready = await handle.jsonValue();
      checkDeadline();
      if (ready) {
        owned = undefined;
        return handle;
      }
      owned = undefined;
      await handle.dispose();
      checkDeadline();
      await new Promise((resolve) => setTimeout(resolve, Math.min(50, deadline - performance.now())));
    }
  };
  try {
    return await Promise.race([
      polling(),
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(timeoutError), Math.max(0, Math.ceil(deadline - performance.now())));
      }),
    ]);
  } finally {
    expired = true;
    clearTimeout(timer);
    // Outstanding protocol promises stay observed by Promise.race; late
    // handles are disposed above. Cleanup cannot extend a missed deadline.
    if (owned) void dispose(owned);
  }
};
