package safety

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceGrantStoreGrantsNestedWorkspace(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatalf("MkdirAll(child) error = %v", err)
	}
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))

	grant, err := store.Grant(root, "Root", "test")
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}
	if grant.ID == "" {
		t.Fatal("grant ID is empty")
	}

	granted, matched, err := store.IsGranted(child)
	if err != nil {
		t.Fatalf("IsGranted() error = %v", err)
	}
	if !granted {
		t.Fatal("child path was not covered by parent grant")
	}
	if matched.ID != grant.ID {
		t.Fatalf("matched grant = %q, want %q", matched.ID, grant.ID)
	}
}

func TestWorkspaceGrantStoreCoversNestedFile(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "docs", "company.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("MkdirAll(file dir) error = %v", err)
	}
	if err := os.WriteFile(file, []byte("local document"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	grant, err := store.Grant(root, "Root", "test")
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	granted, matched, err := store.IsGranted(file)
	if err != nil {
		t.Fatalf("IsGranted(file) error = %v", err)
	}
	if !granted {
		t.Fatal("nested file was not covered by parent grant")
	}
	if matched.ID != grant.ID {
		t.Fatalf("matched grant = %q, want %q", matched.ID, grant.ID)
	}
}

func TestWorkspaceGrantStoreGrantsFileParentDirectory(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "docs", "company.md")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatalf("MkdirAll(file dir) error = %v", err)
	}
	if err := os.WriteFile(file, []byte("local document"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))

	grant, err := store.Grant(file, "Company", "test")
	if err != nil {
		t.Fatalf("Grant(file) error = %v", err)
	}
	if grant.Path != filepath.Dir(file) {
		t.Fatalf("grant path = %q, want file parent %q", grant.Path, filepath.Dir(file))
	}

	granted, matched, err := store.IsGranted(file)
	if err != nil {
		t.Fatalf("IsGranted(file) error = %v", err)
	}
	if !granted {
		t.Fatal("file was not covered by its parent grant")
	}
	if matched.ID != grant.ID {
		t.Fatalf("matched grant = %q, want %q", matched.ID, grant.ID)
	}

	revoked, ok, err := store.Revoke(file)
	if err != nil {
		t.Fatalf("Revoke(file) error = %v", err)
	}
	if !ok || revoked.ID != grant.ID {
		t.Fatalf("revoked = %+v, ok = %v, want grant %q", revoked, ok, grant.ID)
	}
}

func TestWorkspaceGrantStoreNormalizesStoredBrowserHomeGrant(t *testing.T) {
	home := filepath.Join(t.TempDir(), "example")
	docs := filepath.Join(home, "Documents")
	email := filepath.Join(docs, "email")
	if err := os.MkdirAll(email, 0o755); err != nil {
		t.Fatalf("MkdirAll(email) error = %v", err)
	}
	t.Setenv("HOME", home)
	storePath := filepath.Join(t.TempDir(), "workspace_grants.json")
	file := workspaceGrantFile{
		Version: workspaceGrantFileVersion,
		Grants: []WorkspaceGrant{{
			ID:        "wsp_existing",
			Path:      "/users/example/documents",
			Label:     "documents",
			Source:    "local_web",
			CreatedAt: "2026-06-04T00:00:00Z",
		}},
	}
	data, err := json.Marshal(file)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(storePath, data, 0o600); err != nil {
		t.Fatalf("WriteFile(store) error = %v", err)
	}
	store := NewWorkspaceGrantStore(storePath)

	granted, matched, err := store.IsGranted(email)
	if err != nil {
		t.Fatalf("IsGranted(email) error = %v", err)
	}
	if !granted {
		t.Fatal("email folder was not covered by stored browser home grant")
	}
	if matched.Path != docs {
		t.Fatalf("matched path = %q, want %q", matched.Path, docs)
	}
	listed, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(listed) != 1 || listed[0].Path != docs {
		t.Fatalf("listed grants = %+v, want normalized docs path", listed)
	}
}

func TestRequireWorkspaceAccessAllowsCurrentRootAndRejectsExternal(t *testing.T) {
	current := t.TempDir()
	external := t.TempDir()
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))

	source, err := RequireWorkspaceAccess(current, current, store)
	if err != nil {
		t.Fatalf("RequireWorkspaceAccess(current) error = %v", err)
	}
	if source != "current_workspace" {
		t.Fatalf("source = %q, want current_workspace", source)
	}

	_, err = RequireWorkspaceAccess(current, external, store)
	if err == nil {
		t.Fatal("external workspace was allowed without grant")
	}
	if !strings.Contains(err.Error(), "workspace grant") {
		t.Fatalf("error = %q, want workspace grant hint", err)
	}
}

func TestRequireWorkspaceAccessAllowsFileInsideCurrentRoot(t *testing.T) {
	current := t.TempDir()
	file := filepath.Join(current, "company.md")
	if err := os.WriteFile(file, []byte("local document"), 0o644); err != nil {
		t.Fatalf("WriteFile(file) error = %v", err)
	}
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))

	source, err := RequireWorkspaceAccess(current, file, store)
	if err != nil {
		t.Fatalf("RequireWorkspaceAccess(file) error = %v", err)
	}
	if source != "current_workspace" {
		t.Fatalf("source = %q, want current_workspace", source)
	}
}

func TestRequireWorkspaceAccessWithGrantReturnsCloseableAccess(t *testing.T) {
	current := t.TempDir()
	external := t.TempDir()
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	grant, err := store.Grant(external, "External", "test")
	if err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	access, err := RequireWorkspaceAccessWithBookmark(current, external, store)
	if err != nil {
		t.Fatalf("RequireWorkspaceAccessWithBookmark() error = %v", err)
	}
	defer access.Close()
	if access.Source != "workspace_grant:"+grant.ID {
		t.Fatalf("Source = %q, want grant source", access.Source)
	}
	if access.Grant.ID != grant.ID {
		t.Fatalf("Grant ID = %q, want %q", access.Grant.ID, grant.ID)
	}
	if access.BookmarkStarted {
		t.Fatal("BookmarkStarted = true for path-only grant")
	}
}

func TestRequireWorkspaceAccessWithInvalidBookmarkPromptsRegrant(t *testing.T) {
	current := t.TempDir()
	external := t.TempDir()
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	if _, err := store.GrantWithOptions(external, WorkspaceGrantOptions{
		Source:           "desktop_file_picker",
		SecurityBookmark: "not-valid-bookmark-data",
	}); err != nil {
		t.Fatalf("GrantWithOptions() error = %v", err)
	}

	_, err := RequireWorkspaceAccessWithBookmark(current, external, store)
	if err == nil {
		t.Fatal("RequireWorkspaceAccessWithBookmark() allowed invalid bookmark")
	}
	if !strings.Contains(err.Error(), "re-grant") {
		t.Fatalf("error = %q, want re-grant prompt", err)
	}
}

func TestWorkspaceGrantStoreMarksBookmarkStale(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	grant, err := store.GrantWithOptions(root, WorkspaceGrantOptions{
		Source:           "desktop_file_picker",
		SecurityBookmark: "bookmark-data",
	})
	if err != nil {
		t.Fatalf("GrantWithOptions() error = %v", err)
	}
	if err := store.MarkBookmarkStale(grant.ID, true); err != nil {
		t.Fatalf("MarkBookmarkStale() error = %v", err)
	}
	grants, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(grants) != 1 || !grants[0].BookmarkStale {
		t.Fatalf("bookmark stale not persisted: %+v", grants)
	}
}

func TestWorkspaceGrantStoreRevokesByPath(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	if _, err := store.Grant(root, "", "test"); err != nil {
		t.Fatalf("Grant() error = %v", err)
	}

	revoked, ok, err := store.Revoke(root)
	if err != nil {
		t.Fatalf("Revoke() error = %v", err)
	}
	if !ok {
		t.Fatal("grant was not revoked")
	}
	if revoked.Path == "" {
		t.Fatal("revoked path is empty")
	}

	grants, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(grants) != 0 {
		t.Fatalf("grant count = %d, want 0", len(grants))
	}
}

func TestWorkspaceGrantStorePersistsSecurityBookmarkMetadata(t *testing.T) {
	root := t.TempDir()
	store := NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	grant, err := store.GrantWithOptions(root, WorkspaceGrantOptions{
		Label:             "Picked",
		Source:            "desktop_file_picker",
		SecurityBookmark:  "bookmark-data",
		BookmarkCreatedAt: "2026-05-07T00:00:00Z",
		BookmarkStale:     true,
	})
	if err != nil {
		t.Fatalf("GrantWithOptions() error = %v", err)
	}
	if grant.SecurityBookmark != "bookmark-data" {
		t.Fatalf("SecurityBookmark = %q, want bookmark-data", grant.SecurityBookmark)
	}
	if grant.BookmarkCreatedAt == "" {
		t.Fatal("BookmarkCreatedAt is empty")
	}
	if !grant.BookmarkStale {
		t.Fatal("BookmarkStale = false, want true")
	}

	grants, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(grants) != 1 {
		t.Fatalf("grant count = %d, want 1", len(grants))
	}
	if grants[0].SecurityBookmark != grant.SecurityBookmark {
		t.Fatalf("listed bookmark = %q, want %q", grants[0].SecurityBookmark, grant.SecurityBookmark)
	}
}
