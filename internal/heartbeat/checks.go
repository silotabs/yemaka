package heartbeat

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"

	"yemaka/internal/config"
	"yemaka/internal/connectors"
	"yemaka/internal/extensions"
	"yemaka/internal/internet"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
	"yemaka/internal/scheduler"
)

type Input struct {
	Config          *config.Config
	Profile         *profiles.Profile
	Memory          *memory.Store
	Runtime         models.Runtime
	SchedulerStatus *scheduler.Status
	ExtensionStore  *extensions.Store
}

func Run(ctx context.Context, input Input) Report {
	generatedAt := time.Now().UTC().Format(time.RFC3339)
	report := Report{
		Overall:     StatusOK,
		GeneratedAt: generatedAt,
	}
	add := func(name string, status string, detail string) {
		check := Check{
			ID:        "hb_" + uuid.NewString(),
			Name:      name,
			Status:    status,
			Detail:    detail,
			CheckedAt: generatedAt,
		}
		report.Checks = append(report.Checks, check)
		report.Overall = combine(report.Overall, status)
	}

	cfg := input.Config
	if cfg == nil {
		add("config", StatusNeedsConfig, "config unavailable")
		return report
	}
	checks := cfg.Heartbeat.Checks
	if checks.SQLite {
		if input.Memory == nil {
			add("sqlite", StatusError, "memory store unavailable")
		} else if err := input.Memory.Ping(ctx); err != nil {
			add("sqlite", StatusError, err.Error())
		} else {
			add("sqlite", StatusOK, "memory database reachable")
		}
	}
	if checks.Ollama {
		if input.Runtime == nil {
			add("ollama", StatusError, "runtime unavailable")
		} else if err := input.Runtime.Health(ctx); err != nil {
			add("ollama", StatusError, err.Error())
		} else {
			modelsList, err := input.Runtime.ListModels(ctx)
			if err != nil {
				add("models", StatusWarn, err.Error())
			} else {
				selected := selectedModelName(cfg)
				if models.ModelInstalled(selected, modelsList) {
					add("models", StatusOK, fmt.Sprintf("%s installed", selected))
				} else {
					add("models", StatusWarn, fmt.Sprintf("%s not installed", selected))
				}
			}
			add("ollama", StatusOK, "runtime reachable")
		}
	}
	if checks.Scheduler {
		if input.SchedulerStatus == nil {
			status := StatusDisabled
			if cfg.Scheduler.Enabled {
				status = StatusNeedsConfig
			}
			add("scheduler", status, "scheduler status unavailable")
		} else {
			status := StatusOK
			detail := fmt.Sprintf("%d jobs, %d enabled, max parallel %d", input.SchedulerStatus.TotalJobs, input.SchedulerStatus.EnabledJobs, input.SchedulerStatus.MaxParallelJobs)
			if !input.SchedulerStatus.Enabled {
				status = StatusDisabled
				detail = "scheduler disabled"
			} else if input.SchedulerStatus.LastRunStatus == scheduler.StatusFailed || input.SchedulerStatus.LastRunStatus == scheduler.StatusTimeout {
				status = StatusWarn
				detail = fmt.Sprintf("%s; last run %s at %s", detail, input.SchedulerStatus.LastRunStatus, input.SchedulerStatus.LastRunAt)
			}
			add("scheduler", status, detail)
		}
	}
	if checks.Internet {
		if cfg.Internet.Enabled {
			searchProvider := strings.ToLower(strings.TrimSpace(cfg.Internet.Search.Provider))
			if cfg.Internet.Search.Enabled && (searchProvider == "" || searchProvider == "none" || searchProvider == "off" || searchProvider == "disabled") {
				add("internet", StatusNeedsConfig, "search enabled without a configured provider")
			} else if cfg.Internet.Search.Enabled && (searchProvider == "searx" || searchProvider == "searxng") && strings.TrimSpace(cfg.Internet.Search.Endpoint) == "" {
				add("internet", StatusNeedsConfig, "searxng search enabled without an endpoint")
			} else if cfg.Internet.Search.Enabled && (searchProvider == "brave" || searchProvider == "brave_search" || searchProvider == "firecrawl" || searchProvider == "firecrawl_search" || searchProvider == "mojeek") {
				apiKeyEnv := strings.TrimSpace(cfg.Internet.Search.APIKeyEnv)
				label := "brave"
				if searchProvider == "firecrawl" || searchProvider == "firecrawl_search" {
					label = "firecrawl"
				} else if searchProvider == "mojeek" {
					label = "mojeek"
				}
				if apiKeyEnv == "" {
					add("internet", StatusNeedsAuth, fmt.Sprintf("%s search enabled without an api key env", label))
				} else if err := internet.ValidateSearchAPIKeyEnvForProvider(searchProvider, apiKeyEnv); err != nil {
					add("internet", StatusNeedsAuth, err.Error())
				} else if strings.TrimSpace(os.Getenv(apiKeyEnv)) == "" {
					add("internet", StatusNeedsAuth, fmt.Sprintf("%s search api key env %s is not set or not visible to Yemaka; restart the app after setting it", label, apiKeyEnv))
				} else {
					add("internet", StatusOK, "enabled for profile")
				}
			} else {
				add("internet", StatusOK, "enabled for profile")
			}
		} else {
			add("internet", StatusDisabled, "disabled by default")
		}
	}
	if checks.Connectors {
		statuses := connectors.List(cfg.Connectors)
		enabled := 0
		missingToken := 0
		for _, status := range statuses {
			if !status.Enabled {
				continue
			}
			enabled++
			if !status.TokenReady {
				missingToken++
			}
		}
		switch {
		case enabled == 0:
			add("connectors", StatusDisabled, "no connectors enabled")
		case missingToken > 0:
			add("connectors", StatusNeedsAuth, fmt.Sprintf("%d enabled, %d missing token", enabled, missingToken))
		default:
			add("connectors", StatusOK, fmt.Sprintf("%d enabled", enabled))
		}
	}
	if checks.Extensions {
		if input.ExtensionStore == nil {
			add("extensions", StatusNeedsConfig, "extension registry unavailable")
		} else {
			items, err := input.ExtensionStore.List()
			if err != nil {
				add("extensions", StatusWarn, err.Error())
			} else {
				callable := 0
				for _, item := range items {
					if item.Callable {
						callable++
					}
				}
				add("extensions", StatusOK, fmt.Sprintf("%d registered, %d callable", len(items), callable))
			}
		}
	}
	if checks.DiskSpace {
		root := ""
		if input.Profile != nil {
			root = input.Profile.Root
		}
		if strings.TrimSpace(root) == "" {
			add("disk_space", StatusNeedsConfig, "profile path unavailable")
		} else if _, err := os.Stat(root); err != nil {
			add("disk_space", StatusError, err.Error())
		} else {
			add("disk_space", StatusOK, root)
		}
	}
	if checks.MemoryPressure {
		var stats runtime.MemStats
		runtime.ReadMemStats(&stats)
		add("memory_pressure", StatusOK, fmt.Sprintf("go heap %.1f MB", float64(stats.Alloc)/1024/1024))
	}
	return report
}

func combine(current string, next string) string {
	if statusSeverity(next) > statusSeverity(current) {
		return next
	}
	if current == "" {
		return next
	}
	return current
}

func statusSeverity(status string) int {
	switch status {
	case StatusError:
		return 5
	case StatusNeedsAuth, StatusNeedsConfig:
		return 4
	case StatusWarn:
		return 3
	case StatusOK:
		return 2
	case StatusDisabled:
		return 1
	default:
		return 0
	}
}

func selectedModelName(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
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
