package heartbeat

import (
	"context"
	"fmt"
	"time"
)

type Reporter interface {
	Report(ctx context.Context) (Report, error)
}

type ReporterFunc func(ctx context.Context) (Report, error)

func (fn ReporterFunc) Report(ctx context.Context) (Report, error) {
	return fn(ctx)
}

type Loop struct {
	Enabled  bool
	Store    *Store
	Reporter Reporter
	Interval time.Duration
}

func (l *Loop) Tick(ctx context.Context) (Report, error) {
	if l == nil || !l.Enabled {
		return Report{}, fmt.Errorf("heartbeat is disabled")
	}
	if l.Store == nil {
		return Report{}, fmt.Errorf("heartbeat store is required")
	}
	if l.Reporter == nil {
		return Report{}, fmt.Errorf("heartbeat reporter is required")
	}
	report, err := l.Reporter.Report(ctx)
	if err != nil {
		return Report{}, err
	}
	if err := l.Store.Record(ctx, report); err != nil {
		return Report{}, err
	}
	return report, nil
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
