package replay

import (
	"fmt"
	"sort"
	"strings"
)

const TraceSchema = "yemaka.replay.trace.v1"

type Trace struct {
	Schema               string              `json:"schema"`
	ID                   string              `json:"id,omitempty"`
	UserRequest          string              `json:"userRequest"`
	Route                RouteSnapshot       `json:"route"`
	Plan                 PlanSnapshot        `json:"plan"`
	Context              *ContextSnapshot    `json:"context,omitempty"`
	MemoryUsed           []DocumentRef       `json:"memoryUsed,omitempty"`
	RAGDocsUsed          []DocumentRef       `json:"ragDocsUsed,omitempty"`
	ToolsConsidered      []ToolConsideration `json:"toolsConsidered,omitempty"`
	ToolsCalled          []ToolCall          `json:"toolsCalled,omitempty"`
	PermissionsRequested []PermissionRequest `json:"permissionsRequested,omitempty"`
	Errors               []TraceError        `json:"errors,omitempty"`
	FinalResult          ResultSnapshot      `json:"finalResult,omitempty"`
	Verification         VerificationResult  `json:"verification,omitempty"`
	LearningSaved        []LearningArtifact  `json:"learningSaved,omitempty"`
	Attributes           []Attribute         `json:"attributes,omitempty"`
}

type RouteSnapshot struct {
	Category                 string   `json:"category,omitempty"`
	Intent                   string   `json:"intent,omitempty"`
	Domain                   string   `json:"domain,omitempty"`
	Target                   string   `json:"target,omitempty"`
	Capability               string   `json:"capability,omitempty"`
	RiskLevel                string   `json:"riskLevel,omitempty"`
	Confidence               int      `json:"confidence,omitempty"`
	Reasons                  []string `json:"reasons,omitempty"`
	ShouldUseTool            bool     `json:"shouldUseTool,omitempty"`
	ShouldUseInternet        bool     `json:"shouldUseInternet,omitempty"`
	ShouldReadFile           bool     `json:"shouldReadFile,omitempty"`
	ShouldWriteFiles         bool     `json:"shouldWriteFiles,omitempty"`
	ShouldAskApproval        bool     `json:"shouldAskApproval,omitempty"`
	ShouldAskClarification   bool     `json:"shouldAskClarification,omitempty"`
	ShouldGenerateExtension  bool     `json:"shouldGenerateExtension,omitempty"`
	ShouldCreateSchedulerJob bool     `json:"shouldCreateSchedulerJob,omitempty"`
}

type PlanSnapshot struct {
	Goal         string   `json:"goal,omitempty"`
	Assumptions  []string `json:"assumptions,omitempty"`
	Steps        []string `json:"steps,omitempty"`
	ToolsNeeded  []string `json:"toolsNeeded,omitempty"`
	FilesNeeded  []string `json:"filesNeeded,omitempty"`
	Verification []string `json:"verification,omitempty"`
	MaxSteps     int      `json:"maxSteps,omitempty"`
}

type ContextSnapshot struct {
	LowMemory       bool                 `json:"lowMemory,omitempty"`
	Budget          ContextBudget        `json:"budget,omitempty"`
	Usage           ContextUsage         `json:"usage,omitempty"`
	ItemsConsidered int                  `json:"itemsConsidered,omitempty"`
	ItemsSelected   int                  `json:"itemsSelected,omitempty"`
	ItemsDropped    int                  `json:"itemsDropped,omitempty"`
	Compressed      bool                 `json:"compressed,omitempty"`
	Truncated       bool                 `json:"truncated,omitempty"`
	BySource        []ContextSourceUsage `json:"bySource,omitempty"`
	Included        []ContextItem        `json:"included,omitempty"`
	Dropped         []ContextItem        `json:"dropped,omitempty"`
}

type ContextBudget struct {
	MaxBytes      int `json:"maxBytes,omitempty"`
	MaxTokens     int `json:"maxTokens,omitempty"`
	MinItemBytes  int `json:"minItemBytes,omitempty"`
	MinItemTokens int `json:"minItemTokens,omitempty"`
}

type ContextUsage struct {
	UsedBytes      int `json:"usedBytes,omitempty"`
	UsedTokens     int `json:"usedTokens,omitempty"`
	OriginalBytes  int `json:"originalBytes,omitempty"`
	OriginalTokens int `json:"originalTokens,omitempty"`
}

type ContextSourceUsage struct {
	Source string `json:"source,omitempty"`
	Bytes  int    `json:"bytes,omitempty"`
	Tokens int    `json:"tokens,omitempty"`
}

type ContextItem struct {
	ID             string  `json:"id,omitempty"`
	Source         string  `json:"source,omitempty"`
	Title          string  `json:"title,omitempty"`
	Reason         string  `json:"reason,omitempty"`
	Relevance      float64 `json:"relevance,omitempty"`
	Priority       int     `json:"priority,omitempty"`
	Required       bool    `json:"required,omitempty"`
	Compressed     bool    `json:"compressed,omitempty"`
	Truncated      bool    `json:"truncated,omitempty"`
	UsedBytes      int     `json:"usedBytes,omitempty"`
	UsedTokens     int     `json:"usedTokens,omitempty"`
	OriginalBytes  int     `json:"originalBytes,omitempty"`
	OriginalTokens int     `json:"originalTokens,omitempty"`
}

type DocumentRef struct {
	ID      string  `json:"id,omitempty"`
	Source  string  `json:"source,omitempty"`
	Title   string  `json:"title,omitempty"`
	Snippet string  `json:"snippet,omitempty"`
	Score   float64 `json:"score,omitempty"`
}

type ToolConsideration struct {
	Name       string      `json:"name"`
	Source     string      `json:"source,omitempty"`
	Status     string      `json:"status,omitempty"`
	Reason     string      `json:"reason,omitempty"`
	RiskLevel  string      `json:"riskLevel,omitempty"`
	Attributes []Attribute `json:"attributes,omitempty"`
}

type ToolCall struct {
	Name          string      `json:"name"`
	InputSummary  string      `json:"inputSummary,omitempty"`
	OutputSummary string      `json:"outputSummary,omitempty"`
	Status        string      `json:"status,omitempty"`
	RiskLevel     string      `json:"riskLevel,omitempty"`
	StartedAt     string      `json:"startedAt,omitempty"`
	CompletedAt   string      `json:"completedAt,omitempty"`
	Error         string      `json:"error,omitempty"`
	Attributes    []Attribute `json:"attributes,omitempty"`
}

type PermissionRequest struct {
	ID                string      `json:"id,omitempty"`
	ToolName          string      `json:"toolName,omitempty"`
	Reason            string      `json:"reason,omitempty"`
	RiskLevel         string      `json:"riskLevel,omitempty"`
	Status            string      `json:"status,omitempty"`
	PolicyLevel       int         `json:"policyLevel,omitempty"`
	PolicyExplanation string      `json:"policyExplanation,omitempty"`
	Attributes        []Attribute `json:"attributes,omitempty"`
}

type TraceError struct {
	Stage             string      `json:"stage,omitempty"`
	Code              string      `json:"code,omitempty"`
	Subject           string      `json:"subject,omitempty"`
	Message           string      `json:"message,omitempty"`
	Recoverable       bool        `json:"recoverable,omitempty"`
	ExpectedRoute     string      `json:"expectedRoute,omitempty"`
	ExpectedRiskLevel string      `json:"expectedRiskLevel,omitempty"`
	Attributes        []Attribute `json:"attributes,omitempty"`
}

type ResultSnapshot struct {
	Status        string      `json:"status,omitempty"`
	Summary       string      `json:"summary,omitempty"`
	OutputSummary string      `json:"outputSummary,omitempty"`
	Attributes    []Attribute `json:"attributes,omitempty"`
}

type VerificationResult struct {
	Status string   `json:"status,omitempty"`
	Checks []string `json:"checks,omitempty"`
	Notes  []string `json:"notes,omitempty"`
}

type LearningArtifact struct {
	Kind    string `json:"kind,omitempty"`
	ID      string `json:"id,omitempty"`
	Source  string `json:"source,omitempty"`
	Summary string `json:"summary,omitempty"`
}

type Attribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewTrace(userRequest string) Trace {
	return Trace{
		Schema:      TraceSchema,
		UserRequest: strings.TrimSpace(userRequest),
	}
}

// AppendRouteSnapshot records the current route snapshot.
func (t *Trace) AppendRouteSnapshot(route RouteSnapshot) {
	if t == nil {
		return
	}
	t.Route = route
	*t = t.Normalize()
}

// AppendPlanSnapshot records the current plan snapshot.
func (t *Trace) AppendPlanSnapshot(plan PlanSnapshot) {
	if t == nil {
		return
	}
	t.Plan = plan
	*t = t.Normalize()
}

// AppendToolCall adds a tool execution snapshot.
func (t *Trace) AppendToolCall(tool ToolCall) {
	if t == nil {
		return
	}
	t.ToolsCalled = append(t.ToolsCalled, tool)
	*t = t.Normalize()
}

// AppendToolConsideration adds a tool planning or selection diagnostic.
func (t *Trace) AppendToolConsideration(tool ToolConsideration) {
	if t == nil {
		return
	}
	t.ToolsConsidered = append(t.ToolsConsidered, tool)
	*t = t.Normalize()
}

// AppendPermissionRequest adds a permission decision snapshot.
func (t *Trace) AppendPermissionRequest(permission PermissionRequest) {
	if t == nil {
		return
	}
	t.PermissionsRequested = append(t.PermissionsRequested, permission)
	*t = t.Normalize()
}

// AppendError adds a failure or warning snapshot.
func (t *Trace) AppendError(err TraceError) {
	if t == nil {
		return
	}
	t.Errors = append(t.Errors, err)
	*t = t.Normalize()
}

// AppendFinalResult records the latest final result snapshot.
func (t *Trace) AppendFinalResult(result ResultSnapshot) {
	if t == nil {
		return
	}
	t.FinalResult = result
	*t = t.Normalize()
}

// AppendVerification records the latest verification snapshot.
func (t *Trace) AppendVerification(verification VerificationResult) {
	if t == nil {
		return
	}
	t.Verification = verification
	*t = t.Normalize()
}

func (t Trace) Normalize() Trace {
	t.Schema = strings.TrimSpace(t.Schema)
	if t.Schema == "" {
		t.Schema = TraceSchema
	}
	t.ID = strings.TrimSpace(t.ID)
	t.UserRequest = strings.TrimSpace(t.UserRequest)
	t.Route = t.Route.Normalize()
	t.Plan = t.Plan.Normalize()
	if t.Context != nil {
		context := t.Context.Normalize()
		if context.Empty() {
			t.Context = nil
		} else {
			t.Context = &context
		}
	}
	t.MemoryUsed = cleanDocuments(t.MemoryUsed)
	t.RAGDocsUsed = cleanDocuments(t.RAGDocsUsed)
	t.ToolsConsidered = cleanToolConsiderations(t.ToolsConsidered)
	t.ToolsCalled = cleanTools(t.ToolsCalled)
	t.PermissionsRequested = cleanPermissions(t.PermissionsRequested)
	t.Errors = cleanErrors(t.Errors)
	t.FinalResult = t.FinalResult.Normalize()
	t.Verification = t.Verification.Normalize()
	t.LearningSaved = cleanLearning(t.LearningSaved)
	t.Attributes = cleanAttributes(t.Attributes)
	return t
}

func (r RouteSnapshot) Normalize() RouteSnapshot {
	r.Category = strings.TrimSpace(r.Category)
	r.Intent = strings.TrimSpace(r.Intent)
	r.Domain = strings.TrimSpace(r.Domain)
	r.Target = strings.TrimSpace(r.Target)
	r.Capability = strings.TrimSpace(r.Capability)
	r.RiskLevel = strings.TrimSpace(r.RiskLevel)
	if r.Confidence < 0 {
		r.Confidence = 0
	}
	if r.Confidence > 100 {
		r.Confidence = 100
	}
	r.Reasons = cleanStrings(r.Reasons)
	return r
}

func (p PlanSnapshot) Normalize() PlanSnapshot {
	p.Goal = strings.TrimSpace(p.Goal)
	p.Assumptions = cleanStrings(p.Assumptions)
	p.Steps = cleanStrings(p.Steps)
	p.ToolsNeeded = cleanStrings(p.ToolsNeeded)
	p.FilesNeeded = cleanStrings(p.FilesNeeded)
	p.Verification = cleanStrings(p.Verification)
	if p.MaxSteps < 0 {
		p.MaxSteps = 0
	}
	return p
}

func (c ContextSnapshot) Normalize() ContextSnapshot {
	c.Budget.MaxBytes = nonNegative(c.Budget.MaxBytes)
	c.Budget.MaxTokens = nonNegative(c.Budget.MaxTokens)
	c.Budget.MinItemBytes = nonNegative(c.Budget.MinItemBytes)
	c.Budget.MinItemTokens = nonNegative(c.Budget.MinItemTokens)
	c.Usage.UsedBytes = nonNegative(c.Usage.UsedBytes)
	c.Usage.UsedTokens = nonNegative(c.Usage.UsedTokens)
	c.Usage.OriginalBytes = nonNegative(c.Usage.OriginalBytes)
	c.Usage.OriginalTokens = nonNegative(c.Usage.OriginalTokens)
	c.ItemsConsidered = nonNegative(c.ItemsConsidered)
	c.ItemsSelected = nonNegative(c.ItemsSelected)
	c.ItemsDropped = nonNegative(c.ItemsDropped)
	c.BySource = cleanContextSourceUsage(c.BySource)
	c.Included = cleanContextItems(c.Included)
	c.Dropped = cleanContextItems(c.Dropped)
	return c
}

func (c ContextSnapshot) Empty() bool {
	return !c.LowMemory &&
		c.Budget.MaxBytes == 0 &&
		c.Budget.MaxTokens == 0 &&
		c.Usage.UsedBytes == 0 &&
		c.Usage.UsedTokens == 0 &&
		c.ItemsConsidered == 0 &&
		c.ItemsSelected == 0 &&
		c.ItemsDropped == 0 &&
		len(c.BySource) == 0 &&
		len(c.Included) == 0 &&
		len(c.Dropped) == 0
}

func (r ResultSnapshot) Normalize() ResultSnapshot {
	r.Status = strings.TrimSpace(r.Status)
	r.Summary = strings.TrimSpace(r.Summary)
	r.OutputSummary = strings.TrimSpace(r.OutputSummary)
	r.Attributes = cleanAttributes(r.Attributes)
	return r
}

func (v VerificationResult) Normalize() VerificationResult {
	v.Status = strings.TrimSpace(v.Status)
	v.Checks = cleanStrings(v.Checks)
	v.Notes = cleanStrings(v.Notes)
	return v
}

func (t Trace) HasFailures() bool {
	t = t.Normalize()
	if len(t.Errors) > 0 || StatusFailed(t.FinalResult.Status) || StatusFailed(t.Verification.Status) {
		return true
	}
	for _, tool := range t.ToolsCalled {
		if StatusFailed(tool.Status) || strings.TrimSpace(tool.Error) != "" {
			return true
		}
	}
	for _, permission := range t.PermissionsRequested {
		if StatusFailed(permission.Status) {
			return true
		}
	}
	return false
}

func (t Trace) PrimaryFailure() (TraceError, bool) {
	t = t.Normalize()
	if len(t.Errors) > 0 {
		return t.Errors[0], true
	}
	for _, permission := range t.PermissionsRequested {
		if StatusFailed(permission.Status) {
			return TraceError{
				Stage:   "permission",
				Code:    statusCode("permission", permission.Status),
				Subject: permission.ToolName,
				Message: compactFailureMessage(permission.Reason, permission.PolicyExplanation),
			}, true
		}
	}
	for _, tool := range t.ToolsCalled {
		if StatusFailed(tool.Status) || strings.TrimSpace(tool.Error) != "" {
			return TraceError{
				Stage:   "tool",
				Code:    statusCode("tool", tool.Status),
				Subject: tool.Name,
				Message: compactFailureMessage(tool.Error, tool.OutputSummary),
			}, true
		}
	}
	if StatusFailed(t.Verification.Status) {
		return TraceError{
			Stage:   "verification",
			Code:    statusCode("verification", t.Verification.Status),
			Message: strings.Join(t.Verification.Notes, "; "),
		}, true
	}
	if StatusFailed(t.FinalResult.Status) {
		return TraceError{
			Stage:   "result",
			Code:    statusCode("result", t.FinalResult.Status),
			Message: compactFailureMessage(t.FinalResult.Summary, t.FinalResult.OutputSummary),
		}, true
	}
	return TraceError{}, false
}

func StatusFailed(status string) bool {
	switch normalizeStatus(status) {
	case "failed", "failure", "error", "errored", "blocked", "timeout", "timed_out", "denied", "rejected", "broken", "rollback_required", "needs_follow_up":
		return true
	default:
		return false
	}
}

func StatusSuccessful(status string) bool {
	switch normalizeStatus(status) {
	case "ok", "pass", "passed", "success", "succeeded", "completed", "complete":
		return true
	default:
		return false
	}
}

func cleanContextSourceUsage(values []ContextSourceUsage) []ContextSourceUsage {
	out := make([]ContextSourceUsage, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value.Source = strings.TrimSpace(value.Source)
		if value.Source == "" || seen[value.Source] {
			continue
		}
		value.Bytes = nonNegative(value.Bytes)
		value.Tokens = nonNegative(value.Tokens)
		seen[value.Source] = true
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Source < out[j].Source
	})
	return out
}

func cleanContextItems(values []ContextItem) []ContextItem {
	out := make([]ContextItem, 0, len(values))
	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.Source = strings.TrimSpace(value.Source)
		value.Title = strings.TrimSpace(value.Title)
		value.Reason = strings.TrimSpace(value.Reason)
		value.Priority = nonNegative(value.Priority)
		if value.Relevance < 0 {
			value.Relevance = 0
		}
		if value.Relevance > 1 {
			value.Relevance = 1
		}
		value.UsedBytes = nonNegative(value.UsedBytes)
		value.UsedTokens = nonNegative(value.UsedTokens)
		value.OriginalBytes = nonNegative(value.OriginalBytes)
		value.OriginalTokens = nonNegative(value.OriginalTokens)
		if value.ID == "" && value.Source == "" && value.Title == "" && value.Reason == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cleanDocuments(values []DocumentRef) []DocumentRef {
	out := make([]DocumentRef, 0, len(values))
	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.Source = strings.TrimSpace(value.Source)
		value.Title = strings.TrimSpace(value.Title)
		value.Snippet = strings.TrimSpace(value.Snippet)
		if value.ID == "" && value.Source == "" && value.Title == "" && value.Snippet == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cleanToolConsiderations(values []ToolConsideration) []ToolConsideration {
	out := make([]ToolConsideration, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		value.Source = strings.TrimSpace(value.Source)
		value.Status = strings.TrimSpace(value.Status)
		value.Reason = strings.TrimSpace(value.Reason)
		value.RiskLevel = strings.TrimSpace(value.RiskLevel)
		value.Attributes = cleanAttributes(value.Attributes)
		if value.Name == "" && value.Status == "" && value.Reason == "" {
			continue
		}
		key := strings.Join([]string{value.Name, value.Source, value.Status, value.Reason}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	return out
}

func cleanTools(values []ToolCall) []ToolCall {
	out := make([]ToolCall, 0, len(values))
	for _, value := range values {
		value.Name = strings.TrimSpace(value.Name)
		value.InputSummary = strings.TrimSpace(value.InputSummary)
		value.OutputSummary = strings.TrimSpace(value.OutputSummary)
		value.Status = strings.TrimSpace(value.Status)
		value.RiskLevel = strings.TrimSpace(value.RiskLevel)
		value.StartedAt = strings.TrimSpace(value.StartedAt)
		value.CompletedAt = strings.TrimSpace(value.CompletedAt)
		value.Error = strings.TrimSpace(value.Error)
		value.Attributes = cleanAttributes(value.Attributes)
		if value.Name == "" && value.Status == "" && value.Error == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cleanPermissions(values []PermissionRequest) []PermissionRequest {
	out := make([]PermissionRequest, 0, len(values))
	for _, value := range values {
		value.ID = strings.TrimSpace(value.ID)
		value.ToolName = strings.TrimSpace(value.ToolName)
		value.Reason = strings.TrimSpace(value.Reason)
		value.RiskLevel = strings.TrimSpace(value.RiskLevel)
		value.Status = strings.TrimSpace(value.Status)
		value.PolicyExplanation = strings.TrimSpace(value.PolicyExplanation)
		value.Attributes = cleanAttributes(value.Attributes)
		if value.ID == "" && value.ToolName == "" && value.Status == "" && value.Reason == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cleanErrors(values []TraceError) []TraceError {
	out := make([]TraceError, 0, len(values))
	for _, value := range values {
		value.Stage = strings.TrimSpace(value.Stage)
		value.Code = strings.TrimSpace(value.Code)
		value.Subject = strings.TrimSpace(value.Subject)
		value.Message = strings.TrimSpace(value.Message)
		value.ExpectedRoute = strings.TrimSpace(value.ExpectedRoute)
		value.ExpectedRiskLevel = strings.TrimSpace(value.ExpectedRiskLevel)
		value.Attributes = cleanAttributes(value.Attributes)
		if value.Stage == "" && value.Code == "" && value.Subject == "" && value.Message == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cleanLearning(values []LearningArtifact) []LearningArtifact {
	out := make([]LearningArtifact, 0, len(values))
	for _, value := range values {
		value.Kind = strings.TrimSpace(value.Kind)
		value.ID = strings.TrimSpace(value.ID)
		value.Source = strings.TrimSpace(value.Source)
		value.Summary = strings.TrimSpace(value.Summary)
		if value.Kind == "" && value.ID == "" && value.Source == "" && value.Summary == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func cleanAttributes(values []Attribute) []Attribute {
	out := make([]Attribute, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value.Key = strings.TrimSpace(value.Key)
		value.Value = strings.TrimSpace(value.Value)
		if value.Key == "" || value.Value == "" {
			continue
		}
		key := value.Key + "\x00" + value.Value
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, value)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Key == out[j].Key {
			return out[i].Value < out[j].Value
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
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

func normalizeStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	status = strings.ReplaceAll(status, "-", "_")
	status = strings.ReplaceAll(status, " ", "_")
	return status
}

func statusCode(prefix string, status string) string {
	status = normalizeStatus(status)
	if status == "" {
		status = "failed"
	}
	return fmt.Sprintf("%s_%s", prefix, status)
}

func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func compactFailureMessage(values ...string) string {
	for _, value := range values {
		value = strings.Join(strings.Fields(value), " ")
		if value != "" {
			return value
		}
	}
	return "failure captured in replay trace"
}
