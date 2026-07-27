// Hold-to-submit enhancement over the ordinary status forms, plus removal of
// the one-shot post-redirect seal signal.
export function initStatus() {
  const holdDuration = 430;
  let holdTimer = null;
  let holding = false;
  let sealing = false;
  // A click is ceremony-owned only when a preceding pointer/key event armed it.
  // Unowned activation (including HTMLElement.click() and AT) stays native.
  let pendingHoldClick = null;

  function sealFills() {
    return document.querySelectorAll('.y-sealfill');
  }

  function armHoldClick(activation) {
    pendingHoldClick = activation;
  }

  function matchingHoldClick(button, source, pointerId) {
    if (!pendingHoldClick || pendingHoldClick.button !== button || pendingHoldClick.source !== source) return null;
    if (source === 'pointer' && pendingHoldClick.pointerId !== pointerId) return null;
    return pendingHoldClick;
  }

  function releaseHoldClick(activation) {
    if (pendingHoldClick === activation) pendingHoldClick = null;
  }

  function releaseHoldClickAfterNativeDefault(activation) {
    if (!activation) return;
    setTimeout(() => releaseHoldClick(activation), 0);
  }

  function ownsHoldClick(event, button) {
    const activation = pendingHoldClick;
    if (!activation || activation.button !== button || !event.isTrusted) return false;
    if (
      activation.source === 'pointer' &&
      typeof event.pointerId === 'number' &&
      event.pointerId !== activation.pointerId
    ) {
      return false;
    }
    releaseHoldClick(activation);
    return true;
  }

  // A control the working tree would refuse is rendered inert, and the hold has
  // to ask the same question the markup answered. Two ways past it otherwise:
  // Chrome still dispatches pointerdown on a disabled button, and requestSubmit
  // submits a form whose only submit button is disabled — so the ceremony would
  // become the one route around a refusal every other route already respects.
  // Read live, because the page is the only thing that knows the answer.
  function sealRefused(form) {
    const button = form && form.querySelector('[data-seal-btn]');
    return !button || button.disabled;
  }

  function startHold(form) {
    if (sealing || holding || !form || sealRefused(form)) return;
    holding = true;
    sealFills().forEach((fill) => {
      fill.style.transition = `width ${holdDuration}ms linear`;
      fill.style.width = '100%';
    });
    holdTimer = setTimeout(() => {
      holding = false;
      sealing = true;
      form.requestSubmit();
    }, holdDuration + 20);
  }

  function cancelHold() {
    if (!holding) return;
    holding = false;
    clearTimeout(holdTimer);
    sealFills().forEach((fill) => {
      fill.style.transition = 'width 150ms ease';
      fill.style.width = '0';
    });
  }

  document.querySelectorAll('[data-seal]').forEach((form) => {
    const button = form.querySelector('[data-seal-btn]');
    if (!button) return;
    button.addEventListener('click', (event) => {
      if (!ownsHoldClick(event, button)) return;
      event.preventDefault();
    });
    button.addEventListener('pointerdown', (event) => {
      event.preventDefault();
      armHoldClick({ button, pointerId: event.pointerId, source: 'pointer' });
      startHold(form);
    });
    button.addEventListener('pointerup', (event) => {
      cancelHold();
      releaseHoldClickAfterNativeDefault(matchingHoldClick(button, 'pointer', event.pointerId));
    });
    button.addEventListener('pointerleave', (event) => {
      cancelHold();
      releaseHoldClick(matchingHoldClick(button, 'pointer', event.pointerId));
    });
    button.addEventListener('pointercancel', (event) => {
      cancelHold();
      releaseHoldClick(matchingHoldClick(button, 'pointer', event.pointerId));
    });
    button.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      if (event.repeat) return;
      armHoldClick({ button, source: 'keyboard' });
      startHold(form);
    });
    button.addEventListener('keyup', (event) => {
      if (event.key !== 'Enter' && event.key !== ' ') return;
      event.preventDefault();
      cancelHold();
      releaseHoldClickAfterNativeDefault(matchingHoldClick(button, 'keyboard'));
    });
  });
  window.addEventListener('blur', () => {
    cancelHold();
    releaseHoldClick(pendingHoldClick);
  });

  const url = new URL(location.href);
  if (url.searchParams.get('sealed') === '1') {
    url.searchParams.delete('sealed');
    history.replaceState(null, '', url);
  }

  const shortcutForm = document.querySelector('[data-seal]');
  return {
    // Answering false leaves R with its native meaning on a note whose seal is
    // refused, instead of swallowing the key for an act that cannot happen.
    canStartShortcutHold: () => Boolean(shortcutForm && !sealing && !sealRefused(shortcutForm)),
    startShortcutHold: () => startHold(shortcutForm),
    cancelHold,
  };
}
