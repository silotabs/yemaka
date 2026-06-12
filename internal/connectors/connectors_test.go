package connectors

import (
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/secrets"
)

func TestLocalAPIStatusDisabledByDefault(t *testing.T) {
	cfg := config.Default().Connectors
	statuses := List(cfg)
	if len(statuses) != 6 {
		t.Fatalf("connector count = %d, want 6", len(statuses))
	}
	status := LocalAPIStatus(cfg)
	if status.Enabled {
		t.Fatal("local API connector should be disabled by default")
	}
	if !status.RequireToken {
		t.Fatal("local API connector should require token by default")
	}
	if status.Endpoint != "/connectors/local/v1" {
		t.Fatalf("endpoint = %q", status.Endpoint)
	}
	if status.Secret.Provider != "env" || status.Secret.Name != "YEMAKA_CONNECTOR_TOKEN" {
		t.Fatalf("secret = %+v, want env token reference", status.Secret)
	}
	if status.Permissions.Posting || status.Permissions.Trading || status.Permissions.Deployment || status.Permissions.Mutating {
		t.Fatalf("permissions = %+v, want non-mutating defaults", status.Permissions)
	}
	if status.RateLimit.RequestsPerMinute == 0 {
		t.Fatalf("rate limit = %+v, want bounded rate limit", status.RateLimit)
	}
}

func TestAdapterStatusDisabledByDefault(t *testing.T) {
	cfg := config.Default().Connectors
	for _, name := range AdapterNames() {
		status := AdapterStatus(cfg, name)
		if status.Enabled {
			t.Fatalf("%s connector should be disabled by default", name)
		}
		if status.Bind != "webhook" {
			t.Fatalf("%s Bind = %q, want webhook", name, status.Bind)
		}
		if !status.RequireToken {
			t.Fatalf("%s connector should require a token", name)
		}
		if status.Endpoint != "/connectors/adapters/v1/"+name {
			t.Fatalf("%s endpoint = %q", name, status.Endpoint)
		}
	}
}

func TestConnectorHealthUsesStandardStatuses(t *testing.T) {
	if got := healthFor(false, false, "").Status; got != "disabled" {
		t.Fatalf("disabled health status = %q", got)
	}
	if got := healthFor(true, false, "missing token").Status; got != "warning" {
		t.Fatalf("missing token health status = %q", got)
	}
	if got := healthFor(true, true, "").Status; got != "healthy" {
		t.Fatalf("ready health status = %q", got)
	}
}

func TestMCPServerStatusDisabledByDefault(t *testing.T) {
	cfg := config.Default().Connectors
	status := MCPServerStatus(cfg)
	if status.Enabled {
		t.Fatal("MCP server connector should be disabled by default")
	}
	if status.Bind != "stdio" {
		t.Fatalf("Bind = %q, want stdio", status.Bind)
	}
	if status.RequireToken {
		t.Fatal("MCP stdio connector should not require a token")
	}
	if status.Endpoint != "stdio" {
		t.Fatalf("Endpoint = %q, want stdio", status.Endpoint)
	}
}

func TestEnableLocalAPIUsesTokenEnv(t *testing.T) {
	cfg := config.Default()
	if err := EnableLocalAPI(cfg, "YEMAKA_TEST_CONNECTOR_TOKEN"); err != nil {
		t.Fatalf("EnableLocalAPI() error = %v", err)
	}
	if !cfg.Connectors.Enabled || !cfg.Connectors.LocalAPI.Enabled {
		t.Fatal("local API connector was not enabled")
	}
	if cfg.Connectors.LocalAPI.TokenEnv != "YEMAKA_TEST_CONNECTOR_TOKEN" {
		t.Fatalf("TokenEnv = %q", cfg.Connectors.LocalAPI.TokenEnv)
	}
}

func TestEnableDisableMCPServerKeepsLocalAPIIndependent(t *testing.T) {
	cfg := config.Default()
	if err := EnableLocalAPI(cfg, "YEMAKA_TEST_CONNECTOR_TOKEN"); err != nil {
		t.Fatalf("EnableLocalAPI() error = %v", err)
	}
	if err := EnableMCPServer(cfg); err != nil {
		t.Fatalf("EnableMCPServer() error = %v", err)
	}
	if !cfg.Connectors.Enabled || !cfg.Connectors.LocalAPI.Enabled || !cfg.Connectors.MCPServer.Enabled {
		t.Fatal("expected both connectors to be enabled")
	}
	if err := DisableLocalAPI(cfg); err != nil {
		t.Fatalf("DisableLocalAPI() error = %v", err)
	}
	if !cfg.Connectors.Enabled || cfg.Connectors.LocalAPI.Enabled || !cfg.Connectors.MCPServer.Enabled {
		t.Fatal("disabling local_api should leave mcp_server enabled")
	}
	if err := DisableMCPServer(cfg); err != nil {
		t.Fatalf("DisableMCPServer() error = %v", err)
	}
	if cfg.Connectors.Enabled || cfg.Connectors.MCPServer.Enabled {
		t.Fatal("all connectors should be disabled")
	}
}

func TestEnableDisableAdapterKeepsConnectorsIndependent(t *testing.T) {
	cfg := config.Default()
	if err := EnableAdapter(cfg, Slack, "YEMAKA_TEST_SLACK_TOKEN"); err != nil {
		t.Fatalf("EnableAdapter() error = %v", err)
	}
	if !cfg.Connectors.Enabled || !cfg.Connectors.Slack.Enabled {
		t.Fatal("slack adapter was not enabled")
	}
	if cfg.Connectors.Slack.TokenEnv != "YEMAKA_TEST_SLACK_TOKEN" {
		t.Fatalf("TokenEnv = %q", cfg.Connectors.Slack.TokenEnv)
	}
	if err := EnableMCPServer(cfg); err != nil {
		t.Fatalf("EnableMCPServer() error = %v", err)
	}
	if err := DisableAdapter(cfg, Slack); err != nil {
		t.Fatalf("DisableAdapter() error = %v", err)
	}
	if !cfg.Connectors.Enabled || cfg.Connectors.Slack.Enabled || !cfg.Connectors.MCPServer.Enabled {
		t.Fatal("disabling slack should leave mcp_server enabled")
	}
}

func TestTokenReadsEnvironment(t *testing.T) {
	t.Setenv("YEMAKA_TEST_CONNECTOR_TOKEN", "secret")
	cfg := config.Default().Connectors.LocalAPI
	cfg.TokenEnv = "YEMAKA_TEST_CONNECTOR_TOKEN"
	token, ok := Token(cfg)
	if !ok || token != "secret" {
		t.Fatalf("Token() = %q, %t; want secret, true", token, ok)
	}
}

func TestRegistryProvidesValidatedBuiltInManifests(t *testing.T) {
	cfg := config.Default().Connectors
	entries := Registry(cfg)
	if len(entries) != 6 {
		t.Fatalf("registry count = %d, want 6", len(entries))
	}
	for _, entry := range entries {
		if err := ValidateManifest(entry.Manifest); err != nil {
			t.Fatalf("ValidateManifest(%s) error = %v", entry.Manifest.Name, err)
		}
		if entry.Manifest.Permissions.Posting || entry.Manifest.Permissions.Trading || entry.Manifest.Permissions.Deployment || entry.Manifest.Permissions.Mutating {
			t.Fatalf("%s has mutating permissions: %+v", entry.Manifest.Name, entry.Manifest.Permissions)
		}
	}
}

func TestRegistryWithGeneratedManifestsKeepsGeneratedDisabled(t *testing.T) {
	cfg := config.Default().Connectors
	manifest := validGeneratedConnectorManifest()
	manifest.AllowedDomains = []string{"API.GITHUB.COM", "api.github.com"}
	manifest.Permissions.Scopes = []string{"chat", "ask", "chat"}

	entries, err := RegistryWithGenerated(cfg, []Manifest{manifest})
	if err != nil {
		t.Fatalf("RegistryWithGenerated() error = %v", err)
	}
	if len(entries) != 7 {
		t.Fatalf("registry count = %d, want 7", len(entries))
	}

	var generated *RegistryEntry
	for i := range entries {
		if entries[i].Status.Name == manifest.Name {
			generated = &entries[i]
			break
		}
	}
	if generated == nil {
		t.Fatal("generated connector entry not found")
	}
	if generated.Status.Enabled {
		t.Fatal("generated connector should remain disabled until explicitly enabled")
	}
	if generated.Status.Bind != "generated" {
		t.Fatalf("generated Bind = %q, want generated", generated.Status.Bind)
	}
	if !generated.Status.RequireToken || generated.Status.TokenEnv != "YEMAKA_GITHUB_CONNECTOR_TOKEN" {
		t.Fatalf("generated auth status = require %t tokenEnv %q", generated.Status.RequireToken, generated.Status.TokenEnv)
	}
	if got := strings.Join(generated.Manifest.AllowedDomains, ","); got != "api.github.com" {
		t.Fatalf("allowed domains = %q, want normalized duplicate-free domain", got)
	}
	if got := strings.Join(generated.Manifest.Permissions.Scopes, ","); got != "chat,ask" {
		t.Fatalf("scopes = %q, want normalized duplicate-free scopes", got)
	}
	if err := ValidateManifest(generated.Manifest); err != nil {
		t.Fatalf("ValidateManifest(generated) error = %v", err)
	}
}

func TestRegistryWithGeneratedRejectsBuiltInNameConflict(t *testing.T) {
	manifest := validGeneratedConnectorManifest()
	manifest.Name = LocalAPI
	manifest.Endpoint = "extension:local_api"
	_, err := RegistryWithGenerated(config.Default().Connectors, []Manifest{manifest})
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("RegistryWithGenerated() error = %v, want conflict", err)
	}
}

func TestConnectorStatusRedactsSecretLikeReferences(t *testing.T) {
	cfg := config.Default().Connectors
	cfg.LocalAPI.TokenEnv = "sk-live-secret"
	cfg.LocalAPI.SecretRef = config.SecretRefConfig{Provider: secrets.ProviderEnv, Name: "sk-live-secret"}

	status := LocalAPIStatus(cfg)
	if status.TokenEnv != "" {
		t.Fatalf("TokenEnv = %q, want redacted empty token env", status.TokenEnv)
	}
	if status.Secret.Name != "" {
		t.Fatalf("Secret.Name = %q, want redacted empty secret reference", status.Secret.Name)
	}
	if strings.Contains(status.Secret.Detail, "sk-live-secret") {
		t.Fatalf("secret detail leaked token value: %q", status.Secret.Detail)
	}
	manifest := ManifestFromStatus(status)
	if manifest.Auth.Name != "" {
		t.Fatalf("manifest auth name = %q, want redacted empty auth name", manifest.Auth.Name)
	}
}

func TestValidateGeneratedConnectorManifestHardening(t *testing.T) {
	manifest := validGeneratedConnectorManifest()
	manifest.RateLimit = config.ConnectorRateLimitConfig{}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "positive rate limits") {
		t.Fatalf("ValidateManifest(missing rate limit) error = %v, want rate limit error", err)
	}

	manifest = validGeneratedConnectorManifest()
	manifest.Name = "../bad"
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "name must match") {
		t.Fatalf("ValidateManifest(bad name) error = %v, want name pattern error", err)
	}

	manifest = validGeneratedConnectorManifest()
	manifest.Auth = secrets.Reference{Provider: secrets.ProviderEnv, Name: "sk-live-secret"}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "secret value") {
		t.Fatalf("ValidateManifest(secret auth) error = %v, want secret value error", err)
	}

	manifest = validGeneratedConnectorManifest()
	manifest.Endpoint = "https://api.github.com"
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "local core route") {
		t.Fatalf("ValidateManifest(remote endpoint) error = %v, want local endpoint error", err)
	}

	manifest = validGeneratedConnectorManifest()
	manifest.AllowedDomains = []string{"https://api.github.com"}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "domain only") {
		t.Fatalf("ValidateManifest(remote domain) error = %v, want domain-only error", err)
	}

	manifest = validGeneratedConnectorManifest()
	manifest.AllowedDomains = nil
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "allowed domains") {
		t.Fatalf("ValidateManifest(no domains) error = %v, want allowed domains error", err)
	}
}

func TestMutatingConnectorManifestRequiresPolicyApproval(t *testing.T) {
	manifest := Manifest{
		Name:        "poster",
		Version:     "0.1.0",
		Kind:        "social",
		Description: "post updates",
		AllowedDomains: []string{
			"social.example",
		},
		Permissions: config.ConnectorPermissionsConfig{
			Inbound: true,
			Posting: true,
		},
		RateLimit: config.ConnectorRateLimitConfig{
			RequestsPerMinute: 5,
			RequestsPerDay:    50,
			Burst:             1,
		},
		Endpoint: "extension:poster",
	}
	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("ValidateManifest() error = nil, want approval requirement")
	}
	manifest.RequiresApproval = true
	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("ValidateManifest(approved posting) error = %v", err)
	}
	manifest.Permissions.Trading = true
	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("ValidateManifest(trading) error = nil, want live trading block")
	}
	if err := ValidateManifestWithPolicyMode(manifest, "full_access"); err != nil {
		t.Fatalf("ValidateManifestWithPolicyMode(full_access trading) error = %v", err)
	}
}

func validGeneratedConnectorManifest() Manifest {
	return Manifest{
		Name:        "github_bridge",
		Version:     "0.1.0",
		Kind:        "github",
		Description: "Generated GitHub connector manifest for tests.",
		Auth:        secrets.Reference{Provider: secrets.ProviderEnv, Name: "YEMAKA_GITHUB_CONNECTOR_TOKEN"},
		AllowedDomains: []string{
			"api.github.com",
		},
		Permissions: config.ConnectorPermissionsConfig{
			Inbound:  true,
			Outbound: true,
			Scopes:   []string{"chat", "ask"},
		},
		RateLimit: config.ConnectorRateLimitConfig{
			RequestsPerMinute: 10,
			RequestsPerDay:    100,
			Burst:             2,
		},
		Endpoint:         "extension:github_bridge",
		RequiresApproval: true,
	}
}

func TestParseAdapterInputSupportsSlackFormAndEmailJSON(t *testing.T) {
	slack, err := ParseAdapterInput(Slack, "application/x-www-form-urlencoded", []byte("text=hello+from+slack&user_name=silo"))
	if err != nil {
		t.Fatalf("ParseAdapterInput(slack) error = %v", err)
	}
	if slack.Content != "hello from slack" || slack.User != "silo" {
		t.Fatalf("slack input = %+v", slack)
	}

	email, err := ParseAdapterInput(Email, "application/json", []byte(`{"subject":"Project","body":"Summarize this","from":"me@example.test"}`))
	if err != nil {
		t.Fatalf("ParseAdapterInput(email) error = %v", err)
	}
	if email.Content != "Subject: Project\n\nSummarize this" {
		t.Fatalf("email content = %q", email.Content)
	}
}
