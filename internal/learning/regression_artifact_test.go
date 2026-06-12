package learning

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/replay"
)

func TestSaveRegressionArtifactsFromTraceWritesRedactedArtifact(t *testing.T) {
	trace := replay.NewTrace("Search docs with token=super-secret-value")
	trace.ID = "trace-token-secret"
	trace.Route = replay.RouteSnapshot{Category: "internet_search", RiskLevel: "medium"}
	trace.PermissionsRequested = []replay.PermissionRequest{{
		ToolName: "internet_search",
		Status:   "denied",
		Reason:   "internet disabled for token=super-secret-value",
	}}

	artifacts, err := SaveRegressionArtifactsFromTrace(t.TempDir(), trace)
	if err != nil {
		t.Fatalf("SaveRegressionArtifactsFromTrace() error = %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("SaveRegressionArtifactsFromTrace() wrote %d artifacts, want 1", len(artifacts))
	}

	data, err := os.ReadFile(artifacts[0].Path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if strings.Contains(text, "super-secret-value") {
		t.Fatalf("artifact leaked secret:\n%s", text)
	}
	if !strings.Contains(text, `token=[REDACTED]`) {
		t.Fatalf("artifact missing redaction marker:\n%s", text)
	}
	if !strings.HasSuffix(text, "\n") {
		t.Fatalf("artifact should end with newline: %q", text)
	}
}

func TestSaveRegressionArtifactsFromRealTraceFailuresAreStableLocalAndRedacted(t *testing.T) {
	commonForbidden := []string{
		"/Users/example/",
		"abcdefghijklmnop",
		"ghp_abcdefghijklmnop",
	}
	tests := []struct {
		name  string
		trace replay.Trace
		kind  string
		want  []string
	}{
		{
			name:  "routing",
			trace: artifactRoutingTrace(),
			kind:  FailureKindRouting,
			want: []string{
				`kind: "routing_failure"`,
				`source_trace_id: "trace-routing-[REDACTED]"`,
				`prompt: "Explain ~/Downloads/yemaka release flow with Authorization: [REDACTED]"`,
				`route: "chat_explanation"`,
				`should_use_internet: false`,
				`should_create_scheduler_job: false`,
				`  - "scheduler_create"`,
			},
		},
		{
			name:  "internet search",
			trace: artifactInternetSearchTrace(),
			kind:  FailureKindInternetSearch,
			want: []string{
				`kind: "internet_search_failure"`,
				`route: "internet_search"`,
				`should_use_internet: true`,
				`should_ask_approval: true`,
				`  - "internet_search"`,
			},
		},
		{
			name:  "extension generation",
			trace: artifactExtensionGenerationTrace(),
			kind:  FailureKindExtensionGeneration,
			want: []string{
				`kind: "extension_generation_failure"`,
				`route: "extension_generate"`,
				`should_ask_approval: true`,
				`should_generate_extension: true`,
				`  - "extension_generate"`,
			},
		},
		{
			name:  "document ingestion",
			trace: artifactDocumentIngestionTrace(),
			kind:  FailureKindDocumentIngestion,
			want: []string{
				`kind: "document_ingestion_failure"`,
				`route: "rag_search"`,
				`should_read_file: true`,
				`  - "pdf_extract"`,
				`~/Documents/Specs/private.pdf`,
			},
		},
		{
			name:  "rag retrieval",
			trace: artifactRAGRetrievalTrace(),
			kind:  FailureKindRAGRetrieval,
			want: []string{
				`kind: "rag_retrieval_failure"`,
				`route: "rag_search"`,
				`should_read_file: true`,
				`  - "rag_search"`,
			},
		},
		{
			name:  "ui flow",
			trace: artifactUIFlowTrace(),
			kind:  FailureKindUIFlow,
			want: []string{
				`kind: "ui_flow_failure"`,
				`route: "chat_explanation"`,
				`  - "ui"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			firstDir := t.TempDir()
			secondDir := t.TempDir()
			firstArtifacts, err := SaveRegressionArtifactsFromTrace(firstDir, tt.trace)
			if err != nil {
				t.Fatalf("first SaveRegressionArtifactsFromTrace() error = %v", err)
			}
			secondArtifacts, err := SaveRegressionArtifactsFromTrace(secondDir, tt.trace)
			if err != nil {
				t.Fatalf("second SaveRegressionArtifactsFromTrace() error = %v", err)
			}

			first := artifactByKind(t, firstArtifacts, tt.kind)
			second := artifactByKind(t, secondArtifacts, tt.kind)
			assertArtifactPathInside(t, firstDir, first.Path)
			assertArtifactPathInside(t, secondDir, second.Path)
			if filepath.Base(first.Path) != filepath.Base(second.Path) {
				t.Fatalf("artifact filename changed: %q vs %q", first.Path, second.Path)
			}

			firstText := readArtifactText(t, first.Path)
			secondText := readArtifactText(t, second.Path)
			if firstText != secondText {
				t.Fatalf("artifact content changed between writes:\nfirst:\n%s\nsecond:\n%s", firstText, secondText)
			}
			if !strings.HasSuffix(firstText, "\n") {
				t.Fatalf("artifact should end with newline: %q", firstText)
			}
			for _, want := range tt.want {
				if !strings.Contains(firstText, want) {
					t.Fatalf("artifact missing %q:\n%s", want, firstText)
				}
			}
			for _, forbidden := range commonForbidden {
				if strings.Contains(firstText, forbidden) {
					t.Fatalf("artifact leaked %q:\n%s", forbidden, firstText)
				}
			}
		})
	}
}

func TestSaveRegressionArtifactsKeepsPathsInsideOutputDirectory(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(filepath.Dir(dir), "outside.yml")
	_ = os.Remove(outside)

	artifacts, err := SaveRegressionArtifacts(dir, []RegressionTestCase{{
		Name:   "../../outside",
		Kind:   FailureKindToolCall,
		Prompt: "Run tests",
		Failure: RegressionFailure{
			Stage:   "tool",
			Subject: "run_tests",
			Message: "exit status 1",
		},
	}})
	if err != nil {
		t.Fatalf("SaveRegressionArtifacts() error = %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("SaveRegressionArtifacts() wrote %d artifacts, want 1", len(artifacts))
	}
	if !strings.HasPrefix(artifacts[0].Path, filepath.Clean(dir)+string(os.PathSeparator)) {
		t.Fatalf("artifact path %q is outside dir %q", artifacts[0].Path, dir)
	}
	if strings.Contains(filepath.Base(artifacts[0].Path), "..") {
		t.Fatalf("artifact filename was not sanitized: %q", artifacts[0].Path)
	}
	if _, err := os.Stat(outside); !os.IsNotExist(err) {
		t.Fatalf("outside path exists or stat failed unexpectedly: %v", err)
	}
}

func TestSaveRegressionArtifactsRejectsRemoteOutputDirectory(t *testing.T) {
	testCase := RegressionTestCase{
		Kind:   FailureKindUnknown,
		Prompt: "Replay failure locally",
		Failure: RegressionFailure{
			Stage:   "tool",
			Message: "failed",
		},
	}
	for _, dir := range []string{
		"https://example.com/regressions",
		"s3://bucket/regressions",
		"git@github.com:example/yemaka-regressions",
	} {
		if _, err := SaveRegressionArtifacts(dir, []RegressionTestCase{testCase}); err == nil {
			t.Fatalf("SaveRegressionArtifacts(%q) error = nil, want local path rejection", dir)
		}
	}
}

func TestSaveRegressionArtifactsStableContent(t *testing.T) {
	testCase := RegressionTestCase{
		Name:   "stable-case",
		Kind:   FailureKindRouting,
		Prompt: "Explain website monitoring",
		Failure: RegressionFailure{
			Stage:   "routing",
			Code:    "false_positive_action",
			Message: "created scheduler job",
		},
		Expected: RegressionExpected{
			Route:          "chat_explanation",
			RiskLevel:      "low",
			ForbiddenTools: []string{"scheduler_create"},
		},
	}

	firstDir := t.TempDir()
	secondDir := t.TempDir()
	first, err := SaveRegressionArtifacts(firstDir, []RegressionTestCase{testCase})
	if err != nil {
		t.Fatalf("first SaveRegressionArtifacts() error = %v", err)
	}
	second, err := SaveRegressionArtifacts(secondDir, []RegressionTestCase{testCase})
	if err != nil {
		t.Fatalf("second SaveRegressionArtifacts() error = %v", err)
	}

	firstData, err := os.ReadFile(first[0].Path)
	if err != nil {
		t.Fatalf("read first artifact: %v", err)
	}
	secondData, err := os.ReadFile(second[0].Path)
	if err != nil {
		t.Fatalf("read second artifact: %v", err)
	}
	if string(firstData) != string(secondData) {
		t.Fatalf("artifact content changed between writes:\nfirst:\n%s\nsecond:\n%s", firstData, secondData)
	}
	if filepath.Base(first[0].Path) != filepath.Base(second[0].Path) {
		t.Fatalf("artifact filename changed: %q vs %q", first[0].Path, second[0].Path)
	}
}

func TestSaveRegressionArtifactsRejectsEmptyDirectory(t *testing.T) {
	if _, err := SaveRegressionArtifacts(" ", []RegressionTestCase{{Kind: FailureKindUnknown, Prompt: "x"}}); err == nil {
		t.Fatal("SaveRegressionArtifacts() error = nil, want empty directory error")
	}
}

func artifactRoutingTrace() replay.Trace {
	trace := replay.NewTrace("Explain /Users/example/Downloads/yemaka release flow with Authorization: Bearer abcdefghijklmnop")
	trace.ID = "trace-routing-ghp_abcdefghijklmnop"
	trace.Route = replay.RouteSnapshot{
		Category:                 "scheduler_create",
		RiskLevel:                "medium",
		ShouldUseTool:            true,
		ShouldCreateSchedulerJob: true,
	}
	trace.Plan = replay.PlanSnapshot{
		ToolsNeeded: []string{"scheduler_create"},
	}
	trace.ToolsCalled = []replay.ToolCall{{
		Name:   "scheduler_create",
		Status: "completed",
	}}
	trace.Errors = []replay.TraceError{{
		Stage:             "routing",
		Code:              "false_positive_action",
		Subject:           "scheduler_create",
		Message:           "explanation request created a scheduler job",
		ExpectedRoute:     "chat_explanation",
		ExpectedRiskLevel: "low",
	}}
	return trace
}

func artifactInternetSearchTrace() replay.Trace {
	trace := replay.NewTrace("Find current SearXNG provider status")
	trace.ID = "trace-internet-search"
	trace.Route = replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseTool:     true,
		ShouldUseInternet: true,
		ShouldAskApproval: true,
	}
	trace.ToolsCalled = []replay.ToolCall{{
		Name:   "internet_search",
		Status: "failed",
		Error:  "SearXNG provider not configured",
	}}
	return trace
}

func artifactExtensionGenerationTrace() replay.Trace {
	trace := replay.NewTrace("Generate a CSV cleanup extension")
	trace.ID = "trace-extension-generation"
	trace.Route = replay.RouteSnapshot{
		Category:                "extension_generate",
		RiskLevel:               "medium",
		ShouldUseTool:           true,
		ShouldGenerateExtension: true,
		ShouldAskApproval:       true,
	}
	trace.ToolsCalled = []replay.ToolCall{{
		Name:   "extension_generate",
		Status: "failed",
		Error:  "manifest validation failed before registration",
	}}
	return trace
}

func artifactDocumentIngestionTrace() replay.Trace {
	trace := replay.NewTrace("Ingest /Users/example/Documents/Specs/private.pdf")
	trace.ID = "trace-document-ingestion"
	trace.Route = replay.RouteSnapshot{
		Category:       "rag_search",
		RiskLevel:      "low",
		ShouldUseTool:  true,
		ShouldReadFile: true,
	}
	trace.ToolsCalled = []replay.ToolCall{{
		Name:   "pdf_extract",
		Status: "failed",
		Error:  "unsupported PDF encoding in /Users/example/Documents/Specs/private.pdf",
	}}
	return trace
}

func artifactRAGRetrievalTrace() replay.Trace {
	trace := replay.NewTrace("Answer from project docs")
	trace.ID = "trace-rag-retrieval"
	trace.Route = replay.RouteSnapshot{
		Category:       "rag_search",
		RiskLevel:      "low",
		ShouldUseTool:  true,
		ShouldReadFile: true,
	}
	trace.ToolsCalled = []replay.ToolCall{{
		Name:   "rag_search",
		Status: "failed",
		Error:  "retrieval returned zero chunks for the local fixture",
	}}
	return trace
}

func artifactUIFlowTrace() replay.Trace {
	trace := replay.NewTrace("Retry the last chat response")
	trace.ID = "trace-ui-flow"
	trace.Route = replay.RouteSnapshot{
		Category:  "chat_explanation",
		RiskLevel: "low",
	}
	trace.Errors = []replay.TraceError{{
		Stage:   "ui",
		Code:    "stream_render_failed",
		Subject: "chat_stream",
		Message: "frontend stream stalled after reconnect",
	}}
	return trace
}

func artifactByKind(t *testing.T, artifacts []RegressionArtifact, kind string) RegressionArtifact {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact.Case.Kind == kind {
			return artifact
		}
	}
	t.Fatalf("artifacts = %+v, want kind %q", artifacts, kind)
	return RegressionArtifact{}
}

func assertArtifactPathInside(t *testing.T, dir string, path string) {
	t.Helper()
	cleanDir := filepath.Clean(dir)
	if !strings.HasPrefix(path, cleanDir+string(os.PathSeparator)) {
		t.Fatalf("artifact path %q is outside dir %q", path, cleanDir)
	}
	name := filepath.Base(path)
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\:`) || !strings.HasSuffix(name, ".yml") {
		t.Fatalf("artifact filename is not path-safe: %q", name)
	}
}

func readArtifactText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}
