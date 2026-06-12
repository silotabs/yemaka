const collapsePrefix = 'yemaka.collapse.';

export function stableCollapseKey(scope: string, parts: Array<string | number | undefined | null>) {
  const subject = parts
    .map((part) => String(part ?? '').trim())
    .filter(Boolean)
    .join(':')
    .toLowerCase()
    .replace(/[^a-z0-9:_-]+/g, '_')
    .replace(/_+/g, '_')
    .slice(0, 180);
  return `${collapsePrefix}${scope}:${subject || 'default'}`;
}

export function readPersistedCollapse(key: string, fallback = false) {
  if (!key || typeof localStorage === 'undefined') return fallback;
  try {
    const value = localStorage.getItem(key);
    if (value === 'true') return true;
    if (value === 'false') return false;
  } catch {
    return fallback;
  }
  return fallback;
}

export function writePersistedCollapse(key: string, collapsed: boolean) {
  if (!key || typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(key, collapsed ? 'true' : 'false');
  } catch {
    // Ignore persistence failures; collapse still works for the current render.
  }
}
