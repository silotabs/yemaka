package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yemaka/internal/memory"
)

const (
	AgentTurnRunning     = "running"
	AgentTurnCompleted   = "completed"
	AgentTurnFailed      = "failed"
	AgentTurnInterrupted = "interrupted"
)

func (s *Service) startAgentTurn(ctx context.Context, conversationID string, meta toolRunMetadata) (memory.AgentTurn, error) {
	if s == nil || s.Memory == nil || strings.TrimSpace(meta.UserMessageID) == "" {
		return memory.AgentTurn{}, nil
	}
	return s.Memory.SaveAgentTurn(ctx, memory.AgentTurn{
		ConversationID: conversationID,
		SessionID:      meta.SessionID,
		UserMessageID:  meta.UserMessageID,
		Status:         AgentTurnRunning,
		Trace:          []string{"accepted task"},
	})
}

func (s *Service) trackAgentTurn(ctx context.Context, turnID string, emit EventHandler) EventHandler {
	if emit == nil {
		emit = func(Event) error { return nil }
	}
	turnID = strings.TrimSpace(turnID)
	if s == nil || s.Memory == nil || turnID == "" {
		return emit
	}
	return func(event Event) error {
		_ = s.recordAgentTurnEvent(context.WithoutCancel(ctx), turnID, event)
		return emit(event)
	}
}

func (s *Service) recordAgentTurnEvent(ctx context.Context, turnID string, event Event) error {
	if s == nil || s.Memory == nil || strings.TrimSpace(turnID) == "" {
		return nil
	}
	turn, err := s.Memory.GetAgentTurn(ctx, turnID)
	if err != nil {
		return nil
	}
	data := event.Data
	switch event.Type {
	case EventTaskClassified:
		turn.Trace = appendMessageTrace(turn.Trace, fmt.Sprintf("task: %s (%s risk)", firstNonEmptyString(data["task_type"], "chat"), firstNonEmptyString(data["risk_level"], "low")))
	case EventPlanCreated:
		turn.Trace = appendMessageTrace(turn.Trace, "plan: "+data["goal"])
	case EventExecutionDecided:
		tool := firstNonEmptyString(data["tool_name"], data["action"])
		turn.Trace = appendMessageTrace(turn.Trace, strings.TrimSpace(fmt.Sprintf("executor: %s %s", firstNonEmptyString(data["status"], "decided"), tool)))
	case EventModelSelected:
		turn.Model = data["model"]
		turn.Trace = appendMessageTrace(turn.Trace, "model: "+data["model"])
	case EventWorkspaceUsed:
		turn.SourceKind = firstNonEmptyString(data["source_kind"], turn.SourceKind)
		turn.Sources = appendUniqueStrings(turn.Sources, splitCommaValues(data["sources"]))
		count := len(turn.Sources)
		if count == 0 {
			turn.Trace = appendMessageTrace(turn.Trace, fmt.Sprintf("%s: local sources", firstNonEmptyString(turn.SourceKind, "workspace")))
		} else {
			turn.Trace = appendMessageTrace(turn.Trace, fmt.Sprintf("%s: %d source%s", firstNonEmptyString(turn.SourceKind, "workspace"), count, pluralSuffix(count)))
		}
	case EventRAGUsed:
		turn.Trace = appendMessageTrace(turn.Trace, "RAG: retrieved local context")
	case EventMemoryUsed:
		count := firstNonEmptyString(data["count"], data["memories"], data["source_count"])
		turn.Trace = appendMessageTrace(turn.Trace, "memory: "+firstNonEmptyString(count, "relevant context"))
	case EventModelToolCall:
		turn.Trace = appendMessageTrace(turn.Trace, "model tool call: "+firstNonEmptyString(data["names"], data["count"]))
	case EventToolCompleted:
		turn.Trace = appendMessageTrace(turn.Trace, strings.TrimSpace(fmt.Sprintf("tool: %s (%s)", data["tool_name"], firstNonEmptyString(data["status"], "completed"))))
	case EventPermissionRequested:
		turn.Trace = appendMessageTrace(turn.Trace, "permission needed: "+data["tool_name"])
	case EventEditProposed:
		turn.Trace = appendMessageTrace(turn.Trace, "edit proposed: "+data["path"])
	case EventCapabilityGap:
		turn.Trace = appendMessageTrace(turn.Trace, "capability: "+firstNonEmptyString(data["title"], event.Message))
	case EventSchedulerJobProposal:
		turn.Trace = appendMessageTrace(turn.Trace, "scheduler proposal: "+firstNonEmptyString(data["target_name"], data["targetName"]))
	case EventSkillSelected:
		turn.Trace = appendMessageTrace(turn.Trace, "skill: "+data["name"])
	case EventCloudFallback:
		turn.Model = data["model"]
		turn.Trace = appendMessageTrace(turn.Trace, "cloud fallback: "+data["model"])
	case EventVerificationCompleted:
		turn.Trace = appendMessageTrace(turn.Trace, "verification: "+data["status"])
	case EventMessageSaved:
		if data["role"] == "assistant" {
			turn.AssistantMessageID = data["message_id"]
		}
	case EventAgentCompleted:
		turn.Status = AgentTurnCompleted
		turn.CompletedAt = timestampForAgentTurn()
	case EventAgentError:
		if agentTurnErrorIsInterruption(event.Message) {
			turn.Status = AgentTurnInterrupted
			turn.Trace = appendMessageTrace(turn.Trace, "interrupted: "+event.Message)
		} else {
			turn.Status = AgentTurnFailed
			turn.Trace = appendMessageTrace(turn.Trace, "error: "+event.Message)
		}
		turn.CompletedAt = timestampForAgentTurn()
	}
	turn.UpdatedAt = timestampForAgentTurn()
	_, err = s.Memory.SaveAgentTurn(ctx, turn)
	return err
}

func (s *Service) markRunningAgentTurnInterrupted(ctx context.Context, turnID string) {
	if s == nil || s.Memory == nil || strings.TrimSpace(turnID) == "" {
		return
	}
	turn, err := s.Memory.GetAgentTurn(ctx, turnID)
	if err != nil || turn.Status != AgentTurnRunning {
		return
	}
	turn.Status = AgentTurnInterrupted
	turn.Trace = appendMessageTrace(turn.Trace, "interrupted before final answer")
	turn.UpdatedAt = timestampForAgentTurn()
	turn.CompletedAt = turn.UpdatedAt
	_, _ = s.Memory.SaveAgentTurn(ctx, turn)
}

func splitCommaValues(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func timestampForAgentTurn() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func agentTurnErrorIsInterruption(message string) bool {
	lower := strings.ToLower(strings.TrimSpace(message))
	return strings.Contains(lower, "context canceled") ||
		strings.Contains(lower, "context cancelled") ||
		strings.Contains(lower, "request canceled") ||
		strings.Contains(lower, "request cancelled") ||
		strings.Contains(lower, "operation was aborted") ||
		strings.Contains(lower, "aborterror")
}
