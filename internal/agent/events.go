package agent

const (
	EventAgentStarted          = "agent.started"
	EventTaskClassified        = "task.classified"
	EventPlanCreated           = "plan.created"
	EventExecutionDecided      = "execution.decided"
	EventPermissionRequested   = "permission.requested"
	EventEditProposed          = "edit.proposed"
	EventToolCompleted         = "tool.completed"
	EventModelSelected         = "model.selected"
	EventModelToken            = "model.token"
	EventModelToolCall         = "model.tool_call"
	EventRoutingAdvisory       = "routing.advisory"
	EventCapabilityGap         = "capability.gap"
	EventSchedulerJobProposal  = "scheduler.job_proposal"
	EventRAGUsed               = "rag.used"
	EventWorkspaceUsed         = "workspace.used"
	EventMemoryUsed            = "memory.used"
	EventKnowledgeUsed         = "knowledge.used"
	EventSkillSelected         = "skill.selected"
	EventCloudFallback         = "cloud_fallback.used"
	EventVerificationCompleted = "verification.completed"
	EventMessageSaved          = "message.saved"
	EventAgentCompleted        = "agent.completed"
	EventAgentError            = "agent.error"
)

type Event struct {
	Type    string            `json:"type"`
	Token   string            `json:"token,omitempty"`
	Message string            `json:"message,omitempty"`
	Data    map[string]string `json:"data,omitempty"`
}

type EventHandler func(Event) error
