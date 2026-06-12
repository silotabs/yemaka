package heartbeat

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/extensions"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
	"yemaka/internal/scheduler"
)

type fakeRuntime struct {
	models []models.ModelInfo
}

func TestRunHeartbeatUsesRequiredStatusVocabulary(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default()
	cfg.Heartbeat.Checks = config.HeartbeatChecksConfig{
		Scheduler:  true,
		Internet:   true,
		Connectors: true,
	}
	cfg.Scheduler.Enabled = false
	cfg.Internet.Enabled = true
	cfg.Internet.Search.Enabled = true
	cfg.Internet.Search.Provider = "none"
	cfg.Connectors.Enabled = true
	cfg.Connectors.LocalAPI.Enabled = true
	cfg.Connectors.LocalAPI.RequireToken = true
	cfg.Connectors.LocalAPI.TokenEnv = "YEMAKA_TEST_MISSING_CONNECTOR_TOKEN"
	cfg.Connectors.LocalAPI.SecretRef = config.SecretRefConfig{Provider: "env", Name: "YEMAKA_TEST_MISSING_CONNECTOR_TOKEN"}
	t.Setenv("YEMAKA_TEST_MISSING_CONNECTOR_TOKEN", "")

	schedulerStatus := scheduler.Status{Enabled: false}
	report := Run(ctx, Input{
		Config:          cfg,
		SchedulerStatus: &schedulerStatus,
	})
	assertCheckStatus(t, report, "scheduler", StatusDisabled)
	assertCheckStatus(t, report, "internet", StatusNeedsConfig)
	assertCheckStatus(t, report, "connectors", StatusNeedsAuth)
	for _, check := range report.Checks {
		switch check.Status {
		case StatusHealthy, StatusWarning, StatusBroken, StatusDisabled, StatusNeedsAuth, StatusNeedsConfig:
		default:
			t.Fatalf("check %s has unsupported status %q", check.Name, check.Status)
		}
	}
}

func assertCheckStatus(t *testing.T, report Report, name string, want string) {
	t.Helper()
	for _, check := range report.Checks {
		if check.Name == name {
			if check.Status != want {
				t.Fatalf("%s status = %q, want %q; report=%+v", name, check.Status, want, report)
			}
			return
		}
	}
	t.Fatalf("missing heartbeat check %q in %s", name, strings.Join(checkNames(report), ", "))
}

func checkNames(report Report) []string {
	names := make([]string, 0, len(report.Checks))
	for _, check := range report.Checks {
		names = append(names, check.Name)
	}
	return names
}

func (f fakeRuntime) Health(ctx context.Context) error {
	return nil
}

func (f fakeRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return f.models, nil
}

func (f fakeRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (f fakeRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	return nil
}

func TestRunHeartbeatRecordsCoreChecks(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv(config.EnvHome, root)
	cfg := config.Default()
	cfg.Path = filepath.Join(root, config.ConfigName)
	cfg.Models["low_memory"] = config.ModelConfig{Name: "small:2b", Provider: "ollama"}
	cfg.Memory.Database = filepath.Join(root, "profiles", "default", "memory.sqlite")
	if err := config.Write(cfg.Path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("init profile: %v", err)
	}
	mem, err := memory.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	defer mem.Close()
	schedule, err := scheduler.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("open scheduler: %v", err)
	}
	defer schedule.Close()
	schedulerStatus, err := schedule.Status(ctx, cfg.Scheduler.Enabled, cfg.Scheduler.MaxParallelJobs)
	if err != nil {
		t.Fatalf("scheduler status: %v", err)
	}
	report := Run(ctx, Input{
		Config:          cfg,
		Profile:         profile,
		Memory:          mem,
		Runtime:         fakeRuntime{models: []models.ModelInfo{{Name: "small:2b"}}},
		SchedulerStatus: &schedulerStatus,
		ExtensionStore:  extensions.NewStore(profile.GeneratedExtensions, profile.Logs),
	})
	if report.Overall != StatusOK {
		t.Fatalf("overall = %q, want ok; report=%+v", report.Overall, report)
	}
	if len(report.Checks) == 0 {
		t.Fatal("expected heartbeat checks")
	}

	store, err := Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("open heartbeat: %v", err)
	}
	defer store.Close()
	if err := store.Record(ctx, report); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	latest, err := store.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if len(latest.Checks) != len(report.Checks) {
		t.Fatalf("latest checks = %d, want %d", len(latest.Checks), len(report.Checks))
	}
}
