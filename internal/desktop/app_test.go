package desktop

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/agent"
	"yemaka/internal/config"
	"yemaka/internal/extensions"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/safety"
	"yemaka/internal/scheduler"
)

func TestLocalInstalledModelsExcludesCloudPlaceholders(t *testing.T) {
	installed := []models.ModelInfo{
		{Name: "glm-5.1:cloud", Size: 327},
		{Name: "ministral-3:3b", Size: 2953840808},
	}
	local := localInstalledModels(installed)
	if len(local) != 1 {
		t.Fatalf("local model count = %d, want 1", len(local))
	}
	if local[0].Name != "ministral-3:3b" {
		t.Fatalf("local model = %q", local[0].Name)
	}
}

func TestRecommendInstalledModelPrefersSmallLocalModel(t *testing.T) {
	installed := []models.ModelInfo{
		{Name: "larger:8b", Size: 5000000000},
		{Name: "small:2b", Size: 1500000000},
	}
	got := recommendInstalledModel(installed)
	if got != "small:2b" {
		t.Fatalf("recommendInstalledModel() = %q, want small:2b", got)
	}
}

func TestDesktopConversationMessageResultsIncludesRunningAgentTurnPlaceholder(t *testing.T) {
	user := memory.Message{
		ID:             "msg_user",
		ConversationID: "conv_1",
		Role:           "user",
		Content:        "plain local question",
		CreatedAt:      "2026-06-03T12:00:00Z",
		ActiveVariant:  true,
	}
	turn := memory.AgentTurn{
		ID:             "turn_1",
		ConversationID: "conv_1",
		UserMessageID:  user.ID,
		Status:         agent.AgentTurnRunning,
		Trace:          []string{"accepted task", "task: chat (low risk)", "model: local-small"},
		Model:          "local-small",
		CreatedAt:      "2026-06-03T12:00:01Z",
		UpdatedAt:      "2026-06-03T12:00:02Z",
	}

	result := desktopConversationMessageResults([]memory.Message{user}, nil, []memory.AgentTurn{turn})
	if len(result) != 2 {
		t.Fatalf("messages = %d, want user plus running assistant placeholder: %+v", len(result), result)
	}
	placeholder := result[1]
	if placeholder.ID != turn.ID || placeholder.Role != "assistant" || placeholder.Meta != "working" || placeholder.ParentID != user.ID {
		t.Fatalf("placeholder = %+v", placeholder)
	}
	if strings.Join(placeholder.Trace, "\n") != strings.Join(turn.Trace, "\n") {
		t.Fatalf("placeholder trace = %+v, want %+v", placeholder.Trace, turn.Trace)
	}
}

func TestDesktopCreateJobRejectsKnownExtensionInvalidInput(t *testing.T) {
	app, cleanup := newTestDesktopApp(t)
	defer cleanup()
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	store := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	if _, err := store.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	_, err := app.CreateJob(JobCreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task",
		Input:        map[string]any{},
		Approved:     true,
		Enabled:      true,
	})
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("CreateJob(invalid extension input) error = %v, want task requirement", err)
	}
}

func TestDesktopCreateJobRejectsMissingExtensionTarget(t *testing.T) {
	app, cleanup := newTestDesktopApp(t)
	defer cleanup()

	_, err := app.CreateJob(JobCreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "missing_test_extension",
		Approved:     true,
	})
	if err == nil || !strings.Contains(err.Error(), "extension target") || !strings.Contains(err.Error(), "is not installed") {
		t.Fatalf("CreateJob(missing extension) error = %v, want missing extension guidance", err)
	}
}

func TestDesktopSetJobEnabledRejectsKnownExtensionInvalidStoredInput(t *testing.T) {
	app, cleanup := newTestDesktopApp(t)
	defer cleanup()
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	extensionStore := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	if _, err := extensionStore.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_enable",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scheduleStore, err := scheduler.Open(context.Background(), app.profile.Database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	defer scheduleStore.Close()
	job, err := scheduleStore.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_enable",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = app.SetJobEnabled(JobEnabledInput{ID: job.ID, Enabled: true})
	if err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("SetJobEnabled(invalid extension input) error = %v, want task requirement", err)
	}
}

func TestDesktopUpdateJobInputValidatesAndAnnotatesInvalidStoredInput(t *testing.T) {
	app, cleanup := newTestDesktopApp(t)
	defer cleanup()
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	extensionStore := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	if _, err := extensionStore.Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_update",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	scheduleStore, err := scheduler.Open(context.Background(), app.profile.Database)
	if err != nil {
		t.Fatalf("scheduler.Open() error = %v", err)
	}
	job, err := scheduleStore.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_update",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	scheduleStore.Close()

	jobs, err := app.ListJobs()
	if err != nil {
		t.Fatalf("ListJobs() error = %v", err)
	}
	if len(jobs) != 1 || jobs[0].InputStatus != scheduler.InputStatusInvalid || !strings.Contains(jobs[0].InputValidationError, "input.task is required") {
		t.Fatalf("jobs = %+v, want invalid input annotation", jobs)
	}
	status, err := app.JobStatus()
	if err != nil {
		t.Fatalf("JobStatus() error = %v", err)
	}
	if status.InvalidInputJobs != 1 {
		t.Fatalf("JobStatus() = %+v, want one invalid input job", status)
	}

	if _, err := app.UpdateJobInput(JobInputUpdateInput{ID: job.ID, Input: map[string]any{}, Approved: true}); err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("UpdateJobInput(invalid) error = %v, want task requirement", err)
	}
	updated, err := app.UpdateJobInput(JobInputUpdateInput{ID: job.ID, Input: map[string]any{"task": "scheduled cleanup"}, Approved: true})
	if err != nil {
		t.Fatalf("UpdateJobInput(valid) error = %v", err)
	}
	if updated.InputStatus != scheduler.InputStatusValid || updated.Input["task"] != "scheduled cleanup" || updated.Enabled {
		t.Fatalf("updated = %+v, want valid disabled input", updated)
	}
}

func TestWorkspaceRootFromGrantsPrefersProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	project := filepath.Join(tmp, "yemaka")
	if err := os.MkdirAll(filepath.Join(project, "docs"), 0o755); err != nil {
		t.Fatalf("create project docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	store := safety.NewWorkspaceGrantStore(filepath.Join(tmp, "permissions", "workspace_grants.json"))
	if _, err := store.Grant(filepath.Join(project, "docs"), "docs", "test"); err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	root, ok := workspaceRootFromGrants(store)
	if !ok {
		t.Fatal("workspaceRootFromGrants() ok = false, want true")
	}
	if root != project {
		t.Fatalf("workspaceRootFromGrants() = %q, want %q", root, project)
	}
}

func newTestDesktopApp(t *testing.T) (*App, func()) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv(config.EnvHome, root)
	cfg := config.Default()
	cfg.Path = filepath.Join(root, config.ConfigName)
	cfg.App.SetupComplete = true
	cfg.Memory.Database = filepath.Join(root, "profiles", "default", "memory.sqlite")
	if err := config.Write(cfg.Path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("init profile: %v", err)
	}
	store, err := memory.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	ragStore, err := rag.Open(ctx, profile.Database)
	if err != nil {
		_ = store.Close()
		t.Fatalf("open rag: %v", err)
	}
	app := &App{
		config:  cfg,
		profile: profile,
		store:   store,
		rag:     ragStore,
	}
	return app, func() {
		_ = ragStore.Close()
		_ = store.Close()
	}
}

func TestBroadDesktopWorkspaceRootBlocksSystemRoots(t *testing.T) {
	for _, path := range []string{"/", "/Applications", "/System", "/Library", "/Users"} {
		if !isBroadDesktopWorkspaceRoot(path) {
			t.Fatalf("isBroadDesktopWorkspaceRoot(%q) = false, want true", path)
		}
	}
	if isBroadDesktopWorkspaceRoot(filepath.Join(t.TempDir(), "project")) {
		t.Fatal("isBroadDesktopWorkspaceRoot(temp project) = true, want false")
	}
}

func TestInsideAppBundleDetectedAsUnsafeWorkspaceRoot(t *testing.T) {
	path := filepath.Join("/Applications", "Yemaka.app", "Contents", "MacOS")
	if !isInsideAppBundle(path) {
		t.Fatalf("isInsideAppBundle(%q) = false, want true", path)
	}
	if isInsideAppBundle(filepath.Join(t.TempDir(), "project")) {
		t.Fatal("isInsideAppBundle(temp project) = true, want false")
	}
}

func TestApplySettingsPreservesSearchProviderWhenSearchDisabled(t *testing.T) {
	cfg := config.Default()
	err := applySettings(cfg, SettingsInput{
		LowMemoryMode:           true,
		MaxContextTokens:        4096,
		InternetEnabled:         false,
		InternetSearchEnabled:   false,
		InternetSearchProvider:  "firecrawl",
		InternetSearchEndpoint:  "https://api.firecrawl.dev/v2/search",
		InternetSearchAPIKeyEnv: "FIRECRAWL_API_KEY",
	})
	if err != nil {
		t.Fatalf("applySettings() error = %v", err)
	}
	if cfg.Internet.Enabled || cfg.Internet.Search.Enabled {
		t.Fatalf("internet/search enabled = %t/%t, want disabled defaults", cfg.Internet.Enabled, cfg.Internet.Search.Enabled)
	}
	if cfg.Internet.Search.Provider != "firecrawl" {
		t.Fatalf("search provider = %q, want firecrawl preserved for later enablement", cfg.Internet.Search.Provider)
	}
	if cfg.Internet.Search.APIKeyEnv != "FIRECRAWL_API_KEY" {
		t.Fatalf("api key env = %q, want FIRECRAWL_API_KEY", cfg.Internet.Search.APIKeyEnv)
	}
}

func TestDesktopSettingsViewReportsResponseModeSupport(t *testing.T) {
	cfg := config.Default()
	err := applySettings(cfg, SettingsInput{
		LowMemoryMode:     true,
		MaxContextTokens:  4096,
		ResponseMode:      config.ResponseModeDeep,
		ShowThinkingTrace: true,
		UITheme:           "system",
		RAGEnabled:        true,
		EmbeddingModel:    "nomic-embed-text",
	})
	if err != nil {
		t.Fatalf("applySettings() error = %v", err)
	}

	view := settingsView(cfg)
	if view.SettingsSchemaVersion < 2 {
		t.Fatalf("settings schema version = %d, want >= 2", view.SettingsSchemaVersion)
	}
	if !view.FeatureSupport.ResponseMode || !view.FeatureSupport.ShowThinkingTrace {
		t.Fatalf("feature support = %+v, want response mode and thinking trace", view.FeatureSupport)
	}
	if view.ResponseMode != config.ResponseModeDeep {
		t.Fatalf("response mode = %q, want deep", view.ResponseMode)
	}
	if !view.ShowThinkingTrace {
		t.Fatal("show thinking trace = false, want true")
	}
}

func TestApplySettingsNormalizesStaleSearchProviderDefaultEnv(t *testing.T) {
	cfg := config.Default()
	err := applySettings(cfg, SettingsInput{
		LowMemoryMode:           true,
		MaxContextTokens:        4096,
		InternetEnabled:         true,
		InternetSearchEnabled:   true,
		InternetSearchProvider:  "firecrawl",
		InternetSearchAPIKeyEnv: "TAVILY_API_KEY",
	})
	if err != nil {
		t.Fatalf("applySettings() error = %v", err)
	}
	if cfg.Internet.Search.APIKeyEnv != "FIRECRAWL_API_KEY" {
		t.Fatalf("api key env = %q, want FIRECRAWL_API_KEY", cfg.Internet.Search.APIKeyEnv)
	}

	err = applySettings(cfg, SettingsInput{
		LowMemoryMode:           true,
		MaxContextTokens:        4096,
		InternetEnabled:         true,
		InternetSearchEnabled:   true,
		InternetSearchProvider:  "serper",
		InternetSearchAPIKeyEnv: "FIRECRAWL_API_KEY",
	})
	if err != nil {
		t.Fatalf("applySettings() serper error = %v", err)
	}
	if cfg.Internet.Search.APIKeyEnv != "SERPER_API_KEY" {
		t.Fatalf("api key env = %q, want SERPER_API_KEY", cfg.Internet.Search.APIKeyEnv)
	}
}
