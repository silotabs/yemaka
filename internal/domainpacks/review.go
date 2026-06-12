package domainpacks

import (
	"fmt"
	"path/filepath"
	"strings"

	"yemaka/internal/skills"
)

type PackReview struct {
	Name                string            `json:"name"`
	Version             string            `json:"version,omitempty"`
	Description         string            `json:"description,omitempty"`
	Category            string            `json:"category,omitempty"`
	Source              string            `json:"source,omitempty"`
	Path                string            `json:"path,omitempty"`
	Installed           bool              `json:"installed"`
	Enabled             bool              `json:"enabled"`
	Valid               bool              `json:"valid"`
	ValidationError     string            `json:"validationError,omitempty"`
	RepairHint          string            `json:"repairHint,omitempty"`
	DefaultState        string            `json:"defaultState,omitempty"`
	RequiredTools       []string          `json:"requiredTools,omitempty"`
	OptionalTools       []string          `json:"optionalTools,omitempty"`
	ApprovalRequiredFor []string          `json:"approvalRequiredFor,omitempty"`
	BlockedActions      []string          `json:"blockedActions,omitempty"`
	SensitiveDataRules  []string          `json:"sensitiveDataRules,omitempty"`
	SafetySummary       string            `json:"safetySummary,omitempty"`
	RiskyToolReasons    []string          `json:"riskyToolReasons,omitempty"`
	Permissions         Permissions       `json:"permissions,omitempty"`
	Safety              Safety            `json:"safety,omitempty"`
	Skills              []PackSkillReview `json:"skills,omitempty"`
	Tests               []string          `json:"tests,omitempty"`
	Status              *PackStatus       `json:"status,omitempty"`
	Manifest            *Manifest         `json:"manifest,omitempty"`
}

type PackSkillReview struct {
	Ref              string               `json:"ref"`
	Name             string               `json:"name,omitempty"`
	Version          string               `json:"version,omitempty"`
	Description      string               `json:"description,omitempty"`
	Triggers         []string             `json:"triggers,omitempty"`
	RequiredTools    []string             `json:"requiredTools,omitempty"`
	Permissions      skills.Permissions   `json:"permissions,omitempty"`
	ContextBudget    skills.ContextBudget `json:"contextBudget,omitempty"`
	InstructionChars int                  `json:"instructionChars,omitempty"`
	Valid            bool                 `json:"valid"`
	ValidationError  string               `json:"validationError,omitempty"`
	Dir              string               `json:"dir,omitempty"`
}

func ReviewInstalled(profilePackRoot string, name string) (PackReview, error) {
	dir, err := packDir(profilePackRoot, name)
	if err != nil {
		return PackReview{}, err
	}
	status, statusErr := statusForDir(dir)
	if statusErr != nil {
		status = PackStatus{
			Name:            strings.TrimSpace(name),
			Enabled:         false,
			Installed:       true,
			Valid:           false,
			ValidationError: statusErr.Error(),
			RepairHint:      repairHintForError(statusErr),
			Dir:             dir,
		}
		return PackReview{
			Name:            status.Name,
			Source:          "domain_pack",
			Path:            dir,
			Installed:       true,
			Enabled:         false,
			Valid:           false,
			ValidationError: status.ValidationError,
			RepairHint:      status.RepairHint,
			Status:          &status,
		}, nil
	}
	return ReviewDir(dir, "domain_pack", &status)
}

func ReviewDir(dir string, source string, status *PackStatus) (PackReview, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return PackReview{}, fmt.Errorf("pack directory is required")
	}
	manifest, err := Load(dir)
	if err != nil {
		return PackReview{}, err
	}
	manifest.Dir = dir
	review := reviewFromManifest(manifest, source)
	review.Path = dir
	if status != nil {
		statusCopy := *status
		review.Status = &statusCopy
		review.Installed = status.Installed
		review.Enabled = status.Enabled
		review.Valid = status.Valid
		review.ValidationError = status.ValidationError
		review.RepairHint = status.RepairHint
	} else {
		review.Valid = true
	}
	return review, nil
}

func reviewFromManifest(manifest Manifest, source string) PackReview {
	manifest = Normalize(manifest)
	manifestCopy := manifest
	review := PackReview{
		Name:                manifest.Name,
		Version:             manifest.Version,
		Description:         manifest.Description,
		Category:            manifest.Category,
		Source:              strings.TrimSpace(source),
		Path:                manifest.Dir,
		Valid:               true,
		DefaultState:        manifest.DefaultState,
		RequiredTools:       append([]string{}, manifest.RequiredTools...),
		OptionalTools:       append([]string{}, manifest.OptionalTools...),
		ApprovalRequiredFor: append([]string{}, manifest.Safety.ApprovalRequiredFor...),
		BlockedActions:      append([]string{}, manifest.Safety.BlockedActions...),
		SensitiveDataRules:  append([]string{}, manifest.Safety.SensitiveDataRules...),
		SafetySummary:       packSafetySummary(manifest),
		RiskyToolReasons:    riskyToolReasons(manifest),
		Permissions:         manifest.Permissions,
		Safety:              manifest.Safety,
		Tests:               append([]string{}, manifest.Tests...),
		Manifest:            &manifestCopy,
	}
	for _, ref := range manifest.Skills {
		review.Skills = append(review.Skills, reviewSkill(filepath.Join(manifest.Dir, filepath.FromSlash(ref)), ref))
	}
	return review
}

func reviewSkill(dir string, ref string) PackSkillReview {
	item := PackSkillReview{Ref: ref, Dir: dir}
	skill, err := skills.Load(dir)
	if err != nil {
		item.Valid = false
		item.ValidationError = err.Error()
		return item
	}
	item.Valid = true
	item.Name = skill.Name
	item.Version = skill.Version
	item.Description = skill.Description
	item.Triggers = append([]string{}, skill.Triggers...)
	item.RequiredTools = append([]string{}, skill.RequiredTools...)
	item.Permissions = skill.Permissions
	item.ContextBudget = skill.ContextBudget
	item.InstructionChars = len(skill.Instructions)
	return item
}

func riskyToolReasons(manifest Manifest) []string {
	seen := map[string]struct{}{}
	add := func(reason string) {
		reason = strings.TrimSpace(reason)
		if reason == "" {
			return
		}
		if _, ok := seen[reason]; ok {
			return
		}
		seen[reason] = struct{}{}
	}
	for _, tool := range append(append([]string{}, manifest.RequiredTools...), manifest.OptionalTools...) {
		switch strings.TrimSpace(tool) {
		case "internet_search", "internet_fetch", "internet_head":
			add("Internet tools require configured provider, policy approval, and disabled-by-default release settings.")
		case "connector_action":
			add("Connector actions require configured connector scope, auth reference, rate limits, and policy approval.")
		case "scheduler_create":
			add("Scheduler jobs require explicit approval and stay disabled until enabled.")
		case "run_shell_safe":
			add("Shell actions require core policy approval and cannot bypass safe-command rules.")
		case "memory_write":
			add("Memory writes require explicit user approval and must avoid sensitive data capture.")
		case "edit_file", "write_file":
			add("File writes require workspace scope, snapshot, diff review, confirmation, and rollback.")
		}
	}
	for _, action := range manifest.Safety.BlockedActions {
		action = strings.TrimSpace(action)
		if action != "" {
			add(action + " is blocked by this pack manifest.")
		}
	}
	for _, action := range manifest.Safety.ApprovalRequiredFor {
		action = strings.TrimSpace(action)
		if action != "" {
			add(action + " requires explicit approval before this pack can use it.")
		}
	}
	if manifest.Permissions.Internet.Required {
		add("Internet permission is declared and remains controlled by core internet policy.")
	}
	if manifest.Permissions.Connectors.Required {
		add("Connector permission is declared and remains controlled by connector policy.")
	}
	if manifest.Permissions.Scheduler.Required {
		add("Scheduler permission is declared and remains approval-gated.")
	}
	return mapKeysSorted(seen)
}

func mapKeysSorted(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sortStrings(out)
	return out
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
