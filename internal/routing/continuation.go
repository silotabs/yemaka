package routing

import "strings"

const (
	ContinuationKindNewTask          = "new_task"
	ContinuationKindRetry            = "retry"
	ContinuationKindTargetCorrection = "target_correction"
	ContinuationKindSourceCorrection = "source_correction"
	ContinuationKindApproval         = "approval"
	ContinuationKindCancel           = "cancel"
	ContinuationKindClarify          = "clarify"

	ContinuationTargetExplicitPrompt    = "explicit_prompt"
	ContinuationTargetPriorRoute        = "prior_route"
	ContinuationTargetPriorToolRun      = "prior_tool_run"
	ContinuationTargetPriorConversation = "prior_conversation"
	ContinuationTargetMissing           = "missing"
	ContinuationTargetAmbiguous         = "ambiguous"
)

type ContinuationPriorState struct {
	RouteCategory string `json:"route_category,omitempty"`
	ToolName      string `json:"tool_name,omitempty"`
	Target        string `json:"target,omitempty"`
	SourceOfTruth string `json:"source_of_truth,omitempty"`
	FailureStatus string `json:"failure_status,omitempty"`
	FailureReason string `json:"failure_reason,omitempty"`
}

type ContinuationFrame struct {
	RawPrompt          string   `json:"raw_prompt,omitempty"`
	NormalizedPrompt   string   `json:"normalized_prompt,omitempty"`
	Kind               string   `json:"kind,omitempty"`
	PriorRouteCategory string   `json:"prior_route_category,omitempty"`
	PriorToolName      string   `json:"prior_tool_name,omitempty"`
	PriorTarget        string   `json:"prior_target,omitempty"`
	PriorSourceOfTruth string   `json:"prior_source_of_truth,omitempty"`
	PriorFailureStatus string   `json:"prior_failure_status,omitempty"`
	PriorFailureReason string   `json:"prior_failure_reason,omitempty"`
	NewTarget          string   `json:"new_target,omitempty"`
	NewSourceOfTruth   string   `json:"new_source_of_truth,omitempty"`
	TargetSource       string   `json:"target_source,omitempty"`
	MissingSlots       []string `json:"missing_slots,omitempty"`
	ConflictSignals    []string `json:"conflict_signals,omitempty"`
	Reasons            []string `json:"reasons,omitempty"`
}

type ContinuationInput struct {
	Content    string
	TaskMemory string
	Prior      ContinuationPriorState
}

func (frame ContinuationFrame) IsZero() bool {
	return strings.TrimSpace(frame.RawPrompt) == "" &&
		strings.TrimSpace(frame.NormalizedPrompt) == "" &&
		strings.TrimSpace(frame.Kind) == ""
}

func BuildContinuationFrame(input ContinuationInput) ContinuationFrame {
	raw := strings.TrimSpace(input.Content)
	normalized := strings.ToLower(strings.Join(strings.Fields(raw), " "))
	frame := ContinuationFrame{
		RawPrompt:          raw,
		NormalizedPrompt:   normalized,
		Kind:               ContinuationKindNewTask,
		PriorRouteCategory: strings.TrimSpace(input.Prior.RouteCategory),
		PriorToolName:      strings.TrimSpace(input.Prior.ToolName),
		PriorTarget:        strings.TrimSpace(input.Prior.Target),
		PriorSourceOfTruth: strings.TrimSpace(input.Prior.SourceOfTruth),
		PriorFailureStatus: strings.TrimSpace(input.Prior.FailureStatus),
		PriorFailureReason: strings.TrimSpace(input.Prior.FailureReason),
		TargetSource:       ContinuationTargetMissing,
	}
	if frame.PriorTarget != "" {
		frame.TargetSource = ContinuationTargetPriorToolRun
	}
	if normalized == "" {
		frame.Kind = ContinuationKindClarify
		frame.MissingSlots = appendContinuationUnique(frame.MissingSlots, "prompt")
		frame.Reasons = appendContinuationUnique(frame.Reasons, "empty continuation prompt")
		return frame
	}
	if looksContinuationCancel(normalized) {
		frame.Kind = ContinuationKindCancel
		frame.Reasons = appendContinuationUnique(frame.Reasons, "user cancelled prior action")
		return continuationFinalize(frame)
	}
	if looksAuthorizationOnlyFollowup(normalized) {
		frame.Kind = ContinuationKindApproval
		frame.Reasons = appendContinuationUnique(frame.Reasons, "authorization-only continuation")
		return continuationFinalize(frame)
	}
	if source := continuationExplicitSource(normalized); source != "" && looksContinuationSourceCorrection(normalized) {
		frame.Kind = ContinuationKindSourceCorrection
		frame.NewSourceOfTruth = source
		frame.TargetSource = continuationPriorTargetSource(frame.TargetSource)
		frame.Reasons = appendContinuationUnique(frame.Reasons, "user corrected source of truth")
		return continuationFinalize(frame)
	}
	if target := continuationExplicitTargetCorrection(raw, normalized); target != "" {
		frame.Kind = ContinuationKindTargetCorrection
		frame.NewTarget = target
		frame.TargetSource = ContinuationTargetExplicitPrompt
		frame.Reasons = appendContinuationUnique(frame.Reasons, "user corrected target")
		if source := continuationExplicitSource(normalized); source != "" {
			frame.NewSourceOfTruth = source
		}
		return continuationFinalize(frame)
	}
	if looksContinuationRetry(normalized) {
		frame.Kind = ContinuationKindRetry
		frame.TargetSource = continuationPriorTargetSource(frame.TargetSource)
		frame.Reasons = appendContinuationUnique(frame.Reasons, "user requested retry")
		if frame.PriorRouteCategory == "" && frame.PriorToolName == "" {
			frame.MissingSlots = appendContinuationUnique(frame.MissingSlots, "prior_route")
		}
		if frame.PriorTarget == "" {
			if target, ambiguous := continuationPriorConversationTarget(input.TaskMemory); ambiguous {
				frame.TargetSource = ContinuationTargetAmbiguous
				frame.MissingSlots = appendContinuationUnique(frame.MissingSlots, "ambiguous_prior_target")
				frame.ConflictSignals = appendContinuationUnique(frame.ConflictSignals, "multiple prior conversation targets")
			} else if target != "" {
				frame.PriorTarget = target
				frame.TargetSource = ContinuationTargetPriorConversation
			} else {
				frame.MissingSlots = appendContinuationUnique(frame.MissingSlots, "target")
			}
		}
		return continuationFinalize(frame)
	}
	if looksContinuationVagueReference(normalized) {
		if target, ambiguous := continuationPriorConversationTarget(input.TaskMemory); ambiguous && frame.PriorTarget == "" {
			frame.Kind = ContinuationKindClarify
			frame.TargetSource = ContinuationTargetAmbiguous
			frame.MissingSlots = appendContinuationUnique(frame.MissingSlots, "ambiguous_prior_target")
			frame.ConflictSignals = appendContinuationUnique(frame.ConflictSignals, "multiple prior conversation targets")
			frame.Reasons = appendContinuationUnique(frame.Reasons, "vague continuation has multiple possible targets")
		} else if frame.PriorTarget == "" && target != "" {
			frame.PriorTarget = target
			frame.TargetSource = ContinuationTargetPriorConversation
			frame.Kind = ContinuationKindRetry
			frame.Reasons = appendContinuationUnique(frame.Reasons, "vague continuation can carry prior target")
		}
	}
	return continuationFinalize(frame)
}

func continuationFinalize(frame ContinuationFrame) ContinuationFrame {
	if frame.Kind == "" {
		frame.Kind = ContinuationKindNewTask
	}
	if frame.TargetSource == "" {
		frame.TargetSource = ContinuationTargetMissing
	}
	if frame.PriorSourceOfTruth == "" {
		frame.PriorSourceOfTruth = continuationSourceForRoute(frame.PriorRouteCategory, frame.PriorToolName)
	}
	return frame
}

func continuationPriorTargetSource(current string) string {
	if current != "" && current != ContinuationTargetMissing {
		return current
	}
	return ContinuationTargetPriorRoute
}

func looksContinuationRetry(content string) bool {
	return containsAnyContinuationTerm(content,
		"try again",
		"retry",
		"retry that",
		"retry it",
		"do it again",
		"run it again",
		"search again",
		"try the search again",
	)
}

func looksContinuationCancel(content string) bool {
	return containsAnyContinuationTerm(content, "cancel", "stop", "never mind", "nevermind", "forget it")
}

func looksContinuationSourceCorrection(content string) bool {
	return containsAnyContinuationTerm(content, "i meant", "no,", "instead", "not internet", "not web", "not online", "use")
}

func looksContinuationVagueReference(content string) bool {
	return containsAnyContinuationTerm(content,
		"it", "this", "that", "there", "them", "him", "her",
		"that target", "the target", "that company", "the company",
	)
}

func continuationExplicitSource(content string) string {
	switch {
	case containsAnyContinuationTerm(content, "ingested document", "ingested documents", "indexed document", "indexed documents", "uploaded pdf", "uploaded document", "local documents", "local docs", "rag"):
		return PreflightSourceLocalDocuments
	case containsAnyContinuationTerm(content, "memory", "conversation history", "what i said", "what we discussed"):
		return PreflightSourceMemory
	case containsAnyContinuationTerm(content, "repo", "repository", "workspace", "project files", "local files", "codebase"):
		return PreflightSourceWorkspace
	case containsAnyContinuationTerm(content, "web search", "internet search", "search online", "use web", "use internet", "latest", "current"):
		return PreflightSourceInternet
	default:
		return ""
	}
}

func continuationExplicitTargetCorrection(raw string, normalized string) string {
	for _, marker := range []string{"i meant ", "no, i meant ", "no i meant "} {
		if idx := strings.Index(normalized, marker); idx >= 0 {
			start := idx + len(marker)
			return cleanContinuationTarget(raw[start:])
		}
	}
	lowerRaw := strings.ToLower(raw)
	for _, marker := range []string{" as the target", " as target"} {
		if idx := strings.LastIndex(lowerRaw, marker); idx > 0 && strings.TrimSpace(lowerRaw[idx+len(marker):]) == "" {
			return cleanContinuationTarget(stripContinuationTargetLeadIn(raw[:idx]))
		}
	}
	for _, marker := range []string{"target is ", "target: ", "target = "} {
		if idx := strings.Index(lowerRaw, marker); idx >= 0 {
			return cleanContinuationTarget(raw[idx+len(marker):])
		}
	}
	return ""
}

func stripContinuationTargetLeadIn(value string) string {
	trimmed := strings.TrimSpace(value)
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"please use ", "yes use ", "use ", "please set ", "set "} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(trimmed[len(prefix):])
		}
	}
	return trimmed
}

func continuationPriorConversationTarget(taskMemory string) (string, bool) {
	targets := continuationPriorConversationTargets(taskMemory)
	if len(targets) == 1 {
		return targets[0], false
	}
	return "", len(targets) > 1
}

func continuationPriorConversationTargets(taskMemory string) []string {
	targets := []string{}
	seen := map[string]bool{}
	lines := strings.Split(taskMemory, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "user:") {
			continue
		}
		target := cleanContinuationTarget(line[strings.Index(line, ":")+1:])
		if target == "" || looksContinuationRetry(strings.ToLower(target)) {
			continue
		}
		key := strings.ToLower(target)
		if seen[key] {
			continue
		}
		seen[key] = true
		targets = append(targets, target)
		if len(targets) > 1 {
			return targets
		}
	}
	return targets
}

func continuationSourceForRoute(route string, tool string) string {
	switch strings.TrimSpace(route) {
	case RouteRAGSearch:
		return PreflightSourceLocalDocuments
	case RouteMemorySearch:
		return PreflightSourceMemory
	case RouteWorkspaceRead, RouteFileRead:
		return PreflightSourceWorkspace
	case RouteInternetSearch, RouteInternetFetch, RouteInternetHead:
		return PreflightSourceInternet
	}
	switch strings.TrimSpace(tool) {
	case "rag_search":
		return PreflightSourceLocalDocuments
	case "memory_search":
		return PreflightSourceMemory
	case "read_file", "search_files", "list_files", "file_tree", "file_stat", "symbol_search":
		return PreflightSourceWorkspace
	case "internet_search", "internet_fetch", "internet_head":
		return PreflightSourceInternet
	default:
		return ""
	}
}

func ContinuationSourceForRoute(route string, tool string) string {
	return continuationSourceForRoute(route, tool)
}

func continuationRouteForTool(tool string) string {
	switch strings.TrimSpace(tool) {
	case "rag_search":
		return RouteRAGSearch
	case "memory_search":
		return RouteMemorySearch
	case "read_file":
		return RouteFileRead
	case "search_files", "list_files", "file_tree", "file_stat", "symbol_search":
		return RouteWorkspaceRead
	case "internet_search":
		return RouteInternetSearch
	case "internet_fetch":
		return RouteInternetFetch
	case "internet_head":
		return RouteInternetHead
	default:
		return ""
	}
}

func RouteCategoryForTool(tool string) string {
	return continuationRouteForTool(tool)
}

func containsAnyContinuationTerm(content string, terms ...string) bool {
	for _, term := range terms {
		if ContainsSearchTerm(content, term) {
			return true
		}
	}
	return false
}

func appendContinuationUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		addition = strings.TrimSpace(addition)
		if addition == "" || containsContinuationValue(values, addition) {
			continue
		}
		values = append(values, addition)
	}
	return values
}

func containsContinuationValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cleanContinuationTarget(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, " \t\r\n:;,.!?()[]{}\"'`")
	if len(value) > 160 {
		value = strings.TrimSpace(value[:160])
	}
	return value
}
