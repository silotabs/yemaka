<script lang="ts">
  import PaginationControls from '../PaginationControls.svelte';
  import ActionButton from '../ActionButton.svelte';
  import Badge from '../Badge.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import StructuredDataView from '../StructuredDataView.svelte';
  import type { FileWritePlan, NotificationItem, ReplayTraceExplanation, ReplayTraceFile, ToolCatalogEntry, ToolRun } from '../lib/appTypes';
  import type { ToolRunConversationGroup } from '../lib/toolRunHelpers';

  export let fileWritePath = '';
  export let fileWriteContent = '';
  export let fileWritePlan: FileWritePlan | null = null;
  export let notifications: NotificationItem[] = [];
  export let notificationBusy: Record<string, boolean> = {};
  export let replayTraces: ReplayTraceFile[] = [];
  export let replayExplainBusy = false;
  export let selectedReplayTraceId = '';
  export let replayExplanation: ReplayTraceExplanation | null = null;
  export let toolCatalog: ToolCatalogEntry[] = [];
  export let toolRuns: ToolRun[] = [];
  export let groupedToolRuns: Array<ToolRunConversationGroup<ToolRun>> = [];
  export let expandedToolRunGroups: Record<string, boolean> = {};
  export let expandedToolRunSessions: Record<string, boolean> = {};
  export let listPages: Record<string, number> = {};
  export let pageSizeFor: (key: string) => number = () => 10;
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let refreshDiagnostics: () => Promise<void> | void = () => {};
  export let refreshToolRuns: () => Promise<void> | void = () => {};
  export let previewFileWrite: () => Promise<void> | void = () => {};
  export let applyFileWrite: () => Promise<void> | void = () => {};
  export let markNotificationRead: (id: string) => Promise<void> | void = () => {};
  export let dismissNotification: (id: string) => Promise<void> | void = () => {};
  export let explainReplayTrace: (id: string) => Promise<void> | void = () => {};
  export let clearReplayExplanation: () => void = () => {};
  export let shortId: (value: string | undefined) => string = (value) => String(value || '');
  export let replayRouteSummary: (explanation: ReplayTraceExplanation | null) => string = () => '';
  export let replayToolSummary: (explanation: ReplayTraceExplanation | null) => string = () => '';
  export let formatSurfaces: (surfaces: string[] | undefined) => string = (surfaces) => (surfaces ?? []).join(', ');
  export let formatToolName: (name: string | undefined) => string = (name) => String(name || '');
  export let formatToolStatus: (status: string) => string = (status) => status;
  export let toggleToolRunGroup: (id: string) => void = () => {};
  export let toolRunGroupExpanded: (id: string, total: number, expandedState?: Record<string, boolean>) => boolean = () => false;
  export let toggleToolRunSession: (id: string) => void = () => {};
  export let toolRunSessionExpanded: (id: string, total: number, expandedState?: Record<string, boolean>) => boolean = () => false;
  export let toolRunTimestamp: (run: ToolRun) => string = (run) => run.completedAt || run.createdAt;
  export let formatShortTime: (value: string | undefined) => string = (value) => String(value || '');
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  {#if replayExplainBusy}
    <div class="mb-4">
      <PageStatusStrip kind="info" title="Diagnostics running" message="Yemaka is inspecting the selected replay trace." />
    </div>
  {/if}
  <div class="mb-5 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="write"
      title="File Write"
      description="Preview a diff before applying a local file write."
      className="mb-3"
    />
    <div class="mb-3 grid gap-3 lg:grid-cols-[260px_minmax(0,1fr)]">
      <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={fileWritePath} placeholder="path/to/file.md" />
      <div class="flex gap-2">
        <ActionButton
          variant="secondary"
          icon="eye"
          disabled={!fileWritePath.trim()}
          disabledReason="Enter a target file path before previewing a diff."
          onclick={previewFileWrite}
        >
          Preview Diff
        </ActionButton>
        <ActionButton
          variant="danger"
          icon="check"
          disabled={!fileWritePlan?.changed}
          disabledReason="Preview a changed diff before applying a file write."
          onclick={applyFileWrite}
        >
          Apply
        </ActionButton>
      </div>
    </div>
    <textarea class="mb-3 h-28 w-full resize-none rounded-md border border-line bg-field p-3 text-sm" bind:value={fileWriteContent} placeholder="File content"></textarea>
    {#if fileWritePlan}
      <pre class="max-h-72 overflow-auto rounded-md border border-line bg-field p-3 text-xs whitespace-pre-wrap">{fileWritePlan.diff}</pre>
    {/if}
  </div>

  <div class="mb-5 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="info"
      title="Notification Inbox"
      description="Job failures, heartbeat transitions, and local system notices."
      meta={`${notifications.length} active notifications`}
      className="mb-3"
    >
      <svelte:fragment slot="actions">
        <ActionButton variant="secondary" icon="retry" onclick={refreshDiagnostics}>Refresh</ActionButton>
      </svelte:fragment>
    </SectionHeader>
    <div class="divide-y divide-line text-sm">
      {#if notifications.length === 0}
        <div class="p-4">
          <EmptyState
            icon="info"
            title="No active notifications"
            message="Job failures, heartbeat transitions, and important local system notices will appear here."
          />
        </div>
      {/if}
      {#each pagedItems(notifications, 'notifications-inbox', pageSizeFor('notifications-inbox'), listPages) as item (item.id)}
        <div class="grid gap-3 p-4 lg:grid-cols-[minmax(0,1fr)_auto]">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <span class="font-semibold">{item.title}</span>
              <Badge variant={item.severity === 'error' ? 'danger' : item.severity === 'warning' ? 'warning' : 'info'}>{item.severity || 'info'}</Badge>
              {#if item.actionRequired}
                <Badge variant="warning">action required</Badge>
              {/if}
            </div>
            {#if item.message}
              <div class="mt-1 text-xs text-slate-600">{item.message}</div>
            {/if}
            <div class="mt-1 text-xs text-slate-500">{item.source || item.type || 'system'} | {formatShortTime(item.createdAt)}</div>
          </div>
          <div class="flex flex-wrap items-start gap-2">
            <ActionButton
              size="xs"
              variant="secondary"
              icon="check"
              disabled={item.read || notificationBusy[item.id]}
              disabledReason={item.read ? 'This notification is already marked read.' : 'This notification action is already running.'}
              onclick={() => markNotificationRead(item.id)}
            >
              Mark read
            </ActionButton>
            <ActionButton
              size="xs"
              variant="secondary"
              icon="close"
              disabled={notificationBusy[item.id]}
              disabledReason="This notification action is already running."
              onclick={() => dismissNotification(item.id)}
            >
              Dismiss
            </ActionButton>
          </div>
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('notifications-inbox', notifications.length, pageSizeFor('notifications-inbox'), listPages)}
      total={notifications.length}
      pageSize={pageSizeFor('notifications-inbox')}
      label="notifications"
      onChange={(page) => setListPage('notifications-inbox', page)}
    />
  </div>

  <div class="mb-5 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="learning"
      title="Replay Diagnostics"
      description="Inspect recent route and executor traces."
      meta={`${replayTraces.length} recent traces`}
      className="mb-3"
    >
      <svelte:fragment slot="actions">
        <ActionButton variant="secondary" icon="retry" onclick={refreshDiagnostics}>Refresh</ActionButton>
      </svelte:fragment>
    </SectionHeader>
    <div class="divide-y divide-line text-sm">
      {#if replayTraces.length === 0}
        <div class="p-4">
          <EmptyState
            icon="learning"
            title="No replay traces recorded yet"
            message="Recent route and executor traces will appear here after chat or tool activity."
          />
        </div>
      {/if}
      {#each pagedItems(replayTraces, 'replay-traces', pageSizeFor('replay-traces'), listPages) as trace (trace.id)}
        <div class="p-4">
          <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto]">
            <div class="min-w-0">
              <div class="font-semibold">{shortId(trace.id) || trace.filename}</div>
              <div class="mt-1 break-all text-xs text-slate-500">{trace.filename}</div>
            </div>
            <ActionButton
              size="xs"
              variant="secondary"
              icon="learning"
              disabled={replayExplainBusy && selectedReplayTraceId === trace.id}
              disabledReason="Yemaka is already explaining this replay trace."
              onclick={() => explainReplayTrace(trace.id)}
            >
              Explain
            </ActionButton>
          </div>

          {#if replayExplanation && replayExplanation.id === trace.id}
            <div class="mt-3 rounded-md border border-line bg-field p-4 text-sm">
              <div class="mb-3 flex flex-wrap items-start justify-between gap-3">
                <div class="min-w-0">
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="font-semibold">Trace {shortId(replayExplanation.id)}</span>
                    <Badge>{replayExplanation.verification?.status || 'diagnostic'}</Badge>
                  </div>
                  {#if replayExplanation.requestSummary}
                    <div class="mt-2 text-xs text-slate-600">{replayExplanation.requestSummary}</div>
                  {/if}
                </div>
                <ActionButton size="xs" variant="secondary" icon="close" onclick={clearReplayExplanation}>Close</ActionButton>
              </div>
              <div class="mb-2 text-xs text-slate-500">{replayRouteSummary(replayExplanation)}</div>
              <div class="mb-3 text-xs text-slate-500">Tools: {replayToolSummary(replayExplanation)}</div>
              {#if replayExplanation.diagnosticWarnings?.length}
                <ul class="mb-3 list-disc space-y-1 pl-5 text-xs text-amber-700">
                  {#each replayExplanation.diagnosticWarnings as warning}
                    <li>{warning}</li>
                  {/each}
                </ul>
              {/if}
              <div class="grid gap-3 md:grid-cols-2">
                <details class="tool-run-json">
                  <summary>Route</summary>
                  <StructuredDataView value={replayExplanation.route} title="Route" maxPreviewLines={14} />
                </details>
                <details class="tool-run-json">
                  <summary>Context</summary>
                  <StructuredDataView value={replayExplanation.context} title="Context" maxPreviewLines={14} />
                </details>
                <details class="tool-run-json">
                  <summary>Permissions</summary>
                  <StructuredDataView value={replayExplanation.permissions} title="Permissions" maxPreviewLines={14} />
                </details>
                <details class="tool-run-json">
                  <summary>Result</summary>
                  <StructuredDataView value={replayExplanation.result} title="Result" maxPreviewLines={14} />
                </details>
              </div>
            </div>
          {/if}
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('replay-traces', replayTraces.length, pageSizeFor('replay-traces'), listPages)}
      total={replayTraces.length}
      pageSize={pageSizeFor('replay-traces')}
      label="traces"
      onChange={(page) => setListPage('replay-traces', page)}
    />
  </div>

  <div class="mb-5 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="tools"
      title="Capability Truth"
      description="Typed tools, policy state, and model-callable status."
      meta={`${toolCatalog.length} registry entries`}
      className="mb-3"
    >
      <svelte:fragment slot="actions">
        <ActionButton variant="secondary" icon="retry" onclick={refreshToolRuns}>Refresh</ActionButton>
      </svelte:fragment>
    </SectionHeader>
    <div class="divide-y divide-line text-sm">
      {#if toolCatalog.length === 0}
        <div class="p-4">
          <EmptyState
            icon="tools"
            title="No tool catalog entries"
            message="Typed tools and their policy status will appear here after diagnostics load."
          />
        </div>
      {/if}
      {#each pagedItems(toolCatalog, 'tool-catalog', pageSizeFor('tool-catalog'), listPages) as tool (tool.name)}
        <div class="grid gap-3 p-4 lg:grid-cols-[190px_170px_minmax(0,1fr)]">
          <div>
            <div class="font-semibold" title={tool.name}>{formatToolName(tool.name)}</div>
            <div class="mt-1 text-xs text-slate-500">
              <Badge variant='neutral'>{formatSurfaces(tool.surfaces)}</Badge>
            </div>
          </div>
          <div>
            <Badge variant={tool.status === 'available' ? 'success' : tool.status === 'approval-required' ? 'warning' : 'info'}>{formatToolStatus(tool.status)}</Badge>
          </div>  
          <div class="text-xs text-slate-500">
            {tool.modelCallable ? 'model-callable' : 'not model-callable'} | {tool.mutating ? 'mutating' : 'read-only'}{tool.notes ? ` | ${tool.notes}` : ''}
          </div>
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('tool-catalog', toolCatalog.length, pageSizeFor('tool-catalog'), listPages)}
      total={toolCatalog.length}
      pageSize={pageSizeFor('tool-catalog')}
      label="tools"
      onChange={(page) => setListPage('tool-catalog', page)}
    />
  </div>

  <SectionHeader
    compact
    icon="tools"
    title="Recent Tool Runs"
    description="Grouped tool calls with inputs, outputs, status, and risk."
    meta={`${toolRuns.length} runs`}
    className="mb-4"
  >
    <svelte:fragment slot="actions">
      <ActionButton variant="secondary" icon="retry" onclick={refreshToolRuns}>Refresh Logs</ActionButton>
    </svelte:fragment>
  </SectionHeader>
  {#if (toolRuns ?? []).length === 0}
    <EmptyState
      icon="tools"
      title="No tool runs logged yet"
      message="Tool calls will appear here with grouped inputs, outputs, status, and risk once Yemaka uses a tool."
    />
  {:else}
    <div class="rounded-md border border-line bg-white p-4">
      {#each pagedItems(groupedToolRuns, 'tool-run-groups', pageSizeFor('tool-run-groups'), listPages) as group (group.id)}
        <div class="tool-run-group hover:bg-slate-50">
          <button class="tool-run-group-header" type="button" onclick={() => toggleToolRunGroup(group.id)}>
            <span class="min-w-0">
              <span>{group.label}</span>
              <small>{group.runs.length} run{group.runs.length === 1 ? '' : 's'} | {group.detail || 'recent'}</small>
            </span>
            <span class="tool-run-toggle">{toolRunGroupExpanded(group.id, groupedToolRuns.length, expandedToolRunGroups) ? 'Collapse' : 'Expand'}</span>
          </button>
          {#if toolRunGroupExpanded(group.id, groupedToolRuns.length, expandedToolRunGroups)}
            <div class="tool-run-group-body">
              {#each group.sessions as session (session.id)}
                <div class="tool-run-session">
                  <button class="tool-run-session-header" type="button" onclick={() => toggleToolRunSession(session.id)}>
                    <span class="min-w-0">
                      <span>{session.label}</span>
                      <small>{session.detail || 'No prompt metadata'} | {session.runs.length} run{session.runs.length === 1 ? '' : 's'}</small>
                    </span>
                    <span class="tool-run-toggle">{toolRunSessionExpanded(session.id, group.sessions.length, expandedToolRunSessions) ? 'Less' : 'More'}</span>
                  </button>
                  {#if toolRunSessionExpanded(session.id, group.sessions.length, expandedToolRunSessions)}
                    <div class="divide-y divide-line">
                      {#each session.runs as run (run.id)}
                        <div class="grid gap-3 p-4 text-sm lg:grid-cols-[220px_minmax(0,1fr)]">
                          <div>
                            <div class="font-semibold" title={run.toolName}>{formatToolName(run.toolName)}</div>
                            <div class="mt-1 text-xs text-slate-500">{formatToolStatus(run.status)} | {formatToolStatus(run.riskLevel || 'low')} risk</div>
                            <div class="mt-1 break-all text-xs text-slate-500">{formatShortTime(toolRunTimestamp(run)) || run.createdAt}</div>
                          </div>
                          <div class="grid gap-3 md:grid-cols-2">
                            <details class="tool-run-json">
                              <summary>Input</summary>
                              <StructuredDataView value={run.input} title="Input" maxPreviewLines={14} />
                            </details>
                            <details class="tool-run-json">
                              <summary>Output</summary>
                              <StructuredDataView value={run.output} title="Output" maxPreviewLines={14} />
                            </details>
                          </div>
                        </div>
                      {/each}
                    </div>
                  {/if}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('tool-run-groups', groupedToolRuns.length, pageSizeFor('tool-run-groups'), listPages)}
      total={groupedToolRuns.length}
      pageSize={pageSizeFor('tool-run-groups')}
      label="groups"
      onChange={(page) => setListPage('tool-run-groups', page)}
    />
  {/if}
</div>
