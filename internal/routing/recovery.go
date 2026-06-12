package routing

import "strings"

const (
	RouteRecoveryTriggerProviderUnavailable        = "provider_unavailable"
	RouteRecoveryTriggerMissingPermission          = "missing_permission"
	RouteRecoveryTriggerMissingCapability          = "missing_capability"
	RouteRecoveryTriggerWrongSourceType            = "wrong_source_type"
	RouteRecoveryTriggerTargetNotFound             = "target_not_found"
	RouteRecoveryTriggerUserCorrection             = "user_correction"
	RouteRecoveryTriggerQAReviewRouteMismatch      = "qa_review_route_mismatch"
	RouteRecoveryTriggerToolResultContradictsRoute = "tool_result_contradicts_route"

	RouteRecoveryActionContinueSameRoute      = "continue_same_route"
	RouteRecoveryActionSwitchToSaferRoute     = "switch_to_safer_route"
	RouteRecoveryActionAskClarification       = "ask_clarification"
	RouteRecoveryActionRequestApproval        = "request_approval"
	RouteRecoveryActionRequestConfiguration   = "request_configuration"
	RouteRecoveryActionBlock                  = "block"
	RouteRecoveryActionFailWithProductMessage = "fail_with_product_message"
)

type RouteRecoveryInput struct {
	UserPrompt        string
	SelectedRoute     string
	SelectedLane      string
	SelectedRiskLevel string
	SelectedSource    string
	ToolName          string
	ToolStatus        string
	ErrorText         string
	ResultSourceKind  string
	ResultSummary     string
	ProposedRoute     string
	ExpectedRoute     string
	QAReviewCategory  string
	Continuation      ContinuationFrame
}

type RouteRecoveryDecision struct {
	Trigger               string `json:"trigger,omitempty"`
	PreviousRoute         string `json:"previous_route,omitempty"`
	ProposedRoute         string `json:"proposed_route,omitempty"`
	Reason                string `json:"reason,omitempty"`
	Confidence            int    `json:"confidence,omitempty"`
	SafeToContinue        bool   `json:"safe_to_continue,omitempty"`
	RequiresClarification bool   `json:"requires_clarification,omitempty"`
	RequiresApproval      bool   `json:"requires_approval,omitempty"`
	RequiresConfiguration bool   `json:"requires_configuration,omitempty"`
	FinalAction           string `json:"final_action,omitempty"`
}

func (d RouteRecoveryDecision) IsZero() bool {
	return strings.TrimSpace(d.Trigger) == "" &&
		strings.TrimSpace(d.PreviousRoute) == "" &&
		strings.TrimSpace(d.ProposedRoute) == "" &&
		strings.TrimSpace(d.FinalAction) == ""
}

func RecoverRoute(input RouteRecoveryInput) RouteRecoveryDecision {
	input = normalizeRouteRecoveryInput(input)
	if input.SelectedRoute == "" {
		input.SelectedRoute = RouteChatExplanation
	}
	text := strings.ToLower(strings.Join([]string{
		input.UserPrompt,
		input.ToolName,
		input.ToolStatus,
		input.ErrorText,
		input.ResultSourceKind,
		input.ResultSummary,
		input.QAReviewCategory,
		input.Continuation.Kind,
		input.Continuation.NewSourceOfTruth,
		input.Continuation.NewTarget,
	}, " "))

	if decision, ok := routeRecoveryFromUserCorrection(input); ok {
		return decision
	}
	if decision, ok := routeRecoveryFromQAReview(input); ok {
		return decision
	}
	if routeRecoveryToolContradictsRoute(input) {
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerToolResultContradictsRoute,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "The tool result does not belong to the selected route lane.",
			Confidence:            92,
			RequiresClarification: true,
			FinalAction:           RouteRecoveryActionAskClarification,
		}
	}
	if routeRecoveryLooksProviderUnavailable(text) {
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerProviderUnavailable,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "The selected route needs a configured and reachable provider.",
			Confidence:            88,
			RequiresConfiguration: true,
			FinalAction:           RouteRecoveryActionRequestConfiguration,
		}
	}
	if routeRecoveryLooksMissingPermission(text) {
		reason := "The selected route needs local workspace or OS permission before it can continue."
		if routeRecoveryLooksUploadedCopy(text, input) {
			reason = "The attached upload is a conversation copy, not a granted filesystem path. Use the attached content directly, or grant/select the original folder before filesystem tools can read or edit it."
		}
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerMissingPermission,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                reason,
			Confidence:            88,
			RequiresConfiguration: true,
			FinalAction:           RouteRecoveryActionRequestConfiguration,
		}
	}
	if routeRecoveryLooksMissingCapability(text) {
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerMissingCapability,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "The selected route needs a capability that is not installed, enabled, or available in the current tool lane.",
			Confidence:            86,
			RequiresConfiguration: true,
			FinalAction:           RouteRecoveryActionRequestConfiguration,
		}
	}
	if routeRecoverySourceMismatch(input) {
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerWrongSourceType,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "The result source does not match the selected route source of truth.",
			Confidence:            84,
			RequiresClarification: true,
			FinalAction:           RouteRecoveryActionAskClarification,
		}
	}
	if routeRecoveryLooksTargetNotFound(text) {
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerTargetNotFound,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "The selected target was not found in the current route source.",
			Confidence:            82,
			RequiresClarification: true,
			FinalAction:           RouteRecoveryActionAskClarification,
		}
	}
	return RouteRecoveryDecision{}
}

func normalizeRouteRecoveryInput(input RouteRecoveryInput) RouteRecoveryInput {
	input.UserPrompt = strings.TrimSpace(input.UserPrompt)
	input.SelectedRoute = strings.TrimSpace(input.SelectedRoute)
	input.SelectedLane = strings.TrimSpace(input.SelectedLane)
	input.SelectedRiskLevel = strings.TrimSpace(input.SelectedRiskLevel)
	input.SelectedSource = strings.TrimSpace(input.SelectedSource)
	input.ToolName = strings.TrimSpace(input.ToolName)
	input.ToolStatus = strings.TrimSpace(input.ToolStatus)
	input.ErrorText = strings.TrimSpace(input.ErrorText)
	input.ResultSourceKind = strings.TrimSpace(input.ResultSourceKind)
	input.ResultSummary = strings.TrimSpace(input.ResultSummary)
	input.ProposedRoute = strings.TrimSpace(input.ProposedRoute)
	input.ExpectedRoute = strings.TrimSpace(input.ExpectedRoute)
	input.QAReviewCategory = strings.TrimSpace(input.QAReviewCategory)
	return input
}

func routeRecoveryFromUserCorrection(input RouteRecoveryInput) (RouteRecoveryDecision, bool) {
	kind := strings.TrimSpace(input.Continuation.Kind)
	if kind != ContinuationKindTargetCorrection && kind != ContinuationKindSourceCorrection {
		return RouteRecoveryDecision{}, false
	}
	proposed := firstNonEmptyString(input.ProposedRoute, input.ExpectedRoute)
	if proposed == "" {
		proposed = routeForRecoverySource(input.Continuation.NewSourceOfTruth)
	}
	if proposed == "" {
		proposed = input.SelectedRoute
	}
	decision := RouteRecoveryDecision{
		Trigger:        RouteRecoveryTriggerUserCorrection,
		PreviousRoute:  input.SelectedRoute,
		ProposedRoute:  proposed,
		Reason:         "The user corrected the previous target or source.",
		Confidence:     90,
		FinalAction:    RouteRecoveryActionContinueSameRoute,
		SafeToContinue: true,
	}
	if proposed != input.SelectedRoute {
		if routeRecoverySwitchNeedsApproval(input.SelectedRoute, proposed) {
			decision.SafeToContinue = false
			decision.RequiresApproval = true
			decision.FinalAction = RouteRecoveryActionRequestApproval
			decision.Reason = "The user correction points to a route that needs explicit approval before switching."
			return decision, true
		}
		decision.FinalAction = RouteRecoveryActionSwitchToSaferRoute
	}
	return decision, true
}

func routeRecoveryFromQAReview(input RouteRecoveryInput) (RouteRecoveryDecision, bool) {
	category := strings.TrimSpace(input.QAReviewCategory)
	if category == "" {
		return RouteRecoveryDecision{}, false
	}
	switch category {
	case "wrong_route", "wrong_source", "wrong_tool_lane":
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerQAReviewRouteMismatch,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         firstNonEmptyString(input.ExpectedRoute, input.ProposedRoute, input.SelectedRoute),
			Reason:                "QAReview classified this as a route-governance mismatch.",
			Confidence:            88,
			RequiresClarification: true,
			FinalAction:           RouteRecoveryActionAskClarification,
		}, true
	case "provider_config_issue":
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerProviderUnavailable,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "QAReview classified this as a provider or configuration issue.",
			Confidence:            86,
			RequiresConfiguration: true,
			FinalAction:           RouteRecoveryActionRequestConfiguration,
		}, true
	case "permission_missing":
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerMissingPermission,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "QAReview classified this as a missing local permission or workspace grant.",
			Confidence:            86,
			RequiresConfiguration: true,
			FinalAction:           RouteRecoveryActionRequestConfiguration,
		}, true
	case "missing_capability":
		return RouteRecoveryDecision{
			Trigger:               RouteRecoveryTriggerMissingCapability,
			PreviousRoute:         input.SelectedRoute,
			ProposedRoute:         input.SelectedRoute,
			Reason:                "QAReview classified this as a missing or disabled capability.",
			Confidence:            86,
			RequiresConfiguration: true,
			FinalAction:           RouteRecoveryActionRequestConfiguration,
		}, true
	default:
		return RouteRecoveryDecision{}, false
	}
}

func routeRecoveryToolContradictsRoute(input RouteRecoveryInput) bool {
	tool := strings.TrimSpace(input.ToolName)
	if tool == "" {
		return false
	}
	lane := ToolLaneForRoute(input.SelectedRoute)
	for _, blocked := range lane.BlockedTools {
		if strings.TrimSpace(blocked) == tool {
			return true
		}
	}
	if len(lane.AllowedTools) == 0 {
		return false
	}
	for _, allowed := range lane.AllowedTools {
		if strings.TrimSpace(allowed) == tool {
			return false
		}
	}
	return input.SelectedRoute != "" && input.SelectedRoute != RouteChatExplanation && input.SelectedRoute != RouteClarify
}

func routeRecoveryLooksProviderUnavailable(text string) bool {
	return strings.Contains(text, "provider") ||
		strings.Contains(text, "searxng") ||
		strings.Contains(text, "firecrawl") ||
		strings.Contains(text, "brave") ||
		strings.Contains(text, "api key") ||
		strings.Contains(text, "not configured") ||
		strings.Contains(text, "connection refused") ||
		strings.Contains(text, "context deadline exceeded") ||
		strings.Contains(text, "no such host") ||
		strings.Contains(text, "internet access is disabled")
}

func routeRecoveryLooksMissingPermission(text string) bool {
	return strings.Contains(text, "operation not permitted") ||
		strings.Contains(text, "permission denied") ||
		strings.Contains(text, "workspace grant") ||
		strings.Contains(text, "path is not granted") ||
		strings.Contains(text, "path not granted") ||
		strings.Contains(text, "security-scoped") ||
		strings.Contains(text, "outside workspace") ||
		strings.Contains(text, "folder access")
}

func routeRecoveryLooksMissingCapability(text string) bool {
	return strings.Contains(text, "missing capability") ||
		strings.Contains(text, "no matching safe executor") ||
		strings.Contains(text, "no safe executor") ||
		strings.Contains(text, "unknown tool") ||
		strings.Contains(text, "tool not available") ||
		strings.Contains(text, "not implemented")
}

func routeRecoveryLooksTargetNotFound(text string) bool {
	return strings.Contains(text, "no such file or directory") ||
		strings.Contains(text, "target not found") ||
		strings.Contains(text, "file not found") ||
		strings.Contains(text, "no matching document") ||
		strings.Contains(text, "no matches")
}

func routeRecoveryLooksUploadedCopy(text string, input RouteRecoveryInput) bool {
	if strings.EqualFold(input.ResultSourceKind, "attachment") ||
		strings.Contains(strings.ToLower(input.SelectedSource), "attachment") {
		return true
	}
	return strings.Contains(text, "attachment") ||
		strings.Contains(text, "uploaded copy") ||
		strings.Contains(text, "uploaded file") ||
		strings.Contains(text, "conversation copy")
}

func routeRecoverySourceMismatch(input RouteRecoveryInput) bool {
	selectedSource := strings.TrimSpace(input.SelectedSource)
	if selectedSource == "" {
		selectedSource = sourceForRoute(input.SelectedRoute)
	}
	resultSource := strings.TrimSpace(input.ResultSourceKind)
	if selectedSource == "" || resultSource == "" {
		return false
	}
	resultSource = routeRecoveryNormalizeSource(resultSource)
	selectedSource = routeRecoveryNormalizeSource(selectedSource)
	return resultSource != "" && selectedSource != "" && resultSource != selectedSource
}

func routeRecoveryNormalizeSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "rag", "local_document", "local_documents", "documents":
		return PreflightSourceLocalDocuments
	case "workspace", "file", "files", "local_files":
		return PreflightSourceWorkspace
	case "internet", "web", "search":
		return PreflightSourceInternet
	case "memory":
		return PreflightSourceMemory
	case "attachment", "attachments":
		return "attachment"
	default:
		return strings.TrimSpace(source)
	}
}

func routeForRecoverySource(source string) string {
	switch strings.TrimSpace(source) {
	case PreflightSourceLocalDocuments:
		return RouteRAGSearch
	case PreflightSourceMemory:
		return RouteMemorySearch
	case PreflightSourceWorkspace:
		return RouteWorkspaceRead
	case PreflightSourceInternet:
		return RouteInternetSearch
	default:
		return ""
	}
}

func routeRecoverySwitchNeedsApproval(previousRoute string, proposedRoute string) bool {
	if strings.TrimSpace(proposedRoute) == "" || previousRoute == proposedRoute {
		return false
	}
	if ToolLaneForRoute(proposedRoute).RequiresApproval {
		return true
	}
	return routeRecoveryRouteRisk(proposedRoute) > routeRecoveryRouteRisk(previousRoute)
}

func routeRecoveryRouteRisk(route string) int {
	switch strings.TrimSpace(route) {
	case RouteChatExplanation, RouteClarify, "":
		return 0
	case RouteRAGSearch, RouteMemorySearch, RouteWorkspaceRead, RouteFileRead, RouteLocalTime, RouteHeartbeatStatus, RouteModelSettings, RouteSkillAction, RouteLearningAction, RouteSettingsAction:
		return 1
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
		return 2
	case RouteFileWrite, RouteShellTool, RouteCrawlerTask, RouteSchedulerCreate, RouteExtensionGenerate, RouteExtensionRun, RouteConnectorAction:
		return 3
	default:
		return 1
	}
}
