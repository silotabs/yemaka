package routing

import (
	"strings"
	"testing"
	"time"
)

func TestClassifyRealWorldRoutingMatrix(t *testing.T) {
	cases := []struct {
		name              string
		prompt            string
		wantTask          string
		wantTools         []string
		notTools          []string
		wantWorkspace     bool
		wantRAG           bool
		wantInternet      bool
		wantClarification bool
		minConfidence     int
	}{
		{
			name:          "casual chat does not retrieve workspace",
			prompt:        "Hello, how are you today?",
			wantTask:      TaskChat,
			wantWorkspace: false,
			wantRAG:       false,
			minConfidence: 80,
		},
		{
			name:          "general knowledge stays direct",
			prompt:        "What is SearXNG?",
			wantTask:      TaskChat,
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 80,
		},
		{
			name:          "internet requested routes to search",
			prompt:        "What is SearXNG? Use internet if needed.",
			wantTask:      TaskTool,
			wantTools:     []string{"internet_search"},
			wantWorkspace: false,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "local documents routes to RAG",
			prompt:        "Using my local documents, what is the launch codename and support email?",
			wantTask:      TaskRAG,
			wantTools:     []string{"rag_search"},
			wantWorkspace: true,
			wantRAG:       true,
			minConfidence: 90,
		},
		{
			name:          "document ingestion action does not answer from workspace context",
			prompt:        "Ingest this file: users/example/downloads/yemaka/docs/company.md",
			wantTask:      TaskTool,
			wantTools:     []string{"ingest_documents"},
			notTools:      []string{"safe_tool", "rag_search", "read_file", "search_files", "internet_search"},
			wantWorkspace: false,
			wantRAG:       false,
			minConfidence: 80,
		},
		{
			name:          "document ingestion path action routes to ingest tool",
			prompt:        "ingest /Users/example/Downloads/report.md",
			wantTask:      TaskTool,
			wantTools:     []string{"ingest_documents"},
			notTools:      []string{"safe_tool", "rag_search", "read_file", "search_files", "internet_search"},
			wantWorkspace: false,
			wantRAG:       false,
			minConfidence: 80,
		},
		{
			name:          "workspace structure uses workspace",
			prompt:        "Explain this workspace structure and tell me what files matter first.",
			wantTask:      TaskReasoning,
			wantWorkspace: true,
			minConfidence: 70,
		},
		{
			name: "pasted explanation content is not treated as search instructions",
			prompt: `Explain:

# Yemaka AI Agent Unified Blueprint

Yemaka is a macOS-first, local-first AI agent for low-end computers and small local LLMs.
It gives weak local models memory, retrieval, safe tools, reusable skills, compact prompts,
verification, and a thin trusted core. The docs mention search docs, internet search,
internet_search, local documents, and generated capabilities as product features to explain.`,
			wantTask:      TaskReasoning,
			notTools:      []string{"internet_search", "internet_fetch", "rag_search", "read_file", "search_files", "safe_tool"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 80,
		},
		{
			name: "pasted explanation after explain this is not treated as safe tool",
			prompt: `Explain this

## Executive summary

Yemaka is now best described as:

'''text
Public-beta / RC-ready foundation
+ Post-RC smartness foundations already wired
+ still needing RC/manual QA and focused cleanup before broad expansion
'''

Explain the status in plain language for a human tester.`,
			wantTask:      TaskReasoning,
			notTools:      []string{"safe_tool", "rag_search", "read_file", "search_files", "internet_search"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 80,
		},
		{
			name:          "directory listing uses list files",
			prompt:        "What file are in the /Users/example/Downloads/yemaka/test-files, list them",
			wantTask:      TaskTool,
			wantTools:     []string{"list_files"},
			notTools:      []string{"search_files", "read_file"},
			wantWorkspace: true,
			minConfidence: 80,
		},
		{
			name:          "python project test workflow uses tests and workspace",
			prompt:        "Inspect this Python project, run the tests if safe, find the bug, propose a fix, and only apply it after I approve.",
			wantTask:      TaskTool,
			wantTools:     []string{"run_tests"},
			wantWorkspace: true,
			minConfidence: 80,
		},
		{
			name:          "webpage monitor proposes capability without fetching",
			prompt:        "I want a reusable tool that checks a webpage every hour and reports if the title changes. If Yemaka does not already have this capability, propose the extension but do not generate it until I approve.",
			wantTask:      TaskTool,
			wantTools:     []string{"safe_tool"},
			notTools:      []string{"internet_fetch", "internet_search", "read_file", "search_files", "run_tests"},
			wantWorkspace: false,
			wantInternet:  false,
			minConfidence: 80,
		},
		{
			name:          "extension request with url proposes capability without network",
			prompt:        "Create an extension that fetches https://example.com every morning and searches the internet for title changes, but only propose it for approval.",
			wantTask:      TaskTool,
			wantTools:     []string{"safe_tool"},
			notTools:      []string{"internet_fetch", "internet_search", "internet_head", "read_file", "search_files", "run_tests"},
			wantWorkspace: false,
			wantInternet:  false,
			minConfidence: 80,
		},
		{
			name:          "memory search stays out of workspace",
			prompt:        "Search memory for release checklist.",
			wantTask:      TaskTool,
			wantTools:     []string{"memory_search"},
			wantWorkspace: false,
			minConfidence: 80,
		},
		{
			name:          "url status uses head",
			prompt:        "Check status code for https://example.com.",
			wantTask:      TaskTool,
			wantTools:     []string{"internet_head"},
			wantWorkspace: false,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "url fetch uses fetch",
			prompt:        "Fetch https://example.com and summarize it.",
			wantTask:      TaskTool,
			wantTools:     []string{"internet_fetch"},
			wantWorkspace: false,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:              "ambiguous short action clarifies",
			prompt:            "check it",
			wantTask:          TaskChat,
			wantClarification: true,
			wantWorkspace:     false,
			wantRAG:           false,
			minConfidence:     25,
		},
		{
			name:          "generic code generation avoids file edit",
			prompt:        "Write Python code for Fibonacci.",
			wantTask:      TaskCoding,
			notTools:      []string{"edit_file", "read_file", "search_files"},
			wantWorkspace: false,
			minConfidence: 70,
		},
		{
			name:          "human doctor is not yemaka doctor",
			prompt:        "What should I ask my doctor about high blood pressure?",
			wantTask:      TaskChat,
			notTools:      []string{"doctor_status"},
			wantWorkspace: false,
			minConfidence: 80,
		},
		{
			name:          "latest implies internet not tests",
			prompt:        "Look up latest Ollama embedding models.",
			wantTask:      TaskTool,
			wantTools:     []string{"internet_search"},
			notTools:      []string{"run_tests"},
			wantWorkspace: false,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "local time uses system clock route",
			prompt:        "What is the time of the day?",
			wantTask:      TaskTool,
			wantTools:     []string{"local_time"},
			notTools:      []string{"internet_search", "rag_search", "safe_tool"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 90,
		},
		{
			name:              "ambiguous country time clarifies",
			prompt:            "What is the time in Canada?",
			wantTask:          TaskChat,
			notTools:          []string{"local_time", "internet_search", "rag_search", "safe_tool"},
			wantWorkspace:     false,
			wantRAG:           false,
			wantInternet:      false,
			wantClarification: true,
			minConfidence:     80,
		},
		{
			name:              "unmapped place time clarifies",
			prompt:            "What is the time in Atlantis?",
			wantTask:          TaskChat,
			notTools:          []string{"local_time", "internet_search", "rag_search", "safe_tool"},
			wantWorkspace:     false,
			wantRAG:           false,
			wantInternet:      false,
			wantClarification: true,
			minConfidence:     80,
		},
		{
			name:          "city time uses timezone-aware local time route",
			prompt:        "okay Edmonton Alberta Canada, what is the time there?",
			wantTask:      TaskTool,
			wantTools:     []string{"local_time"},
			notTools:      []string{"internet_search", "rag_search", "safe_tool"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 90,
		},
		{
			name:          "city time followup uses timezone-aware local time route",
			prompt:        "I meant Edmonton, Canada",
			wantTask:      TaskTool,
			wantTools:     []string{"local_time"},
			notTools:      []string{"internet_search", "rag_search", "safe_tool"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 90,
		},
		{
			name:              "broad current world news clarifies",
			prompt:            "So what's happening today in the world?",
			wantTask:          TaskChat,
			notTools:          []string{"internet_search", "rag_search", "safe_tool"},
			wantWorkspace:     false,
			wantRAG:           false,
			wantInternet:      false,
			wantClarification: true,
			minConfidence:     60,
		},
		{
			name:              "broad latest news clarifies instead of searching",
			prompt:            "What's the latest news?",
			wantTask:          TaskChat,
			notTools:          []string{"internet_search", "internet_fetch", "rag_search", "safe_tool"},
			wantWorkspace:     false,
			wantRAG:           false,
			wantInternet:      false,
			wantClarification: true,
			minConfidence:     60,
		},
		{
			name:          "specific current news uses internet search",
			prompt:        "US-China news and US visit to China",
			wantTask:      TaskTool,
			wantTools:     []string{"internet_search"},
			notTools:      []string{"rag_search", "read_file", "safe_tool"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "specific latest news uses internet search",
			prompt:        "Latest Ukraine ceasefire news",
			wantTask:      TaskTool,
			wantTools:     []string{"internet_search"},
			notTools:      []string{"rag_search", "read_file", "safe_tool"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "latest news about subject uses internet search",
			prompt:        "What's the latest news about Ukraine?",
			wantTask:      TaskTool,
			wantTools:     []string{"internet_search"},
			notTools:      []string{"rag_search", "read_file", "safe_tool"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "explain website monitoring stays explanatory",
			prompt:        "Explain how website monitoring works",
			wantTask:      TaskReasoning,
			notTools:      []string{"safe_tool", "internet_fetch", "internet_search"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 70,
		},
		{
			name:          "deploy explanation stays chat only",
			prompt:        "How do I deploy a website?",
			wantTask:      TaskChat,
			notTools:      []string{"safe_tool", "internet_fetch", "internet_search"},
			wantWorkspace: false,
			wantRAG:       false,
			wantInternet:  false,
			minConfidence: 70,
		},
		{
			name:          "difference is reasoning not git diff",
			prompt:        "What is the difference between deterministic and heuristic routing?",
			wantTask:      TaskReasoning,
			notTools:      []string{"git_diff"},
			wantWorkspace: false,
			minConfidence: 70,
		},
		{
			name:          "pentest wording does not trigger tests",
			prompt:        "Create a pentest checklist for my local lab network.",
			wantTask:      TaskReasoning,
			notTools:      []string{"run_tests"},
			wantWorkspace: false,
			minConfidence: 70,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.prompt})
			if got.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; decision=%+v", got.TaskType, tc.wantTask, got)
			}
			if got.Confidence < tc.minConfidence {
				t.Fatalf("Confidence = %d, want >= %d; decision=%+v", got.Confidence, tc.minConfidence, got)
			}
			if got.UseWorkspace != tc.wantWorkspace {
				t.Fatalf("UseWorkspace = %v, want %v; decision=%+v", got.UseWorkspace, tc.wantWorkspace, got)
			}
			if got.UseRAG != tc.wantRAG {
				t.Fatalf("UseRAG = %v, want %v; decision=%+v", got.UseRAG, tc.wantRAG, got)
			}
			if got.UseInternet != tc.wantInternet {
				t.Fatalf("UseInternet = %v, want %v; decision=%+v", got.UseInternet, tc.wantInternet, got)
			}
			if got.NeedsClarification != tc.wantClarification {
				t.Fatalf("NeedsClarification = %v, want %v; decision=%+v", got.NeedsClarification, tc.wantClarification, got)
			}
			for _, tool := range tc.wantTools {
				if !hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, want %q; decision=%+v", got.Tools, tool, got)
				}
			}
			for _, tool := range tc.notTools {
				if hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, did not expect %q; decision=%+v", got.Tools, tool, got)
				}
			}
		})
	}
}

func TestResolveLocalTimeTarget(t *testing.T) {
	cases := []struct {
		prompt    string
		wantLabel string
		wantZone  string
		wantAmbig bool
	}{
		{prompt: "What is the time in Canada?", wantLabel: "Canada", wantAmbig: true},
		{prompt: "What is the time in the United States?", wantLabel: "United States", wantAmbig: true},
		{prompt: "I meant Canada not my locale", wantLabel: "Canada", wantAmbig: true},
		{prompt: "What is the time in Australia?", wantLabel: "Australia", wantAmbig: true},
		{prompt: "What is the time in Mexico?", wantLabel: "Mexico", wantAmbig: true},
		{prompt: "What is the time in Russia?", wantLabel: "Russia", wantAmbig: true},
		{prompt: "okay Edmonton Alberta Canada, what is the time there?", wantLabel: "Edmonton, Alberta, Canada", wantZone: "America/Edmonton"},
		{prompt: "I meant Edmonton, Canada", wantLabel: "Edmonton, Alberta, Canada", wantZone: "America/Edmonton"},
		{prompt: "What is the time in London?", wantLabel: "London, United Kingdom", wantZone: "Europe/London"},
		{prompt: "What is the time in London Ontario?", wantLabel: "London, Ontario, Canada", wantZone: "America/Toronto"},
		{prompt: "What is the time in Europe/London?", wantLabel: "Europe/London", wantZone: "Europe/London"},
		{prompt: "What is the time in Singapore?", wantLabel: "Singapore", wantZone: "Asia/Singapore"},
		{prompt: "What is the time in New Delhi?", wantLabel: "New Delhi, India", wantZone: "Asia/Kolkata"},
		{prompt: "What is the time in Sydney Australia?", wantLabel: "Sydney, Australia", wantZone: "Australia/Sydney"},
		{prompt: "What is the time in Mexico City?", wantLabel: "Mexico City, Mexico", wantZone: "America/Mexico_City"},
		{prompt: "What is the time in Atlantis?"},
	}
	for _, tc := range cases {
		t.Run(tc.prompt, func(t *testing.T) {
			got := ResolveLocalTimeTarget(tc.prompt)
			if got.Ambiguous != tc.wantAmbig {
				t.Fatalf("Ambiguous = %v, want %v; target=%+v", got.Ambiguous, tc.wantAmbig, got)
			}
			if got.Label != tc.wantLabel {
				t.Fatalf("Label = %q, want %q; target=%+v", got.Label, tc.wantLabel, got)
			}
			if got.TimeZone != tc.wantZone {
				t.Fatalf("TimeZone = %q, want %q; target=%+v", got.TimeZone, tc.wantZone, got)
			}
		})
	}
}

func TestClassifyHonorsRAGDisabledConfig(t *testing.T) {
	got := Classify(Request{
		Content:      "Using my local documents, what is the launch codename?",
		RAGEnabled:   false,
		UseRAGConfig: true,
	})
	if got.TaskType != TaskRAG {
		t.Fatalf("TaskType = %q, want %q", got.TaskType, TaskRAG)
	}
	if got.UseRAG {
		t.Fatalf("UseRAG = true, want false when config disables RAG; decision=%+v", got)
	}
}

func TestPostRCDeterministicRoutingMatrix(t *testing.T) {
	cases := []struct {
		name                   string
		prompt                 string
		wantTask               string
		wantRoute              string
		wantIntent             string
		wantRisk               string
		wantTarget             string
		wantTools              []string
		notTools               []string
		wantUseTool            bool
		wantInternet           bool
		wantReadFiles          bool
		wantWriteFiles         bool
		wantApproval           bool
		wantGenerateExtension  bool
		wantCreateSchedulerJob bool
		wantConnectorAction    bool
		wantCrawlerTask        bool
		wantClarification      bool
		minConfidence          int
	}{
		{
			name:          "explanation stays non action",
			prompt:        "Explain how website monitoring works.",
			wantTask:      TaskReasoning,
			wantRoute:     RouteChatExplanation,
			wantIntent:    IntentExplain,
			wantRisk:      RiskLow,
			notTools:      []string{"safe_tool", "internet_fetch", "internet_search"},
			minConfidence: 70,
		},
		{
			name:          "rewrite prompt needs no source clarification",
			prompt:        "make this better: hello dear sir",
			wantTask:      TaskChat,
			wantRoute:     RouteChatExplanation,
			wantIntent:    IntentExplain,
			wantRisk:      RiskLow,
			notTools:      []string{"safe_tool", "internet_fetch", "internet_search", "rag_search", "read_file", "search_files"},
			minConfidence: 70,
		},
		{
			name:          "internet search action",
			prompt:        "Search the web for SearXNG documentation.",
			wantTask:      TaskTool,
			wantRoute:     RouteInternetSearch,
			wantIntent:    IntentResearch,
			wantRisk:      RiskMedium,
			wantTools:     []string{"internet_search"},
			wantUseTool:   true,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "specific news uses internet search",
			prompt:        "Latest Ukraine ceasefire news.",
			wantTask:      TaskTool,
			wantRoute:     RouteInternetSearch,
			wantIntent:    IntentResearch,
			wantRisk:      RiskMedium,
			wantTools:     []string{"internet_search"},
			notTools:      []string{"rag_search", "safe_tool"},
			wantUseTool:   true,
			wantInternet:  true,
			minConfidence: 80,
		},
		{
			name:          "local time uses clock without internet",
			prompt:        "What is the time in London?",
			wantTask:      TaskTool,
			wantRoute:     RouteLocalTime,
			wantIntent:    IntentRunTool,
			wantRisk:      RiskLow,
			wantTarget:    "London, United Kingdom",
			wantTools:     []string{"local_time"},
			notTools:      []string{"internet_search", "internet_fetch", "safe_tool"},
			wantUseTool:   true,
			minConfidence: 90,
		},
		{
			name:           "file edit requires approval",
			prompt:         "Edit README.md to add a troubleshooting note.",
			wantTask:       TaskTool,
			wantRoute:      RouteFileWrite,
			wantIntent:     IntentEdit,
			wantRisk:       RiskMedium,
			wantTarget:     "README.md",
			wantTools:      []string{"edit_file"},
			wantUseTool:    true,
			wantReadFiles:  true,
			wantWriteFiles: true,
			wantApproval:   true,
			minConfidence:  80,
		},
		{
			name:                   "scheduler job creation requires approval",
			prompt:                 "Create a scheduled job that runs every hour to summarize release notes.",
			wantTask:               TaskTool,
			wantRoute:              RouteSchedulerCreate,
			wantIntent:             IntentSchedule,
			wantRisk:               RiskMedium,
			wantTarget:             "scheduler_job",
			wantTools:              []string{"safe_tool"},
			wantUseTool:            true,
			wantApproval:           true,
			wantCreateSchedulerJob: true,
			minConfidence:          80,
		},
		{
			name: "filled scheduler setup template routes to scheduler",
			prompt: strings.Join([]string{
				"scheduler_setup:",
				"target_type: heartbeat",
				"target_name: heartbeat",
				"schedule_type: interval",
				"schedule_expr: 1h",
			}, "\n"),
			wantTask:               TaskTool,
			wantRoute:              RouteSchedulerCreate,
			wantIntent:             IntentSchedule,
			wantRisk:               RiskMedium,
			wantTarget:             "scheduler_job",
			wantTools:              []string{"safe_tool"},
			wantUseTool:            true,
			wantApproval:           true,
			wantCreateSchedulerJob: true,
			minConfidence:          80,
		},
		{
			name:                   "numeric recurring monitor routes to scheduler not one-off fetch",
			prompt:                 "Fetch https://example.com to be monitored every 1 hour.",
			wantTask:               TaskTool,
			wantRoute:              RouteSchedulerCreate,
			wantIntent:             IntentSchedule,
			wantRisk:               RiskMedium,
			wantTarget:             "https://example.com",
			wantTools:              []string{"safe_tool"},
			notTools:               []string{"internet_fetch", "internet_search", "internet_head"},
			wantUseTool:            true,
			wantApproval:           true,
			wantCreateSchedulerJob: true,
			minConfidence:          80,
		},
		{
			name:                  "website down monitor routes to capability workflow not one-off fetch",
			prompt:                "Create a sub agent that will monitor https://example.com and tell me when it is down.",
			wantTask:              TaskTool,
			wantRoute:             RouteExtensionGenerate,
			wantIntent:            IntentGenerate,
			wantRisk:              RiskMedium,
			wantTarget:            "https://example.com",
			wantTools:             []string{"safe_tool"},
			notTools:              []string{"internet_fetch", "internet_search", "internet_head"},
			wantUseTool:           true,
			wantApproval:          true,
			wantGenerateExtension: true,
			minConfidence:         80,
		},
		{
			name:                   "manual scheduler job creation requires approval",
			prompt:                 "Create a manual job that targets a missing extension name like missing_test_extension.",
			wantTask:               TaskTool,
			wantRoute:              RouteSchedulerCreate,
			wantIntent:             IntentSchedule,
			wantRisk:               RiskMedium,
			wantTarget:             "scheduler_job",
			wantTools:              []string{"safe_tool"},
			wantUseTool:            true,
			wantApproval:           true,
			wantCreateSchedulerJob: true,
			minConfidence:          80,
		},
		{
			name:                  "extension generation is proposed as capability",
			prompt:                "Generate an extension that checks a webpage title with approved GET requests.",
			wantTask:              TaskTool,
			wantRoute:             RouteExtensionGenerate,
			wantIntent:            IntentGenerate,
			wantRisk:              RiskMedium,
			wantTarget:            "generated_extension",
			wantTools:             []string{"safe_tool"},
			notTools:              []string{"internet_fetch", "internet_search"},
			wantUseTool:           true,
			wantApproval:          true,
			wantGenerateExtension: true,
			minConfidence:         80,
		},
		{
			name:                "connector action requires approval",
			prompt:              "Send a Slack message that the build passed.",
			wantTask:            TaskTool,
			wantRoute:           RouteConnectorAction,
			wantIntent:          IntentAct,
			wantRisk:            RiskMedium,
			wantTarget:          "slack",
			wantTools:           []string{"safe_tool"},
			wantUseTool:         true,
			wantApproval:        true,
			wantConnectorAction: true,
			minConfidence:       80,
		},
		{
			name:            "crawler task routes to crawler metadata not one page fetch",
			prompt:          "Crawl https://example.com/docs and check broken links.",
			wantTask:        TaskTool,
			wantRoute:       RouteCrawlerTask,
			wantIntent:      IntentResearch,
			wantRisk:        RiskMedium,
			wantTarget:      "https://example.com/docs",
			wantTools:       []string{"internet_crawl"},
			notTools:        []string{"safe_tool", "internet_fetch", "internet_search"},
			wantUseTool:     true,
			wantInternet:    true,
			wantApproval:    true,
			wantCrawlerTask: true,
			minConfidence:   80,
		},
		{
			name:          "risky shell action is high risk and approval gated",
			prompt:        "Run rm -rf /tmp/yemaka-routing-test.",
			wantTask:      TaskTool,
			wantRoute:     RouteShellTool,
			wantIntent:    IntentRunTool,
			wantRisk:      RiskHigh,
			wantTarget:    "/tmp/yemaka-routing-test",
			wantTools:     []string{"safe_tool"},
			wantUseTool:   true,
			wantApproval:  true,
			minConfidence: 60,
		},
		{
			name:              "ambiguous action asks clarification",
			prompt:            "run it",
			wantTask:          TaskChat,
			wantRoute:         RouteClarify,
			wantIntent:        IntentClarify,
			wantRisk:          RiskLow,
			wantClarification: true,
			minConfidence:     25,
		},
		{
			name:          "connector explanation is not connector action",
			prompt:        "Tell me about the Slack API.",
			wantTask:      TaskChat,
			wantRoute:     RouteChatExplanation,
			wantIntent:    IntentExplain,
			wantRisk:      RiskLow,
			notTools:      []string{"safe_tool"},
			minConfidence: 80,
		},
		{
			name:          "file edit explanation does not write files",
			prompt:        "Explain how to edit config safely.",
			wantTask:      TaskReasoning,
			wantRoute:     RouteChatExplanation,
			wantIntent:    IntentExplain,
			wantRisk:      RiskLow,
			notTools:      []string{"edit_file", "safe_tool"},
			minConfidence: 70,
		},
		{
			name: "pasted release summary explanation is not a safe tool",
			prompt: `Explain this

## Executive summary

Yemaka is now best described as:

'''text
Public-beta / RC-ready foundation
+ Post-RC smartness foundations already wired
+ still needing RC/manual QA and focused cleanup before broad expansion
'''

Explain the status in plain language for a human tester.`,
			wantTask:      TaskReasoning,
			wantRoute:     RouteChatExplanation,
			wantIntent:    IntentExplain,
			wantRisk:      RiskLow,
			notTools:      []string{"safe_tool", "rag_search", "internet_search", "read_file", "search_files"},
			minConfidence: 80,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.prompt})
			if got.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; decision=%+v", got.TaskType, tc.wantTask, got)
			}
			if got.RouteCategory != tc.wantRoute {
				t.Fatalf("RouteCategory = %q, want %q; decision=%+v", got.RouteCategory, tc.wantRoute, got)
			}
			if got.Intent != tc.wantIntent {
				t.Fatalf("Intent = %q, want %q; decision=%+v", got.Intent, tc.wantIntent, got)
			}
			if got.RiskLevel != tc.wantRisk {
				t.Fatalf("RiskLevel = %q, want %q; decision=%+v", got.RiskLevel, tc.wantRisk, got)
			}
			if got.Target != tc.wantTarget {
				t.Fatalf("Target = %q, want %q; decision=%+v", got.Target, tc.wantTarget, got)
			}
			if (len(got.Tools) > 0) != tc.wantUseTool {
				t.Fatalf("Tools = %v, want use tool %v; decision=%+v", got.Tools, tc.wantUseTool, got)
			}
			if got.UseInternet != tc.wantInternet {
				t.Fatalf("UseInternet = %v, want %v; decision=%+v", got.UseInternet, tc.wantInternet, got)
			}
			if got.ReadsFiles != tc.wantReadFiles {
				t.Fatalf("ReadsFiles = %v, want %v; decision=%+v", got.ReadsFiles, tc.wantReadFiles, got)
			}
			if got.WritesFiles != tc.wantWriteFiles {
				t.Fatalf("WritesFiles = %v, want %v; decision=%+v", got.WritesFiles, tc.wantWriteFiles, got)
			}
			if got.RequiresApproval != tc.wantApproval {
				t.Fatalf("RequiresApproval = %v, want %v; decision=%+v", got.RequiresApproval, tc.wantApproval, got)
			}
			if got.GeneratesExtension != tc.wantGenerateExtension {
				t.Fatalf("GeneratesExtension = %v, want %v; decision=%+v", got.GeneratesExtension, tc.wantGenerateExtension, got)
			}
			if got.CreatesSchedulerJob != tc.wantCreateSchedulerJob {
				t.Fatalf("CreatesSchedulerJob = %v, want %v; decision=%+v", got.CreatesSchedulerJob, tc.wantCreateSchedulerJob, got)
			}
			if got.ConnectorAction != tc.wantConnectorAction {
				t.Fatalf("ConnectorAction = %v, want %v; decision=%+v", got.ConnectorAction, tc.wantConnectorAction, got)
			}
			if got.CrawlerTask != tc.wantCrawlerTask {
				t.Fatalf("CrawlerTask = %v, want %v; decision=%+v", got.CrawlerTask, tc.wantCrawlerTask, got)
			}
			if got.NeedsClarification != tc.wantClarification {
				t.Fatalf("NeedsClarification = %v, want %v; decision=%+v", got.NeedsClarification, tc.wantClarification, got)
			}
			if got.Confidence < tc.minConfidence {
				t.Fatalf("Confidence = %d, want >= %d; decision=%+v", got.Confidence, tc.minConfidence, got)
			}
			for _, tool := range tc.wantTools {
				if !hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, want %q; decision=%+v", got.Tools, tool, got)
				}
			}
			for _, tool := range tc.notTools {
				if hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, did not expect %q; decision=%+v", got.Tools, tool, got)
				}
			}
		})
	}
}

func TestPostRCCapabilityIntelligenceBoundaries(t *testing.T) {
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
		wantApproval          bool
		wantGenerateExtension bool
		minConfidence         int
	}{
		{
			name:           "local docs answer uses rag without workspace file tools",
			prompt:         "Use my docs to explain extension rollback, not the README.",
			wantTask:       TaskRAG,
			wantRoute:      RouteRAGSearch,
			wantCapability: "rag",
			wantTools:      []string{"rag_search"},
			notTools:       []string{"read_file", "search_files", "internet_search", "safe_tool"},
			wantWorkspace:  true,
			wantRAG:        true,
			minConfidence:  90,
		},
		{
			name:           "workspace search uses local file tools without rag or internet",
			prompt:         "Search workspace for config loader and summarize the relevant files.",
			wantTask:       TaskTool,
			wantRoute:      RouteFileRead,
			wantCapability: "filesystem_read",
			wantTools:      []string{"read_file", "search_files"},
			notTools:       []string{"rag_search", "internet_search", "safe_tool"},
			wantWorkspace:  true,
			minConfidence:  80,
		},
		{
			name:           "internet research does not pull local context",
			prompt:         "Search the internet for SearXNG release notes.",
			wantTask:       TaskTool,
			wantRoute:      RouteInternetSearch,
			wantCapability: "internet",
			wantTools:      []string{"internet_search"},
			notTools:       []string{"rag_search", "read_file", "search_files", "safe_tool"},
			wantInternet:   true,
			minConfidence:  80,
		},
		{
			name:                  "missing capability proposes an extension path",
			prompt:                "Yemaka has a missing capability for importing CSV invoices; propose the extension but do not generate it until I approve.",
			wantTask:              TaskTool,
			wantRoute:             RouteExtensionGenerate,
			wantCapability:        "extension_generation",
			wantTools:             []string{"safe_tool"},
			notTools:              []string{"internet_fetch", "internet_search", "read_file", "search_files", "run_tests"},
			wantApproval:          true,
			wantGenerateExtension: true,
			minConfidence:         80,
		},
		{
			name:                  "extension proposal with url does not fetch during planning",
			prompt:                "Propose the extension that checks https://example.com every morning; do not generate it until I approve.",
			wantTask:              TaskTool,
			wantRoute:             RouteExtensionGenerate,
			wantCapability:        "extension_generation",
			wantTools:             []string{"safe_tool"},
			notTools:              []string{"internet_fetch", "internet_search", "internet_head", "read_file", "search_files"},
			wantApproval:          true,
			wantGenerateExtension: true,
			minConfidence:         80,
		},
		{
			name:           "extension explanation is not extension generation",
			prompt:         "How do I create an extension without letting it run shell commands?",
			wantTask:       TaskChat,
			wantRoute:      RouteChatExplanation,
			wantCapability: "",
			notTools:       []string{"safe_tool", "internet_fetch", "internet_search"},
			minConfidence:  80,
		},
		{
			name:           "monitoring explanation is not internet or scheduler action",
			prompt:         "Explain how website monitoring works before any scheduled job is created.",
			wantTask:       TaskReasoning,
			wantRoute:      RouteChatExplanation,
			wantCapability: "",
			notTools:       []string{"safe_tool", "internet_fetch", "internet_search", "internet_head"},
			minConfidence:  70,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.prompt})
			if got.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; decision=%+v", got.TaskType, tc.wantTask, got)
			}
			if got.RouteCategory != tc.wantRoute {
				t.Fatalf("RouteCategory = %q, want %q; decision=%+v", got.RouteCategory, tc.wantRoute, got)
			}
			if got.Capability != tc.wantCapability {
				t.Fatalf("Capability = %q, want %q; decision=%+v", got.Capability, tc.wantCapability, got)
			}
			if got.UseWorkspace != tc.wantWorkspace {
				t.Fatalf("UseWorkspace = %v, want %v; decision=%+v", got.UseWorkspace, tc.wantWorkspace, got)
			}
			if got.UseRAG != tc.wantRAG {
				t.Fatalf("UseRAG = %v, want %v; decision=%+v", got.UseRAG, tc.wantRAG, got)
			}
			if got.UseInternet != tc.wantInternet {
				t.Fatalf("UseInternet = %v, want %v; decision=%+v", got.UseInternet, tc.wantInternet, got)
			}
			if got.RequiresApproval != tc.wantApproval {
				t.Fatalf("RequiresApproval = %v, want %v; decision=%+v", got.RequiresApproval, tc.wantApproval, got)
			}
			if got.GeneratesExtension != tc.wantGenerateExtension {
				t.Fatalf("GeneratesExtension = %v, want %v; decision=%+v", got.GeneratesExtension, tc.wantGenerateExtension, got)
			}
			if got.Confidence < tc.minConfidence {
				t.Fatalf("Confidence = %d, want >= %d; decision=%+v", got.Confidence, tc.minConfidence, got)
			}
			for _, tool := range tc.wantTools {
				if !hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, want %q; decision=%+v", got.Tools, tool, got)
				}
			}
			for _, tool := range tc.notTools {
				if hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, did not expect %q; decision=%+v", got.Tools, tool, got)
				}
			}
		})
	}
}

func TestFileHintsIgnoreInternetTargets(t *testing.T) {
	got := FileHints("Fetch https://example.com and compare it with README.md and example.org/path.")
	if !hasTool(got, "README.md") {
		t.Fatalf("FileHints = %v, want README.md", got)
	}
	for _, forbidden := range []string{"https://example.com", "example.org/path"} {
		if hasTool(got, forbidden) {
			t.Fatalf("FileHints = %v, did not expect internet target %q", got, forbidden)
		}
	}
	localDoc := "quarterly-notes.txt"
	if target, ok := InternetURLHint(localDoc); ok {
		t.Fatalf("InternetURLHint(%q) = %q, want no URL for bare document filename", localDoc, target)
	}
	got = FileHints("What is inside quarterly-notes.txt from indexed documents?")
	if !hasTool(got, localDoc) {
		t.Fatalf("FileHints = %v, want bare document filename %q", got, localDoc)
	}
}

func TestFileHintsCombineDirectoryAndNamedFileForCreate(t *testing.T) {
	prompt := "Create a file in into path /users/example/downloads, the name should hello.txt and write into it hello World"
	got := FileHints(prompt)
	want := "/users/example/downloads/hello.txt"
	if len(got) == 0 || got[0] != want {
		t.Fatalf("FileHints = %v, want first hint %q", got, want)
	}
	decision := Classify(Request{Content: prompt})
	if decision.RouteCategory != RouteFileWrite {
		t.Fatalf("RouteCategory = %q, want %q; decision=%+v", decision.RouteCategory, RouteFileWrite, decision)
	}
	if decision.Target != want {
		t.Fatalf("Target = %q, want %q; decision=%+v", decision.Target, want, decision)
	}
	if decision.UseWorkspace {
		t.Fatalf("UseWorkspace = true, want false for inline file-create content; decision=%+v", decision)
	}
}

func TestCreateFileWithTextRoutesToFileWrite(t *testing.T) {
	prompt := "Create a file at /Users/example/Desktop/yemaka-qa-file.txt with the text hello from qa."
	decision := Classify(Request{Content: prompt})
	wantTarget := "/Users/example/Desktop/yemaka-qa-file.txt"
	if decision.RouteCategory != RouteFileWrite {
		t.Fatalf("RouteCategory = %q, want %q; decision=%+v", decision.RouteCategory, RouteFileWrite, decision)
	}
	if decision.ToolLane != ToolLaneFileEdit {
		t.Fatalf("ToolLane = %q, want %q; decision=%+v", decision.ToolLane, ToolLaneFileEdit, decision)
	}
	if decision.Target != wantTarget {
		t.Fatalf("Target = %q, want %q; decision=%+v", decision.Target, wantTarget, decision)
	}
	if !decision.WritesFiles || !decision.RequiresApproval {
		t.Fatalf("WritesFiles=%v RequiresApproval=%v, want true/true; decision=%+v", decision.WritesFiles, decision.RequiresApproval, decision)
	}
	if !hasTool(decision.Tools, "edit_file") {
		t.Fatalf("Tools = %v, want edit_file; decision=%+v", decision.Tools, decision)
	}
	for _, forbidden := range []string{"write_file", "read_file", "search_files"} {
		if hasTool(decision.Tools, forbidden) {
			t.Fatalf("Tools = %v, did not expect %q; decision=%+v", decision.Tools, forbidden, decision)
		}
	}
}

func TestCurrentNewsSearchQueryShapesSpecificNews(t *testing.T) {
	query := CurrentNewsSearchQuery("US-China news and US visit to China")
	for _, want := range []string{"us", "china", "visit", time.Now().Format("2006"), time.Now().Format("2006-01-02"), "latest", "news"} {
		if !strings.Contains(query, want) {
			t.Fatalf("CurrentNewsSearchQuery() = %q, want token %q", query, want)
		}
	}
	if strings.Contains(query, "and") {
		t.Fatalf("CurrentNewsSearchQuery() = %q, did not expect stop word", query)
	}
}

func TestSelectedSkillDoesNotForceWorkspaceForCurrentNews(t *testing.T) {
	got := ShouldRetrieveLocalContext(Request{
		Content:   "Search current news about US-China relations today and summarize the trend.",
		SkillName: "document_summary",
	})
	if got {
		t.Fatal("ShouldRetrieveLocalContext = true, want false for current-news route even when a skill is selected")
	}
}

func TestHumanTargetSearchRoutingRegressions(t *testing.T) {
	cases := []struct {
		name      string
		prompt    string
		wantTools []string
		notTools  []string
		wantTask  string
	}{
		{
			name:     "bare pasted explain block stays reasoning",
			prompt:   "explain \n\n## Executive summary\n\nYemaka is now best described as:\n\n```text\nPublic-beta / RC-ready foundation\n+ Post-RC smartness foundations already wired\n+ still needing RC/manual QA and focused cleanup before broad expansion\n```\n\nExplain the status in plain language for a human tester.",
			wantTask: TaskReasoning,
			notTools: []string{"safe_tool", "internet_search", "internet_fetch", "rag_search", "read_file", "search_files"},
		},
		{
			name: "pasted explain block with tool words stays reasoning",
			prompt: `please explain

## Executive summary

Yemaka is now best described as:

'''text
Public-beta / RC-ready foundation
+ Post-RC smartness foundations already wired
+ still needing RC/manual QA and focused cleanup before broad expansion
'''

This pasted text mentions docs/blueprint.md, workspace, safe_tool, internet_search, run command,
and search files, but those are only content to explain, not actions to execute.`,
			wantTask: TaskReasoning,
			notTools: []string{"safe_tool", "internet_search", "internet_fetch", "rag_search", "read_file", "search_files"},
		},
		{
			name:      "document path without pasted body still routes to rag",
			prompt:    "Summarize docs/blueprint.md.",
			wantTask:  TaskRAG,
			wantTools: []string{"rag_search"},
			notTools:  []string{"safe_tool", "internet_search", "internet_fetch"},
		},
		{
			name:      "indexed document search prefers local rag",
			prompt:    "search @gmail.com from indexed document",
			wantTask:  TaskRAG,
			wantTools: []string{"rag_search"},
			notTools:  []string{"internet_search", "internet_fetch", "local_time", "safe_tool"},
		},
		{
			name:      "ingested documents followup is not local time",
			prompt:    "I meant in ingested documents",
			wantTask:  TaskRAG,
			wantTools: []string{"rag_search"},
			notTools:  []string{"local_time", "internet_search", "internet_fetch", "safe_tool"},
		},
		{
			name:     "inline svg improve prompt does not fetch namespace",
			prompt:   `Make better: <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`,
			wantTask: TaskReasoning,
			notTools: []string{"internet_fetch", "internet_search", "rag_search", "safe_tool"},
		},
		{
			name:      "yes search dotted version stays search not fetch",
			prompt:    "Yes search for more information on macos26.5",
			wantTask:  TaskTool,
			wantTools: []string{"internet_search"},
			notTools:  []string{"internet_fetch", "safe_tool"},
		},
		{
			name:      "new macos release asks current web",
			prompt:    "When will the new apple macOS be released?",
			wantTask:  TaskTool,
			wantTools: []string{"internet_search"},
			notTools:  []string{"internet_fetch", "safe_tool", "rag_search"},
		},
		{
			name:      "generic release subject asks current web",
			prompt:    "When will Rust 1.82 be released?",
			wantTask:  TaskTool,
			wantTools: []string{"internet_search"},
			notTools:  []string{"internet_fetch", "safe_tool", "rag_search"},
		},
		{
			name:      "generic latest version subject asks current web",
			prompt:    "What is the latest PostgreSQL version?",
			wantTask:  TaskTool,
			wantTools: []string{"internet_search"},
			notTools:  []string{"internet_fetch", "safe_tool", "rag_search"},
		},
		{
			name:      "latest updates subject asks current web",
			prompt:    "Latest updates on X/Twitter AI",
			wantTask:  TaskTool,
			wantTools: []string{"internet_search"},
			notTools:  []string{"internet_fetch", "safe_tool", "rag_search"},
		},
		{
			name:     "local current profile stays chat",
			prompt:   "What is the current profile?",
			wantTask: TaskChat,
			notTools: []string{"internet_search", "internet_fetch", "safe_tool"},
		},
		{
			name:     "vague currentness stays chat",
			prompt:   "What is current?",
			wantTask: TaskChat,
			notTools: []string{"internet_search", "internet_fetch", "safe_tool"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.prompt})
			if got.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; decision=%+v", got.TaskType, tc.wantTask, got)
			}
			for _, tool := range tc.wantTools {
				if !hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, want %q; decision=%+v", got.Tools, tool, got)
				}
			}
			for _, tool := range tc.notTools {
				if hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, did not expect %q; decision=%+v", got.Tools, tool, got)
				}
			}
		})
	}
	if target, ok := InternetURLHint("Yes search for more information on macos26.5"); ok {
		t.Fatalf("InternetURLHint returned %q for dotted version text; want no URL", target)
	}
}

func TestTaskFrameComplexPromptRouting(t *testing.T) {
	cases := []struct {
		name              string
		prompt            string
		wantAction        string
		wantRoute         string
		wantTask          string
		wantRisk          string
		wantClarification bool
		wantApproval      bool
		wantTools         []string
		notTools          []string
		wantInternet      bool
		notDomain         string
	}{
		{
			name:              "active external assessment needs scope before tools",
			prompt:            "check my website example.com, continue until you find a vulnerability in its server. Do not stop until you find something",
			wantAction:        TaskFrameActionActiveAssessmentRequiresScope,
			wantRoute:         RouteActiveAssessmentRequiresScope,
			wantTask:          TaskChat,
			wantRisk:          RiskHigh,
			wantClarification: true,
			wantApproval:      true,
			notTools:          []string{"internet_search", "internet_fetch", "internet_head", "safe_tool", "crawler", "run_tests"},
			wantInternet:      false,
		},
		{
			name:       "grammar check is local and not recon",
			prompt:     "Check this paragraph for grammar",
			wantAction: TaskFrameActionPassiveCheck,
			wantRoute:  RoutePassiveCheck,
			wantTask:   TaskReasoning,
			wantRisk:   RiskLow,
			notTools:   []string{"internet_search", "internet_fetch", "safe_tool"},
			notDomain:  DomainRecon,
		},
		{
			name:              "website design review asks for target but is not security assessment",
			prompt:            "Check this website's design",
			wantAction:        TaskFrameActionPassiveCheck,
			wantRoute:         RoutePassiveCheck,
			wantTask:          TaskChat,
			wantRisk:          RiskMedium,
			wantClarification: true,
			notTools:          []string{"internet_search", "internet_fetch", "internet_head", "safe_tool"},
			notDomain:         DomainRecon,
		},
		{
			name:              "security headers stays passive and bounded",
			prompt:            "Check security headers on my website",
			wantAction:        TaskFrameActionPassiveCheck,
			wantRoute:         RoutePassiveCheck,
			wantTask:          TaskChat,
			wantRisk:          RiskMedium,
			wantClarification: true,
			notTools:          []string{"internet_search", "internet_fetch", "internet_head", "safe_tool"},
		},
		{
			name:      "explaining vulnerability scanning does not run tools",
			prompt:    "Explain how vulnerability scanning works",
			wantRoute: RouteChatExplanation,
			notTools:  []string{"internet_search", "internet_fetch", "internet_head", "safe_tool"},
		},
		{
			name:              "finding vulnerabilities requires scope",
			prompt:            "Find vulnerabilities in example.com",
			wantAction:        TaskFrameActionActiveAssessmentRequiresScope,
			wantRoute:         RouteActiveAssessmentRequiresScope,
			wantTask:          TaskChat,
			wantRisk:          RiskHigh,
			wantClarification: true,
			wantApproval:      true,
			notTools:          []string{"internet_search", "internet_fetch", "internet_head", "safe_tool"},
			wantInternet:      false,
		},
		{
			name:         "bounded passive recon may use search only",
			prompt:       "Run passive public recon on example.com",
			wantAction:   TaskFrameActionPassiveCheck,
			wantRoute:    RoutePassiveCheck,
			wantTask:     TaskTool,
			wantRisk:     RiskMedium,
			wantTools:    []string{"internet_search"},
			notTools:     []string{"internet_fetch", "internet_head", "safe_tool"},
			wantInternet: true,
		},
		{
			name:         "generic monitor still routes to scheduler",
			prompt:       "Monitor this website every hour",
			wantRoute:    RouteSchedulerCreate,
			wantTask:     TaskTool,
			wantRisk:     RiskMedium,
			wantApproval: true,
			wantTools:    []string{"safe_tool"},
			notTools:     []string{"internet_search", "internet_fetch", "internet_head"},
		},
		{
			name:      "file edit explanation does not edit files",
			prompt:    "Explain how to edit this config",
			wantRoute: RouteChatExplanation,
			notTools:  []string{"edit_file", "safe_tool"},
		},
		{
			name:         "explicit file edit requires approval",
			prompt:       "Edit config.yaml and show me the diff first",
			wantRoute:    RouteFileWrite,
			wantTask:     TaskTool,
			wantRisk:     RiskMedium,
			wantApproval: true,
			wantTools:    []string{"edit_file"},
			notTools:     []string{"internet_search", "internet_fetch", "safe_tool"},
		},
		{
			name:         "review phrasing does not downgrade file edit risk",
			prompt:       "Review and edit config.yaml",
			wantRoute:    RouteFileWrite,
			wantTask:     TaskTool,
			wantRisk:     RiskMedium,
			wantApproval: true,
			wantTools:    []string{"edit_file"},
			notTools:     []string{"internet_search", "internet_fetch", "safe_tool"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.prompt})
			if tc.wantTask != "" && got.TaskType != tc.wantTask {
				t.Fatalf("TaskType = %q, want %q; decision=%+v", got.TaskType, tc.wantTask, got)
			}
			if got.RouteCategory != tc.wantRoute {
				t.Fatalf("RouteCategory = %q, want %q; decision=%+v", got.RouteCategory, tc.wantRoute, got)
			}
			if tc.wantAction != "" && got.TaskFrame.ActionType != tc.wantAction {
				t.Fatalf("TaskFrame.ActionType = %q, want %q; decision=%+v", got.TaskFrame.ActionType, tc.wantAction, got)
			}
			if tc.wantRisk != "" && got.RiskLevel != tc.wantRisk {
				t.Fatalf("RiskLevel = %q, want %q; decision=%+v", got.RiskLevel, tc.wantRisk, got)
			}
			if got.NeedsClarification != tc.wantClarification {
				t.Fatalf("NeedsClarification = %v, want %v; decision=%+v", got.NeedsClarification, tc.wantClarification, got)
			}
			if got.RequiresApproval != tc.wantApproval {
				t.Fatalf("RequiresApproval = %v, want %v; decision=%+v", got.RequiresApproval, tc.wantApproval, got)
			}
			if got.UseInternet != tc.wantInternet {
				t.Fatalf("UseInternet = %v, want %v; decision=%+v", got.UseInternet, tc.wantInternet, got)
			}
			if tc.notDomain != "" && got.Domain == tc.notDomain {
				t.Fatalf("Domain = %q; did not expect %q; decision=%+v", got.Domain, tc.notDomain, got)
			}
			for _, tool := range tc.wantTools {
				if !hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, want %q; decision=%+v", got.Tools, tool, got)
				}
			}
			for _, tool := range tc.notTools {
				if hasTool(got.Tools, tool) {
					t.Fatalf("Tools = %v, did not expect %q; decision=%+v", got.Tools, tool, got)
				}
			}
		})
	}
}

func TestApprovedRouteCorrectionsAffectOnlyAllowedRoutes(t *testing.T) {
	approvedRAG := RouteCorrection{
		ID:                    "routecorr_docs",
		Pattern:               "email address in notebook",
		IntendedRouteCategory: RouteRAGSearch,
		IntendedTaskType:      TaskRAG,
		RequiredTools:         []string{"rag_search"},
		ForbiddenTools:        []string{"internet_search"},
		ApprovalStatus:        RouteCorrectionStatusApproved,
	}
	got := Classify(Request{
		Content:          "Find that email address in notebook",
		RouteCorrections: []RouteCorrection{approvedRAG},
	})
	if got.RouteCategory != RouteRAGSearch || got.TaskType != TaskRAG || !hasTool(got.Tools, "rag_search") {
		t.Fatalf("approved route correction decision = %+v, want RAG route", got)
	}
	if got.UseInternet || hasTool(got.Tools, "internet_search") {
		t.Fatalf("approved RAG correction used internet: %+v", got)
	}

	pending := approvedRAG
	pending.ID = "routecorr_pending"
	pending.ApprovalStatus = RouteCorrectionStatusPending
	got = Classify(Request{
		Content:          "Find that email address in notebook",
		RouteCorrections: []RouteCorrection{pending},
	})
	if got.RouteCorrectionID != "" {
		t.Fatalf("pending route correction was applied: %+v", got)
	}
}

func TestApprovedRouteCorrectionTagsGeneralizeSimilarPrompts(t *testing.T) {
	correction := RouteCorrection{
		ID:                    "routecorr_local_docs",
		Pattern:               "indexed document",
		IntendedRouteCategory: RouteRAGSearch,
		IntendedTaskType:      TaskRAG,
		RequiredTools:         []string{"rag_search"},
		ForbiddenTools:        []string{"internet_search", "internet_fetch"},
		Tags:                  []string{"source:local_documents", "route:rag_search"},
		ApprovalStatus:        RouteCorrectionStatusApproved,
	}
	got := Classify(Request{
		Content:          "Search @gmail.com in ingested documents",
		RouteCorrections: []RouteCorrection{correction},
	})
	if got.RouteCategory != RouteRAGSearch || got.TaskType != TaskRAG {
		t.Fatalf("decision=%+v, want RAG route from correction tags", got)
	}
	if got.RouteCorrectionID != correction.ID {
		t.Fatalf("RouteCorrectionID = %q, want %q; decision=%+v", got.RouteCorrectionID, correction.ID, got)
	}
	if got.UseInternet || hasTool(got.Tools, "internet_search") || hasTool(got.Tools, "internet_fetch") {
		t.Fatalf("local-doc route correction used internet: %+v", got)
	}
}

func TestApprovedRouteCorrectionRejectsUnsafeRequiredTools(t *testing.T) {
	correction := RouteCorrection{
		ID:                    "routecorr_shell",
		Pattern:               "status summary",
		IntendedRouteCategory: RouteWorkspaceRead,
		IntendedTaskType:      TaskTool,
		RequiredTools:         []string{"run_shell"},
		ApprovalStatus:        RouteCorrectionStatusApproved,
	}
	got := Classify(Request{
		Content:          "status summary",
		RouteCorrections: []RouteCorrection{correction},
	})
	if got.RouteCorrectionID != "" {
		t.Fatalf("unsafe route correction was applied: %+v", got)
	}
	if hasTool(got.Tools, "run_shell") {
		t.Fatalf("unsafe route correction exposed run_shell: %+v", got)
	}
}

func TestApprovedRouteCorrectionCannotOverrideProtectedRoutes(t *testing.T) {
	correction := RouteCorrection{
		ID:                    "routecorr_too_broad",
		Pattern:               "example.com",
		IntendedRouteCategory: RouteChatExplanation,
		IntendedTaskType:      TaskReasoning,
		ApprovalStatus:        RouteCorrectionStatusApproved,
	}
	cases := []struct {
		name      string
		prompt    string
		wantRoute string
	}{
		{
			name:      "file write remains approval gated",
			prompt:    "Edit README.md to add a route note",
			wantRoute: RouteFileWrite,
		},
		{
			name:      "scheduler remains approval gated",
			prompt:    "Create a scheduled job that runs every hour",
			wantRoute: RouteSchedulerCreate,
		},
		{
			name:      "active cyber assessment remains scoped",
			prompt:    "check my website example.com and continue until you find a vulnerability",
			wantRoute: RouteActiveAssessmentRequiresScope,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.prompt, RouteCorrections: []RouteCorrection{correction}})
			if got.RouteCategory != tc.wantRoute {
				t.Fatalf("RouteCategory = %q, want %q; decision=%+v", got.RouteCategory, tc.wantRoute, got)
			}
			if got.RouteCorrectionID != "" {
				t.Fatalf("protected route used correction: %+v", got)
			}
		})
	}
}

func TestApprovedRouteCorrectionValidatesRequiredSlots(t *testing.T) {
	correction := RouteCorrection{
		ID:                    "routecorr_fetch",
		Pattern:               "fetch current source",
		IntendedRouteCategory: RouteInternetFetch,
		IntendedTaskType:      TaskTool,
		ApprovalStatus:        RouteCorrectionStatusApproved,
	}
	got := Classify(Request{Content: "fetch current source", RouteCorrections: []RouteCorrection{correction}})
	if !got.NeedsClarification || got.RouteCategory != RouteClarify {
		t.Fatalf("decision=%+v, want clarification when corrected fetch lacks URL", got)
	}
	if len(got.Tools) != 0 || got.UseInternet {
		t.Fatalf("clarifying corrected fetch should not keep tools/internet: %+v", got)
	}
}

func hasTool(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
