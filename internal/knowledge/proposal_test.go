package knowledge

import (
	"strings"
	"testing"
)

func TestDraftProposalParsesExplicitReviewLines(t *testing.T) {
	proposal, err := DraftProposal(ProposalInput{
		Text: strings.Join([]string{
			"Entity: Cogging Torque | kind: Engineering Concept | source: motor notes | evidence: token=secret-value from /Users/example/Documents/design.md",
			"Concept: Brushless Motor",
			"Relationship: Brushless Motor -> has issue -> Cogging Torque | evidence: notes link the motor to the torque issue",
		}, "\n"),
		Source:     "manual QA",
		SourceKind: "rag",
		SourceRef:  "/Users/example/Documents/design.md token=secret-value",
		Limit:      5,
	})
	if err != nil {
		t.Fatalf("DraftProposal() error = %v", err)
	}
	if !proposal.ReviewRequired || proposal.Message == "" {
		t.Fatalf("proposal review metadata = %+v", proposal)
	}
	if proposal.SourceKind != "rag" || strings.Contains(proposal.SourceRef, "/Users/example") || strings.Contains(proposal.SourceRef, "secret-value") {
		t.Fatalf("proposal provenance = kind %q ref %q, want sanitized RAG provenance", proposal.SourceKind, proposal.SourceRef)
	}
	if len(proposal.EntityProposals) != 2 {
		t.Fatalf("entity proposals = %+v, want 2", proposal.EntityProposals)
	}
	first := proposal.EntityProposals[0]
	if first.Name != "Cogging Torque" || first.Kind != "engineering_concept" || first.Source != "motor_notes" {
		t.Fatalf("first entity = %+v", first)
	}
	if first.SourceKind != "rag" || first.SourceRef != proposal.SourceRef {
		t.Fatalf("first provenance = kind %q ref %q, want proposal provenance", first.SourceKind, first.SourceRef)
	}
	if strings.Contains(first.Evidence, "secret-value") || strings.Contains(first.Evidence, "/Users/example") {
		t.Fatalf("proposal evidence was not sanitized: %q", first.Evidence)
	}
	if len(proposal.RelationProposals) != 1 {
		t.Fatalf("relation proposals = %+v, want 1", proposal.RelationProposals)
	}
	relation := proposal.RelationProposals[0]
	if relation.FromName != "Brushless Motor" || relation.Relation != "has_issue" || relation.ToName != "Cogging Torque" {
		t.Fatalf("relation proposal = %+v", relation)
	}
	if relation.SourceKind != "rag" || relation.SourceRef != proposal.SourceRef {
		t.Fatalf("relation provenance = kind %q ref %q, want proposal provenance", relation.SourceKind, relation.SourceRef)
	}
}

func TestDraftProposalAllowsExplicitLineProvenanceOverride(t *testing.T) {
	proposal, err := DraftProposal(ProposalInput{
		Text: strings.Join([]string{
			"Entity: Profile Draft | kind: model profile | source_kind: workflow | source_ref: reviewed_route_governance_123",
			"Relationship: QAReview -> suggests -> Regression Test | source_kind: qa_review | source_ref: trace_abc123",
		}, "\n"),
		Source:     "memory",
		SourceKind: "memory",
		SourceRef:  "mem_default",
	})
	if err != nil {
		t.Fatalf("DraftProposal() error = %v", err)
	}
	if got := proposal.EntityProposals[0].SourceKind; got != "workflow" {
		t.Fatalf("entity source kind = %q, want workflow", got)
	}
	if got := proposal.EntityProposals[0].SourceRef; got != "reviewed_route_governance_123" {
		t.Fatalf("entity source ref = %q, want reviewed ref", got)
	}
	if got := proposal.RelationProposals[0].SourceKind; got != "qa_review" {
		t.Fatalf("relation source kind = %q, want qa_review", got)
	}
	if got := proposal.RelationProposals[0].SourceRef; got != "trace_abc123" {
		t.Fatalf("relation source ref = %q, want trace ref", got)
	}
}

func TestDraftProposalFallsBackToSingleReviewedNote(t *testing.T) {
	proposal, err := DraftProposal(ProposalInput{
		Text:        "This note explains how a quiet local agent should keep graph extraction manual.",
		Source:      "memory",
		DefaultKind: "Design Note",
	})
	if err != nil {
		t.Fatalf("DraftProposal() error = %v", err)
	}
	if len(proposal.EntityProposals) != 1 {
		t.Fatalf("entity proposals = %+v, want fallback note", proposal.EntityProposals)
	}
	entity := proposal.EntityProposals[0]
	if entity.Kind != "design_note" || entity.Source != "memory" {
		t.Fatalf("fallback entity = %+v", entity)
	}
	if !strings.Contains(entity.Name, "quiet local agent") {
		t.Fatalf("fallback name = %q", entity.Name)
	}
	if len(proposal.RelationProposals) != 0 {
		t.Fatalf("relation proposals = %+v, want none", proposal.RelationProposals)
	}
}

func TestDraftProposalRejectsEmptyText(t *testing.T) {
	if _, err := DraftProposal(ProposalInput{Text: "   "}); err == nil {
		t.Fatal("DraftProposal() accepted empty text")
	}
}
