package agent

import (
	"context"
	"fmt"
	"strings"
)

type PermissionApprovalOutcome struct {
	Decision   ExecutionDecision
	Executed   bool
	ToolStatus string
	Message    string
	Result     *ExecutionResult
	RunError   string
}

func ResolveApprovedPermission(ctx context.Context, request PermissionRequest, executor ToolExecutor) PermissionApprovalOutcome {
	decision := ExecutionDecision{
		Status:               ExecutionReady,
		RequestID:            strings.TrimSpace(request.RequestID),
		ToolName:             normalizeToolName(request.ToolName),
		Command:              append([]string(nil), request.Command...),
		RiskLevel:            request.RiskLevel,
		RequiresConfirmation: false,
		Reason:               "user approved the pending permission request",
		PolicyLevel:          request.PolicyLevel,
		PolicyExplanation:    request.PolicyExplanation,
	}
	if decision.RiskLevel == "" {
		decision.RiskLevel = RiskMedium
	}
	if decision.ToolName == "" {
		decision.ToolName = "requested_action"
	}

	if !permissionCanResume(decision) {
		return PermissionApprovalOutcome{
			Decision:   decision,
			ToolStatus: "not_runnable",
			Message:    PermissionApprovalHoldMessage(request),
		}
	}
	if executor == nil {
		return PermissionApprovalOutcome{
			Decision:   decision,
			ToolStatus: "blocked",
			Message:    fmt.Sprintf("Approval recorded, but `%s` cannot run because the safe tool executor is not available in this interface.", decision.ToolName),
		}
	}

	result, err := executor(ctx, decision)
	if result.Status == "" {
		result.Status = "completed"
	}
	outcome := PermissionApprovalOutcome{
		Decision:   decision,
		Executed:   err == nil,
		ToolStatus: result.Status,
		Result:     &result,
	}
	if err != nil {
		result.Status = "failed"
		outcome.Executed = false
		outcome.ToolStatus = "failed"
		outcome.RunError = err.Error()
		outcome.Message = PermissionApprovalFailureMessage(request, err)
		return outcome
	}
	outcome.Message = PermissionApprovalResultMessage(request, result)
	return outcome
}

func PermissionOutcomeToolName(outcome PermissionApprovalOutcome) string {
	if outcome.Executed && strings.TrimSpace(outcome.Decision.ToolName) != "" {
		return outcome.Decision.ToolName
	}
	return "permission_resume"
}

func PermissionOutcomeStatus(outcome PermissionApprovalOutcome) string {
	if strings.TrimSpace(outcome.ToolStatus) != "" {
		return outcome.ToolStatus
	}
	if outcome.Executed {
		return "completed"
	}
	return "not_runnable"
}

func PermissionOutcomeOutput(outcome PermissionApprovalOutcome) map[string]any {
	output := map[string]any{
		"executed": outcome.Executed,
		"message":  outcome.Message,
	}
	if outcome.Result != nil {
		output["result"] = outcome.Result
	}
	if strings.TrimSpace(outcome.RunError) != "" {
		output["error"] = outcome.RunError
	}
	return output
}

func permissionCanResume(decision ExecutionDecision) bool {
	switch normalizeToolName(decision.ToolName) {
	case "doctor_status", "project_map", "secret_scan", "git_status", "git_diff", "run_tests":
		return true
	case "read_file", "list_files", "file_stat", "file_tree", "search_files", "rag_search", "symbol_search", "patch_preview", "internet_fetch", "internet_head", "internet_search":
		return len(decision.Command) > 1
	case "memory_write":
		if strings.TrimSpace(decision.RequestID) == "" {
			return false
		}
		_, _, ok := memoryWriteCommandInput(decision.Command)
		return ok
	default:
		return false
	}
}

func PermissionApprovalHoldMessage(request PermissionRequest) string {
	tool := normalizeToolName(request.ToolName)
	if tool == "" {
		tool = "the requested action"
	}
	switch tool {
	case "edit_file", "write_file":
		return "Approval recorded. File edits still need the diff and snapshot flow before I apply anything. Open the file write preview, confirm the exact change, then apply it."
	case "safe_tool", "requested_action":
		return "Approval recorded, but the pending request did not include a concrete command, file change, or typed tool input to run. Tell me the exact action you want, and I will route it through the safe tool loop."
	case "internet_fetch", "internet_head":
		return "Approval recorded, but the internet tool needs a specific http(s) URL before I can run it."
	case "internet_search":
		return "Approval recorded, but internet search needs a concrete search query and a configured search provider before I can run it."
	case "memory_write":
		return "Approval recorded, but memory_write needs a concrete memory kind and content before I can save anything."
	case "read_file":
		return "Approval recorded, but file reading needs a concrete workspace path before I can run it."
	case "list_files":
		return "Approval recorded, but directory listing needs a concrete workspace path before I can run it."
	case "file_stat":
		return "Approval recorded, but file metadata needs a concrete workspace path before I can run it."
	case "file_tree":
		return "Approval recorded, but file tree listing needs a concrete workspace folder before I can run it."
	case "search_files", "rag_search", "symbol_search":
		return fmt.Sprintf("Approval recorded, but `%s` needs a concrete query before I can run it.", tool)
	case "patch_preview":
		return "Approval recorded, but patch preview needs a target path and replacement content before I can run it."
	default:
		return fmt.Sprintf("Approval recorded, but `%s` is not a resumable first-class tool yet. I will not run an unknown or implicit action.", tool)
	}
}

func PermissionApprovalFailureMessage(request PermissionRequest, err error) string {
	tool := normalizeToolName(request.ToolName)
	if tool == "" {
		tool = "the approved tool"
	}
	return fmt.Sprintf("Approval received, but `%s` could not run: %s", tool, strings.TrimSpace(err.Error()))
}

func PermissionApprovalResultMessage(request PermissionRequest, result ExecutionResult) string {
	tool := normalizeToolName(request.ToolName)
	if tool == "" {
		tool = "the approved tool"
	}
	status := result.Status
	if status == "" {
		status = "completed"
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "Approval received. I ran `%s` and it %s.", tool, status)
	context := strings.TrimSpace(result.Context)
	if context != "" {
		fmt.Fprintf(&builder, "\n\n%s", trimApprovalResult(context, 1800))
	}
	if len(result.Sources) > 0 {
		fmt.Fprintf(&builder, "\n\nSource: %s", strings.Join(result.Sources, ", "))
	}
	return strings.TrimSpace(builder.String())
}

func trimApprovalResult(value string, max int) string {
	value = strings.TrimSpace(value)
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 14 {
		return value[:max]
	}
	return strings.TrimSpace(value[:max-14]) + "\n[truncated]"
}
