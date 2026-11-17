// Pins the language every probe's browser asks in, so a probe's result does not
// depend on which machine it runs on. The server answers a reader who has
// chosen no language in the one their browser asks for, and a probe is written
// against one of them: left alone, a probe passes on a machine whose browser
// asks in Chinese and fails on a CI runner whose browser asks in English.
//
// probes.sh loads this before each probe (node --import). A context that names
// its own locale keeps it, which is how a probe about English, or about the
// language following the browser, says so.
import { chromium } from 'playwright-core';

export const defaultLocale = 'zh-TW';

const launch = chromium.launch.bind(chromium);
chromium.launch = async (...args) => {
  const browser = await launch(...args);
  const newContext = browser.newContext.bind(browser);
  const newPage = browser.newPage.bind(browser);
  browser.newContext = (options = {}) => newContext({ locale: defaultLocale, ...options });
  browser.newPage = (options = {}) => newPage({ locale: defaultLocale, ...options });
  return browser;
};
