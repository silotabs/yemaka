import type { ModelDetails, ModelGenerateResult, ModelInfo, ModelProfile, ModelProfileApplyInput, ModelProfileDraftInput, ModelProfileInput, ModelProfileResult } from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';

export async function listRuntimeModels() {
  return asArray(await call<ModelInfo[]>('ListModels'));
}

export async function listModelProfiles() {
  return asArray(await call<ModelProfile[]>('ListModelProfiles'));
}

export async function saveModelRoleProvider(role: string, name: string, provider: string, baseURL: string) {
  await call<void>('SetModelRoleProvider', role, name, provider, baseURL);
}

export async function showRuntimeModel(name: string) {
  return await call<ModelDetails>('ShowModel', name);
}

export async function generateRuntimeModelText(name: string, prompt: string) {
  return await call<ModelGenerateResult>('GenerateModel', name, prompt);
}

export async function showModelProfile(name: string) {
  return await call<ModelProfileResult>('ShowModelProfile', name);
}

export async function previewModelProfile(input: ModelProfileInput) {
  return await call<ModelProfileResult>('PreviewModelProfile', input);
}

export async function draftModelProfileFromConversation(input: ModelProfileDraftInput) {
  return await call<ModelProfileResult>('DraftModelProfileFromConversation', input);
}

export async function saveModelProfile(input: ModelProfileInput) {
  return await call<ModelProfileResult>('SaveModelProfile', input);
}

export async function applyModelProfile(input: ModelProfileApplyInput) {
  return await call<ModelProfileResult>('ApplyModelProfile', input);
}

export function configuredModelRoleLabel(role: string, name: string, provider: string) {
  return `configured ${role}: ${name} (${provider})`;
}

export function modelDetailsLabel(name: string) {
  return `details loaded for ${name}`;
}

export function modelGeneratedLabel(model: string) {
  return `generated with ${model}`;
}

export function modelProfileSavedLabel(name: string) {
  return `saved model profile: ${name}`;
}

export function modelProfilePreviewLabel(name: string) {
  return `previewed model profile: ${name}`;
}

export function modelProfileDraftLabel(name: string) {
  return `drafted model profile: ${name}`;
}

export function modelProfileAppliedLabel(name: string, role: string) {
  return `applied model profile: ${name} -> ${role}`;
}
