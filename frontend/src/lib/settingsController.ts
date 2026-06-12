import type { SettingsView, SetupState } from './appTypes';
import {
  internetSearchAPIKeyEnvHelp as internetSearchAPIKeyEnvHelpFor,
  internetSearchAPIKeyEnvValidationMessage as internetSearchAPIKeyEnvValidationMessageFor,
  internetSearchProviderAPIKeyDisabled as internetSearchProviderAPIKeyDisabledFor,
  internetSearchProviderAPIKeyPlaceholder as internetSearchProviderAPIKeyPlaceholderFor,
  internetSearchProviderEndpointDisabled as internetSearchProviderEndpointDisabledFor,
  internetSearchProviderEndpointPlaceholder as internetSearchProviderEndpointPlaceholderFor,
  normalizedInternetSearchAPIKeyEnv as normalizedInternetSearchAPIKeyEnvFor
} from './internetSettingsHelpers';
import {
  completeFirstRunSetup,
  embeddingIndexSummaryText,
  indexEmbeddingChunks,
  saveRuntimeSettings
} from './settingsActions';
import {
  normalizeInternetSearchProviderForm,
  settingsFormFromView,
  settingsSaveInputFromForm,
  type SettingsFormState
} from './settingsFormHelpers';

export type SettingsControllerContext = {
  getLowMemory: () => boolean;
  setLowMemory: (value: boolean) => void;
  getContextTokens: () => number;
  setContextTokens: (value: number) => void;
  getResponseMode: () => string;
  setResponseMode: (value: string) => void;
  getShowThinkingTrace: () => boolean;
  setShowThinkingTrace: (value: boolean) => void;
  getRAGEnabled: () => boolean;
  setRAGEnabled: (value: boolean) => void;
  getEmbeddingsEnabled: () => boolean;
  setEmbeddingsEnabled: (value: boolean) => void;
  getEmbeddingModel: () => string;
  setEmbeddingModel: (value: string) => void;
  getTheme: () => string;
  setTheme: (value: string) => void;
  getCloudEnabled: () => boolean;
  setCloudEnabled: (value: boolean) => void;
  getCloudBaseURL: () => string;
  setCloudBaseURL: (value: string) => void;
  getCloudModel: () => string;
  setCloudModel: (value: string) => void;
  getCloudKeyEnv: () => string;
  setCloudKeyEnv: (value: string) => void;
  getConnectorEnabled: () => boolean;
  setConnectorEnabled: (value: boolean) => void;
  getConnectorTokenEnv: () => string;
  setConnectorTokenEnv: (value: string) => void;
  getMCPConnectorEnabled: () => boolean;
  setMCPConnectorEnabled: (value: boolean) => void;
  getSlackConnectorEnabled: () => boolean;
  setSlackConnectorEnabled: (value: boolean) => void;
  getSlackConnectorTokenEnv: () => string;
  setSlackConnectorTokenEnv: (value: string) => void;
  getDiscordConnectorEnabled: () => boolean;
  setDiscordConnectorEnabled: (value: boolean) => void;
  getDiscordConnectorTokenEnv: () => string;
  setDiscordConnectorTokenEnv: (value: string) => void;
  getTelegramConnectorEnabled: () => boolean;
  setTelegramConnectorEnabled: (value: boolean) => void;
  getTelegramConnectorTokenEnv: () => string;
  setTelegramConnectorTokenEnv: (value: string) => void;
  getEmailConnectorEnabled: () => boolean;
  setEmailConnectorEnabled: (value: boolean) => void;
  getEmailConnectorTokenEnv: () => string;
  setEmailConnectorTokenEnv: (value: string) => void;
  getInternetEnabled: () => boolean;
  setInternetEnabled: (value: boolean) => void;
  getInternetSearchEnabled: () => boolean;
  setInternetSearchEnabled: (value: boolean) => void;
  getInternetSearchProvider: () => string;
  setInternetSearchProvider: (value: string) => void;
  getInternetSearchProviderSnapshot: () => string;
  setInternetSearchProviderSnapshot: (value: string) => void;
  getInternetSearchEndpoint: () => string;
  setInternetSearchEndpoint: (value: string) => void;
  getInternetSearchAPIKeyEnv: () => string;
  setInternetSearchAPIKeyEnv: (value: string) => void;
  getKnowledgeInfluenceEnabled: () => boolean;
  setKnowledgeInfluenceEnabled: (value: boolean) => void;
  setSettings: (settings: SettingsView | null) => void;
  setSettingsSummary: (summary: string) => void;
  setEmbeddingIndexSummary: (summary: string) => void;
  setEmbeddingIndexBusy: (busy: boolean) => void;
  getSetupMode: () => string;
  setSetupMode: (value: string) => void;
  getSetupModel: () => string;
  setSetupModeDirty: (value: boolean) => void;
  setSetupBusy: (busy: boolean) => void;
  setSetupState: (state: SetupState | null) => void;
  setError: (message: string) => void;
  applyTheme: (theme: string) => void;
  refresh: () => Promise<void>;
  pushActivity: (line: string) => void;
};

export function createSettingsController(ctx: SettingsControllerContext) {
  function currentSettingsForm(): SettingsFormState {
    return {
      lowMemory: ctx.getLowMemory(),
      contextTokens: Number(ctx.getContextTokens()),
      responseMode: ctx.getResponseMode(),
      showThinkingTrace: ctx.getShowThinkingTrace(),
      ragEnabled: ctx.getRAGEnabled(),
      embeddingsEnabled: ctx.getEmbeddingsEnabled(),
      embeddingModel: ctx.getEmbeddingModel(),
      theme: ctx.getTheme(),
      cloudEnabled: ctx.getCloudEnabled(),
      cloudBaseURL: ctx.getCloudBaseURL(),
      cloudModel: ctx.getCloudModel(),
      cloudKeyEnv: ctx.getCloudKeyEnv(),
      connectorEnabled: ctx.getConnectorEnabled(),
      connectorTokenEnv: ctx.getConnectorTokenEnv(),
      mcpConnectorEnabled: ctx.getMCPConnectorEnabled(),
      slackConnectorEnabled: ctx.getSlackConnectorEnabled(),
      slackConnectorTokenEnv: ctx.getSlackConnectorTokenEnv(),
      discordConnectorEnabled: ctx.getDiscordConnectorEnabled(),
      discordConnectorTokenEnv: ctx.getDiscordConnectorTokenEnv(),
      telegramConnectorEnabled: ctx.getTelegramConnectorEnabled(),
      telegramConnectorTokenEnv: ctx.getTelegramConnectorTokenEnv(),
      emailConnectorEnabled: ctx.getEmailConnectorEnabled(),
      emailConnectorTokenEnv: ctx.getEmailConnectorTokenEnv(),
      internetEnabled: ctx.getInternetEnabled(),
      internetSearchEnabled: ctx.getInternetSearchEnabled(),
      internetSearchProvider: ctx.getInternetSearchProvider(),
      internetSearchProviderSnapshot: ctx.getInternetSearchProviderSnapshot(),
      internetSearchEndpoint: ctx.getInternetSearchEndpoint(),
      internetSearchAPIKeyEnv: ctx.getInternetSearchAPIKeyEnv(),
      knowledgeInfluenceEnabled: ctx.getKnowledgeInfluenceEnabled()
    };
  }

  function applySettingsForm(form: SettingsFormState) {
    ctx.setLowMemory(form.lowMemory);
    ctx.setContextTokens(form.contextTokens);
    ctx.setResponseMode(form.responseMode);
    ctx.setShowThinkingTrace(form.showThinkingTrace);
    ctx.setRAGEnabled(form.ragEnabled);
    ctx.setEmbeddingsEnabled(form.embeddingsEnabled);
    ctx.setEmbeddingModel(form.embeddingModel);
    ctx.setTheme(form.theme);
    ctx.setCloudEnabled(form.cloudEnabled);
    ctx.setCloudBaseURL(form.cloudBaseURL);
    ctx.setCloudModel(form.cloudModel);
    ctx.setCloudKeyEnv(form.cloudKeyEnv);
    ctx.setConnectorEnabled(form.connectorEnabled);
    ctx.setConnectorTokenEnv(form.connectorTokenEnv);
    ctx.setMCPConnectorEnabled(form.mcpConnectorEnabled);
    ctx.setSlackConnectorEnabled(form.slackConnectorEnabled);
    ctx.setSlackConnectorTokenEnv(form.slackConnectorTokenEnv);
    ctx.setDiscordConnectorEnabled(form.discordConnectorEnabled);
    ctx.setDiscordConnectorTokenEnv(form.discordConnectorTokenEnv);
    ctx.setTelegramConnectorEnabled(form.telegramConnectorEnabled);
    ctx.setTelegramConnectorTokenEnv(form.telegramConnectorTokenEnv);
    ctx.setEmailConnectorEnabled(form.emailConnectorEnabled);
    ctx.setEmailConnectorTokenEnv(form.emailConnectorTokenEnv);
    ctx.setInternetEnabled(form.internetEnabled);
    ctx.setInternetSearchEnabled(form.internetSearchEnabled);
    ctx.setInternetSearchProvider(form.internetSearchProvider);
    ctx.setInternetSearchProviderSnapshot(form.internetSearchProviderSnapshot);
    ctx.setInternetSearchEndpoint(form.internetSearchEndpoint);
    ctx.setInternetSearchAPIKeyEnv(form.internetSearchAPIKeyEnv);
    ctx.setKnowledgeInfluenceEnabled(form.knowledgeInfluenceEnabled);
  }

  function syncSettingsForm(value: SettingsView | null) {
    const form = settingsFormFromView(value, currentSettingsForm());
    applySettingsForm(form);
    ctx.applyTheme(form.theme);
  }

  function internetSearchProviderEndpointDisabled() {
    return internetSearchProviderEndpointDisabledFor(ctx.getInternetSearchProvider());
  }

  function internetSearchProviderAPIKeyDisabled() {
    return internetSearchProviderAPIKeyDisabledFor(ctx.getInternetSearchProvider());
  }

  function internetSearchProviderEndpointPlaceholder() {
    return internetSearchProviderEndpointPlaceholderFor(ctx.getInternetSearchProvider());
  }

  function internetSearchProviderAPIKeyPlaceholder() {
    return internetSearchProviderAPIKeyPlaceholderFor(ctx.getInternetSearchProvider());
  }

  function normalizedInternetSearchAPIKeyEnv() {
    return normalizedInternetSearchAPIKeyEnvFor(ctx.getInternetSearchProvider(), ctx.getInternetSearchAPIKeyEnv());
  }

  function internetSearchAPIKeyEnvValidationMessage() {
    return internetSearchAPIKeyEnvValidationMessageFor(ctx.getInternetSearchProvider(), ctx.getInternetSearchAPIKeyEnv());
  }

  function internetSearchAPIKeyEnvHelp() {
    return internetSearchAPIKeyEnvHelpFor(ctx.getInternetSearchProvider(), ctx.getInternetSearchAPIKeyEnv());
  }

  function onInternetSearchProviderChange(nextProvider = ctx.getInternetSearchProvider(), resetForProviderSwitch = true) {
    applySettingsForm(normalizeInternetSearchProviderForm(currentSettingsForm(), nextProvider, resetForProviderSwitch));
  }

  async function indexEmbeddings() {
    ctx.setError('');
    ctx.setEmbeddingIndexSummary('');
    try {
      ctx.setEmbeddingIndexBusy(true);
      const result = await indexEmbeddingChunks();
      ctx.setEmbeddingIndexSummary(embeddingIndexSummaryText(result));
      ctx.pushActivity(`embedding index: ${result.chunksEmbedded} chunks`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setEmbeddingIndexBusy(false);
    }
  }

  async function saveSettings() {
    ctx.setError('');
    ctx.setSettingsSummary('');
    const apiKeyEnvMessage = internetSearchAPIKeyEnvValidationMessage();
    if (apiKeyEnvMessage) {
      ctx.setError(apiKeyEnvMessage);
      return;
    }
    try {
      const settings = await saveRuntimeSettings(settingsSaveInputFromForm(currentSettingsForm()));
      ctx.setSettings(settings);
      syncSettingsForm(settings);
      ctx.setSettingsSummary('settings saved');
      ctx.pushActivity('settings saved');
      await ctx.refresh();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function completeSetup() {
    ctx.setSetupBusy(true);
    ctx.setError('');
    try {
      const setupState = await completeFirstRunSetup({
        mode: ctx.getSetupMode(),
        lowMemoryModel: ctx.getSetupModel(),
        defaultModel: ctx.getSetupModel()
      });
      ctx.setSetupState(setupState);
      ctx.setSetupMode(setupState.mode || ctx.getSetupMode());
      ctx.setSetupModeDirty(false);
      ctx.pushActivity(`setup saved: ${ctx.getSetupModel() || setupState.selectedModel}`);
      await ctx.refresh();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setSetupBusy(false);
    }
  }

  return {
    currentSettingsForm,
    applySettingsForm,
    syncSettingsForm,
    internetSearchProviderEndpointDisabled,
    internetSearchProviderAPIKeyDisabled,
    internetSearchProviderEndpointPlaceholder,
    internetSearchProviderAPIKeyPlaceholder,
    normalizedInternetSearchAPIKeyEnv,
    internetSearchAPIKeyEnvValidationMessage,
    internetSearchAPIKeyEnvHelp,
    onInternetSearchProviderChange,
    indexEmbeddings,
    saveSettings,
    completeSetup
  };
}
