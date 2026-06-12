<script lang="ts">
  import BitSelect from './BitSelect.svelte';
  import ActionButton from './ActionButton.svelte';
  import Badge from './Badge.svelte';
  import Icon from './Icon.svelte';
  import type { CapabilityHandoff, DomainPack, Extension } from './lib/appTypes';
  import {
    capabilityApproveDisabled as capabilityApproveDisabledFor,
    capabilityApproveLabel as capabilityApproveLabelFor,
    capabilityExtensionName as capabilityExtensionNameFor,
    existingExtensionForCapability as existingExtensionForCapabilityFor
  } from './lib/handoffHelpers';
  import {
    domainPackForHandoff as domainPackForHandoffFor,
    domainPackSetupLabel as domainPackSetupLabelFor,
    isDomainPackHandoff as isDomainPackHandoffFor
  } from './lib/domainPackHelpers';
  import { readPersistedCollapse, stableCollapseKey, writePersistedCollapse } from './lib/collapsePersistence';
  import { humanizeIdentifier, humanizeIdentifierList, humanizePermissionList } from './lib/uiHelpers';

  type Option = {
    value: string;
    label: string;
  };

  export let handoff: CapabilityHandoff;
  export let messageIndex = -1;
  export let jobScheduleTypeOptions: Option[] = [];
  export let extensions: Extension[] = [];
  export let domainPacks: DomainPack[] = [];
  export let capabilityHandoffBusy = false;
  export let capabilityKindLabel: (kind: string | undefined) => string = (kind) => String(kind || '');
  export let setCapabilityRunAfterGenerate: (index: number, enabled: boolean) => void = () => {};
  export let setCapabilityRunInput: (index: number, value: string) => void = () => {};
  export let setCapabilityScheduleAfterGenerate: (index: number, enabled: boolean) => void = () => {};
  export let setCapabilityScheduleType: (index: number, value: string) => void = () => {};
  export let setCapabilityScheduleExpr: (index: number, value: string) => void = () => {};
  export let setCapabilityScheduleEnabled: (index: number, enabled: boolean) => void = () => {};
  export let setCapabilityScheduleInput: (index: number, value: string) => void = () => {};
  export let openMessageDomainPacks: (index: number) => Promise<void> | void = () => {};
  export let openMessageCapabilityInExtensions: (index: number) => void = () => {};
  export let approveChatCapability: (index: number) => Promise<void> | void = () => {};
  export let reuseExtension: (name: string) => Promise<void> | void = () => {};
  export let persistenceKey = '';

  let collapsed = false;
  let loadedCollapseKey = '';

  $: isDomainPack = isDomainPackHandoffFor(handoff);
  $: existingExtension = existingExtensionForCapabilityFor(extensions, handoff);
  $: extensionName = capabilityExtensionNameFor(handoff);
  $: approveDisabled = capabilityApproveDisabledFor(handoff, capabilityHandoffBusy, existingExtension);
  $: approveLabel = capabilityApproveLabelFor(handoff, existingExtension);
  $: approveDisabledReason = capabilityHandoffBusy
    ? 'Yemaka is already processing this capability request.'
    : existingExtension
      ? 'An existing extension is available; reuse it instead of generating another copy.'
      : 'This capability cannot be generated yet.';
  $: domainPackSetupLabel = domainPackSetupLabelFor(handoff, domainPackForHandoffFor(domainPacks, handoff));
  $: statusLabel =
    handoff.status === 'generated'
      ? 'Generated'
      : handoff.status === 'generating'
        ? 'Generating'
        : handoff.status === 'failed'
          ? 'Needs attention'
          : handoff.status === 'not_available'
            ? 'Setup required'
            : existingExtension
              ? 'Existing capability'
              : 'Approval required';
  $: statusVariant =
    handoff.status === 'generated' || existingExtension
      ? 'success'
      : handoff.status === 'failed'
        ? 'danger'
        : handoff.status === 'not_available'
          ? 'warning'
          : 'info';
  $: statusIcon =
    handoff.status === 'generated' || existingExtension
      ? 'success'
      : handoff.status === 'failed' || handoff.status === 'not_available'
        ? 'warning'
        : 'info';
  $: lifecycleText = existingExtension
    ? 'An existing extension can handle this request. Reuse it instead of generating a duplicate capability.'
    : handoff.status === 'generated'
      ? 'The generated extension is available for reuse and remains governed by core policy.'
      : handoff.status === 'generating'
        ? 'Yemaka is generating and validating the capability before it can be registered.'
        : handoff.status === 'failed'
          ? 'Generation stopped before registration. Review the status note before retrying.'
          : handoff.status === 'not_available'
            ? 'Supporting setup is required before this capability can be used.'
            : 'Approval is required before Yemaka generates, tests, and registers this capability.';
  $: collapseKey =
    persistenceKey
      ? stableCollapseKey('capability-handoff', [persistenceKey])
      : stableCollapseKey('capability-handoff', [
          messageIndex,
          handoff.kind,
          handoff.name || handoff.generatedName || handoff.existingCapability,
          handoff.title
        ]);
  $: shortSafetyItems = handoff.permissions.slice(0, 5);
  $: runtimeStatus = handoff.runtimeStatus;
  $: hasRuntimeState = Boolean(
    handoff.existingCapability ||
      handoff.capabilityState ||
      runtimeStatus ||
      handoff.networkMode ||
      handoff.allowedDomains.length
  );
  $: coverageText = handoff.description || handoff.reason || handoff.capabilitySummary || 'Yemaka found a capability gap for this request.';
  $: if (collapseKey !== loadedCollapseKey) {
    loadedCollapseKey = collapseKey;
    collapsed = readPersistedCollapse(collapseKey, false);
  }

  function toggleCollapsed() {
    collapsed = !collapsed;
    writePersistedCollapse(collapseKey, collapsed);
  }

  function capabilityNextStepLabel(step: string) {
    const clean = String(step || '').replace(/\s+/g, ' ').trim();
    const lower = clean.toLowerCase();
    if (!clean) return '';
    if (lower.includes('yemaka ') || lower.startsWith('cli:')) {
      if (lower.includes('job') || lower.includes('scheduler')) {
        return 'Review the generated capability, then create a scheduler job only after approval.';
      }
      if (lower.includes('generate')) {
        return 'Generate this capability only after approval and validation.';
      }
      return 'Review this capability in Extensions before running it.';
    }
    if (lower.includes('--yes') || lower.includes('extension generate')) {
      return 'Approve generation only after reviewing the requested capability.';
    }
    return clean;
  }
</script>

<div class={`capability-handoff-card capability-${handoff.status}`} class:capability-collapsed={collapsed}>
  <div class="capability-handoff-head">
    <div>
      <div class="capability-handoff-eyebrow">
        <Icon name="extensions" size={13} />
        Missing capability
      </div>
      <div class="capability-handoff-title" title={handoff.name}>{handoff.title || humanizeIdentifier(handoff.name, handoff.name)}</div>
    </div>
    <div class="capability-handoff-head-badges">
      <Badge variant={statusVariant} icon={statusIcon}>
        {statusLabel}
      </Badge>
      <Badge variant="neutral">{capabilityKindLabel(handoff.kind)}</Badge>
      <ActionButton
        variant="ghost"
        size="xs"
        icon={collapsed ? 'eye' : 'eye-off'}
        ariaLabel={collapsed ? 'Show capability details' : 'Hide capability details'}
        onclick={toggleCollapsed}
      >
        {collapsed ? 'Show' : 'Hide'}
      </ActionButton>
    </div>
  </div>

  {#if !collapsed}
  <div class="capability-handoff-body-block">
    <div class="capability-handoff-section-label">What it covers</div>
    <div class="capability-handoff-body">
      {coverageText}
    </div>
  </div>

  <div class="capability-handoff-section">
    <div class="capability-handoff-section-label">Lifecycle</div>
    <div class="capability-handoff-body">
      {lifecycleText}
    </div>
  </div>

  {#if isDomainPack}
    <div class="capability-handoff-callout">
      <div class="capability-handoff-section-label">Setup path</div>
      <div>{handoff.configureHint || handoff.suggestedAction || handoff.capabilitySummary || 'Domain packs stay local and require explicit install or enablement before use.'}</div>
    </div>
  {/if}

  {#if hasRuntimeState}
    <div class="capability-handoff-section">
      <div class="capability-handoff-section-label">Runtime state</div>
      <div class="capability-handoff-meta">
        {#if handoff.existingCapability}
          <Badge variant="success" icon="success" title={handoff.existingCapability}>
            {humanizeIdentifier(handoff.existingCapability, handoff.existingCapability)}
          </Badge>
        {/if}
        {#if handoff.capabilityState}
          <Badge variant="neutral">State: {capabilityKindLabel(handoff.capabilityState)}</Badge>
        {/if}
        {#if runtimeStatus}
          <Badge variant={runtimeStatus.installed ? 'success' : 'warning'}>{runtimeStatus.installed ? 'Installed' : 'Not installed'}</Badge>
          <Badge variant={runtimeStatus.enabled ? 'success' : 'warning'}>{runtimeStatus.enabled ? 'Enabled' : 'Disabled'}</Badge>
          <Badge variant={runtimeStatus.valid ? 'success' : 'danger'}>{runtimeStatus.valid ? 'Valid' : 'Invalid'}</Badge>
        {/if}
        {#if handoff.networkMode}
          <Badge variant="info" title={handoff.networkMode}>Network: {humanizeIdentifier(handoff.networkMode, handoff.networkMode)}</Badge>
        {/if}
        {#if handoff.allowedDomains.length}
          <Badge variant="neutral" title={handoff.allowedDomains.join(', ')}>Domains: {handoff.allowedDomains.slice(0, 3).join(', ')}</Badge>
        {/if}
      </div>
    </div>
  {/if}

  {#if shortSafetyItems.length}
    <div class="capability-handoff-safety" title={handoff.permissions.join(', ')}>
      <div class="capability-handoff-section-label">Safety defaults</div>
      <div class="capability-handoff-safety-list">
        {#each shortSafetyItems as permission}
          <Badge variant="neutral">{humanizePermissionList([permission], permission)}</Badge>
        {/each}
      </div>
    </div>
  {/if}

  {#if isDomainPack && handoff.installCommand}
    <div class="capability-handoff-callout">
      <div class="capability-handoff-section-label">Install review</div>
      <div>Open Domain Packs to install or enable this pack with approval.</div>
    </div>
  {/if}
  {#if handoff.runtimeStatus?.riskyTools?.length}
    <div class="capability-handoff-callout capability-handoff-callout-warning">
      <div class="capability-handoff-section-label">Approval-gated tools</div>
      <div>{humanizeIdentifierList(handoff.runtimeStatus.riskyTools, 'None')}</div>
    </div>
  {/if}
  {#if handoff.message}
    <div class={`capability-handoff-callout ${handoff.status === 'failed' ? 'capability-handoff-callout-danger' : ''}`.trim()}>
      <div class="capability-handoff-section-label">Status note</div>
      <div>{handoff.message}</div>
    </div>
  {/if}
  {#if handoff.status === 'pending' && !isDomainPack}
    <div class="capability-handoff-options">
      <div class="capability-handoff-section-label">Approval options</div>
      <label class="capability-option">
        <input
          type="checkbox"
          checked={Boolean(handoff.runAfterGenerate)}
          onchange={(event) => setCapabilityRunAfterGenerate(messageIndex, (event.currentTarget as HTMLInputElement).checked)}
        />
        <span>Run once after generation</span>
      </label>
      {#if handoff.runAfterGenerate}
        <textarea
          class="capability-json-input"
          rows="2"
          value={handoff.runInputJSON || '{}'}
          placeholder="JSON object for first run"
          oninput={(event) => setCapabilityRunInput(messageIndex, (event.currentTarget as HTMLTextAreaElement).value)}
        ></textarea>
      {/if}
      <label class="capability-option">
        <input
          type="checkbox"
          checked={Boolean(handoff.scheduleAfterGenerate)}
          onchange={(event) => setCapabilityScheduleAfterGenerate(messageIndex, (event.currentTarget as HTMLInputElement).checked)}
        />
        <span>Create scheduler job</span>
      </label>
      {#if handoff.scheduleAfterGenerate}
        <div class="capability-schedule-grid">
          <BitSelect
            value={handoff.scheduleType || 'manual'}
            options={jobScheduleTypeOptions}
            onChange={(value) => setCapabilityScheduleType(messageIndex, value)}
          />
          <input
            class="capability-small-input"
            value={handoff.scheduleExpr || ''}
            placeholder="1h, cron, or RFC3339"
            disabled={(handoff.scheduleType || 'manual') === 'manual'}
            oninput={(event) => setCapabilityScheduleExpr(messageIndex, (event.currentTarget as HTMLInputElement).value)}
          />
          <label class="capability-option compact">
            <input
              type="checkbox"
              checked={Boolean(handoff.scheduleEnabled)}
              onchange={(event) => setCapabilityScheduleEnabled(messageIndex, (event.currentTarget as HTMLInputElement).checked)}
            />
            <span>Enable immediately</span>
          </label>
        </div>
        <textarea
          class="capability-json-input"
          rows="2"
          value={handoff.scheduleInputJSON || '{}'}
          placeholder="JSON object for scheduled input"
          oninput={(event) => setCapabilityScheduleInput(messageIndex, (event.currentTarget as HTMLTextAreaElement).value)}
        ></textarea>
      {/if}
    </div>
  {/if}
  {#if handoff.run || handoff.schedule}
    <div class="capability-handoff-section">
      <div class="capability-handoff-section-label">Result</div>
      <div class="capability-handoff-meta">
        {#if handoff.run}
          <Badge variant={handoff.run.error ? 'danger' : 'success'} icon={handoff.run.error ? 'warning' : 'success'}>
            Run {humanizeIdentifier(handoff.run.status, 'Ready')}
          </Badge>
        {/if}
        {#if handoff.schedule}
          <Badge variant="success" icon="success" title={handoff.schedule.id}>{handoff.schedule.id}</Badge>
          <Badge variant={handoff.schedule.enabled ? 'success' : 'neutral'}>{handoff.schedule.enabled ? 'Enabled' : 'Disabled'}</Badge>
        {/if}
      </div>
      {#if handoff.run?.error}
        <div class="capability-handoff-callout capability-handoff-callout-danger">
          <div class="capability-handoff-section-label">Run note</div>
          <div>{handoff.run.error}</div>
        </div>
      {/if}
    </div>
  {/if}
  {#if handoff.nextSteps?.length}
    <div class="capability-handoff-section">
      <div class="capability-handoff-section-label">Next steps</div>
      <div class="capability-handoff-next">
        {#each (handoff.nextSteps ?? []).slice(0, 4) as step}
          <span>{capabilityNextStepLabel(step)}</span>
        {/each}
      </div>
    </div>
  {/if}
  <div class="capability-handoff-actions">
    {#if isDomainPack}
      <ActionButton variant="primary" size="sm" icon="skills" onclick={() => openMessageDomainPacks(messageIndex)}>
        {domainPackSetupLabel}
      </ActionButton>
    {:else if handoff.generatedName || existingExtension}
      <ActionButton variant="primary" size="sm" icon="send" onclick={() => reuseExtension(extensionName)}>
        Reuse extension
      </ActionButton>
    {/if}
    {#if handoff.canGenerate && !handoff.generatedName && !existingExtension}
      <ActionButton
        variant="primary"
        size="sm"
        icon={handoff.status === 'generated' ? 'check' : 'spark'}
        disabled={approveDisabled}
        disabledReason={approveDisabledReason}
        busy={capabilityHandoffBusy}
        busyLabel="Working"
        onclick={() => approveChatCapability(messageIndex)}
      >
        {approveLabel}
      </ActionButton>
    {/if}
    {#if !isDomainPack}
      <ActionButton variant="secondary" size="sm" icon="extensions" onclick={() => openMessageCapabilityInExtensions(messageIndex)}>
        Open in Extensions
      </ActionButton>
    {:else}
      <ActionButton variant="secondary" size="sm" icon="skills" onclick={() => openMessageDomainPacks(messageIndex)}>
        Open Domain Packs
      </ActionButton>
    {/if}
  </div>
  {/if}
</div>
