package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"yemaka/internal/models"
	"yemaka/internal/routing"
	coretools "yemaka/internal/tools"
)

type ModelToolRequest struct {
	ToolName       string   `json:"tool_name"`
	Path           string   `json:"path,omitempty"`
	Query          string   `json:"query,omitempty"`
	Content        string   `json:"content,omitempty"`
	URL            string   `json:"url,omitempty"`
	AllowedDomains []string `json:"allowed_domains,omitempty"`
	ExtractText    bool     `json:"extract_text,omitempty"`
}

func ModelToolInstructions(plan Plan) string {
	return ModelToolInstructionsWithOptions(plan, ModelToolOptions{})
}

type ModelToolOptions struct {
	InternetTools  bool
	InternetSearch bool
}

func ModelToolInstructionsWithOptions(plan Plan, options ModelToolOptions) string {
	tools := modelToolDefinitionNames(plan, options)
	if len(tools) == 0 {
		return ""
	}
	internetGuidance := "Do not request write_file, edit_file, run_shell_safe, network access, or destructive actions."
	if options.InternetTools && options.InternetSearch {
		internetGuidance = "Use internet_fetch/internet_head only for public http(s) URLs; use internet_search only when a configured search provider is needed. Include url and allowed_domains for fetch/head, or query for search. Do not request write_file, edit_file, run_shell_safe, POST, or destructive actions."
	} else if options.InternetTools {
		internetGuidance = "Use internet_fetch/internet_head only for explicit public http(s) URLs. Internet search is unavailable until a profile search provider is configured, so do not request internet_search. Include url and allowed_domains for fetch/head. Do not request write_file, edit_file, run_shell_safe, POST, or destructive actions."
	}
	return "\n\nIf you need local observations before answering, request exactly one safe tool at a time using this shape and no extra prose:\n\n" +
		modelToolRequestExample(tools[0]) + "\n\n" +
		"Allowed tools: " + strings.Join(tools, ", ") + ". " +
		"Use read_file/list_files/file_stat/file_tree with path, search_files/rag_search/symbol_search with query, git_status/doctor_status/heartbeat_status with no input, internet_search with query only when listed, patch_preview with path and content. " +
		fmt.Sprintf("Request one tool at a time and stop after at most %d observations. ", modelToolStepLimit(plan)) +
		"When you have enough context, answer normally without the marker. " +
		internetGuidance
}

func ToolPolicyGuidance(plan Plan, options ModelToolOptions) string {
	if !containsTool(plan.ToolsNeeded, "internet_fetch") && !containsTool(plan.ToolsNeeded, "internet_head") && !containsTool(plan.ToolsNeeded, "internet_search") {
		return ""
	}
	if options.InternetTools && options.InternetSearch {
		return " Controlled internet tools are enabled for explicit public URLs, domains, and configured search providers; use the core internet tool path when an internet observation is needed. "
	}
	if options.InternetTools {
		return " Controlled internet fetch tools are enabled for explicit public URLs and domains, but internet search is unavailable until a search provider is configured. Use fetch/head for URLs; do not claim search is available. "
	}
	return " Controlled internet tools exist in Yemaka, but they are disabled or not profile-approved for this session. Do not say the capability does not exist; say internet access is disabled by policy and can be enabled by the user. "
}

func ModelToolDefinitionsWithOptions(plan Plan, options ModelToolOptions) []models.ToolDefinition {
	names := modelToolDefinitionNames(plan, options)
	definitions := make([]models.ToolDefinition, 0, len(names))
	for _, name := range names {
		if definition, ok := modelToolDefinition(name); ok {
			definitions = append(definitions, definition)
		}
	}
	return definitions
}

func modelToolRequestExample(tool string) string {
	switch normalizeToolName(tool) {
	case "rag_search":
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"rag_search\",\"query\":\"search terms\"}\n```"
	case "search_files":
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"search_files\",\"query\":\"search terms\"}\n```"
	case "list_files", "file_tree":
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"" + normalizeToolName(tool) + "\",\"path\":\".\"}\n```"
	case "file_stat", "read_file":
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"" + normalizeToolName(tool) + "\",\"path\":\"README.md\"}\n```"
	case "internet_search":
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"internet_search\",\"query\":\"search terms\"}\n```"
	case "internet_fetch", "internet_head":
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"" + normalizeToolName(tool) + "\",\"url\":\"https://example.com\",\"allowed_domains\":[\"example.com\"]}\n```"
	case "patch_preview":
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"patch_preview\",\"path\":\"README.md\",\"content\":\"proposed content\"}\n```"
	default:
		return "YEMAKA_TOOL_REQUEST\n```json\n{\"tool_name\":\"" + normalizeToolName(tool) + "\"}\n```"
	}
}

func modelToolDefinitionNames(plan Plan, options ModelToolOptions) []string {
	if plan.RiskLevel == RiskHigh {
		return nil
	}
	names := []string{}
	add := func(tool string) {
		tool = normalizeToolName(tool)
		if tool == "" || !modelToolAllowed(tool, options) {
			return
		}
		names = appendNonDuplicate(names, tool)
	}
	for _, tool := range plan.ToolsNeeded {
		add(tool)
	}
	if plan.TaskType == TaskRAG || plan.RouteUsesRAG {
		add("rag_search")
	}
	ragScoped := plan.TaskType == TaskRAG || plan.RouteUsesRAG || plan.RouteCategory == routing.RouteRAGSearch
	if (len(plan.FilesNeeded) > 0 && !ragScoped) ||
		(plan.RouteUsesWorkspace && !ragScoped) ||
		containsTool(plan.ToolsNeeded, "read_file") ||
		containsTool(plan.ToolsNeeded, "search_files") ||
		containsTool(plan.ToolsNeeded, "list_files") ||
		containsTool(plan.ToolsNeeded, "file_stat") ||
		containsTool(plan.ToolsNeeded, "file_tree") {
		add("read_file")
		add("search_files")
		add("list_files")
	}
	if len(names) > 0 {
		return names
	}
	return modelCallableTools(plan, options)
}

func modelToolDefinition(name string) (models.ToolDefinition, bool) {
	name = normalizeToolName(name)
	properties := map[string]any{}
	required := []string{}
	switch name {
	case "read_file", "file_stat":
		properties["path"] = map[string]any{"type": "string", "description": "Workspace-relative path."}
		required = append(required, "path")
	case "list_files", "file_tree":
		properties["path"] = map[string]any{"type": "string", "description": "Workspace-relative folder path. Defaults to current workspace."}
	case "search_files", "rag_search", "symbol_search", "internet_search":
		properties["query"] = map[string]any{"type": "string", "description": "Search query."}
		required = append(required, "query")
	case "patch_preview":
		properties["path"] = map[string]any{"type": "string", "description": "Workspace-relative path."}
		properties["content"] = map[string]any{"type": "string", "description": "Proposed replacement or patch-preview content."}
		required = append(required, "path", "content")
	case "internet_fetch", "internet_head":
		properties["url"] = map[string]any{"type": "string", "description": "Explicit public http(s) URL."}
		properties["allowed_domains"] = map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Allowed public domains for this request."}
		properties["extract_text"] = map[string]any{"type": "boolean", "description": "Whether to extract readable text when supported."}
		required = append(required, "url")
	case "project_map", "doctor_status", "heartbeat_status", "secret_scan", "run_tests", "git_status", "git_diff":
	default:
		return models.ToolDefinition{}, false
	}
	parameters := map[string]any{
		"type":                 "object",
		"properties":           properties,
		"additionalProperties": false,
	}
	if len(required) > 0 {
		parameters["required"] = required
	}
	return models.ToolDefinition{
		Type: "function",
		Function: models.ToolFunctionDefinition{
			Name:        name,
			Description: modelToolDefinitionDescription(name),
			Parameters:  parameters,
		},
	}, true
}

func modelToolDefinitionDescription(name string) string {
	switch name {
	case "read_file":
		return "Read one local workspace file for observation."
	case "list_files":
		return "List local workspace files in a folder."
	case "file_stat":
		return "Inspect metadata for one local workspace file."
	case "file_tree":
		return "Inspect a bounded local workspace file tree."
	case "search_files":
		return "Search local workspace files."
	case "rag_search":
		return "Search ingested local documents."
	case "project_map":
		return "Summarize the local workspace structure."
	case "symbol_search":
		return "Search local code symbols."
	case "doctor_status":
		return "Check local Yemaka setup status."
	case "heartbeat_status":
		return "Check local Yemaka health status."
	case "secret_scan":
		return "Scan local workspace files for likely secrets."
	case "run_tests":
		return "Run the detected local test command."
	case "git_status":
		return "Read local git status."
	case "git_diff":
		return "Read local git diff."
	case "patch_preview":
		return "Preview a local file patch without writing."
	case "internet_fetch":
		return "Fetch an approved public URL through Yemaka's internet broker."
	case "internet_head":
		return "Check an approved public URL with HEAD through Yemaka's internet broker."
	case "internet_search":
		return "Search the public web through a configured Yemaka search provider."
	default:
		return "Request one safe Yemaka tool observation."
	}
}

func ModelToolRequestMarkerFromNativeToolCalls(calls []models.ToolCall) string {
	request, ok := ModelToolRequestFromNativeToolCalls(calls)
	if !ok {
		return ""
	}
	payload, err := json.Marshal(request)
	if err != nil {
		return ""
	}
	return "YEMAKA_TOOL_REQUEST\n```json\n" + string(payload) + "\n```"
}

func ModelToolRequestFromNativeToolCalls(calls []models.ToolCall) (ModelToolRequest, bool) {
	for _, call := range calls {
		request := ModelToolRequestFromNativeToolCall(call)
		if request.ToolName != "" {
			return request, true
		}
	}
	return ModelToolRequest{}, false
}

func ModelToolRequestFromNativeToolCall(call models.ToolCall) ModelToolRequest {
	args := call.Function.Arguments
	return ModelToolRequest{
		ToolName:       normalizeToolName(call.Function.Name),
		Path:           nativeToolString(args, "path", "file", "folder"),
		Query:          nativeToolString(args, "query", "q", "search"),
		Content:        nativeToolString(args, "content", "text", "input"),
		URL:            nativeToolString(args, "url", "uri", "href"),
		AllowedDomains: nativeToolStringList(args, "allowed_domains", "allowedDomains", "domains"),
		ExtractText:    nativeToolBool(args, "extract_text", "extractText"),
	}
}

func ParseModelToolRequest(text string) (ModelToolRequest, bool) {
	lower := strings.ToLower(text)
	marker := "yemaka_tool_request"
	index := strings.Index(lower, marker)
	if index < 0 {
		return ModelToolRequest{}, false
	}
	after := strings.TrimSpace(text[index+len(marker):])
	if fenced, ok := firstFenceContent(after); ok {
		after = fenced
	}
	start := strings.Index(after, "{")
	end := strings.LastIndex(after, "}")
	if start < 0 || end <= start {
		return ModelToolRequest{}, false
	}
	var request ModelToolRequest
	if err := json.Unmarshal([]byte(after[start:end+1]), &request); err != nil {
		return ModelToolRequest{}, false
	}
	request.ToolName = normalizeToolName(request.ToolName)
	request.Path = strings.TrimSpace(request.Path)
	request.Query = strings.TrimSpace(request.Query)
	request.Content = strings.TrimSpace(request.Content)
	request.URL = strings.TrimSpace(request.URL)
	request.AllowedDomains = compactDomains(request.AllowedDomains)
	return request, request.ToolName != ""
}

func nativeToolString(args map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return strings.TrimSpace(typed)
			}
		case fmt.Stringer:
			if text := strings.TrimSpace(typed.String()); text != "" {
				return text
			}
		}
	}
	return ""
}

func nativeToolStringList(args map[string]any, keys ...string) []string {
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case []string:
			return compactDomains(typed)
		case []any:
			items := make([]string, 0, len(typed))
			for _, item := range typed {
				if text, ok := item.(string); ok {
					items = append(items, text)
				}
			}
			return compactDomains(items)
		case string:
			return compactDomains(strings.Split(typed, ","))
		}
	}
	return nil
}

func nativeToolBool(args map[string]any, keys ...string) bool {
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			return strings.EqualFold(strings.TrimSpace(typed), "true")
		}
	}
	return false
}

func DecisionFromModelToolRequest(plan Plan, request ModelToolRequest) ExecutionDecision {
	return DecisionFromModelToolRequestWithOptions(plan, request, "", ModelToolOptions{})
}

func DecisionFromModelToolRequestWithPolicyMode(plan Plan, request ModelToolRequest, policyMode string) ExecutionDecision {
	return DecisionFromModelToolRequestWithOptions(plan, request, policyMode, ModelToolOptions{})
}

func DecisionFromModelToolRequestWithOptions(plan Plan, request ModelToolRequest, policyMode string, options ModelToolOptions) ExecutionDecision {
	tool := normalizeToolName(request.ToolName)
	if !modelToolAllowedForPlan(plan, tool, options) {
		return ExecutionDecision{
			Status:    ExecutionBlocked,
			ToolName:  tool,
			RiskLevel: RiskMedium,
			Reason:    "model requested a tool that is not allowed in the typed low-resource tool loop",
		}
	}
	command, err := commandFromModelToolRequest(tool, request)
	if err != nil {
		return ExecutionDecision{
			Status:    ExecutionBlocked,
			ToolName:  tool,
			RiskLevel: RiskLow,
			Reason:    err.Error(),
		}
	}
	if plan.RiskLevel == RiskHigh && policyMode != "full_access" {
		return ExecutionDecision{
			Status:               ExecutionNeedsConfirmation,
			RequestID:            newPermissionRequestID(),
			ToolName:             tool,
			Command:              command,
			RiskLevel:            RiskHigh,
			RequiresConfirmation: true,
			Reason:               "model-requested action belongs to a high-risk task and needs confirmation",
		}
	}
	risk := RiskLow
	if tool == "internet_fetch" || tool == "internet_head" || tool == "internet_search" {
		risk = RiskMedium
	}
	return ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  tool,
		Command:   command,
		RiskLevel: risk,
		Reason:    "model requested one typed safe tool observation",
	}
}

func modelToolAllowedForPlan(plan Plan, tool string, options ModelToolOptions) bool {
	tool = normalizeToolName(tool)
	if !modelToolAllowed(tool, options) {
		return false
	}
	if plan.RiskLevel == RiskHigh {
		return true
	}
	if !planHasSpecificToolScope(plan) {
		return true
	}
	for _, allowed := range modelToolDefinitionNames(plan, options) {
		if normalizeToolName(allowed) == tool {
			return true
		}
	}
	return false
}

func planHasSpecificToolScope(plan Plan) bool {
	return strings.TrimSpace(plan.TaskType) != "" ||
		strings.TrimSpace(plan.RouteCategory) != "" ||
		len(plan.ToolsNeeded) > 0 ||
		len(plan.FilesNeeded) > 0 ||
		plan.RouteUsesWorkspace ||
		plan.RouteUsesRAG ||
		plan.RouteUsesInternet ||
		plan.RouteReadsFiles
}

func ToolObservationPrompt(decision ExecutionDecision, result ExecutionResult) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "TOOL OBSERVATION:\n")
	fmt.Fprintf(&builder, "tool: %s\n", decision.ToolName)
	fmt.Fprintf(&builder, "status: %s\n", result.Status)
	if strings.TrimSpace(result.SourceKind) != "" {
		fmt.Fprintf(&builder, "source_kind: %s\n", strings.TrimSpace(result.SourceKind))
	}
	if len(result.Sources) > 0 {
		fmt.Fprintf(&builder, "sources: %s\n", strings.Join(result.Sources, ", "))
	}
	if strings.TrimSpace(result.Context) != "" {
		fmt.Fprintf(&builder, "\n%s\n", strings.TrimSpace(result.Context))
	}
	fmt.Fprintf(&builder, "\nNow answer the original user request using this observation. Do not request another tool.")
	return builder.String()
}

func modelCallableTools(plan Plan, options ModelToolOptions) []string {
	if plan.RiskLevel == RiskHigh {
		return nil
	}
	base := make([]string, 0, len(coretools.ModelObservationToolNames()))
	for _, tool := range coretools.ModelObservationToolNames() {
		switch tool {
		case "internet_fetch", "internet_head":
			if options.InternetTools {
				base = append(base, tool)
			}
		case "internet_search":
			if options.InternetTools && options.InternetSearch {
				base = append(base, tool)
			}
		default:
			base = append(base, tool)
		}
	}
	return base
}

func modelToolAllowed(tool string, options ModelToolOptions) bool {
	switch normalizeToolName(tool) {
	case "internet_fetch", "internet_head":
		return options.InternetTools
	case "internet_search":
		return options.InternetTools && options.InternetSearch
	default:
		return coretools.ModelObservationToolAvailable(normalizeToolName(tool))
	}
}

func isInternetToolName(tool string) bool {
	switch normalizeToolName(tool) {
	case "internet_fetch", "internet_head", "internet_search":
		return true
	default:
		return false
	}
}

func planNeedsModelToolObservation(plan Plan) bool {
	if plan.TaskType == TaskRAG || len(plan.FilesNeeded) > 0 {
		return true
	}
	for _, tool := range []string{"read_file", "list_files", "search_files", "rag_search"} {
		if containsTool(plan.ToolsNeeded, tool) {
			return true
		}
	}
	for _, tool := range []string{"internet_fetch", "internet_head", "internet_search"} {
		if containsTool(plan.ToolsNeeded, tool) {
			return true
		}
	}
	return false
}

func modelToolStepLimit(plan Plan) int {
	limit := plan.MaxSteps
	if limit <= 0 {
		limit = 3
	}
	if limit > 4 {
		limit = 4
	}
	if limit < 1 {
		limit = 1
	}
	return limit
}

func commandFromModelToolRequest(tool string, request ModelToolRequest) ([]string, error) {
	switch tool {
	case "read_file":
		if request.Path == "" {
			return nil, fmt.Errorf("read_file requires path")
		}
		return []string{"read_file", request.Path}, nil
	case "list_files":
		path := strings.TrimSpace(request.Path)
		if path == "" {
			path = "."
		}
		return []string{"list_files", path}, nil
	case "file_stat":
		if request.Path == "" {
			return nil, fmt.Errorf("file_stat requires path")
		}
		return []string{"file_stat", request.Path}, nil
	case "file_tree":
		path := strings.TrimSpace(request.Path)
		if path == "" {
			path = "."
		}
		return []string{"file_tree", path}, nil
	case "search_files":
		query := fallbackToolQuery(request)
		if query == "" {
			return nil, fmt.Errorf("search_files requires query")
		}
		return []string{"search_files", query}, nil
	case "rag_search":
		query := fallbackToolQuery(request)
		if query == "" {
			return nil, fmt.Errorf("rag_search requires query")
		}
		return []string{"rag_search", query}, nil
	case "project_map":
		return []string{"project_map"}, nil
	case "symbol_search":
		query := fallbackToolQuery(request)
		if query == "" {
			return nil, fmt.Errorf("symbol_search requires query")
		}
		return []string{"symbol_search", query}, nil
	case "doctor_status":
		return []string{"doctor_status"}, nil
	case "heartbeat_status":
		return []string{"heartbeat_status"}, nil
	case "secret_scan":
		return []string{"secret_scan"}, nil
	case "run_tests":
		return []string{"detect"}, nil
	case "git_status":
		return []string{"git", "status", "--short"}, nil
	case "git_diff":
		return []string{"git", "diff"}, nil
	case "patch_preview":
		if request.Path == "" || strings.TrimSpace(request.Content) == "" {
			return nil, fmt.Errorf("patch_preview requires path and content")
		}
		return []string{"patch_preview", request.Path, request.Content}, nil
	case "internet_fetch", "internet_head":
		target := fallbackToolURL(request)
		if target == "" {
			return nil, fmt.Errorf("%s requires url", tool)
		}
		domains := request.AllowedDomains
		if len(domains) == 0 {
			host, err := hostFromURL(target)
			if err != nil {
				return nil, err
			}
			domains = []string{host}
		}
		return append([]string{tool, target}, domains...), nil
	case "internet_search":
		query := fallbackToolQuery(request)
		if query == "" {
			return nil, fmt.Errorf("internet_search requires query")
		}
		return []string{"internet_search", query}, nil
	default:
		return nil, fmt.Errorf("unsupported model tool request: %s", tool)
	}
}

func fallbackToolQuery(request ModelToolRequest) string {
	if request.Query != "" {
		return request.Query
	}
	return request.Content
}

func fallbackToolURL(request ModelToolRequest) string {
	for _, candidate := range []string{request.URL, request.Path, request.Query, request.Content} {
		if target, ok := normalizeInternetTarget(candidate); ok {
			return target
		}
	}
	return ""
}

func hostFromURL(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("parse url: %w", err)
	}
	host := strings.TrimSpace(parsed.Hostname())
	if parsed.Scheme == "" || host == "" {
		return "", fmt.Errorf("url must include scheme and host")
	}
	return host, nil
}

func compactDomains(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func normalizeToolName(tool string) string {
	return strings.ToLower(strings.TrimSpace(tool))
}
