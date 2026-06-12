package learning

import (
	"context"
	"fmt"
	"strings"

	"yemaka/internal/memory"
)

const (
	KindWorkflowSuccess = "workflow_success"
	KindWorkflowFailure = "workflow_failure"
	KindWorkflowSkipped = "workflow_skipped"
	KindCorrection      = "correction"
	KindJobRunSummary   = "job_run_summary"
)

type WorkflowRecord struct {
	MemoryID       string   `json:"memoryId"`
	ConversationID string   `json:"conversationId"`
	Kind           string   `json:"kind"`
	Status         string   `json:"status"`
	Goal           string   `json:"goal"`
	Tools          []string `json:"tools"`
	Lesson         string   `json:"lesson"`
}

func RecordWorkflow(ctx context.Context, store *memory.Store, conversationID string) (WorkflowRecord, error) {
	if store == nil {
		return WorkflowRecord{}, fmt.Errorf("memory store is required")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return WorkflowRecord{}, fmt.Errorf("conversation id is required")
	}
	messages, err := store.ListConversationMessages(ctx, conversationID, 100)
	if err != nil {
		return WorkflowRecord{}, err
	}
	if len(messages) == 0 {
		return WorkflowRecord{}, fmt.Errorf("conversation has no messages")
	}
	toolRuns, err := store.ListToolRunsForConversation(ctx, conversationID, 100)
	if err != nil {
		return WorkflowRecord{}, err
	}
	kind, status := workflowKind(toolRuns)
	if shouldSkipWorkflowMemory(kind, toolRuns) {
		return WorkflowRecord{
			ConversationID: conversationID,
			Kind:           KindWorkflowSkipped,
			Status:         "skipped",
			Goal:           sanitizedFirstUser(messages),
			Tools:          toolNames(toolRuns),
			Lesson:         "No reusable workflow memory saved for ordinary chat.",
		}, nil
	}
	source := "workflow:" + conversationID
	if existing, ok, err := store.FindMemoryByKindSource(ctx, kind, source); err != nil {
		return WorkflowRecord{}, err
	} else if ok {
		return WorkflowRecord{
			MemoryID:       existing.ID,
			ConversationID: conversationID,
			Kind:           existing.Kind,
			Status:         status,
			Goal:           sanitizedFirstUser(messages),
			Tools:          toolNames(toolRuns),
			Lesson:         existing.Content,
		}, nil
	}

	content, _ := SanitizeText(workflowMemoryContent(conversationID, kind, status, messages, toolRuns))
	item, err := store.SaveMemory(ctx, memory.Memory{
		Kind:       kind,
		Content:    content,
		Importance: workflowImportance(kind),
		Source:     source,
	})
	if err != nil {
		return WorkflowRecord{}, err
	}
	return WorkflowRecord{
		MemoryID:       item.ID,
		ConversationID: conversationID,
		Kind:           item.Kind,
		Status:         status,
		Goal:           sanitizedFirstUser(messages),
		Tools:          toolNames(toolRuns),
		Lesson:         content,
	}, nil
}

func SaveCorrection(ctx context.Context, store *memory.Store, conversationID string, correction string) (memory.Memory, error) {
	if store == nil {
		return memory.Memory{}, fmt.Errorf("memory store is required")
	}
	conversationID = strings.TrimSpace(conversationID)
	correction = strings.TrimSpace(correction)
	if conversationID == "" {
		return memory.Memory{}, fmt.Errorf("conversation id is required")
	}
	if correction == "" {
		return memory.Memory{}, fmt.Errorf("correction content is required")
	}
	clean, _ := SanitizeText(correction)
	return store.SaveMemory(ctx, memory.Memory{
		Kind:       KindCorrection,
		Content:    "Correction for " + conversationID + ": " + clean,
		Importance: 5,
		Source:     "correction:" + conversationID,
	})
}

func workflowKind(toolRuns []memory.ToolRun) (string, string) {
	pendingStatus := ""
	for _, run := range toolRuns {
		status := strings.ToLower(strings.TrimSpace(run.Status))
		if isWorkflowFailureStatus(status) {
			return KindWorkflowFailure, status
		}
		if isIncompleteWorkflowRun(run, status, toolRuns) {
			if pendingStatus == "" {
				pendingStatus = status
			}
		}
	}
	if pendingStatus != "" {
		return KindWorkflowSkipped, pendingStatus
	}
	return KindWorkflowSuccess, "completed"
}

func shouldSkipWorkflowMemory(kind string, toolRuns []memory.ToolRun) bool {
	if kind == KindWorkflowFailure {
		return false
	}
	if kind == KindWorkflowSkipped {
		return true
	}
	if hasMeaningfulToolRun(toolRuns) {
		return false
	}
	return true
}

func hasMeaningfulToolRun(toolRuns []memory.ToolRun) bool {
	for _, run := range toolRuns {
		name := strings.ToLower(strings.TrimSpace(run.ToolName))
		if name == "" || isWorkflowMetaTool(name) {
			continue
		}
		return true
	}
	return false
}

func isWorkflowFailureStatus(status string) bool {
	switch status {
	case "failed", "blocked", "timeout", "rollback_required", "needs_follow_up", "error", "not_runnable":
		return true
	default:
		return false
	}
}

func isIncompleteWorkflowRun(run memory.ToolRun, status string, toolRuns []memory.ToolRun) bool {
	switch status {
	case "needs_confirmation", "pending", "pending_approval", "awaiting_approval", "approval_required":
		return true
	case "ready", "in_progress", "running", "queued", "proposed", "ready_for_preview":
		if strings.EqualFold(strings.TrimSpace(run.ToolName), "agent_executor") {
			target := agentExecutorTargetTool(run.Input)
			return target == "" || !hasTerminalToolRun(toolRuns, target)
		}
		return true
	default:
		return false
	}
}

func hasTerminalToolRun(toolRuns []memory.ToolRun, toolName string) bool {
	toolName = strings.ToLower(strings.TrimSpace(toolName))
	if toolName == "" {
		return false
	}
	for _, run := range toolRuns {
		if strings.ToLower(strings.TrimSpace(run.ToolName)) != toolName {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(run.Status))
		if status == "completed" || status == "pass" || isWorkflowFailureStatus(status) {
			return true
		}
	}
	return false
}

func agentExecutorTargetTool(input any) string {
	values, ok := input.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range []string{"tool_name", "toolName", "tool"} {
		if value, ok := values[key]; ok {
			tool := strings.TrimSpace(fmt.Sprint(value))
			if tool != "" && tool != "<nil>" {
				return tool
			}
		}
	}
	return ""
}

func isWorkflowMetaTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "agent_verifier", "agent_executor", "permission_request", "edit_proposal":
		return true
	default:
		return false
	}
}

func workflowImportance(kind string) int {
	if kind == KindWorkflowFailure {
		return 4
	}
	return 3
}

func workflowMemoryContent(conversationID string, kind string, status string, messages []memory.Message, toolRuns []memory.ToolRun) string {
	goal := firstUser(messages)
	tools := toolNames(toolRuns)
	parts := []string{
		"Conversation: " + conversationID,
		"Outcome: " + strings.TrimPrefix(kind, "workflow_") + " (" + status + ")",
		"Goal: " + compact(goal, 240),
	}
	if len(tools) > 0 {
		parts = append(parts, "Tools: "+strings.Join(tools, ", "))
	}
	if kind == KindWorkflowFailure {
		parts = append(parts, "Lesson: retrieve this before similar tasks so Yemaka can avoid repeating the failure and suggest skill or extension improvement.")
	} else {
		parts = append(parts, "Lesson: retrieve this before similar tasks and consider saving the workflow as a reusable skill if it repeats.")
	}
	return strings.Join(parts, "\n")
}

func firstUser(messages []memory.Message) string {
	for _, msg := range messages {
		if msg.Role == "user" && strings.TrimSpace(msg.Content) != "" {
			return strings.TrimSpace(msg.Content)
		}
	}
	return ""
}

func sanitizedFirstUser(messages []memory.Message) string {
	clean, _ := SanitizeText(firstUser(messages))
	return clean
}

func toolNames(toolRuns []memory.ToolRun) []string {
	seen := map[string]bool{}
	var tools []string
	for _, run := range toolRuns {
		name := strings.TrimSpace(run.ToolName)
		if name == "" || isWorkflowMetaTool(name) || seen[name] {
			continue
		}
		seen[name] = true
		tools = append(tools, name+"("+strings.TrimSpace(run.Status)+")")
	}
	return tools
}

func compact(input string, max int) string {
	input = strings.Join(strings.Fields(input), " ")
	if max <= 0 || len(input) <= max {
		return input
	}
	if max <= 3 {
		return input[:max]
	}
	return strings.TrimSpace(input[:max-3]) + "..."
}
