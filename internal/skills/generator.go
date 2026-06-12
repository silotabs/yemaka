package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type TranscriptMessage struct {
	Role    string
	Content string
}

type ToolRunSummary struct {
	ToolName string
	Status   string
}

type GenerateInput struct {
	ProfileSkillsDir string
	ConversationID   string
	Messages         []TranscriptMessage
	ToolRuns         []ToolRunSummary
}

type ImproveInput struct {
	ProfileSkillsDir string
	ConversationID   string
	Skill            Skill
	Messages         []TranscriptMessage
	ToolRuns         []ToolRunSummary
}

func CreateFromSession(input GenerateInput) (Skill, error) {
	if strings.TrimSpace(input.ProfileSkillsDir) == "" {
		return Skill{}, fmt.Errorf("profile skills directory is required")
	}
	if strings.TrimSpace(input.ConversationID) == "" {
		return Skill{}, fmt.Errorf("conversation id is required")
	}
	if len(input.Messages) == 0 {
		return Skill{}, fmt.Errorf("conversation has no messages")
	}

	firstUser := sanitizeSkillText(firstUserMessage(input.Messages))
	name := uniqueSkillName(input.ProfileSkillsDir, generatedName(firstUser, input.ConversationID))
	requiredTools := generatedRequiredTools(input)
	skill := Skill{
		Name:          name,
		Version:       "0.1.0",
		Description:   generatedDescription(firstUser),
		Triggers:      generatedTriggers(firstUser),
		RequiredTools: requiredTools,
		Permissions:   generatedPermissions(requiredTools),
		ContextBudget: ContextBudget{
			MaxInstructionChars: 1800,
			MaxExamples:         1,
		},
		Disabled: true,
	}
	instructions := generatedInstructions(input, firstUser, requiredTools)
	skill.Instructions = instructions
	if err := Validate(skill); err != nil {
		return Skill{}, err
	}
	target := filepath.Join(input.ProfileSkillsDir, skill.Name)
	if err := writeSkillPackage(target, skill, instructions); err != nil {
		return Skill{}, err
	}
	created, err := Load(target)
	if err != nil {
		return Skill{}, err
	}
	created.Source = "profile"
	return created, nil
}

func ImproveFromSession(input ImproveInput) (Skill, error) {
	if strings.TrimSpace(input.ProfileSkillsDir) == "" {
		return Skill{}, fmt.Errorf("profile skills directory is required")
	}
	if strings.TrimSpace(input.ConversationID) == "" {
		return Skill{}, fmt.Errorf("conversation id is required")
	}
	if strings.TrimSpace(input.Skill.Name) == "" {
		return Skill{}, fmt.Errorf("skill is required")
	}
	if len(input.Messages) == 0 {
		return Skill{}, fmt.Errorf("conversation has no messages")
	}

	requiredTools := uniqueKnownTools(append(append([]string{}, input.Skill.RequiredTools...), generatedRequiredTools(GenerateInput{
		ConversationID: input.ConversationID,
		Messages:       input.Messages,
		ToolRuns:       input.ToolRuns,
	})...))
	improved := input.Skill
	improved.Version = bumpPatchVersion(improved.Version)
	improved.RequiredTools = requiredTools
	firstUser := sanitizeSkillText(firstUserMessage(input.Messages))
	improved.Triggers = uniqueStrings(append(append([]string{}, improved.Triggers...), generatedTriggers(firstUser)...))
	improved.Permissions = mergePermissions(improved.Permissions, generatedPermissions(requiredTools))
	improved.Instructions = compactImprovedInstructions(input.Skill.Instructions, generatedImprovementBlock(input, requiredTools), improved.ContextBudget.MaxInstructionChars)
	if improved.ContextBudget.MaxInstructionChars == 0 {
		improved.ContextBudget.MaxInstructionChars = defaultMaxInstructionChars
	}
	if err := Validate(improved); err != nil {
		return Skill{}, err
	}
	target := filepath.Join(input.ProfileSkillsDir, improved.Name)
	if err := writeSkillPackage(target, improved, improved.Instructions); err != nil {
		return Skill{}, err
	}
	loaded, err := Load(target)
	if err != nil {
		return Skill{}, err
	}
	loaded.Source = "profile"
	return loaded, nil
}

func generatedName(firstUser string, conversationID string) string {
	words := safeWords(firstUser)
	if len(words) == 0 {
		words = safeWords(conversationID)
	}
	if len(words) == 0 {
		words = []string{"session"}
	}
	if len(words) > 4 {
		words = words[:4]
	}
	return strings.Join(append([]string{"session"}, words...), "_")
}

func generatedDescription(firstUser string) string {
	text := compactLine(firstUser, 90)
	if text == "" {
		return "Reusable workflow generated from a completed local session."
	}
	return "Reusable workflow for: " + text
}

func generatedTriggers(firstUser string) []string {
	words := safeWords(firstUser)
	triggers := []string{}
	if strings.TrimSpace(firstUser) != "" {
		triggers = append(triggers, compactLine(firstUser, 64))
	}
	if len(words) >= 2 {
		triggers = append(triggers, strings.Join(words[:2], " "))
	}
	if len(words) >= 3 {
		triggers = append(triggers, strings.Join(words[:3], " "))
	}
	if len(triggers) == 0 {
		triggers = append(triggers, "reusable session workflow")
	}
	return uniqueStrings(triggers)
}

func generatedRequiredTools(input GenerateInput) []string {
	tools := []string{}
	for _, run := range input.ToolRuns {
		name := strings.TrimSpace(run.ToolName)
		if KnownTool(name) {
			tools = append(tools, name)
		}
	}
	text := strings.ToLower(firstUserMessage(input.Messages))
	if strings.Contains(text, "test") {
		tools = append(tools, "run_tests")
	}
	if strings.Contains(text, "diff") || strings.Contains(text, "review") {
		tools = append(tools, "git_diff")
	}
	if strings.Contains(text, "document") || strings.Contains(text, "docs") {
		tools = append(tools, "rag_search")
	}
	if strings.Contains(text, "file") || strings.Contains(text, "code") || strings.Contains(text, "project") {
		tools = append(tools, "read_file")
	}
	if len(tools) == 0 {
		tools = append(tools, "memory_search")
	}
	return uniqueKnownTools(tools)
}

func generatedPermissions(tools []string) Permissions {
	permissions := Permissions{}
	for _, tool := range tools {
		switch tool {
		case "read_file", "list_files", "file_stat", "file_tree", "search_files", "workspace_summary", "rag_search", "project_map", "symbol_search", "secret_scan", "patch_preview":
			permissions.FilesystemRead = true
		case "write_file", "edit_file":
			permissions.FilesystemRead = true
			permissions.FilesystemWrite = "workspace_only"
		case "run_tests", "git_diff", "git_status", "run_shell_safe":
			permissions.Shell = "limited"
		}
	}
	return permissions
}

func generatedInstructions(input GenerateInput, firstUser string, tools []string) string {
	steps := []string{
		"Restate the user's goal in one sentence.",
		"Load only the local context needed for this task.",
		"Use the required tools one action at a time.",
		"Keep any file edits scoped to the workspace.",
		"Verify the result before the final answer.",
	}
	if len(input.ToolRuns) > 0 {
		steps = append(steps, "Reuse the tool order that worked in the source session when it still fits the request.")
	}
	verification := []string{
		"Confirm the answer is grounded in local context.",
		"Check that required tools succeeded or report the failure clearly.",
		"For edits, review the diff and preserve rollback support.",
	}
	mistakes := []string{
		"Do not assume files or tool output that were not provided.",
		"Do not run destructive or network actions from this skill.",
		"Do not expand the prompt with long history when a compact memory is enough.",
	}
	example := compactLine(firstUser, 140)
	if example == "" {
		example = "Repeat the saved workflow for a similar local task."
	}
	parts := []string{
		"# Generated Skill",
		"",
		"## Problem Type",
		compactLine(generatedDescription(firstUser), 180),
		"",
		"## Steps Followed",
		bullets(steps),
		"",
		"## Tools Used",
		bullets(tools),
		"",
		"## Verification Checks",
		bullets(verification),
		"",
		"## Known Mistakes",
		bullets(mistakes),
		"",
		"## Example Usage",
		"- " + example,
	}
	return strings.Join(parts, "\n")
}

func generatedImprovementBlock(input ImproveInput, tools []string) string {
	firstUser := sanitizeSkillText(firstUserMessage(input.Messages))
	successes := successfulToolNames(input.ToolRuns)
	failures := failedToolNames(input.ToolRuns)
	lines := []string{
		"## Improvements From Session",
		"- Source conversation: " + input.ConversationID,
		"- Refined trigger: " + compactLine(firstUser, 120),
		"- Updated required tools: " + strings.Join(tools, ", "),
		"- Keep the workflow local, bounded, and one tool action at a time.",
	}
	if len(successes) > 0 {
		lines = append(lines, "- Reuse successful tools when relevant: "+strings.Join(successes, ", "))
	}
	if len(failures) > 0 {
		lines = append(lines, "- Avoid repeating failed tool paths without checking context first: "+strings.Join(failures, ", "))
	}
	lines = append(lines,
		"- If required context is missing, ask for the exact file, document, or permission needed.",
		"- Preserve snapshot, diff preview, and confirmation requirements for edits.",
	)
	return strings.Join(lines, "\n")
}

func compactImprovedInstructions(existing string, block string, maxChars int) string {
	if maxChars <= 0 {
		maxChars = defaultMaxInstructionChars
	}
	existing = strings.TrimSpace(existing)
	block = strings.TrimSpace(block)
	if existing == "" {
		return trimInstruction(block, maxChars)
	}
	combined := existing + "\n\n" + block
	return trimInstruction(combined, maxChars)
}

func trimInstruction(input string, maxChars int) string {
	input = strings.TrimSpace(input)
	if maxChars <= 0 || len(input) <= maxChars {
		return input
	}
	marker := "\n\n[truncated for low-memory context budget]"
	if maxChars <= len(marker)+32 {
		return strings.TrimSpace(input[:maxChars])
	}
	return strings.TrimSpace(input[:maxChars-len(marker)]) + marker
}

func mergePermissions(base Permissions, add Permissions) Permissions {
	merged := base
	merged.FilesystemRead = merged.FilesystemRead || add.FilesystemRead
	if merged.FilesystemWrite == "" {
		merged.FilesystemWrite = add.FilesystemWrite
	}
	if merged.Shell == "" {
		merged.Shell = add.Shell
	}
	merged.Network = merged.Network || add.Network
	return merged
}

func bumpPatchVersion(version string) string {
	parts := strings.Split(strings.TrimSpace(version), ".")
	if len(parts) != 3 {
		return "0.1.1"
	}
	patch := 0
	_, _ = fmt.Sscanf(parts[2], "%d", &patch)
	return fmt.Sprintf("%s.%s.%d", parts[0], parts[1], patch+1)
}

func successfulToolNames(runs []ToolRunSummary) []string {
	var names []string
	for _, run := range runs {
		status := strings.ToLower(strings.TrimSpace(run.Status))
		if status == "completed" || status == "pass" || status == "passed" {
			names = append(names, run.ToolName)
		}
	}
	return uniqueKnownTools(names)
}

func failedToolNames(runs []ToolRunSummary) []string {
	var names []string
	for _, run := range runs {
		status := strings.ToLower(strings.TrimSpace(run.Status))
		if status == "failed" || status == "error" || status == "blocked" {
			names = append(names, run.ToolName)
		}
	}
	return uniqueKnownTools(names)
}

func firstUserMessage(messages []TranscriptMessage) string {
	for _, msg := range messages {
		if strings.EqualFold(strings.TrimSpace(msg.Role), "user") && strings.TrimSpace(msg.Content) != "" {
			return msg.Content
		}
	}
	for _, msg := range messages {
		if strings.TrimSpace(msg.Content) != "" {
			return msg.Content
		}
	}
	return ""
}

func uniqueSkillName(profileSkillsDir string, base string) string {
	base = strings.Trim(base, "_")
	if base == "" || !skillNamePattern.MatchString(base) {
		base = "session_skill"
	}
	name := base
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(profileSkillsDir, name)); os.IsNotExist(err) {
			return name
		}
		name = fmt.Sprintf("%s_%d", base, i)
	}
}

func safeWords(input string) []string {
	words := strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
	stop := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "can": true,
		"for": true, "from": true, "in": true, "is": true, "me": true,
		"my": true, "of": true, "on": true, "or": true, "the": true,
		"this": true, "to": true, "with": true, "you": true,
	}
	var clean []string
	for _, word := range words {
		word = strings.TrimSpace(word)
		if len(word) < 3 || stop[word] {
			continue
		}
		clean = append(clean, word)
	}
	return uniqueStrings(clean)
}

func uniqueKnownTools(values []string) []string {
	values = uniqueStrings(values)
	sort.Strings(values)
	clean := values[:0]
	for _, value := range values {
		if KnownTool(value) {
			clean = append(clean, value)
		}
	}
	return clean
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	clean := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		clean = append(clean, value)
	}
	return clean
}

func bullets(values []string) string {
	if len(values) == 0 {
		return "- none"
	}
	lines := make([]string, 0, len(values))
	for _, value := range values {
		lines = append(lines, "- "+value)
	}
	return strings.Join(lines, "\n")
}

func compactLine(input string, maxChars int) string {
	text := strings.Join(strings.Fields(input), " ")
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	if maxChars <= 3 {
		return text[:maxChars]
	}
	return strings.TrimSpace(text[:maxChars-3]) + "..."
}

var (
	skillSecretPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(api[_-]?key|token|password|secret)\b\s*[:=]\s*["']?[^"'\s,}]+`),
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{12,}\b`),
		regexp.MustCompile(`\b[A-Za-z0-9_-]{24,}\.[A-Za-z0-9_-]{6,}\.[A-Za-z0-9_-]{20,}\b`),
	}
	skillUserPathPattern = regexp.MustCompile(`/Users/[^/\s]+/`)
)

func sanitizeSkillText(input string) string {
	output := input
	for _, pattern := range skillSecretPatterns {
		output = pattern.ReplaceAllStringFunc(output, redactSkillSecret)
	}
	return skillUserPathPattern.ReplaceAllString(output, "~/")
}

func redactSkillSecret(input string) string {
	if key, _, ok := strings.Cut(input, "="); ok {
		return strings.TrimSpace(key) + "=[REDACTED]"
	}
	if key, _, ok := strings.Cut(input, ":"); ok {
		return strings.TrimSpace(key) + ": [REDACTED]"
	}
	return "[REDACTED]"
}
