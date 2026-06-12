package notifications

import (
	"fmt"
	"strings"

	"yemaka/internal/heartbeat"
	"yemaka/internal/scheduler"
)

func RecordJobRunNotification(store Store, job scheduler.Job, run scheduler.JobRun) (Notification, bool, error) {
	if !failedJobRun(run) {
		return Notification{}, false, nil
	}
	id := "job_run_" + sanitizeToken(run.ID)
	if id == "job_run_" {
		return Notification{}, false, nil
	}
	severity := SeverityError
	status := strings.TrimSpace(run.Status)
	title := fmt.Sprintf("Job %s %s", firstNonEmpty(job.Name, job.ID, run.JobID), firstNonEmpty(status, "failed"))
	message := firstNonEmpty(run.Error, "The job run did not complete successfully.")
	item, err := store.Record(Notification{
		ID:             id,
		Type:           "job_run",
		Severity:       severity,
		Title:          title,
		Message:        message,
		Source:         "scheduler",
		ActionRequired: true,
		Metadata: map[string]string{
			"job_id":     firstNonEmpty(job.ID, run.JobID),
			"job_name":   job.Name,
			"run_id":     run.ID,
			"run_status": status,
		},
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return Notification{}, false, nil
	}
	return item, err == nil, err
}

func RecordHeartbeatTransitionNotifications(store Store, previous heartbeat.Report, current heartbeat.Report) ([]Notification, error) {
	currentOverall := normalizeHeartbeatStatus(current.Overall)
	previousOverall := normalizeHeartbeatStatus(previous.Overall)
	if currentOverall == "" || currentOverall == heartbeat.StatusDisabled || currentOverall == previousOverall {
		return nil, nil
	}
	if currentOverall == heartbeat.StatusHealthy && previousOverall == "" {
		return nil, nil
	}
	if !heartbeatStatusNotifiable(currentOverall) && currentOverall != heartbeat.StatusHealthy {
		return nil, nil
	}
	id := "heartbeat_" + sanitizeToken(previousOverall) + "_to_" + sanitizeToken(currentOverall)
	if generated := strings.TrimSpace(current.GeneratedAt); generated != "" {
		id += "_" + sanitizeToken(generated)
	}
	severity := SeverityWarning
	title := "Heartbeat needs attention"
	actionRequired := true
	if currentOverall == heartbeat.StatusBroken {
		severity = SeverityError
	} else if currentOverall == heartbeat.StatusHealthy {
		severity = SeveritySuccess
		title = "Heartbeat recovered"
		actionRequired = false
	}
	item, err := store.Record(Notification{
		ID:             id,
		Type:           "heartbeat",
		Severity:       severity,
		Title:          title,
		Message:        heartbeatTransitionMessage(previousOverall, currentOverall, current),
		Source:         "heartbeat",
		ActionRequired: actionRequired,
		Metadata: map[string]string{
			"previous": previousOverall,
			"current":  currentOverall,
		},
	})
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "already exists") {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return []Notification{item}, nil
}

func failedJobRun(run scheduler.JobRun) bool {
	switch strings.TrimSpace(run.Status) {
	case scheduler.StatusFailed, scheduler.StatusTimeout:
		return true
	default:
		return strings.TrimSpace(run.Error) != ""
	}
}

func heartbeatStatusNotifiable(status string) bool {
	switch status {
	case heartbeat.StatusWarning, heartbeat.StatusBroken, heartbeat.StatusNeedsAuth, heartbeat.StatusNeedsConfig:
		return true
	default:
		return false
	}
}

func normalizeHeartbeatStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func heartbeatTransitionMessage(previous string, current string, report heartbeat.Report) string {
	var detail string
	for _, check := range report.Checks {
		if normalizeHeartbeatStatus(check.Status) == current && strings.TrimSpace(check.Detail) != "" {
			detail = check.Name + ": " + check.Detail
			break
		}
	}
	if detail == "" {
		detail = "overall status changed"
	}
	if previous == "" {
		return fmt.Sprintf("Heartbeat is now %s: %s", current, detail)
	}
	return fmt.Sprintf("Heartbeat changed from %s to %s: %s", previous, current, detail)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
