package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

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
	"yemaka/internal/release"
	"yemaka/internal/replay"
	"yemaka/internal/routing"
	"yemaka/internal/safety"
	"yemaka/internal/scheduler"
	"yemaka/internal/secrets"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
	"yemaka/internal/workflows"
	"yemaka/internal/workspace"
)

const DefaultAddr = "127.0.0.1:7727"

type Dependencies struct {
	Config         *config.Config
	Profile        *profiles.Profile
	Memory         *memory.Store
	RAG            *rag.Store
	Skills         skills.Registry
	Runtime        models.Runtime
	RuntimeFactory func(config.ModelConfig) (models.Runtime, error)
	Cloud          models.Runtime
	Router         *models.Router
	StaticDir      string
	Workspace      string
	FrontendWatch  bool
	FrontendRoot   string
	DevLog         io.Writer
	NowTimeout     time.Duration
}

type Server struct {
	mu              sync.Mutex
	deps            Dependencies
	frontendVersion string
	frontendError   string
	activeAskCancel context.CancelFunc
	activeAskID     uint64
	connectorRates  map[string]connectorRateState
}

type connectorRateState struct {
	MinuteWindowStart time.Time
	MinuteCount       int
	DayWindowStart    time.Time
	DayCount          int
	BurstWindowStart  time.Time
	BurstCount        int
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

type OpsStatus struct {
	GeneratedAt    string                       `json:"generatedAt"`
	Overall        string                       `json:"overall"`
	Stages         []OpsStage                   `json:"stages"`
	Timeline       []OpsTimelineEvent           `json:"timeline,omitempty"`
	Heartbeat      heartbeat.Report             `json:"heartbeat"`
	Scheduler      scheduler.Status             `json:"scheduler"`
	Connectors     ConnectorOpsSummary          `json:"connectors"`
	Knowledge      KnowledgeStatus              `json:"knowledge"`
	ModelProfiles  ModelProfileOpsSummary       `json:"modelProfiles"`
	RecentJobRuns  []scheduler.JobRun           `json:"recentJobRuns"`
	LatestEval     *learning.QAEvalSummary      `json:"latestEval,omitempty"`
	QAReview       learning.QAReviewSummary     `json:"qaReview"`
	Notifications  []notifications.Notification `json:"notifications"`
	RecentReplay   []replay.TraceFile           `json:"recentReplay"`
	RecentToolRuns []ToolRunResult              `json:"recentToolRuns"`
	ExtensionAudit []extensions.AuditRecord     `json:"extensionAudit,omitempty"`
	PolicyAudit    []safety.PolicyAuditRecord   `json:"policyAudit,omitempty"`
	Release        *OpsReleaseSummary           `json:"release,omitempty"`
}

type OpsStage struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Details string `json:"details,omitempty"`
}

type OpsTimelineEvent struct {
	ID            string            `json:"id"`
	OccurredAt    string            `json:"occurredAt,omitempty"`
	Source        string            `json:"source"`
	Kind          string            `json:"kind"`
	Severity      string            `json:"severity"`
	Status        string            `json:"status,omitempty"`
	Title         string            `json:"title"`
	Summary       string            `json:"summary,omitempty"`
	EntityType    string            `json:"entityType,omitempty"`
	EntityID      string            `json:"entityId,omitempty"`
	CorrelationID string            `json:"correlationId,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Related       []OpsTimelineLink `json:"related,omitempty"`
}

type OpsTimelineLink struct {
	Source     string `json:"source"`
	EntityType string `json:"entityType"`
	EntityID   string `json:"entityId"`
	Label      string `json:"label,omitempty"`
}

type OpsReleaseSummary struct {
	Version string          `json:"version"`
	Ready   bool            `json:"ready"`
	Checks  []release.Check `json:"checks"`
}

type ConnectorOpsSummary struct {
	Total       int      `json:"total"`
	Enabled     int      `json:"enabled"`
	Disabled    int      `json:"disabled"`
	NeedsAuth   int      `json:"needsAuth"`
	Warning     int      `json:"warning"`
	Broken      int      `json:"broken"`
	Status      string   `json:"status"`
	EnabledList []string `json:"enabledList,omitempty"`
}

type ModelProfileOpsSummary struct {
	ProfileCount      int      `json:"profileCount"`
	ConfiguredRoles   int      `json:"configuredRoles"`
	AppliedRoles      int      `json:"appliedRoles"`
	MissingBindings   int      `json:"missingBindings"`
	MissingProfiles   []string `json:"missingProfiles,omitempty"`
	AvailableProfiles []string `json:"availableProfiles,omitempty"`
	Status            string   `json:"status"`
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

type DomainPackActionResult struct {
	Pack    domainpacks.PackStatus `json:"pack"`
	Message string                 `json:"message"`
}

type DomainPackSkillSummary struct {
	PackName        string `json:"packName"`
	Ref             string `json:"ref"`
	Active          bool   `json:"active"`
	Enabled         bool   `json:"enabled"`
	Valid           bool   `json:"valid"`
	ValidationError string `json:"validationError,omitempty"`
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

type RouteCorrectionInput struct {
	ID                    string   `json:"id,omitempty"`
	ConversationID        string   `json:"conversationId,omitempty"`
	Pattern               string   `json:"pattern"`
	IntendedRouteCategory string   `json:"intendedRouteCategory"`
	IntendedTaskType      string   `json:"intendedTaskType,omitempty"`
	IntendedCapability    string   `json:"intendedCapability,omitempty"`
	RequiredTools         []string `json:"requiredTools,omitempty"`
	ForbiddenTools        []string `json:"forbiddenTools,omitempty"`
	Tags                  []string `json:"tags,omitempty"`
	OriginalPrompt        string   `json:"originalPrompt,omitempty"`
	CorrectionText        string   `json:"correctionText,omitempty"`
	ClarificationQuestion string   `json:"clarificationQuestion,omitempty"`
	Approved              bool     `json:"approved,omitempty"`
}

type RouteCorrectionIDInput struct {
	ID string `json:"id"`
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

type AskStreamEvent struct {
	Type    string            `json:"type"`
	Token   string            `json:"token,omitempty"`
	Message string            `json:"message,omitempty"`
	Data    map[string]string `json:"data,omitempty"`
	Result  *AskResult        `json:"result,omitempty"`
	Error   string            `json:"error,omitempty"`
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

type ConversationTitleInput struct {
	ConversationID string `json:"conversationId"`
	Title          string `json:"title"`
}

type ConversationStarInput struct {
	ConversationID string `json:"conversationId"`
	Starred        bool   `json:"starred"`
}

type ConversationDeleteInput struct {
	ConversationID string `json:"conversationId"`
}

type MessageEditInput struct {
	MessageID string `json:"messageId"`
	Content   string `json:"content"`
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

type DocumentPruneInput struct {
	Confirm bool `json:"confirm"`
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

type ConnectorResult struct {
	ConversationID string   `json:"conversationId"`
	Text           string   `json:"text"`
	Model          string   `json:"model"`
	Skill          string   `json:"skill,omitempty"`
	Sources        []string `json:"sources,omitempty"`
	SourceKind     string   `json:"sourceKind,omitempty"`
}

type GeneratedConnectorEnabledInput struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type GeneratedConnectorRunResult struct {
	Name      string         `json:"name"`
	Extension string         `json:"extension"`
	RunID     string         `json:"runId"`
	Status    string         `json:"status"`
	Output    map[string]any `json:"output,omitempty"`
	Error     string         `json:"error,omitempty"`
	Duration  int64          `json:"durationMs"`
}

func New(deps Dependencies) *Server {
	if strings.TrimSpace(deps.StaticDir) == "" {
		deps.StaticDir = filepath.Join("frontend", "dist")
	}
	if strings.TrimSpace(deps.Workspace) == "" {
		deps.Workspace = "."
	}
	if deps.NowTimeout == 0 {
		deps.NowTimeout = 5 * time.Minute
	}
	return &Server{deps: deps}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.HandleFunc("GET /api/ops/status", s.handleOpsStatus)
	mux.HandleFunc("GET /api/setup", s.handleSetupState)
	mux.HandleFunc("POST /api/setup", s.handleCompleteSetup)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("POST /api/settings", s.handleSaveSettings)
	mux.HandleFunc("GET /api/models", s.handleModels)
	mux.HandleFunc("GET /api/models/show", s.handleShowModel)
	mux.HandleFunc("POST /api/models/generate", s.handleGenerateModel)
	mux.HandleFunc("POST /api/models/role", s.handleSetModelRole)
	mux.HandleFunc("GET /api/model-profiles", s.handleModelProfiles)
	mux.HandleFunc("GET /api/model-profiles/show", s.handleShowModelProfile)
	mux.HandleFunc("POST /api/model-profiles/preview", s.handlePreviewModelProfile)
	mux.HandleFunc("POST /api/model-profiles/draft", s.handleDraftModelProfile)
	mux.HandleFunc("POST /api/model-profiles/apply", s.handleApplyModelProfile)
	mux.HandleFunc("POST /api/model-profiles", s.handleSaveModelProfile)
	mux.HandleFunc("GET /api/skills", s.handleSkills)
	mux.HandleFunc("POST /api/skills/validate", s.handleValidateSkill)
	mux.HandleFunc("POST /api/skills/enabled", s.handleSetSkillEnabled)
	mux.HandleFunc("POST /api/skills/create-from-session", s.handleCreateSkillFromSession)
	mux.HandleFunc("POST /api/skills/improve-from-session", s.handleImproveSkillFromSession)
	mux.HandleFunc("POST /api/skills/import", s.handleImportSkill)
	mux.HandleFunc("POST /api/skills/export", s.handleExportSkill)
	mux.HandleFunc("GET /api/domain-packs", s.handleDomainPacks)
	mux.HandleFunc("GET /api/domain-packs/templates", s.handleDomainPackTemplates)
	mux.HandleFunc("GET /api/domain-packs/skills", s.handleDomainPackSkills)
	mux.HandleFunc("POST /api/domain-packs/review", s.handleReviewDomainPack)
	mux.HandleFunc("POST /api/domain-packs/templates/review", s.handleReviewDomainPackTemplate)
	mux.HandleFunc("POST /api/domain-packs/install", s.handleInstallDomainPack)
	mux.HandleFunc("POST /api/domain-packs/install-template", s.handleInstallDomainPackTemplate)
	mux.HandleFunc("POST /api/domain-packs/enabled", s.handleSetDomainPackEnabled)
	mux.HandleFunc("POST /api/domain-packs/uninstall", s.handleUninstallDomainPack)
	mux.HandleFunc("GET /api/extensions", s.handleExtensions)
	mux.HandleFunc("GET /api/extensions/show", s.handleShowExtension)
	mux.HandleFunc("GET /api/extensions/inspect", s.handleInspectExtension)
	mux.HandleFunc("GET /api/extensions/review", s.handleReviewExtension)
	mux.HandleFunc("POST /api/capabilities/propose", s.handleProposeCapability)
	mux.HandleFunc("POST /api/capabilities/apply", s.handleApplyCapability)
	mux.HandleFunc("POST /api/capabilities/generate", s.handleGenerateCapability)
	mux.HandleFunc("POST /api/extensions/propose", s.handleProposeExtension)
	mux.HandleFunc("POST /api/extensions/generate", s.handleGenerateExtension)
	mux.HandleFunc("POST /api/extensions/validate", s.handleValidateExtension)
	mux.HandleFunc("POST /api/extensions/test", s.handleTestExtension)
	mux.HandleFunc("POST /api/extensions/register", s.handleRegisterExtension)
	mux.HandleFunc("POST /api/extensions/enabled", s.handleSetExtensionEnabled)
	mux.HandleFunc("POST /api/extensions/delete", s.handleDeleteExtension)
	mux.HandleFunc("POST /api/extensions/rollback", s.handleRollbackExtension)
	mux.HandleFunc("POST /api/extensions/run", s.handleRunExtension)
	mux.HandleFunc("GET /api/extensions/failures", s.handleExtensionFailures)
	mux.HandleFunc("GET /api/internet/status", s.handleInternetStatus)
	mux.HandleFunc("POST /api/internet/fetch", s.handleInternetFetch)
	mux.HandleFunc("POST /api/internet/head", s.handleInternetHead)
	mux.HandleFunc("POST /api/internet/search", s.handleInternetSearch)
	mux.HandleFunc("POST /api/internet/crawl", s.handleInternetCrawl)
	mux.HandleFunc("GET /api/internet/crawls", s.handleInternetCrawls)
	mux.HandleFunc("GET /api/internet/crawl-run", s.handleInternetCrawlRun)
	mux.HandleFunc("GET /api/internet/cache", s.handleInternetCache)
	mux.HandleFunc("GET /api/internet/requests", s.handleInternetRequests)
	mux.HandleFunc("GET /api/jobs", s.handleJobs)
	mux.HandleFunc("GET /api/jobs/runs", s.handleJobRuns)
	mux.HandleFunc("GET /api/jobs/status", s.handleJobStatus)
	mux.HandleFunc("POST /api/jobs/create", s.handleCreateJob)
	mux.HandleFunc("POST /api/jobs/input", s.handleUpdateJobInput)
	mux.HandleFunc("POST /api/jobs/enabled", s.handleSetJobEnabled)
	mux.HandleFunc("POST /api/jobs/run", s.handleRunJob)
	mux.HandleFunc("POST /api/jobs/tick", s.handleRunDueJobs)
	mux.HandleFunc("POST /api/jobs/archive", s.handleArchiveJob)
	mux.HandleFunc("GET /api/heartbeat/status", s.handleHeartbeatStatus)
	mux.HandleFunc("GET /api/policy", s.handlePolicyStatus)
	mux.HandleFunc("POST /api/policy/mode", s.handleSetPolicyMode)
	mux.HandleFunc("GET /api/learn/report", s.handleLearningReport)
	mux.HandleFunc("POST /api/learn/export", s.handleExportTrajectory)
	mux.HandleFunc("POST /api/learn/correction", s.handleSaveCorrection)
	mux.HandleFunc("GET /api/learn/route-corrections", s.handleRouteCorrections)
	mux.HandleFunc("POST /api/learn/route-correction", s.handleSaveRouteCorrection)
	mux.HandleFunc("POST /api/learn/route-correction/approve", s.handleApproveRouteCorrection)
	mux.HandleFunc("POST /api/learn/route-correction/disable", s.handleDisableRouteCorrection)
	mux.HandleFunc("GET /api/feedback/review", s.handleFeedbackReview)
	mux.HandleFunc("POST /api/feedback/review/promote", s.handlePromoteFeedbackRegression)
	mux.HandleFunc("POST /api/feedback/review/status", s.handleReviewFeedbackRegressionStatus)
	mux.HandleFunc("POST /api/feedback/review/test-draft", s.handleGenerateFeedbackRegressionTestDraft)
	mux.HandleFunc("POST /api/feedback/review/regression-approval", s.handleApproveFeedbackRegressionTestDraft)
	mux.HandleFunc("POST /api/feedback/review/source-patch", s.handleGenerateFeedbackRegressionSourcePatch)
	mux.HandleFunc("POST /api/feedback/review/source-patch/apply-plan", s.handleApproveFeedbackRegressionSourcePatchApplyPlan)
	mux.HandleFunc("POST /api/feedback/review/source-patch/source-write/plan", s.handlePlanFeedbackRegressionSourceWrite)
	mux.HandleFunc("POST /api/feedback/review/source-patch/source-write/apply", s.handleApplyFeedbackRegressionSourceWrite)
	mux.HandleFunc("GET /api/memory", s.handleMemory)
	mux.HandleFunc("GET /api/knowledge/status", s.handleKnowledgeStatus)
	mux.HandleFunc("POST /api/knowledge/enabled", s.handleSetKnowledgeEnabled)
	mux.HandleFunc("GET /api/knowledge/entities", s.handleKnowledgeEntities)
	mux.HandleFunc("POST /api/knowledge/proposals", s.handleDraftKnowledgeProposal)
	mux.HandleFunc("POST /api/knowledge/repair", s.handleRepairKnowledgeEvidence)
	mux.HandleFunc("GET /api/knowledge/review", s.handleKnowledgeReviewItems)
	mux.HandleFunc("POST /api/knowledge/review", s.handleKnowledgeReviewBatch)
	mux.HandleFunc("POST /api/knowledge/entities", s.handleSaveKnowledgeEntity)
	mux.HandleFunc("POST /api/knowledge/edges", s.handleSaveKnowledgeEdge)
	mux.HandleFunc("GET /api/conversations", s.handleConversations)
	mux.HandleFunc("GET /api/conversations/{id}", s.handleConversation)
	mux.HandleFunc("POST /api/conversations/rename", s.handleRenameConversation)
	mux.HandleFunc("POST /api/conversations/star", s.handleStarConversation)
	mux.HandleFunc("POST /api/conversations/delete", s.handleDeleteConversation)
	mux.HandleFunc("POST /api/messages/edit-user", s.handleEditUserMessage)
	mux.HandleFunc("GET /api/dev/frontend-version", s.handleFrontendVersion)
	mux.HandleFunc("GET /api/memories", s.handleMemories)
	mux.HandleFunc("POST /api/memories", s.handleWriteMemory)
	mux.HandleFunc("POST /api/memories/pin", s.handlePinMemory)
	mux.HandleFunc("POST /api/memories/delete", s.handleDeleteMemory)
	mux.HandleFunc("GET /api/tool-runs", s.handleToolRuns)
	mux.HandleFunc("GET /api/tools/catalog", s.handleToolCatalog)
	mux.HandleFunc("GET /api/notifications", s.handleNotifications)
	mux.HandleFunc("POST /api/notifications/read", s.handleMarkNotificationRead)
	mux.HandleFunc("POST /api/notifications/dismiss", s.handleDismissNotification)
	mux.HandleFunc("GET /api/replay", s.handleReplayTraces)
	mux.HandleFunc("GET /api/replay/explain", s.handleExplainReplayTrace)
	mux.HandleFunc("POST /api/permissions/decision", s.handlePermissionDecision)
	mux.HandleFunc("POST /api/files/plan-write", s.handlePlanFileWrite)
	mux.HandleFunc("POST /api/files/apply-write", s.handleApplyFileWrite)
	mux.HandleFunc("GET /api/documents", s.handleDocuments)
	mux.HandleFunc("GET /api/documents/inventory", s.handleDocumentInventory)
	mux.HandleFunc("POST /api/documents/prune-missing", s.handlePruneMissingDocuments)
	mux.HandleFunc("GET /api/documents/suggestions", s.handleDocumentSuggestions)
	mux.HandleFunc("POST /api/documents/ingest", s.handleIngest)
	mux.HandleFunc("POST /api/documents/upload", s.handleUploadDocument)
	mux.HandleFunc("POST /api/documents/embeddings/index", s.handleIndexEmbeddings)
	mux.HandleFunc("GET /api/workspace/grants", s.handleWorkspaceGrants)
	mux.HandleFunc("POST /api/workspace/grants", s.handleGrantWorkspace)
	mux.HandleFunc("POST /api/workspace/grants/revoke", s.handleRevokeWorkspaceGrant)
	mux.HandleFunc("POST /api/chat/attachments", s.handleUploadChatAttachment)
	mux.HandleFunc("POST /api/chat", s.handleChat)
	mux.HandleFunc("POST /api/ask", s.handleAsk)
	mux.HandleFunc("POST /api/ask/stream", s.handleAskStream)
	mux.HandleFunc("POST /api/ask/stop", s.handleStopAsk)
	mux.HandleFunc("GET /api/connectors", s.handleConnectors)
	mux.HandleFunc("GET /api/connectors/registry", s.handleConnectorRegistry)
	mux.HandleFunc("GET /api/connectors/generated", s.handleGeneratedConnectors)
	mux.HandleFunc("POST /api/connectors/generated/install", s.handleInstallGeneratedConnector)
	mux.HandleFunc("POST /api/connectors/generated/enabled", s.handleSetGeneratedConnectorEnabled)
	mux.HandleFunc("POST /connectors/generated/v1/{name}/run", s.handleGeneratedConnectorRun)
	mux.HandleFunc("GET /connectors/local/v1/health", s.handleLocalConnectorHealth)
	mux.HandleFunc("GET /connectors/local/v1/status", s.handleLocalConnectorHealth)
	mux.HandleFunc("GET /connectors/local/v1/tools", s.handleLocalConnectorTools)
	mux.HandleFunc("GET /connectors/local/v1/memory/search", s.handleLocalConnectorMemorySearch)
	mux.HandleFunc("POST /connectors/local/v1/chat", s.handleLocalConnectorChat)
	mux.HandleFunc("POST /connectors/local/v1/ask", s.handleLocalConnectorAsk)
	mux.HandleFunc("GET /connectors/adapters/v1/{adapter}/status", s.handleAdapterConnectorStatus)
	mux.HandleFunc("POST /connectors/adapters/v1/{adapter}/chat", s.handleAdapterConnectorChat)
	mux.HandleFunc("POST /connectors/adapters/v1/{adapter}/ask", s.handleAdapterConnectorAsk)
	mux.HandleFunc("/", s.handleStatic)
	return localOnly(mux)
}

func (s *Server) ListenAndServe(ctx context.Context, addr string) error {
	if strings.TrimSpace(addr) == "" {
		addr = DefaultAddr
	}
	if err := validateLoopbackAddr(addr); err != nil {
		return err
	}
	s.startFrontendWatcher(ctx)
	httpServer := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()
	err := httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	report := diagnostics.Run(r.Context(), s.deps.Config, s.deps.Profile, s.deps.Memory, s.deps.Runtime)
	statuses := make([]ModelStatus, 0, len(report.ModelStatuses))
	for _, model := range report.ModelStatuses {
		configured := s.deps.Config.Models[model.Role]
		profileValid, profileStatus := s.modelProfileBindingStatus(configured.Profile)
		statuses = append(statuses, ModelStatus{
			Role:          model.Role,
			Name:          model.Name,
			Installed:     model.Installed,
			Profile:       strings.TrimSpace(configured.Profile),
			ProfileValid:  profileValid,
			ProfileStatus: profileStatus,
		})
	}
	writeJSON(w, Status{
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
		SkillCount:     len(s.deps.Skills.Skills),
		Workspace:      s.deps.Workspace,
	})
}

func (s *Server) handleOpsStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.BuildOpsStatus(r.Context(), boolParam(r, "includeRelease", false), intParam(r, "limit", 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, status)
}

func (s *Server) BuildOpsStatus(ctx context.Context, includeRelease bool, limit int) (OpsStatus, error) {
	return s.buildOpsStatus(ctx, includeRelease, limit)
}

func (s *Server) buildOpsStatus(ctx context.Context, includeRelease bool, limit int) (OpsStatus, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	status := OpsStatus{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Overall:     heartbeat.StatusHealthy,
		Stages:      []OpsStage{},
	}
	var qaFailures []learning.QAReplayFailure

	report, err := s.heartbeatReport(ctx)
	if err != nil {
		status.Stages = append(status.Stages, OpsStage{Name: "heartbeat", Status: heartbeat.StatusBroken, Details: err.Error()})
	} else {
		status.Heartbeat = report
		status.Stages = append(status.Stages, OpsStage{Name: "heartbeat", Status: report.Overall, Details: fmt.Sprintf("%d checks", len(report.Checks))})
	}

	if store, err := s.schedulerStore(ctx); err == nil {
		defer store.Close()
		if schedulerStatus, err := store.Status(ctx, s.deps.Config.Scheduler.Enabled, s.schedulerMaxParallel()); err == nil {
			status.Scheduler = schedulerStatus
			stageStatus := heartbeat.StatusHealthy
			if !schedulerStatus.Enabled {
				stageStatus = heartbeat.StatusDisabled
			} else if schedulerStatus.LastRunStatus == scheduler.StatusFailed || schedulerStatus.LastRunStatus == scheduler.StatusTimeout {
				stageStatus = heartbeat.StatusBroken
			}
			status.Stages = append(status.Stages, OpsStage{Name: "jobs", Status: stageStatus, Details: fmt.Sprintf("%d jobs, %d enabled", schedulerStatus.TotalJobs, schedulerStatus.EnabledJobs)})
		} else {
			status.Stages = append(status.Stages, OpsStage{Name: "jobs", Status: heartbeat.StatusBroken, Details: err.Error()})
		}
		if runs, err := store.LastRuns(ctx, limit); err == nil {
			status.RecentJobRuns = runs
		}
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "jobs", Status: heartbeat.StatusBroken, Details: err.Error()})
	}

	if store, err := s.notificationStore(); err == nil {
		items, err := store.List(limit, false)
		if err == nil {
			status.Notifications = items
			status.Stages = append(status.Stages, OpsStage{Name: "notifications", Status: notificationStageStatus(items), Details: fmt.Sprintf("%d recent", len(items))})
		} else {
			status.Stages = append(status.Stages, OpsStage{Name: "notifications", Status: heartbeat.StatusBroken, Details: err.Error()})
		}
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "notifications", Status: heartbeat.StatusNeedsConfig, Details: err.Error()})
	}

	latestEval := s.latestQAEvalSummary()
	status.LatestEval = latestEval
	status.Stages = append(status.Stages, OpsStage{Name: "eval", Status: evalStageStatus(latestEval), Details: evalStageDetails(latestEval)})

	review, err := s.qaReview(ctx, limit)
	if err != nil {
		status.Stages = append(status.Stages, OpsStage{Name: "qa_review", Status: heartbeat.StatusBroken, Details: err.Error()})
	} else {
		status.QAReview = review.Summary
		qaFailures = review.ReplayFailures
		status.Stages = append(status.Stages, OpsStage{Name: "qa_review", Status: qaReviewStageStatus(review.Summary), Details: qaReviewStageDetails(review.Summary)})
	}

	if traces, err := s.replayTraceStore().List(); err == nil {
		status.RecentReplay = limitLatestReplayTraces(traces, limit)
		status.Stages = append(status.Stages, OpsStage{Name: "replay", Status: replayStageStatus(status.RecentReplay), Details: fmt.Sprintf("%d recent traces", len(status.RecentReplay))})
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "replay", Status: heartbeat.StatusBroken, Details: err.Error()})
	}

	if s.deps.Memory != nil {
		runs, err := s.deps.Memory.ListToolRunsFiltered(ctx, memory.ToolRunFilter{Limit: limit})
		if err == nil {
			status.RecentToolRuns = toolRunResults(runs)
			status.Stages = append(status.Stages, OpsStage{Name: "audit", Status: toolRunStageStatus(runs), Details: fmt.Sprintf("%d recent tool runs", len(runs))})
		} else {
			status.Stages = append(status.Stages, OpsStage{Name: "audit", Status: heartbeat.StatusBroken, Details: err.Error()})
		}
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "audit", Status: heartbeat.StatusNeedsConfig, Details: "memory store is not configured"})
	}

	if s.deps.Profile != nil {
		records, err := safety.RecentPolicyAudit(s.deps.Profile.Logs, limit)
		if err == nil {
			status.PolicyAudit = records
			status.Stages = append(status.Stages, OpsStage{Name: "policy_audit", Status: policyAuditStageStatus(records), Details: fmt.Sprintf("%d recent policy decisions", len(records))})
		} else {
			status.Stages = append(status.Stages, OpsStage{Name: "policy_audit", Status: heartbeat.StatusWarning, Details: err.Error()})
		}
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "policy_audit", Status: heartbeat.StatusNeedsConfig, Details: "profile logs are not configured"})
	}

	if records, err := s.extensionStore().RecentAudit(limit); err == nil {
		status.ExtensionAudit = records
		status.Stages = append(status.Stages, OpsStage{Name: "extensions", Status: extensionAuditStageStatus(records), Details: fmt.Sprintf("%d recent extension audit records", len(records))})
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "extensions", Status: heartbeat.StatusWarning, Details: err.Error()})
	}

	connectorStatuses := connectors.List(s.deps.Config.Connectors)
	if entries, err := s.generatedConnectorStore().Registry(s.deps.Config.Connectors); err == nil {
		connectorStatuses = make([]connectors.Status, 0, len(entries))
		for _, entry := range entries {
			connectorStatuses = append(connectorStatuses, entry.Status)
		}
	}
	connectorSummary := connectorOpsSummary(connectorStatuses)
	status.Connectors = connectorSummary
	status.Stages = append(status.Stages, OpsStage{Name: "connectors", Status: connectorSummary.Status, Details: connectorOpsStageDetails(connectorSummary)})

	if knowledgeStatus, err := s.knowledgeStatus(ctx); err == nil {
		status.Knowledge = knowledgeStatus
		status.Stages = append(status.Stages, OpsStage{Name: "knowledge", Status: knowledgeOpsStageStatus(knowledgeStatus), Details: knowledgeStatus.Message})
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "knowledge", Status: heartbeat.StatusBroken, Details: err.Error()})
	}

	if profileSummary, err := modelProfileOpsSummary(s.deps.Config, s.modelProfileStore()); err == nil {
		status.ModelProfiles = profileSummary
		status.Stages = append(status.Stages, OpsStage{Name: "model_profiles", Status: profileSummary.Status, Details: modelProfileOpsStageDetails(profileSummary)})
	} else {
		status.Stages = append(status.Stages, OpsStage{Name: "model_profiles", Status: heartbeat.StatusWarning, Details: err.Error()})
	}

	if includeRelease {
		report := release.Run(ctx, release.Input{
			Config:  s.deps.Config,
			Profile: s.deps.Profile,
			Memory:  s.deps.Memory,
			Runtime: s.deps.Runtime,
			Skills:  s.deps.Skills,
		})
		status.Release = &OpsReleaseSummary{Version: report.Version, Ready: report.Ready, Checks: report.Checks}
		stageStatus := heartbeat.StatusHealthy
		if !report.Ready {
			stageStatus = heartbeat.StatusBroken
		} else if releaseHasWarnings(report.Checks) {
			stageStatus = heartbeat.StatusWarning
		}
		status.Stages = append(status.Stages, OpsStage{Name: "release", Status: stageStatus, Details: fmt.Sprintf("%d checks", len(report.Checks))})
	}

	status.Timeline = buildOpsTimeline(status, qaFailures, limit)
	status.Overall = rollupOpsStatus(status.Stages)
	return status, nil
}

func (s *Server) handleSetupState(w http.ResponseWriter, r *http.Request) {
	state := s.setupState(r.Context())
	writeJSON(w, state)
}

func (s *Server) handleCompleteSetup(w http.ResponseWriter, r *http.Request) {
	var input SetupInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	state, err := s.completeSetup(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, state)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, settingsView(s.deps.Config))
}

func (s *Server) handleSaveSettings(w http.ResponseWriter, r *http.Request) {
	input, err := readSettingsInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := applySettings(s.deps.Config, input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Write(s.deps.Config.Path, s.deps.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.deps.Router = models.NewRouter(s.deps.Config)
	runtime, err := modelruntime.New(selectedModel(s.deps.Config))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.deps.Runtime = runtime
	if s.deps.Config.CloudFallback.Enabled {
		s.deps.Cloud = cloud.New(s.deps.Config.CloudFallback)
	} else {
		s.deps.Cloud = nil
	}
	writeJSON(w, settingsView(s.deps.Config))
}

func readSettingsInput(r *http.Request) (SettingsInput, error) {
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return SettingsInput{}, err
	}
	var input SettingsInput
	inputErr := strictDecodeJSON(body, &input)
	if inputErr == nil {
		return input, nil
	}
	var view SettingsView
	if err := strictDecodeJSON(body, &view); err == nil {
		return settingsInputFromView(view), nil
	}
	if input, ok, err := flexibleSettingsInput(body); ok || err != nil {
		return input, err
	}
	return SettingsInput{}, inputErr
}

func strictDecodeJSON(body []byte, out any) error {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func flexibleSettingsInput(body []byte) (SettingsInput, bool, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return SettingsInput{}, false, err
	}
	if len(raw) == 0 || !hasAnySettingsInputKey(raw) {
		return SettingsInput{}, false, nil
	}
	var input SettingsInput
	var err error
	if input.LowMemoryMode, err = rawBool(raw, "lowMemoryMode", "low_memory_mode"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.MaxContextTokens, err = rawInt(raw, "maxContextTokens", "max_context_tokens"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.ResponseMode, err = rawString(raw, "responseMode", "response_mode"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.ShowThinkingTrace, err = rawBool(raw, "showThinkingTrace", "show_thinking_trace"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.UITheme, err = rawString(raw, "uiTheme", "ui_theme"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.RAGEnabled, err = rawBool(raw, "ragEnabled", "rag_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.EmbeddingsEnabled, err = rawBool(raw, "embeddingsEnabled", "embeddings_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.EmbeddingModel, err = rawString(raw, "embeddingModel", "embedding_model"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.CloudFallbackEnabled, err = rawBool(raw, "cloudFallbackEnabled", "cloud_fallback_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.CloudFallbackBaseURL, err = rawString(raw, "cloudFallbackBaseUrl", "cloudFallbackBaseURL", "cloud_fallback_base_url"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.CloudFallbackModel, err = rawString(raw, "cloudFallbackModel", "cloud_fallback_model"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.CloudFallbackKeyEnv, err = rawString(raw, "cloudFallbackKeyEnv", "cloud_fallback_key_env"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.ConnectorEnabled, err = rawBool(raw, "connectorEnabled", "connector_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.ConnectorTokenEnv, err = rawString(raw, "connectorTokenEnv", "connector_token_env"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.MCPConnectorEnabled, err = rawBool(raw, "mcpConnectorEnabled", "mcp_connector_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.SlackConnectorEnabled, err = rawBool(raw, "slackConnectorEnabled", "slack_connector_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.SlackConnectorTokenEnv, err = rawString(raw, "slackConnectorTokenEnv", "slack_connector_token_env"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.DiscordConnectorEnabled, err = rawBool(raw, "discordConnectorEnabled", "discord_connector_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.DiscordConnectorTokenEnv, err = rawString(raw, "discordConnectorTokenEnv", "discord_connector_token_env"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.TelegramConnectorEnabled, err = rawBool(raw, "telegramConnectorEnabled", "telegram_connector_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.TelegramConnectorTokenEnv, err = rawString(raw, "telegramConnectorTokenEnv", "telegram_connector_token_env"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.EmailConnectorEnabled, err = rawBool(raw, "emailConnectorEnabled", "email_connector_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.EmailConnectorTokenEnv, err = rawString(raw, "emailConnectorTokenEnv", "email_connector_token_env"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.InternetEnabled, err = rawBool(raw, "internetEnabled", "internet_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.InternetSearchEnabled, err = rawBool(raw, "internetSearchEnabled", "internet_search_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.InternetSearchProvider, err = rawString(raw, "internetSearchProvider", "internet_search_provider"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.InternetSearchEndpoint, err = rawString(raw, "internetSearchEndpoint", "internet_search_endpoint"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.InternetSearchAPIKeyEnv, err = rawString(raw, "internetSearchApiKeyEnv", "internetSearchAPIKeyEnv", "internet_search_api_key_env"); err != nil {
		return SettingsInput{}, true, err
	}
	if input.KnowledgeInfluenceEnabled, err = rawBool(raw, "knowledgeInfluenceEnabled", "knowledge_influence_enabled"); err != nil {
		return SettingsInput{}, true, err
	}
	return input, true, nil
}

func hasAnySettingsInputKey(raw map[string]json.RawMessage) bool {
	keys := []string{
		"lowMemoryMode", "low_memory_mode", "maxContextTokens", "max_context_tokens",
		"responseMode", "response_mode", "uiTheme", "ui_theme", "ragEnabled",
		"rag_enabled", "internetEnabled", "internet_enabled",
	}
	for _, key := range keys {
		if _, ok := raw[key]; ok {
			return true
		}
	}
	return false
}

func rawString(raw map[string]json.RawMessage, keys ...string) (string, error) {
	value, ok := rawValue(raw, keys...)
	if !ok || len(value) == 0 || string(value) == "null" {
		return "", nil
	}
	var out string
	if err := json.Unmarshal(value, &out); err == nil {
		return out, nil
	}
	return "", fmt.Errorf("%s must be a string", keys[0])
}

func rawBool(raw map[string]json.RawMessage, keys ...string) (bool, error) {
	value, ok := rawValue(raw, keys...)
	if !ok || len(value) == 0 || string(value) == "null" {
		return false, nil
	}
	var out bool
	if err := json.Unmarshal(value, &out); err == nil {
		return out, nil
	}
	var encoded string
	if err := json.Unmarshal(value, &encoded); err == nil {
		parsed, parseErr := strconv.ParseBool(strings.TrimSpace(encoded))
		if parseErr != nil {
			return false, fmt.Errorf("%s must be a boolean", keys[0])
		}
		return parsed, nil
	}
	return false, fmt.Errorf("%s must be a boolean", keys[0])
}

func rawInt(raw map[string]json.RawMessage, keys ...string) (int, error) {
	value, ok := rawValue(raw, keys...)
	if !ok || len(value) == 0 || string(value) == "null" {
		return 0, nil
	}
	var out int
	if err := json.Unmarshal(value, &out); err == nil {
		return out, nil
	}
	var encoded string
	if err := json.Unmarshal(value, &encoded); err == nil {
		parsed, parseErr := strconv.Atoi(strings.TrimSpace(encoded))
		if parseErr != nil {
			return 0, fmt.Errorf("%s must be a number", keys[0])
		}
		return parsed, nil
	}
	return 0, fmt.Errorf("%s must be a number", keys[0])
}

func rawValue(raw map[string]json.RawMessage, keys ...string) (json.RawMessage, bool) {
	for _, key := range keys {
		if value, ok := raw[key]; ok {
			return value, true
		}
	}
	return nil, false
}

func settingsInputFromView(view SettingsView) SettingsInput {
	return SettingsInput{
		LowMemoryMode:             view.LowMemoryMode,
		MaxContextTokens:          view.MaxContextTokens,
		ResponseMode:              view.ResponseMode,
		ShowThinkingTrace:         view.ShowThinkingTrace,
		UITheme:                   view.UI.Theme,
		RAGEnabled:                view.RAG.Enabled,
		EmbeddingsEnabled:         view.RAG.Embeddings.Enabled,
		EmbeddingModel:            view.RAG.Embeddings.Model,
		CloudFallbackEnabled:      view.CloudFallback.Enabled,
		CloudFallbackBaseURL:      view.CloudFallback.BaseURL,
		CloudFallbackModel:        view.CloudFallback.Name,
		CloudFallbackKeyEnv:       view.CloudFallback.APIKeyEnv,
		ConnectorEnabled:          view.Connectors.LocalAPI.Enabled,
		ConnectorTokenEnv:         view.Connectors.LocalAPI.TokenEnv,
		MCPConnectorEnabled:       view.Connectors.MCPServer.Enabled,
		SlackConnectorEnabled:     view.Connectors.Slack.Enabled,
		SlackConnectorTokenEnv:    view.Connectors.Slack.TokenEnv,
		DiscordConnectorEnabled:   view.Connectors.Discord.Enabled,
		DiscordConnectorTokenEnv:  view.Connectors.Discord.TokenEnv,
		TelegramConnectorEnabled:  view.Connectors.Telegram.Enabled,
		TelegramConnectorTokenEnv: view.Connectors.Telegram.TokenEnv,
		EmailConnectorEnabled:     view.Connectors.Email.Enabled,
		EmailConnectorTokenEnv:    view.Connectors.Email.TokenEnv,
		InternetEnabled:           view.Internet.Enabled,
		InternetSearchEnabled:     view.Internet.Search.Enabled,
		InternetSearchProvider:    view.Internet.Search.Provider,
		InternetSearchEndpoint:    view.Internet.Search.Endpoint,
		InternetSearchAPIKeyEnv:   view.Internet.Search.APIKeyEnv,
		KnowledgeInfluenceEnabled: view.Knowledge.InfluenceEnabled,
	}
}

func (s *Server) handleConnectors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, connectors.List(s.deps.Config.Connectors))
}

func (s *Server) handleConnectorRegistry(w http.ResponseWriter, r *http.Request) {
	entries, err := s.generatedConnectorStore().Registry(s.deps.Config.Connectors)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, entries)
}

func (s *Server) handleGeneratedConnectors(w http.ResponseWriter, r *http.Request) {
	items, err := s.generatedConnectorStore().List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, items)
}

func (s *Server) handleInstallGeneratedConnector(w http.ResponseWriter, r *http.Request) {
	var input connectors.GeneratedInstallInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.generatedConnectorStore().Install(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleSetGeneratedConnectorEnabled(w http.ResponseWriter, r *http.Request) {
	var input GeneratedConnectorEnabledInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.generatedConnectorStore().SetEnabled(input.Name, input.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleGeneratedConnectorRun(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	entry, status, ok := s.generatedConnectorAllowed(w, r, name, "run")
	if !ok {
		return
	}
	input, err := readGeneratedConnectorInput(w, r, status)
	if err != nil {
		_ = s.generatedConnectorStore().RecordRun(name, "blocked", "invalid request body")
		writeError(w, http.StatusBadRequest, err)
		return
	}
	extensionName := strings.TrimSpace(strings.TrimPrefix(entry.Manifest.Endpoint, "extension:"))
	internetService := internet.New(s.deps.Config.Internet, s.deps.Profile.Root, s.deps.Profile.Logs)
	internetService.PolicyMode = safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)
	result, err := s.extensionStore().Run(r.Context(), extensionName, extensions.RunOptions{
		Input:             input,
		MaxRuntimeSeconds: s.deps.Config.Extensions.MaxRuntimeSeconds,
		MaxOutputBytes:    65536,
		Internet:          internetService,
	})
	runResult := GeneratedConnectorRunResult{
		Name:      entry.Name,
		Extension: extensionName,
		RunID:     result.RunID,
		Status:    result.Status,
		Output:    result.Output,
		Error:     result.Error,
		Duration:  result.DurationMS,
	}
	if err != nil {
		_ = s.generatedConnectorStore().RecordRun(name, "failed", err.Error())
		writeError(w, http.StatusBadGateway, err)
		return
	}
	_ = s.generatedConnectorStore().RecordRun(name, "ok", "generated connector executed through extension runner")
	writeJSON(w, runResult)
}

func (s *Server) handleModels(w http.ResponseWriter, r *http.Request) {
	models, err := s.deps.Runtime.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, localInstalledModels(models))
}

func (s *Server) handleShowModel(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("model name is required"))
		return
	}
	detailer, ok := s.deps.Runtime.(models.ModelDetailRuntime)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("configured runtime does not support model details"))
		return
	}
	details, err := detailer.ShowModel(r.Context(), name)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, details)
}

func (s *Server) handleGenerateModel(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name   string `json:"name"`
		Prompt string `json:"prompt"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(input.Name)
	prompt := strings.TrimSpace(input.Prompt)
	if name == "" || prompt == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("model name and prompt are required"))
		return
	}
	generator, ok := s.deps.Runtime.(models.GenerateRuntime)
	if !ok {
		writeError(w, http.StatusNotImplemented, fmt.Errorf("configured runtime does not support direct generation"))
		return
	}
	var output strings.Builder
	if err := generator.GenerateStream(r.Context(), models.GenerateRequest{
		Model:       name,
		Prompt:      prompt,
		Temperature: modelTemperature(s.deps.Config, name),
	}, func(event models.GenerateEvent) error {
		if event.Token != "" {
			_, err := output.WriteString(event.Token)
			return err
		}
		return nil
	}); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, ModelGenerateResult{Text: output.String(), Model: name})
}

func (s *Server) handleSetModelRole(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Role     string `json:"role"`
		Name     string `json:"name"`
		Provider string `json:"provider"`
		BaseURL  string `json:"baseUrl"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.setModelRole(strings.TrimSpace(input.Role), strings.TrimSpace(input.Name), strings.TrimSpace(input.Provider), strings.TrimSpace(input.BaseURL)); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleModelProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.modelProfileStore().List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, profiles)
}

func (s *Server) handleShowModelProfile(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("model profile name is required"))
		return
	}
	store := s.modelProfileStore()
	profile, err := store.Load(name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := modelProfileResult(profile, "", "loaded")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	enrichModelProfileResult(store, &result)
	writeJSON(w, result)
}

func (s *Server) handlePreviewModelProfile(w http.ResponseWriter, r *http.Request) {
	var input ModelProfileInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	profile := modelProfileFromInput(input)
	result, err := modelProfileResult(profile, "", "preview")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	enrichModelProfileResult(s.modelProfileStore(), &result)
	writeJSON(w, result)
}

func (s *Server) handleDraftModelProfile(w http.ResponseWriter, r *http.Request) {
	var input ModelProfileDraftInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	profile, report, err := learning.DraftModelProfileFromConversation(r.Context(), s.deps.Memory, learning.ModelProfileDraftInput{
		ConversationID: input.ConversationID,
		Name:           input.Name,
		BaseModel:      input.BaseModel,
		Temperature:    input.Temperature,
		NumCtx:         input.NumCtx,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := modelProfileResult(profile, "", "drafted")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	enrichModelProfileResult(s.modelProfileStore(), &result)
	result.DraftReport = modelProfileDraftReport(report)
	writeJSON(w, result)
}

func (s *Server) handleSaveModelProfile(w http.ResponseWriter, r *http.Request) {
	var input ModelProfileInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	store := s.modelProfileStore()
	profile := modelProfileFromInput(input)
	comparison, _ := store.CompareCurrent(profile)
	path, err := store.Save(profile)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := modelProfileResult(profile, path, "saved")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result.Comparison = comparison
	enrichModelProfileResult(store, &result)
	writeJSON(w, result)
}

func (s *Server) handleApplyModelProfile(w http.ResponseWriter, r *http.Request) {
	var input ModelProfileApplyInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	role := normalizeModelRole(input.Role)
	if role == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("model role is required"))
		return
	}
	profile, path, err := s.applyModelProfile(role, strings.TrimSpace(input.Name))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := modelProfileResult(profile, path, "applied")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	enrichModelProfileResult(s.modelProfileStore(), &result)
	result.Applied = true
	result.AppliedRole = role
	writeJSON(w, result)
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	items := s.deps.Skills.Statuses()
	result := make([]SkillSummary, 0, len(items))
	for _, skill := range items {
		result = append(result, skillSummary(skill))
	}
	writeJSON(w, result)
}

func (s *Server) handleValidateSkill(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	skill, ok := s.deps.Skills.Get(strings.TrimSpace(input.Name))
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("skill not found: %s", input.Name))
		return
	}
	if err := skills.ValidateWithOptions(skill, skills.ValidationOptions{AvailableTools: availableSkillTools()}); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, SkillActionResult{Skill: skillSummaryFromSkill(skill), Message: "valid"})
}

func (s *Server) handleSetSkillEnabled(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	skill, ok := s.deps.Skills.Get(strings.TrimSpace(input.Name))
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("skill not found: %s", input.Name))
		return
	}
	updated, err := skills.SetEnabled(s.deps.Profile.Skills, skill, input.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	message := "enabled"
	if !input.Enabled {
		message = "disabled"
	}
	writeJSON(w, SkillActionResult{Skill: skillSummaryFromSkill(updated), Message: message})
}

func (s *Server) handleCreateSkillFromSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ConversationID string `json:"conversationId"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	created, err := s.createSkillFromSession(r.Context(), strings.TrimSpace(input.ConversationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, SkillActionResult{Skill: skillSummaryFromSkill(created), Message: "created"})
}

func (s *Server) handleImproveSkillFromSession(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name           string `json:"name"`
		ConversationID string `json:"conversationId"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	improved, err := s.improveSkillFromSession(r.Context(), strings.TrimSpace(input.Name), strings.TrimSpace(input.ConversationID))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, SkillActionResult{Skill: skillSummaryFromSkill(improved), Message: "improved"})
}

func (s *Server) handleImportSkill(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	imported, err := skills.Import(s.deps.Profile.Skills, strings.TrimSpace(input.Path))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, SkillActionResult{Skill: skillSummaryFromSkill(imported), Message: "imported"})
}

func (s *Server) handleExportSkill(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	skill, ok := s.deps.Skills.Get(strings.TrimSpace(input.Name))
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("skill not found: %s", input.Name))
		return
	}
	exportedPath, err := skills.Export(skill, strings.TrimSpace(input.Path))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, SkillActionResult{Skill: skillSummaryFromSkill(skill), Message: "exported", Path: exportedPath})
}

func (s *Server) handleDomainPacks(w http.ResponseWriter, r *http.Request) {
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	statuses, err := domainpacks.List(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, statuses)
}

func (s *Server) handleDomainPackTemplates(w http.ResponseWriter, r *http.Request) {
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	statuses, err := domainpacks.List(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	catalog, err := builtInDomainPackTemplateCatalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, workflows.DomainPackTemplateSummaries(catalog, statuses))
}

func (s *Server) handleDomainPackSkills(w http.ResponseWriter, r *http.Request) {
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	statuses, err := domainpacks.List(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, domainPackSkillSummaries(statuses))
}

func (s *Server) handleReviewDomainPack(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	review, err := domainpacks.ReviewInstalled(root, input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, review)
}

func (s *Server) handleReviewDomainPackTemplate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         string `json:"name"`
		TemplateName string `json:"templateName"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(firstNonEmptyString(input.Name, input.TemplateName))
	if name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("domain pack template name is required"))
		return
	}
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	statuses, err := domainpacks.List(root)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	catalog, err := builtInDomainPackTemplateCatalog()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	template, ok := catalog.Get(name)
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("domain pack template not found: %s", name))
		return
	}
	status := domainPackStatusByName(statuses, template.Name)
	review, err := domainpacks.ReviewDir(template.Path, workflows.SourceDomainPackTemplate, status)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, review)
}

func (s *Server) handleInstallDomainPack(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path      string `json:"path"`
		SourceDir string `json:"sourceDir"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	source := strings.TrimSpace(input.Path)
	if source == "" {
		source = strings.TrimSpace(input.SourceDir)
	}
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status, err := domainpacks.Install(root, source)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, DomainPackActionResult{Pack: status, Message: "installed"})
}

func (s *Server) handleInstallDomainPackTemplate(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name         string `json:"name"`
		TemplateName string `json:"templateName"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	name := strings.TrimSpace(firstNonEmptyString(input.Name, input.TemplateName))
	if name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("domain pack template name is required"))
		return
	}
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status, err := s.installBuiltInDomainPackTemplate(root, name, "")
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, DomainPackActionResult{Pack: status, Message: "installed"})
}

func (s *Server) handleSetDomainPackEnabled(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status, err := domainpacks.SetEnabled(root, input.Name, input.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	message := "enabled"
	if !input.Enabled {
		message = "disabled"
	}
	writeJSON(w, DomainPackActionResult{Pack: status, Message: message})
}

func (s *Server) handleUninstallDomainPack(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	root, err := s.domainPackRoot()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status, err := domainpacks.Uninstall(root, input.Name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.reloadSkills(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, DomainPackActionResult{Pack: status, Message: "uninstalled"})
}

func (s *Server) handleExtensions(w http.ResponseWriter, r *http.Request) {
	items, err := s.extensionStore().List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, items)
}

func (s *Server) handleShowExtension(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("extension name is required"))
		return
	}
	detail, err := s.extensionStore().Show(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, detail)
}

func (s *Server) handleInspectExtension(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("extension name is required"))
		return
	}
	inspection, err := s.extensionStore().InspectPackage(name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, inspection)
}

func (s *Server) handleReviewExtension(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("extension name is required"))
		return
	}
	review, err := s.extensionStore().Review(name)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, review)
}

func (s *Server) handleProposeExtension(w http.ResponseWriter, r *http.Request) {
	var input ExtensionProposalInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().Propose(input.Request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleProposeCapability(w http.ResponseWriter, r *http.Request) {
	var input ExtensionProposalInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := agent.NewCapabilityGapRouter(s.deps.Config, s.extensionStore()).ProposeRequest(input.Request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleApplyCapability(w http.ResponseWriter, r *http.Request) {
	var input CapabilityApplyInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.applyCapabilityHandoff(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleGenerateCapability(w http.ResponseWriter, r *http.Request) {
	var input CapabilityGenerateInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	store := s.extensionStore()
	draftModel := config.ModelConfig{}
	if input.SynthesizeDraft {
		var err error
		draftModel, err = agent.SelectCapabilityDraftModel(s.deps.Config)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	result, err := agent.NewCapabilityGapRouter(s.deps.Config, store).Generate(r.Context(), store, agent.CapabilityGenerationInput{
		Request:             strings.TrimSpace(input.Request),
		Approved:            input.Approved,
		Name:                strings.TrimSpace(input.Name),
		AllowedDomains:      input.AllowedDomains,
		MaxRuntimeSeconds:   s.deps.Config.Extensions.MaxRuntimeSeconds,
		RunAfterGenerate:    input.RunAfterGenerate,
		RunInput:            input.RunInput,
		Draft:               input.Draft,
		SynthesizeDraft:     input.SynthesizeDraft,
		DraftRepairAttempts: input.DraftRepairAttempts,
		DraftModel:          draftModel,
		DraftRuntimeFactory: s.deps.RuntimeFactory,
		RunOptions: extensions.RunOptions{
			Input:             input.RunInput,
			MaxRuntimeSeconds: s.deps.Config.Extensions.MaxRuntimeSeconds,
			Internet:          s.internetService(),
		},
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if input.ScheduleAfterGenerate {
		if err := s.attachGeneratedCapabilityJob(r.Context(), &result, input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if s.deps.Memory != nil {
		_, _ = s.deps.Memory.SaveMemory(r.Context(), memory.Memory{
			Kind:       "capability_generation",
			Content:    result.Generation.SkillCandidate,
			Importance: 4,
			Source:     "capability_generator",
		})
	}
	writeJSON(w, result)
}

func (s *Server) applyCapabilityHandoff(input CapabilityApplyInput) (CapabilityApplyResult, error) {
	if !input.Approved {
		return CapabilityApplyResult{}, fmt.Errorf("approved flag is required before applying a capability handoff")
	}
	if kind := strings.TrimSpace(input.Kind); kind != "" && kind != "domain_pack" {
		return CapabilityApplyResult{}, fmt.Errorf("capability kind %s is not supported by this apply flow", kind)
	}
	root, err := s.domainPackRoot()
	if err != nil {
		return CapabilityApplyResult{}, err
	}
	source := strings.TrimSpace(input.PackSource)
	name := strings.TrimSpace(firstNonEmptyString(input.ExistingCapability, input.Name))
	var status domainpacks.PackStatus
	action := ""
	switch source {
	case "domain_pack_template":
		status, err = s.installBuiltInDomainPackTemplate(root, name, strings.TrimSpace(input.PackDir))
		action = "installed"
	case "", "domain_pack":
		if name == "" {
			return CapabilityApplyResult{}, fmt.Errorf("domain pack name is required")
		}
		status, err = domainpacks.SetEnabled(root, name, true)
		action = "enabled"
	default:
		return CapabilityApplyResult{}, fmt.Errorf("unsupported domain pack source: %s", source)
	}
	if err != nil {
		return CapabilityApplyResult{}, err
	}
	if err := s.reloadSkills(); err != nil {
		return CapabilityApplyResult{}, err
	}
	return CapabilityApplyResult{
		Pack:    status,
		Action:  action,
		Message: action,
	}, nil
}

func (s *Server) installBuiltInDomainPackTemplate(root string, name string, packDir string) (domainpacks.PackStatus, error) {
	catalog, err := builtInDomainPackTemplateCatalog()
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
	return domainpacks.Install(root, template.Path)
}

func builtInDomainPackTemplateCatalog() (workflows.TemplateCatalog, error) {
	root, err := builtInDomainPackTemplateRepoRoot()
	if err != nil {
		return workflows.TemplateCatalog{}, err
	}
	return workflows.NewBuiltInTemplateCatalog(root)
}

func builtInDomainPackTemplateRepoRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}
	dir := filepath.Clean(cwd)
	for {
		templates := filepath.Join(dir, "packs", "templates")
		if info, err := os.Stat(templates); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("built-in domain pack templates not found from %s", cwd)
		}
		dir = parent
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Server) attachGeneratedCapabilityJob(ctx context.Context, result *agent.CapabilityGenerationResult, input CapabilityGenerateInput) error {
	if result == nil {
		return fmt.Errorf("capability result is required")
	}
	extensionName := strings.TrimSpace(result.Generation.Extension.Name)
	if extensionName == "" {
		return fmt.Errorf("generated extension name is required before scheduling")
	}
	store, err := s.schedulerStore(ctx)
	if err != nil {
		return err
	}
	defer store.Close()
	scheduleType := strings.TrimSpace(input.ScheduleType)
	if scheduleType == "" {
		scheduleType = scheduler.ScheduleManual
	}
	scheduleInput := generatedCapabilityScheduleInput(input, result)
	if err := s.validateRequiredExtensionJobInput(extensionName, scheduleInput); err != nil {
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

func (s *Server) validateRequiredExtensionJobInput(targetName string, input map[string]any) error {
	name := strings.TrimSpace(targetName)
	if err := s.extensionStore().ValidateRunInput(name, input); err != nil {
		if errors.Is(err, extensions.ErrExtensionNotFound) {
			return fmt.Errorf("extension target %q is not installed; generate and register the extension before creating or enabling a scheduler job", name)
		}
		return err
	}
	return nil
}

func (s *Server) validateKnownExtensionJobInput(targetName string, input map[string]any) error {
	err := s.extensionStore().ValidateKnownRunInput(strings.TrimSpace(targetName), input)
	if errors.Is(err, extensions.ErrExtensionNotFound) {
		return nil
	}
	return err
}

func (s *Server) annotateSchedulerJobs(jobs []scheduler.Job) ([]scheduler.Job, int) {
	return scheduler.AnnotateJobInputStatuses(jobs, func(job scheduler.Job) (bool, error) {
		if strings.TrimSpace(job.TargetType) != scheduler.TargetExtension {
			return false, nil
		}
		err := s.extensionStore().ValidateRunInput(job.TargetName, job.Input)
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

func (s *Server) handleGenerateExtension(w http.ResponseWriter, r *http.Request) {
	var input ExtensionGenerateInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().Generate(r.Context(), extensions.GenerateInput{
		Name:              strings.TrimSpace(input.Name),
		Description:       strings.TrimSpace(input.Description),
		Approved:          input.Approved,
		MaxRuntimeSeconds: s.deps.Config.Extensions.MaxRuntimeSeconds,
		BrokeredNetwork:   input.BrokeredNetwork,
		AllowedDomains:    input.AllowedDomains,
		Draft:             input.Draft,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if s.deps.Memory != nil {
		_, _ = s.deps.Memory.SaveMemory(r.Context(), memory.Memory{
			Kind:       "extension_generation",
			Content:    result.SkillCandidate,
			Importance: 4,
			Source:     "extension_generator",
		})
	}
	writeJSON(w, result)
}

func (s *Server) handleValidateExtension(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().Validate(strings.TrimSpace(input.Name))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, ExtensionActionResult{Extension: result.Extension, Message: result.Message})
}

func (s *Server) handleTestExtension(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().Test(r.Context(), strings.TrimSpace(input.Name), extensions.TestOptions{})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleRegisterExtension(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().RegisterGenerated(r.Context(), strings.TrimSpace(input.Name), extensions.TestOptions{})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleSetExtensionEnabled(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().SetEnabled(strings.TrimSpace(input.Name), input.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, ExtensionActionResult{Extension: result.Extension, Message: result.Message})
}

func (s *Server) handleDeleteExtension(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().Delete(strings.TrimSpace(input.Name))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, ExtensionActionResult{Extension: result.Extension, Message: result.Message})
}

func (s *Server) handleRollbackExtension(w http.ResponseWriter, r *http.Request) {
	var input ExtensionRollbackInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().RollbackGeneration(input.NameOrSnapshot)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleRunExtension(w http.ResponseWriter, r *http.Request) {
	var input ExtensionRunInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.extensionStore().Run(r.Context(), strings.TrimSpace(input.Name), extensions.RunOptions{
		Input:             input.Input,
		MaxRuntimeSeconds: s.deps.Config.Extensions.MaxRuntimeSeconds,
		Internet:          s.internetService(),
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleExtensionFailures(w http.ResponseWriter, r *http.Request) {
	failures, err := s.extensionStore().Failures(intParam(r, "limit", 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, failures)
}

func (s *Server) handleInternetStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.internetService().Status())
}

func (s *Server) handleInternetFetch(w http.ResponseWriter, r *http.Request) {
	var input InternetFetchInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.internetService().Fetch(r.Context(), internet.FetchInput{
		URL:            input.URL,
		Method:         http.MethodGet,
		ExtractText:    input.ExtractText,
		AllowedDomains: input.AllowedDomains,
		TaskApproved:   input.TaskApproved,
		Caller:         "local_api",
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleInternetHead(w http.ResponseWriter, r *http.Request) {
	var input InternetFetchInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.internetService().Head(r.Context(), internet.FetchInput{
		URL:            input.URL,
		AllowedDomains: input.AllowedDomains,
		TaskApproved:   input.TaskApproved,
		Caller:         "local_api",
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleInternetSearch(w http.ResponseWriter, r *http.Request) {
	var input InternetSearchInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.internetService().Search(r.Context(), internet.SearchInput{
		Query:        input.Query,
		TaskApproved: input.TaskApproved,
		MaxResults:   input.MaxResults,
		Caller:       "local_api",
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleInternetCrawl(w http.ResponseWriter, r *http.Request) {
	var input InternetCrawlInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.internetService().Crawl(r.Context(), internet.CrawlInput{
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
		Caller:             "local_api",
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleInternetCrawls(w http.ResponseWriter, r *http.Request) {
	items, err := s.internetService().CrawlRuns(intParam(r, "limit", 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, items)
}

func (s *Server) handleInternetCrawlRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.internetService().CrawlRun(strings.TrimSpace(r.URL.Query().Get("id")))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeError(w, http.StatusNotFound, err)
			return
		}
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, run)
}

func (s *Server) handleInternetCache(w http.ResponseWriter, r *http.Request) {
	items, err := s.internetService().CacheList()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, items)
}

func (s *Server) handleInternetRequests(w http.ResponseWriter, r *http.Request) {
	items, err := s.internetService().Requests(intParam(r, "limit", 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, items)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	jobs, err := store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	jobs, _ = s.annotateSchedulerJobs(jobs)
	writeJSON(w, jobs)
}

func (s *Server) handleJobRuns(w http.ResponseWriter, r *http.Request) {
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	runs, err := store.LastRuns(r.Context(), intParam(r, "limit", 20))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, runs)
}

func (s *Server) handleJobStatus(w http.ResponseWriter, r *http.Request) {
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	status, err := store.Status(r.Context(), s.deps.Config.Scheduler.Enabled, s.schedulerMaxParallel())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if jobs, listErr := store.List(r.Context()); listErr == nil {
		_, invalid := s.annotateSchedulerJobs(jobs)
		status.InvalidInputJobs = invalid
	}
	writeJSON(w, status)
}

func (s *Server) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var input JobCreateInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if s.deps.Config.Scheduler.RequireApprovalForNewJobs && !input.Approved {
		writeError(w, http.StatusBadRequest, fmt.Errorf("new jobs require approval"))
		return
	}
	if err := scheduler.ValidateTarget(input.TargetType, input.TargetName); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(input.TargetType) == scheduler.TargetExtension {
		if err := s.validateRequiredExtensionJobInput(input.TargetName, input.Input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	job, err := store.Create(r.Context(), scheduler.CreateInput{
		Name:         input.Name,
		ScheduleType: input.ScheduleType,
		ScheduleExpr: input.ScheduleExpr,
		TargetType:   input.TargetType,
		TargetName:   input.TargetName,
		Input:        input.Input,
		Approved:     input.Approved || !s.deps.Config.Scheduler.RequireApprovalForNewJobs,
		Enabled:      input.Enabled,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, job)
}

func (s *Server) handleUpdateJobInput(w http.ResponseWriter, r *http.Request) {
	var input JobInputUpdateInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !input.Approved {
		writeError(w, http.StatusBadRequest, fmt.Errorf("use approved=true to approve updating the local scheduled job input"))
		return
	}
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	job, err := store.Get(r.Context(), input.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if job.TargetType == scheduler.TargetExtension {
		if err := s.validateRequiredExtensionJobInput(job.TargetName, input.Input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	updated, err := store.UpdateInput(r.Context(), scheduler.UpdateInput{ID: input.ID, Input: input.Input})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	annotated, _ := s.annotateSchedulerJobs([]scheduler.Job{updated})
	if len(annotated) == 1 {
		updated = annotated[0]
	}
	writeJSON(w, updated)
}

func (s *Server) handleSetJobEnabled(w http.ResponseWriter, r *http.Request) {
	var input JobEnabledInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	if input.Enabled {
		job, err := store.Get(r.Context(), input.ID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if job.TargetType == scheduler.TargetExtension {
			if err := s.validateRequiredExtensionJobInput(job.TargetName, job.Input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
		}
	}
	job, err := store.SetEnabled(r.Context(), input.ID, input.Enabled)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, job)
}

func (s *Server) handleRunJob(w http.ResponseWriter, r *http.Request) {
	var input JobRunInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	job, jobErr := store.Get(r.Context(), input.ID)
	if jobErr == nil && job.TargetType == scheduler.TargetExtension {
		if err := s.validateKnownExtensionJobInput(job.TargetName, job.Input); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	run, err := store.Run(r.Context(), input.ID, s.jobRunner(), s.jobRunOptions())
	if jobErr == nil {
		s.recordJobRunNotification(job, run)
	}
	if jobErr == nil && s.deps.Memory != nil {
		if _, recordErr := agent.RecordJobRunResult(r.Context(), s.deps.Memory, job, run, input.ConversationID); recordErr != nil && err == nil {
			writeError(w, http.StatusInternalServerError, recordErr)
			return
		}
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, run)
}

func (s *Server) handleRunDueJobs(w http.ResponseWriter, r *http.Request) {
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	loop := scheduler.Loop{
		Store:   store,
		Runner:  s.jobRunner(),
		Options: s.jobRunOptions(),
	}
	result, err := loop.Tick(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.recordJobRunMemories(r.Context(), store, result.Runs, ""); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleArchiveJob(w http.ResponseWriter, r *http.Request) {
	var input JobArchiveInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !input.Approved {
		writeError(w, http.StatusBadRequest, fmt.Errorf("use approved=true to archive the local scheduled job"))
		return
	}
	store, err := s.schedulerStore(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	job, err := store.Archive(r.Context(), input.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, job)
}

func (s *Server) recordJobRunMemories(ctx context.Context, store *scheduler.Store, runs []scheduler.JobRun, conversationID string) error {
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
		s.recordJobRunNotification(job, run)
		if s.deps.Memory == nil {
			continue
		}
		if _, err := agent.RecordJobRunResult(ctx, s.deps.Memory, job, run, conversationID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) recordJobRunNotification(job scheduler.Job, run scheduler.JobRun) {
	store, err := s.notificationStore()
	if err != nil {
		return
	}
	_, _, _ = notifications.RecordJobRunNotification(store, job, run)
}

func (s *Server) handleHeartbeatStatus(w http.ResponseWriter, r *http.Request) {
	report, err := s.heartbeatReport(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	store, err := heartbeat.Open(r.Context(), s.deps.Profile.Database)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer store.Close()
	previous, _ := store.Latest(r.Context())
	if err := store.Record(r.Context(), report); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.recordHeartbeatTransitionNotification(previous, report)
	writeJSON(w, report)
}

func (s *Server) recordHeartbeatTransitionNotification(previous heartbeat.Report, report heartbeat.Report) {
	store, err := s.notificationStore()
	if err != nil {
		return
	}
	_, _ = notifications.RecordHeartbeatTransitionNotifications(store, previous, report)
}

func (s *Server) handlePolicyStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, policyStatusView(s.deps.Config))
}

func (s *Server) handleSetPolicyMode(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Mode string `json:"mode"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := applyPolicyMode(s.deps.Config, input.Mode); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Write(s.deps.Config.Path, s.deps.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, policyStatusView(s.deps.Config))
}

func (s *Server) handleLearningReport(w http.ResponseWriter, r *http.Request) {
	report, err := learning.BuildReport(r.Context(), s.deps.Memory, s.extensionStore())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, report)
}

func (s *Server) handleExportTrajectory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ConversationID string `json:"conversationId"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	trajectory, err := learning.ExportTrajectory(r.Context(), s.deps.Memory, input.ConversationID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, trajectory)
}

func (s *Server) handleSaveCorrection(w http.ResponseWriter, r *http.Request) {
	var input LearningCorrectionInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := learning.SaveCorrection(r.Context(), s.deps.Memory, input.ConversationID, input.Correction)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, memoryItemResult(item))
}

func (s *Server) handleRouteCorrections(w http.ResponseWriter, r *http.Request) {
	corrections, err := learning.ListRouteCorrections(r.Context(), s.deps.Memory, 200)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, corrections)
}

func (s *Server) handleSaveRouteCorrection(w http.ResponseWriter, r *http.Request) {
	var input RouteCorrectionInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	status := routing.RouteCorrectionStatusPending
	if input.Approved {
		status = routing.RouteCorrectionStatusApproved
	}
	correction, err := learning.SaveRouteCorrection(r.Context(), s.deps.Memory, routing.RouteCorrection{
		ID:                    input.ID,
		SourceConversationID:  input.ConversationID,
		Pattern:               input.Pattern,
		IntendedRouteCategory: input.IntendedRouteCategory,
		IntendedTaskType:      input.IntendedTaskType,
		IntendedCapability:    input.IntendedCapability,
		RequiredTools:         append([]string{}, input.RequiredTools...),
		ForbiddenTools:        append([]string{}, input.ForbiddenTools...),
		Tags:                  append([]string{}, input.Tags...),
		OriginalPrompt:        input.OriginalPrompt,
		CorrectionText:        input.CorrectionText,
		ClarificationQuestion: input.ClarificationQuestion,
		ApprovalStatus:        status,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, correction)
}

func (s *Server) handleApproveRouteCorrection(w http.ResponseWriter, r *http.Request) {
	var input RouteCorrectionIDInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	correction, err := learning.ApproveRouteCorrection(r.Context(), s.deps.Memory, input.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, correction)
}

func (s *Server) handleDisableRouteCorrection(w http.ResponseWriter, r *http.Request) {
	var input RouteCorrectionIDInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	correction, err := learning.DisableRouteCorrection(r.Context(), s.deps.Memory, input.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, correction)
}

func (s *Server) handleMemory(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	memories, err := s.deps.Memory.SearchMemories(r.Context(), query, s.deps.Config.Memory.MaxRelevantMemories)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	messages, err := s.deps.Memory.SearchMessages(r.Context(), query, s.deps.Config.Memory.MaxRelevantMemories)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, combinedMemoryResults(memories, messages))
}

func (s *Server) handleKnowledgeStatus(w http.ResponseWriter, r *http.Request) {
	status, err := s.knowledgeStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleSetKnowledgeEnabled(w http.ResponseWriter, r *http.Request) {
	var input KnowledgeEnabledInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.deps.Config.Knowledge.Enabled = input.Enabled
	s.deps.Config.Knowledge.InfluenceEnabled = input.Enabled && input.InfluenceEnabled
	s.deps.Config.Knowledge.ManualOnly = true
	config.ApplyDefaults(s.deps.Config)
	if err := config.Write(s.deps.Config.Path, s.deps.Config); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status, err := s.knowledgeStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, status)
}

func (s *Server) handleKnowledgeEntities(w http.ResponseWriter, r *http.Request) {
	store, err := s.openEnabledKnowledgeStore(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer store.Close()
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	entities, err := store.SearchEntities(r.Context(), query, knowledge.SearchOptions{Limit: intParam(r, "limit", s.deps.Config.Knowledge.MaxEntitiesPerQuery)})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, knowledgeEntityResults(entities))
}

func (s *Server) handleDraftKnowledgeProposal(w http.ResponseWriter, r *http.Request) {
	var input KnowledgeProposalInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
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
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, knowledgeProposalResult(proposal))
}

func (s *Server) handleRepairKnowledgeEvidence(w http.ResponseWriter, r *http.Request) {
	store, err := s.openEnabledKnowledgeStore(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer store.Close()
	result, err := store.RepairEvidence(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	status, err := s.knowledgeStatus(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, knowledgeRepairResult(result, status))
}

func (s *Server) handleKnowledgeReviewItems(w http.ResponseWriter, r *http.Request) {
	store, err := s.openEnabledKnowledgeStore(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer store.Close()
	items, err := store.ListReviewItems(r.Context(), knowledge.ReviewListOptions{
		Status: r.URL.Query().Get("status"),
		Limit:  intParam(r, "limit", 25),
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, knowledgeReviewItemResults(items))
}

func (s *Server) handleKnowledgeReviewBatch(w http.ResponseWriter, r *http.Request) {
	store, err := s.openEnabledKnowledgeStore(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer store.Close()
	var input KnowledgeReviewBatchInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	targets := make([]knowledge.ReviewTarget, 0, len(input.Targets))
	for _, target := range input.Targets {
		targets = append(targets, knowledge.ReviewTarget{Type: target.Type, ID: target.ID})
	}
	result, err := store.ApplyReviewBatch(r.Context(), knowledge.ReviewBatchInput{
		Action:     input.Action,
		Targets:    targets,
		ReviewedBy: input.ReviewedBy,
		Note:       input.Note,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, knowledgeReviewBatchResult(result))
}

func (s *Server) handleSaveKnowledgeEntity(w http.ResponseWriter, r *http.Request) {
	store, err := s.openEnabledKnowledgeStore(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer store.Close()
	var input KnowledgeEntityInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	entity, err := store.UpsertEntity(r.Context(), knowledge.EntityInput{
		Name:       input.Name,
		Kind:       input.Kind,
		Source:     input.Source,
		SourceKind: input.SourceKind,
		SourceRef:  input.SourceRef,
		Evidence:   input.Evidence,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, knowledgeEntityResult(entity))
}

func (s *Server) handleSaveKnowledgeEdge(w http.ResponseWriter, r *http.Request) {
	store, err := s.openEnabledKnowledgeStore(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer store.Close()
	var input KnowledgeEdgeInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	edge, err := store.AddEdge(r.Context(), knowledge.EdgeInput{
		FromEntityID: input.FromEntityID,
		Relation:     input.Relation,
		ToEntityID:   input.ToEntityID,
		Source:       input.Source,
		SourceKind:   input.SourceKind,
		SourceRef:    input.SourceRef,
		Evidence:     input.Evidence,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, knowledgeEdgeResult(edge))
}

func (s *Server) handleConversations(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50)
	conversations, err := s.deps.Memory.ListConversations(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, conversationResults(conversations))
}

func (s *Server) handleConversation(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	conversation, err := s.deps.Memory.GetConversation(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	messages, err := s.deps.Memory.ListConversationMessages(r.Context(), id, 500)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	messages = memory.WithInferredResponseParents(messages)
	activities, err := agent.ConversationMessageActivities(r.Context(), s.deps.Memory, s.replayStore(), id, messages)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	turns, err := s.deps.Memory.ListAgentTurnsForConversation(r.Context(), id, 200)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	attachmentsByMessage, err := s.chatAttachmentsForMessages(r.Context(), messages)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, ConversationDetailResult{
		Conversation: conversationResult(conversation),
		Messages:     conversationMessageResultsWithAttachments(messages, activities, turns, attachmentsByMessage),
	})
}

func (s *Server) handleRenameConversation(w http.ResponseWriter, r *http.Request) {
	var input ConversationTitleInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	conversation, err := s.deps.Memory.UpdateConversationTitle(r.Context(), input.ConversationID, input.Title)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, conversationResult(conversation))
}

func (s *Server) handleStarConversation(w http.ResponseWriter, r *http.Request) {
	var input ConversationStarInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	conversation, err := s.deps.Memory.SetConversationStarred(r.Context(), input.ConversationID, input.Starred)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, conversationResult(conversation))
}

func (s *Server) handleDeleteConversation(w http.ResponseWriter, r *http.Request) {
	var input ConversationDeleteInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.deps.Memory.DeleteConversation(r.Context(), input.ConversationID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (s *Server) handleEditUserMessage(w http.ResponseWriter, r *http.Request) {
	var input MessageEditInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	message, err := s.deps.Memory.CreateUserMessageVariant(r.Context(), input.MessageID, input.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, conversationMessageResults([]memory.Message{message}, nil, nil)[0])
}

func (s *Server) handleMemories(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", s.deps.Config.Memory.MaxRelevantMemories)
	memories, err := s.deps.Memory.ListMemories(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, explicitMemoryResults(memories))
}

func (s *Server) handleWriteMemory(w http.ResponseWriter, r *http.Request) {
	var input MemoryWriteInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	item, err := s.deps.Memory.SaveMemory(r.Context(), memory.Memory{
		Kind:       strings.TrimSpace(input.Kind),
		Content:    strings.TrimSpace(input.Content),
		Importance: input.Importance,
		Source:     "web",
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, memoryItemResult(item))
}

func (s *Server) handlePinMemory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.deps.Memory.PinMemory(r.Context(), input.ID, true); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.deps.Memory.DisableMemory(r.Context(), input.ID); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

func (s *Server) handleToolRuns(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 50)
	filter := memory.ToolRunFilter{
		ConversationID: strings.TrimSpace(r.URL.Query().Get("conversationId")),
		SessionID:      strings.TrimSpace(r.URL.Query().Get("sessionId")),
		UserMessageID:  strings.TrimSpace(r.URL.Query().Get("userMessageId")),
		Limit:          limit,
	}
	runs, err := s.deps.Memory.ListToolRunsFiltered(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, toolRunResults(runs))
}

func (s *Server) handleToolCatalog(w http.ResponseWriter, r *http.Request) {
	surface := toolSurfaceFromQuery(r.URL.Query().Get("surface"))
	writeJSON(w, toolCatalogResults(tools.ToolCatalogForSurface(surface)))
}

func (s *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	store, err := s.notificationStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	items, err := store.List(intParam(r, "limit", 30), boolParam(r, "includeDismissed", false))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, items)
}

func (s *Server) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	store, err := s.notificationStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	item, err := store.MarkRead(input.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, item)
}

func (s *Server) handleDismissNotification(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ID string `json:"id"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	store, err := s.notificationStore()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	item, err := store.Dismiss(input.ID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, item)
}

func (s *Server) handleReplayTraces(w http.ResponseWriter, r *http.Request) {
	traces, err := s.replayTraceStore().List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, limitLatestReplayTraces(traces, intParam(r, "limit", 30)))
}

func (s *Server) handleFeedbackReview(w http.ResponseWriter, r *http.Request) {
	review, err := s.qaReview(r.Context(), intParam(r, "limit", 30))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, review)
}

func (s *Server) qaReview(ctx context.Context, limit int) (learning.QAReview, error) {
	latestEval := s.latestQAEvalSummary()
	review, err := learning.BuildQAReview(ctx, s.deps.Memory, s.replayTraceStore(), latestEval, limit)
	if err != nil {
		return learning.QAReview{}, err
	}
	if dir, err := s.qaRegressionReviewDir(); err == nil {
		records, err := learning.ListQARegressionReviews(dir)
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionReviews(&review, records)
	}
	if draftDir, err := s.qaRegressionTestDraftDir(); err == nil {
		drafts, err := learning.ListQARegressionTestDrafts(draftDir)
		if err != nil {
			return learning.QAReview{}, err
		}
		approvalDir, err := s.qaRegressionApprovalDir()
		if err != nil {
			return learning.QAReview{}, err
		}
		approvals, err := learning.ListQARegressionApprovedRecords(approvalDir)
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionDrafts(&review, drafts, approvals)
		patchDir, err := s.qaRegressionSourcePatchDir()
		if err != nil {
			return learning.QAReview{}, err
		}
		patches, err := learning.ListQARegressionSourcePatchDrafts(patchDir)
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionSourcePatches(&review, patches)
		applyPlanDir, err := s.qaRegressionSourcePatchApplyPlanDir()
		if err != nil {
			return learning.QAReview{}, err
		}
		applyPlans, err := learning.ListQARegressionSourcePatchApplyPlans(applyPlanDir)
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionSourcePatchApplyPlans(&review, applyPlans)
		sourceWriteDir, err := s.qaRegressionSourceWriteDir()
		if err != nil {
			return learning.QAReview{}, err
		}
		sourceWrites, err := learning.ListQARegressionSourceWrites(sourceWriteDir)
		if err != nil {
			return learning.QAReview{}, err
		}
		learning.ApplyQARegressionSourceWrites(&review, sourceWrites)
	}
	return review, nil
}

func (s *Server) handlePromoteFeedbackRegression(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionPromotionRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	dir, err := s.qaRegressionPromotionDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	artifact, err := learning.PromoteQARegressionSuggestion(dir, s.replayTraceStore(), s.latestQAEvalSummary(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, artifact)
}

func (s *Server) handleReviewFeedbackRegressionStatus(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionReviewRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	dir, err := s.qaRegressionReviewDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := learning.ReviewQARegressionSuggestion(dir, s.replayTraceStore(), s.latestQAEvalSummary(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, record)
}

func (s *Server) handleGenerateFeedbackRegressionTestDraft(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionTestDraftRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	reviewDir, err := s.qaRegressionReviewDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	reviews, err := learning.ListQARegressionReviews(reviewDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	draftDir, err := s.qaRegressionTestDraftDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	draft, err := learning.GenerateQARegressionTestDraft(draftDir, reviews, s.replayTraceStore(), s.latestQAEvalSummary(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, draft)
}

func (s *Server) handleApproveFeedbackRegressionTestDraft(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionApprovalRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	draftDir, err := s.qaRegressionTestDraftDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	drafts, err := learning.ListQARegressionTestDrafts(draftDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	approvalDir, err := s.qaRegressionApprovalDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := learning.ApproveQARegressionTestDraft(approvalDir, drafts, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, record)
}

func (s *Server) handleGenerateFeedbackRegressionSourcePatch(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionSourcePatchRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	approvalDir, err := s.qaRegressionApprovalDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	approvals, err := learning.ListQARegressionApprovedRecords(approvalDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	patchDir, err := s.qaRegressionSourcePatchDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := learning.GenerateQARegressionSourcePatchDraft(patchDir, approvals, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, record)
}

func (s *Server) handleApproveFeedbackRegressionSourcePatchApplyPlan(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionSourcePatchApplyPlanRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	patchDir, err := s.qaRegressionSourcePatchDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	patches, err := learning.ListQARegressionSourcePatchDrafts(patchDir)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	applyPlanDir, err := s.qaRegressionSourcePatchApplyPlanDir()
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	record, err := learning.GenerateQARegressionSourcePatchApplyPlan(applyPlanDir, patches, input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, record)
}

func (s *Server) handlePlanFeedbackRegressionSourceWrite(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionSourceWriteRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.planQARegressionSourceWrite(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleApplyFeedbackRegressionSourceWrite(w http.ResponseWriter, r *http.Request) {
	var input learning.QARegressionSourceWriteRequest
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.applyQARegressionSourceWrite(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleExplainReplayTrace(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("replay trace id is required"))
		return
	}
	trace, err := s.replayTraceStore().Load(id)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, replay.ExplainTrace(trace))
}

func (s *Server) latestQAEvalSummary() *learning.QAEvalSummary {
	report, err := evaluation.LoadLatest(s.deps.Profile)
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

func (s *Server) handlePermissionDecision(w http.ResponseWriter, r *http.Request) {
	var input PermissionDecisionInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.recordPermissionDecision(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handlePlanFileWrite(w http.ResponseWriter, r *http.Request) {
	var input FileWriteInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.planFileWrite(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleApplyFileWrite(w http.ResponseWriter, r *http.Request) {
	var input FileWriteInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.applyFileWrite(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	var results []rag.SearchResult
	var err error
	if query == "" {
		results, err = s.deps.RAG.ListChunks(r.Context(), s.deps.Config.RAG.TopK)
	} else {
		results, err = s.deps.RAG.SearchWithConfig(r.Context(), query, ragConfig(s.deps.Config.RAG), embeddingRuntime(s.deps.Runtime))
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
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
	writeJSON(w, out)
}

func (s *Server) handleDocumentInventory(w http.ResponseWriter, r *http.Request) {
	limit := intParam(r, "limit", 100)
	items, err := s.deps.RAG.ListDocuments(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, documentInventoryResults(items))
}

func (s *Server) handlePruneMissingDocuments(w http.ResponseWriter, r *http.Request) {
	var input DocumentPruneInput
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.deps.RAG.PruneMissingDocuments(r.Context(), !input.Confirm)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, documentPruneResult(result))
}

func (s *Server) handleDocumentSuggestions(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		path = "."
	}
	limit := 40
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	effectivePath := s.workspaceRootPath(path)
	accessPath, err := rag.SuggestDocumentAccessPath(effectivePath)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	access, err := safety.RequireWorkspaceAccessWithBookmark(s.workspaceBase(), accessPath, s.workspaceGrantStore())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer access.Close()
	suggestions, err := rag.SuggestDocumentPaths(effectivePath, workspaceLimits(s.deps.Config.Workspace), limit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	out := documentPathSuggestions(suggestions)
	if !filepath.IsAbs(path) {
		out = s.relativeDocumentPathSuggestions(out)
	}
	writeJSON(w, out)
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path string `json:"path"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	path := strings.TrimSpace(input.Path)
	if path == "" {
		path = "."
	}
	effectivePath := s.workspaceRootPath(path)
	access, err := safety.RequireWorkspaceAccessWithBookmark(s.workspaceBase(), effectivePath, s.workspaceGrantStore())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	defer access.Close()
	result, err := s.deps.RAG.IngestPath(r.Context(), s.deps.Profile.Name, effectivePath, ragConfig(s.deps.Config.RAG), workspaceLimits(s.deps.Config.Workspace))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, IngestSummary{
		Root:           result.Root,
		FilesIndexed:   result.FilesIndexed,
		FilesSkipped:   result.FilesSkipped,
		FilesUnchanged: result.FilesUnchanged,
		ChunksCreated:  result.ChunksCreated,
		BytesIndexed:   result.BytesIndexed,
		SkippedReasons: result.SkippedReasons,
	})
}

func (s *Server) handleUploadDocument(w http.ResponseWriter, r *http.Request) {
	limits := workspaceLimits(s.deps.Config.Workspace)
	maxBytes := maxDocumentUploadBytes(limits)
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes+(1<<20))

	if err := r.ParseMultipartForm(maxBytes + (1 << 20)); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parse document upload: %w", err))
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("document file is required"))
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("read document upload: %w", err))
		return
	}
	if int64(len(data)) > maxBytes {
		writeError(w, http.StatusBadRequest, fmt.Errorf("uploaded document exceeds max read size"))
		return
	}

	uploadName := header.Filename
	if relativePath := strings.TrimSpace(r.FormValue("relativePath")); relativePath != "" {
		uploadName = relativePath
	}
	result, err := s.deps.RAG.IngestUploadedFile(r.Context(), s.deps.Profile.Name, uploadName, data, ragConfig(s.deps.Config.RAG), limits)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, IngestSummary{
		Root:           result.Root,
		FilesIndexed:   result.FilesIndexed,
		FilesSkipped:   result.FilesSkipped,
		FilesUnchanged: result.FilesUnchanged,
		ChunksCreated:  result.ChunksCreated,
		BytesIndexed:   result.BytesIndexed,
		SkippedReasons: result.SkippedReasons,
	})
}

func (s *Server) handleUploadChatAttachment(w http.ResponseWriter, r *http.Request) {
	upload, err := readChatAttachmentUpload(w, r, workspaceLimits(s.deps.Config.Workspace))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	attachment, err := attachments.ProcessUpload(r.Context(), attachments.UploadInput{
		FileName:    upload.FileName,
		ContentType: upload.ContentType,
		Data:        upload.Data,
		Retention:   upload.Retention,
	}, workspaceLimits(s.deps.Config.Workspace))
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	saved, err := s.deps.Memory.SaveChatAttachment(r.Context(), attachment)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, ChatAttachmentUploadResult{
		Attachment: chatAttachmentResult(saved),
		Message:    "Attachment ready for this conversation.",
	})
}

func (s *Server) handleIndexEmbeddings(w http.ResponseWriter, r *http.Request) {
	result, err := s.indexEmbeddings(r.Context())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) indexEmbeddings(ctx context.Context) (EmbeddingIndexSummary, error) {
	cfg := ragConfig(s.deps.Config.RAG)
	if !cfg.Embeddings.Enabled {
		return EmbeddingIndexSummary{}, fmt.Errorf("embeddings are disabled; enable embeddings with an installed local model first")
	}
	result, err := s.deps.RAG.EnsureEmbeddings(ctx, cfg, embeddingRuntime(s.deps.Runtime))
	if err != nil {
		return EmbeddingIndexSummary{}, err
	}
	return embeddingIndexSummary(result), nil
}

func (s *Server) handleWorkspaceGrants(w http.ResponseWriter, r *http.Request) {
	grants, err := s.workspaceGrantStore().List()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]WorkspaceGrantResult, 0, len(grants))
	for _, grant := range grants {
		out = append(out, workspaceGrantResult(grant))
	}
	writeJSON(w, out)
}

func (s *Server) handleGrantWorkspace(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Path  string `json:"path"`
		Label string `json:"label"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	path := strings.TrimSpace(input.Path)
	if path == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workspace path is required"))
		return
	}
	effectivePath := s.workspaceRootPath(path)
	label := strings.TrimSpace(input.Label)
	if label == "" {
		label = filepath.Base(effectivePath)
	}
	grant, err := s.workspaceGrantStore().Grant(effectivePath, label, "local_web")
	if err != nil {
		_, _ = s.deps.Memory.SaveToolRun(r.Context(), memory.ToolRun{
			ToolName:  "workspace_grant",
			Input:     map[string]any{"path": effectivePath, "source": "local_web"},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "low",
		})
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_, _ = s.deps.Memory.SaveToolRun(r.Context(), memory.ToolRun{
		ToolName:  "workspace_grant",
		Input:     map[string]any{"path": grant.Path, "label": grant.Label, "source": grant.Source},
		Output:    map[string]any{"id": grant.ID},
		Status:    "completed",
		RiskLevel: "low",
	})
	writeJSON(w, workspaceGrantResult(grant))
}

func (s *Server) handleRevokeWorkspaceGrant(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Match string `json:"match"`
		ID    string `json:"id"`
		Path  string `json:"path"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	match := strings.TrimSpace(input.Match)
	if match == "" {
		match = strings.TrimSpace(input.ID)
	}
	if match == "" {
		match = strings.TrimSpace(input.Path)
	}
	if match == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("workspace grant id or path is required"))
		return
	}
	grant, ok, err := s.workspaceGrantStore().Revoke(match)
	if err != nil {
		_, _ = s.deps.Memory.SaveToolRun(r.Context(), memory.ToolRun{
			ToolName:  "workspace_revoke",
			Input:     map[string]any{"match": match},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "low",
		})
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("workspace grant not found: %s", match))
		return
	}
	_, _ = s.deps.Memory.SaveToolRun(r.Context(), memory.ToolRun{
		ToolName:  "workspace_revoke",
		Input:     map[string]any{"match": match},
		Output:    map[string]any{"id": grant.ID, "path": grant.Path},
		Status:    "completed",
		RiskLevel: "low",
	})
	writeJSON(w, workspaceGrantResult(grant))
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content        string `json:"content"`
		ConversationID string `json:"conversationId"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.chat(r.Context(), input.Content, input.ConversationID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content         string   `json:"content"`
		Skill           string   `json:"skill"`
		ConversationID  string   `json:"conversationId"`
		ParentMessageID string   `json:"parentMessageId"`
		AttachmentIDs   []string `json:"attachmentIds"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.ask(r.Context(), input.Content, input.Skill, input.ConversationID, input.ParentMessageID, input.AttachmentIDs)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, result)
}

func (s *Server) handleAskStream(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Content         string   `json:"content"`
		Skill           string   `json:"skill"`
		ConversationID  string   `json:"conversationId"`
		ParentMessageID string   `json:"parentMessageId"`
		AttachmentIDs   []string `json:"attachmentIds"`
	}
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.streamAsk(w, r.Context(), input.Content, input.Skill, input.ConversationID, input.ParentMessageID, input.AttachmentIDs); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
}

func (s *Server) handleStopAsk(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cancel := s.activeAskCancel
	s.activeAskCancel = nil
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	writeJSON(w, map[string]bool{"stopped": cancel != nil})
}

func (s *Server) handleLocalConnectorHealth(w http.ResponseWriter, r *http.Request) {
	if !s.localConnectorAllowed(w, r, "") {
		return
	}
	writeJSON(w, connectors.LocalAPIStatus(s.deps.Config.Connectors))
}

func (s *Server) handleLocalConnectorTools(w http.ResponseWriter, r *http.Request) {
	if !s.localConnectorAllowed(w, r, "") {
		return
	}
	writeJSON(w, connectors.LocalAPITools())
}

func (s *Server) handleLocalConnectorMemorySearch(w http.ResponseWriter, r *http.Request) {
	if !s.localConnectorAllowed(w, r, "memory_search") {
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("query is required"))
		return
	}
	limit := intParam(r, "limit", s.deps.Config.Memory.MaxRelevantMemories)
	if limit < 1 || limit > 20 {
		limit = s.deps.Config.Memory.MaxRelevantMemories
	}
	memories, err := s.deps.Memory.SearchMemories(r.Context(), query, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, explicitMemoryResults(memories))
}

func (s *Server) handleLocalConnectorChat(w http.ResponseWriter, r *http.Request) {
	if !s.localConnectorAllowed(w, r, "chat") {
		return
	}
	var input struct {
		Content string `json:"content"`
	}
	if err := readConnectorJSON(w, r, s.deps.Config.Connectors.LocalAPI, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.chat(r.Context(), input.Content, "")
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, ConnectorResult{
		ConversationID: result.ConversationID,
		Text:           result.Text,
		Model:          result.Model,
	})
}

func (s *Server) handleLocalConnectorAsk(w http.ResponseWriter, r *http.Request) {
	if !s.localConnectorAllowed(w, r, "ask") {
		return
	}
	var input struct {
		Content string `json:"content"`
		Skill   string `json:"skill"`
	}
	if err := readConnectorJSON(w, r, s.deps.Config.Connectors.LocalAPI, &input); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.ask(r.Context(), input.Content, input.Skill, "", "", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, ConnectorResult{
		ConversationID: result.ConversationID,
		Text:           result.Text,
		Model:          result.Model,
		Skill:          result.Skill,
		Sources:        result.Sources,
		SourceKind:     result.SourceKind,
	})
}

func (s *Server) handleAdapterConnectorStatus(w http.ResponseWriter, r *http.Request) {
	adapter := strings.TrimSpace(r.PathValue("adapter"))
	if !s.adapterConnectorAllowed(w, r, adapter, "") {
		return
	}
	writeJSON(w, connectors.AdapterStatus(s.deps.Config.Connectors, adapter))
}

func (s *Server) handleAdapterConnectorChat(w http.ResponseWriter, r *http.Request) {
	adapter := strings.TrimSpace(r.PathValue("adapter"))
	if !s.adapterConnectorAllowed(w, r, adapter, "chat") {
		return
	}
	input, err := s.readAdapterInput(w, r, adapter)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.chat(r.Context(), input.Content, "")
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, connectors.AdapterResponse(adapter, connectors.CoreResult{
		ConversationID: result.ConversationID,
		Text:           result.Text,
		Model:          result.Model,
	}))
}

func (s *Server) handleAdapterConnectorAsk(w http.ResponseWriter, r *http.Request) {
	adapter := strings.TrimSpace(r.PathValue("adapter"))
	if !s.adapterConnectorAllowed(w, r, adapter, "ask") {
		return
	}
	input, err := s.readAdapterInput(w, r, adapter)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if input.Skill == "" {
		input.Skill = strings.TrimSpace(r.URL.Query().Get("skill"))
	}
	result, err := s.ask(r.Context(), input.Content, input.Skill, "", "", nil)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	writeJSON(w, connectors.AdapterResponse(adapter, connectors.CoreResult{
		ConversationID: result.ConversationID,
		Text:           result.Text,
		Model:          result.Model,
		Skill:          result.Skill,
		Sources:        result.Sources,
		SourceKind:     result.SourceKind,
	}))
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
		return
	}
	staticDir := strings.TrimSpace(s.deps.StaticDir)
	if staticDir != "" {
		requestPath := filepath.Clean(r.URL.Path)
		requestPath = strings.TrimPrefix(requestPath, string(filepath.Separator))
		if requestPath == "." || requestPath == "" {
			requestPath = "index.html"
		}
		target := filepath.Join(staticDir, requestPath)
		if info, err := os.Stat(target); err == nil && !info.IsDir() {
			if s.deps.FrontendWatch {
				w.Header().Set("Cache-Control", "no-store")
			}
			if requestPath == "index.html" && s.deps.FrontendWatch {
				s.serveIndex(w, r, target)
				return
			}
			http.ServeFile(w, r, target)
			return
		}
		index := filepath.Join(staticDir, "index.html")
		if info, err := os.Stat(index); err == nil && !info.IsDir() {
			if s.deps.FrontendWatch {
				w.Header().Set("Cache-Control", "no-store")
				s.serveIndex(w, r, index)
				return
			}
			http.ServeFile(w, r, index)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, fallbackHTML())
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.ServeFile(w, r, path)
		return
	}
	html := string(data)
	if s.deps.FrontendWatch && !strings.Contains(html, "yemakaFrontendLiveReload") {
		html = strings.Replace(html, "</body>", liveReloadScript()+"\n</body>", 1)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodHead {
		return
	}
	_, _ = io.WriteString(w, html)
}

func (s *Server) handleFrontendVersion(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	version := s.frontendVersion
	errText := s.frontendError
	s.mu.Unlock()
	if version == "" {
		version = "static"
	}
	writeJSON(w, map[string]string{
		"version": version,
		"error":   errText,
	})
}

func (s *Server) recordPermissionDecision(ctx context.Context, input PermissionDecisionInput) (PermissionDecisionResult, error) {
	decision, err := normalizePermissionDecision(input.Decision)
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	if strings.TrimSpace(input.Request.ToolName) == "" {
		return PermissionDecisionResult{}, fmt.Errorf("permission tool name is required")
	}
	if decision == "approved" {
		if err := agent.ValidateStoredPermissionApproval(ctx, s.deps.Memory, input.Request); err != nil {
			return PermissionDecisionResult{}, err
		}
	}
	requestRun, _, err := s.permissionRequestRun(ctx, input.Request.RequestID)
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	_, err = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
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
		result, err = s.savePermissionDecisionMessage(ctx, input.Request, result, requestRun)
		if err != nil {
			return PermissionDecisionResult{}, err
		}
		return result, nil
	}
	if editResult, handled, err := s.applyApprovedEditPermission(ctx, input.Request); err != nil {
		return PermissionDecisionResult{}, err
	} else if handled {
		editResult, err = s.savePermissionDecisionMessage(ctx, input.Request, editResult, requestRun)
		if err != nil {
			return PermissionDecisionResult{}, err
		}
		return editResult, nil
	}
	outcome := agent.ResolveApprovedPermission(ctx, input.Request, serverToolExecutor(s.deps))
	result.ToolName = outcome.Decision.ToolName
	result.ToolStatus = outcome.ToolStatus
	result.Executed = outcome.Executed
	result.Message = outcome.Message
	result.Result = outcome.Result
	_, _ = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
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
	result, err = s.savePermissionDecisionMessage(ctx, input.Request, result, requestRun)
	if err != nil {
		return PermissionDecisionResult{}, err
	}
	return result, nil
}

func (s *Server) permissionRequestRun(ctx context.Context, requestID string) (memory.ToolRun, bool, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" || s.deps.Memory == nil {
		return memory.ToolRun{}, false, nil
	}
	runs, err := s.deps.Memory.ListToolRuns(ctx, 200)
	if err != nil {
		return memory.ToolRun{}, false, fmt.Errorf("load permission request run: %w", err)
	}
	for _, run := range runs {
		if normalizePermissionResultToolName(run.ToolName) != "permission_request" {
			continue
		}
		if toolRunRequestID(run.Input) == requestID || toolRunRequestID(run.Output) == requestID {
			return run, true, nil
		}
	}
	return memory.ToolRun{}, false, nil
}

func toolRunRequestID(value any) string {
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

func normalizePermissionResultToolName(tool string) string {
	return strings.ToLower(strings.TrimSpace(tool))
}

func (s *Server) savePermissionDecisionMessage(ctx context.Context, request agent.PermissionRequest, result PermissionDecisionResult, requestRun memory.ToolRun) (PermissionDecisionResult, error) {
	if s.deps.Memory == nil || strings.TrimSpace(result.Message) == "" || strings.TrimSpace(requestRun.ConversationID) == "" {
		return result, nil
	}
	parentID := strings.TrimSpace(requestRun.AssistantMessageID)
	if parentID == "" {
		parentID = strings.TrimSpace(requestRun.ParentMessageID)
	}
	if parentID == "" {
		parentID = strings.TrimSpace(requestRun.UserMessageID)
	}
	msg, err := s.deps.Memory.SaveMessage(ctx, memory.Message{
		ConversationID: requestRun.ConversationID,
		Role:           "assistant",
		Content:        strings.TrimSpace(result.Message),
		Model:          firstNonEmptyString(result.ToolName, request.ToolName, "yemaka-executor"),
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
	_, _ = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
		ConversationID:     requestRun.ConversationID,
		SessionID:          requestRun.SessionID,
		UserMessageID:      requestRun.UserMessageID,
		AssistantMessageID: msg.ID,
		ParentMessageID:    requestRun.ParentMessageID,
		VariantIndex:       msg.VariantIndex,
		ToolName:           "permission_result",
		Input:              map[string]any{"request_id": request.RequestID, "tool_name": request.ToolName},
		Output:             output,
		Status:             firstNonEmptyString(result.ToolStatus, result.Status),
		RiskLevel:          request.RiskLevel,
	})
	return result, nil
}

func (s *Server) applyApprovedEditPermission(ctx context.Context, request agent.PermissionRequest) (PermissionDecisionResult, bool, error) {
	proposal, ok, err := agent.StoredReadyEditProposalForPermission(ctx, s.deps.Memory, request)
	if err != nil || !ok {
		return PermissionDecisionResult{}, ok, err
	}
	applied, err := s.applyFileWrite(ctx, FileWriteInput{Path: proposal.Path, Content: proposal.Content, Approved: true})
	if err != nil {
		return failedEditPermissionResult(request, err), true, nil
	}
	return appliedEditPermissionResult(request, applied), true, nil
}

func failedEditPermissionResult(request agent.PermissionRequest, err error) PermissionDecisionResult {
	tool := normalizeToolNameForPermissionResult(request.ToolName)
	friendlyError := friendlyFileWritePermissionError(err)
	return PermissionDecisionResult{
		Status:     "approved",
		ToolName:   tool,
		ToolStatus: "failed",
		Executed:   false,
		Message:    fmt.Sprintf("Approval received, but `%s` could not run: %s", tool, friendlyError),
		Result: &agent.ExecutionResult{
			Context:    friendlyError,
			SourceKind: "tool",
			Status:     "failed",
		},
	}
}

func friendlyFileWritePermissionError(err error) string {
	raw := strings.TrimSpace(err.Error())
	if raw == "" {
		return "file write failed"
	}
	const marker = "workspace path is not granted:"
	if strings.Contains(raw, marker) {
		rest := strings.TrimSpace(strings.SplitN(raw, marker, 2)[1])
		target := strings.TrimSpace(rest)
		if beforeHint, _, ok := strings.Cut(target, ";"); ok {
			target = strings.TrimSpace(beforeHint)
		}
		if target != "" {
			return fmt.Sprintf("workspace access is not granted for `%s`. Grant that folder in Workspace permissions, then retry the approved edit.", target)
		}
		return "workspace access is not granted for that path. Grant the folder in Workspace permissions, then retry the approved edit."
	}
	return raw
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

func (s *Server) planFileWrite(ctx context.Context, input FileWriteInput) (FileWritePlanResult, error) {
	if err := validateFileWriteInput(input, s.deps.Config); err != nil {
		return FileWritePlanResult{}, err
	}
	target, err := s.resolveFileWriteTarget(strings.TrimSpace(input.Path))
	if err != nil {
		return FileWritePlanResult{}, err
	}
	defer target.close()
	plan, err := workspace.PlanWrite(ctx, target.Root, target.Path, input.Content, fileWriteOptions(s.deps.Config, s.deps.Profile))
	if err != nil {
		return FileWritePlanResult{}, err
	}
	result := fileWritePlanResult(plan)
	result.Path = target.displayPath(plan.Path)
	return result, nil
}

func (s *Server) applyFileWrite(ctx context.Context, input FileWriteInput) (FileWriteApplyResult, error) {
	if !input.Approved {
		return FileWriteApplyResult{}, fmt.Errorf("approved flag is required before applying file writes")
	}
	if err := validateFileWriteInput(input, s.deps.Config); err != nil {
		return FileWriteApplyResult{}, err
	}
	options := fileWriteOptions(s.deps.Config, s.deps.Profile)
	target, err := s.resolveFileWriteTarget(strings.TrimSpace(input.Path))
	if err != nil {
		_, _ = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
			ToolName:  "write_file",
			Input:     map[string]any{"path": input.Path},
			Output:    map[string]any{"error": err.Error()},
			Status:    "blocked",
			RiskLevel: "medium",
		})
		return FileWriteApplyResult{}, err
	}
	defer target.close()
	plan, err := workspace.PlanWrite(ctx, target.Root, target.Path, input.Content, options)
	if err != nil {
		_, _ = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
			ToolName:  "write_file",
			Input:     map[string]any{"path": input.Path},
			Output:    map[string]any{"error": err.Error()},
			Status:    "blocked",
			RiskLevel: "medium",
		})
		return FileWriteApplyResult{}, err
	}
	applied, err := workspace.ApplyWrite(ctx, target.Root, plan, options)
	if err != nil {
		_, _ = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
			ToolName:  "write_file",
			Input:     map[string]any{"path": target.displayPath(plan.Path), "bytes": plan.TargetBytes},
			Output:    map[string]any{"error": err.Error()},
			Status:    "failed",
			RiskLevel: "medium",
		})
		return FileWriteApplyResult{}, err
	}
	verification := workspace.VerifyAppliedWrite(ctx, target.Root, applied, options)
	writeStatus := "completed"
	if verification.Status != "pass" {
		writeStatus = verification.Status
	}
	_, _ = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
		ToolName:  "write_file",
		Input:     map[string]any{"path": target.displayPath(applied.Path), "bytes": applied.TargetBytes},
		Output:    map[string]any{"snapshot_id": applied.SnapshotID, "changed": applied.Changed, "verification_status": verification.Status},
		Status:    writeStatus,
		RiskLevel: "medium",
	})
	_, _ = s.deps.Memory.SaveToolRun(ctx, memory.ToolRun{
		ToolName:  "write_verifier",
		Input:     map[string]any{"path": target.displayPath(applied.Path), "snapshot_id": applied.SnapshotID},
		Output:    verification,
		Status:    verification.Status,
		RiskLevel: "medium",
	})
	result := fileWriteApplyResult(applied, verification)
	result.Path = target.displayPath(applied.Path)
	return result, nil
}

type fileWriteTarget struct {
	Root        string
	Path        string
	DisplayPath string
	access      *safety.WorkspaceAccess
}

func (t fileWriteTarget) close() {
	if t.access != nil {
		t.access.Close()
	}
}

func (t fileWriteTarget) displayPath(relativePath string) string {
	if strings.TrimSpace(t.DisplayPath) != "" {
		return t.DisplayPath
	}
	return relativePath
}

func (s *Server) resolveFileWriteTarget(path string) (fileWriteTarget, error) {
	normalizedInput := safety.NormalizeUserSuppliedPath(strings.TrimSpace(path))
	effectivePath := s.workspaceRootPath(path)
	accessPath, err := existingFileWriteAccessPath(effectivePath)
	if err != nil {
		return fileWriteTarget{}, err
	}
	access, err := safety.RequireWorkspaceAccessWithBookmark(s.workspaceBase(), accessPath, s.workspaceGrantStore())
	if err != nil {
		return fileWriteTarget{}, err
	}
	root := s.workspaceBase()
	if access != nil && access.Source != "current_workspace" {
		root = strings.TrimSpace(access.Grant.Path)
		if root == "" {
			root = filepath.Dir(accessPath)
		}
	}
	displayPath := ""
	if filepath.IsAbs(normalizedInput) {
		displayPath = effectivePath
	}
	return fileWriteTarget{
		Root:        root,
		Path:        effectivePath,
		DisplayPath: displayPath,
		access:      access,
	}, nil
}

func existingFileWriteAccessPath(path string) (string, error) {
	path = filepath.Clean(safety.NormalizeUserSuppliedPath(strings.TrimSpace(path)))
	if path == "" || path == "." {
		return "", fmt.Errorf("file path is required")
	}
	if _, err := os.Stat(path); err == nil {
		return path, nil
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("stat file path: %w", err)
	}
	parent := filepath.Dir(path)
	for parent != "" && parent != "." {
		info, err := os.Stat(parent)
		if err == nil {
			if !info.IsDir() {
				return "", fmt.Errorf("file parent is not a directory: %s", parent)
			}
			return parent, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat file parent: %w", err)
		}
		next := filepath.Dir(parent)
		if next == parent {
			break
		}
		parent = next
	}
	return "", fmt.Errorf("file parent folder does not exist: %s", filepath.Dir(path))
}

func (s *Server) planQARegressionSourceWrite(ctx context.Context, input learning.QARegressionSourceWriteRequest) (QARegressionSourceWritePlanResult, error) {
	plans, err := s.qaRegressionSourceWriteApplyPlans()
	if err != nil {
		return QARegressionSourceWritePlanResult{}, err
	}
	planRecord, err := learning.ValidateQARegressionSourceWrite(plans, input)
	if err != nil {
		return QARegressionSourceWritePlanResult{}, err
	}
	filePlan, err := s.planFileWrite(ctx, FileWriteInput{Path: planRecord.SuggestedTestFile, Content: input.Content})
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

func (s *Server) applyQARegressionSourceWrite(ctx context.Context, input learning.QARegressionSourceWriteRequest) (QARegressionSourceWriteApplyResult, error) {
	if !input.Approved {
		return QARegressionSourceWriteApplyResult{}, fmt.Errorf("source test write requires explicit approval")
	}
	plans, err := s.qaRegressionSourceWriteApplyPlans()
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	planRecord, err := learning.ValidateQARegressionSourceWrite(plans, input)
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	applied, err := s.applyFileWrite(ctx, FileWriteInput{Path: planRecord.SuggestedTestFile, Content: input.Content, Approved: true})
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	recordDir, err := s.qaRegressionSourceWriteDir()
	if err != nil {
		return QARegressionSourceWriteApplyResult{}, err
	}
	record, err := learning.RecordQARegressionSourceWrite(recordDir, planRecord, input, learning.QARegressionSourceWriteOutcome{
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

func (s *Server) qaRegressionSourceWriteApplyPlans() ([]learning.QARegressionSourcePatchApplyPlanRecord, error) {
	applyPlanDir, err := s.qaRegressionSourcePatchApplyPlanDir()
	if err != nil {
		return nil, err
	}
	return learning.ListQARegressionSourcePatchApplyPlans(applyPlanDir)
}

func (s *Server) setupState(ctx context.Context) SetupState {
	cfg := s.deps.Config
	state := SetupState{
		SetupComplete:      cfg.App.SetupComplete,
		ConfigPath:         cfg.Path,
		ProfilePath:        s.deps.Profile.Root,
		LowMemoryMode:      cfg.Runtime.LowMemoryMode,
		Mode:               setupModeFromConfig(cfg),
		SelectedModel:      selectedModel(cfg).Name,
		InstalledModels:    []models.ModelInfo{},
		RecommendedMode:    "student_laptop",
		ConfiguredDefaults: configuredModelStatuses(cfg, nil),
	}
	if err := s.deps.Runtime.Health(ctx); err != nil {
		state.OllamaError = err.Error()
		return state
	}
	state.OllamaOK = true
	installed, err := s.deps.Runtime.ListModels(ctx)
	if err != nil {
		state.OllamaError = err.Error()
		return state
	}
	localModels := localInstalledModels(installed)
	state.InstalledModels = localModels
	state.RecommendedModel = recommendInstalledModel(localModels)
	if state.RecommendedModel == "" && len(localModels) > 0 {
		state.RecommendedModel = localModels[0].Name
	}
	state.ModelReady = modelInstalled(state.SelectedModel, localModels)
	state.ConfiguredDefaults = configuredModelStatuses(cfg, localModels)
	return state
}

func (s *Server) completeSetup(ctx context.Context, input SetupInput) (SetupState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.deps.Config
	mode := strings.TrimSpace(input.Mode)
	if mode == "" {
		mode = "student_laptop"
	}
	if mode != "student_laptop" && mode != "useful_local" {
		return SetupState{}, fmt.Errorf("setup mode must be student_laptop or useful_local")
	}

	installed, _ := s.deps.Runtime.ListModels(ctx)
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

	cfg.App.SetupComplete = true
	cfg.App.Telemetry = false
	cfg.Runtime.LowMemoryMode = mode == "student_laptop"
	cfg.Runtime.MaxParallelTools = 1
	cfg.Runtime.MaxContextTokens = 4096
	if mode == "useful_local" {
		cfg.Runtime.MaxContextTokens = 6144
	}
	cfg.RAG.Enabled = true
	cfg.RAG.Mode = "sqlite_fts"
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.MaxParallelCommands = 1
	cfg.Security.AllowNetworkByDefault = false
	cfg.Security.SecretsAccess = false

	if lowMemoryModel != "" {
		model := cfg.Models["low_memory"]
		model.Provider = "ollama"
		model.Name = lowMemoryModel
		if model.BaseURL == "" {
			model.BaseURL = "http://localhost:11434/api"
		}
		cfg.Models["low_memory"] = model
	}
	if defaultModel != "" {
		model := cfg.Models["default"]
		model.Provider = "ollama"
		model.Name = defaultModel
		if model.BaseURL == "" {
			model.BaseURL = "http://localhost:11434/api"
		}
		cfg.Models["default"] = model
	}
	if err := config.Write(cfg.Path, cfg); err != nil {
		return SetupState{}, err
	}
	s.deps.Router = models.NewRouter(cfg)
	runtime, err := modelruntime.New(selectedModel(cfg))
	if err != nil {
		return SetupState{}, err
	}
	s.deps.Runtime = runtime
	return s.setupState(ctx), nil
}

func (s *Server) setModelRole(role string, name string, provider string, baseURL string) error {
	role = normalizeModelRole(role)
	if !knownModelRole(role) {
		return fmt.Errorf("unknown model role %q", role)
	}
	if name == "" {
		return fmt.Errorf("model name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	model := s.deps.Config.Models[role]
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
	s.deps.Config.Models[role] = model
	if err := config.Write(s.deps.Config.Path, s.deps.Config); err != nil {
		return err
	}
	s.deps.Router = models.NewRouter(s.deps.Config)
	runtime, err := modelruntime.New(selectedModel(s.deps.Config))
	if err != nil {
		return err
	}
	s.deps.Runtime = runtime
	return nil
}

func (s *Server) applyModelProfile(role string, name string) (modelprofiles.Profile, string, error) {
	role = normalizeModelRole(role)
	if !knownModelRole(role) {
		return modelprofiles.Profile{}, "", fmt.Errorf("unknown model role %q", role)
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return modelprofiles.Profile{}, "", fmt.Errorf("model profile name is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	store := s.modelProfileStore()
	profile, err := store.Load(name)
	if err != nil {
		return modelprofiles.Profile{}, "", err
	}
	model := s.deps.Config.Models[role]
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
	s.deps.Config.Models[role] = model
	if err := config.Write(s.deps.Config.Path, s.deps.Config); err != nil {
		return modelprofiles.Profile{}, "", err
	}
	s.deps.Router = models.NewRouter(s.deps.Config)
	runtime, err := modelruntime.New(selectedModel(s.deps.Config))
	if err != nil {
		return modelprofiles.Profile{}, "", err
	}
	s.deps.Runtime = runtime
	return profile, modelProfilePath(store.Dir, profile.Name), nil
}

func (s *Server) chat(ctx context.Context, content string, conversationID string) (ChatResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return ChatResult{}, fmt.Errorf("message is required")
	}
	result := ChatResult{}
	service := &agent.Service{
		Router:           s.deps.Router,
		Runtime:          s.deps.Runtime,
		RuntimeFactory:   s.deps.RuntimeFactory,
		Memory:           s.deps.Memory,
		CloudFallback:    serverCloudFallback(s.deps.Config, s.deps.Cloud),
		ToolExecutor:     serverToolExecutor(s.deps),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: s.deps.Config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(s.deps.Config, s.extensionStore()),
		ReplayStore:      s.replayStore(),
		ModelProfileDir:  s.modelProfileStore().Dir,
		PromptOptions:    agent.PromptOptionsFromConfig(s.deps.Config),
		PolicyMode:       safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode),
		InternetTools:    agent.InternetToolLoopEnabled(s.deps.Config, safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)),
		InternetSearch:   agent.InternetSearchLoopEnabled(s.deps.Config, safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)),
	}
	err := service.Chat(ctx, agent.ChatInput{Content: content, ConversationID: conversationID}, func(event agent.Event) error {
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

func (s *Server) ask(ctx context.Context, content string, skillName string, conversationID string, parentMessageID string, attachmentIDs []string) (AskResult, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return AskResult{}, fmt.Errorf("request is required")
	}
	selectedSkill, hasSkill, err := s.selectSkill(skillName, content)
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
		Router:           s.deps.Router,
		Runtime:          s.deps.Runtime,
		RuntimeFactory:   s.deps.RuntimeFactory,
		Memory:           s.deps.Memory,
		CloudFallback:    serverCloudFallback(s.deps.Config, s.deps.Cloud),
		ToolExecutor:     serverToolExecutor(s.deps),
		Retriever:        serverRetriever(s.deps),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: s.deps.Config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(s.deps.Config, s.extensionStore()),
		ReplayStore:      s.replayStore(),
		ModelProfileDir:  s.modelProfileStore().Dir,
		PromptOptions:    agent.PromptOptionsFromConfig(s.deps.Config),
		PolicyMode:       safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode),
		InternetTools:    agent.InternetToolLoopEnabled(s.deps.Config, safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)),
		InternetSearch:   agent.InternetSearchLoopEnabled(s.deps.Config, safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)),
	}
	err = service.Ask(ctx, agent.AskInput{
		ConversationID:     conversationID,
		ParentMessageID:    parentMessageID,
		Content:            content,
		AttachmentIDs:      attachmentIDs,
		SkillName:          selectedSkill.Name,
		SkillVersion:       selectedSkill.Version,
		SkillRequiredTools: selectedSkill.RequiredTools,
		SkillInstructions:  skillInstructions,
	}, func(event agent.Event) error {
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
		result.Attachments = s.chatAttachmentsForMessage(ctx, result.UserMessageID)
	}
	return result, err
}

func (s *Server) streamAsk(w http.ResponseWriter, ctx context.Context, content string, skillName string, conversationID string, parentMessageID string, attachmentIDs []string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("request is required")
	}
	selectedSkill, hasSkill, err := s.selectSkill(skillName, content)
	if err != nil {
		return err
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming is not supported")
	}
	skillInstructions := ""
	if hasSkill {
		skillInstructions = skills.PromptBlock(selectedSkill)
	}
	result := AskResult{}
	if hasSkill {
		result.Skill = selectedSkill.Name
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	clientDisconnected := false
	send := func(event AskStreamEvent) error {
		if clientDisconnected {
			return nil
		}
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		eventName := strings.TrimSpace(event.Type)
		if eventName == "" {
			eventName = "message"
		}
		if _, err := fmt.Fprintf(w, "event: %s\n", eventName); err != nil {
			clientDisconnected = true
			return nil
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			clientDisconnected = true
			return nil
		}
		flusher.Flush()
		return nil
	}
	serviceCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	unregister := s.registerActiveAskCancel(cancel)
	defer func() {
		unregister()
		cancel()
	}()

	service := &agent.Service{
		Router:           s.deps.Router,
		Runtime:          s.deps.Runtime,
		RuntimeFactory:   s.deps.RuntimeFactory,
		Memory:           s.deps.Memory,
		CloudFallback:    serverCloudFallback(s.deps.Config, s.deps.Cloud),
		ToolExecutor:     serverToolExecutor(s.deps),
		Retriever:        serverRetriever(s.deps),
		KnowledgeContext: agent.NewLocalKnowledgeContextProvider(agent.LocalKnowledgeContextConfig{Config: s.deps.Config}),
		CapabilityGap:    agent.NewCapabilityGapRouter(s.deps.Config, s.extensionStore()),
		ReplayStore:      s.replayStore(),
		ModelProfileDir:  s.modelProfileStore().Dir,
		PromptOptions:    agent.PromptOptionsFromConfig(s.deps.Config),
		PolicyMode:       safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode),
		InternetTools:    agent.InternetToolLoopEnabled(s.deps.Config, safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)),
		InternetSearch:   agent.InternetSearchLoopEnabled(s.deps.Config, safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)),
	}
	err = service.Ask(serviceCtx, agent.AskInput{
		ConversationID:     conversationID,
		ParentMessageID:    parentMessageID,
		Content:            content,
		AttachmentIDs:      attachmentIDs,
		SkillName:          selectedSkill.Name,
		SkillVersion:       selectedSkill.Version,
		SkillRequiredTools: selectedSkill.RequiredTools,
		SkillInstructions:  skillInstructions,
	}, func(event agent.Event) error {
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
		return send(AskStreamEvent{
			Type:    event.Type,
			Token:   event.Token,
			Message: event.Message,
			Data:    event.Data,
		})
	})
	if err != nil {
		if clientDisconnected {
			return nil
		}
		friendly := agent.FriendlyErrorMessage(err)
		_ = send(AskStreamEvent{
			Type:    "agent.error",
			Message: friendly,
			Error:   friendly,
			Data:    map[string]string{"detail": err.Error()},
		})
		return nil
	}
	result.Attachments = s.chatAttachmentsForMessage(context.WithoutCancel(ctx), result.UserMessageID)
	return send(AskStreamEvent{Type: "result", Result: &result})
}

func (s *Server) registerActiveAskCancel(cancel context.CancelFunc) func() {
	if cancel == nil {
		return func() {}
	}
	s.mu.Lock()
	s.activeAskID++
	id := s.activeAskID
	s.activeAskCancel = cancel
	s.mu.Unlock()
	return func() {
		s.mu.Lock()
		if s.activeAskID == id {
			s.activeAskCancel = nil
		}
		s.mu.Unlock()
	}
}

func (s *Server) selectSkill(explicit string, content string) (skills.Skill, bool, error) {
	available := availableSkillTools()
	if explicit != "" {
		skill, ok := s.deps.Skills.Get(explicit)
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
	skill, ok := s.deps.Skills.Select(content, available)
	return skill, ok, nil
}

func (s *Server) reloadSkills() error {
	registry, err := loadSkillRegistryForProfile(s.deps.Profile)
	if err != nil {
		return err
	}
	s.deps.Skills = registry
	return nil
}

func (s *Server) domainPackRoot() (string, error) {
	if s.deps.Profile == nil {
		return "", fmt.Errorf("profile is required")
	}
	return filepath.Join(s.deps.Profile.Root, "domain_packs"), nil
}

func loadSkillRegistryForProfile(profile *profiles.Profile) (skills.Registry, error) {
	if profile == nil {
		return skills.Registry{}, fmt.Errorf("profile is required")
	}
	enabledPackSkillDirs, err := domainpacks.EnabledSkillDirs(filepath.Join(profile.Root, "domain_packs"))
	if err != nil {
		return skills.Registry{}, err
	}
	return skills.LoadProfileRegistry("skills/default", profile.Skills, enabledPackSkillDirs)
}

func (s *Server) extensionStore() *extensions.Store {
	store := extensions.NewStore(s.deps.Profile.GeneratedExtensions, s.deps.Profile.Logs)
	store.PolicyMode = safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)
	return store
}

func (s *Server) generatedConnectorStore() *connectors.GeneratedStore {
	root := filepath.Join(s.deps.Profile.Root, "connectors", "generated")
	store := connectors.NewGeneratedStore(root)
	store.PolicyMode = safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)
	return store
}

func (s *Server) replayStore() agent.ReplayReadWriter {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return nil
	}
	return replay.NewStore(filepath.Join(s.deps.Profile.Root, "replay"))
}

func (s *Server) replayTraceStore() replay.Store {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return replay.Store{}
	}
	return replay.NewStore(filepath.Join(s.deps.Profile.Root, "replay"))
}

func (s *Server) qaRegressionPromotionDir() (string, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return "", fmt.Errorf("profile root is required")
	}
	return filepath.Join(s.deps.Profile.Root, "qa_review", "regression_promotions"), nil
}

func (s *Server) qaRegressionReviewDir() (string, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return "", fmt.Errorf("profile root is required")
	}
	return filepath.Join(s.deps.Profile.Root, "qa_review", "regression_reviews"), nil
}

func (s *Server) qaRegressionTestDraftDir() (string, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return "", fmt.Errorf("profile root is required")
	}
	return filepath.Join(s.deps.Profile.Root, "qa_review", "regression_test_drafts"), nil
}

func (s *Server) qaRegressionApprovalDir() (string, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return "", fmt.Errorf("profile root is required")
	}
	return filepath.Join(s.deps.Profile.Root, "qa_review", "approved_regressions"), nil
}

func (s *Server) qaRegressionSourcePatchDir() (string, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return "", fmt.Errorf("profile root is required")
	}
	return filepath.Join(s.deps.Profile.Root, "qa_review", "regression_source_patches"), nil
}

func (s *Server) qaRegressionSourcePatchApplyPlanDir() (string, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return "", fmt.Errorf("profile root is required")
	}
	return filepath.Join(s.deps.Profile.Root, "qa_review", "regression_source_patch_apply_plans"), nil
}

func (s *Server) qaRegressionSourceWriteDir() (string, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Root) == "" {
		return "", fmt.Errorf("profile root is required")
	}
	return filepath.Join(s.deps.Profile.Root, "qa_review", "regression_source_writes"), nil
}

func (s *Server) notificationStore() (notifications.Store, error) {
	if s == nil || s.deps.Profile == nil || strings.TrimSpace(s.deps.Profile.Notifications) == "" {
		return notifications.Store{}, fmt.Errorf("notification store is not configured")
	}
	return notifications.NewStore(s.deps.Profile.Notifications), nil
}

func (s *Server) internetService() *internet.Service {
	service := internet.New(s.deps.Config.Internet, s.deps.Profile.Root, s.deps.Profile.Logs)
	service.PolicyMode = safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)
	return service
}

func (s *Server) schedulerStore(ctx context.Context) (*scheduler.Store, error) {
	return scheduler.Open(ctx, s.deps.Profile.Database)
}

func (s *Server) jobRunner() scheduler.Runner {
	return scheduler.RunnerFunc(func(ctx context.Context, job scheduler.Job) (any, error) {
		if err := scheduler.ValidateTarget(job.TargetType, job.TargetName); err != nil {
			return nil, err
		}
		switch job.TargetType {
		case scheduler.TargetExtension:
			if err := s.extensionStore().ValidateRunInput(job.TargetName, job.Input); err != nil {
				return nil, err
			}
			result, err := s.extensionStore().Run(ctx, job.TargetName, extensions.RunOptions{
				Input:             job.Input,
				MaxRuntimeSeconds: s.deps.Config.Scheduler.DefaultJobTimeoutSeconds,
				Internet:          s.internetService(),
			})
			if err != nil {
				return result, err
			}
			return result, nil
		case scheduler.TargetHeartbeat:
			report, err := s.heartbeatReport(ctx)
			if err != nil {
				return report, err
			}
			return report, nil
		default:
			return nil, fmt.Errorf("unsupported job target type: %s", job.TargetType)
		}
	})
}

func (s *Server) heartbeatReport(ctx context.Context) (heartbeat.Report, error) {
	store, err := s.schedulerStore(ctx)
	if err != nil {
		return heartbeat.Report{}, err
	}
	defer store.Close()
	schedulerStatus, err := store.Status(ctx, s.deps.Config.Scheduler.Enabled, s.schedulerMaxParallel())
	if err != nil {
		return heartbeat.Report{}, err
	}
	return heartbeat.Run(ctx, heartbeat.Input{
		Config:          s.deps.Config,
		Profile:         s.deps.Profile,
		Memory:          s.deps.Memory,
		Runtime:         s.deps.Runtime,
		SchedulerStatus: &schedulerStatus,
		ExtensionStore:  s.extensionStore(),
	}), nil
}

func (s *Server) jobRunOptions() scheduler.RunOptions {
	return scheduler.RunOptions{
		Enabled:        s.deps.Config.Scheduler.Enabled,
		TimeoutSeconds: s.deps.Config.Scheduler.DefaultJobTimeoutSeconds,
		MaxParallel:    s.schedulerMaxParallel(),
		LowMemoryMode:  s.deps.Config.Runtime.LowMemoryMode,
		PolicyMode:     safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode),
	}
}

func (s *Server) schedulerMaxParallel() int {
	maxParallel := s.deps.Config.Scheduler.MaxParallelJobs
	if s.deps.Config.Runtime.LowMemoryMode && s.deps.Config.Scheduler.LowMemoryMaxParallelJobs > 0 {
		maxParallel = s.deps.Config.Scheduler.LowMemoryMaxParallelJobs
	}
	if maxParallel <= 0 {
		return 1
	}
	return maxParallel
}

func (s *Server) createSkillFromSession(ctx context.Context, conversationID string) (skills.Skill, error) {
	messages, err := s.deps.Memory.ListConversationMessages(ctx, conversationID, s.deps.Config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := s.deps.Memory.ListToolRunsForConversation(ctx, conversationID, 50)
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
		ProfileSkillsDir: s.deps.Profile.Skills,
		ConversationID:   conversationID,
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func (s *Server) improveSkillFromSession(ctx context.Context, name string, conversationID string) (skills.Skill, error) {
	skill, ok := s.deps.Skills.Get(strings.TrimSpace(name))
	if !ok {
		return skills.Skill{}, fmt.Errorf("skill not found: %s", name)
	}
	messages, err := s.deps.Memory.ListConversationMessages(ctx, conversationID, s.deps.Config.Memory.SummarizeAfter)
	if err != nil {
		return skills.Skill{}, err
	}
	toolRuns, err := s.deps.Memory.ListToolRunsForConversation(ctx, conversationID, 50)
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
		ProfileSkillsDir: s.deps.Profile.Skills,
		ConversationID:   conversationID,
		Skill:            skill,
		Messages:         transcript,
		ToolRuns:         runs,
	})
}

func validateLoopbackAddr(addr string) error {
	host := addr
	if strings.Contains(addr, ":") {
		var port string
		host, port, _ = strings.Cut(addr, ":")
		if strings.TrimSpace(port) == "" {
			return fmt.Errorf("serve address must include a port")
		}
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "", "localhost", "127.0.0.1", "::1":
		return nil
	default:
		return fmt.Errorf("serve address must be loopback-only for local-first mode")
	}
}

func localOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func (s *Server) startFrontendWatcher(ctx context.Context) {
	if !s.deps.FrontendWatch {
		return
	}
	root := s.frontendRoot()
	if root == "" {
		s.logDev("frontend watch disabled: frontend root not found")
		return
	}
	if _, err := os.Stat(filepath.Join(root, "package.json")); err != nil {
		s.logDev("frontend watch disabled: %v", err)
		return
	}
	s.setFrontendBuildState(fmt.Sprintf("%d", time.Now().UnixNano()), "")
	if frontendNeedsBuild(root, s.deps.StaticDir) {
		s.logDev("frontend dist is missing or stale; running npm build")
		s.runFrontendBuild(ctx, root)
	}
	go s.watchFrontend(ctx, root)
}

func (s *Server) frontendRoot() string {
	if root := strings.TrimSpace(s.deps.FrontendRoot); root != "" {
		if _, err := os.Stat(root); err == nil {
			return root
		}
	}
	staticDir := strings.TrimSpace(s.deps.StaticDir)
	if filepath.Base(staticDir) == "dist" {
		root := filepath.Dir(staticDir)
		if _, err := os.Stat(root); err == nil {
			return root
		}
	}
	if _, err := os.Stat("frontend"); err == nil {
		return "frontend"
	}
	return ""
}

func (s *Server) watchFrontend(ctx context.Context, root string) {
	last, _ := frontendSnapshot(root)
	ticker := time.NewTicker(900 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			current, err := frontendSnapshot(root)
			if err != nil {
				s.setFrontendBuildState("", err.Error())
				continue
			}
			if current == last {
				continue
			}
			last = current
			time.Sleep(250 * time.Millisecond)
			s.logDev("frontend change detected; rebuilding dist")
			if s.runFrontendBuild(ctx, root) {
				if updated, err := frontendSnapshot(root); err == nil {
					last = updated
				}
			}
		}
	}
}

func (s *Server) runFrontendBuild(ctx context.Context, root string) bool {
	buildCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(buildCtx, "npm", "--prefix", root, "run", "build")
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		} else {
			message = err.Error() + ": " + message
		}
		s.setFrontendBuildState("", message)
		s.logDev("frontend build failed: %s", message)
		return false
	}
	s.setFrontendBuildState(fmt.Sprintf("%d", time.Now().UnixNano()), "")
	s.logDev("frontend build complete; browser will reload")
	return true
}

func (s *Server) setFrontendBuildState(version string, errText string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if version != "" {
		s.frontendVersion = version
	}
	s.frontendError = errText
}

func (s *Server) logDev(format string, args ...any) {
	if s.deps.DevLog == nil {
		return
	}
	fmt.Fprintf(s.deps.DevLog, "frontend: "+format+"\n", args...)
}

func frontendSnapshot(root string) (string, error) {
	paths := []string{
		filepath.Join(root, "index.html"),
		filepath.Join(root, "tailwind.config.cjs"),
		filepath.Join(root, "postcss.config.cjs"),
		filepath.Join(root, "vite.config.ts"),
	}
	var builder strings.Builder
	for _, path := range paths {
		appendFrontendSnapshot(&builder, path)
	}
	srcRoot := filepath.Join(root, "src")
	err := filepath.WalkDir(srcRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		appendFrontendSnapshot(&builder, path)
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return builder.String(), nil
}

func appendFrontendSnapshot(builder *strings.Builder, path string) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return
	}
	fmt.Fprintf(builder, "%s:%d:%d\n", path, info.ModTime().UnixNano(), info.Size())
}

func frontendNeedsBuild(root string, staticDir string) bool {
	index := filepath.Join(staticDir, "index.html")
	indexInfo, err := os.Stat(index)
	if err != nil || indexInfo.IsDir() {
		return true
	}
	indexTime := indexInfo.ModTime()
	paths := []string{
		filepath.Join(root, "index.html"),
		filepath.Join(root, "tailwind.config.cjs"),
		filepath.Join(root, "postcss.config.cjs"),
		filepath.Join(root, "vite.config.ts"),
	}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(indexTime) {
			return true
		}
	}
	stale := false
	_ = filepath.WalkDir(filepath.Join(root, "src"), func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || stale {
			return nil
		}
		if info, err := entry.Info(); err == nil && info.ModTime().After(indexTime) {
			stale = true
		}
		return nil
	})
	return stale
}

func selectedModel(cfg *config.Config) config.ModelConfig {
	selected := cfg.Models["default"]
	if cfg.Runtime.LowMemoryMode {
		if low, ok := cfg.Models["low_memory"]; ok {
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
	for _, role := range []string{"default", "low_memory", "coding", "reasoning", "stronger_local"} {
		model, ok := cfg.Models[role]
		if !ok {
			continue
		}
		status := ModelStatus{
			Role:      role,
			Name:      model.Name,
			Installed: modelInstalled(model.Name, installed),
		}
		if strings.TrimSpace(model.Profile) != "" {
			status.Profile = strings.TrimSpace(model.Profile)
			status.ProfileValid = true
			status.ProfileStatus = "applied"
		}
		statuses = append(statuses, status)
	}
	return statuses
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

func domainPackSkillSummaries(statuses []domainpacks.PackStatus) []DomainPackSkillSummary {
	summaries := make([]DomainPackSkillSummary, 0)
	for _, status := range statuses {
		for _, ref := range status.Skills {
			summaries = append(summaries, DomainPackSkillSummary{
				PackName:        status.Name,
				Ref:             ref,
				Active:          status.Enabled && status.Valid,
				Enabled:         status.Enabled,
				Valid:           status.Valid,
				ValidationError: status.ValidationError,
			})
		}
	}
	return summaries
}

func domainPackStatusByName(statuses []domainpacks.PackStatus, name string) *domainpacks.PackStatus {
	name = strings.TrimSpace(name)
	for _, status := range statuses {
		if status.Name == name {
			statusCopy := status
			return &statusCopy
		}
	}
	return nil
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

func maxDocumentUploadBytes(limits workspace.Limits) int64 {
	if limits.MaxFileBytes > 0 {
		return limits.MaxFileBytes
	}
	return 2 << 20
}

type chatAttachmentUploadData struct {
	FileName    string
	ContentType string
	Data        []byte
	Retention   string
}

func readChatAttachmentUpload(w http.ResponseWriter, r *http.Request, limits workspace.Limits) (chatAttachmentUploadData, error) {
	maxBytes := maxDocumentUploadBytes(limits)
	contentType := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if strings.HasPrefix(contentType, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes+(1<<20))
		if err := r.ParseMultipartForm(maxBytes + (1 << 20)); err != nil {
			return chatAttachmentUploadData{}, fmt.Errorf("parse chat attachment upload: %w", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return chatAttachmentUploadData{}, fmt.Errorf("attachment file is required")
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
		if err != nil {
			return chatAttachmentUploadData{}, fmt.Errorf("read chat attachment: %w", err)
		}
		if int64(len(data)) > maxBytes {
			return chatAttachmentUploadData{}, fmt.Errorf("attachment exceeds max read size")
		}
		name := header.Filename
		if relativePath := strings.TrimSpace(r.FormValue("relativePath")); relativePath != "" {
			name = relativePath
		}
		return chatAttachmentUploadData{
			FileName:    name,
			ContentType: header.Header.Get("Content-Type"),
			Data:        data,
			Retention:   r.FormValue("retention"),
		}, nil
	}

	var input ChatAttachmentUploadInput
	if err := readJSON(r, &input); err != nil {
		return chatAttachmentUploadData{}, err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input.DataBase64))
	if err != nil {
		return chatAttachmentUploadData{}, fmt.Errorf("decode attachment data: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return chatAttachmentUploadData{}, fmt.Errorf("attachment exceeds max read size")
	}
	return chatAttachmentUploadData{
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Data:        data,
		Retention:   input.Retention,
	}, nil
}

func (s *Server) workspaceBase() string {
	base := strings.TrimSpace(s.deps.Workspace)
	if base == "" {
		return "."
	}
	return base
}

func (s *Server) workspaceRootPath(path string) string {
	path = safety.NormalizeUserSuppliedPath(strings.TrimSpace(path))
	if path == "" || path == "." {
		return s.workspaceBase()
	}
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(s.workspaceBase(), path))
}

func (s *Server) workspaceGrantStore() safety.WorkspaceGrantStore {
	if s.deps.Profile == nil {
		return safety.WorkspaceGrantStore{}
	}
	return safety.NewWorkspaceGrantStore(safety.WorkspaceGrantsPath(s.deps.Profile.Permissions))
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

func (s *Server) relativeDocumentPathSuggestions(suggestions []DocumentPathSuggestionResult) []DocumentPathSuggestionResult {
	base, err := filepath.Abs(s.workspaceBase())
	if err != nil {
		return suggestions
	}
	for i := range suggestions {
		rel, relErr := filepath.Rel(base, suggestions[i].Path)
		if relErr != nil || rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			continue
		}
		suggestions[i].Path = filepath.ToSlash(rel)
		directory := filepath.Dir(rel)
		if directory == "." {
			directory = "."
		}
		suggestions[i].Directory = filepath.ToSlash(directory)
	}
	return suggestions
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

func toolSurfaceFromQuery(value string) tools.ToolSurface {
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

func rollupOpsStatus(stages []OpsStage) string {
	overall := heartbeat.StatusHealthy
	for _, stage := range stages {
		overall = worseOpsStatus(overall, stage.Status)
	}
	return overall
}

func worseOpsStatus(current string, next string) string {
	currentRank := opsStatusRank(current)
	nextRank := opsStatusRank(next)
	if nextRank > currentRank {
		return next
	}
	return current
}

func opsStatusRank(status string) int {
	switch status {
	case heartbeat.StatusBroken:
		return 5
	case heartbeat.StatusNeedsConfig, heartbeat.StatusNeedsAuth:
		return 4
	case heartbeat.StatusWarning:
		return 3
	case heartbeat.StatusDisabled:
		return 2
	case heartbeat.StatusHealthy, "":
		return 1
	default:
		return 3
	}
}

func notificationStageStatus(items []notifications.Notification) string {
	for _, item := range items {
		if item.Dismissed {
			continue
		}
		if item.Severity == notifications.SeverityError {
			return heartbeat.StatusWarning
		}
		if item.ActionRequired || item.Severity == notifications.SeverityWarning {
			return heartbeat.StatusWarning
		}
	}
	return heartbeat.StatusHealthy
}

func evalStageStatus(summary *learning.QAEvalSummary) string {
	if summary == nil {
		return heartbeat.StatusWarning
	}
	switch summary.Status {
	case evaluation.StatusFail:
		return heartbeat.StatusBroken
	case evaluation.StatusSkip:
		return heartbeat.StatusWarning
	default:
		return heartbeat.StatusHealthy
	}
}

func evalStageDetails(summary *learning.QAEvalSummary) string {
	if summary == nil {
		return "no latest eval report"
	}
	return fmt.Sprintf("%s: %d passed, %d failed, %d skipped", summary.Status, summary.Passed, summary.Failed, summary.Skipped)
}

func qaReviewStageStatus(summary learning.QAReviewSummary) string {
	if summary.ApprovedRegressions > 0 || summary.TestDrafts > 0 {
		return heartbeat.StatusWarning
	}
	if summary.RegressionSuggestions > 0 || summary.ReplayFailures > 0 || summary.PendingCorrections > 0 {
		return heartbeat.StatusWarning
	}
	return heartbeat.StatusHealthy
}

func qaReviewStageDetails(summary learning.QAReviewSummary) string {
	return fmt.Sprintf("%d failures, %d suggestions, %d drafts, %d approved regressions", summary.ReplayFailures, summary.RegressionSuggestions, summary.TestDrafts, summary.ApprovedRegressions)
}

func replayStageStatus(traces []replay.TraceFile) string {
	if len(traces) == 0 {
		return heartbeat.StatusWarning
	}
	return heartbeat.StatusHealthy
}

func toolRunStageStatus(runs []memory.ToolRun) string {
	for _, run := range runs {
		status := strings.ToLower(strings.TrimSpace(run.Status))
		if status == "failed" || status == "error" || status == "blocked" {
			return heartbeat.StatusWarning
		}
	}
	return heartbeat.StatusHealthy
}

func policyAuditStageStatus(records []safety.PolicyAuditRecord) string {
	for _, record := range records {
		if !record.Decision.Allowed {
			return heartbeat.StatusWarning
		}
		if record.Decision.RequiresConfirmation {
			return heartbeat.StatusWarning
		}
	}
	return heartbeat.StatusHealthy
}

func connectorOpsSummary(items []connectors.Status) ConnectorOpsSummary {
	summary := ConnectorOpsSummary{
		Total:  len(items),
		Status: heartbeat.StatusHealthy,
	}
	for _, item := range items {
		healthStatus := strings.ToLower(strings.TrimSpace(item.Health.Status))
		if item.Enabled {
			summary.Enabled++
			summary.EnabledList = append(summary.EnabledList, item.Name)
			if item.RequireToken && !item.TokenReady {
				summary.NeedsAuth++
			}
		} else {
			summary.Disabled++
		}
		switch healthStatus {
		case heartbeat.StatusBroken:
			summary.Broken++
		case heartbeat.StatusWarning, heartbeat.StatusNeedsConfig, heartbeat.StatusNeedsAuth:
			summary.Warning++
		}
	}
	sort.Strings(summary.EnabledList)
	switch {
	case summary.Broken > 0:
		summary.Status = heartbeat.StatusBroken
	case summary.NeedsAuth > 0:
		summary.Status = heartbeat.StatusNeedsAuth
	case summary.Warning > 0:
		summary.Status = heartbeat.StatusWarning
	case summary.Enabled == 0:
		summary.Status = heartbeat.StatusDisabled
	default:
		summary.Status = heartbeat.StatusHealthy
	}
	return summary
}

func connectorOpsStageDetails(summary ConnectorOpsSummary) string {
	return fmt.Sprintf("%d connectors, %d enabled, %d disabled, %d need auth", summary.Total, summary.Enabled, summary.Disabled, summary.NeedsAuth)
}

func knowledgeOpsStageStatus(status KnowledgeStatus) string {
	if !status.Enabled {
		return heartbeat.StatusDisabled
	}
	return heartbeat.StatusHealthy
}

func modelProfileOpsSummary(cfg *config.Config, store modelprofiles.Store) (ModelProfileOpsSummary, error) {
	summary := ModelProfileOpsSummary{Status: heartbeat.StatusHealthy}
	profiles, err := store.List()
	if err != nil {
		return summary, err
	}
	summary.ProfileCount = len(profiles)
	available := map[string]bool{}
	for _, profile := range profiles {
		name := strings.TrimSpace(profile.Name)
		if name == "" {
			continue
		}
		available[name] = true
		summary.AvailableProfiles = append(summary.AvailableProfiles, name)
	}
	sort.Strings(summary.AvailableProfiles)
	if cfg == nil {
		return summary, nil
	}
	for role, model := range cfg.Models {
		role = strings.TrimSpace(role)
		if role == "" {
			continue
		}
		summary.ConfiguredRoles++
		profileName := strings.TrimSpace(model.Profile)
		if profileName == "" {
			continue
		}
		if available[profileName] {
			summary.AppliedRoles++
			continue
		}
		summary.MissingBindings++
		summary.MissingProfiles = append(summary.MissingProfiles, fmt.Sprintf("%s:%s", role, profileName))
	}
	sort.Strings(summary.MissingProfiles)
	if summary.MissingBindings > 0 {
		summary.Status = heartbeat.StatusWarning
	}
	return summary, nil
}

func modelProfileOpsStageDetails(summary ModelProfileOpsSummary) string {
	if summary.MissingBindings > 0 {
		return fmt.Sprintf("%d profiles, %d applied roles, %d missing bindings", summary.ProfileCount, summary.AppliedRoles, summary.MissingBindings)
	}
	return fmt.Sprintf("%d profiles, %d applied roles", summary.ProfileCount, summary.AppliedRoles)
}

func extensionAuditStageStatus(records []extensions.AuditRecord) string {
	if len(records) == 0 {
		return heartbeat.StatusHealthy
	}
	for _, record := range records {
		status := strings.ToLower(strings.TrimSpace(record.Status))
		if status == "failed" || status == "error" || status == "blocked" {
			return heartbeat.StatusWarning
		}
	}
	return heartbeat.StatusHealthy
}

func buildOpsTimeline(status OpsStatus, qaFailures []learning.QAReplayFailure, limit int) []OpsTimelineEvent {
	if limit <= 0 {
		limit = 20
	}
	toolRunLinks := opsToolRunLinkIndex(status.RecentToolRuns)
	extensionLinks := opsExtensionLinkIndex(status.ExtensionAudit)
	policyLinks := opsPolicyAuditLinkIndex(status.PolicyAudit)
	events := []OpsTimelineEvent{}
	for _, item := range status.Notifications {
		events = append(events, OpsTimelineEvent{
			ID:         opsEventID("notification", item.ID),
			OccurredAt: item.CreatedAt,
			Source:     "notifications",
			Kind:       firstNonEmptyString(item.Type, "notification"),
			Severity:   string(item.Severity),
			Status:     opsNotificationStatus(item),
			Title:      item.Title,
			Summary:    opsSummary(item.Message, 180),
			EntityType: "notification",
			EntityID:   item.ID,
			Metadata:   opsStringMetadata(item.Metadata),
		})
	}
	for _, run := range status.RecentJobRuns {
		events = append(events, OpsTimelineEvent{
			ID:            opsEventID("job_run", run.ID),
			OccurredAt:    firstNonEmptyString(run.FinishedAt, run.StartedAt),
			Source:        "scheduler",
			Kind:          "job_run",
			Severity:      opsSeverityForRunStatus(run.Status),
			Status:        run.Status,
			Title:         opsJobRunTitle(run),
			Summary:       opsSummary(run.Error, 180),
			EntityType:    "job_run",
			EntityID:      run.ID,
			CorrelationID: run.JobID,
			Metadata: opsMetadata(map[string]string{
				"jobId":   run.JobID,
				"attempt": strconv.Itoa(run.Attempt),
			}),
		})
	}
	if status.Heartbeat.GeneratedAt != "" {
		nonHealthy := 0
		for _, check := range status.Heartbeat.Checks {
			if check.Status != heartbeat.StatusHealthy {
				nonHealthy++
				events = append(events, OpsTimelineEvent{
					ID:         opsEventID("heartbeat", check.ID, check.CheckedAt),
					OccurredAt: firstNonEmptyString(check.CheckedAt, status.Heartbeat.GeneratedAt),
					Source:     "heartbeat",
					Kind:       "heartbeat_check",
					Severity:   opsSeverityForHealthStatus(check.Status),
					Status:     check.Status,
					Title:      check.Name,
					Summary:    opsSummary(check.Detail, 180),
					EntityType: "heartbeat_check",
					EntityID:   check.ID,
				})
			}
		}
		if nonHealthy == 0 {
			events = append(events, OpsTimelineEvent{
				ID:         opsEventID("heartbeat", "overall", status.Heartbeat.GeneratedAt),
				OccurredAt: status.Heartbeat.GeneratedAt,
				Source:     "heartbeat",
				Kind:       "heartbeat_report",
				Severity:   opsSeverityForHealthStatus(status.Heartbeat.Overall),
				Status:     status.Heartbeat.Overall,
				Title:      "Heartbeat healthy",
				Summary:    fmt.Sprintf("%d checks passed", len(status.Heartbeat.Checks)),
				EntityType: "heartbeat_report",
				EntityID:   "overall",
			})
		}
	}
	if status.LatestEval != nil {
		events = append(events, OpsTimelineEvent{
			ID:         opsEventID("eval", status.LatestEval.ID, status.LatestEval.CompletedAt),
			OccurredAt: firstNonEmptyString(status.LatestEval.CompletedAt, status.LatestEval.StartedAt),
			Source:     "evaluation",
			Kind:       "eval_run",
			Severity:   opsSeverityForEvalStatus(status.LatestEval.Status),
			Status:     status.LatestEval.Status,
			Title:      "Evaluation run",
			Summary:    fmt.Sprintf("%d passed, %d failed, %d skipped", status.LatestEval.Passed, status.LatestEval.Failed, status.LatestEval.Skipped),
			EntityType: "eval",
			EntityID:   status.LatestEval.ID,
		})
	}
	for _, failure := range qaFailures {
		events = append(events, OpsTimelineEvent{
			ID:            opsEventID("qa", failure.TraceID, failure.RouteFailureCategory),
			OccurredAt:    status.GeneratedAt,
			Source:        "qa_review",
			Kind:          firstNonEmptyString(failure.RouteFailureCategory, "replay_failure"),
			Severity:      "warning",
			Status:        firstNonEmptyString(failure.CoverageStatus, "needs_review"),
			Title:         "QA review finding",
			Summary:       opsSummary(firstNonEmptyString(failure.RouteFailureReason, failure.SuggestedRegression, failure.FailureMessage), 220),
			EntityType:    "replay_trace",
			EntityID:      failure.TraceID,
			CorrelationID: failure.PromotionFingerprint,
			Metadata: opsMetadata(map[string]string{
				"route":        failure.RouteCategory,
				"category":     failure.RouteFailureCategory,
				"reviewStatus": failure.ReviewStatus,
			}),
		})
	}
	for _, trace := range status.RecentReplay {
		events = append(events, OpsTimelineEvent{
			ID:         opsEventID("replay", trace.ID),
			Source:     "replay",
			Kind:       "trace",
			Severity:   "info",
			Status:     "recorded",
			Title:      "Replay trace recorded",
			Summary:    trace.Filename,
			EntityType: "replay_trace",
			EntityID:   trace.ID,
		})
	}
	for _, run := range status.RecentToolRuns {
		events = append(events, OpsTimelineEvent{
			ID:            opsEventID("tool", run.ID),
			OccurredAt:    firstNonEmptyString(run.CompletedAt, run.CreatedAt),
			Source:        "audit",
			Kind:          "tool_run",
			Severity:      opsSeverityForRunStatus(run.Status),
			Status:        run.Status,
			Title:         run.ToolName,
			Summary:       opsSummary(run.RiskLevel, 120),
			EntityType:    "tool_run",
			EntityID:      run.ID,
			CorrelationID: firstNonEmptyString(run.ConversationID, run.SessionID),
			Metadata: opsMetadata(map[string]string{
				"conversationId":     run.ConversationID,
				"userMessageId":      run.UserMessageID,
				"assistantMessageId": run.AssistantMessageID,
				"risk":               run.RiskLevel,
				"relatedPolicyAudit": opsRelatedIDs(policyLinks[opsCorrelationKey("tool", run.ToolName)], 5),
			}),
			Related: opsTimelineLinks(policyLinks[opsCorrelationKey("tool", run.ToolName)], 5),
		})
	}
	for _, record := range status.PolicyAudit {
		events = append(events, OpsTimelineEvent{
			ID:            firstNonEmptyString(record.ID, opsEventID("policy", record.Timestamp, record.Request.Actor, record.Request.Resource)),
			OccurredAt:    record.Timestamp,
			Source:        "policy_audit",
			Kind:          firstNonEmptyString(strings.Trim(record.Request.Domain+"."+record.Request.Action, "."), "policy_decision"),
			Severity:      opsSeverityForPolicyDecision(record.Decision),
			Status:        opsPolicyDecisionStatus(record.Decision),
			Title:         opsPolicyAuditTitle(record),
			Summary:       opsSummary(firstNonEmptyString(record.Decision.Explanation, record.Decision.Reason), 180),
			EntityType:    "policy_decision",
			EntityID:      firstNonEmptyString(record.ID, record.Request.Resource, record.Request.Actor, record.Request.Domain),
			CorrelationID: opsPolicyCorrelationID(record),
			Metadata: opsMetadata(map[string]string{
				"actor":             record.Request.Actor,
				"resource":          record.Request.Resource,
				"domain":            record.Request.Domain,
				"action":            record.Request.Action,
				"risk":              record.Decision.Risk,
				"level":             record.Decision.Level.String(),
				"policyMode":        record.Request.PolicyMode,
				"line":              strconv.Itoa(record.Line),
				"relatedToolRuns":   opsRelatedIDs(toolRunLinks[opsCorrelationKey("tool", record.Request.Resource)], 5),
				"relatedExtensions": opsRelatedIDs(extensionLinks[opsCorrelationKey("extension", record.Request.Resource)], 5),
			}),
			Related: opsTimelineLinks(
				append(
					append([]OpsTimelineLink{}, toolRunLinks[opsCorrelationKey("tool", record.Request.Resource)]...),
					extensionLinks[opsCorrelationKey("extension", record.Request.Resource)]...,
				),
				5,
			),
		})
	}
	for _, record := range status.ExtensionAudit {
		events = append(events, OpsTimelineEvent{
			ID:         opsEventID("extension", record.Action, record.Name, record.Timestamp),
			OccurredAt: record.Timestamp,
			Source:     "extensions",
			Kind:       firstNonEmptyString(record.Action, "extension_audit"),
			Severity:   opsSeverityForExtensionAudit(record.Status),
			Status:     record.Status,
			Title:      opsExtensionAuditTitle(record),
			Summary:    opsSummary(record.Message, 180),
			EntityType: "extension",
			EntityID:   record.Name,
			Metadata: opsMetadata(map[string]string{
				"action":             record.Action,
				"relatedPolicyAudit": opsRelatedIDs(policyLinks[opsCorrelationKey("extension", record.Name)], 5),
			}),
			Related: opsTimelineLinks(policyLinks[opsCorrelationKey("extension", record.Name)], 5),
		})
	}
	sort.SliceStable(events, func(i, j int) bool {
		left := events[i].OccurredAt
		right := events[j].OccurredAt
		if left == "" && right != "" {
			return false
		}
		if right == "" && left != "" {
			return true
		}
		if left != right {
			return left > right
		}
		return events[i].ID < events[j].ID
	})
	if len(events) > limit {
		events = events[:limit]
	}
	return events
}

func opsToolRunLinkIndex(runs []ToolRunResult) map[string][]OpsTimelineLink {
	index := map[string][]OpsTimelineLink{}
	for _, run := range runs {
		link := OpsTimelineLink{Source: "audit", EntityType: "tool_run", EntityID: run.ID, Label: run.ToolName}
		opsIndexTimelineLink(index, opsCorrelationKey("tool", run.ToolName), link)
		opsIndexTimelineLink(index, opsCorrelationKey("conversation", run.ConversationID), link)
		opsIndexTimelineLink(index, opsCorrelationKey("session", run.SessionID), link)
		opsIndexTimelineLink(index, opsCorrelationKey("message", run.UserMessageID), link)
	}
	return index
}

func opsExtensionLinkIndex(records []extensions.AuditRecord) map[string][]OpsTimelineLink {
	index := map[string][]OpsTimelineLink{}
	for _, record := range records {
		id := opsEventID("extension", record.Action, record.Name, record.Timestamp)
		link := OpsTimelineLink{Source: "extensions", EntityType: "extension", EntityID: record.Name, Label: firstNonEmptyString(record.Action, id)}
		opsIndexTimelineLink(index, opsCorrelationKey("extension", record.Name), link)
	}
	return index
}

func opsPolicyAuditLinkIndex(records []safety.PolicyAuditRecord) map[string][]OpsTimelineLink {
	index := map[string][]OpsTimelineLink{}
	for _, record := range records {
		id := firstNonEmptyString(record.ID, opsEventID("policy", record.Timestamp, record.Request.Actor, record.Request.Resource))
		label := strings.Trim(record.Request.Domain+"."+record.Request.Action, ".")
		link := OpsTimelineLink{Source: "policy_audit", EntityType: "policy_decision", EntityID: id, Label: firstNonEmptyString(label, record.Request.Resource)}
		opsIndexTimelineLink(index, opsCorrelationKey("tool", record.Request.Resource), link)
		opsIndexTimelineLink(index, opsCorrelationKey("extension", record.Request.Resource), link)
		if strings.HasPrefix(record.Request.Actor, "tool:") {
			opsIndexTimelineLink(index, opsCorrelationKey("tool", strings.TrimPrefix(record.Request.Actor, "tool:")), link)
		}
		if strings.HasPrefix(record.Request.Actor, "extension:") {
			opsIndexTimelineLink(index, opsCorrelationKey("extension", strings.TrimPrefix(record.Request.Actor, "extension:")), link)
		}
	}
	return index
}

func opsIndexTimelineLink(index map[string][]OpsTimelineLink, key string, link OpsTimelineLink) {
	if key == "" || strings.TrimSpace(link.EntityID) == "" {
		return
	}
	for _, existing := range index[key] {
		if existing.Source == link.Source && existing.EntityType == link.EntityType && existing.EntityID == link.EntityID {
			return
		}
	}
	index[key] = append(index[key], link)
}

func opsCorrelationKey(kind string, value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(kind)) + ":" + value
}

func opsPolicyCorrelationID(record safety.PolicyAuditRecord) string {
	if strings.HasPrefix(record.Request.Actor, "tool:") {
		return firstNonEmptyString(strings.TrimPrefix(record.Request.Actor, "tool:"), record.Request.Resource)
	}
	if strings.HasPrefix(record.Request.Actor, "extension:") {
		return strings.TrimPrefix(record.Request.Actor, "extension:")
	}
	if record.Request.Resource != "" {
		return record.Request.Resource
	}
	return firstNonEmptyString(record.Request.Actor, record.Request.Domain)
}

func opsRelatedIDs(links []OpsTimelineLink, limit int) string {
	links = opsTimelineLinks(links, limit)
	ids := make([]string, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.EntityID)
	}
	return strings.Join(ids, ", ")
}

func opsTimelineLinks(links []OpsTimelineLink, limit int) []OpsTimelineLink {
	if limit <= 0 {
		limit = len(links)
	}
	out := make([]OpsTimelineLink, 0, len(links))
	seen := map[string]bool{}
	for _, link := range links {
		key := link.Source + "\x00" + link.EntityType + "\x00" + link.EntityID
		if strings.TrimSpace(link.EntityID) == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, link)
		if len(out) >= limit {
			break
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func opsEventID(parts ...string) string {
	clean := []string{"ops"}
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

func opsNotificationStatus(item notifications.Notification) string {
	if item.Dismissed {
		return "dismissed"
	}
	if item.Read {
		return "read"
	}
	return "unread"
}

func opsJobRunTitle(run scheduler.JobRun) string {
	switch run.Status {
	case scheduler.StatusFailed, scheduler.StatusTimeout:
		return "Scheduled job failed"
	case scheduler.StatusCompleted:
		return "Scheduled job completed"
	default:
		return "Scheduled job run"
	}
}

func opsSeverityForRunStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case scheduler.StatusFailed, scheduler.StatusTimeout, "error":
		return "error"
	case "blocked":
		return "warning"
	case scheduler.StatusCompleted, "success":
		return "success"
	default:
		return "info"
	}
}

func opsSeverityForHealthStatus(status string) string {
	switch status {
	case heartbeat.StatusBroken:
		return "error"
	case heartbeat.StatusWarning, heartbeat.StatusNeedsAuth, heartbeat.StatusNeedsConfig:
		return "warning"
	case heartbeat.StatusDisabled:
		return "info"
	default:
		return "success"
	}
}

func opsSeverityForEvalStatus(status string) string {
	switch status {
	case evaluation.StatusFail:
		return "error"
	case evaluation.StatusSkip:
		return "warning"
	default:
		return "success"
	}
}

func opsSeverityForPolicyDecision(decision safety.PolicyDecision) string {
	if !decision.Allowed {
		return "warning"
	}
	if decision.RequiresConfirmation {
		return "warning"
	}
	switch strings.ToLower(strings.TrimSpace(decision.Risk)) {
	case safety.PolicyRiskHigh:
		return "warning"
	case safety.PolicyRiskMedium:
		return "info"
	default:
		return "success"
	}
}

func opsPolicyDecisionStatus(decision safety.PolicyDecision) string {
	if !decision.Allowed {
		return "blocked"
	}
	if decision.RequiresConfirmation {
		return "confirmation_required"
	}
	return "allowed"
}

func opsPolicyAuditTitle(record safety.PolicyAuditRecord) string {
	domain := strings.TrimSpace(record.Request.Domain)
	action := strings.TrimSpace(record.Request.Action)
	if domain == "" && action == "" {
		return "Policy decision"
	}
	if action == "" {
		return fmt.Sprintf("Policy decision: %s", domain)
	}
	if domain == "" {
		return fmt.Sprintf("Policy decision: %s", action)
	}
	return fmt.Sprintf("Policy decision: %s %s", domain, action)
}

func opsSeverityForExtensionAudit(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "blocked":
		return "warning"
	case "ok", "passed", "success", "registered", "enabled", "disabled":
		return "success"
	default:
		return "info"
	}
}

func opsExtensionAuditTitle(record extensions.AuditRecord) string {
	action := strings.TrimSpace(record.Action)
	if action == "" {
		action = "extension"
	}
	name := strings.TrimSpace(record.Name)
	if name == "" {
		return "Extension audit"
	}
	return fmt.Sprintf("Extension %s: %s", action, name)
}

func opsStringMetadata(values map[string]string) map[string]string {
	return opsMetadata(values)
}

func opsMetadata(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = opsSummary(value, 160)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func opsSummary(value string, limit int) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 1 {
		return value[:limit]
	}
	if limit <= 3 {
		return value[:limit]
	}
	return strings.TrimSpace(value[:limit-3]) + "..."
}

func releaseHasWarnings(checks []release.Check) bool {
	for _, check := range checks {
		if check.Status == release.StatusWarn {
			return true
		}
	}
	return false
}

func conversationResults(conversations []memory.Conversation) []ConversationResult {
	result := make([]ConversationResult, 0, len(conversations))
	for _, conversation := range conversations {
		result = append(result, conversationResult(conversation))
	}
	return result
}

func conversationResult(conversation memory.Conversation) ConversationResult {
	return ConversationResult{
		ID:        conversation.ID,
		Title:     conversation.Title,
		CreatedAt: conversation.CreatedAt,
		UpdatedAt: conversation.UpdatedAt,
		Starred:   conversation.Starred,
	}
}

func conversationMessageResults(messages []memory.Message, activities map[string]agent.MessageActivity, turns []memory.AgentTurn) []ConversationMessageResult {
	return conversationMessageResultsWithAttachments(messages, activities, turns, nil)
}

func conversationMessageResultsWithAttachments(messages []memory.Message, activities map[string]agent.MessageActivity, turns []memory.AgentTurn, attachmentsByMessage map[string][]ChatAttachmentResult) []ConversationMessageResult {
	messages = memory.WithInferredResponseParents(messages)
	turnsByUser := renderableAgentTurnsByUser(messages, turns)
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
			result = append(result, conversationAgentTurnPlaceholder(turn))
		}
		delete(turnsByUser, message.ID)
	}
	return result
}

func (s *Server) chatAttachmentsForMessage(ctx context.Context, messageID string) []ChatAttachmentResult {
	itemsByMessage, err := s.deps.Memory.ListChatAttachmentsForMessages(ctx, []string{messageID})
	if err != nil {
		return nil
	}
	return chatAttachmentResults(itemsByMessage[messageID])
}

func (s *Server) chatAttachmentsForMessages(ctx context.Context, messages []memory.Message) (map[string][]ChatAttachmentResult, error) {
	messageIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.ID != "" {
			messageIDs = append(messageIDs, message.ID)
		}
	}
	itemsByMessage, err := s.deps.Memory.ListChatAttachmentsForMessages(ctx, messageIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]ChatAttachmentResult, len(itemsByMessage))
	for messageID, items := range itemsByMessage {
		out[messageID] = chatAttachmentResults(items)
	}
	return out, nil
}

func chatAttachmentResults(items []memory.ChatAttachment) []ChatAttachmentResult {
	out := make([]ChatAttachmentResult, 0, len(items))
	for _, item := range items {
		out = append(out, chatAttachmentResult(item))
	}
	return out
}

func chatAttachmentResult(item memory.ChatAttachment) ChatAttachmentResult {
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

func renderableAgentTurnsByUser(messages []memory.Message, turns []memory.AgentTurn) map[string][]memory.AgentTurn {
	answeredUserIDs := make(map[string]bool)
	for _, message := range messages {
		if message.Role == "assistant" && strings.TrimSpace(message.ParentID) != "" {
			answeredUserIDs[message.ParentID] = true
		}
	}
	turnsByUser := make(map[string][]memory.AgentTurn)
	for _, turn := range turns {
		if !shouldRenderAgentTurnPlaceholder(turn) || answeredUserIDs[turn.UserMessageID] {
			continue
		}
		turnsByUser[turn.UserMessageID] = append(turnsByUser[turn.UserMessageID], turn)
	}
	return turnsByUser
}

func shouldRenderAgentTurnPlaceholder(turn memory.AgentTurn) bool {
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

func conversationAgentTurnPlaceholder(turn memory.AgentTurn) ConversationMessageResult {
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

func serverCloudFallback(cfg *config.Config, runtime models.Runtime) *agent.CloudFallback {
	if cfg == nil || !cfg.CloudFallback.Enabled || runtime == nil {
		return nil
	}
	return &agent.CloudFallback{
		Config:  cfg.CloudFallback,
		Runtime: runtime,
	}
}

func serverToolExecutor(deps Dependencies) agent.ToolExecutor {
	cfg := deps.Config
	if cfg == nil {
		return nil
	}
	internetService := internet.New(cfg.Internet, deps.Profile.Root, deps.Profile.Logs)
	internetService.PolicyMode = safety.NormalizePolicyMode(cfg.Security.Policy.Mode)
	return agent.NewSafeToolExecutor(agent.SafeToolConfig{
		WorkspaceRoot:            deps.Workspace,
		TimeoutSeconds:           cfg.Tools.Shell.TimeoutSeconds,
		MaxOutputBytes:           cfg.Tools.Shell.MaxOutputBytes,
		MaxContextChars:          cfg.Workspace.MaxContextChars,
		RequireConfirmationRisky: cfg.Tools.Shell.RequireConfirmationRisky,
		FullAccess:               safety.IsFullAccessMode(cfg.Security.Policy.Mode),
		DisableShellCommands:     !cfg.Tools.Shell.Enabled,
		Config:                   cfg,
		Profile:                  deps.Profile,
		Memory:                   deps.Memory,
		RAG:                      deps.RAG,
		Runtime:                  deps.Runtime,
		Skills:                   deps.Skills,
		Internet:                 internetService,
		WorkspaceGrants:          safety.NewWorkspaceGrantStore(safety.WorkspaceGrantsPath(deps.Profile.Permissions)),
		UseShellHelper:           true,
		ShellHelperPath:          tools.DefaultShellHelperPath(),
	})
}

func serverRetriever(deps Dependencies) agent.Retriever {
	if deps.Config == nil {
		return nil
	}
	return agent.NewLocalRetriever(agent.LocalRetrieverConfig{
		Config:        deps.Config,
		WorkspaceRoot: deps.Workspace,
		RAG:           deps.RAG,
		Runtime:       deps.Runtime,
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

func oneLine(input string) string {
	input = strings.Join(strings.Fields(input), " ")
	if len(input) > 280 {
		return input[:280] + "..."
	}
	return input
}

func intParam(r *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	return atoiDefault(raw, fallback)
}

func boolParam(r *http.Request, name string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(r.URL.Query().Get(name)))
	switch raw {
	case "":
		return fallback
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func atoiDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func readJSON(r *http.Request, out any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(out)
}

func readConnectorJSON(w http.ResponseWriter, r *http.Request, cfg config.ConnectorConfig, out any) error {
	limit := cfg.MaxBodyBytes
	if limit <= 0 {
		limit = 65536
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	return readJSON(r, out)
}

func (s *Server) readAdapterInput(w http.ResponseWriter, r *http.Request, adapter string) (connectors.AdapterInput, error) {
	cfg, ok := connectors.AdapterConfig(s.deps.Config.Connectors, adapter)
	if !ok {
		return connectors.AdapterInput{}, fmt.Errorf("unknown adapter connector: %s", adapter)
	}
	limit := cfg.MaxBodyBytes
	if limit <= 0 {
		limit = 65536
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return connectors.AdapterInput{}, err
	}
	return connectors.ParseAdapterInput(adapter, r.Header.Get("Content-Type"), body)
}

func (s *Server) localConnectorAllowed(w http.ResponseWriter, r *http.Request, scope string) bool {
	status := connectors.LocalAPIStatus(s.deps.Config.Connectors)
	if !status.Enabled {
		writeError(w, http.StatusNotFound, fmt.Errorf("local_api connector is disabled"))
		return false
	}
	token, ok := connectors.Token(s.deps.Config.Connectors.LocalAPI)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("local_api connector token env is not set: %s", status.TokenEnv))
		return false
	}
	if status.RequireToken {
		got := strings.TrimSpace(r.Header.Get("Authorization"))
		if got == "" {
			got = strings.TrimSpace(r.Header.Get("X-Yemaka-Connector-Token"))
		}
		want := "Bearer " + token
		if got != want && got != token {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("local_api connector token is required"))
			return false
		}
	}
	if !s.connectorActionAllowed(w, r, connectors.LocalAPI, status, scope) {
		return false
	}
	return true
}

func (s *Server) adapterConnectorAllowed(w http.ResponseWriter, r *http.Request, adapter string, scope string) bool {
	if !connectors.IsAdapter(adapter) {
		writeError(w, http.StatusNotFound, fmt.Errorf("unknown adapter connector: %s", adapter))
		return false
	}
	status := connectors.AdapterStatus(s.deps.Config.Connectors, adapter)
	if !status.Enabled {
		writeError(w, http.StatusNotFound, fmt.Errorf("%s connector is disabled", adapter))
		return false
	}
	cfg, _ := connectors.AdapterConfig(s.deps.Config.Connectors, adapter)
	if cfg.TokenEnv == "" {
		cfg.TokenEnv = status.TokenEnv
	}
	cfg.RequireToken = status.RequireToken
	token, ok := connectors.Token(cfg)
	if !ok {
		writeError(w, http.StatusServiceUnavailable, fmt.Errorf("%s connector token env is not set: %s", adapter, status.TokenEnv))
		return false
	}
	if status.RequireToken {
		got := strings.TrimSpace(r.Header.Get("Authorization"))
		if got == "" {
			got = strings.TrimSpace(r.Header.Get("X-Yemaka-Connector-Token"))
		}
		if got == "" {
			got = strings.TrimSpace(r.Header.Get(adapterTokenHeader(adapter)))
		}
		want := "Bearer " + token
		if got != want && got != token {
			writeError(w, http.StatusUnauthorized, fmt.Errorf("%s connector token is required", adapter))
			return false
		}
	}
	if !s.connectorActionAllowed(w, r, adapter, status, scope) {
		return false
	}
	return true
}

func (s *Server) generatedConnectorAllowed(w http.ResponseWriter, r *http.Request, name string, scope string) (connectors.GeneratedEntry, connectors.Status, bool) {
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusNotFound, fmt.Errorf("generated connector name is required"))
		return connectors.GeneratedEntry{}, connectors.Status{}, false
	}
	store := s.generatedConnectorStore()
	entry, err := store.Get(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return connectors.GeneratedEntry{}, connectors.Status{}, false
	}
	policyMode := safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode)
	status, err := connectors.StatusFromGeneratedEntryWithPolicy(entry, policyMode)
	if err != nil {
		_ = store.RecordRun(name, "blocked", err.Error())
		writeError(w, http.StatusBadRequest, err)
		return entry, connectors.Status{}, false
	}
	if !entry.Enabled || !status.Enabled {
		_ = store.RecordRun(name, "blocked", "generated connector is disabled")
		writeError(w, http.StatusNotFound, fmt.Errorf("%s generated connector is disabled", name))
		return entry, status, false
	}
	if strings.TrimSpace(r.URL.Query().Get("token")) != "" {
		_ = store.RecordRun(name, "blocked", "query token rejected")
		writeError(w, http.StatusUnauthorized, fmt.Errorf("generated connector token must be sent in an authorization header"))
		return entry, status, false
	}
	token, tokenReady := generatedConnectorToken(r.Context(), entry.Manifest.Auth)
	if status.RequireToken {
		if !tokenReady {
			_ = store.RecordRun(name, "blocked", "token reference not ready")
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("%s generated connector token reference is not ready", name))
			return entry, status, false
		}
		got := strings.TrimSpace(r.Header.Get("Authorization"))
		if got == "" {
			got = strings.TrimSpace(r.Header.Get("X-Yemaka-Connector-Token"))
		}
		if got == "" {
			got = strings.TrimSpace(r.Header.Get("X-Yemaka-Generated-Connector-Token"))
		}
		want := "Bearer " + token
		if got != want && got != token {
			_ = store.RecordRun(name, "blocked", "token required")
			writeError(w, http.StatusUnauthorized, fmt.Errorf("%s generated connector token is required", name))
			return entry, status, false
		}
	}
	if !s.connectorActionAllowed(w, r, name, status, scope) {
		_ = store.RecordRun(name, "blocked", "policy, scope, or rate gate blocked run")
		return entry, status, false
	}
	return entry, status, true
}

func (s *Server) connectorActionAllowed(w http.ResponseWriter, r *http.Request, connector string, status connectors.Status, scope string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return true
	}
	if !connectorScopeAllowed(status.Permissions, scope) {
		writeError(w, http.StatusForbidden, fmt.Errorf("%s connector does not allow scope %q", connector, scope))
		return false
	}
	decision := safety.EvaluatePolicy(safety.PolicyRequest{
		Domain:       safety.DomainConnector,
		Action:       safety.ActionRead,
		Level:        safety.LevelReadOnly,
		PolicyMode:   safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode),
		Actor:        "connector:" + connector,
		Resource:     scope,
		Enabled:      status.Enabled,
		Mutating:     status.Permissions.Mutating,
		Posting:      status.Permissions.Posting,
		Trading:      status.Permissions.Trading,
		Deployment:   status.Permissions.Deployment,
		Network:      status.Permissions.Outbound,
		Domains:      status.AllowedDomains,
		TaskApproved: false,
	})
	if s.deps.Config != nil && s.deps.Config.Security.Policy.AuditEnabled && s.deps.Profile != nil {
		_ = safety.AppendPolicyAudit(s.deps.Profile.Logs, safety.PolicyRequest{
			Domain:       safety.DomainConnector,
			Action:       safety.ActionRead,
			Level:        safety.LevelReadOnly,
			PolicyMode:   safety.NormalizePolicyMode(s.deps.Config.Security.Policy.Mode),
			Actor:        "connector:" + connector,
			Resource:     scope,
			Enabled:      status.Enabled,
			Mutating:     status.Permissions.Mutating,
			Posting:      status.Permissions.Posting,
			Trading:      status.Permissions.Trading,
			Deployment:   status.Permissions.Deployment,
			Network:      status.Permissions.Outbound,
			Domains:      status.AllowedDomains,
			TaskApproved: false,
		}, decision)
	}
	if !decision.Allowed || decision.RequiresConfirmation {
		writeError(w, http.StatusForbidden, fmt.Errorf("%s connector blocked by policy: %s", connector, decision.Reason))
		return false
	}
	if !s.connectorRateAllowed(connector, scope, status.RateLimit) {
		writeError(w, http.StatusTooManyRequests, fmt.Errorf("%s connector rate limit exceeded for scope %q", connector, scope))
		return false
	}
	return true
}

func generatedConnectorToken(ctx context.Context, ref secrets.Reference) (string, bool) {
	ref = secrets.Normalize(ref)
	if ref.Provider == "" || ref.Provider == secrets.ProviderNone {
		return "", true
	}
	value, ok, err := secrets.Resolve(ctx, ref)
	if err != nil {
		return "", false
	}
	return value, ok
}

func readGeneratedConnectorInput(w http.ResponseWriter, r *http.Request, status connectors.Status) (map[string]any, error) {
	limit := status.MaxBodyBytes
	if limit <= 0 {
		limit = 65536
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)
	defer r.Body.Close()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(data)) == "" {
		return map[string]any{}, nil
	}
	var input map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return nil, err
	}
	if input == nil {
		input = map[string]any{}
	}
	return input, nil
}

func connectorScopeAllowed(perms config.ConnectorPermissionsConfig, scope string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" {
		return true
	}
	for _, allowed := range perms.Scopes {
		if strings.EqualFold(strings.TrimSpace(allowed), scope) {
			return true
		}
	}
	return false
}

func (s *Server) connectorRateAllowed(connector string, scope string, limits config.ConnectorRateLimitConfig) bool {
	if limits.RequestsPerMinute <= 0 && limits.RequestsPerDay <= 0 && limits.Burst <= 0 {
		return true
	}
	key := strings.TrimSpace(connector) + ":" + strings.TrimSpace(scope)
	now := time.Now().UTC()
	minuteWindow := now.Truncate(time.Minute)
	dayWindow := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	burstWindow := now.Truncate(time.Second)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.connectorRates == nil {
		s.connectorRates = map[string]connectorRateState{}
	}
	state := s.connectorRates[key]
	if state.MinuteWindowStart.IsZero() || !state.MinuteWindowStart.Equal(minuteWindow) {
		state.MinuteWindowStart = minuteWindow
		state.MinuteCount = 0
	}
	if state.DayWindowStart.IsZero() || !state.DayWindowStart.Equal(dayWindow) {
		state.DayWindowStart = dayWindow
		state.DayCount = 0
	}
	if state.BurstWindowStart.IsZero() || !state.BurstWindowStart.Equal(burstWindow) {
		state.BurstWindowStart = burstWindow
		state.BurstCount = 0
	}
	if limits.RequestsPerMinute > 0 && state.MinuteCount >= limits.RequestsPerMinute {
		s.connectorRates[key] = state
		return false
	}
	if limits.RequestsPerDay > 0 && state.DayCount >= limits.RequestsPerDay {
		s.connectorRates[key] = state
		return false
	}
	if limits.Burst > 0 && state.BurstCount >= limits.Burst {
		s.connectorRates[key] = state
		return false
	}
	if limits.RequestsPerMinute > 0 {
		state.MinuteCount++
	}
	if limits.RequestsPerDay > 0 {
		state.DayCount++
	}
	if limits.Burst > 0 {
		state.BurstCount++
	}
	s.connectorRates[key] = state
	return true
}

func adapterTokenHeader(adapter string) string {
	switch adapter {
	case connectors.Slack:
		return "X-Yemaka-Slack-Token"
	case connectors.Discord:
		return "X-Yemaka-Discord-Token"
	case connectors.Telegram:
		return "X-Yemaka-Telegram-Token"
	case connectors.Email:
		return "X-Yemaka-Email-Token"
	default:
		return "X-Yemaka-Connector-Token"
	}
}

func (s *Server) openEnabledKnowledgeStore(ctx context.Context) (*knowledge.Store, error) {
	if s.deps.Config == nil || !s.deps.Config.Knowledge.Enabled {
		return nil, fmt.Errorf("local knowledge graph is disabled; enable manual knowledge graph first")
	}
	if strings.TrimSpace(s.deps.Config.Memory.Database) == "" {
		return nil, fmt.Errorf("knowledge graph database is not configured")
	}
	return knowledge.OpenWithOptions(ctx, s.deps.Config.Memory.Database, knowledge.Options{
		MaxEvidenceChars: s.deps.Config.Knowledge.MaxEvidenceChars,
	})
}

func (s *Server) knowledgeStatus(ctx context.Context) (KnowledgeStatus, error) {
	if s.deps.Config == nil {
		return KnowledgeStatus{}, fmt.Errorf("config is not loaded")
	}
	status := KnowledgeStatus{
		Enabled:              s.deps.Config.Knowledge.Enabled,
		InfluenceEnabled:     s.deps.Config.Knowledge.InfluenceEnabled && s.deps.Config.Knowledge.Enabled,
		ManualOnly:           true,
		MaxEntitiesPerQuery:  s.deps.Config.Knowledge.MaxEntitiesPerQuery,
		MaxEvidenceChars:     s.deps.Config.Knowledge.MaxEvidenceChars,
		MaxInfluenceEntities: s.deps.Config.Knowledge.MaxInfluenceEntities,
		MaxInfluenceChars:    s.deps.Config.Knowledge.MaxInfluenceChars,
		Database:             s.deps.Config.Memory.Database,
		Message:              "manual-only local graph; disabled until explicitly enabled",
	}
	if !s.deps.Config.Knowledge.Enabled {
		return status, nil
	}
	store, err := knowledge.OpenWithOptions(ctx, s.deps.Config.Memory.Database, knowledge.Options{
		MaxEvidenceChars: s.deps.Config.Knowledge.MaxEvidenceChars,
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

func (s *Server) modelProfileStore() modelprofiles.Store {
	root := ""
	if s != nil && s.deps.Profile != nil {
		root = s.deps.Profile.Root
	}
	return modelprofiles.NewStore(filepath.Join(root, "model_profiles"))
}

func (s *Server) modelProfileBindingStatus(name string) (bool, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return false, "none"
	}
	if _, err := s.modelProfileStore().Load(name); err != nil {
		return false, "missing"
	}
	return true, "applied"
}

func modelProfileFromInput(input ModelProfileInput) modelprofiles.Profile {
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

func modelProfileResult(profile modelprofiles.Profile, path string, message string) (ModelProfileResult, error) {
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

func enrichModelProfileResult(store modelprofiles.Store, result *ModelProfileResult) {
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

func modelProfileDraftReport(report learning.ModelProfileDraftReport) *ModelProfileDraftReport {
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

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	_, _ = w.Write(data)
	_, _ = w.Write([]byte("\n"))
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}

func fallbackHTML() string {
	return `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <title>Yemaka Local Web</title>
    <style>
      body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 40px; color: #17201b; }
      code { background: #f3f4f6; padding: 2px 5px; border-radius: 4px; }
    </style>
  </head>
  <body>
    <h1>Yemaka Local Web</h1>
    <p>The Go core is serving locally. Build the Svelte UI with <code>npm --prefix frontend run build</code>, then refresh.</p>
    <p>API status is available at <code>/api/status</code>.</p>
  </body>
</html>`
}

func liveReloadScript() string {
	return `<script>
(() => {
  const key = 'yemakaFrontendLiveReload';
  if (window[key]) return;
  window[key] = true;
  let version = '';
  async function check() {
    try {
      const response = await fetch('/api/dev/frontend-version', { cache: 'no-store' });
      if (!response.ok) return;
      const data = await response.json();
      if (!version) {
        version = data.version || '';
        return;
      }
      if (data.version && data.version !== version) {
        window.location.reload();
      }
    } catch (_) {}
  }
  window.setInterval(check, 1000);
  check();
})();
</script>`
}
