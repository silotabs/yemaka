package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteCompiledWorkflowCreatesDisabledProfileSkillAndTests(t *testing.T) {
	profileSkills := t.TempDir()
	compiled, err := WriteCompiledWorkflow(WorkflowCompileInput{
		ProfileSkillsDir: profileSkills,
		Name:             "review_workflow",
		Description:      "Review a local diff.",
		Triggers:         []string{"review local diff"},
		Steps: []WorkflowStep{
			{Title: "Inspect", Instruction: "Inspect the current git diff.", Tool: "git_diff"},
			{Title: "Verify", Instruction: "Run the focused tests.", Tool: "run_tests"},
		},
		VerificationChecks: []string{"Report test failures with the command that failed."},
		FailureHandling:    []string{"If tests are unavailable, explain that gap."},
		ExamplePrompts:     []string{"Review this local diff and run tests."},
		Disabled:           true,
	})
	if err != nil {
		t.Fatalf("WriteCompiledWorkflow() error = %v", err)
	}
	if compiled.Skill.Name != "review_workflow" || !compiled.Skill.Disabled {
		t.Fatalf("compiled skill = %#v, want disabled review_workflow", compiled.Skill)
	}
	if !contains(compiled.Skill.RequiredTools, "git_diff") || !contains(compiled.Skill.RequiredTools, "run_tests") {
		t.Fatalf("RequiredTools = %v, want git_diff and run_tests", compiled.Skill.RequiredTools)
	}
	if !strings.Contains(compiled.Skill.Instructions, "## Failure Handling") {
		t.Fatalf("instructions missing failure handling: %s", compiled.Skill.Instructions)
	}
	if _, err := os.Stat(filepath.Join(profileSkills, "review_workflow", "tests", "workflow.yaml")); err != nil {
		t.Fatalf("workflow tests missing: %v", err)
	}
}

func TestWriteCompiledWorkflowPackageWritesExactSkillDir(t *testing.T) {
	compiled, err := CompileWorkflow(WorkflowCompileInput{
		Name:        "triage_workflow",
		Description: "Triage a local project issue.",
		Triggers:    []string{"triage local issue"},
		Steps: []WorkflowStep{
			{Title: "Inspect", Instruction: "Inspect relevant local files.", Tool: "read_file"},
		},
		ExamplePrompts: []string{"Triage this local project issue."},
	})
	if err != nil {
		t.Fatalf("CompileWorkflow() error = %v", err)
	}
	if !strings.Contains(compiled.Skill.Instructions, "# Compiled Workflow Skill") ||
		!strings.Contains(compiled.Skill.Instructions, "## Verification Checks") ||
		!strings.Contains(compiled.Skill.Instructions, "## Failure Handling") {
		t.Fatalf("compiled instructions are not workflow-shaped: %s", compiled.Skill.Instructions)
	}
	target := filepath.Join(t.TempDir(), "pack", "skills", "triage_workflow")
	written, err := WriteCompiledWorkflowPackage(target, compiled)
	if err != nil {
		t.Fatalf("WriteCompiledWorkflowPackage() error = %v", err)
	}
	if written.Skill.Dir != target {
		t.Fatalf("Dir = %q, want %q", written.Skill.Dir, target)
	}
	skillYAML, err := os.ReadFile(filepath.Join(target, "skill.yaml"))
	if err != nil {
		t.Fatalf("skill.yaml missing: %v", err)
	}
	if !strings.Contains(string(skillYAML), "disabled: false") {
		t.Fatalf("skill.yaml did not preserve exact compiled package state: %s", skillYAML)
	}
	workflowTests, err := os.ReadFile(filepath.Join(target, "tests", "workflow.yaml"))
	if err != nil {
		t.Fatalf("workflow tests missing: %v", err)
	}
	if !strings.Contains(string(workflowTests), "prompt: Triage this local project issue.") ||
		!strings.Contains(string(workflowTests), "read_file") {
		t.Fatalf("workflow tests are not shaped from example prompt and tools: %s", workflowTests)
	}
}

func TestWriteCompiledWorkflowDefaultsProfileSkillDisabled(t *testing.T) {
	profileSkills := t.TempDir()
	compiled, err := WriteCompiledWorkflow(WorkflowCompileInput{
		ProfileSkillsDir: profileSkills,
		Name:             "triage_workflow",
		Description:      "Triage a local project issue.",
		Triggers:         []string{"triage local issue"},
		Steps: []WorkflowStep{
			{Title: "Inspect", Instruction: "Inspect relevant local files.", Tool: "read_file"},
		},
		ExamplePrompts: []string{"Triage this local project issue."},
	})
	if err != nil {
		t.Fatalf("WriteCompiledWorkflow() error = %v", err)
	}
	if !compiled.Skill.Disabled {
		t.Fatal("profile workflow skill Disabled = false, want disabled until reviewed")
	}
	skillYAML, err := os.ReadFile(filepath.Join(profileSkills, "triage_workflow", "skill.yaml"))
	if err != nil {
		t.Fatalf("skill.yaml missing: %v", err)
	}
	if !strings.Contains(string(skillYAML), "disabled: true") {
		t.Fatalf("profile workflow skill was not written disabled: %s", skillYAML)
	}
	registry, err := LoadRegistry([]string{profileSkills})
	if err != nil {
		t.Fatalf("LoadRegistry(profile) error = %v", err)
	}
	if selected, ok := registry.Select("Triage this local project issue.", map[string]bool{"read_file": true}); ok {
		t.Fatalf("disabled profile workflow skill was selected: %s", selected.Name)
	}
}

func TestCompileWorkflowRejectsUnknownTools(t *testing.T) {
	_, err := CompileWorkflow(WorkflowCompileInput{
		Name:          "bad_workflow",
		Description:   "Bad workflow.",
		Triggers:      []string{"bad"},
		RequiredTools: []string{"curl_internet"},
		Steps:         []WorkflowStep{{Instruction: "Use an unknown network tool."}},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown or unsafe") {
		t.Fatalf("CompileWorkflow() error = %v, want validation rejection", err)
	}
}
