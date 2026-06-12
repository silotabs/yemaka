package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
	"yemaka/internal/workflows"
)

func TestDomainPackLifecycleEndpointsReloadSkills(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	initialPacks := getDomainPacksForTest(t, srv)
	if len(initialPacks) != 0 {
		t.Fatalf("initial domain packs = %#v, want none", initialPacks)
	}

	sourcePack := writeServerWorkflowPack(t, "triage_pack", "triage_workflow")
	installRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/install", map[string]string{
		"path": sourcePack,
	})
	if installRecorder.Code != http.StatusOK {
		t.Fatalf("install status = %d, want 200; body: %s", installRecorder.Code, installRecorder.Body.String())
	}
	var installed DomainPackActionResult
	decodeDomainPackJSONForTest(t, installRecorder, &installed)
	if installed.Message != "installed" || installed.Pack.Name != "triage_pack" || !installed.Pack.Valid {
		t.Fatalf("installed = %#v, want valid installed triage_pack", installed)
	}
	if installed.Pack.Enabled {
		t.Fatal("installed pack Enabled = true, want disabled until explicit enable")
	}
	wantDir := filepath.Join(srv.deps.Profile.Root, "domain_packs", "triage_pack")
	if filepath.Clean(installed.Pack.Dir) != filepath.Clean(wantDir) {
		t.Fatalf("installed pack dir = %q, want %q", installed.Pack.Dir, wantDir)
	}
	if _, ok := srv.deps.Skills.Get("triage_workflow"); ok {
		t.Fatal("disabled domain-pack skill was loaded after install")
	}

	packs := getDomainPacksForTest(t, srv)
	if len(packs) != 1 || packs[0].Name != "triage_pack" || packs[0].Enabled {
		t.Fatalf("packs = %#v, want one disabled triage_pack", packs)
	}
	skillSummaries := getDomainPackSkillsForTest(t, srv)
	if len(skillSummaries) != 1 {
		t.Fatalf("domain pack skill summaries = %#v, want one", skillSummaries)
	}
	if got := skillSummaries[0]; got.PackName != "triage_pack" || got.Ref != "skills/triage_workflow" || got.Active || got.Enabled || !got.Valid {
		t.Fatalf("disabled skill summary = %#v, want inactive valid triage_workflow", got)
	}

	enableRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/enabled", map[string]any{
		"name":    "triage_pack",
		"enabled": true,
	})
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want 200; body: %s", enableRecorder.Code, enableRecorder.Body.String())
	}
	var enabled DomainPackActionResult
	decodeDomainPackJSONForTest(t, enableRecorder, &enabled)
	if enabled.Message != "enabled" || !enabled.Pack.Enabled {
		t.Fatalf("enabled result = %#v, want enabled pack", enabled)
	}
	loadedSkill, ok := srv.deps.Skills.Get("triage_workflow")
	if !ok {
		t.Fatalf("enabled domain-pack skill missing; statuses = %#v", srv.deps.Skills.Statuses())
	}
	if loadedSkill.Source != "domain_pack" {
		t.Fatalf("enabled skill source = %q, want domain_pack", loadedSkill.Source)
	}
	apiSkills := getSkillsForTest(t, srv)
	if !hasSkillSummaryForTest(apiSkills, "triage_workflow", "domain_pack", true) {
		t.Fatalf("/api/skills = %#v, want enabled domain_pack triage_workflow", apiSkills)
	}
	enabledSkillSummaries := getDomainPackSkillsForTest(t, srv)
	if len(enabledSkillSummaries) != 1 || !enabledSkillSummaries[0].Active || !enabledSkillSummaries[0].Enabled {
		t.Fatalf("enabled skill summaries = %#v, want active enabled triage_workflow", enabledSkillSummaries)
	}

	disableRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/enabled", map[string]any{
		"name":    "triage_pack",
		"enabled": false,
	})
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want 200; body: %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	var disabled DomainPackActionResult
	decodeDomainPackJSONForTest(t, disableRecorder, &disabled)
	if disabled.Message != "disabled" || disabled.Pack.Enabled {
		t.Fatalf("disabled result = %#v, want disabled pack", disabled)
	}
	if _, ok := srv.deps.Skills.Get("triage_workflow"); ok {
		t.Fatal("domain-pack skill still loaded after disable")
	}

	uninstallRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/uninstall", map[string]string{
		"name": "triage_pack",
	})
	if uninstallRecorder.Code != http.StatusOK {
		t.Fatalf("uninstall status = %d, want 200; body: %s", uninstallRecorder.Code, uninstallRecorder.Body.String())
	}
	var uninstalled DomainPackActionResult
	decodeDomainPackJSONForTest(t, uninstallRecorder, &uninstalled)
	if uninstalled.Message != "uninstalled" || uninstalled.Pack.Installed || uninstalled.Pack.Enabled {
		t.Fatalf("uninstalled result = %#v, want uninstalled disabled pack", uninstalled)
	}
	if _, err := os.Stat(wantDir); !os.IsNotExist(err) {
		t.Fatalf("installed pack dir still exists or stat failed unexpectedly: %v", err)
	}
	packsAfterUninstall := getDomainPacksForTest(t, srv)
	if len(packsAfterUninstall) != 0 {
		t.Fatalf("packs after uninstall = %#v, want none", packsAfterUninstall)
	}
	skillsAfterUninstall := getDomainPackSkillsForTest(t, srv)
	if len(skillsAfterUninstall) != 0 {
		t.Fatalf("domain pack skill summaries after uninstall = %#v, want none", skillsAfterUninstall)
	}
}

func TestCapabilityApplyInstallsBuiltInDomainPackTemplate(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	recorder := postDomainPackJSONForTest(t, srv, "/api/capabilities/apply", map[string]any{
		"approved":   true,
		"kind":       "domain_pack",
		"name":       "personal_productivity",
		"packSource": "domain_pack_template",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("apply template status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result CapabilityApplyResult
	decodeDomainPackJSONForTest(t, recorder, &result)
	if result.Action != "installed" || result.Pack.Name != "personal_productivity" || !result.Pack.Valid {
		t.Fatalf("apply result = %#v, want installed valid personal_productivity", result)
	}
	if result.Pack.Enabled {
		t.Fatal("installed template pack Enabled = true, want disabled until explicit enable")
	}
	if _, ok := srv.deps.Skills.Get("daily_planner"); ok {
		t.Fatal("template skill loaded before explicit domain pack enable")
	}
	packs := getDomainPacksForTest(t, srv)
	if len(packs) != 1 || packs[0].Name != "personal_productivity" || packs[0].Enabled {
		t.Fatalf("packs = %#v, want one disabled personal_productivity pack", packs)
	}
}

func TestDomainPackReviewEndpointsExposeReadOnlyDetails(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	sourcePack := writeServerWorkflowPack(t, "review_pack", "review_workflow")
	installRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/install", map[string]string{
		"path": sourcePack,
	})
	if installRecorder.Code != http.StatusOK {
		t.Fatalf("install status = %d, want 200; body: %s", installRecorder.Code, installRecorder.Body.String())
	}

	reviewRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/review", map[string]string{
		"name": "review_pack",
	})
	if reviewRecorder.Code != http.StatusOK {
		t.Fatalf("review status = %d, want 200; body: %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}
	var review domainpacks.PackReview
	decodeDomainPackJSONForTest(t, reviewRecorder, &review)
	if review.Name != "review_pack" || !review.Installed || review.Enabled || !review.Valid {
		t.Fatalf("review = %#v, want installed disabled valid review_pack", review)
	}
	if review.Manifest == nil || len(review.Skills) != 1 || review.Skills[0].Name != "review_workflow" {
		t.Fatalf("review = %#v, want manifest and skill metadata", review)
	}
	if review.SafetySummary == "" {
		t.Fatalf("review = %#v, want safety summary", review)
	}

	templates := getDomainPackTemplatesForTest(t, srv)
	if len(templates) == 0 {
		t.Fatal("templates = empty, want built-in template")
	}
	templateRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/templates/review", map[string]string{
		"name": templates[0].Name,
	})
	if templateRecorder.Code != http.StatusOK {
		t.Fatalf("template review status = %d, want 200; body: %s", templateRecorder.Code, templateRecorder.Body.String())
	}
	var templateReview domainpacks.PackReview
	decodeDomainPackJSONForTest(t, templateRecorder, &templateReview)
	if templateReview.Name != templates[0].Name || templateReview.Source != workflows.SourceDomainPackTemplate || templateReview.Installed {
		t.Fatalf("template review = %#v, want uninstalled built-in template review", templateReview)
	}
	if templateReview.Manifest == nil || len(templateReview.Skills) == 0 || len(templateReview.Tests) == 0 {
		t.Fatalf("template review = %#v, want manifest, skills, and tests", templateReview)
	}
}

func TestDomainPackTemplateEndpointsLifecycleMatrix(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	templates := getDomainPackTemplatesForTest(t, srv)
	if len(templates) == 0 {
		t.Fatal("templates = empty, want built-in templates")
	}
	for _, template := range templates {
		assertDomainPackTemplateSummaryForTest(t, template, false)
		recorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/install-template", map[string]string{
			"name": template.Name,
		})
		if recorder.Code != http.StatusOK {
			t.Fatalf("install-template %s status = %d, want 200; body: %s", template.Name, recorder.Code, recorder.Body.String())
		}
		var installed DomainPackActionResult
		decodeDomainPackJSONForTest(t, recorder, &installed)
		if installed.Message != "installed" || installed.Pack.Name != template.Name || !installed.Pack.Valid {
			t.Fatalf("install-template %s result = %#v, want valid installed pack", template.Name, installed)
		}
		if installed.Pack.Enabled {
			t.Fatalf("installed built-in template %s Enabled = true, want disabled until explicit enable", template.Name)
		}
		assertNoTemplateSkillsLoadedForTest(t, srv, template)
		assertDomainPackSkillSummariesForTemplate(t, srv, template, false)

		templatesAfterInstall := getDomainPackTemplatesForTest(t, srv)
		installedTemplate, ok := findDomainPackTemplateForTest(templatesAfterInstall, template.Name)
		if !ok || !installedTemplate.Installed || installedTemplate.Enabled || !installedTemplate.Valid {
			t.Fatalf("templates after install = %#v, want %s installed disabled valid", templatesAfterInstall, template.Name)
		}

		enableRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/enabled", map[string]any{
			"name":    template.Name,
			"enabled": true,
		})
		if enableRecorder.Code != http.StatusOK {
			t.Fatalf("enable %s status = %d, want 200; body: %s", template.Name, enableRecorder.Code, enableRecorder.Body.String())
		}
		assertDomainPackSkillSummariesForTemplate(t, srv, template, true)
		for _, skill := range template.Skills {
			if _, ok := srv.deps.Skills.Get(skill); !ok {
				t.Fatalf("enabled %s missing skill %s; statuses = %#v", template.Name, skill, srv.deps.Skills.Statuses())
			}
		}

		disableRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/enabled", map[string]any{
			"name":    template.Name,
			"enabled": false,
		})
		if disableRecorder.Code != http.StatusOK {
			t.Fatalf("disable %s status = %d, want 200; body: %s", template.Name, disableRecorder.Code, disableRecorder.Body.String())
		}
		assertNoTemplateSkillsLoadedForTest(t, srv, template)
	}
}

func TestDomainPackInvalidPackListedWithRepairHintAndCannotEnable(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	root, err := srv.domainPackRoot()
	if err != nil {
		t.Fatalf("domainPackRoot() error = %v", err)
	}
	brokenDir := filepath.Join(root, "broken_pack")
	if err := os.MkdirAll(brokenDir, 0o755); err != nil {
		t.Fatalf("mkdir invalid pack: %v", err)
	}
	if err := os.WriteFile(filepath.Join(brokenDir, domainpacks.ManifestFile), []byte("name: Broken Pack\nversion: 0.1.0\n"), 0o644); err != nil {
		t.Fatalf("write invalid pack manifest: %v", err)
	}

	packs := getDomainPacksForTest(t, srv)
	if len(packs) != 1 {
		t.Fatalf("packs = %#v, want one invalid pack", packs)
	}
	if packs[0].Name != "broken_pack" || packs[0].Valid || packs[0].Enabled || packs[0].ValidationError == "" || packs[0].RepairHint == "" {
		t.Fatalf("invalid pack status = %#v, want invalid disabled pack with repair guidance", packs[0])
	}

	recorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/enabled", map[string]any{
		"name":    "broken_pack",
		"enabled": true,
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("enable invalid pack status = %d, want 400; body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestCapabilityApplyEnablesInstalledDomainPack(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	sourcePack := writeServerWorkflowPack(t, "triage_pack", "triage_workflow")
	installRecorder := postDomainPackJSONForTest(t, srv, "/api/domain-packs/install", map[string]string{
		"path": sourcePack,
	})
	if installRecorder.Code != http.StatusOK {
		t.Fatalf("install status = %d, want 200; body: %s", installRecorder.Code, installRecorder.Body.String())
	}

	recorder := postDomainPackJSONForTest(t, srv, "/api/capabilities/apply", map[string]any{
		"approved":           true,
		"kind":               "domain_pack",
		"existingCapability": "triage_pack",
		"packSource":         "domain_pack",
	})
	if recorder.Code != http.StatusOK {
		t.Fatalf("apply enable status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result CapabilityApplyResult
	decodeDomainPackJSONForTest(t, recorder, &result)
	if result.Action != "enabled" || result.Pack.Name != "triage_pack" || !result.Pack.Enabled {
		t.Fatalf("apply result = %#v, want enabled triage_pack", result)
	}
	if _, ok := srv.deps.Skills.Get("triage_workflow"); !ok {
		t.Fatal("domain-pack skill missing after capability apply enable")
	}
}

func TestCapabilityApplyRequiresApproval(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	recorder := postDomainPackJSONForTest(t, srv, "/api/capabilities/apply", map[string]any{
		"kind":       "domain_pack",
		"name":       "personal_productivity",
		"packSource": "domain_pack_template",
	})
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("apply without approval status = %d, want 400; body: %s", recorder.Code, recorder.Body.String())
	}
}

func writeServerWorkflowPack(t *testing.T, packName string, skillName string) string {
	t.Helper()
	packDir := filepath.Join(t.TempDir(), packName)
	if _, err := domainpacks.WriteWorkflowSkillPack(domainpacks.WorkflowSkillPackInput{
		PackDir:      packDir,
		Name:         packName,
		Version:      "0.1.0",
		Description:  "Local workflow pack.",
		Category:     "capability",
		DefaultState: domainpacks.DefaultStateEnabled,
		Workflows: []skills.WorkflowCompileInput{{
			Name:        skillName,
			Description: "Triage local project work.",
			Triggers:    []string{"triage local work"},
			Steps: []skills.WorkflowStep{
				{Instruction: "Read the relevant local files.", Tool: "read_file"},
			},
			ExamplePrompts: []string{"Triage this local project work."},
		}},
	}); err != nil {
		t.Fatalf("WriteWorkflowSkillPack() error = %v", err)
	}
	return packDir
}

func getDomainPacksForTest(t *testing.T, srv *Server) []domainpacks.PackStatus {
	t.Helper()
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/domain-packs", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list domain packs status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var packs []domainpacks.PackStatus
	decodeDomainPackJSONForTest(t, recorder, &packs)
	return packs
}

func getDomainPackTemplatesForTest(t *testing.T, srv *Server) []workflows.TemplateSummary {
	t.Helper()
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/domain-packs/templates", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list domain pack templates status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var templates []workflows.TemplateSummary
	decodeDomainPackJSONForTest(t, recorder, &templates)
	return templates
}

func getDomainPackSkillsForTest(t *testing.T, srv *Server) []DomainPackSkillSummary {
	t.Helper()
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/domain-packs/skills", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list domain pack skills status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var summaries []DomainPackSkillSummary
	decodeDomainPackJSONForTest(t, recorder, &summaries)
	return summaries
}

func findDomainPackTemplateForTest(templates []workflows.TemplateSummary, name string) (workflows.TemplateSummary, bool) {
	for _, template := range templates {
		if template.Name == name {
			return template, true
		}
	}
	return workflows.TemplateSummary{}, false
}

func assertDomainPackTemplateSummaryForTest(t *testing.T, template workflows.TemplateSummary, installed bool) {
	t.Helper()
	if template.Name == "" || template.Version == "" || template.Description == "" || template.Category == "" {
		t.Fatalf("template summary missing identity metadata: %#v", template)
	}
	if template.Source != workflows.SourceDomainPackTemplate || template.Path == "" || template.InstallAction != "install" {
		t.Fatalf("template summary source/path/action = %#v, want built-in installable template", template)
	}
	if template.Installed != installed || template.Enabled || !template.Valid || template.DefaultState != workflows.DefaultStateDisabled {
		t.Fatalf("template summary lifecycle = %#v, want installed=%v enabled=false valid=true default disabled", template, installed)
	}
	if len(template.RequiredTools) == 0 || len(template.Skills) == 0 || len(template.Workflows) == 0 || template.SafetySummary == "" {
		t.Fatalf("template summary missing tools/skills/workflows/safety metadata: %#v", template)
	}
}

func assertNoTemplateSkillsLoadedForTest(t *testing.T, srv *Server, template workflows.TemplateSummary) {
	t.Helper()
	for _, skill := range template.Skills {
		if _, ok := srv.deps.Skills.Get(skill); ok {
			t.Fatalf("skill %s from %s loaded while pack should be inactive", skill, template.Name)
		}
	}
}

func assertDomainPackSkillSummariesForTemplate(t *testing.T, srv *Server, template workflows.TemplateSummary, active bool) {
	t.Helper()
	summaries := getDomainPackSkillsForTest(t, srv)
	for _, skill := range template.Skills {
		wantRef := filepath.ToSlash(filepath.Join(domainpacks.PackSkillsDir, skill))
		found := false
		for _, summary := range summaries {
			if summary.PackName == template.Name && summary.Ref == wantRef {
				found = true
				if summary.Active != active || summary.Enabled != active || !summary.Valid {
					t.Fatalf("skill summary for %s/%s = %#v, want active=%v enabled=%v valid=true", template.Name, skill, summary, active, active)
				}
				break
			}
		}
		if !found {
			t.Fatalf("domain pack skill summaries = %#v, missing %s/%s", summaries, template.Name, wantRef)
		}
	}
}

func getSkillsForTest(t *testing.T, srv *Server) []SkillSummary {
	t.Helper()
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/skills", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list skills status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var summaries []SkillSummary
	decodeDomainPackJSONForTest(t, recorder, &summaries)
	return summaries
}

func postDomainPackJSONForTest(t *testing.T, srv *Server, path string, payload any) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	request := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	return recorder
}

func decodeDomainPackJSONForTest(t *testing.T, recorder *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(recorder.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body.String(), err)
	}
}

func hasSkillSummaryForTest(summaries []SkillSummary, name string, source string, enabled bool) bool {
	for _, summary := range summaries {
		if summary.Name == name && summary.Source == source && summary.Enabled == enabled {
			return true
		}
	}
	return false
}
