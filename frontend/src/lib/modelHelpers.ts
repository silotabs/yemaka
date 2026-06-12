import { providerDefaultBaseURL } from './appOptions';

export type ModelRoleConfigLike = {
  Provider?: string;
  provider?: string;
  Name?: string;
  name?: string;
  BaseURL?: string;
  base_url?: string;
  baseUrl?: string;
};

export function roleValue(role: ModelRoleConfigLike | undefined, key: 'provider' | 'name' | 'baseURL') {
  if (!role) return '';
  if (key === 'provider') return role.Provider || role.provider || '';
  if (key === 'name') return role.Name || role.name || '';
  return role.BaseURL || role.base_url || role.baseUrl || '';
}

export type ModelRoleSettingsLike = {
  modelRoles?: Record<string, ModelRoleConfigLike>;
};

export type ModelRoleFormState = {
  provider: string;
  baseURL: string;
  name: string;
};

export function modelRoleFormFromSettings(settings: ModelRoleSettingsLike | null | undefined, roleName: string, currentName = ''): ModelRoleFormState {
  const role = settings?.modelRoles?.[roleName];
  const provider = roleValue(role, 'provider') || 'ollama';
  return {
    provider,
    baseURL: roleValue(role, 'baseURL') || providerDefaultBaseURL(provider),
    name: roleValue(role, 'name') || currentName
  };
}

export function titleFromPrompt(value: string, compactTitle: (value: string) => string) {
  const title = compactTitle(value);
  return title ? title.charAt(0).toUpperCase() + title.slice(1) : 'New chat';
}
