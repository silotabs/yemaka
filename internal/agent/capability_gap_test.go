package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/capabilities"
	"yemaka/internal/config"
	"yemaka/internal/domainpacks"
	"yemaka/internal/extensions"
)

type recordingCapabilityGenerator struct {
	proposal extensions.Proposal
	called   bool
}

func (g *recordingCapabilityGenerator) Propose(request string) (extensions.Proposal, error) {
	if strings.TrimSpace(g.proposal.Name) != "" {
		return g.proposal, nil
	}
	return extensions.Proposal{
		Name:             "website_monitor",
		Description:      request,
		Type:             extensions.TypeTool,
		Reason:           "test proposal",
		RequiresApproval: true,
		Files:            []string{extensions.ManifestFile, "main.go", "main_test.go"},
		Permissions:      []string{"network=core_broker", "network.methods=GET,HEAD"},
		NetworkMode:      "core_broker",
		AllowedDomains:   []string{"example.com"},
		AllowedMethods:   []string{"GET", "HEAD"},
	}, nil
}

func (g *recordingCapabilityGenerator) Generate(ctx context.Context, input extensions.GenerateInput) (extensions.GenerationResult, error) {
	g.called = true
	return extensions.GenerationResult{}, nil
}

func TestCapabilityGapRouterScheduledWebsiteProposalNamesBrokerAndSchedulerApproval(t *testing.T) {
	cfg := config.Default()
	generator := &recordingCapabilityGenerator{}
	router := NewCapabilityGapRouter(cfg, generator)

	proposal, err := router.ProposeRequest("Monitor example.com every hour and report when the page title changes")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Kind != CapabilityKindWorkflow {
		t.Fatalf("proposal.Kind = %q, want %q", proposal.Kind, CapabilityKindWorkflow)
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("proposal.NetworkMode = %q, want core_broker", proposal.NetworkMode)
	}
	if got := strings.Join(proposal.AllowedDomains, ","); got != "example.com" {
		t.Fatalf("AllowedDomains = %q, want example.com", got)
	}
	joined := strings.Join([]string{
		proposal.SuggestedAction,
		strings.Join(proposal.Permissions, " "),
		proposal.Response(),
	}, " ")
	for _, want := range []string{"core internet broker", "scheduler job", "explicitly approved", "scheduler=approval_required", "methods=GET,HEAD"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("proposal text = %q, want %q", joined, want)
		}
	}
	if strings.Contains(strings.ToLower(joined), "enabled by default") {
		t.Fatalf("proposal text = %q, must not imply defaults are enabled", joined)
	}
}

func TestCapabilityGapRouterGenericDailyRoutineDoesNotUseURLMonitoringCopy(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, &recordingCapabilityGenerator{proposal: extensions.Proposal{
		Name:             "daily_routine",
		Description:      "Create a daily routine workflow that reminds me every morning to review my study notes",
		Type:             extensions.TypeTool,
		Reason:           "test proposal",
		RequiresApproval: true,
		Files:            []string{extensions.ManifestFile, "main.go", "main_test.go"},
		Permissions:      []string{"scheduler=approval_required", "policy=core_enforced"},
	}})

	proposal, err := router.ProposeRequest("Create a daily routine workflow that reminds me every morning to review my study notes")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	joined := strings.ToLower(strings.Join([]string{
		proposal.Title,
		proposal.Description,
		proposal.CapabilitySummary,
		proposal.SuggestedAction,
		proposal.Response(),
	}, " "))
	for _, unwanted := range []string{"url", "webpage", "website", "title changes", "allowed domain", "allowed domains"} {
		if strings.Contains(joined, unwanted) {
			t.Fatalf("generic routine proposal text = %q, did not expect URL-monitoring term %q", joined, unwanted)
		}
	}
}

func TestCapabilityGapRouterExplicitExtensionRequestUsesCleanSubject(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir()))

	proposal, err := router.ProposeRequest("okay first generate or create extension for web monitoring")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if !proposal.Needed {
		t.Fatalf("proposal.Needed = false, want capability proposal")
	}
	if proposal.Name != "web_monitoring" {
		t.Fatalf("proposal.Name = %q, want web_monitoring", proposal.Name)
	}
	if proposal.Description != "web monitoring" {
		t.Fatalf("proposal.Description = %q, want web monitoring", proposal.Description)
	}
	for _, unwanted := range []string{"okay", "generate or create extension"} {
		if strings.Contains(strings.ToLower(proposal.Name), unwanted) {
			t.Fatalf("proposal.Name = %q, did not expect %q", proposal.Name, unwanted)
		}
	}
}

func TestCapabilityGapRouterResponseHidesRawCapabilityMetadata(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, extensions.NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), t.TempDir()))

	proposal, err := router.ProposeRequest("okay first generate or create extension for web monitoring")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	response := proposal.Response()
	for _, want := range []string{"Proposed next capability", "web monitoring", "without your approval"} {
		if !strings.Contains(strings.ToLower(response), strings.ToLower(want)) {
			t.Fatalf("response = %q, want %q", response, want)
		}
	}
	for _, unwanted := range []string{
		"state=",
		"source=",
		"installed=",
		"enabled=",
		"valid=",
		"Generated files:",
		"Generate:",
		"filesystem.read=false",
		"yemaka extension generate",
	} {
		if strings.Contains(response, unwanted) {
			t.Fatalf("response = %q, did not expect raw metadata %q", response, unwanted)
		}
	}
}

func TestCapabilityGapRouterExplanationPromptDoesNotProposeExtension(t *testing.T) {
	cfg := config.Default()
	router := NewCapabilityGapRouter(cfg, &recordingCapabilityGenerator{})

	proposal, err := router.ProposeRequest("Explain how website monitoring works.")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Needed {
		t.Fatalf("proposal = %+v, want no capability gap for explanation prompt", proposal)
	}
	if strings.Contains(proposal.SuggestedAction, "generate") || strings.Contains(proposal.SuggestedAction, "enable") {
		t.Fatalf("SuggestedAction = %q, want normal explanation guidance only", proposal.SuggestedAction)
	}
}

func TestCapabilityGapRouterGenerateRequiresExplicitApprovalBeforeGenerator(t *testing.T) {
	cfg := config.Default()
	generator := &recordingCapabilityGenerator{}
	router := NewCapabilityGapRouter(cfg, generator)

	_, err := router.Generate(t.Context(), generator, CapabilityGenerationInput{
		Request: "Monitor example.com every hour and report when the page title changes",
		Name:    "website_monitor",
	})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("Generate() error = %v, want explicit approval requirement", err)
	}
	if generator.called {
		t.Fatal("Generate() called extension generator before explicit approval")
	}
}

func TestCapabilityGapRouterUsesRegistrySummaryForDisabledInternetDefault(t *testing.T) {
	router := NewCapabilityGapRouter(nil, &recordingCapabilityGenerator{})

	proposal, err := router.ProposeRequest("Search the web for local-first agents")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Kind != CapabilityKindSettings {
		t.Fatalf("proposal.Kind = %q, want settings", proposal.Kind)
	}
	if proposal.CanGenerate {
		t.Fatal("proposal.CanGenerate = true, want false for disabled internet default")
	}
	if proposal.CapabilityAction != "ask_config" || proposal.CapabilityState != "disabled" {
		t.Fatalf("registry decision = %q/%q, want ask_config/disabled", proposal.CapabilityAction, proposal.CapabilityState)
	}
	if !strings.Contains(proposal.CapabilitySummary, "Capability needs setup") || !strings.Contains(proposal.CapabilitySummary, "internet access is disabled") {
		t.Fatalf("CapabilitySummary = %q, want registry-backed disabled internet summary", proposal.CapabilitySummary)
	}
	if !proposal.RequiresApproval {
		t.Fatal("proposal.RequiresApproval = false, want approval for enabling internet")
	}
}

func TestCapabilityGapRouterRegistryDefaultsDoNotEnableGenerationConnectorsOrJobs(t *testing.T) {
	router := NewCapabilityGapRouter(nil, &recordingCapabilityGenerator{})
	tests := []struct {
		name        string
		request     string
		wantKind    string
		wantSummary string
	}{
		{
			name:        "extension generation disabled",
			request:     "Build a tool that converts CSV notes into study flashcards",
			wantKind:    CapabilityKindToolExtension,
			wantSummary: "generated extensions are disabled",
		},
		{
			name:        "connector disabled",
			request:     "Send a Slack message that the build passed",
			wantKind:    CapabilityKindConnector,
			wantSummary: "connectors are disabled",
		},
		{
			name:        "scheduler disabled",
			request:     "Create a scheduled job that runs every hour to summarize release notes",
			wantKind:    CapabilityKindWorkflow,
			wantSummary: "scheduler support is disabled",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proposal, err := router.ProposeRequest(tc.request)
			if err != nil {
				t.Fatalf("ProposeRequest() error = %v", err)
			}
			if proposal.Kind != tc.wantKind {
				t.Fatalf("proposal.Kind = %q, want %q; proposal=%+v", proposal.Kind, tc.wantKind, proposal)
			}
			if proposal.CanGenerate {
				t.Fatalf("proposal.CanGenerate = true, want false while registry default is disabled; proposal=%+v", proposal)
			}
			if proposal.CapabilityAction != "ask_config" || proposal.CapabilityState != "disabled" {
				t.Fatalf("registry decision = %q/%q, want ask_config/disabled; summary=%q", proposal.CapabilityAction, proposal.CapabilityState, proposal.CapabilitySummary)
			}
			if !strings.Contains(proposal.CapabilitySummary, tc.wantSummary) {
				t.Fatalf("CapabilitySummary = %q, want %q", proposal.CapabilitySummary, tc.wantSummary)
			}
			if !proposal.RequiresApproval {
				t.Fatal("proposal.RequiresApproval = false, want approval-gated capability setup")
			}
		})
	}
}

func TestCapabilityGapRouterUsesEnabledDomainPackBeforeGeneration(t *testing.T) {
	cfg, root := domainPackRouterTestConfig(t)
	installRouterDomainPack(t, root, true, `
name: csv_tools
version: 0.1.0
description: Normalize CSV rows.
category: data
optional_tools:
  - safe_tool
default_state: disabled
`)

	router := NewCapabilityGapRouter(cfg, &recordingCapabilityGenerator{})
	proposal, err := router.ProposeRequest("Build a tool to normalize CSV rows")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if proposal.Needed {
		t.Fatalf("proposal.Needed = true, want existing enabled domain pack; proposal=%+v", proposal)
	}
	if proposal.Kind != string(capabilities.KindDomainPack) || proposal.ExistingCapability != "csv_tools" {
		t.Fatalf("proposal = %+v, want csv_tools domain pack selected", proposal)
	}
	if proposal.CapabilityAction != string(capabilities.ActionUsePack) || proposal.CapabilityState != string(capabilities.StateReady) {
		t.Fatalf("capability decision = %q/%q, want use_pack/ready; proposal=%+v", proposal.CapabilityAction, proposal.CapabilityState, proposal)
	}
	if proposal.CanGenerate {
		t.Fatal("proposal.CanGenerate = true, want no generation when enabled pack matches")
	}
	if proposal.RuntimeStatus == nil || !proposal.RuntimeStatus.Installed || !proposal.RuntimeStatus.Enabled || !proposal.RuntimeStatus.Valid {
		t.Fatalf("RuntimeStatus = %+v, want installed/enabled/valid pack status", proposal.RuntimeStatus)
	}
	if proposal.RuntimeStatus.Source != "domain_pack" || proposal.RuntimeStatus.State != string(capabilities.StateReady) {
		t.Fatalf("RuntimeStatus = %+v, want domain_pack ready state", proposal.RuntimeStatus)
	}
	response := proposal.Response()
	if strings.Contains(response, "Generate a small tool extension") || !strings.Contains(response, "policy=core_enforced") || !strings.Contains(response, "Capability status") {
		t.Fatalf("response = %q, want existing pack response with core policy", response)
	}
}

func TestCapabilityGapRouterDisabledDomainPackIsNotUsable(t *testing.T) {
	cfg, root := domainPackRouterTestConfig(t)
	installRouterDomainPack(t, root, false, `
name: csv_tools
version: 0.1.0
description: Normalize CSV rows.
category: data
optional_tools:
  - safe_tool
default_state: enabled
`)

	router := NewCapabilityGapRouter(cfg, &recordingCapabilityGenerator{})
	proposal, err := router.ProposeRequest("Build a tool to normalize CSV rows")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if !proposal.Needed {
		t.Fatalf("proposal.Needed = false, want setup step for disabled pack; proposal=%+v", proposal)
	}
	if proposal.CapabilityAction == string(capabilities.ActionUsePack) || proposal.CapabilityState == string(capabilities.StateReady) {
		t.Fatalf("proposal = %+v, disabled pack was treated as usable", proposal)
	}
	if proposal.CapabilityAction != string(capabilities.ActionAskConfig) || proposal.CapabilityState != string(capabilities.StateDisabled) {
		t.Fatalf("capability decision = %q/%q, want ask_config/disabled; proposal=%+v", proposal.CapabilityAction, proposal.CapabilityState, proposal)
	}
	if proposal.CanGenerate {
		t.Fatal("proposal.CanGenerate = true, want explicit pack enablement before generation")
	}
	if proposal.ExistingCapability != "csv_tools" {
		t.Fatalf("ExistingCapability = %q, want csv_tools setup target", proposal.ExistingCapability)
	}
	if proposal.ConfigureHint == "" {
		t.Fatalf("ConfigureHint is empty, want domain-pack setup hint")
	}
	if proposal.PackSource != "domain_pack" {
		t.Fatalf("PackSource = %q, want domain_pack", proposal.PackSource)
	}
	if !strings.HasPrefix(proposal.InstallCommand, "yemaka domain-pack enable csv_tools") {
		t.Fatalf("InstallCommand = %q, want enable command for disabled pack", proposal.InstallCommand)
	}
	if proposal.RuntimeStatus == nil || !proposal.RuntimeStatus.Installed || proposal.RuntimeStatus.Enabled || !proposal.RuntimeStatus.Valid {
		t.Fatalf("RuntimeStatus = %+v, want installed/disabled/valid pack status", proposal.RuntimeStatus)
	}
	if proposal.RuntimeStatus.State != string(capabilities.StateDisabled) || proposal.RuntimeStatus.Source != "domain_pack" {
		t.Fatalf("RuntimeStatus = %+v, want disabled domain pack state", proposal.RuntimeStatus)
	}

	var events []Event
	if err := emitCapabilityGap(func(event Event) error {
		events = append(events, event)
		return nil
	}, proposal, "Build a tool to normalize CSV rows"); err != nil {
		t.Fatalf("emitCapabilityGap() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	data := events[0].Data
	for key, want := range map[string]string{
		"kind":                string(capabilities.KindDomainPack),
		"existing_capability": "csv_tools",
		"capability_action":   string(capabilities.ActionAskConfig),
		"capability_state":    string(capabilities.StateDisabled),
		"pack_source":         "domain_pack",
		"install_command":     "yemaka domain-pack enable csv_tools",
		"runtime_source":      "domain_pack",
		"runtime_state":       string(capabilities.StateDisabled),
		"runtime_installed":   "true",
		"runtime_enabled":     "false",
		"runtime_valid":       "true",
	} {
		if data[key] != want {
			t.Fatalf("event data[%q] = %q, want %q; data=%#v", key, data[key], want, data)
		}
	}
}

func TestCapabilityGapRouterSuggestsWorkflowTemplateBeforeGeneration(t *testing.T) {
	cfg, _ := domainPackRouterTestConfig(t)
	router := NewCapabilityGapRouter(cfg, &recordingCapabilityGenerator{})

	proposal, err := router.ProposeRequest("Plan my day and save follow-up tasks")
	if err != nil {
		t.Fatalf("ProposeRequest() error = %v", err)
	}
	if !proposal.Needed {
		t.Fatalf("proposal.Needed = false, want setup proposal for workflow template; proposal=%+v", proposal)
	}
	if proposal.Kind != string(capabilities.KindDomainPack) || proposal.Name != "personal_productivity" {
		t.Fatalf("proposal = %+v, want personal_productivity domain pack template", proposal)
	}
	if proposal.CanGenerate {
		t.Fatalf("proposal.CanGenerate = true, want install/enable pack path before generation; proposal=%+v", proposal)
	}
	if proposal.CapabilityAction != string(capabilities.ActionAskConfig) || proposal.CapabilityState != string(capabilities.StateNeedsConfig) {
		t.Fatalf("capability decision = %q/%q, want ask_config/needs_config; proposal=%+v", proposal.CapabilityAction, proposal.CapabilityState, proposal)
	}
	if proposal.PackSource != "domain_pack_template" {
		t.Fatalf("PackSource = %q, want domain_pack_template", proposal.PackSource)
	}
	if proposal.InstallCommand != "yemaka domain-pack install-template personal_productivity" {
		t.Fatalf("InstallCommand = %q, want template install command", proposal.InstallCommand)
	}
	joined := strings.Join([]string{proposal.Title, proposal.Reason, proposal.SuggestedAction, proposal.Response()}, " ")
	for _, want := range []string{"Install the matching domain pack", "workflow pack template", "Install the local workflow pack template"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("proposal text = %q, want %q", joined, want)
		}
	}
	if strings.Contains(joined, "Generate a small tool extension") {
		t.Fatalf("proposal text = %q, must not jump to generated extension while a template matches", joined)
	}
}

func TestCapabilityGapRouterBuiltInTemplatePackSelectionMatrix(t *testing.T) {
	cfg, _ := domainPackRouterTestConfig(t)
	router := NewCapabilityGapRouter(cfg, &recordingCapabilityGenerator{})

	tests := []struct {
		name     string
		request  string
		wantPack string
	}{
		{
			name:     "draft editor",
			request:  "Review this draft for clarity and make it more concise.",
			wantPack: "local_draft_editor",
		},
		{
			name:     "career notes",
			request:  "Compare resume notes with this role description and list fit gaps.",
			wantPack: "local_career_notes",
		},
		{
			name:     "coding notes",
			request:  "Review these local coding notes and list API questions.",
			wantPack: "local_coding_notes",
		},
		{
			name:     "contract notes",
			request:  "Review these contract notes and list questions.",
			wantPack: "local_contract_notes",
		},
		{
			name:     "content calendar notes",
			request:  "Create a content calendar outline from my local campaign notes.",
			wantPack: "local_content_calendar_notes",
		},
		{
			name:     "customer support notes",
			request:  "Summarize this local support ticket and list missing customer details.",
			wantPack: "local_customer_support_notes",
		},
		{
			name:     "data notes",
			request:  "Summarize table notes and identify repeated items.",
			wantPack: "local_data_notes",
		},
		{
			name:     "email drafts",
			request:  "Draft a polite email from these notes.",
			wantPack: "local_email_drafts",
		},
		{
			name:     "finance notes",
			request:  "Review these budget notes and summarize expense categories.",
			wantPack: "local_finance_notes",
		},
		{
			name:     "form prep notes",
			request:  "Make a form field checklist from these local notes.",
			wantPack: "local_form_prep_notes",
		},
		{
			name:     "project brief",
			request:  "Give me a local project status brief from the ingested project notes.",
			wantPack: "local_project_brief",
		},
		{
			name:     "project milestone plan",
			request:  "Draft a milestone review plan from my local project notes without assigning work.",
			wantPack: "local_project_brief",
		},
		{
			name:     "meeting notes",
			request:  "Summarize meeting notes and extract action items.",
			wantPack: "local_meeting_notes",
		},
		{
			name:     "study helper",
			request:  "Make a study guide from my local notes and create practice questions.",
			wantPack: "local_study_helper",
		},
		{
			name:     "file triage",
			request:  "Inventory local files and propose file cleanup.",
			wantPack: "local_file_triage",
		},
		{
			name:     "file organization plan",
			request:  "Propose a folder organization plan for my workspace without changing files.",
			wantPack: "local_file_triage",
		},
		{
			name:     "health notes",
			request:  "Appointment note prep for my local health notes.",
			wantPack: "local_health_notes",
		},
		{
			name:     "local knowledge base",
			request:  "Create a local knowledge base FAQ from my ingested docs.",
			wantPack: "local_knowledge_base",
		},
		{
			name:     "maintenance notes",
			request:  "Review these maintenance log notes and prepare repair visit questions.",
			wantPack: "local_maintenance_notes",
		},
		{
			name:     "research assistant",
			request:  "Create a source backed brief and check this claim against my local docs.",
			wantPack: "research_assistant",
		},
		{
			name:     "sales client notes",
			request:  "Create a client call brief from my local sales notes.",
			wantPack: "local_sales_client_notes",
		},
		{
			name:     "security privacy notes",
			request:  "Create an account hygiene checklist from my local privacy notes.",
			wantPack: "local_security_privacy_notes",
		},
		{
			name:     "travel notes",
			request:  "Summarize these trip notes into an itinerary checklist.",
			wantPack: "local_travel_notes",
		},
		{
			name:     "personal productivity",
			request:  "Plan my day and save follow-up tasks.",
			wantPack: "personal_productivity",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			decision := router.CapabilityRegistry.ClassifyGap(capabilities.GapQuery{
				Request: tc.request,
				Kind:    capabilities.KindDomainPack,
			})
			if decision.Action != capabilities.ActionAskConfig ||
				decision.State != capabilities.StateNeedsConfig ||
				decision.Name != tc.wantPack {
				t.Fatalf("decision = %+v, want setup-only template %s", decision, tc.wantPack)
			}
			if decision.Capability == nil || decision.Capability.Metadata["template"] != "true" {
				t.Fatalf("decision.Capability = %+v, want built-in template metadata", decision.Capability)
			}
			if decision.Capability.Source != "domain_pack_template" {
				t.Fatalf("Capability.Source = %q, want domain_pack_template; decision=%+v", decision.Capability.Source, decision)
			}
			if decision.Action == capabilities.ActionUsePack || decision.State == capabilities.StateReady {
				t.Fatalf("decision = %+v, template must remain setup-only until installed and enabled", decision)
			}
			wantInstall := "yemaka domain-pack install-template " + tc.wantPack
			proposal, ok := packCapabilityProposal(tc.request, ExecutionDecision{}, decision)
			if !ok {
				t.Fatalf("packCapabilityProposal() ok = false, want setup proposal for %s", tc.wantPack)
			}
			if proposal.InstallCommand != wantInstall || proposal.PackSource != "domain_pack_template" || proposal.CanGenerate {
				t.Fatalf("proposal = %+v, want install command %q and no generation", proposal, wantInstall)
			}
		})
	}
}

func TestCapabilityGapRouterBuiltInTemplatesDoNotStealCapabilityGaps(t *testing.T) {
	router := NewCapabilityGapRouter(nil, &recordingCapabilityGenerator{})

	tests := []struct {
		name       string
		request    string
		wantKind   string
		wantNeeded *bool
	}{
		{
			name:     "generated extension remains extension setup",
			request:  "Build a tool that converts CSV notes into study flashcards",
			wantKind: CapabilityKindToolExtension,
		},
		{
			name:     "scheduled job remains scheduler setup",
			request:  "Create a scheduled job that runs every hour to summarize release notes",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "website monitor remains workflow setup",
			request:  "Monitor example.com every hour and report when the page title changes",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "website down monitor remains workflow setup without explicit interval",
			request:  "Create a sub agent that will monitor https://example.com and tell me when it is down",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "connector action remains connector setup",
			request:  "Send a Slack message that the build passed",
			wantKind: CapabilityKindConnector,
		},
		{
			name:     "web search remains internet setup",
			request:  "Search the web for local-first agents",
			wantKind: CapabilityKindSettings,
		},
		{
			name:     "data update search remains internet setup",
			request:  "Search the web for the latest dataset updates",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "code edit does not become coding notes pack",
			request: "Fix this code and apply patch",
		},
		{
			name:    "test run does not become coding notes pack",
			request: "Run tests for this Go package",
		},
		{
			name:    "dependency install does not become coding notes pack",
			request: "Install dependency and update this file",
		},
		{
			name:    "git action does not become coding notes pack",
			request: "Git commit these code changes",
		},
		{
			name:     "coding docs search remains internet setup",
			request:  "Search the web for the latest API documentation",
			wantKind: CapabilityKindSettings,
		},
		{
			name:     "code generator remains extension setup",
			request:  "Build a tool that generates code comments from source files",
			wantKind: CapabilityKindToolExtension,
		},
		{
			name:       "normal explanation does not ask for pack",
			request:    "Explain what a CEO does",
			wantKind:   "none",
			wantNeeded: boolPtr(false),
		},
		{
			name:       "pasted svg cleanup remains normal chat",
			request:    `Make this SVG cleaner: <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`,
			wantKind:   "none",
			wantNeeded: boolPtr(false),
		},
		{
			name:    "csv file output request does not become pack",
			request: "Convert this CSV into a new file",
		},
		{
			name:    "file move action does not become file triage pack",
			request: "Move these files into project folders",
		},
		{
			name:    "file rename action does not become file triage pack",
			request: "Rename these duplicate files",
		},
		{
			name:    "file delete action does not become file triage pack",
			request: "Delete old build artifacts",
		},
		{
			name:    "folder creation action does not become file triage pack",
			request: "Create folders and move these files",
		},
		{
			name:    "project task assignment does not become project pack",
			request: "Assign these tasks to the team from the project notes",
		},
		{
			name:    "project ticket creation does not become project pack",
			request: "Create tickets from these milestone notes",
		},
		{
			name:    "project board update does not become project pack",
			request: "Update the project board with these blockers",
		},
		{
			name:    "project deadline scheduling does not become project pack",
			request: "Schedule deadlines for this project plan",
		},
		{
			name:    "project status send does not become project pack",
			request: "Send status update to the team",
		},
		{
			name:    "project plan file write does not become project pack",
			request: "Write project plan to a file",
		},
		{
			name:    "command log parsing request does not become pack",
			request: "Run a command to parse this log file",
		},
		{
			name:     "career web search remains internet setup",
			request:  "Search the web for remote design roles",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "career message request does not become pack",
			request: "Send my resume notes to this recruiter",
		},
		{
			name:    "online application request does not become pack",
			request: "Apply to this role online",
		},
		{
			name:    "diagnosis request does not become pack",
			request: "Diagnose these symptoms and tell me what treatment to take",
		},
		{
			name:     "health web search remains internet setup",
			request:  "Search the web for the latest treatment updates",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "health email request does not become pack",
			request: "Email my health notes to my doctor",
		},
		{
			name:     "send email request remains connector setup",
			request:  "Send this email to the project sponsor",
			wantKind: CapabilityKindConnector,
		},
		{
			name:     "email reminder remains scheduler setup",
			request:  "Create a scheduled job to remind me to send a follow-up email",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "email web search remains internet setup",
			request:  "Search the web for the latest email deliverability rules",
			wantKind: CapabilityKindSettings,
		},
		{
			name:     "health reminder remains scheduler setup",
			request:  "Create a scheduled job every day to remind me about medication notes",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:    "active vulnerability finding does not become privacy pack",
			request: "Find vulnerabilities on example.com and keep going until you find one",
		},
		{
			name:    "security header check does not become privacy pack",
			request: "Check security headers on my website",
		},
		{
			name:    "secret scan does not become privacy pack",
			request: "Run a secret scan on this repo",
		},
		{
			name:     "privacy law web search remains internet setup",
			request:  "Search the web for the latest privacy law updates",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "password manager connector request does not become privacy pack",
			request: "Connect to my password manager and export secrets",
		},
		{
			name:     "password reminder remains scheduler setup",
			request:  "Create a scheduled job to remind me to review password notes every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:    "account settings change does not become privacy pack",
			request: "Change my account privacy settings",
		},
		{
			name:     "legal web search remains internet setup",
			request:  "Search the web for the latest contract law updates",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "signing contract does not become contract pack",
			request: "Sign this contract for me",
		},
		{
			name:    "court filing does not become contract pack",
			request: "File this court form online",
		},
		{
			name:    "send contract request does not become contract pack",
			request: "Send this contract to my lawyer",
		},
		{
			name:     "contract reminder remains scheduler setup",
			request:  "Create a scheduled job to remind me to review contract notes every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:    "legal advice request does not become contract pack",
			request: "Tell me whether I should sue or terminate this contract",
		},
		{
			name:     "finance web search remains internet setup",
			request:  "Search the web for the latest stock price updates",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "live trading does not become finance pack",
			request: "Place trades for me based on these notes",
		},
		{
			name:    "tax advice does not become finance pack",
			request: "Tell me whether I should claim this tax deduction",
		},
		{
			name:    "bank connector request does not become finance pack",
			request: "Connect to my bank and export transactions",
		},
		{
			name:    "payment request does not become finance pack",
			request: "Pay this invoice for me",
		},
		{
			name:    "send invoice request does not become finance pack",
			request: "Send this invoice to the client",
		},
		{
			name:     "finance reminder remains scheduler setup",
			request:  "Create a scheduled job to remind me to review budget notes every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "invoice importer remains extension setup",
			request:  "Build a tool that imports CSV invoices",
			wantKind: CapabilityKindToolExtension,
		},
		{
			name:    "form filling does not become form prep pack",
			request: "Fill out this form for me",
		},
		{
			name:    "form signing does not become form prep pack",
			request: "Sign this form for me",
		},
		{
			name:    "form submission does not become form prep pack",
			request: "Submit this form online",
		},
		{
			name:    "form upload does not become form prep pack",
			request: "Upload this application form to the portal",
		},
		{
			name:    "form email does not become form prep pack",
			request: "Email this form to the office",
		},
		{
			name:     "form requirements search remains internet setup",
			request:  "Search the web for the latest visa application form requirements",
			wantKind: CapabilityKindSettings,
		},
		{
			name:     "form reminder remains scheduler setup",
			request:  "Create a scheduled job to remind me to submit the form every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "form filler tool remains extension setup",
			request:  "Build a tool that fills PDF forms from CSV rows",
			wantKind: CapabilityKindToolExtension,
		},
		{
			name:    "knowledge graph build does not become knowledge base pack",
			request: "Build a SQLite knowledge graph from my documents",
		},
		{
			name:    "knowledge base file export does not become knowledge base pack",
			request: "Export this knowledge base to a PDF file",
		},
		{
			name:    "knowledge base sync does not become knowledge base pack",
			request: "Sync this knowledge base to Notion",
		},
		{
			name:     "knowledge base refresh remains scheduler setup",
			request:  "Create a scheduled job to refresh my knowledge base every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "knowledge base web search remains internet setup",
			request:  "Search the web for current documentation updates for my knowledge base",
			wantKind: CapabilityKindSettings,
		},
		{
			name:     "knowledge base exporter remains extension setup",
			request:  "Build a tool that exports my knowledge base to PDF",
			wantKind: CapabilityKindToolExtension,
		},
		{
			name:     "travel web search remains internet setup",
			request:  "Search the web for the latest flight prices",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "flight booking does not become travel pack",
			request: "Book a flight for this trip",
		},
		{
			name:    "hotel booking does not become travel pack",
			request: "Book a hotel from these itinerary notes",
		},
		{
			name:    "travel payment does not become travel pack",
			request: "Pay for this hotel reservation",
		},
		{
			name:    "send itinerary does not become travel pack",
			request: "Send this itinerary to my family",
		},
		{
			name:     "travel reminder remains scheduler setup",
			request:  "Create a scheduled job to remind me to review packing notes every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:    "visa application does not become travel pack",
			request: "Apply for a visa using these travel notes",
		},
		{
			name:     "weather lookup remains internet setup",
			request:  "Search the web for weather updates for my trip",
			wantKind: CapabilityKindSettings,
		},
		{
			name:     "content trend search remains internet setup",
			request:  "Search the web for the latest content trends",
			wantKind: CapabilityKindSettings,
		},
		{
			name:    "social posting does not become content calendar pack",
			request: "Post this campaign update to X",
		},
		{
			name:    "publishing does not become content calendar pack",
			request: "Publish this newsletter to subscribers",
		},
		{
			name:    "newsletter sending does not become content calendar pack",
			request: "Send a newsletter from these campaign notes",
		},
		{
			name:     "content scheduling remains scheduler setup",
			request:  "Create a scheduled job to publish posts every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "campaign reminder remains scheduler setup",
			request:  "Create a scheduled job to remind me to review campaign notes every week",
			wantKind: CapabilityKindWorkflow,
		},
		{
			name:     "media generation remains extension setup",
			request:  "Build a tool that generates social media images from campaign notes",
			wantKind: CapabilityKindToolExtension,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			proposal, err := router.ProposeRequest(tc.request)
			if err != nil {
				t.Fatalf("ProposeRequest() error = %v", err)
			}
			if proposal.Kind == string(capabilities.KindDomainPack) || proposal.PackSource == "domain_pack_template" {
				t.Fatalf("proposal = %+v, built-in template stole %q", proposal, tc.request)
			}
			if tc.wantKind != "" && proposal.Kind != tc.wantKind {
				t.Fatalf("proposal.Kind = %q, want %q; proposal=%+v", proposal.Kind, tc.wantKind, proposal)
			}
			if tc.wantNeeded != nil && proposal.Needed != *tc.wantNeeded {
				t.Fatalf("proposal.Needed = %v, want %v; proposal=%+v", proposal.Needed, *tc.wantNeeded, proposal)
			}
		})
	}
}

func TestCapabilityGapRouterDomainPackProtectedPolicyStaysCoreEnforced(t *testing.T) {
	cfg, root := domainPackRouterTestConfig(t)
	writeInstalledRouterDomainPack(t, root, "trading_pack", `
name: trading_pack
version: 0.1.0
description: Place live trades from chat requests.
category: trading
safety:
  policy_overrides:
    - trading_policy
default_state: disabled
`, true)

	router := NewCapabilityGapRouter(cfg, &recordingCapabilityGenerator{})
	pack, ok := router.CapabilityRegistry.Get(capabilities.KindDomainPack, "trading_pack")
	if !ok {
		t.Fatal("trading_pack was not loaded into the capability registry")
	}
	if pack.State != capabilities.StateUnavailable || pack.Enabled || pack.Available {
		t.Fatalf("pack = %+v, want unavailable protected-policy pack", pack)
	}
	if !strings.Contains(pack.Reason, "protected policy") {
		t.Fatalf("pack.Reason = %q, want protected policy enforcement", pack.Reason)
	}

	decision := router.CapabilityRegistry.ClassifyGap(capabilities.GapQuery{
		Request: "Use the trading pack to place trades",
		Kind:    capabilities.KindDomainPack,
	})
	if decision.Action != capabilities.ActionUnsupported {
		t.Fatalf("decision = %+v, want unsupported protected-policy pack", decision)
	}
	if decision.Action == capabilities.ActionUsePack || decision.State == capabilities.StateReady {
		t.Fatalf("decision = %+v, protected-policy pack was treated as usable", decision)
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func domainPackRouterTestConfig(t *testing.T) (*config.Config, string) {
	t.Helper()
	supportDir := t.TempDir()
	cfg := config.Default()
	cfg.Path = filepath.Join(supportDir, config.ConfigName)
	cfg.App.Profile = "default"
	cfg.Extensions.Enabled = true
	return cfg, filepath.Join(supportDir, "profiles", cfg.App.Profile, "domain_packs")
}

func installRouterDomainPack(t *testing.T, root string, enabled bool, manifest string) {
	t.Helper()
	source := t.TempDir()
	writeRouterDomainPackManifest(t, source, manifest)
	status, err := domainpacks.Install(root, source)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if enabled {
		if _, err := domainpacks.SetEnabled(root, status.Name, true); err != nil {
			t.Fatalf("SetEnabled(%s) error = %v", status.Name, err)
		}
	}
}

func writeInstalledRouterDomainPack(t *testing.T, root string, name string, manifest string, enabled bool) {
	t.Helper()
	dir := filepath.Join(root, name)
	writeRouterDomainPackManifest(t, dir, manifest)
	state := "enabled: false\n"
	if enabled {
		state = "enabled: true\n"
	}
	if err := os.WriteFile(filepath.Join(dir, domainpacks.StateFile), []byte(state), 0o644); err != nil {
		t.Fatalf("write domain pack state: %v", err)
	}
}

func writeRouterDomainPackManifest(t *testing.T, dir string, manifest string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir domain pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, domainpacks.ManifestFile), []byte(strings.TrimSpace(manifest)+"\n"), 0o644); err != nil {
		t.Fatalf("write domain pack manifest: %v", err)
	}
}
