import type { ChatMessage, ConversationDetail, QueuedFollowUpPrompt, SendAskOptions } from './appTypes';
import {
  activateAssistantVariant,
  activateUserVariant,
  assistantVariants,
  groupConversationMessages,
  syncActiveAssistantVariant,
  userVariantRootKey,
  userVariants,
  variantSnapshot
} from './conversationHelpers';
import { isAssistantActiveMeta } from './uiHelpers';

export type RetryContext = {
  promptIndex: number;
  assistantIndex: number;
  content: string;
  parentMessageId: string;
};

export function assistantIndexForPrompt(messages: ChatMessage[], promptIndex: number) {
  for (let cursor = promptIndex + 1; cursor < messages.length; cursor += 1) {
    const candidate = messages[cursor];
    if (candidate?.role === 'user') return -1;
    if (candidate?.role === 'assistant') return cursor;
  }
  return -1;
}

export function retryPromptForMessage(messages: ChatMessage[], index: number) {
  const message = messages[index];
  if (!message) return '';
  if (message.role === 'user') return message.content;
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    if (messages[cursor]?.role === 'user') return messages[cursor].content;
  }
  return '';
}

export function retryContextForMessage(messages: ChatMessage[], index: number): RetryContext | null {
  const message = messages[index];
  if (!message) return null;
  if (message.role === 'user') {
    const assistantIndex = assistantIndexForPrompt(messages, index);
    return { promptIndex: index, assistantIndex, content: message.content, parentMessageId: message.id || '' };
  }
  for (let cursor = index - 1; cursor >= 0; cursor -= 1) {
    if (messages[cursor]?.role === 'user') {
      return { promptIndex: cursor, assistantIndex: index, content: messages[cursor].content, parentMessageId: messages[cursor].id || message.parentId || '' };
    }
  }
  return null;
}

export function hasActiveAssistantRun(messages: ChatMessage[]) {
  return messages.some((message) => message.role === 'assistant' && isAssistantActiveMeta(message.meta));
}

export function streamTargetIndexFor(messages: ChatMessage[], streamingMessageIndex: number) {
  if (streamingMessageIndex >= 0 && messages[streamingMessageIndex]?.role === 'assistant') {
    return streamingMessageIndex;
  }
  return messages.length - 1;
}

export function updateAssistantMessageList(messages: ChatMessage[], index: number, updater: (message: ChatMessage) => ChatMessage) {
  return messages.map((message, cursor) => {
    if (cursor !== index || message.role !== 'assistant') return message;
    const next = updater(message);
    const withVariants =
      message.variants?.length && !next.variants
        ? { ...next, variants: message.variants, activeVariantIndex: message.activeVariantIndex }
        : next;
    return syncActiveAssistantVariant(withVariants);
  });
}

export function switchAssistantVariantList(messages: ChatMessage[], index: number, direction: number) {
  const message = messages[index];
  if (!message || message.role !== 'assistant') return messages;
  const variants = assistantVariants(message);
  if (variants.length < 2) return messages;
  const current = message.activeVariantIndex ?? variants.findIndex((variant) => variant.activeVariant);
  const next = Math.min(Math.max((current >= 0 ? current : 0) + direction, 0), variants.length - 1);
  return messages.map((item, cursor) => (cursor === index ? activateAssistantVariant(message, variants, next) : item));
}

export type UserVariantSwitchResult = {
  messages: ChatMessage[];
  promptVariantOverrides: Record<string, string>;
};

export function switchUserVariantList(
  messages: ChatMessage[],
  conversationFlatMessages: ConversationDetail['messages'],
  promptVariantOverrides: Record<string, string>,
  index: number,
  direction: number
): UserVariantSwitchResult {
  const message = messages[index];
  if (!message || message.role !== 'user') return { messages, promptVariantOverrides };
  const variants = userVariants(message);
  if (variants.length < 2) return { messages, promptVariantOverrides };
  const current = message.activeVariantIndex ?? variants.findIndex((variant) => variant.activeVariant);
  const next = Math.min(Math.max((current >= 0 ? current : 0) + direction, 0), variants.length - 1);
  const selected = variants[next];
  const rootKey = userVariantRootKey(message);
  if (rootKey && selected?.id) {
    const nextOverrides = { ...promptVariantOverrides, [rootKey]: selected.id };
    if (conversationFlatMessages.length) {
      return {
        messages: groupConversationMessages(conversationFlatMessages, nextOverrides),
        promptVariantOverrides: nextOverrides
      };
    }
    return {
      messages: messages.map((item, cursor) => (cursor === index ? activateUserVariant(message, variants, next) : item)),
      promptVariantOverrides: nextOverrides
    };
  }
  return {
    messages: messages.map((item, cursor) => (cursor === index ? activateUserVariant(message, variants, next) : item)),
    promptVariantOverrides
  };
}

export function chatMessageSnapshot(messages: ChatMessage[]) {
  return messages
    .map((message) => `${message.role}:${message.content.length}:${message.meta || ''}:${message.trace?.length || 0}:${message.sources?.length || 0}:${message.activeVariantIndex ?? 0}:${message.variants?.length ?? 0}:${message.capabilityHandoff?.status || ''}:${message.capabilityHandoff?.generatedName || ''}:${message.schedulerHandoff?.status || ''}:${message.schedulerHandoff?.job?.id || ''}`)
    .join('|');
}

export function prepareRetryAssistantState(messages: ChatMessage[], options: SendAskOptions, parentMessageId: string): { messages: ChatMessage[]; assistantIndex: number } {
  let assistantIndex = options.assistantIndex ?? -1;
  const placeholder: ChatMessage = {
    id: undefined,
    role: 'assistant',
    content: '',
    meta: 'working',
    model: undefined,
    skill: undefined,
    sourceKind: undefined,
    sources: [],
    trace: [],
    parentId: parentMessageId,
    variantIndex: 0,
    activeVariant: true,
    capabilityHandoff: undefined,
    schedulerHandoff: undefined
  };
  if (assistantIndex >= 0 && messages[assistantIndex]?.role === 'assistant') {
    const assistant = messages[assistantIndex];
    const variants = [...assistantVariants(assistant), variantSnapshot(placeholder)];
    return {
      messages: messages.map((message, index) => (index === assistantIndex ? activateAssistantVariant(assistant, variants, variants.length - 1) : message)),
      assistantIndex
    };
  }
  const insertAt = Math.max(options.promptIndex ?? messages.length - 1, -1) + 1;
  return {
    messages: [...messages.slice(0, insertAt), placeholder, ...messages.slice(insertAt)],
    assistantIndex: insertAt
  };
}

export function removeLatestAssistantVariant(messages: ChatMessage[], index: number) {
  return messages.map((message, cursor) => {
    if (cursor !== index || message.role !== 'assistant') return message;
    const variants = assistantVariants(message).slice(0, -1);
    return variants.length ? activateAssistantVariant(message, variants, variants.length - 1) : message;
  });
}

export function queuedFollowUpId(now: number, sequence: number) {
  return `followup_${now.toString(36)}_${sequence}`;
}

export function createQueuedFollowUp(content: string, skill: string, id: string, createdAt: number): QueuedFollowUpPrompt {
  const clean = content.trim();
  return {
    id,
    content: clean,
    draft: clean,
    skill,
    createdAt,
    editing: false
  };
}

export function startQueuedFollowUpEdit(items: QueuedFollowUpPrompt[], id: string) {
  return items.map((item) => (item.id === id ? { ...item, draft: item.content, editing: true } : item));
}

export function updateQueuedFollowUpDraft(items: QueuedFollowUpPrompt[], id: string, draft: string) {
  return items.map((item) => (item.id === id ? { ...item, draft } : item));
}

export function saveQueuedFollowUpDraft(items: QueuedFollowUpPrompt[], id: string) {
  return items.flatMap((item) => {
    if (item.id !== id) return [item];
    const content = item.draft.trim();
    if (!content) return [];
    return [{ ...item, content, draft: content, editing: false }];
  });
}

export function cancelQueuedFollowUpEdit(items: QueuedFollowUpPrompt[], id: string) {
  return items.map((item) => (item.id === id ? { ...item, draft: item.content, editing: false } : item));
}

export function deleteQueuedFollowUpItem(items: QueuedFollowUpPrompt[], id: string) {
  return items.filter((item) => item.id !== id);
}
