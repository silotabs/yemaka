package agent

import (
	"context"
	"fmt"
	"strings"

	"yemaka/internal/memory"
	"yemaka/internal/replay"
)

type ReplayLoader interface {
	Load(string) (replay.Trace, error)
}

type ReplayReadWriter interface {
	ReplaySaver
	ReplayLoader
}

type MessageActivity struct {
	Trace      []string
	Sources    []string
	SourceKind string
}

func ConversationMessageActivities(ctx context.Context, memories *memory.Store, loader ReplayLoader, conversationID string, messages []memory.Message) (map[string]MessageActivity, error) {
	activities := map[string]MessageActivity{}
	if memories == nil || strings.TrimSpace(conversationID) == "" {
		return activities, nil
	}
	messages = memory.WithInferredResponseParents(messages)
	runs, err := memories.ListToolRunsForConversation(ctx, conversationID, 200)
	if err != nil {
		return nil, err
	}
	for _, msg := range messages {
		if msg.Role != "assistant" || strings.TrimSpace(msg.ID) == "" {
			continue
		}
		activity, ok := replayMessageActivity(loader, conversationID, msg)
		if !ok {
			activity = toolRunMessageActivity(runs, msg)
		}
		if len(activity.Trace) == 0 && len(activity.Sources) == 0 {
			continue
		}
		activities[msg.ID] = activity
	}
	return activities, nil
}

func replayMessageActivity(loader ReplayLoader, conversationID string, msg memory.Message) (MessageActivity, bool) {
	if loader == nil || strings.TrimSpace(msg.ParentID) == "" {
		return MessageActivity{}, false
	}
	trace, err := loader.Load(replayTraceID(conversationID, msg.ParentID, msg.ID))
	if err != nil {
		return MessageActivity{}, false
	}
	activity := MessageActivity{
		Sources:    replaySourceTitles(trace.RAGDocsUsed),
		SourceKind: replaySourceKind(trace.RAGDocsUsed),
	}
	activity.Trace = appendMessageTrace(activity.Trace, fmt.Sprintf("task: %s (%s risk)", firstNonEmptyString(attributeValue(trace.Attributes, "task_type"), "chat"), firstNonEmptyString(trace.Route.RiskLevel, "low")))
	activity.Trace = appendMessageTrace(activity.Trace, "plan: "+trace.Plan.Goal)
	activity.Trace = appendExecutorTrace(activity.Trace, trace)
	activity.Trace = appendMessageTrace(activity.Trace, "model: "+msg.Model)
	if len(trace.MemoryUsed) > 0 {
		activity.Trace = appendMessageTrace(activity.Trace, fmt.Sprintf("memory: %d relevant item%s", len(trace.MemoryUsed), pluralSuffix(len(trace.MemoryUsed))))
	}
	if len(activity.Sources) > 0 {
		label := "RAG"
		if activity.SourceKind != "" && activity.SourceKind != "rag" {
			label = activity.SourceKind
		}
		activity.Trace = appendMessageTrace(activity.Trace, fmt.Sprintf("%s: %d source%s", label, len(activity.Sources), pluralSuffix(len(activity.Sources))))
	}
	activity.Trace = appendMessageTrace(activity.Trace, "verification: "+trace.Verification.Status)
	return activity, len(activity.Trace) > 0 || len(activity.Sources) > 0
}

func toolRunMessageActivity(runs []memory.ToolRun, msg memory.Message) MessageActivity {
	matched := matchingMessageRuns(runs, msg)
	if len(matched) == 0 {
		return MessageActivity{}
	}
	activity := MessageActivity{}
	hasExecutor := false
	hasVerification := false
	for _, run := range matched {
		tool := normalizeToolName(run.ToolName)
		switch tool {
		case "agent_executor":
			hasExecutor = true
			activity.Trace = appendMessageTrace(activity.Trace, fmt.Sprintf("executor: %s %s", firstNonEmptyString(run.Status, "decided"), storedString(run.Output, "tool_name", "ToolName")))
		case "agent_verifier":
			hasVerification = true
			activity.Trace = appendMessageTrace(activity.Trace, "verification: "+firstNonEmptyString(run.Status, storedString(run.Output, "status", "Status")))
			if modelTask := storedString(run.Input, "model_task", "modelTask", "ModelTask"); modelTask != "" {
				activity.Trace = prependMessageTrace(activity.Trace, fmt.Sprintf("task: %s (%s risk)", modelTask, firstNonEmptyString(run.RiskLevel, "low")))
			}
			if goal := storedString(run.Input, "goal", "Goal"); goal != "" {
				activity.Trace = prependMessageTrace(activity.Trace, fmt.Sprintf("plan: %s", goal))
			}
		case "permission_request":
			activity.Trace = appendMessageTrace(activity.Trace, "permission needed: "+firstNonEmptyString(storedString(run.Output, "tool_name", "toolName", "ToolName"), storedString(run.Input, "tool_name", "toolName")))
		case "edit_proposal":
			activity.Trace = appendMessageTrace(activity.Trace, "edit proposed: "+storedString(run.Output, "path", "Path"))
		default:
			activity.Trace = appendMessageTrace(activity.Trace, fmt.Sprintf("tool: %s (%s)", run.ToolName, firstNonEmptyString(run.Status, "completed")))
			activity.Sources = appendUniqueString(activity.Sources, storedString(run.Output, "source", "Source"))
			activity.Sources = appendUniqueStrings(activity.Sources, storedStringSlice(run.Output, "sources", "Sources"))
			activity.SourceKind = firstNonEmptyString(activity.SourceKind, storedString(run.Output, "source_kind", "sourceKind", "SourceKind"))
		}
	}
	if !hasExecutor {
		activity.Trace = appendMessageTrace(activity.Trace, "executor: not_required")
	}
	if msg.Model != "" {
		activity.Trace = appendMessageTrace(activity.Trace, "model: "+msg.Model)
	}
	if len(activity.Sources) > 0 {
		activity.Trace = appendMessageTrace(activity.Trace, fmt.Sprintf("%s: %d source%s", firstNonEmptyString(activity.SourceKind, "tool"), len(activity.Sources), pluralSuffix(len(activity.Sources))))
	}
	if !hasVerification {
		for _, run := range matched {
			if run.Status != "" {
				activity.Trace = appendMessageTrace(activity.Trace, "verification: "+run.Status)
				break
			}
		}
	}
	return activity
}

func appendExecutorTrace(trace []string, item replay.Trace) []string {
	for _, permission := range item.PermissionsRequested {
		trace = appendMessageTrace(trace, "permission needed: "+permission.ToolName)
	}
	for _, tool := range item.ToolsConsidered {
		if tool.Source != "execution_decision" {
			continue
		}
		trace = appendMessageTrace(trace, fmt.Sprintf("executor: %s %s", firstNonEmptyString(tool.Status, "decided"), tool.Name))
	}
	for _, tool := range item.ToolsCalled {
		trace = appendMessageTrace(trace, fmt.Sprintf("tool: %s (%s)", tool.Name, firstNonEmptyString(tool.Status, item.FinalResult.Status, "completed")))
	}
	if len(item.ToolsCalled) == 0 && len(item.PermissionsRequested) == 0 {
		hasExecutionDecision := false
		for _, tool := range item.ToolsConsidered {
			if tool.Source == "execution_decision" {
				hasExecutionDecision = true
				break
			}
		}
		if !hasExecutionDecision {
			trace = appendMessageTrace(trace, "executor: not_required")
		}
	}
	return trace
}

func matchingMessageRuns(runs []memory.ToolRun, msg memory.Message) []memory.ToolRun {
	var matched []memory.ToolRun
	for _, run := range runs {
		if run.AssistantMessageID == msg.ID ||
			(msg.ParentID != "" && (run.UserMessageID == msg.ParentID || run.ParentMessageID == msg.ParentID)) {
			matched = append(matched, run)
		}
	}
	return matched
}

func replaySourceTitles(refs []replay.DocumentRef) []string {
	var out []string
	for _, ref := range refs {
		out = appendUniqueString(out, firstNonEmptyString(ref.Title, ref.ID, ref.Source))
	}
	return out
}

func replaySourceKind(refs []replay.DocumentRef) string {
	for _, ref := range refs {
		if strings.TrimSpace(ref.Source) != "" {
			return strings.TrimSpace(ref.Source)
		}
	}
	return ""
}

func attributeValue(attrs []replay.Attribute, key string) string {
	for _, attr := range attrs {
		if strings.EqualFold(strings.TrimSpace(attr.Key), key) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func appendMessageTrace(trace []string, value string) []string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if value == "" {
		return trace
	}
	for _, existing := range trace {
		if existing == value {
			return trace
		}
	}
	return append(trace, value)
}

func prependMessageTrace(trace []string, value string) []string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if value == "" {
		return trace
	}
	for _, existing := range trace {
		if existing == value {
			return trace
		}
	}
	return append([]string{value}, trace...)
}

func appendUniqueString(values []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func appendUniqueStrings(values []string, next []string) []string {
	for _, value := range next {
		values = appendUniqueString(values, value)
	}
	return values
}

func pluralSuffix(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}

func storedString(value any, keys ...string) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if out := strings.TrimSpace(fmt.Sprint(typed[key])); out != "" && out != "<nil>" {
				return out
			}
		}
	}
	return ""
}

func storedStringSlice(value any, keys ...string) []string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range keys {
			if out := anyStringSlice(typed[key]); len(out) > 0 {
				return out
			}
		}
	}
	return nil
}

func anyStringSlice(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			out = appendUniqueString(out, fmt.Sprint(item))
		}
		return out
	}
	return nil
}
