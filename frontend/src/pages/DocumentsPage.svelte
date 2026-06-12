<script lang="ts">
  import { DropdownMenu } from 'bits-ui';
  import ActionButton from '../ActionButton.svelte';
  import Badge from '../Badge.svelte';
  import EmptyState from '../EmptyState.svelte';
  import Icon from '../Icon.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import PaginationControls from '../PaginationControls.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import StructuredDataView from '../StructuredDataView.svelte';
  import { inventoryCount } from '../lib/documentHelpers';
  import { humanizeIdentifier } from '../lib/uiHelpers';
  import type { ToastKind } from '../lib/toastCenter';

  type DocumentResult = {
    chunkId: string;
    path: string;
    content: string;
    rank: number;
    score: number;
    source: string;
    explanation: string[];
  };

  type DocumentInventoryItem = {
    id: string;
    path: string;
    status: string;
    reason: string;
    missing: boolean;
    stale: boolean;
    sizeBytes: number;
    chunkCount: number;
  };

  type DocumentPruneResult = {
    dryRun: boolean;
  };

  type DocumentPathSuggestion = {
    path: string;
    name: string;
    directory: string;
    kind: string;
    sizeBytes: number;
  };

  type WorkspaceGrant = {
    id: string;
    path: string;
    label: string;
    source: string;
    bookmarkReady: boolean;
    bookmarkStale: boolean;
  };

  let documentFileInput: HTMLInputElement | null = null;
  let documentFolderInput: HTMLInputElement | null = null;

  export let localWeb = false;
  export let documentFileAccept = '';
  export let documentPath = './docs';
  export let documentPathSuggestions: DocumentPathSuggestion[] = [];
  export let documentPathSuggestionOpen = false;
  export let documentPathSuggestionSummary = '';
  export let documentPageStatus = '';
  export let documentPageStatusKind: ToastKind = 'info';
  export let documentInventory: DocumentInventoryItem[] = [];
  export let documentInventoryBusy = false;
  export let documentPruneBusy = false;
  export let documentPrunePreview: DocumentPruneResult | null = null;
  export let workspaceGrants: WorkspaceGrant[] = [];
  export let documentQuery = '';
  export let documentResults: DocumentResult[] = [];
  export let listPages: Record<string, number> = {};
  export let pageSizeFor: (key: string) => number = () => 10;
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let directoryInput: (node: HTMLInputElement) => { destroy?: () => void } = () => ({});
  export let handleDocumentFileSelected: (event: Event) => Promise<void> | void = () => {};
  export let handleDocumentFolderSelected: (event: Event) => Promise<void> | void = () => {};
  export let scheduleDocumentPathSuggestions: () => void = () => {};
  export let selectDocumentPathSuggestion: (item: DocumentPathSuggestion) => void = () => {};
  export let pickDocumentFile: () => Promise<void> | void = () => {};
  export let pickWorkspaceFolder: () => Promise<void> | void = () => {};
  export let grantDocumentPath: () => Promise<void> | void = () => {};
  export let ingestDocuments: () => Promise<void> | void = () => {};
  export let refreshDocumentInventory: () => Promise<void> | void = () => {};
  export let previewPruneMissingDocuments: () => Promise<void> | void = () => {};
  export let applyPruneMissingDocuments: () => Promise<void> | void = () => {};
  export let refreshWorkspaceGrants: () => Promise<void> | void = () => {};
  export let revokeWorkspaceGrant: (id: string) => Promise<void> | void = () => {};
  export let searchDocuments: () => Promise<void> | void = () => {};
  export let draftKnowledgeFromDocument: (result: DocumentResult) => Promise<void> | void = () => {};
  export let formatFileSize: (bytes: number) => string = (bytes) => `${bytes} B`;

</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  <div class={`document-source-card mb-4 rounded-md border border-line bg-white p-4 ${documentPathSuggestionOpen && documentPathSuggestions.length > 0 ? 'document-source-card-open' : ''}`}>
    <SectionHeader
      icon="documents"
      title="Document Source"
      description="Grant, pick, upload, or ingest local documents for search and chat context."
    />
    <input
      class="sr-only"
      type="file"
      accept={documentFileAccept}
      bind:this={documentFileInput}
      onchange={handleDocumentFileSelected}
    />
    <input
      class="sr-only"
      type="file"
      accept={documentFileAccept}
      multiple
      bind:this={documentFolderInput}
      use:directoryInput
      onchange={handleDocumentFolderSelected}
    />
    <div class="grid gap-3 lg:grid-cols-[minmax(0,1fr)_auto]">
      <div class="document-path-field">
        <input
          class="document-path-input"
          bind:value={documentPath}
          placeholder="./docs or ./docs/company.md"
          oninput={scheduleDocumentPathSuggestions}
          onfocus={scheduleDocumentPathSuggestions}
          onkeydown={(event) => {
            if (event.key === 'Escape') documentPathSuggestionOpen = false;
          }}
        />
        <button
          class="document-path-menu-trigger"
          type="button"
          disabled={documentPathSuggestions.length === 0}
          title="Matching documents"
          aria-label="Show matching document paths"
          aria-expanded={documentPathSuggestionOpen}
          onclick={() => (documentPathSuggestionOpen = !documentPathSuggestionOpen)}
        >
          <Icon name="documents" size={15} />
          <span>{documentPathSuggestions.length || 0}</span>
        </button>
        {#if documentPathSuggestionOpen && documentPathSuggestions.length > 0}
          <div class="document-path-suggestions" role="listbox" aria-label="Matching document paths">
            <div class="document-path-suggestions-label">Matching files</div>
            {#each documentPathSuggestions as item (item.path)}
              <button
                class="document-path-suggestion"
                type="button"
                role="option"
                aria-selected={documentPath === item.path}
                onmousedown={(event) => event.preventDefault()}
                onclick={() => selectDocumentPathSuggestion(item)}
              >
                <span class="document-path-suggestion-name">{item.name}</span>
                <span class="document-path-suggestion-meta">{humanizeIdentifier(item.kind, item.kind)} | {formatFileSize(item.sizeBytes)} | {item.directory}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
      <div class="flex flex-wrap gap-2">
        {#if localWeb}
          <DropdownMenu.Root>
            <DropdownMenu.Trigger class="document-upload-trigger" type="button">
              <Icon name="plus" size={15} />
              <span>Upload</span>
              <Icon name="chevron" size={15} />
            </DropdownMenu.Trigger>
            <DropdownMenu.Portal>
              <DropdownMenu.Content class="document-upload-menu bits-menu-content" sideOffset={8} align="end">
                <DropdownMenu.Item class="document-upload-item" onSelect={() => documentFileInput?.click()}>
                  <Icon name="documents" size={15} />
                  <span class="document-upload-item-copy">
                    <span>Upload file</span>
                    <small>Browser-selected copy</small>
                  </span>
                </DropdownMenu.Item>
                <DropdownMenu.Item class="document-upload-item" onSelect={() => documentFolderInput?.click()}>
                  <Icon name="plus" size={15} />
                  <span class="document-upload-item-copy">
                    <span>Upload folder copy</span>
                    <small>Browser confirmation follows</small>
                  </span>
                </DropdownMenu.Item>
              </DropdownMenu.Content>
            </DropdownMenu.Portal>
          </DropdownMenu.Root>
        {:else}
          <ActionButton
            variant="secondary"
            size="md"
            icon="documents"
            title="Pick a single document file"
            onclick={pickDocumentFile}
          >
            Pick File
          </ActionButton>
          <ActionButton
            variant="secondary"
            size="md"
            icon="plus"
            title="Pick a folder to grant and ingest"
            onclick={pickWorkspaceFolder}
          >
            Pick Folder
          </ActionButton>
        {/if}
        <ActionButton
          variant="secondary"
          size="md"
          icon="check"
          disabled={!documentPath.trim()}
          disabledReason="Enter or pick a document path before granting access."
          onclick={grantDocumentPath}
        >
          Grant Path
        </ActionButton>
        <ActionButton
          variant="primary"
          size="md"
          icon="documents"
          disabled={!documentPath.trim()}
          disabledReason="Enter, pick, upload, or grant a document path before ingesting."
          onclick={ingestDocuments}
        >
          Ingest
        </ActionButton>
      </div>
    </div>
    {#if localWeb}
      <div class="mt-3">
        <PageStatusStrip
          kind="info"
          title="Local web document access"
          message="Upload indexes a browser-selected copy. Folder upload may show the browser-owned file confirmation dialog; Yemaka cannot style or suppress that security prompt. Grant Path only records Yemaka policy permission; typed-path ingest still depends on the process running Yemaka having macOS access to that folder."
        />
      </div>
    {/if}
    {#if documentPageStatus}
      <div class="mt-3">
        <PageStatusStrip kind={documentPageStatusKind} title="Document status" message={documentPageStatus} />
      </div>
    {/if}
    {#if documentPathSuggestionSummary}
      <div class="mt-3">
        <PageStatusStrip kind="info" title="Path suggestions" message={documentPathSuggestionSummary} />
      </div>
    {/if}
  </div>
  <div class="document-inventory-card mb-4 rounded-md border border-line bg-white p-4">
    <SectionHeader
      icon="documents"
      title="Document Inventory"
      description="Indexed files, skipped/missing states, and prune safety previews."
      meta={`${documentInventory.length} indexed | ${inventoryCount(documentInventory, 'missing')} missing | ${inventoryCount(documentInventory, 'stale')} stale`}
    >
      <svelte:fragment slot="actions">
        <ActionButton
          variant="secondary"
          size="xs"
          icon="retry"
          disabled={documentInventoryBusy}
          disabledReason="Document inventory is already refreshing."
          busy={documentInventoryBusy}
          busyLabel="Refreshing"
          onclick={() => refreshDocumentInventory()}
        >
          Refresh
        </ActionButton>
        <ActionButton
          variant="secondary"
          size="xs"
          icon="eye"
          disabled={documentPruneBusy}
          disabledReason="Document prune preview is already running."
          busy={documentPruneBusy && !documentPrunePreview}
          busyLabel="Previewing"
          onclick={previewPruneMissingDocuments}
        >
          Preview Prune
        </ActionButton>
        <ActionButton
          variant="danger"
          size="xs"
          icon="trash"
          disabled={documentPruneBusy || !documentPrunePreview?.dryRun}
          disabledReason={documentPrunePreview?.dryRun ? 'Document prune is already running.' : 'Preview missing documents before pruning.'}
          busy={documentPruneBusy && Boolean(documentPrunePreview)}
          busyLabel="Pruning"
          onclick={applyPruneMissingDocuments}
        >
          Prune Missing
        </ActionButton>
      </svelte:fragment>
    </SectionHeader>
    <div class="divide-y divide-line text-sm">
      {#if documentInventory.length === 0}
        <div class="py-2">
          <EmptyState
            icon="documents"
            title="No indexed documents"
            message="Ingest a granted local path, pick a folder from the desktop app, or upload files from local web."
          />
        </div>
      {/if}
      {#each pagedItems(documentInventory, 'document-inventory', pageSizeFor('document-inventory'), listPages) as item (item.id)}
        <div class="grid gap-2 py-3 lg:grid-cols-[110px_minmax(0,1fr)_120px_120px] px-4">
          <div>
            <Badge variant={item.missing ? 'warning' : item.stale ? 'info' : 'success'}>
            {item.status}
            </Badge>
          </div>
          <div class="min-w-0">
            <div class="break-all font-medium">{item.path}</div>
            {#if item.reason}
              <div class="mt-1 text-xs text-slate-500">{item.reason}</div>
            {/if}
          </div>
          <div class="text-xs text-slate-500">{item.chunkCount} chunks</div>
          <div class="text-xs text-slate-500">{formatFileSize(item.sizeBytes)}</div>
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('document-inventory', documentInventory.length, pageSizeFor('document-inventory'), listPages)}
      total={documentInventory.length}
      pageSize={pageSizeFor('document-inventory')}
      label="documents"
      onChange={(page) => setListPage('document-inventory', page)}
    />
  </div>
  <div class="mb-4 rounded-md border border-line bg-white p-4">
    <SectionHeader
      icon="documents"
      title="Workspace Grants"
      description="Local folders Yemaka is allowed to read for document ingestion."
      meta={`${workspaceGrants.length} grants`}
    >
      <svelte:fragment slot="actions">
      <ActionButton variant="secondary" size="xs" icon="retry" onclick={refreshWorkspaceGrants}>
        Refresh
      </ActionButton>
      </svelte:fragment>
    </SectionHeader>
    <div class="divide-y divide-line text-sm">
      {#if (workspaceGrants ?? []).length === 0}
        <div class="py-2">
          <EmptyState
            icon="documents"
            title="No external folders granted"
            message="Use Pick Folder in the desktop app for macOS bookmark access, or Grant Path when the running process already has OS access."
          />
        </div>
      {/if}
      {#each pagedItems(workspaceGrants, 'workspace-grants', pageSizeFor('workspace-grants'), listPages) as grant}
        <div class="grid gap-2 py-2 px-4 lg:grid-cols-[180px_minmax(0,1fr)_180px_auto] items-center">
          <div class="truncate font-medium">{grant.label || 'workspace'}</div>
          <div class="break-all text-slate-600">{grant.path}</div>
          <div class="text-xs text-slate-500">
            <Badge variant={grant.bookmarkStale ? 'warning' : grant.bookmarkReady ? 'success' : 'neutral'}>
              {humanizeIdentifier(grant.source, grant.source)}{grant.bookmarkReady ? ' | bookmark' : ''}{grant.bookmarkStale ? ' | stale' : ''}
            </Badge>
          </div>
          <ActionButton variant="danger" size="xs" icon="trash" onclick={() => revokeWorkspaceGrant(grant.id)}>
            Revoke
          </ActionButton>
        </div>
      {/each}
    </div>
    <PaginationControls
      page={currentPage('workspace-grants', workspaceGrants.length, pageSizeFor('workspace-grants'), listPages)}
      total={workspaceGrants.length}
      pageSize={pageSizeFor('workspace-grants')}
      label="grants"
      onChange={(page) => setListPage('workspace-grants', page)}
    />
  </div>
  <div class="mb-4 flex gap-2">
    <input class="h-10 flex-1 rounded-md border border-line bg-white px-3 text-sm" bind:value={documentQuery} placeholder="Search documents" />
    <ActionButton
      variant="secondary"
      size="md"
      onclick={() => {
        documentQuery = '';
        searchDocuments();
      }}
    >
      Chunks
    </ActionButton>
    <ActionButton variant="primary" size="md" icon="search" onclick={searchDocuments}>
      Search
    </ActionButton>
  </div>
  {#if documentResults.length === 0}
    <EmptyState
      icon="search"
      title="No document search results"
      message="Search indexed chunks, or ingest/upload documents first if the local corpus is empty."
    />
  {:else}
    <div class="divide-y divide-line rounded-md border border-line bg-white">
      {#each pagedItems(documentResults, 'document-results', pageSizeFor('document-results'), listPages) as result}
        <div class="p-4 text-sm">
          <div class="mb-1 break-all text-xs font-medium text-slate-500">
            {result.path} | {humanizeIdentifier(result.source || 'stored', 'Stored')} | score {result.score ? result.score.toFixed(2) : '0.00'} | {result.chunkId}
          </div>
          {#if result.explanation?.length}
            <div class="mb-2 text-xs text-slate-500">{result.explanation.join(', ')}</div>
          {/if}
          <StructuredDataView value={result.content} title="Chunk content" filename={result.path} maxPreviewLines={12} />
          <div class="mt-3 flex justify-end">
            <ActionButton
              size="xs"
              variant="secondary"
              icon="memory"
              disabled={!result.content?.trim()}
              disabledReason="This document result has no text to draft from."
              onclick={() => draftKnowledgeFromDocument(result)}
            >
              Draft Graph
            </ActionButton>
          </div>
        </div>
      {/each}
    </div>
  {/if}
  <PaginationControls
    page={currentPage('document-results', documentResults.length, pageSizeFor('document-results'), listPages)}
    total={documentResults.length}
    pageSize={pageSizeFor('document-results')}
    label="chunks"
    onChange={(page) => setListPage('document-results', page)}
  />
</div>
