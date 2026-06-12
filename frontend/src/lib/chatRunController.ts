import { tick } from 'svelte';
import type { AskResult, ChatMessage, ConversationDetail, ConversationSession, SendAskOptions, Status } from './appTypes';
import { desktopAPI, setActiveRequestSignal, shouldUseHTTPAskStream, stopActiveAskHTTP } from './api';
import { agentStreamEventEffect } from './agentStreamEventHelpers';
import { askOnce } from './chatActions';
import { appendAssistantTraceLine, assistantResultMessage, visibleTextLength } from './chatResultHelpers';
import { streamAskHTTP } from './chatStream';
import {
  appendWorkingAskMessages,
  askContentFromInput,
  askSkillForOptions,
  chatTitleFromConversations,
  flatAssistantMessageFromAskResult,
  flatUserMessageFromAskResult,
  markSavedUserMessage,
  messagesAfterAskError,
  shouldAppendAskUser,
  streamTransportActivity
} from './chatSubmitHelpers';
import { normalizeStreamEvent } from './handoffHelpers';
import { persistMessageHandoffs } from './handoffPersistence';
import { titleFromPrompt } from './modelHelpers';
import { promptTitle } from './uiHelpers';

export type ChatRunControllerContext = {
  getMessages: () => ChatMessage[];
  setMessages: (messages: ChatMessage[]) => void;
  getStreamingMessageIndex: () => number;
  setStreamingMessageIndex: (index: number) => void;
  streamTargetIndex: () => number;
  updateAssistantMessageAt: (index: number, updater: (message: ChatMessage) => ChatMessage) => void;
  prepareRetryAssistant: (options: SendAskOptions) => number;
  getBusy: () => boolean;
  setBusy: (busy: boolean) => void;
  getStopRequested: () => boolean;
  setStopRequested: (stop: boolean) => void;
  getActiveController: () => AbortController | null;
  setActiveController: (controller: AbortController | null) => void;
  getPrompt: () => string;
  setPrompt: (prompt: string) => void;
  getSelectedSkill: () => string;
  getActiveConversationId: () => string;
  setActiveConversation: (conversationId: string) => void;
  getChatTitle: () => string;
  setChatTitle: (title: string) => void;
  getConversations: () => ConversationSession[];
  setKeepChatPinned: (pinned: boolean) => void;
  setComposerFocused: (focused: boolean) => void;
  getQueuedFollowUps: () => unknown[];
  enqueueFollowUpPrompt: (content: string, skill?: string) => void;
  runNextQueuedFollowUp: () => Promise<void>;
  upsertFlatConversationMessage: (message: ConversationDetail['messages'][number]) => void;
  rememberCapabilityHandoff: (data: Record<string, string> | undefined) => void;
  rememberSchedulerHandoff: (data: Record<string, string> | undefined) => void;
  rememberPermission: (data: Record<string, string>) => void;
  rememberEditProposal: (data: Record<string, string>) => void;
  getStatus: () => Status | null;
  setStatus: (status: Status) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  refreshConversations: () => Promise<void>;
  refreshToolRuns: () => Promise<void>;
};

export function createChatRunController(ctx: ChatRunControllerContext) {
  function appendAssistantTrace(line: string) {
    if (!line || ctx.getMessages().length === 0) return;
    const target = ctx.streamTargetIndex();
    ctx.updateAssistantMessageAt(target, (message) => appendAssistantTraceLine(message, line));
  }

  async function waitForPaint() {
    await tick();
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
  }

  async function applyAssistantResult(result: AskResult, animateBufferedResult: boolean, targetIndex = ctx.streamTargetIndex()) {
    const previous = ctx.getMessages()[targetIndex];
    const finalMessage = assistantResultMessage(result, previous);
    const target = result.text || previous?.content || '';

    if (!animateBufferedResult || !target || visibleTextLength(previous?.content || '') > 0) {
      ctx.updateAssistantMessageAt(targetIndex, () => finalMessage);
      return;
    }

    let cursor = 0;
    while (cursor < target.length && !ctx.getStopRequested()) {
      const remaining = target.length - cursor;
      const step = remaining > 420 ? 24 : remaining > 180 ? 14 : remaining > 70 ? 8 : 4;
      cursor = Math.min(target.length, cursor + step);
      ctx.updateAssistantMessageAt(targetIndex, () => ({ ...finalMessage, content: target.slice(0, cursor), meta: 'streaming' }));
      await waitForPaint();
    }

    ctx.updateAssistantMessageAt(targetIndex, () => finalMessage);
  }

  function handleAgentStreamEvent(input: unknown) {
    const current = normalizeStreamEvent(input);
    if (current.type === 'message.saved') {
      const data = current.data ?? {};
      const conversationID = data.conversation_id || data.conversationId || '';
      if (conversationID && (ctx.getStreamingMessageIndex() >= 0 || !ctx.getActiveConversationId())) {
        ctx.setActiveConversation(conversationID);
      }
      if (data.role === 'user' && data.message_id) {
        const userIndex = ctx.getStreamingMessageIndex() > 0 ? ctx.getStreamingMessageIndex() - 1 : -1;
        const savedContent = data.content || (userIndex >= 0 ? ctx.getMessages()[userIndex]?.content : '') || '';
        ctx.setMessages(ctx.getMessages().map((message, index) => {
          if (index !== userIndex || message.role !== 'user') return message;
          if (data.content && message.content !== data.content) return message;
          return { ...message, id: data.message_id };
        }));
        if (savedContent) {
          ctx.upsertFlatConversationMessage({
            id: data.message_id,
            role: 'user',
            content: savedContent,
            model: 'yemaka',
            createdAt: new Date().toISOString(),
            activeVariant: true
          });
        }
      }
      if (data.role === 'assistant' && data.message_id && ctx.getStreamingMessageIndex() >= 0) {
        ctx.updateAssistantMessageAt(ctx.getStreamingMessageIndex(), (message) => {
          const next = {
            ...message,
            id: data.message_id,
            content: data.content || message.content,
            parentId: data.parent_message_id || message.parentId,
            variantIndex: Number(data.variant_index || message.variantIndex || 0),
            meta: data.content ? undefined : message.meta
          };
          persistMessageHandoffs(next);
          return next;
        });
      }
    }
    if (current.type === 'model.token' && current.token) {
      ctx.updateAssistantMessageAt(ctx.streamTargetIndex(), (message) => ({ ...message, content: message.content + current.token, meta: 'streaming' }));
    }
    const effect = agentStreamEventEffect(current);
    if (effect.capabilityHandoffData) ctx.rememberCapabilityHandoff(effect.capabilityHandoffData);
    if (effect.schedulerHandoffData) ctx.rememberSchedulerHandoff(effect.schedulerHandoffData);
    if (effect.permissionData) ctx.rememberPermission(effect.permissionData);
    if (effect.editProposalData) ctx.rememberEditProposal(effect.editProposalData);
    if (effect.activity) ctx.pushActivity(effect.activity);
    if (effect.trace) appendAssistantTrace(effect.trace);
    const status = ctx.getStatus();
    if (effect.statusPatch && status) {
      ctx.setStatus({ ...status, ...effect.statusPatch });
    }
  }

  async function sendAsk(contentOverride?: string | Event, options: SendAskOptions = {}) {
    const content = askContentFromInput(contentOverride, ctx.getPrompt());
    const askSkill = askSkillForOptions(options, ctx.getSelectedSkill());
    const attachmentIds = [...(options.attachmentIds ?? [])].filter(Boolean);
    const attachmentDisplay = Array.isArray(options.attachments) ? options.attachments : [];
    if (!content) return false;
    if (ctx.getBusy() && !options.fromQueue) {
      if (attachmentIds.length > 0) {
        ctx.setError('Finish the current response before sending attachments.');
        return false;
      }
      ctx.enqueueFollowUpPrompt(content, askSkill);
      return false;
    }
    if (ctx.getBusy()) return false;
    ctx.setBusy(true);
    ctx.setStopRequested(false);
    const controller = new AbortController();
    ctx.setActiveController(controller);
    setActiveRequestSignal(controller.signal);
    ctx.setError('');
    if (ctx.getChatTitle() === 'New chat') {
      ctx.setChatTitle(titleFromPrompt(content, promptTitle));
    }
    const appendUser = shouldAppendAskUser(options);
    if (appendUser) {
      const prepared = appendWorkingAskMessages(ctx.getMessages(), content, attachmentDisplay);
      ctx.setMessages(prepared.messages);
      ctx.setStreamingMessageIndex(prepared.assistantIndex);
    } else {
      ctx.setStreamingMessageIndex(ctx.prepareRetryAssistant(options));
    }
    ctx.setKeepChatPinned(true);
    ctx.setPrompt('');
    ctx.setComposerFocused(false);
    try {
      const useHTTPStream = shouldUseHTTPAskStream();
      ctx.pushActivity(streamTransportActivity(useHTTPStream));
      const result = useHTTPStream
        ? await streamAskHTTP({
            content,
            skill: askSkill,
            conversationId: ctx.getActiveConversationId(),
            parentMessageId: options.parentMessageId || '',
            attachmentIds,
            signal: controller.signal,
            onEvent: handleAgentStreamEvent,
            yieldPaint: waitForPaint
          })
        : await askOnce(content, askSkill, ctx.getActiveConversationId(), options.parentMessageId || '', attachmentIds);
      if (result.conversationId) {
        ctx.setActiveConversation(result.conversationId);
      }
      if (appendUser && result.userMessageId) {
        ctx.setMessages(markSavedUserMessage(ctx.getMessages(), ctx.getStreamingMessageIndex(), result.userMessageId).map((message) => {
          if (message.id !== result.userMessageId || message.role !== 'user') return message;
          return { ...message, attachments: result.attachments ?? message.attachments ?? [] };
        }));
        const flatUserMessage = flatUserMessageFromAskResult(content, result, () => new Date().toISOString());
        if (flatUserMessage) ctx.upsertFlatConversationMessage(flatUserMessage);
      }
      const currentAssistant = ctx.getMessages()[ctx.getStreamingMessageIndex()];
      const hasVisibleStreamedContent = visibleTextLength(currentAssistant?.content || '') > 0;
      await applyAssistantResult(result, !hasVisibleStreamedContent, ctx.getStreamingMessageIndex());
      persistMessageHandoffs(ctx.getMessages()[ctx.getStreamingMessageIndex()] ?? {});
      const flatAssistantMessage = flatAssistantMessageFromAskResult(result, currentAssistant, () => new Date().toISOString());
      if (flatAssistantMessage) ctx.upsertFlatConversationMessage(flatAssistantMessage);
      ctx.pushActivity(`ask completed ${result.conversationId || ''}`.trim());
      await ctx.refreshConversations();
      ctx.setChatTitle(chatTitleFromConversations(ctx.getConversations(), ctx.getActiveConversationId(), ctx.getChatTitle()));
      await ctx.refreshToolRuns();
      return true;
    } catch (err) {
      if (ctx.getStopRequested()) {
        ctx.updateAssistantMessageAt(ctx.getStreamingMessageIndex(), (message) => ({ ...message, content: message.content || 'Stopped.', meta: 'stopped' }));
      } else {
        ctx.setError(err instanceof Error ? err.message : String(err));
        ctx.setMessages(messagesAfterAskError(ctx.getMessages(), ctx.getStreamingMessageIndex(), appendUser));
      }
      return false;
    } finally {
      ctx.setBusy(false);
      ctx.setActiveController(null);
      setActiveRequestSignal(null);
      ctx.setStopRequested(false);
      ctx.setStreamingMessageIndex(-1);
      if (ctx.getQueuedFollowUps().length > 0) {
        setTimeout(() => void ctx.runNextQueuedFollowUp(), 0);
      }
    }
  }

  async function stopGeneration() {
    if (!ctx.getBusy()) return;
    ctx.setStopRequested(true);
    if (shouldUseHTTPAskStream()) {
      try {
        await stopActiveAskHTTP();
      } catch {
        // The running request will surface any meaningful cancellation error.
      }
    }
    ctx.getActiveController()?.abort();
    const target = desktopAPI();
    if (!shouldUseHTTPAskStream() && target?.StopGeneration) {
      try {
        await target.StopGeneration();
      } catch {
        // The running call will surface any meaningful cancellation error.
      }
    }
    ctx.updateAssistantMessageAt(ctx.streamTargetIndex(), (message) => ({ ...message, meta: 'stopped' }));
    ctx.pushActivity('generation stopped');
  }

  return {
    appendAssistantTrace,
    waitForPaint,
    applyAssistantResult,
    handleAgentStreamEvent,
    sendAsk,
    stopGeneration
  };
}
