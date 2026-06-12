package learning

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"yemaka/internal/replay"
)

const RegressionCaseSchema = "yemaka.regression_case.v1"

const (
	FailureKindRouting             = "routing_failure"
	FailureKindToolCall            = "tool_call_failure"
	FailureKindPermission          = "permission_failure"
	FailureKindInternetSearch      = "internet_search_failure"
	FailureKindExtensionGeneration = "extension_generation_failure"
	FailureKindDocumentIngestion   = "document_ingestion_failure"
	FailureKindRAGRetrieval        = "rag_retrieval_failure"
	FailureKindUIFlow              = "ui_flow_failure"
	FailureKindUnknown             = "unknown_failure"
)

var (
	regressionAuthorizationPattern = regexp.MustCompile(`(?i)\bauthorization\s*[:=]\s*bearer\s+[^,\s;"'}]+`)
	regressionBearerPattern        = regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9._~+/=-]{8,}`)
	regressionSecretTokenPattern   = regexp.MustCompile(`\b(gh[pousr]_[A-Za-z0-9_]{12,}|AKIA[0-9A-Z]{12,})\b`)
)

type RegressionTestCase struct {
	Schema        string             `json:"schema" yaml:"schema"`
	Name          string             `json:"name" yaml:"name"`
	Kind          string             `json:"kind" yaml:"kind"`
	SourceTraceID string             `json:"sourceTraceId,omitempty" yaml:"source_trace_id,omitempty"`
	Prompt        string             `json:"prompt" yaml:"prompt"`
	Failure       RegressionFailure  `json:"failure" yaml:"failure"`
	Expected      RegressionExpected `json:"expected" yaml:"expected"`
	Reproduction  []string           `json:"reproduction" yaml:"reproduction"`
	Tags          []string           `json:"tags" yaml:"tags"`
}

type RegressionFailure struct {
	Stage   string `json:"stage,omitempty" yaml:"stage,omitempty"`
	Code    string `json:"code,omitempty" yaml:"code,omitempty"`
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}

type RegressionExpected struct {
	Route                    string   `json:"route,omitempty" yaml:"route,omitempty"`
	RiskLevel                string   `json:"riskLevel,omitempty" yaml:"risk_level,omitempty"`
	ShouldUseTool            bool     `json:"shouldUseTool" yaml:"should_use_tool"`
	ShouldUseInternet        bool     `json:"shouldUseInternet" yaml:"should_use_internet"`
	ShouldReadFile           bool     `json:"shouldReadFile" yaml:"should_read_file"`
	ShouldWriteFiles         bool     `json:"shouldWriteFiles" yaml:"should_write_files"`
	ShouldAskApproval        bool     `json:"shouldAskApproval" yaml:"should_ask_approval"`
	ShouldAskClarification   bool     `json:"shouldAskClarification" yaml:"should_ask_clarification"`
	ShouldGenerateExtension  bool     `json:"shouldGenerateExtension" yaml:"should_generate_extension"`
	ShouldCreateSchedulerJob bool     `json:"shouldCreateSchedulerJob" yaml:"should_create_scheduler_job"`
	RequiredTools            []string `json:"requiredTools,omitempty" yaml:"required_tools,omitempty"`
	ForbiddenTools           []string `json:"forbiddenTools,omitempty" yaml:"forbidden_tools,omitempty"`
}

func GenerateRegressionTestCase(trace replay.Trace) (RegressionTestCase, bool) {
	cases := GenerateRegressionTestCases(trace)
	if len(cases) == 0 {
		return RegressionTestCase{}, false
	}
	return cases[0], true
}

func GenerateRegressionTestCases(trace replay.Trace) []RegressionTestCase {
	trace = trace.Normalize()
	if strings.TrimSpace(trace.UserRequest) == "" {
		return nil
	}

	detectors := []func(replay.Trace) (RegressionTestCase, bool){
		routingRegressionCase,
		permissionRegressionCase,
		internetRegressionCase,
		extensionRegressionCase,
		documentIngestionRegressionCase,
		ragRetrievalRegressionCase,
		uiFlowRegressionCase,
		toolCallRegressionCase,
	}

	var cases []RegressionTestCase
	seen := map[string]bool{}
	for _, detect := range detectors {
		testCase, ok := detect(trace)
		if !ok {
			continue
		}
		testCase = normalizeRegressionCase(testCase)
		if testCase.Kind == "" || seen[testCase.Kind] {
			continue
		}
		seen[testCase.Kind] = true
		cases = append(cases, testCase)
	}
	if len(cases) == 0 && trace.HasFailures() {
		if testCase, ok := unknownRegressionCase(trace); ok {
			cases = append(cases, normalizeRegressionCase(testCase))
		}
	}
	return cases
}

func FormatRegressionTestCaseYAMLish(testCase RegressionTestCase) string {
	testCase = normalizeRegressionCase(testCase)
	var out strings.Builder
	fmt.Fprintf(&out, "schema: %s\n", yamlString(testCase.Schema))
	fmt.Fprintf(&out, "name: %s\n", yamlString(testCase.Name))
	fmt.Fprintf(&out, "kind: %s\n", yamlString(testCase.Kind))
	if testCase.SourceTraceID != "" {
		fmt.Fprintf(&out, "source_trace_id: %s\n", yamlString(testCase.SourceTraceID))
	}
	fmt.Fprintf(&out, "prompt: %s\n", yamlString(testCase.Prompt))
	out.WriteString("failure:\n")
	fmt.Fprintf(&out, "  stage: %s\n", yamlString(testCase.Failure.Stage))
	fmt.Fprintf(&out, "  code: %s\n", yamlString(testCase.Failure.Code))
	fmt.Fprintf(&out, "  subject: %s\n", yamlString(testCase.Failure.Subject))
	fmt.Fprintf(&out, "  message: %s\n", yamlString(testCase.Failure.Message))
	out.WriteString("expected:\n")
	fmt.Fprintf(&out, "  route: %s\n", yamlString(testCase.Expected.Route))
	fmt.Fprintf(&out, "  risk_level: %s\n", yamlString(testCase.Expected.RiskLevel))
	fmt.Fprintf(&out, "  should_use_tool: %t\n", testCase.Expected.ShouldUseTool)
	fmt.Fprintf(&out, "  should_use_internet: %t\n", testCase.Expected.ShouldUseInternet)
	fmt.Fprintf(&out, "  should_read_file: %t\n", testCase.Expected.ShouldReadFile)
	fmt.Fprintf(&out, "  should_write_files: %t\n", testCase.Expected.ShouldWriteFiles)
	fmt.Fprintf(&out, "  should_ask_approval: %t\n", testCase.Expected.ShouldAskApproval)
	fmt.Fprintf(&out, "  should_ask_clarification: %t\n", testCase.Expected.ShouldAskClarification)
	fmt.Fprintf(&out, "  should_generate_extension: %t\n", testCase.Expected.ShouldGenerateExtension)
	fmt.Fprintf(&out, "  should_create_scheduler_job: %t\n", testCase.Expected.ShouldCreateSchedulerJob)
	writeYAMLStringList(&out, "  ", "required_tools", testCase.Expected.RequiredTools)
	writeYAMLStringList(&out, "  ", "forbidden_tools", testCase.Expected.ForbiddenTools)
	writeYAMLStringList(&out, "", "reproduction", testCase.Reproduction)
	writeYAMLStringList(&out, "", "tags", testCase.Tags)
	return strings.TrimRight(out.String(), "\n")
}

func routingRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	failure, hasExplicit := firstTraceError(trace, "routing", "route")
	falsePositive := looksExplanationRequest(trace.UserRequest) && routeLooksAction(trace)
	if !hasExplicit && !falsePositive {
		return RegressionTestCase{}, false
	}
	if !hasExplicit {
		failure = replay.TraceError{
			Stage:         "routing",
			Code:          "false_positive_action",
			Subject:       trace.Route.Category,
			Message:       "explanation request routed to an action path",
			ExpectedRoute: routeChatExplanation,
		}
	}
	expectedRoute := firstNonEmpty(failure.ExpectedRoute, expectedRouteForPrompt(trace.UserRequest), trace.Route.Category, routeClarify)
	expectedRisk := firstNonEmpty(failure.ExpectedRiskLevel, riskLow)
	return buildRegressionCase(trace, FailureKindRouting, failure, RegressionExpected{
		Route:                    expectedRoute,
		RiskLevel:                expectedRisk,
		ShouldUseTool:            false,
		ShouldUseInternet:        false,
		ShouldReadFile:           false,
		ShouldWriteFiles:         false,
		ShouldAskApproval:        false,
		ShouldAskClarification:   expectedRoute == routeClarify,
		ShouldGenerateExtension:  false,
		ShouldCreateSchedulerJob: false,
		ForbiddenTools:           traceTools(trace),
	}, []string{"routing"}), true
}

func permissionRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	for _, permission := range trace.PermissionsRequested {
		if !replay.StatusFailed(permission.Status) {
			continue
		}
		failure := replay.TraceError{
			Stage:             "permission",
			Code:              "permission_" + normalizedToken(permission.Status, "required"),
			Subject:           permission.ToolName,
			Message:           firstNonEmpty(permission.Reason, permission.PolicyExplanation, "permission was not approved"),
			ExpectedRiskLevel: firstNonEmpty(permission.RiskLevel, trace.Route.RiskLevel, riskMedium),
		}
		return buildRegressionCase(trace, FailureKindPermission, failure, RegressionExpected{
			Route:                   firstNonEmpty(trace.Route.Category, routePermissionRequired),
			RiskLevel:               firstNonEmpty(failure.ExpectedRiskLevel, riskMedium),
			ShouldUseTool:           true,
			ShouldAskApproval:       true,
			RequiredTools:           requiredTool(permission.ToolName),
			ShouldUseInternet:       isInternetName(permission.ToolName),
			ShouldWriteFiles:        strings.Contains(strings.ToLower(permission.ToolName), "write"),
			ShouldGenerateExtension: strings.Contains(strings.ToLower(permission.ToolName), "extension"),
		}, []string{"permission"}), true
	}
	if failure, ok := firstTraceError(trace, "permission", "policy", "approval"); ok {
		return buildRegressionCase(trace, FailureKindPermission, failure, RegressionExpected{
			Route:             firstNonEmpty(trace.Route.Category, routePermissionRequired),
			RiskLevel:         firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskMedium),
			ShouldUseTool:     true,
			ShouldAskApproval: true,
			RequiredTools:     requiredTool(failure.Subject),
		}, []string{"permission"}), true
	}
	return RegressionTestCase{}, false
}

func internetRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	if failure, ok := firstSpecificFailure(trace, isInternetName, "internet", "searxng", "network", "provider", "private ip"); ok {
		tool := firstNonEmpty(failure.Subject, firstFailedToolName(trace, isInternetName), "internet_search")
		return buildRegressionCase(trace, FailureKindInternetSearch, failure, RegressionExpected{
			Route:             firstNonEmpty(expectedInternetRoute(trace), trace.Route.Category, routeInternetSearch),
			RiskLevel:         firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskMedium),
			ShouldUseTool:     true,
			ShouldUseInternet: true,
			ShouldAskApproval: true,
			RequiredTools:     requiredTool(tool),
		}, []string{"internet"}), true
	}
	return RegressionTestCase{}, false
}

func extensionRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	if failure, ok := firstSpecificFailure(trace, isExtensionName, "extension", "manifest", "generated capability"); ok {
		tool := firstNonEmpty(failure.Subject, firstFailedToolName(trace, isExtensionName), "extension_generate")
		return buildRegressionCase(trace, FailureKindExtensionGeneration, failure, RegressionExpected{
			Route:                   firstNonEmpty(trace.Route.Category, routeExtensionGenerate),
			RiskLevel:               firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskMedium),
			ShouldUseTool:           true,
			ShouldAskApproval:       true,
			ShouldGenerateExtension: true,
			RequiredTools:           requiredTool(tool),
		}, []string{"extension"}), true
	}
	return RegressionTestCase{}, false
}

func documentIngestionRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	if failure, ok := firstSpecificFailure(trace, isDocumentName, "document", "ingest", "ingestion", "pdf", "docx", "extract"); ok {
		tool := firstNonEmpty(failure.Subject, firstFailedToolName(trace, isDocumentName), "document_ingest")
		return buildRegressionCase(trace, FailureKindDocumentIngestion, failure, RegressionExpected{
			Route:          firstNonEmpty(trace.Route.Category, routeRAGSearch),
			RiskLevel:      firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskLow),
			ShouldUseTool:  true,
			ShouldReadFile: true,
			RequiredTools:  requiredTool(tool),
		}, []string{"document"}), true
	}
	return RegressionTestCase{}, false
}

func ragRetrievalRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	if failure, ok := firstSpecificFailure(trace, isRAGName, "rag", "retrieval", "retriever"); ok {
		tool := firstNonEmpty(failure.Subject, firstFailedToolName(trace, isRAGName), "rag_search")
		return buildRegressionCase(trace, FailureKindRAGRetrieval, failure, RegressionExpected{
			Route:          firstNonEmpty(trace.Route.Category, routeRAGSearch),
			RiskLevel:      firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskLow),
			ShouldUseTool:  true,
			ShouldReadFile: true,
			RequiredTools:  requiredTool(tool),
		}, []string{"rag"}), true
	}
	return RegressionTestCase{}, false
}

func uiFlowRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	if failure, ok := firstTraceError(trace, "ui", "frontend", "wails", "svelte", "render", "stream"); ok {
		return buildRegressionCase(trace, FailureKindUIFlow, failure, RegressionExpected{
			Route:     firstNonEmpty(trace.Route.Category, routeChatExplanation),
			RiskLevel: firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskLow),
		}, []string{"ui"}), true
	}
	return RegressionTestCase{}, false
}

func toolCallRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	for _, tool := range trace.ToolsCalled {
		if !toolFailed(tool) || isSpecificToolName(tool.Name) {
			continue
		}
		failure := replay.TraceError{
			Stage:             "tool",
			Code:              "tool_" + normalizedToken(tool.Status, "failed"),
			Subject:           tool.Name,
			Message:           firstNonEmpty(tool.Error, tool.OutputSummary, "tool call failed"),
			ExpectedRiskLevel: trace.Route.RiskLevel,
		}
		return buildRegressionCase(trace, FailureKindToolCall, failure, RegressionExpected{
			Route:         firstNonEmpty(trace.Route.Category, routeChatExplanation),
			RiskLevel:     firstNonEmpty(failure.ExpectedRiskLevel, riskLow),
			ShouldUseTool: true,
			RequiredTools: requiredTool(tool.Name),
		}, []string{"tool"}), true
	}
	if failure, ok := firstTraceError(trace, "tool"); ok && !isSpecificToolName(failure.Subject) {
		return buildRegressionCase(trace, FailureKindToolCall, failure, RegressionExpected{
			Route:         firstNonEmpty(trace.Route.Category, routeChatExplanation),
			RiskLevel:     firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskLow),
			ShouldUseTool: true,
			RequiredTools: requiredTool(failure.Subject),
		}, []string{"tool"}), true
	}
	return RegressionTestCase{}, false
}

func unknownRegressionCase(trace replay.Trace) (RegressionTestCase, bool) {
	failure, ok := trace.PrimaryFailure()
	if !ok {
		return RegressionTestCase{}, false
	}
	return buildRegressionCase(trace, FailureKindUnknown, failure, RegressionExpected{
		Route:     firstNonEmpty(trace.Route.Category, routeClarify),
		RiskLevel: firstNonEmpty(failure.ExpectedRiskLevel, trace.Route.RiskLevel, riskLow),
	}, []string{"unknown"}), true
}

func buildRegressionCase(trace replay.Trace, kind string, failure replay.TraceError, expected RegressionExpected, tags []string) RegressionTestCase {
	prompt := sanitizeRegressionText(trace.UserRequest, 240)
	return RegressionTestCase{
		Schema:        RegressionCaseSchema,
		Name:          regressionCaseName(kind, prompt, trace.ID),
		Kind:          kind,
		SourceTraceID: sanitizeRegressionText(trace.ID, 120),
		Prompt:        prompt,
		Failure: RegressionFailure{
			Stage:   sanitizeRegressionText(failure.Stage, 80),
			Code:    sanitizeRegressionText(failure.Code, 100),
			Subject: sanitizeRegressionText(failure.Subject, 120),
			Message: sanitizeRegressionText(failure.Message, 240),
		},
		Expected: RegressionExpected{
			Route:                    sanitizeRegressionText(expected.Route, 120),
			RiskLevel:                sanitizeRegressionText(expected.RiskLevel, 40),
			ShouldUseTool:            expected.ShouldUseTool,
			ShouldUseInternet:        expected.ShouldUseInternet,
			ShouldReadFile:           expected.ShouldReadFile,
			ShouldWriteFiles:         expected.ShouldWriteFiles,
			ShouldAskApproval:        expected.ShouldAskApproval,
			ShouldAskClarification:   expected.ShouldAskClarification,
			ShouldGenerateExtension:  expected.ShouldGenerateExtension,
			ShouldCreateSchedulerJob: expected.ShouldCreateSchedulerJob,
			RequiredTools:            sanitizeStringList(expected.RequiredTools),
			ForbiddenTools:           sanitizeStringList(expected.ForbiddenTools),
		},
		Reproduction: reproductionSteps(kind),
		Tags:         uniqueStringsStable(append([]string{"replay", "generated_regression"}, tags...)),
	}
}

func normalizeRegressionCase(testCase RegressionTestCase) RegressionTestCase {
	testCase.Schema = firstNonEmpty(strings.TrimSpace(testCase.Schema), RegressionCaseSchema)
	testCase.Kind = sanitizeRegressionText(testCase.Kind, 80)
	testCase.SourceTraceID = sanitizeRegressionText(testCase.SourceTraceID, 120)
	testCase.Prompt = sanitizeRegressionText(testCase.Prompt, 240)
	if testCase.Name == "" {
		testCase.Name = regressionCaseName(testCase.Kind, testCase.Prompt, testCase.SourceTraceID)
	} else {
		testCase.Name = sanitizeRegressionName(testCase.Name)
	}
	testCase.Failure.Stage = sanitizeRegressionText(testCase.Failure.Stage, 80)
	testCase.Failure.Code = sanitizeRegressionText(testCase.Failure.Code, 100)
	testCase.Failure.Subject = sanitizeRegressionText(testCase.Failure.Subject, 120)
	testCase.Failure.Message = sanitizeRegressionText(testCase.Failure.Message, 240)
	testCase.Expected.Route = sanitizeRegressionText(testCase.Expected.Route, 120)
	testCase.Expected.RiskLevel = sanitizeRegressionText(testCase.Expected.RiskLevel, 40)
	testCase.Expected.RequiredTools = sanitizeStringList(testCase.Expected.RequiredTools)
	testCase.Expected.ForbiddenTools = sanitizeStringList(testCase.Expected.ForbiddenTools)
	testCase.Reproduction = sanitizeStringList(testCase.Reproduction)
	testCase.Tags = uniqueStringsStable(sanitizeStringList(testCase.Tags))
	if len(testCase.Reproduction) == 0 {
		testCase.Reproduction = reproductionSteps(testCase.Kind)
	}
	if len(testCase.Tags) == 0 {
		testCase.Tags = []string{"replay", "generated_regression"}
	}
	return testCase
}

func firstTraceError(trace replay.Trace, terms ...string) (replay.TraceError, bool) {
	for _, err := range trace.Errors {
		if textMatchesTerms(traceErrorText(err), terms...) {
			return err, true
		}
	}
	return replay.TraceError{}, false
}

func firstSpecificFailure(trace replay.Trace, nameMatch func(string) bool, terms ...string) (replay.TraceError, bool) {
	for _, err := range trace.Errors {
		if nameMatch(err.Subject) || textMatchesTerms(traceErrorText(err), terms...) {
			return err, true
		}
	}
	for _, tool := range trace.ToolsCalled {
		if !toolFailed(tool) || !nameMatch(tool.Name) {
			continue
		}
		return replay.TraceError{
			Stage:   "tool",
			Code:    "tool_" + normalizedToken(tool.Status, "failed"),
			Subject: tool.Name,
			Message: firstNonEmpty(tool.Error, tool.OutputSummary, "tool call failed"),
		}, true
	}
	return replay.TraceError{}, false
}

func traceErrorText(err replay.TraceError) string {
	return strings.ToLower(strings.Join([]string{err.Stage, err.Code, err.Subject, err.Message}, " "))
}

func textMatchesTerms(text string, terms ...string) bool {
	text = strings.ToLower(text)
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term != "" && strings.Contains(text, term) {
			return true
		}
	}
	return false
}

func routeLooksAction(trace replay.Trace) bool {
	if trace.Route.ShouldUseTool ||
		trace.Route.ShouldUseInternet ||
		trace.Route.ShouldWriteFiles ||
		trace.Route.ShouldGenerateExtension ||
		trace.Route.ShouldCreateSchedulerJob {
		return true
	}
	switch trace.Route.Category {
	case routeSchedulerCreate, routeFileWrite, routeShellTool, routeConnectorAction, routeExtensionGenerate, routeExtensionRun, routeInternetSearch, routeInternetFetch, routeInternetHead, routeCrawlerTask:
		return true
	default:
		return len(trace.ToolsCalled) > 0
	}
}

func expectedRouteForPrompt(prompt string) string {
	if looksExplanationRequest(prompt) {
		return routeChatExplanation
	}
	return ""
}

func looksExplanationRequest(prompt string) bool {
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	prompt = strings.Join(strings.Fields(prompt), " ")
	for _, prefix := range []string{
		"what is ", "what are ", "who is ", "who are ", "explain ", "how does ", "how do i ", "how can i ", "tell me about ", "describe ", "why does ", "why is ",
	} {
		if strings.HasPrefix(prompt, prefix) {
			return true
		}
	}
	return false
}

func expectedInternetRoute(trace replay.Trace) string {
	switch trace.Route.Category {
	case routeInternetSearch, routeInternetFetch, routeInternetHead:
		return trace.Route.Category
	default:
		return ""
	}
}

func traceTools(trace replay.Trace) []string {
	var tools []string
	for _, tool := range trace.Plan.ToolsNeeded {
		tools = append(tools, tool)
	}
	for _, tool := range trace.ToolsCalled {
		tools = append(tools, tool.Name)
	}
	return uniqueSortedStrings(tools)
}

func requiredTool(tool string) []string {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return nil
	}
	return []string{tool}
}

func firstFailedToolName(trace replay.Trace, match func(string) bool) string {
	for _, tool := range trace.ToolsCalled {
		if toolFailed(tool) && match(tool.Name) {
			return tool.Name
		}
	}
	return ""
}

func toolFailed(tool replay.ToolCall) bool {
	return replay.StatusFailed(tool.Status) || strings.TrimSpace(tool.Error) != ""
}

func isSpecificToolName(name string) bool {
	return isInternetName(name) || isExtensionName(name) || isDocumentName(name) || isRAGName(name) || isUIName(name)
}

func isInternetName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(name, "internet_") ||
		strings.Contains(name, "searxng") ||
		strings.Contains(name, "network") ||
		strings.Contains(name, "fetch_url")
}

func isExtensionName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "extension")
}

func isDocumentName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "document") ||
		strings.Contains(name, "ingest") ||
		strings.Contains(name, "extract") ||
		strings.Contains(name, "pdf") ||
		strings.Contains(name, "docx")
}

func isRAGName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "rag") || strings.Contains(name, "retriev")
}

func isUIName(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return strings.Contains(name, "ui") ||
		strings.Contains(name, "frontend") ||
		strings.Contains(name, "wails") ||
		strings.Contains(name, "svelte")
}

func reproductionSteps(kind string) []string {
	switch kind {
	case FailureKindRouting:
		return []string{
			"Classify the prompt with deterministic routing.",
			"Assert route, risk, and action booleans before any tool execution.",
		}
	case FailureKindPermission:
		return []string{
			"Classify the prompt and build the permission request.",
			"Assert the action requires approval and does not run before approval.",
		}
	case FailureKindInternetSearch:
		return []string{
			"Classify the prompt with internet disabled by default.",
			"Assert internet use is policy-gated and the failure message is clear.",
		}
	case FailureKindExtensionGeneration:
		return []string{
			"Build the generated extension proposal.",
			"Assert manifest, permissions, and approval gates are required before registration.",
		}
	case FailureKindDocumentIngestion:
		return []string{
			"Run document extraction with a local fixture.",
			"Assert extraction failure is reported without enabling external services.",
		}
	case FailureKindRAGRetrieval:
		return []string{
			"Run local RAG retrieval with bounded context.",
			"Assert retrieval results and fallback behavior are deterministic.",
		}
	case FailureKindUIFlow:
		return []string{
			"Replay the UI event sequence with a local fixture.",
			"Assert visible state and error handling without network access.",
		}
	default:
		return []string{
			"Replay the captured trace using local fixtures only.",
			"Assert the expected behavior before saving the regression.",
		}
	}
}

func sanitizeRegressionText(value string, maxChars int) string {
	value = strings.Join(strings.Fields(value), " ")
	value, _ = SanitizeText(value)
	value = redactRegressionSecretText(value)
	return compact(value, maxChars)
}

func redactRegressionSecretText(value string) string {
	value = regressionAuthorizationPattern.ReplaceAllStringFunc(value, redactSecretAssignment)
	value = regressionBearerPattern.ReplaceAllString(value, "Bearer [REDACTED]")
	value = regressionSecretTokenPattern.ReplaceAllString(value, "[REDACTED]")
	return value
}

func sanitizeStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = sanitizeRegressionText(value, 180)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return uniqueStringsStable(out)
}

func uniqueStringsStable(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func uniqueSortedStrings(values []string) []string {
	values = uniqueStringsStable(values)
	sort.Strings(values)
	return values
}

func regressionCaseName(kind string, prompt string, fallback string) string {
	words := safeRegressionWords(prompt)
	if len(words) == 0 {
		words = safeRegressionWords(fallback)
	}
	if len(words) == 0 {
		words = []string{"trace"}
	}
	if len(words) > 5 {
		words = words[:5]
	}
	return sanitizeRegressionName(kind + "_" + strings.Join(words, "_"))
}

func sanitizeRegressionName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var out strings.Builder
	lastUnderscore := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			out.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			out.WriteRune('_')
			lastUnderscore = true
		}
	}
	clean := strings.Trim(out.String(), "_")
	if clean == "" {
		return "generated_regression"
	}
	if len(clean) > 96 {
		clean = strings.Trim(clean[:96], "_")
	}
	return clean
}

func safeRegressionWords(input string) []string {
	input = strings.ToLower(input)
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "can": true, "for": true, "from": true, "how": true, "into": true, "is": true, "me": true, "my": true, "of": true, "on": true, "or": true, "the": true, "this": true, "to": true, "what": true, "with": true,
	}
	var words []string
	var current strings.Builder
	flush := func() {
		word := current.String()
		current.Reset()
		if len(word) < 3 || stop[word] {
			return
		}
		words = append(words, word)
	}
	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return uniqueStringsStable(words)
}

func writeYAMLStringList(out *strings.Builder, indent string, name string, values []string) {
	if len(values) == 0 {
		fmt.Fprintf(out, "%s%s: []\n", indent, name)
		return
	}
	fmt.Fprintf(out, "%s%s:\n", indent, name)
	for _, value := range values {
		fmt.Fprintf(out, "%s  - %s\n", indent, yamlString(value))
	}
}

func yamlString(value string) string {
	return strconv.Quote(value)
}

func normalizedToken(value string, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = sanitizeRegressionName(value)
	if value == "" {
		return fallback
	}
	return value
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

const (
	routeChatExplanation    = "chat_explanation"
	routeRAGSearch          = "rag_search"
	routeInternetSearch     = "internet_search"
	routeInternetFetch      = "internet_fetch"
	routeInternetHead       = "internet_head"
	routeCrawlerTask        = "crawler_task"
	routeFileWrite          = "file_write"
	routeShellTool          = "shell_tool"
	routeExtensionRun       = "extension_run"
	routeExtensionGenerate  = "extension_generate"
	routeSchedulerCreate    = "scheduler_create"
	routeConnectorAction    = "connector_action"
	routePermissionRequired = "permission_required"
	routeClarify            = "clarify"
	riskLow                 = "low"
	riskMedium              = "medium"
)
