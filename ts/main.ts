import {cancelIncrementalRender, renderMarkdown, renderMarkdownIncremental} from './markdown';
import { addCopyButtons } from './clipboard';
import { setupAutoScroll } from './autoscroll';

let currentEventSource: EventSource | null = null;
let isStreaming = false;
let lastUserMessage = '';

const MAX_MESSAGE_LENGTH = 4000;

function init() {
  setupAutoScroll('chat-messages');
  renderExistingMessages();
  setupSuggestionButtons();
  setupTextareaAutoResize();
  setupKeyboardShortcuts();
  setupCharacterCounter();
  updateRelativeTimestamps();
  setInterval(updateRelativeTimestamps, 60000);

  // Listen for HTMX swap events to handle new content
  document.body.addEventListener('htmx:afterSwap', (event: Event) => {
    const detail = (event as CustomEvent).detail;
    const target = detail.target as HTMLElement;

    if (target.id === 'chat-messages') {
      // Check if we need to start streaming
      const streamTarget = document.getElementById('stream-target');
      if (streamTarget) {
        startStreaming(streamTarget);
      } else {
        // Non-streaming response - render markdown
        renderNewMessages(target);
      }

      // Remove welcome screen if present
      const welcome = document.getElementById('welcome-screen');
      if (welcome && target.children.length > 1) {
        welcome.remove();
      }

      updateRelativeTimestamps();
    }
  });

  // Re-render markdown and setup suggestions after clear
  document.body.addEventListener('htmx:afterSettle', () => {
    setupSuggestionButtons();
    updateRelativeTimestamps();
  });

  // Handle HTMX request errors (network failures)
  document.body.addEventListener('htmx:responseError', () => {
    setInputEnabled(true);
    showAbortButton(false);
  });

  // Expose retry function globally
  (window as unknown as WindowWithRetry).retryLastMessage = retryLastMessage;
}

function renderExistingMessages() {
  document.querySelectorAll<HTMLElement>('.markdown-body[data-raw-content]').forEach((el) => {
    const raw = el.getAttribute('data-raw-content');
    if (raw) {
      el.innerHTML = renderMarkdown(raw);
      addCopyButtons(el);
    }
  });
}

function renderNewMessages(container: HTMLElement) {
  container.querySelectorAll<HTMLElement>('.markdown-body[data-raw-content]').forEach((el) => {
    const raw = el.getAttribute('data-raw-content');
    if (raw && !el.classList.contains('rendered')) {
      el.innerHTML = renderMarkdown(raw);
      el.classList.add('rendered');
      addCopyButtons(el);
    }
  });
}

function startStreaming(target: HTMLElement) {
  const sseUrl = target.getAttribute('data-sse-url');
  if (!sseUrl) return;

  // Finish previous stream using a direct reference, not getElementById
  if (currentEventSource) {
    currentEventSource.close();
    currentEventSource = null;

    // Clean up any lingering cursors across the whole document
    document.querySelectorAll('.cursor').forEach((el) => el.remove());
  }

  isStreaming = true;
  setInputEnabled(false);
  showAbortButton(true);

  let accumulated = '';
  const es = new EventSource(sseUrl);
  currentEventSource = es;

  es.addEventListener('delta', (e: MessageEvent) => {
    // Unescape the server-escaped newlines
    let text = e.data;
    text = text.replace(/\\n/g, '\n').replace(/\\r/g, '\r');
    accumulated += text;
    if (isStreaming) {
      renderMarkdownIncremental(target, accumulated);
    }
  });

  es.addEventListener('done', () => {
    finishStreaming(target, accumulated, false);
  });

  es.addEventListener('error', (e: Event) => {
    console.error('SSE error:', e);
    if (accumulated.length === 0) {
      target.innerHTML = '<div class="error-content"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"></circle><line x1="15" y1="9" x2="9" y2="15"></line><line x1="9" y1="9" x2="15" y2="15"></line></svg><span>Connection lost. Please try again.</span></div>';
    }
    finishStreaming(target, accumulated, false);
  });
}

function finishStreaming(target: HTMLElement, accumulated: string, aborted: boolean) {
  cancelIncrementalRender();

  if (currentEventSource) {
    currentEventSource.close();
    currentEventSource = null;
  }

  isStreaming = false;
  target.classList.remove('streaming');
  target.removeAttribute('id'); // Remove id to avoid collision
  target.removeAttribute('data-sse-url');

  // Remove cursor (unified — no duplicate declaration)
  target.querySelectorAll('.cursor').forEach((el) => el.remove());

  if (accumulated.length > 0) {
    let html = renderMarkdown(accumulated);
    if (aborted) {
      html += '<div class="generation-stopped"><em>(generation stopped)</em></div>';
    }
    target.innerHTML = html;
    addCopyButtons(target);
  }

  setInputEnabled(true);
  showAbortButton(false);

  // Focus input
  const input = document.getElementById('message-input') as HTMLTextAreaElement;
  if (input) input.focus();

  // Reset char counter
  resetCharCounter();
}

function setInputEnabled(enabled: boolean) {
  const input = document.getElementById('message-input') as HTMLTextAreaElement;
  const sendBtn = document.getElementById('send-btn') as HTMLButtonElement;

  if (input) input.disabled = !enabled;
  if (sendBtn) sendBtn.disabled = !enabled;
}

function showAbortButton(show: boolean) {
  const container = document.getElementById('abort-container');
  if (container) {
    container.classList.toggle('hidden', !show);
  }
}

function setupCharacterCounter() {
  const textarea = document.getElementById('message-input') as HTMLTextAreaElement;
  const counter = document.getElementById('char-counter');
  if (!textarea || !counter) return;

  textarea.addEventListener('input', () => {
    const len = textarea.value.length;
    if (len > MAX_MESSAGE_LENGTH * 0.8) {
      counter.textContent = `${len}/${MAX_MESSAGE_LENGTH}`;
      counter.classList.remove('hidden');
      counter.classList.toggle('char-counter-warning', len > MAX_MESSAGE_LENGTH * 0.95);
    } else {
      counter.classList.add('hidden');
    }
  });
}

function resetCharCounter() {
  const counter = document.getElementById('char-counter');
  if (counter) {
    counter.classList.add('hidden');
    counter.textContent = '';
  }
}

function setupSuggestionButtons() {
  document.querySelectorAll<HTMLButtonElement>('.btn-suggestion').forEach((btn) => {
    btn.addEventListener('click', () => {
      const prompt = btn.getAttribute('data-prompt');
      if (!prompt) return;

      const input = document.getElementById('message-input') as HTMLTextAreaElement;
      if (input) {
        input.value = prompt;
        lastUserMessage = prompt;
        const form = document.getElementById('chat-form') as HTMLFormElement;
        if (form) {
          htmx.trigger(form, 'submit');
        }
      }
    });
  });
}

function setupTextareaAutoResize() {
  const textarea = document.getElementById('message-input') as HTMLTextAreaElement;
  if (!textarea) return;

  textarea.addEventListener('input', () => {
    textarea.style.height = 'auto';
    textarea.style.height = Math.min(textarea.scrollHeight, 150) + 'px';
  });
}

function setupKeyboardShortcuts() {
  const textarea = document.getElementById('message-input') as HTMLTextAreaElement;
  if (!textarea) return;

  textarea.addEventListener('keydown', (e: KeyboardEvent) => {
    // Enter to send (without Shift)
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (!textarea.disabled && textarea.value.trim()) {
        lastUserMessage = textarea.value.trim();
        const form = document.getElementById('chat-form') as HTMLFormElement;
        if (form) {
          htmx.trigger(form, 'submit');
        }
      }
    }
  });

  // Escape to abort
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Escape' && isStreaming) {
      abortStreaming();
    }
  });
}

function abortStreaming() {
  // Tell the server to cancel
  const abortBtn = document.getElementById('abort-btn') as HTMLButtonElement;
  if (abortBtn) abortBtn.click();

  // Close client-side connection and show partial content with "(generation stopped)"
  if (currentEventSource) {
    const target = document.getElementById('stream-target');
    if (target) {
      const cursorEl = target.querySelector('.cursor');
      if (cursorEl) cursorEl.remove();
      const textContent = target.textContent || '';
      finishStreaming(target, textContent, true);
    }
  }
}

function retryLastMessage(_btn: HTMLElement) {
  // Remove the error bubble
  const errorMessages = document.querySelectorAll('.message-error');
  errorMessages.forEach((el) => el.remove());

  if (!lastUserMessage) return;

  // Set the message in the input and resubmit
  const input = document.getElementById('message-input') as HTMLTextAreaElement;
  if (input) {
    input.value = lastUserMessage;
    const form = document.getElementById('chat-form') as HTMLFormElement;
    if (form) {
      htmx.trigger(form, 'submit');
    }
  }
}

function updateRelativeTimestamps() {
  document.querySelectorAll<HTMLElement>('.message-time[data-timestamp]').forEach((el) => {
    const ts = el.getAttribute('data-timestamp');
    if (!ts) return;
    const date = new Date(ts);
    el.textContent = formatRelativeTime(date);
  });
}

function formatRelativeTime(date: Date): string {
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHour = Math.floor(diffMin / 60);

  if (diffSec < 60) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHour < 24) return `${diffHour}h ago`;
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

// Declare htmx global
declare const htmx: {
  trigger: (element: HTMLElement, event: string) => void;
};

interface WindowWithRetry {
  retryLastMessage: (btn: HTMLElement) => void;
}

// Initialize when DOM is ready
if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', init);
} else {
  init();
}
