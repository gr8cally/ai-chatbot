export function addCopyButtons(container: HTMLElement): void {
  container.querySelectorAll<HTMLButtonElement>('.copy-btn').forEach((btn) => {
    // Avoid adding duplicate listeners
    if (btn.dataset.listenerAttached) return;
    btn.dataset.listenerAttached = 'true';

    btn.addEventListener('click', async () => {
      const code = btn.getAttribute('data-code');
      if (!code) return;

      // Decode HTML entities
      const decoded = code
        .replace(/&amp;/g, '&')
        .replace(/&quot;/g, '"')
        .replace(/&lt;/g, '<')
        .replace(/&gt;/g, '>');

      try {
        await navigator.clipboard.writeText(decoded);
        btn.textContent = 'Copied!';
        btn.classList.add('copied');
        setTimeout(() => {
          btn.textContent = 'Copy';
          btn.classList.remove('copied');
        }, 2000);
      } catch {
        // Fallback for older browsers
        const textarea = document.createElement('textarea');
        textarea.value = decoded;
        textarea.style.position = 'fixed';
        textarea.style.opacity = '0';
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand('copy');
        document.body.removeChild(textarea);
        btn.textContent = 'Copied!';
        btn.classList.add('copied');
        setTimeout(() => {
          btn.textContent = 'Copy';
          btn.classList.remove('copied');
        }, 2000);
      }
    });
  });
}
