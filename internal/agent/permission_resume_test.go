package agent

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/memory"
)

func TestResolveApprovedPermissionRunsConcreteResumableTool(t *testing.T) {
	request := PermissionRequest{
		RequestID: "perm_1",
		ToolName:  "internet_search",
		Command:   []string{"internet_search", "SearXNG"},
		RiskLevel: RiskMedium,
	}
	var executed ExecutionDecision
	outcome := ResolveApprovedPermission(context.Background(), request, func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
		executed = decision
		return ExecutionResult{
			Context:    "searched SearXNG",
			Sources:    []string{"internet:search"},
			SourceKind: "internet",
			Status:     "completed",
		}, nil
	})

	if !outcome.Executed || outcome.ToolStatus != "completed" {
		t.Fatalf("outcome = %+v, want completed execution", outcome)
	}
	if executed.ToolName != "internet_search" || len(executed.Command) != 2 || executed.Command[1] != "SearXNG" {
		t.Fatalf("executed decision = %+v, want concrete internet_search command", executed)
	}
	if !strings.Contains(outcome.Message, "Approval received") || !strings.Contains(outcome.Message, "searched SearXNG") {
		t.Fatalf("message = %q, want approval result context", outcome.Message)
	}
	if outcome.Result == nil || outcome.Result.SourceKind != "internet" {
		t.Fatalf("result = %+v, want final internet tool result", outcome.Result)
	}
	if PermissionOutcomeToolName(outcome) != "internet_search" {
		t.Fatalf("PermissionOutcomeToolName() = %q, want internet_search", PermissionOutcomeToolName(outcome))
	}
	if PermissionOutcomeStatus(outcome) != "completed" {
		t.Fatalf("PermissionOutcomeStatus() = %q, want completed", PermissionOutcomeStatus(outcome))
	}
	output := PermissionOutcomeOutput(outcome)
	if output["executed"] != true {
		t.Fatalf("output executed = %#v, want true", output["executed"])
	}
	result, ok := output["result"].(*ExecutionResult)
	if !ok || result.Context != "searched SearXNG" {
		t.Fatalf("output result = %#v, want resumed final result", output["result"])
	}
	if message, ok := output["message"].(string); !ok || !strings.Contains(message, "Approval received") {
		t.Fatalf("output message = %#v, want approval resume summary", output["message"])
	}
}

func TestResolveApprovedPermissionHoldsImplicitSafeTool(t *testing.T) {
	outcome := ResolveApprovedPermission(context.Background(), PermissionRequest{
		RequestID: "perm_2",
		ToolName:  "safe_tool",
		RiskLevel: RiskHigh,
	}, func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
		t.Fatal("executor should not run without a concrete resumable action")
		return ExecutionResult{}, nil
	})

	if outcome.Executed {
		t.Fatalf("Executed = true, want hold")
	}
	if outcome.ToolStatus != "not_runnable" {
		t.Fatalf("ToolStatus = %q, want not_runnable", outcome.ToolStatus)
	}
	if !strings.Contains(outcome.Message, "did not include a concrete command") {
		t.Fatalf("message = %q, want concrete action guidance", outcome.Message)
	}
}

func TestResolveApprovedPermissionReportsToolFailureWithoutRetryingImplicitly(t *testing.T) {
	outcome := ResolveApprovedPermission(context.Background(), PermissionRequest{
		RequestID: "perm_3",
		ToolName:  "read_file",
		Command:   []string{"read_file", "missing.md"},
		RiskLevel: RiskLow,
	}, func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
		return ExecutionResult{}, errors.New("file not found")
	})

	if outcome.Executed {
		t.Fatalf("Executed = true, want failed execution")
	}
	if outcome.ToolStatus != "failed" {
		t.Fatalf("ToolStatus = %q, want failed", outcome.ToolStatus)
	}
	if !strings.Contains(outcome.Message, "could not run") || !strings.Contains(outcome.Message, "file not found") {
		t.Fatalf("message = %q, want helpful failure", outcome.Message)
	}
}

func TestResolveApprovedPermissionRunsApprovedMemoryWrite(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   t.TempDir(),
		MaxContextChars: 2000,
		Memory:          store,
	})
	outcome := ResolveApprovedPermission(ctx, PermissionRequest{
		RequestID: "perm_memory_1",
		ToolName:  "memory_write",
		Command: []string{
			"memory_write",
			"follow_up",
			"Email Alex about the release checklist. token=super-secret-value lives under /Users/example/project.",
		},
		RiskLevel:            RiskMedium,
		Reason:               "user confirmed saving this follow-up memory",
		RequiresConfirmation: true,
	}, executor)

	if !outcome.Executed || outcome.ToolStatus != "completed" {
		t.Fatalf("outcome = %+v, want completed memory_write execution", outcome)
	}
	if outcome.Result == nil || outcome.Result.SourceKind != "memory" || len(outcome.Result.Sources) != 1 {
		t.Fatalf("result = %+v, want memory source", outcome.Result)
	}
	if PermissionOutcomeToolName(outcome) != "memory_write" {
		t.Fatalf("PermissionOutcomeToolName() = %q, want memory_write", PermissionOutcomeToolName(outcome))
	}

	memories, err := store.SearchMemories(ctx, "release checklist Alex", 5)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("SearchMemories() len = %d, want 1", len(memories))
	}
	if memories[0].Kind != "follow_up" {
		t.Fatalf("memory kind = %q, want follow_up", memories[0].Kind)
	}
	if strings.Contains(memories[0].Content, "super-secret") || strings.Contains(memories[0].Content, "/Users/example") {
		t.Fatalf("memory content was not stored through redaction path: %q", memories[0].Content)
	}
}

func TestResolveApprovedPermissionDoesNotResumeIncompleteMemoryWrite(t *testing.T) {
	cases := []PermissionRequest{
		{
			RequestID: "perm_memory_missing_content",
			ToolName:  "memory_write",
			Command:   []string{"memory_write", "follow_up"},
			RiskLevel: RiskMedium,
		},
		{
			RequestID: "perm_memory_missing_kind",
			ToolName:  "memory_write",
			Command:   []string{"memory_write", " ", "Remember the release checklist"},
			RiskLevel: RiskMedium,
		},
		{
			ToolName:  "memory_write",
			Command:   []string{"memory_write", "follow_up", "Remember the release checklist"},
			RiskLevel: RiskMedium,
		},
	}

	for _, request := range cases {
		outcome := ResolveApprovedPermission(context.Background(), request, func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
			t.Fatalf("executor should not run incomplete memory_write request: %+v", request)
			return ExecutionResult{}, nil
		})
		if outcome.Executed {
			t.Fatalf("Executed = true for request %+v, want hold", request)
		}
		if outcome.ToolStatus != "not_runnable" {
			t.Fatalf("ToolStatus = %q for request %+v, want not_runnable", outcome.ToolStatus, request)
		}
		if !strings.Contains(outcome.Message, "memory kind and content") {
			t.Fatalf("message = %q, want memory_write guidance", outcome.Message)
		}
	}
}
