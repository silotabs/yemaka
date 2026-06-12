package extensions

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/internet"

	"gopkg.in/yaml.v3"
)

func TestValidateObjectAgainstSchemaRejectsBlankRequiredString(t *testing.T) {
	schema := map[string]any{
		"type":     "object",
		"required": []string{"task"},
		"properties": map[string]any{
			"task": map[string]any{"type": "string"},
		},
	}
	if err := ValidateObjectAgainstSchema("input", schema, map[string]any{"task": "   "}); err == nil || !strings.Contains(err.Error(), "input.task is required") {
		t.Fatalf("ValidateObjectAgainstSchema(blank task) error = %v, want required field error", err)
	}
}

func TestValidateRunInputReturnsSampleForMissingRequiredField(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	if _, err := store.Generate(t.Context(), GenerateInput{
		Name:        "local_notes",
		Description: "Summarize local notes.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	err := store.ValidateRunInput("local_notes", map[string]any{})
	if err == nil || !strings.Contains(err.Error(), "input.task is required") || !strings.Contains(err.Error(), `{"task":"Describe the task to run."}`) {
		t.Fatalf("ValidateRunInput() error = %v, want task requirement with sample", err)
	}
}

func TestProposeCreatesSafeLocalProposal(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), filepath.Join(t.TempDir(), "logs"))
	proposal, err := store.Propose("Monitor my local notes folder every morning")
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.Name != "monitor_my_local_notes_folder_every_morning" {
		t.Fatalf("proposal name = %q", proposal.Name)
	}
	if !proposal.RequiresApproval {
		t.Fatal("proposal should require approval")
	}
	if strings.Contains(strings.Join(proposal.Permissions, " "), "network=true") {
		t.Fatalf("proposal permissions = %v, want local-only", proposal.Permissions)
	}
}

func TestProposeNamesExplicitExtensionFromCapabilitySubject(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), filepath.Join(t.TempDir(), "logs"))
	proposal, err := store.Propose("okay first generate or create extension for web monitoring")
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.Name != "web_monitoring" {
		t.Fatalf("proposal name = %q, want web_monitoring", proposal.Name)
	}
	if strings.Contains(proposal.Name, "okay") || strings.Contains(proposal.Name, "generate") || strings.Contains(proposal.Name, "extension") {
		t.Fatalf("proposal name = %q, want capability subject without chat/setup words", proposal.Name)
	}
}

func TestProposeDetectsBrokeredNetworkProposal(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), filepath.Join(t.TempDir(), "logs"))
	proposal, err := store.Propose("Check https://example.com every morning")
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("network mode = %q, want core_broker", proposal.NetworkMode)
	}
	if strings.Join(proposal.AllowedDomains, ",") != "example.com" {
		t.Fatalf("allowed domains = %+v, want example.com", proposal.AllowedDomains)
	}
	permissions := strings.Join(proposal.Permissions, " ")
	if !strings.Contains(permissions, "network=core_broker") || strings.Contains(permissions, "network=true") {
		t.Fatalf("permissions = %+v, want brokered network only", proposal.Permissions)
	}
}

func TestProposeDetectsBareDomainMonitoringAsBrokeredNetwork(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), filepath.Join(t.TempDir(), "logs"))
	proposal, err := store.Propose("Create a scheduled monitor for example.com every hour and record when it is down")
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("network mode = %q, want core_broker", proposal.NetworkMode)
	}
	if strings.Join(proposal.AllowedDomains, ",") != "example.com" {
		t.Fatalf("allowed domains = %+v, want example.com", proposal.AllowedDomains)
	}
	permissions := strings.Join(proposal.Permissions, " ")
	for _, want := range []string{"network=core_broker", "network.methods=GET,HEAD", "network.domains=example.com"} {
		if !strings.Contains(permissions, want) {
			t.Fatalf("permissions = %+v, want %q", proposal.Permissions, want)
		}
	}
}

func TestProposeDetectsOneLetterPublicDomainURL(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), filepath.Join(t.TempDir(), "logs"))
	proposal, err := store.Propose("Create a cron job to check latest AI news from https://x.com")
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.NetworkMode != "core_broker" {
		t.Fatalf("network mode = %q, want core_broker", proposal.NetworkMode)
	}
	if strings.Join(proposal.AllowedDomains, ",") != "x.com" {
		t.Fatalf("allowed domains = %+v, want x.com", proposal.AllowedDomains)
	}
}

func TestProposeDoesNotTreatEmailDomainAsBrokeredNetwork(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "extensions", "generated"), filepath.Join(t.TempDir(), "logs"))
	proposal, err := store.Propose("Create a local monitor for messages from support@example.com")
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if proposal.NetworkMode != "" || len(proposal.AllowedDomains) != 0 {
		t.Fatalf("proposal = %+v, want local-only email handling", proposal)
	}
	permissions := strings.Join(proposal.Permissions, " ")
	if strings.Contains(permissions, "network=core_broker") || strings.Contains(permissions, "network.domains=") {
		t.Fatalf("permissions = %+v, want no brokered network", proposal.Permissions)
	}
}

func TestGenerateRequiresApprovalBeforeWritingFiles(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	_, err := store.Generate(t.Context(), GenerateInput{
		Name:        "local_notes",
		Description: "Summarize local notes.",
		Approved:    false,
	})
	if err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("Generate() error = %v, want approval error", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "extensions", "generated", "local_notes")); !os.IsNotExist(statErr) {
		t.Fatalf("extension directory was created before approval; statErr=%v", statErr)
	}
}

func TestGenerateCreatesTestedRegisteredRunnablePackage(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))

	result, err := store.Generate(t.Context(), GenerateInput{
		Name:        "local_notes",
		Description: "Summarize local notes.",
		Approved:    true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if result.Extension.Name != "local_notes" || !result.Extension.Registered || !result.Extension.Callable {
		t.Fatalf("extension status = %+v, want registered callable local_notes", result.Extension)
	}
	if result.Tests.Status != "passed" {
		t.Fatalf("tests = %+v, want passed", result.Tests)
	}
	for _, filename := range []string{ManifestFile, "README.md", "go.mod", "main.go", "main_test.go", "local_notes"} {
		if _, err := os.Stat(filepath.Join(root, "extensions", "generated", "local_notes", filename)); err != nil {
			t.Fatalf("generated file %s missing: %v", filename, err)
		}
	}
	run, err := store.Run(t.Context(), "local_notes", RunOptions{Input: map[string]any{"task": "summarize"}})
	if err != nil {
		t.Fatalf("Run(generated) error = %v", err)
	}
	if run.Status != "completed" || !strings.Contains(run.Output["summary"].(string), "local_notes handled task") {
		t.Fatalf("run result = %+v", run)
	}
}

func TestGenerateCreatesTestedRegisteredRunnableDraftPackage(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))

	result, err := store.Generate(t.Context(), GenerateInput{
		Name:        "draft_notes",
		Description: "Summarize notes with a drafted package.",
		Approved:    true,
		Draft:       generatedDraftForTest(t, "draft_notes", "Summarize notes with a drafted package."),
	})
	if err != nil {
		t.Fatalf("Generate(draft) error = %v", err)
	}
	if result.Extension.Name != "draft_notes" || !result.Extension.Registered || !result.Extension.Callable {
		t.Fatalf("extension status = %+v, want registered callable draft_notes", result.Extension)
	}
	if result.Tests.Status != "passed" {
		t.Fatalf("tests = %+v, want passed", result.Tests)
	}
	run, err := store.Run(t.Context(), "draft_notes", RunOptions{Input: map[string]any{"task": "summarize"}})
	if err != nil {
		t.Fatalf("Run(draft) error = %v", err)
	}
	if run.Status != "completed" || !strings.Contains(run.Output["summary"].(string), "draft_notes handled task") {
		t.Fatalf("run result = %+v", run)
	}
}

func TestGenerateDraftRejectsUnsafePathBeforeWritingFiles(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	draft := generatedDraftForTest(t, "unsafe_draft", "Unsafe draft.")
	draft.Files = append(draft.Files, DraftFile{Path: "../escape.go", Content: "package main\n"})

	_, err := store.Generate(t.Context(), GenerateInput{
		Name:        "unsafe_draft",
		Description: "Unsafe draft.",
		Approved:    true,
		Draft:       draft,
	})
	if err == nil || !strings.Contains(err.Error(), "unsafe extension draft path") {
		t.Fatalf("Generate(unsafe draft) error = %v, want unsafe path error", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "extensions", "generated", "unsafe_draft")); !os.IsNotExist(statErr) {
		t.Fatalf("unsafe draft directory was created; statErr=%v", statErr)
	}
}

func TestGenerateDraftRejectsManifestNameMismatch(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	draft := generatedDraftForTest(t, "other_name", "Mismatched draft.")

	_, err := store.Generate(t.Context(), GenerateInput{
		Name:        "requested_name",
		Description: "Mismatched draft.",
		Approved:    true,
		Draft:       draft,
	})
	if err == nil || !strings.Contains(err.Error(), "does not match requested name") {
		t.Fatalf("Generate(mismatched draft) error = %v, want name mismatch", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "extensions", "generated", "requested_name")); !os.IsNotExist(statErr) {
		t.Fatalf("mismatched draft directory was created; statErr=%v", statErr)
	}
}

func TestGenerateDraftRejectsSecretManifestBeforeWritingFiles(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	draft := generatedDraftForTest(t, "secret_draft", "Secret draft.")
	var manifest Manifest
	if err := yaml.Unmarshal([]byte(draft.Files[0].Content), &manifest); err != nil {
		t.Fatalf("unmarshal draft manifest: %v", err)
	}
	manifest.Metadata = map[string]string{"token": "sk-" + "live-secret-value-1234567890"}
	data, err := yaml.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal secret manifest: %v", err)
	}
	draft.Files[0].Content = string(data)

	_, err = store.Generate(t.Context(), GenerateInput{
		Name:        "secret_draft",
		Description: "Secret draft.",
		Approved:    true,
		Draft:       draft,
	})
	if err == nil || !strings.Contains(err.Error(), "secret-like value") {
		t.Fatalf("Generate(secret draft) error = %v, want secret-like value block", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "extensions", "generated", "secret_draft")); !os.IsNotExist(statErr) {
		t.Fatalf("secret draft directory was created; statErr=%v", statErr)
	}
}

func TestGenerateCreatesBrokeredNetworkPackage(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))

	result, err := store.Generate(t.Context(), GenerateInput{
		Name:              "web_check",
		Description:       "Check https://example.com through the core broker.",
		Approved:          true,
		BrokeredNetwork:   true,
		AllowedDomains:    []string{"https://example.com/path"},
		MaxRuntimeSeconds: 5,
	})
	if err != nil {
		t.Fatalf("Generate(brokered) error = %v", err)
	}
	if result.Extension.Name != "web_check" || !result.Extension.Registered || !result.Extension.Callable {
		t.Fatalf("extension status = %+v, want registered callable web_check", result.Extension)
	}
	detail, err := store.Show("web_check")
	if err != nil {
		t.Fatalf("Show(web_check) error = %v", err)
	}
	if detail.Manifest.Metadata["network_mode"] != "core_broker" {
		t.Fatalf("metadata = %+v, want network_mode core_broker", detail.Manifest.Metadata)
	}
	if got := strings.Join(detail.Manifest.Permissions.Network.AllowedDomains, ","); got != "example.com" {
		t.Fatalf("allowed domains = %s, want example.com", got)
	}

	cfg := config.Default().Internet
	cfg.Enabled = true
	cfg.Policy.BlockPrivateIPRanges = false
	cfg.Policy.BlockLocalNetworkByDefault = false
	service := internet.New(cfg, root, filepath.Join(root, "logs"))
	service.Client = &http.Client{Transport: extensionRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: 200,
			Status:     "OK",
			Request:    request,
			Header: http.Header{
				"Content-Type": []string{"text/plain"},
			},
			Body: io.NopCloser(strings.NewReader("hello from generated broker")),
		}, nil
	})}

	run, err := store.Run(t.Context(), "web_check", RunOptions{
		Input: map[string]any{
			"url":  "https://example.com/page",
			"task": "check page",
		},
		Internet: service,
	})
	if err != nil {
		t.Fatalf("Run(web_check) result=%+v error=%v", run, err)
	}
	summary, _ := run.Output["summary"].(string)
	if run.Status != "completed" || !strings.Contains(summary, "hello from generated broker") {
		t.Fatalf("run result = %+v, want brokered fetch summary", run)
	}
	if len(run.CoreResults) != 1 || run.CoreResults[0].Status != "ok" {
		t.Fatalf("core results = %+v, want one successful broker result", run.CoreResults)
	}
}

func TestRegisterGeneratedDisablesFailedTests(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	store := NewStore(generated, filepath.Join(root, "logs"))
	dir := filepath.Join(generated, "broken_tool")
	writeBrokenGeneratedPackage(t, dir)

	result, err := store.RegisterGenerated(t.Context(), "broken_tool", TestOptions{})
	if err == nil || !strings.Contains(err.Error(), "tests failed") {
		t.Fatalf("RegisterGenerated() result=%+v error=%v, want test failure", result, err)
	}
	items, listErr := store.List()
	if listErr != nil {
		t.Fatalf("List() error = %v", listErr)
	}
	if len(items) != 1 || items[0].Enabled || items[0].Callable {
		t.Fatalf("status after failed registration = %+v, want disabled not callable", items)
	}
}

func TestRegisteredExtensionEditRequiresRetestBeforeRun(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))

	result, err := store.Generate(t.Context(), GenerateInput{
		Name:        "local_notes",
		Description: "Summarize local notes.",
		Approved:    true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if !result.Extension.Registered || !result.Extension.Callable {
		t.Fatalf("generated status = %+v, want registered callable", result.Extension)
	}

	source := filepath.Join(root, "extensions", "generated", "local_notes", "main.go")
	if err := os.WriteFile(source, []byte(generatedMainGo("local_notes")+"\n// local edit requires retest\n"), 0o644); err != nil {
		t.Fatalf("edit generated source: %v", err)
	}
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 1 || items[0].Registered || items[0].Enabled || items[0].Callable {
		t.Fatalf("status after edit = %+v, want unregistered disabled not callable", items)
	}
	if !strings.Contains(items[0].RunBlockedReason, "changed since last registration") {
		t.Fatalf("run blocked reason = %q, want changed package gate", items[0].RunBlockedReason)
	}
	if _, err := store.Run(t.Context(), "local_notes", RunOptions{Input: map[string]any{"task": "summarize"}}); err == nil || !strings.Contains(err.Error(), "tested and registered") {
		t.Fatalf("Run(after edit) error = %v, want registration gate", err)
	}

	testResult, err := store.RegisterGenerated(t.Context(), "local_notes", TestOptions{})
	if err != nil {
		t.Fatalf("RegisterGenerated(after edit) result=%+v error=%v", testResult, err)
	}
	run, err := store.Run(t.Context(), "local_notes", RunOptions{Input: map[string]any{"task": "summarize"}})
	if err != nil {
		t.Fatalf("Run(after re-register) error = %v", err)
	}
	if run.Status != "completed" {
		t.Fatalf("run status = %q, want completed", run.Status)
	}
}

func TestRollbackGenerationRemovesFilesAndRegistryEntry(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	result, err := store.Generate(t.Context(), GenerateInput{
		Name:        "local_notes",
		Description: "Summarize local notes.",
		Approved:    true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	rollback, err := store.RollbackGeneration(result.Snapshot.ID)
	if err != nil {
		t.Fatalf("RollbackGeneration() error = %v", err)
	}
	if rollback.Name != "local_notes" {
		t.Fatalf("rollback = %+v, want local_notes", rollback)
	}
	if _, err := os.Stat(filepath.Join(root, "extensions", "generated", "local_notes")); !os.IsNotExist(err) {
		t.Fatalf("generated directory still exists or stat error = %v", err)
	}
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("registry items = %+v, want empty", items)
	}
}

func TestGeneratePersistsSnapshotBeforePackageRegistration(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	result, err := store.Generate(t.Context(), GenerateInput{
		Name:        "local_notes",
		Description: "Summarize local notes.",
		Approved:    true,
	})
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	snapshot, err := store.findSnapshot(result.Snapshot.ID)
	if err != nil {
		t.Fatalf("findSnapshot() error = %v", err)
	}
	if snapshot.Name != "local_notes" || snapshot.Existed {
		t.Fatalf("snapshot = %+v, want non-existing local_notes snapshot", snapshot)
	}
	if snapshot.TargetDir != filepath.Join(root, "extensions", "generated", "local_notes") {
		t.Fatalf("snapshot target = %q", snapshot.TargetDir)
	}
	if len(snapshot.Files) == 0 {
		t.Fatalf("snapshot files empty: %+v", snapshot)
	}
}

func TestFailedGeneratedExtensionRegistrationCanBeRolledBack(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	result, err := store.Generate(t.Context(), GenerateInput{
		Name:        "broken_tool",
		Description: "Broken extension for rollback test.",
		Approved:    true,
		Draft:       brokenDraftForTest(t, "broken_tool"),
	})
	if err == nil || !strings.Contains(err.Error(), "tests failed") {
		t.Fatalf("Generate(broken) result=%+v error=%v, want test failure", result, err)
	}
	rollback, err := store.RollbackGeneration(result.Snapshot.ID)
	if err != nil {
		t.Fatalf("RollbackGeneration(failed snapshot) error = %v", err)
	}
	if rollback.Name != "broken_tool" {
		t.Fatalf("rollback = %+v, want broken_tool", rollback)
	}
	if _, err := os.Stat(filepath.Join(root, "extensions", "generated", "broken_tool")); !os.IsNotExist(err) {
		t.Fatalf("broken extension dir should be absent after rollback: %v", err)
	}
	items, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("registry items = %+v, want empty after failed rollback", items)
	}
}

func TestRollbackRejectsSnapshotOutsideGeneratedDir(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatalf("mkdir outside: %v", err)
	}
	snapshot := Snapshot{
		ID:        "outside-snapshot",
		Name:      "outside",
		TargetDir: outside,
		Files:     []string{"extension.yaml"},
		CreatedAt: store.timestamp(),
	}
	if err := store.saveSnapshot(snapshot); err != nil {
		t.Fatalf("saveSnapshot() error = %v", err)
	}
	if _, err := store.RollbackGeneration(snapshot.ID); err == nil || !strings.Contains(err.Error(), "outside generated extension directory") {
		t.Fatalf("RollbackGeneration(outside) error = %v, want outside path block", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside dir should remain untouched: %v", err)
	}
}

func TestRollbackRejectsSnapshotTargetMismatch(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	store := NewStore(generated, filepath.Join(root, "logs"))
	other := filepath.Join(generated, "other_tool")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("mkdir other extension: %v", err)
	}
	snapshot := Snapshot{
		ID:        "mismatched-snapshot",
		Name:      "target_tool",
		TargetDir: other,
		Files:     []string{"extension.yaml"},
		CreatedAt: store.timestamp(),
	}
	if err := store.saveSnapshot(snapshot); err != nil {
		t.Fatalf("saveSnapshot() error = %v", err)
	}
	if _, err := store.RollbackGeneration(snapshot.ID); err == nil || !strings.Contains(err.Error(), "snapshot target does not match generated extension scope") {
		t.Fatalf("RollbackGeneration(mismatch) error = %v, want scope mismatch block", err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatalf("other extension dir should remain untouched: %v", err)
	}
}

func TestRollbackRejectsSymlinkTargetDir(t *testing.T) {
	root := t.TempDir()
	generated := filepath.Join(root, "extensions", "generated")
	store := NewStore(generated, filepath.Join(root, "logs"))
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
	snapshot := Snapshot{
		ID:        "symlink-snapshot",
		Name:      "linked_tool",
		TargetDir: link,
		Files:     []string{"extension.yaml"},
		CreatedAt: store.timestamp(),
	}
	if err := store.saveSnapshot(snapshot); err != nil {
		t.Fatalf("saveSnapshot() error = %v", err)
	}
	if _, err := store.RollbackGeneration(snapshot.ID); err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("RollbackGeneration(symlink) error = %v, want symlink path block", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside dir should remain untouched: %v", err)
	}
}

func TestRollbackPreExistingSnapshotRestoresBackup(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	dir := filepath.Join(root, "extensions", "generated", "existing_tool")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir existing extension: %v", err)
	}
	marker := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(marker, []byte("changed"), 0o644); err != nil {
		t.Fatalf("write marker: %v", err)
	}
	backup := filepath.Join(store.snapshotDir(), "backups", "existing-snapshot")
	if err := os.MkdirAll(backup, 0o755); err != nil {
		t.Fatalf("mkdir backup: %v", err)
	}
	if err := os.WriteFile(filepath.Join(backup, "keep.txt"), []byte("keep"), 0o644); err != nil {
		t.Fatalf("write backup marker: %v", err)
	}
	snapshot := Snapshot{
		ID:        "existing-snapshot",
		Name:      "existing_tool",
		TargetDir: dir,
		Existed:   true,
		BackupDir: backup,
		Files:     []string{"keep.txt"},
		CreatedAt: store.timestamp(),
	}
	if err := store.saveSnapshot(snapshot); err != nil {
		t.Fatalf("saveSnapshot() error = %v", err)
	}
	rollback, err := store.RollbackGeneration(snapshot.ID)
	if err != nil {
		t.Fatalf("RollbackGeneration(existing) error = %v", err)
	}
	if rollback.Message != "pre-existing generated extension restored from backup" {
		t.Fatalf("rollback = %+v, want pre-existing restore message", rollback)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatalf("existing extension marker not restored: data=%q err=%v", string(data), err)
	}
}

func TestRollbackPreExistingSnapshotWithoutBackupIsExplicitlyBlocked(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	dir := filepath.Join(root, "extensions", "generated", "existing_tool")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir existing extension: %v", err)
	}
	snapshot := Snapshot{
		ID:        "existing-without-backup",
		Name:      "existing_tool",
		TargetDir: dir,
		Existed:   true,
		Files:     []string{"keep.txt"},
		CreatedAt: store.timestamp(),
	}
	if err := store.saveSnapshot(snapshot); err != nil {
		t.Fatalf("saveSnapshot() error = %v", err)
	}
	if _, err := store.RollbackGeneration(snapshot.ID); err == nil || !strings.Contains(err.Error(), "requires a backup directory") {
		t.Fatalf("RollbackGeneration(existing without backup) error = %v, want backup requirement", err)
	}
}

func TestRollbackPreExistingSnapshotRejectsBackupOutsideSnapshotDir(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	dir := filepath.Join(root, "extensions", "generated", "existing_tool")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir existing extension: %v", err)
	}
	outsideBackup := filepath.Join(root, "outside-backup")
	if err := os.MkdirAll(outsideBackup, 0o755); err != nil {
		t.Fatalf("mkdir outside backup: %v", err)
	}
	snapshot := Snapshot{
		ID:        "existing-outside-backup",
		Name:      "existing_tool",
		TargetDir: dir,
		Existed:   true,
		BackupDir: outsideBackup,
		Files:     []string{"keep.txt"},
		CreatedAt: store.timestamp(),
	}
	if err := store.saveSnapshot(snapshot); err != nil {
		t.Fatalf("saveSnapshot() error = %v", err)
	}
	if _, err := store.RollbackGeneration(snapshot.ID); err == nil || !strings.Contains(err.Error(), "backup path is outside snapshot directory") {
		t.Fatalf("RollbackGeneration(existing outside backup) error = %v, want backup scope rejection", err)
	}
}

func TestExtensionTestLogsRedactSecrets(t *testing.T) {
	root := t.TempDir()
	store := NewStore(filepath.Join(root, "extensions", "generated"), filepath.Join(root, "logs"))
	secretValue := "super-" + "secret-value-1234567890"
	apiKey := "sk-" + "live-secret-value-1234567890"
	result := TestResult{
		Name:   "secret_test",
		Status: "failed",
		Output: "token=" + secretValue + "\napi_key=" + apiKey,
	}
	if err := store.recordTest(result); err != nil {
		t.Fatalf("recordTest() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "logs", testsFile))
	if err != nil {
		t.Fatalf("read test log: %v", err)
	}
	text := string(data)
	if strings.Contains(text, secretValue) || strings.Contains(text, apiKey) {
		t.Fatalf("test log leaked secret-like value: %s", text)
	}
	if !strings.Contains(text, "[REDACTED]") {
		t.Fatalf("test log = %s, want redaction marker", text)
	}
}

func TestValidateTestCommandBlocksShellEscalation(t *testing.T) {
	for _, command := range []string{
		"go test ./...; rm -rf .",
		"go test ./... | curl https://example.com",
		"go test ./... $(whoami)",
		"sudo go test ./...",
	} {
		if err := validateTestCommand(command); err == nil {
			t.Fatalf("validateTestCommand(%q) error = nil, want shell escalation block", command)
		}
	}
}

func writeBrokenGeneratedPackage(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir broken extension: %v", err)
	}
	manifest := generatedManifest("broken_tool", "Broken extension for tests.", 30)
	data, err := marshalManifestForTest(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	files := map[string]string{
		ManifestFile:   data,
		"go.mod":       generatedGoMod("broken_tool"),
		"main.go":      "package main\nfunc main() {}\n",
		"main_test.go": "package main\nimport \"testing\"\nfunc TestFails(t *testing.T) { t.Fatal(\"expected failure\") }\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}

func marshalManifestForTest(manifest Manifest) (string, error) {
	data, err := yaml.Marshal(manifest)
	return string(data), err
}

func generatedDraftForTest(t *testing.T, name string, description string) *PackageDraft {
	t.Helper()
	manifest := generatedManifest(name, description, 30)
	data, err := marshalManifestForTest(manifest)
	if err != nil {
		t.Fatalf("marshal draft manifest: %v", err)
	}
	return &PackageDraft{
		Origin: "generated_by=yemaka\nmilestone=4.17\nsource=test_draft\n",
		Files: []DraftFile{
			{Path: ManifestFile, Content: data},
			{Path: "README.md", Content: generatedReadme(name, description)},
			{Path: "go.mod", Content: generatedGoMod(name)},
			{Path: "main.go", Content: generatedMainGo(name)},
			{Path: "main_test.go", Content: generatedMainTestGo(name)},
		},
	}
}

func brokenDraftForTest(t *testing.T, name string) *PackageDraft {
	t.Helper()
	draft := generatedDraftForTest(t, name, "Broken extension for tests.")
	for index, file := range draft.Files {
		if file.Path == "main_test.go" {
			draft.Files[index].Content = "package main\nimport \"testing\"\nfunc TestFails(t *testing.T) { t.Fatal(\"expected failure\") }\n"
			return draft
		}
	}
	t.Fatal("test draft did not contain main_test.go")
	return nil
}
