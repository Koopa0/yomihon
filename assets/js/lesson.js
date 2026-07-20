// Lesson-only enhancements: shared Japanese speech, sentence controls, slot
// practice, and native concept sheets. The authored lesson remains readable
// when this module is absent or speech is unavailable.
export function initLesson() {
  let speechRate = 0.8;
  let speechGeneration = 0;
  let activeSpeakButton = null;
  let speechStatus = null;

  function resetSpeechButton() {
    if (!activeSpeakButton) return;
    activeSpeakButton.removeAttribute('data-speaking');
    activeSpeakButton.setAttribute('aria-label', '朗讀這段日文');
    activeSpeakButton = null;
  }

  function stopSpeech() {
    if (!('speechSynthesis' in window)) return;
    speechGeneration += 1;
    speechSynthesis.cancel();
    resetSpeechButton();
    if (speechStatus) speechStatus.textContent = '已停止';
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
    utterance.lang = 'ja-JP';
    utterance.rate = speechRate;
    if (trigger) {
      activeSpeakButton = trigger;
      trigger.setAttribute('data-speaking', '');
      trigger.setAttribute('aria-label', '停止朗讀');
    }
    utterance.addEventListener('start', () => {
      if (generation === speechGeneration && speechStatus) speechStatus.textContent = '播放中';
    }, { once: true });
    utterance.addEventListener('end', () => {
      if (generation === speechGeneration && speechStatus) speechStatus.textContent = '播放完成';
      if (generation === speechGeneration) resetSpeechButton();
    }, { once: true });
    utterance.addEventListener('error', () => {
      if (generation === speechGeneration && speechStatus) speechStatus.textContent = '目前無法播放日語語音';
      if (generation === speechGeneration) resetSpeechButton();
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
    toolbar.setAttribute('aria-label', '日文朗讀控制');
    const label = document.createElement('span');
    label.className = 'y-ttsbar__label';
    label.textContent = '朗讀速度';
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
        if (speechStatus) speechStatus.textContent = `速度 ${rate.toFixed(1)}×`;
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
    stopButton.textContent = '停止';
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
