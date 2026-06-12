import type { Job, SchedulerHandoff } from './appTypes';
import type { SchedulerJobInput, SchedulerJobUpdateInput } from './automationActions';
import { humanizeIdentifier } from './uiHelpers';

export type SchedulerJobFormState = {
  scheduleType: string;
  scheduleExpr: string;
  targetType: string;
  targetName: string;
  inputJSON: string;
};

export function schedulerJobInputFromForm(form: SchedulerJobFormState): SchedulerJobInput {
  const schedule = normalizeSchedulerSchedule(form.scheduleType, form.scheduleExpr);
  return {
    scheduleType: schedule.scheduleType,
    scheduleExpr: schedule.scheduleType === 'manual' ? '' : schedule.scheduleExpr,
    targetType: form.targetType,
    targetName: form.targetName,
    input: parseSchedulerJobInput(form.inputJSON, 'Job input'),
    approved: true,
    enabled: false
  };
}

export function schedulerJobInputFromHandoff(handoff: SchedulerHandoff, activeConversationId = ''): SchedulerJobInput {
  const schedule = normalizeSchedulerSchedule(handoff.scheduleType || 'manual', handoff.scheduleExpr);
  const input = parseSchedulerJobInput(schedulerHandoffInputJSON(handoff), 'Job input');
  if (activeConversationId && !input.conversation_id && !input.conversationId) {
    input.conversation_id = activeConversationId;
  }
  return {
    scheduleType: schedule.scheduleType,
    scheduleExpr: schedule.scheduleType === 'manual' ? '' : schedule.scheduleExpr,
    targetType: handoff.targetType,
    targetName: handoff.targetName,
    input,
    approved: true,
    enabled: false
  };
}

export function schedulerHandoffInputJSON(handoff: SchedulerHandoff) {
  const current = String(handoff.inputJSON || '').trim();
  if (current && current !== '{}') return current;
  return schedulerHandoffDefaultInputJSON(handoff);
}

export function schedulerHandoffDefaultInputJSON(handoff: SchedulerHandoff) {
  if (String(handoff.targetType || '').trim() !== 'extension') return '{}';
  const input: Record<string, string> = {};
  const url = firstURL(handoff.request);
  if (url) input.url = url;
  const task = schedulerHandoffDefaultTask(handoff);
  if (task) input.task = task;
  return Object.keys(input).length ? JSON.stringify(input, null, 2) : '{}';
}

function schedulerHandoffDefaultTask(handoff: SchedulerHandoff) {
  const request = String(handoff.request || '').replace(/\s+/g, ' ').trim();
  if (request && !request.toLowerCase().startsWith('scheduler_setup:')) {
    return request.length > 220 ? `${request.slice(0, 220)}...` : request;
  }
  const target = humanizeIdentifier(handoff.targetName, 'scheduled task');
  return `Run ${target} and report the result.`;
}

function firstURL(value: string) {
  const match = String(value || '').match(/https?:\/\/[^\s"'<>]+/i);
  return match ? match[0].replace(/[),.;]+$/, '') : '';
}

export function normalizeSchedulerSchedule(scheduleType: string | undefined, scheduleExpr: string | undefined) {
  const type = String(scheduleType || 'manual').trim().toLowerCase();
  const expr = String(scheduleExpr || '').trim();
  const normalizedExpr = expr.toLowerCase().replace(/\s+/g, ' ');
  if (type === 'cron' && looksLikeIntervalExpression(normalizedExpr)) {
    return { scheduleType: 'interval', scheduleExpr: normalizeIntervalExpression(normalizedExpr) };
  }
  if ((type === 'interval' || !type) && normalizedExpr) {
    return { scheduleType: 'interval', scheduleExpr: normalizeIntervalExpression(normalizedExpr) };
  }
  return { scheduleType: type || 'manual', scheduleExpr: expr };
}

function looksLikeIntervalExpression(value: string) {
  if (!value) return false;
  if (/^\d+(\.\d+)?(ms|s|m|h)$/.test(value)) return true;
  return ['hour', '1 hour', 'one hour', 'every hour', 'hourly', 'day', '1 day', 'one day', 'every day', 'daily', 'week', '1 week', 'one week', 'every week', 'weekly'].includes(value);
}

function normalizeIntervalExpression(value: string) {
  const everyMatch = value.match(/^every\s+(\d+)\s+(minute|minutes|hour|hours|day|days|week|weeks)$/);
  if (everyMatch) {
    const amount = Number(everyMatch[1]);
    const unit = everyMatch[2];
    if (unit.startsWith('minute')) return `${amount}m`;
    if (unit.startsWith('hour')) return `${amount}h`;
    if (unit.startsWith('day')) return `${amount * 24}h`;
    if (unit.startsWith('week')) return `${amount * 24 * 7}h`;
  }
  switch (value) {
    case 'hour':
    case '1 hour':
    case 'one hour':
    case 'every hour':
    case 'hourly':
      return '1h';
    case 'day':
    case '1 day':
    case 'one day':
    case 'every day':
    case 'daily':
      return '24h';
    case 'week':
    case '1 week':
    case 'one week':
    case 'every week':
    case 'weekly':
      return '168h';
    default:
      return value.replace(/^every\s+/, '');
  }
}

export function parseSchedulerJobInput(value: string | undefined, label: string): Record<string, unknown> {
  const trimmed = String(value || '').trim();
  if (!trimmed) return {};
  const parsed = JSON.parse(trimmed) as unknown;
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(`${label} must be a JSON object`);
  }
  return parsed as Record<string, unknown>;
}

export function jsonObjectInputError(value: string | undefined, label: string) {
  try {
    parseSchedulerJobInput(value, label);
    return '';
  } catch (err) {
    return err instanceof Error ? err.message : String(err);
  }
}

export function schedulerJobUpdateInputFromJSON(id: string, inputJSON: string): SchedulerJobUpdateInput {
  return {
    id,
    input: parseSchedulerJobInput(inputJSON, 'Job input'),
    approved: true
  };
}

export function schedulerJobInputJSON(job: Job) {
  try {
    return JSON.stringify(job.input ?? {}, null, 2);
  } catch {
    return '{}';
  }
}

export function schedulerJobFailureTitle(job: Job, message: string) {
  const lower = String(message || '').toLowerCase();
  if (job.targetType === 'extension' && (lower.includes('extension input') || lower.includes('input.'))) {
    return 'Stored extension job input is invalid';
  }
  if (lower.includes('updatejobinput') || lower.includes('/api/jobs/input') || lower.includes('unsupported local web call')) {
    return 'Job input update is not available yet';
  }
  return 'Job operation failed';
}

export function schedulerJobCreatedSummary(job: Job) {
  return `created ${job.id}`;
}

export function schedulerJobCreatedMessage(job: Job) {
  return `Created ${job.id}. It is disabled until you enable it.`;
}

export function schedulerJobConversationId(job: Job | undefined) {
  const input = job?.input ?? {};
  for (const key of ['conversation_id', 'conversationId', '_conversation_id']) {
    const raw = input[key];
    if (typeof raw === 'string' && raw.trim()) return raw.trim();
  }
  return '';
}

export function schedulerJobOriginLabel(job: Job | undefined) {
  return schedulerJobConversationId(job) ? 'Chat origin recorded' : 'No chat origin';
}

export function schedulerJobRunOutputSummary(output: unknown) {
  if (output === null || output === undefined || output === '') return 'No output was returned.';
  if (typeof output === 'string') {
    const parsed = parseJSONObject(output);
    if (parsed) return schedulerJobRunOutputSummary(parsed);
    return compactRunSummary(output);
  }
  if (typeof output !== 'object' || Array.isArray(output)) return compactRunSummary(String(output));

  const values = output as Record<string, unknown>;
  const direct = firstTextField(values, ['summary', 'message', 'text']);
  if (direct) return compactRunSummary(direct);

  const nested = objectField(values.output);
  if (nested) {
    const nestedText = firstTextField(nested, ['summary', 'message', 'text', 'status']);
    if (nestedText) return compactRunSummary(nestedText);
  }

  const status = firstTextField(values, ['status']);
  const extension = firstTextField(values, ['extension']);
  if (status && extension) return `${humanizeIdentifier(extension, extension)} finished with status ${humanizeIdentifier(status, status)}.`;
  if (status) return `Job finished with status ${humanizeIdentifier(status, status)}.`;
  return 'Output is available in details.';
}

function parseJSONObject(value: string) {
  const trimmed = value.trim();
  if (!trimmed.startsWith('{')) return null;
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    return objectField(parsed);
  } catch {
    return null;
  }
}

function objectField(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null;
}

function firstTextField(values: Record<string, unknown>, keys: string[]) {
  for (const key of keys) {
    const value = values[key];
    if (typeof value === 'string' && value.trim()) return value.trim();
    if (typeof value === 'number' || typeof value === 'boolean') return String(value);
  }
  return '';
}

function compactRunSummary(value: string, limit = 260) {
  const clean = value.replace(/\s+/g, ' ').trim();
  if (!clean) return 'No output was returned.';
  return clean.length > limit ? `${clean.slice(0, limit)}...` : clean;
}
