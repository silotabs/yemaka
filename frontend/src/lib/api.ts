let activeRequestSignal: AbortSignal | null = null;

export function desktopAPI() {
  return window.go?.desktop?.App ?? window.go?.main?.App;
}

export function isLocalWebPage() {
  return ['http:', 'https:'].includes(window.location.protocol) && !window.runtime?.EventsOn;
}

export function shouldUseHTTPAskStream() {
  return isLocalWebPage() || !desktopAPI()?.Ask;
}

export function desktopPickerAvailable() {
  return Boolean(desktopAPI()?.PickWorkspaceFolder);
}

export function desktopFilePickerAvailable() {
  return Boolean(desktopAPI()?.PickDocumentFile);
}

export function setActiveRequestSignal(signal: AbortSignal | null) {
  activeRequestSignal = signal;
}

export async function stopActiveAskHTTP() {
  const response = await fetch('/api/ask/stop', { method: 'POST' });
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || 'Stop generation failed');
  }
  return payload as { stopped?: boolean };
}

export async function call<T>(name: string, ...args: unknown[]): Promise<T> {
  const target = desktopAPI();
  if (target?.[name]) {
    return (await target[name](...args)) as T;
  }
  return httpCall<T>(name, ...args);
}

export async function httpCall<T>(name: string, ...args: unknown[]): Promise<T> {
  let path = '';
  let init: RequestInit = {};
  switch (name) {
    case 'Status':
      path = '/api/status';
      break;
    case 'SetupState':
      path = '/api/setup';
      break;
    case 'Settings':
      path = '/api/settings';
      break;
    case 'SaveSettings':
      path = '/api/settings';
      init = jsonPost(args[0] ?? {});
      break;
    case 'RequestRestart':
      path = '/api/runtime/restart';
      init = runtimeControlPost();
      break;
    case 'RequestShutdown':
      path = '/api/runtime/shutdown';
      init = runtimeControlPost();
      break;
    case 'ListSkills':
      path = '/api/skills';
      break;
    case 'ListDomainPacks':
      path = '/api/domain-packs';
      break;
    case 'ListDomainPackTemplates':
      path = '/api/domain-packs/templates';
      break;
    case 'ListDomainPackSkills':
      path = '/api/domain-packs/skills';
      break;
    case 'ReviewDomainPack':
      path = '/api/domain-packs/review';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'ReviewDomainPackTemplate':
      path = '/api/domain-packs/templates/review';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'InstallDomainPack':
      path = '/api/domain-packs/install';
      init = jsonPost({ path: args[0] ?? '' });
      break;
    case 'InstallDomainPackTemplate':
      path = '/api/domain-packs/install-template';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'SetDomainPackEnabled':
      path = '/api/domain-packs/enabled';
      init = jsonPost({ name: args[0] ?? '', enabled: Boolean(args[1]) });
      break;
    case 'UninstallDomainPack':
      path = '/api/domain-packs/uninstall';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'ApplyCapability':
      path = '/api/capabilities/apply';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ValidateSkill':
      path = '/api/skills/validate';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'SetSkillEnabled':
      path = '/api/skills/enabled';
      init = jsonPost({ name: args[0] ?? '', enabled: Boolean(args[1]) });
      break;
    case 'CreateSkillFromSession':
      path = '/api/skills/create-from-session';
      init = jsonPost({ conversationId: args[0] ?? '' });
      break;
    case 'ImproveSkillFromSession':
      path = '/api/skills/improve-from-session';
      init = jsonPost({ name: args[0] ?? '', conversationId: args[1] ?? '' });
      break;
    case 'ImportSkill':
      path = '/api/skills/import';
      init = jsonPost({ path: args[0] ?? '' });
      break;
    case 'ExportSkill':
      path = '/api/skills/export';
      init = jsonPost({ name: args[0] ?? '', path: args[1] ?? '' });
      break;
    case 'ListExtensions':
      path = '/api/extensions';
      break;
    case 'ExtensionFailures':
      path = `/api/extensions/failures?limit=${encodeURIComponent(String(args[0] ?? 20))}`;
      break;
    case 'ShowExtension':
      path = `/api/extensions/show?name=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'InspectExtension':
      path = `/api/extensions/inspect?name=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'ReviewExtension':
      path = `/api/extensions/review?name=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'ProposeExtension':
      path = '/api/extensions/propose';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ProposeCapability':
      path = '/api/capabilities/propose';
      init = jsonPost(args[0] ?? {});
      break;
    case 'GenerateCapability':
      path = '/api/capabilities/generate';
      init = jsonPost(args[0] ?? {});
      break;
    case 'GenerateExtension':
      path = '/api/extensions/generate';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ValidateExtension':
      path = '/api/extensions/validate';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'TestExtension':
      path = '/api/extensions/test';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'RegisterExtension':
      path = '/api/extensions/register';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'SetExtensionEnabled':
      path = '/api/extensions/enabled';
      init = jsonPost({ name: args[0] ?? '', enabled: Boolean(args[1]) });
      break;
    case 'DeleteExtension':
      path = '/api/extensions/delete';
      init = jsonPost({ name: args[0] ?? '' });
      break;
    case 'RollbackExtension':
      path = '/api/extensions/rollback';
      init = jsonPost(args[0] ?? {});
      break;
    case 'RunExtension':
      path = '/api/extensions/run';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ListJobs':
      path = '/api/jobs';
      break;
    case 'JobStatus':
      path = '/api/jobs/status';
      break;
    case 'JobRuns':
      path = `/api/jobs/runs?limit=${encodeURIComponent(String(args[0] ?? 20))}`;
      break;
    case 'OpsStatus': {
      const params = new URLSearchParams();
      params.set('limit', String(args[1] ?? 20));
      if (args[0]) params.set('includeRelease', 'true');
      path = `/api/ops/status?${params.toString()}`;
      break;
    }
    case 'CreateJob':
      path = '/api/jobs/create';
      init = jsonPost(args[0] ?? {});
      break;
    case 'UpdateJobInput':
    case 'UpdateJob':
      path = '/api/jobs/input';
      init = jsonPost(args[0] ?? {});
      break;
    case 'SetJobEnabled':
      path = '/api/jobs/enabled';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ArchiveJob':
      path = '/api/jobs/archive';
      init = jsonPost(args[0] ?? {});
      break;
    case 'RunJob':
      path = '/api/jobs/run';
      init = jsonPost(args[0] ?? {});
      break;
    case 'RunDueJobs':
      path = '/api/jobs/tick';
      init = jsonPost({});
      break;
    case 'HeartbeatStatus':
      path = '/api/heartbeat/status';
      break;
    case 'InternetStatus':
      path = '/api/internet/status';
      break;
    case 'InternetFetch':
      path = '/api/internet/fetch';
      init = jsonPost(args[0] ?? {});
      break;
    case 'InternetHead':
      path = '/api/internet/head';
      init = jsonPost(args[0] ?? {});
      break;
    case 'InternetSearch':
      path = '/api/internet/search';
      init = jsonPost(args[0] ?? {});
      break;
    case 'InternetCrawl':
      path = '/api/internet/crawl';
      init = jsonPost(args[0] ?? {});
      break;
    case 'InternetCrawlRun':
      path = `/api/internet/crawl-run?id=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'InternetCrawls':
      path = `/api/internet/crawls?limit=${encodeURIComponent(String(args[0] ?? 20))}`;
      break;
    case 'InternetCache':
      path = '/api/internet/cache';
      break;
    case 'InternetRequests':
      path = `/api/internet/requests?limit=${encodeURIComponent(String(args[0] ?? 20))}`;
      break;
    case 'PolicyStatus':
      path = '/api/policy';
      break;
    case 'SetPolicyMode':
      path = '/api/policy/mode';
      init = jsonPost({ mode: args[0] ?? '' });
      break;
    case 'LearningReport':
      path = '/api/learn/report';
      break;
    case 'ExportTrajectory':
      path = '/api/learn/export';
      init = jsonPost({ conversationId: args[0] ?? '' });
      break;
    case 'SaveCorrection':
      path = '/api/learn/correction';
      init = jsonPost(args[0] ?? {});
      break;
    case 'QAReview':
      path = `/api/feedback/review?limit=${encodeURIComponent(String(args[0] ?? 30))}`;
      break;
    case 'PromoteQARegressionSuggestion':
      path = '/api/feedback/review/promote';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ReviewQARegressionSuggestion':
      path = '/api/feedback/review/status';
      init = jsonPost(args[0] ?? {});
      break;
    case 'GenerateQARegressionTestDraft':
      path = '/api/feedback/review/test-draft';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ApproveQARegressionTestDraft':
      path = '/api/feedback/review/regression-approval';
      init = jsonPost(args[0] ?? {});
      break;
    case 'GenerateQARegressionSourcePatch':
      path = '/api/feedback/review/source-patch';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ApproveQARegressionSourcePatchApplyPlan':
      path = '/api/feedback/review/source-patch/apply-plan';
      init = jsonPost(args[0] ?? {});
      break;
    case 'PlanQARegressionSourceWrite':
      path = '/api/feedback/review/source-patch/source-write/plan';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ApplyQARegressionSourceWrite':
      path = '/api/feedback/review/source-patch/source-write/apply';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ListRouteCorrections':
      path = `/api/learn/route-corrections?limit=${encodeURIComponent(String(args[0] ?? 50))}`;
      break;
    case 'ApproveRouteCorrection':
      path = '/api/learn/route-correction/approve';
      init = jsonPost({ id: args[0] ?? '' });
      break;
    case 'DisableRouteCorrection':
      path = '/api/learn/route-correction/disable';
      init = jsonPost({ id: args[0] ?? '' });
      break;
    case 'ConnectorRegistry':
      path = '/api/connectors/registry';
      break;
    case 'ListGeneratedConnectors':
      path = '/api/connectors/generated';
      break;
    case 'InstallGeneratedConnector':
      path = '/api/connectors/generated/install';
      init = jsonPost(args[0] ?? {});
      break;
    case 'SetGeneratedConnectorEnabled':
      path = '/api/connectors/generated/enabled';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ListModels':
      path = '/api/models';
      break;
    case 'ListModelProfiles':
      path = '/api/model-profiles';
      break;
    case 'ShowModel':
      path = `/api/models/show?name=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'ShowModelProfile':
      path = `/api/model-profiles/show?name=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'PreviewModelProfile':
      path = '/api/model-profiles/preview';
      init = jsonPost(args[0] ?? {});
      break;
    case 'DraftModelProfileFromConversation':
      path = '/api/model-profiles/draft';
      init = jsonPost(args[0] ?? {});
      break;
    case 'SaveModelProfile':
      path = '/api/model-profiles';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ApplyModelProfile':
      path = '/api/model-profiles/apply';
      init = jsonPost(args[0] ?? {});
      break;
    case 'GenerateModel':
      path = '/api/models/generate';
      init = jsonPost({ name: args[0] ?? '', prompt: args[1] ?? '' });
      break;
    case 'SearchMemory':
      path = `/api/memory?query=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'KnowledgeStatus':
      path = '/api/knowledge/status';
      break;
    case 'SetKnowledgeEnabled':
      path = '/api/knowledge/enabled';
      init = jsonPost(typeof args[0] === 'object' && args[0] !== null ? args[0] : { enabled: Boolean(args[0]) });
      break;
    case 'RepairKnowledgeEvidence':
      path = '/api/knowledge/repair';
      init = jsonPost({});
      break;
    case 'ListKnowledgeReviewItems':
      path = `/api/knowledge/review?status=${encodeURIComponent(String(args[0] ?? 'pending'))}&limit=${encodeURIComponent(String(args[1] ?? 25))}`;
      break;
    case 'ApplyKnowledgeReviewBatch':
      path = '/api/knowledge/review';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ListKnowledgeEntities':
      path = `/api/knowledge/entities?query=${encodeURIComponent(String(args[0] ?? ''))}&limit=${encodeURIComponent(String(args[1] ?? 12))}`;
      break;
    case 'DraftKnowledgeProposal':
      path = '/api/knowledge/proposals';
      init = jsonPost(args[0] ?? {});
      break;
    case 'SaveKnowledgeEntity':
      path = '/api/knowledge/entities';
      init = jsonPost(args[0] ?? {});
      break;
    case 'SaveKnowledgeEdge':
      path = '/api/knowledge/edges';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ListConversations':
      path = `/api/conversations?limit=${encodeURIComponent(String(args[0] ?? 50))}`;
      break;
    case 'GetConversation':
      path = `/api/conversations/${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'RenameConversation':
      path = '/api/conversations/rename';
      init = jsonPost({ conversationId: args[0] ?? '', title: args[1] ?? '' });
      break;
    case 'SetConversationStarred':
      path = '/api/conversations/star';
      init = jsonPost({ conversationId: args[0] ?? '', starred: Boolean(args[1]) });
      break;
    case 'DeleteConversation':
      path = '/api/conversations/delete';
      init = jsonPost({ conversationId: args[0] ?? '' });
      break;
    case 'EditUserMessage':
      path = '/api/messages/edit-user';
      init = jsonPost({ messageId: args[0] ?? '', content: args[1] ?? '' });
      break;
    case 'ListMemories':
      path = `/api/memories?limit=${encodeURIComponent(String(args[0] ?? 50))}`;
      break;
    case 'WriteMemory':
      path = '/api/memories';
      init = jsonPost(args[0] ?? {});
      break;
    case 'PinMemory':
      path = '/api/memories/pin';
      init = jsonPost({ id: args[0] ?? '' });
      break;
    case 'DeleteMemory':
      path = '/api/memories/delete';
      init = jsonPost({ id: args[0] ?? '' });
      break;
    case 'SearchDocuments':
      path = `/api/documents?query=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'ListDocumentInventory':
      path = `/api/documents/inventory?limit=${encodeURIComponent(String(args[0] ?? 100))}`;
      break;
    case 'PruneMissingDocuments':
      path = '/api/documents/prune-missing';
      init = jsonPost({ confirm: Boolean(args[0]) });
      break;
    case 'SuggestDocumentPaths':
      path = `/api/documents/suggestions?path=${encodeURIComponent(String(args[0] ?? ''))}&limit=${encodeURIComponent(String(args[1] ?? 40))}`;
      break;
    case 'ListToolRuns':
      path = `/api/tool-runs?limit=${encodeURIComponent(String(args[0] ?? 50))}`;
      break;
    case 'ToolCatalog':
      path = `/api/tools/catalog?surface=${encodeURIComponent(String(args[0] ?? 'chat-callable'))}`;
      break;
    case 'ListNotifications':
      path = `/api/notifications?limit=${encodeURIComponent(String(args[0] ?? 30))}&includeDismissed=${args[1] ? 'true' : 'false'}`;
      break;
    case 'MarkNotificationRead':
      path = '/api/notifications/read';
      init = jsonPost({ id: args[0] ?? '' });
      break;
    case 'DismissNotification':
      path = '/api/notifications/dismiss';
      init = jsonPost({ id: args[0] ?? '' });
      break;
    case 'ListReplayTraces':
      path = `/api/replay?limit=${encodeURIComponent(String(args[0] ?? 30))}`;
      break;
    case 'ExplainReplayTrace':
      path = `/api/replay/explain?id=${encodeURIComponent(String(args[0] ?? ''))}`;
      break;
    case 'RecordPermissionDecision':
      path = '/api/permissions/decision';
      init = jsonPost(args[0] ?? {});
      break;
    case 'PlanFileWrite':
      path = '/api/files/plan-write';
      init = jsonPost(args[0] ?? {});
      break;
    case 'ApplyFileWrite':
      path = '/api/files/apply-write';
      init = jsonPost(args[0] ?? {});
      break;
    case 'UploadChatAttachment': {
      path = '/api/chat/attachments';
      const input = args[0];
      if (isBrowserFile(input)) {
        const form = new FormData();
        form.append('file', input, input.name);
        init = {
          method: 'POST',
          body: form
        };
      } else {
        init = jsonPost(input ?? {});
      }
      break;
    }
    case 'IngestDocuments':
      path = '/api/documents/ingest';
      init = jsonPost({ path: args[0] ?? '' });
      break;
    case 'IndexEmbeddings':
      path = '/api/documents/embeddings/index';
      init = jsonPost({});
      break;
    case 'ListWorkspaceGrants':
      path = '/api/workspace/grants';
      break;
    case 'GrantWorkspace':
      path = '/api/workspace/grants';
      init = jsonPost({ path: args[0] ?? '', label: args[1] ?? '' });
      break;
    case 'RevokeWorkspaceGrant':
      path = '/api/workspace/grants/revoke';
      init = jsonPost({ match: args[0] ?? '' });
      break;
    case 'SetModelRole':
      path = '/api/models/role';
      init = jsonPost({ role: args[0] ?? '', name: args[1] ?? '' });
      break;
    case 'SetModelRoleProvider':
      path = '/api/models/role';
      init = jsonPost({ role: args[0] ?? '', name: args[1] ?? '', provider: args[2] ?? '', baseUrl: args[3] ?? '' });
      break;
    case 'CompleteSetup':
      path = '/api/setup';
      init = jsonPost(args[0] ?? {});
      break;
    case 'Ask':
      path = '/api/ask';
      init = jsonPost({
        content: args[0] ?? '',
        skill: args[1] ?? '',
        conversationId: args[2] ?? '',
        parentMessageId: args[3] ?? '',
        attachmentIds: Array.isArray(args[4]) ? args[4] : []
      });
      if (activeRequestSignal) init.signal = activeRequestSignal;
      break;
    case 'Chat':
      path = '/api/chat';
      init = jsonPost({ content: args[0] ?? '', conversationId: args[1] ?? '' });
      if (activeRequestSignal) init.signal = activeRequestSignal;
      break;
    default:
      throw new Error(`Unsupported local web call: ${name}`);
  }
  const response = await fetch(path, init);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(payload.error || `${name} failed`);
  }
  return payload as T;
}

export function jsonPost(body: unknown): RequestInit {
  return {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  };
}

function runtimeControlPost(): RequestInit {
  return {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Yemaka-Runtime-Control': '1'
    },
    body: '{}'
  };
}

function isBrowserFile(value: unknown): value is File {
  return typeof File !== 'undefined' && value instanceof File;
}
