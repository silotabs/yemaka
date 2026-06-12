package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreSavesAndSearchesMessagesWithFTS5(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "Explain local-first memory")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Explain why SQLite FTS5 helps low-end local agents.",
		Model:          "deepseek-r1:1.5b",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "FTS5 keeps retrieval lightweight without embeddings.",
		Model:          "deepseek-r1:1.5b",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	results, err := store.SearchMessages(ctx, "SQLite FTS5", 5)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(results) == 0 {
		t.Fatal("SearchMessages() returned no matches")
	}
	if results[0].ConversationID != conversation.ID {
		t.Fatalf("ConversationID = %q, want %q", results[0].ConversationID, conversation.ID)
	}
}

func TestListConversationMessagesPreservesInsertOrderForSameTimestamp(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "Same timestamp")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}

	createdAt := "2026-05-09T00:00:00Z"
	if _, err := store.SaveMessage(ctx, Message{
		ID:             "msg_user_same_second",
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "hello",
		Model:          "qwen",
		CreatedAt:      createdAt,
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ID:             "msg_assistant_same_second",
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Hello there.",
		Model:          "qwen",
		CreatedAt:      createdAt,
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	messages, err := store.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(messages))
	}
	if messages[0].Role != "user" || messages[1].Role != "assistant" {
		t.Fatalf("message order = %s, %s; want user, assistant", messages[0].Role, messages[1].Role)
	}
}

func TestCreateUserMessageVariantPreservesPromptHistory(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "Edit prompt")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "old prompt about apples",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "old answer",
		Model:          "qwen",
		ParentID:       user.ID,
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "later turn",
		Model:          "qwen",
	}); err != nil {
		t.Fatalf("SaveMessage(later user) error = %v", err)
	}

	updated, err := store.CreateUserMessageVariant(ctx, user.ID, "new prompt about oranges")
	if err != nil {
		t.Fatalf("CreateUserMessageVariant() error = %v", err)
	}
	if updated.Content != "new prompt about oranges" {
		t.Fatalf("updated content = %q", updated.Content)
	}
	if updated.ID == user.ID {
		t.Fatal("updated variant reused original message id")
	}
	if updated.ParentID != user.ID {
		t.Fatalf("updated parent id = %q, want %q", updated.ParentID, user.ID)
	}
	if updated.VariantIndex != 1 || !updated.ActiveVariant {
		t.Fatalf("updated variant index/active = %d/%v, want 1/true", updated.VariantIndex, updated.ActiveVariant)
	}

	messages, err := store.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("messages length = %d, want 4", len(messages))
	}
	if messages[0].ID != user.ID || messages[0].Content != "old prompt about apples" || messages[0].ActiveVariant {
		t.Fatalf("original prompt = %#v, want preserved inactive prompt", messages[0])
	}
	if messages[3].ID != updated.ID || messages[3].Content != "new prompt about oranges" || !messages[3].ActiveVariant {
		t.Fatalf("new prompt variant = %#v, want active edited variant", messages[3])
	}

	oldResults, err := store.SearchMessages(ctx, "apples", 5)
	if err != nil {
		t.Fatalf("SearchMessages(old) error = %v", err)
	}
	if len(oldResults) != 1 || oldResults[0].MessageID != user.ID {
		t.Fatalf("old prompt search results = %#v", oldResults)
	}
	newResults, err := store.SearchMessages(ctx, "oranges", 5)
	if err != nil {
		t.Fatalf("SearchMessages(new) error = %v", err)
	}
	if len(newResults) != 1 || newResults[0].MessageID != updated.ID {
		t.Fatalf("new prompt search results = %#v", newResults)
	}
}

func TestCreateUserMessageVariantSurvivesStoreReload(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.sqlite")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	conversation, err := store.CreateConversation(ctx, "Edit prompt reload")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	original, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "original prompt",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage(original) error = %v", err)
	}
	edited, err := store.CreateUserMessageVariant(ctx, original.ID, "edited prompt")
	if err != nil {
		t.Fatalf("CreateUserMessageVariant() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reloaded, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open(reloaded) error = %v", err)
	}
	defer reloaded.Close()
	messages, err := reloaded.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages(reloaded) error = %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages length = %d, want 2", len(messages))
	}
	if messages[0].ID != original.ID || messages[0].ActiveVariant {
		t.Fatalf("original after reload = %#v, want preserved inactive original", messages[0])
	}
	if messages[1].ID != edited.ID || messages[1].ParentID != original.ID || !messages[1].ActiveVariant || messages[1].VariantIndex != 1 {
		t.Fatalf("edited after reload = %#v, want active edited variant", messages[1])
	}
}

func TestEditedPromptVariantsRemainGroupedAfterReload(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.sqlite")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	conversation, err := store.CreateConversation(ctx, "Edit prompt grouped reload")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	original, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "original prompt",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage(original) error = %v", err)
	}
	firstEdit, err := store.CreateUserMessageVariant(ctx, original.ID, "first edited prompt")
	if err != nil {
		t.Fatalf("CreateUserMessageVariant(first) error = %v", err)
	}
	secondEdit, err := store.CreateUserMessageVariant(ctx, firstEdit.ID, "second edited prompt")
	if err != nil {
		t.Fatalf("CreateUserMessageVariant(second) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reloaded, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open(reloaded) error = %v", err)
	}
	defer reloaded.Close()
	messages, err := reloaded.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages(reloaded) error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("messages length = %d, want 3", len(messages))
	}
	if messages[0].ID != original.ID || messages[0].ParentID != "" || messages[0].ActiveVariant {
		t.Fatalf("original after reload = %#v, want inactive root prompt", messages[0])
	}
	if messages[1].ID != firstEdit.ID || messages[1].ParentID != original.ID || messages[1].VariantIndex != 1 || messages[1].ActiveVariant {
		t.Fatalf("first edit after reload = %#v, want inactive grouped variant", messages[1])
	}
	if messages[2].ID != secondEdit.ID || messages[2].ParentID != original.ID || messages[2].VariantIndex != 2 || !messages[2].ActiveVariant {
		t.Fatalf("second edit after reload = %#v, want active grouped variant", messages[2])
	}
}

func TestSearchMessagesSanitizesPunctuation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "Punctuation")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "hello from yemaka",
		Model:          "qwen2.5:3b",
	}); err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}

	results, err := store.SearchMessages(ctx, "hello!!!", 5)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("SearchMessages() len = %d, want 1", len(results))
	}
}

func TestSearchMessagesSanitizesStackTraceLikeTokens(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	_, err = store.SearchMessages(ctx, "index-P89FhvJf.js:6 Uncaught Error: direct children require key", 5)
	if err != nil {
		t.Fatalf("SearchMessages() stack trace query error = %v", err)
	}

	_, err = store.SearchMemories(ctx, "index-P89FhvJf.js:6 Uncaught Error: direct children require key", 5)
	if err != nil {
		t.Fatalf("SearchMemories() stack trace query error = %v", err)
	}
}

func TestConversationSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "hello session")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	renamed, err := store.UpdateConversationTitle(ctx, conversation.ID, "Greeting")
	if err != nil {
		t.Fatalf("UpdateConversationTitle() error = %v", err)
	}
	if renamed.Title != "Greeting" {
		t.Fatalf("renamed title = %q, want Greeting", renamed.Title)
	}
	starred, err := store.SetConversationStarred(ctx, conversation.ID, true)
	if err != nil {
		t.Fatalf("SetConversationStarred() error = %v", err)
	}
	if !starred.Starred {
		t.Fatal("Starred = false, want true")
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "persist me",
		Model:          "qwen",
	}); err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}
	listed, err := store.ListConversations(ctx, 10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != conversation.ID || !listed[0].Starred {
		t.Fatalf("ListConversations() = %+v, want one starred conversation", listed)
	}
	if err := store.DeleteConversation(ctx, conversation.ID); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}
	if _, err := store.GetConversation(ctx, conversation.ID); err == nil {
		t.Fatal("GetConversation() after delete succeeded, want error")
	}
	results, err := store.SearchMessages(ctx, "persist", 5)
	if err != nil {
		t.Fatalf("SearchMessages() after delete error = %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("SearchMessages() after delete len = %d, want 0", len(results))
	}
}

func TestAssistantResponseVariantsUseSameUserPrompt(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "retry")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "hello",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	first, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "hello one",
		Model:          "qwen",
		ParentID:       user.ID,
	})
	if err != nil {
		t.Fatalf("SaveMessage(first assistant) error = %v", err)
	}
	second, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "hello two",
		Model:          "qwen",
		ParentID:       user.ID,
	})
	if err != nil {
		t.Fatalf("SaveMessage(second assistant) error = %v", err)
	}
	if first.VariantIndex != 1 || second.VariantIndex != 2 {
		t.Fatalf("variant indexes = %d, %d; want 1, 2", first.VariantIndex, second.VariantIndex)
	}

	messages, err := store.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("messages length = %d, want 3", len(messages))
	}
	if messages[1].ActiveVariant || !messages[2].ActiveVariant {
		t.Fatalf("active variants = %v, %v; want first inactive second active", messages[1].ActiveVariant, messages[2].ActiveVariant)
	}
	if messages[1].ParentID != user.ID || messages[2].ParentID != user.ID {
		t.Fatalf("parent ids = %q, %q; want %q", messages[1].ParentID, messages[2].ParentID, user.ID)
	}
}

func TestAssistantResponseVariantsSurviveStoreReload(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.sqlite")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	conversation, err := store.CreateConversation(ctx, "retry reload")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "hello",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "first response",
		Model:          "qwen",
		ParentID:       user.ID,
	}); err != nil {
		t.Fatalf("SaveMessage(first assistant) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "retry response",
		Model:          "qwen",
		ParentID:       user.ID,
	}); err != nil {
		t.Fatalf("SaveMessage(second assistant) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reloaded, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open(reloaded) error = %v", err)
	}
	defer reloaded.Close()
	messages, err := reloaded.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages(reloaded) error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("messages length = %d, want 3", len(messages))
	}
	if messages[1].ParentID != user.ID || messages[2].ParentID != user.ID {
		t.Fatalf("parent ids after reload = %q, %q; want %q", messages[1].ParentID, messages[2].ParentID, user.ID)
	}
	if messages[1].VariantIndex != 1 || messages[2].VariantIndex != 2 || messages[1].ActiveVariant || !messages[2].ActiveVariant {
		t.Fatalf("variants after reload = (%d,%v), (%d,%v); want first inactive and second active", messages[1].VariantIndex, messages[1].ActiveVariant, messages[2].VariantIndex, messages[2].ActiveVariant)
	}
}

func TestAssistantResponseVariantsRemainGroupedAfterReloadWithInferredParents(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "memory.sqlite")
	store, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	conversation, err := store.CreateConversation(ctx, "legacy retry reload")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "hello",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	legacy, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "first response from before response parents existed",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage(legacy assistant) error = %v", err)
	}
	retry, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "retry response",
		Model:          "qwen",
		ParentID:       user.ID,
	})
	if err != nil {
		t.Fatalf("SaveMessage(retry assistant) error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	reloaded, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatalf("Open(reloaded) error = %v", err)
	}
	defer reloaded.Close()
	messages, err := reloaded.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages(reloaded) error = %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("messages length = %d, want 3", len(messages))
	}
	if messages[1].ID != legacy.ID || messages[1].ParentID != "" {
		t.Fatalf("legacy assistant before display inference = %#v, want stored parentless row", messages[1])
	}
	grouped := WithInferredResponseParents(messages)
	if grouped[1].ID != legacy.ID || grouped[1].ParentID != user.ID {
		t.Fatalf("legacy assistant after display inference = %#v, want grouped under %q", grouped[1], user.ID)
	}
	if grouped[2].ID != retry.ID || grouped[2].ParentID != user.ID {
		t.Fatalf("retry assistant after reload = %#v, want grouped under %q", grouped[2], user.ID)
	}
	if messages[1].ParentID != "" {
		t.Fatal("WithInferredResponseParents mutated the reloaded message slice")
	}
}

func TestSaveToolRun(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	run, err := store.SaveToolRun(ctx, ToolRun{
		ConversationID:     "conv_test",
		SessionID:          "sess_test",
		UserMessageID:      "msg_user",
		AssistantMessageID: "msg_assistant",
		ParentMessageID:    "msg_user",
		VariantIndex:       2,
		ToolName:           "write_file",
		Input:              map[string]any{"path": "README.md"},
		Output:             map[string]any{"snapshot_id": "snap_test"},
		Status:             "completed",
		RiskLevel:          "medium",
	})
	if err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}
	if run.ID == "" {
		t.Fatal("tool run ID is empty")
	}

	runs, err := store.ListToolRuns(ctx, 10)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("ListToolRuns() len = %d, want 1", len(runs))
	}
	if runs[0].ToolName != "write_file" {
		t.Fatalf("ToolName = %q, want write_file", runs[0].ToolName)
	}
	if runs[0].Status != "completed" {
		t.Fatalf("Status = %q, want completed", runs[0].Status)
	}
	if runs[0].ConversationID != "conv_test" || runs[0].SessionID != "sess_test" || runs[0].UserMessageID != "msg_user" || runs[0].AssistantMessageID != "msg_assistant" || runs[0].ParentMessageID != "msg_user" || runs[0].VariantIndex != 2 {
		t.Fatalf("metadata = %#v, want persisted conversation/session/message grouping fields", runs[0])
	}
	filtered, err := store.ListToolRunsFiltered(ctx, ToolRunFilter{ConversationID: "conv_test", SessionID: "sess_test", UserMessageID: "msg_user", Limit: 10})
	if err != nil {
		t.Fatalf("ListToolRunsFiltered() error = %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != run.ID {
		t.Fatalf("ListToolRunsFiltered() = %#v, want saved run %q", filtered, run.ID)
	}
	misses, err := store.ListToolRunsFiltered(ctx, ToolRunFilter{SessionID: "sess_other", Limit: 10})
	if err != nil {
		t.Fatalf("ListToolRunsFiltered(miss) error = %v", err)
	}
	if len(misses) != 0 {
		t.Fatalf("ListToolRunsFiltered(miss) len = %d, want 0", len(misses))
	}
}

func TestChatAttachmentsLinkListAndDeleteWithConversation(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "attachment session")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Use the attachment",
		Model:          "qwen",
	})
	if err != nil {
		t.Fatalf("SaveMessage() error = %v", err)
	}
	saved, err := store.SaveChatAttachment(ctx, ChatAttachment{
		ID:          "att_test",
		FileName:    "/Users/example/Documents/private-note.txt",
		ContentType: "text/plain",
		SizeBytes:   42,
		Status:      "ready",
		Summary:     "ready",
		Preview:     "project alpha",
		SourceKind:  "attachment",
		Sources:     []string{"/Users/example/Documents/private-note.txt"},
		Retention:   "conversation",
		Content:     "project alpha should be local to this turn token=secret-value",
	})
	if err != nil {
		t.Fatalf("SaveChatAttachment() error = %v", err)
	}
	if saved.FileName != "private-note.txt" {
		t.Fatalf("saved.FileName = %q, want redacted basename", saved.FileName)
	}
	if saved.ExpiresAt == "" {
		t.Fatal("saved.ExpiresAt is empty, want expiry for unlinked upload")
	}
	if strings.Contains(strings.Join(saved.Sources, " "), "/Users/example") || strings.Contains(saved.Content, "secret-value") {
		t.Fatalf("saved attachment was not redacted: %+v", saved)
	}
	linked, err := store.LinkChatAttachments(ctx, conversation.ID, user.ID, []string{saved.ID})
	if err != nil {
		t.Fatalf("LinkChatAttachments() error = %v", err)
	}
	if len(linked) != 1 || linked[0].ConversationID != conversation.ID || linked[0].UserMessageID != user.ID {
		t.Fatalf("linked attachments = %+v, want linked conversation/message ids", linked)
	}
	if linked[0].ExpiresAt != "" {
		t.Fatalf("linked ExpiresAt = %q, want cleared conversation retention", linked[0].ExpiresAt)
	}
	byMessage, err := store.ListChatAttachmentsForMessages(ctx, []string{user.ID})
	if err != nil {
		t.Fatalf("ListChatAttachmentsForMessages() error = %v", err)
	}
	if len(byMessage[user.ID]) != 1 || byMessage[user.ID][0].Content == "" {
		t.Fatalf("message attachments = %+v, want saved attachment with content", byMessage[user.ID])
	}
	if err := store.DeleteConversation(ctx, conversation.ID); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}
	byMessage, err = store.ListChatAttachmentsForMessages(ctx, []string{user.ID})
	if err != nil {
		t.Fatalf("ListChatAttachmentsForMessages(after delete) error = %v", err)
	}
	if len(byMessage[user.ID]) != 0 {
		t.Fatalf("attachments after conversation delete = %+v, want none", byMessage[user.ID])
	}
}

func TestConversationRouteStatePersistsAndDeletes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	conversation, err := store.CreateConversation(ctx, "route state")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	state, err := store.SaveConversationRouteState(ctx, ConversationRouteState{
		ConversationID: conversation.ID,
		StateJSON:      `{"active_route":"rag_search","task_status":"active"}`,
	})
	if err != nil {
		t.Fatalf("SaveConversationRouteState() error = %v", err)
	}
	if state.CreatedAt == "" || state.UpdatedAt == "" {
		t.Fatalf("timestamps were not populated: %+v", state)
	}
	loaded, ok, err := store.GetConversationRouteState(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("GetConversationRouteState() error = %v", err)
	}
	if !ok {
		t.Fatal("GetConversationRouteState() ok = false, want true")
	}
	if loaded.StateJSON != `{"active_route":"rag_search","task_status":"active"}` {
		t.Fatalf("StateJSON = %q", loaded.StateJSON)
	}
	if err := store.DeleteConversation(ctx, conversation.ID); err != nil {
		t.Fatalf("DeleteConversation() error = %v", err)
	}
	if _, ok, err := store.GetConversationRouteState(ctx, conversation.ID); err != nil {
		t.Fatalf("GetConversationRouteState(after delete) error = %v", err)
	} else if ok {
		t.Fatal("route state still exists after conversation delete")
	}
}

func TestMemoryStorageRedactsSecretsAndUserPaths(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	item, err := store.SaveMemory(ctx, Memory{
		Kind:       "workflow_success",
		Content:    "Use token=super-secret-value in /Users/example/project and sk-thisshouldberemoved123.",
		Importance: 3,
		Source:     "test",
	})
	if err != nil {
		t.Fatalf("SaveMemory() error = %v", err)
	}
	if strings.Contains(item.Content, "super-secret") || strings.Contains(item.Content, "/Users/example") || strings.Contains(item.Content, "sk-this") {
		t.Fatalf("memory content was not redacted: %q", item.Content)
	}

	conversation, err := store.CreateConversation(ctx, "summary")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	summary := BuildConversationSummary([]Message{{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "The password=hunter2 lives under /Users/example/project.",
	}}, 500)
	if strings.Contains(summary, "hunter2") || strings.Contains(summary, "/Users/example") {
		t.Fatalf("summary was not redacted: %q", summary)
	}
}

func TestToolRunStorageRedactsSecretsAndUserPaths(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.SaveToolRun(ctx, ToolRun{
		ToolName: "read_file",
		Input:    map[string]any{"path": "/Users/example/project/.env"},
		Output:   map[string]any{"content": "api_key=sk-thisshouldberemoved123"},
		Status:   "completed",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}
	runs, err := store.ListToolRuns(ctx, 1)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	input := runs[0].Input.(map[string]any)["path"].(string)
	output := runs[0].Output.(map[string]any)["content"].(string)
	if strings.Contains(input, "/Users/example") || strings.Contains(output, "sk-this") {
		t.Fatalf("tool run was not redacted: input=%q output=%q", input, output)
	}
}

func TestExplicitMemorySearchPinAndDelete(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	low, err := store.SaveMemory(ctx, Memory{
		Kind:       "project_note",
		Content:    "The project uses SQLite FTS5 for local memory.",
		Importance: 2,
		Source:     "test",
	})
	if err != nil {
		t.Fatalf("SaveMemory(low) error = %v", err)
	}
	high, err := store.SaveMemory(ctx, Memory{
		Kind:       "preference",
		Content:    "Prefer concise SQLite explanations.",
		Importance: 5,
		Source:     "user",
	})
	if err != nil {
		t.Fatalf("SaveMemory(high) error = %v", err)
	}

	results, err := store.SearchMemories(ctx, "SQLite", 5)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("SearchMemories() len = %d, want 2", len(results))
	}
	if results[0].ID != high.ID {
		t.Fatalf("first memory = %q, want high importance %q", results[0].ID, high.ID)
	}

	if err := store.PinMemory(ctx, low.ID, true); err != nil {
		t.Fatalf("PinMemory() error = %v", err)
	}
	results, err = store.SearchMemories(ctx, "SQLite", 5)
	if err != nil {
		t.Fatalf("SearchMemories() after pin error = %v", err)
	}
	if results[0].ID != low.ID || !results[0].Pinned {
		t.Fatalf("pinned memory was not ranked first: %+v", results[0])
	}

	if err := store.DisableMemory(ctx, low.ID); err != nil {
		t.Fatalf("DisableMemory() error = %v", err)
	}
	results, err = store.SearchMemories(ctx, "SQLite", 5)
	if err != nil {
		t.Fatalf("SearchMemories() after delete error = %v", err)
	}
	for _, result := range results {
		if result.ID == low.ID {
			t.Fatal("disabled memory was returned in search")
		}
	}
}

func TestConversationSummaryMemoryAndPruneDuplicates(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	conversation, err := store.CreateConversation(ctx, "Summarize memory")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Remember that Yemaka uses compact local summaries.",
		Model:          "qwen2.5:3b",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := store.SaveMessage(ctx, Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Compact local summaries avoid injecting long history.",
		Model:          "qwen2.5:3b",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	count, err := store.CountConversationMessages(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("CountConversationMessages() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("message count = %d, want 2", count)
	}
	messages, err := store.ListConversationMessages(ctx, conversation.ID, 2)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	summary := BuildConversationSummary(messages, 500)
	if summary == "" {
		t.Fatal("summary is empty")
	}
	if _, err := store.SaveConversationSummary(ctx, conversation.ID, summary); err != nil {
		t.Fatalf("SaveConversationSummary() error = %v", err)
	}
	if _, err := store.SaveConversationSummary(ctx, conversation.ID, summary+" Updated summary content."); err != nil {
		t.Fatalf("SaveConversationSummary(update) error = %v", err)
	}
	results, err := store.SearchMemories(ctx, "Updated summary content", 5)
	if err != nil {
		t.Fatalf("SearchMemories(summary) error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("summary search len = %d, want 1", len(results))
	}
	if results[0].Kind != "summary" {
		t.Fatalf("summary kind = %q, want summary", results[0].Kind)
	}

	if _, err := store.SaveMemory(ctx, Memory{
		Kind:       "project_note",
		Content:    "Duplicate local memory content.",
		Importance: 1,
		Source:     "first",
	}); err != nil {
		t.Fatalf("SaveMemory(duplicate first) error = %v", err)
	}
	kept, err := store.SaveMemory(ctx, Memory{
		Kind:       "project_note",
		Content:    "Duplicate local memory content.",
		Importance: 5,
		Source:     "second",
	})
	if err != nil {
		t.Fatalf("SaveMemory(duplicate second) error = %v", err)
	}
	pruned, err := store.PruneDuplicateMemories(ctx)
	if err != nil {
		t.Fatalf("PruneDuplicateMemories() error = %v", err)
	}
	if pruned != 1 {
		t.Fatalf("pruned = %d, want 1", pruned)
	}
	results, err = store.SearchMemories(ctx, "Duplicate local memory", 5)
	if err != nil {
		t.Fatalf("SearchMemories(duplicate) error = %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("duplicate search len = %d, want 1", len(results))
	}
	if results[0].ID != kept.ID {
		t.Fatalf("kept memory = %q, want high-importance %q", results[0].ID, kept.ID)
	}
}

func TestSaveSkillUsed(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	usage, err := store.SaveSkillUsed(ctx, SkillUsed{
		SkillName:    "project_explainer",
		SkillVersion: "0.1.0",
	})
	if err != nil {
		t.Fatalf("SaveSkillUsed() error = %v", err)
	}
	if usage.ID == "" {
		t.Fatal("skill usage ID is empty")
	}
}
