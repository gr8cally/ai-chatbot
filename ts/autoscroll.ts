export function setupAutoScroll(containerId: string): void {
  const container = document.getElementById(containerId);
  if (!container) return;

  let userScrolledUp = false;

  container.addEventListener('scroll', () => {
    const atBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 60;
    userScrolledUp = !atBottom;
  });

  const observer = new MutationObserver(() => {
    if (!userScrolledUp) {
      container.scrollTop = container.scrollHeight;
    }
  });

  observer.observe(container, {
    childList: true,
    subtree: true,
    characterData: true,
  });
}
