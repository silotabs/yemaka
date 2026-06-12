package routing

import (
	"strings"
	"testing"
)

func TestPreflightCardClassifiesSourceOfTruthByRouteClass(t *testing.T) {
	cases := []struct {
		name          string
		prompt        string
		wantSource    string
		wantRoute     string
		wantFreshness string
	}{
		{
			name:          "stable explanation",
			prompt:        "Explain what a CEO does.",
			wantSource:    PreflightSourceChat,
			wantRoute:     RouteChatExplanation,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "supplied svg explanation",
			prompt:        `Make better: <svg xmlns="http://www.w3.org/2000/svg"></svg>`,
			wantSource:    PreflightSourceChat,
			wantRoute:     RouteChatExplanation,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "local document retrieval",
			prompt:        "Find the owner email in the indexed documents.",
			wantSource:    PreflightSourceLocalDocuments,
			wantRoute:     RouteRAGSearch,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "memory retrieval",
			prompt:        "What did I say about the CEO yesterday?",
			wantSource:    PreflightSourceMemory,
			wantRoute:     RouteMemorySearch,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "workspace retrieval",
			prompt:        "Where is CEO extraction implemented in this repo?",
			wantSource:    PreflightSourceWorkspace,
			wantRoute:     RouteWorkspaceRead,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "public role factual lookup",
			prompt:        "Who is the CEO of Phoenix Group UAE?",
			wantSource:    PreflightSourceInternet,
			wantRoute:     RouteInternetSearch,
			wantFreshness: PreflightFreshnessHigh,
		},
		{
			name:          "current news lookup",
			prompt:        "Latest updates on X/Twitter AI",
			wantSource:    PreflightSourceInternet,
			wantRoute:     RouteInternetSearch,
			wantFreshness: PreflightFreshnessHigh,
		},
		{
			name:          "scheduler proposal",
			prompt:        "Create a scheduled job that checks my notes every morning.",
			wantSource:    PreflightSourceScheduler,
			wantRoute:     RouteSchedulerCreate,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "extension proposal",
			prompt:        "Build a reusable tool that monitors a web page title.",
			wantSource:    PreflightSourceExtension,
			wantRoute:     RouteExtensionGenerate,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "website monitor notification proposal",
			prompt:        "Create a sub agent that will monitor https://example.com and tell me when it is down.",
			wantSource:    PreflightSourceExtension,
			wantRoute:     RouteExtensionGenerate,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "connector action",
			prompt:        "Send a Slack message that the build passed.",
			wantSource:    PreflightSourceConnector,
			wantRoute:     RouteConnectorAction,
			wantFreshness: PreflightFreshnessNone,
		},
		{
			name:          "cyber scope gate",
			prompt:        "Find vulnerabilities on example.com and keep going until you find one.",
			wantSource:    PreflightSourceClarify,
			wantRoute:     RouteActiveAssessmentRequiresScope,
			wantFreshness: PreflightFreshnessNone,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card := preflightForTest(tc.prompt)
			if card.SourceOfTruth != tc.wantSource {
				t.Fatalf("SourceOfTruth = %q, want %q; card=%+v", card.SourceOfTruth, tc.wantSource, card)
			}
			if card.SelectedRoute != tc.wantRoute {
				t.Fatalf("SelectedRoute = %q, want %q; card=%+v", card.SelectedRoute, tc.wantRoute, card)
			}
			if card.FreshnessRisk != tc.wantFreshness {
				t.Fatalf("FreshnessRisk = %q, want %q; card=%+v", card.FreshnessRisk, tc.wantFreshness, card)
			}
		})
	}
}

func TestPreflightPublicFactRoutingGeneralizesBeyondCurrentKeywords(t *testing.T) {
	cases := []string{
		"Who is the CEO of Phoenix Group UAE?",
		"What is the name of the chairman of Apple?",
		"Who is the current president of France?",
	}
	for _, prompt := range cases {
		t.Run(prompt, func(t *testing.T) {
			decision := Classify(Request{Content: prompt})
			if decision.RouteCategory != RouteInternetSearch || !hasPreflightTool(decision.Tools, "internet_search") {
				t.Fatalf("decision=%+v, want internet_search for public mutable fact", decision)
			}
			if decision.Preflight.FreshnessRisk != PreflightFreshnessHigh {
				t.Fatalf("Preflight freshness = %q, want high; card=%+v", decision.Preflight.FreshnessRisk, decision.Preflight)
			}
		})
	}
}

func TestPreflightPublicFactNearMissesUseOtherSources(t *testing.T) {
	cases := []struct {
		name      string
		prompt    string
		wantRoute string
		notTool   string
	}{
		{
			name:      "role concept is stable chat",
			prompt:    "Explain what a CEO does.",
			wantRoute: RouteChatExplanation,
			notTool:   "internet_search",
		},
		{
			name:      "uploaded bio uses local docs",
			prompt:    "Summarize the CEO bio in this uploaded PDF.",
			wantRoute: RouteRAGSearch,
			notTool:   "internet_search",
		},
		{
			name:      "repo implementation uses workspace",
			prompt:    "Where is CEO extraction implemented in this repo?",
			wantRoute: RouteFileRead,
			notTool:   "internet_search",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := Classify(Request{Content: tc.prompt})
			if decision.RouteCategory != tc.wantRoute {
				t.Fatalf("RouteCategory = %q, want %q; decision=%+v", decision.RouteCategory, tc.wantRoute, decision)
			}
			if hasPreflightTool(decision.Tools, tc.notTool) {
				t.Fatalf("Tools = %v, did not expect %q", decision.Tools, tc.notTool)
			}
		})
	}
}

func TestPreflightLocalDocumentDottedFilenameTargetStaysDocumentTarget(t *testing.T) {
	card := preflightForTest("What is the content of quarterly-notes.txt from indexed files?")
	if card.SelectedRoute != RouteRAGSearch {
		t.Fatalf("SelectedRoute = %q, want %q; card=%+v", card.SelectedRoute, RouteRAGSearch, card)
	}
	if card.SourceOfTruth != PreflightSourceLocalDocuments {
		t.Fatalf("SourceOfTruth = %q, want local documents; card=%+v", card.SourceOfTruth, card)
	}
	if card.Target != "quarterly-notes.txt" {
		t.Fatalf("Target = %q, want bare document filename; card=%+v", card.Target, card)
	}
	if strings.HasPrefix(strings.ToLower(card.Target), "http://") || strings.HasPrefix(strings.ToLower(card.Target), "https://") {
		t.Fatalf("Target = %q, should not be normalized as a URL", card.Target)
	}
}

func TestPreflightDoesNotTreatPastedSVGAsAmbiguousFollowup(t *testing.T) {
	input := Request{
		Content: `Make this SVG cleaner: <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M2 2h20v20H2z"/></svg>`,
		TaskMemory: strings.Join([]string{
			"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
			"user: Tell me about Apple.",
			"assistant: Apple is one possible target.",
			"user: Tell me about Microsoft.",
			"assistant: Microsoft is another possible target.",
		}, "\n"),
	}
	decision := Classify(input)
	if decision.NeedsClarification {
		t.Fatalf("NeedsClarification = true, want pasted SVG handled as supplied text; decision=%+v", decision)
	}
	if decision.RouteCategory != RouteChatExplanation {
		t.Fatalf("RouteCategory = %q, want chat_explanation; decision=%+v", decision.RouteCategory, decision)
	}
	if hasPreflightTool(decision.Tools, "internet_fetch") || hasPreflightTool(decision.Tools, "internet_search") {
		t.Fatalf("Tools = %v, want no internet tools for pasted SVG", decision.Tools)
	}
}

func TestPreflightCarriesPriorPublicTargetWhenUnambiguous(t *testing.T) {
	input := Request{
		Content: "Who is the CEO?",
		TaskMemory: strings.Join([]string{
			"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
			"user: Tell me about Phoenix Group UAE.",
			"assistant: I need current public search before answering.",
		}, "\n"),
	}
	decision := Classify(input)
	if decision.RouteCategory != RouteInternetSearch || !hasPreflightTool(decision.Tools, "internet_search") {
		t.Fatalf("decision=%+v, want internet search for remembered public target", decision)
	}
	if !strings.Contains(decision.Preflight.FollowupTarget, "Phoenix Group UAE") {
		t.Fatalf("FollowupTarget = %q, want Phoenix Group UAE; card=%+v", decision.Preflight.FollowupTarget, decision.Preflight)
	}
	if decision.Preflight.TargetSource != PreflightTargetPriorConversation {
		t.Fatalf("TargetSource = %q, want prior conversation; card=%+v", decision.Preflight.TargetSource, decision.Preflight)
	}
}

func TestPreflightRetriesBlockedInternetSearchWithPriorTarget(t *testing.T) {
	input := Request{
		Content: "try again",
		Continuation: BuildContinuationFrame(ContinuationInput{
			Content: "try again",
			Prior: ContinuationPriorState{
				RouteCategory: RouteInternetSearch,
				ToolName:      "internet_search",
				Target:        "Phoenix Group UAE CEO",
				SourceOfTruth: PreflightSourceInternet,
				FailureStatus: "blocked",
			},
		}),
		TaskMemory: strings.Join([]string{
			"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
			"user: Who is the CEO of Phoenix Group UAE?",
			"assistant: I paused before running `internet_search`.",
		}, "\n"),
	}
	decision := Classify(input)
	if decision.RouteCategory != RouteInternetSearch || !hasPreflightTool(decision.Tools, "internet_search") {
		t.Fatalf("decision=%+v, want retry to preserve internet_search route", decision)
	}
	if !strings.Contains(decision.Preflight.FollowupTarget, "Phoenix Group UAE") {
		t.Fatalf("FollowupTarget = %q, want Phoenix Group UAE; card=%+v", decision.Preflight.FollowupTarget, decision.Preflight)
	}
}

func TestPreflightClarifiesMissingPublicTarget(t *testing.T) {
	decision := Classify(Request{Content: "Who is the CEO?"})
	if !decision.NeedsClarification || decision.RouteCategory != RouteClarify {
		t.Fatalf("decision=%+v, want clarification for missing public entity", decision)
	}
	if !strings.Contains(strings.ToLower(decision.ClarificationQuestion), "company") {
		t.Fatalf("ClarificationQuestion = %q, want public entity question", decision.ClarificationQuestion)
	}
}

func TestPreflightClarifiesAmbiguousPriorTargets(t *testing.T) {
	input := Request{
		Content: "Search more about him",
		TaskMemory: strings.Join([]string{
			"RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):",
			"user: Tell me about Apple.",
			"assistant: Apple is one possible target.",
			"user: Tell me about Microsoft.",
			"assistant: Microsoft is another possible target.",
		}, "\n"),
	}
	decision := Classify(input)
	if !decision.NeedsClarification || decision.RouteCategory != RouteClarify {
		t.Fatalf("decision=%+v, want clarification for ambiguous prior target", decision)
	}
	if hasPreflightTool(decision.Tools, "internet_search") {
		t.Fatalf("Tools = %v, want no internet search before target clarification", decision.Tools)
	}
	if !containsPreflightValue(decision.Preflight.MissingSlots, "ambiguous_prior_target") {
		t.Fatalf("MissingSlots = %v, want ambiguous_prior_target; card=%+v", decision.Preflight.MissingSlots, decision.Preflight)
	}
}

func TestPreflightIncludesApprovedCorrectionIDsWithoutBypassingPolicy(t *testing.T) {
	correction := RouteCorrection{
		ID:                    "routecorr_docs",
		Pattern:               "indexed document",
		IntendedRouteCategory: RouteRAGSearch,
		IntendedTaskType:      TaskRAG,
		RequiredTools:         []string{"rag_search"},
		Tags:                  []string{"source:local_documents"},
		ApprovalStatus:        RouteCorrectionStatusApproved,
	}
	decision := Classify(Request{
		Content:          "Search owner email in indexed documents",
		RouteCorrections: []RouteCorrection{correction},
	})
	if !containsPreflightValue(decision.Preflight.AppliedCorrectionIDs, correction.ID) {
		t.Fatalf("AppliedCorrectionIDs = %v, want %q", decision.Preflight.AppliedCorrectionIDs, correction.ID)
	}

	protected := Classify(Request{
		Content:          "Edit config.yaml to add Phoenix Group UAE CEO",
		RouteCorrections: []RouteCorrection{correction},
	})
	if protected.RouteCorrectionID != "" || containsPreflightValue(protected.Preflight.AppliedCorrectionIDs, correction.ID) {
		t.Fatalf("protected route used correction: %+v", protected)
	}
	if protected.RouteCategory != RouteFileWrite {
		t.Fatalf("RouteCategory = %q, want file_write", protected.RouteCategory)
	}
}

func preflightForTest(prompt string) PreflightCard {
	input := Request{Content: prompt}
	return BuildPreflightCard(input, ExtractTaskFrame(input))
}

func hasPreflightTool(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
