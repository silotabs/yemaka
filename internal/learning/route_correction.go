package learning

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"yemaka/internal/memory"
	"yemaka/internal/routing"
)

const (
	KindRouteCorrection       = "route_correction"
	RouteCorrectionSchema     = "yemaka.route_correction.v1"
	defaultRouteCorrectionMax = 8
)

type storedRouteCorrection struct {
	Schema                string   `json:"schema"`
	ID                    string   `json:"id"`
	SourceConversationID  string   `json:"sourceConversationId,omitempty"`
	Pattern               string   `json:"pattern"`
	IntendedRouteCategory string   `json:"intendedRouteCategory"`
	IntendedTaskType      string   `json:"intendedTaskType,omitempty"`
	IntendedCapability    string   `json:"intendedCapability,omitempty"`
	RequiredTools         []string `json:"requiredTools,omitempty"`
	ForbiddenTools        []string `json:"forbiddenTools,omitempty"`
	Tags                  []string `json:"tags,omitempty"`
	OriginalPrompt        string   `json:"originalPrompt,omitempty"`
	CorrectionText        string   `json:"correctionText,omitempty"`
	ClarificationQuestion string   `json:"clarificationQuestion,omitempty"`
	ApprovalStatus        string   `json:"approvalStatus"`
	Disabled              bool     `json:"disabled"`
	CreatedAt             string   `json:"createdAt"`
	UpdatedAt             string   `json:"updatedAt"`
}

func SaveRouteCorrection(ctx context.Context, store *memory.Store, correction routing.RouteCorrection) (routing.RouteCorrection, error) {
	if store == nil {
		return routing.RouteCorrection{}, fmt.Errorf("memory store is required")
	}
	correction, err := normalizeRouteCorrection(correction)
	if err != nil {
		return routing.RouteCorrection{}, err
	}
	data, err := json.Marshal(storedRouteCorrectionFromRouting(correction))
	if err != nil {
		return routing.RouteCorrection{}, fmt.Errorf("marshal route correction: %w", err)
	}
	item, err := store.SaveMemory(ctx, memory.Memory{
		ID:         correction.ID,
		Kind:       KindRouteCorrection,
		Content:    string(data),
		Importance: 5,
		Source:     "route_correction:" + correction.SourceConversationID,
		Pinned:     correction.ApprovalStatus == routing.RouteCorrectionStatusApproved,
		Disabled:   correction.Disabled,
		CreatedAt:  correction.CreatedAt,
		UpdatedAt:  correction.UpdatedAt,
	})
	if err != nil {
		return routing.RouteCorrection{}, err
	}
	correction.ID = item.ID
	correction.CreatedAt = item.CreatedAt
	correction.UpdatedAt = item.UpdatedAt
	return correction, nil
}

func ListRouteCorrections(ctx context.Context, store *memory.Store, limit int) ([]routing.RouteCorrection, error) {
	if store == nil {
		return nil, fmt.Errorf("memory store is required")
	}
	if limit <= 0 {
		limit = 50
	}
	memories, err := store.ListMemories(ctx, limit)
	if err != nil {
		return nil, err
	}
	corrections := make([]routing.RouteCorrection, 0, len(memories))
	for _, item := range memories {
		if item.Kind != KindRouteCorrection {
			continue
		}
		correction, ok := routeCorrectionFromMemory(item)
		if ok {
			corrections = append(corrections, correction)
		}
	}
	return corrections, nil
}

func ApprovedRouteCorrections(ctx context.Context, store *memory.Store, query string, limit int) ([]routing.RouteCorrection, error) {
	if store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultRouteCorrectionMax
	}
	seen := map[string]bool{}
	corrections := make([]routing.RouteCorrection, 0, limit)
	add := func(item memory.MemorySearchResult) {
		if len(corrections) >= limit || seen[item.ID] || item.Kind != KindRouteCorrection {
			return
		}
		correction, ok := routeCorrectionFromMemorySearchResult(item)
		if !ok || correction.Disabled || correction.ApprovalStatus != routing.RouteCorrectionStatusApproved {
			return
		}
		seen[item.ID] = true
		corrections = append(corrections, correction)
	}
	matches, err := store.SearchMemories(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	for _, item := range matches {
		add(item)
	}
	if len(corrections) < limit {
		listed, err := store.ListMemories(ctx, limit*4)
		if err != nil {
			return nil, err
		}
		for _, item := range listed {
			add(item)
		}
	}
	return corrections, nil
}

func ApproveRouteCorrection(ctx context.Context, store *memory.Store, id string) (routing.RouteCorrection, error) {
	return setRouteCorrectionStatus(ctx, store, id, routing.RouteCorrectionStatusApproved, false)
}

func DisableRouteCorrection(ctx context.Context, store *memory.Store, id string) (routing.RouteCorrection, error) {
	return setRouteCorrectionStatus(ctx, store, id, routing.RouteCorrectionStatusRejected, true)
}

func LatestPendingRouteCorrection(ctx context.Context, store *memory.Store, conversationID string) (routing.RouteCorrection, bool, error) {
	if store == nil {
		return routing.RouteCorrection{}, false, fmt.Errorf("memory store is required")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return routing.RouteCorrection{}, false, nil
	}
	corrections, err := ListRouteCorrections(ctx, store, 100)
	if err != nil {
		return routing.RouteCorrection{}, false, err
	}
	for _, correction := range corrections {
		if correction.Disabled || correction.ApprovalStatus != routing.RouteCorrectionStatusPending {
			continue
		}
		if correction.SourceConversationID == conversationID {
			return correction, true, nil
		}
	}
	return routing.RouteCorrection{}, false, nil
}

func setRouteCorrectionStatus(ctx context.Context, store *memory.Store, id string, status string, disabled bool) (routing.RouteCorrection, error) {
	if store == nil {
		return routing.RouteCorrection{}, fmt.Errorf("memory store is required")
	}
	item, err := store.GetMemory(ctx, id)
	if err != nil {
		return routing.RouteCorrection{}, err
	}
	if item.Kind != KindRouteCorrection {
		return routing.RouteCorrection{}, fmt.Errorf("memory %s is not a route correction", id)
	}
	correction, ok := routeCorrectionFromMemory(memory.MemorySearchResult{
		ID:         item.ID,
		Kind:       item.Kind,
		Content:    item.Content,
		Importance: item.Importance,
		Source:     item.Source,
		Pinned:     item.Pinned,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
	})
	if !ok {
		return routing.RouteCorrection{}, fmt.Errorf("invalid route correction: %s", id)
	}
	correction.ApprovalStatus = status
	correction.Disabled = disabled
	correction.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(storedRouteCorrectionFromRouting(correction))
	if err != nil {
		return routing.RouteCorrection{}, fmt.Errorf("marshal route correction: %w", err)
	}
	updated, err := store.UpdateMemory(ctx, memory.Memory{
		ID:         item.ID,
		Kind:       KindRouteCorrection,
		Content:    string(data),
		Importance: item.Importance,
		Source:     item.Source,
		Pinned:     status == routing.RouteCorrectionStatusApproved && !disabled,
		Disabled:   disabled,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  correction.UpdatedAt,
	})
	if err != nil {
		return routing.RouteCorrection{}, err
	}
	correction.UpdatedAt = updated.UpdatedAt
	return correction, nil
}

func routeCorrectionFromMemorySearchResult(item memory.MemorySearchResult) (routing.RouteCorrection, bool) {
	return routeCorrectionFromMemory(item)
}

func routeCorrectionFromMemory(item memory.MemorySearchResult) (routing.RouteCorrection, bool) {
	if item.Kind != KindRouteCorrection {
		return routing.RouteCorrection{}, false
	}
	var stored storedRouteCorrection
	if err := json.Unmarshal([]byte(item.Content), &stored); err != nil {
		return routing.RouteCorrection{}, false
	}
	if stored.Schema != RouteCorrectionSchema {
		return routing.RouteCorrection{}, false
	}
	correction := routing.RouteCorrection{
		ID:                    firstNonEmpty(stored.ID, item.ID),
		SourceConversationID:  stored.SourceConversationID,
		Pattern:               stored.Pattern,
		IntendedRouteCategory: stored.IntendedRouteCategory,
		IntendedTaskType:      stored.IntendedTaskType,
		IntendedCapability:    stored.IntendedCapability,
		RequiredTools:         append([]string{}, stored.RequiredTools...),
		ForbiddenTools:        append([]string{}, stored.ForbiddenTools...),
		Tags:                  append([]string{}, stored.Tags...),
		OriginalPrompt:        stored.OriginalPrompt,
		CorrectionText:        stored.CorrectionText,
		ClarificationQuestion: stored.ClarificationQuestion,
		ApprovalStatus:        stored.ApprovalStatus,
		Disabled:              stored.Disabled,
		CreatedAt:             firstNonEmpty(stored.CreatedAt, item.CreatedAt),
		UpdatedAt:             firstNonEmpty(stored.UpdatedAt, item.UpdatedAt),
	}
	return correction, true
}

func normalizeRouteCorrection(correction routing.RouteCorrection) (routing.RouteCorrection, error) {
	if strings.TrimSpace(correction.ID) == "" {
		correction.ID = newRouteCorrectionID()
	}
	correction.SourceConversationID, _ = SanitizeText(strings.TrimSpace(correction.SourceConversationID))
	correction.Pattern, _ = SanitizeText(strings.TrimSpace(correction.Pattern))
	correction.IntendedRouteCategory = strings.TrimSpace(correction.IntendedRouteCategory)
	correction.IntendedTaskType = strings.TrimSpace(correction.IntendedTaskType)
	correction.IntendedCapability = strings.TrimSpace(correction.IntendedCapability)
	correction.OriginalPrompt, _ = SanitizeText(strings.TrimSpace(correction.OriginalPrompt))
	correction.CorrectionText, _ = SanitizeText(strings.TrimSpace(correction.CorrectionText))
	correction.ClarificationQuestion, _ = SanitizeText(strings.TrimSpace(correction.ClarificationQuestion))
	correction.RequiredTools = sanitizeRouteCorrectionList(correction.RequiredTools)
	correction.ForbiddenTools = sanitizeRouteCorrectionList(correction.ForbiddenTools)
	correction.Tags = sanitizeRouteCorrectionTags(correction.Tags)
	correction.ApprovalStatus = strings.ToLower(strings.TrimSpace(correction.ApprovalStatus))
	if correction.ApprovalStatus == "" {
		correction.ApprovalStatus = routing.RouteCorrectionStatusPending
	}
	if correction.Pattern == "" {
		return routing.RouteCorrection{}, fmt.Errorf("route correction pattern is required")
	}
	if !validRouteCorrectionStatus(correction.ApprovalStatus) {
		return routing.RouteCorrection{}, fmt.Errorf("unsupported route correction status: %s", correction.ApprovalStatus)
	}
	if !validRouteCorrectionCategory(correction.IntendedRouteCategory) {
		return routing.RouteCorrection{}, fmt.Errorf("unsupported route correction category: %s", correction.IntendedRouteCategory)
	}
	if !validRouteCorrectionTask(correction.IntendedTaskType) {
		return routing.RouteCorrection{}, fmt.Errorf("unsupported route correction task type: %s", correction.IntendedTaskType)
	}
	if !validRouteCorrectionTools(correction.RequiredTools) || !validRouteCorrectionTools(correction.ForbiddenTools) {
		return routing.RouteCorrection{}, fmt.Errorf("route correction tools must be read/search-only tools")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if correction.CreatedAt == "" {
		correction.CreatedAt = now
	}
	if correction.UpdatedAt == "" {
		correction.UpdatedAt = correction.CreatedAt
	}
	return correction, nil
}

func storedRouteCorrectionFromRouting(correction routing.RouteCorrection) storedRouteCorrection {
	return storedRouteCorrection{
		Schema:                RouteCorrectionSchema,
		ID:                    correction.ID,
		SourceConversationID:  correction.SourceConversationID,
		Pattern:               correction.Pattern,
		IntendedRouteCategory: correction.IntendedRouteCategory,
		IntendedTaskType:      correction.IntendedTaskType,
		IntendedCapability:    correction.IntendedCapability,
		RequiredTools:         append([]string{}, correction.RequiredTools...),
		ForbiddenTools:        append([]string{}, correction.ForbiddenTools...),
		Tags:                  append([]string{}, correction.Tags...),
		OriginalPrompt:        correction.OriginalPrompt,
		CorrectionText:        correction.CorrectionText,
		ClarificationQuestion: correction.ClarificationQuestion,
		ApprovalStatus:        correction.ApprovalStatus,
		Disabled:              correction.Disabled,
		CreatedAt:             correction.CreatedAt,
		UpdatedAt:             correction.UpdatedAt,
	}
}

func validRouteCorrectionStatus(status string) bool {
	switch status {
	case routing.RouteCorrectionStatusPending, routing.RouteCorrectionStatusApproved, routing.RouteCorrectionStatusRejected:
		return true
	default:
		return false
	}
}

func validRouteCorrectionCategory(category string) bool {
	switch category {
	case routing.RouteChatExplanation,
		routing.RouteMemorySearch,
		routing.RouteRAGSearch,
		routing.RouteInternetSearch,
		routing.RouteInternetFetch,
		routing.RouteInternetHead,
		routing.RouteWorkspaceRead,
		routing.RouteFileRead,
		routing.RouteClarify:
		return true
	default:
		return false
	}
}

func validRouteCorrectionTask(task string) bool {
	switch task {
	case "", routing.TaskChat, routing.TaskReasoning, routing.TaskRAG, routing.TaskTool:
		return true
	default:
		return false
	}
}

func validRouteCorrectionTools(tools []string) bool {
	for _, tool := range tools {
		switch strings.TrimSpace(tool) {
		case "", "memory_search", "rag_search", "internet_search", "internet_fetch", "internet_head", "read_file", "search_files", "list_files":
			continue
		default:
			return false
		}
	}
	return true
}

func sanitizeRouteCorrectionList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value, _ = SanitizeText(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	return out
}

func sanitizeRouteCorrectionTags(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value, _ = SanitizeText(strings.ToLower(strings.TrimSpace(value)))
		if value == "" || seen[value] {
			continue
		}
		if !validRouteCorrectionTag(value) {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func validRouteCorrectionTag(value string) bool {
	for _, prefix := range []string{"source:", "intent:", "avoid:", "route:"} {
		if strings.HasPrefix(value, prefix) && len(value) > len(prefix) {
			return true
		}
	}
	return false
}

func newRouteCorrectionID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err == nil {
		return "routecorr_" + hex.EncodeToString(raw[:])
	}
	return fmt.Sprintf("routecorr_%d", time.Now().UnixNano())
}
