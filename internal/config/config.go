package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	AppName        = "Yemaka"
	EnvHome        = "YEMAKA_HOME"
	ConfigName     = "config.yaml"
	DefaultProfile = "default"

	ResponseModeAuto     = "auto"
	ResponseModeFast     = "fast"
	ResponseModeBalanced = "balanced"
	ResponseModeDeep     = "deep"
)

type Config struct {
	App           AppConfig              `yaml:"app"`
	UI            UIConfig               `yaml:"ui"`
	Runtime       RuntimeConfig          `yaml:"runtime"`
	Models        map[string]ModelConfig `yaml:"models"`
	CloudFallback CloudFallbackConfig    `yaml:"cloud_fallback"`
	Connectors    ConnectorsConfig       `yaml:"connectors"`
	Extensions    ExtensionsConfig       `yaml:"extensions"`
	Internet      InternetConfig         `yaml:"internet"`
	Scheduler     SchedulerConfig        `yaml:"scheduler"`
	Heartbeat     HeartbeatConfig        `yaml:"heartbeat"`
	Memory        MemoryConfig           `yaml:"memory"`
	Knowledge     KnowledgeGraphConfig   `yaml:"knowledge_graph"`
	Workspace     WorkspaceConfig        `yaml:"workspace"`
	RAG           RAGConfig              `yaml:"rag"`
	Tools         ToolsConfig            `yaml:"tools"`
	Security      SecurityConfig         `yaml:"security"`

	Path string `yaml:"-"`
}

type AppConfig struct {
	Name          string `yaml:"name"`
	Mode          string `yaml:"mode"`
	Profile       string `yaml:"profile"`
	SetupComplete bool   `yaml:"setup_complete"`
	Telemetry     bool   `yaml:"telemetry"`
}

type UIConfig struct {
	Theme string `yaml:"theme"`
}

type RuntimeConfig struct {
	LowMemoryMode     bool   `yaml:"low_memory_mode"`
	MaxParallelTools  int    `yaml:"max_parallel_tools"`
	MaxContextTokens  int    `yaml:"max_context_tokens"`
	AutoSummarize     bool   `yaml:"auto_summarize"`
	ResponseMode      string `yaml:"response_mode"`
	ShowThinkingTrace bool   `yaml:"show_thinking_trace"`
}

type ModelConfig struct {
	Provider    string  `yaml:"provider"`
	Name        string  `yaml:"name"`
	BaseURL     string  `yaml:"base_url"`
	Temperature float64 `yaml:"temperature"`
	Profile     string  `yaml:"profile,omitempty"`
}

type CloudFallbackConfig struct {
	Enabled        bool    `yaml:"enabled"`
	Provider       string  `yaml:"provider"`
	Name           string  `yaml:"name"`
	BaseURL        string  `yaml:"base_url"`
	APIKeyEnv      string  `yaml:"api_key_env"`
	Temperature    float64 `yaml:"temperature"`
	TimeoutSeconds int     `yaml:"timeout_seconds"`
}

type ConnectorsConfig struct {
	Enabled   bool            `yaml:"enabled"`
	LocalAPI  ConnectorConfig `yaml:"local_api"`
	MCPServer ConnectorConfig `yaml:"mcp_server"`
	Slack     ConnectorConfig `yaml:"slack"`
	Discord   ConnectorConfig `yaml:"discord"`
	Telegram  ConnectorConfig `yaml:"telegram"`
	Email     ConnectorConfig `yaml:"email"`
}

type ExtensionsConfig struct {
	Enabled                 bool   `yaml:"enabled"`
	RequireTests            bool   `yaml:"require_tests"`
	RegisterOnlyIfTestsPass bool   `yaml:"register_only_if_tests_pass"`
	RunOutOfProcess         bool   `yaml:"run_out_of_process"`
	MaxRuntimeSeconds       int    `yaml:"max_runtime_seconds"`
	GeneratedDir            string `yaml:"generated_dir"`
	PreferredLanguage       string `yaml:"preferred_language"`
}

type InternetConfig struct {
	Enabled          bool                 `yaml:"enabled"`
	DefaultMode      string               `yaml:"default_mode"`
	AllowMethods     []string             `yaml:"allow_methods"`
	MaxResponseBytes int64                `yaml:"max_response_bytes"`
	TimeoutSeconds   int                  `yaml:"timeout_seconds"`
	RedirectLimit    int                  `yaml:"redirect_limit"`
	Cache            InternetCacheConfig  `yaml:"cache"`
	Search           InternetSearchConfig `yaml:"search"`
	Policy           InternetPolicyConfig `yaml:"policy"`
}

type InternetCacheConfig struct {
	Enabled    bool `yaml:"enabled"`
	TTLSeconds int  `yaml:"ttl_seconds"`
}

type InternetSearchConfig struct {
	Enabled           bool     `yaml:"enabled"`
	Provider          string   `yaml:"provider"`
	FallbackProviders []string `yaml:"fallback_providers"`
	Endpoint          string   `yaml:"endpoint"`
	APIKeyEnv         string   `yaml:"api_key_env"`
	MaxResults        int      `yaml:"max_results"`
	SafeSearch        bool     `yaml:"safe_search"`
}

type InternetPolicyConfig struct {
	RespectRobotsTxt           bool `yaml:"respect_robots_txt"`
	BlockPrivateIPRanges       bool `yaml:"block_private_ip_ranges"`
	BlockLocalNetworkByDefault bool `yaml:"block_local_network_by_default"`
	LogRequests                bool `yaml:"log_requests"`
}

type SchedulerConfig struct {
	Enabled                   bool `yaml:"enabled"`
	MaxParallelJobs           int  `yaml:"max_parallel_jobs"`
	LowMemoryMaxParallelJobs  int  `yaml:"low_memory_max_parallel_jobs"`
	DefaultJobTimeoutSeconds  int  `yaml:"default_job_timeout_seconds"`
	RequireApprovalForNewJobs bool `yaml:"require_approval_for_new_jobs"`
}

type HeartbeatConfig struct {
	Enabled         bool                  `yaml:"enabled"`
	IntervalSeconds int                   `yaml:"interval_seconds"`
	Checks          HeartbeatChecksConfig `yaml:"checks"`
}

type HeartbeatChecksConfig struct {
	Ollama         bool `yaml:"ollama"`
	SQLite         bool `yaml:"sqlite"`
	Scheduler      bool `yaml:"scheduler"`
	Internet       bool `yaml:"internet"`
	Connectors     bool `yaml:"connectors"`
	Extensions     bool `yaml:"extensions"`
	DiskSpace      bool `yaml:"disk_space"`
	MemoryPressure bool `yaml:"memory_pressure"`
}

type ConnectorConfig struct {
	Enabled        bool                       `yaml:"enabled"`
	Kind           string                     `yaml:"kind"`
	Bind           string                     `yaml:"bind"`
	RequireToken   bool                       `yaml:"require_token"`
	TokenEnv       string                     `yaml:"token_env"`
	SecretRef      SecretRefConfig            `yaml:"secret_ref"`
	AllowedDomains []string                   `yaml:"allowed_domains"`
	Permissions    ConnectorPermissionsConfig `yaml:"permissions"`
	RateLimit      ConnectorRateLimitConfig   `yaml:"rate_limit"`
	MaxBodyBytes   int64                      `yaml:"max_body_bytes"`
}

type SecretRefConfig struct {
	Provider string `yaml:"provider" json:"provider"`
	Name     string `yaml:"name" json:"name"`
	Service  string `yaml:"service,omitempty" json:"service,omitempty"`
	Account  string `yaml:"account,omitempty" json:"account,omitempty"`
}

type ConnectorPermissionsConfig struct {
	Inbound    bool     `yaml:"inbound" json:"inbound"`
	Outbound   bool     `yaml:"outbound" json:"outbound"`
	Posting    bool     `yaml:"posting" json:"posting"`
	Trading    bool     `yaml:"trading" json:"trading"`
	Deployment bool     `yaml:"deployment" json:"deployment"`
	Mutating   bool     `yaml:"mutating" json:"mutating"`
	Scopes     []string `yaml:"scopes" json:"scopes"`
}

type ConnectorRateLimitConfig struct {
	RequestsPerMinute int `yaml:"requests_per_minute" json:"requestsPerMinute"`
	RequestsPerDay    int `yaml:"requests_per_day" json:"requestsPerDay"`
	Burst             int `yaml:"burst" json:"burst"`
}

type MemoryConfig struct {
	Database            string `yaml:"database"`
	FTSEnabled          bool   `yaml:"fts_enabled"`
	SummarizeAfter      int    `yaml:"summarize_after_messages"`
	MaxRelevantMemories int    `yaml:"max_relevant_memories"`
}

type KnowledgeGraphConfig struct {
	Enabled              bool `yaml:"enabled"`
	InfluenceEnabled     bool `yaml:"influence_enabled"`
	MaxEntitiesPerQuery  int  `yaml:"max_entities_per_query"`
	MaxEvidenceChars     int  `yaml:"max_evidence_chars"`
	MaxInfluenceEntities int  `yaml:"max_influence_entities"`
	MaxInfluenceChars    int  `yaml:"max_influence_chars"`
	ManualOnly           bool `yaml:"manual_only"`
}

type WorkspaceConfig struct {
	MaxFilesScanned   int   `yaml:"max_files_scanned"`
	MaxFileBytes      int64 `yaml:"max_file_bytes"`
	MaxTotalScanBytes int64 `yaml:"max_total_scan_bytes"`
	MaxSearchResults  int   `yaml:"max_search_results"`
	MaxContextFiles   int   `yaml:"max_context_files"`
	MaxContextChars   int   `yaml:"max_context_chars"`
	IncludeHidden     bool  `yaml:"include_hidden"`
	FollowSymlinks    bool  `yaml:"follow_symlinks"`
}

type RAGConfig struct {
	Enabled          bool            `yaml:"enabled"`
	Mode             string          `yaml:"mode"`
	ChunkSize        int             `yaml:"chunk_size"`
	ChunkOverlap     int             `yaml:"chunk_overlap"`
	TopK             int             `yaml:"top_k"`
	MaxFileBytes     int64           `yaml:"max_file_bytes"`
	MaxTotalBytes    int64           `yaml:"max_total_bytes"`
	MaxChunksPerFile int             `yaml:"max_chunks_per_file"`
	Rerank           RerankConfig    `yaml:"rerank"`
	Embeddings       EmbeddingConfig `yaml:"embeddings"`
	VectorDB         VectorDBConfig  `yaml:"vector_db"`
}

type RerankConfig struct {
	CandidateLimit  int `yaml:"candidate_limit"`
	SourceDiversity int `yaml:"source_diversity"`
}

type EmbeddingConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Provider       string `yaml:"provider"`
	Model          string `yaml:"model"`
	BatchSize      int    `yaml:"batch_size"`
	MaxTextChars   int    `yaml:"max_text_chars"`
	CandidateLimit int    `yaml:"candidate_limit"`
}

type VectorDBConfig struct {
	Enabled                bool   `yaml:"enabled"`
	Provider               string `yaml:"provider"`
	ResearchOnly           bool   `yaml:"research_only"`
	LocalOnly              bool   `yaml:"local_only"`
	AllowBackgroundService bool   `yaml:"allow_background_service"`
	MaxRAMMB               int    `yaml:"max_ram_mb"`
	MaxStorageMB           int    `yaml:"max_storage_mb"`
	MinBenefitPercent      int    `yaml:"min_benefit_percent"`
}

type ToolsConfig struct {
	Filesystem FilesystemConfig `yaml:"filesystem"`
	Shell      ShellConfig      `yaml:"shell"`
}

type FilesystemConfig struct {
	WorkspaceOnly       bool  `yaml:"workspace_only"`
	SnapshotBeforeWrite bool  `yaml:"snapshot_before_write"`
	RequireConfirmation bool  `yaml:"require_confirmation"`
	MaxEditFileBytes    int64 `yaml:"max_edit_file_bytes"`
	MaxFullRewriteBytes int64 `yaml:"max_full_rewrite_bytes"`
}

type ShellConfig struct {
	Enabled                  bool     `yaml:"enabled"`
	TimeoutSeconds           int      `yaml:"timeout_seconds"`
	MaxOutputBytes           int      `yaml:"max_output_bytes"`
	MaxParallelCommands      int      `yaml:"max_parallel_commands"`
	RequireConfirmationRisky bool     `yaml:"require_confirmation_for_risky"`
	RequireConfirmationFor   []string `yaml:"require_confirmation_for"`
}

type SecurityConfig struct {
	AllowNetworkByDefault bool         `yaml:"allow_network_by_default"`
	SecretsAccess         bool         `yaml:"secrets_access"`
	Policy                PolicyConfig `yaml:"policy"`
}

type PolicyConfig struct {
	Mode                         string `yaml:"mode"`
	AuditEnabled                 bool   `yaml:"audit_enabled"`
	MaxAutonomousLevel           int    `yaml:"max_autonomous_level"`
	AllowAutonomousPosting       bool   `yaml:"allow_autonomous_posting"`
	AllowAutonomousDeployment    bool   `yaml:"allow_autonomous_deployment"`
	AllowLiveTrading             bool   `yaml:"allow_live_trading"`
	GeneratedCanModifyCorePolicy bool   `yaml:"generated_can_modify_core_policy"`
}

func LoadOrCreate() (*Config, error) {
	dir, err := SupportDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, ConfigName)

	cfg := Default()
	cfg.Path = path
	cfg.Memory.Database = filepath.Join(dir, "profiles", cfg.App.Profile, "memory.sqlite")
	cfg.Extensions.GeneratedDir = filepath.Join(dir, "profiles", cfg.App.Profile, "extensions", "generated")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := Write(path, cfg); err != nil {
			return nil, err
		}
		return cfg, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	ApplyDefaults(cfg)
	cfg.Path = path
	if cfg.Memory.Database == "" {
		cfg.Memory.Database = filepath.Join(dir, "profiles", cfg.App.Profile, "memory.sqlite")
	}
	cfg.Memory.Database = ExpandPath(cfg.Memory.Database)
	if cfg.Extensions.GeneratedDir == "" {
		cfg.Extensions.GeneratedDir = filepath.Join(dir, "profiles", cfg.App.Profile, "extensions", "generated")
	}
	cfg.Extensions.GeneratedDir = ExpandPath(cfg.Extensions.GeneratedDir)

	return cfg, nil
}

func Write(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func Default() *Config {
	return &Config{
		App: AppConfig{
			Name:          AppName,
			Mode:          "local_first",
			Profile:       DefaultProfile,
			SetupComplete: false,
			Telemetry:     false,
		},
		UI: UIConfig{
			Theme: "system",
		},
		Runtime: RuntimeConfig{
			LowMemoryMode:     true,
			MaxParallelTools:  1,
			MaxContextTokens:  4096,
			AutoSummarize:     true,
			ResponseMode:      ResponseModeBalanced,
			ShowThinkingTrace: false,
		},
		Models: map[string]ModelConfig{
			"default": {
				Provider:    "ollama",
				Name:        "qwen2.5:3b",
				BaseURL:     "http://localhost:11434/api",
				Temperature: 0.2,
			},
			"low_memory": {
				Provider:    "ollama",
				Name:        "deepseek-r1:1.5b",
				BaseURL:     "http://localhost:11434/api",
				Temperature: 0.1,
			},
			"coding": {
				Provider:    "ollama",
				Name:        "qwen2.5-coder:3b",
				BaseURL:     "http://localhost:11434/api",
				Temperature: 0.1,
			},
			"reasoning": {
				Provider:    "ollama",
				Name:        "deepseek-r1:1.5b",
				BaseURL:     "http://localhost:11434/api",
				Temperature: 0.1,
			},
			"stronger_local": {
				Provider:    "ollama",
				Name:        "phi3:mini",
				BaseURL:     "http://localhost:11434/api",
				Temperature: 0.2,
			},
		},
		CloudFallback: CloudFallbackConfig{
			Enabled:        false,
			Provider:       "openai_compatible",
			Name:           "",
			BaseURL:        "http://localhost:4000/v1",
			APIKeyEnv:      "",
			Temperature:    0.2,
			TimeoutSeconds: 45,
		},
		Connectors: ConnectorsConfig{
			Enabled: false,
			LocalAPI: ConnectorConfig{
				Enabled:      false,
				Kind:         "local_api",
				Bind:         "loopback",
				RequireToken: true,
				TokenEnv:     "YEMAKA_CONNECTOR_TOKEN",
				MaxBodyBytes: 65536,
				SecretRef: SecretRefConfig{
					Provider: "env",
					Name:     "YEMAKA_CONNECTOR_TOKEN",
				},
				AllowedDomains: []string{"localhost", "127.0.0.1"},
				Permissions: ConnectorPermissionsConfig{
					Inbound: true,
					Scopes:  []string{"chat", "ask", "memory_search"},
				},
				RateLimit: ConnectorRateLimitConfig{
					RequestsPerMinute: 60,
					RequestsPerDay:    1000,
					Burst:             5,
				},
			},
			MCPServer: ConnectorConfig{
				Enabled:      false,
				Kind:         "mcp_server",
				Bind:         "stdio",
				RequireToken: false,
				MaxBodyBytes: 65536,
				SecretRef: SecretRefConfig{
					Provider: "none",
				},
				Permissions: ConnectorPermissionsConfig{
					Inbound: true,
					Scopes:  []string{"chat", "ask", "memory_search"},
				},
				RateLimit: ConnectorRateLimitConfig{
					RequestsPerMinute: 60,
					RequestsPerDay:    1000,
					Burst:             5,
				},
			},
			Slack: ConnectorConfig{
				Enabled:      false,
				Kind:         "slack",
				Bind:         "webhook",
				RequireToken: true,
				TokenEnv:     "YEMAKA_SLACK_CONNECTOR_TOKEN",
				MaxBodyBytes: 65536,
				SecretRef: SecretRefConfig{
					Provider: "env",
					Name:     "YEMAKA_SLACK_CONNECTOR_TOKEN",
				},
				AllowedDomains: []string{"slack.com", "hooks.slack.com", "api.slack.com"},
				Permissions: ConnectorPermissionsConfig{
					Inbound: true,
					Scopes:  []string{"chat", "ask"},
				},
				RateLimit: ConnectorRateLimitConfig{RequestsPerMinute: 30, RequestsPerDay: 500, Burst: 3},
			},
			Discord: ConnectorConfig{
				Enabled:      false,
				Kind:         "discord",
				Bind:         "webhook",
				RequireToken: true,
				TokenEnv:     "YEMAKA_DISCORD_CONNECTOR_TOKEN",
				MaxBodyBytes: 65536,
				SecretRef: SecretRefConfig{
					Provider: "env",
					Name:     "YEMAKA_DISCORD_CONNECTOR_TOKEN",
				},
				AllowedDomains: []string{"discord.com", "discordapp.com"},
				Permissions: ConnectorPermissionsConfig{
					Inbound: true,
					Scopes:  []string{"chat", "ask"},
				},
				RateLimit: ConnectorRateLimitConfig{RequestsPerMinute: 30, RequestsPerDay: 500, Burst: 3},
			},
			Telegram: ConnectorConfig{
				Enabled:      false,
				Kind:         "telegram",
				Bind:         "webhook",
				RequireToken: true,
				TokenEnv:     "YEMAKA_TELEGRAM_CONNECTOR_TOKEN",
				MaxBodyBytes: 65536,
				SecretRef: SecretRefConfig{
					Provider: "env",
					Name:     "YEMAKA_TELEGRAM_CONNECTOR_TOKEN",
				},
				AllowedDomains: []string{"api.telegram.org"},
				Permissions: ConnectorPermissionsConfig{
					Inbound: true,
					Scopes:  []string{"chat", "ask"},
				},
				RateLimit: ConnectorRateLimitConfig{RequestsPerMinute: 30, RequestsPerDay: 500, Burst: 3},
			},
			Email: ConnectorConfig{
				Enabled:      false,
				Kind:         "email",
				Bind:         "webhook",
				RequireToken: true,
				TokenEnv:     "YEMAKA_EMAIL_CONNECTOR_TOKEN",
				MaxBodyBytes: 65536,
				SecretRef: SecretRefConfig{
					Provider: "env",
					Name:     "YEMAKA_EMAIL_CONNECTOR_TOKEN",
				},
				Permissions: ConnectorPermissionsConfig{
					Inbound: true,
					Scopes:  []string{"chat", "ask"},
				},
				RateLimit: ConnectorRateLimitConfig{RequestsPerMinute: 30, RequestsPerDay: 500, Burst: 3},
			},
		},
		Extensions: ExtensionsConfig{
			Enabled:                 true,
			RequireTests:            true,
			RegisterOnlyIfTestsPass: true,
			RunOutOfProcess:         true,
			MaxRuntimeSeconds:       60,
			PreferredLanguage:       "go",
		},
		Internet: InternetConfig{
			Enabled:          false,
			DefaultMode:      "ask_each_time",
			AllowMethods:     []string{"GET", "HEAD"},
			MaxResponseBytes: 2000000,
			TimeoutSeconds:   20,
			RedirectLimit:    3,
			Cache: InternetCacheConfig{
				Enabled:    true,
				TTLSeconds: 3600,
			},
			Search: InternetSearchConfig{
				Enabled:           false,
				Provider:          "none",
				FallbackProviders: []string{"tavily", "serper", "brave", "firecrawl", "wikimedia", "duckduckgo", "searxng"},
				Endpoint:          "",
				APIKeyEnv:         "",
				MaxResults:        10,
				SafeSearch:        false,
			},
			Policy: InternetPolicyConfig{
				RespectRobotsTxt:           true,
				BlockPrivateIPRanges:       true,
				BlockLocalNetworkByDefault: true,
				LogRequests:                true,
			},
		},
		Scheduler: SchedulerConfig{
			Enabled:                   true,
			MaxParallelJobs:           1,
			LowMemoryMaxParallelJobs:  1,
			DefaultJobTimeoutSeconds:  60,
			RequireApprovalForNewJobs: true,
		},
		Heartbeat: HeartbeatConfig{
			Enabled:         true,
			IntervalSeconds: 60,
			Checks: HeartbeatChecksConfig{
				Ollama:         true,
				SQLite:         true,
				Scheduler:      true,
				Internet:       true,
				Connectors:     true,
				Extensions:     true,
				DiskSpace:      true,
				MemoryPressure: true,
			},
		},
		Memory: MemoryConfig{
			FTSEnabled:          true,
			SummarizeAfter:      20,
			MaxRelevantMemories: 6,
		},
		Knowledge: KnowledgeGraphConfig{
			Enabled:              false,
			InfluenceEnabled:     false,
			MaxEntitiesPerQuery:  12,
			MaxEvidenceChars:     2000,
			MaxInfluenceEntities: 4,
			MaxInfluenceChars:    1200,
			ManualOnly:           true,
		},
		Workspace: WorkspaceConfig{
			MaxFilesScanned:   2000,
			MaxFileBytes:      200000,
			MaxTotalScanBytes: 20000000,
			MaxSearchResults:  20,
			MaxContextFiles:   6,
			MaxContextChars:   10000,
			IncludeHidden:     false,
			FollowSymlinks:    false,
		},
		RAG: RAGConfig{
			Enabled:          true,
			Mode:             "sqlite_fts",
			ChunkSize:        700,
			ChunkOverlap:     100,
			TopK:             5,
			MaxFileBytes:     300000,
			MaxTotalBytes:    50000000,
			MaxChunksPerFile: 200,
			Rerank: RerankConfig{
				CandidateLimit:  24,
				SourceDiversity: 2,
			},
			Embeddings: EmbeddingConfig{
				Enabled:        false,
				Provider:       "ollama",
				Model:          "nomic-embed-text",
				BatchSize:      8,
				MaxTextChars:   2000,
				CandidateLimit: 200,
			},
			VectorDB: VectorDBConfig{
				Enabled:                false,
				Provider:               "research_only",
				ResearchOnly:           true,
				LocalOnly:              true,
				AllowBackgroundService: false,
				MaxRAMMB:               512,
				MaxStorageMB:           1024,
				MinBenefitPercent:      20,
			},
		},
		Tools: ToolsConfig{
			Filesystem: FilesystemConfig{
				WorkspaceOnly:       true,
				SnapshotBeforeWrite: true,
				RequireConfirmation: true,
				MaxEditFileBytes:    200000,
				MaxFullRewriteBytes: 20000,
			},
			Shell: ShellConfig{
				Enabled:                  true,
				TimeoutSeconds:           30,
				MaxOutputBytes:           100000,
				MaxParallelCommands:      1,
				RequireConfirmationRisky: true,
				RequireConfirmationFor: []string{
					"npm install",
					"pnpm install",
					"pip install",
					"brew install",
					"git clean",
					"git reset",
					"git push",
					"rm",
					"mv",
					"chmod",
					"chown",
					"sudo",
					"curl",
					"wget",
					"docker",
					"ssh",
					"scp",
				},
			},
		},
		Security: SecurityConfig{
			AllowNetworkByDefault: false,
			SecretsAccess:         false,
			Policy: PolicyConfig{
				Mode:                         "safe",
				AuditEnabled:                 true,
				MaxAutonomousLevel:           4,
				AllowAutonomousPosting:       false,
				AllowAutonomousDeployment:    false,
				AllowLiveTrading:             false,
				GeneratedCanModifyCorePolicy: false,
			},
		},
	}
}

func ApplyDefaults(cfg *Config) {
	defaults := Default()
	if cfg.App.Name == "" {
		cfg.App.Name = defaults.App.Name
	}
	if cfg.App.Mode == "" {
		cfg.App.Mode = defaults.App.Mode
	}
	if cfg.App.Profile == "" {
		cfg.App.Profile = defaults.App.Profile
	}
	cfg.UI.Theme = NormalizeUITheme(cfg.UI.Theme)
	if cfg.Runtime.MaxParallelTools == 0 {
		cfg.Runtime.MaxParallelTools = defaults.Runtime.MaxParallelTools
	}
	if cfg.Runtime.MaxContextTokens == 0 {
		cfg.Runtime.MaxContextTokens = defaults.Runtime.MaxContextTokens
	}
	cfg.Runtime.ResponseMode = NormalizeResponseMode(cfg.Runtime.ResponseMode)
	if cfg.Models == nil {
		cfg.Models = map[string]ModelConfig{}
	}
	for key, model := range defaults.Models {
		existing, ok := cfg.Models[key]
		if !ok {
			cfg.Models[key] = model
			continue
		}
		if existing.Provider == "" {
			existing.Provider = model.Provider
		}
		if existing.Name == "" {
			existing.Name = model.Name
		}
		if existing.BaseURL == "" {
			existing.BaseURL = model.BaseURL
		}
		if existing.Temperature == 0 {
			existing.Temperature = model.Temperature
		}
		cfg.Models[key] = existing
	}
	if cfg.CloudFallback.Provider == "" {
		cfg.CloudFallback.Provider = defaults.CloudFallback.Provider
	}
	if cfg.CloudFallback.BaseURL == "" {
		cfg.CloudFallback.BaseURL = defaults.CloudFallback.BaseURL
	}
	if cfg.CloudFallback.Temperature == 0 {
		cfg.CloudFallback.Temperature = defaults.CloudFallback.Temperature
	}
	if cfg.CloudFallback.TimeoutSeconds == 0 {
		cfg.CloudFallback.TimeoutSeconds = defaults.CloudFallback.TimeoutSeconds
	}
	if cfg.Connectors.LocalAPI.Kind == "" {
		cfg.Connectors.LocalAPI.Kind = defaults.Connectors.LocalAPI.Kind
	}
	if cfg.Connectors.LocalAPI.Bind == "" {
		cfg.Connectors.LocalAPI.Bind = defaults.Connectors.LocalAPI.Bind
	}
	cfg.Connectors.LocalAPI.RequireToken = true
	if cfg.Connectors.LocalAPI.TokenEnv == "" {
		cfg.Connectors.LocalAPI.TokenEnv = defaults.Connectors.LocalAPI.TokenEnv
	}
	cfg.Connectors.LocalAPI = applyConnectorV2Defaults(cfg.Connectors.LocalAPI, defaults.Connectors.LocalAPI)
	if cfg.Connectors.LocalAPI.MaxBodyBytes == 0 {
		cfg.Connectors.LocalAPI.MaxBodyBytes = defaults.Connectors.LocalAPI.MaxBodyBytes
	}
	if cfg.Connectors.MCPServer.Kind == "" {
		cfg.Connectors.MCPServer.Kind = defaults.Connectors.MCPServer.Kind
	}
	if cfg.Connectors.MCPServer.Bind == "" {
		cfg.Connectors.MCPServer.Bind = defaults.Connectors.MCPServer.Bind
	}
	cfg.Connectors.MCPServer.RequireToken = false
	if cfg.Connectors.MCPServer.MaxBodyBytes == 0 {
		cfg.Connectors.MCPServer.MaxBodyBytes = defaults.Connectors.MCPServer.MaxBodyBytes
	}
	cfg.Connectors.MCPServer = applyConnectorV2Defaults(cfg.Connectors.MCPServer, defaults.Connectors.MCPServer)
	cfg.Connectors.Slack = applyConnectorDefaults(cfg.Connectors.Slack, defaults.Connectors.Slack, true)
	cfg.Connectors.Discord = applyConnectorDefaults(cfg.Connectors.Discord, defaults.Connectors.Discord, true)
	cfg.Connectors.Telegram = applyConnectorDefaults(cfg.Connectors.Telegram, defaults.Connectors.Telegram, true)
	cfg.Connectors.Email = applyConnectorDefaults(cfg.Connectors.Email, defaults.Connectors.Email, true)
	if !cfg.Extensions.Enabled {
		cfg.Extensions.Enabled = defaults.Extensions.Enabled
	}
	if !cfg.Extensions.RequireTests {
		cfg.Extensions.RequireTests = defaults.Extensions.RequireTests
	}
	if !cfg.Extensions.RegisterOnlyIfTestsPass {
		cfg.Extensions.RegisterOnlyIfTestsPass = defaults.Extensions.RegisterOnlyIfTestsPass
	}
	if !cfg.Extensions.RunOutOfProcess {
		cfg.Extensions.RunOutOfProcess = defaults.Extensions.RunOutOfProcess
	}
	if cfg.Extensions.MaxRuntimeSeconds == 0 {
		cfg.Extensions.MaxRuntimeSeconds = defaults.Extensions.MaxRuntimeSeconds
	}
	if cfg.Extensions.PreferredLanguage == "" {
		cfg.Extensions.PreferredLanguage = defaults.Extensions.PreferredLanguage
	}
	if cfg.Internet.DefaultMode == "" {
		cfg.Internet.DefaultMode = defaults.Internet.DefaultMode
	}
	if len(cfg.Internet.AllowMethods) == 0 {
		cfg.Internet.AllowMethods = defaults.Internet.AllowMethods
	}
	cfg.Internet.AllowMethods = normalizeInternetMethods(cfg.Internet.AllowMethods, defaults.Internet.AllowMethods)
	if cfg.Internet.MaxResponseBytes == 0 {
		cfg.Internet.MaxResponseBytes = defaults.Internet.MaxResponseBytes
	}
	if cfg.Internet.TimeoutSeconds == 0 {
		cfg.Internet.TimeoutSeconds = defaults.Internet.TimeoutSeconds
	}
	if cfg.Internet.RedirectLimit == 0 {
		cfg.Internet.RedirectLimit = defaults.Internet.RedirectLimit
	}
	if !cfg.Internet.Cache.Enabled {
		cfg.Internet.Cache.Enabled = defaults.Internet.Cache.Enabled
	}
	if cfg.Internet.Cache.TTLSeconds == 0 {
		cfg.Internet.Cache.TTLSeconds = defaults.Internet.Cache.TTLSeconds
	}
	if cfg.Internet.Search.Provider == "" {
		cfg.Internet.Search.Provider = defaults.Internet.Search.Provider
	}
	if len(cfg.Internet.Search.FallbackProviders) == 0 {
		cfg.Internet.Search.FallbackProviders = append([]string{}, defaults.Internet.Search.FallbackProviders...)
	}
	if cfg.Internet.Search.MaxResults == 0 {
		cfg.Internet.Search.MaxResults = defaults.Internet.Search.MaxResults
	}
	cfg.Internet.Search.SafeSearch = true
	cfg.Internet.Policy.RespectRobotsTxt = true
	cfg.Internet.Policy.BlockPrivateIPRanges = true
	cfg.Internet.Policy.BlockLocalNetworkByDefault = true
	cfg.Internet.Policy.LogRequests = true
	if schedulerLooksUnset(cfg.Scheduler) {
		cfg.Scheduler.Enabled = defaults.Scheduler.Enabled
		cfg.Scheduler.RequireApprovalForNewJobs = defaults.Scheduler.RequireApprovalForNewJobs
	}
	if cfg.Scheduler.MaxParallelJobs == 0 {
		cfg.Scheduler.MaxParallelJobs = defaults.Scheduler.MaxParallelJobs
	}
	if cfg.Scheduler.LowMemoryMaxParallelJobs == 0 {
		cfg.Scheduler.LowMemoryMaxParallelJobs = defaults.Scheduler.LowMemoryMaxParallelJobs
	}
	if cfg.Scheduler.DefaultJobTimeoutSeconds == 0 {
		cfg.Scheduler.DefaultJobTimeoutSeconds = defaults.Scheduler.DefaultJobTimeoutSeconds
	}
	if heartbeatLooksUnset(cfg.Heartbeat) {
		cfg.Heartbeat.Enabled = defaults.Heartbeat.Enabled
		cfg.Heartbeat.Checks = defaults.Heartbeat.Checks
	}
	if cfg.Heartbeat.IntervalSeconds == 0 {
		cfg.Heartbeat.IntervalSeconds = defaults.Heartbeat.IntervalSeconds
	}
	cfg.Heartbeat.Checks = applyHeartbeatCheckDefaults(cfg.Heartbeat.Checks, defaults.Heartbeat.Checks)
	if cfg.Memory.SummarizeAfter == 0 {
		cfg.Memory.SummarizeAfter = defaults.Memory.SummarizeAfter
	}
	if cfg.Memory.MaxRelevantMemories == 0 {
		cfg.Memory.MaxRelevantMemories = defaults.Memory.MaxRelevantMemories
	}
	if cfg.Knowledge.MaxEntitiesPerQuery == 0 {
		cfg.Knowledge.MaxEntitiesPerQuery = defaults.Knowledge.MaxEntitiesPerQuery
	}
	if cfg.Knowledge.MaxEvidenceChars == 0 {
		cfg.Knowledge.MaxEvidenceChars = defaults.Knowledge.MaxEvidenceChars
	}
	if cfg.Knowledge.MaxInfluenceEntities == 0 {
		cfg.Knowledge.MaxInfluenceEntities = defaults.Knowledge.MaxInfluenceEntities
	}
	if cfg.Knowledge.MaxInfluenceChars == 0 {
		cfg.Knowledge.MaxInfluenceChars = defaults.Knowledge.MaxInfluenceChars
	}
	if !cfg.Knowledge.Enabled {
		cfg.Knowledge.InfluenceEnabled = false
	}
	cfg.Knowledge.ManualOnly = true
	if cfg.Workspace.MaxFilesScanned == 0 {
		cfg.Workspace.MaxFilesScanned = defaults.Workspace.MaxFilesScanned
	}
	if cfg.Workspace.MaxFileBytes == 0 {
		cfg.Workspace.MaxFileBytes = defaults.Workspace.MaxFileBytes
	}
	if cfg.Workspace.MaxTotalScanBytes == 0 {
		cfg.Workspace.MaxTotalScanBytes = defaults.Workspace.MaxTotalScanBytes
	}
	if cfg.Workspace.MaxSearchResults == 0 {
		cfg.Workspace.MaxSearchResults = defaults.Workspace.MaxSearchResults
	}
	if cfg.Workspace.MaxContextFiles == 0 {
		cfg.Workspace.MaxContextFiles = defaults.Workspace.MaxContextFiles
	}
	if cfg.Workspace.MaxContextChars == 0 {
		cfg.Workspace.MaxContextChars = defaults.Workspace.MaxContextChars
	}
	if cfg.RAG.Mode == "" {
		cfg.RAG.Mode = defaults.RAG.Mode
	}
	if cfg.RAG.ChunkSize == 0 {
		cfg.RAG.ChunkSize = defaults.RAG.ChunkSize
	}
	if cfg.RAG.ChunkOverlap == 0 {
		cfg.RAG.ChunkOverlap = defaults.RAG.ChunkOverlap
	}
	if cfg.RAG.TopK == 0 {
		cfg.RAG.TopK = defaults.RAG.TopK
	}
	if cfg.RAG.MaxFileBytes == 0 {
		cfg.RAG.MaxFileBytes = defaults.RAG.MaxFileBytes
	}
	if cfg.RAG.MaxTotalBytes == 0 {
		cfg.RAG.MaxTotalBytes = defaults.RAG.MaxTotalBytes
	}
	if cfg.RAG.MaxChunksPerFile == 0 {
		cfg.RAG.MaxChunksPerFile = defaults.RAG.MaxChunksPerFile
	}
	if cfg.RAG.Rerank.CandidateLimit == 0 {
		cfg.RAG.Rerank.CandidateLimit = defaults.RAG.Rerank.CandidateLimit
	}
	if cfg.RAG.Rerank.SourceDiversity == 0 {
		cfg.RAG.Rerank.SourceDiversity = defaults.RAG.Rerank.SourceDiversity
	}
	if cfg.RAG.Embeddings.Provider == "" {
		cfg.RAG.Embeddings.Provider = defaults.RAG.Embeddings.Provider
	}
	if cfg.RAG.Embeddings.Model == "" {
		cfg.RAG.Embeddings.Model = defaults.RAG.Embeddings.Model
	}
	if cfg.RAG.Embeddings.BatchSize == 0 {
		cfg.RAG.Embeddings.BatchSize = defaults.RAG.Embeddings.BatchSize
	}
	if cfg.RAG.Embeddings.MaxTextChars == 0 {
		cfg.RAG.Embeddings.MaxTextChars = defaults.RAG.Embeddings.MaxTextChars
	}
	if cfg.RAG.Embeddings.CandidateLimit == 0 {
		cfg.RAG.Embeddings.CandidateLimit = defaults.RAG.Embeddings.CandidateLimit
	}
	if cfg.RAG.VectorDB.Provider == "" {
		cfg.RAG.VectorDB.Provider = defaults.RAG.VectorDB.Provider
	}
	if !cfg.RAG.VectorDB.ResearchOnly {
		cfg.RAG.VectorDB.ResearchOnly = defaults.RAG.VectorDB.ResearchOnly
	}
	if !cfg.RAG.VectorDB.LocalOnly {
		cfg.RAG.VectorDB.LocalOnly = defaults.RAG.VectorDB.LocalOnly
	}
	if cfg.RAG.VectorDB.MaxRAMMB == 0 {
		cfg.RAG.VectorDB.MaxRAMMB = defaults.RAG.VectorDB.MaxRAMMB
	}
	if cfg.RAG.VectorDB.MaxStorageMB == 0 {
		cfg.RAG.VectorDB.MaxStorageMB = defaults.RAG.VectorDB.MaxStorageMB
	}
	if cfg.RAG.VectorDB.MinBenefitPercent == 0 {
		cfg.RAG.VectorDB.MinBenefitPercent = defaults.RAG.VectorDB.MinBenefitPercent
	}
	if cfg.Tools.Filesystem.MaxEditFileBytes == 0 {
		cfg.Tools.Filesystem.MaxEditFileBytes = defaults.Tools.Filesystem.MaxEditFileBytes
	}
	if cfg.Tools.Filesystem.MaxFullRewriteBytes == 0 {
		cfg.Tools.Filesystem.MaxFullRewriteBytes = defaults.Tools.Filesystem.MaxFullRewriteBytes
	}
	if !cfg.Tools.Filesystem.WorkspaceOnly {
		cfg.Tools.Filesystem.WorkspaceOnly = defaults.Tools.Filesystem.WorkspaceOnly
	}
	if !cfg.Tools.Filesystem.SnapshotBeforeWrite {
		cfg.Tools.Filesystem.SnapshotBeforeWrite = defaults.Tools.Filesystem.SnapshotBeforeWrite
	}
	if !cfg.Tools.Filesystem.RequireConfirmation {
		cfg.Tools.Filesystem.RequireConfirmation = defaults.Tools.Filesystem.RequireConfirmation
	}
	if !cfg.Tools.Shell.Enabled {
		cfg.Tools.Shell.Enabled = defaults.Tools.Shell.Enabled
	}
	if cfg.Tools.Shell.TimeoutSeconds == 0 {
		cfg.Tools.Shell.TimeoutSeconds = defaults.Tools.Shell.TimeoutSeconds
	}
	if cfg.Tools.Shell.MaxOutputBytes == 0 {
		cfg.Tools.Shell.MaxOutputBytes = defaults.Tools.Shell.MaxOutputBytes
	}
	if cfg.Tools.Shell.MaxParallelCommands == 0 {
		cfg.Tools.Shell.MaxParallelCommands = defaults.Tools.Shell.MaxParallelCommands
	}
	if !cfg.Tools.Shell.RequireConfirmationRisky {
		cfg.Tools.Shell.RequireConfirmationRisky = defaults.Tools.Shell.RequireConfirmationRisky
	}
	if len(cfg.Tools.Shell.RequireConfirmationFor) == 0 {
		cfg.Tools.Shell.RequireConfirmationFor = defaults.Tools.Shell.RequireConfirmationFor
	}
	if !cfg.Security.Policy.AuditEnabled {
		cfg.Security.Policy.AuditEnabled = defaults.Security.Policy.AuditEnabled
	}
	cfg.Security.Policy.Mode = normalizePolicyMode(cfg.Security.Policy.Mode)
	if cfg.Security.Policy.MaxAutonomousLevel == 0 {
		cfg.Security.Policy.MaxAutonomousLevel = defaults.Security.Policy.MaxAutonomousLevel
	}
	cfg.Security.Policy.AllowAutonomousPosting = false
	cfg.Security.Policy.AllowAutonomousDeployment = false
	cfg.Security.Policy.AllowLiveTrading = false
	cfg.Security.Policy.GeneratedCanModifyCorePolicy = false
}

func NormalizeUITheme(theme string) string {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return "system"
	}
}

func NormalizeResponseMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ResponseModeAuto:
		return ResponseModeAuto
	case ResponseModeFast:
		return ResponseModeFast
	case ResponseModeDeep:
		return ResponseModeDeep
	default:
		return ResponseModeBalanced
	}
}

func normalizePolicyMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "full_access":
		return "full_access"
	default:
		return "safe"
	}
}

func applyConnectorDefaults(cfg ConnectorConfig, defaults ConnectorConfig, requireToken bool) ConnectorConfig {
	if cfg.Kind == "" {
		cfg.Kind = defaults.Kind
	}
	if cfg.Bind == "" {
		cfg.Bind = defaults.Bind
	}
	if requireToken {
		cfg.RequireToken = true
	} else {
		cfg.RequireToken = defaults.RequireToken
	}
	if cfg.TokenEnv == "" {
		cfg.TokenEnv = defaults.TokenEnv
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = defaults.MaxBodyBytes
	}
	cfg = applyConnectorV2Defaults(cfg, defaults)
	return cfg
}

func applyConnectorV2Defaults(cfg ConnectorConfig, defaults ConnectorConfig) ConnectorConfig {
	if cfg.SecretRef.Provider == "" {
		cfg.SecretRef = defaults.SecretRef
	}
	if cfg.SecretRef.Provider == "env" && cfg.TokenEnv != "" && (cfg.SecretRef.Name == "" || cfg.SecretRef.Name == defaults.SecretRef.Name) {
		cfg.SecretRef.Name = cfg.TokenEnv
	}
	if cfg.SecretRef.Provider == "env" && cfg.SecretRef.Name == "" {
		if cfg.TokenEnv != "" {
			cfg.SecretRef.Name = cfg.TokenEnv
		} else {
			cfg.SecretRef.Name = defaults.SecretRef.Name
		}
	}
	if cfg.TokenEnv == "" && cfg.SecretRef.Provider == "env" && cfg.SecretRef.Name != "" {
		cfg.TokenEnv = cfg.SecretRef.Name
	}
	if len(cfg.AllowedDomains) == 0 {
		cfg.AllowedDomains = append([]string{}, defaults.AllowedDomains...)
	}
	if connectorPermissionsUnset(cfg.Permissions) {
		cfg.Permissions = defaults.Permissions
	}
	if cfg.RateLimit.RequestsPerMinute == 0 {
		cfg.RateLimit.RequestsPerMinute = defaults.RateLimit.RequestsPerMinute
	}
	if cfg.RateLimit.RequestsPerDay == 0 {
		cfg.RateLimit.RequestsPerDay = defaults.RateLimit.RequestsPerDay
	}
	if cfg.RateLimit.Burst == 0 {
		cfg.RateLimit.Burst = defaults.RateLimit.Burst
	}
	return cfg
}

func connectorPermissionsUnset(cfg ConnectorPermissionsConfig) bool {
	return !cfg.Inbound &&
		!cfg.Outbound &&
		!cfg.Posting &&
		!cfg.Trading &&
		!cfg.Deployment &&
		!cfg.Mutating &&
		len(cfg.Scopes) == 0
}

func schedulerLooksUnset(cfg SchedulerConfig) bool {
	return !cfg.Enabled &&
		cfg.MaxParallelJobs == 0 &&
		cfg.LowMemoryMaxParallelJobs == 0 &&
		cfg.DefaultJobTimeoutSeconds == 0 &&
		!cfg.RequireApprovalForNewJobs
}

func heartbeatLooksUnset(cfg HeartbeatConfig) bool {
	return !cfg.Enabled &&
		cfg.IntervalSeconds == 0 &&
		!cfg.Checks.Ollama &&
		!cfg.Checks.SQLite &&
		!cfg.Checks.Scheduler &&
		!cfg.Checks.Internet &&
		!cfg.Checks.Connectors &&
		!cfg.Checks.Extensions &&
		!cfg.Checks.DiskSpace &&
		!cfg.Checks.MemoryPressure
}

func applyHeartbeatCheckDefaults(cfg HeartbeatChecksConfig, defaults HeartbeatChecksConfig) HeartbeatChecksConfig {
	if !cfg.Ollama && !cfg.SQLite && !cfg.Scheduler && !cfg.Internet && !cfg.Connectors && !cfg.Extensions && !cfg.DiskSpace && !cfg.MemoryPressure {
		return defaults
	}
	return cfg
}

func normalizeInternetMethods(methods []string, fallback []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, method := range methods {
		switch normalized := strings.ToUpper(strings.TrimSpace(method)); normalized {
		case "GET", "HEAD":
			if !seen[normalized] {
				result = append(result, normalized)
				seen[normalized] = true
			}
		}
	}
	if len(result) == 0 {
		return append([]string{}, fallback...)
	}
	return result
}

func SupportDir() (string, error) {
	if custom := os.Getenv(EnvHome); custom != "" {
		return ExpandPath(custom), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", AppName), nil
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, AppName), nil
		}
		return filepath.Join(home, "AppData", "Roaming", AppName), nil
	default:
		if dataHome := os.Getenv("XDG_DATA_HOME"); dataHome != "" {
			return filepath.Join(dataHome, strings.ToLower(AppName)), nil
		}
		return filepath.Join(home, ".local", "share", strings.ToLower(AppName)), nil
	}
}

func ExpandPath(path string) string {
	if path == "" {
		return path
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if len(path) > 2 && path[:2] == "~/" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
