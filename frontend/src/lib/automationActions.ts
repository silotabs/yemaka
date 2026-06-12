import type { ConnectorRegistryEntry, GeneratedConnectorActionResult, GeneratedConnectorEntry, HeartbeatReport, Job, JobRun, JobStatus, JobTickResult } from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';

export type AutomationSurface = {
  jobStatus: JobStatus;
  jobs: Job[];
  jobRuns: JobRun[];
  heartbeatReport: HeartbeatReport;
  connectorRegistry: ConnectorRegistryEntry[];
  generatedConnectors: GeneratedConnectorEntry[];
};

export type SchedulerJobInput = {
  scheduleType: string;
  scheduleExpr: string;
  targetType: string;
  targetName: string;
  input: Record<string, unknown>;
  approved: boolean;
  enabled: boolean;
};

export type SchedulerJobUpdateInput = {
  id: string;
  input: Record<string, unknown>;
  approved: boolean;
};

export type SchedulerJobArchiveInput = {
  id: string;
  approved: boolean;
};

export async function loadAutomationSurface(runLimit = 20): Promise<AutomationSurface> {
  return {
    jobStatus: await call<JobStatus>('JobStatus'),
    jobs: asArray(await call<Job[]>('ListJobs')),
    jobRuns: asArray(await call<JobRun[]>('JobRuns', runLimit)),
    heartbeatReport: await call<HeartbeatReport>('HeartbeatStatus'),
    connectorRegistry: asArray(await call<ConnectorRegistryEntry[]>('ConnectorRegistry')),
    generatedConnectors: asArray(await call<GeneratedConnectorEntry[]>('ListGeneratedConnectors'))
  };
}

export async function createSchedulerJob(input: SchedulerJobInput) {
  return await call<Job>('CreateJob', input);
}

export async function updateSchedulerJobInput(input: SchedulerJobUpdateInput) {
  return await call<Job>('UpdateJobInput', input);
}

export async function setSchedulerJobEnabled(id: string, enabled: boolean) {
  return await call<Job>('SetJobEnabled', { id, enabled });
}

export async function archiveSchedulerJob(id: string) {
  return await call<Job>('ArchiveJob', { id, approved: true } satisfies SchedulerJobArchiveInput);
}

export async function runSchedulerJob(id: string, conversationId: string) {
  return await call<JobRun>('RunJob', { id, conversationId });
}

export async function runDueSchedulerJobs() {
  return await call<JobTickResult>('RunDueJobs');
}

export async function setGeneratedConnectorEnabledAction(name: string, enabled: boolean) {
  return await call<GeneratedConnectorActionResult>('SetGeneratedConnectorEnabled', { name, enabled });
}

export function jobTickSummary(result: JobTickResult) {
  const runCount = result.runs?.length ?? 0;
  let summary = runCount > 0 ? `ran ${runCount} due job${runCount === 1 ? '' : 's'}` : 'no due jobs at this time';
  if (result.skipped > 0) {
    summary += `; ${result.skipped} skipped by parallel limit`;
  }
  return summary;
}
