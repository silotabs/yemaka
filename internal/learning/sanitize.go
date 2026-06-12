package learning

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type PrivacyFilterReport struct {
	SecretsRedacted bool `json:"secretsRedacted"`
	PathsRedacted   bool `json:"pathsRedacted"`
}

var (
	secretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(api[_-]?key|token|password|secret)\s*[:=]\s*["']?[^"'\s,}]+`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}\b`),
		regexp.MustCompile(`\b[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{20,}\b`),
	}
	userPathPattern = regexp.MustCompile(`/Users/[^/\s]+/`)
)

func SanitizeValue(value any) (any, PrivacyFilterReport) {
	switch typed := value.(type) {
	case nil:
		return nil, PrivacyFilterReport{}
	case string:
		return SanitizeText(typed)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed, PrivacyFilterReport{}
	case map[string]any:
		out := map[string]any{}
		var report PrivacyFilterReport
		for key, item := range typed {
			clean, itemReport := SanitizeValue(item)
			out[key] = clean
			report = mergeReports(report, itemReport)
		}
		return out, report
	case []any:
		out := make([]any, 0, len(typed))
		var report PrivacyFilterReport
		for _, item := range typed {
			clean, itemReport := SanitizeValue(item)
			out = append(out, clean)
			report = mergeReports(report, itemReport)
		}
		return out, report
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return typed, PrivacyFilterReport{}
		}
		var decoded any
		if err := json.Unmarshal(data, &decoded); err != nil {
			clean, report := SanitizeText(fmt.Sprint(typed))
			return clean, report
		}
		switch decoded.(type) {
		case string, map[string]any, []any:
			return SanitizeValue(decoded)
		default:
			return decoded, PrivacyFilterReport{}
		}
	}
}

func SanitizeText(input string) (string, PrivacyFilterReport) {
	output := input
	report := PrivacyFilterReport{}
	for _, pattern := range secretPatterns {
		if pattern.MatchString(output) {
			report.SecretsRedacted = true
			output = pattern.ReplaceAllStringFunc(output, redactSecretAssignment)
		}
	}
	if userPathPattern.MatchString(output) {
		report.PathsRedacted = true
		output = userPathPattern.ReplaceAllString(output, "~/")
	}
	return output, report
}

func redactSecretAssignment(input string) string {
	if key, _, ok := strings.Cut(input, "="); ok {
		return strings.TrimSpace(key) + "=[REDACTED]"
	}
	if key, _, ok := strings.Cut(input, ":"); ok {
		return strings.TrimSpace(key) + ": [REDACTED]"
	}
	return "[REDACTED]"
}

func mergeReports(left PrivacyFilterReport, right PrivacyFilterReport) PrivacyFilterReport {
	return PrivacyFilterReport{
		SecretsRedacted: left.SecretsRedacted || right.SecretsRedacted,
		PathsRedacted:   left.PathsRedacted || right.PathsRedacted,
	}
}
