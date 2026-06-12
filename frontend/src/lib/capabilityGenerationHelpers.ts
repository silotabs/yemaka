import type { CapabilityGenerationResult, CapabilityHandoff, CapabilityProposal } from './appTypes';
import { parseCapabilityObject, proposalFromCapabilityHandoff } from './handoffHelpers';

export type CapabilityFormGenerationOptions = {
  request: string;
  name: string;
  proposalRequest: string;
  proposal: CapabilityProposal | null;
  runAfterGenerate: boolean;
  runInputJSON: string;
  scheduleAfterGenerate: boolean;
  scheduleType: string;
  scheduleExpr: string;
  scheduleEnabled: boolean;
  scheduleInputJSON: string;
};

export type CapabilityExtensionFormState = {
  request: string;
  name: string;
  description: string;
  proposal: CapabilityProposal;
  proposalRequest: string;
  nextSteps: string[];
  summary: string;
};

export function capabilityGenerationInputFromForm(options: CapabilityFormGenerationOptions): Record<string, unknown> {
  return {
    request: options.request,
    name: options.name,
    allowedDomains: options.proposalRequest === options.request ? options.proposal?.allowedDomains ?? [] : [],
    approved: true,
    runAfterGenerate: options.runAfterGenerate,
    runInput: options.runAfterGenerate ? parseCapabilityObject(options.runInputJSON, 'Run input') : undefined,
    scheduleAfterGenerate: options.scheduleAfterGenerate,
    scheduleType: options.scheduleType,
    scheduleExpr: options.scheduleType === 'manual' ? '' : options.scheduleExpr,
    scheduleEnabled: options.scheduleEnabled,
    scheduleInput: options.scheduleAfterGenerate ? parseCapabilityObject(options.scheduleInputJSON, 'Job input') : undefined
  };
}

export function capabilityGenerationInputFromHandoff(handoff: CapabilityHandoff, activeConversationId: string): Record<string, unknown> {
  const scheduleInput = handoff.scheduleAfterGenerate ? parseCapabilityObject(handoff.scheduleInputJSON, 'Job input') : undefined;
  if (scheduleInput && activeConversationId && !scheduleInput.conversation_id && !scheduleInput.conversationId) {
    scheduleInput.conversation_id = activeConversationId;
  }
  return {
    request: handoff.request || handoff.description || handoff.title,
    name: handoff.name,
    allowedDomains: handoff.allowedDomains ?? [],
    approved: true,
    runAfterGenerate: Boolean(handoff.runAfterGenerate),
    runInput: handoff.runAfterGenerate ? parseCapabilityObject(handoff.runInputJSON, 'Run input') : undefined,
    scheduleAfterGenerate: Boolean(handoff.scheduleAfterGenerate),
    scheduleType: handoff.scheduleAfterGenerate ? handoff.scheduleType || 'manual' : undefined,
    scheduleExpr: handoff.scheduleAfterGenerate && (handoff.scheduleType || 'manual') !== 'manual' ? handoff.scheduleExpr : undefined,
    scheduleEnabled: Boolean(handoff.scheduleEnabled),
    scheduleInput
  };
}

export function generatedCapabilityName(result: CapabilityGenerationResult, fallbackName: string, fallbackTitle = '') {
  return result.generation?.extension?.name || result.proposal?.name || fallbackName || fallbackTitle;
}

export function capabilityExtensionFormFromHandoff(
  handoff: CapabilityHandoff,
  currentName: string,
  currentDescription: string,
  currentNextSteps: string[]
): CapabilityExtensionFormState {
  const request = handoff.request || handoff.description || handoff.title;
  return {
    request,
    name: handoff.generatedName || handoff.name || currentName,
    description: handoff.description || currentDescription,
    proposal: proposalFromCapabilityHandoff(handoff),
    proposalRequest: request,
    nextSteps: handoff.nextSteps ?? currentNextSteps,
    summary: handoff.generatedName ? `generated ${handoff.generatedName}` : `capability proposal: ${handoff.title}`
  };
}

export function generatedExtensionNameFromLegacyResult(result: { extension?: { name?: string } }, fallbackName: string) {
  return result.extension?.name || fallbackName;
}
