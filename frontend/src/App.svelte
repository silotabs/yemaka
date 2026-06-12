<script lang="ts">
  import { onDestroy, onMount, tick } from 'svelte';
  import AppTopbar from './AppTopbar.svelte';
  import ChatSidebar from './ChatSidebar.svelte';
  import FirstRunSetup from './FirstRunSetup.svelte';
  import ComposerCapabilityMenu from './ComposerCapabilityMenu.svelte';
  import QueuedFollowUpList from './QueuedFollowUpList.svelte';
  import AutomationPage from './pages/AutomationPage.svelte';
  import ChatPage from './pages/ChatPage.svelte';
  import DocumentsPage from './pages/DocumentsPage.svelte';
  import ExtensionsPage from './pages/ExtensionsPage.svelte';
  import LearningPage from './pages/LearningPage.svelte';
  import MemoryPage from './pages/MemoryPage.svelte';
  import ModelsPage from './pages/ModelsPage.svelte';
  import SettingsPage from './pages/SettingsPage.svelte';
  import SkillsPage from './pages/SkillsPage.svelte';
  import ToolsPage from './pages/ToolsPage.svelte';
  import ToastCenter from './ToastCenter.svelte';
  import YemakaConfirmDialog from './YemakaConfirmDialog.svelte';
  import {
    activityIcon,
    activityKey,
    activityKind
  } from './lib/activityHelpers';
  import { createActivityController } from './lib/activityController';
  import {
    documentFileAccept,
    internetSearchProviderOption,
    internetSearchProviderSelectOptions,
    isTab,
    jobScheduleTypeOptions,
    jobTargetTypeOptions,
    memoryKindOptions,
    modelRoleOptions,
    moreTabs,
    promptChips,
    providerDefaultBaseURL,
    runtimeProviders,
    tabs,
    tabSubtitles,
    type Tab
  } from './lib/appOptions';
  import { desktopFilePickerAvailable, desktopPickerAvailable, isLocalWebPage } from './lib/api';
  import { loadAutomationSurface, setGeneratedConnectorEnabledAction } from './lib/automationActions';
  import { createAutomationController } from './lib/automationController';
  import { createChatHandoffController } from './lib/chatHandoffController';
  import { createChatInteractionController } from './lib/chatInteractionController';
  import { createChatRunController } from './lib/chatRunController';
  import { chatContextItems as chatContextItemsFor, type ChatContextItem } from './lib/chatDisplayHelpers';
  import { createAppRefreshController } from './lib/appRefreshController';
  import { createConversationController } from './lib/conversationController';
  import {
    listDocumentInventory,
    listWorkspaceGrants
  } from './lib/documentActions';
  import { createDocumentController } from './lib/documentController';
  import { directoryInput } from './lib/documentPageHelpers';
  import { loadExtensionSurface } from './lib/extensionActions';
  import { createExtensionsController } from './lib/extensionsController';
  import { capabilityKindLabel } from './lib/handoffHelpers';
  import { loadToolActivity } from './lib/diagnosticActions';
  import { createDiagnosticsController } from './lib/diagnosticsController';
  import {
    permissionCommandPreview,
    replayRouteSummary,
    replayToolSummary
  } from './lib/diagnosticHelpers';
  import {
    assistantVariantCount,
    assistantVariantPosition,
    userVariantCount,
    userVariantPosition,
  } from './lib/conversationHelpers';
  import { loadInternetSurface } from './lib/internetActions';
  import { createInternetController } from './lib/internetController';
  import { loadLearningSurface } from './lib/learningActions';
  import { createLearningController } from './lib/learningController';
  import { createKnowledgeController } from './lib/knowledgeController';
  import { createListExpansionController } from './lib/listExpansionController';
  import { startVisibleAutoRefresh } from './lib/liveRefresh';
  import { listMemoryResults } from './lib/memoryActions';
  import { createMemoryController } from './lib/memoryController';
  import { listModelProfiles, listRuntimeModels } from './lib/modelActions';
  import { createModelController } from './lib/modelController';
  import { modelRoleFormFromSettings } from './lib/modelHelpers';
  import {
    persistActiveConversation,
    persistActiveTab,
    persistSidebarVisible,
    restoreSidebarVisible,
    restoreStoredNavigation as restoreStoredNavigationFor
  } from './lib/navigationHelpers';
  import {
    currentRouteTab,
    navigateRouteForTab,
    replaceRouteForTab,
    startHashRouter
  } from './lib/router';
  import {
    loadRuntimeSettingsSurface,
  } from './lib/settingsActions';
  import { createSettingsController } from './lib/settingsController';
  import { normalizeResponseMode, settingsViewSupportsResponseMode } from './lib/settingsFormHelpers';
  import { createPermissionController } from './lib/permissionController';
  import { permissionItemsFromToolRuns } from './lib/permissionStreamHelpers';
  import {
    asArray,
    compactSource,
    currentPage,
    humanizeIdentifier,
    humanizeIdentifierList,
    isAssistantActiveMeta,
    pageSizeFor,
    pagedItems,
    promptTitle,
    skillDisplayLabel,
    userMessageNeedsCollapse,
    userMessageVisibleContent
  } from './lib/uiHelpers';
  import {
    buildToolRunGroups,
    formatFileSize,
    formatShortTime,
    formatSurfaces,
    formatToolName,
    formatToolStatus,
    shortId,
    toolRunTimestamp,
    type ToolRunConversationGroup
  } from './lib/toolRunHelpers';
  import { internetSearchReadiness } from './lib/statusHelpers';
  import { applyTheme, normalizeTheme } from './lib/themeHelpers';
  import { toastKindForMessage, toastTimeout, toastTitle, type ToastItem, type ToastKind } from './lib/toastCenter';
  import {
    listSkillCatalog,
  } from './lib/skillActions';
  import { createSkillsController } from './lib/skillsController';
  import type {
    Status,
    Skill,
    DomainPack,
    DomainPackReview,
    DomainPackTemplate,
    DomainPackSkillStatus,
    Extension,
    ExtensionReview,
    ExtensionFailureTrend,
    CapabilityProposal,
    CapabilityHandoff,
    PolicyStatus,
    LearningReport,
    ModelInfo,
    ModelDetails,
    ModelProfile,
    ModelProfileDraftInput,
    ModelProfileInput,
    ModelProfileResult,
    SetupState,
    MemoryResult,
    KnowledgeStatus,
    KnowledgeEntity,
    KnowledgeProposal,
    KnowledgeReviewItem,
    DocumentResult,
    DocumentInventoryItem,
    DocumentPruneResult,
    DocumentPathSuggestion,
    WorkspaceGrant,
    ToolRun,
    ToolCatalogEntry,
    NotificationItem,
    ReplayTraceFile,
    QAReview,
    ReplayTraceExplanation,
    Job,
    JobRun,
    JobStatus,
    HeartbeatReport,
    InternetStatus,
    InternetCacheSummary,
    InternetCrawlRunRecord,
    InternetRequestRecord,
    ConnectorRegistryEntry,
    GeneratedConnectorEntry,
    PermissionItem,
    FileWritePlan,
    ChatMessage,
    SendAskOptions,
    QueuedFollowUpPrompt,
    ActivityEntry,
    ConversationSession,
    ConversationDetail,
    SettingsView
  } from './lib/appTypes';

  let activeTab: Tab = 'chat';
  let status: Status | null = null;
  let setupState: SetupState | null = null;
  let settings: SettingsView | null = null;
  let skills: Skill[] = [];
  let domainPacks: DomainPack[] = [];
  let domainPackReview: DomainPackReview | null = null;
  let domainPackTemplates: DomainPackTemplate[] = [];
  let domainPackSkills: DomainPackSkillStatus[] = [];
  let extensions: Extension[] = [];
  let jobs: Job[] = [];
  let jobRuns: JobRun[] = [];
  let jobStatus: JobStatus | null = null;
  let heartbeatReport: HeartbeatReport | null = null;
  let internetStatus: InternetStatus | null = null;
  let internetUrl = '';
  let internetAllowedDomain = '';
  let internetExtractText = true;
  let internetTaskApproved = false;
  let internetCrawlMaxPages = 8;
  let internetCrawlMaxDepth = 1;
  let internetCrawlMaxDurationSeconds = 30;
  let internetCrawlMaxLinksPerPage = 50;
  let internetCrawlMaxTextChars = 4000;
  let internetFetchOutput = '';
  let internetCache: InternetCacheSummary[] = [];
  let internetCrawls: InternetCrawlRunRecord[] = [];
  let internetRequests: InternetRequestRecord[] = [];
  let connectorRegistry: ConnectorRegistryEntry[] = [];
  let generatedConnectors: GeneratedConnectorEntry[] = [];
  let policyStatus: PolicyStatus | null = null;
  let learningReport: LearningReport | null = null;
  let correctionConversationId = '';
  let correctionContent = '';
  let trajectoryConversationId = '';
  let trajectoryPreview = '';
  let models: ModelInfo[] = [];
  let messages: ChatMessage[] = [];
  let conversationFlatMessages: ConversationDetail['messages'] = [];
  let promptVariantOverrides: Record<string, string> = {};
  let chatScroll: HTMLDivElement | null = null;
  let streamingMessageIndex = -1;
  let keepChatPinned = true;
  let messageSnapshot = '';
  let activity: ActivityEntry[] = [];
  let activitySequence = 0;
  let listPages: Record<string, number> = {};
  let expandedToolRunGroups: Record<string, boolean> = {};
  let expandedToolRunSessions: Record<string, boolean> = {};
  let conversations: ConversationSession[] = [];
  let starredConversations: ConversationSession[] = [];
  let recentConversations: ConversationSession[] = [];
  let activeConversationId = '';
  let weekdayLabel = '';
  let sidebarVisible = true;
  let moreOpen = false;
  let chatMenuOpen = false;
  let renamingChatTitle = false;
  let chatTitle = 'New chat';
  let prompt = '';
  let selectedSkill = '';
  let queuedFollowUps: QueuedFollowUpPrompt[] = [];
  let queuedFollowUpSequence = 0;
  let processingQueuedFollowUp = false;
  let composerMenuOpen = false;
  let composerRouteOpen = false;
  let composerFocused = false;
  let composerTextarea: HTMLTextAreaElement | null = null;
  let copiedMessageIndex: number | null = null;
  let copyMessageResetTimer: ReturnType<typeof setTimeout> | undefined;
  let expandedMessageIndexes: Record<number, boolean> = {};
  let editingUserMessageIndex = -1;
  let editingUserPrompt = '';
  let editingUserBusy = false;
  let skillSessionId = '';
  let skillImproveName = '';
  let skillImportPath = '';
  let skillExportPath = '';
  let skillsPageStatus = '';
  let skillsPageStatusKind: ToastKind = 'info';
  let domainPackInstallPath = '';
  let activeDomainPackHandoff: CapabilityHandoff | null = null;
  let extensionProposalRequest = '';
  let extensionGenerateName = '';
  let extensionGenerateDescription = '';
  let extensionGenerateRun = false;
  let extensionGenerateJob = false;
  let extensionJobScheduleType = 'manual';
  let extensionJobScheduleExpr = '';
  let extensionJobEnabled = false;
  let extensionJobInputJSON = '{}';
  let extensionRunName = '';
  let extensionRunInputJSON = '{}';
  let extensionRunOutput = '';
  let extensionRollbackTarget = '';
  let extensionFailures: ExtensionFailureTrend[] = [];
  let extensionReview: ExtensionReview | null = null;
  let extensionReviewBusy = false;
  let extensionReviewOpenFile = '';
  let capabilityProposal: CapabilityProposal | null = null;
  let capabilityProposalRequest = '';
  let capabilityNextSteps: string[] = [];
  let capabilityHandoffBusy = false;
  let schedulerHandoffBusy = false;
  let jobScheduleType = 'interval';
  let jobScheduleExpr = '1h';
  let jobTargetType = 'extension';
  let jobTargetName = '';
  let jobInputJSON = '{}';
  let jobFailureDetails: Record<string, string> = {};
  let busy = false;
  let memoryQuery = '';
  let memoryKind = 'preference';
  let memoryContent = '';
  let memoryImportance = 3;
  let memoryResults: MemoryResult[] = [];
  let memoryPageStatus = '';
  let memoryPageStatusKind: ToastKind = 'info';
  let knowledgeStatus: KnowledgeStatus | null = null;
  let knowledgeQuery = '';
  let knowledgeEntityName = '';
  let knowledgeEntityKind = 'concept';
  let knowledgeEntitySource = 'manual';
  let knowledgeEntitySourceKind = 'manual';
  let knowledgeEntitySourceRef = '';
  let knowledgeEntityEvidence = '';
  let knowledgeFromEntityId = '';
  let knowledgeRelation = '';
  let knowledgeToEntityId = '';
  let knowledgeEdgeSource = 'manual';
  let knowledgeEdgeSourceKind = 'manual';
  let knowledgeEdgeSourceRef = '';
  let knowledgeEdgeEvidence = '';
  let knowledgeResults: KnowledgeEntity[] = [];
  let knowledgeReviewItems: KnowledgeReviewItem[] = [];
  let knowledgeDraftText = '';
  let knowledgeDraftSource = 'review_draft';
  let knowledgeDraftSourceKind = 'manual';
  let knowledgeDraftSourceRef = '';
  let knowledgeProposal: KnowledgeProposal | null = null;
  let documentPath = './docs';
  let documentPathSuggestions: DocumentPathSuggestion[] = [];
  let documentPathSuggestionOpen = false;
  let documentPathSuggestionSummary = '';
  let documentPageStatus = '';
  let documentPageStatusKind: ToastKind = 'info';
  let documentQuery = '';
  let documentResults: DocumentResult[] = [];
  let documentInventory: DocumentInventoryItem[] = [];
  let documentPrunePreview: DocumentPruneResult | null = null;
  let documentInventoryBusy = false;
  let documentPruneBusy = false;
  let workspaceGrants: WorkspaceGrant[] = [];
  let toolRuns: ToolRun[] = [];
  let groupedToolRuns: Array<ToolRunConversationGroup<ToolRun>> = [];
  let toolCatalog: ToolCatalogEntry[] = [];
  let notifications: NotificationItem[] = [];
  let replayTraces: ReplayTraceFile[] = [];
  let qaReview: QAReview | null = null;
  let replayExplanation: ReplayTraceExplanation | null = null;
  let selectedReplayTraceId = '';
  let notificationBusy: Record<string, boolean> = {};
  let replayExplainBusy = false;
  let permissionItems: PermissionItem[] = [];
  let permissionDecisionBusy: Record<string, boolean> = {};
  let settledPermissionRequestIds: Record<string, boolean> = {};
  let fileWritePath = '';
  let fileWriteContent = '';
  let fileWritePlan: FileWritePlan | null = null;
  let embeddingIndexBusy = false;
  let modelRole = 'low_memory';
  let modelName = '';
  let modelProvider = 'ollama';
  let modelBaseURL = 'http://localhost:11434/api';
  let modelDetails: ModelDetails | null = null;
  let modelPrompt = 'Say hello briefly';
  let modelOutput = '';
  let modelBusy = false;
  let modelProfiles: ModelProfile[] = [];
  let modelProfileName = 'Local Behavior Helper';
  let modelProfileDescription = '';
  let modelProfileBaseModel = '';
  let modelProfileSystem = 'Be concise, local-first, and careful. Use tools only when Yemaka makes them available.';
  let modelProfileTemperature = 0.2;
  let modelProfileNumCtx = 4096;
  let modelProfileTags = 'local, low-resource';
  let modelProfilePurpose = 'general assistant behavior';
  let modelProfileRefinedFrom = 'manual';
  let modelProfileConversationId = '';
  let modelProfileResult: ModelProfileResult | null = null;
  let modelProfileBusy = false;
  let settingLowMemory = true;
  let settingContextTokens = 4096;
  let settingResponseMode = 'balanced';
  let settingShowThinkingTrace = false;
  let settingRAGEnabled = true;
  let settingEmbeddingsEnabled = false;
  let settingEmbeddingModel = '';
  let settingTheme = 'system';
  let settingCloudEnabled = false;
  let settingCloudBaseURL = '';
  let settingCloudModel = '';
  let settingCloudKeyEnv = '';
  let settingConnectorEnabled = false;
  let settingConnectorTokenEnv = '';
  let settingMCPConnectorEnabled = false;
  let settingSlackConnectorEnabled = false;
  let settingSlackConnectorTokenEnv = '';
  let settingDiscordConnectorEnabled = false;
  let settingDiscordConnectorTokenEnv = '';
  let settingTelegramConnectorEnabled = false;
  let settingTelegramConnectorTokenEnv = '';
  let settingEmailConnectorEnabled = false;
  let settingEmailConnectorTokenEnv = '';
  let settingInternetEnabled = false;
  let settingInternetSearchEnabled = false;
  let settingInternetSearchProvider = 'none';
  let settingInternetSearchProviderSnapshot = 'none';
  let settingInternetSearchEndpoint = '';
  let settingInternetSearchAPIKeyEnv = '';
  let settingKnowledgeInfluenceEnabled = false;
  let composerEnabledSkills: Skill[] = [];
  let composerChatContextItems: ChatContextItem[] = [];
  let activeController: AbortController | null = null;
  let stopRequested = false;
  let setupMode = 'student_laptop';
  let setupModeDirty = false;
  let setupModel = '';
  let setupBusy = false;
  const composerResponseModeOptions = [
    { value: 'auto', label: 'Auto', meta: 'Yemaka chooses' },
    { value: 'fast', label: 'Fast', meta: 'Quick replies' },
    { value: 'balanced', label: 'Balanced', meta: 'Recommended' },
    { value: 'deep', label: 'Deep', meta: 'Complex work' }
  ];
  let automationBusy = false;
  let learningBusy = false;
  let domainPackBusy = false;
  let extensionBusy = false;
  let liveRefreshBusy = false;
  let confirmDialogOpen = false;
  let confirmDialogTitle = 'Confirm action';
  let confirmDialogMessage = '';
  let confirmDialogConfirmLabel = 'Continue';
  let confirmDialogCancelLabel = 'Cancel';
  let confirmDialogDestructive = false;
  let confirmDialogResolve: ((value: boolean) => void) | null = null;
  let toasts: ToastItem[] = [];
  let toastSequence = 0;
  const toastTimers = new Map<number, ReturnType<typeof setTimeout>>();
  let sendAsk: (contentOverride?: string | Event, options?: SendAskOptions) => Promise<boolean | void> = async () => false;
  let stopGeneration: () => Promise<void> = async () => {};
  let handleAgentStreamEvent: (input: unknown) => void = () => {};
  let selectTabAndRefresh: (tab: Tab) => Promise<void> = async (tab) => {
    setActiveTab(tab);
  };

  $: starredConversations = conversations.filter((conversation) => conversation.starred);
  $: recentConversations = conversations.filter((conversation) => !conversation.starred);
  $: composerResponseModeDisabled = !settingsViewSupportsResponseMode(settings);
  $: composerEnabledSkills = asArray(skills).filter((skill) => skill.enabled && skill.valid);
  $: composerChatContextItems = chatContextItemsFor({
    status,
    settings,
    ragEnabled: settingRAGEnabled,
    internetEnabled: settingInternetEnabled,
    internetStatus
  });

  function pushToast(message: string, kind?: ToastKind, title?: string) {
    const text = String(message || '').trim();
    if (!text) return;
    const toastKind = kind ?? toastKindForMessage(text);
    const duplicate = toasts.find((toast) => toast.message === text && toast.kind === toastKind);
    if (duplicate) {
      dismissToast(duplicate.id);
    }
    const id = ++toastSequence;
    const toast: ToastItem = {
      id,
      kind: toastKind,
      title: title || toastTitle(toastKind),
      message: text
    };
    toasts = [toast, ...toasts].slice(0, 5);
    const timeout = window.setTimeout(() => dismissToast(id), toastTimeout(toastKind));
    toastTimers.set(id, timeout);
  }

  function dismissToast(id: number) {
    const timeout = toastTimers.get(id);
    if (timeout) window.clearTimeout(timeout);
    toastTimers.delete(id);
    toasts = toasts.filter((toast) => toast.id !== id);
  }

  function notifySummary(message: string, kind?: ToastKind) {
    pushToast(message, kind);
  }

  function notifyError(message: string) {
    pushToast(message, 'error');
  }

  function setDocumentPageStatus(message: string, kind?: ToastKind) {
    documentPageStatus = message;
    documentPageStatusKind = kind || toastKindForMessage(message);
    notifySummary(message, kind);
  }

  function setMemoryPageStatus(message: string, kind?: ToastKind) {
    memoryPageStatus = message;
    memoryPageStatusKind = kind || toastKindForMessage(message);
    notifySummary(message, kind);
  }

  function setSkillsPageStatus(message: string, kind?: ToastKind) {
    skillsPageStatus = message;
    skillsPageStatusKind = kind || toastKindForMessage(message);
    notifySummary(message, kind);
  }

  function setActiveTab(tab: Tab) {
    activeTab = tab;
    persistActiveTab(tab);
  }

  async function navigateTab(tab: Tab) {
    navigateRouteForTab(tab);
    await selectTabAndRefresh(tab);
  }

  function startNewChatRoute() {
    navigateRouteForTab('chat');
    newChat();
  }

  async function openMemorySearchRoute() {
    navigateRouteForTab('memory');
    openMemorySearch();
    await listMemories(true);
    await refreshKnowledge(true);
  }

  async function openConversationRoute(conversationID: string) {
    navigateRouteForTab('chat');
    await openConversation(conversationID);
  }

  async function handleHashRoute(tab: Tab) {
    if (activeTab === tab) return;
    await selectTabAndRefresh(tab);
  }

  function setActiveConversation(conversationID: string) {
    activeConversationId = conversationID;
    persistActiveConversation(conversationID);
  }

  function restoreStoredNavigation() {
    const stored = restoreStoredNavigationFor(isTab);
    if (stored.tab) activeTab = stored.tab;
    activeConversationId = stored.conversationId;
  }

  function resolveConfirmDialog(value: boolean) {
    const resolve = confirmDialogResolve;
    confirmDialogOpen = false;
    confirmDialogResolve = null;
    if (resolve) resolve(value);
  }

  function confirmYemaka(message: string) {
    const clean = String(message || '').trim();
    const lower = clean.toLowerCase();
    const uninstall = lower.startsWith('uninstall');
    const deleteAction = lower.startsWith('delete');
    const archiveAction = lower.startsWith('archive');
    if (confirmDialogResolve) {
      confirmDialogResolve(false);
      confirmDialogResolve = null;
    }
    confirmDialogTitle = uninstall
      ? 'Uninstall domain pack?'
      : deleteAction
        ? 'Delete conversation?'
        : archiveAction
          ? 'Archive scheduled job?'
          : 'Confirm action';
    confirmDialogMessage = clean || 'Confirm this action.';
    confirmDialogConfirmLabel = uninstall ? 'Uninstall' : deleteAction ? 'Delete' : archiveAction ? 'Archive' : 'Continue';
    confirmDialogCancelLabel = 'Cancel';
    confirmDialogDestructive = uninstall || deleteAction || archiveAction;
    confirmDialogOpen = true;
    return new Promise<boolean>((resolve) => {
      confirmDialogResolve = resolve;
    });
  }

  function syncModelRoleForm() {
    const form = modelRoleFormFromSettings(settings, modelRole, modelName);
    modelProvider = form.provider;
    modelBaseURL = form.baseURL;
    modelName = form.name;
    if (!modelProfileBaseModel) {
      modelProfileBaseModel = form.name;
    }
  }

  function modelProfileInput(): ModelProfileInput {
    return {
      name: modelProfileName,
      description: modelProfileDescription,
      baseModel: modelProfileBaseModel,
      system: modelProfileSystem,
      temperature: Number(modelProfileTemperature) || 0,
      numCtx: Number(modelProfileNumCtx) || 0,
      tags: modelProfileTags
        .split(',')
        .map((tag) => tag.trim())
        .filter(Boolean),
      purpose: modelProfilePurpose,
      refinedFrom: modelProfileRefinedFrom
    };
  }

  function modelProfileDraftInput(): ModelProfileDraftInput {
    return {
      conversationId: modelProfileConversationId,
      name: modelProfileName,
      baseModel: modelProfileBaseModel,
      temperature: Number(modelProfileTemperature) || 0,
      numCtx: Number(modelProfileNumCtx) || 0
    };
  }

  function setModelProfileForm(profile: ModelProfile) {
    modelProfileName = profile.name || '';
    modelProfileDescription = profile.description || '';
    modelProfileBaseModel = profile.baseModel || '';
    modelProfileSystem = profile.system || '';
    modelProfileTemperature = profile.parameters?.temperature ?? 0.2;
    modelProfileNumCtx = profile.parameters?.numCtx ?? 4096;
    modelProfileTags = (profile.tags ?? []).join(', ');
    modelProfilePurpose = profile.metadata?.purpose || '';
    modelProfileRefinedFrom = profile.metadata?.refinedFrom || '';
  }

  const { pushActivity, hydrateActivityFromToolRuns } = createActivityController({
    getActivity: () => activity,
    setActivity: (items) => {
      activity = items;
    },
    getActivitySequence: () => activitySequence,
    setActivitySequence: (value) => {
      activitySequence = value;
    },
    getToolRuns: () => toolRuns
  });

  const {
    setListPage,
    toolRunGroupExpanded,
    toolRunSessionExpanded,
    toggleToolRunGroup,
    toggleToolRunSession
  } = createListExpansionController({
    getListPages: () => listPages,
    setListPages: (pages) => {
      listPages = pages;
    },
    getExpandedToolRunGroups: () => expandedToolRunGroups,
    setExpandedToolRunGroups: (state) => {
      expandedToolRunGroups = state;
    },
    getExpandedToolRunSessions: () => expandedToolRunSessions,
    setExpandedToolRunSessions: (state) => {
      expandedToolRunSessions = state;
    },
    getGroupedToolRuns: () => groupedToolRuns
  });

  const {
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
    upsertFlatConversationMessage,
    submitEditedUserMessage,
    copyMessage,
    retryPromptForMessage,
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
  } = createChatInteractionController({
    getMessages: () => messages,
    setMessages: (items) => {
      messages = items;
    },
    getConversationFlatMessages: () => conversationFlatMessages,
    setConversationFlatMessages: (items) => {
      conversationFlatMessages = items;
    },
    getPromptVariantOverrides: () => promptVariantOverrides,
    setPromptVariantOverrides: (items) => {
      promptVariantOverrides = items;
    },
    getStreamingMessageIndex: () => streamingMessageIndex,
    setStreamingMessageIndex: (value) => {
      streamingMessageIndex = value;
    },
    getActiveTab: () => activeTab,
    getBusy: () => busy,
    getChatScroll: () => chatScroll,
    getKeepChatPinned: () => keepChatPinned,
    setKeepChatPinned: (value) => {
      keepChatPinned = value;
    },
    getMessageSnapshot: () => messageSnapshot,
    setMessageSnapshot: (value) => {
      messageSnapshot = value;
    },
    getExpandedMessageIndexes: () => expandedMessageIndexes,
    setExpandedMessageIndexes: (value) => {
      expandedMessageIndexes = value;
    },
    setEditingUserMessageIndex: (value) => {
      editingUserMessageIndex = value;
    },
    getEditingUserPrompt: () => editingUserPrompt,
    setEditingUserPrompt: (value) => {
      editingUserPrompt = value;
    },
    getEditingUserBusy: () => editingUserBusy,
    setEditingUserBusy: (value) => {
      editingUserBusy = value;
    },
    setCopiedMessageIndex: (value) => {
      copiedMessageIndex = value;
    },
    getCopyMessageResetTimer: () => copyMessageResetTimer,
    setCopyMessageResetTimer: (value) => {
      copyMessageResetTimer = value;
    },
    getQueuedFollowUps: () => queuedFollowUps,
    setQueuedFollowUps: (items) => {
      queuedFollowUps = items;
    },
    getQueuedFollowUpSequence: () => queuedFollowUpSequence,
    setQueuedFollowUpSequence: (value) => {
      queuedFollowUpSequence = value;
    },
    getProcessingQueuedFollowUp: () => processingQueuedFollowUp,
    setProcessingQueuedFollowUp: (value) => {
      processingQueuedFollowUp = value;
    },
    getPrompt: () => prompt,
    setPrompt: (value) => {
      prompt = value;
    },
    getSelectedSkill: () => selectedSkill,
    getComposerFocused: () => composerFocused,
    setComposerFocused: (value) => {
      composerFocused = value;
    },
    getComposerTextarea: () => composerTextarea,
    setError: notifyError,
    pushActivity,
    sendAsk: (contentOverride, options) => sendAsk(contentOverride, options)
  });

  const {
    refreshConversations,
    newChat,
    renameChatTitle,
    startRenameChatTitle,
    toggleStarChat,
    toggleSidebarConversationStar,
    renameSidebarConversation,
    exportActiveConversationMarkdown,
    exportActiveConversationJSON,
    exportSidebarConversationMarkdown,
    exportSidebarConversationJSON,
    openConversation,
    deleteChat,
    deleteConversationById,
    openMemorySearch,
    setSidebarVisible
  } = createConversationController({
    getActiveConversationId: () => activeConversationId,
    getConversations: () => conversations,
    setConversations: (items) => {
      conversations = items;
    },
    setActiveTab,
    setActiveConversation,
    getChatTitle: () => chatTitle,
    setChatTitle: (title) => {
      chatTitle = title;
    },
    setMessages: (items) => {
      messages = items;
    },
    setConversationFlatMessages: (items) => {
      conversationFlatMessages = items;
    },
    setPromptVariantOverrides: (items) => {
      promptVariantOverrides = items;
    },
    setPrompt: (value) => {
      prompt = value;
    },
    setSelectedSkill: (value) => {
      selectedSkill = value;
    },
    setMoreOpen: (value) => {
      moreOpen = value;
    },
    setChatMenuOpen: (value) => {
      chatMenuOpen = value;
    },
    setRenamingChatTitle: (value) => {
      renamingChatTitle = value;
    },
    setKeepChatPinned: (value) => {
      keepChatPinned = value;
    },
    setMemoryQuery: (value) => {
      memoryQuery = value;
    },
    setSidebarVisible: (value) => {
      sidebarVisible = value;
    },
    persistSidebarVisible,
    setError: notifyError,
    notify: notifySummary,
    confirm: confirmYemaka,
    resetUserMessageEdit,
    scrollChatToBottom,
    pushActivity
  });

  const {
    syncSettingsForm,
    onInternetSearchProviderChange,
    indexEmbeddings,
    saveSettings,
    completeSetup
  } = createSettingsController({
    getLowMemory: () => settingLowMemory,
    setLowMemory: (value) => {
      settingLowMemory = value;
    },
    getContextTokens: () => settingContextTokens,
    setContextTokens: (value) => {
      settingContextTokens = value;
    },
    getResponseMode: () => settingResponseMode,
    setResponseMode: (value) => {
      settingResponseMode = value;
    },
    getShowThinkingTrace: () => settingShowThinkingTrace,
    setShowThinkingTrace: (value) => {
      settingShowThinkingTrace = value;
    },
    getRAGEnabled: () => settingRAGEnabled,
    setRAGEnabled: (value) => {
      settingRAGEnabled = value;
    },
    getEmbeddingsEnabled: () => settingEmbeddingsEnabled,
    setEmbeddingsEnabled: (value) => {
      settingEmbeddingsEnabled = value;
    },
    getEmbeddingModel: () => settingEmbeddingModel,
    setEmbeddingModel: (value) => {
      settingEmbeddingModel = value;
    },
    getTheme: () => settingTheme,
    setTheme: (value) => {
      settingTheme = value;
    },
    getCloudEnabled: () => settingCloudEnabled,
    setCloudEnabled: (value) => {
      settingCloudEnabled = value;
    },
    getCloudBaseURL: () => settingCloudBaseURL,
    setCloudBaseURL: (value) => {
      settingCloudBaseURL = value;
    },
    getCloudModel: () => settingCloudModel,
    setCloudModel: (value) => {
      settingCloudModel = value;
    },
    getCloudKeyEnv: () => settingCloudKeyEnv,
    setCloudKeyEnv: (value) => {
      settingCloudKeyEnv = value;
    },
    getConnectorEnabled: () => settingConnectorEnabled,
    setConnectorEnabled: (value) => {
      settingConnectorEnabled = value;
    },
    getConnectorTokenEnv: () => settingConnectorTokenEnv,
    setConnectorTokenEnv: (value) => {
      settingConnectorTokenEnv = value;
    },
    getMCPConnectorEnabled: () => settingMCPConnectorEnabled,
    setMCPConnectorEnabled: (value) => {
      settingMCPConnectorEnabled = value;
    },
    getSlackConnectorEnabled: () => settingSlackConnectorEnabled,
    setSlackConnectorEnabled: (value) => {
      settingSlackConnectorEnabled = value;
    },
    getSlackConnectorTokenEnv: () => settingSlackConnectorTokenEnv,
    setSlackConnectorTokenEnv: (value) => {
      settingSlackConnectorTokenEnv = value;
    },
    getDiscordConnectorEnabled: () => settingDiscordConnectorEnabled,
    setDiscordConnectorEnabled: (value) => {
      settingDiscordConnectorEnabled = value;
    },
    getDiscordConnectorTokenEnv: () => settingDiscordConnectorTokenEnv,
    setDiscordConnectorTokenEnv: (value) => {
      settingDiscordConnectorTokenEnv = value;
    },
    getTelegramConnectorEnabled: () => settingTelegramConnectorEnabled,
    setTelegramConnectorEnabled: (value) => {
      settingTelegramConnectorEnabled = value;
    },
    getTelegramConnectorTokenEnv: () => settingTelegramConnectorTokenEnv,
    setTelegramConnectorTokenEnv: (value) => {
      settingTelegramConnectorTokenEnv = value;
    },
    getEmailConnectorEnabled: () => settingEmailConnectorEnabled,
    setEmailConnectorEnabled: (value) => {
      settingEmailConnectorEnabled = value;
    },
    getEmailConnectorTokenEnv: () => settingEmailConnectorTokenEnv,
    setEmailConnectorTokenEnv: (value) => {
      settingEmailConnectorTokenEnv = value;
    },
    getInternetEnabled: () => settingInternetEnabled,
    setInternetEnabled: (value) => {
      settingInternetEnabled = value;
    },
    getInternetSearchEnabled: () => settingInternetSearchEnabled,
    setInternetSearchEnabled: (value) => {
      settingInternetSearchEnabled = value;
    },
    getInternetSearchProvider: () => settingInternetSearchProvider,
    setInternetSearchProvider: (value) => {
      settingInternetSearchProvider = value;
    },
    getInternetSearchProviderSnapshot: () => settingInternetSearchProviderSnapshot,
    setInternetSearchProviderSnapshot: (value) => {
      settingInternetSearchProviderSnapshot = value;
    },
    getInternetSearchEndpoint: () => settingInternetSearchEndpoint,
    setInternetSearchEndpoint: (value) => {
      settingInternetSearchEndpoint = value;
    },
    getInternetSearchAPIKeyEnv: () => settingInternetSearchAPIKeyEnv,
    setInternetSearchAPIKeyEnv: (value) => {
      settingInternetSearchAPIKeyEnv = value;
    },
    getKnowledgeInfluenceEnabled: () => settingKnowledgeInfluenceEnabled,
    setKnowledgeInfluenceEnabled: (value) => {
      settingKnowledgeInfluenceEnabled = value;
    },
    setSettings: (value) => {
      settings = value;
    },
    setSettingsSummary: notifySummary,
    setEmbeddingIndexSummary: notifySummary,
    setEmbeddingIndexBusy: (value) => {
      embeddingIndexBusy = value;
    },
    getSetupMode: () => setupMode,
    setSetupMode: (value) => {
      setupMode = value;
    },
    getSetupModel: () => setupModel,
    setSetupModeDirty: (value) => {
      setupModeDirty = value;
    },
    setSetupBusy: (value) => {
      setupBusy = value;
    },
    setSetupState: (value) => {
      setupState = value;
    },
    setError: notifyError,
    applyTheme,
    refresh,
    pushActivity
  });

  const {
    ingestDocuments,
    refreshDocumentsSurface,
    refreshDocumentInventory,
    previewPruneMissingDocuments,
    applyPruneMissingDocuments,
    scheduleDocumentPathSuggestions,
    selectDocumentPathSuggestion,
    refreshWorkspaceGrants,
    pickWorkspaceFolder,
    pickDocumentFile,
    handleDocumentFileSelected,
    handleDocumentFolderSelected,
    grantDocumentPath,
    revokeWorkspaceGrant,
    searchDocuments
  } = createDocumentController({
    getPath: () => documentPath,
    setPath: (path) => {
      documentPath = path;
    },
    getQuery: () => documentQuery,
    getGrants: () => workspaceGrants,
    setGrants: (grants) => {
      workspaceGrants = grants;
    },
    setResults: (results) => {
      documentResults = results;
    },
    setInventory: (items) => {
      documentInventory = items;
    },
    setInventoryBusy: (value) => {
      documentInventoryBusy = value;
    },
    setPruneBusy: (value) => {
      documentPruneBusy = value;
    },
    setPrunePreview: (result) => {
      documentPrunePreview = result;
    },
    setPruneSummary: setDocumentPageStatus,
    setIngestSummary: setDocumentPageStatus,
    setWorkspaceGrantSummary: setDocumentPageStatus,
    setSuggestionState: (state) => {
      documentPathSuggestions = state.suggestions;
      documentPathSuggestionOpen = state.open;
      documentPathSuggestionSummary = state.summary;
    },
    setSuggestionOpen: (open) => {
      documentPathSuggestionOpen = open;
    },
    setError: notifyError,
    isLocalWeb: isLocalWebPage,
    desktopPickerAvailable,
    desktopFilePickerAvailable,
    pushActivity
  });

  const { searchMemory, listMemories, writeMemory, pinMemory, deleteMemory } = createMemoryController({
    getQuery: () => memoryQuery,
    getKind: () => memoryKind,
    getContent: () => memoryContent,
    getImportance: () => memoryImportance,
    getResults: () => memoryResults,
    setContent: (content) => {
      memoryContent = content;
    },
    setResults: (results) => {
      memoryResults = results;
    },
    setSummary: setMemoryPageStatus,
    setError: notifyError,
    pushActivity
  });

  const {
    refreshKnowledge,
    toggleKnowledge,
    searchKnowledge,
    repairKnowledgeEvidence,
    draftProposal,
    useEntityProposal,
    useRelationProposal,
    addKnowledgeEntity,
    addKnowledgeEdge,
    refreshKnowledgeReview,
    applyKnowledgeReview
  } = createKnowledgeController({
    getQuery: () => knowledgeQuery,
    getEntityName: () => knowledgeEntityName,
    getEntityKind: () => knowledgeEntityKind,
    getEntitySource: () => knowledgeEntitySource,
    getEntitySourceKind: () => knowledgeEntitySourceKind,
    getEntitySourceRef: () => knowledgeEntitySourceRef,
    getEntityEvidence: () => knowledgeEntityEvidence,
    getFromEntityID: () => knowledgeFromEntityId,
    getRelation: () => knowledgeRelation,
    getToEntityID: () => knowledgeToEntityId,
    getEdgeSource: () => knowledgeEdgeSource,
    getEdgeSourceKind: () => knowledgeEdgeSourceKind,
    getEdgeSourceRef: () => knowledgeEdgeSourceRef,
    getEdgeEvidence: () => knowledgeEdgeEvidence,
    getDraftText: () => knowledgeDraftText,
    getDraftSource: () => knowledgeDraftSource,
    getDraftSourceKind: () => knowledgeDraftSourceKind,
    getDraftSourceRef: () => knowledgeDraftSourceRef,
    getResults: () => knowledgeResults,
    setStatus: (value) => {
      knowledgeStatus = value;
    },
    setResults: (items) => {
      knowledgeResults = items;
    },
    setReviewItems: (items) => {
      knowledgeReviewItems = items;
    },
    setProposal: (proposal) => {
      knowledgeProposal = proposal;
    },
    setEntityName: (value) => {
      knowledgeEntityName = value;
    },
    setEntityKind: (value) => {
      knowledgeEntityKind = value;
    },
    setEntitySource: (value) => {
      knowledgeEntitySource = value;
    },
    setEntitySourceKind: (value) => {
      knowledgeEntitySourceKind = value;
    },
    setEntitySourceRef: (value) => {
      knowledgeEntitySourceRef = value;
    },
    setEntityEvidence: (value) => {
      knowledgeEntityEvidence = value;
    },
    setFromEntityID: (value) => {
      knowledgeFromEntityId = value;
    },
    setToEntityID: (value) => {
      knowledgeToEntityId = value;
    },
    setRelation: (value) => {
      knowledgeRelation = value;
    },
    setEdgeSource: (value) => {
      knowledgeEdgeSource = value;
    },
    setEdgeSourceKind: (value) => {
      knowledgeEdgeSourceKind = value;
    },
    setEdgeSourceRef: (value) => {
      knowledgeEdgeSourceRef = value;
    },
    setEdgeEvidence: (value) => {
      knowledgeEdgeEvidence = value;
    },
    setSummary: setMemoryPageStatus,
    setError: notifyError,
    pushActivity
  });

  async function draftKnowledgeFromMemory(result: MemoryResult) {
    const text = (result.content || result.snippet || '').trim();
    if (!text) {
      notifyError('This memory row has no text to draft from.');
      return;
    }
    knowledgeDraftText = text;
    knowledgeDraftSource = result.explicit ? 'memory_review' : 'conversation_memory_review';
    knowledgeDraftSourceKind = result.explicit ? 'memory' : 'conversation';
    knowledgeDraftSourceRef = result.id || result.messageId || result.conversationId || '';
    await navigateTab('memory');
    await draftProposal();
  }

  async function draftKnowledgeFromDocument(result: DocumentResult) {
    const text = (result.content || '').trim();
    if (!text) {
      notifyError('This document result has no text to draft from.');
      return;
    }
    knowledgeDraftText = text;
    knowledgeDraftSource = 'document_search_result';
    knowledgeDraftSourceKind = 'document';
    knowledgeDraftSourceRef = result.chunkId || result.path || '';
    await navigateTab('memory');
    await draftProposal();
  }

  const {
    saveModelRole,
    selectModel,
    showModelDetails,
    generateModelText,
    refreshModelProfiles,
    inspectModelProfile,
    previewModelProfile,
    draftModelProfileFromConversation,
    saveModelProfile,
    applyModelProfile
  } = createModelController({
    getRole: () => modelRole,
    getName: () => modelName,
    getProvider: () => modelProvider,
    getBaseURL: () => modelBaseURL,
    getPrompt: () => modelPrompt,
    getProfileInput: modelProfileInput,
    getProfileDraftInput: modelProfileDraftInput,
    setName: (name) => {
      modelName = name;
      if (!modelProfileBaseModel) {
        modelProfileBaseModel = name;
      }
    },
    setDetails: (details) => {
      modelDetails = details;
    },
    setOutput: (output) => {
      modelOutput = output;
    },
    setProfiles: (profiles) => {
      modelProfiles = profiles;
    },
    setProfileResult: (result) => {
      modelProfileResult = result;
    },
    setProfileForm: setModelProfileForm,
    setProfileBusy: (value) => {
      modelProfileBusy = value;
    },
    setSummary: notifySummary,
    setBusy: (value) => {
      modelBusy = value;
    },
    setError: notifyError,
    pushActivity,
    refresh,
    refreshProfiles: async () => {
      modelProfiles = await listModelProfiles();
    }
  });

  const { internetSearchProviderLabel, refreshInternet, internetFetch, internetCrawl, inspectInternetCrawl } = createInternetController({
    getSearchProvider: () => settingInternetSearchProvider,
    getUrl: () => internetUrl,
    getAllowedDomain: () => internetAllowedDomain,
    getExtractText: () => internetExtractText,
    getTaskApproved: () => internetTaskApproved,
    getCrawlMaxPages: () => internetCrawlMaxPages,
    getCrawlMaxDepth: () => internetCrawlMaxDepth,
    getCrawlMaxDurationSeconds: () => internetCrawlMaxDurationSeconds,
    getCrawlMaxLinksPerPage: () => internetCrawlMaxLinksPerPage,
    getCrawlMaxTextChars: () => internetCrawlMaxTextChars,
    setStatus: (value) => {
      internetStatus = value;
    },
    setRequests: (items) => {
      internetRequests = items;
    },
    setCrawls: (items) => {
      internetCrawls = items;
    },
    setCache: (items) => {
      internetCache = items;
    },
    setFetchOutput: (output) => {
      internetFetchOutput = output;
    },
    setError: notifyError,
    pushActivity
  });

  const { refreshAutomation, createJob, updateJobInput, setJobEnabled, archiveJob, runJob, runDueJobs } = createAutomationController({
    getScheduleType: () => jobScheduleType,
    getScheduleExpr: () => jobScheduleExpr,
    getTargetType: () => jobTargetType,
    getTargetName: () => jobTargetName,
    getInputJSON: () => jobInputJSON,
    getJobs: () => jobs,
    setTargetName: (value) => {
      jobTargetName = value;
    },
    setJobSummary: notifySummary,
    setJobStatus: (value) => {
      jobStatus = value;
    },
    setJobs: (items) => {
      jobs = items;
    },
    setJobRuns: (items) => {
      jobRuns = items;
    },
    setHeartbeatReport: (value) => {
      heartbeatReport = value;
    },
    setInternetStatus: (value) => {
      internetStatus = value;
    },
    setInternetRequests: (items) => {
      internetRequests = items;
    },
    setInternetCrawls: (items) => {
      internetCrawls = items;
    },
    setInternetCache: (items) => {
      internetCache = items;
    },
    setConnectorRegistry: (items) => {
      connectorRegistry = items;
    },
    setGeneratedConnectors: (items) => {
      generatedConnectors = items;
    },
    setJobFailureDetail: (id, message) => {
      const key = String(id || '').trim();
      if (!key) return;
      jobFailureDetails = { ...jobFailureDetails, [key]: message };
    },
    clearJobFailureDetail: (id) => {
      const key = String(id || '').trim();
      if (!key || !jobFailureDetails[key]) return;
      const next = { ...jobFailureDetails };
      delete next[key];
      jobFailureDetails = next;
    },
    setAutomationBusy: (value) => {
      automationBusy = value;
    },
    setError: notifyError,
    pushActivity,
    openConversation: openConversationRoute,
    confirm: confirmYemaka
  });

  const {
    refreshExtensions,
    reviewExtension,
    useReviewedExtensionInRunner,
    testAndRegisterExtension,
    reuseExtension,
    proposeExtension,
    generateExtension,
    validateExtension,
    testExtension,
    registerExtension,
    setExtensionEnabled,
    deleteExtension,
    runExtension,
    rollbackExtension
  } = createExtensionsController({
    getExtensionProposalRequest: () => extensionProposalRequest,
    setExtensionProposalRequest: (value) => {
      extensionProposalRequest = value;
    },
    getExtensionGenerateName: () => extensionGenerateName,
    setExtensionGenerateName: (value) => {
      extensionGenerateName = value;
    },
    getExtensionGenerateDescription: () => extensionGenerateDescription,
    setExtensionGenerateDescription: (value) => {
      extensionGenerateDescription = value;
    },
    getExtensionGenerateRun: () => extensionGenerateRun,
    getExtensionGenerateJob: () => extensionGenerateJob,
    getExtensionJobScheduleType: () => extensionJobScheduleType,
    getExtensionJobScheduleExpr: () => extensionJobScheduleExpr,
    getExtensionJobEnabled: () => extensionJobEnabled,
    getExtensionJobInputJSON: () => extensionJobInputJSON,
    getExtensionRunName: () => extensionRunName,
    setExtensionRunName: (value) => {
      extensionRunName = value;
    },
    getExtensionRunInputJSON: () => extensionRunInputJSON,
    setExtensionRunInputJSON: (value) => {
      extensionRunInputJSON = value;
    },
    setExtensionRunOutput: (value) => {
      extensionRunOutput = value;
    },
    getExtensionRollbackTarget: () => extensionRollbackTarget,
    setExtensionRollbackTarget: (value) => {
      extensionRollbackTarget = value;
    },
    getExtensionReview: () => extensionReview,
    setExtensionReview: (review) => {
      extensionReview = review;
    },
    setExtensionReviewOpenFile: (path) => {
      extensionReviewOpenFile = path;
    },
    setExtensionReviewBusy: (value) => {
      extensionReviewBusy = value;
    },
    setExtensions: (items) => {
      extensions = items;
    },
    setExtensionFailures: (items) => {
      extensionFailures = items;
    },
    setExtensionBusy: (value) => {
      extensionBusy = value;
    },
    setExtensionSummary: notifySummary,
    getCapabilityProposal: () => capabilityProposal,
    setCapabilityProposal: (proposal) => {
      capabilityProposal = proposal;
    },
    getCapabilityProposalRequest: () => capabilityProposalRequest,
    setCapabilityProposalRequest: (request) => {
      capabilityProposalRequest = request;
    },
    setCapabilityNextSteps: (steps) => {
      capabilityNextSteps = steps;
    },
    setJobSummary: notifySummary,
    setError: notifyError,
    setActiveTab: (tab) => {
      void navigateTab(tab);
    },
    refreshAutomation,
    pushActivity
  });

  const { refreshLearning, setPolicyMode, saveCorrection, exportTrajectory } = createLearningController({
    getCorrectionConversationId: () => correctionConversationId,
    getCorrectionContent: () => correctionContent,
    getTrajectoryConversationId: () => trajectoryConversationId,
    setCorrectionContent: (content) => {
      correctionContent = content;
    },
    setLearningReport: (report) => {
      learningReport = report;
    },
    setPolicyStatus: (policy) => {
      policyStatus = policy;
    },
    setExtensionFailures: (items) => {
      extensionFailures = items;
    },
    setLearningBusy: (value) => {
      learningBusy = value;
    },
    setLearningSummary: notifySummary,
    setPolicySummary: notifySummary,
    setTrajectoryPreview: (preview) => {
      trajectoryPreview = preview;
    },
    setError: notifyError,
    pushActivity,
    loadDiagnostics: async () => {
      await loadDiagnostics();
    }
  });

  const {
    previewFileWrite,
    applyFileWrite,
    loadDiagnostics,
    refreshDiagnostics,
    markNotificationRead,
    dismissNotification,
    explainReplayTrace,
    promoteQARegressionSuggestion,
    reviewQARegressionSuggestion,
    generateQARegressionTestDraft,
    approveQARegressionTestDraft,
    generateQARegressionSourcePatch,
    approveQARegressionSourcePatchApplyPlan,
    planQARegressionSourceWrite,
    applyQARegressionSourceWrite
  } = createDiagnosticsController({
    getFileWritePath: () => fileWritePath,
    getFileWriteContent: () => fileWriteContent,
    getNotificationBusy: () => notificationBusy,
    setFileWritePlan: (plan) => {
      fileWritePlan = plan;
    },
    setFileWriteSummary: notifySummary,
    setNotifications: (items) => {
      notifications = items;
    },
    setReplayTraces: (items) => {
      replayTraces = items;
    },
    setQAReview: (review) => {
      qaReview = review;
    },
    setDiagnosticsSummary: notifySummary,
    setNotificationBusy: (value) => {
      notificationBusy = value;
    },
    setReplayExplainBusy: (value) => {
      replayExplainBusy = value;
    },
    setSelectedReplayTraceId: (id) => {
      selectedReplayTraceId = id;
    },
    setReplayExplanation: (explanation) => {
      replayExplanation = explanation;
    },
    setError: notifyError,
    pushActivity,
    refreshToolRuns
  });

  function clearReplayExplanation() {
    replayExplanation = null;
    selectedReplayTraceId = '';
  }

  const {
    rememberPermission,
    rememberEditProposal,
    decidePermission
  } = createPermissionController({
    getPermissionItems: () => permissionItems,
    setPermissionItems: (items) => {
      permissionItems = items;
    },
    getPermissionDecisionBusy: () => permissionDecisionBusy,
    setPermissionDecisionBusyMap: (items) => {
      permissionDecisionBusy = items;
    },
    getSettledPermissionRequestIds: () => settledPermissionRequestIds,
    setSettledPermissionRequestIds: (items) => {
      settledPermissionRequestIds = items;
    },
    getMessages: () => messages,
    setMessages: (items) => {
      messages = items;
    },
    setFileWritePath: (path) => {
      fileWritePath = path;
    },
    setFileWriteContent: (content) => {
      fileWriteContent = content;
    },
    setFileWritePlan: (plan) => {
      fileWritePlan = plan;
    },
    setFileWriteSummary: notifySummary,
    setError: notifyError,
    pushActivity,
    refreshToolRuns
  });

  const {
    loadDomainPackState,
    refreshSkillsView,
    installDomainPack,
    installDomainPackTemplate,
    reviewDomainPack,
    reviewDomainPackTemplate,
    setDomainPackEnabled,
    uninstallDomainPack,
    applyDomainPackHandoff,
    validateSkill,
    setSkillEnabled,
    createSkillFromSession,
    improveSkillFromSession,
    importSkill,
    exportSkill,
    domainPackTemplateInstallPath
  } = createSkillsController({
    getSkills: () => skills,
    setSkills: (items) => {
      skills = items;
    },
    getDomainPacks: () => domainPacks,
    setDomainPacks: (items) => {
      domainPacks = items;
    },
    getDomainPackTemplates: () => domainPackTemplates,
    setDomainPackTemplates: (items) => {
      domainPackTemplates = items;
    },
    getDomainPackSkills: () => domainPackSkills,
    setDomainPackSkills: (items) => {
      domainPackSkills = items;
    },
    getDomainPackInstallPath: () => domainPackInstallPath,
    setDomainPackInstallPath: (path) => {
      domainPackInstallPath = path;
    },
    getDomainPackBusy: () => domainPackBusy,
    setDomainPackBusy: (value) => {
      domainPackBusy = value;
    },
    setDomainPackSummary: setSkillsPageStatus,
    setDomainPackReview: (review) => {
      domainPackReview = review;
    },
    getSelectedSkill: () => selectedSkill,
    setSelectedSkill: (skill) => {
      selectedSkill = skill;
    },
    getSkillSessionId: () => skillSessionId,
    setSkillSessionId: (id) => {
      skillSessionId = id;
    },
    getSkillImproveName: () => skillImproveName,
    setSkillImproveName: (name) => {
      skillImproveName = name;
    },
    getSkillImportPath: () => skillImportPath,
    setSkillImportPath: (path) => {
      skillImportPath = path;
    },
    getSkillExportPath: () => skillExportPath,
    setSkillSummary: setSkillsPageStatus,
    setError: notifyError,
    pushActivity,
    confirm: confirmYemaka
  });

  const {
    rememberCapabilityHandoff,
    rememberSchedulerHandoff,
    setSchedulerHandoffInput,
    setCapabilityRunAfterGenerate,
    setCapabilityRunInput,
    setCapabilityScheduleAfterGenerate,
    setCapabilityScheduleType,
    setCapabilityScheduleExpr,
    setCapabilityScheduleEnabled,
    setCapabilityScheduleInput,
    openMessageCapabilityInExtensions,
    openMessageDomainPacks,
    approveChatCapability,
    createSchedulerJobFromChat
  } = createChatHandoffController({
    getMessages: () => messages,
    updateAssistantMessageAt,
    streamTargetIndex,
    getExtensions: () => extensions,
    setExtensions: (items) => {
      extensions = items;
    },
    getCapabilityHandoffBusy: () => capabilityHandoffBusy,
    setCapabilityHandoffBusy: (value) => {
      capabilityHandoffBusy = value;
    },
    getSchedulerHandoffBusy: () => schedulerHandoffBusy,
    setSchedulerHandoffBusy: (value) => {
      schedulerHandoffBusy = value;
    },
    getExtensionGenerateName: () => extensionGenerateName,
    setExtensionGenerateName: (value) => {
      extensionGenerateName = value;
    },
    getExtensionGenerateDescription: () => extensionGenerateDescription,
    setExtensionGenerateDescription: (value) => {
      extensionGenerateDescription = value;
    },
    getCapabilityNextSteps: () => capabilityNextSteps,
    setCapabilityNextSteps: (items) => {
      capabilityNextSteps = items;
    },
    setCapabilityProposal: (proposal) => {
      capabilityProposal = proposal;
    },
    setCapabilityProposalRequest: (request) => {
      capabilityProposalRequest = request;
    },
    setExtensionProposalRequest: (request) => {
      extensionProposalRequest = request;
    },
    setExtensionSummary: notifySummary,
    setExtensionRunName: (name) => {
      extensionRunName = name;
    },
    setExtensionRunOutput: (output) => {
      extensionRunOutput = output;
    },
    getActiveConversationId: () => activeConversationId,
    setActiveDomainPackHandoff: (handoff) => {
      activeDomainPackHandoff = handoff;
    },
    getDomainPackInstallPath: () => domainPackInstallPath,
    setDomainPackInstallPath: (path) => {
      domainPackInstallPath = path;
    },
    setDomainPackSummary: notifySummary,
    domainPackTemplateInstallPath,
    refreshSkillsView: async () => {
      await refreshSkillsView();
    },
    reviewExtension,
    refreshExtensions: async () => {
      await refreshExtensions();
    },
    refreshAutomation: async () => {
      await refreshAutomation();
    },
    setJobSummary: notifySummary,
    setActiveTab: (tab) => {
      void navigateTab(tab);
    },
    afterOpen: () => {
      void tick();
    },
    setError: notifyError,
    pushActivity
  });

  const appRefreshController = createAppRefreshController({
    getActiveTab: () => activeTab,
    setActiveTab,
    setMoreOpen: (value) => {
      moreOpen = value;
    },
    getBusy: () => busy,
    getLiveRefreshBusy: () => liveRefreshBusy,
    setLiveRefreshBusy: (value) => {
      liveRefreshBusy = value;
    },
    getActiveConversationId: () => activeConversationId,
    getMessages: () => messages,
    setStatus: (value) => {
      status = value;
    },
    setSetupState: (value) => {
      setupState = value;
    },
    setSettings: (value) => {
      settings = value;
    },
    syncSettingsForm,
    refreshAutomation,
    refreshLearning,
    refreshExtensions,
    refreshSkillsView,
    refreshDocumentsSurface,
    refreshModels: refreshModelsSurface,
    refreshToolRuns,
    listMemories,
    refreshKnowledge,
    refreshConversations,
    openConversation,
    refresh
  });
  const {
    refreshActiveTab,
    refreshLiveVisibleData
  } = appRefreshController;
  selectTabAndRefresh = appRefreshController.selectTab;

  const chatRunController = createChatRunController({
    getMessages: () => messages,
    setMessages: (items) => {
      messages = items;
    },
    getStreamingMessageIndex: () => streamingMessageIndex,
    setStreamingMessageIndex: (value) => {
      streamingMessageIndex = value;
    },
    streamTargetIndex,
    updateAssistantMessageAt,
    prepareRetryAssistant,
    getBusy: () => busy,
    setBusy: (value) => {
      busy = value;
    },
    getStopRequested: () => stopRequested,
    setStopRequested: (value) => {
      stopRequested = value;
    },
    getActiveController: () => activeController,
    setActiveController: (value) => {
      activeController = value;
    },
    getPrompt: () => prompt,
    setPrompt: (value) => {
      prompt = value;
    },
    getSelectedSkill: () => selectedSkill,
    getActiveConversationId: () => activeConversationId,
    setActiveConversation,
    getChatTitle: () => chatTitle,
    setChatTitle: (value) => {
      chatTitle = value;
    },
    getConversations: () => conversations,
    setKeepChatPinned: (value) => {
      keepChatPinned = value;
    },
    setComposerFocused: (value) => {
      composerFocused = value;
    },
    getQueuedFollowUps: () => queuedFollowUps,
    enqueueFollowUpPrompt,
    runNextQueuedFollowUp,
    upsertFlatConversationMessage,
    rememberCapabilityHandoff,
    rememberSchedulerHandoff,
    rememberPermission,
    rememberEditProposal,
    getStatus: () => status,
    setStatus: (value) => {
      status = value;
    },
    setError: notifyError,
    pushActivity,
    refreshConversations,
    refreshToolRuns
  });
  sendAsk = chatRunController.sendAsk;
  stopGeneration = chatRunController.stopGeneration;
  handleAgentStreamEvent = chatRunController.handleAgentStreamEvent;

  $: syncScrollForMessages();

  $: {
    prompt;
    composerFocused;
    void resizeComposerTextarea();
  }

  async function toggleComposerSetting(kind: 'lowMemory' | 'rag' | 'internet') {
    if (kind === 'lowMemory') settingLowMemory = !settingLowMemory;
    if (kind === 'rag') settingRAGEnabled = !settingRAGEnabled;
    if (kind === 'internet') settingInternetEnabled = !settingInternetEnabled;
    await saveSettings();
  }

  async function setComposerResponseMode(value: string) {
    if (composerResponseModeDisabled) return;
    const next = normalizeResponseMode(value);
    if (settingResponseMode === next) return;
    settingResponseMode = next;
    await saveSettings();
  }

  async function setGeneratedConnectorEnabled(name: string, enabled: boolean) {
    try {
      const result = await setGeneratedConnectorEnabledAction(name, enabled);
      notifySummary(result.message || `${name} ${enabled ? 'enabled' : 'disabled'}`, enabled ? 'info' : 'success');
      const automationSurface = await loadAutomationSurface(100);
      connectorRegistry = automationSurface.connectorRegistry;
      generatedConnectors = automationSurface.generatedConnectors;
    } catch (err) {
      notifyError(err instanceof Error ? err.message : String(err));
    }
  }

  $: groupedToolRuns = buildToolRunGroups(toolRuns);
  let permissionHydrationKey = '';

  function hydratePermissionItemsFromToolRuns() {
    const hydrated = permissionItemsFromToolRuns(toolRuns, activeConversationId, settledPermissionRequestIds, permissionDecisionBusy);
    const hydratedIds = new Set(hydrated.map((item) => item.requestId));
    const liveOnly = permissionItems.filter((item) => item.status === 'pending' && !hydratedIds.has(item.requestId));
    permissionItems = [...hydrated, ...liveOnly].slice(0, 6);
  }

  $: {
    const nextPermissionHydrationKey = [
      activeConversationId,
      toolRuns.map((run) => `${run.id}:${run.toolName}:${run.status}`).join('|'),
      Object.keys(settledPermissionRequestIds).sort().join('|'),
      Object.keys(permissionDecisionBusy).sort().join('|')
    ].join('::');
    if (nextPermissionHydrationKey !== permissionHydrationKey) {
      permissionHydrationKey = nextPermissionHydrationKey;
      hydratePermissionItemsFromToolRuns();
    }
  }

  async function refresh() {
    try {
      const runtimeSettings = await loadRuntimeSettingsSurface();
      status = runtimeSettings.status;
      setupState = runtimeSettings.setupState;
      settings = runtimeSettings.settings;
      syncSettingsForm(settings);
      syncModelRoleForm();
      skills = await listSkillCatalog();
      await loadDomainPackState();
      const extensionSurface = await loadExtensionSurface(100);
      extensions = extensionSurface.extensions;
      extensionFailures = extensionSurface.failures;
      const automationSurface = await loadAutomationSurface(100);
      jobStatus = automationSurface.jobStatus;
      jobs = automationSurface.jobs;
      jobRuns = automationSurface.jobRuns;
      heartbeatReport = automationSurface.heartbeatReport;
      const internetSurface = await loadInternetSurface(100);
      internetStatus = internetSurface.status;
      internetRequests = internetSurface.requests;
      internetCrawls = internetSurface.crawls;
      internetCache = internetSurface.cache;
      connectorRegistry = automationSurface.connectorRegistry;
      generatedConnectors = automationSurface.generatedConnectors;
      const learningSurface = await loadLearningSurface();
      policyStatus = learningSurface.policy;
      learningReport = learningSurface.report;
      await refreshConversations();
      const toolActivity = await loadToolActivity(100);
      toolRuns = toolActivity.toolRuns;
      hydrateActivityFromToolRuns();
      hydratePermissionItemsFromToolRuns();
      toolCatalog = toolActivity.toolCatalog;
      await loadDiagnostics();
      memoryResults = await listMemoryResults(100);
      workspaceGrants = await listWorkspaceGrants();
      documentInventory = await listDocumentInventory(1000);
      const backendSetupMode = setupState.mode || setupState.recommendedMode;
      if (backendSetupMode && !setupModeDirty) {
        setupMode = backendSetupMode;
      }
      if (!setupModel) {
        setupModel = setupState.recommendedModel || setupState.selectedModel;
      }
      if (setupState.installedModels?.length) {
        models = asArray(setupState.installedModels);
      } else if (status.ollamaOk) {
        models = await listRuntimeModels();
      }
      modelProfiles = await listModelProfiles();
    } catch (err) {
      notifyError(err instanceof Error ? err.message : String(err));
    }
  }

  async function refreshToolRuns(silent = false) {
    try {
      const toolActivity = await loadToolActivity(50);
      toolRuns = toolActivity.toolRuns;
      hydrateActivityFromToolRuns();
      hydratePermissionItemsFromToolRuns();
      toolCatalog = toolActivity.toolCatalog;
      await loadDiagnostics();
      if (!silent) pushActivity(`tool catalog: ${toolCatalog.length} entries`);
    } catch (err) {
      if (!silent) notifyError(err instanceof Error ? err.message : String(err));
    }
  }

  async function refreshModelsSurface(silent = false) {
    try {
      const runtimeSettings = await loadRuntimeSettingsSurface();
      status = runtimeSettings.status;
      setupState = runtimeSettings.setupState;
      settings = runtimeSettings.settings;
      if (!silent) {
        syncSettingsForm(settings);
        syncModelRoleForm();
      }
      if (setupState.installedModels?.length) {
        models = asArray(setupState.installedModels);
      } else if (status.ollamaOk) {
        models = await listRuntimeModels();
      } else {
        models = [];
      }
      modelProfiles = await listModelProfiles();
      if (!silent) pushActivity(`models: ${models.length}`);
    } catch (err) {
      if (!silent) notifyError(err instanceof Error ? err.message : String(err));
    }
  }

  async function refreshStartup() {
    await refresh();
    if (activeTab === 'chat' && activeConversationId) {
      await openConversation(activeConversationId);
    }
  }

  onMount(() => {
    weekdayLabel = new Intl.DateTimeFormat(undefined, { weekday: 'long' }).format(new Date());
    sidebarVisible = restoreSidebarVisible();
    restoreStoredNavigation();
    const initialRoute = currentRouteTab(isTab);
    if (initialRoute) {
      setActiveTab(initialRoute);
    } else {
      replaceRouteForTab(activeTab);
    }
    settingTheme = normalizeTheme(localStorage.getItem('yemaka.theme') || settingTheme);
    applyTheme(settingTheme);
    const themeMedia = window.matchMedia?.('(prefers-color-scheme: dark)');
    const handleThemeChange = () => {
      if (settingTheme === 'system') applyTheme(settingTheme);
    };
    themeMedia?.addEventListener?.('change', handleThemeChange);
    const offAsk = window.runtime?.EventsOn?.('yemaka:ask', (event: unknown) => {
      handleAgentStreamEvent(event);
    });
    const stopHashRouter = startHashRouter(isTab, handleHashRoute, () => activeTab);
    const stopLiveRefresh = startVisibleAutoRefresh({
      intervalMs: 9000,
      shouldRefresh: () => Boolean(status || activeConversationId || activeTab !== 'chat'),
      refresh: refreshLiveVisibleData
    });
    void refreshStartup();
    return () => {
      themeMedia?.removeEventListener?.('change', handleThemeChange);
      offAsk?.();
      stopHashRouter();
      stopLiveRefresh();
    };
  });

  onDestroy(() => {
    for (const timeout of toastTimers.values()) {
      window.clearTimeout(timeout);
    }
    toastTimers.clear();
  });
</script>

{#snippet composerCapabilityMenu()}
  <ComposerCapabilityMenu
    bind:composerMenuOpen
    bind:selectedSkill
    bind:composerRouteOpen
    {settingLowMemory}
    {settingRAGEnabled}
    {settingInternetEnabled}
    {settings}
    {status}
    enabledSkills={composerEnabledSkills}
    chatContextItems={composerChatContextItems}
    {skillDisplayLabel}
    {toggleComposerSetting}
    setActiveTab={navigateTab}
  />
{/snippet}

{#snippet queuedFollowUpList()}
  <QueuedFollowUpList
    {queuedFollowUps}
    {promptTitle}
    {skillDisplayLabel}
    {updateQueuedFollowUpDraft}
    {cancelEditQueuedFollowUp}
    {saveQueuedFollowUp}
    {startEditQueuedFollowUp}
    {deleteQueuedFollowUp}
  />
{/snippet}

<main class="h-dvh bg-field text-ink">
  {#if setupState && !setupState.setupComplete}
    <FirstRunSetup
      {setupState}
      bind:setupMode
      bind:setupModeDirty
      bind:setupModel
      {setupBusy}
      {refresh}
      {completeSetup}
    />
  {/if}

  <div class={`grid h-full grid-cols-1 grid-rows-[auto_minmax(0,1fr)] overflow-hidden ${sidebarVisible ? 'lg:grid-cols-[260px_minmax(0,1fr)]' : 'lg:grid-cols-[60px_minmax(0,1fr)]'} lg:grid-rows-1`}>
    <ChatSidebar
      bind:moreOpen
      {sidebarVisible}
      {activeTab}
      {moreTabs}
      {starredConversations}
      {recentConversations}
      {activeConversationId}
      {status}
      {setSidebarVisible}
      newChat={startNewChatRoute}
      openMemorySearch={openMemorySearchRoute}
      selectTab={navigateTab}
      openConversation={openConversationRoute}
      {toggleSidebarConversationStar}
      {renameSidebarConversation}
      {exportSidebarConversationMarkdown}
      {exportSidebarConversationJSON}
      {deleteConversationById}
      {promptTitle}
    />

    <section class="flex min-h-0 min-w-0 flex-col">
      <AppTopbar
        {activeTab}
        {tabs}
        {tabSubtitles}
        {status}
        bind:chatTitle
        bind:renamingChatTitle
        bind:chatMenuOpen
        {conversations}
        {activeConversationId}
        {automationBusy}
        {learningBusy}
        {extensionBusy}
        {domainPackBusy}
        {renameChatTitle}
        {startRenameChatTitle}
        {toggleStarChat}
        {exportActiveConversationMarkdown}
        {exportActiveConversationJSON}
        {deleteChat}
        {refreshActiveTab}
      />

      {#if activeTab === 'chat'}
        <ChatPage
          {messages}
          bind:chatScroll
          {weekdayLabel}
          bind:prompt
          {composerFocused}
          bind:composerTextarea
          {activeConversationId}
          {busy}
          {keepChatPinned}
          {promptChips}
          {queuedFollowUpList}
          {composerCapabilityMenu}
          responseMode={settingResponseMode}
          responseModeOptions={composerResponseModeOptions}
          responseModeDisabled={composerResponseModeDisabled}
          setResponseMode={setComposerResponseMode}
          {editingUserBusy}
          {editingUserMessageIndex}
          bind:editingUserPrompt
          {expandedMessageIndexes}
          {copiedMessageIndex}
          {jobScheduleTypeOptions}
          {extensions}
          {domainPacks}
          {capabilityHandoffBusy}
          {schedulerHandoffBusy}
          {permissionItems}
          {permissionDecisionBusy}
          {activity}
          {groupedToolRuns}
          {expandedToolRunGroups}
          {listPages}
          {handleChatScroll}
          {scrollChatToBottom}
          {focusComposer}
          {blurComposer}
          {resizeComposerTextarea}
          {handleComposerKeydown}
          {sendAsk}
          {stopGeneration}
          {isAssistantActiveMeta}
          {assistantMetaItems}
          {capabilityKindLabel}
          {setSchedulerHandoffInput}
          {setCapabilityRunAfterGenerate}
          {setCapabilityRunInput}
          {setCapabilityScheduleAfterGenerate}
          {setCapabilityScheduleType}
          {setCapabilityScheduleExpr}
          {setCapabilityScheduleEnabled}
          {setCapabilityScheduleInput}
          {openMessageDomainPacks}
          {openMessageCapabilityInExtensions}
          {approveChatCapability}
          {reuseExtension}
          {createSchedulerJobFromChat}
          openAutomation={() => navigateTab('automation')}
          {userMessageVisibleContent}
          {userMessageNeedsCollapse}
          {cancelEditUserMessage}
          {submitEditedUserMessage}
          {toggleUserMessageExpanded}
          {compactSource}
          {userVariantCount}
          {userVariantPosition}
          {assistantVariantCount}
          {assistantVariantPosition}
          {switchUserVariant}
          {switchAssistantVariant}
          {copyMessage}
          {startEditUserMessage}
          {retryPromptForMessage}
          {retryMessage}
          {currentPage}
          {pagedItems}
          {setListPage}
          {permissionCommandPreview}
          {decidePermission}
          {activityKey}
          {activityIcon}
          {activityKind}
          {toggleToolRunGroup}
          {toolRunGroupExpanded}
          {toolRunTimestamp}
          {formatShortTime}
          {formatToolName}
          {formatToolStatus}
        />
      {:else if activeTab === 'models'}
        <ModelsPage
          {models}
          {status}
          bind:modelRole
          bind:modelProvider
          bind:modelBaseURL
          bind:modelName
          bind:modelPrompt
          {modelBusy}
          {modelDetails}
          {modelOutput}
          {modelProfiles}
          bind:modelProfileName
          bind:modelProfileDescription
          bind:modelProfileBaseModel
          bind:modelProfileSystem
          bind:modelProfileTemperature
          bind:modelProfileNumCtx
          bind:modelProfileTags
          bind:modelProfilePurpose
          bind:modelProfileRefinedFrom
          bind:modelProfileConversationId
          {modelProfileResult}
          {modelProfileBusy}
          {modelRoleOptions}
          {runtimeProviders}
          {selectModel}
          {saveModelRole}
          showModelDetails={() => showModelDetails()}
          {generateModelText}
          {refreshModelProfiles}
          {inspectModelProfile}
          {previewModelProfile}
          {draftModelProfileFromConversation}
          {saveModelProfile}
          {applyModelProfile}
          onModelRoleChange={(next) => {
            modelRole = next;
            syncModelRoleForm();
          }}
          onModelProviderChange={(next) => {
            modelProvider = next;
            modelBaseURL = providerDefaultBaseURL(next);
          }}
        />
      {:else if activeTab === 'memory'}
        <MemoryPage
          bind:memoryKind
          bind:memoryImportance
          bind:memoryContent
          bind:memoryQuery
          bind:knowledgeQuery
          bind:knowledgeEntityName
          bind:knowledgeEntityKind
          bind:knowledgeEntitySource
          bind:knowledgeEntitySourceKind
          bind:knowledgeEntitySourceRef
          bind:knowledgeEntityEvidence
          bind:knowledgeFromEntityId
          bind:knowledgeRelation
          bind:knowledgeToEntityId
          bind:knowledgeEdgeSource
          bind:knowledgeEdgeSourceKind
          bind:knowledgeEdgeSourceRef
          bind:knowledgeEdgeEvidence
          bind:knowledgeDraftText
          bind:knowledgeDraftSource
          bind:knowledgeDraftSourceKind
          bind:knowledgeDraftSourceRef
          {memoryResults}
          {knowledgeStatus}
          {knowledgeResults}
          {knowledgeReviewItems}
          {knowledgeProposal}
          {memoryPageStatus}
          {memoryPageStatusKind}
          {memoryKindOptions}
          {listPages}
          {pageSizeFor}
          {currentPage}
          {pagedItems}
          {setListPage}
          {writeMemory}
          listMemories={() => listMemories()}
          {searchMemory}
          {pinMemory}
          {deleteMemory}
          {draftKnowledgeFromMemory}
          {refreshKnowledge}
          {toggleKnowledge}
          {searchKnowledge}
          {repairKnowledgeEvidence}
          {refreshKnowledgeReview}
          {applyKnowledgeReview}
          {draftProposal}
          {useEntityProposal}
          {useRelationProposal}
          {addKnowledgeEntity}
          {addKnowledgeEdge}
        />
      {:else if activeTab === 'documents'}
        <DocumentsPage
          localWeb={isLocalWebPage()}
          {documentFileAccept}
          bind:documentPath
          {documentPathSuggestions}
          bind:documentPathSuggestionOpen
          {documentPathSuggestionSummary}
          {documentPageStatus}
          {documentPageStatusKind}
          {documentInventory}
          {documentInventoryBusy}
          {documentPruneBusy}
          {documentPrunePreview}
          {workspaceGrants}
          bind:documentQuery
          {documentResults}
          {listPages}
          {pageSizeFor}
          {currentPage}
          {pagedItems}
          {setListPage}
          {directoryInput}
          {handleDocumentFileSelected}
          {handleDocumentFolderSelected}
          {scheduleDocumentPathSuggestions}
          {selectDocumentPathSuggestion}
          {pickDocumentFile}
          {pickWorkspaceFolder}
          {grantDocumentPath}
          {ingestDocuments}
          {refreshDocumentInventory}
          {previewPruneMissingDocuments}
          {applyPruneMissingDocuments}
          {refreshWorkspaceGrants}
          {revokeWorkspaceGrant}
          {searchDocuments}
          {draftKnowledgeFromDocument}
          {formatFileSize}
        />
      {:else if activeTab === 'skills'}
        <SkillsPage
          {skills}
          {domainPacks}
          {domainPackReview}
          {domainPackTemplates}
          {domainPackSkills}
          bind:domainPackInstallPath
          {domainPackBusy}
          {skillsPageStatus}
          {skillsPageStatusKind}
          {activeDomainPackHandoff}
          bind:skillSessionId
          bind:skillImproveName
          bind:skillImportPath
          bind:skillExportPath
          {listPages}
          {pageSizeFor}
          {currentPage}
          {pagedItems}
          {setListPage}
          {asArray}
          {humanizeIdentifier}
          {humanizeIdentifierList}
          {createSkillFromSession}
          {improveSkillFromSession}
          {importSkill}
          {exportSkill}
          {refreshSkillsView}
          {installDomainPack}
          {installDomainPackTemplate}
          {reviewDomainPack}
          {reviewDomainPackTemplate}
          closeDomainPackReview={() => {
            domainPackReview = null;
          }}
          {applyDomainPackHandoff}
          {setDomainPackEnabled}
          {uninstallDomainPack}
          {validateSkill}
          {setSkillEnabled}
        />
      {:else if activeTab === 'extensions'}
        <ExtensionsPage
          {extensions}
          {extensionBusy}
          bind:extensionProposalRequest
          bind:extensionGenerateName
          bind:extensionGenerateDescription
          bind:extensionGenerateRun
          bind:extensionGenerateJob
          bind:extensionJobScheduleType
          bind:extensionJobScheduleExpr
          bind:extensionJobEnabled
          bind:extensionJobInputJSON
          bind:extensionRunName
          bind:extensionRunInputJSON
          {extensionRunOutput}
          bind:extensionRollbackTarget
          {extensionFailures}
          {extensionReview}
          {extensionReviewBusy}
          bind:extensionReviewOpenFile
          {capabilityProposal}
          {capabilityNextSteps}
          {jobScheduleTypeOptions}
          {listPages}
          {pageSizeFor}
          {currentPage}
          {pagedItems}
          {setListPage}
          {refreshExtensions}
          {proposeExtension}
          {generateExtension}
          {runExtension}
          {rollbackExtension}
          {testAndRegisterExtension}
          {useReviewedExtensionInRunner}
          {reviewExtension}
          {validateExtension}
          {testExtension}
          {registerExtension}
          {reuseExtension}
          {setExtensionEnabled}
          {deleteExtension}
          {humanizeIdentifier}
        />
      {:else if activeTab === 'automation'}
        <AutomationPage
          {automationBusy}
          {heartbeatReport}
          {jobStatus}
          {internetStatus}
          bind:internetUrl
          bind:internetAllowedDomain
          bind:internetExtractText
          bind:internetTaskApproved
          bind:internetCrawlMaxPages
          bind:internetCrawlMaxDepth
          bind:internetCrawlMaxDurationSeconds
          bind:internetCrawlMaxLinksPerPage
          bind:internetCrawlMaxTextChars
          {internetFetchOutput}
          {internetRequests}
          {internetCrawls}
          {internetCache}
          bind:jobScheduleType
          bind:jobScheduleExpr
          bind:jobTargetName
          bind:jobTargetType
          bind:jobInputJSON
          {jobs}
          {jobRuns}
          {jobFailureDetails}
          {jobScheduleTypeOptions}
          {jobTargetTypeOptions}
          {listPages}
          {pageSizeFor}
          {currentPage}
          {pagedItems}
          {setListPage}
          {refreshAutomation}
          {refreshInternet}
          {runDueJobs}
          {internetFetch}
          {internetCrawl}
          {inspectInternetCrawl}
          {createJob}
          {updateJobInput}
          {runJob}
          {setJobEnabled}
          {archiveJob}
          openConversation={openConversationRoute}
          {humanizeIdentifier}
          {internetSearchReadiness}
          {internetSearchProviderLabel}
        />
      {:else if activeTab === 'learning'}
        <LearningPage
          {learningReport}
          {learningBusy}
          {policyStatus}
          {qaReview}
          bind:correctionConversationId
          bind:correctionContent
          bind:trajectoryConversationId
          {trajectoryPreview}
          {listPages}
          {pageSizeFor}
          {currentPage}
          {pagedItems}
          {setListPage}
          {refreshLearning}
          {refreshDiagnostics}
          {promoteQARegressionSuggestion}
          {reviewQARegressionSuggestion}
          {generateQARegressionTestDraft}
          {approveQARegressionTestDraft}
          {generateQARegressionSourcePatch}
          {approveQARegressionSourcePatchApplyPlan}
          {planQARegressionSourceWrite}
          {applyQARegressionSourceWrite}
          {setPolicyMode}
          {saveCorrection}
          {exportTrajectory}
          {shortId}
        />
      {:else if activeTab === 'tools'}
        <ToolsPage
          bind:fileWritePath
          bind:fileWriteContent
          {fileWritePlan}
          {notifications}
          {notificationBusy}
          {replayTraces}
          {replayExplainBusy}
          {selectedReplayTraceId}
          {replayExplanation}
          {toolCatalog}
          {toolRuns}
          {groupedToolRuns}
          {expandedToolRunGroups}
          {expandedToolRunSessions}
          {listPages}
          {pageSizeFor}
          {currentPage}
          {pagedItems}
          {setListPage}
          {refreshDiagnostics}
          {refreshToolRuns}
          {previewFileWrite}
          {applyFileWrite}
          {markNotificationRead}
          {dismissNotification}
          {explainReplayTrace}
          {clearReplayExplanation}
          {shortId}
          {replayRouteSummary}
          {replayToolSummary}
          {formatSurfaces}
          {formatToolName}
          {formatToolStatus}
          {toggleToolRunGroup}
          {toolRunGroupExpanded}
          {toggleToolRunSession}
          {toolRunSessionExpanded}
          {toolRunTimestamp}
          {formatShortTime}
        />
      {:else}
        <SettingsPage
          {status}
          {settings}
          {connectorRegistry}
          {generatedConnectors}
          {internetStatus}
          bind:settingTheme
          bind:settingLowMemory
          bind:settingContextTokens
          bind:settingResponseMode
          bind:settingShowThinkingTrace
          bind:settingRAGEnabled
          bind:settingEmbeddingsEnabled
          bind:settingEmbeddingModel
          {embeddingIndexBusy}
          bind:settingInternetEnabled
          bind:settingInternetSearchEnabled
          bind:settingInternetSearchProvider
          bind:settingInternetSearchEndpoint
          bind:settingInternetSearchAPIKeyEnv
          bind:settingKnowledgeInfluenceEnabled
          bind:settingCloudEnabled
          bind:settingCloudBaseURL
          bind:settingCloudModel
          bind:settingCloudKeyEnv
          bind:settingConnectorEnabled
          bind:settingConnectorTokenEnv
          bind:settingMCPConnectorEnabled
          bind:settingSlackConnectorEnabled
          bind:settingSlackConnectorTokenEnv
          bind:settingDiscordConnectorEnabled
          bind:settingDiscordConnectorTokenEnv
          bind:settingTelegramConnectorEnabled
          bind:settingTelegramConnectorTokenEnv
          bind:settingEmailConnectorEnabled
          bind:settingEmailConnectorTokenEnv
          {internetSearchProviderSelectOptions}
          {applyTheme}
          {indexEmbeddings}
          {saveSettings}
          {internetSearchProviderOption}
          {onInternetSearchProviderChange}
          {internetSearchReadiness}
          {internetSearchProviderLabel}
          {setGeneratedConnectorEnabled}
        />
      {/if}
    </section>
  </div>
  <YemakaConfirmDialog
    open={confirmDialogOpen}
    title={confirmDialogTitle}
    message={confirmDialogMessage}
    confirmLabel={confirmDialogConfirmLabel}
    cancelLabel={confirmDialogCancelLabel}
    destructive={confirmDialogDestructive}
    onConfirm={() => resolveConfirmDialog(true)}
    onCancel={() => resolveConfirmDialog(false)}
  />
  <ToastCenter {toasts} {dismissToast} />
</main>
