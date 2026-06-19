import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';
import { compile } from 'svelte/compiler';

const root = dirname(dirname(fileURLToPath(import.meta.url)));
const appPath = join(root, 'src', 'App.svelte');
const actionButtonPath = join(root, 'src', 'ActionButton.svelte');
const appTopbarPath = join(root, 'src', 'AppTopbar.svelte');
const badgePath = join(root, 'src', 'Badge.svelte');
const assistantSourceStripPath = join(root, 'src', 'AssistantSourceStrip.svelte');
const capabilityHandoffCardPath = join(root, 'src', 'CapabilityHandoffCard.svelte');
const chatActivityPanelPath = join(root, 'src', 'ChatActivityPanel.svelte');
const chatComposerCardPath = join(root, 'src', 'ChatComposerCard.svelte');
const chatEmptyStatePath = join(root, 'src', 'ChatEmptyState.svelte');
const chatMessageActionsPath = join(root, 'src', 'ChatMessageActions.svelte');
const chatMessageHeaderPath = join(root, 'src', 'ChatMessageHeader.svelte');
const chatMessageItemPath = join(root, 'src', 'ChatMessageItem.svelte');
const chatMessageListPath = join(root, 'src', 'ChatMessageList.svelte');
const chatSidebarPath = join(root, 'src', 'ChatSidebar.svelte');
const chatSurfacePath = join(root, 'src', 'ChatSurface.svelte');
const chatTitleMenuPath = join(root, 'src', 'ChatTitleMenu.svelte');
const agentTimelinePath = join(root, 'src', 'AgentTimeline.svelte');
const markdownPath = join(root, 'src', 'Markdown.svelte');
const composerCapabilityMenuPath = join(root, 'src', 'ComposerCapabilityMenu.svelte');
const firstRunSetupPath = join(root, 'src', 'FirstRunSetup.svelte');
const emptyStatePath = join(root, 'src', 'EmptyState.svelte');
const pageStatusStripPath = join(root, 'src', 'PageStatusStrip.svelte');
const paginationControlsPath = join(root, 'src', 'PaginationControls.svelte');
const queuedFollowUpListPath = join(root, 'src', 'QueuedFollowUpList.svelte');
const schedulerHandoffCardPath = join(root, 'src', 'SchedulerHandoffCard.svelte');
const sectionHeaderPath = join(root, 'src', 'SectionHeader.svelte');
const structuredDataViewPath = join(root, 'src', 'StructuredDataView.svelte');
const toastCenterPath = join(root, 'src', 'ToastCenter.svelte');
const yemakaConfirmDialogPath = join(root, 'src', 'YemakaConfirmDialog.svelte');
const activityControllerPath = join(root, 'src', 'lib', 'activityController.ts');
const activityHelpersPath = join(root, 'src', 'lib', 'activityHelpers.ts');
const agentStreamEventHelpersPath = join(root, 'src', 'lib', 'agentStreamEventHelpers.ts');
const automationActionsPath = join(root, 'src', 'lib', 'automationActions.ts');
const automationControllerPath = join(root, 'src', 'lib', 'automationController.ts');
const appOptionsPath = join(root, 'src', 'lib', 'appOptions.ts');
const appRefreshControllerPath = join(root, 'src', 'lib', 'appRefreshController.ts');
const appTypesPath = join(root, 'src', 'lib', 'appTypes.ts');
const apiPath = join(root, 'src', 'lib', 'api.ts');
const capabilityGenerationHelpersPath = join(root, 'src', 'lib', 'capabilityGenerationHelpers.ts');
const chatActionsPath = join(root, 'src', 'lib', 'chatActions.ts');
const chatDisplayHelpersPath = join(root, 'src', 'lib', 'chatDisplayHelpers.ts');
const chatHandoffControllerPath = join(root, 'src', 'lib', 'chatHandoffController.ts');
const chatInteractionControllerPath = join(root, 'src', 'lib', 'chatInteractionController.ts');
const chatResultHelpersPath = join(root, 'src', 'lib', 'chatResultHelpers.ts');
const chatRunControllerPath = join(root, 'src', 'lib', 'chatRunController.ts');
const chatStateHelpersPath = join(root, 'src', 'lib', 'chatStateHelpers.ts');
const chatSubmitHelpersPath = join(root, 'src', 'lib', 'chatSubmitHelpers.ts');
const conversationActionsPath = join(root, 'src', 'lib', 'conversationActions.ts');
const conversationControllerPath = join(root, 'src', 'lib', 'conversationController.ts');
const conversationExportPath = join(root, 'src', 'lib', 'conversationExport.ts');
const conversationHelpersPath = join(root, 'src', 'lib', 'conversationHelpers.ts');
const documentActionsPath = join(root, 'src', 'lib', 'documentActions.ts');
const documentControllerPath = join(root, 'src', 'lib', 'documentController.ts');
const documentHelpersPath = join(root, 'src', 'lib', 'documentHelpers.ts');
const documentPageHelpersPath = join(root, 'src', 'lib', 'documentPageHelpers.ts');
const diagnosticActionsPath = join(root, 'src', 'lib', 'diagnosticActions.ts');
const diagnosticsControllerPath = join(root, 'src', 'lib', 'diagnosticsController.ts');
const diagnosticHelpersPath = join(root, 'src', 'lib', 'diagnosticHelpers.ts');
const domainPackHelpersPath = join(root, 'src', 'lib', 'domainPackHelpers.ts');
const extensionActionsPath = join(root, 'src', 'lib', 'extensionActions.ts');
const extensionsControllerPath = join(root, 'src', 'lib', 'extensionsController.ts');
const extensionPageHelpersPath = join(root, 'src', 'lib', 'extensionPageHelpers.ts');
const handoffHelpersPath = join(root, 'src', 'lib', 'handoffHelpers.ts');
const statusHelpersPath = join(root, 'src', 'lib', 'statusHelpers.ts');
const themeHelpersPath = join(root, 'src', 'lib', 'themeHelpers.ts');
const toastCenterHelpersPath = join(root, 'src', 'lib', 'toastCenter.ts');
const internetControllerPath = join(root, 'src', 'lib', 'internetController.ts');
const internetActionsPath = join(root, 'src', 'lib', 'internetActions.ts');
const learningActionsPath = join(root, 'src', 'lib', 'learningActions.ts');
const learningControllerPath = join(root, 'src', 'lib', 'learningController.ts');
const knowledgeActionsPath = join(root, 'src', 'lib', 'knowledgeActions.ts');
const knowledgeControllerPath = join(root, 'src', 'lib', 'knowledgeController.ts');
const listExpansionControllerPath = join(root, 'src', 'lib', 'listExpansionController.ts');
const memoryActionsPath = join(root, 'src', 'lib', 'memoryActions.ts');
const memoryControllerPath = join(root, 'src', 'lib', 'memoryController.ts');
const modelControllerPath = join(root, 'src', 'lib', 'modelController.ts');
const modelHelpersPath = join(root, 'src', 'lib', 'modelHelpers.ts');
const modelActionsPath = join(root, 'src', 'lib', 'modelActions.ts');
const navigationHelpersPath = join(root, 'src', 'lib', 'navigationHelpers.ts');
const pageRefreshHelpersPath = join(root, 'src', 'lib', 'pageRefreshHelpers.ts');
const permissionControllerPath = join(root, 'src', 'lib', 'permissionController.ts');
const permissionStreamHelpersPath = join(root, 'src', 'lib', 'permissionStreamHelpers.ts');
const schedulerJobHelpersPath = join(root, 'src', 'lib', 'schedulerJobHelpers.ts');
const settingsActionsPath = join(root, 'src', 'lib', 'settingsActions.ts');
const settingsControllerPath = join(root, 'src', 'lib', 'settingsController.ts');
const settingsFormHelpersPath = join(root, 'src', 'lib', 'settingsFormHelpers.ts');
const skillActionsPath = join(root, 'src', 'lib', 'skillActions.ts');
const skillsControllerPath = join(root, 'src', 'lib', 'skillsController.ts');
const toolRunHelpersPath = join(root, 'src', 'lib', 'toolRunHelpers.ts');
const routerPath = join(root, 'src', 'lib', 'router.ts');
const structuredDisplayPath = join(root, 'src', 'lib', 'structuredDisplay.ts');
const uiHelpersPath = join(root, 'src', 'lib', 'uiHelpers.ts');
const automationPagePath = join(root, 'src', 'pages', 'AutomationPage.svelte');
const chatPagePath = join(root, 'src', 'pages', 'ChatPage.svelte');
const documentsPagePath = join(root, 'src', 'pages', 'DocumentsPage.svelte');
const extensionsPagePath = join(root, 'src', 'pages', 'ExtensionsPage.svelte');
const learningPagePath = join(root, 'src', 'pages', 'LearningPage.svelte');
const modelsPagePath = join(root, 'src', 'pages', 'ModelsPage.svelte');
const memoryPagePath = join(root, 'src', 'pages', 'MemoryPage.svelte');
const settingsPagePath = join(root, 'src', 'pages', 'SettingsPage.svelte');
const skillsPagePath = join(root, 'src', 'pages', 'SkillsPage.svelte');
const toolsPagePath = join(root, 'src', 'pages', 'ToolsPage.svelte');
const userMessageBodyPath = join(root, 'src', 'UserMessageBody.svelte');
const uiGuidelinesPath = join(root, '..', 'docs', 'ui-guidelines.md');
const wailsDesktopDTSPath = join(root, 'wailsjs', 'go', 'desktop', 'App.d.ts');
const wailsDesktopJSPath = join(root, 'wailsjs', 'go', 'desktop', 'App.js');
const app = readFileSync(appPath, 'utf8');
const actionButton = readFileSync(actionButtonPath, 'utf8');
const appTopbar = readFileSync(appTopbarPath, 'utf8');
const badge = readFileSync(badgePath, 'utf8');
const assistantSourceStrip = readFileSync(assistantSourceStripPath, 'utf8');
const capabilityHandoffCard = readFileSync(capabilityHandoffCardPath, 'utf8');
const chatActivityPanel = readFileSync(chatActivityPanelPath, 'utf8');
const chatComposerCard = readFileSync(chatComposerCardPath, 'utf8');
const chatEmptyState = readFileSync(chatEmptyStatePath, 'utf8');
const chatMessageActions = readFileSync(chatMessageActionsPath, 'utf8');
const chatMessageHeader = readFileSync(chatMessageHeaderPath, 'utf8');
const chatMessageItem = readFileSync(chatMessageItemPath, 'utf8');
const chatMessageList = readFileSync(chatMessageListPath, 'utf8');
const chatSidebar = readFileSync(chatSidebarPath, 'utf8');
const chatSurface = readFileSync(chatSurfacePath, 'utf8');
const chatTitleMenu = readFileSync(chatTitleMenuPath, 'utf8');
const agentTimeline = readFileSync(agentTimelinePath, 'utf8');
const markdown = readFileSync(markdownPath, 'utf8');
const composerCapabilityMenu = readFileSync(composerCapabilityMenuPath, 'utf8');
const firstRunSetup = readFileSync(firstRunSetupPath, 'utf8');
const emptyState = readFileSync(emptyStatePath, 'utf8');
const pageStatusStrip = readFileSync(pageStatusStripPath, 'utf8');
const paginationControls = readFileSync(paginationControlsPath, 'utf8');
const queuedFollowUpList = readFileSync(queuedFollowUpListPath, 'utf8');
const schedulerHandoffCard = readFileSync(schedulerHandoffCardPath, 'utf8');
const sectionHeader = readFileSync(sectionHeaderPath, 'utf8');
const structuredDataView = readFileSync(structuredDataViewPath, 'utf8');
const toastCenter = readFileSync(toastCenterPath, 'utf8');
const yemakaConfirmDialog = readFileSync(yemakaConfirmDialogPath, 'utf8');
const activityController = readFileSync(activityControllerPath, 'utf8');
const activityHelpers = readFileSync(activityHelpersPath, 'utf8');
const agentStreamEventHelpers = readFileSync(agentStreamEventHelpersPath, 'utf8');
const automationActions = readFileSync(automationActionsPath, 'utf8');
const automationController = readFileSync(automationControllerPath, 'utf8');
const appOptions = readFileSync(appOptionsPath, 'utf8');
const appRefreshController = readFileSync(appRefreshControllerPath, 'utf8');
const appTypes = readFileSync(appTypesPath, 'utf8');
const api = readFileSync(apiPath, 'utf8');
const capabilityGenerationHelpers = readFileSync(capabilityGenerationHelpersPath, 'utf8');
const chatActions = readFileSync(chatActionsPath, 'utf8');
const chatDisplayHelpers = readFileSync(chatDisplayHelpersPath, 'utf8');
const chatHandoffController = readFileSync(chatHandoffControllerPath, 'utf8');
const chatInteractionController = readFileSync(chatInteractionControllerPath, 'utf8');
const chatResultHelpers = readFileSync(chatResultHelpersPath, 'utf8');
const chatRunController = readFileSync(chatRunControllerPath, 'utf8');
const chatStateHelpers = readFileSync(chatStateHelpersPath, 'utf8');
const chatSubmitHelpers = readFileSync(chatSubmitHelpersPath, 'utf8');
const conversationActions = readFileSync(conversationActionsPath, 'utf8');
const conversationController = readFileSync(conversationControllerPath, 'utf8');
const conversationExport = readFileSync(conversationExportPath, 'utf8');
const conversationHelpers = readFileSync(conversationHelpersPath, 'utf8');
const documentActions = readFileSync(documentActionsPath, 'utf8');
const documentController = readFileSync(documentControllerPath, 'utf8');
const documentHelpers = readFileSync(documentHelpersPath, 'utf8');
const documentPageHelpers = readFileSync(documentPageHelpersPath, 'utf8');
const diagnosticActions = readFileSync(diagnosticActionsPath, 'utf8');
const diagnosticsController = readFileSync(diagnosticsControllerPath, 'utf8');
const diagnosticHelpers = readFileSync(diagnosticHelpersPath, 'utf8');
const domainPackHelpers = readFileSync(domainPackHelpersPath, 'utf8');
const extensionActions = readFileSync(extensionActionsPath, 'utf8');
const extensionsController = readFileSync(extensionsControllerPath, 'utf8');
const extensionPageHelpers = readFileSync(extensionPageHelpersPath, 'utf8');
const handoffHelpers = readFileSync(handoffHelpersPath, 'utf8');
const statusHelpers = readFileSync(statusHelpersPath, 'utf8');
const themeHelpers = readFileSync(themeHelpersPath, 'utf8');
const toastCenterHelpers = readFileSync(toastCenterHelpersPath, 'utf8');
const internetController = readFileSync(internetControllerPath, 'utf8');
const internetActions = readFileSync(internetActionsPath, 'utf8');
const learningActions = readFileSync(learningActionsPath, 'utf8');
const learningController = readFileSync(learningControllerPath, 'utf8');
const knowledgeActions = readFileSync(knowledgeActionsPath, 'utf8');
const knowledgeController = readFileSync(knowledgeControllerPath, 'utf8');
const listExpansionController = readFileSync(listExpansionControllerPath, 'utf8');
const memoryActions = readFileSync(memoryActionsPath, 'utf8');
const memoryController = readFileSync(memoryControllerPath, 'utf8');
const modelController = readFileSync(modelControllerPath, 'utf8');
const modelHelpers = readFileSync(modelHelpersPath, 'utf8');
const modelActions = readFileSync(modelActionsPath, 'utf8');
const navigationHelpers = readFileSync(navigationHelpersPath, 'utf8');
const pageRefreshHelpers = readFileSync(pageRefreshHelpersPath, 'utf8');
const permissionController = readFileSync(permissionControllerPath, 'utf8');
const permissionStreamHelpers = readFileSync(permissionStreamHelpersPath, 'utf8');
const schedulerJobHelpers = readFileSync(schedulerJobHelpersPath, 'utf8');
const settingsActions = readFileSync(settingsActionsPath, 'utf8');
const settingsController = readFileSync(settingsControllerPath, 'utf8');
const settingsFormHelpers = readFileSync(settingsFormHelpersPath, 'utf8');
const skillActions = readFileSync(skillActionsPath, 'utf8');
const skillsController = readFileSync(skillsControllerPath, 'utf8');
const toolRunHelpers = readFileSync(toolRunHelpersPath, 'utf8');
const router = readFileSync(routerPath, 'utf8');
const structuredDisplay = readFileSync(structuredDisplayPath, 'utf8');
const uiHelpers = readFileSync(uiHelpersPath, 'utf8');
const automationPage = readFileSync(automationPagePath, 'utf8');
const chatPage = readFileSync(chatPagePath, 'utf8');
const documentsPage = readFileSync(documentsPagePath, 'utf8');
const extensionsPage = readFileSync(extensionsPagePath, 'utf8');
const learningPage = readFileSync(learningPagePath, 'utf8');
const modelsPage = readFileSync(modelsPagePath, 'utf8');
const memoryPage = readFileSync(memoryPagePath, 'utf8');
const settingsPage = readFileSync(settingsPagePath, 'utf8');
const skillsPage = readFileSync(skillsPagePath, 'utf8');
const toolsPage = readFileSync(toolsPagePath, 'utf8');
const userMessageBody = readFileSync(userMessageBodyPath, 'utf8');
const uiGuidelines = readFileSync(uiGuidelinesPath, 'utf8');
const wailsDesktopDTS = readFileSync(wailsDesktopDTSPath, 'utf8');
const wailsDesktopJS = readFileSync(wailsDesktopJSPath, 'utf8');

const failures = [];
const routeOrder = ['chat', 'models', 'memory', 'documents', 'skills', 'extensions', 'automation', 'learning', 'tools'];

function fail(label, detail) {
  failures.push(`${label}: ${detail}`);
}

function includes(label, marker, source = app) {
  if (!source.includes(marker)) fail(label, `missing "${marker}"`);
}

function matches(label, pattern, source = app) {
  if (!pattern.test(source)) fail(label, `missing pattern ${pattern}`);
}

function routeMarker(tab) {
  return tab === 'chat' ? "{#if activeTab === 'chat'}" : `{:else if activeTab === '${tab}'}`;
}

function routeSection(tab) {
  let start = -1;
  if (tab === 'chat') {
    const routeAnchor = app.indexOf('{#if error}');
    start = app.indexOf(routeMarker(tab), routeAnchor);
  } else {
    start = app.indexOf(routeMarker(tab));
  }
  if (start < 0) return '';

  const nextMarkers = routeOrder.slice(routeOrder.indexOf(tab) + 1).map(routeMarker);
  const end = Math.min(
    ...nextMarkers
      .map((marker) => app.indexOf(marker, start + routeMarker(tab).length))
      .filter((index) => index >= 0)
  );
  return app.slice(start, Number.isFinite(end) ? end : app.length);
}

for (const [name, source] of [
  ['App.svelte', app],
  ['AppTopbar.svelte', appTopbar],
  ['AutomationPage.svelte', automationPage],
  ['DocumentsPage.svelte', documentsPage],
  ['ExtensionsPage.svelte', extensionsPage],
  ['LearningPage.svelte', learningPage],
  ['SettingsPage.svelte', settingsPage],
  ['SkillsPage.svelte', skillsPage],
  ['ToolsPage.svelte', toolsPage],
  ['YemakaConfirmDialog.svelte', yemakaConfirmDialog]
]) {
  if (source.includes('window.confirm(') || source.includes('window.alert(') || source.includes('alert(')) {
    fail('enterprise confirm dialog', `native browser modal call is still present in ${name}`);
  }
}

try {
  compile(app, { filename: appPath, generate: 'client' });
  compile(actionButton, { filename: actionButtonPath, generate: 'client' });
  compile(appTopbar, { filename: appTopbarPath, generate: 'client' });
  compile(badge, { filename: badgePath, generate: 'client' });
  compile(assistantSourceStrip, { filename: assistantSourceStripPath, generate: 'client' });
  compile(capabilityHandoffCard, { filename: capabilityHandoffCardPath, generate: 'client' });
  compile(chatActivityPanel, { filename: chatActivityPanelPath, generate: 'client' });
  compile(chatComposerCard, { filename: chatComposerCardPath, generate: 'client' });
  compile(chatEmptyState, { filename: chatEmptyStatePath, generate: 'client' });
  compile(chatMessageActions, { filename: chatMessageActionsPath, generate: 'client' });
  compile(chatMessageHeader, { filename: chatMessageHeaderPath, generate: 'client' });
  compile(chatMessageItem, { filename: chatMessageItemPath, generate: 'client' });
  compile(chatMessageList, { filename: chatMessageListPath, generate: 'client' });
  compile(chatSidebar, { filename: chatSidebarPath, generate: 'client' });
  compile(chatSurface, { filename: chatSurfacePath, generate: 'client' });
  compile(chatTitleMenu, { filename: chatTitleMenuPath, generate: 'client' });
  compile(composerCapabilityMenu, { filename: composerCapabilityMenuPath, generate: 'client' });
  compile(firstRunSetup, { filename: firstRunSetupPath, generate: 'client' });
  compile(emptyState, { filename: emptyStatePath, generate: 'client' });
  compile(pageStatusStrip, { filename: pageStatusStripPath, generate: 'client' });
  compile(paginationControls, { filename: paginationControlsPath, generate: 'client' });
  compile(queuedFollowUpList, { filename: queuedFollowUpListPath, generate: 'client' });
  compile(schedulerHandoffCard, { filename: schedulerHandoffCardPath, generate: 'client' });
  compile(sectionHeader, { filename: sectionHeaderPath, generate: 'client' });
  compile(structuredDataView, { filename: structuredDataViewPath, generate: 'client' });
  compile(toastCenter, { filename: toastCenterPath, generate: 'client' });
  compile(yemakaConfirmDialog, { filename: yemakaConfirmDialogPath, generate: 'client' });
  compile(automationPage, { filename: automationPagePath, generate: 'client' });
  compile(chatPage, { filename: chatPagePath, generate: 'client' });
  compile(documentsPage, { filename: documentsPagePath, generate: 'client' });
  compile(extensionsPage, { filename: extensionsPagePath, generate: 'client' });
  compile(learningPage, { filename: learningPagePath, generate: 'client' });
  compile(modelsPage, { filename: modelsPagePath, generate: 'client' });
  compile(memoryPage, { filename: memoryPagePath, generate: 'client' });
  compile(settingsPage, { filename: settingsPagePath, generate: 'client' });
  compile(skillsPage, { filename: skillsPagePath, generate: 'client' });
  compile(toolsPage, { filename: toolsPagePath, generate: 'client' });
  compile(userMessageBody, { filename: userMessageBodyPath, generate: 'client' });
} catch (error) {
  fail('Svelte compile', error?.message || String(error));
}

for (const marker of [
  "id: 'chat', label: 'Chat'",
  "id: 'memory', label: 'Memory'",
  "id: 'extensions', label: 'Extensions'",
  "id: 'automation', label: 'Automation'",
  "id: 'settings', label: 'Settings'"
]) {
  includes('tab registry', marker, appOptions);
}

for (const marker of [
  "import YemakaConfirmDialog from './YemakaConfirmDialog.svelte'",
  "import ToastCenter from './ToastCenter.svelte'",
  'function confirmYemaka',
  'function pushToast',
  'function notifySummary',
  'function notifyError',
  "'Delete conversation?'",
  "deleteAction ? 'Delete'",
  '<YemakaConfirmDialog'
]) {
  includes('enterprise confirm dialog', marker, app);
}
for (const marker of [
  'export function toastKindForMessage',
  'renamed|deleted|starred|unstarred',
  'export function toastIcon',
  'export function toastTimeout'
]) {
  includes('toast center helper extraction', marker, toastCenterHelpers);
}
includes('toast center app wiring', '<ToastCenter {toasts} {dismissToast} />', app);
for (const marker of [
  'page-status-strip',
  'role={kind ===',
  'toastIcon(kind)',
  'page-status-action'
]) {
  includes('page status strip primitive', marker, pageStatusStrip);
}
for (const marker of [
  'empty-state',
  'empty-state-icon',
  'empty-state-action'
]) {
  includes('empty state primitive', marker, emptyState);
}
for (const marker of [
  'action-button',
  'disabledReason',
  'aria-busy={busy || undefined}',
  'export function focus()'
]) {
  includes('action button primitive', marker, actionButton);
}
for (const marker of [
  'ui-badge',
  'ui-badge-label',
  "variant: 'neutral' | 'success' | 'warning' | 'danger' | 'info'",
  '<slot />'
]) {
  includes('badge primitive', marker, badge);
}
for (const marker of [
  'section-header',
  'section-header-icon',
  'section-header-description',
  '<slot name="actions" />'
]) {
  includes('section header primitive', marker, sectionHeader);
}
for (const marker of [
  'toast-center',
  'Yemaka notifications',
  'toast-card',
  'dismissToast(toast.id)'
]) {
  includes('toast center', marker, toastCenter);
}
for (const marker of [
  'class="yemaka-confirm-card"',
  'role="dialog"',
  'aria-modal="true"',
  '<ActionButton',
  "variant={destructive ? 'danger' : 'primary'}",
  '<svelte:window onkeydown={handleKeydown} />'
]) {
  includes('enterprise confirm dialog', marker, yemakaConfirmDialog);
}
for (const marker of [
  'ActionButton.svelte',
  'Badge.svelte',
  'SectionHeader.svelte',
  'PageStatusStrip.svelte',
  'ToastCenter.svelte',
  'YemakaConfirmDialog.svelte',
  'StructuredDataView.svelte',
  'Avoid:',
  'native `window.alert`, `window.confirm`, or browser prompt calls',
  'raw JSON/YAML `<pre>` blocks',
  'Browser-owned security prompts'
]) {
  includes('ui guidelines doc', marker, uiGuidelines);
}
for (const marker of [
  'formatStructuredData',
  'humanizeStructuredKey',
  "export type StructuredDataLanguage = 'auto' | 'json' | 'yaml' | 'text'",
  'looksLikeYaml',
  'tokenizeLines'
]) {
  includes('structured data helper', marker, structuredDisplay);
}
for (const marker of [
  'formatStructuredData(value, language, filename)',
  'Raw',
  'Copy',
  'More',
  'structured-data-view',
  'structured-token-${part.kind}'
]) {
  includes('structured data view', marker, structuredDataView);
}

includes('empty API data guard', "return asArray(await call<ConversationSession[]>('ListConversations', limit));", conversationActions);
includes('empty API data guard', "jobs: asArray(await call<Job[]>('ListJobs'))", automationActions);
includes('empty API data guard', "jobRuns: asArray(await call<JobRun[]>('JobRuns', runLimit))", automationActions);
includes('empty API data guard', "extensions: asArray(await call<Extension[]>('ListExtensions'))", extensionActions);
includes('empty API data guard', "failures: asArray(await call<ExtensionFailureTrend[]>('ExtensionFailures', failureLimit))", extensionActions);
includes('empty API data guard', "return asArray(await call<MemoryResult[]>('ListMemories', limit));", memoryActions);
includes('empty API data guard', "return asArray(await call<DocumentInventoryItem[]>('ListDocumentInventory', limit));", documentActions);
includes('empty API data guard', "toolRuns: asArray(await call<ToolRun[]>('ListToolRuns', limit))", diagnosticActions);
includes('empty API data guard', "toolCatalog: asArray(await call<ToolCatalogEntry[]>('ToolCatalog', 'chat-callable'))", diagnosticActions);
includes('empty API data guard', "notifications: asArray(await call<NotificationItem[]>('ListNotifications', limit, false))", diagnosticActions);
includes('empty API data guard', "replayTraces: asArray(await call<ReplayTraceFile[]>('ListReplayTraces', limit))", diagnosticActions);
for (const marker of [
  "packs: asArray(await call<DomainPack[]>('ListDomainPacks'))",
  "templates: asArray(await call<DomainPackTemplate[]>('ListDomainPackTemplates'))",
  "skills: asArray(await call<DomainPackSkillStatus[]>('ListDomainPackSkills'))"
]) {
  includes('empty API data guard', marker, skillActions);
}
for (const marker of [
  'export function tabRouteHash(tab: Tab)',
  'export function currentRouteTab(isTab: IsTab)',
  'export function navigateRouteForTab(tab: Tab)',
  'export function startHashRouter'
]) {
  includes('hash route helper', marker, router);
}
for (const marker of [
  'currentRouteTab(isTab)',
  'replaceRouteForTab(activeTab)',
  'startHashRouter(isTab, handleHashRoute',
  'selectTab={navigateTab}',
  "openAutomation={() => navigateTab('automation')}"
]) {
  includes('hash route wiring', marker, app);
}
includes('chat retry edit uses live sendAsk', 'sendAsk: (contentOverride, options) => sendAsk(contentOverride, options)', app);
for (const marker of [
  "export type RefreshTarget = 'chat'",
  "if (tab === 'chat') return 'chat';"
]) {
  includes('chat route refresh target', marker, pageRefreshHelpers);
}
for (const marker of [
  "if (target === 'chat')",
  'await ctx.openConversation(ctx.getActiveConversationId(), { silent: true });'
]) {
  includes('chat route refresh controller', marker, appRefreshController);
}
for (const marker of [
  'export function asArray<T>(value: T[] | null | undefined): T[]',
  'return Array.isArray(value) ? value : [];',
  'export function previewJSON(value: unknown)',
  'export function humanizeIdentifier(value: string | undefined',
  'export function humanizeIdentifierList(values: string[] | null | undefined',
  ".replace(/([a-z0-9])([A-Z])/g, '$1 $2')",
  'export function userMessageNeedsCollapse(content: string)',
  'export function userMessageVisibleContent(content: string, expanded: boolean)',
  'export function promptTitle(value: string)',
  'export function sourceKindLabel(kind: string | undefined)',
  'export function isAssistantActiveMeta(meta: string | undefined)',
  'export function pageSizeFor(key: string)',
  'export function pageCount(total: number, size: number)',
  'export function currentPage(key: string, total: number',
  'export function pagedItems<T>'
]) {
  includes('ui helper extraction', marker, uiHelpers);
}
for (const marker of [
  'export function healthBadge(statusValue =',
  'export function internetSearchReadiness(statusValue: InternetStatusLike | null)',
  "detail.includes('api key')"
]) {
  includes('status helper extraction', marker, statusHelpers);
}
for (const marker of [
  'export async function loadInternetSurface',
  'export function allowedDomainsFromInput',
  'export async function fetchInternet',
  'export function internetFetchPreview',
  'export function internetActivityLabel'
]) {
  includes('internet action extraction', marker, internetActions);
}
for (const marker of [
  'export function createInternetController',
  'function internetSearchProviderLabel',
  'async function refreshInternet',
  "async function internetFetch(method: 'GET' | 'HEAD')",
  'allowedDomainsFromInput(ctx.getAllowedDomain())',
  'ctx.setFetchOutput(previewJSON(internetFetchPreview(result)))',
  'return {'
]) {
  includes('internet controller extraction', marker, internetController);
}
for (const marker of [
  'export async function loadLearningSurface',
  'export async function setLearningPolicyMode',
  'export async function saveRouteCorrection',
  'export async function exportConversationTrajectory',
  'export function learningReportActivity',
  'export function policyModeSummary',
  'export function trajectorySummary'
]) {
  includes('learning action extraction', marker, learningActions);
}
for (const marker of [
  'export function createLearningController',
  'async function refreshLearning',
  'async function setPolicyMode',
  'async function saveCorrection',
  'async function exportTrajectory',
  'await ctx.loadDiagnostics()',
  'listExtensionFailures(20)',
  'trajectorySummary(trajectory)',
  'return {'
]) {
  includes('learning controller extraction', marker, learningController);
}
for (const marker of [
  'export async function askOnce',
  'attachmentIds: string[] = []',
  "return await call<AskResult>('Ask', content, skill, conversationId, parentMessageId, attachmentIds);",
  'export async function uploadChatAttachment'
]) {
  includes('chat action extraction', marker, chatActions);
}
for (const marker of [
  'export type ChatAttachment',
  'attachmentIds?: string[]',
  'attachments?: ChatAttachment[]'
]) {
  includes('chat attachment types', marker, appTypes);
}
for (const marker of [
  'uploadChatAttachment(file)',
  'composer-attach-button',
  'hasUploadingAttachments',
  'conversationKey',
  'attachmentIds.length > 0 ? { attachmentIds, attachments: attachmentItems }'
]) {
  includes('chat attachment composer', marker, chatComposerCard);
}
for (const marker of [
  'message-attachment-strip',
  'attachmentSizeLabel(attachment.sizeBytes)'
]) {
  includes('chat attachment message display', marker, userMessageBody);
}
for (const marker of [
  "case 'UploadChatAttachment':",
  "path = '/api/chat/attachments'",
  'attachmentIds: Array.isArray(args[4]) ? args[4] : []'
]) {
  includes('chat attachment API wiring', marker, api);
}
for (const marker of [
  'export function UploadChatAttachment(arg1:desktop.ChatAttachmentUploadInput):Promise<desktop.ChatAttachmentUploadResult>',
  "export function UploadChatAttachment(arg1) {"
]) {
  includes('chat attachment desktop wiring', marker, `${wailsDesktopDTS}\n${wailsDesktopJS}`);
}
for (const marker of [
  'export function visibleTextLength(value: string)',
  'export function normalizedAssistantTraceLine(line: string)',
  'export function appendAssistantTraceLine(message: ChatMessage',
  'export function assistantResultMessage(result: AskResult'
]) {
  includes('chat result helper extraction', marker, chatResultHelpers);
}
for (const marker of [
  'export function askContentFromInput',
  'export function askSkillForOptions',
  'export function shouldAppendAskUser',
  'export function appendWorkingAskMessages',
  'export function markSavedUserMessage',
  'export function flatUserMessageFromAskResult',
  'export function flatAssistantMessageFromAskResult',
  'export function chatTitleFromConversations',
  'export function messagesAfterAskError',
  'export function streamTransportActivity'
]) {
  includes('chat submit helper extraction', marker, chatSubmitHelpers);
}
for (const marker of [
  'export function assistantIndexForPrompt',
  'export function retryPromptForMessage',
  'export function retryContextForMessage',
  'export function hasActiveAssistantRun',
  'export function streamTargetIndexFor',
  'export function updateAssistantMessageList',
  'export function switchAssistantVariantList',
  'export function switchUserVariantList',
  'export function chatMessageSnapshot',
  'export function prepareRetryAssistantState',
  'sources: []',
  'trace: []',
  'capabilityHandoff: undefined',
  'schedulerHandoff: undefined',
  'export function removeLatestAssistantVariant',
  'export function queuedFollowUpId',
  'export function createQueuedFollowUp',
  'export function startQueuedFollowUpEdit',
  'export function updateQueuedFollowUpDraft',
  'export function saveQueuedFollowUpDraft',
  'export function cancelQueuedFollowUpEdit',
  'export function deleteQueuedFollowUpItem'
]) {
  includes('chat state helper extraction', marker, chatStateHelpers);
}
for (const marker of [
  'export function createChatInteractionController',
  'function streamTargetIndex',
  'function updateAssistantMessageAt',
  'function switchAssistantVariant',
  'function switchUserVariant',
  'function handleChatScroll',
  'async function scrollChatToBottom',
  'function syncScrollForMessages',
  'function assistantMetaItems',
  'function toggleUserMessageExpanded',
  'function resetUserMessageEdit',
  'function startEditUserMessage',
  'async function submitEditedUserMessage',
  'async function copyMessage',
  'async function retryMessage',
  'function enqueueFollowUpPrompt',
  'async function runNextQueuedFollowUp',
  'async function resizeComposerTextarea',
  'function handleComposerKeydown',
  'function prepareRetryAssistant',
  'return {'
]) {
  includes('chat interaction controller extraction', marker, chatInteractionController);
}
for (const marker of [
  'export function createChatRunController',
  'function appendAssistantTrace',
  'async function waitForPaint',
  'async function applyAssistantResult',
  'function handleAgentStreamEvent',
  'async function sendAsk',
  'async function stopGeneration',
  'shouldUseHTTPAskStream()',
  'streamAskHTTP({',
  'attachmentIds,',
  'stopActiveAskHTTP()',
  'ctx.getStreamingMessageIndex() >= 0 || !ctx.getActiveConversationId()',
  'askOnce(content, askSkill',
  'setActiveRequestSignal(controller.signal)',
  'setActiveRequestSignal(null)',
  'ctx.enqueueFollowUpPrompt(content, askSkill)',
  'ctx.runNextQueuedFollowUp()',
  'return {'
]) {
  includes('chat run controller extraction', marker, chatRunController);
}
includes('local web ask stop API', "fetch('/api/ask/stop'", api);
for (const marker of [
  'export function assistantMetaItems(message: ChatMessage)',
  'export function chatContextItems(options: ChatContextOptions)',
  "label: status?.lowMemoryMode ? 'low memory' : 'standard'",
  "label: internetOn ? 'internet on' : 'internet off'"
]) {
  includes('chat display helper extraction', marker, chatDisplayHelpers);
}
for (const marker of [
  'export function activityKey(item: ActivityKeyItem, index: number)',
  'export function activityLabel(line: string)',
  'export function activityTimeLabel(date = new Date())',
  'export function createActivityEntry(line: string',
  'export function prependActivityEntry',
  'export function activityEntriesFromToolRuns',
  'export function shouldHydrateActivity',
  'export function activityKind(line: string)',
  "value.includes('verification')",
  "value.includes('heartbeat')",
  'export function activityIcon(kind: string)',
  "if (kind === 'automation') return 'automation';"
]) {
  includes('activity helper extraction', marker, activityHelpers);
}
for (const marker of [
  'export function createActivityController',
  'function pushActivity(line: string)',
  'createActivityEntry(line, nextSequence)',
  'prependActivityEntry(ctx.getActivity(), entry)',
  'function hydrateActivityFromToolRuns()',
  'shouldHydrateActivity(activity, toolRuns)',
  'activityEntriesFromToolRuns(toolRuns, toolRunTimestamp, formatShortTime)'
]) {
  includes('activity controller extraction', marker, activityController);
}
for (const marker of [
  'export function agentStreamEventEffect',
  "event.type === 'model.selected'",
  "event.type === 'workspace.used'",
  "event.type === 'capability.gap'",
  "event.type === 'scheduler.job_proposal'",
  "event.type === 'permission.requested'",
  "event.type === 'edit.proposed'",
  "event.type === 'tool.completed'",
  "event.type === 'verification.completed'",
  'statusPatch'
]) {
  includes('agent stream event helper extraction', marker, agentStreamEventHelpers);
}
for (const marker of [
  'export function permissionItemFromEvent',
  'permissionCommand(data)',
  'requiresConfirmation: boolValue(data.requires_confirmation)',
  'export function shouldStorePermissionItem',
  'export function upsertPermissionItem',
  'export function permissionDecisionBusyState',
  'export function editProposalDraftFromEvent'
]) {
  includes('permission stream helper extraction', marker, permissionStreamHelpers);
}
for (const marker of [
  'export function createPermissionController',
  'function rememberPermission',
  'function setPermissionDecisionBusy',
  'function markPermissionSettled',
  'function permissionActionDisabled',
  'function rememberEditProposal',
  'async function decidePermission',
  'recordPermissionDecision(item, decision)',
  'permissionDecisionErrorIsStale(message)',
  'upsertPermissionResultMessage',
  'markPermissionSettled(item.requestId, decision)',
  'return {'
]) {
  includes('permission controller extraction', marker, permissionController);
}
for (const marker of [
  'export async function loadAutomationSurface',
  'export async function createSchedulerJob',
  'export async function updateSchedulerJobInput',
  'export async function setSchedulerJobEnabled',
  'export async function runSchedulerJob',
  'export async function runDueSchedulerJobs',
  'export function jobTickSummary'
]) {
  includes('automation action extraction', marker, automationActions);
}
for (const marker of [
  'export function createAutomationController',
  'async function refreshAutomation',
  'async function createJob',
  'async function updateJobInput',
  'async function setJobEnabled',
  'async function runJob',
  'async function runDueJobs',
  'schedulerJobInputFromForm',
  'ctx.setJobFailureDetail(id, message)',
  'ctx.clearJobFailureDetail(id)',
  'jobTickSummary(tickResult)',
  'await ctx.openConversation(conversationId)',
  'return {'
]) {
  includes('automation controller extraction', marker, automationController);
}
for (const marker of [
  'export type SchedulerJobFormState',
  'export function schedulerJobInputFromForm',
  'const schedule = normalizeSchedulerSchedule(form.scheduleType, form.scheduleExpr)',
  "scheduleExpr: schedule.scheduleType === 'manual' ? '' : schedule.scheduleExpr",
  'export function normalizeSchedulerSchedule',
  'export function schedulerJobInputFromHandoff',
  'if (activeConversationId && !input.conversation_id && !input.conversationId)',
  'input.conversation_id = activeConversationId',
  'throw new Error(`${label} must be a JSON object`)',
  'export function jsonObjectInputError',
  'export function schedulerJobUpdateInputFromJSON',
  'export function schedulerJobInputJSON',
  'export function schedulerJobFailureTitle',
  'approved: true',
  'enabled: false',
  'export function schedulerJobCreatedSummary',
  'export function schedulerJobCreatedMessage',
  'export function schedulerJobConversationId',
  'export function schedulerJobOriginLabel',
  'export function schedulerJobRunOutputSummary'
]) {
  includes('scheduler job helper extraction', marker, schedulerJobHelpers);
}
for (const marker of [
  'export async function listConversations',
  'export async function editConversationUserMessage',
  'export async function renameConversation',
  'export async function setConversationStarredState',
  'export async function getConversationDetail',
  'export async function deleteConversation'
]) {
  includes('conversation action extraction', marker, conversationActions);
}
for (const marker of [
  'export function createConversationController',
  'async function refreshConversations',
  'function newChat',
  'function renameChatTitle',
  'async function renameActiveConversation',
  'function startRenameChatTitle',
  'async function setConversationStarred',
  'async function toggleStarChat',
  'async function toggleSidebarConversationStar',
  'async function renameSidebarConversation',
  'async function openConversation',
  'async function deleteChat',
  'async function deleteConversationById',
  'ctx.notify(`conversation renamed:',
  'ctx.notify(message)',
  'await ctx.confirm(`Delete "',
  'ctx.notify(`conversation deleted:',
  'async function exportActiveConversationMarkdown',
  'async function exportActiveConversationJSON',
  'function openMemorySearch',
  'function setSidebarVisible',
  'ctx.setConversationFlatMessages(flatMessages)',
  'ctx.setMessages(groupConversationMessages(flatMessages, overrides))',
  'ctx.persistSidebarVisible(value)',
  'return {'
]) {
  includes('conversation controller extraction', marker, conversationController);
}
for (const marker of [
  'export function exportConversationDetail',
  'export function conversationExportMarkdown',
  'export function conversationExportJSON',
  'downloadTextFile'
]) {
  includes('conversation export helper', marker, conversationExport);
}
for (const marker of [
  'Export as Markdown',
  'Export as JSON',
  'exportSidebarConversationMarkdown',
  'exportSidebarConversationJSON'
]) {
  includes('conversation export menu', marker, chatSidebar);
}
for (const marker of [
  'Export as Markdown',
  'Export as JSON',
  'exportConversationMarkdown',
  'exportConversationJSON'
]) {
  includes('conversation export title menu', marker, chatTitleMenu);
}
for (const marker of [
  '{exportActiveConversationMarkdown}',
  '{exportActiveConversationJSON}',
  '{exportSidebarConversationMarkdown}',
  '{exportSidebarConversationJSON}'
]) {
  includes('conversation export app wiring', marker, app);
}
for (const marker of [
  'export function variantSnapshotForRole',
  'export function assistantVariants',
  'export function userVariants',
  'export function activateAssistantVariant',
  'export function activateUserVariant',
  'export function syncActiveAssistantVariant',
  'export function userVariantRootKey',
  'export function conversationDisplayPrefersPreviousUser',
  'export function inferredConversationParentKey',
  'export function orderedAssistantVariants',
  'export function activeAssistantVariantIndex',
  'export function promptVariantRootKey',
  'export function groupConversationMessages',
  'export function rawConversationMessage',
  'export function displayedMessagesAsFlatMessages',
  'export function ensureDisplayedFlatMessages',
  'return displayedMessagesAsFlatMessages(messages, idFactory, nowFactory);',
  'export function upsertFlatMessage'
]) {
  includes('conversation helper extraction', marker, conversationHelpers);
}
for (const marker of [
  "export type Tab = 'chat'",
  'export const tabs',
  'export const moreTabs',
  'export const promptChips',
  'export const tabSubtitles',
  'export const internetSearchProviderOptions',
  'export const internetSearchProviderSelectOptions',
  'export const runtimeProviders',
  'export const modelRoleOptions',
  'export const memoryKindOptions',
  'export const jobScheduleTypeOptions',
  'export const jobTargetTypeOptions',
  'export const documentFileAccept',
  'export function internetSearchProviderOption',
  'export function providerDefaultBaseURL',
  'export function isTab'
]) {
  includes('app options extraction', marker, appOptions);
}
for (const marker of [
  'export type Status',
  'export type ChatMessage',
  'export type ConversationDetail',
  'export type CapabilityHandoff',
  'export type SchedulerHandoff',
  'export type SettingsView',
  'export type InternetStatus',
  'export type ToolRun',
  'export type QAReview',
  'export type QARegressionReviewRecord',
  'export type QARegressionTestDraftRecord',
  'export type QARegressionApprovedRecord',
  'export type QARegressionSourcePatchRecord',
  'export type QARegressionSourcePatchApplyPlanRecord',
  'export type QARegressionSourceWritePlanResult',
  'export type QARegressionSourceWriteApplyResult'
]) {
  includes('app type extraction', marker, appTypes);
}
for (const marker of [
  'export function permissionCommand',
  'export function permissionCommandPreview',
  'export function permissionActionDisabled',
  'export function permissionDecisionErrorIsStale',
  'export function permissionPayload',
  'export function replayRouteSummary',
  'export function replayToolSummary'
]) {
  includes('diagnostic helper extraction', marker, diagnosticHelpers);
}
for (const marker of [
  'export async function loadToolActivity',
  'export async function loadDiagnosticsSurface',
  'export async function markNotificationAsRead',
  'export async function dismissNotificationItem',
  'export async function explainReplayTraceById',
  'export async function reviewQARegressionSuggestionStatus',
  'export async function generateReviewedQARegressionTestDraft',
  'export async function approveReviewedQARegressionTestDraft',
  'export async function generateReviewedQARegressionSourcePatch',
  'export async function approveReviewedQARegressionSourcePatchApplyPlan',
  'export async function planReviewedQARegressionSourceWrite',
  'export async function applyReviewedQARegressionSourceWrite',
  'export async function recordPermissionDecision',
  'export async function planFileWrite',
  'export async function applyFileWritePlan',
  'export function diagnosticsSummary'
]) {
  includes('diagnostic action extraction', marker, diagnosticActions);
}
for (const marker of [
  'export function createDiagnosticsController',
  'async function previewFileWrite',
  'async function applyFileWrite',
  'async function loadDiagnostics',
  'async function refreshDiagnostics',
  'async function markNotificationRead',
  'async function dismissNotification',
  'async function explainReplayTrace',
  'async function reviewQARegressionSuggestion',
  'async function generateQARegressionTestDraft',
  'async function approveQARegressionTestDraft',
  'async function generateQARegressionSourcePatch',
  'async function approveQARegressionSourcePatchApplyPlan',
  'async function planQARegressionSourceWrite',
  'async function applyQARegressionSourceWrite',
  'await ctx.refreshToolRuns()',
  'return {'
]) {
  includes('diagnostics controller extraction', marker, diagnosticsController);
}
for (const marker of [
  'export function roleValue',
  'export type ModelRoleFormState',
  'export function modelRoleFormFromSettings',
  "roleValue(role, 'baseURL') || providerDefaultBaseURL(provider)",
  'export function titleFromPrompt'
]) {
  includes('model helper extraction', marker, modelHelpers);
}
for (const marker of [
  'export async function loadRuntimeSettingsSurface',
  "status: await call<Status>('Status')",
  "setupState: await call<SetupState>('SetupState')",
  "settings: await call<SettingsView>('Settings')",
  'export async function saveRuntimeSettings',
  'export async function requestYemakaRestart',
  'export async function requestYemakaShutdown',
  'export async function completeFirstRunSetup',
  'export async function indexEmbeddingChunks',
  'export function embeddingIndexSummaryText'
]) {
  includes('settings action extraction', marker, settingsActions);
}
for (const marker of [
  'export function createSettingsController',
  'function currentSettingsForm',
  'function applySettingsForm',
  'function syncSettingsForm',
  'settingsChangeNeedsRestart',
  'function internetSearchProviderEndpointDisabled',
  'function internetSearchProviderAPIKeyDisabled',
  'function onInternetSearchProviderChange',
  'async function indexEmbeddings',
  'async function saveSettings',
  'async function completeSetup',
  'normalizeInternetSearchProviderForm(currentSettingsForm(), nextProvider, resetForProviderSwitch)',
  'const form = currentSettingsForm();',
  'settingsSaveInputFromForm(form)',
  'await completeFirstRunSetup',
  'ctx.applyTheme(form.theme)',
  'return {'
]) {
  includes('settings controller extraction', marker, settingsController);
}
for (const marker of [
  'export type SettingsFormState',
  'export function defaultSettingsForm',
  'export function settingsFormsEqual',
  'export function settingsFormFromView',
  'export function normalizeInternetSearchProviderForm',
  'export function settingsSaveInputFromForm',
  "internetSearchEnabled: form.internetEnabled && form.internetSearchEnabled",
  "internetSearchProvider: normalizedInternetSearchProvider(form.internetSearchProvider)",
  "internetSearchApiKeyEnv: normalizedInternetSearchAPIKeyEnv(form.internetSearchProvider, form.internetSearchAPIKeyEnv)",
  'slackConnectorEnabled: false',
  'discordConnectorEnabled: false',
  'telegramConnectorEnabled: false',
  'emailConnectorEnabled: false',
  'slackConnectorEnabled: form.slackConnectorEnabled',
  'discordConnectorEnabled: form.discordConnectorEnabled',
  'telegramConnectorEnabled: form.telegramConnectorEnabled',
  'emailConnectorEnabled: form.emailConnectorEnabled'
]) {
  includes('settings form helper extraction', marker, settingsFormHelpers);
}
for (const marker of [
  'export async function listRuntimeModels',
  'export async function listModelProfiles',
  'export async function saveModelRoleProvider',
  'export async function showRuntimeModel',
  'export async function generateRuntimeModelText',
  'export async function showModelProfile',
  'export async function previewModelProfile',
  'export async function draftModelProfileFromConversation',
  'export async function saveModelProfile',
  'export function configuredModelRoleLabel',
  'export function modelDetailsLabel',
  'export function modelGeneratedLabel',
  'export function modelProfileSavedLabel',
  'export function modelProfilePreviewLabel',
  'export function modelProfileDraftLabel'
]) {
  includes('model action extraction', marker, modelActions);
}
for (const marker of [
  'export function createModelController',
  'async function saveModelRole',
  'async function showModelDetails',
  'async function selectModel',
  'async function generateModelText',
  'async function refreshModelProfiles',
  'async function inspectModelProfile',
  'async function previewModelProfile',
  'async function draftModelProfileFromConversation',
  'async function saveModelProfile',
  'return {'
]) {
  includes('model controller extraction', marker, modelController);
}
for (const marker of [
  'export async function searchMemoryResults',
  'export async function listMemoryResults',
  'export async function writeMemoryItem',
  'export async function pinMemoryItem',
  'export async function deleteMemoryItem',
  'export function mergeWrittenMemory',
  'export function removeMemoryResult',
  'export function memoryCountLabel',
  'export function savedMemoryLabel'
]) {
  includes('memory action extraction', marker, memoryActions);
}
for (const marker of [
  'export async function loadKnowledgeStatus',
  'export async function setKnowledgeEnabled',
  'export async function repairKnowledgeEvidence',
  'export async function listKnowledgeEntities',
  'export async function draftKnowledgeProposal',
  'export async function saveKnowledgeEntity',
  'export async function saveKnowledgeEdge',
  'export function mergeKnowledgeEntity',
  'export function knowledgeStatusLabel'
]) {
  includes('knowledge action extraction', marker, knowledgeActions);
}
for (const marker of [
  'export function createMemoryController',
  'async function searchMemory',
  'async function listMemories',
  'async function writeMemory',
  'async function pinMemory',
  'async function deleteMemory',
  'return {'
]) {
  includes('memory controller extraction', marker, memoryController);
}
for (const marker of [
  'export function createKnowledgeController',
  'async function refreshKnowledge',
  'async function toggleKnowledge',
  'async function searchKnowledge',
  'async function repairKnowledgeEvidence',
  'async function draftProposal',
  'function useEntityProposal',
  'async function addKnowledgeEntity',
  'async function addKnowledgeEdge',
  'return {'
]) {
  includes('knowledge controller extraction', marker, knowledgeController);
}
for (const marker of [
  'export type CapabilityFormGenerationOptions',
  'export function capabilityGenerationInputFromForm',
  "parseCapabilityObject(options.runInputJSON, 'Run input')",
  'export function capabilityGenerationInputFromHandoff',
  'scheduleInput.conversation_id = activeConversationId',
  'export function generatedCapabilityName',
  'export function capabilityExtensionFormFromHandoff',
  'proposalFromCapabilityHandoff(handoff)',
  'export function generatedExtensionNameFromLegacyResult'
]) {
  includes('capability generation helper extraction', marker, capabilityGenerationHelpers);
}
for (const marker of [
  'export async function loadExtensionSurface',
  'export async function listExtensions',
  'export async function listExtensionFailures',
  'export async function reviewExtensionByName',
  'export async function testExtensionByName',
  'export async function registerExtensionByName',
  'export async function proposeCapability',
  'export async function generateCapability',
  'export async function generateExtensionFromDescription',
  'export async function validateExtensionByName',
  'export async function setExtensionEnabledState',
  'export async function deleteExtensionByName',
  'export async function runExtensionByName',
  'export async function rollbackExtensionTarget',
  'export function extensionActionSummary'
]) {
  includes('extension action extraction', marker, extensionActions);
}
for (const marker of [
  'export function createExtensionsController',
  'async function refreshExtensions',
  'async function reviewExtension',
  'function useReviewedExtensionInRunner',
  'async function testAndRegisterExtension',
  'async function reuseExtension',
  'async function proposeExtension',
  'async function generateExtension',
  'async function validateExtension',
  'async function testExtension',
  'async function registerExtension',
  'async function setExtensionEnabled',
  'async function deleteExtension',
  'async function runExtension',
  'async function rollbackExtension',
  'proposalRequest: ctx.getCapabilityProposalRequest()',
  'await ctx.refreshAutomation()',
  'extensionRunInputFromJSON(ctx.getExtensionRunInputJSON())',
  'return {'
]) {
  includes('extension controller extraction', marker, extensionsController);
}
for (const marker of [
  'export type ExtensionReviewFormState',
  'export function extensionReviewFormState',
  'review.sampleInputJson || currentInputJSON',
  'export function reviewedExtensionRunnerState',
  'export function extensionTestSummary',
  'export function extensionRegisterSummary',
  'export function extensionReuseSummary',
  'export function extensionRunInputFromJSON',
  "parseSchedulerJobInput(value, 'Run input')",
  'export function extensionRunSummary'
]) {
  includes('extension page helper extraction', marker, extensionPageHelpers);
}
for (const marker of [
  'export async function listSkillCatalog',
  'export async function loadDomainPackState',
  'export async function loadSkillSurface',
  'export async function installDomainPackSource',
  'export async function installDomainPackTemplateByName',
  'export async function reviewDomainPackByName',
  'export async function reviewDomainPackTemplateByName',
  'export async function setDomainPackEnabledState',
  'export async function uninstallDomainPackByName',
  'export async function applyCapabilityDomainPack',
  'export async function validateSkillByName',
  'export async function setSkillEnabledState',
  'export async function createSkillFromConversation',
  'export async function improveSkillFromConversation',
  'export async function importSkillFromPath',
  'export async function exportSkillToPath',
  'export function domainPackResultSummary',
  'export function skillActionSummary',
  'export function skillStillAvailable'
]) {
  includes('skill action extraction', marker, skillActions);
}
for (const marker of [
  'export function createSkillsController',
  'async function loadDomainPackState',
  'async function refreshSkillsView',
  'async function installDomainPack',
  'async function installDomainPackTemplate',
  'async function reviewDomainPack',
  'async function reviewDomainPackTemplate',
  'async function setDomainPackEnabled',
  'async function uninstallDomainPack',
  'async function applyDomainPackHandoff',
  'async function validateSkill',
  'async function setSkillEnabled',
  'async function createSkillFromSession',
  'async function improveSkillFromSession',
  'async function importSkill',
  'async function exportSkill',
  'function domainPackHandoffName',
  'function domainPackTemplateInstallPath',
  'await ctx.confirm(domainPackUninstallConfirmMessage(name))',
  'ctx.setDomainPackReview(review)',
  'await refreshSkillsView(true)',
  'return {'
]) {
  includes('skills controller extraction', marker, skillsController);
}
for (const marker of [
  "export const activeTabStorageKey = 'yemaka.activeTab'",
  'export function persistActiveTab',
  'export function persistActiveConversation',
  'export function restoreStoredNavigation',
  'export function persistSidebarVisible',
  'export function restoreSidebarVisible'
]) {
  includes('navigation helper extraction', marker, navigationHelpers);
}
for (const marker of [
  "export type RefreshTarget = 'chat'",
  'export function entryRefreshTargetForTab',
  'export function manualRefreshTargetForTab',
  'export function liveRefreshTargetForTab',
  'export function shouldRefreshChatOnLive',
  "if (tab === 'models') return 'models';",
  "if (tab === 'memory') return 'memory';",
  "if (tab === 'settings') return 'settings';",
  "reason !== 'interval'"
]) {
  includes('page refresh helper extraction', marker, pageRefreshHelpers);
}
for (const marker of [
  'export function createAppRefreshController',
  'async function refreshTarget',
  'async function selectTab',
  'async function refreshActiveTab',
  'function hasActiveAssistantRun',
  'async function refreshLiveVisibleData',
  'const assistantRunActive = ctx.getBusy() || hasActiveAssistantRun()',
  'if (ctx.getActiveConversationId() && !assistantRunActive)',
  'refreshModels: (silent?: boolean) => Promise<void>',
  "if (target === 'models')",
  'await ctx.openConversation(ctx.getActiveConversationId(), { silent: true })',
  'await refreshTarget(liveRefreshTargetForTab(ctx.getActiveTab()), true)',
  'return {'
]) {
  includes('app refresh controller extraction', marker, appRefreshController);
}
for (const marker of [
  'export function compactAnyText',
  'export function toolRunTimestamp',
  'export function formatShortTime',
  'export function shortId',
  'export function toolRunInputLabel',
  'export function toolRunSessionKey',
  'export function buildToolRunGroups',
  'export function expandedByDefault',
  'export function toggledExpandedState',
  'export function formatFileSize',
  'export function formatToolStatus',
  'export function formatSurfaces',
  'export function formatToolName'
]) {
  includes('tool-run helper extraction', marker, toolRunHelpers);
}
for (const marker of [
  'export function createListExpansionController',
  'function setListPage(key: string, page: number)',
  'Math.max(1, page)',
  'function toolRunGroupExpanded',
  'function toolRunSessionExpanded',
  'function toggleToolRunGroup',
  'function toggleToolRunSession',
  'toggledExpandedState(id, expandedGroups, current)',
  'toggledExpandedState(id, expandedSessions, current)'
]) {
  includes('list expansion controller extraction', marker, listExpansionController);
}
for (const marker of [
  'export function normalizeStreamEvent',
  'export function splitEventList',
  'export function capabilityKindLabel',
  'export function eventValue',
  'export function runtimeStatusFromEvent',
  'export function capabilityHandoffFromEvent',
  'export function schedulerHandoffFromEvent',
  'export function schedulerHandoffCardClass',
  'export function schedulerHandoffDisabled',
  'export function parseCapabilityObject',
  'export function capabilityExtensionName',
  'export function existingExtensionForCapability',
  'export function capabilityApproveDisabled',
  'export function capabilityApproveLabel',
  'export function proposalFromCapabilityHandoff'
]) {
  includes('handoff helper extraction', marker, handoffHelpers);
}
for (const marker of [
  'export function summarizeIngestResult',
  'export function documentIngestErrorSummary',
  'export function summarizeDocumentPrune',
  'export function inventoryCount',
  'export function uploadedRelativePath'
]) {
  includes('document helper extraction', marker, documentHelpers);
}
for (const marker of [
  'export function directoryInput',
  "node.setAttribute('webkitdirectory', '')",
  'export function documentFileFromEvent',
  'export function documentFolderFilesFromEvent',
  'export function emptyDocumentSuggestionState',
  'export function documentSuggestionState',
  'export function selectedDocumentSuggestionState',
  'export function documentPickerUnavailableSummary',
  'export function uploadedDocumentFileSummary',
  'export function uploadedDocumentFolderSummary'
]) {
  includes('document page helper extraction', marker, documentPageHelpers);
}
for (const marker of [
  'export async function ingestDocumentPath',
  'export async function listDocumentInventory',
  'export async function pruneMissingDocuments',
  'export async function suggestDocumentPaths',
  'export async function listWorkspaceGrants',
  'export async function pickWorkspaceFolderGrant',
  'export async function pickDocumentFileSource',
  'export async function grantWorkspacePath',
  'export async function revokeWorkspaceGrantPath',
  'export async function searchDocumentResults',
  'export async function uploadDocumentFileToServer',
  'export async function uploadDocumentFolderToServer',
  "fetch('/api/documents/upload'",
  'export function mergeWorkspaceGrant',
  'export function removeWorkspaceGrant',
  'export function documentSearchLabel',
  'export function documentSuggestionSummary'
]) {
  includes('document action extraction', marker, documentActions);
}
for (const marker of [
  'export function createDocumentController',
  'let suggestionTimer',
  'let suggestionSeq',
  'async function refreshDocumentInventory',
  'async function refreshWorkspaceGrants',
  'async function refreshDocumentsSurface',
  'async function ingestDocuments',
  'async function previewPruneMissingDocuments',
  'async function applyPruneMissingDocuments',
  'function scheduleDocumentPathSuggestions',
  'async function refreshDocumentPathSuggestions',
  'function selectDocumentPathSuggestion',
  'async function pickWorkspaceFolder',
  'async function pickDocumentFile',
  'async function uploadDocumentFile',
  'async function uploadDocumentFolder',
  'async function grantDocumentPath',
  'async function revokeWorkspaceGrant',
  'async function searchDocuments',
  'return {'
]) {
  includes('document controller extraction', marker, documentController);
}
for (const marker of [
  'export function domainPackFromResult',
  'export function domainPackMessage',
  'export function domainPackTemplateSafetyText',
  'export function domainPackSkillRefs',
  'export function availableDomainPackTemplates',
  'export function domainPackTemplateInstallPath',
  'export function isDomainPackHandoff',
  'export function domainPackHandoffName',
  'export function domainPackForHandoff',
  'export function domainPackSetupLabel',
  'export function domainPackHandoffActionLabel',
  'export function domainPackHandoffActionDisabled',
  'export function domainPackHandoffApplyInput',
  'export function domainPackInstallSummary',
  'export function domainPackTemplateInstallSummary',
  'export function domainPackUpdateSummary',
  'export function domainPackUninstallConfirmMessage',
  'export function domainPackUninstallSummary',
  'export function domainPackHandoffBlockedSummary',
  'export function domainPackHandoffSummary'
]) {
  includes('domain-pack helper extraction', marker, domainPackHelpers);
}
for (const marker of [
  'export function normalizeTheme',
  'export function resolvedTheme',
  'export function applyTheme'
]) {
  includes('theme helper extraction', marker, themeHelpers);
}

for (const pattern of [
  /let messages: ChatMessage\[\] = \[\];/,
  /let conversations: ConversationSession\[\] = \[\];/,
  /let domainPacks: DomainPack\[\] = \[\];/,
  /let domainPackTemplates: DomainPackTemplate\[\] = \[\];/,
  /let domainPackSkills: DomainPackSkillStatus\[\] = \[\];/,
  /let extensions: Extension\[\] = \[\];/,
  /let jobs: Job\[\] = \[\];/,
  /let jobRuns: JobRun\[\] = \[\];/,
  /let internetCache: InternetCacheSummary\[\] = \[\];/,
  /let internetRequests: InternetRequestRecord\[\] = \[\];/,
  /let memoryResults: MemoryResult\[\] = \[\];/
]) {
  matches('empty state defaults', pattern);
}

const chat = routeSection('chat');
includes('chat route', '<ChatPage', chat);
includes('chat sidebar component', '<ChatSidebar', app);
includes('app topbar component', '<AppTopbar', app);
includes('chat title component', '<ChatTitleMenu', appTopbar);
for (const marker of [
  'topbar',
  'ollama off',
  'sqlite off',
  'refreshLabelText',
  '<ActionButton',
  '<Badge',
  'exportConversationMarkdown={exportActiveConversationMarkdown}',
  'exportConversationJSON={exportActiveConversationJSON}',
  'disabledReason="This page is already refreshing."',
  "activeTab === 'automation' ? 'health' : 'retry'"
]) {
  includes('app topbar', marker, appTopbar);
}
for (const marker of ['chat-title-wrap', 'Rename', 'Export as Markdown', 'Export as JSON', 'Delete', 'toggleStarChat']) {
  includes('chat title menu', marker, chatTitleMenu);
}
includes('chat surface component', '<ChatSurface', chatPage);
includes('chat activity component', '<ChatActivityPanel', chatPage);
for (const marker of [
  'chat-surface',
  "{#if messages.length === 0}",
  'bind:this={chatScroll}',
  'onscroll={handleChatScroll}',
  '<ChatEmptyState',
  '<ChatComposerCard',
  '<ChatMessageList',
  'data-smoke="chat-scroll-latest-control"',
  'composer-dock',
  'mode="dock"'
]) {
  includes('chat surface', marker, chatSurface);
}
for (const marker of [
  '<ChatMessageItem',
  '{#each messages as message, index',
  'bind:editingUserPrompt',
  'openAutomation'
]) {
  includes('chat message list', marker, chatMessageList);
}
for (const marker of [
  '<ChatMessageHeader',
  '<ChatMessageActions',
  '<UserMessageBody',
  '<AssistantSourceStrip',
  '<CapabilityHandoffCard',
  '<SchedulerHandoffCard',
  '<AgentTimeline',
  '<Markdown',
  "message.role === 'assistant' && message.sources?.length && !isAssistantActiveMeta(message.meta)",
  'message-shell',
  'message-bubble'
]) {
  includes('chat message item composition', marker, chatMessageItem);
}
for (const marker of [
  'function codeBlockKind',
  'Scheduler Setup',
  'md-code-block-${slugClass(kind.className)}',
  'function isSetupBlockStart',
  'function trailingURLPunctuation',
  'function anchorHTML',
  "rel=\"noopener noreferrer\"",
  "'missing details'",
  "'status note'",
  'md-stream-status'
]) {
  includes('chat markdown response polish', marker, markdown);
}
for (const marker of [
  'Completed steps',
  'agent-step-rail',
  'role="listitem"',
  'agent-step-last',
  'aria-label={title}',
  '<Icon name="check"'
]) {
  includes('agent timeline product stepper', marker, agentTimeline);
}
for (const marker of [
  '<ActionButton',
  'approveDisabledReason',
  'Reuse extension',
  'Open in Extensions'
]) {
  includes('capability handoff action buttons', marker, capabilityHandoffCard);
}
for (const marker of [
  '<ActionButton',
  'disabledReason={schedulerHandoffBusy',
  'Create disabled job',
  'Open Automation'
]) {
  includes('scheduler handoff action buttons', marker, schedulerHandoffCard);
}
includes('chat empty composer mode', 'mode="empty"', chatEmptyState);
includes('chat dock composer mode', 'mode="dock"', chatSurface);
includes('chat empty route', 'How can Yemaka help?', chatEmptyState);
includes('chat composer empty placeholder', 'Ask about your files, notes, tools, or next task...', chatEmptyState);
includes('chat empty prompt chips', '{#each promptChips as chip}', chatEmptyState);
includes('chat empty prompt chips', 'aria-label={`Use prompt: ${chip.label}`}', chatEmptyState);
for (const marker of [
  'composer-card',
  'composer-after-chat',
  'main-empty-composer',
  "composerAction === 'queue'",
  "composerAction === 'stop'",
  "'Queue follow-up prompt'",
  "'Stop generation'",
  "'Send prompt'",
  "'send-button-queue'",
  "'send-button-stop'",
  '{@render queuedFollowUpList()}',
  '{@render composerCapabilityMenu()}'
]) {
  includes('chat composer card', marker, chatComposerCard);
}
includes('composer capability menu component', '<ComposerCapabilityMenu', app);
includes('composer capability menu component', 'enabledSkills={composerEnabledSkills}', app);
includes('composer capability menu component', 'chatContextItems={composerChatContextItems}', app);
includes('composer capability menu component', '{skillDisplayLabel}', app);
for (const marker of [
  'composer-menu',
  'Yemaka controls',
  'Auto skill',
  'Current route',
  'activeContextCount',
  'primarySkills = enabledSkills.slice(0, 5)',
  'moreSkills = enabledSkills.slice(5)',
  'moreSkillsOpen = false',
  '<ScrollArea.Root class="composer-skill-scroll"',
  'More skills',
  'aria-controls="composer-more-skills"',
  'aria-expanded={moreSkillsOpen}',
  'skillDisplayLabel(skill.name)',
  '{#each primarySkills as skill}',
  '{#each moreSkills as skill}',
  '{#each chatContextItems as item}',
  'Yemaka tuning',
  'Low memory',
  'RAG',
  'Internet',
  'safe allowlist',
  'Open settings',
  "toggleComposerSetting('lowMemory')",
  "setActiveTab('settings')"
]) {
  includes('composer capability menu', marker, composerCapabilityMenu);
}
includes('queued follow-up component', '<QueuedFollowUpList', app);
for (const marker of [
  'queued-followups',
  'Follow-up queue',
  'queued-followup-edit',
  '<ActionButton',
  'disabledReason="Enter follow-up text before saving."',
  'Edit queued prompt',
  'Delete queued prompt',
  'runs after current response',
  'updateQueuedFollowUpDraft(item.id',
  'saveQueuedFollowUp(item.id)'
]) {
  includes('queued follow-up list', marker, queuedFollowUpList);
}
for (const marker of [
  'message-actions',
  'Prompt variants',
  'Response variants',
  'aria-label="Previous prompt variant"',
  'aria-label="Try this prompt again"',
  'Copy message',
  'Edit and resend this prompt',
  'Try this prompt again',
  'switchUserVariant(messageIndex, -1)',
  'switchAssistantVariant(messageIndex, 1)'
]) {
  includes('chat message actions', marker, chatMessageActions);
}
for (const marker of [
  'message-meta-row',
  'message-state-pill working-shimmer',
  'working-shimmer-label',
  'message-meta-pill',
  'message-user-label',
  "message.meta === 'streaming' ? 'Writing' : 'Working'",
  "message.role === 'system'"
]) {
  includes('chat message header', marker, chatMessageHeader);
}
for (const marker of [
  'message-edit-panel',
  'message-edit-textarea',
  '<ActionButton',
  'disabledReason={editingUserBusy ?',
  'Save and resend',
  'message-user-content',
  'message-show-more',
  'Show more',
  'Show less',
  'submitEditedUserMessage(messageIndex)',
  'toggleUserMessageExpanded(messageIndex)'
]) {
  includes('user message body', marker, userMessageBody);
}
for (const marker of [
  'source-strip',
  'source-title',
  'sources used',
  'sources.slice(0, 6)',
  'compactSource(source)'
]) {
  includes('assistant source strip', marker, assistantSourceStrip);
}
includes('permission exact action preview', 'function permissionCommandPreview(item: PermissionItemLike)', diagnosticHelpers);
for (const marker of [
  'Approvals',
  'Exact action',
  'Activity',
  '<SectionHeader',
  'Review exact tool actions before Yemaka continues.',
  'Live steps, approvals, and local action history.',
  '<ActionButton',
  'disabledReason="This permission decision is already being processed."',
  '<EmptyState',
  'No activity yet',
  "{#each pagedItems(activity, 'activity-live', 5, listPages) as item, index (activityKey(item, index))}",
  "{#each pagedItems(groupedToolRuns, 'activity-run-groups', 5, listPages) as group"
]) {
  includes('chat activity panel', marker, chatActivityPanel);
}
for (const marker of [
  "existingCapability: eventValue(data, 'existing_capability', 'existingCapability')",
  "capabilityState: eventValue(data, 'capability_state', 'capabilityState')",
  "packSource: eventValue(data, 'pack_source', 'packSource', 'capability_source', 'source')",
  "installCommand: eventValue(data, 'install_command', 'installCommand', 'pack_install_command', 'packInstallCommand')"
]) {
  includes('domain-pack chat handoff parser', marker, handoffHelpers);
}
for (const marker of [
  'export function createChatHandoffController',
  'function updateCapabilityHandoff',
  'function rememberCapabilityHandoff',
  'function updateSchedulerHandoff',
  'function rememberSchedulerHandoff',
  'function schedulerHandoffDisabled',
  'function setCapabilityRunAfterGenerate',
  'function setCapabilityScheduleAfterGenerate',
  'function capabilityExtensionName',
  'function openCapabilityInExtensions',
  'function openDomainPacksFromHandoff',
  'async function approveChatCapability',
  'async function createSchedulerJobFromChat',
  'capabilityGenerationInputFromHandoff(handoff, ctx.getActiveConversationId())',
  'schedulerJobInputFromHandoff(handoff, ctx.getActiveConversationId())',
  'ctx.setDomainPackSummary(handoff.configureHint || handoff.suggestedAction || handoff.capabilitySummary || \'\')',
  'return {'
]) {
  includes('chat handoff controller extraction', marker, chatHandoffController);
}
for (const marker of [
  'let activeDomainPackHandoff: CapabilityHandoff | null = null;',
  'const {',
  '{domainPacks}',
  '{domainPackTemplates}',
  '{domainPackSkills}',
  'domainPackTemplateInstallPath',
  'applyDomainPackHandoff'
]) {
  includes('domain-pack chat handoff', marker);
}
includes('domain-pack chat handoff', 'activeHandoffSetupLabel', skillsPage);
for (const marker of [
  "function domainPackTemplateInstallPath(handoff: CapabilityHandoff | null | undefined)",
  "function domainPackHandoffApplyInput(handoff: CapabilityHandoff, installed: DomainPack | null)",
  "async function applyDomainPackHandoff(handoff: CapabilityHandoff | null | undefined)"
]) {
  includes('domain-pack chat handoff controller', marker, skillsController);
}
includes('domain-pack chat handoff path helper', 'return `packs/templates/${handoffName}`;', domainPackHelpers);
for (const marker of [
  'capability-handoff-card',
  'Missing capability',
  'Domain packs stay local and require explicit install or enablement before use.',
  'Open Domain Packs',
  'Open in Extensions',
  'Run once after generation',
  'Create scheduler job',
  'Approval-gated tools',
  'Reuse extension'
]) {
  includes('capability handoff card', marker, capabilityHandoffCard);
}
for (const marker of [
  'capability-handoff-card',
  'Scheduler proposal',
  'Approval creates a disabled local job record.',
  'Job profile',
  'Create disabled job',
  'Open Automation',
  'schedulerHandoffDisabledFor(handoff, schedulerHandoffBusy)',
  'createSchedulerJobFromChat(messageIndex)'
]) {
  includes('scheduler handoff card', marker, schedulerHandoffCard);
}

for (const marker of [
  'sidebar-shell',
  'rail-shell',
  'New chat',
  'Search memory',
  'Documents',
  'No saved chats yet',
  'Local profile',
  'profile-settings-button',
  'aria-label="Open settings"',
  'aria-label="Primary navigation"',
  "aria-current={activeTab === 'chat' ? 'page' : undefined}",
  "aria-current={activeConversationId === item.id ? 'page' : undefined}",
  'conversation-ellipsis',
  'toggleSidebarConversationStar(item)',
  'deleteConversationById(item.id)'
]) {
  includes('sidebar sessions', marker, chatSidebar);
}
matches('sidebar starred sessions', /\$: starredConversations = conversations\.filter\(\(conversation\) => conversation\.starred\);/);
matches('sidebar recent sessions', /\$: recentConversations = conversations\.filter\(\(conversation\) => !conversation\.starred\);/);

const memory = routeSection('memory');
includes('memory route component', '<MemoryPage', memory);
includes('memory route', 'Write Memory', memoryPage);
includes('memory route', 'Search memory', memoryPage);
includes('memory route knowledge graph', 'Local Knowledge Graph', memoryPage);
includes('memory route knowledge graph', 'Manual-only local entities and relationships', memoryPage);
includes('memory route knowledge graph draft', 'Draft From Text', memoryPage);
includes('memory route knowledge graph draft', 'review-only graph candidates', memoryPage);
includes('memory route knowledge graph draft', 'Draft Review', memoryPage);
includes('memory route knowledge graph draft', 'Use Draft', memoryPage);
includes('memory route knowledge graph', 'Enable Manual Graph', memoryPage);
includes('memory route knowledge graph', 'Repair Redaction', memoryPage);
includes('memory route knowledge graph', 'Save Entity', memoryPage);
includes('memory route knowledge graph', 'Save Link', memoryPage);
includes('memory paginated data guard', "{#each pagedItems(memoryResults, 'memory-results', pageSizeFor('memory-results'), listPages) as result}", memoryPage);
includes('knowledge paginated data guard', "{#each pagedItems(knowledgeResults, 'knowledge-results', pageSizeFor('knowledge-results'), listPages) as entity}", memoryPage);
for (const marker of [
  '<SectionHeader',
  '<StructuredDataView',
  'Save explicit preferences, corrections, or notes into local memory.',
  'Browse local memory records returned by list or search.',
  '<ActionButton',
  'disabledReason="Enter memory content before saving."',
  'disabledReason="Enter a memory query before searching."',
  '<PageStatusStrip',
  '<EmptyState',
  'memoryPageStatus',
  'Memory status',
  'No saved memory',
  'title="Memory content"'
]) {
  includes('memory route status surface', marker, memoryPage);
}

const models = routeSection('models');
includes('models route component', '<ModelsPage', models);
includes('models route', 'Installed Runtime Models', modelsPage);
includes('models route', 'Model Role', modelsPage);
includes('models route', 'Model Profiles', modelsPage);
for (const marker of [
  '<SectionHeader',
  'Local runtime models available for Yemaka roles.',
  'Assign a local model to a Yemaka role.',
  'Run a short prompt against the selected model.',
  'Inspected local behavior artifacts for already-installed models.',
  'Preview and save a local profile artifact. It will not train, download, or switch models.',
  'Reviewed conversation draft',
  'Draft From Conversation',
  'Raw transcript text is not copied',
  'draftReport',
  'No model profiles yet',
  'Saved path:',
  'safetySummary',
  '<ActionButton',
  'disabledReason={modelBusy ?',
  'disabledReason={modelProfileBusy ?',
  'Select a model and enter a test prompt before generating.',
  'aria-pressed={modelName === model.name}',
  '<PageStatusStrip',
  '<EmptyState',
  'Model action running',
  'Model profile action running',
  'No installed local models detected'
]) {
  includes('models route status surface', marker, modelsPage);
}

const documents = routeSection('documents');
includes('documents route component', '<DocumentsPage', documents);
includes('documents local web wiring', 'localWeb={isLocalWebPage()}', documents);
for (const marker of [
  'Document Source',
  '<SectionHeader',
  '<Badge',
  '<StructuredDataView',
  'Grant, pick, upload, or ingest local documents for search and chat context.',
  'Document Inventory',
  'Workspace Grants',
  '<ActionButton',
  'disabledReason="Enter or pick a document path before granting access."',
  'disabledReason="Enter, pick, upload, or grant a document path before ingesting."',
  'disabledReason={documentPrunePreview?.dryRun ?',
  '<PageStatusStrip',
  '<EmptyState',
  'documentPageStatus',
  'Local web document access',
  'browser-owned file confirmation dialog',
  'Upload folder copy',
  'Browser confirmation follows',
  'macOS access',
  'aria-expanded={documentPathSuggestionOpen}',
  'Upload file',
  'Upload folder',
  'Grant Path',
  'Ingest',
  'Preview Prune',
  'Prune Missing',
  'title="Chunk content"',
  "'document-inventory'",
  "'workspace-grants'",
  "'document-results'"
]) {
  includes('documents route', marker, documentsPage);
}
includes('documents upload API wiring', "/api/documents/upload", documentActions);

const skills = routeSection('skills');
includes('skills route component', '<SkillsPage', skills);
for (const marker of [
  'Domain Packs',
  '<ActionButton',
  '<Badge',
  'disabledReason="Enter a conversation id before creating a skill."',
  'disabledReason="Enter a local skill folder before importing."',
  'disabledReason={domainPackBusy ?',
  'Profile-local packs are installed from a local folder and stay disabled until enabled.',
  'Built-in pack templates',
  '<SectionHeader',
  'Create From Session',
  'Import / Export',
  'Installed Skills',
  'Install template',
  'Pack review:',
  'data-smoke="domain-pack-template-review-inline"',
  'data-smoke="domain-pack-installed-review-inline"',
  'onclick={closeDomainPackReview}',
  'Inspect template',
  'Inspect pack',
  'Pack skills are inactive until this pack is enabled.',
  'pack.repairHint',
  'humanizeIdentifierList',
  'humanizeIdentifier(skill.name',
  'humanizeIdentifier(template.name',
  'humanizeIdentifier(pack.name',
  'visibleDomainPackTemplates',
  'domainPackSkillRefsFor(domainPackSkills, pack.name)',
  'onclick={() => installDomainPackTemplate(template.name)}',
  'onclick={() => reviewDomainPackTemplate(template.name)}',
  'onclick={() => reviewDomainPack(pack)}',
  '<PageStatusStrip',
  '<EmptyState',
  'skillsPageStatus',
  'Skills status',
  'No domain packs installed',
  'No skills installed',
  'onclick={() => uninstallDomainPack(pack)}',
  'Uninstall',
  "placeholder=\"local pack folder\"",
  'data-smoke="domain-pack-handoff-apply"',
  'Chat handoff:',
  'onclick={() => applyDomainPackHandoff(activeDomainPackHandoff)}',
  "{#each pagedItems(domainPacks, 'domain-packs', pageSizeFor('domain-packs'), listPages) as pack}",
  'onclick={installDomainPack}',
  'onclick={() => setDomainPackEnabled(pack.name, true)}',
  'onclick={() => setDomainPackEnabled(pack.name, false)}'
]) {
  includes('domain packs route', marker, skillsPage);
}
for (const marker of [
  "case 'ListDomainPacks':",
  "case 'ListDomainPackTemplates':",
  "case 'ListDomainPackSkills':",
  "case 'InstallDomainPack':",
  "case 'InstallDomainPackTemplate':",
  "case 'ReviewDomainPack':",
  "case 'ReviewDomainPackTemplate':",
  "case 'SetDomainPackEnabled':",
  "case 'UninstallDomainPack':",
  "case 'ApplyCapability':"
]) {
  includes('domain pack API call wiring', marker, api);
}

const extensions = routeSection('extensions');
includes('extensions route component', '<ExtensionsPage', extensions);
for (const marker of [
  'Generated Extensions',
  'Generate Capability',
  'Run Or Roll Back',
  'Review And Reuse',
  'Failure Trends',
  '<SectionHeader',
  '<Badge',
  '<StructuredDataView',
  'Profile-local tools, validation, tests, execution, and rollback.',
  'Propose the smallest profile-local tool, then generate it only after approval.',
  'Run a callable extension or roll back a generated capability snapshot.',
  'Inspect generated files before running existing capabilities.',
  'Repeated extension failures that may need review or repair.',
  '<ActionButton',
  'disabledReason="Enter a capability request before proposing an extension."',
  'disabledReason={generateCapabilityDisabledReason}',
  'disabled={runExtensionDisabled}',
  'disabledReason={runExtensionDisabledReason}',
  'Invalid scheduler input JSON',
  'Invalid run input JSON',
  'disabledReason="Enter a snapshot or extension name before rolling back."',
  'disabledReason={extension.validationError || extension.runBlockedReason ||',
  '<PageStatusStrip',
  '<EmptyState',
  'Extension review running',
  'No extension selected',
  'No generated extensions registered yet',
  'let selectedReviewFile',
  '$: selectedReviewFile = extensionReview',
  'title="Runner sample input"',
  'title="File preview"',
  "{#each pagedItems(extensions, 'extensions-list', pageSizeFor('extensions-list'), listPages) as extension}"
]) {
  includes('extensions route', marker, extensionsPage);
}

const tools = routeSection('tools');
includes('tools route component', '<ToolsPage', tools);
for (const marker of [
  'File Write',
  'Notification Inbox',
  'Replay Diagnostics',
  'Capability Truth',
  'Recent Tool Runs',
  '<SectionHeader',
  '<Badge',
  '<StructuredDataView',
  'formatToolName(tool.name)',
  'formatToolName(run.toolName)',
  'formatToolStatus(run.status)',
  'Preview a diff before applying a local file write.',
  'Job failures, heartbeat transitions, and local system notices.',
  'Inspect recent route and executor traces.',
  'Typed tools, policy state, and model-callable status.',
  'Grouped tool calls with inputs, outputs, status, and risk.',
  '<ActionButton',
  'disabledReason="Enter a target file path before previewing a diff."',
  'disabledReason="Preview a changed diff before applying a file write."',
  'disabledReason="This notification action is already running."',
  'disabledReason="Yemaka is already explaining this replay trace."',
  '<PageStatusStrip',
  '<EmptyState',
  'Diagnostics running',
  'No active notifications',
  'No replay traces recorded yet',
  'No tool catalog entries',
  'No tool runs logged yet',
  'title="Route"',
  'title="Output"',
  "{#each pagedItems(toolCatalog, 'tool-catalog', pageSizeFor('tool-catalog'), listPages) as tool",
  "{#each pagedItems(groupedToolRuns, 'tool-run-groups', pageSizeFor('tool-run-groups'), listPages) as group"
]) {
  includes('tools route', marker, toolsPage);
}

const automation = routeSection('automation');
includes('automation route component', '<AutomationPage', automation);
for (const marker of [
  'Heartbeat',
  'Scheduler',
  'Internet Service',
  'Create Scheduled Job',
  'Jobs',
  'Job Runs',
  '<SectionHeader',
  '<Badge',
  'Local runtime, scheduler, internet, connector, and extension health.',
  'Approved jobs run locally and stay bounded by policy.',
  'Controlled GET/HEAD requests and configured search provider status.',
  'Configured local jobs with approval and enabled state.',
  'Manual and scheduled execution history.'
]) {
  includes('automation route', marker, automationPage);
}
for (const marker of [
  '<ActionButton',
  '<StructuredDataView',
  'disabledReason={automationBusy ?',
  'disabledReason="Enter a URL before sending a controlled HEAD request."',
  'disabledReason="Enter a URL before sending a controlled GET request."',
  'disabledReason={createJobDisabledReason}',
  'jobFailureDetails[job.id]',
  'startEditingJobInput(job)',
  'Save input',
  '<PageStatusStrip',
  '<EmptyState',
  'Automation refresh running',
  'No heartbeat checks recorded',
  'No requests logged',
  'No crawl runs',
  'inspectInternetCrawl(run.runId)',
  'No cached responses',
  'No jobs registered',
  'No job runs recorded',
  'Invalid job input JSON',
  'Job run failed',
  'title="Internet output"',
  "title={run.error ? 'Failure detail' : 'Output'}"
]) {
  includes('automation empty state', marker, automationPage);
}
for (const marker of [
  "{#each pagedItems(heartbeatReport?.checks, 'heartbeat-checks', pageSizeFor('heartbeat-checks'), listPages) as check}",
  "{#each pagedItems(internetRequests, 'internet-requests', pageSizeFor('internet-requests'), listPages) as request}",
  "{#each pagedItems(internetCrawls, 'internet-crawls', pageSizeFor('internet-crawls'), listPages) as run}",
  "{#each pagedItems(internetCache, 'internet-cache', pageSizeFor('internet-cache'), listPages) as item}",
  "{#each pagedItems(jobs, 'jobs-list', pageSizeFor('jobs-list'), listPages) as job}",
  "{#each pagedItems(jobRuns, 'job-runs', pageSizeFor('job-runs'), listPages) as run}"
]) {
  includes('automation paginated data guard', marker, automationPage);
}

const learning = routeSection('learning');
includes('learning route component', '<LearningPage', learning);
for (const marker of [
  'Learning Report',
  'QA Review',
  'Replay Failure Coverage',
  'Route Corrections',
  'Correction Memory',
  'Trajectory Export',
  'Recent Learning Memory',
  '<SectionHeader',
  '<Badge',
  '<StructuredDataView',
  'Workflow outcomes, corrections, extension generation, and learning suggestions.',
  'Current local safety policy and autonomy bounds.',
  'Replay failures, route corrections, and regression coverage status.',
  'Failures that need generic regression coverage.',
  'Save explicit user teaching for future routing behavior.',
  'Export a conversation trajectory for replay and QA review.',
  'Recent local memories created from workflows, corrections, and failures.',
  '<ActionButton',
  'disabledReason="Enter a conversation id and correction content before saving."',
  'disabledReason="Enter a conversation id before exporting a trajectory."',
  '<PageStatusStrip',
  '<EmptyState',
  'Learning refresh running',
  'No learning suggestions',
  'No replay failures needing review',
  'No route corrections saved yet',
  'No learning memories recorded yet',
  'title="Regression suggestion"',
  'Approve',
  'Mark covered',
  'Dismiss',
  'Test draft',
  'Approve regression',
  'Source patch',
  'Apply plan',
  'Developer source write',
  'Preview write',
  'Apply source test',
  'testDraftStatusFor(failure)',
  'approvedRegressionFor(failure)',
  'sourcePatchStatusFor(failure)',
  'sourcePatchApplyStatusFor(failure)',
  'sourceTestWriteStatusFor(failure)',
  'reviewStatusFor(failure)',
  'reviewQARegressionSuggestion',
  'title="Learning memory"',
  'title="Trajectory preview"',
  "{#each pagedItems(learningReport?.suggestions, 'learning-suggestions', pageSizeFor('learning-suggestions'), listPages) as suggestion}",
  "{#each pagedItems(qaReview?.replayFailures, 'qa-replay-failures', pageSizeFor('qa-replay-failures'), listPages) as failure",
  "{#each pagedItems(qaReview?.routeCorrections, 'qa-route-corrections', pageSizeFor('qa-route-corrections'), listPages) as correction",
  "{#each pagedItems(learningReport?.recentLearningMemory, 'learning-memory', pageSizeFor('learning-memory'), listPages) as item}"
]) {
  includes('learning route', marker, learningPage);
}

for (const marker of [
  'Internet access',
  'Internet search',
  'Search ready',
  'Search provider',
  'Search status',
  'providerEndpointDisabled',
  'providerAPIKeyDisabled',
  'providerAPIKeyEnvValidationMessage',
  'internetSearchReadiness(internetStatus)',
  'internetSearchProviderLabel(internetStatus)'
]) {
  includes('settings internet status', marker, settingsPage);
}
for (const marker of [
  'SearXNG',
  'Brave Search',
  'Wikimedia',
  'DuckDuckGo',
  'Firecrawl',
  'Mojeek Search',
  'Custom GET provider',
  'Auto fallback',
  'Tavily',
  'Serper.dev',
  'Supported POST vendor API for AI-agent/RAG search.',
  'Supported POST Google SERP provider for basic web search.'
]) {
  includes('settings internet provider metadata', marker, appOptions);
}
for (const marker of [
  'statusValue.searchProviderNeedsAuth',
  'statusValue.searchProviderNeedsConfig'
]) {
  includes('settings internet provider readiness', marker, statusHelpers);
}
includes('settings route component', '<SettingsPage', app);
for (const marker of [
  '<SectionHeader',
  '<Badge',
  'Local runtime defaults, memory profile, RAG, embeddings, and controlled internet search.',
  'Cloud fallback and connectors stay disabled until explicitly configured.',
  'Registered connectors with health, scopes, rate limits, and secret references.',
  'Current profile, config, and SQLite paths for this local Yemaka profile.',
  '<ActionButton',
  'disabledReason={settingEmbeddingsEnabled ?',
  'Save Settings',
  'Runtime Control',
  'Restart Yemaka',
  'Shut Down',
  '<EmptyState',
  'No connectors registered',
  'Optional connectors stay disabled until configured'
]) {
  includes('settings empty state', marker, settingsPage);
}

for (const marker of [
  'First Run Setup',
  '<ActionButton',
  "aria-pressed={setupMode === 'student_laptop'}",
  "aria-pressed={setupMode === 'useful_local'}",
  'disabledReason={setupBusy ?',
  'setupModeDirty',
  'Save Setup'
]) {
  includes('setup persistence flow', marker, firstRunSetup);
}
includes('setup component wiring', '<FirstRunSetup', app);
includes('setup persistence API wiring', "case 'CompleteSetup':", api);

includes('pagination component usage', "import PaginationControls from './PaginationControls.svelte';", chatActivityPanel);
includes('pagination accessibility', 'aria-label={`Previous ${label} page`}', paginationControls);
includes('pagination accessibility', 'aria-label={`Next ${label} page`}', paginationControls);
includes('pagination state controller', 'function setListPage(key: string, page: number)', listExpansionController);
includes('message collapse state', 'let expandedMessageIndexes: Record<number, boolean> = {};', app);
includes('pagination helper extraction', 'export function currentPage(key: string, total: number', uiHelpers);
for (const marker of [
  'message-show-more',
  'Show more',
  'Show less'
]) {
  includes('pagination and collapse UI', marker, userMessageBody);
}

for (const marker of ['yemaka.activeTab', 'yemaka.sidebarVisible']) {
  includes('local UI persistence', marker, navigationHelpers);
}
includes('local UI persistence', 'yemaka.theme');

if (failures.length > 0) {
  console.error('UI smoke check failed:');
  for (const failure of failures) console.error(`- ${failure}`);
  process.exit(1);
}

console.log('UI smoke check passed: chat, sidebar sessions, settings internet status, domain packs, extensions, automation, memory, and empty data guards are present.');
