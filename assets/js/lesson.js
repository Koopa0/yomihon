// Lesson-only enhancements: shared Japanese speech, sentence controls, slot
// practice, and native concept sheets. The authored lesson remains readable
// when this module is absent or speech is unavailable.
// speechLanguage reads the voice off the passage the server already marked,
// rather than naming a language here that the server names too. It is given the
// passage, never the button: a button's own lang belongs to its label, which is
// interface Chinese wrapped around Japanese text, and a sentence spoken in the
// language of the label around it is the wrong voice. A paragraph's passage is
// what encloses its button; the practice card's is the line it rewrites, which
// the server marks Japanese however the chrome around it is written.
function speechLanguage(passage) {
  const declared = passage?.closest?.('[lang]')?.getAttribute('lang');
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
  const playAllLabel = column?.dataset.readaloudPlayall ?? '';
  const previousLabel = column?.dataset.readaloudPrevious ?? '';
  const nextLabel = column?.dataset.readaloudNext ?? '';
  const progressTemplate = column?.dataset.readaloudProgress ?? '';

  // Reading a note through: every marked paragraph in document order, the
  // cursor on the one the voice last took, and whether the walk is still live.
  // The cursor outlives the walk, so previous and next carry on from wherever
  // the reader was left rather than from the top of the note.
  let readingButtons = [];
  let cursor = -1;
  let running = false;
  let readingMark = null;
  let playAllButton = null;
  let previousButton = null;
  let nextButton = null;

  // The paragraph the voice is on, which the stylesheet colours. It is state
  // about a moment rather than a fact about the note, so no server can know it
  // and the page is never served carrying one.
  function markReading(wrapper) {
    if (readingMark === wrapper) return;
    readingMark?.removeAttribute('data-reading');
    readingMark = wrapper ?? null;
    readingMark?.setAttribute('data-reading', '');
  }

  // Giving the walk up. The cursor stays where it is: a reader who stops and
  // then asks for the next paragraph means the one after what they just heard.
  function endRun() {
    running = false;
    refreshRunControls();
  }

  function refreshRunControls() {
    playAllButton?.setAttribute('aria-pressed', String(running));
    // Refused at the ends rather than wrapping: a note read in order has a
    // first paragraph with nothing before it and a last with nothing after.
    setStep(previousButton, cursor <= 0);
    setStep(nextButton, cursor >= readingButtons.length - 1);
  }

  // Pressing previous back to the first paragraph is what switches previous
  // off, and a control that goes dead under the reader's finger takes their
  // place in the page with it: focus falls to the document and the next Tab
  // starts again from the top. Play through is the control beside it and is
  // never off, so the reader is left standing in the bar they were using.
  function setStep(button, off) {
    if (!button) return;
    if (off && document.activeElement === button) playAllButton?.focus();
    button.disabled = off;
  }

  function announceProgress(index) {
    announce(progressTemplate
      .replace('{n}', String(index + 1))
      .replace('{total}', String(readingButtons.length)));
  }

  function resetSpeakButton() {
    if (!activeSpeakButton) return;
    activeSpeakButton.removeAttribute('data-speaking');
    // The server labelled this button; putting its own label back is one
    // fewer copy of the same sentence than carrying a second one here.
    if (activeSpeakButton.dataset.readaloudIdle) {
      activeSpeakButton.setAttribute('aria-label', activeSpeakButton.dataset.readaloudIdle);
    }
    // The colour stands for the voice this button started, so the two are let
    // go in the same breath. Left behind, it points at a paragraph nothing is
    // reading and the reader follows it.
    markReading(null);
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

  // fromBar says the bar asked for this paragraph — the walk's own advance, or
  // a press on play through, previous or next. Every other press is a reader
  // asking for one paragraph and not for the ones after it, so it gives the
  // walk up and hears the voice announce itself; the bar has already said which
  // paragraph this is, and the voice starting is not news on top of that. The
  // giving up is the default, so a speaker added later has to say it belongs to
  // the walk before it can keep one alive.
  function speakJapanese(text, trigger = null, passage = trigger?.parentElement, fromBar = false) {
    if (!text || !('speechSynthesis' in window)) return;
    if (!fromBar) {
      endRun();
      if (trigger && trigger === activeSpeakButton) {
        stopSpeech();
        return;
      }
    }
    stopSpeech();
    const generation = speechGeneration;
    const utterance = new SpeechSynthesisUtterance(text);
    // The note says what language it is in; reading it aloud in another one is
    // not a smaller version of the feature, it is the wrong words. A note that
    // declares nothing falls back to the passage's own marker.
    utterance.lang = speechLanguage(passage);
    utterance.rate = speechRate;
    if (trigger) {
      activeSpeakButton = trigger;
      trigger.setAttribute('data-speaking', '');
      trigger.dataset.readaloudIdle ??= trigger.getAttribute('aria-label') ?? '';
      if (stopThisLabel) trigger.setAttribute('aria-label', stopThisLabel);
      markReading(trigger.closest('.y-reading'));
    }
    utterance.addEventListener('start', () => {
      if (generation === speechGeneration && !fromBar) announce(playingLabel);
    }, { once: true });
    utterance.addEventListener('end', () => {
      if (generation !== speechGeneration) return;
      if (running) {
        advance();
        return;
      }
      announce(finishedLabel);
      resetSpeakButton();
    }, { once: true });
    utterance.addEventListener('error', () => {
      if (generation !== speechGeneration) return;
      // The walk gives up here, so the sentence about the missing voice is said
      // once rather than once for every paragraph still ahead.
      endRun();
      announce(unavailableLabel);
      resetSpeakButton();
    }, { once: true });
    speechSynthesis.speak(utterance);
  }

  // One paragraph of the walk. Whether the walk goes on afterwards is the
  // caller's to say: an advance and play through keep it, previous and next
  // keep whatever the reader already had, and a press on a paragraph's own
  // speaker never comes through here at all.
  function speakAt(index, keepRunning) {
    const button = readingButtons[index];
    if (!button) return;
    cursor = index;
    running = keepRunning;
    // The passage is left to the default — the element enclosing this
    // paragraph's own speaker — so a note read through resolves the voice once
    // for each paragraph rather than once for the note.
    speakJapanese(button.getAttribute('data-tts'), button, undefined, true);
    announceProgress(index);
    refreshRunControls();
  }

  // The walk moves on when an utterance ends of its own accord.
  function advance() {
    if (cursor + 1 >= readingButtons.length) {
      endRun();
      announce(finishedLabel);
      resetSpeakButton();
      return;
    }
    speakAt(cursor + 1, true);
  }

  // Previous and next move the reader whether a walk is live or not: mid-walk
  // they skip and the walk carries on from the new paragraph, at rest they read
  // one paragraph and stop there. Either way the same sentence says where the
  // reader now is, so skipping and advancing sound alike.
  function step(delta) {
    const target = cursor + delta;
    if (target < 0 || target >= readingButtons.length) return;
    speakAt(target, running);
  }

  function barButton(text, className) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = className;
    button.textContent = text;
    return button;
  }

  function initTextToSpeech() {
    if (!('speechSynthesis' in window)) return;
    // Above the test for marked paragraphs, because the practice card can be
    // speaking on a note that marks none. Speech outlives the page it was
    // started on, so a reader who leaves mid-sentence is otherwise followed by
    // the voice onto whatever they opened next.
    window.addEventListener('pagehide', () => {
      endRun();
      speechGeneration += 1;
      speechSynthesis.cancel();
    });
    readingButtons = [...document.querySelectorAll('[data-tts]')];
    if (readingButtons.length === 0) return;

    const toolbar = document.createElement('div');
    toolbar.className = 'y-ttsbar';
    toolbar.setAttribute('role', 'group');
    toolbar.setAttribute('aria-label', controlsLabel);
    playAllButton = barButton(playAllLabel, 'y-ttsbar__play');
    playAllButton.addEventListener('click', () => {
      if (running) {
        endRun();
        stopSpeech();
        return;
      }
      // Play through picks the reader up where they are: the paragraph the
      // voice last took, or the first when they have not started yet or are
      // standing on the last one with nothing after it.
      speakAt(cursor >= 0 && cursor < readingButtons.length - 1 ? cursor : 0, true);
    });
    previousButton = barButton(previousLabel, 'y-ttsbar__prev');
    previousButton.addEventListener('click', () => step(-1));
    nextButton = barButton(nextLabel, 'y-ttsbar__next');
    nextButton.addEventListener('click', () => step(1));
    const stopButton = barButton(stopLabel, 'y-ttsbar__stop');
    stopButton.addEventListener('click', () => {
      endRun();
      stopSpeech();
    });
    toolbar.append(playAllButton, previousButton, nextButton, stopButton);
    const label = document.createElement('span');
    label.className = 'y-ttsbar__label';
    label.textContent = speedLabel;
    toolbar.append(label);
    [0.8, 1, 1.25, 1.5].forEach((rate) => {
      const rateButton = document.createElement('button');
      rateButton.type = 'button';
      // The number as it is, not rounded to a fixed place: one decimal turns
      // 1.25 into a button reading 1.3× that sets a speed of 1.25.
      rateButton.textContent = `${rate}×`;
      rateButton.setAttribute('aria-pressed', String(rate === speechRate));
      rateButton.addEventListener('click', () => {
        speechRate = rate;
        toolbar.querySelectorAll('button[data-speech-rate]').forEach((candidate) => {
          candidate.setAttribute('aria-pressed', String(candidate === rateButton));
        });
        endRun();
        stopSpeech();
        announce(rateTemplate.replace('{rate}', String(rate)));
      });
      rateButton.dataset.speechRate = String(rate);
      toolbar.append(rateButton);
    });
    speechStatus = document.createElement('span');
    speechStatus.className = 'y-ttsbar__status';
    speechStatus.setAttribute('aria-live', 'polite');
    toolbar.append(speechStatus);
    readingButtons[0].closest('.y-reading')?.before(toolbar);
    refreshRunControls();

    readingButtons.forEach((button) => {
      button.addEventListener('click', () => {
        // The cursor follows a reader who presses a paragraph's own speaker, so
        // next afterwards means the paragraph after that one.
        cursor = readingButtons.indexOf(button);
        speakJapanese(button.getAttribute('data-tts'), button);
        refreshRunControls();
      });
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
    const speakButton = card.querySelector('[data-slot-action="speak"]');
    // Handing the button over is what makes this the same control the reader
    // already met further up the page: pressing it while it speaks stops,
    // instead of cancelling and starting the same sentence over again, and it
    // carries the speaking state and the stop label while it runs.
    speakButton?.addEventListener('click', () => {
      speakJapanese(
        data.template.replace(/\{([A-Za-z0-9]+)\}/g, (_, key) => fill(key)?.jp || ''),
        speakButton,
        card.querySelector('.y-slotoutput'),
      );
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

  // The section a link names, if it names one. A link that carries no fragment,
  // or one this browser cannot decode, names the note itself.
  function fragmentOf(href) {
    const hash = (href || '').split('#')[1];
    if (!hash) return '';
    try {
      return decodeURIComponent(hash);
    } catch {
      return hash;
    }
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
        if (!dialog.open) dialog.showModal();
        // A link may name one section of the note rather than the note. The
        // sheet is its own scrolling box, so the jump a page makes to an id
        // cannot reach inside it, and a reader who asked for a section arrived
        // at the top of the note with no sign of where they had asked to be.
        // Measured after the sheet is shown, because a closed one has no
        // layout to measure.
        const section = fragmentOf(trigger.getAttribute('href'));
        const target = section ? body.querySelector(`#${CSS.escape(section)}`) : null;
        body.scrollTop = target
          ? target.getBoundingClientRect().top - body.getBoundingClientRect().top + body.scrollTop
          : 0;
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
