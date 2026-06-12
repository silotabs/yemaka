<script lang="ts">
  import { onMount } from 'svelte';
  import BitSelect from '../BitSelect.svelte';
  import ActionButton from '../ActionButton.svelte';
  import Badge from '../Badge.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import PaginationControls from '../PaginationControls.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import StructuredDataView from '../StructuredDataView.svelte';
  import type { InternetCrawlRunRecord, InternetStatus, OpsStage, OpsStatus, OpsTimelineEvent } from '../lib/appTypes';
  import { loadOpsStatus } from '../lib/opsActions';
  import {
    jsonObjectInputError,
    schedulerJobConversationId,
    schedulerJobFailureTitle,
    schedulerJobInputJSON,
    schedulerJobOriginLabel,
    schedulerJobRunOutputSummary
  } from '../lib/schedulerJobHelpers';

  type Option = {
    value: string;
    label: string;
  };

  type HeartbeatReport = {
    overall: string;
    generatedAt: string;
    checks: Array<{ id: string; name: string; status: string; detail: string; checkedAt: string }>;
  };

  type Job = {
    id: string;
    name: string;
    scheduleType: string;
    scheduleExpr: string;
    targetType: string;
    targetName: string;
    input: Record<string, unknown>;
    enabled: boolean;
    approved: boolean;
    retryPolicy?: string;
    maxAttempts?: number;
    backoffSeconds?: number;
    createdAt: string;
    updatedAt: string;
    lastRunAt: string;
    nextDueAt?: string;
    archivedAt?: string;
    inputStatus?: string;
    inputValidationError?: string;
  };

  type JobRun = {
    id: string;
    jobId: string;
    jobName?: string;
    targetType?: string;
    targetName?: string;
    scheduleType?: string;
    status: string;
    output: unknown;
    startedAt: string;
    finishedAt: string;
    durationMs: number;
    error: string;
  };

  type JobStatus = {
    enabled: boolean;
    totalJobs: number;
    enabledJobs: number;
    approvedJobs: number;
    runningJobs: number;
    maxParallelJobs: number;
    archivedJobs?: number;
    invalidInputJobs?: number;
    lastRunAt: string;
    lastRunStatus: string;
  };

  type InternetCacheSummary = {
    key: string;
    url: string;
    method: string;
    bodyBytes: number;
    cachedAt: string;
    expiresAt: string;
    expired: boolean;
    state?: string;
    ageSeconds?: number;
    expiresInSeconds?: number;
  };

  type InternetRequestRecord = {
    url: string;
    method: string;
    statusCode: number;
    allowed: boolean;
    error: string;
  };

  export let automationBusy = false;
  export let heartbeatReport: HeartbeatReport | null = null;
  export let jobStatus: JobStatus | null = null;
  export let internetStatus: InternetStatus | null = null;
  export let internetUrl = '';
  export let internetAllowedDomain = '';
  export let internetExtractText = true;
  export let internetTaskApproved = false;
  export let internetCrawlMaxPages = 8;
  export let internetCrawlMaxDepth = 1;
  export let internetCrawlMaxDurationSeconds = 30;
  export let internetCrawlMaxLinksPerPage = 50;
  export let internetCrawlMaxTextChars = 4000;
  export let internetFetchOutput = '';
  export let internetRequests: InternetRequestRecord[] = [];
  export let internetCrawls: InternetCrawlRunRecord[] = [];
  export let internetCache: InternetCacheSummary[] = [];
  export let jobScheduleType = 'interval';
  export let jobScheduleExpr = '1h';
  export let jobTargetName = '';
  export let jobTargetType = 'extension';
  export let jobInputJSON = '{}';
  export let jobs: Job[] = [];
  export let jobRuns: JobRun[] = [];
  export let jobFailureDetails: Record<string, string> = {};
  export let jobScheduleTypeOptions: Option[] = [];
  export let jobTargetTypeOptions: Option[] = [];
  export let listPages: Record<string, number> = {};
  export let pageSizeFor: (key: string) => number = () => 10;
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let refreshAutomation: () => Promise<void> | void = () => {};
  export let refreshInternet: () => Promise<void> | void = () => {};
  export let runDueJobs: () => Promise<void> | void = () => {};
  export let internetFetch: (method: 'GET' | 'HEAD') => Promise<void> | void = () => {};
  export let internetCrawl: (method?: 'GET' | 'HEAD') => Promise<void> | void = () => {};
  export let inspectInternetCrawl: (runId: string) => Promise<void> | void = () => {};
  export let createJob: () => Promise<void> | void = () => {};
  export let updateJobInput: (id: string, inputJSON: string) => Promise<boolean | void> | boolean | void = () => {};
  export let runJob: (id: string) => Promise<void> | void = () => {};
  export let setJobEnabled: (id: string, enabled: boolean) => Promise<void> | void = () => {};
  export let archiveJob: (id: string) => Promise<void> | void = () => {};
  export let openConversation: (id: string) => Promise<void> | void = () => {};
  export let humanizeIdentifier: (value: string | undefined, fallback?: string) => string = (value) => String(value || '');
  export let internetSearchReadiness: (statusValue: InternetStatus | null) => string = () => 'disabled';
  export let internetSearchProviderLabel: (statusValue: InternetStatus | null) => string = () => 'No provider';

  let editingJobId = '';
  let editingJobInputJSON = '{}';
  let editingJobOriginalInputJSON = '{}';
  let opsStatus: OpsStatus | null = null;
  let opsBusy = false;
  let opsError = '';
  let opsTimelineFilter = 'all';

  onMount(() => {
    void refreshOpsStatus(true);
  });

  $: internetUrlMissing = internetUrl.trim().length === 0;
  $: internetCrawlDisabled = internetUrlMissing || !internetTaskApproved;
  $: internetCrawlDisabledReason = internetUrlMissing
    ? 'Enter a seed URL before starting a bounded crawl.'
    : 'Check Task approved before starting a bounded crawl.';
  $: jobTargetMissing = jobTargetName.trim().length === 0;
  $: jobScheduleMissing = jobScheduleType !== 'manual' && jobScheduleExpr.trim().length === 0;
  $: jobInputJSONError = jsonObjectInputError(jobInputJSON, 'Job input');
  $: createJobDisabled = jobTargetMissing || jobScheduleMissing || Boolean(jobInputJSONError);
  $: createJobDisabledReason = jobInputJSONError
    ? jobInputJSONError
    : jobScheduleMissing
      ? 'Enter a schedule expression, or choose a manual job.'
      : 'Enter a job target before creating a scheduled job.';
  $: editingJobInputError = editingJobId ? jsonObjectInputError(editingJobInputJSON, 'Job input') : '';
  $: editingJobDirty = editingJobInputJSON.trim() !== editingJobOriginalInputJSON.trim();
  $: updateJobInputDisabled = Boolean(editingJobInputError) || !editingJobDirty;
  $: updateJobInputDisabledReason = editingJobInputError || 'Change the JSON object before saving this job input.';
  $: jobNamesById = jobs.reduce<Record<string, string>>((names, job) => {
    names[job.id] = job.name || job.targetName || job.id;
    return names;
  }, {});
  $: jobsById = jobs.reduce<Record<string, Job>>((items, job) => {
    items[job.id] = job;
    return items;
  }, {});
  $: opsStages = opsStatus?.stages ?? [];
  $: opsTimeline = opsStatus?.timeline ?? [];
  $: auditTimelineEvents = opsTimeline.filter((event) => isOpsAuditEvent(event));
  $: filteredOpsTimeline = opsTimelineFilter === 'audit' ? auditTimelineEvents : opsTimeline;
  $: opsSummaryMessage = opsStatus
    ? `${opsStages.length} stages | ${(opsStatus.notifications ?? []).length} notifications | ${auditTimelineEvents.length} audit events`
    : '';

  async function refreshOperatingStatus() {
    await Promise.all([refreshOpsStatus(false), Promise.resolve(refreshAutomation())]);
  }

  async function refreshOpsStatus(silent = false) {
    if (!silent) opsError = '';
    opsBusy = true;
    try {
      opsStatus = await loadOpsStatus(20, false);
      opsError = '';
    } catch (err) {
      opsError = err instanceof Error ? err.message : String(err);
    } finally {
      opsBusy = false;
    }
  }

  function startEditingJobInput(job: Job) {
    const inputJSON = schedulerJobInputJSON(job);
    editingJobId = job.id;
    editingJobInputJSON = inputJSON;
    editingJobOriginalInputJSON = inputJSON;
  }

  function cancelEditingJobInput() {
    editingJobId = '';
    editingJobInputJSON = '{}';
    editingJobOriginalInputJSON = '{}';
  }

  async function saveEditingJobInput(job: Job) {
    if (!editingJobId || updateJobInputDisabled) return;
    const updated = await updateJobInput(job.id, editingJobInputJSON);
    if (updated !== false) {
      cancelEditingJobInput();
    }
  }

  function jobRunStatusVariant(status: string): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
    const value = String(status || '').toLowerCase();
    if (value === 'success' || value === 'succeeded' || value === 'completed') return 'success';
    if (value === 'failed' || value === 'error' || value === 'broken') return 'danger';
    if (value === 'running') return 'info';
    if (value === 'skipped' || value === 'blocked') return 'warning';
    return 'neutral';
  }

  function jobRunFailureTitle(run: JobRun) {
    const job = jobs.find((item) => item.id === run.jobId);
    if (job) return schedulerJobFailureTitle(job, run.error);
    const lower = String(run.error || '').toLowerCase();
    if (lower.includes('extension input') || lower.includes('input.')) return 'Stored extension job input is invalid';
    return 'Job run failed';
  }

  function opsVariant(status: string | undefined): 'neutral' | 'success' | 'warning' | 'danger' | 'info' {
    const value = String(status || '').toLowerCase();
    if (value === 'healthy' || value === 'pass' || value === 'ready' || value === 'success' || value === 'completed') return 'success';
    if (value === 'broken' || value === 'fail' || value === 'failed' || value === 'error') return 'danger';
    if (value === 'warning' || value === 'warn' || value === 'needs_config' || value === 'needs_auth' || value === 'blocked') return 'warning';
    if (value === 'disabled') return 'neutral';
    if (value === 'running' || value === 'info' || value === 'recorded') return 'info';
    return 'neutral';
  }

  function opsStatusKind(status: string | undefined): 'info' | 'success' | 'warning' | 'error' {
    const variant = opsVariant(status);
    if (variant === 'success') return 'success';
    if (variant === 'danger') return 'error';
    if (variant === 'warning') return 'warning';
    return 'info';
  }

  function opsStageDetail(stage: OpsStage) {
    return stage.details || `${humanizeIdentifier(stage.name, 'Stage')} status is ${humanizeIdentifier(stage.status, 'unknown')}.`;
  }

  function opsEventTime(event: OpsTimelineEvent) {
    return formatOpsTime(event.occurredAt);
  }

  function formatOpsTime(value: string | undefined) {
    if (!value) return 'time not recorded';
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(date);
  }

  function opsEventMeta(event: OpsTimelineEvent) {
    const parts = [humanizeIdentifier(event.source, 'Source'), humanizeIdentifier(event.kind, 'Event')];
    if (event.entityType) parts.push(humanizeIdentifier(event.entityType, 'Entity'));
    return parts.filter(Boolean).join(' | ');
  }

  function isOpsAuditEvent(event: OpsTimelineEvent) {
    const source = String(event.source || '').toLowerCase();
    const kind = String(event.kind || '').toLowerCase();
    return source === 'audit' || source === 'policy_audit' || source === 'extensions' || kind.includes('permission') || kind.includes('policy');
  }

  function hasOpsMetadata(event: OpsTimelineEvent) {
    return Boolean(event.metadata && Object.keys(event.metadata).length > 0);
  }
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  {#if automationBusy}
    <div class="mb-4">
      <PageStatusStrip kind="info" title="Automation refresh running" message="Yemaka is checking heartbeat, scheduler, and automation state." />
    </div>
  {/if}

  <div class="mb-4 rounded-md border border-line bg-white">
    <div class="border-b border-line px-4 py-3">
      <SectionHeader
        compact
        icon="health"
        title="Operating Status"
        description="Composed local operations health across jobs, QA, replay, audit, and runtime checks."
        meta={opsStatus?.generatedAt ? `generated ${formatOpsTime(opsStatus.generatedAt)}` : 'not loaded'}
      >
        <svelte:fragment slot="actions">
          {#if opsStatus}
            <Badge variant={opsVariant(opsStatus.overall)}>{humanizeIdentifier(opsStatus.overall, 'Unknown')}</Badge>
          {/if}
          <ActionButton variant="secondary" icon="retry" busy={opsBusy || automationBusy} busyLabel="Refreshing" onclick={refreshOperatingStatus}>
            Refresh
          </ActionButton>
        </svelte:fragment>
      </SectionHeader>
    </div>
    <div class="p-4">
      {#if opsError}
        <PageStatusStrip
          kind="warning"
          title="Ops status unavailable"
          message={opsError}
          actionLabel="Retry"
          onAction={() => refreshOpsStatus(false)}
        />
      {:else if opsBusy && !opsStatus}
        <PageStatusStrip
          kind="info"
          title="Loading operating status"
          message="Yemaka is reading the local ops status endpoint."
        />
      {:else if opsStatus}
        <PageStatusStrip
          kind={opsStatusKind(opsStatus.overall)}
          title={`Operating loop ${humanizeIdentifier(opsStatus.overall, 'Unknown')}`}
          message={opsSummaryMessage}
        />
        <div class="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
          <div>
            <div class="mb-2 text-xs font-medium text-slate-500">Stages</div>
            <div class="divide-y divide-line rounded-md border border-line">
              {#each opsStages as stage}
                <div class="grid gap-3 px-3 py-2 text-sm md:grid-cols-[150px_120px_minmax(0,1fr)]">
                  <div class="font-medium">{humanizeIdentifier(stage.name, 'Stage')}</div>
                  <div><Badge variant={opsVariant(stage.status)}>{humanizeIdentifier(stage.status, 'Unknown')}</Badge></div>
                  <div class="break-words text-slate-600">{opsStageDetail(stage)}</div>
                </div>
              {/each}
              {#if opsStages.length === 0}
                <div class="p-3">
                  <EmptyState
                    icon="health"
                    title="No operating stages"
                    message="The ops status endpoint responded without stage data."
                  />
                </div>
              {/if}
            </div>
          </div>
          <div class="rounded-md border border-line bg-field p-3 text-sm">
            <div class="mb-2 text-xs font-medium text-slate-500">Operational Signals</div>
            <div class="grid grid-cols-2 gap-y-2 text-slate-700">
              <span>Scheduler</span><span>{opsStatus.scheduler?.enabled ? 'enabled' : 'disabled'}</span>
              <span>Recent runs</span><span>{(opsStatus.recentJobRuns ?? []).length}</span>
              <span>QA failures</span><span>{opsStatus.qaReview?.replayFailures ?? 0}</span>
              <span>Suggestions</span><span>{opsStatus.qaReview?.regressionSuggestions ?? 0}</span>
              <span>Latest eval</span><span>{opsStatus.latestEval?.status || 'none'}</span>
              <span>Audit events</span><span>{auditTimelineEvents.length}</span>
              <span>Extensions</span><span>{(opsStatus.extensionAudit ?? []).length} audits</span>
              <span>Policy</span><span>{(opsStatus.policyAudit ?? []).length} decisions</span>
              <span>Connectors</span><span>{opsStatus.connectors?.enabled ?? 0}/{opsStatus.connectors?.total ?? 0} enabled</span>
              <span>Knowledge</span><span>{opsStatus.knowledge?.enabled ? `${opsStatus.knowledge.entityCount} entities` : 'disabled'}</span>
              <span>Model profiles</span><span>{opsStatus.modelProfiles?.appliedRoles ?? 0} applied</span>
            </div>
          </div>
        </div>

        <div class="mt-4">
          <SectionHeader
            compact
            icon="automation"
            title="Recent Timeline"
            description="Notifications, scheduler runs, QA findings, replay traces, and audit events in one local view."
            meta={`${filteredOpsTimeline.length} shown`}
          >
            <svelte:fragment slot="actions">
              <BitSelect
                value={opsTimelineFilter}
                options={[
                  { value: 'all', label: 'All events', meta: `${opsTimeline.length}` },
                  { value: 'audit', label: 'Audit only', meta: `${auditTimelineEvents.length}` }
                ]}
                onChange={(value) => {
                  opsTimelineFilter = value;
                  setListPage('ops-timeline', 1);
                }}
              />
            </svelte:fragment>
          </SectionHeader>
          <div class="divide-y divide-line rounded-md border border-line">
            {#if filteredOpsTimeline.length === 0}
              <div class="p-4">
                <EmptyState
                  icon="automation"
                  title={opsTimelineFilter === 'audit' ? 'No audit timeline events' : 'No ops timeline events'}
                  message={opsTimelineFilter === 'audit' ? 'Policy, tool, and extension audit events will appear here after gated activity is recorded.' : 'Recent operating events will appear here after jobs, QA, replay, heartbeat, or tool activity is recorded.'}
                />
              </div>
            {/if}
            {#each pagedItems(filteredOpsTimeline, 'ops-timeline', pageSizeFor('ops-timeline'), listPages) as event}
              <div class="grid gap-3 px-3 py-3 text-sm lg:grid-cols-[190px_minmax(0,1fr)_120px]">
                <div>
                  <div class="font-medium text-slate-700">{opsEventTime(event)}</div>
                  <div class="mt-1 text-xs text-slate-500">{opsEventMeta(event)}</div>
                </div>
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="font-semibold">{event.title}</span>
                    <Badge variant={opsVariant(event.severity)}>{humanizeIdentifier(event.severity, 'Info')}</Badge>
                    {#if event.status}
                      <Badge variant={opsVariant(event.status)}>{humanizeIdentifier(event.status, 'Status')}</Badge>
                    {/if}
                  </div>
                  {#if event.summary}
                    <div class="mt-1 break-words text-slate-600">{event.summary}</div>
                  {/if}
                  {#if event.correlationId}
                    <div class="mt-1 break-all text-xs text-slate-500">correlation {event.correlationId}</div>
                  {/if}
                  {#if (event.related ?? []).length > 0}
                    <div class="mt-2 flex flex-wrap gap-1">
                      {#each event.related ?? [] as link}
                        <span class="rounded border border-line bg-field px-2 py-1 text-[11px] text-slate-600" title={`${link.source}:${link.entityType}:${link.entityId}`}>
                          {link.label || `${humanizeIdentifier(link.source, 'Related')} ${humanizeIdentifier(link.entityType, '')}`}
                        </span>
                      {/each}
                    </div>
                  {/if}
                  {#if hasOpsMetadata(event)}
                    <div class="mt-2">
                      <StructuredDataView value={event.metadata} title="Metadata" maxPreviewLines={4} />
                    </div>
                  {/if}
                </div>
                <div class="break-all text-xs text-slate-500">{event.entityId || event.id}</div>
              </div>
            {/each}
          </div>
          <PaginationControls
            page={currentPage('ops-timeline', filteredOpsTimeline.length, pageSizeFor('ops-timeline'), listPages)}
            total={filteredOpsTimeline.length}
            pageSize={pageSizeFor('ops-timeline')}
            label="events"
            onChange={(page) => setListPage('ops-timeline', page)}
          />
        </div>
      {:else}
        <EmptyState
          icon="health"
          title="Operating status not loaded"
          message="Refresh operating status to load staged health and recent timeline events from the local ops endpoint."
        />
      {/if}
    </div>
  </div>

  <div class="mb-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
    <div class="rounded-md border border-line bg-white">
      <div class="border-b border-line px-4 py-3">
        <SectionHeader
          compact
          icon="health"
          title="Heartbeat"
          description="Local runtime, scheduler, internet, connector, and extension health."
          meta={heartbeatReport?.generatedAt || 'not checked'}
        >
          <svelte:fragment slot="actions">
            <ActionButton variant="secondary" icon="retry" busy={automationBusy} busyLabel="Checking" onclick={refreshOperatingStatus}>
              Refresh
            </ActionButton>
          </svelte:fragment>
        </SectionHeader>
      </div>
      <div class="divide-y divide-line">
        {#if !heartbeatReport?.checks?.length}
          <div class="p-4">
            <EmptyState
              icon="health"
              title="No heartbeat checks recorded"
              message="Run a heartbeat refresh to see local runtime, scheduler, internet, connector, and extension health."
            />
          </div>
        {/if}
        {#each pagedItems(heartbeatReport?.checks, 'heartbeat-checks', pageSizeFor('heartbeat-checks'), listPages) as check}
          <div class="grid gap-3 px-4 py-3 text-sm md:grid-cols-[160px_90px_minmax(0,1fr)]">
            <div class="font-medium">{humanizeIdentifier(check.name == 'sqlite' ? 'Memory' : check.name)}</div>
            <div><Badge variant={check.status === 'healthy' ? 'success' : check.status === 'broken' ? 'danger' : check.status === 'disabled' ? 'neutral' : 'warning'}>{check.status}</Badge></div>
            <div class="break-all text-slate-600">{check.detail}</div>
          </div>
        {/each}
      </div>
      <PaginationControls
        page={currentPage('heartbeat-checks', heartbeatReport?.checks?.length ?? 0, pageSizeFor('heartbeat-checks'), listPages)}
        total={heartbeatReport?.checks?.length ?? 0}
        pageSize={pageSizeFor('heartbeat-checks')}
        label="checks"
        onChange={(page) => setListPage('heartbeat-checks', page)}
      />
    </div>
    <div class="rounded-md border border-line bg-white p-4 text-sm">
      <SectionHeader
        compact
        icon="automation"
        title="Scheduler"
        description="Approved jobs run locally and stay bounded by policy."
        className="mb-3"
      >
        <svelte:fragment slot="actions">
          <Badge variant={jobStatus?.enabled ? 'success' : 'neutral'}>
            {jobStatus?.enabled ? 'enabled' : 'disabled'}
          </Badge>
          <ActionButton
            size="xs"
            variant="secondary"
            icon="send"
            disabled={automationBusy || !jobStatus?.enabled}
            disabledReason={automationBusy ? 'Automation refresh is already running.' : 'Scheduler must be enabled before running due jobs.'}
            onclick={runDueJobs}
          >
            Run due now
          </ActionButton>
        </svelte:fragment>
      </SectionHeader>
      <div class="grid grid-cols-2 gap-y-2 text-slate-700">
        <span>Total</span><span>{jobStatus?.totalJobs ?? 0}</span>
        <span>Enabled</span><span>{jobStatus?.enabledJobs ?? 0}</span>
        <span>Approved</span><span>{jobStatus?.approvedJobs ?? 0}</span>
        <span>Archived</span><span>{jobStatus?.archivedJobs ?? 0}</span>
        <span>Invalid input</span><span>{jobStatus?.invalidInputJobs ?? 0}</span>
        <span>Max parallel</span><span>{jobStatus?.maxParallelJobs ?? 1}</span>
        <span>Last run</span><span>{jobStatus?.lastRunStatus || 'none'}</span>
        <span>Schedules</span><span>manual, interval, cron</span>
      </div>
      <div class="mt-3 text-xs text-slate-500">Enabled jobs run when this action or the CLI job loop ticks them; manual jobs use Run.</div>
    </div>
  </div>

  <div class="mb-4 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="search"
      title="Internet Service"
      description="Controlled GET/HEAD requests and configured search provider status."
      meta={`${internetStatus?.enabled ? internetStatus.defaultMode : 'disabled'} | cache ${internetStatus?.cacheEnabled ? 'on' : 'off'}`}
      className="mb-3"
    >
      <svelte:fragment slot="actions">
        <Badge variant={internetSearchReadiness(internetStatus) === 'healthy' ? 'success' : internetSearchReadiness(internetStatus) === 'disabled' ? 'neutral' : 'warning'}>
          search {internetSearchReadiness(internetStatus)}
        </Badge>
        <ActionButton variant="secondary" icon="retry" onclick={refreshInternet}>Refresh</ActionButton>
      </svelte:fragment>
    </SectionHeader>
    <div class="mb-3 grid gap-2 rounded-md border border-line bg-field px-3 py-2 text-xs text-slate-600 sm:grid-cols-2 lg:grid-cols-4">
      <div><span class="font-medium text-slate-700">Provider</span> {internetSearchProviderLabel(internetStatus)}</div>
      <div><span class="font-medium text-slate-700">Endpoint</span> <span class="break-all">{internetStatus?.searchEndpoint || 'not configured'}</span></div>
      <div><span class="font-medium text-slate-700">Results</span> {internetStatus?.searchMaxResults ?? 'default'}</div>
      <div><span class="font-medium text-slate-700">Safe search</span> {internetStatus?.searchSafeSearch ? 'on' : 'off'}</div>
      <div><span class="font-medium text-slate-700">Health</span> {internetStatus?.searchHealthScore ?? 0}/100</div>
      <div><span class="font-medium text-slate-700">Cache</span> {internetStatus?.cacheFreshEntries ?? 0} fresh / {internetStatus?.cacheStaleEntries ?? 0} stale</div>
      <div class="sm:col-span-2 lg:col-span-4"><span class="font-medium text-slate-700">Search status</span> {internetStatus?.searchStatus || 'not checked'}</div>
      <div class="sm:col-span-2 lg:col-span-4"><span class="font-medium text-slate-700">Next action</span> {internetStatus?.searchNextAction || 'none'}</div>
    </div>
    <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_220px_150px]">
      <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={internetUrl} placeholder="https://example.com" />
      <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={internetAllowedDomain} placeholder="allowed domain" />
      <div class="flex gap-2">
        <ActionButton
          variant="secondary"
          icon="search"
          disabled={internetUrlMissing}
          disabledReason="Enter a URL before sending a controlled HEAD request."
          onclick={() => internetFetch('HEAD')}
        >
          HEAD
        </ActionButton>
        <ActionButton
          variant="primary"
          icon="search"
          disabled={internetUrlMissing}
          disabledReason="Enter a URL before sending a controlled GET request."
          onclick={() => internetFetch('GET')}
        >
          GET
        </ActionButton>
      </div>
    </div>
    <div class="mt-3 flex flex-wrap gap-4 text-sm">
      <label class="flex items-center gap-2"><input type="checkbox" bind:checked={internetExtractText} /> Extract text</label>
      <label class="flex items-center gap-2"><input type="checkbox" bind:checked={internetTaskApproved} /> Task approved</label>
    </div>
    <div class="mt-4 rounded-md border border-line bg-field p-3">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-2">
        <div>
          <div class="text-sm font-semibold text-slate-800">Bounded Crawl</div>
          <div class="text-xs text-slate-500">{internetStatus?.crawlerStatus || 'Manual bounded crawler. No scheduler or automatic ingestion.'}</div>
        </div>
        <div class="flex flex-wrap gap-2">
          <Badge variant={internetStatus?.crawlerAvailable ? 'info' : 'neutral'} size="xs">
            {internetStatus?.crawlerAvailable ? 'manual only' : 'disabled'}
          </Badge>
          <Badge variant="warning" size="xs">approval required</Badge>
        </div>
      </div>
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <label class="text-xs font-medium text-slate-500">
          Max pages
          <input class="mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm" type="number" min="1" max={internetStatus?.crawlerMaxPages ?? 25} bind:value={internetCrawlMaxPages} />
        </label>
        <label class="text-xs font-medium text-slate-500">
          Max depth
          <input class="mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm" type="number" min="0" max={internetStatus?.crawlerMaxDepth ?? 3} bind:value={internetCrawlMaxDepth} />
        </label>
        <label class="text-xs font-medium text-slate-500">
          Seconds
          <input class="mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm" type="number" min="1" max={internetStatus?.crawlerMaxDurationSeconds ?? 120} bind:value={internetCrawlMaxDurationSeconds} />
        </label>
        <label class="text-xs font-medium text-slate-500">
          Links/page
          <input class="mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm" type="number" min="1" max={internetStatus?.crawlerMaxLinksPerPage ?? 200} bind:value={internetCrawlMaxLinksPerPage} />
        </label>
        <label class="text-xs font-medium text-slate-500">
          Text chars
          <input class="mt-1 h-9 w-full rounded-md border border-line bg-white px-2 text-sm" type="number" min="200" max="12000" bind:value={internetCrawlMaxTextChars} />
        </label>
      </div>
      <div class="mt-3 flex flex-wrap items-center justify-between gap-3">
        <div class="text-xs text-slate-500">
          Same-domain by default. Add comma-separated allowed domains above for a bounded cross-domain crawl.
        </div>
        <ActionButton
          variant="primary"
          icon="search"
          disabled={internetCrawlDisabled}
          disabledReason={internetCrawlDisabledReason}
          onclick={() => internetCrawl('GET')}
        >
          Crawl
        </ActionButton>
      </div>
    </div>
    {#if internetFetchOutput}
      <StructuredDataView className="mt-3" value={internetFetchOutput} title="Internet output" maxPreviewLines={16} />
    {/if}
    <div class="mt-4 grid gap-4 lg:grid-cols-3">
      <div>
        <div class="mb-2 text-xs font-medium text-slate-500">Recent Requests</div>
        <div class="max-h-44 overflow-auto rounded-md border border-line">
          {#each pagedItems(internetRequests, 'internet-requests', pageSizeFor('internet-requests'), listPages) as request}
            <div class="border-b border-line px-3 py-2 text-xs last:border-b-0 space-y-1">
              <div class="break-all font-medium">
                <Badge variant={request.method === 'GET' ? 'success' : request.method === 'POST' ? 'info' : 'danger'} size="xs" icon={request.method === 'GET' ? 'arrow-right' : request.method === 'POST' ? 'arrow-left' : 'question'}>
                  {request.method}
                </Badge> {request.url}
              </div>
              <div class="text-slate-500">{request.allowed ? 'allowed' : 'blocked'} {request.statusCode || ''} {request.error || ''}</div>
            </div>
          {/each}
          {#if (internetRequests ?? []).length === 0}
            <div class="p-3">
              <EmptyState
                icon="search"
                title="No requests logged"
                message="Internet fetch/search requests will appear here when the controlled internet service is used."
              />
            </div>
          {/if}
        </div>
        <PaginationControls
          page={currentPage('internet-requests', internetRequests.length, pageSizeFor('internet-requests'), listPages)}
          total={internetRequests.length}
          pageSize={pageSizeFor('internet-requests')}
          label="requests"
          onChange={(page) => setListPage('internet-requests', page)}
        />
      </div>
      <div>
        <div class="mb-2 text-xs font-medium text-slate-500">Recent Crawls</div>
        <div class="max-h-44 overflow-auto rounded-md border border-line">
          {#each pagedItems(internetCrawls, 'internet-crawls', pageSizeFor('internet-crawls'), listPages) as run}
            <div class="space-y-1 border-b border-line px-3 py-2 text-xs last:border-b-0">
              <div class="flex flex-wrap items-center gap-2">
                <Badge
                  variant={run.status === 'completed' ? 'success' : run.status === 'blocked' || run.status === 'cancelled' ? 'warning' : 'info'}
                  size="xs"
                >
                  {run.status}
                </Badge>
                <span class="font-medium">{run.fetched} pages</span>
                <span class="text-slate-500">{run.skipped} skipped</span>
              </div>
              <div class="break-all font-medium">{run.method} {run.seedUrl}</div>
              {#if run.failureReason}
                <div class="text-warning">{run.failureReason}</div>
              {/if}
              <div class="flex flex-wrap items-center justify-between gap-2 text-slate-500">
                <span>depth {run.maxDepth} | pages {run.maxPages} | run {run.runId}</span>
                <ActionButton variant="secondary" size="sm" icon="eye" onclick={() => inspectInternetCrawl(run.runId)}>Inspect</ActionButton>
              </div>
            </div>
          {/each}
          {#if (internetCrawls ?? []).length === 0}
            <div class="p-3">
              <EmptyState
                icon="search"
                title="No crawl runs"
                message="Manual bounded crawl runs will appear here after an approved crawl finishes or is blocked."
              />
            </div>
          {/if}
        </div>
        <PaginationControls
          page={currentPage('internet-crawls', internetCrawls.length, pageSizeFor('internet-crawls'), listPages)}
          total={internetCrawls.length}
          pageSize={pageSizeFor('internet-crawls')}
          label="crawls"
          onChange={(page) => setListPage('internet-crawls', page)}
        />
      </div>
      <div>
        <div class="mb-2 text-xs font-medium text-slate-500">Cache</div>
        <div class="max-h-44 overflow-auto rounded-md border border-line">
          {#each pagedItems(internetCache, 'internet-cache', pageSizeFor('internet-cache'), listPages) as item}
            <div class="border-b border-line px-3 py-2 text-xs last:border-b-0">
              <div class="break-all font-medium">{item.method} {item.url}</div>
              <div class="text-slate-500">{item.bodyBytes} bytes | {item.state || (item.expired ? 'stale' : 'fresh')} | age {item.ageSeconds ?? 0}s</div>
            </div>
          {/each}
          {#if (internetCache ?? []).length === 0}
            <div class="p-3">
              <EmptyState
                icon="documents"
                title="No cached responses"
                message="Cached controlled internet responses will appear here when caching is enabled and requests complete."
              />
            </div>
          {/if}
        </div>
        <PaginationControls
          page={currentPage('internet-cache', internetCache.length, pageSizeFor('internet-cache'), listPages)}
          total={internetCache.length}
          pageSize={pageSizeFor('internet-cache')}
          label="cached"
          onChange={(page) => setListPage('internet-cache', page)}
        />
      </div>
    </div>
  </div>

  <div class="mb-4 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="plus"
      title="Create Scheduled Job"
      description="Use manual, interval, cron, or one-time schedules. Jobs stay disabled until enabled."
      className="mb-3"
    />
    <div class="grid gap-3 lg:grid-cols-[150px_150px_minmax(0,1fr)_160px]">
      <BitSelect bind:value={jobScheduleType} options={jobScheduleTypeOptions} />
      <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={jobScheduleExpr} placeholder="1h or cron" disabled={jobScheduleType === 'manual'} />
      <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={jobTargetName} placeholder="extension name or heartbeat" />
      <BitSelect bind:value={jobTargetType} options={jobTargetTypeOptions} />
    </div>
    <textarea
      class={`mt-3 h-24 w-full resize-none rounded-md border bg-field p-3 text-sm ${jobInputJSONError ? 'border-amber-300' : 'border-line'}`}
      bind:value={jobInputJSON}
      placeholder="Job input JSON object"
    ></textarea>
    {#if jobInputJSONError}
      <PageStatusStrip className="mt-3" kind="warning" title="Invalid job input JSON" message={jobInputJSONError} />
    {/if}
    <div class="mt-3 flex justify-end">
      <ActionButton
        variant="primary"
        icon="plus"
        disabled={createJobDisabled}
        disabledReason={createJobDisabledReason}
        onclick={createJob}
      >
        Create
      </ActionButton>
    </div>
  </div>

  <div class="mb-4 rounded-md border border-line bg-white">
    <div class="border-b border-line px-4 py-3">
      <SectionHeader
        compact
        icon="automation"
        title="Jobs"
        description="Configured local jobs with approval and enabled state."
        meta={`${jobs.length} jobs`}
      />
    </div>
    <div class="divide-y divide-line">
      {#if (jobs ?? []).length === 0}
        <div class="p-4">
          <EmptyState
            icon="automation"
            title="No jobs registered"
            message="Approved scheduled jobs will appear here after you create them."
          />
        </div>
      {/if}
      {#each pagedItems(jobs, 'jobs-list', pageSizeFor('jobs-list'), listPages) as job}
        <div class="grid gap-3 p-4 text-sm xl:grid-cols-[minmax(0,1fr)_220px]">
          <div>
            <div class="mb-1 flex flex-wrap items-center gap-2">
              <span class="font-semibold" title={job.name || job.id}>{humanizeIdentifier(job.name || job.id, job.id)}</span>
              <Badge variant={job.enabled ? 'success' : 'neutral'}>{job.enabled ? 'enabled' : 'disabled'}</Badge>
              <Badge variant={job.approved ? 'info' : 'warning'}>{job.approved ? 'approved' : 'unapproved'}</Badge>
              <Badge variant={schedulerJobConversationId(job) ? 'info' : 'neutral'}>{schedulerJobOriginLabel(job)}</Badge>
              {#if job.inputStatus === 'invalid'}
                <Badge variant="danger">input invalid</Badge>
              {/if}
            </div>
            <div class="break-all text-xs text-slate-500">{job.id}</div>
            <div class="mt-2 text-slate-700" title={`${job.scheduleType} ${job.scheduleExpr} | ${job.targetType}:${job.targetName}`}>
              {humanizeIdentifier(job.scheduleType, 'Manual')} {job.scheduleExpr} | {humanizeIdentifier(job.targetType, 'Target')}: {humanizeIdentifier(job.targetName, job.targetName)}
            </div>
            {#if job.lastRunAt}<div class="mt-1 text-xs text-slate-500">last run {job.lastRunAt}</div>{/if}
            {#if job.inputStatus === 'invalid' && job.inputValidationError}
              <div class="mt-3">
                <PageStatusStrip
                  kind="warning"
                  title="Stored extension job input is invalid"
                  message={job.inputValidationError}
                  actionLabel="Edit input"
                  onAction={() => startEditingJobInput(job)}
                />
              </div>
            {/if}
            {#if jobFailureDetails[job.id]}
              <div class="mt-3">
                <PageStatusStrip
                  kind={schedulerJobFailureTitle(job, jobFailureDetails[job.id]).includes('not available') ? 'info' : 'warning'}
                  title={schedulerJobFailureTitle(job, jobFailureDetails[job.id])}
                  message={jobFailureDetails[job.id]}
                  actionLabel="Edit input"
                  onAction={() => startEditingJobInput(job)}
                />
              </div>
            {/if}
            {#if editingJobId === job.id}
              <div class="mt-3 rounded-md border border-line bg-field p-3">
                <div class="mb-2 text-xs font-medium uppercase tracking-[0.08em] text-slate-500">Job input JSON object</div>
                <textarea
                  class={`h-32 w-full resize-none rounded-md border bg-white p-3 text-sm ${editingJobInputError ? 'border-amber-300' : 'border-line'}`}
                  bind:value={editingJobInputJSON}
                  placeholder="JSON object"
                ></textarea>
                {#if editingJobInputError}
                  <PageStatusStrip className="mt-3" kind="warning" title="Invalid job input JSON" message={editingJobInputError} />
                {/if}
                <div class="mt-3 flex flex-wrap justify-end gap-2">
                  <ActionButton size="xs" variant="secondary" icon="close" onclick={cancelEditingJobInput}>Cancel</ActionButton>
                  <ActionButton
                    size="xs"
                    variant="primary"
                    icon="check"
                    disabled={updateJobInputDisabled}
                    disabledReason={updateJobInputDisabledReason}
                    onclick={() => saveEditingJobInput(job)}
                  >
                    Save input
                  </ActionButton>
                </div>
              </div>
            {/if}
          </div>
          <div class="flex flex-wrap content-start gap-2">
            <ActionButton size="xs" variant="secondary" icon="send" onclick={() => runJob(job.id)}>Run</ActionButton>
            <ActionButton size="xs" variant="secondary" icon="write" onclick={() => startEditingJobInput(job)}>Edit input</ActionButton>
            {#if schedulerJobConversationId(job)}
              <ActionButton size="xs" variant="secondary" icon="chat" onclick={() => openConversation(schedulerJobConversationId(job))}>Open origin</ActionButton>
            {/if}
            {#if job.enabled}
              <ActionButton size="xs" variant="secondary" icon="stop" onclick={() => setJobEnabled(job.id, false)}>Disable</ActionButton>
            {:else}
              <ActionButton
                size="xs"
                variant="secondary"
                icon="check"
                disabled={!job.approved}
                disabledReason="This job must be approved before it can be enabled."
                onclick={() => setJobEnabled(job.id, true)}
              >
                Enable
              </ActionButton>
            {/if}
            <ActionButton size="xs" variant="danger" icon="trash" onclick={() => archiveJob(job.id)}>Archive</ActionButton>
          </div>
        </div>
      {/each}
    </div>
  </div>
  <PaginationControls
    page={currentPage('jobs-list', jobs.length, pageSizeFor('jobs-list'), listPages)}
    total={jobs.length}
    pageSize={pageSizeFor('jobs-list')}
    label="jobs"
    onChange={(page) => setListPage('jobs-list', page)}
  />

  <div class="rounded-md border border-line bg-white">
    <div class="border-b border-line px-4 py-3">
      <SectionHeader
        compact
        icon="automation"
        title="Job Runs"
        description="Manual and scheduled execution history."
        meta={`${jobRuns.length} runs`}
      />
    </div>
    <div class="divide-y divide-line">
      {#if (jobRuns ?? []).length === 0}
        <div class="p-4">
          <EmptyState
            icon="automation"
            title="No job runs recorded"
            message="Manual and scheduled job run history will appear here after jobs execute."
          />
        </div>
      {/if}
      {#each pagedItems(jobRuns, 'job-runs', pageSizeFor('job-runs'), listPages) as run}
        <div class="grid gap-3 p-4 text-sm lg:grid-cols-[220px_minmax(0,1fr)]">
          <div>
            <div class="mb-1 flex flex-wrap items-center gap-2">
              <span class="font-semibold">{humanizeIdentifier(jobNamesById[run.jobId] || run.jobId, run.jobId)}</span>
              <Badge variant={jobRunStatusVariant(run.status)}>{humanizeIdentifier(run.status, 'Run')}</Badge>
              {#if schedulerJobConversationId(jobsById[run.jobId])}
                <Badge variant="info">Chat origin</Badge>
              {/if}
            </div>
            <div class="mt-1 break-all text-xs text-slate-500">{run.jobId}</div>
            <div class="mt-1 break-all text-xs text-slate-500">{run.id}</div>
            <div class="mt-1 text-xs text-slate-500">{run.startedAt || 'not started'} -> {run.finishedAt || 'not finished'}</div>
            <div class="mt-1 text-xs text-slate-500">{run.durationMs} ms</div>
          </div>
          <div class="min-w-0 space-y-3">
            {#if run.error}
              <PageStatusStrip kind="error" title={jobRunFailureTitle(run)} message={run.error} />
            {:else}
              <PageStatusStrip kind="success" title="Run summary" message={schedulerJobRunOutputSummary(run.output)} />
            {/if}
            <StructuredDataView
              value={run.error || run.output}
              title={run.error ? 'Failure detail' : 'Output'}
              language={run.error ? 'text' : 'auto'}
              maxPreviewLines={14}
            />
          </div>
        </div>
      {/each}
    </div>
  </div>
  <PaginationControls
    page={currentPage('job-runs', jobRuns.length, pageSizeFor('job-runs'), listPages)}
    total={jobRuns.length}
    pageSize={pageSizeFor('job-runs')}
    label="runs"
    onChange={(page) => setListPage('job-runs', page)}
  />
</div>
