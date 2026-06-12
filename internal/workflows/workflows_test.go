package workflows

import (
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
)

func TestWorkflowDefaultsDisabledAndLocal(t *testing.T) {
	workflow := Normalize(Workflow{
		Name:          "daily_focus",
		Version:       "0.1.0",
		Description:   "Build a local day plan from memory.",
		Triggers:      []string{"plan my day"},
		RequiredTools: []string{"memory_search"},
	})
	if err := Validate(workflow); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if workflow.DefaultState != DefaultStateDisabled || workflow.EnabledByDefault() {
		t.Fatalf("DefaultState = %q enabledByDefault=%v, want disabled", workflow.DefaultState, workflow.EnabledByDefault())
	}
	if workflow.Enabled {
		t.Fatal("Enabled = true, want false until explicitly enabled")
	}
	if workflow.RunMode != RunModeManual {
		t.Fatalf("RunMode = %q, want manual", workflow.RunMode)
	}
	if !workflow.LocalOnly() {
		t.Fatalf("LocalOnly() = false, permissions = %#v", workflow.Permissions)
	}
	if workflow.Permissions.Internet.Required ||
		workflow.Permissions.Cloud.Required ||
		workflow.Permissions.Connectors.Required ||
		workflow.Permissions.Scheduler.Required ||
		workflow.Permissions.Notifications.Required ||
		workflow.Permissions.Embeddings.Required {
		t.Fatalf("workflow enabled an optional external capability by default: %#v", workflow.Permissions)
	}
}

func TestExplainPromptsDoNotTriggerActionWorkflows(t *testing.T) {
	library := newTestLibrary(t, Workflow{
		Name:          "followup_capture",
		Version:       "0.1.0",
		Description:   "Save confirmed follow-up tasks to local memory.",
		Intent:        IntentAction,
		Reusable:      true,
		Enabled:       true,
		Triggers:      []string{"save follow-up tasks"},
		RequiredTools: []string{"memory_search", "memory_write"},
	})

	if match, ok := library.Match("Explain how to save follow-up tasks without doing it.", MatchOptions{}); ok && match.Intent == IntentAction {
		t.Fatalf("explain prompt matched action workflow: %#v", match)
	}
}

func TestActionPromptsCanMatchReusableWorkflows(t *testing.T) {
	library := newTestLibrary(t, Workflow{
		Name:          "followup_capture",
		Version:       "0.1.0",
		Description:   "Save confirmed follow-up tasks to local memory.",
		Intent:        IntentAction,
		Reusable:      true,
		Enabled:       true,
		Triggers:      []string{"save follow-up tasks", "remember action items"},
		RequiredTools: []string{"memory_search", "memory_write"},
		Permissions: Permissions{
			Memory: Capability{Required: true, Scopes: []string{"search", "write"}},
		},
	})

	match, ok := library.Match("Please save follow-up tasks from these notes.", MatchOptions{})
	if !ok {
		t.Fatal("Match() ok = false, want true")
	}
	if match.Name != "followup_capture" || !match.Reusable || match.Intent != IntentAction {
		t.Fatalf("Match() = %#v, want reusable action followup_capture", match)
	}
	if match.EnabledByDefault() {
		t.Fatal("matched action workflow is enabled by default, want runtime-enabled only")
	}
}

func TestUnsafeDefaultsRejected(t *testing.T) {
	cases := []Workflow{
		{
			Name:          "auto_followup_capture",
			Version:       "0.1.0",
			Description:   "Save follow-up tasks without explicit enablement.",
			Intent:        IntentAction,
			DefaultState:  DefaultStateEnabled,
			Triggers:      []string{"save follow-up tasks"},
			RequiredTools: []string{"memory_write"},
		},
		{
			Name:         "scheduled_daily_digest",
			Version:      "0.1.0",
			Description:  "Run a daily digest on a schedule.",
			DefaultState: DefaultStateEnabled,
			RunMode:      RunModeScheduled,
			Triggers:     []string{"daily digest"},
			Permissions: Permissions{
				Scheduler: Capability{Required: true, Scopes: []string{"cron"}},
			},
		},
		{
			Name:         "connector_summary",
			Version:      "0.1.0",
			Description:  "Summarize connector data.",
			DefaultState: DefaultStateEnabled,
			Triggers:     []string{"summarize connector data"},
			Permissions: Permissions{
				Connectors: Capability{Required: true, Names: []string{"calendar"}},
			},
		},
	}
	for _, tc := range cases {
		err := Validate(tc)
		if err == nil || !strings.Contains(err.Error(), "unsafe default") {
			t.Fatalf("Validate(%s) error = %v, want unsafe default rejection", tc.Name, err)
		}
	}
}

func TestPersonalProductivityPackTemplatesRepresentedWithoutEnablingOptionalSystems(t *testing.T) {
	packDir := filepath.Join("..", "..", "packs", "templates", "personal_productivity")
	manifest, err := domainpacks.Load(packDir)
	if err != nil {
		t.Fatalf("Load(personal_productivity) error = %v", err)
	}
	var packSkills []skills.Skill
	for _, ref := range manifest.Skills {
		skill, err := skills.Load(filepath.Join(packDir, filepath.FromSlash(ref)))
		if err != nil {
			t.Fatalf("Load(%s) error = %v", ref, err)
		}
		packSkills = append(packSkills, skill)
	}

	items, err := FromDomainPackSkills(manifest, packSkills)
	if err != nil {
		t.Fatalf("FromDomainPackSkills() error = %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(workflows) = %d, want 2", len(items))
	}
	library, err := NewLibrary(items...)
	if err != nil {
		t.Fatalf("NewLibrary() error = %v", err)
	}

	seen := map[string]Workflow{}
	for _, workflow := range library.List() {
		seen[workflow.SkillName] = workflow
		if workflow.Enabled || workflow.EnabledByDefault() {
			t.Fatalf("%s enabled unexpectedly: enabled=%v default=%q", workflow.Name, workflow.Enabled, workflow.DefaultState)
		}
		if workflow.RunMode != RunModeManual {
			t.Fatalf("%s RunMode = %q, want manual", workflow.Name, workflow.RunMode)
		}
		if workflow.Permissions.Scheduler.Required || workflow.Permissions.Notifications.Required || workflow.Permissions.Connectors.Required {
			t.Fatalf("%s enabled scheduler/notifications/connectors: %#v", workflow.Name, workflow.Permissions)
		}
		if !workflow.LocalOnly() {
			t.Fatalf("%s LocalOnly() = false, permissions = %#v", workflow.Name, workflow.Permissions)
		}
	}
	if seen["daily_planner"].Intent != IntentExplain {
		t.Fatalf("daily_planner intent = %q, want explain", seen["daily_planner"].Intent)
	}
	if seen["followup_capture"].Intent != IntentAction {
		t.Fatalf("followup_capture intent = %q, want action", seen["followup_capture"].Intent)
	}
	if !contains(seen["followup_capture"].Safety.BlockedActions, "scheduler_create") ||
		!contains(seen["followup_capture"].Safety.BlockedActions, "connector_action") {
		t.Fatalf("followup_capture blocked actions = %v, want scheduler_create and connector_action", seen["followup_capture"].Safety.BlockedActions)
	}
}

func newTestLibrary(t *testing.T, workflows ...Workflow) Library {
	t.Helper()
	library, err := NewLibrary(workflows...)
	if err != nil {
		t.Fatalf("NewLibrary() error = %v", err)
	}
	return library
}

func contains(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
