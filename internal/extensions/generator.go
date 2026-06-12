package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	generationsFile = "extension_generations.jsonl"
	testsFile       = "extension_tests.jsonl"
	snapshotDirName = "snapshots"

	defaultExtensionTestTimeout = 2 * time.Minute
)

var slugPattern = regexp.MustCompile(`[^a-z0-9_]+`)

type Proposal struct {
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Type             string   `json:"type"`
	Reason           string   `json:"reason"`
	RequiresApproval bool     `json:"requiresApproval"`
	Files            []string `json:"files"`
	Permissions      []string `json:"permissions"`
	NetworkMode      string   `json:"networkMode,omitempty"`
	AllowedDomains   []string `json:"allowedDomains,omitempty"`
	AllowedMethods   []string `json:"allowedMethods,omitempty"`
}

type GenerateInput struct {
	Name              string        `json:"name"`
	Description       string        `json:"description"`
	Approved          bool          `json:"approved"`
	MaxRuntimeSeconds int           `json:"maxRuntimeSeconds"`
	BrokeredNetwork   bool          `json:"brokeredNetwork"`
	AllowedDomains    []string      `json:"allowedDomains,omitempty"`
	Draft             *PackageDraft `json:"draft,omitempty"`
	TestTimeout       time.Duration `json:"-"`
}

type GenerationResult struct {
	Extension      Status     `json:"extension"`
	Snapshot       Snapshot   `json:"snapshot"`
	Tests          TestResult `json:"tests"`
	SkillCandidate string     `json:"skillCandidate"`
	Message        string     `json:"message"`
}

type Snapshot struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	TargetDir string   `json:"targetDir"`
	Existed   bool     `json:"existed"`
	BackupDir string   `json:"backupDir,omitempty"`
	Files     []string `json:"files"`
	CreatedAt string   `json:"createdAt"`
}

type TestResult struct {
	Name        string   `json:"name"`
	Commands    []string `json:"commands"`
	Status      string   `json:"status"`
	Output      string   `json:"output,omitempty"`
	DurationMS  int64    `json:"durationMs"`
	StartedAt   string   `json:"startedAt"`
	CompletedAt string   `json:"completedAt"`
}

type RollbackResult struct {
	Name       string `json:"name"`
	SnapshotID string `json:"snapshotId"`
	TargetDir  string `json:"targetDir"`
	Message    string `json:"message"`
}

func (s *Store) Propose(request string) (Proposal, error) {
	request = strings.TrimSpace(request)
	if request == "" {
		return Proposal{}, fmt.Errorf("extension request is required")
	}
	name := safeExtensionName(request)
	if looksBrokeredNetworkRequest(request) {
		domains := inferAllowedDomains(request)
		permissions := []string{
			"filesystem.read=false",
			"filesystem.write=false",
			"network=core_broker",
			"network.methods=GET,HEAD",
			"shell=false",
			"secrets=false",
		}
		if len(domains) > 0 {
			permissions = append(permissions, "network.domains="+strings.Join(domains, ","))
		}
		return Proposal{
			Name:             name,
			Description:      compactDescription(request),
			Type:             TypeTool,
			Reason:           "This capability needs public web access, so Yemaka can generate a tool that asks the trusted core internet service to fetch approved domains.",
			RequiresApproval: true,
			Files: []string{
				ManifestFile,
				"README.md",
				"go.mod",
				"main.go",
				"main_test.go",
			},
			Permissions:    permissions,
			NetworkMode:    "core_broker",
			AllowedDomains: domains,
			AllowedMethods: []string{"GET", "HEAD"},
		}, nil
	}
	return Proposal{
		Name:             name,
		Description:      compactDescription(request),
		Type:             TypeTool,
		Reason:           "No generated extension is selected for this capability yet.",
		RequiresApproval: true,
		Files: []string{
			ManifestFile,
			"README.md",
			"go.mod",
			"main.go",
			"main_test.go",
		},
		Permissions: []string{
			"filesystem.read=false",
			"filesystem.write=false",
			"network=false",
			"shell=false",
			"secrets=false",
		},
	}, nil
}

func (s *Store) Generate(ctx context.Context, input GenerateInput) (GenerationResult, error) {
	if !input.Approved {
		return GenerationResult{}, fmt.Errorf("user approval is required before extension files are created")
	}
	name := strings.TrimSpace(input.Name)
	if !namePattern.MatchString(name) {
		return GenerationResult{}, fmt.Errorf("extension name must match %s", namePattern.String())
	}
	description := strings.TrimSpace(input.Description)
	if description == "" {
		return GenerationResult{}, fmt.Errorf("extension description is required")
	}
	if err := os.MkdirAll(s.GeneratedDir, 0o755); err != nil {
		return GenerationResult{}, fmt.Errorf("create generated extension directory: %w", err)
	}
	targetDir := filepath.Join(s.GeneratedDir, name)
	if err := ensureChild(s.GeneratedDir, targetDir); err != nil {
		return GenerationResult{}, err
	}
	if _, err := os.Stat(targetDir); err == nil {
		return GenerationResult{}, fmt.Errorf("extension already exists: %s", name)
	} else if !errors.Is(err, os.ErrNotExist) {
		return GenerationResult{}, fmt.Errorf("check extension directory: %w", err)
	}

	var draft normalizedDraft
	hasDraft := input.Draft != nil
	if hasDraft {
		var err error
		draft, err = s.normalizePackageDraft(name, input.Draft)
		if err != nil {
			return GenerationResult{}, err
		}
	}
	snapshotFiles := generatedPackageFiles(name)
	if hasDraft {
		snapshotFiles = generatedSnapshotFilesFromDraft(name, draft.Files)
	}
	snapshot := Snapshot{
		ID:        uuid.NewString(),
		Name:      name,
		TargetDir: targetDir,
		Existed:   false,
		Files:     snapshotFiles,
		CreatedAt: s.timestamp(),
	}
	if err := s.saveSnapshot(snapshot); err != nil {
		return GenerationResult{}, err
	}
	var err error
	if hasDraft {
		err = s.writeDraftGeneratedPackage(targetDir, input.Draft, draft)
	} else if input.BrokeredNetwork {
		err = writeGeneratedBrokeredNetworkPackage(targetDir, name, description, input.MaxRuntimeSeconds, input.AllowedDomains)
	} else {
		err = writeGeneratedPackage(targetDir, name, description, input.MaxRuntimeSeconds)
	}
	if err != nil {
		_ = os.RemoveAll(targetDir)
		_ = s.audit("generate", name, "failed", err.Error())
		return GenerationResult{Snapshot: snapshot}, err
	}

	testResult, err := s.RegisterGenerated(ctx, name, TestOptions{Timeout: input.TestTimeout})
	if err != nil {
		_ = os.RemoveAll(targetDir)
		_ = s.removeRegistryEntry(name)
		_ = s.audit("generate", name, "failed", err.Error())
		return GenerationResult{Snapshot: snapshot, Tests: testResult}, err
	}
	detail, err := s.Show(name)
	if err != nil {
		return GenerationResult{Snapshot: snapshot, Tests: testResult}, err
	}
	result := GenerationResult{
		Extension:      detail.Status,
		Snapshot:       snapshot,
		Tests:          testResult,
		SkillCandidate: skillCandidateText(name, description),
		Message:        "generated, tested, built, and registered",
	}
	_ = s.recordGeneration(result)
	_ = s.audit("generate", name, "ok", result.Message)
	return result, nil
}

type TestOptions struct {
	Timeout time.Duration
}

func (s *Store) Test(ctx context.Context, name string, options TestOptions) (TestResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return TestResult{}, fmt.Errorf("extension name is required")
	}
	detail, err := s.Show(name)
	if err != nil {
		return TestResult{}, err
	}
	return s.testManifest(ctx, detail.Status.Dir, detail.Manifest, options)
}

func (s *Store) RegisterGenerated(ctx context.Context, name string, options TestOptions) (TestResult, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return TestResult{}, fmt.Errorf("extension name is required")
	}
	detail, err := s.Show(name)
	if err != nil {
		return TestResult{}, err
	}
	if err := ValidateManifestWithPolicyMode(detail.Manifest, s.PolicyMode); err != nil {
		_ = s.disableIfPresent(name)
		return TestResult{}, err
	}
	if err := validateRunnableManifestWithPolicyMode(detail.Manifest, s.PolicyMode); err != nil {
		_ = s.disableIfPresent(name)
		return TestResult{}, err
	}
	testResult, err := s.testManifest(ctx, detail.Status.Dir, detail.Manifest, options)
	if err != nil {
		_ = s.disableIfPresent(name)
		return testResult, err
	}
	if err := s.buildGoExtension(ctx, detail.Status.Dir, detail.Manifest, options); err != nil {
		_ = s.disableIfPresent(name)
		return testResult, err
	}
	if _, err := s.inspectPackageDir(detail.Status.Dir, detail.Manifest, packageInspectionOptions{RequireEntrypoint: true}); err != nil {
		_ = s.disableIfPresent(name)
		return testResult, fmt.Errorf("extension package hardening check failed: %w", err)
	}
	if _, err := s.Validate(name); err != nil {
		_ = s.disableIfPresent(name)
		return testResult, err
	}
	if err := s.markRegistered(name); err != nil {
		_ = s.disableIfPresent(name)
		return testResult, err
	}
	if _, err := s.SetEnabled(name, true); err != nil {
		return testResult, err
	}
	_ = s.audit("register", name, "ok", "tests passed")
	return testResult, nil
}

func (s *Store) RollbackGeneration(nameOrSnapshot string) (RollbackResult, error) {
	query := strings.TrimSpace(nameOrSnapshot)
	if query == "" {
		return RollbackResult{}, fmt.Errorf("extension name or snapshot id is required")
	}
	snapshot, err := s.findSnapshot(query)
	if err != nil {
		return RollbackResult{}, err
	}
	if snapshot.TargetDir == "" {
		return RollbackResult{}, fmt.Errorf("snapshot target directory is empty")
	}
	if err := ensureChild(s.GeneratedDir, snapshot.TargetDir); err != nil {
		return RollbackResult{}, err
	}
	if err := s.ensureRollbackSnapshotScope(snapshot); err != nil {
		return RollbackResult{}, err
	}
	if snapshot.Existed {
		return s.rollbackPreExistingSnapshot(snapshot)
	}
	if err := os.RemoveAll(snapshot.TargetDir); err != nil {
		return RollbackResult{}, fmt.Errorf("remove generated extension: %w", err)
	}
	_ = s.removeRegistryEntry(snapshot.Name)
	_ = s.audit("rollback", snapshot.Name, "ok", snapshot.ID)
	return RollbackResult{
		Name:       snapshot.Name,
		SnapshotID: snapshot.ID,
		TargetDir:  snapshot.TargetDir,
		Message:    "generated extension files and registry entry removed",
	}, nil
}

func (s *Store) rollbackPreExistingSnapshot(snapshot Snapshot) (RollbackResult, error) {
	backupDir := strings.TrimSpace(snapshot.BackupDir)
	if backupDir == "" {
		return RollbackResult{}, fmt.Errorf("rollback for pre-existing extension snapshot %s requires a backup directory", snapshot.ID)
	}
	if err := ensureChildOf(s.snapshotDir(), backupDir, "extension backup path is outside snapshot directory"); err != nil {
		return RollbackResult{}, err
	}
	info, err := os.Stat(backupDir)
	if err != nil {
		return RollbackResult{}, fmt.Errorf("stat extension backup: %w", err)
	}
	if !info.IsDir() {
		return RollbackResult{}, fmt.Errorf("extension backup path is not a directory")
	}

	tempDir := snapshot.TargetDir + ".rollback_tmp_" + safeExtensionName(snapshot.ID)
	if err := ensureChild(s.GeneratedDir, tempDir); err != nil {
		return RollbackResult{}, err
	}
	_ = os.RemoveAll(tempDir)
	if err := copyDirectoryStrict(backupDir, tempDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return RollbackResult{}, fmt.Errorf("restore extension backup: %w", err)
	}
	if err := os.RemoveAll(snapshot.TargetDir); err != nil {
		_ = os.RemoveAll(tempDir)
		return RollbackResult{}, fmt.Errorf("remove current extension before restore: %w", err)
	}
	if err := os.Rename(tempDir, snapshot.TargetDir); err != nil {
		return RollbackResult{}, fmt.Errorf("activate restored extension backup: %w", err)
	}
	_ = s.audit("rollback", snapshot.Name, "ok", snapshot.ID)
	return RollbackResult{
		Name:       snapshot.Name,
		SnapshotID: snapshot.ID,
		TargetDir:  snapshot.TargetDir,
		Message:    "pre-existing generated extension restored from backup",
	}, nil
}

func (s *Store) ensureRollbackSnapshotScope(snapshot Snapshot) error {
	if !namePattern.MatchString(strings.TrimSpace(snapshot.Name)) {
		return fmt.Errorf("snapshot name is invalid")
	}
	expected, err := filepath.Abs(filepath.Join(s.GeneratedDir, snapshot.Name))
	if err != nil {
		return err
	}
	target, err := filepath.Abs(snapshot.TargetDir)
	if err != nil {
		return err
	}
	if filepath.Clean(expected) != filepath.Clean(target) {
		return fmt.Errorf("snapshot target does not match generated extension scope")
	}
	return nil
}

func ensureChildOf(root string, child string, outsideMessage string) error {
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
		return fmt.Errorf("%s", outsideMessage)
	}
	return ensureChildPathHasNoSymlink(rootAbs, rel)
}

func copyDirectoryStrict(source string, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("backup path contains symlink")
	}
	if !info.IsDir() {
		return fmt.Errorf("backup path is not a directory")
	}
	if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		src := filepath.Join(source, entry.Name())
		dst := filepath.Join(target, entry.Name())
		entryInfo, err := os.Lstat(src)
		if err != nil {
			return err
		}
		if entryInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup contains symlink: %s", entry.Name())
		}
		if entryInfo.IsDir() {
			if err := copyDirectoryStrict(src, dst); err != nil {
				return err
			}
			continue
		}
		if !entryInfo.Mode().IsRegular() {
			return fmt.Errorf("backup contains unsupported file type: %s", entry.Name())
		}
		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, entryInfo.Mode().Perm()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) testManifest(ctx context.Context, dir string, manifest Manifest, options TestOptions) (TestResult, error) {
	if len(manifest.Tests) == 0 {
		return TestResult{}, fmt.Errorf("extension manifest must declare at least one test command")
	}
	startedAt := s.timestamp()
	start := time.Now()
	result := TestResult{
		Name:      manifest.Name,
		Commands:  append([]string{}, manifest.Tests...),
		Status:    "running",
		StartedAt: startedAt,
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultExtensionTestTimeout
	}
	var combined strings.Builder
	for _, testCommand := range manifest.Tests {
		if err := validateTestCommand(testCommand); err != nil {
			result.Status = "failed"
			result.Output = redactExtensionText(err.Error())
			result.CompletedAt = s.timestamp()
			result.DurationMS = time.Since(start).Milliseconds()
			_ = s.recordTest(result)
			return result, err
		}
		parts := strings.Fields(testCommand)
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		cmd := exec.CommandContext(runCtx, parts[0], parts[1:]...)
		cmd.Dir = dir
		var stdout, stderr boundedBuffer
		stdout.limit = defaultMaxBytes
		stderr.limit = 65536
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()
		cancel()
		if stdout.String() != "" {
			combined.WriteString(stdout.String())
		}
		if stderr.String() != "" {
			if combined.Len() > 0 {
				combined.WriteString("\n")
			}
			combined.WriteString(stderr.String())
		}
		if runCtx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
			result.Output = redactExtensionText(strings.TrimSpace(combined.String()))
			result.CompletedAt = s.timestamp()
			result.DurationMS = time.Since(start).Milliseconds()
			_ = s.recordTest(result)
			return result, fmt.Errorf("extension tests timed out")
		}
		if stdout.truncated || stderr.truncated {
			result.Status = "failed"
			result.Output = redactExtensionText(strings.TrimSpace(combined.String()))
			result.CompletedAt = s.timestamp()
			result.DurationMS = time.Since(start).Milliseconds()
			_ = s.recordTest(result)
			return result, fmt.Errorf("extension test output exceeded limit")
		}
		if err != nil {
			result.Status = "failed"
			result.Output = redactExtensionText(strings.TrimSpace(combined.String()))
			result.CompletedAt = s.timestamp()
			result.DurationMS = time.Since(start).Milliseconds()
			_ = s.recordTest(result)
			return result, fmt.Errorf("extension tests failed: %w", err)
		}
	}
	result.Status = "passed"
	result.Output = redactExtensionText(strings.TrimSpace(combined.String()))
	result.CompletedAt = s.timestamp()
	result.DurationMS = time.Since(start).Milliseconds()
	_ = s.recordTest(result)
	return result, nil
}

func (s *Store) buildGoExtension(ctx context.Context, dir string, manifest Manifest, options TestOptions) error {
	command := strings.TrimSpace(manifest.Entrypoint.Command)
	if !strings.HasPrefix(command, "./") {
		return nil
	}
	output := filepath.Join(dir, strings.TrimPrefix(command, "./"))
	if err := ensureChild(dir, output); err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = defaultExtensionTestTimeout
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "go", "build", "-o", output, ".")
	cmd.Dir = dir
	var stdout, stderr boundedBuffer
	stdout.limit = defaultMaxBytes
	stderr.limit = 65536
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(strings.Join(nonEmpty(stdout.String(), stderr.String()), "\n"))
		if runCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("extension build timed out")
		}
		if msg != "" {
			return fmt.Errorf("extension build failed: %w: %s", err, msg)
		}
		return fmt.Errorf("extension build failed: %w", err)
	}
	return nil
}

func validateTestCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return fmt.Errorf("test command cannot be empty")
	}
	if strings.ContainsAny(command, "|;&`<>") || strings.Contains(command, "$(") {
		return fmt.Errorf("test command contains blocked shell syntax")
	}
	parts := strings.Fields(command)
	if len(parts) < 2 || parts[0] != "go" || parts[1] != "test" {
		return fmt.Errorf("only go test commands are allowed for generated extension tests in this milestone")
	}
	for _, part := range parts[2:] {
		if strings.HasPrefix(part, "-") {
			switch {
			case strings.HasPrefix(part, "-run"), strings.HasPrefix(part, "-count"), strings.HasPrefix(part, "-timeout"):
			default:
				return fmt.Errorf("go test option is not allowed in generated extension tests: %s", part)
			}
			continue
		}
		switch part {
		case ".", "./...":
		default:
			return fmt.Errorf("go test package path is not allowed in generated extension tests: %s", part)
		}
	}
	return nil
}

func writeGeneratedPackage(dir string, name string, description string, maxRuntimeSeconds int) error {
	if maxRuntimeSeconds <= 0 {
		maxRuntimeSeconds = 30
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create generated extension package: %w", err)
	}
	manifest := generatedManifest(name, description, maxRuntimeSeconds)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode generated manifest: %w", err)
	}
	files := map[string]string{
		ManifestFile:     string(manifestData),
		"README.md":      generatedReadme(name, description),
		"go.mod":         generatedGoMod(name),
		"main.go":        generatedMainGo(name),
		"main_test.go":   generatedMainTestGo(name),
		".yemaka_origin": "generated_by=yemaka\nmilestone=4.3\n",
	}
	names := make([]string, 0, len(files))
	for filename := range files {
		names = append(names, filename)
	}
	sort.Strings(names)
	for _, filename := range names {
		path := filepath.Join(dir, filename)
		if err := ensureChild(dir, path); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(files[filename]), fileMode(filename)); err != nil {
			return fmt.Errorf("write generated extension file %s: %w", filename, err)
		}
	}
	return nil
}

func writeGeneratedBrokeredNetworkPackage(dir string, name string, description string, maxRuntimeSeconds int, allowedDomains []string) error {
	if maxRuntimeSeconds <= 0 {
		maxRuntimeSeconds = 30
	}
	domains := normalizeAllowedDomains(allowedDomains)
	if len(domains) == 0 {
		domains = inferAllowedDomains(description)
	}
	if len(domains) == 0 {
		return fmt.Errorf("brokered network extensions require at least one allowed domain")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create generated extension package: %w", err)
	}
	manifest := generatedBrokeredNetworkManifest(name, description, maxRuntimeSeconds, domains)
	manifestData, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode generated manifest: %w", err)
	}
	files := map[string]string{
		ManifestFile:     string(manifestData),
		"README.md":      generatedBrokeredNetworkReadme(name, description, domains),
		"go.mod":         generatedGoMod(name),
		"main.go":        generatedBrokeredNetworkMainGo(name),
		"main_test.go":   generatedBrokeredNetworkMainTestGo(name),
		".yemaka_origin": "generated_by=yemaka\nmilestone=4.14\nnetwork_mode=core_broker\n",
	}
	names := make([]string, 0, len(files))
	for filename := range files {
		names = append(names, filename)
	}
	sort.Strings(names)
	for _, filename := range names {
		path := filepath.Join(dir, filename)
		if err := ensureChild(dir, path); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(files[filename]), fileMode(filename)); err != nil {
			return fmt.Errorf("write generated extension file %s: %w", filename, err)
		}
	}
	return nil
}

func generatedManifest(name string, description string, maxRuntimeSeconds int) Manifest {
	return Manifest{
		Name:        name,
		Version:     "0.1.0",
		Description: description,
		Type:        TypeTool,
		Entrypoint:  Entrypoint{Type: "command", Command: "./" + name},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"task"},
			"properties": map[string]any{
				"task":    map[string]any{"type": "string"},
				"context": map[string]any{"type": "string"},
			},
		},
		OutputSchema: map[string]any{
			"type":     "object",
			"required": []string{"ok", "summary"},
			"properties": map[string]any{
				"ok":      map[string]any{"type": "boolean"},
				"summary": map[string]any{"type": "string"},
			},
		},
		Permissions: Permissions{
			Network:    NetworkPermission{Enabled: false},
			Filesystem: FilesystemPermission{Read: false, Write: false},
			Shell:      false,
			Secrets:    false,
			Memory:     false,
		},
		Safety: Safety{
			RequiresUserApproval: false,
			MaxRuntimeSeconds:    maxRuntimeSeconds,
			MaxResponseBytes:     100000,
		},
		Tests: []string{"go test ./..."},
		Metadata: map[string]string{
			"generated_by": "yemaka",
			"milestone":    "4.3",
		},
	}
}

func generatedBrokeredNetworkManifest(name string, description string, maxRuntimeSeconds int, allowedDomains []string) Manifest {
	return Manifest{
		Name:        name,
		Version:     "0.1.0",
		Description: description,
		Type:        TypeTool,
		Entrypoint:  Entrypoint{Type: "command", Command: "./" + name},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []string{"url"},
			"properties": map[string]any{
				"url":         map[string]any{"type": "string"},
				"task":        map[string]any{"type": "string"},
				"extractText": map[string]any{"type": "boolean"},
			},
		},
		OutputSchema: map[string]any{
			"type":     "object",
			"required": []string{"ok", "summary"},
			"properties": map[string]any{
				"ok":      map[string]any{"type": "boolean"},
				"summary": map[string]any{"type": "string"},
				"url":     map[string]any{"type": "string"},
			},
		},
		Permissions: Permissions{
			Network: NetworkPermission{
				Enabled:        true,
				AllowedMethods: []string{"GET", "HEAD"},
				AllowedDomains: append([]string{}, allowedDomains...),
			},
			Filesystem: FilesystemPermission{Read: false, Write: false},
			Shell:      false,
			Secrets:    false,
			Memory:     false,
		},
		Safety: Safety{
			RequiresUserApproval: true,
			MaxRuntimeSeconds:    maxRuntimeSeconds,
			MaxResponseBytes:     100000,
		},
		Tests: []string{"go test ./..."},
		Metadata: map[string]string{
			"generated_by":  "yemaka",
			"milestone":     "4.14",
			"network_mode":  "core_broker",
			"network_scope": "fetch",
		},
	}
}

func generatedGoMod(name string) string {
	return "module yemaka_generated_" + name + "\n\ngo 1.22\n"
}

func generatedReadme(name string, description string) string {
	return fmt.Sprintf(`# %s

%s

This package was generated locally by Yemaka. It is a safe starter extension:
no network access, no shell access, no secret access, and no filesystem writes.
Yemaka registers it only after the manifest validates and tests pass.
`, name, description)
}

func generatedBrokeredNetworkReadme(name string, description string, allowedDomains []string) string {
	return fmt.Sprintf(`# %s

%s

This package was generated locally by Yemaka as a core-brokered network tool.
The extension does not open sockets or import network libraries. It emits
Yemaka core_requests, and the trusted core internet service enforces methods,
domain allow-lists, timeouts, response limits, logging, and user approval.

Allowed domains:

%s
`, name, description, markdownDomainList(allowedDomains))
}

func generatedMainGo(name string) string {
	quotedName, _ := json.Marshal(name)
	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const protocolVersion = %q

type request struct {
	Protocol  string         `+"`json:\"protocol\"`"+`
	RunID     string         `+"`json:\"run_id\"`"+`
	Extension string         `+"`json:\"extension\"`"+`
	Version   string         `+"`json:\"version\"`"+`
	Input     map[string]any `+"`json:\"input\"`"+`
	Timestamp string         `+"`json:\"timestamp\"`"+`
}

type response struct {
	Protocol string         `+"`json:\"protocol\"`"+`
	OK       bool           `+"`json:\"ok\"`"+`
	Output   map[string]any `+"`json:\"output\"`"+`
	Error    string         `+"`json:\"error,omitempty\"`"+`
	Logs     string         `+"`json:\"logs,omitempty\"`"+`
}

func main() {
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		emit(false, nil, err.Error())
		os.Exit(1)
	}
	var req request
	if err := json.Unmarshal(payload, &req); err != nil {
		emit(false, nil, "invalid request: "+err.Error())
		os.Exit(1)
	}
	output, err := handle(req.Input)
	if err != nil {
		emit(false, nil, err.Error())
		os.Exit(1)
	}
	emit(true, output, "")
}

func handle(input map[string]any) (map[string]any, error) {
	task, _ := input["task"].(string)
	task = strings.TrimSpace(task)
	if task == "" {
		return nil, fmt.Errorf("task is required")
	}
	context, _ := input["context"].(string)
	summary := fmt.Sprintf("%%s handled task: %%s", %s, task)
	if strings.TrimSpace(context) != "" {
		summary = fmt.Sprintf("%%s Context bytes: %%d.", summary, len(context))
	}
	return map[string]any{"ok": true, "summary": summary}, nil
}

func emit(ok bool, output map[string]any, message string) {
	resp := response{Protocol: protocolVersion, OK: ok, Output: output}
	if !ok {
		resp.Error = message
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode response: %%v", err)
		return
	}
	fmt.Print(string(data))
}
`, ProtocolVersion, string(quotedName))
}

func generatedMainTestGo(name string) string {
	quotedName, _ := json.Marshal(name)
	return fmt.Sprintf(`package main

import (
	"strings"
	"testing"
)

func TestHandleRequiresTask(t *testing.T) {
	if _, err := handle(map[string]any{}); err == nil {
		t.Fatal("handle() error = nil, want task requirement")
	}
}

func TestHandleReturnsSummary(t *testing.T) {
	output, err := handle(map[string]any{"task": "summarize notes", "context": "abc"})
	if err != nil {
		t.Fatalf("handle() error = %%v", err)
	}
	if output["ok"] != true {
		t.Fatalf("ok = %%v, want true", output["ok"])
	}
	summary, _ := output["summary"].(string)
	if !strings.Contains(summary, %s) || !strings.Contains(summary, "summarize notes") {
		t.Fatalf("summary = %%q, want extension name and task", summary)
	}
}
`, string(quotedName))
}

func generatedBrokeredNetworkMainGo(name string) string {
	quotedName, _ := json.Marshal(name)
	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

const protocolVersion = %q

type request struct {
	Protocol    string         `+"`json:\"protocol\"`"+`
	RunID       string         `+"`json:\"run_id\"`"+`
	Extension   string         `+"`json:\"extension\"`"+`
	Version     string         `+"`json:\"version\"`"+`
	Input       map[string]any `+"`json:\"input\"`"+`
	CoreResults []coreResult   `+"`json:\"core_results,omitempty\"`"+`
	Timestamp   string         `+"`json:\"timestamp\"`"+`
}

type response struct {
	Protocol     string         `+"`json:\"protocol\"`"+`
	OK           bool           `+"`json:\"ok\"`"+`
	Output       map[string]any `+"`json:\"output,omitempty\"`"+`
	Error        string         `+"`json:\"error,omitempty\"`"+`
	Logs         string         `+"`json:\"logs,omitempty\"`"+`
	CoreRequests []coreRequest  `+"`json:\"core_requests,omitempty\"`"+`
}

type coreRequest struct {
	ID    string         `+"`json:\"id\"`"+`
	Tool  string         `+"`json:\"tool\"`"+`
	Input map[string]any `+"`json:\"input,omitempty\"`"+`
}

type coreResult struct {
	ID     string         `+"`json:\"id\"`"+`
	Tool   string         `+"`json:\"tool\"`"+`
	Status string         `+"`json:\"status\"`"+`
	Output map[string]any `+"`json:\"output,omitempty\"`"+`
	Error  string         `+"`json:\"error,omitempty\"`"+`
}

func main() {
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		emit(false, nil, nil, err.Error())
		os.Exit(1)
	}
	var req request
	if err := json.Unmarshal(payload, &req); err != nil {
		emit(false, nil, nil, "invalid request: "+err.Error())
		os.Exit(1)
	}
	if len(req.CoreResults) == 0 {
		coreRequests, err := buildCoreRequests(req.Input)
		if err != nil {
			emit(false, nil, nil, err.Error())
			os.Exit(1)
		}
		emit(true, nil, coreRequests, "")
		return
	}
	output, err := handleCoreResults(req.Input, req.CoreResults)
	if err != nil {
		emit(false, nil, nil, err.Error())
		os.Exit(1)
	}
	emit(true, output, nil, "")
}

func buildCoreRequests(input map[string]any) ([]coreRequest, error) {
	rawURL, _ := input["url"].(string)
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("url is required")
	}
	extractText, ok := input["extractText"].(bool)
	if !ok {
		extractText = true
	}
	return []coreRequest{{
		ID:   "fetch_page",
		Tool: "internet_fetch",
		Input: map[string]any{
			"url":         rawURL,
			"extractText": extractText,
		},
	}}, nil
}

func handleCoreResults(input map[string]any, results []coreResult) (map[string]any, error) {
	if len(results) == 0 {
		return nil, fmt.Errorf("core result is required")
	}
	result := results[0]
	if result.Status != "ok" {
		if strings.TrimSpace(result.Error) == "" {
			return nil, fmt.Errorf("core request failed")
		}
		return nil, fmt.Errorf("%%s", result.Error)
	}
	finalURL := stringValue(result.Output["finalUrl"])
	if finalURL == "" {
		finalURL = stringValue(result.Output["url"])
	}
	status := stringValue(result.Output["statusCode"])
	text := stringValue(result.Output["extractedText"])
	if text == "" {
		text = stringValue(result.Output["body"])
	}
	text = compactText(text, 1200)
	task := stringValue(input["task"])
	if task == "" {
		task = "fetch"
	}
	summary := fmt.Sprintf("%%s completed %%s for %%s", %s, task, finalURL)
	if status != "" {
		summary += " with status " + status
	}
	if text != "" {
		summary += ". Extracted text: " + text
	}
	return map[string]any{"ok": true, "summary": summary, "url": finalURL}, nil
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func compactText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit > 3 {
		return strings.TrimSpace(value[:limit-3]) + "..."
	}
	return value[:limit]
}

func emit(ok bool, output map[string]any, coreRequests []coreRequest, message string) {
	resp := response{Protocol: protocolVersion, OK: ok, Output: output, CoreRequests: coreRequests}
	if !ok {
		resp.Error = message
	}
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode response: %%v", err)
		return
	}
	fmt.Print(string(data))
}
`, ProtocolVersion, string(quotedName))
}

func generatedBrokeredNetworkMainTestGo(name string) string {
	quotedName, _ := json.Marshal(name)
	return fmt.Sprintf(`package main

import (
	"strings"
	"testing"
)

func TestBuildCoreRequestsRequiresURL(t *testing.T) {
	if _, err := buildCoreRequests(map[string]any{}); err == nil {
		t.Fatal("buildCoreRequests() error = nil, want url requirement")
	}
}

func TestBuildCoreRequestsUsesBrokerTool(t *testing.T) {
	requests, err := buildCoreRequests(map[string]any{"url": "https://example.com/page"})
	if err != nil {
		t.Fatalf("buildCoreRequests() error = %%v", err)
	}
	if len(requests) != 1 || requests[0].Tool != "internet_fetch" {
		t.Fatalf("requests = %%+v, want one internet_fetch", requests)
	}
}

func TestHandleCoreResultsSummarizesBrokerResult(t *testing.T) {
	output, err := handleCoreResults(map[string]any{"task": "check page"}, []coreResult{{
		ID:     "fetch_page",
		Tool:   "internet_fetch",
		Status: "ok",
		Output: map[string]any{
			"finalUrl":      "https://example.com/page",
			"statusCode":    200,
			"extractedText": "Hello from example.",
		},
	}})
	if err != nil {
		t.Fatalf("handleCoreResults() error = %%v", err)
	}
	summary, _ := output["summary"].(string)
	if output["ok"] != true || !strings.Contains(summary, %s) || !strings.Contains(summary, "Hello from example") {
		t.Fatalf("output = %%+v, want summary from brokered result", output)
	}
}
`, string(quotedName))
}

func fileMode(filename string) os.FileMode {
	if strings.HasSuffix(filename, ".sh") {
		return 0o755
	}
	return 0o644
}

func looksBrokeredNetworkRequest(request string) bool {
	lower := strings.ToLower(request)
	if len(inferAllowedDomains(request)) == 0 {
		return false
	}
	for _, token := range []string{
		"web", "website", "url", "page", "internet", "fetch", "head",
		"monitor", "status", "uptime", "downtime", "availability", "reachable",
		"monitor site", "check site", "scrape", "public",
	} {
		if strings.Contains(lower, token) {
			return true
		}
	}
	return strings.Contains(lower, "http://") || strings.Contains(lower, "https://")
}

func inferAllowedDomains(text string) []string {
	raw := []string{}
	urlPattern := regexp.MustCompile(`https?://[^\s<>"']+`)
	for _, match := range urlPattern.FindAllString(text, -1) {
		raw = append(raw, match)
	}
	barePattern := regexp.MustCompile(`(?i)\b[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+\b`)
	for _, indexes := range barePattern.FindAllStringIndex(text, -1) {
		if len(indexes) != 2 || bareDomainMatchLooksEmail(text, indexes[0]) {
			continue
		}
		match := text[indexes[0]:indexes[1]]
		if looksPublicDomain(match) {
			raw = append(raw, match)
		}
	}
	return normalizeAllowedDomains(raw)
}

func NormalizeAllowedDomains(values []string) []string {
	return normalizeAllowedDomains(values)
}

func bareDomainMatchLooksEmail(text string, start int) bool {
	if start <= 0 || start > len(text) {
		return false
	}
	return text[start-1] == '@'
}

func normalizeAllowedDomains(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		domain := normalizeAllowedDomain(value)
		if domain == "" || seen[domain] {
			continue
		}
		seen[domain] = true
		result = append(result, domain)
	}
	sort.Strings(result)
	return result
}

func normalizeAllowedDomain(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.Trim(value, ".,;:()[]{}<>\"'")
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		withoutScheme := value
		if index := strings.Index(withoutScheme, "://"); index >= 0 {
			withoutScheme = withoutScheme[index+3:]
		}
		if slash := strings.IndexByte(withoutScheme, '/'); slash >= 0 {
			withoutScheme = withoutScheme[:slash]
		}
		value = withoutScheme
	}
	if at := strings.LastIndexByte(value, '@'); at >= 0 {
		value = value[at+1:]
	}
	if colon := strings.IndexByte(value, ':'); colon >= 0 {
		value = value[:colon]
	}
	value = strings.TrimSuffix(value, ".")
	if value == "localhost" || strings.HasPrefix(value, "127.") || strings.HasPrefix(value, "10.") || strings.HasPrefix(value, "192.168.") {
		return ""
	}
	if !looksPublicDomain(value) {
		return ""
	}
	return value
}

func looksPublicDomain(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || strings.ContainsAny(value, "/:_") {
		return false
	}
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return false
	}
	tld := parts[len(parts)-1]
	if len(tld) < 2 || len(tld) > 24 {
		return false
	}
	common := map[string]bool{
		"com": true, "org": true, "net": true, "io": true, "dev": true, "ai": true,
		"app": true, "co": true, "edu": true, "gov": true, "site": true, "cloud": true,
		"me": true, "info": true, "biz": true, "us": true, "uk": true, "ca": true,
	}
	if !common[tld] && len(tld) <= 3 {
		return false
	}
	for _, part := range parts {
		if part == "" || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return false
		}
		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

func markdownDomainList(domains []string) string {
	if len(domains) == 0 {
		return "- none\n"
	}
	var builder strings.Builder
	for _, domain := range domains {
		builder.WriteString("- ")
		builder.WriteString(domain)
		builder.WriteByte('\n')
	}
	return builder.String()
}

func generatedPackageFiles(name string) []string {
	return []string{
		filepath.Join(name, ManifestFile),
		filepath.Join(name, "README.md"),
		filepath.Join(name, "go.mod"),
		filepath.Join(name, "main.go"),
		filepath.Join(name, "main_test.go"),
	}
}

func safeExtensionName(request string) string {
	value := strings.ToLower(strings.TrimSpace(extensionNameSubject(request)))
	value = strings.ReplaceAll(value, "-", "_")
	value = slugPattern.ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if value == "" {
		value = "generated_extension"
	}
	if value[0] < 'a' || value[0] > 'z' {
		value = "generated_" + value
	}
	if len(value) > 48 {
		value = strings.Trim(value[:48], "_")
	}
	if !namePattern.MatchString(value) {
		value = "generated_extension"
	}
	return value
}

func extensionNameSubject(request string) string {
	clean := strings.TrimSpace(strings.Join(strings.Fields(request), " "))
	if clean == "" {
		return clean
	}
	lower := strings.ToLower(clean)
	for _, phrase := range []string{
		"generate or create extension for ",
		"create or generate extension for ",
		"generate an extension for ",
		"generate extension for ",
		"create an extension for ",
		"create extension for ",
		"build an extension for ",
		"build extension for ",
		"generate a tool for ",
		"create a tool for ",
		"build a tool for ",
		"generate a capability for ",
		"create a capability for ",
		"build a capability for ",
	} {
		if idx := strings.Index(lower, phrase); idx >= 0 {
			subject := strings.TrimSpace(clean[idx+len(phrase):])
			if subject != "" {
				return trimExtensionSubjectTail(subject)
			}
		}
	}
	for _, prefix := range []string{"okay ", "ok ", "please ", "first "} {
		if strings.HasPrefix(lower, prefix) {
			return extensionNameSubject(clean[len(prefix):])
		}
	}
	return clean
}

func trimExtensionSubjectTail(subject string) string {
	subject = strings.TrimSpace(subject)
	lower := strings.ToLower(subject)
	for _, marker := range []string{
		" and then ",
		" then ",
		" after ",
		" so that ",
		" with approval",
		" until i approve",
	} {
		if idx := strings.Index(lower, marker); idx > 0 {
			subject = strings.TrimSpace(subject[:idx])
			lower = strings.ToLower(subject)
		}
	}
	return subject
}

func compactDescription(request string) string {
	request = strings.TrimSpace(strings.Join(strings.Fields(request), " "))
	if len(request) > 180 {
		request = strings.TrimSpace(request[:180])
	}
	if request == "" {
		return "Generated local Yemaka extension."
	}
	return request
}

func skillCandidateText(name string, description string) string {
	return fmt.Sprintf("Extension %s generated and tested for: %s. Consider saving repeated usage as a Yemaka skill after a successful session.", name, description)
}

func (s *Store) saveSnapshot(snapshot Snapshot) error {
	path := s.snapshotPath(snapshot.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create extension snapshot directory: %w", err)
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode extension snapshot: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write extension snapshot: %w", err)
	}
	return nil
}

func (s *Store) findSnapshot(query string) (Snapshot, error) {
	query = strings.TrimSpace(query)
	paths, err := filepath.Glob(filepath.Join(s.snapshotDir(), "*.json"))
	if err != nil {
		return Snapshot{}, err
	}
	sort.Slice(paths, func(i int, j int) bool {
		infoI, errI := os.Stat(paths[i])
		infoJ, errJ := os.Stat(paths[j])
		if errI != nil || errJ != nil {
			return paths[i] > paths[j]
		}
		return infoI.ModTime().After(infoJ.ModTime())
	})
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var snapshot Snapshot
		if err := json.Unmarshal(data, &snapshot); err != nil {
			continue
		}
		if snapshot.ID == query || snapshot.Name == query {
			return snapshot, nil
		}
	}
	return Snapshot{}, fmt.Errorf("extension generation snapshot not found: %s", query)
}

func (s *Store) snapshotPath(id string) string {
	return filepath.Join(s.snapshotDir(), id+".json")
}

func (s *Store) snapshotDir() string {
	return filepath.Join(filepath.Dir(s.GeneratedDir), snapshotDirName)
}

func (s *Store) removeRegistryEntry(name string) error {
	registry, err := s.load()
	if err != nil {
		return err
	}
	if registry.Entries == nil {
		registry.Entries = map[string]Entry{}
	}
	delete(registry.Entries, name)
	return s.save(registry)
}

func (s *Store) markRegistered(name string) error {
	registry, err := s.discover()
	if err != nil {
		return err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return fmt.Errorf("extension not found: %s", name)
	}
	if !entry.Valid {
		return fmt.Errorf("invalid extension cannot be registered: %s", entry.ValidationError)
	}
	fingerprint, err := fingerprintPackageDir(entry.Dir)
	if err != nil {
		return err
	}
	entry.Registered = true
	entry.PackageFingerprint = fingerprint
	entry.RegisteredAt = s.timestamp()
	entry.UpdatedAt = entry.RegisteredAt
	registry.Entries[name] = s.entryFromDir(entry.Dir, entry)
	return s.save(registry)
}

func (s *Store) disableIfPresent(name string) error {
	registry, err := s.discover()
	if err != nil {
		return err
	}
	entry, ok := registry.Entries[name]
	if !ok {
		return nil
	}
	entry.Enabled = false
	entry.UpdatedAt = s.timestamp()
	registry.Entries[name] = entry
	return s.save(registry)
}

func (s *Store) recordGeneration(result GenerationResult) error {
	result = redactedGenerationResult(result)
	path := filepath.Join(filepath.Dir(s.AuditPath), generationsFile)
	if strings.TrimSpace(s.AuditPath) == "" {
		path = filepath.Join(filepath.Dir(s.GeneratedDir), generationsFile)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func (s *Store) recordTest(result TestResult) error {
	result = redactedTestResult(result)
	path := filepath.Join(filepath.Dir(s.AuditPath), testsFile)
	if strings.TrimSpace(s.AuditPath) == "" {
		path = filepath.Join(filepath.Dir(s.GeneratedDir), testsFile)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}
