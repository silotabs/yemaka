package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadOrCreateUsesLowResourceDefaults(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvHome, home)

	cfg, err := LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}

	if cfg.App.Name != AppName {
		t.Fatalf("App.Name = %q, want %q", cfg.App.Name, AppName)
	}
	if !cfg.Runtime.LowMemoryMode {
		t.Fatal("low_memory_mode should default to true")
	}
	if cfg.Runtime.ResponseMode != ResponseModeBalanced {
		t.Fatalf("ResponseMode = %q, want %q", cfg.Runtime.ResponseMode, ResponseModeBalanced)
	}
	if cfg.Runtime.ShowThinkingTrace {
		t.Fatal("show_thinking_trace should default to false")
	}
	if cfg.Runtime.MaxParallelTools != 1 {
		t.Fatalf("MaxParallelTools = %d, want 1", cfg.Runtime.MaxParallelTools)
	}
	if !cfg.RAG.Enabled {
		t.Fatal("RAG should be enabled for SQLite FTS5 RAG")
	}
	if cfg.RAG.Mode != "sqlite_fts" {
		t.Fatalf("RAG.Mode = %q, want sqlite_fts", cfg.RAG.Mode)
	}
	if cfg.RAG.ChunkSize != 700 {
		t.Fatalf("RAG.ChunkSize = %d, want 700", cfg.RAG.ChunkSize)
	}
	if cfg.RAG.ChunkOverlap != 100 {
		t.Fatalf("RAG.ChunkOverlap = %d, want 100", cfg.RAG.ChunkOverlap)
	}
	if cfg.RAG.TopK != 5 {
		t.Fatalf("RAG.TopK = %d, want 5", cfg.RAG.TopK)
	}
	if cfg.RAG.Rerank.CandidateLimit != 24 {
		t.Fatalf("RAG.Rerank.CandidateLimit = %d, want 24", cfg.RAG.Rerank.CandidateLimit)
	}
	if cfg.RAG.Rerank.SourceDiversity != 2 {
		t.Fatalf("RAG.Rerank.SourceDiversity = %d, want 2", cfg.RAG.Rerank.SourceDiversity)
	}
	if cfg.RAG.Embeddings.Enabled {
		t.Fatal("RAG embeddings should be disabled by default")
	}
	if cfg.RAG.Embeddings.Provider != "ollama" {
		t.Fatalf("RAG.Embeddings.Provider = %q, want ollama", cfg.RAG.Embeddings.Provider)
	}
	if cfg.RAG.Embeddings.Model != "nomic-embed-text" {
		t.Fatalf("RAG.Embeddings.Model = %q, want nomic-embed-text", cfg.RAG.Embeddings.Model)
	}
	if cfg.RAG.VectorDB.Enabled {
		t.Fatal("vector DB should be disabled by default")
	}
	if cfg.RAG.VectorDB.Provider != "research_only" {
		t.Fatalf("RAG.VectorDB.Provider = %q, want research_only", cfg.RAG.VectorDB.Provider)
	}
	if !cfg.RAG.VectorDB.ResearchOnly {
		t.Fatal("vector DB gate should default to research_only")
	}
	if !cfg.RAG.VectorDB.LocalOnly {
		t.Fatal("vector DB gate should default to local_only")
	}
	if cfg.RAG.VectorDB.AllowBackgroundService {
		t.Fatal("vector DB gate should not allow background services by default")
	}
	if cfg.RAG.VectorDB.MaxRAMMB != 512 {
		t.Fatalf("RAG.VectorDB.MaxRAMMB = %d, want 512", cfg.RAG.VectorDB.MaxRAMMB)
	}
	if cfg.RAG.VectorDB.MaxStorageMB != 1024 {
		t.Fatalf("RAG.VectorDB.MaxStorageMB = %d, want 1024", cfg.RAG.VectorDB.MaxStorageMB)
	}
	if cfg.RAG.VectorDB.MinBenefitPercent != 20 {
		t.Fatalf("RAG.VectorDB.MinBenefitPercent = %d, want 20", cfg.RAG.VectorDB.MinBenefitPercent)
	}
	if cfg.Knowledge.Enabled {
		t.Fatal("local knowledge graph should be disabled by default")
	}
	if cfg.Knowledge.InfluenceEnabled {
		t.Fatal("local knowledge graph influence should be disabled by default")
	}
	if !cfg.Knowledge.ManualOnly {
		t.Fatal("local knowledge graph should default to manual-only")
	}
	if cfg.Knowledge.MaxEntitiesPerQuery != 12 {
		t.Fatalf("Knowledge.MaxEntitiesPerQuery = %d, want 12", cfg.Knowledge.MaxEntitiesPerQuery)
	}
	if cfg.Knowledge.MaxEvidenceChars != 2000 {
		t.Fatalf("Knowledge.MaxEvidenceChars = %d, want 2000", cfg.Knowledge.MaxEvidenceChars)
	}
	if cfg.Knowledge.MaxInfluenceEntities != 4 {
		t.Fatalf("Knowledge.MaxInfluenceEntities = %d, want 4", cfg.Knowledge.MaxInfluenceEntities)
	}
	if cfg.Knowledge.MaxInfluenceChars != 1200 {
		t.Fatalf("Knowledge.MaxInfluenceChars = %d, want 1200", cfg.Knowledge.MaxInfluenceChars)
	}
	if !cfg.Tools.Shell.Enabled {
		t.Fatal("safe shell tools should be enabled for verification")
	}
	if cfg.Tools.Shell.TimeoutSeconds != 30 {
		t.Fatalf("Shell.TimeoutSeconds = %d, want 30", cfg.Tools.Shell.TimeoutSeconds)
	}
	if cfg.Tools.Shell.MaxOutputBytes != 100000 {
		t.Fatalf("Shell.MaxOutputBytes = %d, want 100000", cfg.Tools.Shell.MaxOutputBytes)
	}
	if cfg.Tools.Shell.MaxParallelCommands != 1 {
		t.Fatalf("Shell.MaxParallelCommands = %d, want 1", cfg.Tools.Shell.MaxParallelCommands)
	}
	if cfg.Security.AllowNetworkByDefault {
		t.Fatal("network should be disabled by default")
	}
	if !cfg.Security.Policy.AuditEnabled {
		t.Fatal("policy audit should be enabled by default")
	}
	if cfg.Security.Policy.Mode != "safe" {
		t.Fatalf("policy mode = %q, want safe", cfg.Security.Policy.Mode)
	}
	if cfg.Security.Policy.MaxAutonomousLevel != 4 {
		t.Fatalf("policy max autonomous level = %d, want 4", cfg.Security.Policy.MaxAutonomousLevel)
	}
	if cfg.Security.Policy.AllowAutonomousPosting || cfg.Security.Policy.AllowAutonomousDeployment || cfg.Security.Policy.AllowLiveTrading || cfg.Security.Policy.GeneratedCanModifyCorePolicy {
		t.Fatalf("unsafe policy toggles should default false: %+v", cfg.Security.Policy)
	}
	if cfg.App.Telemetry {
		t.Fatal("telemetry should be disabled by default")
	}
	if cfg.App.SetupComplete {
		t.Fatal("setup_complete should default to false")
	}
	if cfg.Models["low_memory"].Name != "deepseek-r1:1.5b" {
		t.Fatalf("low_memory model = %q", cfg.Models["low_memory"].Name)
	}
	if cfg.CloudFallback.Enabled {
		t.Fatal("cloud fallback should be disabled by default")
	}
	if cfg.CloudFallback.Provider != "openai_compatible" {
		t.Fatalf("CloudFallback.Provider = %q, want openai_compatible", cfg.CloudFallback.Provider)
	}
	if cfg.CloudFallback.Name != "" {
		t.Fatalf("CloudFallback.Name = %q, want empty", cfg.CloudFallback.Name)
	}
	if cfg.Connectors.Enabled {
		t.Fatal("connectors should be disabled by default")
	}
	if cfg.Internet.Enabled {
		t.Fatal("internet should be disabled by default")
	}
	if cfg.Internet.DefaultMode != "ask_each_time" {
		t.Fatalf("Internet.DefaultMode = %q, want ask_each_time", cfg.Internet.DefaultMode)
	}
	if len(cfg.Internet.AllowMethods) != 2 || cfg.Internet.AllowMethods[0] != "GET" || cfg.Internet.AllowMethods[1] != "HEAD" {
		t.Fatalf("Internet.AllowMethods = %v, want GET/HEAD only", cfg.Internet.AllowMethods)
	}
	if cfg.Internet.MaxResponseBytes != 2000000 {
		t.Fatalf("Internet.MaxResponseBytes = %d, want 2000000", cfg.Internet.MaxResponseBytes)
	}
	if !cfg.Internet.Cache.Enabled {
		t.Fatal("internet cache should be enabled by default")
	}
	if cfg.Internet.Search.Enabled {
		t.Fatal("internet search should be disabled by default")
	}
	if !cfg.Internet.Policy.BlockPrivateIPRanges || !cfg.Internet.Policy.BlockLocalNetworkByDefault {
		t.Fatal("internet should block private/local networks by default")
	}
	if !cfg.Scheduler.Enabled {
		t.Fatal("scheduler should be enabled but idle by default")
	}
	if cfg.Scheduler.MaxParallelJobs != 1 || cfg.Scheduler.LowMemoryMaxParallelJobs != 1 {
		t.Fatalf("Scheduler parallelism = %d/%d, want 1/1", cfg.Scheduler.MaxParallelJobs, cfg.Scheduler.LowMemoryMaxParallelJobs)
	}
	if cfg.Scheduler.DefaultJobTimeoutSeconds != 60 {
		t.Fatalf("Scheduler.DefaultJobTimeoutSeconds = %d, want 60", cfg.Scheduler.DefaultJobTimeoutSeconds)
	}
	if !cfg.Scheduler.RequireApprovalForNewJobs {
		t.Fatal("scheduler should require approval for new jobs by default")
	}
	if !cfg.Heartbeat.Enabled {
		t.Fatal("heartbeat should be enabled by default")
	}
	if cfg.Heartbeat.IntervalSeconds != 60 {
		t.Fatalf("Heartbeat.IntervalSeconds = %d, want 60", cfg.Heartbeat.IntervalSeconds)
	}
	if !cfg.Heartbeat.Checks.Ollama || !cfg.Heartbeat.Checks.SQLite || !cfg.Heartbeat.Checks.Scheduler || !cfg.Heartbeat.Checks.Internet || !cfg.Heartbeat.Checks.Connectors || !cfg.Heartbeat.Checks.Extensions || !cfg.Heartbeat.Checks.DiskSpace || !cfg.Heartbeat.Checks.MemoryPressure {
		t.Fatal("heartbeat checks should default on")
	}
	if !cfg.Extensions.Enabled {
		t.Fatal("extension registry should be enabled by default")
	}
	if !cfg.Extensions.RequireTests {
		t.Fatal("generated extensions should require tests by default")
	}
	if !cfg.Extensions.RegisterOnlyIfTestsPass {
		t.Fatal("generated extensions should register only after tests pass")
	}
	if !cfg.Extensions.RunOutOfProcess {
		t.Fatal("generated extensions should run out of process by default")
	}
	if cfg.Extensions.MaxRuntimeSeconds != 60 {
		t.Fatalf("Extensions.MaxRuntimeSeconds = %d, want 60", cfg.Extensions.MaxRuntimeSeconds)
	}
	if cfg.Extensions.PreferredLanguage != "go" {
		t.Fatalf("Extensions.PreferredLanguage = %q, want go", cfg.Extensions.PreferredLanguage)
	}
	if cfg.Connectors.LocalAPI.Enabled {
		t.Fatal("local_api connector should be disabled by default")
	}
	if !cfg.Connectors.LocalAPI.RequireToken {
		t.Fatal("local_api connector should require token by default when enabled")
	}
	if cfg.Connectors.LocalAPI.TokenEnv != "YEMAKA_CONNECTOR_TOKEN" {
		t.Fatalf("LocalAPI.TokenEnv = %q, want YEMAKA_CONNECTOR_TOKEN", cfg.Connectors.LocalAPI.TokenEnv)
	}
	if cfg.Connectors.LocalAPI.SecretRef.Provider != "env" || cfg.Connectors.LocalAPI.SecretRef.Name != "YEMAKA_CONNECTOR_TOKEN" {
		t.Fatalf("LocalAPI.SecretRef = %+v, want env YEMAKA_CONNECTOR_TOKEN", cfg.Connectors.LocalAPI.SecretRef)
	}
	if len(cfg.Connectors.LocalAPI.AllowedDomains) == 0 {
		t.Fatal("local_api should declare loopback allowed domains")
	}
	if !cfg.Connectors.LocalAPI.Permissions.Inbound || cfg.Connectors.LocalAPI.Permissions.Outbound || cfg.Connectors.LocalAPI.Permissions.Posting {
		t.Fatalf("LocalAPI.Permissions = %+v, want inbound-only non-posting", cfg.Connectors.LocalAPI.Permissions)
	}
	if cfg.Connectors.LocalAPI.RateLimit.RequestsPerMinute != 60 {
		t.Fatalf("LocalAPI rate limit = %+v, want 60/min", cfg.Connectors.LocalAPI.RateLimit)
	}
	if cfg.Connectors.MCPServer.Enabled {
		t.Fatal("mcp_server connector should be disabled by default")
	}
	if cfg.Connectors.MCPServer.Kind != "mcp_server" {
		t.Fatalf("MCPServer.Kind = %q, want mcp_server", cfg.Connectors.MCPServer.Kind)
	}
	if cfg.Connectors.MCPServer.Bind != "stdio" {
		t.Fatalf("MCPServer.Bind = %q, want stdio", cfg.Connectors.MCPServer.Bind)
	}
	if cfg.Connectors.MCPServer.RequireToken {
		t.Fatal("mcp_server should not require a token because it is stdio-local")
	}
	if cfg.Connectors.MCPServer.SecretRef.Provider != "none" {
		t.Fatalf("MCPServer.SecretRef.Provider = %q, want none", cfg.Connectors.MCPServer.SecretRef.Provider)
	}
	adapterDefaults := map[string]ConnectorConfig{
		"slack":    cfg.Connectors.Slack,
		"discord":  cfg.Connectors.Discord,
		"telegram": cfg.Connectors.Telegram,
		"email":    cfg.Connectors.Email,
	}
	for name, adapter := range adapterDefaults {
		if adapter.Enabled {
			t.Fatalf("%s connector should be disabled by default", name)
		}
		if adapter.Kind != name {
			t.Fatalf("%s Kind = %q, want %q", name, adapter.Kind, name)
		}
		if adapter.Bind != "webhook" {
			t.Fatalf("%s Bind = %q, want webhook", name, adapter.Bind)
		}
		if !adapter.RequireToken {
			t.Fatalf("%s connector should require token by default", name)
		}
		if adapter.TokenEnv == "" {
			t.Fatalf("%s TokenEnv is empty", name)
		}
		if adapter.SecretRef.Provider != "env" || adapter.SecretRef.Name != adapter.TokenEnv {
			t.Fatalf("%s SecretRef = %+v, want env %s", name, adapter.SecretRef, adapter.TokenEnv)
		}
		if !adapter.Permissions.Inbound || adapter.Permissions.Outbound || adapter.Permissions.Posting || adapter.Permissions.Trading || adapter.Permissions.Deployment {
			t.Fatalf("%s Permissions = %+v, want inbound-only non-mutating", name, adapter.Permissions)
		}
		if adapter.RateLimit.RequestsPerMinute == 0 || adapter.RateLimit.RequestsPerDay == 0 {
			t.Fatalf("%s RateLimit = %+v, want bounded limits", name, adapter.RateLimit)
		}
	}
	if cfg.Workspace.MaxFilesScanned != 2000 {
		t.Fatalf("Workspace.MaxFilesScanned = %d, want 2000", cfg.Workspace.MaxFilesScanned)
	}
	if cfg.Workspace.MaxFileBytes != 200000 {
		t.Fatalf("Workspace.MaxFileBytes = %d, want 200000", cfg.Workspace.MaxFileBytes)
	}
	if cfg.Workspace.IncludeHidden {
		t.Fatal("workspace include_hidden should default to false")
	}
	if cfg.Workspace.FollowSymlinks {
		t.Fatal("workspace follow_symlinks should default to false")
	}
	if !cfg.Tools.Filesystem.WorkspaceOnly {
		t.Fatal("filesystem workspace_only should default to true")
	}
	if !cfg.Tools.Filesystem.SnapshotBeforeWrite {
		t.Fatal("filesystem snapshot_before_write should default to true")
	}
	if !cfg.Tools.Filesystem.RequireConfirmation {
		t.Fatal("filesystem require_confirmation should default to true")
	}
	if cfg.Tools.Filesystem.MaxEditFileBytes != 200000 {
		t.Fatalf("MaxEditFileBytes = %d, want 200000", cfg.Tools.Filesystem.MaxEditFileBytes)
	}

	wantConfig := filepath.Join(home, ConfigName)
	if cfg.Path != wantConfig {
		t.Fatalf("Path = %q, want %q", cfg.Path, wantConfig)
	}
	if _, err := os.Stat(wantConfig); err != nil {
		t.Fatalf("expected config file to be written: %v", err)
	}
}

func TestSupportDirUsesOSNativeDefault(t *testing.T) {
	t.Setenv(EnvHome, "")
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("APPDATA", "")

	dir, err := SupportDir()
	if err != nil {
		t.Fatalf("SupportDir() error = %v", err)
	}
	if dir == "" {
		t.Fatal("SupportDir() returned empty path")
	}
	switch runtime.GOOS {
	case "darwin":
		if !strings.Contains(dir, filepath.Join("Library", "Application Support", AppName)) {
			t.Fatalf("SupportDir() = %q, want macOS Application Support path", dir)
		}
	case "windows":
		if !strings.Contains(dir, filepath.Join("AppData", "Roaming", AppName)) {
			t.Fatalf("SupportDir() = %q, want Windows roaming app data path", dir)
		}
	default:
		if !strings.Contains(dir, filepath.Join(".local", "share", strings.ToLower(AppName))) {
			t.Fatalf("SupportDir() = %q, want XDG local data path", dir)
		}
	}
}

func TestSupportDirHonorsXDGDataHome(t *testing.T) {
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		t.Skip("XDG_DATA_HOME is used for Unix-like non-macOS platforms")
	}
	dataHome := t.TempDir()
	t.Setenv(EnvHome, "")
	t.Setenv("XDG_DATA_HOME", dataHome)

	dir, err := SupportDir()
	if err != nil {
		t.Fatalf("SupportDir() error = %v", err)
	}
	want := filepath.Join(dataHome, strings.ToLower(AppName))
	if dir != want {
		t.Fatalf("SupportDir() = %q, want %q", dir, want)
	}
}

func TestApplyDefaultsPreservesExplicitFullAccessPolicyMode(t *testing.T) {
	cfg := Default()
	cfg.Security.Policy.Mode = "full_access"
	ApplyDefaults(cfg)
	if cfg.Security.Policy.Mode != "full_access" {
		t.Fatalf("policy mode = %q, want full_access", cfg.Security.Policy.Mode)
	}
	cfg.Security.Policy.Mode = "surprise"
	ApplyDefaults(cfg)
	if cfg.Security.Policy.Mode != "safe" {
		t.Fatalf("unknown policy mode = %q, want safe", cfg.Security.Policy.Mode)
	}
}

func TestApplyDefaultsNormalizesUITheme(t *testing.T) {
	cfg := Default()
	if cfg.UI.Theme != "system" {
		t.Fatalf("default theme = %q, want system", cfg.UI.Theme)
	}
	cfg.UI.Theme = "DARK"
	ApplyDefaults(cfg)
	if cfg.UI.Theme != "dark" {
		t.Fatalf("normalized theme = %q, want dark", cfg.UI.Theme)
	}
	cfg.UI.Theme = "surprise"
	ApplyDefaults(cfg)
	if cfg.UI.Theme != "system" {
		t.Fatalf("unknown theme = %q, want system", cfg.UI.Theme)
	}
}

func TestNormalizeResponseMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want string
	}{
		{name: "auto", mode: "auto", want: ResponseModeAuto},
		{name: "fast", mode: "FAST", want: ResponseModeFast},
		{name: "balanced", mode: "balanced", want: ResponseModeBalanced},
		{name: "deep", mode: "deep", want: ResponseModeDeep},
		{name: "empty", mode: "", want: ResponseModeBalanced},
		{name: "unknown", mode: "turbo", want: ResponseModeBalanced},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeResponseMode(tt.mode); got != tt.want {
				t.Fatalf("NormalizeResponseMode(%q) = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

func TestApplyDefaultsBackfillsKnowledgeInfluenceSafely(t *testing.T) {
	cfg := &Config{}
	cfg.Knowledge.Enabled = false
	cfg.Knowledge.InfluenceEnabled = true

	ApplyDefaults(cfg)

	if cfg.Knowledge.InfluenceEnabled {
		t.Fatal("knowledge graph influence should be forced off when graph is disabled")
	}
	if !cfg.Knowledge.ManualOnly {
		t.Fatal("knowledge graph should stay manual-only after defaults")
	}
	if cfg.Knowledge.MaxInfluenceEntities != 4 {
		t.Fatalf("Knowledge.MaxInfluenceEntities = %d, want 4", cfg.Knowledge.MaxInfluenceEntities)
	}
	if cfg.Knowledge.MaxInfluenceChars != 1200 {
		t.Fatalf("Knowledge.MaxInfluenceChars = %d, want 1200", cfg.Knowledge.MaxInfluenceChars)
	}
}
