import type { DocumentPathSuggestion, IngestSummaryResult } from './appTypes';
import type { FolderUploadResult } from './documentActions';
import { summarizeIngestResult } from './documentHelpers';

export type DocumentSuggestionViewState = {
  suggestions: DocumentPathSuggestion[];
  open: boolean;
  summary: string;
};

export function directoryInput(node: HTMLInputElement) {
  node.setAttribute('webkitdirectory', '');
  node.setAttribute('directory', '');
  return {
    destroy() {
      node.removeAttribute('webkitdirectory');
      node.removeAttribute('directory');
    }
  };
}

export function documentFileFromEvent(event: Event) {
  const input = event.currentTarget as HTMLInputElement;
  const file = input.files?.[0] ?? null;
  input.value = '';
  return file;
}

export function documentFolderFilesFromEvent(event: Event) {
  const input = event.currentTarget as HTMLInputElement;
  const files = Array.from(input.files ?? []);
  input.value = '';
  return files;
}

export function emptyDocumentSuggestionState(summary = ''): DocumentSuggestionViewState {
  return {
    suggestions: [],
    open: false,
    summary
  };
}

export function documentSuggestionState(suggestions: DocumentPathSuggestion[], summary: string): DocumentSuggestionViewState {
  return {
    suggestions,
    open: suggestions.length > 0,
    summary: suggestions.length > 0 ? summary : ''
  };
}

export function selectedDocumentSuggestionState(item: DocumentPathSuggestion): DocumentSuggestionViewState {
  return emptyDocumentSuggestionState(`selected ${item.name}`);
}

export function documentPickerUnavailableSummary(kind: 'file' | 'folder', localWeb: boolean) {
  if (kind === 'file') {
    if (localWeb) {
      return 'Web upload indexes a browser-selected file copy for RAG. To grant an existing file path, paste its full local path and use Grant Path.';
    }
    return 'Native file browsing is available in the desktop app. In the browser, paste the file path, then use Grant Path or Ingest.';
  }
  if (localWeb) {
    return 'Web upload indexes a browser-selected folder copy for RAG. To grant an existing folder path, paste its full local path and use Grant Path.';
  }
  return 'Native folder browsing is available in the desktop app. In the browser, paste a folder path, then use Grant Path or Ingest.';
}

export function uploadedDocumentFileSummary(file: File, result: IngestSummaryResult) {
  return `uploaded ${file.name}: ${summarizeIngestResult(result)}. The uploaded copy is indexed for Documents search and chat/RAG; Grant Path is only needed when ingesting a real local path.`;
}

export function uploadedDocumentFolderSummary(result: FolderUploadResult) {
  return `uploaded folder ${result.folderName}: ${summarizeIngestResult(result.summary)}. The uploaded copy is indexed for Documents search and chat/RAG; Grant Path is only needed when ingesting a real local path.`;
}
