package routing

import (
	"net/url"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	TaskChat      = "chat"
	TaskCoding    = "coding"
	TaskReasoning = "reasoning"
	TaskRAG       = "rag"
	TaskTool      = "tool"

	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"

	RouteChatExplanation               = "chat_explanation"
	RouteMemorySearch                  = "memory_search"
	RouteRAGSearch                     = "rag_search"
	RouteInternetSearch                = "internet_search"
	RouteInternetFetch                 = "internet_fetch"
	RouteInternetHead                  = "internet_head"
	RouteCrawlerTask                   = "crawler_task"
	RouteWorkspaceRead                 = "workspace_read"
	RouteFileRead                      = "file_read"
	RouteFileWrite                     = "file_write"
	RouteShellTool                     = "shell_tool"
	RouteExtensionRun                  = "extension_run"
	RouteExtensionGenerate             = "extension_generate"
	RouteSchedulerCreate               = "scheduler_create"
	RouteConnectorAction               = "connector_action"
	RouteModelSettings                 = "model_settings"
	RouteSkillAction                   = "skill_action"
	RouteLearningAction                = "learning_action"
	RouteSettingsAction                = "settings_action"
	RoutePermissionRequired            = "permission_required"
	RouteClarify                       = "clarify"
	RouteClarifyScope                  = "clarify_scope"
	RouteAuthorizationRequired         = "authorization_required"
	RouteActiveAssessmentRequiresScope = "active_assessment_requires_scope"
	RoutePassiveCheck                  = "passive_check"
	RouteApprovalRequired              = "approval_required"
	RouteSafeAlternativeOffer          = "safe_alternative_offer"
	RouteLocalTime                     = "local_time"
	RouteHeartbeatStatus               = "heartbeat_status"

	IntentExplain   = "explain"
	IntentResearch  = "research"
	IntentAct       = "act"
	IntentGenerate  = "generate"
	IntentSchedule  = "schedule"
	IntentEdit      = "edit"
	IntentRunTool   = "run_tool"
	IntentSummarize = "summarize"
	IntentCompare   = "compare"
	IntentMonitor   = "monitor"
	IntentClarify   = "clarify"

	DomainGeneral      = "general"
	DomainCode         = "code"
	DomainRecon        = "recon"
	DomainMedical      = "medical"
	DomainEngineering  = "engineering"
	DomainArchitecture = "architecture"
	DomainLegal        = "legal"
	DomainFinance      = "finance"
	DomainTrading      = "trading"
	DomainMarketing    = "marketing"
	DomainEducation    = "education"
)

type Request struct {
	Content          string
	SourceKind       string
	HasContext       bool
	SkillName        string
	RAGEnabled       bool
	UseRAGConfig     bool
	WorkspaceContext string
	ProfileMemory    string
	TaskMemory       string
	Sources          []string
	MemorySources    []string
	RouteCorrections []RouteCorrection
	Continuation     ContinuationFrame
	SessionContract  SessionContract
}

type Decision struct {
	TaskType              string
	Confidence            int
	Reasons               []string
	NeedsClarification    bool
	ClarificationQuestion string
	Tools                 []string
	Files                 []string
	RiskLevel             string
	UseWorkspace          bool
	UseRAG                bool
	UseInternet           bool
	RouteCategory         string
	Intent                string
	Domain                string
	Target                string
	Capability            string
	RequiresApproval      bool
	ReadsFiles            bool
	WritesFiles           bool
	GeneratesExtension    bool
	CreatesSchedulerJob   bool
	ConnectorAction       bool
	CrawlerTask           bool
	RunsShell             bool
	TaskFrame             TaskFrame
	Preflight             PreflightCard
	PreflightSelected     bool
	Continuation          ContinuationFrame
	ContinuationMode      string
	AmbiguityFlags        []string
	ToolLane              string
	AllowedToolset        []string
	BlockedToolset        []string
	RouteCandidates       []RouteCandidate
	RouteCorrectionID     string
}

const (
	RouteCorrectionStatusPending  = "pending"
	RouteCorrectionStatusApproved = "approved"
	RouteCorrectionStatusRejected = "rejected"
)

type RouteCorrection struct {
	ID                    string   `json:"id"`
	SourceConversationID  string   `json:"sourceConversationId,omitempty"`
	Pattern               string   `json:"pattern"`
	IntendedRouteCategory string   `json:"intendedRouteCategory"`
	IntendedTaskType      string   `json:"intendedTaskType,omitempty"`
	IntendedCapability    string   `json:"intendedCapability,omitempty"`
	RequiredTools         []string `json:"requiredTools,omitempty"`
	ForbiddenTools        []string `json:"forbiddenTools,omitempty"`
	Tags                  []string `json:"tags,omitempty"`
	OriginalPrompt        string   `json:"originalPrompt,omitempty"`
	CorrectionText        string   `json:"correctionText,omitempty"`
	ClarificationQuestion string   `json:"clarificationQuestion,omitempty"`
	ApprovalStatus        string   `json:"approvalStatus"`
	Disabled              bool     `json:"disabled"`
	CreatedAt             string   `json:"createdAt,omitempty"`
	UpdatedAt             string   `json:"updatedAt,omitempty"`
}

func Classify(input Request) Decision {
	input.SessionContract = SanitizeSessionContract(input.SessionContract, time.Now())
	content := strings.ToLower(strings.TrimSpace(input.Content))
	if input.HasContext || hasProvidedContext(input) {
		input.HasContext = true
	}
	if content == "" {
		return enrichDecision(input, Decision{
			TaskType:              TaskChat,
			Confidence:            100,
			Reasons:               []string{"empty request"},
			NeedsClarification:    true,
			ClarificationQuestion: "What would you like Yemaka to do?",
		})
	}
	frame := ExtractTaskFrame(input)
	continuation := input.Continuation
	if continuation.IsZero() {
		continuation = BuildContinuationFrame(ContinuationInput{
			Content:    input.Content,
			TaskMemory: input.TaskMemory,
			Prior: ContinuationPriorState{
				RouteCategory: input.SessionContract.ActiveRoute,
				Target:        input.SessionContract.ActiveTarget,
				SourceOfTruth: sourceForRoute(input.SessionContract.ActiveRoute),
				FailureStatus: input.SessionContract.LastOutcome,
				FailureReason: input.SessionContract.FailureReason,
			},
		})
	}
	input.Continuation = continuation
	resolution := ResolveContinuation(input.Content, input.SessionContract, continuation)
	preflight := BuildPreflightCard(input, frame)
	candidates := BuildRouteCandidates(input, frame, preflight, resolution)
	if looksAuthorizationOnlyFollowup(content) && taskMemoryNeedsActiveAssessmentScope(input.TaskMemory) {
		decision := activeAssessmentAuthorizationFollowupDecision(input)
		decision.Preflight = preflight
		decision.Continuation = continuation
		decision.ContinuationMode = resolution.Mode
		decision.AmbiguityFlags = append([]string{}, resolution.AmbiguityFlags...)
		decision.RouteCandidates = candidates
		return enrichDecision(input, decision)
	}
	if decision, ok := decisionFromTaskFrame(frame); ok {
		decision.Preflight = preflight
		decision.Continuation = continuation
		decision.ContinuationMode = resolution.Mode
		decision.AmbiguityFlags = append([]string{}, resolution.AmbiguityFlags...)
		decision.RouteCandidates = candidates
		return enrichDecision(input, decision)
	}
	if decision, ok := decisionFromApprovedRouteCorrection(input, frame); ok {
		decision.Preflight = preflight
		decision.Continuation = continuation
		decision.ContinuationMode = resolution.Mode
		decision.AmbiguityFlags = append([]string{}, resolution.AmbiguityFlags...)
		decision.RouteCandidates = candidates
		return enrichDecision(input, decision)
	}
	if decision, ok := ArbitrateRoute(input, frame, preflight, resolution, candidates); ok {
		if len(decision.RouteCandidates) == 0 {
			decision.RouteCandidates = candidates
		}
		return enrichDecision(input, decision)
	}
	if decision, ok := decisionFromPreflight(input, preflight, frame); ok {
		decision.Continuation = continuation
		decision.ContinuationMode = resolution.Mode
		decision.AmbiguityFlags = append([]string{}, resolution.AmbiguityFlags...)
		decision.RouteCandidates = candidates
		return enrichDecision(input, decision)
	}
	if !input.HasContext {
		if clarification, ok := ambiguousReferenceQuestion(content); ok {
			return enrichDecision(input, Decision{
				TaskType:              TaskChat,
				Confidence:            30,
				Reasons:               []string{"ambiguous referenced target"},
				NeedsClarification:    true,
				ClarificationQuestion: clarification,
			})
		}
		if clarification, ok := missingToolTargetQuestion(content); ok {
			return enrichDecision(input, Decision{
				TaskType:              TaskChat,
				Confidence:            35,
				Reasons:               []string{"tool intent without target"},
				NeedsClarification:    true,
				ClarificationQuestion: clarification,
			})
		}
	}
	if LooksInlinePastedExplanationRequest(content) {
		return enrichDecision(input, Decision{TaskType: TaskReasoning, Confidence: 84, Reasons: []string{"inline pasted explanation intent"}})
	}
	if clarification, ok := conflictingRouteQuestion(content); ok {
		return enrichDecision(input, Decision{
			TaskType:              TaskChat,
			Confidence:            45,
			Reasons:               []string{"conflicting route signals"},
			NeedsClarification:    true,
			ClarificationQuestion: clarification,
		})
	}
	switch {
	case LooksAmbiguousLocalTimeTask(content):
		return enrichDecision(input, Decision{
			TaskType:              TaskChat,
			Confidence:            88,
			Reasons:               []string{"time request needs a more specific location"},
			NeedsClarification:    true,
			ClarificationQuestion: LocalTimeClarificationQuestion(content),
		})
	case LooksKnownLocalTimeTargetTask(content):
		return enrichDecision(input, Decision{TaskType: TaskTool, Confidence: 94, Reasons: []string{"location time intent"}})
	case LooksUnresolvedLocalTimeTargetTask(content):
		return enrichDecision(input, Decision{
			TaskType:              TaskChat,
			Confidence:            82,
			Reasons:               []string{"time request has an unresolved location"},
			NeedsClarification:    true,
			ClarificationQuestion: LocalTimeClarificationQuestion(content),
		})
	case LooksLocalTimeTask(content):
		return enrichDecision(input, Decision{TaskType: TaskTool, Confidence: 96, Reasons: []string{"local time intent"}})
	case LooksBroadCurrentNewsTask(content):
		return enrichDecision(input, Decision{
			TaskType:              TaskChat,
			Confidence:            64,
			Reasons:               []string{"broad current-news request requires clarification"},
			NeedsClarification:    true,
			ClarificationQuestion: "What kind of current news should I check: world news, technology, markets, politics, conflicts, sports, or local news?",
		})
	case LooksSpecificCurrentNewsTask(content):
		return enrichDecision(input, Decision{TaskType: TaskTool, Confidence: 88, Reasons: []string{"specific current-news search intent"}})
	case LooksCurrentWebFactTask(content):
		return enrichDecision(input, Decision{TaskType: TaskTool, Confidence: 86, Reasons: []string{"current web fact search intent"}})
	case LooksDocumentIngestAction(content):
		return enrichDecision(input, Decision{TaskType: TaskTool, Confidence: 88, Reasons: []string{"document ingestion action"}})
	case strings.EqualFold(input.SourceKind, "rag") || LooksRAGTask(content) || LooksDocumentSearchTask(content):
		return enrichDecision(input, Decision{TaskType: TaskRAG, Confidence: 95, Reasons: []string{"document or RAG intent"}})
	case LooksToolTask(content):
		return enrichDecision(input, Decision{TaskType: TaskTool, Confidence: toolConfidence(content), Reasons: toolRouteReasons(content)})
	case LooksDocumentContextTask(content):
		return enrichDecision(input, Decision{TaskType: TaskRAG, Confidence: 90, Reasons: []string{"document or RAG intent"}})
	case LooksCodingTask(content):
		return enrichDecision(input, Decision{TaskType: TaskCoding, Confidence: 78, Reasons: []string{"coding or code-analysis intent"}})
	case LooksReasoningTask(content):
		return enrichDecision(input, Decision{TaskType: TaskReasoning, Confidence: 72, Reasons: []string{"reasoning or explanation intent"}})
	default:
		return enrichDecision(input, Decision{TaskType: TaskChat, Confidence: 82, Reasons: []string{"direct chat intent"}})
	}
}

func enrichDecision(input Request, decision Decision) Decision {
	content := strings.TrimSpace(input.Content)
	if decision.TaskFrame.RawRequest == "" {
		decision.TaskFrame = ExtractTaskFrame(input)
	}
	if decision.Continuation.IsZero() {
		decision.Continuation = input.Continuation
		if decision.Continuation.IsZero() {
			decision.Continuation = BuildContinuationFrame(ContinuationInput{
				Content:    input.Content,
				TaskMemory: input.TaskMemory,
			})
		}
	}
	if strings.TrimSpace(decision.ContinuationMode) == "" {
		decision.ContinuationMode = ResolveContinuation(content, input.SessionContract, decision.Continuation).Mode
	}
	if decision.Preflight.IsZero() {
		decision.Preflight = BuildPreflightCard(input, decision.TaskFrame)
	}
	if len(decision.RouteCandidates) == 0 {
		decision.RouteCandidates = BuildRouteCandidates(input, decision.TaskFrame, decision.Preflight, ResolveContinuation(content, input.SessionContract, decision.Continuation))
	}
	inlinePastedExplanation := LooksInlinePastedExplanationRequest(content)
	if !inlinePastedExplanation {
		decision.Files = FileHints(content)
	}
	if len(decision.Files) == 0 && !inlinePastedExplanation && LooksFileFollowupReference(content) {
		decision.Files = FileHints(input.TaskMemory)
	}
	if decision.RouteCorrectionID == "" && !decision.PreflightSelected {
		decision.Tools = ToolsFor(content, decision.TaskType)
		if frameTools, ok := ToolsForTaskFrame(content, decision.TaskType, decision.TaskFrame); ok {
			decision.Tools = frameTools
		}
	}
	decision.RiskLevel = RiskLevel(content)
	if decision.TaskFrame.RiskLevel != "" &&
		(taskFrameOverridesRoute(decision.TaskFrame) ||
			decision.TaskFrame.PolicyDecision == TaskFramePolicyApprovalRequired) {
		decision.RiskLevel = decision.TaskFrame.RiskLevel
	}
	decision.UseWorkspace = ShouldRetrieveLocalContext(input)
	if hasDecisionTool(decision, "read_file") ||
		hasDecisionTool(decision, "list_files") ||
		hasDecisionTool(decision, "file_stat") ||
		hasDecisionTool(decision, "file_tree") ||
		hasDecisionTool(decision, "search_files") {
		decision.UseWorkspace = true
	}
	decision.UseRAG = ShouldUseRAG(input.RAGEnabled || !input.UseRAGConfig, content)
	decision.UseInternet = LooksInternetForDecision(strings.ToLower(content))
	if decision.RouteCorrectionID != "" || decision.Preflight.SelectedRoute != "" {
		switch decision.RouteCategory {
		case RouteRAGSearch:
			decision.UseRAG = ShouldUseRAG(input.RAGEnabled || !input.UseRAGConfig, content)
			decision.UseWorkspace = true
			decision.UseInternet = false
		case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
			decision.UseInternet = true
			decision.UseRAG = false
			decision.UseWorkspace = false
		case RouteWorkspaceRead, RouteFileRead:
			decision.UseWorkspace = true
			decision.UseRAG = false
			decision.UseInternet = false
		case RouteChatExplanation, RouteClarify:
			decision.UseWorkspace = false
			decision.UseRAG = false
			decision.UseInternet = false
		}
	}
	if inlinePastedExplanation {
		decision.Files = nil
		decision.Tools = nil
		decision.RiskLevel = RiskLow
		decision.UseWorkspace = false
		decision.UseRAG = false
		decision.UseInternet = false
	}
	if decision.NeedsClarification {
		decision.Files = nil
		decision.Tools = nil
		decision.RiskLevel = RiskLow
		if decision.TaskFrame.PolicyDecision == TaskFramePolicyAuthorizationRequired ||
			decision.TaskFrame.PolicyDecision == TaskFramePolicyClarifyScope {
			decision.RiskLevel = decision.TaskFrame.RiskLevel
		}
		decision.UseWorkspace = false
		decision.UseRAG = false
		decision.UseInternet = false
	}
	return withRouteMetadata(input, decision)
}

func LooksInternetForDecision(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	if LooksCapabilityActionTask(content) {
		return false
	}
	if LooksCrawlerTask(content) {
		return true
	}
	return LooksInternetTask(content)
}

func decisionFromApprovedRouteCorrection(input Request, frame TaskFrame) (Decision, bool) {
	if !routeCorrectionsMayApply(input.Content, frame) {
		return Decision{}, false
	}
	for _, correction := range input.RouteCorrections {
		if !routeCorrectionUsable(correction) || !routeCorrectionMatches(input.Content, correction) {
			continue
		}
		decision, ok := decisionFromRouteCorrection(correction)
		if !ok {
			continue
		}
		return decision, true
	}
	return Decision{}, false
}

func routeCorrectionsMayApply(content string, frame TaskFrame) bool {
	content = strings.ToLower(strings.TrimSpace(content))
	if content == "" {
		return false
	}
	if frame.PolicyDecision == TaskFramePolicyAuthorizationRequired ||
		frame.PolicyDecision == TaskFramePolicyApprovalRequired ||
		frame.ActionType == TaskFrameActionActiveAssessmentRequiresScope ||
		frame.ActionType == TaskFrameActionApprovalRequired {
		return false
	}
	if LooksFileEditIntent(content) ||
		LooksShellToolTask(content) ||
		LooksSchedulerCreateTask(content) ||
		LooksConnectorActionTask(content) ||
		LooksExtensionGenerationTask(content) ||
		LooksCrawlerTask(content) {
		return false
	}
	return true
}

func routeCorrectionUsable(correction RouteCorrection) bool {
	if correction.Disabled || !strings.EqualFold(strings.TrimSpace(correction.ApprovalStatus), RouteCorrectionStatusApproved) {
		return false
	}
	if strings.TrimSpace(correction.Pattern) == "" {
		return false
	}
	return routeCorrectionCategoryAllowed(correction.IntendedRouteCategory) &&
		routeCorrectionToolsAllowed(correction.RequiredTools) &&
		routeCorrectionToolsAllowed(correction.ForbiddenTools)
}

func routeCorrectionCategoryAllowed(category string) bool {
	switch strings.TrimSpace(category) {
	case RouteChatExplanation,
		RouteMemorySearch,
		RouteRAGSearch,
		RouteInternetSearch,
		RouteInternetFetch,
		RouteInternetHead,
		RouteWorkspaceRead,
		RouteFileRead,
		RouteClarify:
		return true
	default:
		return false
	}
}

func routeCorrectionToolsAllowed(tools []string) bool {
	for _, tool := range tools {
		switch strings.TrimSpace(tool) {
		case "", "memory_search", "rag_search", "internet_search", "internet_fetch", "internet_head", "internet_crawl", "read_file", "search_files", "list_files":
			continue
		default:
			return false
		}
	}
	return true
}

func routeCorrectionMatches(content string, correction RouteCorrection) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	pattern := strings.ToLower(strings.Join(strings.Fields(correction.Pattern), " "))
	if content == "" || pattern == "" {
		return false
	}
	if strings.Contains(content, pattern) || strings.Contains(pattern, content) {
		return true
	}
	patternTokens := routeCorrectionMeaningfulTokens(pattern)
	if len(patternTokens) == 0 {
		return routeCorrectionTagsMatch(content, correction.Tags)
	}
	contentTokens := WordTokens(content)
	matches := 0
	for _, token := range patternTokens {
		if containsAnyToken(contentTokens, token) {
			matches++
		}
	}
	if len(patternTokens) <= 2 {
		return matches == len(patternTokens) || routeCorrectionTagsMatch(content, correction.Tags)
	}
	if matches >= 2 && matches*100/len(patternTokens) >= 60 {
		return true
	}
	return routeCorrectionTagsMatch(content, correction.Tags)
}

func routeCorrectionMeaningfulTokens(content string) []string {
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "for": true, "from": true,
		"i": true, "in": true, "is": true, "it": true, "meant": true, "my": true,
		"of": true, "on": true, "or": true, "please": true, "the": true, "this": true,
		"to": true, "use": true, "with": true, "you": true,
	}
	tokens := WordTokens(content)
	result := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if stop[token] || len(token) < 2 {
			continue
		}
		result = appendNonDuplicate(result, token)
	}
	return result
}

func routeCorrectionTagsMatch(content string, tags []string) bool {
	if len(tags) == 0 {
		return false
	}
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	for _, tag := range tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "source:local_documents":
			if LooksDocumentContextTask(content) || LooksDocumentSearchTask(content) {
				return true
			}
		case "source:memory":
			if LooksMemorySearchTask(content) || ContainsSearchTerm(content, "memory") || ContainsSearchTerm(content, "conversation history") {
				return true
			}
		case "source:internet", "intent:freshness_required":
			if LooksSpecificCurrentNewsTask(content) || LooksCurrentWebFactTask(content) || LooksInternetSearchTask(content) {
				return true
			}
		case "source:workspace":
			if LooksLocalFileToolTask(content) || ContainsSearchTerm(content, "workspace") || ContainsSearchTerm(content, "repo") || ContainsSearchTerm(content, "repository") {
				return true
			}
		case "source:pasted_code":
			if LooksInlinePastedExplanationRequest(content) || LooksCodingTask(content) {
				return true
			}
		case "source:pasted_svg":
			if strings.Contains(content, "<svg") || ContainsSearchTerm(content, "svg") || LooksInlinePastedExplanationRequest(content) {
				return true
			}
		case "intent:clarify_before_acting", "intent:do_not_guess":
			if ambiguousReferenceLooksVague(content) {
				return true
			}
		}
	}
	return false
}

func ambiguousReferenceLooksVague(content string) bool {
	for _, term := range []string{"it", "this", "that", "there", "them", "check it", "search it", "open that", "fix it"} {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func decisionFromRouteCorrection(correction RouteCorrection) (Decision, bool) {
	category := strings.TrimSpace(correction.IntendedRouteCategory)
	taskType := strings.TrimSpace(correction.IntendedTaskType)
	if taskType == "" {
		taskType = routeCorrectionDefaultTask(category)
	}
	if taskType == "" {
		return Decision{}, false
	}
	tools := routeCorrectionDefaultTools(category)
	if len(correction.RequiredTools) > 0 {
		tools = append([]string{}, correction.RequiredTools...)
	}
	tools = removeForbiddenRouteCorrectionTools(tools, correction.ForbiddenTools)
	decision := Decision{
		TaskType:          taskType,
		Confidence:        91,
		Reasons:           []string{"approved route correction " + strings.TrimSpace(correction.ID)},
		Tools:             tools,
		RouteCategory:     category,
		Intent:            routeCorrectionDefaultIntent(category),
		Target:            routeCorrectionDefaultTarget(category),
		Capability:        firstNonEmptyString(correction.IntendedCapability, routeCorrectionDefaultCapability(category)),
		RouteCorrectionID: strings.TrimSpace(correction.ID),
	}
	if category == RouteClarify {
		decision.TaskType = TaskChat
		decision.NeedsClarification = true
		decision.ClarificationQuestion = firstNonEmptyString(correction.ClarificationQuestion, "What should Yemaka use as the target for this request?")
		decision.Tools = nil
	}
	return decision, true
}

func routeCorrectionDefaultTask(category string) string {
	switch category {
	case RouteRAGSearch:
		return TaskRAG
	case RouteMemorySearch, RouteInternetSearch, RouteInternetFetch, RouteInternetHead, RouteWorkspaceRead, RouteFileRead:
		return TaskTool
	case RouteClarify:
		return TaskChat
	case RouteChatExplanation:
		return TaskReasoning
	default:
		return ""
	}
}

func routeCorrectionDefaultTools(category string) []string {
	switch category {
	case RouteRAGSearch:
		return []string{"rag_search"}
	case RouteMemorySearch:
		return []string{"memory_search"}
	case RouteInternetSearch:
		return []string{"internet_search"}
	case RouteInternetFetch:
		return []string{"internet_fetch"}
	case RouteInternetHead:
		return []string{"internet_head"}
	case RouteWorkspaceRead:
		return []string{"search_files"}
	case RouteFileRead:
		return []string{"read_file"}
	case RouteShellTool:
		return []string{"run_shell"}
	default:
		return nil
	}
}

func routeCorrectionDefaultIntent(category string) string {
	switch category {
	case RouteChatExplanation:
		return IntentExplain
	case RouteClarify:
		return IntentClarify
	default:
		return IntentResearch
	}
}

func routeCorrectionDefaultTarget(category string) string {
	switch category {
	case RouteRAGSearch:
		return "local_documents"
	case RouteMemorySearch:
		return "memory"
	default:
		return ""
	}
}

func routeCorrectionDefaultCapability(category string) string {
	switch category {
	case RouteRAGSearch:
		return "rag"
	case RouteMemorySearch:
		return "memory"
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
		return "internet"
	case RouteWorkspaceRead, RouteFileRead:
		return "filesystem_read"
	default:
		return ""
	}
}

func removeForbiddenRouteCorrectionTools(tools []string, forbidden []string) []string {
	if len(tools) == 0 || len(forbidden) == 0 {
		return tools
	}
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		blocked := false
		for _, forbiddenTool := range forbidden {
			if strings.TrimSpace(tool) == strings.TrimSpace(forbiddenTool) {
				blocked = true
				break
			}
		}
		if !blocked {
			out = append(out, tool)
		}
	}
	return out
}

func validateCorrectedRouteSlots(content string, decision Decision) Decision {
	switch decision.RouteCategory {
	case RouteInternetFetch, RouteInternetHead:
		if _, ok := InternetURLHint(content); !ok {
			return correctedRouteClarification(decision, "What exact public URL or domain should I use for this web fetch?")
		}
	case RouteInternetSearch:
		if strings.TrimSpace(stripInternetSearchWords(content)) == "" {
			return correctedRouteClarification(decision, "What should I search for on the web?")
		}
	case RouteRAGSearch:
		if !LooksDocumentContextTask(content) && !LooksDocumentSearchTask(content) && decision.Target != "local_documents" {
			return correctedRouteClarification(decision, "Which local or ingested documents should I search?")
		}
	}
	return decision
}

func correctedRouteClarification(decision Decision, question string) Decision {
	decision.TaskType = TaskChat
	decision.Tools = nil
	decision.Files = nil
	decision.NeedsClarification = true
	decision.ClarificationQuestion = question
	decision.UseWorkspace = false
	decision.UseRAG = false
	decision.UseInternet = false
	decision.RouteCategory = RouteClarify
	decision.Intent = IntentClarify
	decision.Capability = ""
	return decision
}

func stripInternetSearchWords(content string) string {
	content = StripSearchIntentPrefix(content)
	for _, term := range []string{"search web", "search the web", "web search", "internet search", "search internet", "search the internet", "latest", "current", "updates", "look up"} {
		content = strings.ReplaceAll(content, term, " ")
	}
	return strings.Join(strings.Fields(content), " ")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func withRouteMetadata(input Request, decision Decision) Decision {
	rawContent := strings.TrimSpace(input.Content)
	content := strings.ToLower(rawContent)
	explicitRouteCategory := decision.RouteCategory
	explicitIntent := decision.Intent
	explicitDomain := decision.Domain
	explicitTarget := decision.Target
	explicitCapability := decision.Capability
	decision.RouteCategory = RouteCategoryFor(content, decision)
	decision.Intent = IntentFor(content, decision)
	decision.Domain = DomainFor(content, decision)
	decision.Target = TargetFor(rawContent, decision)
	decision.Capability = CapabilityFor(content, decision)
	preservePreflightRoute := decision.PreflightSelected &&
		(decision.Preflight.SelectedRoute != RouteWorkspaceRead ||
			decision.Continuation.Kind == ContinuationKindSourceCorrection) &&
		explicitRouteCategory != ""
	if decision.RouteCorrectionID != "" || preservePreflightRoute {
		if explicitRouteCategory != "" {
			decision.RouteCategory = explicitRouteCategory
		}
		if explicitIntent != "" {
			decision.Intent = explicitIntent
		}
		if explicitDomain != "" {
			decision.Domain = explicitDomain
		}
		if explicitTarget != "" {
			decision.Target = explicitTarget
		}
		if explicitCapability != "" {
			decision.Capability = explicitCapability
		}
	}
	if decision.PreflightSelected && explicitRouteCategory == RouteChatExplanation {
		decision.Target = ""
	}
	if decision.RouteCorrectionID != "" {
		decision = validateCorrectedRouteSlots(content, decision)
	}
	decision.ReadsFiles = routeReadsFiles(decision)
	decision.WritesFiles = !decision.NeedsClarification && LooksFileEditIntent(content)
	decision.GeneratesExtension = !decision.NeedsClarification && (LooksExtensionGenerationTask(content) || decision.RouteCategory == RouteExtensionGenerate)
	decision.CreatesSchedulerJob = !decision.NeedsClarification && LooksSchedulerCreateTask(content)
	decision.ConnectorAction = !decision.NeedsClarification && LooksConnectorActionTask(content) && !LooksExtensionGenerationTask(content)
	decision.CrawlerTask = !decision.NeedsClarification && LooksCrawlerTask(content)
	decision.RunsShell = !decision.NeedsClarification && LooksShellToolTask(content)
	decision.RequiresApproval = RequiresApprovalFor(decision)
	lane := ToolLaneForRoute(decision.RouteCategory)
	decision.ToolLane = lane.Name
	decision.AllowedToolset = append([]string{}, lane.AllowedTools...)
	decision.BlockedToolset = append([]string{}, lane.BlockedTools...)
	decision.Tools = ApplyToolLane(decision.Tools, decision.RouteCategory)
	if lane.RequiresApproval {
		decision.RequiresApproval = true
	}
	decision.Preflight = finalizeDecisionPreflight(input, decision)
	return decision
}

func finalizeDecisionPreflight(input Request, decision Decision) PreflightCard {
	card := decision.Preflight
	if card.IsZero() {
		card = BuildPreflightCard(input, decision.TaskFrame)
	}
	card.SelectedRoute = decision.RouteCategory
	card.Intent = decision.Intent
	card.Domain = decision.Domain
	card.Target = decision.Target
	card.RiskLevel = decision.RiskLevel
	card.Confidence = decision.Confidence
	card.ProtectedPolicyDecision = decision.TaskFrame.PolicyDecision
	card.RequiredTool = firstDecisionTool(decision.Tools)
	if !decision.Continuation.IsZero() {
		card.ContinuationKind = decision.Continuation.Kind
		card.ContinuationTargetSource = decision.Continuation.TargetSource
		card.PriorTarget = decision.Continuation.PriorTarget
		if decision.Continuation.PriorFailureReason != "" {
			card.BlockedRequirement = decision.Continuation.PriorFailureReason
		}
	}
	if decision.RouteCorrectionID != "" {
		card.AppliedCorrectionIDs = appendPreflightUnique(card.AppliedCorrectionIDs, decision.RouteCorrectionID)
	}
	if decision.NeedsClarification {
		card.SourceOfTruth = PreflightSourceClarify
		card.MissingSlots = appendPreflightUnique(card.MissingSlots, "clarification")
		card.StopReason = firstNonEmptyString(decision.ClarificationQuestion, "clarification required before tools")
	}
	return preflightFinalize(card)
}

func firstDecisionTool(tools []string) string {
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool != "" {
			return tool
		}
	}
	return ""
}

func RouteCategoryFor(content string, decision Decision) string {
	content = strings.ToLower(strings.TrimSpace(content))
	switch {
	case taskFrameOverridesRoute(decision.TaskFrame) && decision.TaskFrame.RouteCategory != "":
		return decision.TaskFrame.RouteCategory
	case decision.NeedsClarification:
		return RouteClarify
	case LooksInlinePastedExplanationRequest(content):
		return RouteChatExplanation
	case LooksExtensionGenerationTask(content):
		return RouteExtensionGenerate
	case LooksSkillCreationTask(content):
		return RouteSkillAction
	case LooksCrawlerTask(content):
		return RouteCrawlerTask
	case LooksSchedulerCreateTask(content):
		return RouteSchedulerCreate
	case LooksWorkflowActionTask(content):
		return RouteExtensionGenerate
	case LooksConnectorActionTask(content):
		return RouteConnectorAction
	case LooksDocumentIngestAction(content):
		return RouteSettingsAction
	case LooksFileEditIntent(content):
		return RouteFileWrite
	case LooksShellToolTask(content):
		return RouteShellTool
	case hasDecisionTool(decision, "memory_search"):
		return RouteMemorySearch
	case decision.TaskType == TaskRAG || hasDecisionTool(decision, "rag_search"):
		return RouteRAGSearch
	case hasDecisionTool(decision, "internet_search"):
		return RouteInternetSearch
	case hasDecisionTool(decision, "internet_head"):
		return RouteInternetHead
	case hasDecisionTool(decision, "internet_fetch"):
		return RouteInternetFetch
	case hasDecisionTool(decision, "local_time"):
		return RouteLocalTime
	case hasDecisionTool(decision, "heartbeat_status"):
		return RouteHeartbeatStatus
	case hasDecisionTool(decision, "list_files") || hasDecisionTool(decision, "file_tree") || hasDecisionTool(decision, "file_stat"):
		return RouteWorkspaceRead
	case hasDecisionTool(decision, "read_file"):
		return RouteFileRead
	case hasDecisionTool(decision, "search_files") || decision.UseWorkspace:
		return RouteWorkspaceRead
	case decision.TaskType == TaskTool:
		return RoutePermissionRequired
	default:
		return RouteChatExplanation
	}
}

func IntentFor(content string, decision Decision) string {
	content = strings.ToLower(strings.TrimSpace(content))
	switch {
	case taskFrameOverridesRoute(decision.TaskFrame) && decision.TaskFrame.Intent != "":
		return decision.TaskFrame.Intent
	case decision.NeedsClarification:
		return IntentClarify
	case LooksInlinePastedExplanationRequest(content):
		return IntentExplain
	case LooksExtensionGenerationTask(content):
		return IntentGenerate
	case LooksSkillCreationTask(content):
		return IntentGenerate
	case LooksSchedulerCreateTask(content):
		return IntentSchedule
	case LooksWorkflowActionTask(content):
		return IntentGenerate
	case LooksFileEditIntent(content):
		return IntentEdit
	case LooksCrawlerTask(content):
		if ContainsSearchTerm(content, "monitor") {
			return IntentMonitor
		}
		return IntentResearch
	case LooksConnectorActionTask(content):
		return IntentAct
	case LooksDocumentIngestAction(content):
		return IntentAct
	case ContainsSearchTerm(content, "compare") || ContainsSearchTerm(content, "difference"):
		return IntentCompare
	case ContainsSearchTerm(content, "summarize") || ContainsSearchTerm(content, "summary"):
		return IntentSummarize
	case decision.TaskType == TaskRAG || hasDecisionTool(decision, "memory_search") || hasDecisionTool(decision, "internet_search"):
		return IntentResearch
	case LooksShellToolTask(content) || decision.TaskType == TaskTool:
		return IntentRunTool
	default:
		return IntentExplain
	}
}

func DomainFor(content string, decision Decision) string {
	content = strings.ToLower(strings.TrimSpace(content))
	if taskFrameOverridesRoute(decision.TaskFrame) && decision.TaskFrame.Domain != "" {
		return decision.TaskFrame.Domain
	}
	tokens := WordTokens(content)
	switch {
	case LooksCodingTask(content) || containsAnyToken(tokens, "code", "codebase", "function", "class", "compile", "test", "tests"):
		return DomainCode
	case containsAnyToken(tokens, "pentest", "recon", "vulnerability", "vulnerabilities", "security", "crawl", "crawler"):
		return DomainRecon
	case containsAnyToken(tokens, "medical", "doctor", "health", "diagnosis", "blood", "pressure"):
		return DomainMedical
	case containsAnyToken(tokens, "architecture", "architectural"):
		return DomainArchitecture
	case containsAnyToken(tokens, "engineering", "engineer"):
		return DomainEngineering
	case containsAnyToken(tokens, "legal", "contract", "law", "lawyer"):
		return DomainLegal
	case containsAnyToken(tokens, "trading", "trade", "stock", "stocks", "crypto"):
		return DomainTrading
	case containsAnyToken(tokens, "finance", "financial", "budget", "invoice"):
		return DomainFinance
	case containsAnyToken(tokens, "marketing", "campaign", "copywriting"):
		return DomainMarketing
	case containsAnyToken(tokens, "study", "teacher", "tutor", "homework"):
		return DomainEducation
	case decision.TaskType == TaskCoding:
		return DomainCode
	default:
		return DomainGeneral
	}
}

func TargetFor(content string, decision Decision) string {
	content = strings.TrimSpace(content)
	if taskFrameOverridesRoute(decision.TaskFrame) && decision.TaskFrame.Target != "" {
		return decision.TaskFrame.Target
	}
	if decision.NeedsClarification {
		return ""
	}
	if LooksInlinePastedExplanationRequest(content) {
		return ""
	}
	if target := ResolveLocalTimeTarget(content); target.TimeZone != "" {
		return target.Label
	}
	if target, ok := InternetURLHint(content); ok {
		return target
	}
	if files := FileHints(content); len(files) > 0 {
		return files[0]
	}
	switch {
	case LooksConnectorActionTask(strings.ToLower(content)):
		return connectorTarget(content)
	case LooksSchedulerCreateTask(strings.ToLower(content)):
		return "scheduler_job"
	case LooksExtensionGenerationTask(strings.ToLower(content)):
		return "generated_extension"
	case hasDecisionTool(decision, "memory_search"):
		return "memory"
	case decision.TaskType == TaskRAG:
		return "local_documents"
	default:
		return ""
	}
}

func CapabilityFor(content string, decision Decision) string {
	content = strings.ToLower(strings.TrimSpace(content))
	if taskFrameOverridesRoute(decision.TaskFrame) && decision.TaskFrame.Capability != "" {
		return decision.TaskFrame.Capability
	}
	switch decision.RouteCategory {
	case RouteExtensionGenerate:
		return "extension_generation"
	case RouteCrawlerTask:
		return "crawler"
	case RouteSchedulerCreate:
		return "scheduler"
	case RouteConnectorAction:
		return "connector"
	case RouteFileWrite:
		return "filesystem_write"
	case RouteFileRead, RouteWorkspaceRead:
		return "filesystem_read"
	case RouteShellTool:
		return "shell"
	case RouteMemorySearch:
		return "memory"
	case RouteRAGSearch:
		return "rag"
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
		return "internet"
	case RouteLocalTime:
		return "local_time"
	case RouteSkillAction:
		return "skills"
	case RouteSettingsAction:
		if LooksDocumentIngestAction(content) {
			return "document_ingestion"
		}
	}
	if LooksSkillCreationTask(content) {
		return "skills"
	}
	return ""
}

func RequiresApprovalFor(decision Decision) bool {
	return decision.RiskLevel == RiskHigh ||
		decision.WritesFiles ||
		decision.GeneratesExtension ||
		decision.CreatesSchedulerJob ||
		decision.ConnectorAction ||
		decision.CrawlerTask ||
		decision.RunsShell ||
		decision.RouteCategory == RoutePermissionRequired
}

func routeReadsFiles(decision Decision) bool {
	return hasDecisionTool(decision, "list_files") ||
		hasDecisionTool(decision, "read_file") ||
		hasDecisionTool(decision, "search_files") ||
		hasDecisionTool(decision, "rag_search") ||
		decision.RouteCategory == RouteFileRead ||
		decision.RouteCategory == RouteWorkspaceRead ||
		decision.RouteCategory == RouteRAGSearch
}

func hasDecisionTool(decision Decision, want string) bool {
	for _, tool := range decision.Tools {
		if tool == want {
			return true
		}
	}
	return false
}

func connectorTarget(content string) string {
	lower := strings.ToLower(content)
	for _, target := range []string{"slack", "telegram", "discord", "email", "webhook", "connector"} {
		if ContainsSearchTerm(lower, target) {
			return target
		}
	}
	return "connector"
}

func ShouldRetrieveLocalContext(input Request) bool {
	content := strings.ToLower(strings.TrimSpace(input.Content))
	if content == "" {
		return false
	}
	if input.SessionContract.ActiveRoute == RouteRAGSearch &&
		(ResolveContinuation(input.Content, input.SessionContract, input.Continuation).KeepActiveRoute || looksSameTaskFollowup(content) || looksFollowupQuestion(content)) {
		return true
	}
	if (input.SessionContract.ActiveRoute == RouteWorkspaceRead || input.SessionContract.ActiveRoute == RouteFileRead) &&
		(ResolveContinuation(input.Content, input.SessionContract, input.Continuation).KeepActiveRoute || looksSameTaskFollowup(content) || looksFollowupQuestion(content)) {
		return true
	}
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	if LooksLocalTimeTask(content) || LooksKnownLocalTimeTargetTask(content) || LooksUnresolvedLocalTimeTargetTask(content) || LooksBroadCurrentNewsTask(content) || LooksSpecificCurrentNewsTask(content) {
		return false
	}
	if LooksShellToolTask(content) || LooksCrawlerTask(content) {
		return false
	}
	if LooksDocumentIngestAction(content) {
		return false
	}
	if LooksMemoryContextTask(content) || LooksCapabilityActionTask(content) || LooksInternetTask(content) {
		return false
	}
	if LooksFileTreeTask(content) || LooksListFilesTask(content) || LooksFileMetadataTask(content) || LooksGitStatusIntent(content) {
		return false
	}
	if LooksFileEditIntent(content) && looksInlineFileWriteContent(content) {
		return false
	}
	if strings.TrimSpace(input.SkillName) != "" {
		return true
	}
	if hasExplicitLocalRetrievalIntent(content) {
		return true
	}
	if LooksToolTask(content) && LooksLocalContextToolTask(content) {
		return true
	}
	return false
}

func ShouldUseRAG(enabled bool, content string) bool {
	if !enabled {
		return false
	}
	content = strings.ToLower(strings.TrimSpace(content))
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	if LooksDocumentIngestAction(content) {
		return false
	}
	if LooksLocalTimeTask(content) || LooksKnownLocalTimeTargetTask(content) || LooksUnresolvedLocalTimeTargetTask(content) || LooksBroadCurrentNewsTask(content) || LooksSpecificCurrentNewsTask(content) {
		return false
	}
	return content != "" && LooksDocumentContextTask(content)
}

func ToolsFor(content string, taskType string) []string {
	lower := strings.ToLower(content)
	tools := make([]string, 0, 4)
	add := func(name string) {
		for _, tool := range tools {
			if tool == name {
				return
			}
		}
		tools = append(tools, name)
	}
	if LooksInlinePastedExplanationRequest(lower) {
		return tools
	}
	if taskType == TaskRAG {
		add("rag_search")
		return tools
	}
	if LooksDocumentIngestAction(lower) {
		add("ingest_documents")
		return tools
	}
	if LooksCapabilityActionTask(lower) {
		add("safe_tool")
		return tools
	}
	if LooksCrawlerTask(lower) {
		add("internet_crawl")
		return tools
	}
	if LooksShellToolTask(lower) {
		add("safe_tool")
		return tools
	}
	if LooksFileEditIntent(lower) && looksInlineFileWriteContent(lower) {
		add("edit_file")
		return tools
	}
	if LooksKnownLocalTimeTargetTask(lower) || (LooksLocalTimeTask(lower) && !LooksUnresolvedLocalTimeTargetTask(lower)) {
		add("local_time")
		return tools
	}
	if LooksFileTreeTask(lower) {
		add("file_tree")
		return tools
	}
	if LooksListFilesTask(lower) {
		add("list_files")
		return tools
	}
	if LooksFileMetadataTask(lower) {
		add("file_stat")
	}
	if LooksLocalFileToolTask(lower) || (LooksFileStateQuestion(lower) && !LooksFileMetadataTask(lower)) || (ContainsLocalContextTerm(lower) && !LooksFileMetadataTask(lower)) {
		add("read_file")
		add("search_files")
	}
	if LooksFileEditIntent(lower) {
		add("edit_file")
	}
	if LooksTestToolIntent(lower) {
		add("run_tests")
	}
	if LooksGitStatusIntent(lower) {
		add("git_status")
	} else if LooksGitToolIntent(lower) {
		add("git_diff")
	}
	if LooksMemorySearchTask(lower) {
		add("memory_search")
	}
	if LooksDoctorStatusTask(lower) {
		add("doctor_status")
	}
	if LooksHeartbeatStatusTask(lower) {
		add("heartbeat_status")
	}
	if ContainsSearchTerm(lower, "project map") || ContainsSearchTerm(lower, "map project") || ContainsSearchTerm(lower, "project overview") {
		add("project_map")
	}
	if ContainsSearchTerm(lower, "symbol search") || ContainsSearchTerm(lower, "function search") || ContainsSearchTerm(lower, "class search") {
		add("symbol_search")
	}
	if ContainsSearchTerm(lower, "secret scan") || ContainsSearchTerm(lower, "scan secrets") || ContainsSearchTerm(lower, "find secrets") {
		add("secret_scan")
	}
	if ContainsSearchTerm(lower, "patch preview") || ContainsSearchTerm(lower, "preview patch") {
		add("patch_preview")
	}
	if LooksSpecificCurrentNewsTask(lower) || LooksCurrentWebFactTask(lower) {
		add("internet_search")
	} else if LooksInternetTask(lower) {
		if ContainsSearchTerm(lower, "head") || ContainsSearchTerm(lower, "headers") || ContainsSearchTerm(lower, "status code") {
			add("internet_head")
		} else if LooksInternetSearchTask(lower) {
			add("internet_search")
		} else {
			add("internet_fetch")
		}
	}
	if len(tools) == 0 && taskType == TaskTool {
		add("safe_tool")
	}
	return tools
}

func RiskLevel(content string) string {
	lower := strings.ToLower(content)
	if LooksInlinePastedExplanationRequest(lower) || LooksExplanationOnlyRequest(lower) {
		return RiskLow
	}
	highRisk := []string{"rm", "delete", "remove", "sudo", "chmod", "chown", "git reset", "git clean", "git push", "curl | sh", "wget | sh", "brew install", "pip install", "npm install", "docker", "ssh", "scp", "pnpm install", "pip3 install", "apt install", "apt-get install", "yum install", "dnf install", "zypper install"}
	for _, term := range highRisk {
		if containsCommandRiskTerm(lower, term) {
			return RiskHigh
		}
	}
	if LooksFileEditIntent(lower) ||
		LooksTestToolIntent(lower) ||
		LooksConnectorActionTask(lower) ||
		LooksWorkflowActionTask(lower) ||
		LooksExtensionGenerationTask(lower) ||
		LooksSkillCreationTask(lower) ||
		LooksCrawlerTask(lower) ||
		LooksInternetTask(lower) {
		return RiskMedium
	}
	mediumRisk := []string{"shell command", "run command", "execute command", "patch preview", "preview patch"}
	for _, term := range mediumRisk {
		if ContainsSearchTerm(lower, term) {
			return RiskMedium
		}
	}
	return RiskLow
}

func FileHints(content string) []string {
	fields := strings.Fields(content)
	hints := make([]string, 0, 4)
	for _, field := range fields {
		candidate := strings.Trim(field, ".,:;()[]{}\"'")
		if candidate == "" {
			continue
		}
		if _, ok := normalizeInternetTarget(candidate); ok {
			continue
		}
		if strings.Contains(candidate, "/") || strings.Contains(filepath.Base(candidate), ".") {
			hints = append(hints, candidate)
		}
		if len(hints) >= 6 {
			break
		}
	}
	if combined := namedFilePathHints(content, hints); len(combined) > 0 {
		return appendUniqueFileHints(combined, hints)
	}
	return hints
}

func namedFilePathHints(content string, hints []string) []string {
	if len(hints) < 2 || !looksNamedFilePathRequest(content) {
		return nil
	}
	dirs := make([]string, 0, 1)
	files := make([]string, 0, 1)
	for _, hint := range hints {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			continue
		}
		hasSlash := strings.Contains(filepath.ToSlash(hint), "/")
		isFile := looksFileLikePath(hint)
		switch {
		case hasSlash && !isFile:
			dirs = append(dirs, hint)
		case !hasSlash && isFile:
			files = append(files, hint)
		}
	}
	if len(dirs) == 0 || len(files) == 0 {
		return nil
	}
	return []string{filepath.ToSlash(filepath.Join(dirs[0], files[0]))}
}

func looksNamedFilePathRequest(content string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
	if normalized == "" {
		return false
	}
	hasNameCue := ContainsSearchTerm(normalized, "name") ||
		ContainsSearchTerm(normalized, "named") ||
		strings.Contains(normalized, "file name") ||
		strings.Contains(normalized, "filename")
	if !hasNameCue {
		return false
	}
	return ContainsSearchTerm(normalized, "create") ||
		ContainsSearchTerm(normalized, "write") ||
		ContainsSearchTerm(normalized, "save") ||
		ContainsSearchTerm(normalized, "file")
}

func looksFileLikePath(path string) bool {
	base := filepath.Base(filepath.ToSlash(strings.TrimSpace(path)))
	return base != "" && base != "." && base != ".." && strings.Contains(base, ".")
}

func looksInlineFileWriteContent(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	for _, marker := range []string{
		"with exactly this line:",
		"exactly this line:",
		"with this line:",
		"this line:",
		"with the text",
		"with text",
		"with content:",
		"content:",
		"set content to:",
		"containing",
		"that says",
		"write content:",
		"write into it",
		"write to it",
		"write in it",
		"write into the file",
		"write to the file",
		"put into it",
		"put in it",
	} {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}

func appendUniqueFileHints(groups ...[]string) []string {
	out := make([]string, 0, 6)
	for _, group := range groups {
		for _, value := range group {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			seen := false
			for _, existing := range out {
				if existing == value {
					seen = true
					break
				}
			}
			if !seen {
				out = append(out, value)
			}
			if len(out) >= 6 {
				return out
			}
		}
	}
	return out
}

func LooksToolTask(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	if (LooksLocalTimeTask(content) && !LooksUnresolvedLocalTimeTargetTask(content)) ||
		LooksKnownLocalTimeTargetTask(content) ||
		LooksSpecificCurrentNewsTask(content) ||
		LooksGenericInternetSearchTask(content) ||
		LooksInternetTask(content) ||
		LooksCrawlerTask(content) ||
		LooksMemorySearchTask(content) ||
		LooksDocumentIngestAction(content) ||
		LooksCapabilityActionTask(content) ||
		LooksFileTreeTask(content) ||
		LooksListFilesTask(content) ||
		LooksLocalFileToolTask(content) ||
		LooksFileStateQuestion(content) ||
		LooksFileEditIntent(content) ||
		LooksTestToolIntent(content) ||
		LooksGitToolIntent(content) ||
		LooksShellToolTask(content) ||
		LooksDoctorStatusTask(content) ||
		LooksHeartbeatStatusTask(content) ||
		LooksLocalContextToolTask(content) {
		return true
	}
	terms := []string{
		"shell command", "run command", "execute command",
		"rollback snapshot", "create snapshot", "restore snapshot",
		"git reset", "git clean", "git push", "npm install", "pip install", "brew install",
		"curl | sh", "wget | sh",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksFileEditIntent(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	if len(FileHints(content)) > 0 {
		for _, term := range []string{"write", "edit", "change", "modify", "update", "replace", "append"} {
			if ContainsSearchTerm(content, term) {
				return true
			}
		}
		for _, term := range []string{
			"create a file", "create file", "make a file", "make file",
			"new file", "save to", "save as",
		} {
			if ContainsSearchTerm(content, term) {
				return true
			}
		}
		if looksInlineFileWriteContent(content) &&
			(ContainsSearchTerm(content, "create") || ContainsSearchTerm(content, "save")) {
			return true
		}
	}
	if strings.Contains(content, "write file") ||
		strings.Contains(content, "edit file") ||
		strings.Contains(content, "create file") ||
		strings.Contains(content, "create a file") ||
		strings.Contains(content, "make file") ||
		strings.Contains(content, "make a file") ||
		strings.Contains(content, "new file") ||
		strings.Contains(content, "write to file") ||
		strings.Contains(content, "edit the file") {
		return true
	}
	for _, term := range []string{"change file", "change the file", "change in ", "change line", "modify file", "modify the file", "update file", "update the file"} {
		if strings.Contains(content, term) {
			return true
		}
	}
	return false
}

func LooksLocalFileToolTask(content string) bool {
	terms := []string{
		"read file", "read the file", "open file", "open the file",
		"search file", "search files", "find file", "find files",
		"search workspace", "search project", "search repo", "search repository",
		"search code", "inspect file", "inspect files",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksListFilesTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	terms := []string{
		"list files", "list the files", "list them", "show files", "show the files",
		"what files are in", "what file are in", "which files are in",
		"folder contains", "directory contains",
		"what is in the folder", "what is in this folder",
		"what is in the directory", "what is in this directory",
		"contents of folder", "contents of directory",
	}
	hasListIntent := false
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			hasListIntent = true
			break
		}
	}
	if !hasListIntent {
		return false
	}
	if len(FileHints(content)) > 0 {
		return true
	}
	return ContainsSearchTerm(content, "folder") ||
		ContainsSearchTerm(content, "directory") ||
		ContainsSearchTerm(content, "workspace") ||
		ContainsSearchTerm(content, "repo") ||
		ContainsSearchTerm(content, "project")
}

func LooksFileTreeTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	for _, term := range []string{
		"file tree", "directory tree", "folder tree",
		"recursive list", "list recursively", "show recursively",
		"list all files", "show all files", "all files under",
	} {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksFileStateQuestion(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	statusTerms := []string{
		"current content", "current contents", "file content", "file contents",
		"where is", "where it is", "where is it", "where saved", "where is saved",
		"file path", "path of", "saved", "located", "location",
		"exists", "exist", "missing", "not found", "no file", "found no file",
		"can not find", "cannot find", "can't find", "could not find", "couldn't find",
		"created", "create it", "create the file", "did you create",
		"written", "wrote", "write it", "write the file", "did you write",
		"saved it", "save it", "save the file", "did you save",
		"listed", "see it", "see the file", "did you see",
		"done retrieving", "done reading", "retrieved", "retrieving", "read yet",
	}
	hasStatusTerm := false
	for _, term := range statusTerms {
		if ContainsSearchTerm(content, term) {
			hasStatusTerm = true
			break
		}
	}
	if !hasStatusTerm {
		return false
	}
	if len(FileHints(content)) > 0 {
		return true
	}
	for _, reference := range []string{"the file", "this file", "that file", "the folder", "this folder", "that folder", "there"} {
		if ContainsSearchTerm(content, reference) {
			return true
		}
	}
	return false
}

func LooksFileMetadataTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	hasTerm := false
	for _, term := range []string{
		"where is", "where it is", "where is it", "where saved", "where is saved",
		"file path", "path of", "saved", "located", "location",
		"exists", "exist", "missing", "not found", "no file", "found no file",
		"created", "create it", "create the file", "did you create",
		"written", "wrote", "write it", "write the file", "did you write",
		"saved it", "save it", "save the file", "did you save",
		"listed", "see it", "see the file", "did you see",
		"file size", "last modified", "modified time", "mtime",
	} {
		if ContainsSearchTerm(content, term) {
			hasTerm = true
			break
		}
	}
	if !hasTerm {
		return false
	}
	if len(FileHints(content)) > 0 {
		return true
	}
	for _, reference := range []string{"the file", "this file", "that file", "the folder", "this folder", "that folder", "there"} {
		if ContainsSearchTerm(content, reference) {
			return true
		}
	}
	return false
}

func LooksFileFollowupReference(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" || !LooksFileStateQuestion(content) {
		return false
	}
	return len(FileHints(content)) == 0
}

func LooksLocalContextToolTask(content string) bool {
	terms := []string{
		"project map", "map project", "project overview",
		"secret scan", "scan secrets", "find secrets",
		"symbol search", "function search", "class search",
		"patch preview", "preview patch",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksTestToolIntent(content string) bool {
	terms := []string{
		"run tests", "run the tests", "run test", "run the test",
		"run test suite", "test suite", "test failures", "failing test",
		"failing tests", "unit tests", "integration tests", "go test", "npm test", "pytest",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "test ") || strings.HasPrefix(trimmed, "test the ")
}

func LooksGitToolIntent(content string) bool {
	terms := []string{"git diff", "show diff", "show git diff", "review diff", "review git diff", "git status", "show git status"}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksGitStatusIntent(content string) bool {
	for _, term := range []string{"git status", "show git status", "repo status", "repository status", "working tree status"} {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksMemorySearchTask(content string) bool {
	terms := []string{"search memory", "search memories", "memory search", "find memory", "find memories", "search conversation", "search conversations", "search chat", "search chats"}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksDoctorStatusTask(content string) bool {
	terms := []string{"doctor status", "yemaka doctor", "run doctor", "release check", "readiness check", "check readiness", "system readiness"}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksHeartbeatStatusTask(content string) bool {
	terms := []string{
		"heartbeat status",
		"yemaka heartbeat",
		"run heartbeat",
		"agent heartbeat",
		"system heartbeat",
		"heartbeat report",
		"yemaka health",
		"yemaka health status",
		"agent health status",
		"system health status",
		"runtime health status",
		"health panel",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksCapabilityActionTask(content string) bool {
	return LooksExtensionGenerationTask(content) ||
		LooksConnectorActionTask(content) ||
		LooksWorkflowActionTask(content) ||
		LooksSkillCreationTask(content)
}

func LooksExplanationOnlyRequest(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	content = StripSearchIntentPrefix(content)
	if content == "" {
		return false
	}
	for _, action := range []string{" and run ", " then run ", " and execute ", " then execute ", " and edit ", " then edit ", " and create ", " then create ", " and delete ", " then delete "} {
		if strings.Contains(content, action) {
			return false
		}
	}
	if LooksBroadCurrentNewsTask(content) || LooksSpecificCurrentNewsTask(content) {
		return false
	}
	prefixes := []string{
		"what is ", "what are ", "what's ", "whats ",
		"explain:", "explain ", "explain how ", "explain why ",
		"explain this:", "explain the following:",
		"summarize:", "summarize this:", "summarize the following:",
		"analyze:", "analyze this:", "analyze the following:",
		"how does ", "how do i ", "how can i ",
		"tell me about ", "why does ", "why is ",
		"compare ", "what should i ask ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(content, prefix) {
			return true
		}
	}
	return false
}

func LooksInlinePastedExplanationRequest(content string) bool {
	_, body, ok := splitInlinePastedExplanationRequest(content)
	return ok && looksLikePastedExplanationBody(body)
}

func splitInlinePastedExplanationRequest(content string) (string, string, bool) {
	raw := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(content, "\r\n", "\n"), "\r", "\n"))
	if raw == "" {
		return "", "", false
	}
	lines := strings.Split(raw, "\n")
	firstLineIndex := -1
	for i, line := range lines {
		if strings.TrimSpace(line) != "" {
			firstLineIndex = i
			break
		}
	}
	if firstLineIndex == -1 {
		return "", "", false
	}
	firstLine := strings.TrimSpace(lines[firstLineIndex])
	rest := ""
	if firstLineIndex+1 < len(lines) {
		rest = strings.TrimSpace(strings.Join(lines[firstLineIndex+1:], "\n"))
	}
	if instruction, body, ok := splitInlinePastedInstructionLine(firstLine, rest); ok {
		return instruction, body, true
	}
	if isPastedExplanationInstruction(firstLine) {
		return firstLine, rest, rest != ""
	}
	return "", "", false
}

func splitInlinePastedInstructionLine(firstLine string, rest string) (string, string, bool) {
	colon := strings.Index(firstLine, ":")
	if colon < 0 {
		return "", "", false
	}
	instruction := strings.TrimSpace(firstLine[:colon])
	if !isPastedExplanationInstruction(instruction) {
		return "", "", false
	}
	body := strings.TrimSpace(firstLine[colon+1:])
	if rest != "" {
		if body != "" {
			body += "\n"
		}
		body += rest
	}
	return instruction, body, body != ""
}

func isPastedExplanationInstruction(instruction string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(instruction), " "))
	normalized = strings.Trim(normalized, " \t\n\r:;,.!?")
	if normalized == "" {
		return false
	}
	actionTerms := []string{
		" and run", " then run", " and execute", " then execute",
		" and edit", " then edit", " and create", " then create",
		" and delete", " then delete", " and apply", " then apply",
		" and send", " then send", " and post", " then post",
	}
	for _, term := range actionTerms {
		if strings.Contains(normalized, term+" ") || strings.HasSuffix(normalized, term) {
			return false
		}
	}
	direct := map[string]bool{
		"explain":                 true,
		"explain this":            true,
		"explain it":              true,
		"explain the following":   true,
		"explain this text":       true,
		"explain this content":    true,
		"explain this document":   true,
		"summarize":               true,
		"summarise":               true,
		"summarize this":          true,
		"summarise this":          true,
		"summarize it":            true,
		"summarise it":            true,
		"summarize the following": true,
		"summarise the following": true,
		"analyze":                 true,
		"analyse":                 true,
		"analyze this":            true,
		"analyse this":            true,
		"analyze it":              true,
		"analyse it":              true,
		"analyze the following":   true,
		"analyse the following":   true,
		"review":                  true,
		"review this":             true,
		"review the following":    true,
		"describe":                true,
		"describe this":           true,
		"clarify":                 true,
		"clarify this":            true,
		"translate":               true,
		"translate this":          true,
		"rewrite":                 true,
		"rewrite this":            true,
		"improve":                 true,
		"improve this":            true,
		"make better":             true,
		"make this better":        true,
		"what does this mean":     true,
	}
	if direct[normalized] {
		return true
	}
	prefixes := []string{
		"please explain", "please summarize", "please summarise", "please analyze", "please analyse",
		"please review", "please describe", "please clarify", "please translate", "please rewrite",
		"can you explain", "can you summarize", "can you summarise", "can you analyze", "can you analyse",
		"could you explain", "could you summarize", "could you summarise", "could you analyze", "could you analyse",
	}
	for _, prefix := range prefixes {
		if normalized == prefix || strings.HasPrefix(normalized, prefix+" this") || strings.HasPrefix(normalized, prefix+" the following") {
			return true
		}
	}
	return false
}

func looksLikePastedExplanationBody(body string) bool {
	body = strings.TrimSpace(body)
	if body == "" {
		return false
	}
	normalized := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	nonEmptyLines := 0
	for _, line := range strings.Split(normalized, "\n") {
		if strings.TrimSpace(line) != "" {
			nonEmptyLines++
		}
	}
	if len(normalized) >= 160 || nonEmptyLines >= 3 {
		return true
	}
	if strings.Contains(normalized, "```") || strings.Contains(normalized, "'''") {
		return true
	}
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") ||
			strings.HasPrefix(line, "- ") ||
			strings.HasPrefix(line, "* ") ||
			strings.HasPrefix(line, "+ ") ||
			strings.HasPrefix(line, "> ") {
			return true
		}
	}
	codeMarkers := []string{"<svg", "xmlns=", "</svg>", "<html", "<code", "function ", "const ", "let ", "var "}
	lower := strings.ToLower(normalized)
	for _, marker := range codeMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func LooksExtensionGenerationTask(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	terms := []string{
		"build a tool", "generate a tool", "create a tool", "reusable tool",
		"build an extension", "generate extension", "generate an extension",
		"create extension", "create an extension", "propose extension", "propose the extension",
		"new extension", "missing capability", "does not already have this capability",
		"do not generate it until i approve", "build connector", "generate connector", "create connector",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksConnectorActionTask(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	hasConnectorTarget := ContainsSearchTerm(content, "webhook") ||
		ContainsSearchTerm(content, "connector") ||
		ContainsSearchTerm(content, "slack") ||
		ContainsSearchTerm(content, "telegram") ||
		ContainsSearchTerm(content, "discord") ||
		ContainsSearchTerm(content, "email")
	hasAction := ContainsSearchTerm(content, "post") ||
		ContainsSearchTerm(content, "send") ||
		ContainsSearchTerm(content, "publish") ||
		ContainsSearchTerm(content, "message") ||
		ContainsSearchTerm(content, "notify") ||
		ContainsSearchTerm(content, "tweet")
	return hasConnectorTarget && hasAction
}

func LooksSchedulerCreateTask(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	if LooksSchedulerSetupTemplate(content) {
		return true
	}
	if LooksRecurringScheduleCue(content) {
		return true
	}
	terms := []string{
		"create scheduled job", "create a scheduled job",
		"schedule job", "schedule a job", "scheduled job",
		"create manual job", "create a manual job", "manual job",
		"manual scheduler job", "background job", "recurring job", "cron job",
		"run every hour", "run every day", "run daily", "run weekly",
		"every morning", "every evening", "every weekday",
		"monitor this website every hour", "monitor website every hour",
		"monitor this site every hour", "monitor site every hour",
		"monitor this url every hour", "monitor url every hour",
		"save changes as a scheduled job", "create the scheduler job",
		"create a scheduler", "create scheduler",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksSchedulerSetupTemplate(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	if ContainsSearchTerm(content, "scheduler_setup") || ContainsSearchTerm(content, "scheduler setup") {
		return true
	}
	hasTargetField := strings.Contains(content, "target_type:") ||
		strings.Contains(content, "target type:") ||
		strings.Contains(content, "target-type:") ||
		strings.Contains(content, "target_name:") ||
		strings.Contains(content, "target name:") ||
		strings.Contains(content, "target-name:")
	hasScheduleField := strings.Contains(content, "schedule_type:") ||
		strings.Contains(content, "schedule type:") ||
		strings.Contains(content, "schedule-type:") ||
		strings.Contains(content, "schedule_expr:") ||
		strings.Contains(content, "schedule expr:") ||
		strings.Contains(content, "schedule-expr:") ||
		strings.Contains(content, "every:")
	return hasTargetField && hasScheduleField
}

func LooksWorkflowActionTask(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	if LooksSchedulerCreateTask(content) {
		return true
	}
	if LooksWebsiteMonitoringWorkflowTask(content) {
		return true
	}
	if LooksRecurringScheduleCue(content) {
		return true
	}
	terms := []string{
		"create scheduled job", "schedule job", "scheduled job", "background job",
		"recurring job", "cron job", "run every hour", "run every day", "run daily", "run weekly",
		"checks a webpage every hour", "check a webpage every hour", "webpage every hour",
		"monitor a website", "monitor the website", "monitor this website", "monitor url",
		"monitor this url", "reports if the title changes", "create a scheduler",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksWebsiteMonitoringWorkflowTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" || LooksExplanationOnlyRequest(content) {
		return false
	}
	hasMonitorIntent := ContainsSearchTerm(content, "monitor") ||
		ContainsSearchTerm(content, "watch") ||
		ContainsSearchTerm(content, "keep an eye")
	if !hasMonitorIntent {
		return false
	}
	if _, ok := InternetURLHint(content); !ok &&
		!ContainsSearchTerm(content, "website") &&
		!ContainsSearchTerm(content, "site") &&
		!ContainsSearchTerm(content, "url") &&
		!ContainsSearchTerm(content, "domain") {
		return false
	}
	return true
}

func LooksRecurringScheduleCue(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	for _, term := range []string{
		"every hour", "hourly", "each hour",
		"every day", "daily", "each day",
		"every week", "weekly", "each week",
		"every morning", "every evening", "every weekday",
	} {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	tokens := strings.Fields(content)
	for i := 0; i+2 < len(tokens); i++ {
		if strings.Trim(tokens[i], ",.;:!?()[]{}") != "every" {
			continue
		}
		count := strings.Trim(tokens[i+1], ",.;:!?()[]{}")
		if !looksPositiveIntegerToken(count) {
			continue
		}
		unit := strings.Trim(tokens[i+2], ",.;:!?()[]{}")
		if strings.HasPrefix(unit, "minute") ||
			strings.HasPrefix(unit, "hour") ||
			strings.HasPrefix(unit, "day") ||
			strings.HasPrefix(unit, "week") {
			return true
		}
	}
	return false
}

func looksPositiveIntegerToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r < '0' || r > '9' {
			return false
		}
	}
	return token != "0"
}

func LooksSkillCreationTask(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	terms := []string{"save this workflow", "save workflow", "reusable skill", "create skill", "create a skill", "new skill", "teach yemaka", "learn this workflow"}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksCrawlerTask(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	terms := []string{
		"crawl website", "crawl a website", "crawl the website", "crawl this website",
		"crawl site", "crawl a site", "crawl the site", "crawl this site",
		"crawl url", "crawl this url", "crawler task", "run crawler",
		"follow multiple links", "follow links", "ingest a documentation site",
		"ingest docs site", "ingest documentation site", "check broken links",
		"find broken links", "extract structured site data", "extract data from this site",
		"scrape the site", "scrape this site",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	tokens := WordTokens(content)
	return containsAnyToken(tokens, "crawl", "crawler") && containsAnyToken(tokens, "site", "website", "url", "links", "docs", "documentation")
}

func LooksShellToolTask(content string) bool {
	if LooksExplanationOnlyRequest(content) {
		return false
	}
	normalized := StripSearchIntentPrefix(strings.ToLower(strings.Join(strings.Fields(content), " ")))
	if normalized == "" {
		return false
	}
	for _, term := range []string{"shell command", "run command", "execute command"} {
		if ContainsSearchTerm(normalized, term) {
			return true
		}
	}
	if !(strings.HasPrefix(normalized, "run ") || strings.HasPrefix(normalized, "execute ")) {
		return false
	}
	commandTerms := []string{
		"rm", "sudo", "chmod", "chown", "curl", "wget", "brew", "pip", "pip3",
		"npm", "pnpm", "yarn", "go", "git", "docker", "ssh", "scp", "python",
		"python3", "node", "make",
	}
	tokens := WordTokens(normalized)
	for _, token := range tokens[1:] {
		for _, term := range commandTerms {
			if token == term {
				return true
			}
		}
	}
	return false
}

func LooksInternetTask(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	if LooksDocumentSearchTask(content) || LooksDocumentContextTask(content) {
		return false
	}
	if LooksSpecificCurrentNewsTask(content) {
		return true
	}
	if LooksCurrentWebFactTask(content) {
		return true
	}
	if LooksGenericInternetSearchTask(content) {
		return true
	}
	if _, hasURL := InternetURLHint(content); hasURL {
		return true
	}
	terms := []string{
		"use internet", "using internet", "with internet", "internet access", "check the web",
		"search the web", "on the web", "web search", "internet search", "find online",
		"research online", "fetch url", "check url", "check http", "check https",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksInternetSearchTask(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	if LooksSpecificCurrentNewsTask(content) {
		return true
	}
	if LooksCurrentWebFactTask(content) {
		return true
	}
	if LooksGenericInternetSearchTask(content) {
		return true
	}
	terms := []string{"search the web", "web search", "internet search", "look up", "lookup", "find online", "research online", "latest"}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	_, hasURL := InternetURLHint(content)
	return !hasURL && (ContainsSearchTerm(content, "internet") || ContainsSearchTerm(content, "online") || ContainsSearchTerm(content, "on the web"))
}

func LooksGenericInternetSearchTask(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	content = StripSearchIntentPrefix(content)
	genericSearch := strings.HasPrefix(content, "search ") ||
		strings.HasPrefix(content, "search for ") ||
		strings.HasPrefix(content, "search about ") ||
		strings.HasPrefix(content, "google ") ||
		strings.HasPrefix(content, "look up ")
	if !genericSearch {
		return false
	}
	if looksDocumentScopeCue(content) {
		return false
	}
	return !hasLocalSearchPrefix(content)
}

func hasLocalSearchPrefix(content string) bool {
	localSearchPrefixes := []string{
		"search file", "search files", "search memory", "search memories",
		"search doc", "search docs", "search document", "search documents",
		"search rag", "search workspace", "search project", "search repo",
		"search repository", "search code", "search symbol", "search symbols",
		"search conversation", "search conversations", "search chat", "search chats",
	}
	for _, prefix := range localSearchPrefixes {
		if content == prefix || strings.HasPrefix(content, prefix+" ") {
			return true
		}
	}
	return false
}

func LooksCurrentWebFactTask(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" || LooksBroadCurrentNewsTask(content) || LooksSpecificCurrentNewsTask(content) {
		return false
	}
	normalized := StripSearchIntentPrefix(content)
	if normalized == "" {
		return false
	}
	if hasLocalSearchPrefix(normalized) {
		return false
	}
	tokens := WordTokens(strings.NewReplacer("-", " ", "–", " ", "—", " ", ".", " ").Replace(normalized))
	hasCurrentSignal := containsAnyToken(tokens,
		"latest", "current", "new", "newest", "release", "released", "releases",
		"version", "versions", "update", "updates", "launch", "launched",
		"available", "availability",
	) || ContainsSearchTerm(normalized, "release date") ||
		ContainsSearchTerm(normalized, "be released") ||
		ContainsSearchTerm(normalized, "latest version") ||
		ContainsSearchTerm(normalized, "current version")
	if !hasCurrentSignal {
		return false
	}
	hasQuestionSignal := strings.HasPrefix(normalized, "when ") ||
		strings.HasPrefix(normalized, "what ") ||
		strings.HasPrefix(normalized, "which ") ||
		strings.HasPrefix(normalized, "is ") ||
		strings.HasPrefix(normalized, "has ") ||
		strings.HasPrefix(normalized, "latest ") ||
		strings.HasPrefix(normalized, "current ") ||
		strings.HasPrefix(normalized, "new ") ||
		strings.HasPrefix(normalized, "search ") ||
		strings.HasPrefix(normalized, "look up ") ||
		strings.HasPrefix(normalized, "lookup ")
	if !hasQuestionSignal && !ContainsSearchTerm(normalized, "latest version") && !ContainsSearchTerm(normalized, "current version") {
		return false
	}
	return hasCurrentWebFactSubject(tokens)
}

func hasCurrentWebFactSubject(tokens []string) bool {
	for _, token := range tokens {
		token = strings.Trim(token, "_-")
		if token == "" || isCurrentWebFactFillerToken(token) {
			continue
		}
		if len(token) >= 2 && tokenHasLetter(token) {
			return true
		}
	}
	return false
}

func isCurrentWebFactFillerToken(token string) bool {
	switch token {
	case "a", "an", "and", "are", "as", "at", "be", "been", "by", "can", "could", "did", "do", "does", "for", "from", "has", "have", "how", "i", "if", "in", "is", "it", "its", "me", "my", "of", "on", "or", "please", "should", "that", "the", "their", "this", "to", "up", "we", "what", "when", "which", "will", "with", "you", "your":
		return true
	case "available", "availability", "current", "date", "latest", "launch", "launched", "new", "newest", "release", "released", "releases", "update", "updates", "version", "versions":
		return true
	case "agent", "app", "application", "config", "configuration", "file", "files", "mode", "model", "models", "package", "profile", "project", "repo", "repository", "settings", "software", "status", "system", "workspace", "yemaka":
		return true
	case "about", "find", "google", "look", "lookup", "online", "research", "search", "web":
		return true
	case "info", "information", "more", "notes", "result", "results", "summary":
		return true
	default:
		return false
	}
}

func tokenHasLetter(token string) bool {
	for _, r := range token {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func LooksLocalTimeTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	falsePositives := []string{"time complexity", "runtime", "timeline", "time series", "compile time", "response time"}
	for _, term := range falsePositives {
		if ContainsSearchTerm(content, term) {
			return false
		}
	}
	phrases := []string{
		"what time is it",
		"what is the time",
		"what's the time",
		"time of the day",
		"current time",
		"local time",
		"system time",
		"what day is it",
		"what date is it",
		"date and time",
	}
	for _, phrase := range phrases {
		if ContainsSearchTerm(content, phrase) {
			return true
		}
	}
	tokens := WordTokens(content)
	return containsAnyToken(tokens, "time", "date") && containsAnyToken(tokens, "now", "current", "local", "today")
}

type LocalTimeTarget struct {
	Label                 string
	TimeZone              string
	Ambiguous             bool
	ClarificationQuestion string
}

func ResolveLocalTimeTarget(content string) LocalTimeTarget {
	if target, ok := findIanaLocalTimeTarget(content); ok {
		return target
	}
	normalized := normalizeLocationText(content)
	if looksDocumentScopeCue(normalized) {
		return LocalTimeTarget{}
	}
	if normalized == "" || (!LooksLocalTimeTask(normalized) && !looksLocalTimeFollowupCue(normalized)) {
		return LocalTimeTarget{}
	}
	for _, target := range knownLocalTimeTargets() {
		for _, alias := range target.aliases {
			if containsLocationAlias(normalized, alias) {
				return LocalTimeTarget{Label: target.label, TimeZone: target.timeZone}
			}
		}
	}
	for _, target := range ambiguousLocalTimeTargets() {
		for _, alias := range target.aliases {
			if containsLocationAlias(normalized, alias) {
				return LocalTimeTarget{
					Label:                 target.label,
					Ambiguous:             true,
					ClarificationQuestion: target.question,
				}
			}
		}
	}
	return LocalTimeTarget{}
}

func looksLocalTimeFollowupCue(content string) bool {
	return ContainsSearchTerm(content, "not my locale") ||
		ContainsSearchTerm(content, "not my local") ||
		ContainsSearchTerm(content, "time there") ||
		ContainsSearchTerm(content, "i meant") ||
		ContainsSearchTerm(content, "you mean") ||
		ContainsSearchTerm(content, "What is")
}

func LooksAmbiguousLocalTimeTask(content string) bool {
	return ResolveLocalTimeTarget(content).Ambiguous
}

func LooksKnownLocalTimeTargetTask(content string) bool {
	return ResolveLocalTimeTarget(content).TimeZone != ""
}

func LooksUnresolvedLocalTimeTargetTask(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	normalized := normalizeLocationText(content)
	if looksDocumentScopeCue(normalized) {
		return false
	}
	if normalized == "" || (!LooksLocalTimeTask(normalized) && !looksLocalTimeFollowupCue(normalized)) {
		return false
	}
	target := ResolveLocalTimeTarget(content)
	if target.TimeZone != "" || target.Ambiguous {
		return false
	}
	return unresolvedLocalTimeLocationHint(content) != ""
}

func LocalTimeClarificationQuestion(content string) string {
	target := ResolveLocalTimeTarget(content)
	if strings.TrimSpace(target.ClarificationQuestion) != "" {
		return target.ClarificationQuestion
	}
	if strings.TrimSpace(target.Label) != "" {
		return "Which city or time zone should I use for " + target.Label + "?"
	}
	if hint := unresolvedLocalTimeLocationHint(content); hint != "" {
		return "I can answer from the local system clock for known time zones, but I do not have a reliable timezone mapping for " + hint + ". Which city or IANA time zone should I use?"
	}
	return "Which city or time zone should I use?"
}

type localTimeTargetRule struct {
	label    string
	timeZone string
	aliases  []string
}

type ambiguousLocalTimeTargetRule struct {
	label    string
	aliases  []string
	question string
}

func knownLocalTimeTargets() []localTimeTargetRule {
	targets := make([]localTimeTargetRule, 0, len(ianaTimeZoneLabels)+len(staticLocalTimeTargets()))
	targets = append(targets, staticLocalTimeTargets()...)
	for _, zone := range ianaTimeZoneLabels {
		targets = append(targets, localTimeTargetRule{
			label:    zone,
			timeZone: zone,
			aliases:  aliasesForIanaZone(zone),
		})
	}
	return targets
}

var ianaTimeZoneLabels = []string{
	"Africa/Abidjan",
	"Africa/Accra",
	"Africa/Addis_Ababa",
	"Africa/Algiers",
	"Africa/Asmara",
	"Africa/Bamako",
	"Africa/Bangui",
	"Africa/Banjul",
	"Africa/Bissau",
	"Africa/Blantyre",
	"Africa/Brazzaville",
	"Africa/Bujumbura",
	"Africa/Cairo",
	"Africa/Casablanca",
	"Africa/Ceuta",
	"Africa/Conakry",
	"Africa/Dakar",
	"Africa/Dar_es_Salaam",
	"Africa/Djibouti",
	"Africa/Douala",
	"Africa/El_Aaiun",
	"Africa/Freetown",
	"Africa/Gaborone",
	"Africa/Harare",
	"Africa/Johannesburg",
	"Africa/Juba",
	"Africa/Kampala",
	"Africa/Khartoum",
	"Africa/Kigali",
	"Africa/Kinshasa",
	"Africa/Lagos",
	"Africa/Libreville",
	"Africa/Lome",
	"Africa/Luanda",
	"Africa/Lubumbashi",
	"Africa/Lusaka",
	"Africa/Malabo",
	"Africa/Maputo",
	"Africa/Maseru",
	"Africa/Mbabane",
	"Africa/Mogadishu",
	"Africa/Monrovia",
	"Africa/Nairobi",
	"Africa/Ndjamena",
	"Africa/Niamey",
	"Africa/Nouakchott",
	"Africa/Ouagadougou",
	"Africa/Porto-Novo",
	"Africa/Sao_Tome",
	"Africa/Tripoli",
	"Africa/Tunis",
	"Africa/Windhoek",

	"America/Adak",
	"America/Anchorage",
	"America/Anguilla",
	"America/Antigua",
	"America/Araguaina",
	"America/Argentina/Buenos_Aires",
	"America/Argentina/Catamarca",
	"America/Argentina/Cordoba",
	"America/Argentina/Jujuy",
	"America/Argentina/La_Rioja",
	"America/Argentina/Mendoza",
	"America/Argentina/Rio_Gallegos",
	"America/Argentina/Salta",
	"America/Argentina/San_Juan",
	"America/Argentina/San_Luis",
	"America/Argentina/Tucuman",
	"America/Argentina/Ushuaia",
	"America/Aruba",
	"America/Asuncion",
	"America/Atikokan",
	"America/Bahia",
	"America/Bahia_Banderas",
	"America/Barbados",
	"America/Belem",
	"America/Belize",
	"America/Blanc-Sablon",
	"America/Boa_Vista",
	"America/Bogota",
	"America/Boise",
	"America/Cambridge_Bay",
	"America/Campo_Grande",
	"America/Cancun",
	"America/Caracas",
	"America/Cayenne",
	"America/Cayman",
	"America/Chicago",
	"America/Chihuahua",
	"America/Costa_Rica",
	"America/Creston",
	"America/Cuiaba",
	"America/Curacao",
	"America/Danmarkshavn",
	"America/Dawson",
	"America/Dawson_Creek",
	"America/Denver",
	"America/Detroit",
	"America/Dominica",
	"America/Edmonton",
	"America/Eirunepe",
	"America/El_Salvador",
	"America/Fort_Nelson",
	"America/Fortaleza",
	"America/Glace_Bay",
	"America/Godthab",
	"America/Goose_Bay",
	"America/Grand_Turk",
	"America/Grenada",
	"America/Guadeloupe",
	"America/Guatemala",
	"America/Guayaquil",
	"America/Guyana",
	"America/Halifax",
	"America/Havana",
	"America/Hermosillo",
	"America/Indiana/Indianapolis",
	"America/Indiana/Knox",
	"America/Indiana/Marengo",
	"America/Indiana/Petersburg",
	"America/Indiana/Tell_City",
	"America/Indiana/Vevay",
	"America/Indiana/Vincennes",
	"America/Indiana/Winamac",
	"America/Inuvik",
	"America/Iqaluit",
	"America/Jamaica",
	"America/Juneau",
	"America/Kentucky/Louisville",
	"America/Kentucky/Monticello",
	"America/Kralendijk",
	"America/La_Paz",
	"America/Lima",
	"America/Los_Angeles",
	"America/Lower_Princes",
	"America/Maceio",
	"America/Managua",
	"America/Manaus",
	"America/Marigot",
	"America/Martinique",
	"America/Matamoros",
	"America/Mazatlan",
	"America/Menominee",
	"America/Merida",
	"America/Metlakatla",
	"America/Mexico_City",
	"America/Miquelon",
	"America/Moncton",
	"America/Monterrey",
	"America/Montevideo",
	"America/Montserrat",
	"America/Nassau",
	"America/New_York",
	"America/Nipigon",
	"America/Nome",
	"America/Noronha",
	"America/North_Dakota/Beulah",
	"America/North_Dakota/Center",
	"America/North_Dakota/New_Salem",
	"America/Nuuk",
	"America/Ojinaga",
	"America/Panama",
	"America/Pangnirtung",
	"America/Paramaribo",
	"America/Phoenix",
	"America/Port-au-Prince",
	"America/Port_of_Spain",
	"America/Porto_Velho",
	"America/Puerto_Rico",
	"America/Punta_Arenas",
	"America/Rainy_River",
	"America/Rankin_Inlet",
	"America/Recife",
	"America/Regina",
	"America/Resolute",
	"America/Rio_Branco",
	"America/Santarem",
	"America/Santiago",
	"America/Santo_Domingo",
	"America/Sao_Paulo",
	"America/Scoresbysund",
	"America/Sitka",
	"America/St_Barthelemy",
	"America/St_Johns",
	"America/St_Kitts",
	"America/St_Lucia",
	"America/St_Thomas",
	"America/St_Vincent",
	"America/Swift_Current",
	"America/Tegucigalpa",
	"America/Thule",
	"America/Thunder_Bay",
	"America/Tijuana",
	"America/Toronto",
	"America/Tortola",
	"America/Vancouver",
	"America/Whitehorse",
	"America/Winnipeg",
	"America/Yakutat",
	"America/Yellowknife",

	"Antarctica/Casey",
	"Antarctica/Davis",
	"Antarctica/DumontDUrville",
	"Antarctica/Macquarie",
	"Antarctica/Mawson",
	"Antarctica/McMurdo",
	"Antarctica/Palmer",
	"Antarctica/Rothera",
	"Antarctica/South_Pole",
	"Antarctica/Syowa",
	"Antarctica/Troll",
	"Antarctica/Vostok",

	"Arctic/Longyearbyen",

	"Asia/Aden",
	"Asia/Almaty",
	"Asia/Amman",
	"Asia/Anadyr",
	"Asia/Aqtau",
	"Asia/Aqtobe",
	"Asia/Ashgabat",
	"Asia/Atyrau",
	"Asia/Baghdad",
	"Asia/Bahrain",
	"Asia/Baku",
	"Asia/Bangkok",
	"Asia/Barnaul",
	"Asia/Beirut",
	"Asia/Bishkek",
	"Asia/Brunei",
	"Asia/Chita",
	"Asia/Choibalsan",
	"Asia/Chongqing",
	"Asia/Chungking",
	"Asia/Colombo",
	"Asia/Damascus",
	"Asia/Dhaka",
	"Asia/Dili",
	"Asia/Dubai",
	"Asia/Dushanbe",
	"Asia/Famagusta",
	"Asia/Gaza",
	"Asia/Hebron",
	"Asia/Ho_Chi_Minh",
	"Asia/Hong_Kong",
	"Asia/Hovd",
	"Asia/Irkutsk",
	"Asia/Jakarta",
	"Asia/Jayapura",
	"Asia/Jerusalem",
	"Asia/Kabul",
	"Asia/Kamchatka",
	"Asia/Karachi",
	"Asia/Kathmandu",
	"Asia/Khandyga",
	"Asia/Kolkata",
	"Asia/Krasnoyarsk",
	"Asia/Kuala_Lumpur",
	"Asia/Kuching",
	"Asia/Macau",
	"Asia/Magadan",
	"Asia/Makassar",
	"Asia/Manila",
	"Asia/Muscat",
	"Asia/Nicosia",
	"Asia/Novokuznetsk",
	"Asia/Novosibirsk",
	"Asia/Omsk",
	"Asia/Oral",
	"Asia/Phnom_Penh",
	"Asia/Pontianak",
	"Asia/Pyongyang",
	"Asia/Qatar",
	"Asia/Qostanay",
	"Asia/Qyzylorda",
	"Asia/Riyadh",
	"Asia/Sakhalin",
	"Asia/Samarkand",
	"Asia/Seoul",
	"Asia/Shanghai",
	"Asia/Singapore",
	"Asia/Srednekolymsk",
	"Asia/Taipei",
	"Asia/Tashkent",
	"Asia/Tbilisi",
	"Asia/Tehran",
	"Asia/Thimphu",
	"Asia/Tokyo",
	"Asia/Tomsk",
	"Asia/Ulaanbaatar",
	"Asia/Urumqi",
	"Asia/Ust-Nera",
	"Asia/Vientiane",
	"Asia/Vladivostok",
	"Asia/Yakutsk",
	"Asia/Yangon",
	"Asia/Yekaterinburg",

	"Atlantic/Azores",
	"Atlantic/Bermuda",
	"Atlantic/Canary",
	"Atlantic/Cape_Verde",
	"Atlantic/Faroe",
	"Atlantic/Madeira",
	"Atlantic/Reykjavik",
	"Atlantic/South_Georgia",
	"Atlantic/St_Helena",
	"Atlantic/Stanley",

	"Australia/Adelaide",
	"Australia/Brisbane",
	"Australia/Broken_Hill",
	"Australia/Currie",
	"Australia/Darwin",
	"Australia/Eucla",
	"Australia/Hobart",
	"Australia/Lindeman",
	"Australia/Lord_Howe",
	"Australia/Melbourne",
	"Australia/Perth",
	"Australia/Sydney",

	"Europe/Amsterdam",
	"Europe/Andorra",
	"Europe/Astrakhan",
	"Europe/Athens",
	"Europe/Belfast",
	"Europe/Belgrade",
	"Europe/Berlin",
	"Europe/Bratislava",
	"Europe/Brussels",
	"Europe/Bucharest",
	"Europe/Budapest",
	"Europe/Busingen",
	"Europe/Chisinau",
	"Europe/Copenhagen",
	"Europe/Dublin",
	"Europe/Gibraltar",
	"Europe/Guernsey",
	"Europe/Helsinki",
	"Europe/Isle_of_Man",
	"Europe/Istanbul",
	"Europe/Jersey",
	"Europe/Kaliningrad",
	"Europe/Kiev",
	"Europe/Kirov",
	"Europe/Lisbon",
	"Europe/Ljubljana",
	"Europe/London",
	"Europe/Luxembourg",
	"Europe/Madrid",
	"Europe/Malta",
	"Europe/Mariehamn",
	"Europe/Minsk",
	"Europe/Monaco",
	"Europe/Moscow",
	"Europe/Nicosia",
	"Europe/Oslo",
	"Europe/Paris",
	"Europe/Podgorica",
	"Europe/Prague",
	"Europe/Riga",
	"Europe/Rome",
	"Europe/Samara",
	"Europe/San_Marino",
	"Europe/Sarajevo",
	"Europe/Simferopol",
	"Europe/Skopje",
	"Europe/Sofia",
	"Europe/Stockholm",
	"Europe/Tallinn",
	"Europe/Tirane",
	"Europe/Tiraspol",
	"Europe/Ulyanovsk",
	"Europe/Uzhgorod",
	"Europe/Vaduz",
	"Europe/Vatican",
	"Europe/Vienna",
	"Europe/Vilnius",
	"Europe/Volgograd",
	"Europe/Warsaw",
	"Europe/Zagreb",
	"Europe/Zaporozhye",

	"Indian/Antananarivo",
	"Indian/Chagos",
	"Indian/Christmas",
	"Indian/Cocos",
	"Indian/Comoro",
	"Indian/Kerguelen",
	"Indian/Mahe",
	"Indian/Maldives",
	"Indian/Mauritius",
	"Indian/Mayotte",
	"Indian/Reunion",

	"Pacific/Apia",
	"Pacific/Auckland",
	"Pacific/Bougainville",
	"Pacific/Chatham",
	"Pacific/Chuuk",
	"Pacific/Easter",
	"Pacific/Efate",
	"Pacific/Enderbury",
	"Pacific/Fakaofo",
	"Pacific/Fiji",
	"Pacific/Funafuti",
	"Pacific/Galapagos",
	"Pacific/Gambier",
	"Pacific/Guadalcanal",
	"Pacific/Guam",
	"Pacific/Honolulu",
	"Pacific/Johnston",
	"Pacific/Kiritimati",
	"Pacific/Kosrae",
	"Pacific/Kwajalein",
	"Pacific/Majuro",
	"Pacific/Marquesas",
	"Pacific/Midway",
	"Pacific/Nauru",
	"Pacific/Niue",
	"Pacific/Norfolk",
	"Pacific/Noumea",
	"Pacific/Pitcairn",
	"Pacific/Pohnpei",
	"Pacific/Port_Moresby",
	"Pacific/Rarotonga",
	"Pacific/Saipan",
	"Pacific/Tahiti",
	"Pacific/Tarawa",
	"Pacific/Tongatapu",
	"Pacific/Wake",
	"Pacific/Wallis",
	"Pacific/Yap",

	"UTC",
	"Etc/GMT",
	"Etc/GMT+0",
	"Etc/GMT-0",
	"Etc/GMT+1",
	"Etc/GMT+2",
	"Etc/GMT+3",
	"Etc/GMT+4",
	"Etc/GMT+5",
	"Etc/GMT+6",
	"Etc/GMT+7",
	"Etc/GMT+8",
	"Etc/GMT+9",
	"Etc/GMT+10",
	"Etc/GMT+11",
	"Etc/GMT+12",
	"Etc/GMT+13",
	"Etc/GMT+14",
	"Etc/GMT-1",
	"Etc/GMT-2",
	"Etc/GMT-3",
	"Etc/GMT-4",
	"Etc/GMT-5",
	"Etc/GMT-6",
	"Etc/GMT-7",
	"Etc/GMT-8",
	"Etc/GMT-9",
	"Etc/GMT-10",
	"Etc/GMT-11",
	"Etc/GMT-12",
	"Etc/UCT",
	"Etc/UTC",
	"Etc/Universal",
	"Etc/Zulu",
	"SystemV/AST4ADT",
	"SystemV/CST6CDT",
	"SystemV/EST5EDT",
	"SystemV/HST10",
	"SystemV/MST7MDT",
	"SystemV/PST8PDT",
	"SystemV/YST9YDT",
	"SystemV/EST5",
	"SystemV/CST6",
	"SystemV/MST7",
	"SystemV/PST8",
	"SystemV/YST9",
	"US/Aleutian",
	"US/Arizona",
	"US/Central",
	"US/East-Indiana",
	"US/Eastern",
	"US/Hawaii",
	"US/Mountain",
	"US/Pacific",
	"US/Samoa",
	"Canada/Atlantic",
	"Canada/Central",
	"Canada/Eastern",
	"Canada/Mountain",
	"Canada/Newfoundland",
	"Canada/Pacific",
	"Canada/Saskatchewan",
	"Canada/Yukon",
	"Mexico/BajaNorte",
	"Mexico/BajaSur",
	"Mexico/General",
	"Brazil/Acre",
	"Brazil/DeNoronha",
	"Brazil/East",
	"Brazil/West",
	"Chile/Continental",
	"Chile/EasterIsland",
}

func aliasesForIanaZone(zone string) []string {
	aliases := []string{zone}
	normalized := strings.ReplaceAll(strings.ReplaceAll(zone, "_", " "), "/", " ")
	aliases = append(aliases, normalized)
	parts := strings.Split(zone, "/")
	for i := 0; i < len(parts); i++ {
		aliases = append(aliases, strings.ReplaceAll(strings.Join(parts[i:], " "), "_", " "))
	}
	return uniqueStrings(aliases)
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func staticLocalTimeTargets() []localTimeTargetRule {
	return []localTimeTargetRule{
		{label: "Edmonton, Alberta, Canada", timeZone: "America/Edmonton", aliases: []string{"edmonton", "edmonton alberta", "edmonton canada", "edmonton alberta canada"}},
		{label: "Calgary, Alberta, Canada", timeZone: "America/Edmonton", aliases: []string{"calgary", "calgary alberta", "calgary canada", "calgary alberta canada"}},
		{label: "Alberta, Canada", timeZone: "America/Edmonton", aliases: []string{"alberta", "alberta canada"}},
		{label: "Vancouver, British Columbia, Canada", timeZone: "America/Vancouver", aliases: []string{"vancouver", "vancouver canada", "vancouver british columbia", "british columbia", "bc canada"}},
		{label: "Toronto, Ontario, Canada", timeZone: "America/Toronto", aliases: []string{"toronto", "toronto canada", "toronto ontario"}},
		{label: "Ottawa, Ontario, Canada", timeZone: "America/Toronto", aliases: []string{"ottawa", "ottawa canada", "ottawa ontario"}},
		{label: "Montreal, Quebec, Canada", timeZone: "America/Toronto", aliases: []string{"montreal", "montreal canada", "montreal quebec", "montréal", "montréal canada"}},
		{label: "Winnipeg, Manitoba, Canada", timeZone: "America/Winnipeg", aliases: []string{"winnipeg", "winnipeg canada", "winnipeg manitoba", "manitoba", "manitoba canada"}},
		{label: "Halifax, Nova Scotia, Canada", timeZone: "America/Halifax", aliases: []string{"halifax", "halifax canada", "halifax nova scotia", "nova scotia", "nova scotia canada"}},
		{label: "St. John's, Newfoundland and Labrador, Canada", timeZone: "America/St_Johns", aliases: []string{"st johns", "st john's", "saint johns", "newfoundland", "newfoundland canada", "newfoundland and labrador"}},
		{label: "Saskatchewan, Canada", timeZone: "America/Regina", aliases: []string{"saskatchewan", "saskatchewan canada", "regina", "saskatoon"}},
		{label: "Yukon, Canada", timeZone: "America/Whitehorse", aliases: []string{"yukon", "yukon canada", "whitehorse"}},
		{label: "London, Ontario, Canada", timeZone: "America/Toronto", aliases: []string{"london ontario", "london ontario canada", "london canada"}},
		{label: "London, United Kingdom", timeZone: "Europe/London", aliases: []string{"london", "london uk", "london england", "london united kingdom", "united kingdom", "uk time", "britain"}},
		{label: "Paris, France", timeZone: "Europe/Paris", aliases: []string{"paris", "paris france", "france"}},
		{label: "Berlin, Germany", timeZone: "Europe/Berlin", aliases: []string{"berlin", "berlin germany", "germany"}},
		{label: "Lagos, Nigeria", timeZone: "Africa/Lagos", aliases: []string{"lagos", "lagos nigeria", "nigeria"}},
		{label: "New York, United States", timeZone: "America/New_York", aliases: []string{"new york", "new york city", "nyc"}},
		{label: "Los Angeles, United States", timeZone: "America/Los_Angeles", aliases: []string{"los angeles", "la california", "california"}},
		{label: "Chicago, United States", timeZone: "America/Chicago", aliases: []string{"chicago"}},
		{label: "Denver, United States", timeZone: "America/Denver", aliases: []string{"denver"}},
		{label: "Tokyo, Japan", timeZone: "Asia/Tokyo", aliases: []string{"tokyo", "tokyo japan", "japan"}},
		{label: "Singapore", timeZone: "Asia/Singapore", aliases: []string{"singapore"}},
		{label: "Beijing, China", timeZone: "Asia/Shanghai", aliases: []string{"beijing", "beijing china", "china", "shanghai", "shanghai china"}},
		{label: "New Delhi, India", timeZone: "Asia/Kolkata", aliases: []string{"new delhi", "new delhi india", "delhi", "india", "mumbai", "mumbai india"}},
		{label: "Hong Kong", timeZone: "Asia/Hong_Kong", aliases: []string{"hong kong", "hong kong china"}},
		{label: "Seoul, South Korea", timeZone: "Asia/Seoul", aliases: []string{"seoul", "seoul korea", "south korea", "korea"}},
		{label: "Bangkok, Thailand", timeZone: "Asia/Bangkok", aliases: []string{"bangkok", "bangkok thailand", "thailand"}},
		{label: "Manila, Philippines", timeZone: "Asia/Manila", aliases: []string{"manila", "manila philippines", "philippines"}},
		{label: "Jakarta, Indonesia", timeZone: "Asia/Jakarta", aliases: []string{"jakarta", "jakarta indonesia", "indonesia"}},
		{label: "Dubai, United Arab Emirates", timeZone: "Asia/Dubai", aliases: []string{"dubai", "dubai uae", "uae", "united arab emirates"}},
		{label: "Doha, Qatar", timeZone: "Asia/Qatar", aliases: []string{"doha", "doha qatar", "qatar"}},
		{label: "Riyadh, Saudi Arabia", timeZone: "Asia/Riyadh", aliases: []string{"riyadh", "riyadh saudi arabia", "saudi arabia", "saudi"}},
		{label: "Nicosia, Cyprus", timeZone: "Asia/Nicosia", aliases: []string{"nicosia", "cyprus"}},
		{label: "Istanbul, Turkey", timeZone: "Europe/Istanbul", aliases: []string{"istanbul", "istanbul turkey", "turkey"}},
		{label: "Cairo, Egypt", timeZone: "Africa/Cairo", aliases: []string{"cairo", "cairo egypt", "egypt"}},
		{label: "Johannesburg, South Africa", timeZone: "Africa/Johannesburg", aliases: []string{"johannesburg", "johannesburg south africa", "south africa"}},
		{label: "Accra, Ghana", timeZone: "Africa/Accra", aliases: []string{"accra", "accra ghana", "ghana"}},
		{label: "Nairobi, Kenya", timeZone: "Africa/Nairobi", aliases: []string{"nairobi", "nairobi kenya", "kenya"}},
		{label: "Sydney, Australia", timeZone: "Australia/Sydney", aliases: []string{"sydney", "sydney australia"}},
		{label: "Melbourne, Australia", timeZone: "Australia/Melbourne", aliases: []string{"melbourne", "melbourne australia"}},
		{label: "Brisbane, Australia", timeZone: "Australia/Brisbane", aliases: []string{"brisbane", "brisbane australia"}},
		{label: "Perth, Australia", timeZone: "Australia/Perth", aliases: []string{"perth", "perth australia"}},
		{label: "Rio de Janeiro, Brazil", timeZone: "America/Sao_Paulo", aliases: []string{"rio de janeiro", "rio", "rio brazil"}},
		{label: "Mexico City, Mexico", timeZone: "America/Mexico_City", aliases: []string{"mexico city"}},
		{label: "Moscow, Russia", timeZone: "Europe/Moscow", aliases: []string{"moscow", "moscow russia"}},
		{label: "Saint Petersburg, Russia", timeZone: "Europe/Moscow", aliases: []string{"saint petersburg", "st petersburg", "petersburg"}},
		{label: "UTC", timeZone: "UTC", aliases: []string{"utc", "gmt"}},
	}
}

func ambiguousLocalTimeTargets() []ambiguousLocalTimeTargetRule {
	return []ambiguousLocalTimeTargetRule{
		{label: "Canada", aliases: []string{"canada", "canadian"}, question: "Canada spans multiple time zones. Which city, province, or time zone should I use? For example: Edmonton, Toronto, Vancouver, Winnipeg, Halifax, or St. John's."},
		{label: "United States", aliases: []string{"united states", "usa", "us time", "america"}, question: "The United States spans multiple time zones. Which city, state, or time zone should I use?"},
		{label: "Australia", aliases: []string{"australia"}, question: "Australia spans multiple time zones. Which city, state, or time zone should I use?"},
		{label: "Brazil", aliases: []string{"brazil"}, question: "Brazil spans multiple time zones. Which city, state, or time zone should I use?"},
		{label: "Mexico", aliases: []string{"mexico"}, question: "Mexico spans multiple time zones. Which city, state, or time zone should I use?"},
		{label: "Russia", aliases: []string{"russia"}, question: "Russia spans many time zones. Which city, region, or time zone should I use?"},
	}
}

func normalizeLocationText(content string) string {
	content = strings.ToLower(content)
	var builder strings.Builder
	lastSpace := true
	for _, r := range content {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			builder.WriteRune(r)
			lastSpace = false
		case r == '\'':
			continue
		default:
			if !lastSpace {
				builder.WriteByte(' ')
				lastSpace = true
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func containsLocationAlias(content string, alias string) bool {
	alias = normalizeLocationText(alias)
	if alias == "" {
		return false
	}
	return ContainsSearchTerm(content, alias)
}

func findIanaLocalTimeTarget(content string) (LocalTimeTarget, bool) {
	for _, field := range strings.Fields(content) {
		candidate := strings.Trim(field, ".,:;()[]{}\"'<>?!")
		if candidate == "" || strings.Contains(candidate, "://") || !strings.Contains(candidate, "/") {
			continue
		}
		if _, err := time.LoadLocation(candidate); err == nil {
			return LocalTimeTarget{Label: candidate, TimeZone: candidate}, true
		}
	}
	return LocalTimeTarget{}, false
}

func unresolvedLocalTimeLocationHint(content string) string {
	normalized := normalizeLocationText(StripSearchIntentPrefix(strings.ToLower(strings.TrimSpace(content))))
	if normalized == "" {
		return ""
	}
	prefixes := []string{
		"what is the time in ",
		"whats the time in ",
		"what s the time in ",
		"what time is it in ",
		"current time in ",
		"local time in ",
		"time in ",
		"i meant ",
		"you mean ",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			return cleanLocalTimeLocationHint(strings.TrimPrefix(normalized, prefix))
		}
	}
	markers := []string{
		" what is the time there",
		" what time is it there",
		" time there",
		" not my locale",
		" not my local",
	}
	for _, marker := range markers {
		if idx := strings.Index(normalized, marker); idx > 0 {
			return cleanLocalTimeLocationHint(normalized[:idx])
		}
	}
	return ""
}

func cleanLocalTimeLocationHint(value string) string {
	value = strings.TrimSpace(value)
	for _, suffix := range []string{" please", " now", " today", " right now"} {
		value = strings.TrimSpace(strings.TrimSuffix(value, suffix))
	}
	value = strings.TrimSpace(strings.Trim(value, " .,!?:;"))
	switch value {
	case "", "here", "there", "my locale", "my local", "local", "system":
		return ""
	default:
		return value
	}
}

func LooksBroadCurrentNewsTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	hasSubject := hasSpecificNewsSubject(content)
	broadPhrases := []string{
		"what's happening today in the world",
		"what is happening today in the world",
		"what's happening in the world today",
		"what is happening in the world today",
		"what is going on in the world today",
		"what's going on in the world today",
		"what is going on today",
		"what's going on today",
		"what's happening today",
		"what is happening today",
		"world news today",
		"today's world news",
		"today news",
		"today's news",
		"latest news",
		"breaking news",
		"what happened today",
		"what's happened today",
		"what happened in the world today",
		"what's happened in the world today",
		"current events today",
		"latest world news",
	}
	for _, phrase := range broadPhrases {
		if ContainsSearchTerm(content, phrase) {
			return !hasSubject
		}
	}
	if (ContainsWord(content, "news") || ContainsSearchTerm(content, "current events")) &&
		containsAnyToken(WordTokens(content), "today", "world") &&
		!hasSpecificNewsSubject(content) {
		return true
	}
	return false
}

func LooksSpecificCurrentNewsTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" || LooksBroadCurrentNewsTask(content) {
		return false
	}
	hasNewsSignal := ContainsWord(content, "news") ||
		ContainsWord(content, "breaking") ||
		ContainsSearchTerm(content, "current events") ||
		ContainsSearchTerm(content, "latest news") ||
		ContainsSearchTerm(content, "today in") ||
		ContainsSearchTerm(content, "visit to")
	if !hasNewsSignal {
		return false
	}
	return hasSpecificNewsSubject(content)
}

func CurrentNewsSearchQuery(content string) string {
	if !LooksSpecificCurrentNewsTask(content) {
		return strings.TrimSpace(content)
	}
	normalized := strings.NewReplacer("-", " ", "–", " ", "—", " ").Replace(strings.ToLower(content))
	tokens := WordTokens(normalized)
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "about": true,
		"breaking": true, "current": true, "events": true, "for": true,
		"in": true, "is": true, "latest": true, "news": true, "of": true,
		"on": true, "please": true, "s": true, "search": true, "the": true, "to": true,
		"today": true, "what": true, "whats": true, "what's": true, "with": true,
	}
	seen := map[string]bool{}
	keywords := make([]string, 0, len(tokens)+2)
	for _, token := range tokens {
		if stop[token] || seen[token] {
			continue
		}
		seen[token] = true
		keywords = append(keywords, token)
	}
	if len(keywords) == 0 {
		return strings.TrimSpace(content)
	}
	now := time.Now()
	keywords = append(keywords, now.Format("2006"), now.Format("2006-01-02"), "latest", "news")
	return strings.Join(keywords, " ")
}

func hasSpecificNewsSubject(content string) bool {
	tokens := WordTokens(strings.NewReplacer("-", " ", "–", " ", "—", " ").Replace(content))
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "around": true,
		"breaking": true, "current": true, "events": true, "for": true,
		"happening": true, "in": true, "is": true, "latest": true, "news": true,
		"me": true, "of": true, "on": true, "s": true, "show": true, "so": true,
		"tell": true, "the": true, "today": true, "to": true,
		"visit": true, "what": true, "whats": true, "world": true,
	}
	for _, token := range tokens {
		if !stop[token] {
			return true
		}
	}
	return false
}

func LooksRAGTask(content string) bool {
	terms := []string{"use my docs", "using my docs", "using local docs", "use my local documents", "using my local documents", "from my docs", "from my documents", "from local documents", "from ingested", "from indexed", "in ingested documents", "in indexed documents", "in the documents", "according to the docs", "according to the retrieved", "in the retrieved", "from the retrieved"}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksDocumentSearchTask(content string) bool {
	terms := []string{"search docs", "search document", "search documents", "search indexed document", "search indexed documents", "search ingested document", "search ingested documents", "search rag", "search my docs", "search local docs"}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksDocumentIngestAction(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" || LooksExplanationOnlyRequest(content) {
		return false
	}
	terms := []string{
		"ingest this file",
		"ingest file",
		"ingest the file",
		"ingest document",
		"ingest documents",
		"ingest the document",
		"ingest the documents",
		"ingest path",
		"ingest the path",
		"ingest this folder",
		"ingest folder",
		"ingest the folder",
		"index this file",
		"index file",
		"index the file",
		"index document",
		"index documents",
		"index the document",
		"index the documents",
		"index path",
		"index the path",
		"index this folder",
		"index folder",
		"index the folder",
		"add this file to documents",
		"add this document",
		"add document to rag",
		"add document to documents",
		"add path to documents",
		"add this folder to documents",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	if (strings.HasPrefix(content, "ingest ") || strings.HasPrefix(content, "index ")) &&
		(strings.Contains(content, "/") || strings.Contains(content, `\`) || strings.Contains(content, ".md") ||
			strings.Contains(content, ".pdf") || strings.Contains(content, ".docx") || strings.Contains(content, ".txt")) {
		return true
	}
	return false
}

func LooksDocumentContextTask(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if LooksRAGTask(content) {
		return true
	}
	terms := []string{
		"rag", "retrieved document", "retrieved docs", "ingested document", "ingested docs",
		"indexed document", "indexed docs",
		"my docs", "my documents", "local docs", "local documents",
		"the docs", "the documents", "docs/", "readme", "blueprint", "roadmap",
		"specification", "spec", "design doc", "design document",
		"architecture doc", "architecture document", "according to the doc",
		"according to the retrieved", "in the retrieved", "from the retrieved",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func LooksMemoryContextTask(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	terms := []string{
		"what did i say", "what did we discuss", "what do you remember",
		"previous conversation", "previous chat", "last conversation", "last chat",
		"conversation history", "session history", "my memory", "your memory",
		"remember when", "do you remember", "recall when", "what do you recall",
		"what's in memory", "what is in memory",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func ContainsLocalContextTerm(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	if content == "" {
		return false
	}
	if len(FileHints(content)) > 0 {
		return true
	}
	anchors := []string{
		"this project", "this repo", "this repository", "this workspace", "this codebase",
		"current project", "current repo", "current repository", "current workspace", "current codebase",
		"my project", "my repo", "my repository", "my workspace", "my codebase",
		"my files", "my docs", "my documents", "local files", "local docs", "local documents",
		"the project", "the repo", "the repository", "the workspace", "the codebase",
		"workspace", "codebase", "repository", "repo",
		"readme", "blueprint", "roadmap", "docs/", "ingested document", "ingested docs",
		"indexed document", "indexed docs",
		"tool log", "tool logs",
	}
	for _, anchor := range anchors {
		if ContainsSearchTerm(content, anchor) {
			return true
		}
	}
	tokens := WordTokens(content)
	if containsAnyToken(tokens, "this", "my", "current", "local") && containsAnyToken(tokens, "project", "workspace", "repo", "repository", "codebase", "files", "documents", "docs") {
		return true
	}
	if containsAnyToken(tokens, "python", "go", "javascript", "typescript", "svelte") && containsAnyToken(tokens, "project", "workspace", "repo", "codebase", "file", "files", "folder", "directory", "test", "tests") {
		return true
	}
	if strings.Contains(content, "yemaka") {
		yemakaLocalTerms := []string{"architecture", "blueprint", "config", "configured", "core", "feature", "implementation", "roadmap", "routing", "skill", "tool", "workspace"}
		for _, term := range yemakaLocalTerms {
			if ContainsSearchTerm(content, term) {
				return true
			}
		}
	}
	return false
}

func LooksCodingTask(content string) bool {
	if LooksInlinePastedExplanationRequest(content) {
		return false
	}
	if ContainsSearchTerm(content, "status code") {
		withoutStatusCode := strings.ReplaceAll(content, "status code", "")
		return LooksCodingTask(withoutStatusCode)
	}
	terms := []string{
		"codebase", "bug", "fix", "test", "compile", "function", "class",
		"interface", "package", "traceback", "stack trace", ".go", ".py", ".js",
		".ts", ".svelte", "git diff", "git status", "syntax error", "runtime error",
		"exception", "error log", "vulnerability", "vulnerabilities", "command injection",
		"authentication",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return ContainsSearchTerm(content, "code") && !ContainsSearchTerm(content, "status code")
}

func LooksReasoningTask(content string) bool {
	terms := []string{
		"reason", "why", "compare", "difference", "different", "decide",
		"analyze", "trace", "tradeoff", "recommend", "plan", "checklist",
		"strategy", "architecture", "design", "optimize", "improve", "suggest",
		"explain", "debug", "troubleshoot", "conceptually",
	}
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func ContainsSearchTerm(content string, term string) bool {
	content = strings.ToLower(content)
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	if strings.ContainsAny(term, "/.") || strings.ContainsAny(term, "|") {
		return strings.Contains(content, term)
	}
	if strings.Contains(term, " ") {
		return containsPhrase(content, term)
	}
	return ContainsWord(content, term)
}

func ContainsWord(content string, word string) bool {
	for _, field := range WordTokens(content) {
		if field == strings.ToLower(strings.TrimSpace(word)) {
			return true
		}
	}
	return false
}

func WordTokens(content string) []string {
	content = strings.ToLower(content)
	fields := strings.FieldsFunc(content, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-')
	})
	tokens := fields[:0]
	for _, field := range fields {
		if field != "" {
			tokens = append(tokens, field)
		}
	}
	return tokens
}

func StripSearchIntentPrefix(content string) string {
	prefixes := []string{
		"okay now ", "ok now ", "okay ", "ok ", "yes ", "yeah ", "yep ", "please ",
		"can you ", "could you ", "would you ", "will you ",
		"can yemaka ", "could yemaka ", "yemaka ",
		"i need you to ", "i want you to ", "now ",
	}
	for {
		trimmed := content
		for _, prefix := range prefixes {
			if strings.HasPrefix(content, prefix) {
				content = strings.TrimSpace(strings.TrimPrefix(content, prefix))
				break
			}
		}
		if content == trimmed {
			return content
		}
	}
}

func InternetURLHint(content string) (string, bool) {
	for _, field := range strings.Fields(content) {
		if target, ok := normalizeInternetTarget(field); ok {
			return target, true
		}
	}
	return "", false
}

func hasExplicitLocalRetrievalIntent(content string) bool {
	if len(FileHints(content)) > 0 {
		return true
	}
	return LooksDocumentSearchTask(content) ||
		LooksDocumentContextTask(content) ||
		LooksLocalFileToolTask(content) ||
		ContainsLocalContextTerm(content)
}

func toolConfidence(content string) int {
	switch {
	case LooksLocalTimeTask(content):
		return 96
	case LooksSpecificCurrentNewsTask(content):
		return 88
	case LooksCrawlerTask(content):
		return 82
	case LooksInternetTask(content):
		return 86
	case LooksMemorySearchTask(content):
		return 88
	case LooksCapabilityActionTask(content):
		return 82
	case LooksDoctorStatusTask(content):
		return 90
	case LooksHeartbeatStatusTask(content):
		return 90
	case LooksTestToolIntent(content), LooksGitToolIntent(content):
		return 88
	case LooksListFilesTask(content):
		return 88
	case LooksFileEditIntent(content):
		return 82
	case LooksLocalFileToolTask(content), LooksFileStateQuestion(content), LooksLocalContextToolTask(content):
		return 84
	default:
		return 62
	}
}

func toolRouteReasons(content string) []string {
	reasons := make([]string, 0, 4)
	add := func(reason string) {
		reasons = appendNonDuplicate(reasons, reason)
	}
	if LooksInternetTask(content) {
		if LooksInternetSearchTask(content) {
			add("internet search intent")
		} else {
			add("internet fetch intent")
		}
	}
	if LooksCrawlerTask(content) {
		add("crawler task intent")
	}
	if LooksLocalTimeTask(content) {
		add("local time intent")
	}
	if LooksSpecificCurrentNewsTask(content) {
		add("specific current-news search intent")
	}
	if LooksMemorySearchTask(content) {
		add("memory search intent")
	}
	if LooksCapabilityActionTask(content) {
		add("capability generation or automation intent")
	}
	if LooksDoctorStatusTask(content) {
		add("local diagnostic intent")
	}
	if LooksHeartbeatStatusTask(content) {
		add("heartbeat health intent")
	}
	if LooksTestToolIntent(content) {
		add("test execution intent")
	}
	if LooksGitToolIntent(content) {
		add("git inspection intent")
	}
	if LooksListFilesTask(content) {
		add("workspace directory listing intent")
	}
	if LooksFileEditIntent(content) {
		add("file edit intent")
	}
	if LooksLocalFileToolTask(content) || LooksFileStateQuestion(content) || LooksLocalContextToolTask(content) {
		add("workspace tool intent")
	}
	if len(reasons) == 0 {
		add("tool-like action intent")
	}
	return reasons
}

func hasProvidedContext(input Request) bool {
	return strings.TrimSpace(input.WorkspaceContext) != "" ||
		strings.TrimSpace(input.ProfileMemory) != "" ||
		strings.TrimSpace(input.TaskMemory) != "" ||
		len(input.Sources) > 0 ||
		len(input.MemorySources) > 0 ||
		strings.TrimSpace(input.SkillName) != ""
}

func looksAuthorizationOnlyFollowup(content string) bool {
	content = strings.ToLower(strings.Join(strings.Fields(content), " "))
	content = StripSearchIntentPrefix(content)
	if content == "" {
		return false
	}
	terms := []string{
		"i authorize you to continue",
		"i authorize this",
		"authorized to continue",
		"you are authorized",
		"you have authorization",
		"you have my authorization",
		"i give authorization",
		"i approve",
		"approved",
		"go ahead",
		"continue",
	}
	for _, term := range terms {
		if content == term || ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func taskMemoryNeedsActiveAssessmentScope(taskMemory string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(taskMemory), " "))
	if normalized == "" {
		return false
	}
	hasAssessment := containsAnyTaskFrameTerm(normalized,
		"active security assessment", "find vulnerability", "find vulnerabilities",
		"vulnerability", "vulnerabilities", "pentest", "penetration test",
	)
	hasScopeRequest := containsAnyTaskFrameTerm(normalized,
		"need explicit authorization", "authorization, scope", "scope, allowed methods",
		"allowed methods", "time/rate limits", "bounded stop condition",
		"before using any tools",
	)
	return hasAssessment && hasScopeRequest
}

func activeAssessmentAuthorizationFollowupDecision(input Request) Decision {
	frame := TaskFrame{
		RawRequest:            strings.TrimSpace(input.Content),
		Domain:                DomainRecon,
		Intent:                IntentClarify,
		TargetType:            TaskFrameTargetExternal,
		Target:                activeAssessmentTargetFromMemory(input.TaskMemory),
		ActionType:            TaskFrameActionActiveAssessmentRequiresScope,
		AutonomyLevel:         TaskFrameAutonomyBounded,
		AuthorizationStatus:   TaskFrameAuthorizationRequired,
		RiskLevel:             RiskHigh,
		RequiresApproval:      true,
		RequiresClarification: true,
		PolicyDecision:        TaskFramePolicyAuthorizationRequired,
		RouteCategory:         RouteActiveAssessmentRequiresScope,
		Capability:            "scope_authorization",
		SafeAlternative:       "I can do a bounded passive check after you provide the exact target, allowed methods, limits, exclusions, output, and stop condition.",
		Explanation:           "Authorization alone is not enough for active security work; scope, limits, and a stop condition are still required before any tool use.",
	}
	return Decision{
		TaskType:              TaskChat,
		Confidence:            90,
		Reasons:               []string{"authorization follow-up still lacks active assessment scope and limits"},
		NeedsClarification:    true,
		ClarificationQuestion: taskFrameScopeQuestion(frame),
		TaskFrame:             frame,
	}
}

func activeAssessmentTargetFromMemory(taskMemory string) string {
	if target, ok := InternetURLHint(taskMemory); ok {
		return target
	}
	return ""
}

func looksDocumentScopeCue(content string) bool {
	return ContainsSearchTerm(content, "ingested document") ||
		ContainsSearchTerm(content, "ingested documents") ||
		ContainsSearchTerm(content, "indexed document") ||
		ContainsSearchTerm(content, "indexed documents") ||
		ContainsSearchTerm(content, "local document") ||
		ContainsSearchTerm(content, "local documents")
}

func ambiguousReferenceQuestion(content string) (string, bool) {
	trimmed := StripSearchIntentPrefix(strings.ToLower(strings.Join(strings.Fields(content), " ")))
	tokens := WordTokens(trimmed)
	if len(tokens) == 0 || len(tokens) > 5 {
		return "", false
	}
	action := tokens[0]
	actionTerms := map[string]string{
		"search":  "Should I search local files, memory, documents, or the internet, and what exact target should I use?",
		"check":   "What exactly should I check, and should I use local files, memory, documents, tools, or the internet?",
		"open":    "Which file, page, or item should I open?",
		"read":    "Which file, document, or source should I read?",
		"edit":    "Which file should I edit, and what change should I make?",
		"fix":     "What should I fix, and which file, error, or project should I use?",
		"run":     "What should I run: tests, a command, a scheduled job, or a specific tool?",
		"do":      "What exact action should Yemaka perform?",
		"use":     "Which tool, skill, connector, file, or source should I use?",
		"send":    "What should I send, and through which connector or destination?",
		"post":    "What should I post, and through which connector or destination?",
		"publish": "What should I publish, and to which destination?",
		"build":   "What capability should I build, and what should it accept and return?",
		"create":  "What should I create: a file, skill, extension, connector, or scheduled job?",
		"make":    "What should I make, and where should it live?",
		"look":    "Should I look in local files, memory, documents, or the internet, and what exact target should I use?",
	}
	question, ok := actionTerms[action]
	if !ok {
		return "", false
	}
	if len(tokens) == 1 {
		return question, true
	}
	if hasSpecificTarget(tokens[1:]) {
		return "", false
	}
	for _, token := range tokens[1:] {
		if isAmbiguousReference(token) {
			return question, true
		}
	}
	return "", false
}

func missingToolTargetQuestion(content string) (string, bool) {
	trimmed := StripSearchIntentPrefix(strings.ToLower(strings.Join(strings.Fields(content), " ")))
	switch trimmed {
	case "run command", "execute command", "run shell command":
		return "What exact command should I run, and is it inside the current workspace?", true
	case "open file", "read file", "read the file", "open the file":
		return "Which file path should I read?", true
	case "edit file", "edit the file", "write file", "write to file":
		return "Which file should I edit, and what exact content or change should I apply?", true
	case "search memory", "search memories":
		return "What should I search for in memory?", true
	case "search docs", "search documents", "search rag":
		return "What should I search for in your local documents?", true
	case "search", "search for", "look up", "lookup":
		return "What should I search for, and should I use local documents, memory, workspace files, or the internet?", true
	default:
		return "", false
	}
}

func conflictingRouteQuestion(content string) (string, bool) {
	if !LooksInternetTask(content) || !LooksDocumentSearchTask(content) {
		return "", false
	}
	return "Should I search your local documents, or use the internet for this?", true
}

func hasSpecificTarget(tokens []string) bool {
	for _, token := range tokens {
		if token == "" || isAmbiguousReference(token) {
			continue
		}
		switch token {
		case "repo", "repository", "project", "workspace", "docs", "documents", "memory", "memories", "web", "internet", "online", "url", "link", "site", "website", "tests", "test", "diff", "status", "telegram", "slack", "discord", "email", "webhook", "connector":
			return true
		default:
			if strings.Contains(token, ".") || strings.Contains(token, "/") {
				return true
			}
		}
	}
	return false
}

func isAmbiguousReference(token string) bool {
	switch token {
	case "it", "this", "that", "these", "those", "thing", "stuff", "there", "here":
		return true
	default:
		return false
	}
}

func containsCommandRiskTerm(content string, term string) bool {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return false
	}
	if strings.Contains(term, "|") {
		return strings.Contains(content, term)
	}
	if strings.Contains(term, " ") {
		return ContainsSearchTerm(content, term)
	}
	return ContainsWord(content, term)
}

func containsPhrase(content string, phrase string) bool {
	contentTerms := WordTokens(content)
	phraseTerms := WordTokens(phrase)
	if len(phraseTerms) == 0 || len(phraseTerms) > len(contentTerms) {
		return false
	}
	for i := 0; i <= len(contentTerms)-len(phraseTerms); i++ {
		matched := true
		for j := range phraseTerms {
			if contentTerms[i+j] != phraseTerms[j] {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func normalizeInternetTarget(candidate string) (string, bool) {
	candidate = strings.TrimSpace(candidate)
	candidate = strings.Trim(candidate, ".,:;()[]{}\"'<>")
	candidate = strings.TrimSuffix(candidate, "/")
	if candidate == "" {
		return "", false
	}
	lower := strings.ToLower(candidate)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		parsed, err := url.Parse(candidate)
		if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
			return "", false
		}
		return candidate, true
	}
	if strings.Contains(candidate, "@") || strings.Contains(candidate, "://") {
		return "", false
	}
	hostPart := candidate
	for _, separator := range []string{"/", "?", "#"} {
		if index := strings.Index(hostPart, separator); index >= 0 {
			hostPart = hostPart[:index]
		}
	}
	hostPart = strings.TrimSuffix(hostPart, ".")
	hostLower := strings.ToLower(hostPart)
	if !strings.Contains(hostPart, ".") || looksLikeLocalCodeFile(hostLower) || looksLikeLocalCodeFile(lower) {
		return "", false
	}
	if looksLikeDottedVersionToken(hostLower) {
		return "", false
	}
	return "https://" + candidate, true
}

func looksLikeLocalCodeFile(candidate string) bool {
	for _, suffix := range []string{".go", ".py", ".js", ".ts", ".svelte", ".md", ".markdown", ".txt", ".pdf", ".doc", ".docx", ".csv", ".tsv", ".rtf", ".odt", ".xlsx", ".yaml", ".yml", ".json", ".toml", ".css", ".html", ".xml", ".sh", ".bat", ".ps1", ".rb", ".php", ".java", ".c", ".cpp", ".h", ".rs"} {
		if strings.HasSuffix(candidate, suffix) {
			return true
		}
	}
	return false
}

func looksLikeDottedVersionToken(candidate string) bool {
	parts := strings.Split(strings.Trim(candidate, "."), ".")
	if len(parts) < 2 {
		return false
	}
	last := parts[len(parts)-1]
	if last == "" {
		return false
	}
	hasAlphaBeforeLast := false
	for _, part := range parts[:len(parts)-1] {
		for _, r := range part {
			if unicode.IsLetter(r) {
				hasAlphaBeforeLast = true
				break
			}
		}
	}
	if !hasAlphaBeforeLast {
		return false
	}
	for _, r := range last {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func containsAnyToken(tokens []string, values ...string) bool {
	for _, token := range tokens {
		for _, value := range values {
			if token == value {
				return true
			}
		}
	}
	return false
}

func appendNonDuplicate(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
