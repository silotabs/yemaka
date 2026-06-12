package domainpacks

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const StateFile = ".yemaka-pack-state.yaml"

type PackState struct {
	Enabled bool `yaml:"enabled" json:"enabled"`
}

type PackStatus struct {
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Description         string   `json:"description,omitempty"`
	Category            string   `json:"category,omitempty"`
	Enabled             bool     `json:"enabled"`
	Installed           bool     `json:"installed"`
	Valid               bool     `json:"valid"`
	ValidationError     string   `json:"validationError,omitempty"`
	RepairHint          string   `json:"repairHint,omitempty"`
	Skills              []string `json:"skills,omitempty"`
	RequiredTools       []string `json:"requiredTools,omitempty"`
	OptionalTools       []string `json:"optionalTools,omitempty"`
	ApprovalRequiredFor []string `json:"approvalRequiredFor,omitempty"`
	BlockedActions      []string `json:"blockedActions,omitempty"`
	SensitiveDataRules  []string `json:"sensitiveDataRules,omitempty"`
	SafetySummary       string   `json:"safetySummary,omitempty"`
	Dir                 string   `json:"dir"`
}

func Install(profilePackRoot string, sourceDir string) (PackStatus, error) {
	profilePackRoot = strings.TrimSpace(profilePackRoot)
	if profilePackRoot == "" {
		return PackStatus{}, fmt.Errorf("profile pack root is required")
	}
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return PackStatus{}, fmt.Errorf("source pack directory is required")
	}
	if looksRemote(sourceDir) {
		return PackStatus{}, fmt.Errorf("remote pack installs are not supported")
	}
	manifest, err := Load(sourceDir)
	if err != nil {
		return PackStatus{}, err
	}
	target, err := packDir(profilePackRoot, manifest.Name)
	if err != nil {
		return PackStatus{}, err
	}
	if _, err := os.Stat(target); err == nil {
		return PackStatus{}, fmt.Errorf("domain pack already installed: %s", manifest.Name)
	} else if !os.IsNotExist(err) {
		return PackStatus{}, fmt.Errorf("check installed domain pack: %w", err)
	}
	if err := copyPackDir(sourceDir, target); err != nil {
		return PackStatus{}, err
	}
	if err := writeState(target, PackState{Enabled: false}); err != nil {
		return PackStatus{}, err
	}
	return statusForDir(target)
}

func SetEnabled(profilePackRoot string, name string, enabled bool) (PackStatus, error) {
	target, err := packDir(profilePackRoot, name)
	if err != nil {
		return PackStatus{}, err
	}
	if _, err := Load(target); err != nil {
		return PackStatus{}, err
	}
	if err := writeState(target, PackState{Enabled: enabled}); err != nil {
		return PackStatus{}, err
	}
	return statusForDir(target)
}

func Uninstall(profilePackRoot string, name string) (PackStatus, error) {
	target, err := packDir(profilePackRoot, name)
	if err != nil {
		return PackStatus{}, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return PackStatus{}, fmt.Errorf("domain pack is not installed: %s", strings.TrimSpace(name))
		}
		return PackStatus{}, fmt.Errorf("check installed domain pack: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return PackStatus{}, fmt.Errorf("domain pack path is a symlink and cannot be uninstalled safely: %s", strings.TrimSpace(name))
	}
	if !info.IsDir() {
		return PackStatus{}, fmt.Errorf("domain pack path is not a directory: %s", strings.TrimSpace(name))
	}
	status, err := statusForDir(target)
	if err != nil {
		status = PackStatus{
			Name:            filepath.Base(target),
			Enabled:         false,
			Installed:       true,
			Valid:           false,
			ValidationError: err.Error(),
			RepairHint:      repairHintForError(err),
			Dir:             target,
		}
	}
	if err := os.RemoveAll(target); err != nil {
		return PackStatus{}, fmt.Errorf("uninstall domain pack: %w", err)
	}
	status.Enabled = false
	status.Installed = false
	status.Dir = target
	return status, nil
}

func List(profilePackRoot string) ([]PackStatus, error) {
	profilePackRoot = strings.TrimSpace(profilePackRoot)
	if profilePackRoot == "" {
		return nil, fmt.Errorf("profile pack root is required")
	}
	entries, err := os.ReadDir(profilePackRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return []PackStatus{}, nil
		}
		return nil, fmt.Errorf("read profile pack root %s: %w", profilePackRoot, err)
	}
	statuses := make([]PackStatus, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		status, err := statusForDir(filepath.Join(profilePackRoot, entry.Name()))
		if err != nil {
			status = PackStatus{
				Name:            entry.Name(),
				Enabled:         false,
				Installed:       true,
				Valid:           false,
				ValidationError: err.Error(),
				RepairHint:      repairHintForError(err),
				Dir:             filepath.Join(profilePackRoot, entry.Name()),
			}
		}
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})
	return statuses, nil
}

func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("pack name is required")
	}
	if !packNamePattern.MatchString(name) {
		return fmt.Errorf("pack name must match %s", packNamePattern.String())
	}
	return nil
}

func statusForDir(dir string) (PackStatus, error) {
	manifest, err := Load(dir)
	if err != nil {
		return PackStatus{}, err
	}
	state, err := readState(dir)
	if err != nil {
		return PackStatus{}, err
	}
	return PackStatus{
		Name:                manifest.Name,
		Version:             manifest.Version,
		Description:         manifest.Description,
		Category:            manifest.Category,
		Enabled:             state.Enabled,
		Installed:           true,
		Valid:               true,
		Skills:              append([]string{}, manifest.Skills...),
		RequiredTools:       append([]string{}, manifest.RequiredTools...),
		OptionalTools:       append([]string{}, manifest.OptionalTools...),
		ApprovalRequiredFor: append([]string{}, manifest.Safety.ApprovalRequiredFor...),
		BlockedActions:      append([]string{}, manifest.Safety.BlockedActions...),
		SensitiveDataRules:  append([]string{}, manifest.Safety.SensitiveDataRules...),
		SafetySummary:       packSafetySummary(manifest),
		Dir:                 dir,
	}, nil
}

func packSafetySummary(manifest Manifest) string {
	approval := strings.TrimSpace(strings.Join(manifest.Safety.ApprovalRequiredFor, ", "))
	switch {
	case approval != "" && len(manifest.Safety.BlockedActions) > 0:
		return "Approval required for " + approval + "; blocked actions cannot run through this pack."
	case approval != "":
		return "Approval required for " + approval + "."
	case len(manifest.Safety.BlockedActions) > 0:
		return "Blocked actions cannot run through this pack."
	default:
		return "Pack remains subject to core policy and explicit enablement."
	}
}

func repairHintForError(err error) string {
	message := strings.ToLower(strings.TrimSpace(fmt.Sprint(err)))
	switch {
	case strings.Contains(message, "pack.yaml") && (strings.Contains(message, "no such file") || strings.Contains(message, "not exist")):
		return "Add a valid pack.yaml manifest, then refresh the pack list."
	case strings.Contains(message, "parse") || strings.Contains(message, "field not found"):
		return "Fix the pack.yaml syntax or unknown fields, then refresh the pack list."
	case strings.Contains(message, "pack name") || strings.Contains(message, "version") || strings.Contains(message, "default_state"):
		return "Fix the required manifest fields in pack.yaml, then refresh the pack list."
	case strings.Contains(message, "permission") || strings.Contains(message, "policy") || strings.Contains(message, "tool"):
		return "Fix the manifest permissions, safety metadata, or declared tools before enabling this pack."
	default:
		return "Repair the pack manifest and referenced skill folders, then refresh. Invalid packs cannot be enabled."
	}
}

func packDir(root string, name string) (string, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return "", fmt.Errorf("profile pack root is required")
	}
	if err := ValidateName(name); err != nil {
		return "", err
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve profile pack root: %w", err)
	}
	target := filepath.Join(cleanRoot, strings.TrimSpace(name))
	rel, err := filepath.Rel(cleanRoot, target)
	if err != nil {
		return "", fmt.Errorf("resolve pack path: %w", err)
	}
	if rel == "." || unsafeRelativePath(rel) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("pack path escapes profile pack root")
	}
	return target, nil
}

func readState(dir string) (PackState, error) {
	data, err := os.ReadFile(filepath.Join(dir, StateFile))
	if err != nil {
		if os.IsNotExist(err) {
			return PackState{Enabled: false}, nil
		}
		return PackState{}, fmt.Errorf("read pack state: %w", err)
	}
	var state PackState
	if err := yaml.Unmarshal(data, &state); err != nil {
		return PackState{}, fmt.Errorf("parse pack state: %w", err)
	}
	return state, nil
}

func writeState(dir string, state PackState) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create pack directory: %w", err)
	}
	data, err := yaml.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode pack state: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, StateFile), data, 0o644); err != nil {
		return fmt.Errorf("write pack state: %w", err)
	}
	return nil
}

func copyPackDir(source string, target string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source pack directory: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source pack is not a directory: %s", source)
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel != "." && unsafeRelativePath(rel) {
			return fmt.Errorf("pack file path escapes source: %s", rel)
		}
		dst := filepath.Join(target, rel)
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("pack install does not allow symlinks: %s", rel)
		}
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if info.Mode()&os.ModeType != 0 {
			return nil
		}
		return copyPackFile(path, dst, info.Mode().Perm())
	})
}

func copyPackFile(source string, target string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func looksRemote(path string) bool {
	lower := strings.ToLower(strings.TrimSpace(path))
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "git@")
}
