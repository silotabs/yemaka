export type LiveRefreshReason = 'interval' | 'focus' | 'visible';

type LiveRefreshOptions = {
  intervalMs?: number;
  refresh: (reason: LiveRefreshReason) => void | Promise<void>;
  shouldRefresh?: () => boolean;
};

export function startVisibleAutoRefresh(options: LiveRefreshOptions) {
  const intervalMs = Math.max(3000, options.intervalMs ?? 9000);
  let running = false;

  const run = (reason: LiveRefreshReason) => {
    if (running) return;
    if (document.visibilityState === 'hidden') return;
    if (options.shouldRefresh && !options.shouldRefresh()) return;
    running = true;
    Promise.resolve(options.refresh(reason))
      .catch(() => {
        // Surface-specific refresh functions already record user-facing errors.
      })
      .finally(() => {
        running = false;
      });
  };

  const onFocus = () => run('focus');
  const onVisible = () => {
    if (document.visibilityState === 'visible') run('visible');
  };

  const timer = window.setInterval(() => run('interval'), intervalMs);
  window.addEventListener('focus', onFocus);
  document.addEventListener('visibilitychange', onVisible);

  return () => {
    window.clearInterval(timer);
    window.removeEventListener('focus', onFocus);
    document.removeEventListener('visibilitychange', onVisible);
  };
}
