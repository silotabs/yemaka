package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/extensions"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
)

func TestRunExtensionListValidateDisableDelete(t *testing.T) {
	app := newExtensionTestApp(t)
	writeCLIExtension(t, filepath.Join(app.profile.GeneratedExtensions, "website_monitor"))

	var out bytes.Buffer
	if err := runExtension(context.Background(), app, []string{"list"}, &out); err != nil {
		t.Fatalf("runExtension(list) error = %v", err)
	}
	if !strings.Contains(out.String(), "website_monitor") || !strings.Contains(out.String(), "valid") {
		t.Fatalf("list output = %q, want valid website_monitor", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"validate", "website_monitor"}, &out); err != nil {
		t.Fatalf("runExtension(validate) error = %v", err)
	}
	if !strings.Contains(out.String(), "valid") {
		t.Fatalf("validate output = %q, want valid", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"register", "website_monitor"}, &out); err != nil {
		t.Fatalf("runExtension(register) error = %v", err)
	}
	if !strings.Contains(out.String(), `"status": "passed"`) {
		t.Fatalf("register output = %q, want passed tests", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"inspect", "website_monitor"}, &out); err != nil {
		t.Fatalf("runExtension(inspect) error = %v", err)
	}
	if !strings.Contains(out.String(), `"status": "passed"`) || !strings.Contains(out.String(), `"entrypointPath": "website_monitor"`) {
		t.Fatalf("inspect output = %q, want passed entrypoint inspection", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"review", "website_monitor"}, &out); err != nil {
		t.Fatalf("runExtension(review) error = %v", err)
	}
	if !strings.Contains(out.String(), `"reuseCommand": "yemaka extension run website_monitor '{\"url\":\"https://example.com\"}'"`) ||
		!strings.Contains(out.String(), `"sampleInputJson": "{\"url\":\"https://example.com\"}"`) ||
		!strings.Contains(out.String(), `"content": "name: website_monitor`) {
		t.Fatalf("review output = %q, want reuse command and manifest preview", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"run", "website_monitor", "{}"}, &out); err != nil {
		t.Fatalf("runExtension(run) error = %v", err)
	}
	if !strings.Contains(out.String(), `"status": "completed"`) || !strings.Contains(out.String(), "cli extension") {
		t.Fatalf("run output = %q, want completed cli extension output", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"disable", "website_monitor"}, &out); err != nil {
		t.Fatalf("runExtension(disable) error = %v", err)
	}
	if !strings.Contains(out.String(), "disabled") {
		t.Fatalf("disable output = %q, want disabled", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"delete", "website_monitor"}, &out); err != nil {
		t.Fatalf("runExtension(delete) error = %v", err)
	}
	if !strings.Contains(out.String(), "deleted") {
		t.Fatalf("delete output = %q, want deleted", out.String())
	}
	if _, err := os.Stat(filepath.Join(app.profile.GeneratedExtensions, "website_monitor")); !os.IsNotExist(err) {
		t.Fatalf("extension directory still present or stat error = %v", err)
	}
}

func TestRunExtensionProposeGenerateAndRollback(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runExtension(context.Background(), app, []string{"propose", "summarize", "local", "notes"}, &out); err != nil {
		t.Fatalf("runExtension(propose) error = %v", err)
	}
	if !strings.Contains(out.String(), "summarize_local_notes") || !strings.Contains(out.String(), `"requiresApproval": true`) {
		t.Fatalf("propose output = %q", out.String())
	}

	out.Reset()
	err := runExtension(context.Background(), app, []string{"generate", "local_notes", "Summarize local notes"}, &out)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("generate without approval error = %v, want --yes requirement", err)
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"generate", "local_notes", "Summarize local notes", "--yes"}, &out); err != nil {
		t.Fatalf("runExtension(generate) error = %v", err)
	}
	if !strings.Contains(out.String(), `"message": "generated, tested, built, and registered"`) {
		t.Fatalf("generate output = %q", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"run", "local_notes", `{"task":"summarize"}`}, &out); err != nil {
		t.Fatalf("runExtension(run generated) error = %v", err)
	}
	if !strings.Contains(out.String(), `"status": "completed"`) || !strings.Contains(out.String(), "local_notes handled task") {
		t.Fatalf("run generated output = %q", out.String())
	}

	out.Reset()
	if err := runExtension(context.Background(), app, []string{"rollback", "local_notes"}, &out); err != nil {
		t.Fatalf("runExtension(rollback) error = %v", err)
	}
	if !strings.Contains(out.String(), "registry entry removed") {
		t.Fatalf("rollback output = %q", out.String())
	}
}

func TestRunExtensionGenerateFromDraftJSON(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	app := newExtensionTestApp(t)
	draftPath := filepath.Join(t.TempDir(), "draft.json")
	writeCLIDraftJSON(t, draftPath, "draft_cli")

	var out bytes.Buffer
	if err := runExtension(context.Background(), app, []string{"generate", "draft_cli", "Drafted CLI extension", "--yes", "--draft-json", draftPath}, &out); err != nil {
		t.Fatalf("runExtension(generate --draft-json) error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, `"name": "draft_cli"`) || !strings.Contains(output, `"status": "passed"`) {
		t.Fatalf("draft generation output = %q, want registered draft_cli with passing tests", output)
	}
}

func TestRunCapabilityGenerateSynthesizesDraftWithLocalModel(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	app := newExtensionTestApp(t)
	app.config.Models["coding"] = config.ModelConfig{Provider: "ollama", Name: "local-coder:3b", BaseURL: "http://localhost:11434/api"}
	draftPath := filepath.Join(t.TempDir(), "draft.json")
	writeCLIDraftJSON(t, draftPath, "csv_synth_cli")
	draftJSON, err := os.ReadFile(draftPath)
	if err != nil {
		t.Fatalf("read draft JSON: %v", err)
	}
	app.runtimeFactory = func(model config.ModelConfig) (models.Runtime, error) {
		if model.Name != "local-coder:3b" {
			t.Fatalf("draft model = %q, want local-coder:3b", model.Name)
		}
		return cliStaticRuntime{text: string(draftJSON)}, nil
	}

	var out bytes.Buffer
	err = runCapability(app, []string{"generate", "build", "a", "tool", "that", "formats", "CSV", "--name", "csv_synth_cli", "--yes", "--synthesize"}, &out)
	if err != nil {
		t.Fatalf("runCapability(generate --synthesize) error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, `"name": "csv_synth_cli"`) || !strings.Contains(output, `"status": "passed"`) {
		t.Fatalf("capability synthesize output = %q, want registered csv_synth_cli", output)
	}
}

func TestRunCapabilityPropose(t *testing.T) {
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	if err := runCapability(app, []string{"propose", "build", "a", "tool", "that", "normalizes", "CSV"}, &out); err != nil {
		t.Fatalf("runCapability(propose) error = %v", err)
	}
	if !strings.Contains(out.String(), `"kind": "tool_extension"`) || !strings.Contains(out.String(), `"canGenerate": true`) {
		t.Fatalf("capability output = %q, want generated tool extension route", out.String())
	}

	out.Reset()
	if err := runCapability(app, []string{"propose", "build", "a", "tool", "to", "check", "https://example.com"}, &out); err != nil {
		t.Fatalf("runCapability(propose brokered) error = %v", err)
	}
	if !strings.Contains(out.String(), `"networkMode": "core_broker"`) || !strings.Contains(out.String(), "--brokered-network") {
		t.Fatalf("capability brokered output = %q, want brokered generation command", out.String())
	}
}

func TestRunCapabilityGenerateRequiresApprovalAndCreatesExtension(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	app := newExtensionTestApp(t)

	var out bytes.Buffer
	err := runCapability(app, []string{"generate", "build", "a", "tool", "that", "normalizes", "CSV", "--name", "csv_normalizer"}, &out)
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("capability generate without approval error = %v, want --yes requirement", err)
	}

	out.Reset()
	if err := runCapability(app, []string{"generate", "build", "a", "tool", "that", "normalizes", "CSV", "--name", "csv_normalizer", "--yes"}, &out); err != nil {
		t.Fatalf("runCapability(generate) error = %v", err)
	}
	output := out.String()
	if !strings.Contains(output, `"message": "capability generated, tested, and registered after explicit approval"`) ||
		!strings.Contains(output, `"name": "csv_normalizer"`) ||
		!strings.Contains(output, "yemaka extension inspect csv_normalizer") {
		t.Fatalf("capability generate output = %q", output)
	}

	out.Reset()
	if err := runCapability(app, []string{"generate", "build", "a", "tool", "that", "normalizes", "CSV", "--name", "csv_normalizer_run", "--yes", "--run", "--input-json", `{"task":"normalize rows"}`}, &out); err != nil {
		t.Fatalf("runCapability(generate --run) error = %v", err)
	}
	output = out.String()
	if !strings.Contains(output, `"message": "capability generated, tested, registered, and run after explicit approval"`) ||
		!strings.Contains(output, `"extension": "csv_normalizer_run"`) ||
		!strings.Contains(output, `"status": "completed"`) {
		t.Fatalf("capability generate --run output = %q", output)
	}

	out.Reset()
	if err := runCapability(app, []string{"generate", "build", "a", "tool", "that", "cleans", "CSV", "--name", "csv_normalizer_job", "--yes", "--job", "--every", "1h", "--job-input-json", `{"task":"scheduled cleanup"}`}, &out); err != nil {
		t.Fatalf("runCapability(generate --job) error = %v", err)
	}
	output = out.String()
	if !strings.Contains(output, `"message": "capability generated, tested, registered, and attached to an approved disabled job"`) ||
		!strings.Contains(output, `"targetName": "csv_normalizer_job"`) ||
		!strings.Contains(output, `"scheduleType": "interval"`) ||
		!strings.Contains(output, `"enabled": false`) {
		t.Fatalf("capability generate --job output = %q", output)
	}

	draftPath := filepath.Join(t.TempDir(), "draft.json")
	writeCLIDraftJSON(t, draftPath, "csv_draft_capability")
	out.Reset()
	if err := runCapability(app, []string{"generate", "build", "a", "tool", "that", "formats", "CSV", "--name", "csv_draft_capability", "--yes", "--draft-json", draftPath}, &out); err != nil {
		t.Fatalf("runCapability(generate --draft-json) error = %v", err)
	}
	output = out.String()
	if !strings.Contains(output, `"name": "csv_draft_capability"`) || !strings.Contains(output, `"status": "passed"`) {
		t.Fatalf("capability draft output = %q, want generated draft capability", output)
	}
}

func newExtensionTestApp(t *testing.T) *appContext {
	t.Helper()
	root := t.TempDir()
	t.Setenv(config.EnvHome, root)
	cfg := config.Default()
	cfg.Path = filepath.Join(root, config.ConfigName)
	cfg.Memory.Database = filepath.Join(root, "profiles", "default", "memory.sqlite")
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("profiles.Init() error = %v", err)
	}
	return &appContext{config: cfg, profile: profile}
}

type cliStaticRuntime struct {
	text string
}

func (r cliStaticRuntime) Health(ctx context.Context) error {
	return nil
}

func (r cliStaticRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return nil, nil
}

func (r cliStaticRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (r cliStaticRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if err := emit(models.ChatEvent{Token: r.text}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

func writeCLIExtension(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir extension: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, extensions.ManifestFile), []byte(`name: website_monitor
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
tests:
  - go test ./...
`), 0o644); err != nil {
		t.Fatalf("write extension manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module cli_extension_fixture\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write extension go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const protocolVersion = "yemaka.extension.v1"

func main() {
	_, _ = io.ReadAll(os.Stdin)
	output, err := handle(nil)
	if err != nil {
		emit(false, nil, err.Error())
		os.Exit(1)
	}
	emit(true, output, "")
}

func handle(input map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true, "summary": "cli extension"}, nil
}

func emit(ok bool, output map[string]any, message string) {
	response := map[string]any{"protocol": protocolVersion, "ok": ok, "output": output}
	if !ok {
		response["error"] = message
	}
	data, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode response: %v", err)
		return
	}
	fmt.Print(string(data))
}
`), 0o644); err != nil {
		t.Fatalf("write extension main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(`package main

import "testing"

func TestHandleReturnsSummary(t *testing.T) {
	output, err := handle(nil)
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if output["ok"] != true || output["summary"] != "cli extension" {
		t.Fatalf("output = %+v, want cli extension summary", output)
	}
}
`), 0o644); err != nil {
		t.Fatalf("write extension main_test.go: %v", err)
	}
}

func registerCLIExtensionForTest(t *testing.T, app *appContext) {
	t.Helper()
	store := extensions.NewStore(app.profile.GeneratedExtensions, app.profile.Logs)
	result, err := store.RegisterGenerated(context.Background(), "website_monitor", extensions.TestOptions{})
	if err != nil {
		t.Fatalf("RegisterGenerated(website_monitor) result=%+v error=%v", result, err)
	}
}

func writeCLIDraftJSON(t *testing.T, path string, name string) {
	t.Helper()
	manifest := `name: ` + name + `
version: 0.1.0
description: Draft generated from CLI JSON.
type: tool
entrypoint:
  type: command
  command: ./` + name + `
input_schema:
  type: object
  properties:
    task:
      type: string
output_schema:
  type: object
  properties:
    ok:
      type: boolean
    summary:
      type: string
permissions:
  network:
    enabled: false
  filesystem:
    read: false
    write: false
  shell: false
  secrets: false
safety:
  requires_user_approval: false
  max_runtime_seconds: 30
tests:
  - go test ./...
`
	mainGo := `package main

import "fmt"

func main() {}

func handle(input map[string]any) (map[string]any, error) {
	task, _ := input["task"].(string)
	return map[string]any{"ok": true, "summary": fmt.Sprintf("draft handled %s", task)}, nil
}
`
	mainTest := `package main

import "testing"

func TestHandle(t *testing.T) {
	output, err := handle(map[string]any{"task": "notes"})
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if output["ok"] != true {
		t.Fatalf("output = %+v, want ok", output)
	}
}
`
	draft := extensions.PackageDraft{
		Origin: "generated_by=yemaka\nmilestone=4.17\nsource=cli_test\n",
		Files: []extensions.DraftFile{
			{Path: extensions.ManifestFile, Content: manifest},
			{Path: "README.md", Content: "# " + name + "\n\nDraft generated from CLI JSON.\n"},
			{Path: "go.mod", Content: "module yemaka_generated_" + name + "\n\ngo 1.22\n"},
			{Path: "main.go", Content: mainGo},
			{Path: "main_test.go", Content: mainTest},
		},
	}
	data, err := json.Marshal(draft)
	if err != nil {
		t.Fatalf("marshal draft: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write draft JSON: %v", err)
	}
}
