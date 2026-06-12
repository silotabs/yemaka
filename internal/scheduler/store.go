package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

type Store struct {
	db  *sql.DB
	Now func() time.Time
}

var extensionJobTargetNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

func Open(ctx context.Context, databasePath string) (*Store, error) {
	if strings.TrimSpace(databasePath) == "" {
		return nil, fmt.Errorf("scheduler database path is empty")
	}
	if err := os.MkdirAll(filepath.Dir(databasePath), 0o755); err != nil {
		return nil, fmt.Errorf("create scheduler database directory: %w", err)
	}
	db, err := sql.Open("sqlite", databasePath)
	if err != nil {
		return nil, fmt.Errorf("open scheduler sqlite database: %w", err)
	}
	store := &Store{db: db, Now: time.Now}
	if _, err := db.ExecContext(ctx, "PRAGMA busy_timeout=5000"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set scheduler busy timeout: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable scheduler WAL: %w", err)
	}
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("enable scheduler foreign keys: %w", err)
	}
	if err := store.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Create(ctx context.Context, input CreateInput) (Job, error) {
	if err := validateCreateInput(input); err != nil {
		return Job{}, err
	}
	now := s.timestamp()
	if input.Input == nil {
		input.Input = map[string]any{}
	}
	input.RetryPolicy = normalizeRetryPolicy(input.RetryPolicy)
	if err := validateRetryPolicy(input.RetryPolicy, input.MaxAttempts, input.BackoffSeconds); err != nil {
		return Job{}, err
	}
	inputJSON, err := json.Marshal(input.Input)
	if err != nil {
		return Job{}, fmt.Errorf("encode job input: %w", err)
	}
	job := Job{
		ID:             "job_" + uuid.NewString(),
		Name:           strings.TrimSpace(input.Name),
		ScheduleType:   strings.TrimSpace(input.ScheduleType),
		ScheduleExpr:   strings.TrimSpace(input.ScheduleExpr),
		TargetType:     strings.TrimSpace(input.TargetType),
		TargetName:     strings.TrimSpace(input.TargetName),
		Input:          input.Input,
		Enabled:        input.Enabled,
		Approved:       input.Approved,
		RetryPolicy:    input.RetryPolicy,
		MaxAttempts:    input.MaxAttempts,
		BackoffSeconds: input.BackoffSeconds,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	job.NextDueAt = formatOptionalTime(NextDue(job, parseTimeOrZero(now)))
	if job.Name == "" {
		job.Name = job.TargetName
	}
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO jobs (
			id, name, schedule_type, schedule_expr, target_type, target_name,
			input_json, enabled, approved, retry_policy, max_attempts, backoff_seconds,
			created_at, updated_at, last_run_at, next_due_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', ?)
	`, job.ID, job.Name, job.ScheduleType, job.ScheduleExpr, job.TargetType, job.TargetName, string(inputJSON), boolInt(job.Enabled), boolInt(job.Approved), job.RetryPolicy, job.MaxAttempts, job.BackoffSeconds, job.CreatedAt, job.UpdatedAt, job.NextDueAt)
	if err != nil {
		return Job{}, fmt.Errorf("create job: %w", err)
	}
	return job, nil
}

func (s *Store) List(ctx context.Context) ([]Job, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, schedule_type, schedule_expr, target_type, target_name,
		       input_json, enabled, approved, retry_policy, max_attempts, backoff_seconds,
		       created_at, updated_at, last_run_at, next_due_at, archived_at
		FROM jobs
		WHERE archived_at = ''
		ORDER BY created_at DESC, name ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("list jobs: %w", err)
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *Store) Get(ctx context.Context, id string) (Job, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Job{}, fmt.Errorf("job id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, schedule_type, schedule_expr, target_type, target_name,
		       input_json, enabled, approved, retry_policy, max_attempts, backoff_seconds,
		       created_at, updated_at, last_run_at, next_due_at, archived_at
		FROM jobs
		WHERE id = ?
	`, id)
	job, err := scanJob(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return Job{}, fmt.Errorf("job not found: %s", id)
		}
		return Job{}, err
	}
	return job, nil
}

func (s *Store) SetEnabled(ctx context.Context, id string, enabled bool) (Job, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if enabled && !job.Approved {
		return Job{}, fmt.Errorf("job must be approved before it can be enabled")
	}
	if job.ArchivedAt != "" {
		return Job{}, fmt.Errorf("archived job cannot be enabled")
	}
	now := s.timestamp()
	_, err = s.db.ExecContext(ctx, `
		UPDATE jobs
		SET enabled = ?, updated_at = ?, next_due_at = ?
		WHERE id = ?
	`, boolInt(enabled), now, formatOptionalTime(NextDue(job, parseTimeOrZero(now))), job.ID)
	if err != nil {
		return Job{}, fmt.Errorf("update job enabled state: %w", err)
	}
	job.Enabled = enabled
	job.UpdatedAt = now
	job.NextDueAt = formatOptionalTime(NextDue(job, parseTimeOrZero(now)))
	return job, nil
}

func (s *Store) Approve(ctx context.Context, id string) (Job, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if job.ArchivedAt != "" {
		return Job{}, fmt.Errorf("archived job cannot be approved")
	}
	now := s.timestamp()
	_, err = s.db.ExecContext(ctx, `
		UPDATE jobs
		SET approved = 1, updated_at = ?
		WHERE id = ?
	`, now, job.ID)
	if err != nil {
		return Job{}, fmt.Errorf("approve job: %w", err)
	}
	job.Approved = true
	job.UpdatedAt = now
	return job, nil
}

func (s *Store) UpdateInput(ctx context.Context, input UpdateInput) (Job, error) {
	id := strings.TrimSpace(input.ID)
	if id == "" {
		return Job{}, fmt.Errorf("job id is required")
	}
	job, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if job.ArchivedAt != "" {
		return Job{}, fmt.Errorf("archived job input cannot be updated")
	}
	if err := ValidateTarget(job.TargetType, job.TargetName); err != nil {
		return Job{}, err
	}
	if input.Input == nil {
		input.Input = map[string]any{}
	}
	inputJSON, err := json.Marshal(input.Input)
	if err != nil {
		return Job{}, fmt.Errorf("encode job input: %w", err)
	}
	now := s.timestamp()
	if _, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET input_json = ?, updated_at = ?
		WHERE id = ?
	`, string(inputJSON), now, job.ID); err != nil {
		return Job{}, fmt.Errorf("update job input: %w", err)
	}
	job.Input = input.Input
	job.UpdatedAt = now
	return job, nil
}

func (s *Store) DueJobs(ctx context.Context, now time.Time, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 1
	}
	nowText := now.UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, schedule_type, schedule_expr, target_type, target_name,
		       input_json, enabled, approved, retry_policy, max_attempts, backoff_seconds,
		       created_at, updated_at, last_run_at, next_due_at, archived_at
		FROM jobs
		WHERE enabled = 1
		  AND approved = 1
		  AND archived_at = ''
		  AND schedule_type != ?
		  AND next_due_at != ''
		  AND next_due_at <= ?
		ORDER BY next_due_at ASC, created_at ASC
		LIMIT ?
	`, ScheduleManual, nowText, limit)
	if err != nil {
		return nil, fmt.Errorf("list due jobs: %w", err)
	}
	defer rows.Close()
	return scanJobs(rows)
}

func (s *Store) RefreshNextDue(ctx context.Context, id string) (Job, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if job.ArchivedAt != "" {
		return job, nil
	}
	next := formatOptionalTime(NextDue(job, s.now()))
	now := s.timestamp()
	_, err = s.db.ExecContext(ctx, `
		UPDATE jobs
		SET next_due_at = ?, updated_at = ?
		WHERE id = ?
	`, next, now, job.ID)
	if err != nil {
		return Job{}, fmt.Errorf("update job next due: %w", err)
	}
	job.NextDueAt = next
	job.UpdatedAt = now
	return job, nil
}

func (s *Store) Archive(ctx context.Context, id string) (Job, error) {
	job, err := s.Get(ctx, id)
	if err != nil {
		return Job{}, err
	}
	if job.ArchivedAt != "" {
		return job, nil
	}
	now := s.timestamp()
	_, err = s.db.ExecContext(ctx, `
		UPDATE jobs
		SET enabled = 0, archived_at = ?, updated_at = ?, next_due_at = ''
		WHERE id = ?
	`, now, now, job.ID)
	if err != nil {
		return Job{}, fmt.Errorf("archive job: %w", err)
	}
	job.Enabled = false
	job.ArchivedAt = now
	job.UpdatedAt = now
	job.NextDueAt = ""
	return job, nil
}

func (s *Store) LastRuns(ctx context.Context, limit int) ([]JobRun, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, job_id, job_name, target_type, target_name, schedule_type, status, output_json, started_at, finished_at, duration_ms, error, attempt, retry_due_at
		FROM job_runs
		ORDER BY started_at DESC, id DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list job runs: %w", err)
	}
	defer rows.Close()
	return scanRuns(rows)
}

func (s *Store) RunsForJob(ctx context.Context, jobID string, limit int) ([]JobRun, error) {
	jobID = strings.TrimSpace(jobID)
	if jobID == "" {
		return nil, fmt.Errorf("job id is required")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, job_id, job_name, target_type, target_name, schedule_type, status, output_json, started_at, finished_at, duration_ms, error, attempt, retry_due_at
		FROM job_runs
		WHERE job_id = ?
		ORDER BY started_at DESC, id DESC
		LIMIT ?
	`, jobID, limit)
	if err != nil {
		return nil, fmt.Errorf("list job runs for job: %w", err)
	}
	defer rows.Close()
	return scanRuns(rows)
}

func (s *Store) GetRun(ctx context.Context, id string) (JobRun, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return JobRun{}, fmt.Errorf("job run id is required")
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, job_id, job_name, target_type, target_name, schedule_type, status, output_json, started_at, finished_at, duration_ms, error, attempt, retry_due_at
		FROM job_runs
		WHERE id = ?
	`, id)
	run, err := scanRun(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return JobRun{}, fmt.Errorf("job run not found: %s", id)
		}
		return JobRun{}, err
	}
	return run, nil
}

func (s *Store) Status(ctx context.Context, enabled bool, maxParallel int) (Status, error) {
	var total, enabledJobs, approvedJobs int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE archived_at = ''`).Scan(&total); err != nil {
		return Status{}, fmt.Errorf("count jobs: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE archived_at = '' AND enabled = 1`).Scan(&enabledJobs); err != nil {
		return Status{}, fmt.Errorf("count enabled jobs: %w", err)
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE archived_at = '' AND approved = 1`).Scan(&approvedJobs); err != nil {
		return Status{}, fmt.Errorf("count approved jobs: %w", err)
	}
	var archivedJobs int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jobs WHERE archived_at != ''`).Scan(&archivedJobs); err != nil {
		return Status{}, fmt.Errorf("count archived jobs: %w", err)
	}
	status := Status{
		Enabled:         enabled,
		TotalJobs:       total,
		EnabledJobs:     enabledJobs,
		ApprovedJobs:    approvedJobs,
		ArchivedJobs:    archivedJobs,
		MaxParallelJobs: maxParallel,
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT status, finished_at
		FROM job_runs
		ORDER BY started_at DESC, id DESC
		LIMIT 1
	`)
	if err := row.Scan(&status.LastRunStatus, &status.LastRunAt); err != nil && err != sql.ErrNoRows {
		return Status{}, fmt.Errorf("load last job run: %w", err)
	}
	return status, nil
}

func (s *Store) recordRun(ctx context.Context, run JobRun) error {
	outputJSON, err := json.Marshal(run.Output)
	if err != nil {
		return fmt.Errorf("encode job run output: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO job_runs (
			id, job_id, job_name, target_type, target_name, schedule_type, status, output_json, started_at, finished_at, duration_ms, error, attempt, retry_due_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, run.ID, run.JobID, run.JobName, run.TargetType, run.TargetName, run.ScheduleType, run.Status, string(outputJSON), run.StartedAt, run.FinishedAt, run.DurationMS, run.Error, run.Attempt, run.RetryDueAt); err != nil {
		return fmt.Errorf("record job run: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `
		UPDATE jobs
		SET last_run_at = ?, updated_at = ?
		WHERE id = ?
	`, run.FinishedAt, run.FinishedAt, run.JobID); err != nil {
		return fmt.Errorf("update job last run: %w", err)
	}
	return nil
}

func (s *Store) migrate(ctx context.Context) error {
	tableStatements := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			schedule_type TEXT NOT NULL,
			schedule_expr TEXT NOT NULL DEFAULT '',
			target_type TEXT NOT NULL,
			target_name TEXT NOT NULL,
			input_json TEXT NOT NULL DEFAULT '{}',
			enabled INTEGER NOT NULL DEFAULT 0,
			approved INTEGER NOT NULL DEFAULT 0,
			retry_policy TEXT NOT NULL DEFAULT 'none',
			max_attempts INTEGER NOT NULL DEFAULT 0,
			backoff_seconds INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			last_run_at TEXT NOT NULL DEFAULT '',
			next_due_at TEXT NOT NULL DEFAULT '',
			archived_at TEXT NOT NULL DEFAULT ''
		);`,
		`CREATE TABLE IF NOT EXISTS job_runs (
			id TEXT PRIMARY KEY,
			job_id TEXT NOT NULL,
			job_name TEXT NOT NULL DEFAULT '',
			target_type TEXT NOT NULL DEFAULT '',
			target_name TEXT NOT NULL DEFAULT '',
			schedule_type TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			output_json TEXT NOT NULL,
			started_at TEXT NOT NULL,
			finished_at TEXT NOT NULL,
			duration_ms INTEGER NOT NULL DEFAULT 0,
			error TEXT NOT NULL DEFAULT '',
			attempt INTEGER NOT NULL DEFAULT 1,
			retry_due_at TEXT NOT NULL DEFAULT '',
			FOREIGN KEY(job_id) REFERENCES jobs(id) ON DELETE CASCADE
		);`,
	}
	for index, statement := range tableStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply scheduler table migration %d: %w", index+1, err)
		}
	}
	if err := s.ensureColumn(ctx, "jobs", "next_due_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "jobs", "archived_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "jobs", "retry_policy", "TEXT NOT NULL DEFAULT 'none'"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "jobs", "max_attempts", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "jobs", "backoff_seconds", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "job_runs", "attempt", "INTEGER NOT NULL DEFAULT 1"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "job_runs", "retry_due_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "job_runs", "job_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "job_runs", "target_type", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "job_runs", "target_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "job_runs", "schedule_type", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	indexStatements := []string{
		`CREATE INDEX IF NOT EXISTS idx_jobs_enabled ON jobs(enabled);`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_next_due_at ON jobs(next_due_at);`,
		`CREATE INDEX IF NOT EXISTS idx_job_runs_job_id ON job_runs(job_id);`,
		`CREATE INDEX IF NOT EXISTS idx_job_runs_started_at ON job_runs(started_at);`,
	}
	for index, statement := range indexStatements {
		if _, err := s.db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply scheduler index migration %d: %w", index+1, err)
		}
	}
	return s.backfillNextDue(ctx)
}

func (s *Store) backfillNextDue(ctx context.Context) error {
	jobs, err := s.List(ctx)
	if err != nil {
		return err
	}
	now := s.now()
	for _, job := range jobs {
		if strings.TrimSpace(job.NextDueAt) != "" {
			continue
		}
		next := formatOptionalTime(NextDue(job, now))
		if next == "" {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE jobs SET next_due_at = ? WHERE id = ?`, next, job.ID); err != nil {
			return fmt.Errorf("backfill job next due: %w", err)
		}
	}
	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &pk); err != nil {
			return fmt.Errorf("scan %s column: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read %s columns: %w", table, err)
	}
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return nil
}

func validateCreateInput(input CreateInput) error {
	scheduleType := strings.TrimSpace(input.ScheduleType)
	targetType := strings.TrimSpace(input.TargetType)
	targetName := strings.TrimSpace(input.TargetName)
	if scheduleType == "" {
		return fmt.Errorf("schedule type is required")
	}
	if targetType == "" {
		return fmt.Errorf("target type is required")
	}
	if targetName == "" {
		return fmt.Errorf("target name is required")
	}
	if err := ValidateTarget(targetType, targetName); err != nil {
		return err
	}
	switch scheduleType {
	case ScheduleManual:
	case ScheduleInterval:
		if _, err := time.ParseDuration(strings.TrimSpace(input.ScheduleExpr)); err != nil {
			return fmt.Errorf("interval jobs require a valid --every duration: %w", err)
		}
	case ScheduleCron:
		if err := validateCronExpression(input.ScheduleExpr); err != nil {
			return err
		}
	case ScheduleOneTime:
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(input.ScheduleExpr)); err != nil {
			return fmt.Errorf("one_time jobs require an RFC3339 schedule expression: %w", err)
		}
	default:
		return fmt.Errorf("unknown schedule type %q", scheduleType)
	}
	return nil
}

func ValidateTarget(targetType, targetName string) error {
	targetType = strings.TrimSpace(targetType)
	targetName = strings.TrimSpace(targetName)
	if targetType == "" {
		return fmt.Errorf("target type is required")
	}
	if targetName == "" {
		return fmt.Errorf("target name is required")
	}
	switch targetType {
	case TargetExtension:
		if !extensionJobTargetNamePattern.MatchString(targetName) {
			return fmt.Errorf("extension job target must be a generated extension name matching %s; generate and register the extension first, then schedule that extension", extensionJobTargetNamePattern.String())
		}
		return nil
	case TargetHeartbeat:
		if targetName != TargetHeartbeat {
			return fmt.Errorf("heartbeat job target must be %q", TargetHeartbeat)
		}
		return nil
	default:
		return fmt.Errorf("unknown job target type %q", targetType)
	}
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (Job, error) {
	var job Job
	var inputJSON string
	var enabled, approved int
	if err := row.Scan(
		&job.ID,
		&job.Name,
		&job.ScheduleType,
		&job.ScheduleExpr,
		&job.TargetType,
		&job.TargetName,
		&inputJSON,
		&enabled,
		&approved,
		&job.RetryPolicy,
		&job.MaxAttempts,
		&job.BackoffSeconds,
		&job.CreatedAt,
		&job.UpdatedAt,
		&job.LastRunAt,
		&job.NextDueAt,
		&job.ArchivedAt,
	); err != nil {
		return Job{}, err
	}
	job.Enabled = enabled != 0
	job.Approved = approved != 0
	job.RetryPolicy = normalizeRetryPolicy(job.RetryPolicy)
	job.Input = map[string]any{}
	if strings.TrimSpace(inputJSON) != "" {
		_ = json.Unmarshal([]byte(inputJSON), &job.Input)
	}
	return job, nil
}

func scanJobs(rows *sql.Rows) ([]Job, error) {
	jobs := []Job{}
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job: %w", err)
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read jobs: %w", err)
	}
	return jobs, nil
}

func scanRuns(rows *sql.Rows) ([]JobRun, error) {
	runs := []JobRun{}
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, fmt.Errorf("scan job run: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read job runs: %w", err)
	}
	return runs, nil
}

func scanRun(row rowScanner) (JobRun, error) {
	var run JobRun
	var outputJSON string
	if err := row.Scan(
		&run.ID,
		&run.JobID,
		&run.JobName,
		&run.TargetType,
		&run.TargetName,
		&run.ScheduleType,
		&run.Status,
		&outputJSON,
		&run.StartedAt,
		&run.FinishedAt,
		&run.DurationMS,
		&run.Error,
		&run.Attempt,
		&run.RetryDueAt,
	); err != nil {
		return JobRun{}, err
	}
	run.Output = map[string]any{}
	if strings.TrimSpace(outputJSON) != "" {
		_ = json.Unmarshal([]byte(outputJSON), &run.Output)
	}
	if run.Attempt <= 0 {
		run.Attempt = 1
	}
	return run, nil
}

func (s *Store) timestamp() string {
	now := time.Now
	if s != nil && s.Now != nil {
		now = s.Now
	}
	return now().UTC().Format(time.RFC3339)
}

func (s *Store) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
