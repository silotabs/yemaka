package desktop

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"yemaka/internal/agent"
	"yemaka/internal/attachments"
	"yemaka/internal/config"
	"yemaka/internal/connectors"
	"yemaka/internal/diagnostics"
	"yemaka/internal/domainpacks"
	"yemaka/internal/evaluation"
	"yemaka/internal/extensions"
	"yemaka/internal/heartbeat"
	"yemaka/internal/internet"
	"yemaka/internal/knowledge"
	"yemaka/internal/learning"
	"yemaka/internal/memory"
	"yemaka/internal/modelprofiles"
	"yemaka/internal/models"
	"yemaka/internal/models/cloud"
	"yemaka/internal/models/modelruntime"
	"yemaka/internal/notifications"
	"yemaka/internal/profiles"
	"yemaka/internal/rag"
	"yemaka/internal/replay"
	"yemaka/internal/safety"
	"yemaka/internal/scheduler"
	servercore "yemaka/internal/server"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
	"yemaka/internal/workflows"
	"yemaka/internal/workspace"
)

type App struct {
	ctx context.Context
	mu  sync.Mutex

	config          *config.Config
	profile         *profiles.Profile
	store           *memory.Store
	rag             *rag.Store
	skills          skills.Registry
	modelRuntime    models.Runtime
	runtimeFactory  func(config.ModelConfig) (models.Runtime, error)
	cloudRuntime    models.Runtime
	router          *models.Router
	activeCancel    context.CancelFunc
	activeRunID     int
	restartLauncher func(path string, args []string, dir string, env []string) error
	runtimeQuitter  func(context.Context)
}

type Status struct {
	ConfigPath     string        `json:"configPath"`
	ProfilePath    string        `json:"profilePath"`
	SQLitePath     string        `json:"sqlitePath"`
	SQLiteOK       bool          `json:"sqliteOk"`
	SQLiteError    string        `json:"sqliteError"`
	OllamaOK       bool          `json:"ollamaOk"`
	OllamaError    string        `json:"ollamaError"`
	DefaultModel   string        `json:"defaultModel"`
	LowMemoryModel string        `json:"lowMemoryModel"`
	SelectedModel  string        `json:"selectedModel"`
	LowMemoryMode  bool          `json:"lowMemoryMode"`
	SetupComplete  bool          `json:"setupComplete"`
	ModelReady     bool          `json:"modelReady"`
	RAGEnabled     bool          `json:"ragEnabled"`
	ShellEnabled   bool          `json:"shellEnabled"`
	Telemetry      bool          `json:"telemetry"`
	ModelStatuses  []ModelStatus `json:"modelStatuses"`
	SkillCount     int           `json:"skillCount"`
	Workspace      string        `json:"workspace"`
}

type ModelStatus struct {
	Role          string `json:"role"`
	Name          string `json:"name"`
	Installed     bool   `json:"installed"`
	Profile       string `json:"profile,omitempty"`
	ProfileValid  bool   `json:"profileValid,omitempty"`
	ProfileStatus string `json:"profileStatus,omitempty"`
}

type SkillSummary struct {
	Name            string   `json:"name"`
	Version         string   `json:"version"`
	Description     string   `json:"description"`
	Triggers        []string `json:"triggers"`
	RequiredTools   []string `json:"requiredTools"`
	Enabled         bool     `json:"enabled"`
	Valid           bool     `json:"valid"`
	ValidationError string   `json:"validationError"`
	Source          string   `json:"source"`
	Dir             string   `json:"dir"`
}

type SkillActionResult struct {
	Skill   SkillSummary `json:"skill"`
	Message string       `json:"message"`
	Path    string       `json:"path"`
}

type DomainPackSkillStatus struct {
	PackName        string `json:"packName"`
	Ref             string `json:"ref"`
	Active          bool   `json:"active"`
	Enabled         bool   `json:"enabled"`
	Valid           bool   `json:"valid"`
	ValidationError string `json:"validationError,omitempty"`
	PackDir         string `json:"packDir"`
}

type ExtensionActionResult struct {
	Extension extensions.Status `json:"extension"`
	Message   string            `json:"message"`
}

type ExtensionRunInput struct {
	Name  string         `json:"name"`
	Input map[string]any `json:"input"`
}

type ExtensionProposalInput struct {
	Request string `json:"request"`
}

type CapabilityGenerateInput struct {
	Request               string                   `json:"request"`
	Approved              bool                     `json:"approved"`
	Name                  string                   `json:"name,omitempty"`
	AllowedDomains        []string                 `json:"allowedDomains,omitempty"`
	RunAfterGenerate      bool                     `json:"runAfterGenerate,omitempty"`
	RunInput              map[string]any           `json:"runInput,omitempty"`
	Draft                 *extensions.PackageDraft `json:"draft,omitempty"`
	SynthesizeDraft       bool                     `json:"synthesizeDraft,omitempty"`
	DraftRepairAttempts   int                      `json:"draftRepairAttempts,omitempty"`
	ScheduleAfterGenerate bool                     `json:"scheduleAfterGenerate,omitempty"`
	ScheduleName          string                   `json:"scheduleName,omitempty"`
	ScheduleType          string                   `json:"scheduleType,omitempty"`
	ScheduleExpr          string                   `json:"scheduleExpr,omitempty"`
	ScheduleEnabled       bool                     `json:"scheduleEnabled,omitempty"`
	ScheduleInput         map[string]any           `json:"scheduleInput,omitempty"`
}

type CapabilityApplyInput struct {
	Approved           bool   `json:"approved"`
	Kind               string `json:"kind,omitempty"`
	Name               string `json:"name,omitempty"`
	ExistingCapability string `json:"existingCapability,omitempty"`
	PackSource         string `json:"packSource,omitempty"`
	PackDir            string `json:"packDir,omitempty"`
}

type CapabilityApplyResult struct {
	Pack    domainpacks.PackStatus `json:"pack"`
	Action  string                 `json:"action"`
	Message string                 `json:"message"`
}

type ExtensionGenerateInput struct {
	Name            string                   `json:"name"`
	Description     string                   `json:"description"`
	Approved        bool                     `json:"approved"`
	BrokeredNetwork bool                     `json:"brokeredNetwork"`
	AllowedDomains  []string                 `json:"allowedDomains,omitempty"`
	Draft           *extensions.PackageDraft `json:"draft,omitempty"`
}

type ExtensionRollbackInput struct {
	NameOrSnapshot string `json:"nameOrSnapshot"`
}

type PolicyStatusResult struct {
	Mode                         string `json:"mode"`
	AuditEnabled                 bool   `json:"auditEnabled"`
	MaxAutonomousLevel           int    `json:"maxAutonomousLevel"`
	AllowAutonomousPosting       bool   `json:"allowAutonomousPosting"`
	AllowAutonomousDeployment    bool   `json:"allowAutonomousDeployment"`
	AllowLiveTrading             bool   `json:"allowLiveTrading"`
	GeneratedCanModifyCorePolicy bool   `json:"generatedCanModifyCorePolicy"`
	FullAccess                   bool   `json:"fullAccess"`
}

type LearningCorrectionInput struct {
	ConversationID string `json:"conversationId"`
	Correction     string `json:"correction"`
}

type SetupState struct {
	SetupComplete      bool               `json:"setupComplete"`
	OllamaOK           bool               `json:"ollamaOk"`
	OllamaError        string             `json:"ollamaError"`
	ConfigPath         string             `json:"configPath"`
	ProfilePath        string             `json:"profilePath"`
	LowMemoryMode      bool               `json:"lowMemoryMode"`
	Mode               string             `json:"mode"`
	SelectedModel      string             `json:"selectedModel"`
	ModelReady         bool               `json:"modelReady"`
	InstalledModels    []models.ModelInfo `json:"installedModels"`
	RecommendedMode    string             `json:"recommendedMode"`
	RecommendedModel   string             `json:"recommendedModel"`
	ConfiguredDefaults []ModelStatus      `json:"configuredDefaults"`
}

type SetupInput struct {
	Mode           string `json:"mode"`
	LowMemoryModel string `json:"lowMemoryModel"`
	DefaultModel   string `json:"defaultModel"`
}

func setupModeFromConfig(cfg *config.Config) string {
	if cfg != nil && cfg.Runtime.MaxContextTokens >= 6144 {
		return "useful_local"
	}
	return "student_laptop"
}

type ChatResult struct {
	ConversationID string   `json:"conversationId"`
	Text           string   `json:"text"`
	Model          string   `json:"model"`
	ToolCalls      []string `json:"toolCalls"`
}

type ModelGenerateResult struct {
	Text  string `json:"text"`
	Model string `json:"model"`
}

type ModelProfileInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	BaseModel   string   `json:"baseModel"`
	System      string   `json:"system,omitempty"`
	Temperature float64  `json:"temperature,omitempty"`
	NumCtx      int      `json:"numCtx,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Purpose     string   `json:"purpose,omitempty"`
	RefinedFrom string   `json:"refinedFrom,omitempty"`
}

type ModelProfileDraftInput struct {
	ConversationID string  `json:"conversationId"`
	Name           string  `json:"name,omitempty"`
	BaseModel      string  `json:"baseModel,omitempty"`
	Temperature    float64 `json:"temperature,omitempty"`
	NumCtx         int     `json:"numCtx,omitempty"`
}

type ModelProfileApplyInput struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type ModelProfileDraftReport struct {
	Schema            string                        `json:"schema"`
	ConversationID    string                        `json:"conversationId"`
	Eligibility       string                        `json:"eligibility"`
	Reason            string                        `json:"reason"`
	MessageCount      int                           `json:"messageCount"`
	AssistantMessages int                           `json:"assistantMessages"`
	ToolRunCount      int                           `json:"toolRunCount"`
	FailedToolRuns    int                           `json:"failedToolRuns"`
	ObservedTraits    []string                      `json:"observedTraits,omitempty"`
	PrivacyFilter     learning.PrivacyFilterReport  `json:"privacyFilter"`
	SafetySummary     []string                      `json:"safetySummary,omitempty"`
	Readiness         modelprofiles.ReadinessReport `json:"readiness"`
}

type ModelProfileResult struct {
	Profile       modelprofiles.Profile          `json:"profile"`
	Path          string                         `json:"path,omitempty"`
	Modelfile     string                         `json:"modelfile,omitempty"`
	Message       string                         `json:"message,omitempty"`
	Applied       bool                           `json:"applied,omitempty"`
	AppliedRole   string                         `json:"appliedRole,omitempty"`
	SafetySummary []string                       `json:"safetySummary,omitempty"`
	DraftReport   *ModelProfileDraftReport       `json:"draftReport,omitempty"`
	Readiness     modelprofiles.ReadinessReport  `json:"readiness"`
	Comparison    modelprofiles.ComparisonReport `json:"comparison"`
	History       []modelprofiles.HistoryEvent   `json:"history,omitempty"`
}

type AskResult struct {
	ConversationID     string                 `json:"conversationId"`
	Text               string                 `json:"text"`
	Model              string                 `json:"model"`
	Skill              string                 `json:"skill"`
	Sources            []string               `json:"sources"`
	SourceKind         string                 `json:"sourceKind"`
	ToolCalls          []string               `json:"toolCalls"`
	UserMessageID      string                 `json:"userMessageId"`
	AssistantMessageID string                 `json:"assistantMessageId"`
	ParentMessageID    string                 `json:"parentMessageId"`
	VariantIndex       int                    `json:"variantIndex"`
	Attachments        []ChatAttachmentResult `json:"attachments,omitempty"`
}

type ConversationResult struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Starred   bool   `json:"starred"`
}

type ConversationMessageResult struct {
	ID            string                 `json:"id"`
	Role          string                 `json:"role"`
	Content       string                 `json:"content"`
	Meta          string                 `json:"meta,omitempty"`
	Model         string                 `json:"model"`
	CreatedAt     string                 `json:"createdAt"`
	ParentID      string                 `json:"parentId"`
	VariantIndex  int                    `json:"variantIndex"`
	ActiveVariant bool                   `json:"activeVariant"`
	Trace         []string               `json:"trace,omitempty"`
	Sources       []string               `json:"sources,omitempty"`
	SourceKind    string                 `json:"sourceKind,omitempty"`
	Attachments   []ChatAttachmentResult `json:"attachments,omitempty"`
}

type ChatAttachmentUploadInput struct {
	FileName    string `json:"fileName"`
	ContentType string `json:"contentType"`
	DataBase64  string `json:"dataBase64"`
	Retention   string `json:"retention,omitempty"`
}

type ChatAttachmentResult struct {
	ID          string   `json:"id"`
	FileName    string   `json:"fileName"`
	ContentType string   `json:"contentType"`
	SizeBytes   int64    `json:"sizeBytes"`
	Status      string   `json:"status"`
	Summary     string   `json:"summary,omitempty"`
	Preview     string   `json:"preview,omitempty"`
	SourceKind  string   `json:"sourceKind,omitempty"`
	Sources     []string `json:"sources,omitempty"`
	Retention   string   `json:"retention,omitempty"`
	CreatedAt   string   `json:"createdAt,omitempty"`
}

type ChatAttachmentUploadResult struct {
	Attachment ChatAttachmentResult `json:"attachment"`
	Message    string               `json:"message"`
}

type ConversationDetailResult struct {
	Conversation ConversationResult          `json:"conversation"`
	Messages     []ConversationMessageResult `json:"messages"`
}

type MemoryResult struct {
	ID             string `json:"id"`
	MessageID      string `json:"messageId"`
	ConversationID string `json:"conversationId"`
	Role           string `json:"role"`
	Kind           string `json:"kind"`
	Content        string `json:"content"`
	Snippet        string `json:"snippet"`
	Importance     int    `json:"importance"`
	Source         string `json:"source"`
	Pinned         bool   `json:"pinned"`
	Explicit       bool   `json:"explicit"`
	CreatedAt      string `json:"createdAt"`
}

type MemoryWriteInput struct {
	Kind       string `json:"kind"`
	Content    string `json:"content"`
	Importance int    `json:"importance"`
}

type KnowledgeStatus struct {
	Enabled              bool   `json:"enabled"`
	InfluenceEnabled     bool   `json:"influenceEnabled"`
	ManualOnly           bool   `json:"manualOnly"`
	MaxEntitiesPerQuery  int    `json:"maxEntitiesPerQuery"`
	MaxEvidenceChars     int    `json:"maxEvidenceChars"`
	MaxInfluenceEntities int    `json:"maxInfluenceEntities"`
	MaxInfluenceChars    int    `json:"maxInfluenceChars"`
	Database             string `json:"database"`
	EntityCount          int    `json:"entityCount"`
	EdgeCount            int    `json:"edgeCount"`
	Message              string `json:"message"`
}

type KnowledgeEntityResult struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	Source       string `json:"source"`
	SourceKind   string `json:"sourceKind"`
	SourceRef    string `json:"sourceRef"`
	Evidence     string `json:"evidence"`
	ReviewStatus string `json:"reviewStatus"`
	ReviewNote   string `json:"reviewNote"`
	ReviewedBy   string `json:"reviewedBy"`
	ReviewedAt   string `json:"reviewedAt"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type KnowledgeEdgeResult struct {
	ID           string `json:"id"`
	FromEntityID string `json:"fromEntityId"`
	Relation     string `json:"relation"`
	ToEntityID   string `json:"toEntityId"`
	Source       string `json:"source"`
	SourceKind   string `json:"sourceKind"`
	SourceRef    string `json:"sourceRef"`
	Evidence     string `json:"evidence"`
	ReviewStatus string `json:"reviewStatus"`
	ReviewNote   string `json:"reviewNote"`
	ReviewedBy   string `json:"reviewedBy"`
	ReviewedAt   string `json:"reviewedAt"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type KnowledgeEntityInput struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	SourceKind string `json:"sourceKind"`
	SourceRef  string `json:"sourceRef"`
	Evidence   string `json:"evidence"`
}

type KnowledgeEdgeInput struct {
	FromEntityID string `json:"fromEntityId"`
	Relation     string `json:"relation"`
	ToEntityID   string `json:"toEntityId"`
	Source       string `json:"source"`
	SourceKind   string `json:"sourceKind"`
	SourceRef    string `json:"sourceRef"`
	Evidence     string `json:"evidence"`
}

type KnowledgeEnabledInput struct {
	Enabled          bool `json:"enabled"`
	InfluenceEnabled bool `json:"influenceEnabled"`
}

type KnowledgeRepairResult struct {
	EntitiesScanned int             `json:"entitiesScanned"`
	EntitiesUpdated int             `json:"entitiesUpdated"`
	EdgesScanned    int             `json:"edgesScanned"`
	EdgesUpdated    int             `json:"edgesUpdated"`
	Status          KnowledgeStatus `json:"status"`
	Message         string          `json:"message"`
}

type KnowledgeProposalInput struct {
	Text        string `json:"text"`
	Source      string `json:"source"`
	SourceKind  string `json:"sourceKind"`
	SourceRef   string `json:"sourceRef"`
	DefaultKind string `json:"defaultKind"`
	Limit       int    `json:"limit"`
}

type KnowledgeEntityProposalResult struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Source     string `json:"source"`
	SourceKind string `json:"sourceKind"`
	SourceRef  string `json:"sourceRef"`
	Evidence   string `json:"evidence"`
	Reason     string `json:"reason"`
}

type KnowledgeRelationProposalResult struct {
	FromName   string `json:"fromName"`
	Relation   string `json:"relation"`
	ToName     string `json:"toName"`
	Source     string `json:"source"`
	SourceKind string `json:"sourceKind"`
	SourceRef  string `json:"sourceRef"`
	Evidence   string `json:"evidence"`
	Reason     string `json:"reason"`
}

type KnowledgeProposalResult struct {
	Source            string                            `json:"source"`
	SourceKind        string                            `json:"sourceKind"`
	SourceRef         string                            `json:"sourceRef"`
	ReviewRequired    bool                              `json:"reviewRequired"`
	Message           string                            `json:"message"`
	EntityProposals   []KnowledgeEntityProposalResult   `json:"entityProposals"`
	RelationProposals []KnowledgeRelationProposalResult `json:"relationProposals"`
}

type KnowledgeReviewItemResult struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Name         string `json:"name"`
	Kind         string `json:"kind"`
	FromEntityID string `json:"fromEntityId"`
	Relation     string `json:"relation"`
	ToEntityID   string `json:"toEntityId"`
	Source       string `json:"source"`
	SourceKind   string `json:"sourceKind"`
	SourceRef    string `json:"sourceRef"`
	Evidence     string `json:"evidence"`
	ReviewStatus string `json:"reviewStatus"`
	ReviewNote   string `json:"reviewNote"`
	ReviewedBy   string `json:"reviewedBy"`
	ReviewedAt   string `json:"reviewedAt"`
	UpdatedAt    string `json:"updatedAt"`
}

type KnowledgeReviewTargetInput struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type KnowledgeReviewBatchInput struct {
	Action     string                       `json:"action"`
	Targets    []KnowledgeReviewTargetInput `json:"targets"`
	ReviewedBy string                       `json:"reviewedBy"`
	Note       string                       `json:"note"`
}

type KnowledgeReviewBatchResult struct {
	Action          string                      `json:"action"`
	Requested       int                         `json:"requested"`
	EntitiesMatched int                         `json:"entitiesMatched"`
	EdgesMatched    int                         `json:"edgesMatched"`
	EntitiesUpdated int                         `json:"entitiesUpdated"`
	EdgesUpdated    int                         `json:"edgesUpdated"`
	Items           []KnowledgeReviewItemResult `json:"items"`
}

type DocumentResult struct {
	ChunkID     string   `json:"chunkId"`
	Path        string   `json:"path"`
	Content     string   `json:"content"`
	Rank        float64  `json:"rank"`
	Score       float64  `json:"score"`
	Source      string   `json:"source"`
	Explanation []string `json:"explanation"`
}

type DocumentInventoryResult struct {
	ID                 string `json:"id"`
	Path               string `json:"path"`
	WorkspaceRoot      string `json:"workspaceRoot"`
	ManagedSource      string `json:"managedSource"`
	Status             string `json:"status"`
	Reason             string `json:"reason"`
	Missing            bool   `json:"missing"`
	Stale              bool   `json:"stale"`
	SizeBytes          int64  `json:"sizeBytes"`
	ChunkCount         int    `json:"chunkCount"`
	EmbeddingCount     int    `json:"embeddingCount"`
	IndexedAt          string `json:"indexedAt"`
	UpdatedAt          string `json:"updatedAt"`
	ContentHash        string `json:"contentHash"`
	CurrentContentHash string `json:"currentContentHash"`
}

type DocumentPruneResult struct {
	DryRun            bool     `json:"dryRun"`
	DocumentsMatched  int      `json:"documentsMatched"`
	ChunksMatched     int      `json:"chunksMatched"`
	FTSRowsMatched    int      `json:"ftsRowsMatched"`
	EmbeddingsMatched int      `json:"embeddingsMatched"`
	DocumentsRemoved  int      `json:"documentsRemoved"`
	ChunksRemoved     int      `json:"chunksRemoved"`
	FTSRowsRemoved    int      `json:"ftsRowsRemoved"`
	EmbeddingsRemoved int      `json:"embeddingsRemoved"`
	DocumentIDs       []string `json:"documentIds"`
}

type DocumentPathSuggestionResult struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Directory string `json:"directory"`
	Kind      string `json:"kind"`
	SizeBytes int64  `json:"sizeBytes"`
}

type IngestSummary struct {
	Root           string   `json:"root"`
	FilesIndexed   int      `json:"filesIndexed"`
	FilesSkipped   int      `json:"filesSkipped"`
	FilesUnchanged int      `json:"filesUnchanged"`
	ChunksCreated  int      `json:"chunksCreated"`
	BytesIndexed   int64    `json:"bytesIndexed"`
	SkippedReasons []string `json:"skippedReasons"`
}

type EmbeddingIndexSummary struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	ChunksEmbedded int    `json:"chunksEmbedded"`
	ChunksSkipped  int    `json:"chunksSkipped"`
	Dimensions     int    `json:"dimensions"`
}

type WorkspaceGrantResult struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Label         string `json:"label"`
	Source        string `json:"source"`
	CreatedAt     string `json:"createdAt"`
	BookmarkReady bool   `json:"bookmarkReady"`
	BookmarkStale bool   `json:"bookmarkStale"`
}

type DocumentSourceResult struct {
	Path  string               `json:"path"`
	Grant WorkspaceGrantResult `json:"grant"`
}

type ToolRunResult struct {
	ID                 string `json:"id"`
	ConversationID     string `json:"conversationId"`
	SessionID          string `json:"sessionId"`
	UserMessageID      string `json:"userMessageId"`
	AssistantMessageID string `json:"assistantMessageId"`
	ParentMessageID    string `json:"parentMessageId"`
	VariantIndex       int    `json:"variantIndex"`
	ToolName           string `json:"toolName"`
	Input              any    `json:"input"`
	Output             any    `json:"output"`
	Status             string `json:"status"`
	RiskLevel          string `json:"riskLevel"`
	CreatedAt          string `json:"createdAt"`
	CompletedAt        string `json:"completedAt"`
}

type ToolCatalogEntryResult struct {
	Name             string   `json:"name"`
	Status           string   `json:"status"`
	Surfaces         []string `json:"surfaces"`
	ChatCallable     bool     `json:"chatCallable"`
	ModelCallable    bool     `json:"modelCallable"`
	RequiresApproval bool     `json:"requiresApproval"`
	EnabledByDefault bool     `json:"enabledByDefault"`
	Mutating         bool     `json:"mutating"`
	Notes            string   `json:"notes"`
}

type PermissionDecisionInput struct {
	Request  agent.PermissionRequest `json:"request"`
	Decision string                  `json:"decision"`
	Note     string                  `json:"note"`
}

type PermissionDecisionResult struct {
	Status             string                 `json:"status"`
	ToolName           string                 `json:"toolName,omitempty"`
	ToolStatus         string                 `json:"toolStatus,omitempty"`
	Executed           bool                   `json:"executed,omitempty"`
	Message            string                 `json:"message,omitempty"`
	Result             *agent.ExecutionResult `json:"result,omitempty"`
	ConversationID     string                 `json:"conversationId,omitempty"`
	AssistantMessageID string                 `json:"assistantMessageId,omitempty"`
	ParentMessageID    string                 `json:"parentMessageId,omitempty"`
	VariantIndex       int                    `json:"variantIndex,omitempty"`
	ActiveVariant      bool                   `json:"activeVariant,omitempty"`
	CreatedAt          string                 `json:"createdAt,omitempty"`
}

type FileWriteInput struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Approved bool   `json:"approved"`
}

type FileWritePlanResult struct {
	Path        string `json:"path"`
	Diff        string `json:"diff"`
	IsNew       bool   `json:"isNew"`
	Changed     bool   `json:"changed"`
	TargetBytes int    `json:"targetBytes"`
}

type FileWriteApplyResult struct {
	Path         string                      `json:"path"`
	Diff         string                      `json:"diff"`
	SnapshotID   string                      `json:"snapshotId"`
	IsNew        bool                        `json:"isNew"`
	Changed      bool                        `json:"changed"`
	TargetBytes  int                         `json:"targetBytes"`
	Verification workspace.WriteVerification `json:"verification"`
}

type QARegressionSourceWritePlanResult struct {
	TraceID             string              `json:"traceId"`
	ApplyPlanPath       string              `json:"applyPlanPath,omitempty"`
	SuggestedTestFile   string              `json:"suggestedTestFile"`
	SuggestedTestName   string              `json:"suggestedTestName"`
	RequiresApproval    bool                `json:"requiresApproval"`
	FileWritePlanResult FileWritePlanResult `json:"fileWritePlan"`
}

type QARegressionSourceWriteApplyResult struct {
	TraceID              string                                 `json:"traceId"`
	Record               learning.QARegressionSourceWriteRecord `json:"record"`
	FileWriteApplyResult FileWriteApplyResult                   `json:"fileWriteApply"`
}

type SettingsView struct {
	SettingsSchemaVersion int                           `json:"settingsSchemaVersion"`
	FeatureSupport        SettingsFeatureSupport        `json:"featureSupport"`
	LowMemoryMode         bool                          `json:"lowMemoryMode"`
	MaxContextTokens      int                           `json:"maxContextTokens"`
	ResponseMode          string                        `json:"responseMode"`
	ShowThinkingTrace     bool                          `json:"showThinkingTrace"`
	UI                    config.UIConfig               `json:"ui"`
	ModelRoles            map[string]config.ModelConfig `json:"modelRoles"`
	CloudFallback         config.CloudFallbackConfig    `json:"cloudFallback"`
	Connectors            config.ConnectorsConfig       `json:"connectors"`
	Internet              config.InternetConfig         `json:"internet"`
	Scheduler             config.SchedulerConfig        `json:"scheduler"`
	Heartbeat             config.HeartbeatConfig        `json:"heartbeat"`
	RAG                   config.RAGConfig              `json:"rag"`
	Knowledge             config.KnowledgeGraphConfig   `json:"knowledge"`
	Workspace             config.WorkspaceConfig        `json:"workspace"`
	ShellEnabled          bool                          `json:"shellEnabled"`
	Telemetry             bool                          `json:"telemetry"`
}

type SettingsFeatureSupport struct {
	ResponseMode      bool `json:"responseMode"`
	ShowThinkingTrace bool `json:"showThinkingTrace"`
}

type SettingsInput struct {
	LowMemoryMode             bool   `json:"lowMemoryMode"`
	MaxContextTokens          int    `json:"maxContextTokens"`
	ResponseMode              string `json:"responseMode"`
	ShowThinkingTrace         bool   `json:"showThinkingTrace"`
	UITheme                   string `json:"uiTheme"`
	RAGEnabled                bool   `json:"ragEnabled"`
	EmbeddingsEnabled         bool   `json:"embeddingsEnabled"`
	EmbeddingModel            string `json:"embeddingModel"`
	CloudFallbackEnabled      bool   `json:"cloudFallbackEnabled"`
	CloudFallbackBaseURL      string `json:"cloudFallbackBaseUrl"`
	CloudFallbackModel        string `json:"cloudFallbackModel"`
	CloudFallbackKeyEnv       string `json:"cloudFallbackKeyEnv"`
	ConnectorEnabled          bool   `json:"connectorEnabled"`
	ConnectorTokenEnv         string `json:"connectorTokenEnv"`
	MCPConnectorEnabled       bool   `json:"mcpConnectorEnabled"`
	SlackConnectorEnabled     bool   `json:"slackConnectorEnabled"`
	SlackConnectorTokenEnv    string `json:"slackConnectorTokenEnv"`
	DiscordConnectorEnabled   bool   `json:"discordConnectorEnabled"`
	DiscordConnectorTokenEnv  string `json:"discordConnectorTokenEnv"`
	TelegramConnectorEnabled  bool   `json:"telegramConnectorEnabled"`
	TelegramConnectorTokenEnv string `json:"telegramConnectorTokenEnv"`
	EmailConnectorEnabled     bool   `json:"emailConnectorEnabled"`
	EmailConnectorTokenEnv    string `json:"emailConnectorTokenEnv"`
	InternetEnabled           bool   `json:"internetEnabled"`
	InternetSearchEnabled     bool   `json:"internetSearchEnabled"`
	InternetSearchProvider    string `json:"internetSearchProvider"`
	InternetSearchEndpoint    string `json:"internetSearchEndpoint"`
	InternetSearchAPIKeyEnv   string `json:"internetSearchApiKeyEnv"`
	KnowledgeInfluenceEnabled bool   `json:"knowledgeInfluenceEnabled"`
}

type InternetFetchInput struct {
	URL            string   `json:"url"`
	ExtractText    bool     `json:"extractText"`
	AllowedDomains []string `json:"allowedDomains"`
	TaskApproved   bool     `json:"taskApproved"`
}

type InternetCrawlInput struct {
	SeedURL            string   `json:"seedUrl"`
	Method             string   `json:"method"`
	ExtractText        bool     `json:"extractText"`
	AllowedDomains     []string `json:"allowedDomains"`
	MaxPages           int      `json:"maxPages"`
	MaxDepth           int      `json:"maxDepth"`
	MaxDurationSeconds int      `json:"maxDurationSeconds"`
	MaxLinksPerPage    int      `json:"maxLinksPerPage"`
	MaxTextChars       int      `json:"maxTextChars"`
	TaskApproved       bool     `json:"taskApproved"`
}

type InternetSearchInput struct {
	Query        string `json:"query"`
	TaskApproved bool   `json:"taskApproved"`
	MaxResults   int    `json:"maxResults"`
}

type JobCreateInput struct {
	Name         string         `json:"name"`
	ScheduleType string         `json:"scheduleType"`
	ScheduleExpr string         `json:"scheduleExpr"`
	TargetType   string         `json:"targetType"`
	TargetName   string         `json:"targetName"`
	Input        map[string]any `json:"input"`
	Approved     bool           `json:"approved"`
	Enabled      bool           `json:"enabled"`
}

type JobEnabledInput struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type JobInputUpdateInput struct {
	ID       string         `json:"id"`
	Input    map[string]any `json:"input"`
	Approved bool           `json:"approved"`
}

type JobRunInput struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversationId,omitempty"`
}

type JobArchiveInput struct {
	ID       string `json:"id"`
	Approved bool   `json:"approved"`
}

type GeneratedConnectorEnabledInput struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

func NewApp() *App {
	return &App{
		runtimeFactory:  modelruntime.New,
		restartLauncher: startYemakaProcess,
		runtimeQuitter:  wailsruntime.Quit,
	}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	_ = a.ensure()
}

func (a *App) Shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.store != nil {
		_ = a.store.Close()
		a.store = nil
	}
	if a.rag != nil {
		_ = a.rag.Close()
		a.rag = nil
	}
}

func (a *App) RequestRestart() (servercore.RuntimeControlResult, error) {
	executable, err := os.Executable()
	if err != nil {
		return servercore.RuntimeControlResult{}, fmt.Errorf("resolve Yemaka executable: %w", err)
	}
	dir, err := os.Getwd()
	if err != nil {
		return servercore.RuntimeControlResult{}, fmt.Errorf("resolve Yemaka working directory: %w", err)
	}
	launcher := a.restartLauncher
	if launcher == nil {
		launcher = startYemakaProcess
	}
	if err := launcher(executable, append([]string{}, os.Args[1:]...), dir, os.Environ()); err != nil {
		return servercore.RuntimeControlResult{}, fmt.Errorf("start Yemaka again: %w", err)
	}
	a.quitSoon()
	return servercore.RuntimeControlResult{
		Action:   "restart",
		Accepted: true,
		Message:  "Yemaka is restarting.",
	}, nil
}

func (a *App) RequestShutdown() (servercore.RuntimeControlResult, error) {
	a.quitSoon()
	return servercore.RuntimeControlResult{
		Action:   "shutdown",
		Accepted: true,
		Message:  "Yemaka is shutting down.",
	}, nil
}

func (a *App) quitSoon() {
	quitter := a.runtimeQuitter
	if quitter == nil {
		quitter = wailsruntime.Quit
	}
	ctx := a.ctxOrBackground()
	go func() {
		// Keep the Wails bridge alive long enough to return the action result.
		time.Sleep(150 * time.Millisecond)
		quitter(ctx)
	}()
}

func startYemakaProcess(path string, args []string, dir string, env []string) error {
	command := exec.Command(path, args...)
	command.Dir = dir
	command.Env = env
	return command.Start()
}

func (a *App) Status() (Status, error) {
	if err := a.ensure(); err != nil {
		return Status{}, err
	}
	report := diagnostics.Run(a.ctxOrBackground(), a.config, a.profile, a.store, a.modelRuntime)
	statuses := make([]ModelStatus, 0, len(report.ModelStatuses))
	for _, model := range report.ModelStatuses {
		configured := a.config.Models[model.Role]
		profileValid, profileStatus := a.modelProfileBindingStatus(configured.Profile)
		statuses = append(statuses, ModelStatus{
			Role:          model.Role,
			Name:          model.Name,
			Installed:     model.Installed,
			Profile:       strings.TrimSpace(configured.Profile),
			ProfileValid:  profileValid,
			ProfileStatus: profileStatus,
		})
	}
	return Status{
		ConfigPath:     report.ConfigPath,
		ProfilePath:    report.ProfilePath,
		SQLitePath:     report.SQLitePath,
		SQLiteOK:       report.SQLiteOK,
		SQLiteError:    report.SQLiteError,
		OllamaOK:       report.OllamaOK,
		OllamaError:    report.OllamaError,
		DefaultModel:   report.DefaultModel,
		LowMemoryModel: report.LowMemoryModel,
		SelectedModel:  report.SelectedModel,
		LowMemoryMode:  report.LowMemoryMode,
		SetupComplete:  report.SetupComplete,
		ModelReady:     report.ModelReady,
		RAGEnabled:     report.RAGEnabled,
		ShellEnabled:   report.ShellEnabled,
		Telemetry:      report.ConfigTelemetry,
		ModelStatuses:  statuses,
		SkillCount:     len(a.skills.Skills),
		Workspace:      a.workspaceRoot(),
	}, nil
}

func (a *App) OpsStatus(includeRelease bool, limit int) (servercore.OpsStatus, error) {
	if err := a.ensure(); err != nil {
		return servercore.OpsStatus{}, err
	}
	srv := servercore.New(servercore.Dependencies{
		Config:         a.config,
		Profile:        a.profile,
		Memory:         a.store,
		RAG:            a.rag,
		Skills:         a.skills,
		Runtime:        a.modelRuntime,
		RuntimeFactory: a.runtimeFactory,
		Cloud:          a.cloudRuntime,
		Router:         a.router,
	})
	return srv.BuildOpsStatus(a.ctxOrBackground(), includeRelease, limit)
}

func (a *App) SetupState() (SetupState, error) {
	if err := a.ensure(); err != nil {
		return SetupState{}, err
	}
	state := SetupState{
		SetupComplete:      a.config.App.SetupComplete,
		ConfigPath:         a.config.Path,
		ProfilePath:        a.profile.Root,
		LowMemoryMode:      a.config.Runtime.LowMemoryMode,
		Mode:               setupModeFromConfig(a.config),
		SelectedModel:      a.selectedModel().Name,
		InstalledModels:    []models.ModelInfo{},
		RecommendedMode:    "student_laptop",
		ConfiguredDefaults: configuredModelStatuses(a.config, nil),
	}
	if err := a.modelRuntime.Health(a.ctxOrBackground()); err != nil {
		state.OllamaError = err.Error()
		state.ModelReady = false
		return state, nil
	}
	state.OllamaOK = true
	installed, err := a.modelRuntime.ListModels(a.ctxOrBackground())
	if err != nil {
		state.OllamaError = err.Error()
		return state, nil
	}
	localModels := localInstalledModels(installed)
	state.InstalledModels = localModels
	state.RecommendedModel = recommendInstalledModel(localModels)
	if state.RecommendedModel == "" && len(localModels) > 0 {
		state.RecommendedModel = localModels[0].Name
	}
	state.ModelReady = modelInstalled(state.SelectedModel, localModels)
	state.ConfiguredDefaults = configuredModelStatuses(a.config, localModels)
	return state, nil
}

func (a *App) CompleteSetup(input SetupInput) (SetupState, error) {
	if err := a.ensure(); err != nil {
		return SetupState{}, err
	}
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "student_laptop"
	}
	if mode != "student_laptop" && mode != "useful_local" {
		return SetupState{}, fmt.Errorf("setup mode must be student_laptop or useful_local")
	}

	installed, _ := a.modelRuntime.ListModels(a.ctxOrBackground())
	localModels := localInstalledModels(installed)
	lowMemoryModel := strings.TrimSpace(input.LowMemoryModel)
	defaultModel := strings.TrimSpace(input.DefaultModel)
	if defaultModel == "" {
		defaultModel = lowMemoryModel
	}
	if lowMemoryModel != "" && len(localModels) > 0 && !modelInstalled(lowMemoryModel, localModels) {
		return SetupState{}, fmt.Errorf("low memory model is not an installed local model: %s", lowMemoryModel)
	}
	if defaultModel != "" && len(localModels) > 0 && !modelInstalled(defaultModel, localModels) {
		return SetupState{}, fmt.Errorf("default model is not an installed local model: %s", defaultModel)
	}

	a.config.App.SetupComplete = true
	a.config.App.Telemetry = false
	a.config.Runtime.LowMemoryMode = mode == "student_laptop"
	a.config.Runtime.MaxParallelTools = 1
	a.config.Runtime.MaxContextTokens = 4096
	if mode == "useful_local" {
		a.config.Runtime.MaxContextTokens = 6144
	}
	a.config.RAG.Enabled = true
	a.config.RAG.Mode = "sqlite_fts"
	a.config.Tools.Shell.Enabled = true
	a.config.Tools.Shell.MaxParallelCommands = 1
	a.config.Security.AllowNetworkByDefault = false
	a.config.Security.SecretsAccess = false

	if lowMemoryModel != "" {
		model := a.config.Models["low_memory"]
		model.Provider = "ollama"
		model.Name = lowMemoryModel
		if model.BaseURL == "" {
			model.BaseURL = "http://localhost:11434/api"
		}
		a.config.Models["low_memory"] = model
	}
	if defaultModel != "" {
		model := a.config.Models["default"]
		model.Provider = "ollama"
		model.Name = defaultModel
		if model.BaseURL == "" {
			model.BaseURL = "http://localhost:11434/api"
		}
		a.config.Models["default"] = model
	}

	if err := config.Write(a.config.Path, a.config); err != nil {
		return SetupState{}, err
	}
	a.router = models.NewRouter(a.config)
	runtime, err := modelruntime.New(a.selectedModel())
	if err != nil {
		return SetupState{}, err
	}
	a.modelRuntime = runtime
	if a.config.CloudFallback.Enabled {
		a.cloudRuntime = cloud.New(a.config.CloudFallback)
	} else {
		a.cloudRuntime = nil
	}
	return a.SetupState()
}

func (a *App) Settings() (SettingsView, error) {
	if err := a.ensure(); err != nil {
		return SettingsView{}, err
	}
	return settingsView(a.config), nil
}

func (a *App) SaveSettings(input SettingsInput) (SettingsView, error) {
	if err := a.ensure(); err != nil {
		return SettingsView{}, err
	}
	if err := applySettings(a.config, input); err != nil {
		return SettingsView{}, err
	}
	if err := config.Write(a.config.Path, a.config); err != nil {
		return SettingsView{}, err
	}
	a.router = models.NewRouter(a.config)
	runtime, err := modelruntime.New(a.selectedModel())
	if err != nil {
		return SettingsView{}, err
	}
	a.modelRuntime = runtime
	if a.config.CloudFallback.Enabled {
		a.cloudRuntime = cloud.New(a.config.CloudFallback)
	} else {
		a.cloudRuntime = nil
	}
	return settingsView(a.config), nil
}

func (a *App) ListModels() ([]models.ModelInfo, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.modelRuntime.ListModels(a.ctxOrBackground())
}

func (a *App) ShowModel(name string) (models.ModelDetails, error) {
	if err := a.ensure(); err != nil {
		return models.ModelDetails{}, err
	}
	detailer, ok := a.modelRuntime.(models.ModelDetailRuntime)
	if !ok {
		return models.ModelDetails{}, fmt.Errorf("configured runtime does not support model details")
	}
	return detailer.ShowModel(a.ctxOrBackground(), strings.TrimSpace(name))
}

func (a *App) GenerateModel(name string, prompt string) (ModelGenerateResult, error) {
	if err := a.ensure(); err != nil {
		return ModelGenerateResult{}, err
	}
	name = strings.TrimSpace(name)
	prompt = strings.TrimSpace(prompt)
	if name == "" || prompt == "" {
		return ModelGenerateResult{}, fmt.Errorf("model name and prompt are required")
	}
	generator, ok := a.modelRuntime.(models.GenerateRuntime)
	if !ok {
		return ModelGenerateResult{}, fmt.Errorf("configured runtime does not support direct generation")
	}
	var output strings.Builder
	runCtx, runID := a.beginRun()
	defer a.endRun(runID)
	if err := generator.GenerateStream(runCtx, models.GenerateRequest{
		Model:       name,
		Prompt:      prompt,
		Temperature: modelTemperature(a.config, name),
	}, func(event models.GenerateEvent) error {
		if event.Token != "" {
			_, err := output.WriteString(event.Token)
			return err
		}
		return nil
	}); err != nil {
		return ModelGenerateResult{}, err
	}
	return ModelGenerateResult{Text: output.String(), Model: name}, nil
}

func (a *App) SetModelRole(role string, name string) error {
	return a.SetModelRoleProvider(role, name, "", "")
}

func (a *App) SetModelRoleProvider(role string, name string, provider string, baseURL string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	role = normalizeModelRole(role)
	name = strings.TrimSpace(name)
	provider = strings.TrimSpace(provider)
	baseURL = strings.TrimSpace(baseURL)
	if !knownModelRole(role) {
		return fmt.Errorf("unknown model role %q", role)
	}
	if name == "" {
		return fmt.Errorf("model name is required")
	}
	model := a.config.Models[role]
	previousProvider := models.NormalizeProvider(model.Provider)
	if provider == "" {
		provider = model.Provider
	}
	provider = models.NormalizeProvider(provider)
	if !models.IsRuntimeProvider(provider) {
		return fmt.Errorf("unsupported model provider %q", provider)
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(model.BaseURL)
	}
	if baseURL == "" || provider != previousProvider {
		baseURL = modelruntime.DefaultBaseURL(provider)
	}
	if !modelruntime.IsLocalBaseURL(baseURL) {
		return fmt.Errorf("model runtime base URL must be local: %s", baseURL)
	}
	model.Provider = provider
	model.BaseURL = baseURL
	model.Name = name
	model.Profile = ""
	a.config.Models[role] = model
	if err := config.Write(a.config.Path, a.config); err != nil {
		return err
	}
	a.router = models.NewRouter(a.config)
	runtime, err := modelruntime.New(a.selectedModel())
	if err != nil {
		return err
	}
	a.modelRuntime = runtime
	return nil
}

func (a *App) ListModelProfiles() ([]modelprofiles.Profile, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.modelProfileStore().List()
}

func (a *App) ShowModelProfile(name string) (ModelProfileResult, error) {
	if err := a.ensure(); err != nil {
		return ModelProfileResult{}, err
	}
	store := a.modelProfileStore()
	profile, err := store.Load(strings.TrimSpace(name))
	if err != nil {
		return ModelProfileResult{}, err
	}
	result, err := desktopModelProfileResult(profile, "", "loaded")
	if err != nil {
		return ModelProfileResult{}, err
	}
	enrichDesktopModelProfileResult(store, &result)
	return result, nil
}

func (a *App) PreviewModelProfile(input ModelProfileInput) (ModelProfileResult, error) {
	if err := a.ensure(); err != nil {
		return ModelProfileResult{}, err
	}
	result, err := desktopModelProfileResult(desktopModelProfileFromInput(input), "", "preview")
	if err != nil {
		return ModelProfileResult{}, err
	}
	enrichDesktopModelProfileResult(a.modelProfileStore(), &result)
	return result, nil
}

func (a *App) DraftModelProfileFromConversation(input ModelProfileDraftInput) (ModelProfileResult, error) {
	if err := a.ensure(); err != nil {
		return ModelProfileResult{}, err
	}
	profile, report, err := learning.DraftModelProfileFromConversation(a.ctxOrBackground(), a.store, learning.ModelProfileDraftInput{
		ConversationID: input.ConversationID,
		Name:           input.Name,
		BaseModel:      input.BaseModel,
		Temperature:    input.Temperature,
		NumCtx:         input.NumCtx,
	})
	if err != nil {
		return ModelProfileResult{}, err
	}
	result, err := desktopModelProfileResult(profile, "", "drafted")
	if err != nil {
		return ModelProfileResult{}, err
	}
	enrichDesktopModelProfileResult(a.modelProfileStore(), &result)
	result.DraftReport = desktopModelProfileDraftReport(report)
	return result, nil
}

func (a *App) SaveModelProfile(input ModelProfileInput) (ModelProfileResult, error) {
	if err := a.ensure(); err != nil {
		return ModelProfileResult{}, err
	}
	store := a.modelProfileStore()
	profile := desktopModelProfileFromInput(input)
	comparison, _ := store.CompareCurrent(profile)
	path, err := store.Save(profile)
	if err != nil {
		return ModelProfileResult{}, err
	}
	result, err := desktopModelProfileResult(profile, path, "saved")
	if err != nil {
		return ModelProfileResult{}, err
	}
	result.Comparison = comparison
	enrichDesktopModelProfileResult(store, &result)
	return result, nil
}

func (a *App) ApplyModelProfile(input ModelProfileApplyInput) (ModelProfileResult, error) {
	if err := a.ensure(); err != nil {
		return ModelProfileResult{}, err
	}
	role := normalizeModelRole(input.Role)
	if !knownModelRole(role) {
		return ModelProfileResult{}, fmt.Errorf("unknown model role %q", role)
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return ModelProfileResult{}, fmt.Errorf("model profile name is required")
	}
	store := a.modelProfileStore()
	profile, err := store.Load(name)
	if err != nil {
		return ModelProfileResult{}, err
	}
	model := a.config.Models[role]
	provider := models.NormalizeProvider(model.Provider)
	if provider == "" {
		provider = models.ProviderOllama
	}
	model.Provider = provider
	if strings.TrimSpace(model.BaseURL) == "" {
		model.BaseURL = modelruntime.DefaultBaseURL(provider)
	}
	model.Name = profile.BaseModel
	model.Temperature = profile.Parameters.Temperature
	model.Profile = profile.Name
	a.config.Models[role] = model
	if err := config.Write(a.config.Path, a.config); err != nil {
		return ModelProfileResult{}, err
	}
	a.router = models.NewRouter(a.config)
	runtime, err := modelruntime.New(a.selectedModel())
	if err != nil {
		return ModelProfileResult{}, err
	}
	a.modelRuntime = runtime
	result, err := desktopModelProfileResult(profile, modelProfilePath(store.Dir, profile.Name), "applied")
	if err != nil {
		return ModelProfileResult{}, err
	}
	enrichDesktopModelProfileResult(store, &result)
	result.Applied = true
	result.AppliedRole = role
	return result, nil
}

func (a *App) ListSkills() ([]SkillSummary, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	items := a.skills.Statuses()
	result := make([]SkillSummary, 0, len(items))
	for _, skill := range items {
		result = append(result, skillSummary(skill))
	}
	return result, nil
}

func (a *App) ValidateSkill(name string) (SkillActionResult, error) {
	if err := a.ensure(); err != nil {
		return SkillActionResult{}, err
	}
	skill, ok := a.skills.Get(strings.TrimSpace(name))
	if !ok {
		return SkillActionResult{}, fmt.Errorf("skill not found: %s", name)
	}
	if err := skills.ValidateWithOptions(skill, skills.ValidationOptions{AvailableTools: availableSkillTools()}); err != nil {
		return SkillActionResult{}, err
	}
	return SkillActionResult{Skill: skillSummaryFromSkill(skill), Message: "valid"}, nil
}

func (a *App) SetSkillEnabled(name string, enabled bool) (SkillActionResult, error) {
	if err := a.ensure(); err != nil {
		return SkillActionResult{}, err
	}
	skill, ok := a.skills.Get(strings.TrimSpace(name))
	if !ok {
		return SkillActionResult{}, fmt.Errorf("skill not found: %s", name)
	}
	updated, err := skills.SetEnabled(a.profile.Skills, skill, enabled)
	if err != nil {
		return SkillActionResult{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return SkillActionResult{}, err
	}
	message := "enabled"
	if !enabled {
		message = "disabled"
	}
	return SkillActionResult{Skill: skillSummaryFromSkill(updated), Message: message}, nil
}

func (a *App) CreateSkillFromSession(conversationID string) (SkillActionResult, error) {
	if err := a.ensure(); err != nil {
		return SkillActionResult{}, err
	}
	created, err := a.createSkillFromSession(strings.TrimSpace(conversationID))
	if err != nil {
		return SkillActionResult{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return SkillActionResult{}, err
	}
	return SkillActionResult{Skill: skillSummaryFromSkill(created), Message: "created"}, nil
}

func (a *App) ImproveSkillFromSession(name string, conversationID string) (SkillActionResult, error) {
	if err := a.ensure(); err != nil {
		return SkillActionResult{}, err
	}
	improved, err := a.improveSkillFromSession(strings.TrimSpace(name), strings.TrimSpace(conversationID))
	if err != nil {
		return SkillActionResult{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return SkillActionResult{}, err
	}
	return SkillActionResult{Skill: skillSummaryFromSkill(improved), Message: "improved"}, nil
}

func (a *App) ImportSkill(path string) (SkillActionResult, error) {
	if err := a.ensure(); err != nil {
		return SkillActionResult{}, err
	}
	imported, err := skills.Import(a.profile.Skills, strings.TrimSpace(path))
	if err != nil {
		return SkillActionResult{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return SkillActionResult{}, err
	}
	return SkillActionResult{Skill: skillSummaryFromSkill(imported), Message: "imported"}, nil
}

func (a *App) ExportSkill(name string, path string) (SkillActionResult, error) {
	if err := a.ensure(); err != nil {
		return SkillActionResult{}, err
	}
	skill, ok := a.skills.Get(strings.TrimSpace(name))
	if !ok {
		return SkillActionResult{}, fmt.Errorf("skill not found: %s", name)
	}
	exportedPath, err := skills.Export(skill, strings.TrimSpace(path))
	if err != nil {
		return SkillActionResult{}, err
	}
	return SkillActionResult{Skill: skillSummaryFromSkill(skill), Message: "exported", Path: exportedPath}, nil
}

func (a *App) ListDomainPacks() ([]domainpacks.PackStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return domainpacks.List(a.domainPackRoot())
}

func (a *App) ListDomainPackTemplates() ([]workflows.TemplateSummary, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	packs, err := domainpacks.List(a.domainPackRoot())
	if err != nil {
		return nil, err
	}
	catalog, err := desktopBuiltInDomainPackTemplateCatalog()
	if err != nil {
		return nil, err
	}
	return workflows.DomainPackTemplateSummaries(catalog, packs), nil
}

func (a *App) ListDomainPackSkills() ([]DomainPackSkillStatus, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	packs, err := domainpacks.List(a.domainPackRoot())
	if err != nil {
		return nil, err
	}
	statuses := make([]DomainPackSkillStatus, 0)
	for _, pack := range packs {
		active := pack.Enabled && pack.Valid
		for _, ref := range pack.Skills {
			statuses = append(statuses, DomainPackSkillStatus{
				PackName:        pack.Name,
				Ref:             ref,
				Active:          active,
				Enabled:         pack.Enabled,
				Valid:           pack.Valid,
				ValidationError: pack.ValidationError,
				PackDir:         pack.Dir,
			})
		}
	}
	return statuses, nil
}

func (a *App) ReviewDomainPack(name string) (domainpacks.PackReview, error) {
	if err := a.ensure(); err != nil {
		return domainpacks.PackReview{}, err
	}
	return domainpacks.ReviewInstalled(a.domainPackRoot(), strings.TrimSpace(name))
}

func (a *App) ReviewDomainPackTemplate(name string) (domainpacks.PackReview, error) {
	if err := a.ensure(); err != nil {
		return domainpacks.PackReview{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return domainpacks.PackReview{}, fmt.Errorf("domain pack template name is required")
	}
	packs, err := domainpacks.List(a.domainPackRoot())
	if err != nil {
		return domainpacks.PackReview{}, err
	}
	catalog, err := desktopBuiltInDomainPackTemplateCatalog()
	if err != nil {
		return domainpacks.PackReview{}, err
	}
	template, ok := catalog.Get(name)
	if !ok {
		return domainpacks.PackReview{}, fmt.Errorf("domain pack template not found: %s", name)
	}
	return domainpacks.ReviewDir(template.Path, workflows.SourceDomainPackTemplate, desktopDomainPackStatusByName(packs, template.Name))
}

func (a *App) InstallDomainPack(path string) (domainpacks.PackStatus, error) {
	if err := a.ensure(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	status, err := domainpacks.Install(a.domainPackRoot(), strings.TrimSpace(path))
	if err != nil {
		return domainpacks.PackStatus{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	return status, nil
}

func (a *App) InstallDomainPackTemplate(name string) (domainpacks.PackStatus, error) {
	if err := a.ensure(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	status, err := a.installBuiltInDomainPackTemplate(strings.TrimSpace(name), "")
	if err != nil {
		return domainpacks.PackStatus{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	return status, nil
}

func (a *App) SetDomainPackEnabled(name string, enabled bool) (domainpacks.PackStatus, error) {
	if err := a.ensure(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	status, err := domainpacks.SetEnabled(a.domainPackRoot(), strings.TrimSpace(name), enabled)
	if err != nil {
		return domainpacks.PackStatus{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	return status, nil
}

func (a *App) UninstallDomainPack(name string) (domainpacks.PackStatus, error) {
	if err := a.ensure(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	status, err := domainpacks.Uninstall(a.domainPackRoot(), strings.TrimSpace(name))
	if err != nil {
		return domainpacks.PackStatus{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return domainpacks.PackStatus{}, err
	}
	return status, nil
}

func (a *App) ApplyCapability(input CapabilityApplyInput) (CapabilityApplyResult, error) {
	if err := a.ensure(); err != nil {
		return CapabilityApplyResult{}, err
	}
	if !input.Approved {
		return CapabilityApplyResult{}, fmt.Errorf("approved flag is required before applying a capability handoff")
	}
	if kind := strings.TrimSpace(input.Kind); kind != "" && kind != "domain_pack" {
		return CapabilityApplyResult{}, fmt.Errorf("capability kind %s is not supported by this apply flow", kind)
	}

	source := strings.TrimSpace(input.PackSource)
	name := strings.TrimSpace(firstNonEmptyDesktopString(input.ExistingCapability, input.Name))
	var status domainpacks.PackStatus
	action := ""
	var err error
	switch source {
	case "domain_pack_template":
		status, err = a.installBuiltInDomainPackTemplate(name, strings.TrimSpace(input.PackDir))
		action = "installed"
	case "", "domain_pack":
		if name == "" {
			return CapabilityApplyResult{}, fmt.Errorf("domain pack name is required")
		}
		status, err = domainpacks.SetEnabled(a.domainPackRoot(), name, true)
		action = "enabled"
	default:
		return CapabilityApplyResult{}, fmt.Errorf("unsupported domain pack source: %s", source)
	}
	if err != nil {
		return CapabilityApplyResult{}, err
	}
	if err := a.reloadSkills(); err != nil {
		return CapabilityApplyResult{}, err
	}
	return CapabilityApplyResult{
		Pack:    status,
		Action:  action,
		Message: action,
	}, nil
}

func (a *App) installBuiltInDomainPackTemplate(name string, packDir string) (domainpacks.PackStatus, error) {
	catalog, err := desktopBuiltInDomainPackTemplateCatalog()
	if err != nil {
		return domainpacks.PackStatus{}, err
	}
	template, ok := catalog.Get(name)
	if !ok && packDir != "" {
		for _, candidate := range catalog.List() {
			if filepath.Clean(candidate.Path) == filepath.Clean(packDir) {
				template = candidate
				ok = true
				break
			}
		}
	}
	if !ok {
		return domainpacks.PackStatus{}, fmt.Errorf("built-in domain pack template not found: %s", name)
	}
	return domainpacks.Install(a.domainPackRoot(), template.Path)
}

func desktopBuiltInDomainPackTemplateCatalog() (workflows.TemplateCatalog, error) {
	root, err := workflows.BuiltInTemplateRoot()
	if err != nil {
		return workflows.TemplateCatalog{}, err
	}
	return workflows.NewBuiltInTemplateCatalog(root)
}

func desktopDomainPackStatusByName(statuses []domainpacks.PackStatus, name string) *domainpacks.PackStatus {
	name = strings.TrimSpace(name)
	for _, status := range statuses {
		if status.Name == name {
			statusCopy := status
			return &statusCopy
		}
	}
	return nil
}

func firstNonEmptyDesktopString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (a *App) ListExtensions() ([]extensions.Status, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.extensionStore().List()
}

func (a *App) ExtensionFailures(limit int) ([]extensions.FailureTrend, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.extensionStore().Failures(limit)
}

func (a *App) ShowExtension(name string) (extensions.Detail, error) {
	if err := a.ensure(); err != nil {
		return extensions.Detail{}, err
	}
	return a.extensionStore().Show(strings.TrimSpace(name))
}

func (a *App) InspectExtension(name string) (extensions.PackageInspection, error) {
	if err := a.ensure(); err != nil {
		return extensions.PackageInspection{}, err
	}
	return a.extensionStore().InspectPackage(strings.TrimSpace(name))
}

func (a *App) ReviewExtension(name string) (extensions.Review, error) {
	if err := a.ensure(); err != nil {
		return extensions.Review{}, err
	}
	return a.extensionStore().Review(strings.TrimSpace(name))
}

func (a *App) ProposeExtension(input ExtensionProposalInput) (extensions.Proposal, error) {
	if err := a.ensure(); err != nil {
		return extensions.Proposal{}, err
	}
	return a.extensionStore().Propose(input.Request)
}

func (a *App) ProposeCapability(input ExtensionProposalInput) (agent.CapabilityGapProposal, error) {
	if err := a.ensure(); err != nil {
		return agent.CapabilityGapProposal{}, err
	}
	return agent.NewCapabilityGapRouter(a.config, a.extensionStore()).ProposeRequest(input.Request)
}

func (a *App) GenerateCapability(input CapabilityGenerateInput) (agent.CapabilityGenerationResult, error) {
	if err := a.ensure(); err != nil {
		return agent.CapabilityGenerationResult{}, err
	}
	store := a.extensionStore()
	draftModel := config.ModelConfig{}
	if input.SynthesizeDraft {
		var err error
		draftModel, err = agent.SelectCapabilityDraftModel(a.config)
		if err != nil {
			return agent.CapabilityGenerationResult{}, err
		}
	}
	result, err := agent.NewCapabilityGapRouter(a.config, store).Generate(a.ctxOrBackground(), store, agent.CapabilityGenerationInput{
		Request:             strings.TrimSpace(input.Request),
		Approved:            input.Approved,
		Name:                strings.TrimSpace(input.Name),
		AllowedDomains:      input.AllowedDomains,
		MaxRuntimeSeconds:   a.config.Extensions.MaxRuntimeSeconds,
		RunAfterGenerate:    input.RunAfterGenerate,
		RunInput:            input.RunInput,
		Draft:               input.Draft,
		SynthesizeDraft:     input.SynthesizeDraft,
		DraftRepairAttempts: input.DraftRepairAttempts,
		DraftModel:          draftModel,
		DraftRuntimeFactory: a.runtimeFactory,
		RunOptions: extensions.RunOptions{
			Input:             input.RunInput,
			MaxRuntimeSeconds: a.config.Extensions.MaxRuntimeSeconds,
			Internet:          a.internetService(),
		},
	})
	if err != nil {
		return agent.CapabilityGenerationResult{}, err
	}
	if input.ScheduleAfterGenerate {
		if err := a.attachGeneratedCapabilityJob(&result, input); err != nil {
			return agent.CapabilityGenerationResult{}, err
		}
	}
	if a.store != nil {
		_, _ = a.store.SaveMemory(a.ctxOrBackground(), memory.Memory{
			Kind:       "capability_generation",
			Content:    result.Generation.SkillCandidate,
			Importance: 4,
			Source:     "capability_generator",
		})
	}
	return result, nil
}

func (a *App) attachGeneratedCapabilityJob(result *agent.CapabilityGenerationResult, input CapabilityGenerateInput) error {
	if result == nil {
		return fmt.Errorf("capability result is required")
	}
	extensionName := strings.TrimSpace(result.Generation.Extension.Name)
	if extensionName == "" {
		return fmt.Errorf("generated extension name is required before scheduling")
	}
	ctx := a.ctxOrBackground()
	store, err := a.schedulerStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	scheduleType := strings.TrimSpace(input.ScheduleType)
	if scheduleType == "" {
		scheduleType = scheduler.ScheduleManual
	}
	scheduleInput := generatedCapabilityScheduleInput(input, result)
	if err := a.validateRequiredExtensionJobInput(extensionName, scheduleInput); err != nil {
		return err
	}
	job, err := store.Create(ctx, scheduler.CreateInput{
		Name:         strings.TrimSpace(input.ScheduleName),
		ScheduleType: scheduleType,
		ScheduleExpr: strings.TrimSpace(input.ScheduleExpr),
		TargetType:   scheduler.TargetExtension,
		TargetName:   extensionName,
		Input:        scheduleInput,
		Approved:     true,
		Enabled:      input.ScheduleEnabled,
	})
	if err != nil {
		return err
	}
	result.Schedule = capabilityScheduledJob(job)
	result.NextSteps = append([]string{capabilityJobNextStep(job)}, result.NextSteps...)
	if input.ScheduleEnabled {
		result.Message = "capability generated, tested, registered, and scheduled after explicit approval"
	} else {
		result.Message = "capability generated, tested, registered, and attached to an approved disabled job"
	}
	return nil
}

func generatedCapabilityScheduleInput(input CapabilityGenerateInput, result *agent.CapabilityGenerationResult) map[string]any {
	if !mapEmpty(input.ScheduleInput) {
		return cloneInputMap(input.ScheduleInput)
	}
	if !mapEmpty(input.RunInput) {
		return cloneInputMap(input.RunInput)
	}
	if result == nil {
		return map[string]any{}
	}
	return agent.SuggestedCapabilityInput(result.Proposal, input.Request)
}

func mapEmpty(value map[string]any) bool {
	return len(value) == 0
}

func cloneInputMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	clone := make(map[string]any, len(value))
	for key, item := range value {
		clone[key] = item
	}
	return clone
}

func (a *App) validateRequiredExtensionJobInput(targetName string, input map[string]any) error {
	name := strings.TrimSpace(targetName)
	if err := a.extensionStore().ValidateRunInput(name, input); err != nil {
		if errors.Is(err, extensions.ErrExtensionNotFound) {
			return fmt.Errorf("extension target %q is not installed; generate and register the extension before creating or enabling a scheduler job", name)
		}
		return err
	}
	return nil
}

func (a *App) validateKnownExtensionJobInput(targetName string, input map[string]any) error {
	return a.extensionStore().ValidateKnownRunInput(strings.TrimSpace(targetName), input)
}

func (a *App) annotateSchedulerJobs(jobs []scheduler.Job) ([]scheduler.Job, int) {
	return scheduler.AnnotateJobInputStatuses(jobs, func(job scheduler.Job) (bool, error) {
		if strings.TrimSpace(job.TargetType) != scheduler.TargetExtension {
			return false, nil
		}
		err := a.extensionStore().ValidateRunInput(job.TargetName, job.Input)
		if errors.Is(err, extensions.ErrExtensionNotFound) {
			return false, nil
		}
		return true, err
	})
}

func capabilityScheduledJob(job scheduler.Job) *agent.CapabilityScheduledJob {
	return &agent.CapabilityScheduledJob{
		ID:           job.ID,
		Name:         job.Name,
		ScheduleType: job.ScheduleType,
		ScheduleExpr: job.ScheduleExpr,
		TargetType:   job.TargetType,
		TargetName:   job.TargetName,
		Input:        job.Input,
		Enabled:      job.Enabled,
		Approved:     job.Approved,
		NextDueAt:    job.NextDueAt,
	}
}

func capabilityJobNextStep(job scheduler.Job) string {
	if job.Enabled {
		return "Scheduler job `" + job.ID + "` is enabled and will run when due."
	}
	if job.ScheduleType == scheduler.ScheduleManual {
		return "Run the approved manual job when ready with `yemaka job run " + job.ID + "`."
	}
	return "Enable the approved job when ready with `yemaka job enable " + job.ID + "`."
}

func (a *App) GenerateExtension(input ExtensionGenerateInput) (extensions.GenerationResult, error) {
	if err := a.ensure(); err != nil {
		return extensions.GenerationResult{}, err
	}
	result, err := a.extensionStore().Generate(a.ctxOrBackground(), extensions.GenerateInput{
		Name:              strings.TrimSpace(input.Name),
		Description:       strings.TrimSpace(input.Description),
		Approved:          input.Approved,
		MaxRuntimeSeconds: a.config.Extensions.MaxRuntimeSeconds,
		BrokeredNetwork:   input.BrokeredNetwork,
		AllowedDomains:    input.AllowedDomains,
		Draft:             input.Draft,
	})
	if err != nil {
		return extensions.GenerationResult{}, err
	}
	if a.store != nil {
		_, _ = a.store.SaveMemory(a.ctxOrBackground(), memory.Memory{
			Kind:       "extension_generation",
			Content:    result.SkillCandidate,
			Importance: 4,
			Source:     "extension_generator",
		})
	}
	return result, nil
}

func (a *App) ValidateExtension(name string) (ExtensionActionResult, error) {
	if err := a.ensure(); err != nil {
		return ExtensionActionResult{}, err
	}
	result, err := a.extensionStore().Validate(strings.TrimSpace(name))
	if err != nil {
		return ExtensionActionResult{}, err
	}
	return ExtensionActionResult{Extension: result.Extension, Message: result.Message}, nil
}

func (a *App) TestExtension(name string) (extensions.TestResult, error) {
	if err := a.ensure(); err != nil {
		return extensions.TestResult{}, err
	}
	return a.extensionStore().Test(a.ctxOrBackground(), strings.TrimSpace(name), extensions.TestOptions{})
}

func (a *App) RegisterExtension(name string) (extensions.TestResult, error) {
	if err := a.ensure(); err != nil {
		return extensions.TestResult{}, err
	}
	return a.extensionStore().RegisterGenerated(a.ctxOrBackground(), strings.TrimSpace(name), extensions.TestOptions{})
}

func (a *App) SetExtensionEnabled(name string, enabled bool) (ExtensionActionResult, error) {
	if err := a.ensure(); err != nil {
		return ExtensionActionResult{}, err
	}
	result, err := a.extensionStore().SetEnabled(strings.TrimSpace(name), enabled)
	if err != nil {
		return ExtensionActionResult{}, err
	}
	return ExtensionActionResult{Extension: result.Extension, Message: result.Message}, nil
}

func (a *App) DeleteExtension(name string) (ExtensionActionResult, error) {
	if err := a.ensure(); err != nil {
		return ExtensionActionResult{}, err
	}
	result, err := a.extensionStore().Delete(strings.TrimSpace(name))
	if err != nil {
		return ExtensionActionResult{}, err
	}
	return ExtensionActionResult{Extension: result.Extension, Message: result.Message}, nil
}

func (a *App) RollbackExtension(input ExtensionRollbackInput) (extensions.RollbackResult, error) {
	if err := a.ensure(); err != nil {
		return extensions.RollbackResult{}, err
	}
	return a.extensionStore().RollbackGeneration(input.NameOrSnapshot)
}

func (a *App) RunExtension(input ExtensionRunInput) (extensions.RunResult, error) {
	if err := a.ensure(); err != nil {
		return extensions.RunResult{}, err
	}
	return a.extensionStore().Run(a.ctxOrBackground(), strings.TrimSpace(input.Name), extensions.RunOptions{
		Input:             input.Input,
		MaxRuntimeSeconds: a.config.Extensions.MaxRuntimeSeconds,
		Internet:          a.internetService(),
	})
}

func (a *App) InternetStatus() (internet.Status, error) {
	if err := a.ensure(); err != nil {
		return internet.Status{}, err
	}
	return a.internetService().Status(), nil
}

func (a *App) ListConnectors() ([]connectors.Status, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return connectors.List(a.config.Connectors), nil
}

func (a *App) ConnectorRegistry() ([]connectors.RegistryEntry, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.generatedConnectorStore().Registry(a.config.Connectors)
}

func (a *App) ListGeneratedConnectors() ([]connectors.GeneratedEntry, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.generatedConnectorStore().List()
}

func (a *App) InstallGeneratedConnector(input connectors.GeneratedInstallInput) (connectors.GeneratedActionResult, error) {
	if err := a.ensure(); err != nil {
		return connectors.GeneratedActionResult{}, err
	}
	return a.generatedConnectorStore().Install(input)
}

func (a *App) SetGeneratedConnectorEnabled(input GeneratedConnectorEnabledInput) (connectors.GeneratedActionResult, error) {
	if err := a.ensure(); err != nil {
		return connectors.GeneratedActionResult{}, err
	}
	return a.generatedConnectorStore().SetEnabled(input.Name, input.Enabled)
}

func (a *App) InternetFetch(input InternetFetchInput) (internet.FetchResult, error) {
	if err := a.ensure(); err != nil {
		return internet.FetchResult{}, err
	}
	return a.internetService().Fetch(a.ctxOrBackground(), internet.FetchInput{
		URL:            input.URL,
		Method:         "GET",
		ExtractText:    input.ExtractText,
		AllowedDomains: input.AllowedDomains,
		TaskApproved:   input.TaskApproved,
		Caller:         "desktop",
	})
}

func (a *App) InternetHead(input InternetFetchInput) (internet.FetchResult, error) {
	if err := a.ensure(); err != nil {
		return internet.FetchResult{}, err
	}
	return a.internetService().Head(a.ctxOrBackground(), internet.FetchInput{
		URL:            input.URL,
		AllowedDomains: input.AllowedDomains,
		TaskApproved:   input.TaskApproved,
		Caller:         "desktop",
	})
}

func (a *App) InternetSearch(input InternetSearchInput) (internet.SearchResult, error) {
	if err := a.ensure(); err != nil {
		return internet.SearchResult{}, err
	}
	return a.internetService().Search(a.ctxOrBackground(), internet.SearchInput{
		Query:        input.Query,
		TaskApproved: input.TaskApproved,
		MaxResults:   input.MaxResults,
		Caller:       "desktop",
	})
}

func (a *App) InternetCrawl(input InternetCrawlInput) (internet.CrawlResult, error) {
	if err := a.ensure(); err != nil {
		return internet.CrawlResult{}, err
	}
	return a.internetService().Crawl(a.ctxOrBackground(), internet.CrawlInput{
		SeedURL:            input.SeedURL,
		Method:             input.Method,
		ExtractText:        input.ExtractText,
		AllowedDomains:     input.AllowedDomains,
		MaxPages:           input.MaxPages,
		MaxDepth:           input.MaxDepth,
		MaxDurationSeconds: input.MaxDurationSeconds,
		MaxLinksPerPage:    input.MaxLinksPerPage,
		MaxTextChars:       input.MaxTextChars,
		TaskApproved:       input.TaskApproved,
		Caller:             "desktop",
	})
}

func (a *App) InternetCrawls(limit int) ([]internet.CrawlRunRecord, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.internetService().CrawlRuns(limit)
}

func (a *App) InternetCrawlRun(id string) (internet.CrawlRunRecord, error) {
	if err := a.ensure(); err != nil {
		return internet.CrawlRunRecord{}, err
	}
	return a.internetService().CrawlRun(id)
}

func (a *App) InternetCache() ([]internet.CacheSummary, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.internetService().CacheList()
}

func (a *App) InternetRequests(limit int) ([]internet.RequestRecord, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.internetService().Requests(limit)
}

func (a *App) ListJobs() ([]scheduler.Job, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	store, err := a.schedulerStore(a.ctxOrBackground())
	if err != nil {
		return nil, err
	}
	defer store.Close()
	jobs, err := store.List(a.ctxOrBackground())
	if err != nil {
		return nil, err
	}
	jobs, _ = a.annotateSchedulerJobs(jobs)
	return jobs, nil
}

func (a *App) CreateJob(input JobCreateInput) (scheduler.Job, error) {
	if err := a.ensure(); err != nil {
		return scheduler.Job{}, err
	}
	if a.config.Scheduler.RequireApprovalForNewJobs && !input.Approved {
		return scheduler.Job{}, fmt.Errorf("new jobs require approval")
	}
	if err := scheduler.ValidateTarget(input.TargetType, input.TargetName); err != nil {
		return scheduler.Job{}, err
	}
	if strings.TrimSpace(input.TargetType) == scheduler.TargetExtension {
		if err := a.validateRequiredExtensionJobInput(input.TargetName, input.Input); err != nil {
			return scheduler.Job{}, err
		}
	}
	store, err := a.schedulerStore(a.ctxOrBackground())
	if err != nil {
		return scheduler.Job{}, err
	}
	defer store.Close()
	return store.Create(a.ctxOrBackground(), scheduler.CreateInput{
		Name:         input.Name,
		ScheduleType: input.ScheduleType,
		ScheduleExpr: input.ScheduleExpr,
		TargetType:   input.TargetType,
		TargetName:   input.TargetName,
		Input:        input.Input,
		Approved:     input.Approved || !a.config.Scheduler.RequireApprovalForNewJobs,
		Enabled:      input.Enabled,
	})
}

func (a *App) UpdateJobInput(input JobInputUpdateInput) (scheduler.Job, error) {
	if err := a.ensure(); err != nil {
		return scheduler.Job{}, err
	}
	if !input.Approved {
		return scheduler.Job{}, fmt.Errorf("use approved=true to approve updating the local scheduled job input")
	}
	ctx := a.ctxOrBackground()
	store, err := a.schedulerStore(ctx)
	if err != nil {
		return scheduler.Job{}, err
	}
	defer store.Close()
	job, err := store.Get(ctx, input.ID)
	if err != nil {
		return scheduler.Job{}, err
	}
	if job.TargetType == scheduler.TargetExtension {
		if err := a.validateRequiredExtensionJobInput(job.TargetName, input.Input); err != nil {
			return scheduler.Job{}, err
		}
	}
	updated, err := store.UpdateInput(ctx, scheduler.UpdateInput{ID: input.ID, Input: input.Input})
	if err != nil {
		return scheduler.Job{}, err
	}
	annotated, _ := a.annotateSchedulerJobs([]scheduler.Job{updated})
	if len(annotated) == 1 {
		updated = annotated[0]
	}
	return updated, nil
}

func (a *App) SetJobEnabled(input JobEnabledInput) (scheduler.Job, error) {
	if err := a.ensure(); err != nil {
		return scheduler.Job{}, err
	}
	store, err := a.schedulerStore(a.ctxOrBackground())
	if err != nil {
		return scheduler.Job{}, err
	}
	defer store.Close()
	if input.Enabled {
		job, err := store.Get(a.ctxOrBackground(), input.ID)
		if err != nil {
			return scheduler.Job{}, err
		}
		if job.TargetType == scheduler.TargetExtension {
			if err := a.validateRequiredExtensionJobInput(job.TargetName, job.Input); err != nil {
				return scheduler.Job{}, err
			}
		}
	}
	return store.SetEnabled(a.ctxOrBackground(), input.ID, input.Enabled)
}

func (a *App) RunJob(input JobRunInput) (scheduler.JobRun, error) {
	if err := a.ensure(); err != nil {
		return scheduler.JobRun{}, err
	}
	store, err := a.schedulerStore(a.ctxOrBackground())
	if err != nil {
		return scheduler.JobRun{}, err
	}
	defer store.Close()
	ctx := a.ctxOrBackground()
	job, jobErr := store.Get(ctx, input.ID)
	if jobErr == nil && job.TargetType == scheduler.TargetExtension {
		if err := a.validateKnownExtensionJobInput(job.TargetName, job.Input); err != nil {
			return scheduler.JobRun{}, err
		}
	}
	run, err := store.Run(ctx, input.ID, a.jobRunner(), a.jobRunOptions())
	if jobErr == nil {
		a.recordJobRunNotification(job, run)
	}
	if jobErr == nil && a.store != nil {
		if _, recordErr := agent.RecordJobRunResult(ctx, a.store, job, run, input.ConversationID); recordErr != nil && err == nil {
			return scheduler.JobRun{}, recordErr
		}
	}
	return run, err
}

func (a *App) RunDueJobs() (scheduler.TickResult, error) {
	if err := a.ensure(); err != nil {
		return scheduler.TickResult{}, err
	}
	ctx := a.ctxOrBackground()
	store, err := a.schedulerStore(ctx)
	if err != nil {
		return scheduler.TickResult{}, err
	}
	defer store.Close()
	loop := scheduler.Loop{
		Store:   store,
		Runner:  a.jobRunner(),
		Options: a.jobRunOptions(),
	}
	result, err := loop.Tick(ctx)
	if err != nil {
		return result, err
	}
	if err := a.recordJobRunMemories(ctx, store, result.Runs, ""); err != nil {
		return result, err
	}
	return result, nil
}

func (a *App) ArchiveJob(input JobArchiveInput) (scheduler.Job, error) {
	if err := a.ensure(); err != nil {
		return scheduler.Job{}, err
	}
	if !input.Approved {
		return scheduler.Job{}, fmt.Errorf("use approved=true to archive the local scheduled job")
	}
	store, err := a.schedulerStore(a.ctxOrBackground())
	if err != nil {
		return scheduler.Job{}, err
	}
	defer store.Close()
	return store.Archive(a.ctxOrBackground(), input.ID)
}

func (a *App) recordJobRunMemories(ctx context.Context, store *scheduler.Store, runs []scheduler.JobRun, conversationID string) error {
	if len(runs) == 0 {
		return nil
	}
	for _, run := range runs {
		if strings.TrimSpace(run.JobID) == "" {
			continue
		}
		job, err := store.Get(ctx, run.JobID)
		if err != nil {
			continue
		}
		a.recordJobRunNotification(job, run)
		if a.store == nil {
			continue
		}
		if _, err := agent.RecordJobRunResult(ctx, a.store, job, run, conversationID); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) recordJobRunNotification(job scheduler.Job, run scheduler.JobRun) {
	_, _, _ = notifications.RecordJobRunNotification(a.notificationStore(), job, run)
}

func (a *App) JobStatus() (scheduler.Status, error) {
	if err := a.ensure(); err != nil {
		return scheduler.Status{}, err
	}
	store, err := a.schedulerStore(a.ctxOrBackground())
	if err != nil {
		return scheduler.Status{}, err
	}
	defer store.Close()
	status, err := store.Status(a.ctxOrBackground(), a.config.Scheduler.Enabled, a.schedulerMaxParallel())
	if err != nil {
		return scheduler.Status{}, err
	}
	if jobs, listErr := store.List(a.ctxOrBackground()); listErr == nil {
		_, invalid := a.annotateSchedulerJobs(jobs)
		status.InvalidInputJobs = invalid
	}
	return status, nil
}

func (a *App) JobRuns(limit int) ([]scheduler.JobRun, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	store, err := a.schedulerStore(a.ctxOrBackground())
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return store.LastRuns(a.ctxOrBackground(), limit)
}

func (a *App) HeartbeatStatus() (heartbeat.Report, error) {
	if err := a.ensure(); err != nil {
		return heartbeat.Report{}, err
	}
	report, err := a.heartbeatReport(a.ctxOrBackground())
	if err != nil {
		return heartbeat.Report{}, err
	}
	store, err := heartbeat.Open(a.ctxOrBackground(), a.profile.Database)
	if err != nil {
		return heartbeat.Report{}, err
	}
	defer store.Close()
	previous, _ := store.Latest(a.ctxOrBackground())
	if err := store.Record(a.ctxOrBackground(), report); err != nil {
		return heartbeat.Report{}, err
	}
	a.recordHeartbeatTransitionNotification(previous, report)
	return report, nil
}

func (a *App) recordHeartbeatTransitionNotification(previous heartbeat.Report, report heartbeat.Report) {
	_, _ = notifications.RecordHeartbeatTransitionNotifications(a.notificationStore(), previous, report)
}

func (a *App) PolicyStatus() (PolicyStatusResult, error) {
	if err := a.ensure(); err != nil {
		return PolicyStatusResult{}, err
	}
	return policyStatusView(a.config), nil
}

func (a *App) SetPolicyMode(mode string) (PolicyStatusResult, error) {
	if err := a.ensure(); err != nil {
		return PolicyStatusResult{}, err
	}
	if err := applyPolicyMode(a.config, mode); err != nil {
		return PolicyStatusResult{}, err
	}
	if err := config.Write(a.config.Path, a.config); err != nil {
		return PolicyStatusResult{}, err
	}
	return policyStatusView(a.config), nil
}

func (a *App) LearningReport() (learning.Report, error) {
	if err := a.ensure(); err != nil {
		return learning.Report{}, err
	}
	return learning.BuildReport(a.ctxOrBackground(), a.store, a.extensionStore())
}

func (a *App) QAReview(limit int) (learning.QAReview, error) {
	if err := a.ensure(); err != nil {
		return learning.QAReview{}, err
	}
	review, err := learning.BuildQAReview(a.ctxOrBackground(), a.store, a.replayTraceStore(), a.latestQAEvalSummary(), limit)
	if err != nil {
		return learning.QAReview{}, err
	}
	if a.profile != nil && strings.TrimSpace(a.profile.Root) != "" {
		records, err := learning.ListQARegressionReviews(filepath.Join(a.profile.Root, "qa_review", "regression_reviews"))
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionReviews(&review, records)
		drafts, err := learning.ListQARegressionTestDrafts(filepath.Join(a.profile.Root, "qa_review", "regression_test_drafts"))
		if err != nil {
			return learning.QAReview{}, err
		}
		approvals, err := learning.ListQARegressionApprovedRecords(filepath.Join(a.profile.Root, "qa_review", "approved_regressions"))
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionDrafts(&review, drafts, approvals)
		patches, err := learning.ListQARegressionSourcePatchDrafts(filepath.Join(a.profile.Root, "qa_review", "regression_source_patches"))
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionSourcePatches(&review, patches)
		applyPlans, err := learning.ListQARegressionSourcePatchApplyPlans(filepath.Join(a.profile.Root, "qa_review", "regression_source_patch_apply_plans"))
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionSourcePatchApplyPlans(&review, applyPlans)
		sourceWrites, err := learning.ListQARegressionSourceWrites(filepath.Join(a.profile.Root, "qa_review", "regression_source_writes"))
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionSourceWrites(&review, sourceWrites)
	}
	return review, nil
}

func (a *App) PromoteQARegressionSuggestion(input learning.QARegressionPromotionRequest) (learning.QARegressionPromotionArtifact, error) {
	if err := a.ensure(); err != nil {
		return learning.QARegressionPromotionArtifact{}, err
	}
	if a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return learning.QARegressionPromotionArtifact{}, fmt.Errorf("profile root is required")
	}
	dir := filepath.Join(a.profile.Root, "qa_review", "regression_promotions")
	return learning.PromoteQARegressionSuggestion(dir, a.replayTraceStore(), a.latestQAEvalSummary(), input)
}

func (a *App) ReviewQARegressionSuggestion(input learning.QARegressionReviewRequest) (learning.QARegressionReviewRecord, error) {
	if err := a.ensure(); err != nil {
		return learning.QARegressionReviewRecord{}, err
	}
	if a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return learning.QARegressionReviewRecord{}, fmt.Errorf("profile root is required")
	}
	dir := filepath.Join(a.profile.Root, "qa_review", "regression_reviews")
	return learning.ReviewQARegressionSuggestion(dir, a.replayTraceStore(), a.latestQAEvalSummary(), input)
}

func (a *App) GenerateQARegressionTestDraft(input learning.QARegressionTestDraftRequest) (learning.QARegressionTestDraftRecord, error) {
	if err := a.ensure(); err != nil {
		return learning.QARegressionTestDraftRecord{}, err
	}
	if a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return learning.QARegressionTestDraftRecord{}, fmt.Errorf("profile root is required")
	}
	reviews, err := learning.ListQARegressionReviews(filepath.Join(a.profile.Root, "qa_review", "regression_reviews"))
	if err != nil {
		return learning.QARegressionTestDraftRecord{}, err
	}
	dir := filepath.Join(a.profile.Root, "qa_review", "regression_test_drafts")
	return learning.GenerateQARegressionTestDraft(dir, reviews, a.replayTraceStore(), a.latestQAEvalSummary(), input)
}

func (a *App) ApproveQARegressionTestDraft(input learning.QARegressionApprovalRequest) (learning.QARegressionApprovedRecord, error) {
	if err := a.ensure(); err != nil {
		return learning.QARegressionApprovedRecord{}, err
	}
	if a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return learning.QARegressionApprovedRecord{}, fmt.Errorf("profile root is required")
	}
	drafts, err := learning.ListQARegressionTestDrafts(filepath.Join(a.profile.Root, "qa_review", "regression_test_drafts"))
	if err != nil {
		return learning.QARegressionApprovedRecord{}, err
	}
	dir := filepath.Join(a.profile.Root, "qa_review", "approved_regressions")
	return learning.ApproveQARegressionTestDraft(dir, drafts, input)
}

func (a *App) GenerateQARegressionSourcePatch(input learning.QARegressionSourcePatchRequest) (learning.QARegressionSourcePatchRecord, error) {
	if err := a.ensure(); err != nil {
		return learning.QARegressionSourcePatchRecord{}, err
	}
	if a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return learning.QARegressionSourcePatchRecord{}, fmt.Errorf("profile root is required")
	}
	approvals, err := learning.ListQARegressionApprovedRecords(filepath.Join(a.profile.Root, "qa_review", "approved_regressions"))
	if err != nil {
		return learning.QARegressionSourcePatchRecord{}, err
	}
	dir := filepath.Join(a.profile.Root, "qa_review", "regression_source_patches")
	return learning.GenerateQARegressionSourcePatchDraft(dir, approvals, input)
}

func (a *App) ApproveQARegressionSourcePatchApplyPlan(input learning.QARegressionSourcePatchApplyPlanRequest) (learning.QARegressionSourcePatchApplyPlanRecord, error) {
	if err := a.ensure(); err != nil {
		return learning.QARegressionSourcePatchApplyPlanRecord{}, err
	}
	if a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return learning.QARegressionSourcePatchApplyPlanRecord{}, fmt.Errorf("profile root is required")
	}
	patches, err := learning.ListQARegressionSourcePatchDrafts(filepath.Join(a.profile.Root, "qa_review", "regression_source_patches"))
	if err != nil {
		return learning.QARegressionSourcePatchApplyPlanRecord{}, err
	}
	dir := filepath.Join(a.profile.Root, "qa_review", "regression_source_patch_apply_plans")
	return learning.GenerateQARegressionSourcePatchApplyPlan(dir, patches, input)
}

func (a *App) PlanQARegressionSourceWrite(input learning.QARegressionSourceWriteRequest) (QARegressionSourceWritePlanResult, error) {
	if err := a.ensure(); err != nil {
		return QARegressionSourceWritePlanResult{}, err
	}
	plans, err := a.qaRegressionSourceWriteApplyPlans()
	if err != nil {
		return QARegressionSourceWritePlanResult{}, err
	}
	planRecord, err := learning.ValidateQARegressionSourceWrite(plans, input)
	if err != nil {
		return QARegressionSourceWritePlanResult{}, err
	}
	filePlan, err := a.PlanFileWrite(FileWriteInput{Path: planRecord.SuggestedTestFile, Content: input.Content})
	if err != nil {
		return QARegressionSourceWritePlanResult{}, err
	}
	return QARegressionSourceWritePlanResult{
		TraceID:             planRecord.TraceID,
		ApplyPlanPath:       planRecord.Path,
		SuggestedTestFile:   planRecord.SuggestedTestFile,
		SuggestedTestName:   planRecord.SuggestedTestName,
		RequiresApproval:    true,
		FileWritePlanResult: filePlan,
	}, nil
}

func (a *App) ApplyQARegressionSourceWrite(input learning.QARegressionSourceWriteRequest) (QARegressionSourceWriteApplyResult, error) {
	if err := a.ensure(); err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	if !input.Approved {
		return QARegressionSourceWriteApplyResult{}, fmt.Errorf("source test write requires explicit approval")
	}
	plans, err := a.qaRegressionSourceWriteApplyPlans()
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	planRecord, err := learning.ValidateQARegressionSourceWrite(plans, input)
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	applied, err := a.ApplyFileWrite(FileWriteInput{Path: planRecord.SuggestedTestFile, Content: input.Content, Approved: true})
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	record, err := learning.RecordQARegressionSourceWrite(filepath.Join(a.profile.Root, "qa_review", "regression_source_writes"), planRecord, input, learning.QARegressionSourceWriteOutcome{
		SnapshotID:         applied.SnapshotID,
		VerificationStatus: applied.Verification.Status,
		Changed:            applied.Changed,
		TargetBytes:        applied.TargetBytes,
		Diff:               applied.Diff,
	})
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	return QARegressionSourceWriteApplyResult{
		TraceID:              planRecord.TraceID,
		Record:               record,
		FileWriteApplyResult: applied,
	}, nil
}

func (a *App) qaRegressionSourceWriteApplyPlans() ([]learning.QARegressionSourcePatchApplyPlanRecord, error) {
	if a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return nil, fmt.Errorf("profile root is required")
	}
	return learning.ListQARegressionSourcePatchApplyPlans(filepath.Join(a.profile.Root, "qa_review", "regression_source_patch_apply_plans"))
}

func (a *App) latestQAEvalSummary() *learning.QAEvalSummary {
	report, err := evaluation.LoadLatest(a.profile)
	if err != nil {
		return nil
	}
	tasks := make([]learning.QAEvalTaskSummary, 0, len(report.Tasks))
	for _, task := range report.Tasks {
		tasks = append(tasks, learning.QAEvalTaskSummary{
			Name:    task.Name,
			Status:  task.Status,
			Details: task.Details,
		})
	}
	status := evaluation.StatusPass
	if report.Summary.Failed > 0 {
		status = evaluation.StatusFail
	} else if report.Summary.Passed == 0 && report.Summary.Skipped > 0 {
		status = evaluation.StatusSkip
	}
	return &learning.QAEvalSummary{
		ID:          report.ID,
		Mode:        report.Mode,
		Model:       report.Model,
		StartedAt:   report.StartedAt,
		CompletedAt: report.CompletedAt,
		Passed:      report.Summary.Passed,
		Failed:      report.Summary.Failed,
		Skipped:     report.Summary.Skipped,
		Status:      status,
		Metrics:     report.Metrics,
		Tasks:       tasks,
	}
}

func (a *App) ExportTrajectory(conversationID string) (learning.Trajectory, error) {
	if err := a.ensure(); err != nil {
		return learning.Trajectory{}, err
	}
	return learning.ExportTrajectory(a.ctxOrBackground(), a.store, conversationID)
}

func (a *App) SaveCorrection(input LearningCorrectionInput) (MemoryResult, error) {
	if err := a.ensure(); err != nil {
		return MemoryResult{}, err
	}
	item, err := learning.SaveCorrection(a.ctxOrBackground(), a.store, input.ConversationID, input.Correction)
	if err != nil {
		return MemoryResult{}, err
	}
	return memoryItemResult(item), nil
}

func (a *App) Chat(content string, conversationID string) (ChatResult, error) {
	if err := a.ensure(); err != nil {
		return ChatResult{}, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return ChatResult{}, fmt.Errorf("message is required")
	}

	result := ChatResult{}
	service := &agent.Service{
		Router:           a.router,
		Runtime:          a.modelRuntime,
		RuntimeFactory:   a.runtimeFactory,
		Memory:           a.store,
		CloudFallback:    desktopCloudFallback(a.config, a.cloudRuntime),
		ToolExecutor:     desktopToolExecutor(a),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: a.config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(a.config, a.extensionStore()),
		ReplayStore:      a.replayStore(),
		ModelProfileDir:  a.modelProfileStore().Dir,
		PromptOptions:    agent.PromptOptionsFromConfig(a.config),
		PolicyMode:       safety.NormalizePolicyMode(a.config.Security.Policy.Mode),
		InternetTools:    agent.InternetToolLoopEnabled(a.config, safety.NormalizePolicyMode(a.config.Security.Policy.Mode)),
		InternetSearch:   agent.InternetSearchLoopEnabled(a.config, safety.NormalizePolicyMode(a.config.Security.Policy.Mode)),
	}
	runCtx, runID := a.beginRun()
	defer a.endRun(runID)
	err := service.Chat(runCtx, agent.ChatInput{Content: content, ConversationID: conversationID}, func(event agent.Event) error {
		a.emit("chat", event)
		switch event.Type {
		case agent.EventModelSelected:
			result.Model = event.Data["model"]
		case agent.EventModelToken:
			result.Text += event.Token
		case agent.EventModelToolCall:
			result.ToolCalls = appendEventToolCallNames(result.ToolCalls, event)
		case agent.EventMessageSaved:
			if event.Data["conversation_id"] != "" {
				result.ConversationID = event.Data["conversation_id"]
			}
		case agent.EventAgentCompleted:
			result.ConversationID = event.Data["conversation_id"]
		}
		return nil
	})
	return result, err
}

func (a *App) Ask(content string, skillName string, conversationID string, parentMessageID string, attachmentIDs []string) (AskResult, error) {
	if err := a.ensure(); err != nil {
		return AskResult{}, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return AskResult{}, fmt.Errorf("request is required")
	}

	selectedSkill, hasSkill, err := a.selectSkill(skillName, content)
	if err != nil {
		return AskResult{}, err
	}
	skillInstructions := ""
	if hasSkill {
		skillInstructions = skills.PromptBlock(selectedSkill)
	}

	result := AskResult{}
	if hasSkill {
		result.Skill = selectedSkill.Name
	}

	service := &agent.Service{
		Router:           a.router,
		Runtime:          a.modelRuntime,
		RuntimeFactory:   a.runtimeFactory,
		Memory:           a.store,
		CloudFallback:    desktopCloudFallback(a.config, a.cloudRuntime),
		ToolExecutor:     desktopToolExecutor(a),
		Retriever:        desktopRetriever(a),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: a.config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(a.config, a.extensionStore()),
		ReplayStore:      a.replayStore(),
		ModelProfileDir:  a.modelProfileStore().Dir,
		PromptOptions:    agent.PromptOptionsFromConfig(a.config),
		PolicyMode:       safety.NormalizePolicyMode(a.config.Security.Policy.Mode),
		InternetTools:    agent.InternetToolLoopEnabled(a.config, safety.NormalizePolicyMode(a.config.Security.Policy.Mode)),
		InternetSearch:   agent.InternetSearchLoopEnabled(a.config, safety.NormalizePolicyMode(a.config.Security.Policy.Mode)),
	}
	runCtx, runID := a.beginRun()
	defer a.endRun(runID)
	err = service.Ask(runCtx, agent.AskInput{
		ConversationID:     conversationID,
		ParentMessageID:    parentMessageID,
		Content:            content,
		AttachmentIDs:      attachmentIDs,
		SkillName:          selectedSkill.Name,
		SkillVersion:       selectedSkill.Version,
		SkillRequiredTools: selectedSkill.RequiredTools,
		SkillInstructions:  skillInstructions,
	}, func(event agent.Event) error {
		a.emit("ask", event)
		switch event.Type {
		case agent.EventModelSelected:
			result.Model = event.Data["model"]
		case agent.EventWorkspaceUsed:
			result.Sources = splitEventSources(event.Data["sources"])
			result.SourceKind = event.Data["source_kind"]
		case agent.EventToolCompleted:
			if sources := splitEventSources(event.Data["sources"]); len(sources) > 0 {
				result.Sources = sources
				if strings.TrimSpace(event.Data["source_kind"]) != "" {
					result.SourceKind = event.Data["source_kind"]
				}
			}
		case agent.EventModelToken:
			result.Text += event.Token
		case agent.EventModelToolCall:
			result.ToolCalls = appendEventToolCallNames(result.ToolCalls, event)
		case agent.EventMessageSaved:
			if event.Data["conversation_id"] != "" {
				result.ConversationID = event.Data["conversation_id"]
			}
			switch event.Data["role"] {
			case "user":
				result.UserMessageID = event.Data["message_id"]
			case "assistant":
				result.AssistantMessageID = event.Data["message_id"]
				result.ParentMessageID = event.Data["parent_message_id"]
				result.VariantIndex = atoiDefault(event.Data["variant_index"], 0)
				if event.Data["content"] != "" {
					result.Text = event.Data["content"]
				}
			}
		case agent.EventAgentCompleted:
			result.ConversationID = event.Data["conversation_id"]
		}
		return nil
	})
	if err == nil {
		result.Attachments = a.chatAttachmentsForMessage(a.ctxOrBackground(), result.UserMessageID)
	}
	return result, err
}

func (a *App) ListConversations(limit int) ([]ConversationResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	conversations, err := a.store.ListConversations(a.ctxOrBackground(), limit)
	if err != nil {
		return nil, err
	}
	return desktopConversationResults(conversations), nil
}

func (a *App) GetConversation(conversationID string) (ConversationDetailResult, error) {
	if err := a.ensure(); err != nil {
		return ConversationDetailResult{}, err
	}
	conversation, err := a.store.GetConversation(a.ctxOrBackground(), conversationID)
	if err != nil {
		return ConversationDetailResult{}, err
	}
	messages, err := a.store.ListConversationMessages(a.ctxOrBackground(), conversationID, 500)
	if err != nil {
		return ConversationDetailResult{}, err
	}
	messages = memory.WithInferredResponseParents(messages)
	activities, err := agent.ConversationMessageActivities(a.ctxOrBackground(), a.store, a.replayStore(), conversationID, messages)
	if err != nil {
		return ConversationDetailResult{}, err
	}
	turns, err := a.store.ListAgentTurnsForConversation(a.ctxOrBackground(), conversationID, 200)
	if err != nil {
		return ConversationDetailResult{}, err
	}
	attachmentsByMessage, err := a.chatAttachmentsForMessages(a.ctxOrBackground(), messages)
	if err != nil {
		return ConversationDetailResult{}, err
	}
	return ConversationDetailResult{
		Conversation: desktopConversationResult(conversation),
		Messages:     desktopConversationMessageResultsWithAttachments(messages, activities, turns, attachmentsByMessage),
	}, nil
}

func (a *App) RenameConversation(conversationID string, title string) (ConversationResult, error) {
	if err := a.ensure(); err != nil {
		return ConversationResult{}, err
	}
	conversation, err := a.store.UpdateConversationTitle(a.ctxOrBackground(), conversationID, title)
	if err != nil {
		return ConversationResult{}, err
	}
	return desktopConversationResult(conversation), nil
}

func (a *App) SetConversationStarred(conversationID string, starred bool) (ConversationResult, error) {
	if err := a.ensure(); err != nil {
		return ConversationResult{}, err
	}
	conversation, err := a.store.SetConversationStarred(a.ctxOrBackground(), conversationID, starred)
	if err != nil {
		return ConversationResult{}, err
	}
	return desktopConversationResult(conversation), nil
}

func (a *App) DeleteConversation(conversationID string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.store.DeleteConversation(a.ctxOrBackground(), conversationID)
}

func (a *App) EditUserMessage(messageID string, content string) (ConversationMessageResult, error) {
	if err := a.ensure(); err != nil {
		return ConversationMessageResult{}, err
	}
	message, err := a.store.CreateUserMessageVariant(a.ctxOrBackground(), messageID, content)
	if err != nil {
		return ConversationMessageResult{}, err
	}
	results := desktopConversationMessageResults([]memory.Message{message}, nil, nil)
	if len(results) == 0 {
		return ConversationMessageResult{}, nil
	}
	return results[0], nil
}

func (a *App) SearchMemory(query string) ([]MemoryResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	memories, err := a.store.SearchMemories(a.ctxOrBackground(), query, a.config.Memory.MaxRelevantMemories)
	if err != nil {
		return nil, err
	}
	messages, err := a.store.SearchMessages(a.ctxOrBackground(), query, a.config.Memory.MaxRelevantMemories)
	if err != nil {
		return nil, err
	}
	return combinedMemoryResults(memories, messages), nil
}

func (a *App) ListMemories(limit int) ([]MemoryResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	memories, err := a.store.ListMemories(a.ctxOrBackground(), limit)
	if err != nil {
		return nil, err
	}
	return explicitMemoryResults(memories), nil
}

func (a *App) WriteMemory(input MemoryWriteInput) (MemoryResult, error) {
	if err := a.ensure(); err != nil {
		return MemoryResult{}, err
	}
	item, err := a.store.SaveMemory(a.ctxOrBackground(), memory.Memory{
		Kind:       strings.TrimSpace(input.Kind),
		Content:    strings.TrimSpace(input.Content),
		Importance: input.Importance,
		Source:     "desktop",
	})
	if err != nil {
		return MemoryResult{}, err
	}
	return memoryItemResult(item), nil
}

func (a *App) KnowledgeStatus() (KnowledgeStatus, error) {
	if err := a.ensure(); err != nil {
		return KnowledgeStatus{}, err
	}
	return a.knowledgeStatus(a.ctxOrBackground())
}

func (a *App) SetKnowledgeEnabled(input KnowledgeEnabledInput) (KnowledgeStatus, error) {
	if err := a.ensure(); err != nil {
		return KnowledgeStatus{}, err
	}
	a.config.Knowledge.Enabled = input.Enabled
	a.config.Knowledge.InfluenceEnabled = input.Enabled && input.InfluenceEnabled
	a.config.Knowledge.ManualOnly = true
	config.ApplyDefaults(a.config)
	if err := config.Write(a.config.Path, a.config); err != nil {
		return KnowledgeStatus{}, err
	}
	return a.knowledgeStatus(a.ctxOrBackground())
}

func (a *App) ListKnowledgeEntities(query string, limit int) ([]KnowledgeEntityResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	store, err := a.openEnabledKnowledgeStore(a.ctxOrBackground())
	if err != nil {
		return nil, err
	}
	defer store.Close()
	entities, err := store.SearchEntities(a.ctxOrBackground(), query, knowledge.SearchOptions{Limit: limit})
	if err != nil {
		return nil, err
	}
	return knowledgeEntityResults(entities), nil
}

func (a *App) DraftKnowledgeProposal(input KnowledgeProposalInput) (KnowledgeProposalResult, error) {
	if err := a.ensure(); err != nil {
		return KnowledgeProposalResult{}, err
	}
	proposal, err := knowledge.DraftProposal(knowledge.ProposalInput{
		Text:        input.Text,
		Source:      input.Source,
		SourceKind:  input.SourceKind,
		SourceRef:   input.SourceRef,
		DefaultKind: input.DefaultKind,
		Limit:       input.Limit,
	})
	if err != nil {
		return KnowledgeProposalResult{}, err
	}
	return knowledgeProposalResult(proposal), nil
}

func (a *App) SaveKnowledgeEntity(input KnowledgeEntityInput) (KnowledgeEntityResult, error) {
	if err := a.ensure(); err != nil {
		return KnowledgeEntityResult{}, err
	}
	store, err := a.openEnabledKnowledgeStore(a.ctxOrBackground())
	if err != nil {
		return KnowledgeEntityResult{}, err
	}
	defer store.Close()
	entity, err := store.UpsertEntity(a.ctxOrBackground(), knowledge.EntityInput{
		Name:       input.Name,
		Kind:       input.Kind,
		Source:     input.Source,
		SourceKind: input.SourceKind,
		SourceRef:  input.SourceRef,
		Evidence:   input.Evidence,
	})
	if err != nil {
		return KnowledgeEntityResult{}, err
	}
	return knowledgeEntityResult(entity), nil
}

func (a *App) SaveKnowledgeEdge(input KnowledgeEdgeInput) (KnowledgeEdgeResult, error) {
	if err := a.ensure(); err != nil {
		return KnowledgeEdgeResult{}, err
	}
	store, err := a.openEnabledKnowledgeStore(a.ctxOrBackground())
	if err != nil {
		return KnowledgeEdgeResult{}, err
	}
	defer store.Close()
	edge, err := store.AddEdge(a.ctxOrBackground(), knowledge.EdgeInput{
		FromEntityID: input.FromEntityID,
		Relation:     input.Relation,
		ToEntityID:   input.ToEntityID,
		Source:       input.Source,
		SourceKind:   input.SourceKind,
		SourceRef:    input.SourceRef,
		Evidence:     input.Evidence,
	})
	if err != nil {
		return KnowledgeEdgeResult{}, err
	}
	return knowledgeEdgeResult(edge), nil
}

func (a *App) RepairKnowledgeEvidence() (KnowledgeRepairResult, error) {
	if err := a.ensure(); err != nil {
		return KnowledgeRepairResult{}, err
	}
	store, err := a.openEnabledKnowledgeStore(a.ctxOrBackground())
	if err != nil {
		return KnowledgeRepairResult{}, err
	}
	defer store.Close()
	result, err := store.RepairEvidence(a.ctxOrBackground())
	if err != nil {
		return KnowledgeRepairResult{}, err
	}
	status, err := a.knowledgeStatus(a.ctxOrBackground())
	if err != nil {
		return KnowledgeRepairResult{}, err
	}
	return knowledgeRepairResult(result, status), nil
}

func (a *App) ListKnowledgeReviewItems(status string, limit int) ([]KnowledgeReviewItemResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	store, err := a.openEnabledKnowledgeStore(a.ctxOrBackground())
	if err != nil {
		return nil, err
	}
	defer store.Close()
	items, err := store.ListReviewItems(a.ctxOrBackground(), knowledge.ReviewListOptions{Status: status, Limit: limit})
	if err != nil {
		return nil, err
	}
	return knowledgeReviewItemResults(items), nil
}

func (a *App) ApplyKnowledgeReviewBatch(input KnowledgeReviewBatchInput) (KnowledgeReviewBatchResult, error) {
	if err := a.ensure(); err != nil {
		return KnowledgeReviewBatchResult{}, err
	}
	store, err := a.openEnabledKnowledgeStore(a.ctxOrBackground())
	if err != nil {
		return KnowledgeReviewBatchResult{}, err
	}
	defer store.Close()
	targets := make([]knowledge.ReviewTarget, 0, len(input.Targets))
	for _, target := range input.Targets {
		targets = append(targets, knowledge.ReviewTarget{Type: target.Type, ID: target.ID})
	}
	result, err := store.ApplyReviewBatch(a.ctxOrBackground(), knowledge.ReviewBatchInput{
		Action:     input.Action,
		Targets:    targets,
		ReviewedBy: input.ReviewedBy,
		Note:       input.Note,
	})
	if err != nil {
		return KnowledgeReviewBatchResult{}, err
	}
	return knowledgeReviewBatchResult(result), nil
}

func (a *App) PinMemory(id string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.store.PinMemory(a.ctxOrBackground(), id, true)
}

func (a *App) DeleteMemory(id string) error {
	if err := a.ensure(); err != nil {
		return err
	}
	return a.store.DisableMemory(a.ctxOrBackground(), id)
}

func (a *App) IngestDocuments(path string) (IngestSummary, error) {
	if err := a.ensure(); err != nil {
		return IngestSummary{}, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = "."
	}
	root := a.workspaceRoot()
	effectivePath := a.workspaceRootPath(path)
	access, err := safety.RequireWorkspaceAccessWithBookmark(root, effectivePath, a.workspaceGrantStore())
	if err != nil {
		return IngestSummary{}, err
	}
	defer access.Close()
	result, err := a.rag.IngestPath(a.ctxOrBackground(), a.profile.Name, effectivePath, ragConfig(a.config.RAG), workspaceLimits(a.config.Workspace))
	if err != nil {
		return IngestSummary{}, err
	}
	return IngestSummary{
		Root:           result.Root,
		FilesIndexed:   result.FilesIndexed,
		FilesSkipped:   result.FilesSkipped,
		FilesUnchanged: result.FilesUnchanged,
		ChunksCreated:  result.ChunksCreated,
		BytesIndexed:   result.BytesIndexed,
		SkippedReasons: result.SkippedReasons,
	}, nil
}

func (a *App) UploadChatAttachment(input ChatAttachmentUploadInput) (ChatAttachmentUploadResult, error) {
	if err := a.ensure(); err != nil {
		return ChatAttachmentUploadResult{}, err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.DataBase64))
	if err != nil {
		return ChatAttachmentUploadResult{}, fmt.Errorf("decode attachment data: %w", err)
	}
	attachment, err := attachments.ProcessUpload(a.ctxOrBackground(), attachments.UploadInput{
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Data:        data,
		Retention:   input.Retention,
	}, workspaceLimits(a.config.Workspace))
	if err != nil {
		return ChatAttachmentUploadResult{}, err
	}
	saved, err := a.store.SaveChatAttachment(a.ctxOrBackground(), attachment)
	if err != nil {
		return ChatAttachmentUploadResult{}, err
	}
	return ChatAttachmentUploadResult{
		Attachment: desktopChatAttachmentResult(saved),
		Message:    "Attachment ready for this conversation.",
	}, nil
}

func (a *App) ListDocumentInventory(limit int) ([]DocumentInventoryResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	items, err := a.rag.ListDocuments(a.ctxOrBackground(), limit)
	if err != nil {
		return nil, err
	}
	return documentInventoryResults(items), nil
}

func (a *App) PruneMissingDocuments(confirm bool) (DocumentPruneResult, error) {
	if err := a.ensure(); err != nil {
		return DocumentPruneResult{}, err
	}
	result, err := a.rag.PruneMissingDocuments(a.ctxOrBackground(), !confirm)
	if err != nil {
		return DocumentPruneResult{}, err
	}
	return documentPruneResult(result), nil
}

func (a *App) SuggestDocumentPaths(path string, limit int) ([]DocumentPathSuggestionResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		path = "."
	}
	effectivePath := a.workspaceRootPath(path)
	accessPath, err := rag.SuggestDocumentAccessPath(effectivePath)
	if err != nil {
		return nil, err
	}
	access, err := safety.RequireWorkspaceAccessWithBookmark(a.workspaceRoot(), accessPath, a.workspaceGrantStore())
	if err != nil {
		return nil, err
	}
	defer access.Close()
	suggestions, err := rag.SuggestDocumentPaths(effectivePath, workspaceLimits(a.config.Workspace), limit)
	if err != nil {
		return nil, err
	}
	return documentPathSuggestions(suggestions), nil
}

func (a *App) IndexEmbeddings() (EmbeddingIndexSummary, error) {
	if err := a.ensure(); err != nil {
		return EmbeddingIndexSummary{}, err
	}
	cfg := ragConfig(a.config.RAG)
	if !cfg.Embeddings.Enabled {
		return EmbeddingIndexSummary{}, fmt.Errorf("embeddings are disabled; enable embeddings with an installed local model first")
	}
	result, err := a.rag.EnsureEmbeddings(a.ctxOrBackground(), cfg, embeddingRuntime(a.modelRuntime))
	if err != nil {
		return EmbeddingIndexSummary{}, err
	}
	return embeddingIndexSummary(result), nil
}

func documentPathSuggestions(suggestions []rag.PathSuggestion) []DocumentPathSuggestionResult {
	out := make([]DocumentPathSuggestionResult, 0, len(suggestions))
	for _, suggestion := range suggestions {
		out = append(out, DocumentPathSuggestionResult{
			Path:      suggestion.Path,
			Name:      suggestion.Name,
			Directory: suggestion.Directory,
			Kind:      suggestion.Kind,
			SizeBytes: suggestion.SizeBytes,
		})
	}
	return out
}

func (a *App) PickWorkspaceFolder(label string) (WorkspaceGrantResult, error) {
	if err := a.ensure(); err != nil {
		return WorkspaceGrantResult{}, err
	}
	selected, err := wailsruntime.OpenDirectoryDialog(a.ctxOrBackground(), wailsruntime.OpenDialogOptions{
		Title: "Grant Yemaka Workspace Folder",
	})
	if err != nil {
		return WorkspaceGrantResult{}, err
	}
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return WorkspaceGrantResult{}, fmt.Errorf("no folder selected")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = filepath.Base(selected)
	}
	grantOptions := safety.WorkspaceGrantOptions{Label: label, Source: "desktop_file_picker"}
	bookmark, bookmarkErr := safety.CreateSecurityScopedBookmark(selected)
	if bookmarkErr == nil {
		grantOptions.SecurityBookmark = bookmark.Data
		grantOptions.BookmarkCreatedAt = bookmark.CreatedAt
		grantOptions.BookmarkStale = bookmark.Stale
	}
	grant, err := a.workspaceGrantStore().GrantWithOptions(selected, grantOptions)
	if err != nil {
		_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
			ToolName:  "workspace_grant",
			Input:     map[string]any{"path": selected, "source": "desktop_file_picker"},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "low",
		})
		return WorkspaceGrantResult{}, err
	}
	output := map[string]any{"id": grant.ID, "bookmark_ready": grant.SecurityBookmark != "", "bookmark_supported": safety.SecurityScopedBookmarksSupported()}
	if bookmarkErr != nil {
		output["bookmark_error"] = bookmarkErr.Error()
	}
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ToolName:  "workspace_grant",
		Input:     map[string]any{"path": grant.Path, "label": grant.Label, "source": grant.Source},
		Output:    output,
		Status:    "completed",
		RiskLevel: "low",
	})
	return workspaceGrantResult(grant), nil
}

func (a *App) PickDocumentFile(label string) (DocumentSourceResult, error) {
	if err := a.ensure(); err != nil {
		return DocumentSourceResult{}, err
	}
	selected, err := wailsruntime.OpenFileDialog(a.ctxOrBackground(), wailsruntime.OpenDialogOptions{
		Title: "Select Document To Ingest",
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Supported documents",
				Pattern:     "*.md;*.markdown;*.txt;*.pdf;*.docx;*.json;*.yaml;*.yml;*.toml;*.html;*.css;*.go;*.py;*.js;*.ts;*.svelte;*.vue;*.java;*.rb;*.rs;*.cpp;*.c;*.h",
			},
			{
				DisplayName: "All files",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return DocumentSourceResult{}, err
	}
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return DocumentSourceResult{}, fmt.Errorf("no file selected")
	}
	label = strings.TrimSpace(label)
	if label == "" {
		label = filepath.Base(selected)
	}
	grantRoot := filepath.Dir(selected)
	grantOptions := safety.WorkspaceGrantOptions{Label: label, Source: "desktop_file_picker"}
	bookmark, bookmarkErr := safety.CreateSecurityScopedBookmark(grantRoot)
	if bookmarkErr == nil {
		grantOptions.SecurityBookmark = bookmark.Data
		grantOptions.BookmarkCreatedAt = bookmark.CreatedAt
		grantOptions.BookmarkStale = bookmark.Stale
	}
	grant, err := a.workspaceGrantStore().GrantWithOptions(grantRoot, grantOptions)
	if err != nil {
		_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
			ToolName:  "workspace_grant",
			Input:     map[string]any{"path": selected, "grant_root": grantRoot, "source": "desktop_file_picker"},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "low",
		})
		return DocumentSourceResult{}, err
	}
	output := map[string]any{"id": grant.ID, "document": selected, "bookmark_ready": grant.SecurityBookmark != "", "bookmark_supported": safety.SecurityScopedBookmarksSupported()}
	if bookmarkErr != nil {
		output["bookmark_error"] = bookmarkErr.Error()
	}
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ToolName:  "workspace_grant",
		Input:     map[string]any{"path": grant.Path, "document": selected, "label": grant.Label, "source": grant.Source},
		Output:    output,
		Status:    "completed",
		RiskLevel: "low",
	})
	return DocumentSourceResult{Path: selected, Grant: workspaceGrantResult(grant)}, nil
}

func (a *App) GrantWorkspace(path string, label string) (WorkspaceGrantResult, error) {
	if err := a.ensure(); err != nil {
		return WorkspaceGrantResult{}, err
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return WorkspaceGrantResult{}, fmt.Errorf("workspace path is required")
	}
	path = a.workspaceRootPath(path)
	label = strings.TrimSpace(label)
	if label == "" {
		label = filepath.Base(path)
	}
	grant, err := a.workspaceGrantStore().Grant(path, label, "desktop_manual")
	if err != nil {
		_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
			ToolName:  "workspace_grant",
			Input:     map[string]any{"path": path, "source": "desktop_manual"},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "low",
		})
		return WorkspaceGrantResult{}, err
	}
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ToolName:  "workspace_grant",
		Input:     map[string]any{"path": grant.Path, "label": grant.Label, "source": grant.Source},
		Output:    map[string]any{"id": grant.ID},
		Status:    "completed",
		RiskLevel: "low",
	})
	return workspaceGrantResult(grant), nil
}

func (a *App) ListWorkspaceGrants() ([]WorkspaceGrantResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	grants, err := a.workspaceGrantStore().List()
	if err != nil {
		return nil, err
	}
	out := make([]WorkspaceGrantResult, 0, len(grants))
	for _, grant := range grants {
		out = append(out, workspaceGrantResult(grant))
	}
	return out, nil
}

func (a *App) RevokeWorkspaceGrant(match string) (WorkspaceGrantResult, error) {
	if err := a.ensure(); err != nil {
		return WorkspaceGrantResult{}, err
	}
	grant, ok, err := a.workspaceGrantStore().Revoke(match)
	if err != nil {
		_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
			ToolName:  "workspace_revoke",
			Input:     map[string]any{"match": match},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "low",
		})
		return WorkspaceGrantResult{}, err
	}
	if !ok {
		return WorkspaceGrantResult{}, fmt.Errorf("workspace grant not found: %s", strings.TrimSpace(match))
	}
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ToolName:  "workspace_revoke",
		Input:     map[string]any{"match": match},
		Output:    map[string]any{"id": grant.ID, "path": grant.Path},
		Status:    "completed",
		RiskLevel: "low",
	})
	return workspaceGrantResult(grant), nil
}

func (a *App) SearchDocuments(query string) ([]DocumentResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	query = strings.TrimSpace(query)
	var results []rag.SearchResult
	var err error
	if query == "" {
		results, err = a.rag.ListChunks(a.ctxOrBackground(), a.config.RAG.TopK)
	} else {
		results, err = a.rag.SearchWithConfig(a.ctxOrBackground(), query, ragConfig(a.config.RAG), embeddingRuntime(a.modelRuntime))
	}
	if err != nil {
		return nil, err
	}
	out := make([]DocumentResult, 0, len(results))
	for _, result := range results {
		out = append(out, DocumentResult{
			ChunkID:     result.ChunkID,
			Path:        result.Path,
			Content:     oneLine(result.Content),
			Rank:        result.Rank,
			Score:       result.Score,
			Source:      result.Source,
			Explanation: result.Explanation,
		})
	}
	return out, nil
}

func (a *App) ListToolRuns(limit int) ([]ToolRunResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	runs, err := a.store.ListToolRuns(a.ctxOrBackground(), limit)
	if err != nil {
		return nil, err
	}
	return toolRunResults(runs), nil
}

func (a *App) ToolCatalog(surface string) ([]ToolCatalogEntryResult, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return toolCatalogResults(tools.ToolCatalogForSurface(toolSurfaceFromString(surface))), nil
}

func (a *App) ListNotifications(limit int, includeDismissed bool) ([]notifications.Notification, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	return a.notificationStore().List(limit, includeDismissed)
}

func (a *App) MarkNotificationRead(id string) (notifications.Notification, error) {
	if err := a.ensure(); err != nil {
		return notifications.Notification{}, err
	}
	return a.notificationStore().MarkRead(id)
}

func (a *App) DismissNotification(id string) (notifications.Notification, error) {
	if err := a.ensure(); err != nil {
		return notifications.Notification{}, err
	}
	return a.notificationStore().Dismiss(id)
}

func (a *App) ListReplayTraces(limit int) ([]replay.TraceFile, error) {
	if err := a.ensure(); err != nil {
		return nil, err
	}
	traces, err := a.replayTraceStore().List()
	if err != nil {
		return nil, err
	}
	return limitLatestReplayTraces(traces, limit), nil
}

func (a *App) ExplainReplayTrace(id string) (replay.TraceExplanation, error) {
	if err := a.ensure(); err != nil {
		return replay.TraceExplanation{}, err
	}
	trace, err := a.replayTraceStore().Load(id)
	if err != nil {
		return replay.TraceExplanation{}, err
	}
	return replay.ExplainTrace(trace), nil
}

func (a *App) RecordPermissionDecision(input PermissionDecisionInput) (PermissionDecisionResult, error) {
	if err := a.ensure(); err != nil {
		return PermissionDecisionResult{}, err
	}
	decision, err := normalizePermissionDecision(input.Decision)
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	if strings.TrimSpace(input.Request.ToolName) == "" {
		return PermissionDecisionResult{}, fmt.Errorf("permission tool name is required")
	}
	if decision == "approved" {
		if err := agent.ValidateStoredPermissionApproval(a.ctxOrBackground(), a.store, input.Request); err != nil {
			return PermissionDecisionResult{}, err
		}
	}
	requestRun, _, err := a.permissionRequestRun(input.Request.RequestID)
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	_, err = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ConversationID:     requestRun.ConversationID,
		SessionID:          requestRun.SessionID,
		UserMessageID:      requestRun.UserMessageID,
		AssistantMessageID: requestRun.AssistantMessageID,
		ParentMessageID:    requestRun.ParentMessageID,
		VariantIndex:       requestRun.VariantIndex,
		ToolName:           "permission_decision",
		Input:              input.Request,
		Output:             map[string]any{"decision": decision, "note": strings.TrimSpace(input.Note)},
		Status:             decision,
		RiskLevel:          input.Request.RiskLevel,
	})
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	result := PermissionDecisionResult{Status: decision}
	if decision != "approved" {
		result.Message = fmt.Sprintf("Permission rejected. I will not run `%s`.", input.Request.ToolName)
		result, err = a.savePermissionDecisionMessage(input.Request, result, requestRun)
		if err != nil {
			return PermissionDecisionResult{}, err
		}
		if err := agent.ClearPendingOperationState(a.ctxOrBackground(), a.store, requestRun.ConversationID, input.Request.RequestID); err != nil {
			return PermissionDecisionResult{}, err
		}
		return result, nil
	}
	if editResult, handled, err := a.applyApprovedEditPermission(input.Request); err != nil {
		return PermissionDecisionResult{}, err
	} else if handled {
		editResult, err = a.savePermissionDecisionMessage(input.Request, editResult, requestRun)
		if err != nil {
			return PermissionDecisionResult{}, err
		}
		if err := agent.ClearPendingOperationState(a.ctxOrBackground(), a.store, requestRun.ConversationID, input.Request.RequestID); err != nil {
			return PermissionDecisionResult{}, err
		}
		return editResult, nil
	}
	outcome := agent.ResolveApprovedPermission(a.ctxOrBackground(), input.Request, desktopToolExecutor(a))
	result.ToolName = outcome.Decision.ToolName
	result.ToolStatus = outcome.ToolStatus
	result.Executed = outcome.Executed
	result.Message = outcome.Message
	result.Result = outcome.Result
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ConversationID:     requestRun.ConversationID,
		SessionID:          requestRun.SessionID,
		UserMessageID:      requestRun.UserMessageID,
		AssistantMessageID: requestRun.AssistantMessageID,
		ParentMessageID:    requestRun.ParentMessageID,
		VariantIndex:       requestRun.VariantIndex,
		ToolName:           agent.PermissionOutcomeToolName(outcome),
		Input:              outcome.Decision,
		Output:             agent.PermissionOutcomeOutput(outcome),
		Status:             agent.PermissionOutcomeStatus(outcome),
		RiskLevel:          input.Request.RiskLevel,
	})
	result, err = a.savePermissionDecisionMessage(input.Request, result, requestRun)
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	if err := agent.ClearPendingOperationState(a.ctxOrBackground(), a.store, requestRun.ConversationID, input.Request.RequestID); err != nil {
		return PermissionDecisionResult{}, err
	}
	return result, nil
}

func (a *App) permissionRequestRun(requestID string) (memory.ToolRun, bool, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || a.store == nil {
		return memory.ToolRun{}, false, nil
	}
	runs, err := a.store.ListToolRuns(a.ctxOrBackground(), 200)
	if err != nil {
		return memory.ToolRun{}, false, fmt.Errorf("load permission request run: %w", err)
	}
	for _, run := range runs {
		if strings.ToLower(strings.TrimSpace(run.ToolName)) != "permission_request" {
			continue
		}
		if desktopToolRunRequestID(run.Input) == requestID || desktopToolRunRequestID(run.Output) == requestID {
			return run, true, nil
		}
	}
	return memory.ToolRun{}, false, nil
}

func desktopToolRunRequestID(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"request_id", "requestId", "RequestID"} {
			if value, ok := typed[key]; ok {
				return strings.TrimSpace(fmt.Sprint(value))
			}
		}
	case map[string]string:
		for _, key := range []string{"request_id", "requestId", "RequestID"} {
			if value, ok := typed[key]; ok {
				return strings.TrimSpace(value)
			}
		}
	case agent.PermissionRequest:
		return strings.TrimSpace(typed.RequestID)
	}
	return ""
}

func (a *App) savePermissionDecisionMessage(request agent.PermissionRequest, result PermissionDecisionResult, requestRun memory.ToolRun) (PermissionDecisionResult, error) {
	if a.store == nil || strings.TrimSpace(result.Message) == "" || strings.TrimSpace(requestRun.ConversationID) == "" {
		return result, nil
	}
	parentID := strings.TrimSpace(requestRun.AssistantMessageID)
	if parentID == "" {
		parentID = strings.TrimSpace(requestRun.ParentMessageID)
	}
	if parentID == "" {
		parentID = strings.TrimSpace(requestRun.UserMessageID)
	}
	model := result.ToolName
	if strings.TrimSpace(model) == "" {
		model = request.ToolName
	}
	if strings.TrimSpace(model) == "" {
		model = "yemaka-executor"
	}
	msg, err := a.store.SaveMessage(a.ctxOrBackground(), memory.Message{
		ConversationID: requestRun.ConversationID,
		Role:           "assistant",
		Content:        strings.TrimSpace(result.Message),
		Model:          model,
		ParentID:       parentID,
	})
	if err != nil {
		return result, fmt.Errorf("save permission decision message: %w", err)
	}
	result.ConversationID = msg.ConversationID
	result.AssistantMessageID = msg.ID
	result.ParentMessageID = msg.ParentID
	result.VariantIndex = msg.VariantIndex
	result.ActiveVariant = msg.ActiveVariant
	result.CreatedAt = msg.CreatedAt
	output := map[string]any{
		"message":     result.Message,
		"tool_name":   result.ToolName,
		"tool_status": result.ToolStatus,
		"executed":    result.Executed,
		"status":      result.Status,
	}
	if result.Result != nil {
		output["context"] = result.Result.Context
		output["source_kind"] = result.Result.SourceKind
		output["sources"] = result.Result.Sources
	}
	status := result.ToolStatus
	if strings.TrimSpace(status) == "" {
		status = result.Status
	}
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ConversationID:     requestRun.ConversationID,
		SessionID:          requestRun.SessionID,
		UserMessageID:      requestRun.UserMessageID,
		AssistantMessageID: msg.ID,
		ParentMessageID:    msg.ParentID,
		VariantIndex:       msg.VariantIndex,
		ToolName:           "permission_result",
		Input:              map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:             output,
		Status:             status,
		RiskLevel:          request.RiskLevel,
	})
	return result, nil
}

func (a *App) applyApprovedEditPermission(request agent.PermissionRequest) (PermissionDecisionResult, bool, error) {
	proposal, ok, err := agent.StoredReadyEditProposalForPermission(a.ctxOrBackground(), a.store, request)
	if err != nil || !ok {
		return PermissionDecisionResult{}, ok, err
	}
	applied, err := a.ApplyFileWrite(FileWriteInput{Path: proposal.Path, Content: proposal.Content, Approved: true})
	if err != nil {
		return failedEditPermissionResult(request, err), true, nil
	}
	return appliedEditPermissionResult(request, applied), true, nil
}

func failedEditPermissionResult(request agent.PermissionRequest, err error) PermissionDecisionResult {
	tool := normalizeToolNameForPermissionResult(request.ToolName)
	return PermissionDecisionResult{
		Status:     "approved",
		ToolName:   tool,
		ToolStatus: "failed",
		Executed:   false,
		Message:    fmt.Sprintf("Approval received, but `%s` could not run: %s", tool, strings.TrimSpace(err.Error())),
		Result: &agent.ExecutionResult{
			Context:    strings.TrimSpace(err.Error()),
			SourceKind: "tool",
			Status:     "failed",
		},
	}
}

func appliedEditPermissionResult(request agent.PermissionRequest, applied FileWriteApplyResult) PermissionDecisionResult {
	tool := normalizeToolNameForPermissionResult(request.ToolName)
	status := "completed"
	if strings.TrimSpace(applied.Verification.Status) != "" && applied.Verification.Status != "pass" {
		status = applied.Verification.Status
	}
	message := fmt.Sprintf("Approval received. I applied the approved edit to `%s`.", applied.Path)
	if !applied.Changed {
		message = fmt.Sprintf("Approval received. `%s` already matched the approved edit when I tried to apply it, so I did not write the file or create a new snapshot.", applied.Path)
	} else if strings.TrimSpace(applied.SnapshotID) != "" {
		message += fmt.Sprintf(" Snapshot: `%s`.", applied.SnapshotID)
	}
	if strings.TrimSpace(applied.Verification.Status) != "" {
		message += fmt.Sprintf(" Verification: %s.", applied.Verification.Status)
	}
	context := fmt.Sprintf("FILE WRITE APPLIED\npath: %s\nsnapshot_id: %s\nverification: %s\nchanged: %t\n\n%s", applied.Path, applied.SnapshotID, applied.Verification.Status, applied.Changed, strings.TrimSpace(applied.Diff))
	return PermissionDecisionResult{
		Status:     "approved",
		ToolName:   tool,
		ToolStatus: status,
		Executed:   true,
		Message:    strings.TrimSpace(message),
		Result: &agent.ExecutionResult{
			Context:    strings.TrimSpace(context),
			Sources:    []string{applied.Path},
			SourceKind: "tool",
			Status:     status,
		},
	}
}

func normalizeToolNameForPermissionResult(tool string) string {
	tool = strings.TrimSpace(tool)
	if tool == "" {
		return "edit_file"
	}
	return tool
}

func (a *App) PlanFileWrite(input FileWriteInput) (FileWritePlanResult, error) {
	if err := a.ensure(); err != nil {
		return FileWritePlanResult{}, err
	}
	if err := validateFileWriteInput(input, a.config); err != nil {
		return FileWritePlanResult{}, err
	}
	root := a.workspaceRoot()
	plan, err := planFileWrite(a.ctxOrBackground(), root, input, fileWriteOptions(a.config, a.profile))
	if err != nil {
		return FileWritePlanResult{}, err
	}
	return fileWritePlanResult(plan), nil
}

func (a *App) ApplyFileWrite(input FileWriteInput) (FileWriteApplyResult, error) {
	if err := a.ensure(); err != nil {
		return FileWriteApplyResult{}, err
	}
	if !input.Approved {
		return FileWriteApplyResult{}, fmt.Errorf("approved flag is required before applying file writes")
	}
	if err := validateFileWriteInput(input, a.config); err != nil {
		return FileWriteApplyResult{}, err
	}
	options := fileWriteOptions(a.config, a.profile)
	root := a.workspaceRoot()
	plan, err := planFileWrite(a.ctxOrBackground(), root, input, options)
	if err != nil {
		_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
			ToolName:  "write_file",
			Input:     map[string]any{"path": input.Path},
			Output:    map[string]any{"error": err.Error()},
			Status:    "blocked",
			RiskLevel: "medium",
		})
		return FileWriteApplyResult{}, err
	}
	applied, err := workspace.ApplyWrite(a.ctxOrBackground(), root, plan, options)
	if err != nil {
		_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
			ToolName:  "write_file",
			Input:     map[string]any{"path": plan.Path, "bytes": plan.TargetBytes},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "medium",
		})
		return FileWriteApplyResult{}, err
	}
	verification := workspace.VerifyAppliedWrite(a.ctxOrBackground(), root, applied, options)
	writeStatus := "completed"
	if verification.Status != "pass" {
		writeStatus = verification.Status
	}
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ToolName:  "write_file",
		Input:     map[string]any{"path": applied.Path, "bytes": applied.TargetBytes},
		Output:    map[string]any{"snapshot_id": applied.SnapshotID, "changed": applied.Changed, "verification_status": verification.Status},
		Status:    writeStatus,
		RiskLevel: "medium",
	})
	_, _ = a.store.SaveToolRun(a.ctxOrBackground(), memory.ToolRun{
		ToolName:  "write_verifier",
		Input:     map[string]any{"path": applied.Path, "snapshot_id": applied.SnapshotID},
		Output:    verification,
		Status:    verification.Status,
		RiskLevel: "medium",
	})
	return fileWriteApplyResult(applied, verification), nil
}

func (a *App) StopGeneration() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeCancel != nil {
		a.activeCancel()
		a.activeCancel = nil
	}
	return nil
}

func (a *App) ensure() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.config != nil && a.store != nil && a.rag != nil {
		return nil
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		return err
	}
	profile, err := profiles.Init(cfg)
	if err != nil {
		return err
	}
	store, err := memory.Open(a.ctxOrBackground(), profile.Database)
	if err != nil {
		return err
	}
	ragStore, err := rag.Open(a.ctxOrBackground(), profile.Database)
	if err != nil {
		_ = store.Close()
		return err
	}
	skillRegistry, err := loadSkillRegistryForProfile(profile)
	if err != nil {
		_ = store.Close()
		_ = ragStore.Close()
		return err
	}

	a.config = cfg
	a.profile = profile
	a.store = store
	a.rag = ragStore
	a.skills = skillRegistry
	a.router = models.NewRouter(cfg)
	runtime, err := modelruntime.New(a.selectedModel())
	if err != nil {
		_ = store.Close()
		_ = ragStore.Close()
		return err
	}
	a.modelRuntime = runtime
	if cfg.CloudFallback.Enabled {
		a.cloudRuntime = cloud.New(cfg.CloudFallback)
	}
	return nil
}

func (a *App) selectedModel() config.ModelConfig {
	selected := a.config.Models["default"]
	if a.config.Runtime.LowMemoryMode {
		if low, ok := a.config.Models["low_memory"]; ok {
			selected = low
		}
	}
	return selected
}

func modelTemperature(cfg *config.Config, name string) float64 {
	if cfg == nil {
		return 0.2
	}
	name = strings.TrimSpace(name)
	for _, role := range []string{"default", "low_memory", "coding", "reasoning", "stronger_local"} {
		model := cfg.Models[role]
		if strings.EqualFold(strings.TrimSpace(model.Name), name) && model.Temperature > 0 {
			return model.Temperature
		}
	}
	if model := cfg.Models["default"]; model.Temperature > 0 {
		return model.Temperature
	}
	return 0.2
}

func settingsView(cfg *config.Config) SettingsView {
	return SettingsView{
		SettingsSchemaVersion: 2,
		FeatureSupport: SettingsFeatureSupport{
			ResponseMode:      true,
			ShowThinkingTrace: true,
		},
		LowMemoryMode:     cfg.Runtime.LowMemoryMode,
		MaxContextTokens:  cfg.Runtime.MaxContextTokens,
		ResponseMode:      config.NormalizeResponseMode(cfg.Runtime.ResponseMode),
		ShowThinkingTrace: cfg.Runtime.ShowThinkingTrace,
		UI:                cfg.UI,
		ModelRoles:        cfg.Models,
		CloudFallback:     cfg.CloudFallback,
		Connectors:        cfg.Connectors,
		Internet:          cfg.Internet,
		Scheduler:         cfg.Scheduler,
		Heartbeat:         cfg.Heartbeat,
		RAG:               cfg.RAG,
		Knowledge:         cfg.Knowledge,
		Workspace:         cfg.Workspace,
		ShellEnabled:      cfg.Tools.Shell.Enabled,
		Telemetry:         cfg.App.Telemetry,
	}
}

func policyStatusView(cfg *config.Config) PolicyStatusResult {
	if cfg == nil {
		return PolicyStatusResult{Mode: safety.PolicyModeSafe}
	}
	mode := safety.NormalizePolicyMode(cfg.Security.Policy.Mode)
	return PolicyStatusResult{
		Mode:                         mode,
		AuditEnabled:                 cfg.Security.Policy.AuditEnabled,
		MaxAutonomousLevel:           cfg.Security.Policy.MaxAutonomousLevel,
		AllowAutonomousPosting:       cfg.Security.Policy.AllowAutonomousPosting,
		AllowAutonomousDeployment:    cfg.Security.Policy.AllowAutonomousDeployment,
		AllowLiveTrading:             cfg.Security.Policy.AllowLiveTrading,
		GeneratedCanModifyCorePolicy: cfg.Security.Policy.GeneratedCanModifyCorePolicy,
		FullAccess:                   safety.IsFullAccessMode(mode),
	}
}

func applyPolicyMode(cfg *config.Config, mode string) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != safety.PolicyModeSafe && mode != safety.PolicyModeFullAccess {
		return fmt.Errorf("unknown policy mode %q; use safe or full_access", mode)
	}
	cfg.Security.Policy.Mode = safety.NormalizePolicyMode(mode)
	if safety.IsFullAccessMode(cfg.Security.Policy.Mode) {
		cfg.Tools.Shell.RequireConfirmationRisky = false
	}
	return nil
}

func applySettings(cfg *config.Config, input SettingsInput) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	if input.MaxContextTokens < 1024 || input.MaxContextTokens > 16384 {
		return fmt.Errorf("max context tokens must be between 1024 and 16384")
	}
	if input.LowMemoryMode && input.MaxContextTokens > 8192 {
		return fmt.Errorf("low-memory mode max context tokens cannot exceed 8192")
	}

	cfg.UI.Theme = config.NormalizeUITheme(input.UITheme)
	cfg.Runtime.LowMemoryMode = input.LowMemoryMode
	cfg.Runtime.MaxContextTokens = input.MaxContextTokens
	cfg.Runtime.ResponseMode = config.NormalizeResponseMode(input.ResponseMode)
	cfg.Runtime.ShowThinkingTrace = input.ShowThinkingTrace
	cfg.RAG.Enabled = input.RAGEnabled
	cfg.Knowledge.InfluenceEnabled = input.KnowledgeInfluenceEnabled && cfg.Knowledge.Enabled
	cfg.Knowledge.ManualOnly = true
	cfg.Internet.Enabled = input.InternetEnabled
	if input.InternetEnabled {
		cfg.Internet.DefaultMode = "profile_enabled"
	} else if strings.TrimSpace(cfg.Internet.DefaultMode) == "" || cfg.Internet.DefaultMode == "profile_enabled" {
		cfg.Internet.DefaultMode = "ask_each_time"
	}
	cfg.Internet.Search.Enabled = input.InternetSearchEnabled
	cfg.Internet.Search.Provider = strings.TrimSpace(input.InternetSearchProvider)
	if cfg.Internet.Search.Provider == "" {
		cfg.Internet.Search.Provider = "none"
	}
	cfg.Internet.Search.Endpoint = strings.TrimSpace(input.InternetSearchEndpoint)
	apiKeyEnv := internet.NormalizeSearchAPIKeyEnvForProvider(cfg.Internet.Search.Provider, input.InternetSearchAPIKeyEnv)
	if err := internet.ValidateSearchAPIKeyEnvForProvider(cfg.Internet.Search.Provider, apiKeyEnv); err != nil {
		return err
	}
	cfg.Internet.Search.APIKeyEnv = apiKeyEnv

	if input.EmbeddingsEnabled {
		model := strings.TrimSpace(input.EmbeddingModel)
		if model == "" {
			return fmt.Errorf("embedding model is required before enabling embeddings")
		}
		cfg.RAG.Embeddings.Enabled = true
		cfg.RAG.Embeddings.Provider = "ollama"
		cfg.RAG.Embeddings.Model = model
	} else {
		cfg.RAG.Embeddings.Enabled = false
		if strings.TrimSpace(input.EmbeddingModel) != "" {
			cfg.RAG.Embeddings.Model = strings.TrimSpace(input.EmbeddingModel)
		}
	}

	if input.CloudFallbackEnabled {
		baseURL := strings.TrimSpace(input.CloudFallbackBaseURL)
		model := strings.TrimSpace(input.CloudFallbackModel)
		keyEnv := strings.TrimSpace(input.CloudFallbackKeyEnv)
		if baseURL == "" || model == "" {
			return fmt.Errorf("cloud fallback base URL and model are required before enabling cloud fallback")
		}
		if looksLikeSecretValue(keyEnv) {
			return fmt.Errorf("cloud fallback key must be an environment variable name, not a secret value")
		}
		cfg.CloudFallback.Enabled = true
		cfg.CloudFallback.Provider = "openai_compatible"
		cfg.CloudFallback.BaseURL = baseURL
		cfg.CloudFallback.Name = model
		cfg.CloudFallback.APIKeyEnv = keyEnv
		if cfg.CloudFallback.TimeoutSeconds == 0 {
			cfg.CloudFallback.TimeoutSeconds = 45
		}
		if cfg.CloudFallback.Temperature == 0 {
			cfg.CloudFallback.Temperature = 0.2
		}
	} else {
		cfg.CloudFallback.Enabled = false
		if strings.TrimSpace(input.CloudFallbackBaseURL) != "" {
			cfg.CloudFallback.BaseURL = strings.TrimSpace(input.CloudFallbackBaseURL)
		}
		if strings.TrimSpace(input.CloudFallbackModel) != "" {
			cfg.CloudFallback.Name = strings.TrimSpace(input.CloudFallbackModel)
		}
		if strings.TrimSpace(input.CloudFallbackKeyEnv) != "" {
			if looksLikeSecretValue(input.CloudFallbackKeyEnv) {
				return fmt.Errorf("cloud fallback key must be an environment variable name, not a secret value")
			}
			cfg.CloudFallback.APIKeyEnv = strings.TrimSpace(input.CloudFallbackKeyEnv)
		}
	}

	if input.ConnectorEnabled {
		if err := connectors.EnableLocalAPI(cfg, strings.TrimSpace(input.ConnectorTokenEnv)); err != nil {
			return err
		}
	} else if err := connectors.DisableLocalAPI(cfg); err != nil {
		return err
	}
	if input.MCPConnectorEnabled {
		if err := connectors.EnableMCPServer(cfg); err != nil {
			return err
		}
	} else if err := connectors.DisableMCPServer(cfg); err != nil {
		return err
	}
	if err := applyAdapterSetting(cfg, connectors.Slack, input.SlackConnectorEnabled, input.SlackConnectorTokenEnv); err != nil {
		return err
	}
	if err := applyAdapterSetting(cfg, connectors.Discord, input.DiscordConnectorEnabled, input.DiscordConnectorTokenEnv); err != nil {
		return err
	}
	if err := applyAdapterSetting(cfg, connectors.Telegram, input.TelegramConnectorEnabled, input.TelegramConnectorTokenEnv); err != nil {
		return err
	}
	if err := applyAdapterSetting(cfg, connectors.Email, input.EmailConnectorEnabled, input.EmailConnectorTokenEnv); err != nil {
		return err
	}
	return nil
}

func applyAdapterSetting(cfg *config.Config, name string, enabled bool, tokenEnv string) error {
	if enabled {
		return connectors.EnableAdapter(cfg, name, strings.TrimSpace(tokenEnv))
	}
	return connectors.DisableAdapter(cfg, name)
}

func looksLikeSecretValue(value string) bool {
	value = strings.TrimSpace(value)
	lower := strings.ToLower(value)
	return strings.HasPrefix(lower, "sk-") ||
		strings.HasPrefix(lower, "xox") ||
		strings.Contains(value, " ")
}

func (a *App) reloadSkills() error {
	registry, err := loadSkillRegistryForProfile(a.profile)
	if err != nil {
		return err
	}
	a.skills = registry
	return nil
}

func (a *App) domainPackRoot() string {
	return filepath.Join(a.profile.Root, "domain_packs")
}

func loadSkillRegistryForProfile(profile *profiles.Profile) (skills.Registry, error) {
	if profile == nil {
		return skills.Registry{}, fmt.Errorf("profile is required")
	}
	defaultSkillsDir, err := skills.DefaultDir()
	if err != nil {
		return skills.Registry{}, err
	}
	enabledPackSkillDirs, err := domainpacks.EnabledSkillDirs(filepath.Join(profile.Root, "domain_packs"))
	if err != nil {
		return skills.Registry{}, err
	}
	return skills.LoadProfileRegistry(defaultSkillsDir, profile.Skills, enabledPackSkillDirs)
}

func (a *App) extensionStore() *extensions.Store {
	store := extensions.NewStore(a.profile.GeneratedExtensions, a.profile.Logs)
	store.PolicyMode = safety.NormalizePolicyMode(a.config.Security.Policy.Mode)
	return store
}

func (a *App) generatedConnectorStore() *connectors.GeneratedStore {
	root := filepath.Join(a.profile.Root, "connectors", "generated")
	store := connectors.NewGeneratedStore(root)
	store.PolicyMode = safety.NormalizePolicyMode(a.config.Security.Policy.Mode)
	return store
}

func (a *App) replayStore() agent.ReplayReadWriter {
	if a == nil || a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return nil
	}
	return replay.NewStore(filepath.Join(a.profile.Root, "replay"))
}

func (a *App) replayTraceStore() replay.Store {
	if a == nil || a.profile == nil || strings.TrimSpace(a.profile.Root) == "" {
		return replay.Store{}
	}
	return replay.NewStore(filepath.Join(a.profile.Root, "replay"))
}

func (a *App) notificationStore() notifications.Store {
	if a == nil || a.profile == nil {
		return notifications.Store{}
	}
	return notifications.NewStore(a.profile.Notifications)
}

func (a *App) internetService() *internet.Service {
	service := internet.New(a.config.Internet, a.profile.Root, a.profile.Logs)
	service.PolicyMode = safety.NormalizePolicyMode(a.config.Security.Policy.Mode)
	return service
}

func (a *App) schedulerStore(ctx context.Context) (*scheduler.Store, error) {
	return scheduler.Open(ctx, a.profile.Database)
}

func (a *App) jobRunner() scheduler.Runner {
	return scheduler.RunnerFunc(func(ctx context.Context, job scheduler.Job) (any, error) {
		if err := scheduler.ValidateTarget(job.TargetType, job.TargetName); err != nil {
			return nil, err
		}
		switch job.TargetType {
		case scheduler.TargetExtension:
			if err := a.extensionStore().ValidateRunInput(job.TargetName, job.Input); err != nil {
				return nil, err
			}
			result, err := a.extensionStore().Run(ctx, job.TargetName, extensions.RunOptions{
				Input:             job.Input,
				MaxRuntimeSeconds: a.config.Scheduler.DefaultJobTimeoutSeconds,
				Internet:          a.internetService(),
			})
			if err != nil {
				return result, err
			}
			return result, nil
		case scheduler.TargetHeartbeat:
			report, err := a.heartbeatReport(ctx)
			if err != nil {
				return report, err
			}
			return report, nil
		default:
			return nil, fmt.Errorf("unsupported job target type: %s", job.TargetType)
		}
	})
}

func (a *App) heartbeatReport(ctx context.Context) (heartbeat.Report, error) {
	store, err := a.schedulerStore(ctx)
	if err != nil {
		return heartbeat.Report{}, err
	}
	defer store.Close()
	schedulerStatus, err := store.Status(ctx, a.config.Scheduler.Enabled, a.schedulerMaxParallel())
	if err != nil {
		return heartbeat.Report{}, err
	}
	return heartbeat.Run(ctx, heartbeat.Input{
		Config:          a.config,
		Profile:         a.profile,
		Memory:          a.store,
		Runtime:         a.modelRuntime,
		SchedulerStatus: &schedulerStatus,
		ExtensionStore:  a.extensionStore(),
	}), nil
}

func (a *App) jobRunOptions() scheduler.RunOptions {
	return scheduler.RunOptions{
		Enabled:        a.config.Scheduler.Enabled,
		TimeoutSeconds: a.config.Scheduler.DefaultJobTimeoutSeconds,
		MaxParallel:    a.schedulerMaxParallel(),
		LowMemoryMode:  a.config.Runtime.LowMemoryMode,
		PolicyMode:     safety.NormalizePolicyMode(a.config.Security.Policy.Mode),
	}
}

func (a *App) schedulerMaxParallel() int {
	maxParallel := a.config.Scheduler.MaxParallelJobs
	if a.config.Runtime.LowMemoryMode && a.config.Scheduler.LowMemoryMaxParallelJobs > 0 {
		maxParallel = a.config.Scheduler.LowMemoryMaxParallelJobs
	}
	if maxParallel <= 0 {
		return 1
	}
	return maxParallel
}

func (a *App) createSkillFromSession(conversationID string) (skills.Skill, error) {
	messages, err := a.store.ListConversationMessages(a.ctxOrBackground(), conversationID, a.config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := a.store.ListToolRunsForConversation(a.ctxOrBackground(), conversationID, 50)
	if err != nil {
		return skills.Skill{}, err
	}
	transcript := make([]skills.TranscriptMessage, 0, len(messages))
	for _, msg := range messages {
		transcript = append(transcript, skills.TranscriptMessage{Role: msg.Role, Content: msg.Content})
	}
	runs := make([]skills.ToolRunSummary, 0, len(toolRuns))
	for _, run := range toolRuns {
		runs = append(runs, skills.ToolRunSummary{ToolName: run.ToolName, Status: run.Status})
	}
	return skills.CreateFromSession(skills.GenerateInput{
		ProfileSkillsDir: a.profile.Skills,
		ConversationID:   conversationID,
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func (a *App) improveSkillFromSession(name string, conversationID string) (skills.Skill, error) {
	skill, ok := a.skills.Get(strings.TrimSpace(name))
	if !ok {
		return skills.Skill{}, fmt.Errorf("skill not found: %s", name)
	}
	messages, err := a.store.ListConversationMessages(a.ctxOrBackground(), conversationID, a.config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := a.store.ListToolRunsForConversation(a.ctxOrBackground(), conversationID, 50)
	if err != nil {
		return skills.Skill{}, err
	}
	transcript := make([]skills.TranscriptMessage, 0, len(messages))
	for _, msg := range messages {
		transcript = append(transcript, skills.TranscriptMessage{Role: msg.Role, Content: msg.Content})
	}
	runs := make([]skills.ToolRunSummary, 0, len(toolRuns))
	for _, run := range toolRuns {
		runs = append(runs, skills.ToolRunSummary{ToolName: run.ToolName, Status: run.Status})
	}
	return skills.ImproveFromSession(skills.ImproveInput{
		ProfileSkillsDir: a.profile.Skills,
		ConversationID:   conversationID,
		Skill:            skill,
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func (a *App) ctxOrBackground() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) workspaceGrantStore() safety.WorkspaceGrantStore {
	if a.profile == nil {
		return safety.WorkspaceGrantStore{}
	}
	return safety.NewWorkspaceGrantStore(safety.WorkspaceGrantsPath(a.profile.Permissions))
}

func (a *App) workspaceRoot() string {
	if root, ok := currentProjectRoot(); ok {
		return root
	}
	if root, ok := workspaceRootFromGrants(a.workspaceGrantStore()); ok {
		return root
	}
	cwd, err := os.Getwd()
	if err == nil && !isBroadDesktopWorkspaceRoot(cwd) && !isInsideAppBundle(cwd) {
		return filepath.Clean(cwd)
	}
	if a.profile != nil && strings.TrimSpace(a.profile.Root) != "" {
		return filepath.Clean(a.profile.Root)
	}
	return "."
}

func (a *App) workspaceRootPath(path string) string {
	path = safety.NormalizeUserSuppliedPath(strings.TrimSpace(path))
	if path == "" || path == "." {
		return a.workspaceRoot()
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(a.workspaceRoot(), path))
}

func currentProjectRoot() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	return nearestDesktopProjectRoot(cwd)
}

func workspaceRootFromGrants(store safety.WorkspaceGrantStore) (string, bool) {
	grants, err := store.List()
	if err != nil || len(grants) == 0 {
		return "", false
	}
	paths := make([]string, 0, len(grants))
	for i := len(grants) - 1; i >= 0; i-- {
		path := filepath.Clean(strings.TrimSpace(grants[i].Path))
		if path == "" {
			continue
		}
		if root, ok := nearestDesktopProjectRoot(path); ok {
			return root, true
		}
		if !isBroadDesktopWorkspaceRoot(path) {
			paths = append(paths, path)
		}
	}
	if root := commonDesktopWorkspaceRoot(paths); root != "" && !isBroadDesktopWorkspaceRoot(root) {
		return root, true
	}
	if len(paths) > 0 {
		return paths[0], true
	}
	return "", false
}

func nearestDesktopProjectRoot(path string) (string, bool) {
	path = filepath.Clean(path)
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		path = filepath.Dir(path)
	}
	for {
		if isBroadDesktopWorkspaceRoot(path) {
			return "", false
		}
		if hasDesktopProjectMarker(path) {
			return path, true
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", false
		}
		path = parent
	}
}

func hasDesktopProjectMarker(path string) bool {
	for _, marker := range []string{".git", "go.mod", "wails.json", "package.json", "pyproject.toml", "pytest.ini", "requirements.txt"} {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	return false
}

func commonDesktopWorkspaceRoot(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	common := filepath.Clean(paths[0])
	for _, path := range paths[1:] {
		common = commonPathPrefix(common, filepath.Clean(path))
		if common == "" {
			return ""
		}
	}
	return common
}

func commonPathPrefix(left string, right string) string {
	leftParts := splitCleanPath(left)
	rightParts := splitCleanPath(right)
	if len(leftParts) == 0 || len(rightParts) == 0 || leftParts[0] != rightParts[0] {
		return ""
	}
	limit := len(leftParts)
	if len(rightParts) < limit {
		limit = len(rightParts)
	}
	i := 0
	for i < limit && leftParts[i] == rightParts[i] {
		i++
	}
	if i == 0 {
		return ""
	}
	return filepath.Join(leftParts[:i]...)
}

func splitCleanPath(path string) []string {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	rest := strings.TrimPrefix(clean, volume)
	rest = strings.Trim(rest, string(filepath.Separator))
	parts := []string{volume + string(filepath.Separator)}
	if volume == "" {
		parts[0] = string(filepath.Separator)
	}
	if rest == "" {
		return parts
	}
	return append(parts, strings.Split(rest, string(filepath.Separator))...)
}

func isBroadDesktopWorkspaceRoot(path string) bool {
	clean := filepath.Clean(path)
	if clean == "." {
		return false
	}
	broad := map[string]bool{
		string(filepath.Separator):      true,
		filepath.Clean("/Applications"): true,
		filepath.Clean("/System"):       true,
		filepath.Clean("/Library"):      true,
		filepath.Clean("/Users"):        true,
	}
	if home, err := os.UserHomeDir(); err == nil {
		broad[filepath.Clean(home)] = true
	}
	return broad[clean]
}

func isInsideAppBundle(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(filepath.Clean(path)), "/") {
		if strings.HasSuffix(strings.ToLower(part), ".app") {
			return true
		}
	}
	return false
}

func (a *App) beginRun() (context.Context, int) {
	ctx, cancel := context.WithCancel(a.ctxOrBackground())
	a.mu.Lock()
	if a.activeCancel != nil {
		a.activeCancel()
	}
	a.activeRunID++
	runID := a.activeRunID
	a.activeCancel = cancel
	a.mu.Unlock()
	return ctx, runID
}

func (a *App) endRun(runID int) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.activeRunID == runID {
		a.activeCancel = nil
	}
}

func (a *App) emit(channel string, event agent.Event) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, "yemaka:"+channel, event)
}

func (a *App) selectSkill(explicit string, content string) (skills.Skill, bool, error) {
	available := availableSkillTools()
	if explicit != "" {
		skill, ok := a.skills.Get(explicit)
		if !ok {
			return skills.Skill{}, false, fmt.Errorf("skill not found: %s", explicit)
		}
		if skill.Disabled {
			return skills.Skill{}, false, fmt.Errorf("skill is disabled: %s", explicit)
		}
		for _, tool := range skill.RequiredTools {
			if !available[tool] {
				return skills.Skill{}, false, fmt.Errorf("skill %s requires unavailable tool: %s", explicit, tool)
			}
		}
		return skill, true, nil
	}
	skill, ok := a.skills.Select(content, available)
	return skill, ok, nil
}

func knownModelRole(role string) bool {
	switch role {
	case "default", "low_memory", "coding", "reasoning", "stronger_local":
		return true
	default:
		return false
	}
}

func normalizeModelRole(role string) string {
	return strings.ToLower(strings.TrimSpace(role))
}

func configuredModelStatuses(cfg *config.Config, installed []models.ModelInfo) []ModelStatus {
	statuses := make([]ModelStatus, 0, len(cfg.Models))
	roles := []string{"default", "low_memory", "coding", "reasoning", "stronger_local"}
	for _, role := range roles {
		model, ok := cfg.Models[role]
		if !ok {
			continue
		}
		statuses = append(statuses, ModelStatus{
			Role:          role,
			Name:          model.Name,
			Installed:     modelInstalled(model.Name, installed),
			Profile:       strings.TrimSpace(model.Profile),
			ProfileValid:  strings.TrimSpace(model.Profile) != "",
			ProfileStatus: modelProfileStatusLabel(model),
		})
	}
	return statuses
}

func modelProfileStatusLabel(model config.ModelConfig) string {
	if strings.TrimSpace(model.Profile) == "" {
		return "none"
	}
	return "applied"
}

func modelInstalled(name string, installed []models.ModelInfo) bool {
	return models.ModelInstalled(name, installed)
}

func skillSummary(status skills.SkillStatus) SkillSummary {
	return SkillSummary{
		Name:            status.Name,
		Version:         status.Version,
		Description:     status.Description,
		Triggers:        status.Triggers,
		RequiredTools:   status.RequiredTools,
		Enabled:         status.Enabled,
		Valid:           status.Valid,
		ValidationError: status.ValidationError,
		Source:          status.Source,
		Dir:             status.Dir,
	}
}

func skillSummaryFromSkill(skill skills.Skill) SkillSummary {
	return SkillSummary{
		Name:          skill.Name,
		Version:       skill.Version,
		Description:   skill.Description,
		Triggers:      skill.Triggers,
		RequiredTools: skill.RequiredTools,
		Enabled:       !skill.Disabled,
		Valid:         true,
		Source:        skill.Source,
		Dir:           skill.Dir,
	}
}

func (a *App) openEnabledKnowledgeStore(ctx context.Context) (*knowledge.Store, error) {
	if a.config == nil || !a.config.Knowledge.Enabled {
		return nil, fmt.Errorf("local knowledge graph is disabled; enable manual knowledge graph first")
	}
	if strings.TrimSpace(a.config.Memory.Database) == "" {
		return nil, fmt.Errorf("knowledge graph database is not configured")
	}
	return knowledge.OpenWithOptions(ctx, a.config.Memory.Database, knowledge.Options{
		MaxEvidenceChars: a.config.Knowledge.MaxEvidenceChars,
	})
}

func (a *App) knowledgeStatus(ctx context.Context) (KnowledgeStatus, error) {
	if a.config == nil {
		return KnowledgeStatus{}, fmt.Errorf("config is not loaded")
	}
	status := KnowledgeStatus{
		Enabled:              a.config.Knowledge.Enabled,
		InfluenceEnabled:     a.config.Knowledge.InfluenceEnabled && a.config.Knowledge.Enabled,
		ManualOnly:           true,
		MaxEntitiesPerQuery:  a.config.Knowledge.MaxEntitiesPerQuery,
		MaxEvidenceChars:     a.config.Knowledge.MaxEvidenceChars,
		MaxInfluenceEntities: a.config.Knowledge.MaxInfluenceEntities,
		MaxInfluenceChars:    a.config.Knowledge.MaxInfluenceChars,
		Database:             a.config.Memory.Database,
		Message:              "manual-only local graph; disabled until explicitly enabled",
	}
	if !a.config.Knowledge.Enabled {
		return status, nil
	}
	store, err := knowledge.OpenWithOptions(ctx, a.config.Memory.Database, knowledge.Options{
		MaxEvidenceChars: a.config.Knowledge.MaxEvidenceChars,
	})
	if err != nil {
		return status, err
	}
	defer store.Close()
	stats, err := store.Stats(ctx)
	if err != nil {
		return status, err
	}
	status.EntityCount = stats.Entities
	status.EdgeCount = stats.Edges
	if status.InfluenceEnabled {
		status.Message = "manual-only local graph is enabled; approved records may influence ordinary chat as background context"
	} else {
		status.Message = "manual-only local graph is enabled; routing and RAG do not use it automatically"
	}
	return status, nil
}

func (a *App) modelProfileStore() modelprofiles.Store {
	root := ""
	if a != nil && a.profile != nil {
		root = a.profile.Root
	}
	return modelprofiles.NewStore(filepath.Join(root, "model_profiles"))
}

func (a *App) modelProfileBindingStatus(name string) (bool, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, "none"
	}
	if _, err := a.modelProfileStore().Load(name); err != nil {
		return false, "missing"
	}
	return true, "applied"
}

func desktopModelProfileFromInput(input ModelProfileInput) modelprofiles.Profile {
	return modelprofiles.Profile{
		Name:        input.Name,
		Description: input.Description,
		BaseModel:   input.BaseModel,
		System:      input.System,
		Parameters: modelprofiles.Parameters{
			Temperature: input.Temperature,
			NumCtx:      input.NumCtx,
		},
		Metadata: modelprofiles.Metadata{
			Purpose:     input.Purpose,
			RefinedFrom: input.RefinedFrom,
			CreatedBy:   "yemaka",
		},
		Tags: input.Tags,
	}
}

func desktopModelProfileResult(profile modelprofiles.Profile, path string, message string) (ModelProfileResult, error) {
	profile = modelprofiles.Normalize(profile)
	modelfile, err := modelprofiles.RenderValidatedModelfile(profile)
	if err != nil {
		return ModelProfileResult{}, err
	}
	return ModelProfileResult{
		Profile:   profile,
		Path:      path,
		Modelfile: modelfile,
		Message:   message,
		Readiness: modelprofiles.AssessReadiness(profile),
		SafetySummary: []string{
			"local artifact only",
			"does not train or upload data",
			"does not auto-switch model roles",
			"does not download or pull models",
			"secrets and raw private memory markers are rejected",
		},
	}, nil
}

func enrichDesktopModelProfileResult(store modelprofiles.Store, result *ModelProfileResult) {
	if result == nil {
		return
	}
	result.Readiness = modelprofiles.AssessReadiness(result.Profile)
	if strings.TrimSpace(result.Comparison.Schema) == "" {
		if comparison, err := store.CompareCurrent(result.Profile); err == nil {
			result.Comparison = comparison
		}
	}
	if history, err := store.History(result.Profile.Name); err == nil {
		result.History = history
	}
}

func modelProfilePath(dir string, name string) string {
	filename, err := modelprofiles.FileName(name)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, filename)
}

func desktopModelProfileDraftReport(report learning.ModelProfileDraftReport) *ModelProfileDraftReport {
	return &ModelProfileDraftReport{
		Schema:            report.Schema,
		ConversationID:    report.ConversationID,
		Eligibility:       report.Eligibility,
		Reason:            report.Reason,
		MessageCount:      report.MessageCount,
		AssistantMessages: report.AssistantMessages,
		ToolRunCount:      report.ToolRunCount,
		FailedToolRuns:    report.FailedToolRuns,
		ObservedTraits:    append([]string{}, report.ObservedTraits...),
		PrivacyFilter:     report.PrivacyFilter,
		SafetySummary:     append([]string{}, report.SafetySummary...),
		Readiness:         report.Readiness,
	}
}

func localInstalledModels(installed []models.ModelInfo) []models.ModelInfo {
	return models.LocalInstalledModels(installed)
}

func recommendInstalledModel(installed []models.ModelInfo) string {
	for _, model := range installed {
		name := strings.ToLower(model.Name)
		if strings.Contains(name, "1.5b") || strings.Contains(name, "2b") || strings.Contains(name, "3b") || strings.Contains(name, "mini") {
			return model.Name
		}
	}
	return ""
}

func availableSkillTools() map[string]bool {
	return tools.ToolAvailabilityForSurface(tools.ToolSurfaceSkill)
}

func workspaceLimits(cfg config.WorkspaceConfig) workspace.Limits {
	return workspace.Limits{
		MaxFilesScanned:   cfg.MaxFilesScanned,
		MaxFileBytes:      cfg.MaxFileBytes,
		MaxTotalScanBytes: cfg.MaxTotalScanBytes,
		MaxSearchResults:  cfg.MaxSearchResults,
		MaxContextFiles:   cfg.MaxContextFiles,
		MaxContextChars:   cfg.MaxContextChars,
		IncludeHidden:     cfg.IncludeHidden,
		FollowSymlinks:    cfg.FollowSymlinks,
	}
}

func workspaceGrantResult(grant safety.WorkspaceGrant) WorkspaceGrantResult {
	return WorkspaceGrantResult{
		ID:            grant.ID,
		Path:          grant.Path,
		Label:         grant.Label,
		Source:        grant.Source,
		CreatedAt:     grant.CreatedAt,
		BookmarkReady: grant.SecurityBookmark != "",
		BookmarkStale: grant.BookmarkStale,
	}
}

func embeddingIndexSummary(result rag.EmbeddingIndexResult) EmbeddingIndexSummary {
	return EmbeddingIndexSummary{
		Enabled:        result.Enabled,
		Provider:       result.Provider,
		Model:          result.Model,
		ChunksEmbedded: result.ChunksEmbedded,
		ChunksSkipped:  result.ChunksSkipped,
		Dimensions:     result.Dimensions,
	}
}

func documentInventoryResults(items []rag.DocumentInventoryItem) []DocumentInventoryResult {
	out := make([]DocumentInventoryResult, 0, len(items))
	for _, item := range items {
		out = append(out, DocumentInventoryResult{
			ID:                 item.ID,
			Path:               item.Path,
			WorkspaceRoot:      item.WorkspaceRoot,
			ManagedSource:      item.ManagedSource,
			Status:             item.Status,
			Reason:             item.Reason,
			Missing:            item.Missing,
			Stale:              item.Stale,
			SizeBytes:          item.SizeBytes,
			ChunkCount:         item.ChunkCount,
			EmbeddingCount:     item.EmbeddingCount,
			IndexedAt:          item.IndexedAt,
			UpdatedAt:          item.UpdatedAt,
			ContentHash:        item.ContentHash,
			CurrentContentHash: item.CurrentContentHash,
		})
	}
	return out
}

func documentPruneResult(result rag.DocumentPruneResult) DocumentPruneResult {
	return DocumentPruneResult{
		DryRun:            result.DryRun,
		DocumentsMatched:  result.DocumentsMatched,
		ChunksMatched:     result.ChunksMatched,
		FTSRowsMatched:    result.FTSRowsMatched,
		EmbeddingsMatched: result.EmbeddingsMatched,
		DocumentsRemoved:  result.DocumentsRemoved,
		ChunksRemoved:     result.ChunksRemoved,
		FTSRowsRemoved:    result.FTSRowsRemoved,
		EmbeddingsRemoved: result.EmbeddingsRemoved,
		DocumentIDs:       append([]string(nil), result.DocumentIDs...),
	}
}

func ragConfig(cfg config.RAGConfig) rag.Config {
	return rag.Config{
		Enabled:          cfg.Enabled,
		Mode:             cfg.Mode,
		ChunkSize:        cfg.ChunkSize,
		ChunkOverlap:     cfg.ChunkOverlap,
		TopK:             cfg.TopK,
		MaxFileBytes:     cfg.MaxFileBytes,
		MaxTotalBytes:    cfg.MaxTotalBytes,
		MaxChunksPerFile: cfg.MaxChunksPerFile,
		Rerank: rag.RerankConfig{
			CandidateLimit:  cfg.Rerank.CandidateLimit,
			SourceDiversity: cfg.Rerank.SourceDiversity,
		},
		Embeddings: rag.EmbeddingConfig{
			Enabled:        cfg.Embeddings.Enabled,
			Provider:       cfg.Embeddings.Provider,
			Model:          cfg.Embeddings.Model,
			BatchSize:      cfg.Embeddings.BatchSize,
			MaxTextChars:   cfg.Embeddings.MaxTextChars,
			CandidateLimit: cfg.Embeddings.CandidateLimit,
		},
	}
}

func embeddingRuntime(runtime models.Runtime) models.EmbeddingRuntime {
	embedder, ok := runtime.(models.EmbeddingRuntime)
	if !ok {
		return nil
	}
	return embedder
}

func toolRunResults(runs []memory.ToolRun) []ToolRunResult {
	result := make([]ToolRunResult, 0, len(runs))
	for _, run := range runs {
		result = append(result, ToolRunResult{
			ID:                 run.ID,
			ConversationID:     run.ConversationID,
			SessionID:          run.SessionID,
			UserMessageID:      run.UserMessageID,
			AssistantMessageID: run.AssistantMessageID,
			ParentMessageID:    run.ParentMessageID,
			VariantIndex:       run.VariantIndex,
			ToolName:           run.ToolName,
			Input:              run.Input,
			Output:             run.Output,
			Status:             run.Status,
			RiskLevel:          run.RiskLevel,
			CreatedAt:          run.CreatedAt,
			CompletedAt:        run.CompletedAt,
		})
	}
	return result
}

func limitLatestReplayTraces(traces []replay.TraceFile, limit int) []replay.TraceFile {
	if len(traces) == 0 {
		return []replay.TraceFile{}
	}
	out := make([]replay.TraceFile, 0, len(traces))
	for i := len(traces) - 1; i >= 0; i-- {
		out = append(out, traces[i])
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

func toolCatalogResults(entries []tools.ToolCatalogEntry) []ToolCatalogEntryResult {
	out := make([]ToolCatalogEntryResult, 0, len(entries))
	for _, entry := range entries {
		surfaces := make([]string, 0, len(entry.Surfaces))
		for _, surface := range entry.Surfaces {
			surfaces = append(surfaces, string(surface))
		}
		out = append(out, ToolCatalogEntryResult{
			Name:             entry.Name,
			Status:           string(entry.Status),
			Surfaces:         surfaces,
			ChatCallable:     entry.ChatCallable,
			ModelCallable:    entry.ModelCallable,
			RequiresApproval: entry.RequiresApproval,
			EnabledByDefault: entry.EnabledByDefault,
			Mutating:         entry.Mutating,
			Notes:            entry.Notes,
		})
	}
	return out
}

func toolSurfaceFromString(value string) tools.ToolSurface {
	switch tools.ToolSurface(strings.TrimSpace(value)) {
	case tools.ToolSurfaceCLI:
		return tools.ToolSurfaceCLI
	case tools.ToolSurfaceSkill:
		return tools.ToolSurfaceSkill
	case tools.ToolSurfaceExtension:
		return tools.ToolSurfaceExtension
	case tools.ToolSurfaceFuture:
		return tools.ToolSurfaceFuture
	default:
		return tools.ToolSurfaceChat
	}
}

func desktopConversationResults(conversations []memory.Conversation) []ConversationResult {
	result := make([]ConversationResult, 0, len(conversations))
	for _, conversation := range conversations {
		result = append(result, desktopConversationResult(conversation))
	}
	return result
}

func desktopConversationResult(conversation memory.Conversation) ConversationResult {
	return ConversationResult{
		ID:        conversation.ID,
		Title:     conversation.Title,
		CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt,
		Starred:   conversation.Starred,
	}
}

func desktopConversationMessageResults(messages []memory.Message, activities map[string]agent.MessageActivity, turns []memory.AgentTurn) []ConversationMessageResult {
	return desktopConversationMessageResultsWithAttachments(messages, activities, turns, nil)
}

func desktopConversationMessageResultsWithAttachments(messages []memory.Message, activities map[string]agent.MessageActivity, turns []memory.AgentTurn, attachmentsByMessage map[string][]ChatAttachmentResult) []ConversationMessageResult {
	messages = memory.WithInferredResponseParents(messages)
	turnsByUser := desktopRenderableAgentTurnsByUser(messages, turns)
	result := make([]ConversationMessageResult, 0, len(messages)+len(turnsByUser))
	for _, message := range messages {
		activity := activities[message.ID]
		result = append(result, ConversationMessageResult{
			ID:            message.ID,
			Role:          message.Role,
			Content:       message.Content,
			Model:         message.Model,
			CreatedAt:     message.CreatedAt,
			ParentID:      message.ParentID,
			VariantIndex:  message.VariantIndex,
			ActiveVariant: message.ActiveVariant,
			Trace:         activity.Trace,
			Sources:       activity.Sources,
			SourceKind:    activity.SourceKind,
			Attachments:   attachmentsByMessage[message.ID],
		})
		if message.Role != "user" || message.ID == "" {
			continue
		}
		for _, turn := range turnsByUser[message.ID] {
			result = append(result, desktopConversationAgentTurnPlaceholder(turn))
		}
		delete(turnsByUser, message.ID)
	}
	return result
}

func (a *App) chatAttachmentsForMessage(ctx context.Context, messageID string) []ChatAttachmentResult {
	itemsByMessage, err := a.store.ListChatAttachmentsForMessages(ctx, []string{messageID})
	if err != nil {
		return nil
	}
	return desktopChatAttachmentResults(itemsByMessage[messageID])
}

func (a *App) chatAttachmentsForMessages(ctx context.Context, messages []memory.Message) (map[string][]ChatAttachmentResult, error) {
	messageIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.ID != "" {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	itemsByMessage, err := a.store.ListChatAttachmentsForMessages(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]ChatAttachmentResult, len(itemsByMessage))
	for messageID, items := range itemsByMessage {
		out[messageID] = desktopChatAttachmentResults(items)
	}
	return out, nil
}

func desktopChatAttachmentResults(items []memory.ChatAttachment) []ChatAttachmentResult {
	out := make([]ChatAttachmentResult, 0, len(items))
	for _, item := range items {
		out = append(out, desktopChatAttachmentResult(item))
	}
	return out
}

func desktopChatAttachmentResult(item memory.ChatAttachment) ChatAttachmentResult {
	return ChatAttachmentResult{
		ID:          item.ID,
		FileName:    item.FileName,
		ContentType: item.ContentType,
		SizeBytes:   item.SizeBytes,
		Status:      item.Status,
		Summary:     item.Summary,
		Preview:     item.Preview,
		SourceKind:  item.SourceKind,
		Sources:     append([]string{}, item.Sources...),
		Retention:   item.Retention,
		CreatedAt:   item.CreatedAt,
	}
}

func desktopRenderableAgentTurnsByUser(messages []memory.Message, turns []memory.AgentTurn) map[string][]memory.AgentTurn {
	answeredUserIDs := make(map[string]bool)
	for _, message := range messages {
		if message.Role == "assistant" && strings.TrimSpace(message.ParentID) != "" {
			answeredUserIDs[message.ParentID] = true
		}
	}
	turnsByUser := make(map[string][]memory.AgentTurn)
	for _, turn := range turns {
		if !desktopShouldRenderAgentTurnPlaceholder(turn) || answeredUserIDs[turn.UserMessageID] {
			continue
		}
		turnsByUser[turn.UserMessageID] = append(turnsByUser[turn.UserMessageID], turn)
	}
	return turnsByUser
}

func desktopShouldRenderAgentTurnPlaceholder(turn memory.AgentTurn) bool {
	if strings.TrimSpace(turn.UserMessageID) == "" || strings.TrimSpace(turn.AssistantMessageID) != "" {
		return false
	}
	switch strings.TrimSpace(turn.Status) {
	case agent.AgentTurnRunning, agent.AgentTurnInterrupted, agent.AgentTurnFailed, agent.AgentTurnCompleted:
		return true
	default:
		return false
	}
}

func desktopConversationAgentTurnPlaceholder(turn memory.AgentTurn) ConversationMessageResult {
	createdAt := turn.UpdatedAt
	if createdAt == "" {
		createdAt = turn.CreatedAt
	}
	meta := "stopped"
	content := "Stopped before a final answer was saved."
	switch strings.TrimSpace(turn.Status) {
	case agent.AgentTurnRunning:
		meta = "working"
		content = ""
	case agent.AgentTurnFailed:
		meta = "failed"
		content = "Yemaka stopped before a final answer was saved."
	case agent.AgentTurnCompleted:
		meta = "completed"
		content = "Completed before a final answer was saved."
	}
	return ConversationMessageResult{
		ID:            turn.ID,
		Role:          "assistant",
		Content:       content,
		Meta:          meta,
		Model:         turn.Model,
		CreatedAt:     createdAt,
		ParentID:      turn.UserMessageID,
		ActiveVariant: true,
		Trace:         turn.Trace,
		Sources:       turn.Sources,
		SourceKind:    turn.SourceKind,
	}
}

func combinedMemoryResults(memories []memory.MemorySearchResult, messages []memory.SearchResult) []MemoryResult {
	result := explicitMemoryResults(memories)
	for _, item := range messages {
		result = append(result, MemoryResult{
			MessageID:      item.MessageID,
			ConversationID: item.ConversationID,
			Role:           item.Role,
			Snippet:        item.Snippet,
			CreatedAt:      item.CreatedAt,
		})
	}
	return result
}

func explicitMemoryResults(memories []memory.MemorySearchResult) []MemoryResult {
	result := make([]MemoryResult, 0, len(memories))
	for _, item := range memories {
		result = append(result, MemoryResult{
			ID:         item.ID,
			Role:       item.Kind,
			Kind:       item.Kind,
			Content:    item.Content,
			Snippet:    item.Snippet,
			Importance: item.Importance,
			Source:     item.Source,
			Pinned:     item.Pinned,
			Explicit:   true,
			CreatedAt:  item.CreatedAt,
		})
	}
	return result
}

func memoryItemResult(item memory.Memory) MemoryResult {
	return MemoryResult{
		ID:         item.ID,
		Role:       item.Kind,
		Kind:       item.Kind,
		Content:    item.Content,
		Snippet:    item.Content,
		Importance: item.Importance,
		Source:     item.Source,
		Pinned:     item.Pinned,
		Explicit:   true,
		CreatedAt:  item.CreatedAt,
	}
}

func knowledgeEntityResults(entities []knowledge.Entity) []KnowledgeEntityResult {
	result := make([]KnowledgeEntityResult, 0, len(entities))
	for _, entity := range entities {
		result = append(result, knowledgeEntityResult(entity))
	}
	return result
}

func knowledgeEntityResult(entity knowledge.Entity) KnowledgeEntityResult {
	return KnowledgeEntityResult{
		ID:           entity.ID,
		Name:         entity.Name,
		Kind:         entity.Kind,
		Source:       entity.Source,
		SourceKind:   entity.SourceKind,
		SourceRef:    entity.SourceRef,
		Evidence:     entity.Evidence,
		ReviewStatus: entity.ReviewStatus,
		ReviewNote:   entity.ReviewNote,
		ReviewedBy:   entity.ReviewedBy,
		ReviewedAt:   entity.ReviewedAt,
		CreatedAt:    entity.CreatedAt,
		UpdatedAt:    entity.UpdatedAt,
	}
}

func knowledgeEdgeResult(edge knowledge.Edge) KnowledgeEdgeResult {
	return KnowledgeEdgeResult{
		ID:           edge.ID,
		FromEntityID: edge.FromEntityID,
		Relation:     edge.Relation,
		ToEntityID:   edge.ToEntityID,
		Source:       edge.Source,
		SourceKind:   edge.SourceKind,
		SourceRef:    edge.SourceRef,
		Evidence:     edge.Evidence,
		ReviewStatus: edge.ReviewStatus,
		ReviewNote:   edge.ReviewNote,
		ReviewedBy:   edge.ReviewedBy,
		ReviewedAt:   edge.ReviewedAt,
		CreatedAt:    edge.CreatedAt,
		UpdatedAt:    edge.UpdatedAt,
	}
}

func knowledgeProposalResult(proposal knowledge.Proposal) KnowledgeProposalResult {
	result := KnowledgeProposalResult{
		Source:            proposal.Source,
		SourceKind:        proposal.SourceKind,
		SourceRef:         proposal.SourceRef,
		ReviewRequired:    proposal.ReviewRequired,
		Message:           proposal.Message,
		EntityProposals:   []KnowledgeEntityProposalResult{},
		RelationProposals: []KnowledgeRelationProposalResult{},
	}
	for _, entity := range proposal.EntityProposals {
		result.EntityProposals = append(result.EntityProposals, KnowledgeEntityProposalResult{
			Name:       entity.Name,
			Kind:       entity.Kind,
			Source:     entity.Source,
			SourceKind: entity.SourceKind,
			SourceRef:  entity.SourceRef,
			Evidence:   entity.Evidence,
			Reason:     entity.Reason,
		})
	}
	for _, relation := range proposal.RelationProposals {
		result.RelationProposals = append(result.RelationProposals, KnowledgeRelationProposalResult{
			FromName:   relation.FromName,
			Relation:   relation.Relation,
			ToName:     relation.ToName,
			Source:     relation.Source,
			SourceKind: relation.SourceKind,
			SourceRef:  relation.SourceRef,
			Evidence:   relation.Evidence,
			Reason:     relation.Reason,
		})
	}
	return result
}

func knowledgeReviewItemResults(items []knowledge.ReviewItem) []KnowledgeReviewItemResult {
	result := make([]KnowledgeReviewItemResult, 0, len(items))
	for _, item := range items {
		result = append(result, knowledgeReviewItemResult(item))
	}
	return result
}

func knowledgeReviewItemResult(item knowledge.ReviewItem) KnowledgeReviewItemResult {
	return KnowledgeReviewItemResult{
		ID:           item.ID,
		Type:         item.Type,
		Name:         item.Name,
		Kind:         item.Kind,
		FromEntityID: item.FromEntityID,
		Relation:     item.Relation,
		ToEntityID:   item.ToEntityID,
		Source:       item.Source,
		SourceKind:   item.SourceKind,
		SourceRef:    item.SourceRef,
		Evidence:     item.Evidence,
		ReviewStatus: item.ReviewStatus,
		ReviewNote:   item.ReviewNote,
		ReviewedBy:   item.ReviewedBy,
		ReviewedAt:   item.ReviewedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func knowledgeReviewBatchResult(result knowledge.ReviewBatchResult) KnowledgeReviewBatchResult {
	return KnowledgeReviewBatchResult{
		Action:          result.Action,
		Requested:       result.Requested,
		EntitiesMatched: result.EntitiesMatched,
		EdgesMatched:    result.EdgesMatched,
		EntitiesUpdated: result.EntitiesUpdated,
		EdgesUpdated:    result.EdgesUpdated,
		Items:           knowledgeReviewItemResults(result.Items),
	}
}

func knowledgeRepairResult(result knowledge.RepairResult, status KnowledgeStatus) KnowledgeRepairResult {
	updates := result.EntitiesUpdated + result.EdgesUpdated
	message := "knowledge graph evidence already clean"
	if updates == 1 {
		message = "repaired 1 knowledge graph evidence field"
	} else if updates > 1 {
		message = fmt.Sprintf("repaired %d knowledge graph evidence fields", updates)
	}
	return KnowledgeRepairResult{
		EntitiesScanned: result.EntitiesScanned,
		EntitiesUpdated: result.EntitiesUpdated,
		EdgesScanned:    result.EdgesScanned,
		EdgesUpdated:    result.EdgesUpdated,
		Status:          status,
		Message:         message,
	}
}

func fileWriteOptions(cfg *config.Config, profile *profiles.Profile) workspace.WriteOptions {
	options := workspace.WriteOptions{}
	if cfg != nil {
		options.SnapshotBeforeWrite = cfg.Tools.Filesystem.SnapshotBeforeWrite
		options.FollowSymlinks = cfg.Workspace.FollowSymlinks
		options.IncludeHidden = cfg.Workspace.IncludeHidden
		options.MaxEditFileBytes = cfg.Tools.Filesystem.MaxEditFileBytes
	}
	if profile != nil {
		options.SnapshotsRoot = profile.Snapshots
	}
	return options
}

func planFileWrite(ctx context.Context, root string, input FileWriteInput, options workspace.WriteOptions) (workspace.ChangePlan, error) {
	path := strings.TrimSpace(input.Path)
	if path == "" {
		return workspace.ChangePlan{}, fmt.Errorf("file path is required")
	}
	return workspace.PlanWrite(ctx, root, path, input.Content, options)
}

func validateFileWriteInput(input FileWriteInput, cfg *config.Config) error {
	if strings.TrimSpace(input.Path) == "" {
		return fmt.Errorf("file path is required")
	}
	if cfg != nil && cfg.Tools.Filesystem.MaxFullRewriteBytes > 0 && int64(len(input.Content)) > cfg.Tools.Filesystem.MaxFullRewriteBytes {
		return fmt.Errorf("file content exceeds max rewrite size: %d bytes", cfg.Tools.Filesystem.MaxFullRewriteBytes)
	}
	return nil
}

func fileWritePlanResult(plan workspace.ChangePlan) FileWritePlanResult {
	return FileWritePlanResult{
		Path:        plan.Path,
		Diff:        plan.Diff,
		IsNew:       plan.IsNew,
		Changed:     plan.Changed,
		TargetBytes: plan.TargetBytes,
	}
}

func fileWriteApplyResult(plan workspace.ChangePlan, verification workspace.WriteVerification) FileWriteApplyResult {
	return FileWriteApplyResult{
		Path:         plan.Path,
		Diff:         plan.Diff,
		SnapshotID:   plan.SnapshotID,
		IsNew:        plan.IsNew,
		Changed:      plan.Changed,
		TargetBytes:  plan.TargetBytes,
		Verification: verification,
	}
}

func normalizePermissionDecision(decision string) (string, error) {
	decision = strings.ToLower(strings.TrimSpace(decision))
	switch decision {
	case "approved", "rejected":
		return decision, nil
	default:
		return "", fmt.Errorf("permission decision must be approved or rejected")
	}
}

func desktopCloudFallback(cfg *config.Config, runtime models.Runtime) *agent.CloudFallback {
	if cfg == nil || !cfg.CloudFallback.Enabled || runtime == nil {
		return nil
	}
	return &agent.CloudFallback{
		Config:  cfg.CloudFallback,
		Runtime: runtime,
	}
}

func desktopToolExecutor(a *App) agent.ToolExecutor {
	if a == nil {
		return nil
	}
	cfg := a.config
	if cfg == nil {
		return nil
	}
	internetService := a.internetService()
	return agent.NewSafeToolExecutor(agent.SafeToolConfig{
		WorkspaceRoot:            a.workspaceRoot(),
		TimeoutSeconds:           cfg.Tools.Shell.TimeoutSeconds,
		MaxOutputBytes:           cfg.Tools.Shell.MaxOutputBytes,
		MaxContextChars:          cfg.Workspace.MaxContextChars,
		RequireConfirmationRisky: cfg.Tools.Shell.RequireConfirmationRisky,
		FullAccess:               safety.IsFullAccessMode(cfg.Security.Policy.Mode),
		DisableShellCommands:     !cfg.Tools.Shell.Enabled,
		Config:                   cfg,
		Profile:                  a.profile,
		Memory:                   a.store,
		RAG:                      a.rag,
		Runtime:                  a.modelRuntime,
		Skills:                   a.skills,
		Internet:                 internetService,
		WorkspaceGrants:          a.workspaceGrantStore(),
		UseShellHelper:           true,
		ShellHelperPath:          tools.DefaultShellHelperPath(),
	})
}

func desktopRetriever(a *App) agent.Retriever {
	if a == nil || a.config == nil {
		return nil
	}
	return agent.NewLocalRetriever(agent.LocalRetrieverConfig{
		Config:        a.config,
		WorkspaceRoot: a.workspaceRoot(),
		RAG:           a.rag,
		Runtime:       a.modelRuntime,
	})
}

func splitEventSources(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ", ")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			clean = append(clean, part)
		}
	}
	return clean
}

func appendEventToolCallNames(current []string, event agent.Event) []string {
	names := splitEventSources(event.Data["names"])
	if len(names) == 0 && strings.TrimSpace(event.Data["count"]) != "" {
		names = []string{event.Data["count"] + " call(s)"}
	}
	return append(current, names...)
}

func atoiDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return value
}

func oneLine(input string) string {
	input = strings.Join(strings.Fields(input), " ")
	if len(input) > 280 {
		return input[:280] + "..."
	}
	return input
}
