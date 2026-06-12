package capabilities

import "strings"

// RouteMetadata is a package-local, dependency-free view of routing metadata.
// It intentionally mirrors stable route concepts without importing agent code.
type RouteMetadata struct {
	Request             string   `json:"request,omitempty"`
	RouteCategory       string   `json:"routeCategory,omitempty"`
	Intent              string   `json:"intent,omitempty"`
	Domain              string   `json:"domain,omitempty"`
	Target              string   `json:"target,omitempty"`
	Capability          string   `json:"capability,omitempty"`
	Tools               []string `json:"tools,omitempty"`
	RequiresApproval    bool     `json:"requiresApproval,omitempty"`
	UseWorkspace        bool     `json:"useWorkspace,omitempty"`
	UseRAG              bool     `json:"useRAG,omitempty"`
	UseInternet         bool     `json:"useInternet,omitempty"`
	ReadsFiles          bool     `json:"readsFiles,omitempty"`
	WritesFiles         bool     `json:"writesFiles,omitempty"`
	GeneratesExtension  bool     `json:"generatesExtension,omitempty"`
	CreatesSchedulerJob bool     `json:"createsSchedulerJob,omitempty"`
	ConnectorAction     bool     `json:"connectorAction,omitempty"`
	CrawlerTask         bool     `json:"crawlerTask,omitempty"`
	RunsShell           bool     `json:"runsShell,omitempty"`
}

// RouteGapQuery turns route metadata into a deterministic registry query.
func RouteGapQuery(route RouteMetadata) GapQuery {
	route = normalizeRouteMetadata(route)
	generates := routeRequestsGeneration(route)
	capability := route.Capability
	if generates && normalizeKey(capability) == "extension_generation" {
		capability = ""
	}
	if capability == "" && !generates {
		capability = inferredRouteCapability(route)
	}
	kind := routeKind(route, capability)
	if generates {
		kind = ""
	}
	target := route.Target
	if generates && isGenerationControlValue(target) {
		target = ""
	}
	requestParts := []string{
		route.Request,
		route.Intent,
		route.Domain,
		target,
		capability,
	}
	if !generates {
		requestParts = append(requestParts, route.RouteCategory)
	}
	requestParts = append(requestParts, route.Tools...)
	return GapQuery{
		Request:     strings.Join(normalizeList(requestParts), " "),
		Capability:  capability,
		Kind:        kind,
		Domain:      route.Domain,
		AllowCreate: generates,
	}
}

// ClassifyRouteGap classifies a capability gap directly from route metadata.
func (r Registry) ClassifyRouteGap(route RouteMetadata) GapDecision {
	return r.ClassifyGap(RouteGapQuery(route))
}

func normalizeRouteMetadata(route RouteMetadata) RouteMetadata {
	route.Request = strings.TrimSpace(route.Request)
	route.RouteCategory = strings.TrimSpace(route.RouteCategory)
	route.Intent = strings.TrimSpace(route.Intent)
	route.Domain = strings.TrimSpace(route.Domain)
	route.Target = strings.TrimSpace(route.Target)
	route.Capability = strings.TrimSpace(route.Capability)
	route.Tools = normalizeList(route.Tools)
	return route
}

func routeRequestsGeneration(route RouteMetadata) bool {
	category := normalizeKey(route.RouteCategory)
	return route.GeneratesExtension || category == "extension_generate"
}

func isGenerationControlValue(value string) bool {
	switch normalizeKey(value) {
	case "extension_generation", "generated_extension":
		return true
	default:
		return false
	}
}

func inferredRouteCapability(route RouteMetadata) string {
	category := normalizeKey(route.RouteCategory)
	switch {
	case route.CrawlerTask || category == "crawler_task":
		return "crawler"
	case route.CreatesSchedulerJob || category == "scheduler_create":
		return "scheduler"
	case route.ConnectorAction || category == "connector_action":
		return "connector"
	case route.WritesFiles || category == "file_write":
		return "filesystem_write"
	case route.RunsShell || category == "shell_tool":
		return "shell"
	case route.UseInternet || category == "internet_search" || category == "internet_fetch" || category == "internet_head" || hasTool(route.Tools, "internet_search") || hasTool(route.Tools, "internet_fetch") || hasTool(route.Tools, "internet_head"):
		return "internet"
	case route.UseRAG || category == "rag_search" || hasTool(route.Tools, "rag_search"):
		return "rag"
	case category == "memory_search" || hasTool(route.Tools, "memory_search"):
		return "memory"
	case category == "local_time" || hasTool(route.Tools, "local_time"):
		return "local_time"
	case route.ReadsFiles || route.UseWorkspace || category == "file_read" || category == "workspace_read" || hasTool(route.Tools, "read_file") || hasTool(route.Tools, "search_files"):
		return "filesystem_read"
	case category == "skill_action":
		return "skills"
	default:
		return ""
	}
}

func routeKind(route RouteMetadata, capability string) Kind {
	inferred := inferredRouteCapability(route)
	key := normalizeKey(nonEmpty(capability, inferred))
	if key == "" {
		return ""
	}
	switch key {
	case "connector":
		return KindConnector
	case "crawler":
		return KindCrawler
	case "scheduler":
		return KindScheduler
	case "internet":
		return KindInternetProvider
	case "skills":
		return KindSkill
	case "extension_generation", "generated_extension":
		return KindExtension
	}
	if capability != "" {
		return inferPrimitiveKind(capability)
	}
	if inferred != "" {
		return inferPrimitiveKind(inferred)
	}
	return ""
}

func hasTool(tools []string, want string) bool {
	want = normalizeKey(want)
	for _, tool := range tools {
		if normalizeKey(tool) == want {
			return true
		}
	}
	return false
}
