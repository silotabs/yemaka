package contextcore

import (
	"regexp"
	"strings"

	"yemaka/internal/routing"
)

const (
	DefaultTaskStateMaxBytes = 1800
	DefaultTaskStateMaxItems = 8
)

type TaskStateOptions struct {
	MaxBytes int
	MaxItems int
}

type TaskStateInput struct {
	ActiveGoal            string
	ActiveRoute           string
	ActiveTaskFrame       routing.TaskFrame
	PendingApproval       string
	PendingClarification  string
	SelectedSources       []string
	ImportantToolOutcomes []string
	UserConstraints       []string
	DecisionsMade         []string
	UnfinishedSteps       []string
	SourceReferences      []string
	LastKnownOutcome      string
	FreshnessRequired     bool
}

type TaskStatePackage struct {
	ActiveGoal            string            `json:"activeGoal,omitempty"`
	ActiveRoute           string            `json:"activeRoute,omitempty"`
	ActiveTaskFrame       map[string]string `json:"activeTaskFrame,omitempty"`
	PendingApproval       string            `json:"pendingApproval,omitempty"`
	PendingClarification  string            `json:"pendingClarification,omitempty"`
	SelectedSources       []string          `json:"selectedSources,omitempty"`
	ImportantToolOutcomes []string          `json:"importantToolOutcomes,omitempty"`
	UserConstraints       []string          `json:"userConstraints,omitempty"`
	DecisionsMade         []string          `json:"decisionsMade,omitempty"`
	UnfinishedSteps       []string          `json:"unfinishedSteps,omitempty"`
	SourceReferences      []string          `json:"sourceReferences,omitempty"`
	LastKnownOutcome      string            `json:"lastKnownOutcome,omitempty"`
	FreshnessGuard        string            `json:"freshnessGuard,omitempty"`
}

func BuildTaskStatePackage(input TaskStateInput) TaskStatePackage {
	pkg := TaskStatePackage{
		ActiveGoal:            sanitizeTaskStateText(input.ActiveGoal),
		ActiveRoute:           sanitizeTaskStateText(input.ActiveRoute),
		ActiveTaskFrame:       taskStateFrameMap(input.ActiveTaskFrame),
		PendingApproval:       sanitizeTaskStateText(input.PendingApproval),
		PendingClarification:  sanitizeTaskStateText(input.PendingClarification),
		SelectedSources:       compactTaskStateValues(input.SelectedSources, DefaultTaskStateMaxItems),
		ImportantToolOutcomes: compactTaskStateValues(input.ImportantToolOutcomes, DefaultTaskStateMaxItems),
		UserConstraints:       compactTaskStateValues(input.UserConstraints, DefaultTaskStateMaxItems),
		DecisionsMade:         compactTaskStateValues(input.DecisionsMade, DefaultTaskStateMaxItems),
		UnfinishedSteps:       compactTaskStateValues(input.UnfinishedSteps, DefaultTaskStateMaxItems),
		SourceReferences:      compactTaskStateValues(input.SourceReferences, DefaultTaskStateMaxItems),
		LastKnownOutcome:      sanitizeTaskStateText(input.LastKnownOutcome),
	}
	if input.FreshnessRequired || pkg.ActiveRoute == routing.RouteInternetSearch || pkg.ActiveRoute == routing.RouteInternetFetch || pkg.ActiveRoute == routing.RouteInternetHead {
		pkg.FreshnessGuard = "current/fresh routes require current configured provider evidence; memory, summaries, and prior chat are background only and must not be treated as current facts"
	}
	return pkg
}

func TaskStateContextItem(pkg TaskStatePackage, options TaskStateOptions) Item {
	content := RenderTaskStatePackage(pkg, options)
	return TaskStateItem("compact_task_state", "COMPACT TASK STATE", content, 1, WithPriority(95), WithRequired(true), WithMetadata(map[string]string{
		"kind": "task_state_compaction_v1",
	}))
}

func RenderTaskStatePackage(pkg TaskStatePackage, options TaskStateOptions) string {
	options = normalizeTaskStateOptions(options)
	var lines []string
	add := func(label string, value string) {
		value = sanitizeTaskStateText(value)
		if value != "" {
			lines = append(lines, label+": "+value)
		}
	}
	addList := func(label string, values []string) {
		values = compactTaskStateValues(values, options.MaxItems)
		if len(values) > 0 {
			lines = append(lines, label+": "+strings.Join(values, " | "))
		}
	}

	add("active_goal", pkg.ActiveGoal)
	add("active_route", pkg.ActiveRoute)
	if frame := renderTaskStateFrame(pkg.ActiveTaskFrame); frame != "" {
		add("active_task_frame", frame)
	}
	add("pending_approval", pkg.PendingApproval)
	add("pending_clarification", pkg.PendingClarification)
	addList("selected_sources", pkg.SelectedSources)
	addList("important_tool_outcomes", pkg.ImportantToolOutcomes)
	addList("user_constraints", pkg.UserConstraints)
	addList("decisions_made", pkg.DecisionsMade)
	addList("unfinished_steps", pkg.UnfinishedSteps)
	addList("source_references", pkg.SourceReferences)
	add("last_known_outcome", pkg.LastKnownOutcome)
	add("freshness_guard", pkg.FreshnessGuard)
	if len(lines) == 0 {
		return ""
	}
	rendered := CompactWhitespace(strings.Join(lines, "\n"))
	if len(rendered) <= options.MaxBytes {
		return rendered
	}
	return TrimToLimit(rendered, Limit{MaxBytes: options.MaxBytes, MaxTokens: options.MaxBytes / 4})
}

func normalizeTaskStateOptions(options TaskStateOptions) TaskStateOptions {
	if options.MaxBytes <= 0 {
		options.MaxBytes = DefaultTaskStateMaxBytes
	}
	if options.MaxItems <= 0 {
		options.MaxItems = DefaultTaskStateMaxItems
	}
	return options
}

func taskStateFrameMap(frame routing.TaskFrame) map[string]string {
	out := map[string]string{}
	add := func(key string, value string) {
		value = sanitizeTaskStateText(value)
		if value != "" {
			out[key] = value
		}
	}
	add("intent", frame.Intent)
	add("domain", frame.Domain)
	add("target_type", frame.TargetType)
	add("target", frame.Target)
	add("action_type", frame.ActionType)
	add("risk_level", frame.RiskLevel)
	add("policy_decision", frame.PolicyDecision)
	add("route_category", frame.RouteCategory)
	add("capability", frame.Capability)
	if frame.RequiresApproval {
		out["requires_approval"] = "true"
	}
	if frame.RequiresClarification {
		out["requires_clarification"] = "true"
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func renderTaskStateFrame(frame map[string]string) string {
	if len(frame) == 0 {
		return ""
	}
	order := []string{
		"intent", "domain", "target_type", "target", "action_type", "risk_level",
		"policy_decision", "route_category", "capability", "requires_approval",
		"requires_clarification",
	}
	parts := make([]string, 0, len(frame))
	for _, key := range order {
		if value := strings.TrimSpace(frame[key]); value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	return strings.Join(parts, ", ")
}

func compactTaskStateValues(values []string, maxItems int) []string {
	if maxItems <= 0 {
		maxItems = DefaultTaskStateMaxItems
	}
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = sanitizeTaskStateText(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
		if len(out) >= maxItems {
			break
		}
	}
	return out
}

func sanitizeTaskStateText(input string) string {
	input = CompactWhitespace(input)
	for _, pattern := range taskStateSecretPatterns {
		input = pattern.ReplaceAllStringFunc(input, redactTaskStateSecret)
	}
	return strings.TrimSpace(input)
}

func redactTaskStateSecret(input string) string {
	if key, _, ok := strings.Cut(input, "="); ok {
		return strings.TrimSpace(key) + "=[REDACTED]"
	}
	if key, _, ok := strings.Cut(input, ":"); ok {
		return strings.TrimSpace(key) + ": [REDACTED]"
	}
	return "[REDACTED]"
}

var taskStateSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(api[_-]?key|token|password|secret)\b\s*[:=]\s*["']?[^"'\s,}]+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}\b`),
	regexp.MustCompile(`\b[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{20,}\b`),
}
