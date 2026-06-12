package evaluation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"time"

	"yemaka/internal/capabilities"
	"yemaka/internal/config"
	contextcore "yemaka/internal/context"
	"yemaka/internal/domainpacks"
	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/modelprofiles"
	"yemaka/internal/models"
	"yemaka/internal/notifications"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
	"yemaka/internal/safety"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
	"yemaka/internal/workflows"
	"yemaka/internal/workspace"
)

type RunOptions struct {
	Mode      string
	Model     string
	SkipModel bool
}

type Runner struct {
	Config  *config.Config
	Profile *profiles.Profile
	Runtime models.Runtime
}

type runState struct {
	workspace     string
	docs          string
	snapshots     string
	database      string
	modelProfiles string
}

func (r Runner) Run(ctx context.Context, options RunOptions) (Report, error) {
	if r.Config == nil {
		return Report{}, fmt.Errorf("config is required")
	}
	if r.Profile == nil {
		return Report{}, fmt.Errorf("profile is required")
	}
	mode := strings.TrimSpace(options.Mode)
	if mode == "" {
		mode = "low-memory"
	}
	model := strings.TrimSpace(options.Model)
	if model == "" {
		model = selectedModelName(r.Config, mode)
	}

	start := time.Now()
	previous := loadPreviousReport(r.Profile)
	report := Report{
		ID:        newReportID(),
		Mode:      mode,
		Model:     model,
		StartedAt: start.UTC().Format(time.RFC3339Nano),
	}
	state, err := r.prepare(report.ID)
	if err != nil {
		return Report{}, err
	}

	report.add(measure("startup_time", func() TaskResult {
		return passWithMetrics("eval runner initialized", map[string]any{
			"startup_ms": time.Since(start).Milliseconds(),
		})
	}))
	report.add(measure("ram_usage_sample", func() TaskResult {
		return evalRAMUsageSample()
	}))
	report.add(measure("profile_layout", func() TaskResult {
		return r.evalProfileLayout()
	}))
	report.add(measure("notification_center_local_inbox", func() TaskResult {
		return evalNotificationCenterLocalInbox(r.Profile, report.ID)
	}))
	report.add(measure("sqlite_memory_fts", func() TaskResult {
		return evalSQLiteMemoryFTS(ctx, state.database)
	}))
	report.add(measure("rag_keyword_hit", func() TaskResult {
		return evalRAGHit(ctx, state.database, state.docs)
	}))
	report.add(measure("snapshot_rollback", func() TaskResult {
		return evalRollback(ctx, state.workspace, state.snapshots)
	}))
	report.add(measure("dangerous_command_block", func() TaskResult {
		return evalDangerousCommandBlock()
	}))
	report.add(measure("safe_command_allowlist", func() TaskResult {
		return evalSafeCommandAllowlist()
	}))
	report.add(measure("skill_reuse_rate", func() TaskResult {
		return evalSkillReuse(r.Profile)
	}))
	report.add(measure("routing_matrix_behavior", func() TaskResult {
		return evalRoutingMatrixBehavior()
	}))
	report.add(measure("real_world_route_smoke", func() TaskResult {
		return evalRealWorldRouteSmoke()
	}))
	report.add(measure("conversation_route_continuity", func() TaskResult {
		return evalConversationRouteContinuity()
	}))
	report.add(measure("route_governance_classifier", func() TaskResult {
		return evalRouteGovernanceClassifier()
	}))
	report.add(measure("qa_review_regression_ledger", func() TaskResult {
		return evalQAReviewRegressionLedger(r.Profile)
	}))
	report.add(measure("context_compiler_low_memory", func() TaskResult {
		return evalContextCompilerLowMemoryBehavior()
	}))
	report.add(measure("capability_registry_gap", func() TaskResult {
		return evalCapabilityRegistryGapBehavior()
	}))
	report.add(measure("domain_pack_template_awareness", func() TaskResult {
		return evalDomainPackTemplateAwareness()
	}))
	report.add(measure("model_profile_artifact_safety", func() TaskResult {
		return evalModelProfileArtifactSafety(state.modelProfiles)
	}))
	if options.SkipModel {
		report.add(TaskResult{Name: "model_readiness", Status: StatusSkip, Details: "model checks skipped by request"})
		report.add(TaskResult{Name: "model_chat_smoke", Status: StatusSkip, Details: "model checks skipped by request"})
	} else {
		report.add(measure("model_readiness", func() TaskResult {
			return r.evalModelReadiness(ctx, model)
		}))
		report.add(measure("model_chat_smoke", func() TaskResult {
			return r.evalModelChatSmoke(ctx, model)
		}))
	}

	completed := time.Now()
	report.CompletedAt = completed.UTC().Format(time.RFC3339Nano)
	report.DurationMS = completed.Sub(start).Milliseconds()
	report.Metrics = aggregateMetrics(report, previous)
	return Save(r.Profile, report)
}

func (r Runner) prepare(reportID string) (runState, error) {
	root := filepath.Join(ReportsRoot(r.Profile), "runs", reportID)
	state := runState{
		workspace:     filepath.Join(root, "workspace"),
		docs:          filepath.Join(root, "docs"),
		snapshots:     filepath.Join(root, "snapshots"),
		database:      filepath.Join(root, "eval.sqlite"),
		modelProfiles: filepath.Join(root, "model_profiles"),
	}
	for _, dir := range []string{state.workspace, state.docs, state.snapshots, state.modelProfiles} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return runState{}, fmt.Errorf("create eval fixture directory: %w", err)
		}
	}
	fixtures := map[string]string{
		filepath.Join(state.docs, "model-routing.md"): "Yemaka model routing chooses low-memory local models for small machines. SQLite FTS5 RAG keeps retrieval lightweight.",
		filepath.Join(state.docs, "safety.md"):        "Yemaka blocks dangerous shell commands and snapshots workspace files before edits.",
		filepath.Join(state.workspace, "notes.md"):    "before rollback\n",
	}
	for path, content := range fixtures {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return runState{}, fmt.Errorf("write eval fixture: %w", err)
		}
	}
	return state, nil
}

func (r Runner) evalProfileLayout() TaskResult {
	required := []string{
		r.Profile.Root,
		filepath.Dir(r.Profile.Database),
		r.Profile.Sessions,
		r.Profile.Skills,
		r.Profile.RAG,
		r.Profile.Logs,
		r.Profile.Notifications,
		r.Profile.Snapshots,
		r.Profile.Evals,
		r.Profile.Permissions,
	}
	for _, path := range required {
		info, err := os.Stat(path)
		if err != nil {
			return fail("missing profile path: " + path)
		}
		if !info.IsDir() {
			return fail("profile path is not a directory: " + path)
		}
	}
	return pass("profile directories are ready")
}

func evalNotificationCenterLocalInbox(profile *profiles.Profile, runID string) TaskResult {
	if profile == nil {
		return fail("profile is required")
	}
	if strings.TrimSpace(profile.Notifications) == "" {
		return fail("profile notifications directory is required")
	}
	if info, err := os.Stat(profile.Notifications); err == nil && !info.IsDir() {
		return fail("profile notifications path is not a directory: " + profile.Notifications)
	} else if err != nil && !os.IsNotExist(err) {
		return fail("stat notification inbox directory: " + err.Error())
	}

	store := notifications.NewStore(profile.Notifications)
	store.Now = func() time.Time { return time.Date(2026, 5, 18, 10, 0, 0, 0, time.UTC) }
	item, err := store.Record(notifications.Notification{
		ID:       "eval_" + runID,
		Type:     "evaluation",
		Severity: notifications.SeverityInfo,
		Title:    "Evaluation notification redacts sk-abcdefghijklmnopqrstuvwxyz123456",
		Message:  "Local inbox check redacts ghp_abcdefghijklmnopqrstuvwxyz123456 before persistence.",
		Source:   "evaluation",
		Metadata: map[string]string{
			"api_key": "AKIA1234567890ABCDEF",
		},
	})
	if err != nil {
		return fail("record local notification: " + err.Error())
	}
	items, err := store.List(10, true)
	if err != nil {
		return fail("list local notifications: " + err.Error())
	}
	found := false
	for _, listed := range items {
		if listed.ID == item.ID {
			found = true
			break
		}
	}
	if !found {
		return fail("recorded local notification was not listed")
	}

	inboxPath := filepath.Join(profile.Notifications, notifications.InboxFile)
	data, err := os.ReadFile(inboxPath)
	if err != nil {
		return fail("read local notification inbox: " + err.Error())
	}
	combined := item.Title + "\n" + item.Message + "\n" + item.Metadata["api_key"] + "\n" + string(data)
	for _, leaked := range []string{
		"sk-abcdefghijklmnopqrstuvwxyz123456",
		"ghp_abcdefghijklmnopqrstuvwxyz123456",
		"AKIA1234567890ABCDEF",
	} {
		if strings.Contains(combined, leaked) {
			return failWithMetrics("local notification leaked secret-like value", map[string]any{
				"notification_secret_redaction_rate": 0.0,
			})
		}
	}
	return passWithMetrics("local notification inbox is writable and redacts secret-like content", map[string]any{
		"notification_inbox_records_checked": 1,
		"notification_secret_redaction_rate": 1.0,
	})
}

func evalSQLiteMemoryFTS(ctx context.Context, database string) TaskResult {
	store, err := memory.Open(ctx, database)
	if err != nil {
		return fail(err.Error())
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "Eval memory")
	if err != nil {
		return fail(err.Error())
	}
	if _, err := store.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "SQLite FTS5 retrieval helps a small local model remember prior work.",
		Model:          "eval",
	}); err != nil {
		return fail(err.Error())
	}
	results, err := store.SearchMessages(ctx, "SQLite FTS5 retrieval", 3)
	if err != nil {
		return fail(err.Error())
	}
	if len(results) == 0 {
		return fail("expected memory FTS match")
	}
	return passWithMetrics("memory FTS returned matches", map[string]any{"matches": len(results)})
}

func evalRAMUsageSample() TaskResult {
	var stats goruntime.MemStats
	goruntime.ReadMemStats(&stats)
	return passWithMetrics("runtime memory sampled", map[string]any{
		"heap_alloc_bytes":  int64(stats.HeapAlloc),
		"heap_sys_bytes":    int64(stats.HeapSys),
		"sys_bytes":         int64(stats.Sys),
		"garbage_collector": int64(stats.NumGC),
	})
}

func evalRAGHit(ctx context.Context, database string, docs string) TaskResult {
	store, err := rag.Open(ctx, database)
	if err != nil {
		return fail(err.Error())
	}
	defer store.Close()
	result, err := store.IngestPath(ctx, "eval", docs, rag.DefaultConfig(), workspace.Limits{
		MaxFilesScanned:   20,
		MaxFileBytes:      200000,
		MaxTotalScanBytes: 1000000,
		MaxSearchResults:  10,
		MaxContextFiles:   4,
		MaxContextChars:   4000,
	})
	if err != nil {
		return fail(err.Error())
	}
	matches, err := store.Search(ctx, "model routing low memory", 5)
	if err != nil {
		return fail(err.Error())
	}
	for index, match := range matches {
		if strings.Contains(match.Path, "model-routing.md") {
			rank := index + 1
			return passWithMetrics("RAG search found model-routing.md", map[string]any{
				"files_indexed":      result.FilesIndexed,
				"chunks_created":     result.ChunksCreated,
				"matches":            len(matches),
				"expected_path":      "model-routing.md",
				"rank":               rank,
				"top_k":              5,
				"retrieval_accuracy": 1.0,
				"reciprocal_rank":    1.0 / float64(rank),
			})
		}
	}
	return failWithMetrics("expected RAG hit for model-routing.md", map[string]any{
		"matches":            len(matches),
		"expected_path":      "model-routing.md",
		"top_k":              5,
		"retrieval_accuracy": 0.0,
		"reciprocal_rank":    0.0,
	})
}

func evalRollback(ctx context.Context, root string, snapshots string) TaskResult {
	plan, err := workspace.PlanWrite(ctx, root, "notes.md", "after edit\n", workspace.WriteOptions{
		SnapshotsRoot:       snapshots,
		SnapshotBeforeWrite: true,
		MaxEditFileBytes:    200000,
	})
	if err != nil {
		return fail(err.Error())
	}
	applied, err := workspace.ApplyWrite(ctx, root, plan, workspace.WriteOptions{
		SnapshotsRoot:       snapshots,
		SnapshotBeforeWrite: true,
		MaxEditFileBytes:    200000,
	})
	if err != nil {
		return fail(err.Error())
	}
	if applied.SnapshotID == "" {
		return fail("expected snapshot before write")
	}
	manager := safety.NewSnapshotManager(snapshots)
	if _, err := manager.Rollback(ctx, "last"); err != nil {
		return fail(err.Error())
	}
	data, err := os.ReadFile(filepath.Join(root, "notes.md"))
	if err != nil {
		return fail(err.Error())
	}
	if string(data) != "before rollback\n" {
		return fail("rollback did not restore original content")
	}
	return passWithMetrics("snapshot rollback restored file", map[string]any{
		"snapshot_id":           applied.SnapshotID,
		"rollback_attempts":     1,
		"rollback_successes":    1,
		"rollback_success_rate": 1.0,
	})
}

func evalDangerousCommandBlock() TaskResult {
	policy := tools.CommandPolicy{WorkspaceRoot: ".", RequireConfirmationRisky: true}
	rm := tools.AnalyzeCommand("rm -rf .", policy)
	chained := tools.AnalyzeCommand("go test ./... && rm -rf .", policy)
	if rm.Allowed || rm.RiskLevel != tools.RiskBlocked {
		return failWithMetrics("rm command was not blocked", map[string]any{
			"dangerous_commands_checked": 2,
			"dangerous_commands_blocked": 0,
			"prevention_rate":            0.0,
		})
	}
	if chained.Allowed || chained.RiskLevel != tools.RiskBlocked {
		return failWithMetrics("shell control command was not blocked", map[string]any{
			"dangerous_commands_checked": 2,
			"dangerous_commands_blocked": 1,
			"prevention_rate":            0.5,
		})
	}
	return passWithMetrics("dangerous commands are blocked", map[string]any{
		"dangerous_commands_checked": 2,
		"dangerous_commands_blocked": 2,
		"prevention_rate":            1.0,
	})
}

func evalSafeCommandAllowlist() TaskResult {
	policy := tools.CommandPolicy{WorkspaceRoot: ".", RequireConfirmationRisky: true}
	analysis := tools.AnalyzeCommand("go test ./...", policy)
	if !analysis.Allowed || analysis.RiskLevel != tools.RiskLow {
		return fail("go test ./... should be allowed")
	}
	return passWithMetrics("safe verification command is allowed", map[string]any{
		"safe_commands_checked": 1,
		"safe_commands_allowed": 1,
		"allow_rate":            1.0,
	})
}

func evalSkillReuse(profile *profiles.Profile) TaskResult {
	if profile == nil {
		return fail("profile is required")
	}
	registry, err := skills.LoadRegistry(defaultSkillDirs(profile))
	if err != nil {
		return fail(err.Error())
	}
	cases := []struct {
		request string
		want    string
	}{
		{request: "Explain this project structure", want: "project_explainer"},
		{request: "Summarize this document", want: "document_summary"},
		{request: "Review the diff", want: "git_diff_review"},
		{request: "Fix this pytest traceback", want: "python_bugfix"},
		{request: "Code review this file", want: "code_review"},
	}
	matched := 0
	for _, item := range cases {
		skill, ok := registry.Select(item.request, evalSkillTools())
		if ok && skill.Name == item.want {
			matched++
		}
	}
	rate := float64(matched) / float64(len(cases))
	metrics := map[string]any{
		"skill_requests":   len(cases),
		"skill_matches":    matched,
		"skill_reuse_rate": rate,
		"valid_skills":     len(registry.Skills),
		"invalid_skills":   len(registry.Invalid),
	}
	if matched != len(cases) {
		return failWithMetrics("skill router missed one or more reusable workflows", metrics)
	}
	return passWithMetrics("skill router selected reusable workflows", metrics)
}

func evalRoutingMatrixBehavior() TaskResult {
	cases := []struct {
		name                  string
		prompt                string
		wantTask              string
		wantRoute             string
		wantCapability        string
		wantTools             []string
		notTools              []string
		wantWorkspace         bool
		wantRAG               bool
		wantInternet          bool
		checkWorkspace        bool
		checkRAG              bool
		checkInternet         bool
		wantApproval          bool
		wantWriteFiles        bool
		wantGenerateExtension bool
		wantSchedulerJob      bool
		minConfidence         int
	}{
		{
			name:           "local docs route to rag",
			prompt:         "Use my local documents to explain extension rollback.",
			wantTask:       routing.TaskRAG,
			wantRoute:      routing.RouteRAGSearch,
			wantCapability: "rag",
			wantTools:      []string{"rag_search"},
			notTools:       []string{"internet_search", "safe_tool"},
			wantWorkspace:  true,
			wantRAG:        true,
			checkWorkspace: true,
			checkRAG:       true,
			checkInternet:  true,
			minConfidence:  90,
		},
		{
			name:           "internet research stays explicit",
			prompt:         "Search the web for SearXNG documentation.",
			wantTask:       routing.TaskTool,
			wantRoute:      routing.RouteInternetSearch,
			wantCapability: "internet",
			wantTools:      []string{"internet_search"},
			notTools:       []string{"rag_search", "safe_tool"},
			wantInternet:   true,
			checkWorkspace: true,
			checkRAG:       true,
			checkInternet:  true,
			minConfidence:  80,
		},
		{
			name:                  "missing capability proposes extension path",
			prompt:                "Yemaka has a missing capability for importing CSV invoices; propose the extension but do not generate it until I approve.",
			wantTask:              routing.TaskTool,
			wantRoute:             routing.RouteExtensionGenerate,
			wantCapability:        "extension_generation",
			wantTools:             []string{"safe_tool"},
			notTools:              []string{"internet_fetch", "internet_search", "read_file", "search_files"},
			checkInternet:         true,
			wantApproval:          true,
			wantGenerateExtension: true,
			minConfidence:         80,
		},
		{
			name:           "file writes remain approval gated",
			prompt:         "Edit README.md to add a troubleshooting note.",
			wantTask:       routing.TaskTool,
			wantRoute:      routing.RouteFileWrite,
			wantCapability: "filesystem_write",
			wantTools:      []string{"edit_file"},
			wantApproval:   true,
			wantWriteFiles: true,
			minConfidence:  80,
		},
		{
			name:             "scheduler creation remains optional",
			prompt:           "Create a scheduled job that runs every hour to summarize release notes.",
			wantTask:         routing.TaskTool,
			wantRoute:        routing.RouteSchedulerCreate,
			wantCapability:   "scheduler",
			wantTools:        []string{"safe_tool"},
			wantApproval:     true,
			wantSchedulerJob: true,
			minConfidence:    80,
		},
		{
			name:          "explanation does not trigger action",
			prompt:        "Explain how website monitoring works.",
			wantTask:      routing.TaskReasoning,
			wantRoute:     routing.RouteChatExplanation,
			notTools:      []string{"safe_tool", "internet_fetch", "internet_search"},
			checkInternet: true,
			minConfidence: 70,
		},
	}

	passed := 0
	for _, tc := range cases {
		got := routing.Classify(routing.Request{Content: tc.prompt})
		if got.TaskType != tc.wantTask {
			return failWithMetrics(fmt.Sprintf("%s task = %q, want %q", tc.name, got.TaskType, tc.wantTask), evalCaseMetrics(len(cases), passed))
		}
		if got.RouteCategory != tc.wantRoute {
			return failWithMetrics(fmt.Sprintf("%s route = %q, want %q", tc.name, got.RouteCategory, tc.wantRoute), evalCaseMetrics(len(cases), passed))
		}
		if got.Capability != tc.wantCapability {
			return failWithMetrics(fmt.Sprintf("%s capability = %q, want %q", tc.name, got.Capability, tc.wantCapability), evalCaseMetrics(len(cases), passed))
		}
		if (tc.checkWorkspace && got.UseWorkspace != tc.wantWorkspace) || (tc.checkRAG && got.UseRAG != tc.wantRAG) || (tc.checkInternet && got.UseInternet != tc.wantInternet) {
			return failWithMetrics(fmt.Sprintf("%s context flags workspace/rag/internet = %t/%t/%t", tc.name, got.UseWorkspace, got.UseRAG, got.UseInternet), evalCaseMetrics(len(cases), passed))
		}
		if got.RequiresApproval != tc.wantApproval || got.WritesFiles != tc.wantWriteFiles || got.GeneratesExtension != tc.wantGenerateExtension || got.CreatesSchedulerJob != tc.wantSchedulerJob {
			return failWithMetrics(fmt.Sprintf("%s action flags approval/write/generate/schedule = %t/%t/%t/%t", tc.name, got.RequiresApproval, got.WritesFiles, got.GeneratesExtension, got.CreatesSchedulerJob), evalCaseMetrics(len(cases), passed))
		}
		if got.Confidence < tc.minConfidence {
			return failWithMetrics(fmt.Sprintf("%s confidence = %d, want >= %d", tc.name, got.Confidence, tc.minConfidence), evalCaseMetrics(len(cases), passed))
		}
		for _, tool := range tc.wantTools {
			if !containsValue(got.Tools, tool) {
				return failWithMetrics(fmt.Sprintf("%s tools = %v, missing %q", tc.name, got.Tools, tool), evalCaseMetrics(len(cases), passed))
			}
		}
		for _, tool := range tc.notTools {
			if containsValue(got.Tools, tool) {
				return failWithMetrics(fmt.Sprintf("%s tools = %v, unexpected %q", tc.name, got.Tools, tool), evalCaseMetrics(len(cases), passed))
			}
		}
		passed++
	}

	return passWithMetrics("routing matrix matched Post-RC primitive routes", evalCaseMetrics(len(cases), passed))
}

func evalRealWorldRouteSmoke() TaskResult {
	type routeCase struct {
		name  string
		input routing.Request
		check func(routing.Decision) string
	}
	cases := []routeCase{
		{
			name: "pasted code urls stay inert",
			input: routing.Request{
				Content: `Make this SVG cleaner: <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`,
			},
			check: func(got routing.Decision) string {
				if got.RouteCategory != routing.RouteChatExplanation {
					return fmt.Sprintf("route = %q, want chat explanation", got.RouteCategory)
				}
				if got.UseInternet || containsValue(got.Tools, "internet_fetch") || containsValue(got.Tools, "internet_search") {
					return fmt.Sprintf("pasted code used internet tools: route=%q tools=%v", got.RouteCategory, got.Tools)
				}
				if got.NeedsClarification {
					return "pasted code was treated as an ambiguous follow-up"
				}
				return ""
			},
		},
		{
			name: "indexed local documents beat search wording",
			input: routing.Request{
				Content: "Search indexed documents for the owner email.",
			},
			check: func(got routing.Decision) string {
				if got.RouteCategory != routing.RouteRAGSearch || got.TaskType != routing.TaskRAG || !containsValue(got.Tools, "rag_search") {
					return fmt.Sprintf("route/task/tools = %q/%q/%v, want rag search", got.RouteCategory, got.TaskType, got.Tools)
				}
				if containsValue(got.Tools, "internet_search") {
					return fmt.Sprintf("tools = %v, should not include internet_search", got.Tools)
				}
				return ""
			},
		},
		{
			name: "fresh public facts require internet source",
			input: routing.Request{
				Content: "Who is the current CEO of Example Robotics Ltd?",
			},
			check: func(got routing.Decision) string {
				if got.RouteCategory != routing.RouteInternetSearch || !got.UseInternet || !containsValue(got.Tools, "internet_search") {
					return fmt.Sprintf("route/internet/tools = %q/%t/%v, want internet_search", got.RouteCategory, got.UseInternet, got.Tools)
				}
				if got.UseRAG {
					return "fresh public fact unexpectedly used RAG"
				}
				return ""
			},
		},
		{
			name: "authorization follow-up stays inert without scope",
			input: routing.Request{
				Content: "I authorize you to continue.",
				Continuation: routing.BuildContinuationFrame(routing.ContinuationInput{
					Content: "I authorize you to continue.",
					Prior: routing.ContinuationPriorState{
						RouteCategory: routing.RouteActiveAssessmentRequiresScope,
						ToolName:      "internet_search",
						Target:        "example.test",
						SourceOfTruth: "internet",
					},
				}),
			},
			check: func(got routing.Decision) string {
				if got.UseInternet || len(got.Tools) > 0 {
					return fmt.Sprintf("authorization-only follow-up used tools: internet=%t tools=%v", got.UseInternet, got.Tools)
				}
				if got.RouteCategory == routing.RouteInternetSearch || got.RouteCategory == routing.RouteInternetFetch || got.RouteCategory == routing.RouteInternetHead {
					return fmt.Sprintf("authorization-only follow-up routed to internet: %q", got.RouteCategory)
				}
				return ""
			},
		},
		{
			name: "ambiguous follow-up asks before guessing",
			input: routing.Request{
				Content: "Search more about him.",
				TaskMemory: strings.Join([]string{
					"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
					"user: Tell me about Example Alpha.",
					"assistant: Example Alpha is one possible target.",
					"user: Tell me about Example Beta.",
					"assistant: Example Beta is another possible target.",
				}, "\n"),
			},
			check: func(got routing.Decision) string {
				if got.RouteCategory != routing.RouteClarify || !got.NeedsClarification {
					return fmt.Sprintf("route/clarification = %q/%t, want clarify", got.RouteCategory, got.NeedsClarification)
				}
				if got.UseInternet || containsValue(got.Tools, "internet_search") {
					return fmt.Sprintf("ambiguous retry used internet tools: %v", got.Tools)
				}
				return ""
			},
		},
	}

	passed := 0
	for _, tc := range cases {
		got := routing.Classify(tc.input)
		if failure := tc.check(got); failure != "" {
			return failWithMetrics(tc.name+": "+failure, realWorldRouteMetrics(len(cases), passed))
		}
		passed++
	}
	return passWithMetrics("generic real-world routing smoke cases passed", realWorldRouteMetrics(len(cases), passed))
}

func evalConversationRouteContinuity() TaskResult {
	total := 11
	passed := 0
	failCase := func(name string, details string) TaskResult {
		return failWithMetrics(name+": "+details, conversationRouteMetrics(total, passed))
	}

	activeNews := routing.SessionContract{
		ActiveGoal:       "Search current public news",
		ActiveRoute:      routing.RouteInternetSearch,
		ActiveDomain:     routing.DomainGeneral,
		ActiveTarget:     "current public news",
		ActiveCapability: "internet",
		ActiveLane:       routing.ToolLaneWebSearch,
		TaskStatus:       routing.TaskStatusActive,
	}
	got := routing.Classify(routing.Request{Content: "what about the visit?", SessionContract: activeNews})
	if got.RouteCategory != routing.RouteInternetSearch || got.ContinuationMode != routing.ContinuationModeAskFollowupPrevious || !containsValue(got.Tools, "internet_search") {
		return failCase("current-news follow-up", fmt.Sprintf("route/mode/tools = %q/%q/%v", got.RouteCategory, got.ContinuationMode, got.Tools))
	}
	passed++

	got = routing.Classify(routing.Request{Content: "No, I meant local documents", SessionContract: activeNews})
	if got.RouteCategory != routing.RouteRAGSearch || got.ContinuationMode != routing.ContinuationModeRevisePreviousTask || containsValue(got.Tools, "internet_search") {
		return failCase("source correction repair", fmt.Sprintf("route/mode/tools = %q/%q/%v", got.RouteCategory, got.ContinuationMode, got.Tools))
	}
	passed++

	got = routing.Classify(routing.Request{Content: "No, use memory instead", SessionContract: activeNews})
	if got.RouteCategory != routing.RouteMemorySearch || !containsValue(got.Tools, "memory_search") || containsValue(got.Tools, "internet_search") {
		return failCase("memory source correction repair", fmt.Sprintf("route/tools = %q/%v", got.RouteCategory, got.Tools))
	}
	passed++

	got = routing.Classify(routing.Request{Content: "No, use workspace files instead", SessionContract: activeNews})
	if got.RouteCategory != routing.RouteWorkspaceRead || !containsValue(got.Tools, "search_files") || containsValue(got.Tools, "internet_search") {
		return failCase("workspace source correction repair", fmt.Sprintf("route/tools = %q/%v", got.RouteCategory, got.Tools))
	}
	passed++

	activeDocs := routing.SessionContract{
		ActiveGoal:       "Answer from local documents",
		ActiveRoute:      routing.RouteRAGSearch,
		ActiveDomain:     routing.DomainGeneral,
		ActiveTarget:     "local study notes",
		ActiveCapability: "rag",
		ActiveLane:       routing.ToolLaneDocument,
		TaskStatus:       routing.TaskStatusActive,
	}
	got = routing.Classify(routing.Request{Content: "do the same for this", SessionContract: activeDocs})
	if got.RouteCategory != routing.RouteRAGSearch || got.ContinuationMode != routing.ContinuationModeContinueSameTask || !containsValue(got.Tools, "rag_search") {
		return failCase("do-same continuation", fmt.Sprintf("route/mode/tools = %q/%q/%v", got.RouteCategory, got.ContinuationMode, got.Tools))
	}
	next := routing.UpdateSessionContract(activeDocs, got, routing.SessionUpdate{
		Goal:        "Answer from local documents",
		LastOutcome: routing.LastOutcomeCompleted,
	})
	if next.ActiveRoute != routing.RouteRAGSearch || next.TaskStatus != routing.TaskStatusActive {
		return failCase("document route commitment", fmt.Sprintf("contract = %+v", next))
	}
	passed++

	got = routing.Classify(routing.Request{Content: "Search current shipping news today", SessionContract: activeDocs})
	if got.RouteCategory != routing.RouteInternetSearch || got.ContinuationMode != routing.ContinuationModeNewTask || !containsValue(got.Tools, "internet_search") {
		return failCase("task switch detection", fmt.Sprintf("route/mode/tools = %q/%q/%v", got.RouteCategory, got.ContinuationMode, got.Tools))
	}
	passed++

	pendingApproval := routing.SessionContract{
		ActiveGoal:       "Edit a local workspace file",
		ActiveRoute:      routing.RouteFileWrite,
		ActiveDomain:     routing.DomainCode,
		ActiveTarget:     "workspace file",
		ActiveCapability: "filesystem_write",
		ActiveLane:       routing.ToolLaneFileEdit,
		TaskStatus:       routing.TaskStatusAwaitingApproval,
		PendingApproval:  "edit_file",
	}
	got = routing.Classify(routing.Request{Content: "yes", SessionContract: pendingApproval})
	if got.RouteCategory != routing.RouteFileWrite || got.ContinuationMode != routing.ContinuationModeApprovePendingAction || got.ToolLane != routing.ToolLaneFileEdit {
		return failCase("approval resume", fmt.Sprintf("route/mode/lane = %q/%q/%q", got.RouteCategory, got.ContinuationMode, got.ToolLane))
	}
	next = routing.UpdateSessionContract(pendingApproval, got, routing.SessionUpdate{
		Goal:        "Edit a local workspace file",
		LastOutcome: routing.LastOutcomeCompleted,
	})
	if next.PendingApproval != "" || next.TaskStatus != routing.TaskStatusActive {
		return failCase("approval state cleared", fmt.Sprintf("contract = %+v", next))
	}
	passed++

	activeFileRead := routing.SessionContract{
		ActiveGoal:       "Read a local workspace file",
		ActiveRoute:      routing.RouteFileRead,
		ActiveDomain:     routing.DomainCode,
		ActiveTarget:     "first file",
		ActiveCapability: "filesystem_read",
		ActiveLane:       routing.ToolLaneResearch,
		TaskStatus:       routing.TaskStatusActive,
	}
	got = routing.Classify(routing.Request{Content: "no, I meant the other file", SessionContract: activeFileRead})
	if got.RouteCategory != routing.RouteFileRead || got.ContinuationMode != routing.ContinuationModeRevisePreviousTask {
		return failCase("target correction", fmt.Sprintf("route/mode = %q/%q", got.RouteCategory, got.ContinuationMode))
	}
	passed++

	got = routing.Classify(routing.Request{Content: "Compare it with the previous one", SessionContract: activeFileRead})
	if got.RouteCategory != routing.RouteFileRead || got.Target != "first file" || got.ContinuationMode != routing.ContinuationModeAskFollowupPrevious {
		return failCase("comparison follow-up target continuity", fmt.Sprintf("route/target/mode = %q/%q/%q", got.RouteCategory, got.Target, got.ContinuationMode))
	}
	passed++

	got = routing.Classify(routing.Request{Content: "that", SessionContract: activeNews})
	if got.RouteCategory != routing.RouteClarify || !got.NeedsClarification || got.ContinuationMode != routing.ContinuationModeUnclear {
		return failCase("ambiguous follow-up clarification", fmt.Sprintf("route/mode/clarify = %q/%q/%t", got.RouteCategory, got.ContinuationMode, got.NeedsClarification))
	}
	passed++

	pendingClarification := routing.SessionContract{
		ActiveGoal:           "Answer from local documents",
		ActiveRoute:          routing.RouteRAGSearch,
		ActiveDomain:         routing.DomainGeneral,
		ActiveTarget:         "local documents",
		ActiveCapability:     "rag",
		ActiveLane:           routing.ToolLaneDocument,
		TaskStatus:           routing.TaskStatusAwaitingClarification,
		PendingClarification: "Which document should I use?",
		ContinuationMode:     routing.ContinuationModeUnclear,
	}
	got = routing.Classify(routing.Request{Content: "the second note", SessionContract: pendingClarification})
	if got.RouteCategory != routing.RouteRAGSearch || got.ContinuationMode != routing.ContinuationModeAnswerPendingClarify {
		return failCase("clarification answer continuation", fmt.Sprintf("route/mode = %q/%q", got.RouteCategory, got.ContinuationMode))
	}
	passed++

	return passWithMetrics("multi-turn routing control-plane cases passed", conversationRouteMetrics(total, passed))
}

func evalRouteGovernanceClassifier() TaskResult {
	cases := []struct {
		name  string
		trace replay.Trace
		want  string
	}{
		{
			name: "wrong_source",
			trace: replay.Trace{
				UserRequest: "Use indexed documents for this answer",
				Route:       replay.RouteSnapshot{Category: routing.RouteInternetSearch, ShouldUseInternet: true},
				Errors:      []replay.TraceError{{Stage: "routing", Code: "wrong_source", Message: "local documents should have been used", ExpectedRoute: routing.RouteRAGSearch}},
			},
			want: learning.RouteFailureWrongSource,
		},
		{
			name: "wrong_tool_lane",
			trace: replay.Trace{
				UserRequest:  "Answer from local documents",
				Route:        replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled:  []replay.ToolCall{{Name: "internet_search", Status: "completed"}},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"tool lane mismatch"}},
			},
			want: learning.RouteFailureWrongToolLane,
		},
		{
			name: "stale_answer",
			trace: replay.Trace{
				UserRequest:  "What is the latest public update?",
				Route:        replay.RouteSnapshot{Category: routing.RouteChatExplanation},
				Attributes:   []replay.Attribute{{Key: "route_preflight_source_of_truth", Value: routing.PreflightSourceInternet}, {Key: "route_preflight_freshness_risk", Value: routing.PreflightFreshnessHigh}},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"fresh source required"}},
			},
			want: learning.RouteFailureStaleAnswer,
		},
		{
			name: "missing_approval",
			trace: replay.Trace{
				UserRequest:          "Create the scheduled job",
				Route:                replay.RouteSnapshot{Category: routing.RouteSchedulerCreate},
				PermissionsRequested: []replay.PermissionRequest{{ToolName: "scheduler_create", Status: "denied", Reason: "approval required"}},
			},
			want: learning.RouteFailureMissingApproval,
		},
		{
			name: "missing_evidence",
			trace: replay.Trace{
				UserRequest:  "Answer from local notes",
				Route:        replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"no local document evidence"}},
			},
			want: learning.RouteFailureMissingEvidence,
		},
		{
			name: "rag_no_matches_missing_evidence",
			trace: replay.Trace{
				UserRequest: "Answer from local notes",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{{Name: "rag_search", Status: "completed", OutputSummary: "No RAG matches."}},
				Verification: replay.VerificationResult{
					Status: "needs_follow_up",
					Notes:  []string{"RAG answer should cite or name retrieved local sources"},
				},
			},
			want: learning.RouteFailureMissingEvidence,
		},
		{
			name: "general_knowledge_optional_evidence_contract",
			trace: replay.Trace{
				UserRequest: "Explain a general concept",
				Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
				Attributes: []replay.Attribute{
					{Key: "evidence_policy", Value: "general_knowledge_allowed"},
					{Key: "response_contract", Value: "general model knowledge is allowed"},
				},
				Verification: replay.VerificationResult{
					Status: "needs_follow_up",
					Notes:  []string{"general knowledge route refused because optional local/workspace evidence was absent"},
				},
				FinalResult: replay.ResultSnapshot{
					OutputSummary: "The concept is not present in the available local context or workspace files, so I cannot explain it.",
				},
			},
			want: learning.RouteFailureEvidenceContract,
		},
		{
			name: "bad_clarification",
			trace: replay.Trace{
				UserRequest:  "Do that one",
				Route:        replay.RouteSnapshot{Category: routing.RouteChatExplanation, ShouldAskClarification: true},
				Attributes:   []replay.Attribute{{Key: "route_preflight_missing_slots", Value: "target"}},
				Verification: replay.VerificationResult{Status: "failed", Notes: []string{"should ask clarification"}},
			},
			want: learning.RouteFailureBadClarification,
		},
		{
			name: "provider_config_issue",
			trace: replay.Trace{
				UserRequest: "Search current news",
				Route:       replay.RouteSnapshot{Category: routing.RouteInternetSearch, ShouldUseInternet: true},
				ToolsCalled: []replay.ToolCall{{Name: "internet_search", Status: "failed", Error: "provider not configured"}},
			},
			want: learning.RouteFailureProviderConfigIssue,
		},
		{
			name: "rag_embedding_runtime_provider_issue",
			trace: replay.Trace{
				UserRequest: "Search local documents",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{{Name: "rag_search", Status: "failed", Error: "ollama embed request failed: context deadline exceeded"}},
			},
			want: learning.RouteFailureProviderConfigIssue,
		},
		{
			name: "internet_provider_failure_wrong_lane",
			trace: replay.Trace{
				UserRequest: "Answer from local documents",
				Route:       replay.RouteSnapshot{Category: routing.RouteRAGSearch},
				ToolsCalled: []replay.ToolCall{{Name: "internet_search", Status: "failed", Error: "provider not configured"}},
			},
			want: learning.RouteFailureWrongToolLane,
		},
		{
			name: "permission_missing",
			trace: replay.Trace{
				UserRequest: "Read a local workspace file",
				Route:       replay.RouteSnapshot{Category: routing.RouteWorkspaceRead},
				ToolsCalled: []replay.ToolCall{{Name: "read_file", Status: "failed", Error: "operation not permitted"}},
			},
			want: learning.RouteFailurePermissionMissing,
		},
		{
			name: "missing_capability",
			trace: replay.Trace{
				UserRequest: "Create a reusable capability",
				Route:       replay.RouteSnapshot{Category: routing.RouteExtensionGenerate},
				Errors:      []replay.TraceError{{Stage: "capability", Code: "missing_capability", Message: "no matching safe executor"}},
			},
			want: learning.RouteFailureMissingCapability,
		},
		{
			name: "wrong_route",
			trace: replay.Trace{
				UserRequest: "Search current public updates",
				Route:       replay.RouteSnapshot{Category: routing.RouteChatExplanation},
				Errors:      []replay.TraceError{{Stage: "routing", Code: "route_mismatch", Message: "expected route did not match task frame", ExpectedRoute: routing.RouteInternetSearch}},
			},
			want: learning.RouteFailureWrongRoute,
		},
	}

	passed := 0
	for _, tc := range cases {
		got := learning.ClassifyRouteFailure(tc.trace)
		if got.Category != tc.want {
			return failWithMetrics(fmt.Sprintf("%s category = %q, want %q", tc.name, got.Category, tc.want), evalCaseMetrics(len(cases), passed))
		}
		if got.RegressionPromotion.SuggestedTestName == "" || got.SuggestedRegression == "" {
			return failWithMetrics(tc.name+": missing route-governance promotion metadata", evalCaseMetrics(len(cases), passed))
		}
		passed++
	}
	return passWithMetrics("route-governance classifier categorized generic replay failures", evalCaseMetrics(len(cases), passed))
}

func evalQAReviewRegressionLedger(profile *profiles.Profile) TaskResult {
	if profile == nil || strings.TrimSpace(profile.Root) == "" {
		return fail("profile root is required")
	}
	records, err := learning.ListQARegressionReviews(filepath.Join(profile.Root, "qa_review", "regression_reviews"))
	if err != nil {
		return fail("QA review regression ledger is invalid: " + err.Error())
	}
	drafts, err := learning.ListQARegressionTestDrafts(filepath.Join(profile.Root, "qa_review", "regression_test_drafts"))
	if err != nil {
		return fail("QA review regression test draft ledger is invalid: " + err.Error())
	}
	approvals, err := learning.ListQARegressionApprovedRecords(filepath.Join(profile.Root, "qa_review", "approved_regressions"))
	if err != nil {
		return fail("QA review approved regression ledger is invalid: " + err.Error())
	}
	if err := learning.ValidateQARegressionReviewFlow(records, drafts, approvals); err != nil {
		return fail("QA review regression flow is invalid: " + err.Error())
	}
	counts := map[string]int{}
	for _, record := range records {
		counts[record.Status]++
	}
	return passWithMetrics("QA review regression ledger is readable and schema-valid", map[string]any{
		"qa_review_review_records":        len(records),
		"qa_review_approved_regressions":  counts[learning.QARegressionReviewApproved],
		"qa_review_covered_regressions":   counts[learning.QARegressionReviewCovered],
		"qa_review_dismissed_regressions": counts[learning.QARegressionReviewDismissed],
		"qa_review_test_drafts":           len(drafts),
		"qa_review_regression_approvals":  len(approvals),
	})
}

func evalContextCompilerLowMemoryBehavior() TaskResult {
	budget := contextcore.Budget{
		MaxBytes:     420,
		MaxTokens:    150,
		MinItemBytes: 8,
		PerSource: map[contextcore.Source]contextcore.Limit{
			contextcore.SourceMemory: {MaxBytes: 96},
			contextcore.SourceRAG:    {MaxBytes: 96},
		},
	}
	result := contextcore.Compile(contextcore.Input{
		Request: "Summarize local launch notes for a small local model.",
		Items: []contextcore.Item{
			contextcore.PolicyItem("policy", "Safety policy", "File writes require snapshot, diff, confirmation, and rollback.", 1, contextcore.WithRequired(true)),
			contextcore.PreferenceItem("preference", "Preference", "Prefer concise local-first answers.", 0.9),
			contextcore.MemoryItem("memory", "Correction memory", strings.Repeat("Keep optional internet, connector, and background systems disabled. ", 8), 1),
			contextcore.RAGItem("rag", "Release notes", strings.Repeat("Extension rollback uses manifests, tests, approval, audit logs, and snapshots. ", 8), 1),
			contextcore.MemoryItem("stale", "Stale memory", "Old low-value context that should not survive the relevance threshold.", 0.1),
		},
		Options: contextcore.Options{
			Budget:       budget,
			MinRelevance: 0.25,
		},
	})

	metrics := map[string]any{
		"context_used_bytes":     result.Stats.UsedBytes,
		"context_used_tokens":    result.Stats.UsedTokens,
		"context_selected_items": result.Stats.ItemsSelected,
		"context_dropped_items":  result.Stats.ItemsDropped,
		"context_truncated":      result.Stats.Truncated,
	}
	if result.Stats.UsedBytes > budget.MaxBytes || result.Stats.UsedTokens > budget.MaxTokens {
		metrics["context_low_memory_budget_fit_rate"] = 0.0
		return failWithMetrics("compiled context exceeded low-memory budget", metrics)
	}
	if result.Stats.BySource[contextcore.SourceMemory].Bytes > budget.PerSource[contextcore.SourceMemory].MaxBytes {
		metrics["context_low_memory_budget_fit_rate"] = 0.0
		return failWithMetrics("memory context exceeded per-source budget", metrics)
	}
	if result.Stats.BySource[contextcore.SourceRAG].Bytes > budget.PerSource[contextcore.SourceRAG].MaxBytes {
		metrics["context_low_memory_budget_fit_rate"] = 0.0
		return failWithMetrics("RAG context exceeded per-source budget", metrics)
	}
	for _, id := range []string{"policy", "preference", "memory", "rag"} {
		if !compiledHasID(result.Items, id) {
			metrics["context_low_memory_budget_fit_rate"] = 0.0
			return failWithMetrics("compiled context missing selected item: "+id, metrics)
		}
	}
	if !droppedHasID(result.Dropped, "stale") {
		metrics["context_low_memory_budget_fit_rate"] = 0.0
		return failWithMetrics("low-relevance context was not dropped", metrics)
	}
	if !result.Stats.Truncated {
		metrics["context_low_memory_budget_fit_rate"] = 0.0
		return failWithMetrics("large local context was not reduced for low-memory budget", metrics)
	}
	metrics["context_low_memory_budget_fit_rate"] = 1.0
	return passWithMetrics("context compiler preserved high-value context inside low-memory budgets", metrics)
}

func evalCapabilityRegistryGapBehavior() TaskResult {
	registry, err := capabilities.DefaultLocalFirstSnapshot()
	if err != nil {
		return fail(err.Error())
	}
	cases := []struct {
		name       string
		route      capabilities.RouteMetadata
		wantAction capabilities.GapAction
		wantName   string
		wantKind   capabilities.Kind
		wantState  capabilities.State
	}{
		{
			name: "local filesystem read is ready",
			route: capabilities.RouteMetadata{
				Request:       "Search workspace for config loader and summarize matching files.",
				RouteCategory: routing.RouteFileRead,
				Tools:         []string{"read_file", "search_files"},
				UseWorkspace:  true,
				ReadsFiles:    true,
			},
			wantAction: capabilities.ActionUseExisting,
			wantName:   "filesystem_read",
			wantKind:   capabilities.KindTool,
			wantState:  capabilities.StateReady,
		},
		{
			name: "internet provider asks config",
			route: capabilities.RouteMetadata{
				Request:       "Search the web for SearXNG documentation.",
				RouteCategory: routing.RouteInternetSearch,
				Tools:         []string{"internet_search"},
				UseInternet:   true,
			},
			wantAction: capabilities.ActionAskConfig,
			wantName:   "internet",
			wantKind:   capabilities.KindInternetProvider,
			wantState:  capabilities.StateDisabled,
		},
		{
			name: "extension generation asks config by default",
			route: capabilities.RouteMetadata{
				Request:            "Normalize CSV invoices into a custom ledger format.",
				RouteCategory:      routing.RouteExtensionGenerate,
				Capability:         "extension_generation",
				Target:             "generated_extension",
				GeneratesExtension: true,
			},
			wantAction: capabilities.ActionAskConfig,
			wantName:   "extension_generation",
			wantKind:   capabilities.KindExtension,
			wantState:  capabilities.StateDisabled,
		},
		{
			name: "scheduler asks config",
			route: capabilities.RouteMetadata{
				Request:             "Run this check every hour.",
				RouteCategory:       routing.RouteSchedulerCreate,
				CreatesSchedulerJob: true,
			},
			wantAction: capabilities.ActionAskConfig,
			wantName:   "scheduler",
			wantKind:   capabilities.KindScheduler,
			wantState:  capabilities.StateDisabled,
		},
	}

	passed := 0
	for _, tc := range cases {
		decision := registry.ClassifyRouteGap(tc.route)
		if decision.Action != tc.wantAction || decision.Name != tc.wantName || decision.Kind != tc.wantKind || decision.State != tc.wantState {
			return failWithMetrics(fmt.Sprintf("%s decision = %+v", tc.name, decision), capabilityGapMetrics(len(cases), passed))
		}
		if decision.Action == capabilities.ActionGenerateExtension {
			return failWithMetrics("default registry generated an extension without explicit enablement", capabilityGapMetrics(len(cases), passed))
		}
		passed++
	}
	return passWithMetrics("capability registry classified local-ready and optional-disabled gaps", capabilityGapMetrics(len(cases), passed))
}

func evalDomainPackTemplateAwareness() TaskResult {
	templateRoot := locateRepoPath("packs", "templates")
	if templateRoot == "" {
		return skip("domain pack templates are not available from the current working tree")
	}
	repoRoot := filepath.Dir(filepath.Dir(templateRoot))
	catalog, err := workflows.NewBuiltInTemplateCatalog(repoRoot)
	if err != nil {
		return fail("load built-in domain pack template catalog: " + err.Error())
	}
	templates := catalog.List()
	if len(templates) == 0 {
		return failWithMetrics("no domain pack templates found", map[string]any{"domain_pack_templates_checked": 0})
	}
	summaries := workflows.DomainPackTemplateSummaries(catalog, nil)
	if len(summaries) != len(templates) {
		return failWithMetrics("domain pack template summaries do not match catalog", map[string]any{"domain_pack_templates_checked": 0})
	}
	checked := 0
	for _, template := range templates {
		if err := workflows.ValidateTemplate(template); err != nil {
			return failWithMetrics(err.Error(), map[string]any{"domain_pack_templates_checked": checked})
		}
		summary, ok := templateSummaryByName(summaries, template.Name)
		if !ok {
			return failWithMetrics("domain pack template missing summary: "+template.Name, map[string]any{"domain_pack_templates_checked": checked})
		}
		if summary.Source != workflows.SourceDomainPackTemplate ||
			summary.Path == "" ||
			summary.InstallAction != "install" ||
			summary.Installed ||
			summary.Enabled ||
			!summary.Valid ||
			summary.DefaultState != workflows.DefaultStateDisabled ||
			len(summary.RequiredTools) == 0 ||
			len(summary.Skills) == 0 ||
			len(summary.Workflows) == 0 ||
			summary.SafetySummary == "" {
			return failWithMetrics("domain pack template summary missing lifecycle metadata: "+template.Name, map[string]any{"domain_pack_templates_checked": checked})
		}
		manifest, err := domainpacks.Load(template.Path)
		if err != nil {
			return failWithMetrics(err.Error(), map[string]any{"domain_pack_templates_checked": checked})
		}
		if manifest.EnabledByDefault() {
			return failWithMetrics("domain pack template is enabled by default: "+manifest.Name, map[string]any{"domain_pack_templates_checked": checked})
		}
		if templateRequiresOptionalSystem(manifest.Permissions) {
			return failWithMetrics("domain pack template requires optional online/background/secrets capability: "+manifest.Name, map[string]any{"domain_pack_templates_checked": checked})
		}
		if len(manifest.Skills) == 0 || len(manifest.Tests) == 0 {
			return failWithMetrics("domain pack template must declare skills and tests: "+manifest.Name, map[string]any{"domain_pack_templates_checked": checked})
		}
		status := domainpacks.PackStatus{
			Name:          manifest.Name,
			Version:       manifest.Version,
			Description:   manifest.Description,
			Category:      manifest.Category,
			Enabled:       false,
			Installed:     true,
			Valid:         true,
			Skills:        manifest.Skills,
			RequiredTools: manifest.RequiredTools,
			OptionalTools: manifest.OptionalTools,
			Dir:           manifest.Dir,
		}
		capability, err := capabilities.DomainPackCapability(status, manifest)
		if err != nil {
			return failWithMetrics(err.Error(), map[string]any{"domain_pack_templates_checked": checked})
		}
		if capability.State != capabilities.StateDisabled || capability.Enabled {
			return failWithMetrics("disabled domain pack template looked ready: "+manifest.Name, map[string]any{"domain_pack_templates_checked": checked})
		}
		for _, tool := range manifest.RequiredTools {
			if !containsValue(capability.Provides, "tool:"+tool) {
				return failWithMetrics("domain pack capability missing required tool provide: "+tool, map[string]any{"domain_pack_templates_checked": checked})
			}
		}
		checked++
	}
	return passWithMetrics("domain pack templates are disabled, local, and capability-aware", map[string]any{
		"domain_pack_templates_checked":  checked,
		"domain_pack_template_pass_rate": 1.0,
	})
}

func templateSummaryByName(summaries []workflows.TemplateSummary, name string) (workflows.TemplateSummary, bool) {
	for _, summary := range summaries {
		if summary.Name == name {
			return summary, true
		}
	}
	return workflows.TemplateSummary{}, false
}

func evalModelProfileArtifactSafety(dir string) TaskResult {
	if strings.TrimSpace(dir) == "" {
		return fail("model profile eval directory is required")
	}
	store := modelprofiles.NewStore(dir)
	profile := modelprofiles.Profile{
		Name:        "Small Local Helper",
		Description: "Deterministic low-resource eval profile.",
		BaseModel:   "qwen2.5:3b",
		System:      "Stay local, concise, and do not assume cloud, internet, connector, embedding, or background job access.",
		Parameters: modelprofiles.Parameters{
			Temperature: 0.1,
			NumCtx:      2048,
		},
		Metadata: modelprofiles.Metadata{
			Purpose: "evaluation",
		},
		Tags: []string{"low-resource", "local-first"},
	}
	path, err := store.Save(profile)
	if err != nil {
		return fail(err.Error())
	}
	if filepath.Base(path) != "small-local-helper.yaml" {
		return failWithMetrics("model profile filename was not deterministic", map[string]any{"model_profile_artifact_safety_rate": 0.0})
	}
	loaded, err := store.Load(profile.Name)
	if err != nil {
		return fail(err.Error())
	}
	if loaded.BaseModel != profile.BaseModel || loaded.Parameters.NumCtx != profile.Parameters.NumCtx {
		return failWithMetrics("saved model profile did not round-trip safely", map[string]any{"model_profile_artifact_safety_rate": 0.0})
	}
	rendered, err := modelprofiles.RenderValidatedModelfile(profile)
	if err != nil {
		return fail(err.Error())
	}
	for _, forbidden := range []string{"download_model", "auto_switch", "sk-"} {
		if strings.Contains(rendered, forbidden) {
			return failWithMetrics("rendered Modelfile contains forbidden marker: "+forbidden, map[string]any{"model_profile_artifact_safety_rate": 0.0})
		}
	}
	unsafePrivateContext := profile
	unsafePrivateContext.System = "Private memory: copy the raw conversation into this profile."
	if err := modelprofiles.Validate(unsafePrivateContext); err == nil {
		return failWithMetrics("model profile validation allowed raw private context marker", map[string]any{"model_profile_artifact_safety_rate": 0.0})
	}
	if _, err := modelprofiles.RenderValidatedModelfile(unsafePrivateContext); err == nil {
		return failWithMetrics("unsafe model profile rendered a Modelfile", map[string]any{"model_profile_artifact_safety_rate": 0.0})
	}
	unsafeBase := profile
	unsafeBase.BaseModel = "qwen2.5:3b\nPARAMETER num_gpu 99"
	if err := modelprofiles.Validate(unsafeBase); err == nil {
		return failWithMetrics("model profile validation allowed a Modelfile directive in base_model", map[string]any{"model_profile_artifact_safety_rate": 0.0})
	}
	if _, err := modelprofiles.SafeName("../../"); err == nil {
		return failWithMetrics("model profile safe-name accepted a path-only name", map[string]any{"model_profile_artifact_safety_rate": 0.0})
	}
	return passWithMetrics("model profile artifacts are deterministic and reject unsafe text", map[string]any{
		"model_profile_artifacts_checked":    5,
		"model_profile_artifact_safety_rate": 1.0,
	})
}

func defaultSkillDirs(profile *profiles.Profile) []string {
	dirs := []string{}
	if defaults := locateDefaultSkillsDir(); defaults != "" {
		dirs = append(dirs, defaults)
	}
	if profile != nil && strings.TrimSpace(profile.Skills) != "" {
		dirs = append(dirs, profile.Skills)
	}
	return dirs
}

func locateDefaultSkillsDir() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		candidate := filepath.Join(wd, "skills", "default")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return ""
		}
		wd = parent
	}
}

func locateRepoPath(parts ...string) string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		segments := append([]string{wd}, parts...)
		candidate := filepath.Join(segments...)
		if info, err := os.Stat(candidate); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return candidate
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			return ""
		}
		wd = parent
	}
}

func (r Runner) evalModelReadiness(ctx context.Context, model string) TaskResult {
	if r.Runtime == nil {
		return skip("model runtime is not configured")
	}
	if err := r.Runtime.Health(ctx); err != nil {
		return skip("Ollama not reachable: " + err.Error())
	}
	installed, err := r.Runtime.ListModels(ctx)
	if err != nil {
		return fail(err.Error())
	}
	localModels := localInstalledModels(installed)
	if !modelInstalled(model, localModels) {
		return fail("selected model is not an installed local model: " + model)
	}
	return passWithMetrics("selected model is installed", map[string]any{"model": model})
}

func (r Runner) evalModelChatSmoke(ctx context.Context, model string) TaskResult {
	if r.Runtime == nil {
		return skip("model runtime is not configured")
	}
	installed, err := r.Runtime.ListModels(ctx)
	if err != nil {
		return skip("cannot list models: " + err.Error())
	}
	if !modelInstalled(model, localInstalledModels(installed)) {
		return skip("selected model is not an installed local model: " + model)
	}
	smokeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var output strings.Builder
	tokenEvents := 0
	start := time.Now()
	err = r.Runtime.ChatStream(smokeCtx, models.ChatRequest{
		Model: model,
		Messages: []models.ChatMessage{
			{Role: "system", Content: "You are a local smoke test. Reply with one short sentence."},
			{Role: "user", Content: "Say READY."},
		},
		Temperature: 0.1,
	}, func(event models.ChatEvent) error {
		if event.Token != "" {
			tokenEvents++
			output.WriteString(event.Token)
		}
		return nil
	})
	elapsed := time.Since(start)
	if err != nil {
		return fail(err.Error())
	}
	text := strings.TrimSpace(output.String())
	if text == "" {
		return fail("model returned empty response")
	}
	seconds := elapsed.Seconds()
	if seconds < 0.001 {
		seconds = 0.001
	}
	return passWithMetrics("model returned a response", map[string]any{
		"chars":             len(text),
		"model":             model,
		"stream_events":     tokenEvents,
		"tokens_per_second": float64(tokenEvents) / seconds,
		"latency_ms":        elapsed.Milliseconds(),
	})
}

func measure(name string, fn func() TaskResult) TaskResult {
	start := time.Now()
	result := fn()
	result.Name = name
	result.DurationMS = time.Since(start).Milliseconds()
	if result.Status == "" {
		result.Status = StatusFail
	}
	return result
}

func aggregateMetrics(report Report, previous *Report) map[string]any {
	metrics := map[string]any{
		"task_success_rate": successRate(report.Summary.Passed, report.Summary.Failed),
	}
	for _, task := range report.Tasks {
		switch task.Name {
		case "startup_time":
			copyMetric(metrics, task.Metrics, "startup_ms", "startup_ms")
		case "ram_usage_sample":
			copyMetric(metrics, task.Metrics, "heap_alloc_bytes", "ram_heap_alloc_bytes")
			copyMetric(metrics, task.Metrics, "heap_sys_bytes", "ram_heap_sys_bytes")
			copyMetric(metrics, task.Metrics, "sys_bytes", "ram_sys_bytes")
		case "rag_keyword_hit":
			copyMetric(metrics, task.Metrics, "retrieval_accuracy", "rag_retrieval_accuracy")
			copyMetric(metrics, task.Metrics, "reciprocal_rank", "rag_reciprocal_rank")
		case "snapshot_rollback":
			copyMetric(metrics, task.Metrics, "rollback_success_rate", "rollback_success_rate")
		case "dangerous_command_block":
			copyMetric(metrics, task.Metrics, "prevention_rate", "dangerous_command_prevention_rate")
		case "skill_reuse_rate":
			copyMetric(metrics, task.Metrics, "skill_reuse_rate", "skill_reuse_rate")
		case "routing_matrix_behavior":
			copyMetric(metrics, task.Metrics, "routing_matrix_pass_rate", "routing_matrix_pass_rate")
		case "real_world_route_smoke":
			copyMetric(metrics, task.Metrics, "real_world_route_pass_rate", "real_world_route_pass_rate")
			copyMetric(metrics, task.Metrics, "real_world_route_cases", "real_world_route_cases")
		case "context_compiler_low_memory":
			copyMetric(metrics, task.Metrics, "context_low_memory_budget_fit_rate", "context_low_memory_budget_fit_rate")
			copyMetric(metrics, task.Metrics, "context_used_bytes", "context_used_bytes")
			copyMetric(metrics, task.Metrics, "context_used_tokens", "context_used_tokens")
		case "capability_registry_gap":
			copyMetric(metrics, task.Metrics, "capability_gap_pass_rate", "capability_gap_pass_rate")
		case "domain_pack_template_awareness":
			copyMetric(metrics, task.Metrics, "domain_pack_templates_checked", "domain_pack_templates_checked")
			copyMetric(metrics, task.Metrics, "domain_pack_template_pass_rate", "domain_pack_template_pass_rate")
		case "model_profile_artifact_safety":
			copyMetric(metrics, task.Metrics, "model_profile_artifact_safety_rate", "model_profile_artifact_safety_rate")
		case "notification_center_local_inbox":
			copyMetric(metrics, task.Metrics, "notification_secret_redaction_rate", "notification_secret_redaction_rate")
		case "model_chat_smoke":
			copyMetric(metrics, task.Metrics, "tokens_per_second", "tokens_per_second")
			copyMetric(metrics, task.Metrics, "stream_events", "model_stream_events")
			copyMetric(metrics, task.Metrics, "latency_ms", "model_latency_ms")
		}
	}

	toolPassed, toolTotal := taskGroupRate(report.Tasks, map[string]bool{
		"snapshot_rollback":       true,
		"dangerous_command_block": true,
		"safe_command_allowlist":  true,
	})
	metrics["tool_call_tasks_passed"] = toolPassed
	metrics["tool_call_tasks_total"] = toolTotal
	metrics["tool_call_success_rate"] = successRate(toolPassed, toolTotal-toolPassed)

	smartPassed, smartTotal := taskGroupRate(report.Tasks, map[string]bool{
		"routing_matrix_behavior":         true,
		"real_world_route_smoke":          true,
		"context_compiler_low_memory":     true,
		"capability_registry_gap":         true,
		"domain_pack_template_awareness":  true,
		"model_profile_artifact_safety":   true,
		"notification_center_local_inbox": true,
	})
	metrics["post_rc_smartness_tasks_passed"] = smartPassed
	metrics["post_rc_smartness_tasks_total"] = smartTotal
	metrics["post_rc_smartness_success_rate"] = successRate(smartPassed, smartTotal-smartPassed)

	if current, ok := floatMetric(metrics, "dangerous_command_prevention_rate"); ok {
		if previous != nil {
			if prior, ok := floatMetric(previous.Metrics, "dangerous_command_prevention_rate"); ok {
				metrics["dangerous_command_prevention_previous"] = prior
				switch {
				case current > prior:
					metrics["dangerous_command_prevention_trend"] = "improved"
				case current < prior:
					metrics["dangerous_command_prevention_trend"] = "regressed"
				default:
					metrics["dangerous_command_prevention_trend"] = "stable"
				}
			} else {
				metrics["dangerous_command_prevention_trend"] = "no_previous_metric"
			}
		} else {
			metrics["dangerous_command_prevention_trend"] = "no_previous_report"
		}
	}
	return metrics
}

func copyMetric(target map[string]any, source map[string]any, sourceKey string, targetKey string) {
	if target == nil || source == nil {
		return
	}
	if value, ok := source[sourceKey]; ok {
		target[targetKey] = value
	}
}

func taskGroupRate(tasks []TaskResult, names map[string]bool) (int, int) {
	passed := 0
	total := 0
	for _, task := range tasks {
		if !names[task.Name] || task.Status == StatusSkip {
			continue
		}
		total++
		if task.Status == StatusPass {
			passed++
		}
	}
	return passed, total
}

func successRate(passed int, failed int) float64 {
	total := passed + failed
	if total <= 0 {
		return 0
	}
	return float64(passed) / float64(total)
}

func evalCaseMetrics(total int, passed int) map[string]any {
	return map[string]any{
		"routing_matrix_cases":     total,
		"routing_matrix_passed":    passed,
		"routing_matrix_pass_rate": successRate(passed, total-passed),
	}
}

func realWorldRouteMetrics(total int, passed int) map[string]any {
	return map[string]any{
		"real_world_route_cases":     total,
		"real_world_route_passed":    passed,
		"real_world_route_pass_rate": successRate(passed, total-passed),
	}
}

func conversationRouteMetrics(total int, passed int) map[string]any {
	return map[string]any{
		"conversation_route_cases":     total,
		"conversation_route_passed":    passed,
		"conversation_route_pass_rate": successRate(passed, total-passed),
	}
}

func capabilityGapMetrics(total int, passed int) map[string]any {
	return map[string]any{
		"capability_gap_cases":     total,
		"capability_gap_passed":    passed,
		"capability_gap_pass_rate": successRate(passed, total-passed),
	}
}

func containsValue(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func compiledHasID(items []contextcore.CompiledItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func droppedHasID(items []contextcore.DroppedItem, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func templateRequiresOptionalSystem(permissions domainpacks.Permissions) bool {
	return permissions.Internet.Required ||
		permissions.Scheduler.Required ||
		permissions.Notifications.Required ||
		permissions.Secrets.Required ||
		permissions.Connectors.Required
}

func floatMetric(metrics map[string]any, key string) (float64, bool) {
	if metrics == nil {
		return 0, false
	}
	switch value := metrics[key].(type) {
	case float64:
		return value, true
	case float32:
		return float64(value), true
	case int:
		return float64(value), true
	case int64:
		return float64(value), true
	case uint64:
		return float64(value), true
	default:
		return 0, false
	}
}

func loadPreviousReport(profile *profiles.Profile) *Report {
	report, err := LoadLatest(profile)
	if err != nil {
		return nil
	}
	return &report
}

func evalSkillTools() map[string]bool {
	return tools.ToolAvailabilityForSurface(tools.ToolSurfaceSkill)
}

func selectedModelName(cfg *config.Config, mode string) string {
	if mode == "low-memory" || cfg.Runtime.LowMemoryMode {
		if model, ok := cfg.Models["low_memory"]; ok {
			return model.Name
		}
	}
	if model, ok := cfg.Models["default"]; ok {
		return model.Name
	}
	return ""
}

func modelInstalled(name string, installed []models.ModelInfo) bool {
	return models.ModelInstalled(name, installed)
}

func localInstalledModels(installed []models.ModelInfo) []models.ModelInfo {
	return models.LocalInstalledModels(installed)
}

func pass(details string) TaskResult {
	return TaskResult{Status: StatusPass, Details: details}
}

func passWithMetrics(details string, metrics map[string]any) TaskResult {
	return TaskResult{Status: StatusPass, Details: details, Metrics: metrics}
}

func fail(details string) TaskResult {
	return TaskResult{Status: StatusFail, Details: details}
}

func failWithMetrics(details string, metrics map[string]any) TaskResult {
	return TaskResult{Status: StatusFail, Details: details, Metrics: metrics}
}

func skip(details string) TaskResult {
	return TaskResult{Status: StatusSkip, Details: details}
}
