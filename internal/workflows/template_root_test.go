package workflows

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltInTemplateRootUsesYemakaHomeOutsideRepo(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "packs", "templates"), 0o755); err != nil {
		t.Fatalf("create installed template root: %v", err)
	}

	originalCWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	outsideRepo := t.TempDir()
	if err := os.Chdir(outsideRepo); err != nil {
		t.Fatalf("chdir outside repo: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalCWD)
	})

	t.Setenv(yemakaHomeEnv, home)
	root, err := BuiltInTemplateRoot()
	if err != nil {
		t.Fatalf("BuiltInTemplateRoot() error = %v", err)
	}
	if filepath.Clean(root) != filepath.Clean(home) {
		t.Fatalf("BuiltInTemplateRoot() = %q, want %q", root, home)
	}
}
