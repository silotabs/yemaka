package safety

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type WorkspaceGrant struct {
	ID                string `json:"id"`
	Path              string `json:"path"`
	Label             string `json:"label,omitempty"`
	Source            string `json:"source"`
	CreatedAt         string `json:"created_at"`
	SecurityBookmark  string `json:"security_bookmark,omitempty"`
	BookmarkCreatedAt string `json:"bookmark_created_at,omitempty"`
	BookmarkStale     bool   `json:"bookmark_stale,omitempty"`
}

type WorkspaceGrantStore struct {
	Path string
}

type WorkspaceAccess struct {
	Source          string
	Path            string
	Grant           WorkspaceGrant
	BookmarkStarted bool
	BookmarkStale   bool
	closeFn         func()
}

func (a *WorkspaceAccess) Close() {
	if a == nil || a.closeFn == nil {
		return
	}
	a.closeFn()
	a.closeFn = nil
}

const workspaceGrantFileVersion = 2

type WorkspaceGrantOptions struct {
	Label             string
	Source            string
	SecurityBookmark  string
	BookmarkCreatedAt string
	BookmarkStale     bool
}

type workspaceGrantFile struct {
	Version int              `json:"version"`
	Grants  []WorkspaceGrant `json:"grants"`
}

func NewWorkspaceGrantStore(path string) WorkspaceGrantStore {
	return WorkspaceGrantStore{Path: path}
}

func (s WorkspaceGrantStore) Grant(path string, label string, source string) (WorkspaceGrant, error) {
	return s.GrantWithOptions(path, WorkspaceGrantOptions{Label: label, Source: source})
}

func (s WorkspaceGrantStore) GrantWithOptions(path string, options WorkspaceGrantOptions) (WorkspaceGrant, error) {
	abs, err := ResolveWorkspaceGrantRoot(path)
	if err != nil {
		return WorkspaceGrant{}, err
	}
	file, err := s.load()
	if err != nil {
		return WorkspaceGrant{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i, grant := range file.Grants {
		if samePath(grant.Path, abs) {
			file.Grants[i].Label = strings.TrimSpace(options.Label)
			file.Grants[i].Source = grantSource(options.Source)
			if file.Grants[i].CreatedAt == "" {
				file.Grants[i].CreatedAt = now
			}
			applyBookmarkOptions(&file.Grants[i], options, now)
			if err := s.save(file); err != nil {
				return WorkspaceGrant{}, err
			}
			return file.Grants[i], nil
		}
	}
	grant := WorkspaceGrant{
		ID:        workspaceGrantID(abs, now),
		Path:      abs,
		Label:     strings.TrimSpace(options.Label),
		Source:    grantSource(options.Source),
		CreatedAt: now,
	}
	applyBookmarkOptions(&grant, options, now)
	file.Grants = append(file.Grants, grant)
	if err := s.save(file); err != nil {
		return WorkspaceGrant{}, err
	}
	return grant, nil
}

func (s WorkspaceGrantStore) List() ([]WorkspaceGrant, error) {
	file, err := s.load()
	if err != nil {
		return nil, err
	}
	grants := append([]WorkspaceGrant{}, file.Grants...)
	for i := range grants {
		normalizeWorkspaceGrantPath(&grants[i])
	}
	return grants, nil
}

func (s WorkspaceGrantStore) Revoke(match string) (WorkspaceGrant, bool, error) {
	match = strings.TrimSpace(match)
	if match == "" {
		return WorkspaceGrant{}, false, fmt.Errorf("workspace grant id or path is required")
	}
	file, err := s.load()
	if err != nil {
		return WorkspaceGrant{}, false, err
	}
	var revoked WorkspaceGrant
	kept := file.Grants[:0]
	for _, grant := range file.Grants {
		if revoked.ID == "" && grantMatches(grant, match) {
			revoked = grant
			continue
		}
		kept = append(kept, grant)
	}
	if revoked.ID == "" {
		return WorkspaceGrant{}, false, nil
	}
	file.Grants = kept
	if err := s.save(file); err != nil {
		return WorkspaceGrant{}, false, err
	}
	return revoked, true, nil
}

func (s WorkspaceGrantStore) MarkBookmarkStale(id string, stale bool) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("workspace grant id is required")
	}
	file, err := s.load()
	if err != nil {
		return err
	}
	for i := range file.Grants {
		if file.Grants[i].ID == id {
			file.Grants[i].BookmarkStale = stale
			return s.save(file)
		}
	}
	return fmt.Errorf("workspace grant not found: %s", id)
}

func (s WorkspaceGrantStore) IsGranted(path string) (bool, WorkspaceGrant, error) {
	abs, _, err := ResolveWorkspaceTarget(path)
	if err != nil {
		return false, WorkspaceGrant{}, err
	}
	grants, err := s.List()
	if err != nil {
		return false, WorkspaceGrant{}, err
	}
	for _, grant := range grants {
		normalizeWorkspaceGrantPath(&grant)
		if pathInside(grant.Path, abs) {
			return true, grant, nil
		}
	}
	return false, WorkspaceGrant{}, nil
}

func RequireWorkspaceAccess(currentRoot string, requestedRoot string, store WorkspaceGrantStore) (string, error) {
	access, err := RequireWorkspaceAccessWithBookmark(currentRoot, requestedRoot, store)
	if access != nil {
		access.Close()
	}
	if err != nil {
		return "", err
	}
	if access == nil {
		return "", fmt.Errorf("workspace access result is empty")
	}
	return access.Source, nil
}

func RequireWorkspaceAccessWithBookmark(currentRoot string, requestedRoot string, store WorkspaceGrantStore) (*WorkspaceAccess, error) {
	current, err := ResolveWorkspace(currentRoot)
	if err != nil {
		return nil, err
	}
	requested, _, err := ResolveWorkspaceTarget(requestedRoot)
	if err != nil {
		return nil, err
	}
	if pathInside(current, requested) {
		return &WorkspaceAccess{Source: "current_workspace", Path: requested}, nil
	}
	granted, grant, err := store.IsGranted(requested)
	if err != nil {
		return nil, err
	}
	if granted {
		access := &WorkspaceAccess{
			Source: "workspace_grant:" + grant.ID,
			Path:   requested,
			Grant:  grant,
		}
		if strings.TrimSpace(grant.SecurityBookmark) == "" {
			return access, nil
		}
		bookmarkAccess, err := StartSecurityScopedBookmarkAccess(grant.SecurityBookmark)
		if err != nil {
			return nil, fmt.Errorf("workspace security bookmark for %s is unavailable; re-grant this folder from the desktop folder picker: %w", grant.Path, err)
		}
		if bookmarkAccess.Stale {
			_ = store.MarkBookmarkStale(grant.ID, true)
			bookmarkAccess.Close()
			return nil, fmt.Errorf("workspace security bookmark for %s is stale; re-grant this folder from the desktop folder picker", grant.Path)
		}
		if !bookmarkAccess.Started {
			bookmarkAccess.Close()
			return nil, fmt.Errorf("workspace security bookmark for %s could not start scoped access; re-grant this folder from the desktop folder picker", grant.Path)
		}
		if bookmarkAccess.Path != "" && !pathInside(bookmarkAccess.Path, requested) {
			bookmarkAccess.Close()
			return nil, fmt.Errorf("workspace security bookmark resolved to %s, not requested path %s; re-grant this folder", bookmarkAccess.Path, requested)
		}
		access.BookmarkStarted = bookmarkAccess.Started
		access.BookmarkStale = bookmarkAccess.Stale
		access.closeFn = bookmarkAccess.Close
		return access, nil
	}
	return nil, fmt.Errorf("workspace path is not granted: %s; run `yemaka workspace grant %q` first", requested, requested)
}

func WorkspaceGrantsPath(permissionsDir string) string {
	return filepath.Join(permissionsDir, "workspace_grants.json")
}

func (s WorkspaceGrantStore) load() (workspaceGrantFile, error) {
	if strings.TrimSpace(s.Path) == "" {
		return workspaceGrantFile{}, fmt.Errorf("workspace grant store path is empty")
	}
	data, err := os.ReadFile(s.Path)
	if os.IsNotExist(err) {
		return workspaceGrantFile{Version: workspaceGrantFileVersion}, nil
	}
	if err != nil {
		return workspaceGrantFile{}, fmt.Errorf("read workspace grants: %w", err)
	}
	var file workspaceGrantFile
	if err := json.Unmarshal(data, &file); err != nil {
		return workspaceGrantFile{}, fmt.Errorf("parse workspace grants: %w", err)
	}
	if file.Version == 0 {
		file.Version = 1
	}
	for i := range file.Grants {
		normalizeWorkspaceGrantPath(&file.Grants[i])
	}
	return file, nil
}

func (s WorkspaceGrantStore) save(file workspaceGrantFile) error {
	if strings.TrimSpace(s.Path) == "" {
		return fmt.Errorf("workspace grant store path is empty")
	}
	file.Version = workspaceGrantFileVersion
	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace grants: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o755); err != nil {
		return fmt.Errorf("create workspace grant directory: %w", err)
	}
	tmp := s.Path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write workspace grants: %w", err)
	}
	if err := os.Rename(tmp, s.Path); err != nil {
		return fmt.Errorf("replace workspace grants: %w", err)
	}
	return nil
}

func applyBookmarkOptions(grant *WorkspaceGrant, options WorkspaceGrantOptions, now string) {
	if grant == nil {
		return
	}
	bookmark := strings.TrimSpace(options.SecurityBookmark)
	if bookmark == "" {
		return
	}
	grant.SecurityBookmark = bookmark
	grant.BookmarkStale = options.BookmarkStale
	createdAt := strings.TrimSpace(options.BookmarkCreatedAt)
	if createdAt == "" {
		createdAt = now
	}
	grant.BookmarkCreatedAt = createdAt
}

func workspaceGrantID(path string, createdAt string) string {
	sum := sha256.Sum256([]byte(path + "\x00" + createdAt))
	return "wsp_" + hex.EncodeToString(sum[:])[:16]
}

func grantSource(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return "manual"
	}
	return source
}

func grantMatches(grant WorkspaceGrant, match string) bool {
	normalizeWorkspaceGrantPath(&grant)
	if grant.ID == match {
		return true
	}
	if abs, err := ResolveWorkspaceGrantRoot(match); err == nil {
		return samePath(grant.Path, abs)
	}
	return samePath(grant.Path, match)
}

func pathInside(parent string, child string) bool {
	parent = filepath.Clean(NormalizeUserSuppliedPath(parent))
	child = filepath.Clean(NormalizeUserSuppliedPath(child))
	if samePath(parent, child) {
		return true
	}
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func samePath(left string, right string) bool {
	left = filepath.Clean(NormalizeUserSuppliedPath(left))
	right = filepath.Clean(NormalizeUserSuppliedPath(right))
	return left == right
}

func normalizeWorkspaceGrantPath(grant *WorkspaceGrant) {
	if grant == nil {
		return
	}
	path := strings.TrimSpace(grant.Path)
	if path == "" {
		return
	}
	grant.Path = filepath.Clean(NormalizeUserSuppliedPath(path))
}
