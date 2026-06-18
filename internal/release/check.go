package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/evaluation"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/models/modelruntime"
	"yemaka/internal/profiles"
	"yemaka/internal/skills"
)

const (
	VersionRC = "1.0-rc"

	StatusPass = "pass"
	StatusWarn = "warn"
	StatusFail = "fail"

	expectedBundleIdentifier = "com.yemaka.agent"
	minimumMacOSVersion      = "13.0.0"
)

type Report struct {
	Version string  `json:"version"`
	Ready   bool    `json:"ready"`
	Checks  []Check `json:"checks"`
}

type Check struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details"`
}

type Input struct {
	Config           *config.Config
	Profile          *profiles.Profile
	Memory           *memory.Store
	Runtime          models.Runtime
	Skills           skills.Registry
	AppBundlePath    string
	ArtifactsDir     string
	EntitlementsPath string
	SignScriptPath   string
}

func Run(ctx context.Context, input Input) Report {
	report := Report{Version: VersionRC, Ready: true}
	report.add(checkPlatform())
	report.add(checkLocalFirstConfig(input.Config))
	report.add(checkExpansionDefaults(input.Config))
	report.add(checkSetupState(input.Config))
	report.add(checkRuntimeConfig(input.Config))
	report.add(checkModelConfig(input.Config))
	report.add(checkMemory(ctx, input.Memory))
	report.add(checkRAG(input.Config))
	report.add(checkSafeTools(input.Config))
	report.add(checkProfile(input.Profile))
	report.add(checkSkills(input.Skills))
	report.add(checkModelReadiness(ctx, input.Config, input.Runtime))
	report.add(checkLatestEval(input.Profile))
	report.add(checkAppBundle(input.AppBundlePath))
	report.add(checkHardeningConfig(input.EntitlementsPath, input.SignScriptPath))
	report.add(checkSandboxTemplate(DefaultSandboxEntitlementsPath(), input.SignScriptPath))
	report.add(checkNoBundledModels(input.AppBundlePath))
	report.add(checkBundleSecretHygiene(input.AppBundlePath))
	report.add(checkSigningStatus(input.AppBundlePath))
	report.add(checkDistributionArtifacts(input.ArtifactsDir))
	report.add(checkNotarizationStatus(input.ArtifactsDir))
	for _, check := range report.Checks {
		if check.Status == StatusFail {
			report.Ready = false
			break
		}
	}
	return report
}

func checkSetupState(cfg *config.Config) Check {
	if cfg == nil {
		return fail("setup_state", "config is missing")
	}
	if !cfg.App.SetupComplete {
		return warn("setup_state", "first-run setup has not been completed for this profile")
	}
	return pass("setup_state", "first-run setup is complete")
}

func (r *Report) add(check Check) {
	r.Checks = append(r.Checks, check)
}

func checkPlatform() Check {
	if runtime.GOOS != "darwin" {
		return warn("platform_macos", "current platform is "+runtime.GOOS+"; 1.0 RC target is macOS")
	}
	return pass("platform_macos", "running on macOS")
}

func checkLocalFirstConfig(cfg *config.Config) Check {
	if cfg == nil {
		return fail("local_first_config", "config is missing")
	}
	if cfg.App.Mode != "local_first" {
		return fail("local_first_config", "app.mode must be local_first")
	}
	if cfg.App.Telemetry {
		return fail("local_first_config", "telemetry must be disabled")
	}
	if cfg.Security.AllowNetworkByDefault {
		return fail("local_first_config", "network must be disabled by default")
	}
	if cfg.Security.SecretsAccess {
		return fail("local_first_config", "secrets access must be disabled by default")
	}
	if cfg.CloudFallback.Enabled {
		return fail("local_first_config", "cloud fallback must be disabled for 1.0 RC")
	}
	if cfg.Connectors.Enabled {
		return fail("local_first_config", "connectors must be disabled for 1.0 RC")
	}
	return pass("local_first_config", "local-first hard exclusions are respected")
}

func checkExpansionDefaults(cfg *config.Config) Check {
	if cfg == nil {
		return fail("expansion_defaults", "config is missing")
	}
	var activeProfileWarnings []string
	if cfg.Internet.Enabled {
		activeProfileWarnings = append(activeProfileWarnings, "internet is enabled in the active profile")
	}
	if cfg.Internet.DefaultMode != "ask_each_time" {
		activeProfileWarnings = append(activeProfileWarnings, "internet default_mode is "+cfg.Internet.DefaultMode)
	}
	if !sameStrings(cfg.Internet.AllowMethods, []string{"GET", "HEAD"}) {
		return fail("expansion_defaults", "internet allow_methods must be GET/HEAD only by default")
	}
	if cfg.Internet.Search.Enabled {
		activeProfileWarnings = append(activeProfileWarnings, "internet search is enabled in the active profile")
	}
	if strings.TrimSpace(cfg.Internet.Search.Provider) != "none" || strings.TrimSpace(cfg.Internet.Search.Endpoint) != "" {
		activeProfileWarnings = append(activeProfileWarnings, "internet search provider is configured in the active profile")
	}
	if !cfg.Internet.Policy.BlockPrivateIPRanges || !cfg.Internet.Policy.BlockLocalNetworkByDefault || !cfg.Internet.Policy.LogRequests {
		return fail("expansion_defaults", "internet policy must block private/local networks and log requests")
	}
	if !cfg.Extensions.Enabled || !cfg.Extensions.RequireTests || !cfg.Extensions.RegisterOnlyIfTestsPass || !cfg.Extensions.RunOutOfProcess {
		return fail("expansion_defaults", "extensions must require tests, register only after tests pass, and run out-of-process")
	}
	if cfg.Extensions.MaxRuntimeSeconds <= 0 || cfg.Extensions.MaxRuntimeSeconds > 60 {
		return fail("expansion_defaults", "extension runtime limit must be bounded at 60 seconds or less")
	}
	if !cfg.Scheduler.Enabled {
		return fail("expansion_defaults", "scheduler must be available but approval-gated")
	}
	if cfg.Scheduler.MaxParallelJobs != 1 || cfg.Scheduler.LowMemoryMaxParallelJobs != 1 {
		return fail("expansion_defaults", "scheduler max parallel jobs must be 1 for low-resource release")
	}
	if !cfg.Scheduler.RequireApprovalForNewJobs {
		return fail("expansion_defaults", "scheduler must require approval for new jobs")
	}
	if !cfg.Heartbeat.Enabled {
		return fail("expansion_defaults", "heartbeat must be enabled for health visibility")
	}
	if !cfg.Heartbeat.Checks.Ollama || !cfg.Heartbeat.Checks.SQLite || !cfg.Heartbeat.Checks.Scheduler ||
		!cfg.Heartbeat.Checks.Internet || !cfg.Heartbeat.Checks.Connectors || !cfg.Heartbeat.Checks.Extensions ||
		!cfg.Heartbeat.Checks.DiskSpace || !cfg.Heartbeat.Checks.MemoryPressure {
		return fail("expansion_defaults", "heartbeat must cover Ollama, SQLite, scheduler, internet, connectors, extensions, disk, and memory")
	}
	if cfg.Connectors.Enabled {
		return fail("expansion_defaults", "connectors must be globally disabled by default")
	}
	for name, connector := range map[string]config.ConnectorConfig{
		"local_api":  cfg.Connectors.LocalAPI,
		"mcp_server": cfg.Connectors.MCPServer,
		"slack":      cfg.Connectors.Slack,
		"discord":    cfg.Connectors.Discord,
		"telegram":   cfg.Connectors.Telegram,
		"email":      cfg.Connectors.Email,
	} {
		if connector.Enabled {
			return fail("expansion_defaults", name+" connector must be disabled by default")
		}
		if connector.Permissions.Outbound || connector.Permissions.Posting || connector.Permissions.Trading || connector.Permissions.Deployment || connector.Permissions.Mutating {
			return fail("expansion_defaults", name+" connector must not default to outbound or mutating permissions")
		}
		if connector.SecretRef.Provider == "" {
			return fail("expansion_defaults", name+" connector must use a secret reference, not inline secrets")
		}
	}
	if len(activeProfileWarnings) > 0 {
		return warn("expansion_defaults", strings.Join(activeProfileWarnings, "; ")+"; release defaults still require internet/search disabled and unconfigured. Run release checks with a clean YEMAKA_HOME to validate packaged defaults.")
	}
	return pass("expansion_defaults", "expansion systems stay local-first, approval-gated, and release-bounded")
}

func checkRuntimeConfig(cfg *config.Config) Check {
	if cfg == nil {
		return fail("runtime_low_resource", "config is missing")
	}
	if !cfg.Runtime.LowMemoryMode {
		return warn("runtime_low_resource", "low_memory_mode is disabled")
	}
	if cfg.Runtime.MaxParallelTools != 1 {
		return fail("runtime_low_resource", "max_parallel_tools must be 1 for the low-resource RC")
	}
	if cfg.Runtime.MaxContextTokens > 6144 {
		return warn("runtime_low_resource", "max_context_tokens is above the useful local target")
	}
	return pass("runtime_low_resource", "runtime is conservative for low-end machines")
}

func checkModelConfig(cfg *config.Config) Check {
	if cfg == nil {
		return fail("local_model_policy", "config is missing")
	}
	for _, role := range []string{"default", "low_memory", "coding", "reasoning", "stronger_local"} {
		model, ok := cfg.Models[role]
		if !ok {
			return fail("local_model_policy", "missing model role: "+role)
		}
		provider := models.NormalizeProvider(model.Provider)
		if !models.IsRuntimeProvider(provider) {
			return fail("local_model_policy", fmt.Sprintf("%s provider must be an explicit local runtime provider", role))
		}
		baseURL := strings.TrimSpace(model.BaseURL)
		if baseURL == "" {
			baseURL = modelruntime.DefaultBaseURL(provider)
		}
		if !modelruntime.IsLocalBaseURL(baseURL) {
			return fail("local_model_policy", fmt.Sprintf("%s base_url must be local", role))
		}
		if model.Name == "" {
			return fail("local_model_policy", fmt.Sprintf("%s model name is empty", role))
		}
	}
	return pass("local_model_policy", "model roles use explicit local runtime providers")
}

func checkMemory(ctx context.Context, store *memory.Store) Check {
	if store == nil {
		return fail("sqlite_memory", "memory store is missing")
	}
	if err := store.Ping(ctx); err != nil {
		return fail("sqlite_memory", err.Error())
	}
	return pass("sqlite_memory", "SQLite memory is reachable")
}

func checkRAG(cfg *config.Config) Check {
	if cfg == nil {
		return fail("sqlite_fts_rag", "config is missing")
	}
	if !cfg.RAG.Enabled {
		return fail("sqlite_fts_rag", "RAG must be enabled for 1.0 RC")
	}
	if cfg.RAG.Mode != "sqlite_fts" {
		return fail("sqlite_fts_rag", "RAG mode must be sqlite_fts; embeddings are post-1.0")
	}
	if cfg.RAG.VectorDB.Enabled {
		return fail("sqlite_fts_rag", "external vector DB must stay disabled; use the research gate only")
	}
	if !cfg.RAG.VectorDB.ResearchOnly || !cfg.RAG.VectorDB.LocalOnly {
		return fail("sqlite_fts_rag", "vector DB gate must remain research-only and local-only")
	}
	if cfg.RAG.VectorDB.AllowBackgroundService {
		return fail("sqlite_fts_rag", "vector DB gate must not require a background service")
	}
	return pass("sqlite_fts_rag", "SQLite FTS5 RAG is the active default; vector DB gate is research-only")
}

func checkSafeTools(cfg *config.Config) Check {
	if cfg == nil {
		return fail("safe_tools", "config is missing")
	}
	if !cfg.Tools.Filesystem.WorkspaceOnly {
		return fail("safe_tools", "filesystem tools must be workspace-only")
	}
	if !cfg.Tools.Filesystem.SnapshotBeforeWrite {
		return fail("safe_tools", "snapshot_before_write must be enabled")
	}
	if !cfg.Tools.Filesystem.RequireConfirmation {
		return fail("safe_tools", "file writes must require confirmation")
	}
	if !cfg.Tools.Shell.Enabled {
		return warn("safe_tools", "safe shell verification tools are disabled")
	}
	if cfg.Tools.Shell.MaxParallelCommands != 1 {
		return fail("safe_tools", "max_parallel_commands must be 1")
	}
	if !contains(cfg.Tools.Shell.RequireConfirmationFor, "rm") || !contains(cfg.Tools.Shell.RequireConfirmationFor, "sudo") {
		return fail("safe_tools", "risky command confirmation list is incomplete")
	}
	return pass("safe_tools", "workspace writes, snapshots, rollback, and safe shell policy are configured")
}

func checkProfile(profile *profiles.Profile) Check {
	if profile == nil {
		return fail("profile_layout", "profile is missing")
	}
	required := []string{
		profile.Root,
		filepath.Dir(profile.Database),
		profile.Sessions,
		profile.Skills,
		profile.RAG,
		profile.Logs,
		profile.Snapshots,
		profile.Evals,
		profile.Permissions,
	}
	for _, path := range required {
		info, err := os.Stat(path)
		if err != nil {
			return fail("profile_layout", "missing path: "+path)
		}
		if !info.IsDir() {
			return fail("profile_layout", "path is not a directory: "+path)
		}
	}
	return pass("profile_layout", "profile directories exist")
}

func checkSkills(registry skills.Registry) Check {
	required := []string{"project_explainer", "document_summary", "git_diff_review", "python_bugfix", "code_review"}
	for _, name := range required {
		if _, ok := registry.Get(name); !ok {
			return fail("default_skills", "missing default skill: "+name)
		}
	}
	for _, skill := range registry.List() {
		if skill.Permissions.Network {
			return fail("default_skills", "network-enabled skill is not allowed in 1.0 RC: "+skill.Name)
		}
	}
	return pass("default_skills", "default local skills are installed and network-free")
}

func checkModelReadiness(ctx context.Context, cfg *config.Config, modelRuntime models.Runtime) Check {
	if modelRuntime == nil {
		return warn("model_readiness", "model runtime is missing")
	}
	if err := modelRuntime.Health(ctx); err != nil {
		return warn("model_readiness", "Ollama is not reachable: "+err.Error())
	}
	installed, err := modelRuntime.ListModels(ctx)
	if err != nil {
		return warn("model_readiness", "cannot list local models: "+err.Error())
	}
	selected := selectedModelName(cfg)
	if !modelInstalled(selected, localInstalledModels(installed)) {
		return warn("model_readiness", "selected model is not an installed local model: "+selected)
	}
	return pass("model_readiness", "selected model is installed locally")
}

func checkLatestEval(profile *profiles.Profile) Check {
	if profile == nil {
		return warn("latest_eval_report", "profile is missing")
	}
	report, err := evaluation.LoadLatest(profile)
	if err != nil {
		return warn("latest_eval_report", "no latest eval report found")
	}
	if report.Summary.Failed > 0 {
		return fail("latest_eval_report", fmt.Sprintf("latest eval has %d failed task(s)", report.Summary.Failed))
	}
	if report.Summary.Skipped > 0 {
		return warn("latest_eval_report", fmt.Sprintf("latest eval has %d skipped task(s)", report.Summary.Skipped))
	}
	return pass("latest_eval_report", "latest eval passed with no failures or skips")
}

func checkAppBundle(path string) Check {
	if strings.TrimSpace(path) == "" {
		path = DefaultAppBundlePath()
	}
	if _, err := os.Stat(path); err != nil {
		return warn("desktop_app_bundle", "app bundle not built at "+path)
	}
	binary := filepath.Join(path, "Contents", "MacOS", "Yemaka")
	info, err := os.Stat(binary)
	if err != nil {
		return fail("desktop_app_bundle", "app executable is missing from "+path)
	}
	if info.IsDir() {
		return fail("desktop_app_bundle", "app executable path is a directory")
	}
	plistPath := filepath.Join(path, "Contents", "Info.plist")
	plist, err := os.ReadFile(plistPath)
	if err != nil {
		return fail("desktop_app_bundle", "Info.plist is missing from app bundle")
	}
	if got := plistString(plist, "CFBundleName"); got != "Yemaka" {
		return fail("desktop_app_bundle", "CFBundleName must be Yemaka")
	}
	if got := plistString(plist, "CFBundleExecutable"); got != "Yemaka" {
		return fail("desktop_app_bundle", "CFBundleExecutable must be Yemaka")
	}
	if got := plistString(plist, "CFBundleIdentifier"); got != expectedBundleIdentifier {
		return fail("desktop_app_bundle", fmt.Sprintf("CFBundleIdentifier must be %s", expectedBundleIdentifier))
	}
	if got := plistString(plist, "LSMinimumSystemVersion"); !versionAtLeast(got, minimumMacOSVersion) {
		return fail("desktop_app_bundle", fmt.Sprintf("LSMinimumSystemVersion must be at least %s", minimumMacOSVersion))
	}
	if !plistBoolTrue(plist, "NSAllowsLocalNetworking") {
		return fail("desktop_app_bundle", "NSAllowsLocalNetworking must be enabled for local Ollama/API access")
	}
	if _, err := os.Stat(filepath.Join(path, "Contents", "Resources", "iconfile.icns")); err != nil {
		return fail("desktop_app_bundle", "app icon is missing")
	}
	if _, err := os.Stat(filepath.Join(path, "Contents", "_CodeSignature", "CodeResources")); err != nil {
		return warn("desktop_app_bundle", "Yemaka.app is built but not signed")
	}
	return pass("desktop_app_bundle", "Yemaka.app metadata, icon, and local networking are ready")
}

func checkHardeningConfig(entitlementsPath string, signScriptPath string) Check {
	if strings.TrimSpace(entitlementsPath) == "" {
		entitlementsPath = DefaultEntitlementsPath()
	}
	data, err := os.ReadFile(entitlementsPath)
	if err != nil {
		return fail("macos_hardening", "entitlements file is missing: "+entitlementsPath)
	}
	if !plistBoolTrue(data, "com.apple.security.network.client") {
		return fail("macos_hardening", "entitlements must allow local network client access for Ollama")
	}
	for _, entitlement := range riskyEntitlements() {
		if plistBoolTrue(data, entitlement) {
			return fail("macos_hardening", "risky entitlement must not be enabled: "+entitlement)
		}
	}

	if strings.TrimSpace(signScriptPath) == "" {
		signScriptPath = DefaultSignScriptPath()
	}
	script, err := os.ReadFile(signScriptPath)
	if err != nil {
		return fail("macos_hardening", "signing script is missing: "+signScriptPath)
	}
	text := string(script)
	if !strings.Contains(text, "--options runtime") {
		return fail("macos_hardening", "signing script must enable Hardened Runtime")
	}
	if !strings.Contains(text, "--timestamp") {
		return fail("macos_hardening", "signing script must request a timestamp")
	}
	if !strings.Contains(text, "--entitlements") {
		return fail("macos_hardening", "signing script must use the entitlements file")
	}
	return pass("macos_hardening", "Hardened Runtime signing and local-network entitlements are configured")
}

func checkSandboxTemplate(sandboxEntitlementsPath string, signScriptPath string) Check {
	if strings.TrimSpace(sandboxEntitlementsPath) == "" {
		sandboxEntitlementsPath = DefaultSandboxEntitlementsPath()
	}
	data, err := os.ReadFile(sandboxEntitlementsPath)
	if err != nil {
		return warn("app_sandbox_template", "sandbox entitlements template is missing: "+sandboxEntitlementsPath)
	}
	required := []string{
		"com.apple.security.app-sandbox",
		"com.apple.security.network.client",
		"com.apple.security.files.user-selected.read-write",
		"com.apple.security.files.bookmarks.app-scope",
	}
	for _, entitlement := range required {
		if !plistBoolTrue(data, entitlement) {
			return fail("app_sandbox_template", "sandbox template must enable "+entitlement)
		}
	}
	for _, entitlement := range riskyEntitlements() {
		if plistBoolTrue(data, entitlement) {
			return fail("app_sandbox_template", "risky entitlement must not be enabled in sandbox template: "+entitlement)
		}
	}
	if strings.TrimSpace(signScriptPath) == "" {
		signScriptPath = DefaultSignScriptPath()
	}
	script, err := os.ReadFile(signScriptPath)
	if err != nil {
		return warn("app_sandbox_template", "signing script is missing; sandbox signing path cannot be checked")
	}
	text := string(script)
	if !strings.Contains(text, "SANDBOX") || !strings.Contains(text, "entitlements.sandbox.plist") {
		return warn("app_sandbox_template", "signing script does not expose the optional sandbox signing path")
	}
	return pass("app_sandbox_template", "optional App Sandbox test entitlements and signing path are ready")
}

func checkNoBundledModels(path string) Check {
	if strings.TrimSpace(path) == "" {
		path = DefaultAppBundlePath()
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return warn("model_bundle_policy", "app bundle not built; bundled model scan skipped")
	}
	matches, err := findBundleMatches(path, isModelArtifactPath, 8)
	if err != nil {
		return warn("model_bundle_policy", "could not scan app bundle: "+err.Error())
	}
	if len(matches) > 0 {
		return fail("model_bundle_policy", "model artifacts must not be bundled: "+strings.Join(matches, ", "))
	}
	return pass("model_bundle_policy", "no bundled model artifacts found")
}

func checkBundleSecretHygiene(path string) Check {
	if strings.TrimSpace(path) == "" {
		path = DefaultAppBundlePath()
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return warn("bundle_secret_hygiene", "app bundle not built; secret hygiene scan skipped")
	}
	matches, err := findBundleMatches(path, isSecretArtifactPath, 8)
	if err != nil {
		return warn("bundle_secret_hygiene", "could not scan app bundle: "+err.Error())
	}
	if len(matches) > 0 {
		return fail("bundle_secret_hygiene", "secret-like files must not be bundled: "+strings.Join(matches, ", "))
	}
	contentMatches, err := findBundleSecretContent(path, 8)
	if err != nil {
		return warn("bundle_secret_hygiene", "could not scan bundle text files: "+err.Error())
	}
	if len(contentMatches) > 0 {
		return fail("bundle_secret_hygiene", "secret-like values must not be bundled: "+strings.Join(contentMatches, ", "))
	}
	return pass("bundle_secret_hygiene", "no secret-like bundle paths or values found")
}

func checkSigningStatus(path string) Check {
	if strings.TrimSpace(path) == "" {
		path = DefaultAppBundlePath()
	}
	signaturePath := filepath.Join(path, "Contents", "_CodeSignature", "CodeResources")
	if _, err := os.Stat(signaturePath); err != nil {
		return warn("codesigning", "app is unsigned or ad-hoc unsigned; local development builds remain allowed")
	}
	return pass("codesigning", "app bundle contains a code signature resource")
}

func checkDistributionArtifacts(dir string) Check {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultArtifactsDir()
	}
	info, err := os.Stat(dir)
	if err != nil {
		return warn("distribution_artifacts", "release artifacts have not been packaged at "+dir)
	}
	if !info.IsDir() {
		return warn("distribution_artifacts", "artifact path is not a directory: "+dir)
	}
	dmg, err := latestArtifact(dir, ".dmg")
	if err != nil {
		return warn("distribution_artifacts", "no DMG artifact found in "+dir)
	}
	if _, err := os.Stat(dmg + ".sha256"); err != nil {
		return warn("distribution_artifacts", "checksum missing for "+filepath.Base(dmg))
	}
	manifest := filepath.Join(dir, "manifest.json")
	if _, err := os.Stat(manifest); err != nil {
		return warn("distribution_artifacts", "manifest.json is missing")
	}
	if matches, err := findForbiddenReleaseArchiveEntries(dir); err != nil {
		return warn("distribution_artifacts", "could not inspect release artifact hygiene: "+err.Error())
	} else if len(matches) > 0 {
		return fail("distribution_artifacts", "release artifacts include forbidden development files: "+strings.Join(matches, ", "))
	}
	return pass("distribution_artifacts", "DMG, checksum, and manifest are present")
}

func findForbiddenReleaseArchiveEntries(dir string) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if forbiddenReleaseArchivePath(rel) {
			matches = append(matches, rel)
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		switch {
		case strings.HasSuffix(strings.ToLower(rel), ".zip"):
			archiveMatches, err := inspectZipArchive(path, rel)
			if err != nil {
				return err
			}
			matches = append(matches, archiveMatches...)
		case strings.HasSuffix(strings.ToLower(rel), ".tar.gz") || strings.HasSuffix(strings.ToLower(rel), ".tgz") || strings.HasSuffix(strings.ToLower(rel), ".tar"):
			archiveMatches, err := inspectTarArchive(path, rel)
			if err != nil {
				return err
			}
			matches = append(matches, archiveMatches...)
		}
		return nil
	})
	return matches, err
}

func inspectZipArchive(path string, label string) ([]string, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	var matches []string
	for _, file := range reader.File {
		name := strings.TrimSpace(file.Name)
		if forbiddenReleaseArchivePath(name) {
			matches = append(matches, label+":"+filepath.ToSlash(name))
		}
	}
	return matches, nil
}

func inspectTarArchive(path string, label string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var reader io.Reader = file
	if strings.HasSuffix(strings.ToLower(path), ".tar.gz") || strings.HasSuffix(strings.ToLower(path), ".tgz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	tarReader := tar.NewReader(reader)
	var matches []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if forbiddenReleaseArchivePath(header.Name) {
			matches = append(matches, label+":"+filepath.ToSlash(header.Name))
		}
	}
	return matches, nil
}

func forbiddenReleaseArchivePath(path string) bool {
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" {
		return false
	}
	trimmed := strings.Trim(path, "/")
	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		switch part {
		case ".DS_Store", "__MACOSX", "node_modules", "playwright-report", "test-results", "logs", "tmp", "temp", "coverage":
			return true
		}
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{
		"frontend/playwright-report/",
		"frontend/test-results/",
		"frontend/node_modules/",
		"frontend/dist/",
		"build/",
	} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	for _, suffix := range []string{".sqlite", ".sqlite3", ".db", ".db-shm", ".db-wal", ".log", ".tmp"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func checkNotarizationStatus(dir string) Check {
	if strings.TrimSpace(dir) == "" {
		dir = DefaultArtifactsDir()
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return warn("notarization", "manifest.json is missing; notarization status unknown")
	}
	var manifest struct {
		NotarizationStatus string `json:"notarization_status"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return warn("notarization", "manifest.json could not be parsed")
	}
	status := strings.TrimSpace(manifest.NotarizationStatus)
	switch status {
	case "accepted", "stapled":
		return pass("notarization", "notarization status: "+status)
	case "", "not_submitted":
		return warn("notarization", "notarization has not been submitted; local development builds remain allowed")
	default:
		return warn("notarization", "notarization status: "+status)
	}
}

func DefaultAppBundlePath() string {
	return filepath.Join("build", "bin", "Yemaka.app")
}

func DefaultArtifactsDir() string {
	return filepath.Join("dist", "macos")
}

func DefaultEntitlementsPath() string {
	return filepath.Join("build", "darwin", "entitlements.plist")
}

func DefaultSandboxEntitlementsPath() string {
	return filepath.Join("build", "darwin", "entitlements.sandbox.plist")
}

func DefaultSignScriptPath() string {
	return filepath.Join("scripts", "macos", "sign-app.sh")
}

func latestArtifact(dir string, suffix string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var latest string
	var latestMod int64
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		mod := info.ModTime().UnixNano()
		if latest == "" || mod > latestMod {
			latest = filepath.Join(dir, entry.Name())
			latestMod = mod
		}
	}
	if latest == "" {
		return "", os.ErrNotExist
	}
	return latest, nil
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

func modelInstalled(name string, installed []models.ModelInfo) bool {
	return models.ModelInstalled(name, installed)
}

func localInstalledModels(installed []models.ModelInfo) []models.ModelInfo {
	return models.LocalInstalledModels(installed)
}

func riskyEntitlements() []string {
	return []string{
		"com.apple.security.get-task-allow",
		"com.apple.security.cs.allow-jit",
		"com.apple.security.cs.allow-unsigned-executable-memory",
		"com.apple.security.cs.disable-library-validation",
		"com.apple.security.cs.allow-dyld-environment-variables",
	}
}

var bundleSecretValuePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bsk-[a-z0-9_\-]{20,}`),
	regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9\-]{20,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
}

func findBundleMatches(root string, match func(string) bool, limit int) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if len(matches) >= limit {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			rel = path
		}
		if match(rel) {
			matches = append(matches, filepath.ToSlash(rel))
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	return matches, err
}

func findBundleSecretContent(root string, limit int) ([]string, error) {
	var matches []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || len(matches) >= limit || !shouldScanBundleContent(path) {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() > 512*1024 {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(data)
		if containsBundleSecretValue(text) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				rel = path
			}
			matches = append(matches, filepath.ToSlash(rel))
		}
		return nil
	})
	return matches, err
}

func isModelArtifactPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	if base == "ollama" || base == "modelfile" || base == "pytorch_model.bin" {
		return true
	}
	if strings.Contains(lower, "/models/") || strings.HasPrefix(lower, "models/") {
		return true
	}
	for _, ext := range []string{".gguf", ".ggml", ".safetensors", ".onnx", ".pt", ".pth", ".ckpt", ".tflite", ".mlmodel"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func isSecretArtifactPath(path string) bool {
	lower := strings.ToLower(filepath.ToSlash(path))
	base := filepath.Base(lower)
	switch {
	case base == ".env" || strings.HasPrefix(base, ".env."):
		return true
	case strings.Contains(lower, "id_rsa") || strings.Contains(lower, "id_ed25519"):
		return true
	case strings.HasSuffix(lower, ".pem") || strings.HasSuffix(lower, ".p12") || strings.HasSuffix(lower, ".key"):
		return true
	case strings.Contains(lower, "credentials") || strings.Contains(lower, "secret"):
		return true
	default:
		return false
	}
}

func shouldScanBundleContent(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".js", ".json", ".html", ".css", ".plist", ".txt", ".md", ".yml", ".yaml", ".env":
		return true
	default:
		return false
	}
}

func containsBundleSecretValue(text string) bool {
	for _, pattern := range bundleSecretValuePatterns {
		if pattern.MatchString(text) {
			return true
		}
	}
	return false
}

func plistString(data []byte, key string) string {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	current := ""
	lastKey := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return ""
		}
		switch value := token.(type) {
		case xml.StartElement:
			current = value.Name.Local
		case xml.EndElement:
			current = ""
		case xml.CharData:
			text := strings.TrimSpace(string(value))
			if text == "" {
				continue
			}
			switch current {
			case "key":
				lastKey = text
			case "string":
				if lastKey == key {
					return text
				}
			}
		}
	}
}

func plistBoolTrue(data []byte, key string) bool {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	current := ""
	lastKey := ""
	for {
		token, err := decoder.Token()
		if err != nil {
			return false
		}
		switch value := token.(type) {
		case xml.StartElement:
			current = value.Name.Local
			if current == "true" && lastKey == key {
				return true
			}
		case xml.EndElement:
			current = ""
		case xml.CharData:
			if current == "key" {
				lastKey = strings.TrimSpace(string(value))
			}
		}
	}
}

func versionAtLeast(actual string, minimum string) bool {
	actualParts, ok := parseVersion(actual)
	if !ok {
		return false
	}
	minimumParts, ok := parseVersion(minimum)
	if !ok {
		return false
	}
	for i := 0; i < len(minimumParts); i++ {
		if actualParts[i] > minimumParts[i] {
			return true
		}
		if actualParts[i] < minimumParts[i] {
			return false
		}
	}
	return true
}

func parseVersion(input string) ([3]int, bool) {
	var out [3]int
	parts := strings.Split(strings.TrimSpace(input), ".")
	if len(parts) == 0 {
		return out, false
	}
	for i := 0; i < len(out); i++ {
		if i >= len(parts) {
			continue
		}
		part := strings.TrimSpace(parts[i])
		if part == "" {
			return out, false
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return out, false
		}
		out[i] = value
	}
	return out, true
}

func contains(items []string, needle string) bool {
	for _, item := range items {
		if item == needle {
			return true
		}
	}
	return false
}

func sameStrings(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func pass(name string, details string) Check {
	return Check{Name: name, Status: StatusPass, Details: details}
}

func warn(name string, details string) Check {
	return Check{Name: name, Status: StatusWarn, Details: details}
}

func fail(name string, details string) Check {
	return Check{Name: name, Status: StatusFail, Details: details}
}
