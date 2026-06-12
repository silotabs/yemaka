package knowledge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestStoreUpsertsEntitiesAndSearches(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	first, err := store.UpsertEntity(ctx, EntityInput{
		Name:     "Cogging Torque",
		Kind:     "Engineering Concept",
		Source:   "manual QA",
		Evidence: "Cogging torque appears in motor design notes.",
	})
	if err != nil {
		t.Fatalf("UpsertEntity() error = %v", err)
	}
	second, err := store.UpsertEntity(ctx, EntityInput{
		Name:     "  cogging   torque ",
		Kind:     "engineering concept",
		Source:   "manual QA",
		Evidence: "Updated evidence about electric motors.",
	})
	if err != nil {
		t.Fatalf("UpsertEntity() second error = %v", err)
	}
	if first.ID != second.ID {
		t.Fatalf("duplicate entity ID = %q, want %q", second.ID, first.ID)
	}
	if second.Kind != "engineering_concept" || second.Source != "manual_qa" {
		t.Fatalf("normalized entity = %+v", second)
	}
	if !strings.Contains(second.Evidence, "Updated evidence") {
		t.Fatalf("evidence was not updated: %+v", second)
	}

	results, err := store.SearchEntities(ctx, "motor torque", SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("SearchEntities() error = %v", err)
	}
	if len(results) != 1 || results[0].ID != first.ID {
		t.Fatalf("SearchEntities() = %+v, want entity %q", results, first.ID)
	}

	recent, err := store.SearchEntities(ctx, "", SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("SearchEntities(empty) error = %v", err)
	}
	if len(recent) != 1 || recent[0].ID != first.ID {
		t.Fatalf("SearchEntities(empty) = %+v, want recent entity %q", recent, first.ID)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.Entities != 1 || stats.Edges != 0 {
		t.Fatalf("Stats() = %+v, want 1 entity and 0 edges", stats)
	}
}

func TestStoreAddsEdgesAndListsRelations(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	motor, err := store.UpsertEntity(ctx, EntityInput{Name: "Brushless Motor", Kind: "component"})
	if err != nil {
		t.Fatalf("UpsertEntity(motor) error = %v", err)
	}
	torque, err := store.UpsertEntity(ctx, EntityInput{Name: "Cogging Torque", Kind: "concept"})
	if err != nil {
		t.Fatalf("UpsertEntity(torque) error = %v", err)
	}
	edge, err := store.AddEdge(ctx, EdgeInput{
		FromEntityID: motor.ID,
		Relation:     "has issue",
		ToEntityID:   torque.ID,
		Source:       "manual",
		Evidence:     "The notes mention cogging torque as a motor behavior.",
	})
	if err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	updated, err := store.AddEdge(ctx, EdgeInput{
		FromEntityID: motor.ID,
		Relation:     "has issue",
		ToEntityID:   torque.ID,
		Source:       "manual",
		Evidence:     "Updated relationship evidence.",
	})
	if err != nil {
		t.Fatalf("AddEdge() update error = %v", err)
	}
	if edge.ID != updated.ID {
		t.Fatalf("duplicate edge ID = %q, want %q", updated.ID, edge.ID)
	}
	if updated.Relation != "has_issue" {
		t.Fatalf("Relation = %q, want has_issue", updated.Relation)
	}

	edges, err := store.ListEdgesForEntity(ctx, motor.ID, 10)
	if err != nil {
		t.Fatalf("ListEdgesForEntity() error = %v", err)
	}
	if len(edges) != 1 || edges[0].ID != edge.ID || !strings.Contains(edges[0].Evidence, "Updated") {
		t.Fatalf("edges = %+v, want updated edge %q", edges, edge.ID)
	}

	stats, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats() error = %v", err)
	}
	if stats.Entities != 2 || stats.Edges != 1 {
		t.Fatalf("Stats() = %+v, want 2 entities and 1 edge", stats)
	}
}

func TestStoreBoundsAndSanitizesEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := OpenWithOptions(ctx, filepath.Join(t.TempDir(), "memory.sqlite"), Options{MaxEvidenceChars: 80})
	if err != nil {
		t.Fatalf("OpenWithOptions() error = %v", err)
	}
	defer store.Close()

	entity, err := store.UpsertEntity(ctx, EntityInput{
		Name:     "Private Note",
		Kind:     "note",
		Evidence: "token=super-secret-value from /Users/example/Documents/private-notes and lots of extra detail that should be trimmed",
	})
	if err != nil {
		t.Fatalf("UpsertEntity() error = %v", err)
	}
	if strings.Contains(entity.Evidence, "super-secret-value") || strings.Contains(entity.Evidence, "/Users/example") {
		t.Fatalf("evidence was not sanitized: %q", entity.Evidence)
	}
	if len(entity.Evidence) > 80 {
		t.Fatalf("evidence length = %d, want <= 80", len(entity.Evidence))
	}
}

func TestStoreSanitizesCredentialPairsAndSensitiveEvidence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	entity, err := store.UpsertEntity(ctx, EntityInput{
		Name: "Credential note",
		Kind: "note",
		Evidence: strings.Join([]string{
			"alice@example.com:summer2024",
			"Authorization: Bearer abcdefghijklmnopqrstuvwxyz",
			"https://admin:secret@example.com/private",
			"api_key=abc123456789",
			"call +1 415 555 0199",
			"date 2026-06-05 should remain readable",
		}, " "),
	})
	if err != nil {
		t.Fatalf("UpsertEntity() error = %v", err)
	}
	for _, leaked := range []string{"alice@example.com", "summer2024", "abcdefghijklmnopqrstuvwxyz", "admin:secret", "abc123456789", "415 555 0199"} {
		if strings.Contains(entity.Evidence, leaked) {
			t.Fatalf("evidence leaked %q: %q", leaked, entity.Evidence)
		}
	}
	for _, marker := range []string{"[REDACTED_CREDENTIAL_PAIR]", "Bearer [REDACTED]", "[REDACTED_CREDENTIALS]", "api_key=[REDACTED]", "[REDACTED_PHONE]"} {
		if !strings.Contains(entity.Evidence, marker) {
			t.Fatalf("evidence missing marker %q: %q", marker, entity.Evidence)
		}
	}
	if !strings.Contains(entity.Evidence, "2026-06-05") {
		t.Fatalf("date-like text should not be redacted as phone: %q", entity.Evidence)
	}
}

func TestRepairEvidenceSanitizesExistingRowsAndRebuildsFTS(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	entity, err := store.UpsertEntity(ctx, EntityInput{Name: "Old Leak", Kind: "note", Evidence: "safe evidence"})
	if err != nil {
		t.Fatalf("UpsertEntity() error = %v", err)
	}
	other, err := store.UpsertEntity(ctx, EntityInput{Name: "Related Entity", Kind: "note"})
	if err != nil {
		t.Fatalf("UpsertEntity(other) error = %v", err)
	}
	edge, err := store.AddEdge(ctx, EdgeInput{FromEntityID: entity.ID, Relation: "related to", ToEntityID: other.ID, Evidence: "safe edge"})
	if err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}

	if _, err := store.db.ExecContext(ctx, `UPDATE kg_entities SET evidence = ? WHERE id = ?`, "bob@example.com:hunter2 from /Users/example/Secrets", entity.ID); err != nil {
		t.Fatalf("seed leaked entity evidence: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE kg_entity_fts SET evidence = ? WHERE entity_id = ?`, "bob@example.com:hunter2 from /Users/example/Secrets", entity.ID); err != nil {
		t.Fatalf("seed leaked entity fts: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE kg_edges SET evidence = ? WHERE id = ?`, "Bearer abcdefghijklmnopqrstuvwxyz", edge.ID); err != nil {
		t.Fatalf("seed leaked edge evidence: %v", err)
	}

	result, err := store.RepairEvidence(ctx)
	if err != nil {
		t.Fatalf("RepairEvidence() error = %v", err)
	}
	if result.EntitiesScanned != 2 || result.EntitiesUpdated != 1 || result.EdgesScanned != 1 || result.EdgesUpdated != 1 {
		t.Fatalf("RepairEvidence() = %+v", result)
	}
	repaired, err := store.GetEntity(ctx, entity.ID)
	if err != nil {
		t.Fatalf("GetEntity() error = %v", err)
	}
	for _, leaked := range []string{"bob@example.com", "hunter2", "/Users/example"} {
		if strings.Contains(repaired.Evidence, leaked) {
			t.Fatalf("repaired entity leaked %q: %q", leaked, repaired.Evidence)
		}
	}
	search, err := store.SearchEntities(ctx, "hunter2", SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("SearchEntities() error = %v", err)
	}
	if len(search) != 0 {
		t.Fatalf("repaired FTS still finds secret: %+v", search)
	}
	repairedEdge, err := store.GetEdge(ctx, edge.ID)
	if err != nil {
		t.Fatalf("GetEdge() error = %v", err)
	}
	if strings.Contains(repairedEdge.Evidence, "abcdefghijklmnopqrstuvwxyz") || !strings.Contains(repairedEdge.Evidence, "Bearer [REDACTED]") {
		t.Fatalf("repaired edge evidence = %q", repairedEdge.Evidence)
	}
}

func TestReviewBatchListsActionsAndRepairsMetadata(t *testing.T) {
	ctx := context.Background()
	store, err := OpenWithOptions(ctx, filepath.Join(t.TempDir(), "memory.sqlite"), Options{MaxEvidenceChars: 90})
	if err != nil {
		t.Fatalf("OpenWithOptions() error = %v", err)
	}
	defer store.Close()

	motor, err := store.UpsertEntity(ctx, EntityInput{
		Name:       "Brushless Motor",
		Kind:       "component",
		Source:     "review draft",
		SourceKind: "document",
		SourceRef:  "/Users/example/private/design.md token=secret-value",
		Evidence:   "Motor note evidence.",
	})
	if err != nil {
		t.Fatalf("UpsertEntity(motor) error = %v", err)
	}
	torque, err := store.UpsertEntity(ctx, EntityInput{Name: "Cogging Torque", Kind: "concept", Source: "review draft"})
	if err != nil {
		t.Fatalf("UpsertEntity(torque) error = %v", err)
	}
	edge, err := store.AddEdge(ctx, EdgeInput{
		FromEntityID: motor.ID,
		Relation:     "has issue",
		ToEntityID:   torque.ID,
		Source:       "review draft",
		SourceKind:   "rag",
		SourceRef:    "chunk_123",
		Evidence:     "Bearer abcdefghijklmnopqrstuvwxyz",
	})
	if err != nil {
		t.Fatalf("AddEdge() error = %v", err)
	}
	if motor.ReviewStatus != "pending" || torque.ReviewStatus != "pending" || edge.ReviewStatus != "pending" {
		t.Fatalf("new review statuses = entity %q %q edge %q, want pending", motor.ReviewStatus, torque.ReviewStatus, edge.ReviewStatus)
	}
	if strings.Contains(motor.SourceRef, "/Users/example") || strings.Contains(motor.SourceRef, "secret-value") {
		t.Fatalf("source ref was not sanitized: %q", motor.SourceRef)
	}

	pending, err := store.ListReviewItems(ctx, ReviewListOptions{Status: "pending", Limit: 10})
	if err != nil {
		t.Fatalf("ListReviewItems() error = %v", err)
	}
	if len(pending) != 3 {
		t.Fatalf("pending review items = %+v, want 3", pending)
	}

	result, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:     "approve",
		ReviewedBy: "qa review",
		Note:       "human checked",
		Targets:    []ReviewTarget{{Type: "entity", ID: motor.ID}},
	})
	if err != nil {
		t.Fatalf("ApplyReviewBatch(approve) error = %v", err)
	}
	if result.EntitiesMatched != 1 || result.EntitiesUpdated != 1 || len(result.Items) != 1 {
		t.Fatalf("approve result = %+v", result)
	}
	approved, err := store.GetEntity(ctx, motor.ID)
	if err != nil {
		t.Fatalf("GetEntity(approved) error = %v", err)
	}
	if approved.ReviewStatus != "approved" || approved.ReviewedBy != "qa_review" || approved.ReviewNote != "human checked" || approved.ReviewedAt == "" {
		t.Fatalf("approved review metadata = %+v", approved)
	}

	rejected, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:     "reject",
		ReviewedBy: "qa review",
		Targets:    []ReviewTarget{{Type: "edge", ID: edge.ID}},
	})
	if err != nil {
		t.Fatalf("ApplyReviewBatch(reject) error = %v", err)
	}
	if rejected.EdgesMatched != 1 || rejected.EdgesUpdated != 1 || rejected.Items[0].ReviewStatus != "rejected" {
		t.Fatalf("reject result = %+v", rejected)
	}

	if _, err := store.db.ExecContext(ctx, `UPDATE kg_entities SET evidence = ? WHERE id = ?`, "alice@example.com:hunter2 from /Users/example/Secrets", torque.ID); err != nil {
		t.Fatalf("seed leaked entity evidence: %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `UPDATE kg_entity_fts SET evidence = ? WHERE entity_id = ?`, "alice@example.com:hunter2 from /Users/example/Secrets", torque.ID); err != nil {
		t.Fatalf("seed leaked entity fts: %v", err)
	}
	repaired, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:     "repair",
		ReviewedBy: "qa review",
		Targets:    []ReviewTarget{{Type: "entity", ID: torque.ID}},
	})
	if err != nil {
		t.Fatalf("ApplyReviewBatch(repair) error = %v", err)
	}
	if repaired.EntitiesMatched != 1 || repaired.EntitiesUpdated != 1 || repaired.Items[0].ReviewStatus != "needs_repair" {
		t.Fatalf("repair result = %+v", repaired)
	}
	clean, err := store.GetEntity(ctx, torque.ID)
	if err != nil {
		t.Fatalf("GetEntity(repaired) error = %v", err)
	}
	for _, leaked := range []string{"alice@example.com", "hunter2", "/Users/example"} {
		if strings.Contains(clean.Evidence, leaked) {
			t.Fatalf("repaired evidence leaked %q: %q", leaked, clean.Evidence)
		}
	}
	search, err := store.SearchEntities(ctx, "hunter2", SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("SearchEntities() error = %v", err)
	}
	if len(search) != 0 {
		t.Fatalf("repaired FTS still finds secret: %+v", search)
	}
}

func TestReviewBatchDoesNotPromoteGraphIntoAutomaticInfluence(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	entity, err := store.UpsertEntity(ctx, EntityInput{Name: "Manual Knowledge Only", Kind: "note", Evidence: "visible only through explicit graph search"})
	if err != nil {
		t.Fatalf("UpsertEntity() error = %v", err)
	}
	before, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats(before) error = %v", err)
	}
	if _, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:  "approve",
		Targets: []ReviewTarget{{Type: "entity", ID: entity.ID}},
	}); err != nil {
		t.Fatalf("ApplyReviewBatch() error = %v", err)
	}
	after, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats(after) error = %v", err)
	}
	if before != after {
		t.Fatalf("review metadata changed graph cardinality from %+v to %+v", before, after)
	}
	results, err := store.SearchEntities(ctx, "Manual Knowledge", SearchOptions{Limit: 5})
	if err != nil {
		t.Fatalf("SearchEntities() error = %v", err)
	}
	if len(results) != 1 || results[0].ID != entity.ID || results[0].ReviewStatus != "approved" {
		t.Fatalf("explicit graph search after approval = %+v, want same row with metadata only", results)
	}
}

func TestStoreRejectsIncompleteInputs(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	if _, err := store.UpsertEntity(ctx, EntityInput{}); err == nil {
		t.Fatal("UpsertEntity() accepted empty entity")
	}
	entity, err := store.UpsertEntity(ctx, EntityInput{Name: "A", Kind: "concept"})
	if err != nil {
		t.Fatalf("UpsertEntity() error = %v", err)
	}
	if _, err := store.AddEdge(ctx, EdgeInput{FromEntityID: entity.ID, ToEntityID: entity.ID}); err == nil {
		t.Fatal("AddEdge() accepted empty relation")
	}
}
