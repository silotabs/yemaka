package memory

import (
	"strings"
	"testing"
)

func TestWithInferredResponseParentsChronological(t *testing.T) {
	messages := []Message{
		{ID: "user_1", Role: "user", Content: "hello"},
		{ID: "msg_old", Role: "assistant", Content: "first"},
		{ID: "msg_retry", Role: "assistant", Content: "retry", ParentID: "user_1", VariantIndex: 1},
	}

	result := WithInferredResponseParents(messages)

	if result[1].ParentID != "user_1" {
		t.Fatalf("old assistant parent = %q, want user_1", result[1].ParentID)
	}
	if result[2].ParentID != "user_1" {
		t.Fatalf("retry parent = %q, want user_1", result[2].ParentID)
	}
	if messages[1].ParentID != "" {
		t.Fatal("WithInferredResponseParents mutated the input slice")
	}
}

func TestWithInferredResponseParentsDescending(t *testing.T) {
	messages := []Message{
		{ID: "msg_retry", Role: "assistant", Content: "retry", ParentID: "user_1", VariantIndex: 1},
		{ID: "msg_old", Role: "assistant", Content: "first"},
		{ID: "user_1", Role: "user", Content: "hello"},
	}

	result := WithInferredResponseParents(messages)

	if result[1].ParentID != "user_1" {
		t.Fatalf("old assistant parent = %q, want user_1", result[1].ParentID)
	}
}

func TestWithInferredResponseParentsKeepsSeparateTurns(t *testing.T) {
	messages := []Message{
		{ID: "user_1", Role: "user", Content: "hello"},
		{ID: "msg_1", Role: "assistant", Content: "first"},
		{ID: "user_2", Role: "user", Content: "next"},
		{ID: "msg_2", Role: "assistant", Content: "second"},
	}

	result := WithInferredResponseParents(messages)

	if result[1].ParentID != "user_1" {
		t.Fatalf("first assistant parent = %q, want user_1", result[1].ParentID)
	}
	if result[3].ParentID != "user_2" {
		t.Fatalf("second assistant parent = %q, want user_2", result[3].ParentID)
	}
}

func TestActiveConversationMessagesKeepsLatestAssistantVariant(t *testing.T) {
	messages := []Message{
		{ID: "user_1", Role: "user", Content: "hello", ActiveVariant: true},
		{ID: "msg_old", Role: "assistant", Content: "old answer", ParentID: "user_1", ActiveVariant: false},
		{ID: "msg_retry", Role: "assistant", Content: "retry answer", ParentID: "user_1", VariantIndex: 1, ActiveVariant: true},
	}

	result := ActiveConversationMessages(messages)

	if len(result) != 2 {
		t.Fatalf("active messages len = %d, want 2: %+v", len(result), result)
	}
	if result[0].ID != "user_1" || result[1].ID != "msg_retry" {
		t.Fatalf("active messages = %+v, want user_1 and msg_retry", result)
	}
}

func TestActiveConversationMessagesDropsInactiveEditedPromptBranch(t *testing.T) {
	messages := []Message{
		{ID: "user_old", Role: "user", Content: "old prompt", ActiveVariant: false},
		{ID: "msg_old", Role: "assistant", Content: "old branch answer", ParentID: "user_old", ActiveVariant: true},
		{ID: "user_new", Role: "user", Content: "new prompt", ParentID: "user_old", VariantIndex: 1, ActiveVariant: true},
		{ID: "msg_new", Role: "assistant", Content: "new branch answer", ParentID: "user_new", ActiveVariant: true},
	}

	result := ActiveConversationMessages(messages)

	if len(result) != 2 {
		t.Fatalf("active messages len = %d, want 2: %+v", len(result), result)
	}
	if result[0].ID != "user_new" || result[1].ID != "msg_new" {
		t.Fatalf("active messages = %+v, want user_new and msg_new", result)
	}
}

func TestActiveConversationMessagesEmptyReturnsNonNilSlice(t *testing.T) {
	result := ActiveConversationMessages(nil)

	if result == nil {
		t.Fatal("ActiveConversationMessages(nil) returned nil, want empty slice")
	}
	if len(result) != 0 {
		t.Fatalf("active messages len = %d, want 0", len(result))
	}
}

func TestBuildConversationSummaryUsesActiveConversationMessages(t *testing.T) {
	messages := []Message{
		{ID: "user_old", Role: "user", Content: "stale prompt", ActiveVariant: false},
		{ID: "msg_old", Role: "assistant", Content: "stale answer", ParentID: "user_old", ActiveVariant: true},
		{ID: "user_new", Role: "user", Content: "current prompt", ParentID: "user_old", VariantIndex: 1, ActiveVariant: true},
		{ID: "msg_new", Role: "assistant", Content: "current answer", ParentID: "user_new", ActiveVariant: true},
	}

	summary := BuildConversationSummary(messages, 500)

	if !strings.Contains(summary, "current prompt") || !strings.Contains(summary, "current answer") {
		t.Fatalf("summary = %q, want current active branch", summary)
	}
	if strings.Contains(summary, "stale prompt") || strings.Contains(summary, "stale answer") {
		t.Fatalf("summary = %q, want inactive branch excluded", summary)
	}
}
