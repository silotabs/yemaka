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

func TestSaveRoutingSessionContractPersistsPendingOperation(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "pending operation")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	service := &Service{Memory: store}
	plan := Plan{
		Goal:                  "Edit notes",
		RouteCategory:         routing.RouteFileWrite,
		RouteLane:             routing.ToolLaneFileEdit,
		RouteCapability:       "filesystem_write",
		RouteTarget:           "notes.md",
		RouteWritesFiles:      true,
		RouteRequiresApproval: true,
		FilesNeeded:           []string{"notes.md"},
		RiskLevel:             RiskMedium,
	}
	decision := ExecutionDecision{
		Status:               ExecutionNeedsConfirmation,
		RequestID:            "perm_notes",
		ToolName:             "edit_file",
		Command:              []string{"edit_file", "notes.md"},
		RiskLevel:            RiskMedium,
		RequiresConfirmation: true,
		Reason:               "file edits require confirmation and snapshot flow",
	}
	if err := service.saveRoutingSessionContract(ctx, conversation.ID, routing.SessionContract{}, plan, decision, nil, VerificationResult{}); err != nil {
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
	if contract.PendingOperationID != "perm_notes" ||
		contract.PendingOperationType != "edit_file" ||
		contract.PendingOperationTarget != "notes.md" ||
		contract.PendingOperationStatus != "awaiting_approval" ||
		contract.RouteLockStrength != "pending_operation" {
		t.Fatalf("contract pending operation = %+v, want saved permission operation", contract)
	}
	if contract.TaskStatus != routing.TaskStatusAwaitingApproval {
		t.Fatalf("TaskStatus = %q, want awaiting_approval", contract.TaskStatus)
	}
}

func TestAttachRoutingSessionContractRequiresStoredPendingOperation(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "pending operation validation")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	stalePending := routing.SessionContract{
		ActiveRoute:            routing.RouteFileWrite,
		ActiveCapability:       "filesystem_write",
		ActiveLane:             routing.ToolLaneFileEdit,
		ActiveTarget:           "notes.md",
		TaskStatus:             routing.TaskStatusAwaitingApproval,
		PendingApproval:        "edit_file",
		PendingOperationID:     "perm_missing",
		PendingOperationType:   "edit_file",
		PendingOperationTarget: "notes.md",
		PendingOperationStatus: "awaiting_approval",
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(stalePending)
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
	input := PlanInput{Content: "apply it"}
	if err := service.attachRoutingSessionContract(ctx, conversation.ID, &input); err != nil {
		t.Fatalf("attachRoutingSessionContract() error = %v", err)
	}
	if strings.TrimSpace(input.SessionContract.PendingOperationID) != "" || strings.TrimSpace(input.SessionContract.PendingApproval) != "" {
		t.Fatalf("SessionContract = %+v, want missing pending operation cleared", input.SessionContract)
	}
}

func TestAttachRoutingSessionContractKeepsStoredPendingOperation(t *testing.T) {
	ctx := context.Background()
	store, err := memory.Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("memory.Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "pending operation validation")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	contract := routing.SessionContract{
		ActiveRoute:            routing.RouteFileWrite,
		ActiveCapability:       "filesystem_write",
		ActiveLane:             routing.ToolLaneFileEdit,
		ActiveTarget:           "notes.md",
		TaskStatus:             routing.TaskStatusAwaitingApproval,
		PendingApproval:        "edit_file",
		PendingOperationID:     "perm_notes",
		PendingOperationType:   "edit_file",
		PendingOperationTarget: "notes.md",
		PendingOperationStatus: "awaiting_approval",
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339Nano),
	}
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if _, err := store.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversation.ID,
		StateJSON:      string(data),
	}); err != nil {
		t.Fatalf("SaveConversationRouteState() error = %v", err)
	}
	if _, err := store.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "permission_request",
		Input:          map[string]any{"request_id": "perm_notes", "tool_name": "edit_file"},
		Output: PermissionRequest{
			RequestID:            "perm_notes",
			ToolName:             "edit_file",
			Command:              []string{"edit_file", "notes.md"},
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
	service := &Service{Memory: store}
	input := PlanInput{Content: "apply it"}
	if err := service.attachRoutingSessionContract(ctx, conversation.ID, &input); err != nil {
		t.Fatalf("attachRoutingSessionContract() error = %v", err)
	}
	if input.SessionContract.PendingOperationID != "perm_notes" {
		t.Fatalf("SessionContract = %+v, want pending operation kept", input.SessionContract)
	}
}
