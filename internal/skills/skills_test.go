package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRegistryLoadsDefaultSkills(t *testing.T) {
	registry, err := LoadRegistry([]string{filepath.Join("..", "..", "skills", "default")})
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	for _, name := range []string{"project_explainer", "document_summary", "git_diff_review", "python_bugfix", "code_review"} {
		if _, ok := registry.Get(name); !ok {
			t.Fatalf("missing default skill %s", name)
		}
	}
}

func TestSelectSkillDeterministically(t *testing.T) {
	registry, err := LoadRegistry([]string{filepath.Join("..", "..", "skills", "default")})
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	available := map[string]bool{
		"workspace_summary": true,
		"read_file":         true,
		"search_files":      true,
		"git_diff":          true,
		"rag_search":        true,
	}

	skill, ok := registry.Select("Explain this codebase", available)
	if !ok {
		t.Fatal("Select() returned no skill")
	}
	if skill.Name != "project_explainer" {
		t.Fatalf("skill = %q, want project_explainer", skill.Name)
	}

	skill, ok = registry.Select("Review my diff", available)
	if !ok {
		t.Fatal("Select() returned no skill")
	}
	if skill.Name != "git_diff_review" {
		t.Fatalf("skill = %q, want git_diff_review", skill.Name)
	}
}

func TestValidateRejectsNetworkSkill(t *testing.T) {
	skill := Skill{
		Name:          "bad_skill",
		Version:       "0.1.0",
		Description:   "bad skill",
		Triggers:      []string{"bad"},
		RequiredTools: []string{"read_file"},
		Instructions:  "Do something.",
		Permissions:   Permissions{Network: true},
	}
	if err := Validate(skill); err == nil {
		t.Fatal("Validate() should reject network-enabled skill")
	}
}

func TestValidateRejectsUnknownTool(t *testing.T) {
	skill := Skill{
		Name:          "bad_tool",
		Version:       "0.1.0",
		Description:   "bad skill",
		Triggers:      []string{"bad"},
		RequiredTools: []string{"curl_internet"},
		Instructions:  "Do something.",
	}
	if err := Validate(skill); err == nil {
		t.Fatal("Validate() should reject unknown tools")
	}
}

func TestLoadRejectsOversizedInstructions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "skill.yaml"), `name: too_big
version: 0.1.0
description: Too large.
triggers:
  - too big
required_tools:
  - read_file
permissions:
  network: false
context_budget:
  max_instruction_chars: 4
`)
	writeFile(t, filepath.Join(dir, "instructions.md"), "This is too long.")

	if _, err := Load(dir); err == nil {
		t.Fatal("Load() should reject oversized instructions")
	}
}

func TestSetEnabledOverridesDefaultSkillInProfile(t *testing.T) {
	registry, err := LoadRegistry([]string{filepath.Join("..", "..", "skills", "default")})
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	skill, ok := registry.Get("project_explainer")
	if !ok {
		t.Fatal("missing project_explainer")
	}
	profileSkills := t.TempDir()
	disabled, err := SetEnabled(profileSkills, skill, false)
	if err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	if !disabled.Disabled {
		t.Fatal("Disabled = false, want true")
	}
	registry, err = LoadRegistry([]string{filepath.Join("..", "..", "skills", "default"), profileSkills})
	if err != nil {
		t.Fatalf("LoadRegistry(profile) error = %v", err)
	}
	loaded, ok := registry.Get("project_explainer")
	if !ok {
		t.Fatal("missing overridden project_explainer")
	}
	if !loaded.Disabled {
		t.Fatal("overridden skill Disabled = false, want true")
	}
	if selected, ok := registry.Select("Explain this project", map[string]bool{"workspace_summary": true, "read_file": true}); ok && selected.Name == "project_explainer" {
		t.Fatal("disabled skill was selected")
	}
}

func TestCreateFromSessionWritesCompleteProfileSkill(t *testing.T) {
	profileSkills := t.TempDir()
	created, err := CreateFromSession(GenerateInput{
		ProfileSkillsDir: profileSkills,
		ConversationID:   "conv_test",
		Messages: []TranscriptMessage{
			{Role: "user", Content: "Run tests and explain failures"},
			{Role: "assistant", Content: "Tests failed because one assertion expected old output."},
		},
		ToolRuns: []ToolRunSummary{{ToolName: "run_tests", Status: "completed"}},
	})
	if err != nil {
		t.Fatalf("CreateFromSession() error = %v", err)
	}
	if created.Name == "" {
		t.Fatal("generated skill name is empty")
	}
	if created.Source != "profile" {
		t.Fatalf("Source = %q, want profile", created.Source)
	}
	if !created.Disabled {
		t.Fatal("generated skill Disabled = false, want true until reviewed")
	}
	if !contains(created.RequiredTools, "run_tests") {
		t.Fatalf("RequiredTools = %v, want run_tests", created.RequiredTools)
	}
	if !strings.Contains(created.Instructions, "## Verification Checks") {
		t.Fatalf("instructions missing verification section: %s", created.Instructions)
	}
	if !strings.Contains(created.Instructions, "## Known Mistakes") {
		t.Fatalf("instructions missing known mistakes: %s", created.Instructions)
	}
	if _, err := os.Stat(filepath.Join(profileSkills, created.Name, "skill.yaml")); err != nil {
		t.Fatalf("generated skill.yaml missing: %v", err)
	}

	registry, err := LoadRegistry([]string{profileSkills})
	if err != nil {
		t.Fatalf("LoadRegistry(profile) error = %v", err)
	}
	loaded, ok := registry.Get(created.Name)
	if !ok {
		t.Fatalf("generated skill %q missing from registry", created.Name)
	}
	if !loaded.Disabled {
		t.Fatal("loaded generated skill Disabled = false, want true until reviewed")
	}
	statuses := registry.Statuses()
	if len(statuses) != 1 || statuses[0].Name != created.Name || statuses[0].Enabled || !statuses[0].Valid {
		t.Fatalf("generated skill status = %#v, want valid disabled reviewable skill", statuses)
	}
	if selected, ok := registry.Select("Run tests and explain failures", map[string]bool{"run_tests": true}); ok && selected.Name == created.Name {
		t.Fatal("disabled generated skill was selected before review")
	}
}

func TestCreateAndImproveFromSessionRedactSensitiveText(t *testing.T) {
	profileSkills := t.TempDir()
	created, err := CreateFromSession(GenerateInput{
		ProfileSkillsDir: profileSkills,
		ConversationID:   "conv_secret",
		Messages: []TranscriptMessage{
			{Role: "user", Content: "Fix tests using token=super-secret-value in /Users/example/project"},
			{Role: "assistant", Content: "Done."},
		},
		ToolRuns: []ToolRunSummary{{ToolName: "run_tests", Status: "completed"}},
	})
	if err != nil {
		t.Fatalf("CreateFromSession() error = %v", err)
	}
	rendered := created.Description + "\n" + strings.Join(created.Triggers, "\n") + "\n" + created.Instructions + "\n" + created.Name
	if strings.Contains(rendered, "super-secret") || strings.Contains(rendered, "/Users/example") {
		t.Fatalf("generated skill leaked sensitive text: %s", rendered)
	}

	improved, err := ImproveFromSession(ImproveInput{
		ProfileSkillsDir: profileSkills,
		ConversationID:   "conv_secret_2",
		Skill:            created,
		Messages: []TranscriptMessage{
			{Role: "user", Content: "Review diff with api_key=sk-thisshouldberemoved123 from /Users/example/project"},
		},
		ToolRuns: []ToolRunSummary{{ToolName: "git_diff", Status: "completed"}},
	})
	if err != nil {
		t.Fatalf("ImproveFromSession() error = %v", err)
	}
	rendered = improved.Description + "\n" + strings.Join(improved.Triggers, "\n") + "\n" + improved.Instructions
	if strings.Contains(rendered, "sk-this") || strings.Contains(rendered, "/Users/example") {
		t.Fatalf("improved skill leaked sensitive text: %s", rendered)
	}
}

func TestImproveFromSessionUpdatesProfileSkill(t *testing.T) {
	profileSkills := t.TempDir()
	base := Skill{
		Name:          "project_explainer",
		Version:       "0.1.0",
		Description:   "Explain projects.",
		Triggers:      []string{"explain project"},
		RequiredTools: []string{"read_file"},
		Permissions:   Permissions{FilesystemRead: true},
		ContextBudget: ContextBudget{MaxInstructionChars: 1800, MaxExamples: 1},
		Instructions:  "Restate the project goal and use local files.",
	}
	improved, err := ImproveFromSession(ImproveInput{
		ProfileSkillsDir: profileSkills,
		ConversationID:   "conv_improve",
		Skill:            base,
		Messages: []TranscriptMessage{
			{Role: "user", Content: "Create a project map and explain the codebase"},
			{Role: "assistant", Content: "The project map shows a local Go core."},
		},
		ToolRuns: []ToolRunSummary{{ToolName: "project_map", Status: "completed"}},
	})
	if err != nil {
		t.Fatalf("ImproveFromSession() error = %v", err)
	}
	if improved.Version != "0.1.1" {
		t.Fatalf("Version = %q, want 0.1.1", improved.Version)
	}
	if !contains(improved.RequiredTools, "project_map") {
		t.Fatalf("RequiredTools = %v, want project_map", improved.RequiredTools)
	}
	if !strings.Contains(improved.Instructions, "## Improvements From Session") {
		t.Fatalf("instructions missing improvement block: %s", improved.Instructions)
	}
	if improved.Source != "profile" {
		t.Fatalf("Source = %q, want profile", improved.Source)
	}
}

func TestImportExportLocalSkill(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source_skill")
	writeFile(t, filepath.Join(source, "skill.yaml"), `name: local_import
version: 0.1.0
description: Local import.
triggers:
  - local import
required_tools:
  - read_file
permissions:
  filesystem_read: true
  network: false
context_budget:
  max_instruction_chars: 1000
`)
	writeFile(t, filepath.Join(source, "instructions.md"), "Use local context only.")

	profileSkills := t.TempDir()
	imported, err := Import(profileSkills, source)
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if imported.Name != "local_import" {
		t.Fatalf("imported name = %q, want local_import", imported.Name)
	}
	exportParent := t.TempDir()
	exportedPath, err := Export(imported, exportParent)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(exportedPath, "instructions.md")); err != nil {
		t.Fatalf("exported instructions missing: %v", err)
	}
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
