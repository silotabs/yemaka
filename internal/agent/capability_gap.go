package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yemaka/internal/capabilities"
	"yemaka/internal/config"
	"yemaka/internal/extensions"
	"yemaka/internal/tools"
	"yemaka/internal/workflows"
)

const (
	defaultSynthesizedDraftRepairAttempts = 1
	maxSynthesizedDraftRepairAttempts     = 2
)

const (
	CapabilityKindSettings      = "settings"
	CapabilityKindToolExtension = "tool_extension"
	CapabilityKindConnector     = "connector"
	CapabilityKindWorkflow      = "workflow"
	CapabilityKindSkill         = "skill"
)

type ExtensionProposer interface {
	Propose(request string) (extensions.Proposal, error)
}

type ExtensionGenerator interface {
	ExtensionProposer
	Generate(ctx context.Context, input extensions.GenerateInput) (extensions.GenerationResult, error)
}

type ExtensionRunner interface {
	Run(ctx context.Context, name string, options extensions.RunOptions) (extensions.RunResult, error)
}

type CapabilityGapRouter struct {
	Extensions               ExtensionProposer
	CapabilityRegistry       capabilities.Registry
	ExtensionsEnabled        bool
	InternetEnabled          bool
	InternetProfileApproved  bool
	InternetSearchEnabled    bool
	InternetSearchProvider   string
	ConnectorsEnabled        bool
	PreferredExtensionRunner string
}

type CapabilityGapProposal struct {
	Needed             bool                     `json:"needed"`
	Kind               string                   `json:"kind"`
	Name               string                   `json:"name,omitempty"`
	Title              string                   `json:"title"`
	Description        string                   `json:"description"`
	Reason             string                   `json:"reason"`
	SuggestedAction    string                   `json:"suggestedAction"`
	CanGenerate        bool                     `json:"canGenerate"`
	RequiresApproval   bool                     `json:"requiresApproval"`
	Files              []string                 `json:"files,omitempty"`
	Permissions        []string                 `json:"permissions,omitempty"`
	GenerationCommand  string                   `json:"generationCommand,omitempty"`
	NetworkMode        string                   `json:"networkMode,omitempty"`
	AllowedDomains     []string                 `json:"allowedDomains,omitempty"`
	ExistingCapability string                   `json:"existingCapability,omitempty"`
	CapabilitySummary  string                   `json:"capabilitySummary,omitempty"`
	CapabilityAction   string                   `json:"capabilityAction,omitempty"`
	CapabilityState    string                   `json:"capabilityState,omitempty"`
	ConfigureHint      string                   `json:"configureHint,omitempty"`
	PackSource         string                   `json:"packSource,omitempty"`
	PackDir            string                   `json:"packDir,omitempty"`
	InstallCommand     string                   `json:"installCommand,omitempty"`
	RuntimeStatus      *CapabilityRuntimeStatus `json:"runtimeStatus,omitempty"`
	Extension          *extensions.Proposal     `json:"extension,omitempty"`
}

type CapabilityRuntimeStatus struct {
	Source           string   `json:"source,omitempty"`
	State            string   `json:"state,omitempty"`
	Installed        bool     `json:"installed"`
	Enabled          bool     `json:"enabled"`
	Valid            bool     `json:"valid"`
	RequiresApproval bool     `json:"requiresApproval"`
	Reason           string   `json:"reason,omitempty"`
	ConfigureHint    string   `json:"configureHint,omitempty"`
	RepairHint       string   `json:"repairHint,omitempty"`
	RequiredTools    []string `json:"requiredTools,omitempty"`
	OptionalTools    []string `json:"optionalTools,omitempty"`
	Skills           []string `json:"skills,omitempty"`
	RiskyTools       []string `json:"riskyTools,omitempty"`
	RiskReason       string   `json:"riskReason,omitempty"`
}

type CapabilityGenerationInput struct {
	Request             string                   `json:"request"`
	Approved            bool                     `json:"approved"`
	Name                string                   `json:"name,omitempty"`
	AllowedDomains      []string                 `json:"allowedDomains,omitempty"`
	MaxRuntimeSeconds   int                      `json:"maxRuntimeSeconds,omitempty"`
	RunAfterGenerate    bool                     `json:"runAfterGenerate,omitempty"`
	RunInput            map[string]any           `json:"runInput,omitempty"`
	Draft               *extensions.PackageDraft `json:"draft,omitempty"`
	SynthesizeDraft     bool                     `json:"synthesizeDraft,omitempty"`
	DraftRepairAttempts int                      `json:"draftRepairAttempts,omitempty"`
	DraftModel          config.ModelConfig       `json:"-"`
	DraftRuntimeFactory RuntimeFactory           `json:"-"`
	RunOptions          extensions.RunOptions    `json:"-"`
	TestTimeout         time.Duration            `json:"-"`
}

type CapabilityGenerationResult struct {
	Proposal   CapabilityGapProposal       `json:"proposal"`
	Generation extensions.GenerationResult `json:"generation"`
	Run        *extensions.RunResult       `json:"run,omitempty"`
	Schedule   *CapabilityScheduledJob     `json:"schedule,omitempty"`
	Repairs    int                         `json:"repairs,omitempty"`
	NextSteps  []string                    `json:"nextSteps,omitempty"`
	Message    string                      `json:"message"`
}

type CapabilityScheduledJob struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	ScheduleType string         `json:"scheduleType"`
	ScheduleExpr string         `json:"scheduleExpr"`
	TargetType   string         `json:"targetType"`
	TargetName   string         `json:"targetName"`
	Input        map[string]any `json:"input,omitempty"`
	Enabled      bool           `json:"enabled"`
	Approved     bool           `json:"approved"`
	NextDueAt    string         `json:"nextDueAt,omitempty"`
}

func NewCapabilityGapRouter(cfg *config.Config, proposer ExtensionProposer) *CapabilityGapRouter {
	registry := capabilityRegistryFromConfig(cfg)
	if cfg == nil {
		return &CapabilityGapRouter{Extensions: proposer, CapabilityRegistry: registry}
	}
	policyMode := strings.TrimSpace(cfg.Security.Policy.Mode)
	return &CapabilityGapRouter{
		Extensions:               proposer,
		CapabilityRegistry:       registry,
		ExtensionsEnabled:        cfg.Extensions.Enabled,
		InternetEnabled:          cfg.Internet.Enabled,
		InternetProfileApproved:  InternetToolLoopEnabled(cfg, policyMode),
		InternetSearchEnabled:    InternetSearchLoopEnabled(cfg, policyMode),
		InternetSearchProvider:   strings.TrimSpace(cfg.Internet.Search.Provider),
		ConnectorsEnabled:        cfg.Connectors.Enabled,
		PreferredExtensionRunner: cfg.Extensions.PreferredLanguage,
	}
}

func (r *CapabilityGapRouter) Propose(plan Plan, input PlanInput, decision ExecutionDecision) (CapabilityGapProposal, bool) {
	if r == nil {
		return CapabilityGapProposal{}, false
	}
	if !isCapabilityGapDecision(decision) {
		return CapabilityGapProposal{}, false
	}
	request := strings.TrimSpace(input.Content)
	if request == "" {
		request = strings.TrimSpace(plan.Goal)
	}
	if request == "" {
		request = strings.TrimSpace(decision.ToolName)
	}
	lowerRequest := strings.ToLower(request)
	lowerReason := strings.ToLower(strings.TrimSpace(decision.Reason))
	if isExplanationOnlyCapabilityRequest(lowerRequest) {
		return CapabilityGapProposal{}, false
	}
	registryDecision := r.classifyCapabilityGap(plan, input, decision)
	if !requestPrefersNonPackCapability(lowerRequest, decision, lowerReason) || !isTemplateCapability(registryDecision) {
		if proposal, ok := packCapabilityProposal(request, decision, registryDecision); ok {
			return proposal, true
		}
	}

	if (looksExplicitToolGeneration(lowerRequest) || looksBrokeredWorkflowGeneration(lowerRequest)) && looksBrokeredNetworkExtensionRequest(lowerRequest) {
		if looksWorkflowCapability(lowerRequest) {
			return annotateCapabilityProposal(r.workflowProposal(request, decision), registryDecision), true
		}
		return annotateCapabilityProposal(r.toolExtensionProposal(request, decision), registryDecision), true
	}
	if isInternetConfigurationIssue(decision, lowerReason) {
		return CapabilityGapProposal{}, false
	}
	if isInternetToolName(decision.ToolName) || strings.Contains(lowerReason, "internet") {
		return annotateCapabilityProposal(r.internetProposal(decision, lowerReason), registryDecision), true
	}
	if looksConnectorCapability(lowerRequest) {
		return annotateCapabilityProposal(r.connectorProposal(request, decision), registryDecision), true
	}
	if looksSkillCapability(lowerRequest) {
		return annotateCapabilityProposal(r.skillProposal(request, decision), registryDecision), true
	}
	if looksWorkflowCapability(lowerRequest) {
		return annotateCapabilityProposal(r.workflowProposal(request, decision), registryDecision), true
	}
	if decision.ToolName == "safe_tool" ||
		strings.Contains(lowerReason, "no matching safe executor") ||
		strings.Contains(lowerReason, "no safe executor") ||
		strings.Contains(lowerReason, "not allowed in the typed low-resource tool loop") ||
		strings.Contains(lowerReason, "model requested a tool but no safe executor") {
		return annotateCapabilityProposal(r.toolExtensionProposal(request, decision), registryDecision), true
	}
	if proposal, ok := r.registryBackedProposal(request, decision, registryDecision); ok {
		return proposal, true
	}
	return CapabilityGapProposal{}, false
}

func (r *CapabilityGapRouter) ProposeRequest(request string) (CapabilityGapProposal, error) {
	if r == nil {
		return CapabilityGapProposal{}, fmt.Errorf("capability gap router is nil")
	}
	request = strings.TrimSpace(request)
	if request == "" {
		return CapabilityGapProposal{}, fmt.Errorf("capability request is required")
	}
	if isExplanationOnlyCapabilityRequest(strings.ToLower(request)) {
		return CapabilityGapProposal{
			Needed:          false,
			Kind:            "none",
			Title:           "No capability gap detected",
			Description:     compactCapabilityDescription(request),
			Reason:          "This is an explanation request, not a request to generate or enable a capability.",
			SuggestedAction: "Answer normally without creating, enabling, or scheduling any extension.",
		}, nil
	}
	input := PlanInput{Content: request}
	plan := BuildPlan(input)
	decision := DecideExecution(plan, input)
	if decision.Status == ExecutionReady && isInternetToolName(decision.ToolName) && !r.InternetProfileApproved {
		decision.Status = ExecutionBlocked
		decision.Reason = "controlled internet tools exist, but internet access is disabled or not profile-approved for this agent session"
	}
	if proposal, ok := r.Propose(plan, input, decision); ok {
		return proposal, nil
	}
	registryDecision := r.classifyCapabilityGap(plan, input, decision)
	if !requestPrefersNonPackCapability(strings.ToLower(request), decision, strings.ToLower(strings.TrimSpace(decision.Reason))) || !isTemplateCapability(registryDecision) {
		if proposal, ok := packCapabilityProposal(request, decision, registryDecision); ok {
			return proposal, nil
		}
	}
	if proposal, ok := r.registryBackedProposal(request, decision, registryDecision); ok {
		return proposal, nil
	}
	return CapabilityGapProposal{
		Needed:          false,
		Kind:            "none",
		Title:           "No capability gap detected",
		Description:     compactCapabilityDescription(request),
		Reason:          "The current thin core can route this request through existing chat, RAG, memory, or safe tool paths.",
		SuggestedAction: "Run the request normally. If Yemaka later hits a missing tool, it will propose the smallest safe capability.",
	}, nil
}

func capabilityRegistryFromConfig(cfg *config.Config) capabilities.Registry {
	statuses := []capabilities.PrimitiveStatus{}
	if cfg != nil {
		if cfg.Extensions.Enabled {
			statuses = append(statuses, capabilities.Ready("extension_generation", capabilities.KindExtension))
		}
		if cfg.Internet.Enabled {
			statuses = append(statuses, capabilities.Ready("internet", capabilities.KindInternetProvider))
		}
		if cfg.Connectors.Enabled {
			statuses = append(statuses, capabilities.Ready("connector", capabilities.KindConnector))
		}
		if cfg.Scheduler.Enabled {
			scheduler := capabilities.Ready("scheduler", capabilities.KindScheduler)
			scheduler.RequiresApproval = cfg.Scheduler.RequireApprovalForNewJobs
			statuses = append(statuses, scheduler)
		}
		for role, model := range cfg.Models {
			role = strings.TrimSpace(role)
			if role == "" {
				continue
			}
			capability := capabilities.Ready(role, capabilities.KindModelRole)
			capability.Description = strings.TrimSpace(strings.Join([]string{model.Provider, model.Name}, " "))
			capability.Tags = []string{"local", "model"}
			capability.Provides = []string{role, model.Provider, model.Name}
			statuses = append(statuses, capability)
		}
	}
	registry, err := capabilities.DefaultLocalFirstSnapshot(statuses...)
	if err != nil {
		return capabilities.Registry{}
	}
	addInstalledDomainPacks(&registry, cfg)
	addTemplateDomainPacks(&registry)
	return registry
}

func addInstalledDomainPacks(registry *capabilities.Registry, cfg *config.Config) {
	if registry == nil {
		return
	}
	root := domainPackRootFromConfig(cfg)
	if root == "" {
		return
	}
	packs, err := capabilities.DomainPackCapabilitiesFromRoot(root)
	if err != nil {
		return
	}
	for _, pack := range packs {
		_ = registry.AddDomainPack(pack)
	}
}

func addTemplateDomainPacks(registry *capabilities.Registry) {
	if registry == nil {
		return
	}
	repoRoot := workflowTemplateRepoRoot()
	if repoRoot == "" {
		return
	}
	catalog, err := workflows.NewBuiltInTemplateCatalog(repoRoot)
	if err != nil {
		return
	}
	packs, err := capabilities.DomainPackTemplateCapabilities(catalog)
	if err != nil {
		return
	}
	for _, pack := range packs {
		if _, exists := registry.Get(capabilities.KindDomainPack, pack.Name); exists {
			continue
		}
		_ = registry.AddDomainPack(pack)
	}
}

func workflowTemplateRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	dir = filepath.Clean(dir)
	for {
		templates := filepath.Join(dir, "packs", "templates")
		if info, err := os.Stat(templates); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func domainPackRootFromConfig(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	profile := strings.TrimSpace(cfg.App.Profile)
	if profile == "" {
		profile = config.DefaultProfile
	}
	supportDir := ""
	if path := strings.TrimSpace(cfg.Path); path != "" {
		supportDir = filepath.Dir(config.ExpandPath(path))
	} else {
		resolved, err := config.SupportDir()
		if err != nil {
			return ""
		}
		supportDir = resolved
	}
	if strings.TrimSpace(supportDir) == "" {
		return ""
	}
	return filepath.Join(supportDir, "profiles", profile, "domain_packs")
}

func (r *CapabilityGapRouter) classifyCapabilityGap(plan Plan, input PlanInput, decision ExecutionDecision) capabilities.GapDecision {
	if r == nil {
		return capabilities.GapDecision{}
	}
	registry := r.CapabilityRegistry.Snapshot()
	route := capabilities.RouteMetadata{
		Request:             strings.TrimSpace(firstNonEmpty(input.Content, plan.Goal, decision.ToolName)),
		RouteCategory:       plan.RouteCategory,
		Intent:              plan.RouteIntent,
		Domain:              plan.RouteDomain,
		Target:              plan.RouteTarget,
		Capability:          plan.RouteCapability,
		Tools:               append([]string{}, plan.ToolsNeeded...),
		RequiresApproval:    plan.RouteRequiresApproval,
		UseWorkspace:        plan.RouteUsesWorkspace,
		UseRAG:              plan.RouteUsesRAG,
		UseInternet:         plan.RouteUsesInternet,
		ReadsFiles:          plan.RouteReadsFiles,
		WritesFiles:         plan.RouteWritesFiles,
		GeneratesExtension:  plan.RouteGeneratesExtension,
		CreatesSchedulerJob: plan.RouteCreatesSchedulerJob,
		ConnectorAction:     plan.RouteConnectorAction,
		CrawlerTask:         plan.RouteCrawlerTask,
		RunsShell:           plan.RouteRunsShell,
	}
	if strings.TrimSpace(route.Capability) == "" {
		route.Capability = capabilityFromDecision(decision)
	}
	if strings.TrimSpace(route.RouteCategory) == "" && isInternetToolName(decision.ToolName) {
		route.UseInternet = true
	}
	if route.GeneratesExtension || (strings.TrimSpace(decision.ToolName) == "safe_tool" && looksExplicitToolGeneration(strings.ToLower(route.Request))) {
		route.GeneratesExtension = true
		route.Tools = nil
		return registry.ClassifyRouteGap(route)
	}
	return registry.ClassifyRouteGap(route)
}

func capabilityFromDecision(decision ExecutionDecision) string {
	switch strings.TrimSpace(decision.ToolName) {
	case "internet_search", "internet_fetch", "internet_head":
		return "internet"
	case "memory_search":
		return "memory"
	case "rag_search":
		return "rag"
	case "safe_tool":
		return "extension_generation"
	case "local_time":
		return "local_time"
	default:
		return ""
	}
}

func (r *CapabilityGapRouter) registryBackedProposal(request string, decision ExecutionDecision, gap capabilities.GapDecision) (CapabilityGapProposal, bool) {
	if gap.Action == "" || gap.Action == capabilities.ActionUseExisting || gap.Action == capabilities.ActionUsePack || gap.Action == capabilities.ActionAskClarification {
		return CapabilityGapProposal{}, false
	}
	switch gap.Kind {
	case capabilities.KindInternetProvider:
		return annotateCapabilityProposal(r.internetProposal(decision, strings.ToLower(gap.Reason)), gap), true
	case capabilities.KindConnector:
		return annotateCapabilityProposal(r.connectorProposal(request, decision), gap), true
	case capabilities.KindScheduler:
		return annotateCapabilityProposal(r.workflowProposal(request, decision), gap), true
	case capabilities.KindExtension:
		if gap.Name == "extension_generation" || gap.Action == capabilities.ActionGenerateExtension {
			if looksWorkflowCapability(strings.ToLower(request)) {
				return annotateCapabilityProposal(r.workflowProposal(request, decision), gap), true
			}
			return annotateCapabilityProposal(r.toolExtensionProposal(request, decision), gap), true
		}
	case capabilities.KindModelRole:
		return annotateCapabilityProposal(settingsCapabilityProposal(request, decision, gap), gap), true
	}
	return CapabilityGapProposal{}, false
}

func packCapabilityProposal(request string, decision ExecutionDecision, gap capabilities.GapDecision) (CapabilityGapProposal, bool) {
	if !isPackKind(gap.Kind) {
		return CapabilityGapProposal{}, false
	}
	label := packKindLabel(gap.Kind)
	switch gap.Action {
	case capabilities.ActionUsePack:
		return annotateCapabilityProposal(CapabilityGapProposal{
			Needed:             false,
			Kind:               string(gap.Kind),
			Name:               gap.Name,
			Title:              "Use enabled " + label,
			Description:        compactCapabilityDescription(request),
			Reason:             firstNonEmpty(gap.Reason, decision.Reason, "an enabled "+label+" matches this task"),
			SuggestedAction:    nonEmpty(gap.SuggestedAction, "Use the enabled pack through the normal policy flow."),
			CanGenerate:        false,
			RequiresApproval:   gap.RequiresApproval,
			Permissions:        []string{"policy=core_enforced"},
			ExistingCapability: gap.Name,
		}, gap), true
	case capabilities.ActionAskConfig:
		title := "Enable the matching " + label
		suggested := "Enable the pack explicitly before routing work to it."
		if isTemplateCapability(gap) {
			title = "Install the matching " + label
			suggested = "Install the local workflow pack template, then enable it explicitly before routing work to it."
		}
		return annotateCapabilityProposal(CapabilityGapProposal{
			Needed:             true,
			Kind:               string(gap.Kind),
			Name:               gap.Name,
			Title:              title,
			Description:        compactCapabilityDescription(request),
			Reason:             firstNonEmpty(gap.Reason, decision.Reason, "a matching "+label+" is installed but not ready"),
			SuggestedAction:    nonEmpty(gap.SuggestedAction, suggested),
			CanGenerate:        false,
			RequiresApproval:   true,
			Permissions:        []string{"policy=core_enforced"},
			ExistingCapability: gap.Name,
		}, gap), true
	case capabilities.ActionUnsupported:
		return annotateCapabilityProposal(CapabilityGapProposal{
			Needed:             true,
			Kind:               string(gap.Kind),
			Name:               gap.Name,
			Title:              "Blocked " + label,
			Description:        compactCapabilityDescription(request),
			Reason:             firstNonEmpty(gap.Reason, decision.Reason, "the matching "+label+" is unavailable"),
			SuggestedAction:    nonEmpty(gap.SuggestedAction, "Repair or remove the pack before retrying."),
			CanGenerate:        false,
			RequiresApproval:   false,
			Permissions:        []string{"policy=core_enforced"},
			ExistingCapability: gap.Name,
		}, gap), true
	default:
		return CapabilityGapProposal{}, false
	}
}

func requestPrefersNonPackCapability(lowerRequest string, decision ExecutionDecision, lowerReason string) bool {
	lowerRequest = strings.ToLower(strings.TrimSpace(lowerRequest))
	lowerReason = strings.ToLower(strings.TrimSpace(lowerReason))
	if looksExplicitToolGeneration(lowerRequest) ||
		looksBrokeredWorkflowGeneration(lowerRequest) ||
		looksConnectorCapability(lowerRequest) ||
		looksWorkflowCapability(lowerRequest) ||
		looksSkillCapability(lowerRequest) ||
		looksProtectedHighStakesAction(lowerRequest) ||
		looksCodeActionCapability(lowerRequest) ||
		looksProjectActionCapability(lowerRequest) ||
		looksOutputOrInfrastructureCapability(lowerRequest) {
		return true
	}
	if isInternetToolName(decision.ToolName) ||
		isInternetConfigurationIssue(decision, lowerReason) ||
		strings.Contains(lowerReason, "internet") {
		return true
	}
	return false
}

func isTemplateCapability(gap capabilities.GapDecision) bool {
	if gap.Capability == nil || gap.Capability.Metadata == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(gap.Capability.Metadata["template"]), "true")
}

func isPackKind(kind capabilities.Kind) bool {
	return kind == capabilities.KindDomainPack || kind == capabilities.KindCapabilityPack
}

func packKindLabel(kind capabilities.Kind) string {
	if kind == capabilities.KindCapabilityPack {
		return "capability pack"
	}
	return "domain pack"
}

func settingsCapabilityProposal(request string, decision ExecutionDecision, gap capabilities.GapDecision) CapabilityGapProposal {
	return CapabilityGapProposal{
		Needed:           true,
		Kind:             CapabilityKindSettings,
		Name:             nonEmpty(gap.Name, "model_settings"),
		Title:            "Configure the requested model setting",
		Description:      compactCapabilityDescription(request),
		Reason:           firstNonEmpty(gap.Reason, decision.Reason, "the requested model or settings route is not ready"),
		SuggestedAction:  nonEmpty(gap.SuggestedAction, "Configure the model role explicitly, then retry the request."),
		CanGenerate:      false,
		RequiresApproval: true,
		Permissions:      []string{"settings=profile_local", "policy=core_enforced"},
	}
}

func annotateCapabilityProposal(proposal CapabilityGapProposal, gap capabilities.GapDecision) CapabilityGapProposal {
	if gap.Action == "" {
		return proposal
	}
	if isPackKind(gap.Kind) && proposal.Kind != string(gap.Kind) {
		return proposal
	}
	proposal.CapabilitySummary = gap.UserSummary()
	proposal.CapabilityAction = string(gap.Action)
	proposal.CapabilityState = string(gap.State)
	if gap.Capability != nil {
		capability := gap.Capability
		proposal.RuntimeStatus = capabilityRuntimeStatus(*capability)
		if proposal.ConfigureHint == "" {
			proposal.ConfigureHint = strings.TrimSpace(capability.ConfigureHint)
		}
		if proposal.PackSource == "" && isPackKind(capability.Kind) {
			proposal.PackSource = strings.TrimSpace(capability.Source)
		}
		if proposal.PackDir == "" && capability.Metadata != nil {
			proposal.PackDir = firstNonEmpty(capability.Metadata["dir"], capability.Metadata["template_path"])
		}
		if proposal.InstallCommand == "" && isPackKind(capability.Kind) {
			proposal.InstallCommand = domainPackSetupCommand(gap, *capability)
		}
	}
	if proposal.ExistingCapability == "" && (gap.Action == capabilities.ActionUseExisting || gap.Action == capabilities.ActionUsePack) {
		proposal.ExistingCapability = gap.Name
	}
	if proposal.Reason == "" {
		proposal.Reason = gap.Reason
	}
	return proposal
}

func capabilityRuntimeStatus(capability capabilities.Capability) *CapabilityRuntimeStatus {
	metadata := capability.Metadata
	requiredTools := metadataList(metadata, "required_tools")
	optionalTools := metadataList(metadata, "optional_tools")
	riskyTools := riskyCapabilityTools(append(append([]string{}, requiredTools...), optionalTools...))
	status := &CapabilityRuntimeStatus{
		Source:           strings.TrimSpace(capability.Source),
		State:            string(capability.State),
		Installed:        metadataBoolDefault(metadata, "installed", capability.Available),
		Enabled:          capability.Enabled,
		Valid:            metadataBoolDefault(metadata, "valid", capability.Available),
		RequiresApproval: capability.RequiresApproval,
		Reason:           strings.TrimSpace(capability.Reason),
		ConfigureHint:    strings.TrimSpace(capability.ConfigureHint),
		RepairHint:       firstNonEmpty(metadataValue(metadata, "repair_hint"), metadataValue(metadata, "install_hint")),
		RequiredTools:    requiredTools,
		OptionalTools:    optionalTools,
		Skills:           metadataList(metadata, "skills"),
		RiskyTools:       riskyTools,
	}
	if len(riskyTools) > 0 {
		status.RequiresApproval = true
		status.RiskReason = "pack declares tools that require approval, are mutating, disabled by default, or unknown to the typed tool catalog"
	}
	return status
}

func riskyCapabilityTools(names []string) []string {
	catalog := tools.ToolCapabilitiesByName()
	seen := map[string]bool{}
	out := []string{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		capability, ok := catalog[name]
		if !ok || capability.RequiresApproval || capability.Mutating || !capability.EnabledByDefault {
			out = append(out, name)
		}
	}
	return out
}

func metadataList(metadata map[string]string, key string) []string {
	value := metadataValue(metadata, key)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := []string{}
	seen := map[string]bool{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || seen[part] {
			continue
		}
		seen[part] = true
		out = append(out, part)
	}
	return out
}

func metadataBoolDefault(metadata map[string]string, key string, fallback bool) bool {
	value := strings.ToLower(metadataValue(metadata, key))
	switch value {
	case "true", "yes", "1":
		return true
	case "false", "no", "0":
		return false
	default:
		return fallback
	}
}

func metadataValue(metadata map[string]string, key string) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(metadata[key])
}

func domainPackSetupCommand(gap capabilities.GapDecision, capability capabilities.Capability) string {
	name := strings.TrimSpace(firstNonEmpty(gap.Name, capability.Name))
	if name == "" {
		return ""
	}
	if isTemplateCapability(gap) {
		return "yemaka domain-pack install-template " + name
	}
	switch gap.State {
	case capabilities.StateDisabled:
		return "yemaka domain-pack enable " + name
	case capabilities.StateUnavailable:
		if capability.Metadata != nil {
			if dir := strings.TrimSpace(capability.Metadata["dir"]); dir != "" {
				return "yemaka domain-pack install " + dir
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func (r *CapabilityGapRouter) Generate(ctx context.Context, generator ExtensionGenerator, input CapabilityGenerationInput) (CapabilityGenerationResult, error) {
	if r == nil {
		return CapabilityGenerationResult{}, fmt.Errorf("capability gap router is nil")
	}
	if generator == nil {
		if typed, ok := r.Extensions.(ExtensionGenerator); ok {
			generator = typed
		}
	}
	if generator == nil {
		return CapabilityGenerationResult{}, fmt.Errorf("extension generator is required")
	}
	request := strings.TrimSpace(input.Request)
	if request == "" {
		return CapabilityGenerationResult{}, fmt.Errorf("capability request is required")
	}
	if !input.Approved {
		return CapabilityGenerationResult{}, fmt.Errorf("use --yes to approve local capability generation")
	}
	proposal, err := r.ProposeRequest(request)
	if err != nil {
		return CapabilityGenerationResult{}, err
	}
	if r.ExtensionsEnabled {
		proposal = mergeCapabilityGenerationAllowedDomains(proposal, input.AllowedDomains)
	}
	if !proposal.Needed {
		return CapabilityGenerationResult{Proposal: proposal, Message: "no missing capability detected"}, fmt.Errorf("no missing capability detected")
	}
	if !proposal.CanGenerate {
		return CapabilityGenerationResult{Proposal: proposal}, fmt.Errorf("capability cannot be generated yet: %s", proposal.SuggestedAction)
	}
	if proposal.Kind != CapabilityKindToolExtension && proposal.Kind != CapabilityKindWorkflow {
		return CapabilityGenerationResult{Proposal: proposal}, fmt.Errorf("capability kind %s is not supported by the generator handoff", proposal.Kind)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = proposal.Name
	}
	description := proposal.Description
	if proposal.Extension != nil && strings.TrimSpace(proposal.Extension.Description) != "" {
		description = proposal.Extension.Description
	}
	draft := input.Draft
	if input.SynthesizeDraft {
		if draft != nil {
			return CapabilityGenerationResult{Proposal: proposal}, fmt.Errorf("--synthesize cannot be combined with a provided draft")
		}
		var err error
		draft, err = DraftCapabilityExtension(ctx, CapabilityDraftRequest{
			Request:        request,
			Name:           name,
			Description:    description,
			Proposal:       proposal,
			Model:          input.DraftModel,
			RuntimeFactory: input.DraftRuntimeFactory,
		})
		if err != nil {
			return CapabilityGenerationResult{Proposal: proposal}, err
		}
	}
	result, repairs, err := r.generateWithOptionalDraftRepair(ctx, generator, proposal, input, name, description, draft)
	if err != nil {
		return CapabilityGenerationResult{Proposal: proposal, Generation: result}, err
	}
	output := CapabilityGenerationResult{
		Proposal:   proposal,
		Generation: result,
		Repairs:    repairs,
		NextSteps:  capabilityGenerationNextSteps(proposal, result),
		Message:    "capability generated, tested, and registered after explicit approval",
	}
	if repairs > 0 {
		output.Message = fmt.Sprintf("capability generated, repaired %d time(s), tested, and registered after explicit approval", repairs)
		output.NextSteps = append([]string{"Review the repaired generated files with `yemaka extension inspect " + name + "` before relying on the capability for important work."}, output.NextSteps...)
	}
	if input.RunAfterGenerate {
		runResult := runGeneratedCapability(ctx, generator, name, proposal, input)
		output.Run = &runResult
		if runResult.Status == "completed" {
			if repairs > 0 {
				output.Message = fmt.Sprintf("capability generated, repaired %d time(s), tested, registered, and run after explicit approval", repairs)
			} else {
				output.Message = "capability generated, tested, registered, and run after explicit approval"
			}
			output.NextSteps = append([]string{"Review the run output and rerun with `yemaka extension run " + name + " --input-json '{...}'` if needed."}, output.NextSteps...)
		} else {
			if repairs > 0 {
				output.Message = fmt.Sprintf("capability generated, repaired %d time(s), tested, and registered; the approved first run needs attention", repairs)
			} else {
				output.Message = "capability generated, tested, and registered; the approved first run needs attention"
			}
			output.NextSteps = append([]string{"Review the run error and rerun with explicit JSON input if needed."}, output.NextSteps...)
		}
	}
	return output, nil
}

func mergeCapabilityGenerationAllowedDomains(proposal CapabilityGapProposal, allowedDomains []string) CapabilityGapProposal {
	if !strings.EqualFold(strings.TrimSpace(proposal.NetworkMode), "core_broker") {
		return proposal
	}
	domains := extensions.NormalizeAllowedDomains(append(append([]string{}, proposal.AllowedDomains...), allowedDomains...))
	if len(domains) == 0 {
		return proposal
	}
	proposal.AllowedDomains = domains
	proposal.Permissions = appendCapabilityPermissions(proposal.Permissions, "network.domains="+strings.Join(domains, ","))
	if proposal.Extension != nil {
		proposal.Extension.NetworkMode = "core_broker"
		proposal.Extension.AllowedDomains = append([]string{}, domains...)
		proposal.Extension.Permissions = appendCapabilityPermissions(filteredProposalPermissions(proposal.Extension.Permissions, true), "network.domains="+strings.Join(domains, ","))
	}
	if proposal.GenerationCommand == "" || strings.Contains(proposal.SuggestedAction, "Provide the exact public URL or allowed domain") {
		description := strings.TrimSpace(proposal.Description)
		if proposal.Extension != nil && strings.TrimSpace(proposal.Extension.Description) != "" {
			description = proposal.Extension.Description
		}
		proposal.GenerationCommand = extensionGenerateCommand(extensions.Proposal{
			Name:           proposal.Name,
			Description:    description,
			NetworkMode:    "core_broker",
			AllowedDomains: domains,
		})
	}
	if strings.Contains(proposal.SuggestedAction, "Provide the exact public URL or allowed domain") {
		proposal.CanGenerate = true
		proposal.SuggestedAction = "Review the approved domain scope, then approve the profile-local generated capability. Brokered internet remains limited to the listed domains."
	}
	return proposal
}

func (r *CapabilityGapRouter) generateWithOptionalDraftRepair(ctx context.Context, generator ExtensionGenerator, proposal CapabilityGapProposal, input CapabilityGenerationInput, name string, description string, draft *extensions.PackageDraft) (extensions.GenerationResult, int, error) {
	repairsAllowed := normalizedDraftRepairAttempts(input)
	var result extensions.GenerationResult
	var err error
	repairs := 0
	for {
		result, err = generator.Generate(ctx, extensions.GenerateInput{
			Name:              name,
			Description:       description,
			Approved:          true,
			MaxRuntimeSeconds: input.MaxRuntimeSeconds,
			BrokeredNetwork:   proposal.NetworkMode == "core_broker",
			AllowedDomains:    append([]string{}, proposal.AllowedDomains...),
			Draft:             draft,
			TestTimeout:       input.TestTimeout,
		})
		if err == nil {
			return result, repairs, nil
		}
		if !shouldRepairDraftGeneration(input, draft, repairs, repairsAllowed, err) {
			return result, repairs, err
		}
		repairedDraft, repairErr := RepairCapabilityExtensionDraft(ctx, CapabilityDraftRepairRequest{
			CapabilityDraftRequest: CapabilityDraftRequest{
				Request:        input.Request,
				Name:           name,
				Description:    description,
				Proposal:       proposal,
				Model:          input.DraftModel,
				RuntimeFactory: input.DraftRuntimeFactory,
			},
			PreviousDraft: draft,
			Failure:       capabilityGenerationFailureSummary(result, err),
			Attempt:       repairs + 1,
		})
		if repairErr != nil {
			return result, repairs, fmt.Errorf("generation failed and synthesized draft repair failed: %v; repair: %w", err, repairErr)
		}
		draft = repairedDraft
		repairs++
	}
}

func normalizedDraftRepairAttempts(input CapabilityGenerationInput) int {
	if !input.SynthesizeDraft {
		return 0
	}
	if input.DraftRepairAttempts < 0 {
		return 0
	}
	attempts := input.DraftRepairAttempts
	if attempts == 0 {
		attempts = defaultSynthesizedDraftRepairAttempts
	}
	if attempts > maxSynthesizedDraftRepairAttempts {
		return maxSynthesizedDraftRepairAttempts
	}
	return attempts
}

func shouldRepairDraftGeneration(input CapabilityGenerationInput, draft *extensions.PackageDraft, repairs int, repairsAllowed int, err error) bool {
	if err == nil || draft == nil || !input.SynthesizeDraft || repairs >= repairsAllowed {
		return false
	}
	message := strings.ToLower(err.Error())
	if strings.Contains(message, "already exists") ||
		strings.Contains(message, "approval") ||
		strings.Contains(message, "user approval") {
		return false
	}
	return true
}

func capabilityGenerationFailureSummary(result extensions.GenerationResult, err error) string {
	var builder strings.Builder
	if err != nil {
		fmt.Fprintf(&builder, "error: %s\n", err.Error())
	}
	if result.Tests.Status != "" {
		fmt.Fprintf(&builder, "test status: %s\n", result.Tests.Status)
	}
	if len(result.Tests.Commands) > 0 {
		fmt.Fprintf(&builder, "test commands: %s\n", strings.Join(result.Tests.Commands, "; "))
	}
	if strings.TrimSpace(result.Tests.Output) != "" {
		fmt.Fprintf(&builder, "test output:\n%s\n", strings.TrimSpace(result.Tests.Output))
	}
	if result.Snapshot.ID != "" {
		fmt.Fprintf(&builder, "snapshot: %s\n", result.Snapshot.ID)
	}
	return strings.TrimSpace(builder.String())
}

func runGeneratedCapability(ctx context.Context, generator ExtensionGenerator, name string, proposal CapabilityGapProposal, input CapabilityGenerationInput) extensions.RunResult {
	runner, ok := generator.(ExtensionRunner)
	if !ok {
		return failedCapabilityRun(name, fmt.Errorf("extension runner is required for --run"))
	}
	options := input.RunOptions
	options.Input = capabilityFirstRunInput(proposal, input)
	if options.MaxRuntimeSeconds <= 0 {
		options.MaxRuntimeSeconds = input.MaxRuntimeSeconds
	}
	result, err := runner.Run(ctx, name, options)
	if err != nil {
		if result.Extension != "" || result.RunID != "" || result.Status != "" {
			if result.Status == "" {
				result.Status = "failed"
			}
			if result.Error == "" {
				result.Error = err.Error()
			}
			return result
		}
		return failedCapabilityRun(name, err)
	}
	return result
}

func failedCapabilityRun(name string, err error) extensions.RunResult {
	message := ""
	if err != nil {
		message = err.Error()
	}
	return extensions.RunResult{
		Extension: name,
		Status:    "failed",
		Error:     message,
	}
}

func capabilityFirstRunInput(proposal CapabilityGapProposal, input CapabilityGenerationInput) map[string]any {
	if input.RunInput != nil {
		return input.RunInput
	}
	return SuggestedCapabilityInput(proposal, input.Request)
}

func SuggestedCapabilityInput(proposal CapabilityGapProposal, request string) map[string]any {
	runInput := map[string]any{
		"task": compactCapabilityDescription(firstNonEmpty(proposal.Description, request)),
	}
	if strings.EqualFold(strings.TrimSpace(proposal.NetworkMode), "core_broker") {
		if target := firstCapabilityInternetTarget(proposal); target != "" {
			runInput["url"] = target
		}
		runInput["extractText"] = true
	}
	return runInput
}

func firstCapabilityInternetTarget(proposal CapabilityGapProposal) string {
	for _, field := range strings.Fields(proposal.Description + " " + proposal.Reason) {
		if target, ok := normalizeInternetTarget(field); ok {
			return target
		}
	}
	for _, domain := range proposal.AllowedDomains {
		if target, ok := normalizeInternetTarget(domain); ok {
			return target
		}
	}
	return ""
}

func isCapabilityGapDecision(decision ExecutionDecision) bool {
	switch decision.Status {
	case ExecutionBlocked:
		return true
	case ExecutionReady:
		return strings.TrimSpace(decision.ToolName) != ""
	default:
		return false
	}
}

func isInternetConfigurationIssue(decision ExecutionDecision, reason string) bool {
	lower := strings.ToLower(strings.TrimSpace(strings.Join([]string{decision.ToolName, reason}, " ")))
	if isInternetToolName(decision.ToolName) {
		return true
	}
	terms := []string{
		"internet search provider",
		"internet.search.provider",
		"search provider",
		"searxng search requires",
		"brave search requires",
		"could not reach searxng",
		"context deadline exceeded",
		"client.timeout exceeded",
		"connection refused",
		"no such host",
		"provider returned http",
		"internet access is disabled",
		"not profile-approved",
		"controlled internet",
	}
	for _, term := range terms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func (r *CapabilityGapRouter) internetProposal(decision ExecutionDecision, reason string) CapabilityGapProposal {
	if !r.InternetEnabled || !r.InternetProfileApproved {
		return CapabilityGapProposal{
			Needed:           true,
			Kind:             CapabilityKindSettings,
			Name:             "controlled_internet_access",
			Title:            "Enable controlled internet access",
			Description:      "Yemaka has core internet tools, but they are disabled or not profile-approved for this session.",
			Reason:           nonEmpty(decision.Reason, "controlled internet access is disabled by policy"),
			SuggestedAction:  "Enable internet access from Settings, or run `yemaka internet enable`. Generated extensions must still use the core internet service.",
			CanGenerate:      false,
			RequiresApproval: true,
			Permissions:      []string{"network=profile_enabled", "methods=GET,HEAD", "policy=core_enforced"},
		}
	}
	if decision.ToolName == "internet_search" || strings.Contains(reason, "search") || !r.InternetSearchEnabled {
		provider := r.InternetSearchProvider
		if provider == "" {
			provider = "none"
		}
		return CapabilityGapProposal{
			Needed:           true,
			Kind:             CapabilityKindSettings,
			Name:             "internet_search_provider",
			Title:            "Configure a search provider",
			Description:      "Yemaka can fetch public URLs, but search requires an explicit local profile search provider.",
			Reason:           nonEmpty(decision.Reason, "internet_search needs a configured provider"),
			SuggestedAction:  "Configure `internet.search.provider` and its endpoint/API key env, then retry the same request.",
			CanGenerate:      false,
			RequiresApproval: true,
			Permissions:      []string{"network=profile_enabled", "search_provider=" + provider, "policy=core_enforced"},
		}
	}
	return CapabilityGapProposal{
		Needed:           true,
		Kind:             CapabilityKindSettings,
		Name:             "internet_request_scope",
		Title:            "Approve a scoped internet request",
		Description:      "The request needs a URL/domain or task-scoped approval before Yemaka can fetch public content.",
		Reason:           decision.Reason,
		SuggestedAction:  "Provide an explicit public http(s) URL or allowed domain, then approve the controlled fetch.",
		CanGenerate:      false,
		RequiresApproval: true,
		Permissions:      []string{"network=task_scoped", "methods=GET,HEAD", "policy=core_enforced"},
	}
}

func (r *CapabilityGapRouter) toolExtensionProposal(request string, decision ExecutionDecision) CapabilityGapProposal {
	proposal, ok := r.extensionProposal(request, extensions.TypeTool)
	networkRequired := looksBrokeredNetworkExtensionRequest(strings.ToLower(request))
	result := CapabilityGapProposal{
		Needed:           true,
		Kind:             CapabilityKindToolExtension,
		Title:            "Generate a small tool extension",
		Description:      capabilityProposalDescription(request),
		Reason:           nonEmpty(decision.Reason, "no built-in tool covers this capability"),
		SuggestedAction:  "Review the proposal in Extensions, then approve generation. CLI: `yemaka extension generate <name> \"<description>\" --yes`.",
		CanGenerate:      ok && r.ExtensionsEnabled,
		RequiresApproval: true,
	}
	if !r.ExtensionsEnabled {
		result.SuggestedAction = "Enable generated extensions first, then review and approve a profile-local tool extension."
	}
	if networkRequired {
		result.Permissions = appendCapabilityPermissions(result.Permissions, "internet=core_broker", "network=task_scoped", "methods=GET,HEAD")
		result.NetworkMode = "core_broker"
	}
	if ok {
		result.Name = proposal.Name
		result.Files = proposal.Files
		result.Permissions = appendCapabilityPermissions(result.Permissions, filteredProposalPermissions(proposal.Permissions, networkRequired)...)
		result.GenerationCommand = extensionGenerateCommand(proposal)
		if proposal.NetworkMode != "" {
			result.NetworkMode = proposal.NetworkMode
		}
		result.AllowedDomains = append([]string{}, proposal.AllowedDomains...)
		if result.GenerationCommand != "" {
			result.SuggestedAction = "Review the proposal, then approve the profile-local generated tool. CLI: `" + result.GenerationCommand + "`."
		}
		result.Extension = &proposal
	}
	if networkRequired && len(result.AllowedDomains) == 0 {
		result.CanGenerate = false
		result.GenerationCommand = ""
		result.SuggestedAction = "Provide the exact public URL or allowed domain this tool may access, then approve generation. Yemaka will broker internet access through the core policy layer."
	}
	return result
}

func (r *CapabilityGapRouter) connectorProposal(request string, decision ExecutionDecision) CapabilityGapProposal {
	proposal, ok := r.extensionProposal(request, extensions.TypeConnector)
	result := CapabilityGapProposal{
		Needed:           true,
		Kind:             CapabilityKindConnector,
		Title:            "Connector capability required",
		Description:      capabilityProposalDescription(request),
		Reason:           nonEmpty(decision.Reason, "the requested external service is not enabled as a connector"),
		SuggestedAction:  "Enable an existing connector if available, or approve a generated connector package once connector generation is enabled for this service.",
		CanGenerate:      false,
		RequiresApproval: true,
		Permissions:      []string{"network=connector_scoped", "secrets=keychain_or_env_ref", "posting=confirm"},
	}
	if !r.ConnectorsEnabled {
		result.SuggestedAction = "Enable connectors only for the specific service you need, then configure the required token or generated connector."
	}
	if ok {
		result.Name = proposal.Name
		result.Files = proposal.Files
		result.Extension = &proposal
	}
	return result
}

func (r *CapabilityGapRouter) workflowProposal(request string, decision ExecutionDecision) CapabilityGapProposal {
	proposal, ok := r.extensionProposal(request, extensions.TypeTool)
	networkRequired := looksBrokeredNetworkExtensionRequest(strings.ToLower(request))
	result := CapabilityGapProposal{
		Needed:           true,
		Kind:             CapabilityKindWorkflow,
		Title:            "Create a workflow from a generated tool",
		Description:      capabilityProposalDescription(request),
		Reason:           nonEmpty(decision.Reason, "the request needs repeatable automation that is not in the current arsenal"),
		SuggestedAction:  "Generate the smallest tool extension first; after it passes tests, attach it to a scheduler job with explicit approval.",
		CanGenerate:      ok && r.ExtensionsEnabled,
		RequiresApproval: true,
		Permissions:      []string{"scheduler=approval_required", "max_parallel_jobs=1", "policy=core_enforced"},
	}
	if networkRequired {
		result.Permissions = appendCapabilityPermissions(result.Permissions, "internet=core_broker", "network=task_scoped", "methods=GET,HEAD")
		result.NetworkMode = "core_broker"
	}
	if ok {
		result.Name = proposal.Name
		result.Files = proposal.Files
		result.Permissions = appendCapabilityPermissions(result.Permissions, filteredProposalPermissions(proposal.Permissions, networkRequired)...)
		result.GenerationCommand = extensionGenerateCommand(proposal)
		if proposal.NetworkMode != "" {
			result.NetworkMode = proposal.NetworkMode
		}
		result.AllowedDomains = append([]string{}, proposal.AllowedDomains...)
		if result.GenerationCommand != "" {
			result.SuggestedAction = "Generate the smallest tool extension first, then attach it to a scheduler job after tests pass. CLI: `" + result.GenerationCommand + "`."
		}
		result.Extension = &proposal
	}
	if networkRequired && result.NetworkMode == "core_broker" && result.GenerationCommand != "" {
		result.SuggestedAction = "Generate the smallest tool extension first. It must use Yemaka's core internet broker for approved GET/HEAD requests; create the scheduler job only after the generated tool passes tests and the job is explicitly approved."
	}
	if networkRequired && len(result.AllowedDomains) == 0 {
		result.CanGenerate = false
		result.GenerationCommand = ""
		result.SuggestedAction = "Provide the exact public URL or allowed domain to monitor, then approve generation. The generated tool must use Yemaka's core internet broker, and no scheduler job is enabled until you explicitly approve it after tests pass."
	}
	return result
}

func (r *CapabilityGapRouter) skillProposal(request string, decision ExecutionDecision) CapabilityGapProposal {
	return CapabilityGapProposal{
		Needed:             true,
		Kind:               CapabilityKindSkill,
		Name:               "reusable_skill_candidate",
		Title:              "Save a reusable skill after this workflow",
		Description:        compactCapabilityDescription(request),
		Reason:             nonEmpty(decision.Reason, "this looks like a repeated workflow, but no matching skill is active"),
		SuggestedAction:    "Complete the task once with available tools, then run `yemaka skill create-from-session <conversation_id>` to save the reusable workflow.",
		CanGenerate:        false,
		RequiresApproval:   true,
		ExistingCapability: "skill create-from-session",
		Permissions:        []string{"skills=profile_local", "approval=required"},
	}
}

func (r *CapabilityGapRouter) extensionProposal(request string, typ string) (extensions.Proposal, bool) {
	if r == nil || r.Extensions == nil || strings.TrimSpace(request) == "" {
		return extensions.Proposal{}, false
	}
	proposal, err := r.Extensions.Propose(request)
	if err != nil {
		return extensions.Proposal{}, false
	}
	proposal.Type = typ
	return proposal, true
}

func (p CapabilityGapProposal) Response() string {
	var builder strings.Builder
	if !p.Needed {
		fmt.Fprintf(&builder, "Yemaka already has an enabled capability for this.\n\n")
		if p.Title != "" {
			fmt.Fprintf(&builder, "**Selected capability:** %s\n", strings.TrimSpace(p.Title))
		}
		if p.Kind != "" {
			fmt.Fprintf(&builder, "**Type:** %s\n", capabilityKindDisplay(p.Kind))
		}
		if p.ExistingCapability != "" {
			fmt.Fprintf(&builder, "**Using:** `%s`\n", p.ExistingCapability)
		} else if p.Name != "" {
			fmt.Fprintf(&builder, "**Using:** `%s`\n", p.Name)
		}
		if p.Description != "" {
			fmt.Fprintf(&builder, "**What it covers:** %s\n", p.Description)
		}
		if p.Reason != "" {
			fmt.Fprintf(&builder, "**Why:** %s\n", p.Reason)
		}
		if summary := runtimeStatusSummary(p.RuntimeStatus); summary != "" {
			fmt.Fprintf(&builder, "**Capability status:** %s\n", summary)
		}
		if len(p.Permissions) > 0 {
			fmt.Fprintf(&builder, "**Policy:** %s\n", strings.Join(p.Permissions, ", "))
		}
		if action := capabilityChatSuggestedAction(p.SuggestedAction); action != "" {
			fmt.Fprintf(&builder, "\n**Next step:** %s", action)
		}
		return strings.TrimSpace(builder.String())
	}
	if p.Kind == CapabilityKindSettings {
		fmt.Fprintf(&builder, "I can help with this, but one setting needs to be ready first.\n\n")
	} else if p.CanGenerate {
		fmt.Fprintf(&builder, "I can help with this by adding the smallest safe capability to the profile.\n\n")
	} else {
		fmt.Fprintf(&builder, "I can help with this, but it needs one safe capability step before I run it.\n\n")
	}
	fmt.Fprintf(&builder, "**Proposed next capability:** %s\n", strings.TrimSpace(p.Title))
	if p.Kind != "" {
		fmt.Fprintf(&builder, "**Type:** %s\n", capabilityKindDisplay(p.Kind))
	}
	if p.Description != "" {
		fmt.Fprintf(&builder, "**What it covers:** %s\n", p.Description)
	}
	if p.Reason != "" {
		fmt.Fprintf(&builder, "**Why I paused:** %s\n", p.Reason)
	}
	if p.Name != "" {
		fmt.Fprintf(&builder, "**Suggested name:** `%s`\n", p.Name)
	}
	if summary := runtimeStatusSummary(p.RuntimeStatus); summary != "" {
		fmt.Fprintf(&builder, "**Capability status:** %s\n", summary)
	}
	if safety := capabilitySafetySummary(p); safety != "" {
		fmt.Fprintf(&builder, "**Safety:** %s\n", safety)
	}
	if len(p.AllowedDomains) > 0 {
		fmt.Fprintf(&builder, "**Allowed domains:** %s\n", strings.Join(p.AllowedDomains, ", "))
	}
	if action := capabilityChatSuggestedAction(p.SuggestedAction); action != "" {
		fmt.Fprintf(&builder, "\n**Next step:** %s", action)
	}
	if p.RequiresApproval {
		fmt.Fprintf(&builder, "\n\nI will not create or enable this capability without your approval.")
	}
	return strings.TrimSpace(builder.String())
}

func runtimeStatusSummary(status *CapabilityRuntimeStatus) string {
	if status == nil {
		return ""
	}
	parts := []string{}
	if status.State != "" {
		parts = append(parts, capabilityKindDisplay(status.State))
	}
	if status.Source != "" {
		parts = append(parts, capabilityKindDisplay(status.Source))
	}
	if status.Installed {
		parts = append(parts, "installed")
	} else {
		parts = append(parts, "not installed")
	}
	if status.Enabled {
		parts = append(parts, "enabled")
	} else {
		parts = append(parts, "disabled")
	}
	if status.Valid {
		parts = append(parts, "valid")
	} else {
		parts = append(parts, "invalid")
	}
	if status.RequiresApproval {
		parts = append(parts, "approval required")
	}
	if len(status.RiskyTools) > 0 {
		parts = append(parts, "approval-gated tools: "+strings.Join(status.RiskyTools, ", "))
	}
	return strings.Join(parts, ", ")
}

func capabilityKindDisplay(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	words := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '-' || r == '.'
	})
	if len(words) == 0 {
		return value
	}
	for i, word := range words {
		lower := strings.ToLower(word)
		switch lower {
		case "ai", "api", "cli", "get", "head", "http", "https", "json", "rag", "ui", "url":
			words[i] = strings.ToUpper(lower)
		default:
			words[i] = lower
		}
	}
	return strings.Join(words, " ")
}

func capabilityChatSuggestedAction(action string) string {
	action = strings.TrimSpace(action)
	if action == "" {
		return ""
	}
	for _, marker := range []string{" CLI:", " CLI `", " Command:"} {
		if idx := strings.Index(action, marker); idx > 0 {
			action = strings.TrimSpace(action[:idx])
		}
	}
	return strings.TrimSpace(action)
}

func capabilitySafetySummary(proposal CapabilityGapProposal) string {
	permissions := append([]string{}, proposal.Permissions...)
	if proposal.NetworkMode != "" {
		permissions = append(permissions, "network_mode="+proposal.NetworkMode)
	}
	seen := map[string]bool{}
	parts := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		parts = append(parts, value)
	}
	for _, permission := range permissions {
		switch strings.TrimSpace(permission) {
		case "scheduler=approval_required":
			add("scheduler jobs require approval")
		case "policy=core_enforced":
			add("core policy enforced")
		case "internet=core_broker", "network=core_broker", "network_mode=core_broker":
			add("internet uses the core broker")
		case "network=task_scoped":
			add("network is task scoped")
		case "network=profile_enabled":
			add("network requires profile enablement")
		case "methods=GET,HEAD", "network.methods=GET,HEAD":
			add("GET/HEAD only")
		case "filesystem.read=false":
			add("no file read")
		case "filesystem.write=false":
			add("no file write")
		case "shell=false":
			add("no shell")
		case "secrets=false":
			add("no secrets")
		case "skills=profile_local", "settings=profile_local":
			add("profile local")
		}
		if strings.HasPrefix(permission, "max_parallel_jobs=") {
			add("max parallel jobs " + strings.TrimPrefix(permission, "max_parallel_jobs="))
		}
	}
	if proposal.RuntimeStatus != nil && proposal.RuntimeStatus.RequiresApproval {
		add("approval required")
	}
	if len(parts) == 0 {
		return ""
	}
	if len(parts) > 6 {
		parts = parts[:6]
	}
	return strings.Join(parts, "; ")
}

func emitCapabilityGap(emit EventHandler, proposal CapabilityGapProposal, request string) error {
	if emit == nil || !proposal.Needed {
		return nil
	}
	request = strings.TrimSpace(request)
	data := map[string]string{
		"kind":                proposal.Kind,
		"name":                proposal.Name,
		"title":               proposal.Title,
		"request":             request,
		"description":         proposal.Description,
		"reason":              proposal.Reason,
		"suggested_action":    proposal.SuggestedAction,
		"can_generate":        fmt.Sprintf("%t", proposal.CanGenerate),
		"requires_approval":   fmt.Sprintf("%t", proposal.RequiresApproval),
		"files":               strings.Join(proposal.Files, ", "),
		"permissions":         strings.Join(proposal.Permissions, ", "),
		"generation_command":  proposal.GenerationCommand,
		"network_mode":        proposal.NetworkMode,
		"allowed_domains":     strings.Join(proposal.AllowedDomains, ", "),
		"existing_capability": proposal.ExistingCapability,
		"capability_summary":  proposal.CapabilitySummary,
		"capability_action":   proposal.CapabilityAction,
		"capability_state":    proposal.CapabilityState,
		"configure_hint":      proposal.ConfigureHint,
		"pack_source":         proposal.PackSource,
		"pack_dir":            proposal.PackDir,
		"install_command":     proposal.InstallCommand,
	}
	if status := proposal.RuntimeStatus; status != nil {
		data["runtime_source"] = status.Source
		data["runtime_state"] = status.State
		data["runtime_installed"] = fmt.Sprintf("%t", status.Installed)
		data["runtime_enabled"] = fmt.Sprintf("%t", status.Enabled)
		data["runtime_valid"] = fmt.Sprintf("%t", status.Valid)
		data["runtime_requires_approval"] = fmt.Sprintf("%t", status.RequiresApproval)
		data["runtime_reason"] = status.Reason
		data["runtime_configure_hint"] = status.ConfigureHint
		data["runtime_repair_hint"] = status.RepairHint
		data["runtime_required_tools"] = strings.Join(status.RequiredTools, ", ")
		data["runtime_optional_tools"] = strings.Join(status.OptionalTools, ", ")
		data["runtime_skills"] = strings.Join(status.Skills, ", ")
		data["runtime_risky_tools"] = strings.Join(status.RiskyTools, ", ")
		data["runtime_risk_reason"] = status.RiskReason
	}
	return emit(Event{Type: EventCapabilityGap, Message: proposal.Title, Data: data})
}

func looksExplicitToolGeneration(content string) bool {
	terms := []string{
		"build a tool",
		"generate a tool",
		"create a tool",
		"reusable tool",
		"create extension",
		"create an extension",
		"propose extension",
		"propose the extension",
		"new extension",
		"generate extension",
		"generate it until i approve",
		"missing capability",
		"does not already have this capability",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func looksBrokeredWorkflowGeneration(content string) bool {
	if !looksWorkflowCapability(content) {
		return false
	}
	return looksBrokeredNetworkExtensionRequest(content) ||
		strings.Contains(content, "http://") ||
		strings.Contains(content, "https://") ||
		strings.Contains(content, ".com") ||
		strings.Contains(content, ".org") ||
		strings.Contains(content, ".net") ||
		strings.Contains(content, ".io") ||
		strings.Contains(content, ".dev") ||
		strings.Contains(content, ".ai") ||
		strings.Contains(content, ".online") ||
		strings.Contains(content, "website") ||
		strings.Contains(content, "webpage") ||
		strings.Contains(content, "web page") ||
		strings.Contains(content, "page title") ||
		strings.Contains(content, "url") ||
		strings.Contains(content, "internet") ||
		strings.Contains(content, "online") ||
		strings.Contains(content, "site")
}

func looksBrokeredNetworkExtensionRequest(content string) bool {
	return strings.Contains(content, "http://") ||
		strings.Contains(content, "https://") ||
		strings.Contains(content, ".com") ||
		strings.Contains(content, ".org") ||
		strings.Contains(content, ".net") ||
		strings.Contains(content, ".io") ||
		strings.Contains(content, ".dev") ||
		strings.Contains(content, ".ai") ||
		strings.Contains(content, ".online") ||
		strings.Contains(content, "website") ||
		strings.Contains(content, "webpage") ||
		strings.Contains(content, "web page") ||
		strings.Contains(content, "web site") ||
		strings.Contains(content, "page title") ||
		strings.Contains(content, "url") ||
		strings.Contains(content, "internet") ||
		strings.Contains(content, "online") ||
		strings.Contains(content, "site")
}

func isExplanationOnlyCapabilityRequest(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	explanationPrefixes := []string{
		"explain ",
		"describe ",
		"what is ",
		"what are ",
		"how does ",
		"how do ",
		"how would ",
		"why does ",
		"why do ",
		"teach me ",
	}
	matchesPrefix := false
	for _, prefix := range explanationPrefixes {
		if strings.HasPrefix(content, prefix) {
			matchesPrefix = true
			break
		}
	}
	if !matchesPrefix {
		return false
	}
	actionTerms := []string{
		"build a tool",
		"generate a tool",
		"create a tool",
		"create extension",
		"generate extension",
		"propose extension",
		"propose the extension",
		"missing capability",
		"does not already have this capability",
		"do not generate",
		"until i approve",
		"schedule it",
		"save as a scheduled job",
	}
	for _, term := range actionTerms {
		if strings.Contains(content, term) {
			return false
		}
	}
	return true
}

func appendCapabilityPermissions(base []string, extra ...string) []string {
	out := append([]string{}, base...)
	seen := make(map[string]bool, len(out)+len(extra))
	for _, permission := range out {
		seen[permission] = true
	}
	for _, permission := range extra {
		permission = strings.TrimSpace(permission)
		if permission == "" || seen[permission] {
			continue
		}
		seen[permission] = true
		out = append(out, permission)
	}
	return out
}

func filteredProposalPermissions(permissions []string, networkRequired bool) []string {
	out := make([]string, 0, len(permissions))
	for _, permission := range permissions {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if networkRequired && permission == "network=false" {
			continue
		}
		out = append(out, permission)
	}
	return out
}

func extensionGenerateCommand(proposal extensions.Proposal) string {
	if strings.TrimSpace(proposal.Name) == "" || strings.TrimSpace(proposal.Description) == "" {
		return ""
	}
	parts := []string{
		"yemaka",
		"extension",
		"generate",
		shellQuote(proposal.Name),
		shellQuote(proposal.Description),
		"--yes",
	}
	if strings.EqualFold(strings.TrimSpace(proposal.NetworkMode), "core_broker") {
		parts = append(parts, "--brokered-network")
		for _, domain := range proposal.AllowedDomains {
			domain = strings.TrimSpace(domain)
			if domain == "" {
				continue
			}
			parts = append(parts, "--allow-domain", shellQuote(domain))
		}
	}
	return strings.Join(parts, " ")
}

func capabilityGenerationNextSteps(proposal CapabilityGapProposal, result extensions.GenerationResult) []string {
	name := strings.TrimSpace(result.Extension.Name)
	if name == "" {
		name = strings.TrimSpace(proposal.Name)
	}
	if name == "" {
		return nil
	}
	steps := []string{
		"Inspect the generated package before relying on it: yemaka extension inspect " + shellQuote(name),
		"Run it with a scoped input: yemaka extension run " + shellQuote(name) + " '{\"task\":\"" + strings.ReplaceAll(compactCapabilityDescription(proposal.Description), `"`, `\"`) + "\"}'",
		"Rollback if it does not behave as expected: yemaka extension rollback " + shellQuote(name),
	}
	if proposal.NetworkMode == "core_broker" {
		steps = append(steps, "Brokered internet still obeys core policy; enable or approve controlled internet before running network fetches.")
	}
	if proposal.Kind == CapabilityKindWorkflow {
		every := workflowIntervalHint(proposal.Description)
		steps = append([]string{
			"Create the scheduler job only after reviewing the generated tool: yemaka job create interval " + shellQuote(name) + " --every " + every + " --target-type extension --yes",
		}, steps...)
	}
	return steps
}

func workflowIntervalHint(description string) string {
	lower := strings.ToLower(description)
	switch {
	case strings.Contains(lower, "every minute"):
		return "1m"
	case strings.Contains(lower, "every hour"), strings.Contains(lower, "hourly"):
		return "1h"
	case strings.Contains(lower, "weekly"), strings.Contains(lower, "every week"):
		return "168h"
	case strings.Contains(lower, "daily"), strings.Contains(lower, "every day"), strings.Contains(lower, "morning"):
		return "24h"
	default:
		return "1h"
	}
}

func shellQuote(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return `""`
	}
	if strings.IndexFunc(value, func(r rune) bool {
		return r == ' ' || r == '\t' || r == '\n' || r == '"' || r == '\'' || r == '`' || r == '$' || r == '\\'
	}) < 0 {
		return value
	}
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}

func looksConnectorCapability(content string) bool {
	if looksLocalCommunicationDraft(content) {
		return false
	}
	terms := []string{"slack", "telegram", "discord", "gmail", "outlook", "webhook", "connector", "post to x", "tweet", "social connector", "social account", "send to", "messaging", "notification", "publish to", "subscribe to", "api call", "external service", "integrate with", "third-party", "post message", "send email", "send notification", "recon", "integration", "install", "connect to", "export secrets", "export passwords", "change account", "book flight", "book a flight", "book hotel", "book a hotel", "book travel", "book a trip", "make reservation", "cancel reservation", "send itinerary", "post this", "post on", "publish this", "publish on", "send newsletter", "send a newsletter", "publish newsletter", "submit form", "submit this form", "submit application", "upload form", "upload this form", "upload application", "send form", "send this form", "email form", "mail form", "file application", "apply online"}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	if strings.HasPrefix(content, "email ") ||
		containsAnyPhrase(content, "send my", "send this", "send these", "send the", "send an email", "send email", "mail this", "mail these", "email my", "email this", "email these") {
		return true
	}
	return false
}

func looksLocalCommunicationDraft(content string) bool {
	content = strings.ToLower(strings.TrimSpace(content))
	if content == "" {
		return false
	}
	if containsAnyPhrase(content, "send ", "sent ", "mail this", "mail these", "email my", "email this", "email these", "post ", "publish ", "notify ", "schedule ", "connect ", "integrate ") {
		return false
	}
	return containsAnyPhrase(content,
		"draft an email",
		"draft a email",
		"draft email",
		"email draft",
		"reply draft",
		"write a reply",
		"write an email",
		"review this reply",
		"review my reply",
		"make this email",
		"polish this email",
	)
}

func looksWorkflowCapability(content string) bool {
	terms := []string{
		"schedule",
		"cron",
		"every hour",
		"every day",
		"daily",
		"weekly",
		"monitor",
		"when changed",
		"recurring",
		"background job",
		"reports if the title changes",
		"check a webpage",
		"checks a webpage",
	}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func looksProtectedHighStakesAction(content string) bool {
	terms := []string{
		"place trade",
		"place trades",
		"trade for me",
		"buy stock",
		"sell stock",
		"buy crypto",
		"sell crypto",
		"move money",
		"transfer money",
		"pay invoice",
		"pay this invoice",
		"make a payment",
		"file taxes",
		"claim this tax",
		"tax deduction",
		"sign this",
		"sign the",
		"file this court",
		"submit this court",
		"fill this form",
		"fill out this form",
		"sign form",
		"sign this form",
		"submit form",
		"submit this form",
		"file this form",
		"sue ",
		"terminate this contract",
		"apply for visa",
		"apply for a visa",
		"immigration advice",
		"passport application",
	}
	return containsAnyPhrase(content, terms...)
}

func looksOutputOrInfrastructureCapability(content string) bool {
	terms := []string{
		"write to a file",
		"write this to a file",
		"write these to a file",
		"create a file",
		"create folder",
		"create folders",
		"create a folder",
		"create directory",
		"create directories",
		"create new file",
		"new file",
		"update file",
		"move file",
		"move files",
		"move this file",
		"move these files",
		"rename file",
		"rename files",
		"rename this file",
		"rename these files",
		"rename this",
		"rename these",
		"delete file",
		"delete files",
		"delete this file",
		"delete these files",
		"delete old",
		"delete this",
		"delete these",
		"remove file",
		"remove files",
		"remove this",
		"remove these",
		"save as pdf",
		"generate pdf",
		"generate a pdf",
		"export",
		"sync this",
		"sync these",
		"sync my",
		"sync the",
		"sync to",
		"sync with",
		"build a database",
		"create a database",
		"build a sqlite",
		"create a sqlite",
		"knowledge graph",
		"vector database",
		"vector store",
		"embedding index",
		"train a model",
		"fine tune",
		"fine-tune",
	}
	return containsAnyPhrase(content, terms...)
}

func looksCodeActionCapability(content string) bool {
	terms := []string{
		"edit this code",
		"fix this code",
		"fix this bug",
		"patch this",
		"refactor this",
		"write code",
		"generate code",
		"create script",
		"write a script",
		"modify this file",
		"update this file",
		"change this file",
		"read this file",
		"open this file",
		"run tests",
		"run test",
		"go test",
		"npm test",
		"execute this",
		"run command",
		"run a command",
		"install dependency",
		"install package",
		"npm install",
		"go get",
		"git commit",
		"git diff",
		"apply patch",
		"compile",
		"build the app",
		"deploy",
	}
	return containsAnyPhrase(content, terms...)
}

func looksProjectActionCapability(content string) bool {
	terms := []string{
		"assign task",
		"assign tasks",
		"assign this task",
		"assign these tasks",
		"create ticket",
		"create tickets",
		"create issue",
		"create issues",
		"update project board",
		"update board",
		"update status board",
		"send status update",
		"send project update",
		"notify team",
		"notify the team",
		"schedule deadline",
		"schedule deadlines",
		"set deadline",
		"set deadlines",
		"save project plan",
		"write project plan",
	}
	return containsAnyPhrase(content, terms...)
}

func containsAnyPhrase(content string, phrases ...string) bool {
	for _, phrase := range phrases {
		if strings.Contains(content, phrase) {
			return true
		}
	}
	return false
}

func looksSkillCapability(content string) bool {
	terms := []string{"save this workflow", "reusable skill", "create skill", "new skill", "teach yemaka", "learn this workflow"}
	for _, term := range terms {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func compactCapabilityDescription(request string) string {
	request = strings.Join(strings.Fields(request), " ")
	if request == "" {
		return "A missing capability requested by the user."
	}
	if len(request) <= 180 {
		return request
	}
	return strings.TrimSpace(request[:177]) + "..."
}

func capabilityProposalDescription(request string) string {
	subject := capabilityRequestSubject(request)
	return compactCapabilityDescription(subject)
}

func capabilityRequestSubject(request string) string {
	clean := strings.TrimSpace(strings.Join(strings.Fields(request), " "))
	if clean == "" {
		return clean
	}
	lower := strings.ToLower(clean)
	for _, phrase := range []string{
		"generate or create extension for ",
		"create or generate extension for ",
		"generate an extension for ",
		"generate extension for ",
		"create an extension for ",
		"create extension for ",
		"build an extension for ",
		"build extension for ",
		"generate a tool for ",
		"create a tool for ",
		"build a tool for ",
		"generate a capability for ",
		"create a capability for ",
		"build a capability for ",
	} {
		if idx := strings.Index(lower, phrase); idx >= 0 {
			subject := strings.TrimSpace(clean[idx+len(phrase):])
			if subject != "" {
				return trimCapabilitySubjectTail(subject)
			}
		}
	}
	for _, prefix := range []string{"okay ", "ok ", "please ", "first "} {
		if strings.HasPrefix(lower, prefix) {
			return capabilityRequestSubject(clean[len(prefix):])
		}
	}
	return clean
}

func trimCapabilitySubjectTail(subject string) string {
	subject = strings.TrimSpace(subject)
	lower := strings.ToLower(subject)
	for _, marker := range []string{
		" and then ",
		" then ",
		" after ",
		" so that ",
		" with approval",
		" until i approve",
	} {
		if idx := strings.Index(lower, marker); idx > 0 {
			subject = strings.TrimSpace(subject[:idx])
			lower = strings.ToLower(subject)
		}
	}
	return subject
}

func nonEmpty(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value != "" {
		return value
	}
	return fallback
}
