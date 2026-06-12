package notifications

import (
	"testing"
	"time"

	"yemaka/internal/heartbeat"
	"yemaka/internal/scheduler"
)

func TestRecordJobRunNotificationOnlyRecordsFailures(t *testing.T) {
	store := NewStore(t.TempDir())
	store.Now = func() time.Time { return time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC) }

	job := scheduler.Job{ID: "job_1", Name: "Nightly review"}
	okRun := scheduler.JobRun{ID: "run_ok", JobID: job.ID, Status: scheduler.StatusCompleted}
	if _, recorded, err := RecordJobRunNotification(store, job, okRun); err != nil || recorded {
		t.Fatalf("RecordJobRunNotification(completed) recorded=%v err=%v", recorded, err)
	}

	failedRun := scheduler.JobRun{ID: "run_fail", JobID: job.ID, Status: scheduler.StatusFailed, Error: "tool unavailable"}
	item, recorded, err := RecordJobRunNotification(store, job, failedRun)
	if err != nil {
		t.Fatalf("RecordJobRunNotification(failed) error = %v", err)
	}
	if !recorded {
		t.Fatalf("RecordJobRunNotification(failed) recorded = false")
	}
	if item.ID != "job_run_run_fail" || item.Severity != SeverityError || !item.ActionRequired {
		t.Fatalf("unexpected notification: %+v", item)
	}

	if _, recorded, err := RecordJobRunNotification(store, job, failedRun); err != nil || recorded {
		t.Fatalf("duplicate RecordJobRunNotification recorded=%v err=%v", recorded, err)
	}
}

func TestRecordHeartbeatTransitionNotifications(t *testing.T) {
	store := NewStore(t.TempDir())
	store.Now = func() time.Time { return time.Date(2026, 6, 2, 10, 0, 0, 0, time.UTC) }

	healthy := heartbeat.Report{Overall: heartbeat.StatusHealthy, GeneratedAt: "2026-06-02T10:00:00Z"}
	if items, err := RecordHeartbeatTransitionNotifications(store, heartbeat.Report{}, healthy); err != nil || len(items) != 0 {
		t.Fatalf("initial healthy items=%d err=%v", len(items), err)
	}

	warning := heartbeat.Report{
		Overall:     heartbeat.StatusNeedsConfig,
		GeneratedAt: "2026-06-02T10:01:00Z",
		Checks: []heartbeat.Check{
			{Name: "internet", Status: heartbeat.StatusNeedsConfig, Detail: "provider not configured"},
		},
	}
	items, err := RecordHeartbeatTransitionNotifications(store, healthy, warning)
	if err != nil {
		t.Fatalf("degraded heartbeat notification error = %v", err)
	}
	if len(items) != 1 || items[0].Severity != SeverityWarning || !items[0].ActionRequired {
		t.Fatalf("unexpected degraded heartbeat notification: %+v", items)
	}

	items, err = RecordHeartbeatTransitionNotifications(store, warning, warning)
	if err != nil || len(items) != 0 {
		t.Fatalf("same-status heartbeat items=%d err=%v", len(items), err)
	}

	recovered := heartbeat.Report{Overall: heartbeat.StatusHealthy, GeneratedAt: "2026-06-02T10:02:00Z"}
	items, err = RecordHeartbeatTransitionNotifications(store, warning, recovered)
	if err != nil {
		t.Fatalf("recovery heartbeat notification error = %v", err)
	}
	if len(items) != 1 || items[0].Severity != SeveritySuccess || items[0].ActionRequired {
		t.Fatalf("unexpected recovery heartbeat notification: %+v", items)
	}
}
