// Hold one application module until the first reveal. Routing disables the
// real back/forward cache, so the forced lifecycle and the real Back journey
// are observed in separate browser contexts.
export const lateArrivalRestore = async (browser, base, pagePath, module, suffix, mutate) => {
  const context = await browser.newContext({ viewport: { width: 1600, height: 900 } });
  try {
    const page = await context.newPage();
    let release;
    const revealed = new Promise((resolve) => { release = resolve; });
    await page.exposeBinding('__firstReveal', release);
    await page.addInitScript(() => {
      window.__arrivalRevealed = false;
      addEventListener('pagereveal', () => {
        window.__arrivalRevealed = true;
        window.__firstReveal();
      }, { once: true });
    });
    let loads = 0;
    await page.route(`**/${module}{,?*}`, async (route) => {
      const response = await route.fetch();
      if (response.status() !== 200) throw new Error(`BROKEN: ${module} returned ${response.status()}`);
      let source = await response.text();
      if (mutate) {
        const needle = '{ once: true, signal: arrival.signal }';
        const hits = source.split(needle).length - 1;
        if (hits !== 1) throw new Error(`NOT-APPLIED: ${module} arrival signal matched ${hits}, want 1`);
        source = source.replace(needle, '{ once: true }');
      }
      loads++;
      let timer;
      try {
        await Promise.race([
          revealed,
          new Promise((_, reject) => {
            timer = setTimeout(() => reject(new Error('BROKEN: module delivery did not follow first reveal')), 3000);
          }),
        ]);
      } finally { clearTimeout(timer); }
      await route.fulfill({ response, body: `window.__arrivalModuleWasLate = window.__arrivalRevealed;\n${source}` });
    });
    await page.goto(base + pagePath + suffix, { waitUntil: 'load' });
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    if (loads !== 1 || await page.evaluate(() => window.__arrivalModuleWasLate) !== true) {
      throw new Error(`BROKEN: ${module} was not delivered exactly once after reveal`);
    }
    const landed = await page.evaluate(() => scrollY);
    if (Math.abs(landed - 900) > 4) throw new Error(`BROKEN: arrival landed at ${landed}, want 900`);
    await page.evaluate(() => scrollTo(0, 1500));
    const before = await page.evaluate(() => scrollY);
    if (Math.abs(before - landed) < 500) throw new Error('BROKEN: reader did not move away from the arrival');
    const after = await page.evaluate(() => {
      dispatchEvent(new PageTransitionEvent('pagehide', { persisted: true }));
      dispatchEvent(new PageTransitionEvent('pageshow', { persisted: true }));
      dispatchEvent(new Event('pagereveal'));
      return scrollY;
    });
    console.log(`INVOCATION-HIT arrival-lifetime ${module}: late registration, landing=${landed}, before=${before}, after=${after}`);
    if (mutate) console.log(`APPLIED arrival-lifetime ${module}: removed the one arrival signal`);
    return { before, after };
  } finally { await context.close(); }
};

export const measureBackArrival = async (browser, base, pagePath, suffix) => {
  const context = await browser.newContext({ viewport: { width: 1600, height: 900 } });
  try {
    const page = await context.newPage();
    await page.addInitScript(() => {
      window.__arrivalBack = [];
      addEventListener('pageshow', (event) => window.__arrivalBack.push({ persisted: event.persisted, y: scrollY }));
      addEventListener('pagereveal', () => window.__arrivalBack.push({ reveal: true, y: scrollY }));
    });
    await page.goto(base + pagePath + suffix, { waitUntil: 'load' });
    await page.evaluate(() => new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await page.evaluate(() => scrollTo(0, 1500));
    const before = await page.evaluate(() => scrollY);
    await page.goto(base + '/notes/Notes/alpha.md', { waitUntil: 'load' });
    await page.goBack({ waitUntil: 'load' });
    const result = await page.evaluate(() => ({ after: scrollY, events: window.__arrivalBack }));
    console.log(`MEASURE arrival-lifetime Back: ${JSON.stringify({ before, ...result })}`);
  } finally { await context.close(); }
};
