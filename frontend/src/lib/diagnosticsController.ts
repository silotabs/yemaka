import type { FileWritePlan, NotificationItem, QAReview, ReplayTraceExplanation, ReplayTraceFile } from './appTypes';
import {
  applyFileWritePlan,
  approveReviewedQARegressionSourcePatchApplyPlan,
  approveReviewedQARegressionTestDraft,
  diagnosticsSummary,
  dismissNotificationItem,
  explainReplayTraceById,
  generateReviewedQARegressionSourcePatch,
  generateReviewedQARegressionTestDraft,
  loadDiagnosticsSurface,
  markNotificationAsRead,
  applyReviewedQARegressionSourceWrite,
  planReviewedQARegressionSourceWrite,
  planFileWrite,
  promoteReviewedQARegressionSuggestion,
  reviewQARegressionSuggestionStatus
} from './diagnosticActions';

export type DiagnosticsControllerContext = {
  getFileWritePath: () => string;
  getFileWriteContent: () => string;
  getNotificationBusy: () => Record<string, boolean>;
  setFileWritePlan: (plan: FileWritePlan | null) => void;
  setFileWriteSummary: (summary: string) => void;
  setNotifications: (notifications: NotificationItem[]) => void;
  setReplayTraces: (traces: ReplayTraceFile[]) => void;
  setQAReview: (review: QAReview) => void;
  setDiagnosticsSummary: (summary: string) => void;
  setNotificationBusy: (busy: Record<string, boolean>) => void;
  setReplayExplainBusy: (busy: boolean) => void;
  setSelectedReplayTraceId: (id: string) => void;
  setReplayExplanation: (explanation: ReplayTraceExplanation | null) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  refreshToolRuns: () => Promise<void>;
};

export function createDiagnosticsController(ctx: DiagnosticsControllerContext) {
  async function previewFileWrite() {
    ctx.setError('');
    ctx.setFileWriteSummary('');
    try {
      const plan = await planFileWrite(ctx.getFileWritePath(), ctx.getFileWriteContent());
      ctx.setFileWritePlan(plan);
      ctx.pushActivity(`file preview: ${plan.path}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function applyFileWrite() {
    ctx.setError('');
    ctx.setFileWriteSummary('');
    try {
      const result = await applyFileWritePlan(ctx.getFileWritePath(), ctx.getFileWriteContent());
      ctx.setFileWritePlan(result);
      const verification = result.verification?.status ? `; verification ${result.verification.status}` : '';
      ctx.setFileWriteSummary(result.snapshotId ? `snapshot ${result.snapshotId}${verification}` : `applied${verification}`);
      ctx.pushActivity(`file applied: ${result.path}`);
      await ctx.refreshToolRuns();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function loadDiagnostics() {
    const diagnostics = await loadDiagnosticsSurface(30);
    ctx.setNotifications(diagnostics.notifications);
    ctx.setReplayTraces(diagnostics.replayTraces);
    ctx.setQAReview(diagnostics.qaReview);
  }

  async function refreshDiagnostics() {
    ctx.setError('');
    ctx.setDiagnosticsSummary('');
    try {
      const diagnostics = await loadDiagnosticsSurface(30);
      ctx.setNotifications(diagnostics.notifications);
      ctx.setReplayTraces(diagnostics.replayTraces);
      ctx.setQAReview(diagnostics.qaReview);
      ctx.setDiagnosticsSummary(diagnosticsSummary(diagnostics));
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function markNotificationRead(id: string) {
    if (!id) return;
    ctx.setNotificationBusy({ ...ctx.getNotificationBusy(), [id]: true });
    try {
      await markNotificationAsRead(id);
      await loadDiagnostics();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      const next = { ...ctx.getNotificationBusy() };
      delete next[id];
      ctx.setNotificationBusy(next);
    }
  }

  async function dismissNotification(id: string) {
    if (!id) return;
    ctx.setNotificationBusy({ ...ctx.getNotificationBusy(), [id]: true });
    try {
      await dismissNotificationItem(id);
      await loadDiagnostics();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      const next = { ...ctx.getNotificationBusy() };
      delete next[id];
      ctx.setNotificationBusy(next);
    }
  }

  async function explainReplayTrace(id: string) {
    if (!id) return;
    ctx.setReplayExplainBusy(true);
    ctx.setSelectedReplayTraceId(id);
    try {
      ctx.setReplayExplanation(await explainReplayTraceById(id));
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setReplayExplainBusy(false);
    }
  }

  async function promoteQARegressionSuggestion(traceId: string, suggestionName = '') {
    if (!traceId) return null;
    ctx.setError('');
    try {
      const artifact = await promoteReviewedQARegressionSuggestion(traceId, suggestionName);
      ctx.setDiagnosticsSummary(`Reviewed regression artifact saved: ${artifact.name}`);
      await loadDiagnostics();
      return artifact;
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  async function reviewQARegressionSuggestion(traceId: string, suggestionName = '', status: 'approved' | 'dismissed' | 'covered') {
    if (!traceId || !status) return null;
    ctx.setError('');
    try {
      const record = await reviewQARegressionSuggestionStatus(traceId, suggestionName, status);
      ctx.setDiagnosticsSummary(`QA regression ${status}: ${record.name || record.kind || traceId}`);
      await loadDiagnostics();
      return record;
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  async function generateQARegressionTestDraft(traceId: string, suggestionName = '') {
    if (!traceId) return null;
    ctx.setError('');
    try {
      const draft = await generateReviewedQARegressionTestDraft(traceId, suggestionName);
      ctx.setDiagnosticsSummary(`QA regression test draft saved: ${draft.name}`);
      await loadDiagnostics();
      return draft;
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  async function approveQARegressionTestDraft(traceId: string, suggestionName = '', draftName = '') {
    if (!traceId) return null;
    ctx.setError('');
    try {
      const approval = await approveReviewedQARegressionTestDraft(traceId, suggestionName, draftName);
      ctx.setDiagnosticsSummary(`QA regression approved: ${approval.name}`);
      await loadDiagnostics();
      return approval;
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  async function generateQARegressionSourcePatch(traceId: string, suggestionName = '', draftName = '') {
    if (!traceId) return null;
    ctx.setError('');
    try {
      const patch = await generateReviewedQARegressionSourcePatch(traceId, suggestionName, draftName);
      ctx.setDiagnosticsSummary(`QA source patch draft saved: ${patch.name}`);
      await loadDiagnostics();
      return patch;
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  async function approveQARegressionSourcePatchApplyPlan(traceId: string, suggestionName = '', sourcePatchName = '') {
    if (!traceId) return null;
    ctx.setError('');
    try {
      const plan = await approveReviewedQARegressionSourcePatchApplyPlan(traceId, suggestionName, sourcePatchName);
      ctx.setDiagnosticsSummary(`QA source patch apply plan approved: ${plan.name}`);
      await loadDiagnostics();
      return plan;
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  async function planQARegressionSourceWrite(traceId: string, suggestionName = '', sourcePatchName = '', content = '') {
    if (!traceId) return null;
    ctx.setError('');
    try {
      return await planReviewedQARegressionSourceWrite(traceId, suggestionName, sourcePatchName, content);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  async function applyQARegressionSourceWrite(traceId: string, suggestionName = '', sourcePatchName = '', content = '') {
    if (!traceId) return null;
    ctx.setError('');
    try {
      const result = await applyReviewedQARegressionSourceWrite(traceId, suggestionName, sourcePatchName, content);
      ctx.setDiagnosticsSummary(`QA source test written: ${result.record?.name || traceId}`);
      await loadDiagnostics();
      return result;
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
      return null;
    }
  }

  return {
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
  };
}
