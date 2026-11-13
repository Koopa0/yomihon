// Every piece of text a reader can see, with the size the browser set it at:
// an element's own words, or the value or hint of a field. Text clipped to a
// pixel for a listener, and anything not drawn, is not text a reader sees.
// `within` is a selector; each piece says whether it stands inside an element
// that matches it, so a probe can ask about one region against the rest.
export const readSizes = (page, within = null) => page.evaluate((region) => {
  const found = [];
  const seen = (element) => {
    const rect = element.getBoundingClientRect();
    return rect.width > 2 && rect.height > 2 && element.checkVisibility({ checkOpacity: true, checkVisibilityCSS: true });
  };
  for (const element of document.body.querySelectorAll('*')) {
    if (['SCRIPT', 'STYLE', 'NOSCRIPT', 'TEMPLATE'].includes(element.tagName)) continue;
    const own = [...element.childNodes].some((node) => node.nodeType === Node.TEXT_NODE && node.textContent.trim() !== '');
    const field = element.matches('input, textarea') && (element.value !== '' || element.placeholder !== '');
    if (!own && !field) continue;
    if (!seen(element)) continue;
    found.push({
      size: Number.parseFloat(getComputedStyle(element).fontSize),
      what: `${element.tagName.toLowerCase()}${element.className ? `.${String(element.className).trim().split(/\s+/).join('.')}` : ''}`,
      words: (element.value || element.textContent || '').trim().slice(0, 30),
      inside: region ? element.closest(region) !== null : false,
    });
  }
  return found;
}, within);

