import type {
  DocumentInventoryItem,
  DocumentPathSuggestion,
  DocumentPruneResult,
  DocumentResult,
  DocumentSourceResult,
  IngestSummaryResult,
  WorkspaceGrant
} from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';
import { uploadedRelativePath } from './documentHelpers';

export async function ingestDocumentPath(path: string) {
  return await call<IngestSummaryResult>('IngestDocuments', path);
}

export async function listDocumentInventory(limit = 100) {
  return asArray(await call<DocumentInventoryItem[]>('ListDocumentInventory', limit));
}

export async function pruneMissingDocuments(apply: boolean) {
  return await call<DocumentPruneResult>('PruneMissingDocuments', apply);
}

export async function suggestDocumentPaths(query: string, limit = 40) {
  return asArray(await call<DocumentPathSuggestion[]>('SuggestDocumentPaths', query, limit));
}

export async function listWorkspaceGrants() {
  return asArray(await call<WorkspaceGrant[]>('ListWorkspaceGrants'));
}

export async function pickWorkspaceFolderGrant() {
  return await call<WorkspaceGrant>('PickWorkspaceFolder', '');
}

export async function pickDocumentFileSource() {
  return await call<DocumentSourceResult>('PickDocumentFile', '');
}

export async function grantWorkspacePath(path: string) {
  return await call<WorkspaceGrant>('GrantWorkspace', path, '');
}

export async function revokeWorkspaceGrantPath(match: string) {
  return await call<WorkspaceGrant>('RevokeWorkspaceGrant', match);
}

export async function searchDocumentResults(query: string) {
  return asArray(await call<DocumentResult[]>('SearchDocuments', query));
}

export async function uploadDocumentFileToServer(file: File, relativePath = '') {
  const form = new FormData();
  form.append('file', file, file.name);
  if (relativePath) form.append('relativePath', relativePath);
  const response = await fetch('/api/documents/upload', {
    method: 'POST',
    body: form
  });
  if (!response.ok) {
    let message = response.statusText || 'document upload failed';
    try {
      const payload = (await response.json()) as { error?: string };
      if (payload.error) message = payload.error;
    } catch {
      // Keep the HTTP status text when the server did not return JSON.
    }
    throw new Error(message);
  }
  return (await response.json()) as IngestSummaryResult;
}

export type FolderUploadResult = {
  folderName: string;
  summary: IngestSummaryResult;
};

export async function uploadDocumentFolderToServer(files: File[]): Promise<FolderUploadResult> {
  const totals: IngestSummaryResult = {
    root: 'browser_upload',
    filesIndexed: 0,
    filesSkipped: 0,
    filesUnchanged: 0,
    chunksCreated: 0,
    bytesIndexed: 0,
    skippedReasons: []
  };
  const firstRelativePath = uploadedRelativePath(files[0]);
  const folderName = firstRelativePath.includes('/') ? firstRelativePath.split('/')[0] : 'browser folder';
  for (const file of files) {
    const result = await uploadDocumentFileToServer(file, uploadedRelativePath(file));
    totals.filesIndexed += result.filesIndexed || 0;
    totals.filesSkipped += result.filesSkipped || 0;
    totals.filesUnchanged += result.filesUnchanged || 0;
    totals.chunksCreated += result.chunksCreated || 0;
    totals.bytesIndexed += result.bytesIndexed || 0;
    if (result.skippedReasons?.length) {
      totals.skippedReasons = [...(totals.skippedReasons ?? []), ...result.skippedReasons].slice(0, 8);
    }
  }
  return { folderName, summary: totals };
}

export function mergeWorkspaceGrant(grants: WorkspaceGrant[] | null | undefined, grant: WorkspaceGrant) {
  return [grant, ...asArray(grants).filter((item) => item.id !== grant.id)];
}

export function removeWorkspaceGrant(grants: WorkspaceGrant[] | null | undefined, grant: WorkspaceGrant) {
  return asArray(grants).filter((item) => item.id !== grant.id);
}

export function documentSearchLabel(count: number) {
  return `document search: ${count} match${count === 1 ? '' : 'es'}`;
}

export function documentSuggestionSummary(count: number) {
  return `${count} matching document${count === 1 ? '' : 's'}`;
}
