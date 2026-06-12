package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	registryFile = "registry.json"
	auditFile    = "extensions_audit.jsonl"
)

var ErrExtensionNotFound = errors.New("extension not found")

type Store struct {
	GeneratedDir string
	RegistryPath string
	AuditPath    string
	PolicyMode   string
	Now          func() time.Time
}

type Registry struct {
	Entries map[string]Entry `json:"entries"`
}

type Entry struct {
	Name               string `json:"name"`
	Version            string `json:"version"`
	Type               string `json:"type"`
	Description        string `json:"description"`
	Dir                string `json:"dir"`
	ManifestPath       string `json:"manifestPath"`
	Enabled            bool   `json:"enabled"`
	Registered         bool   `json:"registered"`
	Valid              bool   `json:"valid"`
	Runnable           bool   `json:"runnable"`
	RunBlockedReason   string `json:"runBlockedReason"`
	ValidationError    string `json:"validationError"`
	PackageFingerprint string `json:"packageFingerprint,omitempty"`
	RegisteredAt       string `json:"registeredAt,omitempty"`
	CreatedAt          string `json:"createdAt"`
	UpdatedAt          string `json:"updatedAt"`
}

type Status struct {
	Name             string `json:"name"`
	Version          string `json:"version"`
	Type             string `json:"type"`
	Description      string `json:"description"`
	Dir              string `json:"dir"`
	ManifestPath     string `json:"manifestPath"`
	Enabled          bool   `json:"enabled"`
	Registered       bool   `json:"registered"`
	Valid            bool   `json:"valid"`
	Runnable         bool   `json:"runnable"`
	Callable         bool   `json:"callable"`
	RunBlockedReason string `json:"runBlockedReason"`
	ValidationError  string `json:"validationError"`
	RegisteredAt     string `json:"registeredAt,omitempty"`
	CreatedAt        string `json:"createdAt"`
	UpdatedAt        string `json:"updatedAt"`
}

type Detail struct {
	Status     Status            `json:"status"`
	Manifest   Manifest          `json:"manifest"`
	Inspection PackageInspection `json:"inspection"`
}

type ActionResult struct {
	Extension Status `json:"extension"`
	Message   string `json:"message"`
}

type AuditRecord struct {
	Action    string `json:"action"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	Timestamp string `json:"timestamp"`
}

func NewStore(generatedDir string, logsDir string) *Store {
	generatedDir = strings.TrimSpace(generatedDir)
	if logsDir == "" {
		logsDir = filepath.Dir(generatedDir)
	}
	return &Store{
		GeneratedDir: generatedDir,
		RegistryPath: filepath.Join(filepath.Dir(generatedDir), registryFile),
		AuditPath:    filepath.Join(logsDir, auditFile),
		PolicyMode:   "safe",
		Now:          time.Now,
	}
}

func (s *Store) List() ([]Status, error) {
	registry, err := s.discover()
	if err != nil {
		return nil, err
	}
	return registry.statuses(), nil
}

func (s *Store) RecentAudit(limit int) ([]AuditRecord, error) {
	if strings.TrimSpace(s.AuditPath) == "" {
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
	records := []AuditRecord{}
	for index := len(lines) - 1; index >= 0 && len(records) < limit; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		var record AuditRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if strings.TrimSpace(record.Action) == "" || strings.TrimSpace(record.Name) == "" {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

func (s *Store) Show(name string) (Detail, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Detail{}, fmt.Errorf("extension name is required")
	}
	registry, err := s.discover()
	if err != nil {
		return Detail{}, err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return Detail{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, name)
	}
	manifest, err := LoadManifest(entry.ManifestPath)
	if err != nil {
		return Detail{Status: entry.status()}, nil
	}
	inspection, _ := s.inspectPackageDir(entry.Dir, manifest, packageInspectionOptions{RequireEntrypoint: entry.status().Runnable})
	return Detail{Status: entry.status(), Manifest: manifest, Inspection: inspection}, nil
}

func (s *Store) Validate(name string) (ActionResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ActionResult{}, fmt.Errorf("extension name is required")
	}
	registry, err := s.discover()
	if err != nil {
		return ActionResult{}, err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return ActionResult{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, name)
	}
	entry = s.entryFromDir(entry.Dir, registry.Entries[name])
	registry.Entries[entry.Name] = entry
	if err := s.save(registry); err != nil {
		return ActionResult{}, err
	}
	status := "valid"
	if !entry.Valid {
		status = "invalid"
	}
	_ = s.audit("validate", entry.Name, status, entry.ValidationError)
	if !entry.Valid {
		return ActionResult{Extension: entry.status(), Message: entry.ValidationError}, errors.New(entry.ValidationError)
	}
	return ActionResult{Extension: entry.status(), Message: "valid"}, nil
}

func (s *Store) SetEnabled(name string, enabled bool) (ActionResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ActionResult{}, fmt.Errorf("extension name is required")
	}
	registry, err := s.discover()
	if err != nil {
		return ActionResult{}, err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return ActionResult{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, name)
	}
	entry = s.entryFromDir(entry.Dir, entry)
	delete(registry.Entries, name)
	registry.Entries[entry.Name] = entry
	if err := s.save(registry); err != nil {
		return ActionResult{}, err
	}
	if enabled && !entry.Valid {
		return ActionResult{}, fmt.Errorf("invalid extension cannot be enabled: %s", entry.ValidationError)
	}
	if enabled && !entry.Registered {
		return ActionResult{}, fmt.Errorf("extension must be tested and registered before it can be enabled")
	}
	if enabled && !entry.Runnable {
		reason := strings.TrimSpace(entry.RunBlockedReason)
		if reason == "" {
			reason = "extension is not runnable"
		}
		return ActionResult{}, fmt.Errorf("extension cannot be enabled: %s", reason)
	}
	entry.Enabled = enabled
	entry.UpdatedAt = s.timestamp()
	registry.Entries[entry.Name] = entry
	if err := s.save(registry); err != nil {
		return ActionResult{}, err
	}
	message := "enabled"
	if !enabled {
		message = "disabled"
	}
	_ = s.audit(message, entry.Name, "ok", "")
	return ActionResult{Extension: entry.status(), Message: message}, nil
}

func (s *Store) Delete(name string) (ActionResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return ActionResult{}, fmt.Errorf("extension name is required")
	}
	registry, err := s.discover()
	if err != nil {
		return ActionResult{}, err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return ActionResult{}, fmt.Errorf("%w: %s", ErrExtensionNotFound, name)
	}
	if err := ensureChild(s.GeneratedDir, entry.Dir); err != nil {
		return ActionResult{}, err
	}
	if err := os.RemoveAll(entry.Dir); err != nil {
		return ActionResult{}, fmt.Errorf("delete extension files: %w", err)
	}
	delete(registry.Entries, name)
	if err := s.save(registry); err != nil {
		return ActionResult{}, err
	}
	_ = s.audit("delete", name, "ok", "")
	return ActionResult{Extension: entry.status(), Message: "deleted"}, nil
}

func LoadManifest(path string) (Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read extension manifest: %w", err)
	}
	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("parse extension manifest: %w", err)
	}
	return manifest, nil
}

func (s *Store) discover() (Registry, error) {
	if strings.TrimSpace(s.GeneratedDir) == "" {
		return Registry{}, fmt.Errorf("generated extension directory is required")
	}
	if err := os.MkdirAll(s.GeneratedDir, 0o755); err != nil {
		return Registry{}, fmt.Errorf("create generated extension directory: %w", err)
	}
	registry, err := s.load()
	if err != nil {
		return Registry{}, err
	}
	if registry.Entries == nil {
		registry.Entries = map[string]Entry{}
	}
	found := map[string]bool{}
	children, err := os.ReadDir(s.GeneratedDir)
	if err != nil {
		return Registry{}, fmt.Errorf("read generated extension directory: %w", err)
	}
	for _, child := range children {
		if !child.IsDir() {
			continue
		}
		dir := filepath.Join(s.GeneratedDir, child.Name())
		existing := registry.Entries[child.Name()]
		entry := s.entryFromDir(dir, existing)
		if entry.Name != child.Name() {
			delete(registry.Entries, child.Name())
		}
		found[entry.Name] = true
		registry.Entries[entry.Name] = entry
	}
	for name, entry := range registry.Entries {
		if !found[name] {
			if _, err := os.Stat(entry.Dir); errors.Is(err, os.ErrNotExist) {
				delete(registry.Entries, name)
			}
		}
	}
	if err := s.save(registry); err != nil {
		return Registry{}, err
	}
	return registry, nil
}

func (s *Store) entryFromDir(dir string, existing Entry) Entry {
	now := s.timestamp()
	manifestPath := filepath.Join(dir, ManifestFile)
	entry := existing
	if entry.CreatedAt == "" {
		entry.CreatedAt = now
	}
	entry.UpdatedAt = now
	entry.Dir = dir
	entry.ManifestPath = manifestPath
	if entry.Name == "" {
		entry.Name = filepath.Base(dir)
	}
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		entry.Name = filepath.Base(dir)
		entry.Enabled = false
		entry.Registered = false
		entry.Valid = false
		entry.Runnable = false
		entry.RunBlockedReason = ""
		entry.ValidationError = err.Error()
		return entry
	}
	entry.Name = filepath.Base(dir)
	entry.Version = manifest.Version
	entry.Type = manifest.Type
	entry.Description = manifest.Description
	if err := ValidateManifestWithPolicyMode(manifest, s.PolicyMode); err != nil {
		entry.Enabled = false
		entry.Registered = false
		entry.Valid = false
		entry.Runnable = false
		entry.RunBlockedReason = ""
		entry.ValidationError = err.Error()
		return entry
	}
	if err := ensureManifestDirName(dir, manifest.Name); err != nil {
		entry.Enabled = false
		entry.Registered = false
		entry.Valid = false
		entry.Runnable = false
		entry.RunBlockedReason = ""
		entry.ValidationError = err.Error()
		return entry
	}
	if inspection, err := s.inspectPackageDir(dir, manifest, packageInspectionOptions{}); err != nil {
		entry.Name = manifest.Name
		entry.Enabled = false
		entry.Registered = false
		entry.Valid = false
		entry.Runnable = false
		entry.RunBlockedReason = ""
		entry.ValidationError = err.Error()
		_ = inspection
		return entry
	}
	fingerprint, err := fingerprintPackageDir(dir)
	if err != nil {
		entry.Name = manifest.Name
		entry.Enabled = false
		entry.Registered = false
		entry.Valid = false
		entry.Runnable = false
		entry.RunBlockedReason = ""
		entry.ValidationError = err.Error()
		return entry
	}
	entry.PackageFingerprint = fingerprint
	entry.Name = manifest.Name
	entry.Valid = true
	entry.ValidationError = ""
	if err := validateRunnableManifestWithPolicyMode(manifest, s.PolicyMode); err != nil {
		entry.Runnable = false
		entry.RunBlockedReason = err.Error()
	} else if _, err := s.inspectPackageDir(dir, manifest, packageInspectionOptions{RequireEntrypoint: true}); err != nil {
		entry.Runnable = false
		entry.RunBlockedReason = err.Error()
	} else {
		entry.Runnable = true
		entry.RunBlockedReason = ""
	}
	if entry.Runnable {
		switch {
		case !existing.Registered:
			entry.Registered = false
			entry.Enabled = false
			entry.Runnable = false
			entry.RunBlockedReason = "extension must be tested and registered before it can run"
		case existing.PackageFingerprint != fingerprint:
			entry.Registered = false
			entry.Enabled = false
			entry.Runnable = false
			entry.RunBlockedReason = "extension package changed since last registration; run tests and register again"
		default:
			entry.Registered = true
			entry.RegisteredAt = existing.RegisteredAt
		}
	} else {
		entry.Registered = existing.Registered && existing.PackageFingerprint == fingerprint
		if !entry.Registered {
			entry.Enabled = false
		}
		entry.RegisteredAt = existing.RegisteredAt
	}
	return entry
}

func (s *Store) load() (Registry, error) {
	var registry Registry
	data, err := os.ReadFile(s.RegistryPath)
	if errors.Is(err, os.ErrNotExist) {
		registry.Entries = map[string]Entry{}
		return registry, nil
	}
	if err != nil {
		return Registry{}, fmt.Errorf("read extension registry: %w", err)
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		return Registry{}, fmt.Errorf("parse extension registry: %w", err)
	}
	if registry.Entries == nil {
		registry.Entries = map[string]Entry{}
	}
	return registry, nil
}

func (s *Store) save(registry Registry) error {
	if err := os.MkdirAll(filepath.Dir(s.RegistryPath), 0o755); err != nil {
		return fmt.Errorf("create extension registry directory: %w", err)
	}
	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return fmt.Errorf("encode extension registry: %w", err)
	}
	if err := os.WriteFile(s.RegistryPath, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write extension registry: %w", err)
	}
	return nil
}

func (s *Store) audit(action string, name string, status string, message string) error {
	if strings.TrimSpace(s.AuditPath) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(s.AuditPath), 0o755); err != nil {
		return err
	}
	record := AuditRecord{
		Action:    action,
		Name:      name,
		Status:    status,
		Message:   redactExtensionText(message),
		Timestamp: s.timestamp(),
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(s.AuditPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func (r Registry) statuses() []Status {
	statuses := make([]Status, 0, len(r.Entries))
	for _, entry := range r.Entries {
		statuses = append(statuses, entry.status())
	}
	sort.Slice(statuses, func(i int, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})
	return statuses
}

func (e Entry) status() Status {
	return Status{
		Name:             e.Name,
		Version:          e.Version,
		Type:             e.Type,
		Description:      e.Description,
		Dir:              e.Dir,
		ManifestPath:     e.ManifestPath,
		Enabled:          e.Enabled,
		Registered:       e.Registered,
		Valid:            e.Valid,
		Runnable:         e.Runnable,
		Callable:         e.Enabled && e.Registered && e.Valid && e.Runnable,
		RunBlockedReason: e.RunBlockedReason,
		ValidationError:  e.ValidationError,
		RegisteredAt:     e.RegisteredAt,
		CreatedAt:        e.CreatedAt,
		UpdatedAt:        e.UpdatedAt,
	}
}

func (s *Store) timestamp() string {
	now := time.Now
	if s.Now != nil {
		now = s.Now
	}
	return now().UTC().Format(time.RFC3339)
}

func ensureManifestDirName(dir string, manifestName string) error {
	if filepath.Base(dir) != manifestName {
		return fmt.Errorf("extension directory name %q must match manifest name %q", filepath.Base(dir), manifestName)
	}
	return nil
}

func ensureChild(root string, child string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	childAbs, err := filepath.Abs(child)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(rootAbs, childAbs)
	if err != nil {
		return err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) || rel == ".." || filepath.IsAbs(rel) {
		return fmt.Errorf("extension path is outside generated extension directory")
	}
	return ensureChildPathHasNoSymlink(rootAbs, rel)
}

func ensureChildPathHasNoSymlink(rootAbs string, rel string) error {
	current := rootAbs
	for _, part := range strings.Split(filepath.Clean(rel), string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect extension path: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("extension path contains symlink inside generated extension directory")
		}
	}
	return nil
}
