package agent

import (
	"fmt"
	"strings"
)

func BuildEditProposal(plan Plan, input PlanInput, decision ExecutionDecision) EditProposal {
	path := firstEditPath(plan, input)
	content, hasContent := inlineEditContent(input.Content, path)
	contentSource := ""
	status := "needs_content"
	reason := "A full replacement file body is required before Yemaka can preview and apply a workspace-only edit."
	if strings.TrimSpace(path) == "" {
		status = "needs_path"
		reason = "A workspace-relative file path is required before Yemaka can preview the edit."
	}
	if hasContent && strings.TrimSpace(path) != "" {
		status = "ready_for_preview"
		reason = "A candidate file body is available for diff preview."
		contentSource = "inline"
	}
	return EditProposal{
		RequestID:           decision.RequestID,
		Path:                path,
		Content:             content,
		ContentSource:       contentSource,
		Status:              status,
		Reason:              reason,
		NeedsContent:        status != "ready_for_preview",
		DiffPreview:         true,
		SnapshotBeforeWrite: true,
		RollbackSupported:   true,
	}
}

func EditProposalResponse(decision ExecutionDecision, proposal EditProposal) string {
	response := MissingExecutorResponse(decision)
	if proposal.Path == "" {
		return response + " I also need the workspace-relative file path before I can prepare a diff preview."
	}
	if proposal.NeedsContent {
		return response + " I have not changed the file. I prepared an edit proposal for " + proposal.Path + ", but I need the full replacement content before preview/apply."
	}
	return response + " I have not changed the file. I prepared an edit proposal for " + proposal.Path + ". Review the diff preview before applying it."
}

func EditProposalNeedsContentResponse(proposal EditProposal) string {
	path := strings.TrimSpace(proposal.Path)
	if path == "" {
		return "I can prepare a file edit, but I need the target workspace path before I can show a diff preview or ask for approval."
	}
	return "I have not changed the file. I found the target path " + path + ", but I still need the full replacement content before I can show a diff preview or ask for approval."
}

func EditDraftInstructions(proposal EditProposal) string {
	path := strings.TrimSpace(proposal.Path)
	if path == "" {
		path = "<workspace-relative path>"
	}
	return fmt.Sprintf(
		" You may draft a safe edit proposal for preview only; do not claim the file was changed. "+
			"If the provided context is sufficient, return a complete replacement file body using exactly this shape:\n\n"+
			"YEMAKA_EDIT_PROPOSAL\npath: %s\ncontent:\n```text\n<complete replacement file content>\n```\n\n"+
			"If the full replacement body cannot be produced from the available context, say what file context is missing.",
		path,
	)
}

func BuildModelEditProposal(base EditProposal, modelText string) (EditProposal, bool) {
	path, content, ok := parseModelEditProposal(modelText, base.Path)
	if !ok {
		return base, false
	}
	base.Path = path
	base.Content = content
	base.ContentSource = "model"
	base.Status = "ready_for_preview"
	base.Reason = "A model-authored full replacement body is available for diff preview."
	base.NeedsContent = false
	base.DiffPreview = true
	base.SnapshotBeforeWrite = true
	base.RollbackSupported = true
	return base, true
}

func firstEditPath(plan Plan, input PlanInput) string {
	for _, path := range plan.FilesNeeded {
		if strings.TrimSpace(path) != "" {
			return strings.TrimSpace(path)
		}
	}
	for _, source := range input.Sources {
		source = strings.TrimSpace(source)
		if source != "" {
			return source
		}
	}
	return ""
}

func inlineEditContent(content string, path string) (string, bool) {
	if strings.TrimSpace(content) == "" {
		return "", false
	}
	type markerRule struct {
		text     string
		lineOnly bool
	}
	markers := []markerRule{
		{text: "with exactly this line:", lineOnly: true},
		{text: "exactly this line:", lineOnly: true},
		{text: "with this line:", lineOnly: true},
		{text: "this line:", lineOnly: true},
		{text: "with the text"},
		{text: "with text"},
		{text: "with content:"},
		{text: "content:"},
		{text: "set content to:"},
		{text: "containing"},
		{text: "that says"},
		{text: "write content:"},
		{text: "write into it"},
		{text: "write to it"},
		{text: "write in it"},
		{text: "write into the file"},
		{text: "write to the file"},
		{text: "put into it"},
		{text: "put in it"},
	}
	lower := strings.ToLower(content)
	startAt := 0
	if path != "" {
		if index := strings.Index(lower, strings.ToLower(path)); index >= 0 {
			startAt = index + len(path)
		}
	}
	for _, marker := range markers {
		index := strings.Index(lower[startAt:], marker.text)
		if index < 0 {
			continue
		}
		start := startAt + index + len(marker.text)
		candidate := strings.TrimSpace(content[start:])
		candidate = strings.TrimLeft(candidate, ":- ")
		candidate = strings.Trim(candidate, "\"'`")
		if marker.lineOnly {
			candidate = firstNonEmptyLine(candidate)
		}
		if candidate != "" {
			return candidate, true
		}
	}
	return "", false
}

func inlineAppendLine(content string, path string) (string, bool) {
	if strings.TrimSpace(content) == "" || strings.TrimSpace(path) == "" {
		return "", false
	}
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "append") && !strings.Contains(lower, "add this line") {
		return "", false
	}
	pathIndex := strings.Index(lower, strings.ToLower(path))
	if pathIndex < 0 {
		return "", false
	}
	afterPath := content[pathIndex+len(path):]
	colon := strings.Index(afterPath, ":")
	if colon < 0 {
		return "", false
	}
	line := firstNonEmptyLine(afterPath[colon+1:])
	if line == "" {
		return "", false
	}
	return line, true
}

func appendLineToContent(existing string, line string) string {
	line = strings.TrimSpace(line)
	line = strings.Trim(line, "\"'`")
	if line == "" {
		return existing
	}
	existing = strings.TrimRight(existing, "\r\n")
	if existing == "" {
		return line
	}
	return existing + "\n" + line
}

func firstNonEmptyLine(content string) string {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "\"'`")
		if line != "" {
			return line
		}
	}
	return ""
}

func parseModelEditProposal(text string, fallbackPath string) (string, string, bool) {
	if path, content, ok := parseNamedFence(text); ok {
		return path, content, true
	}
	if path, content, ok := parseStructuredEditProposal(text, fallbackPath); ok {
		return path, content, true
	}
	if content, ok := firstFenceContent(text); ok && strings.TrimSpace(fallbackPath) != "" {
		return strings.TrimSpace(fallbackPath), content, true
	}
	return "", "", false
}

func parseNamedFence(text string) (string, string, bool) {
	lower := strings.ToLower(text)
	search := 0
	for {
		index := strings.Index(lower[search:], "```yemaka-file:")
		if index < 0 {
			return "", "", false
		}
		start := search + index
		headerEnd := strings.Index(text[start:], "\n")
		if headerEnd < 0 {
			return "", "", false
		}
		header := strings.TrimSpace(text[start+3 : start+headerEnd])
		if !strings.HasPrefix(strings.ToLower(header), "yemaka-file:") {
			search = start + headerEnd + 1
			continue
		}
		path := strings.TrimSpace(header[len("yemaka-file:"):])
		path = strings.Trim(path, "\"'")
		contentStart := start + headerEnd + 1
		end := strings.Index(text[contentStart:], "```")
		if end < 0 {
			return "", "", false
		}
		content := strings.Trim(text[contentStart:contentStart+end], "\r\n")
		if path != "" && strings.TrimSpace(content) != "" {
			return path, content, true
		}
		search = contentStart + end + 3
	}
}

func parseStructuredEditProposal(text string, fallbackPath string) (string, string, bool) {
	lower := strings.ToLower(text)
	marker := "yemaka_edit_proposal"
	index := strings.Index(lower, marker)
	if index < 0 {
		return "", "", false
	}
	block := text[index+len(marker):]
	path := strings.TrimSpace(fallbackPath)
	lines := strings.Split(block, "\n")
	contentLine := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lowerLine := strings.ToLower(trimmed)
		if strings.HasPrefix(lowerLine, "path:") {
			path = strings.TrimSpace(trimmed[len("path:"):])
			path = strings.Trim(path, "\"'")
			continue
		}
		if lowerLine == "content:" || strings.HasPrefix(lowerLine, "content:") {
			contentLine = i
			break
		}
	}
	if contentLine < 0 || path == "" {
		return "", "", false
	}
	after := strings.TrimSpace(strings.Join(lines[contentLine+1:], "\n"))
	if after == "" {
		trimmed := strings.TrimSpace(lines[contentLine])
		if len(trimmed) > len("content:") {
			after = strings.TrimSpace(trimmed[len("content:"):])
		}
	}
	content := strings.Trim(after, "\r\n")
	if fenced, ok := firstFenceContent(after); ok {
		content = fenced
	}
	if strings.TrimSpace(content) == "" {
		return "", "", false
	}
	return path, content, true
}

func firstFenceContent(text string) (string, bool) {
	start := strings.Index(text, "```")
	if start < 0 {
		return "", false
	}
	lineEnd := strings.Index(text[start:], "\n")
	if lineEnd < 0 {
		return "", false
	}
	contentStart := start + lineEnd + 1
	end := strings.Index(text[contentStart:], "```")
	if end < 0 {
		return "", false
	}
	content := strings.Trim(text[contentStart:contentStart+end], "\r\n")
	return content, strings.TrimSpace(content) != ""
}
