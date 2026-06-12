import type { ChatMessage, ConversationDetail, ConversationSession, OpenConversationOptions } from './appTypes';
import type { Tab } from './appOptions';
import {
  deleteConversation,
  getConversationDetail,
  listConversations,
  renameConversation,
  setConversationStarredState
} from './conversationActions';
import { exportConversationDetail, type ConversationExportFormat } from './conversationExport';
import { groupConversationMessages } from './conversationHelpers';
import { asArray, promptTitle } from './uiHelpers';

export type ConversationControllerContext = {
  getActiveConversationId: () => string;
  getConversations: () => ConversationSession[];
  setConversations: (items: ConversationSession[]) => void;
  setActiveTab: (tab: Tab) => void;
  setActiveConversation: (conversationId: string) => void;
  getChatTitle: () => string;
  setChatTitle: (title: string) => void;
  setMessages: (messages: ChatMessage[]) => void;
  setConversationFlatMessages: (messages: ConversationDetail['messages']) => void;
  setPromptVariantOverrides: (overrides: Record<string, string>) => void;
  setPrompt: (prompt: string) => void;
  setSelectedSkill: (skill: string) => void;
  setMoreOpen: (open: boolean) => void;
  setChatMenuOpen: (open: boolean) => void;
  setRenamingChatTitle: (value: boolean) => void;
  setKeepChatPinned: (value: boolean) => void;
  setMemoryQuery: (query: string) => void;
  setSidebarVisible: (visible: boolean) => void;
  persistSidebarVisible: (visible: boolean) => void;
  setError: (message: string) => void;
  notify: (message: string) => void;
  confirm: (message: string) => Promise<boolean> | boolean;
  resetUserMessageEdit: () => void;
  scrollChatToBottom: (force?: boolean) => Promise<void>;
  pushActivity: (line: string) => void;
};

export function createConversationController(ctx: ConversationControllerContext) {
  async function refreshConversations() {
    ctx.setConversations(await listConversations(80));
  }

  function conversationTitle(conversationID: string) {
    const conversation = ctx.getConversations().find((item) => item.id === conversationID);
    return promptTitle(conversation?.title || (conversationID === ctx.getActiveConversationId() ? ctx.getChatTitle() : '') || 'conversation');
  }

  function newChat() {
    ctx.setActiveTab('chat');
    ctx.setMoreOpen(false);
    ctx.setChatMenuOpen(false);
    ctx.setRenamingChatTitle(false);
    ctx.resetUserMessageEdit();
    ctx.setActiveConversation('');
    ctx.setChatTitle('New chat');
    ctx.setMessages([]);
    ctx.setConversationFlatMessages([]);
    ctx.setPromptVariantOverrides({});
    ctx.setPrompt('');
    ctx.setSelectedSkill('');
    ctx.setError('');
    ctx.setKeepChatPinned(true);
  }

  function renameChatTitle(nextTitle: string) {
    const clean = promptTitle(nextTitle);
    if (!clean) return;
    ctx.setChatTitle(clean);
    if (ctx.getActiveConversationId()) {
      void renameActiveConversation(clean);
    }
  }

  async function renameActiveConversation(title: string) {
    ctx.setError('');
    try {
      const updated = await renameConversation(ctx.getActiveConversationId(), title);
      ctx.setChatTitle(updated.title || title);
      await refreshConversations();
      ctx.notify(`conversation renamed: ${updated.title || title}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  function startRenameChatTitle() {
    ctx.setActiveTab('chat');
    ctx.setChatMenuOpen(false);
    ctx.setRenamingChatTitle(true);
  }

  async function setConversationStarred(conversationID: string, starred: boolean) {
    if (!conversationID) return;
    ctx.setError('');
    try {
      const updated = await setConversationStarredState(conversationID, starred);
      if (ctx.getActiveConversationId() === conversationID) {
        ctx.setChatTitle(updated.title || ctx.getChatTitle());
      }
      await refreshConversations();
      const message = `${updated.starred ? 'starred' : 'unstarred'}: ${updated.title}`;
      ctx.pushActivity(message);
      ctx.notify(message);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setChatMenuOpen(false);
    }
  }

  async function toggleStarChat() {
    const conversationID = ctx.getActiveConversationId();
    if (!conversationID) return;
    const current = ctx.getConversations().find((conversation) => conversation.id === conversationID);
    await setConversationStarred(conversationID, !current?.starred);
  }

  async function toggleSidebarConversationStar(item: ConversationSession) {
    await setConversationStarred(item.id, !item.starred);
  }

  async function renameSidebarConversation(item: ConversationSession) {
    await openConversation(item.id);
    ctx.setRenamingChatTitle(true);
  }

  async function openConversation(conversationID: string, options: OpenConversationOptions = {}) {
    if (!conversationID) return;
    const silent = Boolean(options.silent);
    if (!silent) {
      ctx.setError('');
      ctx.setActiveTab('chat');
      ctx.setMoreOpen(false);
      ctx.setChatMenuOpen(false);
      ctx.setRenamingChatTitle(false);
      ctx.resetUserMessageEdit();
    }
    try {
      const detail = await getConversationDetail(conversationID);
      ctx.setActiveConversation(detail.conversation.id);
      ctx.setChatTitle(detail.conversation.title || 'New chat');
      const flatMessages = asArray(detail.messages);
      const overrides: Record<string, string> = {};
      ctx.setConversationFlatMessages(flatMessages);
      ctx.setPromptVariantOverrides(overrides);
      ctx.setMessages(groupConversationMessages(flatMessages, overrides));
      if (!silent) {
        ctx.setKeepChatPinned(true);
        void ctx.scrollChatToBottom(true);
      } else {
        void ctx.scrollChatToBottom(false);
      }
    } catch (err) {
      if (!silent) {
        ctx.setError(err instanceof Error ? err.message : String(err));
      }
    }
  }

  async function deleteChat() {
    const conversationID = ctx.getActiveConversationId();
    if (!conversationID) {
      newChat();
      return;
    }
    await deleteConversationById(conversationID);
  }

  async function deleteConversationById(conversationID: string) {
    const title = conversationTitle(conversationID);
    if (!(await ctx.confirm(`Delete "${title}"? This removes the conversation from this local profile.`))) {
      ctx.setChatMenuOpen(false);
      return;
    }
    ctx.setError('');
    try {
      await deleteConversation(conversationID);
      await refreshConversations();
      if (ctx.getActiveConversationId() === conversationID) {
        newChat();
      }
      ctx.pushActivity(`conversation deleted: ${title}`);
      ctx.notify(`conversation deleted: ${title}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setChatMenuOpen(false);
    }
  }

  async function exportConversationById(conversationID: string, format: ConversationExportFormat) {
    if (!conversationID) return;
    ctx.setError('');
    try {
      const detail = await getConversationDetail(conversationID);
      exportConversationDetail(detail, format);
      const title = promptTitle(detail.conversation.title || conversationTitle(conversationID));
      const label = format === 'json' ? 'JSON' : 'Markdown';
      ctx.pushActivity(`conversation exported as ${label}: ${title}`);
      ctx.notify(`conversation exported as ${label}: ${title}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setChatMenuOpen(false);
    }
  }

  async function exportSidebarConversationMarkdown(item: ConversationSession) {
    await exportConversationById(item.id, 'markdown');
  }

  async function exportSidebarConversationJSON(item: ConversationSession) {
    await exportConversationById(item.id, 'json');
  }

  async function exportActiveConversationMarkdown() {
    await exportConversationById(ctx.getActiveConversationId(), 'markdown');
  }

  async function exportActiveConversationJSON() {
    await exportConversationById(ctx.getActiveConversationId(), 'json');
  }

  function openMemorySearch() {
    ctx.setMoreOpen(false);
    ctx.setActiveTab('memory');
    ctx.setMemoryQuery('');
  }

  function setSidebarVisible(value: boolean) {
    ctx.setSidebarVisible(value);
    ctx.setMoreOpen(false);
    ctx.persistSidebarVisible(value);
  }

  return {
    refreshConversations,
    newChat,
    renameChatTitle,
    renameActiveConversation,
    startRenameChatTitle,
    setConversationStarred,
    toggleStarChat,
    toggleSidebarConversationStar,
    renameSidebarConversation,
    exportConversationById,
    exportActiveConversationMarkdown,
    exportActiveConversationJSON,
    exportSidebarConversationMarkdown,
    exportSidebarConversationJSON,
    openConversation,
    deleteChat,
    deleteConversationById,
    openMemorySearch,
    setSidebarVisible
  };
}
