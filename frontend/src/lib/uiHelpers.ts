const identifierAcronyms = new Set([
  'ai',
  'api',
  'cli',
  'cpu',
  'docx',
  'fts',
  'gpu',
  'html',
  'http',
  'https',
  'id',
  'json',
  'llm',
  'mcp',
  'os',
  'pdf',
  'rag',
  'sql',
  'sqlite',
  'svg',
  'tui',
  'ui',
  'url',
  'xml',
  'yaml'
]);

export const userMessagePreviewChars = 900;
export const userMessagePreviewLines = 12;

export function sourceKindLabel(kind: string | undefined) {
  const value = String(kind || '').trim().toLowerCase();
  if (!value) return '';
  if (value === 'rag') return 'RAG';
  if (value === 'workspace') return 'workspace';
  if (value === 'internet') return 'internet';
  if (value === 'memory') return 'memory';
  if (value === 'tool' || value === 'tools') return 'tools';
  if (value === 'chat') return 'chat';
  return value.replace(/_/g, ' ');
}

export function humanizeIdentifier(value: string | undefined, fallback = 'Unknown') {
  const clean = String(value || '').trim();
  if (!clean) return fallback;
  if (/^[a-z]+:\/\//i.test(clean) || /^(?:\/|\.\/|\.\.\/|~\/)/.test(clean)) return clean;
  const words = String(value || '')
    .trim()
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/([A-Z]+)([A-Z][a-z])/g, '$1 $2')
    .split(/[\s_.:/-]+/)
    .filter(Boolean);
  if (!words.length) return fallback;
  return words
    .map((word) => {
      const lower = word.toLowerCase();
      if (identifierAcronyms.has(lower)) return lower.toUpperCase();
      return lower.charAt(0).toUpperCase() + lower.slice(1);
    })
    .join(' ');
}

export function humanizeIdentifierList(values: string[] | null | undefined, fallback = 'none') {
  const items = (values ?? []).map((value) => humanizeIdentifier(value, '')).filter(Boolean);
  return items.length ? items.join(', ') : fallback;
}

export function humanizePermission(value: string | undefined) {
  const clean = String(value || '').trim();
  if (!clean) return '';
  const normalized = clean.toLowerCase();
  const labels: Record<string, string> = {
    'scheduler=approval_required': 'Scheduler jobs require approval',
    'policy=core_enforced': 'Core policy enforced',
    'internet=core_broker': 'Internet uses core broker',
    'network=core_broker': 'Internet uses core broker',
    'network=task_scoped': 'Network is task scoped',
    'network=profile_enabled': 'Network requires profile enablement',
    'methods=get,head': 'GET/HEAD only',
    'network.methods=get,head': 'GET/HEAD only',
    'filesystem.read=false': 'No file read',
    'filesystem.write=false': 'No file write',
    'shell=false': 'No shell',
    'secrets=false': 'No secrets',
    'skills=profile_local': 'Profile-local skills',
    'settings=profile_local': 'Profile-local settings'
  };
  if (labels[normalized]) return labels[normalized];
  if (normalized.startsWith('max_parallel_jobs=')) {
    return `Max parallel jobs ${clean.slice(clean.indexOf('=') + 1)}`;
  }
  if (normalized.startsWith('search_provider=')) {
    return `Search provider ${humanizeIdentifier(clean.slice(clean.indexOf('=') + 1), 'none')}`;
  }
  const [key, rawValue] = clean.split('=');
  if (rawValue !== undefined) {
    return `${humanizeIdentifier(key, key)}: ${humanizeIdentifier(rawValue, rawValue)}`;
  }
  return humanizeIdentifier(clean, clean);
}

export function humanizePermissionList(values: string[] | null | undefined, fallback = 'none') {
  const items = (values ?? []).map((value) => humanizePermission(value)).filter(Boolean);
  return items.length ? items.join(', ') : fallback;
}

export function isAssistantActiveMeta(meta: string | undefined) {
  return meta === 'working' || meta === 'streaming';
}

export function compactSource(source: string) {
  const clean = String(source || '').replace(/\s+/g, ' ').trim();
  return clean.length > 92 ? `${clean.slice(0, 92)}...` : clean;
}

export function compactText(value: string | undefined, limit = 38) {
  const clean = String(value || '').replace(/\s+/g, ' ').trim();
  return clean.length > limit ? `${clean.slice(0, limit)}...` : clean;
}

export function skillDisplayLabel(skill: string) {
  return skill ? humanizeIdentifier(skill, skill) : 'Auto skill';
}

export function userMessageNeedsCollapse(content: string) {
  const normalized = String(content || '');
  return normalized.length > userMessagePreviewChars || normalized.split(/\r?\n/).length > userMessagePreviewLines;
}

export function userMessageVisibleContent(content: string, expanded: boolean) {
  const normalized = String(content || '');
  if (expanded || !userMessageNeedsCollapse(normalized)) return normalized;
  const linePreview = normalized.split(/\r?\n/).slice(0, userMessagePreviewLines).join('\n');
  const preview = linePreview.length > userMessagePreviewChars ? linePreview.slice(0, userMessagePreviewChars) : linePreview;
  return `${preview.trimEnd()}\n...`;
}

export function promptTitle(value: string) {
  const clean = value.replace(/\s+/g, ' ').trim();
  return clean.length > 44 ? `${clean.slice(0, 44)}...` : clean;
}

export function previewJSON(value: unknown) {
  if (value === null || value === undefined || value === '') return '';
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch {
    return String(value);
  }
}

export function boolValue(value: string | undefined) {
  return value === 'true';
}

export function field<T>(...values: Array<T | null | undefined>): T {
  for (const value of values) {
    if (value !== null && value !== undefined) return value;
  }
  return values[values.length - 1] as T;
}

export function asArray<T>(value: T[] | null | undefined): T[] {
  return Array.isArray(value) ? value : [];
}

export function pageSizeFor(key: string) {
  if (key.includes('catalog')) return 10;
  if (key.includes('activity')) return 5;
  if (key.includes('run-groups')) return 5;
  if (key.includes('jobs-list')) return 5;
  if (key.includes('job-runs')) return 5;
  if (key.includes('requests') || key.includes('cache')) return 5;
  return 10;
}

export function pageCount(total: number, size: number) {
  return Math.max(1, Math.ceil(Math.max(0, total) / Math.max(1, size)));
}

export function currentPage(key: string, total: number, size = pageSizeFor(key), pagesState: Record<string, number> = {}) {
  const pages = pageCount(total, size);
  return Math.min(Math.max(1, pagesState[key] || 1), pages);
}

export function pagedItems<T>(items: T[] | null | undefined, key: string, size = pageSizeFor(key), pagesState: Record<string, number> = {}): T[] {
  const list = asArray(items);
  const page = currentPage(key, list.length, size, pagesState);
  return list.slice((page - 1) * size, page * size);
}
