import type {
  CloudFallbackConfig,
  ConnectorsConfig,
  EmbeddingsConfig,
  FlagConfig,
  InternetConfig,
  InternetSearchConfig,
  KnowledgeConfig,
  RAGConfig,
  SettingsView,
  TokenConnectorConfig,
  UIConfig
} from './appTypes';
import { internetSearchProviderDefaultAPIKeyEnv, internetSearchProviderOption } from './appOptions';
import { normalizedInternetSearchAPIKeyEnv, normalizedInternetSearchProvider } from './internetSettingsHelpers';
import type { SettingsSaveInput } from './settingsActions';
import { normalizeTheme } from './themeHelpers';
import { field } from './uiHelpers';

export type SettingsFormState = {
  lowMemory: boolean;
  contextTokens: number;
  responseMode: string;
  showThinkingTrace: boolean;
  ragEnabled: boolean;
  embeddingsEnabled: boolean;
  embeddingModel: string;
  theme: string;
  cloudEnabled: boolean;
  cloudBaseURL: string;
  cloudModel: string;
  cloudKeyEnv: string;
  connectorEnabled: boolean;
  connectorTokenEnv: string;
  mcpConnectorEnabled: boolean;
  slackConnectorEnabled: boolean;
  slackConnectorTokenEnv: string;
  discordConnectorEnabled: boolean;
  discordConnectorTokenEnv: string;
  telegramConnectorEnabled: boolean;
  telegramConnectorTokenEnv: string;
  emailConnectorEnabled: boolean;
  emailConnectorTokenEnv: string;
  internetEnabled: boolean;
  internetSearchEnabled: boolean;
  internetSearchProvider: string;
  internetSearchProviderSnapshot: string;
  internetSearchEndpoint: string;
  internetSearchAPIKeyEnv: string;
  knowledgeInfluenceEnabled: boolean;
};

export function defaultSettingsForm(): SettingsFormState {
  return {
    lowMemory: true,
    contextTokens: 4096,
    responseMode: 'balanced',
    showThinkingTrace: false,
    ragEnabled: true,
    embeddingsEnabled: false,
    embeddingModel: '',
    theme: 'system',
    cloudEnabled: false,
    cloudBaseURL: '',
    cloudModel: '',
    cloudKeyEnv: '',
    connectorEnabled: false,
    connectorTokenEnv: '',
    mcpConnectorEnabled: false,
    slackConnectorEnabled: false,
    slackConnectorTokenEnv: '',
    discordConnectorEnabled: false,
    discordConnectorTokenEnv: '',
    telegramConnectorEnabled: false,
    telegramConnectorTokenEnv: '',
    emailConnectorEnabled: false,
    emailConnectorTokenEnv: '',
    internetEnabled: false,
    internetSearchEnabled: false,
    internetSearchProvider: 'none',
    internetSearchProviderSnapshot: 'none',
    internetSearchEndpoint: '',
    internetSearchAPIKeyEnv: '',
    knowledgeInfluenceEnabled: false
  };
}

export function settingsFormsEqual(left: SettingsFormState, right: SettingsFormState) {
  return (
    left.lowMemory === right.lowMemory &&
    left.contextTokens === right.contextTokens &&
    left.responseMode === right.responseMode &&
    left.showThinkingTrace === right.showThinkingTrace &&
    left.ragEnabled === right.ragEnabled &&
    left.embeddingsEnabled === right.embeddingsEnabled &&
    left.embeddingModel === right.embeddingModel &&
    left.theme === right.theme &&
    left.cloudEnabled === right.cloudEnabled &&
    left.cloudBaseURL === right.cloudBaseURL &&
    left.cloudModel === right.cloudModel &&
    left.cloudKeyEnv === right.cloudKeyEnv &&
    left.connectorEnabled === right.connectorEnabled &&
    left.connectorTokenEnv === right.connectorTokenEnv &&
    left.mcpConnectorEnabled === right.mcpConnectorEnabled &&
    left.slackConnectorEnabled === right.slackConnectorEnabled &&
    left.slackConnectorTokenEnv === right.slackConnectorTokenEnv &&
    left.discordConnectorEnabled === right.discordConnectorEnabled &&
    left.discordConnectorTokenEnv === right.discordConnectorTokenEnv &&
    left.telegramConnectorEnabled === right.telegramConnectorEnabled &&
    left.telegramConnectorTokenEnv === right.telegramConnectorTokenEnv &&
    left.emailConnectorEnabled === right.emailConnectorEnabled &&
    left.emailConnectorTokenEnv === right.emailConnectorTokenEnv &&
    left.internetEnabled === right.internetEnabled &&
    left.internetSearchEnabled === right.internetSearchEnabled &&
    left.internetSearchProvider === right.internetSearchProvider &&
    left.internetSearchProviderSnapshot === right.internetSearchProviderSnapshot &&
    left.internetSearchEndpoint === right.internetSearchEndpoint &&
    left.internetSearchAPIKeyEnv === right.internetSearchAPIKeyEnv &&
    left.knowledgeInfluenceEnabled === right.knowledgeInfluenceEnabled
  );
}

export function settingsFormFromView(value: SettingsView | null, current: SettingsFormState = defaultSettingsForm()) {
  if (!value) return current;
  const ui: UIConfig = value.ui ?? value.UI ?? {};
  const rag: RAGConfig = value.rag ?? {};
  const embeddings: EmbeddingsConfig = rag.embeddings ?? rag.Embeddings ?? {};
  const cloud: CloudFallbackConfig = value.cloudFallback ?? {};
  const connectors: ConnectorsConfig = value.connectors ?? {};
  const localApi: TokenConnectorConfig = connectors.local_api ?? connectors.localApi ?? connectors.LocalAPI ?? {};
  const mcpServer: FlagConfig = connectors.mcp_server ?? connectors.mcpServer ?? connectors.MCPServer ?? {};
  const slack: TokenConnectorConfig = connectors.slack ?? connectors.Slack ?? {};
  const discord: TokenConnectorConfig = connectors.discord ?? connectors.Discord ?? {};
  const telegram: TokenConnectorConfig = connectors.telegram ?? connectors.Telegram ?? {};
  const email: TokenConnectorConfig = connectors.email ?? connectors.Email ?? {};
  const internet: InternetConfig = value.internet ?? {};
  const internetSearch: InternetSearchConfig = internet.search ?? internet.Search ?? {};
  const knowledge: KnowledgeConfig = value.knowledge ?? {};
  const responseModeSupported = settingsViewSupportsResponseMode(value);
  const thinkingTraceSupported = settingsViewSupportsThinkingTrace(value);
  const next: SettingsFormState = {
    ...current,
    lowMemory: value.lowMemoryMode,
    contextTokens: value.maxContextTokens || 4096,
    responseMode: responseModeSupported ? normalizeResponseMode(field(value.responseMode, 'balanced')) : 'balanced',
    showThinkingTrace: thinkingTraceSupported ? Boolean(field(value.showThinkingTrace, false)) : false,
    theme: normalizeTheme(field(ui.theme, ui.Theme, current.theme || 'system')),
    ragEnabled: Boolean(field(rag.enabled, rag.Enabled, true)),
    embeddingsEnabled: Boolean(field(embeddings.enabled, embeddings.Enabled, false)),
    embeddingModel: field(embeddings.model, embeddings.Model, current.embeddingModel || 'nomic-embed-text'),
    cloudEnabled: Boolean(field(cloud.enabled, cloud.Enabled, false)),
    cloudBaseURL: field(cloud.base_url, cloud.baseUrl, cloud.baseURL, cloud.BaseURL, current.cloudBaseURL || 'http://localhost:4000/v1'),
    cloudModel: field(cloud.name, cloud.Name, current.cloudModel),
    cloudKeyEnv: field(cloud.api_key_env, cloud.apiKeyEnv, cloud.APIKeyEnv, current.cloudKeyEnv),
    connectorEnabled: Boolean(field(connectors.enabled, connectors.Enabled, false) && field(localApi.enabled, localApi.Enabled, false)),
    connectorTokenEnv: field(localApi.token_env, localApi.tokenEnv, localApi.TokenEnv, current.connectorTokenEnv || 'YEMAKA_CONNECTOR_TOKEN'),
    mcpConnectorEnabled: Boolean(field(connectors.enabled, connectors.Enabled, false) && field(mcpServer.enabled, mcpServer.Enabled, false)),
    slackConnectorEnabled: Boolean(field(connectors.enabled, connectors.Enabled, false) && field(slack.enabled, slack.Enabled, false)),
    slackConnectorTokenEnv: field(slack.token_env, slack.tokenEnv, slack.TokenEnv, current.slackConnectorTokenEnv || 'YEMAKA_SLACK_CONNECTOR_TOKEN'),
    discordConnectorEnabled: Boolean(field(connectors.enabled, connectors.Enabled, false) && field(discord.enabled, discord.Enabled, false)),
    discordConnectorTokenEnv: field(discord.token_env, discord.tokenEnv, discord.TokenEnv, current.discordConnectorTokenEnv || 'YEMAKA_DISCORD_CONNECTOR_TOKEN'),
    telegramConnectorEnabled: Boolean(field(connectors.enabled, connectors.Enabled, false) && field(telegram.enabled, telegram.Enabled, false)),
    telegramConnectorTokenEnv: field(telegram.token_env, telegram.tokenEnv, telegram.TokenEnv, current.telegramConnectorTokenEnv || 'YEMAKA_TELEGRAM_CONNECTOR_TOKEN'),
    emailConnectorEnabled: Boolean(field(connectors.enabled, connectors.Enabled, false) && field(email.enabled, email.Enabled, false)),
    emailConnectorTokenEnv: field(email.token_env, email.tokenEnv, email.TokenEnv, current.emailConnectorTokenEnv || 'YEMAKA_EMAIL_CONNECTOR_TOKEN'),
    internetEnabled: Boolean(field(internet.enabled, internet.Enabled, false)),
    internetSearchEnabled: Boolean(field(internetSearch.enabled, internetSearch.Enabled, false)),
    internetSearchProvider: field(internetSearch.provider, internetSearch.Provider, current.internetSearchProvider || 'none') || 'none',
    internetSearchEndpoint: field(internetSearch.endpoint, internetSearch.Endpoint, current.internetSearchEndpoint),
    internetSearchAPIKeyEnv: field(internetSearch.api_key_env, internetSearch.apiKeyEnv, internetSearch.APIKeyEnv, current.internetSearchAPIKeyEnv),
    knowledgeInfluenceEnabled: Boolean(field(knowledge.influence_enabled, knowledge.influenceEnabled, knowledge.InfluenceEnabled, false))
  };
  next.internetSearchProviderSnapshot = next.internetSearchProvider;
  return normalizeInternetSearchProviderForm(next, next.internetSearchProvider, false);
}

export function normalizeInternetSearchProviderForm(form: SettingsFormState, nextProvider = form.internetSearchProvider, resetForProviderSwitch = true): SettingsFormState {
  const previousProvider = form.internetSearchProviderSnapshot || form.internetSearchProvider;
  const option = internetSearchProviderOption(String(nextProvider || 'none'));
  if (option.disabled) {
    return {
      ...form,
      internetSearchProvider: 'none',
      internetSearchProviderSnapshot: 'none',
      internetSearchEndpoint: '',
      internetSearchAPIKeyEnv: ''
    };
  }
  const providerChanged = previousProvider !== option.id;
  if (resetForProviderSwitch && providerChanged) {
    return {
      ...form,
      internetSearchProvider: option.id,
      internetSearchProviderSnapshot: option.id,
      internetSearchEndpoint: '',
      internetSearchAPIKeyEnv: option.apiKeyEnv ? internetSearchProviderDefaultAPIKeyEnv(option.id) : ''
    };
  }
  return {
    ...form,
    internetSearchProvider: option.id,
    internetSearchProviderSnapshot: option.id,
    internetSearchEndpoint: option.endpointMode === 'none' || option.endpointMode === 'unsupported' ? '' : form.internetSearchEndpoint,
    internetSearchAPIKeyEnv: option.apiKeyEnv ? normalizedInternetSearchAPIKeyEnv(option.id, form.internetSearchAPIKeyEnv) : ''
  };
}

export function normalizeResponseMode(mode: string | undefined) {
  const normalized = String(mode || '').trim().toLowerCase();
  if (normalized === 'auto' || normalized === 'fast' || normalized === 'deep') return normalized;
  return 'balanced';
}

export function settingsViewSupportsResponseMode(value: SettingsView | null | undefined) {
  if (!value) return true;
  return Boolean(value.featureSupport?.responseMode ?? Object.prototype.hasOwnProperty.call(value, 'responseMode'));
}

export function settingsViewSupportsThinkingTrace(value: SettingsView | null | undefined) {
  if (!value) return true;
  return Boolean(value.featureSupport?.showThinkingTrace ?? Object.prototype.hasOwnProperty.call(value, 'showThinkingTrace'));
}

export function settingsSaveInputFromForm(form: SettingsFormState): SettingsSaveInput {
  return {
    lowMemoryMode: form.lowMemory,
    maxContextTokens: Number(form.contextTokens),
    responseMode: normalizeResponseMode(form.responseMode),
    showThinkingTrace: Boolean(form.showThinkingTrace),
    uiTheme: form.theme,
    ragEnabled: form.ragEnabled,
    embeddingsEnabled: form.embeddingsEnabled,
    embeddingModel: form.embeddingModel,
    cloudFallbackEnabled: form.cloudEnabled,
    cloudFallbackBaseUrl: form.cloudBaseURL,
    cloudFallbackModel: form.cloudModel,
    cloudFallbackKeyEnv: form.cloudKeyEnv,
    connectorEnabled: form.connectorEnabled,
    connectorTokenEnv: form.connectorTokenEnv,
    mcpConnectorEnabled: form.mcpConnectorEnabled,
    slackConnectorEnabled: form.slackConnectorEnabled,
    slackConnectorTokenEnv: form.slackConnectorTokenEnv,
    discordConnectorEnabled: form.discordConnectorEnabled,
    discordConnectorTokenEnv: form.discordConnectorTokenEnv,
    telegramConnectorEnabled: form.telegramConnectorEnabled,
    telegramConnectorTokenEnv: form.telegramConnectorTokenEnv,
    emailConnectorEnabled: form.emailConnectorEnabled,
    emailConnectorTokenEnv: form.emailConnectorTokenEnv,
    internetEnabled: form.internetEnabled,
    internetSearchEnabled: form.internetEnabled && form.internetSearchEnabled,
    internetSearchProvider: normalizedInternetSearchProvider(form.internetSearchProvider),
    internetSearchEndpoint: form.internetSearchEndpoint,
    internetSearchApiKeyEnv: normalizedInternetSearchAPIKeyEnv(form.internetSearchProvider, form.internetSearchAPIKeyEnv),
    knowledgeInfluenceEnabled: form.knowledgeInfluenceEnabled
  };
}
