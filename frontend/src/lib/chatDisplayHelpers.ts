import type { ChatMessage, InternetStatus, SettingsView, Status } from './appTypes';
import { compactText, isAssistantActiveMeta, sourceKindLabel } from './uiHelpers';

export type ChatContextItem = {
  icon: string;
  label: string;
  tone: string;
};

export type ChatContextOptions = {
  status: Status | null;
  settings: SettingsView | null;
  ragEnabled: boolean;
  internetEnabled: boolean;
  internetStatus: InternetStatus | null;
};

export function assistantMetaItems(message: ChatMessage) {
  const items: Array<{ label: string; icon: string }> = [];
  if (message.model) items.push({ label: message.model, icon: 'models' });
  if (message.skill) items.push({ label: message.skill, icon: 'skills' });
  const source = sourceKindLabel(message.sourceKind);
  if (source) items.push({ label: source, icon: source === 'internet' ? 'search' : 'documents' });
  if (message.sources?.length) items.push({ label: `${message.sources.length} source${message.sources.length === 1 ? '' : 's'}`, icon: 'documents' });
  if (message.meta && !isAssistantActiveMeta(message.meta)) items.push({ label: message.meta, icon: 'system' });
  return items;
}

export function chatContextItems(options: ChatContextOptions): ChatContextItem[] {
  const { status, settings, ragEnabled, internetEnabled, internetStatus } = options;
  const modelLabel = status?.modelReady
    ? compactText(status.selectedModel || status.defaultModel, 46)
    : status?.selectedModel || status?.defaultModel
      ? compactText(status.selectedModel || status.defaultModel, 46)
      : 'model not ready';
  const modelReady = Boolean(status?.modelReady || status?.selectedModel || status?.defaultModel);
  const ragOn = Boolean(ragEnabled || status?.ragEnabled);
  const toolsOn = Boolean(settings?.shellEnabled ?? status?.shellEnabled);
  const internetOn = Boolean(internetEnabled || internetStatus?.enabled);
  return [
    {
      icon: 'models',
      label: modelLabel,
      tone: modelReady ? 'ok' : 'warn'
    },
    {
      icon: 'settings',
      label: status?.lowMemoryMode ? 'low memory' : 'standard',
      tone: 'neutral'
    },
    {
      icon: 'documents',
      label: ragOn ? 'RAG on' : 'RAG off',
      tone: ragOn ? 'ok' : 'muted'
    },
    {
      icon: 'tools',
      label: toolsOn ? 'tools on' : 'tools off',
      tone: toolsOn ? 'ok' : 'muted'
    },
    {
      icon: 'search',
      label: internetOn ? 'internet on' : 'internet off',
      tone: internetOn ? 'warn' : 'muted'
    }
  ];
}
