package agent

import (
	"strconv"
	"strings"

	contextcore "yemaka/internal/context"
	promptcore "yemaka/internal/prompt"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
)

type ReplaySaver interface {
	Save(replay.Trace) (replay.TraceFile, error)
}

type agentReplayTraceInput struct {
	ConversationID     string
	UserMessageID      string
	AssistantMessageID string
	Input              PlanInput
	Plan               Plan
	Decision           ExecutionDecision
	Result             *ExecutionResult
	ExtraTraces        []executionTrace
	EditProposal       *EditProposal
	AssistantContent   string
	Verification       VerificationResult
	PromptContext      contextcore.Result
	PromptOptions      promptcore.Options
	ModelResponse      ModelResponseTrace
}

func ReplayTraceFromPlan(input PlanInput, plan Plan) replay.Trace {
	trace := replay.NewTrace(input.Content)
	trace.Route = replay.RouteSnapshot{
		Category:                 plan.RouteCategory,
		Intent:                   plan.RouteIntent,
		Domain:                   plan.RouteDomain,
		Target:                   plan.RouteTarget,
		Capability:               plan.RouteCapability,
		RiskLevel:                plan.RiskLevel,
		Confidence:               plan.RouteConfidence,
		Reasons:                  replayStringSlice(plan.RouteReasons),
		ShouldUseTool:            plan.TaskType == TaskTool && len(plan.ToolsNeeded) > 0,
		ShouldUseInternet:        plan.RouteUsesInternet,
		ShouldReadFile:           plan.RouteReadsFiles,
		ShouldWriteFiles:         plan.RouteWritesFiles,
		ShouldAskApproval:        plan.RouteRequiresApproval,
		ShouldAskClarification:   plan.NeedsClarification,
		ShouldGenerateExtension:  plan.RouteGeneratesExtension,
		ShouldCreateSchedulerJob: plan.RouteCreatesSchedulerJob,
	}
	trace.Plan = replay.PlanSnapshot{
		Goal:         plan.Goal,
		Assumptions:  replayStringSlice(plan.Assumptions),
		Steps:        replayStringSlice(plan.Steps),
		ToolsNeeded:  replayStringSlice(plan.ToolsNeeded),
		FilesNeeded:  replayStringSlice(plan.FilesNeeded),
		Verification: replayStringSlice(plan.Verification),
		MaxSteps:     plan.MaxSteps,
	}
	trace.MemoryUsed = replayDocumentRefs("memory", input.MemorySources)
	trace.RAGDocsUsed = replayDocumentRefs(firstNonEmptyString(input.SourceKind, "rag"), input.Sources)
	knowledgeSources := []string{}
	if planAllowsKnowledgeContext(plan) {
		knowledgeSources = input.KnowledgeSources
	}
	knowledgeRefs := replayDocumentRefs("knowledge", knowledgeSources)
	for _, tool := range replayStringSlice(plan.ToolsNeeded) {
		trace.AppendToolConsideration(replay.ToolConsideration{
			Name:      tool,
			Source:    "plan",
			Status:    "planned",
			Reason:    "tool selected by deterministic route plan",
			RiskLevel: plan.RiskLevel,
		})
	}
	trace.Attributes = []replay.Attribute{
		{Key: "task_type", Value: plan.TaskType},
		{Key: "model_task", Value: plan.ModelTask},
		{Key: "route_uses_workspace", Value: strconv.FormatBool(plan.RouteUsesWorkspace)},
		{Key: "route_uses_rag", Value: strconv.FormatBool(plan.RouteUsesRAG)},
		{Key: "route_connector_action", Value: strconv.FormatBool(plan.RouteConnectorAction)},
		{Key: "route_crawler_task", Value: strconv.FormatBool(plan.RouteCrawlerTask)},
		{Key: "route_runs_shell", Value: strconv.FormatBool(plan.RouteRunsShell)},
		{Key: "route_lane", Value: plan.RouteLane},
		{Key: "evidence_policy", Value: plan.EvidencePolicy},
		{Key: "evidence_source", Value: plan.EvidenceSource},
		{Key: "response_contract", Value: plan.ResponseContract},
		{Key: "continuation_mode", Value: plan.ContinuationMode},
		{Key: "ambiguity_flags", Value: strings.Join(plan.AmbiguityFlags, ",")},
		{Key: "knowledge_sources", Value: strings.Join(knowledgeSources, ",")},
		{Key: "message_rewrite", Value: strconv.FormatBool(plan.MessageFrame.IsRewriteLike)},
		{Key: "message_route_teaching", Value: strconv.FormatBool(plan.MessageFrame.IsRouteTeachingLike)},
		{Key: "message_apply_like", Value: strconv.FormatBool(plan.MessageFrame.IsApplyLike)},
		{Key: "message_last_response_artifact", Value: strconv.FormatBool(plan.MessageFrame.IsLastResponseArtifactAction)},
		{Key: "message_requested_action", Value: plan.MessageFrame.RequestedAction},
		{Key: "message_target_kind", Value: plan.MessageFrame.TargetKind},
	}
	if len(knowledgeRefs) > 0 {
		trace.MemoryUsed = append(trace.MemoryUsed, knowledgeRefs...)
	}
	if plan.RouteSession != nil {
		trace.Attributes = append(trace.Attributes,
			replay.Attribute{Key: "session_active_route", Value: plan.RouteSession.ActiveRoute},
			replay.Attribute{Key: "session_task_status", Value: plan.RouteSession.TaskStatus},
			replay.Attribute{Key: "session_pending_clarification", Value: plan.RouteSession.PendingClarification},
			replay.Attribute{Key: "session_pending_approval", Value: plan.RouteSession.PendingApproval},
			replay.Attribute{Key: "session_pending_operation_id", Value: plan.RouteSession.PendingOperationID},
			replay.Attribute{Key: "session_pending_operation_type", Value: plan.RouteSession.PendingOperationType},
			replay.Attribute{Key: "session_pending_operation_target", Value: plan.RouteSession.PendingOperationTarget},
			replay.Attribute{Key: "session_route_lock_strength", Value: plan.RouteSession.RouteLockStrength},
			replay.Attribute{Key: "session_last_outcome", Value: plan.RouteSession.LastOutcome},
			replay.Attribute{Key: "session_failure_reason", Value: plan.RouteSession.FailureReason},
		)
		trace.Attributes = append(trace.Attributes, routeRecoveryReplayAttributes("session_last_", plan.RouteSession.LastRecovery)...)
	}
	for index, candidate := range plan.RouteCandidates {
		if index >= 5 {
			break
		}
		trace.Attributes = append(trace.Attributes,
			replay.Attribute{Key: "route_candidate_" + strconv.Itoa(index+1), Value: candidate.Route},
			replay.Attribute{Key: "route_candidate_" + strconv.Itoa(index+1) + "_source", Value: candidate.Source},
			replay.Attribute{Key: "route_candidate_" + strconv.Itoa(index+1) + "_confidence", Value: strconv.Itoa(candidate.Confidence)},
			replay.Attribute{Key: "route_candidate_" + strconv.Itoa(index+1) + "_lane", Value: candidate.ToolLane},
			replay.Attribute{Key: "route_candidate_" + strconv.Itoa(index+1) + "_required_state", Value: strings.Join(candidate.RequiredState, ",")},
			replay.Attribute{Key: "route_candidate_" + strconv.Itoa(index+1) + "_ambiguity_flags", Value: strings.Join(candidate.AmbiguityFlags, ",")},
			replay.Attribute{Key: "route_candidate_" + strconv.Itoa(index+1) + "_suppressed_reason", Value: candidate.SuppressedReason},
		)
	}
	if plan.RoutePreflight != nil {
		trace.Attributes = append(trace.Attributes,
			replay.Attribute{Key: "route_preflight_source_of_truth", Value: plan.RoutePreflight.SourceOfTruth},
			replay.Attribute{Key: "route_preflight_freshness_risk", Value: plan.RoutePreflight.FreshnessRisk},
			replay.Attribute{Key: "route_preflight_target_source", Value: plan.RoutePreflight.TargetSource},
			replay.Attribute{Key: "route_preflight_selected_route", Value: plan.RoutePreflight.SelectedRoute},
			replay.Attribute{Key: "route_preflight_target", Value: plan.RoutePreflight.Target},
			replay.Attribute{Key: "route_preflight_required_tool", Value: plan.RoutePreflight.RequiredTool},
			replay.Attribute{Key: "route_preflight_blocked_requirement", Value: plan.RoutePreflight.BlockedRequirement},
			replay.Attribute{Key: "route_preflight_stop_reason", Value: plan.RoutePreflight.StopReason},
			replay.Attribute{Key: "route_preflight_missing_slots", Value: strings.Join(plan.RoutePreflight.MissingSlots, ",")},
			replay.Attribute{Key: "route_preflight_correction_ids", Value: strings.Join(plan.RoutePreflight.AppliedCorrectionIDs, ",")},
		)
	}
	if plan.RouteContinuation != nil {
		trace.Attributes = append(trace.Attributes,
			replay.Attribute{Key: "route_continuation_kind", Value: plan.RouteContinuation.Kind},
			replay.Attribute{Key: "route_continuation_target_source", Value: plan.RouteContinuation.TargetSource},
			replay.Attribute{Key: "route_continuation_prior_route", Value: plan.RouteContinuation.PriorRouteCategory},
			replay.Attribute{Key: "route_continuation_prior_tool", Value: plan.RouteContinuation.PriorToolName},
			replay.Attribute{Key: "route_continuation_prior_target", Value: plan.RouteContinuation.PriorTarget},
			replay.Attribute{Key: "route_continuation_prior_source", Value: plan.RouteContinuation.PriorSourceOfTruth},
			replay.Attribute{Key: "route_continuation_prior_failure", Value: plan.RouteContinuation.PriorFailureStatus},
			replay.Attribute{Key: "route_continuation_new_target", Value: plan.RouteContinuation.NewTarget},
			replay.Attribute{Key: "route_continuation_new_source", Value: plan.RouteContinuation.NewSourceOfTruth},
			replay.Attribute{Key: "route_continuation_missing_slots", Value: strings.Join(plan.RouteContinuation.MissingSlots, ",")},
		)
	}
	return trace.Normalize()
}

func replayTraceFromAgentRun(input agentReplayTraceInput) replay.Trace {
	trace := ReplayTraceFromPlan(input.Input, input.Plan)
	trace.ID = replayTraceID(input.ConversationID, input.UserMessageID, input.AssistantMessageID)
	trace.Context = replayContextSnapshot(input.PromptContext, input.PromptOptions)
	for _, execution := range input.ExtraTraces {
		appendReplayExecution(&trace, execution.Decision, execution.Result)
	}
	appendReplayExecution(&trace, input.Decision, input.Result)
	trace.Attributes = append(trace.Attributes, routeRecoveryReplayAttributes("", RouteRecoveryForExecution(input.Plan, input.Decision, input.Result, nil))...)
	if strings.TrimSpace(input.ModelResponse.EffectiveMode) != "" {
		trace.Attributes = append(trace.Attributes,
			replay.Attribute{Key: "response_mode", Value: input.ModelResponse.EffectiveMode},
			replay.Attribute{Key: "response_mode_configured", Value: input.ModelResponse.ConfiguredMode},
			replay.Attribute{Key: "thinking_requested", Value: input.ModelResponse.ThinkingRequested},
			replay.Attribute{Key: "thinking_returned", Value: strconv.FormatBool(input.ModelResponse.ThinkingReturned)},
			replay.Attribute{Key: "show_thinking_trace", Value: strconv.FormatBool(input.ModelResponse.ShowThinkingTrace)},
			replay.Attribute{Key: "num_predict", Value: strconv.Itoa(input.ModelResponse.NumPredict)},
			replay.Attribute{Key: "num_ctx", Value: strconv.Itoa(input.ModelResponse.NumCtx)},
			replay.Attribute{Key: "thinking_unsupported", Value: strconv.FormatBool(input.ModelResponse.UnsupportedThinking)},
		)
	}
	trace.AppendFinalResult(replayFinalResult(input.Decision, input.Result, input.AssistantContent))
	trace.AppendVerification(replayVerification(input.Verification))
	trace.Attributes = append(trace.Attributes,
		replay.Attribute{Key: "conversation_id", Value: input.ConversationID},
		replay.Attribute{Key: "user_message_id", Value: input.UserMessageID},
		replay.Attribute{Key: "assistant_message_id", Value: input.AssistantMessageID},
	)
	if input.EditProposal != nil {
		trace.Attributes = append(trace.Attributes,
			replay.Attribute{Key: "edit_proposal_status", Value: input.EditProposal.Status},
			replay.Attribute{Key: "edit_proposal_path", Value: input.EditProposal.Path},
			replay.Attribute{Key: "edit_proposal_snapshot_before_write", Value: strconv.FormatBool(input.EditProposal.SnapshotBeforeWrite)},
		)
	}
	return sanitizeReplayTrace(trace.Normalize()).Normalize()
}

func routeRecoveryReplayAttributes(prefix string, recovery routing.RouteRecoveryDecision) []replay.Attribute {
	if recovery.IsZero() {
		return nil
	}
	key := func(name string) string { return prefix + "route_recovery_" + name }
	return []replay.Attribute{
		{Key: key("trigger"), Value: recovery.Trigger},
		{Key: key("previous_route"), Value: recovery.PreviousRoute},
		{Key: key("proposed_route"), Value: recovery.ProposedRoute},
		{Key: key("reason"), Value: recovery.Reason},
		{Key: key("confidence"), Value: strconv.Itoa(recovery.Confidence)},
		{Key: key("safe_to_continue"), Value: strconv.FormatBool(recovery.SafeToContinue)},
		{Key: key("requires_clarification"), Value: strconv.FormatBool(recovery.RequiresClarification)},
		{Key: key("requires_approval"), Value: strconv.FormatBool(recovery.RequiresApproval)},
		{Key: key("requires_configuration"), Value: strconv.FormatBool(recovery.RequiresConfiguration)},
		{Key: key("final_action"), Value: recovery.FinalAction},
	}
}

func appendReplayExecution(trace *replay.Trace, decision ExecutionDecision, result *ExecutionResult) {
	if trace == nil {
		return
	}
	if strings.TrimSpace(decision.ToolName) != "" {
		trace.AppendToolConsideration(toolConsiderationFromExecution(decision, result))
	}
	if result != nil && strings.TrimSpace(decision.ToolName) != "" {
		trace.AppendToolCall(toolCallFromExecution(decision, result, nil))
	}
	if permission, ok := permissionRequestFromExecution(decision); ok {
		trace.AppendPermissionRequest(permission)
	}
	if errTrace, ok := replayErrorFromExecution(decision, result); ok {
		trace.AppendError(errTrace)
	}
}

func replayErrorFromExecution(decision ExecutionDecision, result *ExecutionResult) (replay.TraceError, bool) {
	if decision.Status == ExecutionNeedsConfirmation {
		return replay.TraceError{}, false
	}
	if result != nil && replay.StatusFailed(result.Status) {
		message := firstNonEmptyString(result.Context, result.Status)
		return replay.TraceError{
			Stage:       failureStageFromDecision(decision),
			Code:        failureCode(failureStageFromDecision(decision), decision, result, nil),
			Subject:     decision.ToolName,
			Message:     message,
			Recoverable: true,
		}, true
	}
	if replay.StatusFailed(decision.Status) {
		stage := failureStageFromDecision(decision)
		return replay.TraceError{
			Stage:       stage,
			Code:        failureCode(stage, decision, result, nil),
			Subject:     decision.ToolName,
			Message:     decision.Reason,
			Recoverable: true,
		}, true
	}
	return replay.TraceError{}, false
}

func toolConsiderationFromExecution(decision ExecutionDecision, result *ExecutionResult) replay.ToolConsideration {
	status := strings.TrimSpace(decision.Status)
	if result != nil {
		status = firstNonEmptyString(result.Status, status)
		if replay.StatusSuccessful(status) {
			status = "used"
		}
	}
	attributes := []replay.Attribute{
		{Key: "decision_status", Value: decision.Status},
		{Key: "policy_explanation", Value: decision.PolicyExplanation},
	}
	if decision.PolicyLevel > 0 {
		attributes = append(attributes, replay.Attribute{Key: "policy_level", Value: strconv.Itoa(decision.PolicyLevel)})
	}
	if result != nil {
		attributes = append(attributes,
			replay.Attribute{Key: "result_status", Value: result.Status},
			replay.Attribute{Key: "result_source_kind", Value: result.SourceKind},
		)
	}
	return replay.ToolConsideration{
		Name:       decision.ToolName,
		Source:     "execution_decision",
		Status:     status,
		Reason:     decision.Reason,
		RiskLevel:  decision.RiskLevel,
		Attributes: attributes,
	}
}

func replayFinalResult(decision ExecutionDecision, result *ExecutionResult, assistantContent string) replay.ResultSnapshot {
	if result != nil {
		status := result.Status
		if strings.TrimSpace(status) == "" {
			status = "completed"
		}
		return replay.ResultSnapshot{
			Status:        status,
			Summary:       replaySummary(firstNonEmptyString(resultSummary(result, nil), assistantContent)),
			OutputSummary: replaySummary(firstNonEmptyString(result.Context, assistantContent)),
			Attributes: []replay.Attribute{
				{Key: "source_kind", Value: result.SourceKind},
				{Key: "sources", Value: strings.Join(result.Sources, ", ")},
			},
		}
	}
	status := "completed"
	summary := assistantContent
	switch decision.Status {
	case ExecutionBlocked, ExecutionNeedsConfirmation:
		status = decision.Status
		summary = firstNonEmptyString(decision.Reason, assistantContent)
	case ExecutionReady:
		if strings.TrimSpace(decision.ToolName) != "" {
			status = "completed"
		}
	case ExecutionNotRequired, ExecutionAlreadyProvided, "":
		status = "completed"
	default:
		status = decision.Status
	}
	return replay.ResultSnapshot{
		Status:        status,
		Summary:       replaySummary(summary),
		OutputSummary: replaySummary(assistantContent),
	}
}

func replayVerification(verification VerificationResult) replay.VerificationResult {
	return replay.VerificationResult{
		Status: verification.Status,
		Checks: replayStringSlice(verification.Checks),
		Notes:  replayStringSlice(verification.Reasons),
	}
}

func replayContextSnapshot(result contextcore.Result, options promptcore.Options) *replay.ContextSnapshot {
	if result.Request.ID == "" && len(result.Items) == 0 && len(result.Dropped) == 0 && result.Stats.UsedBytes == 0 {
		return nil
	}
	budget := promptContextBudget(options)
	snapshot := replay.ContextSnapshot{
		LowMemory: options.LowMemory,
		Budget: replay.ContextBudget{
			MaxBytes:      budget.MaxBytes,
			MaxTokens:     budget.MaxTokens,
			MinItemBytes:  budget.MinItemBytes,
			MinItemTokens: budget.MinItemTokens,
		},
		Usage: replay.ContextUsage{
			UsedBytes:      result.Stats.UsedBytes,
			UsedTokens:     result.Stats.UsedTokens,
			OriginalBytes:  result.Stats.OriginalBytes,
			OriginalTokens: result.Stats.OriginalTokens,
		},
		ItemsConsidered: result.Stats.ItemsConsidered,
		ItemsSelected:   result.Stats.ItemsSelected,
		ItemsDropped:    result.Stats.ItemsDropped,
		Compressed:      result.Stats.Compressed,
		Truncated:       result.Stats.Truncated,
	}
	for source, usage := range result.Stats.BySource {
		snapshot.BySource = append(snapshot.BySource, replay.ContextSourceUsage{
			Source: string(source),
			Bytes:  usage.Bytes,
			Tokens: usage.Tokens,
		})
	}
	for _, item := range result.Items {
		snapshot.Included = append(snapshot.Included, replay.ContextItem{
			ID:             item.ID,
			Source:         string(item.Source),
			Title:          item.Title,
			Reason:         replayContextIncludedReason(item),
			Relevance:      item.Relevance,
			Priority:       item.Priority,
			Required:       item.Required,
			Compressed:     item.Compressed,
			Truncated:      item.Truncated,
			UsedBytes:      item.UsedBytes,
			UsedTokens:     item.UsedTokens,
			OriginalBytes:  item.OriginalBytes,
			OriginalTokens: item.OriginalTokens,
		})
	}
	for _, item := range result.Dropped {
		snapshot.Dropped = append(snapshot.Dropped, replay.ContextItem{
			ID:             item.ID,
			Source:         string(item.Source),
			Title:          item.Title,
			Reason:         item.Reason,
			Relevance:      item.Relevance,
			Priority:       item.Priority,
			Required:       item.Required,
			OriginalBytes:  item.OriginalBytes,
			OriginalTokens: item.OriginalTokens,
		})
	}
	normalized := snapshot.Normalize()
	if normalized.Empty() {
		return nil
	}
	return &normalized
}

func replayContextIncludedReason(item contextcore.CompiledItem) string {
	switch {
	case item.Required:
		return "required"
	case item.Compressed || item.Truncated:
		return "selected_reduced_to_budget"
	default:
		return "selected_by_priority_relevance"
	}
}

func replayTraceID(parts ...string) string {
	cleaned := []string{"agent"}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	if len(cleaned) == 1 {
		return ""
	}
	return strings.Join(cleaned, "-")
}

func replayDocumentRefs(source string, values []string) []replay.DocumentRef {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "context"
	}
	refs := make([]replay.DocumentRef, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		refs = append(refs, replay.DocumentRef{Source: source, Title: value})
	}
	return refs
}

func replaySummary(value string) string {
	value = strings.TrimSpace(value)
	const limit = 2000
	if len(value) <= limit {
		return value
	}
	return strings.TrimSpace(value[:limit]) + "..."
}

func replayStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}
