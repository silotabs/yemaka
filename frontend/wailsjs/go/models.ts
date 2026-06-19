export namespace agent {
	
	export class CapabilityRuntimeStatus {
	    source?: string;
	    state?: string;
	    installed: boolean;
	    enabled: boolean;
	    valid: boolean;
	    requiresApproval: boolean;
	    reason?: string;
	    configureHint?: string;
	    repairHint?: string;
	    requiredTools?: string[];
	    optionalTools?: string[];
	    skills?: string[];
	    riskyTools?: string[];
	    riskReason?: string;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityRuntimeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.state = source["state"];
	        this.installed = source["installed"];
	        this.enabled = source["enabled"];
	        this.valid = source["valid"];
	        this.requiresApproval = source["requiresApproval"];
	        this.reason = source["reason"];
	        this.configureHint = source["configureHint"];
	        this.repairHint = source["repairHint"];
	        this.requiredTools = source["requiredTools"];
	        this.optionalTools = source["optionalTools"];
	        this.skills = source["skills"];
	        this.riskyTools = source["riskyTools"];
	        this.riskReason = source["riskReason"];
	    }
	}
	export class CapabilityGapProposal {
	    needed: boolean;
	    kind: string;
	    name?: string;
	    title: string;
	    description: string;
	    reason: string;
	    suggestedAction: string;
	    canGenerate: boolean;
	    requiresApproval: boolean;
	    files?: string[];
	    permissions?: string[];
	    generationCommand?: string;
	    networkMode?: string;
	    allowedDomains?: string[];
	    existingCapability?: string;
	    capabilitySummary?: string;
	    capabilityAction?: string;
	    capabilityState?: string;
	    configureHint?: string;
	    packSource?: string;
	    packDir?: string;
	    installCommand?: string;
	    runtimeStatus?: CapabilityRuntimeStatus;
	    extension?: extensions.Proposal;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityGapProposal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.needed = source["needed"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.reason = source["reason"];
	        this.suggestedAction = source["suggestedAction"];
	        this.canGenerate = source["canGenerate"];
	        this.requiresApproval = source["requiresApproval"];
	        this.files = source["files"];
	        this.permissions = source["permissions"];
	        this.generationCommand = source["generationCommand"];
	        this.networkMode = source["networkMode"];
	        this.allowedDomains = source["allowedDomains"];
	        this.existingCapability = source["existingCapability"];
	        this.capabilitySummary = source["capabilitySummary"];
	        this.capabilityAction = source["capabilityAction"];
	        this.capabilityState = source["capabilityState"];
	        this.configureHint = source["configureHint"];
	        this.packSource = source["packSource"];
	        this.packDir = source["packDir"];
	        this.installCommand = source["installCommand"];
	        this.runtimeStatus = this.convertValues(source["runtimeStatus"], CapabilityRuntimeStatus);
	        this.extension = this.convertValues(source["extension"], extensions.Proposal);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CapabilityScheduledJob {
	    id: string;
	    name: string;
	    scheduleType: string;
	    scheduleExpr: string;
	    targetType: string;
	    targetName: string;
	    input?: Record<string, any>;
	    enabled: boolean;
	    approved: boolean;
	    nextDueAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityScheduledJob(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scheduleType = source["scheduleType"];
	        this.scheduleExpr = source["scheduleExpr"];
	        this.targetType = source["targetType"];
	        this.targetName = source["targetName"];
	        this.input = source["input"];
	        this.enabled = source["enabled"];
	        this.approved = source["approved"];
	        this.nextDueAt = source["nextDueAt"];
	    }
	}
	export class CapabilityGenerationResult {
	    proposal: CapabilityGapProposal;
	    generation: extensions.GenerationResult;
	    run?: extensions.RunResult;
	    schedule?: CapabilityScheduledJob;
	    repairs?: number;
	    nextSteps?: string[];
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityGenerationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.proposal = this.convertValues(source["proposal"], CapabilityGapProposal);
	        this.generation = this.convertValues(source["generation"], extensions.GenerationResult);
	        this.run = this.convertValues(source["run"], extensions.RunResult);
	        this.schedule = this.convertValues(source["schedule"], CapabilityScheduledJob);
	        this.repairs = source["repairs"];
	        this.nextSteps = source["nextSteps"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ExecutionResult {
	    Context: string;
	    Sources: string[];
	    SourceKind: string;
	    Status: string;
	
	    static createFrom(source: any = {}) {
	        return new ExecutionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Context = source["Context"];
	        this.Sources = source["Sources"];
	        this.SourceKind = source["SourceKind"];
	        this.Status = source["Status"];
	    }
	}
	export class PermissionRequest {
	    request_id: string;
	    tool_name: string;
	    command?: string[];
	    risk_level: string;
	    reason: string;
	    requires_confirmation: boolean;
	    workspace_only: boolean;
	    diff_preview: boolean;
	    snapshot_before_write: boolean;
	    rollback_supported: boolean;
	    destructive: boolean;
	    next_step: string;
	    policy_level?: number;
	    policy_explanation?: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.request_id = source["request_id"];
	        this.tool_name = source["tool_name"];
	        this.command = source["command"];
	        this.risk_level = source["risk_level"];
	        this.reason = source["reason"];
	        this.requires_confirmation = source["requires_confirmation"];
	        this.workspace_only = source["workspace_only"];
	        this.diff_preview = source["diff_preview"];
	        this.snapshot_before_write = source["snapshot_before_write"];
	        this.rollback_supported = source["rollback_supported"];
	        this.destructive = source["destructive"];
	        this.next_step = source["next_step"];
	        this.policy_level = source["policy_level"];
	        this.policy_explanation = source["policy_explanation"];
	    }
	}

}

export namespace config {
	
	export class CloudFallbackConfig {
	    Enabled: boolean;
	    Provider: string;
	    Name: string;
	    BaseURL: string;
	    APIKeyEnv: string;
	    Temperature: number;
	    TimeoutSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new CloudFallbackConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Provider = source["Provider"];
	        this.Name = source["Name"];
	        this.BaseURL = source["BaseURL"];
	        this.APIKeyEnv = source["APIKeyEnv"];
	        this.Temperature = source["Temperature"];
	        this.TimeoutSeconds = source["TimeoutSeconds"];
	    }
	}
	export class ConnectorRateLimitConfig {
	    requestsPerMinute: number;
	    requestsPerDay: number;
	    burst: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectorRateLimitConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requestsPerMinute = source["requestsPerMinute"];
	        this.requestsPerDay = source["requestsPerDay"];
	        this.burst = source["burst"];
	    }
	}
	export class ConnectorPermissionsConfig {
	    inbound: boolean;
	    outbound: boolean;
	    posting: boolean;
	    trading: boolean;
	    deployment: boolean;
	    mutating: boolean;
	    scopes: string[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectorPermissionsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.inbound = source["inbound"];
	        this.outbound = source["outbound"];
	        this.posting = source["posting"];
	        this.trading = source["trading"];
	        this.deployment = source["deployment"];
	        this.mutating = source["mutating"];
	        this.scopes = source["scopes"];
	    }
	}
	export class SecretRefConfig {
	    provider: string;
	    name: string;
	    service?: string;
	    account?: string;
	
	    static createFrom(source: any = {}) {
	        return new SecretRefConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.name = source["name"];
	        this.service = source["service"];
	        this.account = source["account"];
	    }
	}
	export class ConnectorConfig {
	    Enabled: boolean;
	    Kind: string;
	    Bind: string;
	    RequireToken: boolean;
	    TokenEnv: string;
	    SecretRef: SecretRefConfig;
	    AllowedDomains: string[];
	    Permissions: ConnectorPermissionsConfig;
	    RateLimit: ConnectorRateLimitConfig;
	    MaxBodyBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Kind = source["Kind"];
	        this.Bind = source["Bind"];
	        this.RequireToken = source["RequireToken"];
	        this.TokenEnv = source["TokenEnv"];
	        this.SecretRef = this.convertValues(source["SecretRef"], SecretRefConfig);
	        this.AllowedDomains = source["AllowedDomains"];
	        this.Permissions = this.convertValues(source["Permissions"], ConnectorPermissionsConfig);
	        this.RateLimit = this.convertValues(source["RateLimit"], ConnectorRateLimitConfig);
	        this.MaxBodyBytes = source["MaxBodyBytes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class ConnectorsConfig {
	    Enabled: boolean;
	    LocalAPI: ConnectorConfig;
	    MCPServer: ConnectorConfig;
	    Slack: ConnectorConfig;
	    Discord: ConnectorConfig;
	    Telegram: ConnectorConfig;
	    Email: ConnectorConfig;
	
	    static createFrom(source: any = {}) {
	        return new ConnectorsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.LocalAPI = this.convertValues(source["LocalAPI"], ConnectorConfig);
	        this.MCPServer = this.convertValues(source["MCPServer"], ConnectorConfig);
	        this.Slack = this.convertValues(source["Slack"], ConnectorConfig);
	        this.Discord = this.convertValues(source["Discord"], ConnectorConfig);
	        this.Telegram = this.convertValues(source["Telegram"], ConnectorConfig);
	        this.Email = this.convertValues(source["Email"], ConnectorConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class EmbeddingConfig {
	    Enabled: boolean;
	    Provider: string;
	    Model: string;
	    BatchSize: number;
	    MaxTextChars: number;
	    CandidateLimit: number;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Provider = source["Provider"];
	        this.Model = source["Model"];
	        this.BatchSize = source["BatchSize"];
	        this.MaxTextChars = source["MaxTextChars"];
	        this.CandidateLimit = source["CandidateLimit"];
	    }
	}
	export class HeartbeatChecksConfig {
	    Ollama: boolean;
	    SQLite: boolean;
	    Scheduler: boolean;
	    Internet: boolean;
	    Connectors: boolean;
	    Extensions: boolean;
	    DiskSpace: boolean;
	    MemoryPressure: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HeartbeatChecksConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Ollama = source["Ollama"];
	        this.SQLite = source["SQLite"];
	        this.Scheduler = source["Scheduler"];
	        this.Internet = source["Internet"];
	        this.Connectors = source["Connectors"];
	        this.Extensions = source["Extensions"];
	        this.DiskSpace = source["DiskSpace"];
	        this.MemoryPressure = source["MemoryPressure"];
	    }
	}
	export class HeartbeatConfig {
	    Enabled: boolean;
	    IntervalSeconds: number;
	    Checks: HeartbeatChecksConfig;
	
	    static createFrom(source: any = {}) {
	        return new HeartbeatConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.IntervalSeconds = source["IntervalSeconds"];
	        this.Checks = this.convertValues(source["Checks"], HeartbeatChecksConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class InternetCacheConfig {
	    Enabled: boolean;
	    TTLSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new InternetCacheConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.TTLSeconds = source["TTLSeconds"];
	    }
	}
	export class InternetPolicyConfig {
	    RespectRobotsTxt: boolean;
	    BlockPrivateIPRanges: boolean;
	    BlockLocalNetworkByDefault: boolean;
	    LogRequests: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InternetPolicyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.RespectRobotsTxt = source["RespectRobotsTxt"];
	        this.BlockPrivateIPRanges = source["BlockPrivateIPRanges"];
	        this.BlockLocalNetworkByDefault = source["BlockLocalNetworkByDefault"];
	        this.LogRequests = source["LogRequests"];
	    }
	}
	export class InternetSearchConfig {
	    Enabled: boolean;
	    Provider: string;
	    FallbackProviders: string[];
	    Endpoint: string;
	    APIKeyEnv: string;
	    MaxResults: number;
	    SafeSearch: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InternetSearchConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Provider = source["Provider"];
	        this.FallbackProviders = source["FallbackProviders"];
	        this.Endpoint = source["Endpoint"];
	        this.APIKeyEnv = source["APIKeyEnv"];
	        this.MaxResults = source["MaxResults"];
	        this.SafeSearch = source["SafeSearch"];
	    }
	}
	export class InternetConfig {
	    Enabled: boolean;
	    DefaultMode: string;
	    AllowMethods: string[];
	    MaxResponseBytes: number;
	    TimeoutSeconds: number;
	    RedirectLimit: number;
	    Cache: InternetCacheConfig;
	    Search: InternetSearchConfig;
	    Policy: InternetPolicyConfig;
	
	    static createFrom(source: any = {}) {
	        return new InternetConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.DefaultMode = source["DefaultMode"];
	        this.AllowMethods = source["AllowMethods"];
	        this.MaxResponseBytes = source["MaxResponseBytes"];
	        this.TimeoutSeconds = source["TimeoutSeconds"];
	        this.RedirectLimit = source["RedirectLimit"];
	        this.Cache = this.convertValues(source["Cache"], InternetCacheConfig);
	        this.Search = this.convertValues(source["Search"], InternetSearchConfig);
	        this.Policy = this.convertValues(source["Policy"], InternetPolicyConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class KnowledgeGraphConfig {
	    Enabled: boolean;
	    InfluenceEnabled: boolean;
	    MaxEntitiesPerQuery: number;
	    MaxEvidenceChars: number;
	    MaxInfluenceEntities: number;
	    MaxInfluenceChars: number;
	    ManualOnly: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeGraphConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.InfluenceEnabled = source["InfluenceEnabled"];
	        this.MaxEntitiesPerQuery = source["MaxEntitiesPerQuery"];
	        this.MaxEvidenceChars = source["MaxEvidenceChars"];
	        this.MaxInfluenceEntities = source["MaxInfluenceEntities"];
	        this.MaxInfluenceChars = source["MaxInfluenceChars"];
	        this.ManualOnly = source["ManualOnly"];
	    }
	}
	export class ModelConfig {
	    Provider: string;
	    Name: string;
	    BaseURL: string;
	    Temperature: number;
	    Profile: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Provider = source["Provider"];
	        this.Name = source["Name"];
	        this.BaseURL = source["BaseURL"];
	        this.Temperature = source["Temperature"];
	        this.Profile = source["Profile"];
	    }
	}
	export class VectorDBConfig {
	    Enabled: boolean;
	    Provider: string;
	    ResearchOnly: boolean;
	    LocalOnly: boolean;
	    AllowBackgroundService: boolean;
	    MaxRAMMB: number;
	    MaxStorageMB: number;
	    MinBenefitPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new VectorDBConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Provider = source["Provider"];
	        this.ResearchOnly = source["ResearchOnly"];
	        this.LocalOnly = source["LocalOnly"];
	        this.AllowBackgroundService = source["AllowBackgroundService"];
	        this.MaxRAMMB = source["MaxRAMMB"];
	        this.MaxStorageMB = source["MaxStorageMB"];
	        this.MinBenefitPercent = source["MinBenefitPercent"];
	    }
	}
	export class RerankConfig {
	    CandidateLimit: number;
	    SourceDiversity: number;
	
	    static createFrom(source: any = {}) {
	        return new RerankConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CandidateLimit = source["CandidateLimit"];
	        this.SourceDiversity = source["SourceDiversity"];
	    }
	}
	export class RAGConfig {
	    Enabled: boolean;
	    Mode: string;
	    ChunkSize: number;
	    ChunkOverlap: number;
	    TopK: number;
	    MaxFileBytes: number;
	    MaxTotalBytes: number;
	    MaxChunksPerFile: number;
	    Rerank: RerankConfig;
	    Embeddings: EmbeddingConfig;
	    VectorDB: VectorDBConfig;
	
	    static createFrom(source: any = {}) {
	        return new RAGConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Mode = source["Mode"];
	        this.ChunkSize = source["ChunkSize"];
	        this.ChunkOverlap = source["ChunkOverlap"];
	        this.TopK = source["TopK"];
	        this.MaxFileBytes = source["MaxFileBytes"];
	        this.MaxTotalBytes = source["MaxTotalBytes"];
	        this.MaxChunksPerFile = source["MaxChunksPerFile"];
	        this.Rerank = this.convertValues(source["Rerank"], RerankConfig);
	        this.Embeddings = this.convertValues(source["Embeddings"], EmbeddingConfig);
	        this.VectorDB = this.convertValues(source["VectorDB"], VectorDBConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class SchedulerConfig {
	    Enabled: boolean;
	    MaxParallelJobs: number;
	    LowMemoryMaxParallelJobs: number;
	    DefaultJobTimeoutSeconds: number;
	    RequireApprovalForNewJobs: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SchedulerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.MaxParallelJobs = source["MaxParallelJobs"];
	        this.LowMemoryMaxParallelJobs = source["LowMemoryMaxParallelJobs"];
	        this.DefaultJobTimeoutSeconds = source["DefaultJobTimeoutSeconds"];
	        this.RequireApprovalForNewJobs = source["RequireApprovalForNewJobs"];
	    }
	}
	
	export class UIConfig {
	    Theme: string;
	
	    static createFrom(source: any = {}) {
	        return new UIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Theme = source["Theme"];
	    }
	}
	
	export class WorkspaceConfig {
	    MaxFilesScanned: number;
	    MaxFileBytes: number;
	    MaxTotalScanBytes: number;
	    MaxSearchResults: number;
	    MaxContextFiles: number;
	    MaxContextChars: number;
	    IncludeHidden: boolean;
	    FollowSymlinks: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MaxFilesScanned = source["MaxFilesScanned"];
	        this.MaxFileBytes = source["MaxFileBytes"];
	        this.MaxTotalScanBytes = source["MaxTotalScanBytes"];
	        this.MaxSearchResults = source["MaxSearchResults"];
	        this.MaxContextFiles = source["MaxContextFiles"];
	        this.MaxContextChars = source["MaxContextChars"];
	        this.IncludeHidden = source["IncludeHidden"];
	        this.FollowSymlinks = source["FollowSymlinks"];
	    }
	}

}

export namespace connectors {
	
	export class TestReport {
	    status: string;
	    commands?: string[];
	    summary?: string;
	    passedAt?: string;
	    artifacts?: string[];
	
	    static createFrom(source: any = {}) {
	        return new TestReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.commands = source["commands"];
	        this.summary = source["summary"];
	        this.passedAt = source["passedAt"];
	        this.artifacts = source["artifacts"];
	    }
	}
	export class Manifest {
	    name: string;
	    version: string;
	    kind: string;
	    description: string;
	    auth: secrets.Reference;
	    allowedDomains: string[];
	    permissions: config.ConnectorPermissionsConfig;
	    rateLimit: config.ConnectorRateLimitConfig;
	    endpoint: string;
	    requiresApproval: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.kind = source["kind"];
	        this.description = source["description"];
	        this.auth = this.convertValues(source["auth"], secrets.Reference);
	        this.allowedDomains = source["allowedDomains"];
	        this.permissions = this.convertValues(source["permissions"], config.ConnectorPermissionsConfig);
	        this.rateLimit = this.convertValues(source["rateLimit"], config.ConnectorRateLimitConfig);
	        this.endpoint = source["endpoint"];
	        this.requiresApproval = source["requiresApproval"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GeneratedEntry {
	    name: string;
	    manifest: Manifest;
	    tests: TestReport;
	    approved: boolean;
	    installed: boolean;
	    enabled: boolean;
	    validationError?: string;
	    createdAt: string;
	    updatedAt: string;
	    installedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new GeneratedEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.manifest = this.convertValues(source["manifest"], Manifest);
	        this.tests = this.convertValues(source["tests"], TestReport);
	        this.approved = source["approved"];
	        this.installed = source["installed"];
	        this.enabled = source["enabled"];
	        this.validationError = source["validationError"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.installedAt = source["installedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class GeneratedActionResult {
	    connector: GeneratedEntry;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new GeneratedActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connector = this.convertValues(source["connector"], GeneratedEntry);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class GeneratedInstallInput {
	    manifest: Manifest;
	    tests: TestReport;
	    approved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GeneratedInstallInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.manifest = this.convertValues(source["manifest"], Manifest);
	        this.tests = this.convertValues(source["tests"], TestReport);
	        this.approved = source["approved"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Health {
	    status: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Health(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.detail = source["detail"];
	    }
	}
	
	export class Status {
	    name: string;
	    kind: string;
	    enabled: boolean;
	    bind: string;
	    requireToken: boolean;
	    tokenEnv: string;
	    tokenReady: boolean;
	    secret: secrets.Status;
	    allowedDomains: string[];
	    permissions: config.ConnectorPermissionsConfig;
	    rateLimit: config.ConnectorRateLimitConfig;
	    maxBodyBytes: number;
	    endpoint: string;
	    health: Health;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.enabled = source["enabled"];
	        this.bind = source["bind"];
	        this.requireToken = source["requireToken"];
	        this.tokenEnv = source["tokenEnv"];
	        this.tokenReady = source["tokenReady"];
	        this.secret = this.convertValues(source["secret"], secrets.Status);
	        this.allowedDomains = source["allowedDomains"];
	        this.permissions = this.convertValues(source["permissions"], config.ConnectorPermissionsConfig);
	        this.rateLimit = this.convertValues(source["rateLimit"], config.ConnectorRateLimitConfig);
	        this.maxBodyBytes = source["maxBodyBytes"];
	        this.endpoint = source["endpoint"];
	        this.health = this.convertValues(source["health"], Health);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RegistryEntry {
	    status: Status;
	    manifest: Manifest;
	
	    static createFrom(source: any = {}) {
	        return new RegistryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], Status);
	        this.manifest = this.convertValues(source["manifest"], Manifest);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace desktop {
	
	export class ChatAttachmentResult {
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
	    createdAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatAttachmentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fileName = source["fileName"];
	        this.contentType = source["contentType"];
	        this.sizeBytes = source["sizeBytes"];
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.preview = source["preview"];
	        this.sourceKind = source["sourceKind"];
	        this.sources = source["sources"];
	        this.retention = source["retention"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class AskResult {
	    conversationId: string;
	    text: string;
	    model: string;
	    skill: string;
	    sources: string[];
	    sourceKind: string;
	    toolCalls: string[];
	    userMessageId: string;
	    assistantMessageId: string;
	    parentMessageId: string;
	    variantIndex: number;
	    attachments?: ChatAttachmentResult[];
	
	    static createFrom(source: any = {}) {
	        return new AskResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conversationId = source["conversationId"];
	        this.text = source["text"];
	        this.model = source["model"];
	        this.skill = source["skill"];
	        this.sources = source["sources"];
	        this.sourceKind = source["sourceKind"];
	        this.toolCalls = source["toolCalls"];
	        this.userMessageId = source["userMessageId"];
	        this.assistantMessageId = source["assistantMessageId"];
	        this.parentMessageId = source["parentMessageId"];
	        this.variantIndex = source["variantIndex"];
	        this.attachments = this.convertValues(source["attachments"], ChatAttachmentResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CapabilityApplyInput {
	    approved: boolean;
	    kind?: string;
	    name?: string;
	    existingCapability?: string;
	    packSource?: string;
	    packDir?: string;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityApplyInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.approved = source["approved"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.existingCapability = source["existingCapability"];
	        this.packSource = source["packSource"];
	        this.packDir = source["packDir"];
	    }
	}
	export class CapabilityApplyResult {
	    pack: domainpacks.PackStatus;
	    action: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pack = this.convertValues(source["pack"], domainpacks.PackStatus);
	        this.action = source["action"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CapabilityGenerateInput {
	    request: string;
	    approved: boolean;
	    name?: string;
	    allowedDomains?: string[];
	    runAfterGenerate?: boolean;
	    runInput?: Record<string, any>;
	    draft?: extensions.PackageDraft;
	    synthesizeDraft?: boolean;
	    draftRepairAttempts?: number;
	    scheduleAfterGenerate?: boolean;
	    scheduleName?: string;
	    scheduleType?: string;
	    scheduleExpr?: string;
	    scheduleEnabled?: boolean;
	    scheduleInput?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityGenerateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.request = source["request"];
	        this.approved = source["approved"];
	        this.name = source["name"];
	        this.allowedDomains = source["allowedDomains"];
	        this.runAfterGenerate = source["runAfterGenerate"];
	        this.runInput = source["runInput"];
	        this.draft = this.convertValues(source["draft"], extensions.PackageDraft);
	        this.synthesizeDraft = source["synthesizeDraft"];
	        this.draftRepairAttempts = source["draftRepairAttempts"];
	        this.scheduleAfterGenerate = source["scheduleAfterGenerate"];
	        this.scheduleName = source["scheduleName"];
	        this.scheduleType = source["scheduleType"];
	        this.scheduleExpr = source["scheduleExpr"];
	        this.scheduleEnabled = source["scheduleEnabled"];
	        this.scheduleInput = source["scheduleInput"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ChatAttachmentUploadInput {
	    fileName: string;
	    contentType: string;
	    dataBase64: string;
	    retention?: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatAttachmentUploadInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.contentType = source["contentType"];
	        this.dataBase64 = source["dataBase64"];
	        this.retention = source["retention"];
	    }
	}
	export class ChatAttachmentUploadResult {
	    attachment: ChatAttachmentResult;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatAttachmentUploadResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.attachment = this.convertValues(source["attachment"], ChatAttachmentResult);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ChatResult {
	    conversationId: string;
	    text: string;
	    model: string;
	    toolCalls: string[];
	
	    static createFrom(source: any = {}) {
	        return new ChatResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conversationId = source["conversationId"];
	        this.text = source["text"];
	        this.model = source["model"];
	        this.toolCalls = source["toolCalls"];
	    }
	}
	export class ConversationMessageResult {
	    id: string;
	    role: string;
	    content: string;
	    meta?: string;
	    model: string;
	    createdAt: string;
	    parentId: string;
	    variantIndex: number;
	    activeVariant: boolean;
	    trace?: string[];
	    sources?: string[];
	    sourceKind?: string;
	    attachments?: ChatAttachmentResult[];
	
	    static createFrom(source: any = {}) {
	        return new ConversationMessageResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.content = source["content"];
	        this.meta = source["meta"];
	        this.model = source["model"];
	        this.createdAt = source["createdAt"];
	        this.parentId = source["parentId"];
	        this.variantIndex = source["variantIndex"];
	        this.activeVariant = source["activeVariant"];
	        this.trace = source["trace"];
	        this.sources = source["sources"];
	        this.sourceKind = source["sourceKind"];
	        this.attachments = this.convertValues(source["attachments"], ChatAttachmentResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConversationResult {
	    id: string;
	    title: string;
	    createdAt: string;
	    updatedAt: string;
	    starred: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConversationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.starred = source["starred"];
	    }
	}
	export class ConversationDetailResult {
	    conversation: ConversationResult;
	    messages: ConversationMessageResult[];
	
	    static createFrom(source: any = {}) {
	        return new ConversationDetailResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conversation = this.convertValues(source["conversation"], ConversationResult);
	        this.messages = this.convertValues(source["messages"], ConversationMessageResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class DocumentInventoryResult {
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
	
	    static createFrom(source: any = {}) {
	        return new DocumentInventoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.workspaceRoot = source["workspaceRoot"];
	        this.managedSource = source["managedSource"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.missing = source["missing"];
	        this.stale = source["stale"];
	        this.sizeBytes = source["sizeBytes"];
	        this.chunkCount = source["chunkCount"];
	        this.embeddingCount = source["embeddingCount"];
	        this.indexedAt = source["indexedAt"];
	        this.updatedAt = source["updatedAt"];
	        this.contentHash = source["contentHash"];
	        this.currentContentHash = source["currentContentHash"];
	    }
	}
	export class DocumentPathSuggestionResult {
	    path: string;
	    name: string;
	    directory: string;
	    kind: string;
	    sizeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new DocumentPathSuggestionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.kind = source["kind"];
	        this.sizeBytes = source["sizeBytes"];
	    }
	}
	export class DocumentPruneResult {
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
	
	    static createFrom(source: any = {}) {
	        return new DocumentPruneResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dryRun = source["dryRun"];
	        this.documentsMatched = source["documentsMatched"];
	        this.chunksMatched = source["chunksMatched"];
	        this.ftsRowsMatched = source["ftsRowsMatched"];
	        this.embeddingsMatched = source["embeddingsMatched"];
	        this.documentsRemoved = source["documentsRemoved"];
	        this.chunksRemoved = source["chunksRemoved"];
	        this.ftsRowsRemoved = source["ftsRowsRemoved"];
	        this.embeddingsRemoved = source["embeddingsRemoved"];
	        this.documentIds = source["documentIds"];
	    }
	}
	export class DocumentResult {
	    chunkId: string;
	    path: string;
	    content: string;
	    rank: number;
	    score: number;
	    source: string;
	    explanation: string[];
	
	    static createFrom(source: any = {}) {
	        return new DocumentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chunkId = source["chunkId"];
	        this.path = source["path"];
	        this.content = source["content"];
	        this.rank = source["rank"];
	        this.score = source["score"];
	        this.source = source["source"];
	        this.explanation = source["explanation"];
	    }
	}
	export class WorkspaceGrantResult {
	    id: string;
	    path: string;
	    label: string;
	    source: string;
	    createdAt: string;
	    bookmarkReady: boolean;
	    bookmarkStale: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceGrantResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.label = source["label"];
	        this.source = source["source"];
	        this.createdAt = source["createdAt"];
	        this.bookmarkReady = source["bookmarkReady"];
	        this.bookmarkStale = source["bookmarkStale"];
	    }
	}
	export class DocumentSourceResult {
	    path: string;
	    grant: WorkspaceGrantResult;
	
	    static createFrom(source: any = {}) {
	        return new DocumentSourceResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.grant = this.convertValues(source["grant"], WorkspaceGrantResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DomainPackSkillStatus {
	    packName: string;
	    ref: string;
	    active: boolean;
	    enabled: boolean;
	    valid: boolean;
	    validationError?: string;
	    packDir: string;
	
	    static createFrom(source: any = {}) {
	        return new DomainPackSkillStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.packName = source["packName"];
	        this.ref = source["ref"];
	        this.active = source["active"];
	        this.enabled = source["enabled"];
	        this.valid = source["valid"];
	        this.validationError = source["validationError"];
	        this.packDir = source["packDir"];
	    }
	}
	export class EmbeddingIndexSummary {
	    enabled: boolean;
	    provider: string;
	    model: string;
	    chunksEmbedded: number;
	    chunksSkipped: number;
	    dimensions: number;
	
	    static createFrom(source: any = {}) {
	        return new EmbeddingIndexSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.chunksEmbedded = source["chunksEmbedded"];
	        this.chunksSkipped = source["chunksSkipped"];
	        this.dimensions = source["dimensions"];
	    }
	}
	export class ExtensionActionResult {
	    extension: extensions.Status;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extension = this.convertValues(source["extension"], extensions.Status);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExtensionGenerateInput {
	    name: string;
	    description: string;
	    approved: boolean;
	    brokeredNetwork: boolean;
	    allowedDomains?: string[];
	    draft?: extensions.PackageDraft;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionGenerateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.approved = source["approved"];
	        this.brokeredNetwork = source["brokeredNetwork"];
	        this.allowedDomains = source["allowedDomains"];
	        this.draft = this.convertValues(source["draft"], extensions.PackageDraft);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ExtensionProposalInput {
	    request: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionProposalInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.request = source["request"];
	    }
	}
	export class ExtensionRollbackInput {
	    nameOrSnapshot: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionRollbackInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nameOrSnapshot = source["nameOrSnapshot"];
	    }
	}
	export class ExtensionRunInput {
	    name: string;
	    input: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ExtensionRunInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.input = source["input"];
	    }
	}
	export class FileWriteApplyResult {
	    path: string;
	    diff: string;
	    snapshotId: string;
	    isNew: boolean;
	    changed: boolean;
	    targetBytes: number;
	    verification: workspace.WriteVerification;
	
	    static createFrom(source: any = {}) {
	        return new FileWriteApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.diff = source["diff"];
	        this.snapshotId = source["snapshotId"];
	        this.isNew = source["isNew"];
	        this.changed = source["changed"];
	        this.targetBytes = source["targetBytes"];
	        this.verification = this.convertValues(source["verification"], workspace.WriteVerification);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class FileWriteInput {
	    path: string;
	    content: string;
	    approved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new FileWriteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	        this.approved = source["approved"];
	    }
	}
	export class FileWritePlanResult {
	    path: string;
	    diff: string;
	    isNew: boolean;
	    changed: boolean;
	    targetBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new FileWritePlanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.diff = source["diff"];
	        this.isNew = source["isNew"];
	        this.changed = source["changed"];
	        this.targetBytes = source["targetBytes"];
	    }
	}
	export class GeneratedConnectorEnabledInput {
	    name: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GeneratedConnectorEnabledInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	    }
	}
	export class IngestSummary {
	    root: string;
	    filesIndexed: number;
	    filesSkipped: number;
	    filesUnchanged: number;
	    chunksCreated: number;
	    bytesIndexed: number;
	    skippedReasons: string[];
	
	    static createFrom(source: any = {}) {
	        return new IngestSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root = source["root"];
	        this.filesIndexed = source["filesIndexed"];
	        this.filesSkipped = source["filesSkipped"];
	        this.filesUnchanged = source["filesUnchanged"];
	        this.chunksCreated = source["chunksCreated"];
	        this.bytesIndexed = source["bytesIndexed"];
	        this.skippedReasons = source["skippedReasons"];
	    }
	}
	export class InternetCrawlInput {
	    seedUrl: string;
	    method: string;
	    extractText: boolean;
	    allowedDomains: string[];
	    maxPages: number;
	    maxDepth: number;
	    maxDurationSeconds: number;
	    maxLinksPerPage: number;
	    maxTextChars: number;
	    taskApproved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InternetCrawlInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seedUrl = source["seedUrl"];
	        this.method = source["method"];
	        this.extractText = source["extractText"];
	        this.allowedDomains = source["allowedDomains"];
	        this.maxPages = source["maxPages"];
	        this.maxDepth = source["maxDepth"];
	        this.maxDurationSeconds = source["maxDurationSeconds"];
	        this.maxLinksPerPage = source["maxLinksPerPage"];
	        this.maxTextChars = source["maxTextChars"];
	        this.taskApproved = source["taskApproved"];
	    }
	}
	export class InternetFetchInput {
	    url: string;
	    extractText: boolean;
	    allowedDomains: string[];
	    taskApproved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new InternetFetchInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.extractText = source["extractText"];
	        this.allowedDomains = source["allowedDomains"];
	        this.taskApproved = source["taskApproved"];
	    }
	}
	export class InternetSearchInput {
	    query: string;
	    taskApproved: boolean;
	    maxResults: number;
	
	    static createFrom(source: any = {}) {
	        return new InternetSearchInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.taskApproved = source["taskApproved"];
	        this.maxResults = source["maxResults"];
	    }
	}
	export class JobArchiveInput {
	    id: string;
	    approved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JobArchiveInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.approved = source["approved"];
	    }
	}
	export class JobCreateInput {
	    name: string;
	    scheduleType: string;
	    scheduleExpr: string;
	    targetType: string;
	    targetName: string;
	    input: Record<string, any>;
	    approved: boolean;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JobCreateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.scheduleType = source["scheduleType"];
	        this.scheduleExpr = source["scheduleExpr"];
	        this.targetType = source["targetType"];
	        this.targetName = source["targetName"];
	        this.input = source["input"];
	        this.approved = source["approved"];
	        this.enabled = source["enabled"];
	    }
	}
	export class JobEnabledInput {
	    id: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JobEnabledInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.enabled = source["enabled"];
	    }
	}
	export class JobInputUpdateInput {
	    id: string;
	    input: Record<string, any>;
	    approved: boolean;
	
	    static createFrom(source: any = {}) {
	        return new JobInputUpdateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.input = source["input"];
	        this.approved = source["approved"];
	    }
	}
	export class JobRunInput {
	    id: string;
	    conversationId?: string;
	
	    static createFrom(source: any = {}) {
	        return new JobRunInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	    }
	}
	export class KnowledgeEdgeInput {
	    fromEntityId: string;
	    relation: string;
	    toEntityId: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    evidence: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeEdgeInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fromEntityId = source["fromEntityId"];
	        this.relation = source["relation"];
	        this.toEntityId = source["toEntityId"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.evidence = source["evidence"];
	    }
	}
	export class KnowledgeEdgeResult {
	    id: string;
	    fromEntityId: string;
	    relation: string;
	    toEntityId: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    evidence: string;
	    reviewStatus: string;
	    reviewNote: string;
	    reviewedBy: string;
	    reviewedAt: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeEdgeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fromEntityId = source["fromEntityId"];
	        this.relation = source["relation"];
	        this.toEntityId = source["toEntityId"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.evidence = source["evidence"];
	        this.reviewStatus = source["reviewStatus"];
	        this.reviewNote = source["reviewNote"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewedAt = source["reviewedAt"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class KnowledgeEnabledInput {
	    enabled: boolean;
	    influenceEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeEnabledInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.influenceEnabled = source["influenceEnabled"];
	    }
	}
	export class KnowledgeEntityInput {
	    name: string;
	    kind: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    evidence: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeEntityInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.evidence = source["evidence"];
	    }
	}
	export class KnowledgeEntityProposalResult {
	    name: string;
	    kind: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    evidence: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeEntityProposalResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.evidence = source["evidence"];
	        this.reason = source["reason"];
	    }
	}
	export class KnowledgeEntityResult {
	    id: string;
	    name: string;
	    kind: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    evidence: string;
	    reviewStatus: string;
	    reviewNote: string;
	    reviewedBy: string;
	    reviewedAt: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeEntityResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.evidence = source["evidence"];
	        this.reviewStatus = source["reviewStatus"];
	        this.reviewNote = source["reviewNote"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewedAt = source["reviewedAt"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class KnowledgeProposalInput {
	    text: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    defaultKind: string;
	    limit: number;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeProposalInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.defaultKind = source["defaultKind"];
	        this.limit = source["limit"];
	    }
	}
	export class KnowledgeRelationProposalResult {
	    fromName: string;
	    relation: string;
	    toName: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    evidence: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeRelationProposalResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fromName = source["fromName"];
	        this.relation = source["relation"];
	        this.toName = source["toName"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.evidence = source["evidence"];
	        this.reason = source["reason"];
	    }
	}
	export class KnowledgeProposalResult {
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    reviewRequired: boolean;
	    message: string;
	    entityProposals: KnowledgeEntityProposalResult[];
	    relationProposals: KnowledgeRelationProposalResult[];
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeProposalResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.reviewRequired = source["reviewRequired"];
	        this.message = source["message"];
	        this.entityProposals = this.convertValues(source["entityProposals"], KnowledgeEntityProposalResult);
	        this.relationProposals = this.convertValues(source["relationProposals"], KnowledgeRelationProposalResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class KnowledgeStatus {
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
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.influenceEnabled = source["influenceEnabled"];
	        this.manualOnly = source["manualOnly"];
	        this.maxEntitiesPerQuery = source["maxEntitiesPerQuery"];
	        this.maxEvidenceChars = source["maxEvidenceChars"];
	        this.maxInfluenceEntities = source["maxInfluenceEntities"];
	        this.maxInfluenceChars = source["maxInfluenceChars"];
	        this.database = source["database"];
	        this.entityCount = source["entityCount"];
	        this.edgeCount = source["edgeCount"];
	        this.message = source["message"];
	    }
	}
	export class KnowledgeRepairResult {
	    entitiesScanned: number;
	    entitiesUpdated: number;
	    edgesScanned: number;
	    edgesUpdated: number;
	    status: KnowledgeStatus;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeRepairResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entitiesScanned = source["entitiesScanned"];
	        this.entitiesUpdated = source["entitiesUpdated"];
	        this.edgesScanned = source["edgesScanned"];
	        this.edgesUpdated = source["edgesUpdated"];
	        this.status = this.convertValues(source["status"], KnowledgeStatus);
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class KnowledgeReviewTargetInput {
	    type: string;
	    id: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeReviewTargetInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.id = source["id"];
	    }
	}
	export class KnowledgeReviewBatchInput {
	    action: string;
	    targets: KnowledgeReviewTargetInput[];
	    reviewedBy: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeReviewBatchInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.targets = this.convertValues(source["targets"], KnowledgeReviewTargetInput);
	        this.reviewedBy = source["reviewedBy"];
	        this.note = source["note"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class KnowledgeReviewItemResult {
	    id: string;
	    type: string;
	    name: string;
	    kind: string;
	    fromEntityId: string;
	    relation: string;
	    toEntityId: string;
	    source: string;
	    sourceKind: string;
	    sourceRef: string;
	    evidence: string;
	    reviewStatus: string;
	    reviewNote: string;
	    reviewedBy: string;
	    reviewedAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeReviewItemResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.fromEntityId = source["fromEntityId"];
	        this.relation = source["relation"];
	        this.toEntityId = source["toEntityId"];
	        this.source = source["source"];
	        this.sourceKind = source["sourceKind"];
	        this.sourceRef = source["sourceRef"];
	        this.evidence = source["evidence"];
	        this.reviewStatus = source["reviewStatus"];
	        this.reviewNote = source["reviewNote"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewedAt = source["reviewedAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class KnowledgeReviewBatchResult {
	    action: string;
	    requested: number;
	    entitiesMatched: number;
	    edgesMatched: number;
	    entitiesUpdated: number;
	    edgesUpdated: number;
	    items: KnowledgeReviewItemResult[];
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeReviewBatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.requested = source["requested"];
	        this.entitiesMatched = source["entitiesMatched"];
	        this.edgesMatched = source["edgesMatched"];
	        this.entitiesUpdated = source["entitiesUpdated"];
	        this.edgesUpdated = source["edgesUpdated"];
	        this.items = this.convertValues(source["items"], KnowledgeReviewItemResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class LearningCorrectionInput {
	    conversationId: string;
	    correction: string;
	
	    static createFrom(source: any = {}) {
	        return new LearningCorrectionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conversationId = source["conversationId"];
	        this.correction = source["correction"];
	    }
	}
	export class MemoryResult {
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
	
	    static createFrom(source: any = {}) {
	        return new MemoryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.messageId = source["messageId"];
	        this.conversationId = source["conversationId"];
	        this.role = source["role"];
	        this.kind = source["kind"];
	        this.content = source["content"];
	        this.snippet = source["snippet"];
	        this.importance = source["importance"];
	        this.source = source["source"];
	        this.pinned = source["pinned"];
	        this.explicit = source["explicit"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class MemoryWriteInput {
	    kind: string;
	    content: string;
	    importance: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryWriteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.content = source["content"];
	        this.importance = source["importance"];
	    }
	}
	export class ModelGenerateResult {
	    text: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelGenerateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.model = source["model"];
	    }
	}
	export class ModelProfileApplyInput {
	    name: string;
	    role: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelProfileApplyInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.role = source["role"];
	    }
	}
	export class ModelProfileDraftInput {
	    conversationId: string;
	    name?: string;
	    baseModel?: string;
	    temperature?: number;
	    numCtx?: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelProfileDraftInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.conversationId = source["conversationId"];
	        this.name = source["name"];
	        this.baseModel = source["baseModel"];
	        this.temperature = source["temperature"];
	        this.numCtx = source["numCtx"];
	    }
	}
	export class ModelProfileDraftReport {
	    schema: string;
	    conversationId: string;
	    eligibility: string;
	    reason: string;
	    messageCount: number;
	    assistantMessages: number;
	    toolRunCount: number;
	    failedToolRuns: number;
	    observedTraits?: string[];
	    privacyFilter: learning.PrivacyFilterReport;
	    safetySummary?: string[];
	    readiness: modelprofiles.ReadinessReport;
	
	    static createFrom(source: any = {}) {
	        return new ModelProfileDraftReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.conversationId = source["conversationId"];
	        this.eligibility = source["eligibility"];
	        this.reason = source["reason"];
	        this.messageCount = source["messageCount"];
	        this.assistantMessages = source["assistantMessages"];
	        this.toolRunCount = source["toolRunCount"];
	        this.failedToolRuns = source["failedToolRuns"];
	        this.observedTraits = source["observedTraits"];
	        this.privacyFilter = this.convertValues(source["privacyFilter"], learning.PrivacyFilterReport);
	        this.safetySummary = source["safetySummary"];
	        this.readiness = this.convertValues(source["readiness"], modelprofiles.ReadinessReport);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelProfileInput {
	    name: string;
	    description?: string;
	    baseModel: string;
	    system?: string;
	    temperature?: number;
	    numCtx?: number;
	    tags?: string[];
	    purpose?: string;
	    refinedFrom?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.baseModel = source["baseModel"];
	        this.system = source["system"];
	        this.temperature = source["temperature"];
	        this.numCtx = source["numCtx"];
	        this.tags = source["tags"];
	        this.purpose = source["purpose"];
	        this.refinedFrom = source["refinedFrom"];
	    }
	}
	export class ModelProfileResult {
	    profile: modelprofiles.Profile;
	    path?: string;
	    modelfile?: string;
	    message?: string;
	    applied?: boolean;
	    appliedRole?: string;
	    safetySummary?: string[];
	    draftReport?: ModelProfileDraftReport;
	    readiness: modelprofiles.ReadinessReport;
	    comparison: modelprofiles.ComparisonReport;
	    history?: modelprofiles.HistoryEvent[];
	
	    static createFrom(source: any = {}) {
	        return new ModelProfileResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], modelprofiles.Profile);
	        this.path = source["path"];
	        this.modelfile = source["modelfile"];
	        this.message = source["message"];
	        this.applied = source["applied"];
	        this.appliedRole = source["appliedRole"];
	        this.safetySummary = source["safetySummary"];
	        this.draftReport = this.convertValues(source["draftReport"], ModelProfileDraftReport);
	        this.readiness = this.convertValues(source["readiness"], modelprofiles.ReadinessReport);
	        this.comparison = this.convertValues(source["comparison"], modelprofiles.ComparisonReport);
	        this.history = this.convertValues(source["history"], modelprofiles.HistoryEvent);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelStatus {
	    role: string;
	    name: string;
	    installed: boolean;
	    profile?: string;
	    profileValid?: boolean;
	    profileStatus?: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.profile = source["profile"];
	        this.profileValid = source["profileValid"];
	        this.profileStatus = source["profileStatus"];
	    }
	}
	export class PermissionDecisionInput {
	    request: agent.PermissionRequest;
	    decision: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionDecisionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.request = this.convertValues(source["request"], agent.PermissionRequest);
	        this.decision = source["decision"];
	        this.note = source["note"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PermissionDecisionResult {
	    status: string;
	    toolName?: string;
	    toolStatus?: string;
	    executed?: boolean;
	    message?: string;
	    result?: agent.ExecutionResult;
	    conversationId?: string;
	    assistantMessageId?: string;
	    parentMessageId?: string;
	    variantIndex?: number;
	    activeVariant?: boolean;
	    createdAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionDecisionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.toolName = source["toolName"];
	        this.toolStatus = source["toolStatus"];
	        this.executed = source["executed"];
	        this.message = source["message"];
	        this.result = this.convertValues(source["result"], agent.ExecutionResult);
	        this.conversationId = source["conversationId"];
	        this.assistantMessageId = source["assistantMessageId"];
	        this.parentMessageId = source["parentMessageId"];
	        this.variantIndex = source["variantIndex"];
	        this.activeVariant = source["activeVariant"];
	        this.createdAt = source["createdAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PolicyStatusResult {
	    mode: string;
	    auditEnabled: boolean;
	    maxAutonomousLevel: number;
	    allowAutonomousPosting: boolean;
	    allowAutonomousDeployment: boolean;
	    allowLiveTrading: boolean;
	    generatedCanModifyCorePolicy: boolean;
	    fullAccess: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PolicyStatusResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.auditEnabled = source["auditEnabled"];
	        this.maxAutonomousLevel = source["maxAutonomousLevel"];
	        this.allowAutonomousPosting = source["allowAutonomousPosting"];
	        this.allowAutonomousDeployment = source["allowAutonomousDeployment"];
	        this.allowLiveTrading = source["allowLiveTrading"];
	        this.generatedCanModifyCorePolicy = source["generatedCanModifyCorePolicy"];
	        this.fullAccess = source["fullAccess"];
	    }
	}
	export class QARegressionSourceWriteApplyResult {
	    traceId: string;
	    record: learning.QARegressionSourceWriteRecord;
	    fileWriteApply: FileWriteApplyResult;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourceWriteApplyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.record = this.convertValues(source["record"], learning.QARegressionSourceWriteRecord);
	        this.fileWriteApply = this.convertValues(source["fileWriteApply"], FileWriteApplyResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionSourceWritePlanResult {
	    traceId: string;
	    applyPlanPath?: string;
	    suggestedTestFile: string;
	    suggestedTestName: string;
	    requiresApproval: boolean;
	    fileWritePlan: FileWritePlanResult;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourceWritePlanResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.applyPlanPath = source["applyPlanPath"];
	        this.suggestedTestFile = source["suggestedTestFile"];
	        this.suggestedTestName = source["suggestedTestName"];
	        this.requiresApproval = source["requiresApproval"];
	        this.fileWritePlan = this.convertValues(source["fileWritePlan"], FileWritePlanResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SettingsFeatureSupport {
	    responseMode: boolean;
	    showThinkingTrace: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsFeatureSupport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.responseMode = source["responseMode"];
	        this.showThinkingTrace = source["showThinkingTrace"];
	    }
	}
	export class SettingsInput {
	    lowMemoryMode: boolean;
	    maxContextTokens: number;
	    responseMode: string;
	    showThinkingTrace: boolean;
	    uiTheme: string;
	    ragEnabled: boolean;
	    embeddingsEnabled: boolean;
	    embeddingModel: string;
	    cloudFallbackEnabled: boolean;
	    cloudFallbackBaseUrl: string;
	    cloudFallbackModel: string;
	    cloudFallbackKeyEnv: string;
	    connectorEnabled: boolean;
	    connectorTokenEnv: string;
	    mcpConnectorEnabled: boolean;
	    slackConnectorEnabled: boolean;
	    slackConnectorTokenEnv: string;
	    discordConnectorEnabled: boolean;
	    discordConnectorTokenEnv: string;
	    telegramConnectorEnabled: boolean;
	    telegramConnectorTokenEnv: string;
	    emailConnectorEnabled: boolean;
	    emailConnectorTokenEnv: string;
	    internetEnabled: boolean;
	    internetSearchEnabled: boolean;
	    internetSearchProvider: string;
	    internetSearchEndpoint: string;
	    internetSearchApiKeyEnv: string;
	    knowledgeInfluenceEnabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lowMemoryMode = source["lowMemoryMode"];
	        this.maxContextTokens = source["maxContextTokens"];
	        this.responseMode = source["responseMode"];
	        this.showThinkingTrace = source["showThinkingTrace"];
	        this.uiTheme = source["uiTheme"];
	        this.ragEnabled = source["ragEnabled"];
	        this.embeddingsEnabled = source["embeddingsEnabled"];
	        this.embeddingModel = source["embeddingModel"];
	        this.cloudFallbackEnabled = source["cloudFallbackEnabled"];
	        this.cloudFallbackBaseUrl = source["cloudFallbackBaseUrl"];
	        this.cloudFallbackModel = source["cloudFallbackModel"];
	        this.cloudFallbackKeyEnv = source["cloudFallbackKeyEnv"];
	        this.connectorEnabled = source["connectorEnabled"];
	        this.connectorTokenEnv = source["connectorTokenEnv"];
	        this.mcpConnectorEnabled = source["mcpConnectorEnabled"];
	        this.slackConnectorEnabled = source["slackConnectorEnabled"];
	        this.slackConnectorTokenEnv = source["slackConnectorTokenEnv"];
	        this.discordConnectorEnabled = source["discordConnectorEnabled"];
	        this.discordConnectorTokenEnv = source["discordConnectorTokenEnv"];
	        this.telegramConnectorEnabled = source["telegramConnectorEnabled"];
	        this.telegramConnectorTokenEnv = source["telegramConnectorTokenEnv"];
	        this.emailConnectorEnabled = source["emailConnectorEnabled"];
	        this.emailConnectorTokenEnv = source["emailConnectorTokenEnv"];
	        this.internetEnabled = source["internetEnabled"];
	        this.internetSearchEnabled = source["internetSearchEnabled"];
	        this.internetSearchProvider = source["internetSearchProvider"];
	        this.internetSearchEndpoint = source["internetSearchEndpoint"];
	        this.internetSearchApiKeyEnv = source["internetSearchApiKeyEnv"];
	        this.knowledgeInfluenceEnabled = source["knowledgeInfluenceEnabled"];
	    }
	}
	export class SettingsView {
	    settingsSchemaVersion: number;
	    featureSupport: SettingsFeatureSupport;
	    lowMemoryMode: boolean;
	    maxContextTokens: number;
	    responseMode: string;
	    showThinkingTrace: boolean;
	    ui: config.UIConfig;
	    modelRoles: Record<string, config.ModelConfig>;
	    cloudFallback: config.CloudFallbackConfig;
	    connectors: config.ConnectorsConfig;
	    internet: config.InternetConfig;
	    scheduler: config.SchedulerConfig;
	    heartbeat: config.HeartbeatConfig;
	    rag: config.RAGConfig;
	    knowledge: config.KnowledgeGraphConfig;
	    workspace: config.WorkspaceConfig;
	    shellEnabled: boolean;
	    telemetry: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SettingsView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settingsSchemaVersion = source["settingsSchemaVersion"];
	        this.featureSupport = this.convertValues(source["featureSupport"], SettingsFeatureSupport);
	        this.lowMemoryMode = source["lowMemoryMode"];
	        this.maxContextTokens = source["maxContextTokens"];
	        this.responseMode = source["responseMode"];
	        this.showThinkingTrace = source["showThinkingTrace"];
	        this.ui = this.convertValues(source["ui"], config.UIConfig);
	        this.modelRoles = this.convertValues(source["modelRoles"], config.ModelConfig, true);
	        this.cloudFallback = this.convertValues(source["cloudFallback"], config.CloudFallbackConfig);
	        this.connectors = this.convertValues(source["connectors"], config.ConnectorsConfig);
	        this.internet = this.convertValues(source["internet"], config.InternetConfig);
	        this.scheduler = this.convertValues(source["scheduler"], config.SchedulerConfig);
	        this.heartbeat = this.convertValues(source["heartbeat"], config.HeartbeatConfig);
	        this.rag = this.convertValues(source["rag"], config.RAGConfig);
	        this.knowledge = this.convertValues(source["knowledge"], config.KnowledgeGraphConfig);
	        this.workspace = this.convertValues(source["workspace"], config.WorkspaceConfig);
	        this.shellEnabled = source["shellEnabled"];
	        this.telemetry = source["telemetry"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SetupInput {
	    mode: string;
	    lowMemoryModel: string;
	    defaultModel: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.lowMemoryModel = source["lowMemoryModel"];
	        this.defaultModel = source["defaultModel"];
	    }
	}
	export class SetupState {
	    setupComplete: boolean;
	    ollamaOk: boolean;
	    ollamaError: string;
	    configPath: string;
	    profilePath: string;
	    lowMemoryMode: boolean;
	    mode: string;
	    selectedModel: string;
	    modelReady: boolean;
	    installedModels: models.ModelInfo[];
	    recommendedMode: string;
	    recommendedModel: string;
	    configuredDefaults: ModelStatus[];
	
	    static createFrom(source: any = {}) {
	        return new SetupState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.setupComplete = source["setupComplete"];
	        this.ollamaOk = source["ollamaOk"];
	        this.ollamaError = source["ollamaError"];
	        this.configPath = source["configPath"];
	        this.profilePath = source["profilePath"];
	        this.lowMemoryMode = source["lowMemoryMode"];
	        this.mode = source["mode"];
	        this.selectedModel = source["selectedModel"];
	        this.modelReady = source["modelReady"];
	        this.installedModels = this.convertValues(source["installedModels"], models.ModelInfo);
	        this.recommendedMode = source["recommendedMode"];
	        this.recommendedModel = source["recommendedModel"];
	        this.configuredDefaults = this.convertValues(source["configuredDefaults"], ModelStatus);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillSummary {
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
	
	    static createFrom(source: any = {}) {
	        return new SkillSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.triggers = source["triggers"];
	        this.requiredTools = source["requiredTools"];
	        this.enabled = source["enabled"];
	        this.valid = source["valid"];
	        this.validationError = source["validationError"];
	        this.source = source["source"];
	        this.dir = source["dir"];
	    }
	}
	export class SkillActionResult {
	    skill: SkillSummary;
	    message: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillActionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.skill = this.convertValues(source["skill"], SkillSummary);
	        this.message = source["message"];
	        this.path = source["path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Status {
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
	    modelStatuses: ModelStatus[];
	    skillCount: number;
	    workspace: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configPath = source["configPath"];
	        this.profilePath = source["profilePath"];
	        this.sqlitePath = source["sqlitePath"];
	        this.sqliteOk = source["sqliteOk"];
	        this.sqliteError = source["sqliteError"];
	        this.ollamaOk = source["ollamaOk"];
	        this.ollamaError = source["ollamaError"];
	        this.defaultModel = source["defaultModel"];
	        this.lowMemoryModel = source["lowMemoryModel"];
	        this.selectedModel = source["selectedModel"];
	        this.lowMemoryMode = source["lowMemoryMode"];
	        this.setupComplete = source["setupComplete"];
	        this.modelReady = source["modelReady"];
	        this.ragEnabled = source["ragEnabled"];
	        this.shellEnabled = source["shellEnabled"];
	        this.telemetry = source["telemetry"];
	        this.modelStatuses = this.convertValues(source["modelStatuses"], ModelStatus);
	        this.skillCount = source["skillCount"];
	        this.workspace = source["workspace"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolCatalogEntryResult {
	    name: string;
	    status: string;
	    surfaces: string[];
	    chatCallable: boolean;
	    modelCallable: boolean;
	    requiresApproval: boolean;
	    enabledByDefault: boolean;
	    mutating: boolean;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolCatalogEntryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.surfaces = source["surfaces"];
	        this.chatCallable = source["chatCallable"];
	        this.modelCallable = source["modelCallable"];
	        this.requiresApproval = source["requiresApproval"];
	        this.enabledByDefault = source["enabledByDefault"];
	        this.mutating = source["mutating"];
	        this.notes = source["notes"];
	    }
	}
	export class ToolRunResult {
	    id: string;
	    conversationId: string;
	    sessionId: string;
	    userMessageId: string;
	    assistantMessageId: string;
	    parentMessageId: string;
	    variantIndex: number;
	    toolName: string;
	    input: any;
	    output: any;
	    status: string;
	    riskLevel: string;
	    createdAt: string;
	    completedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolRunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.sessionId = source["sessionId"];
	        this.userMessageId = source["userMessageId"];
	        this.assistantMessageId = source["assistantMessageId"];
	        this.parentMessageId = source["parentMessageId"];
	        this.variantIndex = source["variantIndex"];
	        this.toolName = source["toolName"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.status = source["status"];
	        this.riskLevel = source["riskLevel"];
	        this.createdAt = source["createdAt"];
	        this.completedAt = source["completedAt"];
	    }
	}

}

export namespace domainpacks {
	
	export class PolicyOverlay {
	    name: string;
	    mode?: string;
	    rules?: string[];
	    appliesTo?: string[];
	
	    static createFrom(source: any = {}) {
	        return new PolicyOverlay(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.mode = source["mode"];
	        this.rules = source["rules"];
	        this.appliesTo = source["appliesTo"];
	    }
	}
	export class Safety {
	    approvalRequiredFor?: string[];
	    blockedActions?: string[];
	    sensitiveDataRules?: string[];
	    policyOverlays?: PolicyOverlay[];
	    policyOverrides?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Safety(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.approvalRequiredFor = source["approvalRequiredFor"];
	        this.blockedActions = source["blockedActions"];
	        this.sensitiveDataRules = source["sensitiveDataRules"];
	        this.policyOverlays = this.convertValues(source["policyOverlays"], PolicyOverlay);
	        this.policyOverrides = source["policyOverrides"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PermissionDeclaration {
	    required?: boolean;
	    scopes?: string[];
	    allowedDomains?: string[];
	    allowedMethods?: string[];
	    paths?: string[];
	    names?: string[];
	    secretRefs?: string[];
	    notes?: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionDeclaration(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.required = source["required"];
	        this.scopes = source["scopes"];
	        this.allowedDomains = source["allowedDomains"];
	        this.allowedMethods = source["allowedMethods"];
	        this.paths = source["paths"];
	        this.names = source["names"];
	        this.secretRefs = source["secretRefs"];
	        this.notes = source["notes"];
	    }
	}
	export class Permissions {
	    internet: PermissionDeclaration;
	    filesystem: PermissionDeclaration;
	    scheduler: PermissionDeclaration;
	    notifications: PermissionDeclaration;
	    secrets: PermissionDeclaration;
	    connectors: PermissionDeclaration;
	
	    static createFrom(source: any = {}) {
	        return new Permissions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.internet = this.convertValues(source["internet"], PermissionDeclaration);
	        this.filesystem = this.convertValues(source["filesystem"], PermissionDeclaration);
	        this.scheduler = this.convertValues(source["scheduler"], PermissionDeclaration);
	        this.notifications = this.convertValues(source["notifications"], PermissionDeclaration);
	        this.secrets = this.convertValues(source["secrets"], PermissionDeclaration);
	        this.connectors = this.convertValues(source["connectors"], PermissionDeclaration);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Manifest {
	    name: string;
	    version: string;
	    description?: string;
	    category?: string;
	    skills?: string[];
	    requiredTools?: string[];
	    optionalTools?: string[];
	    permissions: Permissions;
	    safety: Safety;
	    tests?: string[];
	    defaultState: string;
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.skills = source["skills"];
	        this.requiredTools = source["requiredTools"];
	        this.optionalTools = source["optionalTools"];
	        this.permissions = this.convertValues(source["permissions"], Permissions);
	        this.safety = this.convertValues(source["safety"], Safety);
	        this.tests = source["tests"];
	        this.defaultState = source["defaultState"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PackStatus {
	    name: string;
	    version: string;
	    description?: string;
	    category?: string;
	    enabled: boolean;
	    installed: boolean;
	    valid: boolean;
	    validationError?: string;
	    repairHint?: string;
	    skills?: string[];
	    requiredTools?: string[];
	    optionalTools?: string[];
	    approvalRequiredFor?: string[];
	    blockedActions?: string[];
	    sensitiveDataRules?: string[];
	    safetySummary?: string;
	    dir: string;
	
	    static createFrom(source: any = {}) {
	        return new PackStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.enabled = source["enabled"];
	        this.installed = source["installed"];
	        this.valid = source["valid"];
	        this.validationError = source["validationError"];
	        this.repairHint = source["repairHint"];
	        this.skills = source["skills"];
	        this.requiredTools = source["requiredTools"];
	        this.optionalTools = source["optionalTools"];
	        this.approvalRequiredFor = source["approvalRequiredFor"];
	        this.blockedActions = source["blockedActions"];
	        this.sensitiveDataRules = source["sensitiveDataRules"];
	        this.safetySummary = source["safetySummary"];
	        this.dir = source["dir"];
	    }
	}
	export class PackSkillReview {
	    ref: string;
	    name?: string;
	    version?: string;
	    description?: string;
	    triggers?: string[];
	    requiredTools?: string[];
	    permissions?: skills.Permissions;
	    contextBudget?: skills.ContextBudget;
	    instructionChars?: number;
	    valid: boolean;
	    validationError?: string;
	    dir?: string;
	
	    static createFrom(source: any = {}) {
	        return new PackSkillReview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ref = source["ref"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.triggers = source["triggers"];
	        this.requiredTools = source["requiredTools"];
	        this.permissions = this.convertValues(source["permissions"], skills.Permissions);
	        this.contextBudget = this.convertValues(source["contextBudget"], skills.ContextBudget);
	        this.instructionChars = source["instructionChars"];
	        this.valid = source["valid"];
	        this.validationError = source["validationError"];
	        this.dir = source["dir"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class PackReview {
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
	    permissions?: Permissions;
	    safety?: Safety;
	    skills?: PackSkillReview[];
	    tests?: string[];
	    status?: PackStatus;
	    manifest?: Manifest;
	
	    static createFrom(source: any = {}) {
	        return new PackReview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.source = source["source"];
	        this.path = source["path"];
	        this.installed = source["installed"];
	        this.enabled = source["enabled"];
	        this.valid = source["valid"];
	        this.validationError = source["validationError"];
	        this.repairHint = source["repairHint"];
	        this.defaultState = source["defaultState"];
	        this.requiredTools = source["requiredTools"];
	        this.optionalTools = source["optionalTools"];
	        this.approvalRequiredFor = source["approvalRequiredFor"];
	        this.blockedActions = source["blockedActions"];
	        this.sensitiveDataRules = source["sensitiveDataRules"];
	        this.safetySummary = source["safetySummary"];
	        this.riskyToolReasons = source["riskyToolReasons"];
	        this.permissions = this.convertValues(source["permissions"], Permissions);
	        this.safety = this.convertValues(source["safety"], Safety);
	        this.skills = this.convertValues(source["skills"], PackSkillReview);
	        this.tests = source["tests"];
	        this.status = this.convertValues(source["status"], PackStatus);
	        this.manifest = this.convertValues(source["manifest"], Manifest);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	

}

export namespace extensions {
	
	export class AuditRecord {
	    action: string;
	    name: string;
	    status: string;
	    message?: string;
	    timestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.timestamp = source["timestamp"];
	    }
	}
	export class CoreResult {
	    id: string;
	    tool: string;
	    status: string;
	    output?: Record<string, any>;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CoreResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.tool = source["tool"];
	        this.status = source["status"];
	        this.output = source["output"];
	        this.error = source["error"];
	    }
	}
	export class PackageFile {
	    path: string;
	    size: number;
	    executable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PackageFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	        this.executable = source["executable"];
	    }
	}
	export class PackageInspection {
	    name: string;
	    dir: string;
	    status: string;
	    fileCount: number;
	    totalBytes: number;
	    entrypointPath?: string;
	    files: PackageFile[];
	    errors?: string[];
	    warnings?: string[];
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageInspection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.dir = source["dir"];
	        this.status = source["status"];
	        this.fileCount = source["fileCount"];
	        this.totalBytes = source["totalBytes"];
	        this.entrypointPath = source["entrypointPath"];
	        this.files = this.convertValues(source["files"], PackageFile);
	        this.errors = source["errors"];
	        this.warnings = source["warnings"];
	        this.checkedAt = source["checkedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Safety {
	    requiresUserApproval: boolean;
	    maxRuntimeSeconds: number;
	    maxResponseBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Safety(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requiresUserApproval = source["requiresUserApproval"];
	        this.maxRuntimeSeconds = source["maxRuntimeSeconds"];
	        this.maxResponseBytes = source["maxResponseBytes"];
	    }
	}
	export class Permission {
	    enabled: boolean;
	    scopes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Permission(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.scopes = source["scopes"];
	    }
	}
	export class FilesystemPermission {
	    read: boolean;
	    write: boolean;
	    paths?: string[];
	
	    static createFrom(source: any = {}) {
	        return new FilesystemPermission(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.read = source["read"];
	        this.write = source["write"];
	        this.paths = source["paths"];
	    }
	}
	export class NetworkPermission {
	    enabled: boolean;
	    allowedMethods: string[];
	    allowedDomains: string[];
	
	    static createFrom(source: any = {}) {
	        return new NetworkPermission(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.allowedMethods = source["allowedMethods"];
	        this.allowedDomains = source["allowedDomains"];
	    }
	}
	export class Permissions {
	    network: NetworkPermission;
	    filesystem: FilesystemPermission;
	    shell: boolean;
	    secrets: boolean;
	    memory: boolean;
	    extra?: Record<string, Permission>;
	
	    static createFrom(source: any = {}) {
	        return new Permissions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.network = this.convertValues(source["network"], NetworkPermission);
	        this.filesystem = this.convertValues(source["filesystem"], FilesystemPermission);
	        this.shell = source["shell"];
	        this.secrets = source["secrets"];
	        this.memory = source["memory"];
	        this.extra = this.convertValues(source["extra"], Permission, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Entrypoint {
	    type: string;
	    command: string;
	    args?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Entrypoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.command = source["command"];
	        this.args = source["args"];
	    }
	}
	export class Manifest {
	    name: string;
	    version: string;
	    description: string;
	    type: string;
	    entrypoint: Entrypoint;
	    inputSchema: Record<string, any>;
	    outputSchema: Record<string, any>;
	    permissions: Permissions;
	    safety: Safety;
	    tests: string[];
	    metadata?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.entrypoint = this.convertValues(source["entrypoint"], Entrypoint);
	        this.inputSchema = source["inputSchema"];
	        this.outputSchema = source["outputSchema"];
	        this.permissions = this.convertValues(source["permissions"], Permissions);
	        this.safety = this.convertValues(source["safety"], Safety);
	        this.tests = source["tests"];
	        this.metadata = source["metadata"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Status {
	    name: string;
	    version: string;
	    type: string;
	    description: string;
	    dir: string;
	    manifestPath: string;
	    enabled: boolean;
	    registered: boolean;
	    valid: boolean;
	    runnable: boolean;
	    callable: boolean;
	    runBlockedReason: string;
	    validationError: string;
	    registeredAt?: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.type = source["type"];
	        this.description = source["description"];
	        this.dir = source["dir"];
	        this.manifestPath = source["manifestPath"];
	        this.enabled = source["enabled"];
	        this.registered = source["registered"];
	        this.valid = source["valid"];
	        this.runnable = source["runnable"];
	        this.callable = source["callable"];
	        this.runBlockedReason = source["runBlockedReason"];
	        this.validationError = source["validationError"];
	        this.registeredAt = source["registeredAt"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class Detail {
	    status: Status;
	    manifest: Manifest;
	    inspection: PackageInspection;
	
	    static createFrom(source: any = {}) {
	        return new Detail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = this.convertValues(source["status"], Status);
	        this.manifest = this.convertValues(source["manifest"], Manifest);
	        this.inspection = this.convertValues(source["inspection"], PackageInspection);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DraftFile {
	    path: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new DraftFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.content = source["content"];
	    }
	}
	
	export class FailureTrend {
	    name: string;
	    failures: number;
	    lastStatus: string;
	    lastError?: string;
	    lastSeenAt: string;
	    suggestReview: boolean;
	    sources: string[];
	
	    static createFrom(source: any = {}) {
	        return new FailureTrend(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.failures = source["failures"];
	        this.lastStatus = source["lastStatus"];
	        this.lastError = source["lastError"];
	        this.lastSeenAt = source["lastSeenAt"];
	        this.suggestReview = source["suggestReview"];
	        this.sources = source["sources"];
	    }
	}
	
	export class TestResult {
	    name: string;
	    commands: string[];
	    status: string;
	    output?: string;
	    durationMs: number;
	    startedAt: string;
	    completedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.commands = source["commands"];
	        this.status = source["status"];
	        this.output = source["output"];
	        this.durationMs = source["durationMs"];
	        this.startedAt = source["startedAt"];
	        this.completedAt = source["completedAt"];
	    }
	}
	export class Snapshot {
	    id: string;
	    name: string;
	    targetDir: string;
	    existed: boolean;
	    backupDir?: string;
	    files: string[];
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.targetDir = source["targetDir"];
	        this.existed = source["existed"];
	        this.backupDir = source["backupDir"];
	        this.files = source["files"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class GenerationResult {
	    extension: Status;
	    snapshot: Snapshot;
	    tests: TestResult;
	    skillCandidate: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new GenerationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.extension = this.convertValues(source["extension"], Status);
	        this.snapshot = this.convertValues(source["snapshot"], Snapshot);
	        this.tests = this.convertValues(source["tests"], TestResult);
	        this.skillCandidate = source["skillCandidate"];
	        this.message = source["message"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class PackageDraft {
	    files: DraftFile[];
	    origin?: string;
	
	    static createFrom(source: any = {}) {
	        return new PackageDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], DraftFile);
	        this.origin = source["origin"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	export class Proposal {
	    name: string;
	    description: string;
	    type: string;
	    reason: string;
	    requiresApproval: boolean;
	    files: string[];
	    permissions: string[];
	    networkMode?: string;
	    allowedDomains?: string[];
	    allowedMethods?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Proposal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.type = source["type"];
	        this.reason = source["reason"];
	        this.requiresApproval = source["requiresApproval"];
	        this.files = source["files"];
	        this.permissions = source["permissions"];
	        this.networkMode = source["networkMode"];
	        this.allowedDomains = source["allowedDomains"];
	        this.allowedMethods = source["allowedMethods"];
	    }
	}
	export class ReviewFile {
	    path: string;
	    size: number;
	    executable: boolean;
	    content?: string;
	    truncated?: boolean;
	    binary?: boolean;
	    omittedReason?: string;
	
	    static createFrom(source: any = {}) {
	        return new ReviewFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.size = source["size"];
	        this.executable = source["executable"];
	        this.content = source["content"];
	        this.truncated = source["truncated"];
	        this.binary = source["binary"];
	        this.omittedReason = source["omittedReason"];
	    }
	}
	export class Review {
	    detail: Detail;
	    files: ReviewFile[];
	    suggestedActions: string[];
	    reuseCommand: string;
	    sampleInput?: Record<string, any>;
	    sampleInputJson?: string;
	    canRun: boolean;
	    canRegister: boolean;
	    runBlockedReason?: string;
	    registerBlockedReason?: string;
	
	    static createFrom(source: any = {}) {
	        return new Review(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.detail = this.convertValues(source["detail"], Detail);
	        this.files = this.convertValues(source["files"], ReviewFile);
	        this.suggestedActions = source["suggestedActions"];
	        this.reuseCommand = source["reuseCommand"];
	        this.sampleInput = source["sampleInput"];
	        this.sampleInputJson = source["sampleInputJson"];
	        this.canRun = source["canRun"];
	        this.canRegister = source["canRegister"];
	        this.runBlockedReason = source["runBlockedReason"];
	        this.registerBlockedReason = source["registerBlockedReason"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RollbackResult {
	    name: string;
	    snapshotId: string;
	    targetDir: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RollbackResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.snapshotId = source["snapshotId"];
	        this.targetDir = source["targetDir"];
	        this.message = source["message"];
	    }
	}
	export class RunResult {
	    runId: string;
	    extension: string;
	    version: string;
	    status: string;
	    output?: Record<string, any>;
	    coreResults?: CoreResult[];
	    error?: string;
	    logs?: string;
	    exitCode: number;
	    timedOut: boolean;
	    truncated: boolean;
	    durationMs: number;
	    startedAt: string;
	    completedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new RunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.extension = source["extension"];
	        this.version = source["version"];
	        this.status = source["status"];
	        this.output = source["output"];
	        this.coreResults = this.convertValues(source["coreResults"], CoreResult);
	        this.error = source["error"];
	        this.logs = source["logs"];
	        this.exitCode = source["exitCode"];
	        this.timedOut = source["timedOut"];
	        this.truncated = source["truncated"];
	        this.durationMs = source["durationMs"];
	        this.startedAt = source["startedAt"];
	        this.completedAt = source["completedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	

}

export namespace heartbeat {
	
	export class Check {
	    id: string;
	    name: string;
	    status: string;
	    detail: string;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Check(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	        this.checkedAt = source["checkedAt"];
	    }
	}
	export class Report {
	    overall: string;
	    generatedAt: string;
	    checks: Check[];
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overall = source["overall"];
	        this.generatedAt = source["generatedAt"];
	        this.checks = this.convertValues(source["checks"], Check);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace internet {
	
	export class CacheSummary {
	    key: string;
	    url: string;
	    method: string;
	    bodyBytes: number;
	    cachedAt: string;
	    expiresAt: string;
	    expired: boolean;
	    state: string;
	    ageSeconds: number;
	    expiresInSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new CacheSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.url = source["url"];
	        this.method = source["method"];
	        this.bodyBytes = source["bodyBytes"];
	        this.cachedAt = source["cachedAt"];
	        this.expiresAt = source["expiresAt"];
	        this.expired = source["expired"];
	        this.state = source["state"];
	        this.ageSeconds = source["ageSeconds"];
	        this.expiresInSeconds = source["expiresInSeconds"];
	    }
	}
	export class RobotsResult {
	    url: string;
	    robotsUrl: string;
	    userAgent: string;
	    allowed: boolean;
	    statusCode: number;
	    reason: string;
	    checkedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new RobotsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.robotsUrl = source["robotsUrl"];
	        this.userAgent = source["userAgent"];
	        this.allowed = source["allowed"];
	        this.statusCode = source["statusCode"];
	        this.reason = source["reason"];
	        this.checkedAt = source["checkedAt"];
	    }
	}
	export class CrawlPage {
	    url: string;
	    finalUrl: string;
	    depth: number;
	    statusCode: number;
	    contentType?: string;
	    bodyBytes: number;
	    fromCache: boolean;
	    extractedText?: string;
	    links?: string[];
	    robots?: RobotsResult;
	    fetchedAt: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new CrawlPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.finalUrl = source["finalUrl"];
	        this.depth = source["depth"];
	        this.statusCode = source["statusCode"];
	        this.contentType = source["contentType"];
	        this.bodyBytes = source["bodyBytes"];
	        this.fromCache = source["fromCache"];
	        this.extractedText = source["extractedText"];
	        this.links = source["links"];
	        this.robots = this.convertValues(source["robots"], RobotsResult);
	        this.fetchedAt = source["fetchedAt"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CrawlSkip {
	    url: string;
	    depth: number;
	    reason: string;
	    robots?: RobotsResult;
	
	    static createFrom(source: any = {}) {
	        return new CrawlSkip(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.depth = source["depth"];
	        this.reason = source["reason"];
	        this.robots = this.convertValues(source["robots"], RobotsResult);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CrawlResult {
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
	    pages: CrawlPage[];
	    skips: CrawlSkip[];
	
	    static createFrom(source: any = {}) {
	        return new CrawlResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.seedUrl = source["seedUrl"];
	        this.method = source["method"];
	        this.status = source["status"];
	        this.failureReason = source["failureReason"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.allowedDomains = source["allowedDomains"];
	        this.maxPages = source["maxPages"];
	        this.maxDepth = source["maxDepth"];
	        this.maxDurationSeconds = source["maxDurationSeconds"];
	        this.maxLinksPerPage = source["maxLinksPerPage"];
	        this.maxTextChars = source["maxTextChars"];
	        this.visited = source["visited"];
	        this.fetched = source["fetched"];
	        this.skipped = source["skipped"];
	        this.skipOverflow = source["skipOverflow"];
	        this.pages = this.convertValues(source["pages"], CrawlPage);
	        this.skips = this.convertValues(source["skips"], CrawlSkip);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CrawlRunPageSummary {
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
	
	    static createFrom(source: any = {}) {
	        return new CrawlRunPageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.finalUrl = source["finalUrl"];
	        this.depth = source["depth"];
	        this.statusCode = source["statusCode"];
	        this.contentType = source["contentType"];
	        this.bodyBytes = source["bodyBytes"];
	        this.fromCache = source["fromCache"];
	        this.linkCount = source["linkCount"];
	        this.textBytes = source["textBytes"];
	        this.robotsAllowed = source["robotsAllowed"];
	        this.fetchedAt = source["fetchedAt"];
	        this.error = source["error"];
	    }
	}
	export class CrawlRunRecord {
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
	    pages?: CrawlRunPageSummary[];
	    skips?: CrawlSkip[];
	
	    static createFrom(source: any = {}) {
	        return new CrawlRunRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.seedUrl = source["seedUrl"];
	        this.method = source["method"];
	        this.status = source["status"];
	        this.failureReason = source["failureReason"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.allowedDomains = source["allowedDomains"];
	        this.maxPages = source["maxPages"];
	        this.maxDepth = source["maxDepth"];
	        this.maxDurationSeconds = source["maxDurationSeconds"];
	        this.maxLinksPerPage = source["maxLinksPerPage"];
	        this.maxTextChars = source["maxTextChars"];
	        this.visited = source["visited"];
	        this.fetched = source["fetched"];
	        this.skipped = source["skipped"];
	        this.skipOverflow = source["skipOverflow"];
	        this.pages = this.convertValues(source["pages"], CrawlRunPageSummary);
	        this.skips = this.convertValues(source["skips"], CrawlSkip);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class FetchResult {
	    url: string;
	    finalUrl: string;
	    method: string;
	    statusCode: number;
	    header?: Record<string, Array<string>>;
	    contentType?: string;
	    body?: string;
	    extractedText?: string;
	    bodyBytes: number;
	    fromCache: boolean;
	    cachedAt?: string;
	    fetchedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new FetchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.finalUrl = source["finalUrl"];
	        this.method = source["method"];
	        this.statusCode = source["statusCode"];
	        this.header = source["header"];
	        this.contentType = source["contentType"];
	        this.body = source["body"];
	        this.extractedText = source["extractedText"];
	        this.bodyBytes = source["bodyBytes"];
	        this.fromCache = source["fromCache"];
	        this.cachedAt = source["cachedAt"];
	        this.fetchedAt = source["fetchedAt"];
	    }
	}
	export class RequestRecord {
	    timestamp: string;
	    caller: string;
	    method: string;
	    url: string;
	    finalUrl?: string;
	    host: string;
	    provider?: string;
	    statusCode?: number;
	    bytes?: number;
	    fromCache: boolean;
	    allowed: boolean;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new RequestRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.caller = source["caller"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.finalUrl = source["finalUrl"];
	        this.host = source["host"];
	        this.provider = source["provider"];
	        this.statusCode = source["statusCode"];
	        this.bytes = source["bytes"];
	        this.fromCache = source["fromCache"];
	        this.allowed = source["allowed"];
	        this.error = source["error"];
	    }
	}
	
	export class SearchItem {
	    title: string;
	    url: string;
	    snippet?: string;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.url = source["url"];
	        this.snippet = source["snippet"];
	        this.source = source["source"];
	    }
	}
	export class SearchProviderReadiness {
	    provider: string;
	    label: string;
	    configured: boolean;
	    ready: boolean;
	    needsConfig: boolean;
	    needsAuth: boolean;
	    networkChecked: boolean;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchProviderReadiness(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.label = source["label"];
	        this.configured = source["configured"];
	        this.ready = source["ready"];
	        this.needsConfig = source["needsConfig"];
	        this.needsAuth = source["needsAuth"];
	        this.networkChecked = source["networkChecked"];
	        this.status = source["status"];
	    }
	}
	export class SearchResult {
	    query: string;
	    provider: string;
	    url?: string;
	    items: SearchItem[];
	    message?: string;
	    fromCache: boolean;
	    cachedAt?: string;
	    fetchedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new SearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.provider = source["provider"];
	        this.url = source["url"];
	        this.items = this.convertValues(source["items"], SearchItem);
	        this.message = source["message"];
	        this.fromCache = source["fromCache"];
	        this.cachedAt = source["cachedAt"];
	        this.fetchedAt = source["fetchedAt"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Status {
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
	    searchMaxResults: number;
	    searchSafeSearch: boolean;
	    searchProviderReady: boolean;
	    searchProviderConfigured: boolean;
	    searchProviderNeedsConfig: boolean;
	    searchProviderNeedsAuth: boolean;
	    searchNetworkChecked: boolean;
	    searchStatus: string;
	    searchFallbackProviders?: string[];
	    searchFallbackReadiness?: SearchProviderReadiness[];
	    searchState: string;
	    searchHealthScore: number;
	    searchNextAction?: string;
	    cacheFreshEntries: number;
	    cacheStaleEntries: number;
	    cacheLatestCachedAt?: string;
	    logPath: string;
	    cacheDir: string;
	    robotsAware: boolean;
	    crawlerAvailable: boolean;
	    crawlerManualOnly: boolean;
	    crawlerRequiresApproval: boolean;
	    crawlerMaxPages: number;
	    crawlerMaxDepth: number;
	    crawlerMaxDurationSeconds: number;
	    crawlerMaxLinksPerPage: number;
	    crawlerStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.defaultMode = source["defaultMode"];
	        this.allowMethods = source["allowMethods"];
	        this.maxResponseBytes = source["maxResponseBytes"];
	        this.timeoutSeconds = source["timeoutSeconds"];
	        this.redirectLimit = source["redirectLimit"];
	        this.cacheEnabled = source["cacheEnabled"];
	        this.cacheTtlSeconds = source["cacheTtlSeconds"];
	        this.searchEnabled = source["searchEnabled"];
	        this.searchProvider = source["searchProvider"];
	        this.searchProviderLabel = source["searchProviderLabel"];
	        this.searchEndpoint = source["searchEndpoint"];
	        this.searchApiKeyEnv = source["searchApiKeyEnv"];
	        this.searchMaxResults = source["searchMaxResults"];
	        this.searchSafeSearch = source["searchSafeSearch"];
	        this.searchProviderReady = source["searchProviderReady"];
	        this.searchProviderConfigured = source["searchProviderConfigured"];
	        this.searchProviderNeedsConfig = source["searchProviderNeedsConfig"];
	        this.searchProviderNeedsAuth = source["searchProviderNeedsAuth"];
	        this.searchNetworkChecked = source["searchNetworkChecked"];
	        this.searchStatus = source["searchStatus"];
	        this.searchFallbackProviders = source["searchFallbackProviders"];
	        this.searchFallbackReadiness = this.convertValues(source["searchFallbackReadiness"], SearchProviderReadiness);
	        this.searchState = source["searchState"];
	        this.searchHealthScore = source["searchHealthScore"];
	        this.searchNextAction = source["searchNextAction"];
	        this.cacheFreshEntries = source["cacheFreshEntries"];
	        this.cacheStaleEntries = source["cacheStaleEntries"];
	        this.cacheLatestCachedAt = source["cacheLatestCachedAt"];
	        this.logPath = source["logPath"];
	        this.cacheDir = source["cacheDir"];
	        this.robotsAware = source["robotsAware"];
	        this.crawlerAvailable = source["crawlerAvailable"];
	        this.crawlerManualOnly = source["crawlerManualOnly"];
	        this.crawlerRequiresApproval = source["crawlerRequiresApproval"];
	        this.crawlerMaxPages = source["crawlerMaxPages"];
	        this.crawlerMaxDepth = source["crawlerMaxDepth"];
	        this.crawlerMaxDurationSeconds = source["crawlerMaxDurationSeconds"];
	        this.crawlerMaxLinksPerPage = source["crawlerMaxLinksPerPage"];
	        this.crawlerStatus = source["crawlerStatus"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace learning {
	
	export class LearningMemory {
	    id: string;
	    kind: string;
	    source: string;
	    snippet: string;
	    importance: number;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new LearningMemory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	        this.snippet = source["snippet"];
	        this.importance = source["importance"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class PrivacyFilterReport {
	    secretsRedacted: boolean;
	    pathsRedacted: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrivacyFilterReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.secretsRedacted = source["secretsRedacted"];
	        this.pathsRedacted = source["pathsRedacted"];
	    }
	}
	export class QAEvalTaskSummary {
	    name: string;
	    status: string;
	    details?: string;
	
	    static createFrom(source: any = {}) {
	        return new QAEvalTaskSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.details = source["details"];
	    }
	}
	export class QAEvalSummary {
	    id?: string;
	    mode?: string;
	    model?: string;
	    startedAt?: string;
	    completedAt?: string;
	    passed: number;
	    failed: number;
	    skipped: number;
	    status: string;
	    metrics?: Record<string, any>;
	    tasks?: QAEvalTaskSummary[];
	
	    static createFrom(source: any = {}) {
	        return new QAEvalSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.mode = source["mode"];
	        this.model = source["model"];
	        this.startedAt = source["startedAt"];
	        this.completedAt = source["completedAt"];
	        this.passed = source["passed"];
	        this.failed = source["failed"];
	        this.skipped = source["skipped"];
	        this.status = source["status"];
	        this.metrics = source["metrics"];
	        this.tasks = this.convertValues(source["tasks"], QAEvalTaskSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class QARegressionApprovalRequest {
	    traceId: string;
	    suggestionName?: string;
	    draftName?: string;
	    reviewedBy?: string;
	    reviewNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionApprovalRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.draftName = source["draftName"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	    }
	}
	export class RouteRegressionPromotion {
	    suggestedTestName?: string;
	    routeUnderTest?: string;
	    observedRoute?: string;
	    expectedRoute?: string;
	    expectedTaskType?: string;
	    expectedRiskLevel?: string;
	    continuationMode?: string;
	    expectedSource?: string;
	    expectedToolLane?: string;
	    requiredTools?: string[];
	    forbiddenTools?: string[];
	    expectedOutcome?: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteRegressionPromotion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.suggestedTestName = source["suggestedTestName"];
	        this.routeUnderTest = source["routeUnderTest"];
	        this.observedRoute = source["observedRoute"];
	        this.expectedRoute = source["expectedRoute"];
	        this.expectedTaskType = source["expectedTaskType"];
	        this.expectedRiskLevel = source["expectedRiskLevel"];
	        this.continuationMode = source["continuationMode"];
	        this.expectedSource = source["expectedSource"];
	        this.expectedToolLane = source["expectedToolLane"];
	        this.requiredTools = source["requiredTools"];
	        this.forbiddenTools = source["forbiddenTools"];
	        this.expectedOutcome = source["expectedOutcome"];
	    }
	}
	export class QARegressionApprovedRecord {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionApprovedRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.category = source["category"];
	        this.fingerprint = source["fingerprint"];
	        this.status = source["status"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	        this.approvedAt = source["approvedAt"];
	        this.path = source["path"];
	        this.testDraftPath = source["testDraftPath"];
	        this.suggestedTestFile = source["suggestedTestFile"];
	        this.suggestedTestName = source["suggestedTestName"];
	        this.automaticTestWritten = source["automaticTestWritten"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionPromotionArtifact {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionPromotionArtifact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.traceId = source["traceId"];
	        this.category = source["category"];
	        this.fingerprint = source["fingerprint"];
	        this.path = source["path"];
	        this.preview = source["preview"];
	        this.promotionMode = source["promotionMode"];
	        this.automaticTestWritten = source["automaticTestWritten"];
	        this.coveredByEvalOrTest = source["coveredByEvalOrTest"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionPromotionRequest {
	    traceId: string;
	    suggestionName?: string;
	    reviewedBy?: string;
	    reviewNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionPromotionRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	    }
	}
	export class QARegressionReviewRecord {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionReviewRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.category = source["category"];
	        this.fingerprint = source["fingerprint"];
	        this.status = source["status"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	        this.reviewedAt = source["reviewedAt"];
	        this.coveredByEvalOrTest = source["coveredByEvalOrTest"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	        this.path = source["path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionReviewRequest {
	    traceId: string;
	    suggestionName?: string;
	    status: string;
	    reviewedBy?: string;
	    reviewNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionReviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.status = source["status"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	    }
	}
	export class QARegressionSourcePatchApplyPlanRecord {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourcePatchApplyPlanRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.category = source["category"];
	        this.fingerprint = source["fingerprint"];
	        this.status = source["status"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	        this.approvedAt = source["approvedAt"];
	        this.path = source["path"];
	        this.sourcePatchPath = source["sourcePatchPath"];
	        this.approvedRegressionPath = source["approvedRegressionPath"];
	        this.testDraftPath = source["testDraftPath"];
	        this.suggestedTestFile = source["suggestedTestFile"];
	        this.suggestedTestName = source["suggestedTestName"];
	        this.automaticTestWritten = source["automaticTestWritten"];
	        this.sourceTestWritten = source["sourceTestWritten"];
	        this.requiresManualApply = source["requiresManualApply"];
	        this.patchPreview = source["patchPreview"];
	        this.manualApplyChecklist = source["manualApplyChecklist"];
	        this.preview = source["preview"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionSourcePatchApplyPlanRequest {
	    traceId: string;
	    suggestionName?: string;
	    draftName?: string;
	    sourcePatchName?: string;
	    approved: boolean;
	    reviewedBy?: string;
	    reviewNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourcePatchApplyPlanRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.draftName = source["draftName"];
	        this.sourcePatchName = source["sourcePatchName"];
	        this.approved = source["approved"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	    }
	}
	export class QARegressionSourcePatchRecord {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourcePatchRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.category = source["category"];
	        this.fingerprint = source["fingerprint"];
	        this.status = source["status"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	        this.draftedAt = source["draftedAt"];
	        this.path = source["path"];
	        this.approvedRegressionPath = source["approvedRegressionPath"];
	        this.testDraftPath = source["testDraftPath"];
	        this.suggestedTestFile = source["suggestedTestFile"];
	        this.suggestedTestName = source["suggestedTestName"];
	        this.automaticTestWritten = source["automaticTestWritten"];
	        this.requiresManualApply = source["requiresManualApply"];
	        this.testSnippet = source["testSnippet"];
	        this.patchPreview = source["patchPreview"];
	        this.preview = source["preview"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionSourcePatchRequest {
	    traceId: string;
	    suggestionName?: string;
	    draftName?: string;
	    reviewedBy?: string;
	    reviewNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourcePatchRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.draftName = source["draftName"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	    }
	}
	export class QARegressionSourceWriteRecord {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourceWriteRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.category = source["category"];
	        this.fingerprint = source["fingerprint"];
	        this.status = source["status"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	        this.writtenAt = source["writtenAt"];
	        this.path = source["path"];
	        this.applyPlanPath = source["applyPlanPath"];
	        this.sourcePatchPath = source["sourcePatchPath"];
	        this.suggestedTestFile = source["suggestedTestFile"];
	        this.suggestedTestName = source["suggestedTestName"];
	        this.automaticTestWritten = source["automaticTestWritten"];
	        this.sourceTestWritten = source["sourceTestWritten"];
	        this.requiresManualApply = source["requiresManualApply"];
	        this.snapshotId = source["snapshotId"];
	        this.verificationStatus = source["verificationStatus"];
	        this.changed = source["changed"];
	        this.targetBytes = source["targetBytes"];
	        this.diff = source["diff"];
	        this.preview = source["preview"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionSourceWriteRequest {
	    traceId: string;
	    suggestionName?: string;
	    sourcePatchName?: string;
	    content: string;
	    approved: boolean;
	    reviewedBy?: string;
	    reviewNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSourceWriteRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.sourcePatchName = source["sourcePatchName"];
	        this.content = source["content"];
	        this.approved = source["approved"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	    }
	}
	export class QARegressionSuggestion {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.prompt = source["prompt"];
	        this.sourceTraceId = source["sourceTraceId"];
	        this.promotionFingerprint = source["promotionFingerprint"];
	        this.repeatCount = source["repeatCount"];
	        this.promotionPriority = source["promotionPriority"];
	        this.coverageStatus = source["coverageStatus"];
	        this.coveredByTest = source["coveredByTest"];
	        this.coverageSources = source["coverageSources"];
	        this.reviewStatus = source["reviewStatus"];
	        this.reviewNote = source["reviewNote"];
	        this.reviewedAt = source["reviewedAt"];
	        this.testDraftStatus = source["testDraftStatus"];
	        this.testDraftPath = source["testDraftPath"];
	        this.approvedRegression = source["approvedRegression"];
	        this.approvedRegressionAt = source["approvedRegressionAt"];
	        this.approvedRegressionPath = source["approvedRegressionPath"];
	        this.sourcePatchStatus = source["sourcePatchStatus"];
	        this.sourcePatchPath = source["sourcePatchPath"];
	        this.sourcePatchRequiresManualApply = source["sourcePatchRequiresManualApply"];
	        this.sourcePatchApplyStatus = source["sourcePatchApplyStatus"];
	        this.sourcePatchApplyPath = source["sourcePatchApplyPath"];
	        this.sourceTestWriteStatus = source["sourceTestWriteStatus"];
	        this.sourceTestWritePath = source["sourceTestWritePath"];
	        this.sourceTestWriteSnapshotId = source["sourceTestWriteSnapshotId"];
	        this.tags = source["tags"];
	        this.preview = source["preview"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionTestDraftRecord {
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
	
	    static createFrom(source: any = {}) {
	        return new QARegressionTestDraftRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.category = source["category"];
	        this.fingerprint = source["fingerprint"];
	        this.status = source["status"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	        this.draftedAt = source["draftedAt"];
	        this.path = source["path"];
	        this.preview = source["preview"];
	        this.suggestedTestFile = source["suggestedTestFile"];
	        this.suggestedTestName = source["suggestedTestName"];
	        this.automaticTestWritten = source["automaticTestWritten"];
	        this.promotion = this.convertValues(source["promotion"], RouteRegressionPromotion);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QARegressionTestDraftRequest {
	    traceId: string;
	    suggestionName?: string;
	    reviewedBy?: string;
	    reviewNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new QARegressionTestDraftRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.suggestionName = source["suggestionName"];
	        this.reviewedBy = source["reviewedBy"];
	        this.reviewNote = source["reviewNote"];
	    }
	}
	export class QAReplayFailure {
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
	    routeRecovery?: routing.RouteRecoveryDecision;
	    suggestedGenericRegression?: string;
	    promotionFingerprint?: string;
	    repeatCount?: number;
	    promotionPriority?: string;
	    coveredByEvalOrTest: boolean;
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
	
	    static createFrom(source: any = {}) {
	        return new QAReplayFailure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.traceId = source["traceId"];
	        this.requestSummary = source["requestSummary"];
	        this.routeCategory = source["routeCategory"];
	        this.routeIntent = source["routeIntent"];
	        this.routeTarget = source["routeTarget"];
	        this.riskLevel = source["riskLevel"];
	        this.failureStage = source["failureStage"];
	        this.failureCode = source["failureCode"];
	        this.failureSubject = source["failureSubject"];
	        this.failureMessage = source["failureMessage"];
	        this.routeFailureCategory = source["routeFailureCategory"];
	        this.routeFailureReason = source["routeFailureReason"];
	        this.routeRecovery = this.convertValues(source["routeRecovery"], routing.RouteRecoveryDecision);
	        this.suggestedGenericRegression = source["suggestedGenericRegression"];
	        this.promotionFingerprint = source["promotionFingerprint"];
	        this.repeatCount = source["repeatCount"];
	        this.promotionPriority = source["promotionPriority"];
	        this.coveredByEvalOrTest = source["coveredByEvalOrTest"];
	        this.coverageSources = source["coverageSources"];
	        this.reviewStatus = source["reviewStatus"];
	        this.reviewNote = source["reviewNote"];
	        this.reviewedAt = source["reviewedAt"];
	        this.testDraftStatus = source["testDraftStatus"];
	        this.testDraftPath = source["testDraftPath"];
	        this.approvedRegression = source["approvedRegression"];
	        this.approvedRegressionAt = source["approvedRegressionAt"];
	        this.approvedRegressionPath = source["approvedRegressionPath"];
	        this.sourcePatchStatus = source["sourcePatchStatus"];
	        this.sourcePatchPath = source["sourcePatchPath"];
	        this.sourcePatchRequiresManualApply = source["sourcePatchRequiresManualApply"];
	        this.sourcePatchApplyStatus = source["sourcePatchApplyStatus"];
	        this.sourcePatchApplyPath = source["sourcePatchApplyPath"];
	        this.sourceTestWriteStatus = source["sourceTestWriteStatus"];
	        this.sourceTestWritePath = source["sourceTestWritePath"];
	        this.sourceTestWriteSnapshotId = source["sourceTestWriteSnapshotId"];
	        this.regressionPromotion = this.convertValues(source["regressionPromotion"], RouteRegressionPromotion);
	        this.resultStatus = source["resultStatus"];
	        this.verificationStatus = source["verificationStatus"];
	        this.coverageStatus = source["coverageStatus"];
	        this.regressionSuggestions = this.convertValues(source["regressionSuggestions"], QARegressionSuggestion);
	        this.matchingCorrectionIds = source["matchingCorrectionIds"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class QAReviewSummary {
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
	
	    static createFrom(source: any = {}) {
	        return new QAReviewSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.replayFailures = source["replayFailures"];
	        this.routeCorrections = source["routeCorrections"];
	        this.pendingCorrections = source["pendingCorrections"];
	        this.regressionSuggestions = source["regressionSuggestions"];
	        this.repeatedSuggestions = source["repeatedSuggestions"];
	        this.approvedSuggestions = source["approvedSuggestions"];
	        this.dismissedSuggestions = source["dismissedSuggestions"];
	        this.coveredSuggestions = source["coveredSuggestions"];
	        this.testDrafts = source["testDrafts"];
	        this.approvedRegressions = source["approvedRegressions"];
	        this.sourcePatches = source["sourcePatches"];
	        this.sourcePatchApplyPlans = source["sourcePatchApplyPlans"];
	        this.sourceTestWrites = source["sourceTestWrites"];
	        this.routeFailureCategories = source["routeFailureCategories"];
	        this.evalStatus = source["evalStatus"];
	    }
	}
	export class QAReview {
	    generatedAt: string;
	    summary: QAReviewSummary;
	    replayFailures: QAReplayFailure[];
	    routeCorrections: routing.RouteCorrection[];
	    latestEval?: QAEvalSummary;
	
	    static createFrom(source: any = {}) {
	        return new QAReview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.summary = this.convertValues(source["summary"], QAReviewSummary);
	        this.replayFailures = this.convertValues(source["replayFailures"], QAReplayFailure);
	        this.routeCorrections = this.convertValues(source["routeCorrections"], routing.RouteCorrection);
	        this.latestEval = this.convertValues(source["latestEval"], QAEvalSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Report {
	    generatedAt: string;
	    workflowSuccesses: number;
	    workflowFailures: number;
	    corrections: number;
	    extensionGenerations: number;
	    recentLearningMemory: LearningMemory[];
	    extensionFailureTrend: extensions.FailureTrend[];
	    suggestions: string[];
	    automaticTraining: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.workflowSuccesses = source["workflowSuccesses"];
	        this.workflowFailures = source["workflowFailures"];
	        this.corrections = source["corrections"];
	        this.extensionGenerations = source["extensionGenerations"];
	        this.recentLearningMemory = this.convertValues(source["recentLearningMemory"], LearningMemory);
	        this.extensionFailureTrend = this.convertValues(source["extensionFailureTrend"], extensions.FailureTrend);
	        this.suggestions = source["suggestions"];
	        this.automaticTraining = source["automaticTraining"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class TrainingInfo {
	    automaticTraining: boolean;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new TrainingInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.automaticTraining = source["automaticTraining"];
	        this.note = source["note"];
	    }
	}
	export class TrajectoryToolRun {
	    toolName: string;
	    status: string;
	    riskLevel?: string;
	    input?: any;
	    output?: any;
	    createdAt: string;
	    completedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new TrajectoryToolRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toolName = source["toolName"];
	        this.status = source["status"];
	        this.riskLevel = source["riskLevel"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.createdAt = source["createdAt"];
	        this.completedAt = source["completedAt"];
	    }
	}
	export class TrajectoryMessage {
	    role: string;
	    content: string;
	    model?: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new TrajectoryMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.model = source["model"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class Trajectory {
	    schema: string;
	    conversationId: string;
	    exportedAt: string;
	    messages: TrajectoryMessage[];
	    toolRuns: TrajectoryToolRun[];
	    privacyFilter: PrivacyFilterReport;
	    training: TrainingInfo;
	
	    static createFrom(source: any = {}) {
	        return new Trajectory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.conversationId = source["conversationId"];
	        this.exportedAt = source["exportedAt"];
	        this.messages = this.convertValues(source["messages"], TrajectoryMessage);
	        this.toolRuns = this.convertValues(source["toolRuns"], TrajectoryToolRun);
	        this.privacyFilter = this.convertValues(source["privacyFilter"], PrivacyFilterReport);
	        this.training = this.convertValues(source["training"], TrainingInfo);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace modelprofiles {
	
	export class ComparisonReport {
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
	
	    static createFrom(source: any = {}) {
	        return new ComparisonReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.baselineName = source["baselineName"];
	        this.candidateName = source["candidateName"];
	        this.hasBaseline = source["hasBaseline"];
	        this.sameArtifact = source["sameArtifact"];
	        this.changedFields = source["changedFields"];
	        this.addedTags = source["addedTags"];
	        this.removedTags = source["removedTags"];
	        this.systemCharsDelta = source["systemCharsDelta"];
	        this.readinessScoreDelta = source["readinessScoreDelta"];
	    }
	}
	export class HistoryEvent {
	    schema: string;
	    kind: string;
	    profileName: string;
	    path?: string;
	    modifiedAt?: string;
	    sizeBytes?: number;
	    readinessScore: number;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.kind = source["kind"];
	        this.profileName = source["profileName"];
	        this.path = source["path"];
	        this.modifiedAt = source["modifiedAt"];
	        this.sizeBytes = source["sizeBytes"];
	        this.readinessScore = source["readinessScore"];
	        this.message = source["message"];
	    }
	}
	export class Metadata {
	    purpose?: string;
	    refinedFrom?: string;
	    createdBy?: string;
	
	    static createFrom(source: any = {}) {
	        return new Metadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.purpose = source["purpose"];
	        this.refinedFrom = source["refinedFrom"];
	        this.createdBy = source["createdBy"];
	    }
	}
	export class Parameters {
	    temperature?: number;
	    numCtx?: number;
	
	    static createFrom(source: any = {}) {
	        return new Parameters(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.temperature = source["temperature"];
	        this.numCtx = source["numCtx"];
	    }
	}
	export class Profile {
	    schemaVersion: number;
	    name: string;
	    description?: string;
	    baseModel: string;
	    system?: string;
	    parameters?: Parameters;
	    metadata?: Metadata;
	    tags?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schemaVersion = source["schemaVersion"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.baseModel = source["baseModel"];
	        this.system = source["system"];
	        this.parameters = this.convertValues(source["parameters"], Parameters);
	        this.metadata = this.convertValues(source["metadata"], Metadata);
	        this.tags = source["tags"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReadinessCheck {
	    id: string;
	    status: string;
	    message: string;
	    points: number;
	    max: number;
	
	    static createFrom(source: any = {}) {
	        return new ReadinessCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.points = source["points"];
	        this.max = source["max"];
	    }
	}
	export class ReadinessReport {
	    schema: string;
	    status: string;
	    score: number;
	    max: number;
	    checks: ReadinessCheck[];
	
	    static createFrom(source: any = {}) {
	        return new ReadinessReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.status = source["status"];
	        this.score = source["score"];
	        this.max = source["max"];
	        this.checks = this.convertValues(source["checks"], ReadinessCheck);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace models {
	
	export class ModelDetails {
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
	    details: Record<string, any>;
	    modelInfo: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ModelDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.modifiedAt = source["modifiedAt"];
	        this.size = source["size"];
	        this.digest = source["digest"];
	        this.family = source["family"];
	        this.format = source["format"];
	        this.parameterSize = source["parameterSize"];
	        this.quantizationLevel = source["quantizationLevel"];
	        this.contextLength = source["contextLength"];
	        this.template = source["template"];
	        this.system = source["system"];
	        this.license = source["license"];
	        this.parameters = source["parameters"];
	        this.modelfile = source["modelfile"];
	        this.details = source["details"];
	        this.modelInfo = source["modelInfo"];
	    }
	}
	export class ModelInfo {
	    name: string;
	    modifiedAt: string;
	    size: number;
	    digest: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.modifiedAt = source["modifiedAt"];
	        this.size = source["size"];
	        this.digest = source["digest"];
	    }
	}

}

export namespace notifications {
	
	export class Notification {
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
	
	    static createFrom(source: any = {}) {
	        return new Notification(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = source["createdAt"];
	        this.type = source["type"];
	        this.severity = source["severity"];
	        this.title = source["title"];
	        this.message = source["message"];
	        this.source = source["source"];
	        this.actionRequired = source["actionRequired"];
	        this.read = source["read"];
	        this.dismissed = source["dismissed"];
	        this.metadata = source["metadata"];
	    }
	}

}

export namespace release {
	
	export class Check {
	    name: string;
	    status: string;
	    details: string;
	
	    static createFrom(source: any = {}) {
	        return new Check(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.details = source["details"];
	    }
	}

}

export namespace replay {
	
	export class Attribute {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new Attribute(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class CapabilityExplanation {
	    capability?: string;
	    action?: string;
	
	    static createFrom(source: any = {}) {
	        return new CapabilityExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.capability = source["capability"];
	        this.action = source["action"];
	    }
	}
	export class ContextBudget {
	    maxBytes?: number;
	    maxTokens?: number;
	    minItemBytes?: number;
	    minItemTokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new ContextBudget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.maxBytes = source["maxBytes"];
	        this.maxTokens = source["maxTokens"];
	        this.minItemBytes = source["minItemBytes"];
	        this.minItemTokens = source["minItemTokens"];
	    }
	}
	export class ContextItemExplanation {
	    source?: string;
	    title?: string;
	    reason?: string;
	    priority?: number;
	    required?: boolean;
	    compressed?: boolean;
	    truncated?: boolean;
	    usedBytes?: number;
	    originalBytes?: number;
	    originalTokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new ContextItemExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.title = source["title"];
	        this.reason = source["reason"];
	        this.priority = source["priority"];
	        this.required = source["required"];
	        this.compressed = source["compressed"];
	        this.truncated = source["truncated"];
	        this.usedBytes = source["usedBytes"];
	        this.originalBytes = source["originalBytes"];
	        this.originalTokens = source["originalTokens"];
	    }
	}
	export class ContextSourceUsage {
	    source?: string;
	    bytes?: number;
	    tokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new ContextSourceUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.bytes = source["bytes"];
	        this.tokens = source["tokens"];
	    }
	}
	export class ContextUsage {
	    usedBytes?: number;
	    usedTokens?: number;
	    originalBytes?: number;
	    originalTokens?: number;
	
	    static createFrom(source: any = {}) {
	        return new ContextUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.usedBytes = source["usedBytes"];
	        this.usedTokens = source["usedTokens"];
	        this.originalBytes = source["originalBytes"];
	        this.originalTokens = source["originalTokens"];
	    }
	}
	export class ContextExplanation {
	    lowMemory?: boolean;
	    budget?: ContextBudget;
	    usage?: ContextUsage;
	    itemsConsidered?: number;
	    itemsSelected?: number;
	    itemsDropped?: number;
	    compressed?: boolean;
	    truncated?: boolean;
	    bySource?: ContextSourceUsage[];
	    included?: ContextItemExplanation[];
	    dropped?: ContextItemExplanation[];
	
	    static createFrom(source: any = {}) {
	        return new ContextExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lowMemory = source["lowMemory"];
	        this.budget = this.convertValues(source["budget"], ContextBudget);
	        this.usage = this.convertValues(source["usage"], ContextUsage);
	        this.itemsConsidered = source["itemsConsidered"];
	        this.itemsSelected = source["itemsSelected"];
	        this.itemsDropped = source["itemsDropped"];
	        this.compressed = source["compressed"];
	        this.truncated = source["truncated"];
	        this.bySource = this.convertValues(source["bySource"], ContextSourceUsage);
	        this.included = this.convertValues(source["included"], ContextItemExplanation);
	        this.dropped = this.convertValues(source["dropped"], ContextItemExplanation);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class PermissionExplanation {
	    id?: string;
	    toolName?: string;
	    status?: string;
	    riskLevel?: string;
	    reason?: string;
	    policyExplanation?: string;
	
	    static createFrom(source: any = {}) {
	        return new PermissionExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.toolName = source["toolName"];
	        this.status = source["status"];
	        this.riskLevel = source["riskLevel"];
	        this.reason = source["reason"];
	        this.policyExplanation = source["policyExplanation"];
	    }
	}
	export class ResultExplanation {
	    status?: string;
	    summary?: string;
	    outputSummary?: string;
	    attributes?: Attribute[];
	
	    static createFrom(source: any = {}) {
	        return new ResultExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.outputSummary = source["outputSummary"];
	        this.attributes = this.convertValues(source["attributes"], Attribute);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RouteExplanation {
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
	
	    static createFrom(source: any = {}) {
	        return new RouteExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.category = source["category"];
	        this.intent = source["intent"];
	        this.domain = source["domain"];
	        this.target = source["target"];
	        this.capability = source["capability"];
	        this.riskLevel = source["riskLevel"];
	        this.confidence = source["confidence"];
	        this.reasons = source["reasons"];
	        this.usesTool = source["usesTool"];
	        this.usesInternet = source["usesInternet"];
	        this.readsFiles = source["readsFiles"];
	        this.writesFiles = source["writesFiles"];
	        this.needsApproval = source["needsApproval"];
	        this.needsClarify = source["needsClarify"];
	        this.generatesTool = source["generatesTool"];
	        this.createsJob = source["createsJob"];
	    }
	}
	export class SourceExplanation {
	    source?: string;
	    title?: string;
	    snippet?: string;
	    score?: number;
	
	    static createFrom(source: any = {}) {
	        return new SourceExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.title = source["title"];
	        this.snippet = source["snippet"];
	        this.score = source["score"];
	    }
	}
	export class ToolConsiderationInfo {
	    name?: string;
	    source?: string;
	    status?: string;
	    reason?: string;
	    riskLevel?: string;
	    attributes?: Attribute[];
	
	    static createFrom(source: any = {}) {
	        return new ToolConsiderationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.riskLevel = source["riskLevel"];
	        this.attributes = this.convertValues(source["attributes"], Attribute);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ToolExplanation {
	    name?: string;
	    status?: string;
	    riskLevel?: string;
	    inputSummary?: string;
	    outputSummary?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.riskLevel = source["riskLevel"];
	        this.inputSummary = source["inputSummary"];
	        this.outputSummary = source["outputSummary"];
	        this.error = source["error"];
	    }
	}
	export class VerificationExplanation {
	    status?: string;
	    checks?: string[];
	    notes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new VerificationExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.checks = source["checks"];
	        this.notes = source["notes"];
	    }
	}
	export class TraceExplanation {
	    schema: string;
	    id: string;
	    requestSummary?: string;
	    route: RouteExplanation;
	    toolsConsidered?: ToolConsiderationInfo[];
	    toolsUsed?: ToolExplanation[];
	    memoryUsed?: SourceExplanation[];
	    ragDocsUsed?: SourceExplanation[];
	    context?: ContextExplanation;
	    permissions?: PermissionExplanation[];
	    verification?: VerificationExplanation;
	    result?: ResultExplanation;
	    missingCapability?: CapabilityExplanation;
	    diagnosticWarnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new TraceExplanation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = source["schema"];
	        this.id = source["id"];
	        this.requestSummary = source["requestSummary"];
	        this.route = this.convertValues(source["route"], RouteExplanation);
	        this.toolsConsidered = this.convertValues(source["toolsConsidered"], ToolConsiderationInfo);
	        this.toolsUsed = this.convertValues(source["toolsUsed"], ToolExplanation);
	        this.memoryUsed = this.convertValues(source["memoryUsed"], SourceExplanation);
	        this.ragDocsUsed = this.convertValues(source["ragDocsUsed"], SourceExplanation);
	        this.context = this.convertValues(source["context"], ContextExplanation);
	        this.permissions = this.convertValues(source["permissions"], PermissionExplanation);
	        this.verification = this.convertValues(source["verification"], VerificationExplanation);
	        this.result = this.convertValues(source["result"], ResultExplanation);
	        this.missingCapability = this.convertValues(source["missingCapability"], CapabilityExplanation);
	        this.diagnosticWarnings = source["diagnosticWarnings"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TraceFile {
	    id: string;
	    path: string;
	    filename: string;
	
	    static createFrom(source: any = {}) {
	        return new TraceFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.path = source["path"];
	        this.filename = source["filename"];
	    }
	}

}

export namespace routing {
	
	export class RouteCorrection {
	    id: string;
	    sourceConversationId?: string;
	    pattern: string;
	    intendedRouteCategory: string;
	    intendedTaskType?: string;
	    intendedCapability?: string;
	    requiredTools?: string[];
	    forbiddenTools?: string[];
	    tags?: string[];
	    originalPrompt?: string;
	    correctionText?: string;
	    clarificationQuestion?: string;
	    approvalStatus: string;
	    disabled: boolean;
	    createdAt?: string;
	    updatedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteCorrection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sourceConversationId = source["sourceConversationId"];
	        this.pattern = source["pattern"];
	        this.intendedRouteCategory = source["intendedRouteCategory"];
	        this.intendedTaskType = source["intendedTaskType"];
	        this.intendedCapability = source["intendedCapability"];
	        this.requiredTools = source["requiredTools"];
	        this.forbiddenTools = source["forbiddenTools"];
	        this.tags = source["tags"];
	        this.originalPrompt = source["originalPrompt"];
	        this.correctionText = source["correctionText"];
	        this.clarificationQuestion = source["clarificationQuestion"];
	        this.approvalStatus = source["approvalStatus"];
	        this.disabled = source["disabled"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class RouteRecoveryDecision {
	    trigger?: string;
	    previous_route?: string;
	    proposed_route?: string;
	    reason?: string;
	    confidence?: number;
	    safe_to_continue?: boolean;
	    requires_clarification?: boolean;
	    requires_approval?: boolean;
	    requires_configuration?: boolean;
	    final_action?: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteRecoveryDecision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.trigger = source["trigger"];
	        this.previous_route = source["previous_route"];
	        this.proposed_route = source["proposed_route"];
	        this.reason = source["reason"];
	        this.confidence = source["confidence"];
	        this.safe_to_continue = source["safe_to_continue"];
	        this.requires_clarification = source["requires_clarification"];
	        this.requires_approval = source["requires_approval"];
	        this.requires_configuration = source["requires_configuration"];
	        this.final_action = source["final_action"];
	    }
	}

}

export namespace safety {
	
	export class PolicyDecision {
	    allowed: boolean;
	    requiresConfirmation: boolean;
	    level: number;
	    risk: string;
	    reason: string;
	    explanation: string;
	
	    static createFrom(source: any = {}) {
	        return new PolicyDecision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.allowed = source["allowed"];
	        this.requiresConfirmation = source["requiresConfirmation"];
	        this.level = source["level"];
	        this.risk = source["risk"];
	        this.reason = source["reason"];
	        this.explanation = source["explanation"];
	    }
	}
	export class PolicyRequest {
	    domain: string;
	    action: string;
	    level: number;
	    policyMode?: string;
	    actor?: string;
	    resource?: string;
	    enabled?: boolean;
	    profileEnabled?: boolean;
	    taskApproved?: boolean;
	    scheduled?: boolean;
	    generated?: boolean;
	    providerConfigured?: boolean;
	    connectorApproved?: boolean;
	    mutating?: boolean;
	    posting?: boolean;
	    trading?: boolean;
	    liveTrading?: boolean;
	    paperTrading?: boolean;
	    deployment?: boolean;
	    privacySensitive?: boolean;
	    passive?: boolean;
	    network?: boolean;
	    secrets?: boolean;
	    fileWrite?: boolean;
	    domains?: string[];
	
	    static createFrom(source: any = {}) {
	        return new PolicyRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.domain = source["domain"];
	        this.action = source["action"];
	        this.level = source["level"];
	        this.policyMode = source["policyMode"];
	        this.actor = source["actor"];
	        this.resource = source["resource"];
	        this.enabled = source["enabled"];
	        this.profileEnabled = source["profileEnabled"];
	        this.taskApproved = source["taskApproved"];
	        this.scheduled = source["scheduled"];
	        this.generated = source["generated"];
	        this.providerConfigured = source["providerConfigured"];
	        this.connectorApproved = source["connectorApproved"];
	        this.mutating = source["mutating"];
	        this.posting = source["posting"];
	        this.trading = source["trading"];
	        this.liveTrading = source["liveTrading"];
	        this.paperTrading = source["paperTrading"];
	        this.deployment = source["deployment"];
	        this.privacySensitive = source["privacySensitive"];
	        this.passive = source["passive"];
	        this.network = source["network"];
	        this.secrets = source["secrets"];
	        this.fileWrite = source["fileWrite"];
	        this.domains = source["domains"];
	    }
	}
	export class PolicyAuditRecord {
	    id?: string;
	    line?: number;
	    timestamp: string;
	    request: PolicyRequest;
	    decision: PolicyDecision;
	
	    static createFrom(source: any = {}) {
	        return new PolicyAuditRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.line = source["line"];
	        this.timestamp = source["timestamp"];
	        this.request = this.convertValues(source["request"], PolicyRequest);
	        this.decision = this.convertValues(source["decision"], PolicyDecision);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	

}

export namespace scheduler {
	
	export class Job {
	    id: string;
	    name: string;
	    scheduleType: string;
	    scheduleExpr: string;
	    targetType: string;
	    targetName: string;
	    input: Record<string, any>;
	    enabled: boolean;
	    approved: boolean;
	    retryPolicy: string;
	    maxAttempts: number;
	    backoffSeconds: number;
	    createdAt: string;
	    updatedAt: string;
	    lastRunAt: string;
	    nextDueAt: string;
	    archivedAt?: string;
	    inputStatus?: string;
	    inputValidationError?: string;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.scheduleType = source["scheduleType"];
	        this.scheduleExpr = source["scheduleExpr"];
	        this.targetType = source["targetType"];
	        this.targetName = source["targetName"];
	        this.input = source["input"];
	        this.enabled = source["enabled"];
	        this.approved = source["approved"];
	        this.retryPolicy = source["retryPolicy"];
	        this.maxAttempts = source["maxAttempts"];
	        this.backoffSeconds = source["backoffSeconds"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastRunAt = source["lastRunAt"];
	        this.nextDueAt = source["nextDueAt"];
	        this.archivedAt = source["archivedAt"];
	        this.inputStatus = source["inputStatus"];
	        this.inputValidationError = source["inputValidationError"];
	    }
	}
	export class JobRun {
	    id: string;
	    jobId: string;
	    jobName?: string;
	    targetType?: string;
	    targetName?: string;
	    scheduleType?: string;
	    status: string;
	    output: any;
	    startedAt: string;
	    finishedAt: string;
	    durationMs: number;
	    error: string;
	    attempt: number;
	    retryDueAt: string;
	
	    static createFrom(source: any = {}) {
	        return new JobRun(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.jobId = source["jobId"];
	        this.jobName = source["jobName"];
	        this.targetType = source["targetType"];
	        this.targetName = source["targetName"];
	        this.scheduleType = source["scheduleType"];
	        this.status = source["status"];
	        this.output = source["output"];
	        this.startedAt = source["startedAt"];
	        this.finishedAt = source["finishedAt"];
	        this.durationMs = source["durationMs"];
	        this.error = source["error"];
	        this.attempt = source["attempt"];
	        this.retryDueAt = source["retryDueAt"];
	    }
	}
	export class Status {
	    enabled: boolean;
	    totalJobs: number;
	    enabledJobs: number;
	    approvedJobs: number;
	    archivedJobs: number;
	    runningJobs: number;
	    maxParallelJobs: number;
	    invalidInputJobs: number;
	    lastRunAt: string;
	    lastRunStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.totalJobs = source["totalJobs"];
	        this.enabledJobs = source["enabledJobs"];
	        this.approvedJobs = source["approvedJobs"];
	        this.archivedJobs = source["archivedJobs"];
	        this.runningJobs = source["runningJobs"];
	        this.maxParallelJobs = source["maxParallelJobs"];
	        this.invalidInputJobs = source["invalidInputJobs"];
	        this.lastRunAt = source["lastRunAt"];
	        this.lastRunStatus = source["lastRunStatus"];
	    }
	}
	export class TickResult {
	    checkedAt: string;
	    runs: JobRun[];
	    skipped: number;
	    missedRuns: number;
	    missedPolicy: string;
	
	    static createFrom(source: any = {}) {
	        return new TickResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.checkedAt = source["checkedAt"];
	        this.runs = this.convertValues(source["runs"], JobRun);
	        this.skipped = source["skipped"];
	        this.missedRuns = source["missedRuns"];
	        this.missedPolicy = source["missedPolicy"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace secrets {
	
	export class Reference {
	    provider: string;
	    name: string;
	    service?: string;
	    account?: string;
	
	    static createFrom(source: any = {}) {
	        return new Reference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.name = source["name"];
	        this.service = source["service"];
	        this.account = source["account"];
	    }
	}
	export class Status {
	    provider: string;
	    name: string;
	    service?: string;
	    account?: string;
	    ready: boolean;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.name = source["name"];
	        this.service = source["service"];
	        this.account = source["account"];
	        this.ready = source["ready"];
	        this.detail = source["detail"];
	    }
	}

}

export namespace server {
	
	export class ConnectorOpsSummary {
	    total: number;
	    enabled: number;
	    disabled: number;
	    needsAuth: number;
	    warning: number;
	    broken: number;
	    status: string;
	    enabledList?: string[];
	
	    static createFrom(source: any = {}) {
	        return new ConnectorOpsSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.enabled = source["enabled"];
	        this.disabled = source["disabled"];
	        this.needsAuth = source["needsAuth"];
	        this.warning = source["warning"];
	        this.broken = source["broken"];
	        this.status = source["status"];
	        this.enabledList = source["enabledList"];
	    }
	}
	export class KnowledgeStatus {
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
	
	    static createFrom(source: any = {}) {
	        return new KnowledgeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.influenceEnabled = source["influenceEnabled"];
	        this.manualOnly = source["manualOnly"];
	        this.maxEntitiesPerQuery = source["maxEntitiesPerQuery"];
	        this.maxEvidenceChars = source["maxEvidenceChars"];
	        this.maxInfluenceEntities = source["maxInfluenceEntities"];
	        this.maxInfluenceChars = source["maxInfluenceChars"];
	        this.database = source["database"];
	        this.entityCount = source["entityCount"];
	        this.edgeCount = source["edgeCount"];
	        this.message = source["message"];
	    }
	}
	export class ModelProfileOpsSummary {
	    profileCount: number;
	    configuredRoles: number;
	    appliedRoles: number;
	    missingBindings: number;
	    missingProfiles?: string[];
	    availableProfiles?: string[];
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new ModelProfileOpsSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profileCount = source["profileCount"];
	        this.configuredRoles = source["configuredRoles"];
	        this.appliedRoles = source["appliedRoles"];
	        this.missingBindings = source["missingBindings"];
	        this.missingProfiles = source["missingProfiles"];
	        this.availableProfiles = source["availableProfiles"];
	        this.status = source["status"];
	    }
	}
	export class OpsReleaseSummary {
	    version: string;
	    ready: boolean;
	    checks: release.Check[];
	
	    static createFrom(source: any = {}) {
	        return new OpsReleaseSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.ready = source["ready"];
	        this.checks = this.convertValues(source["checks"], release.Check);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OpsStage {
	    name: string;
	    status: string;
	    details?: string;
	
	    static createFrom(source: any = {}) {
	        return new OpsStage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.details = source["details"];
	    }
	}
	export class ToolRunResult {
	    id: string;
	    conversationId: string;
	    sessionId: string;
	    userMessageId: string;
	    assistantMessageId: string;
	    parentMessageId: string;
	    variantIndex: number;
	    toolName: string;
	    input: any;
	    output: any;
	    status: string;
	    riskLevel: string;
	    createdAt: string;
	    completedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolRunResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.conversationId = source["conversationId"];
	        this.sessionId = source["sessionId"];
	        this.userMessageId = source["userMessageId"];
	        this.assistantMessageId = source["assistantMessageId"];
	        this.parentMessageId = source["parentMessageId"];
	        this.variantIndex = source["variantIndex"];
	        this.toolName = source["toolName"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.status = source["status"];
	        this.riskLevel = source["riskLevel"];
	        this.createdAt = source["createdAt"];
	        this.completedAt = source["completedAt"];
	    }
	}
	export class OpsTimelineLink {
	    source: string;
	    entityType: string;
	    entityId: string;
	    label?: string;
	
	    static createFrom(source: any = {}) {
	        return new OpsTimelineLink(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.entityType = source["entityType"];
	        this.entityId = source["entityId"];
	        this.label = source["label"];
	    }
	}
	export class OpsTimelineEvent {
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
	    related?: OpsTimelineLink[];
	
	    static createFrom(source: any = {}) {
	        return new OpsTimelineEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.occurredAt = source["occurredAt"];
	        this.source = source["source"];
	        this.kind = source["kind"];
	        this.severity = source["severity"];
	        this.status = source["status"];
	        this.title = source["title"];
	        this.summary = source["summary"];
	        this.entityType = source["entityType"];
	        this.entityId = source["entityId"];
	        this.correlationId = source["correlationId"];
	        this.metadata = source["metadata"];
	        this.related = this.convertValues(source["related"], OpsTimelineLink);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OpsStatus {
	    generatedAt: string;
	    overall: string;
	    stages: OpsStage[];
	    timeline?: OpsTimelineEvent[];
	    heartbeat: heartbeat.Report;
	    scheduler: scheduler.Status;
	    connectors: ConnectorOpsSummary;
	    knowledge: KnowledgeStatus;
	    modelProfiles: ModelProfileOpsSummary;
	    recentJobRuns: scheduler.JobRun[];
	    latestEval?: learning.QAEvalSummary;
	    qaReview: learning.QAReviewSummary;
	    notifications: notifications.Notification[];
	    recentReplay: replay.TraceFile[];
	    recentToolRuns: ToolRunResult[];
	    extensionAudit?: extensions.AuditRecord[];
	    policyAudit?: safety.PolicyAuditRecord[];
	    release?: OpsReleaseSummary;
	
	    static createFrom(source: any = {}) {
	        return new OpsStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.generatedAt = source["generatedAt"];
	        this.overall = source["overall"];
	        this.stages = this.convertValues(source["stages"], OpsStage);
	        this.timeline = this.convertValues(source["timeline"], OpsTimelineEvent);
	        this.heartbeat = this.convertValues(source["heartbeat"], heartbeat.Report);
	        this.scheduler = this.convertValues(source["scheduler"], scheduler.Status);
	        this.connectors = this.convertValues(source["connectors"], ConnectorOpsSummary);
	        this.knowledge = this.convertValues(source["knowledge"], KnowledgeStatus);
	        this.modelProfiles = this.convertValues(source["modelProfiles"], ModelProfileOpsSummary);
	        this.recentJobRuns = this.convertValues(source["recentJobRuns"], scheduler.JobRun);
	        this.latestEval = this.convertValues(source["latestEval"], learning.QAEvalSummary);
	        this.qaReview = this.convertValues(source["qaReview"], learning.QAReviewSummary);
	        this.notifications = this.convertValues(source["notifications"], notifications.Notification);
	        this.recentReplay = this.convertValues(source["recentReplay"], replay.TraceFile);
	        this.recentToolRuns = this.convertValues(source["recentToolRuns"], ToolRunResult);
	        this.extensionAudit = this.convertValues(source["extensionAudit"], extensions.AuditRecord);
	        this.policyAudit = this.convertValues(source["policyAudit"], safety.PolicyAuditRecord);
	        this.release = this.convertValues(source["release"], OpsReleaseSummary);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RuntimeControlResult {
	    action: string;
	    accepted: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RuntimeControlResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.accepted = source["accepted"];
	        this.message = source["message"];
	    }
	}

}

export namespace skills {
	
	export class ContextBudget {
	    MaxInstructionChars: number;
	    MaxExamples: number;
	
	    static createFrom(source: any = {}) {
	        return new ContextBudget(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.MaxInstructionChars = source["MaxInstructionChars"];
	        this.MaxExamples = source["MaxExamples"];
	    }
	}
	export class Permissions {
	    FilesystemRead: boolean;
	    FilesystemWrite: string;
	    Shell: string;
	    Network: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Permissions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.FilesystemRead = source["FilesystemRead"];
	        this.FilesystemWrite = source["FilesystemWrite"];
	        this.Shell = source["Shell"];
	        this.Network = source["Network"];
	    }
	}

}

export namespace workflows {
	
	export class TemplateSummary {
	    name: string;
	    version: string;
	    description?: string;
	    category?: string;
	    source?: string;
	    path?: string;
	    installed: boolean;
	    enabled: boolean;
	    valid: boolean;
	    validationError?: string;
	    repairHint?: string;
	    defaultState: string;
	    requiredTools?: string[];
	    skills?: string[];
	    workflows?: string[];
	    approvalRequiredFor?: string[];
	    blockedActions?: string[];
	    sensitiveDataRules?: string[];
	    safetySummary?: string;
	    installAction: string;
	
	    static createFrom(source: any = {}) {
	        return new TemplateSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.category = source["category"];
	        this.source = source["source"];
	        this.path = source["path"];
	        this.installed = source["installed"];
	        this.enabled = source["enabled"];
	        this.valid = source["valid"];
	        this.validationError = source["validationError"];
	        this.repairHint = source["repairHint"];
	        this.defaultState = source["defaultState"];
	        this.requiredTools = source["requiredTools"];
	        this.skills = source["skills"];
	        this.workflows = source["workflows"];
	        this.approvalRequiredFor = source["approvalRequiredFor"];
	        this.blockedActions = source["blockedActions"];
	        this.sensitiveDataRules = source["sensitiveDataRules"];
	        this.safetySummary = source["safetySummary"];
	        this.installAction = source["installAction"];
	    }
	}

}

export namespace workspace {
	
	export class WriteVerification {
	    status: string;
	    checks: string[];
	    reasons: string[];
	    snapshotId: string;
	    rollbackAvailable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WriteVerification(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.checks = source["checks"];
	        this.reasons = source["reasons"];
	        this.snapshotId = source["snapshotId"];
	        this.rollbackAvailable = source["rollbackAvailable"];
	    }
	}

}
