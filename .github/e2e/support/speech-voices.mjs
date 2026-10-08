// This fixture supplies browser capabilities, never the page's voice choice.
// Existing probes keep their own speak/cancel handlers unless record is set.
export const LOCAL_VOICES = [
  { name: 'Japanese device voice', lang: 'ja-JP', localService: true, default: true },
  { name: 'Traditional Chinese device voice', lang: 'zh-TW', localService: true, default: false },
  { name: 'English device voice', lang: 'en-US', localService: true, default: false },
  { name: 'French device voice', lang: 'fr-FR', localService: true, default: false },
];

export async function installSpeechVoices(page, {
  voices = LOCAL_VOICES, record = false, startOnSpeak = true, endOnCancel = false,
  manualReadinessDeadline = false,
} = {}) {
  await page.addInitScript((configuration) => {
    const selected = new WeakMap();
    Object.defineProperty(SpeechSynthesisUtterance.prototype, 'voice', {
      configurable: true,
      get() { return selected.get(this) ?? null; },
      set(voice) { selected.set(this, voice); },
    });
    const fixture = {
      voices: configuration.voices,
      reads: 0,
      utterances: [],
      receipts: [],
      cancels: 0,
      pending: null,
      setVoices(voices, notify = true) {
        this.voices = voices;
        if (notify) speechSynthesis.dispatchEvent(new Event('voiceschanged'));
      },
      end() {
        const utterance = this.pending;
        this.pending = null;
        utterance?.dispatchEvent(new Event('end'));
      },
      error() {
        const utterance = this.pending;
        this.pending = null;
        const event = new Event('error');
        Object.defineProperty(event, 'error', { value: 'language-unavailable' });
        utterance?.dispatchEvent(event);
      },
    };
    window.__speechFixture = fixture;
    // Only the explicitly armed one-second readiness deadline is controlled.
    // Startup, utterance events and every other timer use the native scheduler.
    if (configuration.manualReadinessDeadline) {
      const nativeSetTimeout = window.setTimeout.bind(window);
      const nativeClearTimeout = window.clearTimeout.bind(window);
      const deadlines = new Map();
      let armed = false;
      fixture.armReadinessDeadline = () => { armed = true; };
      fixture.readinessDeadlines = () => [...deadlines.values()].map(({ id, delay, state }) => ({ id, delay, state }));
      fixture.expireReadinessDeadline = () => {
        const pending = [...deadlines.values()].filter((deadline) => deadline.state === 'pending');
        if (pending.length !== 1) throw new Error(`expected one pending readiness deadline, got ${pending.length}`);
        const deadline = pending[0];
        deadline.state = 'expired';
        deadlines.delete(deadline.id);
        deadline.callback.call(window, ...deadline.args);
      };
      window.setTimeout = (callback, delay, ...args) => {
        if (!armed || delay !== 1000 || typeof callback !== 'function') {
          return nativeSetTimeout(callback, delay, ...args);
        }
        // Reserve a browser timer identity so clearTimeout keeps its native
        // numeric API without colliding with an unrelated timer.
        const id = nativeSetTimeout(() => {}, 2147483647);
        nativeClearTimeout(id);
        deadlines.set(id, { id, delay, state: 'pending', callback, args });
        return id;
      };
      window.clearTimeout = (id) => {
        const deadline = deadlines.get(id);
        if (deadline) {
          deadlines.delete(id);
          return;
        }
        nativeClearTimeout(id);
      };
    }
    speechSynthesis.getVoices = () => {
      fixture.reads += 1;
      return [...fixture.voices];
    };
    if (!configuration.record) return;
    speechSynthesis.speak = (utterance) => {
      fixture.utterances.push(utterance);
      // Capture at handoff, so later mutation of an utterance cannot rewrite
      // what the product actually handed to the browser.
      fixture.receipts.push({
        text: utterance.text, lang: utterance.lang,
        voice: utterance.voice ? { ...utterance.voice } : null,
        voiceWasListed: fixture.voices.includes(utterance.voice),
      });
      fixture.pending = utterance;
      if (configuration.startOnSpeak) {
        setTimeout(() => utterance.dispatchEvent(new Event('start')), 0);
      }
    };
    speechSynthesis.cancel = () => {
      fixture.cancels += 1;
      const utterance = fixture.pending;
      fixture.pending = null;
      if (configuration.endOnCancel && utterance) {
        setTimeout(() => utterance.dispatchEvent(new Event('end')), 0);
      }
    };
  }, { voices, record, startOnSpeak, endOnCancel, manualReadinessDeadline });
}
