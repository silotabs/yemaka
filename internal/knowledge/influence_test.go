package knowledge

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInfluenceContextUsesApprovedRecordsOnly(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	approved, err := store.UpsertEntity(ctx, EntityInput{Name: "Cogging torque", Kind: "concept", Source: "manual", Evidence: "Torque ripple in electric motors."})
	if err != nil {
		t.Fatalf("UpsertEntity approved error = %v", err)
	}
	pending, err := store.UpsertEntity(ctx, EntityInput{Name: "Unreviewed motor claim", Kind: "concept", Source: "manual", Evidence: "Pending evidence should not be prompt context."})
	if err != nil {
		t.Fatalf("UpsertEntity pending error = %v", err)
	}
	if _, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:     "approve",
		ReviewedBy: "test",
		Targets:    []ReviewTarget{{Type: "entity", ID: approved.ID}},
	}); err != nil {
		t.Fatalf("ApplyReviewBatch approve error = %v", err)
	}

	result, err := store.BuildInfluenceContext(ctx, "cogging torque", InfluenceOptions{Limit: 10, MaxChars: 1200})
	if err != nil {
		t.Fatalf("BuildInfluenceContext() error = %v", err)
	}
	if !strings.Contains(result.Text, "Cogging torque") || !strings.Contains(result.Text, "Torque ripple") {
		t.Fatalf("influence text missing approved entity:\n%s", result.Text)
	}
	if strings.Contains(result.Text, "Unreviewed") || strings.Contains(result.Text, pending.ID) {
		t.Fatalf("pending entity leaked into influence text:\n%s", result.Text)
	}
	if len(result.Sources) != 1 || result.Sources[0] != "kg:entity:"+approved.ID {
		t.Fatalf("Sources = %v, want approved entity source", result.Sources)
	}
}

func TestBuildInfluenceContextIncludesOnlyApprovedEdges(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	left, err := store.UpsertEntity(ctx, EntityInput{Name: "Rotor teeth", Kind: "component", Source: "manual", Evidence: "Rotor geometry affects torque."})
	if err != nil {
		t.Fatalf("UpsertEntity left error = %v", err)
	}
	right, err := store.UpsertEntity(ctx, EntityInput{Name: "Cogging torque", Kind: "concept", Source: "manual", Evidence: "A motor design concern."})
	if err != nil {
		t.Fatalf("UpsertEntity right error = %v", err)
	}
	edge, err := store.AddEdge(ctx, EdgeInput{FromEntityID: left.ID, Relation: "influences", ToEntityID: right.ID, Source: "manual", Evidence: "Teeth alignment changes torque ripple."})
	if err != nil {
		t.Fatalf("AddEdge error = %v", err)
	}
	if _, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:     "approve",
		ReviewedBy: "test",
		Targets: []ReviewTarget{
			{Type: "entity", ID: left.ID},
			{Type: "entity", ID: right.ID},
			{Type: "edge", ID: edge.ID},
		},
	}); err != nil {
		t.Fatalf("ApplyReviewBatch approve error = %v", err)
	}

	result, err := store.BuildInfluenceContext(ctx, "rotor", InfluenceOptions{Limit: 4, MaxChars: 1200})
	if err != nil {
		t.Fatalf("BuildInfluenceContext() error = %v", err)
	}
	if !strings.Contains(result.Text, "Rotor teeth --influences--> Cogging torque") {
		t.Fatalf("approved edge missing from influence text:\n%s", result.Text)
	}
	if !contains(result.Sources, "kg:edge:"+edge.ID) {
		t.Fatalf("Sources = %v, want approved edge source", result.Sources)
	}
}

func TestBuildInfluenceContextBoundsOutput(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	entity, err := store.UpsertEntity(ctx, EntityInput{Name: "Bounded note", Kind: "concept", Source: "manual", Evidence: strings.Repeat("evidence ", 100)})
	if err != nil {
		t.Fatalf("UpsertEntity error = %v", err)
	}
	if _, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:     "approve",
		ReviewedBy: "test",
		Targets:    []ReviewTarget{{Type: "entity", ID: entity.ID}},
	}); err != nil {
		t.Fatalf("ApplyReviewBatch approve error = %v", err)
	}

	result, err := store.BuildInfluenceContext(ctx, "bounded", InfluenceOptions{Limit: 1, MaxChars: 140})
	if err != nil {
		t.Fatalf("BuildInfluenceContext() error = %v", err)
	}
	if len(result.Text) > 140 {
		t.Fatalf("influence text len = %d, want <= 140", len(result.Text))
	}
}

func TestBuildInfluenceContextRequiresSignalQuery(t *testing.T) {
	ctx := context.Background()
	store, err := Open(ctx, filepath.Join(t.TempDir(), "memory.sqlite"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()

	entity, err := store.UpsertEntity(ctx, EntityInput{Name: "Unrelated approved note", Kind: "concept", Source: "manual", Evidence: "Should not be returned for empty queries."})
	if err != nil {
		t.Fatalf("UpsertEntity error = %v", err)
	}
	if _, err := store.ApplyReviewBatch(ctx, ReviewBatchInput{
		Action:     "approve",
		ReviewedBy: "test",
		Targets:    []ReviewTarget{{Type: "entity", ID: entity.ID}},
	}); err != nil {
		t.Fatalf("ApplyReviewBatch approve error = %v", err)
	}

	result, err := store.BuildInfluenceContext(ctx, "what is the", InfluenceOptions{Limit: 4, MaxChars: 1200})
	if err != nil {
		t.Fatalf("BuildInfluenceContext() error = %v", err)
	}
	if strings.TrimSpace(result.Text) != "" || len(result.Sources) != 0 || result.EntityCount != 0 {
		t.Fatalf("empty-signal query returned influence context: %+v", result)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
