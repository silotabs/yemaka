import type {
  KnowledgeEdge,
  KnowledgeEdgeInput,
  KnowledgeEntity,
  KnowledgeEntityInput,
  KnowledgeProposal,
  KnowledgeProposalInput,
  KnowledgeRepairResult,
  KnowledgeReviewBatchInput,
  KnowledgeReviewBatchResult,
  KnowledgeReviewItem,
  KnowledgeStatus
} from './appTypes';
import { call } from './api';
import { asArray } from './uiHelpers';

export async function loadKnowledgeStatus() {
  return await call<KnowledgeStatus>('KnowledgeStatus');
}

export async function setKnowledgeEnabled(enabled: boolean) {
  return await call<KnowledgeStatus>('SetKnowledgeEnabled', { enabled });
}

export async function repairKnowledgeEvidence() {
  return await call<KnowledgeRepairResult>('RepairKnowledgeEvidence');
}

export async function listKnowledgeReviewItems(status = 'pending', limit = 25) {
  return asArray(await call<KnowledgeReviewItem[]>('ListKnowledgeReviewItems', status, limit));
}

export async function applyKnowledgeReviewBatch(input: KnowledgeReviewBatchInput) {
  return await call<KnowledgeReviewBatchResult>('ApplyKnowledgeReviewBatch', input);
}

export async function listKnowledgeEntities(query = '', limit = 12) {
  return asArray(await call<KnowledgeEntity[]>('ListKnowledgeEntities', query, limit));
}

export async function draftKnowledgeProposal(input: KnowledgeProposalInput) {
  return await call<KnowledgeProposal>('DraftKnowledgeProposal', input);
}

export async function saveKnowledgeEntity(input: KnowledgeEntityInput) {
  return await call<KnowledgeEntity>('SaveKnowledgeEntity', input);
}

export async function saveKnowledgeEdge(input: KnowledgeEdgeInput) {
  return await call<KnowledgeEdge>('SaveKnowledgeEdge', input);
}

export function mergeKnowledgeEntity(results: KnowledgeEntity[] | null | undefined, entity: KnowledgeEntity) {
  return [entity, ...asArray(results).filter((item) => item.id !== entity.id)];
}

export function knowledgeStatusLabel(status: KnowledgeStatus | null | undefined) {
  if (!status?.enabled) return 'knowledge graph disabled';
  const influence = status.influenceEnabled ? 'influence on' : 'influence off';
  return `knowledge graph: ${status.entityCount || 0} entities, ${status.edgeCount || 0} links, ${influence}`;
}
