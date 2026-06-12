import type { EmbeddingIndexSummary, SettingsView, SetupState, Status } from './appTypes';
import { call } from './api';

export type RuntimeSettingsSurface = {
  status: Status;
  setupState: SetupState;
  settings: SettingsView;
};

export type SettingsSaveInput = {
  lowMemoryMode: boolean;
  maxContextTokens: number;
  responseMode: string;
  showThinkingTrace: boolean;
  uiTheme: string;
  ragEnabled: boolean;
  embeddingsEnabled: boolean;
  embeddingModel: string;
  cloudFallbackEnabled: boolean;
  cloudFallbackBaseUrl: string;
  cloudFallbackModel: string;
  cloudFallbackKeyEnv: string;
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
  internetSearchEndpoint: string;
  internetSearchApiKeyEnv: string;
  knowledgeInfluenceEnabled: boolean;
};

export type SetupCompleteInput = {
  mode: string;
  lowMemoryModel: string;
  defaultModel: string;
};

export async function loadRuntimeSettingsSurface(): Promise<RuntimeSettingsSurface> {
  return {
    status: await call<Status>('Status'),
    setupState: await call<SetupState>('SetupState'),
    settings: await call<SettingsView>('Settings')
  };
}

export async function saveRuntimeSettings(input: SettingsSaveInput) {
  try {
    return await call<SettingsView>('SaveSettings', input);
  } catch (err) {
    if (!isLegacyResponseModeSettingsError(err)) {
      throw err;
    }
    return await call<SettingsView>('SaveSettings', legacySettingsSaveInput(input));
  }
}

function isLegacyResponseModeSettingsError(err: unknown) {
  const message = err instanceof Error ? err.message : String(err);
  return message.includes('unknown field "responseMode"') || message.includes('unknown field "showThinkingTrace"');
}

function legacySettingsSaveInput(input: SettingsSaveInput) {
  const { responseMode: _responseMode, showThinkingTrace: _showThinkingTrace, ...legacyInput } = input;
  return legacyInput;
}

export async function completeFirstRunSetup(input: SetupCompleteInput) {
  return await call<SetupState>('CompleteSetup', input);
}

export async function indexEmbeddingChunks() {
  return await call<EmbeddingIndexSummary>('IndexEmbeddings');
}

export function embeddingIndexSummaryText(result: EmbeddingIndexSummary) {
  return `${result.chunksEmbedded} embedded, ${result.chunksSkipped} skipped, ${result.dimensions} dimensions`;
}
