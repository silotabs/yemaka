import type { AskResult, ChatAttachment, ChatMessage, ConversationDetail, ConversationSession, SendAskOptions } from './appTypes';
import { removeLatestAssistantVariant } from './chatStateHelpers';

export function askContentFromInput(contentOverride: string | Event | undefined, prompt: string) {
  const override = typeof contentOverride === 'string' ? contentOverride : '';
  return (override || prompt).trim();
}

export function askSkillForOptions(options: SendAskOptions, selectedSkill: string) {
  return options.skillOverride ?? selectedSkill;
}

export function shouldAppendAskUser(options: SendAskOptions) {
  return options.appendUser !== false || !options.parentMessageId;
}

export function appendWorkingAskMessages(messages: ChatMessage[], content: string, attachments: ChatAttachment[] = []) {
  const nextMessages: ChatMessage[] = [...messages, { role: 'user', content, attachments }, { role: 'assistant', content: '', meta: 'working' }];
  return {
    messages: nextMessages,
    assistantIndex: nextMessages.length - 1
  };
}

export function markSavedUserMessage(messages: ChatMessage[], streamingMessageIndex: number, userMessageId: string) {
  const userIndex = streamingMessageIndex - 1;
  return messages.map((message, index) => (index === userIndex && message.role === 'user' ? { ...message, id: userMessageId } : message));
}

export function flatUserMessageFromAskResult(content: string, result: AskResult, nowFactory: () => string): ConversationDetail['messages'][number] | null {
  if (!result.userMessageId) return null;
  return {
    id: result.userMessageId,
    role: 'user',
    content,
    model: result.model || '',
    createdAt: nowFactory(),
    attachments: result.attachments ?? [],
    activeVariant: true
  };
}

export function flatAssistantMessageFromAskResult(
  result: AskResult,
  currentAssistant: ChatMessage | undefined,
  nowFactory: () => string
): ConversationDetail['messages'][number] | null {
  if (!result.assistantMessageId) return null;
  return {
    id: result.assistantMessageId,
    role: 'assistant',
    content: result.text,
    model: result.model || currentAssistant?.model || '',
    createdAt: nowFactory(),
    parentId: result.parentMessageId || currentAssistant?.parentId,
    variantIndex: result.variantIndex,
    activeVariant: true
  };
}

export function chatTitleFromConversations(conversations: ConversationSession[], activeConversationId: string, currentTitle: string) {
  return conversations.find((conversation) => conversation.id === activeConversationId)?.title || currentTitle;
}

export function messagesAfterAskError(messages: ChatMessage[], streamingMessageIndex: number, appendUser: boolean) {
  if (appendUser) return messages.slice(0, -1);
  return removeLatestAssistantVariant(messages, streamingMessageIndex);
}

export function streamTransportActivity(useHTTPStream: boolean) {
  return useHTTPStream ? 'stream transport: HTTP SSE' : 'stream transport: desktop events';
}
