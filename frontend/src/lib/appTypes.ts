export type ConfiguredModelStatus = {
    role: string;
    name: string;
    installed: boolean;
    profile?: string;
    profileValid?: boolean;
    profileStatus?: string;
  };

export type Status = {
    configPath: string;
    profilePath: string;
    sqlitePath: string;
    sqliteOk: boolean;
    sqliteError: string;
    ollamaOk: boolean;
    ollamaError: string;
    defaultModel: string;
    lowMemoryModel: string;
    selectedModel: string;
    lowMemoryMode: boolean;
    setupComplete: boolean;
    modelReady: boolean;
    ragEnabled: boolean;
    shellEnabled: boolean;
    telemetry: boolean;
    modelStatuses: ConfiguredModelStatus[];
    skillCount: number;
  };

export type Skill = {
    name: string;
    version: string;
    description: string;
    triggers: string[];
    requiredTools: string[];
    enabled: boolean;
    valid: boolean;
    validationError: string;
    source: string;
    dir: string;
  };

export type SkillActionResult = {
    skill: Skill;
    message: string;
    path: string;
  };

export type DomainPack = {
    name: string;
    version: string;
    description: string;
    category: string;
    enabled: boolean;
    installed: boolean;
    valid: boolean;
    validationError: string;
    repairHint: string;
    skills: string[];
    requiredTools: string[];
    optionalTools: string[];
    approvalRequiredFor: string[];
    blockedActions: string[];
    sensitiveDataRules: string[];
    safetySummary: string;
    dir: string;
  };

export type DomainPackTemplate = {
    name: string;
    version: string;
    description: string;
    category: string;
    source: string;
    path: string;
    installed: boolean;
    enabled: boolean;
    valid: boolean;
    validationError: string;
    repairHint: string;
    defaultState: string;
    requiredTools: string[];
    skills: string[];
    workflows: string[];
    approvalRequiredFor: string[];
    blockedActions: string[];
    sensitiveDataRules: string[];
    safetySummary: string;
    installAction: string;
  };

export type DomainPackSkillReview = {
    ref: string;
    name?: string;
    version?: string;
    description?: string;
    triggers?: string[];
    requiredTools?: string[];
    permissions?: Record<string, unknown>;
    contextBudget?: Record<string, unknown>;
    instructionChars?: number;
    valid: boolean;
    validationError?: string;
    dir?: string;
  };

export type DomainPackReview = {
    name: string;
    version?: string;
    description?: string;
    category?: string;
    source?: string;
    path?: string;
    installed: boolean;
    enabled: boolean;
    valid: boolean;
    validationError?: string;
    repairHint?: string;
    defaultState?: string;
    requiredTools?: string[];
    optionalTools?: string[];
    approvalRequiredFor?: string[];
    blockedActions?: string[];
    sensitiveDataRules?: string[];
    safetySummary?: string;
    riskyToolReasons?: string[];
    permissions?: Record<string, unknown>;
    safety?: Record<string, unknown>;
    skills?: DomainPackSkillReview[];
    tests?: string[];
    status?: DomainPack;
    manifest?: Record<string, unknown>;
  };

export type DomainPackSkillStatus = {
    packName: string;
    ref: string;
    active: boolean;
    enabled: boolean;
    valid: boolean;
    validationError: string;
    packDir?: string;
  };

export type DomainPackActionResult = {
    pack: DomainPack;
    action?: string;
    message: string;
  };

export type Extension = {
    name: string;
    version: string;
    type: string;
    description: string;
    dir: string;
    manifestPath: string;
    enabled: boolean;
    valid: boolean;
    runnable: boolean;
    callable: boolean;
    runBlockedReason: string;
    validationError: string;
    createdAt: string;
    updatedAt: string;
  };

export type ExtensionActionResult = {
    extension: Extension;
    message: string;
  };

export type ExtensionRollbackResult = {
    name: string;
    snapshotId: string;
    targetDir: string;
    message: string;
  };

export type ExtensionPackageFile = {
    path: string;
    size: number;
    executable: boolean;
  };

export type ExtensionPackageInspection = {
    name: string;
    dir: string;
    status: string;
    fileCount: number;
    totalBytes: number;
    entrypointPath?: string;
    files: ExtensionPackageFile[];
    errors?: string[];
    warnings?: string[];
    checkedAt: string;
  };

export type ExtensionDetail = {
    status: Extension;
    manifest: Record<string, unknown>;
    inspection: ExtensionPackageInspection;
  };

export type ExtensionReviewFile = {
    path: string;
    size: number;
    executable: boolean;
    content?: string;
    truncated?: boolean;
    binary?: boolean;
    omittedReason?: string;
  };

export type ExtensionReview = {
    detail: ExtensionDetail;
    files: ExtensionReviewFile[];
    suggestedActions: string[];
    reuseCommand: string;
    sampleInput?: Record<string, unknown>;
    sampleInputJson?: string;
    canRun: boolean;
    canRegister: boolean;
    runBlockedReason?: string;
    registerBlockedReason?: string;
  };

export type ExtensionFailureTrend = {
    name: string;
    failures: number;
    lastStatus: string;
    lastError?: string;
    lastSeenAt: string;
    suggestReview: boolean;
    sources: string[];
  };

export type ExtensionProposal = {
    name: string;
    description: string;
    type: string;
    requiresApproval: boolean;
    reason: string;
    permissions?: string[];
    networkMode?: string;
    allowedDomains?: string[];
  };

export type CapabilityRuntimeStatus = {
    source?: string;
    state?: string;
    installed?: boolean;
    enabled?: boolean;
    valid?: boolean;
    requiresApproval?: boolean;
    reason?: string;
    configureHint?: string;
    repairHint?: string;
    requiredTools?: string[];
    optionalTools?: string[];
    skills?: string[];
    riskyTools?: string[];
    riskReason?: string;
  };

export type CapabilityProposal = {
    needed: boolean;
    kind: string;
    name: string;
    title: string;
    description: string;
    reason: string;
    suggestedAction: string;
    requiresApproval: boolean;
    canGenerate: boolean;
    files: string[];
    permissions: string[];
    generationCommand: string;
    networkMode: string;
    allowedDomains: string[];
    existingCapability?: string;
    capabilitySummary?: string;
    capabilityAction?: string;
    capabilityState?: string;
    runtimeStatus?: CapabilityRuntimeStatus;
    extension?: ExtensionProposal;
  };

export type CapabilityHandoff = {
    request: string;
    kind: string;
    name: string;
    title: string;
    description: string;
    reason: string;
    suggestedAction: string;
    requiresApproval: boolean;
    canGenerate: boolean;
    files: string[];
    permissions: string[];
    generationCommand: string;
    networkMode: string;
    allowedDomains: string[];
    existingCapability?: string;
    capabilitySummary?: string;
    capabilityAction?: string;
    capabilityState?: string;
    configureHint?: string;
    packSource?: string;
    packDir?: string;
    installCommand?: string;
    runtimeStatus?: CapabilityRuntimeStatus;
    status: 'pending' | 'generating' | 'generated' | 'failed' | 'not_available';
    message?: string;
    generatedName?: string;
    nextSteps?: string[];
    runAfterGenerate?: boolean;
    runInputJSON?: string;
    scheduleAfterGenerate?: boolean;
    scheduleType?: string;
    scheduleExpr?: string;
    scheduleEnabled?: boolean;
    scheduleInputJSON?: string;
    run?: ExtensionRunResult;
    schedule?: Job;
  };

export type SchedulerHandoff = {
    request: string;
    targetType: string;
    targetName: string;
    scheduleType: string;
    scheduleExpr: string;
    intent?: string;
    intentLabel?: string;
    intentSummary?: string;
    setupHint?: string;
    approved: boolean;
    enabled: boolean;
    missing: string[];
    canCreate: boolean;
    cliCommand: string;
    enableCommand: string;
    setupTemplate: string;
    inputJSON?: string;
    status: 'incomplete' | 'pending' | 'creating' | 'created' | 'failed';
    message?: string;
    job?: Job;
  };

export type CapabilityGenerationResult = {
    proposal: CapabilityProposal;
    generation: {
      extension: Extension;
      tests?: { status?: string };
      skillCandidate?: string;
      message?: string;
    };
    run?: ExtensionRunResult;
    schedule?: Job;
    repairs?: number;
    nextSteps: string[];
    message: string;
  };

export type ExtensionRunResult = {
    extension: string;
    status: string;
    output: Record<string, unknown>;
    error: string;
    startedAt: string;
    completedAt: string;
  };

export type PolicyStatus = {
    mode: string;
    auditEnabled: boolean;
    maxAutonomousLevel: number;
    allowAutonomousPosting: boolean;
    allowAutonomousDeployment: boolean;
    allowLiveTrading: boolean;
    generatedCanModifyCorePolicy: boolean;
    fullAccess: boolean;
  };

export type LearningReport = {
    generatedAt: string;
    workflowSuccesses: number;
    workflowFailures: number;
    corrections: number;
    extensionGenerations: number;
    recentLearningMemory: Array<{ id: string; kind: string; source: string; snippet: string; importance: number; updatedAt: string }>;
    extensionFailureTrend: ExtensionFailureTrend[];
    suggestions: string[];
    automaticTraining: boolean;
  };

export type Trajectory = {
    schema: string;
    conversationId: string;
    exportedAt: string;
    messages: Array<{ role: string; content: string; model?: string; createdAt: string }>;
    toolRuns: Array<{ toolName: string; status: string; riskLevel?: string; input?: unknown; output?: unknown; createdAt: string; completedAt: string }>;
    privacyFilter: unknown;
    training: { automaticTraining: boolean; note: string };
  };

export type ModelInfo = {
    name: string;
    modifiedAt: string;
    size: number;
    digest?: string;
  };

export type ModelDetails = {
    name: string;
    modifiedAt: string;
    size: number;
    digest: string;
    family: string;
    format: string;
    parameterSize: string;
    quantizationLevel: string;
    contextLength: number;
    template: string;
    system: string;
    license: string;
    parameters: string;
    modelfile: string;
  };

export type ModelGenerateResult = {
    text: string;
    model: string;
  };

export type ModelProfile = {
    schemaVersion: number;
    name: string;
    description?: string;
    baseModel: string;
    system?: string;
    parameters?: {
      temperature?: number;
      numCtx?: number;
    };
    metadata?: {
      purpose?: string;
      refinedFrom?: string;
      createdBy?: string;
    };
    tags?: string[];
  };

export type ModelProfileInput = {
    name: string;
    description?: string;
    baseModel: string;
    system?: string;
    temperature?: number;
    numCtx?: number;
    tags?: string[];
    purpose?: string;
    refinedFrom?: string;
  };

export type ModelProfileDraftInput = {
    conversationId: string;
    name?: string;
    baseModel?: string;
    temperature?: number;
    numCtx?: number;
  };

export type ModelProfileApplyInput = {
    name: string;
    role: string;
  };

export type ModelProfileDraftReport = {
    schema: string;
    conversationId: string;
    eligibility: string;
    reason: string;
    messageCount: number;
    assistantMessages: number;
    toolRunCount: number;
    failedToolRuns: number;
    observedTraits?: string[];
    privacyFilter?: {
      secretsRedacted?: boolean;
      pathsRedacted?: boolean;
    };
    safetySummary?: string[];
    readiness?: ModelProfileReadiness;
  };

export type ModelProfileReadiness = {
    schema: string;
    status: string;
    score: number;
    max: number;
    checks?: Array<{
      id: string;
      status: string;
      message: string;
      points: number;
      max: number;
    }>;
  };

export type ModelProfileComparison = {
    schema: string;
    baselineName?: string;
    candidateName: string;
    hasBaseline: boolean;
    sameArtifact: boolean;
    changedFields?: string[];
    addedTags?: string[];
    removedTags?: string[];
    systemCharsDelta?: number;
    readinessScoreDelta?: number;
  };

export type ModelProfileHistoryEvent = {
    schema: string;
    kind: string;
    profileName: string;
    path?: string;
    modifiedAt?: string;
    sizeBytes?: number;
    readinessScore: number;
    message?: string;
  };

export type ModelProfileResult = {
    profile: ModelProfile;
    path?: string;
    modelfile?: string;
    message?: string;
    applied?: boolean;
    appliedRole?: string;
    safetySummary?: string[];
    draftReport?: ModelProfileDraftReport;
    readiness?: ModelProfileReadiness;
    comparison?: ModelProfileComparison;
    history?: ModelProfileHistoryEvent[];
  };

export type SetupState = {
    setupComplete: boolean;
    ollamaOk: boolean;
    ollamaError: string;
    configPath: string;
    profilePath: string;
    lowMemoryMode: boolean;
    mode: string;
    selectedModel: string;
    modelReady: boolean;
    installedModels: ModelInfo[];
    recommendedMode: string;
    recommendedModel: string;
    configuredDefaults: ConfiguredModelStatus[];
  };

export type MemoryResult = {
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

export type KnowledgeStatus = {
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

export type KnowledgeRepairResult = {
    entitiesScanned: number;
    entitiesUpdated: number;
    edgesScanned: number;
    edgesUpdated: number;
    status: KnowledgeStatus;
    message: string;
  };

export type KnowledgeEntity = {
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

export type KnowledgeEdge = {
    id: string;
    fromEntityId: string;
    relation: string;
    toEntityId: string;
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

export type KnowledgeEntityInput = {
    name: string;
    kind: string;
    source: string;
    sourceKind?: string;
    sourceRef?: string;
    evidence: string;
  };

export type KnowledgeEdgeInput = {
    fromEntityId: string;
    relation: string;
    toEntityId: string;
    source: string;
    sourceKind?: string;
    sourceRef?: string;
    evidence: string;
  };

export type KnowledgeProposalInput = {
    text: string;
    source?: string;
    sourceKind?: string;
    sourceRef?: string;
    defaultKind?: string;
    limit?: number;
  };

export type KnowledgeEntityProposal = {
    name: string;
    kind: string;
    source: string;
    sourceKind: string;
    sourceRef: string;
    evidence: string;
    reason: string;
  };

export type KnowledgeRelationProposal = {
    fromName: string;
    relation: string;
    toName: string;
    source: string;
    sourceKind: string;
    sourceRef: string;
    evidence: string;
    reason: string;
  };

export type KnowledgeProposal = {
    source: string;
    sourceKind: string;
    sourceRef: string;
    reviewRequired: boolean;
    message: string;
    entityProposals: KnowledgeEntityProposal[];
    relationProposals: KnowledgeRelationProposal[];
  };

export type KnowledgeReviewItem = {
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

export type KnowledgeReviewTarget = {
    type: string;
    id: string;
  };

export type KnowledgeReviewBatchInput = {
    action: string;
    targets: KnowledgeReviewTarget[];
    reviewedBy?: string;
    note?: string;
  };

export type KnowledgeReviewBatchResult = {
    action: string;
    requested: number;
    entitiesMatched: number;
    edgesMatched: number;
    entitiesUpdated: number;
    edgesUpdated: number;
    items: KnowledgeReviewItem[];
  };

export type DocumentResult = {
    chunkId: string;
    path: string;
    content: string;
    rank: number;
    score: number;
    source: string;
    explanation: string[];
  };

export type DocumentInventoryItem = {
    id: string;
    path: string;
    workspaceRoot: string;
    managedSource: string;
    status: string;
    reason: string;
    missing: boolean;
    stale: boolean;
    sizeBytes: number;
    chunkCount: number;
    embeddingCount: number;
    indexedAt: string;
    updatedAt: string;
    contentHash: string;
    currentContentHash: string;
  };

export type DocumentPruneResult = {
    dryRun: boolean;
    documentsMatched: number;
    chunksMatched: number;
    ftsRowsMatched: number;
    embeddingsMatched: number;
    documentsRemoved: number;
    chunksRemoved: number;
    ftsRowsRemoved: number;
    embeddingsRemoved: number;
    documentIds: string[];
  };

export type DocumentPathSuggestion = {
    path: string;
    name: string;
    directory: string;
    kind: string;
    sizeBytes: number;
  };

export type IngestSummaryResult = {
    root: string;
    filesIndexed: number;
    filesSkipped: number;
    filesUnchanged: number;
    chunksCreated: number;
    bytesIndexed: number;
    skippedReasons?: string[];
  };

export type EmbeddingIndexSummary = {
    enabled: boolean;
    provider: string;
    model: string;
    chunksEmbedded: number;
    chunksSkipped: number;
    dimensions: number;
  };

export type WorkspaceGrant = {
    id: string;
    path: string;
    label: string;
    source: string;
    createdAt: string;
    bookmarkReady: boolean;
    bookmarkStale: boolean;
  };

export type DocumentSourceResult = {
    path: string;
    grant: WorkspaceGrant;
  };

export type ToolRun = {
    id: string;
    conversationId: string;
    sessionId?: string;
    userMessageId?: string;
    assistantMessageId?: string;
    parentMessageId?: string;
    variantIndex?: number;
    toolName: string;
    input: unknown;
    output: unknown;
    status: string;
    riskLevel: string;
    createdAt: string;
    completedAt: string;
  };

export type ToolCatalogEntry = {
    name: string;
    status: string;
    surfaces: string[];
    chatCallable: boolean;
    modelCallable: boolean;
    requiresApproval: boolean;
    enabledByDefault: boolean;
    mutating: boolean;
    notes: string;
  };

export type NotificationItem = {
    id: string;
    createdAt: string;
    type: string;
    severity: string;
    title: string;
    message?: string;
    source?: string;
    actionRequired?: boolean;
    read: boolean;
    dismissed: boolean;
    metadata?: Record<string, string>;
  };

export type ReplayTraceFile = {
    id: string;
    path: string;
    filename: string;
  };

export type OpsStage = {
    name: string;
    status: string;
    details?: string;
  };

export type OpsTimelineEvent = {
    id: string;
    occurredAt?: string;
    source: string;
    kind: string;
    severity: string;
    status?: string;
    title: string;
    summary?: string;
    entityType?: string;
    entityId?: string;
    correlationId?: string;
    metadata?: Record<string, string>;
    related?: OpsTimelineLink[] | null;
  };

export type OpsTimelineLink = {
    source: string;
    entityType: string;
    entityId: string;
    label?: string;
  };

export type OpsReleaseCheck = {
    name: string;
    status: string;
    details: string;
  };

export type OpsReleaseSummary = {
    version: string;
    ready: boolean;
    checks: OpsReleaseCheck[];
  };

export type ConnectorOpsSummary = {
    total: number;
    enabled: number;
    disabled: number;
    needsAuth: number;
    warning: number;
    broken: number;
    status: string;
    enabledList?: string[] | null;
  };

export type KnowledgeOpsStatus = {
    enabled: boolean;
    manualOnly: boolean;
    maxEntitiesPerQuery: number;
    maxEvidenceChars: number;
    database: string;
    entityCount: number;
    edgeCount: number;
    message: string;
  };

export type ModelProfileOpsSummary = {
    profileCount: number;
    configuredRoles: number;
    appliedRoles: number;
    missingBindings: number;
    missingProfiles?: string[] | null;
    availableProfiles?: string[] | null;
    status: string;
  };

export type ExtensionAuditRecord = {
    action: string;
    name: string;
    status: string;
    message?: string;
    timestamp: string;
  };

export type PolicyAuditRecord = {
    id?: string;
    line?: number;
    timestamp: string;
    request: Record<string, unknown>;
    decision: Record<string, unknown>;
  };

export type OpsStatus = {
    generatedAt: string;
    overall: string;
    stages: OpsStage[];
    timeline?: OpsTimelineEvent[] | null;
    heartbeat: HeartbeatReport;
    scheduler: JobStatus;
    connectors: ConnectorOpsSummary;
    knowledge: KnowledgeOpsStatus;
    modelProfiles: ModelProfileOpsSummary;
    recentJobRuns?: JobRun[] | null;
    latestEval?: QAEvalSummary | null;
    qaReview: QAReview['summary'];
    notifications?: NotificationItem[] | null;
    recentReplay?: ReplayTraceFile[] | null;
    recentToolRuns?: ToolRun[] | null;
    extensionAudit?: ExtensionAuditRecord[] | null;
    policyAudit?: PolicyAuditRecord[] | null;
    release?: OpsReleaseSummary | null;
  };

export type RouteCorrection = {
    id: string;
    sourceConversationId?: string;
    pattern: string;
    intendedRouteCategory: string;
    intendedTaskType?: string;
    requiredTools?: string[];
    forbiddenTools?: string[];
    tags?: string[];
    approvalStatus: string;
    disabled: boolean;
    updatedAt?: string;
  };

export type QARegressionSuggestion = {
    name: string;
    kind: string;
    prompt: string;
    sourceTraceId?: string;
    promotionFingerprint?: string;
    repeatCount?: number;
    promotionPriority?: string;
    coverageStatus: string;
    coveredByTest: boolean;
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
	    tags?: string[];
    preview?: string;
    promotion?: RouteRegressionPromotion;
  };

export type RouteRegressionPromotion = {
    suggestedTestName?: string;
    routeUnderTest?: string;
    continuationMode?: string;
    expectedSource?: string;
    expectedToolLane?: string;
    expectedOutcome?: string;
  };

export type QAReplayFailure = {
    traceId: string;
    requestSummary: string;
    routeCategory?: string;
    routeIntent?: string;
    routeTarget?: string;
    riskLevel?: string;
    failureStage?: string;
    failureCode?: string;
    failureSubject?: string;
    failureMessage?: string;
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
    resultStatus?: string;
    verificationStatus?: string;
    coverageStatus: string;
    regressionSuggestions: QARegressionSuggestion[];
    matchingCorrectionIds?: string[];
  };

export type QAEvalSummary = {
    id?: string;
    mode?: string;
    model?: string;
    startedAt?: string;
    completedAt?: string;
    passed: number;
    failed: number;
    skipped: number;
    status: string;
    metrics?: Record<string, unknown>;
    tasks?: Array<{ name: string; status: string; details?: string }>;
  };

export type QAReview = {
    generatedAt: string;
    summary: {
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
      evalStatus?: string;
    };
    replayFailures: QAReplayFailure[];
    routeCorrections: RouteCorrection[];
    latestEval?: QAEvalSummary;
  };

export type QARegressionPromotionArtifact = {
    schema: string;
    name: string;
    kind: string;
    traceId: string;
    category?: string;
    fingerprint?: string;
    path: string;
    preview: string;
    promotionMode: string;
    automaticTestWritten: boolean;
    coveredByEvalOrTest: boolean;
    promotion: RouteRegressionPromotion;
  };

export type QARegressionReviewRecord = {
    schema: string;
    name: string;
    kind: string;
    traceId: string;
    suggestionName?: string;
    category?: string;
    fingerprint?: string;
    status: string;
    reviewedBy?: string;
    reviewNote?: string;
    reviewedAt: string;
    coveredByEvalOrTest: boolean;
    promotion?: RouteRegressionPromotion;
    path?: string;
  };

export type QARegressionTestDraftRecord = {
    schema: string;
    name: string;
    kind: string;
    traceId: string;
    suggestionName?: string;
    category?: string;
    fingerprint?: string;
    status: string;
    reviewedBy?: string;
    reviewNote?: string;
    draftedAt: string;
    path?: string;
    preview: string;
    suggestedTestFile: string;
    suggestedTestName: string;
    automaticTestWritten: boolean;
    promotion: RouteRegressionPromotion;
  };

export type QARegressionApprovedRecord = {
    schema: string;
    name: string;
    kind: string;
    traceId: string;
    suggestionName?: string;
    category?: string;
    fingerprint?: string;
    status: string;
    reviewedBy?: string;
    reviewNote?: string;
    approvedAt: string;
    path?: string;
    testDraftPath?: string;
    suggestedTestFile: string;
    suggestedTestName: string;
	    automaticTestWritten: boolean;
	    promotion: RouteRegressionPromotion;
	  };

export type QARegressionSourcePatchRecord = {
	    schema: string;
	    name: string;
	    kind: string;
	    traceId: string;
	    suggestionName?: string;
	    category?: string;
	    fingerprint?: string;
	    status: string;
	    reviewedBy?: string;
	    reviewNote?: string;
	    draftedAt: string;
	    path?: string;
	    approvedRegressionPath?: string;
	    testDraftPath?: string;
	    suggestedTestFile: string;
	    suggestedTestName: string;
	    automaticTestWritten: boolean;
	    requiresManualApply: boolean;
	    testSnippet: string;
	    patchPreview: string;
	    preview: string;
	    promotion: RouteRegressionPromotion;
	  };

export type QARegressionSourcePatchApplyPlanRecord = {
	    schema: string;
	    name: string;
	    kind: string;
	    traceId: string;
	    suggestionName?: string;
	    category?: string;
	    fingerprint?: string;
	    status: string;
	    reviewedBy?: string;
	    reviewNote?: string;
	    approvedAt: string;
	    path?: string;
	    sourcePatchPath?: string;
	    approvedRegressionPath?: string;
	    testDraftPath?: string;
	    suggestedTestFile: string;
	    suggestedTestName: string;
	    automaticTestWritten: boolean;
	    sourceTestWritten: boolean;
	    requiresManualApply: boolean;
	    patchPreview: string;
	    manualApplyChecklist: string[];
	    preview: string;
	    promotion: RouteRegressionPromotion;
	  };

export type QARegressionSourceWritePlanResult = {
    traceId: string;
    applyPlanPath?: string;
    suggestedTestFile: string;
    suggestedTestName: string;
    requiresApproval: boolean;
    fileWritePlan: FileWritePlan;
  };

export type QARegressionSourceWriteApplyResult = {
    traceId: string;
    record: QARegressionSourceWriteRecord;
    fileWriteApply: FileWriteApplyResult;
  };

export type QARegressionSourceWriteRecord = {
    schema: string;
    name: string;
    kind: string;
    traceId: string;
    suggestionName?: string;
    category?: string;
    fingerprint?: string;
    status: string;
    reviewedBy?: string;
    reviewNote?: string;
    writtenAt: string;
    path?: string;
    applyPlanPath?: string;
    sourcePatchPath?: string;
    suggestedTestFile: string;
    suggestedTestName: string;
    automaticTestWritten: boolean;
    sourceTestWritten: boolean;
    requiresManualApply: boolean;
    snapshotId?: string;
    verificationStatus?: string;
    changed: boolean;
    targetBytes: number;
    diff?: string;
    preview?: string;
    promotion: RouteRegressionPromotion;
  };

export type ReplayTraceExplanation = {
    schema: string;
    id: string;
    requestSummary?: string;
    route?: {
      category?: string;
      intent?: string;
      domain?: string;
      target?: string;
      capability?: string;
      riskLevel?: string;
      confidence?: number;
      reasons?: string[];
      usesTool?: boolean;
      usesInternet?: boolean;
      readsFiles?: boolean;
      writesFiles?: boolean;
      needsApproval?: boolean;
      needsClarify?: boolean;
      generatesTool?: boolean;
      createsJob?: boolean;
    };
    toolsConsidered?: Array<Record<string, unknown>>;
    toolsUsed?: Array<{ name?: string; status?: string; riskLevel?: string; inputSummary?: string; outputSummary?: string; error?: string }>;
    memoryUsed?: Array<Record<string, unknown>>;
    ragDocsUsed?: Array<Record<string, unknown>>;
    context?: Record<string, unknown>;
    permissions?: Array<Record<string, unknown>>;
    verification?: { status?: string; checks?: string[]; notes?: string[] };
    result?: { status?: string; summary?: string; outputSummary?: string; attributes?: Array<Record<string, unknown>> };
    missingCapability?: Record<string, unknown> | null;
    diagnosticWarnings?: string[];
  };

export type Job = {
    id: string;
    name: string;
    scheduleType: string;
    scheduleExpr: string;
    targetType: string;
    targetName: string;
    input: Record<string, unknown>;
    enabled: boolean;
    approved: boolean;
    retryPolicy?: string;
    maxAttempts?: number;
    backoffSeconds?: number;
    createdAt: string;
    updatedAt: string;
    lastRunAt: string;
    nextDueAt?: string;
    archivedAt?: string;
    inputStatus?: string;
    inputValidationError?: string;
  };

export type JobRun = {
    id: string;
    jobId: string;
    jobName?: string;
    targetType?: string;
    targetName?: string;
    scheduleType?: string;
    status: string;
    output: unknown;
    startedAt: string;
    finishedAt: string;
    durationMs: number;
    error: string;
    attempt?: number;
    retryDueAt?: string;
  };

export type JobTickResult = {
    checkedAt: string;
    runs: JobRun[];
    skipped: number;
    missedRuns: number;
    missedPolicy: string;
  };

export type JobStatus = {
    enabled: boolean;
    totalJobs: number;
    enabledJobs: number;
    approvedJobs: number;
    runningJobs: number;
    maxParallelJobs: number;
    archivedJobs?: number;
    invalidInputJobs?: number;
    lastRunAt: string;
    lastRunStatus: string;
  };

export type HeartbeatReport = {
    overall: string;
    generatedAt: string;
    checks: Array<{ id: string; name: string; status: string; detail: string; checkedAt: string }>;
  };

export type InternetStatus = {
    enabled: boolean;
    defaultMode: string;
    allowMethods: string[];
    maxResponseBytes: number;
    timeoutSeconds: number;
    redirectLimit: number;
    cacheEnabled: boolean;
    cacheTtlSeconds: number;
    searchEnabled: boolean;
    searchProvider: string;
    searchProviderLabel?: string;
    searchEndpoint?: string;
    searchApiKeyEnv?: string;
    searchMaxResults?: number;
    searchSafeSearch?: boolean;
    searchProviderReady?: boolean;
    searchProviderConfigured?: boolean;
    searchProviderNeedsConfig?: boolean;
    searchProviderNeedsAuth?: boolean;
    searchStatus?: string;
    searchFallbackProviders?: string[];
    searchFallbackReadiness?: Array<{
      provider: string;
      label: string;
      configured: boolean;
      ready: boolean;
      needsConfig: boolean;
      needsAuth: boolean;
      networkChecked: boolean;
      status: string;
    }>;
    searchState?: string;
    searchHealthScore?: number;
    searchNextAction?: string;
    cacheFreshEntries?: number;
    cacheStaleEntries?: number;
    cacheLatestCachedAt?: string;
    logPath?: string;
    cacheDir?: string;
    robotsAware: boolean;
    crawlerAvailable?: boolean;
    crawlerManualOnly?: boolean;
    crawlerRequiresApproval?: boolean;
    crawlerMaxPages?: number;
    crawlerMaxDepth?: number;
    crawlerMaxDurationSeconds?: number;
    crawlerMaxLinksPerPage?: number;
    crawlerStatus?: string;
  };

export type InternetFetchResult = {
    url: string;
    method: string;
    statusCode: number;
    contentType: string;
    body: string;
    extractedText: string;
    bodyBytes: number;
    fromCache: boolean;
    fetchedAt: string;
  };

export type InternetCrawlPage = {
    url: string;
    finalUrl: string;
    depth: number;
    statusCode: number;
    contentType?: string;
    bodyBytes: number;
    fromCache: boolean;
    extractedText?: string;
    links?: string[];
    fetchedAt: string;
    error?: string;
    robots?: Record<string, unknown>;
  };

export type InternetCrawlSkip = {
    url?: string;
    depth: number;
    reason: string;
    robots?: Record<string, unknown>;
  };

export type InternetCrawlResult = {
    seedUrl: string;
    runId?: string;
    method: string;
    status: string;
    failureReason?: string;
    startedAt: string;
    finishedAt: string;
    allowedDomains: string[];
    maxPages: number;
    maxDepth: number;
    maxDurationSeconds: number;
    maxLinksPerPage: number;
    maxTextChars: number;
    visited: number;
    fetched: number;
    skipped: number;
    skipOverflow?: number;
    pages: InternetCrawlPage[];
    skips: InternetCrawlSkip[];
  };

export type InternetCrawlRunPageSummary = {
    url: string;
    finalUrl: string;
    depth: number;
    statusCode: number;
    contentType?: string;
    bodyBytes: number;
    fromCache: boolean;
    linkCount: number;
    textBytes: number;
    robotsAllowed: boolean;
    fetchedAt: string;
    error?: string;
  };

export type InternetCrawlRunRecord = {
    runId: string;
    seedUrl: string;
    method: string;
    status: string;
    failureReason?: string;
    startedAt: string;
    finishedAt: string;
    allowedDomains: string[];
    maxPages: number;
    maxDepth: number;
    maxDurationSeconds: number;
    maxLinksPerPage: number;
    maxTextChars: number;
    visited: number;
    fetched: number;
    skipped: number;
    skipOverflow?: number;
    pages?: InternetCrawlRunPageSummary[];
    skips?: InternetCrawlSkip[];
  };

export type InternetCacheSummary = {
    key: string;
    url: string;
    method: string;
    bodyBytes: number;
    cachedAt: string;
    expiresAt: string;
    expired: boolean;
    state?: string;
    ageSeconds?: number;
    expiresInSeconds?: number;
  };

export type InternetRequestRecord = {
    url: string;
    method: string;
    finalUrl?: string;
    statusCode: number;
    caller: string;
    provider?: string;
    timestamp: string;
    bytes: number;
    fromCache: boolean;
    allowed: boolean;
    error: string;
  };

export type ConnectorRegistryEntry = {
    status: {
      name: string;
      kind: string;
      enabled: boolean;
      bind: string;
      tokenReady: boolean;
      endpoint: string;
      secret: { provider: string; name: string; ready: boolean; detail: string };
      allowedDomains: string[];
      permissions: {
        inbound: boolean;
        outbound: boolean;
        posting: boolean;
        trading: boolean;
        deployment: boolean;
        mutating: boolean;
        scopes: string[];
      };
      rateLimit: { requestsPerMinute: number; requestsPerDay: number; burst: number };
      health: { status: string; detail: string };
    };
  };

export type ConnectorManifest = {
    name: string;
    version: string;
    kind: string;
    description: string;
    auth?: { provider?: string; name?: string; service?: string; account?: string };
    allowedDomains?: string[];
    permissions?: {
      inbound?: boolean;
      outbound?: boolean;
      posting?: boolean;
      trading?: boolean;
      deployment?: boolean;
      mutating?: boolean;
      scopes?: string[];
    };
    rateLimit?: { requestsPerMinute?: number; requestsPerDay?: number; burst?: number };
    endpoint: string;
    requiresApproval?: boolean;
  };

export type GeneratedConnectorTestReport = {
    status: string;
    commands?: string[];
    summary?: string;
    passedAt?: string;
    artifacts?: string[];
  };

export type GeneratedConnectorEntry = {
    name: string;
    manifest: ConnectorManifest;
    tests: GeneratedConnectorTestReport;
    approved: boolean;
    installed: boolean;
    enabled: boolean;
    validationError?: string;
    createdAt: string;
    updatedAt: string;
    installedAt?: string;
  };

export type GeneratedConnectorInstallInput = {
    manifest: ConnectorManifest;
    tests: GeneratedConnectorTestReport;
    approved: boolean;
  };

export type GeneratedConnectorActionResult = {
    connector: GeneratedConnectorEntry;
    message: string;
  };

export type PermissionItem = {
    requestId: string;
    toolName: string;
    command: string[];
    riskLevel: string;
    reason: string;
    nextStep: string;
    requiresConfirmation: boolean;
    workspaceOnly: boolean;
    diffPreview: boolean;
    snapshotBeforeWrite: boolean;
    rollbackSupported: boolean;
    destructive: boolean;
    policyLevel: string;
    policyExplanation: string;
    status: 'pending' | 'approved' | 'rejected';
    conversationId?: string;
    userMessageId?: string;
    assistantMessageId?: string;
    parentMessageId?: string;
  };

export type PermissionDecisionResult = {
    status: string;
    toolName?: string;
    toolStatus?: string;
    executed?: boolean;
    message?: string;
    conversationId?: string;
    assistantMessageId?: string;
    parentMessageId?: string;
    variantIndex?: number;
    activeVariant?: boolean;
    createdAt?: string;
    result?: {
      Context?: string;
      context?: string;
      Sources?: string[];
      sources?: string[];
      SourceKind?: string;
      sourceKind?: string;
      Status?: string;
      status?: string;
    };
  };

export type FileWritePlan = {
    path: string;
    diff: string;
    isNew: boolean;
    changed: boolean;
    targetBytes: number;
    snapshotId?: string;
  };

export type FileWriteVerification = {
    status: string;
    checks?: string[];
    reasons?: string[];
    snapshotId?: string;
    rollbackAvailable?: boolean;
  };

export type FileWriteApplyResult = FileWritePlan & {
    verification?: FileWriteVerification;
  };

export type ChatMessage = {
    id?: string;
    role: 'user' | 'assistant' | 'system';
    content: string;
    meta?: string;
    model?: string;
    skill?: string;
    sourceKind?: string;
    sources?: string[];
    attachments?: ChatAttachment[];
    trace?: string[];
    parentId?: string;
    variantIndex?: number;
    activeVariant?: boolean;
    variants?: ChatMessage[];
    activeVariantIndex?: number;
    capabilityHandoff?: CapabilityHandoff;
    schedulerHandoff?: SchedulerHandoff;
  };

export type ChatAttachment = {
    id: string;
    fileName: string;
    contentType: string;
    sizeBytes: number;
    status: string;
    summary?: string;
    preview?: string;
    sourceKind?: string;
    sources?: string[];
    retention?: string;
  };

export type ChatAttachmentUploadResult = {
    attachment: ChatAttachment;
    message: string;
  };

export type AskResult = {
    text: string;
    model: string;
    conversationId: string;
    skill: string;
    sourceKind: string;
    sources: string[];
    toolCalls?: string[];
    streamedVisibleChars?: number;
    userMessageId?: string;
    assistantMessageId?: string;
    parentMessageId?: string;
    variantIndex?: number;
    attachments?: ChatAttachment[];
  };

export type SendAskOptions = {
    appendUser?: boolean;
    assistantIndex?: number;
    promptIndex?: number;
    parentMessageId?: string;
    skillOverride?: string;
    fromQueue?: boolean;
    attachmentIds?: string[];
    attachments?: ChatAttachment[];
  };

export type OpenConversationOptions = {
    silent?: boolean;
  };

export type QueuedFollowUpPrompt = {
    id: string;
    content: string;
    draft: string;
    skill: string;
    createdAt: number;
    editing: boolean;
  };

export type ActivityEntry = {
    id: string;
    kind: string;
    label: string;
    at: string;
  };

export type ConversationSession = {
    id: string;
    title: string;
    createdAt: string;
    updatedAt: string;
    starred: boolean;
  };

export type ConversationDetail = {
    conversation: ConversationSession;
    messages: Array<{
      id: string;
      role: 'user' | 'assistant' | 'system';
      content: string;
      meta?: string;
      model: string;
      createdAt: string;
      parentId?: string;
      variantIndex?: number;
      activeVariant?: boolean;
      trace?: string[];
      sources?: string[];
      sourceKind?: string;
      attachments?: ChatAttachment[];
    }>;
  };

export type ModelRoleConfig = {
    Provider?: string;
    Name?: string;
    BaseURL?: string;
    Temperature?: number;
    provider?: string;
    name?: string;
    base_url?: string;
    baseUrl?: string;
    baseURL?: string;
    temperature?: number;
  };

export type UIConfig = {
    theme?: string;
    Theme?: string;
  };

export type FlagConfig = {
    enabled?: boolean;
    Enabled?: boolean;
  };

export type TokenConnectorConfig = FlagConfig & {
    token_env?: string;
    tokenEnv?: string;
    TokenEnv?: string;
  };

export type CloudFallbackConfig = FlagConfig & {
    provider?: string;
    Provider?: string;
    name?: string;
    Name?: string;
    base_url?: string;
    baseUrl?: string;
    baseURL?: string;
    BaseURL?: string;
    api_key_env?: string;
    apiKeyEnv?: string;
    APIKeyEnv?: string;
    timeout_seconds?: number;
    timeoutSeconds?: number;
    TimeoutSeconds?: number;
  };

export type InternetConfig = FlagConfig & {
    default_mode?: string;
    defaultMode?: string;
    DefaultMode?: string;
    search?: InternetSearchConfig;
    Search?: InternetSearchConfig;
  };

export type InternetSearchConfig = FlagConfig & {
    provider?: string;
    Provider?: string;
    endpoint?: string;
    Endpoint?: string;
    api_key_env?: string;
    apiKeyEnv?: string;
    APIKeyEnv?: string;
  };

export type EmbeddingsConfig = FlagConfig & {
    model?: string;
    Model?: string;
  };

export type RAGConfig = FlagConfig & {
    mode?: string;
    Mode?: string;
    chunk_size?: number;
    chunkSize?: number;
    ChunkSize?: number;
    chunk_overlap?: number;
    chunkOverlap?: number;
    ChunkOverlap?: number;
    top_k?: number;
    topK?: number;
    TopK?: number;
    embeddings?: EmbeddingsConfig;
    Embeddings?: EmbeddingsConfig;
  };

export type KnowledgeConfig = FlagConfig & {
    influence_enabled?: boolean;
    influenceEnabled?: boolean;
    InfluenceEnabled?: boolean;
    max_influence_entities?: number;
    maxInfluenceEntities?: number;
    MaxInfluenceEntities?: number;
    max_influence_chars?: number;
    maxInfluenceChars?: number;
    MaxInfluenceChars?: number;
  };

export type ConnectorsConfig = FlagConfig & {
    local_api?: TokenConnectorConfig;
    localApi?: TokenConnectorConfig;
    LocalAPI?: TokenConnectorConfig;
    mcp_server?: FlagConfig;
    mcpServer?: FlagConfig;
    MCPServer?: FlagConfig;
    slack?: TokenConnectorConfig;
    Slack?: TokenConnectorConfig;
    discord?: TokenConnectorConfig;
    Discord?: TokenConnectorConfig;
    telegram?: TokenConnectorConfig;
    Telegram?: TokenConnectorConfig;
    email?: TokenConnectorConfig;
    Email?: TokenConnectorConfig;
  };

export type WorkspaceConfig = {
    max_files_scanned?: number;
    maxFilesScanned?: number;
    MaxFilesScanned?: number;
    max_context_files?: number;
    maxContextFiles?: number;
    MaxContextFiles?: number;
    max_context_chars?: number;
    maxContextChars?: number;
    MaxContextChars?: number;
  };

export type SettingsFeatureSupport = {
    responseMode?: boolean;
    showThinkingTrace?: boolean;
  };

export type SettingsView = {
    settingsSchemaVersion?: number;
    featureSupport?: SettingsFeatureSupport;
    lowMemoryMode: boolean;
    maxContextTokens: number;
    responseMode?: string;
    showThinkingTrace?: boolean;
    ui?: UIConfig;
    UI?: UIConfig;
    modelRoles: Record<string, ModelRoleConfig>;
    cloudFallback?: CloudFallbackConfig;
    connectors?: ConnectorsConfig;
    internet?: InternetConfig;
    rag?: RAGConfig;
    knowledge?: KnowledgeConfig;
    workspace?: WorkspaceConfig;
    shellEnabled: boolean;
    telemetry: boolean;
  };
