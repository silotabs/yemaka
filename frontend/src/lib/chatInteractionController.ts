import { tick } from 'svelte';
import type {
  ChatMessage,
  ConversationDetail,
  QueuedFollowUpPrompt,
  SendAskOptions
} from './appTypes';
import { assistantMetaItems as assistantMetaItemsFor } from './chatDisplayHelpers';
import {
  cancelQueuedFollowUpEdit,
  chatMessageSnapshot,
  createQueuedFollowUp,
  deleteQueuedFollowUpItem,
  prepareRetryAssistantState,
  queuedFollowUpId,
  retryContextForMessage as retryContextForMessageFor,
  retryPromptForMessage as retryPromptForMessageFor,
  saveQueuedFollowUpDraft,
  streamTargetIndexFor,
  startQueuedFollowUpEdit,
  switchAssistantVariantList,
  switchUserVariantList,
  updateAssistantMessageList,
  updateQueuedFollowUpDraft as updateQueuedFollowUpDraftFor
} from './chatStateHelpers';
import {
  ensureDisplayedFlatMessages,
  groupConversationMessages,
  upsertFlatMessage,
  userVariantCount,
  userVariants
} from './conversationHelpers';
import { editConversationUserMessage } from './conversationActions';

export type ChatInteractionControllerContext = {
  getMessages: () => ChatMessage[];
  setMessages: (messages: ChatMessage[]) => void;
  getConversationFlatMessages: () => ConversationDetail['messages'];
  setConversationFlatMessages: (messages: ConversationDetail['messages']) => void;
  getPromptVariantOverrides: () => Record<string, string>;
  setPromptVariantOverrides: (overrides: Record<string, string>) => void;
  getStreamingMessageIndex: () => number;
  setStreamingMessageIndex: (index: number) => void;
  getActiveTab: () => string;
  getBusy: () => boolean;
  getChatScroll: () => HTMLDivElement | null;
  getKeepChatPinned: () => boolean;
  setKeepChatPinned: (value: boolean) => void;
  getMessageSnapshot: () => string;
  setMessageSnapshot: (snapshot: string) => void;
  getExpandedMessageIndexes: () => Record<number, boolean>;
  setExpandedMessageIndexes: (items: Record<number, boolean>) => void;
  setEditingUserMessageIndex: (index: number) => void;
  getEditingUserPrompt: () => string;
  setEditingUserPrompt: (prompt: string) => void;
  getEditingUserBusy: () => boolean;
  setEditingUserBusy: (busy: boolean) => void;
  setCopiedMessageIndex: (index: number | null) => void;
  getCopyMessageResetTimer: () => ReturnType<typeof setTimeout> | undefined;
  setCopyMessageResetTimer: (timer: ReturnType<typeof setTimeout> | undefined) => void;
  getQueuedFollowUps: () => QueuedFollowUpPrompt[];
  setQueuedFollowUps: (items: QueuedFollowUpPrompt[]) => void;
  getQueuedFollowUpSequence: () => number;
  setQueuedFollowUpSequence: (sequence: number) => void;
  getProcessingQueuedFollowUp: () => boolean;
  setProcessingQueuedFollowUp: (processing: boolean) => void;
  getPrompt: () => string;
  setPrompt: (prompt: string) => void;
  getSelectedSkill: () => string;
  getComposerFocused: () => boolean;
  setComposerFocused: (focused: boolean) => void;
  getComposerTextarea: () => HTMLTextAreaElement | null;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  sendAsk: (contentOverride?: string | Event, options?: SendAskOptions) => Promise<boolean | void>;
};

export function createChatInteractionController(ctx: ChatInteractionControllerContext) {
  function streamTargetIndex() {
    return streamTargetIndexFor(ctx.getMessages(), ctx.getStreamingMessageIndex());
  }

  function updateAssistantMessageAt(index: number, updater: (message: ChatMessage) => ChatMessage) {
    ctx.setMessages(updateAssistantMessageList(ctx.getMessages(), index, updater));
  }

  function switchAssistantVariant(index: number, direction: number) {
    ctx.setMessages(switchAssistantVariantList(ctx.getMessages(), index, direction));
  }

  function switchUserVariant(index: number, direction: number) {
    const result = switchUserVariantList(
      ctx.getMessages(),
      ctx.getConversationFlatMessages(),
      ctx.getPromptVariantOverrides(),
      index,
      direction
    );
    ctx.setMessages(result.messages);
    ctx.setPromptVariantOverrides(result.promptVariantOverrides);
  }

  function handleChatScroll() {
    const chatScroll = ctx.getChatScroll();
    if (!chatScroll) return;
    const distanceFromBottom = chatScroll.scrollHeight - chatScroll.scrollTop - chatScroll.clientHeight;
    ctx.setKeepChatPinned(distanceFromBottom < 160);
  }

  async function scrollChatToBottom(force = false, behavior?: ScrollBehavior) {
    const chatScroll = ctx.getChatScroll();
    if (ctx.getActiveTab() !== 'chat' || !chatScroll) return;
    if (!force && !ctx.getKeepChatPinned()) return;
    await tick();
    chatScroll.scrollTo({
      top: chatScroll.scrollHeight,
      behavior: behavior ?? (force || ctx.getBusy() ? 'auto' : 'smooth')
    });
    if (force) {
      ctx.setKeepChatPinned(true);
    }
  }

  function syncScrollForMessages() {
    const nextSnapshot = chatMessageSnapshot(ctx.getMessages());
    if (nextSnapshot !== ctx.getMessageSnapshot()) {
      ctx.setMessageSnapshot(nextSnapshot);
      void scrollChatToBottom();
    }
  }

  function assistantMetaItems(message: ChatMessage) {
    return assistantMetaItemsFor(message);
  }

  function toggleUserMessageExpanded(index: number) {
    ctx.setExpandedMessageIndexes({
      ...ctx.getExpandedMessageIndexes(),
      [index]: !ctx.getExpandedMessageIndexes()[index]
    });
  }

  function resetUserMessageEdit() {
    ctx.setEditingUserMessageIndex(-1);
    ctx.setEditingUserPrompt('');
    ctx.setEditingUserBusy(false);
  }

  function startEditUserMessage(index: number) {
    const message = ctx.getMessages()[index];
    if (!message || message.role !== 'user' || !message.id || ctx.getBusy()) return;
    ctx.setEditingUserMessageIndex(index);
    ctx.setEditingUserPrompt(message.content);
    ctx.setExpandedMessageIndexes({ ...ctx.getExpandedMessageIndexes(), [index]: true });
  }

  function cancelEditUserMessage() {
    resetUserMessageEdit();
  }

  function ensureFlatConversationMessages() {
    ctx.setConversationFlatMessages(
      ensureDisplayedFlatMessages(
        ctx.getConversationFlatMessages(),
        ctx.getMessages(),
        () => `local_${Date.now()}_${Math.random().toString(16).slice(2)}`,
        () => new Date().toISOString()
      )
    );
  }

  function upsertFlatConversationMessage(message: ConversationDetail['messages'][number]) {
    ctx.setConversationFlatMessages(upsertFlatMessage(ctx.getConversationFlatMessages(), message));
  }

  async function submitEditedUserMessage(index: number) {
    const message = ctx.getMessages()[index];
    const content = ctx.getEditingUserPrompt().trim();
    if (!message || message.role !== 'user' || !message.id || !content || ctx.getBusy() || ctx.getEditingUserBusy()) return;
    ctx.setEditingUserBusy(true);
    ctx.setError('');
    try {
      const updated = await editConversationUserMessage(message.id, content);
      ensureFlatConversationMessages();
      const flatMessage = {
        id: updated.id || message.id,
        role: 'user' as const,
        content: updated.content || content,
        model: updated.model || message.model || '',
        createdAt: updated.createdAt || new Date().toISOString(),
        parentId: updated.parentId || message.parentId || message.id,
        variantIndex: updated.variantIndex ?? userVariantCount(message),
        activeVariant: updated.activeVariant ?? true
      };
      upsertFlatConversationMessage(flatMessage);
      const rootKey = updated.parentId || message.parentId || message.id || '';
      if (rootKey && updated.id) {
        ctx.setPromptVariantOverrides({ ...ctx.getPromptVariantOverrides(), [rootKey]: updated.id });
      }
      ctx.setMessages(groupConversationMessages(ctx.getConversationFlatMessages(), ctx.getPromptVariantOverrides()));
      const promptIndex = Math.max(
        ctx.getMessages().findIndex((item) => item.role === 'user' && userVariants(item).some((variant) => variant.id === (updated.id || message.id))),
        0
      );
      resetUserMessageEdit();
      await ctx.sendAsk(content, {
        appendUser: false,
        assistantIndex: -1,
        promptIndex,
        parentMessageId: updated.id || message.id
      });
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setEditingUserBusy(false);
    }
  }

  async function copyMessage(index: number) {
    const message = ctx.getMessages()[index];
    if (!message?.content) return;
    try {
      await navigator.clipboard.writeText(message.content);
      ctx.setCopiedMessageIndex(index);
      const existing = ctx.getCopyMessageResetTimer();
      if (existing) clearTimeout(existing);
      ctx.setCopyMessageResetTimer(setTimeout(() => {
        ctx.setCopiedMessageIndex(null);
      }, 1400));
    } catch {
      ctx.setCopiedMessageIndex(null);
    }
  }

  function retryPromptForMessage(index: number) {
    return retryPromptForMessageFor(ctx.getMessages(), index);
  }

  function retryContextForMessage(index: number) {
    return retryContextForMessageFor(ctx.getMessages(), index);
  }

  async function retryMessage(index: number) {
    const retry = retryContextForMessage(index);
    if (!retry?.content || ctx.getBusy()) return;
    await ctx.sendAsk(retry.content, {
      appendUser: false,
      assistantIndex: retry.assistantIndex,
      promptIndex: retry.promptIndex,
      parentMessageId: retry.parentMessageId
    });
  }

  function nextQueuedFollowUpId() {
    const nextSequence = ctx.getQueuedFollowUpSequence() + 1;
    ctx.setQueuedFollowUpSequence(nextSequence);
    return queuedFollowUpId(Date.now(), nextSequence);
  }

  function enqueueFollowUpPrompt(content: string, skill = ctx.getSelectedSkill()) {
    const clean = content.trim();
    if (!clean) return;
    ctx.setQueuedFollowUps([...ctx.getQueuedFollowUps(), createQueuedFollowUp(clean, skill, nextQueuedFollowUpId(), Date.now())]);
    ctx.setPrompt('');
    ctx.setComposerFocused(false);
    ctx.pushActivity('follow-up queued');
    void resizeComposerTextarea();
  }

  function startEditQueuedFollowUp(id: string) {
    ctx.setQueuedFollowUps(startQueuedFollowUpEdit(ctx.getQueuedFollowUps(), id));
  }

  function updateQueuedFollowUpDraft(id: string, draft: string) {
    ctx.setQueuedFollowUps(updateQueuedFollowUpDraftFor(ctx.getQueuedFollowUps(), id, draft));
  }

  function saveQueuedFollowUp(id: string) {
    ctx.setQueuedFollowUps(saveQueuedFollowUpDraft(ctx.getQueuedFollowUps(), id));
    if (!ctx.getBusy()) {
      setTimeout(() => void runNextQueuedFollowUp(), 0);
    }
  }

  function cancelEditQueuedFollowUp(id: string) {
    ctx.setQueuedFollowUps(cancelQueuedFollowUpEdit(ctx.getQueuedFollowUps(), id));
    if (!ctx.getBusy()) {
      setTimeout(() => void runNextQueuedFollowUp(), 0);
    }
  }

  function deleteQueuedFollowUp(id: string) {
    ctx.setQueuedFollowUps(deleteQueuedFollowUpItem(ctx.getQueuedFollowUps(), id));
    if (!ctx.getBusy()) {
      setTimeout(() => void runNextQueuedFollowUp(), 0);
    }
  }

  async function runNextQueuedFollowUp() {
    const queuedFollowUps = ctx.getQueuedFollowUps();
    if (ctx.getBusy() || ctx.getProcessingQueuedFollowUp() || queuedFollowUps.length === 0) return;
    const next = queuedFollowUps[0];
    if (!next || next.editing) return;
    ctx.setQueuedFollowUps(queuedFollowUps.slice(1));
    ctx.setProcessingQueuedFollowUp(true);
    try {
      await ctx.sendAsk(next.content, { skillOverride: next.skill, fromQueue: true });
    } finally {
      ctx.setProcessingQueuedFollowUp(false);
      if (ctx.getQueuedFollowUps().length > 0) {
        setTimeout(() => void runNextQueuedFollowUp(), 0);
      }
    }
  }

  async function resizeComposerTextarea() {
    await tick();
    const textarea = ctx.getComposerTextarea();
    if (!textarea) return;
    const minHeight = ctx.getComposerFocused() ? 56 : 40;
    const maxHeight = 192;
    textarea.style.height = 'auto';
    const nextHeight = Math.min(maxHeight, Math.max(minHeight, textarea.scrollHeight));
    textarea.style.height = `${nextHeight}px`;
    textarea.style.overflowY = textarea.scrollHeight > maxHeight ? 'auto' : 'hidden';
  }

  function focusComposer() {
    ctx.setComposerFocused(true);
    void resizeComposerTextarea();
  }

  function blurComposer() {
    ctx.setComposerFocused(false);
    void resizeComposerTextarea();
  }

  function handleComposerKeydown(event: KeyboardEvent) {
    if (event.isComposing || event.key !== 'Enter') return;
    if (!event.metaKey && !event.ctrlKey) return;
    if (event.shiftKey) {
      event.preventDefault();
      void resizeComposerTextarea();
      return;
    }
    event.preventDefault();
    event.stopPropagation();
    if (!ctx.getPrompt().trim()) return;
    void ctx.sendAsk();
  }

  function prepareRetryAssistant(options: SendAskOptions) {
    const parentMessageId = options.parentMessageId || ctx.getMessages()[options.promptIndex ?? -1]?.id || '';
    const prepared = prepareRetryAssistantState(ctx.getMessages(), options, parentMessageId);
    ctx.setMessages(prepared.messages);
    return prepared.assistantIndex;
  }

  return {
    streamTargetIndex,
    updateAssistantMessageAt,
    switchAssistantVariant,
    switchUserVariant,
    handleChatScroll,
    scrollChatToBottom,
    syncScrollForMessages,
    assistantMetaItems,
    toggleUserMessageExpanded,
    resetUserMessageEdit,
    startEditUserMessage,
    cancelEditUserMessage,
    ensureFlatConversationMessages,
    upsertFlatConversationMessage,
    submitEditedUserMessage,
    copyMessage,
    retryPromptForMessage,
    retryContextForMessage,
    retryMessage,
    enqueueFollowUpPrompt,
    startEditQueuedFollowUp,
    updateQueuedFollowUpDraft,
    saveQueuedFollowUp,
    cancelEditQueuedFollowUp,
    deleteQueuedFollowUp,
    runNextQueuedFollowUp,
    resizeComposerTextarea,
    focusComposer,
    blurComposer,
    handleComposerKeydown,
    prepareRetryAssistant
  };
}
