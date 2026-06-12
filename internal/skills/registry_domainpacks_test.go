package skills_test

import (
	"path/filepath"
	"testing"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
)

func TestLoadProfileRegistrySkipsDisabledDomainPackSkills(t *testing.T) {
	profilePackRoot := testProfilePackRoot(t)
	installRegistryPack(t, profilePackRoot, "triage_pack", "triage_workflow")

	enabledDirs, err := domainpacks.EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs(disabled) error = %v", err)
	}
	if len(enabledDirs) != 0 {
		t.Fatalf("EnabledSkillDirs(disabled) = %#v, want no skill roots", enabledDirs)
	}

	registry, err := skills.LoadProfileRegistry("", "", enabledDirs)
	if err != nil {
		t.Fatalf("LoadProfileRegistry(disabled pack) error = %v", err)
	}
	if _, ok := registry.Get("triage_workflow"); ok {
		t.Fatal("disabled domain-pack skill was loaded")
	}
}

func TestLoadProfileRegistryLoadsEnabledDomainPackSkills(t *testing.T) {
	profilePackRoot := testProfilePackRoot(t)
	installRegistryPack(t, profilePackRoot, "triage_pack", "triage_workflow")

	if _, err := domainpacks.SetEnabled(profilePackRoot, "triage_pack", true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	enabledDirs, err := domainpacks.EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs(enabled) error = %v", err)
	}

	registry, err := skills.LoadProfileRegistry("", "", enabledDirs)
	if err != nil {
		t.Fatalf("LoadProfileRegistry(enabled pack) error = %v", err)
	}
	skill, ok := registry.Get("triage_workflow")
	if !ok {
		t.Fatalf("enabled domain-pack skill missing; statuses = %#v", registry.Statuses())
	}
	if skill.Source != "domain_pack" {
		t.Fatalf("enabled domain-pack skill Source = %q, want domain_pack", skill.Source)
	}
}

func TestProfileRegistryDirsLoadsProfileAfterEnabledDomainPacks(t *testing.T) {
	dirs := skills.ProfileRegistryDirs("skills/default", "/profile/skills", []string{
		"/profile/domain_packs/alpha/skills",
		"/profile/domain_packs/beta/skills",
	})
	want := []string{
		"skills/default",
		"/profile/domain_packs/alpha/skills",
		"/profile/domain_packs/beta/skills",
		"/profile/skills",
	}
	if len(dirs) != len(want) {
		t.Fatalf("ProfileRegistryDirs() = %#v, want %#v", dirs, want)
	}
	for i := range want {
		if dirs[i] != want[i] {
			t.Fatalf("ProfileRegistryDirs()[%d] = %q, want %q; all dirs = %#v", i, dirs[i], want[i], dirs)
		}
	}
}

func installRegistryPack(t *testing.T, profilePackRoot string, packName string, skillName string) {
	t.Helper()
	sourcePack := filepath.Join(t.TempDir(), packName)
	if _, err := domainpacks.WriteWorkflowSkillPack(domainpacks.WorkflowSkillPackInput{
		PackDir: sourcePack,
		Name:    packName,
		Workflows: []skills.WorkflowCompileInput{{
			Name:        skillName,
			Description: "Triage local project work.",
			Triggers:    []string{"triage local work"},
			Steps: []skills.WorkflowStep{
				{Instruction: "Read the relevant local files.", Tool: "read_file"},
			},
			ExamplePrompts: []string{"Triage this local project work."},
		}},
		DefaultState: domainpacks.DefaultStateEnabled,
	}); err != nil {
		t.Fatalf("WriteWorkflowSkillPack() error = %v", err)
	}
	status, err := domainpacks.Install(profilePackRoot, sourcePack)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if status.Enabled {
		t.Fatal("installed pack enabled = true, want disabled until explicit enable")
	}
}

func testProfilePackRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "profiles", "default", "domain_packs")
}
