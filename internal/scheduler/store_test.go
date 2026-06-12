package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateListEnableAndRunJob(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	job, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleInterval,
		ScheduleExpr: "1h",
		TargetType:   TargetExtension,
		TargetName:   "website_monitor",
		Input:        map[string]any{"url": "https://example.com"},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if job.Enabled {
		t.Fatal("new jobs should be disabled until explicitly enabled")
	}
	if job.NextDueAt != "2026-05-08T13:00:00Z" {
		t.Fatalf("NextDueAt = %q, want 2026-05-08T13:00:00Z", job.NextDueAt)
	}
	enabled, err := store.SetEnabled(ctx, job.ID, true)
	if err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("job should be enabled")
	}

	run, err := store.Run(ctx, job.ID, RunnerFunc(func(ctx context.Context, job Job) (any, error) {
		if job.TargetName != "website_monitor" {
			t.Fatalf("target = %q, want website_monitor", job.TargetName)
		}
		return map[string]any{"ok": true}, nil
	}), RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != StatusCompleted {
		t.Fatalf("run status = %q, want completed", run.Status)
	}
	runs, err := store.LastRuns(ctx, 10)
	if err != nil {
		t.Fatalf("LastRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].JobID != job.ID {
		t.Fatalf("runs = %+v, want one run for job", runs)
	}
	status, err := store.Status(ctx, true, 1)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.TotalJobs != 1 || status.EnabledJobs != 1 || status.ApprovedJobs != 1 || status.LastRunStatus != StatusCompleted {
		t.Fatalf("status = %+v, want one completed enabled job", status)
	}
	updated, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if updated.NextDueAt == "" {
		t.Fatal("NextDueAt should be refreshed after run")
	}
}

func TestArchiveJobDisablesAndHidesActiveJobButPreservesRuns(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	job, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleInterval,
		ScheduleExpr: "1h",
		TargetType:   TargetHeartbeat,
		TargetName:   TargetHeartbeat,
		Approved:     true,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	run, err := store.Run(ctx, job.ID, RunnerFunc(func(ctx context.Context, job Job) (any, error) {
		return map[string]any{"ok": true}, nil
	}), RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if run.Status != StatusCompleted {
		t.Fatalf("run status = %q, want completed", run.Status)
	}

	archived, err := store.Archive(ctx, job.ID)
	if err != nil {
		t.Fatalf("Archive() error = %v", err)
	}
	if archived.ArchivedAt == "" || archived.Enabled || archived.NextDueAt != "" {
		t.Fatalf("archived job = %+v, want archived disabled job without next due", archived)
	}
	listed, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("List() = %+v, want archived job hidden from active list", listed)
	}
	due, err := store.DueJobs(ctx, time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC), 10)
	if err != nil {
		t.Fatalf("DueJobs() error = %v", err)
	}
	if len(due) != 0 {
		t.Fatalf("DueJobs() = %+v, want archived job excluded", due)
	}
	status, err := store.Status(ctx, true, 1)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if status.TotalJobs != 0 || status.EnabledJobs != 0 || status.ApprovedJobs != 0 || status.ArchivedJobs != 1 {
		t.Fatalf("status = %+v, want one archived job and no active jobs", status)
	}
	runs, err := store.LastRuns(ctx, 10)
	if err != nil {
		t.Fatalf("LastRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].JobID != job.ID {
		t.Fatalf("runs = %+v, want preserved run history", runs)
	}
	if _, err := store.SetEnabled(ctx, job.ID, true); err == nil || !strings.Contains(err.Error(), "archived job") {
		t.Fatalf("SetEnabled(archived) error = %v, want archived job error", err)
	}
	if _, err := store.UpdateInput(ctx, UpdateInput{ID: job.ID, Input: map[string]any{}}); err == nil || !strings.Contains(err.Error(), "archived job") {
		t.Fatalf("UpdateInput(archived) error = %v, want archived job error", err)
	}
	if _, err := store.Approve(ctx, job.ID); err == nil || !strings.Contains(err.Error(), "archived job") {
		t.Fatalf("Approve(archived) error = %v, want archived job error", err)
	}
	if _, err := store.Run(ctx, job.ID, RunnerFunc(func(ctx context.Context, job Job) (any, error) {
		return nil, nil
	}), RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1}); err == nil || !strings.Contains(err.Error(), "archived job") {
		t.Fatalf("Run(archived) error = %v, want archived job error", err)
	}
}

func TestUnapprovedJobCannotEnableOrRun(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	job, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleManual,
		TargetType:   TargetHeartbeat,
		TargetName:   "heartbeat",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := store.SetEnabled(ctx, job.ID, true); err == nil || !strings.Contains(err.Error(), "approved") {
		t.Fatalf("SetEnabled() error = %v, want approval error", err)
	}
	_, err = store.Run(ctx, job.ID, RunnerFunc(func(ctx context.Context, job Job) (any, error) {
		return nil, nil
	}), RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1})
	if err == nil || !strings.Contains(err.Error(), "approved") {
		t.Fatalf("Run() error = %v, want approval error", err)
	}
}

func TestUpdateInputPreservesJobStateAndRunMetadata(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	job, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleInterval,
		ScheduleExpr: "1h",
		TargetType:   TargetExtension,
		TargetName:   "website_monitor",
		Input:        map[string]any{"url": "https://example.com"},
		Approved:     true,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	updated, err := store.UpdateInput(ctx, UpdateInput{
		ID:    job.ID,
		Input: map[string]any{"url": "https://example.org", "task": "check"},
	})
	if err != nil {
		t.Fatalf("UpdateInput() error = %v", err)
	}
	if !updated.Enabled || !updated.Approved || updated.NextDueAt != job.NextDueAt {
		t.Fatalf("updated job state = %+v, want enabled/approved and same next due %q", updated, job.NextDueAt)
	}
	if updated.Input["url"] != "https://example.org" || updated.Input["task"] != "check" {
		t.Fatalf("updated input = %+v", updated.Input)
	}
	loaded, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if loaded.Input["url"] != "https://example.org" || loaded.Input["task"] != "check" {
		t.Fatalf("loaded input = %+v", loaded.Input)
	}

	run, err := store.Run(ctx, job.ID, RunnerFunc(func(ctx context.Context, job Job) (any, error) {
		return nil, errors.New("validation failed")
	}), RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1})
	if err == nil || !strings.Contains(err.Error(), "validation failed") {
		t.Fatalf("Run() error = %v, want validation failure", err)
	}
	if run.JobName != "website_monitor" || run.TargetType != TargetExtension || run.TargetName != "website_monitor" || run.ScheduleType != ScheduleInterval {
		t.Fatalf("run metadata = %+v, want job target metadata", run)
	}
	output, ok := run.Output.(map[string]any)
	if !ok || output["jobRun"] == nil {
		t.Fatalf("run output = %#v, want failure jobRun metadata", run.Output)
	}
	loadedRun, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if loadedRun.TargetName != "website_monitor" || loadedRun.ScheduleType != ScheduleInterval {
		t.Fatalf("loaded run metadata = %+v", loadedRun)
	}
}

func TestValidateScheduleExpressions(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	_, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleInterval,
		ScheduleExpr: "soon",
		TargetType:   TargetExtension,
		TargetName:   "bad",
		Approved:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "duration") {
		t.Fatalf("Create(interval) error = %v, want duration error", err)
	}
	_, err = store.Create(ctx, CreateInput{
		ScheduleType: ScheduleCron,
		ScheduleExpr: "* * *",
		TargetType:   TargetExtension,
		TargetName:   "bad",
		Approved:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "five-field") {
		t.Fatalf("Create(cron) error = %v, want cron error", err)
	}
}

func TestCreateRejectsMisleadingJobTargets(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	_, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleManual,
		TargetType:   TargetExtension,
		TargetName:   "monitor https://example.com",
		Approved:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "extension job target must be a generated extension name") {
		t.Fatalf("Create(extension URL target) error = %v, want extension name guidance", err)
	}
	_, err = store.Create(ctx, CreateInput{
		ScheduleType: ScheduleInterval,
		ScheduleExpr: "1h",
		TargetType:   TargetHeartbeat,
		TargetName:   "monitor example.com",
		Approved:     true,
	})
	if err == nil || !strings.Contains(err.Error(), `heartbeat job target must be "heartbeat"`) {
		t.Fatalf("Create(heartbeat alias target) error = %v, want heartbeat target guidance", err)
	}
}

func TestValidateTargetAllowsMissingExtensionIdentifierForReviewRuns(t *testing.T) {
	if err := ValidateTarget(TargetExtension, "missing_test_extension"); err != nil {
		t.Fatalf("ValidateTarget(missing extension-shaped id) error = %v", err)
	}
}

func TestOpenMigratesOldJobsTableWithoutNextDueAt(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE jobs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			schedule_type TEXT NOT NULL,
			schedule_expr TEXT NOT NULL DEFAULT '',
			target_type TEXT NOT NULL,
			target_name TEXT NOT NULL,
			input_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 0,
			approved INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_run_at TEXT NOT NULL DEFAULT ''
		);
		INSERT INTO jobs (
			id, name, schedule_type, schedule_expr, target_type, target_name,
			input_json, enabled, approved, created_at, updated_at, last_run_at
		)
		VALUES (
			'job_old', 'old heartbeat', 'interval', '1h', 'heartbeat', 'heartbeat',
			'{}', 1, 1, '2026-05-08T12:00:00Z', '2026-05-08T12:00:00Z', ''
		);
	`)
	if err != nil {
		t.Fatalf("seed old scheduler schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close seed db: %v", err)
	}

	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open() old schema error = %v", err)
	}
	defer store.Close()

	job, err := store.Get(ctx, "job_old")
	if err != nil {
		t.Fatalf("Get() migrated job error = %v", err)
	}
	if job.NextDueAt != "2026-05-08T13:00:00Z" {
		t.Fatalf("NextDueAt = %q, want backfilled next due", job.NextDueAt)
	}
}

func TestDueJobsAndSchedulerTick(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	store.Now = func() time.Time { return time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) }
	job, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleInterval,
		ScheduleExpr: "1h",
		TargetType:   TargetHeartbeat,
		TargetName:   TargetHeartbeat,
		Approved:     true,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	none, err := store.DueJobs(ctx, time.Date(2026, 5, 8, 10, 30, 0, 0, time.UTC), 5)
	if err != nil {
		t.Fatalf("DueJobs(early) error = %v", err)
	}
	if len(none) != 0 {
		t.Fatalf("early due jobs = %+v, want none", none)
	}

	store.Now = func() time.Time { return time.Date(2026, 5, 8, 11, 0, 0, 0, time.UTC) }
	var ran []string
	loop := Loop{
		Store: store,
		Runner: RunnerFunc(func(ctx context.Context, job Job) (any, error) {
			ran = append(ran, job.ID)
			return map[string]any{"heartbeat": true}, nil
		}),
		Options: RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true},
		Now:     func() time.Time { return time.Date(2026, 5, 8, 11, 0, 0, 0, time.UTC) },
	}
	tick, err := loop.Tick(ctx)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(tick.Runs) != 1 || len(ran) != 1 || ran[0] != job.ID {
		t.Fatalf("tick = %+v ran=%+v, want one due job", tick, ran)
	}
	refreshed, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if refreshed.NextDueAt <= tick.CheckedAt {
		t.Fatalf("NextDueAt = %q should be after tick %q", refreshed.NextDueAt, tick.CheckedAt)
	}
}

func TestSchedulerTickCoalescesMissedIntervalRuns(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	store.Now = func() time.Time { return time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) }
	job, err := store.Create(ctx, CreateInput{
		ScheduleType: ScheduleInterval,
		ScheduleExpr: "1h",
		TargetType:   TargetHeartbeat,
		TargetName:   TargetHeartbeat,
		Approved:     true,
		Enabled:      true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	store.Now = func() time.Time { return time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC) }
	loop := Loop{
		Store: store,
		Runner: RunnerFunc(func(ctx context.Context, job Job) (any, error) {
			return map[string]any{"heartbeat": true}, nil
		}),
		Options: RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1, LowMemoryMode: true},
		Now:     func() time.Time { return time.Date(2026, 5, 8, 14, 0, 0, 0, time.UTC) },
	}
	tick, err := loop.Tick(ctx)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(tick.Runs) != 1 {
		t.Fatalf("runs = %d, want one coalesced run", len(tick.Runs))
	}
	if tick.MissedPolicy != "coalesce" || tick.MissedRuns != 3 {
		t.Fatalf("tick missed policy/count = %q/%d, want coalesce/3", tick.MissedPolicy, tick.MissedRuns)
	}
	refreshed, err := store.Get(ctx, job.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if refreshed.NextDueAt != "2026-05-08T15:00:00Z" {
		t.Fatalf("NextDueAt = %q, want next run after coalesced tick", refreshed.NextDueAt)
	}
}

func TestSchedulerTickLowMemoryRunsOnlyOneDueJob(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	store.Now = func() time.Time { return time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) }
	for _, name := range []string{"heartbeat_one", "heartbeat_two"} {
		if _, err := store.Create(ctx, CreateInput{
			Name:         name,
			ScheduleType: ScheduleOneTime,
			ScheduleExpr: "2026-05-08T10:00:00Z",
			TargetType:   TargetHeartbeat,
			TargetName:   TargetHeartbeat,
			Approved:     true,
			Enabled:      true,
		}); err != nil {
			t.Fatalf("Create(%s) error = %v", name, err)
		}
	}
	loop := Loop{
		Store: store,
		Runner: RunnerFunc(func(ctx context.Context, job Job) (any, error) {
			return map[string]any{"heartbeat": true}, nil
		}),
		Options: RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 4, LowMemoryMode: true},
		Now:     func() time.Time { return time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC) },
	}
	tick, err := loop.Tick(ctx)
	if err != nil {
		t.Fatalf("Tick() error = %v", err)
	}
	if len(tick.Runs) != 1 || tick.Skipped != 1 {
		t.Fatalf("tick = %+v, want one run and one skipped in low-memory mode", tick)
	}
}

func TestRetryPolicyRecordsPassiveBackoffAndRunVisibility(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	job, err := store.Create(ctx, CreateInput{
		ScheduleType:   ScheduleManual,
		TargetType:     TargetHeartbeat,
		TargetName:     TargetHeartbeat,
		Approved:       true,
		RetryPolicy:    RetryFixed,
		MaxAttempts:    3,
		BackoffSeconds: 30,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if job.RetryPolicy != RetryFixed || job.MaxAttempts != 3 || job.BackoffSeconds != 30 {
		t.Fatalf("job retry fields = %+v, want fixed retry policy", job)
	}

	run, err := store.Run(ctx, job.ID, RunnerFunc(func(ctx context.Context, job Job) (any, error) {
		return map[string]any{"partial": true}, errors.New("temporary failure")
	}), RunOptions{Enabled: true, TimeoutSeconds: 5, MaxParallel: 1})
	if err == nil || !strings.Contains(err.Error(), "temporary failure") {
		t.Fatalf("Run() error = %v, want temporary failure", err)
	}
	if run.Status != StatusFailed || run.Attempt != 1 || run.RetryDueAt != "2026-05-08T12:00:30Z" {
		t.Fatalf("run = %+v, want failed first attempt with passive retry due", run)
	}
	runs, err := store.RunsForJob(ctx, job.ID, 10)
	if err != nil {
		t.Fatalf("RunsForJob() error = %v", err)
	}
	if len(runs) != 1 || runs[0].ID != run.ID || runs[0].RetryDueAt != run.RetryDueAt {
		t.Fatalf("RunsForJob() = %+v, want visible retry due run", runs)
	}
	loaded, err := store.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if loaded.Attempt != 1 || loaded.RetryDueAt != run.RetryDueAt {
		t.Fatalf("GetRun() = %+v, want persisted attempt/retry due", loaded)
	}
}

func TestRetryPolicyValidationAndExponentialBackoff(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	_, err := store.Create(ctx, CreateInput{
		ScheduleType:   ScheduleManual,
		TargetType:     TargetHeartbeat,
		TargetName:     TargetHeartbeat,
		Approved:       true,
		RetryPolicy:    RetryExponential,
		MaxAttempts:    1,
		BackoffSeconds: 30,
	})
	if err == nil || !strings.Contains(err.Error(), "retry max attempts") {
		t.Fatalf("Create() error = %v, want retry attempts validation", err)
	}
	job := Job{RetryPolicy: RetryExponential, MaxAttempts: 4, BackoffSeconds: 30}
	due := RetryDueAt(job, 3, time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 5, 8, 12, 2, 0, 0, time.UTC); !due.Equal(want) {
		t.Fatalf("RetryDueAt() = %s, want %s", due, want)
	}
	if due := RetryDueAt(job, 4, time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)); !due.IsZero() {
		t.Fatalf("RetryDueAt(exhausted) = %s, want zero", due)
	}
}

func TestNextCronTime(t *testing.T) {
	next, err := NextCronTime("*/15 9-17 * * 1-5", time.Date(2026, 5, 8, 9, 14, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("NextCronTime() error = %v", err)
	}
	if want := time.Date(2026, 5, 8, 9, 15, 0, 0, time.UTC); !next.Equal(want) {
		t.Fatalf("next = %s, want %s", next, want)
	}
}

func TestMissedRunCountForCron(t *testing.T) {
	job := Job{
		ScheduleType: ScheduleCron,
		ScheduleExpr: "*/15 * * * *",
		NextDueAt:    "2026-05-08T10:15:00Z",
	}
	count := MissedRunCount(job, time.Date(2026, 5, 8, 11, 0, 0, 0, time.UTC), 1000)
	if count != 3 {
		t.Fatalf("MissedRunCount() = %d, want 3", count)
	}
}

func testStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	store.Now = func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) }
	return store
}
