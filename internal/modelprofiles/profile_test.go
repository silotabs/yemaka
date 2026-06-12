package modelprofiles

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveLoadListValidProfile(t *testing.T) {
	store := NewStore(t.TempDir())
	profile := Profile{
		Name:        "Refined Coding",
		Description: "Small local coding behavior profile.",
		BaseModel:   "qwen2.5-coder:3b",
		System:      "Prefer concise patches and preserve local-first defaults.",
		Tags:        []string{"coding", " low-resource ", "coding"},
	}

	path, err := store.Save(profile)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if filepath.Base(path) != "refined-coding.yaml" {
		t.Fatalf("saved filename = %q, want refined-coding.yaml", filepath.Base(path))
	}

	loaded, err := store.Load(profile.Name)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", loaded.SchemaVersion, SchemaVersion)
	}
	if loaded.Name != profile.Name {
		t.Fatalf("Name = %q, want %q", loaded.Name, profile.Name)
	}
	if loaded.Parameters.Temperature != DefaultTemperature {
		t.Fatalf("Temperature = %v, want %v", loaded.Parameters.Temperature, DefaultTemperature)
	}
	if loaded.Parameters.NumCtx != DefaultNumCtx {
		t.Fatalf("NumCtx = %d, want %d", loaded.Parameters.NumCtx, DefaultNumCtx)
	}
	if len(loaded.Tags) != 2 || loaded.Tags[1] != "low-resource" {
		t.Fatalf("Tags = %#v, want normalized unique tags", loaded.Tags)
	}

	profiles, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(profiles) != 1 || profiles[0].Name != profile.Name {
		t.Fatalf("List() = %#v, want saved profile", profiles)
	}
}

func TestRenderModelfileIncludesCoreDirectives(t *testing.T) {
	profile := Profile{
		Name:      "Tool Helper",
		BaseModel: "qwen2.5:3b",
		System:    "Use tools only when they are explicitly available.",
		Parameters: Parameters{
			Temperature: 0.1,
			NumCtx:      2048,
		},
	}

	rendered := RenderModelfile(profile)
	for _, want := range []string{
		"FROM qwen2.5:3b",
		"SYSTEM \"\"\"",
		"Use tools only when they are explicitly available.",
		"PARAMETER temperature 0.1",
		"PARAMETER num_ctx 2048",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered Modelfile missing %q:\n%s", want, rendered)
		}
	}
}

func TestValidationBlocksSecretsAndRawPrivateMarkers(t *testing.T) {
	base := Profile{
		Name:      "Safe Profile",
		BaseModel: "qwen2.5:3b",
		System:    "Stay local and concise.",
	}

	withSecret := base
	withSecret.System = "Use api_key = sk-testsecretvalue1234567890 in requests."
	if err := Validate(withSecret); err == nil {
		t.Fatal("Validate() allowed profile with secret-like value")
	}
	if rendered := RenderModelfile(withSecret); rendered != "" {
		t.Fatalf("RenderModelfile() rendered unsafe profile: %q", rendered)
	}

	withRawMemory := base
	withRawMemory.System = "PRIVATE MEMORY: user raw note should not be stored here."
	if err := Validate(withRawMemory); err == nil {
		t.Fatal("Validate() allowed profile with raw private content marker")
	}
}

func TestLoadRejectsAutoSwitchAndDownloadFields(t *testing.T) {
	store := NewStore(t.TempDir())

	for filename, content := range map[string]string{
		"auto-switch.yaml": `
schema_version: 1
name: Auto Switch
base_model: qwen2.5:3b
system: Stay local.
auto_switch: true
`,
		"download.yaml": `
schema_version: 1
name: Download
base_model: qwen2.5:3b
system: Stay local.
download_model: true
`,
	} {
		if err := os.WriteFile(filepath.Join(store.Dir, filename), []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", filename, err)
		}
	}

	if _, err := store.Load("Auto Switch"); err == nil {
		t.Fatal("Load() allowed auto_switch field")
	}
	if _, err := store.Load("Download"); err == nil {
		t.Fatal("Load() allowed download_model field")
	}
}

func TestDeterministicFilenamesAndSafeNames(t *testing.T) {
	name := " ../Qwen Coding/Profile!! "
	safe, err := SafeName(name)
	if err != nil {
		t.Fatalf("SafeName() error = %v", err)
	}
	if safe != "qwen-coding-profile" {
		t.Fatalf("SafeName() = %q, want qwen-coding-profile", safe)
	}

	filename, err := FileName(name)
	if err != nil {
		t.Fatalf("FileName() error = %v", err)
	}
	if filename != "qwen-coding-profile.yaml" {
		t.Fatalf("FileName() = %q, want qwen-coding-profile.yaml", filename)
	}

	store := NewStore(t.TempDir())
	path, err := store.Save(Profile{
		Name:      name,
		BaseModel: "qwen2.5:3b",
		System:    "Stay local.",
	})
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if filepath.Base(path) != filename {
		t.Fatalf("saved filename = %q, want %q", filepath.Base(path), filename)
	}

	if _, err := SafeName("../../"); err == nil {
		t.Fatal("SafeName() allowed path-only name")
	}
}

func TestAssessReadinessScoresReviewedLocalProfile(t *testing.T) {
	profile := Profile{
		Name:        "Reviewed Local Helper",
		Description: "Reviewed low-resource assistant behavior.",
		BaseModel:   "qwen2.5:3b",
		System:      "Stay local-first. Use tools only when policy approval is satisfied. Ask clarification when unclear.",
		Parameters: Parameters{
			Temperature: 0.2,
			NumCtx:      4096,
		},
		Metadata: Metadata{
			Purpose:     "reviewed support behavior",
			RefinedFrom: "conversation-123",
		},
		Tags: []string{"reviewed", "local"},
	}

	report := AssessReadiness(profile)
	if report.Schema != "yemaka.model_profile_readiness.v1" {
		t.Fatalf("Schema = %q", report.Schema)
	}
	if report.Status != "ready" || report.Score < 90 {
		t.Fatalf("readiness = %+v, want ready high score", report)
	}
}

func TestAssessReadinessBlocksInvalidProfile(t *testing.T) {
	report := AssessReadiness(Profile{
		Name:      "Unsafe",
		BaseModel: "qwen2.5:3b",
		System:    "api_key = sk-testsecretvalue1234567890",
	})
	if report.Status != "blocked" || report.Score != 0 {
		t.Fatalf("readiness = %+v, want blocked zero score", report)
	}
	if len(report.Checks) != 1 || report.Checks[0].Status != "blocked" {
		t.Fatalf("checks = %+v, want blocked validation check", report.Checks)
	}
}

func TestCompareProfilesReportsSummaryDeltasWithoutPromptDiff(t *testing.T) {
	before := Normalize(Profile{
		Name:        "Coding Helper",
		Description: "Old description",
		BaseModel:   "small:2b",
		System:      "Stay local. Use tools only with policy approval. Ask clarification when unclear.",
		Tags:        []string{"reviewed", "coding"},
	})
	after := before
	after.BaseModel = "qwen2.5-coder:3b"
	after.System = before.System + " Prefer concise patches."
	after.Tags = []string{"reviewed", "patching"}

	report := Compare(before, after)
	if !report.HasBaseline || report.SameArtifact {
		t.Fatalf("comparison = %+v, want changed baseline", report)
	}
	for _, want := range []string{"base_model", "system", "tags"} {
		if !containsString(report.ChangedFields, want) {
			t.Fatalf("ChangedFields = %#v, missing %q", report.ChangedFields, want)
		}
	}
	if !containsString(report.AddedTags, "patching") || !containsString(report.RemovedTags, "coding") {
		t.Fatalf("tag delta added=%#v removed=%#v", report.AddedTags, report.RemovedTags)
	}
	if report.SystemCharsDelta <= 0 {
		t.Fatalf("SystemCharsDelta = %d, want positive", report.SystemCharsDelta)
	}
}

func TestStoreHistoryReturnsLatestSavedArtifactOnly(t *testing.T) {
	store := NewStore(t.TempDir())
	profile := Profile{
		Name:        "History Helper",
		Description: "Saved local profile.",
		BaseModel:   "small:2b",
		System:      "Stay local. Use tools only with policy approval. Ask clarification when unclear.",
		Metadata: Metadata{
			RefinedFrom: "conversation-1",
		},
	}
	if _, err := store.Save(profile); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	events, err := store.History(profile.Name)
	if err != nil {
		t.Fatalf("History() error = %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("History() len = %d, want 1", len(events))
	}
	if events[0].Kind != "saved_profile" || events[0].ReadinessScore <= 0 || events[0].Path == "" {
		t.Fatalf("History() = %+v", events)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
