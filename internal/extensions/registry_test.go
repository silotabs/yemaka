package extensions

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStoreDiscoversValidExtensionDisabledUntilRegisteredAndAuditsActions(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	logs := filepath.Join(root, "logs")
	writeExtensionManifest(t, filepath.Join(generated, "website_monitor"), validManifestYAML())

	store := NewStore(generated, logs)
	store.Now = func() time.Time { return time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC) }

	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("extension count = %d, want 1", len(items))
	}
	item := items[0]
	if item.Name != "website_monitor" || !item.Valid || item.Registered || item.Enabled || item.Callable {
		t.Fatalf("status = %+v, want valid unregistered disabled website_monitor", item)
	}
	if !strings.Contains(item.RunBlockedReason, "tested and registered") {
		t.Fatalf("run blocked reason = %q, want registration gate", item.RunBlockedReason)
	}

	result, err := store.Validate("website_monitor")
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if result.Message != "valid" {
		t.Fatalf("validate message = %q, want valid", result.Message)
	}
	if _, err := store.SetEnabled("website_monitor", true); err == nil || !strings.Contains(err.Error(), "tested and registered") {
		t.Fatalf("SetEnabled(true) error = %v, want registration gate", err)
	}

	disabled, err := store.SetEnabled("website_monitor", false)
	if err != nil {
		t.Fatalf("SetEnabled(false) error = %v", err)
	}
	if disabled.Extension.Enabled || disabled.Extension.Callable {
		t.Fatalf("disabled status = %+v, want not callable", disabled.Extension)
	}

	audit, err := os.ReadFile(filepath.Join(logs, auditFile))
	if err != nil {
		t.Fatalf("read audit: %v", err)
	}
	if !strings.Contains(string(audit), `"action":"validate"`) || !strings.Contains(string(audit), `"action":"disabled"`) {
		t.Fatalf("audit = %s, want validate and disabled records", string(audit))
	}
	records, err := store.RecentAudit(5)
	if err != nil {
		t.Fatalf("RecentAudit() error = %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("RecentAudit() = %+v, want recent validate/disabled records", records)
	}
	if records[0].Action != "disabled" || records[0].Name != "website_monitor" {
		t.Fatalf("latest audit record = %+v, want disabled website_monitor", records[0])
	}
}

func TestStoreRejectsUnsafeManifest(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	manifest := strings.ReplaceAll(validManifestYAML(), "name: website_monitor", "name: unsafe_shell")
	manifest = strings.ReplaceAll(manifest, "shell: false", "shell: true")
	writeExtensionManifest(t, filepath.Join(generated, "unsafe_shell"), manifest)

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("extension count = %d, want 1", len(items))
	}
	if items[0].Valid {
		t.Fatalf("unsafe extension marked valid: %+v", items[0])
	}
	if !strings.Contains(items[0].ValidationError, "shell permission") {
		t.Fatalf("validation error = %q, want shell permission error", items[0].ValidationError)
	}
	if _, err := store.SetEnabled("unsafe_shell", true); err == nil {
		t.Fatal("SetEnabled(invalid) error = nil, want rejection")
	}
}

func TestStoreRejectsSecretPermission(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	manifest := strings.ReplaceAll(validManifestYAML(), "secrets: false", "secrets: true")
	writeExtensionManifest(t, filepath.Join(generated, "secret_tool"), strings.ReplaceAll(manifest, "name: website_monitor", "name: secret_tool"))

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("secret extension marked valid: %+v", items)
	}
	if !strings.Contains(items[0].ValidationError, "secrets permission") {
		t.Fatalf("validation error = %q, want secrets permission error", items[0].ValidationError)
	}
}

func TestStoreMarksMissingEntrypointNotCallable(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir extension: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(validManifestYAML()), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || !items[0].Valid || items[0].Runnable || items[0].Callable {
		t.Fatalf("status = %+v, want valid but not runnable/callable", items)
	}
	if !strings.Contains(items[0].RunBlockedReason, "entrypoint command not found") {
		t.Fatalf("run blocked reason = %q, want missing entrypoint", items[0].RunBlockedReason)
	}
	inspection, err := store.InspectPackage("website_monitor")
	if err == nil || !strings.Contains(err.Error(), "entrypoint command not found") {
		t.Fatalf("InspectPackage() inspection=%+v error=%v, want missing entrypoint error", inspection, err)
	}
}

func TestStoreRejectsSymlinkInExtensionPackage(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())
	if err := os.Symlink(filepath.Join(dir, "website_monitor"), filepath.Join(dir, "linked_runner")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("status = %+v, want invalid symlink package", items)
	}
	if !strings.Contains(items[0].ValidationError, "symlink is not allowed") {
		t.Fatalf("validation error = %q, want symlink error", items[0].ValidationError)
	}
}

func TestStoreRejectsHiddenFileInExtensionPackage(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("SECRET=value\n"), 0o600); err != nil {
		t.Fatalf("write hidden file: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("status = %+v, want invalid hidden file package", items)
	}
	if !strings.Contains(items[0].ValidationError, "hidden file is not allowed") {
		t.Fatalf("validation error = %q, want hidden file error", items[0].ValidationError)
	}
}

func TestStoreRejectsHiddenDirectoryInExtensionPackage(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())
	if err := os.MkdirAll(filepath.Join(dir, ".cache"), 0o755); err != nil {
		t.Fatalf("create hidden dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".cache", "payload.txt"), []byte("hidden"), 0o644); err != nil {
		t.Fatalf("write hidden dir payload: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("status = %+v, want invalid hidden directory package", items)
	}
	if !strings.Contains(items[0].ValidationError, "hidden directory is not allowed") {
		t.Fatalf("validation error = %q, want hidden directory error", items[0].ValidationError)
	}
}

func TestStoreRejectsLargeFileInExtensionPackage(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())
	if err := os.WriteFile(filepath.Join(dir, "payload.bin"), []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("create large file placeholder: %v", err)
	}
	if err := os.Truncate(filepath.Join(dir, "payload.bin"), maxFileBytes+1); err != nil {
		t.Fatalf("create large file: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("status = %+v, want invalid large file package", items)
	}
	if !strings.Contains(items[0].ValidationError, "extension file exceeds") {
		t.Fatalf("validation error = %q, want large file error", items[0].ValidationError)
	}
}

func TestStoreRejectsDirectNetworkCodeWithoutNetworkPermission(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "net/http"

func main() {
	_, _ = http.Get("https://example.com")
}
`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("status = %+v, want invalid direct network source", items)
	}
	if !strings.Contains(items[0].ValidationError, "direct network code is blocked") {
		t.Fatalf("validation error = %q, want direct network block", items[0].ValidationError)
	}
}

func TestStoreRejectsDirectFilesystemWriteCode(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "os"

func main() {
	_ = os.WriteFile("/tmp/yemaka-extension-escape", []byte("unsafe"), 0o600)
}
`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("status = %+v, want invalid direct filesystem write source", items)
	}
	if !strings.Contains(items[0].ValidationError, "direct filesystem write code is blocked") {
		t.Fatalf("validation error = %q, want filesystem write block", items[0].ValidationError)
	}
}

func TestStoreRejectsDirectNetworkCodeInCoreBrokerPackage(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	manifest := strings.ReplaceAll(validManifestYAML(), "permissions:\n", `metadata:
  network_mode: core_broker
permissions:
  network:
    enabled: true
    allowed_methods:
      - GET
    allowed_domains:
      - example.com
`)
	writeExtensionManifest(t, dir, manifest)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import "net/http"

func main() {
	_, _ = http.Get("https://example.com")
}
`), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	store := NewStore(generated, filepath.Join(root, "logs"))
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Valid {
		t.Fatalf("status = %+v, want invalid direct network source", items)
	}
	if !strings.Contains(items[0].ValidationError, "direct network code is blocked") {
		t.Fatalf("validation error = %q, want direct network block", items[0].ValidationError)
	}
}

func TestStoreDeletesGeneratedExtensionPackage(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "website_monitor")
	writeExtensionManifest(t, dir, validManifestYAML())

	store := NewStore(generated, filepath.Join(root, "logs"))
	if _, err := store.List(); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	result, err := store.Delete("website_monitor")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if result.Message != "deleted" {
		t.Fatalf("delete message = %q, want deleted", result.Message)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("extension dir still exists or stat error = %v", err)
	}
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() after delete error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("extension count after delete = %d, want 0", len(items))
	}
}

func TestValidateManifestRequiresConnectorForPost(t *testing.T) {
	manifest := validManifest()
	manifest.Permissions.Network.Enabled = true
	manifest.Permissions.Network.AllowedDomains = []string{"example.com"}
	manifest.Permissions.Network.AllowedMethods = []string{"POST"}
	if err := ValidateManifest(manifest); err == nil {
		t.Fatal("ValidateManifest() error = nil, want POST rejection for non-connector")
	}
	manifest.Type = TypeConnector
	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("ValidateManifest(connector POST) error = %v", err)
	}
}

func TestValidateManifestAppliesGeneratedExtensionPolicy(t *testing.T) {
	manifest := validManifest()
	manifest.Safety.RequiresUserApproval = false
	manifest.Permissions.Network.Enabled = true
	manifest.Permissions.Network.AllowedDomains = []string{"example.com"}
	manifest.Permissions.Network.AllowedMethods = []string{"GET"}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "requires safety.requires_user_approval") {
		t.Fatalf("ValidateManifest(network no approval) error = %v, want approval requirement", err)
	}

	manifest = validManifest()
	manifest.Permissions.Extra = map[string]Permission{
		"policy_override": {Enabled: true},
	}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("ValidateManifest(policy extra) error = %v, want policy block", err)
	}

	manifest = validManifest()
	manifest.Permissions.Extra = map[string]Permission{
		"deployment": {Enabled: true},
	}
	if err := ValidateManifest(manifest); err != nil {
		t.Fatalf("ValidateManifest(approved deployment extra) error = %v", err)
	}

	manifest.Permissions.Extra = map[string]Permission{
		"active_recon": {Enabled: true},
	}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "passive-only") {
		t.Fatalf("ValidateManifest(active recon) error = %v, want passive-only block", err)
	}
}

func TestValidateManifestRejectsPrivateLocalNetworkDomains(t *testing.T) {
	for _, domain := range []string{
		"localhost",
		"service.local",
		"host.docker.internal",
		"127.0.0.1",
		"10.0.0.5",
		"172.16.0.5",
		"192.168.1.5",
		"169.254.1.5",
		"0.0.0.0",
	} {
		manifest := validManifest()
		manifest.Permissions.Network.Enabled = true
		manifest.Permissions.Network.AllowedDomains = []string{domain}
		manifest.Permissions.Network.AllowedMethods = []string{"GET"}
		if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "private/local") {
			t.Fatalf("ValidateManifest(domain %q) error = %v, want private/local block", domain, err)
		}
	}
}

func TestValidateManifestRejectsFilesystemWriteOutsideScope(t *testing.T) {
	manifest := validManifest()
	manifest.Permissions.Filesystem.Write = true
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "explicit generated extension or profile-scoped paths") {
		t.Fatalf("ValidateManifest(filesystem write without paths) error = %v, want explicit scoped paths", err)
	}

	for _, path := range []string{
		"/tmp/outside",
		"../outside",
		"~/secrets",
		"profile://../outside",
		"extension://../outside",
	} {
		manifest := validManifest()
		manifest.Permissions.Filesystem.Write = true
		manifest.Permissions.Filesystem.Paths = []string{path}
		if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "scope") {
			t.Fatalf("ValidateManifest(filesystem path %q) error = %v, want scope block", path, err)
		}
	}
}

func TestValidateManifestFullAccessDoesNotBypassGeneratedExtensionPolicy(t *testing.T) {
	manifest := validManifest()
	manifest.Permissions.Extra = map[string]Permission{
		"policy_override": {Enabled: true},
		"active_recon":    {Enabled: true},
	}
	if err := ValidateManifestWithPolicyMode(manifest, "full_access"); err == nil || !strings.Contains(err.Error(), "blocked by policy") {
		t.Fatalf("ValidateManifestWithPolicyMode(full_access) error = %v, want generated policy block", err)
	}

	manifest = validManifest()
	manifest.Permissions.Shell = true
	if err := ValidateManifestWithPolicyMode(manifest, "full_access"); err == nil || !strings.Contains(err.Error(), "shell permission") {
		t.Fatalf("ValidateManifestWithPolicyMode(full_access shell) error = %v, want shell block", err)
	}
}

func TestValidateManifestRejectsEntrypointShellArgs(t *testing.T) {
	manifest := validManifest()
	manifest.Entrypoint.Command = "python3"
	manifest.Entrypoint.Args = []string{"main.py; rm -rf ."}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "entrypoint.args") {
		t.Fatalf("ValidateManifest(shell arg) error = %v, want entrypoint args block", err)
	}

	manifest = validManifest()
	manifest.Entrypoint.Command = "python3"
	manifest.Entrypoint.Args = []string{"../escape.py"}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "stay inside") {
		t.Fatalf("ValidateManifest(escaping arg) error = %v, want path escape block", err)
	}
}

func TestValidateManifestRejectsSecretLikeValues(t *testing.T) {
	apiKey := "sk-" + "live-secret-value-1234567890"
	githubToken := "ghp_" + "abcdefghijklmnopqrstuvwxyz"
	manifest := validManifest()
	manifest.Metadata = map[string]string{
		"token": apiKey,
	}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "secret-like value") {
		t.Fatalf("ValidateManifest(secret metadata) error = %v, want secret-like value block", err)
	}

	manifest = validManifest()
	manifest.InputSchema["properties"] = map[string]any{
		"api_key": map[string]any{"type": "string", "default": githubToken},
	}
	if err := ValidateManifest(manifest); err == nil || !strings.Contains(err.Error(), "secret-like value") {
		t.Fatalf("ValidateManifest(secret schema default) error = %v, want secret-like value block", err)
	}
}

func TestDeleteRejectsRegistryEntryOutsideGeneratedDir(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	store := NewStore(generated, filepath.Join(root, "logs"))
	if err := store.save(Registry{Entries: map[string]Entry{
		"outside": {Name: "outside", Dir: outside, ManifestPath: filepath.Join(outside, ManifestFile), Valid: true},
	}}); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	if _, err := store.Delete("outside"); err == nil || !strings.Contains(err.Error(), "outside generated extension directory") {
		t.Fatalf("Delete(outside) error = %v, want outside path block", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside dir should remain untouched: %v", err)
	}
}

func TestDeleteRejectsSymlinkedRegistryEntry(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(generated, 0o755); err != nil {
		t.Fatalf("mkdir generated: %v", err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	link := filepath.Join(generated, "linked_tool")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	store := NewStore(generated, filepath.Join(root, "logs"))
	if err := store.save(Registry{Entries: map[string]Entry{
		"linked_tool": {Name: "linked_tool", Dir: link, ManifestPath: filepath.Join(link, ManifestFile), Valid: true},
	}}); err != nil {
		t.Fatalf("save registry: %v", err)
	}
	if _, err := store.Delete("linked_tool"); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("Delete(symlink) error = %v, want symlink path block", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside dir should remain untouched: %v", err)
	}
}

func writeExtensionManifest(t *testing.T, dir string, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir extension: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(content), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	entrypoint := filepath.Base(dir)
	if err := os.WriteFile(filepath.Join(dir, entrypoint), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write entrypoint: %v", err)
	}
}

func validManifest() Manifest {
	return Manifest{
		Name:        "website_monitor",
		Version:     "0.1.0",
		Description: "Monitor a website and report changes.",
		Type:        TypeTool,
		Entrypoint:  Entrypoint{Type: "command", Command: "./website_monitor"},
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"url": map[string]any{"type": "string"}},
		},
		OutputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"ok": map[string]any{"type": "boolean"}},
		},
		Permissions: Permissions{},
		Safety:      Safety{RequiresUserApproval: true, MaxRuntimeSeconds: 30, MaxResponseBytes: 2000000},
		Tests:       []string{"go test ./..."},
	}
}

func validManifestYAML() string {
	return `name: website_monitor
version: 0.1.0
description: Monitor a website and report changes.
type: tool
entrypoint:
  type: command
  command: ./website_monitor
input_schema:
  type: object
  properties:
    url:
      type: string
output_schema:
  type: object
  properties:
    ok:
      type: boolean
permissions:
  filesystem:
    read: false
    write: false
  shell: false
  secrets: false
safety:
  requires_user_approval: true
  max_runtime_seconds: 30
  max_response_bytes: 2000000
tests:
  - go test ./...
`
}
