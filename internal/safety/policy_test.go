package safety

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPolicyInternetFetchRequiresConfirmationUnlessApproved(t *testing.T) {
	disabled := EvaluatePolicy(PolicyRequest{
		Domain: DomainInternet,
		Action: ActionFetch,
		Level:  LevelConfirm,
	})
	if disabled.Allowed || !strings.Contains(disabled.Reason, "disabled") {
		t.Fatalf("disabled decision = %+v, want blocked disabled", disabled)
	}

	ask := EvaluatePolicy(PolicyRequest{
		Domain:  DomainInternet,
		Action:  ActionFetch,
		Level:   LevelConfirm,
		Enabled: true,
	})
	if !ask.Allowed || !ask.RequiresConfirmation {
		t.Fatalf("ask decision = %+v, want allowed with confirmation", ask)
	}

	profile := EvaluatePolicy(PolicyRequest{
		Domain:         DomainInternet,
		Action:         ActionFetch,
		Level:          LevelConfirm,
		Enabled:        true,
		ProfileEnabled: true,
	})
	if !profile.Allowed || profile.RequiresConfirmation {
		t.Fatalf("profile decision = %+v, want allowed without confirmation", profile)
	}
}

func TestPolicySearchRequiresConfiguredProvider(t *testing.T) {
	decision := EvaluatePolicy(PolicyRequest{
		Domain:  DomainInternet,
		Action:  ActionSearch,
		Level:   LevelConfirm,
		Enabled: true,
	})
	if decision.Allowed || !strings.Contains(decision.Reason, "provider") {
		t.Fatalf("search decision = %+v, want provider block", decision)
	}
}

func TestPolicyBlocksGeneratedPolicyMutationAndAutonomousRisk(t *testing.T) {
	policyMutation := EvaluatePolicy(PolicyRequest{
		Domain:    DomainExtension,
		Action:    ActionModifyPolicy,
		Level:     LevelConfirm,
		Generated: true,
	})
	if policyMutation.Allowed {
		t.Fatalf("policy mutation decision = %+v, want blocked", policyMutation)
	}

	autonomousPost := EvaluatePolicy(PolicyRequest{
		Domain:  DomainSocial,
		Action:  ActionPost,
		Level:   LevelAutonomous,
		Posting: true,
	})
	if autonomousPost.Allowed || !strings.Contains(autonomousPost.Reason, "autonomously") {
		t.Fatalf("autonomous post decision = %+v, want blocked", autonomousPost)
	}
}

func TestPolicyDeploymentSocialTradingReconAndPrivacy(t *testing.T) {
	deploy := EvaluatePolicy(PolicyRequest{Domain: DomainDeployment, Action: ActionDeploy, Level: LevelConfirm})
	if !deploy.Allowed || !deploy.RequiresConfirmation {
		t.Fatalf("deploy decision = %+v, want confirmation", deploy)
	}

	social := EvaluatePolicy(PolicyRequest{Domain: DomainSocial, Action: ActionReply, Level: LevelConfirm})
	if !social.Allowed || !social.RequiresConfirmation {
		t.Fatalf("social decision = %+v, want confirmation", social)
	}

	liveTrade := EvaluatePolicy(PolicyRequest{Domain: DomainTrading, Action: ActionTrade, Level: LevelConfirm, LiveTrading: true})
	if liveTrade.Allowed || !strings.Contains(liveTrade.Reason, "live trading") {
		t.Fatalf("live trading decision = %+v, want block", liveTrade)
	}

	paperTrade := EvaluatePolicy(PolicyRequest{Domain: DomainTrading, Action: ActionTrade, Level: LevelConfirm, PaperTrading: true})
	if !paperTrade.Allowed || !paperTrade.RequiresConfirmation {
		t.Fatalf("paper trading decision = %+v, want confirmation", paperTrade)
	}

	activeRecon := EvaluatePolicy(PolicyRequest{Domain: DomainRecon, Action: ActionRecon, Level: LevelConfirm, Passive: false})
	if activeRecon.Allowed || !strings.Contains(activeRecon.Reason, "passive-only") {
		t.Fatalf("active recon decision = %+v, want block", activeRecon)
	}

	personResearch := EvaluatePolicy(PolicyRequest{Domain: DomainPrivacy, Level: LevelAutonomous, PrivacySensitive: true})
	if personResearch.Allowed || !strings.Contains(personResearch.Reason, "privacy-sensitive") {
		t.Fatalf("privacy decision = %+v, want block", personResearch)
	}
}

func TestPolicySecretsRequireConnectorApproval(t *testing.T) {
	blocked := EvaluatePolicy(PolicyRequest{Domain: DomainSecrets, Action: ActionRead, Level: LevelConfirm, Secrets: true})
	if blocked.Allowed {
		t.Fatalf("secrets decision = %+v, want blocked", blocked)
	}

	approved := EvaluatePolicy(PolicyRequest{
		Domain:            DomainSecrets,
		Action:            ActionRead,
		Level:             LevelConfirm,
		Secrets:           true,
		ConnectorApproved: true,
	})
	if !approved.Allowed || !approved.RequiresConfirmation {
		t.Fatalf("approved secrets decision = %+v, want confirmation", approved)
	}
}

func TestPolicyFullAccessModeBypassesNonGeneratedPolicyAndConfirmation(t *testing.T) {
	requests := []PolicyRequest{
		{Domain: DomainInternet, Action: ActionFetch, Level: LevelAutonomous, Network: true},
		{Domain: DomainSocial, Action: ActionPost, Level: LevelAutonomous, Posting: true},
		{Domain: DomainTrading, Action: ActionTrade, Level: LevelAutonomous, LiveTrading: true},
	}
	for _, request := range requests {
		request.PolicyMode = PolicyModeFullAccess
		decision := EvaluatePolicy(request)
		if !decision.Allowed || decision.RequiresConfirmation {
			t.Fatalf("full access decision for %+v = %+v, want allowed without confirmation", request, decision)
		}
		if !strings.Contains(decision.Reason, "full access") {
			t.Fatalf("full access reason = %q, want explicit reason", decision.Reason)
		}
	}
}

func TestPolicyFullAccessModeDoesNotBypassGeneratedGuardrails(t *testing.T) {
	requests := []struct {
		name string
		req  PolicyRequest
		want string
	}{
		{
			name: "policy mutation",
			req: PolicyRequest{
				Domain:     DomainExtension,
				Action:     ActionModifyPolicy,
				Level:      LevelAutonomous,
				PolicyMode: PolicyModeFullAccess,
				Generated:  true,
			},
			want: "policy",
		},
		{
			name: "secrets",
			req: PolicyRequest{
				Domain:     DomainSecrets,
				Action:     ActionRead,
				Level:      LevelConfirm,
				PolicyMode: PolicyModeFullAccess,
				Generated:  true,
				Secrets:    true,
			},
			want: "secrets",
		},
		{
			name: "autonomous file write",
			req: PolicyRequest{
				Domain:     DomainFilesystem,
				Action:     ActionWrite,
				Level:      LevelAutonomous,
				PolicyMode: PolicyModeFullAccess,
				Generated:  true,
				Mutating:   true,
				FileWrite:  true,
			},
			want: "file writes",
		},
	}
	for _, tc := range requests {
		t.Run(tc.name, func(t *testing.T) {
			decision := EvaluatePolicy(tc.req)
			if decision.Allowed || !strings.Contains(decision.Reason, tc.want) {
				t.Fatalf("decision = %+v, want blocked containing %q", decision, tc.want)
			}
		})
	}

	confirmedWrite := EvaluatePolicy(PolicyRequest{
		Domain:       DomainFilesystem,
		Action:       ActionWrite,
		Level:        LevelConfirm,
		PolicyMode:   PolicyModeFullAccess,
		Generated:    true,
		TaskApproved: true,
		Mutating:     true,
		FileWrite:    true,
	})
	if !confirmedWrite.Allowed || !confirmedWrite.RequiresConfirmation {
		t.Fatalf("generated confirmed write = %+v, want confirmation with snapshot/diff/rollback guardrail", confirmedWrite)
	}
	if !strings.Contains(confirmedWrite.Reason, "snapshot") {
		t.Fatalf("generated write reason = %q, want snapshot guardrail", confirmedWrite.Reason)
	}
}

func TestAppendPolicyAuditWritesJSONL(t *testing.T) {
	root := t.TempDir()
	request := PolicyRequest{Domain: DomainFilesystem, Action: ActionWrite, Level: LevelConfirm, FileWrite: true}
	decision := EvaluatePolicy(request)
	if err := AppendPolicyAudit(root, request, decision); err != nil {
		t.Fatalf("AppendPolicyAudit() error = %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, PolicyAuditFile))
	if err != nil {
		t.Fatalf("read audit = %v", err)
	}
	if !strings.Contains(string(data), `"domain":"filesystem"`) || !strings.Contains(string(data), `"requiresConfirmation":true`) {
		t.Fatalf("audit = %s, want filesystem confirmation record", string(data))
	}
}

func TestRecentPolicyAuditReadsNewestFirstAndSkipsMalformedLines(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, PolicyAuditFile)
	older := `{"timestamp":"2026-06-06T10:00:00Z","request":{"domain":"connector","action":"read","actor":"connector:local_api","resource":"chat"},"decision":{"allowed":true,"requiresConfirmation":false,"level":0,"risk":"low","reason":"allowed"}}`
	newer := `{"timestamp":"2026-06-06T11:00:00Z","request":{"domain":"internet","action":"search","actor":"test","resource":"news"},"decision":{"allowed":false,"requiresConfirmation":false,"level":0,"risk":"blocked","reason":"disabled"}}`
	if err := os.WriteFile(path, []byte(older+"\nnot-json\n"+newer+"\n"), 0o644); err != nil {
		t.Fatalf("write policy audit: %v", err)
	}
	records, err := RecentPolicyAudit(root, 1)
	if err != nil {
		t.Fatalf("RecentPolicyAudit() error = %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("RecentPolicyAudit() returned %d records, want 1", len(records))
	}
	if records[0].Request.Domain != DomainInternet || records[0].Request.Resource != "news" {
		t.Fatalf("latest record = %+v, want internet news record", records[0])
	}
	if records[0].ID == "" || records[0].Line != 3 {
		t.Fatalf("latest record readback id/line = %q/%d, want populated id and source line 3", records[0].ID, records[0].Line)
	}
}
