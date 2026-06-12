package scheduler

import (
	"context"
	"time"
)

type Loop struct {
	Store    *Store
	Runner   Runner
	Options  RunOptions
	Interval time.Duration
	Now      func() time.Time
}

func (l *Loop) Tick(ctx context.Context) (TickResult, error) {
	now := l.now()
	result := TickResult{
		CheckedAt:    now.UTC().Format(time.RFC3339),
		MissedPolicy: "coalesce",
	}
	if l == nil || l.Store == nil || l.Runner == nil {
		return result, nil
	}
	if err := validateRunOptions(l.Options); err != nil {
		return result, err
	}
	limit := effectiveMaxParallel(l.Options)
	dueJobs, err := l.Store.DueJobs(ctx, now, limit+1)
	if err != nil {
		return result, err
	}
	jobs := dueJobs
	if len(jobs) > limit {
		result.Skipped = len(jobs) - limit
		jobs = jobs[:limit]
	}
	for _, job := range jobs {
		result.MissedRuns += MissedRunCount(job, now, 1000)
		run, err := l.Store.Run(ctx, job.ID, l.Runner, l.Options)
		if err != nil {
			result.Runs = append(result.Runs, run)
			continue
		}
		result.Runs = append(result.Runs, run)
	}
	return result, nil
}

func (l *Loop) Run(ctx context.Context) error {
	interval := l.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	if _, err := l.Tick(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := l.Tick(ctx); err != nil {
				return err
			}
		}
	}
}

func (l *Loop) now() time.Time {
	if l != nil && l.Now != nil {
		return l.Now().UTC()
	}
	return time.Now().UTC()
}

func effectiveMaxParallel(options RunOptions) int {
	if options.LowMemoryMode {
		return 1
	}
	if options.MaxParallel <= 0 {
		return 1
	}
	return options.MaxParallel
}
