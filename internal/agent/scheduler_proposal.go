package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"yemaka/internal/memory"
)

type SchedulerJobProposal struct {
	TargetType    string   `json:"target_type"`
	TargetName    string   `json:"target_name"`
	ScheduleType  string   `json:"schedule_type"`
	ScheduleExpr  string   `json:"schedule_expr"`
	Intent        string   `json:"intent,omitempty"`
	IntentLabel   string   `json:"intent_label,omitempty"`
	IntentSummary string   `json:"intent_summary,omitempty"`
	SetupHint     string   `json:"setup_hint,omitempty"`
	Approved      bool     `json:"approved"`
	Enabled       bool     `json:"enabled"`
	Missing       []string `json:"missing,omitempty"`
	CLICommand    string   `json:"cli_command,omitempty"`
	EnableCommand string   `json:"enable_command,omitempty"`
	SetupTemplate string   `json:"setup_template,omitempty"`
	InputJSON     string   `json:"input_json,omitempty"`
}

func BuildSchedulerJobProposal(plan Plan, input PlanInput) (SchedulerJobProposal, bool) {
	if !plan.RouteCreatesSchedulerJob || plan.RouteGeneratesExtension {
		return SchedulerJobProposal{}, false
	}
	content := strings.ToLower(strings.Join(strings.Fields(input.Content), " "))
	fields := schedulerSetupFields(input.Content)
	intent := inferSchedulerJobIntent(input.Content, fields)
	proposal := SchedulerJobProposal{
		TargetType:    "extension",
		Intent:        intent.Name,
		IntentLabel:   intent.Label,
		IntentSummary: intent.Summary,
		SetupHint:     intent.SetupHint,
		Approved:      false,
		Enabled:       false,
	}
	if len(fields) > 0 {
		applySchedulerSetupFields(&proposal, fields)
	}
	if proposal.TargetName == "" && intent.TargetTypeHint == "heartbeat" {
		proposal.TargetType = "heartbeat"
		proposal.TargetName = "heartbeat"
	} else if proposal.TargetName == "" {
		target := namedSchedulerExtensionTarget(content)
		proposal.TargetName = target
	}
	if proposal.TargetName == "heartbeat" {
		proposal.TargetType = "heartbeat"
	}
	if proposal.TargetName == "" {
		proposal.Missing = append(proposal.Missing, "target extension or heartbeat target")
	}

	if proposal.ScheduleType == "" || (proposal.ScheduleType != "manual" && proposal.ScheduleExpr == "") {
		scheduleType, scheduleExpr := schedulerScheduleFromText(content)
		if proposal.ScheduleType == "" {
			proposal.ScheduleType = scheduleType
		}
		if proposal.ScheduleExpr == "" {
			proposal.ScheduleExpr = scheduleExpr
		}
	}
	if proposal.ScheduleType == "" || (proposal.ScheduleType != "manual" && proposal.ScheduleExpr == "") {
		proposal.Missing = append(proposal.Missing, "exact schedule")
	}
	if proposal.InputJSON == "" && proposal.TargetType == "extension" {
		proposal.InputJSON = schedulerProposalDefaultInputJSON(input.Content, intent)
	}
	proposal.SetupTemplate = schedulerSetupTemplate(proposal)
	if len(proposal.Missing) == 0 {
		switch proposal.ScheduleType {
		case "manual":
			proposal.CLICommand = fmt.Sprintf("yemaka job create manual %s --target-type %s --yes", shellQuote(proposal.TargetName), shellQuote(proposal.TargetType))
		case "interval":
			proposal.CLICommand = fmt.Sprintf("yemaka job create interval %s --every %s --target-type %s --yes", shellQuote(proposal.TargetName), shellQuote(proposal.ScheduleExpr), shellQuote(proposal.TargetType))
		case "cron":
			proposal.CLICommand = fmt.Sprintf("yemaka job create cron %s --cron %s --target-type %s --yes", shellQuote(proposal.TargetName), shellQuote(proposal.ScheduleExpr), shellQuote(proposal.TargetType))
		case "one_time":
			proposal.CLICommand = fmt.Sprintf("yemaka job create one_time %s --at %s --target-type %s --yes", shellQuote(proposal.TargetName), shellQuote(proposal.ScheduleExpr), shellQuote(proposal.TargetType))
		}
		proposal.EnableCommand = "yemaka job enable <job_id>"
	}
	return proposal, true
}

func SchedulerJobProposalResponse(proposal SchedulerJobProposal) string {
	var builder strings.Builder
	builder.WriteString("I did not create a scheduler job. Scheduler jobs are local automation and stay approval-gated.\n\n")
	builder.WriteString("Proposed job:\n")
	if proposal.IntentLabel != "" {
		fmt.Fprintf(&builder, "- intent: %s\n", proposal.IntentLabel)
	}
	fmt.Fprintf(&builder, "- target_type: %s\n", nonEmpty(proposal.TargetType, "missing"))
	fmt.Fprintf(&builder, "- target_name: %s\n", nonEmpty(proposal.TargetName, "missing"))
	fmt.Fprintf(&builder, "- schedule_type: %s\n", nonEmpty(proposal.ScheduleType, "missing"))
	fmt.Fprintf(&builder, "- schedule_expr: %s\n", nonEmpty(proposal.ScheduleExpr, "missing"))
	fmt.Fprintf(&builder, "- approved: %t\n", proposal.Approved)
	fmt.Fprintf(&builder, "- enabled: %t\n", proposal.Enabled)
	if len(proposal.Missing) > 0 {
		fmt.Fprintf(&builder, "\nMissing before creation: %s.\n", strings.Join(proposal.Missing, ", "))
		if proposal.SetupHint != "" {
			fmt.Fprintf(&builder, "Setup guidance: %s\n", proposal.SetupHint)
		}
		builder.WriteString("Copy, fill, and send this back when you have the details:\n\n")
		fmt.Fprintf(&builder, "```text\n%s\n```\n\n", proposal.SetupTemplate)
		builder.WriteString("Use `target_type: heartbeat` and `target_name: heartbeat` for a Yemaka health check. Use `target_type: extension` only after the generated extension exists. Jobs stay disabled until you explicitly enable them.")
		return strings.TrimSpace(builder.String())
	}
	builder.WriteString("\nI can use this filled setup:\n\n")
	fmt.Fprintf(&builder, "```text\n%s\n```\n", proposal.SetupTemplate)
	builder.WriteString("\nNext step: create it from the Automation page, or run:\n")
	fmt.Fprintf(&builder, "`%s`\n\n", proposal.CLICommand)
	builder.WriteString("After reviewing the created job, enable it with:\n")
	fmt.Fprintf(&builder, "`%s`", proposal.EnableCommand)
	return strings.TrimSpace(builder.String())
}

func (s *Service) schedulerJobProposalResponse(ctx context.Context, conversationID string, plan Plan, input PlanInput, decision ExecutionDecision, emit EventHandler) (string, bool, error) {
	if decision.Status == ExecutionNeedsConfirmation {
		return "", false, nil
	}
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		return "", false, nil
	}
	if s != nil && s.Memory != nil {
		_, err := s.Memory.SaveToolRun(ctx, memory.ToolRun{
			ConversationID: conversationID,
			ToolName:       "scheduler_job_proposal",
			Input:          map[string]any{"request": input.Content, "decision": decision},
			Output:         proposal,
			Status:         "proposed",
			RiskLevel:      nonEmpty(plan.RiskLevel, RiskMedium),
		})
		if err != nil {
			return "", false, err
		}
	}
	if err := emitSchedulerJobProposal(emit, proposal, input.Content); err != nil {
		return "", false, err
	}
	return SchedulerJobProposalResponse(proposal), true, nil
}

func emitSchedulerJobProposal(emit EventHandler, proposal SchedulerJobProposal, request string) error {
	if emit == nil {
		return nil
	}
	complete := len(proposal.Missing) == 0
	data := map[string]string{
		"request":           strings.TrimSpace(request),
		"target_type":       proposal.TargetType,
		"target_name":       proposal.TargetName,
		"schedule_type":     proposal.ScheduleType,
		"schedule_expr":     proposal.ScheduleExpr,
		"approved":          fmt.Sprintf("%t", proposal.Approved),
		"enabled":           fmt.Sprintf("%t", proposal.Enabled),
		"missing":           strings.Join(proposal.Missing, ", "),
		"complete":          fmt.Sprintf("%t", complete),
		"can_create":        fmt.Sprintf("%t", complete),
		"cli_command":       proposal.CLICommand,
		"enable_command":    proposal.EnableCommand,
		"setup_template":    proposal.SetupTemplate,
		"input_json":        proposal.InputJSON,
		"intent":            proposal.Intent,
		"intent_label":      proposal.IntentLabel,
		"intent_summary":    proposal.IntentSummary,
		"setup_hint":        proposal.SetupHint,
		"requires_approval": "true",
	}
	return emit(Event{Type: EventSchedulerJobProposal, Message: "Scheduler job proposal", Data: data})
}

func schedulerScheduleFromText(content string) (string, string) {
	if strings.Contains(content, "manual job") || strings.Contains(content, "manual scheduler job") {
		return "manual", ""
	}
	if strings.Contains(content, "every hour") || strings.Contains(content, "hourly") || strings.Contains(content, "each hour") {
		return "interval", "1h"
	}
	if strings.Contains(content, "every morning") {
		return "interval", "24h"
	}
	if strings.Contains(content, "every day") || strings.Contains(content, "daily") || strings.Contains(content, "each day") {
		return "interval", "24h"
	}
	if strings.Contains(content, "every week") || strings.Contains(content, "weekly") || strings.Contains(content, "each week") {
		return "interval", "168h"
	}
	tokens := strings.Fields(content)
	for i := 0; i+2 < len(tokens); i++ {
		if tokens[i] != "every" {
			continue
		}
		count, err := strconv.Atoi(strings.Trim(tokens[i+1], ",."))
		if err != nil || count <= 0 {
			continue
		}
		unit := strings.Trim(tokens[i+2], ",.")
		switch {
		case strings.HasPrefix(unit, "minute"):
			return "interval", fmt.Sprintf("%dm", count)
		case strings.HasPrefix(unit, "hour"):
			return "interval", fmt.Sprintf("%dh", count)
		case strings.HasPrefix(unit, "day"):
			return "interval", fmt.Sprintf("%dh", count*24)
		case strings.HasPrefix(unit, "week"):
			return "interval", fmt.Sprintf("%dh", count*24*7)
		}
	}
	return "", ""
}

type schedulerJobIntent struct {
	Name           string
	Label          string
	Summary        string
	SetupHint      string
	TargetTypeHint string
}

func inferSchedulerJobIntent(content string, fields map[string]string) schedulerJobIntent {
	lower := strings.ToLower(strings.Join(strings.Fields(content), " "))
	targetType := normalizeSchedulerTargetType(fields["target_type"])
	targetName := strings.ToLower(strings.TrimSpace(fields["target_name"]))
	if targetType == "heartbeat" || targetName == "heartbeat" || strings.Contains(lower, "heartbeat") || strings.Contains(lower, "health check") || strings.Contains(lower, "yemaka health") {
		return schedulerJobIntent{
			Name:           "health_check",
			Label:          "Health check",
			Summary:        "Run a local Yemaka heartbeat check on a schedule.",
			SetupHint:      "Use the built-in heartbeat target. The job is created disabled and never runs until enabled.",
			TargetTypeHint: "heartbeat",
		}
	}
	if looksWebsiteMonitorIntent(lower) {
		return schedulerJobIntent{
			Name:      "website_monitor",
			Label:     "Website monitor",
			Summary:   "Check a specific website or URL and report availability changes.",
			SetupHint: "Generate or install the matching web-monitor extension first, then schedule that extension.",
		}
	}
	if looksInternetSummaryIntent(lower) {
		return schedulerJobIntent{
			Name:      "internet_summary",
			Label:     "Internet summary",
			Summary:   "Run an approved internet-capable extension to summarize current external information.",
			SetupHint: "Use an approved generated extension with internet broker permissions. Search/fetch remains disabled until configured and approved.",
		}
	}
	if looksLocalSummaryIntent(lower) {
		return schedulerJobIntent{
			Name:      "local_summary",
			Label:     "Local summary",
			Summary:   "Summarize local documents, notes, or ingested workspace content on a schedule.",
			SetupHint: "Use a local document or notes extension that only reads approved workspace/context sources.",
		}
	}
	if looksReminderIntent(lower) {
		return schedulerJobIntent{
			Name:      "reminder",
			Label:     "Reminder",
			Summary:   "Create a local reminder or checklist-style scheduled task.",
			SetupHint: "Use a local extension or workflow that writes only to approved local memory/notifications.",
		}
	}
	return schedulerJobIntent{
		Name:      "extension_task",
		Label:     "Extension task",
		Summary:   "Run a named generated extension on a local schedule.",
		SetupHint: "Choose an installed generated extension target. If it does not exist yet, generate and register it first.",
	}
}

func looksWebsiteMonitorIntent(lower string) bool {
	hasMonitor := strings.Contains(lower, "monitor") || strings.Contains(lower, "watch") || strings.Contains(lower, "keep an eye")
	hasSite := strings.Contains(lower, "http://") || strings.Contains(lower, "https://") || strings.Contains(lower, "website") || strings.Contains(lower, "site") || strings.Contains(lower, "domain")
	hasAvailability := strings.Contains(lower, "down") || strings.Contains(lower, "up") || strings.Contains(lower, "outage") || strings.Contains(lower, "available") || strings.Contains(lower, "status")
	return hasMonitor && hasSite && hasAvailability
}

func looksInternetSummaryIntent(lower string) bool {
	hasCurrentInfo := strings.Contains(lower, "latest") || strings.Contains(lower, "current") || strings.Contains(lower, "news") || strings.Contains(lower, "trend") || strings.Contains(lower, "headline") || strings.Contains(lower, "market")
	hasExternalSource := strings.Contains(lower, "internet") || strings.Contains(lower, "web") || strings.Contains(lower, "news") || strings.Contains(lower, "site") || strings.Contains(lower, "http://") || strings.Contains(lower, "https://")
	hasReport := strings.Contains(lower, "summary") || strings.Contains(lower, "summarize") || strings.Contains(lower, "digest") || strings.Contains(lower, "report") || strings.Contains(lower, "tell me") || strings.Contains(lower, "notify")
	hasScheduledWork := strings.Contains(lower, "job") || strings.Contains(lower, "schedule") || strings.Contains(lower, "scheduled") || strings.Contains(lower, "background") || strings.Contains(lower, "every ")
	return hasCurrentInfo && hasExternalSource && (hasReport || hasScheduledWork)
}

func looksLocalSummaryIntent(lower string) bool {
	hasLocal := strings.Contains(lower, "local") || strings.Contains(lower, "document") || strings.Contains(lower, "documents") || strings.Contains(lower, "docs") || strings.Contains(lower, "file") || strings.Contains(lower, "folder") || strings.Contains(lower, "notes") || strings.Contains(lower, "ingested")
	hasSummary := strings.Contains(lower, "summary") || strings.Contains(lower, "summarize") || strings.Contains(lower, "digest") || strings.Contains(lower, "review") || strings.Contains(lower, "report")
	return hasLocal && hasSummary
}

func looksReminderIntent(lower string) bool {
	return strings.Contains(lower, "remind me") ||
		strings.Contains(lower, "todo") ||
		strings.Contains(lower, "to-do") ||
		strings.Contains(lower, "checklist") ||
		strings.Contains(lower, "follow up") ||
		strings.Contains(lower, "follow-up")
}

func schedulerSetupFields(content string) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		line = strings.Trim(line, "`")
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "scheduler_setup:" || lower == "scheduler setup:" || lower == "scheduler_setup" || lower == "scheduler setup" {
			fields["scheduler_setup"] = "true"
			continue
		}
		key, value, ok := splitSchedulerSetupLine(line)
		if !ok {
			continue
		}
		key = normalizeSchedulerSetupKey(key)
		if key == "" {
			continue
		}
		value = cleanSchedulerSetupValue(value)
		if value == "" {
			continue
		}
		fields[key] = value
	}
	return fields
}

func splitSchedulerSetupLine(line string) (string, string, bool) {
	if idx := strings.Index(line, ":"); idx > 0 {
		return line[:idx], line[idx+1:], true
	}
	if idx := strings.Index(line, "="); idx > 0 {
		return line[:idx], line[idx+1:], true
	}
	return "", "", false
}

func normalizeSchedulerSetupKey(key string) string {
	key = strings.ToLower(strings.TrimSpace(key))
	key = strings.Trim(key, "`'\"")
	key = strings.ReplaceAll(key, "-", "_")
	key = strings.Join(strings.Fields(key), "_")
	switch key {
	case "target_type", "target_name", "schedule_type", "schedule_expr", "every", "cron", "at", "task", "url":
		return key
	default:
		return ""
	}
}

func cleanSchedulerSetupValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ",")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`'\"")
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, ",")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`'\"")
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(value, "<") && strings.HasSuffix(value, ">") {
		return ""
	}
	switch lower {
	case "missing", "required", "fill", "fill me", "replace me", "todo", "n/a", "none":
		return ""
	default:
		return value
	}
}

func applySchedulerSetupFields(proposal *SchedulerJobProposal, fields map[string]string) {
	if proposal == nil {
		return
	}
	if targetType := normalizeSchedulerTargetType(fields["target_type"]); targetType != "" {
		proposal.TargetType = targetType
	}
	if targetName := cleanSchedulerTargetToken(fields["target_name"]); targetName != "" {
		proposal.TargetName = targetName
	}
	if proposal.TargetName == "heartbeat" {
		proposal.TargetType = "heartbeat"
	}
	scheduleType := normalizeSchedulerScheduleType(fields["schedule_type"])
	scheduleExpr := firstNonEmptyString(fields["schedule_expr"], fields["every"], fields["cron"], fields["at"])
	if scheduleType == "" {
		switch {
		case fields["every"] != "":
			scheduleType = "interval"
		case fields["cron"] != "":
			scheduleType = "cron"
		case fields["at"] != "":
			scheduleType = "one_time"
		}
	}
	scheduleType, scheduleExpr = normalizeSchedulerSchedulePair(scheduleType, scheduleExpr)
	if scheduleType != "" {
		proposal.ScheduleType = scheduleType
	}
	if scheduleExpr != "" {
		proposal.ScheduleExpr = normalizeSchedulerScheduleExpr(scheduleType, scheduleExpr)
	}
	if proposal.ScheduleType == "manual" {
		proposal.ScheduleExpr = ""
	}
	proposal.InputJSON = schedulerProposalInputJSON(fields)
}

func normalizeSchedulerTargetType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "extension", "generated_extension", "tool", "generated_tool":
		return "extension"
	case "heartbeat", "health", "health_check", "yemaka_health":
		return "heartbeat"
	default:
		return ""
	}
}

func normalizeSchedulerScheduleType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	switch value {
	case "manual", "once":
		return "manual"
	case "interval", "every", "recurring":
		return "interval"
	case "cron":
		return "cron"
	case "one_time", "one time", "at":
		return "one_time"
	default:
		return ""
	}
}

func normalizeSchedulerScheduleExpr(scheduleType string, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(value), " "))
	if strings.HasPrefix(normalized, "every ") {
		if _, expr := schedulerScheduleFromText(normalized); expr != "" {
			return expr
		}
	}
	if scheduleType == "interval" {
		switch normalized {
		case "hour", "1 hour", "one hour", "every hour", "hourly":
			return "1h"
		case "day", "1 day", "one day", "every day", "daily":
			return "24h"
		case "week", "1 week", "one week", "every week", "weekly":
			return "168h"
		}
	}
	return value
}

func normalizeSchedulerSchedulePair(scheduleType string, scheduleExpr string) (string, string) {
	scheduleType = normalizeSchedulerScheduleType(scheduleType)
	scheduleExpr = strings.TrimSpace(scheduleExpr)
	if scheduleExpr == "" {
		return scheduleType, ""
	}
	normalized := strings.ToLower(strings.Join(strings.Fields(scheduleExpr), " "))
	if strings.HasPrefix(normalized, "every ") {
		if detectedType, detectedExpr := schedulerScheduleFromText(normalized); detectedExpr != "" {
			return detectedType, detectedExpr
		}
	}
	if scheduleType == "cron" && schedulerLooksLikeIntervalExpr(scheduleExpr) {
		return "interval", normalizeSchedulerScheduleExpr("interval", scheduleExpr)
	}
	if scheduleType == "" && schedulerLooksLikeIntervalExpr(scheduleExpr) {
		return "interval", normalizeSchedulerScheduleExpr("interval", scheduleExpr)
	}
	return scheduleType, normalizeSchedulerScheduleExpr(scheduleType, scheduleExpr)
}

func schedulerLooksLikeIntervalExpr(value string) bool {
	normalized := strings.ToLower(strings.Join(strings.Fields(value), " "))
	if normalized == "" {
		return false
	}
	switch normalized {
	case "hour", "1 hour", "one hour", "every hour", "hourly", "day", "1 day", "one day", "every day", "daily", "week", "1 week", "one week", "every week", "weekly":
		return true
	}
	if strings.HasPrefix(normalized, "every ") {
		_, expr := schedulerScheduleFromText(normalized)
		return expr != ""
	}
	duration, err := time.ParseDuration(normalized)
	return err == nil && duration > 0
}

func schedulerProposalInputJSON(fields map[string]string) string {
	input := map[string]string{}
	if task := strings.TrimSpace(fields["task"]); task != "" {
		input["task"] = task
	}
	if url := strings.TrimSpace(fields["url"]); url != "" {
		input["url"] = url
	}
	if len(input) == 0 {
		return ""
	}
	encoded, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return ""
	}
	return string(encoded)
}

func schedulerProposalDefaultInputJSON(request string, intent schedulerJobIntent) string {
	input := map[string]string{}
	if url := schedulerFirstURL(request); url != "" {
		input["url"] = url
	}
	task := schedulerDefaultTask(request, intent)
	if task != "" {
		input["task"] = task
	}
	if len(input) == 0 {
		return ""
	}
	encoded, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		return ""
	}
	return string(encoded)
}

func schedulerDefaultTask(request string, intent schedulerJobIntent) string {
	clean := strings.Join(strings.Fields(request), " ")
	if clean != "" && !strings.HasPrefix(strings.ToLower(clean), "scheduler_setup:") {
		if len(clean) > 240 {
			return strings.TrimSpace(clean[:240]) + "..."
		}
		return clean
	}
	switch intent.Name {
	case "website_monitor":
		return "Check the target website and report whether it is down."
	case "internet_summary":
		return "Collect approved current information and summarize the result."
	case "local_summary":
		return "Summarize the selected local documents or notes."
	case "reminder":
		return "Send a local reminder or checklist update."
	default:
		return ""
	}
}

func schedulerFirstURL(value string) string {
	for _, token := range strings.Fields(value) {
		candidate := strings.Trim(token, "`'\"()[]{}<>,.;")
		lower := strings.ToLower(candidate)
		if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
			return candidate
		}
	}
	return ""
}

func schedulerSetupTemplate(proposal SchedulerJobProposal) string {
	targetType := nonEmpty(proposal.TargetType, "extension")
	targetName := proposal.TargetName
	if targetName == "" {
		if targetType == "heartbeat" {
			targetName = "heartbeat"
		} else {
			targetName = "<generated_extension_name>"
		}
	}
	scheduleType := nonEmpty(proposal.ScheduleType, "interval")
	scheduleExpr := proposal.ScheduleExpr
	if scheduleExpr == "" && scheduleType != "manual" {
		scheduleExpr = "<1h>"
	}
	lines := []string{
		"scheduler_setup:",
		"target_type: " + targetType,
		"target_name: " + targetName,
		"schedule_type: " + scheduleType,
		"schedule_expr: " + scheduleExpr,
	}
	if targetType == "extension" {
		if input := schedulerSetupInputFields(proposal.InputJSON); len(input) > 0 {
			if task := strings.TrimSpace(input["task"]); task != "" {
				lines = append(lines, "task: "+task)
			}
			if url := strings.TrimSpace(input["url"]); url != "" {
				lines = append(lines, "url: "+url)
			}
		} else {
			lines = append(lines, "task: <what this job should do>")
		}
	}
	return strings.Join(lines, "\n")
}

func schedulerSetupInputFields(inputJSON string) map[string]string {
	if strings.TrimSpace(inputJSON) == "" {
		return nil
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(inputJSON), &raw); err != nil {
		return nil
	}
	result := map[string]string{}
	for _, key := range []string{"task", "url"} {
		if value, ok := raw[key].(string); ok && strings.TrimSpace(value) != "" {
			result[key] = strings.TrimSpace(value)
		}
	}
	return result
}

func namedSchedulerExtensionTarget(content string) string {
	tokens := strings.Fields(content)
	for i, token := range tokens {
		token = strings.TrimSpace(token)
		if candidate := explicitSchedulerTargetName(token, i, tokens); candidate != "" {
			return candidate
		}
	}
	for i, token := range tokens {
		if token != "extension" && token != "tool" {
			continue
		}
		for j := i + 1; j < len(tokens) && j <= i+5; j++ {
			candidate := cleanSchedulerTargetToken(tokens[j])
			if candidate == "" || schedulerTargetConnectorWord(candidate) {
				continue
			}
			return candidate
		}
	}
	return ""
}

func explicitSchedulerTargetName(token string, index int, tokens []string) string {
	lower := strings.ToLower(strings.TrimSpace(token))
	for _, prefix := range []string{"target_name", "target-name"} {
		if strings.HasPrefix(lower, prefix+"=") || strings.HasPrefix(lower, prefix+":") {
			if candidate := cleanSchedulerTargetToken(token[len(prefix)+1:]); candidate != "" {
				return candidate
			}
		}
		if lower == prefix || lower == prefix+":" || lower == prefix+"=" {
			if index+1 < len(tokens) {
				return cleanSchedulerTargetToken(tokens[index+1])
			}
		}
	}
	if lower == "target" && index+2 < len(tokens) && strings.Trim(strings.ToLower(tokens[index+1]), "`'\".,:;()[]{}") == "name" {
		return cleanSchedulerTargetToken(tokens[index+2])
	}
	return ""
}

func cleanSchedulerTargetToken(token string) string {
	token = strings.TrimSpace(token)
	token = strings.Trim(token, "`'\".,:;()[]{}")
	if token == "" {
		return ""
	}
	return token
}

func schedulerTargetConnectorWord(token string) bool {
	switch strings.ToLower(strings.TrimSpace(token)) {
	case "a", "an", "the", "name", "named", "called", "like", "as", "target", "targets", "missing", "extension", "tool", "with":
		return true
	default:
		return false
	}
}
