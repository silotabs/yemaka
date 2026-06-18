import { humanizeIdentifier, productStatusLabel, productToolLabel } from './uiHelpers';

export type ToolRunLike = {
  id?: string;
  conversationId?: string;
  sessionId?: string;
  userMessageId?: string;
  assistantMessageId?: string;
  parentMessageId?: string;
  variantIndex?: number;
  toolName?: string;
  input?: unknown;
  output?: unknown;
  status?: string;
  riskLevel?: string;
  createdAt?: string;
  completedAt?: string;
};

export type ToolRunSessionGroup<T extends ToolRunLike> = {
  id: string;
  label: string;
  detail: string;
  runs: T[];
};

export type ToolRunConversationGroup<T extends ToolRunLike> = {
  id: string;
  label: string;
  detail: string;
  runs: T[];
  sessions: Array<ToolRunSessionGroup<T>>;
};

export function compactAnyText(value: unknown, limit = 96) {
  const clean = String(value ?? '').replace(/\s+/g, ' ').trim();
  if (!clean) return '';
  return clean.length > limit ? `${clean.slice(0, Math.max(0, limit - 1))}...` : clean;
}

export function stringField(value: unknown, fallback = '') {
  const clean = String(value ?? '').trim();
  return clean || fallback;
}

export function toolRunField<T extends ToolRunLike>(run: T, camel: keyof T, snake: string) {
  const record = run as unknown as Record<string, unknown>;
  return stringField(record[camel as string] ?? record[snake]);
}

export function toolRunTimestamp<T extends ToolRunLike>(run: T) {
  return stringField(run.createdAt || run.completedAt);
}

export function formatShortTime(value: string | undefined) {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return compactAnyText(value, 22);
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date);
}

export function shortId(value: string | undefined) {
  const clean = String(value || '').trim();
  if (!clean) return '';
  return clean.length > 14 ? `${clean.slice(0, 8)}...${clean.slice(-4)}` : clean;
}

export function toolRunInputLabel<T extends ToolRunLike>(run: T) {
  const input = run.input as Record<string, unknown> | unknown[];
  if (Array.isArray(input)) return compactAnyText(input.join(' '));
  if (input && typeof input === 'object') {
    const record = input as Record<string, unknown>;
    const command = Array.isArray(record.command) ? record.command.join(' ') : record.command;
    return compactAnyText(record.prompt ?? record.query ?? record.goal ?? record.path ?? command ?? record.reason ?? run.toolName);
  }
  return compactAnyText(input || run.toolName);
}

export function toolRunSessionKey<T extends ToolRunLike>(run: T) {
  const sessionID = toolRunField(run, 'sessionId', 'session_id');
  const userMessageID = toolRunField(run, 'userMessageId', 'user_message_id');
  const parentMessageID = toolRunField(run, 'parentMessageId', 'parent_message_id');
  if (sessionID) return `session:${sessionID}`;
  if (userMessageID) return `prompt:${userMessageID}`;
  if (parentMessageID) return `parent:${parentMessageID}`;
  return `ungrouped:${run.conversationId || 'local'}`;
}

export function buildToolRunGroups<T extends ToolRunLike>(runs: T[] | null | undefined, limit = 80): Array<ToolRunConversationGroup<T>> {
  const ordered = Array.isArray(runs) ? runs.slice(0, limit) : [];
  const conversationMap = new Map<string, ToolRunConversationGroup<T>>();

  for (const run of ordered) {
    const conversationID = stringField(run.conversationId, 'local');
    const conversationKey = `conversation:${conversationID}`;
    let conversation = conversationMap.get(conversationKey);
    if (!conversation) {
      conversation = {
        id: conversationKey,
        label: conversationID === 'local' ? 'Local / unscoped' : `Conversation ${shortId(conversationID)}`,
        detail: formatShortTime(toolRunTimestamp(run)),
        runs: [],
        sessions: []
      };
      conversationMap.set(conversationKey, conversation);
    }
    conversation.runs.push(run);

    const sessionKey = `${conversationKey}:${toolRunSessionKey(run)}`;
    let session = conversation.sessions.find((item) => item.id === sessionKey);
    if (!session) {
      const sessionID = toolRunField(run, 'sessionId', 'session_id');
      const promptID = toolRunField(run, 'userMessageId', 'user_message_id');
      session = {
        id: sessionKey,
        label: sessionID ? `Session ${shortId(sessionID)}` : promptID ? `Prompt ${shortId(promptID)}` : 'Ungrouped prompt',
        detail: toolRunInputLabel(run) || formatShortTime(toolRunTimestamp(run)),
        runs: []
      };
      conversation.sessions.push(session);
    }
    session.runs.push(run);
  }

  return Array.from(conversationMap.values());
}

export function expandedByDefault(id: string, total: number, expandedState: Record<string, boolean>, defaultExpanded: boolean) {
  return expandedState[id] ?? (defaultExpanded || total <= 1);
}

export function toggledExpandedState(id: string, expandedState: Record<string, boolean>, current: boolean) {
  return { ...expandedState, [id]: !current };
}

export function formatFileSize(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B';
  if (bytes < 1024) return `${bytes} B`;
  const kib = bytes / 1024;
  if (kib < 1024) return `${kib.toFixed(kib >= 10 ? 0 : 1)} KB`;
  const mib = kib / 1024;
  return `${mib.toFixed(mib >= 10 ? 0 : 1)} MB`;
}

export function formatToolStatus(status: string) {
  return productStatusLabel(status, 'Available');
}

export function formatSurfaces(surfaces: string[] | undefined) {
  return (surfaces ?? []).map((surface) => humanizeIdentifier(surface, surface)).join(', ');
}

export function formatToolName(name: string | undefined) {
  return productToolLabel(name, 'Local action');
}
