import type { ExtensionRunResult, Job } from './appTypes';
import { humanizeIdentifier } from './uiHelpers';

export type NormalizedAgentStreamEvent<T = unknown> = {
  type: string;
  token: string;
  message: string;
  data: Record<string, string>;
  result?: T;
  error: string;
};

export type RawAgentStreamEvent<T = unknown> = Partial<NormalizedAgentStreamEvent<T>> & {
  Type?: string;
  Token?: string;
  Message?: string;
  Data?: Record<string, string>;
  Result?: T;
  Error?: string;
};

export type CapabilityRuntimeStatus = {
  source?: string;
  state?: string;
  installed?: boolean;
  enabled?: boolean;
  valid?: boolean;
  requiresApproval?: boolean;
  reason?: string;
  configureHint?: string;
  repairHint?: string;
  requiredTools?: string[];
  optionalTools?: string[];
  skills?: string[];
  riskyTools?: string[];
  riskReason?: string;
};

export type CapabilityHandoffStatus = 'pending' | 'generating' | 'generated' | 'failed' | 'not_available';

export type CapabilityHandoff = {
  request: string;
  kind: string;
  name: string;
  title: string;
  description: string;
  reason: string;
  suggestedAction: string;
  requiresApproval: boolean;
  canGenerate: boolean;
  files: string[];
  permissions: string[];
  generationCommand: string;
  networkMode: string;
  allowedDomains: string[];
  existingCapability?: string;
  capabilitySummary?: string;
  capabilityAction?: string;
  capabilityState?: string;
  configureHint?: string;
  packSource?: string;
  packDir?: string;
  installCommand?: string;
  runtimeStatus?: CapabilityRuntimeStatus;
  status: CapabilityHandoffStatus;
  message?: string;
  generatedName?: string;
  nextSteps?: string[];
  runAfterGenerate?: boolean;
  runInputJSON?: string;
  scheduleAfterGenerate?: boolean;
  scheduleType?: string;
  scheduleExpr?: string;
  scheduleEnabled?: boolean;
  scheduleInputJSON?: string;
  run?: ExtensionRunResult;
  schedule?: Job;
};

export type CapabilityProposal = {
  needed: boolean;
  kind: string;
  name: string;
  title: string;
  description: string;
  reason: string;
  suggestedAction: string;
  requiresApproval: boolean;
  canGenerate: boolean;
  files: string[];
  permissions: string[];
  generationCommand: string;
  networkMode: string;
  allowedDomains: string[];
  existingCapability?: string;
  capabilitySummary?: string;
  capabilityAction?: string;
  capabilityState?: string;
  runtimeStatus?: CapabilityRuntimeStatus;
};

export type SchedulerHandoffStatus = 'incomplete' | 'pending' | 'creating' | 'created' | 'failed';

export type SchedulerHandoff = {
  request: string;
  targetType: string;
  targetName: string;
  scheduleType: string;
  scheduleExpr: string;
  intent?: string;
  intentLabel?: string;
  intentSummary?: string;
  setupHint?: string;
  approved: boolean;
  enabled: boolean;
  missing: string[];
  canCreate: boolean;
  cliCommand: string;
  enableCommand: string;
  setupTemplate: string;
  inputJSON?: string;
  status: SchedulerHandoffStatus;
  message?: string;
  job?: unknown;
};

export type CapabilityExtensionLike = {
  name?: string;
  generatedName?: string;
  canGenerate?: boolean;
  status?: string;
  runAfterGenerate?: boolean;
  scheduleAfterGenerate?: boolean;
  scheduleType?: string;
  scheduleExpr?: string;
  runInputJSON?: string;
  scheduleInputJSON?: string;
};

export type ExtensionLike = {
  name?: string;
};

export function normalizeStreamEvent<T = unknown>(event: unknown): NormalizedAgentStreamEvent<T> {
  const raw = (event || {}) as RawAgentStreamEvent<T>;
  return {
    type: raw.type ?? raw.Type ?? '',
    token: raw.token ?? raw.Token ?? '',
    message: raw.message ?? raw.Message ?? '',
    data: raw.data ?? raw.Data ?? {},
    result: raw.result ?? raw.Result,
    error: raw.error ?? raw.Error ?? ''
  };
}

export function splitEventList(value: string | undefined) {
  return String(value || '')
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean);
}

export function eventFlag(value: string | undefined) {
  return String(value || '').trim().toLowerCase() === 'true';
}

export function capabilityKindLabel(kind: string | undefined) {
  return humanizeIdentifier(kind, 'Capability');
}

export function eventValue(data: Record<string, string>, ...keys: string[]) {
  for (const key of keys) {
    const value = data[key];
    if (String(value || '').trim()) return value;
  }
  return '';
}

export function runtimeStatusFromEvent(data: Record<string, string>): CapabilityRuntimeStatus | undefined {
  const hasStatus =
    eventValue(data, 'runtime_state') ||
    eventValue(data, 'runtime_source') ||
    eventValue(data, 'runtime_required_tools') ||
    eventValue(data, 'runtime_risky_tools');
  if (!hasStatus) return undefined;
  return {
    source: eventValue(data, 'runtime_source'),
    state: eventValue(data, 'runtime_state'),
    installed: eventFlag(eventValue(data, 'runtime_installed')),
    enabled: eventFlag(eventValue(data, 'runtime_enabled')),
    valid: eventFlag(eventValue(data, 'runtime_valid')),
    requiresApproval: eventFlag(eventValue(data, 'runtime_requires_approval')),
    reason: eventValue(data, 'runtime_reason'),
    configureHint: eventValue(data, 'runtime_configure_hint'),
    repairHint: eventValue(data, 'runtime_repair_hint'),
    requiredTools: splitEventList(eventValue(data, 'runtime_required_tools')),
    optionalTools: splitEventList(eventValue(data, 'runtime_optional_tools')),
    skills: splitEventList(eventValue(data, 'runtime_skills')),
    riskyTools: splitEventList(eventValue(data, 'runtime_risky_tools')),
    riskReason: eventValue(data, 'runtime_risk_reason')
  };
}

export function capabilityHandoffFromEvent(data: Record<string, string> | undefined): CapabilityHandoff | null {
  if (!data) return null;
  const title = eventValue(data, 'title') || eventValue(data, 'name') || 'Missing capability';
  const canGenerate = eventFlag(eventValue(data, 'can_generate', 'canGenerate'));
  return {
    request: eventValue(data, 'request') || eventValue(data, 'description') || title,
    kind: eventValue(data, 'kind') || 'tool_extension',
    name: eventValue(data, 'name'),
    title,
    description: eventValue(data, 'description') || eventValue(data, 'reason'),
    reason: eventValue(data, 'reason'),
    suggestedAction: eventValue(data, 'suggested_action', 'suggestedAction'),
    requiresApproval: eventFlag(eventValue(data, 'requires_approval', 'requiresApproval')),
    canGenerate,
    files: splitEventList(eventValue(data, 'files')),
    permissions: splitEventList(eventValue(data, 'permissions')),
    generationCommand: eventValue(data, 'generation_command', 'generationCommand'),
    networkMode: eventValue(data, 'network_mode', 'networkMode'),
    allowedDomains: splitEventList(eventValue(data, 'allowed_domains', 'allowedDomains')),
    existingCapability: eventValue(data, 'existing_capability', 'existingCapability'),
    capabilitySummary: eventValue(data, 'capability_summary', 'capabilitySummary'),
    capabilityAction: eventValue(data, 'capability_action', 'capabilityAction'),
    capabilityState: eventValue(data, 'capability_state', 'capabilityState'),
    configureHint: eventValue(data, 'configure_hint', 'configureHint'),
    packSource: eventValue(data, 'pack_source', 'packSource', 'capability_source', 'source'),
    packDir: eventValue(data, 'pack_dir', 'packDir', 'dir'),
    installCommand: eventValue(data, 'install_command', 'installCommand', 'pack_install_command', 'packInstallCommand'),
    runtimeStatus: runtimeStatusFromEvent(data),
    status: canGenerate ? 'pending' : 'not_available',
    runInputJSON: '{}',
    scheduleType: 'manual',
    scheduleExpr: '',
    scheduleEnabled: false,
    scheduleInputJSON: '{}'
  };
}

export function schedulerHandoffFromEvent(data: Record<string, string> | undefined): SchedulerHandoff | null {
  if (!data) return null;
  const missing = splitEventList(eventValue(data, 'missing'));
  const canCreate = eventFlag(eventValue(data, 'can_create', 'canCreate')) && missing.length === 0;
  const request = eventValue(data, 'request');
  const targetType = eventValue(data, 'target_type', 'targetType') || 'extension';
  const targetName = eventValue(data, 'target_name', 'targetName');
  return {
    request,
    targetType,
    targetName,
    scheduleType: eventValue(data, 'schedule_type', 'scheduleType') || 'manual',
    scheduleExpr: eventValue(data, 'schedule_expr', 'scheduleExpr'),
    intent: eventValue(data, 'intent'),
    intentLabel: eventValue(data, 'intent_label', 'intentLabel'),
    intentSummary: eventValue(data, 'intent_summary', 'intentSummary'),
    setupHint: eventValue(data, 'setup_hint', 'setupHint'),
    approved: eventFlag(eventValue(data, 'approved')),
    enabled: eventFlag(eventValue(data, 'enabled')),
    missing,
    canCreate,
    cliCommand: eventValue(data, 'cli_command', 'cliCommand'),
    enableCommand: eventValue(data, 'enable_command', 'enableCommand'),
    setupTemplate: eventValue(data, 'setup_template', 'setupTemplate'),
    inputJSON: eventValue(data, 'input_json', 'inputJson') || schedulerHandoffDefaultInputJSON(targetType, targetName, request),
    status: canCreate ? 'pending' : 'incomplete'
  };
}

function schedulerHandoffDefaultInputJSON(targetType: string, targetName: string, request: string) {
  if (String(targetType || '').trim() !== 'extension') return '{}';
  const input: Record<string, string> = {};
  const url = firstURL(request);
  if (url) input.url = url;
  const task = schedulerHandoffDefaultTask(targetName, request);
  if (task) input.task = task;
  return Object.keys(input).length ? JSON.stringify(input, null, 2) : '{}';
}

function schedulerHandoffDefaultTask(targetName: string, request: string) {
  const cleanRequest = String(request || '').replace(/\s+/g, ' ').trim();
  if (cleanRequest && !cleanRequest.toLowerCase().startsWith('scheduler_setup:')) {
    return cleanRequest.length > 220 ? `${cleanRequest.slice(0, 220)}...` : cleanRequest;
  }
  const target = humanizeIdentifier(targetName, 'scheduled task');
  return `Run ${target} and report the result.`;
}

function firstURL(value: string) {
  const match = String(value || '').match(/https?:\/\/[^\s"'<>]+/i);
  return match ? match[0].replace(/[),.;]+$/, '') : '';
}

export function schedulerHandoffCardClass(handoff: SchedulerHandoff) {
  switch (handoff.status) {
    case 'created':
      return 'generated';
    case 'creating':
      return 'generating';
    case 'failed':
      return 'failed';
    case 'incomplete':
      return 'not_available';
    default:
      return 'pending';
  }
}

export function schedulerHandoffDisabled(handoff: SchedulerHandoff, busy: boolean) {
  if (busy || !handoff.canCreate || handoff.status === 'creating' || handoff.status === 'created') return true;
  if (!String(handoff.targetType || '').trim() || !String(handoff.targetName || '').trim()) return true;
  if ((handoff.scheduleType || 'manual') !== 'manual' && !String(handoff.scheduleExpr || '').trim()) return true;
  return false;
}

export function parseCapabilityObject(value: string | undefined, label: string): Record<string, unknown> | undefined {
  const trimmed = String(value || '').trim();
  if (!trimmed || trimmed === '{}') return undefined;
  const parsed = JSON.parse(trimmed) as unknown;
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object`);
  }
  return parsed as Record<string, unknown>;
}

export function capabilityExtensionName(handoff: CapabilityExtensionLike | undefined) {
  if (!handoff) return '';
  return String(handoff.generatedName || handoff.name || '').trim();
}

export function existingExtensionForCapability<TExtension extends ExtensionLike>(
  extensions: TExtension[] | null | undefined,
  handoff: CapabilityExtensionLike | undefined
) {
  const name = capabilityExtensionName(handoff);
  if (!name) return null;
  return (extensions ?? []).find((extension) => extension.name === name) ?? null;
}

export function capabilityApproveDisabled(handoff: CapabilityExtensionLike, busy: boolean, existingExtension: unknown) {
  if (existingExtension) return true;
  if (busy || !handoff.canGenerate || handoff.status === 'generating' || handoff.status === 'generated') return true;
  if (handoff.scheduleAfterGenerate && (handoff.scheduleType || 'manual') !== 'manual' && !String(handoff.scheduleExpr || '').trim()) return true;
  return false;
}

export function capabilityApproveLabel(handoff: CapabilityExtensionLike, existingExtension: unknown) {
  if (existingExtension) return 'Already generated';
  if (handoff.status === 'generated') return 'Generated';
  if (handoff.status === 'generating') return 'Generating...';
  if (handoff.runAfterGenerate && handoff.scheduleAfterGenerate) return 'Approve, run, and create job';
  if (handoff.runAfterGenerate) return 'Approve and run once';
  if (handoff.scheduleAfterGenerate) return 'Approve and create job';
  return 'Approve generation';
}

export function proposalFromCapabilityHandoff(handoff: CapabilityHandoff): CapabilityProposal {
  return {
    needed: true,
    kind: handoff.kind,
    name: handoff.generatedName || handoff.name,
    title: handoff.title,
    description: handoff.description,
    reason: handoff.reason,
    suggestedAction: handoff.suggestedAction,
    requiresApproval: handoff.requiresApproval,
    canGenerate: handoff.canGenerate,
    files: handoff.files,
    permissions: handoff.permissions,
    generationCommand: handoff.generationCommand,
    networkMode: handoff.networkMode,
    allowedDomains: handoff.allowedDomains,
    existingCapability: handoff.existingCapability,
    capabilitySummary: handoff.capabilitySummary,
    capabilityAction: handoff.capabilityAction,
    capabilityState: handoff.capabilityState,
    runtimeStatus: handoff.runtimeStatus
  };
}
