package skills_test

import (
	"path/filepath"
	"testing"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
)

func TestDomainPackSkillsSelectedOnlyWhenPackEnabled(t *testing.T) {
	sourcePack := t.TempDir()
	if _, err := domainpacks.WriteWorkflowSkillPack(domainpacks.WorkflowSkillPackInput{
		PackDir:      sourcePack,
		Name:         "deploy_pack",
		Version:      "0.1.0",
		Description:  "Local deployment workflows.",
		Category:     "capability",
		DefaultState: domainpacks.DefaultStateEnabled,
		Workflows: []skills.WorkflowCompileInput{{
			Name:        "deploy_workflow",
			Description: "Deploy a local app through an approved workflow.",
			Triggers:    []string{"deploy local app"},
			Steps: []skills.WorkflowStep{
				{Instruction: "Inspect local deployment files.", Tool: "read_file"},
				{Instruction: "Run only an explicitly approved deployment command.", Tool: "run_shell_safe"},
			},
			ExamplePrompts: []string{"Deploy local app."},
		}},
	}); err != nil {
		t.Fatalf("WriteWorkflowSkillPack() error = %v", err)
	}

	profilePackRoot := t.TempDir()
	installed, err := domainpacks.Install(profilePackRoot, sourcePack)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if installed.Enabled {
		t.Fatal("installed domain pack Enabled = true, want disabled until explicit enable")
	}

	available := map[string]bool{"read_file": true, "run_shell_safe": true}
	registry := loadPackSkillRegistry(t, profilePackRoot)
	if selected, ok := registry.Select("Deploy local app", available); ok {
		t.Fatalf("Select(disabled pack skill) = %q, want no skill", selected.Name)
	}

	if _, err := domainpacks.SetEnabled(profilePackRoot, "deploy_pack", true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	registry = loadPackSkillRegistry(t, profilePackRoot)
	selected, ok := registry.Select("Deploy local app", available)
	if !ok {
		t.Fatal("Select(enabled pack skill) returned no skill")
	}
	if selected.Name != "deploy_workflow" {
		t.Fatalf("Select(enabled pack skill) = %q, want deploy_workflow", selected.Name)
	}
	if filepath.Base(filepath.Dir(selected.Dir)) != domainpacks.PackSkillsDir {
		t.Fatalf("selected skill Dir = %q, want domain pack skills directory", selected.Dir)
	}
}

func TestExplanationPromptDoesNotSelectActionWorkflowSkill(t *testing.T) {
	compiled, err := skills.CompileWorkflow(skills.WorkflowCompileInput{
		Name:        "deploy_workflow",
		Version:     "0.1.0",
		Description: "Deploy an app through an approved workflow.",
		Triggers:    []string{"deploy"},
		Steps: []skills.WorkflowStep{
			{Instruction: "Inspect local deployment files.", Tool: "read_file"},
			{Instruction: "Run only an explicitly approved deployment command.", Tool: "run_shell_safe"},
		},
		ExamplePrompts: []string{"Deploy this app."},
	})
	if err != nil {
		t.Fatalf("CompileWorkflow() error = %v", err)
	}
	registry := skills.Registry{
		Skills: map[string]skills.Skill{
			compiled.Skill.Name: compiled.Skill,
		},
	}
	available := map[string]bool{"read_file": true, "run_shell_safe": true}

	if selected, ok := registry.Select("How do I deploy a website?", available); ok {
		t.Fatalf("Select(explanation prompt) = %q, want no action workflow skill", selected.Name)
	}

	selected, ok := registry.Select("Deploy this website using the saved workflow.", available)
	if !ok {
		t.Fatal("Select(action prompt) returned no skill")
	}
	if selected.Name != "deploy_workflow" {
		t.Fatalf("Select(action prompt) = %q, want deploy_workflow", selected.Name)
	}
}

func TestFollowupCaptureSelectsSingularFollowupPrompt(t *testing.T) {
	registry := skills.Registry{
		Skills: map[string]skills.Skill{
			"followup_capture": {
				Name:          "followup_capture",
				Version:       "0.1.0",
				Description:   "Capture confirmed follow-ups.",
				Triggers:      []string{"capture follow ups", "save follow-up tasks", "remember action items"},
				RequiredTools: []string{"memory_search", "memory_write"},
				Instructions:  "Use memory_write only for confirmed local follow-up memory.",
			},
		},
	}
	available := map[string]bool{"memory_search": true, "memory_write": true}

	selected, ok := registry.Select("capture this follow-up: email Alex tomorrow", available)
	if !ok {
		t.Fatal("Select(follow-up prompt) returned no skill")
	}
	if selected.Name != "followup_capture" {
		t.Fatalf("Select(follow-up prompt) = %q, want followup_capture", selected.Name)
	}
}

func loadPackSkillRegistry(t *testing.T, profilePackRoot string) skills.Registry {
	t.Helper()
	dirs, err := domainpacks.EnabledSkillDirs(profilePackRoot)
	if err != nil {
		t.Fatalf("EnabledSkillDirs() error = %v", err)
	}
	registry, err := skills.LoadRegistry(dirs)
	if err != nil {
		t.Fatalf("LoadRegistry(%v) error = %v", dirs, err)
	}
	return registry
}
