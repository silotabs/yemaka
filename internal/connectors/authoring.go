package connectors

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/secrets"
)

const (
	generatedRegistryFile = "generated_connectors.json"
	generatedAuditFile    = "generated_connectors_audit.jsonl"
)

var ErrGeneratedConnectorNotFound = errors.New("generated connector not found")

type GeneratedStore struct {
	RootDir      string
	RegistryPath string
	AuditPath    string
	PolicyMode   string
	Now          func() time.Time
}

type GeneratedRegistry struct {
	Entries map[string]GeneratedEntry `json:"entries"`
}

type GeneratedEntry struct {
	Name            string     `json:"name"`
	Manifest        Manifest   `json:"manifest"`
	Tests           TestReport `json:"tests"`
	Approved        bool       `json:"approved"`
	Installed       bool       `json:"installed"`
	Enabled         bool       `json:"enabled"`
	ValidationError string     `json:"validationError,omitempty"`
	CreatedAt       string     `json:"createdAt"`
	UpdatedAt       string     `json:"updatedAt"`
	InstalledAt     string     `json:"installedAt,omitempty"`
}

type TestReport struct {
	Status    string   `json:"status"`
	Commands  []string `json:"commands,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	PassedAt  string   `json:"passedAt,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
}

type GeneratedInstallInput struct {
	Manifest Manifest   `json:"manifest"`
	Tests    TestReport `json:"tests"`
	Approved bool       `json:"approved"`
}

type GeneratedActionResult struct {
	Connector GeneratedEntry `json:"connector"`
	Message   string         `json:"message"`
}

type GeneratedAuditRecord struct {
	Action    string `json:"action"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp"`
}

func NewGeneratedStore(rootDir string) *GeneratedStore {
	rootDir = strings.TrimSpace(rootDir)
	return &GeneratedStore{
		RootDir:      rootDir,
		RegistryPath: filepath.Join(rootDir, generatedRegistryFile),
		AuditPath:    filepath.Join(rootDir, generatedAuditFile),
		PolicyMode:   "safe",
		Now:          time.Now,
	}
}

func (s *GeneratedStore) Install(input GeneratedInstallInput) (GeneratedActionResult, error) {
	if s == nil {
		return GeneratedActionResult{}, fmt.Errorf("generated connector store is required")
	}
	if !input.Approved {
		return GeneratedActionResult{}, fmt.Errorf("user approval is required before installing a generated connector")
	}
	if err := validateGeneratedTests(input.Tests); err != nil {
		return GeneratedActionResult{}, err
	}
	manifest := normalizeValidatedManifest(input.Manifest)
	if err := validateGeneratedConnectorManifest(manifest, s.PolicyMode); err != nil {
		_ = s.audit("install", strings.TrimSpace(input.Manifest.Name), "blocked", err.Error())
		return GeneratedActionResult{}, err
	}
	registry, err := s.load()
	if err != nil {
		return GeneratedActionResult{}, err
	}
	now := s.timestamp()
	entry := registry.Entries[manifest.Name]
	if entry.CreatedAt == "" {
		entry.CreatedAt = now
	}
	entry.Name = manifest.Name
	entry.Manifest = manifest
	entry.Tests = normalizeTestReport(input.Tests, now)
	entry.Approved = true
	entry.Installed = true
	entry.Enabled = false
	entry.ValidationError = ""
	entry.UpdatedAt = now
	entry.InstalledAt = now
	registry.Entries[entry.Name] = entry
	if err := s.save(registry); err != nil {
		return GeneratedActionResult{}, err
	}
	_ = s.audit("install", entry.Name, "ok", "installed disabled generated connector candidate")
	return GeneratedActionResult{Connector: entry, Message: "installed disabled; enable readiness after secret references are configured"}, nil
}

func (s *GeneratedStore) List() ([]GeneratedEntry, error) {
	if s == nil {
		return nil, fmt.Errorf("generated connector store is required")
	}
	registry, err := s.load()
	if err != nil {
		return nil, err
	}
	entries := make([]GeneratedEntry, 0, len(registry.Entries))
	for _, entry := range registry.Entries {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func (s *GeneratedStore) Get(name string) (GeneratedEntry, error) {
	if s == nil {
		return GeneratedEntry{}, fmt.Errorf("generated connector store is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return GeneratedEntry{}, fmt.Errorf("generated connector name is required")
	}
	registry, err := s.load()
	if err != nil {
		return GeneratedEntry{}, err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return GeneratedEntry{}, fmt.Errorf("%w: %s", ErrGeneratedConnectorNotFound, name)
	}
	return entry, nil
}

func (s *GeneratedStore) Registry(cfg config.ConnectorsConfig) ([]RegistryEntry, error) {
	entries, err := s.List()
	if err != nil {
		return nil, err
	}
	return RegistryWithGeneratedEntriesWithPolicy(cfg, entries, s.PolicyMode)
}

func (s *GeneratedStore) SetEnabled(name string, enabled bool) (GeneratedActionResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return GeneratedActionResult{}, fmt.Errorf("generated connector name is required")
	}
	registry, err := s.load()
	if err != nil {
		return GeneratedActionResult{}, err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return GeneratedActionResult{}, fmt.Errorf("%w: %s", ErrGeneratedConnectorNotFound, name)
	}
	if enabled {
		if err := s.validateGeneratedConnectorEnablement(entry); err != nil {
			entry.Enabled = false
			entry.ValidationError = err.Error()
			entry.UpdatedAt = s.timestamp()
			registry.Entries[name] = entry
			_ = s.save(registry)
			_ = s.audit("enable", name, "blocked", err.Error())
			return GeneratedActionResult{Connector: entry}, err
		}
		entry.Enabled = true
		entry.ValidationError = ""
		entry.UpdatedAt = s.timestamp()
		registry.Entries[name] = entry
		if err := s.save(registry); err != nil {
			return GeneratedActionResult{}, err
		}
		_ = s.audit("enable", name, "ok", "runtime execution enabled with connector policy gates")
		return GeneratedActionResult{Connector: entry, Message: "enabled for governed runtime execution; generated connector runs remain token-, policy-, rate-, and extension-gated"}, nil
	}
	entry.Enabled = false
	entry.ValidationError = ""
	entry.UpdatedAt = s.timestamp()
	registry.Entries[name] = entry
	if err := s.save(registry); err != nil {
		return GeneratedActionResult{}, err
	}
	_ = s.audit("disable", name, "ok", "")
	return GeneratedActionResult{Connector: entry, Message: "disabled"}, nil
}

func (s *GeneratedStore) RecentAudit(limit int) ([]GeneratedAuditRecord, error) {
	if s == nil || strings.TrimSpace(s.AuditPath) == "" {
		return nil, nil
	}
	data, err := os.ReadFile(s.AuditPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	records := []GeneratedAuditRecord{}
	for index := len(lines) - 1; index >= 0 && len(records) < limit; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		var record GeneratedAuditRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if record.Action == "" || record.Name == "" {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *GeneratedStore) RecordRun(name string, status string, message string) error {
	return s.audit("run", strings.TrimSpace(name), strings.TrimSpace(status), strings.TrimSpace(message))
}

func validateGeneratedTests(tests TestReport) error {
	status := strings.ToLower(strings.TrimSpace(tests.Status))
	if status != "passed" {
		return fmt.Errorf("generated connector tests must pass before install")
	}
	if len(tests.Commands) == 0 && strings.TrimSpace(tests.Summary) == "" && len(tests.Artifacts) == 0 {
		return fmt.Errorf("generated connector tests must include commands, artifacts, or a summary")
	}
	for _, command := range tests.Commands {
		if strings.TrimSpace(command) == "" {
			return fmt.Errorf("generated connector test command cannot be empty")
		}
	}
	for _, artifact := range tests.Artifacts {
		artifact = strings.TrimSpace(artifact)
		if artifact == "" || filepath.IsAbs(artifact) || strings.Contains(artifact, "..") {
			return fmt.Errorf("generated connector test artifacts must be relative safe paths")
		}
	}
	return nil
}

func normalizeTestReport(tests TestReport, now string) TestReport {
	tests.Status = strings.ToLower(strings.TrimSpace(tests.Status))
	tests.Summary = strings.TrimSpace(tests.Summary)
	tests.Commands = normalizeConnectorList(tests.Commands, false)
	tests.Artifacts = normalizeConnectorList(tests.Artifacts, false)
	if strings.TrimSpace(tests.PassedAt) == "" {
		tests.PassedAt = now
	}
	return tests
}

func validateGeneratedConnectorAuth(ref secrets.Reference) error {
	ref = secrets.Normalize(ref)
	if secretReferenceLooksLikeValue(ref) {
		return fmt.Errorf("generated connector auth must use secret references only, not secret values")
	}
	if ref.Provider != secrets.ProviderNone && ref.Provider != secrets.ProviderEnv && ref.Provider != secrets.ProviderKeychain {
		return fmt.Errorf("unsupported generated connector auth provider %q", ref.Provider)
	}
	return nil
}

func validateGeneratedConnectorPermissions(manifest Manifest) error {
	perms := manifest.Permissions
	if perms.Posting || perms.Trading || perms.Deployment || perms.Mutating {
		return fmt.Errorf("generated connectors cannot request posting, trading, deployment, or mutating permissions")
	}
	if perms.Outbound {
		if !manifest.RequiresApproval {
			return fmt.Errorf("generated outbound connectors require approval")
		}
		if len(manifest.AllowedDomains) == 0 {
			return fmt.Errorf("generated outbound connectors must declare allowed domains")
		}
	}
	return nil
}

func (s *GeneratedStore) validateGeneratedConnectorEnablement(entry GeneratedEntry) error {
	if !entry.Installed {
		return fmt.Errorf("generated connector must be installed before enabling")
	}
	if !entry.Approved {
		return fmt.Errorf("generated connector must be approved before enabling")
	}
	if err := validateGeneratedTests(entry.Tests); err != nil {
		return err
	}
	manifest := normalizeValidatedManifest(entry.Manifest)
	if err := validateGeneratedConnectorManifest(manifest, s.PolicyMode); err != nil {
		return err
	}
	required := manifest.Auth.Provider != "" && manifest.Auth.Provider != secrets.ProviderNone
	secret := secretStatusFromReference(manifest.Auth, required)
	if required && !secret.Ready {
		return fmt.Errorf("generated connector secret is not ready: %s", secret.Detail)
	}
	return nil
}

func validateGeneratedConnectorManifest(manifest Manifest, policyMode string) error {
	manifest = normalizeValidatedManifest(manifest)
	if isBuiltInConnectorName(manifest.Name) {
		return fmt.Errorf("generated connector %q conflicts with an existing connector", manifest.Name)
	}
	if err := validateGeneratedConnectorEndpoint(manifest); err != nil {
		return err
	}
	if err := validateGeneratedConnectorPermissions(manifest); err != nil {
		return err
	}
	if err := validateGeneratedConnectorAuth(manifest.Auth); err != nil {
		return err
	}
	if err := ValidateManifestWithPolicyMode(manifest, policyMode); err != nil {
		return err
	}
	if err := validateGeneratedConnectorDomains(manifest.AllowedDomains); err != nil {
		return err
	}
	return nil
}

func validateGeneratedConnectorEndpoint(manifest Manifest) error {
	endpoint := strings.TrimSpace(manifest.Endpoint)
	if !strings.HasPrefix(endpoint, "extension:") {
		return fmt.Errorf("generated connector endpoint must reference extension:%s", manifest.Name)
	}
	target := strings.TrimSpace(strings.TrimPrefix(endpoint, "extension:"))
	if target == "" || !connectorNamePattern.MatchString(target) {
		return fmt.Errorf("generated connector endpoint must reference a safe generated extension name")
	}
	if target != manifest.Name {
		return fmt.Errorf("generated connector endpoint must match connector name %q", manifest.Name)
	}
	return nil
}

func validateGeneratedConnectorDomains(domains []string) error {
	for _, domain := range domains {
		original := domain
		domain = strings.ToLower(strings.TrimSpace(domain))
		if domain == "" {
			return fmt.Errorf("generated connector allowed domains cannot contain empty values")
		}
		addrValue := strings.Trim(domain, "[]")
		if addr, err := netip.ParseAddr(addrValue); err == nil {
			if generatedConnectorPrivateOrLocalAddress(addr) {
				return fmt.Errorf("generated connector allowed domains cannot target private/local network addresses: %s", original)
			}
			continue
		}
		if domain == "localhost" ||
			strings.HasSuffix(domain, ".localhost") ||
			strings.HasSuffix(domain, ".local") ||
			domain == "host.docker.internal" {
			return fmt.Errorf("generated connector allowed domains cannot target private/local network names: %s", original)
		}
	}
	return nil
}

func generatedConnectorPrivateOrLocalAddress(addr netip.Addr) bool {
	addr = addr.Unmap()
	return addr.IsLoopback() ||
		addr.IsPrivate() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsUnspecified() ||
		addr.IsMulticast()
}

func (s *GeneratedStore) load() (GeneratedRegistry, error) {
	registry := GeneratedRegistry{Entries: map[string]GeneratedEntry{}}
	path := strings.TrimSpace(s.RegistryPath)
	if path == "" {
		if strings.TrimSpace(s.RootDir) == "" {
			return registry, fmt.Errorf("generated connector registry path is required")
		}
		path = filepath.Join(s.RootDir, generatedRegistryFile)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return registry, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return registry, nil
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return registry, err
	}
	if registry.Entries == nil {
		registry.Entries = map[string]GeneratedEntry{}
	}
	return registry, nil
}

func (s *GeneratedStore) save(registry GeneratedRegistry) error {
	path := strings.TrimSpace(s.RegistryPath)
	if path == "" {
		path = filepath.Join(s.RootDir, generatedRegistryFile)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (s *GeneratedStore) audit(action string, name string, status string, message string) error {
	path := strings.TrimSpace(s.AuditPath)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	record := GeneratedAuditRecord{
		Action:    strings.TrimSpace(action),
		Name:      strings.TrimSpace(name),
		Status:    strings.TrimSpace(status),
		Message:   strings.TrimSpace(message),
		Timestamp: s.timestamp(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	return appendLine(path, data)
}

func (s *GeneratedStore) timestamp() string {
	now := time.Now
	if s != nil && s.Now != nil {
		now = s.Now
	}
	return now().UTC().Format(time.RFC3339)
}

func appendLine(path string, line []byte) error {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(line); err != nil {
		return err
	}
	_, err = file.WriteString("\n")
	return err
}

func isBuiltInConnectorName(name string) bool {
	for _, known := range KnownNames() {
		if strings.TrimSpace(name) == known {
			return true
		}
	}
	return false
}
