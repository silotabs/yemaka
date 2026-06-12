package connectors

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/safety"
	"yemaka/internal/secrets"
)

var connectorNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

type Manifest struct {
	Name             string                            `json:"name" yaml:"name"`
	Version          string                            `json:"version" yaml:"version"`
	Kind             string                            `json:"kind" yaml:"kind"`
	Description      string                            `json:"description" yaml:"description"`
	Auth             secrets.Reference                 `json:"auth" yaml:"auth"`
	AllowedDomains   []string                          `json:"allowedDomains" yaml:"allowed_domains"`
	Permissions      config.ConnectorPermissionsConfig `json:"permissions" yaml:"permissions"`
	RateLimit        config.ConnectorRateLimitConfig   `json:"rateLimit" yaml:"rate_limit"`
	Endpoint         string                            `json:"endpoint" yaml:"endpoint"`
	RequiresApproval bool                              `json:"requiresApproval" yaml:"requires_approval"`
}

type RegistryEntry struct {
	Status   Status   `json:"status"`
	Manifest Manifest `json:"manifest"`
}

func Registry(cfg config.ConnectorsConfig) []RegistryEntry {
	return registryFromStatuses(List(cfg))
}

func RegistryWithGenerated(cfg config.ConnectorsConfig, generated []Manifest) ([]RegistryEntry, error) {
	entries := registryFromStatuses(List(cfg))
	seen := make(map[string]struct{}, len(entries)+len(generated))
	for _, entry := range entries {
		seen[entry.Manifest.Name] = struct{}{}
	}
	for _, manifest := range generated {
		status, err := StatusFromManifest(manifest)
		if err != nil {
			return nil, fmt.Errorf("generated connector manifest %q: %w", strings.TrimSpace(manifest.Name), err)
		}
		if _, exists := seen[status.Name]; exists {
			return nil, fmt.Errorf("generated connector %q conflicts with an existing connector", status.Name)
		}
		normalized := normalizeValidatedManifest(manifest)
		entries = append(entries, RegistryEntry{Status: status, Manifest: normalized})
		seen[status.Name] = struct{}{}
	}
	sortRegistryEntries(entries)
	return entries, nil
}

func RegistryWithGeneratedEntries(cfg config.ConnectorsConfig, generated []GeneratedEntry) ([]RegistryEntry, error) {
	return RegistryWithGeneratedEntriesWithPolicy(cfg, generated, safety.PolicyModeSafe)
}

func RegistryWithGeneratedEntriesWithPolicy(cfg config.ConnectorsConfig, generated []GeneratedEntry, policyMode string) ([]RegistryEntry, error) {
	entries := registryFromStatuses(List(cfg))
	seen := make(map[string]struct{}, len(entries)+len(generated))
	for _, entry := range entries {
		seen[entry.Manifest.Name] = struct{}{}
	}
	for _, generatedEntry := range generated {
		if !generatedEntry.Installed {
			continue
		}
		status, err := StatusFromGeneratedEntryWithPolicy(generatedEntry, policyMode)
		if err != nil {
			return nil, fmt.Errorf("generated connector manifest %q: %w", strings.TrimSpace(generatedEntry.Name), err)
		}
		if _, exists := seen[status.Name]; exists {
			return nil, fmt.Errorf("generated connector %q conflicts with an existing connector", status.Name)
		}
		normalized := normalizeValidatedManifest(generatedEntry.Manifest)
		entries = append(entries, RegistryEntry{Status: status, Manifest: normalized})
		seen[status.Name] = struct{}{}
	}
	sortRegistryEntries(entries)
	return entries, nil
}

func registryFromStatuses(statuses []Status) []RegistryEntry {
	entries := make([]RegistryEntry, 0, len(statuses))
	for _, status := range statuses {
		manifest := ManifestFromStatus(status)
		entries = append(entries, RegistryEntry{Status: status, Manifest: manifest})
	}
	sortRegistryEntries(entries)
	return entries
}

func ManifestFromStatus(status Status) Manifest {
	description := "Optional Yemaka connector"
	switch status.Name {
	case LocalAPI:
		description = "Loopback local REST connector for the same Yemaka Go core."
	case MCPServer:
		description = "Local stdio MCP connector for clients that launch Yemaka as a subprocess."
	case Slack:
		description = "Inbound Slack webhook adapter for local Yemaka chat and ask."
	case Discord:
		description = "Inbound Discord webhook adapter for local Yemaka chat and ask."
	case Telegram:
		description = "Inbound Telegram webhook adapter for local Yemaka chat and ask."
	case Email:
		description = "Inbound email webhook adapter for local Yemaka chat and ask."
	}
	return Manifest{
		Name:           status.Name,
		Version:        "0.1.0",
		Kind:           status.Kind,
		Description:    description,
		Auth:           secrets.Reference{Provider: status.Secret.Provider, Name: status.Secret.Name, Service: status.Secret.Service, Account: status.Secret.Account},
		AllowedDomains: append([]string{}, status.AllowedDomains...),
		Permissions:    status.Permissions,
		RateLimit:      status.RateLimit,
		Endpoint:       status.Endpoint,
	}
}

func StatusFromManifest(manifest Manifest) (Status, error) {
	if err := ValidateManifest(manifest); err != nil {
		return Status{}, err
	}
	manifest = normalizeValidatedManifest(manifest)
	return statusFromGeneratedManifest(manifest, false, "")
}

func StatusFromGeneratedEntry(entry GeneratedEntry) (Status, error) {
	return StatusFromGeneratedEntryWithPolicy(entry, safety.PolicyModeSafe)
}

func StatusFromGeneratedEntryWithPolicy(entry GeneratedEntry, policyMode string) (Status, error) {
	manifest := normalizeValidatedManifest(entry.Manifest)
	if err := validateGeneratedConnectorManifest(manifest, policyMode); err != nil {
		return Status{}, err
	}
	return statusFromGeneratedManifest(manifest, entry.Enabled, strings.TrimSpace(entry.ValidationError))
}

func statusFromGeneratedManifest(manifest Manifest, enabled bool, validationError string) (Status, error) {
	requireToken := manifest.Auth.Provider != "" && manifest.Auth.Provider != secrets.ProviderNone
	secret := secretStatusFromReference(manifest.Auth, requireToken)
	tokenEnv := ""
	if manifest.Auth.Provider == secrets.ProviderEnv {
		tokenEnv = safeTokenEnv(manifest.Auth.Name)
	}
	health := Health{Status: "disabled", Detail: "generated connector disabled until explicitly enabled"}
	if validationError != "" {
		health = Health{Status: "broken", Detail: validationError}
	} else if enabled && requireToken && !secret.Ready {
		health = Health{Status: "warning", Detail: secret.Detail}
	} else if enabled {
		health = Health{Status: "warning", Detail: "runtime execution enabled; token, policy, rate, and extension gates remain enforced"}
	}
	return Status{
		Name:           manifest.Name,
		Kind:           manifest.Kind,
		Enabled:        enabled,
		Bind:           "generated",
		RequireToken:   requireToken,
		TokenEnv:       tokenEnv,
		TokenReady:     !requireToken || secret.Ready,
		Secret:         secret,
		AllowedDomains: append([]string{}, manifest.AllowedDomains...),
		Permissions:    manifest.Permissions,
		RateLimit:      manifest.RateLimit,
		MaxBodyBytes:   65536,
		Endpoint:       manifest.Endpoint,
		Health:         health,
	}, nil
}

func ValidateManifest(manifest Manifest) error {
	return ValidateManifestWithPolicyMode(manifest, safety.PolicyModeSafe)
}

func ValidateManifestWithPolicyMode(manifest Manifest, policyMode string) error {
	manifest = trimConnectorManifest(manifest)
	if !connectorNamePattern.MatchString(manifest.Name) {
		return fmt.Errorf("connector manifest name must match %s", connectorNamePattern.String())
	}
	if manifest.Version == "" {
		return fmt.Errorf("connector manifest version is required")
	}
	if !connectorNamePattern.MatchString(manifest.Kind) {
		return fmt.Errorf("connector manifest kind must match %s", connectorNamePattern.String())
	}
	if manifest.Description == "" {
		return fmt.Errorf("connector manifest description is required")
	}
	if manifest.Endpoint == "" {
		return fmt.Errorf("connector manifest endpoint is required")
	}
	if strings.Contains(manifest.Endpoint, "://") {
		return fmt.Errorf("connector manifest endpoint must be a local core route, stdio, or generated extension endpoint")
	}
	if err := secrets.Validate(manifest.Auth, manifest.Auth.Provider != secrets.ProviderNone && manifest.Auth.Provider != ""); err != nil {
		return err
	}
	if err := validateConnectorPolicy(manifest, policyMode); err != nil {
		return err
	}
	fullAccess := safety.IsFullAccessMode(policyMode)
	if connectorNeedsAllowedDomains(manifest.Permissions) && len(manifest.AllowedDomains) == 0 && !fullAccess {
		return fmt.Errorf("network-capable connectors must declare allowed domains")
	}
	if err := validateConnectorRateLimit(manifest.RateLimit); err != nil {
		return err
	}
	if !fullAccess {
		if err := validateConnectorDomains(manifest.AllowedDomains); err != nil {
			return err
		}
	}
	return nil
}

func validateConnectorPolicy(manifest Manifest, policyMode string) error {
	perms := manifest.Permissions
	if safety.IsFullAccessMode(policyMode) {
		return nil
	}
	if !perms.Posting && !perms.Trading && !perms.Deployment && !perms.Mutating && !perms.Outbound {
		return nil
	}
	level := safety.LevelAutonomous
	if manifest.RequiresApproval {
		level = safety.LevelConfirm
	}
	decision := safety.EvaluatePolicy(safety.PolicyRequest{
		Domain:       safety.DomainConnector,
		Action:       safety.ActionRun,
		Level:        level,
		PolicyMode:   policyMode,
		Actor:        "connector_manifest",
		Resource:     manifest.Name,
		Enabled:      true,
		TaskApproved: manifest.RequiresApproval,
		Network:      perms.Outbound || perms.Posting || perms.Deployment || perms.Trading,
		Mutating:     perms.Mutating || perms.Posting || perms.Deployment || perms.Trading,
		Posting:      perms.Posting,
		Trading:      perms.Trading,
		LiveTrading:  perms.Trading,
		Deployment:   perms.Deployment,
		Domains:      manifest.AllowedDomains,
	})
	if !decision.Allowed {
		return fmt.Errorf("connector policy blocked %s: %s", manifest.Name, decision.Reason)
	}
	if decision.RequiresConfirmation && !manifest.RequiresApproval {
		return fmt.Errorf("connector policy requires requires_approval for %s: %s", manifest.Name, decision.Reason)
	}
	return nil
}

func sortRegistryEntries(entries []RegistryEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Status.Name < entries[j].Status.Name
	})
}

func trimConnectorManifest(manifest Manifest) Manifest {
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Kind = strings.TrimSpace(manifest.Kind)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Endpoint = strings.TrimSpace(manifest.Endpoint)
	manifest.Auth = secrets.Normalize(manifest.Auth)
	return manifest
}

func normalizeValidatedManifest(manifest Manifest) Manifest {
	manifest = trimConnectorManifest(manifest)
	manifest.AllowedDomains = normalizeConnectorList(manifest.AllowedDomains, true)
	manifest.Permissions.Scopes = normalizeConnectorList(manifest.Permissions.Scopes, false)
	return manifest
}

func normalizeConnectorList(values []string, lower bool) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func connectorNeedsAllowedDomains(perms config.ConnectorPermissionsConfig) bool {
	return perms.Outbound || perms.Posting || perms.Trading || perms.Deployment
}

func validateConnectorRateLimit(rate config.ConnectorRateLimitConfig) error {
	if rate.RequestsPerMinute < 0 || rate.RequestsPerDay < 0 || rate.Burst < 0 {
		return fmt.Errorf("connector rate limits cannot be negative")
	}
	if rate.RequestsPerMinute == 0 || rate.RequestsPerDay == 0 || rate.Burst == 0 {
		return fmt.Errorf("connector manifests must declare positive rate limits")
	}
	return nil
}

func validateConnectorDomains(domains []string) error {
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			return fmt.Errorf("connector allowed domains cannot contain empty values")
		}
		if strings.ContainsAny(domain, "/:") || strings.ContainsAny(domain, " \t\r\n") {
			return fmt.Errorf("connector allowed domain must be a domain only: %s", domain)
		}
	}
	return nil
}
