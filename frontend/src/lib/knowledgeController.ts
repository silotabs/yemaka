import type { KnowledgeEntity, KnowledgeEntityProposal, KnowledgeProposal, KnowledgeRelationProposal, KnowledgeReviewItem } from './appTypes';
import {
  applyKnowledgeReviewBatch as applyKnowledgeReviewBatchAction,
  draftKnowledgeProposal,
  knowledgeStatusLabel,
  listKnowledgeReviewItems,
  listKnowledgeEntities,
  loadKnowledgeStatus,
  mergeKnowledgeEntity,
  repairKnowledgeEvidence as repairKnowledgeEvidenceAction,
  saveKnowledgeEdge,
  saveKnowledgeEntity,
  setKnowledgeEnabled
} from './knowledgeActions';

export type KnowledgeControllerContext = {
  getQuery: () => string;
  getEntityName: () => string;
  getEntityKind: () => string;
  getEntitySource: () => string;
  getEntitySourceKind: () => string;
  getEntitySourceRef: () => string;
  getEntityEvidence: () => string;
  getFromEntityID: () => string;
  getRelation: () => string;
  getToEntityID: () => string;
  getEdgeSource: () => string;
  getEdgeSourceKind: () => string;
  getEdgeSourceRef: () => string;
  getEdgeEvidence: () => string;
  getDraftText: () => string;
  getDraftSource: () => string;
  getDraftSourceKind: () => string;
  getDraftSourceRef: () => string;
  getResults: () => KnowledgeEntity[];
  setStatus: (status: Awaited<ReturnType<typeof loadKnowledgeStatus>> | null) => void;
  setResults: (results: KnowledgeEntity[]) => void;
  setReviewItems: (items: KnowledgeReviewItem[]) => void;
  setProposal: (proposal: KnowledgeProposal | null) => void;
  setEntityName: (value: string) => void;
  setEntityKind: (value: string) => void;
  setEntitySource: (value: string) => void;
  setEntitySourceKind: (value: string) => void;
  setEntitySourceRef: (value: string) => void;
  setEntityEvidence: (value: string) => void;
  setFromEntityID: (value: string) => void;
  setToEntityID: (value: string) => void;
  setRelation: (value: string) => void;
  setEdgeSource: (value: string) => void;
  setEdgeSourceKind: (value: string) => void;
  setEdgeSourceRef: (value: string) => void;
  setEdgeEvidence: (value: string) => void;
  setSummary: (message: string, kind?: 'success' | 'error' | 'warning' | 'info') => void;
  setError: (message: string) => void;
  pushActivity: (line: string) => void;
};

export function createKnowledgeController(ctx: KnowledgeControllerContext) {
  async function refreshKnowledge(silent = false) {
    if (!silent) ctx.setError('');
    try {
      const status = await loadKnowledgeStatus();
      ctx.setStatus(status);
      if (status.enabled) {
        const entities = await listKnowledgeEntities('', status.maxEntitiesPerQuery || 12);
        ctx.setResults(entities);
        ctx.setReviewItems(await listKnowledgeReviewItems('pending', 25));
      } else {
        ctx.setResults([]);
        ctx.setReviewItems([]);
      }
      if (!silent) {
        const summary = knowledgeStatusLabel(status);
        ctx.setSummary(summary, status.enabled ? 'success' : 'info');
        ctx.pushActivity(summary);
      }
    } catch (err) {
      if (!silent) ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function toggleKnowledge(enabled: boolean) {
    ctx.setError('');
    try {
      const status = await setKnowledgeEnabled(enabled);
      ctx.setStatus(status);
      if (status.enabled) {
        const entities = await listKnowledgeEntities('', status.maxEntitiesPerQuery || 12);
        ctx.setResults(entities);
        ctx.setReviewItems(await listKnowledgeReviewItems('pending', 25));
      } else {
        ctx.setResults([]);
        ctx.setReviewItems([]);
      }
      const summary = enabled ? 'knowledge graph enabled for manual entries' : 'knowledge graph disabled';
      ctx.setSummary(summary, enabled ? 'success' : 'info');
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function searchKnowledge() {
    ctx.setError('');
    try {
      const results = await listKnowledgeEntities(ctx.getQuery(), 12);
      ctx.setResults(results);
      ctx.setReviewItems(await listKnowledgeReviewItems('pending', 25));
      const summary = `knowledge search: ${results.length} match${results.length === 1 ? '' : 'es'}`;
      ctx.setSummary(summary);
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function repairKnowledgeEvidence() {
    ctx.setError('');
    try {
      const result = await repairKnowledgeEvidenceAction();
      ctx.setStatus(result.status);
      if (result.status?.enabled) {
        const entities = await listKnowledgeEntities(ctx.getQuery(), result.status.maxEntitiesPerQuery || 12);
        ctx.setResults(entities);
        ctx.setReviewItems(await listKnowledgeReviewItems('pending', 25));
      }
      const summary = result.message || 'knowledge graph evidence repair complete';
      ctx.setSummary(summary, result.entitiesUpdated + result.edgesUpdated > 0 ? 'success' : 'info');
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function draftProposal() {
    ctx.setError('');
    try {
      const proposal = await draftKnowledgeProposal({
        text: ctx.getDraftText(),
        source: ctx.getDraftSource() || 'review_draft',
        sourceKind: ctx.getDraftSourceKind() || 'manual',
        sourceRef: ctx.getDraftSourceRef(),
        defaultKind: 'note',
        limit: 6
      });
      ctx.setProposal(proposal);
      const entityCount = proposal.entityProposals?.length || 0;
      const relationCount = proposal.relationProposals?.length || 0;
      const summary = `knowledge draft ready: ${entityCount} ${entityCount === 1 ? 'entity' : 'entities'}, ${relationCount} ${relationCount === 1 ? 'link' : 'links'}`;
      ctx.setSummary(summary, 'info');
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  function useEntityProposal(entity: KnowledgeEntityProposal) {
    ctx.setEntityName(entity.name || '');
    ctx.setEntityKind(entity.kind || 'note');
    ctx.setEntitySource(entity.source || 'review_draft');
    ctx.setEntitySourceKind(entity.sourceKind || 'manual');
    ctx.setEntitySourceRef(entity.sourceRef || '');
    ctx.setEntityEvidence(withProvenance(entity.evidence || entity.reason, entity));
    const summary = `knowledge draft selected: ${entity.name || 'entity'}`;
    ctx.setSummary(summary, 'info');
    ctx.pushActivity(summary);
  }

  function useRelationProposal(relation: KnowledgeRelationProposal) {
    const entities = ctx.getResults();
    const from = findEntityByName(entities, relation.fromName);
    const to = findEntityByName(entities, relation.toName);
    ctx.setFromEntityID(from?.id || '');
    ctx.setToEntityID(to?.id || '');
    ctx.setRelation(relation.relation || '');
    ctx.setEdgeSource(relation.source || 'review_draft');
    ctx.setEdgeSourceKind(relation.sourceKind || 'manual');
    ctx.setEdgeSourceRef(relation.sourceRef || '');
    ctx.setEdgeEvidence(withProvenance(relation.evidence || relation.reason, relation));
    const missing = [from ? '' : relation.fromName, to ? '' : relation.toName].filter(Boolean);
    const summary =
      missing.length === 0
        ? `knowledge link draft selected: ${relation.relation || 'relationship'}`
        : `knowledge link draft selected; save or select missing entities: ${missing.join(', ')}`;
    ctx.setSummary(summary, missing.length === 0 ? 'info' : 'warning');
    ctx.pushActivity(summary);
  }

  async function addKnowledgeEntity() {
    ctx.setError('');
    try {
      const entity = await saveKnowledgeEntity({
        name: ctx.getEntityName(),
        kind: ctx.getEntityKind(),
        source: ctx.getEntitySource(),
        sourceKind: ctx.getEntitySourceKind(),
        sourceRef: ctx.getEntitySourceRef(),
        evidence: ctx.getEntityEvidence()
      });
      ctx.setEntityName('');
      ctx.setEntityEvidence('');
      ctx.setResults(mergeKnowledgeEntity(ctx.getResults(), entity));
      const status = await loadKnowledgeStatus();
      ctx.setStatus(status);
      ctx.setReviewItems(await listKnowledgeReviewItems('pending', 25));
      const summary = `knowledge entity saved: ${entity.name}`;
      ctx.setSummary(summary, 'success');
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function addKnowledgeEdge() {
    ctx.setError('');
    try {
      const edge = await saveKnowledgeEdge({
        fromEntityId: ctx.getFromEntityID(),
        relation: ctx.getRelation(),
        toEntityId: ctx.getToEntityID(),
        source: ctx.getEdgeSource(),
        sourceKind: ctx.getEdgeSourceKind(),
        sourceRef: ctx.getEdgeSourceRef(),
        evidence: ctx.getEdgeEvidence()
      });
      ctx.setRelation('');
      ctx.setEdgeEvidence('');
      const status = await loadKnowledgeStatus();
      ctx.setStatus(status);
      ctx.setReviewItems(await listKnowledgeReviewItems('pending', 25));
      const summary = `knowledge link saved: ${edge.relation}`;
      ctx.setSummary(summary, 'success');
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function refreshKnowledgeReview() {
    ctx.setError('');
    try {
      const items = await listKnowledgeReviewItems('pending', 25);
      ctx.setReviewItems(items);
      const summary = `knowledge review queue: ${items.length} pending`;
      ctx.setSummary(summary, 'info');
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  async function applyKnowledgeReview(action: string, targets: Array<{ type: string; id: string }>, note = '') {
    ctx.setError('');
    try {
      const result = await applyKnowledgeReviewBatchAction({
        action,
        targets,
        reviewedBy: 'user',
        note
      });
      ctx.setReviewItems(await listKnowledgeReviewItems('pending', 25));
      const status = await loadKnowledgeStatus();
      ctx.setStatus(status);
      const summary = `knowledge review ${result.action}: ${result.entitiesUpdated + result.edgesUpdated} updated`;
      ctx.setSummary(summary, 'success');
      ctx.pushActivity(summary);
    } catch (err) {
      ctx.setError(err instanceof Error ? err.message : String(err));
    }
  }

  return {
    refreshKnowledge,
    toggleKnowledge,
    searchKnowledge,
    repairKnowledgeEvidence,
    draftProposal,
    useEntityProposal,
    useRelationProposal,
    addKnowledgeEntity,
    addKnowledgeEdge,
    refreshKnowledgeReview,
    applyKnowledgeReview
  };
}

function findEntityByName(entities: KnowledgeEntity[], name: string) {
  const target = normalizeComparableName(name);
  if (!target) return null;
  return entities.find((entity) => normalizeComparableName(entity.name) === target) || null;
}

function normalizeComparableName(value: string) {
  return String(value || '').trim().replace(/\s+/g, ' ').toLowerCase();
}

function withProvenance(text: string, item: Pick<KnowledgeEntityProposal, 'sourceKind' | 'sourceRef'>) {
  const cleaned = String(text || '').trim();
  const suffix = provenanceSuffix(item);
  if (!suffix) return cleaned;
  if (!cleaned) return suffix;
  if (cleaned.includes(suffix)) return cleaned;
  return `${cleaned}\n\n${suffix}`;
}

function provenanceSuffix(item: Pick<KnowledgeEntityProposal, 'sourceKind' | 'sourceRef'>) {
  const parts = [item.sourceKind, item.sourceRef].filter(Boolean);
  if (parts.length === 0) return '';
  return `Provenance: ${parts.join(' / ')}`;
}
