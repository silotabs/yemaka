package learning

import (
	"fmt"
	"strings"

	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

const (
	RouteFailureWrongRoute          = "wrong_route"
	RouteFailureWrongSource         = "wrong_source"
	RouteFailureWrongToolLane       = "wrong_tool_lane"
	RouteFailureStaleAnswer         = "stale_answer"
	RouteFailureMissingApproval     = "missing_approval"
	RouteFailureMissingEvidence     = "missing_evidence"
	RouteFailureEvidenceContract    = "evidence_contract_mismatch"
	RouteFailureBadClarification    = "bad_clarification"
	RouteFailureProviderConfigIssue = "provider_config_issue"
	RouteFailurePermissionMissing   = "permission_missing"
	RouteFailureMissingCapability   = "missing_capability"
)

const RouteGovernanceRegressionSchema = "yemaka.route_governance_regression.v1"

type RouteFailureClassification struct {
	Category            string                        `json:"category"`
	Reason              string                        `json:"reason"`
	SuggestedRegression string                        `json:"suggestedRegression,omitempty"`
	CoveredByEvalOrTest bool                          `json:"coveredByEvalOrTest"`
	CoverageSources     []string                      `json:"coverageSources,omitempty"`
	RegressionPromotion RouteRegressionPromotion      `json:"regressionPromotion,omitempty"`
	RouteRecovery       routing.RouteRecoveryDecision `json:"routeRecovery,omitempty"`
}

type RouteRegressionPromotion struct {
	SuggestedTestName string   `json:"suggestedTestName,omitempty"`
	RouteUnderTest    string   `json:"routeUnderTest,omitempty"`
	ObservedRoute     string   `json:"observedRoute,omitempty"`
	ExpectedRoute     string   `json:"expectedRoute,omitempty"`
	ExpectedTaskType  string   `json:"expectedTaskType,omitempty"`
	ExpectedRiskLevel string   `json:"expectedRiskLevel,omitempty"`
	ContinuationMode  string   `json:"continuationMode,omitempty"`
	ExpectedSource    string   `json:"expectedSource,omitempty"`
	ExpectedToolLane  string   `json:"expectedToolLane,omitempty"`
	RequiredTools     []string `json:"requiredTools,omitempty"`
	ForbiddenTools    []string `json:"forbiddenTools,omitempty"`
	ExpectedOutcome   string   `json:"expectedOutcome,omitempty"`
}

func ClassifyRouteFailure(trace replay.Trace) RouteFailureClassification {
	trace = trace.Normalize()
	text := routeFailureTraceText(trace)
	category, reason := classifyRouteFailureCategory(trace, text)
	if category == "" {
		category = RouteFailureWrongRoute
		reason = "Replay failed and needs route-governance review."
	}
	classification := RouteFailureClassification{
		Category:            category,
		Reason:              reason,
		SuggestedRegression: routeFailureSuggestedRegression(category),
		CoveredByEvalOrTest: routeFailureCoveredByEvalOrTest(trace),
	}
	if classification.CoveredByEvalOrTest {
		classification.CoverageSources = []string{"trace_metadata"}
	}
	classification.RegressionPromotion = routeFailurePromotion(trace, classification)
	classification.RouteRecovery = routeRecoveryForFailureClassification(trace, classification)
	return classification
}

func routeRecoveryForFailureClassification(trace replay.Trace, classification RouteFailureClassification) routing.RouteRecoveryDecision {
	return routing.RecoverRoute(routing.RouteRecoveryInput{
		UserPrompt:        trace.UserRequest,
		SelectedRoute:     trace.Route.Category,
		SelectedRiskLevel: trace.Route.RiskLevel,
		SelectedSource:    traceAttribute(trace, "route_preflight_source_of_truth"),
		ErrorText:         routeFailureTraceText(trace),
		ResultSourceKind:  traceAttribute(trace, "source_kind"),
		ResultSummary:     trace.FinalResult.OutputSummary,
		ExpectedRoute:     routeFailureExpectedRoute(trace),
		QAReviewCategory:  classification.Category,
	})
}

func ApplyRouteGovernanceCoverage(trace replay.Trace, classification RouteFailureClassification, latestEval *QAEvalSummary) RouteFailureClassification {
	trace = trace.Normalize()
	coverage := RouteGovernanceCoverageFor(classification.Category, latestEval)
	sources := append([]string{}, classification.CoverageSources...)
	sources = appendUniqueStrings(sources, coverage.Sources...)
	if routeFailureCoveredByEvalOrTest(trace) {
		sources = appendUniqueStrings(sources, "trace_metadata")
	}
	classification.CoveredByEvalOrTest = coverage.Covered || len(sources) > 0
	classification.CoverageSources = sources
	return classification
}

type RouteGovernanceCoverage struct {
	Category string   `json:"category"`
	Covered  bool     `json:"covered"`
	Sources  []string `json:"sources,omitempty"`
}

func RouteGovernanceCoverageFor(category string, latestEval *QAEvalSummary) RouteGovernanceCoverage {
	category = strings.TrimSpace(category)
	sources := append([]string{}, routeGovernanceStaticCoverageSources(category)...)
	if routeGovernanceEvalCoversCategory(category, latestEval) {
		sources = appendUniqueStrings(sources, "eval:route_governance_classifier")
	}
	return RouteGovernanceCoverage{
		Category: category,
		Covered:  len(sources) > 0,
		Sources:  sources,
	}
}

func classifyRouteFailureCategory(trace replay.Trace, text string) (string, string) {
	if routeFailureTextHas(text, "wrong tool lane", "tool lane mismatch", "outside selected route lane", "blocked tool lane", "blocked by route lane") ||
		routeFailureCalledOutsideLaneTool(trace) {
		return RouteFailureWrongToolLane, "A tool outside the selected route lane was exposed or called."
	}
	if routeFailureTextHas(text, "operation not permitted", "permission denied", "workspace grant", "path not granted", "security-scoped", "outside workspace", "folder access") {
		return RouteFailurePermissionMissing, "A file or workspace action failed because required local access was missing."
	}
	if routeFailureTextHas(text, "missing approval", "approval required", "without approval", "before approval", "confirmation required", "needs confirmation", "authorize", "authorization required") ||
		routeFailureHasFailedApproval(trace) {
		return RouteFailureMissingApproval, "A risky or approval-gated action was blocked or attempted without the required approval."
	}
	if routeFailureSourceMismatch(trace) || routeFailureTextHas(text, "wrong source", "source mismatch", "source of truth mismatch", "local documents should have been used", "not internet", "not web search", "wrong retrieval source") {
		return RouteFailureWrongSource, "The selected source of truth did not match the task requirements."
	}
	if routeFailureLocalRuntimeProviderIssue(trace, text) {
		return RouteFailureProviderConfigIssue, "The selected route needed a reachable local model or embedding runtime."
	}
	if routeFailureTextHas(text, "provider", "not configured", "needs config", "configuration", "searxng", "search endpoint", "connection refused", "no such host", "api key", "internet access is disabled") &&
		(routeFailureUsesInternet(trace) || routeFailureSource(trace) == routing.PreflightSourceInternet || routeFailureTextHas(text, "internet", "search")) {
		return RouteFailureProviderConfigIssue, "The selected route needed a configured provider or approved internet access."
	}
	if routeFailureTextHas(text, "missing capability", "capability missing", "no capability", "capability not found", "no matching safe executor", "unknown tool", "tool not available", "not implemented", "extension proposal required") {
		return RouteFailureMissingCapability, "The task required a capability that was unavailable or not enabled."
	}
	if routeFailureEvidenceContractMismatch(trace, text) {
		return RouteFailureEvidenceContract, "The selected route allowed general knowledge, but the response treated optional local/workspace evidence as required."
	}
	if routeFailureTextHas(text, "stale answer", "stale local context", "fresh source required", "currentness", "current public fact", "latest requires search") ||
		routeFailureFreshnessMismatch(trace) {
		return RouteFailureStaleAnswer, "The task required fresh/current evidence but did not have an approved fresh source."
	}
	if routeFailureTextHas(text, "missing evidence", "no evidence", "empty required evidence", "no sources", "no retrieved", "no citations", "no local document evidence", "no rag matches", "should cite", "cite or name retrieved", "citation missing", "source citation") ||
		routeFailureRequiredEvidenceEmpty(trace) {
		return RouteFailureMissingEvidence, "The selected route needed evidence, but no usable evidence was available."
	}
	if routeFailureTextHas(text, "bad clarification", "should ask clarification", "clarification missing", "missing clarification", "ambiguous", "missing slots", "unclear follow-up") ||
		routeFailureClarificationMismatch(trace) {
		return RouteFailureBadClarification, "The route should have asked a clearer follow-up before proceeding."
	}
	if routeFailureWrongRoute(trace, text) {
		return RouteFailureWrongRoute, "The selected route did not match the expected route or task frame."
	}
	return "", ""
}

func routeFailurePromotion(trace replay.Trace, classification RouteFailureClassification) RouteRegressionPromotion {
	observedRoute := firstNonEmpty(traceAttribute(trace, "route_preflight_selected_route"), trace.Route.Category, "unknown_route")
	expectedRoute := firstNonEmpty(routeFailureExpectedRoute(trace), observedRoute)
	continuationMode := traceAttribute(trace, "continuation_mode")
	source := firstNonEmpty(traceAttribute(trace, "route_preflight_source_of_truth"), routeFailureSourceForRoute(expectedRoute), traceAttribute(trace, "source_kind"))
	lane := firstNonEmpty(traceAttribute(trace, "route_lane"), routing.ToolLaneForRoute(expectedRoute).Name)
	return RouteRegressionPromotion{
		SuggestedTestName: regressionCaseName("route_governance_"+classification.Category, trace.UserRequest, trace.ID),
		RouteUnderTest:    sanitizeRegressionText(expectedRoute, 120),
		ObservedRoute:     sanitizeRegressionText(observedRoute, 120),
		ExpectedRoute:     sanitizeRegressionText(expectedRoute, 120),
		ExpectedTaskType:  sanitizeRegressionText(traceAttribute(trace, "task_frame_task_type"), 120),
		ExpectedRiskLevel: sanitizeRegressionText(routeFailureExpectedRiskLevel(trace), 80),
		ContinuationMode:  sanitizeRegressionText(continuationMode, 80),
		ExpectedSource:    sanitizeRegressionText(source, 80),
		ExpectedToolLane:  sanitizeRegressionText(lane, 80),
		RequiredTools:     sanitizeRegressionTextList(routeFailureRequiredTools(trace), 120),
		ForbiddenTools:    sanitizeRegressionTextList(routeFailureForbiddenTools(trace, expectedRoute), 120),
		ExpectedOutcome:   routeFailureExpectedOutcome(classification.Category),
	}
}

func FormatRouteGovernanceRegressionYAMLish(name string, trace replay.Trace, classification RouteFailureClassification) string {
	promotion := classification.RegressionPromotion
	if promotion.SuggestedTestName == "" {
		promotion = routeFailurePromotion(trace, classification)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\n", yamlString(RouteGovernanceRegressionSchema))
	fmt.Fprintf(&out, "name: %s\n", yamlString(firstNonEmpty(name, promotion.SuggestedTestName)))
	fmt.Fprintf(&out, "category: %s\n", yamlString(classification.Category))
	fmt.Fprintf(&out, "reason: %s\n", yamlString(classification.Reason))
	fmt.Fprintf(&out, "prompt: %s\n", yamlString(sanitizeRegressionText(trace.UserRequest, 240)))
	fmt.Fprintf(&out, "route_under_test: %s\n", yamlString(promotion.RouteUnderTest))
	fmt.Fprintf(&out, "observed_route: %s\n", yamlString(promotion.ObservedRoute))
	fmt.Fprintf(&out, "expected_route: %s\n", yamlString(promotion.ExpectedRoute))
	fmt.Fprintf(&out, "expected_task_type: %s\n", yamlString(promotion.ExpectedTaskType))
	fmt.Fprintf(&out, "expected_risk_level: %s\n", yamlString(promotion.ExpectedRiskLevel))
	fmt.Fprintf(&out, "continuation_mode: %s\n", yamlString(promotion.ContinuationMode))
	fmt.Fprintf(&out, "expected_source: %s\n", yamlString(promotion.ExpectedSource))
	fmt.Fprintf(&out, "expected_tool_lane: %s\n", yamlString(promotion.ExpectedToolLane))
	writeYAMLStringList(&out, "", "required_tools", promotion.RequiredTools)
	writeYAMLStringList(&out, "", "forbidden_tools", promotion.ForbiddenTools)
	fmt.Fprintf(&out, "expected_outcome: %s\n", yamlString(promotion.ExpectedOutcome))
	fmt.Fprintf(&out, "covered_by_eval_or_test: %t\n", classification.CoveredByEvalOrTest)
	return strings.TrimRight(out.String(), "\n")
}

func routeFailureTraceText(trace replay.Trace) string {
	var parts []string
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				parts = append(parts, value)
			}
		}
	}
	add(trace.UserRequest, trace.Route.Category, trace.Route.Intent, trace.Route.Domain, trace.Route.Target, trace.Route.RiskLevel)
	add(trace.Route.Reasons...)
	add(trace.Plan.Goal)
	add(trace.Plan.Assumptions...)
	add(trace.Plan.Steps...)
	add(trace.Plan.ToolsNeeded...)
	for _, tool := range trace.ToolsCalled {
		add(tool.Name, tool.InputSummary, tool.OutputSummary, tool.Status, tool.RiskLevel, tool.Error)
		for _, attr := range tool.Attributes {
			add(attr.Key, attr.Value)
		}
	}
	for _, tool := range trace.ToolsConsidered {
		add(tool.Name, tool.Source, tool.Status, tool.Reason, tool.RiskLevel)
		for _, attr := range tool.Attributes {
			add(attr.Key, attr.Value)
		}
	}
	for _, permission := range trace.PermissionsRequested {
		add(permission.ToolName, permission.Reason, permission.RiskLevel, permission.Status, permission.PolicyExplanation)
		for _, attr := range permission.Attributes {
			add(attr.Key, attr.Value)
		}
	}
	for _, err := range trace.Errors {
		add(err.Stage, err.Code, err.Subject, err.Message, err.ExpectedRoute, err.ExpectedRiskLevel)
		for _, attr := range err.Attributes {
			add(attr.Key, attr.Value)
		}
	}
	add(trace.FinalResult.Status, trace.FinalResult.Summary, trace.FinalResult.OutputSummary)
	for _, attr := range trace.FinalResult.Attributes {
		add(attr.Key, attr.Value)
	}
	add(trace.Verification.Status)
	add(trace.Verification.Checks...)
	add(trace.Verification.Notes...)
	for _, attr := range trace.Attributes {
		add(attr.Key, attr.Value)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func routeFailureTextHas(text string, terms ...string) bool {
	text = strings.ToLower(text)
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func routeFailureUsesInternet(trace replay.Trace) bool {
	if trace.Route.ShouldUseInternet {
		return true
	}
	for _, tool := range trace.ToolsCalled {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tool.Name)), "internet_") {
			return true
		}
	}
	for _, tool := range trace.Plan.ToolsNeeded {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(tool)), "internet_") {
			return true
		}
	}
	return false
}

func routeFailureLocalRuntimeProviderIssue(trace replay.Trace, text string) bool {
	if routeFailureTextHas(text, "ollama chat request failed", "api/chat", "local model runtime", "selected model is not an installed local model", "selected model is not installed", "model is not installed", "model not found") {
		return true
	}
	if routeFailureTextHas(text, "ollama not reachable") && routeFailureTextHas(text, "ollama") {
		return true
	}
	if !routeFailureTextHas(text, "ollama embed", "api/embed", "embed request", "embedding model", "local embedding") &&
		!(routeFailureTextHas(text, "context deadline exceeded", "client.timeout exceeded") && routeFailureTextHas(text, "rag_search", "embedding", "embed")) {
		return false
	}
	route := firstNonEmpty(trace.Route.Category, traceAttribute(trace, "route_preflight_selected_route"))
	if route == routing.RouteRAGSearch {
		return true
	}
	for _, tool := range trace.ToolsCalled {
		if strings.TrimSpace(tool.Name) == "rag_search" {
			return true
		}
	}
	for _, tool := range trace.Plan.ToolsNeeded {
		if strings.TrimSpace(tool) == "rag_search" {
			return true
		}
	}
	return false
}

func routeFailureHasFailedApproval(trace replay.Trace) bool {
	for _, permission := range trace.PermissionsRequested {
		status := strings.ToLower(strings.TrimSpace(permission.Status))
		if replay.StatusFailed(status) && routeFailureTextHas(status+" "+permission.Reason+" "+permission.PolicyExplanation, "approval", "confirm", "authorize", "denied", "rejected") {
			return true
		}
	}
	return false
}

func routeFailureSourceMismatch(trace replay.Trace) bool {
	source := routeFailureSource(trace)
	if source == "" || source == routing.PreflightSourceClarify {
		return false
	}
	route := firstNonEmpty(trace.Route.Category, traceAttribute(trace, "route_preflight_selected_route"))
	selectedSource := routeFailureSourceForRoute(route)
	if selectedSource == "" || selectedSource == routing.PreflightSourceClarify {
		return false
	}
	if source == routing.PreflightSourceInternet && selectedSource != routing.PreflightSourceInternet && traceAttribute(trace, "route_preflight_freshness_risk") == routing.PreflightFreshnessHigh {
		return false
	}
	return source != selectedSource
}

func routeFailureCalledOutsideLaneTool(trace replay.Trace) bool {
	route := firstNonEmpty(trace.Route.Category, traceAttribute(trace, "route_preflight_selected_route"))
	if route == "" {
		return false
	}
	lane := routing.ToolLaneForRoute(route)
	blocked := map[string]bool{}
	for _, tool := range lane.BlockedTools {
		blocked[strings.TrimSpace(tool)] = true
	}
	allowed := map[string]bool{}
	for _, tool := range lane.AllowedTools {
		allowed[strings.TrimSpace(tool)] = true
	}
	for _, tool := range trace.ToolsCalled {
		if routeFailureToolOutsideLane(strings.TrimSpace(tool.Name), allowed, blocked) {
			return true
		}
	}
	for _, tool := range trace.ToolsConsidered {
		if routeFailureToolOutsideLane(strings.TrimSpace(tool.Name), allowed, blocked) {
			return true
		}
	}
	return false
}

func routeFailureToolOutsideLane(tool string, allowed map[string]bool, blocked map[string]bool) bool {
	if tool == "" {
		return false
	}
	if blocked[tool] {
		return true
	}
	if len(allowed) > 0 && !allowed[tool] {
		return true
	}
	return false
}

func routeFailureFreshnessMismatch(trace replay.Trace) bool {
	if traceAttribute(trace, "route_preflight_freshness_risk") != routing.PreflightFreshnessHigh {
		return false
	}
	route := firstNonEmpty(trace.Route.Category, traceAttribute(trace, "route_preflight_selected_route"))
	return route != routing.RouteInternetSearch && route != routing.RouteInternetFetch && route != routing.RouteInternetHead
}

func routeFailureRequiredEvidenceEmpty(trace replay.Trace) bool {
	route := firstNonEmpty(trace.Route.Category, traceAttribute(trace, "route_preflight_selected_route"))
	switch route {
	case routing.RouteRAGSearch:
		return len(trace.RAGDocsUsed) == 0 && replay.StatusFailed(trace.Verification.Status)
	case routing.RouteMemorySearch:
		return len(trace.MemoryUsed) == 0 && replay.StatusFailed(trace.Verification.Status)
	default:
		return false
	}
}

func routeFailureEvidenceContractMismatch(trace replay.Trace, text string) bool {
	policy := strings.TrimSpace(traceAttribute(trace, "evidence_policy"))
	contract := strings.ToLower(strings.TrimSpace(traceAttribute(trace, "response_contract")))
	if policy != "general_knowledge_allowed" && !strings.Contains(contract, "general model knowledge is allowed") {
		return false
	}
	if routeFailureTextHas(text, "general knowledge route refused because optional local/workspace evidence was absent") {
		return true
	}
	return routeFailureTextHas(text,
		"not present in the available local context",
		"not present in available local context",
		"not present in the current workspace context",
		"not present in current workspace context",
		"not present in the current workspace files",
		"not present in workspace files",
		"cannot provide an explanation based on available evidence",
		"cannot be explained at this time because it is not present",
		"cannot be derived from the current workspace",
		"cannot be generated from the current source of truth",
		"outside the available tools and context",
	)
}

func routeFailureClarificationMismatch(trace replay.Trace) bool {
	if trace.Route.ShouldAskClarification && trace.Route.Category != routing.RouteClarify && replay.StatusFailed(trace.Verification.Status) {
		return true
	}
	if traceAttribute(trace, "route_preflight_missing_slots") != "" && trace.Route.Category != routing.RouteClarify && replay.StatusFailed(trace.Verification.Status) {
		return true
	}
	return false
}

func routeFailureWrongRoute(trace replay.Trace, text string) bool {
	for _, err := range trace.Errors {
		if strings.TrimSpace(err.ExpectedRoute) != "" && strings.TrimSpace(err.ExpectedRoute) != strings.TrimSpace(trace.Route.Category) {
			return true
		}
	}
	return routeFailureTextHas(text, "wrong route", "route mismatch", "expected route", "task frame mismatch")
}

func routeFailureExpectedRoute(trace replay.Trace) string {
	for _, err := range trace.Errors {
		if route := strings.TrimSpace(err.ExpectedRoute); route != "" {
			return route
		}
		for _, attr := range err.Attributes {
			switch strings.TrimSpace(attr.Key) {
			case "expected_route", "expectedRoute":
				if route := strings.TrimSpace(attr.Value); route != "" {
					return route
				}
			}
		}
	}
	for _, key := range []string{"expected_route", "route_expected_route", "task_frame_expected_route"} {
		if route := traceAttribute(trace, key); route != "" {
			return route
		}
	}
	return ""
}

func routeFailureExpectedRiskLevel(trace replay.Trace) string {
	for _, err := range trace.Errors {
		if risk := strings.TrimSpace(err.ExpectedRiskLevel); risk != "" {
			return risk
		}
		for _, attr := range err.Attributes {
			switch strings.TrimSpace(attr.Key) {
			case "expected_risk_level", "expectedRiskLevel":
				if risk := strings.TrimSpace(attr.Value); risk != "" {
					return risk
				}
			}
		}
	}
	return firstNonEmpty(traceAttribute(trace, "expected_risk_level"), trace.Route.RiskLevel)
}

func routeFailureRequiredTools(trace replay.Trace) []string {
	tools := []string{}
	for _, tool := range trace.Plan.ToolsNeeded {
		tools = appendUniqueStrings(tools, tool)
	}
	for _, attr := range trace.Attributes {
		key := strings.TrimSpace(attr.Key)
		if key != "required_tools" && key != "route_required_tools" {
			continue
		}
		for _, tool := range strings.Split(attr.Value, ",") {
			tools = appendUniqueStrings(tools, tool)
		}
	}
	return tools
}

func routeFailureForbiddenTools(trace replay.Trace, expectedRoute string) []string {
	forbidden := []string{}
	lane := routing.ToolLaneForRoute(expectedRoute)
	for _, tool := range lane.BlockedTools {
		forbidden = appendUniqueStrings(forbidden, tool)
	}
	allowed := map[string]bool{}
	for _, tool := range lane.AllowedTools {
		allowed[strings.TrimSpace(tool)] = true
	}
	blocked := map[string]bool{}
	for _, tool := range lane.BlockedTools {
		blocked[strings.TrimSpace(tool)] = true
	}
	for _, tool := range trace.ToolsCalled {
		name := strings.TrimSpace(tool.Name)
		if routeFailureToolOutsideLane(name, allowed, blocked) {
			forbidden = appendUniqueStrings(forbidden, name)
		}
	}
	for _, tool := range trace.ToolsConsidered {
		name := strings.TrimSpace(tool.Name)
		if routeFailureToolOutsideLane(name, allowed, blocked) {
			forbidden = appendUniqueStrings(forbidden, name)
		}
	}
	return forbidden
}

func routeFailureSource(trace replay.Trace) string {
	return firstNonEmpty(traceAttribute(trace, "route_preflight_source_of_truth"), traceAttribute(trace, "result_source_kind"), traceAttribute(trace, "source_kind"))
}

func routeFailureSourceForRoute(route string) string {
	switch strings.TrimSpace(route) {
	case routing.RouteRAGSearch:
		return routing.PreflightSourceLocalDocuments
	case routing.RouteMemorySearch:
		return routing.PreflightSourceMemory
	case routing.RouteWorkspaceRead, routing.RouteFileRead, routing.RouteFileWrite:
		return routing.PreflightSourceWorkspace
	case routing.RouteInternetSearch, routing.RouteInternetFetch, routing.RouteInternetHead, routing.RouteCrawlerTask:
		return routing.PreflightSourceInternet
	case routing.RouteSchedulerCreate:
		return routing.PreflightSourceScheduler
	case routing.RouteExtensionGenerate, routing.RouteExtensionRun:
		return routing.PreflightSourceExtension
	case routing.RouteConnectorAction:
		return routing.PreflightSourceConnector
	case routing.RouteClarify:
		return routing.PreflightSourceClarify
	default:
		return routing.PreflightSourceChat
	}
}

func routeFailureCoveredByEvalOrTest(trace replay.Trace) bool {
	for _, attr := range trace.Attributes {
		key := strings.ToLower(strings.TrimSpace(attr.Key))
		value := strings.ToLower(strings.TrimSpace(attr.Value))
		if (key == "covered_by_eval" || key == "covered_by_test" || key == "qa_covered_by_test") &&
			(value == "true" || value == "yes" || value == "covered") {
			return true
		}
	}
	return false
}

func routeGovernanceStaticCoverageSources(category string) []string {
	switch strings.TrimSpace(category) {
	case RouteFailureWrongRoute,
		RouteFailureWrongSource,
		RouteFailureWrongToolLane,
		RouteFailureStaleAnswer,
		RouteFailureMissingApproval,
		RouteFailureMissingEvidence,
		RouteFailureEvidenceContract,
		RouteFailureBadClarification,
		RouteFailureProviderConfigIssue,
		RouteFailurePermissionMissing,
		RouteFailureMissingCapability:
		return []string{"go_test:internal/learning/TestClassifyRouteFailureCategories"}
	default:
		return nil
	}
}

func routeGovernanceEvalCoversCategory(category string, latestEval *QAEvalSummary) bool {
	if latestEval == nil || strings.TrimSpace(category) == "" {
		return false
	}
	for _, task := range latestEval.Tasks {
		if task.Name != "route_governance_classifier" || !evalTaskStatusPassed(task.Status) {
			continue
		}
		return routeGovernanceClassifierEvalCategory(category)
	}
	return false
}

func routeGovernanceClassifierEvalCategory(category string) bool {
	switch strings.TrimSpace(category) {
	case RouteFailureWrongRoute,
		RouteFailureWrongSource,
		RouteFailureWrongToolLane,
		RouteFailureStaleAnswer,
		RouteFailureMissingApproval,
		RouteFailureMissingEvidence,
		RouteFailureEvidenceContract,
		RouteFailureBadClarification,
		RouteFailureProviderConfigIssue,
		RouteFailurePermissionMissing,
		RouteFailureMissingCapability:
		return true
	default:
		return false
	}
}

func evalTaskStatusPassed(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "pass", "passed", "success", "succeeded", "ok":
		return true
	default:
		return false
	}
}

func appendUniqueStrings(values []string, additions ...string) []string {
	out := append([]string{}, values...)
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition == "" {
			continue
		}
		exists := false
		for _, existing := range out {
			if existing == addition {
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

func sanitizeRegressionTextList(values []string, maxChars int) []string {
	out := []string{}
	for _, value := range values {
		value = sanitizeRegressionText(value, maxChars)
		if value != "" {
			out = appendUniqueStrings(out, value)
		}
	}
	return out
}

func routeFailureSuggestedRegression(category string) string {
	switch category {
	case RouteFailureWrongRoute:
		return "Add a deterministic route-selection regression for the task frame and continuation state."
	case RouteFailureWrongSource:
		return "Add a source-of-truth regression that asserts the selected source and forbids conflicting tools."
	case RouteFailureWrongToolLane:
		return "Add a tool-lane regression that asserts only lane-approved tools can be exposed or called."
	case RouteFailureStaleAnswer:
		return "Add a freshness regression that requires configured search or a clear decline for current public facts."
	case RouteFailureMissingApproval:
		return "Add an approval-gate regression that stops before risky tools until explicit approval is present."
	case RouteFailureMissingEvidence:
		return "Add an evidence regression that declines or asks follow-up when required local evidence is empty."
	case RouteFailureEvidenceContract:
		return "Add an evidence-contract regression that allows general knowledge when local evidence is optional."
	case RouteFailureBadClarification:
		return "Add a clarification regression that asks a precise follow-up when confidence or target is ambiguous."
	case RouteFailureProviderConfigIssue:
		return "Add a provider/config regression that surfaces setup guidance for the selected route."
	case RouteFailurePermissionMissing:
		return "Add a permission regression that surfaces workspace or OS grant guidance without changing route."
	case RouteFailureMissingCapability:
		return "Add a capability regression that offers configuration or approved extension generation without bypassing policy."
	default:
		return "Add a generic route-governance regression for this replay failure."
	}
}

func routeFailureExpectedOutcome(category string) string {
	switch category {
	case RouteFailureWrongRoute:
		return "selected_route_matches_task_frame_continuation_and_source"
	case RouteFailureWrongSource:
		return "selected_source_matches_request_or_clarifies"
	case RouteFailureWrongToolLane:
		return "only_selected_lane_tools_are_exposed_or_called"
	case RouteFailureStaleAnswer:
		return "requires_fresh_approved_source_or_clear_decline"
	case RouteFailureMissingApproval:
		return "stops_before_action_until_explicit_approval"
	case RouteFailureMissingEvidence:
		return "requires_evidence_or_declines_without_claims"
	case RouteFailureEvidenceContract:
		return "uses_general_knowledge_when_optional_local_evidence_is_absent"
	case RouteFailureBadClarification:
		return "asks_precise_clarification_before_guessing"
	case RouteFailureProviderConfigIssue:
		return "surfaces_provider_configuration_guidance_for_selected_route"
	case RouteFailurePermissionMissing:
		return "surfaces_workspace_or_os_permission_guidance"
	case RouteFailureMissingCapability:
		return "offers_configuration_or_approved_capability_proposal"
	default:
		return "route_governance_failure_is_classified_and_reviewable"
	}
}

func traceAttribute(trace replay.Trace, key string) string {
	key = strings.TrimSpace(key)
	for _, attr := range trace.Attributes {
		if strings.TrimSpace(attr.Key) == key {
			return strings.TrimSpace(attr.Value)
		}
	}
	for _, attr := range trace.FinalResult.Attributes {
		if strings.TrimSpace(attr.Key) == key {
			return strings.TrimSpace(attr.Value)
		}
	}
	for _, tool := range trace.ToolsCalled {
		for _, attr := range tool.Attributes {
			if strings.TrimSpace(attr.Key) == key {
				return strings.TrimSpace(attr.Value)
			}
		}
	}
	for _, tool := range trace.ToolsConsidered {
		for _, attr := range tool.Attributes {
			if strings.TrimSpace(attr.Key) == key {
				return strings.TrimSpace(attr.Value)
			}
		}
	}
	for _, err := range trace.Errors {
		for _, attr := range err.Attributes {
			if strings.TrimSpace(attr.Key) == key {
				return strings.TrimSpace(attr.Value)
			}
		}
	}
	for _, permission := range trace.PermissionsRequested {
		for _, attr := range permission.Attributes {
			if strings.TrimSpace(attr.Key) == key {
				return strings.TrimSpace(attr.Value)
			}
		}
	}
	return ""
}
