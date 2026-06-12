package domainpacks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"yemaka/internal/skills"
)

const PackSkillsDir = "skills"

// WorkflowSkillPackInput describes a local pack made from compiled workflow
// skills. Installing the pack still leaves it disabled until explicit enable.
type WorkflowSkillPackInput struct {
	PackDir      string
	Name         string
	Version      string
	Description  string
	Category     string
	Workflows    []skills.WorkflowCompileInput
	DefaultState string
}

// WorkflowSkillPack is the generated pack manifest plus written skill outputs.
type WorkflowSkillPack struct {
	Manifest Manifest
	Skills   []skills.CompiledWorkflowSkill
}

// WriteWorkflowSkillPack writes a lightweight domain/capability pack containing
// compiled workflow skills and a validating pack manifest.
func WriteWorkflowSkillPack(input WorkflowSkillPackInput) (WorkflowSkillPack, error) {
	packDir := strings.TrimSpace(input.PackDir)
	if packDir == "" {
		return WorkflowSkillPack{}, fmt.Errorf("pack directory is required")
	}
	if len(input.Workflows) == 0 {
		return WorkflowSkillPack{}, fmt.Errorf("at least one workflow skill is required")
	}

	compiled := make([]skills.CompiledWorkflowSkill, 0, len(input.Workflows))
	seen := map[string]struct{}{}
	for _, workflow := range input.Workflows {
		workflow.ProfileSkillsDir = ""
		item, err := skills.CompileWorkflow(workflow)
		if err != nil {
			return WorkflowSkillPack{}, err
		}
		if _, exists := seen[item.Skill.Name]; exists {
			return WorkflowSkillPack{}, fmt.Errorf("duplicate workflow skill in pack: %s", item.Skill.Name)
		}
		seen[item.Skill.Name] = struct{}{}
		compiled = append(compiled, item)
	}

	manifest := workflowSkillPackManifest(input, compiled)
	if err := Validate(manifest); err != nil {
		return WorkflowSkillPack{}, err
	}
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return WorkflowSkillPack{}, fmt.Errorf("create workflow skill pack directory: %w", err)
	}
	if err := writeManifestFile(packDir, manifest); err != nil {
		return WorkflowSkillPack{}, err
	}
	for i, item := range compiled {
		target := filepath.Join(packDir, PackSkillsDir, item.Skill.Name)
		written, err := skills.WriteCompiledWorkflowPackage(target, item)
		if err != nil {
			return WorkflowSkillPack{}, err
		}
		compiled[i] = written
	}
	loaded, err := Load(packDir)
	if err != nil {
		return WorkflowSkillPack{}, err
	}
	return WorkflowSkillPack{Manifest: loaded, Skills: compiled}, nil
}

// EnabledSkillDirs returns skill registry roots for installed, valid, explicitly
// enabled domain packs only.
func EnabledSkillDirs(profilePackRoot string) ([]string, error) {
	statuses, err := List(profilePackRoot)
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(statuses))
	for _, status := range statuses {
		if !status.Enabled || !status.Valid {
			continue
		}
		skillDir := filepath.Join(status.Dir, PackSkillsDir)
		info, err := os.Stat(skillDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("check domain pack skills for %s: %w", status.Name, err)
		}
		if info.IsDir() {
			dirs = append(dirs, skillDir)
		}
	}
	return dirs, nil
}

func workflowSkillPackManifest(input WorkflowSkillPackInput, compiled []skills.CompiledWorkflowSkill) Manifest {
	manifest := Manifest{
		Name:         strings.TrimSpace(input.Name),
		Version:      strings.TrimSpace(input.Version),
		Description:  strings.TrimSpace(input.Description),
		Category:     strings.TrimSpace(input.Category),
		DefaultState: normalizeDefaultState(input.DefaultState),
	}
	if manifest.Version == "" {
		manifest.Version = "0.1.0"
	}
	if manifest.Category == "" {
		manifest.Category = "capability"
	}
	if manifest.Description == "" && len(compiled) == 1 {
		manifest.Description = compiled[0].Skill.Description
	}

	for _, item := range compiled {
		skillRef := filepath.ToSlash(filepath.Join(PackSkillsDir, item.Skill.Name))
		manifest.Skills = append(manifest.Skills, skillRef)
		manifest.RequiredTools = append(manifest.RequiredTools, item.Skill.RequiredTools...)
		if len(item.Tests) > 0 {
			manifest.Tests = append(manifest.Tests, filepath.ToSlash(filepath.Join(skillRef, "tests", "workflow.yaml")))
		}
		manifest.Permissions = mergeSkillManifestPermissions(manifest.Permissions, item.Skill.Permissions)
	}
	return Normalize(manifest)
}

func mergeSkillManifestPermissions(base Permissions, skillPermissions skills.Permissions) Permissions {
	if skillPermissions.FilesystemRead {
		base.Filesystem.Scopes = append(base.Filesystem.Scopes, "read")
		base.Filesystem.Paths = append(base.Filesystem.Paths, "workspace://")
	}
	if skillPermissions.FilesystemWrite != "" {
		base.Filesystem.Required = true
		base.Filesystem.Scopes = append(base.Filesystem.Scopes, "write")
		base.Filesystem.Paths = append(base.Filesystem.Paths, "workspace://")
	}
	if skillPermissions.Network {
		base.Internet.Required = true
		base.Internet.AllowedMethods = append(base.Internet.AllowedMethods, "GET", "HEAD")
	}
	return base
}

func writeManifestFile(dir string, manifest Manifest) error {
	data, err := yaml.Marshal(Normalize(manifest))
	if err != nil {
		return fmt.Errorf("encode pack manifest: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), data, 0o644); err != nil {
		return fmt.Errorf("write pack manifest: %w", err)
	}
	return nil
}
