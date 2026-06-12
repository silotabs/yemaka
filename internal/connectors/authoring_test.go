package connectors

import (
	"strings"
	"testing"
	"time"

	"yemaka/internal/config"
	"yemaka/internal/safety"
	"yemaka/internal/secrets"
)

func TestGeneratedConnectorInstallRequiresApprovalAndPassingTests(t *testing.T) {
	store := testGeneratedStore(t)
	input := validGeneratedInstallInput()
	input.Approved = false
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "approval") {
		t.Fatalf("Install(unapproved) error = %v, want approval error", err)
	}

	input = validGeneratedInstallInput()
	input.Tests.Status = "failed"
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "tests must pass") {
		t.Fatalf("Install(failed tests) error = %v, want tests error", err)
	}
}

func TestGeneratedConnectorInstallStoresDisabledCandidateOnly(t *testing.T) {
	store := testGeneratedStore(t)
	result, err := store.Install(validGeneratedInstallInput())
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	if !result.Connector.Installed || !result.Connector.Approved {
		t.Fatalf("installed connector = %+v, want installed approved entry", result.Connector)
	}
	if result.Connector.Enabled {
		t.Fatal("generated connector was enabled on install")
	}
	if Enabled(config.Default().Connectors, result.Connector.Name) {
		t.Fatal("runtime Enabled() returned true for generated connector")
	}

	entries, err := store.Registry(config.Default().Connectors)
	if err != nil {
		t.Fatalf("Registry() error = %v", err)
	}
	entry := findRegistryEntry(entries, result.Connector.Name)
	if entry == nil {
		t.Fatalf("generated connector %q missing from registry", result.Connector.Name)
	}
	if entry.Status.Enabled {
		t.Fatalf("registry status = %+v, want disabled", entry.Status)
	}
	if entry.Status.Health.Status != "disabled" {
		t.Fatalf("health = %+v, want disabled", entry.Status.Health)
	}
	if entry.Manifest.Auth.Name != "YEMAKA_GITHUB_CONNECTOR_TOKEN" {
		t.Fatalf("auth ref = %+v, want env reference name", entry.Manifest.Auth)
	}
}

func TestGeneratedConnectorEnablementRequiresSecretReadiness(t *testing.T) {
	store := testGeneratedStore(t)
	installed, err := store.Install(validGeneratedInstallInput())
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	result, err := store.SetEnabled(installed.Connector.Name, true)
	if err == nil || !strings.Contains(err.Error(), "secret is not ready") {
		t.Fatalf("SetEnabled(true) error = %v, want secret readiness error", err)
	}
	if result.Connector.Enabled {
		t.Fatalf("blocked enable result = %+v, want disabled", result.Connector)
	}
	if result.Connector.ValidationError == "" {
		t.Fatalf("blocked enable result = %+v, want validation error", result.Connector)
	}
	entries, err := store.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(entries) != 1 || entries[0].Enabled {
		t.Fatalf("entries after blocked enable = %+v, want disabled", entries)
	}
}

func TestGeneratedConnectorEnablementReadinessGate(t *testing.T) {
	t.Setenv("YEMAKA_GITHUB_CONNECTOR_TOKEN", "ready")
	store := testGeneratedStore(t)
	installed, err := store.Install(validGeneratedInstallInput())
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	result, err := store.SetEnabled(installed.Connector.Name, true)
	if err != nil {
		t.Fatalf("SetEnabled(true) error = %v", err)
	}
	if !result.Connector.Enabled {
		t.Fatalf("SetEnabled(true) result = %+v, want enabled readiness", result.Connector)
	}
	if result.Message == "" || !strings.Contains(result.Message, "runs remain token-, policy-, rate-, and extension-gated") {
		t.Fatalf("SetEnabled(true) message = %q, want gated execution notice", result.Message)
	}
	entries, err := store.Registry(config.Default().Connectors)
	if err != nil {
		t.Fatalf("Registry() error = %v", err)
	}
	entry := findRegistryEntry(entries, installed.Connector.Name)
	if entry == nil {
		t.Fatalf("generated connector %q missing from registry", installed.Connector.Name)
	}
	if !entry.Status.Enabled {
		t.Fatalf("registry status = %+v, want readiness enabled", entry.Status)
	}
	if entry.Status.Health.Status != "warning" || !strings.Contains(entry.Status.Health.Detail, "gates remain enforced") {
		t.Fatalf("registry health = %+v, want gated runtime warning", entry.Status.Health)
	}
}

func TestGeneratedConnectorInstallRejectsSecretValuesAndBuiltInConflicts(t *testing.T) {
	store := testGeneratedStore(t)
	input := validGeneratedInstallInput()
	input.Manifest.Auth = secrets.Reference{Provider: secrets.ProviderEnv, Name: "sk-real-secret-value"}
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "secret value") {
		t.Fatalf("Install(secret auth) error = %v, want secret value error", err)
	}

	input = validGeneratedInstallInput()
	input.Manifest.Name = LocalAPI
	input.Manifest.Endpoint = "extension:" + LocalAPI
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("Install(built-in conflict) error = %v, want conflict", err)
	}
}

func TestGeneratedConnectorInstallRejectsLocalDomainsAndEndpointMismatch(t *testing.T) {
	store := testGeneratedStore(t)
	input := validGeneratedInstallInput()
	input.Manifest.AllowedDomains = []string{"localhost"}
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "private/local network names") {
		t.Fatalf("Install(local domain) error = %v, want local network name block", err)
	}

	input = validGeneratedInstallInput()
	input.Manifest.AllowedDomains = []string{"127.0.0.1"}
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "private/local network addresses") {
		t.Fatalf("Install(local address) error = %v, want local address block", err)
	}

	input = validGeneratedInstallInput()
	input.Manifest.Endpoint = "extension:other_bridge"
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "must match connector name") {
		t.Fatalf("Install(endpoint mismatch) error = %v, want endpoint mismatch block", err)
	}
}

func TestGeneratedConnectorInstallKeepsPolicyGates(t *testing.T) {
	store := testGeneratedStore(t)
	input := validGeneratedInstallInput()
	input.Manifest.Permissions.Posting = true
	input.Manifest.RequiresApproval = false
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "generated connectors cannot request") {
		t.Fatalf("Install(posting without manifest approval) error = %v, want generated permission block", err)
	}

	input = validGeneratedInstallInput()
	input.Manifest.Permissions.Trading = true
	input.Manifest.RequiresApproval = true
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "generated connectors cannot request") {
		t.Fatalf("Install(trading) error = %v, want generated permission block", err)
	}
}

func TestGeneratedConnectorInstallBlocksProtectedPermissionsEvenInFullAccess(t *testing.T) {
	store := testGeneratedStore(t)
	store.PolicyMode = safety.PolicyModeFullAccess
	input := validGeneratedInstallInput()
	input.Manifest.Permissions.Trading = true
	input.Manifest.RequiresApproval = true
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "generated connectors cannot request") {
		t.Fatalf("Install(trading full access) error = %v, want generated permission block", err)
	}

	input = validGeneratedInstallInput()
	input.Manifest.Permissions.Deployment = true
	if _, err := store.Install(input); err == nil || !strings.Contains(err.Error(), "generated connectors cannot request") {
		t.Fatalf("Install(deployment full access) error = %v, want generated permission block", err)
	}
}

func TestGeneratedConnectorAuditRecordsInstallAndEnableDecisions(t *testing.T) {
	store := testGeneratedStore(t)
	installed, err := store.Install(validGeneratedInstallInput())
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}
	_, _ = store.SetEnabled(installed.Connector.Name, true)
	records, err := store.RecentAudit(5)
	if err != nil {
		t.Fatalf("RecentAudit() error = %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("audit records = %+v, want install and enable block", records)
	}
	if records[0].Action != "enable" || records[0].Status != "blocked" {
		t.Fatalf("latest audit = %+v, want blocked enable", records[0])
	}
	if records[1].Action != "install" || records[1].Status != "ok" {
		t.Fatalf("previous audit = %+v, want ok install", records[1])
	}

	t.Setenv("YEMAKA_GITHUB_CONNECTOR_TOKEN", "ready")
	if _, err := store.SetEnabled(installed.Connector.Name, true); err != nil {
		t.Fatalf("SetEnabled(true ready) error = %v", err)
	}
	records, err = store.RecentAudit(5)
	if err != nil {
		t.Fatalf("RecentAudit() after ready enable error = %v", err)
	}
	if records[0].Action != "enable" || records[0].Status != "ok" {
		t.Fatalf("latest audit = %+v, want ok enable", records[0])
	}
}

func testGeneratedStore(t *testing.T) *GeneratedStore {
	t.Helper()
	store := NewGeneratedStore(t.TempDir())
	store.Now = func() time.Time {
		return time.Date(2026, 6, 7, 12, 0, 0, 0, time.UTC)
	}
	return store
}

func validGeneratedInstallInput() GeneratedInstallInput {
	return GeneratedInstallInput{
		Manifest: validGeneratedConnectorManifest(),
		Tests: TestReport{
			Status:   "passed",
			Commands: []string{"go test ./..."},
			Summary:  "manifest validation and policy tests passed",
		},
		Approved: true,
	}
}

func findRegistryEntry(entries []RegistryEntry, name string) *RegistryEntry {
	for i := range entries {
		if entries[i].Status.Name == name {
			return &entries[i]
		}
	}
	return nil
}
