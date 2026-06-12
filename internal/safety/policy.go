package safety

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PermissionLevel int

const (
	LevelReadOnly PermissionLevel = iota
	LevelDraft
	LevelConfirm
	LevelScheduled
	LevelAutonomous
)

const (
	DomainGeneral    = "general"
	DomainInternet   = "internet"
	DomainConnector  = "connector"
	DomainAutomation = "automation"
	DomainSocial     = "social"
	DomainTrading    = "trading"
	DomainPrivacy    = "privacy"
	DomainRecon      = "recon"
	DomainDeployment = "deployment"
	DomainExtension  = "extension"
	DomainFilesystem = "filesystem"
	DomainSecrets    = "secrets"

	ActionRead         = "read"
	ActionWrite        = "write"
	ActionFetch        = "fetch"
	ActionSearch       = "search"
	ActionPost         = "post"
	ActionReply        = "reply"
	ActionTrade        = "trade"
	ActionDeploy       = "deploy"
	ActionSchedule     = "schedule"
	ActionRun          = "run"
	ActionGenerate     = "generate"
	ActionRecon        = "recon"
	ActionModifyPolicy = "modify_policy"

	PolicyRiskLow     = "low"
	PolicyRiskMedium  = "medium"
	PolicyRiskHigh    = "high"
	PolicyRiskBlocked = "blocked"

	PolicyModeSafe       = "safe"
	PolicyModeFullAccess = "full_access"

	PolicyAuditFile = "policy_audit.jsonl"
)

type PolicyRequest struct {
	Domain             string          `json:"domain"`
	Action             string          `json:"action"`
	Level              PermissionLevel `json:"level"`
	PolicyMode         string          `json:"policyMode,omitempty"`
	Actor              string          `json:"actor,omitempty"`
	Resource           string          `json:"resource,omitempty"`
	Enabled            bool            `json:"enabled,omitempty"`
	ProfileEnabled     bool            `json:"profileEnabled,omitempty"`
	TaskApproved       bool            `json:"taskApproved,omitempty"`
	Scheduled          bool            `json:"scheduled,omitempty"`
	Generated          bool            `json:"generated,omitempty"`
	ProviderConfigured bool            `json:"providerConfigured,omitempty"`
	ConnectorApproved  bool            `json:"connectorApproved,omitempty"`
	Mutating           bool            `json:"mutating,omitempty"`
	Posting            bool            `json:"posting,omitempty"`
	Trading            bool            `json:"trading,omitempty"`
	LiveTrading        bool            `json:"liveTrading,omitempty"`
	PaperTrading       bool            `json:"paperTrading,omitempty"`
	Deployment         bool            `json:"deployment,omitempty"`
	PrivacySensitive   bool            `json:"privacySensitive,omitempty"`
	Passive            bool            `json:"passive,omitempty"`
	Network            bool            `json:"network,omitempty"`
	Secrets            bool            `json:"secrets,omitempty"`
	FileWrite          bool            `json:"fileWrite,omitempty"`
	Domains            []string        `json:"domains,omitempty"`
}

type PolicyDecision struct {
	Allowed              bool            `json:"allowed"`
	RequiresConfirmation bool            `json:"requiresConfirmation"`
	Level                PermissionLevel `json:"level"`
	Risk                 string          `json:"risk"`
	Reason               string          `json:"reason"`
	Explanation          string          `json:"explanation"`
}

type PolicyAuditRecord struct {
	ID        string         `json:"id,omitempty"`
	Line      int            `json:"line,omitempty"`
	Timestamp string         `json:"timestamp"`
	Request   PolicyRequest  `json:"request"`
	Decision  PolicyDecision `json:"decision"`
}

func (level PermissionLevel) String() string {
	switch level {
	case LevelReadOnly:
		return "read-only"
	case LevelDraft:
		return "draft"
	case LevelConfirm:
		return "confirm"
	case LevelScheduled:
		return "scheduled"
	case LevelAutonomous:
		return "autonomous"
	default:
		return fmt.Sprintf("unknown(%d)", int(level))
	}
}

func EvaluatePolicy(request PolicyRequest) PolicyDecision {
	request = normalizePolicyRequest(request)
	if request.Level < LevelReadOnly || request.Level > LevelAutonomous {
		return blocked(request, "permission level is outside the supported 0-4 range")
	}
	if request.Generated {
		request.PolicyMode = PolicyModeSafe
	} else if request.PolicyMode == PolicyModeFullAccess {
		return fullAccessAllowed(request)
	}
	if modifiesPolicy(request) {
		return blocked(request, "generated extensions and connectors cannot modify or lower core policy")
	}
	if request.Secrets || request.Domain == DomainSecrets {
		return evaluateSecretsPolicy(request)
	}
	if request.Trading || request.Domain == DomainTrading || request.Action == ActionTrade {
		return evaluateTradingPolicy(request)
	}
	if request.Deployment || request.Domain == DomainDeployment || request.Action == ActionDeploy {
		return evaluateDeploymentPolicy(request)
	}
	if request.Posting || request.Domain == DomainSocial || request.Action == ActionPost || request.Action == ActionReply {
		return evaluateSocialPolicy(request)
	}
	if request.PrivacySensitive || request.Domain == DomainPrivacy {
		return evaluatePrivacyPolicy(request)
	}
	if request.Domain == DomainRecon || request.Action == ActionRecon {
		return evaluateReconPolicy(request)
	}
	if request.Domain == DomainInternet || request.Action == ActionFetch || request.Action == ActionSearch {
		return evaluateInternetPolicy(request)
	}
	if request.Domain == DomainConnector {
		return evaluateConnectorPolicy(request)
	}
	if request.Domain == DomainAutomation || request.Action == ActionSchedule {
		return evaluateAutomationPolicy(request)
	}
	if request.FileWrite || request.Domain == DomainFilesystem || request.Action == ActionWrite {
		return evaluateFilesystemPolicy(request)
	}
	if request.Level == LevelAutonomous && !autonomousLowRisk(request) {
		return blocked(request, "Level 4 autonomy is limited to low-risk read-only local actions")
	}
	return allowed(request, PolicyRiskLow, "policy allows this low-risk local action")
}

func AppendPolicyAudit(logsDir string, request PolicyRequest, decision PolicyDecision) error {
	logsDir = strings.TrimSpace(logsDir)
	if logsDir == "" {
		return nil
	}
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return err
	}
	record := PolicyAuditRecord{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Request:   normalizePolicyRequest(request),
		Decision:  decision,
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(logsDir, PolicyAuditFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

func RecentPolicyAudit(logsDir string, limit int) ([]PolicyAuditRecord, error) {
	logsDir = strings.TrimSpace(logsDir)
	if logsDir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(logsDir, PolicyAuditFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	records := []PolicyAuditRecord{}
	for index := len(lines) - 1; index >= 0 && len(records) < limit; index-- {
		line := strings.TrimSpace(lines[index])
		if line == "" {
			continue
		}
		var record PolicyAuditRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			continue
		}
		if strings.TrimSpace(record.Timestamp) == "" || strings.TrimSpace(record.Request.Domain) == "" {
			continue
		}
		record.Line = index + 1
		record.ID = policyAuditRecordID(record)
		records = append(records, record)
	}
	return records, nil
}

func policyAuditRecordID(record PolicyAuditRecord) string {
	parts := []string{"policy", strconv.Itoa(record.Line), record.Timestamp, record.Request.Actor, record.Request.Resource, record.Request.Domain, record.Request.Action}
	clean := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		part = strings.Map(func(r rune) rune {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
				return r
			default:
				return '_'
			}
		}, part)
		part = strings.Trim(part, "_")
		if part != "" {
			clean = append(clean, part)
		}
	}
	return strings.ToLower(strings.Join(clean, "_"))
}

func normalizePolicyRequest(request PolicyRequest) PolicyRequest {
	request.Domain = strings.ToLower(strings.TrimSpace(request.Domain))
	request.Action = strings.ToLower(strings.TrimSpace(request.Action))
	request.PolicyMode = NormalizePolicyMode(request.PolicyMode)
	request.Actor = strings.TrimSpace(request.Actor)
	request.Resource = strings.TrimSpace(request.Resource)
	if request.Domain == "" {
		request.Domain = DomainGeneral
	}
	if request.Action == "" {
		request.Action = ActionRead
	}
	request.Domains = cleanStrings(request.Domains)
	return request
}

func NormalizePolicyMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case PolicyModeFullAccess:
		return PolicyModeFullAccess
	default:
		return PolicyModeSafe
	}
}

func IsFullAccessMode(mode string) bool {
	return NormalizePolicyMode(mode) == PolicyModeFullAccess
}

func evaluateInternetPolicy(request PolicyRequest) PolicyDecision {
	if request.Action == ActionSearch && !request.ProviderConfigured {
		return blocked(request, "internet search is disabled until a search provider is explicitly configured")
	}
	if !request.Enabled && !request.ProfileEnabled && !request.TaskApproved {
		return blocked(request, "internet access is disabled by default")
	}
	if request.Level == LevelAutonomous {
		return blocked(request, "internet access cannot run as an unreviewed Level 4 autonomous action")
	}
	if request.ProfileEnabled || request.TaskApproved {
		return allowed(request, PolicyRiskMedium, "internet access is permitted by profile or task-scoped approval")
	}
	return confirm(request, PolicyRiskMedium, "internet access requires confirmation unless profile-enabled")
}

func evaluateConnectorPolicy(request PolicyRequest) PolicyDecision {
	if !request.Enabled && !request.TaskApproved {
		return blocked(request, "connector is disabled by default")
	}
	if request.Level == LevelAutonomous && !autonomousLowRisk(request) {
		return blocked(request, "connectors cannot perform autonomous mutating or network actions")
	}
	if request.Network || request.Mutating {
		return confirm(request, PolicyRiskMedium, "connector network or mutating access requires policy approval")
	}
	return allowed(request, PolicyRiskLow, "connector is limited to low-risk inbound local access")
}

func evaluateAutomationPolicy(request PolicyRequest) PolicyDecision {
	if !request.Enabled {
		return blocked(request, "scheduler automation is disabled")
	}
	if request.Level >= LevelScheduled && !request.Scheduled {
		return confirm(request, PolicyRiskMedium, "scheduled execution requires an approved job definition")
	}
	if request.Scheduled {
		return allowed(request, PolicyRiskMedium, "approved scheduled execution is allowed within configured limits")
	}
	return confirm(request, PolicyRiskMedium, "automation requires confirmation before execution")
}

func evaluateSocialPolicy(request PolicyRequest) PolicyDecision {
	if request.Level == LevelAutonomous {
		return blocked(request, "social posting and replies cannot run autonomously")
	}
	return confirm(request, PolicyRiskHigh, "social posting and replies require explicit approval")
}

func evaluateTradingPolicy(request PolicyRequest) PolicyDecision {
	if request.LiveTrading || !request.PaperTrading {
		return blocked(request, "live trading is disabled by default; only paper or testnet trading can be considered with approval")
	}
	return confirm(request, PolicyRiskHigh, "paper or testnet trading requires explicit approval and risk limits")
}

func evaluatePrivacyPolicy(request PolicyRequest) PolicyDecision {
	if request.Level > LevelConfirm {
		return blocked(request, "privacy-sensitive research cannot run as scheduled or autonomous work")
	}
	if request.Mutating || request.Posting || request.Network {
		return confirm(request, PolicyRiskHigh, "privacy-sensitive work requires public-source limits and explicit approval")
	}
	if request.Level == LevelDraft {
		return allowed(request, PolicyRiskMedium, "privacy-sensitive work is limited to draft guidance and public information")
	}
	return confirm(request, PolicyRiskHigh, "privacy-sensitive work requires confirmation and filtering")
}

func evaluateReconPolicy(request PolicyRequest) PolicyDecision {
	if !request.Passive {
		return blocked(request, "cyber/recon tasks are passive-only; active scanning or exploit behavior is blocked")
	}
	if request.Network || request.Level >= LevelConfirm {
		return confirm(request, PolicyRiskHigh, "passive recon that touches the network requires confirmation and scope limits")
	}
	return allowed(request, PolicyRiskMedium, "passive local recon is allowed within read-only limits")
}

func evaluateDeploymentPolicy(request PolicyRequest) PolicyDecision {
	if request.Level == LevelAutonomous {
		return blocked(request, "deployment cannot run autonomously")
	}
	return confirm(request, PolicyRiskHigh, "website deployment requires explicit confirmation")
}

func evaluateSecretsPolicy(request PolicyRequest) PolicyDecision {
	if !request.ConnectorApproved {
		return blocked(request, "secrets access is blocked unless a connector-approved secret reference is used")
	}
	if request.Level == LevelAutonomous {
		return blocked(request, "secrets access cannot run autonomously")
	}
	return confirm(request, PolicyRiskHigh, "connector-approved secret access requires confirmation")
}

func evaluateFilesystemPolicy(request PolicyRequest) PolicyDecision {
	if request.FileWrite || request.Mutating || request.Action == ActionWrite {
		if request.Level == LevelAutonomous {
			return blocked(request, "file writes cannot run autonomously")
		}
		return confirm(request, PolicyRiskMedium, "file writes require snapshot, diff preview, confirmation, and rollback support")
	}
	return allowed(request, PolicyRiskLow, "read-only filesystem access is allowed within workspace policy")
}

func modifiesPolicy(request PolicyRequest) bool {
	return request.Action == ActionModifyPolicy ||
		(request.Generated && request.Domain == DomainExtension && strings.Contains(request.Resource, "policy")) ||
		(request.Generated && strings.Contains(request.Action, "policy"))
}

func autonomousLowRisk(request PolicyRequest) bool {
	return !request.Network &&
		!request.Mutating &&
		!request.Posting &&
		!request.Trading &&
		!request.LiveTrading &&
		!request.Deployment &&
		!request.PrivacySensitive &&
		!request.Secrets &&
		!request.FileWrite &&
		(request.Domain == DomainGeneral ||
			request.Domain == DomainFilesystem ||
			request.Domain == DomainConnector ||
			request.Action == ActionRead ||
			request.Action == ActionSearch)
}

func allowed(request PolicyRequest, risk string, reason string) PolicyDecision {
	return PolicyDecision{
		Allowed:     true,
		Level:       request.Level,
		Risk:        risk,
		Reason:      reason,
		Explanation: explain(request, reason),
	}
}

func confirm(request PolicyRequest, risk string, reason string) PolicyDecision {
	return PolicyDecision{
		Allowed:              true,
		RequiresConfirmation: true,
		Level:                request.Level,
		Risk:                 risk,
		Reason:               reason,
		Explanation:          explain(request, reason),
	}
}

func blocked(request PolicyRequest, reason string) PolicyDecision {
	return PolicyDecision{
		Allowed:     false,
		Level:       request.Level,
		Risk:        PolicyRiskBlocked,
		Reason:      reason,
		Explanation: explain(request, reason),
	}
}

func fullAccessAllowed(request PolicyRequest) PolicyDecision {
	reason := "full access mode allows this action without policy confirmation"
	return PolicyDecision{
		Allowed:     true,
		Level:       request.Level,
		Risk:        PolicyRiskHigh,
		Reason:      reason,
		Explanation: explain(request, reason),
	}
}

func explain(request PolicyRequest, reason string) string {
	target := request.Domain + "." + request.Action
	if request.Resource != "" {
		target += " on " + request.Resource
	}
	return fmt.Sprintf("Policy level %d (%s) for %s: %s.", int(request.Level), request.Level.String(), target, reason)
}

func cleanStrings(values []string) []string {
	result := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		result = append(result, value)
		seen[value] = true
	}
	return result
}
