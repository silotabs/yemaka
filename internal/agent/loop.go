package agent

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	contextcore "yemaka/internal/context"
	"yemaka/internal/models"
	promptcore "yemaka/internal/prompt"
	"yemaka/internal/routing"
)

const (
	TaskChat      = models.TaskChat
	TaskCoding    = models.TaskCoding
	TaskReasoning = models.TaskReasoning
	TaskRAG       = models.TaskRAG
	TaskTool      = models.TaskTool

	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"

	EvidenceGeneralKnowledgeAllowed = "general_knowledge_allowed"
	EvidenceLocalDocumentsRequired  = "local_documents_required"
	EvidenceWorkspaceRequired       = "workspace_required"
	EvidenceFreshInternetRequired   = "fresh_internet_required"
	EvidenceMemoryRequired          = "memory_required"
	EvidenceToolResultRequired      = "tool_result_required"
	EvidenceClarificationRequired   = "clarification_required"
)

type PlanInput struct {
	Content            string
	CurrentMessageID   string
	ProfileMemory      string
	TaskMemory         string
	MemorySources      []string
	KnowledgeContext   string
	KnowledgeSources   []string
	WorkspaceContext   string
	SourceKind         string
	Sources            []string
	SkillName          string
	SkillRequiredTools []string
	SkillInstructions  string
	RouteCorrections   []routing.RouteCorrection
	Continuation       routing.ContinuationFrame
	SessionContract    routing.SessionContract
}

type Plan struct {
	TaskType                 string                     `json:"task_type"`
	ModelTask                string                     `json:"model_task"`
	Goal                     string                     `json:"goal"`
	Assumptions              []string                   `json:"assumptions"`
	FilesNeeded              []string                   `json:"files_needed"`
	ToolsNeeded              []string                   `json:"tools_needed"`
	RiskLevel                string                     `json:"risk_level"`
	Steps                    []string                   `json:"steps"`
	Verification             []string                   `json:"verification"`
	MaxSteps                 int                        `json:"max_steps"`
	RouteConfidence          int                        `json:"route_confidence,omitempty"`
	RouteReasons             []string                   `json:"route_reasons,omitempty"`
	RouteCategory            string                     `json:"route_category,omitempty"`
	RouteIntent              string                     `json:"route_intent,omitempty"`
	RouteDomain              string                     `json:"route_domain,omitempty"`
	RouteTarget              string                     `json:"route_target,omitempty"`
	RouteCapability          string                     `json:"route_capability,omitempty"`
	RouteRequiresApproval    bool                       `json:"route_requires_approval,omitempty"`
	RouteUsesWorkspace       bool                       `json:"route_uses_workspace,omitempty"`
	RouteUsesRAG             bool                       `json:"route_uses_rag,omitempty"`
	RouteUsesInternet        bool                       `json:"route_uses_internet,omitempty"`
	RouteReadsFiles          bool                       `json:"route_reads_files,omitempty"`
	RouteWritesFiles         bool                       `json:"route_writes_files,omitempty"`
	RouteGeneratesExtension  bool                       `json:"route_generates_extension,omitempty"`
	RouteCreatesSchedulerJob bool                       `json:"route_creates_scheduler_job,omitempty"`
	RouteConnectorAction     bool                       `json:"route_connector_action,omitempty"`
	RouteCrawlerTask         bool                       `json:"route_crawler_task,omitempty"`
	RouteRunsShell           bool                       `json:"route_runs_shell,omitempty"`
	RouteLane                string                     `json:"route_lane,omitempty"`
	RouteAllowedTools        []string                   `json:"route_allowed_tools,omitempty"`
	RouteBlockedTools        []string                   `json:"route_blocked_tools,omitempty"`
	RouteCandidates          []routing.RouteCandidate   `json:"route_candidates,omitempty"`
	RouteSession             *routing.SessionContract   `json:"route_session,omitempty"`
	RoutePreflight           *routing.PreflightCard     `json:"route_preflight,omitempty"`
	RouteContinuation        *routing.ContinuationFrame `json:"route_continuation,omitempty"`
	ContinuationMode         string                     `json:"continuation_mode,omitempty"`
	AmbiguityFlags           []string                   `json:"ambiguity_flags,omitempty"`
	NeedsClarification       bool                       `json:"needs_clarification,omitempty"`
	ClarificationQuestion    string                     `json:"clarification_question,omitempty"`
	EvidencePolicy           string                     `json:"evidence_policy,omitempty"`
	EvidenceSource           string                     `json:"evidence_source,omitempty"`
	ResponseContract         string                     `json:"response_contract,omitempty"`
}

type RouteDecision struct {
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
	ToolLane              string
	AllowedToolset        []string
	BlockedToolset        []string
	RouteCandidates       []routing.RouteCandidate
	ContinuationMode      string
	AmbiguityFlags        []string
	Preflight             routing.PreflightCard
	Continuation          routing.ContinuationFrame
}

type VerificationResult struct {
	Status  string   `json:"status"`
	Checks  []string `json:"checks"`
	Reasons []string `json:"reasons"`
}

func BuildPlan(input PlanInput) Plan {
	content := strings.TrimSpace(input.Content)
	route := RouteRequest(input)
	taskType := route.TaskType
	plan := Plan{
		TaskType:                 taskType,
		ModelTask:                modelTaskFor(taskType, content),
		Goal:                     compactGoal(content),
		FilesNeeded:              route.Files,
		ToolsNeeded:              route.Tools,
		RiskLevel:                route.RiskLevel,
		MaxSteps:                 maxStepsFor(taskType),
		RouteConfidence:          route.Confidence,
		RouteReasons:             route.Reasons,
		RouteCategory:            route.RouteCategory,
		RouteIntent:              route.Intent,
		RouteDomain:              route.Domain,
		RouteTarget:              route.Target,
		RouteCapability:          route.Capability,
		RouteRequiresApproval:    route.RequiresApproval,
		RouteUsesWorkspace:       route.UseWorkspace,
		RouteUsesRAG:             route.UseRAG,
		RouteUsesInternet:        route.UseInternet,
		RouteReadsFiles:          route.ReadsFiles,
		RouteWritesFiles:         route.WritesFiles,
		RouteGeneratesExtension:  route.GeneratesExtension,
		RouteCreatesSchedulerJob: route.CreatesSchedulerJob,
		RouteConnectorAction:     route.ConnectorAction,
		RouteCrawlerTask:         route.CrawlerTask,
		RouteRunsShell:           route.RunsShell,
		RouteLane:                route.ToolLane,
		RouteAllowedTools:        append([]string{}, route.AllowedToolset...),
		RouteBlockedTools:        append([]string{}, route.BlockedToolset...),
		RouteCandidates:          append([]routing.RouteCandidate{}, route.RouteCandidates...),
		ContinuationMode:         route.ContinuationMode,
		AmbiguityFlags:           append([]string{}, route.AmbiguityFlags...),
		NeedsClarification:       route.NeedsClarification,
		ClarificationQuestion:    route.ClarificationQuestion,
	}
	if !input.SessionContract.IsZero() {
		session := input.SessionContract
		plan.RouteSession = &session
	}
	if !route.Preflight.IsZero() {
		preflight := route.Preflight
		plan.RoutePreflight = &preflight
	}
	if !route.Continuation.IsZero() {
		continuation := route.Continuation
		plan.RouteContinuation = &continuation
	}
	plan.EvidenceSource = evidenceSourceForPlan(plan)
	plan.EvidencePolicy = evidencePolicyForPlan(plan)
	plan.ResponseContract = responseContractForPlan(plan)
	plan.Assumptions = assumptionsFor(input, plan)
	plan.Steps = stepsFor(plan)
	plan.Verification = verificationFor(plan)
	if route.NeedsClarification {
		plan.ModelTask = models.TaskChat
		plan.RiskLevel = RiskLow
		if route.RouteCategory == routing.RouteAuthorizationRequired ||
			route.RouteCategory == routing.RouteActiveAssessmentRequiresScope ||
			route.RouteCategory == routing.RouteClarifyScope ||
			route.RouteCategory == routing.RoutePassiveCheck {
			plan.RiskLevel = route.RiskLevel
		}
		plan.FilesNeeded = nil
		plan.ToolsNeeded = nil
		plan.MaxSteps = 1
		plan.Steps = []string{"Ask one focused clarification before retrieving context or using tools."}
		plan.Verification = []string{"clarification question is specific", "no tool or retrieval action was guessed"}
	}
	return plan
}

func ClassifyTask(input PlanInput) string {
	return RouteRequest(input).TaskType
}

func RouteRequest(input PlanInput) RouteDecision {
	decision := routing.Classify(routing.Request{
		Content:          input.Content,
		SourceKind:       input.SourceKind,
		SkillName:        input.SkillName,
		WorkspaceContext: input.WorkspaceContext,
		ProfileMemory:    input.ProfileMemory,
		TaskMemory:       input.TaskMemory,
		Sources:          input.Sources,
		MemorySources:    input.MemorySources,
		RouteCorrections: input.RouteCorrections,
		Continuation:     input.Continuation,
		SessionContract:  input.SessionContract,
	})
	return RouteDecision{
		TaskType:              decision.TaskType,
		Confidence:            decision.Confidence,
		Reasons:               decision.Reasons,
		NeedsClarification:    decision.NeedsClarification,
		ClarificationQuestion: decision.ClarificationQuestion,
		Tools:                 decision.Tools,
		Files:                 decision.Files,
		RiskLevel:             decision.RiskLevel,
		UseWorkspace:          decision.UseWorkspace,
		UseRAG:                decision.UseRAG,
		UseInternet:           decision.UseInternet,
		RouteCategory:         decision.RouteCategory,
		Intent:                decision.Intent,
		Domain:                decision.Domain,
		Target:                decision.Target,
		Capability:            decision.Capability,
		RequiresApproval:      decision.RequiresApproval,
		ReadsFiles:            decision.ReadsFiles,
		WritesFiles:           decision.WritesFiles,
		GeneratesExtension:    decision.GeneratesExtension,
		CreatesSchedulerJob:   decision.CreatesSchedulerJob,
		ConnectorAction:       decision.ConnectorAction,
		CrawlerTask:           decision.CrawlerTask,
		RunsShell:             decision.RunsShell,
		ToolLane:              decision.ToolLane,
		AllowedToolset:        append([]string{}, decision.AllowedToolset...),
		BlockedToolset:        append([]string{}, decision.BlockedToolset...),
		RouteCandidates:       append([]routing.RouteCandidate{}, decision.RouteCandidates...),
		ContinuationMode:      decision.ContinuationMode,
		AmbiguityFlags:        append([]string{}, decision.AmbiguityFlags...),
		Preflight:             decision.Preflight,
		Continuation:          decision.Continuation,
	}
}

func ClarificationResponse(plan Plan) string {
	question := strings.TrimSpace(plan.ClarificationQuestion)
	if question == "" {
		question = "What should Yemaka use as the target for this request?"
	}
	return question
}

func toolConfidence(content string) int {
	switch {
	case looksInternetTask(content):
		return 86
	case looksMemorySearchTask(content):
		return 88
	case looksCapabilityActionTask(content):
		return 82
	case looksDoctorStatusTask(content):
		return 90
	case looksTestToolIntent(content), looksGitToolIntent(content):
		return 88
	case looksFileEditIntent(content):
		return 82
	case looksLocalFileToolTask(content), looksLocalContextToolTask(content):
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
	if looksInternetTask(content) {
		if looksInternetSearchTask(content) {
			add("internet search intent")
		} else {
			add("internet fetch intent")
		}
	}
	if looksMemorySearchTask(content) {
		add("memory search intent")
	}
	if looksCapabilityActionTask(content) {
		add("capability generation or automation intent")
	}
	if looksDoctorStatusTask(content) {
		add("local diagnostic intent")
	}
	if looksTestToolIntent(content) {
		add("test execution intent")
	}
	if looksGitToolIntent(content) {
		add("git inspection intent")
	}
	if looksFileEditIntent(content) {
		add("file edit intent")
	}
	if looksLocalFileToolTask(content) || looksLocalContextToolTask(content) {
		add("workspace tool intent")
	}
	if len(reasons) == 0 {
		add("tool-like action intent")
	}
	return reasons
}

func hasProvidedContext(input PlanInput) bool {
	return strings.TrimSpace(input.WorkspaceContext) != "" ||
		strings.TrimSpace(input.ProfileMemory) != "" ||
		strings.TrimSpace(input.TaskMemory) != "" ||
		len(input.Sources) > 0 ||
		len(input.MemorySources) > 0 ||
		strings.TrimSpace(input.SkillName) != ""
}

func ambiguousReferenceQuestion(content string) (string, bool) {
	trimmed := stripSearchIntentPrefix(strings.ToLower(strings.Join(strings.Fields(content), " ")))
	tokens := wordTokens(trimmed)
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
	trimmed := stripSearchIntentPrefix(strings.ToLower(strings.Join(strings.Fields(content), " ")))
	if trimmed == "run command" || trimmed == "execute command" || trimmed == "run shell command" {
		return "What exact command should I run, and is it inside the current workspace?", true
	}
	if trimmed == "open file" || trimmed == "read file" || trimmed == "read the file" || trimmed == "open the file" {
		return "Which file path should I read?", true
	}
	if trimmed == "edit file" || trimmed == "edit the file" || trimmed == "write file" || trimmed == "write to file" {
		return "Which file should I edit, and what exact content or change should I apply?", true
	}
	if trimmed == "search memory" || trimmed == "search memories" {
		return "What should I search for in memory?", true
	}
	if trimmed == "search docs" || trimmed == "search documents" || trimmed == "search rag" {
		return "What should I search for in your local documents?", true
	}
	if trimmed == "search" || trimmed == "search for" || trimmed == "look up" || trimmed == "lookup" {
		return "What should I search for, and should I use local documents, memory, workspace files, or the internet?", true
	}
	return "", false
}

func conflictingRouteQuestion(content string) (string, bool) {
	if !looksInternetTask(content) || !looksDocumentSearchTask(content) {
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

func VerifyResponse(plan Plan, content string) VerificationResult {
	cleanContent := strings.TrimSpace(content)
	lowerContent := strings.ToLower(cleanContent)
	result := VerificationResult{
		Status: "pass",
		Checks: []string{
			"response_present",
			"risk_policy_respected",
		},
	}
	if cleanContent == "" {
		markNeedsFollowUp(&result)
		result.Reasons = append(result.Reasons, "assistant response is empty")
	}
	result.Checks = append(result.Checks, "tool_request_marker_absent")
	if strings.Contains(lowerContent, "yemaka_tool_request") {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "model tool request marker leaked into the final answer")
	}
	result.Checks = append(result.Checks, "internal_scaffold_absent")
	if looksInternalScaffold(cleanContent) {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "internal task plan or reasoning scaffold leaked into the final answer")
	}
	if plan.EvidencePolicy == EvidenceGeneralKnowledgeAllowed && looksLikeEvidenceRequiredRefusal(lowerContent) {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "general knowledge route refused because optional local/workspace evidence was absent")
	}
	if plan.TaskType == TaskRAG || len(plan.FilesNeeded) > 0 {
		result.Checks = append(result.Checks, "grounding_expected", "source_citation_checked")
		if len(plan.FilesNeeded) > 0 && !contentMentionsAny(cleanContent, plan.FilesNeeded) {
			markNeedsFollowUp(&result)
			result.Reasons = appendNonDuplicate(result.Reasons, "answer does not mention the expected local source path")
		}
		if plan.TaskType == TaskRAG && !looksGroundedAnswer(cleanContent) {
			markNeedsFollowUp(&result)
			result.Reasons = appendNonDuplicate(result.Reasons, "RAG answer should cite or name retrieved local sources")
		}
	}
	result = VerifyRouteContract(plan, result)
	if containsTool(plan.ToolsNeeded, "run_tests") {
		result.Checks = append(result.Checks, "test_result_status_checked")
		if !mentionsTestOutcome(lowerContent) {
			markNeedsFollowUp(&result)
			result.Reasons = appendNonDuplicate(result.Reasons, "test summary does not clearly say whether tests passed, failed, or were unavailable")
		}
	}
	if plan.RiskLevel == RiskHigh {
		result.Checks = append(result.Checks, "confirmation_required_before_action")
		if strings.Contains(lowerContent, "i ran ") || strings.Contains(lowerContent, "i deleted ") || strings.Contains(lowerContent, "i removed ") {
			result.Status = "rollback_required"
			result.Reasons = append(result.Reasons, "response implies risky action without confirmed tool result")
		}
	}
	if len(result.Reasons) == 0 {
		result.Reasons = append(result.Reasons, "no verifier concerns")
	}
	return result
}

func VerifyRouteContract(plan Plan, result VerificationResult) VerificationResult {
	result.Checks = append(result.Checks, "route_contract_checked")
	if plan.RouteConfidence > 0 && plan.RouteConfidence < 55 && !plan.NeedsClarification {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "route confidence was below clarification threshold")
	}
	if len(plan.AmbiguityFlags) > 0 && !plan.NeedsClarification && plan.ContinuationMode == routing.ContinuationModeUnclear {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "ambiguous continuation was not clarified")
	}
	if plan.RoutePreflight != nil && plan.RoutePreflight.FreshnessRisk == routing.PreflightFreshnessHigh &&
		!plan.RouteUsesInternet && plan.RouteCategory != routing.RouteClarify {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "current public fact route did not require a fresh internet source")
	}
	if plan.RouteCategory == routing.RouteChatExplanation && len(plan.ToolsNeeded) > 0 {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "chat explanation route exposed tools")
	}
	for _, tool := range plan.ToolsNeeded {
		for _, blocked := range plan.RouteBlockedTools {
			if strings.TrimSpace(tool) == strings.TrimSpace(blocked) {
				markNeedsFollowUp(&result)
				result.Reasons = appendNonDuplicate(result.Reasons, "route lane exposed blocked tool "+tool)
			}
		}
	}
	if plan.RouteRequiresApproval && plan.RouteLane != routing.ToolLaneFileEdit &&
		plan.RouteLane != routing.ToolLaneScheduler &&
		plan.RouteLane != routing.ToolLaneExtension &&
		plan.RouteLane != routing.ToolLaneConnector &&
		plan.RouteLane != routing.ToolLaneWebCrawl {
		markNeedsFollowUp(&result)
		result.Reasons = appendNonDuplicate(result.Reasons, "approval-gated route has no approval-capable lane")
	}
	return result
}

func markNeedsFollowUp(result *VerificationResult) {
	if result.Status == "" || result.Status == "pass" {
		result.Status = "needs_follow_up"
	}
}

func looksLikeEvidenceRequiredRefusal(lowerContent string) bool {
	lowerContent = strings.ToLower(strings.TrimSpace(lowerContent))
	if lowerContent == "" {
		return false
	}
	evidenceMissing := strings.Contains(lowerContent, "not present in the available local context") ||
		strings.Contains(lowerContent, "not present in the current workspace") ||
		strings.Contains(lowerContent, "workspace context does not contain") ||
		strings.Contains(lowerContent, "available local context or workspace files") ||
		strings.Contains(lowerContent, "cannot provide") && strings.Contains(lowerContent, "based on available evidence") ||
		strings.Contains(lowerContent, "cannot be generated from the current source of truth") ||
		strings.Contains(lowerContent, "cannot be derived from the current workspace")
	if !evidenceMissing {
		return false
	}
	return strings.Contains(lowerContent, "local context") ||
		strings.Contains(lowerContent, "workspace") ||
		strings.Contains(lowerContent, "retrieved") ||
		strings.Contains(lowerContent, "available evidence") ||
		strings.Contains(lowerContent, "source of truth")
}

func contentMentionsAny(content string, values []string) bool {
	lower := strings.ToLower(content)
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if strings.Contains(lower, value) || strings.Contains(lower, strings.ToLower(filepath.Base(value))) {
			return true
		}
	}
	return false
}

func looksGroundedAnswer(content string) bool {
	lower := strings.ToLower(content)
	if strings.Contains(lower, "source") || strings.Contains(lower, "according to") || strings.Contains(lower, "from ") {
		return true
	}
	for _, field := range strings.Fields(content) {
		field = strings.Trim(field, ".,:;()[]{}\"'")
		if strings.Contains(field, "/") || strings.Contains(filepath.Base(field), ".") {
			return true
		}
	}
	return false
}

func mentionsTestOutcome(lowerContent string) bool {
	terms := []string{"pass", "passed", "fail", "failed", "failure", "error", "no tests", "unavailable", "could not run"}
	for _, term := range terms {
		if strings.Contains(lowerContent, term) {
			return true
		}
	}
	return false
}

func VerifyEditProposal(result VerificationResult, proposal *EditProposal) VerificationResult {
	if proposal == nil {
		return result
	}
	result.Reasons = removeReason(result.Reasons, "no verifier concerns")
	result.Checks = append(result.Checks, "edit_proposal_checked")
	if proposal.NeedsContent {
		result.Checks = append(result.Checks, "full_replacement_content_missing")
		if result.Status == "pass" {
			result.Status = "needs_follow_up"
		}
		result.Reasons = appendNonDuplicate(result.Reasons, "edit proposal still needs a full replacement file body")
		return result
	}
	result.Checks = append(result.Checks, "full_replacement_content_ready", "diff_preview_required_before_apply")
	if proposal.Path == "" || strings.TrimSpace(proposal.Content) == "" {
		result.Status = "needs_follow_up"
		result.Reasons = appendNonDuplicate(result.Reasons, "edit proposal path or content is missing")
		return result
	}
	if result.Status == "pass" {
		result.Status = "needs_follow_up"
	}
	result.Reasons = appendNonDuplicate(result.Reasons, "edit proposal is ready for diff preview and explicit approval")
	return result
}

func PromptMessages(plan Plan, input PlanInput, system string) []models.ChatMessage {
	return PromptMessagesWithOptions(plan, input, system, promptcore.Options{})
}

func ContextGroundingGuidance() string {
	return " Context hierarchy: current tool results and retrieved context are evidence; profile memory, task memory, " +
		"and recent session messages are background only. Do not cite memory or recent chat as current factual proof. " +
		"Recent messages may describe intended actions that never completed; do not say a file exists, was read, or has current content unless a current tool result or retrieved source proves it. " +
		"Current local date: " + time.Now().Format("2006-01-02") + ". " +
		"Approved knowledge graph context, when present, is reviewed local background context only. It may help ordinary explanations, but it is not proof for routes that require local documents, workspace files, current internet evidence, or tool results. " +
		"When an internet/search task is requested and the internet tool is available, use the tool instead of answering from workspace docs. " +
		"For internet/search answers, use only the current tool result or explicitly provided source; do not reuse older search results from recent messages as fresh evidence. " +
		"If the current result is empty, dated, cached, or does not directly support a claim, say that clearly instead of filling gaps from memory. " +
		"If the selected route requires evidence and current evidence is missing, say what exact tool, source, or permission is needed. " +
		"If the selected route is plain chat or explanation, general model knowledge is allowed; do not refuse only because workspace files or local documents are absent."
}

func AnswerContractGuidance(plan Plan) string {
	contract := strings.TrimSpace(plan.ResponseContract)
	if contract == "" {
		contract = responseContractForPlan(plan)
	}
	return " Treat the task plan, route metadata, assumptions, steps, and verification as private control data; never quote or summarize them in the answer. " +
		"Keep reasoning internal and answer in natural user-facing prose. " +
		"Evidence contract: " + contract + "."
}

func PromptMessagesWithOptions(plan Plan, input PlanInput, system string, options promptcore.Options) []models.ChatMessage {
	messages, _ := PromptMessagesWithDiagnostics(plan, input, system, options)
	return messages
}

func PromptMessagesWithDiagnostics(plan Plan, input PlanInput, system string, options promptcore.Options) ([]models.ChatMessage, contextcore.Result) {
	compiled := compilePromptContext(plan, input, options)
	optimized := promptcore.BuildUserPrompt(plan.PromptBlock(), compiled.Sections, input.Content, options)
	return []models.ChatMessage{
		{Role: "system", Content: system},
		{Role: "user", Content: optimized.Content},
	}, compiled.Result
}

func compilePromptContextSections(input PlanInput, options promptcore.Options) []promptcore.Section {
	plan := BuildPlan(input)
	return compilePromptContext(plan, input, options).Sections
}

type compiledPromptContext struct {
	Sections []promptcore.Section
	Result   contextcore.Result
}

func compilePromptContext(plan Plan, input PlanInput, options promptcore.Options) compiledPromptContext {
	knowledgeContext := ""
	if planAllowsKnowledgeContext(plan) {
		knowledgeContext = input.KnowledgeContext
	}
	taskState := compactTaskStateForPrompt(plan, input)
	taskStateContent := contextcore.RenderTaskStatePackage(taskState, contextcore.TaskStateOptions{
		MaxBytes: promptTaskStateBudget(options).MaxBytes,
	})
	ordered := []promptContextItem{
		{
			item:     contextcore.TaskStateItem("compact_task_state", "COMPACT TASK STATE", taskStateContent, 1, contextcore.WithPriority(95), contextcore.WithRequired(true), contextcore.WithMetadata(map[string]string{"kind": "task_state_compaction_v1"})),
			priority: 95,
		},
		{
			item:     contextcore.MemoryItem("profile_memory", "PROFILE MEMORY", input.ProfileMemory, 0.82, contextcore.WithPriority(80), contextcore.WithRequired(true)),
			priority: 80,
		},
		{
			item:     contextcore.MemoryItem("task_memory", "TASK MEMORY", input.TaskMemory, 0.78, contextcore.WithPriority(70)),
			priority: 70,
		},
		{
			item:     contextcore.KnowledgeItem("knowledge_graph", "APPROVED KNOWLEDGE GRAPH CONTEXT", knowledgeContext, 0.74, contextcore.WithPriority(68)),
			priority: 68,
		},
		{
			item:     contextcore.NewItem(promptContextSource(input.SourceKind), "retrieved_context", "RETRIEVED CONTEXT", input.WorkspaceContext, 0.95, contextcore.WithPriority(90)),
			priority: 90,
		},
		{
			item:     contextcore.SkillItem("active_skill", "ACTIVE SKILL", input.SkillName, 0.88, contextcore.WithPriority(85), contextcore.WithRequired(true)),
			priority: 85,
		},
		{
			item:     contextcore.SkillItem("skill_instructions", "SKILL INSTRUCTIONS", input.SkillInstructions, 0.86, contextcore.WithPriority(85), contextcore.WithRequired(true)),
			priority: 85,
		},
	}

	items := make([]contextcore.Item, 0, len(ordered))
	for _, entry := range ordered {
		if strings.TrimSpace(entry.item.Content) != "" {
			items = append(items, entry.item)
		}
	}
	result := contextcore.Compile(contextcore.Input{
		Request: input.Content,
		Items:   items,
		Options: contextcore.Options{
			Budget:       promptContextBudget(options),
			MinRelevance: 0.05,
			SourcePriorities: map[contextcore.Source]int{
				contextcore.SourceTaskState: 920,
				contextcore.SourceRAG:       900,
				contextcore.SourceSkills:    850,
				contextcore.SourceMemory:    800,
				contextcore.SourceKnowledge: 760,
			},
		},
	})

	compiledByID := make(map[string]contextcore.CompiledItem, len(result.Items))
	for _, item := range result.Items {
		compiledByID[item.ID] = item
	}
	sections := make([]promptcore.Section, 0, len(ordered))
	for _, entry := range ordered {
		compiled, ok := compiledByID[entry.item.ID]
		if !ok || strings.TrimSpace(compiled.Content) == "" {
			continue
		}
		sections = append(sections, promptcore.Section{
			Name:     entry.item.Title,
			Content:  compiled.Content,
			Priority: entry.priority,
		})
	}
	return compiledPromptContext{Sections: sections, Result: result}
}

func planAllowsKnowledgeContext(plan Plan) bool {
	if plan.EvidencePolicy != EvidenceGeneralKnowledgeAllowed {
		return false
	}
	if plan.RouteCategory == "" {
		return true
	}
	return plan.RouteCategory == routing.RouteChatExplanation
}

type promptContextItem struct {
	item     contextcore.Item
	priority int
}

func promptContextSource(sourceKind string) contextcore.Source {
	switch strings.ToLower(strings.TrimSpace(sourceKind)) {
	case "memory", "memories":
		return contextcore.SourceMemory
	default:
		return contextcore.SourceRAG
	}
}

func promptContextBudget(options promptcore.Options) contextcore.Budget {
	maxBytes := options.MaxChars
	if maxBytes <= 0 {
		maxBytes = promptcore.DefaultMaxChars
	}
	if options.LowMemory && maxBytes > promptcore.DefaultMaxChars {
		maxBytes = promptcore.DefaultMaxChars
	}
	if maxBytes < 2000 {
		maxBytes = 2000
	}

	memoryLimit := contextcore.Limit{MaxBytes: 3200, MaxTokens: 900}
	taskStateLimit := contextcore.Limit{MaxBytes: 1800, MaxTokens: 520}
	knowledgeLimit := contextcore.Limit{MaxBytes: 1600, MaxTokens: 450}
	ragLimit := contextcore.Limit{MaxBytes: 5200, MaxTokens: 1500}
	skillLimit := contextcore.Limit{MaxBytes: 2840, MaxTokens: 800}
	if options.LowMemory {
		memoryLimit = contextcore.Limit{MaxBytes: 2100, MaxTokens: 600}
		taskStateLimit = contextcore.Limit{MaxBytes: 1200, MaxTokens: 340}
		knowledgeLimit = contextcore.Limit{MaxBytes: 900, MaxTokens: 260}
		ragLimit = contextcore.Limit{MaxBytes: 2800, MaxTokens: 800}
		skillLimit = contextcore.Limit{MaxBytes: 1840, MaxTokens: 520}
	}

	return contextcore.Budget{
		MaxBytes:      maxBytes,
		MaxTokens:     maxBytes / 4,
		MinItemBytes:  80,
		MinItemTokens: 20,
		PerSource: map[contextcore.Source]contextcore.Limit{
			contextcore.SourceTaskState: taskStateLimit,
			contextcore.SourceMemory:    memoryLimit,
			contextcore.SourceKnowledge: knowledgeLimit,
			contextcore.SourceRAG:       ragLimit,
			contextcore.SourceSkills:    skillLimit,
		},
	}
}

func promptTaskStateBudget(options promptcore.Options) contextcore.Limit {
	if options.LowMemory {
		return contextcore.Limit{MaxBytes: 1200, MaxTokens: 340}
	}
	return contextcore.Limit{MaxBytes: 1800, MaxTokens: 520}
}

func appendNonDuplicate(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func removeReason(values []string, value string) []string {
	filtered := values[:0]
	for _, existing := range values {
		if existing != value {
			filtered = append(filtered, existing)
		}
	}
	return filtered
}

func (p Plan) PromptBlock() string {
	lines := []string{
		"Internal plan: do not quote, explain, or reveal this block; use it only to produce the answer.",
		"Goal: " + p.Goal,
		"Task type: " + p.TaskType,
		"Model task: " + p.ModelTask,
		"Risk level: " + p.RiskLevel,
		fmt.Sprintf("Max steps: %d", p.MaxSteps),
		fmt.Sprintf("Route confidence: %d", p.RouteConfidence),
	}
	if p.EvidencePolicy != "" {
		lines = append(lines, "Evidence policy: "+p.EvidencePolicy)
	}
	if p.ResponseContract != "" {
		lines = append(lines, "Response contract: "+p.ResponseContract)
	}
	if len(p.RouteReasons) > 0 {
		lines = append(lines, "Route reasons: "+strings.Join(p.RouteReasons, ", "))
	}
	lines = append(lines, planRouteMetadataLines(p)...)
	if p.NeedsClarification {
		lines = append(lines, "Needs clarification: true")
		lines = append(lines, "Clarification question: "+p.ClarificationQuestion)
	}
	lines = append(lines, prefixed("Assumptions", p.Assumptions)...)
	lines = append(lines, prefixed("Files needed", p.FilesNeeded)...)
	lines = append(lines, prefixed("Tools needed", p.ToolsNeeded)...)
	lines = append(lines, prefixed("Steps", p.Steps)...)
	lines = append(lines, prefixed("Verification", p.Verification)...)
	return strings.Join(lines, "\n")
}

func planRouteMetadataLines(p Plan) []string {
	lines := make([]string, 0, 12)
	add := func(label string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	add("Route category", p.RouteCategory)
	add("Route intent", p.RouteIntent)
	add("Route domain", p.RouteDomain)
	add("Route target", p.RouteTarget)
	add("Route capability", p.RouteCapability)
	if p.RouteRequiresApproval {
		lines = append(lines, "Route requires approval: true")
	}
	if p.RouteUsesInternet {
		lines = append(lines, "Route uses internet: true")
	}
	if p.RouteReadsFiles {
		lines = append(lines, "Route reads files: true")
	}
	if p.RouteWritesFiles {
		lines = append(lines, "Route writes files: true")
	}
	if p.RouteGeneratesExtension {
		lines = append(lines, "Route generates extension: true")
	}
	if p.RouteCreatesSchedulerJob {
		lines = append(lines, "Route creates scheduler job: true")
	}
	add("Route lane", p.RouteLane)
	add("Continuation mode", p.ContinuationMode)
	if p.RoutePreflight != nil {
		add("Route source of truth", p.RoutePreflight.SourceOfTruth)
		add("Route target source", p.RoutePreflight.TargetSource)
		add("Route freshness risk", p.RoutePreflight.FreshnessRisk)
		add("Route required tool", p.RoutePreflight.RequiredTool)
		add("Route blocked requirement", p.RoutePreflight.BlockedRequirement)
		add("Route stop reason", p.RoutePreflight.StopReason)
	}
	if p.RouteContinuation != nil {
		add("Continuation kind", p.RouteContinuation.Kind)
		add("Continuation target source", p.RouteContinuation.TargetSource)
		add("Continuation prior target", p.RouteContinuation.PriorTarget)
		add("Continuation prior source", p.RouteContinuation.PriorSourceOfTruth)
		add("Continuation prior tool", p.RouteContinuation.PriorToolName)
		add("Continuation prior failure", p.RouteContinuation.PriorFailureStatus)
	}
	return lines
}

func emitPlanEvents(emit EventHandler, plan Plan) error {
	if emit == nil {
		return nil
	}
	routeData := planEventRouteData(plan)
	if err := emit(Event{
		Type: EventTaskClassified,
		Data: routeData,
	}); err != nil {
		return err
	}
	planData := planEventRouteData(plan)
	planData["goal"] = plan.Goal
	planData["max_steps"] = fmt.Sprintf("%d", plan.MaxSteps)
	planData["tools_needed"] = strings.Join(plan.ToolsNeeded, ", ")
	planData["route_reasons"] = strings.Join(plan.RouteReasons, ", ")
	planData["clarification_question"] = plan.ClarificationQuestion
	return emit(Event{Type: EventPlanCreated, Data: planData})
}

func planEventRouteData(plan Plan) map[string]string {
	data := map[string]string{
		"task_type":           plan.TaskType,
		"model_task":          plan.ModelTask,
		"risk_level":          plan.RiskLevel,
		"route_confidence":    fmt.Sprintf("%d", plan.RouteConfidence),
		"route_category":      plan.RouteCategory,
		"route_intent":        plan.RouteIntent,
		"route_domain":        plan.RouteDomain,
		"route_target":        plan.RouteTarget,
		"route_capability":    plan.RouteCapability,
		"route_lane":          plan.RouteLane,
		"evidence_policy":     plan.EvidencePolicy,
		"evidence_source":     plan.EvidenceSource,
		"response_contract":   plan.ResponseContract,
		"continuation_mode":   plan.ContinuationMode,
		"ambiguity_flags":     strings.Join(plan.AmbiguityFlags, ", "),
		"requires_approval":   fmt.Sprintf("%t", plan.RouteRequiresApproval),
		"needs_clarification": fmt.Sprintf("%t", plan.NeedsClarification),
	}
	if plan.RoutePreflight != nil {
		data["route_source_of_truth"] = plan.RoutePreflight.SourceOfTruth
		data["route_target_source"] = plan.RoutePreflight.TargetSource
		data["route_freshness_risk"] = plan.RoutePreflight.FreshnessRisk
		data["route_required_tool"] = plan.RoutePreflight.RequiredTool
		data["route_missing_slots"] = strings.Join(plan.RoutePreflight.MissingSlots, ", ")
		data["route_blocked_requirement"] = plan.RoutePreflight.BlockedRequirement
		data["route_stop_reason"] = plan.RoutePreflight.StopReason
	}
	if plan.RouteContinuation != nil {
		data["continuation_kind"] = plan.RouteContinuation.Kind
		data["continuation_target_source"] = plan.RouteContinuation.TargetSource
		data["continuation_prior_target"] = plan.RouteContinuation.PriorTarget
		data["continuation_prior_source"] = plan.RouteContinuation.PriorSourceOfTruth
		data["continuation_prior_tool"] = plan.RouteContinuation.PriorToolName
		data["continuation_prior_failure"] = plan.RouteContinuation.PriorFailureStatus
	}
	return data
}

func emitVerification(emit EventHandler, result VerificationResult) error {
	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventVerificationCompleted,
		Data: map[string]string{
			"status":  result.Status,
			"checks":  strings.Join(result.Checks, ", "),
			"reasons": strings.Join(result.Reasons, "; "),
		},
	})
}

func modelTaskFor(taskType string, content string) string {
	lower := strings.ToLower(content)
	switch taskType {
	case TaskCoding:
		return models.TaskCoding
	case TaskReasoning:
		return models.TaskReasoning
	case TaskTool:
		if looksCodingTask(lower) || looksTestToolIntent(lower) || looksGitToolIntent(lower) {
			return models.TaskCoding
		}
		return models.TaskChat
	case TaskRAG:
		return models.TaskChat
	default:
		return models.TaskChat
	}
}

func assumptionsFor(input PlanInput, plan Plan) []string {
	assumptions := []string{}
	switch plan.EvidencePolicy {
	case EvidenceGeneralKnowledgeAllowed:
		assumptions = append(assumptions, "General model knowledge is allowed for this chat or explanation route.")
	case EvidenceLocalDocumentsRequired:
		assumptions = append(assumptions, "Retrieved local document evidence is required before answering.")
	case EvidenceWorkspaceRequired:
		assumptions = append(assumptions, "Workspace or file evidence is required before making file-specific claims.")
	case EvidenceFreshInternetRequired:
		assumptions = append(assumptions, "Fresh configured internet evidence is required before answering current public facts.")
	case EvidenceMemoryRequired:
		assumptions = append(assumptions, "Saved memory evidence is required before answering from memory.")
	case EvidenceToolResultRequired:
		assumptions = append(assumptions, "Current tool results are required before claiming an action completed.")
	case EvidenceClarificationRequired:
		assumptions = append(assumptions, "Clarification is required before proceeding.")
	default:
		assumptions = append(assumptions, "Use the selected route evidence contract.")
	}
	if len(input.Sources) > 0 {
		assumptions = append(assumptions, "Retrieved sources are available in the prompt.")
	}
	if strings.TrimSpace(input.ProfileMemory) != "" || strings.TrimSpace(input.TaskMemory) != "" {
		assumptions = append(assumptions, "Relevant local memories are available in the prompt.")
	}
	if input.SkillName != "" {
		assumptions = append(assumptions, "A reusable skill is active for this request.")
	}
	if plan.TaskType == TaskTool {
		assumptions = append(assumptions, "Any risky action must be confirmed before execution.")
	}
	return assumptions
}

func evidenceSourceForPlan(plan Plan) string {
	if plan.RoutePreflight != nil && strings.TrimSpace(plan.RoutePreflight.SourceOfTruth) != "" {
		return strings.TrimSpace(plan.RoutePreflight.SourceOfTruth)
	}
	switch {
	case plan.RouteUsesRAG || plan.TaskType == TaskRAG:
		return routing.PreflightSourceLocalDocuments
	case plan.RouteUsesWorkspace || plan.RouteReadsFiles || plan.RouteWritesFiles:
		return routing.PreflightSourceWorkspace
	case plan.RouteUsesInternet:
		return routing.PreflightSourceInternet
	case plan.RouteCategory == routing.RouteMemorySearch:
		return routing.PreflightSourceMemory
	case plan.TaskType == TaskTool:
		return routing.PreflightSourceTool
	default:
		return routing.PreflightSourceChat
	}
}

func evidencePolicyForPlan(plan Plan) string {
	if plan.NeedsClarification || plan.RouteCategory == routing.RouteClarify || plan.RouteCategory == routing.RouteClarifyScope {
		return EvidenceClarificationRequired
	}
	if plan.RoutePreflight != nil && plan.RoutePreflight.FreshnessRisk == routing.PreflightFreshnessHigh {
		return EvidenceFreshInternetRequired
	}
	switch evidenceSourceForPlan(plan) {
	case routing.PreflightSourceLocalDocuments:
		return EvidenceLocalDocumentsRequired
	case routing.PreflightSourceWorkspace:
		return EvidenceWorkspaceRequired
	case routing.PreflightSourceInternet:
		return EvidenceFreshInternetRequired
	case routing.PreflightSourceMemory:
		return EvidenceMemoryRequired
	case routing.PreflightSourceTool, routing.PreflightSourceScheduler, routing.PreflightSourceExtension, routing.PreflightSourceConnector:
		return EvidenceToolResultRequired
	case routing.PreflightSourceClarify:
		return EvidenceClarificationRequired
	default:
		if plan.RouteLane == routing.ToolLaneChat || plan.TaskType == TaskChat || plan.TaskType == TaskReasoning || plan.TaskType == TaskCoding {
			return EvidenceGeneralKnowledgeAllowed
		}
		return EvidenceToolResultRequired
	}
}

func responseContractForPlan(plan Plan) string {
	lane := routing.ToolLaneForRoute(plan.RouteCategory)
	switch evidencePolicyForPlan(plan) {
	case EvidenceGeneralKnowledgeAllowed:
		return "general model knowledge is allowed; use provided local context only when it is relevant, and do not refuse only because workspace files or local documents are absent"
	case EvidenceLocalDocumentsRequired:
		return "answer only from retrieved local document/RAG evidence; if no matching evidence is available, say that and ask whether to use general knowledge or configured search"
	case EvidenceWorkspaceRequired:
		return "answer file/workspace claims only from provided workspace evidence or current file/tool results; ask for a path or grant when evidence is missing"
	case EvidenceFreshInternetRequired:
		return "use configured and approved fresh internet/search evidence for current public facts, or clearly say search is required/unavailable"
	case EvidenceMemoryRequired:
		return "answer only from matching saved memory evidence and say when memory has no match"
	case EvidenceToolResultRequired:
		if strings.TrimSpace(lane.ResponseContract) != "" {
			return lane.ResponseContract
		}
		return "summarize only current tool results and do not claim actions completed without a current tool result"
	case EvidenceClarificationRequired:
		return "ask the required clarification before answering or running tools"
	default:
		if strings.TrimSpace(lane.ResponseContract) != "" {
			return lane.ResponseContract
		}
		return "follow the selected route without exposing internal control data"
	}
}

func toolsFor(content string, taskType string) []string {
	return routing.ToolsFor(content, taskType)
}

func riskLevel(content string) string {
	return routing.RiskLevel(content)
}

func looksFileEditIntent(content string) bool {
	return routing.LooksFileEditIntent(content)
}

func looksLocalFileToolTask(content string) bool {
	return routing.LooksLocalFileToolTask(content)
}

func stepsFor(plan Plan) []string {
	steps := []string{"Use the route and evidence contract.", "Keep reasoning internal."}
	switch plan.TaskType {
	case TaskTool:
		steps = append(steps, "Validate the requested tool action.", "Run only one approved action at a time.", "Summarize the tool result.")
	case TaskCoding:
		if plan.EvidencePolicy == EvidenceWorkspaceRequired {
			steps = append(steps, "Inspect relevant files or context.", "Explain the code-grounded answer.", "Recommend tests when changes are needed.")
		} else {
			steps = append(steps, "Answer from general coding knowledge unless a file/workspace source was requested.", "Recommend tests when changes are needed.")
		}
	case TaskReasoning:
		steps = append(steps, "Answer the concept or reasoning question directly.", "Do not show hidden assumptions or step-by-step reasoning.")
	case TaskRAG:
		steps = append(steps, "Use retrieved document chunks.", "Cite source paths when available.", "Say when context is insufficient.")
	default:
		steps = append(steps, "Answer directly and concisely.")
	}
	if plan.RiskLevel == RiskHigh {
		steps = append(steps, "Stop before destructive action and ask for confirmation.")
	}
	return steps
}

func verificationFor(plan Plan) []string {
	checks := []string{"response is non-empty", "answer follows evidence contract", "internal plan is not exposed"}
	switch plan.TaskType {
	case TaskTool:
		checks = append(checks, "tool output matches requested action", "risky commands were not run without confirmation")
	case TaskCoding:
		checks = append(checks, "file claims are grounded", "tests are mentioned when relevant")
	case TaskRAG:
		checks = append(checks, "answer is grounded in retrieved sources")
	}
	if plan.EvidencePolicy == EvidenceGeneralKnowledgeAllowed {
		checks = append(checks, "general knowledge answer is allowed when no source was required")
	}
	if plan.RiskLevel == RiskHigh {
		checks = append(checks, "rollback or confirmation is considered")
	}
	return checks
}

func maxStepsFor(taskType string) int {
	switch taskType {
	case TaskTool:
		// return 4 for simple tool tasks, but allow more steps for complex tool tasks that require reasoning or multiple actions
		return 8
	case TaskCoding:
		// return 5 for simple coding tasks, but allow more steps for complex coding tasks that require file inspection or test recommendations
		return 12
	default:
		return 5
	}
}

func compactGoal(content string) string {
	content = strings.Join(strings.Fields(content), " ")
	if content == "" {
		return "Respond to the user request."
	}
	if len(content) > 120 {
		return content[:120] + "..."
	}
	return content
}

func fileHints(content string) []string {
	return routing.FileHints(content)
}

func looksToolTask(content string) bool {
	return routing.LooksToolTask(content)
}

func looksTestToolIntent(content string) bool {
	return routing.LooksTestToolIntent(content)
}

func looksGitToolIntent(content string) bool {
	return routing.LooksGitToolIntent(content)
}

func looksMemorySearchTask(content string) bool {
	return routing.LooksMemorySearchTask(content)
}

func looksDoctorStatusTask(content string) bool {
	return routing.LooksDoctorStatusTask(content)
}

func looksCapabilityActionTask(content string) bool {
	return routing.LooksCapabilityActionTask(content)
}

func looksExtensionGenerationTask(content string) bool {
	return routing.LooksExtensionGenerationTask(content)
}

func looksConnectorActionTask(content string) bool {
	return routing.LooksConnectorActionTask(content)
}

func looksWorkflowActionTask(content string) bool {
	return routing.LooksWorkflowActionTask(content)
}

func looksSkillCreationTask(content string) bool {
	return routing.LooksSkillCreationTask(content)
}

func looksInternetTask(content string) bool {
	return routing.LooksInternetTask(content)
}

func looksInternetSearchTask(content string) bool {
	return routing.LooksInternetSearchTask(content)
}

func looksGenericInternetSearchTask(content string) bool {
	return routing.LooksGenericInternetSearchTask(content)
}

func stripSearchIntentPrefix(content string) string {
	return routing.StripSearchIntentPrefix(content)
}

func looksRAGTask(content string) bool {
	return routing.LooksRAGTask(content)
}

func looksDocumentSearchTask(content string) bool {
	return routing.LooksDocumentSearchTask(content)
}

func looksDocumentContextTask(content string) bool {
	return routing.LooksDocumentContextTask(content)
}

func looksMemoryContextTask(content string) bool {
	return routing.LooksMemoryContextTask(content)
}

func containsSearchTerm(content string, term string) bool {
	return routing.ContainsSearchTerm(content, term)
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
		return containsSearchTerm(content, term)
	}
	return containsWord(content, term)
}

func containsPhrase(content string, phrase string) bool {
	contentTerms := wordTokens(content)
	phraseTerms := wordTokens(phrase)
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

func containsWord(content string, word string) bool {
	return routing.ContainsWord(content, word)
}

func wordTokens(content string) []string {
	return routing.WordTokens(content)
}

func looksCodingTask(content string) bool {
	return routing.LooksCodingTask(content)
}

func looksReasoningTask(content string) bool {
	return routing.LooksReasoningTask(content)
}

func prefixed(label string, values []string) []string {
	if len(values) == 0 {
		return []string{label + ": none"}
	}
	lines := []string{label + ":"}
	for _, value := range values {
		lines = append(lines, "- "+value)
	}
	return lines
}
