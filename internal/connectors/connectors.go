package connectors

import (
	"context"
	"fmt"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/secrets"
)

const LocalAPI = "local_api"
const MCPServer = "mcp_server"
const Slack = "slack"
const Discord = "discord"
const Telegram = "telegram"
const Email = "email"

type Status struct {
	Name           string                            `json:"name"`
	Kind           string                            `json:"kind"`
	Enabled        bool                              `json:"enabled"`
	Bind           string                            `json:"bind"`
	RequireToken   bool                              `json:"requireToken"`
	TokenEnv       string                            `json:"tokenEnv"`
	TokenReady     bool                              `json:"tokenReady"`
	Secret         secrets.Status                    `json:"secret"`
	AllowedDomains []string                          `json:"allowedDomains"`
	Permissions    config.ConnectorPermissionsConfig `json:"permissions"`
	RateLimit      config.ConnectorRateLimitConfig   `json:"rateLimit"`
	MaxBodyBytes   int64                             `json:"maxBodyBytes"`
	Endpoint       string                            `json:"endpoint"`
	Health         Health                            `json:"health"`
}

type Health struct {
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type ToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func List(cfg config.ConnectorsConfig) []Status {
	return []Status{
		LocalAPIStatus(cfg),
		MCPServerStatus(cfg),
		AdapterStatus(cfg, Slack),
		AdapterStatus(cfg, Discord),
		AdapterStatus(cfg, Telegram),
		AdapterStatus(cfg, Email),
	}
}

func LocalAPIStatus(cfg config.ConnectorsConfig) Status {
	local := applyDefaults(cfg.LocalAPI)
	secret := secretStatus(local)
	tokenEnv := safeTokenEnv(local.TokenEnv)
	return Status{
		Name:           LocalAPI,
		Kind:           local.Kind,
		Enabled:        cfg.Enabled && local.Enabled,
		Bind:           local.Bind,
		RequireToken:   local.RequireToken,
		TokenEnv:       tokenEnv,
		TokenReady:     !local.RequireToken || secret.Ready,
		Secret:         secret,
		AllowedDomains: local.AllowedDomains,
		Permissions:    local.Permissions,
		RateLimit:      local.RateLimit,
		MaxBodyBytes:   local.MaxBodyBytes,
		Endpoint:       "/connectors/local/v1",
		Health:         healthFor(cfg.Enabled && local.Enabled, !local.RequireToken || secret.Ready, secret.Detail),
	}
}

func MCPServerStatus(cfg config.ConnectorsConfig) Status {
	mcp := applyMCPDefaults(cfg.MCPServer)
	secret := secretStatus(mcp)
	tokenEnv := safeTokenEnv(mcp.TokenEnv)
	return Status{
		Name:           MCPServer,
		Kind:           mcp.Kind,
		Enabled:        cfg.Enabled && mcp.Enabled,
		Bind:           mcp.Bind,
		RequireToken:   mcp.RequireToken,
		TokenEnv:       tokenEnv,
		TokenReady:     true,
		Secret:         secret,
		AllowedDomains: mcp.AllowedDomains,
		Permissions:    mcp.Permissions,
		RateLimit:      mcp.RateLimit,
		MaxBodyBytes:   mcp.MaxBodyBytes,
		Endpoint:       "stdio",
		Health:         healthFor(cfg.Enabled && mcp.Enabled, true, "stdio connector"),
	}
}

func AdapterStatus(cfg config.ConnectorsConfig, name string) Status {
	adapter, ok := AdapterConfig(cfg, name)
	if !ok {
		return Status{Name: strings.TrimSpace(name), Kind: strings.TrimSpace(name)}
	}
	adapter = applyAdapterDefaults(adapter, name)
	secret := secretStatus(adapter)
	tokenEnv := safeTokenEnv(adapter.TokenEnv)
	return Status{
		Name:           strings.TrimSpace(name),
		Kind:           adapter.Kind,
		Enabled:        cfg.Enabled && adapter.Enabled,
		Bind:           adapter.Bind,
		RequireToken:   adapter.RequireToken,
		TokenEnv:       tokenEnv,
		TokenReady:     !adapter.RequireToken || secret.Ready,
		Secret:         secret,
		AllowedDomains: adapter.AllowedDomains,
		Permissions:    adapter.Permissions,
		RateLimit:      adapter.RateLimit,
		MaxBodyBytes:   adapter.MaxBodyBytes,
		Endpoint:       "/connectors/adapters/v1/" + strings.TrimSpace(name),
		Health:         healthFor(cfg.Enabled && adapter.Enabled, !adapter.RequireToken || secret.Ready, secret.Detail),
	}
}

func EnableLocalAPI(cfg *config.Config, tokenEnv string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	local := applyDefaults(cfg.Connectors.LocalAPI)
	tokenEnv = strings.TrimSpace(tokenEnv)
	if tokenEnv != "" {
		if secrets.LooksLikeSecretValue(tokenEnv) {
			return fmt.Errorf("token env must be an environment variable name, not a secret value")
		}
		local.TokenEnv = tokenEnv
		local.SecretRef = config.SecretRefConfig{Provider: secrets.ProviderEnv, Name: tokenEnv}
	}
	if local.RequireToken && strings.TrimSpace(local.TokenEnv) == "" {
		return fmt.Errorf("token env is required for local_api connector")
	}
	cfg.Connectors.Enabled = true
	local.Enabled = true
	local.Kind = LocalAPI
	local.Bind = "loopback"
	cfg.Connectors.LocalAPI = local
	return nil
}

func EnableMCPServer(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	mcp := applyMCPDefaults(cfg.Connectors.MCPServer)
	cfg.Connectors.Enabled = true
	mcp.Enabled = true
	mcp.Kind = MCPServer
	mcp.Bind = "stdio"
	mcp.RequireToken = false
	mcp.TokenEnv = ""
	mcp.SecretRef = config.SecretRefConfig{Provider: secrets.ProviderNone}
	cfg.Connectors.MCPServer = mcp
	return nil
}

func EnableAdapter(cfg *config.Config, name string, tokenEnv string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !IsAdapter(name) {
		return fmt.Errorf("unknown adapter connector %q", name)
	}
	adapter, _ := AdapterConfig(cfg.Connectors, name)
	adapter = applyAdapterDefaults(adapter, name)
	tokenEnv = strings.TrimSpace(tokenEnv)
	if tokenEnv != "" {
		if secrets.LooksLikeSecretValue(tokenEnv) {
			return fmt.Errorf("token env must be an environment variable name, not a secret value")
		}
		adapter.TokenEnv = tokenEnv
		adapter.SecretRef = config.SecretRefConfig{Provider: secrets.ProviderEnv, Name: tokenEnv}
	}
	if adapter.RequireToken && strings.TrimSpace(adapter.TokenEnv) == "" {
		return fmt.Errorf("token env is required for %s connector", name)
	}
	cfg.Connectors.Enabled = true
	adapter.Enabled = true
	adapter.Kind = strings.TrimSpace(name)
	adapter.Bind = "webhook"
	setAdapterConfig(&cfg.Connectors, name, adapter)
	return nil
}

func DisableLocalAPI(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	local := applyDefaults(cfg.Connectors.LocalAPI)
	local.Enabled = false
	cfg.Connectors.LocalAPI = local
	cfg.Connectors.Enabled = anyEnabled(cfg.Connectors)
	return nil
}

func DisableMCPServer(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	mcp := applyMCPDefaults(cfg.Connectors.MCPServer)
	mcp.Enabled = false
	cfg.Connectors.MCPServer = mcp
	cfg.Connectors.Enabled = anyEnabled(cfg.Connectors)
	return nil
}

func DisableAdapter(cfg *config.Config, name string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if !IsAdapter(name) {
		return fmt.Errorf("unknown adapter connector %q", name)
	}
	adapter, _ := AdapterConfig(cfg.Connectors, name)
	adapter = applyAdapterDefaults(adapter, name)
	adapter.Enabled = false
	setAdapterConfig(&cfg.Connectors, name, adapter)
	cfg.Connectors.Enabled = anyEnabled(cfg.Connectors)
	return nil
}

func Enabled(cfg config.ConnectorsConfig, name string) bool {
	name = strings.TrimSpace(name)
	switch name {
	case LocalAPI:
		return cfg.Enabled && cfg.LocalAPI.Enabled
	case MCPServer:
		return cfg.Enabled && cfg.MCPServer.Enabled
	case Slack, Discord, Telegram, Email:
		adapter, _ := AdapterConfig(cfg, name)
		return cfg.Enabled && adapter.Enabled
	default:
		return false
	}
}

func ValidateName(name string) error {
	switch strings.TrimSpace(name) {
	case LocalAPI, MCPServer, Slack, Discord, Telegram, Email:
		return nil
	default:
		return fmt.Errorf("unknown connector %q; known connectors: %s", name, strings.Join(KnownNames(), ", "))
	}
}

func KnownNames() []string {
	return []string{LocalAPI, MCPServer, Slack, Discord, Telegram, Email}
}

func AdapterNames() []string {
	return []string{Slack, Discord, Telegram, Email}
}

func IsAdapter(name string) bool {
	switch strings.TrimSpace(name) {
	case Slack, Discord, Telegram, Email:
		return true
	default:
		return false
	}
}

func AdapterConfig(cfg config.ConnectorsConfig, name string) (config.ConnectorConfig, bool) {
	switch strings.TrimSpace(name) {
	case Slack:
		return cfg.Slack, true
	case Discord:
		return cfg.Discord, true
	case Telegram:
		return cfg.Telegram, true
	case Email:
		return cfg.Email, true
	default:
		return config.ConnectorConfig{}, false
	}
}

func Token(cfg config.ConnectorConfig) (string, bool) {
	cfg = applyDefaults(cfg)
	if !cfg.RequireToken {
		return "", true
	}
	ref, safe := safeSecretReference(cfg)
	if !safe {
		return "", false
	}
	value, ok, err := secrets.Resolve(context.Background(), ref)
	if err != nil {
		return "", false
	}
	return value, ok
}

func applyDefaults(cfg config.ConnectorConfig) config.ConnectorConfig {
	if cfg.Kind == "" {
		cfg.Kind = LocalAPI
	}
	if cfg.Bind == "" {
		cfg.Bind = "loopback"
	}
	cfg.RequireToken = true
	if cfg.TokenEnv == "" {
		cfg.TokenEnv = "YEMAKA_CONNECTOR_TOKEN"
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 65536
	}
	cfg = applySecurityDefaults(cfg, config.SecretRefConfig{Provider: secrets.ProviderEnv, Name: "YEMAKA_CONNECTOR_TOKEN"}, []string{"localhost", "127.0.0.1"}, []string{"chat", "ask", "memory_search"}, config.ConnectorRateLimitConfig{RequestsPerMinute: 60, RequestsPerDay: 1000, Burst: 5})
	return cfg
}

func applyMCPDefaults(cfg config.ConnectorConfig) config.ConnectorConfig {
	if cfg.Kind == "" {
		cfg.Kind = MCPServer
	}
	if cfg.Bind == "" {
		cfg.Bind = "stdio"
	}
	cfg.RequireToken = false
	cfg.TokenEnv = ""
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 65536
	}
	cfg = applySecurityDefaults(cfg, config.SecretRefConfig{Provider: secrets.ProviderNone}, nil, []string{"chat", "ask", "memory_search"}, config.ConnectorRateLimitConfig{RequestsPerMinute: 60, RequestsPerDay: 1000, Burst: 5})
	return cfg
}

func applyAdapterDefaults(cfg config.ConnectorConfig, name string) config.ConnectorConfig {
	name = strings.TrimSpace(name)
	if cfg.Kind == "" {
		cfg.Kind = name
	}
	if cfg.Bind == "" {
		cfg.Bind = "webhook"
	}
	cfg.RequireToken = true
	if cfg.TokenEnv == "" {
		cfg.TokenEnv = defaultAdapterTokenEnv(name)
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 65536
	}
	cfg = applySecurityDefaults(cfg, config.SecretRefConfig{Provider: secrets.ProviderEnv, Name: defaultAdapterTokenEnv(name)}, defaultAdapterDomains(name), []string{"chat", "ask"}, config.ConnectorRateLimitConfig{RequestsPerMinute: 30, RequestsPerDay: 500, Burst: 3})
	return cfg
}

func applySecurityDefaults(cfg config.ConnectorConfig, secret config.SecretRefConfig, domains []string, scopes []string, rate config.ConnectorRateLimitConfig) config.ConnectorConfig {
	if cfg.SecretRef.Provider == "" {
		cfg.SecretRef = secret
	}
	if cfg.SecretRef.Provider == secrets.ProviderEnv && cfg.TokenEnv != "" && (cfg.SecretRef.Name == "" || cfg.SecretRef.Name == secret.Name) {
		cfg.SecretRef.Name = cfg.TokenEnv
	}
	if cfg.SecretRef.Provider == secrets.ProviderEnv && cfg.SecretRef.Name == "" {
		if strings.TrimSpace(cfg.TokenEnv) != "" {
			cfg.SecretRef.Name = strings.TrimSpace(cfg.TokenEnv)
		} else {
			cfg.SecretRef.Name = secret.Name
		}
	}
	if cfg.TokenEnv == "" && cfg.SecretRef.Provider == secrets.ProviderEnv {
		cfg.TokenEnv = cfg.SecretRef.Name
	}
	if len(cfg.AllowedDomains) == 0 {
		cfg.AllowedDomains = append([]string{}, domains...)
	}
	if connectorPermissionsUnset(cfg.Permissions) {
		cfg.Permissions = config.ConnectorPermissionsConfig{Inbound: true, Scopes: append([]string{}, scopes...)}
	}
	if cfg.RateLimit.RequestsPerMinute == 0 {
		cfg.RateLimit.RequestsPerMinute = rate.RequestsPerMinute
	}
	if cfg.RateLimit.RequestsPerDay == 0 {
		cfg.RateLimit.RequestsPerDay = rate.RequestsPerDay
	}
	if cfg.RateLimit.Burst == 0 {
		cfg.RateLimit.Burst = rate.Burst
	}
	return cfg
}

func connectorPermissionsUnset(cfg config.ConnectorPermissionsConfig) bool {
	return !cfg.Inbound &&
		!cfg.Outbound &&
		!cfg.Posting &&
		!cfg.Trading &&
		!cfg.Deployment &&
		!cfg.Mutating &&
		len(cfg.Scopes) == 0
}

func setAdapterConfig(cfg *config.ConnectorsConfig, name string, adapter config.ConnectorConfig) {
	switch strings.TrimSpace(name) {
	case Slack:
		cfg.Slack = adapter
	case Discord:
		cfg.Discord = adapter
	case Telegram:
		cfg.Telegram = adapter
	case Email:
		cfg.Email = adapter
	}
}

func defaultAdapterTokenEnv(name string) string {
	switch strings.TrimSpace(name) {
	case Slack:
		return "YEMAKA_SLACK_CONNECTOR_TOKEN"
	case Discord:
		return "YEMAKA_DISCORD_CONNECTOR_TOKEN"
	case Telegram:
		return "YEMAKA_TELEGRAM_CONNECTOR_TOKEN"
	case Email:
		return "YEMAKA_EMAIL_CONNECTOR_TOKEN"
	default:
		return "YEMAKA_CONNECTOR_TOKEN"
	}
}

func defaultAdapterDomains(name string) []string {
	switch strings.TrimSpace(name) {
	case Slack:
		return []string{"slack.com", "hooks.slack.com", "api.slack.com"}
	case Discord:
		return []string{"discord.com", "discordapp.com"}
	case Telegram:
		return []string{"api.telegram.org"}
	default:
		return nil
	}
}

func secretReference(cfg config.ConnectorConfig) secrets.Reference {
	ref := secrets.Reference{
		Provider: cfg.SecretRef.Provider,
		Name:     cfg.SecretRef.Name,
		Service:  cfg.SecretRef.Service,
		Account:  cfg.SecretRef.Account,
	}
	if ref.Provider == "" && cfg.RequireToken && strings.TrimSpace(cfg.TokenEnv) != "" {
		ref = secrets.Env(cfg.TokenEnv)
	}
	if ref.Provider == "" && !cfg.RequireToken {
		ref = secrets.None()
	}
	return secrets.Normalize(ref)
}

func safeSecretReference(cfg config.ConnectorConfig) (secrets.Reference, bool) {
	ref := secretReference(cfg)
	if secretReferenceLooksLikeValue(ref) {
		return secrets.Reference{Provider: ref.Provider}, false
	}
	return ref, true
}

func secretReferenceLooksLikeValue(ref secrets.Reference) bool {
	return secrets.LooksLikeSecretValue(ref.Name) ||
		secrets.LooksLikeSecretValue(ref.Service) ||
		secrets.LooksLikeSecretValue(ref.Account)
}

func safeTokenEnv(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || secrets.LooksLikeSecretValue(value) {
		return ""
	}
	return value
}

func secretStatus(cfg config.ConnectorConfig) secrets.Status {
	return secretStatusFromReference(secretReference(cfg), cfg.RequireToken)
}

func secretStatusFromReference(ref secrets.Reference, required bool) secrets.Status {
	ref = secrets.Normalize(ref)
	if secretReferenceLooksLikeValue(ref) {
		return secrets.Status{
			Provider: ref.Provider,
			Ready:    false,
			Detail:   "secret reference looks like a secret value; configure a reference name instead",
		}
	}
	return secrets.Check(context.Background(), ref, required)
}

func healthFor(enabled bool, ready bool, detail string) Health {
	if !enabled {
		return Health{Status: "disabled", Detail: "connector disabled"}
	}
	if !ready {
		return Health{Status: "warning", Detail: detail}
	}
	return Health{Status: "healthy", Detail: "connector ready"}
}

func anyEnabled(cfg config.ConnectorsConfig) bool {
	if cfg.LocalAPI.Enabled || cfg.MCPServer.Enabled {
		return true
	}
	for _, name := range AdapterNames() {
		adapter, _ := AdapterConfig(cfg, name)
		if adapter.Enabled {
			return true
		}
	}
	return false
}

func LocalAPITools() []ToolDescriptor {
	return []ToolDescriptor{
		{
			Name:        "chat",
			Description: "Send a plain chat message through the same local Yemaka agent service.",
			InputSchema: objectSchema(map[string]any{
				"content": stringSchema("User message to send to Yemaka."),
			}, []string{"content"}),
		},
		{
			Name:        "ask",
			Description: "Ask using Yemaka workspace, memory, RAG, skills, and safe-tool context.",
			InputSchema: objectSchema(map[string]any{
				"content": stringSchema("Task or question for Yemaka."),
				"skill":   stringSchema("Optional local skill name."),
			}, []string{"content"}),
		},
		{
			Name:        "memory_search",
			Description: "Search explicit local Yemaka memories without contacting remote services.",
			InputSchema: objectSchema(map[string]any{
				"query": stringSchema("Memory search query."),
				"limit": map[string]any{"type": "integer", "minimum": 1, "maximum": 20},
			}, []string{"query"}),
		},
	}
}

func MCPTools() []ToolDescriptor {
	tools := LocalAPITools()
	return []ToolDescriptor{
		{
			Name:        "yemaka_chat",
			Description: tools[0].Description,
			InputSchema: tools[0].InputSchema,
		},
		{
			Name:        "yemaka_ask",
			Description: tools[1].Description,
			InputSchema: tools[1].InputSchema,
		},
		{
			Name:        "yemaka_memory_search",
			Description: tools[2].Description,
			InputSchema: tools[2].InputSchema,
		},
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
		"required":             required,
	}
}

func stringSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
	}
}
