package routing

import (
	"strings"
	"testing"
	"time"
)

func TestResolveContinuationUsesSessionContract(t *testing.T) {
	active := SessionContract{
		ActiveGoal:       "Search current public news",
		ActiveRoute:      RouteInternetSearch,
		ActiveDomain:     DomainGeneral,
		ActiveTarget:     "current public news",
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
	}
	cases := []struct {
		name        string
		content     string
		contract    SessionContract
		frame       ContinuationFrame
		wantMode    string
		wantKeep    bool
		wantClarify bool
	}{
		{
			name:     "current news follow-up stays with task",
			content:  "what about the visit?",
			contract: active,
			wantMode: ContinuationModeAskFollowupPrevious,
			wantKeep: true,
		},
		{
			name:     "do same stays with active task",
			content:  "do the same for this",
			contract: active,
			wantMode: ContinuationModeContinueSameTask,
			wantKeep: true,
		},
		{
			name:     "target correction keeps route",
			content:  "no, I meant the other file",
			contract: active,
			frame:    BuildContinuationFrame(ContinuationInput{Content: "no, I meant the other file"}),
			wantMode: ContinuationModeRevisePreviousTask,
			wantKeep: true,
		},
		{
			name:     "source correction can change route",
			content:  "no, I meant local documents",
			contract: active,
			frame:    BuildContinuationFrame(ContinuationInput{Content: "no, I meant local documents"}),
			wantMode: ContinuationModeRevisePreviousTask,
			wantKeep: false,
		},
		{
			name:    "pending approval answer",
			content: "yes",
			contract: SessionContract{
				ActiveRoute:     RouteFileWrite,
				ActiveLane:      ToolLaneFileEdit,
				TaskStatus:      TaskStatusAwaitingApproval,
				PendingApproval: "edit_file",
			},
			wantMode: ContinuationModeApprovePendingAction,
			wantKeep: true,
		},
		{
			name:     "approval wording inside full prompt is not approval reply",
			content:  "Propose the extension but do not generate it until I approve.",
			contract: SessionContract{},
			wantMode: ContinuationModeNewTask,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveContinuation(tc.content, tc.contract, tc.frame)
			if got.Mode != tc.wantMode {
				t.Fatalf("Mode = %q, want %q; got=%+v", got.Mode, tc.wantMode, got)
			}
			if got.KeepActiveRoute != tc.wantKeep {
				t.Fatalf("KeepActiveRoute = %t, want %t; got=%+v", got.KeepActiveRoute, tc.wantKeep, got)
			}
			if got.NeedsClarification != tc.wantClarify {
				t.Fatalf("NeedsClarification = %t, want %t; got=%+v", got.NeedsClarification, tc.wantClarify, got)
			}
		})
	}
}

func TestClassifyUsesSessionRouteCommitmentForFollowups(t *testing.T) {
	contract := SessionContract{
		ActiveGoal:       "Search current public news",
		ActiveRoute:      RouteInternetSearch,
		ActiveDomain:     DomainGeneral,
		ActiveTarget:     "current public news",
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
	}
	got := Classify(Request{Content: "what about the visit?", SessionContract: contract})
	if got.RouteCategory != RouteInternetSearch || !hasControlPlaneTool(got.Tools, "internet_search") {
		t.Fatalf("route/tools = %q/%v, want internet_search", got.RouteCategory, got.Tools)
	}
	if got.ContinuationMode != ContinuationModeAskFollowupPrevious {
		t.Fatalf("ContinuationMode = %q, want ask_followup_about_previous_result", got.ContinuationMode)
	}
	if got.ToolLane != ToolLaneWebSearch {
		t.Fatalf("ToolLane = %q, want web_search_lane", got.ToolLane)
	}
}

func TestResolveContinuationDoesNotKeepCompletedRouteForStandalonePrompts(t *testing.T) {
	contract := SessionContract{
		ActiveGoal:       "Search AI trends",
		ActiveRoute:      RouteInternetSearch,
		ActiveDomain:     DomainGeneral,
		ActiveTarget:     "AI trends",
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
		LastOutcome:      LastOutcomeCompleted,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	cases := []struct {
		content  string
		wantMode string
	}{
		{content: "hello", wantMode: ContinuationModeSocialChat},
		{content: "what is USA flag color", wantMode: ContinuationModeNewTask},
		{content: "2+2", wantMode: ContinuationModeNewTask},
		{content: "explain gravity", wantMode: ContinuationModeNewTask},
		{content: "create file inside /users/example/documents and name it trends.md and insert the last response inside it", wantMode: ContinuationModeLastResponseArtifact},
	}
	for _, tc := range cases {
		t.Run(tc.content, func(t *testing.T) {
			got := ResolveContinuation(tc.content, contract, ContinuationFrame{})
			if got.Mode != tc.wantMode {
				t.Fatalf("Mode = %q, want %q; resolution=%+v", got.Mode, tc.wantMode, got)
			}
			if got.KeepActiveRoute {
				t.Fatalf("KeepActiveRoute = true, want false; resolution=%+v", got)
			}
		})
	}
}

func TestResolveContinuationKeepsPendingApprovalOnlyWhenPending(t *testing.T) {
	pending := SessionContract{
		ActiveRoute:     RouteFileWrite,
		ActiveLane:      ToolLaneFileEdit,
		TaskStatus:      TaskStatusAwaitingApproval,
		PendingApproval: "edit_file",
		UpdatedAt:       time.Now().UTC().Format(time.RFC3339Nano),
	}
	got := ResolveContinuation("apply it", pending, ContinuationFrame{})
	if got.Mode != ContinuationModeApprovePendingAction || !got.KeepActiveRoute {
		t.Fatalf("pending approval resolution = %+v, want approval keep route", got)
	}
	completed := pending
	completed.TaskStatus = TaskStatusActive
	completed.PendingApproval = ""
	completed.LastOutcome = LastOutcomeCompleted
	got = ResolveContinuation("yes", completed, ContinuationFrame{})
	if got.KeepActiveRoute || got.Mode == ContinuationModeApprovePendingAction {
		t.Fatalf("completed approval-like reply = %+v, want no approval continuation", got)
	}
}

func TestClassifyCompletedRouteDoesNotHijackNewStandaloneRoute(t *testing.T) {
	contract := SessionContract{
		ActiveGoal:       "Search AI trends",
		ActiveRoute:      RouteInternetSearch,
		ActiveDomain:     DomainGeneral,
		ActiveTarget:     "AI trends",
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
		LastOutcome:      LastOutcomeCompleted,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	got := Classify(Request{Content: "create file inside /users/example/documents and name it trends.md and insert the last response inside it", SessionContract: contract})
	if got.RouteCategory != RouteFileWrite {
		t.Fatalf("RouteCategory = %q, want file_write; decision=%+v", got.RouteCategory, got)
	}
	if got.ContinuationMode != ContinuationModeLastResponseArtifact {
		t.Fatalf("ContinuationMode = %q, want last-response artifact", got.ContinuationMode)
	}
	if hasControlPlaneTool(got.Tools, "internet_search") {
		t.Fatalf("Tools = %v, should not keep prior internet route", got.Tools)
	}
}

func TestRoutingKernelArbitrationMatrix(t *testing.T) {
	completed := SessionContract{
		ActiveGoal:       "Search AI trends",
		ActiveRoute:      RouteInternetSearch,
		ActiveDomain:     DomainGeneral,
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
		LastOutcome:      LastOutcomeCompleted,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339Nano),
	}
	incomplete := completed
	incomplete.LastOutcome = ""
	incomplete.ActiveTarget = "AI trends"

	cases := []struct {
		name              string
		content           string
		contract          SessionContract
		wantRoute         string
		wantMode          string
		wantClarification bool
		wantTool          string
		notTool           string
		wantFrameRewrite  bool
		wantFrameTeaching bool
		wantFrameSocial   bool
	}{
		{
			name:             "you should not editorial wins rewrite",
			content:          "you shouldn't phrase it like that, make it better",
			wantRoute:        RouteChatExplanation,
			notTool:          "rag_search",
			wantFrameRewrite: true,
		},
		{
			name:             "you should editorial wins rewrite",
			content:          "you should make this paragraph clearer",
			wantRoute:        RouteChatExplanation,
			notTool:          "rag_search",
			wantFrameRewrite: true,
		},
		{
			name:              "explicit route teaching selected",
			content:           "next time when I ask this, use local documents instead of web",
			wantRoute:         RouteLearningAction,
			wantFrameTeaching: true,
		},
		{
			name:              "explicit workspace source preference selected",
			content:           "when I ask about this project, use workspace files instead of memory",
			wantRoute:         RouteLearningAction,
			wantFrameTeaching: true,
		},
		{
			name:      "continue resumes incomplete turn",
			content:   "continue",
			contract:  incomplete,
			wantRoute: RouteInternetSearch,
			wantMode:  ContinuationModeContinueSameTask,
			wantTool:  "internet_search",
		},
		{
			name:              "continue after completed asks clarification",
			content:           "continue",
			contract:          completed,
			wantRoute:         RouteClarify,
			wantMode:          ContinuationModeUnclear,
			wantClarification: true,
		},
		{
			name:            "how are after completed is social",
			content:         "how are?",
			contract:        completed,
			wantRoute:       RouteChatExplanation,
			wantMode:        ContinuationModeSocialChat,
			notTool:         "internet_search",
			wantFrameSocial: true,
		},
		{
			name:      "standalone question after completed starts new answer",
			content:   "good now what is the color of USA flag",
			contract:  completed,
			wantRoute: RouteChatExplanation,
			wantMode:  ContinuationModeNewTask,
			notTool:   "internet_search",
		},
		{
			name:              "apply without pending operation clarifies",
			content:           "apply it",
			wantRoute:         RouteClarify,
			wantMode:          ContinuationModeUnclear,
			wantClarification: true,
		},
		{
			name:      "yes without pending approval is ordinary chat",
			content:   "yes",
			wantRoute: RouteChatExplanation,
			notTool:   "edit_file",
		},
		{
			name:              "ambiguous completed follow-up clarifies",
			content:           "what about the model?",
			contract:          completed,
			wantRoute:         RouteClarify,
			wantMode:          ContinuationModeUnclear,
			wantClarification: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.content, SessionContract: tc.contract})
			if got.RouteCategory != tc.wantRoute {
				t.Fatalf("RouteCategory = %q, want %q; decision=%+v", got.RouteCategory, tc.wantRoute, got)
			}
			if tc.wantMode != "" && got.ContinuationMode != tc.wantMode {
				t.Fatalf("ContinuationMode = %q, want %q; decision=%+v", got.ContinuationMode, tc.wantMode, got)
			}
			if got.NeedsClarification != tc.wantClarification {
				t.Fatalf("NeedsClarification = %t, want %t; decision=%+v", got.NeedsClarification, tc.wantClarification, got)
			}
			if tc.wantTool != "" && !hasControlPlaneTool(got.Tools, tc.wantTool) {
				t.Fatalf("Tools = %v, want %q; decision=%+v", got.Tools, tc.wantTool, got)
			}
			if tc.notTool != "" && hasControlPlaneTool(got.Tools, tc.notTool) {
				t.Fatalf("Tools = %v, did not expect %q; decision=%+v", got.Tools, tc.notTool, got)
			}
			if got.MessageFrame.IsRewriteLike != tc.wantFrameRewrite {
				t.Fatalf("MessageFrame.IsRewriteLike = %t, want %t; frame=%+v", got.MessageFrame.IsRewriteLike, tc.wantFrameRewrite, got.MessageFrame)
			}
			if got.MessageFrame.IsRouteTeachingLike != tc.wantFrameTeaching {
				t.Fatalf("MessageFrame.IsRouteTeachingLike = %t, want %t; frame=%+v", got.MessageFrame.IsRouteTeachingLike, tc.wantFrameTeaching, got.MessageFrame)
			}
			if got.MessageFrame.IsSocialTurn != tc.wantFrameSocial {
				t.Fatalf("MessageFrame.IsSocialTurn = %t, want %t; frame=%+v", got.MessageFrame.IsSocialTurn, tc.wantFrameSocial, got.MessageFrame)
			}
			if tc.wantFrameTeaching && !firstRouteCandidateSource(got.RouteCandidates, "route_correction_detector") {
				t.Fatalf("RouteCandidates = %+v, want route_correction_detector candidate", got.RouteCandidates)
			}
		})
	}
}

func TestClassifyResumesOnlyStoredPendingOperationShape(t *testing.T) {
	contract := SessionContract{
		ActiveRoute:            RouteFileWrite,
		ActiveCapability:       "filesystem_write",
		ActiveLane:             ToolLaneFileEdit,
		ActiveTarget:           "notes.md",
		TaskStatus:             TaskStatusAwaitingApproval,
		PendingApproval:        "edit_file",
		PendingOperationID:     "perm_edit",
		PendingOperationType:   "edit_file",
		PendingOperationTarget: "notes.md",
		PendingOperationStatus: "awaiting_approval",
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339Nano),
	}
	got := Classify(Request{Content: "apply it", SessionContract: contract})
	if got.RouteCategory != RouteFileWrite || got.ContinuationMode != ContinuationModeApprovePendingAction {
		t.Fatalf("decision=%+v, want pending file_write resume", got)
	}
	if !hasControlPlaneTool(got.Tools, "edit_file") {
		t.Fatalf("Tools = %v, want edit_file", got.Tools)
	}
	if len(got.Files) == 0 || got.Files[0] != "notes.md" {
		t.Fatalf("Files = %v, want pending target notes.md", got.Files)
	}
	if !firstRouteCandidateSource(got.RouteCandidates, "pending_operation") {
		t.Fatalf("RouteCandidates = %+v, want pending_operation candidate", got.RouteCandidates)
	}
}

func TestClassifyIgnoresStaleSessionRouteCommitment(t *testing.T) {
	contract := SessionContract{
		ActiveGoal:       "Search current public news",
		ActiveRoute:      RouteInternetSearch,
		ActiveDomain:     DomainGeneral,
		ActiveTarget:     "current public news",
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
		LastOutcome:      LastOutcomeCompleted,
		UpdatedAt:        time.Now().Add(-SessionContractActiveTTL - time.Minute).UTC().Format(time.RFC3339Nano),
	}
	got := Classify(Request{Content: "what about the visit?", SessionContract: contract})
	if got.RouteCategory == RouteInternetSearch || hasControlPlaneTool(got.Tools, "internet_search") {
		t.Fatalf("route/tools = %q/%v, stale session should not force web search", got.RouteCategory, got.Tools)
	}
	if got.ContinuationMode == ContinuationModeAskFollowupPrevious || got.ContinuationMode == ContinuationModeContinueSameTask {
		t.Fatalf("ContinuationMode = %q, stale session should not continue previous route", got.ContinuationMode)
	}
}

func firstRouteCandidateSource(candidates []RouteCandidate, source string) bool {
	for _, candidate := range candidates {
		if candidate.Source == source {
			return true
		}
	}
	return false
}

func TestSanitizeSessionContractKeepsFreshAndExpiresStaleStates(t *testing.T) {
	now := time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC)
	fresh := SessionContract{
		ActiveRoute: RouteRAGSearch,
		TaskStatus:  TaskStatusActive,
		LastOutcome: LastOutcomeCompleted,
		UpdatedAt:   now.Add(-10 * time.Minute).Format(time.RFC3339Nano),
	}
	if got := SanitizeSessionContract(fresh, now); got.IsZero() {
		t.Fatalf("fresh contract sanitized to zero: %+v", got)
	}
	staleCompleted := fresh
	staleCompleted.UpdatedAt = now.Add(-SessionContractActiveTTL - time.Minute).Format(time.RFC3339Nano)
	if got := SanitizeSessionContract(staleCompleted, now); !got.IsZero() {
		t.Fatalf("stale completed contract = %+v, want zero", got)
	}
	staleBlocked := fresh
	staleBlocked.TaskStatus = TaskStatusBlocked
	staleBlocked.LastOutcome = LastOutcomeBlocked
	staleBlocked.UpdatedAt = now.Add(-SessionContractBlockedTTL - time.Minute).Format(time.RFC3339Nano)
	if got := SanitizeSessionContract(staleBlocked, now); !got.IsZero() {
		t.Fatalf("stale blocked contract = %+v, want zero", got)
	}
	pendingApproval := fresh
	pendingApproval.TaskStatus = TaskStatusAwaitingApproval
	pendingApproval.PendingApproval = "edit_file"
	pendingApproval.UpdatedAt = now.Add(-SessionContractPendingApprovalTTL - time.Minute).Format(time.RFC3339Nano)
	if got := SanitizeSessionContract(pendingApproval, now); !got.IsZero() {
		t.Fatalf("stale pending approval contract = %+v, want zero", got)
	}
}

func TestClassifySourceCorrectionCanRepairCommittedRoute(t *testing.T) {
	contract := SessionContract{
		ActiveGoal:       "Search current public facts",
		ActiveRoute:      RouteInternetSearch,
		ActiveTarget:     "public lookup",
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
	}
	got := Classify(Request{Content: "No, I meant local documents", SessionContract: contract})
	if got.RouteCategory != RouteRAGSearch || !hasControlPlaneTool(got.Tools, "rag_search") {
		t.Fatalf("route/tools = %q/%v, want local document RAG repair", got.RouteCategory, got.Tools)
	}
	if hasControlPlaneTool(got.Tools, "internet_search") {
		t.Fatalf("tools = %v, should not keep internet search after local-doc source correction", got.Tools)
	}
}

func TestClassifySourceCorrectionCanSwitchToMemoryOrWorkspace(t *testing.T) {
	contract := SessionContract{
		ActiveGoal:       "Search current public facts",
		ActiveRoute:      RouteInternetSearch,
		ActiveTarget:     "public lookup",
		ActiveCapability: "internet",
		ActiveLane:       ToolLaneWebSearch,
		TaskStatus:       TaskStatusActive,
	}
	cases := []struct {
		name      string
		content   string
		wantRoute string
		wantTool  string
		notTool   string
	}{
		{
			name:      "memory source correction",
			content:   "No, use memory instead",
			wantRoute: RouteMemorySearch,
			wantTool:  "memory_search",
			notTool:   "internet_search",
		},
		{
			name:      "workspace source correction",
			content:   "No, use workspace files instead",
			wantRoute: RouteWorkspaceRead,
			wantTool:  "search_files",
			notTool:   "internet_search",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Classify(Request{Content: tc.content, SessionContract: contract})
			if got.RouteCategory != tc.wantRoute || !hasControlPlaneTool(got.Tools, tc.wantTool) {
				t.Fatalf("route/tools = %q/%v, want %q with %q", got.RouteCategory, got.Tools, tc.wantRoute, tc.wantTool)
			}
			if hasControlPlaneTool(got.Tools, tc.notTool) {
				t.Fatalf("tools = %v, should not include %q", got.Tools, tc.notTool)
			}
		})
	}
}

func TestClassifyComparisonFollowupPreservesActiveRouteAndTarget(t *testing.T) {
	contract := SessionContract{
		ActiveGoal:       "Read local workspace file",
		ActiveRoute:      RouteFileRead,
		ActiveDomain:     DomainCode,
		ActiveTarget:     "docs/alpha.md",
		ActiveCapability: "filesystem_read",
		ActiveLane:       ToolLaneResearch,
		TaskStatus:       TaskStatusActive,
	}
	got := Classify(Request{Content: "Compare it with the previous one", SessionContract: contract})
	if got.RouteCategory != RouteFileRead {
		t.Fatalf("RouteCategory = %q, want file_read; decision=%+v", got.RouteCategory, got)
	}
	if got.Target != "docs/alpha.md" {
		t.Fatalf("Target = %q, want active target", got.Target)
	}
	if got.ContinuationMode != ContinuationModeAskFollowupPrevious {
		t.Fatalf("ContinuationMode = %q, want ask follow-up", got.ContinuationMode)
	}
}

func TestToolLaneBlocksWrongRouteTools(t *testing.T) {
	got := ApplyToolLane([]string{"rag_search", "internet_search", "read_file"}, RouteRAGSearch)
	if !hasControlPlaneTool(got, "rag_search") {
		t.Fatalf("ApplyToolLane = %v, want rag_search", got)
	}
	for _, forbidden := range []string{"internet_search", "read_file"} {
		if hasControlPlaneTool(got, forbidden) {
			t.Fatalf("ApplyToolLane = %v, should block %s in document lane", got, forbidden)
		}
	}
}

func TestArbitrateRouteClarifiesConflictingLowConfidenceCandidates(t *testing.T) {
	decision, ok := ArbitrateRoute(
		Request{Content: "what about that"},
		TaskFrame{Domain: DomainGeneral},
		PreflightCard{Domain: DomainGeneral},
		ContinuationResolution{Mode: ContinuationModeNewTask, Confidence: 80},
		[]RouteCandidate{
			{Route: RouteInternetSearch, Confidence: 64, Reason: "freshness wording"},
			{Route: RouteRAGSearch, Confidence: 60, Reason: "document wording"},
		},
	)
	if !ok {
		t.Fatal("ArbitrateRoute() did not resolve conflict")
	}
	if decision.RouteCategory != RouteClarify || !decision.NeedsClarification {
		t.Fatalf("route/clarify = %q/%t, want clarify", decision.RouteCategory, decision.NeedsClarification)
	}
	if decision.Confidence != 64 {
		t.Fatalf("Confidence = %d, want best candidate confidence", decision.Confidence)
	}
	if !hasControlPlaneTool(decision.AmbiguityFlags, "route_candidate_conflict") {
		t.Fatalf("AmbiguityFlags = %v, want route_candidate_conflict", decision.AmbiguityFlags)
	}
}

func TestArbitrateRouteClarificationQuestionUsesCandidateSource(t *testing.T) {
	decision, ok := ArbitrateRoute(
		Request{
			Content: "what about that",
			SessionContract: SessionContract{
				ActiveRoute: RouteRAGSearch,
				ActiveLane:  ToolLaneDocument,
				TaskStatus:  TaskStatusActive,
			},
		},
		TaskFrame{Domain: DomainGeneral},
		PreflightCard{Domain: DomainGeneral},
		ContinuationResolution{Mode: ContinuationModeNewTask, Confidence: 80},
		[]RouteCandidate{
			{Route: RouteInternetSearch, SourceOfTruth: PreflightSourceInternet, Confidence: 64, Reason: "freshness wording"},
			{Route: RouteRAGSearch, SourceOfTruth: PreflightSourceLocalDocuments, Confidence: 60, Reason: "document wording"},
		},
	)
	if !ok {
		t.Fatal("ArbitrateRoute() did not resolve conflict")
	}
	if decision.RouteCategory != RouteClarify || !decision.NeedsClarification {
		t.Fatalf("route/clarify = %q/%t, want clarify", decision.RouteCategory, decision.NeedsClarification)
	}
	if !containsControlPlaneText(decision.ClarificationQuestion, "local documents") || !containsControlPlaneText(decision.ClarificationQuestion, "web search") {
		t.Fatalf("ClarificationQuestion = %q, want source-specific options", decision.ClarificationQuestion)
	}
}

func TestUpdateSessionContractPersistsPendingState(t *testing.T) {
	decision := Decision{
		TaskType:              TaskChat,
		RouteCategory:         RouteClarify,
		Domain:                DomainGeneral,
		Capability:            "chat",
		Confidence:            72,
		NeedsClarification:    true,
		ClarificationQuestion: "Which document should I use?",
		ContinuationMode:      ContinuationModeUnclear,
	}
	got := UpdateSessionContract(SessionContract{}, decision, SessionUpdate{
		Goal:        "Search local documents",
		LastOutcome: LastOutcomeCompleted,
	})
	if got.TaskStatus != TaskStatusAwaitingClarification {
		t.Fatalf("TaskStatus = %q, want awaiting_clarification; got=%+v", got.TaskStatus, got)
	}
	if got.PendingClarification == "" {
		t.Fatalf("PendingClarification is empty; got=%+v", got)
	}
	if got.ActiveRoute != RouteClarify {
		t.Fatalf("ActiveRoute = %q, want clarify", got.ActiveRoute)
	}
}

func TestUpdateSessionContractClearsApprovalAfterCompletedAction(t *testing.T) {
	previous := SessionContract{
		ActiveRoute:     RouteFileWrite,
		ActiveLane:      ToolLaneFileEdit,
		TaskStatus:      TaskStatusAwaitingApproval,
		PendingApproval: "edit_file",
	}
	decision := Decision{
		TaskType:         TaskTool,
		RouteCategory:    RouteFileWrite,
		Domain:           DomainCode,
		Capability:       "filesystem_write",
		Tools:            []string{"edit_file"},
		RequiresApproval: true,
		ToolLane:         ToolLaneFileEdit,
		ContinuationMode: ContinuationModeApprovePendingAction,
	}
	got := UpdateSessionContract(previous, decision, SessionUpdate{
		Goal:        "Apply approved file edit",
		LastOutcome: LastOutcomeCompleted,
	})
	if got.TaskStatus != TaskStatusActive {
		t.Fatalf("TaskStatus = %q, want active; got=%+v", got.TaskStatus, got)
	}
	if got.PendingApproval != "" {
		t.Fatalf("PendingApproval = %q, want cleared", got.PendingApproval)
	}
}

func hasControlPlaneTool(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsControlPlaneText(value string, want string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(want))
}
