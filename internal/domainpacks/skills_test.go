package domainpacks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/skills"
)

func TestWriteWorkflowSkillPackCreatesManifestAndCompiledSkill(t *testing.T) {
	packDir := t.TempDir()
	pack, err := WriteWorkflowSkillPack(WorkflowSkillPackInput{
		PackDir:     packDir,
		Name:        "review_pack",
		Version:     "0.1.0",
		Description: "Local review workflow pack.",
		Category:    "capability",
		Workflows: []skills.WorkflowCompileInput{{
			Name:        "review_workflow",
			Description: "Review a local diff.",
			Triggers:    []string{"review local diff"},
			Steps: []skills.WorkflowStep{
				{Title: "Read", Instruction: "Read the relevant local file.", Tool: "read_file"},
				{Title: "Inspect", Instruction: "Inspect the current git diff.", Tool: "git_diff"},
				{Title: "Verify", Instruction: "Run focused tests.", Tool: "run_tests"},
			},
			VerificationChecks: []string{"Report failed tests with the exact command."},
			ExamplePrompts:     []string{"Review this local diff and run focused tests."},
		}},
	})
	if err != nil {
		t.Fatalf("WriteWorkflowSkillPack() error = %v", err)
	}

	if pack.Manifest.Name != "review_pack" || pack.Manifest.DefaultState != DefaultStateDisabled {
		t.Fatalf("manifest = %#v, want disabled review_pack", pack.Manifest)
	}
	if len(pack.Manifest.Skills) != 1 || pack.Manifest.Skills[0] != "skills/review_workflow" {
		t.Fatalf("manifest skills = %#v, want packaged review_workflow skill", pack.Manifest.Skills)
	}
	for _, want := range []string{"git_diff", "read_file", "run_tests"} {
		if !containsPackString(pack.Manifest.RequiredTools, want) {
			t.Fatalf("required tools = %#v, missing %q", pack.Manifest.RequiredTools, want)
		}
	}
	if len(pack.Manifest.Tests) != 1 || pack.Manifest.Tests[0] != "skills/review_workflow/tests/workflow.yaml" {
		t.Fatalf("manifest tests = %#v, want workflow test reference", pack.Manifest.Tests)
	}
	if !containsPackString(pack.Manifest.Permissions.Filesystem.Scopes, "read") {
		t.Fatalf("filesystem scopes = %#v, want read permission declaration", pack.Manifest.Permissions.Filesystem.Scopes)
	}

	loadedManifest, err := Load(packDir)
	if err != nil {
		t.Fatalf("Load(packDir) error = %v", err)
	}
	if len(loadedManifest.Skills) != 1 || loadedManifest.Skills[0] != "skills/review_workflow" {
		t.Fatalf("loaded manifest skills = %#v, want review workflow ref", loadedManifest.Skills)
	}
	loadedSkill, err := skills.Load(filepath.Join(packDir, "skills", "review_workflow"))
	if err != nil {
		t.Fatalf("skills.Load(pack skill) error = %v", err)
	}
	if !strings.Contains(loadedSkill.Instructions, "## Verification Checks") {
		t.Fatalf("compiled skill instructions missing verification section: %s", loadedSkill.Instructions)
	}
	if _, err := os.Stat(filepath.Join(packDir, "skills", "review_workflow", "tests", "workflow.yaml")); err != nil {
		t.Fatalf("workflow test output missing: %v", err)
	}
}

func TestInstalledWorkflowSkillPackInactiveUntilExplicitlyEnabled(t *testing.T) {
	sourcePack := t.TempDir()
	if _, err := WriteWorkflowSkillPack(WorkflowSkillPackInput{
		PackDir: sourcePack,
		Name:    "triage_pack",
		Workflows: []skills.WorkflowCompileInput{{
			Name:        "triage_workflow",
			Description: "Triage a local issue.",
			Triggers:    []string{"triage local issue"},
			Steps: []skills.WorkflowStep{
				{Instruction: "Read the relevant local files.", Tool: "read_file"},
			},
			ExamplePrompts: []string{"Triage this local issue."},
		}},
		DefaultState: DefaultStateEnabled,
	}); err != nil {
		t.Fatalf("WriteWorkflowSkillPack() error = %v", err)
	}

	profilePackRoot := t.TempDir()
	installed, err := Install(profilePackRoot, sourcePack)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if installed.Enabled {
		t.Fatal("installed pack enabled = true, want disabled until explicit enable")
	}
	dirs, err := EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs(disabled) error = %v", err)
	}
	if len(dirs) != 0 {
		t.Fatalf("EnabledSkillDirs(disabled) = %#v, want none", dirs)
	}

	enabled, err := SetEnabled(profilePackRoot, "triage_pack", true)
	if err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("enabled pack status Enabled = false, want true")
	}
	dirs, err = EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs(enabled) error = %v", err)
	}
	if len(dirs) != 1 || filepath.Base(dirs[0]) != PackSkillsDir {
		t.Fatalf("EnabledSkillDirs(enabled) = %#v, want one skills root", dirs)
	}
	registry, err := skills.LoadRegistry(dirs)
	if err != nil {
		t.Fatalf("LoadRegistry(pack skill dirs) error = %v", err)
	}
	if _, ok := registry.Get("triage_workflow"); !ok {
		t.Fatalf("enabled pack skill registry missing triage_workflow; statuses = %#v", registry.Statuses())
	}
}

func containsPackString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
