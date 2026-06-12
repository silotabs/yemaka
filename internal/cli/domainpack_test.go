package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/workflows"
)

func TestRunDomainPackInstallListEnableDisable(t *testing.T) {
	app := newExtensionTestApp(t)
	source := t.TempDir()
	writeCLIDomainPack(t, source, `
name: research_pack
version: 0.1.0
description: Research helpers.
category: research
skills:
  - skills/research_assistant
required_tools:
  - rag_search
default_state: enabled
`)

	var out bytes.Buffer
	if err := runDomainPack(app, []string{"install", source}, &out); err != nil {
		t.Fatalf("runDomainPack(install) error = %v", err)
	}
	if !strings.Contains(out.String(), "domain pack installed: research_pack") || !strings.Contains(out.String(), "enabled: false") {
		t.Fatalf("install output = %q, want installed disabled pack", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"list"}, &out); err != nil {
		t.Fatalf("runDomainPack(list) error = %v", err)
	}
	if !strings.Contains(out.String(), "research_pack") ||
		!strings.Contains(out.String(), "enabled=false") ||
		!strings.Contains(out.String(), "valid") ||
		!strings.Contains(out.String(), "skills=skills/research_assistant") {
		t.Fatalf("list output = %q, want disabled valid research_pack with skill refs", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"skills"}, &out); err != nil {
		t.Fatalf("runDomainPack(skills) error = %v", err)
	}
	if !strings.Contains(out.String(), "research_pack") ||
		!strings.Contains(out.String(), "ref=skills/research_assistant") ||
		!strings.Contains(out.String(), "active=false") {
		t.Fatalf("skills output = %q, want disabled pack skill ref", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"enable", "research_pack"}, &out); err != nil {
		t.Fatalf("runDomainPack(enable) error = %v", err)
	}
	if !strings.Contains(out.String(), "domain pack enabled: research_pack") {
		t.Fatalf("enable output = %q, want enabled message", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"skills"}, &out); err != nil {
		t.Fatalf("runDomainPack(skills enabled) error = %v", err)
	}
	if !strings.Contains(out.String(), "active=true") {
		t.Fatalf("skills output = %q, want enabled pack skill active", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"disable", "research_pack"}, &out); err != nil {
		t.Fatalf("runDomainPack(disable) error = %v", err)
	}
	if !strings.Contains(out.String(), "domain pack disabled: research_pack") {
		t.Fatalf("disable output = %q, want disabled message", out.String())
	}
}

func TestRunDomainPackRejectsRemoteInstall(t *testing.T) {
	app := newExtensionTestApp(t)
	var out bytes.Buffer
	err := runDomainPack(app, []string{"install", "https://example.com/pack.git"}, &out)
	if err == nil || !strings.Contains(err.Error(), "remote pack installs are not supported") {
		t.Fatalf("remote install error = %v, want remote rejection", err)
	}
}

func TestRunDispatchDomainPackAlias(t *testing.T) {
	t.Setenv("YEMAKA_HOME", t.TempDir())
	var out bytes.Buffer
	if err := Run(context.Background(), []string{"domainpack", "list"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run(domainpack list) error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "" {
		t.Fatalf("domainpack list output = %q, want empty list", out.String())
	}
}

func TestRunSkillListShowsEnabledDomainPackSkills(t *testing.T) {
	app := newExtensionTestApp(t)
	source := t.TempDir()
	writeCLIDomainPack(t, source, `
name: triage_pack
version: 0.1.0
description: Triage helpers.
category: review
skills:
  - skills/triage_workflow
required_tools:
  - rag_search
`)
	writeCLISkillPackage(t, filepath.Join(source, "skills", "triage_workflow"), "triage_workflow")

	var out bytes.Buffer
	if err := runDomainPack(app, []string{"install", source}, &out); err != nil {
		t.Fatalf("runDomainPack(install) error = %v", err)
	}
	if err := reloadSkills(app); err != nil {
		t.Fatalf("reloadSkills(disabled pack) error = %v", err)
	}
	out.Reset()
	if err := runSkill(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runSkill(list disabled pack) error = %v", err)
	}
	if strings.Contains(out.String(), "triage_workflow") {
		t.Fatalf("skill list = %q, disabled pack skill should not be loaded", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"enable", "triage_pack"}, &out); err != nil {
		t.Fatalf("runDomainPack(enable) error = %v", err)
	}
	if err := reloadSkills(app); err != nil {
		t.Fatalf("reloadSkills(enabled pack) error = %v", err)
	}
	out.Reset()
	if err := runSkill(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runSkill(list enabled pack) error = %v", err)
	}
	if !strings.Contains(out.String(), "triage_workflow") ||
		!strings.Contains(out.String(), "source=domain-pack:triage_pack") {
		t.Fatalf("skill list = %q, want enabled domain-pack skill source", out.String())
	}
}

func TestRunDomainPackBuiltInTemplateListShowInstallDisabled(t *testing.T) {
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runDomainPack(app, []string{"templates"}, &out); err != nil {
		t.Fatalf("runDomainPack(templates) error = %v", err)
	}
	if !strings.Contains(out.String(), "personal_productivity") ||
		!strings.Contains(out.String(), "installed=false") ||
		!strings.Contains(out.String(), "enabled=false") ||
		!strings.Contains(out.String(), "source=domain_pack_template") ||
		!strings.Contains(out.String(), "path=packs/templates/personal_productivity") {
		t.Fatalf("templates output = %q, want built-in disabled personal_productivity template", out.String())
	}
	for _, ref := range workflows.BuiltInTemplateRefs() {
		if !strings.Contains(out.String(), ref.Name) || !strings.Contains(out.String(), "path="+ref.Path) {
			t.Fatalf("templates output = %q, missing built-in template %s at %s", out.String(), ref.Name, ref.Path)
		}
	}

	out.Reset()
	if err := runDomainPack(app, []string{"template", "show", "personal_productivity"}, &out); err != nil {
		t.Fatalf("runDomainPack(template show) error = %v", err)
	}
	if !strings.Contains(out.String(), "template: personal_productivity") ||
		!strings.Contains(out.String(), "installed: false") ||
		!strings.Contains(out.String(), "daily_planner") ||
		!strings.Contains(out.String(), "install: yemaka domain-pack install-template personal_productivity") {
		t.Fatalf("template show output = %q, want built-in workflow template details", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"install-template", "personal_productivity"}, &out); err != nil {
		t.Fatalf("runDomainPack(install-template) error = %v", err)
	}
	if !strings.Contains(out.String(), "domain pack installed: personal_productivity") ||
		!strings.Contains(out.String(), "template: personal_productivity") ||
		!strings.Contains(out.String(), "enabled: false") ||
		!strings.Contains(out.String(), "enable: yemaka domain-pack enable personal_productivity") {
		t.Fatalf("install-template output = %q, want disabled built-in template install", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"list"}, &out); err != nil {
		t.Fatalf("runDomainPack(list after template install) error = %v", err)
	}
	if !strings.Contains(out.String(), "personal_productivity") ||
		!strings.Contains(out.String(), "enabled=false") ||
		!strings.Contains(out.String(), "skills/daily_planner") {
		t.Fatalf("domain pack list = %q, want disabled installed personal_productivity", out.String())
	}

	if err := reloadSkills(app); err != nil {
		t.Fatalf("reloadSkills(disabled template pack) error = %v", err)
	}
	out.Reset()
	if err := runSkill(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runSkill(list disabled template pack) error = %v", err)
	}
	if strings.Contains(out.String(), "daily_planner") {
		t.Fatalf("skill list = %q, disabled template pack skill should not be loaded", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"enable", "personal_productivity"}, &out); err != nil {
		t.Fatalf("runDomainPack(enable template pack) error = %v", err)
	}
	if !strings.Contains(out.String(), "domain pack enabled: personal_productivity") {
		t.Fatalf("enable output = %q, want enabled template pack", out.String())
	}
	if err := reloadSkills(app); err != nil {
		t.Fatalf("reloadSkills(enabled template pack) error = %v", err)
	}
	out.Reset()
	if err := runSkill(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runSkill(list enabled template pack) error = %v", err)
	}
	if !strings.Contains(out.String(), "daily_planner") ||
		!strings.Contains(out.String(), "source=domain-pack:personal_productivity") {
		t.Fatalf("skill list = %q, want enabled built-in template pack skill source", out.String())
	}
}

func writeCLIDomainPack(t *testing.T, dir string, manifest string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir domain pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pack.yaml"), []byte(strings.TrimSpace(manifest)+"\n"), 0o644); err != nil {
		t.Fatalf("write domain pack manifest: %v", err)
	}
}

func writeCLISkillPackage(t *testing.T, dir string, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill package: %v", err)
	}
	meta := `name: ` + name + `
version: 0.1.0
description: Triage workflow.
triggers:
  - triage
required_tools:
  - rag_search
context_budget:
  max_instruction_chars: 800
`
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(meta), 0o644); err != nil {
		t.Fatalf("write skill.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("Use local context to triage the request.\n"), 0o644); err != nil {
		t.Fatalf("write instructions.md: %v", err)
	}
}
