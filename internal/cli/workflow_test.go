package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunWorkflowListAndMatchEnabledTemplatePack(t *testing.T) {
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runDomainPack(app, []string{"install-template", "personal_productivity"}, &out); err != nil {
		t.Fatalf("runDomainPack(install-template) error = %v", err)
	}

	out.Reset()
	if err := runWorkflow(app, []string{"list"}, &out); err != nil {
		t.Fatalf("runWorkflow(list disabled) error = %v", err)
	}
	if strings.Contains(out.String(), "personal_productivity_daily_planner") {
		t.Fatalf("workflow list = %q, disabled template pack workflow should be excluded by default", out.String())
	}

	out.Reset()
	if err := runWorkflow(app, []string{"list", "--include-disabled"}, &out); err != nil {
		t.Fatalf("runWorkflow(list include disabled) error = %v", err)
	}
	if !strings.Contains(out.String(), "personal_productivity_daily_planner") || !strings.Contains(out.String(), "enabled=false") {
		t.Fatalf("workflow list = %q, want disabled template workflow when requested", out.String())
	}

	out.Reset()
	if err := runDomainPack(app, []string{"enable", "personal_productivity"}, &out); err != nil {
		t.Fatalf("runDomainPack(enable) error = %v", err)
	}

	out.Reset()
	if err := runWorkflow(app, []string{"list"}, &out); err != nil {
		t.Fatalf("runWorkflow(list enabled) error = %v", err)
	}
	if !strings.Contains(out.String(), "personal_productivity_daily_planner") || !strings.Contains(out.String(), "enabled=true") {
		t.Fatalf("workflow list = %q, want enabled template workflow", out.String())
	}

	out.Reset()
	if err := runWorkflow(app, []string{"match", "please build a daily plan"}, &out); err != nil {
		t.Fatalf("runWorkflow(match) error = %v", err)
	}
	if !strings.Contains(out.String(), "match: personal_productivity_daily_planner") {
		t.Fatalf("workflow match = %q, want enabled daily planner workflow", out.String())
	}
}

func TestRunWorkflowDisabledSkillExcludedByDefault(t *testing.T) {
	app := newExtensionTestApp(t)
	source := t.TempDir()
	writeCLIDomainPack(t, source, `
name: disabled_skill_pack
version: 0.1.0
description: Disabled skill helpers.
category: review
skills:
  - skills/triage_workflow
`)
	writeCLIWorkflowSkillPackage(t, filepath.Join(source, "skills", "triage_workflow"), "triage_workflow", true)

	var out bytes.Buffer
	if err := runDomainPack(app, []string{"install", source}, &out); err != nil {
		t.Fatalf("runDomainPack(install) error = %v", err)
	}
	out.Reset()
	if err := runDomainPack(app, []string{"enable", "disabled_skill_pack"}, &out); err != nil {
		t.Fatalf("runDomainPack(enable) error = %v", err)
	}

	out.Reset()
	if err := runWorkflow(app, []string{"list"}, &out); err != nil {
		t.Fatalf("runWorkflow(list) error = %v", err)
	}
	if strings.Contains(out.String(), "disabled_skill_pack_triage_workflow") {
		t.Fatalf("workflow list = %q, disabled skill workflow should be excluded by default", out.String())
	}

	out.Reset()
	if err := runWorkflow(app, []string{"match", "triage this request"}, &out); err != nil {
		t.Fatalf("runWorkflow(match) error = %v", err)
	}
	if !strings.Contains(out.String(), "no workflow match") {
		t.Fatalf("workflow match = %q, want no default match for disabled skill", out.String())
	}

	out.Reset()
	if err := runWorkflow(app, []string{"match", "triage this request", "--include-disabled"}, &out); err != nil {
		t.Fatalf("runWorkflow(match include disabled) error = %v", err)
	}
	if !strings.Contains(out.String(), "match: disabled_skill_pack_triage_workflow") || !strings.Contains(out.String(), "enabled=false") {
		t.Fatalf("workflow match = %q, want disabled workflow only when requested", out.String())
	}
}

func TestRunWorkflowJSONModeValid(t *testing.T) {
	app := newExtensionTestApp(t)
	source := t.TempDir()
	writeCLIDomainPack(t, source, `
name: json_pack
version: 0.1.0
description: JSON helpers.
category: review
skills:
  - skills/triage_workflow
`)
	writeCLISkillPackage(t, filepath.Join(source, "skills", "triage_workflow"), "triage_workflow")

	var out bytes.Buffer
	if err := runDomainPack(app, []string{"install", source}, &out); err != nil {
		t.Fatalf("runDomainPack(install) error = %v", err)
	}
	out.Reset()
	if err := runDomainPack(app, []string{"enable", "json_pack"}, &out); err != nil {
		t.Fatalf("runDomainPack(enable) error = %v", err)
	}

	out.Reset()
	if err := runWorkflow(app, []string{"list", "--json"}, &out); err != nil {
		t.Fatalf("runWorkflow(list json) error = %v", err)
	}
	var listed []map[string]any
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatalf("list json invalid: %v\n%s", err, out.String())
	}
	if len(listed) != 1 || listed[0]["name"] != "json_pack_triage_workflow" {
		t.Fatalf("list json = %#v, want json_pack_triage_workflow", listed)
	}

	out.Reset()
	if err := runWorkflow(app, []string{"match", "triage this request", "--json"}, &out); err != nil {
		t.Fatalf("runWorkflow(match json) error = %v", err)
	}
	var matched struct {
		Matched  bool `json:"matched"`
		Workflow struct {
			Name string `json:"name"`
		} `json:"workflow"`
	}
	if err := json.Unmarshal(out.Bytes(), &matched); err != nil {
		t.Fatalf("match json invalid: %v\n%s", err, out.String())
	}
	if !matched.Matched || matched.Workflow.Name != "json_pack_triage_workflow" {
		t.Fatalf("match json = %#v, want json_pack_triage_workflow", matched)
	}
}

func writeCLIWorkflowSkillPackage(t *testing.T, dir string, name string, disabled bool) {
	t.Helper()
	writeCLISkillPackage(t, dir, name)
	if !disabled {
		return
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
disabled: true
`
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(meta), 0o644); err != nil {
		t.Fatalf("write disabled skill.yaml: %v", err)
	}
}
