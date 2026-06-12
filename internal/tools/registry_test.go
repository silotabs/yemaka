package tools

import "testing"

func TestToolRegistryClassifiesPostRCReadOnlyTools(t *testing.T) {
	tools := ToolCapabilitiesByName()
	for _, name := range []string{"file_stat", "file_tree", "git_status", "heartbeat_status"} {
		capability, ok := tools[name]
		if !ok {
			t.Fatalf("registry missing %s", name)
		}
		if !hasToolSurface(capability.Surfaces, ToolSurfaceChat) {
			t.Fatalf("%s surfaces = %#v, want chat-callable", name, capability.Surfaces)
		}
		if !capability.ModelCallable {
			t.Fatalf("%s ModelCallable = false, want true", name)
		}
		if capability.Mutating || capability.RequiresApproval {
			t.Fatalf("%s should be low-risk read-only: %#v", name, capability)
		}
	}
}

func TestToolRegistrySkillSurfaceIncludesKnownTool(t *testing.T) {
	available := ToolAvailabilityForSurface(ToolSurfaceSkill)
	for _, name := range []string{"read_file", "file_stat", "file_tree", "git_status", "heartbeat_status", "edit_file"} {
		if !available[name] {
			t.Fatalf("skill surface missing %s: %#v", name, available)
		}
		if !KnownTool(name) {
			t.Fatalf("KnownTool(%q) = false, want true", name)
		}
	}
	if available["extension_run"] {
		t.Fatalf("skill surface should not include future extension_run: %#v", available)
	}
}

func TestToolCatalogForChatSurfaceClassifiesVisibility(t *testing.T) {
	catalog := toolCatalogByName(ToolCatalogForSurface(ToolSurfaceChat))
	cases := map[string]ToolStatus{
		"read_file":         ToolStatusAvailable,
		"edit_file":         ToolStatusApprovalRequired,
		"memory_write":      ToolStatusApprovalRequired,
		"internet_search":   ToolStatusDisabledByPolicy,
		"internet_crawl":    ToolStatusDisabledByPolicy,
		"write_file":        ToolStatusCLIOnly,
		"run_shell_safe":    ToolStatusCLIOnly,
		"workspace_summary": ToolStatusSkillOnly,
		"extension_run":     ToolStatusFuture,
	}
	for name, want := range cases {
		entry, ok := catalog[name]
		if !ok {
			t.Fatalf("catalog missing %s", name)
		}
		if entry.Status != want {
			t.Fatalf("%s status = %q, want %q", name, entry.Status, want)
		}
	}
	if !catalog["read_file"].ChatCallable || !catalog["read_file"].ModelCallable {
		t.Fatalf("read_file catalog entry = %#v, want chat and model callable", catalog["read_file"])
	}
	if !catalog["edit_file"].ChatCallable || !catalog["edit_file"].RequiresApproval {
		t.Fatalf("edit_file catalog entry = %#v, want chat approval-required", catalog["edit_file"])
	}
	if catalog["internet_search"].EnabledByDefault {
		t.Fatalf("internet_search catalog entry = %#v, want disabled by default", catalog["internet_search"])
	}
	if catalog["internet_crawl"].ModelCallable || !catalog["internet_crawl"].RequiresApproval {
		t.Fatalf("internet_crawl catalog entry = %#v, want approval-gated and not model-callable", catalog["internet_crawl"])
	}
}

func TestToolStatusForSurfaceSupportsExtensionOnly(t *testing.T) {
	capability := ToolCapability{
		Name:             "generated_csv_cleanup",
		Surfaces:         []ToolSurface{ToolSurfaceExtension},
		EnabledByDefault: true,
	}
	if got := ToolStatusForSurface(capability, ToolSurfaceChat); got != ToolStatusExtensionOnly {
		t.Fatalf("extension-only status = %q, want %q", got, ToolStatusExtensionOnly)
	}
}

func toolCatalogByName(entries []ToolCatalogEntry) map[string]ToolCatalogEntry {
	out := make(map[string]ToolCatalogEntry, len(entries))
	for _, entry := range entries {
		out[entry.Name] = entry
	}
	return out
}
