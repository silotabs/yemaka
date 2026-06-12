<script lang="ts">
  import BitSelect from '../BitSelect.svelte';
  import ActionButton from '../ActionButton.svelte';
  import Badge from '../Badge.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import PaginationControls from '../PaginationControls.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import StructuredDataView from '../StructuredDataView.svelte';
  import { jsonObjectInputError } from '../lib/schedulerJobHelpers';
  import { humanizePermissionList } from '../lib/uiHelpers';

  type Option = {
    value: string;
    label: string;
  };

  type Extension = {
    name: string;
    version: string;
    type: string;
    description: string;
    manifestPath: string;
    enabled: boolean;
    valid: boolean;
    runnable: boolean;
    callable: boolean;
    runBlockedReason: string;
    validationError: string;
  };

  type ExtensionReviewFile = {
    path: string;
    size: number;
    content?: string;
    truncated?: boolean;
    omittedReason?: string;
  };

  type ExtensionReview = {
    detail: {
      status: {
        name: string;
      };
      inspection: {
        status?: string;
      };
    };
    files: ExtensionReviewFile[];
    suggestedActions: string[];
    sampleInputJson?: string;
    canRun: boolean;
    canRegister: boolean;
    runBlockedReason?: string;
    registerBlockedReason?: string;
  };

  type ExtensionFailureTrend = {
    name: string;
    failures: number;
    lastStatus: string;
    lastError?: string;
    suggestReview: boolean;
  };

  type CapabilityProposal = {
    kind: string;
    name: string;
    title: string;
    description: string;
    reason: string;
    permissions: string[];
    generationCommand: string;
    networkMode: string;
  };

  export let extensions: Extension[] = [];
  export let extensionBusy = false;
  export let extensionProposalRequest = '';
  export let extensionGenerateName = '';
  export let extensionGenerateDescription = '';
  export let extensionGenerateRun = false;
  export let extensionGenerateJob = false;
  export let extensionJobScheduleType = 'manual';
  export let extensionJobScheduleExpr = '';
  export let extensionJobEnabled = false;
  export let extensionJobInputJSON = '{}';
  export let extensionRunName = '';
  export let extensionRunInputJSON = '{}';
  export let extensionRunOutput = '';
  export let extensionRollbackTarget = '';
  export let extensionFailures: ExtensionFailureTrend[] = [];
  export let extensionReview: ExtensionReview | null = null;
  export let extensionReviewBusy = false;
  export let extensionReviewOpenFile = '';
  export let capabilityProposal: CapabilityProposal | null = null;
  export let capabilityNextSteps: string[] = [];
  export let jobScheduleTypeOptions: Option[] = [];
  export let listPages: Record<string, number> = {};
  export let pageSizeFor: (key: string) => number = () => 10;
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let refreshExtensions: () => Promise<void> | void = () => {};
  export let proposeExtension: () => Promise<void> | void = () => {};
  export let generateExtension: () => Promise<void> | void = () => {};
  export let runExtension: () => Promise<void> | void = () => {};
  export let rollbackExtension: () => Promise<void> | void = () => {};
  export let testAndRegisterExtension: (name: string) => Promise<void> | void = () => {};
  export let useReviewedExtensionInRunner: () => void = () => {};
  export let reviewExtension: (name: string) => Promise<void> | void = () => {};
  export let validateExtension: (name: string) => Promise<void> | void = () => {};
  export let testExtension: (name: string) => Promise<void> | void = () => {};
  export let registerExtension: (name: string) => Promise<void> | void = () => {};
  export let reuseExtension: (name: string) => Promise<void> | void = () => {};
  export let setExtensionEnabled: (name: string, enabled: boolean) => Promise<void> | void = () => {};
  export let deleteExtension: (name: string) => Promise<void> | void = () => {};
  export let humanizeIdentifier: (value: string | undefined, fallback?: string) => string = (value) => String(value || '');

  let selectedReviewFile: ExtensionReviewFile | null = null;

  $: selectedReviewFile = extensionReview
    ? extensionReview.files.find((file) => file.path === extensionReviewOpenFile) ?? extensionReview.files[0] ?? null
    : null;
  $: capabilityRequestReady = extensionProposalRequest.trim().length > 0;
  $: generatedCapabilityReady =
    capabilityRequestReady || (extensionGenerateName.trim().length > 0 && extensionGenerateDescription.trim().length > 0);
  $: extensionJobScheduleMissing =
    extensionGenerateJob && extensionJobScheduleType !== 'manual' && extensionJobScheduleExpr.trim().length === 0;
  $: extensionJobInputJSONError = extensionGenerateJob ? jsonObjectInputError(extensionJobInputJSON, 'Job input') : '';
  $: extensionRunInputJSONError = jsonObjectInputError(extensionRunInputJSON, 'Run input');
  $: generateCapabilityDisabled =
    !generatedCapabilityReady ||
    extensionJobScheduleMissing ||
    Boolean(extensionJobInputJSONError) ||
    (extensionGenerateRun && Boolean(extensionRunInputJSONError));
  $: generateCapabilityDisabledReason = extensionJobInputJSONError
    ? extensionJobInputJSONError
    : extensionGenerateRun && extensionRunInputJSONError
      ? extensionRunInputJSONError
      : extensionJobScheduleMissing
    ? 'Enter a schedule expression before creating a scheduled capability.'
    : 'Describe the capability, or provide a generated extension name and description.';
  $: runExtensionDisabled = !extensionRunName.trim() || Boolean(extensionRunInputJSONError);
  $: runExtensionDisabledReason = extensionRunInputJSONError || 'Enter an extension name before running.';

  function safeCapabilityNextStep(step: string) {
    const clean = String(step || '').replace(/\s+/g, ' ').trim();
    const lower = clean.toLowerCase();
    if (!clean) return '';
    if (lower.includes('yemaka ') || lower.startsWith('cli:') || lower.includes('--yes')) {
      if (lower.includes('job') || lower.includes('scheduler')) return 'Create a scheduler job only after review and explicit approval.';
      if (lower.includes('generate')) return 'Generate only after reviewing the proposed capability and permissions.';
      return 'Use the reviewed extension workflow before running this capability.';
    }
    return clean;
  }
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  {#if extensionBusy || extensionReviewBusy}
    <div class="mb-4">
      <PageStatusStrip
        kind="info"
        title={extensionReviewBusy ? 'Extension review running' : 'Extension refresh running'}
        message={extensionReviewBusy ? 'Yemaka is inspecting generated files, manifests, tests, and reuse guidance.' : 'Yemaka is refreshing generated extension status.'}
      />
    </div>
  {/if}
  <SectionHeader
    compact
    icon="extensions"
    title="Generated Extensions"
    description="Profile-local tools, validation, tests, execution, and rollback."
    meta={`${extensions.length} extensions`}
    className="mb-4"
  >
    <svelte:fragment slot="actions">
      <ActionButton variant="secondary" icon="retry" busy={extensionBusy} busyLabel="Refreshing" onclick={() => refreshExtensions()}>
        Refresh
      </ActionButton>
    </svelte:fragment>
  </SectionHeader>
  {#if capabilityProposal}
    <div class="mb-4 rounded-md border border-line bg-white p-4 text-sm">
      <div class="mb-1 flex flex-wrap items-center gap-2">
        <span class="font-semibold" title={capabilityProposal.name}>{capabilityProposal.title || humanizeIdentifier(capabilityProposal.name, capabilityProposal.name)}</span>
        {#if capabilityProposal.kind}
          <Badge>{humanizeIdentifier(capabilityProposal.kind, 'Capability')}</Badge>
        {/if}
        {#if capabilityProposal.networkMode}
          <Badge variant="success">{humanizeIdentifier(capabilityProposal.networkMode, capabilityProposal.networkMode)}</Badge>
        {/if}
      </div>
      <div class="mb-2 text-slate-600">{capabilityProposal.description || capabilityProposal.reason}</div>
      {#if capabilityProposal.permissions?.length}
        <div class="mb-2 flex flex-wrap gap-2">
          {#each capabilityProposal.permissions.slice(0, 5) as permission}
            <Badge variant="neutral">{humanizePermissionList([permission], permission)}</Badge>
          {/each}
        </div>
      {/if}
      {#if capabilityProposal.generationCommand}
        <PageStatusStrip
          kind="info"
          title="Generation requires approval"
          message="Yemaka will generate, test, and register this extension only after explicit approval."
        />
      {/if}
      {#if capabilityNextSteps.length}
        <div class="mt-3 space-y-1 text-xs text-slate-600">
          {#each capabilityNextSteps as step}
            <div>{safeCapabilityNextStep(step)}</div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}

  <div class="mb-4 grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
    <div class="rounded-md border border-line bg-white p-4">
      <SectionHeader
        compact
        icon="plus"
        title="Generate Capability"
        description="Propose the smallest profile-local tool, then generate it only after approval."
        className="mb-3"
      />
      <textarea class="mb-3 h-20 w-full resize-none rounded-md border border-line bg-field p-3 text-sm" bind:value={extensionProposalRequest} placeholder="Missing capability request"></textarea>
      <div class="mb-3 flex flex-wrap gap-2">
        <ActionButton
          variant="secondary"
          icon="spark"
          disabled={!capabilityRequestReady}
          disabledReason="Enter a capability request before proposing an extension."
          onclick={proposeExtension}
        >
          Propose
        </ActionButton>
        <ActionButton
          variant="primary"
          icon="plus"
          disabled={generateCapabilityDisabled}
          disabledReason={generateCapabilityDisabledReason}
          onclick={generateExtension}
        >
          Generate capability
        </ActionButton>
        <label class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-line px-3 py-2 text-sm text-slate-600 hover:bg-slate-50">
          <input type="checkbox" class="accent-pine" bind:checked={extensionGenerateRun} />
          <span>Run once</span>
        </label>
        <label class="inline-flex cursor-pointer items-center gap-2 rounded-md border border-line px-3 py-2 text-sm text-slate-600 hover:bg-slate-50">
          <input type="checkbox" class="accent-pine" bind:checked={extensionGenerateJob} />
          <span>Create job</span>
        </label>
      </div>
      <div class="grid gap-3 md:grid-cols-[220px_minmax(0,1fr)]">
        <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={extensionGenerateName} placeholder="extension_name" />
        <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={extensionGenerateDescription} placeholder="Description" />
      </div>
      {#if extensionGenerateJob}
        <div class="mt-3 rounded-md border border-line bg-field p-3">
          <div class="mb-2 text-xs font-medium uppercase tracking-[0.08em] text-slate-500">Approved scheduler job</div>
          <div class="grid gap-3 md:grid-cols-[160px_minmax(0,1fr)_150px]">
            <BitSelect bind:value={extensionJobScheduleType} options={jobScheduleTypeOptions} />
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={extensionJobScheduleExpr} placeholder="1h, cron, or RFC3339" disabled={extensionJobScheduleType === 'manual'} />
            <label class="inline-flex cursor-pointer items-center justify-center gap-2 rounded-md border border-line bg-white px-3 py-2 text-sm text-slate-600">
              <input type="checkbox" class="accent-pine" bind:checked={extensionJobEnabled} />
              <span>Enable</span>
            </label>
          </div>
          <textarea
            class={`mt-3 h-16 w-full resize-none rounded-md border bg-white p-3 text-sm ${extensionJobInputJSONError ? 'border-amber-300' : 'border-line'}`}
            bind:value={extensionJobInputJSON}
            placeholder="Job input JSON object"
          ></textarea>
          {#if extensionJobInputJSONError}
            <PageStatusStrip className="mt-3" kind="warning" title="Invalid scheduler input JSON" message={extensionJobInputJSONError} />
          {/if}
          <div class="mt-2 text-xs text-slate-500">Jobs are approved explicitly here. They stay disabled unless Enable is checked.</div>
        </div>
      {/if}
    </div>
    <div class="space-y-4">
      <div class="rounded-md border border-line bg-white p-4">
        <SectionHeader
          compact
          icon="retry"
          title="Run Or Roll Back"
          description="Run a callable extension or roll back a generated capability snapshot."
          className="mb-3"
        />
        <input class="mb-3 h-10 w-full rounded-md border border-line bg-field px-3 text-sm" bind:value={extensionRunName} placeholder="extension name" />
        <textarea
          class={`mb-3 h-20 w-full resize-none rounded-md border bg-field p-3 text-sm ${extensionRunInputJSONError ? 'border-amber-300' : 'border-line'}`}
          bind:value={extensionRunInputJSON}
          placeholder="Run input JSON object"
        ></textarea>
        {#if extensionRunInputJSONError}
          <PageStatusStrip className="mb-3" kind="warning" title="Invalid run input JSON" message={extensionRunInputJSONError} />
        {/if}
        <div class="mb-3 flex flex-wrap gap-2">
          <ActionButton
            variant="primary"
            icon="send"
            disabled={runExtensionDisabled}
            disabledReason={runExtensionDisabledReason}
            onclick={runExtension}
          >
            Run
          </ActionButton>
        </div>
        <div class="flex gap-2">
          <input class="h-10 min-w-0 flex-1 rounded-md border border-line bg-field px-3 text-sm" bind:value={extensionRollbackTarget} placeholder="snapshot or extension" />
          <ActionButton
            variant="danger"
            icon="retry"
            disabled={!extensionRollbackTarget.trim()}
            disabledReason="Enter a snapshot or extension name before rolling back."
            onclick={rollbackExtension}
          >
            Rollback
          </ActionButton>
        </div>
        {#if extensionRunOutput}
          <StructuredDataView className="mt-3" value={extensionRunOutput} title="Run output" maxPreviewLines={12} />
        {/if}
      </div>

      <div class="rounded-md border border-line bg-white p-4">
        <SectionHeader
          compact
          icon="eye"
          title="Review And Reuse"
          description="Inspect generated files before running existing capabilities."
          className="mb-3"
        >
          <svelte:fragment slot="actions">
            {#if extensionReviewBusy}
              <Badge>reviewing</Badge>
            {/if}
          </svelte:fragment>
        </SectionHeader>
        {#if extensionReview}
          <div class="mb-3 flex flex-wrap items-center gap-2 text-xs">
            <Badge title={extensionReview.detail.status.name}>{humanizeIdentifier(extensionReview.detail.status.name, extensionReview.detail.status.name)}</Badge>
            <Badge variant={extensionReview.canRun ? 'success' : 'warning'}>
              {extensionReview.canRun ? 'callable' : 'not callable'}
            </Badge>
            <Badge>{humanizeIdentifier(extensionReview.detail.inspection.status || 'unchecked', 'Unchecked')}</Badge>
          </div>
          <div class="mb-3 space-y-1 text-xs text-slate-600">
            {#each extensionReview.suggestedActions.slice(0, 4) as action}
              <div>{action}</div>
            {/each}
          </div>
          {#if extensionReview.registerBlockedReason}
            <div class="mb-3 rounded-md border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-800">{extensionReview.registerBlockedReason}</div>
          {/if}
          {#if extensionReview.runBlockedReason}
            <div class="mb-3 rounded-md border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-800">{extensionReview.runBlockedReason}</div>
          {/if}
          {#if extensionReview.sampleInputJson}
            <StructuredDataView className="mb-3" value={extensionReview.sampleInputJson} title="Runner sample input" language="json" maxPreviewLines={10} />
          {/if}
          <div class="mb-3 flex flex-wrap gap-2">
            <ActionButton
              size="xs"
              variant="primary"
              icon="check"
              disabled={!extensionReview.canRegister}
              disabledReason={extensionReview.registerBlockedReason || 'This extension cannot register until validation, tests, and policy checks pass.'}
              onclick={() => testAndRegisterExtension(extensionReview?.detail.status.name || '')}
            >
              Retest + register
            </ActionButton>
            <ActionButton
              size="xs"
              variant="secondary"
              icon="send"
              disabled={!extensionReview.canRun}
              disabledReason={extensionReview.runBlockedReason || 'This extension is not callable yet.'}
              onclick={useReviewedExtensionInRunner}
            >
              Use in runner
            </ActionButton>
          </div>
          <div class="mb-2 flex flex-wrap gap-1.5">
            {#each extensionReview.files.slice(0, 8) as file}
              <button
                class={extensionReviewOpenFile === file.path ? 'rounded border border-pine bg-emerald-50 px-2 py-1 text-xs text-pine' : 'rounded border border-line px-2 py-1 text-xs text-slate-600 hover:bg-slate-50'}
                type="button"
                onclick={() => (extensionReviewOpenFile = file.path)}
              >
                {file.path}
              </button>
            {/each}
          </div>
          {#if selectedReviewFile}
            <div class="rounded-md border border-line bg-field p-3 text-xs">
              <div class="mb-2 flex flex-wrap items-center justify-between gap-2 text-slate-500">
                <span class="break-all">{selectedReviewFile.path}</span>
                <span>{selectedReviewFile.size ?? 0} bytes</span>
              </div>
              {#if selectedReviewFile.omittedReason}
                <div class="text-slate-600">{selectedReviewFile.omittedReason}</div>
              {:else}
                <StructuredDataView
                  value={selectedReviewFile.content}
                  title="File preview"
                  filename={selectedReviewFile.path}
                  maxPreviewLines={16}
                />
                {#if selectedReviewFile.truncated}
                  <div class="mt-2 text-amber-700">Preview truncated for low-memory safety.</div>
                {/if}
              {/if}
            </div>
          {/if}
        {:else}
          <EmptyState
            icon="extensions"
            title="No extension selected"
            message="Select Review on an extension to see manifest, source preview, tests, and reuse guidance."
          />
        {/if}
      </div>
    </div>
  </div>

  {#if (extensionFailures ?? []).length}
    <div class="mb-4 rounded-md border border-line bg-white">
      <div class="border-b border-line px-4 py-3">
        <SectionHeader
          compact
          icon="warning"
          title="Failure Trends"
          description="Repeated extension failures that may need review or repair."
          meta={`${extensionFailures.length} trends`}
        />
      </div>
      <div class="divide-y divide-line">
        {#each pagedItems(extensionFailures, 'extension-failures', pageSizeFor('extension-failures'), listPages) as failure}
          <div class="grid gap-3 p-4 text-sm lg:grid-cols-[220px_120px_minmax(0,1fr)]">
            <div class="font-medium" title={failure.name}>{humanizeIdentifier(failure.name, failure.name)}</div>
            <div class={failure.suggestReview ? 'text-amber-800' : 'text-slate-600'}>{failure.failures} failure{failure.failures === 1 ? '' : 's'}</div>
            <div class="break-all text-slate-600">{failure.lastStatus} {failure.lastError || ''}</div>
          </div>
        {/each}
      </div>
      <PaginationControls
        page={currentPage('extension-failures', extensionFailures.length, pageSizeFor('extension-failures'), listPages)}
        total={extensionFailures.length}
        pageSize={pageSizeFor('extension-failures')}
        label="failures"
        onChange={(page) => setListPage('extension-failures', page)}
      />
    </div>
  {/if}

  {#if (extensions ?? []).length === 0}
    <EmptyState
      icon="extensions"
      title="No generated extensions registered yet"
      message="Generated capabilities will appear here after they pass manifest validation, tests, and policy checks."
    />
  {:else}
    <div class="grid gap-3 lg:grid-cols-2">
      {#each pagedItems(extensions, 'extensions-list', pageSizeFor('extensions-list'), listPages) as extension}
        <div class="rounded-md border border-line bg-white p-4">
          <div class="mb-1 flex flex-wrap items-center justify-between gap-3">
            <div class="flex min-w-0 flex-wrap items-center gap-2">
              <span class="min-w-0 break-all text-sm font-semibold" title={extension.name}>{humanizeIdentifier(extension.name, extension.name)}</span>
              <Badge variant={extension.enabled ? 'success' : 'neutral'}>
                {extension.enabled ? 'enabled' : 'disabled'}
              </Badge>
              <Badge variant={extension.valid ? 'info' : 'danger'}>
                {extension.valid ? 'valid' : 'invalid'}
              </Badge>
            </div>
            <span class="shrink-0 text-xs text-slate-500">v{extension.version || 'unknown'}</span>
          </div>
          <div class="mb-2 text-xs uppercase text-slate-500">{humanizeIdentifier(extension.type || 'extension', 'Extension')}</div>
          <div class="mb-3 text-sm text-slate-700">{extension.description || 'No description provided.'}</div>
          {#if extension.validationError}
            <div class="mb-3 rounded-md border border-rose-100 bg-rose-50 px-3 py-2 text-xs text-rose-700">{extension.validationError}</div>
          {/if}
          {#if extension.runBlockedReason}
            <div class="mb-3 rounded-md border border-amber-100 bg-amber-50 px-3 py-2 text-xs text-amber-800">{extension.runBlockedReason}</div>
          {/if}
          <div class="mb-3 break-all text-xs text-slate-500">{extension.manifestPath}</div>
          <div class="flex flex-wrap gap-2">
            <ActionButton size="xs" variant="secondary" icon="eye" onclick={() => reviewExtension(extension.name)}>Review</ActionButton>
            <ActionButton size="xs" variant="secondary" icon="check" onclick={() => validateExtension(extension.name)}>Validate</ActionButton>
            <ActionButton size="xs" variant="secondary" icon="learning" onclick={() => testExtension(extension.name)}>Test</ActionButton>
            <ActionButton
              size="xs"
              variant="primary"
              icon="check"
              onclick={() => testAndRegisterExtension(extension.name)}
              disabled={!extension.valid || extension.type !== 'tool'}
              disabledReason={extension.valid ? 'Only generated tool extensions can be registered.' : extension.validationError || 'Validate this extension before registration.'}
            >
              Retest + register
            </ActionButton>
            <ActionButton
              size="xs"
              variant="primary"
              icon="plus"
              onclick={() => registerExtension(extension.name)}
              disabled={!extension.valid || extension.type !== 'tool'}
              disabledReason={extension.valid ? 'Only generated tool extensions can be registered.' : extension.validationError || 'Validate this extension before registration.'}
            >
              Register
            </ActionButton>
            {#if extension.callable}
              <ActionButton size="xs" variant="secondary" icon="send" onclick={() => reuseExtension(extension.name)}>Use in runner</ActionButton>
            {/if}
            {#if extension.enabled}
              <ActionButton size="xs" variant="secondary" icon="stop" onclick={() => setExtensionEnabled(extension.name, false)}>Disable</ActionButton>
            {:else}
              <ActionButton
                size="xs"
                variant="secondary"
                icon="check"
                disabled={!extension.valid || !extension.runnable}
                disabledReason={extension.validationError || extension.runBlockedReason || 'This extension must be valid and runnable before it can be enabled.'}
                onclick={() => setExtensionEnabled(extension.name, true)}
              >
                Enable
              </ActionButton>
            {/if}
            <ActionButton size="xs" variant="danger" icon="trash" onclick={() => deleteExtension(extension.name)}>Delete</ActionButton>
          </div>
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('extensions-list', extensions.length, pageSizeFor('extensions-list'), listPages)}
      total={extensions.length}
      pageSize={pageSizeFor('extensions-list')}
      label="extensions"
      onChange={(page) => setListPage('extensions-list', page)}
    />
  {/if}
</div>
