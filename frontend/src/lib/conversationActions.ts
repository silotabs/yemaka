import type { ConversationDetail, ConversationSession } from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';

export async function listConversations(limit = 80) {
  return asArray(await call<ConversationSession[]>('ListConversations', limit));
}

export async function editConversationUserMessage(messageId: string, content: string) {
  return await call<ConversationDetail['messages'][number]>('EditUserMessage', messageId, content);
}

export async function renameConversation(conversationId: string, title: string) {
  return await call<ConversationSession>('RenameConversation', conversationId, title);
}

export async function setConversationStarredState(conversationId: string, starred: boolean) {
  return await call<ConversationSession>('SetConversationStarred', conversationId, starred);
}

export async function getConversationDetail(conversationId: string) {
  return await call<ConversationDetail>('GetConversation', conversationId);
}

export async function deleteConversation(conversationId: string) {
  return await call<{ status: string }>('DeleteConversation', conversationId);
}
