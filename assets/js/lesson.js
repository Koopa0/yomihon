// Lesson-only enhancements: shared Japanese speech, sentence controls, slot
// practice, and native concept sheets. The authored lesson remains readable
// when this module is absent or speech is unavailable.
// speechLanguage reads the voice off the passage the server already marked,
// rather than naming a language here that the server names too. The search
// starts above the button because the button carries its own lang — its label
// is interface Chinese wrapped around Japanese text — so asking the button
// would speak the passage in the language of its own label.
function speechLanguage(trigger) {
  const declared = trigger?.parentElement?.closest?.('[lang]')?.getAttribute('lang');
  return declared && declared !== 'und' ? declared : 'ja-JP';
}

export function initLesson() {
  let speechRate = 0.8;
  let speechGeneration = 0;
  let activeSpeakButton = null;
  let speechStatus = null;
  // The read-aloud bar's words come from the page, in the language its reader
  // asked for. A sentence written here would be written in one language for
  // everyone, on a page that is otherwise theirs. Each is named in full rather
  // than reached through the dataset object, so the check that pairs every
  // attribute the server writes with something that reads it can see them.
  const column = document.querySelector('[data-readaloud-controls]');
  const controlsLabel = column?.dataset.readaloudControls ?? '';
  const speedLabel = column?.dataset.readaloudSpeed ?? '';
  const rateTemplate = column?.dataset.readaloudRate ?? '';
  const stopLabel = column?.dataset.readaloudStop ?? '';
  const stopThisLabel = column?.dataset.readaloudStopthis ?? '';
  const stoppedLabel = column?.dataset.readaloudStopped ?? '';
  const playingLabel = column?.dataset.readaloudPlaying ?? '';
  const finishedLabel = column?.dataset.readaloudFinished ?? '';
  const unavailableLabel = column?.dataset.readaloudUnavailable ?? '';

  function resetSpeakButton() {
    if (!activeSpeakButton) return;
    activeSpeakButton.removeAttribute('data-speaking');
    // The server labelled this button; putting its own label back is one
    // fewer copy of the same sentence than carrying a second one here.
    if (activeSpeakButton.dataset.readaloudIdle) {
      activeSpeakButton.setAttribute('aria-label', activeSpeakButton.dataset.readaloudIdle);
    }
    activeSpeakButton = null;
  }

  function announce(text) {
    if (speechStatus && text) speechStatus.textContent = text;
  }

  function stopSpeech() {
    if (!('speechSynthesis' in window)) return;
    speechGeneration += 1;
    speechSynthesis.cancel();
    resetSpeakButton();
    announce(stoppedLabel);
  }

  function speakJapanese(text, trigger = null) {
    if (!text || !('speechSynthesis' in window)) return;
    if (trigger && trigger === activeSpeakButton) {
      stopSpeech();
      return;
    }
    stopSpeech();
    const generation = speechGeneration;
    const utterance = new SpeechSynthesisUtterance(text);
    // The note says what language it is in; reading it aloud in another one is
  // not a smaller version of the feature, it is the wrong words. A note that
  // declares nothing falls back to the passage's own marker.
  utterance.lang = speechLanguage(trigger);
    utterance.rate = speechRate;
    if (trigger) {
      activeSpeakButton = trigger;
      trigger.setAttribute('data-speaking', '');
      trigger.dataset.readaloudIdle ??= trigger.getAttribute('aria-label') ?? '';
      if (stopThisLabel) trigger.setAttribute('aria-label', stopThisLabel);
    }
    utterance.addEventListener('start', () => {
      if (generation === speechGeneration) announce(playingLabel);
    }, { once: true });
    utterance.addEventListener('end', () => {
      if (generation === speechGeneration) {
        announce(finishedLabel);
        resetSpeakButton();
      }
    }, { once: true });
    utterance.addEventListener('error', () => {
      if (generation === speechGeneration) {
        announce(unavailableLabel);
        resetSpeakButton();
      }
    }, { once: true });
    speechSynthesis.speak(utterance);
  }

  function initTextToSpeech() {
    if (!('speechSynthesis' in window)) return;
    const buttons = [...document.querySelectorAll('[data-tts]')];
    if (buttons.length === 0) return;

    const toolbar = document.createElement('div');
    toolbar.className = 'y-ttsbar';
    toolbar.setAttribute('role', 'group');
    toolbar.setAttribute('aria-label', controlsLabel);
    const label = document.createElement('span');
    label.className = 'y-ttsbar__label';
    label.textContent = speedLabel;
    toolbar.append(label);
    [0.8, 1].forEach((rate) => {
      const rateButton = document.createElement('button');
      rateButton.type = 'button';
      rateButton.textContent = `${rate.toFixed(1)}×`;
      rateButton.setAttribute('aria-pressed', String(rate === speechRate));
      rateButton.addEventListener('click', () => {
        speechRate = rate;
        toolbar.querySelectorAll('button[data-speech-rate]').forEach((candidate) => {
          candidate.setAttribute('aria-pressed', String(candidate === rateButton));
        });
        stopSpeech();
        announce(rateTemplate.replace('{rate}', rate.toFixed(1)));
      });
      rateButton.dataset.speechRate = String(rate);
      toolbar.append(rateButton);
    });
    speechStatus = document.createElement('span');
    speechStatus.className = 'y-ttsbar__status';
    speechStatus.setAttribute('aria-live', 'polite');
    toolbar.append(speechStatus);
    const stopButton = document.createElement('button');
    stopButton.type = 'button';
    stopButton.className = 'y-ttsbar__stop';
    stopButton.textContent = stopLabel;
    stopButton.addEventListener('click', stopSpeech);
    toolbar.append(stopButton);
    buttons[0].closest('.y-reading')?.before(toolbar);

    buttons.forEach((button) => {
      button.addEventListener('click', () => speakJapanese(button.getAttribute('data-tts'), button));
    });
  }

  function initSlotCard(card) {
    const dataElement = card.querySelector('script.y-slotdata');
    if (!dataElement) return;
    let data;
    try {
      data = JSON.parse(dataElement.textContent);
    } catch {
      return;
    }
    const keys = data.keys || [];
    const selection = {};
    keys.forEach((key) => { selection[key] = 0; });
    const fill = (key) => {
      const slot = data.slots[key];
      if (!slot || !slot.fills.length) return null;
      return slot.fills[selection[key]] || slot.fills[0];
    };
    function render() {
      keys.forEach((key) => {
        const value = fill(key);
        if (!value) return;
        card.querySelectorAll(`.y-slotout[data-slot-key="${key}"]`).forEach((output) => {
          const base = output.querySelector('ruby > span');
          const reading = output.querySelector('rt');
          if (base) base.textContent = value.jp;
          if (reading) reading.textContent = value.reading;
        });
      });
      const gloss = card.querySelector('.y-slotgloss');
      if (gloss) {
        gloss.textContent = data.gloss.replace(/\{([A-Za-z0-9]+)\}/g, (_, key) => {
          const value = fill(key);
          return value ? value.zh : `{${key}}`;
        });
      }
    }
    // Shuffle moves every slot at once and writes the new choice straight into
    // each select, which is an assignment and so reports nothing: the sentence
    // and the gloss both change with no sound at all. This says what the card
    // now reads. It is built from the same data the sentence is built from, and
    // takes the gloss off the card after it has been rewritten, so the two can
    // never drift apart. The gloss is marked Traditional Chinese because the
    // region around it is Japanese.
    const live = card.querySelector('.y-slotlive');
    function announce() {
      if (!live) return;
      const sentence = document.createElement('span');
      sentence.textContent = data.template.replace(/\{([A-Za-z0-9]+)\}/g, (_, key) => fill(key)?.jp || '');
      const gloss = document.createElement('span');
      gloss.lang = 'zh-Hant';
      gloss.textContent = card.querySelector('.y-slotgloss')?.textContent || '';
      live.replaceChildren(sentence, ' ', gloss);
    }
    // Picking from a select is left alone: the select speaks the option itself,
    // and repeating the whole sentence over it would talk across that.
    card.querySelectorAll('select[data-slot-key]').forEach((select) => {
      select.addEventListener('change', () => {
        selection[select.getAttribute('data-slot-key')] = Number(select.value) || 0;
        render();
      });
    });
    card.querySelector('[data-slot-action="speak"]')?.addEventListener('click', () => {
      speakJapanese(data.template.replace(/\{([A-Za-z0-9]+)\}/g, (_, key) => fill(key)?.jp || ''));
    });
    card.querySelector('[data-slot-action="shuffle"]')?.addEventListener('click', () => {
      keys.forEach((key) => {
        const count = data.slots[key]?.fills.length || 0;
        if (!count) return;
        selection[key] = Math.floor(Math.random() * count);
        const select = card.querySelector(`select[data-slot-key="${key}"]`);
        if (select) select.value = String(selection[key]);
      });
      render();
      announce();
    });
  }

  function initConceptSheet() {
    const dialog = document.querySelector('[data-concept-sheet]');
    if (!dialog) return;
    const title = dialog.querySelector('[data-concept-title]');
    const body = dialog.querySelector('[data-concept-body]');
    document.addEventListener('click', (event) => {
      const trigger = event.target.closest('[data-concept]');
      if (trigger) {
        const template = document.getElementById(`concept-${trigger.getAttribute('data-concept')}`);
        if (!template) return;
        event.preventDefault();
        title.textContent = template.dataset.title || '';
        body.replaceChildren(template.content.cloneNode(true));
        body.scrollTop = 0;
        if (!dialog.open) dialog.showModal();
        return;
      }
      if (event.target.closest('[data-concept-close]')) {
        dialog.close();
        return;
      }
      if (event.target === dialog) dialog.close();
    });
  }

  initTextToSpeech();
  document.querySelectorAll('.y-slotcard').forEach(initSlotCard);
  initConceptSheet();
}
