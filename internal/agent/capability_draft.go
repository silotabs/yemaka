package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"yemaka/internal/config"
	"yemaka/internal/extensions"
	"yemaka/internal/models"
)

const maxCapabilityDraftModelOutputBytes = 1024 * 1024
const maxCapabilityDraftRepairPromptBytes = 64 * 1024

type RuntimeFactory func(config.ModelConfig) (models.Runtime, error)

type CapabilityDraftRequest struct {
	Request        string
	Name           string
	Description    string
	Proposal       CapabilityGapProposal
	Model          config.ModelConfig
	RuntimeFactory RuntimeFactory
}

type CapabilityDraftRepairRequest struct {
	CapabilityDraftRequest
	PreviousDraft *extensions.PackageDraft
	Failure       string
	Attempt       int
}

func SelectCapabilityDraftModel(cfg *config.Config) (config.ModelConfig, error) {
	router := models.NewRouter(cfg)
	model, err := router.Select(models.TaskCoding)
	if err == nil && strings.TrimSpace(model.Name) != "" {
		return model, nil
	}
	if cfg != nil {
		if fallback, ok := cfg.Models["default"]; ok && strings.TrimSpace(fallback.Name) != "" {
			return fallback, nil
		}
	}
	if err != nil {
		return config.ModelConfig{}, err
	}
	return config.ModelConfig{}, fmt.Errorf("no local draft model configured")
}

func DraftCapabilityExtension(ctx context.Context, input CapabilityDraftRequest) (*extensions.PackageDraft, error) {
	return draftCapabilityExtension(ctx, input, capabilityDraftSystemPrompt(), capabilityDraftUserPrompt(input), "local_model_draft")
}

func RepairCapabilityExtensionDraft(ctx context.Context, input CapabilityDraftRepairRequest) (*extensions.PackageDraft, error) {
	if input.PreviousDraft == nil {
		return nil, fmt.Errorf("previous extension draft is required for repair")
	}
	if strings.TrimSpace(input.Failure) == "" {
		return nil, fmt.Errorf("generation failure summary is required for repair")
	}
	attempt := input.Attempt
	if attempt <= 0 {
		attempt = 1
	}
	return draftCapabilityExtension(ctx, input.CapabilityDraftRequest, capabilityDraftRepairSystemPrompt(), capabilityDraftRepairUserPrompt(input, attempt), fmt.Sprintf("local_model_repair_attempt_%d", attempt))
}

func draftCapabilityExtension(ctx context.Context, input CapabilityDraftRequest, systemPrompt string, userPrompt string, originSource string) (*extensions.PackageDraft, error) {
	if input.RuntimeFactory == nil {
		return nil, fmt.Errorf("local runtime factory is required for synthesized extension drafts")
	}
	if strings.TrimSpace(input.Model.Name) == "" {
		return nil, fmt.Errorf("local draft model is not configured")
	}
	if strings.TrimSpace(input.Name) == "" {
		return nil, fmt.Errorf("extension name is required for synthesized draft")
	}
	if strings.EqualFold(strings.TrimSpace(input.Proposal.NetworkMode), "core_broker") {
		return nil, fmt.Errorf("synthesized drafts are currently limited to local-only tool packages; use brokered generation for internet-backed capabilities")
	}
	runtime, err := input.RuntimeFactory(input.Model)
	if err != nil {
		return nil, fmt.Errorf("create local draft runtime: %w", err)
	}
	var builder strings.Builder
	req := models.NormalizeChatRequest(models.ChatRequest{
		Model:       input.Model.Name,
		Temperature: lowDraftTemperature(input.Model.Temperature),
		Messages: []models.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	})
	if err := runtime.ChatStream(ctx, req, func(event models.ChatEvent) error {
		if event.Token != "" {
			if builder.Len()+len(event.Token) > maxCapabilityDraftModelOutputBytes {
				return fmt.Errorf("synthesized draft exceeded %d bytes", maxCapabilityDraftModelOutputBytes)
			}
			builder.WriteString(event.Token)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("synthesize extension draft: %w", err)
	}
	draft, err := decodeCapabilityPackageDraft(builder.String())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(draft.Origin) == "" {
		milestone := "4.19"
		if originSource == "local_model_draft" {
			milestone = "4.18"
		}
		draft.Origin = fmt.Sprintf(
			"generated_by=yemaka\nmilestone=%s\nsource=%s\nmodel=%s\n",
			milestone,
			safeOriginValue(originSource),
			safeOriginValue(input.Model.Name),
		)
	}
	return draft, nil
}

func lowDraftTemperature(value float64) float64 {
	if value <= 0 || value > 0.2 {
		return 0.1
	}
	return value
}

func capabilityDraftSystemPrompt() string {
	return strings.TrimSpace(`You are drafting a profile-local Yemaka generated extension package.

Return JSON only. Do not use markdown fences unless unavoidable.
The JSON object must match:
{
  "origin": "optional short provenance string",
  "files": [
    {"path": "extension.yaml", "content": "..."},
    {"path": "README.md", "content": "..."},
    {"path": "go.mod", "content": "..."},
    {"path": "main.go", "content": "..."},
    {"path": "main_test.go", "content": "..."}
  ]
}

Hard rules:
- Generate a small Go command extension only.
- Do not request shell, secrets, direct network, or filesystem write permissions.
- The manifest name must exactly match the requested extension name.
- The manifest entrypoint must be a local command: ./<extension_name>.
- The manifest tests must include "go test ./...".
- main.go must read one JSON object from stdin and write one JSON object to stdout.
- Use protocol "yemaka.extension.v1".
- Output fields should include ok and summary.
- Tests must pass without internet and without external services.
- Keep code compact and understandable for low-resource local machines.`)
}

func capabilityDraftRepairSystemPrompt() string {
	return strings.TrimSpace(`You are repairing a profile-local Yemaka generated extension package that failed trusted-core validation, tests, build, or hardening.

Return JSON only. Do not use markdown fences unless unavoidable.
Return a complete corrected PackageDraft object, not a patch.

The JSON object must match:
{
  "origin": "optional short provenance string",
  "files": [
    {"path": "extension.yaml", "content": "..."},
    {"path": "README.md", "content": "..."},
    {"path": "go.mod", "content": "..."},
    {"path": "main.go", "content": "..."},
    {"path": "main_test.go", "content": "..."}
  ]
}

Hard rules:
- Preserve the requested extension name exactly.
- Generate a small Go command extension only.
- Do not request shell, secrets, direct network, or filesystem write permissions.
- The manifest entrypoint must be a local command: ./<extension_name>.
- The manifest tests must include "go test ./...".
- main.go must read one JSON object from stdin and write one JSON object to stdout.
- Use protocol "yemaka.extension.v1".
- Output fields should include ok and summary.
- Tests must pass without internet and without external services.
- Fix only the failure described by the trusted core. Keep the package compact.`)
}

func capabilityDraftUserPrompt(input CapabilityDraftRequest) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Requested extension name: %s\n", strings.TrimSpace(input.Name))
	fmt.Fprintf(&builder, "User request: %s\n", strings.TrimSpace(input.Request))
	fmt.Fprintf(&builder, "Capability description: %s\n", strings.TrimSpace(input.Description))
	if input.Proposal.Reason != "" {
		fmt.Fprintf(&builder, "Why the capability is needed: %s\n", strings.TrimSpace(input.Proposal.Reason))
	}
	if len(input.Proposal.Permissions) > 0 {
		fmt.Fprintf(&builder, "Allowed permission posture from core proposal: %s\n", strings.Join(input.Proposal.Permissions, ", "))
	}
	fmt.Fprintf(&builder, "\nDraft the smallest safe local tool package that handles this task shape through JSON input.")
	return builder.String()
}

func capabilityDraftRepairUserPrompt(input CapabilityDraftRepairRequest, attempt int) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Repair attempt: %d\n", attempt)
	fmt.Fprintf(&builder, "Requested extension name: %s\n", strings.TrimSpace(input.Name))
	fmt.Fprintf(&builder, "User request: %s\n", strings.TrimSpace(input.Request))
	fmt.Fprintf(&builder, "Capability description: %s\n", strings.TrimSpace(input.Description))
	if input.Proposal.Reason != "" {
		fmt.Fprintf(&builder, "Why the capability is needed: %s\n", strings.TrimSpace(input.Proposal.Reason))
	}
	fmt.Fprintf(&builder, "\nTrusted-core failure summary:\n%s\n", compactRepairText(input.Failure, 12*1024))
	fmt.Fprintf(&builder, "\nPrevious PackageDraft JSON:\n%s\n", compactDraftJSON(input.PreviousDraft))
	fmt.Fprintf(&builder, "\nReturn a complete corrected PackageDraft JSON object. Do not explain the change.")
	return builder.String()
}

func compactDraftJSON(draft *extensions.PackageDraft) string {
	if draft == nil {
		return "{}"
	}
	data, err := json.Marshal(draft)
	if err != nil {
		return "{}"
	}
	return compactRepairText(string(data), maxCapabilityDraftRepairPromptBytes)
}

func compactRepairText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	head := limit / 2
	tail := limit - head
	return value[:head] + "\n...[truncated for low-memory repair prompt]...\n" + value[len(value)-tail:]
}

func decodeCapabilityPackageDraft(output string) (*extensions.PackageDraft, error) {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil, fmt.Errorf("local model returned an empty extension draft")
	}
	candidates := draftJSONCandidates(output)
	var lastErr error
	for _, candidate := range candidates {
		var draft extensions.PackageDraft
		if err := json.Unmarshal([]byte(candidate), &draft); err != nil {
			lastErr = err
			continue
		}
		if len(draft.Files) == 0 {
			lastErr = fmt.Errorf("extension draft JSON has no files")
			continue
		}
		return &draft, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no JSON object found in local model output")
	}
	return nil, fmt.Errorf("decode synthesized extension draft JSON: %w", lastErr)
}

func draftJSONCandidates(output string) []string {
	candidates := []string{output}
	if fenced := fencedJSONBlock(output); fenced != "" {
		candidates = append(candidates, fenced)
	}
	start := strings.Index(output, "{")
	end := strings.LastIndex(output, "}")
	if start >= 0 && end > start {
		candidates = append(candidates, output[start:end+1])
	}
	return candidates
}

func fencedJSONBlock(output string) string {
	start := strings.Index(output, "```")
	if start < 0 {
		return ""
	}
	rest := output[start+3:]
	if newline := strings.Index(rest, "\n"); newline >= 0 {
		rest = rest[newline+1:]
	}
	end := strings.Index(rest, "```")
	if end < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:end])
}

func safeOriginValue(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return strings.TrimSpace(value)
}
