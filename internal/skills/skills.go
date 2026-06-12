package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"yemaka/internal/tools"
)

const defaultMaxInstructionChars = 2400

var skillNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

type Skill struct {
	Name          string        `yaml:"name"`
	Version       string        `yaml:"version"`
	Description   string        `yaml:"description"`
	Triggers      []string      `yaml:"triggers"`
	RequiredTools []string      `yaml:"required_tools"`
	Permissions   Permissions   `yaml:"permissions"`
	ContextBudget ContextBudget `yaml:"context_budget"`
	Disabled      bool          `yaml:"disabled"`
	Instructions  string        `yaml:"-"`
	Dir           string        `yaml:"-"`
	Source        string        `yaml:"-"`
}

type Permissions struct {
	FilesystemRead  bool   `yaml:"filesystem_read"`
	FilesystemWrite string `yaml:"filesystem_write"`
	Shell           string `yaml:"shell"`
	Network         bool   `yaml:"network"`
}

type ContextBudget struct {
	MaxInstructionChars int `yaml:"max_instruction_chars"`
	MaxExamples         int `yaml:"max_examples"`
}

type Registry struct {
	Skills  map[string]Skill
	Invalid map[string]SkillIssue
}

type SkillIssue struct {
	Name  string
	Dir   string
	Error string
}

type SkillStatus struct {
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

type ValidationOptions struct {
	AvailableTools      map[string]bool
	AllowNetwork        bool
	MaxInstructionChars int
}

func LoadRegistry(dirs []string) (Registry, error) {
	registry := Registry{
		Skills:  map[string]Skill{},
		Invalid: map[string]SkillIssue{},
	}
	for _, dir := range dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Registry{}, fmt.Errorf("read skills directory %s: %w", dir, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(dir, entry.Name())
			skill, err := Load(skillDir)
			if err != nil {
				registry.Invalid[entry.Name()] = SkillIssue{
					Name:  entry.Name(),
					Dir:   skillDir,
					Error: err.Error(),
				}
				continue
			}
			if skill.Source == "" {
				skill.Source = sourceForDir(dir)
			}
			registry.Skills[skill.Name] = skill
		}
	}
	return registry, nil
}

func LoadProfileRegistry(defaultSkillsDir string, profileSkillsDir string, enabledDomainPackSkillDirs []string) (Registry, error) {
	return LoadRegistry(ProfileRegistryDirs(defaultSkillsDir, profileSkillsDir, enabledDomainPackSkillDirs))
}

func ProfileRegistryDirs(defaultSkillsDir string, profileSkillsDir string, enabledDomainPackSkillDirs []string) []string {
	dirs := make([]string, 0, 2+len(enabledDomainPackSkillDirs))
	seen := map[string]struct{}{}
	appendDir := func(dir string) {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			return
		}
		key := filepath.Clean(dir)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		dirs = append(dirs, dir)
	}

	appendDir(defaultSkillsDir)
	for _, dir := range enabledDomainPackSkillDirs {
		appendDir(dir)
	}
	appendDir(profileSkillsDir)
	return dirs
}

func Load(dir string) (Skill, error) {
	data, err := os.ReadFile(filepath.Join(dir, "skill.yaml"))
	if err != nil {
		return Skill{}, fmt.Errorf("read skill.yaml in %s: %w", dir, err)
	}
	var skill Skill
	if err := yaml.Unmarshal(data, &skill); err != nil {
		return Skill{}, fmt.Errorf("parse skill.yaml in %s: %w", dir, err)
	}
	instructions, err := os.ReadFile(filepath.Join(dir, "instructions.md"))
	if err != nil {
		return Skill{}, fmt.Errorf("read instructions.md in %s: %w", dir, err)
	}
	skill.Instructions = strings.TrimSpace(string(instructions))
	skill.Dir = dir
	skill.Source = sourceForDir(filepath.Dir(dir))
	if err := Validate(skill); err != nil {
		return Skill{}, fmt.Errorf("validate skill %s: %w", dir, err)
	}
	return skill, nil
}

func Validate(skill Skill) error {
	return ValidateWithOptions(skill, ValidationOptions{})
}

func ValidateWithOptions(skill Skill, options ValidationOptions) error {
	if strings.TrimSpace(skill.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !skillNamePattern.MatchString(skill.Name) {
		return fmt.Errorf("name must be lowercase snake_case and start with a letter")
	}
	if strings.TrimSpace(skill.Version) == "" {
		return fmt.Errorf("version is required")
	}
	if strings.TrimSpace(skill.Description) == "" {
		return fmt.Errorf("description is required")
	}
	if len(skill.Triggers) == 0 {
		return fmt.Errorf("at least one trigger is required")
	}
	if strings.TrimSpace(skill.Instructions) == "" {
		return fmt.Errorf("instructions are required")
	}
	if skill.ContextBudget.MaxInstructionChars < 0 {
		return fmt.Errorf("max_instruction_chars cannot be negative")
	}
	maxChars := skill.ContextBudget.MaxInstructionChars
	if maxChars == 0 {
		maxChars = defaultMaxInstructionChars
	}
	if options.MaxInstructionChars > 0 && options.MaxInstructionChars < maxChars {
		maxChars = options.MaxInstructionChars
	}
	if len(skill.Instructions) > maxChars {
		return fmt.Errorf("instructions exceed max_instruction_chars")
	}
	if skill.Permissions.Network && !options.AllowNetwork {
		return fmt.Errorf("network-enabled skills are not supported in MVP")
	}
	if skill.Permissions.FilesystemWrite != "" && skill.Permissions.FilesystemWrite != "workspace_only" {
		return fmt.Errorf("filesystem_write must be workspace_only or empty")
	}
	if skill.Permissions.Shell != "" && skill.Permissions.Shell != "limited" {
		return fmt.Errorf("shell must be limited or empty")
	}
	if len(skill.RequiredTools) == 0 {
		return fmt.Errorf("at least one required tool is required")
	}
	for _, tool := range skill.RequiredTools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			return fmt.Errorf("required_tools contains an empty tool")
		}
		if !KnownTool(tool) {
			return fmt.Errorf("unknown or unsafe required tool: %s", tool)
		}
		if len(options.AvailableTools) > 0 && !options.AvailableTools[tool] {
			return fmt.Errorf("required tool is unavailable: %s", tool)
		}
	}
	return nil
}

func (r Registry) List() []Skill {
	items := make([]Skill, 0, len(r.Skills))
	for _, skill := range r.Skills {
		items = append(items, skill)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func (r Registry) Statuses() []SkillStatus {
	statuses := make([]SkillStatus, 0, len(r.Skills)+len(r.Invalid))
	for _, skill := range r.Skills {
		statuses = append(statuses, SkillStatus{
			Name:          skill.Name,
			Version:       skill.Version,
			Description:   skill.Description,
			Triggers:      skill.Triggers,
			RequiredTools: skill.RequiredTools,
			Enabled:       !skill.Disabled,
			Valid:         true,
			Source:        skill.Source,
			Dir:           skill.Dir,
		})
	}
	for name, issue := range r.Invalid {
		statuses = append(statuses, SkillStatus{
			Name:            name,
			Enabled:         false,
			Valid:           false,
			ValidationError: issue.Error,
			Source:          sourceForDir(filepath.Dir(issue.Dir)),
			Dir:             issue.Dir,
		})
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].Name < statuses[j].Name
	})
	return statuses
}

func (r Registry) Get(name string) (Skill, bool) {
	skill, ok := r.Skills[name]
	return skill, ok
}

func (r Registry) Select(request string, availableTools map[string]bool) (Skill, bool) {
	request = strings.ToLower(strings.TrimSpace(request))
	explanationOnly := isExplanationOnlySkillRequest(request)
	var best Skill
	bestScore := 0
	for _, skill := range r.Skills {
		if skill.Disabled {
			continue
		}
		if explanationOnly && isActionWorkflowSkill(skill) {
			continue
		}
		if !requiredToolsAvailable(skill, availableTools) {
			continue
		}
		score := 0
		for _, trigger := range skill.Triggers {
			trigger = strings.ToLower(strings.TrimSpace(trigger))
			if trigger != "" && strings.Contains(request, trigger) {
				score += 20
			}
		}
		score += deterministicSkillScore(skill.Name, request)
		if score > bestScore {
			best = skill
			bestScore = score
		}
	}
	return best, bestScore > 0
}

func isExplanationOnlySkillRequest(request string) bool {
	request = strings.TrimSpace(strings.ToLower(request))
	if request == "" {
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
		if strings.HasPrefix(request, prefix) {
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
		if strings.Contains(request, term) {
			return false
		}
	}
	return true
}

func isActionWorkflowSkill(skill Skill) bool {
	name := strings.ToLower(strings.TrimSpace(skill.Name))
	instructions := strings.ToLower(strings.TrimSpace(skill.Instructions))
	return strings.HasSuffix(name, "_workflow") ||
		strings.Contains(instructions, "compiled workflow skill") ||
		strings.Contains(instructions, "run the saved workflow")
}

func KnownTool(name string) bool {
	return tools.KnownTool(strings.TrimSpace(name))
}

func PromptBlock(skill Skill) string {
	instructions := strings.TrimSpace(skill.Instructions)
	if skill.ContextBudget.MaxInstructionChars > 0 && len(instructions) > skill.ContextBudget.MaxInstructionChars {
		instructions = strings.TrimSpace(instructions[:skill.ContextBudget.MaxInstructionChars]) + "\n[truncated]"
	}
	return fmt.Sprintf("SKILL: %s v%s\n%s", skill.Name, skill.Version, instructions)
}

func requiredToolsAvailable(skill Skill, available map[string]bool) bool {
	if len(available) == 0 {
		return true
	}
	for _, tool := range skill.RequiredTools {
		if !available[tool] {
			return false
		}
	}
	return true
}

func deterministicSkillScore(name string, request string) int {
	switch name {
	case "project_explainer":
		if strings.Contains(request, "explain") && (strings.Contains(request, "project") || strings.Contains(request, "codebase")) {
			return 30
		}
	case "document_summary":
		if strings.Contains(request, "summarize") || strings.Contains(request, "summary") {
			return 30
		}
	case "git_diff_review":
		if strings.Contains(request, "git diff") || strings.Contains(request, "review my diff") || strings.Contains(request, "review the diff") {
			return 30
		}
	case "python_bugfix":
		if strings.Contains(request, "traceback") || strings.Contains(request, "pytest") || strings.Contains(request, "python bug") {
			return 30
		}
	case "followup_capture":
		if looksFollowupCaptureRequest(request) {
			return 35
		}
	case "code_review":
		if strings.Contains(request, "code review") || strings.Contains(request, "review this code") {
			return 30
		}
	}
	return 0
}

func looksFollowupCaptureRequest(request string) bool {
	hasTarget := strings.Contains(request, "follow-up") ||
		strings.Contains(request, "follow up") ||
		strings.Contains(request, "followup") ||
		strings.Contains(request, "action item") ||
		strings.Contains(request, "action items")
	if !hasTarget {
		return false
	}
	for _, verb := range []string{"capture", "save", "remember", "store", "record"} {
		if strings.Contains(request, verb) {
			return true
		}
	}
	return false
}

func sourceForDir(dir string) string {
	clean := filepath.ToSlash(filepath.Clean(dir))
	switch {
	case strings.HasSuffix(clean, "skills/default"):
		return "default"
	case pathHasSegment(clean, "domain_packs") && strings.HasSuffix(clean, "/skills"):
		return "domain_pack"
	case strings.HasSuffix(clean, "/skills") && hasDomainPackManifest(dir):
		return "domain_pack"
	case strings.Contains(clean, "/profiles/") && strings.HasSuffix(clean, "/skills"):
		return "profile"
	default:
		return "local"
	}
}

func hasDomainPackManifest(skillsDir string) bool {
	_, err := os.Stat(filepath.Join(filepath.Clean(skillsDir), "..", "pack.yaml"))
	return err == nil
}

func pathHasSegment(path string, segment string) bool {
	for _, part := range strings.Split(path, "/") {
		if part == segment {
			return true
		}
	}
	return false
}
