package capabilities

import (
	"encoding/json"
	"testing"
)

func TestRegistrySnapshotSortsAndRoundTrips(t *testing.T) {
	var registry Registry
	if err := registry.AddSkill(Capability{
		Name:     "zeta_skill",
		State:    StateReady,
		Aliases:  []string{"zeta", "Zeta"},
		Provides: []string{"review"},
	}); err != nil {
		t.Fatalf("AddSkill() error = %v", err)
	}
	if err := registry.AddTool(Ready("read_file", KindTool)); err != nil {
		t.Fatalf("AddTool() error = %v", err)
	}
	if err := registry.AddTool(Ready("git_diff", KindTool)); err != nil {
		t.Fatalf("AddTool() error = %v", err)
	}
	if err := registry.AddModelRole(ModelRole{
		Role:       "coding",
		Model:      "qwen2.5-coder:3b",
		Provider:   "ollama",
		State:      StateReady,
		Aliases:    []string{"coder", "Coder"},
		Configured: true,
		Available:  true,
		Enabled:    true,
	}); err != nil {
		t.Fatalf("AddModelRole() error = %v", err)
	}

	snapshot := registry.Snapshot()
	if len(snapshot.Tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(snapshot.Tools))
	}
	if snapshot.Tools[0].Name != "git_diff" || snapshot.Tools[1].Name != "read_file" {
		t.Fatalf("tools order = %v, want deterministic name sort", []string{snapshot.Tools[0].Name, snapshot.Tools[1].Name})
	}
	if got := snapshot.Skills[0].Aliases; len(got) != 1 || got[0] != "zeta" {
		t.Fatalf("aliases = %#v, want normalized unique aliases", got)
	}

	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded Registry
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if _, ok := decoded.Get(KindTool, "read file"); !ok {
		t.Fatal("decoded registry did not find read_file by normalized key")
	}
}

func TestClassifyUsesExistingCapability(t *testing.T) {
	var registry Registry
	if err := registry.AddTool(Capability{
		Name:        "rag_search",
		Kind:        KindTool,
		State:       StateReady,
		Description: "SQLite FTS-backed document search.",
		Aliases:     []string{"docs search"},
		Provides:    []string{"search_docs", "rag"},
	}); err != nil {
		t.Fatalf("AddTool() error = %v", err)
	}

	decision := registry.ClassifyGap(GapQuery{Request: "Search docs for extension rollback"})
	if decision.Action != ActionUseExisting {
		t.Fatalf("action = %q, want %q; decision=%+v", decision.Action, ActionUseExisting, decision)
	}
	if decision.Name != "rag_search" || decision.Kind != KindTool || decision.State != StateReady {
		t.Fatalf("decision = %+v, want ready rag_search tool", decision)
	}
}

func TestClassifyUsesEnabledPackBeforeGeneration(t *testing.T) {
	var registry Registry
	if err := registry.AddCapabilityPack(Capability{
		Name:        "website_monitor",
		State:       StateReady,
		Description: "Reusable website monitoring workflow pack.",
		Aliases:     []string{"website monitor"},
		Tags:        []string{"web", "monitor"},
		Provides:    []string{"monitor website", "check webpage every hour"},
	}); err != nil {
		t.Fatalf("AddCapabilityPack() error = %v", err)
	}
	if err := registry.SetExtensionGeneration(Capability{
		Name:             "extension_generation",
		State:            StateReady,
		RequiresApproval: true,
	}); err != nil {
		t.Fatalf("SetExtensionGeneration() error = %v", err)
	}

	decision := registry.ClassifyGap(GapQuery{Request: "Create a monitor that checks a webpage every hour"})
	if decision.Action != ActionUsePack {
		t.Fatalf("action = %q, want %q; decision=%+v", decision.Action, ActionUsePack, decision)
	}
	if decision.Name != "website_monitor" {
		t.Fatalf("Name = %q, want website_monitor", decision.Name)
	}
}

func TestClassifyDisabledInternetProviderAsksConfig(t *testing.T) {
	var registry Registry
	if err := registry.AddInternetProvider(Capability{
		Name:          "searxng",
		State:         StateDisabled,
		Description:   "SearXNG-compatible search provider.",
		Aliases:       []string{"web search", "internet search"},
		Provides:      []string{"search_web"},
		Reason:        "internet search provider is disabled until configured",
		ConfigureHint: "Set internet.search.provider and enable internet for this profile.",
	}); err != nil {
		t.Fatalf("AddInternetProvider() error = %v", err)
	}
	if err := registry.SetExtensionGeneration(Capability{
		Name:             "extension_generation",
		State:            StateReady,
		RequiresApproval: true,
	}); err != nil {
		t.Fatalf("SetExtensionGeneration() error = %v", err)
	}

	decision := registry.ClassifyGap(GapQuery{Request: "Search the web for SearXNG docs"})
	if decision.Action != ActionAskConfig {
		t.Fatalf("action = %q, want %q; decision=%+v", decision.Action, ActionAskConfig, decision)
	}
	if decision.Name != "searxng" || decision.State != StateDisabled {
		t.Fatalf("decision = %+v, want disabled searxng config request", decision)
	}
}

func TestClassifyDisabledCapabilityDoesNotUseExisting(t *testing.T) {
	var registry Registry
	if err := registry.AddDisabledCapability(Capability{
		Name:          "slack",
		Kind:          KindConnector,
		State:         StateDisabled,
		Aliases:       []string{"slack connector"},
		Provides:      []string{"post to slack"},
		Reason:        "connectors are disabled by default",
		ConfigureHint: "Enable only the Slack connector and use a secret reference.",
	}); err != nil {
		t.Fatalf("AddDisabledCapability() error = %v", err)
	}

	decision := registry.ClassifyGap(GapQuery{Request: "Post the build result to Slack"})
	if decision.Action != ActionAskConfig {
		t.Fatalf("action = %q, want %q; decision=%+v", decision.Action, ActionAskConfig, decision)
	}
	if !decision.RequiresApproval && decision.SuggestedAction == "" {
		t.Fatalf("decision should carry setup guidance: %+v", decision)
	}
}

func TestClassifySchedulerCrawlerAndModelRoles(t *testing.T) {
	var registry Registry
	if err := registry.SetScheduler(Capability{
		State:         StateDisabled,
		Aliases:       []string{"cron", "scheduled jobs"},
		Provides:      []string{"schedule job", "every hour"},
		Reason:        "scheduler loop is disabled",
		ConfigureHint: "Enable scheduler support without auto-enabling new jobs.",
	}); err != nil {
		t.Fatalf("SetScheduler() error = %v", err)
	}
	if err := registry.SetCrawler(Capability{
		State:    StateNeedsConfig,
		Aliases:  []string{"site crawler"},
		Provides: []string{"crawl website", "ingest docs site"},
		Reason:   "crawler backend is not configured",
	}); err != nil {
		t.Fatalf("SetCrawler() error = %v", err)
	}
	if err := registry.AddModelRole(ModelRole{
		Role:       "coding",
		Model:      "qwen2.5-coder:3b",
		Provider:   "ollama",
		State:      StateReady,
		Aliases:    []string{"coding model"},
		Available:  true,
		Enabled:    true,
		Configured: true,
	}); err != nil {
		t.Fatalf("AddModelRole() error = %v", err)
	}

	decision := registry.ClassifyGap(GapQuery{Request: "Schedule this check every hour"})
	if decision.Action != ActionAskConfig || decision.Name != "scheduler" {
		t.Fatalf("scheduler decision = %+v, want ask_config for scheduler", decision)
	}
	decision = registry.ClassifyGap(GapQuery{Request: "Crawl this website and ingest the docs"})
	if decision.Action != ActionAskConfig || decision.Name != "crawler" {
		t.Fatalf("crawler decision = %+v, want ask_config for crawler", decision)
	}
	decision = registry.ClassifyGap(GapQuery{Request: "Use the coding model for this patch"})
	if decision.Action != ActionUseExisting || decision.Name != "coding" || decision.Kind != KindModelRole {
		t.Fatalf("model role decision = %+v, want use_existing coding model role", decision)
	}
}

func TestClassifyGeneratesExtensionOnlyWhenGenerationIsReady(t *testing.T) {
	var registry Registry
	if err := registry.SetExtensionGeneration(Capability{
		Name:             "extension_generation",
		State:            StateReady,
		RequiresApproval: true,
	}); err != nil {
		t.Fatalf("SetExtensionGeneration() error = %v", err)
	}

	decision := registry.ClassifyGap(GapQuery{Request: "Build a tool that normalizes CSV rows"})
	if decision.Action != ActionGenerateExtension {
		t.Fatalf("action = %q, want %q; decision=%+v", decision.Action, ActionGenerateExtension, decision)
	}
	if !decision.RequiresApproval {
		t.Fatal("generated extension decision must require approval")
	}

	var disabled Registry
	if err := disabled.SetExtensionGeneration(Capability{
		Name:          "extension_generation",
		State:         StateDisabled,
		Reason:        "generated extensions are disabled",
		ConfigureHint: "Enable extension generation before proposing a package.",
	}); err != nil {
		t.Fatalf("SetExtensionGeneration(disabled) error = %v", err)
	}
	decision = disabled.ClassifyGap(GapQuery{Request: "Build a tool that normalizes CSV rows"})
	if decision.Action != ActionAskConfig {
		t.Fatalf("action = %q, want %q while generation disabled; decision=%+v", decision.Action, ActionAskConfig, decision)
	}
}

func TestClassifyKnownLimitationUnsupported(t *testing.T) {
	var registry Registry
	if err := registry.AddKnownLimitation(Limitation{
		Name:            "live_trading",
		Description:     "Autonomous live trading is not supported.",
		Aliases:         []string{"live trading", "place trades"},
		Tags:            []string{"trading"},
		AppliesTo:       []string{"connector"},
		Reason:          "Yemaka policy blocks live trading from generated capabilities.",
		SuggestedAction: "Keep analysis local and do not execute trades.",
	}); err != nil {
		t.Fatalf("AddKnownLimitation() error = %v", err)
	}

	decision := registry.ClassifyGap(GapQuery{Request: "Create a connector that can place trades for me"})
	if decision.Action != ActionUnsupported {
		t.Fatalf("action = %q, want %q; decision=%+v", decision.Action, ActionUnsupported, decision)
	}
	if decision.Name != "live_trading" {
		t.Fatalf("Name = %q, want live_trading", decision.Name)
	}
}

func TestClassifyVagueRequestAsksClarification(t *testing.T) {
	var registry Registry
	decision := registry.ClassifyGap(GapQuery{Request: "Can you help with this?"})
	if decision.Action != ActionAskClarification {
		t.Fatalf("action = %q, want %q; decision=%+v", decision.Action, ActionAskClarification, decision)
	}
}
