package skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func SetEnabled(profileSkillsDir string, skill Skill, enabled bool) (Skill, error) {
	if strings.TrimSpace(profileSkillsDir) == "" {
		return Skill{}, fmt.Errorf("profile skills directory is required")
	}
	if err := Validate(skill); err != nil {
		return Skill{}, err
	}
	target := filepath.Join(profileSkillsDir, skill.Name)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return Skill{}, fmt.Errorf("create skill override directory: %w", err)
	}
	next := skill
	next.Disabled = !enabled
	next.Dir = target
	next.Source = "profile"
	if err := writeSkillPackage(target, next, skill.Instructions); err != nil {
		return Skill{}, err
	}
	loaded, err := Load(target)
	if err != nil {
		return Skill{}, err
	}
	loaded.Source = "profile"
	return loaded, nil
}

func Import(profileSkillsDir string, sourceDir string) (Skill, error) {
	if strings.TrimSpace(profileSkillsDir) == "" {
		return Skill{}, fmt.Errorf("profile skills directory is required")
	}
	sourceDir = strings.TrimSpace(sourceDir)
	if sourceDir == "" {
		return Skill{}, fmt.Errorf("source skill directory is required")
	}
	if looksRemote(sourceDir) {
		return Skill{}, fmt.Errorf("remote skill imports are not supported")
	}
	skill, err := Load(sourceDir)
	if err != nil {
		return Skill{}, err
	}
	target := filepath.Join(profileSkillsDir, skill.Name)
	if _, err := os.Stat(target); err == nil {
		return Skill{}, fmt.Errorf("skill already exists in profile: %s", skill.Name)
	} else if !os.IsNotExist(err) {
		return Skill{}, fmt.Errorf("check target skill directory: %w", err)
	}
	if err := copyDir(sourceDir, target); err != nil {
		return Skill{}, err
	}
	imported, err := Load(target)
	if err != nil {
		return Skill{}, err
	}
	imported.Source = "profile"
	return imported, nil
}

func Export(skill Skill, destinationParent string) (string, error) {
	if err := Validate(skill); err != nil {
		return "", err
	}
	destinationParent = strings.TrimSpace(destinationParent)
	if destinationParent == "" {
		return "", fmt.Errorf("destination directory is required")
	}
	target := filepath.Join(destinationParent, skill.Name)
	if _, err := os.Stat(target); err == nil {
		return "", fmt.Errorf("export target already exists: %s", target)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("check export target: %w", err)
	}
	if err := copyDir(skill.Dir, target); err != nil {
		return "", err
	}
	return target, nil
}

func writeSkillPackage(dir string, skill Skill, instructions string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create skill directory: %w", err)
	}
	meta := skill
	meta.Instructions = ""
	meta.Dir = ""
	meta.Source = ""
	data, err := yaml.Marshal(meta)
	if err != nil {
		return fmt.Errorf("encode skill.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), data, 0o644); err != nil {
		return fmt.Errorf("write skill.yaml: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "instructions.md"), []byte(strings.TrimSpace(instructions)+"\n"), 0o644); err != nil {
		return fmt.Errorf("write instructions.md: %w", err)
	}
	return nil
}

func copyDir(source string, target string) error {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("stat source directory: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", source)
	}
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return os.MkdirAll(target, 0o755)
		}
		dst := filepath.Join(target, rel)
		if entry.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeType != 0 {
			return nil
		}
		return copyFile(path, dst, info.Mode().Perm())
	})
}

func copyFile(source string, target string, perm os.FileMode) error {
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
