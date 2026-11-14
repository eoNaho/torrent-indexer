import { applyMode, scheduleMode } from './shared.js';
import { initNav } from './nav.js';
import { initWakeCard } from './wake.js';
import { initStatus } from './status.js';

function init() {
  applyMode();
  if (document.body) {
    const observer = new MutationObserver(scheduleMode);
    observer.observe(document.body, { childList: true, subtree: true });
  }
  initNav();
  initWakeCard();
  initStatus();
}

if (!window.__homeExtrasLoaded) {
  window.__homeExtrasLoaded = true;
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
}
