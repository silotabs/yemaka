package capabilities

import (
	"strings"
	"testing"
)

func TestDefaultLocalFirstSnapshotKeepsOptionalSystemsDisabled(t *testing.T) {
	registry, err := DefaultLocalFirstSnapshot()
	if err != nil {
		t.Fatalf("DefaultLocalFirstSnapshot() error = %v", err)
	}

	read, ok := registry.Get(KindTool, "filesystem_read")
	if !ok || read.State != StateReady || !read.Enabled {
		t.Fatalf("filesystem_read = %+v, ok=%v, want ready local tool", read, ok)
	}
	if registry.Scheduler.State != StateDisabled || registry.Scheduler.Enabled {
		t.Fatalf("Scheduler = %+v, want disabled by default", registry.Scheduler)
	}
	if registry.Crawler.State != StateDisabled || registry.Crawler.Enabled {
		t.Fatalf("Crawler = %+v, want disabled by default", registry.Crawler)
	}
	notifications, ok := registry.Get(KindNotification, "notification_center")
	if !ok || notifications.State != StateDisabled || notifications.Enabled {
		t.Fatalf("notification_center = %+v, ok=%v, want disabled local inbox by default", notifications, ok)
	}
	if registry.ExtensionGeneration.State != StateDisabled || !registry.ExtensionGeneration.RequiresApproval {
		t.Fatalf("ExtensionGeneration = %+v, want disabled and approval-gated", registry.ExtensionGeneration)
	}
	internet, ok := registry.Get(KindInternetProvider, "internet")
	if !ok || internet.State != StateDisabled || internet.Enabled || !strings.Contains(internet.ConfigureHint, "SearXNG") {
		t.Fatalf("internet = %+v, ok=%v, want disabled provider with setup hint", internet, ok)
	}

	decision := registry.ClassifyRouteGap(RouteMetadata{
		Request:       "Search the web for SearXNG docs",
		RouteCategory: "internet_search",
		UseInternet:   true,
	})
	if decision.Action != ActionAskConfig || decision.Name != "internet" || decision.Kind != KindInternetProvider {
		t.Fatalf("internet route decision = %+v, want ask_config for disabled internet provider", decision)
	}
	summary := decision.Summary()
	if summary.Title != "Capability needs setup" || !strings.Contains(summary.Detail, "disabled") || summary.NextStep == "" {
		t.Fatalf("summary = %+v, want user-friendly disabled setup summary", summary)
	}
}

func TestDeclaredPrimitiveStatusOverridesDefaultAndPreservesRouteAliases(t *testing.T) {
	registry, err := DefaultLocalFirstSnapshot(Ready("internet", KindInternetProvider))
	if err != nil {
		t.Fatalf("DefaultLocalFirstSnapshot(ready internet) error = %v", err)
	}
	internet, ok := registry.Get(KindInternetProvider, "internet")
	if !ok || internet.State != StateReady || !internet.Enabled || internet.Reason != "" {
		t.Fatalf("internet = %+v, ok=%v, want ready override without disabled reason", internet, ok)
	}
	if !contains(internet.Provides, "internet_search") {
		t.Fatalf("internet.Provides = %#v, want default internet_search route alias preserved", internet.Provides)
	}

	decision := registry.ClassifyRouteGap(RouteMetadata{
		Request:     "Find current release notes",
		Tools:       []string{"internet_search"},
		UseInternet: true,
	})
	if decision.Action != ActionUseExisting || decision.Name != "internet" || decision.State != StateReady {
		t.Fatalf("decision = %+v, want ready internet provider from route tools", decision)
	}
	if got := decision.Summary().Title; got != "Existing capability is ready" {
		t.Fatalf("summary title = %q, want existing capability summary", got)
	}
}

func TestRouteMetadataUsesPackBeforeGeneratingExtension(t *testing.T) {
	registry, err := DefaultLocalFirstSnapshot(Ready("extension_generation", KindExtension))
	if err != nil {
		t.Fatalf("DefaultLocalFirstSnapshot(ready generation) error = %v", err)
	}
	if err := registry.AddCapabilityPack(Capability{
		Name:     "website_monitor",
		State:    StateReady,
		Aliases:  []string{"website monitor"},
		Tags:     []string{"web"},
		Provides: []string{"check title", "title changes", "website monitor"},
	}); err != nil {
		t.Fatalf("AddCapabilityPack() error = %v", err)
	}

	decision := registry.ClassifyRouteGap(RouteMetadata{
		Request:            "I need a reusable website monitor that checks title changes",
		RouteCategory:      "extension_generate",
		Capability:         "extension_generation",
		Target:             "generated_extension",
		GeneratesExtension: true,
	})
	if decision.Action != ActionUsePack || decision.Name != "website_monitor" {
		t.Fatalf("decision = %+v, want enabled pack before generated extension", decision)
	}
}

func TestRouteMetadataGeneratesOnlyWhenGenerationReady(t *testing.T) {
	disabled, err := DefaultLocalFirstSnapshot()
	if err != nil {
		t.Fatalf("DefaultLocalFirstSnapshot() error = %v", err)
	}
	decision := disabled.ClassifyRouteGap(RouteMetadata{
		Request:            "Build a tool that normalizes CSV rows",
		RouteCategory:      "extension_generate",
		Capability:         "extension_generation",
		Target:             "generated_extension",
		GeneratesExtension: true,
	})
	if decision.Action != ActionAskConfig || decision.Name != "extension_generation" {
		t.Fatalf("disabled decision = %+v, want setup before generation", decision)
	}

	ready, err := DefaultLocalFirstSnapshot(Ready("extension_generation", KindExtension))
	if err != nil {
		t.Fatalf("DefaultLocalFirstSnapshot(ready generation) error = %v", err)
	}
	decision = ready.ClassifyRouteGap(RouteMetadata{
		Request:            "Build a tool that normalizes CSV rows",
		RouteCategory:      "extension_generate",
		Capability:         "extension_generation",
		Target:             "generated_extension",
		GeneratesExtension: true,
	})
	if decision.Action != ActionGenerateExtension || !decision.RequiresApproval {
		t.Fatalf("ready decision = %+v, want approval-gated extension generation", decision)
	}
}

func TestGapSummaryForDisabledSchedulerRoute(t *testing.T) {
	registry, err := DefaultLocalFirstSnapshot(Disabled("scheduler", KindScheduler, "scheduler stays off in low-memory mode"))
	if err != nil {
		t.Fatalf("DefaultLocalFirstSnapshot(disabled scheduler) error = %v", err)
	}
	decision := registry.ClassifyRouteGap(RouteMetadata{
		Request:             "Run this check every hour",
		RouteCategory:       "scheduler_create",
		CreatesSchedulerJob: true,
	})
	if decision.Action != ActionAskConfig || decision.Name != "scheduler" {
		t.Fatalf("decision = %+v, want disabled scheduler setup", decision)
	}
	summary := decision.Summary()
	if summary.Title != "Capability needs setup" || !strings.Contains(summary.Detail, "low-memory") || summary.NextStep == "" {
		t.Fatalf("summary = %+v, want scheduler setup guidance", summary)
	}
	if text := decision.UserSummary(); !strings.Contains(text, "Capability needs setup") || !strings.Contains(text, "low-memory") {
		t.Fatalf("UserSummary() = %q, want user-friendly scheduler reason", text)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
