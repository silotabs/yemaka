import {
  internetSearchProviderDefaultAPIKeyEnv,
  internetSearchProviderOption,
  isKnownSearchProviderDefaultAPIKeyEnv
} from './appOptions';

export function internetSearchProviderEndpointDisabled(provider: string) {
  const option = internetSearchProviderOption(provider);
  return option.endpointMode === 'none' || option.endpointMode === 'unsupported';
}

export function internetSearchProviderAPIKeyDisabled(provider: string) {
  const option = internetSearchProviderOption(provider);
  return option.endpointMode === 'unsupported' || !option.apiKeyEnv;
}

export function internetSearchProviderEndpointPlaceholder(provider: string) {
  const option = internetSearchProviderOption(provider);
  if (option.id === 'searxng') return 'https://searx.example/search';
  if (option.id === 'tavily') return 'optional https://api.tavily.com/search';
  if (option.id === 'serper') return 'optional https://google.serper.dev/search';
  if (option.id === 'brave') return 'optional Brave API endpoint';
  if (option.id === 'wikimedia') return 'optional Wikimedia API endpoint';
  if (option.id === 'duckduckgo') return 'optional DuckDuckGo-compatible endpoint';
  if (option.id === 'firecrawl') return 'optional https://api.firecrawl.dev/v2/search';
  if (option.id === 'mojeek') return 'optional https://api.mojeek.com/search';
  if (option.endpointMode === 'required') return 'https://search.example/search?q={query}';
  return 'provider endpoint';
}

export function internetSearchProviderAPIKeyPlaceholder(provider: string) {
  return internetSearchProviderDefaultAPIKeyEnv(provider) || 'API key env name';
}

export function normalizedInternetSearchAPIKeyEnv(provider: string, value: string) {
  const option = internetSearchProviderOption(provider);
  const clean = value.trim();
  if (!option.apiKeyEnv) return '';
  const providerDefault = internetSearchProviderDefaultAPIKeyEnv(option.id);
  if (!clean) return providerDefault;
  if (providerDefault && clean !== providerDefault && isKnownSearchProviderDefaultAPIKeyEnv(clean)) {
    return providerDefault;
  }
  return clean;
}

export function internetSearchAPIKeyEnvValidationMessage(provider: string, value: string) {
  const option = internetSearchProviderOption(provider);
  const clean = value.trim();
  if (!option.apiKeyEnv || !clean) return '';
  if (/^(fc-|sk-|pk-)/i.test(clean)) {
    return `${option.label} expects an environment variable name such as ${internetSearchProviderAPIKeyPlaceholder(provider)}, not the raw API key. Yemaka will not store raw API keys.`;
  }
  if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(clean)) {
    return `${option.label} API key env name can only use letters, numbers, and underscores. Use ${internetSearchProviderAPIKeyPlaceholder(provider)} or another shell-visible env var name.`;
  }
  return '';
}

export function internetSearchAPIKeyEnvHelp(provider: string, value: string) {
  const option = internetSearchProviderOption(provider);
  if (!option.apiKeyEnv) return '';
  return `Set ${normalizedInternetSearchAPIKeyEnv(provider, value)} before launching Yemaka, or restart the app after setting it.`;
}

export function normalizedInternetSearchProvider(provider: string) {
  const option = internetSearchProviderOption(provider);
  if (option.disabled || option.endpointMode === 'unsupported') return 'none';
  return option.id || 'none';
}
