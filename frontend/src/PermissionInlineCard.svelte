<script lang="ts">
  import ActionButton from './ActionButton.svelte';
  import Badge from './Badge.svelte';
  import Icon from './Icon.svelte';
  import type { PermissionItem } from './lib/appTypes';
  import { readPersistedCollapse, stableCollapseKey, writePersistedCollapse } from './lib/collapsePersistence';
  import { permissionActionDisabled as permissionActionDisabledFor } from './lib/diagnosticHelpers';

  type BadgeVariant = 'neutral' | 'success' | 'warning' | 'danger' | 'info';

  export let item: PermissionItem;
  export let permissionDecisionBusy: Record<string, boolean> = {};
  export let permissionCommandPreview: (item: PermissionItem) => string = () => '';
  export let decidePermission: (item: PermissionItem, decision: 'approved' | 'rejected') => Promise<void> | void = () => {};
  export let formatToolName: (name: string | undefined) => string = (name) => String(name || '');
  export let formatToolStatus: (status: string) => string = (status) => status;
  export let persistenceKey = '';

  let collapsed = false;
  let loadedCollapseKey = '';

  $: commandPreview = permissionCommandPreview(item);
  $: busy = Boolean(permissionDecisionBusy[item.requestId]);
  $: disabled = permissionActionDisabledFor(item, permissionDecisionBusy);
  $: riskVariant = (item.riskLevel === 'high' ? 'danger' : item.riskLevel === 'medium' ? 'warning' : 'neutral') as BadgeVariant;
  $: statusLabel = item.status === 'approved' ? 'Approved' : item.status === 'rejected' ? 'Rejected' : 'Approval required';
  $: statusVariant = (item.status === 'approved' ? 'success' : item.status === 'rejected' ? 'danger' : 'info') as BadgeVariant;
  $: statusIcon = item.status === 'approved' ? 'success' : item.status === 'rejected' ? 'stop' : 'info';
  $: collapseKey =
    persistenceKey
      ? stableCollapseKey('permission-inline', [persistenceKey])
      : stableCollapseKey('permission-inline', [item.conversationId, item.assistantMessageId, item.requestId, item.toolName]);
  $: if (collapseKey !== loadedCollapseKey) {
    loadedCollapseKey = collapseKey;
    collapsed = readPersistedCollapse(collapseKey, false);
  }

  function toggleCollapsed() {
    collapsed = !collapsed;
    writePersistedCollapse(collapseKey, collapsed);
  }
</script>

<div class={`permission-inline-card ${item.destructive ? 'permission-inline-card-risky' : ''}`} class:permission-inline-card-collapsed={collapsed}>
  <div class="permission-inline-card-header">
    <span class="permission-inline-icon"><Icon name="health" size={15} /></span>
    <div class="min-w-0 flex-1">
      <div class="permission-inline-eyebrow">{statusLabel}</div>
      <div class="permission-inline-title">{formatToolName(item.toolName)}</div>
    </div>
    <div class="permission-inline-card-header-actions">
      <Badge variant={statusVariant} icon={statusIcon} size="xs">{statusLabel}</Badge>
      <Badge variant={riskVariant} size="xs">{formatToolStatus(item.riskLevel || 'medium')}</Badge>
      <ActionButton
        variant="ghost"
        size="xs"
        icon={collapsed ? 'eye' : 'eye-off'}
        ariaLabel={collapsed ? 'Show approval details' : 'Hide approval details'}
        onclick={toggleCollapsed}
      >
        {collapsed ? 'Show' : 'Hide'}
      </ActionButton>
    </div>
  </div>
  {#if !collapsed}
    <div class="permission-inline-reason">{item.reason || 'Review the exact action before Yemaka continues.'}</div>
    {#if commandPreview}
      <div class="permission-inline-command">
        <span>Exact action</span>
        <code>{commandPreview}</code>
      </div>
    {/if}
    <div class="permission-inline-chips">
      {#if item.workspaceOnly}<span>workspace</span>{/if}
      {#if item.diffPreview}<span>diff</span>{/if}
      {#if item.snapshotBeforeWrite}<span>snapshot</span>{/if}
      {#if item.rollbackSupported}<span>rollback</span>{/if}
    </div>
    {#if item.status === 'pending'}
      <div class="permission-inline-actions">
        <ActionButton
          variant="secondary"
          size="sm"
          disabled={disabled}
          disabledReason="This permission decision is already being processed."
          onclick={() => decidePermission(item, 'rejected')}
          icon="stop"
        >
          Reject
        </ActionButton>
        <ActionButton
          variant={item.destructive ? 'danger' : 'primary'}
          size="sm"
          disabled={disabled}
          disabledReason="This permission decision is already being processed."
          {busy}
          busyLabel="Working"
          onclick={() => decidePermission(item, 'approved')}
          icon="check"
        >
          Approve
        </ActionButton>
      </div>
    {:else}
      <div class="permission-inline-settled">
        <Badge variant={statusVariant} icon={statusIcon}>{item.status === 'approved' ? 'Approved and recorded' : 'Rejected and recorded'}</Badge>
        <span>{item.status === 'approved' ? 'Yemaka can continue only with this approved action.' : 'Yemaka will not run this action.'}</span>
      </div>
    {/if}
  {/if}
</div>
