package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yemaka/internal/memory"
	"yemaka/internal/routing"
)

func TestDeterministicAgentRouterFoundationContract(t *testing.T) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	completedWebTask := routing.SessionContract{
		ActiveGoal:       "Search AI trends",
		ActiveRoute:      routing.RouteInternetSearch,
		ActiveDomain:     routing.DomainGeneral,
		ActiveTarget:     "AI trends",
		ActiveCapability: "internet",
		ActiveLane:       routing.ToolLaneWebSearch,
		TaskStatus:       routing.TaskStatusActive,
		LastOutcome:      routing.LastOutcomeCompleted,
		UpdatedAt:        now,
	}
	incompleteWebTask := completedWebTask
	incompleteWebTask.LastOutcome = ""
	pendingEdit := routing.SessionContract{
		ActiveRoute:            routing.RouteFileWrite,
		ActiveCapability:       "filesystem_write",
		ActiveLane:             routing.ToolLaneFileEdit,
		ActiveTarget:           "test.md",
		TaskStatus:             routing.TaskStatusAwaitingApproval,
		PendingApproval:        "edit_file",
		PendingOperationID:     "perm_test",
		PendingOperationType:   "edit_file",
		PendingOperationTarget: "test.md",
		PendingOperationStatus: "awaiting_approval",
		RouteLockStrength:      "pending_operation",
		UpdatedAt:              now,
	}

	t.Run("routes_codex_thread_prompts_without_prompt_stealing", func(t *testing.T) {
		cases := []struct {
			name                string
			content             string
			contract            routing.SessionContract
			wantRoute           string
			wantMode            string
			wantTool            string
			forbiddenTool       string
			wantClarification   bool
			wantRewrite         bool
			wantRouteTeaching   bool
			wantCandidateSource string
			wantTargetKind      string
			wantRequiresApprove bool
		}{
			{
				name:                "you should not editorial feedback stays rewrite",
				content:             "you shouldn't phrase it like that, make it better",
				wantRoute:           routing.RouteChatExplanation,
				forbiddenTool:       "rag_search",
				wantRewrite:         true,
				wantCandidateSource: "rewrite_detector",
			},
			{
				name:                "you should editorial feedback stays rewrite",
				content:             "you should make this paragraph clearer",
				wantRoute:           routing.RouteChatExplanation,
				forbiddenTool:       "rag_search",
				wantRewrite:         true,
				wantCandidateSource: "rewrite_detector",
			},
			{
				name:                "make better does not ask source selection",
				content:             "make this better: hello dear sir",
				wantRoute:           routing.RouteChatExplanation,
				forbiddenTool:       "internet_search",
				wantRewrite:         true,
				wantCandidateSource: "rewrite_detector",
			},
			{
				name:                "explicit reusable source teaching becomes learning route",
				content:             "next time when I ask this, use local documents instead of web",
				wantRoute:           routing.RouteLearningAction,
				wantRouteTeaching:   true,
				wantCandidateSource: "route_correction_detector",
			},
			{
				name:      "continue resumes incomplete route from state",
				content:   "continue",
				contract:  incompleteWebTask,
				wantRoute: routing.RouteInternetSearch,
				wantMode:  routing.ContinuationModeContinueSameTask,
				wantTool:  "internet_search",
			},
			{
				name:              "continue after completed task asks one clarification",
				content:           "continue",
				contract:          completedWebTask,
				wantRoute:         routing.RouteClarify,
				wantMode:          routing.ContinuationModeUnclear,
				wantClarification: true,
			},
			{
				name:          "social after completed task does not continue tools",
				content:       "how are?",
				contract:      completedWebTask,
				wantRoute:     routing.RouteChatExplanation,
				wantMode:      routing.ContinuationModeSocialChat,
				forbiddenTool: "internet_search",
			},
			{
				name:          "new standalone question after completed task starts clean answer",
				content:       "good now what is the color of USA flag",
				contract:      completedWebTask,
				wantRoute:     routing.RouteChatExplanation,
				wantMode:      routing.ContinuationModeNewTask,
				forbiddenTool: "internet_search",
			},
			{
				name:                "save last response routes to artifact action",
				content:             "save your last response as test.md",
				contract:            routing.SessionContract{LastFinalMessageID: "msg_final", UpdatedAt: now},
				wantRoute:           routing.RouteFileWrite,
				wantMode:            routing.ContinuationModeLastResponseArtifact,
				wantTool:            "edit_file",
				wantTargetKind:      "file",
				wantRequiresApprove: true,
			},
			{
				name:              "apply without pending operation does not invent work",
				content:           "apply it",
				wantRoute:         routing.RouteClarify,
				wantMode:          routing.ContinuationModeUnclear,
				forbiddenTool:     "edit_file",
				wantClarification: true,
			},
			{
				name:                "apply with pending operation resumes exact edit lane",
				content:             "apply it",
				contract:            pendingEdit,
				wantRoute:           routing.RouteFileWrite,
				wantMode:            routing.ContinuationModeApprovePendingAction,
				wantTool:            "edit_file",
				wantRequiresApprove: true,
				wantCandidateSource: "pending_operation",
			},
			{
				name:          "yes without pending approval stays ordinary chat",
				content:       "yes",
				wantRoute:     routing.RouteChatExplanation,
				forbiddenTool: "edit_file",
			},
			{
				name:              "ambiguous follow-up after completed task clarifies",
				content:           "what about the model?",
				contract:          completedWebTask,
				wantRoute:         routing.RouteClarify,
				wantMode:          routing.ContinuationModeUnclear,
				wantClarification: true,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				decision := routing.Classify(routing.Request{Content: tc.content, SessionContract: tc.contract})
				if decision.RouteCategory != tc.wantRoute {
					t.Fatalf("RouteCategory = %q, want %q; decision=%+v", decision.RouteCategory, tc.wantRoute, decision)
				}
				if tc.wantMode != "" && decision.ContinuationMode != tc.wantMode {
					t.Fatalf("ContinuationMode = %q, want %q; decision=%+v", decision.ContinuationMode, tc.wantMode, decision)
				}
				if decision.NeedsClarification != tc.wantClarification {
					t.Fatalf("NeedsClarification = %t, want %t; decision=%+v", decision.NeedsClarification, tc.wantClarification, decision)
				}
				if tc.wantTool != "" && !foundationHasString(decision.Tools, tc.wantTool) {
					t.Fatalf("Tools = %v, want %q; decision=%+v", decision.Tools, tc.wantTool, decision)
				}
				if tc.forbiddenTool != "" && foundationHasString(decision.Tools, tc.forbiddenTool) {
					t.Fatalf("Tools = %v, did not expect %q; decision=%+v", decision.Tools, tc.forbiddenTool, decision)
				}
				if decision.MessageFrame.IsRewriteLike != tc.wantRewrite {
					t.Fatalf("IsRewriteLike = %t, want %t; frame=%+v", decision.MessageFrame.IsRewriteLike, tc.wantRewrite, decision.MessageFrame)
				}
				if decision.MessageFrame.IsRouteTeachingLike != tc.wantRouteTeaching {
					t.Fatalf("IsRouteTeachingLike = %t, want %t; frame=%+v", decision.MessageFrame.IsRouteTeachingLike, tc.wantRouteTeaching, decision.MessageFrame)
				}
				if tc.wantCandidateSource != "" && !foundationHasCandidateSource(decision.RouteCandidates, tc.wantCandidateSource) {
					t.Fatalf("RouteCandidates = %+v, want source %q", decision.RouteCandidates, tc.wantCandidateSource)
				}
				if tc.wantTargetKind != "" && decision.MessageFrame.TargetKind != tc.wantTargetKind {
					t.Fatalf("TargetKind = %q, want %q; frame=%+v", decision.MessageFrame.TargetKind, tc.wantTargetKind, decision.MessageFrame)
				}
				if decision.RequiresApproval != tc.wantRequiresApprove {
					t.Fatalf("RequiresApproval = %t, want %t; decision=%+v", decision.RequiresApproval, tc.wantRequiresApprove, decision)
				}
				if tc.wantRewrite && strings.Contains(strings.ToLower(decision.ClarificationQuestion), "what source should") {
					t.Fatalf("rewrite route asked generic source clarification: %q", decision.ClarificationQuestion)
				}
			})
		}
	})

	t.Run("last_final_response_beats_operational_messages", func(t *testing.T) {
		ctx, store, conversationID := foundationMemoryStore(t, "last final")
		final, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversationID, Role: "assistant", Content: "Exact final answer", Model: "local-model"})
		if err != nil {
			t.Fatalf("SaveMessage(final) error = %v", err)
		}
		if _, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversationID, Role: "assistant", Content: "Approval recorded. File edits still need the diff and snapshot flow before I apply anything.", Model: "yemaka-executor"}); err != nil {
			t.Fatalf("SaveMessage(operational) error = %v", err)
		}
		foundationSaveRouteState(t, ctx, store, conversationID, routing.SessionContract{LastFinalMessageID: final.ID, UpdatedAt: now})
		content, ok, err := (&Service{Memory: store}).previousAssistantContentForEdit(ctx, conversationID, "")
		if err != nil {
			t.Fatalf("previousAssistantContentForEdit() error = %v", err)
		}
		if !ok || content != "Exact final answer" {
			t.Fatalf("content=%q ok=%t, want exact LastFinalMessageID answer", content, ok)
		}
	})

	t.Run("pending_operation_requires_matching_stored_permission_request", func(t *testing.T) {
		ctx, store, conversationID := foundationMemoryStore(t, "pending operation")
		foundationSaveRouteState(t, ctx, store, conversationID, pendingEdit)
		service := &Service{Memory: store}
		input := PlanInput{Content: "apply it"}
		if err := service.attachRoutingSessionContract(ctx, conversationID, &input); err != nil {
			t.Fatalf("attachRoutingSessionContract() without stored permission error = %v", err)
		}
		if input.SessionContract.PendingOperationID != "" {
			t.Fatalf("PendingOperationID = %q, want cleared without trusted stored permission", input.SessionContract.PendingOperationID)
		}

		foundationSaveRouteState(t, ctx, store, conversationID, pendingEdit)
		if _, err := store.SaveToolRun(ctx, memory.ToolRun{
			ConversationID: conversationID,
			ToolName:       "permission_request",
			Input:          map[string]any{"request_id": "perm_test", "tool_name": "edit_file"},
			Output: PermissionRequest{
				RequestID:            "perm_test",
				ToolName:             "edit_file",
				Command:              []string{"edit_file", "test.md"},
				RiskLevel:            RiskMedium,
				RequiresConfirmation: true,
				DiffPreview:          true,
				SnapshotBeforeWrite:  true,
				RollbackSupported:    true,
			},
			Status:    ExecutionNeedsConfirmation,
			RiskLevel: RiskMedium,
		}); err != nil {
			t.Fatalf("SaveToolRun(permission_request) error = %v", err)
		}
		input = PlanInput{Content: "apply it"}
		if err := service.attachRoutingSessionContract(ctx, conversationID, &input); err != nil {
			t.Fatalf("attachRoutingSessionContract() with stored permission error = %v", err)
		}
		if input.SessionContract.PendingOperationID != "perm_test" {
			t.Fatalf("SessionContract = %+v, want pending operation kept from trusted permission request", input.SessionContract)
		}
	})

	t.Run("tool_runs_rebuild_local_action_activity_without_becoming_final_answer", func(t *testing.T) {
		ctx, store, conversationID := foundationMemoryStore(t, "activity evidence")
		user, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversationID, Role: "user", Content: "save your last response as test.md"})
		if err != nil {
			t.Fatalf("SaveMessage(user) error = %v", err)
		}
		assistant, err := store.SaveMessage(ctx, memory.Message{ConversationID: conversationID, Role: "assistant", Content: "I prepared a file change and need approval before writing it.", Model: "yemaka-executor", ParentID: user.ID})
		if err != nil {
			t.Fatalf("SaveMessage(assistant) error = %v", err)
		}
		if _, err := store.SaveToolRun(ctx, memory.ToolRun{
			ConversationID:     conversationID,
			UserMessageID:      user.ID,
			AssistantMessageID: assistant.ID,
			ToolName:           "agent_executor",
			Input:              map[string]any{"goal": "Prepare file change"},
			Output:             map[string]any{"tool_name": "edit_file"},
			Status:             ExecutionReady,
			RiskLevel:          RiskMedium,
		}); err != nil {
			t.Fatalf("SaveToolRun(agent_executor) error = %v", err)
		}
		if _, err := store.SaveToolRun(ctx, memory.ToolRun{
			ConversationID:     conversationID,
			UserMessageID:      user.ID,
			AssistantMessageID: assistant.ID,
			ToolName:           "agent_verifier",
			Status:             "pass",
			RiskLevel:          RiskLow,
		}); err != nil {
			t.Fatalf("SaveToolRun(agent_verifier) error = %v", err)
		}
		activities, err := ConversationMessageActivities(ctx, store, nil, conversationID, []memory.Message{assistant})
		if err != nil {
			t.Fatalf("ConversationMessageActivities() error = %v", err)
		}
		trace := strings.Join(activities[assistant.ID].Trace, "\n")
		for _, want := range []string{"executor: ready edit_file", "verification: pass"} {
			if !strings.Contains(trace, want) {
				t.Fatalf("activity trace missing %q:\n%s", want, trace)
			}
		}
	})

	t.Run("truth_gate_blocks_unsupported_success_but_allows_trusted_write", func(t *testing.T) {
		plan := BuildPlan(PlanInput{Content: "save your last response as test.md"})
		unsupported := GroundUnsupportedActionClaim("I saved the file as test.md.", plan, ExecutionDecision{}, nil)
		if strings.Contains(strings.ToLower(unsupported), "i saved") || !strings.Contains(strings.ToLower(unsupported), "unverified") {
			t.Fatalf("unsupported success claim was not downgraded:\n%s", unsupported)
		}
		trustedDecision := ExecutionDecision{ToolName: "write_file", Status: ExecutionReady}
		trustedResult := &ExecutionResult{Status: "completed", SourceKind: "workspace", Sources: []string{"test.md"}}
		trusted := GroundUnsupportedActionClaim("I saved the file as test.md.", plan, trustedDecision, trustedResult)
		if trusted != "I saved the file as test.md." {
			t.Fatalf("trusted success claim was not preserved: %q", trusted)
		}
		if verification := VerifyResponseWithEvidence(plan, trusted, trustedDecision, trustedResult); verification.Status != "pass" {
			t.Fatalf("VerifyResponseWithEvidence() = %q, want pass; result=%+v", verification.Status, verification)
		}
	})
}

func foundationMemoryStore(t *testing.T, title string) (context.Context, *memory.Store, string) {
	t.Helper()
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	conversation, err := store.CreateConversation(ctx, title)
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	return ctx, store, conversation.ID
}

func foundationSaveRouteState(t *testing.T, ctx context.Context, store *memory.Store, conversationID string, contract routing.SessionContract) {
	t.Helper()
	if strings.TrimSpace(contract.UpdatedAt) == "" {
		contract.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("Marshal(SessionContract) error = %v", err)
	}
	if _, err := store.SaveConversationRouteState(ctx, memory.ConversationRouteState{ConversationID: conversationID, StateJSON: string(data)}); err != nil {
		t.Fatalf("SaveConversationRouteState() error = %v", err)
	}
}

func foundationHasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func foundationHasCandidateSource(candidates []routing.RouteCandidate, want string) bool {
	for _, candidate := range candidates {
		if candidate.Source == want {
			return true
		}
	}
	return false
}
