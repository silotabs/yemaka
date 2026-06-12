package agent

import (
	"context"
	"strings"

	"yemaka/internal/memory"
	"yemaka/internal/routing"
)

func (s *Service) attachContinuationContext(ctx context.Context, conversationID string, input *PlanInput) error {
	if s == nil || s.Memory == nil || input == nil || strings.TrimSpace(conversationID) == "" {
		return nil
	}
	if !input.Continuation.IsZero() {
		return nil
	}
	prior, ok, err := s.latestContinuationPriorState(ctx, conversationID)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	frame := routing.BuildContinuationFrame(routing.ContinuationInput{
		Content:    input.Content,
		TaskMemory: input.TaskMemory,
		Prior:      prior,
	})
	if frame.Kind == routing.ContinuationKindNewTask {
		return nil
	}
	input.Continuation = frame
	return nil
}

func (s *Service) latestContinuationPriorState(ctx context.Context, conversationID string) (routing.ContinuationPriorState, bool, error) {
	runs, err := s.Memory.ListToolRunsForConversation(ctx, conversationID, 80)
	if err != nil {
		return routing.ContinuationPriorState{}, false, err
	}
	for i := len(runs) - 1; i >= 0; i-- {
		if prior, ok := continuationPriorFromToolRun(runs[i]); ok {
			return prior, true, nil
		}
	}
	return routing.ContinuationPriorState{}, false, nil
}

func continuationPriorFromToolRun(run memory.ToolRun) (routing.ContinuationPriorState, bool) {
	tool := strings.TrimSpace(run.ToolName)
	status := strings.TrimSpace(run.Status)
	command := commandFromToolRunValue(run.Input)
	if output, ok := run.Output.(map[string]any); ok {
		if value := stringMapValue(output, "tool_name"); value != "" {
			tool = value
		}
		if value := stringMapValue(output, "status"); value != "" {
			status = value
		}
		reason := stringMapValue(output, "reason")
		if values := stringSliceMapValue(output, "command"); len(values) > 0 {
			command = values
		}
		if prior, ok := continuationPriorState(tool, status, reason, command); ok {
			return prior, true
		}
	}
	return continuationPriorState(tool, status, "", command)
}

func continuationPriorState(tool string, status string, reason string, command []string) (routing.ContinuationPriorState, bool) {
	route := routing.RouteCategoryForTool(tool)
	if route == "" && strings.TrimSpace(tool) == "agent_executor" {
		return routing.ContinuationPriorState{}, false
	}
	target := continuationCommandTarget(command, tool)
	if target == "" {
		return routing.ContinuationPriorState{}, false
	}
	return routing.ContinuationPriorState{
		RouteCategory: route,
		ToolName:      tool,
		Target:        target,
		SourceOfTruth: routing.ContinuationSourceForRoute(route, tool),
		FailureStatus: continuationFailureStatus(status),
		FailureReason: strings.TrimSpace(reason),
	}, true
}

func commandFromToolRunValue(value any) []string {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return stringSliceMapValue(m, "command")
}

func stringMapValue(values map[string]any, key string) string {
	value, ok := values[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func stringSliceMapValue(values map[string]any, key string) []string {
	value, ok := values[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				continue
			}
			out = append(out, strings.TrimSpace(text))
		}
		return out
	default:
		return nil
	}
}

func continuationCommandTarget(command []string, tool string) string {
	for _, part := range command {
		part = strings.TrimSpace(part)
		if part == "" || part == tool {
			continue
		}
		return part
	}
	return ""
}

func continuationFailureStatus(status string) string {
	switch strings.TrimSpace(status) {
	case ExecutionBlocked, ExecutionNeedsConfirmation, "failed", "error":
		return strings.TrimSpace(status)
	default:
		return ""
	}
}
