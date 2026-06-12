import { humanizeIdentifier } from './uiHelpers';

export type Tab = 'chat' | 'models' | 'memory' | 'documents' | 'skills' | 'extensions' | 'automation' | 'learning' | 'tools' | 'settings';

export type InternetSearchProviderOption = {
  id: string;
  label: string;
  endpointMode: 'none' | 'required' | 'optional' | 'unsupported';
  apiKeyEnv: boolean;
  disabled?: boolean;
  note: string;
};

export const tabs: Array<{ id: Tab; label: string; icon: string }> = [
  { id: 'chat', label: 'Chat', icon: 'chat' },
  { id: 'models', label: 'Models', icon: 'models' },
  { id: 'memory', label: 'Memory', icon: 'memory' },
  { id: 'documents', label: 'Documents', icon: 'documents' },
  { id: 'skills', label: 'Skills', icon: 'skills' },
  { id: 'extensions', label: 'Extensions', icon: 'extensions' },
  { id: 'automation', label: 'Automation', icon: 'automation' },
  { id: 'learning', label: 'Learning', icon: 'learning' },
  { id: 'tools', label: 'Tool Logs', icon: 'tools' },
  { id: 'settings', label: 'Settings', icon: 'settings' }
];

export const moreTabs = tabs.filter((tab) => !['chat', 'memory', 'documents'].includes(tab.id));

export const promptChips = [
  { label: 'Explain', icon: 'documents', prompt: 'Explain this project in a concise architecture summary.' },
  { label: 'Review', icon: 'search', prompt: 'Review the current git diff for bugs, regressions, and missing tests.' },
  { label: 'Fix', icon: 'tools', prompt: 'Help me investigate and fix the next bug safely.' },
  { label: 'Learn', icon: 'learning', prompt: 'Create a reusable skill from the last successful workflow.' }
];

export const tabSubtitles: Record<Tab, string> = {
  chat: 'Chat with your Yemaka and manage conversations',
  models: 'Configure your Yemaka models and providers',
  memory: 'View and manage your Yemaka memory',
  documents: 'View and manage your Yemaka documents',
  skills: 'View and manage your Yemaka skills',
  extensions: 'View and manage your Yemaka extensions',
  automation: 'View and manage your Yemaka automations',
  learning: 'View and manage your Yemaka learning',
  tools: 'View and manage your Yemaka tool logs',
  settings: 'Configure your Yemaka settings'
};

export const internetSearchProviderOptions: InternetSearchProviderOption[] = [
  {
    id: 'none',
    label: 'No provider',
    endpointMode: 'none',
    apiKeyEnv: false,
    note: 'Search stays disabled until internet access, internet search, and a provider are explicitly enabled.'
  },
  {
    id: 'auto',
    label: 'Auto fallback',
    endpointMode: 'none',
    apiKeyEnv: false,
    note: 'Uses the first ready configured provider in priority order: Tavily, Serper.dev, Brave, Firecrawl, Wikimedia, DuckDuckGo, then SearXNG.'
  },
  {
    id: 'tavily',
    label: 'Tavily',
    endpointMode: 'optional',
    apiKeyEnv: true,
    note: 'Supported POST vendor API for AI-agent/RAG search. Enter TAVILY_API_KEY or another environment variable name; endpoint override is optional.'
  },
  {
    id: 'serper',
    label: 'Serper.dev',
    endpointMode: 'optional',
    apiKeyEnv: true,
    note: 'Supported POST Google SERP provider for basic web search. Enter SERPER_API_KEY or another environment variable name; news can come later.'
  },
  {
    id: 'brave',
    label: 'Brave Search',
    endpointMode: 'optional',
    apiKeyEnv: true,
    note: 'Supported freemium GET API provider. Enter BRAVE_SEARCH_API_KEY or another environment variable name; endpoint override is optional.'
  },
  {
    id: 'firecrawl',
    label: 'Firecrawl',
    endpointMode: 'optional',
    apiKeyEnv: true,
    note: 'Supported POST vendor API for search/extract/crawl-oriented workflows. Enter FIRECRAWL_API_KEY or another environment variable name; endpoint override is optional.'
  },
  {
    id: 'searxng',
    label: 'SearXNG',
    endpointMode: 'required',
    apiKeyEnv: false,
    note: 'Optional local/self-hosted GET provider. Requires your own SearXNG-compatible endpoint; Yemaka does not require Docker.'
  },
  {
    id: 'wikimedia',
    label: 'Wikimedia',
    endpointMode: 'optional',
    apiKeyEnv: false,
    note: 'Free no-key GET fallback for encyclopedia-style results. Not a complete general web search provider.'
  },
  {
    id: 'duckduckgo',
    label: 'DuckDuckGo',
    endpointMode: 'optional',
    apiKeyEnv: false,
    note: 'Free no-key Instant Answer fallback using api.duckduckgo.com by default. Not a full web SERP provider.'
  },
  {
    id: 'mojeek',
    label: 'Mojeek Search',
    endpointMode: 'optional',
    apiKeyEnv: true,
    note: 'Supported GET provider. Enter an environment variable name such as MOJEEK_API_KEY, not the raw API key; endpoint override is optional.'
  },
  {
    id: 'yemaka_custom',
    label: 'Custom GET provider',
    endpointMode: 'required',
    apiKeyEnv: false,
    note: 'Configured GET provider for SearXNG-like JSON responses from a user-managed endpoint.'
  }
];

export const internetSearchProviderSelectOptions = internetSearchProviderOptions.map((provider) => ({
  value: provider.id,
  label: provider.label,
  disabled: provider.disabled,
  meta: provider.note
}));

export const runtimeProviders = [
  { id: 'ollama', label: 'Ollama', baseURL: 'http://localhost:11434/api' },
  { id: 'llamacpp', label: 'llama.cpp local server', baseURL: 'http://127.0.0.1:8080/v1' },
  { id: 'openai_compatible', label: 'OpenAI-compatible local', baseURL: 'http://127.0.0.1:4000/v1' }
];

export const modelRoleOptions = [
  { value: 'low_memory', label: 'Low Memory' },
  { value: 'default', label: 'Default' },
  { value: 'coding', label: 'Coding' },
  { value: 'reasoning', label: 'Reasoning' },
  { value: 'stronger_local', label: 'Stronger Local' }
];

export const memoryKindOptions = [
  { value: 'preference', label: 'preference' },
  { value: 'project_note', label: 'project_note' },
  { value: 'workflow', label: 'workflow' },
  { value: 'summary', label: 'summary' }
];

export const jobScheduleTypeOptions = [
  { value: 'manual', label: 'manual' },
  { value: 'interval', label: 'interval' },
  { value: 'cron', label: 'cron' },
  { value: 'one_time', label: 'one_time' }
];

export const jobTargetTypeOptions = [
  { value: 'extension', label: 'extension' },
  { value: 'heartbeat', label: 'heartbeat' }
];

export const documentFileAccept = [
  '.md',
  '.markdown',
  '.txt',
  '.pdf',
  '.docx',
  '.json',
  '.yaml',
  '.yml',
  '.toml',
  '.html',
  '.css',
  '.go',
  '.py',
  '.js',
  '.ts',
  '.svelte'
].join(',');

export function internetSearchProviderOption(provider: string | undefined): InternetSearchProviderOption {
  const clean = String(provider || 'none').trim() || 'none';
  return (
    internetSearchProviderOptions.find((option) => option.id === clean) ?? {
      id: clean,
      label: humanizeIdentifier(clean, clean),
      endpointMode: 'required',
      apiKeyEnv: false,
      note: 'Unknown provider. Yemaka will treat this as unsupported until the core registry recognizes it.'
    }
  );
}

export function internetSearchProviderDefaultAPIKeyEnv(provider: string | undefined): string {
  const option = internetSearchProviderOption(provider);
  if (option.id === 'tavily') return 'TAVILY_API_KEY';
  if (option.id === 'serper') return 'SERPER_API_KEY';
  if (option.id === 'brave') return 'BRAVE_SEARCH_API_KEY';
  if (option.id === 'firecrawl') return 'FIRECRAWL_API_KEY';
  if (option.id === 'mojeek') return 'MOJEEK_API_KEY';
  return '';
}

export function isKnownSearchProviderDefaultAPIKeyEnv(value: string) {
  const clean = String(value || '').trim();
  const knownDefaults: string[] = internetSearchProviderOptions
    .map((option) => internetSearchProviderDefaultAPIKeyEnv(option.id))
    .filter(Boolean);
  return knownDefaults.includes(clean);
}

export function providerDefaultBaseURL(provider: string) {
  return runtimeProviders.find((item) => item.id === provider)?.baseURL ?? runtimeProviders[0].baseURL;
}

export function isTab(value: string | null): value is Tab {
  return tabs.some((tab) => tab.id === value);
}
