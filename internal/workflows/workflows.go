package workflows

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
)

const (
	DefaultStateDisabled = "disabled"
	DefaultStateEnabled  = "enabled"

	IntentExplain = "explain"
	IntentAction  = "action"

	RunModeManual    = "manual"
	RunModeScheduled = "scheduled"

	SourceDomainPackSkill = "domain_pack_skill"
)

var (
	workflowNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,127}$`)
	toolNamePattern     = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)
	nonTokenPattern     = regexp.MustCompile(`[^a-z0-9]+`)
)

type Workflow struct {
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Description   string            `json:"description,omitempty"`
	Source        string            `json:"source,omitempty"`
	PackName      string            `json:"packName,omitempty"`
	SkillName     string            `json:"skillName,omitempty"`
	Intent        string            `json:"intent"`
	RunMode       string            `json:"runMode"`
	Reusable      bool              `json:"reusable"`
	Enabled       bool              `json:"enabled"`
	DefaultState  string            `json:"defaultState"`
	Triggers      []string          `json:"triggers,omitempty"`
	RequiredTools []string          `json:"requiredTools,omitempty"`
	Steps         []Step            `json:"steps,omitempty"`
	Permissions   Permissions       `json:"permissions"`
	Safety        Safety            `json:"safety"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type Step struct {
	Title       string `json:"title,omitempty"`
	Instruction string `json:"instruction,omitempty"`
	Tool        string `json:"tool,omitempty"`
}

type Permissions struct {
	Internet      Capability `json:"internet"`
	Cloud         Capability `json:"cloud"`
	Connectors    Capability `json:"connectors"`
	Scheduler     Capability `json:"scheduler"`
	Notifications Capability `json:"notifications"`
	Embeddings    Capability `json:"embeddings"`
	Filesystem    Capability `json:"filesystem"`
	Memory        Capability `json:"memory"`
	Secrets       Capability `json:"secrets"`
}

type Capability struct {
	Required bool     `json:"required,omitempty"`
	Scopes   []string `json:"scopes,omitempty"`
	Names    []string `json:"names,omitempty"`
	Notes    string   `json:"notes,omitempty"`
}

type Safety struct {
	ApprovalRequiredFor []string `json:"approvalRequiredFor,omitempty"`
	BlockedActions      []string `json:"blockedActions,omitempty"`
	SensitiveDataRules  []string `json:"sensitiveDataRules,omitempty"`
}

type Library struct {
	items map[string]Workflow
}

type MatchOptions struct {
	IncludeDisabled bool
	AvailableTools  map[string]bool
}

func NewLibrary(workflows ...Workflow) (Library, error) {
	library := Library{items: map[string]Workflow{}}
	for _, workflow := range workflows {
		if err := library.Add(workflow); err != nil {
			return Library{}, err
		}
	}
	return library, nil
}

func (l *Library) Add(workflow Workflow) error {
	workflow = Normalize(workflow)
	if err := Validate(workflow); err != nil {
		return err
	}
	if l.items == nil {
		l.items = map[string]Workflow{}
	}
	if _, exists := l.items[workflow.Name]; exists {
		return fmt.Errorf("duplicate workflow: %s", workflow.Name)
	}
	l.items[workflow.Name] = workflow
	return nil
}

func (l Library) List() []Workflow {
	items := make([]Workflow, 0, len(l.items))
	for _, item := range l.items {
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func (l Library) Match(prompt string, options MatchOptions) (Workflow, bool) {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return Workflow{}, false
	}
	explanationOnly := IsExplanationOnlyPrompt(prompt)
	bestScore := 0
	var best Workflow
	for _, workflow := range l.List() {
		if !options.IncludeDisabled && !workflow.Enabled {
			continue
		}
		if explanationOnly && workflow.Intent == IntentAction {
			continue
		}
		if !requiredToolsAvailable(workflow.RequiredTools, options.AvailableTools) {
			continue
		}
		score := workflowScore(workflow, prompt)
		if score > bestScore {
			best = workflow
			bestScore = score
		}
	}
	return best, bestScore > 0
}

func Normalize(workflow Workflow) Workflow {
	workflow.Name = normalizeName(workflow.Name)
	workflow.Version = strings.TrimSpace(workflow.Version)
	workflow.Description = strings.TrimSpace(workflow.Description)
	workflow.Source = strings.TrimSpace(workflow.Source)
	workflow.PackName = normalizeName(workflow.PackName)
	workflow.SkillName = normalizeName(workflow.SkillName)
	workflow.Triggers = cleanList(workflow.Triggers, false)
	workflow.RequiredTools = cleanTools(workflow.RequiredTools)
	workflow.Steps = normalizeSteps(workflow.Steps)
	workflow.DefaultState = normalizeDefaultState(workflow.DefaultState)
	workflow.Intent = normalizeIntent(workflow.Intent, workflow.RequiredTools, workflow.Steps)
	workflow.RunMode = normalizeRunMode(workflow.RunMode)
	workflow.Permissions = normalizePermissions(workflow.Permissions)
	workflow.Safety = normalizeSafety(workflow.Safety)
	if workflow.Metadata != nil {
		workflow.Metadata = cleanMetadata(workflow.Metadata)
	}
	return workflow
}

func Validate(workflow Workflow) error {
	workflow = Normalize(workflow)
	if workflow.Name == "" {
		return fmt.Errorf("workflow name is required")
	}
	if !workflowNamePattern.MatchString(workflow.Name) {
		return fmt.Errorf("workflow name must match %s", workflowNamePattern.String())
	}
	if workflow.Version == "" {
		return fmt.Errorf("workflow version is required")
	}
	if workflow.Description == "" {
		return fmt.Errorf("workflow description is required")
	}
	if len(workflow.Triggers) == 0 {
		return fmt.Errorf("at least one workflow trigger is required")
	}
	switch workflow.DefaultState {
	case DefaultStateDisabled, DefaultStateEnabled:
	default:
		return fmt.Errorf("workflow default_state must be %q or %q", DefaultStateDisabled, DefaultStateEnabled)
	}
	switch workflow.Intent {
	case IntentExplain, IntentAction:
	default:
		return fmt.Errorf("workflow intent must be %q or %q", IntentExplain, IntentAction)
	}
	switch workflow.RunMode {
	case RunModeManual, RunModeScheduled:
	default:
		return fmt.Errorf("workflow run_mode must be %q or %q", RunModeManual, RunModeScheduled)
	}
	for _, tool := range workflow.RequiredTools {
		if !toolNamePattern.MatchString(tool) {
			return fmt.Errorf("required tool %q must match %s", tool, toolNamePattern.String())
		}
	}
	for _, step := range workflow.Steps {
		if strings.TrimSpace(step.Tool) != "" && !toolNamePattern.MatchString(step.Tool) {
			return fmt.Errorf("workflow step tool %q must match %s", step.Tool, toolNamePattern.String())
		}
	}
	if err := validateRuntimeEnablement(workflow); err != nil {
		return err
	}
	return validateSafeDefaults(workflow)
}

func (workflow Workflow) EnabledByDefault() bool {
	return normalizeDefaultState(workflow.DefaultState) == DefaultStateEnabled
}

func (workflow Workflow) LocalOnly() bool {
	return !workflow.Permissions.RequiresExternal()
}

func (permissions Permissions) RequiresExternal() bool {
	return permissions.Internet.Required ||
		permissions.Cloud.Required ||
		permissions.Connectors.Required ||
		permissions.Scheduler.Required ||
		permissions.Notifications.Required ||
		permissions.Embeddings.Required
}

func FromDomainPackSkill(manifest domainpacks.Manifest, skill skills.Skill) (Workflow, error) {
	manifest = domainpacks.Normalize(manifest)
	workflow := Workflow{
		Name:          workflowName(manifest.Name, skill.Name),
		Version:       firstNonEmpty(skill.Version, manifest.Version),
		Description:   skill.Description,
		Source:        SourceDomainPackSkill,
		PackName:      manifest.Name,
		SkillName:     skill.Name,
		Reusable:      true,
		DefaultState:  DefaultStateDisabled,
		RunMode:       RunModeManual,
		Triggers:      skill.Triggers,
		RequiredTools: skill.RequiredTools,
		Permissions:   permissionsFromMetadata(manifest.Permissions, skill),
		Safety: Safety{
			ApprovalRequiredFor: manifest.Safety.ApprovalRequiredFor,
			BlockedActions:      manifest.Safety.BlockedActions,
			SensitiveDataRules:  manifest.Safety.SensitiveDataRules,
		},
		Metadata: map[string]string{
			"pack_default_state": domainpacks.Normalize(manifest).DefaultState,
			"skill_disabled":     fmt.Sprintf("%t", skill.Disabled),
		},
	}
	workflow.Intent = inferIntent(workflow.RequiredTools, workflow.Steps)
	workflow = Normalize(workflow)
	if err := Validate(workflow); err != nil {
		return Workflow{}, err
	}
	return workflow, nil
}

func FromDomainPackSkills(manifest domainpacks.Manifest, packSkills []skills.Skill) ([]Workflow, error) {
	workflows := make([]Workflow, 0, len(packSkills))
	for _, skill := range packSkills {
		workflow, err := FromDomainPackSkill(manifest, skill)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, workflow)
	}
	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Name < workflows[j].Name
	})
	return workflows, nil
}

func IsExplanationOnlyPrompt(prompt string) bool {
	prompt = strings.TrimSpace(strings.ToLower(prompt))
	if prompt == "" {
		return false
	}
	prefixes := []string{
		"explain ",
		"describe ",
		"what is ",
		"what are ",
		"how does ",
		"how do ",
		"how can ",
		"how would ",
		"how should ",
		"why does ",
		"why do ",
		"teach me ",
		"can you explain ",
		"could you explain ",
	}
	matchesPrefix := false
	for _, prefix := range prefixes {
		if strings.HasPrefix(prompt, prefix) {
			matchesPrefix = true
			break
		}
	}
	if !matchesPrefix {
		return false
	}
	actionTerms := []string{
		" and run ",
		" then run ",
		" and execute ",
		" then execute ",
		" and edit ",
		" then edit ",
		" and write ",
		" then write ",
		" and apply ",
		" then apply ",
		" and save ",
		" then save ",
		" and schedule ",
		" then schedule ",
	}
	for _, term := range actionTerms {
		if strings.Contains(prompt, term) {
			return false
		}
	}
	return true
}

func validateRuntimeEnablement(workflow Workflow) error {
	if !workflow.Enabled {
		return nil
	}
	if workflow.RunMode != RunModeManual {
		return fmt.Errorf("enabled workflows must be manual unless scheduler policy enables jobs")
	}
	if workflow.Permissions.RequiresExternal() {
		return fmt.Errorf("enabled workflows cannot require internet, cloud, connectors, scheduler, notifications, or embeddings")
	}
	for _, tool := range workflow.RequiredTools {
		if !skills.KnownTool(tool) {
			return fmt.Errorf("enabled workflow requires unknown or unsafe tool: %s", tool)
		}
	}
	return nil
}

func validateSafeDefaults(workflow Workflow) error {
	if !workflow.EnabledByDefault() {
		return nil
	}
	if workflow.Intent == IntentAction {
		return fmt.Errorf("unsafe default: action workflows cannot be enabled by default")
	}
	if workflow.RunMode != RunModeManual {
		return fmt.Errorf("unsafe default: scheduled workflows cannot be enabled by default")
	}
	if workflow.Permissions.RequiresExternal() {
		return fmt.Errorf("unsafe default: external capabilities cannot be required by default")
	}
	for _, tool := range workflow.RequiredTools {
		if !skills.KnownTool(tool) {
			return fmt.Errorf("unsafe default: unknown or unsafe tool %s cannot be required by default", tool)
		}
	}
	return nil
}

func permissionsFromMetadata(packPermissions domainpacks.Permissions, skill skills.Skill) Permissions {
	permissions := Permissions{
		Internet:      capabilityFromPack(packPermissions.Internet),
		Connectors:    capabilityFromPack(packPermissions.Connectors),
		Scheduler:     capabilityFromPack(packPermissions.Scheduler),
		Notifications: capabilityFromPack(packPermissions.Notifications),
		Filesystem:    capabilityFromPack(packPermissions.Filesystem),
		Secrets:       capabilityFromPack(packPermissions.Secrets),
	}
	if skill.Permissions.Network {
		permissions.Internet.Required = true
		permissions.Internet.Scopes = append(permissions.Internet.Scopes, "GET", "HEAD")
	}
	if skill.Permissions.FilesystemRead {
		permissions.Filesystem.Scopes = append(permissions.Filesystem.Scopes, "read")
	}
	if skill.Permissions.FilesystemWrite != "" {
		permissions.Filesystem.Required = true
		permissions.Filesystem.Scopes = append(permissions.Filesystem.Scopes, skill.Permissions.FilesystemWrite)
	}
	for _, tool := range skill.RequiredTools {
		switch tool {
		case "memory_search":
			permissions.Memory.Scopes = append(permissions.Memory.Scopes, "search")
		case "memory_write":
			permissions.Memory.Required = true
			permissions.Memory.Scopes = append(permissions.Memory.Scopes, "write")
		}
	}
	return normalizePermissions(permissions)
}

func capabilityFromPack(permission domainpacks.PermissionDeclaration) Capability {
	return Capability{
		Required: permission.Required,
		Scopes:   append(append([]string{}, permission.Scopes...), permission.AllowedMethods...),
		Names:    append(append(append([]string{}, permission.Names...), permission.AllowedDomains...), permission.Paths...),
		Notes:    permission.Notes,
	}
}

func workflowScore(workflow Workflow, prompt string) int {
	normalizedPrompt := normalizedText(prompt)
	score := 0
	for _, trigger := range workflow.Triggers {
		normalizedTrigger := normalizedText(trigger)
		if normalizedTrigger != "" && strings.Contains(normalizedPrompt, normalizedTrigger) {
			score += 50
		}
	}
	for _, token := range strings.Fields(normalizedText(workflow.Name + " " + workflow.Description)) {
		if len(token) > 2 && strings.Contains(normalizedPrompt, " "+token+" ") {
			score += 2
		}
	}
	if workflow.Reusable && score > 0 {
		score += 1
	}
	return score
}

func requiredToolsAvailable(tools []string, available map[string]bool) bool {
	if len(available) == 0 {
		return true
	}
	for _, tool := range tools {
		if !available[tool] {
			return false
		}
	}
	return true
}

func normalizeName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ToLower(value)
	value = nonTokenPattern.ReplaceAllString(value, "_")
	return strings.Trim(value, "_")
}

func normalizeDefaultState(state string) string {
	state = strings.ToLower(strings.TrimSpace(state))
	if state == "" {
		return DefaultStateDisabled
	}
	return state
}

func normalizeRunMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return RunModeManual
	}
	return mode
}

func normalizeIntent(intent string, tools []string, steps []Step) string {
	intent = strings.ToLower(strings.TrimSpace(intent))
	switch intent {
	case IntentExplain, IntentAction:
		return intent
	case "":
		return inferIntent(tools, steps)
	default:
		return intent
	}
}

func inferIntent(tools []string, steps []Step) string {
	for _, tool := range tools {
		if isActionTool(tool) {
			return IntentAction
		}
	}
	for _, step := range steps {
		if isActionTool(step.Tool) {
			return IntentAction
		}
	}
	return IntentExplain
}

func isActionTool(tool string) bool {
	switch strings.TrimSpace(tool) {
	case "write_file",
		"edit_file",
		"run_shell_safe",
		"run_tests",
		"memory_write",
		"create_skill",
		"load_skill":
		return true
	default:
		return false
	}
}

func normalizeSteps(steps []Step) []Step {
	cleaned := make([]Step, 0, len(steps))
	for _, step := range steps {
		step.Title = strings.TrimSpace(step.Title)
		step.Instruction = strings.TrimSpace(step.Instruction)
		step.Tool = strings.TrimSpace(step.Tool)
		if step.Title == "" && step.Instruction == "" && step.Tool == "" {
			continue
		}
		cleaned = append(cleaned, step)
	}
	return cleaned
}

func normalizePermissions(permissions Permissions) Permissions {
	permissions.Internet = normalizeCapability(permissions.Internet)
	permissions.Cloud = normalizeCapability(permissions.Cloud)
	permissions.Connectors = normalizeCapability(permissions.Connectors)
	permissions.Scheduler = normalizeCapability(permissions.Scheduler)
	permissions.Notifications = normalizeCapability(permissions.Notifications)
	permissions.Embeddings = normalizeCapability(permissions.Embeddings)
	permissions.Filesystem = normalizeCapability(permissions.Filesystem)
	permissions.Memory = normalizeCapability(permissions.Memory)
	permissions.Secrets = normalizeCapability(permissions.Secrets)
	return permissions
}

func normalizeCapability(capability Capability) Capability {
	capability.Scopes = cleanList(capability.Scopes, false)
	capability.Names = cleanList(capability.Names, false)
	capability.Notes = strings.TrimSpace(capability.Notes)
	return capability
}

func normalizeSafety(safety Safety) Safety {
	safety.ApprovalRequiredFor = cleanList(safety.ApprovalRequiredFor, false)
	safety.BlockedActions = cleanList(safety.BlockedActions, false)
	safety.SensitiveDataRules = cleanList(safety.SensitiveDataRules, false)
	return safety
}

func cleanTools(values []string) []string {
	cleaned := cleanList(values, true)
	sort.Strings(cleaned)
	return cleaned
}

func cleanList(values []string, lower bool) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if lower {
			value = strings.ToLower(value)
		}
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func cleanMetadata(metadata map[string]string) map[string]string {
	cleaned := map[string]string{}
	for key, value := range metadata {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		cleaned[key] = value
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
}

func workflowName(packName string, skillName string) string {
	packName = normalizeName(packName)
	skillName = normalizeName(skillName)
	switch {
	case packName != "" && skillName != "":
		return packName + "_" + skillName
	case skillName != "":
		return skillName
	default:
		return packName
	}
}

func normalizedText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = nonTokenPattern.ReplaceAllString(value, " ")
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return ""
	}
	return " " + value + " "
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
