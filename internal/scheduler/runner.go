package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"yemaka/internal/safety"
)

type Runner interface {
	RunJob(ctx context.Context, job Job) (any, error)
}

type RunnerFunc func(ctx context.Context, job Job) (any, error)

func (fn RunnerFunc) RunJob(ctx context.Context, job Job) (any, error) {
	return fn(ctx, job)
}

var runMu sync.Mutex

func (s *Store) Run(ctx context.Context, id string, runner Runner, options RunOptions) (JobRun, error) {
	if runner == nil {
		return JobRun{}, fmt.Errorf("job runner is required")
	}
	if err := validateRunOptions(options); err != nil {
		return JobRun{}, err
	}
	if options.LowMemoryMode || options.MaxParallel <= 1 {
		runMu.Lock()
		defer runMu.Unlock()
	}

	job, err := s.Get(ctx, id)
	if err != nil {
		return JobRun{}, err
	}
	if !job.Approved {
		return JobRun{}, fmt.Errorf("job must be approved before it can run")
	}
	if job.ArchivedAt != "" {
		return JobRun{}, fmt.Errorf("archived job cannot run")
	}
	attempt, err := s.nextAttempt(ctx, job)
	if err != nil {
		return JobRun{}, err
	}
	decision := safety.EvaluatePolicy(safety.PolicyRequest{
		Domain:       safety.DomainAutomation,
		Action:       safety.ActionSchedule,
		Level:        safety.LevelScheduled,
		PolicyMode:   options.PolicyMode,
		Actor:        "scheduler",
		Resource:     job.ID,
		Enabled:      options.Enabled,
		TaskApproved: job.Approved,
		Scheduled:    job.Approved,
	})
	if !decision.Allowed {
		return JobRun{}, fmt.Errorf("scheduler policy blocked job: %s", decision.Reason)
	}
	if decision.RequiresConfirmation && !job.Approved {
		return JobRun{}, fmt.Errorf("scheduler policy requires approval: %s", decision.Reason)
	}

	timeout := time.Duration(options.TimeoutSeconds) * time.Second
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startedWall := time.Now()
	started := s.now()
	run := JobRun{
		ID:           "run_" + uuid.NewString(),
		JobID:        job.ID,
		JobName:      job.Name,
		TargetType:   job.TargetType,
		TargetName:   job.TargetName,
		ScheduleType: job.ScheduleType,
		Status:       StatusCompleted,
		StartedAt:    started.UTC().Format(time.RFC3339),
		Attempt:      attempt,
	}
	output, runErr := runner.RunJob(runCtx, job)
	run.DurationMS = time.Since(startedWall).Milliseconds()
	run.FinishedAt = s.now().UTC().Format(time.RFC3339)
	run.Output = output
	if runCtx.Err() == context.DeadlineExceeded {
		run.Status = StatusTimeout
		run.Error = "job timed out"
		run.RetryDueAt = formatOptionalTime(RetryDueAt(job, run.Attempt, parseTimeOrZero(run.FinishedAt)))
		run.Output = jobRunFailureOutput(job, run, output)
		_ = s.recordRun(context.Background(), run)
		_, _ = s.RefreshNextDue(context.Background(), job.ID)
		return run, errors.New(run.Error)
	}
	if runErr != nil {
		run.Status = StatusFailed
		run.Error = runErr.Error()
		run.RetryDueAt = formatOptionalTime(RetryDueAt(job, run.Attempt, parseTimeOrZero(run.FinishedAt)))
		run.Output = jobRunFailureOutput(job, run, output)
		_ = s.recordRun(context.Background(), run)
		_, _ = s.RefreshNextDue(context.Background(), job.ID)
		return run, runErr
	}
	if run.Output == nil {
		run.Output = map[string]any{}
	}
	if err := s.recordRun(ctx, run); err != nil {
		return run, err
	}
	_, _ = s.RefreshNextDue(ctx, job.ID)
	return run, nil
}

func (s *Store) nextAttempt(ctx context.Context, job Job) (int, error) {
	if normalizeRetryPolicy(job.RetryPolicy) == RetryNone {
		return 1, nil
	}
	runs, err := s.RunsForJob(ctx, job.ID, 1)
	if err != nil {
		return 0, err
	}
	if len(runs) == 0 || runs[0].Status == StatusCompleted {
		return 1, nil
	}
	attempt := runs[0].Attempt
	if attempt <= 0 {
		attempt = 1
	}
	return attempt + 1, nil
}

func validateRunOptions(options RunOptions) error {
	if !options.Enabled {
		return fmt.Errorf("scheduler is disabled")
	}
	if options.TimeoutSeconds <= 0 {
		return fmt.Errorf("job timeout must be greater than zero")
	}
	if options.MaxParallel <= 0 {
		return fmt.Errorf("max parallel jobs must be greater than zero")
	}
	return nil
}

func jobRunFailureOutput(job Job, run JobRun, output any) any {
	detail := map[string]any{
		"jobId":        job.ID,
		"jobName":      job.Name,
		"targetType":   job.TargetType,
		"targetName":   job.TargetName,
		"scheduleType": job.ScheduleType,
		"attempt":      run.Attempt,
		"status":       run.Status,
		"error":        run.Error,
	}
	if run.RetryDueAt != "" {
		detail["retryDueAt"] = run.RetryDueAt
	}
	if run.DurationMS > 0 {
		detail["durationMs"] = run.DurationMS
	}
	if output == nil {
		return map[string]any{"jobRun": detail}
	}
	if existing, ok := output.(map[string]any); ok {
		if _, exists := existing["jobRun"]; !exists {
			existing["jobRun"] = detail
		}
		return existing
	}
	return map[string]any{
		"output": output,
		"jobRun": detail,
	}
}
