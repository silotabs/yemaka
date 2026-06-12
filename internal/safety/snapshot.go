package safety

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type SnapshotManager struct {
	Root string
}

type SnapshotManifest struct {
	ID            string         `json:"id"`
	WorkspaceRoot string         `json:"workspace_root"`
	CreatedAt     string         `json:"created_at"`
	Reason        string         `json:"reason"`
	Files         []SnapshotFile `json:"files"`
}

type SnapshotFile struct {
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	Existed   bool   `json:"existed"`
}

func NewSnapshotManager(root string) SnapshotManager {
	return SnapshotManager{Root: root}
}

func (m SnapshotManager) Create(ctx context.Context, workspaceRoot string, paths []string, reason string, policy PathPolicy) (SnapshotManifest, error) {
	if m.Root == "" {
		return SnapshotManifest{}, fmt.Errorf("snapshot root is empty")
	}
	workspaceRoot, err := ResolveWorkspace(workspaceRoot)
	if err != nil {
		return SnapshotManifest{}, err
	}
	if reason == "" {
		reason = "manual snapshot"
	}

	manifest := SnapshotManifest{
		ID:            newSnapshotID(),
		WorkspaceRoot: workspaceRoot,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339Nano),
		Reason:        reason,
	}
	snapshotDir := filepath.Join(m.Root, manifest.ID)
	filesDir := filepath.Join(snapshotDir, "files")
	if err := os.MkdirAll(filesDir, 0o755); err != nil {
		return SnapshotManifest{}, fmt.Errorf("create snapshot directory: %w", err)
	}

	for _, path := range dedupe(paths) {
		select {
		case <-ctx.Done():
			return SnapshotManifest{}, ctx.Err()
		default:
		}
		abs, rel, exists, err := ResolveWritePath(PathPolicy{
			WorkspaceRoot:  workspaceRoot,
			FollowSymlinks: policy.FollowSymlinks,
			IncludeHidden:  policy.IncludeHidden,
		}, path)
		if err != nil {
			return SnapshotManifest{}, err
		}

		entry := SnapshotFile{Path: rel, Existed: exists}
		if exists {
			hash, size, err := copySnapshotFile(abs, filepath.Join(filesDir, filepath.FromSlash(rel)))
			if err != nil {
				return SnapshotManifest{}, err
			}
			entry.SHA256 = hash
			entry.SizeBytes = size
		}
		manifest.Files = append(manifest.Files, entry)
	}

	if err := writeManifest(filepath.Join(snapshotDir, "manifest.json"), manifest); err != nil {
		return SnapshotManifest{}, err
	}
	return manifest, nil
}

func (m SnapshotManager) Last() (SnapshotManifest, error) {
	entries, err := os.ReadDir(m.Root)
	if err != nil {
		return SnapshotManifest{}, fmt.Errorf("read snapshots: %w", err)
	}
	var manifests []SnapshotManifest
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifest, err := m.Read(entry.Name())
		if err != nil {
			continue
		}
		manifests = append(manifests, manifest)
	}
	if len(manifests) == 0 {
		return SnapshotManifest{}, fmt.Errorf("no snapshots found")
	}
	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].CreatedAt > manifests[j].CreatedAt
	})
	return manifests[0], nil
}

func (m SnapshotManager) Read(id string) (SnapshotManifest, error) {
	if strings.TrimSpace(id) == "" {
		return SnapshotManifest{}, fmt.Errorf("snapshot id is empty")
	}
	data, err := os.ReadFile(filepath.Join(m.Root, id, "manifest.json"))
	if err != nil {
		return SnapshotManifest{}, fmt.Errorf("read snapshot manifest: %w", err)
	}
	var manifest SnapshotManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return SnapshotManifest{}, fmt.Errorf("parse snapshot manifest: %w", err)
	}
	return manifest, nil
}

func (m SnapshotManager) Rollback(ctx context.Context, id string) (SnapshotManifest, error) {
	var manifest SnapshotManifest
	var err error
	if id == "last" || strings.TrimSpace(id) == "" {
		manifest, err = m.Last()
	} else {
		manifest, err = m.Read(id)
	}
	if err != nil {
		return SnapshotManifest{}, err
	}

	if _, err := ResolveWorkspace(manifest.WorkspaceRoot); err != nil {
		return SnapshotManifest{}, err
	}
	for _, file := range manifest.Files {
		select {
		case <-ctx.Done():
			return SnapshotManifest{}, ctx.Err()
		default:
		}
		target := filepath.Join(manifest.WorkspaceRoot, filepath.FromSlash(file.Path))
		if _, _, _, err := ResolveWritePath(PathPolicy{WorkspaceRoot: manifest.WorkspaceRoot}, target); err != nil {
			return SnapshotManifest{}, err
		}
		if !file.Existed {
			if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
				return SnapshotManifest{}, fmt.Errorf("remove new file during rollback: %w", err)
			}
			continue
		}
		source := filepath.Join(m.Root, manifest.ID, "files", filepath.FromSlash(file.Path))
		if err := copyFile(source, target); err != nil {
			return SnapshotManifest{}, fmt.Errorf("restore file %s: %w", file.Path, err)
		}
	}
	return manifest, nil
}

func copySnapshotFile(source string, target string) (string, int64, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return "", 0, fmt.Errorf("read snapshot source: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", 0, fmt.Errorf("create snapshot file directory: %w", err)
	}
	if err := os.WriteFile(target, data, 0o644); err != nil {
		return "", 0, fmt.Errorf("write snapshot file: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), int64(len(data)), nil
}

func copyFile(source string, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

func writeManifest(path string, manifest SnapshotManifest) error {
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot manifest: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write snapshot manifest: %w", err)
	}
	return nil
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	var result []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func newSnapshotID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return fmt.Sprintf("snap_%s", time.Now().UTC().Format("20060102T150405.000000000Z"))
	}
	return fmt.Sprintf("snap_%s_%s", time.Now().UTC().Format("20060102T150405.000000000Z"), hex.EncodeToString(bytes[:]))
}
