<script lang="ts">
  import BitSelect from '../BitSelect.svelte';
  import ActionButton from '../ActionButton.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import PaginationControls from '../PaginationControls.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import StructuredDataView from '../StructuredDataView.svelte';
  import { humanizeIdentifier } from '../lib/uiHelpers';
  import type { ToastKind } from '../lib/toastCenter';
  import Badge from '../Badge.svelte';

  type Option = {
    value: string;
    label: string;
  };

  type MemoryResult = {
    id: string;
    messageId: string;
    conversationId: string;
    role: string;
    kind: string;
    content: string;
    snippet: string;
    importance: number;
    source: string;
    pinned: boolean;
    explicit: boolean;
    createdAt: string;
  };

  type KnowledgeStatus = {
    enabled: boolean;
    influenceEnabled: boolean;
    manualOnly: boolean;
    maxEntitiesPerQuery: number;
    maxEvidenceChars: number;
    maxInfluenceEntities: number;
    maxInfluenceChars: number;
    database: string;
    entityCount: number;
    edgeCount: number;
    message: string;
  };

  type KnowledgeEntity = {
    id: string;
    name: string;
    kind: string;
    source: string;
    sourceKind?: string;
    sourceRef?: string;
    evidence: string;
    reviewStatus?: string;
    reviewNote?: string;
    reviewedBy?: string;
    reviewedAt?: string;
    createdAt: string;
    updatedAt: string;
  };

  type KnowledgeReviewItem = {
    id: string;
    type: string;
    name: string;
    kind: string;
    fromEntityId?: string;
    relation?: string;
    toEntityId?: string;
    source: string;
    sourceKind: string;
    sourceRef: string;
    evidence: string;
    reviewStatus: string;
    reviewNote: string;
    reviewedBy: string;
    reviewedAt: string;
    updatedAt: string;
  };

  type KnowledgeEntityProposal = {
    name: string;
    kind: string;
    source: string;
    sourceKind: string;
    sourceRef: string;
    evidence: string;
    reason: string;
  };

  type KnowledgeRelationProposal = {
    fromName: string;
    relation: string;
    toName: string;
    source: string;
    sourceKind: string;
    sourceRef: string;
    evidence: string;
    reason: string;
  };

  type KnowledgeProposal = {
    source: string;
    sourceKind: string;
    sourceRef: string;
    reviewRequired: boolean;
    message: string;
    entityProposals: KnowledgeEntityProposal[];
    relationProposals: KnowledgeRelationProposal[];
  };

  export let memoryKind = 'preference';
  export let memoryImportance = 3;
  export let memoryContent = '';
  export let memoryQuery = '';
  export let knowledgeQuery = '';
  export let knowledgeEntityName = '';
  export let knowledgeEntityKind = 'concept';
  export let knowledgeEntitySource = 'manual';
  export let knowledgeEntitySourceKind = 'manual';
  export let knowledgeEntitySourceRef = '';
  export let knowledgeEntityEvidence = '';
  export let knowledgeFromEntityId = '';
  export let knowledgeRelation = '';
  export let knowledgeToEntityId = '';
  export let knowledgeEdgeSource = 'manual';
  export let knowledgeEdgeSourceKind = 'manual';
  export let knowledgeEdgeSourceRef = '';
  export let knowledgeEdgeEvidence = '';
  export let knowledgeDraftText = '';
  export let knowledgeDraftSource = 'review_draft';
  export let knowledgeDraftSourceKind = 'manual';
  export let knowledgeDraftSourceRef = '';
  export let memoryResults: MemoryResult[] = [];
  export let knowledgeStatus: KnowledgeStatus | null = null;
  export let knowledgeResults: KnowledgeEntity[] = [];
  export let knowledgeReviewItems: KnowledgeReviewItem[] = [];
  export let knowledgeProposal: KnowledgeProposal | null = null;
  export let memoryPageStatus = '';
  export let memoryPageStatusKind: ToastKind = 'info';
  export let memoryKindOptions: Option[] = [];
  export let listPages: Record<string, number> = {};
  export let pageSizeFor: (key: string) => number = () => 10;
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let writeMemory: () => Promise<void> | void = () => {};
  export let listMemories: () => Promise<void> | void = () => {};
  export let searchMemory: () => Promise<void> | void = () => {};
  export let pinMemory: (id: string) => Promise<void> | void = () => {};
  export let deleteMemory: (id: string) => Promise<void> | void = () => {};
  export let draftKnowledgeFromMemory: (result: MemoryResult) => Promise<void> | void = () => {};
  export let refreshKnowledge: (silent?: boolean) => Promise<void> | void = () => {};
  export let toggleKnowledge: (enabled: boolean) => Promise<void> | void = () => {};
  export let searchKnowledge: () => Promise<void> | void = () => {};
  export let repairKnowledgeEvidence: () => Promise<void> | void = () => {};
  export let refreshKnowledgeReview: () => Promise<void> | void = () => {};
  export let applyKnowledgeReview: (action: string, targets: Array<{ type: string; id: string }>, note?: string) => Promise<void> | void = () => {};
  export let draftProposal: () => Promise<void> | void = () => {};
  export let useEntityProposal: (entity: KnowledgeEntityProposal) => void = () => {};
  export let useRelationProposal: (relation: KnowledgeRelationProposal) => void = () => {};
  export let addKnowledgeEntity: () => Promise<void> | void = () => {};
  export let addKnowledgeEdge: () => Promise<void> | void = () => {};

  $: knowledgeEnabled = Boolean(knowledgeStatus?.enabled);
  $: knowledgeInfluenceEnabled = Boolean(knowledgeStatus?.influenceEnabled);
  $: knowledgeMeta = knowledgeStatus?.enabled
    ? `${knowledgeStatus.entityCount || 0} entities | ${knowledgeStatus.edgeCount || 0} links`
    : 'disabled';
  $: canLinkKnowledge = Boolean(knowledgeEnabled && knowledgeResults.length >= 2 && knowledgeFromEntityId && knowledgeToEntityId && knowledgeRelation.trim());
  $: proposalEntityCount = knowledgeProposal?.entityProposals?.length || 0;
  $: proposalRelationCount = knowledgeProposal?.relationProposals?.length || 0;
  $: pendingReviewCount = (knowledgeReviewItems ?? []).length;
  $: allReviewTargets = (knowledgeReviewItems ?? []).map((item) => ({ type: item.type, id: item.id }));
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  {#if memoryPageStatus.trim()}
    <div class="mb-4">
      <PageStatusStrip kind={memoryPageStatusKind} title="Memory status" message={memoryPageStatus} />
    </div>
  {/if}
  <div class="mb-4 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="memory"
      title="Write Memory"
      description="Save explicit preferences, corrections, or notes into local memory."
      className="mb-3"
    />
    <div class="grid gap-3 lg:grid-cols-[180px_120px_minmax(0,1fr)_auto]">
      <BitSelect bind:value={memoryKind} options={memoryKindOptions} />
      <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" type="number" min="1" max="5" bind:value={memoryImportance} />
      <input class="h-10 rounded-md border border-line bg-field px-3 text-sm" bind:value={memoryContent} placeholder="Remember..." />
      <ActionButton
        variant="primary"
        icon="check"
        disabled={!memoryContent.trim()}
        disabledReason="Enter memory content before saving."
        onclick={writeMemory}
      >
        Save
      </ActionButton>
    </div>
  </div>
  <div class="mb-4 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto_auto]">
    <input class="h-10 flex-1 rounded-md border border-line bg-white px-3 text-sm" bind:value={memoryQuery} placeholder="Search memory" />
    <ActionButton variant="secondary" icon="memory" onclick={listMemories}>List</ActionButton>
    <ActionButton
      variant="primary"
      icon="search"
      disabled={!memoryQuery.trim()}
      disabledReason="Enter a memory query before searching."
      onclick={searchMemory}
    >
      Search
    </ActionButton>
  </div>
  <SectionHeader
    compact
    icon="memory"
    title="Saved Memory"
    description="Browse local memory records returned by list or search."
    meta={`${memoryResults.length} records`}
    className="mb-3"
  />
  {#if memoryResults.length === 0}
    <EmptyState
      icon="memory"
      title="No saved memory"
      message="Saved preferences, corrections, and pinned notes will appear here after Yemaka records them."
    />
  {:else}
    <div class="divide-y divide-line rounded-md border border-line bg-white">
      {#each pagedItems(memoryResults, 'memory-results', pageSizeFor('memory-results'), listPages) as result}
        <div class="p-4 text-sm">
          <div class="mb-1 flex items-center justify-between gap-3 text-xs text-slate-500">
            <span title={`${result.kind || result.role} | ${result.source || result.conversationId || 'local'}`}>
              {humanizeIdentifier(result.kind || result.role, 'Memory')}{result.pinned ? ' | pinned' : ''}{result.importance ? ` | importance ${result.importance}` : ''}
            </span>
            <span>{result.createdAt}</span>
          </div>
          <StructuredDataView value={result.content || result.snippet} title="Memory content" maxPreviewLines={10} />
          <!-- <div class="mt-2 break-all text-xs text-slate-500">Source: {result.source || result.conversationId || 'local'} | {result.id || result.messageId}</div> -->
          <div class="mt-2 break-all text-xs text-slate-500"  title={result.source}>Source: {humanizeIdentifier(result.source || result.conversationId || 'local', 'Local')} | {result.id || result.messageId}</div>
          <div class="mt-3 flex flex-wrap gap-2">
            <ActionButton
              size="xs"
              variant="secondary"
              icon="memory"
              disabled={!(result.content || result.snippet || '').trim()}
              disabledReason="This memory row has no text to draft from."
              onclick={() => draftKnowledgeFromMemory(result)}
            >
              Draft Graph
            </ActionButton>
            {#if result.explicit}
              <ActionButton size="xs" variant="secondary" icon="star" onclick={() => pinMemory(result.id)}>Pin</ActionButton>
              <ActionButton size="xs" variant="danger" icon="trash" onclick={() => deleteMemory(result.id)}>Delete</ActionButton>
            {/if}
          </div>
        </div>
      {/each}
    </div>
  {/if}
  <PaginationControls
    page={currentPage('memory-results', memoryResults.length, pageSizeFor('memory-results'), listPages)}
    total={memoryResults.length}
    pageSize={pageSizeFor('memory-results')}
    label="memories"
    onChange={(page) => setListPage('memory-results', page)}
  />

  <div class="mt-6 rounded-md border border-line bg-white p-4">
    <SectionHeader
      compact
      icon="memory"
      title="Local Knowledge Graph"
      description={knowledgeInfluenceEnabled ? 'Manual-only local entities and relationships. Approved records can influence ordinary chat as background context.' : 'Manual-only local entities and relationships. It is not used for routing or answers automatically.'}
      meta={knowledgeMeta}
      className="mb-3"
    />
    <div class="mb-4 flex flex-wrap items-center gap-2 text-xs text-slate-500">
      <Badge variant={knowledgeEnabled ? 'success' : 'neutral'}>{knowledgeEnabled ? 'Enabled' : 'Disabled'}</Badge>
      <Badge variant="neutral">Manual only</Badge>
      <Badge variant={knowledgeInfluenceEnabled ? 'info' : 'neutral'}>{knowledgeInfluenceEnabled ? 'Influence on' : 'Influence off'}</Badge>
      <span class="min-w-0 truncate">{knowledgeStatus?.message || 'Load status to inspect the local graph.'}</span>
    </div>
    <div class="mb-4 flex flex-wrap gap-2">
      <ActionButton variant="secondary" icon="retry" onclick={() => refreshKnowledge()}>Refresh</ActionButton>
      <ActionButton
        variant="secondary"
        icon="check"
        disabled={!knowledgeEnabled}
        disabledReason="Enable the manual graph before repairing saved evidence."
        onclick={repairKnowledgeEvidence}
      >
        Repair Redaction
      </ActionButton>
      {#if knowledgeEnabled}
        <ActionButton variant="secondary" icon="eye-off" onclick={() => toggleKnowledge(false)}>Disable</ActionButton>
      {:else}
        <ActionButton variant="primary" icon="check" onclick={() => toggleKnowledge(true)}>Enable Manual Graph</ActionButton>
      {/if}
    </div>

    <div class="mb-4 rounded-md border border-line bg-field p-3">
      <SectionHeader
        compact
        icon="check"
        title="Batch Review"
        description="Review saved graph records before treating them as curated local knowledge."
        meta={`${pendingReviewCount} pending`}
        className="mb-3"
      >
        <svelte:fragment slot="actions">
          <ActionButton variant="secondary" icon="retry" disabled={!knowledgeEnabled} disabledReason="Enable the manual graph before loading review items." onclick={refreshKnowledgeReview}>Refresh</ActionButton>
          <ActionButton
            variant="primary"
            icon="check"
            disabled={!knowledgeEnabled || pendingReviewCount === 0}
            disabledReason={knowledgeEnabled ? 'No pending review items.' : 'Enable the manual graph before approving review items.'}
            onclick={() => applyKnowledgeReview('approve', allReviewTargets, 'Batch approved from review queue.')}
          >
            Approve All
          </ActionButton>
        </svelte:fragment>
      </SectionHeader>
      {#if !knowledgeEnabled}
        <EmptyState icon="memory" title="Review disabled" message="Enable the manual graph to inspect pending entity and link reviews." />
      {:else if pendingReviewCount === 0}
        <EmptyState icon="check" title="No pending review" message="New or repaired graph records that need review will appear here." />
      {:else}
        <div class="divide-y divide-line rounded-md border border-line bg-white">
          {#each knowledgeReviewItems as item}
            <div class="p-3 text-sm">
              <div class="mb-2 flex flex-wrap items-start justify-between gap-2">
                <div class="min-w-0">
                  <div class="font-medium">
                    {item.type === 'edge' ? `${item.fromEntityId} | ${humanizeIdentifier(item.relation || 'link', 'Link')} | ${item.toEntityId}` : item.name}
                  </div>
                  <div class="mt-1 flex flex-wrap gap-1 text-xs text-slate-500">
                    <Badge variant="neutral">{humanizeIdentifier(item.type, 'Record')}</Badge>
                    <Badge variant="neutral">{humanizeIdentifier(item.reviewStatus, 'Pending')}</Badge>
                    <span>{humanizeIdentifier(item.source, 'Manual')} · {humanizeIdentifier(item.sourceKind, 'Manual')}{item.sourceRef ? ` · ${item.sourceRef}` : ''}</span>
                  </div>
                </div>
                <div class="flex shrink-0 flex-wrap gap-1">
                  <ActionButton size="xs" variant="primary" icon="check" onclick={() => applyKnowledgeReview('approve', [{ type: item.type, id: item.id }], 'Approved from review queue.')}>Approve</ActionButton>
                  <ActionButton size="xs" variant="secondary" icon="warning" onclick={() => applyKnowledgeReview('needs_repair', [{ type: item.type, id: item.id }], 'Needs repair before approval.')}>Repair</ActionButton>
                  <ActionButton size="xs" variant="danger" icon="error" onclick={() => applyKnowledgeReview('reject', [{ type: item.type, id: item.id }], 'Rejected from review queue.')}>Reject</ActionButton>
                </div>
              </div>
              <StructuredDataView value={item.evidence || item.reviewNote || item.name} title="Review evidence" maxPreviewLines={4} />
              <div class="mt-2 break-all text-xs text-slate-500">ID: {item.id} · Updated: {item.updatedAt}</div>
            </div>
          {/each}
        </div>
      {/if}
    </div>

    <div class="mb-4 rounded-md border border-line bg-field p-3">
      <SectionHeader
        compact
        icon="memory"
        title="Draft From Text"
        description="Create review-only graph candidates from pasted memory, notes, or document snippets."
        meta={knowledgeProposal ? `${proposalEntityCount} entities | ${proposalRelationCount} links` : 'review required'}
        className="mb-3"
      />
      <div class="grid gap-2">
        <div class="grid gap-2 md:grid-cols-3">
          <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeDraftSource} placeholder="Draft source, e.g. memory review" />
          <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeDraftSourceKind} placeholder="Source kind, e.g. memory or document" />
          <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeDraftSourceRef} placeholder="Reviewed source ref or id" />
        </div>
        <textarea class="min-h-28 rounded-md border border-line bg-white px-3 py-2 text-sm" bind:value={knowledgeDraftText} placeholder="Paste reviewed text. Optional explicit lines: Entity: Name | kind: concept, Relationship: A -> depends on -> B"></textarea>
        <ActionButton
          variant="secondary"
          icon="search"
          disabled={!knowledgeDraftText.trim()}
          disabledReason="Paste reviewed text before drafting graph candidates."
          onclick={draftProposal}
        >
          Draft Review
        </ActionButton>
      </div>
      {#if knowledgeProposal}
        <div class="mt-3 flex flex-wrap gap-2 text-xs text-slate-500">
          <Badge variant="neutral">{humanizeIdentifier(knowledgeProposal.sourceKind || 'manual', 'Manual')}</Badge>
          {#if knowledgeProposal.sourceRef}
            <span class="min-w-0 truncate">Source ref: {knowledgeProposal.sourceRef}</span>
          {/if}
        </div>
        <div class="mt-4 grid gap-3 xl:grid-cols-2">
          <div>
            <div class="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">Entity candidates</div>
            {#if proposalEntityCount === 0}
              <EmptyState icon="memory" title="No entity candidates" message="Try an explicit Entity line or save manually." />
            {:else}
              <div class="divide-y divide-line rounded-md border border-line bg-white">
                {#each knowledgeProposal.entityProposals as entity}
                  <div class="p-3 text-sm">
                    <div class="mb-1 flex flex-wrap items-center justify-between gap-2">
                      <span class="font-medium">{entity.name}</span>
                      <Badge variant="neutral">{humanizeIdentifier(entity.kind, 'Entity')}</Badge>
                    </div>
                    <StructuredDataView value={entity.evidence || entity.reason} title="Draft evidence" maxPreviewLines={4} />
                    <div class="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
                      <span>{humanizeIdentifier(entity.source, 'Review draft')} · {humanizeIdentifier(entity.sourceKind, 'Manual')}{entity.sourceRef ? ` · ${entity.sourceRef}` : ''}</span>
                      <ActionButton size="xs" variant="secondary" icon="check" onclick={() => useEntityProposal(entity)}>Use Draft</ActionButton>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
          <div>
            <div class="mb-2 text-xs font-semibold uppercase tracking-wider text-slate-500">Relationship candidates</div>
            {#if proposalRelationCount === 0}
              <EmptyState icon="memory" title="No relationship candidates" message="Use Relationship: A -> relation -> B to draft a link." />
            {:else}
              <div class="divide-y divide-line rounded-md border border-line bg-white">
                {#each knowledgeProposal.relationProposals as relation}
                  <div class="p-3 text-sm">
                    <div class="mb-1 font-medium">{relation.fromName} | {humanizeIdentifier(relation.relation, 'Relation')} | {relation.toName}</div>
                    <StructuredDataView value={relation.evidence || relation.reason} title="Draft evidence" maxPreviewLines={4} />
                    <div class="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
                      <span>{humanizeIdentifier(relation.source, 'Review draft')} · {humanizeIdentifier(relation.sourceKind, 'Manual')}{relation.sourceRef ? ` · ${relation.sourceRef}` : ''}</span>
                      <ActionButton size="xs" variant="secondary" icon="check" onclick={() => useRelationProposal(relation)}>Use Draft</ActionButton>
                    </div>
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
      {/if}
    </div>

    <div class="grid gap-4 xl:grid-cols-2">
      <div class="rounded-md border border-line bg-field p-3">
        <SectionHeader
          compact
          icon="check"
          title="Add Entity"
          description="Save a reviewed concept, person, project, tool, or note with evidence."
          className="mb-3"
        />
        <div class="grid gap-2">
          <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEntityName} placeholder="Entity name" disabled={!knowledgeEnabled} />
          <div class="grid gap-2 sm:grid-cols-2">
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEntityKind} placeholder="Kind, e.g. concept" disabled={!knowledgeEnabled} />
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEntitySource} placeholder="Source, e.g. manual" disabled={!knowledgeEnabled} />
          </div>
          <div class="grid gap-2 sm:grid-cols-2">
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEntitySourceKind} placeholder="Source kind, e.g. memory" disabled={!knowledgeEnabled} />
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEntitySourceRef} placeholder="Source ref or reviewed id" disabled={!knowledgeEnabled} />
          </div>
          <textarea class="min-h-24 rounded-md border border-line bg-white px-3 py-2 text-sm" bind:value={knowledgeEntityEvidence} placeholder="Short evidence or provenance" disabled={!knowledgeEnabled}></textarea>
          <ActionButton
            variant="primary"
            icon="check"
            disabled={!knowledgeEnabled || !knowledgeEntityName.trim()}
            disabledReason={knowledgeEnabled ? 'Enter an entity name before saving.' : 'Enable the manual graph before saving entities.'}
            onclick={addKnowledgeEntity}
          >
            Save Entity
          </ActionButton>
        </div>
      </div>

      <div class="rounded-md border border-line bg-field p-3">
        <SectionHeader
          compact
          icon="memory"
          title="Link Entities"
          description="Create an explicit relationship between two reviewed entities."
          className="mb-3"
        />
        <div class="grid gap-2">
          <div class="grid gap-2 sm:grid-cols-2">
            <select class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeFromEntityId} disabled={!knowledgeEnabled || knowledgeResults.length === 0}>
              <option value="">From entity</option>
              {#each knowledgeResults as entity}
                <option value={entity.id}>{entity.name}</option>
              {/each}
            </select>
            <select class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeToEntityId} disabled={!knowledgeEnabled || knowledgeResults.length === 0}>
              <option value="">To entity</option>
              {#each knowledgeResults as entity}
                <option value={entity.id}>{entity.name}</option>
              {/each}
            </select>
          </div>
          <div class="grid gap-2 sm:grid-cols-[minmax(0,1fr)_150px]">
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeRelation} placeholder="Relation, e.g. depends on" disabled={!knowledgeEnabled} />
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEdgeSource} placeholder="Source" disabled={!knowledgeEnabled} />
          </div>
          <div class="grid gap-2 sm:grid-cols-2">
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEdgeSourceKind} placeholder="Source kind, e.g. document" disabled={!knowledgeEnabled} />
            <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeEdgeSourceRef} placeholder="Source ref or reviewed id" disabled={!knowledgeEnabled} />
          </div>
          <textarea class="min-h-24 rounded-md border border-line bg-white px-3 py-2 text-sm" bind:value={knowledgeEdgeEvidence} placeholder="Relationship evidence" disabled={!knowledgeEnabled}></textarea>
          <ActionButton
            variant="primary"
            icon="plus"
            disabled={!canLinkKnowledge || knowledgeFromEntityId === knowledgeToEntityId}
            disabledReason={knowledgeEnabled ? 'Select two different entities and enter a relation.' : 'Enable the manual graph before linking entities.'}
            onclick={addKnowledgeEdge}
          >
            Save Link
          </ActionButton>
        </div>
      </div>
    </div>

    <div class="mt-4 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto_auto]">
      <input class="h-10 rounded-md border border-line bg-white px-3 text-sm" bind:value={knowledgeQuery} placeholder="Search graph entities" disabled={!knowledgeEnabled} />
      <ActionButton variant="secondary" icon="memory" disabled={!knowledgeEnabled} disabledReason="Enable the manual graph before listing entities." onclick={() => searchKnowledge()}>List</ActionButton>
      <ActionButton
        variant="primary"
        icon="search"
        disabled={!knowledgeEnabled || !knowledgeQuery.trim()}
        disabledReason={knowledgeEnabled ? 'Enter a graph query before searching.' : 'Enable the manual graph before searching.'}
        onclick={searchKnowledge}
      >
        Search
      </ActionButton>
    </div>

    <div class="mt-4">
      <SectionHeader
        compact
        icon="memory"
        title="Graph Entities"
        description="Recently saved or searched local graph entities."
        meta={`${knowledgeResults.length} records`}
        className="mb-3"
      />
      {#if knowledgeResults.length === 0}
        <EmptyState
          icon="memory"
          title={knowledgeEnabled ? 'No graph entities' : 'Knowledge graph disabled'}
          message={knowledgeEnabled ? 'Manual entities will appear here after you add or search them.' : 'Enable the manual graph when you are ready to curate explicit relationships.'}
        />
      {:else}
        <div class="divide-y divide-line rounded-md border border-line bg-white">
          {#each pagedItems(knowledgeResults, 'knowledge-results', pageSizeFor('knowledge-results'), listPages) as entity}
            <div class="p-4 text-sm">
              <div class="mb-1 flex items-center justify-between gap-3 text-xs text-slate-500">
                <span title={`${entity.kind} | ${entity.source}`}>{entity.name} | {humanizeIdentifier(entity.kind, 'Entity')} | {humanizeIdentifier(entity.source, 'Manual')}</span>
                <span>{entity.updatedAt || entity.createdAt}</span>
              </div>
              <div class="mb-2 flex flex-wrap gap-1 text-xs text-slate-500">
                <Badge variant={entity.reviewStatus === 'approved' ? 'success' : 'neutral'}>{humanizeIdentifier(entity.reviewStatus || 'pending', 'Pending')}</Badge>
                {#if entity.sourceKind}
                  <Badge variant="neutral">{humanizeIdentifier(entity.sourceKind, 'Manual')}</Badge>
                {/if}
                {#if entity.sourceRef}
                  <span class="min-w-0 break-all">{entity.sourceRef}</span>
                {/if}
              </div>
              <StructuredDataView value={entity.evidence || entity.name} title="Evidence" maxPreviewLines={6} />
              <div class="mt-2 break-all text-xs text-slate-500">ID: {entity.id}</div>
            </div>
          {/each}
        </div>
      {/if}
      <PaginationControls
        page={currentPage('knowledge-results', knowledgeResults.length, pageSizeFor('knowledge-results'), listPages)}
        total={knowledgeResults.length}
        pageSize={pageSizeFor('knowledge-results')}
        label="graph entities"
        onChange={(page) => setListPage('knowledge-results', page)}
      />
    </div>
  </div>
</div>
