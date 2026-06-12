import type { OpsStatus } from './appTypes';
import { call } from './api';

export async function loadOpsStatus(limit = 20, includeRelease = false): Promise<OpsStatus> {
  try {
    return await call<OpsStatus>('OpsStatus', includeRelease, limit);
  } catch (err) {
    if (!opsCallNeedsDirectFetch(err)) {
      throw err;
    }
  }

  const params = new URLSearchParams();
  params.set('limit', String(limit));
  if (includeRelease) params.set('includeRelease', 'true');

  const response = await fetch(`/api/ops/status?${params.toString()}`);
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) {
    const message = payload && typeof payload === 'object' && 'error' in payload ? String(payload.error) : '';
    throw new Error(message || `/api/ops/status failed with HTTP ${response.status}`);
  }
  return payload as OpsStatus;
}

function opsCallNeedsDirectFetch(err: unknown) {
  const message = err instanceof Error ? err.message : String(err || '');
  return message.toLowerCase().includes('unsupported local web call: opsstatus');
}
