package release

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yemaka/internal/config"
	"yemaka/internal/evaluation"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/profiles"
	"yemaka/internal/skills"
)

type fakeRuntime struct {
	models []models.ModelInfo
}

func (f fakeRuntime) Health(ctx context.Context) error {
	return nil
}

func (f fakeRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	return f.models, nil
}

func (f fakeRuntime) PullModel(ctx context.Context, name string, emit func(models.PullEvent) error) error {
	return nil
}

func (f fakeRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	return emit(models.ChatEvent{Done: true})
}

func TestRunReportsReadyWhenReleaseChecksPass(t *testing.T) {
	ctx := context.Background()
	home := t.TempDir()
	t.Setenv(config.EnvHome, home)
	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate() error = %v", err)
	}
	cfg.App.SetupComplete = true
	cfg.Models["low_memory"] = config.ModelConfig{
		Provider:    "ollama",
		Name:        "tiny:2b",
		BaseURL:     "http://localhost:11434/api",
		Temperature: 0.1,
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	store, err := memory.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer store.Close()
	registry, err := skills.LoadRegistry([]string{filepath.Join("..", "..", "skills", "default")})
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	if _, err := evaluation.Save(profile, evaluation.Report{
		ID: "eval_ready",
		Summary: evaluation.Summary{
			Passed: 8,
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	appBundle := filepath.Join(home, "Yemaka.app")
	writeTestAppBundle(t, appBundle, releasePlist("com.yemaka.agent", "13.0.0", true))
	entitlementsPath, signScriptPath := writeTestHardeningFiles(t, home, goodEntitlements())

	report := Run(ctx, Input{
		Config:           cfg,
		Profile:          profile,
		Memory:           store,
		Runtime:          fakeRuntime{models: []models.ModelInfo{{Name: "tiny:2b", Size: 1234567890}}},
		Skills:           registry,
		AppBundlePath:    appBundle,
		EntitlementsPath: entitlementsPath,
		SignScriptPath:   signScriptPath,
	})
	if !report.Ready {
		t.Fatalf("Ready = false, checks = %+v", report.Checks)
	}
	for _, check := range report.Checks {
		if check.Status == StatusFail {
			t.Fatalf("unexpected failed check: %+v", check)
		}
	}
}

func TestRunFailsWhenTelemetryEnabled(t *testing.T) {
	cfg := config.Default()
	cfg.App.Telemetry = true
	report := Run(context.Background(), Input{Config: cfg})
	if report.Ready {
		t.Fatal("Ready = true with telemetry enabled")
	}
}

func TestCheckExpansionDefaultsPassesForDefaultConfig(t *testing.T) {
	check := checkExpansionDefaults(config.Default())
	if check.Status != StatusPass {
		t.Fatalf("Status = %q, want pass; details: %s", check.Status, check.Details)
	}
}

func TestCheckExpansionDefaultsWarnsForActiveProfileInternetSearchCustomization(t *testing.T) {
	cfg := config.Default()
	cfg.Internet.Enabled = true
	cfg.Internet.Search.Enabled = true
	cfg.Internet.Search.Provider = "searxng"
	cfg.Internet.Search.Endpoint = "http://127.0.0.1:8080/search"

	check := checkExpansionDefaults(cfg)
	if check.Status != StatusWarn {
		t.Fatalf("Status = %q, want warn", check.Status)
	}
	if !strings.Contains(check.Details, "active profile") || !strings.Contains(check.Details, "clean YEMAKA_HOME") {
		t.Fatalf("Details = %q, want active profile guidance", check.Details)
	}
}

func TestCheckExpansionDefaultsRejectsRegisterBeforeRunDrift(t *testing.T) {
	cfg := config.Default()
	cfg.Extensions.RegisterOnlyIfTestsPass = false

	check := checkExpansionDefaults(cfg)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", check.Status)
	}
	if !strings.Contains(check.Details, "tests pass") {
		t.Fatalf("Details = %q, want tests pass failure", check.Details)
	}
}

func TestCheckExpansionDefaultsRejectsSchedulerAndHeartbeatDrift(t *testing.T) {
	cfg := config.Default()
	cfg.Scheduler.LowMemoryMaxParallelJobs = 2
	cfg.Heartbeat.Checks.Extensions = false

	check := checkExpansionDefaults(cfg)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", check.Status)
	}
	if !strings.Contains(check.Details, "parallel jobs") {
		t.Fatalf("Details = %q, want scheduler parallelism failure", check.Details)
	}
}

func TestCheckExpansionDefaultsRejectsEnabledConnector(t *testing.T) {
	cfg := config.Default()
	cfg.Connectors.LocalAPI.Enabled = true

	check := checkExpansionDefaults(cfg)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", check.Status)
	}
	if !strings.Contains(check.Details, "local_api connector") {
		t.Fatalf("Details = %q, want connector failure", check.Details)
	}
}

func TestCheckRAGRejectsEnabledVectorDB(t *testing.T) {
	cfg := config.Default()
	cfg.RAG.VectorDB.Enabled = true

	check := checkRAG(cfg)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "vector DB") {
		t.Fatalf("Details = %q, want vector DB failure", check.Details)
	}
}

func TestCheckModelConfigAllowsExplicitLocalRuntimeProvider(t *testing.T) {
	cfg := config.Default()
	model := cfg.Models["coding"]
	model.Provider = models.ProviderLlamaCpp
	model.Name = "tiny-local"
	model.BaseURL = "http://127.0.0.1:8080/v1"
	cfg.Models["coding"] = model

	check := checkModelConfig(cfg)
	if check.Status == StatusFail {
		t.Fatalf("Status = fail, details: %s", check.Details)
	}
}

func TestCheckModelConfigRejectsRemoteRuntimeProvider(t *testing.T) {
	cfg := config.Default()
	model := cfg.Models["coding"]
	model.Provider = models.ProviderOpenAICompatible
	model.Name = "remote"
	model.BaseURL = "https://api.example.com/v1"
	cfg.Models["coding"] = model

	check := checkModelConfig(cfg)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail", check.Status)
	}
	if !strings.Contains(check.Details, "base_url") {
		t.Fatalf("Details = %q, want base_url failure", check.Details)
	}
}

func TestCheckAppBundleRejectsGenericWailsMetadata(t *testing.T) {
	appBundle := filepath.Join(t.TempDir(), "Yemaka.app")
	writeTestAppBundle(t, appBundle, releasePlist("com.wails.Yemaka", "10.13.0", true))

	check := checkAppBundle(appBundle)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "CFBundleIdentifier") {
		t.Fatalf("Details = %q, want CFBundleIdentifier failure", check.Details)
	}
}

func TestCheckAppBundleFailsWhenExecutableMissing(t *testing.T) {
	appBundle := filepath.Join(t.TempDir(), "Yemaka.app")
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatalf("MkdirAll(MacOS) error = %v", err)
	}

	check := checkAppBundle(appBundle)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "executable") {
		t.Fatalf("Details = %q, want executable failure", check.Details)
	}
}

func TestCheckAppBundleFailsWhenExecutableIsDirectory(t *testing.T) {
	appBundle := filepath.Join(t.TempDir(), "Yemaka.app")
	writeTestAppBundle(t, appBundle, releasePlist("com.yemaka.agent", "13.0.0", true))
	if err := os.Remove(filepath.Join(appBundle, "Contents", "MacOS", "Yemaka")); err != nil {
		t.Fatalf("Remove(binary) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "MacOS", "Yemaka"), 0o755); err != nil {
		t.Fatalf("MkdirAll(binary dir) error = %v", err)
	}

	check := checkAppBundle(appBundle)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
}

func TestCheckAppBundleRequiresLocalNetworking(t *testing.T) {
	appBundle := filepath.Join(t.TempDir(), "Yemaka.app")
	writeTestAppBundle(t, appBundle, releasePlist("com.yemaka.agent", "13.0.0", false))

	check := checkAppBundle(appBundle)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "NSAllowsLocalNetworking") {
		t.Fatalf("Details = %q, want local networking failure", check.Details)
	}
}

func TestCheckHardeningConfigRejectsRiskyEntitlement(t *testing.T) {
	root := t.TempDir()
	entitlementsPath, signScriptPath := writeTestHardeningFiles(t, root, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.security.network.client</key>
  <true/>
  <key>com.apple.security.get-task-allow</key>
  <true/>
</dict>
</plist>`)

	check := checkHardeningConfig(entitlementsPath, signScriptPath)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "get-task-allow") {
		t.Fatalf("Details = %q, want risky entitlement name", check.Details)
	}
}

func TestCheckHardeningConfigRequiresRuntimeSigning(t *testing.T) {
	root := t.TempDir()
	entitlementsPath, signScriptPath := writeTestHardeningFiles(t, root, goodEntitlements())
	if err := os.WriteFile(signScriptPath, []byte("codesign --sign identity Yemaka.app\n"), 0o755); err != nil {
		t.Fatalf("WriteFile(sign script) error = %v", err)
	}

	check := checkHardeningConfig(entitlementsPath, signScriptPath)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
}

func TestCheckSandboxTemplatePassesWithRequiredEntitlements(t *testing.T) {
	root := t.TempDir()
	sandboxPath := filepath.Join(root, "entitlements.sandbox.plist")
	if err := os.WriteFile(sandboxPath, []byte(goodSandboxEntitlements()), 0o644); err != nil {
		t.Fatalf("WriteFile(sandbox entitlements) error = %v", err)
	}
	signScript := filepath.Join(root, "sign-app.sh")
	if err := os.WriteFile(signScript, []byte(`#!/bin/sh
SANDBOX="${SANDBOX:-0}"
ENTITLEMENTS="build/darwin/entitlements.sandbox.plist"
codesign --force --deep --options runtime --timestamp --entitlements "$ENTITLEMENTS" --sign "$IDENTITY" "$APP_PATH"
`), 0o755); err != nil {
		t.Fatalf("WriteFile(sign script) error = %v", err)
	}

	check := checkSandboxTemplate(sandboxPath, signScript)
	if check.Status != StatusPass {
		t.Fatalf("Status = %q, want pass; details: %s", check.Status, check.Details)
	}
}

func TestCheckSandboxTemplateRejectsMissingBookmarkEntitlement(t *testing.T) {
	root := t.TempDir()
	sandboxPath := filepath.Join(root, "entitlements.sandbox.plist")
	if err := os.WriteFile(sandboxPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.security.app-sandbox</key>
  <true/>
  <key>com.apple.security.network.client</key>
  <true/>
  <key>com.apple.security.files.user-selected.read-write</key>
  <true/>
</dict>
</plist>`), 0o644); err != nil {
		t.Fatalf("WriteFile(sandbox entitlements) error = %v", err)
	}

	check := checkSandboxTemplate(sandboxPath, "")
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
	if !strings.Contains(check.Details, "bookmarks.app-scope") {
		t.Fatalf("Details = %q, want bookmark entitlement", check.Details)
	}
}

func TestCheckNoBundledModelsRejectsModelArtifact(t *testing.T) {
	appBundle := filepath.Join(t.TempDir(), "Yemaka.app")
	writeTestAppBundle(t, appBundle, releasePlist("com.yemaka.agent", "13.0.0", true))
	modelPath := filepath.Join(appBundle, "Contents", "Resources", "models", "tiny.gguf")
	if err := os.MkdirAll(filepath.Dir(modelPath), 0o755); err != nil {
		t.Fatalf("MkdirAll(model dir) error = %v", err)
	}
	if err := os.WriteFile(modelPath, []byte("model"), 0o644); err != nil {
		t.Fatalf("WriteFile(model) error = %v", err)
	}

	check := checkNoBundledModels(appBundle)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
}

func TestCheckBundleSecretHygieneRejectsSecretFile(t *testing.T) {
	appBundle := filepath.Join(t.TempDir(), "Yemaka.app")
	writeTestAppBundle(t, appBundle, releasePlist("com.yemaka.agent", "13.0.0", true))
	secretPath := filepath.Join(appBundle, "Contents", "Resources", ".env")
	if err := os.WriteFile(secretPath, []byte("OPENAI_API_KEY=sk-testsecretvalueforbundle"), 0o644); err != nil {
		t.Fatalf("WriteFile(secret) error = %v", err)
	}

	check := checkBundleSecretHygiene(appBundle)
	if check.Status != StatusFail {
		t.Fatalf("Status = %q, want fail; details: %s", check.Status, check.Details)
	}
}

func TestLocalInstalledModelsKeepsUnknownSizeLocalModels(t *testing.T) {
	local := localInstalledModels([]models.ModelInfo{
		{Name: "local:2b", Size: 0},
		{Name: "remote:cloud", Size: 0},
	})
	if len(local) != 1 {
		t.Fatalf("local count = %d, want 1", len(local))
	}
	if !modelInstalled("local:2b", local) {
		t.Fatal("expected local:2b to be detected as installed")
	}
}

func TestCheckDistributionArtifactsWarnsWithoutDMG(t *testing.T) {
	check := checkDistributionArtifacts(t.TempDir())
	if check.Status != StatusWarn {
		t.Fatalf("Status = %q, want warn", check.Status)
	}
	if !strings.Contains(check.Details, "no DMG") {
		t.Fatalf("Details = %q, want no DMG warning", check.Details)
	}
}

func TestCheckDistributionArtifactsPassesWithManifestAndChecksum(t *testing.T) {
	dir := t.TempDir()
	dmg := filepath.Join(dir, "Yemaka-1.0-rc-macos-arm64.dmg")
	if err := os.WriteFile(dmg, []byte("dmg"), 0o644); err != nil {
		t.Fatalf("WriteFile(dmg) error = %v", err)
	}
	if err := os.WriteFile(dmg+".sha256", []byte("checksum"), 0o644); err != nil {
		t.Fatalf("WriteFile(sha256) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"notarization_status":"not_submitted"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	check := checkDistributionArtifacts(dir)
	if check.Status != StatusPass {
		t.Fatalf("Status = %q, want pass; details: %s", check.Status, check.Details)
	}
}

func TestCheckNotarizationStatusReportsAccepted(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(`{"notarization_status":"accepted"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(manifest) error = %v", err)
	}
	check := checkNotarizationStatus(dir)
	if check.Status != StatusPass {
		t.Fatalf("Status = %q, want pass; details: %s", check.Status, check.Details)
	}
}

func goodEntitlements() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.security.network.client</key>
  <true/>
</dict>
</plist>`
}

func goodSandboxEntitlements() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>com.apple.security.app-sandbox</key>
  <true/>
  <key>com.apple.security.network.client</key>
  <true/>
  <key>com.apple.security.files.user-selected.read-write</key>
  <true/>
  <key>com.apple.security.files.bookmarks.app-scope</key>
  <true/>
</dict>
</plist>`
}

func writeTestHardeningFiles(t *testing.T, root string, entitlements string) (string, string) {
	t.Helper()
	entitlementsPath := filepath.Join(root, "entitlements.plist")
	signScriptPath := filepath.Join(root, "sign-app.sh")
	if err := os.WriteFile(entitlementsPath, []byte(entitlements), 0o644); err != nil {
		t.Fatalf("WriteFile(entitlements) error = %v", err)
	}
	signScript := `#!/bin/sh
codesign --force --deep --options runtime --timestamp --entitlements "$ENTITLEMENTS" --sign "$IDENTITY" "$APP_PATH"
`
	if err := os.WriteFile(signScriptPath, []byte(signScript), 0o755); err != nil {
		t.Fatalf("WriteFile(sign script) error = %v", err)
	}
	return entitlementsPath, signScriptPath
}

func writeTestAppBundle(t *testing.T, appBundle string, plist string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatalf("MkdirAll(MacOS) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "Resources"), 0o755); err != nil {
		t.Fatalf("MkdirAll(Resources) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(appBundle, "Contents", "_CodeSignature"), 0o755); err != nil {
		t.Fatalf("MkdirAll(_CodeSignature) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(appBundle, "Contents", "MacOS", "Yemaka"), []byte("binary"), 0o755); err != nil {
		t.Fatalf("WriteFile(binary) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(appBundle, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatalf("WriteFile(Info.plist) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(appBundle, "Contents", "Resources", "iconfile.icns"), []byte("icon"), 0o644); err != nil {
		t.Fatalf("WriteFile(icon) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(appBundle, "Contents", "_CodeSignature", "CodeResources"), []byte("signature"), 0o644); err != nil {
		t.Fatalf("WriteFile(CodeResources) error = %v", err)
	}
}

func releasePlist(bundleID string, minimumOS string, localNetworking bool) string {
	localNetworkingXML := ""
	if localNetworking {
		localNetworkingXML = `
        <key>NSAppTransportSecurity</key>
        <dict>
            <key>NSAllowsLocalNetworking</key>
            <true/>
        </dict>`
	}
	return `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
    <dict>
        <key>CFBundlePackageType</key>
        <string>APPL</string>
        <key>CFBundleName</key>
        <string>Yemaka</string>
        <key>CFBundleExecutable</key>
        <string>Yemaka</string>
        <key>CFBundleIdentifier</key>
        <string>` + bundleID + `</string>
        <key>LSMinimumSystemVersion</key>
        <string>` + minimumOS + `</string>` + localNetworkingXML + `
    </dict>
</plist>
`
}
