<script lang="ts">
  import BitSelect from '../BitSelect.svelte';
  import ActionButton from '../ActionButton.svelte';
  import Badge from '../Badge.svelte';
  import EmptyState from '../EmptyState.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import type { GeneratedConnectorEntry, InternetStatus } from '../lib/appTypes';
  import {
    internetSearchAPIKeyEnvHelp as internetSearchAPIKeyEnvHelpFor,
    internetSearchAPIKeyEnvValidationMessage as internetSearchAPIKeyEnvValidationMessageFor,
    internetSearchProviderAPIKeyDisabled as internetSearchProviderAPIKeyDisabledFor,
    internetSearchProviderAPIKeyPlaceholder as internetSearchProviderAPIKeyPlaceholderFor,
    internetSearchProviderEndpointDisabled as internetSearchProviderEndpointDisabledFor,
    internetSearchProviderEndpointPlaceholder as internetSearchProviderEndpointPlaceholderFor
  } from '../lib/internetSettingsHelpers';

  type Status = {
    configPath: string;
    profilePath: string;
    sqlitePath: string;
    setupComplete: boolean;
    modelReady: boolean;
  };

  type SettingsView = {
    settingsSchemaVersion?: number;
    featureSupport?: {
      responseMode?: boolean;
      showThinkingTrace?: boolean;
    };
    responseMode?: string;
    showThinkingTrace?: boolean;
    shellEnabled: boolean;
    telemetry: boolean;
    knowledge?: {
      enabled?: boolean;
      Enabled?: boolean;
      influenceEnabled?: boolean;
      InfluenceEnabled?: boolean;
    };
  };

  type InternetSearchProviderOption = {
    id: string;
    label: string;
    note: string;
  };

  type SelectOption = {
    value: string;
    label: string;
    disabled?: boolean;
    meta?: string;
  };

  type ConnectorRegistryEntry = {
    status: {
      name: string;
      bind: string;
      tokenReady: boolean;
      secret: { provider: string; name: string };
      allowedDomains: string[];
      permissions: {
        scopes: string[];
      };
      rateLimit: { requestsPerMinute: number };
      health: { status: string };
    };
  };

  export let status: Status | null = null;
  export let settings: SettingsView | null = null;
  export let connectorRegistry: ConnectorRegistryEntry[] = [];
  export let generatedConnectors: GeneratedConnectorEntry[] = [];
  export let internetStatus: InternetStatus | null = null;
  export let settingTheme = 'system';
  export let settingLowMemory = true;
  export let settingContextTokens = 4096;
  export let settingResponseMode = 'balanced';
  export let settingShowThinkingTrace = false;
  export let settingRAGEnabled = true;
  export let settingEmbeddingsEnabled = false;
  export let settingEmbeddingModel = '';
  export let embeddingIndexBusy = false;
  export let settingInternetEnabled = false;
  export let settingInternetSearchEnabled = false;
  export let settingInternetSearchProvider = 'none';
  export let settingInternetSearchEndpoint = '';
  export let settingInternetSearchAPIKeyEnv = '';
  export let settingKnowledgeInfluenceEnabled = false;
  export let settingCloudEnabled = false;
  export let settingCloudBaseURL = '';
  export let settingCloudModel = '';
  export let settingCloudKeyEnv = '';
  export let settingConnectorEnabled = false;
  export let settingConnectorTokenEnv = '';
  export let settingMCPConnectorEnabled = false;
  export let settingSlackConnectorEnabled = false;
  export let settingSlackConnectorTokenEnv = '';
  export let settingDiscordConnectorEnabled = false;
  export let settingDiscordConnectorTokenEnv = '';
  export let settingTelegramConnectorEnabled = false;
  export let settingTelegramConnectorTokenEnv = '';
  export let settingEmailConnectorEnabled = false;
  export let settingEmailConnectorTokenEnv = '';
  export let internetSearchProviderSelectOptions: SelectOption[] = [];
  export let applyTheme: (theme: string) => void = () => {};
  export let indexEmbeddings: () => Promise<void> | void = () => {};
  export let saveSettings: () => Promise<void> | void = () => {};
  export let internetSearchProviderOption: (provider: string | undefined) => InternetSearchProviderOption = (provider) => ({
    id: provider || 'none',
    label: provider || 'No provider',
    note: ''
  });
  export let onInternetSearchProviderChange: (value: string) => void = () => {};
  export let internetSearchReadiness: (statusValue: InternetStatus | null) => string = () => 'disabled';
  export let internetSearchProviderLabel: (statusValue: InternetStatus | null) => string = () => 'No provider';
  export let setGeneratedConnectorEnabled: (name: string, enabled: boolean) => Promise<void> | void = () => {};

  $: providerEndpointDisabled = internetSearchProviderEndpointDisabledFor(settingInternetSearchProvider);
  $: providerEndpointPlaceholder = internetSearchProviderEndpointPlaceholderFor(settingInternetSearchProvider);
  $: providerAPIKeyDisabled = internetSearchProviderAPIKeyDisabledFor(settingInternetSearchProvider);
  $: providerAPIKeyPlaceholder = internetSearchProviderAPIKeyPlaceholderFor(settingInternetSearchProvider);
  $: providerAPIKeyEnvValidationMessage = internetSearchAPIKeyEnvValidationMessageFor(
    settingInternetSearchProvider,
    settingInternetSearchAPIKeyEnv
  );
  $: providerAPIKeyEnvHelp = internetSearchAPIKeyEnvHelpFor(settingInternetSearchProvider, settingInternetSearchAPIKeyEnv);
  $: knowledgeGraphEnabled = Boolean(settings?.knowledge?.enabled ?? settings?.knowledge?.Enabled ?? false);
  $: responseModeSupported =
    !settings ||
    Boolean(settings.featureSupport?.responseMode ?? Object.prototype.hasOwnProperty.call(settings, 'responseMode'));
  $: thinkingTraceSupported =
    !settings ||
    Boolean(settings.featureSupport?.showThinkingTrace ?? Object.prototype.hasOwnProperty.call(settings, 'showThinkingTrace'));

  const responseModeOptions = [
    { id: 'auto', label: 'Auto', note: 'Yemaka chooses from the route and task type.' },
    { id: 'fast', label: 'Fast', note: 'Quicker responses with tighter generation budgets.' },
    { id: 'balanced', label: 'Balanced', note: 'Recommended default for ordinary work.' },
    { id: 'deep', label: 'Deep', note: 'Slower, better for complex coding, research, and extension work.' }
  ];

  $: responseModeNote = responseModeOptions.find((option) => option.id === settingResponseMode)?.note || responseModeOptions[2].note;

  function generatedConnectorEnableReason(connector: GeneratedConnectorEntry) {
    if (connector.enabled) return 'Runtime readiness is already enabled';
    if (!connector.installed) return 'Install this generated connector before enabling readiness';
    if (!connector.approved) return 'Review and approve this generated connector before enabling readiness';
    if ((connector.tests?.status || '').toLowerCase() !== 'passed') return 'Generated connector tests must pass first';
    return '';
  }
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  <div class="grid gap-4 lg:grid-cols-2">
    <div class="rounded-md border border-line bg-white p-4 text-sm">
      <SectionHeader
        compact
        icon="settings"
        title="Runtime"
        description="Local runtime defaults, memory profile, RAG, embeddings, and controlled internet search."
        className="mb-3"
      />
      <div class="space-y-3">
        <div>
          <div class="mb-1 text-xs text-slate-500">Theme</div>
          <div class="grid grid-cols-3 gap-1 rounded-md border border-line bg-field p-1" role="group" aria-label="Theme">
            {#each ['system', 'light', 'dark'] as theme}
              <button
                type="button"
                class={`h-9 rounded text-sm capitalize transition ${settingTheme === theme ? 'bg-white text-ink' : 'text-slate-600 hover:text-ink'}`}
                aria-pressed={settingTheme === theme}
                onclick={() => {
                  settingTheme = theme;
                  applyTheme(settingTheme);
                }}
              >
                {theme}
              </button>
            {/each}
          </div>
        </div>
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Low memory</span>
          <input type="checkbox" bind:checked={settingLowMemory} />
        </label>
        <label class="block">
          <span class="mb-1 block text-xs text-slate-500">Context tokens</span>
          <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm" type="number" min="1024" max="16384" step="512" bind:value={settingContextTokens} />
        </label>
        <div>
          <div class="mb-1 text-xs text-slate-500">Response Mode</div>
          <div class="grid grid-cols-2 gap-1 rounded-md border border-line bg-field p-1 sm:grid-cols-4" role="group" aria-label="Response Mode">
            {#each responseModeOptions as option}
              <button
                type="button"
                disabled={!responseModeSupported}
                class={`min-h-9 rounded px-2 text-sm transition ${settingResponseMode === option.id ? 'bg-white text-ink' : 'text-slate-600 hover:text-ink'}`}
                aria-pressed={settingResponseMode === option.id}
                onclick={() => {
                  settingResponseMode = option.id;
                }}
              >
                {option.label}
              </button>
            {/each}
          </div>
          <div class="mt-2 rounded-md border border-line bg-field px-3 py-2 text-xs text-slate-600">
            {#if responseModeSupported}
              {responseModeNote} Safety checks, approvals, tool lanes, and local-first defaults stay unchanged.
            {:else}
              This web server has not loaded the Response Mode backend yet. Rebuild or restart the web server, then save again.
            {/if}
          </div>
        </div>
        <label class={`flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2 ${thinkingTraceSupported ? '' : 'opacity-60'}`}>
          <span>
            <span class="block">Show thinking trace</span>
            <span class="block text-xs text-slate-500">
              {thinkingTraceSupported
                ? 'Off by default. Chat still hides raw model thinking; traces record only safe metadata.'
                : 'Requires the current Response Mode backend.'}
            </span>
          </span>
          <input type="checkbox" bind:checked={settingShowThinkingTrace} disabled={!thinkingTraceSupported} />
        </label>
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>RAG</span>
          <input type="checkbox" bind:checked={settingRAGEnabled} />
        </label>
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>
            <span class="block">Knowledge graph influence</span>
            <span class="block text-xs text-slate-500">Approved graph records can help ordinary chat as background context.</span>
          </span>
          <input type="checkbox" bind:checked={settingKnowledgeInfluenceEnabled} disabled={!knowledgeGraphEnabled} />
        </label>
        {#if !knowledgeGraphEnabled}
          <div class="rounded-md border border-line bg-field px-3 py-2 text-xs text-slate-600">
            Enable the manual knowledge graph on Memory before allowing approved records to influence chat.
          </div>
        {/if}
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Embeddings</span>
          <input type="checkbox" bind:checked={settingEmbeddingsEnabled} />
        </label>
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingEmbeddingModel} disabled={!settingEmbeddingsEnabled} placeholder="installed embedding model" />
        <div class="flex flex-wrap items-center gap-2">
          <ActionButton
            variant="secondary"
            icon="documents"
            onclick={indexEmbeddings}
            disabled={!settingEmbeddingsEnabled || embeddingIndexBusy}
            disabledReason={settingEmbeddingsEnabled ? 'Embedding indexing is already running.' : 'Enable embeddings before indexing.'}
            busy={embeddingIndexBusy}
            busyLabel="Indexing"
            size="sm"
          >
            Index Embeddings
          </ActionButton>
        </div>
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Internet access</span>
          <input type="checkbox" bind:checked={settingInternetEnabled} />
        </label>
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Internet search</span>
          <input type="checkbox" bind:checked={settingInternetSearchEnabled} disabled={!settingInternetEnabled} />
        </label>
        <BitSelect
          className="h-10 w-full"
          contentClass="w-[min(520px,calc(100vw-2rem))]"
          bind:value={settingInternetSearchProvider}
          options={internetSearchProviderSelectOptions}
          onChange={onInternetSearchProviderChange}
          placeholder="Select search provider"
        />
        <div class="rounded-md border border-line bg-field px-3 py-2 text-xs text-slate-600">
          <div class="mb-0.5 font-medium text-slate-700">{internetSearchProviderOption(settingInternetSearchProvider).label}</div>
          <div>{internetSearchProviderOption(settingInternetSearchProvider).note}</div>
        </div>
        <input
          class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50"
          bind:value={settingInternetSearchEndpoint}
          disabled={providerEndpointDisabled}
          placeholder={providerEndpointPlaceholder}
        />
        <input
          class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50"
          bind:value={settingInternetSearchAPIKeyEnv}
          disabled={providerAPIKeyDisabled}
          placeholder={providerAPIKeyPlaceholder}
          autocomplete="off"
          spellcheck="false"
        />
        {#if !providerAPIKeyDisabled}
          <div class={`rounded-md border px-3 py-2 text-xs ${providerAPIKeyEnvValidationMessage ? 'border-amber-200 bg-amber-50 text-amber-900' : 'border-line bg-field text-slate-600'}`}>
            {providerAPIKeyEnvValidationMessage || providerAPIKeyEnvHelp}
          </div>
        {/if}
        <div class="grid grid-cols-2 gap-y-2 pt-1 text-xs text-slate-600">
          <span>Setup</span><span>{status?.setupComplete ? 'complete' : 'not complete'}</span>
          <span>Model ready</span><span>{status?.modelReady ? 'yes' : 'no'}</span>
          <span>Shell</span><span>{settings?.shellEnabled ? 'safe allowlist' : 'disabled'}</span>
          <span>Internet</span><span>{settingInternetEnabled ? 'profile enabled' : 'ask/off'}</span>
          <span>Search</span><span>{settingInternetSearchEnabled ? internetSearchProviderOption(settingInternetSearchProvider).label : 'off'}</span>
          <span>Search ready</span>
          <span>
            <Badge variant={internetSearchReadiness(internetStatus) === 'healthy' ? 'success' : internetSearchReadiness(internetStatus) === 'disabled' ? 'neutral' : 'warning'}>
              {internetSearchReadiness(internetStatus)}
            </Badge>
          </span>
          <span>Search provider</span><span class="break-all">{internetSearchProviderLabel(internetStatus)}</span>
          <span>Search health</span><span>{internetStatus?.searchHealthScore ?? 0}/100</span>
          <span>Search status</span><span class="break-words">{internetStatus?.searchStatus || 'not checked'}</span>
          <span>Next action</span><span class="break-words">{internetStatus?.searchNextAction || 'none'}</span>
          <span>Theme</span><span>{settingTheme}</span>
          <span>Response mode</span><span>{settingResponseMode}</span>
          <span>Telemetry</span><span>{settings?.telemetry ? 'enabled' : 'disabled'}</span>
        </div>
      </div>
    </div>
    <div class="rounded-md border border-line bg-white p-4 text-sm">
      <SectionHeader
        compact
        icon="settings"
        title="Optional Paths"
        description="Cloud fallback and connectors stay disabled until explicitly configured."
        className="mb-3"
      />
      <div class="space-y-3">
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Cloud fallback</span>
          <input type="checkbox" bind:checked={settingCloudEnabled} />
        </label>
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingCloudBaseURL} disabled={!settingCloudEnabled} placeholder="http://localhost:4000/v1" />
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingCloudModel} disabled={!settingCloudEnabled} placeholder="fallback model" />
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingCloudKeyEnv} disabled={!settingCloudEnabled} placeholder="API key env name" />
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Local connector</span>
          <input type="checkbox" bind:checked={settingConnectorEnabled} />
        </label>
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingConnectorTokenEnv} disabled={!settingConnectorEnabled} placeholder="YEMAKA_CONNECTOR_TOKEN" />
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>MCP stdio adapter</span>
          <input type="checkbox" bind:checked={settingMCPConnectorEnabled} />
        </label>
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Slack webhook</span>
          <input type="checkbox" bind:checked={settingSlackConnectorEnabled} />
        </label>
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingSlackConnectorTokenEnv} disabled={!settingSlackConnectorEnabled} placeholder="YEMAKA_SLACK_CONNECTOR_TOKEN" />
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Discord webhook</span>
          <input type="checkbox" bind:checked={settingDiscordConnectorEnabled} />
        </label>
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingDiscordConnectorTokenEnv} disabled={!settingDiscordConnectorEnabled} placeholder="YEMAKA_DISCORD_CONNECTOR_TOKEN" />
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Telegram webhook</span>
          <input type="checkbox" bind:checked={settingTelegramConnectorEnabled} />
        </label>
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingTelegramConnectorTokenEnv} disabled={!settingTelegramConnectorEnabled} placeholder="YEMAKA_TELEGRAM_CONNECTOR_TOKEN" />
        <label class="flex items-center justify-between gap-3 rounded-md border border-line px-3 py-2">
          <span>Email webhook</span>
          <input type="checkbox" bind:checked={settingEmailConnectorEnabled} />
        </label>
        <input class="h-10 w-full rounded-md border border-line bg-field px-3 text-sm disabled:opacity-50" bind:value={settingEmailConnectorTokenEnv} disabled={!settingEmailConnectorEnabled} placeholder="YEMAKA_EMAIL_CONNECTOR_TOKEN" />
        <div class="flex justify-end">
          <ActionButton variant="primary" icon="check" onclick={saveSettings}>Save Settings</ActionButton>
        </div>
      </div>
    </div>
    <div class="rounded-md border border-line bg-white p-4 text-sm lg:col-span-2">
      <SectionHeader
        compact
        icon="settings"
        title="Connector Registry"
        description="Registered connectors with health, scopes, rate limits, and secret references."
        meta={`${connectorRegistry.length} connectors`}
        className="mb-3"
      />
      <div class="grid gap-3 lg:grid-cols-2">
        {#if (connectorRegistry ?? []).length === 0}
          <div class="lg:col-span-2">
            <EmptyState
              icon="settings"
              title="No connectors registered"
              message="Optional connectors stay disabled until configured; registered connectors will appear here with health, scopes, and secret references."
            />
          </div>
        {/if}
        {#each connectorRegistry ?? [] as entry}
          <div class="rounded-md border border-line bg-field p-3">
            <div class="mb-2 flex items-center justify-between gap-3">
              <div class="min-w-0 break-all font-medium">{entry.status.name}</div>
              <Badge variant={entry.status.health.status === 'healthy' ? 'success' : entry.status.health.status === 'broken' ? 'danger' : entry.status.health.status === 'disabled' ? 'neutral' : 'warning'}>
                {entry.status.health.status}
              </Badge>
            </div>
            <div class="grid grid-cols-[90px_minmax(0,1fr)] gap-y-1 text-xs text-slate-600">
              <span>Bind</span><span>{entry.status.bind}</span>
              <span>Secret</span><span class="break-all">{entry.status.secret.provider}:{entry.status.secret.name || 'none'}</span>
              <span>Ready</span><span>{entry.status.tokenReady ? 'yes' : 'no'}</span>
              <span>Rate</span><span>{entry.status.rateLimit.requestsPerMinute || 0}/min</span>
              <span>Scopes</span><span class="break-words">{entry.status.permissions.scopes?.join(', ') || 'none'}</span>
            </div>
            {#if entry.status.allowedDomains?.length}
              <div class="mt-2 break-all text-xs text-slate-500">{entry.status.allowedDomains.join(', ')}</div>
            {/if}
          </div>
        {/each}
      </div>
      <div class="mt-4 border-t border-line pt-4">
        <SectionHeader
          compact
          icon="settings"
          title="Generated Connector Candidates"
          description="Profile-local connector manifests installed from reviewed artifacts. Readiness can be enabled after tests, approval, policy, and secrets pass; execution remains future."
          meta={`${generatedConnectors.length} generated`}
          className="mb-3"
        />
        {#if (generatedConnectors ?? []).length === 0}
          <EmptyState
            icon="settings"
            title="No generated connectors"
            message="Generated connectors will appear here after reviewed manifests and passing tests are installed."
          />
        {:else}
          <div class="grid gap-3 lg:grid-cols-2">
            {#each generatedConnectors ?? [] as connector}
              <div class="rounded-md border border-line bg-field p-3">
                <div class="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div class="min-w-0 break-all font-medium">{connector.name}</div>
                  <div class="flex flex-wrap items-center gap-2">
                    <Badge variant={connector.enabled ? 'success' : 'neutral'}>{connector.enabled ? 'Enabled' : 'Disabled'}</Badge>
                    <Badge variant={connector.tests?.status === 'passed' ? 'success' : 'warning'}>{connector.tests?.status || 'untested'}</Badge>
                  </div>
                </div>
                <div class="text-sm text-slate-600">{connector.manifest?.description || 'Generated connector manifest'}</div>
                <div class="mt-2 grid grid-cols-[110px_minmax(0,1fr)] gap-y-1 text-xs text-slate-600">
                  <span>Kind</span><span>{connector.manifest?.kind || 'connector'}</span>
                  <span>Endpoint</span><span class="break-all">{connector.manifest?.endpoint || 'not declared'}</span>
                  <span>Domains</span><span class="break-all">{connector.manifest?.allowedDomains?.join(', ') || 'none'}</span>
                  <span>Installed</span><span>{connector.installedAt || connector.updatedAt || 'local profile'}</span>
                  <span>Tests</span><span>{connector.tests?.summary || connector.tests?.commands?.join(', ') || 'passing evidence required'}</span>
                </div>
                {#if connector.validationError}
                  <div class="mt-2 rounded-md border border-danger/30 bg-danger/5 px-2 py-1 text-xs text-danger">{connector.validationError}</div>
                {/if}
                <div class="mt-3 flex justify-end gap-2">
                  <ActionButton
                    variant="secondary"
                    icon="eye"
                    disabled={connector.enabled || generatedConnectorEnableReason(connector) !== ''}
                    disabledReason={connector.enabled ? 'Runtime readiness is already enabled' : generatedConnectorEnableReason(connector)}
                    onclick={() => setGeneratedConnectorEnabled(connector.name, true)}
                  >
                    Enable readiness
                  </ActionButton>
                  <ActionButton
                    variant="secondary"
                    icon="eye-off"
                    disabled={!connector.enabled}
                    disabledReason={!connector.enabled ? 'Already disabled' : ''}
                    onclick={() => setGeneratedConnectorEnabled(connector.name, false)}
                  >
                    Disable
                  </ActionButton>
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>
    <div class="rounded-md border border-line bg-white p-4 text-sm lg:col-span-2">
      <SectionHeader
        compact
        icon="documents"
        title="Local Paths"
        description="Current profile, config, and SQLite paths for this local Yemaka profile."
        className="mb-3"
      />
      <div class="space-y-2 break-all text-slate-700">
        <div>{status?.configPath}</div>
        <div>{status?.profilePath}</div>
        <div>{status?.sqlitePath}</div>
      </div>
    </div>
  </div>
</div>
