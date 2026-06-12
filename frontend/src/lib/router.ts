import type { Tab } from './appOptions';

export type IsTab = (value: string | null) => value is Tab;

export function tabRouteHash(tab: Tab) {
  return `#/${tab}`;
}

export function tabFromHash(hash: string, isTab: IsTab): Tab | null {
  const clean = String(hash || '')
    .replace(/^#/, '')
    .replace(/^\//, '')
    .split(/[?#]/)[0]
    .trim();
  return isTab(clean) ? clean : null;
}

export function currentRouteTab(isTab: IsTab) {
  if (typeof window === 'undefined') return null;
  return tabFromHash(window.location.hash, isTab);
}

export function replaceRouteForTab(tab: Tab) {
  if (typeof window === 'undefined') return;
  const nextHash = tabRouteHash(tab);
  if (window.location.hash === nextHash) return;
  window.history.replaceState(null, '', `${window.location.pathname}${window.location.search}${nextHash}`);
}

export function navigateRouteForTab(tab: Tab) {
  if (typeof window === 'undefined') return false;
  const nextHash = tabRouteHash(tab);
  if (window.location.hash === nextHash) return false;
  window.location.hash = `/${tab}`;
  return true;
}

export function startHashRouter(isTab: IsTab, onRoute: (tab: Tab) => void | Promise<void>, fallback: () => Tab) {
  const handleRoute = () => {
    const tab = currentRouteTab(isTab);
    if (!tab) {
      replaceRouteForTab(fallback());
      return;
    }
    void onRoute(tab);
  };
  window.addEventListener('hashchange', handleRoute);
  return () => window.removeEventListener('hashchange', handleRoute);
}
