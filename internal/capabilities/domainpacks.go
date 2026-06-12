package capabilities

import (
	"fmt"
	"path/filepath"
	"strings"

	"yemaka/internal/domainpacks"
	"yemaka/internal/skills"
	"yemaka/internal/tools"
	"yemaka/internal/workflows"
)

const (
	domainPackSource         = "domain_pack"
	domainPackTemplateSource = "domain_pack_template"
)

// DomainPackCapability converts an installed domain pack status and manifest
// into a registry entry without changing pack state or policy.
func DomainPackCapability(status domainpacks.PackStatus, manifest domainpacks.Manifest) (Capability, error) {
	return DomainPackCapabilityWithSkills(status, manifest, nil)
}

// DomainPackCapabilityWithSkills converts an installed domain pack plus its
// skill metadata into a registry entry. Skill metadata is used only for
// deterministic matching; it does not enable any pack or tool.
func DomainPackCapabilityWithSkills(status domainpacks.PackStatus, manifest domainpacks.Manifest, packSkills []skills.Skill) (Capability, error) {
	manifest = domainpacks.Normalize(manifest)
	packSkills = normalizePackSkills(packSkills)
	name := firstNonEmpty(status.Name, manifest.Name)
	if strings.TrimSpace(name) == "" {
		return Capability{}, fmt.Errorf("domain pack capability name is required")
	}
	requiresApproval := domainPackRequiresApproval(manifest, packSkills)

	capability := Capability{
		Name:             name,
		Kind:             KindDomainPack,
		Description:      firstNonEmpty(status.Description, manifest.Description),
		Source:           domainPackSource,
		Aliases:          domainPackAliases(status, manifest, packSkills),
		Tags:             domainPackTags(status, manifest, requiresApproval),
		Provides:         domainPackProvides(status, manifest, packSkills),
		RequiresApproval: requiresApproval,
		Metadata:         domainPackMetadata(status, manifest, packSkills),
	}

	manifestErr := domainpacks.Validate(manifest)
	switch {
	case !status.Installed:
		capability.State = StateUnavailable
		capability.Reason = "domain pack is not installed"
		capability.ConfigureHint = "Install the domain pack, validate its manifest, then enable it explicitly."
	case !status.Valid:
		capability.State = StateUnavailable
		capability.Reason = firstNonEmpty(status.ValidationError, "domain pack manifest is invalid")
		capability.ConfigureHint = "Fix the domain pack manifest before enabling or routing work to it."
	case manifest.Name != "" && status.Name != "" && manifest.Name != status.Name:
		capability.State = StateUnavailable
		capability.Reason = "domain pack status and manifest names do not match"
		capability.ConfigureHint = "Reinstall or repair the domain pack before enabling it."
	case manifestErr != nil:
		capability.State = StateUnavailable
		capability.Reason = fmt.Sprintf("domain pack manifest is invalid: %v", manifestErr)
		capability.ConfigureHint = "Fix the domain pack manifest before enabling or routing work to it."
	case status.Enabled:
		capability.State = StateReady
	default:
		capability.State = StateDisabled
		capability.Configured = true
		capability.Reason = "domain pack is disabled"
		capability.ConfigureHint = "Enable the domain pack explicitly before routing work to it."
	}

	return normalizeCapability(capability)
}

// DomainPackTemplateCapability exposes a built-in workflow/domain-pack template
// as a setup-only registry entry. It is intentionally never ready: users must
// explicitly install and enable the pack before Yemaka routes work to it.
func DomainPackTemplateCapability(template workflows.Template) (Capability, error) {
	template = workflows.NormalizeTemplate(template)
	if err := workflows.ValidateTemplate(template); err != nil {
		return Capability{}, err
	}
	capability := Capability{
		Name:             template.Name,
		Kind:             KindDomainPack,
		Description:      template.Description,
		Source:           domainPackTemplateSource,
		State:            StateNeedsConfig,
		Available:        true,
		Enabled:          false,
		Configured:       false,
		RequiresApproval: domainPackTemplateRequiresApproval(template),
		Reason:           "matching workflow pack template is available but not installed or enabled",
		ConfigureHint:    "Install the local workflow pack template, then enable it explicitly before routing work to it.",
		Aliases:          domainPackTemplateAliases(template),
		Tags:             domainPackTemplateTags(template),
		Provides:         domainPackTemplateProvides(template),
		Metadata:         domainPackTemplateMetadata(template),
	}
	return normalizeCapability(capability)
}

func (r *Registry) AddDomainPackStatus(status domainpacks.PackStatus, manifest domainpacks.Manifest) error {
	capability, err := DomainPackCapability(status, manifest)
	if err != nil {
		return err
	}
	return r.AddDomainPack(capability)
}

func DomainPackCapabilitiesFromRoot(profilePackRoot string) ([]Capability, error) {
	profilePackRoot = strings.TrimSpace(profilePackRoot)
	if profilePackRoot == "" {
		return nil, nil
	}
	statuses, err := domainpacks.List(profilePackRoot)
	if err != nil {
		return nil, err
	}
	manifests := make(map[string]domainpacks.Manifest, len(statuses))
	skillSets := make(map[string][]skills.Skill, len(statuses))
	for i := range statuses {
		status := statuses[i]
		if !status.Valid || strings.TrimSpace(status.Dir) == "" {
			continue
		}
		manifest, err := domainpacks.Load(status.Dir)
		if err != nil {
			statuses[i].Valid = false
			statuses[i].Enabled = false
			statuses[i].ValidationError = err.Error()
			continue
		}
		manifests[status.Name] = manifest
		if packSkills, err := loadDomainPackSkills(status.Dir, manifest); err == nil {
			skillSets[status.Name] = packSkills
		}
	}
	return domainPackCapabilities(statuses, manifests, skillSets)
}

func DomainPackCapabilities(statuses []domainpacks.PackStatus, manifests map[string]domainpacks.Manifest) ([]Capability, error) {
	return domainPackCapabilities(statuses, manifests, nil)
}

func DomainPackTemplateCapabilities(catalog workflows.TemplateCatalog) ([]Capability, error) {
	templates := catalog.List()
	capabilities := make([]Capability, 0, len(templates))
	for _, template := range templates {
		capability, err := DomainPackTemplateCapability(template)
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, capability)
	}
	sortCapabilities(capabilities)
	return capabilities, nil
}

func domainPackCapabilities(statuses []domainpacks.PackStatus, manifests map[string]domainpacks.Manifest, skillSets map[string][]skills.Skill) ([]Capability, error) {
	capabilities := make([]Capability, 0, len(statuses))
	for _, status := range statuses {
		manifest := manifests[status.Name]
		capability, err := DomainPackCapabilityWithSkills(status, manifest, skillSets[status.Name])
		if err != nil {
			return nil, err
		}
		capabilities = append(capabilities, capability)
	}
	sortCapabilities(capabilities)
	return capabilities, nil
}

func domainPackAliases(status domainpacks.PackStatus, manifest domainpacks.Manifest, packSkills []skills.Skill) []string {
	aliases := []string{
		status.Description,
		manifest.Description,
		firstNonEmpty(status.Category, manifest.Category),
	}
	aliases = append(aliases, status.RequiredTools...)
	aliases = append(aliases, status.OptionalTools...)
	aliases = append(aliases, manifest.RequiredTools...)
	aliases = append(aliases, manifest.OptionalTools...)
	aliases = append(aliases, domainPackSkillAliases(packSkills)...)
	return aliases
}

func domainPackTags(status domainpacks.PackStatus, manifest domainpacks.Manifest, requiresApproval bool) []string {
	tags := []string{domainPackSource}
	tags = append(tags, firstNonEmpty(status.Category, manifest.Category))
	tags = append(tags, permissionTags(manifest.Permissions)...)
	if requiresApproval {
		tags = append(tags, "requires_approval")
	}
	return tags
}

func domainPackProvides(status domainpacks.PackStatus, manifest domainpacks.Manifest, packSkills []skills.Skill) []string {
	provides := []string{
		status.Name,
		manifest.Name,
		firstNonEmpty(status.Category, manifest.Category),
	}
	provides = append(provides, prefixedValues("tool:", status.RequiredTools)...)
	provides = append(provides, prefixedValues("tool:", status.OptionalTools)...)
	provides = append(provides, prefixedValues("tool:", manifest.RequiredTools)...)
	provides = append(provides, prefixedValues("tool:", manifest.OptionalTools)...)
	provides = append(provides, permissionProvides(manifest.Permissions)...)
	provides = append(provides, domainPackSkillProvides(packSkills)...)
	return provides
}

func domainPackRequiresApproval(manifest domainpacks.Manifest, packSkills []skills.Skill) bool {
	if len(manifest.Safety.ApprovalRequiredFor) > 0 {
		return true
	}
	for _, permission := range permissionDeclarations(manifest.Permissions) {
		if permission.Required {
			return true
		}
	}
	if domainPackToolsRequireApproval(manifest.RequiredTools, manifest.OptionalTools) {
		return true
	}
	for _, skill := range packSkills {
		if domainPackToolsRequireApproval(skill.RequiredTools) {
			return true
		}
	}
	return false
}

func domainPackToolsRequireApproval(toolLists ...[]string) bool {
	catalog := tools.ToolCapabilitiesByName()
	for _, toolList := range toolLists {
		for _, tool := range toolList {
			tool = strings.TrimSpace(tool)
			if tool == "" {
				continue
			}
			capability, ok := catalog[tool]
			if !ok {
				return true
			}
			if capability.RequiresApproval || capability.Mutating || !capability.EnabledByDefault {
				return true
			}
		}
	}
	return false
}

func domainPackMetadata(status domainpacks.PackStatus, manifest domainpacks.Manifest, packSkills []skills.Skill) map[string]string {
	metadata := map[string]string{
		"installed":     fmt.Sprintf("%t", status.Installed),
		"valid":         fmt.Sprintf("%t", status.Valid),
		"default_state": manifest.DefaultState,
	}
	if value := firstNonEmpty(status.Version, manifest.Version); value != "" {
		metadata["version"] = value
	}
	if status.Dir != "" {
		metadata["dir"] = status.Dir
	}
	if len(status.RequiredTools) > 0 || len(manifest.RequiredTools) > 0 {
		metadata["required_tools"] = strings.Join(normalizeList(append(status.RequiredTools, manifest.RequiredTools...)), ",")
	}
	if len(status.OptionalTools) > 0 || len(manifest.OptionalTools) > 0 {
		metadata["optional_tools"] = strings.Join(normalizeList(append(status.OptionalTools, manifest.OptionalTools...)), ",")
	}
	if permissions := permissionTags(manifest.Permissions); len(permissions) > 0 {
		metadata["permissions"] = strings.Join(permissions, ",")
	}
	if skillNames := domainPackSkillNames(packSkills); len(skillNames) > 0 {
		metadata["skills"] = strings.Join(skillNames, ",")
	}
	return metadata
}

func loadDomainPackSkills(packDir string, manifest domainpacks.Manifest) ([]skills.Skill, error) {
	packDir = strings.TrimSpace(packDir)
	if packDir == "" {
		packDir = strings.TrimSpace(manifest.Dir)
	}
	if packDir == "" {
		return nil, nil
	}
	out := make([]skills.Skill, 0, len(manifest.Skills))
	for _, ref := range manifest.Skills {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		skill, err := skills.Load(filepath.Join(packDir, filepath.FromSlash(ref)))
		if err != nil {
			return nil, err
		}
		out = append(out, skill)
	}
	return normalizePackSkills(out), nil
}

func normalizePackSkills(packSkills []skills.Skill) []skills.Skill {
	out := make([]skills.Skill, 0, len(packSkills))
	for _, skill := range packSkills {
		skill.Name = strings.TrimSpace(skill.Name)
		skill.Version = strings.TrimSpace(skill.Version)
		skill.Description = strings.TrimSpace(skill.Description)
		skill.Triggers = normalizeList(skill.Triggers)
		skill.RequiredTools = normalizeList(skill.RequiredTools)
		if skill.Name == "" {
			continue
		}
		out = append(out, skill)
	}
	return out
}

func domainPackSkillAliases(packSkills []skills.Skill) []string {
	aliases := []string{}
	for _, skill := range packSkills {
		aliases = append(aliases, skill.Name, skill.Description)
		aliases = append(aliases, skill.Triggers...)
		aliases = append(aliases, skill.RequiredTools...)
	}
	return aliases
}

func domainPackSkillProvides(packSkills []skills.Skill) []string {
	provides := []string{}
	for _, skill := range packSkills {
		provides = append(provides, skill.Name, "skill:"+skill.Name)
		provides = append(provides, prefixedValues("trigger:", skill.Triggers)...)
		provides = append(provides, skill.Triggers...)
		provides = append(provides, prefixedValues("tool:", skill.RequiredTools)...)
	}
	return provides
}

func domainPackSkillNames(packSkills []skills.Skill) []string {
	names := make([]string, 0, len(packSkills))
	for _, skill := range packSkills {
		if strings.TrimSpace(skill.Name) != "" {
			names = append(names, skill.Name)
		}
	}
	return normalizeList(names)
}

func domainPackTemplateAliases(template workflows.Template) []string {
	aliases := []string{template.Description, template.Category}
	aliases = append(aliases, template.RequiredTools...)
	for _, workflow := range template.Workflows {
		aliases = append(aliases, workflow.Name, workflow.SkillName, workflow.Description)
		aliases = append(aliases, workflow.Triggers...)
		aliases = append(aliases, workflow.RequiredTools...)
	}
	return aliases
}

func domainPackTemplateTags(template workflows.Template) []string {
	tags := []string{domainPackSource, domainPackTemplateSource, template.Category}
	if domainPackTemplateRequiresApproval(template) {
		tags = append(tags, "requires_approval")
	}
	return tags
}

func domainPackTemplateProvides(template workflows.Template) []string {
	provides := []string{template.Name, template.Category}
	provides = append(provides, prefixedValues("tool:", template.RequiredTools)...)
	for _, workflow := range template.Workflows {
		provides = append(provides, workflow.Name, workflow.SkillName, "workflow:"+workflow.Name)
		if workflow.SkillName != "" {
			provides = append(provides, "skill:"+workflow.SkillName)
		}
		provides = append(provides, workflow.Triggers...)
		provides = append(provides, prefixedValues("trigger:", workflow.Triggers)...)
		provides = append(provides, prefixedValues("tool:", workflow.RequiredTools)...)
	}
	return provides
}

func domainPackTemplateRequiresApproval(template workflows.Template) bool {
	if len(template.Safety.ApprovalRequiredFor) > 0 {
		return true
	}
	if domainPackToolsRequireApproval(template.RequiredTools) {
		return true
	}
	for _, workflow := range template.Workflows {
		if len(workflow.Safety.ApprovalRequiredFor) > 0 {
			return true
		}
		if workflow.Permissions.Filesystem.Required ||
			workflow.Permissions.Memory.Required ||
			workflow.Permissions.Secrets.Required {
			return true
		}
		if domainPackToolsRequireApproval(workflow.RequiredTools) {
			return true
		}
	}
	return false
}

func domainPackTemplateMetadata(template workflows.Template) map[string]string {
	metadata := map[string]string{
		"template":      "true",
		"default_state": template.DefaultState,
	}
	if template.Version != "" {
		metadata["version"] = template.Version
	}
	if template.Path != "" {
		metadata["template_path"] = template.Path
	}
	if template.Source != "" {
		metadata["template_source"] = template.Source
	}
	if len(template.RequiredTools) > 0 {
		metadata["required_tools"] = strings.Join(normalizeList(template.RequiredTools), ",")
	}
	if len(template.Workflows) > 0 {
		names := make([]string, 0, len(template.Workflows))
		for _, workflow := range template.Workflows {
			names = append(names, workflow.Name)
		}
		metadata["workflows"] = strings.Join(normalizeList(names), ",")
	}
	if template.Metadata != nil {
		if ref := strings.TrimSpace(template.Metadata["template_ref"]); ref != "" {
			metadata["template_ref"] = ref
		}
	}
	return metadata
}

func permissionTags(permissions domainpacks.Permissions) []string {
	tags := make([]string, 0, 6)
	for _, item := range namedPermissions(permissions) {
		if permissionDeclared(item.permission) {
			tags = append(tags, "permission:"+item.name)
		}
	}
	return tags
}

func permissionProvides(permissions domainpacks.Permissions) []string {
	provides := []string{}
	for _, item := range namedPermissions(permissions) {
		if !permissionDeclared(item.permission) {
			continue
		}
		provides = append(provides, item.name)
		provides = append(provides, prefixedValues(item.name+":scope:", item.permission.Scopes)...)
		provides = append(provides, prefixedValues(item.name+":domain:", item.permission.AllowedDomains)...)
		provides = append(provides, prefixedValues(item.name+":method:", item.permission.AllowedMethods)...)
		provides = append(provides, prefixedValues(item.name+":path:", item.permission.Paths)...)
		provides = append(provides, prefixedValues(item.name+":name:", item.permission.Names)...)
	}
	return provides
}

func permissionDeclarations(permissions domainpacks.Permissions) []domainpacks.PermissionDeclaration {
	declarations := make([]domainpacks.PermissionDeclaration, 0, 6)
	for _, item := range namedPermissions(permissions) {
		declarations = append(declarations, item.permission)
	}
	return declarations
}

func namedPermissions(permissions domainpacks.Permissions) []struct {
	name       string
	permission domainpacks.PermissionDeclaration
} {
	return []struct {
		name       string
		permission domainpacks.PermissionDeclaration
	}{
		{name: "connectors", permission: permissions.Connectors},
		{name: "filesystem", permission: permissions.Filesystem},
		{name: "internet", permission: permissions.Internet},
		{name: "notifications", permission: permissions.Notifications},
		{name: "scheduler", permission: permissions.Scheduler},
		{name: "secrets", permission: permissions.Secrets},
	}
}

func permissionDeclared(permission domainpacks.PermissionDeclaration) bool {
	return permission.Required ||
		len(permission.Scopes) > 0 ||
		len(permission.AllowedDomains) > 0 ||
		len(permission.AllowedMethods) > 0 ||
		len(permission.Paths) > 0 ||
		len(permission.Names) > 0 ||
		len(permission.SecretRefs) > 0
}

func prefixedValues(prefix string, values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, prefix+value)
		}
	}
	return out
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
