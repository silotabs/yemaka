package routing

import (
	"strings"
	"time"
)

const (
	TaskStatusIdle                  = "idle"
	TaskStatusActive                = "active"
	TaskStatusAwaitingClarification = "awaiting_clarification"
	TaskStatusAwaitingApproval      = "awaiting_approval"
	TaskStatusBlocked               = "blocked"
	TaskStatusStopped               = "stopped"

	ContinuationModeNewTask              = "new_task"
	ContinuationModeContinueSameTask     = "continue_same_task"
	ContinuationModeApprovePendingAction = "approve_pending_action"
	ContinuationModeRejectPendingAction  = "reject_pending_action"
	ContinuationModeAnswerPendingClarify = "answer_pending_clarification"
	ContinuationModeRevisePreviousTask   = "revise_previous_task"
	ContinuationModeAskFollowupPrevious  = "ask_followup_about_previous_result"
	ContinuationModeStopCurrentTask      = "stop_current_task"
	ContinuationModeUnclear              = "unclear"
	ContinuationModeSocialChat           = "social_chat"
	ContinuationModeLastResponseArtifact = "last_response_artifact_action"

	LastOutcomeCompleted = "completed"
	LastOutcomeBlocked   = "blocked"
	LastOutcomeFailed    = "failed"

	ToolLaneChat      = "chat_lane"
	ToolLaneResearch  = "research_lane"
	ToolLaneDocument  = "document_lane"
	ToolLaneCoding    = "coding_lane"
	ToolLaneFileEdit  = "file_edit_lane"
	ToolLaneWebSearch = "web_search_lane"
	ToolLaneWebCrawl  = "web_crawl_lane"
	ToolLaneScheduler = "scheduler_lane"
	ToolLaneExtension = "extension_lane"
	ToolLaneConnector = "connector_lane"
	ToolLaneSettings  = "settings_lane"
)

const (
	SessionContractActiveTTL               = 2 * time.Hour
	SessionContractBlockedTTL              = 30 * time.Minute
	SessionContractPendingApprovalTTL      = 30 * time.Minute
	SessionContractPendingClarificationTTL = 2 * time.Hour
)

type SessionContract struct {
	ActiveGoal           string                `json:"active_goal,omitempty"`
	ActiveRoute          string                `json:"active_route,omitempty"`
	ActiveDomain         string                `json:"active_domain,omitempty"`
	ActiveTaskFrame      TaskFrame             `json:"active_task_frame,omitempty"`
	ActiveTarget         string                `json:"active_target,omitempty"`
	ActiveCapability     string                `json:"active_capability,omitempty"`
	ActiveLane           string                `json:"active_lane,omitempty"`
	TaskStatus           string                `json:"task_status,omitempty"`
	ContinuationMode     string                `json:"continuation_mode,omitempty"`
	LastPlan             string                `json:"last_plan,omitempty"`
	CompletedSteps       []string              `json:"completed_steps,omitempty"`
	PendingStep          string                `json:"pending_step,omitempty"`
	PendingClarification string                `json:"pending_clarification,omitempty"`
	PendingApproval      string                `json:"pending_approval,omitempty"`
	AllowedToolset       []string              `json:"allowed_toolset,omitempty"`
	BlockedToolset       []string              `json:"blocked_toolset,omitempty"`
	ContextSources       []string              `json:"context_sources,omitempty"`
	RouteConfidence      int                   `json:"route_confidence,omitempty"`
	AmbiguityFlags       []string              `json:"ambiguity_flags,omitempty"`
	LastOutcome          string                `json:"last_outcome,omitempty"`
	FailureReason        string                `json:"failure_reason,omitempty"`
	LastRecovery         RouteRecoveryDecision `json:"last_recovery,omitempty"`
	UpdatedAt            string                `json:"updated_at,omitempty"`
}

type ContinuationResolution struct {
	Mode                  string   `json:"mode"`
	Confidence            int      `json:"confidence"`
	Reasons               []string `json:"reasons,omitempty"`
	KeepActiveRoute       bool     `json:"keep_active_route,omitempty"`
	NeedsClarification    bool     `json:"needs_clarification,omitempty"`
	ClarificationQuestion string   `json:"clarification_question,omitempty"`
	AmbiguityFlags        []string `json:"ambiguity_flags,omitempty"`
}

type RouteCandidate struct {
	Route                string   `json:"route"`
	Confidence           int      `json:"confidence"`
	Reason               string   `json:"reason,omitempty"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	RequiredTools        []string `json:"required_tools,omitempty"`
	RiskLevel            string   `json:"risk_level,omitempty"`
	ClarificationNeeded  bool     `json:"clarification_needed,omitempty"`
	ApprovalNeeded       bool     `json:"approval_needed,omitempty"`
	SourceOfTruth        string   `json:"source_of_truth,omitempty"`
	ToolLane             string   `json:"tool_lane,omitempty"`
}

type ToolLane struct {
	Name                string   `json:"name"`
	AllowedTools        []string `json:"allowed_tools,omitempty"`
	BlockedTools        []string `json:"blocked_tools,omitempty"`
	RequiresApproval    bool     `json:"requires_approval,omitempty"`
	ContextCompilerMode string   `json:"context_compiler_mode,omitempty"`
	VerificationRule    string   `json:"verification_rule,omitempty"`
	ResponseContract    string   `json:"response_contract,omitempty"`
}

func (contract SessionContract) IsZero() bool {
	return strings.TrimSpace(contract.ActiveGoal) == "" &&
		strings.TrimSpace(contract.ActiveRoute) == "" &&
		strings.TrimSpace(contract.TaskStatus) == "" &&
		strings.TrimSpace(contract.PendingClarification) == "" &&
		strings.TrimSpace(contract.PendingApproval) == ""
}

func ResolveContinuation(content string, contract SessionContract, frame ContinuationFrame) ContinuationResolution {
	contract = SanitizeSessionContract(contract, time.Now())
	normalized := strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(content)), " "))
	result := ContinuationResolution{
		Mode:       ContinuationModeNewTask,
		Confidence: 80,
	}
	if normalized == "" {
		result.Mode = ContinuationModeUnclear
		result.NeedsClarification = true
		result.ClarificationQuestion = "What would you like Yemaka to do?"
		result.Confidence = 100
		return result
	}
	hasActive := strings.TrimSpace(contract.ActiveRoute) != "" && strings.TrimSpace(contract.TaskStatus) != TaskStatusStopped
	if looksContinuationCancel(normalized) || containsAnyContinuationControlTerm(normalized, "start over", "new task", "change topic") {
		result.Mode = ContinuationModeStopCurrentTask
		result.Confidence = 96
		result.Reasons = append(result.Reasons, "user stopped or reset the active task")
		return result
	}
	if strings.TrimSpace(contract.PendingApproval) != "" {
		switch {
		case looksApprovalCue(normalized):
			result.Mode = ContinuationModeApprovePendingAction
			result.KeepActiveRoute = hasActive
			result.Confidence = 95
			result.Reasons = append(result.Reasons, "user answered pending approval")
			return result
		case looksRejectionCue(normalized):
			result.Mode = ContinuationModeRejectPendingAction
			result.Confidence = 95
			result.Reasons = append(result.Reasons, "user rejected pending approval")
			return result
		}
	}
	if strings.TrimSpace(contract.PendingClarification) != "" && hasActive && !looksExplicitNewTask(normalized) {
		result.Mode = ContinuationModeAnswerPendingClarify
		result.KeepActiveRoute = true
		result.Confidence = 88
		result.Reasons = append(result.Reasons, "user answered pending clarification")
		return result
	}
	if looksLastResponseArtifactAction(normalized) {
		result.Mode = ContinuationModeLastResponseArtifact
		result.KeepActiveRoute = false
		result.Confidence = 92
		result.Reasons = append(result.Reasons, "user requested a new artifact from the last assistant response")
		return result
	}
	if looksSocialChat(normalized) {
		result.Mode = ContinuationModeSocialChat
		result.KeepActiveRoute = false
		result.Confidence = 94
		result.Reasons = append(result.Reasons, "social chat starts a fresh lightweight turn")
		return result
	}
	if hasActive && looksStandaloneNewTask(normalized) {
		result.Mode = ContinuationModeNewTask
		result.KeepActiveRoute = false
		result.Confidence = 90
		result.Reasons = append(result.Reasons, "standalone prompt starts a new task instead of continuing prior route")
		return result
	}
	if frame.Kind == ContinuationKindSourceCorrection {
		result.Mode = ContinuationModeRevisePreviousTask
		result.KeepActiveRoute = false
		result.Confidence = 90
		result.Reasons = append(result.Reasons, "user revised the source of truth")
		return result
	}
	if frame.Kind == ContinuationKindTargetCorrection ||
		containsAnyContinuationControlTerm(normalized, "no, i meant", "no i meant", "i meant", "instead", "the other") {
		result.Mode = ContinuationModeRevisePreviousTask
		result.KeepActiveRoute = hasActive
		result.Confidence = 90
		result.Reasons = append(result.Reasons, "user revised the previous task")
		return result
	}
	if hasActive && looksSameTaskFollowup(normalized) {
		if looksBareContinuationCue(normalized) && !hasPendingOrActiveOperation(contract) {
			result.Mode = ContinuationModeUnclear
			result.KeepActiveRoute = false
			result.NeedsClarification = true
			result.ClarificationQuestion = "What should I continue or approve?"
			result.Confidence = 78
			result.AmbiguityFlags = append(result.AmbiguityFlags, "bare_continuation_without_pending_operation")
			return result
		}
		result.Mode = ContinuationModeContinueSameTask
		result.KeepActiveRoute = true
		result.Confidence = 90
		result.Reasons = append(result.Reasons, "short follow-up continues active route")
		return result
	}
	if hasActive && looksFollowupQuestion(normalized) {
		result.Mode = ContinuationModeAskFollowupPrevious
		result.KeepActiveRoute = true
		result.Confidence = 86
		result.Reasons = append(result.Reasons, "follow-up asks about previous result")
		return result
	}
	if hasActive && looksPriorComparisonFollowup(normalized) {
		result.Mode = ContinuationModeAskFollowupPrevious
		result.KeepActiveRoute = true
		result.Confidence = 84
		result.Reasons = append(result.Reasons, "comparison follow-up keeps active route and target context")
		return result
	}
	if hasActive && looksVagueContinuation(normalized) {
		result.Mode = ContinuationModeUnclear
		result.NeedsClarification = true
		result.ClarificationQuestion = "Should I continue the current task, revise it, or start a new task?"
		result.Confidence = 72
		result.AmbiguityFlags = append(result.AmbiguityFlags, "vague_active_task_followup")
		return result
	}
	return result
}

func SanitizeSessionContract(contract SessionContract, now time.Time) SessionContract {
	contract.ActiveGoal = strings.TrimSpace(contract.ActiveGoal)
	contract.ActiveRoute = strings.TrimSpace(contract.ActiveRoute)
	contract.ActiveDomain = strings.TrimSpace(contract.ActiveDomain)
	contract.ActiveTarget = strings.TrimSpace(contract.ActiveTarget)
	contract.ActiveCapability = strings.TrimSpace(contract.ActiveCapability)
	contract.ActiveLane = strings.TrimSpace(contract.ActiveLane)
	contract.TaskStatus = strings.TrimSpace(contract.TaskStatus)
	contract.ContinuationMode = strings.TrimSpace(contract.ContinuationMode)
	contract.LastPlan = strings.TrimSpace(contract.LastPlan)
	contract.PendingStep = strings.TrimSpace(contract.PendingStep)
	contract.PendingClarification = strings.TrimSpace(contract.PendingClarification)
	contract.PendingApproval = strings.TrimSpace(contract.PendingApproval)
	contract.LastOutcome = strings.TrimSpace(contract.LastOutcome)
	contract.FailureReason = strings.TrimSpace(contract.FailureReason)
	contract.LastRecovery = sanitizeRouteRecoveryDecision(contract.LastRecovery)
	contract.UpdatedAt = strings.TrimSpace(contract.UpdatedAt)
	if contract.IsZero() {
		return SessionContract{}
	}
	if contract.TaskStatus == TaskStatusStopped {
		return SessionContract{}
	}
	if contract.ActiveRoute == "" && contract.PendingApproval == "" && contract.PendingClarification == "" {
		return SessionContract{}
	}
	updatedAt, ok := parseSessionTimestamp(contract.UpdatedAt)
	if !ok || now.IsZero() || updatedAt.IsZero() || now.Before(updatedAt) {
		return contract
	}
	age := now.Sub(updatedAt)
	switch {
	case contract.PendingApproval != "" && age > SessionContractPendingApprovalTTL:
		return SessionContract{}
	case contract.PendingClarification != "" && age > SessionContractPendingClarificationTTL:
		return SessionContract{}
	case (contract.TaskStatus == TaskStatusBlocked || contract.LastOutcome == LastOutcomeBlocked || contract.LastOutcome == LastOutcomeFailed) &&
		contract.PendingApproval == "" && contract.PendingClarification == "" && age > SessionContractBlockedTTL:
		return SessionContract{}
	case contract.PendingApproval == "" && contract.PendingClarification == "" && age > SessionContractActiveTTL:
		return SessionContract{}
	default:
		return contract
	}
}

func parseSessionTimestamp(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func BuildRouteCandidates(input Request, frame TaskFrame, preflight PreflightCard, resolution ContinuationResolution) []RouteCandidate {
	var candidates []RouteCandidate
	add := func(candidate RouteCandidate) {
		if strings.TrimSpace(candidate.Route) == "" {
			return
		}
		candidate.Route = strings.TrimSpace(candidate.Route)
		candidate.RiskLevel = firstNonEmptyString(candidate.RiskLevel, firstNonEmptyString(frame.RiskLevel, RiskLevel(input.Content)))
		candidate.ToolLane = firstNonEmptyString(candidate.ToolLane, ToolLaneForRoute(candidate.Route).Name)
		candidate.RequiredCapabilities = appendUniqueRouteValues(candidate.RequiredCapabilities, capabilityForRoute(candidate.Route))
		candidate.RequiredTools = applyToolLaneToTools(candidate.RequiredTools, ToolLaneForRoute(candidate.Route))
		candidates = append(candidates, candidate)
	}
	if resolution.KeepActiveRoute && strings.TrimSpace(input.SessionContract.ActiveRoute) != "" {
		lane := ToolLaneForRoute(input.SessionContract.ActiveRoute)
		add(RouteCandidate{
			Route:                input.SessionContract.ActiveRoute,
			Confidence:           maxInt(resolution.Confidence, 84),
			Reason:               "active session route commitment",
			RequiredCapabilities: []string{firstNonEmptyString(input.SessionContract.ActiveCapability, capabilityForRoute(input.SessionContract.ActiveRoute))},
			RequiredTools:        defaultToolsForRoute(input.SessionContract.ActiveRoute),
			RiskLevel:            firstNonEmptyString(input.SessionContract.ActiveTaskFrame.RiskLevel, RiskLow),
			ApprovalNeeded:       strings.TrimSpace(input.SessionContract.PendingApproval) != "",
			ToolLane:             lane.Name,
			SourceOfTruth:        sourceForRoute(input.SessionContract.ActiveRoute),
		})
	}
	if taskFrameProtected(frame) {
		source, route := preflightProtectedSourceAndRoute(frame)
		add(RouteCandidate{
			Route:                route,
			Confidence:           94,
			Reason:               "protected task frame has priority",
			RequiredCapabilities: []string{frame.Capability},
			RequiredTools:        defaultToolsForRoute(route),
			RiskLevel:            frame.RiskLevel,
			ClarificationNeeded:  frame.RequiresClarification,
			ApprovalNeeded:       frame.RequiresApproval,
			SourceOfTruth:        source,
		})
	}
	for _, candidate := range preflight.RouteCandidates {
		add(RouteCandidate{
			Route:               candidate.Route,
			Confidence:          candidate.Confidence,
			Reason:              strings.Join(candidate.Reasons, "; "),
			RequiredTools:       defaultToolsForRoute(candidate.Route),
			RiskLevel:           preflight.RiskLevel,
			ClarificationNeeded: candidate.Route == RouteClarify || len(preflight.MissingSlots) > 0,
			ApprovalNeeded:      routeRequiresApproval(candidate.Route, frame),
			SourceOfTruth:       candidate.SourceOfTruth,
		})
	}
	if len(candidates) == 0 && strings.TrimSpace(preflight.SelectedRoute) != "" {
		add(RouteCandidate{
			Route:               preflight.SelectedRoute,
			Confidence:          preflight.Confidence,
			Reason:              strings.Join(preflight.Reasons, "; "),
			RequiredTools:       defaultToolsForRoute(preflight.SelectedRoute),
			RiskLevel:           preflight.RiskLevel,
			ClarificationNeeded: preflight.SelectedRoute == RouteClarify || len(preflight.MissingSlots) > 0,
			ApprovalNeeded:      routeRequiresApproval(preflight.SelectedRoute, frame),
			SourceOfTruth:       preflight.SourceOfTruth,
		})
	}
	return candidates
}

func ArbitrateRoute(input Request, frame TaskFrame, preflight PreflightCard, resolution ContinuationResolution, candidates []RouteCandidate) (Decision, bool) {
	if resolution.Mode == ContinuationModeStopCurrentTask {
		return Decision{
			TaskType:              TaskChat,
			Confidence:            resolution.Confidence,
			Reasons:               append([]string{"routing control plane stopped active task"}, resolution.Reasons...),
			NeedsClarification:    true,
			ClarificationQuestion: "Okay. What would you like to do next?",
			RouteCategory:         RouteClarify,
			Intent:                IntentClarify,
			Domain:                DomainGeneral,
			ContinuationMode:      resolution.Mode,
			AmbiguityFlags:        resolution.AmbiguityFlags,
		}, true
	}
	if resolution.NeedsClarification {
		return Decision{
			TaskType:              TaskChat,
			Confidence:            resolution.Confidence,
			Reasons:               append([]string{"routing control plane requires clarification"}, resolution.Reasons...),
			NeedsClarification:    true,
			ClarificationQuestion: resolution.ClarificationQuestion,
			RouteCategory:         RouteClarify,
			Intent:                IntentClarify,
			Domain:                firstNonEmptyString(input.SessionContract.ActiveDomain, DomainGeneral),
			ContinuationMode:      resolution.Mode,
			AmbiguityFlags:        resolution.AmbiguityFlags,
		}, true
	}
	if resolution.KeepActiveRoute && strings.TrimSpace(input.SessionContract.ActiveRoute) != "" {
		candidate := bestCandidate(candidates)
		if candidate.Route == "" || candidate.Route != input.SessionContract.ActiveRoute {
			candidate = RouteCandidate{
				Route:                input.SessionContract.ActiveRoute,
				Confidence:           resolution.Confidence,
				Reason:               "active route commitment",
				RequiredCapabilities: []string{input.SessionContract.ActiveCapability},
				RequiredTools:        defaultToolsForRoute(input.SessionContract.ActiveRoute),
				RiskLevel:            firstNonEmptyString(input.SessionContract.ActiveTaskFrame.RiskLevel, RiskLow),
				ToolLane:             input.SessionContract.ActiveLane,
			}
		}
		return decisionFromRouteCandidate(input, frame, preflight, resolution, candidate), true
	}
	best, second := topTwoCandidates(candidates)
	if best.Route != "" && best.Confidence < 65 {
		return lowConfidenceClarificationDecision(input, frame, preflight, resolution, best, "route confidence is below the safe threshold"), true
	}
	if routeCandidatesConflict(best, second) && best.Confidence < 86 && best.Confidence-second.Confidence <= 10 {
		return lowConfidenceClarificationDecision(input, frame, preflight, resolution, best, "route candidates conflict"), true
	}
	return Decision{}, false
}

func lowConfidenceClarificationDecision(input Request, frame TaskFrame, preflight PreflightCard, resolution ContinuationResolution, candidate RouteCandidate, reason string) Decision {
	confidence := candidate.Confidence
	if confidence <= 0 {
		confidence = resolution.Confidence
	}
	return Decision{
		TaskType:              TaskChat,
		Confidence:            confidence,
		Reasons:               append([]string{"routing control plane requires clarification: " + reason}, resolution.Reasons...),
		NeedsClarification:    true,
		ClarificationQuestion: routeCandidateClarificationQuestion(input, preflight, candidate),
		RouteCategory:         RouteClarify,
		Intent:                IntentClarify,
		Domain:                firstNonEmptyString(input.SessionContract.ActiveDomain, preflight.Domain, frame.Domain, DomainGeneral),
		TaskFrame:             frame,
		Preflight:             preflight,
		Continuation:          input.Continuation,
		ContinuationMode:      firstNonEmptyString(resolution.Mode, ContinuationModeUnclear),
		AmbiguityFlags:        appendUniqueRouteValues(resolution.AmbiguityFlags, "route_candidate_conflict"),
		RouteCandidates:       []RouteCandidate{candidate},
	}
}

func routeCandidateClarificationQuestion(input Request, preflight PreflightCard, candidate RouteCandidate) string {
	source := firstNonEmptyString(candidate.SourceOfTruth, preflight.SourceOfTruth, sourceForRoute(candidate.Route))
	activeRoute := strings.TrimSpace(input.SessionContract.ActiveRoute)
	activeSource := sourceForRoute(activeRoute)
	if activeRoute != "" && source != "" && activeSource != "" && activeSource != source {
		return "Should I continue the current " + sourceLabelForClarification(activeSource) + " task, or switch this turn to " + sourceLabelForClarification(source) + "?"
	}
	if containsPreflightValue(preflight.MissingSlots, "current_news_topic") {
		return "What current-news topic should I search for?"
	}
	if containsPreflightValue(preflight.MissingSlots, "ambiguous_prior_target") {
		return "Which previous target should I use for this follow-up?"
	}
	if source != "" && source != PreflightSourceChat && source != PreflightSourceClarify {
		return "Should I use " + sourceLabelForClarification(source) + " for this request, or should I just answer without tools?"
	}
	return "Should Yemaka continue the current task, use local documents, use workspace files, use web search, or just answer without tools?"
}

func sourceLabelForClarification(source string) string {
	switch strings.TrimSpace(source) {
	case PreflightSourceLocalDocuments:
		return "local documents"
	case PreflightSourceMemory:
		return "saved memory"
	case PreflightSourceWorkspace:
		return "workspace files"
	case PreflightSourceInternet:
		return "web search"
	case PreflightSourceScheduler:
		return "scheduler"
	case PreflightSourceExtension:
		return "extension generation"
	case PreflightSourceConnector:
		return "connector"
	default:
		return "chat"
	}
}

func routeCandidatesConflict(first RouteCandidate, second RouteCandidate) bool {
	if strings.TrimSpace(first.Route) == "" || strings.TrimSpace(second.Route) == "" || first.Route == second.Route {
		return false
	}
	if sourceForRoute(first.Route) != sourceForRoute(second.Route) {
		return true
	}
	return ToolLaneForRoute(first.Route).Name != ToolLaneForRoute(second.Route).Name
}

func decisionFromRouteCandidate(input Request, frame TaskFrame, preflight PreflightCard, resolution ContinuationResolution, candidate RouteCandidate) Decision {
	route := strings.TrimSpace(candidate.Route)
	task := taskForRoute(route)
	tools := applyToolLaneToTools(candidate.RequiredTools, ToolLaneForRoute(route))
	decision := Decision{
		TaskType:              task,
		Confidence:            candidate.Confidence,
		Reasons:               []string{firstNonEmptyString(candidate.Reason, "routing control plane selected candidate")},
		Tools:                 tools,
		RiskLevel:             firstNonEmptyString(candidate.RiskLevel, RiskLow),
		RouteCategory:         route,
		Intent:                intentForRoute(route),
		Domain:                firstNonEmptyString(input.SessionContract.ActiveDomain, preflight.Domain, frame.Domain, DomainGeneral),
		Target:                firstNonEmptyString(input.SessionContract.ActiveTarget, preflight.Target, frame.Target),
		Capability:            firstNonEmptyString(firstRouteValue(candidate.RequiredCapabilities), capabilityForRoute(route)),
		RequiresApproval:      candidate.ApprovalNeeded,
		NeedsClarification:    candidate.ClarificationNeeded,
		ClarificationQuestion: preflightClarificationQuestion(preflight),
		TaskFrame:             frame,
		Preflight:             preflight,
		PreflightSelected:     true,
		Continuation:          input.Continuation,
		ContinuationMode:      resolution.Mode,
		AmbiguityFlags:        resolution.AmbiguityFlags,
		ToolLane:              ToolLaneForRoute(route).Name,
		AllowedToolset:        ToolLaneForRoute(route).AllowedTools,
		BlockedToolset:        ToolLaneForRoute(route).BlockedTools,
		RouteCandidates:       append([]RouteCandidate{}, candidate),
	}
	if decision.NeedsClarification && strings.TrimSpace(decision.ClarificationQuestion) == "" {
		decision.ClarificationQuestion = "What should Yemaka use as the target or source for this request?"
	}
	return decision
}

func UpdateSessionContract(previous SessionContract, decision Decision, update SessionUpdate) SessionContract {
	if strings.TrimSpace(decision.ContinuationMode) == ContinuationModeStopCurrentTask {
		return SessionContract{TaskStatus: TaskStatusStopped, ContinuationMode: ContinuationModeStopCurrentTask, UpdatedAt: nowSessionTimestamp()}
	}
	lane := ToolLaneForRoute(decision.RouteCategory)
	next := SessionContract{
		ActiveGoal:           strings.TrimSpace(update.Goal),
		ActiveRoute:          strings.TrimSpace(decision.RouteCategory),
		ActiveDomain:         strings.TrimSpace(decision.Domain),
		ActiveTaskFrame:      decision.TaskFrame,
		ActiveTarget:         strings.TrimSpace(decision.Target),
		ActiveCapability:     strings.TrimSpace(decision.Capability),
		ActiveLane:           lane.Name,
		TaskStatus:           TaskStatusActive,
		ContinuationMode:     firstNonEmptyString(decision.ContinuationMode, previous.ContinuationMode),
		LastPlan:             strings.TrimSpace(update.LastPlan),
		CompletedSteps:       appendUniqueRouteValues(previous.CompletedSteps, update.CompletedStep),
		PendingStep:          strings.TrimSpace(update.PendingStep),
		PendingClarification: "",
		PendingApproval:      "",
		AllowedToolset:       append([]string{}, lane.AllowedTools...),
		BlockedToolset:       append([]string{}, lane.BlockedTools...),
		ContextSources:       appendUniqueRouteValues(previous.ContextSources, update.ContextSources...),
		RouteConfidence:      decision.Confidence,
		AmbiguityFlags:       appendUniqueRouteValues(decision.AmbiguityFlags, update.AmbiguityFlags...),
		LastOutcome:          strings.TrimSpace(update.LastOutcome),
		FailureReason:        strings.TrimSpace(update.FailureReason),
		LastRecovery:         sanitizeRouteRecoveryDecision(update.LastRecovery),
		UpdatedAt:            nowSessionTimestamp(),
	}
	if next.ActiveGoal == "" {
		next.ActiveGoal = previous.ActiveGoal
	}
	if next.LastPlan == "" {
		next.LastPlan = previous.LastPlan
	}
	if decision.NeedsClarification {
		next.TaskStatus = TaskStatusAwaitingClarification
		next.PendingClarification = strings.TrimSpace(decision.ClarificationQuestion)
	}
	if strings.TrimSpace(update.PendingApproval) != "" {
		next.TaskStatus = TaskStatusAwaitingApproval
		next.PendingApproval = strings.TrimSpace(update.PendingApproval)
	}
	if update.LastOutcome == LastOutcomeBlocked || update.LastOutcome == LastOutcomeFailed {
		next.TaskStatus = TaskStatusBlocked
	}
	return next
}

type SessionUpdate struct {
	Goal            string
	LastPlan        string
	CompletedStep   string
	PendingStep     string
	PendingApproval string
	ContextSources  []string
	AmbiguityFlags  []string
	LastOutcome     string
	FailureReason   string
	LastRecovery    RouteRecoveryDecision
}

func sanitizeRouteRecoveryDecision(decision RouteRecoveryDecision) RouteRecoveryDecision {
	decision.Trigger = strings.TrimSpace(decision.Trigger)
	decision.PreviousRoute = strings.TrimSpace(decision.PreviousRoute)
	decision.ProposedRoute = strings.TrimSpace(decision.ProposedRoute)
	decision.Reason = strings.TrimSpace(decision.Reason)
	decision.FinalAction = strings.TrimSpace(decision.FinalAction)
	if decision.Confidence < 0 {
		decision.Confidence = 0
	}
	if decision.Confidence > 100 {
		decision.Confidence = 100
	}
	if decision.IsZero() {
		return RouteRecoveryDecision{}
	}
	return decision
}

func ToolLaneForRoute(route string) ToolLane {
	switch strings.TrimSpace(route) {
	case RouteRAGSearch:
		return ToolLane{Name: ToolLaneDocument, AllowedTools: []string{"rag_search", "ingest_documents"}, BlockedTools: internetToolNames(), ContextCompilerMode: "local_documents", VerificationRule: "cite_local_document_sources", ResponseContract: "answer only from retrieved local document evidence"}
	case RouteMemorySearch:
		return ToolLane{Name: ToolLaneResearch, AllowedTools: []string{"memory_search"}, BlockedTools: internetToolNames(), ContextCompilerMode: "memory", VerificationRule: "memory_evidence_required", ResponseContract: "answer only from matching saved memory"}
	case RouteWorkspaceRead, RouteFileRead:
		return ToolLane{Name: ToolLaneResearch, AllowedTools: []string{"read_file", "search_files", "list_files", "file_stat", "file_tree", "project_map", "symbol_search", "git_status", "git_diff", "run_tests", "patch_preview"}, BlockedTools: internetToolNames(), ContextCompilerMode: "workspace", VerificationRule: "workspace_source_required", ResponseContract: "use workspace files only when retrieved or read"}
	case RouteFileWrite:
		return ToolLane{Name: ToolLaneFileEdit, AllowedTools: []string{"read_file", "edit_file", "patch_preview"}, RequiresApproval: true, ContextCompilerMode: "workspace", VerificationRule: "snapshot_diff_approval_required", ResponseContract: "show proposal and wait for approval before writing"}
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
		return ToolLane{Name: ToolLaneWebSearch, AllowedTools: internetToolNames(), ContextCompilerMode: "internet", VerificationRule: "fresh_source_required", ResponseContract: "use configured/approved search or clearly decline"}
	case RouteCrawlerTask:
		return ToolLane{Name: ToolLaneWebCrawl, AllowedTools: []string{"internet_crawl"}, RequiresApproval: true, ContextCompilerMode: "internet", VerificationRule: "crawler_policy_required", ResponseContract: "propose bounded crawl before running"}
	case RouteSchedulerCreate:
		return ToolLane{Name: ToolLaneScheduler, AllowedTools: []string{"safe_tool"}, RequiresApproval: true, ContextCompilerMode: "scheduler", VerificationRule: "job_approval_required", ResponseContract: "create jobs only after explicit approval"}
	case RouteExtensionGenerate, RouteExtensionRun:
		return ToolLane{Name: ToolLaneExtension, AllowedTools: []string{"safe_tool"}, RequiresApproval: true, ContextCompilerMode: "extension", VerificationRule: "manifest_tests_policy_required", ResponseContract: "generate/register only after approval and tests"}
	case RouteConnectorAction:
		return ToolLane{Name: ToolLaneConnector, AllowedTools: []string{"safe_tool"}, RequiresApproval: true, ContextCompilerMode: "connector", VerificationRule: "connector_policy_required", ResponseContract: "connector actions remain disabled/approval-gated"}
	case RouteSettingsAction, RouteLocalTime, RouteHeartbeatStatus, RouteModelSettings, RouteSkillAction, RouteLearningAction:
		return ToolLane{Name: ToolLaneSettings, AllowedTools: []string{"ingest_documents", "local_time", "doctor_status", "heartbeat_status", "safe_tool"}, ContextCompilerMode: "settings", VerificationRule: "local_tool_status_required", ResponseContract: "use local settings/tools and explain required user action"}
	case RouteChatExplanation, RouteClarify:
		return ToolLane{Name: ToolLaneChat, BlockedTools: append(internetToolNames(), "read_file", "search_files", "rag_search", "edit_file", "run_shell", "safe_tool"), ContextCompilerMode: "chat", VerificationRule: "no_tool_required", ResponseContract: "ask or answer without tool claims"}
	default:
		return ToolLane{Name: ToolLaneChat, ContextCompilerMode: "chat", VerificationRule: "no_unapproved_tool", ResponseContract: "do not expose tools outside route policy"}
	}
}

func ApplyToolLane(tools []string, route string) []string {
	return applyToolLaneToTools(tools, ToolLaneForRoute(route))
}

func applyToolLaneToTools(tools []string, lane ToolLane) []string {
	if len(tools) == 0 {
		return nil
	}
	allowed := map[string]bool{}
	for _, tool := range lane.AllowedTools {
		allowed[strings.TrimSpace(tool)] = true
	}
	blocked := map[string]bool{}
	for _, tool := range lane.BlockedTools {
		blocked[strings.TrimSpace(tool)] = true
	}
	out := make([]string, 0, len(tools))
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" || blocked[tool] {
			continue
		}
		if len(allowed) > 0 && !allowed[tool] {
			continue
		}
		out = appendPreflightUnique(out, tool)
	}
	return out
}

func defaultToolsForRoute(route string) []string {
	switch route {
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
	case RouteFileWrite:
		return []string{"edit_file"}
	case RouteSettingsAction:
		return []string{"ingest_documents"}
	case RouteLocalTime:
		return []string{"local_time"}
	case RouteHeartbeatStatus:
		return []string{"heartbeat_status"}
	case RouteSchedulerCreate, RouteExtensionGenerate, RouteConnectorAction:
		return []string{"safe_tool"}
	case RouteCrawlerTask:
		return []string{"internet_crawl"}
	default:
		return nil
	}
}

func taskForRoute(route string) string {
	switch route {
	case RouteRAGSearch:
		return TaskRAG
	case RouteChatExplanation:
		return TaskReasoning
	case RouteClarify, RouteAuthorizationRequired, RouteActiveAssessmentRequiresScope:
		return TaskChat
	default:
		return TaskTool
	}
}

func intentForRoute(route string) string {
	switch route {
	case RouteChatExplanation:
		return IntentExplain
	case RouteClarify, RouteAuthorizationRequired, RouteActiveAssessmentRequiresScope:
		return IntentClarify
	case RouteSchedulerCreate:
		return IntentSchedule
	case RouteExtensionGenerate:
		return IntentGenerate
	case RouteFileWrite:
		return IntentEdit
	default:
		return IntentResearch
	}
}

func capabilityForRoute(route string) string {
	switch route {
	case RouteRAGSearch:
		return "rag"
	case RouteMemorySearch:
		return "memory"
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
		return "internet"
	case RouteWorkspaceRead, RouteFileRead:
		return "filesystem_read"
	case RouteFileWrite:
		return "filesystem_write"
	case RouteSchedulerCreate:
		return "scheduler"
	case RouteExtensionGenerate, RouteExtensionRun:
		return "extension_generation"
	case RouteConnectorAction:
		return "connector"
	case RouteLocalTime:
		return "local_time"
	case RouteSettingsAction:
		return "document_ingestion"
	default:
		return ""
	}
}

func routeRequiresApproval(route string, frame TaskFrame) bool {
	return frame.RequiresApproval ||
		ToolLaneForRoute(route).RequiresApproval ||
		route == RouteFileWrite ||
		route == RouteSchedulerCreate ||
		route == RouteExtensionGenerate ||
		route == RouteConnectorAction ||
		route == RouteCrawlerTask
}

func sourceForRoute(route string) string {
	switch route {
	case RouteRAGSearch:
		return PreflightSourceLocalDocuments
	case RouteMemorySearch:
		return PreflightSourceMemory
	case RouteWorkspaceRead, RouteFileRead, RouteFileWrite:
		return PreflightSourceWorkspace
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead, RouteCrawlerTask:
		return PreflightSourceInternet
	case RouteSchedulerCreate:
		return PreflightSourceScheduler
	case RouteExtensionGenerate, RouteExtensionRun:
		return PreflightSourceExtension
	case RouteConnectorAction:
		return PreflightSourceConnector
	default:
		return PreflightSourceChat
	}
}

func bestCandidate(candidates []RouteCandidate) RouteCandidate {
	best := RouteCandidate{}
	for _, candidate := range candidates {
		if candidate.Confidence > best.Confidence {
			best = candidate
		}
	}
	return best
}

func topTwoCandidates(candidates []RouteCandidate) (RouteCandidate, RouteCandidate) {
	first := RouteCandidate{}
	second := RouteCandidate{}
	for _, candidate := range candidates {
		if candidate.Confidence > first.Confidence {
			second = first
			first = candidate
		} else if candidate.Confidence > second.Confidence {
			second = candidate
		}
	}
	return first, second
}

func appendUniqueRouteValues(values []string, additions ...string) []string {
	out := append([]string{}, values...)
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition == "" {
			continue
		}
		exists := false
		for _, existing := range out {
			if strings.TrimSpace(existing) == addition {
				exists = true
				break
			}
		}
		if !exists {
			out = append(out, addition)
		}
	}
	return out
}

func firstRouteValue(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func internetToolNames() []string {
	return []string{"internet_search", "internet_fetch", "internet_head", "internet_crawl"}
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func nowSessionTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func looksApprovalCue(content string) bool {
	return content == "yes" ||
		content == "y" ||
		content == "approve" ||
		content == "approved" ||
		content == "apply it" ||
		content == "go ahead" ||
		content == "continue" ||
		containsAnyContinuationControlTerm(content, "i approve", "i authorize", "authorized", "yes continue", "yes apply it")
}

func looksRejectionCue(content string) bool {
	return content == "no" ||
		content == "reject" ||
		content == "cancel" ||
		content == "stop" ||
		containsAnyContinuationControlTerm(content, "do not", "don't", "not approved", "reject it")
}

func looksSameTaskFollowup(content string) bool {
	return content == "yes" ||
		content == "continue" ||
		content == "next" ||
		content == "again" ||
		content == "try again" ||
		content == "do the same" ||
		containsAnyContinuationControlTerm(content, "do the same for", "same for this", "ask me another", "another one", "explain number", "number 3", "same as before")
}

func looksBareContinuationCue(content string) bool {
	switch strings.TrimSpace(content) {
	case "yes", "ok", "okay":
		return true
	default:
		return false
	}
}

func hasPendingOrActiveOperation(contract SessionContract) bool {
	if strings.TrimSpace(contract.PendingApproval) != "" || strings.TrimSpace(contract.PendingClarification) != "" || strings.TrimSpace(contract.PendingStep) != "" {
		return true
	}
	status := strings.TrimSpace(contract.TaskStatus)
	if status == TaskStatusAwaitingApproval || status == TaskStatusAwaitingClarification {
		return true
	}
	if status == TaskStatusActive && strings.TrimSpace(contract.LastOutcome) == "" {
		return true
	}
	return false
}

func looksFollowupQuestion(content string) bool {
	return strings.HasPrefix(content, "what about ") ||
		strings.HasPrefix(content, "how about ") ||
		strings.HasPrefix(content, "and ") ||
		strings.HasPrefix(content, "also ") ||
		containsAnyContinuationControlTerm(content, "what about that", "what about the", "explain that", "explain this")
}

func looksPriorComparisonFollowup(content string) bool {
	return containsAnyContinuationControlTerm(content,
		"compare it with",
		"compare this with",
		"compare that with",
		"compare them",
		"compare to the previous",
		"compare with the previous",
		"previous one",
		"the previous result",
	)
}

func looksVagueContinuation(content string) bool {
	return content == "that" ||
		content == "this" ||
		content == "it" ||
		content == "what about that?" ||
		content == "what about it?"
}

func looksExplicitNewTask(content string) bool {
	return containsAnyContinuationControlTerm(content, "new task", "change topic", "start over") ||
		(LooksSpecificCurrentNewsTask(content) || LooksCurrentWebFactTask(content) || LooksDocumentIngestAction(content) || LooksFileEditIntent(content) || LooksExtensionGenerationTask(content))
}

func looksStandaloneNewTask(content string) bool {
	if looksExplicitNewTask(content) {
		return true
	}
	if looksFollowupQuestion(content) || looksVagueContinuation(content) || looksPriorComparisonFollowup(content) {
		return false
	}
	if strings.HasPrefix(content, "what is ") ||
		strings.HasPrefix(content, "what are ") ||
		strings.HasPrefix(content, "who is ") ||
		strings.HasPrefix(content, "where is ") ||
		strings.HasPrefix(content, "when is ") ||
		strings.HasPrefix(content, "why is ") ||
		strings.HasPrefix(content, "how do ") ||
		strings.HasPrefix(content, "how does ") ||
		strings.HasPrefix(content, "explain ") ||
		strings.HasPrefix(content, "define ") ||
		strings.HasPrefix(content, "summarize ") {
		return true
	}
	return looksSimpleCalculation(content)
}

func looksLastResponseArtifactAction(content string) bool {
	if !containsAnyContinuationControlTerm(content, "last response", "previous response", "last answer", "previous answer", "last assistant response", "previous assistant response") {
		return false
	}
	return containsAnyContinuationControlTerm(content, "create file", "write", "save", "export", "insert", "put", "copy")
}

func looksSocialChat(content string) bool {
	switch strings.Trim(strings.TrimSpace(content), ".!?") {
	case "hi", "hello", "hey", "how are you", "thanks", "thank you", "good morning", "good afternoon", "good evening":
		return true
	default:
		return false
	}
}

func looksSimpleCalculation(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" || len(content) > 80 {
		return false
	}
	hasDigit := false
	hasOperator := false
	for _, r := range content {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case strings.ContainsRune("+-*/=×÷", r):
			hasOperator = true
		case r == ' ' || r == '.' || r == '?' || r == ',':
		default:
			return false
		}
	}
	return hasDigit && hasOperator
}

func containsAnyContinuationControlTerm(content string, terms ...string) bool {
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if content == term || ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}
