package diagnostics

import (
	"context"
	"fmt"
	"sort"

	"yemaka/internal/config"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
)

type Report struct {
	ConfigPath      string
	ProfilePath     string
	SQLitePath      string
	SQLiteOK        bool
	SQLiteError     string
	OllamaOK        bool
	OllamaError     string
	DefaultModel    string
	LowMemoryModel  string
	SelectedModel   string
	LowMemoryMode   bool
	SetupComplete   bool
	ModelReady      bool
	RAGEnabled      bool
	ShellEnabled    bool
	CloudFallback   bool
	ConfigTelemetry bool
	ModelListError  string
	ModelStatuses   []ModelStatus
}

type ModelStatus struct {
	Role      string
	Name      string
	Installed bool
}

func Run(ctx context.Context, cfg *config.Config, profile *profiles.Profile, store *memory.Store, runtime models.Runtime) Report {
	report := Report{
		ConfigPath:      cfg.Path,
		ProfilePath:     profile.Root,
		SQLitePath:      profile.Database,
		LowMemoryMode:   cfg.Runtime.LowMemoryMode,
		SetupComplete:   cfg.App.SetupComplete,
		RAGEnabled:      cfg.RAG.Enabled,
		ShellEnabled:    cfg.Tools.Shell.Enabled,
		CloudFallback:   cfg.CloudFallback.Enabled,
		ConfigTelemetry: cfg.App.Telemetry,
	}
	if model, ok := cfg.Models["default"]; ok {
		report.DefaultModel = model.Name
	}
	if model, ok := cfg.Models["low_memory"]; ok {
		report.LowMemoryModel = model.Name
	}
	report.SelectedModel = selectedModelName(cfg)

	if err := store.Ping(ctx); err != nil {
		report.SQLiteError = err.Error()
	} else {
		report.SQLiteOK = true
	}

	if err := runtime.Health(ctx); err != nil {
		report.OllamaError = err.Error()
	} else {
		report.OllamaOK = true
		models, err := runtime.ListModels(ctx)
		if err != nil {
			report.ModelListError = err.Error()
		} else {
			report.ModelStatuses = configuredModelStatuses(cfg, models)
			report.ModelReady = configuredModelInstalled(report.SelectedModel, models)
		}
	}

	return report
}

func selectedModelName(cfg *config.Config) string {
	if cfg.Runtime.LowMemoryMode {
		if model, ok := cfg.Models["low_memory"]; ok {
			return model.Name
		}
	}
	if model, ok := cfg.Models["default"]; ok {
		return model.Name
	}
	return ""
}

func configuredModelInstalled(name string, installed []models.ModelInfo) bool {
	return models.ModelInstalled(name, installed)
}

func Status(ok bool, err string) string {
	if ok {
		return "ok"
	}
	if err == "" {
		return "not ok"
	}
	return fmt.Sprintf("not ok (%s)", err)
}

func configuredModelStatuses(cfg *config.Config, installed []models.ModelInfo) []ModelStatus {
	installedByName := map[string]bool{}
	for _, model := range models.LocalInstalledModels(installed) {
		installedByName[model.Name] = true
	}

	roleOrder := map[string]int{
		"default":        0,
		"low_memory":     1,
		"coding":         2,
		"reasoning":      3,
		"stronger_local": 4,
	}
	roles := make([]string, 0, len(cfg.Models))
	for role := range cfg.Models {
		roles = append(roles, role)
	}
	sort.Slice(roles, func(i, j int) bool {
		left, leftOK := roleOrder[roles[i]]
		right, rightOK := roleOrder[roles[j]]
		if leftOK && rightOK {
			return left < right
		}
		if leftOK {
			return true
		}
		if rightOK {
			return false
		}
		return roles[i] < roles[j]
	})

	statuses := make([]ModelStatus, 0, len(roles))
	for _, role := range roles {
		model := cfg.Models[role]
		statuses = append(statuses, ModelStatus{
			Role:      role,
			Name:      model.Name,
			Installed: installedByName[model.Name] || models.ModelInstalled(model.Name, installed),
		})
	}
	return statuses
}
