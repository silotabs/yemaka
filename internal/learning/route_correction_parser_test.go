package learning

import (
	"strings"
	"testing"

	"yemaka/internal/routing"
)

func TestProposeRouteCorrectionInfersCommonNaturalLanguageCorrections(t *testing.T) {
	cases := []struct {
		name          string
		message       string
		previous      string
		wantRoute     string
		wantPattern   string
		wantTools     []string
		wantForbidden []string
		wantTags      []string
	}{
		{
			name:          "local documents instead of internet",
			message:       "No, I meant local ingested documents, not internet. Remember that.",
			previous:      "search @gmail.com from indexed document",
			wantRoute:     routing.RouteRAGSearch,
			wantPattern:   "indexed document",
			wantTools:     []string{"rag_search"},
			wantForbidden: []string{"internet_search", "internet_fetch"},
			wantTags:      []string{"source:local_documents", "intent:avoid_tool", "route:rag_search"},
		},
		{
			name:          "pasted svg should not fetch namespace",
			message:       "Don't fetch URLs inside pasted SVG/code; just explain and improve it.",
			previous:      `Make better: <svg xmlns="http://www.w3.org/2000/svg"></svg>`,
			wantRoute:     routing.RouteChatExplanation,
			wantPattern:   "pasted SVG or code",
			wantForbidden: []string{"internet_search", "internet_fetch"},
			wantTags:      []string{"source:pasted_svg", "source:pasted_code", "intent:explain_only", "route:chat_explanation"},
		},
		{
			name:        "current updates need web search",
			message:     "When I ask latest/current updates, use web search or say search is required.",
			wantRoute:   routing.RouteInternetSearch,
			wantPattern: "current updates",
			wantTools:   []string{"internet_search"},
			wantTags:    []string{"source:internet", "intent:freshness_required", "route:internet_search"},
		},
		{
			name:        "memory search next time",
			message:     "That was the wrong route. Use memory search next time.",
			previous:    "What did I say about the Phoenix target?",
			wantRoute:   routing.RouteMemorySearch,
			wantPattern: "What did I say about the Phoenix target?",
			wantTools:   []string{"memory_search"},
			wantTags:    []string{"source:memory", "route:memory_search"},
		},
		{
			name:          "workspace files not web",
			message:       "Use the repo files, not web.",
			previous:      "Where is route correction implemented?",
			wantRoute:     routing.RouteWorkspaceRead,
			wantPattern:   "repo files",
			wantTools:     []string{"search_files"},
			wantForbidden: []string{"internet_search", "internet_fetch"},
			wantTags:      []string{"source:workspace", "route:workspace_read"},
		},
		{
			name:        "ask followup when target unclear",
			message:     "This should ask a follow-up, not run a tool.",
			previous:    "check it",
			wantRoute:   routing.RouteClarify,
			wantPattern: "unclear target",
			wantTags:    []string{"intent:clarify_before_acting", "intent:do_not_guess", "route:clarify"},
		},
		{
			name:        "rag is accepted as nontechnical local-doc correction",
			message:     "For prompts like this, use RAG.",
			previous:    "find the owner email in indexed document",
			wantRoute:   routing.RouteRAGSearch,
			wantPattern: "indexed document",
			wantTools:   []string{"rag_search"},
			wantTags:    []string{"source:local_documents", "route:rag_search"},
		},
		{
			name:          "explicit next-time local documents instead of web",
			message:       "next time when I ask this, use local documents instead of web",
			wantRoute:     routing.RouteRAGSearch,
			wantPattern:   "local documents",
			wantTools:     []string{"rag_search"},
			wantForbidden: []string{"internet_search", "internet_fetch"},
			wantTags:      []string{"source:local_documents", "intent:avoid_tool", "route:rag_search"},
		},
		{
			name:        "explicit workspace instead of memory preference",
			message:     "when I ask about this project, use workspace files instead of memory",
			wantRoute:   routing.RouteWorkspaceRead,
			wantPattern: "workspace files",
			wantTools:   []string{"search_files"},
			wantTags:    []string{"source:workspace", "route:workspace_read"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proposal, ok := ProposeRouteCorrection(RouteCorrectionProposalInput{
				Message:            tc.message,
				PreviousUserPrompt: tc.previous,
			})
			if !ok {
				t.Fatal("ProposeRouteCorrection() ok = false, want true")
			}
			if proposal.NeedsClarification {
				t.Fatalf("NeedsClarification = true, want false; proposal=%+v", proposal)
			}
			got := proposal.Correction
			if got.ApprovalStatus != routing.RouteCorrectionStatusPending {
				t.Fatalf("ApprovalStatus = %q, want pending", got.ApprovalStatus)
			}
			if got.IntendedRouteCategory != tc.wantRoute {
				t.Fatalf("IntendedRouteCategory = %q, want %q; correction=%+v", got.IntendedRouteCategory, tc.wantRoute, got)
			}
			if got.Pattern != tc.wantPattern {
				t.Fatalf("Pattern = %q, want %q", got.Pattern, tc.wantPattern)
			}
			assertContainsAllStrings(t, got.RequiredTools, tc.wantTools)
			assertContainsAllStrings(t, got.ForbiddenTools, tc.wantForbidden)
			assertContainsAllStrings(t, got.Tags, tc.wantTags)
			if !strings.Contains(proposal.Response, "Save this for next time?") {
				t.Fatalf("Response = %q, want approval question", proposal.Response)
			}
			if strings.Contains(proposal.Response, "rag_search") || strings.Contains(proposal.Response, "internet_fetch") {
				t.Fatalf("Response exposes tool jargon: %q", proposal.Response)
			}
		})
	}
}

func TestProposeRouteCorrectionIgnoresGenericShortCorrectionReplies(t *testing.T) {
	cases := []string{
		"That was wrong. Don't do that.",
		"no",
		"yes",
		"not that",
		"tool failed",
	}
	for _, message := range cases {
		t.Run(message, func(t *testing.T) {
			if proposal, ok := ProposeRouteCorrection(RouteCorrectionProposalInput{
				Message:            message,
				PreviousUserPrompt: "check it",
			}); ok {
				t.Fatalf("ProposeRouteCorrection() ok = true for generic reply; proposal=%+v", proposal)
			}
		})
	}
}

func TestProposeRouteCorrectionDoesNotHijackRewriteEditorialPrompts(t *testing.T) {
	cases := []string{
		"you shouldn't phrase it like that, make it better",
		"you shouldnt phrase it like that, make it better",
		"you should not say it like that, make it better",
		"you should make this paragraph better",
		"make this better: hello dear sir",
		"make this better",
		"rewrite this",
		"improve this",
		"make it professional",
		"you should make this clearer",
		"fix this paragraph",
		"polish this",
		"summarize this better",
	}
	for _, message := range cases {
		t.Run(message, func(t *testing.T) {
			if proposal, ok := ProposeRouteCorrection(RouteCorrectionProposalInput{
				Message:            message,
				PreviousUserPrompt: "Make better: hello dear sir",
			}); ok {
				t.Fatalf("ProposeRouteCorrection() ok = true for rewrite/editorial prompt; proposal=%+v", proposal)
			}
		})
	}
}

func TestProposeRouteCorrectionDoesNotHijackOrdinaryTaskPrompts(t *testing.T) {
	cases := []string{
		"Use the repo files to explain routing.go.",
		"Use local documents to answer the question.",
		"I do not know what this code does.",
		"I meant the CEO of Phoenix Group UAE.",
		"tavily-implementation.md is not in the file path /users/silo/documents",
		"The file is not listed in that folder.",
		"It's still not written, you have to create the file first and write it in.",
		"I can not find /users/silo/documents/tavily-implementation.md",
		"I can not find /users/silo/documents/tavily-implementation.md. check the folder and make sure the file was created",
		"I cannot find the file in that directory.",
	}
	for _, message := range cases {
		t.Run(message, func(t *testing.T) {
			if proposal, ok := ProposeRouteCorrection(RouteCorrectionProposalInput{Message: message}); ok {
				t.Fatalf("ProposeRouteCorrection() ok = true for ordinary task; proposal=%+v", proposal)
			}
		})
	}
}

func TestRouteCorrectionApprovalAndRejectionAreOnlyStandaloneReplies(t *testing.T) {
	if !IsRouteCorrectionApproval("yes, save it") {
		t.Fatal("IsRouteCorrectionApproval() = false, want true")
	}
	if !IsRouteCorrectionRejection("don't save") {
		t.Fatal("IsRouteCorrectionRejection() = false, want true")
	}
	if IsRouteCorrectionApproval("yes, search ingested documents instead") {
		t.Fatal("approval parser treated a correction message as approval")
	}
}

func assertContainsAllStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	for _, expected := range want {
		found := false
		for _, value := range got {
			if value == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("values = %v, want %q", got, expected)
		}
	}
}
