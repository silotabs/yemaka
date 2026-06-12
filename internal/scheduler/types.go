package scheduler

const (
	ScheduleManual   = "manual"
	ScheduleInterval = "interval"
	ScheduleCron     = "cron"
	ScheduleOneTime  = "one_time"

	TargetExtension = "extension"
	TargetHeartbeat = "heartbeat"

	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusTimeout   = "timeout"

	InputStatusValid   = "valid"
	InputStatusInvalid = "invalid"
)

type Job struct {
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	ScheduleType   string         `json:"scheduleType"`
	ScheduleExpr   string         `json:"scheduleExpr"`
	TargetType     string         `json:"targetType"`
	TargetName     string         `json:"targetName"`
	Input          map[string]any `json:"input"`
	Enabled        bool           `json:"enabled"`
	Approved       bool           `json:"approved"`
	RetryPolicy    string         `json:"retryPolicy"`
	MaxAttempts    int            `json:"maxAttempts"`
	BackoffSeconds int            `json:"backoffSeconds"`
	CreatedAt      string         `json:"createdAt"`
	UpdatedAt      string         `json:"updatedAt"`
	LastRunAt      string         `json:"lastRunAt"`
	NextDueAt      string         `json:"nextDueAt"`
	ArchivedAt     string         `json:"archivedAt,omitempty"`

	InputStatus          string `json:"inputStatus,omitempty"`
	InputValidationError string `json:"inputValidationError,omitempty"`
}

type JobRun struct {
	ID           string `json:"id"`
	JobID        string `json:"jobId"`
	JobName      string `json:"jobName,omitempty"`
	TargetType   string `json:"targetType,omitempty"`
	TargetName   string `json:"targetName,omitempty"`
	ScheduleType string `json:"scheduleType,omitempty"`
	Status       string `json:"status"`
	Output       any    `json:"output"`
	StartedAt    string `json:"startedAt"`
	FinishedAt   string `json:"finishedAt"`
	DurationMS   int64  `json:"durationMs"`
	Error        string `json:"error"`
	Attempt      int    `json:"attempt"`
	RetryDueAt   string `json:"retryDueAt"`
}

type CreateInput struct {
	Name           string         `json:"name"`
	ScheduleType   string         `json:"scheduleType"`
	ScheduleExpr   string         `json:"scheduleExpr"`
	TargetType     string         `json:"targetType"`
	TargetName     string         `json:"targetName"`
	Input          map[string]any `json:"input"`
	Approved       bool           `json:"approved"`
	Enabled        bool           `json:"enabled"`
	RetryPolicy    string         `json:"retryPolicy"`
	MaxAttempts    int            `json:"maxAttempts"`
	BackoffSeconds int            `json:"backoffSeconds"`
}

type UpdateInput struct {
	ID    string         `json:"id"`
	Input map[string]any `json:"input"`
}

type RunOptions struct {
	Enabled        bool
	TimeoutSeconds int
	MaxParallel    int
	LowMemoryMode  bool
	PolicyMode     string
}

type Status struct {
	Enabled          bool   `json:"enabled"`
	TotalJobs        int    `json:"totalJobs"`
	EnabledJobs      int    `json:"enabledJobs"`
	ApprovedJobs     int    `json:"approvedJobs"`
	ArchivedJobs     int    `json:"archivedJobs"`
	RunningJobs      int    `json:"runningJobs"`
	MaxParallelJobs  int    `json:"maxParallelJobs"`
	InvalidInputJobs int    `json:"invalidInputJobs"`
	LastRunAt        string `json:"lastRunAt"`
	LastRunStatus    string `json:"lastRunStatus"`
}

type TickResult struct {
	CheckedAt    string   `json:"checkedAt"`
	Runs         []JobRun `json:"runs"`
	Skipped      int      `json:"skipped"`
	MissedRuns   int      `json:"missedRuns"`
	MissedPolicy string   `json:"missedPolicy"`
}

func AnnotateJobInputStatuses(jobs []Job, validate func(Job) (bool, error)) ([]Job, int) {
	if validate == nil {
		return jobs, 0
	}
	invalid := 0
	for index := range jobs {
		checked, err := validate(jobs[index])
		if !checked {
			continue
		}
		if err != nil {
			jobs[index].InputStatus = InputStatusInvalid
			jobs[index].InputValidationError = err.Error()
			invalid++
			continue
		}
		jobs[index].InputStatus = InputStatusValid
		jobs[index].InputValidationError = ""
	}
	return jobs, invalid
}
