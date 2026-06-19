package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"yemaka/internal/config"
)

func TestDefaultServeStaticDirFindsInstalledWebBundle(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	home := filepath.Join(root, "home")
	t.Setenv(config.EnvHome, home)
	webDist := filepath.Join(home, "web", "dist")
	if err := os.MkdirAll(webDist, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDist, "index.html"), []byte("<html>Yemaka</html>"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if got := defaultServeStaticDir(); got != webDist {
		t.Fatalf("defaultServeStaticDir() = %q, want %q", got, webDist)
	}
}

func TestDefaultServeStaticDirPrefersDevelopmentBundle(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)

	home := filepath.Join(root, "home")
	t.Setenv(config.EnvHome, home)
	webDist := filepath.Join(home, "web", "dist")
	if err := os.MkdirAll(webDist, 0o755); err != nil {
		t.Fatalf("MkdirAll(installed) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(webDist, "index.html"), []byte("<html>installed</html>"), 0o644); err != nil {
		t.Fatalf("WriteFile(installed) error = %v", err)
	}
	devDist := filepath.Join(root, "frontend", "dist")
	if err := os.MkdirAll(devDist, 0o755); err != nil {
		t.Fatalf("MkdirAll(dev) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(devDist, "index.html"), []byte("<html>dev</html>"), 0o644); err != nil {
		t.Fatalf("WriteFile(dev) error = %v", err)
	}

	if got := defaultServeStaticDir(); got != "frontend/dist" {
		t.Fatalf("defaultServeStaticDir() = %q, want development frontend/dist", got)
	}
}

func TestServeRuntimeControlRestartsThroughHandoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	launched := false
	control := &serveRuntimeControl{
		cancel: cancel,
		restartLauncher: func() error {
			launched = true
			return nil
		},
	}

	result, err := control.Request("restart")
	if err != nil {
		t.Fatalf("Request(restart) error = %v", err)
	}
	if !result.Accepted || result.Action != "restart" || !launched || !control.RestartRequested() {
		t.Fatalf("restart result = %+v, launched=%t requested=%t", result, launched, control.RestartRequested())
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("restart did not release the server lifecycle")
	}
}
