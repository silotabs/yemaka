import type { ChatMessage, FileWritePlan, PermissionItem } from './appTypes';
import { recordPermissionDecision } from './diagnosticActions';
import { permissionActionDisabled as permissionActionDisabledFor, permissionDecisionErrorIsStale } from './diagnosticHelpers';
import {
  editProposalDraftFromEvent,
  permissionDecisionBusyState,
  permissionItemFromEvent,
  shouldStorePermissionItem,
  upsertPermissionItem
} from './permissionStreamHelpers';
import { asArray } from './uiHelpers';

export type PermissionControllerContext = {
  getPermissionItems: () => PermissionItem[];
  setPermissionItems: (items: PermissionItem[]) => void;
  getPermissionDecisionBusy: () => Record<string, boolean>;
  setPermissionDecisionBusyMap: (items: Record<string, boolean>) => void;
  getSettledPermissionRequestIds: () => Record<string, boolean>;
  setSettledPermissionRequestIds: (items: Record<string, boolean>) => void;
  getMessages: () => ChatMessage[];
  setMessages: (messages: ChatMessage[]) => void;
  setFileWritePath: (path: string) => void;
  setFileWriteContent: (content: string) => void;
  setFileWritePlan: (plan: FileWritePlan | null) => void;
  setFileWriteSummary: (summary: string) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  refreshToolRuns: () => Promise<void>;
};

export function createPermissionController(ctx: PermissionControllerContext) {
  function rememberPermission(data: Record<string, string>) {
    const item = permissionItemFromEvent(data);
    if (
      !shouldStorePermissionItem(
        item,
        ctx.getPermissionItems(),
        ctx.getSettledPermissionRequestIds(),
        ctx.getPermissionDecisionBusy()
      )
    ) {
      return;
    }
    ctx.setPermissionItems(upsertPermissionItem(ctx.getPermissionItems(), item));
  }

  function setPermissionDecisionBusy(requestId: string, busy: boolean) {
    ctx.setPermissionDecisionBusyMap(permissionDecisionBusyState(ctx.getPermissionDecisionBusy(), requestId, busy));
  }

  function markPermissionSettled(requestId: string, status: 'approved' | 'rejected' = 'approved') {
    if (!requestId) return;
    ctx.setSettledPermissionRequestIds({ ...ctx.getSettledPermissionRequestIds(), [requestId]: true });
    ctx.setPermissionItems(
      ctx.getPermissionItems().map((existing) => (existing.requestId === requestId ? { ...existing, status } : existing))
    );
  }

  function permissionActionDisabled(item: PermissionItem) {
    return permissionActionDisabledFor(item, ctx.getPermissionDecisionBusy());
  }

  function rememberEditProposal(data: Record<string, string>) {
    const draft = editProposalDraftFromEvent(data);
    if (draft.path) ctx.setFileWritePath(draft.path);
    if (draft.content) ctx.setFileWriteContent(draft.content);
    ctx.setFileWritePlan(null);
    ctx.setFileWriteSummary(draft.summary);
  }

  async function decidePermission(item: PermissionItem, decision: 'approved' | 'rejected') {
    if (permissionActionDisabled(item)) return;
    ctx.setError('');
    setPermissionDecisionBusy(item.requestId, true);
    try {
      const result = await recordPermissionDecision(item, decision);
      markPermissionSettled(item.requestId, decision);
      ctx.pushActivity(`permission ${decision}: ${item.toolName}`);
      if (result?.message) {
        const persistedMessage: ChatMessage = {
          id: result.assistantMessageId || undefined,
          role: 'assistant',
          content: result.message,
          model: result.toolName || item.toolName || 'yemaka-executor',
          sourceKind: result.result?.sourceKind || result.result?.SourceKind || 'tool',
          sources: asArray(result.result?.sources || result.result?.Sources),
          parentId: result.parentMessageId || item.assistantMessageId || item.userMessageId || item.parentMessageId,
          variantIndex: result.variantIndex,
          activeVariant: result.activeVariant,
          trace: [`permission ${decision}: ${item.toolName}`, result.executed ? 'approved tool executed' : 'approval recorded']
        };
        ctx.setMessages(upsertPermissionResultMessage(ctx.getMessages(), persistedMessage, item.assistantMessageId || item.userMessageId || ''));
      }
      await ctx.refreshToolRuns();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      if (permissionDecisionErrorIsStale(message)) {
        markPermissionSettled(item.requestId, decision);
        ctx.pushActivity(`permission refreshed: ${item.toolName}`);
        await ctx.refreshToolRuns();
      } else {
        ctx.setError(message);
      }
    } finally {
      setPermissionDecisionBusy(item.requestId, false);
    }
  }

  return {
    rememberPermission,
    setPermissionDecisionBusy,
    markPermissionSettled,
    permissionActionDisabled,
    rememberEditProposal,
    decidePermission
  };
}

function upsertPermissionResultMessage(messages: ChatMessage[], message: ChatMessage, anchorId: string) {
  if (!message.content.trim()) return messages;
  if (message.id) {
    const existingIndex = messages.findIndex((candidate) => candidate.id === message.id);
    if (existingIndex >= 0) {
      return messages.map((candidate, index) => (index === existingIndex ? { ...candidate, ...message } : candidate));
    }
  }
  const anchorIndex = anchorId ? messages.findIndex((candidate) => candidate.id === anchorId) : -1;
  if (anchorIndex < 0) return [...messages, message];
  return [...messages.slice(0, anchorIndex + 1), message, ...messages.slice(anchorIndex + 1)];
}
