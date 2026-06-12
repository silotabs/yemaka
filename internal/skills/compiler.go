package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type WorkflowStep struct {
	Title       string `yaml:"title"`
	Instruction string `yaml:"instruction"`
	Tool        string `yaml:"tool,omitempty"`
}

type WorkflowSkillTest struct {
	Name      string   `yaml:"name"`
	Prompt    string   `yaml:"prompt"`
	WantTools []string `yaml:"want_tools,omitempty"`
}

type WorkflowCompileInput struct {
	ProfileSkillsDir   string
	Name               string
	Version            string
	Description        string
	Triggers           []string
	Steps              []WorkflowStep
	RequiredTools      []string
	VerificationChecks []string
	FailureHandling    []string
	ExamplePrompts     []string
	Tests              []WorkflowSkillTest
	Disabled           bool
}

type CompiledWorkflowSkill struct {
	Skill Skill
	Tests []WorkflowSkillTest
}

func CompileWorkflow(input WorkflowCompileInput) (CompiledWorkflowSkill, error) {
	if err := validateWorkflowToolDeclarations(input); err != nil {
		return CompiledWorkflowSkill{}, err
	}
	skill := Skill{
		Name:          strings.TrimSpace(input.Name),
		Version:       strings.TrimSpace(input.Version),
		Description:   strings.TrimSpace(input.Description),
		Triggers:      uniqueStrings(input.Triggers),
		RequiredTools: uniqueKnownTools(input.RequiredTools),
		Permissions:   generatedPermissions(uniqueKnownTools(input.RequiredTools)),
		ContextBudget: ContextBudget{MaxInstructionChars: 1800, MaxExamples: 1},
		Disabled:      input.Disabled,
	}
	if skill.Version == "" {
		skill.Version = "0.1.0"
	}
	if len(skill.RequiredTools) == 0 {
		for _, step := range input.Steps {
			if KnownTool(step.Tool) {
				skill.RequiredTools = append(skill.RequiredTools, strings.TrimSpace(step.Tool))
			}
		}
		skill.RequiredTools = uniqueKnownTools(skill.RequiredTools)
	}
	skill.Permissions = generatedPermissions(skill.RequiredTools)
	skill.Instructions = workflowInstructions(input, skill.RequiredTools)
	if err := Validate(skill); err != nil {
		return CompiledWorkflowSkill{}, err
	}
	return CompiledWorkflowSkill{Skill: skill, Tests: normalizeWorkflowTests(input.Tests, input.ExamplePrompts, skill.RequiredTools)}, nil
}

func validateWorkflowToolDeclarations(input WorkflowCompileInput) error {
	for _, tool := range input.RequiredTools {
		tool = strings.TrimSpace(tool)
		if tool != "" && !KnownTool(tool) {
			return fmt.Errorf("unknown or unsafe required tool: %s", tool)
		}
	}
	for _, step := range input.Steps {
		tool := strings.TrimSpace(step.Tool)
		if tool != "" && !KnownTool(tool) {
			return fmt.Errorf("unknown or unsafe workflow step tool: %s", tool)
		}
	}
	return nil
}

func WriteCompiledWorkflow(input WorkflowCompileInput) (CompiledWorkflowSkill, error) {
	profileSkillsDir := strings.TrimSpace(input.ProfileSkillsDir)
	if profileSkillsDir == "" {
		return CompiledWorkflowSkill{}, fmt.Errorf("profile skills directory is required")
	}
	compiled, err := CompileWorkflow(input)
	if err != nil {
		return CompiledWorkflowSkill{}, err
	}
	compiled.Skill.Disabled = true
	target := filepath.Join(profileSkillsDir, compiled.Skill.Name)
	compiled, err = WriteCompiledWorkflowPackage(target, compiled)
	if err != nil {
		return CompiledWorkflowSkill{}, err
	}
	compiled.Skill.Source = "profile"
	return compiled, nil
}

// WriteCompiledWorkflowPackage writes a compiled workflow skill to an exact
// skill directory without registering or enabling it anywhere.
func WriteCompiledWorkflowPackage(dir string, compiled CompiledWorkflowSkill) (CompiledWorkflowSkill, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return CompiledWorkflowSkill{}, fmt.Errorf("compiled workflow skill directory is required")
	}
	if err := Validate(compiled.Skill); err != nil {
		return CompiledWorkflowSkill{}, err
	}
	if err := writeSkillPackage(dir, compiled.Skill, compiled.Skill.Instructions); err != nil {
		return CompiledWorkflowSkill{}, err
	}
	if len(compiled.Tests) > 0 {
		if err := writeWorkflowTests(dir, compiled.Tests); err != nil {
			return CompiledWorkflowSkill{}, err
		}
	}
	loaded, err := Load(dir)
	if err != nil {
		return CompiledWorkflowSkill{}, err
	}
	compiled.Skill = loaded
	return compiled, nil
}

func workflowInstructions(input WorkflowCompileInput, tools []string) string {
	steps := make([]string, 0, len(input.Steps))
	for _, step := range input.Steps {
		line := strings.TrimSpace(step.Instruction)
		if line == "" {
			line = strings.TrimSpace(step.Title)
		}
		if line == "" {
			continue
		}
		if KnownTool(step.Tool) {
			line = line + " Tool: " + strings.TrimSpace(step.Tool) + "."
		}
		steps = append(steps, line)
	}
	if len(steps) == 0 {
		steps = append(steps, "Run the saved workflow using local context and bounded tool calls.")
	}
	verification := uniqueStrings(input.VerificationChecks)
	if len(verification) == 0 {
		verification = []string{"Confirm required tools succeeded or report the failure clearly."}
	}
	failures := uniqueStrings(input.FailureHandling)
	if len(failures) == 0 {
		failures = []string{"Ask for missing context or permission instead of guessing."}
	}
	examples := uniqueStrings(input.ExamplePrompts)
	if len(examples) == 0 {
		examples = []string{"Use this saved workflow for a similar local task."}
	}
	return strings.Join([]string{
		"# Compiled Workflow Skill",
		"",
		"## Steps",
		bullets(steps),
		"",
		"## Required Tools",
		bullets(tools),
		"",
		"## Verification Checks",
		bullets(verification),
		"",
		"## Failure Handling",
		bullets(failures),
		"",
		"## Example Prompts",
		bullets(examples),
	}, "\n")
}

func normalizeWorkflowTests(tests []WorkflowSkillTest, examples []string, tools []string) []WorkflowSkillTest {
	cleaned := make([]WorkflowSkillTest, 0, len(tests)+len(examples))
	for _, test := range tests {
		test.Name = strings.TrimSpace(test.Name)
		test.Prompt = strings.TrimSpace(test.Prompt)
		test.WantTools = uniqueKnownTools(test.WantTools)
		if test.Name == "" || test.Prompt == "" {
			continue
		}
		cleaned = append(cleaned, test)
	}
	if len(cleaned) == 0 {
		for i, example := range uniqueStrings(examples) {
			cleaned = append(cleaned, WorkflowSkillTest{
				Name:      fmt.Sprintf("example_%d", i+1),
				Prompt:    example,
				WantTools: tools,
			})
		}
	}
	return cleaned
}

func writeWorkflowTests(dir string, tests []WorkflowSkillTest) error {
	data, err := yaml.Marshal(struct {
		Tests []WorkflowSkillTest `yaml:"tests"`
	}{Tests: tests})
	if err != nil {
		return fmt.Errorf("encode workflow skill tests: %w", err)
	}
	testsDir := filepath.Join(dir, "tests")
	if err := os.MkdirAll(testsDir, 0o755); err != nil {
		return fmt.Errorf("create workflow skill tests directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(testsDir, "workflow.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("write workflow skill tests: %w", err)
	}
	return nil
}
