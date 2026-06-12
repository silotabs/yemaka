<script lang="ts">
  import PaginationControls from '../PaginationControls.svelte';
  import ActionButton from '../ActionButton.svelte';
  import Badge from '../Badge.svelte';
  import EmptyState from '../EmptyState.svelte';
  import PageStatusStrip from '../PageStatusStrip.svelte';
  import SectionHeader from '../SectionHeader.svelte';
  import StructuredDataView from '../StructuredDataView.svelte';
  import { humanizeIdentifier } from '../lib/uiHelpers';

  type PolicyStatus = {
    mode: string;
    auditEnabled: boolean;
    maxAutonomousLevel: number;
    allowLiveTrading: boolean;
    generatedCanModifyCorePolicy: boolean;
    fullAccess: boolean;
  };

  type LearningReport = {
    generatedAt: string;
    workflowSuccesses: number;
    workflowFailures: number;
    corrections: number;
    extensionGenerations: number;
    recentLearningMemory: Array<{ id: string; kind: string; source: string; snippet: string; importance: number; updatedAt: string }>;
    suggestions: string[];
  };

  type RouteCorrection = {
    id: string;
    pattern: string;
    intendedRouteCategory: string;
    requiredTools?: string[];
    forbiddenTools?: string[];
    approvalStatus: string;
    disabled: boolean;
  };

  type QAReplayFailure = {
    traceId: string;
    requestSummary: string;
    routeCategory?: string;
    routeFailureCategory?: string;
    routeFailureReason?: string;
    suggestedGenericRegression?: string;
    promotionFingerprint?: string;
    repeatCount?: number;
    promotionPriority?: string;
    coveredByEvalOrTest?: boolean;
    coverageSources?: string[];
    reviewStatus?: string;
    reviewNote?: string;
    reviewedAt?: string;
    testDraftStatus?: string;
    testDraftPath?: string;
    approvedRegression?: string;
    approvedRegressionAt?: string;
    approvedRegressionPath?: string;
    sourcePatchStatus?: string;
    sourcePatchPath?: string;
    sourcePatchRequiresManualApply?: boolean;
    sourcePatchApplyStatus?: string;
    sourcePatchApplyPath?: string;
    sourceTestWriteStatus?: string;
    sourceTestWritePath?: string;
    sourceTestWriteSnapshotId?: string;
    regressionPromotion?: RouteRegressionPromotion;
    failureStage?: string;
    failureCode?: string;
    failureMessage?: string;
    resultStatus?: string;
    verificationStatus?: string;
    coverageStatus: string;
    regressionSuggestions?: Array<{
      name?: string;
      kind?: string;
      preview?: string;
      coverageStatus?: string;
      coveredByTest?: boolean;
      coverageSources?: string[];
      reviewStatus?: string;
      reviewNote?: string;
      reviewedAt?: string;
      testDraftStatus?: string;
      testDraftPath?: string;
      approvedRegression?: string;
      approvedRegressionAt?: string;
      approvedRegressionPath?: string;
      sourcePatchStatus?: string;
      sourcePatchPath?: string;
      sourcePatchRequiresManualApply?: boolean;
      sourcePatchApplyStatus?: string;
      sourcePatchApplyPath?: string;
      sourceTestWriteStatus?: string;
      sourceTestWritePath?: string;
      sourceTestWriteSnapshotId?: string;
      promotion?: RouteRegressionPromotion;
      repeatCount?: number;
      promotionPriority?: string;
    }>;
    matchingCorrectionIds?: string[];
  };

  type RouteRegressionPromotion = {
    suggestedTestName?: string;
    routeUnderTest?: string;
    continuationMode?: string;
    expectedSource?: string;
    expectedToolLane?: string;
    expectedOutcome?: string;
  };

  type QAReview = {
    generatedAt: string;
    summary?: {
      replayFailures: number;
      routeCorrections: number;
      pendingCorrections: number;
      regressionSuggestions: number;
      repeatedSuggestions?: number;
      approvedSuggestions?: number;
      dismissedSuggestions?: number;
      coveredSuggestions?: number;
      testDrafts?: number;
      approvedRegressions?: number;
      sourcePatches?: number;
      sourcePatchApplyPlans?: number;
      sourceTestWrites?: number;
      routeFailureCategories?: Record<string, number>;
    };
    replayFailures?: QAReplayFailure[];
    routeCorrections?: RouteCorrection[];
    latestEval?: {
      status: string;
    };
  };

  export let learningReport: LearningReport | null = null;
  export let learningBusy = false;
  export let policyStatus: PolicyStatus | null = null;
  export let qaReview: QAReview | null = null;
  export let correctionConversationId = '';
  export let correctionContent = '';
  export let trajectoryConversationId = '';
  export let trajectoryPreview = '';
  export let listPages: Record<string, number> = {};
  export let pageSizeFor: (key: string) => number = () => 10;
  export let currentPage: (key: string, total: number, size?: number, pagesState?: Record<string, number>) => number = () => 1;
  export let pagedItems: <T>(items: T[] | null | undefined, key: string, size?: number, pagesState?: Record<string, number>) => T[] = (items) => items ?? [];
  export let setListPage: (key: string, page: number) => void = () => {};
  export let refreshLearning: () => Promise<void> | void = () => {};
  export let refreshDiagnostics: () => Promise<void> | void = () => {};
  export let promoteQARegressionSuggestion: (traceId: string, suggestionName?: string) => Promise<unknown> | unknown = () => {};
  export let reviewQARegressionSuggestion: (traceId: string, suggestionName: string, status: 'approved' | 'dismissed' | 'covered') => Promise<unknown> | unknown = () => {};
  export let generateQARegressionTestDraft: (traceId: string, suggestionName?: string) => Promise<unknown> | unknown = () => {};
  export let approveQARegressionTestDraft: (traceId: string, suggestionName?: string, draftName?: string) => Promise<unknown> | unknown = () => {};
  export let generateQARegressionSourcePatch: (traceId: string, suggestionName?: string, draftName?: string) => Promise<unknown> | unknown = () => {};
  export let approveQARegressionSourcePatchApplyPlan: (traceId: string, suggestionName?: string, sourcePatchName?: string) => Promise<unknown> | unknown = () => {};
  export let planQARegressionSourceWrite: (traceId: string, suggestionName?: string, sourcePatchName?: string, content?: string) => Promise<unknown> | unknown = () => {};
  export let applyQARegressionSourceWrite: (traceId: string, suggestionName?: string, sourcePatchName?: string, content?: string) => Promise<unknown> | unknown = () => {};
  export let setPolicyMode: (mode: string) => Promise<void> | void = () => {};
  export let saveCorrection: () => Promise<void> | void = () => {};
  export let exportTrajectory: () => Promise<void> | void = () => {};
  export let shortId: (value: string | undefined) => string = (value) => String(value || '');

  let promotionBusy: Record<string, boolean> = {};
  let reviewBusy: Record<string, boolean> = {};
  let draftBusy: Record<string, boolean> = {};
  let approvalBusy: Record<string, boolean> = {};
  let sourcePatchBusy: Record<string, boolean> = {};
  let sourcePatchApplyBusy: Record<string, boolean> = {};
  let sourceWriteBusy: Record<string, boolean> = {};
  let sourceWriteContent: Record<string, string> = {};
  let sourceWritePreview: Record<string, string> = {};

  function correctionTools(correction: RouteCorrection) {
    return [
      ...(correction.requiredTools || []).map((tool) => humanizeIdentifier(tool, tool)),
      ...(correction.forbiddenTools || []).map((tool) => `!${humanizeIdentifier(tool, tool)}`)
    ].join(', ');
  }

  async function handlePromoteRegression(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = `${failure.traceId}:${suggestion.name || suggestion.kind || 'suggestion'}`;
    promotionBusy = { ...promotionBusy, [key]: true };
    try {
      await promoteQARegressionSuggestion(failure.traceId, suggestion.name || suggestion.kind || '');
    } finally {
      const next = { ...promotionBusy };
      delete next[key];
      promotionBusy = next;
    }
  }

  async function handleReviewRegression(failure: QAReplayFailure, status: 'approved' | 'dismissed' | 'covered') {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = `${promotionKey(failure)}:${status}`;
    reviewBusy = { ...reviewBusy, [key]: true };
    try {
      await reviewQARegressionSuggestion(failure.traceId, suggestion.name || suggestion.kind || '', status);
    } finally {
      const next = { ...reviewBusy };
      delete next[key];
      reviewBusy = next;
    }
  }

  async function handleGenerateTestDraft(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = promotionKey(failure);
    draftBusy = { ...draftBusy, [key]: true };
    try {
      await generateQARegressionTestDraft(failure.traceId, suggestion.name || suggestion.kind || '');
    } finally {
      const next = { ...draftBusy };
      delete next[key];
      draftBusy = next;
    }
  }

  async function handleApproveRegression(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = promotionKey(failure);
    approvalBusy = { ...approvalBusy, [key]: true };
    try {
      await approveQARegressionTestDraft(failure.traceId, suggestion.name || suggestion.kind || '', suggestion.name || suggestion.kind || '');
    } finally {
      const next = { ...approvalBusy };
      delete next[key];
      approvalBusy = next;
    }
  }

  async function handleGenerateSourcePatch(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = promotionKey(failure);
    sourcePatchBusy = { ...sourcePatchBusy, [key]: true };
    try {
      await generateQARegressionSourcePatch(failure.traceId, suggestion.name || suggestion.kind || '', suggestion.name || suggestion.kind || '');
    } finally {
      const next = { ...sourcePatchBusy };
      delete next[key];
      sourcePatchBusy = next;
    }
  }

  async function handleApproveSourcePatchApplyPlan(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = promotionKey(failure);
    sourcePatchApplyBusy = { ...sourcePatchApplyBusy, [key]: true };
    try {
      await approveQARegressionSourcePatchApplyPlan(failure.traceId, suggestion.name || suggestion.kind || '', suggestion.name || suggestion.kind || '');
    } finally {
      const next = { ...sourcePatchApplyBusy };
      delete next[key];
      sourcePatchApplyBusy = next;
    }
  }

  async function handlePlanSourceWrite(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = promotionKey(failure);
    sourceWriteBusy = { ...sourceWriteBusy, [`${key}:plan`]: true };
    try {
      const result = (await planQARegressionSourceWrite(failure.traceId, suggestion.name || suggestion.kind || '', suggestion.name || suggestion.kind || '', sourceWriteContent[key] || '')) as { fileWritePlan?: { diff?: string } } | null;
      if (result?.fileWritePlan?.diff) {
        sourceWritePreview = { ...sourceWritePreview, [key]: result.fileWritePlan.diff };
      }
    } finally {
      const next = { ...sourceWriteBusy };
      delete next[`${key}:plan`];
      sourceWriteBusy = next;
    }
  }

  async function handleApplySourceWrite(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    if (!failure.traceId || !suggestion) return;
    const key = promotionKey(failure);
    sourceWriteBusy = { ...sourceWriteBusy, [`${key}:apply`]: true };
    try {
      await applyQARegressionSourceWrite(failure.traceId, suggestion.name || suggestion.kind || '', suggestion.name || suggestion.kind || '', sourceWriteContent[key] || '');
      sourceWritePreview = { ...sourceWritePreview, [key]: '' };
    } finally {
      const next = { ...sourceWriteBusy };
      delete next[`${key}:apply`];
      sourceWriteBusy = next;
    }
  }

  function promotionKey(failure: QAReplayFailure) {
    const suggestion = failure.regressionSuggestions?.[0];
    return `${failure.traceId}:${suggestion?.name || suggestion?.kind || 'suggestion'}`;
  }

  function reviewKey(failure: QAReplayFailure, status: string) {
    return `${promotionKey(failure)}:${status}`;
  }

  function reviewStatusFor(failure: QAReplayFailure) {
    return failure.regressionSuggestions?.[0]?.reviewStatus || failure.reviewStatus || '';
  }

  function testDraftStatusFor(failure: QAReplayFailure) {
    return failure.regressionSuggestions?.[0]?.testDraftStatus || failure.testDraftStatus || '';
  }

  function approvedRegressionFor(failure: QAReplayFailure) {
    return failure.regressionSuggestions?.[0]?.approvedRegression || failure.approvedRegression || '';
  }

  function sourcePatchStatusFor(failure: QAReplayFailure) {
    return failure.regressionSuggestions?.[0]?.sourcePatchStatus || failure.sourcePatchStatus || '';
  }

  function sourcePatchApplyStatusFor(failure: QAReplayFailure) {
    return failure.regressionSuggestions?.[0]?.sourcePatchApplyStatus || failure.sourcePatchApplyStatus || '';
  }

  function sourceTestWriteStatusFor(failure: QAReplayFailure) {
    return failure.regressionSuggestions?.[0]?.sourceTestWriteStatus || failure.sourceTestWriteStatus || '';
  }
</script>

<div class="section-scroll min-h-0 flex-1 overflow-y-auto p-4 sm:p-5">
  {#if learningBusy}
    <div class="mb-4">
      <PageStatusStrip kind="info" title="Learning refresh running" message="Yemaka is updating learning report, policy, QA review, and correction memory status." />
    </div>
  {/if}
  <div class="mb-4 grid gap-4 lg:grid-cols-[minmax(0,1fr)_360px]">
    <div class="rounded-md border border-line bg-white">
      <div class="border-b border-line px-4 py-3">
        <SectionHeader
          compact
          icon="learning"
          title="Learning Report"
          description="Workflow outcomes, corrections, extension generation, and learning suggestions."
          meta={learningReport?.generatedAt || 'not generated'}
        >
          <svelte:fragment slot="actions">
            <ActionButton variant="secondary" icon="retry" busy={learningBusy} busyLabel="Refreshing" onclick={refreshLearning}>
              Refresh
            </ActionButton>
          </svelte:fragment>
        </SectionHeader>
      </div>
      <div class="grid gap-3 p-4 text-sm md:grid-cols-4">
        <div class="rounded-md border border-line bg-field p-3">
          <div class="text-xs text-slate-500">Success</div>
          <div class="text-lg font-semibold">{learningReport?.workflowSuccesses ?? 0}</div>
        </div>
        <div class="rounded-md border border-line bg-field p-3">
          <div class="text-xs text-slate-500">Failure</div>
          <div class="text-lg font-semibold">{learningReport?.workflowFailures ?? 0}</div>
        </div>
        <div class="rounded-md border border-line bg-field p-3">
          <div class="text-xs text-slate-500">Corrections</div>
          <div class="text-lg font-semibold">{learningReport?.corrections ?? 0}</div>
        </div>
        <div class="rounded-md border border-line bg-field p-3">
          <div class="text-xs text-slate-500">Extensions</div>
          <div class="text-lg font-semibold">{learningReport?.extensionGenerations ?? 0}</div>
        </div>
      </div>
      <div class="border-t border-line p-4">
        <SectionHeader
          compact
          icon="learning"
          title="Suggestions"
          description="Candidate improvements from local learning reports."
          meta={`${learningReport?.suggestions?.length ?? 0} suggestions`}
          className="mb-2"
        />
        <div class="space-y-2">
          {#if !(learningReport?.suggestions?.length)}
            <EmptyState
              icon="learning"
              title="No learning suggestions"
              message="Yemaka will list improvement suggestions here after learning reports detect useful patterns."
            />
          {/if}
          {#each pagedItems(learningReport?.suggestions, 'learning-suggestions', pageSizeFor('learning-suggestions'), listPages) as suggestion}
            <div class="rounded-md border border-line bg-field px-3 py-2 text-sm text-slate-700">{suggestion}</div>
          {/each}
        </div>
        <PaginationControls
          page={currentPage('learning-suggestions', learningReport?.suggestions?.length ?? 0, pageSizeFor('learning-suggestions'), listPages)}
          total={learningReport?.suggestions?.length ?? 0}
          pageSize={pageSizeFor('learning-suggestions')}
          label="suggestions"
          onChange={(page) => setListPage('learning-suggestions', page)}
        />
      </div>
    </div>
    <div class="rounded-md border border-line bg-white p-4 text-sm">
      <SectionHeader
        compact
        icon="settings"
        title="Policy"
        description="Current local safety policy and autonomy bounds."
        className="mb-3"
      >
        <svelte:fragment slot="actions">
          <Badge variant={policyStatus?.fullAccess ? 'warning' : 'success'}>
            {policyStatus?.mode || 'safe'}
          </Badge>
        </svelte:fragment>
      </SectionHeader>
      <div class="mb-3 grid grid-cols-2 gap-y-2 text-slate-700">
        <span>Autonomy</span><span>{policyStatus?.maxAutonomousLevel ?? 0}</span>
        <span>Audit</span><span>{policyStatus?.auditEnabled ? 'on' : 'off'}</span>
        <span>Trading</span><span>{policyStatus?.allowLiveTrading ? 'allowed' : 'blocked'}</span>
        <span>Policy writes</span><span>{policyStatus?.generatedCanModifyCorePolicy ? 'allowed' : 'blocked'}</span>
      </div>
      <div class="flex flex-wrap gap-2">
        <ActionButton size="xs" variant="secondary" icon="check" onclick={() => setPolicyMode('safe')}>Safe</ActionButton>
        <ActionButton size="xs" variant="danger" icon="warning" onclick={() => setPolicyMode('full_access')}>Full Access</ActionButton>
      </div>
    </div>
  </div>

  <div class="mb-4 rounded-md border border-line bg-white">
    <div class="border-b border-line px-4 py-3">
      <SectionHeader
        compact
        icon="learning"
        title="QA Review"
        description="Replay failures, route corrections, and regression coverage status."
        meta={qaReview?.generatedAt || 'not loaded'}
      >
        <svelte:fragment slot="actions">
          <ActionButton variant="secondary" icon="retry" onclick={refreshDiagnostics}>Refresh</ActionButton>
        </svelte:fragment>
      </SectionHeader>
    </div>
    <div class="grid gap-3 p-4 text-sm md:grid-cols-3 xl:grid-cols-12">
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Replay failures</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.replayFailures ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Corrections</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.routeCorrections ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Pending</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.pendingCorrections ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Regression ideas</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.regressionSuggestions ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Repeated</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.repeatedSuggestions ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Approved</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.approvedSuggestions ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Covered</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.coveredSuggestions ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Dismissed</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.dismissedSuggestions ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Drafts</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.testDrafts ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Approved regressions</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.approvedRegressions ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Source patches</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.sourcePatches ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Apply plans</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.sourcePatchApplyPlans ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Source writes</div>
        <div class="text-lg font-semibold">{qaReview?.summary?.sourceTestWrites ?? 0}</div>
      </div>
      <div class="rounded-md border border-line bg-field p-3">
        <div class="text-xs text-slate-500">Latest eval</div>
        <div class="text-lg font-semibold">{qaReview?.latestEval?.status || 'none'}</div>
      </div>
    </div>
    {#if qaReview?.summary?.routeFailureCategories && Object.keys(qaReview.summary.routeFailureCategories).length}
      <div class="flex flex-wrap gap-2 border-t border-line px-4 py-3 text-xs">
        {#each Object.entries(qaReview.summary.routeFailureCategories) as [category, count]}
          <Badge variant="info">{humanizeIdentifier(category, category)}: {count}</Badge>
        {/each}
      </div>
    {/if}
    <div class="grid gap-4 border-t border-line p-4 lg:grid-cols-[minmax(0,1fr)_340px]">
      <div class="min-w-0">
        <SectionHeader
          compact
          icon="learning"
          title="Replay Failure Coverage"
          description="Failures that need generic regression coverage."
          meta={`${qaReview?.replayFailures?.length ?? 0} failures`}
          className="mb-3"
        />
        <div class="divide-y divide-line rounded-md border border-line">
          {#if !(qaReview?.replayFailures?.length)}
            <div class="p-4">
              <EmptyState
                icon="learning"
                title="No replay failures needing review"
                message="Replay failures and generated regression suggestions will appear here when QA finds gaps."
              />
            </div>
          {/if}
          {#each pagedItems(qaReview?.replayFailures, 'qa-replay-failures', pageSizeFor('qa-replay-failures'), listPages) as failure (failure.traceId)}
            <div class="grid gap-3 p-4 text-sm lg:grid-cols-[minmax(0,1fr)_170px]">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="font-semibold">{shortId(failure.traceId)}</span>
                  <Badge>{humanizeIdentifier(failure.coverageStatus, 'Coverage')}</Badge>
                  {#if failure.routeFailureCategory}
                    <Badge variant="warning">{humanizeIdentifier(failure.routeFailureCategory, 'Route failure')}</Badge>
                  {/if}
                  <Badge variant={failure.coveredByEvalOrTest ? 'success' : 'neutral'}>
                    {failure.coveredByEvalOrTest ? 'covered' : 'needs test'}
                  </Badge>
                  {#if failure.regressionSuggestions?.length}
                    <Badge variant="success">artifact suggested</Badge>
                  {/if}
                  {#if (failure.repeatCount ?? 1) > 1}
                    <Badge variant="warning">{failure.repeatCount} repeats</Badge>
                  {/if}
                  {#if failure.promotionPriority}
                    <Badge variant={failure.promotionPriority === 'high' ? 'danger' : failure.promotionPriority === 'medium' ? 'warning' : 'neutral'}>
                      {humanizeIdentifier(failure.promotionPriority, failure.promotionPriority)}
                    </Badge>
                  {/if}
                  {#if reviewStatusFor(failure)}
                    <Badge variant={reviewStatusFor(failure) === 'approved' ? 'success' : reviewStatusFor(failure) === 'covered' ? 'info' : 'neutral'}>
                      {humanizeIdentifier(reviewStatusFor(failure), reviewStatusFor(failure))}
                    </Badge>
                  {/if}
                  {#if testDraftStatusFor(failure)}
                    <Badge variant="info">{humanizeIdentifier(testDraftStatusFor(failure), testDraftStatusFor(failure))}</Badge>
                  {/if}
                  {#if approvedRegressionFor(failure)}
                    <Badge variant="success">{humanizeIdentifier(approvedRegressionFor(failure), approvedRegressionFor(failure))}</Badge>
                  {/if}
                </div>
                <div class="mt-1 text-slate-700">{failure.requestSummary}</div>
                <div class="mt-1 text-xs text-slate-500">
                  {humanizeIdentifier(failure.routeCategory || 'route', 'Route')} | {humanizeIdentifier(failure.failureStage || 'failure', 'Failure')} | {humanizeIdentifier(failure.failureCode || 'review', 'Review')}
                </div>
                {#if failure.failureMessage}
                  <div class="mt-2 rounded-md border border-line bg-field px-3 py-2 text-xs text-slate-600">{failure.failureMessage}</div>
                {/if}
                {#if failure.routeFailureReason || failure.suggestedGenericRegression}
                  <div class="mt-2 rounded-md border border-line bg-white px-3 py-2 text-xs text-slate-600">
                    {#if failure.routeFailureReason}
                      <div><span class="font-semibold text-slate-700">Category reason:</span> {failure.routeFailureReason}</div>
                    {/if}
                    {#if failure.suggestedGenericRegression}
                      <div class="mt-1"><span class="font-semibold text-slate-700">Suggested regression:</span> {failure.suggestedGenericRegression}</div>
                    {/if}
                  </div>
                {/if}
                {#if failure.regressionSuggestions?.[0]?.preview}
                  <StructuredDataView
                    className="mt-2"
                    value={failure.regressionSuggestions[0].preview}
                    title="Regression suggestion"
                    maxPreviewLines={10}
                  />
                  <div class="mt-2 flex flex-wrap items-center gap-2">
                    <ActionButton
                      size="sm"
                      icon="learning"
                      busy={Boolean(promotionBusy[promotionKey(failure)])}
                      busyLabel="Saving"
                      disabled={!failure.traceId}
                      disabledReason="Replay trace id is required"
                      onclick={() => handlePromoteRegression(failure)}
                    >
                      Promote artifact
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="secondary"
                      icon="check"
                      busy={Boolean(reviewBusy[reviewKey(failure, 'approved')])}
                      busyLabel="Approving"
                      disabled={!failure.traceId}
                      disabledReason="Replay trace id is required"
                      onclick={() => handleReviewRegression(failure, 'approved')}
                    >
                      Approve
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="secondary"
                      icon="learning"
                      busy={Boolean(reviewBusy[reviewKey(failure, 'covered')])}
                      busyLabel="Marking"
                      disabled={!failure.traceId}
                      disabledReason="Replay trace id is required"
                      onclick={() => handleReviewRegression(failure, 'covered')}
                    >
                      Mark covered
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="secondary"
                      icon="close"
                      busy={Boolean(reviewBusy[reviewKey(failure, 'dismissed')])}
                      busyLabel="Dismissing"
                      disabled={!failure.traceId}
                      disabledReason="Replay trace id is required"
                      onclick={() => handleReviewRegression(failure, 'dismissed')}
                    >
                      Dismiss
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="secondary"
                      icon="send"
                      busy={Boolean(draftBusy[promotionKey(failure)])}
                      busyLabel="Drafting"
                      disabled={reviewStatusFor(failure) !== 'approved' || Boolean(testDraftStatusFor(failure))}
                      disabledReason={reviewStatusFor(failure) !== 'approved' ? 'Approve the QA suggestion before creating a test draft.' : 'A test draft already exists for this suggestion.'}
                      onclick={() => handleGenerateTestDraft(failure)}
                    >
                      Test draft
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="primary"
                      icon="check"
                      busy={Boolean(approvalBusy[promotionKey(failure)])}
                      busyLabel="Approving"
                      disabled={!testDraftStatusFor(failure) || Boolean(approvedRegressionFor(failure))}
                      disabledReason={!testDraftStatusFor(failure) ? 'Create a test draft before approving the regression.' : 'This regression is already approved.'}
                      onclick={() => handleApproveRegression(failure)}
                    >
                      Approve regression
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="secondary"
                      icon="code"
                      busy={Boolean(sourcePatchBusy[promotionKey(failure)])}
                      busyLabel="Drafting"
                      disabled={!approvedRegressionFor(failure) || Boolean(sourcePatchStatusFor(failure))}
                      disabledReason={!approvedRegressionFor(failure) ? 'Approve the regression before creating a source patch draft.' : 'A source patch draft already exists for this regression.'}
                      onclick={() => handleGenerateSourcePatch(failure)}
                    >
                      Source patch
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="secondary"
                      icon="check"
                      busy={Boolean(sourcePatchApplyBusy[promotionKey(failure)])}
                      busyLabel="Approving"
                      disabled={!sourcePatchStatusFor(failure) || Boolean(sourcePatchApplyStatusFor(failure))}
                      disabledReason={!sourcePatchStatusFor(failure) ? 'Create a source patch draft before approving the manual apply plan.' : 'A manual apply plan already exists for this source patch.'}
                      onclick={() => handleApproveSourcePatchApplyPlan(failure)}
                    >
                      Apply plan
                    </ActionButton>
                    <span class="text-xs text-slate-500">Saves reviewed artifacts only; no test file is auto-written.</span>
                  </div>
                {/if}
              </div>
              <div class="text-xs text-slate-500">
                <div>Result: {failure.resultStatus || 'unknown'}</div>
                <div>Verify: {failure.verificationStatus || 'unknown'}</div>
                <div>Corrections: {failure.matchingCorrectionIds?.length ?? 0}</div>
                {#if failure.coverageSources?.length}
                  <div class="mt-2 break-words">Coverage: {failure.coverageSources.map((source) => humanizeIdentifier(source, source)).join(', ')}</div>
                {/if}
                {#if failure.regressionPromotion?.suggestedTestName}
                  <div class="mt-2 break-words">Test: {humanizeIdentifier(failure.regressionPromotion.suggestedTestName, failure.regressionPromotion.suggestedTestName)}</div>
                {/if}
                {#if failure.regressionPromotion?.expectedToolLane}
                  <div>Lane: {humanizeIdentifier(failure.regressionPromotion.expectedToolLane, failure.regressionPromotion.expectedToolLane)}</div>
                {/if}
                {#if failure.regressionPromotion?.expectedSource}
                  <div>Source: {humanizeIdentifier(failure.regressionPromotion.expectedSource, failure.regressionPromotion.expectedSource)}</div>
                {/if}
                {#if sourcePatchStatusFor(failure)}
                  <div class="mt-2 break-words">Source patch: {humanizeIdentifier(sourcePatchStatusFor(failure), sourcePatchStatusFor(failure))}</div>
                  {#if failure.sourcePatchPath || failure.regressionSuggestions?.[0]?.sourcePatchPath}
                    <div class="break-words">Patch artifact: {failure.sourcePatchPath || failure.regressionSuggestions?.[0]?.sourcePatchPath}</div>
                  {/if}
                {/if}
                {#if sourcePatchApplyStatusFor(failure)}
                  <div class="mt-2 break-words">Apply plan: {humanizeIdentifier(sourcePatchApplyStatusFor(failure), sourcePatchApplyStatusFor(failure))}</div>
                  {#if failure.sourcePatchApplyPath || failure.regressionSuggestions?.[0]?.sourcePatchApplyPath}
                    <div class="break-words">Apply plan artifact: {failure.sourcePatchApplyPath || failure.regressionSuggestions?.[0]?.sourcePatchApplyPath}</div>
                  {/if}
                {/if}
                {#if sourceTestWriteStatusFor(failure)}
                  <div class="mt-2 break-words">Source write: {humanizeIdentifier(sourceTestWriteStatusFor(failure), sourceTestWriteStatusFor(failure))}</div>
                  {#if failure.sourceTestWritePath || failure.regressionSuggestions?.[0]?.sourceTestWritePath}
                    <div class="break-words">Source write artifact: {failure.sourceTestWritePath || failure.regressionSuggestions?.[0]?.sourceTestWritePath}</div>
                  {/if}
                  {#if failure.sourceTestWriteSnapshotId || failure.regressionSuggestions?.[0]?.sourceTestWriteSnapshotId}
                    <div class="break-words">Snapshot: {failure.sourceTestWriteSnapshotId || failure.regressionSuggestions?.[0]?.sourceTestWriteSnapshotId}</div>
                  {/if}
                {/if}
                {#if failure.promotionFingerprint}
                  <div class="mt-2 break-words">Fingerprint: {shortId(failure.promotionFingerprint)}</div>
                {/if}
              </div>
              {#if sourcePatchApplyStatusFor(failure) && !sourceTestWriteStatusFor(failure)}
                {@const key = promotionKey(failure)}
                <div class="mt-4 rounded-md border border-line bg-field p-3">
                  <div class="mb-2 text-xs font-semibold uppercase tracking-[0.14em] text-slate-500">Developer source write</div>
                  <textarea
                    class="min-h-28 w-full rounded-md border border-line bg-white p-3 text-xs font-mono"
                    placeholder="Paste the full target Go test file content after replacing the QA scaffold with deterministic assertions."
                    value={sourceWriteContent[key] || ''}
                    oninput={(event) => {
                      sourceWriteContent = { ...sourceWriteContent, [key]: event.currentTarget.value };
                    }}
                  ></textarea>
                  <div class="mt-2 flex flex-wrap gap-2">
                    <ActionButton
                      size="sm"
                      variant="secondary"
                      icon="search"
                      busy={Boolean(sourceWriteBusy[`${key}:plan`])}
                      busyLabel="Previewing"
                      disabled={!sourceWriteContent[key]}
                      disabledReason="Paste full source-test file content before previewing the source write."
                      onclick={() => handlePlanSourceWrite(failure)}
                    >
                      Preview write
                    </ActionButton>
                    <ActionButton
                      size="sm"
                      variant="danger"
                      icon="check"
                      busy={Boolean(sourceWriteBusy[`${key}:apply`])}
                      busyLabel="Applying"
                      disabled={!sourceWritePreview[key]}
                      disabledReason="Preview the guarded source write before applying it."
                      onclick={() => handleApplySourceWrite(failure)}
                    >
                      Apply source test
                    </ActionButton>
                    <span class="self-center text-xs text-slate-500">Uses the workspace snapshot/diff writer and rejects QA scaffold text.</span>
                  </div>
                  {#if sourceWritePreview[key]}
                    <pre class="mt-3 max-h-72 overflow-auto rounded-md border border-line bg-white p-3 text-xs whitespace-pre-wrap">{sourceWritePreview[key]}</pre>
                  {/if}
                </div>
              {/if}
            </div>
          {/each}
        </div>
        <PaginationControls
          page={currentPage('qa-replay-failures', qaReview?.replayFailures?.length ?? 0, pageSizeFor('qa-replay-failures'), listPages)}
          total={qaReview?.replayFailures?.length ?? 0}
          pageSize={pageSizeFor('qa-replay-failures')}
          label="failures"
          onChange={(page) => setListPage('qa-replay-failures', page)}
        />
      </div>
      <div class="min-w-0">
        <SectionHeader
          compact
          icon="learning"
          title="Route Corrections"
          description="Approved user teaching and routing corrections."
          meta={`${qaReview?.routeCorrections?.length ?? 0} corrections`}
          className="mb-3"
        />
        <div class="divide-y divide-line rounded-md border border-line">
          {#if !(qaReview?.routeCorrections?.length)}
            <div class="p-4">
              <EmptyState
                icon="learning"
                title="No route corrections saved yet"
                message="Approved user teaching and routing corrections will appear here after they are captured."
              />
            </div>
          {/if}
          {#each pagedItems(qaReview?.routeCorrections, 'qa-route-corrections', pageSizeFor('qa-route-corrections'), listPages) as correction (correction.id)}
            <div class="p-4 text-sm">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-semibold">{humanizeIdentifier(correction.intendedRouteCategory || 'route', 'Route')}</span>
                <Badge>{humanizeIdentifier(correction.approvalStatus || 'pending', 'Pending')}</Badge>
                {#if correction.disabled}
                  <Badge variant="danger">disabled</Badge>
                {/if}
              </div>
              <div class="mt-1 text-slate-700">{correction.pattern}</div>
              {#if correction.requiredTools?.length || correction.forbiddenTools?.length}
                <div class="mt-1 text-xs text-slate-500">Tools: {correctionTools(correction)}</div>
              {/if}
            </div>
          {/each}
        </div>
        <PaginationControls
          page={currentPage('qa-route-corrections', qaReview?.routeCorrections?.length ?? 0, pageSizeFor('qa-route-corrections'), listPages)}
          total={qaReview?.routeCorrections?.length ?? 0}
          pageSize={pageSizeFor('qa-route-corrections')}
          label="corrections"
          onChange={(page) => setListPage('qa-route-corrections', page)}
        />
      </div>
    </div>
  </div>

  <div class="mb-4 grid gap-4 lg:grid-cols-2">
    <div class="rounded-md border border-line bg-white p-4">
      <SectionHeader
        compact
        icon="memory"
        title="Correction Memory"
        description="Save explicit user teaching for future routing behavior."
        className="mb-3"
      />
      <input class="mb-3 h-10 w-full rounded-md border border-line bg-field px-3 text-sm" bind:value={correctionConversationId} placeholder="conversation id" />
      <textarea class="mb-3 h-24 w-full resize-none rounded-md border border-line bg-field p-3 text-sm" bind:value={correctionContent} placeholder="Correction to remember"></textarea>
      <ActionButton
        variant="primary"
        icon="check"
        disabled={!correctionConversationId.trim() || !correctionContent.trim()}
        disabledReason="Enter a conversation id and correction content before saving."
        onclick={saveCorrection}
      >
        Save Correction
      </ActionButton>
    </div>
    <div class="rounded-md border border-line bg-white p-4">
      <SectionHeader
        compact
        icon="send"
        title="Trajectory Export"
        description="Export a conversation trajectory for replay and QA review."
        className="mb-3"
      />
      <input class="mb-3 h-10 w-full rounded-md border border-line bg-field px-3 text-sm" bind:value={trajectoryConversationId} placeholder="conversation id" />
      <ActionButton
        variant="secondary"
        icon="send"
        disabled={!trajectoryConversationId.trim()}
        disabledReason="Enter a conversation id before exporting a trajectory."
        onclick={exportTrajectory}
      >
        Export
      </ActionButton>
    </div>
  </div>

  <div class="mb-4 rounded-md border border-line bg-white">
    <div class="border-b border-line px-4 py-3">
      <SectionHeader
        compact
        icon="memory"
        title="Recent Learning Memory"
        description="Recent local memories created from workflows, corrections, and failures."
        meta={`${learningReport?.recentLearningMemory?.length ?? 0} memories`}
      />
    </div>
    <div class="divide-y divide-line">
      {#if !(learningReport?.recentLearningMemory?.length)}
        <div class="p-4">
          <EmptyState
            icon="memory"
            title="No learning memories recorded yet"
            message="Workflow outcomes, corrections, and failure memories will appear here when learning records are created."
          />
        </div>
      {/if}
      {#each pagedItems(learningReport?.recentLearningMemory, 'learning-memory', pageSizeFor('learning-memory'), listPages) as item}
        <div class="grid gap-3 p-4 text-sm lg:grid-cols-[180px_minmax(0,1fr)]">
          <div>
            <div class="font-semibold" title={item.kind}>{humanizeIdentifier(item.kind, 'Memory')}</div>
            <div class="mt-1 break-all text-xs text-slate-500">{humanizeIdentifier(item.source, item.source)}</div>
          </div>
          <StructuredDataView value={item.snippet} title="Learning memory" maxPreviewLines={8} />
        </div>
      {/each}
    </div>
  </div>
  <PaginationControls
    page={currentPage('learning-memory', learningReport?.recentLearningMemory?.length ?? 0, pageSizeFor('learning-memory'), listPages)}
    total={learningReport?.recentLearningMemory?.length ?? 0}
    pageSize={pageSizeFor('learning-memory')}
    label="memories"
    onChange={(page) => setListPage('learning-memory', page)}
  />
  {#if trajectoryPreview}
    <StructuredDataView value={trajectoryPreview} title="Trajectory preview" maxPreviewLines={22} />
  {/if}
</div>
