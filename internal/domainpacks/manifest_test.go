package domainpacks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidPackDefaultsDisabled(t *testing.T) {
	dir := t.TempDir()
	writePack(t, dir, `
name: research_assistant
version: 0.1.0
description: Local research workflows and templates.
category: capability
skills:
  - skills/research_assistant
required_tools:
  - rag_search
optional_tools:
  - internet_search
  - internet_fetch
permissions:
  internet:
    required: false
    allowed_methods:
      - GET
      - HEAD
    allowed_domains:
      - Example.com
  filesystem:
    required: false
    paths:
      - workspace://reports
  scheduler:
    required: false
safety:
  approval_required_for:
    - internet_fetch
    - scheduler_create
  blocked_actions:
    - live_trading
  sensitive_data_rules:
    - redact secrets and credentials
tests:
  - tests/research_assistant.yaml
`)

	manifest, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if manifest.Name != "research_assistant" || manifest.Version != "0.1.0" {
		t.Fatalf("manifest identity = %s %s, want research_assistant 0.1.0", manifest.Name, manifest.Version)
	}
	if manifest.DefaultState != DefaultStateDisabled || manifest.EnabledByDefault() {
		t.Fatalf("default state = %q enabled=%v, want disabled", manifest.DefaultState, manifest.EnabledByDefault())
	}
	if manifest.Dir != dir {
		t.Fatalf("Dir = %q, want %q", manifest.Dir, dir)
	}
	if got := manifest.Skills; len(got) != 1 || got[0] != "skills/research_assistant" {
		t.Fatalf("skills = %#v, want packaged skill ref", got)
	}
	if got := manifest.Permissions.Internet.AllowedDomains; len(got) != 1 || got[0] != "example.com" {
		t.Fatalf("allowed domains = %#v, want normalized example.com", got)
	}
	if got := manifest.Permissions.Internet.AllowedMethods; len(got) != 2 || got[0] != "GET" || got[1] != "HEAD" {
		t.Fatalf("allowed methods = %#v, want GET/HEAD", got)
	}
}

func TestDefaultStateCanDeclareEnabledMetadata(t *testing.T) {
	manifest, err := Decode([]byte(`
name: writing_editor
version: 0.1.0
default_state: enabled
required_tools:
  - memory_search
`))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !manifest.EnabledByDefault() {
		t.Fatal("EnabledByDefault() = false, want true")
	}
}

func TestValidateRequiresNameAndVersion(t *testing.T) {
	if err := Validate(Manifest{Version: "0.1.0"}); err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("Validate(missing name) error = %v, want name requirement", err)
	}
	if err := Validate(Manifest{Name: "study_tutor"}); err == nil || !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("Validate(missing version) error = %v, want version requirement", err)
	}
}

func TestDecodeRejectsPermissionEnablementKnobs(t *testing.T) {
	_, err := Decode([]byte(`
name: web_pack
version: 0.1.0
permissions:
  internet:
    enabled: true
    allowed_methods:
      - GET
`))
	if err == nil || !strings.Contains(err.Error(), "field enabled not found") {
		t.Fatalf("Decode(permission enabled) error = %v, want strict declarative field rejection", err)
	}
}

func TestValidateRejectsProtectedPolicyOverride(t *testing.T) {
	manifest := Manifest{
		Name:    "privacy_helper",
		Version: "0.1.0",
		Safety:  Safety{PolicyOverrides: []string{"internet_policy"}},
	}
	if err := Validate(manifest); err == nil || !strings.Contains(err.Error(), "protected policy") {
		t.Fatalf("Validate(policy override) error = %v, want protected policy block", err)
	}
}

func TestValidateRejectsUnsupportedPolicyOverrideAfterValidatingList(t *testing.T) {
	manifest := Manifest{
		Name:    "workflow_helper",
		Version: "0.1.0",
		Safety:  Safety{PolicyOverrides: []string{"custom_policy"}},
	}
	if err := Validate(manifest); err == nil || !strings.Contains(err.Error(), "policy overrides are not supported") {
		t.Fatalf("Validate(policy override) error = %v, want unsupported override block", err)
	}
}

func TestValidateRejectsUnsafePermissionDeclarations(t *testing.T) {
	tests := []struct {
		name     string
		manifest Manifest
		want     string
	}{
		{
			name: "skill path escape",
			manifest: Manifest{
				Name:    "skill_pack",
				Version: "0.1.0",
				Skills:  []string{"../outside"},
			},
			want: "pack-relative",
		},
		{
			name: "private internet domain",
			manifest: Manifest{
				Name:    "web_pack",
				Version: "0.1.0",
				Permissions: Permissions{
					Internet: PermissionDeclaration{AllowedDomains: []string{"127.0.0.1"}, AllowedMethods: []string{"GET"}},
				},
			},
			want: "private/local",
		},
		{
			name: "filesystem escape",
			manifest: Manifest{
				Name:    "file_pack",
				Version: "0.1.0",
				Permissions: Permissions{
					Filesystem: PermissionDeclaration{Paths: []string{"../outside"}},
				},
			},
			want: "relative or scoped",
		},
		{
			name: "secret value",
			manifest: Manifest{
				Name:    "secret_pack",
				Version: "0.1.0",
				Permissions: Permissions{
					Secrets: PermissionDeclaration{SecretRefs: []string{"sk-test-secret-value"}},
				},
			},
			want: "references only",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.manifest)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsPolicyOverlayOverrideMode(t *testing.T) {
	manifest := Manifest{
		Name:    "marketing_pack",
		Version: "0.1.0",
		Safety: Safety{
			PolicyOverlays: []PolicyOverlay{{
				Name: "marketing_policy",
				Mode: "override",
			}},
		},
	}
	if err := Validate(manifest); err == nil || !strings.Contains(err.Error(), "cannot override") {
		t.Fatalf("Validate(policy overlay override) error = %v, want override block", err)
	}
}

func writePack(t *testing.T, dir string, content string) {
	t.Helper()
	path := filepath.Join(dir, ManifestFile)
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
