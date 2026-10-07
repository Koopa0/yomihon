export const arrived = (page) => page.waitForFunction(
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
  },
  null,
  { timeout: 3000 },
);
