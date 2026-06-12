package extensions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/internet"
)

func TestMain(m *testing.M) {
	switch os.Getenv("YEMAKA_EXTENSION_HELPER_MODE") {
	case "success":
		fmt.Print(`{"protocol":"yemaka.extension.v1","ok":true,"output":{"message":"hello from extension"}}`)
		os.Exit(0)
	case "bad_output":
		fmt.Print(`{"protocol":"yemaka.extension.v1","ok":true,"output":{"message":42}}`)
		os.Exit(0)
	case "slow":
		time.Sleep(2 * time.Second)
		fmt.Print(`{"protocol":"yemaka.extension.v1","ok":true,"output":{"message":"late"}}`)
		os.Exit(0)
	case "large":
		fmt.Print(`{"protocol":"yemaka.extension.v1","ok":true,"output":{"message":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
		os.Exit(0)
	case "network":
		fmt.Print(`{"protocol":"yemaka.extension.v1","ok":true,"output":{"message":"network"}}`)
		os.Exit(0)
	case "secret_error":
		fmt.Fprint(os.Stderr, "token=super-"+"secret-value-1234567890")
		fmt.Print(`{"protocol":"yemaka.extension.v1","ok":false,"error":"api_key=sk-` + `live-secret-value-1234567890"}`)
		os.Exit(0)
	case "core_fetch":
		runCoreFetchHelper()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func runCoreFetchHelper() {
	data, _ := io.ReadAll(os.Stdin)
	var request InvocationRequest
	_ = json.Unmarshal(data, &request)
	if len(request.CoreResults) == 0 {
		response := InvocationResponse{
			Protocol: ProtocolVersion,
			OK:       true,
			CoreRequests: []CoreRequest{{
				ID:   "fetch_page",
				Tool: "internet_fetch",
				Input: map[string]any{
					"url":         request.Input["url"],
					"extractText": true,
				},
			}},
		}
		_ = json.NewEncoder(os.Stdout).Encode(response)
		return
	}
	if request.CoreResults[0].Status != "ok" {
		_ = json.NewEncoder(os.Stdout).Encode(InvocationResponse{
			Protocol: ProtocolVersion,
			OK:       false,
			Error:    request.CoreResults[0].Error,
		})
		return
	}
	text := fmt.Sprint(request.CoreResults[0].Output["extractedText"])
	_ = json.NewEncoder(os.Stdout).Encode(InvocationResponse{
		Protocol: ProtocolVersion,
		OK:       true,
		Output: map[string]any{
			"message": "fetched: " + text,
		},
	})
}

func TestRunExecutesEnabledExtensionOutOfProcess(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	logs := filepath.Join(root, "logs")
	dir := filepath.Join(generated, "local_echo")
	writeRunnerExtension(t, dir, runnerManifestYAML("local_echo", `./runner`, nil))
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "success")

	store := NewStore(generated, logs)
	registerRunnerExtensionForTest(t, store, "local_echo")
	result, err := store.Run(t.Context(), "local_echo", RunOptions{
		Input:             map[string]any{"message": "hello"},
		MaxRuntimeSeconds: 3,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Status != "completed" || result.Output["message"] != "hello from extension" {
		t.Fatalf("result = %+v, want completed output", result)
	}
	runLog, err := os.ReadFile(filepath.Join(logs, runsFile))
	if err != nil {
		t.Fatalf("read run log: %v", err)
	}
	if !strings.Contains(string(runLog), `"status":"completed"`) || !strings.Contains(string(runLog), `"extension":"local_echo"`) {
		t.Fatalf("run log = %s, want completed local_echo", string(runLog))
	}
}

func TestRunValidatesInputBeforeExecution(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "local_echo")
	writeRunnerExtension(t, dir, runnerManifestYAML("local_echo", `./runner`, nil))
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "success")

	store := NewStore(generated, filepath.Join(root, "logs"))
	registerRunnerExtensionForTest(t, store, "local_echo")
	if _, err := store.Run(t.Context(), "local_echo", RunOptions{Input: map[string]any{}}); err == nil {
		t.Fatal("Run() error = nil, want missing input validation error")
	}
}

func TestRunValidatesOutputEnvelopeAndSchema(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "bad_output")
	writeRunnerExtension(t, dir, runnerManifestYAML("bad_output", `./runner`, nil))
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "bad_output")

	store := NewStore(generated, filepath.Join(root, "logs"))
	registerRunnerExtensionForTest(t, store, "bad_output")
	_, err := store.Run(t.Context(), "bad_output", RunOptions{Input: map[string]any{"message": "hello"}})
	if err == nil || !strings.Contains(err.Error(), "output.message must be a string") {
		t.Fatalf("Run() error = %v, want output schema error", err)
	}
}

func TestRunEnforcesTimeoutAndOutputLimit(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	timeoutDir := filepath.Join(generated, "slow_tool")
	writeRunnerExtension(t, timeoutDir, runnerManifestYAML("slow_tool", `./runner`, nil))
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "slow")

	store := NewStore(generated, filepath.Join(root, "logs"))
	registerRunnerExtensionForTest(t, store, "slow_tool")
	_, err := store.Run(t.Context(), "slow_tool", RunOptions{
		Input:             map[string]any{"message": "hello"},
		DefaultTimeout:    50 * time.Millisecond,
		MaxRuntimeSeconds: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("Run(timeout) error = %v, want timeout", err)
	}

	largeDir := filepath.Join(generated, "large_tool")
	writeRunnerExtension(t, largeDir, runnerManifestYAML("large_tool", `./runner`, nil))
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "large")
	registerRunnerExtensionForTest(t, store, "large_tool")
	_, err = store.Run(t.Context(), "large_tool", RunOptions{
		Input:          map[string]any{"message": "hello"},
		MaxOutputBytes: 20,
	})
	if err == nil || !strings.Contains(err.Error(), "exceeded max output bytes") {
		t.Fatalf("Run(output limit) error = %v, want output limit", err)
	}
}

func TestRunBlocksNetworkEnabledExtensionsInMilestone(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "network_tool")
	manifest := runnerManifestYAML("network_tool", `./runner`, []string{
		"network:",
		"  enabled: true",
		"  allowed_methods:",
		"    - GET",
		"  allowed_domains:",
		"    - example.com",
	})
	manifest = strings.ReplaceAll(manifest, "requires_user_approval: false", "requires_user_approval: true")
	writeRunnerExtension(t, dir, manifest)
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "network")

	store := NewStore(generated, filepath.Join(root, "logs"))
	registerRunnerExtensionForTest(t, store, "network_tool")
	_, err := store.Run(t.Context(), "network_tool", RunOptions{Input: map[string]any{"message": "hello"}})
	if err == nil || !strings.Contains(err.Error(), "metadata.network_mode=core_broker") {
		t.Fatalf("Run(network) error = %v, want broker metadata block", err)
	}
}

func TestRunBrokeredInternetFetchThroughCore(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	logs := filepath.Join(root, "logs")
	dir := filepath.Join(generated, "broker_fetch")
	manifest := runnerManifestYAML("broker_fetch", `./runner`, []string{
		"network:",
		"  enabled: true",
		"  allowed_methods:",
		"    - GET",
		"  allowed_domains:",
		"    - example.com",
	})
	manifest = strings.ReplaceAll(manifest, "requires_user_approval: false", "requires_user_approval: true")
	manifest += `
metadata:
  network_mode: core_broker
`
	writeRunnerExtension(t, dir, manifest)
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "core_fetch")

	cfg := config.Default().Internet
	cfg.Enabled = true
	cfg.Policy.BlockPrivateIPRanges = false
	cfg.Policy.BlockLocalNetworkByDefault = false
	service := internet.New(cfg, root, logs)
	service.Client = &http.Client{Transport: extensionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Status:     "OK",
			Request:    request,
			Header: http.Header{
				"Content-Type": []string{"text/plain"},
			},
			Body: io.NopCloser(strings.NewReader("hello broker")),
		}, nil
	})}

	store := NewStore(generated, logs)
	registerRunnerExtensionForTest(t, store, "broker_fetch")
	result, err := store.Run(t.Context(), "broker_fetch", RunOptions{
		Input:    map[string]any{"message": "hello", "url": "https://example.com/page"},
		Internet: service,
	})
	if err != nil {
		t.Fatalf("Run(brokered fetch) error = %v", err)
	}
	if result.Status != "completed" || result.Output["message"] != "fetched: hello broker" {
		t.Fatalf("result = %+v, want brokered fetch output", result)
	}
	if len(result.CoreResults) != 1 || result.CoreResults[0].Tool != "internet_fetch" || result.CoreResults[0].Status != "ok" {
		t.Fatalf("core results = %+v, want one successful internet_fetch", result.CoreResults)
	}
}

func TestRunFullAccessDoesNotBypassGeneratedExtensionNetworkApproval(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "network_tool")
	writeRunnerExtension(t, dir, runnerManifestYAML("network_tool", `./runner`, []string{
		"network:",
		"  enabled: true",
		"  allowed_methods:",
		"    - GET",
		"  allowed_domains:",
		"    - example.com",
	}))
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "network")

	store := NewStore(generated, filepath.Join(root, "logs"))
	store.PolicyMode = "full_access"
	registerRunnerExtensionForTest(t, store, "network_tool")
	_, err := store.Run(t.Context(), "network_tool", RunOptions{Input: map[string]any{"message": "hello"}})
	if err == nil || !strings.Contains(err.Error(), "network requires safety.requires_user_approval") {
		t.Fatalf("Run(full_access network) error = %v, want generated extension approval block", err)
	}
}

func TestRunFullAccessDoesNotBypassGeneratedExtensionBrokerRequirement(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	dir := filepath.Join(generated, "network_tool")
	manifest := runnerManifestYAML("network_tool", `./runner`, []string{
		"network:",
		"  enabled: true",
		"  allowed_methods:",
		"    - GET",
		"  allowed_domains:",
		"    - example.com",
	})
	manifest = strings.ReplaceAll(manifest, "requires_user_approval: false", "requires_user_approval: true")
	writeRunnerExtension(t, dir, manifest)
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "network")

	store := NewStore(generated, filepath.Join(root, "logs"))
	store.PolicyMode = "full_access"
	registerRunnerExtensionForTest(t, store, "network_tool")
	_, err := store.Run(t.Context(), "network_tool", RunOptions{Input: map[string]any{"message": "hello"}})
	if err == nil || !strings.Contains(err.Error(), "metadata.network_mode=core_broker") {
		t.Fatalf("Run(full_access network no broker) error = %v, want broker requirement", err)
	}
}

func TestRunBrokeredInternetFetchBlocksPrivateLocalNetwork(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	logs := filepath.Join(root, "logs")
	dir := filepath.Join(generated, "broker_private")
	manifest := runnerManifestYAML("broker_private", `./runner`, []string{
		"network:",
		"  enabled: true",
		"  allowed_methods:",
		"    - GET",
		"  allowed_domains:",
		"    - 127.0.0.1",
	})
	manifest = strings.ReplaceAll(manifest, "requires_user_approval: false", "requires_user_approval: true")
	manifest += `
metadata:
  network_mode: core_broker
`
	writeRunnerExtension(t, dir, manifest)
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "core_fetch")

	cfg := config.Default().Internet
	cfg.Enabled = true
	service := internet.New(cfg, root, logs)

	store := NewStore(generated, logs)
	registerRunnerExtensionForTest(t, store, "broker_private")
	result, err := store.Run(t.Context(), "broker_private", RunOptions{
		Input:    map[string]any{"message": "hello", "url": "http://127.0.0.1:1234/private"},
		Internet: service,
	})
	if err == nil || !strings.Contains(err.Error(), "local") {
		t.Fatalf("Run(broker private) result=%+v error=%v, want private/local network block", result, err)
	}
}

func TestRunRedactsSecretLikeValuesFromLogs(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	logs := filepath.Join(root, "logs")
	dir := filepath.Join(generated, "secret_error")
	writeRunnerExtension(t, dir, runnerManifestYAML("secret_error", `./runner`, nil))
	t.Setenv("YEMAKA_EXTENSION_HELPER_MODE", "secret_error")

	store := NewStore(generated, logs)
	registerRunnerExtensionForTest(t, store, "secret_error")
	result, err := store.Run(t.Context(), "secret_error", RunOptions{Input: map[string]any{"message": "hello"}})
	if err == nil {
		t.Fatalf("Run(secret_error) result=%+v error=nil, want failure", result)
	}
	text := strings.Join([]string{err.Error(), result.Error, result.Logs}, "\n")
	if strings.Contains(text, "super-secret") || strings.Contains(text, "sk-live") {
		t.Fatalf("returned result leaked secret-like value: %s", text)
	}
	runLog, readErr := os.ReadFile(filepath.Join(logs, runsFile))
	if readErr != nil {
		t.Fatalf("read run log: %v", readErr)
	}
	audit, readErr := os.ReadFile(filepath.Join(logs, auditFile))
	if readErr != nil {
		t.Fatalf("read audit log: %v", readErr)
	}
	logged := string(runLog) + "\n" + string(audit)
	if strings.Contains(logged, "super-secret") || strings.Contains(logged, "sk-live") {
		t.Fatalf("extension logs leaked secret-like value: %s", logged)
	}
	if !strings.Contains(logged, "[REDACTED]") {
		t.Fatalf("extension logs = %s, want redaction marker", logged)
	}
}

type extensionRoundTripFunc func(*http.Request) (*http.Response, error)

func (f extensionRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func registerRunnerExtensionForTest(t *testing.T, store *Store, name string) {
	t.Helper()
	registry, err := store.discover()
	if err != nil {
		t.Fatalf("discover extension: %v", err)
	}
	entry, ok := registry.Entries[name]
	if !ok {
		t.Fatalf("extension %s not discovered", name)
	}
	fingerprint, err := fingerprintPackageDir(entry.Dir)
	if err != nil {
		t.Fatalf("fingerprint extension: %v", err)
	}
	now := store.timestamp()
	entry.Registered = true
	entry.Enabled = true
	entry.PackageFingerprint = fingerprint
	entry.RegisteredAt = now
	entry.UpdatedAt = now
	registry.Entries[name] = entry
	if err := store.save(registry); err != nil {
		t.Fatalf("save registered extension: %v", err)
	}
}

func writeRunnerExtension(t *testing.T, dir string, manifest string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir extension: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ManifestFile), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("find test executable: %v", err)
	}
	data, err := os.ReadFile(executable)
	if err != nil {
		t.Fatalf("read test executable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "runner"), data, 0o755); err != nil {
		t.Fatalf("write runner: %v", err)
	}
}

func runnerManifestYAML(name string, command string, permissionLines []string) string {
	permissions := []string{
		"filesystem:",
		"  read: false",
		"  write: false",
		"shell: false",
		"secrets: false",
	}
	if len(permissionLines) > 0 {
		permissions = append(permissionLines, permissions...)
	}
	return `name: ` + name + `
version: 0.1.0
description: Local test extension.
type: tool
entrypoint:
  type: command
  command: ` + command + `
input_schema:
  type: object
  required:
    - message
  properties:
    message:
      type: string
output_schema:
  type: object
  required:
    - message
  properties:
    message:
      type: string
permissions:
  ` + strings.ReplaceAll(strings.Join(permissions, "\n"), "\n", "\n  ") + `
safety:
  requires_user_approval: false
  max_runtime_seconds: 5
  max_response_bytes: 100000
tests:
  - ./runner
`
}
