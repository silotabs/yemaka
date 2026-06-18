<script lang="ts">
  import Icon from './Icon.svelte';
  import ActionButton from './ActionButton.svelte';
  import EmptyState from './EmptyState.svelte';
  import PaginationControls from './PaginationControls.svelte';
  import SectionHeader from './SectionHeader.svelte';
  import type { ActivityEntry, PermissionItem, ToolRun } from './lib/appTypes';
  import { permissionActionDisabled as permissionActionDisabledFor } from './lib/diagnosticHelpers';
  import type { ToolRunConversationGroup } from './lib/toolRunHelpers';
  import Badge from './Badge.svelte';

  export let permissionItems: PermissionItem[] = [];
  export let permissionDecisionBusy: Record<string, boolean> = {};
  export let activity: ActivityEntry[] = [];
  export let groupedToolRuns: Array<ToolRunConversationGroup<ToolRun>> = [];
  export let expandedToolRunGroups: Record<string, boolean> = {};
  export let listPages: Record<string, number> = {};
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let permissionCommandPreview: (item: PermissionItem) => string = () => '';
  export let decidePermission: (item: PermissionItem, decision: 'approved' | 'rejected') => Promise<void> | void = () => {};
  export let activityKey: (item: ActivityEntry, index: number) => string = (item, index) => `${item.id}-${index}`;
  export let activityIcon: (kind: string) => string = () => 'chat';
  export let activityKind: (line: string) => string = () => 'system';
  export let toggleToolRunGroup: (id: string) => void = () => {};
  export let toolRunGroupExpanded: (id: string, total: number, expandedState?: Record<string, boolean>) => boolean = () => false;
  export let toolRunTimestamp: (run: ToolRun) => string | undefined = () => '';
  export let formatShortTime: (value: string | undefined) => string = (value) => String(value || '');
  export let formatToolName: (name: string | undefined) => string = (name) => String(name || '');
  export let formatToolStatus: (status: string) => string = (status) => status;

  $: pendingPermissionItems = permissionItems.filter((item) => item.status === 'pending');
</script>

<aside class="activity-panel min-h-0 border-t border-line bg-white xl:border-l xl:border-t-0">
  {#if pendingPermissionItems.length}
    <div class="border-b border-line">
      <div class="px-4 py-3">
        <SectionHeader
          compact
          icon="health"
          title="Approvals"
          description="Review exact tool actions before Yemaka continues."
          meta={`${pendingPermissionItems.length} pending`}
        />
      </div>
      <div class="space-y-2 px-3 pb-3">
        {#each pendingPermissionItems as item (item.requestId)}
          <div class={`permission-card ${item.destructive ? 'permission-card-risky' : ''}`}>
            <div class="mb-1 flex items-center justify-between gap-2">
              <span class="min-w-0 truncate font-semibold" title={item.toolName}>{formatToolName(item.toolName)}</span>
              <Badge variant={item.riskLevel === 'high' ? 'danger' : item.riskLevel === 'medium' ? 'warning' : 'neutral'} size="xs">
                {formatToolStatus(item.riskLevel)}
              </Badge>
            </div>
            <div class="mb-2 text-slate-700">{item.reason || 'Approval required before this action runs.'}</div>
            {#if permissionCommandPreview(item)}
              <div class="mb-2 rounded-md border border-line bg-field px-2 py-1.5 text-[11px]">
                <div class="mb-0.5 font-semibold uppercase tracking-[0.12em] text-slate-500">Exact action</div>
                <div class="break-words font-mono leading-4">{permissionCommandPreview(item)}</div>
              </div>
            {/if}
            <div class="mb-3 flex flex-wrap gap-1 text-[11px] text-slate-600">
              {#if item.workspaceOnly}<span class="permission-chip">workspace</span>{/if}
              {#if item.diffPreview}<span class="permission-chip">diff</span>{/if}
              {#if item.snapshotBeforeWrite}<span class="permission-chip">snapshot</span>{/if}
              {#if item.rollbackSupported}<span class="permission-chip">rollback</span>{/if}
            </div>
            <div class="flex gap-2">
              <ActionButton
                variant="secondary"
                size="xs"
                className="action-button-fill"
                disabled={permissionActionDisabledFor(item, permissionDecisionBusy)}
                disabledReason="This permission decision is already being processed."
                onclick={() => decidePermission(item, 'rejected')}
                icon="stop"
              >
                Reject
              </ActionButton>
              <ActionButton
                variant={item.destructive ? 'danger' : 'primary'}
                size="xs"
                className="action-button-fill"
                disabled={permissionActionDisabledFor(item, permissionDecisionBusy)}
                disabledReason="This permission decision is already being processed."
                busy={Boolean(permissionDecisionBusy[item.requestId])}
                busyLabel="Working"
                onclick={() => decidePermission(item, 'approved')}
                icon="check"
              >
                Approve
              </ActionButton>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}
  <div class="border-b border-line p-4">
    <SectionHeader
      compact
      icon="automation"
      title="Activity"
      description="Live steps, approvals, and local action history."
      meta={`${activity.length + groupedToolRuns.length} items`}
    />
  </div>
  <div class="max-h-full overflow-y-auto p-4">
    {#if activity.length === 0 && groupedToolRuns.length === 0}
      <EmptyState
        icon="automation"
        title="No activity yet"
        message="Live steps, approvals, and local action history will appear here while Yemaka works."
      />
    {/if}
    {#if activity.length}
      <div class="mb-3">
        <div class="activity-section-label">Live</div>
        {#each pagedItems(activity, 'activity-live', 5, listPages) as item, index (activityKey(item, index))}
          <div class={`activity-item activity-${item.kind}`}>
            <span class="activity-icon">
              <Icon name={activityIcon(item.kind)} size={14} />
            </span>
            <span class="min-w-0 flex-1">
              <span class="activity-label">{item.label}</span>
              <span class="activity-time">{item.at}</span>
            </span>
          </div>
        {/each}
        <PaginationControls
          page={currentPage('activity-live', activity.length, 5, listPages)}
          total={activity.length}
          pageSize={5}
          label="activity"
          onChange={(page) => setListPage('activity-live', page)}
        />
      </div>
    {/if}
    {#if groupedToolRuns.length}
      <div class="activity-section-label">Local action history</div>
      {#each pagedItems(groupedToolRuns, 'activity-run-groups', 5, listPages) as group (group.id)}
        <div class="activity-run-group">
          <button class="activity-run-header" type="button" onclick={() => toggleToolRunGroup(group.id)}>
            <span class="min-w-0">
              <span>{group.label}</span>
              <small>{group.runs.length} run{group.runs.length === 1 ? '' : 's'} &middot; {group.detail || 'recent'}</small>
            </span>
            <span>{toolRunGroupExpanded(group.id, groupedToolRuns.length, expandedToolRunGroups) ? 'Less' : 'More'}</span>
          </button>
          {#if toolRunGroupExpanded(group.id, groupedToolRuns.length, expandedToolRunGroups)}
            <div class="activity-run-body">
              {#each group.sessions.slice(0, 4) as session (session.id)}
                <div class="activity-run-session">
                  <div class="activity-run-session-title">{session.label}</div>
                  <div class="activity-run-session-detail">{session.detail}</div>
                  {#each session.runs.slice(0, 4) as run (run.id)}
                    <div class={`activity-item activity-${activityKind(run.toolName || run.status)}`}>
                      <span class="activity-icon"><Icon name={activityIcon(activityKind(run.toolName || run.status))} size={14} /></span>
                      <span class="min-w-0 flex-1">
                        <span class="activity-label" title={run.toolName}>{formatToolName(run.toolName)}</span>
                        <span class="activity-time">{formatToolStatus(run.status)} &middot; {formatToolStatus(run.riskLevel || 'low')} risk &middot; {formatShortTime(toolRunTimestamp(run))}</span>
                      </span>
                    </div>
                  {/each}
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/each}
      <PaginationControls
        page={currentPage('activity-run-groups', groupedToolRuns.length, 5, listPages)}
        total={groupedToolRuns.length}
        pageSize={5}
        label="groups"
        onChange={(page) => setListPage('activity-run-groups', page)}
      />
    {/if}
  </div>
</aside>
