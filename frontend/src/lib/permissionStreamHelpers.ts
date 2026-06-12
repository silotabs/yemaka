import type { PermissionItem, ToolRun } from './appTypes';
import { permissionCommand } from './diagnosticHelpers';
import { boolValue } from './uiHelpers';

export type EditProposalDraft = {
  path?: string;
  content?: string;
  summary: string;
};

export function permissionItemFromEvent(data: Record<string, string>, now = Date.now): PermissionItem {
  const requestId = data.request_id || `${data.tool_name || 'permission'}-${now()}`;
  return {
    requestId,
    toolName: data.tool_name || 'requested_action',
    command: permissionCommand(data),
    riskLevel: data.risk_level || 'medium',
    reason: data.reason || '',
    nextStep: data.next_step || '',
    requiresConfirmation: boolValue(data.requires_confirmation),
    workspaceOnly: boolValue(data.workspace_only),
    diffPreview: boolValue(data.diff_preview),
    snapshotBeforeWrite: boolValue(data.snapshot_before_write),
    rollbackSupported: boolValue(data.rollback_supported),
    destructive: boolValue(data.destructive),
    policyLevel: data.policy_level || '',
    policyExplanation: data.policy_explanation || '',
    status: 'pending',
    conversationId: data.conversation_id || '',
    userMessageId: data.user_message_id || '',
    assistantMessageId: data.assistant_message_id || '',
    parentMessageId: data.parent_message_id || ''
  };
}

export function shouldStorePermissionItem(
  item: PermissionItem,
  items: PermissionItem[],
  settledByRequestId: Record<string, boolean>,
  busyByRequestId: Record<string, boolean>
) {
  const existing = items.find((candidate) => candidate.requestId === item.requestId);
  return !settledByRequestId[item.requestId] && !busyByRequestId[item.requestId] && (!existing || existing.status === 'pending');
}

export function upsertPermissionItem(items: PermissionItem[], item: PermissionItem, limit = 6) {
  return [item, ...items.filter((existing) => existing.requestId !== item.requestId)].slice(0, limit);
}

function normalizeToolName(name: string | undefined) {
  return String(name || '').trim().toLowerCase();
}

function objectRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function stringValue(record: Record<string, unknown>, ...keys: string[]) {
  for (const key of keys) {
    const value = record[key];
    if (value === undefined || value === null) continue;
    const clean = String(value).trim();
    if (clean) return clean;
  }
  return '';
}

function boolString(record: Record<string, unknown>, ...keys: string[]) {
  const raw = stringValue(record, ...keys).toLowerCase();
  return raw === 'true' || raw === '1' || raw === 'yes' ? 'true' : 'false';
}

function commandJSON(record: Record<string, unknown>) {
  const command = record.command;
  if (!Array.isArray(command)) return '';
  try {
    return JSON.stringify(command.map((item) => String(item)));
  } catch {
    return '';
  }
}

function requestIDFromRun(run: ToolRun) {
  const input = objectRecord(run.input);
  const output = objectRecord(run.output);
  return stringValue(input, 'request_id', 'requestId', 'RequestID') || stringValue(output, 'request_id', 'requestId', 'RequestID');
}

function permissionDecisionStatusFromRun(run: ToolRun): PermissionItem['status'] | '' {
  const output = objectRecord(run.output);
  const raw = (
    stringValue(output, 'decision', 'status', 'Status') ||
    stringValue(objectRecord(run.input), 'decision', 'status', 'Status') ||
    run.status ||
    ''
  )
    .trim()
    .toLowerCase();
  if (raw === 'approved') return 'approved';
  if (raw === 'rejected') return 'rejected';
  return '';
}

function permissionItemFromToolRun(run: ToolRun): PermissionItem | null {
  const output = objectRecord(run.output);
  const input = objectRecord(run.input);
  const requestId = requestIDFromRun(run);
  if (!requestId) return null;
  return permissionItemFromEvent({
    request_id: requestId,
    tool_name: stringValue(output, 'tool_name', 'toolName', 'ToolName') || stringValue(input, 'tool_name', 'toolName') || run.toolName,
    command_json: commandJSON(output),
    risk_level: stringValue(output, 'risk_level', 'riskLevel', 'RiskLevel') || run.riskLevel || 'medium',
    reason: stringValue(output, 'reason', 'Reason'),
    next_step: stringValue(output, 'next_step', 'nextStep', 'NextStep'),
    requires_confirmation: boolString(output, 'requires_confirmation', 'requiresConfirmation', 'RequiresConfirmation'),
    workspace_only: boolString(output, 'workspace_only', 'workspaceOnly', 'WorkspaceOnly'),
    diff_preview: boolString(output, 'diff_preview', 'diffPreview', 'DiffPreview'),
    snapshot_before_write: boolString(output, 'snapshot_before_write', 'snapshotBeforeWrite', 'SnapshotBeforeWrite'),
    rollback_supported: boolString(output, 'rollback_supported', 'rollbackSupported', 'RollbackSupported'),
    destructive: boolString(output, 'destructive', 'Destructive'),
    policy_level: stringValue(output, 'policy_level', 'policyLevel', 'PolicyLevel'),
    policy_explanation: stringValue(output, 'policy_explanation', 'policyExplanation', 'PolicyExplanation'),
    conversation_id: run.conversationId || '',
    user_message_id: run.userMessageId || '',
    assistant_message_id: run.assistantMessageId || '',
    parent_message_id: run.parentMessageId || ''
  });
}

export function permissionItemsFromToolRuns(
  runs: ToolRun[] | null | undefined,
  activeConversationId = '',
  settledByRequestId: Record<string, boolean> = {},
  busyByRequestId: Record<string, boolean> = {},
  limit = 6
) {
  const list = Array.isArray(runs) ? runs : [];
  const decisions = new Map<string, PermissionItem['status']>();
  for (const run of list) {
    const tool = normalizeToolName(run.toolName);
    if (tool !== 'permission_decision' && tool !== 'permission_result') continue;
    const requestId = requestIDFromRun(run);
    const status = permissionDecisionStatusFromRun(run);
    if (requestId && status) decisions.set(requestId, status);
  }
  const out: PermissionItem[] = [];
  for (const run of list) {
    if (normalizeToolName(run.toolName) !== 'permission_request') continue;
    if (activeConversationId && run.conversationId && run.conversationId !== activeConversationId) continue;
    const requestId = requestIDFromRun(run);
    if (!requestId) continue;
    const status = decisions.get(requestId) || 'pending';
    if (status === 'pending' && (settledByRequestId[requestId] || busyByRequestId[requestId])) continue;
    if (String(run.status || '').trim() !== 'needs_confirmation') continue;
    const item = permissionItemFromToolRun(run);
    if (!item) continue;
    item.status = status;
    out.push(item);
    if (out.length >= limit) break;
  }
  return out;
}

export function permissionDecisionBusyState(busyByRequestId: Record<string, boolean>, requestId: string, busy: boolean) {
  if (!requestId) return busyByRequestId;
  if (busy) return { ...busyByRequestId, [requestId]: true };
  const next = { ...busyByRequestId };
  delete next[requestId];
  return next;
}

export function editProposalDraftFromEvent(data: Record<string, string>): EditProposalDraft {
  return {
    path: data.path,
    content: data.content,
    summary: data.needs_content === 'true' ? 'Add full file content, then preview the diff.' : 'Preview the proposed edit before applying.'
  };
}
