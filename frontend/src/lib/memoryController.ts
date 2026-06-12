import type { MemoryResult } from './appTypes';
import {
  deleteMemoryItem,
  listMemoryResults,
  memoryCountLabel,
  mergeWrittenMemory,
  pinMemoryItem,
  removeMemoryResult,
  savedMemoryLabel,
  searchMemoryResults,
  writeMemoryItem
} from './memoryActions';

export type MemoryControllerContext = {
  getQuery: () => string;
  getKind: () => string;
  getContent: () => string;
  getImportance: () => number | string;
  getResults: () => MemoryResult[];
  setContent: (content: string) => void;
  setResults: (results: MemoryResult[]) => void;
  setSummary: (summary: string) => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
};

export function createMemoryController(ctx: MemoryControllerContext) {
  async function searchMemory() {
    ctx.setError('');
    try {
      const results = await searchMemoryResults(ctx.getQuery());
      ctx.setResults(results);
      const summary = memoryCountLabel('search', results.length);
      ctx.setSummary(summary);
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function listMemories(silent = false) {
    if (!silent) ctx.setError('');
    try {
      const results = await listMemoryResults(50);
      ctx.setResults(results);
      if (!silent) {
        const summary = memoryCountLabel('list', results.length);
        ctx.setSummary(summary);
        ctx.pushActivity(summary);
      }
    } catch (err) {
      if (!silent) ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function writeMemory() {
    ctx.setError('');
    try {
      const item = await writeMemoryItem({
        kind: ctx.getKind(),
        content: ctx.getContent(),
        importance: ctx.getImportance()
      });
      ctx.setContent('');
      ctx.setResults(mergeWrittenMemory(ctx.getResults(), item));
      const summary = savedMemoryLabel(item);
      ctx.setSummary(summary);
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function pinMemory(id: string) {
    ctx.setError('');
    try {
      await pinMemoryItem(id);
      await listMemories(true);
      const summary = `memory pinned: ${id}`;
      ctx.setSummary(summary);
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function deleteMemory(id: string) {
    ctx.setError('');
    try {
      await deleteMemoryItem(id);
      ctx.setResults(removeMemoryResult(ctx.getResults(), id));
      const summary = `memory deleted: ${id}`;
      ctx.setSummary(summary);
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    searchMemory,
    listMemories,
    writeMemory,
    pinMemory,
    deleteMemory
  };
}
