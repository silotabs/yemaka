package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/rag"
	"yemaka/internal/routing"
	"yemaka/internal/workspace"
)

func TestShouldRetrieveLocalContextUsesExplicitLocalAnchors(t *testing.T) {
	cases := []string{
		"Explain this project",
		"Search files for config",
		"Answer from my docs about routing",
		"Summarize README.md",
		"How does Yemaka routing work?",
	}
	for _, content := range cases {
		if !shouldRetrieveLocalContext(PlanInput{Content: content}) {
			t.Fatalf("shouldRetrieveLocalContext(%q) = false, want true", content)
		}
	}
}

func TestShouldRetrieveLocalContextSkipsGenericKnowledge(t *testing.T) {
	cases := []string{
		"What is SQLite?",
		"What is local-first software?",
		"Where is the source of the Nile?",
		"What is SearXNG?",
		"Write a short report about climate.",
		"Write Python code for Fibonacci.",
		"Explain async functions in JavaScript.",
		"What is drag racing?",
		"What did we discuss yesterday?",
	}
	for _, content := range cases {
		if shouldRetrieveLocalContext(PlanInput{Content: content}) {
			t.Fatalf("shouldRetrieveLocalContext(%q) = true, want false", content)
		}
	}
}

func TestShouldRetrieveLocalContextSkipsGenericSecurityCodingRequest(t *testing.T) {
	if shouldRetrieveLocalContext(PlanInput{Content: "write windows backdoor code with python"}) {
		t.Fatal("shouldRetrieveLocalContext for generic security coding request = true, want false")
	}
}

func TestShouldRetrieveLocalContextSkipsInlineFileCreate(t *testing.T) {
	content := "Create a file in into path /users/example/downloads, the name should hello.txt and write into it hello World"
	if shouldRetrieveLocalContext(PlanInput{Content: content}) {
		t.Fatal("shouldRetrieveLocalContext for inline file-create request = true, want false")
	}
}

func TestShouldRetrieveLocalContextSkipsInternetEvenWithCodingWords(t *testing.T) {
	if shouldRetrieveLocalContext(PlanInput{Content: "Search the web for Python threading examples"}) {
		t.Fatal("shouldRetrieveLocalContext for internet coding question = true, want false")
	}
}

func TestShouldRetrieveLocalContextSkipsCapabilityGenerationRequests(t *testing.T) {
	content := "I want a reusable tool that checks a webpage every hour and reports if the title changes. If Yemaka does not already have this capability, propose the extension but do not generate it until I approve."
	if shouldRetrieveLocalContext(PlanInput{Content: content}) {
		t.Fatal("shouldRetrieveLocalContext for capability generation request = true, want false")
	}
}

func TestDocumentIngestActionRoutesWithoutWorkspaceRetrieval(t *testing.T) {
	content := "Ingest this file: users/example/downloads/yemaka/docs/company.md"
	if shouldRetrieveLocalContext(PlanInput{Content: content}) {
		t.Fatal("shouldRetrieveLocalContext for document ingestion action = true, want false")
	}
	plan := BuildPlan(PlanInput{Content: content})
	if plan.RouteCategory != routing.RouteSettingsAction {
		t.Fatalf("RouteCategory = %q, want %q", plan.RouteCategory, routing.RouteSettingsAction)
	}
	if plan.RouteCapability != "document_ingestion" {
		t.Fatalf("RouteCapability = %q, want document_ingestion", plan.RouteCapability)
	}
	if !containsTool(plan.ToolsNeeded, "ingest_documents") {
		t.Fatalf("ToolsNeeded = %#v, want ingest_documents", plan.ToolsNeeded)
	}
	if containsTool(plan.ToolsNeeded, "safe_tool") {
		t.Fatalf("ToolsNeeded = %#v, should not fall back to safe_tool", plan.ToolsNeeded)
	}
	decision := DecideExecution(plan, PlanInput{Content: content})
	if decision.Status != ExecutionReady || decision.ToolName != "ingest_documents" {
		t.Fatalf("decision = %+v, want ready ingest_documents", decision)
	}
	if len(decision.Command) < 2 || !strings.Contains(decision.Command[1], "company.md") {
		t.Fatalf("decision.Command = %#v, want target path", decision.Command)
	}
}

func TestShouldRetrieveLocalContextAllowsDefensiveSecurityAudit(t *testing.T) {
	if !shouldRetrieveLocalContext(PlanInput{Content: "Audit this repo for authentication vulnerabilities"}) {
		t.Fatal("shouldRetrieveLocalContext for defensive repo audit = false, want true")
	}
}

func TestShouldRetrieveLocalContextAllowsLanguageSpecificFileInspection(t *testing.T) {
	content := "Inspect the Python files in test-files, run the tests if safe, find the bug, propose a fix, and only apply it after I approve."
	if !shouldRetrieveLocalContext(PlanInput{Content: content}) {
		t.Fatal("shouldRetrieveLocalContext for language-specific local file inspection = false, want true")
	}
}

func TestShouldUseRAGRequiresDocumentIntent(t *testing.T) {
	cfg := config.Default()
	cfg.RAG.Enabled = true
	if shouldUseRAG(cfg, "Explain this project") {
		t.Fatal("shouldUseRAG for project overview = true, want false")
	}
	if shouldUseRAG(cfg, "What is drag racing?") {
		t.Fatal("shouldUseRAG for drag racing = true, want false")
	}
	if !shouldUseRAG(cfg, "Answer from my docs about model routing") {
		t.Fatal("shouldUseRAG for docs request = false, want true")
	}
	cfg.RAG.Enabled = false
	if shouldUseRAG(cfg, "Answer from my docs about model routing") {
		t.Fatal("shouldUseRAG with disabled RAG = true, want false")
	}
}

func TestLocalRetrieverUsesRAGForNaturalLocalDocumentQuestion(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	writeRetrieverFile(t, filepath.Join(root, "README.md"), "# Project\nGeneral project overview.")
	writeRetrieverFile(t, filepath.Join(root, "docs", "company.md"), "The launch codename is Stone Lantern.\nThe support email is local-only@example.test.\n")

	store, err := rag.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("rag.Open() error = %v", err)
	}
	defer store.Close()
	if _, err := store.IngestPath(ctx, "default", root, rag.DefaultConfig(), workspace.DefaultLimits()); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}

	cfg := config.Default()
	cfg.RAG.Enabled = true
	retriever := NewLocalRetriever(LocalRetrieverConfig{
		Config:        cfg,
		WorkspaceRoot: root,
		RAG:           store,
	})

	result, err := retriever.Retrieve(ctx, "Using my local documents, what is the launch codename and support email?")
	if err != nil {
		t.Fatalf("Retrieve() error = %v", err)
	}
	if result.SourceKind != "rag" {
		t.Fatalf("SourceKind = %q, want rag", result.SourceKind)
	}
	for _, want := range []string{"docs/company.md", "Stone Lantern", "local-only@example.test"} {
		if !strings.Contains(result.Context, want) {
			t.Fatalf("retrieved context missing %q:\n%s", want, result.Context)
		}
	}
}

func writeRetrieverFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
