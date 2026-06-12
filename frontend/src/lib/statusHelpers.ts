export type InternetStatusLike = {
  enabled?: boolean;
  searchEnabled?: boolean;
  searchProviderReady?: boolean;
  searchProviderNeedsAuth?: boolean;
  searchProviderNeedsConfig?: boolean;
  searchStatus?: string;
  searchState?: string;
};

export function healthBadge(statusValue = '') {
  const value = statusValue.toLowerCase();
  if (value === 'ok' || value === 'healthy' || value === 'ready') return 'bg-emerald-50 text-emerald-700';
  if (value === 'warn' || value === 'warning' || value === 'needs_config') return 'bg-amber-50 text-amber-800';
  if (value === 'error' || value === 'broken') return 'bg-rose-50 text-rose-700';
  if (value === 'needs_auth') return 'bg-sky-50 text-sky-700';
  if (value === 'disabled') return 'bg-slate-100 text-slate-700';
  return 'bg-slate-100 text-slate-700';
}

export function internetSearchReadiness(statusValue: InternetStatusLike | null) {
  if (statusValue?.searchState) return statusValue.searchState;
  if (!statusValue?.enabled || !statusValue.searchEnabled) return 'disabled';
  if (statusValue.searchProviderReady) return 'healthy';
  if (statusValue.searchProviderNeedsAuth) return 'needs_auth';
  if (statusValue.searchProviderNeedsConfig) return 'needs_config';
  const detail = (statusValue.searchStatus || '').toLowerCase();
  if (detail.includes('api key') || detail.includes('auth') || detail.includes('environment variable')) return 'needs_auth';
  return 'needs_config';
}
