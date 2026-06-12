import type { ChatMessage, SettingsView, SetupState, Status } from './appTypes';
import type { Tab } from './appOptions';
import { hasActiveAssistantRun as hasActiveAssistantRunFor } from './chatStateHelpers';
import { loadRuntimeSettingsSurface } from './settingsActions';
import type { LiveRefreshReason } from './liveRefresh';
import {
  entryRefreshTargetForTab,
  liveRefreshTargetForTab,
  manualRefreshTargetForTab,
  shouldRefreshChatOnLive,
  type RefreshTarget
} from './pageRefreshHelpers';

export type AppRefreshControllerContext = {
  getActiveTab: () => Tab;
  setActiveTab: (tab: Tab) => void;
  setMoreOpen: (open: boolean) => void;
  getBusy: () => boolean;
  getLiveRefreshBusy: () => boolean;
  setLiveRefreshBusy: (busy: boolean) => void;
  getActiveConversationId: () => string;
  getMessages: () => ChatMessage[];
  setStatus: (status: Status) => void;
  setSetupState: (setupState: SetupState) => void;
  setSettings: (settings: SettingsView) => void;
  syncSettingsForm: (settings: SettingsView | null) => void;
  refreshAutomation: (silent?: boolean) => Promise<void>;
  refreshLearning: (silent?: boolean) => Promise<void>;
  refreshExtensions: (silent?: boolean) => Promise<void>;
  refreshSkillsView: (silent?: boolean) => Promise<void>;
  refreshDocumentsSurface: (silent?: boolean) => Promise<void>;
  refreshModels: (silent?: boolean) => Promise<void>;
  refreshToolRuns: (silent?: boolean) => Promise<void>;
  listMemories: (silent?: boolean) => Promise<void>;
  refreshKnowledge: (silent?: boolean) => Promise<void>;
  refreshConversations: () => Promise<void>;
  openConversation: (conversationId: string, options?: { silent?: boolean }) => Promise<void>;
  refresh: () => Promise<void>;
};

export function createAppRefreshController(ctx: AppRefreshControllerContext) {
  async function refreshTarget(target: RefreshTarget, silent = false) {
    if (target === 'chat') {
      const assistantRunActive = ctx.getBusy() || hasActiveAssistantRun();
      if (ctx.getActiveConversationId() && !assistantRunActive) {
        await ctx.openConversation(ctx.getActiveConversationId(), { silent: true });
      }
      await ctx.refreshConversations();
      return;
    }
    if (target === 'automation') {
      await ctx.refreshAutomation(silent);
      return;
    }
    if (target === 'models') {
      await ctx.refreshModels(silent);
      return;
    }
    if (target === 'learning') {
      await ctx.refreshLearning(silent);
      return;
    }
    if (target === 'extensions') {
      await ctx.refreshExtensions(silent);
      return;
    }
    if (target === 'skills') {
      await ctx.refreshSkillsView(silent);
      return;
    }
    if (target === 'documents') {
      await ctx.refreshDocumentsSurface(silent);
      return;
    }
    if (target === 'tools') {
      await ctx.refreshToolRuns(silent);
      return;
    }
    if (target === 'memory') {
      await ctx.listMemories(silent);
      await ctx.refreshKnowledge(silent);
      return;
    }
    if (target === 'settings') {
      const runtimeSettings = await loadRuntimeSettingsSurface();
      ctx.setStatus(runtimeSettings.status);
      ctx.setSetupState(runtimeSettings.setupState);
      ctx.setSettings(runtimeSettings.settings);
      ctx.syncSettingsForm(runtimeSettings.settings);
      return;
    }
    if (target === 'full') {
      await ctx.refresh();
    }
  }

  async function selectTab(tab: Tab) {
    ctx.setActiveTab(tab);
    ctx.setMoreOpen(false);
    await refreshTarget(entryRefreshTargetForTab(tab));
  }

  async function refreshActiveTab() {
    await refreshTarget(manualRefreshTargetForTab(ctx.getActiveTab()));
  }

  function hasActiveAssistantRun() {
    return hasActiveAssistantRunFor(ctx.getMessages());
  }

  async function refreshLiveVisibleData(reason: LiveRefreshReason) {
    if (ctx.getLiveRefreshBusy()) return;
    if (ctx.getBusy()) return;
    ctx.setLiveRefreshBusy(true);
    try {
      if (ctx.getActiveTab() === 'chat') {
        if (shouldRefreshChatOnLive(reason, ctx.getActiveConversationId(), hasActiveAssistantRun())) {
          await ctx.openConversation(ctx.getActiveConversationId(), { silent: true });
          await ctx.refreshConversations();
        }
        return;
      }
      await refreshTarget(liveRefreshTargetForTab(ctx.getActiveTab()), true);
    } finally {
      ctx.setLiveRefreshBusy(false);
    }
  }

  return {
    refreshTarget,
    selectTab,
    refreshActiveTab,
    hasActiveAssistantRun,
    refreshLiveVisibleData
  };
}
