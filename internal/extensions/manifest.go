package extensions

import (
	"fmt"
	"net/netip"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"yemaka/internal/safety"
)

const (
	TypeTool      = "tool"
	TypeConnector = "connector"
	TypeWorkflow  = "workflow"
	TypeJob       = "job"

	ManifestFile = "extension.yaml"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

type Manifest struct {
	Name         string            `yaml:"name" json:"name"`
	Version      string            `yaml:"version" json:"version"`
	Description  string            `yaml:"description" json:"description"`
	Type         string            `yaml:"type" json:"type"`
	Entrypoint   Entrypoint        `yaml:"entrypoint" json:"entrypoint"`
	InputSchema  map[string]any    `yaml:"input_schema" json:"inputSchema"`
	OutputSchema map[string]any    `yaml:"output_schema" json:"outputSchema"`
	Permissions  Permissions       `yaml:"permissions" json:"permissions"`
	Safety       Safety            `yaml:"safety" json:"safety"`
	Tests        []string          `yaml:"tests" json:"tests"`
	Metadata     map[string]string `yaml:"metadata" json:"metadata,omitempty"`
}

type Entrypoint struct {
	Type    string   `yaml:"type" json:"type"`
	Command string   `yaml:"command" json:"command"`
	Args    []string `yaml:"args" json:"args,omitempty"`
}

type Permissions struct {
	Network    NetworkPermission     `yaml:"network" json:"network"`
	Filesystem FilesystemPermission  `yaml:"filesystem" json:"filesystem"`
	Shell      bool                  `yaml:"shell" json:"shell"`
	Secrets    bool                  `yaml:"secrets" json:"secrets"`
	Memory     bool                  `yaml:"memory" json:"memory"`
	Extra      map[string]Permission `yaml:"extra" json:"extra,omitempty"`
}

type Permission struct {
	Enabled bool     `yaml:"enabled" json:"enabled"`
	Scopes  []string `yaml:"scopes" json:"scopes,omitempty"`
}

type NetworkPermission struct {
	Enabled        bool     `yaml:"enabled" json:"enabled"`
	AllowedMethods []string `yaml:"allowed_methods" json:"allowedMethods"`
	AllowedDomains []string `yaml:"allowed_domains" json:"allowedDomains"`
}

type FilesystemPermission struct {
	Read  bool     `yaml:"read" json:"read"`
	Write bool     `yaml:"write" json:"write"`
	Paths []string `yaml:"paths" json:"paths,omitempty"`
}

type Safety struct {
	RequiresUserApproval bool  `yaml:"requires_user_approval" json:"requiresUserApproval"`
	MaxRuntimeSeconds    int   `yaml:"max_runtime_seconds" json:"maxRuntimeSeconds"`
	MaxResponseBytes     int64 `yaml:"max_response_bytes" json:"maxResponseBytes"`
}

func ValidateManifest(manifest Manifest) error {
	return ValidateManifestWithPolicyMode(manifest, safety.PolicyModeSafe)
}

func ValidateManifestWithPolicyMode(manifest Manifest, policyMode string) error {
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Type = strings.TrimSpace(manifest.Type)
	if !namePattern.MatchString(manifest.Name) {
		return fmt.Errorf("extension name must match %s", namePattern.String())
	}
	if manifest.Version == "" {
		return fmt.Errorf("extension version is required")
	}
	if !validType(manifest.Type) {
		return fmt.Errorf("extension type must be one of: %s", strings.Join(Types(), ", "))
	}
	if strings.TrimSpace(manifest.Description) == "" {
		return fmt.Errorf("extension description is required")
	}
	if err := validateNoSecretValues(manifest); err != nil {
		return err
	}
	if err := validateEntrypoint(manifest.Entrypoint); err != nil {
		return err
	}
	if err := validateSchema("input_schema", manifest.InputSchema); err != nil {
		return err
	}
	if err := validateSchema("output_schema", manifest.OutputSchema); err != nil {
		return err
	}
	if err := validatePermissions(manifest, policyMode); err != nil {
		return err
	}
	if manifest.Safety.MaxRuntimeSeconds < 0 {
		return fmt.Errorf("safety.max_runtime_seconds cannot be negative")
	}
	if manifest.Safety.MaxResponseBytes < 0 {
		return fmt.Errorf("safety.max_response_bytes cannot be negative")
	}
	return nil
}

func validateNoSecretValues(manifest Manifest) error {
	values := map[string]any{
		"name":          manifest.Name,
		"version":       manifest.Version,
		"description":   manifest.Description,
		"entrypoint":    map[string]any{"type": manifest.Entrypoint.Type, "command": manifest.Entrypoint.Command, "args": stringSliceToAny(manifest.Entrypoint.Args)},
		"tests":         stringSliceToAny(manifest.Tests),
		"metadata":      stringMapToAny(manifest.Metadata),
		"network":       map[string]any{"allowed_methods": stringSliceToAny(manifest.Permissions.Network.AllowedMethods), "allowed_domains": stringSliceToAny(manifest.Permissions.Network.AllowedDomains)},
		"filesystem":    map[string]any{"paths": stringSliceToAny(manifest.Permissions.Filesystem.Paths)},
		"extra":         permissionsMapToAny(manifest.Permissions.Extra),
		"input_schema":  manifest.InputSchema,
		"output_schema": manifest.OutputSchema,
	}
	for field, value := range values {
		if extensionValueContainsSecretValue(value) {
			return fmt.Errorf("extension manifest %s contains a secret-like value; store secret references outside generated manifests", field)
		}
	}
	return nil
}

func stringSliceToAny(values []string) []any {
	if len(values) == 0 {
		return nil
	}
	output := make([]any, 0, len(values))
	for _, value := range values {
		output = append(output, value)
	}
	return output
}

func stringMapToAny(values map[string]string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	output := map[string]any{}
	for key, value := range values {
		output[key] = value
	}
	return output
}

func permissionsMapToAny(values map[string]Permission) map[string]any {
	if len(values) == 0 {
		return nil
	}
	output := map[string]any{}
	for key, value := range values {
		output[key] = map[string]any{"enabled": value.Enabled, "scopes": stringSliceToAny(value.Scopes)}
	}
	return output
}

func Types() []string {
	return []string{TypeTool, TypeConnector, TypeWorkflow, TypeJob}
}

func validType(value string) bool {
	for _, candidate := range Types() {
		if value == candidate {
			return true
		}
	}
	return false
}

func validateEntrypoint(entry Entrypoint) error {
	if strings.TrimSpace(entry.Type) == "" {
		return fmt.Errorf("entrypoint.type is required")
	}
	if strings.TrimSpace(entry.Type) != "command" {
		return fmt.Errorf("entrypoint.type must be command for generated extensions")
	}
	command := strings.TrimSpace(entry.Command)
	if command == "" {
		return fmt.Errorf("entrypoint.command is required")
	}
	if strings.Contains(command, "://") {
		return fmt.Errorf("entrypoint.command must be local")
	}
	if strings.ContainsAny(command, "|;&`") {
		return fmt.Errorf("entrypoint.command must be a single local command, not shell syntax")
	}
	for _, arg := range entry.Args {
		arg = strings.TrimSpace(arg)
		if strings.ContainsAny(arg, "|;&`<>") || strings.Contains(arg, "$(") {
			return fmt.Errorf("entrypoint.args must not contain shell syntax")
		}
		if looksLikeEntrypointPathArg(arg) {
			clean := filepath.Clean(arg)
			if filepath.IsAbs(arg) || clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("entrypoint.args must stay inside the generated extension directory")
			}
		}
	}
	return nil
}

func looksLikeEntrypointPathArg(arg string) bool {
	return strings.Contains(arg, "/") ||
		strings.Contains(arg, string(filepath.Separator)) ||
		strings.HasPrefix(arg, ".") ||
		strings.HasSuffix(arg, ".go") ||
		strings.HasSuffix(arg, ".py") ||
		strings.HasSuffix(arg, ".js") ||
		strings.HasSuffix(arg, ".sh")
}

func validateSchema(name string, schema map[string]any) error {
	if len(schema) == 0 {
		return fmt.Errorf("%s is required", name)
	}
	value, ok := schema["type"]
	if !ok {
		return fmt.Errorf("%s.type is required", name)
	}
	if fmt.Sprint(value) != "object" {
		return fmt.Errorf("%s.type must be object", name)
	}
	if properties, ok := schema["properties"]; ok {
		if _, ok := properties.(map[string]any); !ok {
			return fmt.Errorf("%s.properties must be an object", name)
		}
	}
	if required, ok := schema["required"]; ok {
		switch required.(type) {
		case []any, []string:
		default:
			return fmt.Errorf("%s.required must be a list", name)
		}
	}
	return nil
}

func validatePermissions(manifest Manifest, policyMode string) error {
	perms := manifest.Permissions
	if perms.Shell {
		return fmt.Errorf("shell permission is not allowed for generated extensions in this milestone")
	}
	if perms.Secrets {
		return fmt.Errorf("secrets permission is not available until the secrets vault milestone")
	}
	if err := validateFilesystemPermission(perms.Filesystem); err != nil {
		return err
	}
	if perms.Filesystem.Write && !manifest.Safety.RequiresUserApproval {
		return fmt.Errorf("filesystem.write requires safety.requires_user_approval")
	}
	if perms.Filesystem.Write {
		decision := safety.EvaluatePolicy(safety.PolicyRequest{
			Domain:       safety.DomainFilesystem,
			Action:       safety.ActionWrite,
			Level:        manifestPolicyLevel(manifest),
			PolicyMode:   policyMode,
			Actor:        "extension_manifest",
			Resource:     manifest.Name,
			Generated:    true,
			TaskApproved: manifest.Safety.RequiresUserApproval,
			Mutating:     true,
			FileWrite:    true,
		})
		if err := validatePolicyDecision("filesystem.write", manifest, decision); err != nil {
			return err
		}
	}
	if perms.Network.Enabled {
		if !manifest.Safety.RequiresUserApproval {
			return fmt.Errorf("network requires safety.requires_user_approval")
		}
		if len(perms.Network.AllowedDomains) == 0 {
			return fmt.Errorf("network.allowed_domains is required when network is enabled")
		}
		if len(perms.Network.AllowedMethods) == 0 {
			return fmt.Errorf("network.allowed_methods is required when network is enabled")
		}
		for _, method := range perms.Network.AllowedMethods {
			normalized := strings.ToUpper(strings.TrimSpace(method))
			switch normalized {
			case "GET", "HEAD":
			case "POST":
				if manifest.Type != TypeConnector {
					return fmt.Errorf("POST is allowed only for connector extensions")
				}
				if !manifest.Safety.RequiresUserApproval {
					return fmt.Errorf("network POST requires safety.requires_user_approval")
				}
			default:
				return fmt.Errorf("network method %q is not allowed", method)
			}
		}
		for _, domain := range perms.Network.AllowedDomains {
			if err := validateNetworkAllowedDomain(domain); err != nil {
				return err
			}
		}
		decision := safety.EvaluatePolicy(safety.PolicyRequest{
			Domain:       safety.DomainInternet,
			Action:       safety.ActionFetch,
			Level:        manifestPolicyLevel(manifest),
			PolicyMode:   policyMode,
			Actor:        "extension_manifest",
			Resource:     manifest.Name,
			Enabled:      true,
			Generated:    true,
			TaskApproved: manifest.Safety.RequiresUserApproval,
			Network:      true,
			Domains:      perms.Network.AllowedDomains,
		})
		if err := validatePolicyDecision("network", manifest, decision); err != nil {
			return err
		}
	}
	if len(perms.Extra) > 0 {
		keys := make([]string, 0, len(perms.Extra))
		for key := range perms.Extra {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if strings.TrimSpace(key) == "" {
				return fmt.Errorf("extra permission name cannot be empty")
			}
			if err := validateExtraPolicy(key, perms.Extra[key], manifest, policyMode); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateFilesystemPermission(permission FilesystemPermission) error {
	if permission.Write && len(permission.Paths) == 0 {
		return fmt.Errorf("filesystem.write requires explicit generated extension or profile-scoped paths")
	}
	for _, path := range permission.Paths {
		if err := validateFilesystemPermissionPath(path); err != nil {
			return err
		}
	}
	return nil
}

func validateFilesystemPermissionPath(path string) error {
	original := path
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" {
		return fmt.Errorf("filesystem.paths contains an empty path")
	}
	for _, prefix := range []string{"extension://", "profile://"} {
		if strings.HasPrefix(path, prefix) {
			scoped := strings.TrimPrefix(path, prefix)
			if scoped == "" || scoped == "." {
				return nil
			}
			if unsafePackagePath(scoped) || strings.HasPrefix(scoped, "/") || strings.HasPrefix(scoped, "~") {
				return fmt.Errorf("filesystem path escapes generated extension/profile scope: %s", original)
			}
			return nil
		}
	}
	if filepath.IsAbs(filepath.FromSlash(path)) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~") || path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		return fmt.Errorf("filesystem path escapes generated extension/profile scope: %s", original)
	}
	if unsafePackagePath(path) {
		return fmt.Errorf("unsafe filesystem path for generated extension: %s", original)
	}
	return nil
}

func validateNetworkAllowedDomain(domain string) error {
	original := domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("network.allowed_domains contains an empty domain")
	}
	addrValue := strings.Trim(domain, "[]")
	if addr, err := netip.ParseAddr(addrValue); err == nil {
		if isPrivateOrLocalAddress(addr) {
			return fmt.Errorf("network.allowed_domains cannot target private/local network addresses: %s", original)
		}
		return nil
	}
	if strings.ContainsAny(domain, "/:") {
		return fmt.Errorf("network.allowed_domains must contain domains only: %s", original)
	}
	if domain == "localhost" ||
		strings.HasSuffix(domain, ".localhost") ||
		strings.HasSuffix(domain, ".local") ||
		domain == "host.docker.internal" {
		return fmt.Errorf("network.allowed_domains cannot target private/local network names: %s", original)
	}
	return nil
}

func isPrivateOrLocalAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsUnspecified() ||
		addr.IsMulticast()
}

func manifestPolicyLevel(manifest Manifest) safety.PermissionLevel {
	if manifest.Safety.RequiresUserApproval {
		return safety.LevelConfirm
	}
	return safety.LevelAutonomous
}

func validateExtraPolicy(key string, permission Permission, manifest Manifest, policyMode string) error {
	key = strings.ToLower(strings.TrimSpace(key))
	if !permission.Enabled {
		return nil
	}
	if strings.Contains(key, "policy") {
		decision := safety.EvaluatePolicy(safety.PolicyRequest{
			Domain:     safety.DomainExtension,
			Action:     safety.ActionModifyPolicy,
			Level:      manifestPolicyLevel(manifest),
			PolicyMode: policyMode,
			Actor:      "extension_manifest",
			Resource:   manifest.Name,
			Generated:  true,
		})
		if decision.Allowed {
			return nil
		}
		return fmt.Errorf("extra permission %q blocked by policy: %s", key, decision.Reason)
	}
	request := safety.PolicyRequest{
		Domain:       safety.DomainExtension,
		Action:       safety.ActionRun,
		Level:        manifestPolicyLevel(manifest),
		PolicyMode:   policyMode,
		Actor:        "extension_manifest",
		Resource:     manifest.Name,
		Generated:    true,
		TaskApproved: manifest.Safety.RequiresUserApproval,
	}
	switch key {
	case "posting", "social_posting", "social_reply":
		request.Domain = safety.DomainSocial
		request.Action = safety.ActionPost
		request.Posting = true
		request.Mutating = true
		request.Network = true
	case "deployment", "deploy":
		request.Domain = safety.DomainDeployment
		request.Action = safety.ActionDeploy
		request.Deployment = true
		request.Mutating = true
		request.Network = true
	case "trading", "live_trading":
		request.Domain = safety.DomainTrading
		request.Action = safety.ActionTrade
		request.Trading = true
		request.LiveTrading = true
	case "paper_trading", "testnet_trading":
		request.Domain = safety.DomainTrading
		request.Action = safety.ActionTrade
		request.Trading = true
		request.PaperTrading = true
	case "privacy", "person_research":
		request.Domain = safety.DomainPrivacy
		request.PrivacySensitive = true
	case "recon", "passive_recon":
		request.Domain = safety.DomainRecon
		request.Action = safety.ActionRecon
		request.Passive = true
		request.Network = true
	case "active_recon":
		request.Domain = safety.DomainRecon
		request.Action = safety.ActionRecon
		request.Passive = false
		request.Network = true
	default:
		return nil
	}
	decision := safety.EvaluatePolicy(request)
	return validatePolicyDecision("extra."+key, manifest, decision)
}

func validatePolicyDecision(scope string, manifest Manifest, decision safety.PolicyDecision) error {
	if !decision.Allowed {
		return fmt.Errorf("%s blocked by policy for generated extension %s: %s", scope, manifest.Name, decision.Reason)
	}
	if decision.RequiresConfirmation && !manifest.Safety.RequiresUserApproval {
		return fmt.Errorf("%s requires safety.requires_user_approval for generated extension %s: %s", scope, manifest.Name, decision.Reason)
	}
	return nil
}
