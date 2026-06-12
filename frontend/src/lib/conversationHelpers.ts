import { readPersistedMessageHandoffs } from './handoffPersistence';
import type { ChatAttachment } from './appTypes';

export type ConversationMessageRole = 'user' | 'assistant' | 'system';

export type VariantMessage<T extends VariantMessage<T>> = {
  id?: string;
  role: ConversationMessageRole;
  content?: string;
  meta?: string;
  model?: string;
  sourceKind?: string;
  sources?: string[];
  attachments?: ChatAttachment[];
  trace?: string[];
  parentId?: string;
  variantIndex?: number;
  activeVariant?: boolean;
  variants?: T[];
  activeVariantIndex?: number;
};

export type ConversationRawMessage = {
  id: string;
  role: ConversationMessageRole;
  content: string;
  meta?: string;
  model: string;
  createdAt: string;
  parentId?: string;
  variantIndex?: number;
  activeVariant?: boolean;
  trace?: string[];
  sources?: string[];
  sourceKind?: string;
  attachments?: ChatAttachment[];
};

export function variantSnapshotForRole<T extends VariantMessage<T>>(message: T, role: T['role']): T {
  const { variants, activeVariantIndex, ...snapshot } = message;
  return { ...snapshot, role } as T;
}

export function variantSnapshot<T extends VariantMessage<T>>(message: T): T {
  return variantSnapshotForRole(message, 'assistant' as T['role']);
}

export function userVariantSnapshot<T extends VariantMessage<T>>(message: T): T {
  return variantSnapshotForRole(message, 'user' as T['role']);
}

export function assistantVariants<T extends VariantMessage<T>>(message: T): T[] {
  if (message.variants?.length) return message.variants.map((variant) => variantSnapshot(variant));
  return [variantSnapshot(message)];
}

export function userVariants<T extends VariantMessage<T>>(message: T): T[] {
  if (message.variants?.length) return message.variants.map((variant) => userVariantSnapshot(variant));
  return [userVariantSnapshot(message)];
}

export function activateAssistantVariant<T extends VariantMessage<T>>(message: T, variants: T[], activeIndex: number): T {
  const safeIndex = Math.min(Math.max(activeIndex, 0), Math.max(variants.length - 1, 0));
  const normalized = variants.map((variant, index) => ({ ...variant, activeVariant: index === safeIndex })) as T[];
  const selected = normalized[safeIndex] || variantSnapshot(message);
  return {
    ...message,
    ...selected,
    role: 'assistant' as T['role'],
    variants: normalized,
    activeVariantIndex: safeIndex
  };
}

export function syncActiveAssistantVariant<T extends VariantMessage<T>>(message: T): T {
  if (message.role !== 'assistant' || !message.variants?.length) return message;
  const activeIndex = Math.min(Math.max(message.activeVariantIndex ?? 0, 0), message.variants.length - 1);
  const variants = message.variants.map((variant, index) => ({
    ...(index === activeIndex ? variantSnapshot(message) : variant),
    activeVariant: index === activeIndex
  })) as T[];
  return { ...message, variants, activeVariantIndex: activeIndex };
}

export function activateUserVariant<T extends VariantMessage<T>>(message: T, variants: T[], activeIndex: number): T {
  const safeIndex = Math.min(Math.max(activeIndex, 0), Math.max(variants.length - 1, 0));
  const normalized = variants.map((variant, index) => ({ ...variant, activeVariant: index === safeIndex })) as T[];
  const selected = normalized[safeIndex] || userVariantSnapshot(message);
  return {
    ...message,
    ...selected,
    role: 'user' as T['role'],
    variants: normalized,
    activeVariantIndex: safeIndex
  };
}

export function userVariantRootKey<T extends VariantMessage<T>>(message: T) {
  if (message.role !== 'user') return '';
  return message.parentId || message.id || '';
}

export function userVariantCount<T extends VariantMessage<T>>(message: T) {
  return message.role === 'user' ? userVariants(message).length : 0;
}

export function userVariantPosition<T extends VariantMessage<T>>(message: T) {
  return (message.activeVariantIndex ?? 0) + 1;
}

export function assistantVariantCount<T extends VariantMessage<T>>(message: T) {
  return message.role === 'assistant' ? assistantVariants(message).length : 0;
}

export function assistantVariantPosition<T extends VariantMessage<T>>(message: T) {
  return (message.activeVariantIndex ?? 0) + 1;
}

export function conversationDisplayPrefersPreviousUser<T extends VariantMessage<T>>(items: T[]) {
  const firstUser = items.findIndex((item) => item.role === 'user');
  const firstAssistant = items.findIndex((item) => item.role === 'assistant');
  return firstUser >= 0 && (firstAssistant < 0 || firstUser < firstAssistant);
}

export function scanConversationUserKey<T extends VariantMessage<T>>(items: T[], start: number, stop: number, step: number) {
  for (let cursor = start; cursor !== stop; cursor += step) {
    if (cursor < 0 || cursor >= items.length) return '';
    const item = items[cursor];
    if (item.role === 'user' && item.activeVariant !== false) return item.id ? `user:${item.id}` : `user-position:${cursor}`;
  }
  return '';
}

export function inferredConversationParentKey<T extends VariantMessage<T>>(items: T[], index: number, preferPreviousUser: boolean) {
  const message = items[index];
  if (message.parentId) return `user:${message.parentId}`;
  if (message.role !== 'assistant') return '';
  if (preferPreviousUser) {
    return scanConversationUserKey(items, index - 1, -1, -1) || scanConversationUserKey(items, index + 1, items.length, 1);
  }
  return scanConversationUserKey(items, index + 1, items.length, 1) || scanConversationUserKey(items, index - 1, -1, -1);
}

export function orderedAssistantVariants<T extends VariantMessage<T>>(variants: T[]) {
  return variants
    .map((variant, index) => ({ variant, index }))
    .sort((left, right) => {
      const leftRank = left.variant.variantIndex ?? 0;
      const rightRank = right.variant.variantIndex ?? 0;
      if (leftRank !== rightRank) return leftRank - rightRank;
      return left.index - right.index;
    })
    .map((item) => item.variant);
}

export function activeAssistantVariantIndex<T extends VariantMessage<T>>(variants: T[], fallback = 0) {
  let activeIndex = -1;
  let activeRank = -1;
  variants.forEach((variant, index) => {
    if (!variant.activeVariant) return;
    const rank = variant.variantIndex ?? index;
    if (rank >= activeRank) {
      activeRank = rank;
      activeIndex = index;
    }
  });
  if (activeIndex >= 0) return activeIndex;
  return Math.min(Math.max(fallback, 0), Math.max(variants.length - 1, 0));
}

export function promptVariantRootKey<T extends VariantMessage<T>>(message: T) {
  if (message.role !== 'user') return '';
  return message.parentId || message.id || '';
}

export function groupConversationMessages<T extends VariantMessage<T> & { content: string }>(
  flatMessages: ConversationRawMessage[] | null | undefined,
  promptVariantOverrides: Record<string, string> = {}
): T[] {
  const normalized = (Array.isArray(flatMessages) ? flatMessages : []).map((raw) => ({
    id: raw.id,
    role: raw.role,
    content: raw.content,
    meta: raw.meta || undefined,
    model: raw.model || undefined,
    parentId: raw.parentId || undefined,
    variantIndex: raw.variantIndex ?? undefined,
    activeVariant: raw.activeVariant,
    trace: Array.isArray(raw.trace) ? raw.trace : [],
    sources: Array.isArray(raw.sources) ? raw.sources : [],
    sourceKind: raw.sourceKind || undefined,
    attachments: Array.isArray(raw.attachments) ? raw.attachments : [],
    ...(raw.role === 'assistant' ? readPersistedMessageHandoffs(raw) : {})
  })) as T[];
  const rawIndexByID = new Map<string, number>();
  normalized.forEach((message, index) => {
    if (message.id) rawIndexByID.set(message.id, index);
  });
  const userGroups = new Map<string, T[]>();
  for (const message of normalized) {
    if (message.role !== 'user') continue;
    const rootKey = promptVariantRootKey(message);
    if (!rootKey) continue;
    userGroups.set(rootKey, [...(userGroups.get(rootKey) ?? []), userVariantSnapshot(message)]);
  }
  const userGroupInfo = new Map<string, { variants: T[]; activeIndex: number; activeID: string }>();
  const hiddenUserIDs = new Set<string>();
  const hiddenRawIndexes = new Set<number>();
  for (const [rootKey, group] of userGroups.entries()) {
    const variants = orderedAssistantVariants(group);
    const overrideID = promptVariantOverrides[rootKey];
    const overrideIndex = overrideID ? variants.findIndex((variant) => variant.id === overrideID) : -1;
    const activeIndex = overrideIndex >= 0 ? overrideIndex : activeAssistantVariantIndex(variants, variants.length - 1);
    const activeVariant = variants[activeIndex] ?? variants[variants.length - 1];
    const activeID = activeVariant?.id || '';
    userGroupInfo.set(rootKey, { variants, activeIndex, activeID });
    const variantRawIndexes = variants
      .map((variant) => (variant.id ? rawIndexByID.get(variant.id) : undefined))
      .filter((index): index is number => index !== undefined);
    const rootRawIndex = variantRawIndexes.length ? Math.min(...variantRawIndexes) : -1;
    const activeRawIndex = activeID ? rawIndexByID.get(activeID) ?? rootRawIndex : rootRawIndex;
    if (rootRawIndex >= 0 && activeRawIndex > rootRawIndex) {
      for (let cursor = rootRawIndex + 1; cursor <= activeRawIndex; cursor += 1) {
        hiddenRawIndexes.add(cursor);
      }
    }
    for (const variant of variants) {
      if (!variant.id) continue;
      if (variant.id !== activeID) {
        hiddenUserIDs.add(variant.id);
        const variantRawIndex = rawIndexByID.get(variant.id);
        if (variantRawIndex !== undefined && activeRawIndex >= 0 && variantRawIndex > activeRawIndex) {
          for (let cursor = variantRawIndex; cursor < normalized.length; cursor += 1) {
            hiddenRawIndexes.add(cursor);
          }
        }
      }
    }
  }
  const grouped: T[] = [];
  const assistantByParent = new Map<string, number>();
  const displayedUserGroups = new Set<string>();
  const preferPreviousUser = conversationDisplayPrefersPreviousUser(normalized);
  for (let index = 0; index < normalized.length; index += 1) {
    if (hiddenRawIndexes.has(index)) continue;
    const message = normalized[index];
    if (message.role === 'user') {
      const rootKey = promptVariantRootKey(message);
      const info = rootKey ? userGroupInfo.get(rootKey) : undefined;
      if (!rootKey || !info || displayedUserGroups.has(rootKey)) continue;
      displayedUserGroups.add(rootKey);
      grouped.push(activateUserVariant(info.variants[0] ?? message, info.variants, info.activeIndex));
      continue;
    }
    if (message.role === 'assistant' && message.parentId && hiddenUserIDs.has(message.parentId)) {
      continue;
    }
    const parentKey = inferredConversationParentKey(normalized, index, preferPreviousUser);
    if (message.role !== 'assistant' || !parentKey) {
      grouped.push(message);
      continue;
    }
    const existingIndex = assistantByParent.get(parentKey);
    if (existingIndex === undefined) {
      const variants = orderedAssistantVariants([variantSnapshot(message)]);
      const activeVariantIndex = activeAssistantVariantIndex(variants, 0);
      grouped.push({ ...message, variants, activeVariantIndex });
      assistantByParent.set(parentKey, grouped.length - 1);
      continue;
    }
    const existing = grouped[existingIndex];
    const variants = orderedAssistantVariants([...assistantVariants(existing), variantSnapshot(message)]);
    const activeIndex = activeAssistantVariantIndex(variants, existing.activeVariantIndex ?? 0);
    grouped[existingIndex] = activateAssistantVariant(existing, variants, activeIndex);
  }
  return grouped;
}

export function rawConversationMessage<T extends VariantMessage<T> & { content: string }>(
  message: T,
  fallbackId: string,
  createdAt: string
): ConversationRawMessage {
  return {
    id: message.id || fallbackId,
    role: message.role,
    content: message.content,
    meta: message.meta,
    model: message.model || '',
    createdAt,
    parentId: message.parentId,
    variantIndex: message.variantIndex,
    activeVariant: message.activeVariant,
    trace: message.trace,
    sources: message.sources,
    sourceKind: message.sourceKind,
    attachments: Array.isArray(message.attachments) ? message.attachments : []
  };
}

export function displayedMessagesAsFlatMessages<T extends VariantMessage<T> & { content: string }>(
  messages: T[],
  idFactory: () => string,
  nowFactory: () => string
): ConversationRawMessage[] {
  const flat: ConversationRawMessage[] = [];
  for (const message of messages) {
    const variants = message.variants?.length ? message.variants : [message];
    for (const variant of variants) {
      if (!variant.id) continue;
      flat.push(rawConversationMessage({ ...variant, role: message.role }, idFactory(), nowFactory()));
    }
  }
  return flat;
}

export function ensureDisplayedFlatMessages<T extends VariantMessage<T> & { content: string }>(
  flatMessages: ConversationRawMessage[] | null | undefined,
  messages: T[],
  idFactory: () => string,
  nowFactory: () => string
): ConversationRawMessage[] {
  if (Array.isArray(flatMessages) && flatMessages.length) return flatMessages;
  if (!messages.length) return Array.isArray(flatMessages) ? flatMessages : [];
  return displayedMessagesAsFlatMessages(messages, idFactory, nowFactory);
}

export function upsertFlatMessage(
  flatMessages: ConversationRawMessage[] | null | undefined,
  message: ConversationRawMessage
): ConversationRawMessage[] {
  const next = Array.isArray(flatMessages) ? [...flatMessages] : [];
  if (!message.id) return next;
  const index = next.findIndex((item) => item.id === message.id);
  if (index >= 0) {
    next[index] = { ...next[index], ...message };
    return next;
  }
  return [...next, message];
}
