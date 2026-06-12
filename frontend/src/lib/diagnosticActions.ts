import type {
  FileWriteApplyResult,
  FileWritePlan,
  NotificationItem,
  PermissionDecisionResult,
  PermissionItem,
  QARegressionApprovedRecord,
  QARegressionSourcePatchApplyPlanRecord,
  QARegressionPromotionArtifact,
  QARegressionReviewRecord,
  QARegressionSourcePatchRecord,
  QARegressionSourceWriteApplyResult,
  QARegressionSourceWritePlanResult,
  QARegressionTestDraftRecord,
  QAReview,
  ReplayTraceExplanation,
  ReplayTraceFile,
  ToolCatalogEntry,
  ToolRun
} from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';
import { permissionPayload } from './diagnosticHelpers';

export type ToolActivitySurface = {
  toolRuns: ToolRun[];
  toolCatalog: ToolCatalogEntry[];
};

export type DiagnosticsSurface = {
  notifications: NotificationItem[];
  replayTraces: ReplayTraceFile[];
  qaReview: QAReview;
};

export async function loadToolActivity(limit = 50): Promise<ToolActivitySurface> {
  return {
    toolRuns: asArray(await call<ToolRun[]>('ListToolRuns', limit)),
    toolCatalog: asArray(await call<ToolCatalogEntry[]>('ToolCatalog', 'chat-callable'))
  };
}

export async function loadDiagnosticsSurface(limit = 30): Promise<DiagnosticsSurface> {
  return {
    notifications: asArray(await call<NotificationItem[]>('ListNotifications', limit, false)),
    replayTraces: asArray(await call<ReplayTraceFile[]>('ListReplayTraces', limit)),
    qaReview: await call<QAReview>('QAReview', limit)
  };
}

export async function markNotificationAsRead(id: string) {
  return await call<NotificationItem>('MarkNotificationRead', id);
}

export async function dismissNotificationItem(id: string) {
  return await call<NotificationItem>('DismissNotification', id);
}

export async function explainReplayTraceById(id: string) {
  return await call<ReplayTraceExplanation>('ExplainReplayTrace', id);
}

export async function promoteReviewedQARegressionSuggestion(traceId: string, suggestionName = '') {
  return await call<QARegressionPromotionArtifact>('PromoteQARegressionSuggestion', {
    traceId,
    suggestionName,
    reviewedBy: 'qa_review'
  });
}

export async function reviewQARegressionSuggestionStatus(traceId: string, suggestionName: string, status: string) {
  return await call<QARegressionReviewRecord>('ReviewQARegressionSuggestion', {
    traceId,
    suggestionName,
    status,
    reviewedBy: 'qa_review'
  });
}

export async function generateReviewedQARegressionTestDraft(traceId: string, suggestionName = '') {
  return await call<QARegressionTestDraftRecord>('GenerateQARegressionTestDraft', {
    traceId,
    suggestionName,
    reviewedBy: 'qa_review'
  });
}

export async function approveReviewedQARegressionTestDraft(traceId: string, suggestionName = '', draftName = '') {
  return await call<QARegressionApprovedRecord>('ApproveQARegressionTestDraft', {
    traceId,
    suggestionName,
    draftName,
    reviewedBy: 'qa_review'
  });
}

export async function generateReviewedQARegressionSourcePatch(traceId: string, suggestionName = '', draftName = '') {
  return await call<QARegressionSourcePatchRecord>('GenerateQARegressionSourcePatch', {
    traceId,
    suggestionName,
    draftName,
    reviewedBy: 'qa_review'
  });
}

export async function approveReviewedQARegressionSourcePatchApplyPlan(traceId: string, suggestionName = '', sourcePatchName = '') {
  return await call<QARegressionSourcePatchApplyPlanRecord>('ApproveQARegressionSourcePatchApplyPlan', {
    traceId,
    suggestionName,
    sourcePatchName,
    approved: true,
    reviewedBy: 'qa_review'
  });
}

export async function planReviewedQARegressionSourceWrite(traceId: string, suggestionName = '', sourcePatchName = '', content = '') {
  return await call<QARegressionSourceWritePlanResult>('PlanQARegressionSourceWrite', {
    traceId,
    suggestionName,
    sourcePatchName,
    content,
    reviewedBy: 'qa_review'
  });
}

export async function applyReviewedQARegressionSourceWrite(traceId: string, suggestionName = '', sourcePatchName = '', content = '') {
  return await call<QARegressionSourceWriteApplyResult>('ApplyQARegressionSourceWrite', {
    traceId,
    suggestionName,
    sourcePatchName,
    content,
    approved: true,
    reviewedBy: 'qa_review'
  });
}

export async function recordPermissionDecision(item: PermissionItem, decision: 'approved' | 'rejected') {
  return await call<PermissionDecisionResult>('RecordPermissionDecision', permissionPayload(item, decision));
}

export async function planFileWrite(path: string, content: string) {
  return await call<FileWritePlan>('PlanFileWrite', { path, content });
}

export async function applyFileWritePlan(path: string, content: string) {
  return await call<FileWriteApplyResult>('ApplyFileWrite', { path, content, approved: true });
}

export function diagnosticsSummary(surface: DiagnosticsSurface) {
  return `Loaded ${surface.notifications.length} notifications, ${surface.replayTraces.length} replay traces, and ${surface.qaReview?.summary?.replayFailures ?? 0} QA failures.`;
}
