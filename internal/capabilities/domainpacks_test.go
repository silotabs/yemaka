package capabilities

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
	"yemaka/internal/workflows"
)

func TestDomainPackCapabilityEnabledReadyWithMappedFields(t *testing.T) {
	status := domainpacks.PackStatus{
		Name:          "research_pack",
		Version:       "1.2.3",
		Description:   "Research workflow pack.",
		Category:      "research",
		Enabled:       true,
		Installed:     true,
		Valid:         true,
		RequiredTools: []string{"rag_search"},
		OptionalTools: []string{"memory_search"},
		Dir:           "/tmp/research_pack",
	}
	manifest := domainpacks.Manifest{
		Name:          "research_pack",
		Version:       "1.2.3",
		Description:   "Research workflow pack.",
		Category:      "research",
		RequiredTools: []string{"rag_search"},
		OptionalTools: []string{"memory_search"},
		Permissions: domainpacks.Permissions{
			Internet: domainpacks.PermissionDeclaration{
				Required:       true,
				Scopes:         []string{"search"},
				AllowedDomains: []string{"example.com"},
				AllowedMethods: []string{"GET", "HEAD"},
			},
			Filesystem: domainpacks.PermissionDeclaration{
				Paths: []string{"workspace://notes"},
			},
			Scheduler: domainpacks.PermissionDeclaration{
				Names: []string{"manual_review"},
			},
		},
		Safety: domainpacks.Safety{
			ApprovalRequiredFor: []string{"internet"},
		},
		DefaultState: domainpacks.DefaultStateDisabled,
	}

	capability, err := DomainPackCapability(status, manifest)
	if err != nil {
		t.Fatalf("DomainPackCapability() error = %v", err)
	}
	if capability.State != StateReady || !capability.Enabled || !capability.Available || !capability.Configured {
		t.Fatalf("capability state = %+v, want ready enabled capability", capability)
	}
	if !capability.RequiresApproval {
		t.Fatal("RequiresApproval = false, want true for required permission/approval safety")
	}
	for _, want := range []string{"domain_pack", "research", "permission:internet", "permission:filesystem", "permission:scheduler"} {
		if !containsString(capability.Tags, want) {
			t.Fatalf("Tags = %#v, missing %q", capability.Tags, want)
		}
	}
	for _, want := range []string{"research_pack", "research", "tool:rag_search", "tool:memory_search", "internet", "internet:domain:example.com", "internet:method:GET", "filesystem:path:workspace://notes", "scheduler:name:manual_review"} {
		if !containsString(capability.Provides, want) {
			t.Fatalf("Provides = %#v, missing %q", capability.Provides, want)
		}
	}
	if capability.Metadata["version"] != "1.2.3" || capability.Metadata["installed"] != "true" || capability.Metadata["valid"] != "true" {
		t.Fatalf("Metadata = %#v, want version/install/valid status", capability.Metadata)
	}
	if !strings.Contains(capability.Metadata["permissions"], "permission:internet") {
		t.Fatalf("Metadata permissions = %q, want permission summary", capability.Metadata["permissions"])
	}
}

func TestDomainPackCapabilityDisabledCannotLookReady(t *testing.T) {
	status := domainpacks.PackStatus{
		Name:      "writing_pack",
		Version:   "0.1.0",
		Category:  "writing",
		Enabled:   false,
		Installed: true,
		Valid:     true,
	}
	manifest := domainpacks.Manifest{
		Name:         "writing_pack",
		Version:      "0.1.0",
		Category:     "writing",
		DefaultState: domainpacks.DefaultStateEnabled,
	}

	capability, err := DomainPackCapability(status, manifest)
	if err != nil {
		t.Fatalf("DomainPackCapability() error = %v", err)
	}
	if capability.State != StateDisabled || capability.Enabled || !capability.Available || !capability.Configured {
		t.Fatalf("capability = %+v, want disabled inactive pack despite enabled default metadata", capability)
	}
	if capability.Metadata["default_state"] != domainpacks.DefaultStateEnabled {
		t.Fatalf("default_state metadata = %q, want enabled metadata preserved without auto-enabling", capability.Metadata["default_state"])
	}

	var registry Registry
	if err := registry.AddDomainPackStatus(status, manifest); err != nil {
		t.Fatalf("AddDomainPackStatus() error = %v", err)
	}
	decision := registry.ClassifyGap(GapQuery{Request: "Use the writing pack", Kind: KindDomainPack})
	if decision.Action != ActionAskConfig || decision.State != StateDisabled {
		t.Fatalf("decision = %+v, want ask_config for disabled domain pack", decision)
	}
	if !strings.Contains(decision.SuggestedAction, "Enable the domain pack explicitly") {
		t.Fatalf("SuggestedAction = %q, want explicit enable guidance", decision.SuggestedAction)
	}
}

func TestDomainPackCapabilityRiskyToolsRequireApprovalWithoutSafetyCopy(t *testing.T) {
	status := domainpacks.PackStatus{
		Name:      "web_research_pack",
		Version:   "0.1.0",
		Category:  "research",
		Enabled:   true,
		Installed: true,
		Valid:     true,
	}
	manifest := domainpacks.Manifest{
		Name:          "web_research_pack",
		Version:       "0.1.0",
		Category:      "research",
		RequiredTools: []string{"rag_search"},
		OptionalTools: []string{"internet_search"},
		DefaultState:  domainpacks.DefaultStateDisabled,
	}

	capability, err := DomainPackCapability(status, manifest)
	if err != nil {
		t.Fatalf("DomainPackCapability() error = %v", err)
	}
	if !capability.RequiresApproval {
		t.Fatalf("RequiresApproval = false, want approval metadata for policy-disabled pack tool")
	}
	if !containsString(capability.Tags, "requires_approval") {
		t.Fatalf("Tags = %#v, want requires_approval", capability.Tags)
	}

	var registry Registry
	if err := registry.AddDomainPack(capability); err != nil {
		t.Fatalf("AddDomainPack() error = %v", err)
	}
	decision := registry.ClassifyGap(GapQuery{Request: "Use web research pack", Kind: KindDomainPack})
	if decision.Action != ActionUsePack || !decision.RequiresApproval {
		t.Fatalf("decision = %+v, want approval-gated ready pack", decision)
	}
	if decision.SuggestedAction != "Use the existing capability through the normal policy and approval flow." {
		t.Fatalf("SuggestedAction = %q, want policy approval guidance", decision.SuggestedAction)
	}
}

func TestDomainPackCapabilityPackSkillRiskyToolsRequireApproval(t *testing.T) {
	status := domainpacks.PackStatus{
		Name:      "deploy_pack",
		Version:   "0.1.0",
		Category:  "devops",
		Enabled:   true,
		Installed: true,
		Valid:     true,
	}
	manifest := domainpacks.Manifest{
		Name:         "deploy_pack",
		Version:      "0.1.0",
		Category:     "devops",
		Skills:       []string{"skills/deploy_workflow"},
		DefaultState: domainpacks.DefaultStateDisabled,
	}
	packSkills := []skills.Skill{{
		Name:          "deploy_workflow",
		Version:       "0.1.0",
		Description:   "Run a local deployment workflow.",
		Triggers:      []string{"deploy local app"},
		RequiredTools: []string{"read_file", "run_shell_safe"},
	}}

	capability, err := DomainPackCapabilityWithSkills(status, manifest, packSkills)
	if err != nil {
		t.Fatalf("DomainPackCapabilityWithSkills() error = %v", err)
	}
	if !capability.RequiresApproval {
		t.Fatalf("RequiresApproval = false, want approval metadata for approval-required pack-skill tool")
	}
	metadata := append([]string{}, capability.Tags...)
	metadata = append(metadata, capability.Provides...)
	for _, want := range []string{"requires_approval", "tool:run_shell_safe", "skill:deploy_workflow"} {
		if !containsString(metadata, want) {
			t.Fatalf("capability tags/provides missing %q: tags=%#v provides=%#v", want, capability.Tags, capability.Provides)
		}
	}
}

func TestDomainPackTemplateRiskyToolsRequireApprovalWithoutPermissions(t *testing.T) {
	capability, err := DomainPackTemplateCapability(workflows.Template{
		Name:          "web_template",
		Version:       "0.1.0",
		Description:   "Template that can use a web-backed workflow once installed.",
		Category:      "research",
		DefaultState:  workflows.DefaultStateDisabled,
		RequiredTools: []string{"internet_search"},
		Workflows: []workflows.Workflow{{
			Name:          "web_brief",
			Version:       "0.1.0",
			Description:   "Create a brief from approved sources.",
			SkillName:     "web_brief",
			Intent:        workflows.IntentAction,
			RunMode:       workflows.RunModeManual,
			DefaultState:  workflows.DefaultStateDisabled,
			Triggers:      []string{"create web brief"},
			RequiredTools: []string{"rag_search"},
		}},
	})
	if err != nil {
		t.Fatalf("DomainPackTemplateCapability() error = %v", err)
	}
	if capability.State != StateNeedsConfig || !capability.RequiresApproval {
		t.Fatalf("capability = %+v, want setup-only approval-gated template", capability)
	}
	if !containsString(capability.Tags, "requires_approval") {
		t.Fatalf("Tags = %#v, want requires_approval", capability.Tags)
	}
}

func TestDomainPackHandoffSuggestedActionsByState(t *testing.T) {
	tests := []struct {
		name          string
		capability    Capability
		request       string
		wantAction    GapAction
		wantState     State
		wantSuggested string
	}{
		{
			name: "ready installed pack uses pack",
			capability: mustDomainPackCapability(t,
				domainpacks.PackStatus{Name: "ready_pack", Category: "research", Enabled: true, Installed: true, Valid: true},
				domainpacks.Manifest{Name: "ready_pack", Version: "0.1.0", Category: "research", DefaultState: domainpacks.DefaultStateDisabled},
			),
			request:       "Use the ready pack for research",
			wantAction:    ActionUsePack,
			wantState:     StateReady,
			wantSuggested: "Use the enabled pack instead of generating new trusted-core code.",
		},
		{
			name: "disabled installed pack asks enable",
			capability: mustDomainPackCapability(t,
				domainpacks.PackStatus{Name: "disabled_pack", Category: "writing", Enabled: false, Installed: true, Valid: true},
				domainpacks.Manifest{Name: "disabled_pack", Version: "0.1.0", Category: "writing", DefaultState: domainpacks.DefaultStateDisabled},
			),
			request:       "Use the disabled pack for writing",
			wantAction:    ActionAskConfig,
			wantState:     StateDisabled,
			wantSuggested: "Enable the domain pack explicitly before routing work to it.",
		},
		{
			name: "template pack asks install",
			capability: mustDomainPackTemplateCapability(t, workflows.Template{
				Name:         "template_pack",
				Version:      "0.1.0",
				Description:  "Template pack.",
				Category:     "productivity",
				DefaultState: domainpacks.DefaultStateDisabled,
				Workflows: []workflows.Workflow{
					{
						Name:         "template_workflow",
						Version:      "0.1.0",
						Description:  "Template workflow.",
						SkillName:    "template_skill",
						DefaultState: workflows.DefaultStateDisabled,
						Triggers:     []string{"template task"},
					},
				},
			}),
			request:       "Run the template task",
			wantAction:    ActionAskConfig,
			wantState:     StateNeedsConfig,
			wantSuggested: "Install the local workflow pack template, then enable it explicitly before routing work to it.",
		},
		{
			name: "unavailable invalid pack asks repair",
			capability: mustDomainPackCapability(t,
				domainpacks.PackStatus{Name: "broken_pack", Category: "ops", Installed: true, Valid: false, ValidationError: "bad manifest"},
				domainpacks.Manifest{Name: "broken_pack", Category: "ops", DefaultState: domainpacks.DefaultStateDisabled},
			),
			request:       "Use the broken pack for ops",
			wantAction:    ActionUnsupported,
			wantState:     StateUnavailable,
			wantSuggested: "Fix the domain pack manifest before enabling or routing work to it.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var registry Registry
			if err := registry.AddDomainPack(tt.capability); err != nil {
				t.Fatalf("AddDomainPack() error = %v", err)
			}
			decision := registry.ClassifyGap(GapQuery{Request: tt.request, Kind: KindDomainPack})
			if decision.Action != tt.wantAction || decision.State != tt.wantState || decision.Name != tt.capability.Name {
				t.Fatalf("decision = %+v, want action=%q state=%q name=%q", decision, tt.wantAction, tt.wantState, tt.capability.Name)
			}
			if decision.SuggestedAction != tt.wantSuggested {
				t.Fatalf("SuggestedAction = %q, want %q", decision.SuggestedAction, tt.wantSuggested)
			}
			if decision.Capability == nil || decision.Capability.State != tt.wantState {
				t.Fatalf("decision.Capability = %+v, want matched capability with state %q", decision.Capability, tt.wantState)
			}
		})
	}
}

func TestDomainPackCapabilityRejectsProtectedPolicyOverrideReadiness(t *testing.T) {
	status := domainpacks.PackStatus{
		Name:      "trading_pack",
		Version:   "0.1.0",
		Enabled:   true,
		Installed: true,
		Valid:     true,
	}
	manifest := domainpacks.Manifest{
		Name:    "trading_pack",
		Version: "0.1.0",
		Safety: domainpacks.Safety{
			PolicyOverrides: []string{"trading_policy"},
		},
		DefaultState: domainpacks.DefaultStateDisabled,
	}

	capability, err := DomainPackCapability(status, manifest)
	if err != nil {
		t.Fatalf("DomainPackCapability() error = %v", err)
	}
	if capability.State == StateReady || capability.Enabled || capability.Available {
		t.Fatalf("capability = %+v, want invalid protected-policy override to be unavailable", capability)
	}
	if !strings.Contains(capability.Reason, "protected policy") {
		t.Fatalf("Reason = %q, want protected policy validation failure", capability.Reason)
	}
}

func TestDomainPackCapabilitiesSafeDefaultsAndSort(t *testing.T) {
	statuses := []domainpacks.PackStatus{
		{Name: "z_pack", Installed: true, Valid: false, ValidationError: "bad manifest"},
		{Name: "a_pack", Installed: false, Valid: false},
	}
	capabilities, err := DomainPackCapabilities(statuses, nil)
	if err != nil {
		t.Fatalf("DomainPackCapabilities() error = %v", err)
	}
	if len(capabilities) != 2 {
		t.Fatalf("len(capabilities) = %d, want 2", len(capabilities))
	}
	if capabilities[0].Name != "a_pack" || capabilities[1].Name != "z_pack" {
		t.Fatalf("capability order = %#v, want deterministic name sort", []string{capabilities[0].Name, capabilities[1].Name})
	}
	for _, capability := range capabilities {
		if capability.State == StateReady || capability.Enabled {
			t.Fatalf("capability = %+v, want safe default not ready", capability)
		}
	}
}

func TestDomainPackTemplateCapabilityNeedsInstallAndMatchesWorkflowTriggers(t *testing.T) {
	catalog, err := workflows.NewBuiltInTemplateCatalog(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("NewBuiltInTemplateCatalog() error = %v", err)
	}
	template, ok := catalog.Get("personal_productivity")
	if !ok {
		t.Fatal("personal_productivity template missing from built-in catalog")
	}

	capability, err := DomainPackTemplateCapability(template)
	if err != nil {
		t.Fatalf("DomainPackTemplateCapability() error = %v", err)
	}
	if capability.State != StateNeedsConfig || capability.Enabled || capability.Configured || !capability.Available {
		t.Fatalf("capability = %+v, want setup-only template capability", capability)
	}
	if !capability.RequiresApproval {
		t.Fatal("RequiresApproval = false, want approval for memory-write workflow template")
	}
	if capability.Metadata["template"] != "true" {
		t.Fatalf("Metadata = %#v, want template marker", capability.Metadata)
	}
	for _, want := range []string{"domain_pack_template", "productivity", "requires_approval"} {
		if !containsString(capability.Tags, want) {
			t.Fatalf("Tags = %#v, missing %q", capability.Tags, want)
		}
	}
	for _, want := range []string{"plan my day", "capture follow ups", "workflow:personal_productivity_daily_planner", "skill:daily_planner", "tool:memory_write"} {
		if !containsString(capability.Provides, want) {
			t.Fatalf("Provides = %#v, missing %q", capability.Provides, want)
		}
	}

	var registry Registry
	if err := registry.AddDomainPack(capability); err != nil {
		t.Fatalf("AddDomainPack() error = %v", err)
	}
	decision := registry.ClassifyGap(GapQuery{Request: "Plan my day and save follow-up tasks", Kind: KindDomainPack})
	if decision.Action != ActionAskConfig || decision.State != StateNeedsConfig {
		t.Fatalf("decision = %+v, want ask_config/needs_config for workflow template", decision)
	}
	if decision.Name != "personal_productivity" {
		t.Fatalf("decision.Name = %q, want personal_productivity", decision.Name)
	}
	if decision.Capability == nil || decision.Capability.Metadata["template"] != "true" {
		t.Fatalf("decision.Capability = %+v, want template metadata", decision.Capability)
	}
}

func TestLocalFileTriageTemplateCapabilityIsReadOnlyAndDiscoverable(t *testing.T) {
	catalog, err := workflows.NewBuiltInTemplateCatalog(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("NewBuiltInTemplateCatalog() error = %v", err)
	}
	template, ok := catalog.Get("local_file_triage")
	if !ok {
		t.Fatal("local_file_triage template missing from built-in catalog")
	}

	capability, err := DomainPackTemplateCapability(template)
	if err != nil {
		t.Fatalf("DomainPackTemplateCapability() error = %v", err)
	}
	if capability.State != StateNeedsConfig || capability.Enabled || capability.Configured || !capability.Available {
		t.Fatalf("capability = %+v, want setup-only template capability", capability)
	}
	if capability.RequiresApproval {
		t.Fatalf("RequiresApproval = true, want false for read-only local tools; capability=%+v", capability)
	}
	for _, want := range []string{"domain_pack_template", "workspace"} {
		if !containsString(capability.Tags, want) {
			t.Fatalf("Tags = %#v, missing %q", capability.Tags, want)
		}
	}
	for _, want := range []string{"triage workspace files", "propose file cleanup", "workflow:local_file_triage_file_inventory", "skill:file_inventory", "tool:file_tree", "tool:search_files"} {
		if !containsString(capability.Provides, want) {
			t.Fatalf("Provides = %#v, missing %q", capability.Provides, want)
		}
	}
}

func TestDomainPackCapabilitiesFromRootIncludesSkillTriggers(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	writeCapabilityDomainPack(t, source, `
name: daily_pack
version: 0.1.0
description: Daily planning workflows.
category: productivity
skills:
  - skills/daily_plan
required_tools:
  - memory_search
default_state: disabled
`)
	writeCapabilitySkill(t, filepath.Join(source, "skills", "daily_plan"), "daily_plan", "Plan a local day from memory.", []string{"plan my day", "daily plan"})

	status, err := domainpacks.Install(root, source)
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if _, err := domainpacks.SetEnabled(root, status.Name, true); err != nil {
		t.Fatalf("SetEnabled() error = %v", err)
	}

	capabilities, err := DomainPackCapabilitiesFromRoot(root)
	if err != nil {
		t.Fatalf("DomainPackCapabilitiesFromRoot() error = %v", err)
	}
	var pack Capability
	for _, capability := range capabilities {
		if capability.Name == "daily_pack" {
			pack = capability
			break
		}
	}
	if pack.Name == "" {
		t.Fatalf("capabilities = %#v, missing daily_pack", capabilities)
	}
	if pack.State != StateReady {
		t.Fatalf("pack.State = %q, want ready", pack.State)
	}
	for _, want := range []string{"plan my day", "trigger:daily plan", "skill:daily_plan"} {
		if !containsString(pack.Provides, want) {
			t.Fatalf("Provides = %#v, missing %q", pack.Provides, want)
		}
	}

	var registry Registry
	if err := registry.AddDomainPack(pack); err != nil {
		t.Fatalf("AddDomainPack() error = %v", err)
	}
	decision := registry.ClassifyGap(GapQuery{Request: "Plan my day", Kind: KindDomainPack})
	if decision.Action != ActionUsePack || decision.State != StateReady {
		t.Fatalf("decision = %+v, want use_pack/ready from installed pack skill trigger", decision)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func writeCapabilityDomainPack(t *testing.T, dir string, manifest string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir domain pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, domainpacks.ManifestFile), []byte(strings.TrimSpace(manifest)+"\n"), 0o644); err != nil {
		t.Fatalf("write domain pack manifest: %v", err)
	}
}

func writeCapabilitySkill(t *testing.T, dir string, name string, description string, triggers []string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir skill: %v", err)
	}
	triggerLines := ""
	for _, trigger := range triggers {
		triggerLines += "  - " + trigger + "\n"
	}
	meta := "name: " + name + `
version: 0.1.0
description: ` + description + `
triggers:
` + triggerLines + `required_tools:
  - memory_search
context_budget:
  max_instruction_chars: 800
`
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(meta), 0o644); err != nil {
		t.Fatalf("write skill.yaml: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "instructions.md"), []byte("Use local context only.\n"), 0o644); err != nil {
		t.Fatalf("write instructions.md: %v", err)
	}
}

func mustDomainPackCapability(t *testing.T, status domainpacks.PackStatus, manifest domainpacks.Manifest) Capability {
	t.Helper()
	capability, err := DomainPackCapability(status, manifest)
	if err != nil {
		t.Fatalf("DomainPackCapability() error = %v", err)
	}
	return capability
}

func mustDomainPackTemplateCapability(t *testing.T, template workflows.Template) Capability {
	t.Helper()
	capability, err := DomainPackTemplateCapability(template)
	if err != nil {
		t.Fatalf("DomainPackTemplateCapability() error = %v", err)
	}
	return capability
}
