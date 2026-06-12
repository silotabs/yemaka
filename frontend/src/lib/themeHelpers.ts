export function normalizeTheme(theme: string | null | undefined) {
  const value = String(theme || '').trim().toLowerCase();
  return value === 'light' || value === 'dark' ? value : 'system';
}

export function resolvedTheme(theme: string) {
  if (theme === 'system') {
    return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  }
  return theme;
}

export function applyTheme(theme: string) {
  const normalized = normalizeTheme(theme);
  const resolved = resolvedTheme(normalized);
  document.documentElement.dataset.themePreference = normalized;
  document.documentElement.dataset.theme = resolved;
  localStorage.setItem('yemaka.theme', normalized);
}
