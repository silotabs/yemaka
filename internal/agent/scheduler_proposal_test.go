package agent

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/memory"
	"yemaka/internal/scheduler"
)

func TestBuildSchedulerJobProposalKeepsJobUnapprovedAndDisabled(t *testing.T) {
	input := PlanInput{Content: "Create a scheduled job to run heartbeat every hour."}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.TargetType != "heartbeat" || proposal.TargetName != "heartbeat" {
		t.Fatalf("target = %s/%s, want heartbeat/heartbeat", proposal.TargetType, proposal.TargetName)
	}
	if proposal.Intent != "health_check" || proposal.IntentLabel != "Health check" {
		t.Fatalf("intent = %s/%s, want health_check/Health check", proposal.Intent, proposal.IntentLabel)
	}
	if proposal.ScheduleType != "interval" || proposal.ScheduleExpr != "1h" {
		t.Fatalf("schedule = %s/%s, want interval/1h", proposal.ScheduleType, proposal.ScheduleExpr)
	}
	if proposal.Approved || proposal.Enabled {
		t.Fatalf("proposal should stay unapproved and disabled: %+v", proposal)
	}
	if proposal.CLICommand != "yemaka job create interval heartbeat --every 1h --target-type heartbeat --yes" {
		t.Fatalf("CLICommand = %q", proposal.CLICommand)
	}
}

func TestBuildSchedulerJobProposalParsesManualExtensionTarget(t *testing.T) {
	input := PlanInput{Content: "Create a manual job that targets a missing extension name like missing_test_extension."}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.TargetType != "extension" || proposal.TargetName != "missing_test_extension" {
		t.Fatalf("target = %s/%s, want extension/missing_test_extension", proposal.TargetType, proposal.TargetName)
	}
	if proposal.ScheduleType != "manual" || proposal.ScheduleExpr != "" {
		t.Fatalf("schedule = %s/%s, want manual with empty expr", proposal.ScheduleType, proposal.ScheduleExpr)
	}
	if len(proposal.Missing) != 0 || proposal.Approved || proposal.Enabled {
		t.Fatalf("proposal = %+v, want complete unapproved disabled proposal", proposal)
	}
	if proposal.CLICommand != "yemaka job create manual missing_test_extension --target-type extension --yes" {
		t.Fatalf("CLICommand = %q", proposal.CLICommand)
	}
}

func TestBuildSchedulerJobProposalParsesExplicitTargetName(t *testing.T) {
	input := PlanInput{Content: "target_type: extension target_name: website_monitor schedule_type: interval schedule_expr: every 1 hour"}
	plan := BuildPlan(PlanInput{Content: "Create a cron job for monitoring a website every 1 hour."})
	plan.RouteCreatesSchedulerJob = true
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.TargetName != "website_monitor" || proposal.ScheduleType != "interval" || proposal.ScheduleExpr != "1h" {
		t.Fatalf("proposal = %+v, want explicit target and hourly interval", proposal)
	}
}

func TestBuildSchedulerJobProposalParsesCopiedSetupTemplate(t *testing.T) {
	input := PlanInput{Content: strings.Join([]string{
		"scheduler_setup:",
		"target type: heartbeat",
		"target name: heartbeat",
		"schedule type: interval",
		"schedule expr: 1 hour",
		"approved: false",
		"enabled: false",
	}, "\n")}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.TargetType != "heartbeat" || proposal.TargetName != "heartbeat" || proposal.ScheduleType != "interval" || proposal.ScheduleExpr != "1h" {
		t.Fatalf("proposal = %+v, want filled heartbeat interval proposal", proposal)
	}
	if proposal.CLICommand != "yemaka job create interval heartbeat --every 1h --target-type heartbeat --yes" {
		t.Fatalf("CLICommand = %q", proposal.CLICommand)
	}
}

func TestBuildSchedulerJobProposalNormalizesDurationCronSetup(t *testing.T) {
	input := PlanInput{Content: strings.Join([]string{
		"scheduler_setup:",
		"target_type: extension",
		"target_name: web_monitor",
		"schedule_type: cron",
		"schedule_expr: 1h",
	}, "\n")}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.TargetType != "extension" || proposal.TargetName != "web_monitor" {
		t.Fatalf("target = %s/%s, want extension/web_monitor", proposal.TargetType, proposal.TargetName)
	}
	if proposal.ScheduleType != "interval" || proposal.ScheduleExpr != "1h" {
		t.Fatalf("schedule = %s/%s, want interval/1h", proposal.ScheduleType, proposal.ScheduleExpr)
	}
	if proposal.CLICommand != "yemaka job create interval web_monitor --every 1h --target-type extension --yes" {
		t.Fatalf("CLICommand = %q", proposal.CLICommand)
	}
}

func TestBuildSchedulerJobProposalParsesQuotedCommaSetup(t *testing.T) {
	input := PlanInput{Content: strings.Join([]string{
		"scheduler_setup:",
		`"target_type": "extension",`,
		`"target_name": "web_monitor",`,
		`"schedule_type": "cron",`,
		`"schedule_expr": "1h",`,
		`"task": "Russia-Ukraine war news using top news media houses."`,
	}, "\n")}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.TargetType != "extension" || proposal.TargetName != "web_monitor" || proposal.ScheduleType != "interval" || proposal.ScheduleExpr != "1h" {
		t.Fatalf("proposal = %+v, want quoted comma setup normalized to extension interval", proposal)
	}
	if !strings.Contains(proposal.InputJSON, "Russia-Ukraine war news") {
		t.Fatalf("InputJSON = %q, want parsed task", proposal.InputJSON)
	}
	if !strings.Contains(proposal.SetupTemplate, "task: Russia-Ukraine war news using top news media houses.") {
		t.Fatalf("SetupTemplate = %q, want parsed task carried into setup", proposal.SetupTemplate)
	}
}

func TestBuildSchedulerJobProposalInfersInternetSummaryIntent(t *testing.T) {
	input := PlanInput{Content: "Create a background job for latest Russia-Ukraine war news using top news media houses every morning."}
	plan := BuildPlan(input)
	plan.RouteCreatesSchedulerJob = true
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.Intent != "internet_summary" || proposal.IntentLabel != "Internet summary" {
		t.Fatalf("intent = %s/%s, want internet_summary/Internet summary", proposal.Intent, proposal.IntentLabel)
	}
	if proposal.TargetType != "extension" || proposal.TargetName != "" {
		t.Fatalf("target = %s/%s, want extension target missing until generated extension is selected", proposal.TargetType, proposal.TargetName)
	}
	if proposal.ScheduleType != "interval" || proposal.ScheduleExpr != "24h" {
		t.Fatalf("schedule = %s/%s, want interval/24h for every morning", proposal.ScheduleType, proposal.ScheduleExpr)
	}
	if len(proposal.Missing) != 1 || proposal.Missing[0] != "target extension or heartbeat target" {
		t.Fatalf("Missing = %#v, want missing extension target only", proposal.Missing)
	}
	if !strings.Contains(proposal.InputJSON, "Russia-Ukraine war news") {
		t.Fatalf("InputJSON = %q, want default task from request", proposal.InputJSON)
	}
	if !strings.Contains(proposal.SetupHint, "internet") && !strings.Contains(strings.ToLower(proposal.SetupHint), "search") {
		t.Fatalf("SetupHint = %q, want internet/search guidance", proposal.SetupHint)
	}
}

func TestBuildSchedulerJobProposalInfersWebsiteMonitorIntent(t *testing.T) {
	input := PlanInput{Content: "Create a scheduler job to monitor https://example.com and tell me when it is down every hour."}
	plan := BuildPlan(input)
	plan.RouteCreatesSchedulerJob = true
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.Intent != "website_monitor" {
		t.Fatalf("intent = %q, want website_monitor", proposal.Intent)
	}
	if !strings.Contains(proposal.InputJSON, `"url": "https://example.com"`) {
		t.Fatalf("InputJSON = %q, want URL carried into job input", proposal.InputJSON)
	}
	if !strings.Contains(proposal.SetupTemplate, "url: https://example.com") {
		t.Fatalf("SetupTemplate = %q, want URL carried into setup template", proposal.SetupTemplate)
	}
}

func TestBuildSchedulerJobProposalInfersLocalSummaryIntent(t *testing.T) {
	input := PlanInput{Content: "Create a scheduled local document digest for my ingested project notes every week."}
	plan := BuildPlan(input)
	plan.RouteCreatesSchedulerJob = true
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.Intent != "local_summary" || proposal.ScheduleExpr != "168h" {
		t.Fatalf("proposal = %+v, want local_summary weekly interval", proposal)
	}
}

func TestBuildSchedulerJobProposalKeepsFiveFieldCron(t *testing.T) {
	input := PlanInput{Content: strings.Join([]string{
		"scheduler_setup:",
		"target_type: extension",
		"target_name: web_monitor",
		"schedule_type: cron",
		"schedule_expr: 0 8 * * *",
	}, "\n")}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.ScheduleType != "cron" || proposal.ScheduleExpr != "0 8 * * *" {
		t.Fatalf("schedule = %s/%s, want cron/0 8 * * *", proposal.ScheduleType, proposal.ScheduleExpr)
	}
}

func TestBuildSchedulerJobProposalParsesPlainLanguageFilledFields(t *testing.T) {
	input := PlanInput{Content: strings.Join([]string{
		"scheduler setup:",
		"target_type: extension",
		"target_name: website_monitor",
		"every: every 1 hour",
	}, "\n")}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if proposal.TargetType != "extension" || proposal.TargetName != "website_monitor" || proposal.ScheduleType != "interval" || proposal.ScheduleExpr != "1h" {
		t.Fatalf("proposal = %+v, want filled extension interval proposal", proposal)
	}
}

func TestBuildSchedulerJobProposalMarksMissingTargetAndSchedule(t *testing.T) {
	input := PlanInput{Content: "Create a scheduled job for the extension."}
	plan := BuildPlan(input)
	proposal, ok := BuildSchedulerJobProposal(plan, input)
	if !ok {
		t.Fatalf("BuildSchedulerJobProposal() ok = false; plan=%+v", plan)
	}
	if len(proposal.Missing) != 2 {
		t.Fatalf("Missing = %#v, want target and schedule", proposal.Missing)
	}
	if proposal.CLICommand != "" || proposal.Approved || proposal.Enabled {
		t.Fatalf("incomplete proposal should not include command or approval: %+v", proposal)
	}
	if !strings.Contains(proposal.SetupTemplate, "scheduler_setup:") || !strings.Contains(proposal.SetupTemplate, "target_name: <generated_extension_name>") {
		t.Fatalf("SetupTemplate = %q, want copy-fill scheduler template", proposal.SetupTemplate)
	}
	response := SchedulerJobProposalResponse(proposal)
	for _, want := range []string{"Copy, fill, and send this back", "scheduler_setup:", "Jobs stay disabled"} {
		if !strings.Contains(response, want) {
			t.Fatalf("response missing %q:\n%s", want, response)
		}
	}
}

func TestResolveExecutionReturnsSchedulerProposalOnly(t *testing.T) {
	ctx := context.Background()
	db := filepath.Join(t.TempDir(), "memory.sqlite")
	store, err := memory.Open(ctx, db)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	input := PlanInput{Content: "Create a scheduled job to run heartbeat every hour."}
	plan := BuildPlan(input)
	service := &Service{Memory: store}
	var events []Event
	decision, result, edit, content, handled, err := service.resolveExecution(ctx, "conv_scheduler", toolRunMetadata{SessionID: "sess_test"}, plan, &input, func(event Event) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("resolveExecution() error = %v", err)
	}
	if !handled {
		t.Fatalf("handled = false; decision=%+v", decision)
	}
	if result != nil || edit != nil {
		t.Fatalf("result/edit = %+v/%+v, want proposal-only response", result, edit)
	}
	for _, want := range []string{"I did not create a scheduler job", "approved: false", "enabled: false", "yemaka job create interval heartbeat --every 1h --target-type heartbeat --yes"} {
		if !strings.Contains(content, want) {
			t.Fatalf("content missing %q:\n%s", want, content)
		}
	}
	var proposalEvents int
	for _, event := range events {
		if event.Type != EventSchedulerJobProposal {
			continue
		}
		proposalEvents++
		if event.Data["target_type"] != "heartbeat" || event.Data["target_name"] != "heartbeat" {
			t.Fatalf("scheduler proposal event target = %s/%s", event.Data["target_type"], event.Data["target_name"])
		}
		if event.Data["schedule_type"] != "interval" || event.Data["schedule_expr"] != "1h" {
			t.Fatalf("scheduler proposal event schedule = %s/%s", event.Data["schedule_type"], event.Data["schedule_expr"])
		}
		if event.Data["approved"] != "false" || event.Data["enabled"] != "false" || event.Data["can_create"] != "true" {
			t.Fatalf("scheduler proposal event safety flags = %+v", event.Data)
		}
	}
	if proposalEvents != 1 {
		t.Fatalf("scheduler proposal events = %d, want 1; events=%+v", proposalEvents, events)
	}

	runs, err := store.ListToolRuns(ctx, 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var proposalRuns int
	for _, run := range runs {
		if run.ToolName == "permission_request" {
			t.Fatalf("scheduler proposal should not create permission_request: %+v", run)
		}
		if run.ToolName == "scheduler_job_proposal" {
			proposalRuns++
			if run.Status != "proposed" {
				t.Fatalf("scheduler proposal status = %q, want proposed", run.Status)
			}
		}
	}
	if proposalRuns != 1 {
		t.Fatalf("scheduler_job_proposal runs = %d, want 1; runs=%+v", proposalRuns, runs)
	}

	schedulerStore, err := scheduler.Open(ctx, db)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer schedulerStore.Close()
	jobs, err := schedulerStore.List(ctx)
	if err != nil {
		t.Fatalf("scheduler.List() error = %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("scheduler proposal created jobs: %+v", jobs)
	}
}
