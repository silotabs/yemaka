package contextcore

import (
	"strings"
	"testing"

	"yemaka/internal/routing"
)

func TestTaskStatePackagePreservesPendingApprovalAndSources(t *testing.T) {
	pkg := BuildTaskStatePackage(TaskStateInput{
		ActiveGoal:            "Edit the release notes.",
		ActiveRoute:           routing.RouteFileWrite,
		PendingApproval:       "edit_file",
		SelectedSources:       []string{"docs/release.md", "docs/release.md"},
		UnfinishedSteps:       []string{"show diff before write"},
		LastKnownOutcome:      "pending_approval",
		ImportantToolOutcomes: []string{"read_file=completed"},
	})
	rendered := RenderTaskStatePackage(pkg, TaskStateOptions{})
	for _, want := range []string{"active_route: file_write", "pending_approval: edit_file", "docs/release.md", "show diff before write", "last_known_outcome: pending_approval"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered task state missing %q:\n%s", want, rendered)
		}
	}
	if strings.Count(rendered, "docs/release.md") != 1 {
		t.Fatalf("rendered task state did not compact duplicate sources:\n%s", rendered)
	}
}

func TestTaskStatePackageKeepsDocumentSourceAndUserConstraint(t *testing.T) {
	pkg := BuildTaskStatePackage(TaskStateInput{
		ActiveGoal:      "Quiz me on the selected document.",
		ActiveRoute:     routing.RouteRAGSearch,
		SelectedSources: []string{"ingested:motors/cogging-torque.md"},
		UserConstraints: []string{"Only ask one practice question at a time."},
		SourceReferences: []string{
			"rag:motors/cogging-torque.md",
			"memory:old unrelated search result",
		},
	})
	rendered := RenderTaskStatePackage(pkg, TaskStateOptions{})
	for _, want := range []string{"ingested:motors/cogging-torque.md", "Only ask one practice question", "rag:motors/cogging-torque.md"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered task state missing %q:\n%s", want, rendered)
		}
	}
}

func TestTaskStatePackageAddsFreshnessGuardForCurrentNews(t *testing.T) {
	pkg := BuildTaskStatePackage(TaskStateInput{
		ActiveGoal:        "Search current AI news.",
		ActiveRoute:       routing.RouteInternetSearch,
		SourceReferences:  []string{"memory:last week's AI notes"},
		FreshnessRequired: true,
	})
	rendered := RenderTaskStatePackage(pkg, TaskStateOptions{})
	if !strings.Contains(rendered, "freshness_guard") || !strings.Contains(rendered, "must not be treated as current facts") {
		t.Fatalf("rendered task state missing freshness guard:\n%s", rendered)
	}
}

func TestTaskStatePackageStaysUnderBudgetAndRedactsSecrets(t *testing.T) {
	pkg := BuildTaskStatePackage(TaskStateInput{
		ActiveGoal:      strings.Repeat("long goal ", 100),
		ActiveRoute:     routing.RouteWorkspaceRead,
		UserConstraints: []string{"api_key=sk-" + strings.Repeat("a", 24), "password: hunter2"},
		SelectedSources: []string{strings.Repeat("very-long-source ", 80)},
	})
	rendered := RenderTaskStatePackage(pkg, TaskStateOptions{MaxBytes: 360, MaxItems: 4})
	if len(rendered) > 360 {
		t.Fatalf("rendered len = %d, want <= 360:\n%s", len(rendered), rendered)
	}
	if strings.Contains(rendered, "hunter2") || strings.Contains(rendered, "sk-"+strings.Repeat("a", 24)) {
		t.Fatalf("rendered task state leaked secret:\n%s", rendered)
	}
	if !strings.Contains(rendered, "[REDACTED]") {
		t.Fatalf("rendered task state did not show redaction marker:\n%s", rendered)
	}
}
