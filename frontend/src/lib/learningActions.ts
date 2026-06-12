import type { LearningReport, MemoryResult, PolicyStatus, Trajectory } from './appTypes';
import { call } from './api';

export type LearningSurface = {
  report: LearningReport;
  policy: PolicyStatus;
};

export async function loadLearningSurface(): Promise<LearningSurface> {
  return {
    report: await call<LearningReport>('LearningReport'),
    policy: await call<PolicyStatus>('PolicyStatus')
  };
}

export async function setLearningPolicyMode(mode: string) {
  return await call<PolicyStatus>('SetPolicyMode', mode);
}

export async function saveRouteCorrection(conversationId: string, correction: string) {
  return await call<MemoryResult>('SaveCorrection', { conversationId, correction });
}

export async function exportConversationTrajectory(conversationId: string) {
  return await call<Trajectory>('ExportTrajectory', conversationId);
}

export function learningReportActivity(report: LearningReport) {
  return `learning report: ${report.workflowSuccesses} success memories`;
}

export function policyModeSummary(policy: PolicyStatus) {
  return `policy mode: ${policy.mode}`;
}

export function trajectorySummary(trajectory: Trajectory) {
  return `${trajectory.schema} exported locally in memory view`;
}
