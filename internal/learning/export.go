package learning

import (
	"context"
	"fmt"
	"strings"
	"time"

	"yemaka/internal/memory"
)

const TrajectorySchema = "yemaka.trajectory.v1"

type Trajectory struct {
	Schema         string              `json:"schema"`
	ConversationID string              `json:"conversationId"`
	ExportedAt     string              `json:"exportedAt"`
	Messages       []TrajectoryMessage `json:"messages"`
	ToolRuns       []TrajectoryToolRun `json:"toolRuns"`
	PrivacyFilter  PrivacyFilterReport `json:"privacyFilter"`
	Training       TrainingInfo        `json:"training"`
}

type TrajectoryMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Model     string `json:"model,omitempty"`
	CreatedAt string `json:"createdAt"`
}

type TrajectoryToolRun struct {
	ToolName    string `json:"toolName"`
	Status      string `json:"status"`
	RiskLevel   string `json:"riskLevel,omitempty"`
	Input       any    `json:"input,omitempty"`
	Output      any    `json:"output,omitempty"`
	CreatedAt   string `json:"createdAt"`
	CompletedAt string `json:"completedAt"`
}

type TrainingInfo struct {
	AutomaticTraining bool   `json:"automaticTraining"`
	Note              string `json:"note"`
}

func ExportTrajectory(ctx context.Context, store *memory.Store, conversationID string) (Trajectory, error) {
	if store == nil {
		return Trajectory{}, fmt.Errorf("memory store is required")
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return Trajectory{}, fmt.Errorf("conversation id is required")
	}
	if _, err := store.GetConversation(ctx, conversationID); err != nil {
		return Trajectory{}, err
	}
	messages, err := store.ListConversationMessages(ctx, conversationID, 500)
	if err != nil {
		return Trajectory{}, err
	}
	toolRuns, err := store.ListToolRunsForConversation(ctx, conversationID, 500)
	if err != nil {
		return Trajectory{}, err
	}
	trajectory := Trajectory{
		Schema:         TrajectorySchema,
		ConversationID: conversationID,
		ExportedAt:     time.Now().UTC().Format(time.RFC3339),
		Training: TrainingInfo{
			AutomaticTraining: false,
			Note:              "Trajectory export is local-only and does not start training, uploading, or fine-tuning.",
		},
	}
	var filter PrivacyFilterReport
	for _, msg := range messages {
		content, report := SanitizeText(msg.Content)
		filter = mergeReports(filter, report)
		trajectory.Messages = append(trajectory.Messages, TrajectoryMessage{
			Role:      msg.Role,
			Content:   content,
			Model:     msg.Model,
			CreatedAt: msg.CreatedAt,
		})
	}
	for _, run := range toolRuns {
		input, inputReport := SanitizeValue(run.Input)
		output, outputReport := SanitizeValue(run.Output)
		filter = mergeReports(filter, inputReport)
		filter = mergeReports(filter, outputReport)
		trajectory.ToolRuns = append(trajectory.ToolRuns, TrajectoryToolRun{
			ToolName:    run.ToolName,
			Status:      run.Status,
			RiskLevel:   run.RiskLevel,
			Input:       input,
			Output:      output,
			CreatedAt:   run.CreatedAt,
			CompletedAt: run.CompletedAt,
		})
	}
	trajectory.PrivacyFilter = filter
	return trajectory, nil
}
