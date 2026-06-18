package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/internet"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/safety"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
	"yemaka/internal/workspace"
)

type SafeToolConfig struct {
	WorkspaceRoot            string
	TimeoutSeconds           int
	MaxOutputBytes           int
	MaxContextChars          int
	RequireConfirmationRisky bool
	FullAccess               bool
	DisableShellCommands     bool
	Config                   *config.Config
	Profile                  *profiles.Profile
	Memory                   *memory.Store
	RAG                      *rag.Store
	Runtime                  models.Runtime
	Skills                   skills.Registry
	Internet                 *internet.Service
	WorkspaceGrants          safety.WorkspaceGrantStore
	UseShellHelper           bool
	ShellHelperPath          string
}

func NewSafeToolExecutor(cfg SafeToolConfig) ToolExecutor {
	return func(ctx context.Context, decision ExecutionDecision) (ExecutionResult, error) {
		if result, handled, err := coreToolResult(ctx, cfg, decision); handled {
			return result, err
		}
		command, workspaceRoot, err := commandForDecision(cfg.WorkspaceRoot, decision)
		if err != nil {
			return ExecutionResult{}, err
		}
		if cfg.DisableShellCommands {
			return ExecutionResult{}, fmt.Errorf("shell-backed safe tool execution is disabled")
		}
		if strings.TrimSpace(workspaceRoot) == "" {
			workspaceRoot = cfg.WorkspaceRoot
		}
		policy := tools.CommandPolicy{
			WorkspaceRoot:            workspaceRoot,
			Timeout:                  time.Duration(cfg.TimeoutSeconds) * time.Second,
			MaxOutputBytes:           cfg.MaxOutputBytes,
			RequireConfirmationRisky: cfg.RequireConfirmationRisky,
			FullAccess:               cfg.FullAccess,
			UseHelper:                cfg.UseShellHelper,
			HelperPath:               cfg.ShellHelperPath,
		}
		analysis := tools.AnalyzeArgs(command, policy)
		if !analysis.Allowed {
			return ExecutionResult{}, fmt.Errorf("command blocked: %s", analysis.Reason)
		}
		result, err := tools.RunCommand(ctx, analysis, policy)
		status := "completed"
		if result.ExitCode != 0 {
			status = "failed"
		}
		if err != nil {
			status = "failed"
		}
		return ExecutionResult{
			Context:    "COMMAND RESULT:\n" + tools.FormatCommandResult(result, cfg.MaxContextChars),
			Sources:    []string{strings.Join(command, " ")},
			SourceKind: "tool",
			Status:     status,
		}, err
	}
}

func commandForDecision(root string, decision ExecutionDecision) ([]string, string, error) {
	switch decision.ToolName {
	case "run_tests":
		return tools.DetectTestCommandRootForRequest(root, testRequestFromDecision(decision))
	case "git_status":
		return []string{"git", "status", "--short"}, root, nil
	case "git_diff":
		return []string{"git", "diff"}, root, nil
	default:
		return nil, "", fmt.Errorf("no safe executor for tool: %s", decision.ToolName)
	}
}

func testRequestFromDecision(decision ExecutionDecision) string {
	if len(decision.Command) == 0 {
		return ""
	}
	if decision.Command[0] == "detect" {
		return strings.Join(decision.Command[1:], " ")
	}
	return strings.Join(decision.Command, " ")
}

func workspaceRootAndTargetForTool(cfg SafeToolConfig, target string) (string, string, *safety.WorkspaceAccess, error) {
	root := strings.TrimSpace(cfg.WorkspaceRoot)
	if root == "" {
		root = "."
	}
	target = strings.TrimSpace(target)
	if target == "" {
		target = "."
	}
	normalized := safety.NormalizeUserSuppliedPath(target)
	abs := normalized
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	abs = filepath.Clean(abs)
	currentRoot, err := safety.ResolveWorkspace(root)
	if err != nil {
		return "", "", nil, err
	}
	if pathWithinRoot(currentRoot, abs) {
		return currentRoot, abs, nil, nil
	}
	if strings.TrimSpace(cfg.WorkspaceGrants.Path) == "" {
		return "", "", nil, workspaceGrantRequiredError(abs)
	}
	access, err := safety.RequireWorkspaceAccessWithBookmark(currentRoot, abs, cfg.WorkspaceGrants)
	if err != nil {
		return "", "", nil, err
	}
	grantRoot := strings.TrimSpace(access.Grant.Path)
	if grantRoot == "" {
		access.Close()
		return "", "", nil, workspaceGrantRequiredError(abs)
	}
	rel, err := filepath.Rel(grantRoot, abs)
	if err != nil {
		access.Close()
		return "", "", nil, fmt.Errorf("resolve granted workspace path: %w", err)
	}
	return grantRoot, filepath.ToSlash(rel), access, nil
}

func workspaceRootAndWriteTargetForTool(cfg SafeToolConfig, target string) (string, string, *safety.WorkspaceAccess, error) {
	root := strings.TrimSpace(cfg.WorkspaceRoot)
	if root == "" {
		root = "."
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return "", "", nil, fmt.Errorf("file path is required")
	}
	normalized := safety.NormalizeUserSuppliedPath(target)
	abs := normalized
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	abs = filepath.Clean(abs)
	currentRoot, err := safety.ResolveWorkspace(root)
	if err != nil {
		return "", "", nil, err
	}
	if pathWithinRoot(currentRoot, abs) {
		rel, err := filepath.Rel(currentRoot, abs)
		if err != nil {
			return "", "", nil, fmt.Errorf("resolve workspace write path: %w", err)
		}
		return currentRoot, filepath.ToSlash(rel), nil, nil
	}
	if strings.TrimSpace(cfg.WorkspaceGrants.Path) == "" {
		return "", "", nil, workspaceGrantRequiredError(abs)
	}
	accessPath, err := existingWriteAccessPath(abs)
	if err != nil {
		return "", "", nil, err
	}
	access, err := safety.RequireWorkspaceAccessWithBookmark(currentRoot, accessPath, cfg.WorkspaceGrants)
	if err != nil {
		return "", "", nil, err
	}
	grantRoot := strings.TrimSpace(access.Grant.Path)
	if grantRoot == "" {
		access.Close()
		return "", "", nil, workspaceGrantRequiredError(abs)
	}
	rel, err := filepath.Rel(grantRoot, abs)
	if err != nil {
		access.Close()
		return "", "", nil, fmt.Errorf("resolve granted workspace write path: %w", err)
	}
	return grantRoot, filepath.ToSlash(rel), access, nil
}

func existingWriteAccessPath(path string) (string, error) {
	path = filepath.Clean(safety.NormalizeUserSuppliedPath(strings.TrimSpace(path)))
	if path == "" || path == "." {
		return "", fmt.Errorf("file path is required")
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat file path: %w", err)
	}
	parent := filepath.Dir(path)
	for parent != "" && parent != "." {
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("file parent is not a directory: %s", parent)
			}
			return parent, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat file parent: %w", err)
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}
	return "", fmt.Errorf("file parent folder does not exist: %s", filepath.Dir(path))
}

func requireWorkspaceAccessForTool(cfg SafeToolConfig, target string) (*safety.WorkspaceAccess, error) {
	root := strings.TrimSpace(cfg.WorkspaceRoot)
	if root == "" {
		root = "."
	}
	currentRoot, err := safety.ResolveWorkspace(root)
	if err != nil {
		return nil, err
	}
	normalized := safety.NormalizeUserSuppliedPath(target)
	abs := normalized
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(currentRoot, abs)
	}
	abs = filepath.Clean(abs)
	if pathWithinRoot(currentRoot, abs) {
		return &safety.WorkspaceAccess{Source: "current_workspace", Path: abs}, nil
	}
	if strings.TrimSpace(cfg.WorkspaceGrants.Path) == "" {
		return nil, workspaceGrantRequiredError(abs)
	}
	return safety.RequireWorkspaceAccessWithBookmark(currentRoot, abs, cfg.WorkspaceGrants)
}

func pathWithinRoot(root string, path string) bool {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if root == path {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func workspaceGrantRequiredError(path string) error {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" {
		path = "."
	}
	return fmt.Errorf("workspace path is not granted: %s; grant that folder in Workspace or Documents settings before retrying", path)
}

func coreToolResult(ctx context.Context, cfg SafeToolConfig, decision ExecutionDecision) (ExecutionResult, bool, error) {
	limits := workspaceLimits(cfg)
	switch decision.ToolName {
	case "approved_edit_file":
		if strings.TrimSpace(decision.RequestID) == "" {
			return ExecutionResult{}, true, fmt.Errorf("approved_edit_file requires a permission request id")
		}
		if len(decision.Command) < 3 {
			return ExecutionResult{}, true, fmt.Errorf("approved_edit_file requires path and approved content")
		}
		root, target, access, err := workspaceRootAndWriteTargetForTool(cfg, decision.Command[1])
		if err != nil {
			return ExecutionResult{}, true, err
		}
		if access != nil {
			defer access.Close()
		}
		content := strings.Join(decision.Command[2:], " ")
		options := writeOptions(cfg)
		plan, err := workspace.PlanWrite(ctx, root, target, content, options)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		applied, err := workspace.ApplyWrite(ctx, root, plan, options)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		verification := workspace.VerifyAppliedWrite(ctx, root, applied, options)
		status := "completed"
		if strings.TrimSpace(verification.Status) != "" && verification.Status != "pass" {
			status = verification.Status
		}
		displayPath := strings.TrimSpace(decision.Command[1])
		if displayPath == "" {
			displayPath = applied.Path
		}
		text := fmt.Sprintf(
			"FILE WRITE APPLIED\npath: %s\nsnapshot_id: %s\nverification: %s\nchanged: %t\n\n%s",
			displayPath,
			applied.SnapshotID,
			verification.Status,
			applied.Changed,
			strings.TrimSpace(applied.Diff),
		)
		return ExecutionResult{
			Context:    strings.TrimSpace(text),
			Sources:    []string{displayPath},
			SourceKind: "tool",
			Status:     status,
		}, true, nil
	case "local_time":
		now := time.Now()
		label := ""
		timeZone := ""
		if len(decision.Command) >= 2 && strings.TrimSpace(decision.Command[1]) != "" {
			timeZone = strings.TrimSpace(decision.Command[1])
			loc, err := time.LoadLocation(timeZone)
			if err != nil {
				return ExecutionResult{}, true, fmt.Errorf("local_time could not load timezone %q: %w", timeZone, err)
			}
			now = now.In(loc)
			if len(decision.Command) >= 3 {
				label = strings.TrimSpace(strings.Join(decision.Command[2:], " "))
			}
		} else {
			timeZone = localSystemTimeZoneName()
		}
		sources := []string{"system_clock"}
		if timeZone != "" {
			sources = append(sources, "iana_timezone:"+timeZone)
		}
		return ExecutionResult{
			Context:    formatLocalTimeResult(now, label, timeZone),
			Sources:    sources,
			SourceKind: "local_time",
			Status:     "completed",
		}, true, nil
	case "ingest_documents":
		if cfg.RAG == nil {
			return ExecutionResult{}, true, fmt.Errorf("ingest_documents requires an open RAG store")
		}
		target := documentIngestPathFromDecision(decision)
		if strings.TrimSpace(target) == "" {
			return ExecutionResult{}, true, fmt.Errorf("ingest_documents requires path")
		}
		normalized := safety.NormalizeUserSuppliedPath(target)
		if !filepath.IsAbs(normalized) {
			root := strings.TrimSpace(cfg.WorkspaceRoot)
			if root == "" {
				root = "."
			}
			normalized = filepath.Join(root, normalized)
		}
		normalized = filepath.Clean(normalized)
		access, err := requireWorkspaceAccessForTool(cfg, normalized)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		defer access.Close()
		profileID := config.DefaultProfile
		if cfg.Profile != nil && strings.TrimSpace(cfg.Profile.Name) != "" {
			profileID = strings.TrimSpace(cfg.Profile.Name)
		} else if cfg.Config != nil && strings.TrimSpace(cfg.Config.App.Profile) != "" {
			profileID = strings.TrimSpace(cfg.Config.App.Profile)
		}
		result, err := cfg.RAG.IngestPath(ctx, profileID, normalized, ragConfigFromAgentConfig(cfg.Config), limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{
			Context:    formatDocumentIngestResult(result, normalized, cfg.MaxContextChars),
			Sources:    documentIngestSources(result, normalized),
			SourceKind: "rag",
			Status:     "completed",
		}, true, nil
	case "read_file":
		if len(decision.Command) < 2 {
			return ExecutionResult{}, true, fmt.Errorf("read_file requires path")
		}
		root, target, access, err := workspaceRootAndTargetForTool(cfg, decision.Command[1])
		if err != nil {
			return ExecutionResult{}, true, err
		}
		if access != nil {
			defer access.Close()
		}
		read, err := workspace.ReadFile(ctx, root, target, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		text := fmt.Sprintf("FILE: %s\nSIZE: %d\n\n%s", read.Path, read.Size, trimToolContext(read.Content, cfg.MaxContextChars))
		return ExecutionResult{Context: text, Sources: []string{read.Path}, SourceKind: "tool", Status: "completed"}, true, nil
	case "list_files":
		target := "."
		if len(decision.Command) >= 2 && strings.TrimSpace(decision.Command[1]) != "" {
			target = strings.TrimSpace(decision.Command[1])
		}
		root, target, access, err := workspaceRootAndTargetForTool(cfg, target)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		if access != nil {
			defer access.Close()
		}
		list, err := workspace.ListFiles(ctx, root, target, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{
			Context:    trimToolContext(formatDirectoryListResult(list), cfg.MaxContextChars),
			Sources:    []string{list.Path},
			SourceKind: "tool",
			Status:     "completed",
		}, true, nil
	case "file_stat":
		if len(decision.Command) < 2 {
			return ExecutionResult{}, true, fmt.Errorf("file_stat requires path")
		}
		root, target, access, err := workspaceRootAndTargetForTool(cfg, decision.Command[1])
		if err != nil {
			return ExecutionResult{}, true, err
		}
		if access != nil {
			defer access.Close()
		}
		stat, err := workspace.StatPath(ctx, root, target, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{
			Context:    formatFileStatResult(stat),
			Sources:    []string{stat.Path},
			SourceKind: "tool",
			Status:     "completed",
		}, true, nil
	case "file_tree":
		target := "."
		if len(decision.Command) >= 2 && strings.TrimSpace(decision.Command[1]) != "" {
			target = strings.TrimSpace(decision.Command[1])
		}
		root, target, access, err := workspaceRootAndTargetForTool(cfg, target)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		if access != nil {
			defer access.Close()
		}
		tree, err := workspace.FileTree(ctx, root, target, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{
			Context:    trimToolContext(formatFileTreeResult(tree), cfg.MaxContextChars),
			Sources:    []string{tree.Path},
			SourceKind: "tool",
			Status:     "completed",
		}, true, nil
	case "search_files":
		query := decisionQuery(decision)
		matches, err := workspace.SearchFiles(ctx, cfg.WorkspaceRoot, query, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		var builder strings.Builder
		for _, match := range matches {
			fmt.Fprintf(&builder, "%s:%d\t%s\n", match.Path, match.Line, match.Text)
		}
		text := strings.TrimSpace(builder.String())
		if text == "" {
			text = "No file matches."
		}
		return ExecutionResult{Context: trimToolContext(text, cfg.MaxContextChars), Sources: []string{cfg.WorkspaceRoot}, SourceKind: "tool", Status: "completed"}, true, nil
	case "rag_search":
		if cfg.RAG == nil {
			return ExecutionResult{}, true, fmt.Errorf("rag_search requires an open RAG store")
		}
		query := decisionQuery(decision)
		results, err := cfg.RAG.SearchWithConfig(ctx, query, ragConfigFromAgentConfig(cfg.Config), embeddingRuntime(cfg.Runtime))
		if err != nil {
			return ExecutionResult{}, true, err
		}
		var builder strings.Builder
		sources := make([]string, 0, len(results))
		for i, result := range results {
			if i >= 5 {
				break
			}
			sources = append(sources, result.Path)
			fmt.Fprintf(&builder, "[%d] %s score=%.2f source=%s\n%s\n\n", i+1, result.Path, result.Score, result.Source, trimToolContext(result.Content, 1000))
		}
		text := strings.TrimSpace(builder.String())
		if text == "" {
			text = "No RAG matches."
		}
		return ExecutionResult{Context: trimToolContext(text, cfg.MaxContextChars), Sources: sources, SourceKind: "rag", Status: "completed"}, true, nil
	case "memory_search":
		if cfg.Memory == nil {
			return ExecutionResult{}, true, fmt.Errorf("memory_search requires an open memory store")
		}
		query := decisionQuery(decision)
		results, err := cfg.Memory.SearchMemories(ctx, query, 5)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		var builder strings.Builder
		sources := make([]string, 0, len(results))
		for i, result := range results {
			sources = append(sources, result.ID)
			fmt.Fprintf(&builder, "[%d] %s importance=%d source=%s\n%s\n\n", i+1, result.Kind, result.Importance, result.Source, trimToolContext(result.Content, 1000))
		}
		text := strings.TrimSpace(builder.String())
		if text == "" {
			text = "No memory matches."
		}
		return ExecutionResult{Context: trimToolContext(text, cfg.MaxContextChars), Sources: sources, SourceKind: "memory", Status: "completed"}, true, nil
	case "memory_write":
		if cfg.Memory == nil {
			return ExecutionResult{}, true, fmt.Errorf("memory_write requires an open memory store")
		}
		if !memoryWriteApproved(decision) {
			return ExecutionResult{}, true, fmt.Errorf("memory_write requires an explicit approved permission request")
		}
		kind, content, ok := memoryWriteCommandInput(decision.Command)
		if !ok {
			return ExecutionResult{}, true, fmt.Errorf("memory_write requires kind and content")
		}
		source := "agent:memory_write"
		if requestID := strings.TrimSpace(decision.RequestID); requestID != "" {
			source = "permission:" + requestID
		}
		item, err := cfg.Memory.SaveMemory(ctx, memory.Memory{
			Kind:    kind,
			Content: content,
			Source:  source,
		})
		if err != nil {
			return ExecutionResult{}, true, err
		}
		text := fmt.Sprintf("MEMORY WRITE:\nid: %s\nkind: %s\nsource: %s\n\n%s", item.ID, item.Kind, item.Source, trimToolContext(item.Content, cfg.MaxContextChars))
		return ExecutionResult{Context: strings.TrimSpace(text), Sources: []string{item.ID}, SourceKind: "memory", Status: "completed"}, true, nil
	case "doctor_status":
		text, err := tools.DoctorStatus(ctx, tools.DoctorStatusInput{
			Config:  cfg.Config,
			Profile: cfg.Profile,
			Memory:  cfg.Memory,
			Runtime: cfg.Runtime,
		})
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{Context: text, Sources: []string{"doctor_status"}, SourceKind: "tool", Status: "completed"}, true, nil
	case "heartbeat_status":
		text, err := tools.HeartbeatStatus(ctx, tools.HeartbeatStatusInput{
			Config:  cfg.Config,
			Profile: cfg.Profile,
			Memory:  cfg.Memory,
			Runtime: cfg.Runtime,
		})
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{Context: text, Sources: []string{"heartbeat_status"}, SourceKind: "tool", Status: "completed"}, true, nil
	case "project_map":
		result, err := tools.ProjectMap(cfg.WorkspaceRoot, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{Context: tools.FormatProjectMap(result), Sources: []string{cfg.WorkspaceRoot}, SourceKind: "tool", Status: "completed"}, true, nil
	case "symbol_search":
		query := decisionQuery(decision)
		matches, err := tools.SymbolSearch(ctx, cfg.WorkspaceRoot, query, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		var builder strings.Builder
		for _, match := range matches {
			fmt.Fprintf(&builder, "%s:%d\t%s\t%s\t%s\n", match.Path, match.Line, match.Kind, match.Name, match.Source)
		}
		text := strings.TrimSpace(builder.String())
		if text == "" {
			text = "No symbols matched."
		}
		return ExecutionResult{Context: text, Sources: []string{cfg.WorkspaceRoot}, SourceKind: "tool", Status: "completed"}, true, nil
	case "secret_scan":
		findings, err := tools.SecretScan(ctx, cfg.WorkspaceRoot, limits)
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{Context: tools.FormatSecretFindings(findings), Sources: []string{cfg.WorkspaceRoot}, SourceKind: "tool", Status: "completed"}, true, nil
	case "patch_preview":
		if len(decision.Command) < 3 {
			return ExecutionResult{}, true, fmt.Errorf("patch_preview requires path and replacement content")
		}
		diff, err := tools.PatchPreview(ctx, cfg.WorkspaceRoot, decision.Command[1], strings.Join(decision.Command[2:], " "), writeOptions(cfg))
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{Context: "PATCH PREVIEW:\n" + diff, Sources: []string{decision.Command[1]}, SourceKind: "tool", Status: "completed"}, true, nil
	case "internet_fetch", "internet_head":
		if cfg.Internet == nil {
			return ExecutionResult{}, true, fmt.Errorf("%s requires the core internet service", decision.ToolName)
		}
		if len(decision.Command) < 2 {
			return ExecutionResult{}, true, fmt.Errorf("%s requires url", decision.ToolName)
		}
		input := internet.FetchInput{
			URL:            decision.Command[1],
			AllowedDomains: decision.Command[2:],
			ExtractText:    decision.ToolName == "internet_fetch",
			Caller:         "agent_tool_loop",
		}
		var result internet.FetchResult
		var err error
		if decision.ToolName == "internet_head" {
			result, err = cfg.Internet.Head(ctx, input)
		} else {
			result, err = cfg.Internet.Fetch(ctx, input)
		}
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{
			Context:    formatInternetFetchResult(result, cfg.MaxContextChars),
			Sources:    []string{result.FinalURL},
			SourceKind: "internet",
			Status:     "completed",
		}, true, nil
	case "internet_search":
		if cfg.Internet == nil {
			return ExecutionResult{}, true, fmt.Errorf("internet_search requires the core internet service")
		}
		query := decisionQuery(decision)
		result, err := cfg.Internet.Search(ctx, internet.SearchInput{
			Query:  query,
			Caller: "agent_tool_loop",
		})
		if err != nil {
			return ExecutionResult{}, true, err
		}
		return ExecutionResult{
			Context:    formatInternetSearchResult(result, cfg.MaxContextChars),
			Sources:    internetSearchSources(result),
			SourceKind: "internet",
			Status:     "completed",
		}, true, nil
	default:
		return ExecutionResult{}, false, nil
	}
}

func formatLocalTimeResult(now time.Time, label string, timeZone string) string {
	zone, offset := now.Zone()
	var builder strings.Builder
	fmt.Fprintf(&builder, "LOCAL TIME:\n")
	if strings.TrimSpace(label) != "" {
		fmt.Fprintf(&builder, "location: %s\n", strings.TrimSpace(label))
	}
	if strings.TrimSpace(timeZone) != "" {
		fmt.Fprintf(&builder, "iana_timezone: %s\n", strings.TrimSpace(timeZone))
	}
	fmt.Fprintf(&builder, "date: %s\n", now.Format("Monday, January 2, 2006"))
	fmt.Fprintf(&builder, "time: %s\n", now.Format("15:04:05"))
	fmt.Fprintf(&builder, "timezone: %s (%s)\n", zone, formatUTCOffset(offset))
	fmt.Fprintf(&builder, "time_of_day: %s\n", timeOfDayLabel(now.Hour()))
	return strings.TrimSpace(builder.String())
}

func localSystemTimeZoneName() string {
	if tz := strings.TrimSpace(os.Getenv("TZ")); tz != "" && tz != "localtime" {
		return strings.TrimPrefix(tz, ":")
	}
	if name := strings.TrimSpace(time.Local.String()); name != "" && name != "Local" {
		return name
	}
	for _, path := range []string{"/etc/localtime", "/var/db/timezone/localtime"} {
		link, err := os.Readlink(path)
		if err != nil {
			continue
		}
		if zone := zoneInfoNameFromPath(link); zone != "" {
			return zone
		}
	}
	return ""
}

func zoneInfoNameFromPath(path string) string {
	const marker = "zoneinfo/"
	index := strings.LastIndex(path, marker)
	if index < 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(path[index+len(marker):]), "/")
}

func formatUTCOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	return fmt.Sprintf("%s%02d:%02d", sign, hours, minutes)
}

func timeOfDayLabel(hour int) string {
	switch {
	case hour >= 5 && hour < 12:
		return "morning"
	case hour >= 12 && hour < 17:
		return "afternoon"
	case hour >= 17 && hour < 21:
		return "evening"
	default:
		return "night"
	}
}

func formatInternetFetchResult(result internet.FetchResult, maxChars int) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "INTERNET RESULT:\n")
	fmt.Fprintf(&builder, "url: %s\n", result.URL)
	if result.FinalURL != "" && result.FinalURL != result.URL {
		fmt.Fprintf(&builder, "final_url: %s\n", result.FinalURL)
	}
	fmt.Fprintf(&builder, "method: %s\nstatus: %d\ncontent_type: %s\nbytes: %d\nfrom_cache: %t\n", result.Method, result.StatusCode, result.ContentType, result.BodyBytes, result.FromCache)
	body := strings.TrimSpace(result.ExtractedText)
	if body == "" {
		body = strings.TrimSpace(result.Body)
	}
	if body != "" {
		fmt.Fprintf(&builder, "\n%s\n", trimToolContext(body, maxChars))
	}
	return strings.TrimSpace(builder.String())
}

func formatInternetSearchResult(result internet.SearchResult, maxChars int) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "INTERNET SEARCH RESULT:\n")
	fmt.Fprintf(&builder, "query: %s\nprovider: %s\nfrom_cache: %t\nfetched_at: %s\nresult_count: %d\n", result.Query, result.Provider, result.FromCache, result.FetchedAt, len(result.Items))
	if result.CachedAt != "" {
		fmt.Fprintf(&builder, "cached_at: %s\n", result.CachedAt)
	}
	if result.URL != "" {
		fmt.Fprintf(&builder, "search_url: %s\n", result.URL)
	}
	if len(result.Items) == 0 {
		fmt.Fprintf(&builder, "No search result items were returned. Do not infer current facts from prior chat, memory, or stale search results.\n")
		if strings.EqualFold(result.Provider, "duckduckgo") {
			fmt.Fprintf(&builder, "DuckDuckGo's public API behaves like an instant-answer source and can return zero items for general web or latest-news queries. If more coverage is needed, suggest switching to a configured supported provider such as SearXNG, Brave, Firecrawl, Mojeek, Wikimedia, or custom GET; do not suggest unsupported providers.\n")
		}
	}
	for i, item := range result.Items {
		if i >= 5 {
			break
		}
		fmt.Fprintf(&builder, "\n[%d] %s\nurl: %s\n", i+1, item.Title, item.URL)
		if item.Snippet != "" {
			fmt.Fprintf(&builder, "%s\n", item.Snippet)
		}
	}
	return strings.TrimSpace(trimToolContext(builder.String(), maxChars))
}

func internetSearchSources(result internet.SearchResult) []string {
	sources := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		sources = append(sources, item.URL)
	}
	if len(sources) == 0 && strings.TrimSpace(result.URL) != "" {
		sources = append(sources, result.URL)
	}
	return sources
}

func documentIngestPathFromDecision(decision ExecutionDecision) string {
	if len(decision.Command) > 1 {
		return strings.TrimSpace(strings.Join(decision.Command[1:], " "))
	}
	return strings.TrimSpace(decision.Reason)
}

func formatDocumentIngestResult(result rag.IngestResult, target string, maxChars int) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "DOCUMENT INGEST RESULT:\n")
	if strings.TrimSpace(target) != "" {
		fmt.Fprintf(&builder, "target: %s\n", strings.TrimSpace(target))
	}
	if strings.TrimSpace(result.Root) != "" {
		fmt.Fprintf(&builder, "root: %s\n", strings.TrimSpace(result.Root))
	}
	fmt.Fprintf(&builder, "files_indexed: %d\n", result.FilesIndexed)
	fmt.Fprintf(&builder, "files_unchanged: %d\n", result.FilesUnchanged)
	fmt.Fprintf(&builder, "files_skipped: %d\n", result.FilesSkipped)
	fmt.Fprintf(&builder, "chunks_created: %d\n", result.ChunksCreated)
	fmt.Fprintf(&builder, "bytes_indexed: %d\n", result.BytesIndexed)
	if len(result.SkippedReasons) > 0 {
		fmt.Fprintf(&builder, "skipped_reasons:\n")
		for _, reason := range result.SkippedReasons {
			if strings.TrimSpace(reason) == "" {
				continue
			}
			fmt.Fprintf(&builder, "- %s\n", strings.TrimSpace(reason))
		}
	}
	if result.FilesIndexed == 0 && result.FilesUnchanged == 0 {
		fmt.Fprintf(&builder, "note: no new document chunks were created. The path was scanned, but no supported readable documents needed indexing")
		if result.FilesSkipped > 0 {
			fmt.Fprintf(&builder, ", or files were skipped by document safety/format limits")
		}
		fmt.Fprintf(&builder, ".\n")
	}
	return trimToolContext(builder.String(), maxChars)
}

func documentIngestSources(result rag.IngestResult, target string) []string {
	sources := make([]string, 0, 2)
	seen := map[string]bool{}
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		sources = append(sources, filepath.ToSlash(path))
	}

	root := strings.TrimSpace(result.Root)
	cleanedTarget := strings.TrimSpace(target)
	if root != "" && cleanedTarget != "" {
		if rel, err := filepath.Rel(root, cleanedTarget); err == nil &&
			rel != "." &&
			rel != ".." &&
			!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			add(rel)
		} else {
			add(cleanedTarget)
		}
	} else {
		add(cleanedTarget)
	}
	if len(sources) == 0 && root != "" {
		add(root)
	}
	return sources
}

func ragConfigFromAgentConfig(cfg *config.Config) rag.Config {
	if cfg == nil {
		return rag.DefaultConfig()
	}
	return rag.Config{
		Enabled:          cfg.RAG.Enabled,
		Mode:             cfg.RAG.Mode,
		ChunkSize:        cfg.RAG.ChunkSize,
		ChunkOverlap:     cfg.RAG.ChunkOverlap,
		TopK:             cfg.RAG.TopK,
		MaxFileBytes:     cfg.RAG.MaxFileBytes,
		MaxTotalBytes:    cfg.RAG.MaxTotalBytes,
		MaxChunksPerFile: cfg.RAG.MaxChunksPerFile,
		Rerank: rag.RerankConfig{
			CandidateLimit:  cfg.RAG.Rerank.CandidateLimit,
			SourceDiversity: cfg.RAG.Rerank.SourceDiversity,
		},
		Embeddings: rag.EmbeddingConfig{
			Enabled:        cfg.RAG.Embeddings.Enabled,
			Provider:       cfg.RAG.Embeddings.Provider,
			Model:          cfg.RAG.Embeddings.Model,
			BatchSize:      cfg.RAG.Embeddings.BatchSize,
			MaxTextChars:   cfg.RAG.Embeddings.MaxTextChars,
			CandidateLimit: cfg.RAG.Embeddings.CandidateLimit,
		},
		VectorDB: rag.VectorDBConfig{
			Enabled:                cfg.RAG.VectorDB.Enabled,
			Provider:               cfg.RAG.VectorDB.Provider,
			ResearchOnly:           cfg.RAG.VectorDB.ResearchOnly,
			LocalOnly:              cfg.RAG.VectorDB.LocalOnly,
			AllowBackgroundService: cfg.RAG.VectorDB.AllowBackgroundService,
			MaxRAMMB:               cfg.RAG.VectorDB.MaxRAMMB,
			MaxStorageMB:           cfg.RAG.VectorDB.MaxStorageMB,
			MinBenefitPercent:      cfg.RAG.VectorDB.MinBenefitPercent,
		},
	}
}

func embeddingRuntime(runtime models.Runtime) models.EmbeddingRuntime {
	embedder, ok := runtime.(models.EmbeddingRuntime)
	if !ok {
		return nil
	}
	return embedder
}

func workspaceLimits(cfg SafeToolConfig) workspace.Limits {
	if cfg.Config == nil {
		return workspace.DefaultLimits()
	}
	return workspace.Limits{
		MaxFilesScanned:   cfg.Config.Workspace.MaxFilesScanned,
		MaxFileBytes:      cfg.Config.Workspace.MaxFileBytes,
		MaxTotalScanBytes: cfg.Config.Workspace.MaxTotalScanBytes,
		MaxSearchResults:  cfg.Config.Workspace.MaxSearchResults,
		MaxContextFiles:   cfg.Config.Workspace.MaxContextFiles,
		MaxContextChars:   cfg.Config.Workspace.MaxContextChars,
		IncludeHidden:     cfg.Config.Workspace.IncludeHidden,
		FollowSymlinks:    cfg.Config.Workspace.FollowSymlinks,
	}
}

func writeOptions(cfg SafeToolConfig) workspace.WriteOptions {
	options := workspace.WriteOptions{MaxEditFileBytes: 200000}
	if cfg.Config != nil {
		options.FollowSymlinks = cfg.Config.Workspace.FollowSymlinks
		options.IncludeHidden = cfg.Config.Workspace.IncludeHidden
		options.MaxEditFileBytes = cfg.Config.Tools.Filesystem.MaxEditFileBytes
	}
	if cfg.Profile != nil {
		options.SnapshotsRoot = cfg.Profile.Snapshots
	}
	return options
}

func decisionQuery(decision ExecutionDecision) string {
	if len(decision.Command) > 1 {
		return strings.Join(decision.Command[1:], " ")
	}
	return decision.Reason
}

func formatDirectoryListResult(result workspace.ListResult) string {
	path := strings.TrimSpace(result.Path)
	if path == "" {
		path = "."
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "DIRECTORY: %s\n", path)
	if len(result.Entries) == 0 {
		builder.WriteString("No visible files.\n")
	} else {
		for _, entry := range result.Entries {
			kind := "file"
			size := fmt.Sprintf("%d bytes", entry.Size)
			if entry.IsDir {
				kind = "dir"
				size = "-"
			}
			fmt.Fprintf(&builder, "- %s\t%s\t%s\n", entry.Path, kind, size)
		}
	}
	if result.Skipped > 0 {
		fmt.Fprintf(&builder, "Skipped hidden, protected, unreadable, or limit-exceeded entries: %d\n", result.Skipped)
	}
	return strings.TrimSpace(builder.String())
}

func formatFileStatResult(result workspace.StatResult) string {
	path := strings.TrimSpace(result.Path)
	if path == "" {
		path = "."
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "FILE STAT: %s\n", path)
	fmt.Fprintf(&builder, "exists: %t\n", result.Exists)
	if !result.Exists {
		return strings.TrimSpace(builder.String())
	}
	kind := "file"
	if result.IsDir {
		kind = "directory"
	}
	fmt.Fprintf(&builder, "kind: %s\n", kind)
	fmt.Fprintf(&builder, "size: %d bytes\n", result.Size)
	if !result.ModTime.IsZero() {
		fmt.Fprintf(&builder, "modified: %s\n", result.ModTime.Format(time.RFC3339))
	}
	return strings.TrimSpace(builder.String())
}

func formatFileTreeResult(result workspace.TreeResult) string {
	path := strings.TrimSpace(result.Path)
	if path == "" {
		path = "."
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "FILE TREE: %s\n", path)
	if len(result.Entries) == 0 {
		builder.WriteString("No visible files.\n")
	} else {
		for _, entry := range result.Entries {
			kind := "file"
			size := fmt.Sprintf("%d bytes", entry.Size)
			if entry.IsDir {
				kind = "dir"
				size = "-"
			}
			indent := strings.Repeat("  ", entry.Depth)
			fmt.Fprintf(&builder, "- %s%s\t%s\t%s\n", indent, entry.Path, kind, size)
		}
	}
	if result.Skipped > 0 {
		fmt.Fprintf(&builder, "Skipped hidden, protected, unreadable, or limit-exceeded entries: %d\n", result.Skipped)
	}
	if result.LimitReached {
		builder.WriteString("Limit reached before all entries were listed.\n")
	}
	return strings.TrimSpace(builder.String())
}

func memoryWriteApproved(decision ExecutionDecision) bool {
	return strings.TrimSpace(decision.RequestID) != "" &&
		!decision.RequiresConfirmation &&
		strings.EqualFold(strings.TrimSpace(decision.Reason), "user approved the pending permission request")
}

func memoryWriteCommandInput(command []string) (string, string, bool) {
	if len(command) < 3 {
		return "", "", false
	}
	kind := strings.TrimSpace(command[1])
	content := strings.TrimSpace(strings.Join(command[2:], " "))
	return kind, content, kind != "" && content != ""
}

func trimToolContext(input string, maxChars int) string {
	input = strings.TrimSpace(input)
	if maxChars <= 0 || len(input) <= maxChars {
		return input
	}
	if maxChars <= 12 {
		return input[:maxChars]
	}
	return strings.TrimSpace(input[:maxChars-12]) + "\n[truncated]"
}
