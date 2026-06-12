package extensions

import (
	"fmt"
	"regexp"
	"strings"
)

var extensionSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(api[_-]?key|token|password|secret)\b\s*[:=]\s*["']?[^"'\s,}]+`),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}\b`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`),
	regexp.MustCompile(`\bghp_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bpat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\b[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{20,}\b`),
}

func redactExtensionText(input string) string {
	output := input
	for _, pattern := range extensionSecretPatterns {
		output = pattern.ReplaceAllStringFunc(output, redactExtensionSecret)
	}
	return output
}

func redactExtensionSecret(input string) string {
	if key, _, ok := strings.Cut(input, "="); ok {
		return strings.TrimSpace(key) + "=[REDACTED]"
	}
	if key, _, ok := strings.Cut(input, ":"); ok {
		return strings.TrimSpace(key) + ": [REDACTED]"
	}
	return "[REDACTED]"
}

func extensionTextContainsSecretValue(input string) bool {
	for _, pattern := range extensionSecretPatterns {
		if pattern.MatchString(input) {
			return true
		}
	}
	return false
}

func extensionValueContainsSecretValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return extensionTextContainsSecretValue(typed)
	case map[string]string:
		for key, item := range typed {
			if extensionTextContainsSecretValue(key) || extensionTextContainsSecretValue(item) {
				return true
			}
		}
		return false
	case map[string]any:
		for key, item := range typed {
			if extensionTextContainsSecretValue(key) || extensionValueContainsSecretValue(item) {
				return true
			}
		}
		return false
	case []string:
		for _, item := range typed {
			if extensionTextContainsSecretValue(item) {
				return true
			}
		}
		return false
	case []any:
		for _, item := range typed {
			if extensionValueContainsSecretValue(item) {
				return true
			}
		}
		return false
	default:
		return extensionTextContainsSecretValue(fmt.Sprint(typed))
	}
}

func redactExtensionValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case string:
		return redactExtensionText(typed)
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed
	case map[string]any:
		output := map[string]any{}
		for key, item := range typed {
			output[key] = redactExtensionValue(item)
		}
		return output
	case []any:
		output := make([]any, 0, len(typed))
		for _, item := range typed {
			output = append(output, redactExtensionValue(item))
		}
		return output
	default:
		return redactExtensionText(fmt.Sprint(typed))
	}
}

func redactExtensionMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	redacted := redactExtensionValue(input)
	output, _ := redacted.(map[string]any)
	return output
}

func redactCoreResults(results []CoreResult) []CoreResult {
	if len(results) == 0 {
		return nil
	}
	output := make([]CoreResult, 0, len(results))
	for _, result := range results {
		result.Error = redactExtensionText(result.Error)
		result.Output = redactExtensionMap(result.Output)
		output = append(output, result)
	}
	return output
}

func redactedRunResult(result RunResult) RunResult {
	result.Error = redactExtensionText(result.Error)
	result.Logs = redactExtensionText(result.Logs)
	result.Output = redactExtensionMap(result.Output)
	result.CoreResults = redactCoreResults(result.CoreResults)
	return result
}

func redactedTestResult(result TestResult) TestResult {
	result.Output = redactExtensionText(result.Output)
	return result
}

func redactedGenerationResult(result GenerationResult) GenerationResult {
	result.Message = redactExtensionText(result.Message)
	result.SkillCandidate = redactExtensionText(result.SkillCandidate)
	result.Tests = redactedTestResult(result.Tests)
	return result
}
