package agent

import (
	"strings"

	"yemaka/internal/learning"
	"yemaka/internal/replay"
)

type ReplayFailureInput struct {
	Input                 PlanInput
	Plan                  Plan
	Decision              ExecutionDecision
	Result                *ExecutionResult
	Err                   error
	Stage                 string
	Code                  string
	ExpectedRoute         string
	ExpectedRiskLevel     string
	AdditionalTraceErrors []replay.TraceError
}

type RegressionArtifactRequest struct {
	Trace replay.Trace
	Cases []learning.RegressionTestCase
}

func ReplayTraceFromFailure(input ReplayFailureInput) replay.Trace {
	trace := ReplayTraceFromPlan(input.Input, input.Plan)
	trace.AppendToolCall(toolCallFromExecution(input.Decision, input.Result, input.Err))

	if permission, ok := permissionRequestFromExecution(input.Decision); ok {
		trace.AppendPermissionRequest(permission)
	}

	if errTrace, ok := traceErrorFromFailure(input); ok {
		trace.AppendError(errTrace)
	}
	for _, errTrace := range input.AdditionalTraceErrors {
		trace.AppendError(errTrace)
	}

	if input.Result != nil {
		trace.AppendFinalResult(replay.ResultSnapshot{
			Status:        input.Result.Status,
			Summary:       resultSummary(input.Result, input.Err),
			OutputSummary: input.Result.Context,
			Attributes: []replay.Attribute{
				{Key: "source_kind", Value: input.Result.SourceKind},
				{Key: "sources", Value: strings.Join(input.Result.Sources, ", ")},
			},
		})
	} else if input.Err != nil {
		trace.AppendFinalResult(replay.ResultSnapshot{
			Status:  "failed",
			Summary: input.Err.Error(),
		})
	}

	return sanitizeReplayTrace(trace.Normalize()).Normalize()
}

func RegressionArtifactRequestFromFailure(input ReplayFailureInput) (RegressionArtifactRequest, bool) {
	trace := ReplayTraceFromFailure(input)
	cases := learning.GenerateRegressionTestCases(trace)
	if len(cases) == 0 {
		return RegressionArtifactRequest{}, false
	}
	return RegressionArtifactRequest{Trace: trace, Cases: cases}, true
}

func SaveRegressionArtifactsFromFailure(outputDir string, input ReplayFailureInput) ([]learning.RegressionArtifact, error) {
	request, ok := RegressionArtifactRequestFromFailure(input)
	if !ok {
		return nil, nil
	}
	return learning.SaveRegressionArtifacts(outputDir, request.Cases)
}

func toolCallFromExecution(decision ExecutionDecision, result *ExecutionResult, err error) replay.ToolCall {
	status := strings.TrimSpace(decision.Status)
	if result != nil && strings.TrimSpace(result.Status) != "" {
		status = result.Status
	}
	if err != nil && !replay.StatusFailed(status) {
		status = "failed"
	}
	return replay.ToolCall{
		Name:          decision.ToolName,
		InputSummary:  strings.Join(decision.Command, " "),
		OutputSummary: executionOutputSummary(result),
		Status:        status,
		RiskLevel:     decision.RiskLevel,
		Error:         errorString(err),
		Attributes: []replay.Attribute{
			{Key: "decision_status", Value: decision.Status},
			{Key: "decision_reason", Value: decision.Reason},
			{Key: "policy_explanation", Value: decision.PolicyExplanation},
		},
	}
}

func permissionRequestFromExecution(decision ExecutionDecision) (replay.PermissionRequest, bool) {
	if !decision.RequiresConfirmation && decision.Status != ExecutionNeedsConfirmation && decision.Status != ExecutionBlocked {
		return replay.PermissionRequest{}, false
	}
	status := decision.Status
	if status == ExecutionNeedsConfirmation {
		status = "needs_confirmation"
	}
	return replay.PermissionRequest{
		ID:                decision.RequestID,
		ToolName:          decision.ToolName,
		Reason:            decision.Reason,
		RiskLevel:         decision.RiskLevel,
		Status:            status,
		PolicyLevel:       decision.PolicyLevel,
		PolicyExplanation: decision.PolicyExplanation,
	}, true
}

func traceErrorFromFailure(input ReplayFailureInput) (replay.TraceError, bool) {
	message := errorString(input.Err)
	if message == "" && input.Result != nil && replay.StatusFailed(input.Result.Status) {
		message = firstNonEmptyString(input.Result.Context, input.Result.Status)
	}
	if input.Stage == "" && input.Code == "" && message == "" && !replay.StatusFailed(input.Decision.Status) {
		return replay.TraceError{}, false
	}
	stage := firstNonEmptyString(input.Stage, failureStageFromDecision(input.Decision), "tool")
	code := firstNonEmptyString(input.Code, failureCode(stage, input.Decision, input.Result, input.Err))
	return replay.TraceError{
		Stage:             stage,
		Code:              code,
		Subject:           input.Decision.ToolName,
		Message:           message,
		Recoverable:       true,
		ExpectedRoute:     input.ExpectedRoute,
		ExpectedRiskLevel: input.ExpectedRiskLevel,
	}, true
}

func failureStageFromDecision(decision ExecutionDecision) string {
	if decision.Status == ExecutionBlocked || decision.Status == ExecutionNeedsConfirmation {
		return "permission"
	}
	return "tool"
}

func failureCode(stage string, decision ExecutionDecision, result *ExecutionResult, err error) string {
	if err != nil {
		return stage + "_error"
	}
	status := decision.Status
	if result != nil && strings.TrimSpace(result.Status) != "" {
		status = result.Status
	}
	status = strings.TrimSpace(strings.ReplaceAll(status, " ", "_"))
	if status == "" {
		status = "failed"
	}
	return stage + "_" + status
}

func resultSummary(result *ExecutionResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if result == nil {
		return ""
	}
	return firstNonEmptyString(result.Status, result.Context)
}

func executionOutputSummary(result *ExecutionResult) string {
	if result == nil {
		return ""
	}
	return result.Context
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func sanitizeReplayTrace(trace replay.Trace) replay.Trace {
	trace.ID = sanitizeReplayString(trace.ID)
	trace.UserRequest = sanitizeReplayString(trace.UserRequest)
	trace.Route.Category = sanitizeReplayString(trace.Route.Category)
	trace.Route.Intent = sanitizeReplayString(trace.Route.Intent)
	trace.Route.Domain = sanitizeReplayString(trace.Route.Domain)
	trace.Route.Target = sanitizeReplayString(trace.Route.Target)
	trace.Route.Capability = sanitizeReplayString(trace.Route.Capability)
	trace.Route.RiskLevel = sanitizeReplayString(trace.Route.RiskLevel)
	trace.Route.Reasons = sanitizeReplayStrings(trace.Route.Reasons)
	trace.Plan.Goal = sanitizeReplayString(trace.Plan.Goal)
	trace.Plan.Assumptions = sanitizeReplayStrings(trace.Plan.Assumptions)
	trace.Plan.Steps = sanitizeReplayStrings(trace.Plan.Steps)
	trace.Plan.ToolsNeeded = sanitizeReplayStrings(trace.Plan.ToolsNeeded)
	trace.Plan.FilesNeeded = sanitizeReplayStrings(trace.Plan.FilesNeeded)
	trace.Plan.Verification = sanitizeReplayStrings(trace.Plan.Verification)
	trace.MemoryUsed = sanitizeReplayDocuments(trace.MemoryUsed)
	trace.RAGDocsUsed = sanitizeReplayDocuments(trace.RAGDocsUsed)
	for i := range trace.ToolsConsidered {
		trace.ToolsConsidered[i].Name = sanitizeReplayString(trace.ToolsConsidered[i].Name)
		trace.ToolsConsidered[i].Source = sanitizeReplayString(trace.ToolsConsidered[i].Source)
		trace.ToolsConsidered[i].Status = sanitizeReplayString(trace.ToolsConsidered[i].Status)
		trace.ToolsConsidered[i].Reason = sanitizeReplayString(trace.ToolsConsidered[i].Reason)
		trace.ToolsConsidered[i].RiskLevel = sanitizeReplayString(trace.ToolsConsidered[i].RiskLevel)
		trace.ToolsConsidered[i].Attributes = sanitizeReplayAttributes(trace.ToolsConsidered[i].Attributes)
	}
	if trace.Context != nil {
		context := sanitizeReplayContext(*trace.Context)
		trace.Context = &context
	}
	for i := range trace.ToolsCalled {
		trace.ToolsCalled[i].Name = sanitizeReplayString(trace.ToolsCalled[i].Name)
		trace.ToolsCalled[i].InputSummary = sanitizeReplayString(trace.ToolsCalled[i].InputSummary)
		trace.ToolsCalled[i].OutputSummary = sanitizeReplayString(trace.ToolsCalled[i].OutputSummary)
		trace.ToolsCalled[i].RiskLevel = sanitizeReplayString(trace.ToolsCalled[i].RiskLevel)
		trace.ToolsCalled[i].Error = sanitizeReplayString(trace.ToolsCalled[i].Error)
		trace.ToolsCalled[i].Attributes = sanitizeReplayAttributes(trace.ToolsCalled[i].Attributes)
	}
	for i := range trace.PermissionsRequested {
		trace.PermissionsRequested[i].ID = sanitizeReplayString(trace.PermissionsRequested[i].ID)
		trace.PermissionsRequested[i].ToolName = sanitizeReplayString(trace.PermissionsRequested[i].ToolName)
		trace.PermissionsRequested[i].Reason = sanitizeReplayString(trace.PermissionsRequested[i].Reason)
		trace.PermissionsRequested[i].RiskLevel = sanitizeReplayString(trace.PermissionsRequested[i].RiskLevel)
		trace.PermissionsRequested[i].PolicyExplanation = sanitizeReplayString(trace.PermissionsRequested[i].PolicyExplanation)
		trace.PermissionsRequested[i].Attributes = sanitizeReplayAttributes(trace.PermissionsRequested[i].Attributes)
	}
	for i := range trace.Errors {
		trace.Errors[i].Stage = sanitizeReplayString(trace.Errors[i].Stage)
		trace.Errors[i].Code = sanitizeReplayString(trace.Errors[i].Code)
		trace.Errors[i].Subject = sanitizeReplayString(trace.Errors[i].Subject)
		trace.Errors[i].Message = sanitizeReplayString(trace.Errors[i].Message)
		trace.Errors[i].ExpectedRoute = sanitizeReplayString(trace.Errors[i].ExpectedRoute)
		trace.Errors[i].ExpectedRiskLevel = sanitizeReplayString(trace.Errors[i].ExpectedRiskLevel)
		trace.Errors[i].Attributes = sanitizeReplayAttributes(trace.Errors[i].Attributes)
	}
	trace.FinalResult.Status = sanitizeReplayString(trace.FinalResult.Status)
	trace.FinalResult.Summary = sanitizeReplayString(trace.FinalResult.Summary)
	trace.FinalResult.OutputSummary = sanitizeReplayString(trace.FinalResult.OutputSummary)
	trace.FinalResult.Attributes = sanitizeReplayAttributes(trace.FinalResult.Attributes)
	trace.Verification.Status = sanitizeReplayString(trace.Verification.Status)
	trace.Verification.Checks = sanitizeReplayStrings(trace.Verification.Checks)
	trace.Verification.Notes = sanitizeReplayStrings(trace.Verification.Notes)
	trace.Attributes = sanitizeReplayAttributes(trace.Attributes)
	return trace
}

func sanitizeReplayDocuments(values []replay.DocumentRef) []replay.DocumentRef {
	for i := range values {
		values[i].ID = sanitizeReplayString(values[i].ID)
		values[i].Source = sanitizeReplayString(values[i].Source)
		values[i].Title = sanitizeReplayString(values[i].Title)
		values[i].Snippet = sanitizeReplayString(values[i].Snippet)
	}
	return values
}

func sanitizeReplayContext(value replay.ContextSnapshot) replay.ContextSnapshot {
	for i := range value.BySource {
		value.BySource[i].Source = sanitizeReplayString(value.BySource[i].Source)
	}
	for i := range value.Included {
		value.Included[i].ID = sanitizeReplayString(value.Included[i].ID)
		value.Included[i].Source = sanitizeReplayString(value.Included[i].Source)
		value.Included[i].Title = sanitizeReplayString(value.Included[i].Title)
		value.Included[i].Reason = sanitizeReplayString(value.Included[i].Reason)
	}
	for i := range value.Dropped {
		value.Dropped[i].ID = sanitizeReplayString(value.Dropped[i].ID)
		value.Dropped[i].Source = sanitizeReplayString(value.Dropped[i].Source)
		value.Dropped[i].Title = sanitizeReplayString(value.Dropped[i].Title)
		value.Dropped[i].Reason = sanitizeReplayString(value.Dropped[i].Reason)
	}
	return value
}

func sanitizeReplayString(value string) string {
	clean, _ := learning.SanitizeText(value)
	return clean
}

func sanitizeReplayStrings(values []string) []string {
	for i := range values {
		values[i] = sanitizeReplayString(values[i])
	}
	return values
}

func sanitizeReplayAttributes(values []replay.Attribute) []replay.Attribute {
	for i := range values {
		values[i].Key = sanitizeReplayString(values[i].Key)
		values[i].Value = sanitizeReplayString(values[i].Value)
	}
	return values
}
