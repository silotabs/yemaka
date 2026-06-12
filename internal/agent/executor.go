package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"yemaka/internal/routing"
	"yemaka/internal/safety"
)

const (
	ExecutionNotRequired       = "not_required"
	ExecutionAlreadyProvided   = "already_provided"
	ExecutionReady             = "ready"
	ExecutionNeedsConfirmation = "needs_confirmation"
	ExecutionBlocked           = "blocked"
)

type ExecutionDecision struct {
	Status               string   `json:"status"`
	RequestID            string   `json:"request_id,omitempty"`
	ToolName             string   `json:"tool_name"`
	Command              []string `json:"command,omitempty"`
	RiskLevel            string   `json:"risk_level"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	Reason               string   `json:"reason"`
	PolicyLevel          int      `json:"policy_level,omitempty"`
	PolicyExplanation    string   `json:"policy_explanation,omitempty"`
}

type ExecutionResult struct {
	Context    string
	Sources    []string
	SourceKind string
	Status     string
}

type ToolExecutor func(context.Context, ExecutionDecision) (ExecutionResult, error)

type PermissionRequest struct {
	RequestID            string   `json:"request_id"`
	ToolName             string   `json:"tool_name"`
	Command              []string `json:"command,omitempty"`
	RiskLevel            string   `json:"risk_level"`
	Reason               string   `json:"reason"`
	RequiresConfirmation bool     `json:"requires_confirmation"`
	WorkspaceOnly        bool     `json:"workspace_only"`
	DiffPreview          bool     `json:"diff_preview"`
	SnapshotBeforeWrite  bool     `json:"snapshot_before_write"`
	RollbackSupported    bool     `json:"rollback_supported"`
	Destructive          bool     `json:"destructive"`
	NextStep             string   `json:"next_step"`
	PolicyLevel          int      `json:"policy_level,omitempty"`
	PolicyExplanation    string   `json:"policy_explanation,omitempty"`
}

type EditProposal struct {
	RequestID           string `json:"request_id"`
	Path                string `json:"path"`
	Content             string `json:"content,omitempty"`
	ContentSource       string `json:"content_source,omitempty"`
	Status              string `json:"status"`
	Reason              string `json:"reason"`
	NeedsContent        bool   `json:"needs_content"`
	DiffPreview         bool   `json:"diff_preview"`
	SnapshotBeforeWrite bool   `json:"snapshot_before_write"`
	RollbackSupported   bool   `json:"rollback_supported"`
}

func DecideExecution(plan Plan, input PlanInput) ExecutionDecision {
	return DecideExecutionWithPolicyMode(plan, input, safety.PolicyModeSafe)
}

func DecideExecutionWithPolicyMode(plan Plan, input PlanInput, policyMode string) ExecutionDecision {
	if strings.EqualFold(input.SourceKind, "tool") {
		return ExecutionDecision{
			Status:    ExecutionAlreadyProvided,
			RiskLevel: plan.RiskLevel,
			Reason:    "tool output is already present in retrieved context",
		}
	}
	if plan.NeedsClarification {
		return ExecutionDecision{
			Status:    ExecutionNotRequired,
			RiskLevel: plan.RiskLevel,
			Reason:    "clarification is required before any tool or approval flow can run",
		}
	}
	if plan.RiskLevel == RiskHigh && !safety.IsFullAccessMode(policyMode) {
		policy := safety.EvaluatePolicy(safety.PolicyRequest{
			Domain:     safety.DomainGeneral,
			Action:     safety.ActionRun,
			Level:      safety.LevelConfirm,
			PolicyMode: policyMode,
			Actor:      "agent_executor",
			Resource:   firstTool(plan.ToolsNeeded),
			Mutating:   true,
		})
		return ExecutionDecision{
			Status:               ExecutionNeedsConfirmation,
			RequestID:            newPermissionRequestID(),
			ToolName:             firstTool(plan.ToolsNeeded),
			RiskLevel:            plan.RiskLevel,
			RequiresConfirmation: true,
			Reason:               "high-risk action requires explicit confirmation before execution",
			PolicyLevel:          int(policy.Level),
			PolicyExplanation:    policy.Explanation,
		}
	}
	if containsTool(plan.ToolsNeeded, "edit_file") && !safety.IsFullAccessMode(policyMode) {
		policy := safety.EvaluatePolicy(safety.PolicyRequest{
			Domain:       safety.DomainFilesystem,
			Action:       safety.ActionWrite,
			Level:        safety.LevelConfirm,
			PolicyMode:   policyMode,
			Actor:        "agent_executor",
			Resource:     "edit_file",
			Mutating:     true,
			FileWrite:    true,
			TaskApproved: false,
		})
		return ExecutionDecision{
			Status:               ExecutionNeedsConfirmation,
			RequestID:            newPermissionRequestID(),
			ToolName:             "edit_file",
			Command:              editFileConfirmationCommand(plan, input),
			RiskLevel:            RiskMedium,
			RequiresConfirmation: true,
			Reason:               "file edits require confirmation and snapshot flow",
			PolicyLevel:          int(policy.Level),
			PolicyExplanation:    policy.Explanation,
		}
	}
	if skillRequiresTool(input, "memory_write") {
		if command, ok := memoryWriteCommandFromSkillInput(input); ok {
			policy := safety.EvaluatePolicy(safety.PolicyRequest{
				Domain:           safety.DomainPrivacy,
				Action:           safety.ActionWrite,
				Level:            safety.LevelConfirm,
				PolicyMode:       policyMode,
				Actor:            "agent_executor",
				Resource:         "memory_write",
				Mutating:         true,
				PrivacySensitive: true,
				TaskApproved:     false,
			})
			return ExecutionDecision{
				Status:               ExecutionNeedsConfirmation,
				RequestID:            newPermissionRequestID(),
				ToolName:             "memory_write",
				Command:              command,
				RiskLevel:            RiskMedium,
				RequiresConfirmation: true,
				Reason:               "active skill requested saving a confirmed local memory; durable memory writes require explicit approval",
				PolicyLevel:          int(policy.Level),
				PolicyExplanation:    policy.Explanation,
			}
		}
	}
	if shouldRequireRAGToolEvidence(plan, input) {
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "rag_search",
			Command:   []string{"rag_search", input.Content},
			RiskLevel: RiskLow,
			Reason:    "safe local document search is required before answering because no retrieved local document context is available",
		}
	}
	if plan.TaskType != TaskTool {
		return ExecutionDecision{
			Status:    ExecutionNotRequired,
			RiskLevel: plan.RiskLevel,
			Reason:    "no tool action is required before model response",
		}
	}

	if containsTool(plan.ToolsNeeded, "local_time") {
		command := []string{"local_time"}
		if target := routing.ResolveLocalTimeTarget(input.Content); target.TimeZone != "" {
			command = append(command, target.TimeZone, target.Label)
		}
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "local_time",
			Command:   command,
			RiskLevel: RiskLow,
			Reason:    "local system clock should answer the current-time request",
		}
	}
	if containsTool(plan.ToolsNeeded, "ingest_documents") {
		target := ""
		if len(plan.FilesNeeded) > 0 {
			target = strings.TrimSpace(plan.FilesNeeded[0])
		}
		if target == "" {
			if hints := routing.FileHints(input.Content); len(hints) > 0 {
				target = strings.TrimSpace(hints[0])
			}
		}
		if target == "" {
			return ExecutionDecision{
				Status:    ExecutionBlocked,
				ToolName:  "ingest_documents",
				RiskLevel: RiskLow,
				Reason:    "document ingestion requires a file or folder path",
			}
		}
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "ingest_documents",
			Command:   []string{"ingest_documents", target},
			RiskLevel: RiskLow,
			Reason:    "safe local document ingestion should index the requested file or folder before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "run_tests") {
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "run_tests",
			Command:   []string{"detect", input.Content},
			RiskLevel: RiskLow,
			Reason:    "safe test command should run before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "git_status") {
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "git_status",
			Command:   []string{"git", "status", "--short"},
			RiskLevel: RiskLow,
			Reason:    "safe git status should run before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "git_diff") {
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "git_diff",
			Command:   []string{"git", "diff"},
			RiskLevel: RiskLow,
			Reason:    "safe git diff should run before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "patch_preview") {
		command, ok := patchPreviewCommand(input.Content)
		if !ok {
			return ExecutionDecision{
				Status:    ExecutionBlocked,
				ToolName:  "patch_preview",
				RiskLevel: RiskLow,
				Reason:    "patch_preview requires a workspace path and replacement content marked with `content:`",
			}
		}
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "patch_preview",
			Command:   command,
			RiskLevel: RiskLow,
			Reason:    "safe patch preview should be generated without writing files",
		}
	}
	for _, tool := range []string{"doctor_status", "heartbeat_status", "project_map", "symbol_search", "secret_scan"} {
		if containsTool(plan.ToolsNeeded, tool) {
			command := []string{tool}
			if tool == "symbol_search" {
				command = []string{tool, input.Content}
			}
			return ExecutionDecision{
				Status:    ExecutionReady,
				ToolName:  tool,
				Command:   command,
				RiskLevel: RiskLow,
				Reason:    "safe deterministic local context should be gathered before answering",
			}
		}
	}
	if containsTool(plan.ToolsNeeded, "internet_fetch") || containsTool(plan.ToolsNeeded, "internet_head") || containsTool(plan.ToolsNeeded, "internet_search") {
		target, ok := internetURLHint(input.Content)
		if !ok {
			if containsTool(plan.ToolsNeeded, "internet_search") {
				return ExecutionDecision{
					Status:    ExecutionReady,
					ToolName:  "internet_search",
					Command:   []string{"internet_search", internetSearchQueryForInput(input)},
					RiskLevel: RiskMedium,
					Reason:    "controlled core internet search can gather public context before answering",
				}
			}
			return ExecutionDecision{
				Status:    ExecutionBlocked,
				ToolName:  "internet_fetch",
				RiskLevel: RiskMedium,
				Reason:    "controlled internet fetch requires an explicit http(s) URL or domain; internet search is disabled until a provider is configured",
			}
		}
		if containsTool(plan.ToolsNeeded, "internet_search") && !containsTool(plan.ToolsNeeded, "internet_fetch") && !containsTool(plan.ToolsNeeded, "internet_head") {
			return ExecutionDecision{
				Status:    ExecutionReady,
				ToolName:  "internet_search",
				Command:   []string{"internet_search", internetSearchQueryForInput(input)},
				RiskLevel: RiskMedium,
				Reason:    "controlled core internet search can gather public context before answering",
			}
		}
		tool := "internet_fetch"
		if containsTool(plan.ToolsNeeded, "internet_head") {
			tool = "internet_head"
		}
		host, err := hostFromAgentURL(target)
		if err != nil {
			return ExecutionDecision{
				Status:    ExecutionBlocked,
				ToolName:  tool,
				RiskLevel: RiskMedium,
				Reason:    err.Error(),
			}
		}
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  tool,
			Command:   []string{tool, target, host},
			RiskLevel: RiskMedium,
			Reason:    "controlled core internet tool can gather the requested public URL before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "file_tree") {
		target := "."
		if len(plan.FilesNeeded) > 0 && strings.TrimSpace(plan.FilesNeeded[0]) != "" {
			target = strings.TrimSpace(plan.FilesNeeded[0])
		}
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "file_tree",
			Command:   []string{"file_tree", target},
			RiskLevel: RiskLow,
			Reason:    "safe bounded workspace file tree should run before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "memory_search") {
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "memory_search",
			Command:   []string{"memory_search", input.Content},
			RiskLevel: RiskLow,
			Reason:    "safe local memory search should run before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "list_files") {
		target := "."
		if len(plan.FilesNeeded) > 0 && strings.TrimSpace(plan.FilesNeeded[0]) != "" {
			target = strings.TrimSpace(plan.FilesNeeded[0])
		}
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "list_files",
			Command:   []string{"list_files", target},
			RiskLevel: RiskLow,
			Reason:    "safe workspace directory listing should run before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "file_stat") {
		target := ""
		if len(plan.FilesNeeded) > 0 && strings.TrimSpace(plan.FilesNeeded[0]) != "" {
			target = strings.TrimSpace(plan.FilesNeeded[0])
		}
		if target == "" {
			if hints := routing.FileHints(input.Content); len(hints) > 0 {
				target = strings.TrimSpace(hints[0])
			}
		}
		if target == "" {
			return ExecutionDecision{
				Status:    ExecutionBlocked,
				ToolName:  "file_stat",
				RiskLevel: RiskLow,
				Reason:    "file_stat requires an explicit workspace path",
			}
		}
		return ExecutionDecision{
			Status:    ExecutionReady,
			ToolName:  "file_stat",
			Command:   []string{"file_stat", target},
			RiskLevel: RiskLow,
			Reason:    "safe workspace file metadata should run before answering",
		}
	}
	if containsTool(plan.ToolsNeeded, "read_file") || containsTool(plan.ToolsNeeded, "search_files") || containsTool(plan.ToolsNeeded, "rag_search") {
		if strings.TrimSpace(input.WorkspaceContext) != "" {
			return ExecutionDecision{
				Status:    ExecutionAlreadyProvided,
				ToolName:  firstTool(plan.ToolsNeeded),
				RiskLevel: plan.RiskLevel,
				Reason:    "retrieved context is already present",
			}
		}
		if containsTool(plan.ToolsNeeded, "read_file") && len(plan.FilesNeeded) > 0 {
			return ExecutionDecision{
				Status:    ExecutionReady,
				ToolName:  "read_file",
				Command:   []string{"read_file", plan.FilesNeeded[0]},
				RiskLevel: RiskLow,
				Reason:    "safe file read should run before answering",
			}
		}
		if containsTool(plan.ToolsNeeded, "rag_search") {
			return ExecutionDecision{
				Status:    ExecutionReady,
				ToolName:  "rag_search",
				Command:   []string{"rag_search", input.Content},
				RiskLevel: RiskLow,
				Reason:    "safe local document search should run before answering",
			}
		}
		if containsTool(plan.ToolsNeeded, "search_files") {
			return ExecutionDecision{
				Status:    ExecutionReady,
				ToolName:  "search_files",
				Command:   []string{"search_files", input.Content},
				RiskLevel: RiskLow,
				Reason:    "safe workspace search should run before answering",
			}
		}
		return ExecutionDecision{
			Status:    ExecutionBlocked,
			ToolName:  "read_file",
			RiskLevel: RiskLow,
			Reason:    "read_file requires an explicit workspace path",
		}
	}
	if containsTool(plan.ToolsNeeded, "internet_crawl") {
		target := ""
		if url, ok := internetURLHint(input.Content); ok {
			target = url
		}
		command := []string{"internet_crawl"}
		if target != "" {
			command = append(command, target)
		}
		return ExecutionDecision{
			Status:    ExecutionBlocked,
			ToolName:  "internet_crawl",
			Command:   command,
			RiskLevel: RiskMedium,
			Reason:    "bounded website crawl requires explicit approval and limits; use Automation > Internet Service > Bounded Crawl",
		}
	}

	return ExecutionDecision{
		Status:    ExecutionBlocked,
		ToolName:  firstTool(plan.ToolsNeeded),
		RiskLevel: plan.RiskLevel,
		Reason:    "no matching safe executor is available for this tool request",
	}
}

func hasRetrievedEvidence(input PlanInput) bool {
	return strings.TrimSpace(input.WorkspaceContext) != "" || len(input.Sources) > 0
}

func shouldRequireRAGToolEvidence(plan Plan, input PlanInput) bool {
	if plan.TaskType != TaskRAG || hasRetrievedEvidence(input) {
		return false
	}
	content := strings.ToLower(strings.Join(strings.Fields(input.Content), " "))
	return strings.EqualFold(strings.TrimSpace(input.SourceKind), "rag") ||
		routing.LooksRAGTask(content) ||
		routing.LooksDocumentSearchTask(content)
}

func patchPreviewCommand(content string) ([]string, bool) {
	hints := fileHints(content)
	if len(hints) == 0 {
		return nil, false
	}
	marker := "content:"
	lower := strings.ToLower(content)
	index := strings.Index(lower, marker)
	if index < 0 {
		return nil, false
	}
	replacement := strings.TrimSpace(content[index+len(marker):])
	if replacement == "" {
		return nil, false
	}
	return []string{"patch_preview", hints[0], replacement}, true
}

func internetURLHint(content string) (string, bool) {
	for _, field := range strings.Fields(content) {
		if target, ok := normalizeInternetTarget(field); ok {
			return target, true
		}
	}
	return "", false
}

func internetSearchQuery(content string) string {
	query := strings.TrimSpace(routing.CurrentNewsSearchQuery(content))
	if query == "" {
		return content
	}
	query = strings.TrimSpace(stripInternetSearchCommandPrefix(query))
	if routing.LooksCurrentWebFactTask(content) || routing.LooksSpecificCurrentNewsTask(content) {
		now := time.Now()
		year := now.Format("2006")
		date := now.Format("2006-01-02")
		queryLower := strings.ToLower(query)
		if !strings.Contains(queryLower, year) {
			query += " " + year
		}
		if !strings.Contains(queryLower, date) {
			query += " " + date
		}
		if !strings.Contains(queryLower, "latest") && !strings.Contains(queryLower, "current") {
			query += " latest"
		}
	}
	return query
}

func internetSearchQueryForInput(input PlanInput) string {
	if target := internetSearchContinuationTarget(input); target != "" {
		return target
	}
	query := internetSearchQuery(input.Content)
	if target := internetSearchFollowupTarget(input.Content, input.TaskMemory); target != "" {
		if !strings.Contains(strings.ToLower(query), strings.ToLower(target)) {
			query = strings.TrimSpace(target + " " + query)
		}
	}
	return query
}

func internetSearchFollowupTarget(content string, taskMemory string) string {
	if strings.TrimSpace(taskMemory) == "" {
		return ""
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
	if normalized == "" {
		return ""
	}
	hasFollowupReference := containsAnySearchFollowupTerm(normalized,
		"him", "her", "them", "that person", "the person", "that company",
		"the company", "that business", "the business", "that ceo", "the ceo",
		"that target", "the target",
	)
	if !hasFollowupReference {
		return ""
	}
	if target := searchTargetFromTaskMemory(taskMemory); target != "" {
		return target
	}
	return ""
}

func internetSearchContinuationTarget(input PlanInput) string {
	frame := input.Continuation
	switch frame.Kind {
	case routing.ContinuationKindRetry:
		if frame.PriorToolName == "internet_search" || frame.PriorRouteCategory == routing.RouteInternetSearch {
			return strings.TrimSpace(frame.PriorTarget)
		}
	case routing.ContinuationKindTargetCorrection:
		if frame.PriorToolName == "internet_search" || frame.PriorRouteCategory == routing.RouteInternetSearch {
			return strings.TrimSpace(firstNonEmptyString(frame.NewTarget, frame.PriorTarget))
		}
	case routing.ContinuationKindSourceCorrection:
		if frame.NewSourceOfTruth == routing.PreflightSourceInternet {
			return strings.TrimSpace(firstNonEmptyString(frame.NewTarget, frame.PriorTarget))
		}
	}
	return ""
}

func containsAnySearchFollowupTerm(content string, terms ...string) bool {
	for _, term := range terms {
		if routing.ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func searchTargetFromTaskMemory(taskMemory string) string {
	for _, line := range strings.Split(taskMemory, "\n") {
		fields := searchTargetFields(line)
		if len(fields) == 0 || !strings.EqualFold(fields[0], "user") {
			continue
		}
		target := cleanSearchTarget(strings.Join(fields[1:], " "))
		if target != "" && !containsAnySearchFollowupTerm(strings.ToLower(target), "try again", "retry", "search more about him") {
			return target
		}
	}
	return ""
}

func searchTargetFields(line string) []string {
	rawFields := strings.Fields(line)
	fields := make([]string, 0, len(rawFields))
	for _, field := range rawFields {
		field = strings.Trim(field, " \t\r\n:;,.!?()[]{}\"`")
		if field == "" {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func cleanSearchTarget(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, " \t\r\n:;,.!?()[]{}\"'`")
	if len(value) > 120 {
		value = strings.TrimSpace(value[:120])
	}
	return value
}

func stripInternetSearchCommandPrefix(content string) string {
	trimmed := routing.StripSearchIntentPrefix(strings.ToLower(strings.Join(strings.Fields(content), " ")))
	for _, prefix := range []string{
		"search for more information on ",
		"search more information on ",
		"search for information on ",
		"search information on ",
		"search the internet for ",
		"search the web for ",
		"search web for ",
		"internet search for ",
		"web search for ",
		"look up ",
		"lookup ",
		"search for ",
		"search about ",
		"search ",
		"google ",
	} {
		if strings.HasPrefix(trimmed, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		}
	}
	return content
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
		if _, err := hostFromAgentURL(candidate); err != nil {
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

func hostFromAgentURL(raw string) (string, error) {
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

func MissingExecutorResponse(decision ExecutionDecision) string {
	switch decision.Status {
	case ExecutionNeedsConfirmation:
		if request, ok := PermissionForDecision(decision); ok {
			return fmt.Sprintf("I can do this, but `%s` needs your approval first.\n\n**Why:** %s\n\n**Next step:** %s", request.ToolName, request.Reason, request.NextStep)
		}
		tool := decision.ToolName
		if tool == "" {
			tool = "the requested action"
		}
		return fmt.Sprintf("I can do this, but `%s` needs your approval first.\n\n**Why:** %s", tool, decision.Reason)
	case ExecutionBlocked:
		return FriendlyBlockedDecisionResponse(decision)
	case ExecutionReady:
		tool := decision.ToolName
		if tool == "" {
			tool = "a safe tool"
		}
		return fmt.Sprintf("I need %s output before I can answer this safely.\n\n**Why:** %s", tool, decision.Reason)
	default:
		return ""
	}
}

func PermissionForDecision(decision ExecutionDecision) (PermissionRequest, bool) {
	if decision.Status != ExecutionNeedsConfirmation {
		return PermissionRequest{}, false
	}
	tool := decision.ToolName
	if tool == "" {
		tool = "requested_action"
	}
	requestID := decision.RequestID
	if requestID == "" {
		requestID = newPermissionRequestID()
	}
	request := PermissionRequest{
		RequestID:            requestID,
		ToolName:             tool,
		Command:              append([]string(nil), decision.Command...),
		RiskLevel:            decision.RiskLevel,
		Reason:               decision.Reason,
		RequiresConfirmation: true,
		WorkspaceOnly:        true,
		NextStep:             "Review the requested action, then approve or reject it.",
		PolicyLevel:          decision.PolicyLevel,
		PolicyExplanation:    decision.PolicyExplanation,
	}
	if request.PolicyExplanation == "" {
		policy := safety.EvaluatePolicy(policyRequestForPermission(tool, decision.RiskLevel, safety.PolicyModeSafe))
		request.PolicyLevel = int(policy.Level)
		request.PolicyExplanation = policy.Explanation
	}
	if tool == "edit_file" {
		request.DiffPreview = true
		request.SnapshotBeforeWrite = true
		request.RollbackSupported = true
		request.NextStep = "Review the diff preview, then approve to create a snapshot and apply the workspace-only edit."
	}
	if tool == "memory_write" {
		request.WorkspaceOnly = false
		request.NextStep = "Review the exact memory kind and content, then approve only if this should be saved locally."
	}
	if decision.RiskLevel == RiskHigh {
		request.Destructive = true
		request.NextStep = "Approve only after confirming the exact command or file change is intended."
	}
	return request, true
}

func ensurePermissionDecisionRequestID(decision ExecutionDecision) ExecutionDecision {
	if decision.Status != ExecutionNeedsConfirmation {
		return decision
	}
	decision.RequestID = strings.TrimSpace(decision.RequestID)
	if decision.RequestID == "" {
		decision.RequestID = newPermissionRequestID()
	}
	return decision
}

func editFileConfirmationCommand(plan Plan, input PlanInput) []string {
	path := firstEditPath(plan, input)
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return []string{"edit_file", strings.TrimSpace(path)}
}

func emitExecutionDecision(emit EventHandler, decision ExecutionDecision) error {
	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventExecutionDecided,
		Data: map[string]string{
			"status":                decision.Status,
			"request_id":            decision.RequestID,
			"tool_name":             decision.ToolName,
			"risk_level":            decision.RiskLevel,
			"requires_confirmation": fmt.Sprintf("%t", decision.RequiresConfirmation),
			"reason":                decision.Reason,
			"command":               strings.Join(decision.Command, " "),
		},
	})
}

func emitPermissionRequest(emit EventHandler, request PermissionRequest) error {
	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventPermissionRequested,
		Data: map[string]string{
			"tool_name":             request.ToolName,
			"request_id":            request.RequestID,
			"command_json":          permissionCommandJSON(request.Command),
			"risk_level":            request.RiskLevel,
			"reason":                request.Reason,
			"requires_confirmation": fmt.Sprintf("%t", request.RequiresConfirmation),
			"workspace_only":        fmt.Sprintf("%t", request.WorkspaceOnly),
			"diff_preview":          fmt.Sprintf("%t", request.DiffPreview),
			"snapshot_before_write": fmt.Sprintf("%t", request.SnapshotBeforeWrite),
			"rollback_supported":    fmt.Sprintf("%t", request.RollbackSupported),
			"destructive":           fmt.Sprintf("%t", request.Destructive),
			"next_step":             request.NextStep,
			"policy_level":          fmt.Sprintf("%d", request.PolicyLevel),
			"policy_explanation":    request.PolicyExplanation,
		},
	})
}

func permissionCommandJSON(command []string) string {
	if len(command) == 0 {
		return ""
	}
	data, err := json.Marshal(command)
	if err != nil {
		return ""
	}
	return string(data)
}

func emitEditProposal(emit EventHandler, proposal EditProposal) error {
	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventEditProposed,
		Data: map[string]string{
			"request_id":            proposal.RequestID,
			"path":                  proposal.Path,
			"content":               proposal.Content,
			"content_source":        proposal.ContentSource,
			"status":                proposal.Status,
			"reason":                proposal.Reason,
			"needs_content":         fmt.Sprintf("%t", proposal.NeedsContent),
			"diff_preview":          fmt.Sprintf("%t", proposal.DiffPreview),
			"snapshot_before_write": fmt.Sprintf("%t", proposal.SnapshotBeforeWrite),
			"rollback_supported":    fmt.Sprintf("%t", proposal.RollbackSupported),
		},
	})
}

func emitToolCompleted(emit EventHandler, decision ExecutionDecision, result ExecutionResult) error {
	if emit == nil {
		return nil
	}
	return emit(Event{
		Type: EventToolCompleted,
		Data: map[string]string{
			"tool_name":   decision.ToolName,
			"status":      result.Status,
			"risk_level":  decision.RiskLevel,
			"source_kind": result.SourceKind,
			"sources":     strings.Join(result.Sources, ", "),
		},
	})
}

func newPermissionRequestID() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "perm_pending"
	}
	return "perm_" + hex.EncodeToString(data[:])
}

func policyRequestForPermission(tool string, riskLevel string, policyMode string) safety.PolicyRequest {
	request := safety.PolicyRequest{
		Domain:     safety.DomainGeneral,
		Action:     safety.ActionRun,
		Level:      safety.LevelConfirm,
		PolicyMode: policyMode,
		Actor:      "permission_prompt",
		Resource:   tool,
	}
	if tool == "edit_file" || strings.Contains(tool, "write") {
		request.Domain = safety.DomainFilesystem
		request.Action = safety.ActionWrite
		request.FileWrite = true
		request.Mutating = true
	}
	if tool == "memory_write" {
		request.Domain = safety.DomainPrivacy
		request.Action = safety.ActionWrite
		request.FileWrite = false
		request.Mutating = true
		request.PrivacySensitive = true
	}
	if riskLevel == RiskHigh {
		request.Mutating = true
	}
	return request
}

func skillRequiresTool(input PlanInput, tool string) bool {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return false
	}
	for _, required := range input.SkillRequiredTools {
		if strings.TrimSpace(required) == tool {
			return true
		}
	}
	return false
}

func memoryWriteCommandFromSkillInput(input PlanInput) ([]string, bool) {
	content := strings.TrimSpace(input.Content)
	if content == "" || looksMemoryWriteExplanation(content) || !looksDirectMemoryWriteRequest(content) {
		return nil, false
	}
	memoryContent := extractMemoryWriteContent(content)
	if memoryContent == "" {
		return nil, false
	}
	kind := "note"
	lower := strings.ToLower(content)
	switch {
	case strings.Contains(lower, "follow-up") || strings.Contains(lower, "follow up") || strings.Contains(lower, "followup"):
		kind = "follow_up"
	case strings.Contains(lower, "action item") || strings.Contains(lower, "action items"):
		kind = "action_item"
	case strings.Contains(lower, "preference") || strings.Contains(lower, "prefer "):
		kind = "preference"
	}
	return []string{"memory_write", kind, memoryContent}, true
}

func looksMemoryWriteExplanation(content string) bool {
	lower := strings.ToLower(strings.TrimSpace(content))
	for _, prefix := range []string{"explain ", "what is ", "what are ", "how do ", "how does ", "how can ", "how should ", "why "} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func looksDirectMemoryWriteRequest(content string) bool {
	lower := strings.ToLower(content)
	hasMemoryTarget := strings.Contains(lower, "follow-up") ||
		strings.Contains(lower, "follow up") ||
		strings.Contains(lower, "followup") ||
		strings.Contains(lower, "action item") ||
		strings.Contains(lower, "action items") ||
		strings.Contains(lower, "remember")
	if !hasMemoryTarget {
		return false
	}
	for _, verb := range []string{"capture", "save", "remember", "store", "write down", "record"} {
		if strings.Contains(lower, verb) {
			return true
		}
	}
	return false
}

func extractMemoryWriteContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if index := strings.Index(content, ":"); index >= 0 {
		if extracted := strings.TrimSpace(content[index+1:]); extracted != "" {
			return extracted
		}
	}
	lower := strings.ToLower(content)
	for _, marker := range []string{"remember to ", "save this follow-up ", "save this follow up ", "capture this follow-up ", "capture this follow up ", "capture follow-up ", "capture follow up ", "record this follow-up ", "record this follow up "} {
		if index := strings.Index(lower, marker); index >= 0 {
			if extracted := strings.TrimSpace(content[index+len(marker):]); extracted != "" {
				return extracted
			}
		}
	}
	return ""
}

func containsTool(tools []string, needle string) bool {
	for _, tool := range tools {
		if tool == needle {
			return true
		}
	}
	return false
}

func firstTool(tools []string) string {
	for _, tool := range tools {
		if strings.TrimSpace(tool) != "" {
			return tool
		}
	}
	return ""
}
