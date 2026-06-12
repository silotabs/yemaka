export type IngestSummaryLike = {
  filesIndexed?: number;
  filesSkipped?: number;
  filesUnchanged?: number;
  chunksCreated?: number;
  skippedReasons?: string[];
};

export type DocumentPruneLike = {
  dryRun?: boolean;
  documentsMatched?: number;
  documentsRemoved?: number;
  chunksMatched?: number;
};

export type InventoryStatusLike = {
  status?: string;
};

export function summarizeIngestResult(result: IngestSummaryLike) {
  const skipped = result.filesSkipped ? `, ${result.filesSkipped} skipped` : '';
  const reasons = result.skippedReasons?.length ? `; ${result.skippedReasons.slice(0, 2).join('; ')}` : '';
  return `${result.filesIndexed || 0} indexed, ${result.filesUnchanged || 0} unchanged, ${result.chunksCreated || 0} chunks${skipped}${reasons}`;
}

export function documentIngestErrorSummary(path: string, message: string, localWeb = false) {
  const cleanPath = (path || '').trim() || 'the selected path';
  const lower = message.toLowerCase();
  if (lower.includes('workspace path is not granted') || lower.includes('outside workspace')) {
    if (localWeb) {
      return `Path not granted: ${cleanPath}. Paste the full local path, use Grant Path, then run Ingest again. Browser Upload indexes a copy directly and does not grant filesystem access.`;
    }
    return `Path not granted: ${cleanPath}. Use Grant Path, Pick File, or Pick Folder first, then run Ingest again.`;
  }
  return message;
}

export function summarizeDocumentPrune(result: DocumentPruneLike | null) {
  if (!result) return '';
  const action = result.dryRun ? 'preview' : 'applied';
  return `${action}: ${result.documentsMatched || 0} missing, ${result.documentsRemoved || 0} removed, ${result.chunksMatched || 0} chunks matched`;
}

export function inventoryCount(items: InventoryStatusLike[] | null | undefined, status: string) {
  return (items ?? []).filter((item) => item.status === status).length;
}

export function uploadedRelativePath(file: File) {
  const webkitPath = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
  return webkitPath && webkitPath.trim() ? webkitPath : file.name;
}
