<script lang="ts">
  import BitSelect from '../BitSelect.svelte';
  import ActionButton from '../ActionButton.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import Icon from '../Icon.svelte';

  type Option = {
    value: string;
    label: string;
  };

  type RuntimeProvider = {
    id: string;
    label: string;
    baseURL: string;
  };

  type ModelInfo = {
    name: string;
    modifiedAt: string;
    size: number;
    digest?: string;
  };

  type ModelDetails = {
    name: string;
    family: string;
    format: string;
    parameterSize: string;
    quantizationLevel: string;
    contextLength: number;
  };

  type ModelProfile = {
    name: string;
    description?: string;
    baseModel: string;
    system?: string;
    parameters?: {
      temperature?: number;
      numCtx?: number;
    };
    metadata?: {
      purpose?: string;
      refinedFrom?: string;
      createdBy?: string;
    };
    tags?: string[];
  };

  type ModelProfileResult = {
    profile: ModelProfile;
    path?: string;
    modelfile?: string;
    message?: string;
    readiness?: {
      schema: string;
      status: string;
      score: number;
      max: number;
      checks?: Array<{
        id: string;
        status: string;
        message: string;
        points: number;
        max: number;
      }>;
    };
    comparison?: {
      schema: string;
      baselineName?: string;
      candidateName: string;
      hasBaseline: boolean;
      sameArtifact: boolean;
      changedFields?: string[];
      addedTags?: string[];
      removedTags?: string[];
      systemCharsDelta?: number;
      readinessScoreDelta?: number;
    };
    history?: Array<{
      schema: string;
      kind: string;
      profileName: string;
      path?: string;
      modifiedAt?: string;
      sizeBytes?: number;
      readinessScore: number;
      message?: string;
    }>;
    safetySummary?: string[];
    draftReport?: {
      schema: string;
      conversationId: string;
      eligibility: string;
      reason: string;
      messageCount: number;
      assistantMessages: number;
      toolRunCount: number;
      failedToolRuns: number;
      observedTraits?: string[];
      privacyFilter?: {
        secretsRedacted?: boolean;
        pathsRedacted?: boolean;
      };
      safetySummary?: string[];
      readiness?: ModelProfileResult['readiness'];
    };
  };

  type Status = {
    modelStatuses?: Array<{ role: string; name: string; installed: boolean; profile?: string; profileValid?: boolean; profileStatus?: string }>;
  };

  export let models: ModelInfo[] = [];
  export let status: Status | null = null;
  export let modelRole = 'low_memory';
  export let modelProvider = 'ollama';
  export let modelBaseURL = '';
  export let modelName = '';
  export let modelPrompt = '';
  export let modelBusy = false;
  export let modelDetails: ModelDetails | null = null;
  export let modelOutput = '';
  export let modelProfiles: ModelProfile[] = [];
  export let modelProfileName = '';
  export let modelProfileDescription = '';
  export let modelProfileBaseModel = '';
  export let modelProfileSystem = '';
  export let modelProfileTemperature = 0.2;
  export let modelProfileNumCtx = 4096;
  export let modelProfileTags = '';
  export let modelProfilePurpose = '';
  export let modelProfileRefinedFrom = '';
  export let modelProfileConversationId = '';
  export let modelProfileResult: ModelProfileResult | null = null;
  export let modelProfileBusy = false;
  export let modelRoleOptions: Option[] = [];
  export let runtimeProviders: RuntimeProvider[] = [];
  export let selectModel: (name: string) => Promise<void> | void = () => {};
  export let saveModelRole: () => Promise<void> | void = () => {};
  export let showModelDetails: () => Promise<void> | void = () => {};
  export let generateModelText: () => Promise<void> | void = () => {};
  export let refreshModelProfiles: () => Promise<void> | void = () => {};
  export let inspectModelProfile: (name: string) => Promise<void> | void = () => {};
  export let previewModelProfile: () => Promise<void> | void = () => {};
  export let draftModelProfileFromConversation: () => Promise<void> | void = () => {};
  export let saveModelProfile: () => Promise<void> | void = () => {};
  export let applyModelProfile: () => Promise<void> | void = () => {};
  export let onModelRoleChange: (next: string) => void = () => {};
  export let onModelProviderChange: (next: string) => void = () => {};

  function modelRoleLabel(role: string) {
    const clean = String(role || '').trim();
    if (!clean) return 'Unknown';
    return modelRoleOptions.find((option) => option.value === clean)?.label ?? clean;
  }

  function scoreLabel(score?: number, max?: number) {
    if (typeof score !== 'number') return 'Not scored';
    return `${score}/${max || 100}`;
  }
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  {#if modelBusy}
    <div class="mb-4">
      <PageStatusStrip kind="info" title="Model action running" message="Yemaka is checking or generating with the selected runtime model." />
    </div>
  {/if}
  {#if modelProfileBusy}
    <div class="mb-4">
      <PageStatusStrip kind="info" title="Model profile action running" message="Yemaka is validating the local profile artifact and rendering a Modelfile preview." />
    </div>
  {/if}
  <div class="grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
    <div class="rounded-md border border-line bg-white">
      <div class="border-b border-line px-4 py-3">
        <SectionHeader
          compact
          icon="models"
          title="Installed Runtime Models"
          description="Local runtime models available for Yemaka roles."
          meta={`${models.length} models`}
        />
      </div>
      <div class="divide-y divide-line">
        {#if (models ?? []).length === 0}
          <div class="p-4">
            <EmptyState
              icon="models"
              title="No installed local models detected"
              message="Install an Ollama model, then refresh so Yemaka can assign it to a role."
            />
          </div>
        {/if}
        {#each models ?? [] as model}
          <button
            class="flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-sm soft-hover"
            type="button"
            aria-pressed={modelName === model.name}
            onclick={() => selectModel(model.name)}
          >
            <span class="flex items-center gap-2 justify-between min-w-0 break-all font-medium truncate"><Icon name="models" size={16} /> {model.name}</span>
            <span class="shrink-0 text-xs text-slate-500">{Math.round(model.size / 1024 / 1024)} MB</span>
          </button>
        {/each}
      </div>
    </div>
    <div class="rounded-md border border-line bg-white p-4">
      <SectionHeader
        compact
        icon="models"
        title="Model Role"
        description="Assign a local model to a Yemaka role."
        className="mb-3"
      />
      <BitSelect
        className="mb-2 h-9"
        bind:value={modelRole}
        options={modelRoleOptions}
        onChange={(next) => {
          modelRole = next;
          onModelRoleChange(next);
        }}
      />
      <BitSelect
        className="mb-2 h-9"
        bind:value={modelProvider}
        options={runtimeProviders.map((provider) => ({ value: provider.id, label: provider.label }))}
        onChange={(next) => {
          modelProvider = next;
          onModelProviderChange(next);
        }}
      />
      <input class="mb-2 h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelBaseURL} />
      <input class="mb-3 h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelName} />
      <div class="flex gap-2">
        <ActionButton
          variant="primary"
          icon="check"
          disabled={modelBusy || !modelName.trim()}
          disabledReason={modelBusy ? 'A model action is already running.' : 'Select or enter a model name before setting a role.'}
          onclick={saveModelRole}
        >
          Set Role
        </ActionButton>
        <ActionButton
          variant="secondary"
          icon="eye"
          disabled={modelBusy || !modelName.trim()}
          disabledReason={modelBusy ? 'A model action is already running.' : 'Select or enter a model name before viewing details.'}
          onclick={showModelDetails}
        >
          Details
        </ActionButton>
      </div>
      <div class="mt-4">
        <SectionHeader
          compact
          icon="send"
          title="Generate Test"
          description="Run a short prompt against the selected model."
          className="mb-2"
        />
        <textarea class="mb-2 h-20 w-full resize-none rounded-md border border-line bg-field px-2 py-2 text-sm" bind:value={modelPrompt}></textarea>
        <ActionButton
          variant="secondary"
          icon="send"
          disabled={modelBusy || !modelName.trim() || !modelPrompt.trim()}
          disabledReason={modelBusy ? 'A model action is already running.' : 'Select a model and enter a test prompt before generating.'}
          onclick={generateModelText}
        >
          Generate
        </ActionButton>
      </div>
      {#if modelDetails}
        <div class="mt-4 rounded-md border border-line bg-slate-50 p-3 text-xs text-slate-700">
          <div class="mb-1 font-semibold">{modelDetails.name}</div>
          <div>Family: {modelDetails.family || 'unknown'}</div>
          <div>Parameters: {modelDetails.parameterSize || 'unknown'}</div>
          <div>Quantization: {modelDetails.quantizationLevel || 'unknown'}</div>
          <div>Context: {modelDetails.contextLength || 'unknown'}</div>
          <div>Format: {modelDetails.format || 'unknown'}</div>
        </div>
      {/if}
      {#if modelOutput}
        <pre class="mt-4 max-h-44 overflow-y-auto rounded-md border border-line bg-ink p-3 text-xs leading-5 text-white whitespace-pre-wrap">{modelOutput}</pre>
      {/if}
      <div class="mt-4 divide-y divide-line text-sm">
        {#each status?.modelStatuses ?? [] as item}
          <div class="py-2">
            <div class="flex justify-between gap-3">
              <span>{modelRoleLabel(item.role)}</span>
              <span class={`min-w-0 break-all text-right truncate ${item.installed ? 'text-pine' : 'text-ember'}`}>{item.name}</span>
            </div>
            {#if item.profile}
              <div class={`mt-1 text-xs ${item.profileValid ? 'text-slate-500' : 'text-ember'}`}>
                Profile: {item.profile} ({item.profileStatus || (item.profileValid ? 'applied' : 'missing')})
              </div>
            {/if}
          </div>
        {/each}
      </div>
    </div>
  </div>

  <div class="mt-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_420px]">
    <div class="rounded-md border border-line bg-white">
      <div class="border-b border-line px-4 py-3">
        <SectionHeader
          compact
          icon="settings"
          title="Model Profiles"
          description="Inspected local behavior artifacts for already-installed models."
          meta={`${modelProfiles.length} profiles`}
        >
          <svelte:fragment slot="actions">
            <ActionButton variant="secondary" icon="retry" disabled={modelProfileBusy} onclick={refreshModelProfiles}>
              Refresh
            </ActionButton>
          </svelte:fragment>
        </SectionHeader>
      </div>
      <div class="divide-y divide-line">
        {#if (modelProfiles ?? []).length === 0}
          <div class="p-4">
            <EmptyState
              icon="settings"
              title="No model profiles yet"
              message="Create a profile to preview a local Modelfile artifact without training, downloading, or switching models."
            />
          </div>
        {/if}
        {#each modelProfiles ?? [] as profile}
          <button
            class="flex w-full items-start justify-between gap-3 px-4 py-3 text-left text-sm soft-hover"
            type="button"
            disabled={modelProfileBusy}
            onclick={() => inspectModelProfile(profile.name)}
          >
            <span class="min-w-0">
              <span class="flex items-center gap-2 font-medium"><Icon name="settings" size={16} /> {profile.name}</span>
              <span class="mt-1 block text-xs text-slate-500">{profile.description || profile.metadata?.purpose || 'Local behavior profile'}</span>
              {#if (profile.tags ?? []).length > 0}
                <span class="mt-2 flex flex-wrap gap-1">
                  {#each profile.tags ?? [] as tag}
                    <span class="rounded border border-line bg-slate-50 px-2 py-0.5 text-[11px] text-slate-600">{tag}</span>
                  {/each}
                </span>
              {/if}
            </span>
            <span class="shrink-0 max-w-[11rem] truncate text-right text-xs text-slate-500">{profile.baseModel}</span>
          </button>
        {/each}
      </div>
    </div>

    <div class="rounded-md border border-line bg-white p-4">
      <SectionHeader
        compact
        icon="settings"
        title="Create Profile"
        description="Preview and save a local profile artifact. It will not train, download, or switch models."
        className="mb-3"
      />
      <div class="mb-3 rounded-md border border-line bg-slate-50 p-3">
        <div class="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Reviewed conversation draft</div>
        <div class="grid gap-2">
          <input class="h-9 w-full rounded-md border border-line bg-white px-2 text-sm" bind:value={modelProfileConversationId} placeholder="Conversation id to draft from" />
          <ActionButton
            variant="secondary"
            icon="spark"
            disabled={modelProfileBusy || !modelProfileConversationId.trim()}
            disabledReason={modelProfileBusy ? 'A model profile action is already running.' : 'Enter a conversation id before drafting.'}
            onclick={draftModelProfileFromConversation}
          >
            Draft From Conversation
          </ActionButton>
        </div>
        <div class="mt-2 text-xs leading-5 text-slate-500">
          Drafts use reviewed traits and metadata only. Raw transcript text is not copied.
        </div>
      </div>
      <div class="grid gap-2">
        <input class="h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelProfileName} placeholder="Profile name" />
        <input class="h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelProfileDescription} placeholder="Short description" />
        <input class="h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelProfileBaseModel} placeholder="Base local model, e.g. qwen2.5:3b" />
        <div class="grid grid-cols-2 gap-2">
          <input class="h-9 min-w-0 rounded-md border border-line bg-field px-2 text-sm" type="number" step="0.1" min="0" max="2" bind:value={modelProfileTemperature} />
          <input class="h-9 min-w-0 rounded-md border border-line bg-field px-2 text-sm" type="number" step="512" min="512" max="32768" bind:value={modelProfileNumCtx} />
        </div>
        <input class="h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelProfilePurpose} placeholder="Purpose" />
        <input class="h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelProfileRefinedFrom} placeholder="Refined from" />
        <input class="h-9 w-full rounded-md border border-line bg-field px-2 text-sm" bind:value={modelProfileTags} placeholder="Tags, comma-separated" />
        <textarea class="h-28 w-full resize-none rounded-md border border-line bg-field px-2 py-2 text-sm" bind:value={modelProfileSystem} placeholder="System behavior for this local profile"></textarea>
      </div>
      <div class="mt-3 flex flex-wrap gap-2">
        <ActionButton
          variant="secondary"
          icon="eye"
          disabled={modelProfileBusy || !modelProfileName.trim() || !modelProfileBaseModel.trim()}
          disabledReason={modelProfileBusy ? 'A model profile action is already running.' : 'Profile name and base model are required.'}
          onclick={previewModelProfile}
        >
          Preview
        </ActionButton>
        <ActionButton
          variant="primary"
          icon="check"
          disabled={modelProfileBusy || !modelProfileName.trim() || !modelProfileBaseModel.trim()}
          disabledReason={modelProfileBusy ? 'A model profile action is already running.' : 'Profile name and base model are required.'}
          onclick={saveModelProfile}
        >
          Save Profile
        </ActionButton>
        <ActionButton
          variant="secondary"
          icon="settings"
          disabled={modelProfileBusy || !modelProfileName.trim() || !modelProfileBaseModel.trim() || !modelRole.trim()}
          disabledReason={modelProfileBusy ? 'A model profile action is already running.' : 'Choose a saved profile and model role before applying.'}
          onclick={applyModelProfile}
        >
          Apply to {modelRoleLabel(modelRole)}
        </ActionButton>
      </div>

      {#if modelProfileResult}
        <div class="mt-4 rounded-md border border-line bg-slate-50 p-3 text-xs text-slate-700">
          <div class="mb-1 font-semibold">{modelProfileResult.profile.name}</div>
          <div>Base model: {modelProfileResult.profile.baseModel}</div>
          <div>Temperature: {modelProfileResult.profile.parameters?.temperature ?? 'default'}</div>
          <div>Context: {modelProfileResult.profile.parameters?.numCtx ?? 'default'}</div>
          {#if modelProfileResult.readiness}
            <div class="mt-3 rounded border border-line bg-white p-2">
              <div class="flex items-center justify-between gap-2">
                <div class="font-semibold">Readiness: {modelProfileResult.readiness.status}</div>
                <div class="rounded border border-line bg-slate-50 px-2 py-0.5 text-[11px] text-slate-600">{scoreLabel(modelProfileResult.readiness.score, modelProfileResult.readiness.max)}</div>
              </div>
              {#if (modelProfileResult.readiness.checks ?? []).length > 0}
                <div class="mt-2 grid gap-1">
                  {#each modelProfileResult.readiness.checks ?? [] as check}
                    <div class="flex items-start justify-between gap-2">
                      <span>{check.message}</span>
                      <span class="shrink-0 text-slate-500">{check.points}/{check.max}</span>
                    </div>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
          {#if modelProfileResult.comparison}
            <div class="mt-3 rounded border border-line bg-white p-2">
              <div class="font-semibold">
                {modelProfileResult.comparison.hasBaseline ? 'Comparison with saved profile' : 'Comparison: new profile'}
              </div>
              <div class="mt-1 text-slate-500">
                {modelProfileResult.comparison.sameArtifact ? 'No saved artifact changes detected.' : `${(modelProfileResult.comparison.changedFields ?? []).length} changed field(s)`}
                {#if modelProfileResult.comparison.readinessScoreDelta}
                  · readiness {modelProfileResult.comparison.readinessScoreDelta > 0 ? '+' : ''}{modelProfileResult.comparison.readinessScoreDelta}
                {/if}
              </div>
              {#if (modelProfileResult.comparison.changedFields ?? []).length > 0}
                <div class="mt-2 flex flex-wrap gap-1">
                  {#each modelProfileResult.comparison.changedFields ?? [] as field}
                    <span class="rounded border border-line bg-slate-50 px-2 py-0.5 text-[11px] text-slate-600">{field}</span>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
          {#if modelProfileResult.path}
            <div class="mt-1 break-all">Saved path: {modelProfileResult.path}</div>
          {/if}
          {#if (modelProfileResult.history ?? []).length > 0}
            <div class="mt-3 rounded border border-line bg-white p-2">
              <div class="font-semibold">History</div>
              {#each modelProfileResult.history ?? [] as event}
                <div class="mt-1 break-all text-slate-500">{event.message || event.kind}: {event.modifiedAt || 'local artifact'} · readiness {event.readinessScore}</div>
              {/each}
            </div>
          {/if}
          {#if modelProfileResult.draftReport}
            <div class="mt-3 rounded border border-line bg-white p-2">
              <div class="font-semibold">Draft report: {modelProfileResult.draftReport.eligibility}</div>
              <div class="mt-1">{modelProfileResult.draftReport.reason}</div>
              <div class="mt-2 grid gap-1 sm:grid-cols-2">
                <div>Messages: {modelProfileResult.draftReport.messageCount}</div>
                <div>Assistant turns: {modelProfileResult.draftReport.assistantMessages}</div>
                <div>Tool runs: {modelProfileResult.draftReport.toolRunCount}</div>
                <div>Failed tool runs: {modelProfileResult.draftReport.failedToolRuns}</div>
              </div>
              {#if (modelProfileResult.draftReport.observedTraits ?? []).length > 0}
                <div class="mt-2 flex flex-wrap gap-1">
                  {#each modelProfileResult.draftReport.observedTraits ?? [] as trait}
                    <span class="rounded border border-line bg-slate-50 px-2 py-0.5 text-[11px] text-slate-600">{trait}</span>
                  {/each}
                </div>
              {/if}
            </div>
          {/if}
          {#if (modelProfileResult.safetySummary ?? []).length > 0}
            <div class="mt-3 flex flex-wrap gap-1">
              {#each modelProfileResult.safetySummary ?? [] as item}
                <span class="rounded border border-line bg-white px-2 py-0.5 text-[11px] text-slate-600">{item}</span>
              {/each}
            </div>
          {/if}
        </div>
        {#if modelProfileResult.modelfile}
          <pre class="mt-3 max-h-64 overflow-y-auto rounded-md border border-line bg-ink p-3 text-xs leading-5 text-white whitespace-pre-wrap">{modelProfileResult.modelfile}</pre>
        {/if}
      {/if}
    </div>
  </div>
</div>
