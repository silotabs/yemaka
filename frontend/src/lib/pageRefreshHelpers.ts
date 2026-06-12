import type { Tab } from './appOptions';
import type { LiveRefreshReason } from './liveRefresh';

export type RefreshTarget = 'chat' | 'models' | 'automation' | 'learning' | 'extensions' | 'skills' | 'documents' | 'tools' | 'memory' | 'settings' | 'full' | 'none';

export function entryRefreshTargetForTab(tab: Tab): RefreshTarget {
  if (tab === 'chat') return 'chat';
  if (tab === 'models') return 'models';
  if (tab === 'memory') return 'memory';
  if (tab === 'settings') return 'settings';
  if (tab === 'automation') return 'automation';
  if (tab === 'learning') return 'learning';
  if (tab === 'extensions') return 'extensions';
  if (tab === 'skills') return 'skills';
  if (tab === 'documents') return 'documents';
  if (tab === 'tools') return 'tools';
  return 'none';
}

export function manualRefreshTargetForTab(tab: Tab): RefreshTarget {
  const target = entryRefreshTargetForTab(tab);
  return target === 'none' ? 'full' : target;
}

export function liveRefreshTargetForTab(tab: Tab): RefreshTarget {
  if (tab === 'chat') return 'none';
  if (tab === 'models') return 'models';
  if (tab === 'automation') return 'automation';
  if (tab === 'extensions') return 'extensions';
  if (tab === 'learning') return 'learning';
  if (tab === 'skills') return 'skills';
  if (tab === 'documents') return 'documents';
  if (tab === 'tools') return 'tools';
  if (tab === 'memory') return 'memory';
  if (tab === 'settings') return 'settings';
  return 'none';
}

export function shouldRefreshChatOnLive(reason: LiveRefreshReason, activeConversationId: string, hasActiveAssistantRun: boolean) {
  return Boolean(activeConversationId) && (hasActiveAssistantRun || reason !== 'interval');
}
