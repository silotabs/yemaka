package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"yemaka/internal/memory"
)

// ValidateStoredPermissionApproval ensures an approval corresponds to a
// permission request that Yemaka actually emitted and has not already consumed.
func ValidateStoredPermissionApproval(ctx context.Context, memories *memory.Store, request PermissionRequest) error {
	if memories == nil {
		return fmt.Errorf("permission approval cannot be verified without memory storage")
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		return fmt.Errorf("permission request id is required before approval")
	}
	runs, err := memories.ListToolRuns(ctx, 200)
	if err != nil {
		return fmt.Errorf("verify pending permission request: %w", err)
	}

	for _, run := range runs {
		toolName := normalizeToolName(run.ToolName)
		if toolName == "permission_decision" && permissionRunHasRequestID(run.Input, requestID) {
			return fmt.Errorf("permission request %s was already decided", requestID)
		}
		if toolName == normalizeToolName(request.ToolName) && permissionRunHasRequestID(run.Input, requestID) {
			return fmt.Errorf("permission request %s was already decided", requestID)
		}
	}

	var sawSameID bool
	for _, run := range runs {
		if normalizeToolName(run.ToolName) != "permission_request" {
			continue
		}
		stored, ok := permissionRequestFromStoredValue(run.Output)
		if !ok || strings.TrimSpace(stored.RequestID) != requestID {
			continue
		}
		sawSameID = true
		if samePermissionApprovalRequest(stored, request) {
			return nil
		}
	}
	if sawSameID {
		return fmt.Errorf("submitted permission request does not match the pending request %s", requestID)
	}
	if normalizeToolName(request.ToolName) == "edit_file" {
		if ok, mismatch := storedEditProposalAuthorizesRequest(runs, request); ok {
			return nil
		} else if mismatch {
			return fmt.Errorf("submitted edit permission request does not match the pending edit proposal %s", requestID)
		}
	}
	return fmt.Errorf("pending permission request %s was not found", requestID)
}

// StoredReadyEditProposalForPermission returns the approved, stored edit
// proposal that can be applied after the matching permission request is
// approved.
func StoredReadyEditProposalForPermission(ctx context.Context, memories *memory.Store, request PermissionRequest) (EditProposal, bool, error) {
	if memories == nil {
		return EditProposal{}, false, fmt.Errorf("permission approval cannot be verified without memory storage")
	}
	if normalizeToolName(request.ToolName) != "edit_file" {
		return EditProposal{}, false, nil
	}
	requestID := strings.TrimSpace(request.RequestID)
	if requestID == "" {
		return EditProposal{}, false, fmt.Errorf("permission request id is required before approval")
	}
	runs, err := memories.ListToolRuns(ctx, 200)
	if err != nil {
		return EditProposal{}, false, fmt.Errorf("load approved edit proposal: %w", err)
	}
	for _, run := range runs {
		if normalizeToolName(run.ToolName) != "edit_proposal" {
			continue
		}
		proposal, ok := editProposalFromStoredValue(run.Output)
		if !ok || strings.TrimSpace(proposal.RequestID) != requestID {
			continue
		}
		if !sameEditProposalPermissionRequest(proposal, request) {
			continue
		}
		if !editProposalReadyForApply(proposal) {
			continue
		}
		return proposal, true, nil
	}
	return EditProposal{}, false, nil
}

func editProposalReadyForApply(proposal EditProposal) bool {
	if strings.TrimSpace(proposal.Path) == "" {
		return false
	}
	if proposal.NeedsContent {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(proposal.Status), "ready_for_preview")
}

func storedEditProposalAuthorizesRequest(runs []memory.ToolRun, request PermissionRequest) (bool, bool) {
	requestID := strings.TrimSpace(request.RequestID)
	for _, run := range runs {
		if normalizeToolName(run.ToolName) != "edit_proposal" {
			continue
		}
		proposal, ok := editProposalFromStoredValue(run.Output)
		if !ok || strings.TrimSpace(proposal.RequestID) != requestID {
			continue
		}
		return sameEditProposalPermissionRequest(proposal, request), true
	}
	return false, false
}

func editProposalFromStoredValue(value any) (EditProposal, bool) {
	if proposal, ok := value.(EditProposal); ok {
		return proposal, true
	}
	data, err := json.Marshal(value)
	if err != nil {
		return EditProposal{}, false
	}
	var proposal EditProposal
	if err := json.Unmarshal(data, &proposal); err != nil {
		return EditProposal{}, false
	}
	return proposal, strings.TrimSpace(proposal.RequestID) != ""
}

func sameEditProposalPermissionRequest(proposal EditProposal, request PermissionRequest) bool {
	if strings.TrimSpace(proposal.RequestID) != strings.TrimSpace(request.RequestID) {
		return false
	}
	if normalizeToolName(request.ToolName) != "edit_file" {
		return false
	}
	if !request.DiffPreview || !request.SnapshotBeforeWrite || !request.RollbackSupported {
		return false
	}
	if len(request.Command) > 0 {
		command := normalizedCommand(request.Command)
		if len(command) < 2 || normalizeToolName(command[0]) != "edit_file" {
			return false
		}
		if filepathCleanInsensitive(command[1]) != filepathCleanInsensitive(proposal.Path) {
			return false
		}
	}
	return strings.TrimSpace(proposal.Path) != ""
}

func permissionRunHasRequestID(value any, requestID string) bool {
	request, ok := permissionRequestFromStoredValue(value)
	if ok && strings.TrimSpace(request.RequestID) == requestID {
		return true
	}
	data, err := json.Marshal(value)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), `"`+requestID+`"`)
}

func permissionRequestFromStoredValue(value any) (PermissionRequest, bool) {
	if request, ok := value.(PermissionRequest); ok {
		return request, true
	}
	data, err := json.Marshal(value)
	if err != nil {
		return PermissionRequest{}, false
	}
	var request PermissionRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return PermissionRequest{}, false
	}
	return request, strings.TrimSpace(request.RequestID) != ""
}

func samePermissionApprovalRequest(stored PermissionRequest, submitted PermissionRequest) bool {
	if strings.TrimSpace(stored.RequestID) != strings.TrimSpace(submitted.RequestID) {
		return false
	}
	if normalizeToolName(stored.ToolName) != normalizeToolName(submitted.ToolName) {
		return false
	}
	if !reflect.DeepEqual(normalizedCommand(stored.Command), normalizedCommand(submitted.Command)) {
		return false
	}
	return true
}

func normalizedCommand(command []string) []string {
	out := make([]string, 0, len(command))
	for _, part := range command {
		out = append(out, strings.TrimSpace(part))
	}
	return out
}

func filepathCleanInsensitive(path string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(path, "\\", "/")))
}
