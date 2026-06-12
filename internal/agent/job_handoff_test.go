package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/memory"
	"yemaka/internal/scheduler"
)

func TestRecordJobRunResultSavesMemoryAndConversationMessage(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "memory.sqlite")
	memories, err := memory.Open(ctx, database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer memories.Close()
	jobs, err := scheduler.Open(ctx, database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer jobs.Close()

	conversation, err := memories.CreateConversation(ctx, "scheduled work")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	job, err := jobs.Create(ctx, scheduler.CreateInput{
		Name:         "daily note",
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "note_writer",
		Input: map[string]any{
			"conversation_id": conversation.ID,
		},
		Approved: true,
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	run, err := jobs.Run(ctx, job.ID, scheduler.RunnerFunc(func(context.Context, scheduler.Job) (any, error) {
		return map[string]any{"summary": "wrote the daily note"}, nil
	}), scheduler.RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true, PolicyMode: "safe"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	handoff, err := RecordJobRunResult(ctx, memories, job, run, "")
	if err != nil {
		t.Fatalf("RecordJobRunResult() error = %v", err)
	}
	if handoff.MemoryID == "" {
		t.Fatal("memory id is empty")
	}
	if handoff.MessageID == "" {
		t.Fatal("message id is empty")
	}
	if !strings.Contains(handoff.Summary, "wrote the daily note") {
		t.Fatalf("summary = %q, want output summary", handoff.Summary)
	}
	results, err := memories.SearchMemories(ctx, "daily note", 5)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(results) == 0 || results[0].Kind != JobRunSummaryMemoryKind {
		t.Fatalf("SearchMemories() = %+v, want job run summary", results)
	}
	messages, err := memories.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 1 || messages[0].Model != "scheduler" || !strings.Contains(messages[0].Content, "Scheduled job result") {
		t.Fatalf("conversation messages = %+v, want scheduler job summary", messages)
	}
}

func TestRecordJobRunResultIsIdempotentForSameRun(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "memory.sqlite")
	memories, err := memory.Open(ctx, database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer memories.Close()
	jobs, err := scheduler.Open(ctx, database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer jobs.Close()

	conversation, err := memories.CreateConversation(ctx, "idempotent")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	job, err := jobs.Create(ctx, scheduler.CreateInput{
		Name:         "one shot",
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetHeartbeat,
		TargetName:   "heartbeat",
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	run, err := jobs.Run(ctx, job.ID, scheduler.RunnerFunc(func(context.Context, scheduler.Job) (any, error) {
		return "ok", nil
	}), scheduler.RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true, PolicyMode: "safe"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if _, err := RecordJobRunResult(ctx, memories, job, run, conversation.ID); err != nil {
		t.Fatalf("first RecordJobRunResult() error = %v", err)
	}
	if _, err := RecordJobRunResult(ctx, memories, job, run, conversation.ID); err != nil {
		t.Fatalf("second RecordJobRunResult() error = %v", err)
	}
	messages, err := memories.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
}

func TestRecordJobRunResultPrefersJobConversationOverActiveConversation(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "memory.sqlite")
	memories, err := memory.Open(ctx, database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer memories.Close()
	jobs, err := scheduler.Open(ctx, database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer jobs.Close()

	origin, err := memories.CreateConversation(ctx, "job origin")
	if err != nil {
		t.Fatalf("CreateConversation(origin) error = %v", err)
	}
	active, err := memories.CreateConversation(ctx, "currently open chat")
	if err != nil {
		t.Fatalf("CreateConversation(active) error = %v", err)
	}
	job, err := jobs.Create(ctx, scheduler.CreateInput{
		Name:         "origin report",
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "report_writer",
		Input: map[string]any{
			"conversation_id": origin.ID,
			"task":            "write report",
		},
		Approved: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	run, err := jobs.Run(ctx, job.ID, scheduler.RunnerFunc(func(context.Context, scheduler.Job) (any, error) {
		return map[string]any{"summary": "wrote origin report"}, nil
	}), scheduler.RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true, PolicyMode: "safe"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	handoff, err := RecordJobRunResult(ctx, memories, job, run, active.ID)
	if err != nil {
		t.Fatalf("RecordJobRunResult() error = %v", err)
	}
	if handoff.ConversationID != origin.ID {
		t.Fatalf("ConversationID = %q, want job origin %q", handoff.ConversationID, origin.ID)
	}
	originMessages, err := memories.ListConversationMessages(ctx, origin.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages(origin) error = %v", err)
	}
	activeMessages, err := memories.ListConversationMessages(ctx, active.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages(active) error = %v", err)
	}
	if len(originMessages) != 1 || !strings.Contains(originMessages[0].Content, "wrote origin report") {
		t.Fatalf("origin messages = %+v, want scheduler result", originMessages)
	}
	if len(activeMessages) != 0 {
		t.Fatalf("active messages = %+v, want no scheduler result in currently open chat", activeMessages)
	}
}

func TestRecordJobRunResultAddsConversationMessageForExistingMemory(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "memory.sqlite")
	memories, err := memory.Open(ctx, database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer memories.Close()
	jobs, err := scheduler.Open(ctx, database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer jobs.Close()

	conversation, err := memories.CreateConversation(ctx, "late handoff")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	job, err := jobs.Create(ctx, scheduler.CreateInput{
		Name:         "late report",
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "report_writer",
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	run, err := jobs.Run(ctx, job.ID, scheduler.RunnerFunc(func(context.Context, scheduler.Job) (any, error) {
		return map[string]any{"summary": "wrote the late report"}, nil
	}), scheduler.RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true, PolicyMode: "safe"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	first, err := RecordJobRunResult(ctx, memories, job, run, "")
	if err != nil {
		t.Fatalf("first RecordJobRunResult() error = %v", err)
	}
	if first.MessageID != "" {
		t.Fatalf("first MessageID = %q, want empty without conversation id", first.MessageID)
	}

	second, err := RecordJobRunResult(ctx, memories, job, run, conversation.ID)
	if err != nil {
		t.Fatalf("second RecordJobRunResult() error = %v", err)
	}
	if second.MemoryID != first.MemoryID {
		t.Fatalf("MemoryID = %q, want existing %q", second.MemoryID, first.MemoryID)
	}
	if second.MessageID == "" {
		t.Fatal("second MessageID is empty, want late conversation handoff")
	}

	third, err := RecordJobRunResult(ctx, memories, job, run, conversation.ID)
	if err != nil {
		t.Fatalf("third RecordJobRunResult() error = %v", err)
	}
	if third.MessageID != second.MessageID {
		t.Fatalf("third MessageID = %q, want existing %q", third.MessageID, second.MessageID)
	}
	messages, err := memories.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 1 || !strings.Contains(messages[0].Content, "wrote the late report") {
		t.Fatalf("messages = %+v, want exactly one late scheduler handoff", messages)
	}
}

func TestRecordJobRunResultSanitizesSummaryBeforeConversationHandoff(t *testing.T) {
	ctx := context.Background()
	database := filepath.Join(t.TempDir(), "memory.sqlite")
	memories, err := memory.Open(ctx, database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer memories.Close()
	jobs, err := scheduler.Open(ctx, database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer jobs.Close()

	conversation, err := memories.CreateConversation(ctx, "secret handoff")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	job, err := jobs.Create(ctx, scheduler.CreateInput{
		Name:         "secret report",
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "report_writer",
		Input:        map[string]any{"conversation_id": conversation.ID},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	run, err := jobs.Run(ctx, job.ID, scheduler.RunnerFunc(func(context.Context, scheduler.Job) (any, error) {
		secret := "api_" + "key=sk-" + strings.Repeat("secret", 3)
		return map[string]any{"summary": secret}, nil
	}), scheduler.RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true, PolicyMode: "safe"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	handoff, err := RecordJobRunResult(ctx, memories, job, run, "")
	if err != nil {
		t.Fatalf("RecordJobRunResult() error = %v", err)
	}
	if strings.Contains(handoff.Summary, "sk-"+strings.Repeat("secret", 3)) || !strings.Contains(handoff.Summary, "[REDACTED]") {
		t.Fatalf("handoff summary was not sanitized: %q", handoff.Summary)
	}
	messages, err := memories.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(messages))
	}
	if strings.Contains(messages[0].Content, "sk-"+strings.Repeat("secret", 3)) || !strings.Contains(messages[0].Content, "[REDACTED]") {
		t.Fatalf("conversation handoff was not sanitized: %q", messages[0].Content)
	}
}

func TestBuildJobRunSummaryUsesNestedExtensionSummary(t *testing.T) {
	job := scheduler.Job{
		ID:         "job_123",
		Name:       "web monitoring",
		TargetType: scheduler.TargetExtension,
		TargetName: "web_monitoring",
	}
	run := scheduler.JobRun{
		ID:         "run_123",
		Status:     scheduler.StatusCompleted,
		DurationMS: 29,
		Output: map[string]any{
			"runId":     "run_abc",
			"extension": "web_monitoring",
			"version":   "0.1.0",
			"status":    "completed",
			"output": map[string]any{
				"ok":      true,
				"summary": "web monitoring checked the requested page",
			},
		},
	}

	summary := BuildJobRunSummary(job, run)
	if !strings.Contains(summary, "web monitoring checked the requested page") {
		t.Fatalf("summary = %q, want nested extension output summary", summary)
	}
	if strings.Contains(summary, `"runId"`) || strings.Contains(summary, `"extension"`) {
		t.Fatalf("summary leaked raw extension JSON: %q", summary)
	}
}

func TestBuildJobRunSummaryParsesJSONStringOutput(t *testing.T) {
	job := scheduler.Job{
		ID:         "job_456",
		Name:       "crawl check",
		TargetType: scheduler.TargetExtension,
		TargetName: "crawl_check",
	}
	run := scheduler.JobRun{
		ID:     "run_456",
		Status: scheduler.StatusCompleted,
		Output: `{"runId":"run_xyz","extension":"crawl_check","status":"completed","output":{"ok":true,"summary":"crawl check finished cleanly"}}`,
	}

	summary := BuildJobRunSummary(job, run)
	if !strings.Contains(summary, "crawl check finished cleanly") {
		t.Fatalf("summary = %q, want parsed JSON output summary", summary)
	}
	if strings.Contains(summary, `"runId"`) || strings.Contains(summary, `"output"`) {
		t.Fatalf("summary leaked raw JSON string: %q", summary)
	}
}
