package domainpacks

import (
	"bytes"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ManifestFile = "pack.yaml"

	DefaultStateDisabled = "disabled"
	DefaultStateEnabled  = "enabled"
)

var (
	packNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	toolNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	refNamePattern  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:/-]{0,127}$`)
)

type Manifest struct {
	Name          string      `yaml:"name" json:"name"`
	Version       string      `yaml:"version" json:"version"`
	Description   string      `yaml:"description" json:"description,omitempty"`
	Category      string      `yaml:"category" json:"category,omitempty"`
	Skills        []string    `yaml:"skills" json:"skills,omitempty"`
	RequiredTools []string    `yaml:"required_tools" json:"requiredTools,omitempty"`
	OptionalTools []string    `yaml:"optional_tools" json:"optionalTools,omitempty"`
	Permissions   Permissions `yaml:"permissions" json:"permissions"`
	Safety        Safety      `yaml:"safety" json:"safety"`
	Tests         []string    `yaml:"tests" json:"tests,omitempty"`
	DefaultState  string      `yaml:"default_state" json:"defaultState"`

	Dir string `yaml:"-" json:"-"`
}

type Permissions struct {
	Internet      PermissionDeclaration `yaml:"internet" json:"internet"`
	Filesystem    PermissionDeclaration `yaml:"filesystem" json:"filesystem"`
	Scheduler     PermissionDeclaration `yaml:"scheduler" json:"scheduler"`
	Notifications PermissionDeclaration `yaml:"notifications" json:"notifications"`
	Secrets       PermissionDeclaration `yaml:"secrets" json:"secrets"`
	Connectors    PermissionDeclaration `yaml:"connectors" json:"connectors"`
}

type PermissionDeclaration struct {
	Required       bool     `yaml:"required" json:"required,omitempty"`
	Scopes         []string `yaml:"scopes" json:"scopes,omitempty"`
	AllowedDomains []string `yaml:"allowed_domains" json:"allowedDomains,omitempty"`
	AllowedMethods []string `yaml:"allowed_methods" json:"allowedMethods,omitempty"`
	Paths          []string `yaml:"paths" json:"paths,omitempty"`
	Names          []string `yaml:"names" json:"names,omitempty"`
	SecretRefs     []string `yaml:"secret_refs" json:"secretRefs,omitempty"`
	Notes          string   `yaml:"notes" json:"notes,omitempty"`
}

type Safety struct {
	ApprovalRequiredFor []string        `yaml:"approval_required_for" json:"approvalRequiredFor,omitempty"`
	BlockedActions      []string        `yaml:"blocked_actions" json:"blockedActions,omitempty"`
	SensitiveDataRules  []string        `yaml:"sensitive_data_rules" json:"sensitiveDataRules,omitempty"`
	PolicyOverlays      []PolicyOverlay `yaml:"policy_overlays" json:"policyOverlays,omitempty"`
	PolicyOverrides     []string        `yaml:"policy_overrides" json:"policyOverrides,omitempty"`
}

type PolicyOverlay struct {
	Name    string   `yaml:"name" json:"name"`
	Mode    string   `yaml:"mode" json:"mode,omitempty"`
	Rules   []string `yaml:"rules" json:"rules,omitempty"`
	Applies []string `yaml:"applies_to" json:"appliesTo,omitempty"`
}

func Load(dir string) (Manifest, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return Manifest{}, fmt.Errorf("pack directory is required")
	}
	manifest, err := LoadFile(filepath.Join(dir, ManifestFile))
	if err != nil {
		return Manifest{}, err
	}
	manifest.Dir = dir
	return manifest, nil
}

func LoadFile(path string) (Manifest, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Manifest{}, fmt.Errorf("pack manifest path is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read %s: %w", path, err)
	}
	manifest, err := Decode(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", path, err)
	}
	manifest.Dir = filepath.Dir(path)
	return manifest, nil
}

func Decode(data []byte) (Manifest, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, err
	}
	if err := Validate(manifest); err != nil {
		return Manifest{}, err
	}
	return Normalize(manifest), nil
}

func Validate(manifest Manifest) error {
	name := strings.TrimSpace(manifest.Name)
	if name == "" {
		return fmt.Errorf("pack name is required")
	}
	if !packNamePattern.MatchString(name) {
		return fmt.Errorf("pack name must match %s", packNamePattern.String())
	}
	if strings.TrimSpace(manifest.Version) == "" {
		return fmt.Errorf("pack version is required")
	}
	if err := validateDefaultState(manifest.DefaultState); err != nil {
		return err
	}
	if err := validateTools(manifest.RequiredTools, manifest.OptionalTools); err != nil {
		return err
	}
	if err := validatePackRefList("skills", manifest.Skills); err != nil {
		return err
	}
	if err := validatePermissions(manifest.Permissions); err != nil {
		return err
	}
	if err := validateSafety(manifest.Safety); err != nil {
		return err
	}
	return validatePlainList("tests", manifest.Tests)
}

func Normalize(manifest Manifest) Manifest {
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Version = strings.TrimSpace(manifest.Version)
	manifest.Description = strings.TrimSpace(manifest.Description)
	manifest.Category = strings.TrimSpace(manifest.Category)
	manifest.Skills = cleanList(manifest.Skills, false)
	manifest.RequiredTools = cleanList(manifest.RequiredTools, false)
	manifest.OptionalTools = cleanList(manifest.OptionalTools, false)
	manifest.Tests = cleanList(manifest.Tests, false)
	manifest.DefaultState = normalizeDefaultState(manifest.DefaultState)
	manifest.Permissions = normalizePermissions(manifest.Permissions)
	manifest.Safety = normalizeSafety(manifest.Safety)
	return manifest
}

func (manifest Manifest) EnabledByDefault() bool {
	return normalizeDefaultState(manifest.DefaultState) == DefaultStateEnabled
}

func ProtectedPolicies() []string {
	policies := make([]string, 0, len(protectedPolicies))
	for policy := range protectedPolicies {
		policies = append(policies, policy)
	}
	sort.Strings(policies)
	return policies
}

func validateDefaultState(state string) error {
	switch normalizeDefaultState(state) {
	case DefaultStateDisabled, DefaultStateEnabled:
		return nil
	default:
		return fmt.Errorf("default_state must be %q or %q", DefaultStateDisabled, DefaultStateEnabled)
	}
}

func normalizeDefaultState(state string) string {
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		return DefaultStateDisabled
	}
	return state
}

func validateTools(required []string, optional []string) error {
	if err := validateToolList("required_tools", required); err != nil {
		return err
	}
	if err := validateToolList("optional_tools", optional); err != nil {
		return err
	}
	requiredSet := map[string]struct{}{}
	for _, tool := range required {
		requiredSet[strings.TrimSpace(tool)] = struct{}{}
	}
	for _, tool := range optional {
		tool = strings.TrimSpace(tool)
		if _, exists := requiredSet[tool]; exists {
			return fmt.Errorf("tool %q cannot be both required and optional", tool)
		}
	}
	return nil
}

func validateToolList(field string, values []string) error {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return fmt.Errorf("%s contains an empty tool", field)
		}
		if !toolNamePattern.MatchString(value) {
			return fmt.Errorf("%s tool %q must match %s", field, value, toolNamePattern.String())
		}
	}
	return nil
}

func validatePermissions(permissions Permissions) error {
	checks := []struct {
		name         string
		permission   PermissionDeclaration
		isInternet   bool
		isFilesystem bool
		isSecrets    bool
	}{
		{name: "internet", permission: permissions.Internet, isInternet: true},
		{name: "filesystem", permission: permissions.Filesystem, isFilesystem: true},
		{name: "scheduler", permission: permissions.Scheduler},
		{name: "notifications", permission: permissions.Notifications},
		{name: "secrets", permission: permissions.Secrets, isSecrets: true},
		{name: "connectors", permission: permissions.Connectors},
	}
	for _, check := range checks {
		if err := validatePermissionDeclaration(check.name, check.permission, check.isInternet, check.isFilesystem, check.isSecrets); err != nil {
			return err
		}
	}
	return nil
}

func validatePermissionDeclaration(name string, permission PermissionDeclaration, isInternet bool, isFilesystem bool, isSecrets bool) error {
	if err := validatePlainList("permissions."+name+".scopes", permission.Scopes); err != nil {
		return err
	}
	if err := validatePlainList("permissions."+name+".names", permission.Names); err != nil {
		return err
	}
	if err := validatePlainList("permissions."+name+".secret_refs", permission.SecretRefs); err != nil {
		return err
	}
	for _, ref := range permission.SecretRefs {
		ref = strings.TrimSpace(ref)
		if !refNamePattern.MatchString(ref) || looksLikeSecretValue(ref) {
			return fmt.Errorf("permissions.%s.secret_refs must contain references only, not secret values", name)
		}
	}
	if isInternet {
		if err := validateInternetPermission(permission); err != nil {
			return err
		}
	} else if len(permission.AllowedDomains) > 0 || len(permission.AllowedMethods) > 0 {
		return fmt.Errorf("permissions.%s cannot declare internet allowed_domains or allowed_methods", name)
	}
	if isFilesystem {
		if err := validateFilesystemPermission(permission); err != nil {
			return err
		}
	} else if len(permission.Paths) > 0 {
		return fmt.Errorf("permissions.%s cannot declare filesystem paths", name)
	}
	if !isSecrets && len(permission.SecretRefs) > 0 {
		return fmt.Errorf("permissions.%s cannot declare secret_refs", name)
	}
	return nil
}

func validateInternetPermission(permission PermissionDeclaration) error {
	if err := validatePlainList("permissions.internet.allowed_domains", permission.AllowedDomains); err != nil {
		return err
	}
	if err := validatePlainList("permissions.internet.allowed_methods", permission.AllowedMethods); err != nil {
		return err
	}
	for _, method := range permission.AllowedMethods {
		switch strings.ToUpper(strings.TrimSpace(method)) {
		case "GET", "HEAD":
		default:
			return fmt.Errorf("permissions.internet.allowed_methods supports GET and HEAD declarations only")
		}
	}
	for _, domain := range permission.AllowedDomains {
		if err := validateAllowedDomain(domain); err != nil {
			return err
		}
	}
	return nil
}

func validateFilesystemPermission(permission PermissionDeclaration) error {
	if err := validatePlainList("permissions.filesystem.paths", permission.Paths); err != nil {
		return err
	}
	for _, path := range permission.Paths {
		if err := validatePackPath(path); err != nil {
			return err
		}
	}
	return nil
}

func validatePackRefList(field string, values []string) error {
	for _, value := range values {
		original := value
		value = strings.TrimSpace(filepath.ToSlash(value))
		if value == "" {
			return fmt.Errorf("%s contains an empty value", field)
		}
		if strings.Contains(value, "://") || filepath.IsAbs(filepath.FromSlash(value)) || strings.HasPrefix(value, "/") || strings.HasPrefix(value, "~") || unsafeRelativePath(value) {
			return fmt.Errorf("%s must contain pack-relative paths only: %s", field, original)
		}
		if filepath.Clean(filepath.FromSlash(value)) == "." {
			return fmt.Errorf("%s must name a pack-relative file or directory: %s", field, original)
		}
	}
	return nil
}

func validateAllowedDomain(domain string) error {
	original := domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	if domain == "" {
		return fmt.Errorf("permissions.internet.allowed_domains contains an empty domain")
	}
	addrValue := strings.Trim(domain, "[]")
	if addr, err := netip.ParseAddr(addrValue); err == nil {
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsUnspecified() || addr.IsMulticast() {
			return fmt.Errorf("permissions.internet.allowed_domains cannot target private/local network addresses: %s", original)
		}
		return nil
	}
	if strings.ContainsAny(domain, "/: \t\r\n") {
		return fmt.Errorf("permissions.internet.allowed_domains must contain domains only: %s", original)
	}
	if domain == "localhost" ||
		strings.HasSuffix(domain, ".localhost") ||
		strings.HasSuffix(domain, ".local") ||
		domain == "host.docker.internal" {
		return fmt.Errorf("permissions.internet.allowed_domains cannot target private/local network names: %s", original)
	}
	return nil
}

func validatePackPath(path string) error {
	original := path
	path = strings.TrimSpace(filepath.ToSlash(path))
	if path == "" {
		return fmt.Errorf("permissions.filesystem.paths contains an empty path")
	}
	for _, prefix := range []string{"pack://", "profile://", "workspace://"} {
		if strings.HasPrefix(path, prefix) {
			scoped := strings.TrimPrefix(path, prefix)
			if scoped == "" || scoped == "." {
				return nil
			}
			if unsafeRelativePath(scoped) || strings.HasPrefix(scoped, "/") || strings.HasPrefix(scoped, "~") {
				return fmt.Errorf("permissions.filesystem.paths escapes pack/profile/workspace scope: %s", original)
			}
			return nil
		}
	}
	if filepath.IsAbs(filepath.FromSlash(path)) || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "~") || unsafeRelativePath(path) {
		return fmt.Errorf("permissions.filesystem.paths must stay relative or scoped: %s", original)
	}
	return nil
}

func unsafeRelativePath(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	return clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../")
}

func validateSafety(safety Safety) error {
	if err := validatePlainList("safety.approval_required_for", safety.ApprovalRequiredFor); err != nil {
		return err
	}
	if err := validatePlainList("safety.blocked_actions", safety.BlockedActions); err != nil {
		return err
	}
	if err := validatePlainList("safety.sensitive_data_rules", safety.SensitiveDataRules); err != nil {
		return err
	}
	for _, policy := range safety.PolicyOverrides {
		policy = normalizePolicyName(policy)
		if policy == "" {
			return fmt.Errorf("safety.policy_overrides contains an empty policy")
		}
		if protectedPolicies[policy] {
			return fmt.Errorf("protected policy %q cannot be overridden by a pack", policy)
		}
	}
	if len(safety.PolicyOverrides) > 0 {
		return fmt.Errorf("policy overrides are not supported by pack manifests")
	}
	for _, overlay := range safety.PolicyOverlays {
		if err := validatePolicyOverlay(overlay); err != nil {
			return err
		}
	}
	return nil
}

func validatePolicyOverlay(overlay PolicyOverlay) error {
	name := normalizePolicyName(overlay.Name)
	if name == "" {
		return fmt.Errorf("safety.policy_overlays.name is required")
	}
	mode := strings.ToLower(strings.TrimSpace(overlay.Mode))
	switch mode {
	case "", "advisory", "restrictive":
	default:
		if isOverrideMode(mode) {
			return fmt.Errorf("policy overlay %q cannot override policy mode %q", name, mode)
		}
		return fmt.Errorf("policy overlay %q mode must be advisory or restrictive", name)
	}
	if protectedPolicies[name] && isOverrideMode(mode) {
		return fmt.Errorf("protected policy %q cannot be overridden by a pack", name)
	}
	if err := validatePlainList("safety.policy_overlays.rules", overlay.Rules); err != nil {
		return err
	}
	if err := validatePlainList("safety.policy_overlays.applies_to", overlay.Applies); err != nil {
		return err
	}
	return nil
}

func isOverrideMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "override", "replace", "disable", "bypass", "weaken", "lower", "loosen":
		return true
	default:
		return false
	}
}

func validatePlainList(field string, values []string) error {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s contains an empty value", field)
		}
	}
	return nil
}

func normalizePermissions(permissions Permissions) Permissions {
	permissions.Internet = normalizePermission(permissions.Internet)
	permissions.Filesystem = normalizePermission(permissions.Filesystem)
	permissions.Scheduler = normalizePermission(permissions.Scheduler)
	permissions.Notifications = normalizePermission(permissions.Notifications)
	permissions.Secrets = normalizePermission(permissions.Secrets)
	permissions.Connectors = normalizePermission(permissions.Connectors)
	return permissions
}

func normalizePermission(permission PermissionDeclaration) PermissionDeclaration {
	permission.Scopes = cleanList(permission.Scopes, false)
	permission.AllowedDomains = cleanList(permission.AllowedDomains, true)
	permission.AllowedMethods = cleanMethods(permission.AllowedMethods)
	permission.Paths = cleanList(permission.Paths, false)
	permission.Names = cleanList(permission.Names, false)
	permission.SecretRefs = cleanList(permission.SecretRefs, false)
	permission.Notes = strings.TrimSpace(permission.Notes)
	return permission
}

func normalizeSafety(safety Safety) Safety {
	safety.ApprovalRequiredFor = cleanList(safety.ApprovalRequiredFor, false)
	safety.BlockedActions = cleanList(safety.BlockedActions, false)
	safety.SensitiveDataRules = cleanList(safety.SensitiveDataRules, false)
	safety.PolicyOverrides = cleanList(safety.PolicyOverrides, true)
	for i := range safety.PolicyOverlays {
		safety.PolicyOverlays[i].Name = normalizePolicyName(safety.PolicyOverlays[i].Name)
		safety.PolicyOverlays[i].Mode = strings.ToLower(strings.TrimSpace(safety.PolicyOverlays[i].Mode))
		safety.PolicyOverlays[i].Rules = cleanList(safety.PolicyOverlays[i].Rules, false)
		safety.PolicyOverlays[i].Applies = cleanList(safety.PolicyOverlays[i].Applies, false)
	}
	return safety
}

func cleanMethods(values []string) []string {
	cleaned := cleanList(values, false)
	for i, value := range cleaned {
		cleaned[i] = strings.ToUpper(value)
	}
	return cleaned
}

func cleanList(values []string, lower bool) []string {
	seen := map[string]struct{}{}
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

func normalizePolicyName(value string) string {
	return strings.Trim(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(value), " ", "_")), "_")
}

func looksLikeSecretValue(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(lower, "=") {
		return true
	}
	for _, marker := range []string{"sk-", "token_", "secret_", "password", "api_key"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

var protectedPolicies = map[string]bool{
	"privacy":               true,
	"privacy_policy":        true,
	"cyber":                 true,
	"cyber_policy":          true,
	"cyber_recon":           true,
	"cyber_recon_policy":    true,
	"recon":                 true,
	"recon_policy":          true,
	"medical":               true,
	"medical_policy":        true,
	"medical_safety":        true,
	"medical_safety_policy": true,
	"trading":               true,
	"trading_policy":        true,
	"trading_risk":          true,
	"trading_risk_policy":   true,
	"social":                true,
	"social_policy":         true,
	"social_posting":        true,
	"social_posting_policy": true,
	"secrets":               true,
	"secrets_policy":        true,
	"filesystem":            true,
	"filesystem_policy":     true,
	"internet":              true,
	"internet_policy":       true,
	"scheduler":             true,
	"scheduler_policy":      true,
	"extension":             true,
	"extension_policy":      true,
	"connector":             true,
	"connector_policy":      true,
	"network":               true,
	"network_policy":        true,
}
