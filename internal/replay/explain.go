package replay

import (
	"fmt"
	"strings"
)

type TraceExplanation struct {
	Schema             string                  `json:"schema"`
	ID                 string                  `json:"id"`
	RequestSummary     string                  `json:"requestSummary,omitempty"`
	Route              RouteExplanation        `json:"route"`
	ToolsConsidered    []ToolConsiderationInfo `json:"toolsConsidered,omitempty"`
	ToolsUsed          []ToolExplanation       `json:"toolsUsed,omitempty"`
	MemoryUsed         []SourceExplanation     `json:"memoryUsed,omitempty"`
	RAGDocsUsed        []SourceExplanation     `json:"ragDocsUsed,omitempty"`
	Context            *ContextExplanation     `json:"context,omitempty"`
	Permissions        []PermissionExplanation `json:"permissions,omitempty"`
	Verification       VerificationExplanation `json:"verification,omitempty"`
	Result             ResultExplanation       `json:"result,omitempty"`
	MissingCapability  *CapabilityExplanation  `json:"missingCapability,omitempty"`
	DiagnosticWarnings []string                `json:"diagnosticWarnings,omitempty"`
}

type RouteExplanation struct {
	Category      string   `json:"category,omitempty"`
	Intent        string   `json:"intent,omitempty"`
	Domain        string   `json:"domain,omitempty"`
	Target        string   `json:"target,omitempty"`
	Capability    string   `json:"capability,omitempty"`
	RiskLevel     string   `json:"riskLevel,omitempty"`
	Confidence    int      `json:"confidence,omitempty"`
	Reasons       []string `json:"reasons,omitempty"`
	UsesTool      bool     `json:"usesTool,omitempty"`
	UsesInternet  bool     `json:"usesInternet,omitempty"`
	ReadsFiles    bool     `json:"readsFiles,omitempty"`
	WritesFiles   bool     `json:"writesFiles,omitempty"`
	NeedsApproval bool     `json:"needsApproval,omitempty"`
	NeedsClarify  bool     `json:"needsClarify,omitempty"`
	GeneratesTool bool     `json:"generatesTool,omitempty"`
	CreatesJob    bool     `json:"createsJob,omitempty"`
}

type ToolExplanation struct {
	Name          string `json:"name,omitempty"`
	Status        string `json:"status,omitempty"`
	RiskLevel     string `json:"riskLevel,omitempty"`
	InputSummary  string `json:"inputSummary,omitempty"`
	OutputSummary string `json:"outputSummary,omitempty"`
	Error         string `json:"error,omitempty"`
}

type ToolConsiderationInfo struct {
	Name       string      `json:"name,omitempty"`
	Source     string      `json:"source,omitempty"`
	Status     string      `json:"status,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	RiskLevel  string      `json:"riskLevel,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

type SourceExplanation struct {
	Source  string  `json:"source,omitempty"`
	Title   string  `json:"title,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

type ContextExplanation struct {
	LowMemory       bool                     `json:"lowMemory,omitempty"`
	Budget          ContextBudget            `json:"budget,omitempty"`
	Usage           ContextUsage             `json:"usage,omitempty"`
	ItemsConsidered int                      `json:"itemsConsidered,omitempty"`
	ItemsSelected   int                      `json:"itemsSelected,omitempty"`
	ItemsDropped    int                      `json:"itemsDropped,omitempty"`
	Compressed      bool                     `json:"compressed,omitempty"`
	Truncated       bool                     `json:"truncated,omitempty"`
	BySource        []ContextSourceUsage     `json:"bySource,omitempty"`
	Included        []ContextItemExplanation `json:"included,omitempty"`
	Dropped         []ContextItemExplanation `json:"dropped,omitempty"`
}

type ContextItemExplanation struct {
	Source         string `json:"source,omitempty"`
	Title          string `json:"title,omitempty"`
	Reason         string `json:"reason,omitempty"`
	Priority       int    `json:"priority,omitempty"`
	Required       bool   `json:"required,omitempty"`
	Compressed     bool   `json:"compressed,omitempty"`
	Truncated      bool   `json:"truncated,omitempty"`
	UsedBytes      int    `json:"usedBytes,omitempty"`
	OriginalBytes  int    `json:"originalBytes,omitempty"`
	OriginalTokens int    `json:"originalTokens,omitempty"`
}

type PermissionExplanation struct {
	ID                string `json:"id,omitempty"`
	ToolName          string `json:"toolName,omitempty"`
	Status            string `json:"status,omitempty"`
	RiskLevel         string `json:"riskLevel,omitempty"`
	Reason            string `json:"reason,omitempty"`
	PolicyExplanation string `json:"policyExplanation,omitempty"`
}

type VerificationExplanation struct {
	Status string   `json:"status,omitempty"`
	Checks []string `json:"checks,omitempty"`
	Notes  []string `json:"notes,omitempty"`
}

type ResultExplanation struct {
	Status        string      `json:"status,omitempty"`
	Summary       string      `json:"summary,omitempty"`
	OutputSummary string      `json:"outputSummary,omitempty"`
	Attributes    []Attribute `json:"attributes,omitempty"`
}

type CapabilityExplanation struct {
	Capability string `json:"capability,omitempty"`
	Action     string `json:"action,omitempty"`
}

func ExplainTrace(trace Trace) TraceExplanation {
	trace = redactTrace(trace.Normalize())
	out := TraceExplanation{
		Schema:          trace.Schema,
		ID:              trace.ID,
		RequestSummary:  compactDiagnosticText(trace.UserRequest, 180),
		Route:           explainRoute(trace.Route),
		ToolsConsidered: explainToolConsiderations(trace),
		Verification: VerificationExplanation{
			Status: compactDiagnosticText(trace.Verification.Status, 80),
			Checks: compactStringSlice(trace.Verification.Checks, 12, 160),
			Notes:  compactStringSlice(trace.Verification.Notes, 12, 160),
		},
		Result: ResultExplanation{
			Status:        compactDiagnosticText(trace.FinalResult.Status, 80),
			Summary:       compactDiagnosticText(trace.FinalResult.Summary, 240),
			OutputSummary: compactDiagnosticText(trace.FinalResult.OutputSummary, 320),
			Attributes:    compactAttributes(trace.FinalResult.Attributes),
		},
	}
	out.ToolsUsed = explainTools(trace.ToolsCalled)
	out.MemoryUsed = explainSources(trace.MemoryUsed)
	out.RAGDocsUsed = explainSources(trace.RAGDocsUsed)
	out.Permissions = explainPermissions(trace.PermissionsRequested)
	out.Context = explainContext(trace.Context)
	if trace.Route.ShouldGenerateExtension || strings.TrimSpace(trace.Route.Capability) != "" {
		action := "propose"
		if trace.Route.ShouldGenerateExtension {
			action = "generate_extension"
		}
		out.MissingCapability = &CapabilityExplanation{
			Capability: compactDiagnosticText(trace.Route.Capability, 120),
			Action:     action,
		}
	}
	out.DiagnosticWarnings = diagnosticWarnings(trace)
	return out
}

func explainRoute(route RouteSnapshot) RouteExplanation {
	return RouteExplanation{
		Category:      compactDiagnosticText(route.Category, 120),
		Intent:        compactDiagnosticText(route.Intent, 120),
		Domain:        compactDiagnosticText(route.Domain, 120),
		Target:        compactDiagnosticText(route.Target, 160),
		Capability:    compactDiagnosticText(route.Capability, 120),
		RiskLevel:     compactDiagnosticText(route.RiskLevel, 80),
		Confidence:    route.Confidence,
		Reasons:       compactStringSlice(route.Reasons, 12, 160),
		UsesTool:      route.ShouldUseTool,
		UsesInternet:  route.ShouldUseInternet,
		ReadsFiles:    route.ShouldReadFile,
		WritesFiles:   route.ShouldWriteFiles,
		NeedsApproval: route.ShouldAskApproval,
		NeedsClarify:  route.ShouldAskClarification,
		GeneratesTool: route.ShouldGenerateExtension,
		CreatesJob:    route.ShouldCreateSchedulerJob,
	}
}

func explainToolConsiderations(trace Trace) []ToolConsiderationInfo {
	if len(trace.ToolsConsidered) == 0 {
		out := make([]ToolConsiderationInfo, 0, len(trace.Plan.ToolsNeeded))
		for _, name := range compactStringSlice(trace.Plan.ToolsNeeded, 24, 120) {
			out = append(out, ToolConsiderationInfo{Name: name, Source: "plan", Status: "planned"})
		}
		return out
	}
	out := make([]ToolConsiderationInfo, 0, len(trace.ToolsConsidered))
	for _, item := range trace.ToolsConsidered {
		out = append(out, ToolConsiderationInfo{
			Name:       compactDiagnosticText(item.Name, 120),
			Source:     compactDiagnosticText(item.Source, 120),
			Status:     compactDiagnosticText(item.Status, 80),
			Reason:     compactDiagnosticText(item.Reason, 220),
			RiskLevel:  compactDiagnosticText(item.RiskLevel, 80),
			Attributes: compactAttributes(item.Attributes),
		})
	}
	return out
}

func explainTools(calls []ToolCall) []ToolExplanation {
	out := make([]ToolExplanation, 0, len(calls))
	for _, call := range calls {
		out = append(out, ToolExplanation{
			Name:          compactDiagnosticText(call.Name, 120),
			Status:        compactDiagnosticText(call.Status, 80),
			RiskLevel:     compactDiagnosticText(call.RiskLevel, 80),
			InputSummary:  compactDiagnosticText(call.InputSummary, 240),
			OutputSummary: compactDiagnosticText(call.OutputSummary, 240),
			Error:         compactDiagnosticText(call.Error, 240),
		})
	}
	return out
}

func explainSources(refs []DocumentRef) []SourceExplanation {
	out := make([]SourceExplanation, 0, len(refs))
	for _, ref := range refs {
		out = append(out, SourceExplanation{
			Source:  compactDiagnosticText(ref.Source, 120),
			Title:   compactDiagnosticText(ref.Title, 160),
			Snippet: compactDiagnosticText(ref.Snippet, 220),
			Score:   ref.Score,
		})
	}
	return out
}

func explainPermissions(requests []PermissionRequest) []PermissionExplanation {
	out := make([]PermissionExplanation, 0, len(requests))
	for _, request := range requests {
		out = append(out, PermissionExplanation{
			ID:                compactDiagnosticText(request.ID, 120),
			ToolName:          compactDiagnosticText(request.ToolName, 120),
			Status:            compactDiagnosticText(request.Status, 80),
			RiskLevel:         compactDiagnosticText(request.RiskLevel, 80),
			Reason:            compactDiagnosticText(request.Reason, 240),
			PolicyExplanation: compactDiagnosticText(request.PolicyExplanation, 240),
		})
	}
	return out
}

func explainContext(context *ContextSnapshot) *ContextExplanation {
	if context == nil || context.Empty() {
		return nil
	}
	out := &ContextExplanation{
		LowMemory:       context.LowMemory,
		Budget:          context.Budget,
		Usage:           context.Usage,
		ItemsConsidered: context.ItemsConsidered,
		ItemsSelected:   context.ItemsSelected,
		ItemsDropped:    context.ItemsDropped,
		Compressed:      context.Compressed,
		Truncated:       context.Truncated,
		BySource:        context.BySource,
	}
	for _, item := range context.Included {
		out.Included = append(out.Included, explainContextItem(item))
	}
	for _, item := range context.Dropped {
		out.Dropped = append(out.Dropped, explainContextItem(item))
	}
	return out
}

func explainContextItem(item ContextItem) ContextItemExplanation {
	return ContextItemExplanation{
		Source:         compactDiagnosticText(item.Source, 120),
		Title:          compactDiagnosticText(item.Title, 160),
		Reason:         compactDiagnosticText(item.Reason, 220),
		Priority:       item.Priority,
		Required:       item.Required,
		Compressed:     item.Compressed,
		Truncated:      item.Truncated,
		UsedBytes:      item.UsedBytes,
		OriginalBytes:  item.OriginalBytes,
		OriginalTokens: item.OriginalTokens,
	}
}

func diagnosticWarnings(trace Trace) []string {
	var warnings []string
	if trace.Context != nil && trace.Context.Truncated {
		warnings = append(warnings, "context_truncated")
	}
	if trace.Context != nil && trace.Context.ItemsDropped > 0 {
		warnings = append(warnings, fmt.Sprintf("context_items_dropped:%d", trace.Context.ItemsDropped))
	}
	if trace.Route.ShouldAskApproval && len(trace.PermissionsRequested) == 0 {
		warnings = append(warnings, "route_required_approval_without_permission_record")
	}
	for _, call := range trace.ToolsCalled {
		if StatusFailed(call.Status) {
			warnings = append(warnings, "tool_failed:"+compactDiagnosticText(call.Name, 80))
		}
	}
	return warnings
}

func compactAttributes(values []Attribute) []Attribute {
	out := make([]Attribute, 0, len(values))
	for _, value := range values {
		out = append(out, Attribute{
			Key:   compactDiagnosticText(value.Key, 80),
			Value: compactDiagnosticText(value.Value, 180),
		})
	}
	return out
}

func compactStringSlice(values []string, maxItems int, maxChars int) []string {
	if maxItems <= 0 {
		maxItems = len(values)
	}
	out := make([]string, 0, minInt(len(values), maxItems))
	for i, value := range values {
		if i >= maxItems {
			out = append(out, fmt.Sprintf("... %d more", len(values)-maxItems))
			break
		}
		if cleaned := compactDiagnosticText(value, maxChars); cleaned != "" {
			out = append(out, cleaned)
		}
	}
	return out
}

func compactDiagnosticText(value string, maxChars int) string {
	value = strings.Join(strings.Fields(redactSecretText(value)), " ")
	if maxChars <= 0 || len(value) <= maxChars {
		return value
	}
	if maxChars <= 3 {
		return value[:maxChars]
	}
	return strings.TrimSpace(value[:maxChars-3]) + "..."
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}
