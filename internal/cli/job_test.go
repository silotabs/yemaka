package cli

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/agent"
	"yemaka/internal/extensions"
	"yemaka/internal/memory"
	"yemaka/internal/scheduler"
)

func TestRunJobCreateListRunExtension(t *testing.T) {
	app := newExtensionTestApp(t)
	store, err := memory.Open(context.Background(), app.config.Memory.Database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	app.store = store
	writeCLIExtension(t, filepath.Join(app.profile.GeneratedExtensions, "website_monitor"))
	registerCLIExtensionForTest(t, app)

	var out bytes.Buffer
	err = runJob(context.Background(), app, []string{"create", "interval", "website_monitor", "--every", "1h"}, &out)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("create without approval error = %v, want --yes", err)
	}

	out.Reset()
	if err := runJob(context.Background(), app, []string{"create", "interval", "website_monitor", "--every", "1h", "--yes"}, &out); err != nil {
		t.Fatalf("runJob(create) error = %v", err)
	}
	if !strings.Contains(out.String(), `"targetName": "website_monitor"`) || !strings.Contains(out.String(), `"enabled": false`) {
		t.Fatalf("create output = %q", out.String())
	}

	out.Reset()
	if err := runJob(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runJob(list) error = %v", err)
	}
	fields := strings.Fields(out.String())
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "job_") {
		t.Fatalf("list output = %q, want job id", out.String())
	}
	jobID := fields[0]

	out.Reset()
	if err := runJob(context.Background(), app, []string{"run", jobID}, &out); err != nil {
		t.Fatalf("runJob(run) error = %v", err)
	}
	if !strings.Contains(out.String(), `"status": "completed"`) || !strings.Contains(out.String(), "cli extension") {
		t.Fatalf("run output = %q, want completed extension run", out.String())
	}
	memories, err := store.SearchMemories(context.Background(), "cli extension", 5)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(memories) == 0 || memories[0].Kind != agent.JobRunSummaryMemoryKind {
		t.Fatalf("memories = %+v, want job run summary", memories)
	}
}

func TestRunJobArchiveHidesJobFromActiveList(t *testing.T) {
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runJob(context.Background(), app, []string{"create", "manual", "heartbeat", "--target-type", "heartbeat", "--yes"}, &out); err != nil {
		t.Fatalf("runJob(create heartbeat) error = %v", err)
	}
	out.Reset()
	if err := runJob(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runJob(list) error = %v", err)
	}
	fields := strings.Fields(out.String())
	if len(fields) == 0 || !strings.HasPrefix(fields[0], "job_") {
		t.Fatalf("list output = %q, want job id", out.String())
	}
	jobID := fields[0]

	out.Reset()
	err := runJob(context.Background(), app, []string{"archive", jobID}, &out)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("runJob(archive without yes) error = %v, want --yes guidance", err)
	}

	out.Reset()
	if err := runJob(context.Background(), app, []string{"archive", jobID, "--yes"}, &out); err != nil {
		t.Fatalf("runJob(archive) error = %v", err)
	}
	if !strings.Contains(out.String(), `"archivedAt":`) || !strings.Contains(out.String(), `"enabled": false`) {
		t.Fatalf("archive output = %q, want archived disabled job", out.String())
	}

	out.Reset()
	if err := runJob(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runJob(list after archive) error = %v", err)
	}
	if !strings.Contains(out.String(), "No jobs registered.") {
		t.Fatalf("list after archive = %q, want no active jobs", out.String())
	}

	out.Reset()
	if err := runJob(context.Background(), app, []string{"status"}, &out); err != nil {
		t.Fatalf("runJob(status) error = %v", err)
	}
	if !strings.Contains(out.String(), `"archivedJobs": 1`) || !strings.Contains(out.String(), `"totalJobs": 0`) {
		t.Fatalf("status output = %q, want one archived job and no active jobs", out.String())
	}

	out.Reset()
	err = runJob(context.Background(), app, []string{"run", jobID}, &out)
	if err == nil || !strings.Contains(err.Error(), "archived job") {
		t.Fatalf("runJob(run archived) error = %v, want archived job error", err)
	}
}

func TestRunJobCreateRejectsKnownExtensionInvalidInput(t *testing.T) {
	app := newExtensionTestApp(t)
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	store := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	if _, err := store.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	var out bytes.Buffer
	err := runJob(context.Background(), app, []string{"create", "manual", "needs_task", "--yes"}, &out)
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("runJob(create invalid extension input) error = %v, want task requirement", err)
	}
}

func TestRunJobEnableRejectsKnownExtensionInvalidStoredInput(t *testing.T) {
	app := newExtensionTestApp(t)
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	extensionStore := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	if _, err := extensionStore.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_enable",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scheduleStore, err := scheduler.Open(context.Background(), app.profile.Database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer scheduleStore.Close()
	job, err := scheduleStore.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_enable",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var out bytes.Buffer
	err = runJob(context.Background(), app, []string{"enable", job.ID}, &out)
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("runJob(enable invalid extension input) error = %v, want task requirement", err)
	}
}

func TestRunJobUpdateInputValidatesAndListsInvalidStoredInput(t *testing.T) {
	app := newExtensionTestApp(t)
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	extensionStore := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	if _, err := extensionStore.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_update",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scheduleStore, err := scheduler.Open(context.Background(), app.profile.Database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	job, err := scheduleStore.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_update",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	scheduleStore.Close()

	var out bytes.Buffer
	if err := runJob(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runJob(list) error = %v", err)
	}
	if !strings.Contains(out.String(), "input_invalid") || !strings.Contains(out.String(), "input.task is required") {
		t.Fatalf("list output = %q, want invalid input details", out.String())
	}
	out.Reset()
	if err := runJob(context.Background(), app, []string{"status"}, &out); err != nil {
		t.Fatalf("runJob(status) error = %v", err)
	}
	if !strings.Contains(out.String(), `"invalidInputJobs": 1`) {
		t.Fatalf("status output = %q, want invalid input count", out.String())
	}
	out.Reset()
	err = runJob(context.Background(), app, []string{"update-input", job.ID, "--input-json", `{}`, "--yes"}, &out)
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("runJob(update-input invalid) error = %v, want task requirement", err)
	}
	out.Reset()
	if err := runJob(context.Background(), app, []string{"update-input", job.ID, "--input-json", `{"task":"scheduled cleanup"}`, "--yes"}, &out); err != nil {
		t.Fatalf("runJob(update-input valid) error = %v", err)
	}
	if !strings.Contains(out.String(), `"inputStatus": "valid"`) || !strings.Contains(out.String(), `"task": "scheduled cleanup"`) || strings.Contains(out.String(), `"enabled": true`) {
		t.Fatalf("update output = %q, want valid disabled updated job", out.String())
	}
}

func TestRunJobTickRunsDueExtension(t *testing.T) {
	app := newExtensionTestApp(t)
	writeCLIExtension(t, filepath.Join(app.profile.GeneratedExtensions, "website_monitor"))
	registerCLIExtensionForTest(t, app)

	var out bytes.Buffer
	if err := runJob(context.Background(), app, []string{"create", "one_time", "website_monitor", "--at", "2000-01-01T00:00:00Z", "--enable", "--yes"}, &out); err != nil {
		t.Fatalf("runJob(create one_time) error = %v", err)
	}

	out.Reset()
	if err := runJob(context.Background(), app, []string{"tick"}, &out); err != nil {
		t.Fatalf("runJob(tick) error = %v", err)
	}
	if !strings.Contains(out.String(), `"status": "completed"`) || !strings.Contains(out.String(), "cli extension") {
		t.Fatalf("tick output = %q, want completed due extension run", out.String())
	}
}

func TestRunJobLoopRunsDueExtensionUntilContextDone(t *testing.T) {
	app := newExtensionTestApp(t)
	writeCLIExtension(t, filepath.Join(app.profile.GeneratedExtensions, "website_monitor"))
	registerCLIExtensionForTest(t, app)

	var out bytes.Buffer
	if err := runJob(context.Background(), app, []string{"create", "one_time", "website_monitor", "--at", "2000-01-01T00:00:00Z", "--enable", "--yes"}, &out); err != nil {
		t.Fatalf("runJob(create one_time) error = %v", err)
	}

	out.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := runJob(ctx, app, []string{"loop", "--interval=1h"}, cancelAfterFirstWrite{target: &out, cancel: cancel})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runJob(loop) error = %v, want context canceled", err)
	}
	if !strings.Contains(out.String(), `"runs"`) || !strings.Contains(out.String(), `"status": "completed"`) {
		t.Fatalf("loop output = %q, want completed due extension run", out.String())
	}
}

func TestRunHeartbeatLoopRecordsUntilContextDone(t *testing.T) {
	app := newExtensionTestApp(t)
	store, err := memory.Open(context.Background(), app.config.Memory.Database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	app.store = store
	app.runtime = &modelCommandRuntime{}

	var out bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = runHeartbeat(ctx, app, []string{"loop", "--interval=1h"}, cancelAfterFirstWrite{target: &out, cancel: cancel})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runHeartbeat(loop) error = %v, want context canceled", err)
	}
	if !strings.Contains(out.String(), `"overall"`) || !strings.Contains(out.String(), `"checks"`) {
		t.Fatalf("heartbeat loop output = %q, want report JSON", out.String())
	}
}

type cancelAfterFirstWrite struct {
	target *bytes.Buffer
	cancel context.CancelFunc
}

func (w cancelAfterFirstWrite) Write(data []byte) (int, error) {
	n, err := w.target.Write(data)
	w.cancel()
	return n, err
}
