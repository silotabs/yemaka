package workflows

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
)

const (
	SourceDomainPackTemplate = "domain_pack_template"
)

type TemplateRef struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Template struct {
	Name          string            `json:"name"`
	Version       string            `json:"version"`
	Description   string            `json:"description,omitempty"`
	Category      string            `json:"category,omitempty"`
	Source        string            `json:"source,omitempty"`
	Path          string            `json:"path,omitempty"`
	Installed     bool              `json:"installed"`
	Enabled       bool              `json:"enabled"`
	DefaultState  string            `json:"defaultState"`
	RequiredTools []string          `json:"requiredTools,omitempty"`
	Workflows     []Workflow        `json:"workflows,omitempty"`
	Permissions   Permissions       `json:"permissions"`
	Safety        Safety            `json:"safety"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type TemplateSummary struct {
	Name                string   `json:"name"`
	Version             string   `json:"version"`
	Description         string   `json:"description,omitempty"`
	Category            string   `json:"category,omitempty"`
	Source              string   `json:"source,omitempty"`
	Path                string   `json:"path,omitempty"`
	Installed           bool     `json:"installed"`
	Enabled             bool     `json:"enabled"`
	Valid               bool     `json:"valid"`
	ValidationError     string   `json:"validationError,omitempty"`
	RepairHint          string   `json:"repairHint,omitempty"`
	DefaultState        string   `json:"defaultState"`
	RequiredTools       []string `json:"requiredTools,omitempty"`
	Skills              []string `json:"skills,omitempty"`
	Workflows           []string `json:"workflows,omitempty"`
	ApprovalRequiredFor []string `json:"approvalRequiredFor,omitempty"`
	BlockedActions      []string `json:"blockedActions,omitempty"`
	SensitiveDataRules  []string `json:"sensitiveDataRules,omitempty"`
	SafetySummary       string   `json:"safetySummary,omitempty"`
	InstallAction       string   `json:"installAction"`
}

type TemplateCatalog struct {
	items map[string]Template
}

var builtInTemplateRefs = []TemplateRef{
	{Name: "local_career_notes", Path: "packs/templates/local_career_notes"},
	{Name: "local_coding_notes", Path: "packs/templates/local_coding_notes"},
	{Name: "local_contract_notes", Path: "packs/templates/local_contract_notes"},
	{Name: "local_content_calendar_notes", Path: "packs/templates/local_content_calendar_notes"},
	{Name: "local_customer_support_notes", Path: "packs/templates/local_customer_support_notes"},
	{Name: "local_data_notes", Path: "packs/templates/local_data_notes"},
	{Name: "local_draft_editor", Path: "packs/templates/local_draft_editor"},
	{Name: "local_email_drafts", Path: "packs/templates/local_email_drafts"},
	{Name: "local_file_triage", Path: "packs/templates/local_file_triage"},
	{Name: "local_finance_notes", Path: "packs/templates/local_finance_notes"},
	{Name: "local_form_prep_notes", Path: "packs/templates/local_form_prep_notes"},
	{Name: "local_health_notes", Path: "packs/templates/local_health_notes"},
	{Name: "local_knowledge_base", Path: "packs/templates/local_knowledge_base"},
	{Name: "local_maintenance_notes", Path: "packs/templates/local_maintenance_notes"},
	{Name: "local_meeting_notes", Path: "packs/templates/local_meeting_notes"},
	{Name: "local_project_brief", Path: "packs/templates/local_project_brief"},
	{Name: "local_sales_client_notes", Path: "packs/templates/local_sales_client_notes"},
	{Name: "local_security_privacy_notes", Path: "packs/templates/local_security_privacy_notes"},
	{Name: "local_study_helper", Path: "packs/templates/local_study_helper"},
	{Name: "local_travel_notes", Path: "packs/templates/local_travel_notes"},
	{Name: "personal_productivity", Path: "packs/templates/personal_productivity"},
	{Name: "research_assistant", Path: "packs/templates/research_assistant"},
}

func BuiltInTemplateRefs() []TemplateRef {
	refs := append([]TemplateRef{}, builtInTemplateRefs...)
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].Name == refs[j].Name {
			return refs[i].Path < refs[j].Path
		}
		return refs[i].Name < refs[j].Name
	})
	return refs
}

func NewBuiltInTemplateCatalog(repoRoot string) (TemplateCatalog, error) {
	repoRoot = strings.TrimSpace(repoRoot)
	if repoRoot == "" {
		return TemplateCatalog{}, fmt.Errorf("repository root is required")
	}

	templates := make([]Template, 0, len(builtInTemplateRefs))
	for _, ref := range BuiltInTemplateRefs() {
		dir := filepath.Join(repoRoot, filepath.FromSlash(ref.Path))
		template, err := LoadTemplateFromDomainPackDir(dir)
		if err != nil {
			return TemplateCatalog{}, fmt.Errorf("load built-in workflow template %s: %w", ref.Name, err)
		}
		if template.Name != normalizeName(ref.Name) {
			return TemplateCatalog{}, fmt.Errorf("built-in workflow template %s loaded as %s", ref.Name, template.Name)
		}
		if template.Metadata == nil {
			template.Metadata = map[string]string{}
		}
		template.Metadata["template_ref"] = ref.Path
		templates = append(templates, template)
	}
	return NewTemplateCatalog(templates...)
}

func NewTemplateCatalog(templates ...Template) (TemplateCatalog, error) {
	catalog := TemplateCatalog{items: map[string]Template{}}
	for _, template := range templates {
		if err := catalog.Add(template); err != nil {
			return TemplateCatalog{}, err
		}
	}
	return catalog, nil
}

func (catalog *TemplateCatalog) Add(template Template) error {
	template = NormalizeTemplate(template)
	if err := ValidateTemplate(template); err != nil {
		return err
	}
	if catalog.items == nil {
		catalog.items = map[string]Template{}
	}
	if _, exists := catalog.items[template.Name]; exists {
		return fmt.Errorf("duplicate workflow template: %s", template.Name)
	}
	catalog.items[template.Name] = cloneTemplate(template)
	return nil
}

func (catalog TemplateCatalog) Get(name string) (Template, bool) {
	name = normalizeName(name)
	if name == "" || catalog.items == nil {
		return Template{}, false
	}
	template, ok := catalog.items[name]
	if !ok {
		return Template{}, false
	}
	return cloneTemplate(template), true
}

func (catalog TemplateCatalog) List() []Template {
	items := make([]Template, 0, len(catalog.items))
	for _, item := range catalog.items {
		items = append(items, cloneTemplate(item))
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})
	return items
}

func DomainPackTemplateSummaries(catalog TemplateCatalog, installed []domainpacks.PackStatus) []TemplateSummary {
	installedByName := make(map[string]domainpacks.PackStatus, len(installed))
	for _, status := range installed {
		installedByName[normalizeName(status.Name)] = status
	}

	templates := catalog.List()
	summaries := make([]TemplateSummary, 0, len(templates))
	for _, template := range templates {
		summary := TemplateSummary{
			Name:                template.Name,
			Version:             template.Version,
			Description:         template.Description,
			Category:            template.Category,
			Source:              SourceDomainPackTemplate,
			Path:                templatePathRef(template),
			Valid:               true,
			DefaultState:        template.DefaultState,
			RequiredTools:       append([]string{}, template.RequiredTools...),
			Skills:              templateSkillNames(template.Workflows),
			Workflows:           templateWorkflowNames(template.Workflows),
			ApprovalRequiredFor: append([]string{}, template.Safety.ApprovalRequiredFor...),
			BlockedActions:      append([]string{}, template.Safety.BlockedActions...),
			SensitiveDataRules:  append([]string{}, template.Safety.SensitiveDataRules...),
			SafetySummary:       templateSafetySummary(template),
			InstallAction:       "install",
		}
		if status, ok := installedByName[template.Name]; ok {
			summary.Installed = status.Installed
			summary.Enabled = status.Enabled
			summary.Valid = status.Valid
			summary.ValidationError = status.ValidationError
			summary.RepairHint = status.RepairHint
		}
		summaries = append(summaries, summary)
	}
	return summaries
}

func LoadTemplateFromDomainPackDir(dir string) (Template, error) {
	manifest, err := domainpacks.Load(dir)
	if err != nil {
		return Template{}, err
	}
	packSkills := make([]skills.Skill, 0, len(manifest.Skills))
	for _, ref := range manifest.Skills {
		skill, err := skills.Load(filepath.Join(dir, filepath.FromSlash(ref)))
		if err != nil {
			return Template{}, fmt.Errorf("load template skill %s: %w", ref, err)
		}
		packSkills = append(packSkills, skill)
	}
	return TemplateFromDomainPack(manifest, packSkills)
}

func TemplateFromDomainPack(manifest domainpacks.Manifest, packSkills []skills.Skill) (Template, error) {
	manifest = domainpacks.Normalize(manifest)
	workflows, err := FromDomainPackSkills(manifest, packSkills)
	if err != nil {
		return Template{}, err
	}
	template := Template{
		Name:          manifest.Name,
		Version:       manifest.Version,
		Description:   manifest.Description,
		Category:      manifest.Category,
		Source:        SourceDomainPackTemplate,
		Path:          manifest.Dir,
		DefaultState:  manifest.DefaultState,
		RequiredTools: manifest.RequiredTools,
		Workflows:     workflows,
		Permissions: permissionsFromMetadata(manifest.Permissions, skills.Skill{
			RequiredTools: manifest.RequiredTools,
		}),
		Safety: Safety{
			ApprovalRequiredFor: manifest.Safety.ApprovalRequiredFor,
			BlockedActions:      manifest.Safety.BlockedActions,
			SensitiveDataRules:  manifest.Safety.SensitiveDataRules,
		},
		Metadata: map[string]string{
			"pack_default_state": manifest.DefaultState,
		},
	}
	template = NormalizeTemplate(template)
	if err := ValidateTemplate(template); err != nil {
		return Template{}, err
	}
	return template, nil
}

func NormalizeTemplate(template Template) Template {
	template.Name = normalizeName(template.Name)
	template.Version = strings.TrimSpace(template.Version)
	template.Description = strings.TrimSpace(template.Description)
	template.Category = strings.TrimSpace(template.Category)
	template.Source = strings.TrimSpace(template.Source)
	template.Path = strings.TrimSpace(template.Path)
	template.DefaultState = normalizeDefaultState(template.DefaultState)
	template.RequiredTools = cleanTools(template.RequiredTools)
	template.Permissions = normalizePermissions(template.Permissions)
	template.Safety = normalizeSafety(template.Safety)
	if template.Metadata != nil {
		template.Metadata = cleanMetadata(template.Metadata)
	}

	workflows := make([]Workflow, 0, len(template.Workflows))
	for _, workflow := range template.Workflows {
		workflows = append(workflows, Normalize(workflow))
	}
	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Name < workflows[j].Name
	})
	template.Workflows = workflows
	return template
}

func ValidateTemplate(template Template) error {
	template = NormalizeTemplate(template)
	if template.Name == "" {
		return fmt.Errorf("workflow template name is required")
	}
	if !workflowNamePattern.MatchString(template.Name) {
		return fmt.Errorf("workflow template name must match %s", workflowNamePattern.String())
	}
	if template.Version == "" {
		return fmt.Errorf("workflow template version is required")
	}
	if template.Description == "" {
		return fmt.Errorf("workflow template description is required")
	}
	if template.Installed || template.Enabled {
		return fmt.Errorf("workflow templates are catalog references only and cannot be installed or enabled")
	}
	switch template.DefaultState {
	case DefaultStateDisabled, DefaultStateEnabled:
	default:
		return fmt.Errorf("workflow template default_state must be %q or %q", DefaultStateDisabled, DefaultStateEnabled)
	}
	if template.EnabledByDefault() {
		return fmt.Errorf("workflow templates must be disabled by default")
	}
	if template.Permissions.RequiresExternal() {
		return fmt.Errorf("workflow templates cannot require internet, cloud, connectors, scheduler, notifications, or embeddings")
	}
	if len(template.Workflows) == 0 {
		return fmt.Errorf("workflow template must include at least one workflow")
	}
	seen := map[string]struct{}{}
	for _, workflow := range template.Workflows {
		if err := Validate(workflow); err != nil {
			return fmt.Errorf("workflow template %s: %w", template.Name, err)
		}
		if _, exists := seen[workflow.Name]; exists {
			return fmt.Errorf("duplicate workflow in template %s: %s", template.Name, workflow.Name)
		}
		seen[workflow.Name] = struct{}{}
		if workflow.Enabled || workflow.EnabledByDefault() {
			return fmt.Errorf("workflow template %s includes enabled workflow %s", template.Name, workflow.Name)
		}
		if workflow.RunMode != RunModeManual {
			return fmt.Errorf("workflow template %s includes non-manual workflow %s", template.Name, workflow.Name)
		}
		if workflow.Permissions.RequiresExternal() {
			return fmt.Errorf("workflow template %s includes external-capability workflow %s", template.Name, workflow.Name)
		}
	}
	return nil
}

func (template Template) EnabledByDefault() bool {
	return normalizeDefaultState(template.DefaultState) == DefaultStateEnabled
}

func (template Template) LocalOnly() bool {
	return !template.RequiresExternalCapabilities()
}

func (template Template) RequiresExternalCapabilities() bool {
	if template.Permissions.RequiresExternal() {
		return true
	}
	for _, workflow := range template.Workflows {
		if workflow.Permissions.RequiresExternal() {
			return true
		}
	}
	return false
}

func templatePathRef(template Template) string {
	if template.Metadata != nil && strings.TrimSpace(template.Metadata["template_ref"]) != "" {
		return strings.TrimSpace(template.Metadata["template_ref"])
	}
	return strings.TrimSpace(template.Path)
}

func templateWorkflowNames(workflows []Workflow) []string {
	names := make([]string, 0, len(workflows))
	for _, workflow := range workflows {
		if name := strings.TrimSpace(workflow.Name); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func templateSkillNames(workflows []Workflow) []string {
	names := make([]string, 0, len(workflows))
	for _, workflow := range workflows {
		if name := strings.TrimSpace(workflow.SkillName); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func templateSafetySummary(template Template) string {
	if len(template.Safety.ApprovalRequiredFor) > 0 {
		return "Installs disabled; listed actions require approval and core policy still applies."
	}
	if len(template.Safety.BlockedActions) > 0 {
		return "Installs disabled; blocked actions cannot run through this pack."
	}
	return "Installs disabled; enable separately when ready."
}

func (template Template) Library() (Library, error) {
	return NewLibrary(template.Workflows...)
}

func cloneTemplate(template Template) Template {
	template.RequiredTools = append([]string{}, template.RequiredTools...)
	template.Workflows = append([]Workflow{}, template.Workflows...)
	template.Permissions = clonePermissions(template.Permissions)
	template.Safety = cloneSafety(template.Safety)
	if template.Metadata != nil {
		metadata := map[string]string{}
		for key, value := range template.Metadata {
			metadata[key] = value
		}
		template.Metadata = metadata
	}
	return template
}

func clonePermissions(permissions Permissions) Permissions {
	permissions.Internet = cloneCapability(permissions.Internet)
	permissions.Cloud = cloneCapability(permissions.Cloud)
	permissions.Connectors = cloneCapability(permissions.Connectors)
	permissions.Scheduler = cloneCapability(permissions.Scheduler)
	permissions.Notifications = cloneCapability(permissions.Notifications)
	permissions.Embeddings = cloneCapability(permissions.Embeddings)
	permissions.Filesystem = cloneCapability(permissions.Filesystem)
	permissions.Memory = cloneCapability(permissions.Memory)
	permissions.Secrets = cloneCapability(permissions.Secrets)
	return permissions
}

func cloneCapability(capability Capability) Capability {
	capability.Scopes = append([]string{}, capability.Scopes...)
	capability.Names = append([]string{}, capability.Names...)
	return capability
}

func cloneSafety(safety Safety) Safety {
	safety.ApprovalRequiredFor = append([]string{}, safety.ApprovalRequiredFor...)
	safety.BlockedActions = append([]string{}, safety.BlockedActions...)
	safety.SensitiveDataRules = append([]string{}, safety.SensitiveDataRules...)
	return safety
}
