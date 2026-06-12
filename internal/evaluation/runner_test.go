package evaluation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
)

type fakeRuntime struct {
	healthErr error
	models    []models.ModelInfo
	tokens    []string
}

func (f fakeRuntime) Health(ctx context.Context) error {
	return f.healthErr
}

func (f fakeRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return f.models, nil
}

func (f fakeRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (f fakeRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	for _, token := range f.tokens {
		if err := emit(models.ChatEvent{Token: token}); err != nil {
			return err
		}
	}
	return emit(models.ChatEvent{Done: true})
}

func TestRunnerRunCreatesReport(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv(config.EnvHome, home)
	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	cfg.Models["low_memory"] = config.ModelConfig{
		Provider:    "ollama",
		Name:        "tiny:2b",
		BaseURL:     "http://localhost:11434/api",
		Temperature: 0.1,
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	report, err := Runner{
		Config:  cfg,
		Profile: profile,
		Runtime: fakeRuntime{
			models: []models.ModelInfo{
				{Name: "tiny:2b", Size: 1234567890},
				{Name: "remote:cloud", Size: 300},
			},
			tokens: []string{"READY"},
		},
	}.Run(ctx, RunOptions{Mode: "low-memory"})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Summary.Failed != 0 {
		t.Fatalf("failed tasks = %d, report = %+v", report.Summary.Failed, report)
	}
	if report.Summary.Passed == 0 {
		t.Fatal("expected passed tasks")
	}
	if report.Path == "" {
		t.Fatal("report path is empty")
	}
	assertFloatMetric(t, report.Metrics, "tool_call_success_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "rag_retrieval_accuracy", 1.0)
	assertFloatMetric(t, report.Metrics, "rollback_success_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "skill_reuse_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "routing_matrix_pass_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "real_world_route_pass_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "context_low_memory_budget_fit_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "capability_gap_pass_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "domain_pack_template_pass_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "model_profile_artifact_safety_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "notification_secret_redaction_rate", 1.0)
	assertFloatMetric(t, report.Metrics, "post_rc_smartness_success_rate", 1.0)
	if got := metricFloat(t, report.Metrics, "domain_pack_templates_checked"); got < 3 {
		t.Fatalf("domain_pack_templates_checked = %v, want >= 3", got)
	}
	if got := metricFloat(t, report.Metrics, "tokens_per_second"); got <= 0 {
		t.Fatalf("tokens_per_second = %v, want > 0", got)
	}
	if _, ok := report.Metrics["ram_heap_alloc_bytes"]; !ok {
		t.Fatal("ram_heap_alloc_bytes metric is missing")
	}
	loaded, err := LoadLatest(profile)
	if err != nil {
		t.Fatalf("LoadLatest() error = %v", err)
	}
	if loaded.ID != report.ID {
		t.Fatalf("latest report ID = %q, want %q", loaded.ID, report.ID)
	}
}

func TestRunnerCanSkipModelChecks(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv(config.EnvHome, home)
	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	report, err := Runner{
		Config:  cfg,
		Profile: profile,
	}.Run(ctx, RunOptions{Mode: "low-memory", SkipModel: true})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if report.Summary.Failed != 0 {
		t.Fatalf("failed tasks = %d, report = %+v", report.Summary.Failed, report)
	}
	if report.Summary.Skipped != 2 {
		t.Fatalf("skipped tasks = %d, want 2", report.Summary.Skipped)
	}
	for _, task := range report.Tasks {
		if (task.Name == "model_readiness" || task.Name == "model_chat_smoke") && task.Status != StatusSkip {
			t.Fatalf("model task was not skipped: %+v", task)
		}
	}
}

func TestRunnerReportsDangerousCommandTrend(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv(config.EnvHome, home)
	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	runner := Runner{Config: cfg, Profile: profile}
	first, err := runner.Run(ctx, RunOptions{Mode: "low-memory", SkipModel: true})
	if err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if first.Metrics["dangerous_command_prevention_trend"] != "no_previous_report" {
		t.Fatalf("first trend = %v, want no_previous_report", first.Metrics["dangerous_command_prevention_trend"])
	}
	second, err := runner.Run(ctx, RunOptions{Mode: "low-memory", SkipModel: true})
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if second.Metrics["dangerous_command_prevention_trend"] != "stable" {
		t.Fatalf("second trend = %v, want stable", second.Metrics["dangerous_command_prevention_trend"])
	}
}

func TestPostRCEvalTasksPassDeterministically(t *testing.T) {
	tasks := []struct {
		name string
		run  func() TaskResult
	}{
		{name: "routing_matrix_behavior", run: evalRoutingMatrixBehavior},
		{name: "real_world_route_smoke", run: evalRealWorldRouteSmoke},
		{name: "conversation_route_continuity", run: evalConversationRouteContinuity},
		{name: "route_governance_classifier", run: evalRouteGovernanceClassifier},
		{name: "context_compiler_low_memory", run: evalContextCompilerLowMemoryBehavior},
		{name: "capability_registry_gap", run: evalCapabilityRegistryGapBehavior},
		{name: "domain_pack_template_awareness", run: evalDomainPackTemplateAwareness},
		{name: "model_profile_artifact_safety", run: func() TaskResult {
			return evalModelProfileArtifactSafety(t.TempDir())
		}},
		{name: "notification_center_local_inbox", run: func() TaskResult {
			root := t.TempDir()
			profile := &profiles.Profile{
				Root:          root,
				Notifications: filepath.Join(root, "notifications"),
			}
			return evalNotificationCenterLocalInbox(profile, "deterministic")
		}},
	}

	for _, task := range tasks {
		t.Run(task.name, func(t *testing.T) {
			result := task.run()
			if result.Status != StatusPass {
				t.Fatalf("%s status = %q, details = %s, metrics = %#v", task.name, result.Status, result.Details, result.Metrics)
			}
		})
	}
}

func TestEvalProfileLayoutRequiresEvalDirectory(t *testing.T) {
	root := t.TempDir()
	profile := &profiles.Profile{
		Root:          root,
		Database:      filepath.Join(root, "memory.sqlite"),
		Sessions:      filepath.Join(root, "sessions"),
		Skills:        filepath.Join(root, "skills"),
		RAG:           filepath.Join(root, "rag"),
		Logs:          filepath.Join(root, "logs"),
		Notifications: filepath.Join(root, "notifications"),
		Snapshots:     filepath.Join(root, "snapshots"),
		Evals:         filepath.Join(root, "evals"),
		Permissions:   filepath.Join(root, "permissions"),
	}
	for _, dir := range []string{filepath.Dir(profile.Database), profile.Sessions, profile.Skills, profile.RAG, profile.Logs, profile.Notifications, profile.Snapshots} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", dir, err)
		}
	}

	result := Runner{Profile: profile}.evalProfileLayout()
	if result.Status != StatusFail {
		t.Fatalf("Status = %q, want fail for missing evals directory", result.Status)
	}
}

func TestEvalNotificationCenterLocalInboxRedactsSecrets(t *testing.T) {
	root := t.TempDir()
	profile := &profiles.Profile{
		Root:          root,
		Notifications: filepath.Join(root, "notifications"),
	}
	result := evalNotificationCenterLocalInbox(profile, "redaction")
	if result.Status != StatusPass {
		t.Fatalf("Status = %q, details = %s, metrics = %#v", result.Status, result.Details, result.Metrics)
	}
	assertFloatMetric(t, result.Metrics, "notification_secret_redaction_rate", 1.0)

	data, err := os.ReadFile(filepath.Join(profile.Notifications, "inbox.json"))
	if err != nil {
		t.Fatalf("ReadFile(inbox.json) error = %v", err)
	}
	for _, leaked := range []string{
		"sk-abcdefghijklmnopqrstuvwxyz123456",
		"ghp_abcdefghijklmnopqrstuvwxyz123456",
		"AKIA1234567890ABCDEF",
	} {
		if strings.Contains(string(data), leaked) {
			t.Fatalf("inbox leaked secret-like value %q: %s", leaked, data)
		}
	}
}

func TestLocalInstalledModelsExcludesCloudPlaceholders(t *testing.T) {
	local := localInstalledModels([]models.ModelInfo{
		{Name: "glm:cloud", Size: 323},
		{Name: "tiny-local:2b", Size: 0},
		{Name: "small:2b", Size: 1500000000},
	})
	if len(local) != 2 {
		t.Fatalf("local count = %d, want 2", len(local))
	}
	if local[0].Name != "tiny-local:2b" || local[1].Name != "small:2b" {
		t.Fatalf("local models = %+v", local)
	}
}

func assertFloatMetric(t *testing.T, metrics map[string]any, key string, want float64) {
	t.Helper()
	got := metricFloat(t, metrics, key)
	if got != want {
		t.Fatalf("%s = %v, want %v", key, got, want)
	}
}

func metricFloat(t *testing.T, metrics map[string]any, key string) float64 {
	t.Helper()
	switch value := metrics[key].(type) {
	case float64:
		return value
	case int:
		return float64(value)
	case int64:
		return float64(value)
	default:
		t.Fatalf("%s metric = %T(%v), want numeric", key, value, value)
		return 0
	}
}
