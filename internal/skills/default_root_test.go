package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultDirUsesYemakaHomeOutsideRepo(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, "skills", "default")
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatalf("create installed default skills dir: %v", err)
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
	got, err := DefaultDir()
	if err != nil {
		t.Fatalf("DefaultDir() error = %v", err)
	}
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("DefaultDir() = %q, want %q", got, want)
	}
}
