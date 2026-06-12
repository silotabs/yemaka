package extensions

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const (
	maxPackageFiles = 128
	maxPackageBytes = 128 * 1024 * 1024
	maxFileBytes    = 64 * 1024 * 1024
)

var blockedPackageDirs = map[string]bool{
	".git":         true,
	".hg":          true,
	".svn":         true,
	".venv":        true,
	"__pycache__":  true,
	"dist":         true,
	"node_modules": true,
	"vendor":       true,
}

var allowedHiddenFiles = map[string]bool{
	".yemaka_origin": true,
}

type PackageInspection struct {
	Name           string        `json:"name"`
	Dir            string        `json:"dir"`
	Status         string        `json:"status"`
	FileCount      int           `json:"fileCount"`
	TotalBytes     int64         `json:"totalBytes"`
	EntrypointPath string        `json:"entrypointPath,omitempty"`
	Files          []PackageFile `json:"files"`
	Errors         []string      `json:"errors,omitempty"`
	Warnings       []string      `json:"warnings,omitempty"`
	CheckedAt      string        `json:"checkedAt"`
}

type PackageFile struct {
	Path       string `json:"path"`
	Size       int64  `json:"size"`
	Executable bool   `json:"executable"`
}

type packageInspectionOptions struct {
	RequireEntrypoint bool
}

func (s *Store) InspectPackage(name string) (PackageInspection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return PackageInspection{}, fmt.Errorf("extension name is required")
	}
	detail, err := s.Show(name)
	if err != nil {
		return PackageInspection{}, err
	}
	inspection, err := s.inspectPackageDir(detail.Status.Dir, detail.Manifest, packageInspectionOptions{RequireEntrypoint: true})
	if err != nil {
		return inspection, err
	}
	return inspection, nil
}

func (s *Store) inspectPackageDir(dir string, manifest Manifest, options packageInspectionOptions) (PackageInspection, error) {
	inspection := PackageInspection{
		Name:      manifest.Name,
		Dir:       dir,
		Status:    "passed",
		CheckedAt: s.timestamp(),
	}
	entrypointPath, hasEntrypointPath, err := manifestEntrypointPath(manifest)
	if err != nil {
		inspection.Errors = append(inspection.Errors, err.Error())
		return finishInspection(inspection)
	}
	if hasEntrypointPath {
		inspection.EntrypointPath = entrypointPath
	}

	entrypointFound := !hasEntrypointPath
	entrypointExecutable := !hasEntrypointPath
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			inspection.Errors = append(inspection.Errors, walkErr.Error())
			return nil
		}
		if path == dir {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			inspection.Errors = append(inspection.Errors, err.Error())
			return nil
		}
		rel = filepath.ToSlash(rel)
		if unsafePackagePath(rel) {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("unsafe extension package path: %s", rel))
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("symlink is not allowed in generated extension packages: %s", rel))
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		base := d.Name()
		if d.IsDir() {
			if blockedPackageDirs[base] {
				inspection.Errors = append(inspection.Errors, fmt.Sprintf("blocked generated extension directory: %s", rel))
				return filepath.SkipDir
			}
			if strings.HasPrefix(base, ".") {
				inspection.Errors = append(inspection.Errors, fmt.Sprintf("hidden directory is not allowed in generated extension package: %s", rel))
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("inspect extension file %s: %v", rel, err))
			return nil
		}
		if info.Size() > maxFileBytes {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("extension file exceeds %d bytes: %s", maxFileBytes, rel))
		}
		executable := info.Mode()&0o111 != 0
		if rel == entrypointPath {
			entrypointFound = true
			entrypointExecutable = executable
		}
		if executable && hasEntrypointPath && rel != entrypointPath {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("unexpected executable file in generated extension package: %s", rel))
		}
		if strings.HasPrefix(base, ".") && !allowedHiddenFiles[base] {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("hidden file is not allowed in generated extension package: %s", rel))
		}
		if shouldScanForDirectNetwork(rel) {
			if err := scanForDirectNetwork(path); err != nil {
				inspection.Errors = append(inspection.Errors, fmt.Sprintf("%s: %v", rel, err))
			}
		}
		if shouldScanForDirectFileWrite(rel) {
			if err := scanForDirectFileWrite(path); err != nil {
				inspection.Errors = append(inspection.Errors, fmt.Sprintf("%s: %v", rel, err))
			}
		}
		inspection.FileCount++
		inspection.TotalBytes += info.Size()
		inspection.Files = append(inspection.Files, PackageFile{
			Path:       rel,
			Size:       info.Size(),
			Executable: executable,
		})
		return nil
	})
	if walkErr != nil {
		inspection.Errors = append(inspection.Errors, walkErr.Error())
	}
	if inspection.FileCount > maxPackageFiles {
		inspection.Errors = append(inspection.Errors, fmt.Sprintf("extension package has too many files: %d > %d", inspection.FileCount, maxPackageFiles))
	}
	if inspection.TotalBytes > maxPackageBytes {
		inspection.Errors = append(inspection.Errors, fmt.Sprintf("extension package exceeds %d bytes", maxPackageBytes))
	}
	if options.RequireEntrypoint && hasEntrypointPath {
		if !entrypointFound {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("entrypoint command not found in extension package: %s", entrypointPath))
		} else if !entrypointExecutable {
			inspection.Errors = append(inspection.Errors, fmt.Sprintf("entrypoint command is not executable: %s", entrypointPath))
		}
	}
	slices.SortFunc(inspection.Files, func(a PackageFile, b PackageFile) int {
		return strings.Compare(a.Path, b.Path)
	})
	slices.Sort(inspection.Errors)
	slices.Sort(inspection.Warnings)
	return finishInspection(inspection)
}

func finishInspection(inspection PackageInspection) (PackageInspection, error) {
	if len(inspection.Errors) > 0 {
		inspection.Status = "failed"
		return inspection, errors.New(strings.Join(inspection.Errors, "; "))
	}
	return inspection, nil
}

func manifestEntrypointPath(manifest Manifest) (string, bool, error) {
	command := strings.TrimSpace(manifest.Entrypoint.Command)
	if command == "" {
		return "", false, fmt.Errorf("entrypoint.command is required")
	}
	if strings.Contains(command, string(os.PathSeparator)) || strings.HasPrefix(command, ".") {
		if filepath.IsAbs(command) {
			return "", true, fmt.Errorf("entrypoint command must be relative")
		}
		clean := filepath.ToSlash(filepath.Clean(command))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
			return "", true, fmt.Errorf("entrypoint command escapes extension directory")
		}
		return clean, true, nil
	}
	return "", false, nil
}

func unsafePackagePath(path string) bool {
	if path == "" || path == "." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		return true
	}
	for _, r := range path {
		if r == 0 || r == '\n' || r == '\r' || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func shouldScanForDirectNetwork(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx", ".sh":
		return true
	default:
		return false
	}
}

func shouldScanForDirectFileWrite(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".js", ".mjs", ".cjs", ".ts", ".tsx", ".jsx", ".sh":
		return true
	default:
		return false
	}
}

func scanForDirectNetwork(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("scan brokered network source: %w", err)
	}
	content := strings.ToLower(string(data))
	blocked := []string{
		`"net/http"`,
		`'net/http'`,
		"urllib",
		"requests.",
		"import requests",
		"fetch(",
		"axios",
		"http.get",
		"https.get",
		"net.dial",
		"socket.",
		"import socket",
	}
	for _, token := range blocked {
		if strings.Contains(content, token) {
			return fmt.Errorf("direct network code is blocked for generated extensions; use core_requests through the trusted core instead")
		}
	}
	return nil
}

func scanForDirectFileWrite(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("scan filesystem write source: %w", err)
	}
	content := strings.ToLower(string(data))
	blocked := []string{
		"os.writefile(",
		"ioutil.writefile(",
		"os.create(",
		"os.openfile(",
		"os.mkdir(",
		"os.mkdirall(",
		"os.remove(",
		"os.removeall(",
		"os.rename(",
		"write_text(",
		"write_bytes(",
		"shutil.rmtree(",
		"fs.writefile",
		"fs.writefilesync",
		"fs.rm(",
		"fs.rmsync(",
		"fs.unlink(",
		"fs.mkdir(",
		"fs.rename(",
	}
	if strings.HasSuffix(strings.ToLower(path), ".sh") {
		blocked = append(blocked, "rm ", "mv ", "mkdir ", "touch ", ">")
	}
	for _, token := range blocked {
		if strings.Contains(content, token) {
			return fmt.Errorf("direct filesystem write code is blocked for generated extensions; use trusted core file-write workflows with snapshot, diff, confirmation, and rollback instead")
		}
	}
	return nil
}
