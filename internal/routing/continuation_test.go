package routing

import "testing"

func TestContinuationFrameClassifiesGenericContinuationKinds(t *testing.T) {
	cases := []struct {
		name       string
		prompt     string
		prior      ContinuationPriorState
		wantKind   string
		wantSource string
		wantTarget string
	}{
		{
			name:   "retry preserves prior route target",
			prompt: "retry that",
			prior: ContinuationPriorState{
				RouteCategory: RouteInternetSearch,
				ToolName:      "internet_search",
				Target:        "prior public lookup",
				SourceOfTruth: PreflightSourceInternet,
				FailureStatus: "blocked",
			},
			wantKind:   ContinuationKindRetry,
			wantSource: PreflightSourceInternet,
		},
		{
			name:       "ordinary target correction is not route teaching",
			prompt:     "I meant PostgreSQL",
			wantKind:   ContinuationKindTargetCorrection,
			wantSource: "",
			wantTarget: "PostgreSQL",
		},
		{
			name:       "target suffix correction is not source clarification",
			prompt:     "example.com as the target",
			wantKind:   ContinuationKindTargetCorrection,
			wantSource: "",
			wantTarget: "example.com",
		},
		{
			name:       "target assignment correction is not source clarification",
			prompt:     "target is example.com",
			wantKind:   ContinuationKindTargetCorrection,
			wantSource: "",
			wantTarget: "example.com",
		},
		{
			name:       "source correction to local documents",
			prompt:     "I meant the uploaded PDF, not the web",
			wantKind:   ContinuationKindSourceCorrection,
			wantSource: PreflightSourceLocalDocuments,
		},
		{
			name:     "authorization continuation remains explicit",
			prompt:   "I authorize you to continue",
			wantKind: ContinuationKindApproval,
		},
		{
			name:     "fresh prompt remains new task",
			prompt:   "Explain what a CEO does",
			wantKind: ContinuationKindNewTask,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			frame := BuildContinuationFrame(ContinuationInput{Content: tc.prompt, Prior: tc.prior})
			if frame.Kind != tc.wantKind {
				t.Fatalf("Kind = %q, want %q; frame=%+v", frame.Kind, tc.wantKind, frame)
			}
			if tc.wantSource != "" && frame.NewSourceOfTruth != tc.wantSource && frame.PriorSourceOfTruth != tc.wantSource {
				t.Fatalf("source = new:%q prior:%q, want %q; frame=%+v", frame.NewSourceOfTruth, frame.PriorSourceOfTruth, tc.wantSource, frame)
			}
			if tc.wantTarget != "" && frame.NewTarget != tc.wantTarget {
				t.Fatalf("NewTarget = %q, want %q; frame=%+v", frame.NewTarget, tc.wantTarget, frame)
			}
		})
	}
}

func TestContinuationFrameClarifiesAmbiguousConversationTarget(t *testing.T) {
	frame := BuildContinuationFrame(ContinuationInput{
		Content: "search more about it",
		TaskMemory: "RECENT SESSION MESSAGES (conversation continuity only; not current source evidence):\n" +
			"user: Tell me about Apple.\n" +
			"assistant: Apple is one possible target.\n" +
			"user: Tell me about Microsoft.\n" +
			"assistant: Microsoft is another possible target.",
	})
	if frame.Kind != ContinuationKindClarify {
		t.Fatalf("Kind = %q, want clarify; frame=%+v", frame.Kind, frame)
	}
	if frame.TargetSource != ContinuationTargetAmbiguous {
		t.Fatalf("TargetSource = %q, want ambiguous; frame=%+v", frame.TargetSource, frame)
	}
	if !containsContinuationValue(frame.MissingSlots, "ambiguous_prior_target") {
		t.Fatalf("MissingSlots = %v, want ambiguous_prior_target", frame.MissingSlots)
	}
}
