export type PermissionItemLike = {
  requestId: string;
  toolName: string;
  command: string[];
  riskLevel: string;
  reason: string;
  nextStep: string;
  requiresConfirmation: boolean;
  workspaceOnly: boolean;
  diffPreview: boolean;
  snapshotBeforeWrite: boolean;
  rollbackSupported: boolean;
  destructive: boolean;
  policyLevel: string;
  policyExplanation: string;
  status: string;
};

export type ReplayTraceExplanationLike = {
  route?: {
    category?: string;
    intent?: string;
    domain?: string;
    riskLevel?: string;
    needsApproval?: boolean;
  };
  toolsUsed?: Array<{ name?: string }>;
};

export function permissionCommand(data: Record<string, string>) {
  const encoded = data.command_json || '';
  if (encoded) {
    try {
      const parsed = JSON.parse(encoded);
      if (Array.isArray(parsed)) {
        return parsed.map((value) => String(value));
      }
    } catch {
      // Fall through to the display-only command.
    }
  }
  const command = data.command || '';
  return command ? command.split(' ').filter(Boolean) : [];
}

export function permissionCommandPreview(item: PermissionItemLike) {
  if (!item.command?.length) return '';
  if (item.toolName === 'memory_write' && item.command.length >= 3) {
    return `${item.command[1]}: ${item.command.slice(2).join(' ')}`;
  }
  return item.command.join(' ');
}

export function permissionActionDisabled(item: PermissionItemLike, busyByRequestId: Record<string, boolean>) {
  return item.status !== 'pending' || Boolean(busyByRequestId[item.requestId]);
}

export function permissionDecisionErrorIsStale(message: string) {
  const lower = message.toLowerCase();
  return lower.includes('was already decided') || (lower.includes('pending permission request') && lower.includes('was not found'));
}

export function permissionPayload(item: PermissionItemLike, decision: 'approved' | 'rejected') {
  return {
    request: {
      request_id: item.requestId,
      tool_name: item.toolName,
      command: item.command,
      risk_level: item.riskLevel,
      reason: item.reason,
      requires_confirmation: item.requiresConfirmation,
      workspace_only: item.workspaceOnly,
      diff_preview: item.diffPreview,
      snapshot_before_write: item.snapshotBeforeWrite,
      rollback_supported: item.rollbackSupported,
      destructive: item.destructive,
      policy_level: Number(item.policyLevel || 0),
      policy_explanation: item.policyExplanation,
      next_step: item.nextStep
    },
    decision
  };
}

export function replayRouteSummary(explanation: ReplayTraceExplanationLike | null) {
  const route = explanation?.route;
  if (!route) return 'No route summary available.';
  const parts = [route.category, route.intent, route.domain, route.riskLevel ? `${route.riskLevel} risk` : '', route.needsApproval ? 'approval required' : '']
    .map((part) => String(part || '').trim())
    .filter(Boolean);
  return parts.length ? parts.join(' · ') : 'No route summary available.';
}

export function replayToolSummary(explanation: ReplayTraceExplanationLike | null) {
  const used = (explanation?.toolsUsed ?? [])
    .map((tool) => tool.name || '')
    .filter(Boolean);
  return used.length ? used.join(', ') : 'No tools used.';
}
