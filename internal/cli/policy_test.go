package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
)

func TestRunPolicyModeFullAccessAndSafe(t *testing.T) {
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runPolicy(app, []string{"status"}, &out); err != nil {
		t.Fatalf("runPolicy(status) error = %v", err)
	}
	if !strings.Contains(out.String(), `"Mode": "safe"`) {
		t.Fatalf("status output = %q, want safe mode", out.String())
	}

	out.Reset()
	if err := runPolicy(app, []string{"mode", "full_access"}, &out); err != nil {
		t.Fatalf("runPolicy(full_access) error = %v", err)
	}
	if app.config.Security.Policy.Mode != "full_access" {
		t.Fatalf("policy mode = %q, want full_access", app.config.Security.Policy.Mode)
	}
	if app.config.Tools.Shell.RequireConfirmationRisky {
		t.Fatal("full access should disable risky shell confirmation")
	}

	out.Reset()
	if err := runPolicy(app, []string{"mode", "safe"}, &out); err != nil {
		t.Fatalf("runPolicy(safe) error = %v", err)
	}
	if app.config.Security.Policy.Mode != "safe" {
		t.Fatalf("policy mode = %q, want safe", app.config.Security.Policy.Mode)
	}
}

func TestPolicyCommandRoutesFromCLI(t *testing.T) {
	t.Setenv(config.EnvHome, filepath.Join(t.TempDir(), "home"))
	var out bytes.Buffer
	err := Run(context.Background(), []string{"policy", "status"}, &out, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Run(policy status) error = %v", err)
	}
	if !strings.Contains(out.String(), `"Mode":`) {
		t.Fatalf("output = %q, want policy JSON", out.String())
	}
}
