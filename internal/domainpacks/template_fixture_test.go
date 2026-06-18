package domainpacks

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"yemaka/internal/skills"
)

func TestPersonalProductivityTemplateInstallableDisabledAndLoadable(t *testing.T) {
	source := filepath.Join("..", "..", "packs", "templates", "personal_productivity")
	manifest, err := Load(source)
	if err != nil {
		t.Fatalf("Load(personal productivity template) error = %v", err)
	}
	if manifest.Name != "personal_productivity" {
		t.Fatalf("template name = %q, want personal_productivity", manifest.Name)
	}
	if manifest.DefaultState != DefaultStateDisabled || manifest.EnabledByDefault() {
		t.Fatalf("template default state = %q enabled=%v, want disabled", manifest.DefaultState, manifest.EnabledByDefault())
	}
	if manifest.Permissions.Internet.Required || manifest.Permissions.Connectors.Required || manifest.Permissions.Scheduler.Required {
		t.Fatalf("template declares unexpected online/background permission: %#v", manifest.Permissions)
	}

	profilePackRoot := t.TempDir()
	installed, err := Install(profilePackRoot, source)
	if err != nil {
		t.Fatalf("Install(personal productivity template) error = %v", err)
	}
	if installed.Enabled {
		t.Fatal("installed personal productivity template enabled = true, want disabled")
	}

	if _, err := SetEnabled(profilePackRoot, "personal_productivity", true); err != nil {
		t.Fatalf("SetEnabled(personal_productivity) error = %v", err)
	}
	dirs, err := EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs(personal_productivity) error = %v", err)
	}
	registry, err := skills.LoadRegistry(dirs)
	if err != nil {
		t.Fatalf("LoadRegistry(personal_productivity skills) error = %v", err)
	}
	for _, name := range []string{"daily_planner", "followup_capture"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("enabled personal productivity pack missing skill %s; statuses = %#v", name, registry.Statuses())
		}
	}
}

func TestResearchAssistantTemplateInstallableDisabledAndLoadable(t *testing.T) {
	source := filepath.Join("..", "..", "packs", "templates", "research_assistant")
	manifest, err := Load(source)
	if err != nil {
		t.Fatalf("Load(research assistant template) error = %v", err)
	}
	if manifest.Name != "research_assistant" {
		t.Fatalf("template name = %q, want research_assistant", manifest.Name)
	}
	if manifest.DefaultState != DefaultStateDisabled || manifest.EnabledByDefault() {
		t.Fatalf("template default state = %q enabled=%v, want disabled", manifest.DefaultState, manifest.EnabledByDefault())
	}
	if manifest.Permissions.Internet.Required || manifest.Permissions.Connectors.Required || manifest.Permissions.Scheduler.Required {
		t.Fatalf("template declares unexpected online/background permission: %#v", manifest.Permissions)
	}

	profilePackRoot := t.TempDir()
	installed, err := Install(profilePackRoot, source)
	if err != nil {
		t.Fatalf("Install(research assistant template) error = %v", err)
	}
	if installed.Enabled {
		t.Fatal("installed research assistant template enabled = true, want disabled")
	}

	if _, err := SetEnabled(profilePackRoot, "research_assistant", true); err != nil {
		t.Fatalf("SetEnabled(research_assistant) error = %v", err)
	}
	dirs, err := EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs(research_assistant) error = %v", err)
	}
	registry, err := skills.LoadRegistry(dirs)
	if err != nil {
		t.Fatalf("LoadRegistry(research_assistant skills) error = %v", err)
	}
	for _, name := range []string{"source_brief", "claim_check"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("enabled research assistant pack missing skill %s; statuses = %#v", name, registry.Statuses())
		}
	}
}

func TestLocalFileTriageTemplateInstallableDisabledAndLoadable(t *testing.T) {
	source := filepath.Join("..", "..", "packs", "templates", "local_file_triage")
	manifest, err := Load(source)
	if err != nil {
		t.Fatalf("Load(local file triage template) error = %v", err)
	}
	if manifest.Name != "local_file_triage" {
		t.Fatalf("template name = %q, want local_file_triage", manifest.Name)
	}
	if manifest.DefaultState != DefaultStateDisabled || manifest.EnabledByDefault() {
		t.Fatalf("template default state = %q enabled=%v, want disabled", manifest.DefaultState, manifest.EnabledByDefault())
	}
	if manifest.Permissions.Internet.Required || manifest.Permissions.Connectors.Required || manifest.Permissions.Scheduler.Required {
		t.Fatalf("template declares unexpected online/background permission: %#v", manifest.Permissions)
	}
	for _, blocked := range []string{"edit_file", "write_file", "internet_search", "connector_action", "scheduler_create", "run_shell_safe"} {
		if !containsPackString(manifest.Safety.BlockedActions, blocked) {
			t.Fatalf("blocked actions = %#v, missing %q", manifest.Safety.BlockedActions, blocked)
		}
	}

	profilePackRoot := t.TempDir()
	installed, err := Install(profilePackRoot, source)
	if err != nil {
		t.Fatalf("Install(local file triage template) error = %v", err)
	}
	if installed.Enabled {
		t.Fatal("installed local file triage template enabled = true, want disabled")
	}

	if _, err := SetEnabled(profilePackRoot, "local_file_triage", true); err != nil {
		t.Fatalf("SetEnabled(local_file_triage) error = %v", err)
	}
	dirs, err := EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs(local_file_triage) error = %v", err)
	}
	registry, err := skills.LoadRegistry(dirs)
	if err != nil {
		t.Fatalf("LoadRegistry(local_file_triage skills) error = %v", err)
	}
	for _, name := range []string{"file_inventory", "cleanup_plan"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("enabled local file triage pack missing skill %s; statuses = %#v", name, registry.Statuses())
		}
	}
}

func TestBuiltInTemplatesInstallDisabledEnableDisableMatrix(t *testing.T) {
	sources, err := builtInTemplateSources()
	if err != nil {
		t.Fatalf("list built-in templates: %v", err)
	}
	if len(sources) == 0 {
		t.Fatal("no built-in templates found")
	}

	for _, source := range sources {
		manifest, err := Load(source)
		if err != nil {
			t.Fatalf("Load(%s) error = %v", source, err)
		}
		t.Run(manifest.Name, func(t *testing.T) {
			if manifest.DefaultState != DefaultStateDisabled || manifest.EnabledByDefault() {
				t.Fatalf("template default state = %q enabled=%v, want disabled", manifest.DefaultState, manifest.EnabledByDefault())
			}
			if manifest.Permissions.Internet.Required ||
				manifest.Permissions.Connectors.Required ||
				manifest.Permissions.Scheduler.Required ||
				manifest.Permissions.Notifications.Required ||
				manifest.Permissions.Secrets.Required {
				t.Fatalf("template declares unexpected online/background/secret permission: %#v", manifest.Permissions)
			}
			if len(manifest.Skills) == 0 || len(manifest.Tests) == 0 {
				t.Fatalf("template skills=%#v tests=%#v, want both declared", manifest.Skills, manifest.Tests)
			}

			profilePackRoot := t.TempDir()
			installed, err := Install(profilePackRoot, source)
			if err != nil {
				t.Fatalf("Install(%s) error = %v", manifest.Name, err)
			}
			if !installed.Installed || !installed.Valid || installed.Enabled {
				t.Fatalf("installed status = %#v, want valid installed disabled pack", installed)
			}
			dirs, err := EnabledSkillDirs(profilePackRoot)
			if err != nil {
				t.Fatalf("EnabledSkillDirs(disabled %s) error = %v", manifest.Name, err)
			}
			if len(dirs) != 0 {
				t.Fatalf("disabled %s skill dirs = %#v, want none", manifest.Name, dirs)
			}

			enabled, err := SetEnabled(profilePackRoot, manifest.Name, true)
			if err != nil {
				t.Fatalf("SetEnabled(%s, true) error = %v", manifest.Name, err)
			}
			if !enabled.Enabled {
				t.Fatalf("enabled status = %#v, want enabled", enabled)
			}
			dirs, err = EnabledSkillDirs(profilePackRoot)
			if err != nil {
				t.Fatalf("EnabledSkillDirs(enabled %s) error = %v", manifest.Name, err)
			}
			registry, err := skills.LoadRegistry(dirs)
			if err != nil {
				t.Fatalf("LoadRegistry(%s skills) error = %v", manifest.Name, err)
			}
			for _, ref := range manifest.Skills {
				name := filepath.Base(ref)
				if _, ok := registry.Get(name); !ok {
					t.Fatalf("enabled %s missing skill %s; statuses = %#v", manifest.Name, name, registry.Statuses())
				}
			}

			disabled, err := SetEnabled(profilePackRoot, manifest.Name, false)
			if err != nil {
				t.Fatalf("SetEnabled(%s, false) error = %v", manifest.Name, err)
			}
			if disabled.Enabled {
				t.Fatalf("disabled status = %#v, want disabled", disabled)
			}
			dirs, err = EnabledSkillDirs(profilePackRoot)
			if err != nil {
				t.Fatalf("EnabledSkillDirs(re-disabled %s) error = %v", manifest.Name, err)
			}
			if len(dirs) != 0 {
				t.Fatalf("re-disabled %s skill dirs = %#v, want none", manifest.Name, dirs)
			}
		})
	}
}

func builtInTemplateSources() ([]string, error) {
	sources, err := filepath.Glob(filepath.Join("..", "..", "packs", "templates", "*"))
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(sources))
	for _, source := range sources {
		info, err := os.Stat(source)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(source, "pack.yaml")); err != nil {
			continue
		}
		filtered = append(filtered, source)
	}
	sort.Strings(filtered)
	return filtered, nil
}
