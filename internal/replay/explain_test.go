package replay

import (
	"strings"
	"testing"
)

func TestExplainTraceUsesStructuredToolConsiderationsAndRedacts(t *testing.T) {
	trace := NewTrace("Read docs with token=super-secret-value")
	trace.ID = "trace explain"
	trace.Route = RouteSnapshot{
		Category:          "file",
		RiskLevel:         "low",
		ShouldUseTool:     true,
		ShouldAskApproval: true,
	}
	trace.Plan.ToolsNeeded = []string{"read_file"}
	trace.AppendToolConsideration(ToolConsideration{
		Name:      "read_file",
		Source:    "plan",
		Status:    "planned",
		Reason:    "inspect local docs before answering token=super-secret-value",
		RiskLevel: "low",
	})
	trace.AppendToolCall(ToolCall{
		Name:          "read_file",
		Status:        "completed",
		OutputSummary: "read token=super-secret-value",
	})
	trace.AppendPermissionRequest(PermissionRequest{
		ToolName: "read_file",
		Status:   "needs_confirmation",
		Reason:   "approval for token=super-secret-value",
	})
	trace.AppendVerification(VerificationResult{Status: "pass"})

	explained := ExplainTrace(trace)
	if len(explained.ToolsConsidered) != 1 || explained.ToolsConsidered[0].Status != "planned" {
		t.Fatalf("ToolsConsidered = %+v, want planned read_file", explained.ToolsConsidered)
	}
	if len(explained.ToolsUsed) != 1 || explained.ToolsUsed[0].Status != "completed" {
		t.Fatalf("ToolsUsed = %+v, want completed read_file", explained.ToolsUsed)
	}
	joined := explained.RequestSummary + explained.ToolsConsidered[0].Reason + explained.ToolsUsed[0].OutputSummary + explained.Permissions[0].Reason
	if strings.Contains(joined, "super-secret-value") {
		t.Fatalf("explanation leaked secret: %+v", explained)
	}
}
