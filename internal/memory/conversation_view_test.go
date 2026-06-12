package memory

import "testing"

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
