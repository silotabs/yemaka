package desktop

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/domainpacks"
	"yemaka/internal/memory"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/skills"
	"yemaka/internal/workflows"
)

func TestDomainPackDesktopLifecycleReloadsSkills(t *testing.T) {
	chdirRepoRoot(t)
	app := newDomainPackDesktopTestApp(t)

	initialPacks, err := app.ListDomainPacks()
	if err != nil {
		t.Fatalf("ListDomainPacks() initial error = %v", err)
	}
	if len(initialPacks) != 0 {
		t.Fatalf("ListDomainPacks() initial = %#v, want none", initialPacks)
	}

	sourcePack := filepath.Join(t.TempDir(), "desktop_pack")
	if _, err := domainpacks.WriteWorkflowSkillPack(domainpacks.WorkflowSkillPackInput{
		PackDir:     sourcePack,
		Name:        "desktop_pack",
		Version:     "0.1.0",
		Description: "Desktop domain pack lifecycle test.",
		Category:    "capability",
		Workflows: []skills.WorkflowCompileInput{{
			Name:        "desktop_pack_skill",
			Description: "Exercise desktop domain pack skill reloads.",
			Triggers:    []string{"desktop pack lifecycle"},
			Steps: []skills.WorkflowStep{
				{Instruction: "Search local memory for relevant context.", Tool: "memory_search"},
			},
			ExamplePrompts: []string{"Use the desktop pack lifecycle workflow."},
		}},
	}); err != nil {
		t.Fatalf("WriteWorkflowSkillPack() error = %v", err)
	}

	installed, err := app.InstallDomainPack(sourcePack)
	if err != nil {
		t.Fatalf("InstallDomainPack() error = %v", err)
	}
	if installed.Enabled {
		t.Fatal("InstallDomainPack() enabled pack, want disabled until explicit enable")
	}
	if installed.Dir != filepath.Join(app.profile.Root, "domain_packs", "desktop_pack") {
		t.Fatalf("installed pack Dir = %q, want profile domain_packs root", installed.Dir)
	}
	assertListedDomainPack(t, app, false)
	if _, ok := app.skills.Get("desktop_pack_skill"); ok {
		t.Fatal("disabled domain pack skill is visible before enable")
	}
	status := domainPackSkillStatus(t, app, false)
	if status.Enabled || status.Active {
		t.Fatalf("disabled pack skill status = %#v, want inactive", status)
	}

	enabled, err := app.SetDomainPackEnabled("desktop_pack", true)
	if err != nil {
		t.Fatalf("SetDomainPackEnabled(true) error = %v", err)
	}
	if !enabled.Enabled {
		t.Fatal("SetDomainPackEnabled(true) returned disabled status")
	}
	if _, ok := app.skills.Get("desktop_pack_skill"); !ok {
		t.Fatal("enabled domain pack skill was not reloaded into desktop registry")
	}
	assertListedDomainPack(t, app, true)
	status = domainPackSkillStatus(t, app, true)
	if !status.Enabled || !status.Active {
		t.Fatalf("enabled pack skill status = %#v, want active", status)
	}

	disabled, err := app.SetDomainPackEnabled("desktop_pack", false)
	if err != nil {
		t.Fatalf("SetDomainPackEnabled(false) error = %v", err)
	}
	if disabled.Enabled {
		t.Fatal("SetDomainPackEnabled(false) returned enabled status")
	}
	if _, ok := app.skills.Get("desktop_pack_skill"); ok {
		t.Fatal("disabled domain pack skill remained in desktop registry after reload")
	}
	assertListedDomainPack(t, app, false)

	removed, err := app.UninstallDomainPack("desktop_pack")
	if err != nil {
		t.Fatalf("UninstallDomainPack() error = %v", err)
	}
	if removed.Name != "desktop_pack" || removed.Installed || removed.Enabled {
		t.Fatalf("UninstallDomainPack() = %#v, want uninstalled disabled pack", removed)
	}
	if _, err := os.Stat(installed.Dir); !os.IsNotExist(err) {
		t.Fatalf("installed pack dir still exists or stat failed unexpectedly: %v", err)
	}
	packsAfterRemove, err := app.ListDomainPacks()
	if err != nil {
		t.Fatalf("ListDomainPacks() after uninstall error = %v", err)
	}
	if len(packsAfterRemove) != 0 {
		t.Fatalf("ListDomainPacks() after uninstall = %#v, want none", packsAfterRemove)
	}
	if _, ok := app.skills.Get("desktop_pack_skill"); ok {
		t.Fatal("uninstalled domain pack skill remained in desktop registry")
	}
}

func TestApplyCapabilityInstallsBuiltInDomainPackTemplate(t *testing.T) {
	chdirRepoRoot(t)
	app := newDomainPackDesktopTestApp(t)

	result, err := app.ApplyCapability(CapabilityApplyInput{
		Approved:   true,
		Kind:       "domain_pack",
		Name:       "personal_productivity",
		PackSource: "domain_pack_template",
	})
	if err != nil {
		t.Fatalf("ApplyCapability() template error = %v", err)
	}
	if result.Action != "installed" {
		t.Fatalf("ApplyCapability() action = %q, want installed", result.Action)
	}
	if result.Pack.Name != "personal_productivity" || !result.Pack.Valid {
		t.Fatalf("ApplyCapability() pack = %#v, want valid personal_productivity", result.Pack)
	}
	if result.Pack.Enabled {
		t.Fatal("ApplyCapability() enabled template pack, want disabled until explicit enable")
	}
	if _, ok := app.skills.Get("daily_planner"); ok {
		t.Fatal("disabled template pack skill is visible before enable")
	}
}

func TestDomainPackDesktopTemplateLifecycleMatrix(t *testing.T) {
	chdirRepoRoot(t)
	app := newDomainPackDesktopTestApp(t)

	templates, err := app.ListDomainPackTemplates()
	if err != nil {
		t.Fatalf("ListDomainPackTemplates() error = %v", err)
	}
	if len(templates) == 0 {
		t.Fatal("ListDomainPackTemplates() = empty, want built-in templates")
	}
	for _, template := range templates {
		assertDesktopDomainPackTemplateSummary(t, template, false)

		installed, err := app.InstallDomainPackTemplate(template.Name)
		if err != nil {
			t.Fatalf("InstallDomainPackTemplate(%s) error = %v", template.Name, err)
		}
		if installed.Name != template.Name || !installed.Valid || installed.Enabled {
			t.Fatalf("InstallDomainPackTemplate(%s) = %#v, want valid disabled pack", template.Name, installed)
		}
		assertDesktopTemplateSkillsLoaded(t, app, template, false)
		assertDesktopDomainPackSkillSummaries(t, app, template, false)

		templatesAfterInstall, err := app.ListDomainPackTemplates()
		if err != nil {
			t.Fatalf("ListDomainPackTemplates() after install error = %v", err)
		}
		installedTemplate, ok := findDesktopDomainPackTemplateForTest(templatesAfterInstall, template.Name)
		if !ok || !installedTemplate.Installed || installedTemplate.Enabled || !installedTemplate.Valid {
			t.Fatalf("ListDomainPackTemplates() after install = %#v, want %s installed disabled valid", templatesAfterInstall, template.Name)
		}

		enabled, err := app.SetDomainPackEnabled(template.Name, true)
		if err != nil {
			t.Fatalf("SetDomainPackEnabled(%s, true) error = %v", template.Name, err)
		}
		if !enabled.Enabled {
			t.Fatalf("SetDomainPackEnabled(%s, true) = %#v, want enabled", template.Name, enabled)
		}
		assertDesktopTemplateSkillsLoaded(t, app, template, true)
		assertDesktopDomainPackSkillSummaries(t, app, template, true)

		disabled, err := app.SetDomainPackEnabled(template.Name, false)
		if err != nil {
			t.Fatalf("SetDomainPackEnabled(%s, false) error = %v", template.Name, err)
		}
		if disabled.Enabled {
			t.Fatalf("SetDomainPackEnabled(%s, false) = %#v, want disabled", template.Name, disabled)
		}
		assertDesktopTemplateSkillsLoaded(t, app, template, false)
	}
}

func TestDomainPackDesktopInvalidPackRepairHint(t *testing.T) {
	app := newDomainPackDesktopTestApp(t)

	brokenDir := filepath.Join(app.domainPackRoot(), "broken_pack")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("mkdir invalid pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, domainpacks.ManifestFile), []byte("name: Broken Pack\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write invalid pack manifest: %v", err)
	}

	packs, err := app.ListDomainPacks()
	if err != nil {
		t.Fatalf("ListDomainPacks() error = %v", err)
	}
	if len(packs) != 1 {
		t.Fatalf("ListDomainPacks() = %#v, want one invalid pack", packs)
	}
	if packs[0].Name != "broken_pack" || packs[0].Valid || packs[0].Enabled || packs[0].ValidationError == "" || packs[0].RepairHint == "" {
		t.Fatalf("invalid pack status = %#v, want invalid disabled pack with repair guidance", packs[0])
	}
	if _, err := app.SetDomainPackEnabled("broken_pack", true); err == nil {
		t.Fatal("SetDomainPackEnabled(invalid pack) succeeded, want error")
	}
}

func TestApplyCapabilityEnablesInstalledDomainPack(t *testing.T) {
	chdirRepoRoot(t)
	app := newDomainPackDesktopTestApp(t)
	sourcePack := writeDesktopDomainPack(t, "desktop_pack", "desktop_pack_skill")

	if _, err := app.InstallDomainPack(sourcePack); err != nil {
		t.Fatalf("InstallDomainPack() error = %v", err)
	}
	if _, ok := app.skills.Get("desktop_pack_skill"); ok {
		t.Fatal("disabled domain pack skill is visible before ApplyCapability enable")
	}

	result, err := app.ApplyCapability(CapabilityApplyInput{
		Approved:           true,
		Kind:               "domain_pack",
		ExistingCapability: "desktop_pack",
		PackSource:         "domain_pack",
	})
	if err != nil {
		t.Fatalf("ApplyCapability() enable error = %v", err)
	}
	if result.Action != "enabled" || !result.Pack.Enabled {
		t.Fatalf("ApplyCapability() result = %#v, want enabled action and pack", result)
	}
	if _, ok := app.skills.Get("desktop_pack_skill"); !ok {
		t.Fatal("enabled domain pack skill was not reloaded into desktop registry")
	}
}

func TestApplyCapabilityRequiresApproval(t *testing.T) {
	app := newDomainPackDesktopTestApp(t)

	_, err := app.ApplyCapability(CapabilityApplyInput{
		Kind:       "domain_pack",
		Name:       "personal_productivity",
		PackSource: "domain_pack_template",
	})
	if err == nil {
		t.Fatal("ApplyCapability() without approval succeeded, want error")
	}
}

func newDomainPackDesktopTestApp(t *testing.T) *App {
	t.Helper()

	root := t.TempDir()
	profileSkills := filepath.Join(root, "skills")
	if err := os.MkdirAll(profileSkills, 0o755); err != nil {
		t.Fatalf("create profile skills dir: %v", err)
	}
	dbPath := filepath.Join(root, "memory.sqlite")
	store, err := memory.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	ragStore, err := rag.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = ragStore.Close()
	})

	profile := &profiles.Profile{
		Root:        root,
		Database:    dbPath,
		Skills:      profileSkills,
		Logs:        filepath.Join(root, "logs"),
		Permissions: filepath.Join(root, "permissions"),
	}
	app := &App{
		ctx:     context.Background(),
		config:  config.Default(),
		profile: profile,
		store:   store,
		rag:     ragStore,
	}
	registry, err := loadSkillRegistryForProfile(profile)
	if err != nil {
		t.Fatalf("loadSkillRegistryForProfile() error = %v", err)
	}
	app.skills = registry
	return app
}

func writeDesktopDomainPack(t *testing.T, name string, skillName string) string {
	t.Helper()
	sourcePack := filepath.Join(t.TempDir(), name)
	if _, err := domainpacks.WriteWorkflowSkillPack(domainpacks.WorkflowSkillPackInput{
		PackDir:     sourcePack,
		Name:        name,
		Version:     "0.1.0",
		Description: "Desktop domain pack lifecycle test.",
		Category:    "capability",
		Workflows: []skills.WorkflowCompileInput{{
			Name:        skillName,
			Description: "Exercise desktop domain pack skill reloads.",
			Triggers:    []string{"desktop pack lifecycle"},
			Steps: []skills.WorkflowStep{
				{Instruction: "Search local memory for relevant context.", Tool: "memory_search"},
			},
			ExamplePrompts: []string{"Use the desktop pack lifecycle workflow."},
		}},
	}); err != nil {
		t.Fatalf("WriteWorkflowSkillPack() error = %v", err)
	}
	return sourcePack
}

func domainPackSkillStatus(t *testing.T, app *App, wantActive bool) DomainPackSkillStatus {
	t.Helper()
	statuses, err := app.ListDomainPackSkills()
	if err != nil {
		t.Fatalf("ListDomainPackSkills() error = %v", err)
	}
	for _, status := range statuses {
		if status.PackName == "desktop_pack" && status.Ref == "skills/desktop_pack_skill" && status.Active == wantActive {
			return status
		}
	}
	t.Fatalf("ListDomainPackSkills() = %#v, missing desktop_pack status active=%v", statuses, wantActive)
	return DomainPackSkillStatus{}
}

func assertListedDomainPack(t *testing.T, app *App, wantEnabled bool) {
	t.Helper()
	packs, err := app.ListDomainPacks()
	if err != nil {
		t.Fatalf("ListDomainPacks() error = %v", err)
	}
	if len(packs) != 1 {
		t.Fatalf("ListDomainPacks() = %#v, want one desktop_pack", packs)
	}
	if packs[0].Name != "desktop_pack" || packs[0].Enabled != wantEnabled || !packs[0].Valid {
		t.Fatalf("ListDomainPacks() = %#v, want desktop_pack enabled=%v valid=true", packs, wantEnabled)
	}
}

func findDesktopDomainPackTemplateForTest(templates []workflows.TemplateSummary, name string) (workflows.TemplateSummary, bool) {
	for _, template := range templates {
		if template.Name == name {
			return template, true
		}
	}
	return workflows.TemplateSummary{}, false
}

func assertDesktopDomainPackTemplateSummary(t *testing.T, template workflows.TemplateSummary, installed bool) {
	t.Helper()
	if template.Name == "" || template.Version == "" || template.Description == "" || template.Category == "" {
		t.Fatalf("template summary missing identity metadata: %#v", template)
	}
	if template.Source != workflows.SourceDomainPackTemplate || template.Path == "" || template.InstallAction != "install" {
		t.Fatalf("template source/path/action = %#v, want built-in installable template", template)
	}
	if template.Installed != installed || template.Enabled || !template.Valid || template.DefaultState != workflows.DefaultStateDisabled {
		t.Fatalf("template lifecycle = %#v, want installed=%v enabled=false valid=true default disabled", template, installed)
	}
	if len(template.RequiredTools) == 0 || len(template.Skills) == 0 || len(template.Workflows) == 0 || template.SafetySummary == "" {
		t.Fatalf("template summary missing tools/skills/workflows/safety metadata: %#v", template)
	}
}

func assertDesktopTemplateSkillsLoaded(t *testing.T, app *App, template workflows.TemplateSummary, loaded bool) {
	t.Helper()
	for _, skill := range template.Skills {
		_, ok := app.skills.Get(skill)
		if ok != loaded {
			t.Fatalf("skill %s from %s loaded=%v, want %v; statuses = %#v", skill, template.Name, ok, loaded, app.skills.Statuses())
		}
	}
}

func assertDesktopDomainPackSkillSummaries(t *testing.T, app *App, template workflows.TemplateSummary, active bool) {
	t.Helper()
	statuses, err := app.ListDomainPackSkills()
	if err != nil {
		t.Fatalf("ListDomainPackSkills() error = %v", err)
	}
	for _, skill := range template.Skills {
		wantRef := filepath.ToSlash(filepath.Join(domainpacks.PackSkillsDir, skill))
		found := false
		for _, status := range statuses {
			if status.PackName == template.Name && status.Ref == wantRef {
				found = true
				if status.Active != active || status.Enabled != active || !status.Valid {
					t.Fatalf("skill summary for %s/%s = %#v, want active=%v enabled=%v valid=true", template.Name, skill, status, active, active)
				}
				break
			}
		}
		if !found {
			t.Fatalf("ListDomainPackSkills() = %#v, missing %s/%s", statuses, template.Name, wantRef)
		}
	}
}

func chdirRepoRoot(t *testing.T) {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			t.Chdir(dir)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find repo root from %s", dir)
		}
		dir = parent
	}
}
