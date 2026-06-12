import type { ExtensionFailureTrend, LearningReport, PolicyStatus } from './appTypes';
import { listExtensionFailures } from './extensionActions';
import {
  exportConversationTrajectory,
  learningReportActivity,
  loadLearningSurface,
  policyModeSummary,
  saveRouteCorrection,
  setLearningPolicyMode,
  trajectorySummary
} from './learningActions';
import { previewJSON } from './uiHelpers';

export type LearningControllerContext = {
  getCorrectionConversationId: () => string;
  getCorrectionContent: () => string;
  getTrajectoryConversationId: () => string;
  setCorrectionContent: (content: string) => void;
  setLearningReport: (report: LearningReport) => void;
  setPolicyStatus: (policy: PolicyStatus) => void;
  setExtensionFailures: (failures: ExtensionFailureTrend[]) => void;
  setLearningBusy: (busy: boolean) => void;
  setLearningSummary: (summary: string) => void;
  setPolicySummary: (summary: string) => void;
  setTrajectoryPreview: (preview: string) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  loadDiagnostics: () => Promise<void>;
};

export function createLearningController(ctx: LearningControllerContext) {
  async function refreshLearning(silent = false) {
    if (!silent) {
      ctx.setError('');
      ctx.setLearningSummary('');
      ctx.setLearningBusy(true);
    }
    try {
      const learningSurface = await loadLearningSurface();
      ctx.setLearningReport(learningSurface.report);
      ctx.setPolicyStatus(learningSurface.policy);
      ctx.setExtensionFailures(await listExtensionFailures(20));
      await ctx.loadDiagnostics();
      if (!silent) ctx.pushActivity(learningReportActivity(learningSurface.report));
    } catch (err) {
      if (!silent) ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      if (!silent) ctx.setLearningBusy(false);
    }
  }

  async function setPolicyMode(mode: string) {
    ctx.setError('');
    ctx.setPolicySummary('');
    try {
      const policy = await setLearningPolicyMode(mode);
      const summary = policyModeSummary(policy);
      ctx.setPolicyStatus(policy);
      ctx.setPolicySummary(summary);
      ctx.pushActivity(summary);
      await refreshLearning();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function saveCorrection() {
    ctx.setError('');
    ctx.setLearningSummary('');
    try {
      const item = await saveRouteCorrection(ctx.getCorrectionConversationId(), ctx.getCorrectionContent());
      ctx.setLearningSummary(`correction saved: ${item.id}`);
      ctx.setCorrectionContent('');
      ctx.pushActivity('learning correction saved');
      await refreshLearning();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function exportTrajectory() {
    ctx.setError('');
    ctx.setLearningSummary('');
    ctx.setTrajectoryPreview('');
    try {
      const trajectory = await exportConversationTrajectory(ctx.getTrajectoryConversationId());
      ctx.setTrajectoryPreview(previewJSON(trajectory));
      ctx.setLearningSummary(trajectorySummary(trajectory));
      ctx.pushActivity(`trajectory export: ${trajectory.conversationId}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    refreshLearning,
    setPolicyMode,
    saveCorrection,
    exportTrajectory
  };
}
