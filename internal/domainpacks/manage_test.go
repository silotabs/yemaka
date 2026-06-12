package domainpacks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallDefaultsDisabledAndListStatus(t *testing.T) {
	source := t.TempDir()
	writePack(t, source, `
name: research_pack
version: 0.1.0
description: Research helpers.
category: research
required_tools:
  - rag_search
safety:
  approval_required_for:
    - memory_write
  blocked_actions:
    - internet_search
    - connector_action
  sensitive_data_rules:
    - Use local evidence only.
default_state: enabled
`)
	profileRoot := t.TempDir()

	installed, err := Install(profileRoot, source)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if installed.Name != "research_pack" || !installed.Valid || !installed.Installed {
		t.Fatalf("installed status = %#v, want valid installed research_pack", installed)
	}
	if installed.Enabled {
		t.Fatal("installed pack Enabled = true, want disabled until explicit enable")
	}
	if got := strings.Join(installed.ApprovalRequiredFor, ","); got != "memory_write" {
		t.Fatalf("installed approval = %q, want memory_write", got)
	}
	if got := strings.Join(installed.BlockedActions, ","); got != "internet_search,connector_action" {
		t.Fatalf("installed blocked actions = %q, want internet_search,connector_action", got)
	}
	if got := strings.Join(installed.SensitiveDataRules, " "); got != "Use local evidence only." {
		t.Fatalf("installed sensitive data rules = %q, want rule", got)
	}
	if !strings.Contains(installed.SafetySummary, "Approval required for memory_write") || !strings.Contains(installed.SafetySummary, "blocked actions") {
		t.Fatalf("installed SafetySummary = %q, want approval and blocked-action summary", installed.SafetySummary)
	}

	statuses, err := List(profileRoot)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Name != "research_pack" || statuses[0].Enabled {
		t.Fatalf("statuses = %#v, want one disabled research_pack", statuses)
	}
	if got := strings.Join(statuses[0].BlockedActions, ","); got != "internet_search,connector_action" {
		t.Fatalf("listed blocked actions = %q, want installed safety metadata", got)
	}
}

func TestReviewInstalledPackExposesSafetyMetadata(t *testing.T) {
	source := t.TempDir()
	writePack(t, source, `
name: review_pack
version: 0.1.0
description: Reviewable pack.
category: review
skills:
  - skills/review_skill
required_tools:
  - rag_search
optional_tools:
  - memory_write
safety:
  blocked_actions:
    - internet_search
    - write_file
  sensitive_data_rules:
    - Keep private identifiers out of summaries.
tests:
  - skills/review_skill/tests/workflow.yaml
`)
	writeSkill(t, filepath.Join(source, "skills", "review_skill"), `
name: review_skill
version: 0.1.0
description: Review skill.
triggers:
  - review local pack notes
required_tools:
  - rag_search
context_budget:
  max_instruction_chars: 400
`, "Use local notes only.")

	profileRoot := t.TempDir()
	if _, err := Install(profileRoot, source); err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	review, err := ReviewInstalled(profileRoot, "review_pack")
	if err != nil {
		t.Fatalf("ReviewInstalled() error = %v", err)
	}
	if review.Name != "review_pack" || !review.Installed || review.Enabled || !review.Valid {
		t.Fatalf("review = %#v, want installed disabled valid review_pack", review)
	}
	if review.Manifest == nil || review.Manifest.Name != "review_pack" {
		t.Fatalf("review.Manifest = %#v, want manifest copy", review.Manifest)
	}
	if got := strings.Join(review.BlockedActions, ","); got != "internet_search,write_file" {
		t.Fatalf("review.BlockedActions = %q, want manifest blocked actions", got)
	}
	if got := strings.Join(review.Tests, ","); got != "skills/review_skill/tests/workflow.yaml" {
		t.Fatalf("review.Tests = %q, want manifest tests", got)
	}
	if len(review.Skills) != 1 || review.Skills[0].Name != "review_skill" || !review.Skills[0].Valid {
		t.Fatalf("review.Skills = %#v, want valid skill review", review.Skills)
	}
	if !strings.Contains(strings.Join(review.RiskyToolReasons, " "), "Memory writes require explicit user approval") {
		t.Fatalf("review.RiskyToolReasons = %#v, want memory-write reason", review.RiskyToolReasons)
	}
}

func TestReviewInstalledInvalidPackReturnsRepairMetadata(t *testing.T) {
	profileRoot := t.TempDir()
	invalidDir := filepath.Join(profileRoot, "broken_pack")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, ManifestFile), []byte("name: Broken Pack\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write invalid pack manifest: %v", err)
	}

	review, err := ReviewInstalled(profileRoot, "broken_pack")
	if err != nil {
		t.Fatalf("ReviewInstalled(invalid) error = %v", err)
	}
	if review.Valid || !review.Installed || review.Enabled {
		t.Fatalf("review = %#v, want invalid installed disabled review", review)
	}
	if review.ValidationError == "" || review.RepairHint == "" {
		t.Fatalf("review = %#v, want validation error and repair hint", review)
	}
}

func TestEnableDisablePack(t *testing.T) {
	source := t.TempDir()
	writePack(t, source, `
name: writing_pack
version: 0.1.0
required_tools:
  - memory_search
`)
	profileRoot := t.TempDir()
	if _, err := Install(profileRoot, source); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	enabled, err := SetEnabled(profileRoot, "writing_pack", true)
	if err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("Enabled = false, want true")
	}
	disabled, err := SetEnabled(profileRoot, "writing_pack", false)
	if err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatal("Enabled = true, want false")
	}
}

func TestUninstallRemovesInstalledPack(t *testing.T) {
	source := t.TempDir()
	writePack(t, source, `
name: removable_pack
version: 0.1.0
required_tools:
  - memory_search
`)
	profileRoot := t.TempDir()
	installed, err := Install(profileRoot, source)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if _, err := SetEnabled(profileRoot, "removable_pack", true); err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}

	removed, err := Uninstall(profileRoot, "removable_pack")
	if err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}
	if removed.Name != "removable_pack" || removed.Installed || removed.Enabled {
		t.Fatalf("removed = %#v, want uninstalled disabled removable_pack", removed)
	}
	if _, err := os.Stat(installed.Dir); !os.IsNotExist(err) {
		t.Fatalf("installed dir still exists or stat failed unexpectedly: %v", err)
	}
	statuses, err := List(profileRoot)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses = %#v, want no installed packs after uninstall", statuses)
	}
}

func TestUninstallInvalidPackBySafeName(t *testing.T) {
	profileRoot := t.TempDir()
	invalidDir := filepath.Join(profileRoot, "broken_pack")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(invalidDir, ManifestFile), []byte("name: Broken Pack\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write invalid pack manifest: %v", err)
	}

	removed, err := Uninstall(profileRoot, "broken_pack")
	if err != nil {
		t.Fatalf("Uninstall(invalid) error = %v", err)
	}
	if removed.Name != "broken_pack" || removed.Valid || removed.Installed {
		t.Fatalf("removed invalid pack = %#v, want invalid uninstalled broken_pack", removed)
	}
	if _, err := os.Stat(invalidDir); !os.IsNotExist(err) {
		t.Fatalf("invalid pack dir still exists or stat failed unexpectedly: %v", err)
	}
}

func TestRejectsUnsafePackNamesAndTraversal(t *testing.T) {
	for _, name := range []string{"../escape", "bad/name", ".hidden", "Pack", "x"} {
		if err := ValidateName(name); err == nil {
			t.Fatalf("ValidateName(%q) error = nil, want rejection", name)
		}
	}
	if err := ValidateName("safe_pack_1"); err != nil {
		t.Fatalf("ValidateName(safe_pack_1) error = %v", err)
	}
	if _, err := SetEnabled(t.TempDir(), "../escape", true); err == nil || !strings.Contains(err.Error(), "pack name") {
		t.Fatalf("SetEnabled traversal error = %v, want pack name rejection", err)
	}
	if _, err := Uninstall(t.TempDir(), "../escape"); err == nil || !strings.Contains(err.Error(), "pack name") {
		t.Fatalf("Uninstall traversal error = %v, want pack name rejection", err)
	}
}

func TestInstallRejectsInvalidManifest(t *testing.T) {
	source := t.TempDir()
	writePack(t, source, `
name: unsafe_pack
version: 0.1.0
permissions:
  internet:
    allowed_methods:
      - POST
`)
	if _, err := Install(t.TempDir(), source); err == nil || !strings.Contains(err.Error(), "GET and HEAD") {
		t.Fatalf("Install(invalid manifest) error = %v, want manifest validation rejection", err)
	}
}

func TestListReportsInvalidInstalledPackDisabled(t *testing.T) {
	profileRoot := t.TempDir()
	invalidDir := filepath.Join(profileRoot, "bad_pack")
	if err := os.MkdirAll(invalidDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	writePack(t, invalidDir, `
name: bad_pack
version: 0.1.0
permissions:
  filesystem:
    paths:
      - ../../outside
`)

	statuses, err := List(profileRoot)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Valid || statuses[0].Enabled || !strings.Contains(statuses[0].ValidationError, "relative or scoped") {
		t.Fatalf("statuses = %#v, want invalid disabled pack", statuses)
	}
}

func writeSkill(t *testing.T, dir string, manifest string, instructions string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("create skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(strings.TrimSpace(manifest)+"\n"), 0o644); err != nil {
		t.Fatalf("write skill.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(strings.TrimSpace(instructions)+"\n"), 0o644); err != nil {
		t.Fatalf("write instructions.md: %v", err)
	}
}
