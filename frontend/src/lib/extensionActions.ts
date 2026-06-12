import type {
  CapabilityGenerationResult,
  CapabilityProposal,
  Extension,
  ExtensionActionResult,
  ExtensionFailureTrend,
  ExtensionReview,
  ExtensionRollbackResult,
  ExtensionRunResult
} from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';

export type ExtensionTestResult = {
  name?: string;
  status?: string;
  output?: string;
};

export type ExtensionRegisterResult = {
  extension?: Extension;
  status?: string;
};

export type ExtensionGenerateResult = {
  extension: Extension;
  tests?: { status?: string };
};

export type ExtensionSurface = {
  extensions: Extension[];
  failures: ExtensionFailureTrend[];
};

export async function loadExtensionSurface(failureLimit = 20): Promise<ExtensionSurface> {
  return {
    extensions: asArray(await call<Extension[]>('ListExtensions')),
    failures: asArray(await call<ExtensionFailureTrend[]>('ExtensionFailures', failureLimit))
  };
}

export async function listExtensions() {
  return asArray(await call<Extension[]>('ListExtensions'));
}

export async function listExtensionFailures(limit = 20) {
  return asArray(await call<ExtensionFailureTrend[]>('ExtensionFailures', limit));
}

export async function reviewExtensionByName(name: string) {
  return await call<ExtensionReview>('ReviewExtension', name);
}

export async function testExtensionByName(name: string) {
  return await call<ExtensionTestResult>('TestExtension', name);
}

export async function registerExtensionByName(name: string) {
  return await call<ExtensionRegisterResult>('RegisterExtension', name);
}

export async function proposeCapability(request: string) {
  return await call<CapabilityProposal>('ProposeCapability', { request });
}

export async function generateCapability(input: Record<string, unknown>) {
  return await call<CapabilityGenerationResult>('GenerateCapability', input);
}

export async function generateExtensionFromDescription(input: { name: string; description: string; approved: boolean }) {
  return await call<ExtensionGenerateResult>('GenerateExtension', input);
}

export async function validateExtensionByName(name: string) {
  return await call<ExtensionActionResult>('ValidateExtension', name);
}

export async function setExtensionEnabledState(name: string, enabled: boolean) {
  return await call<ExtensionActionResult>('SetExtensionEnabled', name, enabled);
}

export async function deleteExtensionByName(name: string) {
  return await call<ExtensionActionResult>('DeleteExtension', name);
}

export async function runExtensionByName(name: string, input: Record<string, unknown>) {
  return await call<ExtensionRunResult>('RunExtension', { name, input });
}

export async function rollbackExtensionTarget(nameOrSnapshot: string) {
  return await call<ExtensionRollbackResult>('RollbackExtension', { nameOrSnapshot });
}

export function extensionActionSummary(result: ExtensionActionResult) {
  return `${result.extension.name}: ${result.message}`;
}
