package tools

import (
	"bufio"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/diagnostics"
	"yemaka/internal/extensions"
	"yemaka/internal/heartbeat"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
	"yemaka/internal/scheduler"
	"yemaka/internal/workspace"
)

type DoctorStatusInput struct {
	Config  *config.Config
	Profile *profiles.Profile
	Memory  *memory.Store
	Runtime models.Runtime
}

type HeartbeatStatusInput struct {
	Config  *config.Config
	Profile *profiles.Profile
	Memory  *memory.Store
	Runtime models.Runtime
}

type ProjectMapResult struct {
	Root          string
	Summary       string
	Languages     map[string]int
	TopLevelDirs  []string
	KeyFiles      []string
	Documentation []string
	Manifests     []string
	Entrypoints   []string
	TestCommand   []string
	FilesIndexed  int
	FilesSkipped  int
	LimitReached  bool
}

type SymbolMatch struct {
	Path   string
	Line   int
	Kind   string
	Name   string
	Source string
}

type SecretFinding struct {
	Path   string
	Line   int
	Kind   string
	Reason string
}

func DoctorStatus(ctx context.Context, input DoctorStatusInput) (string, error) {
	if input.Config == nil || input.Profile == nil || input.Memory == nil || input.Runtime == nil {
		return "", fmt.Errorf("doctor_status requires config, profile, memory, and runtime")
	}
	doctor := diagnostics.Run(ctx, input.Config, input.Profile, input.Memory, input.Runtime)
	var builder strings.Builder
	fmt.Fprintf(&builder, "DOCTOR STATUS\n")
	fmt.Fprintf(&builder, "config: %s\n", doctor.ConfigPath)
	fmt.Fprintf(&builder, "profile: %s\n", doctor.ProfilePath)
	fmt.Fprintf(&builder, "sqlite_ok: %t\n", doctor.SQLiteOK)
	if doctor.SQLiteError != "" {
		fmt.Fprintf(&builder, "sqlite_error: %s\n", doctor.SQLiteError)
	}
	fmt.Fprintf(&builder, "ollama_ok: %t\n", doctor.OllamaOK)
	if doctor.OllamaError != "" {
		fmt.Fprintf(&builder, "ollama_error: %s\n", doctor.OllamaError)
	}
	fmt.Fprintf(&builder, "selected_model: %s\n", doctor.SelectedModel)
	fmt.Fprintf(&builder, "model_ready: %t\n", doctor.ModelReady)
	fmt.Fprintf(&builder, "setup_complete: %t\n", doctor.SetupComplete)
	fmt.Fprintf(&builder, "low_memory_mode: %t\n", doctor.LowMemoryMode)
	fmt.Fprintf(&builder, "rag_enabled: %t\n", doctor.RAGEnabled)
	fmt.Fprintf(&builder, "cloud_fallback_enabled: %t\n", doctor.CloudFallback)
	fmt.Fprintf(&builder, "telemetry: %t\n", doctor.ConfigTelemetry)
	if len(doctor.ModelStatuses) > 0 {
		fmt.Fprintf(&builder, "model_roles:\n")
		for _, item := range doctor.ModelStatuses {
			state := "missing"
			if item.Installed {
				state = "installed"
			}
			fmt.Fprintf(&builder, "- %s: %s (%s)\n", item.Role, item.Name, state)
		}
	}
	return strings.TrimSpace(builder.String()), nil
}

func HeartbeatStatus(ctx context.Context, input HeartbeatStatusInput) (string, error) {
	var schedulerStatus *scheduler.Status
	if input.Config != nil && input.Config.Heartbeat.Checks.Scheduler && input.Profile != nil && strings.TrimSpace(input.Profile.Database) != "" {
		store, err := scheduler.Open(ctx, input.Profile.Database)
		if err != nil {
			return "", err
		}
		defer store.Close()
		status, err := store.Status(ctx, input.Config.Scheduler.Enabled, heartbeatSchedulerMaxParallel(input.Config))
		if err != nil {
			return "", err
		}
		schedulerStatus = &status
	}

	var extensionStore *extensions.Store
	if input.Config != nil && input.Config.Heartbeat.Checks.Extensions && input.Profile != nil && strings.TrimSpace(input.Profile.GeneratedExtensions) != "" {
		extensionStore = extensions.NewStore(input.Profile.GeneratedExtensions, input.Profile.Logs)
	}

	report := heartbeat.Run(ctx, heartbeat.Input{
		Config:          input.Config,
		Profile:         input.Profile,
		Memory:          input.Memory,
		Runtime:         input.Runtime,
		SchedulerStatus: schedulerStatus,
		ExtensionStore:  extensionStore,
	})
	return FormatHeartbeatStatus(report), nil
}

func FormatHeartbeatStatus(report heartbeat.Report) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "HEARTBEAT STATUS\n")
	fmt.Fprintf(&builder, "overall: %s\n", report.Overall)
	if strings.TrimSpace(report.GeneratedAt) != "" {
		fmt.Fprintf(&builder, "generated_at: %s\n", report.GeneratedAt)
	}
	if len(report.Checks) > 0 {
		fmt.Fprintf(&builder, "checks:\n")
		for _, check := range report.Checks {
			detail := strings.TrimSpace(check.Detail)
			if detail == "" {
				detail = "-"
			}
			fmt.Fprintf(&builder, "- %s: %s (%s)\n", check.Name, check.Status, detail)
		}
	}
	return strings.TrimSpace(builder.String())
}

func heartbeatSchedulerMaxParallel(cfg *config.Config) int {
	if cfg == nil {
		return 1
	}
	maxParallel := cfg.Scheduler.MaxParallelJobs
	if cfg.Runtime.LowMemoryMode && cfg.Scheduler.LowMemoryMaxParallelJobs > 0 {
		maxParallel = cfg.Scheduler.LowMemoryMaxParallelJobs
	}
	if maxParallel <= 0 {
		return 1
	}
	return maxParallel
}

func ProjectMap(root string, limits workspace.Limits) (ProjectMapResult, error) {
	limits = coreLimits(limits)
	scan, err := workspace.Scan(root, limits)
	if err != nil {
		return ProjectMapResult{}, err
	}
	result := ProjectMapResult{
		Root:          scan.Root,
		Summary:       workspace.Summary(scan),
		Languages:     scan.LanguageCounts,
		TopLevelDirs:  scan.TopLevelDirs,
		KeyFiles:      scan.ImportantFiles,
		Documentation: scan.Documentation,
		FilesIndexed:  len(scan.Files),
		FilesSkipped:  len(scan.Skipped),
		LimitReached:  scan.LimitReached,
	}
	for _, file := range scan.Files {
		base := strings.ToLower(filepath.Base(file.Path))
		switch base {
		case "go.mod", "package.json", "pyproject.toml", "requirements.txt", "wails.json", "vite.config.ts", "svelte.config.js":
			result.Manifests = append(result.Manifests, file.Path)
		case "main.go", "app.go", "server.go", "index.html":
			result.Entrypoints = append(result.Entrypoints, file.Path)
		}
	}
	sort.Strings(result.Manifests)
	sort.Strings(result.Entrypoints)
	if command, err := DetectTestCommand(scan.Root); err == nil {
		result.TestCommand = command
	}
	return result, nil
}

func FormatProjectMap(result ProjectMapResult) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "%s\n", result.Summary)
	if len(result.Manifests) > 0 {
		fmt.Fprintf(&builder, "- Manifests: %s\n", strings.Join(limitCoreStrings(result.Manifests, 12), ", "))
	}
	if len(result.Entrypoints) > 0 {
		fmt.Fprintf(&builder, "- Entrypoints: %s\n", strings.Join(limitCoreStrings(result.Entrypoints, 12), ", "))
	}
	if len(result.TestCommand) > 0 {
		fmt.Fprintf(&builder, "- Detected test command: %s\n", strings.Join(result.TestCommand, " "))
	}
	return strings.TrimRight(builder.String(), "\n")
}

func PatchPreview(ctx context.Context, root string, path string, content string, options workspace.WriteOptions) (string, error) {
	options.SnapshotBeforeWrite = false
	plan, err := workspace.PlanWrite(ctx, root, path, content, options)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(plan.Diff, "\n"), nil
}

func SymbolSearch(ctx context.Context, root string, query string, limits workspace.Limits) ([]SymbolMatch, error) {
	limits = coreLimits(limits)
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, fmt.Errorf("symbol query is empty")
	}
	scan, err := workspace.Scan(root, limits)
	if err != nil {
		return nil, err
	}
	var matches []SymbolMatch
	for _, file := range scan.Files {
		if len(matches) >= limits.MaxSearchResults {
			break
		}
		read, err := workspace.ReadFile(ctx, scan.Root, file.Path, limits)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(read.Content))
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			kind, name, ok := parseSymbolLine(scanner.Text(), file.Language)
			if !ok {
				continue
			}
			if strings.Contains(strings.ToLower(name), query) || strings.Contains(strings.ToLower(scanner.Text()), query) {
				matches = append(matches, SymbolMatch{
					Path:   file.Path,
					Line:   lineNo,
					Kind:   kind,
					Name:   name,
					Source: strings.TrimSpace(scanner.Text()),
				})
				if len(matches) >= limits.MaxSearchResults {
					break
				}
			}
		}
	}
	return matches, nil
}

func SecretScan(ctx context.Context, root string, limits workspace.Limits) ([]SecretFinding, error) {
	limits = coreLimits(limits)
	scan, err := workspace.Scan(root, limits)
	if err != nil {
		return nil, err
	}
	maxFindings := limits.MaxSearchResults
	findingsByKey := map[string]SecretFinding{}
	add := func(finding SecretFinding) {
		if len(findingsByKey) >= maxFindings {
			return
		}
		key := fmt.Sprintf("%s:%d:%s", finding.Path, finding.Line, finding.Kind)
		findingsByKey[key] = finding
	}
	for _, skipped := range scan.Skipped {
		if len(findingsByKey) >= maxFindings {
			break
		}
		if secretPathReason(skipped.Path) != "" {
			add(SecretFinding{Path: skipped.Path, Kind: "secret_path", Reason: secretPathReason(skipped.Path)})
		}
	}
	patterns := secretPatterns()
	for _, file := range scan.Files {
		if len(findingsByKey) >= maxFindings {
			break
		}
		if reason := secretPathReason(file.Path); reason != "" {
			add(SecretFinding{Path: file.Path, Kind: "secret_path", Reason: reason})
			continue
		}
		read, err := workspace.ReadFile(ctx, scan.Root, file.Path, limits)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(strings.NewReader(read.Content))
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		lineNo := 0
		for scanner.Scan() {
			if len(findingsByKey) >= maxFindings {
				break
			}
			lineNo++
			for _, pattern := range patterns {
				if pattern.re.MatchString(scanner.Text()) {
					add(SecretFinding{Path: file.Path, Line: lineNo, Kind: pattern.name, Reason: "secret-like value detected; value hidden"})
				}
			}
		}
	}
	findings := make([]SecretFinding, 0, len(findingsByKey))
	for _, finding := range findingsByKey {
		findings = append(findings, finding)
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path == findings[j].Path {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Path < findings[j].Path
	})
	return findings, nil
}

func FormatSecretFindings(findings []SecretFinding) string {
	if len(findings) == 0 {
		return "No secret-like paths or values found."
	}
	var builder strings.Builder
	for _, finding := range findings {
		location := finding.Path
		if finding.Line > 0 {
			location = fmt.Sprintf("%s:%d", finding.Path, finding.Line)
		}
		fmt.Fprintf(&builder, "%s\t%s\t%s\n", location, finding.Kind, finding.Reason)
	}
	return strings.TrimRight(builder.String(), "\n")
}

func parseSymbolLine(line string, language string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	for _, pattern := range symbolPatterns {
		matches := pattern.re.FindStringSubmatch(trimmed)
		if len(matches) == 2 {
			return pattern.kind, matches[1], true
		}
	}
	return "", "", false
}

type secretPattern struct {
	name string
	re   *regexp.Regexp
}

type symbolPattern struct {
	kind string
	re   *regexp.Regexp
}

var symbolPatterns = []symbolPattern{
	{"function", regexp.MustCompile(`^func\s+(?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\s*\(`)},
	{"type", regexp.MustCompile(`^type\s+([A-Za-z_][A-Za-z0-9_]*)\s+(?:struct|interface|func|string|int|bool|map|\[\])`)},
	{"function", regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*\(`)},
	{"class", regexp.MustCompile(`^(?:export\s+)?class\s+([A-Za-z_$][A-Za-z0-9_$]*)`)},
	{"function", regexp.MustCompile(`^(?:export\s+)?(?:const|let|var)\s+([A-Za-z_$][A-Za-z0-9_$]*)\s*=\s*(?:async\s*)?\(?[^=]*\)?\s*=>`)},
	{"function", regexp.MustCompile(`^def\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(`)},
	{"class", regexp.MustCompile(`^class\s+([A-Za-z_][A-Za-z0-9_]*)`)},
}

var compiledSecretPatterns = []secretPattern{
	{"openai_api_key", regexp.MustCompile(`(?i)\bsk-[a-z0-9_\-]{20,}`)},
	{"github_token", regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{20,}`)},
	{"slack_token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{20,}`)},
	{"aws_access_key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
	{"private_key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	{"secret_assignment", regexp.MustCompile(`(?i)\b(api[_-]?key|secret|token|password)\b\s*[:=]\s*['"]?[A-Za-z0-9_\-./+=]{12,}`)},
}

func secretPatterns() []secretPattern {
	return compiledSecretPatterns
}

func secretPathReason(path string) string {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	switch {
	case base == ".env" || strings.HasPrefix(base, ".env."):
		return "environment secret file"
	case strings.Contains(lower, "id_rsa") || strings.Contains(lower, "id_ed25519"):
		return "private key path"
	case strings.Contains(lower, "secret") || strings.Contains(lower, "credentials") || strings.Contains(lower, "apikey"):
		return "secret-like path"
	default:
		return ""
	}
}

func limitCoreStrings(values []string, limit int) []string {
	if len(values) <= limit {
		return values
	}
	limited := append([]string{}, values[:limit]...)
	limited = append(limited, fmt.Sprintf("+%d more", len(values)-limit))
	return limited
}

func coreLimits(limits workspace.Limits) workspace.Limits {
	defaults := workspace.DefaultLimits()
	if limits.MaxFilesScanned == 0 {
		limits.MaxFilesScanned = defaults.MaxFilesScanned
	}
	if limits.MaxFileBytes == 0 {
		limits.MaxFileBytes = defaults.MaxFileBytes
	}
	if limits.MaxTotalScanBytes == 0 {
		limits.MaxTotalScanBytes = defaults.MaxTotalScanBytes
	}
	if limits.MaxSearchResults == 0 {
		limits.MaxSearchResults = defaults.MaxSearchResults
	}
	if limits.MaxContextFiles == 0 {
		limits.MaxContextFiles = defaults.MaxContextFiles
	}
	if limits.MaxContextChars == 0 {
		limits.MaxContextChars = defaults.MaxContextChars
	}
	return limits
}
