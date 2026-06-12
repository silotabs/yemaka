<script lang="ts">
  import Icon from './Icon.svelte';
  import ActionButton from './ActionButton.svelte';
  import Badge from './Badge.svelte';
  import type { SchedulerHandoff } from './lib/appTypes';
  import { readPersistedCollapse, stableCollapseKey, writePersistedCollapse } from './lib/collapsePersistence';
  import { humanizeIdentifier, humanizeIdentifierList } from './lib/uiHelpers';
  import {
    schedulerHandoffCardClass as schedulerHandoffCardClassFor,
    schedulerHandoffDisabled as schedulerHandoffDisabledFor
  } from './lib/handoffHelpers';
  import { normalizeSchedulerSchedule, schedulerHandoffInputJSON } from './lib/schedulerJobHelpers';

  export let handoff: SchedulerHandoff;
  export let messageIndex = -1;
  export let capabilityKindLabel: (kind: string | undefined) => string = (kind) => String(kind || '');
  export let schedulerHandoffBusy = false;
  export let setSchedulerHandoffInput: (index: number, value: string) => void = () => {};
  export let createSchedulerJobFromChat: (index: number) => Promise<void> | void = () => {};
  export let openAutomation: () => void = () => {};
  export let persistenceKey = '';

  type BadgeVariant = 'success' | 'danger' | 'warning' | 'info' | 'neutral';

  let collapsed = false;
  let loadedCollapseKey = '';
  let jobInputExpanded = false;

  $: handoffCardClass = schedulerHandoffCardClassFor(handoff);
  $: handoffDisabled = schedulerHandoffDisabledFor(handoff, schedulerHandoffBusy);
  $: normalizedSchedule = normalizeSchedulerSchedule(handoff.scheduleType || 'manual', handoff.scheduleExpr);
  $: handoffInputValue = schedulerHandoffInputJSON(handoff);
  $: scheduleWasNormalized =
    String(handoff.scheduleType || '').trim().toLowerCase() === 'cron' &&
    normalizedSchedule.scheduleType === 'interval';
  $: missingSchedule = handoff.missing.some((item) => String(item || '').toLowerCase().includes('schedule'));
  $: statusLabel =
    handoff.status === 'created'
      ? 'Created disabled'
      : handoff.status === 'creating'
        ? 'Creating'
        : handoff.status === 'failed'
          ? 'Needs attention'
          : handoff.status === 'incomplete'
            ? 'Needs details'
            : 'Ready for approval';
  $: statusVariant = (handoff.status === 'created'
    ? 'success'
    : handoff.status === 'failed'
      ? 'danger'
      : handoff.status === 'incomplete'
        ? 'warning'
        : 'info') as BadgeVariant;
  $: statusIcon =
    handoff.status === 'created'
      ? 'success'
      : handoff.status === 'failed' || handoff.status === 'incomplete'
        ? 'warning'
        : 'info';
  $: lifecycleText =
    handoff.status === 'created'
      ? 'A local job record was created and remains disabled until it is enabled from Automation.'
      : handoff.status === 'creating'
        ? 'Creating an approved local scheduler job record. The job will stay disabled after creation.'
        : handoff.status === 'failed'
          ? 'Creation stopped before a job record was saved. Review the note below before retrying.'
          : handoff.status === 'incomplete'
            ? 'This proposal needs the missing target or schedule details before a disabled job can be created.'
            : 'Approval creates a disabled local job record. Nothing runs until the job is enabled later.';
  $: targetLabel = `${humanizeIdentifier(handoff.targetType, 'Missing')} / ${humanizeIdentifier(handoff.targetName, 'Missing')}`;
  $: scheduleLabel = missingSchedule
    ? 'Missing'
    : `${humanizeIdentifier(normalizedSchedule.scheduleType, 'Manual')}${normalizedSchedule.scheduleExpr ? ` ${normalizedSchedule.scheduleExpr}` : ''}`;
  $: intentLabel = handoff.intentLabel || humanizeIdentifier(handoff.intent, '');
  $: collapseKey =
    persistenceKey
      ? stableCollapseKey('scheduler-handoff', [persistenceKey])
      : stableCollapseKey('scheduler-handoff', [
          messageIndex,
          handoff.targetType,
          handoff.targetName,
          handoff.scheduleType,
          handoff.scheduleExpr
        ]);
  $: if (collapseKey !== loadedCollapseKey) {
    loadedCollapseKey = collapseKey;
    collapsed = readPersistedCollapse(collapseKey, false);
  }

  function toggleCollapsed() {
    collapsed = !collapsed;
    writePersistedCollapse(collapseKey, collapsed);
  }
</script>

<div class={`capability-handoff-card capability-${handoffCardClass}`} class:capability-collapsed={collapsed}>
  <div class="capability-handoff-head">
    <div>
      <div class="capability-handoff-eyebrow">
        <Icon name="automation" size={13} />
        Scheduler proposal
      </div>
      <div class="capability-handoff-title" title={handoff.targetName}>{humanizeIdentifier(handoff.targetName, 'Scheduler job setup')}</div>
    </div>
    <div class="capability-handoff-head-badges">
      <Badge variant={statusVariant} icon={statusIcon}>{statusLabel}</Badge>
      <Badge variant="neutral">{capabilityKindLabel(handoff.scheduleType || 'manual')}</Badge>
      <ActionButton
        variant="ghost"
        size="xs"
        icon={collapsed ? 'eye' : 'eye-off'}
        ariaLabel={collapsed ? 'Show scheduler details' : 'Hide scheduler details'}
        onclick={toggleCollapsed}
      >
        {collapsed ? 'Show' : 'Hide'}
      </ActionButton>
    </div>
  </div>
  {#if !collapsed}
  <div class="capability-handoff-section">
    <div class="capability-handoff-section-label">Lifecycle</div>
    <div class="capability-handoff-body">
      {lifecycleText}
    </div>
  </div>

  <div class="capability-handoff-section">
    <div class="capability-handoff-section-label">Job profile</div>
    <div class="capability-handoff-detail-grid">
      <div class="capability-handoff-detail">
        <span>Target</span>
        <strong title={`${handoff.targetType || 'missing'} / ${handoff.targetName || 'missing'}`}>{targetLabel}</strong>
      </div>
      <div class="capability-handoff-detail">
        <span>Schedule</span>
        <strong title={`${handoff.scheduleType || 'missing'} ${handoff.scheduleExpr || ''}`}>{scheduleLabel}</strong>
      </div>
      {#if intentLabel}
        <div class="capability-handoff-detail">
          <span>Intent</span>
          <strong title={handoff.intentSummary || intentLabel}>{intentLabel}</strong>
        </div>
      {/if}
      <div class="capability-handoff-detail">
        <span>Approval</span>
        <strong>{handoff.approved ? 'Approved' : 'Approval required'}</strong>
      </div>
      <div class="capability-handoff-detail">
        <span>Activation</span>
        <strong>{handoff.enabled ? 'Enabled' : 'Disabled by default'}</strong>
      </div>
    </div>
  </div>
  {#if handoff.missing.length}
    <div class="capability-handoff-callout capability-handoff-callout-warning">
      <div class="capability-handoff-section-label">Missing details</div>
      <div>{humanizeIdentifierList(handoff.missing, 'None')}</div>
    </div>
  {/if}
  {#if handoff.setupHint || handoff.intentSummary}
    <div class="capability-handoff-callout">
      <div class="capability-handoff-section-label">Intent guidance</div>
      <div>{handoff.setupHint || handoff.intentSummary}</div>
    </div>
  {/if}
  {#if handoff.message}
    <div class={`capability-handoff-callout ${handoff.status === 'failed' ? 'capability-handoff-callout-danger' : ''}`.trim()}>
      <div class="capability-handoff-section-label">Status note</div>
      <div>{handoff.message}</div>
    </div>
  {/if}
  {#if scheduleWasNormalized}
    <div class="capability-handoff-callout">
      <div class="capability-handoff-section-label">Schedule note</div>
      <div>Yemaka will create this as an interval job because <code>{handoff.scheduleExpr}</code> is a duration. Cron schedules use five fields, for example <code>0 8 * * *</code>.</div>
    </div>
  {/if}
  {#if handoff.status !== 'created'}
    <div class="capability-handoff-section">
      <label class="capability-option stacked">
        <span>Job input</span>
        <textarea
          class="capability-json-input"
          class:capability-json-input-expanded={jobInputExpanded}
          rows={jobInputExpanded ? 8 : 3}
          value={handoffInputValue}
          placeholder={handoffInputValue}
          onfocus={() => (jobInputExpanded = true)}
          onclick={() => (jobInputExpanded = true)}
          oninput={(event) => setSchedulerHandoffInput(messageIndex, (event.currentTarget as HTMLTextAreaElement).value)}
        ></textarea>
        <small class="capability-field-hint">Click the input to expand. Keep it as a JSON object; jobs stay disabled until enabled from Automation.</small>
      </label>
    </div>
  {/if}
  {#if handoff.setupTemplate && handoff.status === 'incomplete'}
    <div class="capability-handoff-callout">
      <div class="capability-handoff-section-label">Setup path</div>
      <div>Use Automation to review the missing fields and create the disabled job after approval.</div>
    </div>
  {/if}
  {#if handoff.cliCommand && handoff.status !== 'created'}
    <div class="capability-handoff-callout">
      <div class="capability-handoff-section-label">Fallback</div>
      <div>A CLI fallback is available for this proposal after review.</div>
    </div>
  {/if}
  {#if handoff.job}
    <div class="capability-handoff-section">
      <div class="capability-handoff-section-label">Created job</div>
      <div class="capability-handoff-meta">
        <Badge variant="success" icon="success" title={handoff.job.id}>{handoff.job.id}</Badge>
        <Badge variant={handoff.job.enabled ? 'success' : 'neutral'}>{handoff.job.enabled ? 'Enabled' : 'Disabled'}</Badge>
      </div>
    </div>
  {/if}
  <div class="capability-handoff-actions">
    <ActionButton
      variant="primary"
      size="sm"
      icon={handoff.status === 'created' ? 'check' : 'automation'}
      disabled={handoffDisabled}
      disabledReason={schedulerHandoffBusy ? 'Yemaka is creating the scheduler job.' : handoff.missing.length ? `Missing required fields: ${handoff.missing.join(', ')}` : 'This scheduler handoff cannot be created yet.'}
      busy={handoff.status === 'creating'}
      busyLabel="Creating..."
      onclick={() => createSchedulerJobFromChat(messageIndex)}
    >
      {handoff.status === 'created' ? 'Created disabled job' : 'Create disabled job'}
    </ActionButton>
    <ActionButton variant="secondary" size="sm" icon="automation" onclick={openAutomation}>
      Open Automation
    </ActionButton>
  </div>
  {/if}
</div>
