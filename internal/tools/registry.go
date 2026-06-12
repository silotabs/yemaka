package tools

import "sort"

type ToolSurface string

const (
	ToolSurfaceChat      ToolSurface = "chat-callable"
	ToolSurfaceCLI       ToolSurface = "cli"
	ToolSurfaceSkill     ToolSurface = "skill"
	ToolSurfaceExtension ToolSurface = "extension"
	ToolSurfaceFuture    ToolSurface = "future"
)

type ToolStatus string

const (
	ToolStatusAvailable        ToolStatus = "available"
	ToolStatusApprovalRequired ToolStatus = "approval-required"
	ToolStatusDisabledByPolicy ToolStatus = "disabled-by-policy"
	ToolStatusCLIOnly          ToolStatus = "cli-only"
	ToolStatusExtensionOnly    ToolStatus = "extension-only"
	ToolStatusSkillOnly        ToolStatus = "skill-only"
	ToolStatusFuture           ToolStatus = "future"
)

type ToolCapability struct {
	Name             string
	Surfaces         []ToolSurface
	ModelCallable    bool
	RequiresApproval bool
	EnabledByDefault bool
	Mutating         bool
	Notes            string
}

type ToolCatalogEntry struct {
	Name             string
	Status           ToolStatus
	Surfaces         []ToolSurface
	ChatCallable     bool
	ModelCallable    bool
	RequiresApproval bool
	EnabledByDefault bool
	Mutating         bool
	Notes            string
}

func ToolRegistry() []ToolCapability {
	return []ToolCapability{
		{Name: "workspace_summary", Surfaces: []ToolSurface{ToolSurfaceSkill}, EnabledByDefault: true},
		{Name: "read_file", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "list_files", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "file_stat", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "file_tree", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "search_files", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "write_file", Surfaces: []ToolSurface{ToolSurfaceCLI, ToolSurfaceSkill}, RequiresApproval: true, Mutating: true, EnabledByDefault: true, Notes: "chat writes use edit_file proposal flow"},
		{Name: "edit_file", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceCLI, ToolSurfaceSkill}, RequiresApproval: true, Mutating: true, EnabledByDefault: true},
		{Name: "rag_search", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "ingest_documents", Surfaces: []ToolSurface{ToolSurfaceChat}, EnabledByDefault: true},
		{Name: "memory_search", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, EnabledByDefault: true},
		{Name: "memory_write", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, RequiresApproval: true, Mutating: true, EnabledByDefault: true},
		{Name: "git_status", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceCLI, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "git_diff", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceCLI, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "run_tests", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceCLI, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "run_shell_safe", Surfaces: []ToolSurface{ToolSurfaceCLI, ToolSurfaceSkill}, RequiresApproval: true, EnabledByDefault: true},
		{Name: "local_time", Surfaces: []ToolSurface{ToolSurfaceChat}, EnabledByDefault: true},
		{Name: "doctor_status", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceCLI, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "heartbeat_status", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceCLI, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "project_map", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "patch_preview", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "symbol_search", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "secret_scan", Surfaces: []ToolSurface{ToolSurfaceChat, ToolSurfaceSkill}, ModelCallable: true, EnabledByDefault: true},
		{Name: "internet_fetch", Surfaces: []ToolSurface{ToolSurfaceChat}, ModelCallable: true, EnabledByDefault: false, Notes: "requires internet profile policy"},
		{Name: "internet_head", Surfaces: []ToolSurface{ToolSurfaceChat}, ModelCallable: true, EnabledByDefault: false, Notes: "requires internet profile policy"},
		{Name: "internet_search", Surfaces: []ToolSurface{ToolSurfaceChat}, ModelCallable: true, EnabledByDefault: false, Notes: "requires internet profile policy and provider"},
		{Name: "internet_crawl", Surfaces: []ToolSurface{ToolSurfaceChat}, RequiresApproval: true, EnabledByDefault: false, Notes: "manual bounded crawler; not model-callable"},
		{Name: "create_skill", Surfaces: []ToolSurface{ToolSurfaceCLI, ToolSurfaceSkill}, RequiresApproval: true, Mutating: true, EnabledByDefault: true},
		{Name: "load_skill", Surfaces: []ToolSurface{ToolSurfaceCLI, ToolSurfaceSkill}, EnabledByDefault: true},
		{Name: "extension_run", Surfaces: []ToolSurface{ToolSurfaceFuture}, RequiresApproval: true, EnabledByDefault: false},
		{Name: "job_run", Surfaces: []ToolSurface{ToolSurfaceFuture}, RequiresApproval: true, EnabledByDefault: false},
	}
}

func ToolCapabilitiesByName() map[string]ToolCapability {
	out := make(map[string]ToolCapability)
	for _, capability := range ToolRegistry() {
		out[capability.Name] = capability
	}
	return out
}

func ToolCatalog() []ToolCatalogEntry {
	return ToolCatalogForSurface("")
}

func ToolCatalogForSurface(surface ToolSurface) []ToolCatalogEntry {
	entries := make([]ToolCatalogEntry, 0, len(ToolRegistry()))
	for _, capability := range ToolRegistry() {
		entries = append(entries, toolCatalogEntry(capability, surface))
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries
}

func ToolStatusForSurface(capability ToolCapability, surface ToolSurface) ToolStatus {
	if isFutureOnly(capability) {
		return ToolStatusFuture
	}
	if surface != "" && !hasToolSurface(capability.Surfaces, surface) {
		switch {
		case hasToolSurface(capability.Surfaces, ToolSurfaceExtension):
			return ToolStatusExtensionOnly
		case hasToolSurface(capability.Surfaces, ToolSurfaceCLI):
			return ToolStatusCLIOnly
		case hasToolSurface(capability.Surfaces, ToolSurfaceSkill):
			return ToolStatusSkillOnly
		}
	}
	if !capability.EnabledByDefault {
		return ToolStatusDisabledByPolicy
	}
	if capability.RequiresApproval {
		return ToolStatusApprovalRequired
	}
	if isExtensionOnly(capability) {
		return ToolStatusExtensionOnly
	}
	if isCLIOnly(capability) {
		return ToolStatusCLIOnly
	}
	if isSkillOnly(capability) {
		return ToolStatusSkillOnly
	}
	return ToolStatusAvailable
}

func ToolNamesForSurface(surface ToolSurface) []string {
	var names []string
	for _, capability := range ToolRegistry() {
		if hasToolSurface(capability.Surfaces, surface) {
			names = append(names, capability.Name)
		}
	}
	sort.Strings(names)
	return names
}

func ToolAvailabilityForSurface(surface ToolSurface) map[string]bool {
	out := map[string]bool{}
	for _, name := range ToolNamesForSurface(surface) {
		out[name] = true
	}
	return out
}

func ModelObservationToolNames() []string {
	var names []string
	for _, capability := range ToolRegistry() {
		if capability.ModelCallable {
			names = append(names, capability.Name)
		}
	}
	sort.Strings(names)
	return names
}

func ModelObservationToolAvailable(name string) bool {
	capability, ok := ToolCapabilitiesByName()[name]
	return ok && capability.ModelCallable
}

func KnownTool(name string) bool {
	capability, ok := ToolCapabilitiesByName()[name]
	return ok && !isFutureOnly(capability)
}

func toolCatalogEntry(capability ToolCapability, surface ToolSurface) ToolCatalogEntry {
	return ToolCatalogEntry{
		Name:             capability.Name,
		Status:           ToolStatusForSurface(capability, surface),
		Surfaces:         append([]ToolSurface(nil), capability.Surfaces...),
		ChatCallable:     hasToolSurface(capability.Surfaces, ToolSurfaceChat),
		ModelCallable:    capability.ModelCallable,
		RequiresApproval: capability.RequiresApproval,
		EnabledByDefault: capability.EnabledByDefault,
		Mutating:         capability.Mutating,
		Notes:            capability.Notes,
	}
}

func hasToolSurface(surfaces []ToolSurface, want ToolSurface) bool {
	for _, surface := range surfaces {
		if surface == want {
			return true
		}
	}
	return false
}

func isFutureOnly(capability ToolCapability) bool {
	return len(capability.Surfaces) == 1 && capability.Surfaces[0] == ToolSurfaceFuture
}

func isExtensionOnly(capability ToolCapability) bool {
	return len(capability.Surfaces) == 1 && capability.Surfaces[0] == ToolSurfaceExtension
}

func isCLIOnly(capability ToolCapability) bool {
	return len(capability.Surfaces) == 1 && capability.Surfaces[0] == ToolSurfaceCLI
}

func isSkillOnly(capability ToolCapability) bool {
	return len(capability.Surfaces) == 1 && capability.Surfaces[0] == ToolSurfaceSkill
}
