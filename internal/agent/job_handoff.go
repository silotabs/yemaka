package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/scheduler"
)

const JobRunSummaryMemoryKind = "job_run_summary"

type JobRunHandoff struct {
	MemoryID       string `json:"memoryId,omitempty"`
	MessageID      string `json:"messageId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	Summary        string `json:"summary"`
}

func RecordJobRunResultFromStore(ctx context.Context, memories *memory.Store, jobs *scheduler.Store, run scheduler.JobRun, conversationID string) (JobRunHandoff, error) {
	if jobs == nil {
		return JobRunHandoff{}, fmt.Errorf("scheduler store is required")
	}
	if strings.TrimSpace(run.JobID) == "" {
		return JobRunHandoff{}, nil
	}
	job, err := jobs.Get(ctx, run.JobID)
	if err != nil {
		return JobRunHandoff{}, err
	}
	return RecordJobRunResult(ctx, memories, job, run, conversationID)
}

func RecordJobRunResult(ctx context.Context, memories *memory.Store, job scheduler.Job, run scheduler.JobRun, conversationID string) (JobRunHandoff, error) {
	if memories == nil {
		return JobRunHandoff{}, fmt.Errorf("memory store is required")
	}
	if strings.TrimSpace(run.ID) == "" {
		return JobRunHandoff{}, nil
	}
	if strings.TrimSpace(job.ID) == "" {
		return JobRunHandoff{}, fmt.Errorf("job is required")
	}

	summary, _ := learning.SanitizeText(BuildJobRunSummary(job, run))
	source := "job_run:" + run.ID
	if existing, ok, err := memories.FindMemoryByKindSource(ctx, JobRunSummaryMemoryKind, source); err != nil {
		return JobRunHandoff{}, err
	} else if ok {
		conversationID := resolveJobConversationID(job, conversationID)
		messageID, err := ensureJobRunConversationMessage(ctx, memories, conversationID, existing.Content, run.ID)
		if err != nil {
			return JobRunHandoff{}, err
		}
		return JobRunHandoff{
			MemoryID:       existing.ID,
			MessageID:      messageID,
			ConversationID: conversationID,
			Summary:        existing.Content,
		}, nil
	}

	item, err := memories.SaveMemory(ctx, memory.Memory{
		Kind:       JobRunSummaryMemoryKind,
		Content:    summary,
		Importance: jobRunImportance(run),
		Source:     source,
	})
	if err != nil {
		return JobRunHandoff{}, err
	}

	result := JobRunHandoff{
		MemoryID:       item.ID,
		ConversationID: resolveJobConversationID(job, conversationID),
		Summary:        summary,
	}
	messageID, err := ensureJobRunConversationMessage(ctx, memories, result.ConversationID, summary, run.ID)
	if err != nil {
		return JobRunHandoff{}, err
	}
	result.MessageID = messageID
	return result, nil
}

func ensureJobRunConversationMessage(ctx context.Context, memories *memory.Store, conversationID string, summary string, runID string) (string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if memories == nil || conversationID == "" {
		return "", nil
	}
	if _, err := memories.GetConversation(ctx, conversationID); err != nil {
		return "", nil
	}
	if existingID, ok, err := existingJobRunConversationMessage(ctx, memories, conversationID, runID, summary); err != nil {
		return "", err
	} else if ok {
		return existingID, nil
	}
	msg, err := memories.SaveMessage(ctx, memory.Message{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        jobRunConversationMessage(summary),
		Model:          "scheduler",
	})
	if err != nil {
		return "", err
	}
	return msg.ID, nil
}

func existingJobRunConversationMessage(ctx context.Context, memories *memory.Store, conversationID string, runID string, summary string) (string, bool, error) {
	if strings.TrimSpace(conversationID) == "" {
		return "", false, nil
	}
	messages, err := memories.ListConversationMessages(ctx, conversationID, 200)
	if err != nil {
		return "", false, err
	}
	runID = strings.TrimSpace(runID)
	expected := strings.TrimSpace(jobRunConversationMessage(summary))
	for _, msg := range messages {
		if msg.Model != "scheduler" {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if expected != "" && content == expected {
			return msg.ID, true, nil
		}
		if runID != "" && strings.Contains(content, "Run ID: `"+runID+"`.") {
			return msg.ID, true, nil
		}
	}
	return "", false, nil
}

func BuildJobRunSummary(job scheduler.Job, run scheduler.JobRun) string {
	name := strings.TrimSpace(job.Name)
	if name == "" {
		name = job.ID
	}
	status := strings.TrimSpace(run.Status)
	if status == "" {
		status = "unknown"
	}
	target := strings.TrimSpace(job.TargetType + ":" + job.TargetName)
	if strings.Trim(target, ":") == "" {
		target = "unknown"
	}
	parts := []string{
		fmt.Sprintf("Job `%s` (`%s`) finished with status `%s`.", name, job.ID, status),
		fmt.Sprintf("Target: `%s`.", target),
	}
	if run.DurationMS > 0 {
		parts = append(parts, fmt.Sprintf("Duration: %d ms.", run.DurationMS))
	}
	if strings.TrimSpace(run.Error) != "" {
		parts = append(parts, "Error: "+compactJobText(run.Error, 1200))
	} else {
		parts = append(parts, "Result: "+jobOutputSummary(run.Output))
	}
	if strings.TrimSpace(run.ID) != "" {
		parts = append(parts, "Run ID: `"+run.ID+"`.")
	}
	return strings.Join(parts, "\n")
}

func jobRunConversationMessage(summary string) string {
	return "Scheduled job result\n\n" + strings.TrimSpace(summary)
}

func jobRunImportance(run scheduler.JobRun) int {
	switch strings.TrimSpace(run.Status) {
	case scheduler.StatusFailed, scheduler.StatusTimeout:
		return 4
	default:
		return 3
	}
}

func resolveJobConversationID(job scheduler.Job, explicit string) string {
	for _, key := range []string{"conversation_id", "conversationId", "_conversation_id"} {
		if raw, ok := job.Input[key]; ok {
			if id := strings.TrimSpace(fmt.Sprint(raw)); id != "" && id != "<nil>" {
				return id
			}
		}
	}
	if id := strings.TrimSpace(explicit); id != "" {
		return id
	}
	return ""
}

func jobOutputSummary(output any) string {
	if output == nil {
		return "no output."
	}
	if text, ok := output.(string); ok {
		if summary := structuredJobOutputSummaryFromJSON(text); summary != "" {
			return compactJobText(summary, 1200)
		}
		return compactJobText(text, 1200)
	}
	if value := outputStringField(output, "summary", "message", "text"); value != "" {
		return compactJobText(value, 1200)
	}
	if value := nestedOutputStringField(output, "summary", "message", "text", "status"); value != "" {
		return compactJobText(value, 1200)
	}
	if value := outputStringField(output, "status"); value != "" {
		if detail := outputStringField(output, "extension", "runId", "runID"); detail != "" {
			return compactJobText(fmt.Sprintf("%s (%s).", value, detail), 1200)
		}
		return compactJobText(value, 1200)
	}
	data, err := json.Marshal(output)
	if err != nil {
		return compactJobText(fmt.Sprint(output), 1200)
	}
	if len(data) == 0 || string(data) == "null" {
		return "no output."
	}
	return compactJobText(string(data), 1200)
}

func structuredJobOutputSummaryFromJSON(text string) string {
	text = strings.TrimSpace(text)
	if text == "" || !strings.HasPrefix(text, "{") {
		return ""
	}
	var values map[string]any
	if err := json.Unmarshal([]byte(text), &values); err != nil {
		return ""
	}
	if value := outputStringField(values, "summary", "message", "text"); value != "" {
		return value
	}
	if value := nestedOutputStringField(values, "summary", "message", "text", "status"); value != "" {
		return value
	}
	if value := outputStringField(values, "status"); value != "" {
		return value
	}
	return ""
}

func outputStringField(output any, keys ...string) string {
	values, ok := output.(map[string]any)
	if !ok {
		return ""
	}
	for _, key := range keys {
		if value, ok := values[key]; ok {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func nestedOutputStringField(output any, keys ...string) string {
	values, ok := output.(map[string]any)
	if !ok {
		return ""
	}
	nested, ok := values["output"]
	if !ok {
		return ""
	}
	return outputStringField(nested, keys...)
}

func compactJobText(text string, max int) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if text == "" {
		return "no output."
	}
	if max > 0 && len(text) > max {
		return text[:max] + "..."
	}
	return text
}
