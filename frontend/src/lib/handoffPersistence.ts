import type { CapabilityHandoff, SchedulerHandoff } from './appTypes';

type HandoffMessageIdentity = {
  id?: string;
  parentId?: string;
};

export type PersistedMessageHandoffs = {
  capabilityHandoff?: CapabilityHandoff;
  schedulerHandoff?: SchedulerHandoff;
};

const handoffPrefix = 'yemaka.message-handoff.v1.';

function canUseLocalStorage() {
  return typeof localStorage !== 'undefined';
}

function cleanKeyPart(value: string) {
  return String(value || '')
    .trim()
    .replace(/[^a-zA-Z0-9:_-]+/g, '_')
    .slice(0, 180);
}

function storageKeysForMessage(message: HandoffMessageIdentity): string[] {
  const keys: string[] = [];
  if (message.id) keys.push(`${handoffPrefix}assistant:${cleanKeyPart(message.id)}`);
  if (message.parentId) keys.push(`${handoffPrefix}parent:${cleanKeyPart(message.parentId)}`);
  return [...new Set(keys)];
}

function readHandoffKey(key: string): PersistedMessageHandoffs {
  if (!canUseLocalStorage()) return {};
  try {
    const raw = localStorage.getItem(key);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as PersistedMessageHandoffs;
    return {
      capabilityHandoff: parsed.capabilityHandoff,
      schedulerHandoff: parsed.schedulerHandoff
    };
  } catch {
    return {};
  }
}

function writeHandoffKey(key: string, value: PersistedMessageHandoffs) {
  if (!canUseLocalStorage()) return;
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Local persistence is best-effort UI recovery. Do not break chat if storage is full or unavailable.
  }
}

function mergeHandoffKey(key: string, patch: PersistedMessageHandoffs) {
  writeHandoffKey(key, { ...readHandoffKey(key), ...patch });
}

export function readPersistedMessageHandoffs(message: HandoffMessageIdentity): PersistedMessageHandoffs {
  for (const key of storageKeysForMessage(message)) {
    const handoffs = readHandoffKey(key);
    if (handoffs.capabilityHandoff || handoffs.schedulerHandoff) return handoffs;
  }
  return {};
}

export function persistCapabilityHandoffForMessage(message: HandoffMessageIdentity, handoff: CapabilityHandoff | undefined) {
  if (!handoff) return;
  for (const key of storageKeysForMessage(message)) {
    mergeHandoffKey(key, { capabilityHandoff: handoff });
  }
}

export function persistSchedulerHandoffForMessage(message: HandoffMessageIdentity, handoff: SchedulerHandoff | undefined) {
  if (!handoff) return;
  for (const key of storageKeysForMessage(message)) {
    mergeHandoffKey(key, { schedulerHandoff: handoff });
  }
}

export function persistMessageHandoffs(message: HandoffMessageIdentity & PersistedMessageHandoffs) {
  if (message.capabilityHandoff) {
    persistCapabilityHandoffForMessage(message, message.capabilityHandoff);
  }
  if (message.schedulerHandoff) {
    persistSchedulerHandoffForMessage(message, message.schedulerHandoff);
  }
}
