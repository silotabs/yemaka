package workflows

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestBuiltInTemplateCatalogLookupByName(t *testing.T) {
	catalog := loadBuiltInTemplateCatalogForTest(t)

	template, ok := catalog.Get("Personal Productivity")
	if !ok {
		t.Fatal("Get(Personal Productivity) ok = false, want true")
	}
	if template.Name != "personal_productivity" {
		t.Fatalf("template.Name = %q, want personal_productivity", template.Name)
	}
	if template.Source != SourceDomainPackTemplate {
		t.Fatalf("template.Source = %q, want %q", template.Source, SourceDomainPackTemplate)
	}
	if template.Metadata["template_ref"] != "packs/templates/personal_productivity" {
		t.Fatalf("template_ref = %q, want packs/templates/personal_productivity", template.Metadata["template_ref"])
	}
	if len(template.Workflows) != 2 {
		t.Fatalf("len(template.Workflows) = %d, want 2", len(template.Workflows))
	}
	if _, ok := catalog.Get("missing_pack"); ok {
		t.Fatal("Get(missing_pack) ok = true, want false")
	}
}

func TestBuiltInTemplateCatalogListDeterministic(t *testing.T) {
	catalog := loadBuiltInTemplateCatalogForTest(t)

	first := templateNames(catalog.List())
	second := templateNames(catalog.List())
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("catalog.List() names changed between calls: %v then %v", first, second)
	}
	if !sort.StringsAreSorted(first) {
		t.Fatalf("catalog.List() names = %v, want sorted", first)
	}
	if !hasTemplateName(first, "personal_productivity") {
		t.Fatalf("catalog.List() names = %v, want personal_productivity", first)
	}
	if !hasTemplateName(first, "local_file_triage") {
		t.Fatalf("catalog.List() names = %v, want local_file_triage", first)
	}
}

func TestBuiltInTemplateCatalogDisabledByDefault(t *testing.T) {
	catalog := loadBuiltInTemplateCatalogForTest(t)

	for _, template := range catalog.List() {
		if template.Installed || template.Enabled || template.EnabledByDefault() {
			t.Fatalf("%s installed=%v enabled=%v enabledByDefault=%v, want catalog-only disabled template",
				template.Name, template.Installed, template.Enabled, template.EnabledByDefault())
		}
		if template.DefaultState != DefaultStateDisabled {
			t.Fatalf("%s DefaultState = %q, want disabled", template.Name, template.DefaultState)
		}
		for _, workflow := range template.Workflows {
			if workflow.Enabled || workflow.EnabledByDefault() {
				t.Fatalf("%s workflow %s enabled=%v enabledByDefault=%v, want disabled",
					template.Name, workflow.Name, workflow.Enabled, workflow.EnabledByDefault())
			}
			if workflow.RunMode != RunModeManual {
				t.Fatalf("%s workflow %s RunMode = %q, want manual", template.Name, workflow.Name, workflow.RunMode)
			}
		}
	}
}

func TestBuiltInTemplateCatalogDoesNotEnableExternalCapabilities(t *testing.T) {
	catalog := loadBuiltInTemplateCatalogForTest(t)

	for _, template := range catalog.List() {
		if template.RequiresExternalCapabilities() || !template.LocalOnly() {
			t.Fatalf("%s external capabilities enabled unexpectedly: %#v", template.Name, template.Permissions)
		}
		if template.Permissions.Internet.Required ||
			template.Permissions.Cloud.Required ||
			template.Permissions.Connectors.Required ||
			template.Permissions.Scheduler.Required ||
			template.Permissions.Notifications.Required ||
			template.Permissions.Embeddings.Required {
			t.Fatalf("%s template permissions enabled an external capability: %#v", template.Name, template.Permissions)
		}
		for _, workflow := range template.Workflows {
			if workflow.Permissions.Internet.Required ||
				workflow.Permissions.Cloud.Required ||
				workflow.Permissions.Connectors.Required ||
				workflow.Permissions.Scheduler.Required ||
				workflow.Permissions.Notifications.Required ||
				workflow.Permissions.Embeddings.Required {
				t.Fatalf("%s workflow %s permissions enabled an external capability: %#v",
					template.Name, workflow.Name, workflow.Permissions)
			}
		}
	}
}

func TestDomainPackTemplateSummariesExposeLifecycleMetadata(t *testing.T) {
	catalog := loadBuiltInTemplateCatalogForTest(t)

	summaries := DomainPackTemplateSummaries(catalog, nil)
	if len(summaries) != len(catalog.List()) {
		t.Fatalf("len(summaries) = %d, want %d", len(summaries), len(catalog.List()))
	}
	for _, summary := range summaries {
		if summary.Name == "" || summary.Version == "" || summary.Description == "" || summary.Category == "" {
			t.Fatalf("summary missing identity metadata: %#v", summary)
		}
		if summary.Source != SourceDomainPackTemplate || summary.Path == "" || summary.InstallAction != "install" {
			t.Fatalf("summary source/path/action = %#v, want built-in installable template", summary)
		}
		if summary.Installed || summary.Enabled || !summary.Valid || summary.DefaultState != DefaultStateDisabled {
			t.Fatalf("summary lifecycle = %#v, want uninstalled valid disabled template", summary)
		}
		if len(summary.RequiredTools) == 0 || len(summary.Skills) == 0 || len(summary.Workflows) == 0 || summary.SafetySummary == "" {
			t.Fatalf("summary missing tools/skills/workflows/safety metadata: %#v", summary)
		}
	}
}

func loadBuiltInTemplateCatalogForTest(t *testing.T) TemplateCatalog {
	t.Helper()
	catalog, err := NewBuiltInTemplateCatalog(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("NewBuiltInTemplateCatalog() error = %v", err)
	}
	return catalog
}

func templateNames(templates []Template) []string {
	names := make([]string, 0, len(templates))
	for _, template := range templates {
		names = append(names, template.Name)
	}
	return names
}

func hasTemplateName(names []string, want string) bool {
	for _, name := range names {
		if name == want {
			return true
		}
	}
	return false
}
