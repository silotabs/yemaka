import type { Status } from './appTypes';
import type { NormalizedAgentStreamEvent } from './handoffHelpers';
import { sourceKindLabel } from './uiHelpers';

export type AgentStreamEventEffect = {
  activity?: string;
  trace?: string;
  statusPatch?: Partial<Status>;
  capabilityHandoffData?: Record<string, string>;
  schedulerHandoffData?: Record<string, string>;
  permissionData?: Record<string, string>;
  editProposalData?: Record<string, string>;
};

export function agentStreamEventEffect(event: NormalizedAgentStreamEvent): AgentStreamEventEffect {
  if (event.type === 'model.selected' && event.data?.model) {
    return {
      activity: `model selected: ${event.data.model}`,
      trace: `model: ${event.data.model}`,
      statusPatch: { modelReady: true, selectedModel: String(event.data.model) }
    };
  }
  if (event.type === 'workspace.used') {
    const kind = sourceKindLabel(event.data?.source_kind || 'workspace');
    const count = String(event.data?.sources || '')
      .split(',')
      .map((source) => source.trim())
      .filter(Boolean).length;
    return {
      activity: `${kind} context: ${count || 'local'} source${count === 1 ? '' : 's'}`,
      trace: `${kind}: ${count || 'local'} source${count === 1 ? '' : 's'}`
    };
  }
  if (event.type === 'rag.used') {
    return {
      activity: 'RAG context used',
      trace: 'RAG: retrieved local context',
      statusPatch: { ragEnabled: true }
    };
  }
  if (event.type === 'memory.used') {
    const count = event.data?.count || event.data?.memories || '';
    return {
      activity: `memory used${count ? `: ${count}` : ''}`,
      trace: `memory: ${count ? `${count} relevant item${count === '1' ? '' : 's'}` : 'relevant context'}`
    };
  }
  if (event.type === 'model.tool_call') {
    const names = event.data?.names || `${event.data?.count || '0'} call(s)`;
    return {
      activity: `model tool call: ${names}`,
      trace: `model tool call: ${names}`
    };
  }
  if (event.type === 'task.classified' && event.data?.task_type) {
    return {
      activity: `task: ${event.data.task_type} (${event.data.risk_level || 'low'} risk)`,
      trace: `task: ${event.data.task_type} (${event.data.risk_level || 'low'} risk)`
    };
  }
  if (event.type === 'routing.advisory') {
    const route = event.data?.suggested_route || 'clarification';
    const status = event.data?.status || 'observed';
    return {
      activity: `route advisory: ${status} ${route}`,
      trace: `route advisory: ${status} ${route}`
    };
  }
  if (event.type === 'plan.created' && event.data?.goal) {
    return {
      activity: `plan: ${event.data.goal}`,
      trace: `plan: ${event.data.goal}`
    };
  }
  if (event.type === 'execution.decided' && event.data?.status) {
    const toolName = event.data.tool_name || event.data.action || '';
    return {
      activity: `executor: ${event.data.status}${event.data.tool_name ? ` ${event.data.tool_name}` : ''}`,
      trace: `executor: ${event.data.status}${toolName ? ` ${toolName}` : ''}`
    };
  }
  if (event.type === 'capability.gap') {
    const title = event.data?.title || event.message || 'capability gap';
    return {
      capabilityHandoffData: event.data,
      activity: `capability gap: ${title}`,
      trace: `capability: ${title}`
    };
  }
  if (event.type === 'scheduler.job_proposal') {
    const target = event.data?.target_name || event.data?.targetName || 'scheduler job';
    return {
      schedulerHandoffData: event.data,
      activity: `scheduler proposal: ${target}`,
      trace: `scheduler proposal: ${target}`
    };
  }
  if (event.type === 'permission.requested' && event.data?.tool_name) {
    return {
      permissionData: event.data,
      activity: `permission: ${event.data.tool_name} (${event.data.risk_level || 'medium'} risk)`,
      trace: `permission needed: ${event.data.tool_name}`
    };
  }
  if (event.type === 'edit.proposed' && event.data?.path) {
    return {
      editProposalData: event.data,
      activity: `edit proposal: ${event.data.path}`,
      trace: `edit proposed: ${event.data.path}`
    };
  }
  if (event.type === 'tool.completed' && event.data?.tool_name) {
    return {
      activity: `tool completed: ${event.data.tool_name} (${event.data.status || 'completed'})`,
      trace: `tool: ${event.data.tool_name} (${event.data.status || 'completed'})`,
      statusPatch: { shellEnabled: true }
    };
  }
  if (event.type === 'skill.selected' && event.data?.name) {
    return {
      activity: `skill selected: ${event.data.name}`,
      trace: `skill: ${event.data.name}`
    };
  }
  if (event.type === 'cloud_fallback.used' && event.data?.model) {
    return {
      activity: `cloud fallback: ${event.data.model}`,
      trace: `cloud fallback: ${event.data.model}`
    };
  }
  if (event.type === 'verification.completed' && event.data?.status) {
    return {
      activity: `verification: ${event.data.status}`,
      trace: `verification: ${event.data.status}`
    };
  }
  return {};
}
