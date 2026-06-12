import type {
  DocumentInventoryItem,
  DocumentPathSuggestion,
  DocumentPruneResult,
  DocumentResult,
  WorkspaceGrant
} from './appTypes';
import {
  documentSearchLabel,
  documentSuggestionSummary,
  grantWorkspacePath,
  ingestDocumentPath,
  listDocumentInventory,
  listWorkspaceGrants,
  mergeWorkspaceGrant,
  pickDocumentFileSource,
  pickWorkspaceFolderGrant,
  pruneMissingDocuments,
  removeWorkspaceGrant,
  revokeWorkspaceGrantPath,
  searchDocumentResults,
  suggestDocumentPaths,
  uploadDocumentFileToServer,
  uploadDocumentFolderToServer
} from './documentActions';
import { documentIngestErrorSummary, summarizeDocumentPrune, summarizeIngestResult } from './documentHelpers';
import {
  documentFileFromEvent,
  documentFolderFilesFromEvent,
  documentPickerUnavailableSummary,
  documentSuggestionState,
  emptyDocumentSuggestionState,
  selectedDocumentSuggestionState,
  uploadedDocumentFileSummary,
  uploadedDocumentFolderSummary,
  type DocumentSuggestionViewState
} from './documentPageHelpers';

export type DocumentControllerContext = {
  getPath: () => string;
  setPath: (path: string) => void;
  getQuery: () => string;
  getGrants: () => WorkspaceGrant[];
  setGrants: (grants: WorkspaceGrant[]) => void;
  setResults: (results: DocumentResult[]) => void;
  setInventory: (items: DocumentInventoryItem[]) => void;
  setInventoryBusy: (busy: boolean) => void;
  setPruneBusy: (busy: boolean) => void;
  setPrunePreview: (result: DocumentPruneResult | null) => void;
  setPruneSummary: (summary: string) => void;
  setIngestSummary: (summary: string) => void;
  setWorkspaceGrantSummary: (summary: string) => void;
  setSuggestionState: (state: DocumentSuggestionViewState) => void;
  setSuggestionOpen: (open: boolean) => void;
  setError: (message: string) => void;
  isLocalWeb: () => boolean;
  desktopPickerAvailable: () => boolean;
  desktopFilePickerAvailable: () => boolean;
  pushActivity: (line: string) => void;
};

export function createDocumentController(ctx: DocumentControllerContext) {
  let suggestionTimer: ReturnType<typeof setTimeout> | undefined;
  let suggestionSeq = 0;

  const clearSuggestions = (summary = '') => ctx.setSuggestionState(emptyDocumentSuggestionState(summary));

  async function refreshDocumentInventory(resetPrune = true, silent = false) {
    if (!silent) {
      ctx.setError('');
      ctx.setInventoryBusy(true);
    }
    try {
      const inventory = await listDocumentInventory(100);
      ctx.setInventory(inventory);
      if (resetPrune) {
        ctx.setPrunePreview(null);
        ctx.setPruneSummary('');
      }
      if (!silent) ctx.pushActivity(`document inventory: ${inventory.length}`);
    } catch (err) {
      if (!silent) ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      if (!silent) ctx.setInventoryBusy(false);
    }
  }

  async function refreshWorkspaceGrants(silent = false) {
    if (!silent) {
      ctx.setError('');
      ctx.setWorkspaceGrantSummary('');
    }
    try {
      const grants = await listWorkspaceGrants();
      ctx.setGrants(grants);
      if (!silent) ctx.pushActivity(`workspace grants: ${grants.length}`);
    } catch (err) {
      if (!silent) ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function refreshDocumentsSurface(silent = false) {
    await Promise.all([refreshWorkspaceGrants(silent), refreshDocumentInventory(!silent, silent)]);
  }

  async function ingestDocuments() {
    ctx.setError('');
    ctx.setIngestSummary('');
    ctx.setSuggestionOpen(false);
    const path = ctx.getPath();
    try {
      const result = await ingestDocumentPath(path);
      ctx.setIngestSummary(summarizeIngestResult(result));
      ctx.pushActivity(`ingested ${path}`);
      await refreshDocumentInventory();
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      ctx.setIngestSummary(documentIngestErrorSummary(path, message, ctx.isLocalWeb()));
      ctx.setError(message);
    }
  }

  async function previewPruneMissingDocuments() {
    ctx.setError('');
    ctx.setPruneSummary('');
    ctx.setPrunePreview(null);
    try {
      ctx.setPruneBusy(true);
      const result = await pruneMissingDocuments(false);
      ctx.setPrunePreview(result);
      ctx.setPruneSummary(summarizeDocumentPrune(result));
      ctx.pushActivity(`RAG prune preview: ${result.documentsMatched} missing`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setPruneBusy(false);
    }
  }

  async function applyPruneMissingDocuments() {
    ctx.setError('');
    ctx.setPruneSummary('');
    try {
      ctx.setPruneBusy(true);
      const result = await pruneMissingDocuments(true);
      ctx.setPrunePreview(result);
      ctx.setPruneSummary(summarizeDocumentPrune(result));
      ctx.pushActivity(`RAG pruned: ${result.documentsRemoved} documents`);
      await refreshDocumentInventory(false);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    } finally {
      ctx.setPruneBusy(false);
    }
  }

  function scheduleDocumentPathSuggestions() {
    if (suggestionTimer) clearTimeout(suggestionTimer);
    const value = ctx.getPath().trim();
    if (!value) {
      clearSuggestions();
      return;
    }
    suggestionTimer = setTimeout(() => {
      void refreshDocumentPathSuggestions();
    }, 180);
  }

  async function refreshDocumentPathSuggestions() {
    const value = ctx.getPath().trim();
    const seq = ++suggestionSeq;
    ctx.setSuggestionState({ ...emptyDocumentSuggestionState(), summary: '' });
    if (!value) {
      clearSuggestions();
      return;
    }
    try {
      const suggestions = await suggestDocumentPaths(value, 40);
      if (seq !== suggestionSeq) return;
      ctx.setSuggestionState(documentSuggestionState(suggestions, documentSuggestionSummary(suggestions.length)));
    } catch (err) {
      if (seq !== suggestionSeq) return;
      clearSuggestions(err instanceof Error ? `suggestions unavailable: ${err.message}` : 'suggestions unavailable');
    }
  }

  function selectDocumentPathSuggestion(item: DocumentPathSuggestion) {
    ctx.setPath(item.path);
    ctx.setSuggestionState(selectedDocumentSuggestionState(item));
  }

  async function pickWorkspaceFolder() {
    ctx.setError('');
    ctx.setWorkspaceGrantSummary('');
    if (!ctx.desktopPickerAvailable()) {
      ctx.setWorkspaceGrantSummary(documentPickerUnavailableSummary('folder', ctx.isLocalWeb()));
      return;
    }
    try {
      const grant = await pickWorkspaceFolderGrant();
      ctx.setPath(grant.path);
      clearSuggestions();
      ctx.setGrants(mergeWorkspaceGrant(ctx.getGrants(), grant));
      ctx.setWorkspaceGrantSummary(`granted ${grant.path}`);
      ctx.pushActivity(`workspace granted: ${grant.label || grant.path}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function pickDocumentFile() {
    ctx.setError('');
    ctx.setWorkspaceGrantSummary('');
    if (!ctx.desktopFilePickerAvailable()) {
      ctx.setWorkspaceGrantSummary(documentPickerUnavailableSummary('file', ctx.isLocalWeb()));
      return;
    }
    try {
      const result = await pickDocumentFileSource();
      const grant = result.grant;
      ctx.setPath(result.path);
      clearSuggestions();
      ctx.setGrants(mergeWorkspaceGrant(ctx.getGrants(), grant));
      ctx.setWorkspaceGrantSummary(`selected ${result.path}; granted ${grant.path}`);
      ctx.pushActivity(`document selected: ${result.path}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function uploadDocumentFile(file: File) {
    ctx.setError('');
    ctx.setIngestSummary('');
    ctx.setWorkspaceGrantSummary('');
    ctx.setSuggestionOpen(false);
    try {
      const result = await uploadDocumentFileToServer(file);
      clearSuggestions();
      ctx.setIngestSummary(uploadedDocumentFileSummary(file, result));
      ctx.pushActivity(`document uploaded: ${file.name}`);
      await refreshDocumentInventory();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function uploadDocumentFolder(files: File[]) {
    ctx.setError('');
    ctx.setIngestSummary('');
    ctx.setWorkspaceGrantSummary('');
    ctx.setSuggestionOpen(false);
    try {
      const result = await uploadDocumentFolderToServer(files);
      clearSuggestions();
      ctx.setIngestSummary(uploadedDocumentFolderSummary(result));
      ctx.pushActivity(`folder uploaded: ${result.folderName}`);
      await refreshDocumentInventory();
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleDocumentFileSelected(event: Event) {
    const file = documentFileFromEvent(event);
    if (!file) return;
    await uploadDocumentFile(file);
  }

  async function handleDocumentFolderSelected(event: Event) {
    const files = documentFolderFilesFromEvent(event);
    if (files.length === 0) return;
    await uploadDocumentFolder(files);
  }

  async function grantDocumentPath() {
    ctx.setError('');
    ctx.setWorkspaceGrantSummary('');
    try {
      const grant = await grantWorkspacePath(ctx.getPath());
      if (!ctx.getPath().trim()) ctx.setPath(grant.path);
      clearSuggestions();
      ctx.setGrants(mergeWorkspaceGrant(ctx.getGrants(), grant));
      ctx.setWorkspaceGrantSummary(`granted ${grant.path}`);
      ctx.pushActivity(`workspace granted: ${grant.label || grant.path}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function revokeWorkspaceGrant(match: string) {
    ctx.setError('');
    ctx.setWorkspaceGrantSummary('');
    try {
      const grant = await revokeWorkspaceGrantPath(match);
      ctx.setGrants(removeWorkspaceGrant(ctx.getGrants(), grant));
      ctx.setWorkspaceGrantSummary(`revoked ${grant.path}`);
      ctx.pushActivity(`workspace revoked: ${grant.label || grant.path}`);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function searchDocuments() {
    ctx.setError('');
    try {
      const results = await searchDocumentResults(ctx.getQuery());
      ctx.setResults(results);
      ctx.pushActivity(documentSearchLabel(results.length));
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    ingestDocuments,
    refreshDocumentsSurface,
    refreshDocumentInventory,
    previewPruneMissingDocuments,
    applyPruneMissingDocuments,
    scheduleDocumentPathSuggestions,
    refreshDocumentPathSuggestions,
    selectDocumentPathSuggestion,
    refreshWorkspaceGrants,
    pickWorkspaceFolder,
    pickDocumentFile,
    handleDocumentFileSelected,
    handleDocumentFolderSelected,
    uploadDocumentFile,
    uploadDocumentFolder,
    grantDocumentPath,
    revokeWorkspaceGrant,
    searchDocuments
  };
}
