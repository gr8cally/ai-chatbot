import { marked } from 'marked';
import hljs from 'highlight.js';
import DOMPurify from 'dompurify';

// Configure marked
marked.setOptions({
  breaks: true,
  gfm: true,
});

// Custom renderer for code blocks with language header and copy button
const renderer = new marked.Renderer();
renderer.code = function ({ text, lang }: { text: string; lang?: string; escaped?: boolean }): string {
  const language = lang && hljs.getLanguage(lang) ? lang : '';
  let highlighted: string;

  if (language) {
    highlighted = hljs.highlight(text, { language }).value;
  } else {
    highlighted = hljs.highlightAuto(text).value;
  }

  const langLabel = language || 'text';
  return `<pre><div class="code-header"><span>${langLabel}</span><button class="copy-btn" data-code="${escapeAttr(text)}">Copy</button></div><code class="hljs language-${langLabel}">${highlighted}</code></pre>`;
};

marked.use({ renderer });

function escapeAttr(str: string): string {
  return str
    .replace(/&/g, '&amp;')
    .replace(/"/g, '&quot;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;');
}

export function renderMarkdown(raw: string): string {
  const html = marked.parse(raw) as string;
  return DOMPurify.sanitize(html, {
    ADD_TAGS: ['button'],
    ADD_ATTR: ['data-code', 'class'],
  });
}

let debounceTimer: number | null = null;

export function renderMarkdownIncremental(element: HTMLElement, raw: string): void {
  if (debounceTimer) clearTimeout(debounceTimer);
  debounceTimer = window.setTimeout(() => {
    const html = renderMarkdown(raw);
    // Keep cursor at the end
    element.innerHTML = html + '<span class="cursor"></span>';
  }, 60);
}

export function cancelIncrementalRender(): void {
  if (debounceTimer) {
    clearTimeout(debounceTimer);
    debounceTimer = null;
  }
}