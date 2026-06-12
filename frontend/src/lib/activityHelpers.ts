import { humanizeIdentifier } from './uiHelpers';

type ActivityKeyItem = {
  id?: string;
};

export type ActivityEntryLike = {
  id: string;
  kind: string;
  label: string;
  at: string;
};

export type ActivityToolRunLike = {
  id?: string;
  toolName?: string;
  status?: string;
};

export function activityKey(item: ActivityKeyItem, index: number) {
  return `${item.id || 'activity'}-${index}`;
}

export function activityLabel(line: string) {
  return String(line || '').replace(/\s+/g, ' ').trim();
}

export function activityTimeLabel(date = new Date()) {
  return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(date);
}

export function createActivityEntry(line: string, sequence: number, date = new Date()): ActivityEntryLike | null {
  const clean = activityLabel(line);
  if (!clean) return null;
  return {
    id: `${date.getTime()}-${sequence}`,
    kind: activityKind(clean),
    label: clean,
    at: activityTimeLabel(date)
  };
}

export function prependActivityEntry<TEntry extends ActivityEntryLike>(activity: TEntry[] | null | undefined, entry: TEntry, limit = 24) {
  return [entry, ...(activity ?? [])].slice(0, limit);
}

export function activityEntriesFromToolRuns<TRun extends ActivityToolRunLike>(
  toolRuns: TRun[] | null | undefined,
  timestampFor: (run: TRun) => string | undefined,
  formatTime: (value: string | undefined) => string,
  limit = 24
) {
  return (toolRuns ?? []).slice(0, limit).map((run, index) => {
    const label = `${humanizeIdentifier(run.toolName, 'Tool')}: ${humanizeIdentifier(run.status || 'logged', 'Logged')}`;
    return {
      id: `persisted-${run.id || index}`,
      kind: activityKind(label),
      label,
      at: formatTime(timestampFor(run)) || 'persisted'
    };
  });
}

export function shouldHydrateActivity(activity: ActivityEntryLike[] | null | undefined, toolRuns: ActivityToolRunLike[] | null | undefined) {
  return !activity?.length && Boolean(toolRuns?.length);
}

export function activityKind(line: string) {
  const value = line.toLowerCase();
  if (value.startsWith('permission')) return 'permission';
  if (value.includes('verification')) return 'verification';
  if (value.includes('tool') || value.includes('executor') || value.includes('file')) return 'tool';
  if (value.includes('model')) return 'model';
  if (value.includes('memory')) return 'memory';
  if (value.includes('document') || value.includes('workspace') || value.includes('ingested')) return 'documents';
  if (value.includes('internet')) return 'internet';
  if (value.includes('skill')) return 'skills';
  if (value.includes('extension')) return 'extensions';
  if (value.includes('job') || value.includes('heartbeat') || value.includes('task:') || value.includes('plan:')) return 'automation';
  if (value.includes('settings') || value.includes('setup') || value.includes('policy')) return 'settings';
  if (value.includes('ask completed') || value.includes('conversation') || value.includes('generation')) return 'chat';
  return 'system';
}

export function activityIcon(kind: string) {
  if (kind === 'permission' || kind === 'verification') return 'health';
  if (kind === 'tool') return 'tools';
  if (kind === 'model') return 'models';
  if (kind === 'memory') return 'memory';
  if (kind === 'documents') return 'documents';
  if (kind === 'internet') return 'search';
  if (kind === 'skills') return 'skills';
  if (kind === 'extensions') return 'extensions';
  if (kind === 'automation') return 'automation';
  if (kind === 'settings') return 'settings';
  return 'chat';
}
