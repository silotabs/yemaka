package agent

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"yemaka/internal/memory"
	"yemaka/internal/routing"
)

func TestAttachRoutingSessionContractDeletesStaleStoredState(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "stale route")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	stale := routing.SessionContract{
		ActiveRoute:      routing.RouteInternetSearch,
		ActiveCapability: "internet",
		ActiveLane:       routing.ToolLaneWebSearch,
		TaskStatus:       routing.TaskStatusActive,
		LastOutcome:      routing.LastOutcomeCompleted,
		UpdatedAt:        time.Now().Add(-routing.SessionContractActiveTTL - time.Minute).UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(stale)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if _, err := store.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversation.ID,
		StateJSON:      string(data),
	}); err != nil {
		t.Fatalf("SaveConversationRouteState() error = %v", err)
	}
	service := &Service{Memory: store}
	input := PlanInput{Content: "what about that?"}
	if err := service.attachRoutingSessionContract(ctx, conversation.ID, &input); err != nil {
		t.Fatalf("attachRoutingSessionContract() error = %v", err)
	}
	if !input.SessionContract.IsZero() {
		t.Fatalf("SessionContract = %+v, want stale state ignored", input.SessionContract)
	}
	if _, ok, err := store.GetConversationRouteState(ctx, conversation.ID); err != nil {
		t.Fatalf("GetConversationRouteState() error = %v", err)
	} else if ok {
		t.Fatal("stale conversation route state still persisted")
	}
}

func TestSaveRoutingSessionContractPersistsRouteRecovery(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "route recovery")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	service := &Service{Memory: store}
	plan := Plan{
		Goal:              "Search current public news",
		RouteCategory:     routing.RouteInternetSearch,
		RouteLane:         routing.ToolLaneWebSearch,
		RouteUsesInternet: true,
		RiskLevel:         RiskLow,
	}
	decision := ExecutionDecision{
		Status:   ExecutionBlocked,
		ToolName: "internet_search",
		Reason:   "provider not configured",
	}
	result := &ExecutionResult{
		Status:     "failed",
		Context:    "SearXNG provider not configured",
		SourceKind: "tool",
	}
	if err := service.saveRoutingSessionContract(ctx, conversation.ID, routing.SessionContract{}, plan, decision, result, VerificationResult{}); err != nil {
		t.Fatalf("saveRoutingSessionContract() error = %v", err)
	}
	stored, ok, err := store.GetConversationRouteState(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("GetConversationRouteState() error = %v", err)
	}
	if !ok {
		t.Fatal("conversation route state missing")
	}
	var contract routing.SessionContract
	if err := json.Unmarshal([]byte(stored.StateJSON), &contract); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if contract.LastRecovery.Trigger != routing.RouteRecoveryTriggerProviderUnavailable {
		t.Fatalf("LastRecovery = %+v, want provider_unavailable", contract.LastRecovery)
	}
	if contract.LastRecovery.FinalAction != routing.RouteRecoveryActionRequestConfiguration {
		t.Fatalf("LastRecovery.FinalAction = %q, want request_configuration", contract.LastRecovery.FinalAction)
	}
}
