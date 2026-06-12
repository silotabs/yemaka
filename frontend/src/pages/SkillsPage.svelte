<script lang="ts">
  import BitSelect from '../BitSelect.svelte';
  import ActionButton from '../ActionButton.svelte';
  import Badge from '../Badge.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import PaginationControls from '../PaginationControls.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import StructuredDataView from '../StructuredDataView.svelte';
  import {
    availableDomainPackTemplates as availableDomainPackTemplatesFor,
    domainPackForHandoff as domainPackForHandoffFor,
    domainPackHandoffActionDisabled as domainPackHandoffActionDisabledFor,
    domainPackHandoffActionLabel as domainPackHandoffActionLabelFor,
    domainPackHandoffName as domainPackHandoffNameFor,
    domainPackSetupLabel as domainPackSetupLabelFor,
    domainPackSkillRefs as domainPackSkillRefsFor,
    domainPackTemplateInstallPath as domainPackTemplateInstallPathFor,
    domainPackTemplateSafetyText
  } from '../lib/domainPackHelpers';
  import type { DomainPack, DomainPackReview, DomainPackSkillStatus, DomainPackTemplate } from '../lib/appTypes';
  import type { ToastKind } from '../lib/toastCenter';

  type Option = {
    value: string;
    label: string;
  };

  type Skill = {
    name: string;
    version: string;
    description: string;
    requiredTools: string[];
    enabled: boolean;
    valid: boolean;
    validationError: string;
    source: string;
  };

  type CapabilityHandoff = {
    title: string;
    name: string;
    existingCapability?: string;
    suggestedAction: string;
    capabilitySummary?: string;
    configureHint?: string;
    installCommand?: string;
    packDir?: string;
    packSource?: string;
  };

  export let skills: Skill[] = [];
  export let domainPacks: DomainPack[] = [];
  export let domainPackReview: DomainPackReview | null = null;
  export let domainPackTemplates: DomainPackTemplate[] = [];
  export let domainPackSkills: DomainPackSkillStatus[] = [];
  export let domainPackInstallPath = '';
  export let domainPackBusy = false;
  export let skillsPageStatus = '';
  export let skillsPageStatusKind: ToastKind = 'info';
  export let activeDomainPackHandoff: CapabilityHandoff | null = null;
  export let skillSessionId = '';
  export let skillImproveName = '';
  export let skillImportPath = '';
  export let skillExportPath = '';
  export let listPages: Record<string, number> = {};
  export let pageSizeFor: (key: string) => number = () => 10;
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let asArray: <T>(value: T[] | null | undefined) => T[] = (value) => value ?? [];
  export let humanizeIdentifier: (value: string | undefined, fallback?: string) => string = (value) => String(value || '');
  export let humanizeIdentifierList: (values: string[] | null | undefined, fallback?: string) => string = (values) => (values ?? []).join(', ');
  export let createSkillFromSession: () => Promise<void> | void = () => {};
  export let improveSkillFromSession: () => Promise<void> | void = () => {};
  export let importSkill: () => Promise<void> | void = () => {};
  export let exportSkill: (name: string) => Promise<void> | void = () => {};
  export let refreshSkillsView: () => Promise<void> | void = () => {};
  export let installDomainPack: () => Promise<void> | void = () => {};
  export let installDomainPackTemplate: (name: string) => Promise<void> | void = () => {};
  export let reviewDomainPack: (pack: DomainPack) => Promise<void> | void = () => {};
  export let reviewDomainPackTemplate: (name: string) => Promise<void> | void = () => {};
  export let closeDomainPackReview: () => void = () => {};
  export let applyDomainPackHandoff: (handoff: any) => Promise<void> | void = () => {};
  export let setDomainPackEnabled: (name: string, enabled: boolean) => Promise<void> | void = () => {};
  export let uninstallDomainPack: (pack: any) => Promise<void> | void = () => {};
  export let validateSkill: (name: string) => Promise<void> | void = () => {};
  export let setSkillEnabled: (name: string, enabled: boolean) => Promise<void> | void = () => {};

  $: visibleDomainPackTemplates = availableDomainPackTemplatesFor(domainPacks, domainPackTemplates);
  $: activeHandoffName = domainPackHandoffNameFor(activeDomainPackHandoff ?? undefined);
  $: activeHandoffInstallPath = domainPackTemplateInstallPathFor(activeDomainPackHandoff, activeHandoffName);
  $: activeHandoffPack = domainPackForHandoffFor(domainPacks, activeDomainPackHandoff ?? undefined);
  $: activeHandoffActionDisabled = domainPackHandoffActionDisabledFor(
    activeDomainPackHandoff,
    domainPackBusy,
    activeHandoffPack,
    activeHandoffInstallPath,
    activeHandoffName
  );
  $: activeHandoffActionLabel = domainPackHandoffActionLabelFor(activeDomainPackHandoff, activeHandoffPack, activeHandoffInstallPath);
  $: activeHandoffSetupLabel = activeDomainPackHandoff
    ? domainPackSetupLabelFor(activeDomainPackHandoff, activeHandoffPack)
    : 'Open Domain Packs';
  $: domainPackReviewDetails = domainPackReview
    ? {
        name: domainPackReview.name,
        source: domainPackReview.source,
        installed: domainPackReview.installed,
        enabled: domainPackReview.enabled,
        valid: domainPackReview.valid,
        defaultState: domainPackReview.defaultState,
        path: domainPackReview.path,
        validationError: domainPackReview.validationError,
        repairHint: domainPackReview.repairHint,
        requiredTools: domainPackReview.requiredTools ?? [],
        optionalTools: domainPackReview.optionalTools ?? [],
        approvalRequiredFor: domainPackReview.approvalRequiredFor ?? [],
        blockedActions: domainPackReview.blockedActions ?? [],
        sensitiveDataRules: domainPackReview.sensitiveDataRules ?? [],
        riskyToolReasons: domainPackReview.riskyToolReasons ?? [],
        tests: domainPackReview.tests ?? [],
        skills: (domainPackReview.skills ?? []).map((skill) => ({
          ref: skill.ref,
          name: skill.name,
          valid: skill.valid,
          validationError: skill.validationError,
          triggers: skill.triggers ?? [],
          requiredTools: skill.requiredTools ?? [],
          instructionChars: skill.instructionChars
        })),
        permissions: domainPackReview.permissions ?? {},
        safety: domainPackReview.safety ?? {}
    }
    : null;

  function hasDomainPackReviewFor(name: string | undefined) {
    return Boolean(domainPackReviewDetails && domainPackReview?.name === name);
  }
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  {#if skillsPageStatus.trim()}
    <div class="mb-4">
      <PageStatusStrip kind={skillsPageStatusKind} title="Skills status" message={skillsPageStatus} />
    </div>
  {/if}
  <div class="mb-4 grid gap-3 rounded-md border border-line bg-white p-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
    <div>
      <SectionHeader
        compact
        icon="skills"
        title="Create From Session"
        description="Turn a completed conversation into a local skill proposal."
      />
      <div class="flex gap-2">
        <input class="h-10 flex-1 rounded-md border border-line bg-field px-3 text-sm" bind:value={skillSessionId} placeholder="conversation id" />
        <ActionButton
          variant="primary"
          size="md"
          icon="plus"
          disabled={!skillSessionId.trim()}
          disabledReason="Enter a conversation id before creating a skill."
          onclick={createSkillFromSession}
        >
          Create
        </ActionButton>
      </div>
      <div class="mt-2 flex gap-2">
        <BitSelect
          className="min-w-0 flex-1"
          bind:value={skillImproveName}
          placeholder="Choose skill to improve"
          options={[
            { value: '', label: 'Choose skill to improve' },
            ...asArray(skills)
              .filter((skill) => skill.valid)
              .map((skill) => ({ value: skill.name, label: humanizeIdentifier(skill.name, skill.name) }))
          ]}
        />
        <ActionButton
          variant="secondary"
          size="md"
          disabled={!skillSessionId.trim() || !skillImproveName.trim()}
          disabledReason={!skillSessionId.trim() ? 'Enter a conversation id before improving a skill.' : 'Choose a valid skill to improve.'}
          onclick={improveSkillFromSession}
        >
          Improve
        </ActionButton>
      </div>
    </div>
    <div>
      <SectionHeader
        compact
        icon="documents"
        title="Import / Export"
        description="Bring in local skill folders or export validated skills."
      />
      <div class="grid gap-2 lg:grid-cols-2">
        <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={skillImportPath} placeholder="folder to import" />
        <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={skillExportPath} placeholder="export destination" />
      </div>
      <div class="mt-2 flex gap-2">
        <ActionButton
          variant="secondary"
          size="md"
          icon="plus"
          disabled={!skillImportPath.trim()}
          disabledReason="Enter a local skill folder before importing."
          onclick={importSkill}
        >
          Import
        </ActionButton>
      </div>
    </div>
  </div>
  <div class="mb-4 rounded-md border border-line bg-white p-4">
    <SectionHeader
      icon="skills"
      title="Domain Packs"
      description="Profile-local packs are installed from a local folder and stay disabled until enabled."
      meta={`${domainPacks.length} installed | ${visibleDomainPackTemplates.length} templates`}
    >
      <svelte:fragment slot="actions">
        <ActionButton
          variant="secondary"
          size="md"
          icon="retry"
          disabled={domainPackBusy}
          disabledReason="Domain pack refresh is already running."
          busy={domainPackBusy}
          busyLabel="Refreshing"
          onclick={() => refreshSkillsView()}
        >
          Refresh
        </ActionButton>
      </svelte:fragment>
    </SectionHeader>
    {#if activeDomainPackHandoff}
      <div class="mb-3 rounded-md border border-line bg-field px-3 py-3 text-sm" data-smoke="domain-pack-handoff-apply">
        <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
          <div class="min-w-0">
            <div class="font-medium">Chat handoff: {activeDomainPackHandoff.title || humanizeIdentifier(activeHandoffName, activeHandoffName)}</div>
            <div class="break-all text-xs text-slate-500">
              {activeDomainPackHandoff.installCommand || activeHandoffInstallPath || activeHandoffSetupLabel}
            </div>
          </div>
          <ActionButton
            variant="primary"
            size="xs"
            icon="skills"
            disabled={activeHandoffActionDisabled}
            disabledReason="This chat handoff cannot be applied until the required pack setup is available."
            onclick={() => applyDomainPackHandoff(activeDomainPackHandoff)}
          >
            {activeHandoffActionLabel}
          </ActionButton>
        </div>
        <div class="text-xs text-slate-600">
          {activeDomainPackHandoff.configureHint || activeDomainPackHandoff.suggestedAction || activeDomainPackHandoff.capabilitySummary || 'Install stays disabled until you enable the pack explicitly.'}
        </div>
      </div>
    {/if}
    <div class="mb-3 flex flex-col gap-2 sm:flex-row">
      <input class="h-10 min-w-0 flex-1 rounded-md border border-line bg-field px-3 text-sm" bind:value={domainPackInstallPath} placeholder="local pack folder" />
      <ActionButton
        variant="primary"
        size="md"
        icon="plus"
        disabled={domainPackBusy || !domainPackInstallPath.trim()}
        disabledReason={domainPackBusy ? 'Domain pack operation is already running.' : 'Enter a local pack folder before installing.'}
        onclick={installDomainPack}
      >
        Install
      </ActionButton>
    </div>
    {#if visibleDomainPackTemplates.length}
      <div class="mb-3 rounded-md border border-line bg-field p-3">
        <SectionHeader
          compact
          icon="plus"
          title="Built-in pack templates"
          description="Install locally first; enable remains a separate action and core policy still applies."
          meta={`${visibleDomainPackTemplates.length} available`}
        />
        <div class="grid gap-3 lg:grid-cols-3">
          {#each visibleDomainPackTemplates as template}
            <div class="rounded-md border border-line bg-white p-3">
              <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
                <div class="min-w-0 break-all text-sm font-semibold" title={template.name}>{humanizeIdentifier(template.name, template.name)}</div>
                <Badge>{humanizeIdentifier(template.category || 'capability', 'Capability')}</Badge>
              </div>
              <div class="mb-2 text-sm text-slate-700">{template.description || 'Local workflow pack template.'}</div>
              <div class="mb-2 space-y-1 text-xs text-slate-600">
                <div title={asArray(template.requiredTools).join(', ')}>Tools: {humanizeIdentifierList(asArray(template.requiredTools), 'none')}</div>
                <div title={asArray(template.skills).join(', ')}>Skills: {humanizeIdentifierList(asArray(template.skills), 'none')}</div>
                <div title={asArray(template.workflows).join(', ')}>Workflows: {humanizeIdentifierList(asArray(template.workflows), 'none')}</div>
                <div>{domainPackTemplateSafetyText(template)}</div>
              </div>
              <details class="mb-3 rounded-md border border-line bg-field px-3 py-2 text-xs text-slate-600">
                <summary class="cursor-pointer font-medium text-slate-700">Inspect template</summary>
                <div class="mt-2 space-y-1">
                  <div class="break-all">Ref: {template.path || template.name}</div>
                  <div>Default: {humanizeIdentifier(template.defaultState || 'disabled', 'Disabled')}</div>
                  <div title={asArray(template.approvalRequiredFor).join(', ')}>Approval: {humanizeIdentifierList(asArray(template.approvalRequiredFor), 'core policy')}</div>
                  <div title={asArray(template.blockedActions).join(', ')}>Blocked: {humanizeIdentifierList(asArray(template.blockedActions), 'none declared')}</div>
                  {#if asArray(template.sensitiveDataRules).length}
                    <div>Data rules: {asArray(template.sensitiveDataRules).map((rule) => humanizeIdentifier(rule, rule)).join(' ')}</div>
                  {/if}
                </div>
              </details>
              <div class="flex flex-wrap gap-2">
                <ActionButton
                  variant="secondary"
                  size="xs"
                  icon="eye"
                  disabled={domainPackBusy}
                  disabledReason="Domain pack operation is already running."
                  onclick={() => reviewDomainPackTemplate(template.name)}
                >
                  Review
                </ActionButton>
                <ActionButton
                  variant="primary"
                  size="xs"
                  icon="plus"
                  disabled={domainPackBusy || !template.valid}
                  disabledReason={domainPackBusy ? 'Domain pack operation is already running.' : 'This template is invalid and cannot be installed.'}
                  onclick={() => installDomainPackTemplate(template.name)}
                >
                  Install template
                </ActionButton>
              </div>
              {#if hasDomainPackReviewFor(template.name)}
                <div class="mt-3 rounded-md border border-line bg-field p-3" data-smoke="domain-pack-template-review-inline">
                  <SectionHeader
                    compact
                    icon="eye"
                    title={`Pack review: ${humanizeIdentifier(domainPackReview?.name, 'Domain pack')}`}
                    description="Read-only manifest, safety, skill, and test metadata. Enable remains a separate action."
                    meta={domainPackReview?.source ? humanizeIdentifier(domainPackReview.source, 'Local') : 'review'}
                  >
                    <svelte:fragment slot="actions">
                      <ActionButton
                        variant="secondary"
                        size="xs"
                        icon="close"
                        onclick={closeDomainPackReview}
                      >
                        Close
                      </ActionButton>
                    </svelte:fragment>
                  </SectionHeader>
                  <StructuredDataView value={domainPackReviewDetails} title="Review details" maxPreviewLines={18} />
                </div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    {/if}
    {#if (domainPacks ?? []).length === 0}
      <EmptyState
        icon="skills"
        title="No domain packs installed"
        message="Built-in templates can be installed from this page, then enabled explicitly after inspection."
      />
    {:else}
      <div class="grid gap-3 lg:grid-cols-2">
        {#each pagedItems(domainPacks, 'domain-packs', pageSizeFor('domain-packs'), listPages) as pack}
          <div class="rounded-md border border-line p-3">
            <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
              <div class="flex min-w-0 flex-wrap items-center gap-2">
                <span class="min-w-0 break-all text-sm font-semibold" title={pack.name}>{humanizeIdentifier(pack.name, pack.name)}</span>
                <Badge variant={pack.enabled ? 'success' : 'neutral'}>
                  {pack.enabled ? 'enabled' : 'disabled'}
                </Badge>
                <Badge variant={pack.valid ? 'info' : 'danger'}>
                  {pack.valid ? 'valid' : 'invalid'}
                </Badge>
              </div>
              <span class="shrink-0 text-xs text-slate-500">v{pack.version || 'unknown'}</span>
            </div>
            <div class="mb-2 text-xs uppercase text-slate-500">{humanizeIdentifier(pack.category || 'capability', 'Capability')}</div>
            <div class="mb-2 text-sm text-slate-700">{pack.description || 'No description provided.'}</div>
            {#if pack.validationError}
              <div class="mb-2 rounded-md border border-rose-100 bg-rose-50 px-3 py-2 text-xs text-rose-700">{pack.validationError}</div>
            {/if}
            {#if pack.repairHint}
              <div class="mb-2 rounded-md border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-800">{pack.repairHint}</div>
            {/if}
            <div class="mb-2 break-all text-xs text-slate-500">{pack.dir}</div>
            <div class="mb-3 space-y-1 text-xs text-slate-600">
              {#if !pack.enabled}
                <div>Pack skills are inactive until this pack is enabled.</div>
              {/if}
              {#if domainPackSkillRefsFor(domainPackSkills, pack.name).length}
                {#each domainPackSkillRefsFor(domainPackSkills, pack.name) as status}
                  <div class="flex flex-wrap items-center gap-2">
                    <span class="break-all" title={status.ref}>{humanizeIdentifier(status.ref, status.ref)}</span>
                    <Badge variant={status.active ? 'success' : 'neutral'}>
                      {status.active ? 'active' : 'inactive'}
                    </Badge>
                  </div>
                {/each}
              {:else}
                <div title={asArray(pack.skills).join(', ')}>{humanizeIdentifierList(asArray(pack.skills), 'No skills declared.')}</div>
              {/if}
              {#if asArray(pack.requiredTools).length}
                <div title={asArray(pack.requiredTools).join(', ')}>Tools: {humanizeIdentifierList(asArray(pack.requiredTools), 'none')}</div>
              {/if}
            </div>
            <details class="mb-3 rounded-md border border-line bg-field px-3 py-2 text-xs text-slate-600">
              <summary class="cursor-pointer font-medium text-slate-700">Inspect pack</summary>
              <div class="mt-2 space-y-1">
                <div title={asArray(pack.requiredTools).join(', ')}>Required tools: {humanizeIdentifierList(asArray(pack.requiredTools), 'none')}</div>
                <div title={asArray(pack.optionalTools).join(', ')}>Optional tools: {humanizeIdentifierList(asArray(pack.optionalTools), 'none')}</div>
                <div title={asArray(pack.skills).join(', ')}>Skills: {humanizeIdentifierList(asArray(pack.skills), 'none declared')}</div>
                <div>{pack.safetySummary || 'Pack remains subject to core policy and explicit enablement.'}</div>
                <div title={asArray(pack.approvalRequiredFor).join(', ')}>Approval: {humanizeIdentifierList(asArray(pack.approvalRequiredFor), 'core policy')}</div>
                <div title={asArray(pack.blockedActions).join(', ')}>Blocked: {humanizeIdentifierList(asArray(pack.blockedActions), 'none declared')}</div>
                {#if asArray(pack.sensitiveDataRules).length}
                  <div>Data rules: {asArray(pack.sensitiveDataRules).map((rule) => humanizeIdentifier(rule, rule)).join(' ')}</div>
                {/if}
              </div>
            </details>
            <div class="flex flex-wrap gap-2">
              <ActionButton
                variant="secondary"
                size="xs"
                icon="eye"
                disabled={domainPackBusy}
                disabledReason="Domain pack operation is already running."
                onclick={() => reviewDomainPack(pack)}
              >
                Review
              </ActionButton>
              {#if pack.enabled}
                <ActionButton
                  variant="secondary"
                  size="xs"
                  disabled={domainPackBusy}
                  disabledReason="Domain pack operation is already running."
                  onclick={() => setDomainPackEnabled(pack.name, false)}
                >
                  Disable
                </ActionButton>
              {:else}
                <ActionButton
                  variant="secondary"
                  size="xs"
                  disabled={domainPackBusy || !pack.valid}
                  disabledReason={domainPackBusy ? 'Domain pack operation is already running.' : 'Fix this pack before enabling it.'}
                  onclick={() => setDomainPackEnabled(pack.name, true)}
                >
                  Enable
                </ActionButton>
              {/if}
              <ActionButton
                variant="danger"
                size="xs"
                icon="trash"
                disabled={domainPackBusy}
                disabledReason="Domain pack operation is already running."
                onclick={() => uninstallDomainPack(pack)}
              >
                Uninstall
              </ActionButton>
            </div>
            {#if hasDomainPackReviewFor(pack.name)}
              <div class="mt-3 rounded-md border border-line bg-field p-3" data-smoke="domain-pack-installed-review-inline">
                <SectionHeader
                  compact
                  icon="eye"
                  title={`Pack review: ${humanizeIdentifier(domainPackReview?.name, 'Domain pack')}`}
                  description="Read-only manifest, safety, skill, and test metadata. Enable remains a separate action."
                  meta={domainPackReview?.source ? humanizeIdentifier(domainPackReview.source, 'Local') : 'review'}
                >
                  <svelte:fragment slot="actions">
                    <ActionButton
                      variant="secondary"
                      size="xs"
                      icon="close"
                      onclick={closeDomainPackReview}
                    >
                      Close
                    </ActionButton>
                  </svelte:fragment>
                </SectionHeader>
                <StructuredDataView value={domainPackReviewDetails} title="Review details" maxPreviewLines={18} />
              </div>
            {/if}
          </div>
        {/each}
      </div>
      <PaginationControls
        page={currentPage('domain-packs', domainPacks.length, pageSizeFor('domain-packs'), listPages)}
        total={domainPacks.length}
        pageSize={pageSizeFor('domain-packs')}
        label="packs"
        onChange={(page) => setListPage('domain-packs', page)}
      />
    {/if}
  </div>
  {#if (skills ?? []).length > 0}
    <SectionHeader
      icon="skills"
      title="Installed Skills"
      description="Validated local skills and domain-pack skills available to Yemaka."
      meta={`${skills.length} skills`}
    />
  {/if}
  {#if (skills ?? []).length === 0}
    <EmptyState
      icon="skills"
      title="No skills installed"
      message="Default skills and enabled domain-pack skills will appear here when they are available."
    />
  {:else}
    <div class="grid gap-3 lg:grid-cols-2">
      {#each pagedItems(skills, 'skills-list', pageSizeFor('skills-list'), listPages) as skill}
        <div class="rounded-md border border-line bg-white p-4">
          <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <span class="min-w-0 break-all text-sm font-semibold" title={skill.name}>{humanizeIdentifier(skill.name, skill.name)}</span>
              <Badge variant={skill.enabled ? 'success' : 'neutral'}>
                {skill.enabled ? 'enabled' : 'disabled'}
              </Badge>
              <Badge variant={skill.valid ? 'info' : 'danger'}>
                {skill.valid ? 'valid' : 'invalid'}
              </Badge>
            </div>
            <span class="shrink-0 text-xs text-slate-500">v{skill.version}</span>
          </div>
          <div class="mb-3 text-sm text-slate-700">{skill.description}</div>
          {#if skill.validationError}
            <div class="mb-3 rounded-md border border-rose-100 bg-rose-50 px-3 py-2 text-xs text-rose-700">{skill.validationError}</div>
          {/if}
          <div class="mb-3 text-xs text-slate-500" title={`Tools: ${skill.requiredTools.join(', ')} | Source: ${skill.source}`}>
            Tools: {humanizeIdentifierList(skill.requiredTools, 'none')} | Source: {humanizeIdentifier(skill.source, 'Local')}
          </div>
          <div class="flex flex-wrap gap-2">
            <ActionButton variant="secondary" size="xs" onclick={() => validateSkill(skill.name)}>
              Validate
            </ActionButton>
            {#if skill.enabled}
              <ActionButton variant="secondary" size="xs" onclick={() => setSkillEnabled(skill.name, false)}>
                Disable
              </ActionButton>
            {:else if skill.valid}
              <ActionButton variant="secondary" size="xs" onclick={() => setSkillEnabled(skill.name, true)}>
                Enable
              </ActionButton>
            {/if}
            <ActionButton
              variant="secondary"
              size="xs"
              disabled={!skillExportPath.trim() || !skill.valid}
              disabledReason={!skill.valid ? 'Validate this skill before exporting.' : 'Enter an export destination before exporting.'}
              onclick={() => exportSkill(skill.name)}
            >
              Export
            </ActionButton>
          </div>
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('skills-list', skills.length, pageSizeFor('skills-list'), listPages)}
      total={skills.length}
      pageSize={pageSizeFor('skills-list')}
      label="skills"
      onChange={(page) => setListPage('skills-list', page)}
    />
  {/if}
</div>
