package server

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"yemaka/internal/agent"
	"yemaka/internal/config"
	"yemaka/internal/connectors"
	"yemaka/internal/extensions"
	"yemaka/internal/heartbeat"
	"yemaka/internal/internet"
	"yemaka/internal/knowledge"
	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/models"
	"yemaka/internal/notifications"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
	"yemaka/internal/safety"
	"yemaka/internal/scheduler"
	"yemaka/internal/secrets"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
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
	if err := emit(models.ChatEvent{Token: "local"}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Token: " web"})
}

func (f fakeRuntime) Embed(ctx context.Context, req models.EmbeddingRequest) (models.EmbeddingResponse, error) {
	embeddings := make([][]float64, 0, len(req.Input))
	for range req.Input {
		embeddings = append(embeddings, []float64{0.1, 0.2, 0.3})
	}
	return models.EmbeddingResponse{Embeddings: embeddings}, nil
}

type setupErrorRuntime struct {
	fakeRuntime
	healthErr error
	listErr   error
}

type slowStreamRuntime struct {
	fakeRuntime
	delay time.Duration
}

type captureChatRuntime struct {
	fakeRuntime
	requests []models.ChatRequest
}

func (r *captureChatRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	r.requests = append(r.requests, req)
	if err := emit(models.ChatEvent{Token: "attachment noted"}); err != nil {
		return err
	}
	return emit(models.ChatEvent{Done: true})
}

func (r slowStreamRuntime) ChatStream(ctx context.Context, req models.ChatRequest, emit func(models.ChatEvent) error) error {
	if err := emit(models.ChatEvent{Token: "first"}); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(r.delay):
	}
	return emit(models.ChatEvent{Token: " second"})
}

type disconnectingStreamWriter struct {
	header    http.Header
	writes    int
	failAfter int
	status    int
}

func (w *disconnectingStreamWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *disconnectingStreamWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

func (w *disconnectingStreamWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.failAfter > 0 && w.writes > w.failAfter {
		return 0, errors.New("client disconnected")
	}
	return len(data), nil
}

func (w *disconnectingStreamWriter) Flush() {}

func (f setupErrorRuntime) Health(ctx context.Context) error {
	if f.healthErr != nil {
		return f.healthErr
	}
	return f.fakeRuntime.Health(ctx)
}

func (f setupErrorRuntime) ListModels(ctx context.Context) ([]models.ModelInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.fakeRuntime.ListModels(ctx)
}

type modelEndpointRuntime struct {
	fakeRuntime
	showName     string
	generateName string
	prompt       string
}

func (r *modelEndpointRuntime) ShowModel(ctx context.Context, name string) (models.ModelDetails, error) {
	r.showName = name
	return models.ModelDetails{
		Name:              name,
		Family:            "qwen2",
		ParameterSize:     "2B",
		QuantizationLevel: "Q4_K_M",
		ContextLength:     4096,
	}, nil
}

func (r *modelEndpointRuntime) GenerateStream(ctx context.Context, req models.GenerateRequest, emit func(models.GenerateEvent) error) error {
	r.generateName = req.Model
	r.prompt = req.Prompt
	if err := emit(models.GenerateEvent{Token: "hello"}); err != nil {
		return err
	}
	return emit(models.GenerateEvent{Done: true})
}

func TestConversationMessageResultsIncludesRunningAgentTurnPlaceholder(t *testing.T) {
	user := memory.Message{
		ID:             "msg_user",
		ConversationID: "conv_1",
		Role:           "user",
		Content:        "plain local question",
		CreatedAt:      "2026-06-03T12:00:00Z",
		ActiveVariant:  true,
	}
	turn := memory.AgentTurn{
		ID:             "turn_1",
		ConversationID: "conv_1",
		UserMessageID:  user.ID,
		Status:         agent.AgentTurnRunning,
		Trace:          []string{"accepted task", "task: chat (low risk)", "model: local-small"},
		Sources:        []string{"README.md"},
		SourceKind:     "workspace",
		Model:          "local-small",
		CreatedAt:      "2026-06-03T12:00:01Z",
		UpdatedAt:      "2026-06-03T12:00:02Z",
	}

	result := conversationMessageResults([]memory.Message{user}, nil, []memory.AgentTurn{turn})
	if len(result) != 2 {
		t.Fatalf("messages = %d, want user plus running assistant placeholder: %+v", len(result), result)
	}
	placeholder := result[1]
	if placeholder.ID != turn.ID || placeholder.Role != "assistant" || placeholder.Meta != "working" || placeholder.ParentID != user.ID {
		t.Fatalf("placeholder = %+v", placeholder)
	}
	if placeholder.Content != "" || placeholder.Model != turn.Model || placeholder.SourceKind != "workspace" {
		t.Fatalf("placeholder content/model/source = %+v", placeholder)
	}
	if strings.Join(placeholder.Trace, "\n") != strings.Join(turn.Trace, "\n") {
		t.Fatalf("placeholder trace = %+v, want %+v", placeholder.Trace, turn.Trace)
	}

	assistant := memory.Message{
		ID:             "msg_assistant",
		ConversationID: "conv_1",
		Role:           "assistant",
		Content:        "final answer",
		ParentID:       user.ID,
		CreatedAt:      "2026-06-03T12:00:03Z",
		ActiveVariant:  true,
	}
	result = conversationMessageResults([]memory.Message{user, assistant}, nil, []memory.AgentTurn{turn})
	if len(result) != 2 {
		t.Fatalf("messages with saved assistant = %d, want no synthetic duplicate: %+v", len(result), result)
	}

	turn.Status = agent.AgentTurnCompleted
	turn.AssistantMessageID = ""
	result = conversationMessageResults([]memory.Message{user}, nil, []memory.AgentTurn{turn})
	if len(result) != 2 || result[1].Meta != "completed" || !strings.Contains(result[1].Content, "final answer") {
		t.Fatalf("completed turn without assistant should render diagnostic placeholder: %+v", result)
	}
}

func TestStatusEndpointUsesLocalCore(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	request := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var status Status
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.SQLiteOK {
		t.Fatal("SQLiteOK = false, want true")
	}
	if !status.ModelReady {
		t.Fatal("ModelReady = false, want true")
	}
	if status.SelectedModel != "small:2b" {
		t.Fatalf("SelectedModel = %q, want small:2b", status.SelectedModel)
	}
}

func TestOpsStatusEndpointComposesOperatingLoopWithoutNotificationNoise(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	ctx := context.Background()

	inbox, err := srv.notificationStore()
	if err != nil {
		t.Fatalf("notificationStore() error = %v", err)
	}
	if _, err := inbox.Record(notifications.Notification{
		Type:           "job_failed",
		Severity:       notifications.SeverityError,
		Title:          "Job failed",
		Message:        "A scheduled job failed.",
		ActionRequired: true,
	}); err != nil {
		t.Fatalf("record notification: %v", err)
	}
	if _, err := srv.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
		ConversationID: "conv_ops",
		SessionID:      "sess_ops",
		ToolName:       "internet_search",
		Status:         "failed",
		RiskLevel:      "medium",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}
	if _, err := srv.replayTraceStore().Save(replay.Trace{
		ID:          "ops_trace",
		UserRequest: "Search current news",
		Route:       replay.RouteSnapshot{Category: routing.RouteInternetSearch},
		FinalResult: replay.ResultSnapshot{Status: "failed"},
	}); err != nil {
		t.Fatalf("save replay trace: %v", err)
	}
	extensionStore := srv.extensionStore()
	if err := os.MkdirAll(filepath.Dir(extensionStore.AuditPath), 0o755); err != nil {
		t.Fatalf("mkdir extension audit: %v", err)
	}
	if err := os.WriteFile(extensionStore.AuditPath, []byte(`{"action":"validate","name":"ops_extension","status":"ok","message":"manifest valid","timestamp":"2026-06-06T12:00:00Z"}`+"\n"), 0o644); err != nil {
		t.Fatalf("write extension audit: %v", err)
	}
	if err := safety.AppendPolicyAudit(srv.deps.Profile.Logs, safety.PolicyRequest{
		Domain:     safety.DomainConnector,
		Action:     safety.ActionRead,
		Level:      safety.LevelReadOnly,
		Actor:      "connector:local_api",
		Resource:   "memory_search",
		PolicyMode: safety.PolicyModeSafe,
	}, safety.PolicyDecision{
		Allowed: true,
		Level:   safety.LevelReadOnly,
		Risk:    safety.PolicyRiskLow,
		Reason:  "connector memory search allowed",
	}); err != nil {
		t.Fatalf("append policy audit: %v", err)
	}
	if err := safety.AppendPolicyAudit(srv.deps.Profile.Logs, safety.PolicyRequest{
		Domain:     safety.DomainInternet,
		Action:     safety.ActionSearch,
		Level:      safety.LevelReadOnly,
		Actor:      "tool:internet_search",
		Resource:   "internet_search",
		PolicyMode: safety.PolicyModeSafe,
	}, safety.PolicyDecision{
		Allowed: false,
		Level:   safety.LevelReadOnly,
		Risk:    safety.PolicyRiskBlocked,
		Reason:  "internet search disabled",
	}); err != nil {
		t.Fatalf("append tool policy audit: %v", err)
	}
	if err := safety.AppendPolicyAudit(srv.deps.Profile.Logs, safety.PolicyRequest{
		Domain:     safety.DomainExtension,
		Action:     safety.ActionRun,
		Level:      safety.LevelReadOnly,
		Actor:      "extension:ops_extension",
		Resource:   "ops_extension",
		PolicyMode: safety.PolicyModeSafe,
	}, safety.PolicyDecision{
		Allowed: true,
		Level:   safety.LevelReadOnly,
		Risk:    safety.PolicyRiskLow,
		Reason:  "extension run allowed",
	}); err != nil {
		t.Fatalf("append extension policy audit: %v", err)
	}

	before, err := inbox.List(100, true)
	if err != nil {
		t.Fatalf("list notifications before: %v", err)
	}
	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodGet, "/api/ops/status?limit=20", nil)
		recorder := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("ops status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
		}
		var status OpsStatus
		if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode ops status: %v", err)
		}
		if len(status.Stages) == 0 || len(status.Notifications) == 0 || len(status.RecentReplay) == 0 || len(status.RecentToolRuns) == 0 {
			t.Fatalf("ops status missing composed data: %+v", status)
		}
		if len(status.Timeline) == 0 {
			t.Fatalf("ops status timeline missing: %+v", status)
		}
		stages := opsStageNames(status.Stages)
		for _, want := range []string{"heartbeat", "qa_review", "audit", "policy_audit", "extensions", "connectors", "knowledge", "model_profiles"} {
			if !stages[want] {
				t.Fatalf("ops stages = %+v, missing %q", status.Stages, want)
			}
		}
		if len(status.ExtensionAudit) == 0 || status.ExtensionAudit[0].Name != "ops_extension" {
			t.Fatalf("extension audit = %+v, want ops_extension record", status.ExtensionAudit)
		}
		if len(status.PolicyAudit) == 0 || !opsPolicyAuditHasActor(status.PolicyAudit, "connector:local_api") {
			t.Fatalf("policy audit = %+v, want connector local_api record", status.PolicyAudit)
		}
		if status.PolicyAudit[0].ID == "" || status.PolicyAudit[0].Line == 0 {
			t.Fatalf("policy audit = %+v, want readback id and line", status.PolicyAudit)
		}
		if status.Connectors.Total == 0 {
			t.Fatalf("connectors summary = %+v, want connector count", status.Connectors)
		}
		if status.ModelProfiles.Status == "" {
			t.Fatalf("model profile summary = %+v, want status", status.ModelProfiles)
		}
		sources := opsTimelineSources(status.Timeline)
		for _, want := range []string{"notifications", "qa_review", "replay", "audit", "extensions", "policy_audit"} {
			if !sources[want] {
				t.Fatalf("ops timeline sources = %+v, missing %q; timeline=%+v", sources, want, status.Timeline)
			}
		}
		toolEvent, ok := opsTimelineEvent(status.Timeline, "audit", "tool_run", "internet_search")
		if !ok || !opsTimelineHasRelated(toolEvent, "policy_audit", "policy_decision") {
			t.Fatalf("tool timeline event missing related policy audit: ok=%v event=%+v timeline=%+v", ok, toolEvent, status.Timeline)
		}
		policyEvent, ok := opsTimelineEvent(status.Timeline, "policy_audit", "internet.search", "Policy decision: internet search")
		if !ok || policyEvent.CorrelationID != "internet_search" || !opsTimelineHasRelated(policyEvent, "audit", "tool_run") {
			t.Fatalf("policy timeline event missing tool correlation: ok=%v event=%+v timeline=%+v", ok, policyEvent, status.Timeline)
		}
		extensionEvent, ok := opsTimelineEvent(status.Timeline, "extensions", "validate", "Extension validate: ops_extension")
		if !ok || !opsTimelineHasRelated(extensionEvent, "policy_audit", "policy_decision") {
			t.Fatalf("extension timeline event missing related policy audit: ok=%v event=%+v timeline=%+v", ok, extensionEvent, status.Timeline)
		}
	}
	after, err := inbox.List(100, true)
	if err != nil {
		t.Fatalf("list notifications after: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("ops status changed notification count: before %d after %d", len(before), len(after))
	}
}

func TestNotificationStageErrorIsActionWarningNotBroken(t *testing.T) {
	got := notificationStageStatus([]notifications.Notification{{
		Type:           "job_failed",
		Severity:       notifications.SeverityError,
		Title:          "Job failed",
		ActionRequired: true,
	}})
	if got != heartbeat.StatusWarning {
		t.Fatalf("notificationStageStatus() = %q, want %q", got, heartbeat.StatusWarning)
	}
}

func TestOpsStatusEndpointIncludesReleaseWhenRequested(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	request := httptest.NewRequest(http.MethodGet, "/api/ops/status?includeRelease=true", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("ops release status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var status OpsStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode ops release status: %v", err)
	}
	if status.Release == nil || len(status.Release.Checks) == 0 {
		t.Fatalf("release summary = %+v, want checks", status.Release)
	}
	if !opsStageNames(status.Stages)["release"] {
		t.Fatalf("ops stages = %+v, want release", status.Stages)
	}
}

func opsStageNames(stages []OpsStage) map[string]bool {
	names := map[string]bool{}
	for _, stage := range stages {
		names[stage.Name] = true
	}
	return names
}

func opsTimelineSources(events []OpsTimelineEvent) map[string]bool {
	sources := map[string]bool{}
	for _, event := range events {
		sources[event.Source] = true
	}
	return sources
}

func opsPolicyAuditHasActor(records []safety.PolicyAuditRecord, actor string) bool {
	for _, record := range records {
		if record.Request.Actor == actor {
			return true
		}
	}
	return false
}

func opsTimelineEvent(events []OpsTimelineEvent, source string, kind string, title string) (OpsTimelineEvent, bool) {
	for _, event := range events {
		if event.Source == source && event.Kind == kind && event.Title == title {
			return event, true
		}
	}
	return OpsTimelineEvent{}, false
}

func opsTimelineHasRelated(event OpsTimelineEvent, source string, entityType string) bool {
	for _, link := range event.Related {
		if link.Source == source && link.EntityType == entityType && link.EntityID != "" {
			return true
		}
	}
	return false
}

func TestSetupEndpointReturnsEmptyArraysWhenOllamaListingFails(t *testing.T) {
	cases := []struct {
		name    string
		runtime models.Runtime
	}{
		{
			name: "health_error",
			runtime: setupErrorRuntime{
				fakeRuntime: fakeRuntime{models: []models.ModelInfo{{Name: "small:2b"}}},
				healthErr:   errors.New("ollama offline"),
			},
		},
		{
			name: "list_error",
			runtime: setupErrorRuntime{
				fakeRuntime: fakeRuntime{models: []models.ModelInfo{{Name: "small:2b"}}},
				listErr:     errors.New("list models failed"),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, cleanup := newTestServerWithRuntime(t, tc.runtime)
			defer cleanup()

			request := httptest.NewRequest(http.MethodGet, "/api/setup", nil)
			recorder := httptest.NewRecorder()
			srv.Handler().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
			}
			var state SetupState
			if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
				t.Fatalf("decode setup state: %v", err)
			}
			if state.OllamaError == "" {
				t.Fatal("OllamaError is empty, want failure detail")
			}
			if state.InstalledModels == nil {
				t.Fatal("InstalledModels = nil, want empty slice for frontend safety")
			}
			if state.ConfiguredDefaults == nil {
				t.Fatal("ConfiguredDefaults = nil, want empty slice for frontend safety")
			}
		})
	}
}

func TestSetupEndpointReturnsCurrentMode(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	srv.deps.Config.Runtime.MaxContextTokens = 6144
	request := httptest.NewRequest(http.MethodGet, "/api/setup", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var state SetupState
	if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
		t.Fatalf("decode setup state: %v", err)
	}
	if state.Mode != "useful_local" {
		t.Fatalf("Mode = %q, want useful_local", state.Mode)
	}
	if state.RecommendedMode == "" {
		t.Fatal("RecommendedMode is empty")
	}
}

func TestCompleteSetupPersistsSelectedModeLowMemoryFlag(t *testing.T) {
	cases := []struct {
		name          string
		mode          string
		wantLowMemory bool
	}{
		{name: "student laptop", mode: "student_laptop", wantLowMemory: true},
		{name: "useful local", mode: "useful_local", wantLowMemory: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv, cleanup := newTestServer(t)
			defer cleanup()

			body := strings.NewReader(`{"mode":"` + tc.mode + `","lowMemoryModel":"small:2b","defaultModel":"small:2b"}`)
			request := httptest.NewRequest(http.MethodPost, "/api/setup", body)
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()
			srv.Handler().ServeHTTP(recorder, request)

			if recorder.Code != http.StatusOK {
				t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
			}
			var state SetupState
			if err := json.Unmarshal(recorder.Body.Bytes(), &state); err != nil {
				t.Fatalf("decode setup state: %v", err)
			}
			if state.Mode != tc.mode {
				t.Fatalf("Mode = %q, want %q", state.Mode, tc.mode)
			}
			if state.LowMemoryMode != tc.wantLowMemory {
				t.Fatalf("LowMemoryMode = %t, want %t", state.LowMemoryMode, tc.wantLowMemory)
			}
			persisted, err := config.LoadOrCreate()
			if err != nil {
				t.Fatalf("load persisted config: %v", err)
			}
			if persisted.Runtime.LowMemoryMode != tc.wantLowMemory {
				t.Fatalf("persisted low_memory_mode = %t, want %t", persisted.Runtime.LowMemoryMode, tc.wantLowMemory)
			}
		})
	}
}

func TestChatEndpointSavesConversation(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"content":"hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/chat", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result ChatResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode chat result: %v", err)
	}
	if result.Text != "local web" {
		t.Fatalf("Text = %q, want local web", result.Text)
	}
	if result.ConversationID == "" {
		t.Fatal("ConversationID is empty")
	}
}

func TestChatAttachmentUploadAndAskUsesSessionContextOnly(t *testing.T) {
	runtime := &captureChatRuntime{fakeRuntime: fakeRuntime{models: []models.ModelInfo{{Name: "small:2b", Size: 1_500_000_000}}}}
	srv, cleanup := newTestServerWithRuntime(t, runtime)
	defer cleanup()

	var form bytes.Buffer
	writer := multipart.NewWriter(&form)
	part, err := writer.CreateFormFile("file", "/Users/example/Documents/brief.txt")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("Project alpha attachment context for this conversation only.")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/chat/attachments", &form)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(uploadRecorder, uploadRequest)
	if uploadRecorder.Code != http.StatusOK {
		t.Fatalf("upload status code = %d, want 200; body: %s", uploadRecorder.Code, uploadRecorder.Body.String())
	}
	var upload ChatAttachmentUploadResult
	if err := json.Unmarshal(uploadRecorder.Body.Bytes(), &upload); err != nil {
		t.Fatalf("decode upload result: %v", err)
	}
	if upload.Attachment.ID == "" || upload.Attachment.FileName != "brief.txt" || upload.Attachment.Status != "ready" {
		t.Fatalf("upload attachment = %+v, want ready redacted attachment", upload.Attachment)
	}

	body, err := json.Marshal(map[string]any{
		"content":       "Summarize the attached brief",
		"attachmentIds": []string{upload.Attachment.ID},
	})
	if err != nil {
		t.Fatalf("marshal ask body: %v", err)
	}
	askRequest := httptest.NewRequest(http.MethodPost, "/api/ask", bytes.NewReader(body))
	askRequest.Header.Set("Content-Type", "application/json")
	askRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(askRecorder, askRequest)
	if askRecorder.Code != http.StatusOK {
		t.Fatalf("ask status code = %d, want 200; body: %s", askRecorder.Code, askRecorder.Body.String())
	}
	var ask AskResult
	if err := json.Unmarshal(askRecorder.Body.Bytes(), &ask); err != nil {
		t.Fatalf("decode ask result: %v", err)
	}
	if len(ask.Attachments) != 1 || ask.Attachments[0].ID != upload.Attachment.ID {
		t.Fatalf("ask attachments = %+v, want linked attachment", ask.Attachments)
	}
	if len(runtime.requests) == 0 {
		t.Fatal("runtime saw no chat request")
	}
	var sawAttachmentContext bool
	for _, message := range runtime.requests[0].Messages {
		if strings.Contains(message.Content, "CHAT ATTACHMENTS") && strings.Contains(message.Content, "Project alpha attachment context") {
			sawAttachmentContext = true
		}
	}
	if !sawAttachmentContext {
		t.Fatalf("runtime request = %+v, want attachment context", runtime.requests[0].Messages)
	}
	searchResults, err := srv.deps.Memory.SearchMessages(context.Background(), "Project alpha attachment context", 5)
	if err != nil {
		t.Fatalf("SearchMessages() error = %v", err)
	}
	if len(searchResults) != 0 {
		t.Fatalf("SearchMessages found attachment text in message FTS: %+v", searchResults)
	}
	conversationRequest := httptest.NewRequest(http.MethodGet, "/api/conversations/"+ask.ConversationID, nil)
	conversationRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(conversationRecorder, conversationRequest)
	if conversationRecorder.Code != http.StatusOK {
		t.Fatalf("conversation status code = %d, want 200; body: %s", conversationRecorder.Code, conversationRecorder.Body.String())
	}
	var detail ConversationDetailResult
	if err := json.Unmarshal(conversationRecorder.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode conversation detail: %v", err)
	}
	var foundUserAttachment bool
	for _, message := range detail.Messages {
		if message.ID == ask.UserMessageID && len(message.Attachments) == 1 {
			foundUserAttachment = true
		}
	}
	if !foundUserAttachment {
		t.Fatalf("conversation messages = %+v, want user attachment metadata", detail.Messages)
	}
}

func TestEditUserMessageEndpointCreatesPromptVariant(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	ctx := context.Background()
	conversation, err := srv.deps.Memory.CreateConversation(ctx, "Original prompt")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := srv.deps.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "old prompt",
		Model:          "small:2b",
	})
	if err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "old response",
		Model:          "small:2b",
		ParentID:       user.ID,
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "later turn",
		Model:          "small:2b",
	}); err != nil {
		t.Fatalf("SaveMessage(later user) error = %v", err)
	}

	body := strings.NewReader(`{"messageId":"` + user.ID + `","content":"edited prompt"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/messages/edit-user", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result ConversationMessageResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode edited message: %v", err)
	}
	if result.ID == user.ID || result.Role != "user" || result.Content != "edited prompt" || result.ParentID != user.ID || !result.ActiveVariant {
		t.Fatalf("edited message = %+v", result)
	}
	messages, err := srv.deps.Memory.ListConversationMessages(ctx, conversation.ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("messages after edit count = %d, want 4: %#v", len(messages), messages)
	}
	if messages[0].ID != user.ID || messages[0].Content != "old prompt" || messages[0].ActiveVariant {
		t.Fatalf("original prompt after edit = %#v", messages[0])
	}
	if messages[3].ID != result.ID || messages[3].Content != "edited prompt" || !messages[3].ActiveVariant {
		t.Fatalf("messages after edit = %#v", messages)
	}
}

func TestAskStreamEndpointStreamsEvents(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"content":"hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/ask/stream", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	var sawToken bool
	var final AskResult
	for _, frame := range strings.Split(strings.TrimSpace(recorder.Body.String()), "\n\n") {
		var data string
		for _, line := range strings.Split(frame, "\n") {
			if strings.HasPrefix(line, "data: ") {
				data = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			}
		}
		if data == "" {
			t.Fatalf("stream frame missing data: %q", frame)
		}
		var event AskStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			t.Fatalf("decode stream frame %q: %v", frame, err)
		}
		if event.Type == "model.token" && event.Token != "" {
			sawToken = true
		}
		if event.Type == "result" && event.Result != nil {
			final = *event.Result
		}
	}
	if !sawToken {
		t.Fatal("stream did not include model.token events")
	}
	if final.Text != "local web" {
		t.Fatalf("final.Text = %q, want local web", final.Text)
	}
	if final.ConversationID == "" {
		t.Fatal("final.ConversationID is empty")
	}
}

func TestAskStreamContinuesAfterClientDisconnect(t *testing.T) {
	srv, cleanup := newTestServerWithRuntime(t, slowStreamRuntime{
		fakeRuntime: fakeRuntime{models: []models.ModelInfo{{Name: "small:2b", Size: 1_500_000_000}}},
		delay:       10 * time.Millisecond,
	})
	defer cleanup()

	writer := &disconnectingStreamWriter{failAfter: 4}
	if err := srv.streamAsk(writer, context.Background(), "hello", "", "", "", nil); err != nil {
		t.Fatalf("streamAsk() error = %v", err)
	}
	conversations, err := srv.deps.Memory.ListConversations(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListConversations() error = %v", err)
	}
	if len(conversations) != 1 {
		t.Fatalf("conversations = %d, want 1", len(conversations))
	}
	messages, err := srv.deps.Memory.ListConversationMessages(context.Background(), conversations[0].ID, 10)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	var assistant memory.Message
	for _, message := range messages {
		if message.Role == "assistant" {
			assistant = message
			break
		}
	}
	if assistant.Content != "first second" {
		t.Fatalf("assistant content = %q, want completed answer after disconnect; messages=%+v", assistant.Content, messages)
	}
	turns, err := srv.deps.Memory.ListAgentTurnsForConversation(context.Background(), conversations[0].ID, 10)
	if err != nil {
		t.Fatalf("ListAgentTurnsForConversation() error = %v", err)
	}
	if len(turns) != 1 || turns[0].Status != agent.AgentTurnCompleted {
		t.Fatalf("turns = %+v, want one completed turn", turns)
	}
}

func TestAskStreamEndpointFlushesBeforeFinalResult(t *testing.T) {
	srv, cleanup := newTestServerWithRuntime(t, slowStreamRuntime{
		fakeRuntime: fakeRuntime{models: []models.ModelInfo{{Name: "small:2b", Size: 1_500_000_000}}},
		delay:       180 * time.Millisecond,
	})
	defer cleanup()

	httpServer := httptest.NewServer(srv.Handler())
	defer httpServer.Close()

	body := strings.NewReader(`{"content":"hello"}`)
	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/api/ask/stream", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 3 * time.Second}
	start := time.Now()
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("post stream: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want 200", response.StatusCode)
	}

	events := make(chan AskStreamEvent, 8)
	errs := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(response.Body)
		var data string
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				if data != "" {
					var event AskStreamEvent
					if err := json.Unmarshal([]byte(data), &event); err != nil {
						errs <- err
						return
					}
					events <- event
					data = ""
				}
				continue
			}
			if strings.HasPrefix(line, "data: ") {
				data = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			}
		}
		if err := scanner.Err(); err != nil {
			errs <- err
		}
	}()

	var firstTokenAt time.Duration
	var resultAt time.Duration
	timeout := time.After(2 * time.Second)
	for resultAt == 0 {
		select {
		case event := <-events:
			if event.Type == "model.token" && event.Token == "first" && firstTokenAt == 0 {
				firstTokenAt = time.Since(start)
			}
			if event.Type == "result" {
				resultAt = time.Since(start)
			}
		case err := <-errs:
			t.Fatalf("read stream: %v", err)
		case <-timeout:
			t.Fatal("timed out waiting for stream result")
		}
	}

	if firstTokenAt == 0 {
		t.Fatal("first token was not observed before final result")
	}
	if resultAt-firstTokenAt < 120*time.Millisecond {
		t.Fatalf("first token and result were batched: first=%s result=%s", firstTokenAt, resultAt)
	}
}

func TestShowModelEndpointUsesRuntimeCapability(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	runtime := &modelEndpointRuntime{fakeRuntime: fakeRuntime{models: []models.ModelInfo{{Name: "small:2b"}}}}
	srv.deps.Runtime = runtime

	request := httptest.NewRequest(http.MethodGet, "/api/models/show?name=small:2b", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result models.ModelDetails
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode model details: %v", err)
	}
	if runtime.showName != "small:2b" {
		t.Fatalf("showName = %q, want small:2b", runtime.showName)
	}
	if result.ParameterSize != "2B" {
		t.Fatalf("ParameterSize = %q, want 2B", result.ParameterSize)
	}
}

func TestGenerateModelEndpointUsesRuntimeCapability(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	runtime := &modelEndpointRuntime{fakeRuntime: fakeRuntime{models: []models.ModelInfo{{Name: "small:2b"}}}}
	srv.deps.Runtime = runtime

	body := strings.NewReader(`{"name":"small:2b","prompt":"Say hello"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/models/generate", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result ModelGenerateResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode generate result: %v", err)
	}
	if runtime.generateName != "small:2b" {
		t.Fatalf("generateName = %q, want small:2b", runtime.generateName)
	}
	if runtime.prompt != "Say hello" {
		t.Fatalf("prompt = %q, want Say hello", runtime.prompt)
	}
	if result.Text != "hello" {
		t.Fatalf("Text = %q, want hello", result.Text)
	}
}

func TestModelProfileEndpointsPreviewSaveListShowAndRejectUnsafe(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	before := srv.deps.Config.Models["low_memory"]

	body := strings.NewReader(`{
		"name":"Careful Local Helper",
		"description":"Concise local behavior profile.",
		"baseModel":"small:2b",
		"system":"Stay concise and preserve local-first defaults.",
		"temperature":0.2,
		"numCtx":2048,
		"tags":["local","careful"],
		"purpose":"general chat"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/model-profiles/preview", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var preview ModelProfileResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Profile.Name != "Careful Local Helper" || !strings.Contains(preview.Modelfile, "FROM small:2b") {
		t.Fatalf("preview = %+v", preview)
	}
	if preview.Path != "" {
		t.Fatalf("preview path = %q, want empty because preview must not write", preview.Path)
	}

	body = strings.NewReader(`{
		"name":"Careful Local Helper",
		"description":"Concise local behavior profile.",
		"baseModel":"small:2b",
		"system":"Stay concise and preserve local-first defaults.",
		"temperature":0.2,
		"numCtx":2048,
		"tags":["local","careful"],
		"purpose":"general chat"
	}`)
	request = httptest.NewRequest(http.MethodPost, "/api/model-profiles", body)
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var saved ModelProfileResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode saved: %v", err)
	}
	if saved.Path == "" || !strings.Contains(saved.Modelfile, "PARAMETER num_ctx 2048") {
		t.Fatalf("saved = %+v", saved)
	}
	if srv.deps.Config.Models["low_memory"] != before {
		t.Fatalf("model profile save mutated model role: got %#v want %#v", srv.deps.Config.Models["low_memory"], before)
	}

	body = strings.NewReader(`{"name":"Careful Local Helper","role":"low_memory"}`)
	request = httptest.NewRequest(http.MethodPost, "/api/model-profiles/apply", body)
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("apply status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var applied ModelProfileResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode applied: %v", err)
	}
	if !applied.Applied || applied.AppliedRole != "low_memory" {
		t.Fatalf("applied result = %+v, want explicit low_memory apply", applied)
	}
	low := srv.deps.Config.Models["low_memory"]
	if low.Name != "small:2b" || low.Profile != "Careful Local Helper" || low.Temperature != 0.2 {
		t.Fatalf("low_memory after apply = %#v, want profile binding and base model", low)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/model-profiles", nil)
	recorder = httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var profiles []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &profiles); err != nil {
		t.Fatalf("decode profiles: %v", err)
	}
	if len(profiles) != 1 || profiles[0]["name"] != "Careful Local Helper" {
		t.Fatalf("profiles = %#v", profiles)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/model-profiles/show?name=Careful%20Local%20Helper", nil)
	recorder = httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("show status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var shown ModelProfileResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &shown); err != nil {
		t.Fatalf("decode shown: %v", err)
	}
	if shown.Profile.Name != "Careful Local Helper" || !strings.Contains(shown.Modelfile, "Stay concise") {
		t.Fatalf("shown = %+v", shown)
	}

	body = strings.NewReader(`{"name":"Leaky","baseModel":"small:2b","system":"api_key = sk-testsecretvalue1234567890"}`)
	request = httptest.NewRequest(http.MethodPost, "/api/model-profiles/preview", body)
	request.Header.Set("Content-Type", "application/json")
	recorder = httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unsafe preview status = %d, want 400; body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestModelProfileDraftEndpointUsesReviewedConversationWithoutWriting(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	conversation, err := srv.deps.Memory.CreateConversation(context.Background(), "Reviewed model behavior")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Explain local routing with api_key = sk-testsecretvalue1234567890",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Model:          "small:2b",
		Content:        "A concise local explanation:\n- keep routing deterministic\n- ask clarification when needed\n\nNeed an example?",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	body := strings.NewReader(fmt.Sprintf(`{"conversationId":%q,"name":"Reviewed Routing Helper"}`, conversation.ID))
	request := httptest.NewRequest(http.MethodPost, "/api/model-profiles/draft", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("draft status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result ModelProfileResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if result.Path != "" {
		t.Fatalf("draft path = %q, want preview-only no write", result.Path)
	}
	if result.Profile.Metadata.RefinedFrom != conversation.ID || result.DraftReport == nil || result.DraftReport.Eligibility != "eligible" {
		t.Fatalf("draft result = %+v", result)
	}
	for _, leaked := range []string{"sk-testsecretvalue", "Explain local routing", "keep routing deterministic"} {
		if strings.Contains(result.Modelfile, leaked) {
			t.Fatalf("draft leaked transcript content %q:\n%s", leaked, result.Modelfile)
		}
	}
	if !strings.Contains(result.Modelfile, "reviewed successful conversation") || !strings.Contains(result.Modelfile, "clarifying") {
		t.Fatalf("draft Modelfile missing reviewed behavior guardrails:\n%s", result.Modelfile)
	}

	request = httptest.NewRequest(http.MethodGet, "/api/model-profiles", nil)
	recorder = httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var profiles []map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &profiles); err != nil {
		t.Fatalf("decode profiles: %v", err)
	}
	if len(profiles) != 0 {
		t.Fatalf("profiles after draft = %#v, want no saved profiles", profiles)
	}
}

func TestModelProfileDraftEndpointRejectsFailedConversation(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	conversation, err := srv.deps.Memory.CreateConversation(context.Background(), "Failed web search")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{ConversationID: conversation.ID, Role: "user", Content: "search latest news"}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{ConversationID: conversation.ID, Role: "assistant", Model: "small:2b", Content: "Search failed."}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveToolRun(context.Background(), memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "internet_search",
		Status:         "failed",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	body := strings.NewReader(fmt.Sprintf(`{"conversationId":%q}`, conversation.ID))
	request := httptest.NewRequest(http.MethodPost, "/api/model-profiles/draft", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("failed draft status = %d, want 400; body: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "successful conversation") {
		t.Fatalf("failed draft body = %q, want success requirement", recorder.Body.String())
	}
}

func TestToolRunsEndpointReturnsRecentRuns(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	if _, err := srv.deps.Memory.SaveToolRun(context.Background(), memory.ToolRun{
		ToolName:  "run_tests",
		Input:     map[string]any{"command": "go test ./..."},
		Output:    map[string]any{"status": "completed"},
		Status:    "completed",
		RiskLevel: "low",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/tool-runs?limit=5", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var runs []ToolRunResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode tool runs: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("tool run count = %d, want 1", len(runs))
	}
	if runs[0].ToolName != "run_tests" {
		t.Fatalf("ToolName = %q, want run_tests", runs[0].ToolName)
	}
}

func TestMemoryEndpointsWriteSearchPinDelete(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	writeRequest := httptest.NewRequest(http.MethodPost, "/api/memories", strings.NewReader(`{"kind":"preference","content":"Prefer concise local answers","importance":5}`))
	writeRequest.Header.Set("Content-Type", "application/json")
	writeRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(writeRecorder, writeRequest)
	if writeRecorder.Code != http.StatusOK {
		t.Fatalf("write status code = %d, want 200; body: %s", writeRecorder.Code, writeRecorder.Body.String())
	}
	var written MemoryResult
	if err := json.Unmarshal(writeRecorder.Body.Bytes(), &written); err != nil {
		t.Fatalf("decode written memory: %v", err)
	}
	if written.ID == "" {
		t.Fatal("memory ID is empty")
	}

	searchRequest := httptest.NewRequest(http.MethodGet, "/api/memory?query=concise", nil)
	searchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK {
		t.Fatalf("search status code = %d, want 200; body: %s", searchRecorder.Code, searchRecorder.Body.String())
	}
	var results []MemoryResult
	if err := json.Unmarshal(searchRecorder.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode memory search: %v", err)
	}
	if len(results) == 0 || results[0].ID != written.ID {
		t.Fatalf("search results = %+v, want written memory first", results)
	}

	pinRequest := httptest.NewRequest(http.MethodPost, "/api/memories/pin", strings.NewReader(`{"id":"`+written.ID+`"}`))
	pinRequest.Header.Set("Content-Type", "application/json")
	pinRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(pinRecorder, pinRequest)
	if pinRecorder.Code != http.StatusOK {
		t.Fatalf("pin status code = %d, want 200; body: %s", pinRecorder.Code, pinRecorder.Body.String())
	}

	deleteRequest := httptest.NewRequest(http.MethodPost, "/api/memories/delete", strings.NewReader(`{"id":"`+written.ID+`"}`))
	deleteRequest.Header.Set("Content-Type", "application/json")
	deleteRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(deleteRecorder, deleteRequest)
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete status code = %d, want 200; body: %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestKnowledgeEndpointsStayDisabledUntilExplicitlyEnabled(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/status", nil)
	statusRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var status KnowledgeStatus
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode knowledge status: %v", err)
	}
	if status.Enabled {
		t.Fatalf("knowledge graph enabled by default: %+v", status)
	}
	if status.InfluenceEnabled {
		t.Fatalf("knowledge graph influence enabled by default: %+v", status)
	}
	if !status.ManualOnly {
		t.Fatalf("knowledge graph manualOnly = false: %+v", status)
	}

	blockedRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/entities", strings.NewReader(`{"name":"Cogging torque"}`))
	blockedRequest.Header.Set("Content-Type", "application/json")
	blockedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(blockedRecorder, blockedRequest)
	if blockedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("blocked status = %d, want 400; body: %s", blockedRecorder.Code, blockedRecorder.Body.String())
	}
	if !strings.Contains(blockedRecorder.Body.String(), "disabled") {
		t.Fatalf("blocked body = %q, want disabled message", blockedRecorder.Body.String())
	}

	repairRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/repair", nil)
	repairRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(repairRecorder, repairRequest)
	if repairRecorder.Code != http.StatusBadRequest {
		t.Fatalf("repair status = %d, want 400 while disabled; body: %s", repairRecorder.Code, repairRecorder.Body.String())
	}
	if !strings.Contains(repairRecorder.Body.String(), "disabled") {
		t.Fatalf("repair body = %q, want disabled message", repairRecorder.Body.String())
	}

	reviewRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/review?status=pending", nil)
	reviewRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(reviewRecorder, reviewRequest)
	if reviewRecorder.Code != http.StatusBadRequest {
		t.Fatalf("review status = %d, want 400 while disabled; body: %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}
	if !strings.Contains(reviewRecorder.Body.String(), "disabled") {
		t.Fatalf("review body = %q, want disabled message", reviewRecorder.Body.String())
	}

	draftRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/proposals", strings.NewReader(`{"text":"Entity: Cogging Torque | kind: engineering concept | evidence: token=secret-value from /Users/example/Documents/design.md","source":"manual QA","sourceKind":"rag","sourceRef":"/Users/example/Documents/design.md token=secret-value"}`))
	draftRequest.Header.Set("Content-Type", "application/json")
	draftRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(draftRecorder, draftRequest)
	if draftRecorder.Code != http.StatusOK {
		t.Fatalf("draft status = %d, want 200 while graph disabled; body: %s", draftRecorder.Code, draftRecorder.Body.String())
	}
	var draft KnowledgeProposalResult
	if err := json.Unmarshal(draftRecorder.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if !draft.ReviewRequired || len(draft.EntityProposals) != 1 {
		t.Fatalf("draft = %+v, want review-only entity proposal", draft)
	}
	if draft.SourceKind != "rag" || strings.Contains(draft.SourceRef, "/Users/example") || strings.Contains(draft.SourceRef, "secret-value") {
		t.Fatalf("draft provenance = kind %q ref %q, want sanitized RAG source", draft.SourceKind, draft.SourceRef)
	}
	if draft.EntityProposals[0].SourceKind != "rag" || draft.EntityProposals[0].SourceRef != draft.SourceRef {
		t.Fatalf("entity provenance = kind %q ref %q, want proposal provenance", draft.EntityProposals[0].SourceKind, draft.EntityProposals[0].SourceRef)
	}
	if strings.Contains(draft.EntityProposals[0].Evidence, "secret-value") || strings.Contains(draft.EntityProposals[0].Evidence, "/Users/example") {
		t.Fatalf("draft evidence was not redacted: %q", draft.EntityProposals[0].Evidence)
	}
}

func TestKnowledgeEndpointsManualAddSearchAndLink(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	enableRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/enabled", strings.NewReader(`{"enabled":true}`))
	enableRequest.Header.Set("Content-Type", "application/json")
	enableRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(enableRecorder, enableRequest)
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want 200; body: %s", enableRecorder.Code, enableRecorder.Body.String())
	}
	var enabled KnowledgeStatus
	if err := json.Unmarshal(enableRecorder.Body.Bytes(), &enabled); err != nil {
		t.Fatalf("decode enabled status: %v", err)
	}
	if !enabled.Enabled || enabled.InfluenceEnabled || !enabled.ManualOnly {
		t.Fatalf("enabled status = %+v, want enabled manual-only with influence off", enabled)
	}

	enableInfluenceRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/enabled", strings.NewReader(`{"enabled":true,"influenceEnabled":true}`))
	enableInfluenceRequest.Header.Set("Content-Type", "application/json")
	enableInfluenceRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(enableInfluenceRecorder, enableInfluenceRequest)
	if enableInfluenceRecorder.Code != http.StatusOK {
		t.Fatalf("enable influence status = %d, want 200; body: %s", enableInfluenceRecorder.Code, enableInfluenceRecorder.Body.String())
	}
	var influenceEnabled KnowledgeStatus
	if err := json.Unmarshal(enableInfluenceRecorder.Body.Bytes(), &influenceEnabled); err != nil {
		t.Fatalf("decode influence enabled status: %v", err)
	}
	if !influenceEnabled.Enabled || !influenceEnabled.InfluenceEnabled || !influenceEnabled.ManualOnly {
		t.Fatalf("influence status = %+v, want graph and influence enabled manual-only", influenceEnabled)
	}

	entityBody := `{"name":"Cogging Torque","kind":"Engineering Concept","source":"manual QA","evidence":"token=super-secret-value from /Users/example/Documents/motor notes"}`
	entityRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/entities", strings.NewReader(entityBody))
	entityRequest.Header.Set("Content-Type", "application/json")
	entityRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(entityRecorder, entityRequest)
	if entityRecorder.Code != http.StatusOK {
		t.Fatalf("entity status = %d, want 200; body: %s", entityRecorder.Code, entityRecorder.Body.String())
	}
	var entity KnowledgeEntityResult
	if err := json.Unmarshal(entityRecorder.Body.Bytes(), &entity); err != nil {
		t.Fatalf("decode entity: %v", err)
	}
	if entity.ID == "" || entity.Kind != "engineering_concept" || entity.Source != "manual_qa" {
		t.Fatalf("entity = %+v", entity)
	}
	if strings.Contains(entity.Evidence, "super-secret-value") || strings.Contains(entity.Evidence, "/Users/example") {
		t.Fatalf("entity evidence was not redacted: %q", entity.Evidence)
	}

	secondRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/entities", strings.NewReader(`{"name":"Brushless Motor","kind":"Component","source":"manual QA"}`))
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusOK {
		t.Fatalf("second entity status = %d, want 200; body: %s", secondRecorder.Code, secondRecorder.Body.String())
	}
	var second KnowledgeEntityResult
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second entity: %v", err)
	}

	edgeBody := fmt.Sprintf(`{"fromEntityId":%q,"relation":"has issue","toEntityId":%q,"source":"manual QA","evidence":"manual relation"}`, second.ID, entity.ID)
	edgeRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/edges", strings.NewReader(edgeBody))
	edgeRequest.Header.Set("Content-Type", "application/json")
	edgeRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(edgeRecorder, edgeRequest)
	if edgeRecorder.Code != http.StatusOK {
		t.Fatalf("edge status = %d, want 200; body: %s", edgeRecorder.Code, edgeRecorder.Body.String())
	}
	var edge KnowledgeEdgeResult
	if err := json.Unmarshal(edgeRecorder.Body.Bytes(), &edge); err != nil {
		t.Fatalf("decode edge: %v", err)
	}
	if edge.Relation != "has_issue" || edge.FromEntityID != second.ID || edge.ToEntityID != entity.ID {
		t.Fatalf("edge = %+v", edge)
	}

	searchRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/entities?query=torque&limit=5", nil)
	searchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200; body: %s", searchRecorder.Code, searchRecorder.Body.String())
	}
	var results []KnowledgeEntityResult
	if err := json.Unmarshal(searchRecorder.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode search results: %v", err)
	}
	if len(results) == 0 || results[0].ID != entity.ID {
		t.Fatalf("search results = %+v, want entity %q", results, entity.ID)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/status", nil)
	statusRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("final status code = %d, want 200; body: %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var finalStatus KnowledgeStatus
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &finalStatus); err != nil {
		t.Fatalf("decode final status: %v", err)
	}
	if finalStatus.EntityCount != 2 || finalStatus.EdgeCount != 1 {
		t.Fatalf("final status = %+v, want 2 entities and 1 edge", finalStatus)
	}
}

func TestKnowledgeReviewEndpointsBatchListAndAction(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	enableRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/enabled", strings.NewReader(`{"enabled":true}`))
	enableRequest.Header.Set("Content-Type", "application/json")
	enableRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(enableRecorder, enableRequest)
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want 200; body: %s", enableRecorder.Code, enableRecorder.Body.String())
	}

	entityRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/entities", strings.NewReader(`{"name":"Review Candidate","kind":"note","source":"manual QA","sourceKind":"document","sourceRef":"/Users/example/private.md token=secret-value","evidence":"ready for review"}`))
	entityRequest.Header.Set("Content-Type", "application/json")
	entityRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(entityRecorder, entityRequest)
	if entityRecorder.Code != http.StatusOK {
		t.Fatalf("entity status = %d, want 200; body: %s", entityRecorder.Code, entityRecorder.Body.String())
	}
	var entity KnowledgeEntityResult
	if err := json.Unmarshal(entityRecorder.Body.Bytes(), &entity); err != nil {
		t.Fatalf("decode entity: %v", err)
	}
	if entity.ReviewStatus != "pending" || entity.SourceKind != "document" {
		t.Fatalf("entity review/provenance = %+v, want pending document entity", entity)
	}
	if strings.Contains(entity.SourceRef, "/Users/example") || strings.Contains(entity.SourceRef, "secret-value") {
		t.Fatalf("entity source ref leaked: %q", entity.SourceRef)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/review?status=pending&limit=5", nil)
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var pending []KnowledgeReviewItemResult
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &pending); err != nil {
		t.Fatalf("decode review items: %v", err)
	}
	if len(pending) != 1 || pending[0].ID != entity.ID || pending[0].Type != "entity" {
		t.Fatalf("pending review items = %+v, want created entity", pending)
	}

	actionBody := fmt.Sprintf(`{"action":"approve","reviewedBy":"qa review","note":"checked","targets":[{"type":"entity","id":%q}]}`, entity.ID)
	actionRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/review", strings.NewReader(actionBody))
	actionRequest.Header.Set("Content-Type", "application/json")
	actionRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(actionRecorder, actionRequest)
	if actionRecorder.Code != http.StatusOK {
		t.Fatalf("action status = %d, want 200; body: %s", actionRecorder.Code, actionRecorder.Body.String())
	}
	var action KnowledgeReviewBatchResult
	if err := json.Unmarshal(actionRecorder.Body.Bytes(), &action); err != nil {
		t.Fatalf("decode review action: %v", err)
	}
	if action.EntitiesUpdated != 1 || len(action.Items) != 1 || action.Items[0].ReviewStatus != "approved" || action.Items[0].ReviewedBy != "qa_review" {
		t.Fatalf("review action = %+v", action)
	}

	searchRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/entities?query=Review%20Candidate&limit=5", nil)
	searchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200; body: %s", searchRecorder.Code, searchRecorder.Body.String())
	}
	var results []KnowledgeEntityResult
	if err := json.Unmarshal(searchRecorder.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode search results: %v", err)
	}
	if len(results) != 1 || results[0].ID != entity.ID || results[0].ReviewStatus != "approved" {
		t.Fatalf("explicit search after review = %+v, want same entity with review metadata", results)
	}
}

func TestKnowledgeRepairEndpointSanitizesExistingEvidence(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	enableRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/enabled", strings.NewReader(`{"enabled":true}`))
	enableRequest.Header.Set("Content-Type", "application/json")
	enableRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(enableRecorder, enableRequest)
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("enable status = %d, want 200; body: %s", enableRecorder.Code, enableRecorder.Body.String())
	}

	store, err := knowledge.Open(context.Background(), srv.deps.Config.Memory.Database)
	if err != nil {
		t.Fatalf("open knowledge store: %v", err)
	}
	entity, err := store.UpsertEntity(context.Background(), knowledge.EntityInput{
		Name:     "Leaked Credential",
		Kind:     "note",
		Source:   "manual QA",
		Evidence: "safe evidence",
	})
	if err != nil {
		_ = store.Close()
		t.Fatalf("seed entity: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close knowledge store: %v", err)
	}

	db, err := sql.Open("sqlite", srv.deps.Config.Memory.Database)
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	defer db.Close()
	leakedEvidence := "qa@example.com:super-secret-value from /Users/example/Secrets"
	if _, err := db.ExecContext(context.Background(), `UPDATE kg_entities SET evidence = ? WHERE id = ?`, leakedEvidence, entity.ID); err != nil {
		t.Fatalf("seed leaked entity evidence: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `UPDATE kg_entity_fts SET evidence = ? WHERE entity_id = ?`, leakedEvidence, entity.ID); err != nil {
		t.Fatalf("seed leaked entity fts: %v", err)
	}

	repairRequest := httptest.NewRequest(http.MethodPost, "/api/knowledge/repair", nil)
	repairRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(repairRecorder, repairRequest)
	if repairRecorder.Code != http.StatusOK {
		t.Fatalf("repair status = %d, want 200; body: %s", repairRecorder.Code, repairRecorder.Body.String())
	}
	var repair KnowledgeRepairResult
	if err := json.Unmarshal(repairRecorder.Body.Bytes(), &repair); err != nil {
		t.Fatalf("decode repair result: %v", err)
	}
	if repair.EntitiesScanned != 1 || repair.EntitiesUpdated != 1 || repair.Status.EntityCount != 1 {
		t.Fatalf("repair result = %+v, want one repaired entity", repair)
	}
	if !strings.Contains(repair.Message, "repaired 1") {
		t.Fatalf("repair message = %q, want repaired count", repair.Message)
	}

	searchRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/entities?query=super-secret-value&limit=5", nil)
	searchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK {
		t.Fatalf("search status = %d, want 200; body: %s", searchRecorder.Code, searchRecorder.Body.String())
	}
	var results []KnowledgeEntityResult
	if err := json.Unmarshal(searchRecorder.Body.Bytes(), &results); err != nil {
		t.Fatalf("decode search results: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("repaired FTS still finds leaked evidence: %+v", results)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/knowledge/entities?query=credential&limit=5", nil)
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var clean []KnowledgeEntityResult
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &clean); err != nil {
		t.Fatalf("decode clean results: %v", err)
	}
	if len(clean) != 1 {
		t.Fatalf("clean results = %+v, want repaired entity", clean)
	}
	for _, leaked := range []string{"qa@example.com", "super-secret-value", "/Users/example"} {
		if strings.Contains(clean[0].Evidence, leaked) {
			t.Fatalf("repaired evidence leaked %q: %q", leaked, clean[0].Evidence)
		}
	}
}

func TestToolCatalogEndpointReportsChatCapabilityTruth(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	request := httptest.NewRequest(http.MethodGet, "/api/tools/catalog?surface=chat-callable", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var entries []ToolCatalogEntryResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	byName := map[string]ToolCatalogEntryResult{}
	for _, entry := range entries {
		byName[entry.Name] = entry
	}
	if byName["read_file"].Status != string(tools.ToolStatusAvailable) || !byName["read_file"].ChatCallable {
		t.Fatalf("read_file entry = %+v, want chat available", byName["read_file"])
	}
	if byName["edit_file"].Status != string(tools.ToolStatusApprovalRequired) || !byName["edit_file"].RequiresApproval {
		t.Fatalf("edit_file entry = %+v, want approval-required", byName["edit_file"])
	}
	if byName["internet_search"].Status != string(tools.ToolStatusDisabledByPolicy) {
		t.Fatalf("internet_search entry = %+v, want disabled by policy", byName["internet_search"])
	}
	if byName["extension_run"].Status != string(tools.ToolStatusFuture) {
		t.Fatalf("extension_run entry = %+v, want future", byName["extension_run"])
	}
}

func TestWorkspaceGrantEndpointsGateExternalIngest(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "outside.md"), []byte("External Yemaka grant notes\n"), 0o644); err != nil {
		t.Fatalf("write external doc: %v", err)
	}

	blockedRequest := httptest.NewRequest(http.MethodPost, "/api/documents/ingest", strings.NewReader(`{"path":"`+external+`"}`))
	blockedRequest.Header.Set("Content-Type", "application/json")
	blockedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(blockedRecorder, blockedRequest)
	if blockedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("blocked ingest status = %d, want 400; body: %s", blockedRecorder.Code, blockedRecorder.Body.String())
	}
	if !strings.Contains(blockedRecorder.Body.String(), "workspace path is not granted") {
		t.Fatalf("blocked ingest body = %q, want workspace grant error", blockedRecorder.Body.String())
	}

	grantRequest := httptest.NewRequest(http.MethodPost, "/api/workspace/grants", strings.NewReader(`{"path":"`+external+`","label":"External docs"}`))
	grantRequest.Header.Set("Content-Type", "application/json")
	grantRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(grantRecorder, grantRequest)
	if grantRecorder.Code != http.StatusOK {
		t.Fatalf("grant status = %d, want 200; body: %s", grantRecorder.Code, grantRecorder.Body.String())
	}
	var grant WorkspaceGrantResult
	if err := json.Unmarshal(grantRecorder.Body.Bytes(), &grant); err != nil {
		t.Fatalf("decode grant: %v", err)
	}
	if grant.ID == "" || grant.Path != external || grant.Label != "External docs" {
		t.Fatalf("grant = %+v, want external grant", grant)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/workspace/grants", nil)
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var grants []WorkspaceGrantResult
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &grants); err != nil {
		t.Fatalf("decode grants: %v", err)
	}
	if len(grants) != 1 || grants[0].ID != grant.ID {
		t.Fatalf("grants = %+v, want granted workspace", grants)
	}

	ingestRequest := httptest.NewRequest(http.MethodPost, "/api/documents/ingest", strings.NewReader(`{"path":"`+external+`"}`))
	ingestRequest.Header.Set("Content-Type", "application/json")
	ingestRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(ingestRecorder, ingestRequest)
	if ingestRecorder.Code != http.StatusOK {
		t.Fatalf("ingest status = %d, want 200; body: %s", ingestRecorder.Code, ingestRecorder.Body.String())
	}
	var summary IngestSummary
	if err := json.Unmarshal(ingestRecorder.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode ingest summary: %v", err)
	}
	if summary.FilesIndexed == 0 || summary.ChunksCreated == 0 {
		t.Fatalf("summary = %+v, want indexed external doc", summary)
	}

	revokeRequest := httptest.NewRequest(http.MethodPost, "/api/workspace/grants/revoke", strings.NewReader(`{"match":"`+grant.ID+`"}`))
	revokeRequest.Header.Set("Content-Type", "application/json")
	revokeRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(revokeRecorder, revokeRequest)
	if revokeRecorder.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, want 200; body: %s", revokeRecorder.Code, revokeRecorder.Body.String())
	}

	blockedAgain := httptest.NewRequest(http.MethodPost, "/api/documents/ingest", strings.NewReader(`{"path":"`+external+`"}`))
	blockedAgain.Header.Set("Content-Type", "application/json")
	blockedAgainRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(blockedAgainRecorder, blockedAgain)
	if blockedAgainRecorder.Code != http.StatusBadRequest {
		t.Fatalf("blocked-again status = %d, want 400; body: %s", blockedAgainRecorder.Code, blockedAgainRecorder.Body.String())
	}
}

func TestDocumentUploadEndpointIngestsBrowserFile(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "browser-notes.md")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("# Browser Notes\nYemaka web upload codename is Silver Orchard.")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var summary IngestSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &summary); err != nil {
		t.Fatalf("decode upload summary: %v", err)
	}
	if summary.Root != uploadedDocumentRootForTest() || summary.FilesIndexed != 1 || summary.ChunksCreated == 0 {
		t.Fatalf("summary = %+v, want uploaded doc indexed", summary)
	}

	results, err := srv.deps.RAG.Search(context.Background(), "Silver Orchard", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 || results[0].Path != "uploads/browser-notes.md" {
		t.Fatalf("results = %+v, want uploaded browser-notes.md", results)
	}
}

func TestDocumentUploadEndpointPreservesBrowserFolderPath(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("relativePath", "folder/docs/browser-notes.md"); err != nil {
		t.Fatalf("WriteField() error = %v", err)
	}
	part, err := writer.CreateFormFile("file", "browser-notes.md")
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write([]byte("# Browser Folder Notes\nYemaka folder upload codename is Amber Signal.")); err != nil {
		t.Fatalf("write multipart file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/upload", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}

	results, err := srv.deps.RAG.Search(context.Background(), "Amber Signal", 5)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if len(results) == 0 || results[0].Path != "uploads/folder/docs/browser-notes.md" {
		t.Fatalf("results = %+v, want uploaded folder path", results)
	}
}

func TestDocumentSuggestionsEndpointListsWorkspaceFiles(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	docsDir := filepath.Join(srv.deps.Workspace, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("create docs dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(docsDir, "company.md"), []byte("# Company\n"), 0o644); err != nil {
		t.Fatalf("write company doc: %v", err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/documents/suggestions?path=./docs/co&limit=10", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("suggestions status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var suggestions []DocumentPathSuggestionResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &suggestions); err != nil {
		t.Fatalf("decode suggestions: %v", err)
	}
	if len(suggestions) != 1 || suggestions[0].Path != "docs/company.md" {
		t.Fatalf("suggestions = %+v, want docs/company.md", suggestions)
	}
}

func TestDocumentInventoryAndPruneMissingEndpointsAreExplicit(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	docsDir := filepath.Join(srv.deps.Workspace, "docs")
	if err := os.MkdirAll(docsDir, 0o755); err != nil {
		t.Fatalf("create docs dir: %v", err)
	}
	docPath := filepath.Join(docsDir, "inventory.md")
	if err := os.WriteFile(docPath, []byte("# Inventory\nYemaka inventory smoke.\n"), 0o644); err != nil {
		t.Fatalf("write inventory doc: %v", err)
	}
	if _, err := srv.deps.RAG.IngestPath(context.Background(), srv.deps.Profile.Name, srv.deps.Workspace, ragConfig(srv.deps.Config.RAG), workspaceLimits(srv.deps.Config.Workspace)); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}
	if err := os.Remove(docPath); err != nil {
		t.Fatalf("remove indexed doc: %v", err)
	}

	inventoryRequest := httptest.NewRequest(http.MethodGet, "/api/documents/inventory?limit=20", nil)
	inventoryRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(inventoryRecorder, inventoryRequest)
	if inventoryRecorder.Code != http.StatusOK {
		t.Fatalf("inventory status = %d, want 200; body: %s", inventoryRecorder.Code, inventoryRecorder.Body.String())
	}
	var inventory []DocumentInventoryResult
	if err := json.Unmarshal(inventoryRecorder.Body.Bytes(), &inventory); err != nil {
		t.Fatalf("decode inventory: %v", err)
	}
	var missing *DocumentInventoryResult
	for i := range inventory {
		if inventory[i].Path == "docs/inventory.md" {
			missing = &inventory[i]
			break
		}
	}
	if missing == nil || missing.Status != rag.DocumentStatusMissing || !missing.Missing {
		t.Fatalf("inventory = %+v, want missing docs/inventory.md", inventory)
	}

	previewRequest := httptest.NewRequest(http.MethodPost, "/api/documents/prune-missing", strings.NewReader(`{"confirm":false}`))
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(previewRecorder, previewRequest)
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status = %d, want 200; body: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview DocumentPruneResult
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if !preview.DryRun || preview.DocumentsMatched != 1 || preview.DocumentsRemoved != 0 {
		t.Fatalf("preview = %+v, want dry-run matched one without removal", preview)
	}

	confirmRequest := httptest.NewRequest(http.MethodPost, "/api/documents/prune-missing", strings.NewReader(`{"confirm":true}`))
	confirmRequest.Header.Set("Content-Type", "application/json")
	confirmRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(confirmRecorder, confirmRequest)
	if confirmRecorder.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, want 200; body: %s", confirmRecorder.Code, confirmRecorder.Body.String())
	}
	var confirmed DocumentPruneResult
	if err := json.Unmarshal(confirmRecorder.Body.Bytes(), &confirmed); err != nil {
		t.Fatalf("decode confirmed: %v", err)
	}
	if confirmed.DryRun || confirmed.DocumentsRemoved != 1 {
		t.Fatalf("confirmed = %+v, want explicit removal", confirmed)
	}
}

func uploadedDocumentRootForTest() string {
	return "browser_upload"
}

func TestDocumentEmbeddingIndexEndpointIsExplicit(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	disabledRequest := httptest.NewRequest(http.MethodPost, "/api/documents/embeddings/index", nil)
	disabledRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(disabledRecorder, disabledRequest)
	if disabledRecorder.Code != http.StatusBadRequest {
		t.Fatalf("disabled status = %d, want 400; body: %s", disabledRecorder.Code, disabledRecorder.Body.String())
	}
	if !strings.Contains(disabledRecorder.Body.String(), "embeddings are disabled") {
		t.Fatalf("disabled body = %q, want explicit disabled guidance", disabledRecorder.Body.String())
	}

	srv.deps.Config.RAG.Embeddings.Enabled = true
	srv.deps.Config.RAG.Embeddings.Model = "small:2b"
	if _, err := srv.deps.RAG.IngestPath(context.Background(), srv.deps.Profile.Name, srv.deps.Workspace, ragConfig(srv.deps.Config.RAG), workspaceLimits(srv.deps.Config.Workspace)); err != nil {
		t.Fatalf("IngestPath() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/documents/embeddings/index", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("index status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result EmbeddingIndexSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode embedding index: %v", err)
	}
	if !result.Enabled || result.Model != "small:2b" || result.Dimensions != 3 {
		t.Fatalf("result = %+v, want enabled small:2b dimensions 3", result)
	}
	if result.ChunksEmbedded == 0 {
		t.Fatalf("ChunksEmbedded = %d, want at least one embedded chunk", result.ChunksEmbedded)
	}
}

func TestSkillManagerEndpointsDisableEnableAndCreate(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	registry, err := skills.LoadRegistry([]string{filepath.Join("..", "..", "skills", "default"), srv.deps.Profile.Skills})
	if err != nil {
		t.Fatalf("LoadRegistry() error = %v", err)
	}
	srv.deps.Skills = registry

	disableRequest := httptest.NewRequest(http.MethodPost, "/api/skills/enabled", strings.NewReader(`{"name":"project_explainer","enabled":false}`))
	disableRequest.Header.Set("Content-Type", "application/json")
	disableRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(disableRecorder, disableRequest)
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable status code = %d, want 200; body: %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	var disabled SkillActionResult
	if err := json.Unmarshal(disableRecorder.Body.Bytes(), &disabled); err != nil {
		t.Fatalf("decode disabled skill: %v", err)
	}
	if disabled.Skill.Enabled {
		t.Fatal("disabled skill Enabled = true, want false")
	}

	validateRequest := httptest.NewRequest(http.MethodPost, "/api/skills/validate", strings.NewReader(`{"name":"project_explainer"}`))
	validateRequest.Header.Set("Content-Type", "application/json")
	validateRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(validateRecorder, validateRequest)
	if validateRecorder.Code != http.StatusOK {
		t.Fatalf("validate status code = %d, want 200; body: %s", validateRecorder.Code, validateRecorder.Body.String())
	}

	conversation, err := srv.deps.Memory.CreateConversation(context.Background(), "Run tests")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Run tests and explain failures",
		Model:          "small:2b",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Tests passed.",
		Model:          "small:2b",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveToolRun(context.Background(), memory.ToolRun{
		ConversationID: conversation.ID,
		ToolName:       "run_tests",
		Input:          map[string]any{"command": "go test ./..."},
		Output:         map[string]any{"status": "pass"},
		Status:         "completed",
		RiskLevel:      "low",
	}); err != nil {
		t.Fatalf("SaveToolRun() error = %v", err)
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/skills/create-from-session", strings.NewReader(`{"conversationId":"`+conversation.ID+`"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status code = %d, want 200; body: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var created SkillActionResult
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created skill: %v", err)
	}
	if created.Skill.Name == "" || created.Skill.Enabled || !created.Skill.Valid {
		t.Fatalf("created skill = %+v, want disabled valid skill pending review", created.Skill)
	}
}

func TestExtensionRegistryEndpoints(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	srv, cleanup := newTestServer(t)
	defer cleanup()
	writeServerExtension(t, filepath.Join(srv.deps.Profile.GeneratedExtensions, "website_monitor"))

	listRequest := httptest.NewRequest(http.MethodGet, "/api/extensions", nil)
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var items []extensions.Status
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &items); err != nil {
		t.Fatalf("decode extensions: %v", err)
	}
	if len(items) != 1 || items[0].Name != "website_monitor" || !items[0].Valid {
		t.Fatalf("items = %+v, want valid website_monitor", items)
	}

	validateRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/validate", strings.NewReader(`{"name":"website_monitor"}`))
	validateRequest.Header.Set("Content-Type", "application/json")
	validateRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(validateRecorder, validateRequest)
	if validateRecorder.Code != http.StatusOK {
		t.Fatalf("validate status = %d, want 200; body: %s", validateRecorder.Code, validateRecorder.Body.String())
	}

	registerRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/register", strings.NewReader(`{"name":"website_monitor"}`))
	registerRequest.Header.Set("Content-Type", "application/json")
	registerRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(registerRecorder, registerRequest)
	if registerRecorder.Code != http.StatusOK {
		t.Fatalf("register status = %d, want 200; body: %s", registerRecorder.Code, registerRecorder.Body.String())
	}

	runRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/run", strings.NewReader(`{"name":"website_monitor","input":{}}`))
	runRequest.Header.Set("Content-Type", "application/json")
	runRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(runRecorder, runRequest)
	if runRecorder.Code != http.StatusOK {
		t.Fatalf("run status = %d, want 200; body: %s", runRecorder.Code, runRecorder.Body.String())
	}
	var runResult extensions.RunResult
	if err := json.Unmarshal(runRecorder.Body.Bytes(), &runResult); err != nil {
		t.Fatalf("decode run result: %v", err)
	}
	if runResult.Status != "completed" || runResult.Output["summary"] != "server extension" {
		t.Fatalf("run result = %+v, want completed server extension", runResult)
	}

	disableRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/enabled", strings.NewReader(`{"name":"website_monitor","enabled":false}`))
	disableRequest.Header.Set("Content-Type", "application/json")
	disableRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(disableRecorder, disableRequest)
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want 200; body: %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	var disabled ExtensionActionResult
	if err := json.Unmarshal(disableRecorder.Body.Bytes(), &disabled); err != nil {
		t.Fatalf("decode disabled extension: %v", err)
	}
	if disabled.Extension.Enabled || disabled.Extension.Callable {
		t.Fatalf("disabled extension = %+v, want not callable", disabled.Extension)
	}
}

func TestExtensionGenerationEndpoints(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	srv, cleanup := newTestServer(t)
	defer cleanup()

	capabilityRequest := httptest.NewRequest(http.MethodPost, "/api/capabilities/propose", strings.NewReader(`{"request":"build a tool that summarizes CSV notes"}`))
	capabilityRequest.Header.Set("Content-Type", "application/json")
	capabilityRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(capabilityRecorder, capabilityRequest)
	if capabilityRecorder.Code != http.StatusOK {
		t.Fatalf("capability status = %d, want 200; body: %s", capabilityRecorder.Code, capabilityRecorder.Body.String())
	}
	var capability agent.CapabilityGapProposal
	if err := json.Unmarshal(capabilityRecorder.Body.Bytes(), &capability); err != nil {
		t.Fatalf("decode capability proposal: %v", err)
	}
	if capability.Kind != agent.CapabilityKindToolExtension || !capability.CanGenerate {
		t.Fatalf("capability = %+v, want generated tool extension route", capability)
	}

	capabilityGenerateRequest := httptest.NewRequest(http.MethodPost, "/api/capabilities/generate", strings.NewReader(`{"request":"build a tool that normalizes CSV","name":"capability_csv","approved":true,"runAfterGenerate":true,"runInput":{"task":"normalize rows"}}`))
	capabilityGenerateRequest.Header.Set("Content-Type", "application/json")
	capabilityGenerateRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(capabilityGenerateRecorder, capabilityGenerateRequest)
	if capabilityGenerateRecorder.Code != http.StatusOK {
		t.Fatalf("capability generate status = %d, want 200; body: %s", capabilityGenerateRecorder.Code, capabilityGenerateRecorder.Body.String())
	}
	var capabilityGenerated agent.CapabilityGenerationResult
	if err := json.Unmarshal(capabilityGenerateRecorder.Body.Bytes(), &capabilityGenerated); err != nil {
		t.Fatalf("decode capability generation: %v", err)
	}
	if capabilityGenerated.Generation.Extension.Name != "capability_csv" || capabilityGenerated.Generation.Tests.Status != "passed" || len(capabilityGenerated.NextSteps) == 0 {
		t.Fatalf("capability generation = %+v, want generated extension with next steps", capabilityGenerated)
	}
	if capabilityGenerated.Run == nil || capabilityGenerated.Run.Status != "completed" {
		t.Fatalf("capability run = %+v, want completed first run", capabilityGenerated.Run)
	}

	capabilityJobRequest := httptest.NewRequest(http.MethodPost, "/api/capabilities/generate", strings.NewReader(`{"request":"build a tool that cleans CSV rows","name":"capability_job","approved":true,"scheduleAfterGenerate":true,"scheduleType":"interval","scheduleExpr":"1h","scheduleInput":{"task":"scheduled cleanup"}}`))
	capabilityJobRequest.Header.Set("Content-Type", "application/json")
	capabilityJobRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(capabilityJobRecorder, capabilityJobRequest)
	if capabilityJobRecorder.Code != http.StatusOK {
		t.Fatalf("capability job status = %d, want 200; body: %s", capabilityJobRecorder.Code, capabilityJobRecorder.Body.String())
	}
	var capabilityJob agent.CapabilityGenerationResult
	if err := json.Unmarshal(capabilityJobRecorder.Body.Bytes(), &capabilityJob); err != nil {
		t.Fatalf("decode capability job generation: %v", err)
	}
	if capabilityJob.Schedule == nil || capabilityJob.Schedule.TargetName != "capability_job" || capabilityJob.Schedule.ScheduleType != scheduler.ScheduleInterval || capabilityJob.Schedule.Enabled {
		t.Fatalf("capability schedule = %+v, want disabled approved interval job for generated extension", capabilityJob.Schedule)
	}

	capabilityDefaultJobRequest := httptest.NewRequest(http.MethodPost, "/api/capabilities/generate", strings.NewReader(`{"request":"build a tool that organizes local notes","name":"capability_job_default","approved":true,"scheduleAfterGenerate":true,"scheduleType":"manual"}`))
	capabilityDefaultJobRequest.Header.Set("Content-Type", "application/json")
	capabilityDefaultJobRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(capabilityDefaultJobRecorder, capabilityDefaultJobRequest)
	if capabilityDefaultJobRecorder.Code != http.StatusOK {
		t.Fatalf("capability default job status = %d, want 200; body: %s", capabilityDefaultJobRecorder.Code, capabilityDefaultJobRecorder.Body.String())
	}
	var capabilityDefaultJob agent.CapabilityGenerationResult
	if err := json.Unmarshal(capabilityDefaultJobRecorder.Body.Bytes(), &capabilityDefaultJob); err != nil {
		t.Fatalf("decode default job generation: %v", err)
	}
	if capabilityDefaultJob.Schedule == nil || strings.TrimSpace(fmt.Sprint(capabilityDefaultJob.Schedule.Input["task"])) == "" {
		t.Fatalf("capability default schedule input = %+v, want generated task input", capabilityDefaultJob.Schedule)
	}

	capabilityDomainRequest := httptest.NewRequest(http.MethodPost, "/api/capabilities/generate", strings.NewReader(`{"request":"create a cron job that checks a webpage every hour and records whether it changed","name":"capability_domain_scope","allowedDomains":["https://example.com/path"],"approved":true}`))
	capabilityDomainRequest.Header.Set("Content-Type", "application/json")
	capabilityDomainRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(capabilityDomainRecorder, capabilityDomainRequest)
	if capabilityDomainRecorder.Code != http.StatusOK {
		t.Fatalf("capability domain status = %d, want 200; body: %s", capabilityDomainRecorder.Code, capabilityDomainRecorder.Body.String())
	}
	var capabilityDomain agent.CapabilityGenerationResult
	if err := json.Unmarshal(capabilityDomainRecorder.Body.Bytes(), &capabilityDomain); err != nil {
		t.Fatalf("decode capability domain generation: %v", err)
	}
	if capabilityDomain.Generation.Extension.Name != "capability_domain_scope" || capabilityDomain.Proposal.NetworkMode != "core_broker" || strings.Join(capabilityDomain.Proposal.AllowedDomains, ",") != "example.com" {
		t.Fatalf("capability domain generation = %+v, want brokered generated extension scoped to example.com", capabilityDomain)
	}

	proposeRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/propose", strings.NewReader(`{"request":"summarize local notes"}`))
	proposeRequest.Header.Set("Content-Type", "application/json")
	proposeRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(proposeRecorder, proposeRequest)
	if proposeRecorder.Code != http.StatusOK {
		t.Fatalf("propose status = %d, want 200; body: %s", proposeRecorder.Code, proposeRecorder.Body.String())
	}
	var proposal extensions.Proposal
	if err := json.Unmarshal(proposeRecorder.Body.Bytes(), &proposal); err != nil {
		t.Fatalf("decode proposal: %v", err)
	}
	if proposal.Name != "summarize_local_notes" || !proposal.RequiresApproval {
		t.Fatalf("proposal = %+v, want approved-gated safe proposal", proposal)
	}

	generateRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/generate", strings.NewReader(`{"name":"local_notes","description":"Summarize local notes","approved":true}`))
	generateRequest.Header.Set("Content-Type", "application/json")
	generateRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(generateRecorder, generateRequest)
	if generateRecorder.Code != http.StatusOK {
		t.Fatalf("generate status = %d, want 200; body: %s", generateRecorder.Code, generateRecorder.Body.String())
	}
	var generated extensions.GenerationResult
	if err := json.Unmarshal(generateRecorder.Body.Bytes(), &generated); err != nil {
		t.Fatalf("decode generation: %v", err)
	}
	if !generated.Extension.Callable || generated.Tests.Status != "passed" {
		t.Fatalf("generated = %+v, want callable with passing tests", generated)
	}

	inspectRequest := httptest.NewRequest(http.MethodGet, "/api/extensions/inspect?name=local_notes", nil)
	inspectRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(inspectRecorder, inspectRequest)
	if inspectRecorder.Code != http.StatusOK {
		t.Fatalf("inspect status = %d, want 200; body: %s", inspectRecorder.Code, inspectRecorder.Body.String())
	}
	var inspection extensions.PackageInspection
	if err := json.Unmarshal(inspectRecorder.Body.Bytes(), &inspection); err != nil {
		t.Fatalf("decode inspection: %v", err)
	}
	if inspection.Status != "passed" || inspection.EntrypointPath != "local_notes" {
		t.Fatalf("inspection = %+v, want passed local_notes entrypoint", inspection)
	}

	reviewRequest := httptest.NewRequest(http.MethodGet, "/api/extensions/review?name=local_notes", nil)
	reviewRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(reviewRecorder, reviewRequest)
	if reviewRecorder.Code != http.StatusOK {
		t.Fatalf("review status = %d, want 200; body: %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}
	var review extensions.Review
	if err := json.Unmarshal(reviewRecorder.Body.Bytes(), &review); err != nil {
		t.Fatalf("decode review: %v", err)
	}
	if !review.CanRun || review.ReuseCommand != `yemaka extension run local_notes '{"task":"Describe the task to run."}'` || len(review.Files) == 0 {
		t.Fatalf("review = %+v, want runnable reuse review with file previews", review)
	}
	if review.SampleInputJSON != `{"task":"Describe the task to run."}` || review.SampleInput["task"] != "Describe the task to run." {
		t.Fatalf("review sample input = %q / %+v, want task sample", review.SampleInputJSON, review.SampleInput)
	}

	testRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/test", strings.NewReader(`{"name":"local_notes"}`))
	testRequest.Header.Set("Content-Type", "application/json")
	testRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(testRecorder, testRequest)
	if testRecorder.Code != http.StatusOK {
		t.Fatalf("test status = %d, want 200; body: %s", testRecorder.Code, testRecorder.Body.String())
	}

	rollbackRequest := httptest.NewRequest(http.MethodPost, "/api/extensions/rollback", strings.NewReader(`{"nameOrSnapshot":"`+generated.Snapshot.ID+`"}`))
	rollbackRequest.Header.Set("Content-Type", "application/json")
	rollbackRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rollbackRecorder, rollbackRequest)
	if rollbackRecorder.Code != http.StatusOK {
		t.Fatalf("rollback status = %d, want 200; body: %s", rollbackRecorder.Code, rollbackRecorder.Body.String())
	}
}

func TestInternetEndpointsDefaultDisabled(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/internet/status", nil)
	statusRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var status internet.Status
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode internet status: %v", err)
	}
	if status.Enabled {
		t.Fatal("internet should be disabled by default")
	}
	if status.CrawlerAvailable || !status.CrawlerManualOnly || !status.CrawlerRequiresApproval {
		t.Fatalf("crawler default status = %+v, want unavailable manual-only approval-gated", status)
	}
	if !strings.Contains(status.CrawlerStatus, "disabled") {
		t.Fatalf("crawler status = %q, want disabled message", status.CrawlerStatus)
	}

	fetchRequest := httptest.NewRequest(http.MethodPost, "/api/internet/fetch", strings.NewReader(`{"url":"https://example.com","allowedDomains":["example.com"]}`))
	fetchRequest.Header.Set("Content-Type", "application/json")
	fetchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(fetchRecorder, fetchRequest)
	if fetchRecorder.Code != http.StatusBadRequest {
		t.Fatalf("fetch status = %d, want 400 while disabled; body: %s", fetchRecorder.Code, fetchRecorder.Body.String())
	}

	searchRequest := httptest.NewRequest(http.MethodPost, "/api/internet/search", strings.NewReader(`{"query":"local agents","taskApproved":true}`))
	searchRequest.Header.Set("Content-Type", "application/json")
	searchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusBadRequest {
		t.Fatalf("search status = %d, want 400 while provider disabled; body: %s", searchRecorder.Code, searchRecorder.Body.String())
	}

	crawlRequest := httptest.NewRequest(http.MethodPost, "/api/internet/crawl", strings.NewReader(`{"seedUrl":"https://example.com","taskApproved":true,"maxPages":1}`))
	crawlRequest.Header.Set("Content-Type", "application/json")
	crawlRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(crawlRecorder, crawlRequest)
	if crawlRecorder.Code != http.StatusBadRequest {
		t.Fatalf("crawl status = %d, want 400 while internet disabled; body: %s", crawlRecorder.Code, crawlRecorder.Body.String())
	}

	crawlsRequest := httptest.NewRequest(http.MethodGet, "/api/internet/crawls?limit=5", nil)
	crawlsRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(crawlsRecorder, crawlsRequest)
	if crawlsRecorder.Code != http.StatusOK {
		t.Fatalf("crawls status = %d, want 200; body: %s", crawlsRecorder.Code, crawlsRecorder.Body.String())
	}
	var crawls []internet.CrawlRunRecord
	if err := json.Unmarshal(crawlsRecorder.Body.Bytes(), &crawls); err != nil {
		t.Fatalf("decode crawl runs: %v", err)
	}
	if len(crawls) != 1 || crawls[0].Status != "blocked" || crawls[0].Fetched != 0 {
		t.Fatalf("crawl runs = %+v, want one blocked disabled run", crawls)
	}
	crawlRunRequest := httptest.NewRequest(http.MethodGet, "/api/internet/crawl-run?id="+crawls[0].RunID, nil)
	crawlRunRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(crawlRunRecorder, crawlRunRequest)
	if crawlRunRecorder.Code != http.StatusOK {
		t.Fatalf("crawl run status = %d, want 200; body: %s", crawlRunRecorder.Code, crawlRunRecorder.Body.String())
	}
	var crawlRun internet.CrawlRunRecord
	if err := json.Unmarshal(crawlRunRecorder.Body.Bytes(), &crawlRun); err != nil {
		t.Fatalf("decode crawl run: %v", err)
	}
	if crawlRun.RunID != crawls[0].RunID || crawlRun.Status != "blocked" {
		t.Fatalf("crawl run = %+v, want blocked run %q", crawlRun, crawls[0].RunID)
	}
}

func TestJobAndHeartbeatEndpoints(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/create", strings.NewReader(`{"scheduleType":"manual","targetType":"heartbeat","targetName":"heartbeat","approved":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var job scheduler.Job
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if job.ID == "" || !job.Approved || job.Enabled {
		t.Fatalf("job = %+v, want approved disabled job", job)
	}

	runRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/run", strings.NewReader(`{"id":"`+job.ID+`"}`))
	runRequest.Header.Set("Content-Type", "application/json")
	runRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(runRecorder, runRequest)
	if runRecorder.Code != http.StatusOK {
		t.Fatalf("run status = %d, want 200; body: %s", runRecorder.Code, runRecorder.Body.String())
	}
	var run scheduler.JobRun
	if err := json.Unmarshal(runRecorder.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.Status != scheduler.StatusCompleted {
		t.Fatalf("run = %+v, want completed", run)
	}

	heartbeatRequest := httptest.NewRequest(http.MethodGet, "/api/heartbeat/status", nil)
	heartbeatRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(heartbeatRecorder, heartbeatRequest)
	if heartbeatRecorder.Code != http.StatusOK {
		t.Fatalf("heartbeat status = %d, want 200; body: %s", heartbeatRecorder.Code, heartbeatRecorder.Body.String())
	}
	var report heartbeat.Report
	if err := json.Unmarshal(heartbeatRecorder.Body.Bytes(), &report); err != nil {
		t.Fatalf("decode heartbeat: %v", err)
	}
	if len(report.Checks) == 0 {
		t.Fatal("heartbeat should return checks")
	}
}

func TestJobArchiveEndpointHidesActiveJobAndBlocksRuns(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/create", strings.NewReader(`{"scheduleType":"interval","scheduleExpr":"1h","targetType":"heartbeat","targetName":"heartbeat","approved":true,"enabled":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, want 200; body: %s", createRecorder.Code, createRecorder.Body.String())
	}
	var job scheduler.Job
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job: %v", err)
	}
	if !job.Enabled {
		t.Fatalf("job = %+v, want enabled job before archive", job)
	}

	rejectedRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/archive", strings.NewReader(`{"id":"`+job.ID+`"}`))
	rejectedRequest.Header.Set("Content-Type", "application/json")
	rejectedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rejectedRecorder, rejectedRequest)
	if rejectedRecorder.Code != http.StatusBadRequest || !strings.Contains(rejectedRecorder.Body.String(), "approved=true") {
		t.Fatalf("archive without approval status/body = %d/%q, want approval guidance", rejectedRecorder.Code, rejectedRecorder.Body.String())
	}

	archiveRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/archive", strings.NewReader(`{"id":"`+job.ID+`","approved":true}`))
	archiveRequest.Header.Set("Content-Type", "application/json")
	archiveRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(archiveRecorder, archiveRequest)
	if archiveRecorder.Code != http.StatusOK {
		t.Fatalf("archive status = %d, want 200; body: %s", archiveRecorder.Code, archiveRecorder.Body.String())
	}
	var archived scheduler.Job
	if err := json.Unmarshal(archiveRecorder.Body.Bytes(), &archived); err != nil {
		t.Fatalf("decode archived job: %v", err)
	}
	if archived.ArchivedAt == "" || archived.Enabled || archived.NextDueAt != "" {
		t.Fatalf("archived job = %+v, want archived disabled job without next due", archived)
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var jobs []scheduler.Job
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs = %+v, want archived job hidden from active list", jobs)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/jobs/status", nil)
	statusRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var status scheduler.Status
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.TotalJobs != 0 || status.ArchivedJobs != 1 {
		t.Fatalf("status = %+v, want no active jobs and one archived job", status)
	}

	runRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/run", strings.NewReader(`{"id":"`+job.ID+`"}`))
	runRequest.Header.Set("Content-Type", "application/json")
	runRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(runRecorder, runRequest)
	if runRecorder.Code != http.StatusBadRequest || !strings.Contains(runRecorder.Body.String(), "archived job") {
		t.Fatalf("run archived status/body = %d/%q, want archived job error", runRecorder.Code, runRecorder.Body.String())
	}
}

func TestFailedJobRunCreatesNotification(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	store, err := srv.schedulerStore(context.Background())
	if err != nil {
		t.Fatalf("schedulerStore() error = %v", err)
	}
	job, err := store.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "missing_test_extension",
		Approved:     true,
	})
	store.Close()
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	runRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/run", strings.NewReader(`{"id":"`+job.ID+`"}`))
	runRequest.Header.Set("Content-Type", "application/json")
	runRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(runRecorder, runRequest)
	if runRecorder.Code != http.StatusBadRequest {
		t.Fatalf("run status = %d, want 400 for missing extension; body: %s", runRecorder.Code, runRecorder.Body.String())
	}

	runsRequest := httptest.NewRequest(http.MethodGet, "/api/jobs/runs", nil)
	runsRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(runsRecorder, runsRequest)
	if runsRecorder.Code != http.StatusOK {
		t.Fatalf("runs status = %d, want 200; body: %s", runsRecorder.Code, runsRecorder.Body.String())
	}
	var runs []scheduler.JobRun
	if err := json.Unmarshal(runsRecorder.Body.Bytes(), &runs); err != nil {
		t.Fatalf("decode runs: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != scheduler.StatusFailed || runs[0].ID == "" {
		t.Fatalf("runs = %+v, want one failed run", runs)
	}

	notificationsRequest := httptest.NewRequest(http.MethodGet, "/api/notifications", nil)
	notificationsRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(notificationsRecorder, notificationsRequest)
	if notificationsRecorder.Code != http.StatusOK {
		t.Fatalf("notifications status = %d, want 200; body: %s", notificationsRecorder.Code, notificationsRecorder.Body.String())
	}
	body := notificationsRecorder.Body.String()
	if !strings.Contains(body, `"type": "job_run"`) || !strings.Contains(body, `"source": "scheduler"`) || !strings.Contains(body, runs[0].ID) {
		t.Fatalf("notifications body missing failed job notification for %s: %s", runs[0].ID, body)
	}
}

func TestJobCreateRejectsMissingExtensionTarget(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/create", strings.NewReader(`{"scheduleType":"manual","targetType":"extension","targetName":"missing_test_extension","approved":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, want 400; body: %s", createRecorder.Code, createRecorder.Body.String())
	}
	if !strings.Contains(createRecorder.Body.String(), "extension target") || !strings.Contains(createRecorder.Body.String(), "is not installed") {
		t.Fatalf("create body = %q, want missing extension guidance", createRecorder.Body.String())
	}
}

func TestJobCreateRejectsURLLikeExtensionTarget(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/create", strings.NewReader(`{"scheduleType":"manual","targetType":"extension","targetName":"monitor https://example.com","approved":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, want 400; body: %s", createRecorder.Code, createRecorder.Body.String())
	}
	if !strings.Contains(createRecorder.Body.String(), "extension job target must be a generated extension name") {
		t.Fatalf("create body = %q, want extension target guidance", createRecorder.Body.String())
	}
}

func TestJobCreateRejectsKnownExtensionInvalidInput(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	srv, cleanup := newTestServer(t)
	defer cleanup()
	if _, err := srv.extensionStore().Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	createRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/create", strings.NewReader(`{"scheduleType":"manual","targetType":"extension","targetName":"needs_task","input":{},"approved":true,"enabled":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, want 400; body: %s", createRecorder.Code, createRecorder.Body.String())
	}
	if !strings.Contains(createRecorder.Body.String(), "input.task is required") || !strings.Contains(createRecorder.Body.String(), "Describe the task to run.") {
		t.Fatalf("create body = %q, want input schema guidance with sample", createRecorder.Body.String())
	}
}

func TestJobEnableRejectsKnownExtensionInvalidStoredInput(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	srv, cleanup := newTestServer(t)
	defer cleanup()
	if _, err := srv.extensionStore().Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_enable",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	store, err := srv.schedulerStore(context.Background())
	if err != nil {
		t.Fatalf("schedulerStore() error = %v", err)
	}
	defer store.Close()
	job, err := store.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_enable",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	enableRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/enabled", strings.NewReader(`{"id":"`+job.ID+`","enabled":true}`))
	enableRequest.Header.Set("Content-Type", "application/json")
	enableRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(enableRecorder, enableRequest)
	if enableRecorder.Code != http.StatusBadRequest {
		t.Fatalf("enable status = %d, want 400; body: %s", enableRecorder.Code, enableRecorder.Body.String())
	}
	if !strings.Contains(enableRecorder.Body.String(), "input.task is required") {
		t.Fatalf("enable body = %q, want input schema guidance", enableRecorder.Body.String())
	}
}

func TestJobUpdateInputValidatesAndSurfacesStoredInvalidExtensionInput(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	srv, cleanup := newTestServer(t)
	defer cleanup()
	if _, err := srv.extensionStore().Generate(context.Background(), extensions.GenerateInput{
		Name:        "needs_task_update",
		Description: "Handle a scheduled task.",
		Approved:    true,
	}); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	store, err := srv.schedulerStore(context.Background())
	if err != nil {
		t.Fatalf("schedulerStore() error = %v", err)
	}
	job, err := store.Create(context.Background(), scheduler.CreateInput{
		ScheduleType: scheduler.ScheduleManual,
		TargetType:   scheduler.TargetExtension,
		TargetName:   "needs_task_update",
		Input:        map[string]any{},
		Approved:     true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	store.Close()

	listRequest := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var jobs []scheduler.Job
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].InputStatus != scheduler.InputStatusInvalid || !strings.Contains(jobs[0].InputValidationError, "input.task is required") {
		t.Fatalf("jobs = %+v, want invalid input status", jobs)
	}

	statusRequest := httptest.NewRequest(http.MethodGet, "/api/jobs/status", nil)
	statusRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(statusRecorder, statusRequest)
	if statusRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	var status scheduler.Status
	if err := json.Unmarshal(statusRecorder.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if status.InvalidInputJobs != 1 {
		t.Fatalf("status = %+v, want one invalid input job", status)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/input", strings.NewReader(`{"id":"`+job.ID+`","input":{},"approved":true}`))
	invalidRequest.Header.Set("Content-Type", "application/json")
	invalidRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(invalidRecorder, invalidRequest)
	if invalidRecorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid update status = %d, want 400; body: %s", invalidRecorder.Code, invalidRecorder.Body.String())
	}

	updateRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/input", strings.NewReader(`{"id":"`+job.ID+`","input":{"task":"scheduled cleanup"},"approved":true}`))
	updateRequest.Header.Set("Content-Type", "application/json")
	updateRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200; body: %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated scheduler.Job
	if err := json.Unmarshal(updateRecorder.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated job: %v", err)
	}
	if updated.InputStatus != scheduler.InputStatusValid || updated.Input["task"] != "scheduled cleanup" || updated.Enabled {
		t.Fatalf("updated job = %+v, want valid disabled updated input", updated)
	}
}

func TestJobCreateWithoutApprovalIsRejectedAndCreatesNoJob(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	createRequest := httptest.NewRequest(http.MethodPost, "/api/jobs/create", strings.NewReader(`{"scheduleType":"manual","targetType":"heartbeat","targetName":"heartbeat"}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusBadRequest {
		t.Fatalf("create status = %d, want 400; body: %s", createRecorder.Code, createRecorder.Body.String())
	}
	if !strings.Contains(createRecorder.Body.String(), "new jobs require approval") {
		t.Fatalf("create body = %q, want approval guidance", createRecorder.Body.String())
	}

	listRequest := httptest.NewRequest(http.MethodGet, "/api/jobs", nil)
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	var jobs []scheduler.Job
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	if len(jobs) != 0 {
		t.Fatalf("jobs = %+v, want none after rejected create", jobs)
	}
}

func TestConnectorRegistryEndpointIncludesSecretReferences(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	request := httptest.NewRequest(http.MethodGet, "/api/connectors/registry", nil)
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var entries []connectors.RegistryEntry
	if err := json.Unmarshal(recorder.Body.Bytes(), &entries); err != nil {
		t.Fatalf("decode registry: %v", err)
	}
	if len(entries) != 6 {
		t.Fatalf("registry entries = %d, want 6", len(entries))
	}
	for _, entry := range entries {
		if entry.Manifest.Permissions.Posting || entry.Manifest.Permissions.Trading || entry.Manifest.Permissions.Deployment || entry.Manifest.Permissions.Mutating {
			t.Fatalf("%s manifest has mutating permissions", entry.Manifest.Name)
		}
		if entry.Status.RequireToken && entry.Status.Secret.Provider == "" {
			t.Fatalf("%s missing secret reference", entry.Status.Name)
		}
	}
}

func TestFeedbackReviewPromotionEndpointWritesSuggestionOnlyArtifact(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	trace := replay.NewTrace("Search local documents with token=super-secret-value")
	trace.ID = "trace-feedback-promotion"
	trace.Route = replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseInternet: true,
	}
	trace.Errors = []replay.TraceError{{
		Stage:   "routing",
		Code:    "wrong_source",
		Message: "wrong source: local documents should have been used, not internet",
	}}
	if _, err := srv.replayTraceStore().Save(trace); err != nil {
		t.Fatalf("save replay trace: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/feedback/review/promote", strings.NewReader(`{"traceId":"trace-feedback-promotion","reviewedBy":"qa"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("promote status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result struct {
		Path                 string `json:"path"`
		Category             string `json:"category"`
		PromotionMode        string `json:"promotionMode"`
		AutomaticTestWritten bool   `json:"automaticTestWritten"`
		Preview              string `json:"preview"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode promotion result: %v", err)
	}
	if result.Category != "wrong_source" || result.PromotionMode != "suggestion_only" || result.AutomaticTestWritten {
		t.Fatalf("promotion result = %+v, want wrong_source suggestion-only without auto test", result)
	}
	if !strings.HasPrefix(result.Path, filepath.Join(srv.deps.Profile.Root, "qa_review", "regression_promotions")+string(os.PathSeparator)) {
		t.Fatalf("promotion path = %q, want inside profile QA review directory", result.Path)
	}
	data, err := os.ReadFile(result.Path)
	if err != nil {
		t.Fatalf("read promotion artifact: %v", err)
	}
	text := string(data)
	if !strings.Contains(text, `automatic_test_written: false`) || !strings.Contains(text, `review_status: "reviewed_suggestion"`) {
		t.Fatalf("promotion artifact missing reviewed suggestion markers:\n%s", text)
	}
	if strings.Contains(text, "super-secret-value") || strings.Contains(result.Preview, "super-secret-value") {
		t.Fatalf("promotion leaked secret:\nresponse=%+v\nartifact=%s", result, text)
	}
}

func TestFeedbackReviewStatusEndpointPersistsReviewState(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	trace := replay.NewTrace("Search local documents")
	trace.ID = "trace-feedback-review-status"
	trace.Route = replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseInternet: true,
	}
	trace.Errors = []replay.TraceError{{
		Stage:   "routing",
		Code:    "wrong_source",
		Message: "wrong source: local documents should have been used, not internet",
	}}
	if _, err := srv.replayTraceStore().Save(trace); err != nil {
		t.Fatalf("save replay trace: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/feedback/review/status", strings.NewReader(`{"traceId":"trace-feedback-review-status","status":"approved","reviewedBy":"qa"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("review status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var record learning.QARegressionReviewRecord
	if err := json.Unmarshal(recorder.Body.Bytes(), &record); err != nil {
		t.Fatalf("decode review record: %v", err)
	}
	if record.Status != learning.QARegressionReviewApproved {
		t.Fatalf("record status = %q, want approved", record.Status)
	}
	if !strings.HasPrefix(record.Path, filepath.Join(srv.deps.Profile.Root, "qa_review", "regression_reviews")+string(os.PathSeparator)) {
		t.Fatalf("record path = %q, want inside profile QA review directory", record.Path)
	}

	feedbackRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(feedbackRecorder, httptest.NewRequest(http.MethodGet, "/api/feedback/review", nil))
	if feedbackRecorder.Code != http.StatusOK {
		t.Fatalf("feedback review status = %d, want 200; body: %s", feedbackRecorder.Code, feedbackRecorder.Body.String())
	}
	var review learning.QAReview
	if err := json.Unmarshal(feedbackRecorder.Body.Bytes(), &review); err != nil {
		t.Fatalf("decode feedback review: %v", err)
	}
	if review.Summary.ApprovedSuggestions != 1 {
		t.Fatalf("approved suggestions = %d, want 1", review.Summary.ApprovedSuggestions)
	}
	if len(review.ReplayFailures) != 1 {
		t.Fatalf("replay failures = %d, want 1", len(review.ReplayFailures))
	}
	failure := review.ReplayFailures[0]
	if failure.ReviewStatus != learning.QARegressionReviewApproved || failure.CoverageStatus != "approved_for_test" {
		t.Fatalf("failure review/coverage = %q/%q, want approved/approved_for_test", failure.ReviewStatus, failure.CoverageStatus)
	}
	if len(failure.RegressionSuggestions) == 0 || failure.RegressionSuggestions[0].ReviewStatus != learning.QARegressionReviewApproved {
		t.Fatalf("regression suggestion review status not approved: %+v", failure.RegressionSuggestions)
	}
}

func TestFeedbackReviewRegressionDraftEndpointsGateApprovedRegression(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	trace := replay.NewTrace("Search local documents")
	trace.ID = "trace-feedback-regression-draft"
	trace.Route = replay.RouteSnapshot{
		Category:          "internet_search",
		RiskLevel:         "medium",
		ShouldUseInternet: true,
	}
	trace.Errors = []replay.TraceError{{
		Stage:   "routing",
		Code:    "wrong_source",
		Message: "wrong source: local documents should have been used, not internet",
	}}
	if _, err := srv.replayTraceStore().Save(trace); err != nil {
		t.Fatalf("save replay trace: %v", err)
	}

	draftBeforeApproval := httptest.NewRequest(http.MethodPost, "/api/feedback/review/test-draft", strings.NewReader(`{"traceId":"trace-feedback-regression-draft","reviewedBy":"qa"}`))
	draftBeforeApproval.Header.Set("Content-Type", "application/json")
	blocked := httptest.NewRecorder()
	srv.Handler().ServeHTTP(blocked, draftBeforeApproval)
	if blocked.Code != http.StatusBadRequest {
		t.Fatalf("draft before approval status = %d, want 400; body: %s", blocked.Code, blocked.Body.String())
	}

	reviewRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/status", strings.NewReader(`{"traceId":"trace-feedback-regression-draft","status":"approved","reviewedBy":"qa"}`))
	reviewRequest.Header.Set("Content-Type", "application/json")
	reviewRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(reviewRecorder, reviewRequest)
	if reviewRecorder.Code != http.StatusOK {
		t.Fatalf("review status = %d, want 200; body: %s", reviewRecorder.Code, reviewRecorder.Body.String())
	}

	draftRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/test-draft", strings.NewReader(`{"traceId":"trace-feedback-regression-draft","reviewedBy":"qa"}`))
	draftRequest.Header.Set("Content-Type", "application/json")
	draftRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(draftRecorder, draftRequest)
	if draftRecorder.Code != http.StatusOK {
		t.Fatalf("draft status = %d, want 200; body: %s", draftRecorder.Code, draftRecorder.Body.String())
	}
	var draft learning.QARegressionTestDraftRecord
	if err := json.Unmarshal(draftRecorder.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode draft: %v", err)
	}
	if draft.Status != "drafted" || draft.AutomaticTestWritten {
		t.Fatalf("draft = %+v, want drafted/no auto test", draft)
	}

	approvalRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/regression-approval", strings.NewReader(`{"traceId":"trace-feedback-regression-draft","reviewedBy":"qa"}`))
	approvalRequest.Header.Set("Content-Type", "application/json")
	approvalRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(approvalRecorder, approvalRequest)
	if approvalRecorder.Code != http.StatusOK {
		t.Fatalf("approval status = %d, want 200; body: %s", approvalRecorder.Code, approvalRecorder.Body.String())
	}
	var approval learning.QARegressionApprovedRecord
	if err := json.Unmarshal(approvalRecorder.Body.Bytes(), &approval); err != nil {
		t.Fatalf("decode approval: %v", err)
	}
	if approval.Status != "approved_regression" || approval.AutomaticTestWritten {
		t.Fatalf("approval = %+v, want approved_regression/no auto test", approval)
	}

	sourcePatchRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/source-patch", strings.NewReader(`{"traceId":"trace-feedback-regression-draft","reviewedBy":"qa"}`))
	sourcePatchRequest.Header.Set("Content-Type", "application/json")
	sourcePatchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(sourcePatchRecorder, sourcePatchRequest)
	if sourcePatchRecorder.Code != http.StatusOK {
		t.Fatalf("source patch status = %d, want 200; body: %s", sourcePatchRecorder.Code, sourcePatchRecorder.Body.String())
	}
	var sourcePatch learning.QARegressionSourcePatchRecord
	if err := json.Unmarshal(sourcePatchRecorder.Body.Bytes(), &sourcePatch); err != nil {
		t.Fatalf("decode source patch: %v", err)
	}
	if sourcePatch.Status != "requires_manual_apply" || sourcePatch.AutomaticTestWritten || !sourcePatch.RequiresManualApply {
		t.Fatalf("source patch = %+v, want manual source patch without auto test", sourcePatch)
	}
	if !strings.HasPrefix(sourcePatch.Path, filepath.Join(srv.deps.Profile.Root, "qa_review", "regression_source_patches")+string(os.PathSeparator)) {
		t.Fatalf("source patch path = %q, want inside profile QA review directory", sourcePatch.Path)
	}
	sourcePatchData, err := os.ReadFile(sourcePatch.Path)
	if err != nil {
		t.Fatalf("read source patch artifact: %v", err)
	}
	if !strings.Contains(string(sourcePatchData), `"requiresManualApply": true`) || !strings.Contains(sourcePatch.PatchPreview, "diff --git") {
		t.Fatalf("source patch artifact missing manual/diff markers:\n%s\npreview=%s", string(sourcePatchData), sourcePatch.PatchPreview)
	}

	applyPlanBlockedRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/source-patch/apply-plan", strings.NewReader(`{"traceId":"trace-feedback-regression-draft","reviewedBy":"qa"}`))
	applyPlanBlockedRequest.Header.Set("Content-Type", "application/json")
	applyPlanBlockedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(applyPlanBlockedRecorder, applyPlanBlockedRequest)
	if applyPlanBlockedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("source patch apply plan without approval status = %d, want 400; body: %s", applyPlanBlockedRecorder.Code, applyPlanBlockedRecorder.Body.String())
	}

	applyPlanRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/source-patch/apply-plan", strings.NewReader(`{"traceId":"trace-feedback-regression-draft","approved":true,"reviewedBy":"qa"}`))
	applyPlanRequest.Header.Set("Content-Type", "application/json")
	applyPlanRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(applyPlanRecorder, applyPlanRequest)
	if applyPlanRecorder.Code != http.StatusOK {
		t.Fatalf("source patch apply plan status = %d, want 200; body: %s", applyPlanRecorder.Code, applyPlanRecorder.Body.String())
	}
	var applyPlan learning.QARegressionSourcePatchApplyPlanRecord
	if err := json.Unmarshal(applyPlanRecorder.Body.Bytes(), &applyPlan); err != nil {
		t.Fatalf("decode source patch apply plan: %v", err)
	}
	if applyPlan.Status != "approved_for_manual_apply" || applyPlan.AutomaticTestWritten || applyPlan.SourceTestWritten || !applyPlan.RequiresManualApply {
		t.Fatalf("source patch apply plan = %+v, want approved manual plan without source writes", applyPlan)
	}
	if !strings.HasPrefix(applyPlan.Path, filepath.Join(srv.deps.Profile.Root, "qa_review", "regression_source_patch_apply_plans")+string(os.PathSeparator)) {
		t.Fatalf("source patch apply plan path = %q, want inside profile QA review directory", applyPlan.Path)
	}

	targetPath := filepath.Join(srv.deps.Workspace, "internal", "routing", "routing_test.go")
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		t.Fatalf("create source test dir: %v", err)
	}
	sourceContent := "package routing\n\nimport \"testing\"\n\nfunc " + sourcePatch.SuggestedTestName + "(t *testing.T) {\n\tif false {\n\t\tt.Fatal(\"deterministic route assertion placeholder\")\n\t}\n}\n"
	planWriteRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/source-patch/source-write/plan", strings.NewReader(fmt.Sprintf(`{"traceId":"trace-feedback-regression-draft","content":%q,"reviewedBy":"qa"}`, sourceContent)))
	planWriteRequest.Header.Set("Content-Type", "application/json")
	planWriteRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(planWriteRecorder, planWriteRequest)
	if planWriteRecorder.Code != http.StatusOK {
		t.Fatalf("source write plan status = %d, want 200; body: %s", planWriteRecorder.Code, planWriteRecorder.Body.String())
	}
	if !strings.Contains(planWriteRecorder.Body.String(), `"requiresApproval": true`) || !strings.Contains(planWriteRecorder.Body.String(), sourcePatch.SuggestedTestName) {
		t.Fatalf("source write plan missing approval/test name:\n%s", planWriteRecorder.Body.String())
	}

	sourceWriteBlockedRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/source-patch/source-write/apply", strings.NewReader(fmt.Sprintf(`{"traceId":"trace-feedback-regression-draft","content":%q,"reviewedBy":"qa"}`, sourceContent)))
	sourceWriteBlockedRequest.Header.Set("Content-Type", "application/json")
	sourceWriteBlockedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(sourceWriteBlockedRecorder, sourceWriteBlockedRequest)
	if sourceWriteBlockedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("source write without approval status = %d, want 400; body: %s", sourceWriteBlockedRecorder.Code, sourceWriteBlockedRecorder.Body.String())
	}

	sourceWriteRequest := httptest.NewRequest(http.MethodPost, "/api/feedback/review/source-patch/source-write/apply", strings.NewReader(fmt.Sprintf(`{"traceId":"trace-feedback-regression-draft","content":%q,"approved":true,"reviewedBy":"qa"}`, sourceContent)))
	sourceWriteRequest.Header.Set("Content-Type", "application/json")
	sourceWriteRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(sourceWriteRecorder, sourceWriteRequest)
	if sourceWriteRecorder.Code != http.StatusOK {
		t.Fatalf("source write status = %d, want 200; body: %s", sourceWriteRecorder.Code, sourceWriteRecorder.Body.String())
	}
	var sourceWrite struct {
		Record learning.QARegressionSourceWriteRecord `json:"record"`
	}
	if err := json.Unmarshal(sourceWriteRecorder.Body.Bytes(), &sourceWrite); err != nil {
		t.Fatalf("decode source write: %v", err)
	}
	if sourceWrite.Record.Status != "source_test_written" || !sourceWrite.Record.SourceTestWritten || sourceWrite.Record.SnapshotID == "" {
		t.Fatalf("source write record = %+v, want source_test_written with snapshot", sourceWrite.Record)
	}
	written, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read written source test: %v", err)
	}
	if !strings.Contains(string(written), sourcePatch.SuggestedTestName) {
		t.Fatalf("written source test missing test name:\n%s", string(written))
	}

	feedbackRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(feedbackRecorder, httptest.NewRequest(http.MethodGet, "/api/feedback/review", nil))
	if feedbackRecorder.Code != http.StatusOK {
		t.Fatalf("feedback review status = %d, want 200; body: %s", feedbackRecorder.Code, feedbackRecorder.Body.String())
	}
	body := feedbackRecorder.Body.String()
	for _, want := range []string{
		`"testDrafts": 1`,
		`"approvedRegressions": 1`,
		`"sourcePatches": 1`,
		`"sourcePatchApplyPlans": 1`,
		`"sourceTestWrites": 1`,
		`"testDraftStatus": "drafted"`,
		`"approvedRegression": "approved_regression"`,
		`"sourcePatchStatus": "requires_manual_apply"`,
		`"sourcePatchApplyStatus": "approved_for_manual_apply"`,
		`"sourceTestWriteStatus": "source_test_written"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("feedback review missing %s:\n%s", want, body)
		}
	}
}

func TestPolicyLearningAndExtensionFailureEndpoints(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	policyRequest := httptest.NewRequest(http.MethodGet, "/api/policy", nil)
	policyRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(policyRecorder, policyRequest)
	if policyRecorder.Code != http.StatusOK {
		t.Fatalf("policy status = %d, want 200; body: %s", policyRecorder.Code, policyRecorder.Body.String())
	}
	var policy PolicyStatusResult
	if err := json.Unmarshal(policyRecorder.Body.Bytes(), &policy); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	if policy.Mode != "safe" || policy.FullAccess {
		t.Fatalf("policy = %+v, want safe mode", policy)
	}

	setPolicy := httptest.NewRequest(http.MethodPost, "/api/policy/mode", strings.NewReader(`{"mode":"full_access"}`))
	setPolicy.Header.Set("Content-Type", "application/json")
	setRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(setRecorder, setPolicy)
	if setRecorder.Code != http.StatusOK {
		t.Fatalf("set policy = %d, want 200; body: %s", setRecorder.Code, setRecorder.Body.String())
	}
	if srv.deps.Config.Security.Policy.Mode != "full_access" {
		t.Fatalf("policy mode = %q, want full_access", srv.deps.Config.Security.Policy.Mode)
	}

	conversation, err := srv.deps.Memory.CreateConversation(context.Background(), "Learning")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Explain local parity",
		Model:          "small:2b",
	}); err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	if _, err := srv.deps.Memory.SaveMessage(context.Background(), memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "Desktop, web, and TUI share the core.",
		Model:          "small:2b",
	}); err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	correctionRequest := httptest.NewRequest(http.MethodPost, "/api/learn/correction", strings.NewReader(`{"conversationId":"`+conversation.ID+`","correction":"Prefer parity checks before expansion."}`))
	correctionRequest.Header.Set("Content-Type", "application/json")
	correctionRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(correctionRecorder, correctionRequest)
	if correctionRecorder.Code != http.StatusOK {
		t.Fatalf("correction status = %d, want 200; body: %s", correctionRecorder.Code, correctionRecorder.Body.String())
	}
	var correction MemoryResult
	if err := json.Unmarshal(correctionRecorder.Body.Bytes(), &correction); err != nil {
		t.Fatalf("decode correction: %v", err)
	}
	if correction.Kind != "correction" {
		t.Fatalf("correction kind = %q, want correction", correction.Kind)
	}

	exportRequest := httptest.NewRequest(http.MethodPost, "/api/learn/export", strings.NewReader(`{"conversationId":"`+conversation.ID+`"}`))
	exportRequest.Header.Set("Content-Type", "application/json")
	exportRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(exportRecorder, exportRequest)
	if exportRecorder.Code != http.StatusOK {
		t.Fatalf("export status = %d, want 200; body: %s", exportRecorder.Code, exportRecorder.Body.String())
	}
	if !strings.Contains(exportRecorder.Body.String(), `"schema": "yemaka.trajectory.v1"`) {
		t.Fatalf("export body missing schema: %s", exportRecorder.Body.String())
	}

	reportRequest := httptest.NewRequest(http.MethodGet, "/api/learn/report", nil)
	reportRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(reportRecorder, reportRequest)
	if reportRecorder.Code != http.StatusOK {
		t.Fatalf("report status = %d, want 200; body: %s", reportRecorder.Code, reportRecorder.Body.String())
	}
	if !strings.Contains(reportRecorder.Body.String(), `"corrections": 1`) {
		t.Fatalf("report body missing correction count: %s", reportRecorder.Body.String())
	}

	routeCorrectionBody := `{"conversationId":"` + conversation.ID + `","pattern":"email address in notebook","intendedRouteCategory":"rag_search","intendedTaskType":"rag","requiredTools":["rag_search"],"forbiddenTools":["internet_search"],"tags":["source:local_documents"],"originalPrompt":"search @gmail.com from indexed document","correctionText":"No, use local documents."}`
	routeCorrectionRequest := httptest.NewRequest(http.MethodPost, "/api/learn/route-correction", strings.NewReader(routeCorrectionBody))
	routeCorrectionRequest.Header.Set("Content-Type", "application/json")
	routeCorrectionRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(routeCorrectionRecorder, routeCorrectionRequest)
	if routeCorrectionRecorder.Code != http.StatusOK {
		t.Fatalf("route correction status = %d, want 200; body: %s", routeCorrectionRecorder.Code, routeCorrectionRecorder.Body.String())
	}
	var routeCorrection routing.RouteCorrection
	if err := json.Unmarshal(routeCorrectionRecorder.Body.Bytes(), &routeCorrection); err != nil {
		t.Fatalf("decode route correction: %v", err)
	}
	if routeCorrection.ApprovalStatus != routing.RouteCorrectionStatusPending {
		t.Fatalf("route correction status = %q, want pending", routeCorrection.ApprovalStatus)
	}
	if len(routeCorrection.Tags) != 1 || routeCorrection.Tags[0] != "source:local_documents" || routeCorrection.OriginalPrompt == "" {
		t.Fatalf("route correction metadata = %+v, want tags and original prompt", routeCorrection)
	}
	approveRequest := httptest.NewRequest(http.MethodPost, "/api/learn/route-correction/approve", strings.NewReader(`{"id":"`+routeCorrection.ID+`"}`))
	approveRequest.Header.Set("Content-Type", "application/json")
	approveRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(approveRecorder, approveRequest)
	if approveRecorder.Code != http.StatusOK {
		t.Fatalf("approve route correction status = %d, want 200; body: %s", approveRecorder.Code, approveRecorder.Body.String())
	}
	listRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/learn/route-corrections", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("route correction list status = %d, want 200; body: %s", listRecorder.Code, listRecorder.Body.String())
	}
	if !strings.Contains(listRecorder.Body.String(), `"approvalStatus": "approved"`) {
		t.Fatalf("route correction list missing approved correction: %s", listRecorder.Body.String())
	}
	feedbackRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(feedbackRecorder, httptest.NewRequest(http.MethodGet, "/api/feedback/review", nil))
	if feedbackRecorder.Code != http.StatusOK {
		t.Fatalf("feedback review status = %d, want 200; body: %s", feedbackRecorder.Code, feedbackRecorder.Body.String())
	}
	if !strings.Contains(feedbackRecorder.Body.String(), `"routeCorrections": 1`) || !strings.Contains(feedbackRecorder.Body.String(), `"routeCorrections": [`) {
		t.Fatalf("feedback review missing route correction summary/items: %s", feedbackRecorder.Body.String())
	}

	if err := os.MkdirAll(srv.deps.Profile.Logs, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	runLine := `{"extension":"broken_tool","status":"failed","error":"boom","completedAt":"2026-05-08T00:00:00Z"}`
	if err := os.WriteFile(filepath.Join(srv.deps.Profile.Logs, "extension_runs.jsonl"), []byte(runLine+"\n"+runLine+"\n"), 0o644); err != nil {
		t.Fatalf("write run log: %v", err)
	}
	failuresRequest := httptest.NewRequest(http.MethodGet, "/api/extensions/failures", nil)
	failuresRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(failuresRecorder, failuresRequest)
	if failuresRecorder.Code != http.StatusOK {
		t.Fatalf("failures status = %d, want 200; body: %s", failuresRecorder.Code, failuresRecorder.Body.String())
	}
	if !strings.Contains(failuresRecorder.Body.String(), `"name": "broken_tool"`) || !strings.Contains(failuresRecorder.Body.String(), `"suggestReview": true`) {
		t.Fatalf("failure body missing trend: %s", failuresRecorder.Body.String())
	}
}

func TestSettingsEndpointGuardsOptionalFeatures(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	badCloud := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{
		"lowMemoryMode": true,
		"maxContextTokens": 4096,
		"ragEnabled": true,
		"embeddingsEnabled": false,
		"cloudFallbackEnabled": true,
		"cloudFallbackBaseUrl": "http://localhost:4000/v1",
		"cloudFallbackModel": "fallback",
		"cloudFallbackKeyEnv": "sk-not-an-env-name",
		"connectorEnabled": false
	}`))
	badCloud.Header.Set("Content-Type", "application/json")
	badRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(badRecorder, badCloud)
	if badRecorder.Code != http.StatusBadRequest {
		t.Fatalf("bad cloud status = %d, want 400; body: %s", badRecorder.Code, badRecorder.Body.String())
	}

	save := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{
		"lowMemoryMode": true,
		"maxContextTokens": 4096,
		"responseMode": "deep",
		"showThinkingTrace": true,
		"uiTheme": "dark",
		"ragEnabled": true,
		"embeddingsEnabled": true,
		"embeddingModel": "nomic-embed-text",
		"cloudFallbackEnabled": false,
		"cloudFallbackBaseUrl": "http://localhost:4000/v1",
		"cloudFallbackModel": "fallback",
		"cloudFallbackKeyEnv": "YEMAKA_CLOUD_KEY",
		"connectorEnabled": true,
		"connectorTokenEnv": "YEMAKA_TEST_CONNECTOR_TOKEN"
	}`))
	save.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, save)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save settings status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var settings SettingsView
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if !settings.RAG.Embeddings.Enabled {
		t.Fatal("embeddings not enabled")
	}
	if settings.RAG.Embeddings.Model != "nomic-embed-text" {
		t.Fatalf("embedding model = %q, want nomic-embed-text", settings.RAG.Embeddings.Model)
	}
	if !settings.Connectors.Enabled || !settings.Connectors.LocalAPI.Enabled {
		t.Fatal("local connector not enabled")
	}
	if settings.CloudFallback.Enabled {
		t.Fatal("cloud fallback enabled unexpectedly")
	}
	if settings.UI.Theme != "dark" {
		t.Fatalf("ui theme = %q, want dark", settings.UI.Theme)
	}
	if settings.SettingsSchemaVersion < 2 {
		t.Fatalf("settings schema version = %d, want >= 2", settings.SettingsSchemaVersion)
	}
	if !settings.FeatureSupport.ResponseMode || !settings.FeatureSupport.ShowThinkingTrace {
		t.Fatalf("settings feature support = %+v, want response mode and thinking trace", settings.FeatureSupport)
	}
	if settings.ResponseMode != config.ResponseModeDeep {
		t.Fatalf("response mode = %q, want deep", settings.ResponseMode)
	}
	if !settings.ShowThinkingTrace {
		t.Fatal("show thinking trace not retained")
	}
	persisted, err := config.LoadOrCreate()
	if err != nil {
		t.Fatalf("reload persisted config: %v", err)
	}
	if persisted.Runtime.ResponseMode != config.ResponseModeDeep {
		t.Fatalf("persisted response mode = %q, want deep", persisted.Runtime.ResponseMode)
	}
	if !persisted.Runtime.ShowThinkingTrace {
		t.Fatal("persisted show thinking trace = false, want true")
	}
}

func TestSettingsEndpointAcceptsSettingsViewRoundTrip(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	get := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
	getRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(getRecorder, get)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("get settings status = %d, want 200; body: %s", getRecorder.Code, getRecorder.Body.String())
	}

	save := httptest.NewRequest(http.MethodPost, "/api/settings", bytes.NewReader(getRecorder.Body.Bytes()))
	save.Header.Set("Content-Type", "application/json")
	saveRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(saveRecorder, save)
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("save round-trip status = %d, want 200; body: %s", saveRecorder.Code, saveRecorder.Body.String())
	}
	var settings SettingsView
	if err := json.Unmarshal(saveRecorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if settings.MaxContextTokens <= 0 {
		t.Fatalf("MaxContextTokens = %d, want retained runtime setting", settings.MaxContextTokens)
	}
	if settings.ResponseMode != config.ResponseModeBalanced {
		t.Fatalf("ResponseMode = %q, want balanced default", settings.ResponseMode)
	}
	if settings.ShowThinkingTrace {
		t.Fatal("ShowThinkingTrace = true, want false default")
	}
}

func TestSettingsEndpointAcceptsBrowserCoercedSettingsPayload(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	save := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{
		"lowMemoryMode": "false",
		"maxContextTokens": "6144",
		"responseMode": "fast",
		"showThinkingTrace": "false",
		"uiTheme": "system",
		"ragEnabled": "true",
		"embeddingsEnabled": "false",
		"embeddingModel": "nomic-embed-text",
		"cloudFallbackEnabled": "false",
		"cloudFallbackBaseUrl": "http://localhost:4000/v1",
		"cloudFallbackModel": "",
		"cloudFallbackKeyEnv": "",
		"connectorEnabled": "false",
		"connectorTokenEnv": "YEMAKA_CONNECTOR_TOKEN",
		"mcpConnectorEnabled": "false",
		"slackConnectorEnabled": "false",
		"slackConnectorTokenEnv": "YEMAKA_SLACK_CONNECTOR_TOKEN",
		"discordConnectorEnabled": "false",
		"discordConnectorTokenEnv": "YEMAKA_DISCORD_CONNECTOR_TOKEN",
		"telegramConnectorEnabled": "false",
		"telegramConnectorTokenEnv": "YEMAKA_TELEGRAM_CONNECTOR_TOKEN",
		"emailConnectorEnabled": "false",
		"emailConnectorTokenEnv": "YEMAKA_EMAIL_CONNECTOR_TOKEN",
		"internetEnabled": "true",
		"internetSearchEnabled": "true",
		"internetSearchProvider": "tavily",
		"internetSearchEndpoint": "",
		"internetSearchApiKeyEnv": "TAVILY_API_KEY",
		"knowledgeInfluenceEnabled": "false",
		"futureFrontendField": "ignored"
	}`))
	save.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, save)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save settings status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var settings SettingsView
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if settings.LowMemoryMode {
		t.Fatal("LowMemoryMode = true, want false")
	}
	if settings.MaxContextTokens != 6144 {
		t.Fatalf("MaxContextTokens = %d, want 6144", settings.MaxContextTokens)
	}
	if settings.ResponseMode != config.ResponseModeFast {
		t.Fatalf("ResponseMode = %q, want fast", settings.ResponseMode)
	}
	if !settings.Internet.Enabled || !settings.Internet.Search.Enabled || settings.Internet.Search.Provider != "tavily" {
		t.Fatalf("internet search settings not retained: %+v", settings.Internet.Search)
	}
}

func TestSettingsEndpointAcceptsSnakeCaseSettingsPayload(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	save := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{
		"low_memory_mode": true,
		"max_context_tokens": 4096,
		"response_mode": "deep",
		"show_thinking_trace": false,
		"ui_theme": "dark",
		"rag_enabled": true,
		"embeddings_enabled": false,
		"embedding_model": "nomic-embed-text",
		"cloud_fallback_enabled": false,
		"cloud_fallback_base_url": "http://localhost:4000/v1",
		"cloud_fallback_model": "",
		"cloud_fallback_key_env": "",
		"connector_enabled": false,
		"connector_token_env": "YEMAKA_CONNECTOR_TOKEN",
		"mcp_connector_enabled": false,
		"slack_connector_enabled": false,
		"slack_connector_token_env": "YEMAKA_SLACK_CONNECTOR_TOKEN",
		"discord_connector_enabled": false,
		"discord_connector_token_env": "YEMAKA_DISCORD_CONNECTOR_TOKEN",
		"telegram_connector_enabled": false,
		"telegram_connector_token_env": "YEMAKA_TELEGRAM_CONNECTOR_TOKEN",
		"email_connector_enabled": false,
		"email_connector_token_env": "YEMAKA_EMAIL_CONNECTOR_TOKEN",
		"internet_enabled": false,
		"internet_search_enabled": false,
		"internet_search_provider": "none",
		"internet_search_endpoint": "",
		"internet_search_api_key_env": "",
		"knowledge_influence_enabled": false
	}`))
	save.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, save)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save settings status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var settings SettingsView
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if settings.ResponseMode != config.ResponseModeDeep {
		t.Fatalf("ResponseMode = %q, want deep", settings.ResponseMode)
	}
	if !settings.LowMemoryMode {
		t.Fatal("LowMemoryMode = false, want true")
	}
}

func TestSettingsEndpointPreservesSearchProviderWhenSearchDisabled(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	save := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{
		"lowMemoryMode": true,
		"maxContextTokens": 4096,
		"ragEnabled": true,
		"embeddingsEnabled": false,
		"internetEnabled": false,
		"internetSearchEnabled": false,
		"internetSearchProvider": "firecrawl",
		"internetSearchEndpoint": "https://api.firecrawl.dev/v2/search",
		"internetSearchApiKeyEnv": "FIRECRAWL_API_KEY"
	}`))
	save.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, save)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save settings status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var settings SettingsView
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if settings.Internet.Enabled || settings.Internet.Search.Enabled {
		t.Fatalf("internet/search enabled = %t/%t, want disabled defaults", settings.Internet.Enabled, settings.Internet.Search.Enabled)
	}
	if settings.Internet.Search.Provider != "firecrawl" {
		t.Fatalf("search provider = %q, want firecrawl preserved for later enablement", settings.Internet.Search.Provider)
	}
	if settings.Internet.Search.APIKeyEnv != "FIRECRAWL_API_KEY" {
		t.Fatalf("api key env = %q, want FIRECRAWL_API_KEY", settings.Internet.Search.APIKeyEnv)
	}
}

func TestSettingsEndpointNormalizesStaleSearchProviderDefaultEnv(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	save := httptest.NewRequest(http.MethodPost, "/api/settings", strings.NewReader(`{
		"lowMemoryMode": true,
		"maxContextTokens": 4096,
		"ragEnabled": true,
		"embeddingsEnabled": false,
		"internetEnabled": true,
		"internetSearchEnabled": true,
		"internetSearchProvider": "firecrawl",
		"internetSearchApiKeyEnv": "TAVILY_API_KEY"
	}`))
	save.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, save)
	if recorder.Code != http.StatusOK {
		t.Fatalf("save settings status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var settings SettingsView
	if err := json.Unmarshal(recorder.Body.Bytes(), &settings); err != nil {
		t.Fatalf("decode settings: %v", err)
	}
	if settings.Internet.Search.Provider != "firecrawl" {
		t.Fatalf("search provider = %q, want firecrawl", settings.Internet.Search.Provider)
	}
	if settings.Internet.Search.APIKeyEnv != "FIRECRAWL_API_KEY" {
		t.Fatalf("api key env = %q, want FIRECRAWL_API_KEY", settings.Internet.Search.APIKeyEnv)
	}
}

func TestPermissionDecisionEndpointLogsDecision(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"request": {
			"request_id": "perm_test",
			"tool_name": "edit_file",
			"risk_level": "medium",
			"reason": "file edits require confirmation",
			"requires_confirmation": true,
			"workspace_only": true,
			"diff_preview": true,
			"snapshot_before_write": true,
			"rollback_supported": true,
			"destructive": false,
			"next_step": "Review the diff preview."
		},
		"decision": "rejected"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result PermissionDecisionResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode permission decision: %v", err)
	}
	if result.Status != "rejected" {
		t.Fatalf("Status = %q, want rejected", result.Status)
	}

	runs, err := srv.deps.Memory.ListToolRuns(context.Background(), 5)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("tool run count = %d, want 1", len(runs))
	}
	if runs[0].ToolName != "permission_decision" {
		t.Fatalf("ToolName = %q, want permission_decision", runs[0].ToolName)
	}
}

func savePendingPermissionRequestForServerTest(t *testing.T, srv *Server, request agent.PermissionRequest) {
	t.Helper()
	_, err := srv.deps.Memory.SaveToolRun(context.Background(), memory.ToolRun{
		ToolName:  "permission_request",
		Input:     map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:    request,
		Status:    agent.ExecutionNeedsConfirmation,
		RiskLevel: request.RiskLevel,
	})
	if err != nil {
		t.Fatalf("SaveToolRun(permission_request) error = %v", err)
	}
}

func TestPermissionDecisionApprovedRunsConcreteTool(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	savePendingPermissionRequestForServerTest(t, srv, agent.PermissionRequest{
		RequestID:            "perm_project_map",
		ToolName:             "project_map",
		RiskLevel:            "medium",
		Reason:               "user approved project map",
		RequiresConfirmation: true,
		WorkspaceOnly:        true,
		NextStep:             "Run project map.",
	})
	body := strings.NewReader(`{
		"request": {
			"request_id": "perm_project_map",
			"tool_name": "project_map",
			"risk_level": "medium",
			"reason": "user approved project map",
			"requires_confirmation": true,
			"workspace_only": true,
			"next_step": "Run project map."
		},
		"decision": "approved"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result PermissionDecisionResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode permission decision: %v", err)
	}
	if result.Status != "approved" {
		t.Fatalf("Status = %q, want approved", result.Status)
	}
	if !result.Executed {
		t.Fatalf("Executed = false, want true; message: %s", result.Message)
	}
	if result.ToolName != "project_map" {
		t.Fatalf("ToolName = %q, want project_map", result.ToolName)
	}
	if !strings.Contains(result.Message, "I ran `project_map`") {
		t.Fatalf("Message = %q, want run summary", result.Message)
	}

	runs, err := srv.deps.Memory.ListToolRuns(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var sawDecision, sawProjectMap bool
	for _, run := range runs {
		if run.ToolName == "permission_decision" {
			sawDecision = true
		}
		if run.ToolName == "project_map" && run.Status == "completed" {
			sawProjectMap = true
		}
	}
	if !sawDecision || !sawProjectMap {
		t.Fatalf("tool runs missing decision/project_map: %#v", runs)
	}
}

func TestPermissionDecisionApprovedAppliesStoredEditProposal(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	request := agent.PermissionRequest{
		RequestID:            "perm_edit_apply",
		ToolName:             "edit_file",
		RiskLevel:            "medium",
		Reason:               "file edits require confirmation and snapshot flow",
		RequiresConfirmation: true,
		WorkspaceOnly:        true,
		DiffPreview:          true,
		SnapshotBeforeWrite:  true,
		RollbackSupported:    true,
		NextStep:             "Review the diff preview, then approve to create a snapshot and apply the workspace-only edit.",
	}
	savePendingPermissionRequestForServerTest(t, srv, request)
	_, err := srv.deps.Memory.SaveToolRun(context.Background(), memory.ToolRun{
		ToolName:  "edit_proposal",
		Input:     map[string]any{"request_id": request.RequestID, "path": "notes.md"},
		Output:    agent.EditProposal{RequestID: request.RequestID, Path: "notes.md", Content: "Permission approval works.\n", Status: "ready_for_preview", DiffPreview: true, SnapshotBeforeWrite: true, RollbackSupported: true},
		Status:    "ready_for_preview",
		RiskLevel: "medium",
	})
	if err != nil {
		t.Fatalf("SaveToolRun(edit_proposal) error = %v", err)
	}
	payload := map[string]any{"request": request, "decision": "approved"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	httpRequest := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", bytes.NewReader(data))
	httpRequest.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httpRequest)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result PermissionDecisionResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode permission decision: %v", err)
	}
	if !result.Executed || result.ToolName != "edit_file" || result.ToolStatus != "completed" {
		t.Fatalf("result = %+v, want executed completed edit_file", result)
	}
	content, err := os.ReadFile(filepath.Join(srv.deps.Workspace, "notes.md"))
	if err != nil {
		t.Fatalf("read notes.md: %v", err)
	}
	if string(content) != "Permission approval works.\n" {
		t.Fatalf("notes.md = %q, want approved content", string(content))
	}
	runs, err := srv.deps.Memory.ListToolRuns(context.Background(), 20)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var sawWrite, sawVerifier bool
	for _, run := range runs {
		if run.ToolName == "write_file" && run.Status == "completed" {
			sawWrite = true
		}
		if run.ToolName == "write_verifier" && run.Status == "pass" {
			sawVerifier = true
		}
	}
	if !sawWrite || !sawVerifier {
		t.Fatalf("tool runs missing write/verifier: %+v", runs)
	}
}

func TestPermissionDecisionApprovedPersistsConversationResult(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	ctx := context.Background()

	conversation, err := srv.deps.Memory.CreateConversation(ctx, "Create file")
	if err != nil {
		t.Fatalf("CreateConversation() error = %v", err)
	}
	user, err := srv.deps.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "Create notes.md",
		Model:          "yemaka",
	})
	if err != nil {
		t.Fatalf("SaveMessage(user) error = %v", err)
	}
	assistant, err := srv.deps.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: conversation.ID,
		Role:           "assistant",
		Content:        "I can do this, but edit_file needs your approval first.",
		Model:          "yemaka-executor",
		ParentID:       user.ID,
	})
	if err != nil {
		t.Fatalf("SaveMessage(assistant) error = %v", err)
	}

	request := agent.PermissionRequest{
		RequestID:            "perm_edit_persist",
		ToolName:             "edit_file",
		RiskLevel:            "medium",
		Reason:               "file edits require confirmation and snapshot flow",
		RequiresConfirmation: true,
		WorkspaceOnly:        true,
		DiffPreview:          true,
		SnapshotBeforeWrite:  true,
		RollbackSupported:    true,
		NextStep:             "Review the diff preview, then approve to create a snapshot and apply the workspace-only edit.",
	}
	_, err = srv.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
		ConversationID:     conversation.ID,
		UserMessageID:      user.ID,
		AssistantMessageID: assistant.ID,
		ParentMessageID:    user.ID,
		VariantIndex:       assistant.VariantIndex,
		ToolName:           "permission_request",
		Input:              map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:             request,
		Status:             agent.ExecutionNeedsConfirmation,
		RiskLevel:          request.RiskLevel,
	})
	if err != nil {
		t.Fatalf("SaveToolRun(permission_request) error = %v", err)
	}
	_, err = srv.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
		ConversationID:     conversation.ID,
		UserMessageID:      user.ID,
		AssistantMessageID: assistant.ID,
		ParentMessageID:    user.ID,
		VariantIndex:       assistant.VariantIndex,
		ToolName:           "edit_proposal",
		Input:              map[string]any{"request_id": request.RequestID, "path": "notes.md"},
		Output:             agent.EditProposal{RequestID: request.RequestID, Path: "notes.md", Content: "Persisted approval works.\n", Status: "ready_for_preview", DiffPreview: true, SnapshotBeforeWrite: true, RollbackSupported: true},
		Status:             "ready_for_preview",
		RiskLevel:          "medium",
	})
	if err != nil {
		t.Fatalf("SaveToolRun(edit_proposal) error = %v", err)
	}
	routeState, err := json.Marshal(routing.SessionContract{
		ActiveRoute:            routing.RouteFileWrite,
		TaskStatus:             routing.TaskStatusAwaitingApproval,
		PendingStep:            "edit_file",
		PendingApproval:        "edit_file",
		PendingOperationID:     request.RequestID,
		PendingOperationType:   "edit_file",
		PendingOperationTarget: "notes.md",
		PendingOperationStatus: "awaiting_approval",
		RouteLockStrength:      "pending_operation",
		LastOutcome:            routing.LastOutcomeCompleted,
		UpdatedAt:              time.Now().UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		t.Fatalf("marshal route state: %v", err)
	}
	if _, err := srv.deps.Memory.SaveConversationRouteState(ctx, memory.ConversationRouteState{
		ConversationID: conversation.ID,
		StateJSON:      string(routeState),
	}); err != nil {
		t.Fatalf("SaveConversationRouteState() error = %v", err)
	}

	payload := map[string]any{"request": request, "decision": "approved"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	httpRequest := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", bytes.NewReader(data))
	httpRequest.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, httpRequest)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result PermissionDecisionResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode permission decision: %v", err)
	}

	messages, err := srv.deps.Memory.ListConversationMessages(ctx, conversation.ID, 20)
	if err != nil {
		t.Fatalf("ListConversationMessages() error = %v", err)
	}
	var persisted, original memory.Message
	for _, message := range messages {
		if message.ID == assistant.ID {
			original = message
		}
		if message.Role == "assistant" && strings.Contains(message.Content, "Approval received. I applied the approved edit") {
			persisted = message
		}
	}
	if persisted.ID == "" {
		t.Fatalf("messages = %+v, want persisted approval result", messages)
	}
	if result.AssistantMessageID != persisted.ID || result.ParentMessageID != assistant.ID {
		t.Fatalf("result metadata = %+v, want saved message %s under approval message %s", result, persisted.ID, assistant.ID)
	}
	if persisted.ParentID != assistant.ID || !persisted.ActiveVariant {
		t.Fatalf("persisted message = %+v, want active child of approval message %s", persisted, assistant.ID)
	}
	if original.ID == "" || original.ParentID != user.ID || !original.ActiveVariant {
		t.Fatalf("original approval message = %+v, want still active under original user %s", original, user.ID)
	}
	runs, err := srv.deps.Memory.ListToolRunsForConversation(ctx, conversation.ID, 50)
	if err != nil {
		t.Fatalf("ListToolRunsForConversation() error = %v", err)
	}
	var sawPermissionDecision, sawPermissionResult bool
	for _, run := range runs {
		if run.ToolName == "permission_decision" && run.AssistantMessageID == assistant.ID && run.ParentMessageID == user.ID {
			sawPermissionDecision = true
		}
		if run.ToolName == "permission_result" && run.AssistantMessageID == persisted.ID {
			sawPermissionResult = true
		}
	}
	if !sawPermissionDecision {
		t.Fatalf("tool runs = %+v, want permission_decision linked to original approval message %s", runs, assistant.ID)
	}
	if !sawPermissionResult {
		t.Fatalf("tool runs = %+v, want permission_result linked to persisted message %s", runs, persisted.ID)
	}
	stored, ok, err := srv.deps.Memory.GetConversationRouteState(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("GetConversationRouteState() error = %v", err)
	}
	if !ok {
		t.Fatal("conversation route state missing")
	}
	var next routing.SessionContract
	if err := json.Unmarshal([]byte(stored.StateJSON), &next); err != nil {
		t.Fatalf("unmarshal route state: %v", err)
	}
	if next.PendingOperationID != "" || next.PendingApproval != "" || next.PendingStep != "" || next.RouteLockStrength == "pending_operation" {
		t.Fatalf("route state = %+v, want permission endpoint to clear pending operation", next)
	}
}

func TestPermissionDecisionApprovedGenericToolDoesNotRunImplicitAction(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	savePendingPermissionRequestForServerTest(t, srv, agent.PermissionRequest{
		RequestID:            "perm_safe_tool",
		ToolName:             "safe_tool",
		RiskLevel:            "high",
		Reason:               "high-risk action requires confirmation",
		RequiresConfirmation: true,
		WorkspaceOnly:        true,
		Destructive:          true,
		NextStep:             "Approve only after confirming the exact command.",
	})
	body := strings.NewReader(`{
		"request": {
			"request_id": "perm_safe_tool",
			"tool_name": "safe_tool",
			"risk_level": "high",
			"reason": "high-risk action requires confirmation",
			"requires_confirmation": true,
			"workspace_only": true,
			"destructive": true,
			"next_step": "Approve only after confirming the exact command."
		},
		"decision": "approved"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	var result PermissionDecisionResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode permission decision: %v", err)
	}
	if result.Executed {
		t.Fatal("Executed = true, want false for generic safe_tool approval")
	}
	if result.ToolStatus != "not_runnable" {
		t.Fatalf("ToolStatus = %q, want not_runnable", result.ToolStatus)
	}
	if !strings.Contains(result.Message, "did not include a concrete command") {
		t.Fatalf("Message = %q, want concrete-command guidance", result.Message)
	}
}

func TestPermissionDecisionRejectsForgedMemoryWriteApproval(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{
		"request": {
			"request_id": "perm_forged_memory",
			"tool_name": "memory_write",
			"command": ["memory_write", "follow_up", "Email Alex tomorrow"],
			"risk_level": "medium",
			"reason": "active skill requested saving a confirmed local memory",
			"requires_confirmation": true,
			"next_step": "Review the exact memory kind and content."
		},
		"decision": "approved"
	}`)
	request := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", body)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status code = %d, want 400; body: %s", recorder.Code, recorder.Body.String())
	}
	memories, err := srv.deps.Memory.SearchMemories(context.Background(), "Alex tomorrow", 5)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(memories) != 0 {
		t.Fatalf("forged approval wrote memories: %#v", memories)
	}
}

func TestPermissionDecisionRejectsReplayMemoryWriteApproval(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	permission := agent.PermissionRequest{
		RequestID:            "perm_memory_replay",
		ToolName:             "memory_write",
		Command:              []string{"memory_write", "follow_up", "Email Alex tomorrow"},
		RiskLevel:            "medium",
		Reason:               "active skill requested saving a confirmed local memory",
		RequiresConfirmation: true,
		NextStep:             "Review the exact memory kind and content.",
	}
	savePendingPermissionRequestForServerTest(t, srv, permission)
	payload := map[string]any{"request": permission, "decision": "approved"}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	first := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/permissions/decision", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(first, req)
	if first.Code != http.StatusOK {
		t.Fatalf("first status code = %d, want 200; body: %s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/permissions/decision", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(second, req)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("second status code = %d, want 400; body: %s", second.Code, second.Body.String())
	}

	memories, err := srv.deps.Memory.SearchMemories(context.Background(), "Alex tomorrow", 10)
	if err != nil {
		t.Fatalf("SearchMemories() error = %v", err)
	}
	if len(memories) != 1 {
		t.Fatalf("SearchMemories() len = %d, want one approved memory", len(memories))
	}
}

func TestFileWritePreviewAndApplyCreatesSnapshot(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	body := strings.NewReader(`{"path":"notes.md","content":"after\n"}`)
	previewRequest := httptest.NewRequest(http.MethodPost, "/api/files/plan-write", body)
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(previewRecorder, previewRequest)

	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status code = %d, want 200; body: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview FileWritePlanResult
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if !strings.Contains(preview.Diff, "+after") {
		t.Fatalf("preview diff = %q, want added content", preview.Diff)
	}

	applyBody := strings.NewReader(`{"path":"notes.md","content":"after\n","approved":true}`)
	applyRequest := httptest.NewRequest(http.MethodPost, "/api/files/apply-write", applyBody)
	applyRequest.Header.Set("Content-Type", "application/json")
	applyRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(applyRecorder, applyRequest)

	if applyRecorder.Code != http.StatusOK {
		t.Fatalf("apply status code = %d, want 200; body: %s", applyRecorder.Code, applyRecorder.Body.String())
	}
	var applied FileWriteApplyResult
	if err := json.Unmarshal(applyRecorder.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode apply: %v", err)
	}
	if applied.SnapshotID == "" {
		t.Fatal("SnapshotID is empty")
	}
	if applied.Verification.Status != "pass" {
		t.Fatalf("verification status = %q, want pass; reasons: %v", applied.Verification.Status, applied.Verification.Reasons)
	}
	if !applied.Verification.RollbackAvailable {
		t.Fatal("RollbackAvailable = false, want true")
	}
	data, err := os.ReadFile(filepath.Join(srv.deps.Workspace, "notes.md"))
	if err != nil {
		t.Fatalf("read applied file: %v", err)
	}
	if string(data) != "after\n" {
		t.Fatalf("file content = %q, want after", string(data))
	}
	runs, err := srv.deps.Memory.ListToolRuns(context.Background(), 10)
	if err != nil {
		t.Fatalf("ListToolRuns() error = %v", err)
	}
	var verifierLogged bool
	for _, run := range runs {
		if run.ToolName == "write_verifier" && run.Status == "pass" {
			verifierLogged = true
		}
	}
	if !verifierLogged {
		t.Fatal("write_verifier pass run was not logged")
	}
}

func TestFileWritePreviewAndApplyUsesWorkspaceGrantForExternalNewFile(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	external := t.TempDir()
	target := filepath.Join(external, "hello.txt")
	if _, err := srv.workspaceGrantStore().Grant(external, "Downloads", "test"); err != nil {
		t.Fatalf("grant external workspace: %v", err)
	}

	previewData, err := json.Marshal(FileWriteInput{Path: target, Content: "hello World\n"})
	if err != nil {
		t.Fatalf("marshal preview: %v", err)
	}
	previewRequest := httptest.NewRequest(http.MethodPost, "/api/files/plan-write", bytes.NewReader(previewData))
	previewRequest.Header.Set("Content-Type", "application/json")
	previewRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(previewRecorder, previewRequest)

	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("preview status code = %d, want 200; body: %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	var preview FileWritePlanResult
	if err := json.Unmarshal(previewRecorder.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.Path != target {
		t.Fatalf("preview path = %q, want %q", preview.Path, target)
	}
	if !strings.Contains(preview.Diff, "+hello World") {
		t.Fatalf("preview diff = %q, want added external content", preview.Diff)
	}

	applyData, err := json.Marshal(FileWriteInput{Path: target, Content: "hello World\n", Approved: true})
	if err != nil {
		t.Fatalf("marshal apply: %v", err)
	}
	applyRequest := httptest.NewRequest(http.MethodPost, "/api/files/apply-write", bytes.NewReader(applyData))
	applyRequest.Header.Set("Content-Type", "application/json")
	applyRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(applyRecorder, applyRequest)

	if applyRecorder.Code != http.StatusOK {
		t.Fatalf("apply status code = %d, want 200; body: %s", applyRecorder.Code, applyRecorder.Body.String())
	}
	var applied FileWriteApplyResult
	if err := json.Unmarshal(applyRecorder.Body.Bytes(), &applied); err != nil {
		t.Fatalf("decode apply: %v", err)
	}
	if applied.Path != target {
		t.Fatalf("applied path = %q, want %q", applied.Path, target)
	}
	if applied.SnapshotID == "" {
		t.Fatal("SnapshotID is empty")
	}
	if applied.Verification.Status != "pass" {
		t.Fatalf("verification status = %q, want pass; reasons: %v", applied.Verification.Status, applied.Verification.Reasons)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read applied external file: %v", err)
	}
	if string(data) != "hello World\n" {
		t.Fatalf("file content = %q, want hello World", string(data))
	}
}

func TestFileWriteApplyExternalNewFileRequiresWorkspaceGrant(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	target := filepath.Join(t.TempDir(), "hello.txt")
	applyData, err := json.Marshal(FileWriteInput{Path: target, Content: "hello World\n", Approved: true})
	if err != nil {
		t.Fatalf("marshal apply: %v", err)
	}
	applyRequest := httptest.NewRequest(http.MethodPost, "/api/files/apply-write", bytes.NewReader(applyData))
	applyRequest.Header.Set("Content-Type", "application/json")
	applyRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(applyRecorder, applyRequest)

	if applyRecorder.Code != http.StatusBadRequest {
		t.Fatalf("apply status code = %d, want 400; body: %s", applyRecorder.Code, applyRecorder.Body.String())
	}
	body := applyRecorder.Body.String()
	if !strings.Contains(body, "workspace path is not granted") {
		t.Fatalf("apply body = %q, want workspace grant message", body)
	}
	if strings.Contains(body, "path is outside workspace") {
		t.Fatalf("apply body = %q, should not expose workspace-root-only failure", body)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target exists or stat failed after blocked write: %v", err)
	}
}

func TestFailedEditPermissionResultTranslatesWorkspaceGrantError(t *testing.T) {
	result := failedEditPermissionResult(
		agent.PermissionRequest{ToolName: "edit_file"},
		fmt.Errorf("workspace path is not granted: /Users/example/Downloads; run `yemaka workspace grant \"/Users/example/Downloads\"` first"),
	)
	if !strings.Contains(result.Message, "Workspace permissions") && !strings.Contains(result.Message, "workspace access is not granted") {
		t.Fatalf("message = %q, want product workspace grant guidance", result.Message)
	}
	if strings.Contains(result.Message, "yemaka workspace grant") {
		t.Fatalf("message = %q, should not expose CLI-only grant instruction", result.Message)
	}
	if result.Result == nil || strings.Contains(result.Result.Context, "yemaka workspace grant") {
		t.Fatalf("result context = %+v, should use friendly grant guidance", result.Result)
	}
}

func TestLocalConnectorChatDisabledByDefault(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	request := httptest.NewRequest(http.MethodPost, "/connectors/local/v1/chat", strings.NewReader(`{"content":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404; body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestLocalConnectorChatRequiresTokenWhenEnabled(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	t.Setenv("YEMAKA_TEST_CONNECTOR_TOKEN", "secret")
	srv.deps.Config.Connectors.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Kind = "local_api"
	srv.deps.Config.Connectors.LocalAPI.Bind = "loopback"
	srv.deps.Config.Connectors.LocalAPI.RequireToken = true
	srv.deps.Config.Connectors.LocalAPI.TokenEnv = "YEMAKA_TEST_CONNECTOR_TOKEN"
	srv.deps.Config.Connectors.LocalAPI.MaxBodyBytes = 65536

	unauthorized := httptest.NewRequest(http.MethodPost, "/connectors/local/v1/chat", strings.NewReader(`{"content":"hello"}`))
	unauthorized.Header.Set("Content-Type", "application/json")
	unauthorizedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(unauthorizedRecorder, unauthorized)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401; body: %s", unauthorizedRecorder.Code, unauthorizedRecorder.Body.String())
	}

	authorized := httptest.NewRequest(http.MethodPost, "/connectors/local/v1/chat", strings.NewReader(`{"content":"hello"}`))
	authorized.Header.Set("Content-Type", "application/json")
	authorized.Header.Set("Authorization", "Bearer secret")
	authorizedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(authorizedRecorder, authorized)
	if authorizedRecorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", authorizedRecorder.Code, authorizedRecorder.Body.String())
	}
	var result ConnectorResult
	if err := json.Unmarshal(authorizedRecorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode connector result: %v", err)
	}
	if result.Text != "local web" {
		t.Fatalf("Text = %q, want local web", result.Text)
	}
}

func TestLocalConnectorEnforcesScopeRateLimitAndPolicyAudit(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	t.Setenv("YEMAKA_TEST_CONNECTOR_TOKEN", "secret")
	srv.deps.Config.Security.Policy.AuditEnabled = true
	srv.deps.Config.Connectors.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Kind = "local_api"
	srv.deps.Config.Connectors.LocalAPI.Bind = "loopback"
	srv.deps.Config.Connectors.LocalAPI.RequireToken = true
	srv.deps.Config.Connectors.LocalAPI.TokenEnv = "YEMAKA_TEST_CONNECTOR_TOKEN"
	srv.deps.Config.Connectors.LocalAPI.MaxBodyBytes = 65536
	srv.deps.Config.Connectors.LocalAPI.Permissions.Scopes = []string{"memory_search"}
	srv.deps.Config.Connectors.LocalAPI.RateLimit.RequestsPerMinute = 1

	chat := httptest.NewRequest(http.MethodPost, "/connectors/local/v1/chat", strings.NewReader(`{"content":"hello"}`))
	chat.Header.Set("Content-Type", "application/json")
	chat.Header.Set("Authorization", "Bearer secret")
	chatRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(chatRecorder, chat)
	if chatRecorder.Code != http.StatusForbidden {
		t.Fatalf("chat status = %d, want 403 for missing scope; body: %s", chatRecorder.Code, chatRecorder.Body.String())
	}

	if _, err := srv.deps.Memory.SaveMemory(context.Background(), memory.Memory{
		Kind:       "preference",
		Content:    "Connector scope memory",
		Importance: 4,
		Source:     "test",
	}); err != nil {
		t.Fatalf("SaveMemory() error = %v", err)
	}
	for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodGet, "/connectors/local/v1/memory/search?query=scope&limit=3", nil)
		request.Header.Set("Authorization", "Bearer secret")
		recorder := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Fatalf("memory search #%d status = %d, want %d; body: %s", i+1, recorder.Code, want, recorder.Body.String())
		}
	}
	audit, err := os.ReadFile(filepath.Join(srv.deps.Profile.Logs, safety.PolicyAuditFile))
	if err != nil {
		t.Fatalf("read policy audit: %v", err)
	}
	if !strings.Contains(string(audit), `"actor":"connector:local_api"`) || !strings.Contains(string(audit), `"resource":"memory_search"`) {
		t.Fatalf("policy audit missing connector request: %s", string(audit))
	}
}

func TestLocalConnectorEnforcesDayAndBurstRateLimits(t *testing.T) {
	t.Run("day", func(t *testing.T) {
		srv, cleanup := newTestServer(t)
		defer cleanup()
		t.Setenv("YEMAKA_TEST_CONNECTOR_TOKEN", "secret")
		enableTestLocalConnector(t, srv)
		srv.deps.Config.Connectors.LocalAPI.Permissions.Scopes = []string{"memory_search"}
		srv.deps.Config.Connectors.LocalAPI.RateLimit.RequestsPerMinute = 100
		srv.deps.Config.Connectors.LocalAPI.RateLimit.RequestsPerDay = 1
		srv.deps.Config.Connectors.LocalAPI.RateLimit.Burst = 100
		saveConnectorScopeMemory(t, srv)

		for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
			request := httptest.NewRequest(http.MethodGet, "/connectors/local/v1/memory/search?query=scope&limit=3", nil)
			request.Header.Set("Authorization", "Bearer secret")
			recorder := httptest.NewRecorder()
			srv.Handler().ServeHTTP(recorder, request)
			if recorder.Code != want {
				t.Fatalf("day limited search #%d status = %d, want %d; body: %s", i+1, recorder.Code, want, recorder.Body.String())
			}
		}
	})

	t.Run("burst", func(t *testing.T) {
		srv, cleanup := newTestServer(t)
		defer cleanup()
		t.Setenv("YEMAKA_TEST_CONNECTOR_TOKEN", "secret")
		enableTestLocalConnector(t, srv)
		srv.deps.Config.Connectors.LocalAPI.Permissions.Scopes = []string{"memory_search"}
		srv.deps.Config.Connectors.LocalAPI.RateLimit.RequestsPerMinute = 100
		srv.deps.Config.Connectors.LocalAPI.RateLimit.RequestsPerDay = 100
		srv.deps.Config.Connectors.LocalAPI.RateLimit.Burst = 1
		saveConnectorScopeMemory(t, srv)

		for i, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
			request := httptest.NewRequest(http.MethodGet, "/connectors/local/v1/memory/search?query=scope&limit=3", nil)
			request.Header.Set("Authorization", "Bearer secret")
			recorder := httptest.NewRecorder()
			srv.Handler().ServeHTTP(recorder, request)
			if recorder.Code != want {
				t.Fatalf("burst limited search #%d status = %d, want %d; body: %s", i+1, recorder.Code, want, recorder.Body.String())
			}
		}
	})
}

func TestLocalConnectorToolsAndMemorySearchUseLocalCore(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	t.Setenv("YEMAKA_TEST_CONNECTOR_TOKEN", "secret")
	srv.deps.Config.Connectors.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Kind = "local_api"
	srv.deps.Config.Connectors.LocalAPI.Bind = "loopback"
	srv.deps.Config.Connectors.LocalAPI.RequireToken = true
	srv.deps.Config.Connectors.LocalAPI.TokenEnv = "YEMAKA_TEST_CONNECTOR_TOKEN"
	srv.deps.Config.Connectors.LocalAPI.MaxBodyBytes = 65536

	if _, err := srv.deps.Memory.SaveMemory(context.Background(), memory.Memory{
		Kind:       "preference",
		Content:    "Prefer short local connector answers",
		Importance: 4,
		Source:     "test",
	}); err != nil {
		t.Fatalf("SaveMemory() error = %v", err)
	}

	toolsRequest := httptest.NewRequest(http.MethodGet, "/connectors/local/v1/tools", nil)
	toolsRequest.Header.Set("Authorization", "Bearer secret")
	toolsRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(toolsRecorder, toolsRequest)
	if toolsRecorder.Code != http.StatusOK {
		t.Fatalf("tools status code = %d, want 200; body: %s", toolsRecorder.Code, toolsRecorder.Body.String())
	}
	if !strings.Contains(toolsRecorder.Body.String(), `"memory_search"`) {
		t.Fatalf("tools response missing memory_search: %s", toolsRecorder.Body.String())
	}

	searchRequest := httptest.NewRequest(http.MethodGet, "/connectors/local/v1/memory/search?query=connector&limit=3", nil)
	searchRequest.Header.Set("Authorization", "Bearer secret")
	searchRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(searchRecorder, searchRequest)
	if searchRecorder.Code != http.StatusOK {
		t.Fatalf("memory search status code = %d, want 200; body: %s", searchRecorder.Code, searchRecorder.Body.String())
	}
	if !strings.Contains(searchRecorder.Body.String(), "short local connector") {
		t.Fatalf("memory search response missing memory: %s", searchRecorder.Body.String())
	}
}

func TestGeneratedConnectorRunRequiresEnabledReadiness(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	t.Setenv("YEMAKA_TEST_GENERATED_CONNECTOR_TOKEN", "secret")
	srv, cleanup := newTestServer(t)
	defer cleanup()
	installTestGeneratedConnector(t, srv, "generated_echo")

	request := httptest.NewRequest(http.MethodPost, "/connectors/generated/v1/generated_echo/run", strings.NewReader(`{"task":"hello"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404 disabled; body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestGeneratedConnectorRunUsesExtensionRunnerAndGates(t *testing.T) {
	t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "gocache"))
	t.Setenv("YEMAKA_TEST_GENERATED_CONNECTOR_TOKEN", "secret")
	srv, cleanup := newTestServer(t)
	defer cleanup()
	srv.deps.Config.Security.Policy.AuditEnabled = true
	installTestGeneratedConnector(t, srv, "generated_echo")
	if _, err := srv.generatedConnectorStore().SetEnabled("generated_echo", true); err != nil {
		t.Fatalf("SetEnabled(generated_echo) error = %v", err)
	}

	queryToken := httptest.NewRequest(http.MethodPost, "/connectors/generated/v1/generated_echo/run?token=secret", strings.NewReader(`{"task":"hello"}`))
	queryToken.Header.Set("Content-Type", "application/json")
	queryTokenRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(queryTokenRecorder, queryToken)
	if queryTokenRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("query token status = %d, want 401; body: %s", queryTokenRecorder.Code, queryTokenRecorder.Body.String())
	}

	for index, want := range []int{http.StatusOK, http.StatusTooManyRequests} {
		request := httptest.NewRequest(http.MethodPost, "/connectors/generated/v1/generated_echo/run", strings.NewReader(`{"task":"hello"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Yemaka-Generated-Connector-Token", "secret")
		recorder := httptest.NewRecorder()
		srv.Handler().ServeHTTP(recorder, request)
		if recorder.Code != want {
			t.Fatalf("run #%d status = %d, want %d; body: %s", index+1, recorder.Code, want, recorder.Body.String())
		}
		if want == http.StatusOK {
			var result GeneratedConnectorRunResult
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatalf("decode generated connector run: %v", err)
			}
			if result.Name != "generated_echo" || result.Extension != "generated_echo" || result.Status != "completed" {
				t.Fatalf("generated run result = %+v, want completed generated_echo", result)
			}
			if !strings.Contains(fmt.Sprint(result.Output["summary"]), "generated_echo handled task") {
				t.Fatalf("generated run output = %+v, want generated extension summary", result.Output)
			}
		}
	}

	records, err := srv.generatedConnectorStore().RecentAudit(10)
	if err != nil {
		t.Fatalf("RecentAudit() error = %v", err)
	}
	foundRun := false
	for _, record := range records {
		if record.Action == "run" && record.Name == "generated_echo" && record.Status == "ok" {
			foundRun = true
			break
		}
	}
	if !foundRun {
		t.Fatalf("generated connector audit = %+v, want ok run record", records)
	}
}

func enableTestLocalConnector(t *testing.T, srv *Server) {
	t.Helper()
	srv.deps.Config.Connectors.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Enabled = true
	srv.deps.Config.Connectors.LocalAPI.Kind = "local_api"
	srv.deps.Config.Connectors.LocalAPI.Bind = "loopback"
	srv.deps.Config.Connectors.LocalAPI.RequireToken = true
	srv.deps.Config.Connectors.LocalAPI.TokenEnv = "YEMAKA_TEST_CONNECTOR_TOKEN"
	srv.deps.Config.Connectors.LocalAPI.MaxBodyBytes = 65536
}

func installTestGeneratedConnector(t *testing.T, srv *Server, name string) {
	t.Helper()
	result, err := srv.extensionStore().Generate(t.Context(), extensions.GenerateInput{
		Name:        name,
		Description: "Generated connector runtime test extension.",
		Approved:    true,
	})
	if err != nil {
		t.Fatalf("Generate(%s) error = %v", name, err)
	}
	if !result.Extension.Callable {
		t.Fatalf("generated extension = %+v, want callable", result.Extension)
	}
	_, err = srv.generatedConnectorStore().Install(connectors.GeneratedInstallInput{
		Manifest: connectors.Manifest{
			Name:        name,
			Version:     "0.1.0",
			Kind:        "generated",
			Description: "Generated connector runtime test manifest.",
			Auth:        secrets.Reference{Provider: secrets.ProviderEnv, Name: "YEMAKA_TEST_GENERATED_CONNECTOR_TOKEN"},
			Permissions: config.ConnectorPermissionsConfig{
				Inbound: true,
				Scopes:  []string{"run"},
			},
			RateLimit: config.ConnectorRateLimitConfig{
				RequestsPerMinute: 1,
				RequestsPerDay:    10,
				Burst:             10,
			},
			Endpoint:         "extension:" + name,
			RequiresApproval: true,
		},
		Tests: connectors.TestReport{
			Status:   "passed",
			Commands: []string{"go test ./..."},
			Summary:  "generated connector runtime fixture passed",
		},
		Approved: true,
	})
	if err != nil {
		t.Fatalf("Install generated connector %s error = %v", name, err)
	}
}

func saveConnectorScopeMemory(t *testing.T, srv *Server) {
	t.Helper()
	if _, err := srv.deps.Memory.SaveMemory(context.Background(), memory.Memory{
		Kind:       "preference",
		Content:    "Connector scope memory",
		Importance: 4,
		Source:     "test",
	}); err != nil {
		t.Fatalf("SaveMemory() error = %v", err)
	}
}

func TestAdapterConnectorDisabledByDefault(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()

	request := httptest.NewRequest(http.MethodPost, "/connectors/adapters/v1/slack/chat", strings.NewReader("text=hello"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status code = %d, want 404; body: %s", recorder.Code, recorder.Body.String())
	}
}

func TestAdapterConnectorChatUsesSameCore(t *testing.T) {
	srv, cleanup := newTestServer(t)
	defer cleanup()
	t.Setenv("YEMAKA_TEST_SLACK_TOKEN", "secret")
	if err := connectors.EnableAdapter(srv.deps.Config, connectors.Slack, "YEMAKA_TEST_SLACK_TOKEN"); err != nil {
		t.Fatalf("EnableAdapter() error = %v", err)
	}

	unauthorized := httptest.NewRequest(http.MethodPost, "/connectors/adapters/v1/slack/chat", strings.NewReader("text=hello"))
	unauthorized.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	unauthorizedRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(unauthorizedRecorder, unauthorized)
	if unauthorizedRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401; body: %s", unauthorizedRecorder.Code, unauthorizedRecorder.Body.String())
	}

	queryToken := httptest.NewRequest(http.MethodPost, "/connectors/adapters/v1/slack/chat?token=secret", strings.NewReader("text=hello"))
	queryToken.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	queryTokenRecorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(queryTokenRecorder, queryToken)
	if queryTokenRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("query token status code = %d, want 401; body: %s", queryTokenRecorder.Code, queryTokenRecorder.Body.String())
	}

	authorized := httptest.NewRequest(http.MethodPost, "/connectors/adapters/v1/slack/chat", strings.NewReader("text=hello"))
	authorized.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	authorized.Header.Set("Authorization", "Bearer secret")
	recorder := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recorder, authorized)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"response_type"`) || !strings.Contains(recorder.Body.String(), "local web") {
		t.Fatalf("slack response missing expected body: %s", recorder.Body.String())
	}
}

func TestValidateLoopbackAddrRejectsLANBinding(t *testing.T) {
	if err := validateLoopbackAddr("0.0.0.0:7727"); err == nil {
		t.Fatal("validateLoopbackAddr allowed 0.0.0.0")
	}
	if err := validateLoopbackAddr(DefaultAddr); err != nil {
		t.Fatalf("validateLoopbackAddr(%q): %v", DefaultAddr, err)
	}
}

func writeServerExtension(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir extension: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, extensions.ManifestFile), []byte(`name: website_monitor
version: 0.1.0
description: Monitor a website and report changes.
type: tool
entrypoint:
  type: command
  command: ./website_monitor
input_schema:
  type: object
  properties:
    url:
      type: string
output_schema:
  type: object
  properties:
    ok:
      type: boolean
permissions:
  filesystem:
    read: false
    write: false
  shell: false
  secrets: false
safety:
  requires_user_approval: true
  max_runtime_seconds: 30
tests:
  - go test ./...
`), 0o644); err != nil {
		t.Fatalf("write extension manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module server_extension_fixture\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write extension go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(`package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

const protocolVersion = "yemaka.extension.v1"

func main() {
	_, _ = io.ReadAll(os.Stdin)
	output, err := handle(nil)
	if err != nil {
		emit(false, nil, err.Error())
		os.Exit(1)
	}
	emit(true, output, "")
}

func handle(input map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true, "summary": "server extension"}, nil
}

func emit(ok bool, output map[string]any, message string) {
	response := map[string]any{"protocol": protocolVersion, "ok": ok, "output": output}
	if !ok {
		response["error"] = message
	}
	data, err := json.Marshal(response)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode response: %v", err)
		return
	}
	fmt.Print(string(data))
}
`), 0o644); err != nil {
		t.Fatalf("write extension main.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main_test.go"), []byte(`package main

import "testing"

func TestHandleReturnsSummary(t *testing.T) {
	output, err := handle(nil)
	if err != nil {
		t.Fatalf("handle() error = %v", err)
	}
	if output["ok"] != true || output["summary"] != "server extension" {
		t.Fatalf("output = %+v, want server extension summary", output)
	}
}
`), 0o644); err != nil {
		t.Fatalf("write extension main_test.go: %v", err)
	}
}

func newTestServer(t *testing.T) (*Server, func()) {
	return newTestServerWithRuntime(t, fakeRuntime{models: []models.ModelInfo{{Name: "small:2b", Size: 1_500_000_000}}})
}

func newTestServerWithRuntime(t *testing.T, runtime models.Runtime) (*Server, func()) {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	t.Setenv(config.EnvHome, root)
	cfg := config.Default()
	cfg.Path = filepath.Join(root, config.ConfigName)
	cfg.App.SetupComplete = true
	cfg.Models["low_memory"] = config.ModelConfig{
		Provider:    "ollama",
		Name:        "small:2b",
		BaseURL:     "http://localhost:11434/api",
		Temperature: 0.1,
	}
	cfg.Memory.Database = filepath.Join(root, "profiles", "default", "memory.sqlite")
	if err := config.Write(cfg.Path, cfg); err != nil {
		t.Fatalf("write config: %v", err)
	}
	workspaceRoot := filepath.Join(root, "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspaceRoot, "notes.md"), []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		t.Fatalf("init profile: %v", err)
	}
	store, err := memory.Open(ctx, profile.Database)
	if err != nil {
		t.Fatalf("open memory: %v", err)
	}
	ragStore, err := rag.Open(ctx, profile.Database)
	if err != nil {
		_ = store.Close()
		t.Fatalf("open rag: %v", err)
	}
	registry := skills.Registry{Skills: map[string]skills.Skill{}}
	srv := New(Dependencies{
		Config:    cfg,
		Profile:   profile,
		Memory:    store,
		RAG:       ragStore,
		Skills:    registry,
		Runtime:   runtime,
		Router:    models.NewRouter(cfg),
		StaticDir: filepath.Join(root, "missing-dist"),
		Workspace: workspaceRoot,
	})
	return srv, func() {
		_ = ragStore.Close()
		_ = store.Close()
	}
}
