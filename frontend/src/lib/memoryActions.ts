import type { MemoryResult } from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';

export type MemoryWriteInput = {
  kind: string;
  content: string;
  importance: number | string;
};

export async function searchMemoryResults(query: string) {
  return asArray(await call<MemoryResult[]>('SearchMemory', query));
}

export async function listMemoryResults(limit = 50) {
  return asArray(await call<MemoryResult[]>('ListMemories', limit));
}

export async function writeMemoryItem(input: MemoryWriteInput) {
  return await call<MemoryResult>('WriteMemory', {
    kind: input.kind,
    content: input.content,
    importance: Number(input.importance)
  });
}

export async function pinMemoryItem(id: string) {
  await call<void>('PinMemory', id);
}

export async function deleteMemoryItem(id: string) {
  await call<void>('DeleteMemory', id);
}

export function mergeWrittenMemory(results: MemoryResult[] | null | undefined, item: MemoryResult) {
  return [item, ...asArray(results).filter((result) => result.id !== item.id)];
}

export function removeMemoryResult(results: MemoryResult[] | null | undefined, id: string) {
  return asArray(results).filter((result) => result.id !== id);
}

export function memoryCountLabel(prefix: string, count: number) {
  if (prefix === 'search') return `memory search: ${count} match${count === 1 ? '' : 'es'}`;
  return `memory list: ${count} item${count === 1 ? '' : 's'}`;
}

export function savedMemoryLabel(item: MemoryResult) {
  return `memory saved: ${item.kind || item.role}`;
}
