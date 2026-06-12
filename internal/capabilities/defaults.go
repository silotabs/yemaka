package capabilities

import (
	"fmt"
	"strings"
)

// PrimitiveStatus is a declared status for a core routing primitive. It is an
// alias so callers can use Ready, Disabled, NeedsConfig, or a full Capability.
type PrimitiveStatus = Capability

// DefaultLocalFirstSnapshot returns a deterministic capability registry for
// local-first routing. Optional network, connector, scheduler, crawler, and
// generation paths stay disabled unless explicitly declared otherwise.
func DefaultLocalFirstSnapshot(statuses ...PrimitiveStatus) (Registry, error) {
	var registry Registry
	for _, capability := range defaultLocalPrimitiveCapabilities() {
		if err := registry.upsertCapability(capability); err != nil {
			return Registry{}, err
		}
	}
	for _, status := range statuses {
		capability := Capability(status)
		if capability.Kind == "" {
			capability.Kind = inferPrimitiveKind(capability.Name)
		}
		if strings.TrimSpace(capability.Name) == "" {
			return Registry{}, fmt.Errorf("primitive status name is required")
		}
		if existing, ok := registry.Get(capability.Kind, capability.Name); ok {
			capability = mergeCapability(existing, capability)
		}
		if err := registry.upsertCapability(capability); err != nil {
			return Registry{}, err
		}
	}
	return registry.Snapshot(), nil
}

func defaultLocalPrimitiveCapabilities() []Capability {
	return []Capability{
		{
			Name:        "filesystem_read",
			Kind:        KindTool,
			Description: "Read files and search the approved local workspace.",
			State:       StateReady,
			Source:      "core",
			Aliases:     []string{"file read", "read_file", "search_files", "file_stat", "file_tree", "workspace read", "workspace search"},
			Tags:        []string{"local", "workspace"},
			Provides:    []string{"file_read", "filesystem_read", "read_file", "search_files", "file_stat", "file_tree", "workspace_read"},
		},
		{
			Name:             "filesystem_write",
			Kind:             KindTool,
			Description:      "Write files through the snapshot, diff, confirmation, and rollback flow.",
			State:            StateReady,
			RequiresApproval: true,
			Source:           "core",
			Aliases:          []string{"edit_file", "file write", "patch preview"},
			Tags:             []string{"local", "safety", "workspace"},
			Provides:         []string{"edit_file", "file_write", "filesystem_write", "patch_preview"},
		},
		{
			Name:        "local_time",
			Kind:        KindTool,
			Description: "Resolve local time for known locations without network access.",
			State:       StateReady,
			Source:      "core",
			Aliases:     []string{"time", "timezone"},
			Tags:        []string{"local"},
			Provides:    []string{"local_time"},
		},
		{
			Name:        "memory",
			Kind:        KindTool,
			Description: "Search local SQLite memory.",
			State:       StateReady,
			Source:      "core",
			Aliases:     []string{"memory search", "memory_search", "profile memory"},
			Tags:        []string{"local", "sqlite"},
			Provides:    []string{"memory", "memory_search"},
		},
		{
			Name:        "rag",
			Kind:        KindTool,
			Description: "Search local documents through SQLite FTS-backed RAG.",
			State:       StateReady,
			Source:      "core",
			Aliases:     []string{"document search", "local documents", "rag_search"},
			Tags:        []string{"fts5", "local", "sqlite"},
			Provides:    []string{"local_documents", "rag", "rag_search"},
		},
		{
			Name:             "shell",
			Kind:             KindTool,
			Description:      "Run approved local tool and shell operations.",
			State:            StateReady,
			RequiresApproval: true,
			Source:           "core",
			Aliases:          []string{"safe_tool", "shell tool", "shell_tool", "git status"},
			Tags:             []string{"local", "safety"},
			Provides:         []string{"git_status", "git_diff", "run_tests", "safe_tool", "shell", "shell_tool"},
		},
		{
			Name:        "skills",
			Kind:        KindSkill,
			Description: "Use installed local skills.",
			State:       StateReady,
			Source:      "profile",
			Aliases:     []string{"local skills", "skill action", "skill_action"},
			Tags:        []string{"local"},
			Provides:    []string{"skill_action", "skills"},
		},
		{
			Name:             "extension_generation",
			Kind:             KindExtension,
			Description:      "Generate profile-local extensions after manifest, policy, test, approval, audit, and rollback checks.",
			State:            StateDisabled,
			RequiresApproval: true,
			Source:           "profile",
			Aliases:          []string{"capability generation", "generated extension", "missing capability"},
			Tags:             []string{"generated", "local", "safety"},
			Provides:         []string{"extension_generation", "generated_extension"},
			Reason:           "generated extensions are disabled until explicitly enabled for this profile",
			ConfigureHint:    "Enable generated extensions only through the manifest, policy, tests, approval, audit, and rollback flow.",
		},
		{
			Name:          "internet",
			Kind:          KindInternetProvider,
			Description:   "Controlled internet search, fetch, and HEAD through an explicitly configured provider.",
			State:         StateDisabled,
			Source:        "profile",
			Aliases:       []string{"internet fetch", "internet head", "internet search", "searxng", "web search"},
			Tags:          []string{"optional", "network"},
			Provides:      []string{"internet", "internet_fetch", "internet_head", "internet_search", "search_web"},
			Reason:        "internet access is disabled until a provider is configured and enabled",
			ConfigureHint: "Configure a SearXNG-compatible provider and explicitly enable internet for this profile.",
		},
		{
			Name:          "connector",
			Kind:          KindConnector,
			Description:   "Optional connector actions through configured connector manifests.",
			State:         StateDisabled,
			Source:        "profile",
			Aliases:       []string{"connector action", "connector_action", "connectors", "mcp"},
			Tags:          []string{"optional"},
			Provides:      []string{"connector", "connector_action"},
			Reason:        "connectors are disabled by default",
			ConfigureHint: "Enable only the needed connector and store credentials as secret references.",
		},
		{
			Name:             "scheduler",
			Kind:             KindScheduler,
			Description:      "Create scheduled jobs only when scheduler support is explicitly enabled.",
			State:            StateDisabled,
			RequiresApproval: true,
			Source:           "profile",
			Aliases:          []string{"cron", "job scheduler", "scheduled jobs"},
			Tags:             []string{"optional", "background"},
			Provides:         []string{"cron", "scheduler", "scheduler_create", "schedule_job"},
			Reason:           "scheduler support is disabled by default",
			ConfigureHint:    "Enable scheduler support explicitly; new jobs still require approval.",
		},
		{
			Name:          "notification_center",
			Kind:          KindNotification,
			Description:   "Local notification inbox for jobs, approvals, monitor results, and agent follow-ups.",
			State:         StateDisabled,
			Source:        "profile",
			Aliases:       []string{"notifications", "notification center", "local inbox", "notify me"},
			Tags:          []string{"local", "optional"},
			Provides:      []string{"notification_center", "notifications", "notify_user"},
			Reason:        "notifications are disabled until a workflow or job explicitly uses the local inbox",
			ConfigureHint: "Enable notifications only for approved local workflows or scheduled jobs.",
		},
		{
			Name:             "crawler",
			Kind:             KindCrawler,
			Description:      "Run crawl tasks only through explicitly configured crawler support.",
			State:            StateDisabled,
			RequiresApproval: true,
			Source:           "profile",
			Aliases:          []string{"crawler task", "crawler_task", "site crawler"},
			Tags:             []string{"optional", "network"},
			Provides:         []string{"crawl_website", "crawler", "crawler_task"},
			Reason:           "crawler support is disabled by default",
			ConfigureHint:    "Configure crawler support only for approved crawl tasks; normal fetch/search remains separate.",
		},
	}
}

func (r *Registry) upsertCapability(capability Capability) error {
	normalized, err := normalizeCapability(capability)
	if err != nil {
		return err
	}
	switch normalized.Kind {
	case KindScheduler:
		r.Scheduler = normalized
	case KindCrawler:
		r.Crawler = normalized
	case KindModelRole:
		role := modelRoleFromCapability(ModelRole{}, normalized)
		if index := modelRoleIndex(r.ModelRoles, role.Role); index >= 0 {
			r.ModelRoles[index] = role
		} else {
			r.ModelRoles = append(r.ModelRoles, role)
		}
		sortModelRoles(r.ModelRoles)
	case KindExtension:
		if normalizeKey(normalized.Name) == "extension_generation" {
			r.ExtensionGeneration = normalized
			return nil
		}
		fallthrough
	default:
		list := r.listForKind(normalized.Kind)
		if index := capabilityIndex(*list, normalized.Kind, normalized.Name); index >= 0 {
			(*list)[index] = normalized
		} else {
			*list = append(*list, normalized)
		}
		sortCapabilities(*list)
	}
	return nil
}

func mergeCapability(base Capability, override Capability) Capability {
	out := base
	if strings.TrimSpace(override.Name) != "" {
		out.Name = strings.TrimSpace(override.Name)
	}
	if override.Kind != "" {
		out.Kind = override.Kind
	}
	if strings.TrimSpace(override.Description) != "" {
		out.Description = strings.TrimSpace(override.Description)
	}
	if strings.TrimSpace(override.Source) != "" {
		out.Source = strings.TrimSpace(override.Source)
	}
	statusSpecified := override.State != "" || override.Available || override.Enabled || override.Configured
	if statusSpecified {
		out.State = override.State
		out.Available = override.Available
		out.Enabled = override.Enabled
		out.Configured = override.Configured
		if override.State == StateReady && strings.TrimSpace(override.Reason) == "" {
			out.Reason = ""
			out.ConfigureHint = ""
		}
	}
	if override.RequiresApproval {
		out.RequiresApproval = true
	}
	out.Aliases = append(out.Aliases, override.Aliases...)
	out.Tags = append(out.Tags, override.Tags...)
	out.Provides = append(out.Provides, override.Provides...)
	if strings.TrimSpace(override.Reason) != "" {
		out.Reason = strings.TrimSpace(override.Reason)
	}
	if strings.TrimSpace(override.ConfigureHint) != "" {
		out.ConfigureHint = strings.TrimSpace(override.ConfigureHint)
	}
	out.Metadata = mergeMetadata(out.Metadata, override.Metadata)
	normalized, err := normalizeCapability(out)
	if err != nil {
		return out
	}
	return normalized
}

func inferPrimitiveKind(name string) Kind {
	switch normalizeKey(name) {
	case "connector", "connector_action", "connectors", "mcp":
		return KindConnector
	case "crawler", "crawler_task", "site_crawler":
		return KindCrawler
	case "domain_pack":
		return KindDomainPack
	case "capability_pack", "pack":
		return KindCapabilityPack
	case "extension_generation", "generated_extension":
		return KindExtension
	case "internet", "internet_fetch", "internet_head", "internet_search", "search_web", "searxng", "web_search":
		return KindInternetProvider
	case "model", "model_role", "coding_model", "reasoning_model":
		return KindModelRole
	case "notification", "notifications", "notification_center", "notify_user":
		return KindNotification
	case "scheduler", "scheduler_create", "schedule_job", "cron":
		return KindScheduler
	case "skill", "skills", "skill_action":
		return KindSkill
	default:
		return KindTool
	}
}

func capabilityIndex(items []Capability, kind Kind, name string) int {
	key := normalizeKey(name)
	for i, item := range items {
		if item.Kind == kind && normalizeKey(item.Name) == key {
			return i
		}
	}
	return -1
}

func modelRoleIndex(items []ModelRole, role string) int {
	key := normalizeKey(role)
	for i, item := range items {
		if normalizeKey(item.Role) == key {
			return i
		}
	}
	return -1
}

func mergeMetadata(base map[string]string, override map[string]string) map[string]string {
	out := copyMetadata(base)
	for key, value := range override {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		if out == nil {
			out = map[string]string{}
		}
		out[key] = value
	}
	return out
}

func copyMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
