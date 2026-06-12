import type {
  ConnectorRegistryEntry,
  GeneratedConnectorEntry,
  HeartbeatReport,
  InternetCacheSummary,
  InternetCrawlRunRecord,
  InternetRequestRecord,
  InternetStatus,
  Job,
  JobRun,
  JobStatus
} from './appTypes';
import {
  archiveSchedulerJob,
  createSchedulerJob,
  jobTickSummary,
  loadAutomationSurface,
  runDueSchedulerJobs,
  runSchedulerJob,
  setSchedulerJobEnabled,
  updateSchedulerJobInput
} from './automationActions';
import { loadInternetSurface } from './internetActions';
import { schedulerJobConversationId, schedulerJobCreatedSummary, schedulerJobInputFromForm, schedulerJobUpdateInputFromJSON } from './schedulerJobHelpers';

export type AutomationControllerContext = {
  getScheduleType: () => string;
  getScheduleExpr: () => string;
  getTargetType: () => string;
  getTargetName: () => string;
  getInputJSON: () => string;
  getJobs: () => Job[];
  setTargetName: (value: string) => void;
  setJobSummary: (summary: string) => void;
  setJobStatus: (status: JobStatus) => void;
  setJobs: (jobs: Job[]) => void;
  setJobRuns: (runs: JobRun[]) => void;
  setHeartbeatReport: (report: HeartbeatReport) => void;
  setInternetStatus: (status: InternetStatus) => void;
  setInternetRequests: (requests: InternetRequestRecord[]) => void;
  setInternetCrawls: (crawls: InternetCrawlRunRecord[]) => void;
  setInternetCache: (cache: InternetCacheSummary[]) => void;
  setConnectorRegistry: (connectors: ConnectorRegistryEntry[]) => void;
  setGeneratedConnectors: (connectors: GeneratedConnectorEntry[]) => void;
  setJobFailureDetail: (id: string, message: string) => void;
  clearJobFailureDetail: (id: string) => void;
  setAutomationBusy: (busy: boolean) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
  openConversation: (conversationId: string) => Promise<void>;
  confirm: (message: string) => boolean | Promise<boolean>;
};

export function createAutomationController(ctx: AutomationControllerContext) {
  async function refreshAutomation(silent = false) {
    if (!silent) {
      ctx.setError('');
      ctx.setAutomationBusy(true);
    }
    try {
      const automationSurface = await loadAutomationSurface(20);
      ctx.setJobStatus(automationSurface.jobStatus);
      ctx.setJobs(automationSurface.jobs);
      ctx.setJobRuns(automationSurface.jobRuns);
      ctx.setHeartbeatReport(automationSurface.heartbeatReport);
      const internetSurface = await loadInternetSurface(20);
      ctx.setInternetStatus(internetSurface.status);
      ctx.setInternetRequests(internetSurface.requests);
      ctx.setInternetCrawls(internetSurface.crawls);
      ctx.setInternetCache(internetSurface.cache);
      ctx.setConnectorRegistry(automationSurface.connectorRegistry);
      ctx.setGeneratedConnectors(automationSurface.generatedConnectors);
      if (!silent) ctx.pushActivity(`heartbeat: ${automationSurface.heartbeatReport.overall}`);
    } catch (err) {
      if (!silent) ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      if (!silent) ctx.setAutomationBusy(false);
    }
  }

  async function createJob() {
    ctx.setError('');
    ctx.setJobSummary('');
    try {
      const job = await createSchedulerJob(
        schedulerJobInputFromForm({
          scheduleType: ctx.getScheduleType(),
          scheduleExpr: ctx.getScheduleExpr(),
          targetType: ctx.getTargetType(),
          targetName: ctx.getTargetName(),
          inputJSON: ctx.getInputJSON()
        })
      );
      ctx.setJobSummary(schedulerJobCreatedSummary(job));
      ctx.setTargetName('');
      ctx.clearJobFailureDetail(job.id);
      ctx.pushActivity(`job created: ${job.name || job.id}`);
      await refreshAutomation();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function updateJobInput(id: string, inputJSON: string) {
    ctx.setError('');
    ctx.setJobSummary('');
    try {
      const job = await updateSchedulerJobInput(schedulerJobUpdateInputFromJSON(id, inputJSON));
      ctx.clearJobFailureDetail(id);
      ctx.setJobSummary(`${job.id}: input updated`);
      ctx.pushActivity(`job input updated: ${job.id}`);
      await refreshAutomation();
      return true;
    } catch (err) {
      const message = schedulerJobUpdateErrorMessage(err instanceof Error ? err.message : String(err));
      ctx.setJobFailureDetail(id, message);
      ctx.setError(message);
      return false;
    }
  }

  async function setJobEnabled(id: string, enabled: boolean) {
    ctx.setError('');
    ctx.setJobSummary('');
    try {
      const job = await setSchedulerJobEnabled(id, enabled);
      ctx.clearJobFailureDetail(id);
      ctx.setJobSummary(`${job.id}: ${job.enabled ? 'enabled' : 'disabled'}`);
      ctx.pushActivity(`job ${job.enabled ? 'enabled' : 'disabled'}: ${job.id}`);
      await refreshAutomation();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setJobFailureDetail(id, message);
      ctx.setError(message);
    }
  }

  async function archiveJob(id: string) {
    ctx.setError('');
    ctx.setJobSummary('');
    const job = ctx.getJobs().find((candidate) => candidate.id === id);
    const label = job?.name || job?.targetName || id;
    const confirmed = await ctx.confirm(
      `Archive ${label}? This disables the scheduled job and hides it from active automation. Run history stays available for audit.`
    );
    if (!confirmed) return;
    try {
      const archived = await archiveSchedulerJob(id);
      ctx.clearJobFailureDetail(id);
      ctx.setJobSummary(`${archived.id}: archived`);
      ctx.pushActivity(`job archived: ${archived.name || archived.id}`);
      await refreshAutomation();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setJobFailureDetail(id, message);
      ctx.setError(message);
    }
  }

  async function runJob(id: string) {
    ctx.setError('');
    ctx.setJobSummary('');
    try {
      const conversationId = schedulerJobConversationId(ctx.getJobs().find((job) => job.id === id));
      const run = await runSchedulerJob(id, conversationId);
      ctx.clearJobFailureDetail(id);
      ctx.setJobSummary(`${run.jobId}: ${run.status}`);
      ctx.pushActivity(`job run: ${run.status}`);
      await refreshAutomation();
      if (conversationId) {
        await ctx.openConversation(conversationId);
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setJobFailureDetail(id, message);
      ctx.setError(message);
    }
  }

  async function runDueJobs() {
    ctx.setError('');
    ctx.setJobSummary('');
    try {
      const tickResult = await runDueSchedulerJobs();
      const summary = jobTickSummary(tickResult);
      ctx.setJobSummary(summary);
      ctx.pushActivity(`job tick: ${summary}`);
      await refreshAutomation();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    refreshAutomation,
    createJob,
    updateJobInput,
    setJobEnabled,
    archiveJob,
    runJob,
    runDueJobs
  };
}

function schedulerJobUpdateErrorMessage(message: string) {
  const lower = String(message || '').toLowerCase();
  if (lower.includes('updatejobinput') || lower.includes('/api/jobs/input') || lower.includes('unsupported local web call')) {
    return 'Scheduler job input update is not available yet. Backend needs UpdateJobInput or /api/jobs/input.';
  }
  return message;
}
