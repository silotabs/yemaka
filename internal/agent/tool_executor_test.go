package agent

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/heartbeat"
	"yemaka/internal/internet"
	"yemaka/internal/memory"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/safety"
)

func TestSafeToolExecutorRunsDetectedGoTests(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/yemaka\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "main_test.go"), []byte("package main\n\nimport \"testing\"\n\nfunc TestPass(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		TimeoutSeconds:  15,
		MaxOutputBytes:  10000,
		MaxContextChars: 2000,
	})
	result, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "run_tests",
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if result.SourceKind != "tool" {
		t.Fatalf("SourceKind = %q, want tool", result.SourceKind)
	}
	if !strings.Contains(result.Context, "COMMAND RESULT") {
		t.Fatalf("Context missing command result: %s", result.Context)
	}
	if len(result.Sources) != 1 || result.Sources[0] != "go test ./..." {
		t.Fatalf("Sources = %#v, want go test ./...", result.Sources)
	}
}

func TestCommandForDecisionUsesPythonTestIntentInMixedRepo(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/yemaka\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "test-files"), 0o755); err != nil {
		t.Fatalf("mkdir test-files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "test-files", "test_calculator.py"), []byte("def test_add():\n    assert 2 + 3 == 5\n"), 0o644); err != nil {
		t.Fatalf("write python test: %v", err)
	}

	command, commandRoot, err := commandForDecision(root, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "run_tests",
		Command:   []string{"detect", "Inspect this Python project, run the tests if safe."},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("commandForDecision() error = %v", err)
	}
	if !isPythonPytestCommand(command) {
		t.Fatalf("command = %q, want python/python3 -m pytest", strings.Join(command, " "))
	}
	if commandRoot != root {
		t.Fatalf("commandRoot = %q, want %q", commandRoot, root)
	}
}

func TestSafeToolExecutorListsWorkspaceDirectoryFromAbsolutePath(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "test-files")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir test-files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "calculator.py"), []byte("def add(a, b): return a + b\n"), 0o644); err != nil {
		t.Fatalf("write calculator.py: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "test_calculator.py"), []byte("def test_add(): assert True\n"), 0o644); err != nil {
		t.Fatalf("write test_calculator.py: %v", err)
	}

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
	})
	result, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "list_files",
		Command:   []string{"list_files", dir},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	for _, want := range []string{"DIRECTORY: test-files", "test-files/calculator.py", "test-files/test_calculator.py"} {
		if !strings.Contains(result.Context, want) {
			t.Fatalf("Context missing %q:\n%s", want, result.Context)
		}
	}
	if len(result.Sources) != 1 || result.Sources[0] != "test-files" {
		t.Fatalf("Sources = %#v, want test-files", result.Sources)
	}
}

func TestSafeToolExecutorRunsFileStatAndFileTree(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "test-files"), 0o755); err != nil {
		t.Fatalf("mkdir test-files: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "test-files", "calculator.py"), []byte("def add(a, b): return a + b\n"), 0o644); err != nil {
		t.Fatalf("write calculator.py: %v", err)
	}

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
	})
	stat, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "file_stat",
		Command:   []string{"file_stat", "test-files/calculator.py"},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("file_stat executor error = %v", err)
	}
	for _, want := range []string{"FILE STAT: test-files/calculator.py", "exists: true", "kind: file", "modified:"} {
		if !strings.Contains(stat.Context, want) {
			t.Fatalf("file_stat context missing %q:\n%s", want, stat.Context)
		}
	}

	tree, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "file_tree",
		Command:   []string{"file_tree", "test-files"},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("file_tree executor error = %v", err)
	}
	for _, want := range []string{"FILE TREE: test-files", "test-files/calculator.py"} {
		if !strings.Contains(tree.Context, want) {
			t.Fatalf("file_tree context missing %q:\n%s", want, tree.Context)
		}
	}
}

func TestSafeToolExecutorRunsGitStatus(t *testing.T) {
	root := t.TempDir()
	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		TimeoutSeconds:  5,
		MaxOutputBytes:  10000,
		MaxContextChars: 2000,
	})
	result, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "git_status",
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("git_status executor error = %v", err)
	}
	if result.Status != "failed" {
		t.Fatalf("git_status = %q, want failed outside git repo", result.Status)
	}
	if len(result.Sources) != 1 || result.Sources[0] != "git status --short" {
		t.Fatalf("Sources = %#v, want git status --short", result.Sources)
	}
}

func TestSafeToolExecutorRunsHeartbeatStatusWithoutRecordingHistory(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	profile := &profiles.Profile{
		Name:                "test",
		Root:                filepath.Join(root, "profile"),
		Database:            filepath.Join(root, "profile", "memory.sqlite"),
		GeneratedExtensions: filepath.Join(root, "profile", "extensions", "generated"),
		Logs:                filepath.Join(root, "profile", "logs"),
	}
	for _, dir := range []string{profile.Root, filepath.Dir(profile.Database), profile.GeneratedExtensions, profile.Logs} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	store, err := memory.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	cfg := config.Default()
	cfg.Heartbeat.Checks.Ollama = false
	cfg.Scheduler.Enabled = false
	cfg.Internet.Enabled = false
	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
		Config:          cfg,
		Profile:         profile,
		Memory:          store,
	})
	result, err := executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "heartbeat_status",
		Command:   []string{"heartbeat_status"},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("heartbeat_status executor error = %v", err)
	}
	for _, want := range []string{"HEARTBEAT STATUS", "overall:", "sqlite: healthy", "scheduler: disabled", "internet: disabled"} {
		if !strings.Contains(result.Context, want) {
			t.Fatalf("heartbeat_status context missing %q:\n%s", want, result.Context)
		}
	}
	if len(result.Sources) != 1 || result.Sources[0] != "heartbeat_status" {
		t.Fatalf("Sources = %#v, want heartbeat_status", result.Sources)
	}

	heartbeatStore, err := heartbeat.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("heartbeat.Open() error = %v", err)
	}
	defer heartbeatStore.Close()
	latest, err := heartbeatStore.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if len(latest.Checks) != 0 {
		t.Fatalf("heartbeat_status should not record history, got %d checks", len(latest.Checks))
	}
}

func TestSafeToolExecutorBlocksShellBackedToolsWhenShellDisabled(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("Yemaka\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:        root,
		MaxContextChars:      2000,
		DisableShellCommands: true,
	})
	read, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "read_file",
		Command:   []string{"read_file", "README.md"},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("read_file with shell disabled error = %v", err)
	}
	if !strings.Contains(read.Context, "Yemaka") {
		t.Fatalf("read_file context missing content: %s", read.Context)
	}
	_, err = executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "git_status",
		RiskLevel: RiskLow,
	})
	if err == nil || !strings.Contains(err.Error(), "shell-backed safe tool execution is disabled") {
		t.Fatalf("git_status error = %v, want shell-disabled block", err)
	}
}

func TestSafeToolExecutorRunsCoreInternetFetchWhenProfileEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.Internet.Enabled = true
	cfg.Internet.DefaultMode = "profile_enabled"
	cfg.Internet.Policy.BlockPrivateIPRanges = false
	cfg.Internet.Policy.BlockLocalNetworkByDefault = false

	service := internet.New(cfg.Internet, t.TempDir(), filepath.Join(t.TempDir(), "logs"))
	service.Client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Header: http.Header{
				"Content-Type": []string{"text/html"},
			},
			Body:          io.NopCloser(strings.NewReader("<html><body><h1>Yemaka web result</h1></body></html>")),
			ContentLength: 51,
			Request:       &http.Request{URL: mustParseURL("https://example.com/final")},
		}, nil
	})}

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   t.TempDir(),
		MaxContextChars: 2000,
		Config:          cfg,
		Internet:        service,
	})
	result, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "internet_fetch",
		Command:   []string{"internet_fetch", "https://example.com", "example.com"},
		RiskLevel: RiskMedium,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if result.SourceKind != "internet" {
		t.Fatalf("SourceKind = %q, want internet", result.SourceKind)
	}
	if !strings.Contains(result.Context, "Yemaka web result") {
		t.Fatalf("Context missing fetched text: %s", result.Context)
	}
}

func TestSafeToolExecutorRunsCoreInternetSearchWhenProviderConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.Internet.Enabled = true
	cfg.Internet.DefaultMode = "profile_enabled"
	cfg.Internet.Policy.BlockPrivateIPRanges = false
	cfg.Internet.Policy.BlockLocalNetworkByDefault = false
	cfg.Internet.Search.Enabled = true
	cfg.Internet.Search.Provider = "searxng"
	cfg.Internet.Search.Endpoint = "https://search.example/search"

	service := internet.New(cfg.Internet, t.TempDir(), filepath.Join(t.TempDir(), "logs"))
	service.Client = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     http.StatusText(http.StatusOK),
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body:          io.NopCloser(strings.NewReader(`{"results":[{"title":"Yemaka","url":"https://example.com/yemaka","content":"Local agent"}]}`)),
			ContentLength: 92,
			Request:       &http.Request{URL: mustParseURL("https://search.example/search")},
		}, nil
	})}

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   t.TempDir(),
		MaxContextChars: 2000,
		Config:          cfg,
		Internet:        service,
	})
	result, err := executor(context.Background(), ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "internet_search",
		Command:   []string{"internet_search", "Yemaka local agent"},
		RiskLevel: RiskMedium,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if result.SourceKind != "internet" || len(result.Sources) != 1 || result.Sources[0] != "https://example.com/yemaka" {
		t.Fatalf("result = %+v, want internet search source", result)
	}
	if !strings.Contains(result.Context, "INTERNET SEARCH RESULT") || !strings.Contains(result.Context, "Local agent") {
		t.Fatalf("Context missing search result: %s", result.Context)
	}
	for _, marker := range []string{"fetched_at:", "result_count: 1"} {
		if !strings.Contains(result.Context, marker) {
			t.Fatalf("Context missing freshness marker %q: %s", marker, result.Context)
		}
	}
}

func TestFormatInternetSearchResultWarnsAboutDuckDuckGoNoResults(t *testing.T) {
	text := formatInternetSearchResult(internet.SearchResult{
		Query:     "macos 26.5 latest version",
		Provider:  "duckduckgo",
		FetchedAt: "2026-05-24T00:00:00Z",
	}, 2000)
	for _, marker := range []string{
		"No search result items were returned",
		"instant-answer source",
		"configured supported provider",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("formatInternetSearchResult() missing %q:\n%s", marker, text)
		}
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "bing") || strings.Contains(lower, "google") {
		t.Fatalf("formatInternetSearchResult() suggested unsupported providers:\n%s", text)
	}
}

func TestSafeToolExecutorRunsCoreMemorySearch(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	if _, err := store.SaveMemory(ctx, memory.Memory{
		Kind:       "preference",
		Content:    "Release checklist: run go test ./... before tagging.",
		Importance: 4,
		Source:     "test",
	}); err != nil {
		t.Fatalf("SaveMemory() error = %v", err)
	}

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   t.TempDir(),
		MaxContextChars: 2000,
		Memory:          store,
	})
	result, err := executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "memory_search",
		Command:   []string{"memory_search", "release checklist"},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if result.SourceKind != "memory" {
		t.Fatalf("SourceKind = %q, want memory", result.SourceKind)
	}
	if !strings.Contains(result.Context, "Release checklist") {
		t.Fatalf("Context missing memory result: %s", result.Context)
	}
}

func TestSafeToolExecutorIngestsSingleDocument(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	docPath := filepath.Join(root, "docs", "company.md")
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(docPath, []byte("Launch codename: Stone Lantern\nSupport email: local-only@example.test\n"), 0o644); err != nil {
		t.Fatalf("write doc: %v", err)
	}
	ragStore, err := rag.Open(ctx, filepath.Join(t.TempDir(), "rag.sqlite"))
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	defer ragStore.Close()

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
		RAG:             ragStore,
		Config:          config.Default(),
	})
	result, err := executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "ingest_documents",
		Command:   []string{"ingest_documents", "docs/company.md"},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if result.SourceKind != "rag" || result.Status != "completed" {
		t.Fatalf("result = %+v, want completed RAG ingest", result)
	}
	if !strings.Contains(result.Context, "DOCUMENT INGEST RESULT") || !strings.Contains(result.Context, "files_indexed: 1") {
		t.Fatalf("Context missing ingest result: %s", result.Context)
	}
	hits, err := ragStore.Search(ctx, "Stone Lantern", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("Search returned no hits after ingest: %+v", result)
	}
}

func TestSafeToolExecutorExplainsSkippedOnlyDocumentIngest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	docPath := filepath.Join(root, "docs", "unsupported.rtf")
	if err := os.MkdirAll(filepath.Dir(docPath), 0o755); err != nil {
		t.Fatalf("mkdir docs: %v", err)
	}
	if err := os.WriteFile(docPath, []byte("{\\rtf1 unsupported}"), 0o644); err != nil {
		t.Fatalf("write unsupported doc: %v", err)
	}
	ragStore, err := rag.Open(ctx, filepath.Join(t.TempDir(), "rag.sqlite"))
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	defer ragStore.Close()

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
		RAG:             ragStore,
		Config:          config.Default(),
	})
	result, err := executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "ingest_documents",
		Command:   []string{"ingest_documents", "docs/unsupported.rtf"},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if result.SourceKind != "rag" || result.Status != "completed" {
		t.Fatalf("result = %+v, want completed RAG scan with skipped-only explanation", result)
	}
	if !strings.Contains(result.Context, "files_skipped: 1") || !strings.Contains(result.Context, "no new document chunks were created") {
		t.Fatalf("Context missing skipped-only ingest explanation: %s", result.Context)
	}
	if !strings.Contains(result.Context, "skipped_reasons") {
		t.Fatalf("Context missing skipped reasons: %s", result.Context)
	}
}

func TestFormatDocumentIngestResultExplainsSkippedFolderWithoutReasons(t *testing.T) {
	text := formatDocumentIngestResult(rag.IngestResult{
		Root:         "/tmp/docs",
		FilesSkipped: 2,
	}, "/tmp/docs", 2000)
	if !strings.Contains(text, "files_skipped: 2") || !strings.Contains(text, "document safety/format limits") {
		t.Fatalf("formatDocumentIngestResult() = %s, want generic skipped-folder explanation", text)
	}
}

func TestDocumentIngestResponseStaysGroundedToToolCounts(t *testing.T) {
	response := DocumentIngestResponse(ExecutionResult{
		Context: "DOCUMENT INGEST RESULT:\n" +
			"target: /Users/example/Documents/email\n" +
			"files_indexed: 0\n" +
			"files_unchanged: 0\n" +
			"files_skipped: 2\n" +
			"chunks_created: 0\n" +
			"bytes_indexed: 0\n" +
			"note: no new document chunks were created. The path was scanned, but no supported readable documents needed indexing, or files were skipped by document safety/format limits.\n",
		SourceKind: "rag",
		Status:     "completed",
	})
	for _, want := range []string{
		"Document ingest completed.",
		"Path: `/Users/example/Documents/email`",
		"Result: 0 indexed, 0 unchanged, 2 skipped, 0 chunks, 0 bytes.",
		"Skipped file names were not returned by the ingest tool",
	} {
		if !strings.Contains(response, want) {
			t.Fatalf("DocumentIngestResponse() missing %q:\n%s", want, response)
		}
	}
	for _, forbidden := range []string{"likely", "local_email_drafts", "PDFs with corrupted content"} {
		if strings.Contains(response, forbidden) {
			t.Fatalf("DocumentIngestResponse() included ungrounded text %q:\n%s", forbidden, response)
		}
	}
}

func TestSafeToolExecutorBlocksUngrantedExternalIngest(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	externalRoot := t.TempDir()
	docPath := filepath.Join(externalRoot, "outside.md")
	if err := os.WriteFile(docPath, []byte("External launch notes.\n"), 0o644); err != nil {
		t.Fatalf("write external doc: %v", err)
	}
	ragStore, err := rag.Open(ctx, filepath.Join(t.TempDir(), "rag.sqlite"))
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	defer ragStore.Close()

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
		RAG:             ragStore,
		Config:          config.Default(),
	})
	_, err = executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "ingest_documents",
		Command:   []string{"ingest_documents", docPath},
		RiskLevel: RiskLow,
	})
	if err == nil || !strings.Contains(err.Error(), "workspace path is not granted") {
		t.Fatalf("error = %v, want workspace grant error", err)
	}
}

func TestSafeToolExecutorIngestsGrantedExternalDocument(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	externalRoot := t.TempDir()
	docPath := filepath.Join(externalRoot, "outside.md")
	if err := os.WriteFile(docPath, []byte("External launch codename: River Clock\n"), 0o644); err != nil {
		t.Fatalf("write external doc: %v", err)
	}
	grantStore := safety.NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	if _, err := grantStore.Grant(externalRoot, "external-docs", "test"); err != nil {
		t.Fatalf("grant external root: %v", err)
	}
	ragStore, err := rag.Open(ctx, filepath.Join(t.TempDir(), "rag.sqlite"))
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	defer ragStore.Close()

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
		RAG:             ragStore,
		Config:          config.Default(),
		WorkspaceGrants: grantStore,
	})
	result, err := executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "ingest_documents",
		Command:   []string{"ingest_documents", docPath},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if result.SourceKind != "rag" || result.Status != "completed" {
		t.Fatalf("result = %+v, want completed RAG ingest", result)
	}
	hits, err := ragStore.Search(ctx, "River Clock", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("Search returned no hits after external ingest: %+v", result)
	}
}

func TestSafeToolExecutorReadsGrantedExternalFile(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	externalRoot := t.TempDir()
	docPath := filepath.Join(externalRoot, "notes.md")
	if err := os.WriteFile(docPath, []byte("Granted file content.\n"), 0o644); err != nil {
		t.Fatalf("write external doc: %v", err)
	}
	grantStore := safety.NewWorkspaceGrantStore(filepath.Join(t.TempDir(), "workspace_grants.json"))
	if _, err := grantStore.Grant(externalRoot, "external-docs", "test"); err != nil {
		t.Fatalf("grant external root: %v", err)
	}
	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   root,
		MaxContextChars: 2000,
		WorkspaceGrants: grantStore,
	})
	result, err := executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "read_file",
		Command:   []string{"read_file", docPath},
		RiskLevel: RiskLow,
	})
	if err != nil {
		t.Fatalf("executor error = %v", err)
	}
	if !strings.Contains(result.Context, "Granted file content.") {
		t.Fatalf("Context missing granted file content:\n%s", result.Context)
	}
}

func TestSafeToolExecutorRejectsUnapprovedMemoryWrite(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   t.TempDir(),
		MaxContextChars: 2000,
		Memory:          store,
	})
	_, err = executor(ctx, ExecutionDecision{
		Status:    ExecutionReady,
		ToolName:  "memory_write",
		Command:   []string{"memory_write", "follow_up", "Remember the release checklist."},
		RiskLevel: RiskMedium,
		Reason:    "model requested memory write",
	})
	if err == nil || !strings.Contains(err.Error(), "explicit approved permission") {
		t.Fatalf("executor error = %v, want approval requirement", err)
	}

	memories, err := store.ListMemories(ctx, 10)
	if err != nil {
		t.Fatalf("ListMemories() error = %v", err)
	}
	if len(memories) != 0 {
		t.Fatalf("ListMemories() len = %d, want no unapproved memory write", len(memories))
	}
}

func TestSafeToolExecutorRejectsMemoryWriteReasonContainingNotApproved(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()

	executor := NewSafeToolExecutor(SafeToolConfig{
		WorkspaceRoot:   t.TempDir(),
		MaxContextChars: 2000,
		Memory:          store,
	})
	_, err = executor(ctx, ExecutionDecision{
		Status:               ExecutionReady,
		RequestID:            "perm_not_approved",
		ToolName:             "memory_write",
		Command:              []string{"memory_write", "follow_up", "Remember the release checklist."},
		RiskLevel:            RiskMedium,
		Reason:               "not approved by the user",
		RequiresConfirmation: false,
	})
	if err == nil || !strings.Contains(err.Error(), "explicit approved permission") {
		t.Fatalf("executor error = %v, want approval requirement", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

func isPythonPytestCommand(command []string) bool {
	return len(command) == 3 &&
		(command[0] == "python" || command[0] == "python3") &&
		command[1] == "-m" &&
		command[2] == "pytest"
}
