package agent

import (
	"fmt"
	"strings"
)

func PrepareAssistantPresentation(content string, decision ExecutionDecision, result *ExecutionResult) string {
	content = StripModelThinkingBlocks(content)
	content = stripHiddenModelToolRequest(content)
	content = stripInternalScaffold(content)
	content = GroundFailedToolResponse(content, decision, result)
	content = GroundCompletedToolResponse(content, decision, result)
	cleaned, removed := stripRawToolOutputBlocks(content)
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" && removed && result != nil {
		return groundedToolFallback(decision, *result)
	}
	return cleaned
}

func GroundUnsupportedActionClaim(content string, plan Plan, decision ExecutionDecision, result *ExecutionResult) string {
	cleaned := strings.TrimSpace(content)
	if cleaned == "" {
		return cleaned
	}
	verb := unsupportedAssistantActionClaim(plan, strings.ToLower(cleaned))
	if verb == "" {
		return cleaned
	}
	if trustedActionEvidenceSupportsClaim(verb, decision, result) {
		return cleaned
	}
	target := strings.TrimSpace(firstPlanFile(plan))
	if target != "" {
		return "That " + verb + " action for `" + target + "` is unverified because I do not have trusted tool evidence for it. I did not perform the requested action in this response. Treat it as pending until the appropriate approved tool flow completes."
	}
	return "That " + verb + " action is unverified because I do not have trusted tool evidence for it. I did not perform the requested action in this response. Treat it as pending until the appropriate approved tool flow completes."
}

func firstPlanFile(plan Plan) string {
	for _, file := range plan.FilesNeeded {
		if strings.TrimSpace(file) != "" {
			return strings.TrimSpace(file)
		}
	}
	return ""
}

func GroundCompletedToolResponse(content string, decision ExecutionDecision, result *ExecutionResult) string {
	if result == nil || !strings.EqualFold(strings.TrimSpace(result.Status), "completed") {
		return content
	}
	if !completedToolFutureClaim(content) {
		return content
	}
	return groundedToolFallback(decision, *result)
}

func completedToolFutureClaim(content string) bool {
	lower := strings.ToLower(content)
	if hasFutureToolClaim(lower) {
		return true
	}
	for _, marker := range []string{
		"i’ll now run",
		"i’ll now execute",
		"i’ll now fetch",
		"i will now fetch",
		"will now fetch",
		"please confirm if you want me to proceed",
		"once you confirm",
		"before i execute",
		"before i run",
		"at this point i will execute",
		"at this point i will run",
		"at this point i will fetch",
		"i’m about to execute",
		"i’m about to run",
		"i’m about to fetch",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func stripInternalScaffold(content string) string {
	content = strings.TrimSpace(content)
	if content == "" || !looksInternalScaffold(content) {
		return content
	}
	if final := finalResponseText(content); final != "" {
		return final
	}
	lines := strings.Split(content, "\n")
	firstScaffold := -1
	for i, line := range lines {
		if internalScaffoldHeading(line) || internalScaffoldFieldLine(line) {
			firstScaffold = i
			break
		}
	}
	if firstScaffold > 0 {
		prefix := strings.TrimSpace(strings.Join(lines[:firstScaffold], "\n"))
		if prefix != "" {
			return collapseBlankLines(prefix)
		}
	}
	out := make([]string, 0, len(lines))
	skipSection := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if internalScaffoldHeading(line) {
			skipSection = true
			continue
		}
		if skipSection {
			if trimmed == "" {
				skipSection = false
			}
			continue
		}
		if internalScaffoldFieldLine(line) {
			continue
		}
		out = append(out, line)
	}
	return collapseBlankLines(strings.Join(out, "\n"))
}

func looksInternalScaffold(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range []string{
		"reasoning step-by-step",
		"task execution summary",
		"task execution:",
		"tool action:",
		"route confidence",
		"verification criteria",
		"classification of the request",
		"source of truth is the chat context",
		"final response:",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return strings.Contains(lower, "assumptions:") && (strings.Contains(lower, "task plan") || strings.Contains(lower, "source of truth") || strings.Contains(lower, "available local context"))
}

func finalResponseText(content string) string {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := cleanFinalResponsePrefix(normalizeScaffoldLine(lines[i]))
		lower := strings.ToLower(trimmed)
		index := strings.Index(lower, "final response:")
		if index >= 0 {
			value := strings.TrimSpace(trimmed[index+len("final response:"):])
			rest := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			if rest != "" {
				value = strings.TrimSpace(value + "\n" + rest)
			}
			return cleanFinalResponsePrefix(value)
		}
	}
	return ""
}

func cleanFinalResponsePrefix(content string) string {
	content = strings.TrimSpace(content)
	content = strings.TrimLeft(content, "👉➜: ")
	content = strings.Trim(content, "*_` ")
	return strings.TrimSpace(content)
}

func internalScaffoldHeading(line string) bool {
	trimmed := normalizeScaffoldLine(line)
	lower := strings.ToLower(trimmed)
	for _, heading := range []string{
		"assumptions:",
		"reasoning step-by-step:",
		"task execution summary:",
		"task execution:",
		"tool action:",
		"classification of the request:",
		"verification criteria:",
		"action needed:",
	} {
		if lower == heading || strings.HasPrefix(lower, heading+" ") {
			return true
		}
	}
	return false
}

func internalScaffoldFieldLine(line string) bool {
	trimmed := normalizeScaffoldLine(line)
	field := strings.TrimLeft(trimmed, "-• ")
	lower := strings.ToLower(field)
	for _, prefix := range []string{
		"task type:",
		"goal:",
		"route confidence:",
		"risk level:",
		"max steps:",
		"tool used:",
		"source of truth:",
		"route source of truth:",
		"verification:",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func normalizeScaffoldLine(line string) string {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.Trim(trimmed, "*_`# ")
	return strings.TrimSpace(trimmed)
}

func stripHiddenModelToolRequest(content string) string {
	if !strings.Contains(strings.ToLower(content), "yemaka_tool_request") {
		return content
	}
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	skipFence := false
	skipNextFence := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if skipNextFence && !inFence {
				inFence = true
				skipFence = true
				skipNextFence = false
				continue
			}
			if inFence {
				if !skipFence {
					out = append(out, line)
				}
				inFence = false
				skipFence = false
				continue
			}
			inFence = true
			skipFence = false
			out = append(out, line)
			continue
		}
		if strings.Contains(strings.ToLower(line), "yemaka_tool_request") {
			if inFence {
				if len(out) > 0 && strings.HasPrefix(strings.TrimSpace(out[len(out)-1]), "```") {
					out = out[:len(out)-1]
				}
				skipFence = true
			} else {
				skipNextFence = true
			}
			continue
		}
		if skipFence {
			continue
		}
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func stripRawToolOutputBlocks(content string) (string, bool) {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	inFence := false
	fenceLines := []string{}
	removed := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inFence {
				fenceLines = append(fenceLines, line)
				if rawToolOutputText(strings.Join(fenceLines, "\n")) {
					removed = true
				} else {
					out = append(out, fenceLines...)
				}
				inFence = false
				fenceLines = nil
				continue
			}
			inFence = true
			fenceLines = []string{line}
			continue
		}
		if inFence {
			fenceLines = append(fenceLines, line)
			continue
		}
		if rawToolOutputLine(line) {
			removed = true
			continue
		}
		out = append(out, line)
	}
	if inFence {
		if rawToolOutputText(strings.Join(fenceLines, "\n")) {
			removed = true
		} else {
			out = append(out, fenceLines...)
		}
	}
	return collapseBlankLines(strings.Join(out, "\n")), removed
}

func rawToolOutputText(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range []string{
		"tool result:",
		"tool observation:",
		"document ingest result:",
		"internet search result:",
		"internet result:",
		"command result:",
		"source_kind:",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func rawToolOutputLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	trimmed = strings.Trim(trimmed, "*_` ")
	fieldTrimmed := strings.TrimLeft(trimmed, "- ")
	lower := strings.ToLower(trimmed)
	for _, marker := range []string{
		"tool result:",
		"tool observation:",
		"document ingest result:",
		"internet search result:",
		"internet result:",
		"command result:",
	} {
		if lower == marker || strings.HasPrefix(lower, marker) {
			return true
		}
	}
	for _, field := range []string{
		"source_kind:",
		"status:",
		"sources:",
		"tool:",
		"target:",
		"root:",
		"query:",
		"provider:",
		"from_cache:",
		"fetched_at:",
		"cached_at:",
		"search_url:",
		"result_count:",
		"url:",
		"final_url:",
		"method:",
		"content_type:",
		"bytes:",
		"files_indexed:",
		"files_unchanged:",
		"files_skipped:",
		"chunks_created:",
		"bytes_indexed:",
		"note:",
		"skipped_reasons:",
	} {
		if strings.HasPrefix(fieldTrimmed, field) {
			return true
		}
	}
	return false
}

func collapseBlankLines(content string) string {
	lines := strings.Split(content, "\n")
	out := make([]string, 0, len(lines))
	blank := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			if blank {
				continue
			}
			blank = true
			out = append(out, "")
			continue
		}
		blank = false
		out = append(out, line)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

func groundedToolFallback(decision ExecutionDecision, result ExecutionResult) string {
	tool := strings.TrimSpace(decision.ToolName)
	if tool == "" {
		tool = "the requested tool"
	}
	if strings.EqualFold(tool, "ingest_documents") {
		return DocumentIngestResponse(result)
	}
	if strings.EqualFold(tool, "internet_fetch") || strings.EqualFold(tool, "internet_head") {
		return groundedInternetToolFallback(tool, result)
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "I used `%s` and hid the raw internal tool transcript from chat.", tool)
	status := strings.TrimSpace(result.Status)
	if status != "" {
		fmt.Fprintf(&builder, "\n\nStatus: %s.", status)
	}
	if len(result.Sources) > 0 {
		fmt.Fprintf(&builder, "\n\nSources are shown below.")
	}
	return strings.TrimSpace(builder.String())
}

func groundedInternetToolFallback(tool string, result ExecutionResult) string {
	target := firstNonEmptyString(firstResultSource(result), outputResultField(result.Context, "final_url"), outputResultField(result.Context, "url"))
	statusCode := outputResultField(result.Context, "status")
	var builder strings.Builder
	if target != "" {
		fmt.Fprintf(&builder, "I checked `%s` with `%s`.", target, tool)
	} else {
		fmt.Fprintf(&builder, "I ran `%s` for the requested web target.", tool)
	}
	if statusCode != "" {
		fmt.Fprintf(&builder, "\n\nResult: HTTP %s.", statusCode)
	} else if strings.TrimSpace(result.Status) != "" {
		fmt.Fprintf(&builder, "\n\nResult: %s.", strings.TrimSpace(result.Status))
	}
	if len(result.Sources) > 0 {
		fmt.Fprintf(&builder, "\n\nSources are shown below.")
	}
	return strings.TrimSpace(builder.String())
}

func firstResultSource(result ExecutionResult) string {
	for _, source := range result.Sources {
		if strings.TrimSpace(source) != "" {
			return strings.TrimSpace(source)
		}
	}
	return ""
}

func outputResultField(context string, name string) string {
	prefix := strings.ToLower(strings.TrimSpace(name)) + ":"
	for _, line := range strings.Split(context, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), prefix) {
			return strings.TrimSpace(line[len(prefix):])
		}
	}
	return ""
}
