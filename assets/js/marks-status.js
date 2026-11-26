// The line at the bottom of the page where a change to a mark says what it did.
//
// It stays until the next thing is said or the reader dismisses it: nothing here
// takes it away on a timer, because a line that goes by itself is read by those
// who happened to look. A failure is a separate region with the alert role, so a
// screen reader says it at once; everything else is polite.

export function createStatus(element) {
  const said = element.querySelector('[data-mstatus-said]');
  const alert = element.querySelector('[data-mstatus-alert]');
  const act = element.querySelector('[data-mstatus-act]');
  const close = element.querySelector('[data-mstatus-close]');
  let run = null;

  function clear() {
    said.textContent = '';
    alert.textContent = '';
    act.hidden = true;
    act.textContent = '';
    run = null;
    element.hidden = true;
  }

  // say puts words in the line. action is a button beside them, which does what
  // its label says; failed puts the words where they are spoken at once.
  function say(text, { action = null, failed = false } = {}) {
    said.textContent = '';
    alert.textContent = '';
    (failed ? alert : said).textContent = text;
    run = action?.run ?? null;
    act.hidden = !action;
    act.textContent = action?.label ?? '';
    element.hidden = false;
  }

  act.addEventListener('click', () => {
    const go = run;
    if (go) go();
  });
  close.addEventListener('click', clear);
  return { say, clear, shown: () => !element.hidden, element };
}
