package scheduler

import (
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	RetryNone        = "none"
	RetryFixed       = "fixed"
	RetryExponential = "exponential"

	maxRetryAttempts       = 10
	maxRetryBackoffSeconds = 24 * 60 * 60
)

func normalizeRetryPolicy(policy string) string {
	switch strings.ToLower(strings.TrimSpace(policy)) {
	case "", RetryNone:
		return RetryNone
	case RetryFixed:
		return RetryFixed
	case RetryExponential:
		return RetryExponential
	default:
		return strings.TrimSpace(policy)
	}
}

func validateRetryPolicy(policy string, maxAttempts int, backoffSeconds int) error {
	policy = normalizeRetryPolicy(policy)
	switch policy {
	case RetryNone:
		if maxAttempts < 0 {
			return fmt.Errorf("retry max attempts cannot be negative")
		}
		if backoffSeconds < 0 {
			return fmt.Errorf("retry backoff seconds cannot be negative")
		}
		return nil
	case RetryFixed, RetryExponential:
		if maxAttempts < 2 || maxAttempts > maxRetryAttempts {
			return fmt.Errorf("retry max attempts must be between 2 and %d", maxRetryAttempts)
		}
		if backoffSeconds <= 0 || backoffSeconds > maxRetryBackoffSeconds {
			return fmt.Errorf("retry backoff seconds must be between 1 and %d", maxRetryBackoffSeconds)
		}
		return nil
	default:
		return fmt.Errorf("unknown retry policy %q", policy)
	}
}

func RetryDueAt(job Job, attempt int, finishedAt time.Time) time.Time {
	policy := normalizeRetryPolicy(job.RetryPolicy)
	if policy == RetryNone || attempt <= 0 || attempt >= job.MaxAttempts || job.BackoffSeconds <= 0 || finishedAt.IsZero() {
		return time.Time{}
	}
	delaySeconds := job.BackoffSeconds
	if policy == RetryExponential {
		multiplier := math.Pow(2, float64(attempt-1))
		if multiplier > float64(maxRetryBackoffSeconds) {
			delaySeconds = maxRetryBackoffSeconds
		} else {
			delaySeconds = int(float64(job.BackoffSeconds) * multiplier)
			if delaySeconds > maxRetryBackoffSeconds {
				delaySeconds = maxRetryBackoffSeconds
			}
		}
	}
	return finishedAt.UTC().Add(time.Duration(delaySeconds) * time.Second)
}
